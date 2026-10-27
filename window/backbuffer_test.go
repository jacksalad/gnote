//go:build windows

package window

import (
	"sync/atomic"
	"testing"

	"github.com/jacksalad/gnote/component"
	"github.com/jacksalad/gnote/graphics"
)

func TestWindow_BackBufferShrinkAndGrow(t *testing.T) {
	win, err := NewWindow(WindowConfig{Title: "BackBuffer Test", Width: 400, Height: 300, Visible: false})
	if err != nil {
		t.Fatalf("failed to create window: %v", err)
	}
	defer win.Destroy()

	if win.dibWidth < 400 || win.dibHeight < 300 {
		t.Fatalf("initial buffer %dx%d does not cover client area", win.dibWidth, win.dibHeight)
	}

	// 客户区显著变小（面积不足一半）：收缩，避免继续按历史最大尺寸清屏/呈现
	if !win.rebuildBackBuffer(100, 100) {
		t.Fatal("shrink rebuild failed")
	}
	if win.dibWidth != 100 || win.dibHeight != 100 {
		t.Fatalf("expected shrink to 100x100, got %dx%d", win.dibWidth, win.dibHeight)
	}

	// 放大时预留 25% 余量
	if !win.rebuildBackBuffer(500, 500) {
		t.Fatal("grow rebuild failed")
	}
	if win.dibWidth != 625 || win.dibHeight != 625 {
		t.Fatalf("expected padded grow to 625x625, got %dx%d", win.dibWidth, win.dibHeight)
	}

	// 同一尺寸重复请求不应重建
	if !win.rebuildBackBuffer(500, 500) {
		t.Fatal("same-size rebuild failed")
	}
	if win.dibWidth != 625 || win.dibHeight != 625 {
		t.Fatalf("same-size request changed buffer to %dx%d", win.dibWidth, win.dibHeight)
	}
}

func TestWindow_SetSizeUsesClientArea(t *testing.T) {
	win, err := NewWindow(WindowConfig{Title: "SetSize Test", Width: 400, Height: 300, Visible: false})
	if err != nil {
		t.Fatalf("failed to create window: %v", err)
	}
	defer win.Destroy()

	win.SetSize(800, 600)
	if w, h := win.GetSize(); w != 800 || h != 600 {
		t.Fatalf("GetSize = %dx%d, want 800x600", w, h)
	}
	rect := GetClientRect(win.Hwnd())
	if gotW, gotH := rect.Right-rect.Left, rect.Bottom-rect.Top; gotW != 800 || gotH != 600 {
		t.Fatalf("client area = %dx%d, want 800x600", gotW, gotH)
	}
}

func TestWindow_RenderRegionOnlyTouchesRect(t *testing.T) {
	win, err := NewWindow(WindowConfig{Title: "region", Width: 40, Height: 30, Visible: false})
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	defer win.Destroy()

	panel := component.NewPanel()
	panel.SetBounds(0, 0, 40, 30)
	win.SetContent(panel)

	oldColor := graphics.RGB(10, 20, 30)
	panel.SetBackground(oldColor)
	win.renderRoot()

	canvas := win.renderer.GetCanvas()
	pixel := func(x, y int32) graphics.Color {
		return graphics.Color(canvas.Buffer[int(y)*int(canvas.Width)+int(x)])
	}
	if got := pixel(20, 20); got != oldColor {
		t.Fatalf("initial pixel = %08X, want %08X", uint32(got), uint32(oldColor))
	}

	before := atomic.LoadInt64(&win.renderCount)
	newColor := graphics.RGB(200, 100, 50)
	panel.SetBackground(newColor)
	win.renderRegion(graphics.NewRect(5, 5, 4, 4))

	if got := pixel(6, 6); got != newColor {
		t.Fatalf("inside pixel = %08X, want %08X", uint32(got), uint32(newColor))
	}
	if got := pixel(20, 20); got != oldColor {
		t.Fatalf("outside pixel changed to %08X", uint32(got))
	}
	if got := pixel(4, 4); got != oldColor {
		t.Fatalf("left edge outside region changed to %08X", uint32(got))
	}
	if atomic.LoadInt64(&win.renderCount) != before+1 {
		t.Fatal("renderRegion should render exactly once")
	}
}

// 光标定时器应对已聚焦编辑器走局部重绘，而不是整窗重绘。
func TestWindow_CaretTimerUsesRegionRender(t *testing.T) {
	win, err := NewWindow(WindowConfig{Title: "caret-region", Width: 320, Height: 200, Visible: true})
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	defer win.Destroy()

	ta := component.NewTextArea()
	ta.SetText("hello world\r\nsecond line")
	ta.SetFocused(true)
	ta.SetBounds(0, 0, 320, 200)
	win.SetContent(ta)
	win.renderRoot()

	before := atomic.LoadInt64(&win.renderCount)
	win.handleMessage(win.hwnd, WM_TIMER, cursorTimerID, 0)
	if got := atomic.LoadInt64(&win.renderCount); got != before+1 {
		t.Fatalf("caret timer rendered %d times, want 1", got-before)
	}
	if _, ok := ta.CaretRect(); !ok {
		t.Fatal("focused editor should expose a caret rect after timer tick")
	}
}
