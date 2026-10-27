package syntax_test

import (
	"testing"

	"github.com/jacksalad/gnote/syntax"
	"github.com/jacksalad/gnote/theme"
)

func TestCSharpHighlighter_KeywordsAndTypes(t *testing.T) {
	h := syntax.NewCSharpHighlighter()
	colors := h.GetColors()

	line := []rune(`public async Task<string> FetchDataAsync(int id) => await Task.FromResult("done");`)
	spans, state := h.HighlightLine(0, line, syntax.StateCSNormal)
	if state != syntax.StateCSNormal {
		t.Fatalf("expected StateCSNormal, got %d", state)
	}

	foundPublic := false
	foundAsync := false
	foundTask := false
	foundString := false
	foundInt := false
	foundAwait := false
	foundArrow := false

	for _, s := range spans {
		str := string(line[s.StartCol:s.EndCol])
		if str == "public" && s.Color == colors.Keyword {
			foundPublic = true
		}
		if str == "async" && s.Color == colors.Keyword {
			foundAsync = true
		}
		if str == "Task" && s.Color == colors.Type {
			foundTask = true
		}
		if str == "string" && s.Color == colors.Type {
			foundString = true
		}
		if str == "int" && s.Color == colors.Type {
			foundInt = true
		}
		if str == "await" && s.Color == colors.Keyword {
			foundAwait = true
		}
		if str == "=>" && s.Color == colors.Operator {
			foundArrow = true
		}
	}

	if !foundPublic || !foundAsync || !foundTask || !foundString || !foundInt || !foundAwait || !foundArrow {
		t.Fatalf("tokens missing: pub=%v, async=%v, task=%v, str=%v, int=%v, await=%v, =>=%v",
			foundPublic, foundAsync, foundTask, foundString, foundInt, foundAwait, foundArrow)
	}
}

func TestCSharpHighlighter_VerbatimStringMultiLine(t *testing.T) {
	h := syntax.NewCSharpHighlighter()
	colors := h.GetColors()

	// 行 1: @" 开启逐字多行字符串
	line1 := []rune(`string query = @"SELECT * FROM Users`)
	spans1, state1 := h.HighlightLine(0, line1, syntax.StateCSNormal)
	if state1 != syntax.StateCSVerbatimString {
		t.Fatalf("expected StateCSVerbatimString on line 1, got %d", state1)
	}
	if len(spans1) == 0 {
		t.Fatalf("expected spans on line 1")
	}

	// 行 2: 逐字字符串内部
	line2 := []rune(`    WHERE Age > 18;`)
	spans2, state2 := h.HighlightLine(1, line2, state1)
	if state2 != syntax.StateCSVerbatimString {
		t.Fatalf("expected StateCSVerbatimString on line 2, got %d", state2)
	}
	if len(spans2) != 1 || spans2[0].Color != colors.String {
		t.Fatalf("expected string color on line 2")
	}

	// 行 3: 逐字字符串闭合
	line3 := []rune(`"; decimal balance = 1_000_000m;`)
	spans3, state3 := h.HighlightLine(2, line3, state2)
	if state3 != syntax.StateCSNormal {
		t.Fatalf("expected StateCSNormal on line 3, got %d", state3)
	}

	foundDecimal := false
	foundNumber := false
	for _, s := range spans3 {
		str := string(line3[s.StartCol:s.EndCol])
		if str == "decimal" && s.Color == colors.Type {
			foundDecimal = true
		}
		if str == "1_000_000m" && s.Color == colors.Number {
			foundNumber = true
		}
	}
	if !foundDecimal || !foundNumber {
		t.Fatalf("tokens missing: dec=%v, num=%v", foundDecimal, foundNumber)
	}
}

func TestCSharpHighlighter_PreprocessorAndTheme(t *testing.T) {
	h := syntax.NewCSharpHighlighter()
	colors := h.GetColors()

	line := []rune(`#region DataContract`)
	spans, _ := h.HighlightLine(0, line, syntax.StateCSNormal)

	foundPre := false
	for _, s := range spans {
		str := string(line[s.StartCol:s.EndCol])
		if str == "#region" && s.Color == colors.Preprocessor {
			foundPre = true
		}
	}
	if !foundPre {
		t.Fatalf("expected #region token")
	}

	h.ApplyTheme(theme.Light())
	colorsLight := h.GetColors()
	if colorsLight.Keyword != syntax.DefaultLightCSharpColors.Keyword {
		t.Errorf("expected light keyword color")
	}
}
