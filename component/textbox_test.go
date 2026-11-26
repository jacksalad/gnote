package component

import (
	"testing"

	"github.com/jacksalad/gnote/event"
)

// paste 模拟 Ctrl+V。剪贴板文本经 readClipboard 注入：这两个用例不读写系统剪贴板，
// 不会覆盖用户当前复制的内容。
func paste(tb *TextBox, clipText string) {
	tb.readClipboard = func() string { return clipText }
	tb.OnEvent(event.NewEvent(event.EventKeyDown, event.KeyData{KeyCode: 'V', Ctrl: true}))
}

func TestTextBoxPasteTruncatesToRemainingRoom(t *testing.T) {
	tb := NewTextBox()
	tb.SetFocused(true)
	tb.SetText("abc")
	tb.SetMaxLength(5)

	paste(tb, "defgh")

	if got, want := tb.GetText(), "abcde"; got != want {
		t.Fatalf("paste should fill remaining room: got %q want %q", got, want)
	}
}

// 回归：maxLength 小于既有文本时（SetText 不受 maxLength 约束），
// 旧实现会计算负的剩余空间并 panic（slice bounds out of range [:-5]）。
func TestTextBoxPasteWhenTextExceedsMaxLength(t *testing.T) {
	tb := NewTextBox()
	tb.SetFocused(true)
	tb.SetText("0123456789")
	tb.SetMaxLength(5)

	paste(tb, "x")

	if got, want := tb.GetText(), "0123456789"; got != want {
		t.Fatalf("paste beyond maxLength must be ignored: got %q want %q", got, want)
	}
}
