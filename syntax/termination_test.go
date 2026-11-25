package syntax

import (
	"strings"
	"testing"
	"time"
)

// 单行高亮在 UI 线程同步执行，高亮器一旦不前进，窗口就永久未响应。
// 回归测试用看门狗快速失败，而不是等 go test 的整体超时；
// 卡住的 goroutine 会一直空转到测试进程退出。
const highlightTimeout = 3 * time.Second

// highlightMustReturn 断言一次高亮在看门狗时限内返回。
func highlightMustReturn(t *testing.T, h Highlighter, line string, prev int) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = h.HighlightLine(0, []rune(line), prev)
	}()
	select {
	case <-done:
	case <-time.After(highlightTimeout):
		t.Fatalf("%T.HighlightLine(%q, state %d) did not return within %v", h, line, prev, highlightTimeout)
	}
}

// assertSpansWellFormed 校验片段非零长、按起点有序、互不重叠且不越界。
func assertSpansWellFormed(t *testing.T, line []rune, spans []Span) {
	t.Helper()
	last := 0
	for i, s := range spans {
		if s.StartCol < last || s.EndCol <= s.StartCol || s.EndCol > len(line) {
			t.Fatalf("span %d = [%d,%d) invalid for a %d-rune line (previous end %d): %q",
				i, s.StartCol, s.EndCol, len(line), last, string(line))
		}
		last = s.EndCol
	}
}

// 回归：标签外的 '<' 后面不是标签名、'/' 或 '!' 时（a < b、行尾的 '<'、内联脚本里的
// i < n），旧实现没有任何分支消费它，正文扫描停在原地死循环。
// 在 HTML 文档里键入第一个 '<' 即冻结，含比较运算的文件一渲染就冻结。
func TestHTMLHighlighter_StrayAngleBracketTerminates(t *testing.T) {
	h := NewHTMLHighlighter()
	for _, line := range []string{
		"<",
		"1 <",
		"a < b",
		"x <= y",
		"if (x<10) {",
		`<p>5 < 6</p>`,
		`<script>for (var i = 0; i < n; i++) { a[i] = 1; }</script>`,
	} {
		runes := []rune(line)
		for _, state := range []int{StateHTMLNormal, StateHTMLTag} {
			highlightMustReturn(t, h, line, state)
			spans, _ := h.HighlightLine(0, runes, state)
			assertSpansWellFormed(t, runes, spans)
		}
		_, end := h.HighlightLine(0, runes, StateHTMLNormal)
		if end != StateHTMLNormal {
			t.Fatalf("%q: end state = %d, want %d", line, end, StateHTMLNormal)
		}
	}

	// 游离的 '<' 按正文着色，两侧文字照常成段
	line := []rune("a < b")
	spans, _ := h.HighlightLine(0, line, StateHTMLNormal)
	covered := 0
	for _, s := range spans {
		if s.Color != h.colors.Text {
			t.Fatalf("stray '<' line must stay plain text, got span [%d,%d) in color %v", s.StartCol, s.EndCol, s.Color)
		}
		covered += s.EndCol - s.StartCol
	}
	if covered != len(line) {
		t.Fatalf("plain-text spans cover %d of %d runes", covered, len(line))
	}
}

// 回归：'@' 被当作标识符起始，但续扫循环不接受它，i 停在原地死循环。
// 任何 T-SQL / MySQL 脚本（DECLARE @n、@@VERSION）一被高亮即冻结。
func TestSQLHighlighter_AtSignTerminates(t *testing.T) {
	h := NewSQLHighlighter()
	for _, line := range []string{
		"@",
		"SET @x = 1;",
		"DECLARE @n INT",
		"SELECT @@VERSION",
		"SELECT * FROM users WHERE id = @id",
		"/* header */ SET @y = 2",
		"me@example.com",
	} {
		runes := []rune(line)
		highlightMustReturn(t, h, line, StateSQLNormal)
		spans, state := h.HighlightLine(0, runes, StateSQLNormal)
		assertSpansWellFormed(t, runes, spans)
		if state != StateSQLNormal {
			t.Fatalf("%q: end state = %d, want %d", line, state, StateSQLNormal)
		}
	}

	// 变量名被整体消费且不着色，前后的关键字照常识别
	line := []rune("SELECT @id @@VERSION FROM t")
	spans, _ := h.HighlightLine(0, line, StateSQLNormal)
	var tokens []string
	for _, s := range spans {
		tok := string(line[s.StartCol:s.EndCol])
		if strings.Contains(tok, "@") {
			t.Fatalf("variable %q must not be coloured as a keyword, got %v", tok, s.Color)
		}
		tokens = append(tokens, tok)
	}
	if got, want := strings.Join(tokens, "|"), "SELECT|FROM"; got != want {
		t.Fatalf("coloured tokens = %q, want %q", got, want)
	}
}

// 回归：引用块路径以非零默认颜色进入 parseInlines，落单的行内标记字符
// （> a * b、> see [1、GitHub 的 > [!NOTE]）让聚合循环停在原地死循环。
func TestMarkdownHighlighter_BlockquoteUnmatchedMarkerTerminates(t *testing.T) {
	h := NewMarkdownHighlighter()
	for _, line := range []string{
		">*",
		"> *",
		"> a * b",
		"> see [1",
		"> todo `x",
		"> snake_case",
		"> x < y",
		"> ~x",
		"> !img",
		"> [!NOTE]",
		"> <b>bold</b>",
		"> mixed [a *b `c _d <e ~f !g",
	} {
		runes := []rune(line)
		highlightMustReturn(t, h, line, StateNormal)
		spans, state := h.HighlightLine(0, runes, StateNormal)
		assertSpansWellFormed(t, runes, spans)
		if state != StateNormal {
			t.Fatalf("%q: end state = %d, want %d", line, state, StateNormal)
		}
	}

	// 落单的标记按引用正文着色：引用标记之后的每一列都有片段覆盖
	line := []rune("> a * b")
	spans, _ := h.HighlightLine(0, line, StateNormal)
	if len(spans) == 0 || spans[0].Color != h.colors.QuoteMarker {
		t.Fatalf("blockquote marker not highlighted: %+v", spans)
	}
	covered := make([]bool, len(line))
	for _, s := range spans {
		for c := s.StartCol; c < s.EndCol; c++ {
			covered[c] = true
		}
	}
	for c := spans[0].EndCol; c < len(line); c++ {
		if !covered[c] {
			t.Fatalf("column %d (%q) of a blockquote line left without a span", c, line[c])
		}
	}

	// 配对完整的行内语法仍按原规则识别，不被兜底分支吞掉
	line = []rune("> a *b* `c` [d](u)")
	spans, _ = h.HighlightLine(0, line, StateNormal)
	if s, ok := spanAt(spans, 5); !ok || s.Color != h.colors.ItalicText {
		t.Fatalf("italic inside a blockquote no longer recognised: %+v", s)
	}
	if s, ok := spanAt(spans, 9); !ok || s.Color != h.colors.InlineCodeText {
		t.Fatalf("inline code inside a blockquote no longer recognised: %+v", s)
	}
	if s, ok := spanAt(spans, 13); !ok || s.Color != h.colors.LinkText {
		t.Fatalf("link inside a blockquote no longer recognised: %+v", s)
	}
}

// 全部高亮器对曾让其中三种语言卡死的那批输入都必须终止，
// 防止同类“未匹配字符不前进”的缺陷换一种语言复现。
func TestAllHighlighters_TerminateOnFormerHangInputs(t *testing.T) {
	lines := []string{
		"<", "a < b", "if (x<10) {", "@", "DECLARE @n INT", "SELECT @@VERSION",
		">*", "> a * b", "> see [1", "> todo `x", "> [!NOTE]", "> x < y", "> ~x",
		"", " ", "\r", "\\", "<!", "<!--", "]]", "}}", "${", "```", "---", "#@", "%",
	}
	for _, h := range allHighlighters() {
		for _, line := range lines {
			for state := 0; state < 3; state++ {
				highlightMustReturn(t, h, line, state)
			}
		}
	}
}
