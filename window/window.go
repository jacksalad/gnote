//go:build windows

package window

import (
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/jacksalad/gnote/component"
	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/theme"
)

// WindowConfig 窗口配置
type WindowConfig struct {
	Title     string
	Width     int32
	Height    int32
	X         int32
	Y         int32
	Center    bool   // 是否在屏幕中央居中显示
	Icon      *Icon  // 自定义窗口图标 (包含 16x16 与 32x32 或更高分辨率)
	IconBytes []byte // .ico 二进制文件流
	Resizable bool
	Visible   bool

	// Owner 可选：拥有者窗口。独立设置窗口使用主窗口作为 owner，
	// 使其始终浮在主窗口之上，并且不在任务栏生成独立按钮。
	Owner *Window
	// Modal 为 true 时，窗口创建后禁用 Owner，窗口销毁后恢复。
	// 同一个 Owner 可以同时拥有多个模态子窗口；最后一个关闭时才恢复。
	Modal bool
}

// Window 窗口结构
type Window struct {
	hwnd            uintptr
	hInstance       uintptr
	config          WindowConfig
	eventBus        *event.EventBus
	renderer        *render.SoftwareRenderer
	rootComponent   component.Component
	backgroundColor graphics.Color
	currentCursor   uintptr
	customIcon      *Icon
	dpi             uint32
	style           uint32 // 创建时的窗口样式，用于客户区/窗口尺寸换算

	// 多窗口 / 模态关系
	ownerWin      *Window // 拥有者窗口（独立设置窗口使用）
	modal         bool    // 当前窗口是否为 owner 的模态子窗口
	modalChildren int     // 当前窗口作为 owner 时，仍在打开的模态子窗口数量

	// GDI资源
	hdcMem  uintptr
	hBitmap uintptr
	pBits   *uint32
	bmi     BITMAPINFO

	// 状态
	running     int32 // 使用atomic操作
	closed      int32
	mu          sync.RWMutex
	onClose     func() bool         // 窗口即将关闭时的拦截回调 (返回 true 允许关闭，返回 false 阻止关闭)
	lastFocused component.Component // 窗口失焦前的焦点组件，激活时恢复

	// 渲染控制：UI 线程标识用于把跨线程重绘请求封送回消息循环
	uiThreadID  uint32
	renderCount int64
	// 局部呈现矩形（光标闪烁只 BitBlt 该区域）；valid=false 时按整个客户区呈现
	presentRect      graphics.Rect
	presentRectValid bool

	// 模态 owner 状态：只有子窗口真正显示后才禁用 owner，避免创建失败/取消时卡死主窗
	ownerDisabled bool

	// UI 线程延迟任务（如查找输入防抖），由 WM_TIMER 驱动的轻量队列
	taskMu      sync.Mutex
	tasks       []uiTask
	taskTimerOn bool

	// 后备缓冲区尺寸与 UTF-16 高位代理
	dibWidth, dibHeight       int32
	clientWidth, clientHeight int32 // 缓存客户区尺寸，避免每帧 GetClientRect 系统调用
	pendingHighSurrogate      uint16

	// 输入法组合状态（组合串由组件内联绘制）
	imeComposing bool
}

var (
	windowMap       = make(map[uintptr]*Window)
	windowMapMu     sync.RWMutex
	className       = StringToUTF16Ptr("GouiWindowClass")
	classRegistered bool
	once            sync.Once
)

// activeWindowCount 返回当前仍存活的窗口数量。
// 只有最后一个窗口销毁时才退出消息循环，独立设置窗口关闭不会杀死主进程。
func activeWindowCount() int {
	windowMapMu.RLock()
	defer windowMapMu.RUnlock()
	return len(windowMap)
}

// uiTask 延迟到 UI 线程执行的一次性任务
type uiTask struct {
	at time.Time
	fn func()
}

// 每个窗口独立安装的系统定时器 ID
const (
	cursorTimerID = 1 // 输入框光标闪烁
	uiTaskTimerID = 2 // 延迟任务队列
)

// NewWindow 创建新窗口
func NewWindow(config WindowConfig) (*Window, error) {
	runtime.LockOSThread()
	hInstance := GetModuleHandle()

	// 注册窗口类（只注册一次）
	once.Do(func() {
		EnableDPIAwareness()
		wndClass := WNDCLASSEXW{
			CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
			Style:         CS_HREDRAW | CS_VREDRAW | CS_DBLCLKS,
			LpfnWndProc:   syscall.NewCallback(globalWndProc),
			CbClsExtra:    0,
			CbWndExtra:    0,
			HInstance:     hInstance,
			HIcon:         0,
			HCursor:       LoadCursor(0, IDC_ARROW),
			HbrBackground: 0,
			LpszMenuName:  nil,
			LpszClassName: className,
			HIconSm:       0,
		}

		atom := RegisterClassEx(&wndClass)
		if atom == 0 {
			// 类可能已注册，忽略错误
		}
		classRegistered = true
	})

	// 计算窗口样式
	dwStyle := uint32(WS_OVERLAPPEDWINDOW)
	if !config.Resizable {
		dwStyle = WS_OVERLAPPED | WS_CAPTION | WS_SYSMENU | WS_MINIMIZEBOX
	}

	// 计算窗口大小（根据当前系统 DPI 自动调整边框，防止高分屏尺寸裁剪与失真）
	sysDpi := GetDpiForSystem()
	rect := RECT{Left: 0, Top: 0, Right: config.Width, Bottom: config.Height}
	AdjustWindowRectExForDpi(&rect, dwStyle, false, 0, sysDpi)
	windowWidth := rect.Right - rect.Left
	windowHeight := rect.Bottom - rect.Top

	// 计算窗口初始位置（若指定 Center 则居中到主屏幕）
	posX := config.X
	posY := config.Y
	if config.Center {
		screenW := GetSystemMetrics(SM_CXSCREEN)
		screenH := GetSystemMetrics(SM_CYSCREEN)
		if screenW > 0 && screenH > 0 {
			posX = (screenW - windowWidth) / 2
			posY = (screenH - windowHeight) / 2
			if posX < 0 {
				posX = 0
			}
			if posY < 0 {
				posY = 0
			}
		}
	}

	// 创建窗口
	// Owner 必须是仍然有效的窗口；已销毁的 owner 不再参与父子/模态关系。
	owner := config.Owner
	if owner != nil && (!IsWindow(owner.hwnd) || atomic.LoadInt32(&owner.closed) == 1) {
		owner = nil
	}
	var ownerHwnd uintptr
	if owner != nil {
		ownerHwnd = owner.hwnd
	}

	titlePtr := StringToUTF16Ptr(config.Title)
	hwnd := CreateWindowEx(
		0,
		className,
		titlePtr,
		dwStyle|WS_CLIPCHILDREN|WS_CLIPSIBLINGS,
		posX, posY,
		windowWidth, windowHeight,
		ownerHwnd, 0, hInstance, 0,
	)

	if hwnd == 0 {
		runtime.UnlockOSThread()
		return nil, ErrCreateWindowFailed
	}

	winDpi := GetDpiForWindow(hwnd)
	if winDpi == 0 {
		winDpi = sysDpi
	}

	w := &Window{
		hwnd:            hwnd,
		hInstance:       hInstance,
		config:          config,
		dpi:             winDpi,
		style:           dwStyle,
		eventBus:        event.NewEventBus(16),
		running:         1,
		closed:          0,
		backgroundColor: theme.Current().Background,
		ownerWin:        owner,
		modal:           config.Modal && owner != nil,
		uiThreadID:      GetCurrentThreadID(),
	}

	// 初始化并应用窗口图标
	if config.Icon != nil {
		w.SetIcon(config.Icon)
	} else if len(config.IconBytes) > 0 {
		_ = w.SetIconFromBytes(config.IconBytes)
	}

	// 初始化渲染器
	w.initRenderer()

	// 注册到窗口映射
	windowMapMu.Lock()
	windowMap[hwnd] = w
	windowMapMu.Unlock()

	// 光标闪烁定时器按窗口独立安装。独立设置窗口不调用 Run，
	// 但它的消息仍由主窗口的 GetMessage(0) 循环统一泵送，因此同样能收到 WM_TIMER。
	SetTimer(hwnd, cursorTimerID, 500, 0)

	// 显示窗口
	if config.Visible {
		w.Show()
		if w.modal {
			SetForegroundWindow(hwnd)
			SetFocus(hwnd)
		}
	}

	return w, nil
}

// rebuildBackBuffer 确保存在覆盖请求尺寸的 DIB 后备缓冲区；失败时保留旧缓冲区并返回 false。
// 客户区略微变小（面积不低于当前缓冲一半）时直接复用；显著变小则重建收缩，
// 避免最大化后长期按历史最大尺寸清屏与呈现。生长时预留 25% 余量减少拖拽重分配。
func (w *Window) rebuildBackBuffer(width, height int32) bool {
	if width <= 0 || height <= 0 || w.hwnd == 0 {
		return false
	}
	if width > 32767 || height > 32767 {
		// 单块 32bpp 画布超过 ~4GB 时拒绝分配，避免 OOM/整数溢出
		return false
	}

	if w.pBits != nil && width <= w.dibWidth && height <= w.dibHeight {
		if int64(width)*int64(height)*2 >= int64(w.dibWidth)*int64(w.dibHeight) {
			return true
		}
		// 面积不足一半：收缩到当前客户区
	} else {
		needW := width * 5 / 4
		needH := height * 5 / 4
		if needW < w.dibWidth {
			needW = w.dibWidth
		}
		if needH < w.dibHeight {
			needH = w.dibHeight
		}
		if needW > 32767 {
			needW = 32767
		}
		if needH > 32767 {
			needH = 32767
		}
		width, height = needW, needH
	}

	hdc := GetDC(w.hwnd)
	if hdc == 0 {
		return false
	}
	defer ReleaseDC(w.hwnd, hdc)

	bmi := w.bmi
	bmi.BmiHeader.BiSize = uint32(unsafe.Sizeof(BITMAPINFOHEADER{}))
	bmi.BmiHeader.BiWidth = width
	bmi.BmiHeader.BiHeight = -height // 负值表示从上到下
	bmi.BmiHeader.BiPlanes = 1
	bmi.BmiHeader.BiBitCount = 32
	bmi.BmiHeader.BiCompression = BI_RGB
	bmi.BmiHeader.BiSizeImage = uint32(width) * uint32(height) * 4

	var pBits unsafe.Pointer
	hBitmap := CreateDIBSection(hdc, &bmi, DIB_RGB_COLORS, &pBits, 0, 0)
	if hBitmap == 0 || pBits == nil {
		return false
	}

	if w.hdcMem == 0 {
		w.hdcMem = CreateCompatibleDC(hdc)
		if w.hdcMem == 0 {
			DeleteObject(hBitmap)
			return false
		}
	}

	// 先换入新位图，旧位图被自动取消选中后再删除（避免删除选中对象）
	old := SelectObject(w.hdcMem, hBitmap)
	if old == 0 {
		DeleteObject(hBitmap)
		return false
	}

	if w.hBitmap != 0 {
		DeleteObject(w.hBitmap)
	}

	w.bmi = bmi
	w.hBitmap = hBitmap
	w.pBits = (*uint32)(pBits)
	w.dibWidth = width
	w.dibHeight = height
	return true
}

// initRenderer 初始化渲染器
func (w *Window) initRenderer() {
	// 获取客户区大小
	rect := GetClientRect(w.hwnd)
	width := rect.Right - rect.Left
	height := rect.Bottom - rect.Top
	if width <= 0 {
		width = w.config.Width
	}
	if height <= 0 {
		height = w.config.Height
	}
	w.clientWidth, w.clientHeight = width, height

	// 创建DIB位图与兼容DC
	if !w.rebuildBackBuffer(width, height) {
		// 极端情况下（尺寸非法/内存不足）仍创建渲染器，Present 时安全跳过
		width, height = 1, 1
	}

	// 创建渲染器并直接绑定到 DIB 后备内存（Present 只需 BitBlt，无整帧拷贝）
	w.renderer = render.NewSoftwareRenderer(width, height, w.presentCallback)
	w.bindCanvasToBackBuffer()
}

// bindCanvasToBackBuffer 让画布直接使用 DIB Section 内存作为像素缓冲
func (w *Window) bindCanvasToBackBuffer() {
	if w.renderer == nil || w.pBits == nil || w.dibWidth <= 0 || w.dibHeight <= 0 {
		return
	}
	buf := unsafe.Slice(w.pBits, int(w.dibWidth)*int(w.dibHeight))
	w.renderer.UseExternalBuffer(buf, w.dibWidth, w.dibHeight)
}

// clientSize 返回当前客户区尺寸（后备缓冲可能大于客户区）。
func (w *Window) clientSize() (int32, int32) {
	rect := GetClientRect(w.hwnd)
	return rect.Right - rect.Left, rect.Bottom - rect.Top
}

// blitBackBuffer 将 DIB 中的当前帧 BitBlt 到窗口客户区（不重新光栅化）
func (w *Window) blitBackBuffer(width, height int32) {
	if w.hdcMem == 0 || w.pBits == nil || width <= 0 || height <= 0 {
		return
	}
	if width > w.dibWidth {
		width = w.dibWidth
	}
	if height > w.dibHeight {
		height = w.dibHeight
	}
	hdc := GetDC(w.hwnd)
	if hdc == 0 {
		return
	}
	defer ReleaseDC(w.hwnd, hdc)
	BitBlt(hdc, 0, 0, width, height, w.hdcMem, 0, 0, SRCCOPY)
}

// presentCallback 渲染器回调，将当前帧显示到窗口。
// 画布已直接绑定 DIB 内存时只做一次 BitBlt；绑定失败时才回退为拷贝。
func (w *Window) presentCallback(buffer []uint32, width, height int32) {
	if w.pBits == nil || len(buffer) == 0 || w.dibWidth <= 0 || w.dibHeight <= 0 {
		return
	}

	if unsafe.Pointer(&buffer[0]) != unsafe.Pointer(w.pBits) {
		// 回退路径：按交集逐行拷贝，绝不越界
		copyW := width
		copyH := height
		if copyW > w.dibWidth {
			copyW = w.dibWidth
		}
		if copyH > w.dibHeight {
			copyH = w.dibHeight
		}
		if copyW <= 0 || copyH <= 0 {
			return
		}
		dst := unsafe.Slice(w.pBits, int(w.dibWidth)*int(w.dibHeight))
		if copyW == width && copyW == w.dibWidth {
			copy(dst[:int(copyW)*int(copyH)], buffer[:int(copyW)*int(copyH)])
		} else {
			for y := int32(0); y < copyH; y++ {
				dstRow := dst[int(y)*int(w.dibWidth):]
				srcRow := buffer[int(y)*int(width):]
				copy(dstRow[:int(copyW)], srcRow[:int(copyW)])
			}
		}
	}

	if w.presentRectValid {
		w.blitBackBufferRect(w.presentRect)
	} else if w.clientWidth > 0 && w.clientHeight > 0 {
		w.blitBackBuffer(w.clientWidth, w.clientHeight)
	}
}

// blitBackBufferRect 只把后备缓冲的指定区域投递到窗口客户区。
func (w *Window) blitBackBufferRect(rect graphics.Rect) {
	if w.hdcMem == 0 || w.pBits == nil {
		return
	}
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	if rect.X < 0 {
		rect.Width += rect.X
		rect.X = 0
	}
	if rect.Y < 0 {
		rect.Height += rect.Y
		rect.Y = 0
	}
	if rect.X+rect.Width > w.clientWidth {
		rect.Width = w.clientWidth - rect.X
	}
	if rect.Y+rect.Height > w.clientHeight {
		rect.Height = w.clientHeight - rect.Y
	}
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	hdc := GetDC(w.hwnd)
	if hdc == 0 {
		return
	}
	defer ReleaseDC(w.hwnd, hdc)
	BitBlt(hdc, rect.X, rect.Y, rect.Width, rect.Height, w.hdcMem, rect.X, rect.Y, SRCCOPY)
}

// globalWndProc 全局窗口过程
func globalWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	windowMapMu.RLock()
	w, ok := windowMap[hwnd]
	windowMapMu.RUnlock()

	if !ok {
		return DefWindowProc(hwnd, msg, wParam, lParam)
	}

	return w.handleMessage(hwnd, msg, wParam, lParam)
}

// dispatchEvent 分发事件给 EventBus 和 rootComponent，并在组件消费事件时自动重绘呈现
func (w *Window) dispatchEvent(evt event.Event) {
	w.eventBus.Publish(evt)
	if w.rootComponent != nil {
		if w.rootComponent.OnEvent(evt) {
			w.renderRoot()
		}
	}
	// 组合输入过程中光标会随预编辑串移动，需同步候选窗口位置
	if w.imeComposing {
		switch evt.Type {
		case event.EventKeyDown, event.EventKeyPress, event.EventMouseDown:
			w.updateImePosition()
		}
	}
}

// updateImePosition 把输入法组合/候选窗口定位到当前编辑光标处
func (w *Window) updateImePosition() {
	if w.rootComponent == nil {
		return
	}
	if x, y, h, ok := findImeAnchor(w.rootComponent); ok {
		SetImeWindowPosition(w.hwnd, x, y, h)
	}
}

// findImeAnchor 深度优先查找当前持有输入焦点的组件光标锚点
func findImeAnchor(c component.Component) (x, y, h int32, ok bool) {
	if c == nil || !c.IsVisible() || !c.IsEnabled() {
		return 0, 0, 0, false
	}
	if provider, isProvider := c.(interface {
		ImeAnchor() (int32, int32, int32, bool)
	}); isProvider {
		if x, y, h, ok = provider.ImeAnchor(); ok {
			return x, y, h, true
		}
	}
	if container, isContainer := c.(component.Container); isContainer {
		for _, child := range container.GetChildren() {
			if x, y, h, ok := findImeAnchor(child); ok {
				return x, y, h, true
			}
		}
	}
	return 0, 0, 0, false
}

// renderRoot 渲染根组件到画布并刷新至屏幕
func (w *Window) renderRoot() {
	if w.renderer == nil {
		return
	}
	atomic.AddInt64(&w.renderCount, 1)
	if w.rootComponent != nil {
		canvas := w.renderer.BeginFrame()
		clearW, clearH := w.clientWidth, w.clientHeight
		if clearW <= 0 || clearH <= 0 {
			clearW, clearH = canvas.Width, canvas.Height
		}
		// DIB 预留了 25% 余量，只清客户区，避免每帧多清 56% 的像素
		canvas.FillRect(0, 0, clearW, clearH, w.backgroundColor)
		w.rootComponent.Render(canvas)
		w.renderer.EndFrame()
	}
	w.presentRectValid = false
	w.renderer.Present()
}

// renderRegion 只重新光栅化并投递指定矩形。
// 用于光标闪烁：清背景、绘制组件树、BitBlt 全部被裁剪在该矩形内。
func (w *Window) renderRegion(rect graphics.Rect) {
	if w.renderer == nil || w.rootComponent == nil {
		return
	}
	client := graphics.NewRect(0, 0, w.clientWidth, w.clientHeight)
	rect = rect.Intersect(client)
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	atomic.AddInt64(&w.renderCount, 1)

	canvas := w.renderer.BeginFrame()
	canvas.PushClip(rect)
	canvas.FillRect(rect.X, rect.Y, rect.Width, rect.Height, w.backgroundColor)
	w.rootComponent.Render(canvas)
	canvas.PopClip()
	w.renderer.EndFrame()

	w.presentRect = rect
	w.presentRectValid = true
	w.renderer.Present()
	w.presentRectValid = false
}

// SetContent 设置窗口的根组件（自动填充客户区并计算布局）
func (w *Window) SetContent(c component.Component) {
	w.rootComponent = c
	w.lastFocused = nil
	if c != nil {
		if th, ok := c.(component.Themeable); ok {
			th.ApplyTheme(theme.Current())
		}
		rect := GetClientRect(w.hwnd)
		width := rect.Right - rect.Left
		height := rect.Bottom - rect.Top
		if width <= 0 {
			width = w.config.Width
		}
		if height <= 0 {
			height = w.config.Height
		}
		c.SetBounds(0, 0, width, height)
		if p, ok := c.(*component.Panel); ok {
			p.DoLayout()
		}
	}
	w.renderRoot()
}

// GetContent 获取窗口的根组件
func (w *Window) GetContent() component.Component {
	return w.rootComponent
}

// Add 向窗口添加组件（若根组件为 Panel 则直接添加，否则自动创建根 Panel 并挂载）
func (w *Window) Add(c component.Component) {
	if w.rootComponent == nil {
		panel := component.NewPanel()
		panel.SetBounds(0, 0, w.config.Width, w.config.Height)
		panel.SetBackground(w.backgroundColor)
		w.SetContent(panel)
	}
	if container, ok := w.rootComponent.(component.Container); ok {
		container.Add(c)
		w.renderRoot()
	}
}

// SetBackground 设置窗口默认背景底色
func (w *Window) SetBackground(color graphics.Color) {
	w.backgroundColor = color
	w.renderRoot()
}

// SetTheme 为窗口及整棵组件树应用指定主题
func (w *Window) SetTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	w.backgroundColor = t.Background
	if w.rootComponent != nil {
		if th, ok := w.rootComponent.(component.Themeable); ok {
			th.ApplyTheme(t)
		}
	}
	w.renderRoot()
}

// ApplyThemeToAllWindows 将主题广播应用到当前所有已创建的系统窗口
func ApplyThemeToAllWindows(t *theme.Theme) {
	if t == nil {
		return
	}
	windowMapMu.RLock()
	defer windowMapMu.RUnlock()
	for _, w := range windowMap {
		w.SetTheme(t)
	}
}

// keyModifiers 读取当前键盘修饰键状态，供普通键与系统键共用。
func keyModifiers() (shift, ctrl, alt bool) {
	shift = (int32(GetKeyState(VK_SHIFT)) & 0x8000) != 0
	ctrl = (int32(GetKeyState(VK_CONTROL)) & 0x8000) != 0
	alt = (int32(GetKeyState(VK_MENU)) & 0x8000) != 0
	return
}

// mouseEventData 构造鼠标事件数据。修饰键取鼠标消息 wParam 的 MK_SHIFT/MK_CONTROL 位，
// 这是 Win32 为鼠标消息提供的规范来源，比 GetKeyState 的队列快照更可靠。
func mouseEventData(wParam uintptr, x, y int32, button event.MouseButton) event.MouseData {
	return event.MouseData{
		X: x, Y: y, Button: button,
		Shift: wParam&MK_SHIFT != 0,
		Ctrl:  wParam&MK_CONTROL != 0,
	}
}

// wheelModifiers 读取滚轮消息 wParam 低字中的 MK_SHIFT/MK_CONTROL。
// GetKeyState 反映的是队列快照，滚轮消息必须用消息自带的修饰键位。
func wheelModifiers(wParam uintptr) (ctrl, shift bool) {
	low := uint32(wParam) & 0xFFFF
	return low&MK_CONTROL != 0, low&MK_SHIFT != 0
}

// handleMessage 处理窗口消息
func (w *Window) handleMessage(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_APP_REDRAW:
		w.renderRoot()
		return 0

	case WM_PAINT:
		var ps PAINTSTRUCT
		BeginPaint(hwnd, &ps)
		// 后备缓冲保存着最近一帧，遮挡恢复时只需重新投递，不再整帧重新光栅化
		if w.clientWidth > 0 && w.clientHeight > 0 {
			w.blitBackBuffer(w.clientWidth, w.clientHeight)
		}
		EndPaint(hwnd, &ps)
		return 0

	case WM_ERASEBKGND:
		// 阻止背景擦除，防止闪烁
		return 1

	case WM_SIZE:
		width := int32(LOWORD(lParam))
		height := int32(HIWORD(lParam))
		if width > 0 && height > 0 {
			w.handleResize(width, height)
		}
		return 0

	case WM_MOUSEMOVE:
		x := int32(GET_X_LPARAM(lParam))
		y := int32(GET_Y_LPARAM(lParam))
		w.updateCursorAt(x, y)
		w.dispatchEvent(event.NewEvent(event.EventMouseMove, event.MouseData{
			X: x, Y: y,
		}))
		return 0

	case WM_LBUTTONDOWN:
		x := int32(GET_X_LPARAM(lParam))
		y := int32(GET_Y_LPARAM(lParam))
		w.dispatchEvent(event.NewEvent(event.EventMouseDown, mouseEventData(wParam, x, y, event.MouseButtonLeft)))
		SetCapture(hwnd)
		return 0

	case WM_LBUTTONUP:
		x := int32(GET_X_LPARAM(lParam))
		y := int32(GET_Y_LPARAM(lParam))
		w.dispatchEvent(event.NewEvent(event.EventMouseUp, mouseEventData(wParam, x, y, event.MouseButtonLeft)))
		ReleaseCapture()
		return 0

	case WM_LBUTTONDBLCLK:
		x := int32(GET_X_LPARAM(lParam))
		y := int32(GET_Y_LPARAM(lParam))
		w.dispatchEvent(event.NewEvent(event.EventMouseDoubleClick, mouseEventData(wParam, x, y, event.MouseButtonLeft)))
		return 0

	case WM_GETMINMAXINFO:
		// lParam 是系统传入的 MINMAXINFO 指针（Win32 消息 ABI 规定），
		// 只能从 uintptr 还原；go vet 的 unsafeptr 检查在此处无法通过静态信息证明安全性。
		if mmi := (*MINMAXINFO)(unsafe.Pointer(lParam)); mmi != nil {
			// 防止窗口被拖到极小尺寸：折行宽度趋近 0 会让大文档排版退化
			mmi.MinTrackSize = POINT{X: 640, Y: 400}
		}
		return 0

	case WM_RBUTTONDOWN:
		x := int32(GET_X_LPARAM(lParam))
		y := int32(GET_Y_LPARAM(lParam))
		w.dispatchEvent(event.NewEvent(event.EventMouseDown, mouseEventData(wParam, x, y, event.MouseButtonRight)))
		return 0

	case WM_RBUTTONUP:
		x := int32(GET_X_LPARAM(lParam))
		y := int32(GET_Y_LPARAM(lParam))
		w.dispatchEvent(event.NewEvent(event.EventMouseUp, mouseEventData(wParam, x, y, event.MouseButtonRight)))
		return 0

	case WM_MBUTTONDOWN:
		x := int32(GET_X_LPARAM(lParam))
		y := int32(GET_Y_LPARAM(lParam))
		w.dispatchEvent(event.NewEvent(event.EventMouseDown, mouseEventData(wParam, x, y, event.MouseButtonMiddle)))
		return 0

	case WM_MBUTTONUP:
		x := int32(GET_X_LPARAM(lParam))
		y := int32(GET_Y_LPARAM(lParam))
		w.dispatchEvent(event.NewEvent(event.EventMouseUp, mouseEventData(wParam, x, y, event.MouseButtonMiddle)))
		return 0

	case WM_MOUSEWHEEL:
		pt := POINT{
			X: int32(int16(LOWORD(lParam))),
			Y: int32(int16(HIWORD(lParam))),
		}
		ScreenToClient(w.hwnd, &pt)
		delta := int16(HIWORD(wParam))
		ctrl, shift := wheelModifiers(wParam)
		w.dispatchEvent(event.NewEvent(event.EventMouseWheel, event.MouseData{
			X: pt.X, Y: pt.Y, Delta: int32(delta), Ctrl: ctrl, Shift: shift,
		}))
		return 0

	case WM_KEYDOWN:
		keyCode := int(wParam)
		shift, ctrl, alt := keyModifiers()
		w.dispatchEvent(event.NewEvent(event.EventKeyDown, event.KeyData{
			KeyCode: keyCode, Shift: shift, Ctrl: ctrl, Alt: alt,
		}))
		return 0

	case WM_SYSKEYDOWN:
		keyCode := int(wParam)
		shift, ctrl, alt := keyModifiers()
		w.dispatchEvent(event.NewEvent(event.EventKeyDown, event.KeyData{
			KeyCode: keyCode, Shift: shift, Ctrl: ctrl, Alt: alt,
		}))
		// Alt+F4 显式走 WM_CLOSE 链路，确保 SetOnClose 拦截在合成消息与真实消息下
		// 都具有一致行为；Alt+Space 仍交给系统打开窗口菜单。
		if keyCode == VK_F4 {
			SendMessage(hwnd, WM_SYSCOMMAND, SC_CLOSE, 0)
			return 0
		}
		if keyCode == VK_SPACE {
			return DefWindowProc(hwnd, msg, wParam, lParam)
		}
		return 0

	case WM_SYSKEYUP:
		keyCode := int(wParam)
		shift, ctrl, alt := keyModifiers()
		w.dispatchEvent(event.NewEvent(event.EventKeyUp, event.KeyData{
			KeyCode: keyCode, Shift: shift, Ctrl: ctrl, Alt: alt,
		}))
		return 0

	case WM_SYSCHAR:
		// 菜单加速键已在 WM_SYSKEYDOWN 中处理；返回 0 抑制系统提示音。
		return 0

	case WM_KEYUP:
		keyCode := int(wParam)
		w.dispatchEvent(event.NewEvent(event.EventKeyUp, event.KeyData{
			KeyCode: keyCode,
		}))
		return 0

	case WM_CHAR:
		char := uint16(wParam)
		// WM_CHAR 以 UTF-16 码元逐个投递，代理对（Emoji、扩展 B 区汉字等）需要拼接
		if char >= 0xD800 && char <= 0xDBFF {
			w.pendingHighSurrogate = char
			return 0
		}
		if char >= 0xDC00 && char <= 0xDFFF && w.pendingHighSurrogate != 0 {
			r := utf16.DecodeRune(rune(w.pendingHighSurrogate), rune(char))
			w.pendingHighSurrogate = 0
			w.dispatchEvent(event.NewEvent(event.EventKeyPress, event.KeyData{
				Char: r,
			}))
			return 0
		}
		w.pendingHighSurrogate = 0
		w.dispatchEvent(event.NewEvent(event.EventKeyPress, event.KeyData{
			Char: rune(char),
		}))
		return 0

	case WM_SETFOCUS:
		// 只恢复窗口失焦前的那个可聚焦组件，避免焦点事件广播给整棵组件树
		w.restoreFocus()
		w.eventBus.Publish(event.NewEvent(event.EventFocus, nil))
		return 0

	case WM_KILLFOCUS:
		ReleaseCapture()
		w.rememberFocus()
		w.eventBus.Publish(event.NewEvent(event.EventBlur, nil))
		return 0

	case WM_SETCURSOR:
		if LOWORD(lParam) == HTCLIENT {
			var pt POINT
			if GetCursorPos(&pt) && ScreenToClient(hwnd, &pt) {
				w.updateCursorAt(pt.X, pt.Y)
			}
			if w.currentCursor != 0 {
				SetCursor(w.currentCursor)
				return 1
			}
		}
		return DefWindowProc(hwnd, msg, wParam, lParam)

	case WM_DPICHANGED:
		oldDpi := w.dpi
		newDpi := uint32(LOWORD(wParam))
		w.dpi = newDpi
		// lParam 是系统建议的新窗口矩形指针（Win32 消息 ABI），同上必须从 uintptr 还原。
		suggestedRect := (*RECT)(unsafe.Pointer(lParam))
		if suggestedRect != nil {
			SetWindowPos(hwnd, 0,
				suggestedRect.Left, suggestedRect.Top,
				suggestedRect.Right-suggestedRect.Left,
				suggestedRect.Bottom-suggestedRect.Top,
				SWP_NOZORDER|SWP_NOACTIVATE,
			)
		}
		if oldDpi != 0 && oldDpi != newDpi {
			w.eventBus.Publish(event.NewEvent(event.EventDpiChanged, event.DpiData{OldDPI: oldDpi, NewDPI: newDpi}))
		}
		w.renderRoot()
		return 0

	case WM_TIMER:
		if wParam == uiTaskTimerID {
			w.runDueTasks()
			return 0
		}
		// 最小化/隐藏时不做任何光栅化，避免空转消耗 CPU 与内存带宽
		if IsIconic(hwnd) || !IsWindowVisible(hwnd) {
			return 0
		}
		// 优先使用组件的局部光标矩形重绘；未实现局部接口的控件（如 TextBox）回退整窗重绘。
		if b := findCaretBlinker(w.rootComponent); b != nil {
			focused := true
			if f, ok := b.(interface{ IsFocused() bool }); ok {
				focused = f.IsFocused()
			}
			if focused {
				b.ToggleCaretBlink()
				if rect, ok := b.CaretRect(); ok {
					w.renderRegion(rect)
					return 0
				}
			}
		}
		if w.rootComponent != nil && hasBlinkingCursor(w.rootComponent) {
			w.renderRoot()
		}
		return 0

	case WM_IME_SETCONTEXT:
		// 组合串由编辑器内联绘制，屏蔽系统默认组合窗口，避免文字重影
		return DefWindowProc(hwnd, msg, wParam, lParam&^ISC_SHOWUICOMPOSITIONWINDOW)

	case WM_IME_STARTCOMPOSITION:
		w.imeComposing = true
		w.dispatchEvent(event.NewEvent(event.EventImeStart, nil))
		w.updateImePosition()
		return 0

	case WM_IME_COMPOSITION:
		hImc := ImmGetContext(hwnd)
		if hImc != 0 {
			if lParam&GCS_RESULTSTR != 0 {
				if text := GetImeCompositionString(hImc, GCS_RESULTSTR); text != "" {
					w.dispatchEvent(event.NewEvent(event.EventImeCommit, event.ImeData{Text: text}))
				}
			}
			if lParam&GCS_COMPSTR != 0 {
				w.dispatchEvent(event.NewEvent(event.EventImeUpdate, event.ImeData{
					Text: GetImeCompositionString(hImc, GCS_COMPSTR),
				}))
			}
			ImmReleaseContext(hwnd, hImc)
		}
		w.updateImePosition()
		return 0

	case WM_IME_ENDCOMPOSITION:
		w.imeComposing = false
		w.dispatchEvent(event.NewEvent(event.EventImeEnd, nil))
		return 0

	case WM_QUERYENDSESSION:
		// 系统关机/注销/重启：系统不会发送 WM_CLOSE，
		// 必须在这里复用未保存拦截逻辑，否则改动会直接丢失。
		w.mu.RLock()
		onClose := w.onClose
		w.mu.RUnlock()
		if onClose != nil && !onClose() {
			return 0 // FALSE：阻止本次会话结束
		}
		return 1 // TRUE：允许系统继续结束会话

	case WM_CLOSE:
		w.mu.RLock()
		onClose := w.onClose
		w.mu.RUnlock()
		if onClose != nil {
			if !onClose() {
				return 0 // 回调返回 false，拦截并取消关闭
			}
		}
		// 发布事件通知应用层
		w.eventBus.Publish(event.NewEvent(event.EventClose, nil))
		// 销毁窗口
		DestroyWindow(hwnd)
		return 0

	case WM_SYSCOMMAND:
		// 系统菜单/标题栏关闭与 Alt+F4 都汇聚到 WM_CLOSE，
		// 确保 SetOnClose 的未保存拦截不会被绕过。
		if wParam&0xFFF0 == SC_CLOSE {
			SendMessage(hwnd, WM_CLOSE, 0, 0)
			return 0
		}
		return DefWindowProc(hwnd, msg, wParam, lParam)

	case WM_DESTROY:
		w.cleanup()
		// 多窗口：仅当最后一个窗口销毁时才结束消息循环。
		// 独立设置窗口关闭后主窗口必须继续运行。
		if activeWindowCount() == 0 {
			PostQuitMessage(0)
		}
		return 0
	}

	return DefWindowProc(hwnd, msg, wParam, lParam)
}

// handleResize 处理窗口大小改变
func (w *Window) handleResize(width, height int32) {
	if width <= 0 || height <= 0 {
		return
	}
	if w.renderer == nil {
		return
	}
	if w.config.Width == width && w.config.Height == height {
		return
	}

	oldWidth := w.config.Width
	oldHeight := w.config.Height

	// 重建 DIB；失败时保持旧尺寸继续运行，绝不把窗口留在空白/半初始化状态。
	// DIB 只增不减，画布始终按 DIB 尺寸渲染，拖动窗口时不再反复重新分配像素内存。
	if !w.rebuildBackBuffer(width, height) {
		return
	}
	w.bindCanvasToBackBuffer()
	w.clientWidth, w.clientHeight = width, height
	w.config.Width = width
	w.config.Height = height

	// 自动调整根组件大小并重算布局
	if w.rootComponent != nil {
		w.rootComponent.SetBounds(0, 0, width, height)
		if p, ok := w.rootComponent.(*component.Panel); ok {
			p.DoLayout()
		}
	}

	// 发布大小改变事件
	w.eventBus.Publish(event.NewEvent(event.EventResize, event.ResizeData{
		Width:     width,
		Height:    height,
		OldWidth:  oldWidth,
		OldHeight: oldHeight,
	}))

	// 触发重绘并呈现
	w.renderRoot()
}

// cleanup 清理资源
func (w *Window) cleanup() {
	if !atomic.CompareAndSwapInt32(&w.closed, 0, 1) {
		return
	}
	atomic.StoreInt32(&w.running, 0)
	ReleaseCapture()

	// 停止本窗口的系统定时器
	KillTimer(w.hwnd, cursorTimerID)
	KillTimer(w.hwnd, uiTaskTimerID)
	w.taskMu.Lock()
	w.tasks = nil
	w.taskTimerOn = false
	w.taskMu.Unlock()

	// 清理GDI资源
	if w.hdcMem != 0 {
		DeleteDC(w.hdcMem)
		w.hdcMem = 0
	}
	if w.hBitmap != 0 {
		DeleteObject(w.hBitmap)
		w.hBitmap = 0
	}
	w.pBits = nil
	w.dibWidth = 0
	w.dibHeight = 0

	// 清理图标资源
	if w.customIcon != nil {
		w.customIcon.Destroy()
		w.customIcon = nil
	}

	// 恢复被模态子窗口禁用的 owner。owner 已销毁时不再操作其句柄。
	if w.ownerWin != nil {
		if w.ownerDisabled {
			w.ownerDisabled = false
			if w.ownerWin.modalChildren > 0 {
				w.ownerWin.modalChildren--
			}
			if w.ownerWin.modalChildren == 0 &&
				IsWindow(w.ownerWin.hwnd) &&
				atomic.LoadInt32(&w.ownerWin.closed) == 0 {
				EnableWindow(w.ownerWin.hwnd, true)
				SetForegroundWindow(w.ownerWin.hwnd)
				SetFocus(w.ownerWin.hwnd)
			}
		}
		w.ownerWin = nil
	}

	// 关闭事件总线
	w.eventBus.Close()

	// 从窗口映射中移除
	windowMapMu.Lock()
	delete(windowMap, w.hwnd)
	windowMapMu.Unlock()
}

// GetHandle 获取窗口句柄
func (w *Window) GetHandle() uintptr {
	return w.hwnd
}

// SetTitle 设置窗口标题
func (w *Window) SetTitle(title string) {
	w.config.Title = title
	SetWindowText(w.hwnd, title)
}

// Hwnd 获取底层 Win32 窗口句柄
func (w *Window) Hwnd() uintptr {
	return w.hwnd
}

// GetTitle 获取窗口标题
func (w *Window) GetTitle() string {
	return w.config.Title
}

// Owner 返回拥有者窗口；普通顶层窗口返回 nil。
func (w *Window) Owner() *Window {
	return w.ownerWin
}

// IsModal 返回当前窗口是否以模态方式拥有。
func (w *Window) IsModal() bool {
	return w.modal
}

// Activate 将窗口置前并设置键盘焦点。
// 独立设置窗口显示后调用，确保立即接管输入而不是停在主窗口。
func (w *Window) Activate() {
	if w.hwnd == 0 || atomic.LoadInt32(&w.closed) == 1 {
		return
	}
	SetForegroundWindow(w.hwnd)
	SetFocus(w.hwnd)
}

// SetIcon 设置窗口图标（同时更新标题栏与任务栏/Alt+Tab图标）
func (w *Window) SetIcon(icon *Icon) {
	if icon == nil || w.hwnd == 0 {
		return
	}
	if w.customIcon != nil && w.customIcon != icon {
		w.customIcon.Destroy()
	}
	w.customIcon = icon
	if icon.hIconSmall != 0 {
		SendMessage(w.hwnd, WM_SETICON, ICON_SMALL, icon.hIconSmall)
	}
	if icon.hIconBig != 0 {
		SendMessage(w.hwnd, WM_SETICON, ICON_BIG, icon.hIconBig)
	}
}

// SetIconFromBytes 从 .ico 文件二进制数据设置窗口图标
func (w *Window) SetIconFromBytes(data []byte) error {
	icon, err := NewIconFromICO(data)
	if err != nil {
		return err
	}
	w.SetIcon(icon)
	return nil
}

// SetIconFromImage 从 render.Image 动态缩放生成并设置窗口图标
func (w *Window) SetIconFromImage(img *render.Image) error {
	icon, err := NewIconFromImage(img)
	if err != nil {
		return err
	}
	w.SetIcon(icon)
	return nil
}

// CenterOnScreen 将当前窗口居中移动到主屏幕中央
func (w *Window) CenterOnScreen() {
	if w.hwnd == 0 {
		return
	}
	screenW := GetSystemMetrics(SM_CXSCREEN)
	screenH := GetSystemMetrics(SM_CYSCREEN)
	if screenW <= 0 || screenH <= 0 {
		return
	}

	rect := GetWindowRect(w.hwnd)
	winW := rect.Right - rect.Left
	winH := rect.Bottom - rect.Top

	posX := (screenW - winW) / 2
	posY := (screenH - winH) / 2
	if posX < 0 {
		posX = 0
	}
	if posY < 0 {
		posY = 0
	}

	SetWindowPos(w.hwnd, 0, posX, posY, 0, 0, SWP_NOSIZE|SWP_NOZORDER|SWP_NOACTIVATE)
}

// GetDPI 获取窗口当前屏幕的物理像素 DPI
func (w *Window) GetDPI() uint32 {
	if w.dpi == 0 {
		return 96
	}
	return w.dpi
}

// GetScale 获取窗口相对于标准 96 DPI 的物理缩放系数
func (w *Window) GetScale() float64 {
	return float64(w.GetDPI()) / 96.0
}

// GetSize 获取窗口大小
func (w *Window) GetSize() (width, height int32) {
	return w.config.Width, w.config.Height
}

// SetSize 设置客户区尺寸（自动换算为包含边框与标题栏的窗口尺寸）。
func (w *Window) SetSize(width, height int32) {
	if width <= 0 || height <= 0 {
		return
	}
	winW, winH := width, height
	rect := RECT{Left: 0, Top: 0, Right: width, Bottom: height}
	if AdjustWindowRectExForDpi(&rect, w.style, false, 0, w.GetDPI()) {
		winW = rect.Right - rect.Left
		winH = rect.Bottom - rect.Top
	}
	SetWindowPos(w.hwnd, 0, 0, 0, winW, winH, SWP_NOMOVE|SWP_NOZORDER)
}

// GetPosition 获取窗口位置
func (w *Window) GetPosition() (x, y int32) {
	rect := GetWindowRect(w.hwnd)
	return rect.Left, rect.Top
}

// SetPosition 设置窗口位置
func (w *Window) SetPosition(x, y int32) {
	w.config.X = x
	w.config.Y = y
	SetWindowPos(w.hwnd, 0, x, y, 0, 0, SWP_NOSIZE|SWP_NOZORDER)
}

// Show 显示窗口。模态子窗口在首次显示时才禁用 owner：
// 创建成功但未被显示（例如构建中途失败或用户取消）不会让主窗口失去响应。
func (w *Window) Show() {
	w.disableOwnerForModal()
	ShowWindow(w.hwnd, SW_SHOW)
	UpdateWindow(w.hwnd)
	w.renderRoot()
}

// disableOwnerForModal 在模态子窗口首次显示时禁用 owner 并登记计数。
func (w *Window) disableOwnerForModal() {
	if !w.modal || w.ownerDisabled || w.ownerWin == nil {
		return
	}
	if !IsWindow(w.ownerWin.hwnd) || atomic.LoadInt32(&w.ownerWin.closed) == 1 {
		w.ownerWin = nil
		w.modal = false
		return
	}
	w.ownerWin.modalChildren++
	w.ownerDisabled = true
	EnableWindow(w.ownerWin.hwnd, false)
}

// Hide 隐藏窗口
func (w *Window) Hide() {
	ShowWindow(w.hwnd, SW_HIDE)
}

// SetOnClose 设置窗口即将关闭时的拦截回调函数。
// 当收到系统关闭请求（标题栏关闭按钮、Alt+F4、系统菜单关闭或调用 Close()）时触发。
// 回调返回 true 表示允许关闭并销毁窗口；返回 false 表示取消并阻止窗口关闭。
func (w *Window) SetOnClose(handler func() bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onClose = handler
}

// SetLastFocused 记录窗口失焦后恢复焦点的目标，不主动改变当前焦点。
// 菜单/模态层关闭后用它把恢复目标同步回宿主窗口，避免窗口重新激活时
// 恢复到已经关闭的菜单层。
func (w *Window) SetLastFocused(c component.Component) {
	w.lastFocused = c
}

// Close 请求关闭窗口（会发送 WM_CLOSE 消息并触发 SetOnClose 拦截回调）
func (w *Window) Close() {
	if atomic.LoadInt32(&w.closed) == 1 {
		return
	}
	SendMessage(w.hwnd, WM_CLOSE, 0, 0)
}

// Destroy 强制销毁窗口（直接销毁，不触发 SetOnClose 拦截回调）
func (w *Window) Destroy() {
	if atomic.LoadInt32(&w.closed) == 1 {
		return
	}
	DestroyWindow(w.hwnd)
}

// EventBus 获取事件总线
func (w *Window) EventBus() *event.EventBus {
	return w.eventBus
}

// Renderer 获取渲染器
func (w *Window) Renderer() *render.SoftwareRenderer {
	return w.renderer
}

// Run 运行消息循环
// Run 运行消息循环。
// 必须在创建本窗口的同一个 goroutine 上调用：NewWindow 已把该 goroutine 绑定到
// 创建窗口的 OS 线程，Run 结束时解除绑定（窗口消息只会投递到创建它的线程）。
func (w *Window) Run() {
	defer runtime.UnlockOSThread()

	// 光标闪烁定时器已在 NewWindow 中按窗口安装；此处只负责泵送消息。
	var msg MSG
	for {
		ret := GetMessage(&msg, 0, 0, 0)
		if ret <= 0 { // 0 为收到 WM_QUIT，-1 为异常退出
			break
		}
		TranslateMessage(&msg)
		DispatchMessage(&msg)
	}

	w.cleanup()
}

// updateCursorAt 根据鼠标坐标更新光标图标
func (w *Window) updateCursorAt(x, y int32) {
	var curId int
	if w.rootComponent != nil {
		if c := findCursorComponent(w.rootComponent, x, y); c != nil {
			if aware, ok := c.(interface{ GetCursor() int }); ok {
				curId = aware.GetCursor()
			}
		}
	}
	var hCursor uintptr
	if curId != 0 {
		hCursor = LoadCursor(0, uintptr(curId))
	} else {
		hCursor = LoadCursor(0, IDC_ARROW)
	}
	if hCursor != w.currentCursor {
		w.currentCursor = hCursor
		SetCursor(hCursor)
	}
}

// findCursorComponent 查找鼠标位置下的最深层叶子组件
func findCursorComponent(c component.Component, x, y int32) component.Component {
	if c == nil || !c.IsVisible() || !c.IsEnabled() {
		return nil
	}
	bounds := c.GetBounds()
	if !bounds.Contains(graphics.NewPoint(x, y)) {
		return nil
	}
	if sv, ok := c.(*component.ScrollView); ok && sv.GetContent() != nil {
		if found := findCursorComponent(sv.GetContent(), x, y); found != nil {
			return found
		}
	}
	if container, ok := c.(component.Container); ok {
		child := container.GetChildAt(x, y)
		if child != nil {
			if found := findCursorComponent(child, x, y); found != nil {
				return found
			}
		}
	}
	return c
}

// rememberFocus 记录并清除当前焦点组件的焦点（窗口失活时停止光标闪烁）。
func (w *Window) rememberFocus() {
	c := findFocusedComponent(w.rootComponent)
	if c == nil {
		return
	}
	if f, ok := c.(interface{ SetFocused(bool) }); ok {
		f.SetFocused(false)
	}
	w.lastFocused = c
	w.renderRoot()
}

// restoreFocus 恢复窗口失焦前的焦点组件；组件已不可用时回退到第一个可聚焦组件。
func (w *Window) restoreFocus() {
	if c := w.lastFocused; c != nil && c.IsVisible() && c.IsEnabled() {
		if f, ok := c.(interface{ SetFocused(bool) }); ok {
			f.SetFocused(true)
			w.renderRoot()
			return
		}
	}
	if c := findFirstFocusable(w.rootComponent); c != nil {
		if f, ok := c.(interface{ SetFocused(bool) }); ok {
			f.SetFocused(true)
			w.lastFocused = c
			w.renderRoot()
		}
	}
}

// findFocusedComponent 按事件派发顺序（后进先出）查找当前持有焦点的组件。
func findFocusedComponent(c component.Component) component.Component {
	if c == nil || !c.IsVisible() || !c.IsEnabled() {
		return nil
	}
	if f, ok := c.(interface{ IsFocused() bool }); ok && f.IsFocused() {
		return c
	}
	if cont, ok := c.(component.Container); ok {
		children := cont.GetChildren()
		for i := len(children) - 1; i >= 0; i-- {
			if found := findFocusedComponent(children[i]); found != nil {
				return found
			}
		}
	}
	return nil
}

// findFirstFocusable 深度优先查找第一个可见、可用的可聚焦组件。
func findFirstFocusable(c component.Component) component.Component {
	if c == nil || !c.IsVisible() || !c.IsEnabled() {
		return nil
	}
	if cont, ok := c.(component.Container); ok {
		children := cont.GetChildren()
		for i := len(children) - 1; i >= 0; i-- {
			if found := findFirstFocusable(children[i]); found != nil {
				return found
			}
		}
	}
	if _, ok := c.(interface {
		IsFocused() bool
		SetFocused(bool)
	}); ok {
		return c
	}
	return nil
}

// hasBlinkingCursor 递归检查组件树中是否存在处于聚焦状态且可编辑的文本输入框
func hasBlinkingCursor(c component.Component) bool {
	if c == nil || !c.IsVisible() || !c.IsEnabled() {
		return false
	}
	if blinker, ok := c.(interface{ HasBlinkingCursor() bool }); ok {
		if blinker.HasBlinkingCursor() {
			return true
		}
	}
	if tb, ok := c.(*component.TextBox); ok {
		if tb.IsFocused() && !tb.IsReadOnly() {
			return true
		}
	}
	if container, ok := c.(component.Container); ok {
		for _, child := range container.GetChildren() {
			if hasBlinkingCursor(child) {
				return true
			}
		}
	}
	return false
}

// caretBlinker 由支持局部重绘的可编辑组件实现（目前是 TextArea）。
type caretBlinker interface {
	ToggleCaretBlink()
	CaretRect() (graphics.Rect, bool)
}

// findCaretBlinker 深度优先查找第一个支持局部光标重绘的组件。
func findCaretBlinker(c component.Component) caretBlinker {
	if c == nil || !c.IsVisible() || !c.IsEnabled() {
		return nil
	}
	if b, ok := c.(caretBlinker); ok {
		return b
	}
	if container, ok := c.(component.Container); ok {
		for _, child := range container.GetChildren() {
			if b := findCaretBlinker(child); b != nil {
				return b
			}
		}
	}
	return nil
}

// Stop 停止消息循环
func (w *Window) Stop() {
	atomic.StoreInt32(&w.running, 0)
	if w.hwnd != 0 {
		PostMessage(w.hwnd, WM_CLOSE, 0, 0)
	}
}

// RequestRedraw 请求重绘。UI 线程调用时立即渲染；其它 goroutine 调用时
// 通过 WM_APP_REDRAW 把渲染封送到 UI 线程，绝不跨线程写画布/DIB。
func (w *Window) RequestRedraw() {
	if w.hwnd == 0 || atomic.LoadInt32(&w.closed) == 1 {
		return
	}
	if GetCurrentThreadID() == atomic.LoadUint32(&w.uiThreadID) {
		w.renderRoot()
		return
	}
	PostMessage(w.hwnd, WM_APP_REDRAW, 0, 0)
}

// Schedule 在 UI 线程延迟执行 fn。可从任意 goroutine 调用；到期任务在窗口消息循环中
// 串行执行（与绘制、输入同线程），适合做查找输入防抖等需要访问 UI 状态的延迟动作。
func (w *Window) Schedule(delay time.Duration, fn func()) {
	if fn == nil || w.hwnd == 0 || atomic.LoadInt32(&w.closed) == 1 {
		return
	}
	w.taskMu.Lock()
	w.tasks = append(w.tasks, uiTask{at: time.Now().Add(delay), fn: fn})
	start := !w.taskTimerOn
	w.taskTimerOn = true
	w.taskMu.Unlock()
	if start {
		SetTimer(w.hwnd, uiTaskTimerID, 30, 0)
	}
}

// runDueTasks 由 WM_TIMER 在 UI 线程调用：执行到期任务，未到期任务继续等待。
func (w *Window) runDueTasks() {
	now := time.Now()
	w.taskMu.Lock()
	pending := w.tasks[:0]
	var ready []func()
	for _, task := range w.tasks {
		if task.at.After(now) {
			pending = append(pending, task)
		} else {
			ready = append(ready, task.fn)
		}
	}
	w.tasks = pending
	if len(pending) == 0 {
		w.taskTimerOn = false
		KillTimer(w.hwnd, uiTaskTimerID)
	}
	w.taskMu.Unlock()

	for _, fn := range ready {
		fn()
	}
}

// 错误定义
var (
	ErrCreateWindowFailed = &WindowError{Msg: "failed to create window"}
)

// WindowError 窗口错误
type WindowError struct {
	Msg string
}

func (e *WindowError) Error() string {
	return e.Msg
}
