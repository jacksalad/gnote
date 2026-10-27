package syntax

import (
	"testing"

	"github.com/jacksalad/gnote/theme"
)

func TestMarkdownHighlighter_Headings(t *testing.T) {
	h := NewMarkdownHighlighter()

	// # H1
	line := []rune("# 标题一 Heading 1")
	spans, state := h.HighlightLine(0, line, StateNormal)
	if state != StateNormal {
		t.Fatalf("expected StateNormal, got %d", state)
	}
	if len(spans) < 2 {
		t.Fatalf("expected at least 2 spans for H1, got %d", len(spans))
	}
	if spans[0].Color != h.colors.HMarker {
		t.Fatalf("expected HMarker color for #")
	}
	if spans[1].Color != h.colors.H1 {
		t.Fatalf("expected H1 color for text, got %v", spans[1].Color)
	}

	// ### H3
	line3 := []rune("### 标题三")
	spans3, _ := h.HighlightLine(1, line3, StateNormal)
	if len(spans3) < 2 || spans3[1].Color != h.colors.H3 {
		t.Fatalf("expected H3 color for text")
	}
}

func TestMarkdownHighlighter_FencedCodeBlock(t *testing.T) {
	h := NewMarkdownHighlighter()

	// ```go
	lineStart := []rune("```go")
	spansStart, stateStart := h.HighlightLine(0, lineStart, StateNormal)
	if stateStart != StateFencedCodeBlock {
		t.Fatalf("expected state FencedCodeBlock after ```go, got %d", stateStart)
	}
	if len(spansStart) < 2 {
		t.Fatalf("expected marker and lang spans, got %d", len(spansStart))
	}

	// Code content
	lineCode := []rune("    fmt.Println(\"hello\")")
	spansCode, stateCode := h.HighlightLine(1, lineCode, StateFencedCodeBlock)
	if stateCode != StateFencedCodeBlock {
		t.Fatalf("expected state FencedCodeBlock inside code, got %d", stateCode)
	}
	if len(spansCode) != 1 || spansCode[0].Color != h.colors.CodeBlockText {
		t.Fatalf("expected code block text color")
	}

	// ``` closing
	lineClose := []rune("```")
	_, stateClose := h.HighlightLine(2, lineClose, StateFencedCodeBlock)
	if stateClose != StateNormal {
		t.Fatalf("expected state Normal after closing ```, got %d", stateClose)
	}
}

func TestMarkdownHighlighter_Inlines(t *testing.T) {
	h := NewMarkdownHighlighter()

	// Inline code: This is `variable` test.
	line := []rune("This is `variable` test.")
	spans, _ := h.HighlightLine(0, line, StateNormal)
	if len(spans) != 3 { // marker, text, marker
		t.Fatalf("expected 3 spans for inline code, got %d", len(spans))
	}
	if spans[1].Color != h.colors.InlineCodeText {
		t.Fatalf("expected inline code text color")
	}

	// Bold: **hello**
	lineBold := []rune("Say **hello** world")
	spansBold, _ := h.HighlightLine(0, lineBold, StateNormal)
	if len(spansBold) != 3 { // **, hello, **
		t.Fatalf("expected 3 spans for bold, got %d", len(spansBold))
	}
	if spansBold[1].Color != h.colors.BoldText {
		t.Fatalf("expected bold text color")
	}

	// Link: [Google](https://google.com)
	lineLink := []rune("[Google](https://google.com)")
	spansLink, _ := h.HighlightLine(0, lineLink, StateNormal)
	if len(spansLink) != 2 {
		t.Fatalf("expected 2 spans for link [text] and (url), got %d", len(spansLink))
	}
	if spansLink[0].Color != h.colors.LinkText || spansLink[1].Color != h.colors.LinkUrl {
		t.Fatalf("expected link colors")
	}
}

func TestMarkdownHighlighter_ThemeSwitch(t *testing.T) {
	h := NewMarkdownHighlighter()
	if !h.isDark {
		t.Fatalf("expected initial dark theme")
	}

	// 切换到浅色主题
	h.ApplyTheme(theme.Light())
	if h.isDark {
		t.Fatalf("expected isDark to be false after ThemeLight")
	}
	if h.colors.H1 != DefaultLightMarkdownColors.H1 {
		t.Fatalf("expected light colors applied")
	}
}

func TestMarkdownHighlighter_NoInlineHTML(t *testing.T) {
	h := NewMarkdownHighlighter()

	// Markdown 文档不解析行内 HTML：尖括号内容一律按正文呈现
	for _, s := range []string{
		"特殊赋值(<-)、管道符(%>%/>)、二元操作符(%in%)",
		"比例 a < b > c 的判定",
		"模板占位符 <*> 与 <name> 与 </name>",
		"<!DOCTYPE html> 与 <!-- 注释 -->",
		"见 <a href=\"#\">链接</a>",
	} {
		line := []rune(s)
		spans, _ := h.HighlightLine(0, line, StateNormal)
		if len(spans) != 0 {
			t.Fatalf("%q: expected no spans, got %d: %+v", s, len(spans), spans)
		}
	}

	// '</' 与 '>' 不得吞掉行内代码的反引号
	codeLine := []rune("运算 a < b 与 `c > d` 结束")
	spans, _ := h.HighlightLine(0, codeLine, StateNormal)
	if len(spans) != 3 {
		t.Fatalf("expected 3 spans for inline code, got %d: %+v", len(spans), spans)
	}
	codeText := string(codeLine[spans[1].StartCol:spans[1].EndCol])
	if codeText != "c > d" || spans[1].Color != h.colors.InlineCodeText {
		t.Fatalf("expected intact inline code %q, got %q", "c > d", codeText)
	}

	// CommonMark 自动链接仍保留
	autoLine := []rune("见 <https://example.com/x> 说明")
	spans, _ = h.HighlightLine(0, autoLine, StateNormal)
	if len(spans) != 1 || spans[0].Color != h.colors.LinkText {
		t.Fatalf("expected single autolink span, got %+v", spans)
	}
	if got := string(autoLine[spans[0].StartCol:spans[0].EndCol]); got != "<https://example.com/x>" {
		t.Fatalf("unexpected autolink range %q", got)
	}
}
