package component

import "testing"

// 单行输入框（如查找框）粘贴多行文本：换行一律去掉，拼接成一行，光标落在粘贴内容之后。
// FindNext 是按行匹配的，带着换行的查找内容永远匹配不到，光标还会画到唯一一行的下面。
func TestTextBox_PasteStripsLineBreaksByDefault(t *testing.T) {
	tb := NewTextBox()
	tb.SetFocused(true)
	tb.SetText("x")
	tb.pasteText("foo\r\nbar\nbaz\rqux")
	if got, want := tb.GetText(), "xfoobarbazqux"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
	if tb.cursorPos != len([]rune("xfoobarbazqux")) {
		t.Errorf("cursor = %d, want after the pasted text", tb.cursorPos)
	}
}

// 允许换行的输入框（替换框）：保留换行，但 CRLF / CR 统一成 LF；多行替换是受支持的功能。
func TestTextBox_PasteKeepsLineBreaksWhenAllowed(t *testing.T) {
	tb := NewTextBox()
	tb.SetFocused(true)
	tb.SetKeepLineBreaks(true)
	tb.pasteText("x\r\ny\rz\nw")
	if got, want := tb.GetText(), "x\ny\nz\nw"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}

// 程序写入的文本走与粘贴相同的规则，输入框不会因为来源不同而混进裸 CR。
func TestTextBox_SetTextFollowsLineBreakPolicy(t *testing.T) {
	plain := NewTextBox()
	plain.SetText("a\r\nb\rc")
	if got := plain.GetText(); got != "abc" {
		t.Errorf("default text box: %q, want line breaks stripped", got)
	}
	multi := NewTextBox()
	multi.SetKeepLineBreaks(true)
	multi.SetText("a\r\nb\rc")
	if got := multi.GetText(); got != "a\nb\nc" {
		t.Errorf("line-break text box: %q, want CRLF/CR normalised to LF", got)
	}
}

// 去换行之后剩下的内容仍受 maxLength 约束。
func TestTextBox_PasteStillHonoursMaxLength(t *testing.T) {
	tb := NewTextBox()
	tb.SetFocused(true)
	tb.SetMaxLength(5)
	tb.pasteText("a\r\nbcdefg")
	if got, want := tb.GetText(), "abcde"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}
