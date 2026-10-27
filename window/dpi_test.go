//go:build windows

package window

import (
	"testing"
)

func TestDpiDiagnostics(t *testing.T) {
	EnableDPIAwareness()
	dpi := GetDpiForSystem()
	if dpi == 0 {
		t.Fatalf("expected non-zero DPI, got 0")
	}

	rect := RECT{Left: 0, Top: 0, Right: 800, Bottom: 600}
	ok := AdjustWindowRectExForDpi(&rect, WS_OVERLAPPEDWINDOW, false, 0, dpi)
	if !ok {
		t.Fatalf("AdjustWindowRectExForDpi failed")
	}
	if rect.Right-rect.Left <= 800 || rect.Bottom-rect.Top <= 600 {
		t.Fatalf("expected adjusted window size to include frame, got %dx%d", rect.Right-rect.Left, rect.Bottom-rect.Top)
	}
}
