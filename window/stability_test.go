//go:build windows

package window

import (
	"sync"
	"testing"

	"github.com/jacksalad/gnote/event"
)

func TestWindow_BackBufferRejectsOversizeWithoutBreakingState(t *testing.T) {
	w, err := NewWindow(WindowConfig{Title: "buffer-guard", Width: 320, Height: 200})
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	defer w.Destroy()

	oldW, oldH := w.dibWidth, w.dibHeight
	oldBits := w.pBits
	if oldBits == nil || oldW <= 0 || oldH <= 0 {
		t.Fatalf("initial back buffer not initialized: %dx%d bits=%v", oldW, oldH, oldBits != nil)
	}

	if w.rebuildBackBuffer(40000, 100) {
		t.Fatalf("expected oversize back buffer allocation to be rejected")
	}
	if w.dibWidth != oldW || w.dibHeight != oldH || w.pBits != oldBits {
		t.Fatalf("rejected resize must preserve the previous back buffer")
	}

	// 尺寸不匹配的 Present 必须只拷贝交集，绝不越界
	w.presentCallback(make([]uint32, 10*10), 10, 10)
	w.presentCallback(make([]uint32, 2000*2000), 2000, 2000)
}

func TestWindow_WMCharSurrogatePair(t *testing.T) {
	w, err := NewWindow(WindowConfig{Title: "surrogate", Width: 200, Height: 120})
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	defer w.Destroy()

	var mu sync.Mutex
	var chars []rune
	w.EventBus().SubscribeHandler(event.EventKeyPress, func(evt event.Event) {
		if data, ok := evt.Data.(event.KeyData); ok {
			mu.Lock()
			chars = append(chars, data.Char)
			mu.Unlock()
		}
	})

	const high, low = 0xD83D, 0xDE00 // U+1F600
	w.handleMessage(w.hwnd, WM_CHAR, uintptr(high), 0)
	mu.Lock()
	if len(chars) != 0 {
		mu.Unlock()
		t.Fatalf("high surrogate must not be dispatched on its own")
	}
	mu.Unlock()

	w.handleMessage(w.hwnd, WM_CHAR, uintptr(low), 0)
	mu.Lock()
	defer mu.Unlock()
	if len(chars) != 1 || chars[0] != '😀' {
		t.Fatalf("expected combined rune U+1F600, got %v", chars)
	}
}
