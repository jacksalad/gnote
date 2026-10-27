//go:build windows

// Package clipboard 提供纯 Go Win32 剪贴板读写操作
package clipboard

import (
	"syscall"
	"time"
	"unsafe"
)

const (
	CF_UNICODETEXT = 13
	GMEM_MOVEABLE  = 0x0002
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	openClipboard    = user32.NewProc("OpenClipboard")
	closeClipboard   = user32.NewProc("CloseClipboard")
	emptyClipboard   = user32.NewProc("EmptyClipboard")
	getClipboardData = user32.NewProc("GetClipboardData")
	setClipboardData = user32.NewProc("SetClipboardData")

	globalAlloc  = kernel32.NewProc("GlobalAlloc")
	globalFree   = kernel32.NewProc("GlobalFree")
	globalLock   = kernel32.NewProc("GlobalLock")
	globalUnlock = kernel32.NewProc("GlobalUnlock")

	// 通过 Win32 完成字符串长度探测与拷贝，避免在 Go 侧把 uintptr 转回指针
	lstrlenW      = kernel32.NewProc("lstrlenW")
	lstrcpyW      = kernel32.NewProc("lstrcpyW")
	rtlMoveMemory = kernel32.NewProc("RtlMoveMemory")
)

// openClipboardRetry 打开剪贴板。剪贴板可能被其它进程短暂占用，
// 立即失败会让复制/粘贴静默无效，因此做有限重试（最多约 50ms）。
func openClipboardRetry() bool {
	for i := 0; i < 5; i++ {
		if ret, _, _ := openClipboard.Call(0); ret != 0 {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// GetText 读取系统剪贴板中的纯文本
func GetText() string {
	if !openClipboardRetry() {
		return ""
	}
	defer closeClipboard.Call()

	hData, _, _ := getClipboardData.Call(CF_UNICODETEXT)
	if hData == 0 {
		return ""
	}

	ptr, _, _ := globalLock.Call(hData)
	if ptr == 0 {
		return ""
	}
	defer globalUnlock.Call(hData)

	// 先用 lstrlenW 求长度，再由 lstrcpyW 复制到 Go 缓冲区：
	// 全程只把 Go 指针转成 uintptr（安全方向），不把 uintptr 转回指针。
	n, _, _ := lstrlenW.Call(ptr)
	length := int32(n)
	if length <= 0 {
		return ""
	}
	buf := make([]uint16, length+1)
	lstrcpyW.Call(uintptr(unsafe.Pointer(&buf[0])), ptr)
	return syscall.UTF16ToString(buf[:length])
}

// SetText 向系统剪贴板写入纯文本
func SetText(text string) bool {
	if !openClipboardRetry() {
		return false
	}
	defer closeClipboard.Call()

	emptyClipboard.Call()

	utf16, err := syscall.UTF16FromString(text)
	if err != nil {
		return false
	}
	size := len(utf16) * 2
	hMem, _, _ := globalAlloc.Call(GMEM_MOVEABLE, uintptr(size))
	if hMem == 0 {
		return false
	}

	ptr, _, _ := globalLock.Call(hMem)
	if ptr == 0 {
		globalFree.Call(hMem)
		return false
	}

	rtlMoveMemory.Call(ptr, uintptr(unsafe.Pointer(&utf16[0])), uintptr(len(utf16))*2)
	globalUnlock.Call(hMem)

	ret, _, _ := setClipboardData.Call(CF_UNICODETEXT, hMem)
	if ret == 0 {
		// 交接失败时所有权仍在本进程，必须释放，否则泄漏全局内存
		globalFree.Call(hMem)
		return false
	}
	return true
}
