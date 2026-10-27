//go:build windows

package window

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacksalad/gnote/event"
)

func drainQuitMessages() {
	var msg MSG
	for PeekMessage(&msg, 0, WM_QUIT, WM_QUIT, PM_REMOVE) {
	}
}

// 关闭多窗口中的任意一个都不得结束消息循环；只有最后一个窗口关闭才退出。
func TestWindow_DestroyOneOfTwoDoesNotQuit(t *testing.T) {
	drainQuitMessages()

	w1, err := NewWindow(WindowConfig{Title: "multi-1", Width: 240, Height: 160})
	if err != nil {
		t.Fatalf("NewWindow w1: %v", err)
	}
	defer func() {
		if IsWindow(w1.Hwnd()) {
			w1.Destroy()
		}
	}()

	w2, err := NewWindow(WindowConfig{Title: "multi-2", Width: 240, Height: 160})
	if err != nil {
		t.Fatalf("NewWindow w2: %v", err)
	}

	w2.Destroy()
	if !IsWindow(w1.Hwnd()) {
		t.Fatalf("destroying the second window must not destroy the first")
	}

	var msg MSG
	if PeekMessage(&msg, 0, WM_QUIT, WM_QUIT, PM_REMOVE) {
		t.Fatalf("destroying one of two windows must not post WM_QUIT")
	}

	w1.Destroy()
	if !PeekMessage(&msg, 0, WM_QUIT, WM_QUIT, PM_REMOVE) {
		t.Fatalf("destroying the last window should post WM_QUIT")
	}
}

// 模态设置窗口创建时禁用 owner，销毁后恢复，并正确解除 owner 关系。
func TestWindow_ModalOwnerReenabledOnDestroy(t *testing.T) {
	main, err := NewWindow(WindowConfig{Title: "owner", Width: 320, Height: 200})
	if err != nil {
		t.Fatalf("NewWindow main: %v", err)
	}
	defer func() {
		if IsWindow(main.Hwnd()) {
			main.Destroy()
		}
	}()

	if !IsWindowEnabled(main.Hwnd()) {
		t.Fatalf("new owner window should start enabled")
	}

	settings, err := NewWindow(WindowConfig{
		Title:   "settings",
		Width:   480,
		Height:  360,
		Owner:   main,
		Modal:   true,
		Visible: false,
	})
	if err != nil {
		t.Fatalf("NewWindow settings: %v", err)
	}
	if settings.Owner() != main {
		t.Fatalf("settings owner = %v, want main window", settings.Owner())
	}
	if !settings.IsModal() {
		t.Fatalf("settings should report modal")
	}
	if !IsWindowEnabled(main.Hwnd()) {
		t.Fatalf("hidden modal child must not disable its owner before Show")
	}

	settings.Show()
	if IsWindowEnabled(main.Hwnd()) {
		t.Fatalf("shown modal child must disable its owner")
	}

	settings.Destroy()
	if !IsWindowEnabled(main.Hwnd()) {
		t.Fatalf("destroying the modal child must re-enable its owner")
	}
	if settings.Owner() != nil {
		t.Fatalf("destroyed modal child should clear its owner reference")
	}
	if !IsWindow(main.Hwnd()) {
		t.Fatalf("owner must remain alive after the modal child closes")
	}
}

// Alt/F10 走 WM_SYSKEYDOWN；WM_SYSCHAR 必须被吞掉，避免系统提示音。
func TestWindow_SysKeyDownDispatched(t *testing.T) {
	w, err := NewWindow(WindowConfig{Title: "syskey", Width: 240, Height: 160, Visible: false})
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	defer w.Destroy()

	var keys []int
	w.EventBus().SubscribeHandler(event.EventKeyDown, func(evt event.Event) {
		if data, ok := evt.Data.(event.KeyData); ok {
			keys = append(keys, data.KeyCode)
		}
	})

	w.handleMessage(w.hwnd, WM_SYSKEYDOWN, VK_F10, 0)
	if len(keys) != 1 || keys[0] != VK_F10 {
		t.Fatalf("WM_SYSKEYDOWN dispatched %v, want [%d]", keys, VK_F10)
	}

	presses := 0
	w.EventBus().SubscribeHandler(event.EventKeyPress, func(event.Event) { presses++ })
	if ret := w.handleMessage(w.hwnd, WM_SYSCHAR, 'F', 0); ret != 0 {
		t.Fatalf("WM_SYSCHAR should return 0, got %d", ret)
	}
	if presses != 0 {
		t.Fatalf("WM_SYSCHAR must not dispatch EventKeyPress, got %d", presses)
	}
}

// Alt+F4 仍必须触发关闭拦截，不能被菜单系统键分发破坏。
func TestWindow_AltF4StillTriggersClose(t *testing.T) {
	w, err := NewWindow(WindowConfig{Title: "alt-f4", Width: 240, Height: 160, Visible: false})
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	defer func() {
		if IsWindow(w.Hwnd()) {
			w.Destroy()
		}
	}()

	calls := 0
	w.SetOnClose(func() bool {
		calls++
		return false // 拦截关闭，验证回调确实被触发
	})

	w.handleMessage(w.hwnd, WM_SYSKEYDOWN, VK_F4, 0)

	// DefWindowProc 可能把 Alt+F4 投递为 WM_SYSCOMMAND/WM_CLOSE，泵一轮消息确认。
	var msg MSG
	for PeekMessage(&msg, w.hwnd, 0, 0, PM_REMOVE) {
		TranslateMessage(&msg)
		DispatchMessage(&msg)
	}
	if calls != 1 {
		t.Fatalf("Alt+F4 onClose calls = %d, want 1", calls)
	}
	if !IsWindow(w.Hwnd()) {
		t.Fatalf("onClose returned false, window must stay alive")
	}
}

// 跨 goroutine 的重绘请求必须封送到 UI 线程，而不是直接写画布。
func TestRequestRedrawMarshalToUIThread(t *testing.T) {
	w, err := NewWindow(WindowConfig{Title: "redraw", Width: 200, Height: 120, Visible: true})
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	defer w.Destroy()

	before := atomic.LoadInt64(&w.renderCount)

	done := make(chan struct{})
	go func() {
		w.RequestRedraw()
		close(done)
	}()
	<-done

	if got := atomic.LoadInt64(&w.renderCount); got != before {
		t.Fatalf("off-thread RequestRedraw rendered directly (%d -> %d)", before, got)
	}

	var msg MSG
	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt64(&w.renderCount) == before && time.Now().Before(deadline) {
		if PeekMessage(&msg, w.hwnd, 0, 0, PM_REMOVE) {
			TranslateMessage(&msg)
			DispatchMessage(&msg)
		} else {
			time.Sleep(5 * time.Millisecond)
		}
	}
	if got := atomic.LoadInt64(&w.renderCount); got <= before {
		t.Fatal("posted redraw was never processed on the UI thread")
	}
}
