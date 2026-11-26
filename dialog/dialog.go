//go:build windows

// Package dialog 提供基于 Windows 原生 comdlg32 与 user32 的纯 Go 零 CGO 文件与消息对话框
package dialog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var (
	comdlg32 = syscall.NewLazyDLL("comdlg32.dll")
	user32   = syscall.NewLazyDLL("user32.dll")

	procGetOpenFileNameW = comdlg32.NewProc("GetOpenFileNameW")
	procGetSaveFileNameW = comdlg32.NewProc("GetSaveFileNameW")
	procMessageBoxW      = user32.NewProc("MessageBoxW")
)

// 对话框样式与标志常量
const (
	// MessageBox 按钮样式
	MB_OK              = 0x00000000
	MB_OKCANCEL        = 0x00000001
	MB_YESNOCANCEL     = 0x00000003
	MB_YESNO           = 0x00000004
	MB_ICONHAND        = 0x00000010
	MB_ICONQUESTION    = 0x00000020
	MB_ICONEXCLAMATION = 0x00000030
	MB_ICONASTERISK    = 0x00000040
	MB_ICONWARNING     = MB_ICONEXCLAMATION
	MB_ICONERROR       = MB_ICONHAND
	MB_ICONINFORMATION = MB_ICONASTERISK

	MB_DEFBUTTON2 = 0x00000100

	// MessageBox 返回值
	IDOK     = 1
	IDCANCEL = 2
	IDABORT  = 3
	IDRETRY  = 4
	IDIGNORE = 5
	IDYES    = 6
	IDNO     = 7

	// 文件对话框 Flags
	OFN_OVERWRITEPROMPT = 0x00000002
	OFN_FILEMUSTEXIST   = 0x00001000
	OFN_PATHMUSTEXIST   = 0x00000800
	OFN_NOCHANGEDIR     = 0x00000008
	OFN_EXPLORER        = 0x00080000
)

// openFileNameW 对应 Win32 OPENFILENAMEW 结构体 (64位对齐)
type openFileNameW struct {
	structSize    uint32
	_             uint32 // 4字节填充保持8字节对齐
	hwndOwner     uintptr
	hInstance     uintptr
	filter        *uint16
	customFilter  *uint16
	maxCustFilter uint32
	filterIndex   uint32
	file          *uint16
	maxFile       uint32
	_             uint32 // 4字节填充
	fileTitle     *uint16
	maxFileTitle  uint32
	_             uint32 // 4字节填充
	initialDir    *uint16
	title         *uint16
	flags         uint32
	fileOffset    uint16
	fileExtension uint16
	defExt        *uint16
	custData      uintptr
	fnHook        uintptr
	templateName  *uint16
	pvReserved    uintptr
	dwReserved    uint32
	flagsEx       uint32
}

// buildFilter 生成 Win32 双重 Null 结尾的 Filter UTF-16 缓冲区
// 支持 "文本文件 (*.txt)|*.txt|所有文件 (*.*)|*.*" 或以 "|" 分隔的字符串
func buildFilter(filterStr string) []uint16 {
	if filterStr == "" {
		filterStr = "所有文件 (*.*)|*.*"
	}
	parts := strings.Split(filterStr, "|")
	var utf16Buf []uint16
	for _, p := range parts {
		u, _ := syscall.UTF16FromString(p) // 已自带 1 个 null 结尾
		utf16Buf = append(utf16Buf, u...)
	}
	// 结尾再追加一个 null
	utf16Buf = append(utf16Buf, 0)
	return utf16Buf
}

// maxPathBufferUTF16 覆盖 Windows 长路径上限（32767 个 UTF-16 码元，含结尾 NUL）。
const maxPathBufferUTF16 = 32768

// copyUTF16Name 把默认文件名写入对话框缓冲区：始终 NUL 结尾；
// 超过缓冲区时按码元截断，且不会在末尾留下孤立的高位代理。
func copyUTF16Name(dst []uint16, name string) {
	if name == "" || len(dst) == 0 {
		return
	}
	u, err := syscall.UTF16FromString(name)
	if err != nil {
		return
	}
	if len(u) > len(dst) {
		u = u[:len(dst)-1]
		for len(u) > 0 && u[len(u)-1] >= 0xD800 && u[len(u)-1] <= 0xDBFF {
			u = u[:len(u)-1]
		}
		u = append(u, 0)
	}
	copy(dst, u)
}

// OpenFileDialog 弹出系统原生“打开文件”对话框
// filter 格式如: "文本文件 (*.txt)|*.txt|所有文件 (*.*)|*.*"
// 返回用户选中的绝对文件路径及是否确认选择
func OpenFileDialog(hwnd uintptr, title, filter, defaultExt string) (string, bool) {
	buf := make([]uint16, maxPathBufferUTF16)
	filterBuf := buildFilter(filter)

	var titlePtr *uint16
	if title != "" {
		titlePtr, _ = syscall.UTF16PtrFromString(title)
	}

	var defExtPtr *uint16
	if defaultExt != "" {
		defExtPtr, _ = syscall.UTF16PtrFromString(strings.TrimPrefix(defaultExt, "."))
	}

	var ofn openFileNameW
	ofn.structSize = uint32(unsafe.Sizeof(ofn))
	ofn.hwndOwner = hwnd
	ofn.filter = &filterBuf[0]
	ofn.file = &buf[0]
	ofn.maxFile = uint32(len(buf))
	ofn.title = titlePtr
	ofn.defExt = defExtPtr
	ofn.filterIndex = 1 // 默认选中第一个过滤器
	ofn.flags = OFN_FILEMUSTEXIST | OFN_PATHMUSTEXIST | OFN_EXPLORER | OFN_NOCHANGEDIR

	ret, _, _ := procGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if ret == 0 {
		return "", false
	}

	return syscall.UTF16ToString(buf), true
}

// WithFilterExtension 给没有扩展名的文件名补上第 index 个过滤器（从 1 起）的首个扩展名，
// 例如选了 “Go 源码文件 (*.go)” 又只输入 main 时得到 main.go。
// 过滤器没有 “*.扩展名” 形式的模式（*.*、Makefile）或文件名已有扩展名时原样返回。
func WithFilterExtension(path, filter string, index int) string {
	ext := ""
	if parts := strings.Split(filter, "|"); index >= 1 && index*2-1 < len(parts) {
		for _, pattern := range strings.Split(parts[index*2-1], ";") {
			if p := strings.TrimSpace(pattern); strings.HasPrefix(p, "*.") && !strings.ContainsAny(p[2:], "*?") {
				ext = strings.ToLower(p[1:])
				break
			}
		}
	}
	if ext == "" || (filepath.Ext(path) != "" && filepath.Ext(path) != ".") {
		return path
	}
	// 结尾单独一个点会被 Win32 丢掉，不算扩展名
	return strings.TrimRight(path, ".") + ext
}

// SaveFileDialog 弹出系统原生“另存为”对话框
// filter 格式如: "文本文件 (*.txt)|*.txt|所有文件 (*.*)|*.*"
// defaultName 默认文件名，如 "未命名.txt"
// defaultExt 非空时照旧作为固定默认扩展名；为空时按用户选中的过滤器补全扩展名
// （对话框的 lpstrDefExt 是静态的，固定传 "txt" 会让选了 “Go 源码文件” 再输入 main 的用户得到 main.txt）。
// 返回用户选中的保存绝对路径及是否确认
func SaveFileDialog(hwnd uintptr, title, filter, defaultExt, defaultName string) (string, bool) {
	buf := make([]uint16, maxPathBufferUTF16)
	if defaultName != "" {
		copyUTF16Name(buf, defaultName)
	}
	filterBuf := buildFilter(filter)

	var titlePtr *uint16
	if title != "" {
		titlePtr, _ = syscall.UTF16PtrFromString(title)
	}

	var defExtPtr *uint16
	if defaultExt != "" {
		defExtPtr, _ = syscall.UTF16PtrFromString(strings.TrimPrefix(defaultExt, "."))
	}

	var ofn openFileNameW
	ofn.structSize = uint32(unsafe.Sizeof(ofn))
	ofn.hwndOwner = hwnd
	ofn.filter = &filterBuf[0]
	ofn.file = &buf[0]
	ofn.maxFile = uint32(len(buf))
	ofn.title = titlePtr
	ofn.defExt = defExtPtr
	ofn.filterIndex = 1 // 默认选中第一个过滤器
	ofn.flags = OFN_OVERWRITEPROMPT | OFN_PATHMUSTEXIST | OFN_EXPLORER | OFN_NOCHANGEDIR

	for {
		ret, _, _ := procGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
		if ret == 0 {
			return "", false
		}
		path := syscall.UTF16ToString(buf)
		if defExtPtr != nil {
			return path, true
		}
		// 对话框自带的覆盖确认只针对用户输入的名字；补全扩展名后的文件若已存在要自己问一次。
		// 用户选择不替换时，带着刚输入的名字回到对话框重新选择。
		final := WithFilterExtension(path, filter, int(ofn.filterIndex))
		if final != path {
			if _, err := os.Stat(final); err == nil {
				msg := fmt.Sprintf("%s 已存在。\n是否要替换它？", filepath.Base(final))
				if MessageBox(hwnd, "确认另存为", msg, MB_YESNO|MB_ICONWARNING|MB_DEFBUTTON2) != IDYES {
					continue
				}
			}
		}
		return final, true
	}
}

// MessageBox 弹出系统原生消息提示框
func MessageBox(hwnd uintptr, title, text string, style uint32) int {
	textPtr, _ := syscall.UTF16PtrFromString(text)
	titlePtr, _ := syscall.UTF16PtrFromString(title)

	ret, _, _ := procMessageBoxW.Call(
		hwnd,
		uintptr(unsafe.Pointer(textPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(style),
	)
	return int(ret)
}

// ShowInfo 快捷弹出信息提示框
func ShowInfo(hwnd uintptr, title, text string) {
	MessageBox(hwnd, title, text, MB_OK|MB_ICONINFORMATION)
}

// ShowWarning 快捷弹出警告提示框
func ShowWarning(hwnd uintptr, title, text string) {
	MessageBox(hwnd, title, text, MB_OK|MB_ICONWARNING)
}

// ShowError 快捷弹出错误提示框
func ShowError(hwnd uintptr, title, text string) {
	MessageBox(hwnd, title, text, MB_OK|MB_ICONERROR)
}

// ShowConfirmYesNoCancel 快捷弹出“是否保存”三态确认框 (IDYES, IDNO, IDCANCEL)
func ShowConfirmYesNoCancel(hwnd uintptr, title, text string) int {
	return MessageBox(hwnd, title, text, MB_YESNOCANCEL|MB_ICONQUESTION)
}
