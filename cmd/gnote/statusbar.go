//go:build windows

package main

import (
	goui "github.com/jacksalad/gnote"
	"github.com/jacksalad/gnote/textio"
)

// statusPopup 在状态段上方弹出原生菜单；系统会把菜单限制在屏幕范围内。
func (a *App) statusPopup(label *goui.Label, items []goui.MenuItem) uint32 {
	if label == nil {
		return 0
	}
	b := label.GetBounds()
	return goui.ShowPopupMenu(a.win.Hwnd(), b.X, b.Y-4, items)
}

func (a *App) showEncodingMenu() {
	acp := textio.GetACP()
	ansi := textio.Encoding{Kind: textio.KindANSI, CodePage: acp}
	choices := []struct {
		label string
		enc   textio.Encoding
	}{
		{"UTF-8", textio.DefaultEncoding()},
		{"UTF-8 BOM", textio.Encoding{Kind: textio.KindUTF8BOM}},
		{"ANSI (" + ansi.Name() + ")", ansi},
		{"UTF-16 LE", textio.Encoding{Kind: textio.KindUTF16LE}},
		{"UTF-16 BE", textio.Encoding{Kind: textio.KindUTF16BE}},
	}

	items := make([]goui.MenuItem, 0, len(choices))
	for i, choice := range choices {
		checked := a.currentEncoding.Kind == choice.enc.Kind
		if choice.enc.Kind == textio.KindANSI {
			checked = a.currentEncoding.Kind == textio.KindANSI && a.currentEncoding.CodePage == choice.enc.CodePage
		}
		items = append(items, goui.MenuItem{
			ID:      uint32(i + 1),
			Text:    choice.label,
			Checked: checked,
		})
	}

	chosen := a.statusPopup(a.lblEncoding, items)
	if chosen == 0 || int(chosen) > len(choices) {
		return
	}
	next := choices[chosen-1].enc
	if next == a.currentEncoding {
		return
	}
	a.currentEncoding = next
	// 编码属于保存输出的一部分：切换后标记为已修改，避免关闭时丢失用户意图。
	a.textArea.SetModified(true)
	a.UpdateHeaderAndStatus()
	a.win.RequestRedraw()
}

func (a *App) showEOLMenu() {
	choices := []struct {
		label string
		eol   textio.EOL
	}{
		{"CRLF", textio.EOL{CRLF: 1}},
		{"LF", textio.EOL{LF: 1}},
		{"CR", textio.EOL{CR: 1}},
	}
	items := make([]goui.MenuItem, 0, len(choices))
	currentKind := a.currentEOL.Kind()
	for i, choice := range choices {
		items = append(items, goui.MenuItem{
			ID:      uint32(i + 1),
			Text:    choice.label,
			Checked: currentKind == choice.eol.Kind(),
		})
	}

	chosen := a.statusPopup(a.lblEOL, items)
	if chosen == 0 || int(chosen) > len(choices) {
		return
	}
	next := choices[chosen-1].eol
	if next == a.currentEOL {
		return
	}
	a.currentEOL = next
	a.textArea.SetLineEnding(a.currentEOL.Sequence())
	a.textArea.SetModified(true)
	a.UpdateHeaderAndStatus()
	a.win.RequestRedraw()
}

func (a *App) showLanguageMenu() {
	choices := []struct {
		name string
		h    goui.Highlighter
	}{
		{"纯文本", nil},
		{"Go", goui.NewGoHighlighter()},
		{"Python", goui.NewPythonHighlighter()},
		{"C/C++", goui.NewCHighlighter()},
		{"C#", goui.NewCSharpHighlighter()},
		{"Java", goui.NewJavaHighlighter()},
		{"Rust", goui.NewRustHighlighter()},
		{"R", goui.NewRHighlighter()},
		{"JavaScript", goui.NewJSHighlighter()},
		{"TypeScript", goui.NewTypeScriptHighlighter()},
		{"HTML", goui.NewHTMLHighlighter()},
		{"CSS", goui.NewCSSHighlighter()},
		{"SQL", goui.NewSQLHighlighter()},
		{"Markdown", goui.NewMarkdownHighlighter()},
		{"JSON", goui.NewJSONHighlighter()},
		{"YAML", goui.NewYAMLHighlighter()},
		{"TOML", goui.NewTOMLHighlighter()},
	}

	current := "纯文本"
	if a.highlightEnabled && a.currentHighlighter != nil {
		current = goui.GetLanguageName(a.currentHighlighter)
	}

	items := make([]goui.MenuItem, 0, len(choices))
	for i, choice := range choices {
		items = append(items, goui.MenuItem{
			ID:      uint32(i + 1),
			Text:    choice.name,
			Checked: current == choice.name,
		})
	}

	chosen := a.statusPopup(a.lblLanguage, items)
	if chosen == 0 || int(chosen) > len(choices) {
		return
	}
	next := choices[chosen-1].h
	if next == nil {
		a.currentHighlighter = nil
		a.highlightEnabled = false
		a.textArea.SetHighlighter(nil)
	} else {
		a.currentHighlighter = next
		a.highlightEnabled = true
		a.textArea.SetHighlighter(next)
	}
	a.UpdateHeaderAndStatus()
	a.win.RequestRedraw()
}

func (a *App) showZoomMenu() {
	choices := []struct {
		label   string
		percent int
		auto    bool
	}{
		{"跟随系统 DPI", 0, true},
		{"50%", 50, false},
		{"75%", 75, false},
		{"100%", 100, false},
		{"125%", 125, false},
		{"150%", 150, false},
		{"200%", 200, false},
		{"300%", 300, false},
	}
	items := make([]goui.MenuItem, 0, len(choices))
	currentPercent := a.textArea.GetZoomPercent()
	for i, choice := range choices {
		checked := false
		if choice.auto {
			checked = a.autoZoom
		} else {
			checked = !a.autoZoom && currentPercent == choice.percent
		}
		items = append(items, goui.MenuItem{
			ID:      uint32(i + 1),
			Text:    choice.label,
			Checked: checked,
		})
	}

	chosen := a.statusPopup(a.lblZoom, items)
	if chosen == 0 || int(chosen) > len(choices) {
		return
	}
	choice := choices[chosen-1]
	if choice.auto {
		a.applyingSettings = true
		if scale := a.win.GetScale(); scale > 1.05 {
			a.textArea.SetZoom(scale)
		} else {
			a.textArea.SetZoom(1)
		}
		a.applyingSettings = false
		a.autoZoom = true
	} else {
		a.autoZoom = false
		a.textArea.SetZoom(float64(choice.percent) / 100)
	}
	a.UpdateHeaderAndStatus()
	a.win.RequestRedraw()
}
