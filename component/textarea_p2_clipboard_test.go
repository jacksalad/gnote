package component

import "testing"

// 回归：Copy 曾忽略 clipboard.SetText 的结果，只要有选区就返回 true，Cut 于是先删掉了选区。
// 剪贴板被剪贴板管理器/远程桌面/同步工具占用时 OpenClipboard 会失败，剪切就成了纯删除。
// 通过可注入的复制实现模拟写入失败，全程不触碰真实剪贴板。
func TestTextArea_CutKeepsSelectionWhenClipboardWriteFails(t *testing.T) {
	ta := newTestTextArea("hello world")
	ta.SetSelection(0, 0, 0, 5)
	var copied string
	ta.copyToClipboard = func(text string) bool {
		copied = text
		return false // 模拟 OpenClipboard 重试后仍然失败
	}

	if ta.Copy() {
		t.Error("Copy must report the failed clipboard write")
	}
	if copied != "hello" {
		t.Errorf("Copy handed %q to the clipboard", copied)
	}

	ta.SetSelection(0, 0, 0, 5)
	if ta.Cut() {
		t.Error("Cut must report failure when the clipboard write failed")
	}
	if got := ta.GetTextWithEOL("\n"); got != "hello world" {
		t.Errorf("Cut removed the selection although the clipboard write failed: %q", got)
	}
	if ta.CanUndo() {
		t.Error("a failed Cut must not create an undo step")
	}
	if sr, sc, er, ec, ok := ta.GetSelectionRange(); !ok || sr != 0 || sc != 0 || er != 0 || ec != 5 {
		t.Errorf("a failed Cut must leave the selection intact: %d,%d-%d,%d ok=%v", sr, sc, er, ec, ok)
	}

	// 快捷键路径走同一个 Cut：Ctrl+X 同样不得删除
	pressKey(ta, 'X', false, true)
	if got := ta.GetTextWithEOL("\n"); got != "hello world" {
		t.Errorf("Ctrl+X removed the selection although the clipboard write failed: %q", got)
	}

	// 写入恢复正常后复制、剪切照常工作
	ta.copyToClipboard = func(text string) bool {
		copied = text
		return true
	}
	ta.SetSelection(0, 0, 0, 5)
	if !ta.Copy() || copied != "hello" {
		t.Errorf("Copy with a healthy clipboard returned false or copied %q", copied)
	}
	if !ta.Cut() || copied != "hello" {
		t.Errorf("Cut with a healthy clipboard returned false or copied %q", copied)
	}
	if got := ta.GetTextWithEOL("\n"); got != " world" {
		t.Errorf("text after a successful Cut = %q, want %q", got, " world")
	}
}

// Copy 的返回值必须反映真实结果；无选区时不触碰剪贴板。
func TestTextArea_CopyReportsClipboardOutcome(t *testing.T) {
	ta := newTestTextArea("l1\r\nl2")
	calls := 0
	ta.copyToClipboard = func(string) bool {
		calls++
		return true
	}
	if ta.Copy() || calls != 0 {
		t.Fatalf("Copy without a selection: calls=%d", calls)
	}

	// 多行选区按文档的换行风格输出（LF 文件不被迫改成 CRLF）
	ta.SetLineEnding("\n")
	ta.SetSelection(0, 1, 1, 1)
	var copied string
	ta.copyToClipboard = func(text string) bool {
		copied = text
		return true
	}
	if !ta.Copy() || copied != "1\nl" {
		t.Fatalf("Copy copied %q, want %q", copied, "1\nl")
	}
}
