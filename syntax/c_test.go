package syntax_test

import (
	"testing"

	"github.com/jacksalad/gnote/syntax"
	"github.com/jacksalad/gnote/theme"
)

func TestCHighlighter_BasicTokens(t *testing.T) {
	h := syntax.NewCHighlighter()
	colors := h.GetColors()

	line := []rune(`#include <iostream>`)
	spans, state := h.HighlightLine(0, line, syntax.StateCNormal)
	if state != syntax.StateCNormal {
		t.Fatalf("expected StateCNormal, got %d", state)
	}

	foundPreprocessor := false
	foundIncludePath := false
	for _, s := range spans {
		str := string(line[s.StartCol:s.EndCol])
		if str == "#include" && s.Color == colors.Preprocessor {
			foundPreprocessor = true
		}
		if str == "<iostream>" && s.Color == colors.String {
			foundIncludePath = true
		}
	}
	if !foundPreprocessor || !foundIncludePath {
		t.Fatalf("include tokens missing: pp=%v, path=%v", foundPreprocessor, foundIncludePath)
	}

	// 测试类、关键字、类型、操作符
	codeLine := []rune(`class Widget : public Base { int count = 42; auto ptr = nullptr; };`)
	spans2, _ := h.HighlightLine(1, codeLine, syntax.StateCNormal)

	foundClass := false
	foundPublic := false
	foundInt := false
	foundNumber := false
	foundAuto := false
	foundNullptr := false

	for _, s := range spans2 {
		str := string(codeLine[s.StartCol:s.EndCol])
		if str == "class" && s.Color == colors.Keyword {
			foundClass = true
		}
		if str == "public" && s.Color == colors.Keyword {
			foundPublic = true
		}
		if str == "int" && s.Color == colors.Type {
			foundInt = true
		}
		if str == "42" && s.Color == colors.Number {
			foundNumber = true
		}
		if str == "auto" && s.Color == colors.Keyword {
			foundAuto = true
		}
		if str == "nullptr" && s.Color == colors.Constant {
			foundNullptr = true
		}
	}

	if !foundClass || !foundPublic || !foundInt || !foundNumber || !foundAuto || !foundNullptr {
		t.Fatalf("tokens missing: class=%v, public=%v, int=%v, 42=%v, auto=%v, nullptr=%v",
			foundClass, foundPublic, foundInt, foundNumber, foundAuto, foundNullptr)
	}
}

func TestCHighlighter_MultiLineBlockComment(t *testing.T) {
	h := syntax.NewCHighlighter()
	colors := h.GetColors()

	// 行 1: 块注释开启
	line1 := []rune(`int a = 1; /* 跨行注释开启`)
	spans1, state1 := h.HighlightLine(0, line1, syntax.StateCNormal)
	if state1 != syntax.StateCBlockComment {
		t.Fatalf("expected StateCBlockComment on line 1, got %d", state1)
	}
	if len(spans1) == 0 {
		t.Fatalf("expected spans on line 1")
	}

	// 行 2: 注释正文
	line2 := []rune(`   跨行注释第二行...`)
	spans2, state2 := h.HighlightLine(1, line2, state1)
	if state2 != syntax.StateCBlockComment {
		t.Fatalf("expected StateCBlockComment on line 2, got %d", state2)
	}
	if len(spans2) != 1 || spans2[0].Color != colors.Comment {
		t.Fatalf("expected comment color on line 2")
	}

	// 行 3: 块注释闭合，后续代码
	line3 := []rune(`   */ return a;`)
	spans3, state3 := h.HighlightLine(2, line3, state2)
	if state3 != syntax.StateCNormal {
		t.Fatalf("expected StateCNormal on line 3, got %d", state3)
	}

	foundComment := false
	foundReturn := false
	for _, s := range spans3 {
		str := string(line3[s.StartCol:s.EndCol])
		if str == "   */" && s.Color == colors.Comment {
			foundComment = true
		}
		if str == "return" && s.Color == colors.Keyword {
			foundReturn = true
		}
	}
	if !foundComment || !foundReturn {
		t.Fatalf("tokens missing: comment=%v, return=%v", foundComment, foundReturn)
	}
}

func TestCHighlighter_OperatorsAndHex(t *testing.T) {
	h := syntax.NewCHighlighter()
	colors := h.GetColors()

	line := []rune(`ptr->render(); val = 0xFF + 3.14f; std::cout << val;`)
	spans, _ := h.HighlightLine(0, line, syntax.StateCNormal)

	foundArrow := false
	foundColons := false
	foundShiftLeft := false
	foundHex := false
	foundFloat := false
	foundFunc := false

	for _, s := range spans {
		str := string(line[s.StartCol:s.EndCol])
		if str == "->" && s.Color == colors.Operator {
			foundArrow = true
		}
		if str == "::" && s.Color == colors.Operator {
			foundColons = true
		}
		if str == "<<" && s.Color == colors.Operator {
			foundShiftLeft = true
		}
		if str == "0xFF" && s.Color == colors.Number {
			foundHex = true
		}
		if str == "3.14f" && s.Color == colors.Number {
			foundFloat = true
		}
		if str == "render" && s.Color == colors.Function {
			foundFunc = true
		}
	}

	if !foundArrow || !foundColons || !foundShiftLeft || !foundHex || !foundFloat || !foundFunc {
		t.Fatalf("tokens missing: ->=%v, ::=%v, <<=%v, hex=%v, float=%v, func=%v",
			foundArrow, foundColons, foundShiftLeft, foundHex, foundFloat, foundFunc)
	}
}

func TestCHighlighter_Theme(t *testing.T) {
	h := syntax.NewCHighlighter()
	h.ApplyTheme(theme.Light())
	colorsLight := h.GetColors()
	if colorsLight.Keyword != syntax.DefaultLightCColors.Keyword {
		t.Errorf("expected light keyword color")
	}
}
