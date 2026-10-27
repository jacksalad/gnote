//go:build windows

package window

import (
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	// 用户32位库
	user32 = syscall.NewLazyDLL("user32.dll")
	// GDI库
	gdi32 = syscall.NewLazyDLL("gdi32.dll")
	// 内核库
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
)

// Win32 API 函数
var (
	// 窗口类和窗口
	registerClassExW              = user32.NewProc("RegisterClassExW")
	unregisterClassW              = user32.NewProc("UnregisterClassW")
	createWindowExW               = user32.NewProc("CreateWindowExW")
	destroyWindow                 = user32.NewProc("DestroyWindow")
	isWindow                      = user32.NewProc("IsWindow")
	defWindowProcW                = user32.NewProc("DefWindowProcW")
	getMessageW                   = user32.NewProc("GetMessageW")
	translateMessage              = user32.NewProc("TranslateMessage")
	dispatchMessageW              = user32.NewProc("DispatchMessageW")
	postQuitMessage               = user32.NewProc("PostQuitMessage")
	postMessageW                  = user32.NewProc("PostMessageW")
	sendMessageW                  = user32.NewProc("SendMessageW")
	peekMessageW                  = user32.NewProc("PeekMessageW")
	waitMessage                   = user32.NewProc("WaitMessage")
	updateWindow                  = user32.NewProc("UpdateWindow")
	invalidateRect                = user32.NewProc("InvalidateRect")
	validateRect                  = user32.NewProc("ValidateRect")
	setWindowTextW                = user32.NewProc("SetWindowTextW")
	getWindowTextW                = user32.NewProc("GetWindowTextW")
	getWindowTextLengthW          = user32.NewProc("GetWindowTextLengthW")
	setWindowPos                  = user32.NewProc("SetWindowPos")
	getClientRect                 = user32.NewProc("GetClientRect")
	getWindowRect                 = user32.NewProc("GetWindowRect")
	showWindow                    = user32.NewProc("ShowWindow")
	isIconic                      = user32.NewProc("IsIconic")
	isWindowVisible               = user32.NewProc("IsWindowVisible")
	enableWindow                  = user32.NewProc("EnableWindow")
	isWindowEnabled               = user32.NewProc("IsWindowEnabled")
	setFocus                      = user32.NewProc("SetFocus")
	getFocus                      = user32.NewProc("GetFocus")
	setForegroundWindow           = user32.NewProc("SetForegroundWindow")
	adjustWindowRectEx            = user32.NewProc("AdjustWindowRectEx")
	screenToClient                = user32.NewProc("ScreenToClient")
	setProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
	setProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	getSystemMetrics              = user32.NewProc("GetSystemMetrics")

	// 图标与位图
	createIconIndirect       = user32.NewProc("CreateIconIndirect")
	createIconFromResourceEx = user32.NewProc("CreateIconFromResourceEx")
	destroyIcon              = user32.NewProc("DestroyIcon")
	createBitmap             = gdi32.NewProc("CreateBitmap")

	// 绘图相关
	beginPaint         = user32.NewProc("BeginPaint")
	endPaint           = user32.NewProc("EndPaint")
	getDC              = user32.NewProc("GetDC")
	releaseDC          = user32.NewProc("ReleaseDC")
	getWindowDC        = user32.NewProc("GetWindowDC")
	createCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	deleteDC           = gdi32.NewProc("DeleteDC")
	selectObject       = gdi32.NewProc("SelectObject")
	deleteObject       = gdi32.NewProc("DeleteObject")
	createDIBSection   = gdi32.NewProc("CreateDIBSection")
	bitBlt             = gdi32.NewProc("BitBlt")
	stretchBlt         = gdi32.NewProc("StretchBlt")
	setDIBitsToDevice  = gdi32.NewProc("SetDIBitsToDevice")
	getStockObject     = gdi32.NewProc("GetStockObject")
	setStretchBltMode  = gdi32.NewProc("SetStretchBltMode")

	// 模块和实例
	getModuleHandleW   = kernel32.NewProc("GetModuleHandleW")
	getCurrentThreadId = kernel32.NewProc("GetCurrentThreadId")
)

// 窗口消息常量
const (
	WM_NULL            = 0x0000
	WM_CREATE          = 0x0001
	WM_DESTROY         = 0x0002
	WM_CLOSE           = 0x0010
	WM_QUERYENDSESSION = 0x0011
	WM_QUIT            = 0x0012
	WM_PAINT           = 0x000F
	WM_APP_REDRAW      = 0x8001 // WM_APP + 1：跨线程请求 UI 线程重绘
	WM_SIZE            = 0x0005
	WM_MOVE            = 0x0003
	WM_ERASEBKGND      = 0x0014
	WM_SHOWWINDOW      = 0x0018
	WM_SETFOCUS        = 0x0007
	WM_KILLFOCUS       = 0x0008
	WM_ENABLE          = 0x000A
	WM_SETREDRAW       = 0x000B
	WM_SETTEXT         = 0x000C
	WM_GETTEXT         = 0x000D
	WM_GETTEXTLENGTH   = 0x000E

	// 键盘消息
	WM_KEYDOWN    = 0x0100
	WM_KEYUP      = 0x0101
	WM_CHAR       = 0x0102
	WM_SYSKEYDOWN = 0x0104
	WM_SYSKEYUP   = 0x0105
	WM_SYSCHAR    = 0x0106
	WM_SYSCOMMAND = 0x0112
	SC_CLOSE      = 0xF060

	// 鼠标消息
	WM_MOUSEMOVE     = 0x0200
	WM_LBUTTONDOWN   = 0x0201
	WM_LBUTTONUP     = 0x0202
	WM_LBUTTONDBLCLK = 0x0203
	WM_RBUTTONDOWN   = 0x0204
	WM_RBUTTONUP     = 0x0205
	WM_RBUTTONDBLCLK = 0x0206
	WM_MBUTTONDOWN   = 0x0207
	WM_MBUTTONUP     = 0x0208
	WM_MBUTTONDBLCLK = 0x0209
	WM_MOUSEWHEEL    = 0x020A
	WM_MOUSEHWHEEL   = 0x020E
	WM_DPICHANGED    = 0x02E0
	WM_GETMINMAXINFO = 0x0024

	// 输入法消息
	WM_IME_SETCONTEXT           = 0x0281
	WM_IME_STARTCOMPOSITION     = 0x010D
	WM_IME_ENDCOMPOSITION       = 0x010E
	WM_IME_COMPOSITION          = 0x010F
	GCS_COMPSTR                 = 0x0008
	GCS_RESULTSTR               = 0x0800
	ISC_SHOWUICOMPOSITIONWINDOW = 0x80000000
	CFS_POINT                   = 0x0002
	CFS_FORCE_POSITION          = 0x0020
	CFS_CANDIDATEPOS            = 0x0040

	// 按键虚拟码
	VK_BACK    = 0x08
	VK_TAB     = 0x09
	VK_RETURN  = 0x0D
	VK_SHIFT   = 0x10
	VK_CONTROL = 0x11
	VK_MENU    = 0x12 // Alt
	VK_ESCAPE  = 0x1B
	VK_SPACE   = 0x20
	VK_LEFT    = 0x25
	VK_UP      = 0x26
	VK_RIGHT   = 0x27
	VK_DOWN    = 0x28
	VK_DELETE  = 0x2E
	VK_F4      = 0x73
	VK_F10     = 0x79

	// 窗口类样式
	CS_VREDRAW         = 0x0001
	CS_HREDRAW         = 0x0002
	CS_DBLCLKS         = 0x0008
	CS_OWNDC           = 0x0020
	CS_CLASSDC         = 0x0040
	CS_PARENTDC        = 0x0080
	CS_NOCLOSE         = 0x0200
	CS_SAVEBITS        = 0x0800
	CS_BYTEALIGNCLIENT = 0x1000
	CS_BYTEALIGNWINDOW = 0x2000
	CS_GLOBALCLASS     = 0x4000

	// 窗口样式
	WS_OVERLAPPED       = 0x00000000
	WS_POPUP            = 0x80000000
	WS_CHILD            = 0x40000000
	WS_MINIMIZE         = 0x20000000
	WS_VISIBLE          = 0x10000000
	WS_DISABLED         = 0x08000000
	WS_CLIPSIBLINGS     = 0x04000000
	WS_CLIPCHILDREN     = 0x02000000
	WS_MAXIMIZE         = 0x01000000
	WS_BORDER           = 0x00800000
	WS_DLGFRAME         = 0x00400000
	WS_VSCROLL          = 0x00200000
	WS_HSCROLL          = 0x00100000
	WS_SYSMENU          = 0x00080000
	WS_THICKFRAME       = 0x00040000
	WS_GROUP            = 0x00020000
	WS_TABSTOP          = 0x00010000
	WS_MINIMIZEBOX      = 0x00020000
	WS_MAXIMIZEBOX      = 0x00010000
	WS_OVERLAPPEDWINDOW = WS_OVERLAPPED | WS_CAPTION | WS_SYSMENU | WS_THICKFRAME | WS_MINIMIZEBOX | WS_MAXIMIZEBOX
	WS_CAPTION          = WS_BORDER | WS_DLGFRAME

	// 扩展窗口样式
	WS_EX_ACCEPTFILES         = 0x00000010
	WS_EX_APPWINDOW           = 0x00040000
	WS_EX_CLIENTEDGE          = 0x00000200
	WS_EX_COMPOSITED          = 0x02000000
	WS_EX_CONTEXTHELP         = 0x00000400
	WS_EX_CONTROLPARENT       = 0x00010000
	WS_EX_DLGMODALFRAME       = 0x00000001
	WS_EX_LAYERED             = 0x00080000
	WS_EX_LAYOUTRTL           = 0x00400000
	WS_EX_LEFT                = 0x00000000
	WS_EX_LEFTSCROLLBAR       = 0x00004000
	WS_EX_LTRREADING          = 0x00000000
	WS_EX_MDICHILD            = 0x00000040
	WS_EX_NOACTIVATE          = 0x08000000
	WS_EX_NOINHERITLAYOUT     = 0x00100000
	WS_EX_NOPARENTNOTIFY      = 0x00000004
	WS_EX_NOREDIRECTIONBITMAP = 0x00200000
	WS_EX_OVERLAPPEDWINDOW    = WS_EX_WINDOWEDGE | WS_EX_CLIENTEDGE
	WS_EX_PALETTEWINDOW       = WS_EX_WINDOWEDGE | WS_EX_TOOLWINDOW | WS_EX_TOPMOST
	WS_EX_RIGHT               = 0x00001000
	WS_EX_RIGHTSCROLLBAR      = 0x00000000
	WS_EX_RTLREADING          = 0x00002000
	WS_EX_STATICEDGE          = 0x00020000
	WS_EX_TOOLWINDOW          = 0x00000080
	WS_EX_TOPMOST             = 0x00000008
	WS_EX_TRANSPARENT         = 0x00000020
	WS_EX_WINDOWEDGE          = 0x00000100

	// ShowWindow 命令
	SW_HIDE            = 0
	SW_SHOWNORMAL      = 1
	SW_NORMAL          = 1
	SW_SHOWMINIMIZED   = 2
	SW_SHOWMAXIMIZED   = 3
	SW_MAXIMIZE        = 3
	SW_SHOWNOACTIVATE  = 4
	SW_SHOW            = 5
	SW_MINIMIZE        = 6
	SW_SHOWMINNOACTIVE = 7
	SW_SHOWNA          = 8
	SW_RESTORE         = 9
	SW_SHOWDEFAULT     = 10
	SW_FORCEMINIMIZE   = 11
	SW_MAX             = 11

	// 鼠标键状态
	MK_LBUTTON = 0x0001
	MK_RBUTTON = 0x0002
	MK_SHIFT   = 0x0004
	MK_CONTROL = 0x0008
	MK_MBUTTON = 0x0010

	// DIB常量
	DIB_RGB_COLORS = 0
	BI_RGB         = 0

	// 光栅操作
	SRCCOPY     = 0x00CC0020
	SRCPAINT    = 0x00EE0086
	SRCAND      = 0x008800C6
	SRCINVERT   = 0x00660046
	SRCERASE    = 0x00440328
	NOTSRCCOPY  = 0x00330008
	NOTSRCERASE = 0x001100A6
	MERGECOPY   = 0x00C000CA
	MERGEPAINT  = 0x00BB0226
	PATCOPY     = 0x00F00021
	PATPAINT    = 0x00FB0A09
	PATINVERT   = 0x005A0049
	DSTINVERT   = 0x00550009
	BLACKNESS   = 0x00000042
	WHITENESS   = 0x00FF0062

	// SetStretchBltMode 模式
	STRETCH_ANDSCANS    = 1
	STRETCH_ORSCANS     = 2
	STRETCH_DELETESCANS = 3
	STRETCH_HALFTONE    = 4

	// 消息参数
	PM_NOREMOVE = 0x0000
	PM_REMOVE   = 0x0001
	PM_NOYIELD  = 0x0002

	// SetWindowPos 标志
	SWP_NOSIZE         = 0x0001
	SWP_NOMOVE         = 0x0002
	SWP_NOZORDER       = 0x0004
	SWP_NOREDRAW       = 0x0008
	SWP_NOACTIVATE     = 0x0010
	SWP_FRAMECHANGED   = 0x0020
	SWP_SHOWWINDOW     = 0x0040
	SWP_HIDEWINDOW     = 0x0080
	SWP_NOCOPYBITS     = 0x0100
	SWP_NOOWNERZORDER  = 0x0200
	SWP_NOSENDCHANGING = 0x0400
	SWP_DRAWFRAME      = SWP_FRAMECHANGED
	SWP_NOREPOSITION   = SWP_NOOWNERZORDER
	SWP_DEFERERASE     = 0x2000
	SWP_ASYNCWINDOWPOS = 0x4000
	// 标准光标
	IDC_ARROW       = 32512
	IDC_IBEAM       = 32513
	IDC_WAIT        = 32514
	IDC_CROSS       = 32515
	IDC_UPARROW     = 32516
	IDC_SIZE        = 32640
	IDC_ICON        = 32641
	IDC_SIZENWSE    = 32642
	IDC_SIZENESW    = 32643
	IDC_SIZEWE      = 32644
	IDC_SIZENS      = 32645
	IDC_SIZEALL     = 32646
	IDC_NO          = 32648
	IDC_HAND        = 32649
	IDC_APPSTARTING = 32650
	IDC_HELP        = 32651

	// 系统度量常量
	SM_CXSCREEN = 0
	SM_CYSCREEN = 1

	// 窗口图标消息与常量
	WM_SETICON      = 0x0080
	ICON_SMALL      = 0
	ICON_BIG        = 1
	LR_DEFAULTCOLOR = 0x0000
	LR_SHARED       = 0x8000
)

// ICONINFO 图标/光标信息结构体 (兼容 64 位平台 8 字节对齐)
type ICONINFO struct {
	FIcon    int32
	XHotspot uint32
	YHotspot uint32
	_        uint32 // 显式 4 字节填充，满足 64 位系统下 HBITMAP 8 字节对齐
	HbmMask  uintptr
	HbmColor uintptr
}

// WNDCLASSEXW 窗口类结构
type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

// MSG 消息结构
type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      Point
}

// Point 点结构
type Point struct {
	X, Y int32
}

// RECT 矩形结构
type RECT struct {
	Left, Top, Right, Bottom int32
}

// PAINTSTRUCT 绘图结构
type PAINTSTRUCT struct {
	Hdc         uintptr
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

// BITMAPINFOHEADER 位图信息头
type BITMAPINFOHEADER struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

// BITMAPINFO 位图信息
type BITMAPINFO struct {
	BmiHeader BITMAPINFOHEADER
	BmiColors [1]uint32
}

// GetModuleHandle 获取模块句柄
func GetModuleHandle() uintptr {
	ret, _, _ := getModuleHandleW.Call(uintptr(0))
	return ret
}

// GetCurrentThreadId 获取当前线程ID
func GetCurrentThreadID() uint32 {
	ret, _, _ := getCurrentThreadId.Call()
	return uint32(ret)
}

// RegisterClassEx 注册窗口类
func RegisterClassEx(wndClass *WNDCLASSEXW) uint16 {
	ret, _, _ := registerClassExW.Call(uintptr(unsafe.Pointer(wndClass)))
	return uint16(ret)
}

// UnregisterClass 注销窗口类
func UnregisterClass(className *uint16, hInstance uintptr) bool {
	ret, _, _ := unregisterClassW.Call(uintptr(unsafe.Pointer(className)), hInstance)
	return ret != 0
}

// CreateWindowEx 创建窗口
func CreateWindowEx(dwExStyle uint32, className, windowName *uint16,
	dwStyle uint32, x, y, nWidth, nHeight int32,
	hWndParent uintptr, hMenu uintptr, hInstance uintptr,
	lpParam uintptr) uintptr {

	ret, _, _ := createWindowExW.Call(
		uintptr(dwExStyle),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		uintptr(dwStyle),
		uintptr(x), uintptr(y),
		uintptr(nWidth), uintptr(nHeight),
		hWndParent, hMenu, hInstance, lpParam,
	)
	return ret
}

// DestroyWindow 销毁窗口
func DestroyWindow(hWnd uintptr) bool {
	ret, _, _ := destroyWindow.Call(hWnd)
	return ret != 0
}

// IsWindow 判断句柄是否对应一个有效的窗口
func IsWindow(hWnd uintptr) bool {
	ret, _, _ := isWindow.Call(hWnd)
	return ret != 0
}

// DefWindowProc 默认窗口过程
func DefWindowProc(hWnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	ret, _, _ := defWindowProcW.Call(hWnd, uintptr(msg), wParam, lParam)
	return ret
}

// GetMessage 获取消息
func GetMessage(msg *MSG, hWnd uintptr, wMsgFilterMin, wMsgFilterMax uint32) int32 {
	ret, _, _ := getMessageW.Call(uintptr(unsafe.Pointer(msg)), hWnd,
		uintptr(wMsgFilterMin), uintptr(wMsgFilterMax))
	return int32(ret)
}

// PeekMessage 查看消息
func PeekMessage(msg *MSG, hWnd uintptr, wMsgFilterMin, wMsgFilterMax uint32, wRemoveMsg uint32) bool {
	ret, _, _ := peekMessageW.Call(
		uintptr(unsafe.Pointer(msg)),
		hWnd,
		uintptr(wMsgFilterMin),
		uintptr(wMsgFilterMax),
		uintptr(wRemoveMsg),
	)
	return ret != 0
}

// WaitMessage 等待新消息到达（使线程休眠）
func WaitMessage() bool {
	ret, _, _ := waitMessage.Call()
	return ret != 0
}

// TranslateMessage 翻译消息
func TranslateMessage(msg *MSG) bool {
	ret, _, _ := translateMessage.Call(uintptr(unsafe.Pointer(msg)))
	return ret != 0
}

// DispatchMessage 分发消息
func DispatchMessage(msg *MSG) uintptr {
	ret, _, _ := dispatchMessageW.Call(uintptr(unsafe.Pointer(msg)))
	return ret
}

// PostQuitMessage 发送退出消息
func PostQuitMessage(nExitCode int32) {
	postQuitMessage.Call(uintptr(nExitCode))
}

// GetSystemMetrics 获取系统度量信息
func GetSystemMetrics(nIndex int32) int32 {
	ret, _, _ := getSystemMetrics.Call(uintptr(nIndex))
	return int32(ret)
}

// CreateIconIndirect 从 ICONINFO 创建图标或光标
func CreateIconIndirect(piconinfo *ICONINFO) uintptr {
	ret, _, _ := createIconIndirect.Call(uintptr(unsafe.Pointer(piconinfo)))
	return ret
}

// CreateIconFromResourceEx 从图标资源字节流创建图标
func CreateIconFromResourceEx(pbIconBits uintptr, cbIconBits uint32, fIcon int32, dwVersion uint32, cxDesired, cyDesired int32, flags uint32) uintptr {
	ret, _, _ := createIconFromResourceEx.Call(
		pbIconBits,
		uintptr(cbIconBits),
		uintptr(fIcon),
		uintptr(dwVersion),
		uintptr(cxDesired),
		uintptr(cyDesired),
		uintptr(flags),
	)
	return ret
}

// DestroyIcon 销毁图标句柄
func DestroyIcon(hIcon uintptr) bool {
	if hIcon == 0 {
		return true
	}
	ret, _, _ := destroyIcon.Call(hIcon)
	return ret != 0
}

// CreateBitmap 创建单色或彩色位图
func CreateBitmap(nWidth, nHeight int32, nPlanes, nBitCount uint32, lpBits uintptr) uintptr {
	ret, _, _ := createBitmap.Call(
		uintptr(nWidth),
		uintptr(nHeight),
		uintptr(nPlanes),
		uintptr(nBitCount),
		lpBits,
	)
	return ret
}

// EnableDPIAwareness 启用进程最高级别 DPI 识别（防止高分屏模糊与窗口客户区尺寸失真）
func EnableDPIAwareness() {
	// 1. 优先采用 Windows 10 (1703+) 推荐的 Per-Monitor V2 模式
	if setProcessDpiAwarenessContext.Find() == nil {
		// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = -4
		ret, _, _ := setProcessDpiAwarenessContext.Call(^uintptr(3))
		if ret != 0 {
			return
		}
	}
	// 2. 备选采用 Shcore.dll 的 SetProcessDpiAwareness (Windows 8.1+)
	shcore := syscall.NewLazyDLL("shcore.dll")
	setAwareness := shcore.NewProc("SetProcessDpiAwareness")
	if setAwareness.Find() == nil {
		// PROCESS_PER_MONITOR_DPI_AWARE = 2
		ret, _, _ := setAwareness.Call(2)
		if ret == 0 { // S_OK = 0
			return
		}
	}
	// 3. 保底采用 Windows Vista/7 的 SetProcessDPIAware
	if setProcessDPIAware.Find() == nil {
		setProcessDPIAware.Call()
	}
}

// DPI 相关动态 API 过程
var (
	getDpiForWindow          = user32.NewProc("GetDpiForWindow")
	getDpiForSystem          = user32.NewProc("GetDpiForSystem")
	adjustWindowRectExForDpi = user32.NewProc("AdjustWindowRectExForDpi")
)

// GetDpiForWindow 获取指定窗口当前的真实 DPI
func GetDpiForWindow(hWnd uintptr) uint32 {
	if getDpiForWindow.Find() == nil {
		ret, _, _ := getDpiForWindow.Call(hWnd)
		if ret > 0 {
			return uint32(ret)
		}
	}
	return GetDpiForSystem()
}

// GetDpiForSystem 获取当前操作系统的默认系统 DPI
func GetDpiForSystem() uint32 {
	if getDpiForSystem.Find() == nil {
		ret, _, _ := getDpiForSystem.Call()
		if ret > 0 {
			return uint32(ret)
		}
	}
	return 96
}

// PostMessage 投递消息
func PostMessage(hWnd uintptr, msg uint32, wParam, lParam uintptr) bool {
	ret, _, _ := postMessageW.Call(hWnd, uintptr(msg), wParam, lParam)
	return ret != 0
}

// SendMessage 发送消息
func SendMessage(hWnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	ret, _, _ := sendMessageW.Call(hWnd, uintptr(msg), wParam, lParam)
	return ret
}

// UpdateWindow 更新窗口
func UpdateWindow(hWnd uintptr) bool {
	ret, _, _ := updateWindow.Call(hWnd)
	return ret != 0
}

// InvalidateRect 使矩形无效
func InvalidateRect(hWnd uintptr, rect *RECT, bErase bool) bool {
	var erase uintptr
	if bErase {
		erase = 1
	}
	ret, _, _ := invalidateRect.Call(hWnd, uintptr(unsafe.Pointer(rect)), erase)
	return ret != 0
}

// ValidateRect 验证矩形
func ValidateRect(hWnd uintptr, rect *RECT) bool {
	ret, _, _ := validateRect.Call(hWnd, uintptr(unsafe.Pointer(rect)))
	return ret != 0
}

// SetWindowText 设置窗口标题
func SetWindowText(hWnd uintptr, title string) {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	setWindowTextW.Call(hWnd, uintptr(unsafe.Pointer(titlePtr)))
}

// GetWindowText 获取窗口标题
func GetWindowText(hWnd uintptr) string {
	length := GetWindowTextLength(hWnd)
	if length == 0 {
		return ""
	}
	buf := make([]uint16, length+1)
	getWindowTextW.Call(hWnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(length+1))
	return syscall.UTF16ToString(buf)
}

// GetWindowTextLength 获取窗口标题长度
func GetWindowTextLength(hWnd uintptr) int32 {
	ret, _, _ := getWindowTextLengthW.Call(hWnd)
	return int32(ret)
}

// GetClientRect 获取客户区矩形
func GetClientRect(hWnd uintptr) RECT {
	var rect RECT
	getClientRect.Call(hWnd, uintptr(unsafe.Pointer(&rect)))
	return rect
}

// GetWindowRect 获取窗口矩形
func GetWindowRect(hWnd uintptr) RECT {
	var rect RECT
	getWindowRect.Call(hWnd, uintptr(unsafe.Pointer(&rect)))
	return rect
}

// SetWindowPos 设置窗口位置
func SetWindowPos(hWnd, hWndInsertAfter uintptr, x, y, cx, cy int32, uFlags uint32) bool {
	ret, _, _ := setWindowPos.Call(hWnd, hWndInsertAfter,
		uintptr(x), uintptr(y), uintptr(cx), uintptr(cy), uintptr(uFlags))
	return ret != 0
}

// ShowWindow 显示窗口
func ShowWindow(hWnd uintptr, nCmdShow int32) bool {
	ret, _, _ := showWindow.Call(hWnd, uintptr(nCmdShow))
	return ret != 0
}

// IsIconic 判断窗口是否处于最小化状态
func IsIconic(hWnd uintptr) bool {
	ret, _, _ := isIconic.Call(hWnd)
	return ret != 0
}

// IsWindowVisible 判断窗口是否可见
func IsWindowVisible(hWnd uintptr) bool {
	ret, _, _ := isWindowVisible.Call(hWnd)
	return ret != 0
}

// EnableWindow 启用窗口
func EnableWindow(hWnd uintptr, bEnable bool) bool {
	var enable uintptr
	if bEnable {
		enable = 1
	}
	ret, _, _ := enableWindow.Call(hWnd, enable)
	return ret != 0
}

// IsWindowEnabled 判断窗口是否处于启用状态
func IsWindowEnabled(hWnd uintptr) bool {
	ret, _, _ := isWindowEnabled.Call(hWnd)
	return ret != 0
}

// SetFocus 设置焦点
func SetFocus(hWnd uintptr) uintptr {
	ret, _, _ := setFocus.Call(hWnd)
	return ret
}

// GetFocus 获取焦点窗口
func GetFocus() uintptr {
	ret, _, _ := getFocus.Call()
	return ret
}

// SetForegroundWindow 设置前台窗口
func SetForegroundWindow(hWnd uintptr) bool {
	ret, _, _ := setForegroundWindow.Call(hWnd)
	return ret != 0
}

// BeginPaint 开始绘图
func BeginPaint(hWnd uintptr, ps *PAINTSTRUCT) uintptr {
	ret, _, _ := beginPaint.Call(hWnd, uintptr(unsafe.Pointer(ps)))
	return ret
}

// EndPaint 结束绘图
func EndPaint(hWnd uintptr, ps *PAINTSTRUCT) {
	endPaint.Call(hWnd, uintptr(unsafe.Pointer(ps)))
}

// GetDC 获取设备上下文
func GetDC(hWnd uintptr) uintptr {
	ret, _, _ := getDC.Call(hWnd)
	return ret
}

// ReleaseDC 释放设备上下文
func ReleaseDC(hWnd, hDC uintptr) int32 {
	ret, _, _ := releaseDC.Call(hWnd, hDC)
	return int32(ret)
}

// GetWindowDC 获取窗口设备上下文
func GetWindowDC(hWnd uintptr) uintptr {
	ret, _, _ := getWindowDC.Call(hWnd)
	return ret
}

// CreateCompatibleDC 创建兼容设备上下文
func CreateCompatibleDC(hDC uintptr) uintptr {
	ret, _, _ := createCompatibleDC.Call(hDC)
	return ret
}

// DeleteDC 删除设备上下文
func DeleteDC(hDC uintptr) bool {
	ret, _, _ := deleteDC.Call(hDC)
	return ret != 0
}

// SelectObject 选择对象
func SelectObject(hDC, hObject uintptr) uintptr {
	ret, _, _ := selectObject.Call(hDC, hObject)
	return ret
}

// DeleteObject 删除对象
func DeleteObject(hObject uintptr) bool {
	ret, _, _ := deleteObject.Call(hObject)
	return ret != 0
}

// CreateDIBSection 创建DIB节
func CreateDIBSection(hDC uintptr, bmi *BITMAPINFO, usage uint32, ppvBits *unsafe.Pointer, hSection uintptr, offset uint32) uintptr {
	ret, _, _ := createDIBSection.Call(hDC, uintptr(unsafe.Pointer(bmi)),
		uintptr(usage), uintptr(unsafe.Pointer(ppvBits)), hSection, uintptr(offset))
	return ret
}

// BitBlt 位块传输
func BitBlt(hdcDest uintptr, nXDest, nYDest, nWidth, nHeight int32,
	hdcSrc uintptr, nXSrc, nYSrc int32, dwRop uint32) bool {

	ret, _, _ := bitBlt.Call(hdcDest,
		uintptr(nXDest), uintptr(nYDest), uintptr(nWidth), uintptr(nHeight),
		hdcSrc, uintptr(nXSrc), uintptr(nYSrc), uintptr(dwRop))
	return ret != 0
}

// StretchBlt 拉伸位块传输
func StretchBlt(hdcDest uintptr, nXOriginDest, nYOriginDest, nWidthDest, nHeightDest int32,
	hdcSrc uintptr, nXOriginSrc, nYOriginSrc, nWidthSrc, nHeightSrc int32, dwRop uint32) bool {

	ret, _, _ := stretchBlt.Call(hdcDest,
		uintptr(nXOriginDest), uintptr(nYOriginDest), uintptr(nWidthDest), uintptr(nHeightDest),
		hdcSrc, uintptr(nXOriginSrc), uintptr(nYOriginSrc), uintptr(nWidthSrc), uintptr(nHeightSrc),
		uintptr(dwRop))
	return ret != 0
}

// SetDIBitsToDevice 设置DIB位到设备
func SetDIBitsToDevice(hdc uintptr, xDest, yDest, w, h int32,
	xSrc, ySrc int32, uStartScan, cScanLines uint32, lpvBits unsafe.Pointer,
	lpbmi *BITMAPINFO, fuColorUse uint32) int32 {

	ret, _, _ := setDIBitsToDevice.Call(hdc,
		uintptr(xDest), uintptr(yDest), uintptr(w), uintptr(h),
		uintptr(xSrc), uintptr(ySrc), uintptr(uStartScan), uintptr(cScanLines),
		uintptr(lpvBits), uintptr(unsafe.Pointer(lpbmi)), uintptr(fuColorUse))
	return int32(ret)
}

// GetStockObject 获取库存对象
func GetStockObject(fnObject int) uintptr {
	ret, _, _ := getStockObject.Call(uintptr(fnObject))
	return ret
}

// SetStretchBltMode 设置拉伸模式
func SetStretchBltMode(hdc uintptr, iStretchMode int32) int32 {
	ret, _, _ := setStretchBltMode.Call(hdc, uintptr(iStretchMode))
	return int32(ret)
}

// AdjustWindowRectEx 根据窗口样式计算窗口大小
func AdjustWindowRectEx(lpRect *RECT, dwStyle uint32, bMenu bool, dwExStyle uint32) bool {
	var menu uintptr
	if bMenu {
		menu = 1
	}
	ret, _, _ := adjustWindowRectEx.Call(uintptr(unsafe.Pointer(lpRect)),
		uintptr(dwStyle), menu, uintptr(dwExStyle))
	return ret != 0
}

// AdjustWindowRectExForDpi 根据 DPI 与窗口样式高精度计算窗口大小（防高分屏尺寸裁切与模糊）
func AdjustWindowRectExForDpi(lpRect *RECT, dwStyle uint32, bMenu bool, dwExStyle uint32, dpi uint32) bool {
	if adjustWindowRectExForDpi.Find() == nil {
		var menu uintptr
		if bMenu {
			menu = 1
		}
		ret, _, _ := adjustWindowRectExForDpi.Call(
			uintptr(unsafe.Pointer(lpRect)),
			uintptr(dwStyle),
			menu,
			uintptr(dwExStyle),
			uintptr(dpi),
		)
		if ret != 0 {
			return true
		}
	}
	return AdjustWindowRectEx(lpRect, dwStyle, bMenu, dwExStyle)
}

// POINT 坐标点结构体 (Win32 原生定义)
type POINT struct {
	X int32
	Y int32
}

// MINMAXINFO 窗口最小/最大尺寸约束
type MINMAXINFO struct {
	Reserved     POINT
	MaxSize      POINT
	MaxPosition  POINT
	MinTrackSize POINT
	MaxTrackSize POINT
}

// COMPOSITIONFORM 输入法组合窗口定位信息
type COMPOSITIONFORM struct {
	Style      uint32
	CurrentPos POINT
	Area       RECT
}

// CANDIDATEFORM 输入法候选列表定位信息
type CANDIDATEFORM struct {
	Index      uint32
	Style      uint32
	CurrentPos POINT
	Area       RECT
}

// ScreenToClient 将屏幕物理坐标转换为窗口客户区相对坐标
func ScreenToClient(hWnd uintptr, lpPoint *POINT) bool {
	ret, _, _ := screenToClient.Call(hWnd, uintptr(unsafe.Pointer(lpPoint)))
	return ret != 0
}

// UTF16PtrToString 将UTF16指针转换为字符串
func UTF16PtrToString(ptr *uint16) string {
	if ptr == nil {
		return ""
	}
	// 手动实现UTF16转字符串（unsafe.Add 保持指针类型，避免 uintptr 往返）
	var length int
	for p := ptr; *p != 0; p = (*uint16)(unsafe.Add(unsafe.Pointer(p), 2)) {
		length++
	}
	if length == 0 {
		return ""
	}
	return syscall.UTF16ToString(unsafe.Slice(ptr, length))
}

// StringToUTF16Ptr 将字符串转换为UTF16指针
func StringToUTF16Ptr(s string) *uint16 {
	ptr, _ := syscall.UTF16PtrFromString(s)
	return ptr
}

// LOWORD 获取低位字
func LOWORD(dw uintptr) uint16 {
	return uint16(dw & 0xFFFF)
}

// HIWORD 获取高位字
func HIWORD(dw uintptr) uint16 {
	return uint16((dw >> 16) & 0xFFFF)
}

// GET_X_LPARAM 从LPARAM获取X坐标
func GET_X_LPARAM(lp uintptr) int16 {
	return int16(LOWORD(lp))
}

// GET_Y_LPARAM 从LPARAM获取Y坐标
func GET_Y_LPARAM(lp uintptr) int16 {
	return int16(HIWORD(lp))
}

// 额外的Win32 API
var (
	loadCursorW    = user32.NewProc("LoadCursorW")
	getKeyState    = user32.NewProc("GetKeyState")
	setCapture     = user32.NewProc("SetCapture")
	releaseCapture = user32.NewProc("ReleaseCapture")
)

// LoadCursor 加载光标
func LoadCursor(hInstance uintptr, lpCursorName uintptr) uintptr {
	ret, _, _ := loadCursorW.Call(hInstance, lpCursorName)
	return ret
}

// GetKeyState 获取键状态
func GetKeyState(nVirtKey int) int16 {
	ret, _, _ := getKeyState.Call(uintptr(nVirtKey))
	return int16(ret)
}

// SetCapture 设置鼠标捕获
func SetCapture(hWnd uintptr) uintptr {
	ret, _, _ := setCapture.Call(hWnd)
	return ret
}

// ReleaseCapture 释放鼠标捕获
func ReleaseCapture() bool {
	ret, _, _ := releaseCapture.Call()
	return ret != 0
}

// 输入法 imm32 API
var (
	imm32                    = syscall.NewLazyDLL("imm32.dll")
	immGetContext            = imm32.NewProc("ImmGetContext")
	immReleaseContext        = imm32.NewProc("ImmReleaseContext")
	immGetCompositionStringW = imm32.NewProc("ImmGetCompositionStringW")
	immSetCompositionWindow  = imm32.NewProc("ImmSetCompositionWindow")
	immSetCandidateWindow    = imm32.NewProc("ImmSetCandidateWindow")
)

// ImmGetContext 获取窗口的输入法上下文
func ImmGetContext(hWnd uintptr) uintptr {
	h, _, _ := immGetContext.Call(hWnd)
	return h
}

// ImmReleaseContext 释放输入法上下文
func ImmReleaseContext(hWnd, hImc uintptr) bool {
	ret, _, _ := immReleaseContext.Call(hWnd, hImc)
	return ret != 0
}

// GetImeCompositionString 读取指定类型的输入法组合字符串
func GetImeCompositionString(hImc uintptr, flag uint32) string {
	if hImc == 0 {
		return ""
	}
	n, _, _ := immGetCompositionStringW.Call(hImc, uintptr(flag), 0, 0)
	size := int32(n)
	if size <= 0 {
		return ""
	}
	buf := make([]uint16, size/2)
	immGetCompositionStringW.Call(hImc, uintptr(flag), uintptr(unsafe.Pointer(&buf[0])), n)
	return string(utf16.Decode(buf))
}

// SetImeWindowPosition 将输入法组合窗口与候选列表定位到客户区坐标 (x, y)
func SetImeWindowPosition(hWnd uintptr, x, y, lineHeight int32) {
	hImc, _, _ := immGetContext.Call(hWnd)
	if hImc == 0 {
		return
	}
	defer immReleaseContext.Call(hWnd, hImc)
	if lineHeight <= 0 {
		lineHeight = 20
	}
	cf := COMPOSITIONFORM{
		Style:      CFS_POINT | CFS_FORCE_POSITION,
		CurrentPos: POINT{X: x, Y: y},
		Area:       RECT{Left: x, Top: y, Right: x + 400, Bottom: y + lineHeight},
	}
	immSetCompositionWindow.Call(hImc, uintptr(unsafe.Pointer(&cf)))
	cd := CANDIDATEFORM{
		Index:      0,
		Style:      CFS_CANDIDATEPOS,
		CurrentPos: POINT{X: x, Y: y + lineHeight},
	}
	immSetCandidateWindow.Call(hImc, uintptr(unsafe.Pointer(&cd)))
}

// 光标与定时器 Win32 API
var (
	getCursorPos = user32.NewProc("GetCursorPos")
	setCursor    = user32.NewProc("SetCursor")
	setTimer     = user32.NewProc("SetTimer")
	killTimer    = user32.NewProc("KillTimer")
)

// GetCursorPos 获取鼠标指针屏幕绝对坐标
func GetCursorPos(lpPoint *POINT) bool {
	ret, _, _ := getCursorPos.Call(uintptr(unsafe.Pointer(lpPoint)))
	return ret != 0
}

// SetCursor 设置鼠标光标
func SetCursor(hCursor uintptr) uintptr {
	ret, _, _ := setCursor.Call(hCursor)
	return ret
}

// SetTimer 创建系统定时器
func SetTimer(hWnd uintptr, nIDEvent uintptr, uElapse uint32, lpTimerFunc uintptr) uintptr {
	ret, _, _ := setTimer.Call(hWnd, nIDEvent, uintptr(uElapse), lpTimerFunc)
	return ret
}

// KillTimer 销毁系统定时器
func KillTimer(hWnd uintptr, uIDEvent uintptr) bool {
	ret, _, _ := killTimer.Call(hWnd, uIDEvent)
	return ret != 0
}

// 剪贴板相关常量（剪贴板读写统一由 clipboard 包提供）
const (
	WM_TIMER     = 0x0113
	WM_SETCURSOR = 0x0020
	HTCLIENT     = 1
)
