package syntax_test

import (
	"testing"

	"github.com/jacksalad/gnote/syntax"
	"github.com/jacksalad/gnote/theme"
)

func TestHTMLHighlighter_BasicTagsAndAttributes(t *testing.T) {
	h := syntax.NewHTMLHighlighter()
	colors := h.GetColors()

	line := []rune(`<!DOCTYPE html><div id="main" class="box">&copy; 文本内容</div>`)
	spans, state := h.HighlightLine(0, line, syntax.StateHTMLNormal)
	if state != syntax.StateHTMLNormal {
		t.Fatalf("expected StateHTMLNormal, got %d", state)
	}

	foundDoctype := false
	foundTagDiv := false
	foundAttrId := false
	foundValMain := false
	foundEntity := false

	for _, s := range spans {
		str := string(line[s.StartCol:s.EndCol])
		if str == "<!DOCTYPE html>" && s.Color == colors.Doctype {
			foundDoctype = true
		}
		if str == "div" && s.Color == colors.Tag {
			foundTagDiv = true
		}
		if str == "id" && s.Color == colors.Attribute {
			foundAttrId = true
		}
		if str == `"main"` && s.Color == colors.Value {
			foundValMain = true
		}
		if str == "&copy;" && s.Color == colors.Entity {
			foundEntity = true
		}
	}

	if !foundDoctype || !foundTagDiv || !foundAttrId || !foundValMain || !foundEntity {
		t.Fatalf("tokens missing: doc=%v, div=%v, id=%v, val=%v, entity=%v",
			foundDoctype, foundTagDiv, foundAttrId, foundValMain, foundEntity)
	}
}

func TestHTMLHighlighter_MultiLineCommentsAndTags(t *testing.T) {
	h := syntax.NewHTMLHighlighter()
	colors := h.GetColors()

	// 跨行注释测试
	line1 := []rune(`<!-- 跨行注释第一行`)
	spans1, state1 := h.HighlightLine(0, line1, syntax.StateHTMLNormal)
	if state1 != syntax.StateHTMLComment {
		t.Fatalf("expected StateHTMLComment on line 1, got %d", state1)
	}
	if len(spans1) == 0 {
		t.Fatalf("expected spans on line 1")
	}

	line2 := []rune(`     注释第二行正文...`)
	spans2, state2 := h.HighlightLine(1, line2, state1)
	if state2 != syntax.StateHTMLComment {
		t.Fatalf("expected StateHTMLComment on line 2, got %d", state2)
	}
	if len(spans2) != 1 || spans2[0].Color != colors.Comment {
		t.Fatalf("expected comment color on line 2")
	}

	line3 := []rune(`     --> <p>Hello</p>`)
	spans3, state3 := h.HighlightLine(2, line3, state2)
	if state3 != syntax.StateHTMLNormal {
		t.Fatalf("expected StateHTMLNormal on line 3, got %d", state3)
	}

	foundCommentEnd := false
	foundTagP := false
	for _, s := range spans3 {
		str := string(line3[s.StartCol:s.EndCol])
		if str == "     -->" && s.Color == colors.Comment {
			foundCommentEnd = true
		}
		if str == "p" && s.Color == colors.Tag {
			foundTagP = true
		}
	}
	if !foundCommentEnd || !foundTagP {
		t.Fatalf("tokens missing: end=%v, p=%v", foundCommentEnd, foundTagP)
	}
}

func TestHTMLHighlighter_Theme(t *testing.T) {
	h := syntax.NewHTMLHighlighter()
	h.ApplyTheme(theme.Light())
	colorsLight := h.GetColors()
	if colorsLight.Tag != syntax.DefaultLightHTMLColors.Tag {
		t.Errorf("expected light tag color")
	}
}
