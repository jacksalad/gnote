//go:build windows

package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// 测试专用的 Win32 绑定，用来给原文件设置并读回各种元数据。
var (
	p2kernel32 = syscall.NewLazyDLL("kernel32.dll")
	p2advapi32 = syscall.NewLazyDLL("advapi32.dll")

	p2GetFileAttributesW           = p2kernel32.NewProc("GetFileAttributesW")
	p2SetFileAttributesW           = p2kernel32.NewProc("SetFileAttributesW")
	p2SetFileTime                  = p2kernel32.NewProc("SetFileTime")
	p2DeviceIoControl              = p2kernel32.NewProc("DeviceIoControl")
	p2GetCompressedFileSizeW       = p2kernel32.NewProc("GetCompressedFileSizeW")
	p2GetNamedSecurityInfoW        = p2advapi32.NewProc("GetNamedSecurityInfoW")
	p2GetSecurityDescriptorControl = p2advapi32.NewProc("GetSecurityDescriptorControl")
	p2SetNamedSecurityInfoW        = p2advapi32.NewProc("SetNamedSecurityInfoW")
	p2LocalFree                    = p2kernel32.NewProc("LocalFree")
)

const (
	p2AttrHidden     = 0x2
	p2AttrSystem     = 0x4
	p2AttrCompressed = 0x800

	p2FSCTLSetCompression      = 0x9C040
	p2CompressionFormatDefault = 1
	p2SEFileObject             = 1
	p2DACLSecurityInformation  = 0x4
	p2ProtectedDACL            = 0x80000000
	p2SEDACLPresent            = 0x4
	p2SEDACLProtected          = 0x1000
)

func p2UTF16(t *testing.T, s string) *uint16 {
	t.Helper()
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func p2Attrs(t *testing.T, path string) uint32 {
	t.Helper()
	ret, _, err := p2GetFileAttributesW.Call(uintptr(unsafe.Pointer(p2UTF16(t, path))))
	if ret == 0xFFFFFFFF {
		t.Fatalf("GetFileAttributesW(%s): %v", path, err)
	}
	return uint32(ret)
}

func p2SetAttrs(t *testing.T, path string, attrs uint32) {
	t.Helper()
	ret, _, err := p2SetFileAttributesW.Call(uintptr(unsafe.Pointer(p2UTF16(t, path))), uintptr(attrs))
	if ret == 0 {
		t.Fatalf("SetFileAttributesW(%s): %v", path, err)
	}
}

func p2CreationTime(t *testing.T, path string) int64 {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	ft := fi.Sys().(*syscall.Win32FileAttributeData).CreationTime
	return int64(uint32(ft.HighDateTime))<<32 | int64(uint32(ft.LowDateTime))
}

// p2SetCreationTime 把创建时间改成一个明显不同的时刻（不必为了 100ns 精度去 sleep）。
func p2SetCreationTime(t *testing.T, path string, when time.Time) {
	t.Helper()
	h, err := syscall.CreateFile(p2UTF16(t, path), syscall.FILE_WRITE_ATTRIBUTES,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatalf("CreateFile(%s): %v", path, err)
	}
	defer syscall.CloseHandle(h)
	ft := syscall.NsecToFiletime(when.UnixNano())
	ret, _, err2 := p2SetFileTime.Call(uintptr(h), uintptr(unsafe.Pointer(&ft)), 0, 0)
	if ret == 0 {
		t.Fatalf("SetFileTime(%s): %v", path, err2)
	}
}

// p2DirNames 返回目录里现存条目的名字，用来确认没有留下临时文件。
func p2DirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func p2WantDir(t *testing.T, dir string, want ...string) {
	t.Helper()
	sort.Strings(want)
	if got := p2DirNames(t, dir); !strings.EqualFold(strings.Join(got, ","), strings.Join(want, ",")) {
		t.Errorf("directory entries = %v, want %v (no leftover temp files)", got, want)
	}
}

// p2Dacl 读回文件的 DACL 原始字节与 “禁止继承” 标志。
// GetNamedSecurityInfoW 返回的安全描述符是自相对形式，DACL 指针要通过它的 ppDacl 参数取，
// 不能交给 GetSecurityDescriptorDacl（会访问冲突）。指针用 unsafe.Pointer 变量承接，go vet 才不报警。
func p2Dacl(t *testing.T, path string) (raw []byte, protected bool) {
	t.Helper()
	var sd unsafe.Pointer
	var dacl *byte
	ret, _, err := p2GetNamedSecurityInfoW.Call(
		uintptr(unsafe.Pointer(p2UTF16(t, path))), p2SEFileObject, p2DACLSecurityInformation,
		0, 0, uintptr(unsafe.Pointer(&dacl)), 0, uintptr(unsafe.Pointer(&sd)))
	if ret != 0 {
		t.Fatalf("GetNamedSecurityInfoW(%s): %v", path, err)
	}
	defer p2LocalFree.Call(uintptr(sd))

	var control uint16
	var revision uint32
	if r, _, err := p2GetSecurityDescriptorControl.Call(uintptr(sd),
		uintptr(unsafe.Pointer(&control)), uintptr(unsafe.Pointer(&revision))); r == 0 {
		t.Fatalf("GetSecurityDescriptorControl(%s): %v", path, err)
	}
	protected = control&p2SEDACLProtected != 0
	if control&p2SEDACLPresent == 0 || dacl == nil {
		return nil, protected
	}
	aclHdr := (*[8]byte)(unsafe.Pointer(dacl))
	size := int(aclHdr[2]) | int(aclHdr[3])<<8
	return append([]byte(nil), (*[4096]byte)(unsafe.Pointer(dacl))[:size]...), protected
}

// p2EveryoneReadACL 一条 “Everyone: 只读” 的允许 ACE。DACL 字节按 winnt.h 里 ACL / ACE 的布局手写：
// ACL 头 8 字节（版本 2、总长 36、1 条 ACE），ACE 头 4 字节（允许访问、28 字节）、访问掩码、SID S-1-1-0。
var p2EveryoneReadACL = []byte{
	0x02, 0x00, 36, 0x00, 1, 0x00, 0x00, 0x00,
	0x00, 0x00, 28, 0x00, 0x89, 0x00, 0x12, 0x00,
	0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00,
}

// p2ProtectWithEveryoneReadDacl 用与实现相同的 API 给文件设一条显式 ACE 并禁止继承，
// 相当于用户在 “属性 → 安全” 里给共享目录下的私有文件收紧过权限。
func p2ProtectWithEveryoneReadDacl(t *testing.T, path string) {
	t.Helper()
	acl := append([]byte(nil), p2EveryoneReadACL...)
	ret, _, err := p2SetNamedSecurityInfoW.Call(
		uintptr(unsafe.Pointer(p2UTF16(t, path))), p2SEFileObject, p2DACLSecurityInformation|p2ProtectedDACL,
		0, 0, uintptr(unsafe.Pointer(&acl[0])), 0)
	if ret != 0 {
		t.Fatalf("SetNamedSecurityInfoW(%s): %v", path, err)
	}
}

// p2Compress 给空文件打开 NTFS 压缩；卷不支持时返回 false。
func p2Compress(path string) bool {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	// FSCTL_SET_COMPRESSION 要求句柄同时具有读写权限，输入参数是 USHORT。
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ|syscall.GENERIC_WRITE, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h)
	format := uint16(p2CompressionFormatDefault)
	var returned uint32
	ret, _, _ := p2DeviceIoControl.Call(uintptr(h), p2FSCTLSetCompression,
		uintptr(unsafe.Pointer(&format)), unsafe.Sizeof(format), 0, 0, uintptr(unsafe.Pointer(&returned)), 0)
	return ret != 0
}

func p2CompressedSize(t *testing.T, path string) uint64 {
	t.Helper()
	var high uint32
	low, _, err := p2GetCompressedFileSizeW.Call(uintptr(unsafe.Pointer(p2UTF16(t, path))), uintptr(unsafe.Pointer(&high)))
	if low == 0xFFFFFFFF {
		if e, ok := err.(syscall.Errno); ok && e != 0 {
			t.Fatalf("GetCompressedFileSizeW(%s): %v", path, e)
		}
	}
	return uint64(high)<<32 | uint64(low)
}

// p2OpenExclusiveForRead 以读者身份独占打开（不共享删除），期间无法原子替换该文件。
func p2OpenExclusiveForRead(t *testing.T, path string) syscall.Handle {
	t.Helper()
	h, err := syscall.CreateFile(p2UTF16(t, path), syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatalf("CreateFile(%s): %v", path, err)
	}
	return h
}

// 保存之后文件不能从资源管理器里“消失”：隐藏 / 系统属性必须跟着新内容留下。
func TestWriteFileAtomic_KeepsHiddenAndSystemAttributes(t *testing.T) {
	for _, c := range []struct {
		name string
		attr uint32
	}{
		{"hidden", p2AttrHidden},
		{"system", p2AttrSystem},
		{"hidden+system", p2AttrHidden | p2AttrSystem},
		{"ordinary", 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "a.cfg")
			if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
				t.Fatal(err)
			}
			p2SetAttrs(t, path, c.attr|0x80)

			if err := writeFileAtomic(path, []byte("v2")); err != nil {
				t.Fatalf("writeFileAtomic: %v", err)
			}
			got := p2Attrs(t, path)
			if got&(p2AttrHidden|p2AttrSystem) != c.attr {
				t.Errorf("attributes = %#x, want %#x kept", got, c.attr)
			}
			if data, _ := os.ReadFile(path); string(data) != "v2" {
				t.Errorf("content = %q", data)
			}
		})
	}
}

// 原子替换会换掉目录项，创建时间必须显式搬到新文件上。
func TestWriteFileAtomic_KeepsCreationTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "created-long-ago.txt")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	born := time.Now().Add(-72 * time.Hour).Truncate(time.Second)
	p2SetCreationTime(t, path, born)
	if got := p2CreationTime(t, path); got != born.UnixNano()/100+116444736000000000 {
		t.Fatalf("precondition: creation time not applied: %d", got)
	}

	if err := writeFileAtomic(path, []byte("v2")); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	if got, want := p2CreationTime(t, path), born.UnixNano()/100+116444736000000000; got != want {
		t.Errorf("creation time = %d, want %d (reset by the replace)", got, want)
	}
}

// 文件上显式设置并禁止继承的 DACL 不能被目录继承来的权限顶掉。
func TestWriteFileAtomic_KeepsExplicitDacl(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private.txt")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	p2ProtectWithEveryoneReadDacl(t, path)
	wantRaw, wantProtected := p2Dacl(t, path)
	if !wantProtected || len(wantRaw) == 0 {
		t.Fatalf("precondition: explicit protected DACL not set (protected=%v len=%d)", wantProtected, len(wantRaw))
	}

	if err := writeFileAtomic(path, []byte("v2")); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	gotRaw, gotProtected := p2Dacl(t, path)
	if !gotProtected || string(gotRaw) != string(wantRaw) {
		t.Errorf("explicit DACL replaced by inherited permissions: protected=%v dacl=% X, want protected=%v dacl=% X",
			gotProtected, gotRaw, wantProtected, wantRaw)
	}
	if data, _ := os.ReadFile(path); string(data) != "v2" {
		t.Errorf("content = %q", data)
	}
}

// Zone.Identifier 之类的备用数据流挂在文件记录上，替换目录项时不能丢。
func TestWriteFileAtomic_KeepsAlternateDataStreams(t *testing.T) {
	path := filepath.Join(t.TempDir(), "downloaded.txt")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	const zone = "[ZoneTransfer3]\r\nZoneId=3\r\n"
	if err := os.WriteFile(path+":Zone.Identifier", []byte(zone), 0o644); err != nil {
		t.Skipf("alternate data streams unavailable: %v", err)
	}

	if err := writeFileAtomic(path, []byte("v2")); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	got, err := os.ReadFile(path + ":Zone.Identifier")
	if err != nil {
		t.Fatalf("alternate data stream lost after save: %v", err)
	}
	if string(got) != zone {
		t.Errorf("alternate data stream = %q, want %q", got, zone)
	}
}

// p2WriteDurably 写入并落盘。NTFS 压缩发生在数据刷到磁盘时，不 Sync 的话量到的还是未压缩的大小。
func p2WriteDurably(t *testing.T, path string, data []byte) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// 按文件（而非按目录）压缩的 NTFS 文件，保存后必须还是压缩的。
func TestWriteFileAtomic_KeepsNtfsCompression(t *testing.T) {
	path := filepath.Join(t.TempDir(), "compressed.log")
	p2WriteDurably(t, path, nil)
	if !p2Compress(path) {
		t.Skip("this volume does not support per-file NTFS compression")
	}
	content := []byte(strings.Repeat("hello world ", 4000))
	p2WriteDurably(t, path, content)
	if p2Attrs(t, path)&p2AttrCompressed == 0 || p2CompressedSize(t, path) >= uint64(len(content)) {
		t.Skipf("compression is reported but not effective on this volume (attrs=%#x size=%d of %d)",
			p2Attrs(t, path), p2CompressedSize(t, path), len(content))
	}

	next := append([]byte("saved by gnote\n"), content...)
	if err := writeFileAtomic(path, next); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	if p2Attrs(t, path)&p2AttrCompressed == 0 {
		t.Errorf("NTFS compression attribute lost after save")
	}
	if got := p2CompressedSize(t, path); got >= uint64(len(next)) {
		t.Errorf("file is flagged compressed but occupies %d bytes for %d bytes of data", got, len(next))
	}
	if data, _ := os.ReadFile(path); !strings.HasPrefix(string(data), "saved by gnote\n") || len(data) != len(next) {
		t.Errorf("content corrupted: %d bytes", len(data))
	}
}

// 有多个硬链接的文件：所有名字必须继续指向同一份内容，不能悄悄分叉成 a.txt=v2、b.txt=v1。
func TestWriteFileAtomic_HardLinksStayInSync(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")
	if err := os.WriteFile(a, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(a, b); err != nil {
		t.Skipf("hard links unavailable here: %v", err)
	}

	if err := writeFileAtomic(a, []byte("v2")); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	for _, p := range []string{a, b} {
		if got, _ := os.ReadFile(p); string(got) != "v2" {
			t.Errorf("%s = %q, want %q (hard link broken by the save)", filepath.Base(p), got, "v2")
		}
	}
	if err := writeFileAtomic(b, []byte("v3")); err != nil {
		t.Fatalf("writeFileAtomic via the other name: %v", err)
	}
	if got, _ := os.ReadFile(a); string(got) != "v3" {
		t.Errorf("a.txt = %q, want v3", got)
	}
	p2WantDir(t, dir, "a.txt", "b.txt")
}

// 通过符号链接保存：链接本身保持不变，真正被更新的是它指向的文件。
func TestWriteFileAtomic_UpdatesSymlinkTarget(t *testing.T) {
	dir := t.TempDir()
	real, link := filepath.Join(dir, "real.txt"), filepath.Join(dir, "link.txt")
	if err := os.WriteFile(real, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks cannot be created here: %v", err)
	}

	if err := writeFileAtomic(link, []byte("v2")); err != nil {
		t.Fatalf("writeFileAtomic via symlink: %v", err)
	}
	fi, err := os.Lstat(link)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the symlink was replaced by a regular file")
	}
	if got, _ := os.ReadFile(real); string(got) != "v2" {
		t.Errorf("real target = %q, want v2", got)
	}
	p2WantDir(t, dir, "link.txt", "real.txt")
}

// 指向尚不存在文件的符号链接：在链接指向的位置创建文件，而不是用普通文件顶掉链接。
func TestWriteFileAtomic_CreatesDanglingSymlinkTarget(t *testing.T) {
	dir := t.TempDir()
	missing, link := filepath.Join(dir, "missing.txt"), filepath.Join(dir, "link.txt")
	if err := os.Symlink(missing, link); err != nil {
		t.Skipf("symlinks cannot be created here: %v", err)
	}

	if err := writeFileAtomic(link, []byte("created")); err != nil {
		t.Fatalf("writeFileAtomic via dangling symlink: %v", err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the dangling symlink was replaced by a regular file")
	}
	if got, _ := os.ReadFile(missing); string(got) != "created" {
		t.Errorf("target = %q, want %q", got, "created")
	}
	p2WantDir(t, dir, "link.txt", "missing.txt")
}

// 只读文件：给出明确的说明，原文件不动，也不留临时文件。
func TestWriteFileAtomic_ReadOnlyTargetGetsClearMessage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ro.txt")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	err := writeFileAtomic(path, []byte("v2"))
	if err == nil {
		t.Fatal("saving over a read-only file must fail")
	}
	if !strings.Contains(err.Error(), "只读") || strings.Contains(err.Error(), ".gnote-") {
		t.Errorf("error should explain the read-only file without the temp path: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "v1" {
		t.Errorf("read-only file was modified: %q", got)
	}
	p2WantDir(t, dir, "ro.txt")
}

// 保存瞬间目标文件被别的程序短暂占用（没有共享删除）：等它放开再替换，而不是直接失败。
func TestWriteFileAtomic_WaitsOutABriefLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "brief.txt")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := p2OpenExclusiveForRead(t, path)
	go func() {
		time.Sleep(80 * time.Millisecond)
		syscall.CloseHandle(h)
	}()

	if err := writeFileAtomic(path, []byte("v2")); err != nil {
		t.Fatalf("a brief lock must be waited out, got: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "v2" {
		t.Errorf("content = %q", got)
	}
	p2WantDir(t, dir, "brief.txt")
}

// 目标文件一直被占用：重试几次后给出不暴露临时文件路径的可读说明，原文件不动。
func TestWriteFileAtomic_PersistentLockGivesReadableErrorWithoutTempPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "locked.txt")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := p2OpenExclusiveForRead(t, path)

	start := time.Now()
	err := writeFileAtomic(path, []byte("v2"))
	elapsed := time.Since(start)
	syscall.CloseHandle(h)

	if err == nil {
		t.Fatal("saving over a locked file must fail")
	}
	if strings.Contains(err.Error(), ".gnote-") || strings.Contains(err.Error(), ".tmp") {
		t.Errorf("error exposes the temp file path: %v", err)
	}
	if !strings.Contains(err.Error(), "占用") || !strings.Contains(err.Error(), "locked.txt") {
		t.Errorf("error should say which file is in use: %v", err)
	}
	if elapsed < 150*time.Millisecond {
		t.Errorf("gave up after %v without retrying", elapsed)
	}
	if got, _ := os.ReadFile(path); string(got) != "v1" {
		t.Errorf("locked file was modified: %q", got)
	}
	p2WantDir(t, dir, "locked.txt")
}

// 全新文件：内容写入、目录里不留临时文件。
func TestWriteFileAtomic_CreatesNewFileCleanly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	if err := writeFileAtomic(path, []byte("hello")); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "hello" {
		t.Errorf("content = %q", got)
	}
	p2WantDir(t, dir, "new.txt")
}
