package syntax_test

import (
	"testing"

	"github.com/jacksalad/gnote/syntax"
	"github.com/jacksalad/gnote/theme"
)

func TestSQLHighlighter_QueryAndFunctions(t *testing.T) {
	h := syntax.NewSQLHighlighter()
	colors := h.GetColors()

	line := []rune(`SELECT id, COUNT(name) AS total FROM users WHERE status = 'active' AND age >= 18;`)
	spans, state := h.HighlightLine(0, line, syntax.StateSQLNormal)
	if state != syntax.StateSQLNormal {
		t.Fatalf("expected StateSQLNormal, got %d", state)
	}

	foundSelect := false
	foundCount := false
	foundAs := false
	foundFrom := false
	foundWhere := false
	foundString := false
	foundAnd := false
	foundGte := false
	foundNumber := false

	for _, s := range spans {
		str := string(line[s.StartCol:s.EndCol])
		if str == "SELECT" && s.Color == colors.Keyword {
			foundSelect = true
		}
		if str == "COUNT" && s.Color == colors.Function {
			foundCount = true
		}
		if str == "AS" && s.Color == colors.Keyword {
			foundAs = true
		}
		if str == "FROM" && s.Color == colors.Keyword {
			foundFrom = true
		}
		if str == "WHERE" && s.Color == colors.Keyword {
			foundWhere = true
		}
		if str == "'active'" && s.Color == colors.String {
			foundString = true
		}
		if str == "AND" && s.Color == colors.Keyword {
			foundAnd = true
		}
		if str == ">=" && s.Color == colors.Operator {
			foundGte = true
		}
		if str == "18" && s.Color == colors.Number {
			foundNumber = true
		}
	}

	if !foundSelect || !foundCount || !foundAs || !foundFrom || !foundWhere || !foundString || !foundAnd || !foundGte || !foundNumber {
		t.Fatalf("tokens missing: sel=%v, count=%v, as=%v, from=%v, where=%v, str=%v, and=%v, gte=%v, num=%v",
			foundSelect, foundCount, foundAs, foundFrom, foundWhere, foundString, foundAnd, foundGte, foundNumber)
	}
}

func TestSQLHighlighter_CommentsAndEscapedStrings(t *testing.T) {
	h := syntax.NewSQLHighlighter()
	colors := h.GetColors()

	// 行 1: 块注释开启
	line1 := []rune(`SELECT 1; /* 跨行注释开启`)
	spans1, state1 := h.HighlightLine(0, line1, syntax.StateSQLNormal)
	if state1 != syntax.StateSQLBlockComment {
		t.Fatalf("expected StateSQLBlockComment on line 1, got %d", state1)
	}
	if len(spans1) == 0 {
		t.Fatalf("expected spans on line 1")
	}

	// 行 2: 块注释内部
	line2 := []rune(`   注释内容第二行...`)
	spans2, state2 := h.HighlightLine(1, line2, state1)
	if state2 != syntax.StateSQLBlockComment {
		t.Fatalf("expected StateSQLBlockComment on line 2, got %d", state2)
	}
	if len(spans2) != 1 || spans2[0].Color != colors.Comment {
		t.Fatalf("expected comment color on line 2")
	}

	// 行 3: 块注释闭合，后续单引号转义字符串
	line3 := []rune(`   */ SELECT 'It''s fine' AS title; -- 结束`)
	spans3, state3 := h.HighlightLine(2, line3, state2)
	if state3 != syntax.StateSQLNormal {
		t.Fatalf("expected StateSQLNormal on line 3, got %d", state3)
	}

	foundStr := false
	foundComment := false
	for _, s := range spans3 {
		str := string(line3[s.StartCol:s.EndCol])
		if str == "'It''s fine'" && s.Color == colors.String {
			foundStr = true
		}
		if str == "-- 结束" && s.Color == colors.Comment {
			foundComment = true
		}
	}

	if !foundStr || !foundComment {
		t.Fatalf("tokens missing on line 3: str=%v, comment=%v", foundStr, foundComment)
	}
}

func TestSQLHighlighter_TypesAndTheme(t *testing.T) {
	h := syntax.NewSQLHighlighter()
	colors := h.GetColors()

	line := []rune(`CREATE TABLE logs (id INT, msg VARCHAR(255));`)
	spans, _ := h.HighlightLine(0, line, syntax.StateSQLNormal)

	foundCreate := false
	foundTable := false
	foundInt := false
	foundVarchar := false

	for _, s := range spans {
		str := string(line[s.StartCol:s.EndCol])
		if str == "CREATE" && s.Color == colors.Keyword {
			foundCreate = true
		}
		if str == "TABLE" && s.Color == colors.Keyword {
			foundTable = true
		}
		if str == "INT" && s.Color == colors.Type {
			foundInt = true
		}
		if str == "VARCHAR" && s.Color == colors.Type {
			foundVarchar = true
		}
	}

	if !foundCreate || !foundTable || !foundInt || !foundVarchar {
		t.Fatalf("tokens missing: create=%v, table=%v, int=%v, varchar=%v",
			foundCreate, foundTable, foundInt, foundVarchar)
	}

	h.ApplyTheme(theme.Light())
	colorsLight := h.GetColors()
	if colorsLight.Keyword != syntax.DefaultLightSQLColors.Keyword {
		t.Errorf("expected light keyword color")
	}
}
