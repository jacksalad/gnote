package theme

import (
	"math"
	"testing"

	"github.com/jacksalad/gnote/graphics"
)

func relativeLuminance(c graphics.Color) float64 {
	channel := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(c.R()) + 0.7152*channel(c.G()) + 0.0722*channel(c.B())
}

func contrastRatio(a, b graphics.Color) float64 {
	l1, l2 := relativeLuminance(a), relativeLuminance(b)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func TestThemes_RegistryAndIdentity(t *testing.T) {
	themes := All()
	if len(themes) < 8 {
		t.Fatalf("expected at least 8 built-in themes, got %d", len(themes))
	}
	seenIDs := map[string]bool{}
	seenLabels := map[string]bool{}
	for _, th := range themes {
		if th.ID == "" || th.Name == "" || th.Label == "" {
			t.Fatalf("theme identity incomplete: %+v", th)
		}
		if seenIDs[th.ID] {
			t.Fatalf("duplicate theme ID %q", th.ID)
		}
		if seenLabels[th.Label] {
			t.Fatalf("duplicate theme label %q", th.Label)
		}
		seenIDs[th.ID] = true
		seenLabels[th.Label] = true
		if got := ByID(th.ID); got == nil || got.ID != th.ID {
			t.Fatalf("ByID(%q) did not resolve", th.ID)
		}
	}
	if ByID("  LIGHT ") == nil || ByID("light").ID != "light" {
		t.Fatalf("ByID must trim and normalize case")
	}
	if ByID("does-not-exist") != nil {
		t.Fatalf("unknown theme ID must return nil")
	}
	if Dark().Name != "Dark" || Light().Name != "Light" || Dark().ID != "dark" || Light().ID != "light" {
		t.Fatalf("legacy Dark/Light identity must be preserved")
	}
}

func TestThemes_DarkFlagMatchesBackground(t *testing.T) {
	for _, th := range All() {
		lum := relativeLuminance(th.Background)
		if th.Dark != (lum < 0.5) {
			t.Fatalf("theme %q Dark=%v but background luminance %.2f", th.ID, th.Dark, lum)
		}
	}
}

func TestThemes_UIContrast(t *testing.T) {
	for _, th := range All() {
		checks := []struct {
			name string
			fg   graphics.Color
			bg   graphics.Color
			min  float64
		}{
			{"text/background", th.TextPrimary, th.Background, 4.5},
			{"text/surface", th.TextPrimary, th.Surface, 5.0},
			{"secondary/surface", th.TextSecondary, th.Surface, 3.2},
			{"onPrimary/primary", th.OnPrimary, th.Primary, 3.5},
			{"gutter/gutterBg", th.GutterText, th.GutterBg, 3.0},
		}
		for _, c := range checks {
			if got := contrastRatio(c.fg, c.bg); got < c.min {
				t.Errorf("theme %q %s contrast %.2f < %.2f", th.ID, c.name, got, c.min)
			}
		}
	}
}

func TestThemes_SyntaxTokensResolvedAndReadable(t *testing.T) {
	for _, th := range All() {
		s := th.SyntaxColors()
		core := map[string]graphics.Color{
			"Keyword": s.Keyword, "Type": s.Type, "Builtin": s.Builtin,
			"Function": s.Function, "String": s.String, "Number": s.Number,
			"Comment": s.Comment, "Constant": s.Constant, "Operator": s.Operator,
			"Punctuation": s.Punctuation, "Meta": s.Meta,
			"Heading1": s.Heading1, "CodeBlockText": s.CodeBlockText,
			"LinkText": s.LinkText, "TaskChecked": s.TaskChecked,
		}
		for name, c := range core {
			if c.A() == 0 {
				t.Errorf("theme %q syntax token %s is unresolved", th.ID, name)
				continue
			}
			if name == "Operator" || name == "Punctuation" {
				continue
			}
			if got := contrastRatio(c, th.Surface); got < 2.5 {
				t.Errorf("theme %q syntax %s contrast %.2f < 2.5", th.ID, name, got)
			}
		}
		if s.CodeBlockBg.A() == 0 || s.InlineCodeBg.A() == 0 {
			t.Errorf("theme %q markdown code backgrounds must be visible", th.ID)
		}
	}
}

// 注释、引用、链接、行内代码是正文可读性的关键角色，必须达到 WCAG AA 4.5:1；
// 分隔线与行号属于非文本装饰，按 WCAG 非文本对比度 3:1。
func TestThemes_ReadabilityCriticalRoles(t *testing.T) {
	for _, th := range All() {
		s := th.SyntaxColors()
		textChecks := []struct {
			name string
			c    graphics.Color
		}{
			{"Comment", s.Comment},
			{"CommentSoft", s.CommentSoft},
			{"QuoteText", s.QuoteText},
			{"LinkUrl", s.LinkUrl},
			{"StrikeText", s.StrikeText},
			{"InlineCodeText", s.InlineCodeText},
		}
		for _, c := range textChecks {
			if got := contrastRatio(c.c, th.Surface); got < 4.5 {
				t.Errorf("theme %q %s contrast %.2f < 4.5 (WCAG AA)", th.ID, c.name, got)
			}
		}
		if got := contrastRatio(s.Hr, th.Surface); got < 3.0 {
			t.Errorf("theme %q Hr contrast %.2f < 3.0", th.ID, got)
		}
	}
}

func TestThemes_SyntaxStyleIsConsistentAcrossThemes(t *testing.T) {
	for _, th := range All() {
		s := th.SyntaxColors()
		if s.String == s.Keyword {
			t.Errorf("theme %q strings and keywords share a hue", th.ID)
		}
		if s.Comment == s.String {
			t.Errorf("theme %q comments must be visually distinct from strings", th.ID)
		}
		if s.Type == s.Number {
			t.Errorf("theme %q types and numbers share a hue", th.ID)
		}
	}
}

// 内置调色板只应构建一次；菜单/设置页重复取列表不能反复分配整套 Theme。
func TestThemes_AllIsCached(t *testing.T) {
	a := All()
	b := All()
	if len(a) != len(b) {
		t.Fatalf("All length changed: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("theme %d rebuilt on second All() call", i)
		}
	}
}
