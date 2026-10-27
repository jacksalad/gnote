//go:build windows

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	goui "github.com/jacksalad/gnote"
)

func TestSettings_DefaultsAndNormalize(t *testing.T) {
	def := DefaultSettings()
	if def.Version != settingsVersion || def.Theme != "dark" || def.BaseFontSize != 16 ||
		def.Zoom != 0 || !def.WordWrap || !def.LineNumbers || !def.Highlight ||
		def.StartupPage != startupPageWelcome {
		t.Fatalf("unexpected defaults: %+v", def)
	}
	if def.WindowWidth != 1560 || def.WindowHeight != 920 {
		t.Fatalf("unexpected default window size: %dx%d", def.WindowWidth, def.WindowHeight)
	}

	s := Settings{
		Version:      0,
		Theme:        "LIGHT",
		BaseFontSize: 1,
		Zoom:         0.1,
		WindowWidth:  100,
		WindowHeight: 100,
		RecentFiles: []string{
			" a.txt ", "", "a.txt", "b.txt", "c.txt", "d.txt", "e.txt",
			"f.txt", "g.txt", "h.txt", "i.txt", "j.txt", "k.txt", "l.txt",
		},
	}.Normalized()

	if s.Version != settingsVersion || s.Theme != "light" {
		t.Fatalf("theme/version normalization failed: %+v", s)
	}
	if s.BaseFontSize != 12 || s.Zoom != 0.5 {
		t.Fatalf("font/zoom clamp failed: %+v", s)
	}
	if s.WindowWidth != 640 || s.WindowHeight != 400 {
		t.Fatalf("window clamp failed: %+v", s)
	}
	if len(s.RecentFiles) != 10 {
		t.Fatalf("recent files = %v, want 10 unique entries", s.RecentFiles)
	}
	if s.RecentFiles[0] != "a.txt" {
		t.Fatalf("recent files not trimmed/deduped: %v", s.RecentFiles)
	}

	high := Settings{Theme: "nope", BaseFontSize: 99, Zoom: 9, WindowWidth: 99999, WindowHeight: 99999}.Normalized()
	if high.Theme != "dark" || high.BaseFontSize != 32 || high.Zoom != 3 {
		t.Fatalf("upper clamp failed: %+v", high)
	}

	if got := (Settings{StartupPage: "  BLANK "}).Normalized().StartupPage; got != startupPageBlank {
		t.Fatalf("startup page = %q, want %q", got, startupPageBlank)
	}
	for _, raw := range []string{"", "nonsense", "   "} {
		if got := (Settings{StartupPage: raw}).Normalized().StartupPage; got != startupPageWelcome {
			t.Fatalf("startup page %q normalized to %q, want %q", raw, got, startupPageWelcome)
		}
	}
}

func TestSettings_JSONRoundTripAndAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	want := Settings{
		Version:      settingsVersion,
		Theme:        "light",
		BaseFontSize: 20,
		Zoom:         1.5,
		WordWrap:     false,
		LineNumbers:  false,
		Highlight:    false,
		StartupPage:  startupPageBlank,
		RecentFiles:  []string{"C:\\a.txt", "C:\\b.md"},
		WindowWidth:  1280,
		WindowHeight: 720,
	}

	if err := saveSettingsTo(path, want); err != nil {
		t.Fatalf("saveSettingsTo: %v", err)
	}
	got, err := loadSettingsFrom(path)
	if err != nil {
		t.Fatalf("loadSettingsFrom: %v", err)
	}
	if !reflect.DeepEqual(got, want.Normalized()) {
		t.Fatalf("round trip mismatch:\n got=%+v\nwant=%+v", got, want.Normalized())
	}

	// 第二次保存仍只留下目标文件（原子写无残留临时文件）
	if err := saveSettingsTo(path, got); err != nil {
		t.Fatalf("second saveSettingsTo: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "settings.json" {
		t.Fatalf("expected only settings.json, got %v", entries)
	}
}

func TestSettings_MissingAndCorrupt(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.json")
	got, err := loadSettingsFrom(missing)
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if !reflect.DeepEqual(got, DefaultSettings()) {
		t.Fatalf("missing file should return defaults, got %+v", got)
	}

	corrupt := filepath.Join(dir, "corrupt.json")
	if err := os.WriteFile(corrupt, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err = loadSettingsFrom(corrupt)
	if err == nil {
		t.Fatalf("corrupt JSON should return an error")
	}
	if !reflect.DeepEqual(got, DefaultSettings()) {
		t.Fatalf("corrupt JSON should fall back to defaults, got %+v", got)
	}
}

func TestApp_CurrentSettingsReflectsState(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	app.textArea.SetBaseFontSize(20)
	app.textArea.SetWordWrap(false)
	app.textArea.SetShowLineNumbers(false)
	app.highlightEnabled = false
	app.autoZoom = false
	app.textArea.SetZoom(1.5)
	win.SetSize(800, 600)

	got := app.CurrentSettings()
	if got.BaseFontSize != 20 || got.WordWrap || got.LineNumbers || got.Highlight {
		t.Fatalf("editor state not reflected: %+v", got)
	}
	if got.Zoom != 1.5 {
		t.Fatalf("zoom = %v, want 1.5", got.Zoom)
	}
	if got.WindowWidth != 800 || got.WindowHeight != 600 {
		t.Fatalf("window size = %dx%d, want 800x600", got.WindowWidth, got.WindowHeight)
	}
}

func TestApp_ApplySettings(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()
	defer goui.SetTheme(goui.ThemeDark())

	s := DefaultSettings()
	s.Theme = "light"
	s.BaseFontSize = 22
	s.WordWrap = false
	s.LineNumbers = false
	s.Highlight = false
	s.Zoom = 1.5
	app.ApplySettings(s)

	if app.themeID != "light" {
		t.Fatalf("theme should be light")
	}
	if got := app.textArea.GetBaseFontSize(); got != 22 {
		t.Fatalf("base font = %d, want 22", got)
	}
	if app.textArea.IsWordWrap() || app.textArea.IsShowLineNumbers() {
		t.Fatalf("wrap/line numbers should be disabled")
	}
	if app.highlightEnabled || app.textArea.GetHighlighter() != nil {
		t.Fatalf("highlight should be disabled")
	}
	if got := app.textArea.GetZoomPercent(); got != 150 {
		t.Fatalf("zoom = %d%%, want 150%%", got)
	}

	// Zoom=0 恢复跟随系统 DPI
	s.Zoom = 0
	app.ApplySettings(s)
	if !app.autoZoom {
		t.Fatalf("zoom=0 should restore auto zoom")
	}
	if got := app.CurrentSettings().Zoom; got != 0 {
		t.Fatalf("current zoom = %v, want auto (0)", got)
	}
}

func TestSettings_ThemeIDNormalization(t *testing.T) {
	for _, th := range goui.Themes() {
		s := Settings{Theme: th.ID}
		if got := s.Normalized().Theme; got != th.ID {
			t.Fatalf("theme %q normalized to %q", th.ID, got)
		}
	}
	if got := (Settings{Theme: "  NORD  "}).Normalized().Theme; got != "nord" {
		t.Fatalf("theme alias normalized to %q, want nord", got)
	}
	if got := (Settings{Theme: "not-a-theme"}).Normalized().Theme; got != "dark" {
		t.Fatalf("unknown theme normalized to %q, want dark", got)
	}
}

func TestApp_ApplySettings_SwitchesEveryTheme(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()
	defer goui.SetTheme(goui.ThemeDark())

	for _, th := range goui.Themes() {
		s := DefaultSettings()
		s.Theme = th.ID
		app.ApplySettings(s)
		if app.themeID != th.ID {
			t.Fatalf("theme ID = %q, want %q", app.themeID, th.ID)
		}
		if got := goui.CurrentTheme(); got == nil || got.ID != th.ID {
			t.Fatalf("global theme = %+v, want %q", got, th.ID)
		}
		if got := app.CurrentSettings().Theme; got != th.ID {
			t.Fatalf("persisted theme = %q, want %q", got, th.ID)
		}
	}
}

func TestApp_StartupDocumentSelection(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	app.settings.StartupPage = startupPageBlank
	app.textArea.SetText("previous")
	app.showStartupDocument("intro")
	if got := app.textArea.GetText(); got != "" {
		t.Fatalf("blank startup should clear text, got %q", got)
	}
	if app.textArea.IsModified() {
		t.Fatalf("blank startup must not mark the document modified")
	}

	app.settings.StartupPage = startupPageWelcome
	app.showStartupDocument("intro")
	if got := app.textArea.GetText(); got != "intro" {
		t.Fatalf("welcome startup text = %q, want %q", got, "intro")
	}
	if app.textArea.IsModified() {
		t.Fatalf("welcome startup must not mark the document modified")
	}
}
