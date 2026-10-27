//go:build windows

package window

import (
	"sync/atomic"
	"testing"
)

func TestWindow_SetOnCloseIntercept(t *testing.T) {
	// 1. 创建不可见测试窗口
	win, err := NewWindow(WindowConfig{
		Title:   "Close Intercept Test",
		Width:   400,
		Height:  300,
		Visible: false,
	})
	if err != nil {
		t.Fatalf("failed to create window: %v", err)
	}

	hwnd := win.Hwnd()
	if !IsWindow(hwnd) {
		t.Fatalf("expected valid window handle")
	}

	// 2. 注册阻止关闭的回调
	var interceptCount int32
	win.SetOnClose(func() bool {
		atomic.AddInt32(&interceptCount, 1)
		return false // 拒绝关闭
	})

	// 3. 发送关闭请求
	win.Close()

	if atomic.LoadInt32(&interceptCount) != 1 {
		t.Fatalf("expected onClose called once, got %d", interceptCount)
	}
	if !IsWindow(hwnd) {
		t.Fatalf("window should NOT be destroyed when onClose returns false")
	}

	// 4. 改为允许关闭的回调
	var allowCount int32
	win.SetOnClose(func() bool {
		atomic.AddInt32(&allowCount, 1)
		return true // 允许关闭
	})

	// 5. 再次请求关闭
	win.Close()

	if atomic.LoadInt32(&allowCount) != 1 {
		t.Fatalf("expected onClose called once, got %d", allowCount)
	}
	if IsWindow(hwnd) {
		t.Fatalf("window SHOULD be destroyed when onClose returns true")
	}
}

// 系统关机/注销发送 WM_QUERYENDSESSION 而不是 WM_CLOSE，必须复用未保存拦截。
func TestWindow_QueryEndSessionIntercept(t *testing.T) {
	win, err := NewWindow(WindowConfig{
		Title:   "Session End Test",
		Width:   400,
		Height:  300,
		Visible: false,
	})
	if err != nil {
		t.Fatalf("failed to create window: %v", err)
	}
	defer win.Destroy()

	var calls int32
	win.SetOnClose(func() bool {
		atomic.AddInt32(&calls, 1)
		return false // 取消关机
	})
	if ret := SendMessage(win.Hwnd(), WM_QUERYENDSESSION, 0, 0); ret != 0 {
		t.Fatalf("expected FALSE (block session end), got %d", ret)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("onClose called %d times, want 1", calls)
	}

	win.SetOnClose(func() bool {
		atomic.AddInt32(&calls, 1)
		return true // 允许关机
	})
	if ret := SendMessage(win.Hwnd(), WM_QUERYENDSESSION, 0, 0); ret != 1 {
		t.Fatalf("expected TRUE (allow session end), got %d", ret)
	}
}
