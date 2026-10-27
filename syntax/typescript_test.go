package syntax_test

import (
	"testing"

	"github.com/jacksalad/gnote/syntax"
	"github.com/jacksalad/gnote/theme"
)

func TestTSHighlighter_TypesAndInterfaces(t *testing.T) {
	h := syntax.NewTypeScriptHighlighter()
	colors := h.GetColors()

	line := []rune(`interface User<T> { id: number; name: string; isReady: boolean; data: T; }`)
	spans, state := h.HighlightLine(0, line, syntax.StateTSNormal)
	if state != syntax.StateTSNormal {
		t.Fatalf("expected StateTSNormal, got %d", state)
	}

	foundInterface := false
	foundNumber := false
	foundString := false
	foundBoolean := false

	for _, s := range spans {
		str := string(line[s.StartCol:s.EndCol])
		if str == "interface" && s.Color == colors.Keyword {
			foundInterface = true
		}
		if str == "number" && s.Color == colors.Type {
			foundNumber = true
		}
		if str == "string" && s.Color == colors.Type {
			foundString = true
		}
		if str == "boolean" && s.Color == colors.Type {
			foundBoolean = true
		}
	}

	if !foundInterface || !foundNumber || !foundString || !foundBoolean {
		t.Fatalf("tokens missing: interface=%v, num=%v, str=%v, bool=%v",
			foundInterface, foundNumber, foundString, foundBoolean)
	}
}

func TestTSHighlighter_DecoratorsAndGenerics(t *testing.T) {
	h := syntax.NewTypeScriptHighlighter()
	colors := h.GetColors()

	line := []rune(`@Injectable() export class AuthService { token: any = null; }`)
	spans, _ := h.HighlightLine(0, line, syntax.StateTSNormal)

	foundDecorator := false
	foundExport := false
	foundClass := false
	foundAny := false
	foundNull := false

	for _, s := range spans {
		str := string(line[s.StartCol:s.EndCol])
		if str == "@Injectable" && s.Color == colors.Decorator {
			foundDecorator = true
		}
		if str == "export" && s.Color == colors.Keyword {
			foundExport = true
		}
		if str == "class" && s.Color == colors.Keyword {
			foundClass = true
		}
		if str == "any" && s.Color == colors.Type {
			foundAny = true
		}
		if str == "null" && s.Color == colors.Constant {
			foundNull = true
		}
	}

	if !foundDecorator || !foundExport || !foundClass || !foundAny || !foundNull {
		t.Fatalf("tokens missing: dec=%v, export=%v, class=%v, any=%v, null=%v",
			foundDecorator, foundExport, foundClass, foundAny, foundNull)
	}
}

func TestTSHighlighter_TypoScriptAliasAndTheme(t *testing.T) {
	// 测试 TypoScript 别名构造函数
	h := syntax.NewTypoScriptHighlighter()
	if h == nil {
		t.Fatalf("expected non-nil TypoScript highlighter")
	}

	h.ApplyTheme(theme.Light())
	colorsLight := h.GetColors()
	if colorsLight.Keyword != syntax.DefaultLightTSColors.Keyword {
		t.Errorf("expected light keyword color")
	}
}
