package syntax_test

import (
	"testing"

	"github.com/jacksalad/gnote/syntax"
	"github.com/jacksalad/gnote/theme"
)

func TestCSSHighlighter_BasicRulesAndProperties(t *testing.T) {
	h := syntax.NewCSSHighlighter()
	colors := h.GetColors()

	line := []rune(`.container { display: flex; margin: 10px; color: #ffffff !important; }`)
	spans, state := h.HighlightLine(0, line, syntax.StateCSSNormal)
	if state != syntax.StateCSSNormal {
		t.Fatalf("expected StateCSSNormal, got %d", state)
	}

	foundSelector := false
	foundDisplay := false
	foundFlex := false
	foundMarginNum := false
	foundHex := false
	foundImportant := false

	for _, s := range spans {
		str := string(line[s.StartCol:s.EndCol])
		if str == ".container" && s.Color == colors.Selector {
			foundSelector = true
		}
		if str == "display" && s.Color == colors.Property {
			foundDisplay = true
		}
		if str == "flex" && s.Color == colors.Value {
			foundFlex = true
		}
		if str == "10px" && s.Color == colors.Number {
			foundMarginNum = true
		}
		if str == "#ffffff" && s.Color == colors.Number {
			foundHex = true
		}
		if str == "!important" && s.Color == colors.Important {
			foundImportant = true
		}
	}

	if !foundSelector || !foundDisplay || !foundFlex || !foundMarginNum || !foundHex || !foundImportant {
		t.Fatalf("tokens missing: sel=%v, prop=%v, val=%v, num=%v, hex=%v, imp=%v",
			foundSelector, foundDisplay, foundFlex, foundMarginNum, foundHex, foundImportant)
	}
}

func TestCSSHighlighter_MultiLineRuleBlockAndComments(t *testing.T) {
	h := syntax.NewCSSHighlighter()
	colors := h.GetColors()

	// 行 1: 规则块开启
	line1 := []rune(`.btn:hover {`)
	spans1, state1 := h.HighlightLine(0, line1, syntax.StateCSSNormal)
	if state1 != syntax.StateCSSRuleBlock {
		t.Fatalf("expected StateCSSRuleBlock on line 1, got %d", state1)
	}
	if len(spans1) == 0 {
		t.Fatalf("expected spans on line 1")
	}

	// 行 2: 规则块内声明
	line2 := []rune(`    background-color: red;`)
	spans2, state2 := h.HighlightLine(1, line2, state1)
	if state2 != syntax.StateCSSRuleBlock {
		t.Fatalf("expected StateCSSRuleBlock on line 2, got %d", state2)
	}

	foundProp := false
	foundVal := false
	for _, s := range spans2 {
		str := string(line2[s.StartCol:s.EndCol])
		if str == "background-color" && s.Color == colors.Property {
			foundProp = true
		}
		if str == "red" && s.Color == colors.Value {
			foundVal = true
		}
	}
	if !foundProp || !foundVal {
		t.Fatalf("tokens missing on line 2: prop=%v, val=%v", foundProp, foundVal)
	}

	// 行 3: 规则块闭合
	line3 := []rune(`}`)
	spans3, state3 := h.HighlightLine(2, line3, state2)
	if state3 != syntax.StateCSSNormal {
		t.Fatalf("expected StateCSSNormal on line 3, got %d", state3)
	}
	if len(spans3) != 1 || spans3[0].Color != colors.Punctuation {
		t.Fatalf("expected punctuation on line 3")
	}
}

func TestCSSHighlighter_AtRulesAndTheme(t *testing.T) {
	h := syntax.NewCSSHighlighter()
	colors := h.GetColors()

	line := []rune(`@media screen and (max-width: 768px) {`)
	spans, _ := h.HighlightLine(0, line, syntax.StateCSSNormal)

	foundAtRule := false
	for _, s := range spans {
		str := string(line[s.StartCol:s.EndCol])
		if str == "@media" && s.Color == colors.AtRule {
			foundAtRule = true
		}
	}
	if !foundAtRule {
		t.Fatalf("expected @media token")
	}

	h.ApplyTheme(theme.Light())
	colorsLight := h.GetColors()
	if colorsLight.Property != syntax.DefaultLightCSSColors.Property {
		t.Errorf("expected light property color")
	}
}
