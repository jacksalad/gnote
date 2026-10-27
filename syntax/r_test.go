package syntax_test

import (
	"testing"

	"github.com/jacksalad/gnote/syntax"
	"github.com/jacksalad/gnote/theme"
)

func TestRHighlighter_AssignmentAndPipes(t *testing.T) {
	h := syntax.NewRHighlighter()
	colors := h.GetColors()

	line := []rune(`model <- lm(mpg ~ wt + hp, data = mtcars) |> summary()`)
	spans, state := h.HighlightLine(0, line, syntax.StateRNormal)
	if state != syntax.StateRNormal {
		t.Fatalf("expected StateRNormal, got %d", state)
	}

	foundAssign := false
	foundLm := false
	foundTilde := false
	foundNativePipe := false
	foundSummary := false

	for _, s := range spans {
		str := string(line[s.StartCol:s.EndCol])
		if str == "<-" && s.Color == colors.Special {
			foundAssign = true
		}
		if str == "lm" && s.Color == colors.Function {
			foundLm = true
		}
		if str == "~" && s.Color == colors.Operator {
			foundTilde = true
		}
		if str == "|>" && s.Color == colors.Special {
			foundNativePipe = true
		}
		if str == "summary" && s.Color == colors.Function {
			foundSummary = true
		}
	}

	if !foundAssign || !foundLm || !foundTilde || !foundNativePipe || !foundSummary {
		t.Fatalf("tokens missing: assign=%v, lm=%v, tilde=%v, pipe=%v, summary=%v",
			foundAssign, foundLm, foundTilde, foundNativePipe, foundSummary)
	}
}

func TestRHighlighter_MultiLineStringsAndSpecialOps(t *testing.T) {
	h := syntax.NewRHighlighter()
	colors := h.GetColors()

	// 行 1: 跨行字符串开启
	line1 := []rune(`prompt <- "这是 R 语言中的`)
	spans1, state1 := h.HighlightLine(0, line1, syntax.StateRNormal)
	if state1 != syntax.StateRMultiLineStringDouble {
		t.Fatalf("expected StateRMultiLineStringDouble on line 1, got %d", state1)
	}
	if len(spans1) == 0 {
		t.Fatalf("expected spans on line 1")
	}

	// 行 2: 字符串内部
	line2 := []rune(`  多行提示词内容...`)
	spans2, state2 := h.HighlightLine(1, line2, state1)
	if state2 != syntax.StateRMultiLineStringDouble {
		t.Fatalf("expected StateRMultiLineStringDouble on line 2, got %d", state2)
	}
	if len(spans2) != 1 || spans2[0].Color != colors.String {
		t.Fatalf("expected string color on line 2")
	}

	// 行 3: 字符串闭合，后跟 %in% 操作符与数值后缀
	line3 := []rune(`"; is_member <- val %in% c(1L, 2L, 3L); # 检查成员`)
	spans3, state3 := h.HighlightLine(2, line3, state2)
	if state3 != syntax.StateRNormal {
		t.Fatalf("expected StateRNormal on line 3, got %d", state3)
	}

	foundInOp := false
	foundIntegerL := false
	foundComment := false

	for _, s := range spans3 {
		str := string(line3[s.StartCol:s.EndCol])
		if str == "%in%" && s.Color == colors.Special {
			foundInOp = true
		}
		if str == "1L" && s.Color == colors.Number {
			foundIntegerL = true
		}
		if str == "# 检查成员" && s.Color == colors.Comment {
			foundComment = true
		}
	}

	if !foundInOp || !foundIntegerL || !foundComment {
		t.Fatalf("tokens missing on line 3: inOp=%v, 1L=%v, comment=%v", foundInOp, foundIntegerL, foundComment)
	}
}

func TestRHighlighter_ConstantsAndTheme(t *testing.T) {
	h := syntax.NewRHighlighter()
	colors := h.GetColors()

	line := []rune(`result <- ifelse(is.na(x), NA, TRUE)`)
	spans, _ := h.HighlightLine(0, line, syntax.StateRNormal)

	foundNA := false
	foundTRUE := false

	for _, s := range spans {
		str := string(line[s.StartCol:s.EndCol])
		if str == "NA" && s.Color == colors.Constant {
			foundNA = true
		}
		if str == "TRUE" && s.Color == colors.Constant {
			foundTRUE = true
		}
	}

	if !foundNA || !foundTRUE {
		t.Fatalf("constants missing: NA=%v, TRUE=%v", foundNA, foundTRUE)
	}

	h.ApplyTheme(theme.Light())
	colorsLight := h.GetColors()
	if colorsLight.Keyword != syntax.DefaultLightRColors.Keyword {
		t.Errorf("expected light keyword color")
	}
}
