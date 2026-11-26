//go:build windows

package window

import (
	"testing"

	"github.com/jacksalad/gnote/event"
)

// p2AltGr 是 Windows 对 AltGr 的按键报告：左 Ctrl + 右 Alt 同时按下，左 Alt 与右 Ctrl 未按下。
var p2AltGr = modifierState{lCtrl: true, rAlt: true}

// p2WithModifiers 在测试内注入修饰键状态，结束后恢复为读取真实键盘。
func p2WithModifiers(t *testing.T, st modifierState) {
	t.Helper()
	prev := currentModifiers
	currentModifiers = func() modifierState { return st }
	t.Cleanup(func() { currentModifiers = prev })
}

func p2AltGrWindow(t *testing.T, title string) *Window {
	t.Helper()
	w, err := NewWindow(WindowConfig{Title: title, Width: 240, Height: 160, Visible: false})
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	t.Cleanup(func() {
		if IsWindow(w.Hwnd()) {
			w.Destroy()
		}
	})
	return w
}

// 修饰键换算：AltGr（左 Ctrl + 右 Alt）不得被当成 Ctrl+Alt，其余组合保持原样。
func TestP2_ModifiersAltGrIsNotCtrlAlt(t *testing.T) {
	cases := []struct {
		name             string
		state            modifierState
		shift, ctrl, alt bool
	}{
		{"no modifiers", modifierState{}, false, false, false},
		{"left Ctrl", modifierState{lCtrl: true}, false, true, false},
		{"right Ctrl", modifierState{rCtrl: true}, false, true, false},
		{"left Shift", modifierState{lShift: true}, true, false, false},
		{"left Alt", modifierState{lAlt: true}, false, false, true},
		{"right Alt alone", modifierState{rAlt: true}, false, false, true},
		{"AltGr = left Ctrl + right Alt", p2AltGr, false, false, false},
		{"Shift + AltGr", modifierState{lShift: true, lCtrl: true, rAlt: true}, true, false, false},
		{"left Ctrl + left Alt (real Ctrl+Alt)", modifierState{lCtrl: true, lAlt: true}, false, true, true},
		{"right Ctrl + right Alt", modifierState{rCtrl: true, rAlt: true}, false, true, true},
		{"left Ctrl + left Alt + right Alt", modifierState{lCtrl: true, lAlt: true, rAlt: true}, false, true, true},
	}
	for _, tc := range cases {
		shift, ctrl, alt := tc.state.modifiers()
		if shift != tc.shift || ctrl != tc.ctrl || alt != tc.alt {
			t.Errorf("%s: modifiers = shift:%v ctrl:%v alt:%v, want shift:%v ctrl:%v alt:%v",
				tc.name, shift, ctrl, alt, tc.shift, tc.ctrl, tc.alt)
		}
	}
}

// AltGr+字母若以 WM_SYSKEYDOWN 到达，派发出去的 KeyDown 不能带 Ctrl/Alt：
// 应用层按 Alt+字母打开菜单，编辑器按 Ctrl+字母执行快捷键，字符会被吞掉。
func TestP2_AltGrSysKeyDownCarriesNoCtrlAlt(t *testing.T) {
	w := p2AltGrWindow(t, "p2-altgr-keydown")

	var keys []event.KeyData
	w.EventBus().SubscribeHandler(event.EventKeyDown, func(evt event.Event) {
		if data, ok := evt.Data.(event.KeyData); ok {
			keys = append(keys, data)
		}
	})

	p2WithModifiers(t, p2AltGr)
	w.handleMessage(w.hwnd, WM_SYSKEYDOWN, 'E', 0)
	w.handleMessage(w.hwnd, WM_KEYDOWN, 'E', 0)
	if len(keys) != 2 {
		t.Fatalf("key events dispatched = %d, want 2", len(keys))
	}
	for i, key := range keys {
		if key.KeyCode != 'E' || key.Ctrl || key.Alt || key.Shift {
			t.Fatalf("AltGr+E dispatched as %+v (event %d): Ctrl/Alt must not be reported", key, i)
		}
	}

	// 真正的左 Alt / Ctrl+Alt 保持原样
	p2WithModifiers(t, modifierState{lAlt: true})
	w.handleMessage(w.hwnd, WM_SYSKEYDOWN, 'E', 0)
	p2WithModifiers(t, modifierState{lCtrl: true, lAlt: true, lShift: true})
	w.handleMessage(w.hwnd, WM_SYSKEYDOWN, 'E', 0)
	if len(keys) != 4 {
		t.Fatalf("key events dispatched = %d, want 4", len(keys))
	}
	if key := keys[2]; !key.Alt || key.Ctrl || key.Shift {
		t.Fatalf("left Alt+E dispatched as %+v, want Alt only", key)
	}
	if key := keys[3]; !key.Alt || !key.Ctrl || !key.Shift {
		t.Fatalf("Ctrl+Alt+Shift+E dispatched as %+v, want all modifiers", key)
	}
}

// AltGr 字符若以 WM_SYSCHAR 到达，它是正文输入而不是菜单加速键：
// 必须派发为 KeyPress，且返回 0 不交给 DefWindowProc（否则无匹配助记键时系统鸣叫）。
func TestP2_AltGrSysCharIsDeliveredAsText(t *testing.T) {
	w := p2AltGrWindow(t, "p2-altgr-syschar")

	var chars []rune
	w.EventBus().SubscribeHandler(event.EventKeyPress, func(evt event.Event) {
		if data, ok := evt.Data.(event.KeyData); ok {
			chars = append(chars, data.Char)
		}
	})

	p2WithModifiers(t, p2AltGr)
	for _, ch := range []rune{'€', '@', 'ą', ' ', 0x00A0} {
		if ret := w.handleMessage(w.hwnd, WM_SYSCHAR, uintptr(ch), 0); ret != 0 {
			t.Fatalf("WM_SYSCHAR %q returned %d, want 0", ch, ret)
		}
	}
	want := []rune{'€', '@', 'ą', ' ', 0x00A0}
	if string(chars) != string(want) {
		t.Fatalf("AltGr characters carried by WM_SYSCHAR = %q, want %q", string(chars), string(want))
	}

	// 真正的 Alt+字母：不是正文输入，照旧吞掉且不派发
	p2WithModifiers(t, modifierState{lAlt: true})
	if ret := w.handleMessage(w.hwnd, WM_SYSCHAR, 'E', 0); ret != 0 {
		t.Fatalf("WM_SYSCHAR under a real Alt returned %d, want 0", ret)
	}
	if len(chars) != len(want) {
		t.Fatalf("Alt+E must not be delivered as text, got %q", string(chars))
	}
}

// AltGr+F4 是 European 布局上的按键组合，不能当作关闭快捷键触发 WM_CLOSE / SetOnClose 拦截。
// （AltGr+空格走同一个判断，不交给 DefWindowProc 弹出系统菜单；真实弹出无法在测试里安全验证。）
func TestP2_AltGrF4DoesNotCloseWindow(t *testing.T) {
	w := p2AltGrWindow(t, "p2-altgr-f4")

	calls := 0
	w.SetOnClose(func() bool {
		calls++
		return false
	})

	p2WithModifiers(t, p2AltGr)
	w.handleMessage(w.hwnd, WM_SYSKEYDOWN, VK_F4, 0)
	var msg MSG
	for PeekMessage(&msg, w.hwnd, 0, 0, PM_REMOVE) {
		DispatchMessage(&msg)
	}
	if calls != 0 {
		t.Fatalf("AltGr+F4 went down the close path: onClose calls = %d, want 0", calls)
	}
	if !IsWindow(w.Hwnd()) {
		t.Fatalf("window must survive AltGr+F4")
	}
}

// AltGr 按住时鼠标消息的 MK_CONTROL 位来自 AltGr 自带的左 Ctrl：不能被当成 Ctrl（Ctrl+滚轮缩放等）。
func TestP2_AltGrDoesNotReportCtrlOnMouseMessages(t *testing.T) {
	p2WithModifiers(t, p2AltGr)
	if data := mouseEventData(MK_CONTROL|MK_SHIFT, 10, 20, event.MouseButtonLeft); data.Ctrl || !data.Shift {
		t.Fatalf("mouse data under AltGr = ctrl:%v shift:%v, want ctrl:false shift:true", data.Ctrl, data.Shift)
	}
	if ctrl, _ := wheelModifiers(uintptr(MK_CONTROL)); ctrl {
		t.Fatalf("wheel modifiers under AltGr report Ctrl")
	}
	// 真正的 Ctrl 保持原样
	p2WithModifiers(t, modifierState{lCtrl: true})
	if data := mouseEventData(MK_CONTROL, 10, 20, event.MouseButtonLeft); !data.Ctrl || data.Shift {
		t.Fatalf("mouse data under Ctrl = ctrl:%v shift:%v, want ctrl:true shift:false", data.Ctrl, data.Shift)
	}
	if ctrl, _ := wheelModifiers(uintptr(MK_CONTROL)); !ctrl {
		t.Fatalf("wheel modifiers under a real Ctrl must report Ctrl")
	}
}
