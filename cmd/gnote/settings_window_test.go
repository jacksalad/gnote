//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goui "github.com/jacksalad/gnote"
	"github.com/jacksalad/gnote/event"
)

func TestSettingsWindow_ApplyPersistsAndRestoresOwner(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()
	defer goui.SetTheme(goui.ThemeDark())
	app.settingsPathOverride = filepath.Join(t.TempDir(), "settings.json")

	sw, err := newSettingsWindow(app)
	if err != nil {
		t.Fatalf("newSettingsWindow: %v", err)
	}
	defer func() {
		if goui.IsWindow(sw.win.Hwnd()) {
			sw.win.Destroy()
		}
	}()
	app.settingsWin = sw
	sw.show()

	if goui.IsWindowEnabled(win.Hwnd()) {
		t.Fatalf("modal settings window must disable the main window")
	}

	sw.theme.SetSelected(1)
	sw.fontSlider.SetValue(20)
	sw.wrap.SetOn(false)
	sw.lineNums.SetOn(false)
	sw.highlight.SetOn(false)
	sw.startup.SetSelected(1)
	sw.autoZoom.SetOn(false)
	sw.zoomSlider.SetValue(150)
	sw.apply()

	if app.settingsWin != sw {
		t.Fatalf("apply must keep the settings window open")
	}
	if goui.IsWindowEnabled(win.Hwnd()) {
		t.Fatalf("modal settings window must stay modal after apply")
	}
	if sw.status == nil || sw.status.GetText() != "已保存" {
		t.Fatalf("apply should show save confirmation")
	}
	if app.themeID != "light" {
		t.Fatalf("theme should be light after apply")
	}
	if got := app.textArea.GetBaseFontSize(); got != 20 {
		t.Fatalf("base font = %d, want 20", got)
	}
	if app.textArea.IsWordWrap() || app.textArea.IsShowLineNumbers() || app.highlightEnabled {
		t.Fatalf("applied switches not reflected in editor state")
	}
	if app.settings.StartupPage != startupPageBlank {
		t.Fatalf("startup page = %q, want %q", app.settings.StartupPage, startupPageBlank)
	}
	if got := app.textArea.GetZoomPercent(); got != 150 {
		t.Fatalf("zoom = %d%%, want 150%%", got)
	}

	data, err := os.ReadFile(app.settingsPathOverride)
	if err != nil {
		t.Fatalf("settings file not written: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, `"theme": "light"`) || !strings.Contains(text, `"baseFontSize": 20`) ||
		!strings.Contains(text, `"startupPage": "blank"`) {
		t.Fatalf("settings file missing applied values:\n%s", text)
	}

	sw.win.Close()
	if app.settingsWin != nil {
		t.Fatalf("closing the settings window must clear App.settingsWin")
	}
	if !goui.IsWindowEnabled(win.Hwnd()) {
		t.Fatalf("closing the settings window must re-enable the main window")
	}
}

func TestSettingsWindow_CancelAndReset(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()
	app.settingsPathOverride = filepath.Join(t.TempDir(), "settings.json")

	sw, err := newSettingsWindow(app)
	if err != nil {
		t.Fatalf("newSettingsWindow: %v", err)
	}
	defer func() {
		if goui.IsWindow(sw.win.Hwnd()) {
			sw.win.Destroy()
		}
	}()
	app.settingsWin = sw

	sw.theme.SetSelected(1)
	sw.fontSlider.SetValue(26)
	sw.wrap.SetOn(false)
	sw.reset()
	if sw.theme.Selected() != 0 || int(sw.fontSlider.GetValue()) != 16 ||
		!sw.wrap.IsOn() || !sw.lineNums.IsOn() || !sw.highlight.IsOn() ||
		sw.startup.Selected() != 0 {
		t.Fatalf("reset did not restore default controls")
	}

	sw.theme.SetSelected(1)
	sw.fontSlider.SetValue(26)
	sw.wrap.SetOn(false)
	sw.cancel()

	if app.settingsWin != nil {
		t.Fatalf("cancel should clear App.settingsWin")
	}
	if app.themeID != "dark" {
		t.Fatalf("cancel must keep the original dark theme")
	}
	if got := app.textArea.GetBaseFontSize(); got != 16 {
		t.Fatalf("cancel changed base font to %d", got)
	}
	if !app.textArea.IsWordWrap() {
		t.Fatalf("cancel changed word wrap")
	}
	if !goui.IsWindowEnabled(win.Hwnd()) {
		t.Fatalf("cancel must re-enable the main window")
	}
}

func TestApp_OpenConfigFile(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()
	app.settingsPathOverride = filepath.Join(t.TempDir(), "settings.json")

	if err := app.SaveSettings(); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	app.OpenConfigFile()

	if filepath.Base(app.currentFileName) != "settings.json" {
		t.Fatalf("current file = %q, want settings.json", app.currentFileName)
	}
	if !strings.Contains(app.textArea.GetText(), `"version"`) {
		t.Fatalf("settings file content not loaded into editor")
	}
}

func TestSettingsWindow_ApplyCustomTheme(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()
	defer goui.SetTheme(goui.ThemeDark())
	app.settingsPathOverride = filepath.Join(t.TempDir(), "settings.json")

	sw, err := newSettingsWindow(app)
	if err != nil {
		t.Fatalf("newSettingsWindow: %v", err)
	}
	defer func() {
		if goui.IsWindow(sw.win.Hwnd()) {
			sw.win.Destroy()
		}
	}()
	app.settingsWin = sw

	index := -1
	for i, th := range goui.Themes() {
		if th.ID == "nord" {
			index = i
		}
	}
	if index < 0 {
		t.Fatal("nord theme missing from registry")
	}
	sw.theme.SetSelected(index)
	sw.apply()

	if app.themeID != "nord" {
		t.Fatalf("theme ID = %q, want nord", app.themeID)
	}
	if got := goui.CurrentTheme(); got == nil || got.ID != "nord" {
		t.Fatalf("global theme = %+v, want nord", got)
	}
	data, err := os.ReadFile(app.settingsPathOverride)
	if err != nil {
		t.Fatalf("settings file not written: %v", err)
	}
	if !strings.Contains(string(data), `"theme": "nord"`) {
		t.Fatalf("settings file missing nord theme:\n%s", data)
	}
}

func selectThemeSegment(t *testing.T, sw *settingsWindow, themeID string) int {
	t.Helper()
	index := -1
	for i, th := range goui.Themes() {
		if th.ID == themeID {
			index = i
			break
		}
	}
	if index < 0 {
		t.Fatalf("theme %q missing from registry", themeID)
	}
	b := sw.theme.GetBounds()
	segW := b.Width / int32(len(goui.Themes()))
	if segW <= 0 {
		t.Fatalf("theme selector has no usable width")
	}
	x := b.X + int32(index)*segW + segW/2
	sw.theme.OnEvent(event.NewEvent(event.EventMouseDown, event.MouseData{
		X: x, Y: b.Y + b.Height/2, Button: event.MouseButtonLeft,
	}))
	return index
}

func TestSettingsWindow_ThemePreviewIsImmediateAndCancelReverts(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()
	defer goui.SetTheme(goui.ThemeDark())

	sw, err := newSettingsWindow(app)
	if err != nil {
		t.Fatalf("newSettingsWindow: %v", err)
	}
	defer func() {
		if goui.IsWindow(sw.win.Hwnd()) {
			sw.win.Destroy()
		}
	}()
	app.settingsWin = sw

	selectThemeSegment(t, sw, "nord")
	if app.themeID != "nord" {
		t.Fatalf("preview did not switch app theme: %q", app.themeID)
	}
	if got := goui.CurrentTheme(); got == nil || got.ID != "nord" {
		t.Fatalf("preview did not switch global theme: %+v", got)
	}

	sw.cancel()
	if app.themeID != "dark" {
		t.Fatalf("cancel did not restore original theme: %q", app.themeID)
	}
	if got := goui.CurrentTheme(); got == nil || got.ID != "dark" {
		t.Fatalf("cancel did not restore global theme: %+v", got)
	}
}

func TestSettingsWindow_ThemePreviewRevertsWhenClosed(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()
	defer goui.SetTheme(goui.ThemeDark())

	sw, err := newSettingsWindow(app)
	if err != nil {
		t.Fatalf("newSettingsWindow: %v", err)
	}
	app.settingsWin = sw

	selectThemeSegment(t, sw, "tokyo-night")
	if app.themeID != "tokyo-night" {
		t.Fatalf("preview did not switch app theme: %q", app.themeID)
	}
	sw.win.Close()
	if app.themeID != "dark" {
		t.Fatalf("closing without apply must restore original theme: %q", app.themeID)
	}
	if app.settingsWin != nil {
		t.Fatalf("close should clear App.settingsWin")
	}
}

func TestSettingsWindow_ApplyBecomesNewCancelBaseline(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()
	defer goui.SetTheme(goui.ThemeDark())
	app.settingsPathOverride = filepath.Join(t.TempDir(), "settings.json")

	sw, err := newSettingsWindow(app)
	if err != nil {
		t.Fatalf("newSettingsWindow: %v", err)
	}
	defer func() {
		if goui.IsWindow(sw.win.Hwnd()) {
			sw.win.Destroy()
		}
	}()
	app.settingsWin = sw

	selectThemeSegment(t, sw, "nord")
	sw.apply()
	if app.themeID != "nord" {
		t.Fatalf("apply did not commit nord: %q", app.themeID)
	}
	if sw.status.GetText() != "已保存" {
		t.Fatalf("apply confirmation missing")
	}

	// 应用后继续预览其他主题，取消应回到最后一次应用的 nord，而不是打开窗口时的 dark。
	selectThemeSegment(t, sw, "rose-pine")
	if app.themeID != "rose-pine" {
		t.Fatalf("preview did not switch to rose-pine: %q", app.themeID)
	}
	if sw.status.GetText() != "" {
		t.Fatalf("changing theme should clear the stale save confirmation")
	}
	sw.cancel()

	if app.themeID != "nord" {
		t.Fatalf("cancel after apply should restore the applied baseline, got %q", app.themeID)
	}
	data, err := os.ReadFile(app.settingsPathOverride)
	if err != nil {
		t.Fatalf("settings file not written: %v", err)
	}
	if !strings.Contains(string(data), `"theme": "nord"`) {
		t.Fatalf("settings file should still contain the applied theme:\n%s", data)
	}
}
