package syntax_test

import (
	"testing"

	"github.com/jacksalad/gnote/syntax"
	"github.com/jacksalad/gnote/theme"
)

func TestRustHighlighter_KeywordsAndMacros(t *testing.T) {
	h := syntax.NewRustHighlighter()
	colors := h.GetColors()

	line := []rune(`pub async fn get_data<'a>(id: i32) -> Option<String> { println!("id: {}", id); }`)
	spans, state := h.HighlightLine(0, line, syntax.StateRustNormal)
	if state != syntax.StateRustNormal {
		t.Fatalf("expected StateRustNormal, got %d", state)
	}

	foundPub := false
	foundAsync := false
	foundFn := false
	foundGetData := false
	foundLifetime := false
	foundI32 := false
	foundArrow := false
	foundOption := false
	foundPrintln := false

	for _, s := range spans {
		str := string(line[s.StartCol:s.EndCol])
		if str == "pub" && s.Color == colors.Keyword {
			foundPub = true
		}
		if str == "async" && s.Color == colors.Keyword {
			foundAsync = true
		}
		if str == "fn" && s.Color == colors.Keyword {
			foundFn = true
		}
		if str == "get_data" && s.Color == colors.Function {
			foundGetData = true
		}
		if str == "'a" && s.Color == colors.Lifetime {
			foundLifetime = true
		}
		if str == "i32" && s.Color == colors.Type {
			foundI32 = true
		}
		if str == "->" && s.Color == colors.Operator {
			foundArrow = true
		}
		if str == "Option" && s.Color == colors.Type {
			foundOption = true
		}
		if str == "println!" && s.Color == colors.Macro {
			foundPrintln = true
		}
	}

	if !foundPub || !foundAsync || !foundFn || !foundGetData || !foundLifetime || !foundI32 || !foundArrow || !foundOption || !foundPrintln {
		t.Fatalf("tokens missing: pub=%v, async=%v, fn=%v, name=%v, life=%v, i32=%v, arrow=%v, opt=%v, print=%v",
			foundPub, foundAsync, foundFn, foundGetData, foundLifetime, foundI32, foundArrow, foundOption, foundPrintln)
	}
}

func TestRustHighlighter_RawStringsAndAttributes(t *testing.T) {
	h := syntax.NewRustHighlighter()
	colors := h.GetColors()

	// 行 1: 属性注解与原生字符串开启
	line1 := []rune(`#[derive(Debug)] const JSON: &str = r#"{"name": "Rust",`)
	spans1, state1 := h.HighlightLine(0, line1, syntax.StateRustNormal)
	if state1 != syntax.StateRustRawString {
		t.Fatalf("expected StateRustRawString on line 1, got %d", state1)
	}

	foundAttr := false
	for _, s := range spans1 {
		str := string(line1[s.StartCol:s.EndCol])
		if str == "#[derive(Debug)]" && s.Color == colors.Macro {
			foundAttr = true
		}
	}
	if !foundAttr {
		t.Fatalf("expected attribute macro token")
	}

	// 行 2: 原生字符串内部
	line2 := []rune(`   "fast": true}`)
	spans2, state2 := h.HighlightLine(1, line2, state1)
	if state2 != syntax.StateRustRawString {
		t.Fatalf("expected StateRustRawString on line 2, got %d", state2)
	}
	if len(spans2) != 1 || spans2[0].Color != colors.String {
		t.Fatalf("expected string color on line 2")
	}

	// 行 3: 原生字符串闭合
	line3 := []rune(`"#; let val = 1_000_000usize;`)
	spans3, state3 := h.HighlightLine(2, line3, state2)
	if state3 != syntax.StateRustNormal {
		t.Fatalf("expected StateRustNormal on line 3, got %d", state3)
	}

	foundCloseStr := false
	foundNumber := false
	for _, s := range spans3 {
		str := string(line3[s.StartCol:s.EndCol])
		if str == `"#` && s.Color == colors.String {
			foundCloseStr = true
		}
		if str == "1_000_000usize" && s.Color == colors.Number {
			foundNumber = true
		}
	}
	if !foundCloseStr || !foundNumber {
		t.Fatalf("tokens missing on line 3: closeStr=%v, num=%v", foundCloseStr, foundNumber)
	}
}

func TestRustHighlighter_Theme(t *testing.T) {
	h := syntax.NewRustHighlighter()
	h.ApplyTheme(theme.Light())
	colorsLight := h.GetColors()
	if colorsLight.Keyword != syntax.DefaultLightRustColors.Keyword {
		t.Errorf("expected light keyword color")
	}
}
