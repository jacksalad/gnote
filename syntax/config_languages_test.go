package syntax

import (
	"strings"
	"testing"

	"github.com/jacksalad/gnote/theme"
)

// spanAt 返回覆盖指定字符列的高亮片段
func spanAt(spans []Span, col int) (Span, bool) {
	for _, s := range spans {
		if col >= s.StartCol && col < s.EndCol {
			return s, true
		}
	}
	return Span{}, false
}

func TestJSONHighlighter_Tokens(t *testing.T) {
	h := NewJSONHighlighter()
	line := []rune(`{"key": "value", "n": 42, "ok": true, "nil": null}`)
	spans, state := h.HighlightLine(0, line, StateNormal)
	if state != StateNormal {
		t.Fatalf("unexpected end state %d", state)
	}

	text := string(line)
	at := func(sub string, offset int) int {
		i := strings.Index(text, sub)
		if i < 0 {
			t.Fatalf("test line lacks %q", sub)
		}
		return i + offset
	}

	if s, ok := spanAt(spans, at(`"key"`, 1)); !ok || s.Color != h.colors.Key {
		t.Fatalf("json key span = %+v ok=%v", s, ok)
	}
	if s, ok := spanAt(spans, at(`"value"`, 1)); !ok || s.Color != h.colors.String {
		t.Fatalf("json string span = %+v ok=%v", s, ok)
	}
	if s, ok := spanAt(spans, at("42", 0)); !ok || s.Color != h.colors.Number {
		t.Fatalf("json number span = %+v ok=%v", s, ok)
	}
	if s, ok := spanAt(spans, at("true", 0)); !ok || s.Color != h.colors.Boolean {
		t.Fatalf("json boolean span = %+v ok=%v", s, ok)
	}
	if s, ok := spanAt(spans, at("null", 0)); !ok || s.Color != h.colors.Null {
		t.Fatalf("json null span = %+v ok=%v", s, ok)
	}
}

func TestJSONHighlighter_Escape(t *testing.T) {
	h := NewJSONHighlighter()
	line := []rune(`{"p": "C:\\dev\\gnote"}`)
	spans, _ := h.HighlightLine(0, line, StateNormal)
	// 定位反斜杠
	bs := -1
	for i, r := range line {
		if r == '\\' {
			bs = i
			break
		}
	}
	if bs < 0 {
		t.Fatal("test line lacks backslash")
	}
	if s, ok := spanAt(spans, bs); !ok || s.Color != h.colors.Escape {
		t.Fatalf("json escape span = %+v ok=%v at %d", s, ok, bs)
	}
}

func TestYAMLHighlighter_Tokens(t *testing.T) {
	h := NewYAMLHighlighter()

	keyLine := []rune("name: value")
	spans, _ := h.HighlightLine(0, keyLine, StateNormal)
	if s, ok := spanAt(spans, 0); !ok || s.Color != h.colors.Key {
		t.Fatalf("yaml key span = %+v ok=%v", s, ok)
	}
	if s, ok := spanAt(spans, 6); !ok || s.Color != h.colors.String {
		t.Fatalf("yaml scalar span = %+v ok=%v", s, ok)
	}

	boolLine := []rune("enabled: true")
	spans, _ = h.HighlightLine(1, boolLine, StateNormal)
	if s, ok := spanAt(spans, 9); !ok || s.Color != h.colors.Boolean {
		t.Fatalf("yaml boolean span = %+v ok=%v", s, ok)
	}

	commentLine := []rune("# 注释 comment")
	spans, _ = h.HighlightLine(2, commentLine, StateNormal)
	if s, ok := spanAt(spans, 0); !ok || s.Color != h.colors.Comment {
		t.Fatalf("yaml comment span = %+v ok=%v", s, ok)
	}

	docLine := []rune("---")
	spans, _ = h.HighlightLine(3, docLine, StateNormal)
	if s, ok := spanAt(spans, 0); !ok || s.Color != h.colors.Directive {
		t.Fatalf("yaml doc separator span = %+v ok=%v", s, ok)
	}

	anchorLine := []rune("base: &anchor text")
	spans, _ = h.HighlightLine(4, anchorLine, StateNormal)
	if s, ok := spanAt(spans, 6); !ok || s.Color != h.colors.Anchor {
		t.Fatalf("yaml anchor span = %+v ok=%v", s, ok)
	}

	// 块标量：| 之后进入带缩进的块状态
	_, state := h.HighlightLine(5, []rune("text: |"), StateNormal)
	if state < StateYAMLBlockScalarBase {
		t.Fatalf("expected block scalar state, got %d", state)
	}
}

func TestTOMLHighlighter_Tokens(t *testing.T) {
	h := NewTOMLHighlighter()

	tableLine := []rune("[server]")
	spans, _ := h.HighlightLine(0, tableLine, StateNormal)
	if s, ok := spanAt(spans, 1); !ok || s.Color != h.colors.Table {
		t.Fatalf("toml table span = %+v ok=%v", s, ok)
	}

	kvLine := []rune(`host = "localhost"`)
	spans, _ = h.HighlightLine(1, kvLine, StateNormal)
	if s, ok := spanAt(spans, 0); !ok || s.Color != h.colors.Key {
		t.Fatalf("toml key span = %+v ok=%v", s, ok)
	}
	if s, ok := spanAt(spans, 8); !ok || s.Color != h.colors.String {
		t.Fatalf("toml string span = %+v ok=%v", s, ok)
	}

	numLine := []rune("port = 8080")
	spans, _ = h.HighlightLine(2, numLine, StateNormal)
	if s, ok := spanAt(spans, 7); !ok || s.Color != h.colors.Number {
		t.Fatalf("toml number span = %+v ok=%v", s, ok)
	}

	boolLine := []rune("debug = false")
	spans, _ = h.HighlightLine(3, boolLine, StateNormal)
	if s, ok := spanAt(spans, 8); !ok || s.Color != h.colors.Boolean {
		t.Fatalf("toml boolean span = %+v ok=%v", s, ok)
	}

	dtLine := []rune("when = 2024-01-02T03:04:05Z")
	spans, _ = h.HighlightLine(4, dtLine, StateNormal)
	if s, ok := spanAt(spans, 8); !ok || s.Color != h.colors.DateTime {
		t.Fatalf("toml datetime span = %+v ok=%v", s, ok)
	}

	commentLine := []rune("# comment")
	spans, _ = h.HighlightLine(5, commentLine, StateNormal)
	if s, ok := spanAt(spans, 0); !ok || s.Color != h.colors.Comment {
		t.Fatalf("toml comment span = %+v ok=%v", s, ok)
	}

	arrayLine := []rune(`tags = ["a", "b"]`)
	spans, _ = h.HighlightLine(6, arrayLine, StateNormal)
	if s, ok := spanAt(spans, 8); !ok || s.Color != h.colors.String {
		t.Fatalf("toml array string span = %+v ok=%v", s, ok)
	}
}

func TestNewHighlighters_ThemeSwitch(t *testing.T) {
	hs := []Highlighter{
		NewJSONHighlighter(),
		NewYAMLHighlighter(),
		NewTOMLHighlighter(),
	}
	for _, h := range hs {
		h.ApplyTheme(theme.Dark())
		h.ApplyTheme(theme.Light())
	}
}
