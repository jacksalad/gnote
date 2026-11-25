package component

import (
	"strings"
	"testing"
)

func TestTextArea_ReplaceDocumentIsSingleUndoStep(t *testing.T) {
	ta := newTestTextArea("one\ntwo\nthree")
	ta.SetSelection(2, 0, 2, 5)

	if !ta.ReplaceDocument("first\nsecond") {
		t.Fatal("ReplaceDocument must apply on a writable document")
	}
	if got := ta.GetText(); got != "first\r\nsecond" {
		t.Fatalf("unexpected text: %q", got)
	}
	if !ta.IsModified() {
		t.Fatal("ReplaceDocument must mark the document modified")
	}
	if r, c := ta.GetCursorPosition(); r != 2 || c != 6 {
		t.Fatalf("cursor should clamp to the new end, got (%d, %d)", r, c)
	}
	if ta.HasSelection() {
		t.Fatal("selection must be cleared after ReplaceDocument")
	}

	if !ta.Undo() {
		t.Fatal("ReplaceDocument must be undoable")
	}
	if got := ta.GetText(); got != "one\r\ntwo\r\nthree" {
		t.Fatalf("undo did not restore original text: %q", got)
	}
	if ta.IsModified() {
		t.Fatal("undo back to the only state must clear modified")
	}
	if !ta.Redo() {
		t.Fatal("ReplaceDocument must be redoable")
	}
	if got := ta.GetText(); got != "first\r\nsecond" {
		t.Fatalf("redo did not restore formatted text: %q", got)
	}
}

func TestTextArea_ReplaceDocumentKeepsCursorWhenPossible(t *testing.T) {
	ta := newTestTextArea("abc\ndef\nghi")
	pressKey(ta, 0x28, false, false) // Down
	moveRight(t, ta, 1)              // col 2 on line 2

	if !ta.ReplaceDocument("abcdef\nghijkl") {
		t.Fatal("ReplaceDocument must apply")
	}
	if r, c := ta.GetCursorPosition(); r != 2 || c != 2 {
		t.Fatalf("cursor should stay at (2, 2), got (%d, %d)", r, c)
	}
}

func TestTextArea_ReplaceDocumentPreservesEOLSerialization(t *testing.T) {
	ta := newTestTextArea("a\r\nb")
	ta.SetLineEnding("\r\n")
	if !ta.ReplaceDocument("x\ny\nz") {
		t.Fatal("ReplaceDocument must apply")
	}
	if got := ta.GetText(); got != "x\r\ny\r\nz" {
		t.Fatalf("EOL serialization lost: %q", got)
	}
	if strings.Contains(ta.GetTextWithEOL("\n"), "\r") {
		t.Fatalf("LF serialization should not contain CR")
	}
}

func TestTextArea_ReplaceDocumentRespectsReadOnly(t *testing.T) {
	ta := newTestTextArea("locked")
	ta.SetReadOnly(true)
	if ta.ReplaceDocument("changed") {
		t.Fatal("read-only document must reject ReplaceDocument")
	}
	if got := ta.GetText(); got != "locked" {
		t.Fatalf("read-only document changed: %q", got)
	}
	if ta.CanUndo() {
		t.Fatal("failed replace must not create undo history")
	}
}

// 回归：growGap 曾按缺口量而不是插入总量分配间隙，单次插入超过 max(2×间隙, 4096)
// 个 rune 时间隙反转，紧接着的编辑即 panic（粘贴大段文本、格式化后膨胀的文档都会触发）。
func TestTextArea_LargeInsertGrowsGapSufficiently(t *testing.T) {
	// 空文档的间隙是 64：阈值由 4096 兜底值决定
	for _, n := range []int{4095, 4096, 4097, 5000, 2*4096 + 1, 20000} {
		ta := newTestTextArea("")
		big := strings.Repeat("a", n)
		ta.insertText(big) // 与 Paste 相同的落盘路径，不依赖系统剪贴板
		if got := ta.GetText(); got != big {
			t.Fatalf("insert of %d runes: document holds %d runes", n, len([]rune(got)))
		}
		assertTextStorageInvariants(t, ta)
		// 间隙反转后的第一次编辑才会 panic：再键入一个字符并走一遍撤销往返
		pressRune(ta, 'Z')
		if got := ta.GetText(); got != big+"Z" {
			t.Fatalf("typing after a %d-rune insert: document holds %d runes", n, len([]rune(got)))
		}
		if !ta.Undo() || ta.GetText() != big {
			t.Fatalf("undo after a %d-rune insert did not restore the document", n)
		}
	}

	// 装载后的间隙是 长度/16 = 4096：阈值由 2×间隙 决定
	long := strings.Repeat("x", 64<<10)
	inserted := strings.Repeat("y", 2*4096+1)
	ta := newTestTextArea(long)
	pressKey(ta, 0x23, false, true) // Ctrl+End：光标移到文档末尾再插入
	ta.insertText(inserted)
	if got := ta.GetText(); got != long+inserted {
		t.Fatalf("large insert into a 64K document: document holds %d runes", len([]rune(got)))
	}
	assertTextStorageInvariants(t, ta)

	// 格式化路径：ReplaceDocument 把原文整体换成远长于 2×(原文+间隙) 的新文本
	source := strings.Repeat(`{"id":1},`, 300)
	formatted := strings.Repeat("    {\n        \"id\": 1\n    },\n", 300)
	ta = newTestTextArea(source)
	if !ta.ReplaceDocument(formatted) {
		t.Fatal("ReplaceDocument must apply")
	}
	if got := ta.GetTextWithEOL("\n"); got != formatted {
		t.Fatalf("ReplaceDocument with a %d-rune result: document holds %d runes", len([]rune(formatted)), len([]rune(got)))
	}
	assertTextStorageInvariants(t, ta)
	if !ta.Undo() || ta.GetText() != source {
		t.Fatal("undo after ReplaceDocument did not restore the source")
	}
}
