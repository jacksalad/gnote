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
