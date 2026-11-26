//go:build windows

package main

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	goui "github.com/jacksalad/gnote"
)

//go:embed font.ttf
var embeddedFontData []byte

func main() {
	// 0. 内嵌字体注册（6.5MB / 9431 码位）与设置读取、图标生成、窗口创建互不依赖：
	// 放到 goroutine 并行执行，只在构建 UI 之前汇合，隐藏约 10–30ms 的字体装载耗时。
	appFontFamily := "Microsoft YaHei"
	fontReady := make(chan string, 1)
	go func() {
		if len(embeddedFontData) == 0 {
			fontReady <- ""
			return
		}
		fam, err := goui.RegisterMemoryFont(embeddedFontData)
		if err != nil {
			fam = ""
		}
		fontReady <- fam
	}()

	// 1. 读取用户偏好并先应用主题，保证窗口创建时就是正确的背景色
	settings, settingsLoadErr := LoadSettings()
	if th := goui.ThemeByID(settings.Theme); th != nil {
		goui.SetTheme(th)
	} else {
		goui.SetTheme(goui.ThemeDark())
	}

	// 2. 创建主窗口 (恢复上次窗口尺寸，屏幕居中显示，挂载专属现代极客 Logo)
	gnoteIcon := GetGnoteIcon()
	win, err := goui.NewWindow(goui.WindowConfig{
		Title:     "未命名 - gnote",
		Width:     settings.WindowWidth,
		Height:    settings.WindowHeight,
		Center:    true,
		Icon:      gnoteIcon,
		Resizable: true,
		Visible:   false,
	})
	if err != nil {
		fmt.Printf("创建窗口失败: %v\n", err)
		return
	}

	// 3. 汇合并行字体注册，构建应用状态、界面与命令，进入主消息循环
	if fam := <-fontReady; fam != "" {
		appFontFamily = fam
		goui.SetDefaultFont(appFontFamily, 16)
	}
	app := NewApp(win, appFontFamily, settings)
	// 配置文件存在却读不出来：本次会话运行在默认值上，退出时不得用默认值覆盖它。
	app.settingsLoadFailed = settingsLoadErr != nil
	app.Build()
	app.Start(parseTargetFileFromArgs(os.Args[1:]), welcomeDoc)
}

// welcomeDoc 首次启动展示的欢迎文档与快捷键指引。
const welcomeDoc = "# 欢迎使用 gnote！\r\n\r\n" +
	"> 纯 Go 零 CGO 软件光栅化双缓冲引擎，具备毫秒级冷启动与原生级流畅度。\r\n\r\n" +
	"---\r\n\r\n" +
	"## 核心技术特性\r\n" +
	"- **零 CGO 依赖**：纯 Windows API 动态绑定，轻量级极速编译\r\n" +
	"- **物理像素抗锯齿**：GDI 矢量轮廓 1:1 线性灰度光栅化，高分屏清晰锐利\r\n" +
	"- **智能自动折行**：超长单行文本自动平滑换行显示，严谨对齐且不超出窗口右侧\r\n" +
	"- **内置多语言高亮**：纯 Go 词法分词状态机，原生支持 Go、Python、C/C++、C#、Java、Rust、R、JavaScript、TypeScript、HTML、CSS、SQL、Markdown、JSON、YAML、TOML 语法高亮\r\n" +
	"- **无级平滑缩放**：支持 `Ctrl + 滚轮` 与 `Ctrl + +/-/0` 实时缩放\r\n" +
	"- **行级视口虚拟化**：万行超大文档丝滑滚动\r\n" +
	"- **JSON / HTML 格式化**：`Ctrl+Shift+F` 缩进排版，保持 JSON 键序与数字字面量，HTML 内联 CSS/JS 同步格式化，保留预格式化内容与模板字符串\r\n" +
	"- **Windows 原生剪贴板**：完整支持撤销 (Ctrl+Z) 与重做 (Ctrl+Y)\r\n\r\n" +
	"## 多语言支持速览\r\n" +
	"- `.go`               : Go 关键字、内置类型、函数声明/调用、多行块注释、反引号原生文本\r\n" +
	"- `.py` / `.pyw`      : Python 关键字、内置函数/类型、三引号文档注释、前缀字符串、装饰器\r\n" +
	"- `.c` / `.cpp` / `.h`: C/C++ 预处理指令、类/命名空间、STL 类型、函数调用、多行块注释、指针操作符\r\n" +
	"- `.cs` / `.csx`      : C# 核心关键字、LINQ 查询、Task 异步、逐字跨行字符串、预处理器、属性标记\r\n" +
	"- `.java`             : Java 关键字、核心类库、注解、Javadoc/多行注释、多行文本块、Lambda\r\n" +
	"- `.rs`               : Rust 所有权生命周期、宏调用(println!/vec!)、属性注解、原生字符串、类型后缀\r\n" +
	"- `.r` / `.R`         : R 语言特殊赋值(<-)、管道符(%>%/|>)、二元操作符(%in%)、点号函数、数据分析函数\r\n" +
	"- `.js` / `.jsx`      : JavaScript 现代语法、异步函数、模板字符串、箭头函数、内置对象\r\n" +
	"- `.ts` / `.tsx`      : TypeScript / TypoScript 接口、类型别名、泛型、装饰器、类型断言\r\n" +
	"- `.html` / `.htm`    : HTML 标签配对、属性键值、<!DOCTYPE>、注释与实体字符\r\n" +
	"- `.css`              : CSS 选择器、属性与单位、@media 规则块、十六进制色值与 !important\r\n" +
	"- `.sql`              : SQL 标准查询、DDL/DML、内置聚合函数、数据类型、单双引号与注释\r\n" +
	"- `.md` / `.markdown` : Markdown 标题、代码块、列表、粗体、链接\r\n" +
	"- `.json`             : JSON 键名、字符串、数值、布尔值、转义字符、结构括号\r\n" +
	"- `.yaml` / `.yml`    : YAML 映射键、列表项、锚点、别名、多行块标量、注释\r\n" +
	"- `.toml`             : TOML 表头 [table]、键名、时间戳、多行字符串、数字\r\n\r\n" +
	"## 快捷键速查清单\r\n" +
	"- [x] `Ctrl + N`       : 新建文档\r\n" +
	"- [x] `Ctrl + O`       : 打开文件\r\n" +
	"- [x] `Ctrl + S`       : 保存文档\r\n" +
	"- [x] `Ctrl + Shift+S` : 另存为文档\r\n" +
	"- [x] `Ctrl + F / H`   : 查找 / 替换（含大小写开关与全部替换）\r\n" +
	"- [x] `Ctrl + Shift+F` : 格式化 JSON / HTML 文档\r\n" +
	"- [x] `Ctrl + A`       : 全选内容\r\n" +
	"- [x] `Ctrl + Z / Y`   : 撤销 / 重做（连续输入自动合组）\r\n" +
	"- [x] `双击 / 三击`    : 选中单词 / 选中整行\r\n" +
	"- [x] `右键`           : 撤销、剪切、复制、粘贴、全选、查找替换、格式化\r\n" +
	"- [x] `Ctrl + 滚轮`     : 实时放大 / 缩小字体\r\n" +
	"- [x] `Ctrl + +/-/0`   : 放大 / 缩小 / 恢复 100%\r\n" +
	"- [x] `Alt + F / F10` : 打开顶部菜单\r\n" +
	"- [x] `Alt + F4`       : 退出\r\n" +
	"- [x] `Ctrl + ,`       : 打开设置窗口\r\n" +
	"- [x] `状态栏右侧`     : 点击切换编码 / EOL / 语言 / 缩放\r\n\r\n" +
	"开源项目仓库: [gnote on GitHub](https://github.com/jacksalad/gnote)\r\n"

// parseTargetFileFromArgs 从命令行参数中提取待打开的目标文件路径（用于支持 Windows“打开方式”、快捷方式传参以及命令行调用）
func parseTargetFileFromArgs(args []string) string {
	for _, arg := range args {
		trimmed := strings.Trim(strings.TrimSpace(arg), "\"")
		if trimmed != "" && !strings.HasPrefix(trimmed, "-") && !strings.HasPrefix(trimmed, "/") {
			return trimmed
		}
	}
	return ""
}

// maxOpenFileSize 单文件打开上限：超过该尺寸的文本会显著拖慢 UI 线程并占用大量内存
const maxOpenFileSize = 64 << 20

// 保存文件时用到的 Win32 接口（kernel32 / advapi32）。
var (
	kernel32Save                 = syscall.NewLazyDLL("kernel32.dll")
	advapi32Save                 = syscall.NewLazyDLL("advapi32.dll")
	setFileInformationByHandle   = kernel32Save.NewProc("SetFileInformationByHandle")
	getFinalPathNameByHandleW    = kernel32Save.NewProc("GetFinalPathNameByHandleW")
	deviceIoControl              = kernel32Save.NewProc("DeviceIoControl")
	findFirstStreamW             = kernel32Save.NewProc("FindFirstStreamW")
	findNextStreamW              = kernel32Save.NewProc("FindNextStreamW")
	localFree                    = kernel32Save.NewProc("LocalFree")
	encryptFileW                 = advapi32Save.NewProc("EncryptFileW") // EFS 接口在 advapi32，不在 kernel32
	decryptFileW                 = advapi32Save.NewProc("DecryptFileW")
	getNamedSecurityInfoW        = advapi32Save.NewProc("GetNamedSecurityInfoW")
	setNamedSecurityInfoW        = advapi32Save.NewProc("SetNamedSecurityInfoW")
	getSecurityDescriptorControl = advapi32Save.NewProc("GetSecurityDescriptorControl")
)

// winnt.h 里的常量。
const (
	fileAttrReadonly   = 0x1    // FILE_ATTRIBUTE_READONLY
	fileAttrHidden     = 0x2    // FILE_ATTRIBUTE_HIDDEN
	fileAttrSystem     = 0x4    // FILE_ATTRIBUTE_SYSTEM
	fileAttrArchive    = 0x20   // FILE_ATTRIBUTE_ARCHIVE
	fileAttrNormal     = 0x80   // FILE_ATTRIBUTE_NORMAL
	fileAttrCompressed = 0x800  // FILE_ATTRIBUTE_COMPRESSED
	fileAttrEncrypted  = 0x4000 // FILE_ATTRIBUTE_ENCRYPTED

	fileReadAttributes = 0x80 // FILE_READ_ATTRIBUTES

	fileBasicInfoClass       = 0       // FileBasicInfo
	fsctlSetCompression      = 0x9C040 // FSCTL_SET_COMPRESSION
	compressionFormatNone    = 0
	compressionFormatDefault = 1
	findStreamInfoStandard   = 0 // FindStreamInfoStandard

	seFileObject                       = 1 // SE_OBJECT_TYPE::SE_FILE_OBJECT
	daclSecurityInformation            = 0x4
	unprotectedDaclSecurityInformation = 0x20000000
	protectedDaclSecurityInformation   = 0x80000000
	seDaclPresent                      = 0x4
	seDaclProtected                    = 0x1000
	inheritedAce                       = 0x10

	// Win32 错误码（syscall 包没有导出这三个）
	errSharingViolation = syscall.Errno(32)   // ERROR_SHARING_VIOLATION
	errLockViolation    = syscall.Errno(33)   // ERROR_LOCK_VIOLATION
	errUserMappedFile   = syscall.Errno(1224) // ERROR_USER_MAPPED_FILE

	// 替换目标文件时的重试：占用类错误通常一瞬间就放开，间隔 20/40/80/160ms 共试 5 次。
	saveLockRetries = 5
	saveLockStep    = 20 * time.Millisecond
)

// fileBasicInfo 对应 FILE_BASIC_INFO：四个时间（0 表示不改）与文件属性。
type fileBasicInfo struct {
	creationTime, lastAccessTime, lastWriteTime, changeTime int64
	fileAttributes                                          uint32
	_                                                       uint32
}

// findStreamData 对应 WIN32_FIND_STREAM_DATA。
type findStreamData struct {
	streamSize int64
	streamName [296]uint16 // MAX_PATH + 36
}

// saveTarget 保存目标的解析结果：符号链接已解析到最终路径，其余是原文件的元数据（新文件时 exists 为 false）。
type saveTarget struct {
	path       string
	exists     bool
	attrs      uint32
	created    int64 // 创建时间，FILETIME 的 100ns 计数
	links      uint32
	encrypted  bool
	compressed bool
}

// writeFileAtomic 先写同目录临时文件并落盘，再原子替换目标文件。
// 避免写入中途崩溃/断电/磁盘写满导致原文件被截断。
//
// 原子替换是用新文件顶掉目录项，挂在原文件记录上的元数据会跟着目录项一起消失。
// 所以替换之前把它们搬到临时文件上：隐藏/系统属性、创建时间、显式设置的 DACL、
// EFS 加密、NTFS 压缩和备用数据流。符号链接先解析到最终目标；有多个硬链接的文件
// 改为原地覆写，保证所有名字继续指向同一份内容。
func writeFileAtomic(path string, data []byte) error {
	target, err := inspectSaveTarget(path)
	if err != nil {
		return err
	}
	if target.attrs&fileAttrReadonly != 0 {
		return fmt.Errorf("目标文件是只读文件，无法保存:\n%s\n请去掉只读属性，或用 “另存为” 换一个文件名。", target.path)
	}
	if target.links > 1 {
		return writeFileInPlace(target.path, data)
	}
	return writeFileReplace(target, data)
}

// inspectSaveTarget 打开保存目标（跟随符号链接）并读取要搬到新文件上的元数据。
func inspectSaveTarget(path string) (saveTarget, error) {
	target := saveTarget{path: path}
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return target, err
	}
	// 只要 FILE_READ_ATTRIBUTES：不触发云端占位文件下载，也不与别的进程读写冲突。
	h, err := syscall.CreateFile(p, fileReadAttributes,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		if fi, statErr := os.Stat(path); statErr == nil && fi.IsDir() {
			return target, fmt.Errorf("目标路径是一个文件夹，无法保存:\n%s", path)
		}
		// 目标不存在按新文件处理；其它打开失败（没有读属性的权限等）也走同一条路，
		// 只是搬不了元数据，行为与加入元数据搬运之前一致。
		return danglingLinkTarget(path), nil
	}
	defer syscall.CloseHandle(h)

	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(h, &info); err != nil {
		return target, nil
	}
	target.exists = true
	target.attrs = info.FileAttributes
	target.created = filetimeToInt64(info.CreationTime)
	target.links = info.NumberOfLinks
	target.encrypted = info.FileAttributes&fileAttrEncrypted != 0
	target.compressed = info.FileAttributes&fileAttrCompressed != 0
	if final := finalPathName(h); final != "" {
		target.path = final
	}
	return target, nil
}

// danglingLinkTarget 目标不存在时按新文件处理。路径本身若是悬空的符号链接，
// 要在它指向的位置创建文件，而不是用普通文件顶掉链接。
func danglingLinkTarget(path string) saveTarget {
	target := saveTarget{path: path}
	fi, err := os.Lstat(path)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		return target
	}
	dest, err := os.Readlink(path)
	if err != nil {
		return target
	}
	if !filepath.IsAbs(dest) {
		dest = filepath.Join(filepath.Dir(path), dest)
	}
	target.path = dest
	return target
}

// finalPathName 经句柄取文件的最终路径（符号链接、目录联接都已解析），并还原成普通 DOS 路径。
func finalPathName(h syscall.Handle) string {
	buf := make([]uint16, 260)
	for {
		n, _, _ := getFinalPathNameByHandleW.Call(uintptr(h),
			uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0)
		if n == 0 {
			return ""
		}
		if int(n) < len(buf) {
			name := syscall.UTF16ToString(buf[:n])
			if strings.HasPrefix(name, `\\?\UNC\`) {
				return `\\` + strings.TrimPrefix(name, `\\?\UNC\`)
			}
			return strings.TrimPrefix(name, `\\?\`)
		}
		buf = make([]uint16, n)
	}
}

func filetimeToInt64(ft syscall.Filetime) int64 {
	return int64(uint32(ft.HighDateTime))<<32 | int64(uint32(ft.LowDateTime))
}

// writeFileReplace 写同目录临时文件并落盘，把原文件的元数据搬到临时文件上，最后原子替换。
// 替换之前任何一步失败都会删掉临时文件，原文件保持不变。
func writeFileReplace(target saveTarget, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(target.path), ".gnote-*.tmp")
	if err != nil {
		return saveError("无法创建临时文件", target.path, err, "")
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = tmp.Close()
			discardTempFile(tmpName)
		}
	}()

	if target.exists {
		if tmp, err = matchStorage(tmp, tmpName, target); err != nil {
			return err
		}
	}
	if _, err := tmp.Write(data); err != nil {
		return saveError("无法写入文件", target.path, err, tmpName)
	}
	if err := tmp.Sync(); err != nil {
		return saveError("无法写入文件", target.path, err, tmpName)
	}
	if target.exists {
		if err := applyBasicInfo(tmp, target); err != nil {
			return saveError("无法保留原文件的属性", target.path, err, tmpName)
		}
	}
	if err := tmp.Close(); err != nil {
		return saveError("无法写入文件", target.path, err, tmpName)
	}

	if target.exists {
		copyStreams(target.path, tmpName)
		if err := copyDacl(target.path, tmpName); err != nil {
			return err
		}
	}
	if err := retryOnLock(func() error { return os.Rename(tmpName, target.path) }); err != nil {
		return saveError("无法替换目标文件", target.path, err, tmpName)
	}
	cleanup = false
	return nil
}

// matchStorage 让临时文件的加密与压缩状态与原文件一致，返回之后用来写内容的句柄。
// 必须在写入内容之前调用：EFS 文件的明文不能先落盘，压缩也要从第一个字节起生效。
func matchStorage(tmp *os.File, tmpName string, target saveTarget) (*os.File, error) {
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(tmp.Fd()), &info); err != nil {
		return tmp, nil
	}
	if (info.FileAttributes&fileAttrEncrypted != 0) != target.encrypted {
		// EncryptFile / DecryptFile 要独占打开文件：先关掉 CreateTemp 的句柄，设置后再重新打开。
		_ = tmp.Close()
		if ok := setFileEncryption(tmpName, target.encrypted); !ok && target.encrypted {
			// 原文件是加密的：临时文件加密失败就绝不能往里写明文。
			return nil, fmt.Errorf("原文件使用了 EFS 加密，但无法加密临时文件；为避免明文落盘已取消保存:\n%s", filepath.Base(target.path))
		}
		return os.OpenFile(tmpName, os.O_WRONLY, 0)
	}
	if (info.FileAttributes&fileAttrCompressed != 0) != target.compressed {
		setCompression(syscall.Handle(tmp.Fd()), target.compressed)
	}
	return tmp, nil
}

// setFileEncryption 用 EncryptFileW / DecryptFileW 打开或关闭单个文件的 EFS 加密，返回是否成功。
func setFileEncryption(path string, encrypt bool) bool {
	setter := decryptFileW
	if encrypt {
		setter = encryptFileW
	}
	if setter.Find() != nil {
		return false
	}
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	ret, _, _ := setter.Call(uintptr(unsafe.Pointer(p)))
	return ret != 0
}

// setCompression 用 FSCTL_SET_COMPRESSION 打开或关闭单个文件的 NTFS 压缩；卷不支持时忽略。
// 输入参数是 USHORT；句柄必须同时具有读写权限（CreateTemp 返回的句柄满足），只有写权限会被拒绝。
func setCompression(h syscall.Handle, compressed bool) {
	format := uint16(compressionFormatNone)
	if compressed {
		format = compressionFormatDefault
	}
	var returned uint32
	_, _, _ = deviceIoControl.Call(uintptr(h), fsctlSetCompression,
		uintptr(unsafe.Pointer(&format)), unsafe.Sizeof(format), 0, 0,
		uintptr(unsafe.Pointer(&returned)), 0)
}

// applyBasicInfo 把创建时间与隐藏 / 系统属性搬到新文件上。只读属性不搬（只读目标在入口处已拒绝）；
// 归档位保留：文件内容刚变过，备份软件靠它识别改动过的文件。
func applyBasicInfo(f *os.File, target saveTarget) error {
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(f.Fd()), &info); err != nil {
		return err
	}
	basic := fileBasicInfo{creationTime: target.created}
	attrs := info.FileAttributes | fileAttrArchive | target.attrs&(fileAttrHidden|fileAttrSystem)
	basic.fileAttributes = attrs &^ fileAttrNormal // NORMAL 只能单独使用
	ret, _, err := setFileInformationByHandle.Call(f.Fd(), fileBasicInfoClass,
		uintptr(unsafe.Pointer(&basic)), unsafe.Sizeof(basic))
	if ret == 0 {
		return err
	}
	return nil
}

// copyStreams 把原文件的备用数据流（如 Zone.Identifier）拷到新文件上。
// 尽力而为：某个流读不了不影响保存，只是那一个流会丢失。
func copyStreams(from, to string) {
	fromPtr, err := syscall.UTF16PtrFromString(from)
	if err != nil {
		return
	}
	var data findStreamData
	h, _, _ := findFirstStreamW.Call(uintptr(unsafe.Pointer(fromPtr)), findStreamInfoStandard,
		uintptr(unsafe.Pointer(&data)), 0)
	if syscall.Handle(h) == syscall.InvalidHandle { // INVALID_HANDLE_VALUE：卷不支持或者没有流
		return
	}
	defer syscall.FindClose(syscall.Handle(h))
	for {
		if name := syscall.UTF16ToString(data.streamName[:]); name != "" && !strings.EqualFold(name, "::$DATA") {
			copyStream(from+name, to+name)
		}
		if ret, _, _ := findNextStreamW.Call(h, uintptr(unsafe.Pointer(&data))); ret == 0 {
			return
		}
	}
}

func copyStream(from, to string) {
	in, err := os.Open(from)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return
	}
	_ = out.Close()
}

// copyDacl 原文件的 DACL 是用户显式设置过的时候（禁止继承，或含非继承的 ACE），把它原样搬到新文件上。
// 否则新文件从同一个目录继承来的权限就是正确的，不必动。
func copyDacl(from, to string) error {
	fromPtr, err := syscall.UTF16PtrFromString(from)
	if err != nil {
		return nil
	}
	toPtr, err := syscall.UTF16PtrFromString(to)
	if err != nil {
		return nil
	}
	// 指针一律用 unsafe.Pointer 变量承接，而不是把 uintptr 转回指针。
	var sd unsafe.Pointer
	var dacl *byte
	ret, _, _ := getNamedSecurityInfoW.Call(uintptr(unsafe.Pointer(fromPtr)), seFileObject, daclSecurityInformation,
		0, 0, uintptr(unsafe.Pointer(&dacl)), 0, uintptr(unsafe.Pointer(&sd)))
	if ret != 0 {
		// 读不到权限信息：退回让新文件继承目录权限（与加入 DACL 搬运之前的行为一致）。
		return nil
	}
	defer localFree.Call(uintptr(sd))

	var control uint16
	var revision uint32
	if r, _, _ := getSecurityDescriptorControl.Call(uintptr(sd),
		uintptr(unsafe.Pointer(&control)), uintptr(unsafe.Pointer(&revision))); r == 0 {
		return nil
	}
	protected := control&seDaclProtected != 0
	if control&seDaclPresent == 0 || dacl == nil || (!protected && !aclHasExplicitAce(dacl)) {
		return nil
	}
	securityInfo := uintptr(daclSecurityInformation | unprotectedDaclSecurityInformation)
	if protected {
		securityInfo = uintptr(daclSecurityInformation | protectedDaclSecurityInformation)
	}
	if ret, _, err := setNamedSecurityInfoW.Call(uintptr(unsafe.Pointer(toPtr)), seFileObject, securityInfo,
		0, 0, uintptr(unsafe.Pointer(dacl)), 0); ret != 0 {
		return fmt.Errorf("无法保留原文件的安全权限，已取消保存:\n%v", err)
	}
	return nil
}

// aclHasExplicitAce 判断 ACL 里有没有非继承来的 ACE（ACL / ACE 头部布局见 winnt.h）。
func aclHasExplicitAce(dacl *byte) bool {
	hdr := (*[8]byte)(unsafe.Pointer(dacl))
	count := int(hdr[4]) | int(hdr[5])<<8
	off := 8
	for i := 0; i < count; i++ {
		ace := (*[4]byte)(unsafe.Add(unsafe.Pointer(dacl), off))
		if ace[1]&inheritedAce == 0 {
			return true
		}
		off += int(ace[2]) | int(ace[3])<<8
	}
	return false
}

// writeFileInPlace 原地覆写，供有多个硬链接的文件使用：原子替换会把其它名字留在旧内容上，
// 而这些名字必须继续指向同一份文件。为尽量保住崩溃安全，先把新内容从头写满并落盘，
// 最后才截断到新长度——中途崩溃时文件不会比旧内容更短，开头也总是新内容。
func writeFileInPlace(target string, data []byte) error {
	var f *os.File
	if err := retryOnLock(func() error {
		var err error
		f, err = os.OpenFile(target, os.O_WRONLY, 0)
		return err
	}); err != nil {
		return saveError("无法写入文件", target, err, "")
	}
	fail := func(err error) error {
		_ = f.Close()
		return saveError("无法写入文件", target, err, "")
	}
	if _, err := f.Write(data); err != nil {
		return fail(err)
	}
	if err := f.Sync(); err != nil {
		return fail(err)
	}
	if err := f.Truncate(int64(len(data))); err != nil {
		return fail(err)
	}
	if err := f.Close(); err != nil {
		return saveError("无法写入文件", target, err, "")
	}
	return nil
}

// retryOnLock 遇到占用类错误（别的程序暂时没共享删除、杀毒软件正扫描刚写完的文件）时
// 间隔 20/40/80/160ms 重试，共 5 次；其它错误立即返回。
func retryOnLock(fn func() error) error {
	var err error
	for attempt := 0; attempt < saveLockRetries; attempt++ {
		if err = fn(); err == nil {
			return nil
		}
		if !isLockError(err) || attempt == saveLockRetries-1 {
			return err
		}
		time.Sleep(time.Duration(uintptr(1)<<uint(attempt)) * saveLockStep)
	}
	return err
}

// isLockError 判断是不是 “文件被别的程序占用 / 拒绝访问” 这类通常很快自行恢复的错误。
func isLockError(err error) bool {
	return errors.Is(err, errSharingViolation) || errors.Is(err, syscall.ERROR_ACCESS_DENIED) ||
		errors.Is(err, errLockViolation) || errors.Is(err, errUserMappedFile)
}

// saveError 把底层错误换成用户看得懂的说明：说清是哪个文件、原文件没有被改动，
// 并且不暴露临时文件的路径。
func saveError(what, target string, err error, tmpName string) error {
	if err == nil {
		return nil
	}
	detail := err.Error()
	if tmpName != "" {
		detail = strings.ReplaceAll(detail, tmpName, "临时文件")
	}
	name := filepath.Base(target)
	if isLockError(err) {
		return fmt.Errorf("%s \"%s\"：它正被其他程序占用，或当前账户没有写入权限。\n原文件未被改动，请关闭占用它的程序后重试。\n(%s)",
			what, name, detail)
	}
	return fmt.Errorf("%s \"%s\":\n%s", what, name, detail)
}

// discardTempFile 删除写了一半的临时文件。它的权限已被收紧到连删除都不允许时，
// 先恢复成从目录继承再删（所有者仍是当前用户，总做得到），保证不留垃圾。
func discardTempFile(tmpName string) {
	if os.Remove(tmpName) == nil {
		return
	}
	p, err := syscall.UTF16PtrFromString(tmpName)
	if err != nil {
		return
	}
	setNamedSecurityInfoW.Call(uintptr(unsafe.Pointer(p)), seFileObject,
		uintptr(daclSecurityInformation|unprotectedDaclSecurityInformation), 0, 0, 0, 0)
	_ = os.Remove(tmpName)
}
