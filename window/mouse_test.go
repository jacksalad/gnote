//go:build windows

package window

import (
	"testing"

	"github.com/jacksalad/gnote/component"
	"github.com/jacksalad/gnote/event"
)

// TestWindow_MouseModifiersFromWParam 验证鼠标消息 wParam 的 MK_SHIFT/MK_CONTROL
// 位会进入 MouseData：Shift+点击扩展选区等组件交互完全依赖这条链路。
func TestWindow_MouseModifiersFromWParam(t *testing.T) {
	w, err := NewWindow(WindowConfig{Title: "mouse-mods", Width: 240, Height: 160, Visible: false})
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	defer w.Destroy()

	var down event.MouseData
	var downType event.EventType
	w.EventBus().SubscribeHandler(event.EventMouseDown, func(evt event.Event) {
		if data, ok := evt.Data.(event.MouseData); ok {
			down, downType = data, evt.Type
		}
	})

	lParam := uintptr(10 | 20<<16)
	w.handleMessage(w.hwnd, WM_LBUTTONDOWN, MK_SHIFT|MK_CONTROL, lParam)
	w.handleMessage(w.hwnd, WM_LBUTTONUP, 0, lParam)

	if downType != event.EventMouseDown {
		t.Fatalf("expected mouse-down event, got %v", downType)
	}
	if !down.Shift || !down.Ctrl {
		t.Fatalf("shift/ctrl must be carried from wParam, got shift=%v ctrl=%v", down.Shift, down.Ctrl)
	}
	if down.X != 10 || down.Y != 20 || down.Button != event.MouseButtonLeft {
		t.Fatalf("mouse data = (%d,%d) button=%v", down.X, down.Y, down.Button)
	}

	var dbl event.MouseData
	w.EventBus().SubscribeHandler(event.EventMouseDoubleClick, func(evt event.Event) {
		if data, ok := evt.Data.(event.MouseData); ok {
			dbl = data
		}
	})
	w.handleMessage(w.hwnd, WM_LBUTTONDBLCLK, MK_SHIFT, lParam)
	if !dbl.Shift || dbl.Ctrl {
		t.Fatalf("double-click modifiers = shift=%v ctrl=%v", dbl.Shift, dbl.Ctrl)
	}
}

func TestWheelModifiersFromMessageBits(t *testing.T) {
	cases := []struct {
		wParam uintptr
		ctrl   bool
		shift  bool
	}{
		{0, false, false},
		{uintptr(MK_CONTROL), true, false},
		{uintptr(MK_SHIFT), false, true},
		{uintptr(MK_CONTROL | MK_SHIFT), true, true},
		// 高字是滚轮增量，不应参与修饰键判断
		{uintptr(uint32(0xFF88)<<16) | uintptr(MK_SHIFT), false, true},
	}
	for _, tc := range cases {
		ctrl, shift := wheelModifiers(tc.wParam)
		if ctrl != tc.ctrl || shift != tc.shift {
			t.Fatalf("wheelModifiers(%#x) = ctrl:%v shift:%v, want %v/%v", tc.wParam, ctrl, shift, tc.ctrl, tc.shift)
		}
	}
}

// TestWindow_CloseInsideEventHandler 回归：组件事件回调中关闭窗口（设置页“取消”、
// Esc、文件菜单“退出”）后，事件分发不得继续重绘已释放的渲染目标。
func TestWindow_CloseInsideEventHandler(t *testing.T) {
	w, err := NewWindow(WindowConfig{Title: "close-in-event", Width: 240, Height: 160, Visible: false})
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	defer w.Destroy()

	panel := component.NewPanel()
	btn := component.NewButton("close")
	btn.SetBounds(10, 10, 100, 40)
	btn.OnClick(func() { w.Close() })
	panel.Add(btn)
	panel.SetBounds(0, 0, 240, 160)
	w.SetContent(panel)

	lParam := uintptr(20 | 20<<16)
	w.handleMessage(w.hwnd, WM_LBUTTONDOWN, 0, lParam)
	w.handleMessage(w.hwnd, WM_LBUTTONUP, 0, lParam)

	if IsWindow(w.hwnd) {
		t.Fatalf("window should be destroyed by the click handler")
	}
}
