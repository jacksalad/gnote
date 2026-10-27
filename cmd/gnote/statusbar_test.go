//go:build windows

package main

import (
	"testing"

	goui "github.com/jacksalad/gnote"
)

func TestApp_StatusBarLayoutNoOverlapAtNarrowWidth(t *testing.T) {
	app, win := newTestAppWindow(t, 640, 400)
	defer win.Destroy()

	app.root.SetBounds(0, 0, 640, 400)
	app.updateLayout()
	app.UpdateHeaderAndStatus()

	labels := []*goui.Label{
		app.lblFileStatus,
		app.lblPosStatus,
		app.lblEncoding,
		app.lblEOL,
		app.lblLanguage,
		app.lblZoom,
	}
	visible := make([]*goui.Label, 0, len(labels))
	for _, label := range labels {
		if label.IsVisible() {
			visible = append(visible, label)
		}
	}
	for i, label := range visible {
		b := label.GetBounds()
		if b.Width <= 0 || b.Height <= 0 {
			t.Fatalf("status label %d has empty bounds: %+v", i, b)
		}
		if b.X < 0 || b.Right() > 640 {
			t.Fatalf("status label %d outside window: %+v", i, b)
		}
		if i > 0 {
			prev := visible[i-1].GetBounds()
			if prev.Right() > b.X {
				t.Fatalf("status labels overlap: %+v and %+v", prev, b)
			}
		}
	}

	if got := app.lblEncoding.GetText(); got == "" {
		t.Fatalf("encoding label is empty")
	}
	if got := app.lblEOL.GetText(); got == "" {
		t.Fatalf("EOL label is empty")
	}
	if got := app.lblZoom.GetText(); got == "" {
		t.Fatalf("zoom label is empty")
	}
}

func TestApp_UIScaleMatchesWindowDPI(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	if got, want := app.menuBar.BarHeight(), app.scale(34); got != want {
		t.Fatalf("menu bar height = %d, want %d", got, want)
	}
	if got, want := app.statusHeight(), app.scale(28); got != want {
		t.Fatalf("status height = %d, want %d", got, want)
	}

	sw, err := newSettingsWindow(app)
	if err != nil {
		t.Fatalf("newSettingsWindow: %v", err)
	}
	defer func() {
		if goui.IsWindow(sw.win.Hwnd()) {
			sw.win.Destroy()
		}
	}()
	if w, h := sw.win.GetSize(); w != sw.windowWidth() || h != sw.windowHeight() {
		t.Fatalf("settings client = %dx%d, want %dx%d", w, h, sw.windowWidth(), sw.windowHeight())
	}
}

func TestApp_ApplyDPIScaleRescalesChrome(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	before := app.menuBar.BarHeight()
	app.applyDPIScale(192)

	if app.uiScale != 2 {
		t.Fatalf("uiScale = %v, want 2", app.uiScale)
	}
	if got, want := app.menuBar.BarHeight(), app.scale(34); got != want {
		t.Fatalf("menu height = %d, want %d", got, want)
	}
	if got, want := app.statusHeight(), app.scale(28); got != want {
		t.Fatalf("status height = %d, want %d", got, want)
	}
	if app.menuBar.BarHeight() <= before {
		t.Fatalf("menu height did not grow after DPI increase")
	}
}

func TestApp_StatusBarLabelsClickable(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	for _, label := range []*goui.Label{app.lblEncoding, app.lblEOL, app.lblLanguage, app.lblZoom} {
		if got := label.GetCursor(); got != 32649 {
			t.Fatalf("status segment %q cursor = %d, want hand", label.GetText(), got)
		}
	}
}
