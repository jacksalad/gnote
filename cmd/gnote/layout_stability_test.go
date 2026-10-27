//go:build windows

package main

import (
	"testing"

	goui "github.com/jacksalad/gnote"
	"github.com/jacksalad/gnote/event"
)

func buildRealRoot(t *testing.T, clicks map[string]int) (*goui.Panel, *goui.TextArea) {
	widths := []int32{120, 120, 120, 180, 60, 60, 92, 90, 120, 80, 80, 70, 90}
	names := []string{"新建", "打开", "保存", "另存为", "撤销", "重做", "行号", "换行", "高亮", "放大", "缩小", "重置", "主题"}

	toolbar := goui.NewSurfacePanel()
	toolbar.SetBounds(0, 0, 1560, 44)
	toolbar.SetPadding(6)
	flow := goui.NewFlowLayout()
	flow.Gap = 6
	flow.HAlign = goui.HAlignStart
	toolbar.SetLayout(flow)
	textArea := goui.NewTextArea()
	textArea.SetBounds(0, 0, 1560, 800)
	textArea.SetFocused(true)
	for i, w := range widths {
		b := goui.NewButton(names[i])
		b.SetBounds(0, 0, w, 32)
		toolbar.Add(b)
		if i == 0 {
			b.OnClick(func() { clicks["new"]++ })
		}
		if i == 4 {
			b.OnClick(func() { clicks["undo"]++; textArea.Undo() })
		}
	}
	statusBar := goui.NewSurfacePanel()
	statusBar.SetBounds(0, 0, 1560, 28)

	root := goui.NewBackgroundPanel()
	root.SetLayout(&goui.BorderLayout{Top: toolbar, Center: textArea, Bottom: statusBar})
	root.Add(toolbar)
	root.Add(textArea)
	root.Add(statusBar)
	root.SetBounds(0, 0, 1248, 735)
	return root, textArea
}

func clickAt(root *goui.Panel, x, y int32) bool {
	down := root.OnEvent(event.NewEvent(event.EventMouseDown, event.MouseData{X: x, Y: y, Button: event.MouseButtonLeft}))
	root.OnEvent(event.NewEvent(event.EventMouseUp, event.MouseData{X: x, Y: y, Button: event.MouseButtonLeft}))
	return down
}

func TestGnote_RealToolbarLayoutClickAndUndo(t *testing.T) {
	clicks := map[string]int{}
	root, ta := buildRealRoot(t, clicks)

	ta.OnEvent(event.NewEvent(event.EventKeyPress, event.KeyData{Char: 'A'}))
	ta.OnEvent(event.NewEvent(event.EventKeyPress, event.KeyData{Char: 'B'}))
	if got := ta.GetText(); got != "AB" {
		t.Fatalf("typing failed: %q", got)
	}
	if !ta.IsModified() {
		t.Fatalf("expected modified after typing")
	}

	if !clickAt(root, 600, 22) {
		t.Fatalf("undo click not consumed")
	}
	t.Logf("clicks=%v text=%q modified=%v focused=%v", clicks, ta.GetText(), ta.IsModified(), ta.IsFocused())
	if clicks["undo"] != 1 {
		t.Fatalf("undo button click not fired")
	}
	// 连续输入合并为一步撤销：一次点击撤销整段 "AB"
	if got := ta.GetText(); got != "" {
		t.Fatalf("undo did not remove the typing run: %q", got)
	}
	if ta.IsModified() {
		t.Fatalf("modified flag should be restored to false after undoing all edits")
	}
	if !clickAt(root, 600, 22) {
		t.Fatalf("second undo click not consumed")
	}
	if got := ta.GetText(); got != "" {
		t.Fatalf("no-op undo changed text: %q", got)
	}

	// 工具栏点击后继续键入
	if !ta.OnEvent(event.NewEvent(event.EventKeyPress, event.KeyData{Char: 'Z'})) {
		t.Fatalf("editor lost focus after undo click")
	}
	if got := ta.GetText(); got != "Z" {
		t.Fatalf("typing after undo click failed: %q", got)
	}
}
