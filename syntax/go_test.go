package syntax_test

import (
	"testing"

	"github.com/jacksalad/gnote/syntax"
	"github.com/jacksalad/gnote/theme"
)

func TestGoHighlighter_KeywordsAndTypes(t *testing.T) {
	h := syntax.NewGoHighlighter()
	colors := h.GetColors()

	line := []rune("package main; var count int = 100")
	spans, endState := h.HighlightLine(0, line, syntax.StateGoNormal)
	if endState != syntax.StateGoNormal {
		t.Fatalf("expected StateGoNormal, got %d", endState)
	}

	foundPkg := false
	foundVar := false
	foundInt := false
	foundNum := false

	for _, s := range spans {
		word := string(line[s.StartCol:s.EndCol])
		if word == "package" && s.Color == colors.Keyword {
			foundPkg = true
		}
		if word == "var" && s.Color == colors.Keyword {
			foundVar = true
		}
		if word == "int" && s.Color == colors.Type {
			foundInt = true
		}
		if word == "100" && s.Color == colors.Number {
			foundNum = true
		}
	}

	if !foundPkg || !foundVar || !foundInt || !foundNum {
		t.Fatalf("failed to highlight basic Go line: pkg=%v, var=%v, int=%v, num=%v",
			foundPkg, foundVar, foundInt, foundNum)
	}
}

func TestGoHighlighter_FunctionsAndCalls(t *testing.T) {
	h := syntax.NewGoHighlighter()
	colors := h.GetColors()

	line := []rune("func ProcessData(items []string) { fmt.Println(len(items)) }")
	spans, _ := h.HighlightLine(0, line, syntax.StateGoNormal)

	foundFuncDecl := false
	foundPrintln := false
	foundLen := false

	for _, s := range spans {
		word := string(line[s.StartCol:s.EndCol])
		if word == "ProcessData" && s.Color == colors.Function {
			foundFuncDecl = true
		}
		if word == "Println" && s.Color == colors.Function {
			foundPrintln = true
		}
		if word == "len" && s.Color == colors.Builtin {
			foundLen = true
		}
	}

	if !foundFuncDecl {
		t.Errorf("expected ProcessData to be highlighted as function")
	}
	if !foundPrintln {
		t.Errorf("expected Println to be highlighted as function call")
	}
	if !foundLen {
		t.Errorf("expected len to be highlighted as builtin")
	}
}

func TestGoHighlighter_MultiLineBlockComments(t *testing.T) {
	h := syntax.NewGoHighlighter()
	colors := h.GetColors()

	// 行 1: 块注释开启
	line1 := []rune("/* 开始多行注释")
	spans1, state1 := h.HighlightLine(0, line1, syntax.StateGoNormal)
	if state1 != syntax.StateGoBlockComment {
		t.Fatalf("expected StateGoBlockComment on line 1, got %d", state1)
	}
	if len(spans1) != 1 || spans1[0].Color != colors.Comment {
		t.Fatalf("expected comment color on line 1")
	}

	// 行 2: 注释内部
	line2 := []rune("   注释正文内容")
	spans2, state2 := h.HighlightLine(1, line2, state1)
	if state2 != syntax.StateGoBlockComment {
		t.Fatalf("expected StateGoBlockComment on line 2, got %d", state2)
	}
	if len(spans2) != 1 || spans2[0].Color != colors.Comment {
		t.Fatalf("expected comment color on line 2")
	}

	// 行 3: 注释结束，后跟正常代码
	line3 := []rune("   结束注释 */ val := 42")
	spans3, state3 := h.HighlightLine(2, line3, state2)
	if state3 != syntax.StateGoNormal {
		t.Fatalf("expected StateGoNormal on line 3, got %d", state3)
	}

	foundComment := false
	foundOp := false
	foundNum := false
	for _, s := range spans3 {
		str := string(line3[s.StartCol:s.EndCol])
		if str == "   结束注释 */" && s.Color == colors.Comment {
			foundComment = true
		}
		if str == ":=" && s.Color == colors.Operator {
			foundOp = true
		}
		if str == "42" && s.Color == colors.Number {
			foundNum = true
		}
	}

	if !foundComment || !foundOp || !foundNum {
		t.Fatalf("line 3 mismatch: comment=%v, op=%v, num=%v", foundComment, foundOp, foundNum)
	}
}

func TestGoHighlighter_MultiLineRawStrings(t *testing.T) {
	h := syntax.NewGoHighlighter()
	colors := h.GetColors()

	// 行 1: 原生反引号字符串开启
	line1 := []rune("query := `SELECT id, name")
	spans1, state1 := h.HighlightLine(0, line1, syntax.StateGoNormal)
	if state1 != syntax.StateGoRawString {
		t.Fatalf("expected StateGoRawString on line 1, got %d", state1)
	}
	if len(spans1) == 0 {
		t.Fatalf("expected non-empty spans on line 1")
	}

	// 行 2: 字符串多行中间
	line2 := []rune("FROM users")
	spans2, state2 := h.HighlightLine(1, line2, state1)
	if state2 != syntax.StateGoRawString {
		t.Fatalf("expected StateGoRawString on line 2, got %d", state2)
	}
	if len(spans2) != 1 || spans2[0].Color != colors.String {
		t.Fatalf("expected string color on line 2")
	}

	// 行 3: 闭合反引号，后跟普通注释
	line3 := []rune("WHERE active = true` // 过滤活跃用户")
	spans3, state3 := h.HighlightLine(2, line3, state2)
	if state3 != syntax.StateGoNormal {
		t.Fatalf("expected StateGoNormal on line 3, got %d", state3)
	}

	foundStr := false
	foundComment := false
	for _, s := range spans3 {
		str := string(line3[s.StartCol:s.EndCol])
		if str == "WHERE active = true`" && s.Color == colors.String {
			foundStr = true
		}
		if str == "// 过滤活跃用户" && s.Color == colors.Comment {
			foundComment = true
		}
	}

	if !foundStr || !foundComment {
		t.Fatalf("line 3 raw string mismatch: str=%v, comment=%v", foundStr, foundComment)
	}
}

func TestGoHighlighter_NumbersAndOperators(t *testing.T) {
	h := syntax.NewGoHighlighter()
	colors := h.GetColors()

	line := []rune("val := 0x1F + 3.1415 + 1_000_000 + 2i; ch <- val")
	spans, _ := h.HighlightLine(0, line, syntax.StateGoNormal)

	foundColonAssign := false
	foundArrow := false
	foundHex := false
	foundFloat := false
	foundUnderline := false
	foundImag := false

	for _, s := range spans {
		str := string(line[s.StartCol:s.EndCol])
		if str == ":=" && s.Color == colors.Operator {
			foundColonAssign = true
		}
		if str == "<-" && s.Color == colors.Operator {
			foundArrow = true
		}
		if str == "0x1F" && s.Color == colors.Number {
			foundHex = true
		}
		if str == "3.1415" && s.Color == colors.Number {
			foundFloat = true
		}
		if str == "1_000_000" && s.Color == colors.Number {
			foundUnderline = true
		}
		if str == "2i" && s.Color == colors.Number {
			foundImag = true
		}
	}

	if !foundColonAssign || !foundArrow || !foundHex || !foundFloat || !foundUnderline || !foundImag {
		t.Fatalf("number/op test failed: :=(%v), <-(%v), hex(%v), float(%v), under(%v), imag(%v)",
			foundColonAssign, foundArrow, foundHex, foundFloat, foundUnderline, foundImag)
	}
}

func TestGoHighlighter_RegistryAndDetection(t *testing.T) {
	h1 := syntax.DetectHighlighter("main.go")
	if h1 == nil {
		t.Fatalf("expected non-nil for main.go")
	}
	if syntax.GetLanguageName(h1) != "Go" {
		t.Fatalf("expected language 'Go', got %q", syntax.GetLanguageName(h1))
	}

	h2 := syntax.DetectHighlighter(".go")
	if h2 == nil || syntax.GetLanguageName(h2) != "Go" {
		t.Fatalf("expected non-nil for .go")
	}

	// 切换主题测试
	h1.ApplyTheme(theme.Light())
	colorsLight := h1.(*syntax.GoHighlighter).GetColors()
	if colorsLight.Keyword != syntax.DefaultLightGoColors.Keyword {
		t.Errorf("expected light theme keyword color")
	}

	h1.ApplyTheme(theme.Dark())
	colorsDark := h1.(*syntax.GoHighlighter).GetColors()
	if colorsDark.Keyword != syntax.DefaultDarkGoColors.Keyword {
		t.Errorf("expected dark theme keyword color")
	}
}
