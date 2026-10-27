//go:build windows

package main

import (
	"testing"

	goui "github.com/jacksalad/gnote"
	"github.com/jacksalad/gnote/event"
)

func newTestAppWindow(t *testing.T, width, height int32) (*App, *goui.Window) {
	t.Helper()
	win, err := goui.NewWindow(goui.WindowConfig{
		Title:   "gnote-menu-test",
		Width:   width,
		Height:  height,
		Visible: false,
	})
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	app := NewApp(win, "Microsoft YaHei", DefaultSettings())
	app.Build()
	win.SetContent(app.root)
	return app, win
}

func TestApp_MenuModelAndFocus(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	app.textArea.SetFocused(true)
	menus := app.buildMenuModel()
	if len(menus) != 5 {
		t.Fatalf("menu count = %d, want 5", len(menus))
	}
	want := []string{"文件", "编辑", "查看", "设置", "帮助"}
	for i, title := range want {
		if menus[i].Text != title {
			t.Fatalf("menu[%d] = %q, want %q", i, menus[i].Text, title)
		}
	}

	if !app.textArea.IsFocused() {
		t.Fatalf("editor should start focused")
	}
	app.menuBar.Open(0)
	if !app.menuBar.IsOpen() || !app.menuBar.IsFocused() {
		t.Fatalf("opening menu should take keyboard focus")
	}
	if app.textArea.IsFocused() {
		t.Fatalf("editor must lose focus while menu is open")
	}
	app.menuBar.Close()
	if !app.textArea.IsFocused() {
		t.Fatalf("closing menu should restore editor focus")
	}
}

func TestApp_AltAndF10OpenMenu(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	win.EventBus().Publish(event.NewEvent(event.EventKeyDown, event.KeyData{
		KeyCode: 'F', Alt: true,
	}))
	if !app.menuBar.IsOpen() {
		t.Fatalf("Alt+F should open the File menu")
	}
	app.menuBar.Close()

	win.EventBus().Publish(event.NewEvent(event.EventKeyDown, event.KeyData{
		KeyCode: 0x79, // F10
	}))
	if !app.menuBar.IsOpen() {
		t.Fatalf("F10 should open the first menu")
	}
	app.menuBar.Close()
}

func TestApp_BlurClosesMenu(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	app.menuBar.Open(0)
	if !app.menuBar.IsOpen() {
		t.Fatalf("menu should be open")
	}
	win.EventBus().Publish(event.NewEvent(event.EventBlur, nil))
	if app.menuBar.IsOpen() {
		t.Fatalf("window blur should close the menu")
	}
}

func TestApp_MenuKeyboardExecutesCommand(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	app.menuBar.Open(0)
	if !app.root.OnEvent(event.NewEvent(event.EventKeyDown, event.KeyData{KeyCode: 0x0D})) {
		t.Fatalf("Enter should be consumed by the open menu")
	}
	if app.currentFileName != "未命名.txt" {
		t.Fatalf("new document command did not run, filename = %q", app.currentFileName)
	}
	if app.menuBar.IsOpen() {
		t.Fatalf("menu should close after executing the command")
	}
}

func TestApp_MenuModelCheckedStateIsLive(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	find := func(menus []goui.Menu, menuTitle, entryText string) goui.MenuEntry {
		for _, menu := range menus {
			if menu.Text != menuTitle {
				continue
			}
			for _, entry := range menu.Items {
				if entry.Text == entryText {
					return entry
				}
			}
		}
		t.Fatalf("entry %q not found in menu %q", entryText, menuTitle)
		return goui.MenuEntry{}
	}

	menus := app.buildMenuModel()
	if !find(menus, "查看", "行号").Checked {
		t.Fatalf("行号 should start checked")
	}
	if !find(menus, "查看", "自动换行").Checked {
		t.Fatalf("自动换行 should start checked")
	}
	if !find(menus, "查看", "语法高亮").Checked {
		t.Fatalf("语法高亮 should start checked")
	}

	app.textArea.SetShowLineNumbers(false)
	app.textArea.SetWordWrap(false)
	app.highlightEnabled = false
	menus = app.buildMenuModel()
	if find(menus, "查看", "行号").Checked ||
		find(menus, "查看", "自动换行").Checked ||
		find(menus, "查看", "语法高亮").Checked {
		t.Fatalf("view menu checks did not follow state changes")
	}
}

func TestApp_NarrowLayoutKeepsMenuAndEditor(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	app.root.SetBounds(0, 0, 640, 400)
	app.updateLayout()

	mb := app.menuBar.GetBounds()
	if mb.Width != 640 || mb.Height != 400 {
		t.Fatalf("menu layer bounds = %dx%d, want 640x400", mb.Width, mb.Height)
	}
	ta := app.textArea.GetBounds()
	if ta.Y != app.menuBar.BarHeight() {
		t.Fatalf("editor y = %d, want menu height %d", ta.Y, app.menuBar.BarHeight())
	}
	if want := int32(400) - app.menuBar.BarHeight() - app.statusHeight(); ta.Height != want {
		t.Fatalf("editor bounds = %+v, want height %d", ta, want)
	}

	// 窄窗口下拉必须仍在窗口内
	app.menuBar.SetBounds(0, 0, 640, 400)
	app.menuBar.Open(0)
	if !app.menuBar.IsOpen() {
		t.Fatalf("menu should open at narrow width")
	}
}

func TestApp_ToggleHighlightKeepsPlainTextPlain(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	app.currentFileName = "notes.txt"
	app.currentHighlighter = nil
	app.highlightEnabled = false
	app.textArea.SetHighlighter(nil)

	app.ToggleHighlight()
	if got := app.textArea.GetHighlighter(); got != nil {
		t.Fatalf("plain text must stay unhighlighted, got %T", got)
	}
	if !app.highlightEnabled {
		t.Fatal("highlight toggle flag should be on")
	}
}
