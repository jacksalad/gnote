package syntax

import (
	"fmt"
	"testing"

	"github.com/jacksalad/gnote/theme"
)

func TestHighlighters_ApplyEveryBuiltInTheme(t *testing.T) {
	for _, th := range theme.All() {
		for _, h := range allHighlighters() {
			name := fmt.Sprintf("%T", h)
			h.ApplyTheme(th)
			spans, state := h.HighlightLine(0, []rune("func main() { true }"), 0)
			if state < 0 {
				t.Fatalf("%s: negative end state", name)
			}
			for _, span := range spans {
				if span.Color.A() == 0 {
					t.Fatalf("%s: transparent span under theme %q", name, th.ID)
				}
			}
		}
	}
}

func TestHighlighters_FollowThemeIdentity(t *testing.T) {
	for _, th := range theme.All() {
		h := NewGoHighlighter()
		h.ApplyTheme(th)
		if h.isDark != th.Dark {
			t.Fatalf("theme %q: isDark=%v want %v", th.ID, h.isDark, th.Dark)
		}
		if th.Dark && h.colors == DefaultLightGoColors {
			t.Fatalf("theme %q: dark theme picked the light palette", th.ID)
		}
		if !th.Dark && h.colors == DefaultDarkGoColors {
			t.Fatalf("theme %q: light theme picked the dark palette", th.ID)
		}
	}
}

func TestHighlighters_ThemePalettesAreDistinct(t *testing.T) {
	seen := make(map[theme.SyntaxPalette]string)
	for _, th := range theme.All() {
		s := th.SyntaxColors()
		if prev, ok := seen[s]; ok {
			t.Fatalf("themes %q and %q share the same syntax palette", prev, th.ID)
		}
		seen[s] = th.ID
	}
}
