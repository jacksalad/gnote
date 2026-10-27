package syntax_test

import (
	"testing"

	"github.com/jacksalad/gnote/syntax"
	"github.com/jacksalad/gnote/theme"
)

func TestJavaHighlighter_AnnotationsAndKeywords(t *testing.T) {
	h := syntax.NewJavaHighlighter()
	colors := h.GetColors()

	line := []rune(`@Override public List<String> getUserNames() { return null; }`)
	spans, state := h.HighlightLine(0, line, syntax.StateJavaNormal)
	if state != syntax.StateJavaNormal {
		t.Fatalf("expected StateJavaNormal, got %d", state)
	}

	foundAnnotation := false
	foundPublic := false
	foundList := false
	foundString := false
	foundFunc := false
	foundReturn := false
	foundNull := false

	for _, s := range spans {
		str := string(line[s.StartCol:s.EndCol])
		if str == "@Override" && s.Color == colors.Annotation {
			foundAnnotation = true
		}
		if str == "public" && s.Color == colors.Keyword {
			foundPublic = true
		}
		if str == "List" && s.Color == colors.Type {
			foundList = true
		}
		if str == "String" && s.Color == colors.Type {
			foundString = true
		}
		if str == "getUserNames" && s.Color == colors.Function {
			foundFunc = true
		}
		if str == "return" && s.Color == colors.Keyword {
			foundReturn = true
		}
		if str == "null" && s.Color == colors.Constant {
			foundNull = true
		}
	}

	if !foundAnnotation || !foundPublic || !foundList || !foundString || !foundFunc || !foundReturn || !foundNull {
		t.Fatalf("tokens missing: anno=%v, pub=%v, list=%v, str=%v, func=%v, ret=%v, null=%v",
			foundAnnotation, foundPublic, foundList, foundString, foundFunc, foundReturn, foundNull)
	}
}

func TestJavaHighlighter_TextBlockMultiLine(t *testing.T) {
	h := syntax.NewJavaHighlighter()
	colors := h.GetColors()

	// 行 1: 文本块开启
	line1 := []rune(`String html = """`)
	spans1, state1 := h.HighlightLine(0, line1, syntax.StateJavaNormal)
	if state1 != syntax.StateJavaTextBlock {
		t.Fatalf("expected StateJavaTextBlock on line 1, got %d", state1)
	}
	if len(spans1) == 0 {
		t.Fatalf("expected spans on line 1")
	}

	// 行 2: 文本块内部
	line2 := []rune(`    <html><body>Hello Java 17</body></html>`)
	spans2, state2 := h.HighlightLine(1, line2, state1)
	if state2 != syntax.StateJavaTextBlock {
		t.Fatalf("expected StateJavaTextBlock on line 2, got %d", state2)
	}
	if len(spans2) != 1 || spans2[0].Color != colors.String {
		t.Fatalf("expected string color on line 2")
	}

	// 行 3: 文本块闭合，后跟代码
	line3 := []rune(`    """; int len = html.length();`)
	spans3, state3 := h.HighlightLine(2, line3, state2)
	if state3 != syntax.StateJavaNormal {
		t.Fatalf("expected StateJavaNormal on line 3, got %d", state3)
	}

	foundCloseStr := false
	foundInt := false
	for _, s := range spans3 {
		str := string(line3[s.StartCol:s.EndCol])
		if str == `    """` && s.Color == colors.String {
			foundCloseStr = true
		}
		if str == "int" && s.Color == colors.Type {
			foundInt = true
		}
	}
	if !foundCloseStr || !foundInt {
		t.Fatalf("tokens missing: closeStr=%v, int=%v", foundCloseStr, foundInt)
	}
}

func TestJavaHighlighter_OperatorsAndNumbers(t *testing.T) {
	h := syntax.NewJavaHighlighter()
	colors := h.GetColors()

	line := []rune(`list.forEach(System.out::println); long val = 1_000_000L >>> 2;`)
	spans, _ := h.HighlightLine(0, line, syntax.StateJavaNormal)

	foundMethodRef := false
	foundUnsignedShift := false
	foundNumber := false
	foundLong := false

	for _, s := range spans {
		str := string(line[s.StartCol:s.EndCol])
		if str == "::" && s.Color == colors.Operator {
			foundMethodRef = true
		}
		if str == ">>>" && s.Color == colors.Operator {
			foundUnsignedShift = true
		}
		if str == "1_000_000L" && s.Color == colors.Number {
			foundNumber = true
		}
		if str == "long" && s.Color == colors.Type {
			foundLong = true
		}
	}

	if !foundMethodRef || !foundUnsignedShift || !foundNumber || !foundLong {
		t.Fatalf("tokens missing: ::=%v, >>>=%v, num=%v, long=%v",
			foundMethodRef, foundUnsignedShift, foundNumber, foundLong)
	}
}

func TestJavaHighlighter_Theme(t *testing.T) {
	h := syntax.NewJavaHighlighter()
	h.ApplyTheme(theme.Light())
	colorsLight := h.GetColors()
	if colorsLight.Keyword != syntax.DefaultLightJavaColors.Keyword {
		t.Errorf("expected light keyword color")
	}
}
