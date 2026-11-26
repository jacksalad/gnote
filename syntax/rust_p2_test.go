package syntax

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jacksalad/gnote/graphics"
)

// 本文件里的 p2 前缀辅助函数同时供 markdown_p2_test.go / yaml_p2_test.go / css_p2_test.go 使用。

// p2LongLineBudget 病态长行 (20 万 rune) 单行高亮的耗时上限。修复后实测都在几毫秒到几十毫秒，
// 这里留出充足的余量，只用来拦住“每个标记都向右扫到行尾”的二次方回归。
const p2LongLineBudget = 200 * time.Millisecond

// p2LongLine 把 unit 重复到不少于 20 万 rune，模拟一行里塞满同一种标记的极端文本
func p2LongLine(unit string) string {
	return strings.Repeat(unit, (200000+len(unit)-1)/len(unit))
}

// p2HighlightDoc 按编辑器的方式逐行高亮 (上一行结束状态作为下一行前置状态)，
// 校验每行片段合法，并返回各行片段与结束状态。
func p2HighlightDoc(t *testing.T, h Highlighter, lines []string) (spans [][]Span, states []int) {
	t.Helper()
	state := 0
	for i, line := range lines {
		runes := []rune(line)
		s, next := h.HighlightLine(i, runes, state)
		assertSpansWellFormed(t, runes, s)
		if next < 0 {
			t.Fatalf("line %d %q: negative end state %d", i, line, next)
		}
		spans = append(spans, s)
		states = append(states, next)
		state = next
	}
	return spans, states
}

// p2AssertWellFormed 与 assertSpansWellFormed 约束相同，但失败信息不回显整行 (病态长行有 20 万字符)。
func p2AssertWellFormed(t *testing.T, line []rune, spans []Span) {
	t.Helper()
	last := 0
	for i, s := range spans {
		if s.StartCol < last || s.EndCol <= s.StartCol || s.EndCol > len(line) {
			t.Fatalf("span %d = [%d,%d) invalid for a %d-rune line (previous end %d)", i, s.StartCol, s.EndCol, len(line), last)
		}
		last = s.EndCol
	}
}

// p2HighlightTimed 断言一行在长行预算内高亮完成并返回耗时；卡死时由看门狗快速失败。
// label 用于失败信息，不回显整行。
func p2HighlightTimed(t *testing.T, h Highlighter, label, line string) time.Duration {
	t.Helper()
	type result struct {
		spans   []Span
		state   int
		elapsed time.Duration
	}
	runes := []rune(line)
	done := make(chan result, 1)
	go func() {
		start := time.Now()
		spans, state := h.HighlightLine(0, runes, 0)
		done <- result{spans, state, time.Since(start)}
	}()
	var r result
	select {
	case r = <-done:
	case <-time.After(highlightTimeout):
		t.Fatalf("%s: %d-rune line did not return within %v", label, len(runes), highlightTimeout)
	}
	p2AssertWellFormed(t, runes, r.spans)
	if r.state < 0 {
		t.Errorf("%s: negative end state %d", label, r.state)
	}
	if r.elapsed > p2LongLineBudget {
		t.Errorf("%s: %d-rune line took %v, want < %v", label, len(runes), r.elapsed, p2LongLineBudget)
	}
	return r.elapsed
}

// p2AssertWholeLine 断言一行只有一段覆盖整行的 color 片段
func p2AssertWholeLine(t *testing.T, line string, spans []Span, color graphics.Color) {
	t.Helper()
	if len(spans) != 1 || spans[0].StartCol != 0 || spans[0].EndCol != len([]rune(line)) || spans[0].Color != color {
		t.Fatalf("line %q: want one whole-line span in color %08X, got %+v", line, uint32(color), spans)
	}
}

// p2AssertToken 断言 line 里第一处出现的 token 恰好是一段 color 片段
func p2AssertToken(t *testing.T, line string, spans []Span, token string, color graphics.Color) {
	t.Helper()
	runes := []rune(line)
	s, ok := spanAt(spans, strings.Index(line, token))
	if !ok || s.Color != color || string(runes[s.StartCol:s.EndCol]) != token {
		t.Errorf("line %q: %q coloured %+v, want %08X", line, token, s, uint32(color))
	}
}

// 回归：原生字符串的跨行状态只记“在原生字符串里”，不记 '#' 的个数。
// 下一行的闭合扫描因此只认“引号后至少一个 '#'”：r"..." (零个 '#') 永远等不到闭合，
// r##"..."## 又被内容里的单个 "# 提前闭合，两种情况下中间的代码都被染成字符串。
// 正确规则与 rustc 一致：闭合引号后必须恰好跟着开头那么多个 '#'。
func TestRustHighlighter_RawStringHashCountCrossesLines(t *testing.T) {
	h := NewRustHighlighter()
	str := h.colors.String

	t.Run("zero hashes", func(t *testing.T) {
		lines := []string{
			`const SCRIPT: &str = r"`,
			`"# is still content`,
			`"; fn after() {}`,
			`fn later() {}`,
		}
		spans, states := p2HighlightDoc(t, h, lines)
		for i := 0; i < 2; i++ {
			if states[i] != StateRustRawString {
				t.Fatalf("line %d: end state = %d, want %d (r\" stays open)", i, states[i], StateRustRawString)
			}
		}
		p2AssertWholeLine(t, lines[1], spans[1], str)
		if states[2] != StateRustNormal || states[3] != StateRustNormal {
			t.Fatalf("state after closing = %d, %d; want %d", states[2], states[3], StateRustNormal)
		}
		// 第 3 行只有闭合引号属于字符串，其后的代码恢复常规则着色
		if s, ok := spanAt(spans[2], 0); !ok || s.EndCol != 1 || s.Color != str {
			t.Fatalf("closing quote span = %+v, want [0,1) in string color", s)
		}
		if s, ok := spanAt(spans[2], 3); !ok || s.Color != h.colors.Keyword {
			t.Fatalf("'fn' after the raw string = %+v, want keyword color", s)
		}
	})

	t.Run("one hash", func(t *testing.T) {
		lines := []string{
			`#[test]`,
			`const JSON: &str = r#"`,
			`  {"name": "Rust", "tip": "a"##b}`,
			`"#;`,
			`fn after() {}`,
		}
		spans, states := p2HighlightDoc(t, h, lines)
		want := StateRustRawString + 1
		if states[1] != want {
			t.Fatalf("end state after r#\" = %d, want %d", states[1], want)
		}
		if states[2] != want {
			t.Fatalf("content line with \"## closed the r#\" string early: end state = %d, want %d", states[2], want)
		}
		p2AssertWholeLine(t, lines[2], spans[2], str)
		if states[3] != StateRustNormal || states[4] != StateRustNormal {
			t.Fatalf("state after closing = %d, %d; want %d", states[3], states[4], StateRustNormal)
		}
		if s, ok := spanAt(spans[3], 0); !ok || s.EndCol != 2 || s.Color != str {
			t.Fatalf("closing \"# span = %+v, want [0,2) in string color", s)
		}
	})

	t.Run("three hashes", func(t *testing.T) {
		lines := []string{
			`let s = r###"`,
			`contains "# and "## inside`,
			`"###; let x = 1;`,
		}
		spans, states := p2HighlightDoc(t, h, lines)
		want := StateRustRawString + 3
		if states[0] != want {
			t.Fatalf("end state after r###\" = %d, want %d", states[0], want)
		}
		if states[1] != want {
			t.Fatalf("content line with \"# / \"## closed the r###\" string early: end state = %d, want %d", states[1], want)
		}
		p2AssertWholeLine(t, lines[1], spans[1], str)
		if states[2] != StateRustNormal {
			t.Fatalf("state after \"### = %d, want %d", states[2], StateRustNormal)
		}
		if s, ok := spanAt(spans[2], 0); !ok || s.EndCol != 4 || s.Color != str {
			t.Fatalf("closing \"### span = %+v, want [0,4) in string color", s)
		}
		if s, ok := spanAt(spans[2], 14); !ok || s.Color != h.colors.Number {
			t.Fatalf("'1' after the raw string = %+v, want number color", s)
		}
	})

	// 字节原生字符串 br#"..."# 走同一条路径
	t.Run("byte raw string", func(t *testing.T) {
		lines := []string{`let b = br#"`, `raw " bytes`, `"#;`}
		spans, states := p2HighlightDoc(t, h, lines)
		if states[0] != StateRustRawString+1 || states[1] != StateRustRawString+1 || states[2] != StateRustNormal {
			t.Fatalf("br#\" states = %v, want [%d %d %d]", states, StateRustRawString+1, StateRustRawString+1, StateRustNormal)
		}
		p2AssertWholeLine(t, lines[1], spans[1], str)
	})

	// 审计复现：零个 '#' 的多行 r"..." 把后面两行代码全部留在字符串状态
	t.Run("audit repro", func(t *testing.T) {
		_, states := p2HighlightDoc(t, h, []string{
			`const SCRIPT: &str = r"`,
			`fn main() { println!("hi"); }`,
			`";`,
		})
		if states[1] != StateRustNormal || states[2] != StateRustNormal {
			t.Fatalf("states after the repro = %v; the r\" string never closed", states)
		}
	})
}

// 同一行内的闭合同样要求 '#' 个数精确匹配：多出来的 '#' 属于内容，不是闭合序列的一部分。
func TestRustHighlighter_RawStringClosesOnExactHashCount(t *testing.T) {
	h := NewRustHighlighter()
	str := h.colors.String

	for _, tc := range []struct {
		line string
		want string // 应当成为单独一段字符串的原生字符串字面量
	}{
		{`let a = r##"x "# y"##;`, `r##"x "# y"##`},
		{`let b = r#"a"##b"#;`, `r#"a"##b"#`},
		{`let c = r###"a"#b"##c"###;`, `r###"a"#b"##c"###`},
		{`let d = r"a ## b";`, `r"a ## b"`},
		{`let e = br#"p"##q"#;`, `br#"p"##q"#`},
	} {
		runes := []rune(tc.line)
		spans, state := h.HighlightLine(0, runes, 0)
		assertSpansWellFormed(t, runes, spans)
		if state != StateRustNormal {
			t.Errorf("%s: end state = %d, want %d", tc.line, state, StateRustNormal)
		}
		start := strings.Index(tc.line, tc.want)
		found := false
		for _, s := range spans {
			if s.StartCol == start && s.EndCol == start+len(tc.want) && s.Color == str {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: no string span covers exactly %s; spans = %+v", tc.line, tc.want, spans)
		}
	}
}

// 编辑器缓存的状态只是不透明的整数：外来取值 (负数、极大值、恰好落在原生字符串区间的数)
// 既不能让高亮器 panic，非负的前置状态也不能得到负的结束状态。
func TestRustHighlighter_RawStringStateToleratesForeignValues(t *testing.T) {
	h := NewRustHighlighter()
	for _, prev := range []int{-1, -1 << 30, 0, 1, 7, 1 << 30, math.MaxInt} {
		for _, line := range []string{"", `"`, `"#`, `"#####`, `x`, `r##"`, `"#; let x = 1;`} {
			runes := []rune(line)
			spans, state := h.HighlightLine(0, runes, prev)
			assertSpansWellFormed(t, runes, spans)
			if prev >= 0 && state < 0 {
				t.Fatalf("prev %d, line %q: negative end state %d", prev, line, state)
			}
		}
	}
}

// 回归：标识符后跟 '<' 时向右找匹配的 '>' 来判定泛型调用，比较运算符链里每个 '<'
// 都要扫到行尾才罢休，16 000 个比较 (28 万字符) 要 1.36 s，20 万字符 1.2–3.1 s。
func TestRustHighlighter_GenericLookaheadStaysBoundedOnComparisons(t *testing.T) {
	h := NewRustHighlighter()

	for _, unit := range []string{"a<b && ", "a<b,c<d,e<f", "a<<b>>c<", "a<a<a<"} {
		line := p2LongLine(unit)
		t.Logf("%q x%d: %v", unit, len([]rune(line))/len(unit), p2HighlightTimed(t, h, strconv.Quote(unit), line))
	}

	// 比较运算中的标识符不是函数 (g(h) 才是)
	for _, tc := range []struct {
		line   string
		idents []string
	}{
		{`if a < b && c < d || e < f { g(h); }`, []string{"a", "c", "e"}},
		{`while i<n && j>=0 && k != l { n += 1; }`, []string{"i", "k"}},
		{`let v = a < b, w = c < d;`, []string{"a", "w"}},
	} {
		runes := []rune(tc.line)
		spans, state := h.HighlightLine(0, runes, 0)
		assertSpansWellFormed(t, runes, spans)
		if state != StateRustNormal {
			t.Fatalf("%s: end state = %d, want %d", tc.line, state, StateRustNormal)
		}
		for _, id := range tc.idents {
			if s, ok := spanAt(spans, strings.Index(tc.line, id)); ok && s.Color == h.colors.Function {
				t.Fatalf("%s: %q in a comparison coloured as a function call", tc.line, id)
			}
		}
	}

	// 泛型调用 (含嵌套与很长的实参列表) 照常识别为函数
	isFunc := func(line string, idents ...string) {
		t.Helper()
		runes := []rune(line)
		spans, _ := h.HighlightLine(0, runes, 0)
		for _, id := range idents {
			col := strings.Index(line, id)
			if s, ok := spanAt(spans, col); !ok || s.Color != h.colors.Function || string(runes[s.StartCol:s.EndCol]) != id {
				t.Fatalf("%s: %q not coloured as a generic call: %+v", line, id, s)
			}
		}
	}
	isFunc(`let n = parse<Vec<u8>>(s);`, "parse")
	isFunc(`let m = foo<HashMap<String, Vec<(u8, i64)>>>(x);`, "foo")
	isFunc(`fn get_data<'a>(id: i32) -> Option<String> { None }`, "get_data")
	isFunc("call<"+strings.Repeat("Type, ", 80)+"Last>(arg)", "call") // 约 480 字符的实参列表，仍在窥探窗口内
	// 同一行前面堆满比较运算之后，后面的泛型调用仍然识别
	isFunc(strings.Repeat("a<b && ", 3000)+"f<T>(x);", "f")
}

func BenchmarkRustHighlighter_ComparisonChain200k(b *testing.B) {
	h := NewRustHighlighter()
	line := []rune(p2LongLine("a<b && "))
	b.ReportAllocs()
	b.SetBytes(int64(len(line)))
	for i := 0; i < b.N; i++ {
		h.HighlightLine(0, line, 0)
	}
}
