package syntax

import (
	"strings"
	"testing"

	"github.com/jacksalad/gnote/graphics"
)

// 回归：parseInlines 里每个 '['、'!['、'<' 都向右寻找配对符，找不到时只前进一格，
// 下一个同样的标记又把剩下的半行重扫一遍，耗时随行长平方增长：
// 20 万个 '[' 要 7.8 s，20 万个 '<' 要 16.0 s (3.2 万个时 185 / 362 ms)，同样长度的普通段落只要 3 ms。
func TestMarkdownHighlighter_UnmatchedInlineMarkersStayLinear(t *testing.T) {
	h := NewMarkdownHighlighter()

	for _, tc := range []struct{ label, line string }{
		{"unmatched [", p2LongLine("[")},
		{"unmatched <", p2LongLine("<")},
		{"unmatched ![", p2LongLine("![")},
		{"blockquote unmatched [", "> " + p2LongLine("[")},
		{"] without (", strings.Repeat("[ ", 100000) + "]"},
		{"( without )", p2LongLine("[a](")},
	} {
		t.Run(tc.label, func(t *testing.T) {
			t.Logf("%d runes: %v", len([]rune(tc.line)), p2HighlightTimed(t, h, tc.label, tc.line))
		})
	}

	// 没配对的标记不是链接，普通段落里不着色
	for _, line := range []string{"[[[", "<<<", "![![![", "[a] [b] [c]"} {
		runes := []rune(line)
		spans, state := h.HighlightLine(0, runes, 0)
		assertSpansWellFormed(t, runes, spans)
		if state != StateNormal || len(spans) != 0 {
			t.Errorf("%q: spans = %+v, state %d; want no spans in StateNormal", line, spans, state)
		}
	}

	// 配对符的窥探缓存是栈上局部量，分配次数不随行长增长
	short := []rune(strings.Repeat("[", 1000))
	long := []rune(p2LongLine("["))
	shortAllocs := testing.AllocsPerRun(10, func() { h.HighlightLine(0, short, 0) })
	longAllocs := testing.AllocsPerRun(5, func() { h.HighlightLine(0, long, 0) })
	if longAllocs > shortAllocs {
		t.Errorf("allocations grow with the line length: %d runes %.0f, %d runes %.0f",
			len(short), shortAllocs, len(long), longAllocs)
	}
}

// 窥探改成“游标只向前”的缓存之后，链接 / 图片 / 自动链接的识别结果不能变。
func TestMarkdownHighlighter_InlinePairsStillRecognised(t *testing.T) {
	h := NewMarkdownHighlighter()
	c := h.colors

	type want struct {
		text  string // 该行里第一处出现的子串
		color graphics.Color
	}
	for _, tc := range []struct {
		line  string
		wants []want
	}{
		{
			"see [a](u) and ![img](p.png) then <https://x.y> plus [b](v) [c](w)",
			[]want{
				{"[a]", c.LinkText}, {"(u)", c.LinkUrl},
				{"!", c.ImageMarker}, {"[img]", c.LinkText}, {"(p.png)", c.LinkUrl},
				{"<https://x.y>", c.LinkText},
				{"[b]", c.LinkText}, {"(v)", c.LinkUrl},
				{"[c]", c.LinkText}, {"(w)", c.LinkUrl},
			},
		},
		// 前一个 '[' 找到的 ']' 后面不是 '('：窥探缓存需要越过它才能找到后面的链接
		{"[a] [b](u)", []want{{"[b]", c.LinkText}, {"(u)", c.LinkUrl}}},
		{"[a](b) [c](d)", []want{{"[a]", c.LinkText}, {"(b)", c.LinkUrl}, {"[c]", c.LinkText}, {"(d)", c.LinkUrl}}},
		{"x [y](z) <https://q> ![i](j)", []want{{"[y]", c.LinkText}, {"(z)", c.LinkUrl}, {"<https://q>", c.LinkText}, {"[i]", c.LinkText}, {"(j)", c.LinkUrl}}},
		// 不是 URL 的尖括号不构成自动链接，不遮挡后面的
		{"<a<b> <https://c>", []want{{"<https://c>", c.LinkText}}},
		// 引用块走带默认颜色的解析路径 (引用标记片段连着后面的空格)
		{"> see [a](u) and ![i](p)", []want{{"> ", c.QuoteMarker}, {"[a]", c.LinkText}, {"(u)", c.LinkUrl}, {"[i]", c.LinkText}, {"(p)", c.LinkUrl}}},
	} {
		runes := []rune(tc.line)
		spans, state := h.HighlightLine(0, runes, 0)
		assertSpansWellFormed(t, runes, spans)
		if state != StateNormal {
			t.Errorf("%q: end state = %d, want %d", tc.line, state, StateNormal)
		}
		for _, w := range tc.wants {
			col := strings.Index(tc.line, w.text)
			s, ok := spanAt(spans, col)
			if !ok || s.Color != w.color || string(runes[s.StartCol:s.EndCol]) != w.text {
				t.Errorf("%q: %q coloured %+v, want %08X", tc.line, w.text, s, uint32(w.color))
			}
		}
	}
}

// 回归：围栏代码块的跨行状态不记围栏字符与长度，闭合时 ``` 与 ~~~ 不分长短：
// ~~~ 块里讲解 Markdown 的 ``` 示例会把代码块提前关掉，四个反引号包住的两个 ``` 示例
// 则把开关状态整个反过来。CommonMark 规定闭合围栏必须与开围栏同字符、且不短于开围栏。
func TestMarkdownHighlighter_ClosingFenceMustMatchOpening(t *testing.T) {
	h := NewMarkdownHighlighter()
	text := h.colors.CodeBlockText
	marker := h.colors.CodeBlockMarker

	// 断言各行的结束状态类别与整行片段颜色
	type lineWant struct {
		color    graphics.Color // 0 表示不检查
		fenced   bool           // 行尾仍处于围栏代码块内
		isMarker bool
	}
	check := func(name string, lines []string, wants []lineWant) {
		t.Helper()
		spans, states := p2HighlightDoc(t, h, lines)
		for i, w := range wants {
			if w.fenced != (states[i] >= StateFencedCodeBlock) {
				t.Errorf("%s: line %d %q: end state = %d, fenced = %v", name, i, lines[i], states[i], w.fenced)
			}
			if w.color != 0 {
				p2AssertWholeLine(t, lines[i], spans[i], w.color)
			}
			if w.isMarker && (len(spans[i]) == 0 || spans[i][0].Color != marker) {
				t.Errorf("%s: line %d %q: want the closing fence in marker color, got %+v", name, i, lines[i], spans[i])
			}
		}
	}

	// ~~~ 块内的 ``` 是代码内容，不是闭合围栏
	check("tilde", []string{"~~~", "```js", "console.log(1)", "```", "~~~", "after"},
		[]lineWant{{marker, true, true}, {text, true, false}, {text, true, false}, {text, true, false}, {marker, false, true}, {0, false, false}})

	// 四个反引号的围栏只被不少于四个反引号的围栏闭合 (空行没有片段，但状态保持)
	check("four backticks", []string{"````md", "```js", "a", "```", "", "```js", "b", "```", "````", "after"},
		[]lineWant{{0, true, false}, {text, true, false}, {text, true, false}, {text, true, false}, {0, true, false},
			{text, true, false}, {text, true, false}, {text, true, false}, {marker, false, true}, {0, false, false}})

	// 闭合围栏可以比开围栏长
	check("longer closer", []string{"```", "code", "`````", "after"},
		[]lineWant{{0, true, false}, {text, true, false}, {marker, false, true}, {0, false, false}})
	check("longer tilde closer", []string{"~~~sh", "echo 1", "~~~~~~", "after"},
		[]lineWant{{0, true, false}, {text, true, false}, {marker, false, true}, {0, false, false}})

	// 反过来不行：三个反引号关不住四个反引号的围栏
	check("shorter closer", []string{"````", "code", "```", "still code", "````", "after"},
		[]lineWant{{0, true, false}, {text, true, false}, {text, true, false}, {text, true, false}, {marker, false, true}, {0, false, false}})

	// 三个反引号的普通围栏保持原有状态值，外部按 StateFencedCodeBlock 传入前置状态的调用方不受影响
	_, states := p2HighlightDoc(t, h, []string{"```go", "code"})
	if states[0] != StateFencedCodeBlock || states[1] != StateFencedCodeBlock {
		t.Errorf("```go states = %v, want [%d %d]", states, StateFencedCodeBlock, StateFencedCodeBlock)
	}
	// 外来取值 (负数、极大值、任意正数) 不能让高亮器 panic 或产出负状态
	for _, prev := range []int{-1, 0, 2, 3, 7, 400, 1 << 30} {
		for _, line := range []string{"", "```", "~~~", "````", "code", "  ```"} {
			runes := []rune(line)
			spans, state := h.HighlightLine(0, runes, prev)
			assertSpansWellFormed(t, runes, spans)
			if prev >= 0 && state < 0 {
				t.Fatalf("prev %d, line %q: negative end state %d", prev, line, state)
			}
		}
	}
}

func BenchmarkMarkdownHighlighter_UnmatchedBrackets200k(b *testing.B) {
	h := NewMarkdownHighlighter()
	line := []rune(p2LongLine("["))
	b.ReportAllocs()
	b.SetBytes(int64(len(line)))
	for i := 0; i < b.N; i++ {
		h.HighlightLine(0, line, 0)
	}
}
