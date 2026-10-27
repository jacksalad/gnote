package syntax_test

import (
	"testing"

	"github.com/jacksalad/gnote/syntax"
	"github.com/jacksalad/gnote/theme"
)

func TestPythonHighlighter_KeywordsAndDecorators(t *testing.T) {
	h := syntax.NewPythonHighlighter()
	colors := h.GetColors()

	line := []rune("@property\ndef get_total(self, items: list) -> int: return len(items)")
	spans, endState := h.HighlightLine(0, line, syntax.StatePyNormal)
	if endState != syntax.StatePyNormal {
		t.Fatalf("expected StatePyNormal, got %d", endState)
	}

	foundDecorator := false
	foundDef := false
	foundFunc := false
	foundSelf := false
	foundList := false
	foundReturn := false
	foundLen := false

	for _, s := range spans {
		word := string(line[s.StartCol:s.EndCol])
		if word == "@property" && s.Color == colors.Decorator {
			foundDecorator = true
		}
		if word == "def" && s.Color == colors.Keyword {
			foundDef = true
		}
		if word == "get_total" && s.Color == colors.Function {
			foundFunc = true
		}
		if word == "self" && s.Color == colors.Constant {
			foundSelf = true
		}
		if word == "list" && s.Color == colors.Type {
			foundList = true
		}
		if word == "return" && s.Color == colors.Keyword {
			foundReturn = true
		}
		if word == "len" && s.Color == colors.Builtin {
			foundLen = true
		}
	}

	if !foundDecorator || !foundDef || !foundFunc || !foundSelf || !foundList || !foundReturn || !foundLen {
		t.Fatalf("python tokens missing: dec=%v, def=%v, func=%v, self=%v, list=%v, ret=%v, len=%v",
			foundDecorator, foundDef, foundFunc, foundSelf, foundList, foundReturn, foundLen)
	}
}

func TestPythonHighlighter_MultiLineTripleQuotes(t *testing.T) {
	h := syntax.NewPythonHighlighter()
	colors := h.GetColors()

	// 行 1: 三引号文档字符串开启
	line1 := []rune(`"""这是多行文档注释`)
	spans1, state1 := h.HighlightLine(0, line1, syntax.StatePyNormal)
	if state1 != syntax.StatePyTripleDouble {
		t.Fatalf("expected StatePyTripleDouble on line 1, got %d", state1)
	}
	if len(spans1) != 1 || spans1[0].Color != colors.String {
		t.Fatalf("expected string color on line 1")
	}

	// 行 2: 多行字符串内部
	line2 := []rune(`   描述内容在这里...`)
	spans2, state2 := h.HighlightLine(1, line2, state1)
	if state2 != syntax.StatePyTripleDouble {
		t.Fatalf("expected StatePyTripleDouble on line 2, got %d", state2)
	}
	if len(spans2) != 1 || spans2[0].Color != colors.String {
		t.Fatalf("expected string color on line 2")
	}

	// 行 3: 闭合三引号，后跟代码
	line3 := []rune(`""" # 结束文档字符串`)
	spans3, state3 := h.HighlightLine(2, line3, state2)
	if state3 != syntax.StatePyNormal {
		t.Fatalf("expected StatePyNormal on line 3, got %d", state3)
	}

	foundStr := false
	foundComment := false
	for _, s := range spans3 {
		str := string(line3[s.StartCol:s.EndCol])
		if str == `"""` && s.Color == colors.String {
			foundStr = true
		}
		if str == "# 结束文档字符串" && s.Color == colors.Comment {
			foundComment = true
		}
	}

	if !foundStr || !foundComment {
		t.Fatalf("line 3 mismatch: str=%v, comment=%v", foundStr, foundComment)
	}
}

func TestPythonHighlighter_NumbersAndPrefixStrings(t *testing.T) {
	h := syntax.NewPythonHighlighter()
	colors := h.GetColors()

	line := []rune(`name = f"User_{id}: {val}"; z = 1_000 + 2.5e-3 + 3j`)
	spans, _ := h.HighlightLine(0, line, syntax.StatePyNormal)

	foundPrefixStr := false
	foundUnderlineNum := false
	foundScientificNum := false
	foundComplexNum := false

	for _, s := range spans {
		str := string(line[s.StartCol:s.EndCol])
		if str == `f"User_{id}: {val}"` && s.Color == colors.String {
			foundPrefixStr = true
		}
		if str == "1_000" && s.Color == colors.Number {
			foundUnderlineNum = true
		}
		if str == "2.5e-3" && s.Color == colors.Number {
			foundScientificNum = true
		}
		if str == "3j" && s.Color == colors.Number {
			foundComplexNum = true
		}
	}

	if !foundPrefixStr || !foundUnderlineNum || !foundScientificNum || !foundComplexNum {
		t.Fatalf("tokens missing: fstr=%v, under=%v, sci=%v, cplx=%v",
			foundPrefixStr, foundUnderlineNum, foundScientificNum, foundComplexNum)
	}
}

func TestPythonHighlighter_RegistryAndDetection(t *testing.T) {
	h := syntax.DetectHighlighter("script.py")
	if h == nil {
		t.Fatalf("expected non-nil for script.py")
	}
	if syntax.GetLanguageName(h) != "Python" {
		t.Fatalf("expected 'Python', got %q", syntax.GetLanguageName(h))
	}

	h.ApplyTheme(theme.Light())
	colorsLight := h.(*syntax.PythonHighlighter).GetColors()
	if colorsLight.Keyword != syntax.DefaultLightPythonColors.Keyword {
		t.Errorf("expected light keyword color")
	}
}
