//go:build windows

package clipboard

import "testing"

func TestClipboardUnicodeRoundTrip(t *testing.T) {
	original := GetText()
	t.Cleanup(func() {
		if original != "" {
			SetText(original)
		}
	})

	const sample = "gnote 剪贴板测试 — 🚀 alpha"
	if !SetText(sample) {
		t.Skip("clipboard unavailable in this session")
	}
	if got := GetText(); got != sample {
		t.Fatalf("clipboard round trip = %q, want %q", got, sample)
	}

	if !SetText("") {
		t.Fatal("SetText(empty) failed")
	}
	if got := GetText(); got != "" {
		t.Fatalf("empty clipboard should read back empty, got %q", got)
	}
}
