package component_test

import (
	"testing"

	"github.com/jacksalad/gnote/component"
	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
)

func TestPanelHierarchyAndEvents(t *testing.T) {
	panel := component.NewPanel()
	panel.SetBounds(0, 0, 400, 400)

	btnClicked := false
	btn := component.NewButton("Test")
	btn.SetBounds(10, 10, 100, 40)
	btn.OnClick(func() {
		btnClicked = true
	})

	panel.Add(btn)

	// Verify child added
	if len(panel.GetChildren()) != 1 {
		t.Fatalf("expected 1 child, got %d", len(panel.GetChildren()))
	}

	// Verify GetChildAt
	child := panel.GetChildAt(20, 20)
	if child != btn {
		t.Fatalf("expected btn at (20, 20), got %+v", child)
	}

	// Mouse Down event inside button
	downEvt := event.NewEvent(event.EventMouseDown, event.MouseData{
		X:      20,
		Y:      20,
		Button: event.MouseButtonLeft,
	})
	handled := panel.OnEvent(downEvt)
	if !handled {
		t.Fatalf("expected mouse down to be handled by button")
	}

	// Mouse Up event inside button (triggers click)
	upEvt := event.NewEvent(event.EventMouseUp, event.MouseData{
		X:      20,
		Y:      20,
		Button: event.MouseButtonLeft,
	})
	handled = panel.OnEvent(upEvt)
	if !handled {
		t.Fatalf("expected mouse up to be handled by button")
	}
	if !btnClicked {
		t.Fatalf("expected button onClick to be triggered")
	}

	// Render test
	canvas := render.NewCanvas(400, 400)
	panel.Render(canvas)
}

func TestTextBoxEditing(t *testing.T) {
	txt := component.NewTextBox()
	txt.SetBounds(0, 0, 200, 30)

	// Focus textbox
	downEvt := event.NewEvent(event.EventMouseDown, event.MouseData{
		X:      10,
		Y:      10,
		Button: event.MouseButtonLeft,
	})
	if !txt.OnEvent(downEvt) {
		t.Fatalf("expected click to be handled")
	}
	if !txt.IsFocused() {
		t.Fatalf("expected textbox to be focused")
	}

	// Input 'A'
	keyEvt := event.NewEvent(event.EventKeyPress, event.KeyData{
		Char: 'A',
	})
	if !txt.OnEvent(keyEvt) {
		t.Fatalf("expected key press to be handled")
	}
	if txt.GetText() != "A" {
		t.Fatalf("expected text 'A', got '%s'", txt.GetText())
	}

	// Input 'B'
	keyEvt2 := event.NewEvent(event.EventKeyPress, event.KeyData{
		Char: 'B',
	})
	txt.OnEvent(keyEvt2)
	if txt.GetText() != "AB" {
		t.Fatalf("expected text 'AB', got '%s'", txt.GetText())
	}

	// Backspace
	backEvt := event.NewEvent(event.EventKeyDown, event.KeyData{
		KeyCode: 0x08, // VK_BACK
	})
	txt.OnEvent(backEvt)
	if txt.GetText() != "A" {
		t.Fatalf("expected text 'A' after backspace, got '%s'", txt.GetText())
	}
}

func TestTextBoxChineseAndUnicode(t *testing.T) {
	txt := component.NewTextBox()
	txt.SetBounds(0, 0, 200, 30)

	// Focus
	txt.OnEvent(event.NewEvent(event.EventMouseDown, event.MouseData{X: 10, Y: 10, Button: event.MouseButtonLeft}))

	// Type '你'
	txt.OnEvent(event.NewEvent(event.EventKeyPress, event.KeyData{Char: '你'}))
	// Type '好'
	txt.OnEvent(event.NewEvent(event.EventKeyPress, event.KeyData{Char: '好'}))

	if txt.GetText() != "你好" {
		t.Fatalf("expected '你好', got '%s'", txt.GetText())
	}

	// Backspace deletes one rune
	txt.OnEvent(event.NewEvent(event.EventKeyDown, event.KeyData{KeyCode: 0x08}))
	if txt.GetText() != "你" {
		t.Fatalf("expected '你' after backspace, got '%s'", txt.GetText())
	}
}

func TestFlowLayout(t *testing.T) {
	panel := component.NewPanel()
	panel.SetBounds(0, 0, 300, 200)
	layout := component.FlowLayout{Gap: 5}
	panel.SetLayout(&layout)

	btn1 := component.NewButton("1")
	btn1.SetBounds(0, 0, 50, 30)
	btn2 := component.NewButton("2")
	btn2.SetBounds(0, 0, 50, 30)

	panel.Add(btn1)
	panel.Add(btn2)
	panel.DoLayout()

	b1 := btn1.GetBounds()
	b2 := btn2.GetBounds()

	if b1.X != 0 || b1.Y != 0 {
		t.Fatalf("unexpected b1 bounds: %+v", b1)
	}
	if b2.X != 55 || b2.Y != 0 {
		t.Fatalf("unexpected b2 bounds: %+v", b2)
	}
	_ = graphics.ColorWhite
}

func TestPanelRecursiveDoLayout(t *testing.T) {
	root := component.NewPanel()
	root.SetBounds(0, 0, 400, 400)

	childPanel := component.NewPanel()
	childPanel.SetBounds(10, 10, 200, 200)
	childPanel.SetLayout(&component.GridLayout{Rows: 2, Cols: 2, Gap: 10})

	btn1 := component.NewButton("1")
	btn2 := component.NewButton("2")
	btn3 := component.NewButton("3")
	btn4 := component.NewButton("4")

	childPanel.Add(btn1)
	childPanel.Add(btn2)
	childPanel.Add(btn3)
	childPanel.Add(btn4)

	root.Add(childPanel)

	// Trigger recursive layout from root
	root.DoLayout()

	b1 := btn1.GetBounds()
	if b1.Width <= 0 || b1.Height <= 0 {
		t.Fatalf("expected btn1 to have positive dimensions after recursive DoLayout, got: %+v", b1)
	}
	if b1.X != 10 || b1.Y != 10 {
		t.Fatalf("expected btn1 at (10, 10), got: %+v", b1)
	}
}

func TestPanelSetBoundsPropagate(t *testing.T) {
	panel := component.NewPanel()
	panel.SetBounds(10, 10, 200, 200)

	btn := component.NewButton("A")
	btn.SetBounds(20, 20, 50, 30)
	panel.Add(btn)

	// Move panel by +30, +40
	panel.SetBounds(40, 50, 200, 200)

	bb := btn.GetBounds()
	if bb.X != 50 || bb.Y != 60 {
		t.Fatalf("expected btn to shift to (50, 60), got: %+v", bb)
	}
}

func TestCheckBoxIgnoresRightClick(t *testing.T) {
	cb := component.NewCheckBox("check")
	cb.SetBounds(0, 0, 120, 24)

	cb.OnEvent(event.NewEvent(event.EventMouseDown, event.MouseData{X: 5, Y: 5, Button: event.MouseButtonRight}))
	if cb.IsChecked() {
		t.Fatal("right click must not toggle checkbox")
	}
	cb.OnEvent(event.NewEvent(event.EventMouseDown, event.MouseData{X: 5, Y: 5, Button: event.MouseButtonLeft}))
	if !cb.IsChecked() {
		t.Fatal("left click should toggle checkbox")
	}
}

func TestRadioButtonIgnoresRightClick(t *testing.T) {
	rb := component.NewRadioButton("radio")
	rb.SetBounds(0, 0, 120, 24)

	rb.OnEvent(event.NewEvent(event.EventMouseDown, event.MouseData{X: 5, Y: 5, Button: event.MouseButtonRight}))
	if rb.IsSelected() {
		t.Fatal("right click must not select radio button")
	}
	rb.OnEvent(event.NewEvent(event.EventMouseDown, event.MouseData{X: 5, Y: 5, Button: event.MouseButtonLeft}))
	if !rb.IsSelected() {
		t.Fatal("left click should select radio button")
	}
}

func TestRequestFocusClearsSiblingFocus(t *testing.T) {
	panel := component.NewPanel()
	panel.SetBounds(0, 0, 300, 200)
	a := component.NewTextBox()
	a.SetBounds(0, 0, 120, 24)
	b := component.NewTextBox()
	b.SetBounds(0, 30, 120, 24)
	panel.Add(a)
	panel.Add(b)

	a.SetFocused(true)
	b.SetFocused(true)
	component.RequestFocus(a)

	if !a.IsFocused() {
		t.Fatal("target should own focus")
	}
	if b.IsFocused() {
		t.Fatal("sibling focus should be cleared")
	}
}

// 默认字体必须在 SetDefaultFont 之后立刻对新建控件生效（内嵌字体替换场景）。
func TestDefaultFontFollowsGlobalSetter(t *testing.T) {
	oldName, oldSize := graphics.DefaultFont.Name, graphics.DefaultFont.Size
	t.Cleanup(func() { graphics.SetDefaultFont(oldName, oldSize) })

	graphics.SetDefaultFont("Gnote Mono", 17)

	label := component.NewLabel("x")
	if got := label.GetFont(); got.Name != "Gnote Mono" || got.Size != 17 {
		t.Fatalf("label font = %s %d, want Gnote Mono 17", got.Name, got.Size)
	}
	button := component.NewButton("ok")
	if got := button.GetFont(); got.Name != "Gnote Mono" || got.Size != 17 {
		t.Fatalf("button font = %s %d, want Gnote Mono 17", got.Name, got.Size)
	}
}
