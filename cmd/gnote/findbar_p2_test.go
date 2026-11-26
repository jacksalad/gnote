//go:build windows

package main

import (
	"testing"

	"github.com/jacksalad/gnote/event"
)

// findBarWithWindow 带真实（隐藏）窗口的查找条：防抖任务要靠窗口的 Schedule 登记，
// 测试不跑消息循环，登记的任务不会执行，正好用来模拟 “120ms 还没到” 的瞬间。
func findBarWithWindow(t *testing.T, doc string) (*App, *findBar) {
	t.Helper()
	app, win := newTestAppWindow(t, 900, 640)
	t.Cleanup(win.Destroy)
	ta := app.textArea
	ta.SetText(doc)
	ta.SetFocused(true)
	app.findBar.show(false, 900)
	return app, app.findBar
}

func findSelection(t *testing.T, fb *findBar) (int, int, int, int) {
	t.Helper()
	sr, sc, er, ec, ok := fb.area.GetSelectionRange()
	if !ok {
		t.Fatalf("expected an active selection")
	}
	return sr, sc, er, ec
}

// 改完查找内容后防抖还没到就点 “替换”：必须先把待执行的搜索落定，再替换新查找内容的第一个匹配。
// 以前直接作废防抖任务，再拿旧匹配的状态去判断，结果什么都不发生。
func TestFindBar_ReplaceFlushesPendingDebouncedSearch(t *testing.T) {
	_, fb := findBarWithWindow(t, "foo bar foo")
	fb.query.SetText("foo")
	fb.searchFromOrigin()
	fb.replaceBox.SetText("X")

	// 用户把查找内容改成 bar：OnChange 只登记了防抖任务，搜索尚未执行
	fb.query.SetText("bar")
	fb.scheduleSearch()
	fb.replaceOne()

	if got, want := fb.area.GetText(), "foo X foo"; got != want {
		t.Fatalf("text = %q, want %q (replace silently did nothing)", got, want)
	}
}

// 同理 “下一个”：防抖中的输入先定位到新查找内容的第一个匹配，而不是沿用旧匹配的位置继续往前跳。
func TestFindBar_NextFlushesPendingDebouncedSearch(t *testing.T) {
	_, fb := findBarWithWindow(t, "foo bar foo")
	fb.query.SetText("bar")
	fb.searchFromOrigin()

	fb.query.SetText("foo")
	fb.scheduleSearch()
	fb.find(true)

	if sr, sc, er, ec := findSelection(t, fb); sr != 0 || sc != 0 || er != 0 || ec != 3 {
		t.Fatalf("selection = %d,%d-%d,%d, want the first match 0,0-0,3", sr, sc, er, ec)
	}
}

// 全部替换不依赖当前匹配，防抖中的输入不能在之后补跑，否则会把已替换的编辑器再选一次。
func TestFindBar_ReplaceAllCancelsPendingDebouncedSearch(t *testing.T) {
	_, fb := findBarWithWindow(t, "foo bar foo")
	fb.query.SetText("foo")
	fb.searchFromOrigin()

	fb.query.SetText("foo")
	fb.replaceBox.SetText("X")
	fb.scheduleSearch()
	fb.replaceAll()
	if got, want := fb.area.GetText(), "X bar X"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
	if fb.hasMatch {
		t.Errorf("replace-all leaves no current match for a stale search to select")
	}
}

// Shift+F3 是 “上一个”：从光标位置向前（朝文档开头）找，而不是和 F3 一样去找下一个。
func TestApp_ShiftF3FindsPreviousMatch(t *testing.T) {
	app, fb := findBarWithWindow(t, "a b a b a")
	fb.query.SetText("a")
	fb.area.SetSelection(0, 4, 0, 4) // 光标停在最后一个 a 之前
	if _, _, _, _, ok := fb.area.GetSelectionRange(); ok {
		t.Fatal("precondition: caret only, no selection")
	}

	app.win.EventBus().Publish(event.NewEvent(event.EventKeyDown, event.KeyData{KeyCode: 0x72, Shift: true}))

	if sr, sc, er, ec := findSelection(t, fb); sr != 0 || sc != 0 || er != 0 || ec != 1 {
		t.Fatalf("Shift+F3 selection = %d,%d-%d,%d, want the previous match 0,0-0,1", sr, sc, er, ec)
	}
}

// 普通的 F3 仍然是 “下一个”。
func TestApp_F3StillFindsNextMatch(t *testing.T) {
	app, fb := findBarWithWindow(t, "a b a b a")
	fb.query.SetText("a")
	fb.area.SetSelection(0, 0, 0, 0)

	app.win.EventBus().Publish(event.NewEvent(event.EventKeyDown, event.KeyData{KeyCode: 0x72}))

	if sr, sc, er, ec := findSelection(t, fb); sr != 0 || sc != 0 || er != 0 || ec != 1 {
		t.Fatalf("F3 selection = %d,%d-%d,%d, want the match at the caret 0,0-0,1", sr, sc, er, ec)
	}
}

// clickMenuTitle 按 Window.dispatchEvent 的顺序模拟鼠标点击：先事件总线上的处理器，再组件树。
func clickMenuTitle(app *App, x, y int32) {
	evt := event.NewEvent(event.EventMouseDown, event.MouseData{X: x, Y: y, Button: event.MouseButtonLeft})
	app.win.EventBus().Publish(evt)
	app.root.OnEvent(evt)
}

// 查找条可见时用鼠标点开菜单：焦点不能被抢到编辑器，否则菜单关闭后输入的字符会写进正文，
// 盖掉刚选中的匹配。点击菜单栏标题不应改变焦点。
func TestApp_MouseOpenedMenuReturnsFocusToFindBar(t *testing.T) {
	app, fb := findBarWithWindow(t, "foo bar foo")
	fb.query.SetText("bar")
	fb.searchFromOrigin()
	if !fb.query.IsFocused() || app.textArea.IsFocused() {
		t.Fatalf("precondition: the find box owns the keyboard")
	}

	clickMenuTitle(app, 8, 8)
	if !app.menuBar.IsOpen() {
		t.Fatal("clicking the menu title should open the menu")
	}
	app.menuBar.Close()

	if !fb.query.IsFocused() || app.textArea.IsFocused() {
		t.Fatalf("after the menu closes the find box must own the keyboard again: query=%v editor=%v",
			fb.query.IsFocused(), app.textArea.IsFocused())
	}
	if sr, sc, er, ec := findSelection(t, fb); sr != 0 || sc != 4 || er != 0 || ec != 7 {
		t.Errorf("the current match must stay selected: %d,%d-%d,%d", sr, sc, er, ec)
	}
}

// 点击查找条和菜单栏以外的区域（编辑器）仍照旧把键盘交给编辑器。
func TestApp_ClickOutsideFindBarStillFocusesEditor(t *testing.T) {
	app, fb := findBarWithWindow(t, "foo bar foo")
	clickMenuTitle(app, 8, 300)
	if app.menuBar.IsOpen() {
		t.Fatal("a click below the menu bar must not open the menu")
	}
	if !app.textArea.IsFocused() || fb.query.IsFocused() {
		t.Fatalf("editor should own the keyboard: editor=%v query=%v", app.textArea.IsFocused(), fb.query.IsFocused())
	}
}

// 替换框里的多行文本保留换行（统一成 LF），多行替换照常可用。
// 不碰真实剪贴板：SetText 与粘贴走同一条换行规则，用它确认替换框配置成了 “保留换行”。
func TestFindBar_ReplaceBoxAcceptsMultilineReplacement(t *testing.T) {
	_, fb := findBarWithWindow(t, "a b c")
	fb.query.SetText("b")
	fb.searchFromOrigin()
	fb.replaceBox.SetText("x\r\ny")
	fb.replaceAll()
	if got, want := fb.area.GetText(), "a x\r\ny c"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}
