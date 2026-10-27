package component

import (
	"strings"
	"testing"
	"time"
)

const searchDoc = "Alpha alpha ALPHA\r\nbeta Alpha\r\ngamma"

// 回归：ReplaceAll 曾对每个匹配单独重建整行，长单行下退化为 O(匹配数 × 行长)。
// 20 万字符单行、20 万次变长替换必须在秒级内完成，并且整体仍是一步可撤销操作。
func TestTextArea_ReplaceAllLongSingleLine(t *testing.T) {
	const n = 200000
	src := strings.Repeat("x", n)
	ta := newTestTextArea(src)

	start := time.Now()
	if got := ta.ReplaceAll("x", "yy", true); got != n {
		t.Fatalf("replace count = %d, want %d", got, n)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("ReplaceAll on one long line took %v, expected non-quadratic behavior", elapsed)
	}

	want := strings.Repeat("yy", n)
	if got := ta.GetText(); got != want {
		t.Fatalf("document not fully replaced (got %d runes, want %d)", len([]rune(got)), len([]rune(want)))
	}

	if !ta.Undo() {
		t.Fatal("undo after ReplaceAll failed")
	}
	if got := ta.GetText(); got != src {
		t.Fatalf("undo did not restore the original long line")
	}
	if !ta.Redo() {
		t.Fatal("redo after ReplaceAll failed")
	}
	if got := ta.GetText(); got != want {
		t.Fatalf("redo did not reapply the batch replacement")
	}
}

// 替换文本含换行时会跨行展开：撤销/重做必须精确还原行数与行序号。
func TestTextArea_ReplaceAllInsertingNewlinesAcrossRows(t *testing.T) {
	ta := newTestTextArea("a-b\r\nc-a\r\na")

	if got := ta.ReplaceAll("a", "1\n2", true); got != 3 {
		t.Fatalf("replace count = %d, want 3", got)
	}
	want := "1\r\n2-b\r\nc-1\r\n2\r\n1\r\n2"
	if got := ta.GetText(); got != want {
		t.Fatalf("after replace: %q want %q", got, want)
	}

	if !ta.Undo() {
		t.Fatal("undo failed")
	}
	if got, want := ta.GetText(), "a-b\r\nc-a\r\na"; got != want {
		t.Fatalf("undo: %q want %q", got, want)
	}

	if !ta.Redo() {
		t.Fatal("redo failed")
	}
	if got := ta.GetText(); got != want {
		t.Fatalf("redo: %q want %q", got, want)
	}
}

func TestTextArea_FindNextForwardWrapAndCase(t *testing.T) {
	ta := newTestTextArea(searchDoc)

	m, ok := ta.FindNext("Alpha", 0, 0, true, true)
	if !ok || m.StartRow != 0 || m.StartCol != 0 || m.EndCol != 5 {
		t.Fatalf("first forward match: %+v ok=%v", m, ok)
	}

	// 从第 0 行第 1 列开始，第 0 行不再有区分大小写的匹配
	m, ok = ta.FindNext("Alpha", 0, 1, true, true)
	if !ok || m.StartRow != 1 || m.StartCol != 5 {
		t.Fatalf("forward match should land on row 1: %+v ok=%v", m, ok)
	}

	// 从末尾继续查找应回绕到文档开头
	m, ok = ta.FindNext("Alpha", 2, 0, true, true)
	if !ok || m.StartRow != 0 || m.StartCol != 0 {
		t.Fatalf("forward search should wrap: %+v ok=%v", m, ok)
	}

	// 反向查找：从 (1,6) 往回找到同一行的 (1,5)
	m, ok = ta.FindNext("Alpha", 1, 6, false, true)
	if !ok || m.StartRow != 1 || m.StartCol != 5 {
		t.Fatalf("backward match: %+v ok=%v", m, ok)
	}

	// 大小写不敏感：第 0 行有三个 alpha
	m, ok = ta.FindNext("alpha", 0, 0, true, false)
	if !ok || m.StartCol != 0 {
		t.Fatalf("case-insensitive first: %+v ok=%v", m, ok)
	}
	m, ok = ta.FindNext("alpha", 0, 1, true, false)
	if !ok || m.StartCol != 6 {
		t.Fatalf("case-insensitive second: %+v ok=%v", m, ok)
	}

	if _, ok := ta.FindNext("missing", 0, 0, true, true); ok {
		t.Fatal("expected no match")
	}
	if _, ok := ta.FindNext("", 0, 0, true, true); ok {
		t.Fatal("empty query must not match")
	}
}

func TestTextArea_FindAllDoesNotOverlap(t *testing.T) {
	ta := newTestTextArea("aaaa")
	matches := ta.FindAll("aa", true)
	if len(matches) != 2 {
		t.Fatalf("expected 2 non-overlapping matches, got %d", len(matches))
	}
	if matches[0].StartCol != 0 || matches[1].StartCol != 2 {
		t.Fatalf("unexpected match columns: %+v", matches)
	}
}

func TestTextArea_SetSelectionAndRange(t *testing.T) {
	ta := newTestTextArea(searchDoc)

	ta.SetSelection(0, 0, 0, 5)
	sr, sc, er, ec, ok := ta.GetSelectionRange()
	if !ok || sr != 0 || sc != 0 || er != 0 || ec != 5 {
		t.Fatalf("unexpected selection range: %d,%d-%d,%d ok=%v", sr, sc, er, ec, ok)
	}
	if got := ta.getSelectedText(); got != "Alpha" {
		t.Fatalf("selected text = %q", got)
	}

	// 反向传入应被规范化
	ta.SetSelection(0, 5, 0, 0)
	sr, sc, er, ec, ok = ta.GetSelectionRange()
	if !ok || sr != 0 || sc != 0 || er != 0 || ec != 5 {
		t.Fatalf("selection should be normalized: %d,%d-%d,%d", sr, sc, er, ec)
	}

	// 越界钳制
	ta.SetSelection(-5, -5, 999, 999)
	sr, sc, er, ec, ok = ta.GetSelectionRange()
	if !ok || sr != 0 || sc != 0 || er != ta.GetLineCount()-1 {
		t.Fatalf("selection should be clamped: %d,%d-%d,%d", sr, sc, er, ec)
	}
}

func TestTextArea_ReplaceAllIsSingleUndoStep(t *testing.T) {
	ta := newTestTextArea("foo bar foo\r\nfoo")

	if n := ta.ReplaceAll("foo", "baz", true); n != 3 {
		t.Fatalf("expected 3 replacements, got %d", n)
	}
	if got, want := ta.GetText(), "baz bar baz\r\nbaz"; got != want {
		t.Fatalf("after replace all: %q want %q", got, want)
	}
	if !ta.IsModified() {
		t.Fatal("replace all must mark document modified")
	}

	if !ta.Undo() {
		t.Fatal("undo failed")
	}
	if got := ta.GetText(); got != "foo bar foo\r\nfoo" {
		t.Fatalf("single undo should restore original: %q", got)
	}
	if ta.IsModified() {
		t.Fatal("undo back to saved revision must clear modified flag")
	}

	if !ta.Redo() {
		t.Fatal("redo failed")
	}
	if got := ta.GetText(); got != "baz bar baz\r\nbaz" {
		t.Fatalf("redo should reapply replacements: %q", got)
	}
}

func TestTextArea_ReplaceAllCaseInsensitiveAndLengthChange(t *testing.T) {
	ta := newTestTextArea("AA aa Aa")

	if n := ta.ReplaceAll("aa", "x", false); n != 3 {
		t.Fatalf("expected 3 case-insensitive replacements, got %d", n)
	}
	if got := ta.GetText(); got != "x x x" {
		t.Fatalf("unexpected result: %q", got)
	}
	ta.Undo()
	if got := ta.GetText(); got != "AA aa Aa" {
		t.Fatalf("undo after case-insensitive replace: %q", got)
	}
}

func TestTextArea_ReplaceAllAcrossManyMatchesStaysConsistent(t *testing.T) {
	ta := newTestTextArea("x-x-x\r\ny-x-y\r\nno match here")

	if n := ta.ReplaceAll("x", "long", true); n != 4 {
		t.Fatalf("expected 4 replacements, got %d", n)
	}
	if got, want := ta.GetText(), "long-long-long\r\ny-long-y\r\nno match here"; got != want {
		t.Fatalf("replace result: %q want %q", got, want)
	}
	ta.Undo()
	if got := ta.GetText(); got != "x-x-x\r\ny-x-y\r\nno match here" {
		t.Fatalf("undo result: %q", got)
	}
	if r, c := ta.GetCursorPosition(); r != 1 || c != 1 {
		t.Fatalf("cursor after undo = %d,%d", r, c)
	}
}

func TestTextArea_ReplaceCurrentSelection(t *testing.T) {
	ta := newTestTextArea("hello world")
	ta.SetSelection(0, 6, 0, 11)

	if !ta.ReplaceCurrentSelection("world", "gnote", true) {
		t.Fatal("replace current selection failed")
	}
	if got := ta.GetText(); got != "hello gnote" {
		t.Fatalf("unexpected text: %q", got)
	}

	// 选区内容不匹配时不应替换
	ta.SetSelection(0, 0, 0, 5)
	if ta.ReplaceCurrentSelection("world", "x", true) {
		t.Fatal("must not replace when selection does not match query")
	}
	if got := ta.GetText(); got != "hello gnote" {
		t.Fatalf("text changed unexpectedly: %q", got)
	}
}

func TestTextArea_LineEndingSerialization(t *testing.T) {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 400, 300)
	ta.SetText("a\r\nb\nc")

	if got := ta.GetText(); got != "a\r\nb\r\nc" {
		t.Fatalf("default CRLF serialization: %q", got)
	}
	if got := ta.GetTextWithEOL("\n"); got != "a\nb\nc" {
		t.Fatalf("LF serialization: %q", got)
	}

	ta.SetLineEnding("\n")
	if got := ta.GetText(); got != "a\nb\nc" {
		t.Fatalf("configured LF serialization: %q", got)
	}
	if got := ta.GetLineEnding(); got != "\n" {
		t.Fatalf("GetLineEnding = %q", got)
	}

	ta.SetLineEnding("")
	if got := ta.GetText(); got != "a\r\nb\r\nc" {
		t.Fatalf("empty line ending should fall back to CRLF: %q", got)
	}
}
