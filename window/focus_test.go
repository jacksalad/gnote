//go:build windows

package window

import (
	"testing"
	"time"
	"unsafe"

	"github.com/jacksalad/gnote/component"
	"github.com/jacksalad/gnote/event"
)

func TestWindow_FocusRestoredOnActivation(t *testing.T) {
	win, err := NewWindow(WindowConfig{Title: "Focus Test", Width: 400, Height: 300, Visible: false})
	if err != nil {
		t.Fatalf("failed to create window: %v", err)
	}
	defer win.Destroy()

	root := component.NewPanel()
	root.SetBounds(0, 0, 400, 300)
	ta := component.NewTextArea()
	ta.SetBounds(0, 0, 400, 200)
	tb := component.NewTextBox()
	tb.SetBounds(0, 210, 200, 26)
	btn := component.NewButton("btn")
	btn.SetBounds(220, 210, 80, 26)
	root.Add(ta)
	root.Add(tb)
	root.Add(btn)
	win.SetContent(root)

	ta.SetFocused(true)
	SendMessage(win.Hwnd(), WM_KILLFOCUS, 0, 0)
	if ta.IsFocused() {
		t.Fatal("editor should blur when window deactivates")
	}

	SendMessage(win.Hwnd(), WM_SETFOCUS, 0, 0)
	if !ta.IsFocused() {
		t.Fatal("editor focus should be restored on activation")
	}
	if tb.IsFocused() {
		t.Fatal("textbox must not steal restored focus")
	}
}

func TestWindow_DpiChangedEvent(t *testing.T) {
	win, err := NewWindow(WindowConfig{Title: "DPI Event Test", Width: 400, Height: 300, Visible: false})
	if err != nil {
		t.Fatalf("failed to create window: %v", err)
	}
	defer win.Destroy()

	got := make(chan event.DpiData, 1)
	win.EventBus().SubscribeHandler(event.EventDpiChanged, func(evt event.Event) {
		if data, ok := evt.Data.(event.DpiData); ok {
			got <- data
		}
	})

	old := win.GetDPI()
	newDpi := old + 48
	rect := RECT{Left: 0, Top: 0, Right: 500, Bottom: 400}
	SendMessage(win.Hwnd(), WM_DPICHANGED, uintptr(newDpi), uintptr(unsafe.Pointer(&rect)))

	select {
	case data := <-got:
		if data.OldDPI != old || data.NewDPI != newDpi {
			t.Fatalf("dpi event = %+v, want old=%d new=%d", data, old, newDpi)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("dpi change event not published")
	}
}
