package component

import (
	"strings"
	"testing"

	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/graphics"
)

func TestTextBox_HorizontalScrollFollowsCaret(t *testing.T) {
	tb := NewTextBox()
	tb.SetFont(graphics.NewFont("Consolas", 14))
	tb.SetBounds(0, 0, 120, 26)
	tb.SetFocused(true)
	tb.SetText(strings.Repeat("abcdefghij", 3))

	if tb.scrollOffsetX <= 0 {
		t.Fatalf("long text should scroll to keep caret visible, offset=%d", tb.scrollOffsetX)
	}

	tb.OnEvent(event.NewEvent(event.EventKeyDown, event.KeyData{KeyCode: 0x24})) // Home
	if tb.scrollOffsetX != 0 {
		t.Fatalf("Home should reset horizontal scroll, offset=%d", tb.scrollOffsetX)
	}

	tb.OnEvent(event.NewEvent(event.EventKeyDown, event.KeyData{KeyCode: 0x23})) // End
	if tb.scrollOffsetX <= 0 {
		t.Fatalf("End should scroll to the caret, offset=%d", tb.scrollOffsetX)
	}
}
