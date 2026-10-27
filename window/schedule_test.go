//go:build windows

package window

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestWindow_ScheduleRunsDueTasksOnTimer(t *testing.T) {
	win, err := NewWindow(WindowConfig{Title: "Schedule Test", Width: 200, Height: 150, Visible: false})
	if err != nil {
		t.Fatalf("failed to create window: %v", err)
	}
	defer win.Destroy()

	var ran int32
	win.Schedule(0, func() { atomic.AddInt32(&ran, 1) })
	time.Sleep(5 * time.Millisecond)
	SendMessage(win.Hwnd(), WM_TIMER, uiTaskTimerID, 0)
	if atomic.LoadInt32(&ran) != 1 {
		t.Fatalf("due task ran %d times, want 1", ran)
	}

	var early int32
	win.Schedule(time.Hour, func() { atomic.AddInt32(&early, 1) })
	SendMessage(win.Hwnd(), WM_TIMER, uiTaskTimerID, 0)
	if atomic.LoadInt32(&early) != 0 {
		t.Fatalf("future task must not run early")
	}
}
