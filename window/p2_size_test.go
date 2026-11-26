//go:build windows

package window

import (
	"syscall"
	"testing"
	"unsafe"

	"github.com/jacksalad/gnote/component"
)

func p2HiddenWindow(t *testing.T, cfg WindowConfig) *Window {
	t.Helper()
	cfg.Visible = false
	w, err := NewWindow(cfg)
	if err != nil {
		t.Fatalf("NewWindow(%dx%d): %v", cfg.Width, cfg.Height, err)
	}
	t.Cleanup(func() {
		if IsWindow(w.Hwnd()) {
			w.Destroy()
		}
	})
	if w.IsVisible() {
		t.Fatalf("test window must stay hidden")
	}
	return w
}

// p2SendSize 模拟系统发送的 WM_SIZE。
func p2SendSize(w *Window, state uint32, width, height int32) {
	lParam := uintptr(uint32(uint16(width)) | uint32(uint16(height))<<16)
	w.handleMessage(w.hwnd, WM_SIZE, uintptr(state), lParam)
}

func p2RectSize(r RECT) (int32, int32) { return r.Right - r.Left, r.Bottom - r.Top }

// p2AssertInside 断言外框矩形整体落在工作区内。
func p2AssertInside(t *testing.T, what string, r, work RECT) {
	t.Helper()
	if r.Left < work.Left || r.Top < work.Top || r.Right > work.Right || r.Bottom > work.Bottom {
		t.Fatalf("%s = [%d,%d]-[%d,%d] lies outside the monitor work area [%d,%d]-[%d,%d]",
			what, r.Left, r.Top, r.Right, r.Bottom, work.Left, work.Top, work.Right, work.Bottom)
	}
}

// 请求的尺寸超出屏幕时，窗口被系统限制在可达范围；GetSize() 曾原样返回请求值，
// 这个永远达不到的尺寸会被持久化并在每次启动时复现。
func TestP2_GetSizeReportsRealClientSize(t *testing.T) {
	w := p2HiddenWindow(t, WindowConfig{Title: "p2-size-oversized", Width: 12000, Height: 9000})

	client := GetClientRect(w.Hwnd())
	clientW, clientH := p2RectSize(client)
	gw, gh := w.GetSize()
	if gw != clientW || gh != clientH {
		t.Fatalf("GetSize() = %dx%d, real client area = %dx%d (requested 12000x9000)", gw, gh, clientW, clientH)
	}
	work, ok := PrimaryWorkArea()
	if !ok {
		t.Fatalf("PrimaryWorkArea failed")
	}
	p2AssertInside(t, "oversized window", GetWindowRect(w.Hwnd()), work)
	workW, workH := p2RectSize(work)
	t.Logf("requested 12000x9000: client=%dx%d GetSize()=%dx%d, primary work area=%dx%d",
		clientW, clientH, gw, gh, workW, workH)
	if clientW > workW || clientH > workH {
		t.Fatalf("client area %dx%d exceeds the monitor work area %dx%d", clientW, clientH, workW, workH)
	}
}

// 最大化时 WM_SIZE 带来的是工作区尺寸，不能覆盖还原后的正常尺寸（下次启动按它创建窗口）。
func TestP2_MaximizedSizeDoesNotOverwriteNormalSize(t *testing.T) {
	w := p2HiddenWindow(t, WindowConfig{Title: "p2-size-maximized", Width: 640, Height: 480})
	work, ok := PrimaryWorkArea()
	if !ok {
		t.Fatalf("PrimaryWorkArea failed")
	}
	if w.IsMaximized() {
		t.Fatalf("a freshly created hidden window must not report maximized")
	}

	// 最大化：客户区变成整个工作区
	p2SetMaximizedBit(t, w, true)
	workW, workH := p2RectSize(work)
	p2SendSize(w, SIZE_MAXIMIZED, workW, workH)
	if !w.IsMaximized() {
		t.Fatalf("IsMaximized() = false with WS_MAXIMIZE set")
	}
	if gw, gh := w.GetSize(); gw != 640 || gh != 480 {
		t.Fatalf("GetSize() = %dx%d while maximized, want the restored 640x480", gw, gh)
	}
	if w.clientWidth != workW || w.clientHeight != workH {
		t.Fatalf("live client size = %dx%d, want the maximized %dx%d", w.clientWidth, w.clientHeight, workW, workH)
	}

	// 还原后以还原尺寸为准
	p2SetMaximizedBit(t, w, false)
	p2SendSize(w, SIZE_RESTORED, 700, 500)
	if w.IsMaximized() {
		t.Fatalf("IsMaximized() = true after restore")
	}
	if gw, gh := w.GetSize(); gw != 700 || gh != 500 {
		t.Fatalf("GetSize() = %dx%d after restore, want 700x500", gw, gh)
	}
}

// 关闭时处于最大化：GetSize() 给 CurrentSettings 的必须是还原尺寸。
// 同时确认只改样式位、没有 WM_SIZE 时也不会把当前客户区当成正常尺寸。
func TestP2_GetSizeWhileMaximizedReturnsRestoredSize(t *testing.T) {
	w := p2HiddenWindow(t, WindowConfig{Title: "p2-size-close-maximized", Width: 1000, Height: 700})
	p2SetMaximizedBit(t, w, true)
	if gw, gh := w.GetSize(); gw != 1000 || gh != 700 {
		t.Fatalf("GetSize() = %dx%d while maximized, want the restored 1000x700", gw, gh)
	}
	p2SetMaximizedBit(t, w, false)
	if gw, gh := w.GetSize(); gw != 1000 || gh != 700 {
		t.Fatalf("GetSize() = %dx%d after clearing the maximized bit, want 1000x700", gw, gh)
	}
}

// 最小化时 WM_SIZE 的尺寸为 0x0：正常尺寸保持不变。
func TestP2_MinimizedSizeDoesNotOverwriteNormalSize(t *testing.T) {
	w := p2HiddenWindow(t, WindowConfig{Title: "p2-size-minimized", Width: 640, Height: 480})

	p2SendSize(w, SIZE_MINIMIZED, 0, 0)
	if gw, gh := w.GetSize(); gw != 640 || gh != 480 {
		t.Fatalf("GetSize() = %dx%d after a minimized WM_SIZE, want 640x480", gw, gh)
	}
}

// 两阶段显示：创建时被收敛的窗口，其根组件必须按真实客户区布局。
func TestP2_OversizedWindowLaysOutRealClientArea(t *testing.T) {
	w := p2HiddenWindow(t, WindowConfig{Title: "p2-size-content", Width: 12000, Height: 9000, Center: true})

	root := component.NewPanel()
	w.SetContent(root)
	client := GetClientRect(w.Hwnd())
	clientW, clientH := p2RectSize(client)
	if b := root.GetBounds(); b.Width != clientW || b.Height != clientH {
		t.Fatalf("root bounds = %dx%d, client = %dx%d", b.Width, b.Height, clientW, clientH)
	}
	if gw, gh := w.GetSize(); gw != clientW || gh != clientH {
		t.Fatalf("GetSize() = %dx%d, client = %dx%d", gw, gh, clientW, clientH)
	}
	work, _ := PrimaryWorkArea()
	p2AssertInside(t, "centered oversized window", GetWindowRect(w.Hwnd()), work)
}

// 居中创建的普通尺寸窗口：位置保持原有的主屏居中结果，并且整体落在工作区内。
func TestP2_CenteredWindowStaysInsideWorkArea(t *testing.T) {
	w := p2HiddenWindow(t, WindowConfig{Title: "p2-size-centered", Width: 800, Height: 500, Center: true})
	work, ok := PrimaryWorkArea()
	if !ok {
		t.Fatalf("PrimaryWorkArea failed")
	}
	p2AssertInside(t, "centered window", GetWindowRect(w.Hwnd()), work)
	if gw, gh := w.GetSize(); gw != 800 || gh != 500 {
		t.Fatalf("GetSize() = %dx%d, want the requested 800x500", gw, gh)
	}
}

// 创建时指定的位置在屏幕外（例如设置来自已拔掉的显示器）：窗口必须被移回该显示器的工作区。
func TestP2_OffscreenPositionIsClampedAtCreation(t *testing.T) {
	for _, tc := range []struct {
		name string
		x, y int32
	}{
		{"far top-left", -5000, -5000},
		{"far bottom-right", 5000, 5000},
		{"int32 extremes", 2147483647, -2147483648},
	} {
		w := p2HiddenWindow(t, WindowConfig{Title: "p2-size-pos", X: tc.x, Y: tc.y, Width: 400, Height: 300})
		work, ok := WorkAreaNear(tc.x, tc.y)
		if !ok {
			t.Fatalf("%s: WorkAreaNear failed", tc.name)
		}
		p2AssertInside(t, tc.name, GetWindowRect(w.Hwnd()), work)
	}
}

// 请求的位置只越出一点：做最小位移，窗口保持在原来的那一侧。
func TestP2_SlightlyOffscreenPositionIsShiftedMinimally(t *testing.T) {
	work, ok := PrimaryWorkArea()
	if !ok {
		t.Fatalf("PrimaryWorkArea failed")
	}
	workW, workH := p2RectSize(work)
	if workW < 1000 || workH < 800 {
		t.Skipf("work area %dx%d too small for this test", workW, workH)
	}
	w := p2HiddenWindow(t, WindowConfig{
		Title: "p2-size-shift", X: work.Right - 100, Y: work.Bottom - 50, Width: 400, Height: 300,
	})
	r := GetWindowRect(w.Hwnd())
	p2AssertInside(t, "shifted window", r, work)
	if r.Right != work.Right || r.Bottom != work.Bottom {
		t.Fatalf("window = [%d,%d]-[%d,%d], want it flush with the work area corner [%d,%d]", r.Left, r.Top, r.Right, r.Bottom, work.Right, work.Bottom)
	}
}

// WM_GETMINMAXINFO 的最小尺寸随窗口 DPI 缩放（高分屏上 640x400 物理像素装不下同等逻辑内容），
// 且不超过所在显示器的工作区（远程桌面/小屏上过大的下限会让窗口无法完整落在可见范围）。
func TestP2_MinTrackSizeScalesWithDPI(t *testing.T) {
	w := p2HiddenWindow(t, WindowConfig{Title: "p2-minmax", Width: 900, Height: 600})

	scale := float64(w.GetDPI()) / 96
	wantW := int32(float64(640)*scale + 0.5)
	wantH := int32(float64(400)*scale + 0.5)
	work, ok := PrimaryWorkArea()
	if !ok {
		t.Fatalf("PrimaryWorkArea failed")
	}
	workW, workH := p2RectSize(work)
	wantW, wantH = min(wantW, workW), min(wantH, workH)

	// lParam 借测试传入的结构必须是包级变量：(*T)(unsafe.Pointer(uintptr)) 没有“原始指针”，
	// checkptr（-race 默认开启）发现还原出的地址落在 Go 的堆或栈分配里就判非法，
	// 局部变量会让本包无法用 -race 检测。真实消息的 lParam 指向系统内存，不受影响。
	p2MinMaxInfo = MINMAXINFO{MinTrackSize: POINT{X: 1, Y: 1}}
	w.handleMessage(w.hwnd, WM_GETMINMAXINFO, 0, uintptr(unsafe.Pointer(&p2MinMaxInfo)))
	mmi := &p2MinMaxInfo
	if mmi.MinTrackSize.X != wantW || mmi.MinTrackSize.Y != wantH {
		t.Fatalf("MinTrackSize = %dx%d at DPI %d, want %dx%d (640x400 scaled and capped to the work area %dx%d)",
			mmi.MinTrackSize.X, mmi.MinTrackSize.Y, w.GetDPI(), wantW, wantH, workW, workH)
	}
}

var (
	p2User32           = syscall.NewLazyDLL("user32.dll")
	p2GetWindowLongPtr = p2User32.NewProc("GetWindowLongPtrW")
	p2SetWindowLongPtr = p2User32.NewProc("SetWindowLongPtrW")
	p2MinMaxInfo       MINMAXINFO // WM_GETMINMAXINFO 测试借 lParam 传入，必须是包级变量
)

// p2SetMaximizedBit 在隐藏窗口上置位/清除 WS_MAXIMIZE（IsZoomed 只看这个样式位），
// 用来在不显示窗口的前提下模拟最大化状态；测试结束时恢复原样式。
func p2SetMaximizedBit(t *testing.T, w *Window, on bool) {
	t.Helper()
	gwlStyle := -16 // GWL_STYLE
	style, _, _ := p2GetWindowLongPtr.Call(w.Hwnd(), uintptr(gwlStyle))
	want := style | uintptr(WS_MAXIMIZE)
	if !on {
		want = style &^ uintptr(WS_MAXIMIZE)
	}
	p2SetWindowLongPtr.Call(w.Hwnd(), uintptr(gwlStyle), want)
	t.Cleanup(func() { p2SetWindowLongPtr.Call(w.Hwnd(), uintptr(gwlStyle), style) })
	if got, _, _ := p2GetWindowLongPtr.Call(w.Hwnd(), uintptr(gwlStyle)); got != want {
		t.Fatalf("failed to toggle WS_MAXIMIZE: style=%#x want %#x", got, want)
	}
}
