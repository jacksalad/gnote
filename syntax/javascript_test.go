package syntax_test

import (
	"testing"

	"github.com/jacksalad/gnote/syntax"
	"github.com/jacksalad/gnote/theme"
)

func TestJSHighlighter_KeywordsAndBuiltins(t *testing.T) {
	h := syntax.NewJSHighlighter()
	colors := h.GetColors()

	line := []rune(`async function fetchData(id) { const res = await api(id); console.log(res); return null; }`)
	spans, state := h.HighlightLine(0, line, syntax.StateJSNormal)
	if state != syntax.StateJSNormal {
		t.Fatalf("expected StateJSNormal, got %d", state)
	}

	foundAsync := false
	foundFuncKw := false
	foundFetchData := false
	foundConst := false
	foundAwait := false
	foundConsole := false
	foundReturn := false
	foundNull := false

	for _, s := range spans {
		str := string(line[s.StartCol:s.EndCol])
		if str == "async" && s.Color == colors.Keyword {
			foundAsync = true
		}
		if str == "function" && s.Color == colors.Keyword {
			foundFuncKw = true
		}
		if str == "fetchData" && s.Color == colors.Function {
			foundFetchData = true
		}
		if str == "const" && s.Color == colors.Keyword {
			foundConst = true
		}
		if str == "await" && s.Color == colors.Keyword {
			foundAwait = true
		}
		if str == "console" && s.Color == colors.Builtin {
			foundConsole = true
		}
		if str == "return" && s.Color == colors.Keyword {
			foundReturn = true
		}
		if str == "null" && s.Color == colors.Constant {
			foundNull = true
		}
	}

	if !foundAsync || !foundFuncKw || !foundFetchData || !foundConst || !foundAwait || !foundConsole || !foundReturn || !foundNull {
		t.Fatalf("tokens missing: async=%v, func=%v, name=%v, const=%v, await=%v, console=%v, return=%v, null=%v",
			foundAsync, foundFuncKw, foundFetchData, foundConst, foundAwait, foundConsole, foundReturn, foundNull)
	}
}

func TestJSHighlighter_TemplateStringMultiLine(t *testing.T) {
	h := syntax.NewJSHighlighter()
	colors := h.GetColors()

	// 行 1: 模板字符串开启
	line1 := []rune("const sql = `SELECT * FROM users")
	spans1, state1 := h.HighlightLine(0, line1, syntax.StateJSNormal)
	if state1 != syntax.StateJSTemplateString {
		t.Fatalf("expected StateJSTemplateString on line 1, got %d", state1)
	}
	if len(spans1) == 0 {
		t.Fatalf("expected spans on line 1")
	}

	// 行 2: 模板字符串内部
	line2 := []rune("WHERE age > 18 ORDER BY id DESC")
	spans2, state2 := h.HighlightLine(1, line2, state1)
	if state2 != syntax.StateJSTemplateString {
		t.Fatalf("expected StateJSTemplateString on line 2, got %d", state2)
	}
	if len(spans2) != 1 || spans2[0].Color != colors.String {
		t.Fatalf("expected string color on line 2")
	}

	// 行 3: 模板字符串闭合，后跟操作符代码
	line3 := []rune("`; const isOk = (val === 42) => true;")
	spans3, state3 := h.HighlightLine(2, line3, state2)
	if state3 != syntax.StateJSNormal {
		t.Fatalf("expected StateJSNormal on line 3, got %d", state3)
	}

	foundTemplateEnd := false
	foundStrictEq := false
	foundArrow := false
	for _, s := range spans3 {
		str := string(line3[s.StartCol:s.EndCol])
		if str == "`" && s.Color == colors.String {
			foundTemplateEnd = true
		}
		if str == "===" && s.Color == colors.Operator {
			foundStrictEq = true
		}
		if str == "=>" && s.Color == colors.Operator {
			foundArrow = true
		}
	}
	if !foundTemplateEnd || !foundStrictEq || !foundArrow {
		t.Fatalf("tokens missing: end=%v, ====%v, =>=%v", foundTemplateEnd, foundStrictEq, foundArrow)
	}
}

func TestJSHighlighter_NumbersAndTheme(t *testing.T) {
	h := syntax.NewJSHighlighter()
	colors := h.GetColors()

	line := []rune(`let val = 100n + 0xFF + 3.14;`)
	spans, _ := h.HighlightLine(0, line, syntax.StateJSNormal)

	foundBigInt := false
	foundHex := false
	foundFloat := false

	for _, s := range spans {
		str := string(line[s.StartCol:s.EndCol])
		if str == "100n" && s.Color == colors.Number {
			foundBigInt = true
		}
		if str == "0xFF" && s.Color == colors.Number {
			foundHex = true
		}
		if str == "3.14" && s.Color == colors.Number {
			foundFloat = true
		}
	}

	if !foundBigInt || !foundHex || !foundFloat {
		t.Fatalf("numbers missing: bigint=%v, hex=%v, float=%v", foundBigInt, foundHex, foundFloat)
	}

	h.ApplyTheme(theme.Light())
	colorsLight := h.GetColors()
	if colorsLight.Keyword != syntax.DefaultLightJSColors.Keyword {
		t.Errorf("expected light keyword color")
	}
}
