package component

import (
	"testing"

	"github.com/jacksalad/gnote/event"
)

func clickSettingsControl(c Component, x, y int32) bool {
	return c.OnEvent(event.NewEvent(event.EventMouseDown, event.MouseData{
		X: x, Y: y, Button: event.MouseButtonLeft,
	}))
}

func TestSwitch_Toggles(t *testing.T) {
	s := NewSwitch()
	s.SetBounds(0, 0, 40, 22)
	var changes []bool
	s.SetOnChange(func(on bool) { changes = append(changes, on) })

	if !clickSettingsControl(s, 20, 11) || !s.IsOn() {
		t.Fatalf("first click should turn switch on")
	}
	if !clickSettingsControl(s, 30, 11) || s.IsOn() {
		t.Fatalf("second click should turn switch off")
	}
	if len(changes) != 2 || !changes[0] || changes[1] {
		t.Fatalf("unexpected change callbacks: %v", changes)
	}
	if clickSettingsControl(s, 100, 11) {
		t.Fatalf("click outside must not toggle")
	}
}

func TestSegmented_SelectsOption(t *testing.T) {
	s := NewSegmented([]string{"深色", "浅色"})
	s.SetBounds(0, 0, 200, 30)
	var picked []int
	s.OnChange(func(i int) { picked = append(picked, i) })

	if !clickSettingsControl(s, 150, 15) {
		t.Fatalf("click should be consumed")
	}
	if s.Selected() != 1 {
		t.Fatalf("selected = %d, want 1", s.Selected())
	}
	if len(picked) != 1 || picked[0] != 1 {
		t.Fatalf("unexpected callbacks: %v", picked)
	}
	clickSettingsControl(s, 10, 15)
	if s.Selected() != 0 || len(picked) != 2 {
		t.Fatalf("second selection failed: selected=%d callbacks=%v", s.Selected(), picked)
	}
}

func TestTabBar_SelectsPage(t *testing.T) {
	tb := NewTabBar([]string{"编辑器", "外观", "关于"})
	tb.SetBounds(0, 0, 160, 200)
	var picked []int
	tb.OnSelect(func(i int) { picked = append(picked, i) })

	if !clickSettingsControl(tb, 40, tb.rowH+10) {
		t.Fatalf("click should be consumed")
	}
	if tb.Selected() != 1 {
		t.Fatalf("selected = %d, want 1", tb.Selected())
	}
	if len(picked) != 1 || picked[0] != 1 {
		t.Fatalf("unexpected callbacks: %v", picked)
	}
	clickSettingsControl(tb, 40, 10)
	if tb.Selected() != 0 {
		t.Fatalf("first tab selection failed")
	}
}
