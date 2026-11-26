package syntax

import (
	"math"
	"testing"
)

// 回归：规则块内没有闭合的块注释只返回 StateCSSBlockComment，丢掉了“还在规则块里”的事实。
// 注释闭合之后，同一规则块里余下的声明被当成选择器着色：color 从 Property 变成 Selector。
func TestCSSHighlighter_BlockContextSurvivesMultilineComment(t *testing.T) {
	h := NewCSSHighlighter()
	c := h.colors

	lines := []string{
		".btn {",
		"  /* Primary CTA,",
		"     used on landing */",
		"  color: red;",
		"  background: blue;",
		"}",
		"h1 {",
		"  margin: 0;",
		"}",
	}
	spans, states := p2HighlightDoc(t, h, lines)
	if states[0] != StateCSSRuleBlock {
		t.Fatalf("end state after %q = %d, want %d", lines[0], states[0], StateCSSRuleBlock)
	}
	// 注释从行内的非空白处开始，一直延伸到行尾
	p2AssertToken(t, lines[1], spans[1], "/* Primary CTA,", c.Comment)
	p2AssertWholeLine(t, lines[2], spans[2], c.Comment)
	if states[2] != StateCSSRuleBlock {
		t.Fatalf("end state after the comment closes = %d, want %d (block forgotten)", states[2], StateCSSRuleBlock)
	}
	p2AssertToken(t, lines[3], spans[3], "color", c.Property)
	p2AssertToken(t, lines[3], spans[3], "red", c.Value)
	p2AssertToken(t, lines[4], spans[4], "background", c.Property)
	p2AssertToken(t, lines[4], spans[4], "blue", c.Value)
	if states[5] != StateCSSNormal {
		t.Fatalf("end state after '}' = %d, want %d", states[5], StateCSSNormal)
	}
	p2AssertToken(t, lines[6], spans[6], "h1", c.Selector)
	p2AssertToken(t, lines[7], spans[7], "margin", c.Property)

	// 注释在行内闭合，同一行随后的声明照常着色
	oneLine := "  /* note */ color: red;"
	spans, _ = p2HighlightDoc(t, h, []string{".btn {", oneLine})
	p2AssertToken(t, oneLine, spans[1], "color", c.Property)

	// 规则块外的注释仍然是普通的块注释状态，闭合后按选择器解析
	lines = []string{"/* header", "   comment */", "h1 { color: red; }"}
	spans, states = p2HighlightDoc(t, h, lines)
	if states[0] != StateCSSBlockComment {
		t.Fatalf("end state after %q = %d, want %d", lines[0], states[0], StateCSSBlockComment)
	}
	p2AssertWholeLine(t, lines[1], spans[1], c.Comment)
	p2AssertToken(t, lines[2], spans[2], "h1", c.Selector)
	p2AssertToken(t, lines[2], spans[2], "color", c.Property)

	// 嵌套两层时注释不会抹掉外层花括号
	lines = []string{
		"@media print {",
		"  .a {",
		"    /* multi",
		"       line */",
		"    color: red;",
		"  }",
		"  .b { x: y; }",
		"}",
		"p { q: r; }",
	}
	spans, states = p2HighlightDoc(t, h, lines)
	p2AssertToken(t, lines[4], spans[4], "color", c.Property)
	p2AssertToken(t, lines[6], spans[6], ".b", c.Selector)
	p2AssertToken(t, lines[6], spans[6], "x", c.Property)
	if states[7] != StateCSSNormal || states[8] != StateCSSNormal {
		t.Fatalf("end states after the closing braces = %d, %d; want %d", states[7], states[8], StateCSSNormal)
	}
	p2AssertToken(t, lines[8], spans[8], "p", c.Selector)
	p2AssertToken(t, lines[8], spans[8], "q", c.Property)
}

// 回归：“在规则块内”只是一个布尔值，任何 '}' 都把它清零。
// 嵌套规则或 @media 里的单行规则闭合之后，同一声明块里随后的 margin: 0 被当成选择器；
// 反过来 @media 块内单行规则的选择器 .nav 又被当成属性值。
func TestCSSHighlighter_NestedRulesKeepTheirContext(t *testing.T) {
	h := NewCSSHighlighter()
	c := h.colors

	// @media 块里是规则，规则里才是声明
	lines := []string{
		"@media (max-width: 768px) {",
		"  .nav { display: none; }",
		"  .content { padding: 10px; }",
		"}",
		"p { color: red; }",
	}
	spans, states := p2HighlightDoc(t, h, lines)
	if states[0] != StateCSSRuleBlock {
		t.Fatalf("end state after %q = %d, want %d", lines[0], states[0], StateCSSRuleBlock)
	}
	p2AssertToken(t, lines[1], spans[1], ".nav", c.Selector)
	p2AssertToken(t, lines[1], spans[1], "display", c.Property)
	p2AssertToken(t, lines[1], spans[1], "none", c.Value)
	p2AssertToken(t, lines[2], spans[2], ".content", c.Selector)
	p2AssertToken(t, lines[2], spans[2], "padding", c.Property)
	p2AssertToken(t, lines[2], spans[2], "10px", c.Number)
	if states[2] != states[0] {
		t.Fatalf("end state after the inner rules = %d, want %d (still inside @media)", states[2], states[0])
	}
	if states[3] != StateCSSNormal {
		t.Fatalf("end state after '}' = %d, want %d", states[3], StateCSSNormal)
	}
	p2AssertToken(t, lines[4], spans[4], "p", c.Selector)
	p2AssertToken(t, lines[4], spans[4], "color", c.Property)

	// 原生嵌套：内层规则闭合之后，外层声明块继续
	lines = []string{
		".card {",
		"  padding: 1rem;",
		"  &:hover { color: red; }",
		"  .title { font-weight: bold; }",
		"  margin: 0;",
		"}",
	}
	spans, states = p2HighlightDoc(t, h, lines)
	p2AssertToken(t, lines[1], spans[1], "padding", c.Property)
	p2AssertToken(t, lines[2], spans[2], "&", c.Selector)
	p2AssertToken(t, lines[2], spans[2], ":hover", c.Selector)
	p2AssertToken(t, lines[2], spans[2], "color", c.Property)
	p2AssertToken(t, lines[2], spans[2], "red", c.Value)
	p2AssertToken(t, lines[3], spans[3], ".title", c.Selector)
	p2AssertToken(t, lines[3], spans[3], "font-weight", c.Property)
	p2AssertToken(t, lines[3], spans[3], "bold", c.Value)
	p2AssertToken(t, lines[4], spans[4], "margin", c.Property)
	p2AssertToken(t, lines[4], spans[4], "0", c.Number)
	if states[4] != StateCSSRuleBlock {
		t.Fatalf("end state after the nested rules = %d, want %d (still inside .card)", states[4], StateCSSRuleBlock)
	}
	if states[5] != StateCSSNormal {
		t.Fatalf("end state after '}' = %d, want %d", states[5], StateCSSNormal)
	}

	// 带声明的 at-rule：块内仍然是声明
	lines = []string{"@font-face {", "  font-family: X;", "  src: url(a.woff2);", "}"}
	spans, states = p2HighlightDoc(t, h, lines)
	p2AssertToken(t, lines[1], spans[1], "font-family", c.Property)
	p2AssertToken(t, lines[2], spans[2], "src", c.Property)
	if states[3] != StateCSSNormal {
		t.Fatalf("end state after '}' = %d, want %d", states[3], StateCSSNormal)
	}
}

// 回归：规则块外没有数值规则，@media (max-width: 768px) 里的数字被逐位染成标点。
func TestCSSHighlighter_NumbersOutsideRuleBlocks(t *testing.T) {
	h := NewCSSHighlighter()
	c := h.colors

	line := "@media screen and (max-width: 768px) {"
	runes := []rune(line)
	spans, _ := h.HighlightLine(0, runes, 0)
	assertSpansWellFormed(t, runes, spans)
	p2AssertToken(t, line, spans, "768px", c.Number)
	p2AssertToken(t, line, spans, "@media", c.AtRule)
	p2AssertToken(t, line, spans, "screen", c.Selector)

	// 选择器里的数字仍属于选择器，不会被拆出来
	line = ".item-2, #col1 > a:hover {"
	runes = []rune(line)
	spans, _ = h.HighlightLine(0, runes, 0)
	assertSpansWellFormed(t, runes, spans)
	p2AssertToken(t, line, spans, ".item-2", c.Selector)
	p2AssertToken(t, line, spans, "#col1", c.Selector)
}

// 判断“这条语句是不是嵌套规则的选择器”时要越过字符串与注释里的花括号，
// 否则 content: "{" 这样的声明会凭空多出一层花括号，把后面的整份样式表带偏。
func TestCSSHighlighter_BracesInStringsAndCommentsDoNotChangeDepth(t *testing.T) {
	h := NewCSSHighlighter()
	c := h.colors

	for _, line := range []string{
		`.a { content: "{"; }`,
		`.a { content: "}"; }`,
		`.a { content: "it\"s {" }`,
		".a { color: red /* { */; }",
		".a { color: red /* } */; }",
		`.a[data-x="a;b"] { color: red; }`,
	} {
		runes := []rune(line)
		spans, state := h.HighlightLine(0, runes, 0)
		assertSpansWellFormed(t, runes, spans)
		if state != StateCSSNormal {
			t.Errorf("%s: end state = %d, want %d", line, state, StateCSSNormal)
		}
	}
	declaration := `.a { content: "{"; }`
	spans, _ := p2HighlightDoc(t, h, []string{declaration})
	p2AssertToken(t, declaration, spans[0], "content", c.Property)
	declaration = ".a { color: red /* { */; }"
	spans, _ = p2HighlightDoc(t, h, []string{declaration})
	p2AssertToken(t, declaration, spans[0], "color", c.Property)
	// 字符串里的分号不会截断选择器
	selector := `.a[data-x="a;b"] { color: red; }`
	spans, _ = p2HighlightDoc(t, h, []string{selector})
	p2AssertToken(t, selector, spans[0], ".a", c.Selector)

	// 没闭合的注释跨越多行时，深度同样保持
	lines := []string{`.a { content: "{"; /* open`, "   still comment", "*/ } p { q: r; }"}
	spans, states := p2HighlightDoc(t, h, lines)
	p2AssertToken(t, lines[2], spans[2], "p", c.Selector)
	p2AssertToken(t, lines[2], spans[2], "q", c.Property)
	if states[2] != StateCSSNormal {
		t.Fatalf("end state = %d, want %d", states[2], StateCSSNormal)
	}
}

// 编辑器缓存的状态只是不透明的整数：外来取值 (负数、极大值、任意奇偶) 不能让高亮器 panic，
// 非负的前置状态也不能得到负的结束状态。
func TestCSSHighlighter_StateToleratesForeignValues(t *testing.T) {
	h := NewCSSHighlighter()
	for _, prev := range []int{-1, -1 << 30, 0, 1, 2, 3, 5, 400, 1 << 30, math.MaxInt} {
		for _, line := range []string{"", "{", "}", "{{{", "}}}", "/*", "*/ x", "color: red;", ".a { b: c; }", "/* { */"} {
			runes := []rune(line)
			spans, state := h.HighlightLine(0, runes, prev)
			assertSpansWellFormed(t, runes, spans)
			if prev >= 0 && state < 0 {
				t.Fatalf("prev %d, line %q: negative end state %d", prev, line, state)
			}
		}
	}
}
