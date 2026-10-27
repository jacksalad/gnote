package component

import (
	"testing"

	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/theme"
)

func buildTestMenuBar(clicks map[string]int) *MenuBar {
	m := NewMenuBar()
	m.SetBounds(0, 0, 800, 400)
	m.SetFont(DefaultFont)
	m.SetMenus([]Menu{
		{
			Text: "文件",
			Items: []MenuEntry{
				{Text: "新建", Accel: "Ctrl+N", OnClick: func() { clicks["new"]++ }},
				{Separator: true},
				{Text: "退出", Accel: "Alt+F4", Disabled: true, OnClick: func() { clicks["quit"]++ }},
				{
					Text: "最近文件",
					Submenu: []MenuEntry{
						{Text: "a.txt", OnClick: func() { clicks["recent"]++ }},
					},
				},
			},
		},
		{
			Text: "编辑",
			Items: []MenuEntry{
				{Text: "撤销", Accel: "Ctrl+Z", Checked: true, OnClick: func() { clicks["undo"]++ }},
				{Text: "重做", Accel: "Ctrl+Y", OnClick: func() { clicks["redo"]++ }},
			},
		},
	})
	return m
}

func menuRowCenter(m *MenuBar, menuIndex, itemIndex int) (int32, int32) {
	rect := m.menuRect(menuIndex)
	y := rect.Y + 4
	for i := 0; i < itemIndex; i++ {
		if m.menus[menuIndex].Items[i].Separator {
			y += m.separatorGap
		} else {
			y += m.rowHeight
		}
	}
	h := m.rowHeight
	if m.menus[menuIndex].Items[itemIndex].Separator {
		h = m.separatorGap
	}
	return rect.X + rect.Width/2, y + h/2
}

func barCenter(m *MenuBar, index int) (int32, int32) {
	r := m.barItemRect(index)
	return r.X + r.Width/2, r.Y + r.Height/2
}

func clickMenuBar(m *MenuBar, x, y int32) bool {
	return m.OnEvent(event.NewEvent(event.EventMouseDown, event.MouseData{
		X: x, Y: y, Button: event.MouseButtonLeft,
	}))
}

func TestMenuBar_ClickOpensAndExecutes(t *testing.T) {
	clicks := map[string]int{}
	m := buildTestMenuBar(clicks)

	x, y := barCenter(m, 0)
	if !clickMenuBar(m, x, y) {
		t.Fatalf("clicking the menu title should be consumed")
	}
	if !m.IsOpen() {
		t.Fatalf("menu should be open after clicking its title")
	}

	ix, iy := menuRowCenter(m, 0, 0)
	if !clickMenuBar(m, ix, iy) {
		t.Fatalf("clicking a menu item should be consumed")
	}
	if clicks["new"] != 1 {
		t.Fatalf("new command fired %d times, want 1", clicks["new"])
	}
	if m.IsOpen() {
		t.Fatalf("menu should close after executing an item")
	}
}

func TestMenuBar_ClickSameTitleTogglesClosed(t *testing.T) {
	m := buildTestMenuBar(map[string]int{})
	x, y := barCenter(m, 0)
	clickMenuBar(m, x, y)
	if !m.IsOpen() {
		t.Fatalf("menu should open")
	}
	clickMenuBar(m, x, y)
	if m.IsOpen() {
		t.Fatalf("clicking the open title again should close the menu")
	}
}

func TestMenuBar_DisabledItemDoesNotRunAndKeepsMenuOpen(t *testing.T) {
	clicks := map[string]int{}
	m := buildTestMenuBar(clicks)

	x, y := barCenter(m, 0)
	clickMenuBar(m, x, y)

	ix, iy := menuRowCenter(m, 0, 2)
	clickMenuBar(m, ix, iy)
	if clicks["quit"] != 0 {
		t.Fatalf("disabled item must not run")
	}
	if !m.IsOpen() {
		t.Fatalf("clicking a disabled item should keep the menu open")
	}
}

func TestMenuBar_SubmenuHoverAndClick(t *testing.T) {
	clicks := map[string]int{}
	m := buildTestMenuBar(clicks)

	x, y := barCenter(m, 0)
	clickMenuBar(m, x, y)

	ix, iy := menuRowCenter(m, 0, 3)
	m.OnEvent(event.NewEvent(event.EventMouseMove, event.MouseData{X: ix, Y: iy}))
	if m.submenuOpen != 3 {
		t.Fatalf("submenuOpen = %d, want 3", m.submenuOpen)
	}

	sub := m.submenuRect(0, 3)
	sx := sub.X + sub.Width/2
	sy := sub.Y + 4 + m.rowHeight/2
	clickMenuBar(m, sx, sy)
	if clicks["recent"] != 1 {
		t.Fatalf("recent command fired %d times, want 1", clicks["recent"])
	}
	if m.IsOpen() {
		t.Fatalf("menu should close after submenu activation")
	}
}

func TestMenuBar_KeyboardNavigation(t *testing.T) {
	clicks := map[string]int{}
	m := buildTestMenuBar(clicks)

	x, y := barCenter(m, 1) // 编辑
	clickMenuBar(m, x, y)

	// 第一项“撤销”默认聚焦，Enter 执行
	if !m.OnEvent(event.NewEvent(event.EventKeyDown, event.KeyData{KeyCode: 0x0D})) {
		t.Fatalf("Enter should be consumed while menu is open")
	}
	if clicks["undo"] != 1 {
		t.Fatalf("undo fired %d times, want 1", clicks["undo"])
	}
	if m.IsOpen() {
		t.Fatalf("Enter should close the menu")
	}
}

func TestMenuBar_EscapeCloses(t *testing.T) {
	m := buildTestMenuBar(map[string]int{})
	x, y := barCenter(m, 0)
	clickMenuBar(m, x, y)

	if !m.OnEvent(event.NewEvent(event.EventKeyDown, event.KeyData{KeyCode: 0x1B})) {
		t.Fatalf("Esc should be consumed while menu is open")
	}
	if m.IsOpen() {
		t.Fatalf("Esc should close the menu")
	}
}

func TestMenuBar_ClickOutsideClosesAndIsConsumed(t *testing.T) {
	m := buildTestMenuBar(map[string]int{})
	x, y := barCenter(m, 0)
	clickMenuBar(m, x, y)

	if !clickMenuBar(m, 700, 380) {
		t.Fatalf("outside click should be consumed while menu is open")
	}
	if m.IsOpen() {
		t.Fatalf("outside click should close the menu")
	}
}

func TestMenuBar_ClosedBelowBarPassesThrough(t *testing.T) {
	m := buildTestMenuBar(map[string]int{})
	if m.OnEvent(event.NewEvent(event.EventMouseDown, event.MouseData{
		X: 300, Y: 200, Button: event.MouseButtonLeft,
	})) {
		t.Fatalf("closed menu must pass editor clicks through")
	}
	if m.OnEvent(event.NewEvent(event.EventKeyDown, event.KeyData{KeyCode: 'A'})) {
		t.Fatalf("closed menu must pass keyboard events through")
	}
}

func TestMenuBar_PopupClampsInsideNarrowWindow(t *testing.T) {
	m := buildTestMenuBar(map[string]int{})
	m.SetBounds(0, 0, 240, 320)
	m.Open(1)

	rect := m.menuRect(1)
	if rect.X < 0 || rect.Right() > 240 {
		t.Fatalf("popup x range = [%d,%d], want inside [0,240]", rect.X, rect.Right())
	}
	if rect.Y < m.barHeight || rect.Bottom() > 320 {
		t.Fatalf("popup y range = [%d,%d], want below bar and inside window", rect.Y, rect.Bottom())
	}
}

func TestMenuBar_RendersBarAndPopup(t *testing.T) {
	m := buildTestMenuBar(map[string]int{})
	canvas := render.NewCanvas(800, 400)
	canvas.Clear(theme.Current().Background)
	m.Open(0)
	m.Render(canvas)

	background := canvas.Buffer[0]
	changed := false
	for _, pixel := range canvas.Buffer {
		if pixel != background {
			changed = true
			break
		}
	}
	if !changed {
		t.Fatalf("menu bar render produced no visible pixels")
	}
}
