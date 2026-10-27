//go:build windows

package main

import (
	"testing"

	goui "github.com/jacksalad/gnote"
	"github.com/jacksalad/gnote/event"
)

func newCommandTestApp() *App {
	ta := goui.NewTextArea()
	ta.SetBounds(0, 0, 800, 600)
	a := &App{
		textArea:         ta,
		currentFileName:  "untitled.txt",
		highlightEnabled: true,
		themeID:          "dark",
	}
	a.buildCommands()
	return a
}

func TestCommandRegistry_UniqueAndComplete(t *testing.T) {
	a := newCommandTestApp()

	ids := []commandID{
		cmdFileNew, cmdFileOpen, cmdFileSave, cmdFileSaveAs, cmdFileQuit,
		cmdEditUndo, cmdEditRedo, cmdEditCut, cmdEditCopy, cmdEditPaste,
		cmdEditSelectAll, cmdEditFind, cmdEditReplace, cmdEditFormat,
		cmdViewLineNumbers, cmdViewWordWrap, cmdViewHighlight,
		cmdViewZoomIn, cmdViewZoomOut, cmdViewZoomReset,
		cmdHelpShortcuts, cmdHelpAbout, cmdAppSettings, cmdAppOpenConfig,
	}
	for _, th := range goui.Themes() {
		ids = append(ids, themeCommandID(th.ID))
	}
	for _, id := range ids {
		cmd := a.commands.Get(id)
		if cmd == nil {
			t.Fatalf("command %q is not registered", id)
		}
		if cmd.Title == "" {
			t.Fatalf("command %q has empty title", id)
		}
	}
	if got := len(a.commands.order); got != len(ids) {
		t.Fatalf("registered %d commands, want %d", got, len(ids))
	}
}

func TestCommandRegistry_UndoRedoStateIsLive(t *testing.T) {
	a := newCommandTestApp()

	if a.commands.Enabled(cmdEditUndo) {
		t.Fatalf("undo must be disabled on an empty document")
	}
	if a.commands.Run(cmdEditUndo) {
		t.Fatalf("disabled undo must not run")
	}

	a.textArea.SetFocused(true)
	a.textArea.OnEvent(event.NewEvent(event.EventKeyPress, event.KeyData{Char: 'A'}))
	if !a.commands.Enabled(cmdEditUndo) {
		t.Fatalf("undo must become enabled after typing")
	}
	if !a.commands.Run(cmdEditUndo) {
		t.Fatalf("enabled undo must run")
	}
	if got := a.textArea.GetText(); got != "" {
		t.Fatalf("undo did not remove typed text: %q", got)
	}
	if !a.commands.Enabled(cmdEditRedo) {
		t.Fatalf("redo must become enabled after undo")
	}
	if a.commands.Enabled(cmdEditUndo) {
		t.Fatalf("undo must be disabled again after undoing the only edit")
	}
}

func TestCommandRegistry_CheckedReflectsState(t *testing.T) {
	a := newCommandTestApp()

	for _, id := range []commandID{cmdViewLineNumbers, cmdViewWordWrap, cmdViewHighlight, cmdViewZoomReset} {
		if !a.commands.Checked(id) {
			t.Fatalf("command %q should be checked by default", id)
		}
	}
	if !a.commands.Checked(themeCommandID("dark")) {
		t.Fatalf("dark theme command should be checked by default")
	}

	a.textArea.SetShowLineNumbers(false)
	a.textArea.SetWordWrap(false)
	a.highlightEnabled = false
	a.themeID = "light"
	a.textArea.SetZoom(1.5)

	for _, id := range []commandID{cmdViewLineNumbers, cmdViewWordWrap, cmdViewHighlight, cmdViewZoomReset} {
		if a.commands.Checked(id) {
			t.Fatalf("command %q should not be checked after state change", id)
		}
	}
	if a.commands.Checked(themeCommandID("dark")) {
		t.Fatalf("dark theme command should be unchecked after switching to light")
	}
	if !a.commands.Checked(themeCommandID("light")) {
		t.Fatalf("light theme command should be checked after switching to light")
	}
}

func TestMenuModel_ThemeSubmenuListsEveryTheme(t *testing.T) {
	a := newCommandTestApp()
	var themeEntry *goui.MenuEntry
	for _, menu := range a.buildMenuModel() {
		if menu.Text != "查看" {
			continue
		}
		for i := range menu.Items {
			if menu.Items[i].Text == "主题" {
				themeEntry = &menu.Items[i]
			}
		}
	}
	if themeEntry == nil {
		t.Fatalf("view menu lacks theme submenu")
	}
	if got, want := len(themeEntry.Submenu), len(goui.Themes()); got != want {
		t.Fatalf("theme submenu has %d entries, want %d", got, want)
	}
	checked := 0
	for i, item := range themeEntry.Submenu {
		if item.Disabled {
			t.Fatalf("theme entry %d is disabled", i)
		}
		if item.Checked {
			checked++
		}
	}
	if checked != 1 {
		t.Fatalf("%d theme entries checked, want exactly 1", checked)
	}
}

func TestThemeCommand_AppliesImmediately(t *testing.T) {
	defer goui.SetTheme(goui.ThemeDark())
	a := newCommandTestApp()

	if !a.commands.Run(themeCommandID("nord")) {
		t.Fatalf("theme command did not run")
	}
	if a.themeID != "nord" {
		t.Fatalf("theme ID = %q, want nord", a.themeID)
	}
	if got := goui.CurrentTheme(); got == nil || got.ID != "nord" {
		t.Fatalf("global theme = %+v, want nord", got)
	}
	if !a.commands.Checked(themeCommandID("nord")) || a.commands.Checked(themeCommandID("dark")) {
		t.Fatalf("theme checked state did not follow the live theme")
	}
}
