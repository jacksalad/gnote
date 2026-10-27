package component

import (
	"strings"
	"testing"

	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/render"
)

func TestLabel_ClickableConsumesAndFires(t *testing.T) {
	l := NewLabel("状态")
	l.SetBounds(0, 0, 100, 20)
	calls := 0
	l.SetOnClick(func() { calls++ })

	if got := l.GetCursor(); got != 32649 {
		t.Fatalf("clickable label cursor = %d, want hand", got)
	}
	if !l.OnEvent(event.NewEvent(event.EventMouseDown, event.MouseData{
		X: 20, Y: 10, Button: event.MouseButtonLeft,
	})) {
		t.Fatalf("click inside clickable label should be consumed")
	}
	if calls != 1 {
		t.Fatalf("click callback fired %d times, want 1", calls)
	}
	if l.OnEvent(event.NewEvent(event.EventMouseDown, event.MouseData{
		X: 200, Y: 10, Button: event.MouseButtonLeft,
	})) {
		t.Fatalf("click outside label should not be consumed")
	}
}

func TestLabel_EllipsisTruncates(t *testing.T) {
	l := NewLabel("C:\\very\\long\\path\\to\\a\\document\\with\\a\\long\\name.txt")
	l.SetFont(DefaultFont)
	l.SetBounds(0, 0, 120, 20)
	l.SetEllipsis(true)

	canvas := render.NewCanvas(200, 30)
	got := l.ellipsizedText(canvas, l.text, 112)
	if got == l.text {
		t.Fatalf("long text was not truncated")
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("truncated text %q must end with an ellipsis", got)
	}
	if width, _ := canvas.MeasureText(got, l.font); width > 112 {
		t.Fatalf("truncated text width %d exceeds limit 112", width)
	}
}
