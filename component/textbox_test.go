package component_test

import (
	"testing"

	"github.com/jacksalad/gnote/clipboard"
	"github.com/jacksalad/gnote/component"
	"github.com/jacksalad/gnote/event"
)

func paste(tb *component.TextBox) {
	tb.OnEvent(event.NewEvent(event.EventKeyDown, event.KeyData{KeyCode: 'V', Ctrl: true}))
}

func TestTextBoxPasteTruncatesToRemainingRoom(t *testing.T) {
	tb := component.NewTextBox()
	tb.SetFocused(true)
	tb.SetText("abc")
	tb.SetMaxLength(5)
	if !clipboard.SetText("defgh") {
		t.Skip("system clipboard unavailable")
	}

	paste(tb)

	if got, want := tb.GetText(), "abcde"; got != want {
		t.Fatalf("paste should fill remaining room: got %q want %q", got, want)
	}
}

// 回归：maxLength 小于既有文本时（SetText 不受 maxLength 约束），
// 旧实现会计算负的剩余空间并 panic（slice bounds out of range [:-5]）。
func TestTextBoxPasteWhenTextExceedsMaxLength(t *testing.T) {
	tb := component.NewTextBox()
	tb.SetFocused(true)
	tb.SetText("0123456789")
	tb.SetMaxLength(5)
	if !clipboard.SetText("x") {
		t.Skip("system clipboard unavailable")
	}

	paste(tb)

	if got, want := tb.GetText(), "0123456789"; got != want {
		t.Fatalf("paste beyond maxLength must be ignored: got %q want %q", got, want)
	}
}
