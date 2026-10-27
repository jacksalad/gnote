//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jacksalad/gnote/event"
)

func TestApp_LoadJSONFileThenFormat(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	path := filepath.Join(t.TempDir(), "data.json")
	if err := os.WriteFile(path, []byte(`{"a":[1,2]}`), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if !app.LoadFile(path) {
		t.Fatal("LoadFile failed")
	}
	if app.commands.Enabled(cmdEditFormat) != true {
		t.Fatal("format must be enabled after opening a .json file")
	}

	win.EventBus().Publish(event.NewEvent(event.EventKeyDown, event.KeyData{
		KeyCode: 'F', Ctrl: true, Shift: true,
	}))

	const want = "{\r\n    \"a\": [\r\n        1,\r\n        2\r\n    ]\r\n}"
	if got := app.textArea.GetText(); got != want {
		t.Fatalf("unexpected formatted file:\n got %q\nwant %q", got, want)
	}
	if !app.textArea.IsModified() {
		t.Fatal("formatting an opened file must mark it modified")
	}
}

func TestApp_FormatCommandEnabledOnlyForJSONAndHTML(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	app.currentFilePath = `C:\docs\data.json`
	if !app.commands.Enabled(cmdEditFormat) {
		t.Fatal("format must be enabled for .json")
	}
	app.currentFilePath = `C:\docs\index.HTM`
	if !app.commands.Enabled(cmdEditFormat) {
		t.Fatal("format must be enabled for .htm (case-insensitive)")
	}
	app.currentFilePath = `C:\docs\main.go`
	if app.commands.Enabled(cmdEditFormat) {
		t.Fatal("format must be disabled for .go")
	}
	app.currentFilePath = ""
	if app.commands.Enabled(cmdEditFormat) {
		t.Fatal("format must be disabled for untitled documents")
	}
	app.currentFilePath = `C:\docs\data.json`
	app.textArea.SetReadOnly(true)
	if app.commands.Enabled(cmdEditFormat) {
		t.Fatal("format must be disabled for read-only documents")
	}
}

func TestApp_CtrlShiftFFormatsJSONAsSingleUndo(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	app.currentFilePath = `C:\docs\data.json`
	app.textArea.SetText(`{"b":[1,2],"a":null}`)
	app.textArea.SetModified(false)

	win.EventBus().Publish(event.NewEvent(event.EventKeyDown, event.KeyData{
		KeyCode: 'F', Ctrl: true, Shift: true,
	}))

	const want = "{\r\n    \"b\": [\r\n        1,\r\n        2\r\n    ],\r\n    \"a\": null\r\n}"
	if got := app.textArea.GetText(); got != want {
		t.Fatalf("unexpected formatted JSON:\n got %q\nwant %q", got, want)
	}
	if !app.textArea.IsModified() {
		t.Fatal("formatting must mark the document modified")
	}
	if app.findBar.visible {
		t.Fatal("Ctrl+Shift+F must not open the find bar")
	}
	if !app.textArea.Undo() {
		t.Fatal("format must be undoable")
	}
	if got := app.textArea.GetText(); got != `{"b":[1,2],"a":null}` {
		t.Fatalf("undo did not restore the original document: %q", got)
	}
}

func TestApp_CtrlShiftFFormatsHTML(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	app.currentFilePath = `C:\docs\page.html`
	app.textArea.SetText(`<div><ul><li>One<li>Two</ul><img src="a.png"></div>`)

	win.EventBus().Publish(event.NewEvent(event.EventKeyDown, event.KeyData{
		KeyCode: 'F', Ctrl: true, Shift: true,
	}))

	const want = "<div>\r\n    <ul>\r\n        <li>One\r\n        <li>Two\r\n    </ul>\r\n    <img src=\"a.png\">\r\n</div>"
	if got := app.textArea.GetText(); got != want {
		t.Fatalf("unexpected formatted HTML:\n got %q\nwant %q", got, want)
	}
}

func TestApp_CtrlShiftFFormatsEmbeddedCSSAndJS(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	app.currentFilePath = `C:\docs\page.html`
	app.textArea.SetText(`<div><style>a{color:red}</style><script>const x=1</script></div>`)

	win.EventBus().Publish(event.NewEvent(event.EventKeyDown, event.KeyData{
		KeyCode: 'F', Ctrl: true, Shift: true,
	}))

	const want = "<div>\r\n    <style>\r\n        a {\r\n            color: red;\r\n        }\r\n    </style>\r\n    <script>\r\n        const x = 1\r\n    </script>\r\n</div>"
	if got := app.textArea.GetText(); got != want {
		t.Fatalf("unexpected formatted HTML:\n got %q\nwant %q", got, want)
	}
}

func TestApp_CtrlFStillOpensFindBar(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	win.EventBus().Publish(event.NewEvent(event.EventKeyDown, event.KeyData{
		KeyCode: 'F', Ctrl: true,
	}))
	if !app.findBar.visible {
		t.Fatal("Ctrl+F must still open the find bar")
	}
}

func TestApp_FormatNoOpLeavesDocumentUnmodified(t *testing.T) {
	app, win := newTestAppWindow(t, 900, 640)
	defer win.Destroy()

	app.currentFilePath = `C:\docs\data.json`
	app.textArea.SetText("{\n    \"a\": 1\n}")
	app.textArea.SetModified(false)

	win.EventBus().Publish(event.NewEvent(event.EventKeyDown, event.KeyData{
		KeyCode: 'F', Ctrl: true, Shift: true,
	}))
	if app.textArea.IsModified() {
		t.Fatal("formatting an already formatted document must not dirty it")
	}
	if app.textArea.CanUndo() {
		t.Fatal("no-op format must not create undo history")
	}
}
