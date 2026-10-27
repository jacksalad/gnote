//go:build windows

package main

import (
	"testing"

	goui "github.com/jacksalad/gnote"
	"github.com/jacksalad/gnote/event"
)

func newTestFindBar(doc string) (*findBar, *goui.TextArea) {
	ta := goui.NewTextArea()
	ta.SetBounds(0, 0, 800, 600)
	ta.SetText(doc)
	ta.SetFocused(true)
	fb := newFindBar(ta, nil, func() {})
	fb.show(false, 800)
	return fb, ta
}

func selection(t *testing.T, ta *goui.TextArea) (int, int, int, int) {
	t.Helper()
	sr, sc, er, ec, ok := ta.GetSelectionRange()
	if !ok {
		t.Fatalf("expected an active selection")
	}
	return sr, sc, er, ec
}

func TestFindBarSearchNavigation(t *testing.T) {
	fb, ta := newTestFindBar("foo one foo\r\ntwo foo")

	fb.query.SetText("foo")
	fb.searchFromOrigin()
	if sr, sc, er, ec := selection(t, ta); sr != 0 || sc != 0 || er != 0 || ec != 3 {
		t.Fatalf("first match selection = %d,%d-%d,%d", sr, sc, er, ec)
	}

	fb.find(true)
	if sr, sc, er, ec := selection(t, ta); sr != 0 || sc != 8 || ec != 11 {
		t.Fatalf("second match selection = %d,%d-%d,%d", sr, sc, er, ec)
	}

	fb.find(true)
	if sr, sc, _, _ := selection(t, ta); sr != 1 || sc != 4 {
		t.Fatalf("third match selection = %d,%d", sr, sc)
	}

	// 回绕：最后一个匹配之后继续查找应回到文档开头
	fb.find(true)
	if sr, sc, _, _ := selection(t, ta); sr != 0 || sc != 0 {
		t.Fatalf("wrap-around selection = %d,%d", sr, sc)
	}

	fb.find(false)
	if sr, sc, _, _ := selection(t, ta); sr != 1 || sc != 4 {
		t.Fatalf("backward wrap selection = %d,%d", sr, sc)
	}

	fb.query.SetText("no-such-token")
	fb.searchFromOrigin()
	if got := fb.status.GetText(); got != "无匹配" {
		t.Fatalf("status after miss = %q", got)
	}
}

func TestFindBarReplaceOneIsSingleUndo(t *testing.T) {
	fb, ta := newTestFindBar("foo one foo\r\ntwo foo")

	fb.query.SetText("foo")
	fb.searchFromOrigin()
	fb.replaceBox.SetText("bar")
	fb.replaceOne()

	if got, want := ta.GetText(), "bar one foo\r\ntwo foo"; got != want {
		t.Fatalf("after replace one: %q want %q", got, want)
	}
	if got := fb.status.GetText(); got != "已替换 1 处" {
		t.Fatalf("status = %q", got)
	}
	// 替换后应自动跳到下一个匹配
	if sr, sc, _, _ := selection(t, ta); sr != 0 || sc != 8 {
		t.Fatalf("selection after replace = %d,%d", sr, sc)
	}

	if !ta.Undo() {
		t.Fatal("undo failed")
	}
	if got := ta.GetText(); got != "foo one foo\r\ntwo foo" {
		t.Fatalf("undo after replace one: %q", got)
	}
}

func TestFindBarReplaceAll(t *testing.T) {
	fb, ta := newTestFindBar("foo one foo\r\ntwo foo")

	fb.query.SetText("foo")
	fb.replaceBox.SetText("bar")
	fb.replaceAll()

	if got, want := ta.GetText(), "bar one bar\r\ntwo bar"; got != want {
		t.Fatalf("after replace all: %q want %q", got, want)
	}
	if got := fb.status.GetText(); got != "已替换 3 处" {
		t.Fatalf("status = %q", got)
	}

	if !ta.Undo() {
		t.Fatal("undo failed")
	}
	if got := ta.GetText(); got != "foo one foo\r\ntwo foo" {
		t.Fatalf("replace all must be a single undo step, got %q", got)
	}
}

func TestFindBarCaseToggleAffectsSearch(t *testing.T) {
	fb, ta := newTestFindBar("Foo foo FOO")

	fb.query.SetText("foo")
	fb.searchFromOrigin()
	if _, sc, _, _ := selection(t, ta); sc != 0 {
		t.Fatalf("case-insensitive first match col = %d", sc)
	}

	fb.caseSensitive = true
	fb.searchFromOrigin()
	if sr, sc, _, _ := selection(t, ta); sr != 0 || sc != 4 {
		t.Fatalf("case-sensitive first match = %d,%d", sr, sc)
	}
}

func TestFindBarTypingSearchesImmediatelyWithoutWindow(t *testing.T) {
	fb, ta := newTestFindBar("foo bar")
	fb.query.SetText("")

	fb.query.OnEvent(event.NewEvent(event.EventKeyPress, event.KeyData{Char: 'o'}))

	sr, sc, er, ec := selection(t, ta)
	if sr != 0 || sc != 1 || er != 0 || ec != 2 {
		t.Fatalf("incremental match = %d,%d-%d,%d", sr, sc, er, ec)
	}
}

func TestFindBarNarrowLayoutKeepsControlsInside(t *testing.T) {
	fb, _ := newTestFindBar("x")
	const width = 600
	fb.layout(width)

	controls := []goui.Component{
		fb.query, fb.btnCase, fb.btnPrev, fb.btnNext,
		fb.replaceBox, fb.btnReplace, fb.btnAll, fb.status, fb.btnClose,
	}
	for _, c := range controls {
		b := c.GetBounds()
		if b.Width < 0 || b.X < 0 || b.X+b.Width > width {
			t.Fatalf("%T bounds %+v outside 0..%d", c, b, width)
		}
	}
	all := fb.btnAll.GetBounds()
	closeB := fb.btnClose.GetBounds()
	if all.X+all.Width > closeB.X {
		t.Fatalf("controls overlap: replace-all ends at %d, close starts at %d", all.X+all.Width, closeB.X)
	}
}
