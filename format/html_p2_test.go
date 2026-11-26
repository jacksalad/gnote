package format

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// 基准：大体量文档的格式化耗时（UI 线程上同步执行，必须保持线性）
// ---------------------------------------------------------------------------

var (
	benchLargeOnce sync.Once
	benchLargeSrc  string
	benchRowsOnce  sync.Once
	benchRowsSrc   string
	benchJSONOnce  sync.Once
	benchJSONSrc   string
)

// benchFormatHTMLLarge 生成约 6.2 MB、结构良好的页面：块级嵌套、行内元素、
// 原始文本元素（pre/textarea/svg）与一段内联 CSS/JS，覆盖格式化器的全部分支。
// 输入惰性生成一次，避免普通 go test 也付出构造开销。
func benchFormatHTMLLarge() string {
	benchLargeOnce.Do(func() {
		var sb strings.Builder
		sb.Grow(7 << 20)
		sb.WriteString("<!DOCTYPE html>\n<html lang=\"zh-CN\">\n<head>\n<meta charset=\"utf-8\">\n<title>Report</title>\n")
		sb.WriteString("<style>\n/* base */\nhtml{box-sizing:border-box}\n*,*::before,*::after{box-sizing:inherit;margin:0}\n")
		sb.WriteString(".row{display:flex;gap:8px;padding:4px 8px}\n@media (min-width:600px){.row{width:calc(100% - 2*var(--gap))}}\n</style>\n")
		sb.WriteString("<script>\nwindow.addEventListener('load', function () {\n  var items = document.querySelectorAll('.row')\n")
		sb.WriteString("  var total = items.length\n  var ratio = total / 2\n  console.log(total, ratio)\n})\n</script>\n")
		sb.WriteString("</head>\n<body>\n<main>\n")
		for i := 0; sb.Len() < 6<<20; i++ {
			n := strconv.Itoa(i)
			sb.WriteString(`<section class="row" data-id="` + n + `"><h3>Item ` + n + ` &amp; notes</h3>`)
			sb.WriteString(`<p>Hello <b>world</b> <a href="/item/` + n + `">link</a> `)
			sb.WriteString(`<button type="button"><svg class="bi" viewBox="0 0 16 16"><path d="M1 1"/></svg>Go</button></p>`)
			sb.WriteString("<pre>  a\n    b</pre><textarea rows=\"2\">\nfoo\n</textarea>")
			sb.WriteString("<span>a</span><!-- c --><span>b</span></section>\n")
		}
		sb.WriteString("</main>\n</body>\n</html>\n")
		benchLargeSrc = sb.String()
	})
	return benchLargeSrc
}

// benchFormatHTMLRows 是审计使用过的“纯块级 + 行内元素”页面，用于对照审计基线。
func benchFormatHTMLRows(n int) string {
	benchRowsOnce.Do(func() {
		var sb strings.Builder
		for i := 0; i < n; i++ {
			sb.WriteString("<div class=\"row\"><p>Hello <b>world</b> &amp; friends</p><span>x</span></div>\n")
		}
		benchRowsSrc = sb.String()
	})
	return benchRowsSrc
}

// benchFormatJSONBig 生成约 50 MB 的 JSON 数组。
func benchFormatJSONBig(b *testing.B) string {
	b.Helper()
	benchJSONOnce.Do(func() {
		var sb strings.Builder
		sb.Grow(52 << 20)
		for i := 0; sb.Len() < 50<<20; i++ {
			fmt.Fprintf(&sb, "{\"id\":%d,\"name\":\"user%d\",\"tags\":[\"a\",\"b\"],\"score\":%d.25},", i, i, i)
		}
		benchJSONSrc = "[" + strings.TrimSuffix(sb.String(), ",") + "]"
	})
	return benchJSONSrc
}

// BenchmarkFormatHTML_Large 约 6.2 MB 混合页面。
func BenchmarkFormatHTML_Large(b *testing.B) {
	src := benchFormatHTMLLarge()
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := FormatHTML(src); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFormatHTML_Rows 约 6.2 MB 纯块级行（审计基线场景）。
func BenchmarkFormatHTML_Rows(b *testing.B) {
	src := benchFormatHTMLRows(80000)
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := FormatHTML(src); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFormatJSON_Big 约 50 MB JSON。
func BenchmarkFormatJSON_Big(b *testing.B) {
	src := benchFormatJSONBig(b)
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := FormatJSON(src); err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// 回归测试：格式化器第二轮审计（D1/D2/D4/D5/D6）
// ---------------------------------------------------------------------------

// stripASCIIWhitespace 只删除 ASCII 空白。格式化器只被允许改动标签之间的
// ASCII 空白与缩进，因此“去掉 ASCII 空白后逐字节相等”是内容未被改写的必要条件。
func stripASCIIWhitespace(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\r', '\f', '\v':
			return -1
		}
		return r
	}, s)
}

func mustFormatHTML(t *testing.T, src string) string {
	t.Helper()
	out, err := FormatHTML(src)
	if err != nil {
		t.Fatalf("FormatHTML(%q): %v", src, err)
	}
	return out
}

// assertFormatted 断言输出恰好为 want、内容只改动了 ASCII 空白，且输出幂等。
func assertFormatted(t *testing.T, src, want string) {
	t.Helper()
	out := mustFormatHTML(t, src)
	if out != want {
		t.Errorf("unexpected output:\n src =%q\n got =%q\n want=%q", src, out, want)
	}
	// 标签名大小写不敏感：原始文本元素的闭合标签会被规范成小写。
	if a, b := stripASCIIWhitespace(src), stripASCIIWhitespace(out); !strings.EqualFold(a, b) {
		t.Errorf("content changed beyond ASCII whitespace:\n src=%q\n got=%q", src, out)
	}
	if two, err := FormatHTML(out); err != nil || two != out {
		t.Errorf("not idempotent:\n src=%q\n one=%q\n two=%q err=%v", src, out, two, err)
	}
}

// assertUnchanged 断言格式化前后文本一致。
func assertUnchanged(t *testing.T, src string) {
	t.Helper()
	assertFormatted(t, src, src)
}

// D1：HTML 的可折叠空白只有 TAB/LF/FF/CR/空格。NBSP 等非 ASCII 空白是可见内容，
// 既不能与相邻空白合并，也不能在行首/行尾被裁掉。
func TestFormatHTML_NonASCIIWhitespaceIsContent(t *testing.T) {
	for _, src := range []string{
		"<p>a\u00A0\u00A0b</p>",
		"<div>\u00A0</div>",
		"<p>10\u00A0km</p>",
		"<p>foo \u00A0 bar</p>",
		"<p><b>a</b>\u00A0<i>b</i></p>",
		"<p>a\u00A0\u00A0b</p>\n<div>\u00A0</div>\n<p>10\u00A0km</p>",
		"<p>10\u0085km</p>",             // NEL：unicode.IsSpace 为真，但不是 HTML 空白
		"<p>a\u2003b\u3000c\u202Fd</p>", // em 空格 / 表意空格 / 窄 NBSP
		// 对照：实体、属性值与 pre 内部本来就逐字保留
		"<p>a&nbsp;b</p>",
		"<p title=\"a\u00A0b\">x</p>",
		"<pre>a\u00A0\u00A0b</pre>",
	} {
		assertUnchanged(t, src)
	}

	// ASCII 空白照常折叠；紧邻的 NBSP 保持原样。块级标签两侧的单个空格按既有行为保留
	// （渲染时落在行首/行尾，会被折叠）。
	assertFormatted(t, "<p>foo  \u00A0  bar</p>", "<p>foo \u00A0 bar</p>")
	assertFormatted(t, "<div>\n  a  \u00A0  b\n</div>", "<div> a \u00A0 b </div>")
	assertFormatted(t, "<div>\n  \u00A0text\n</div>", "<div> \u00A0text </div>")
	assertFormatted(t, "<p>foo\u00A0  </p>", "<p>foo\u00A0 </p>")
	// 只含 NBSP 的行不是空行，不能被丢弃。
	assertFormatted(t, "<div>\n\u00A0\n</div>", "<div> \u00A0 </div>")
	assertUnchanged(t, "\u00A0<!-- c --><span>b</span>")
	assertFormatted(t, "<table><tr><td>\u00A0</td><td>x</td></tr></table>",
		"<table>\n    <tr>\n        <td>\u00A0</td>\n        <td>x</td>\n    </tr>\n</table>")
	// 块级元素之间的 NBSP 在浏览器里会撑出一个匿名行块，必须保留。
	assertFormatted(t, "<div>a</div>\u00A0<div>b</div>", "<div>a</div>\n\u00A0\n<div>b</div>")
	// </pre\u00A0> 不是合法的结束标签（浏览器把它当作 pre 的文本），pre 延伸到真正的 </pre>。
	assertUnchanged(t, "<pre>a</pre\u00A0>b  c</pre>")
}

// D4：未加引号的属性值里的引号只是普通字符。按 HTML 分词规则，引号只有紧跟在
// '='（及可选空白）之后才开启“引号属性值”状态。
func TestFormatHTML_QuoteInsideUnquotedAttributeValue(t *testing.T) {
	// 审计复现：以前 'panic>' 之后整个文档被当成一个文本 Token，换行被压平、
	// <pre> 内容被破坏、脚本里的 // 注释吞掉了下一行代码。
	assertFormatted(t,
		"<div class=box title=Don't panic>x</div>\n<p>a   b</p>\n<pre>  keep\n    me</pre>\n<script>\n  var a = 1  // init\n  doThing(a)\n</script>\n",
		"<div class=box title=Don't panic>x</div>\n<p>a b</p>\n<pre>  keep\n    me</pre>\n<script>\n    var a = 1 // init\n    doThing(a)\n</script>\n")

	assertUnchanged(t, "<div title=it's>text</div>")
	assertFormatted(t, "<span>a</span> <div title=it's>x</div><p>it's ok</p><b>b</b>",
		"<span>a</span>\n<div title=it's>x</div>\n<p>it's ok</p>\n<b>b</b>")
	// 标签内的空白折叠同样只作用于引号值之外。
	assertFormatted(t, "<div title=it's   data-a='x   y'>t</div>", "<div title=it's data-a='x   y'>t</div>")
	assertFormatted(t, "<div title=\"it's   x\"   data-b=\"a   b\">t</div>",
		`<div title="it's   x" data-b="a   b">t</div>`)

	// 对照：引号值里的 '>' 仍然不算标签结束。
	assertUnchanged(t, `<a title="a > b" data-x='c > d'>link</a>`)
	assertUnchanged(t, `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-strict.dtd">`)
	assertUnchanged(t, "<![CDATA[ x > \"y\" ]]>")
	// 无引号属性值里的 '/' 是值的一部分，'>' 才结束标签。
	assertUnchanged(t, `<div data-x=a>b>x</div>`)
	assertUnchanged(t, `<a href=/>x</a>`)
	assertFormatted(t, "<br />", "<br/>")
	assertFormatted(t, "<p   title = \"x\"  >a</p>", `<p title = "x">a</p>`)
}

// D4：到文档末尾都没有 '>' 的标签无法可靠定界（浏览器会丢弃它），必须报错而不是猜测。
func TestFormatHTML_UnterminatedTagIsRejected(t *testing.T) {
	for _, tc := range []struct {
		src      string
		position string
	}{
		{"<p>a</p>\n<div class=box", "第 2 行第 1 列"},
		{"<p>ok</p><span", "第 1 行第 10 列"},
		{"<a b=\"x>", "第 1 行第 1 列"},
		{"<p>a</p>\n</div", "第 2 行第 1 列"},
		{"I <3\n你", "第 1 行第 3 列"},
	} {
		_, err := FormatHTML(tc.src)
		if !errors.Is(err, errInvalidHTML) {
			t.Errorf("FormatHTML(%q) error = %v, want errInvalidHTML", tc.src, err)
			continue
		}
		if !strings.Contains(err.Error(), tc.position) {
			t.Errorf("FormatHTML(%q) error = %q, want position %s", tc.src, err, tc.position)
		}
	}

	// 这些“不完整”输入并不是未闭合标签，必须继续可格式化（与既有测试的约束一致）。
	for _, src := range []string{
		"<div><span>oops", "</div></span>", "<!-- unterminated", "a < b and c > d",
		"<script>", "<", "<!", "<?", "<p>a</p>\n<!", "<p>a</p>\n<?php echo 1;",
	} {
		if _, err := FormatHTML(src); err != nil {
			t.Errorf("FormatHTML(%q): unexpected error %v", src, err)
		}
	}
}

// D2：行内级原始元素（svg/math/textarea）与注释紧贴行内内容时必须留在同一行。
// 在这里插入换行会多出一个可折叠空格：图标与文字之间、inline-block 兄弟之间出现间隙。
// 渲染后果是按 CSS 空白规则推断的，没有在浏览器里实测。
func TestFormatHTML_InlineRawElementsStayGlued(t *testing.T) {
	for _, src := range []string{
		`<button type="button"><svg viewBox="0 0 16 16"><path d="M1 1"/></svg>Warning</button>`,
		`<label>Delay <textarea></textarea><span class="hint">ms</span></label>`,
		`<span>a</span><!-- c --><span>b</span>`,
		`<a href="/"><svg class="logo" viewBox="0 0 10 10"><circle r="5"/></svg>Home</a>`,
		`<a href="#"><svg viewBox="0 0 10 10"><circle r="5"/></svg><span>Next</span></a>`,
		`<p>Value <math><mi>x</mi></math>cm</p>`,
		`<b>Save<svg viewBox="0 0 10 10"><circle r="5"/></svg></b>`,
		`<div><svg viewBox="0 0 10 10"><circle r="5"/></svg>x</div>`,
		"<label>Delay<textarea>\nfoo\n</textarea>ms</label>",
		"<span>a</span><!-- c\n  --><span>b</span>",
		// 连续多个紧贴的注释：逐个共用同一次向后扫描的结论
		"<span>a</span><!--1--><!--2--><!--3--><span>b</span>",
		"<b>x</b><!--\n--><!--\n--><textarea></textarea>y",
	} {
		assertUnchanged(t, src)
	}
	// 多行的 textarea 同样内联。
	assertUnchanged(t, "<div><textarea>\nfoo\n</textarea>x</div>")
}

// D2：块级边界上、或源码本身就有空白的位置，独占一行的既有布局保持不变。
func TestFormatHTML_InlineRawElementsStillBreakAtSafePositions(t *testing.T) {
	assertFormatted(t, `<div><svg viewBox="0 0 10 10"><text> a  b </text></svg></div>`,
		"<div>\n    <svg viewBox=\"0 0 10 10\"><text> a  b </text></svg>\n</div>")
	assertFormatted(t, `<p>Click the <svg viewBox="0 0 10 10"><circle r="5"/></svg> icon to continue</p>`,
		"<p>Click the\n    <svg viewBox=\"0 0 10 10\"><circle r=\"5\"/></svg>\n    icon to continue\n</p>")
	assertFormatted(t, `<p>Comment: <textarea></textarea></p>`,
		"<p>Comment:\n    <textarea></textarea>\n</p>")
	// 注释本身不渲染：两侧有一侧不是紧贴的行内内容时，独占一行不会引入可见空白。
	assertFormatted(t, "<ul class=\"tags\">\n  <li>One</li><!--\n  --><li>Two</li><!--\n  --><li>Three</li>\n</ul>",
		"<ul class=\"tags\">\n    <li>One</li>\n    <!--\n    -->\n    <li>Two</li>\n    <!--\n    -->\n    <li>Three</li>\n</ul>")
	assertFormatted(t, `<!DOCTYPE html><html><head><title>T</title></head><body></body></html>`,
		"<!DOCTYPE html>\n<html>\n    <head>\n        <title>T</title>\n    </head>\n    <body></body>\n</html>")
	// 连续的注释后面是块级元素：每个注释都可以独占一行。
	assertFormatted(t, "<div>a</div><!--1--><!--2--><div>b</div>",
		"<div>a</div>\n<!--1-->\n<!--2-->\n<div>b</div>")
}

// textarea/svg/math 的闭合标签不能被重新缩进：缩进空白会进入表单值或 SVG 文本节点。
// <pre> 的既有行为（独占一行的闭合标签对齐到当前层级）由现有测试固定，保持不变。
func TestFormatHTML_RawElementClosingTagNotReindented(t *testing.T) {
	assertFormatted(t, "<div><textarea>\nfoo\n</textarea></div>",
		"<div>\n    <textarea>\nfoo\n</textarea>\n</div>")
	assertUnchanged(t, "<textarea>\nfoo\n</textarea>")
	assertFormatted(t, "<div>\n<svg viewBox=\"0 0 10 10\">\n<path d=\"M1 1\"/>\n</svg>\n</div>",
		"<div>\n    <svg viewBox=\"0 0 10 10\">\n<path d=\"M1 1\"/>\n</svg>\n</div>")
	assertUnchanged(t, "<p>a</p>\n<math>\n<mi>x</mi>\n</math>\n<p>b</p>")
}

// D5：xmp/listing/plaintext 是原始文本元素；内联 style 声明了 white-space:pre*
// 的元素，其内部的空白与换行都有渲染意义，必须整体逐字保留。
func TestFormatHTML_PreformattedElementsKeptVerbatim(t *testing.T) {
	for _, src := range []string{
		"<div style=\"white-space:pre\">a\n    b</div>",
		"<div class=\"msg\" style=\"white-space:pre-wrap\">line one\nline   two</div>",
		"<p>x<span style=\"white-space: pre-line\">a  b</span>y</p>",
		"<div style=\"white-space:pre\">a\n  <div>b  c</div>\n</div>",
		"<div style=\"white-space:pre\">a  <span style=\"white-space:pre\">b   c</span> d</div>",
		"<p>a</p>\n<div style=\"white-space:pre-wrap\">b  c\n  d",
		"<div style=\"WHITE-SPACE: PRE !important\">a  b</div>",
		"<div style=\"color:red;white-space:break-spaces\">a  b</div>",
		"<xmp>  a\n    b</xmp>",
		"<listing>  x\n y</listing>",
		"<plaintext>  a\n   <b>c</b>",
		"<p>before</p>\n<plaintext><div>x  y</div>\n  z",
	} {
		assertUnchanged(t, src)
	}

	assertFormatted(t, "<section>\n<div style=\"white-space: pre-line\">a  b\n c</div>\n</section>",
		"<section>\n    <div style=\"white-space: pre-line\">a  b\n c</div>\n</section>")
	assertFormatted(t, "<div>\n<xmp>  a\n    b</xmp>\n</div>",
		"<div>\n    <xmp>  a\n    b</xmp>\n</div>")
	// 显式结束标签之后的兄弟元素照常排版。
	assertFormatted(t, "<table><tr><td style=\"white-space:pre-wrap\">a  b</td><td>c</td></tr></table>",
		"<table>\n    <tr>\n        <td style=\"white-space:pre-wrap\">a  b</td>\n        <td>c</td>\n    </tr>\n</table>")
	assertFormatted(t, "<ul><li>a<li style=\"white-space:pre\">b  c</li><li>d</li></ul>",
		"<ul>\n    <li>a\n    <li style=\"white-space:pre\">b  c</li>\n    <li>d</li>\n</ul>")
	// 省略了结束标签：同级开始标签之后的空白仍属于这个单元格，排版器在那里换行或缩进
	// 都会写进它的内容，所以范围延续到文档末尾（或真正的结束标签），其后不再排版。
	assertFormatted(t, "<table><tr><td style=\"white-space:pre-wrap\">a  b<td>c</td></tr></table>",
		"<table>\n    <tr>\n        <td style=\"white-space:pre-wrap\">a  b<td>c</td></tr></table>")
	assertFormatted(t, "<p>a <p style=\"white-space:pre\">b  c</p><p>d</p>",
		"<p>a\n<p style=\"white-space:pre\">b  c</p>\n<p>d</p>")
	// 未命中 pre 系列的写法不受影响。
	assertFormatted(t, "<div style=\"white-space:normal\">a\n    b</div>", "<div style=\"white-space:normal\">a b</div>")
	assertFormatted(t, "<div style=\"white-space-x:pre;color:red\">a\n    b</div>", "<div style=\"white-space-x:pre;color:red\">a b</div>")
	assertFormatted(t, `<br style="white-space:pre">`, `<br style="white-space:pre">`)
}

// 浏览器只对 void 元素承认自闭合写法：<pre/>、<div/> 上的斜杠会被忽略，元素照常打开。
func TestFormatHTML_OnlyVoidElementsAreSelfClosing(t *testing.T) {
	// 以前 <pre/> 被当成自闭合，后面的 pre 文本被当成普通文本折叠。
	assertUnchanged(t, "<pre/>  keep   me\n  more</pre>")
	assertUnchanged(t, "<div/>x</div>")
	assertFormatted(t, "<div/><p>a</p></div>", "<div/>\n    <p>a</p>\n</div>")
	assertFormatted(t, "<ul><li/>a</ul>", "<ul>\n    <li/>a\n</ul>")
	assertUnchanged(t, `<br/><img src="x.png"/>`)
}

// D6：未闭合的块级标签不得让输出体积随嵌套深度平方增长（UI 线程上同步执行）。
func TestFormatHTML_UnclosedTagsStayLinear(t *testing.T) {
	for _, n := range []int{2000, 4000, 8000} {
		src := strings.Repeat("<div>", n)
		out := mustFormatHTML(t, src)
		// 每行 = 缩进（封顶后 ≤ maxIndentDepth 个单位）+ "<div>" + 换行。
		bound := n * (len(htmlIndent)*maxIndentDepth + len("<div>") + 1)
		if len(out) > bound {
			t.Errorf("%d unclosed <div>: output is %d bytes, linear bound is %d bytes", n, len(out), bound)
		}
		assertFormatted(t, src, out)
	}
}

// D6：合法的超深嵌套同样封顶缩进，且输出幂等。
func TestFormatHTML_DeepNestingIndentIsCapped(t *testing.T) {
	const depth = 200
	src := strings.Repeat("<div>", depth) + "x" + strings.Repeat("</div>", depth)
	out := mustFormatHTML(t, src)
	cap := len(htmlIndent) * maxIndentDepth
	for i, ln := range strings.Split(out, "\n") {
		if lead := len(ln) - len(strings.TrimLeft(ln, " ")); lead > cap {
			t.Errorf("line %d: indent of %d spaces exceeds the cap of %d", i+1, lead, cap)
		}
	}
	assertFormatted(t, src, out)
}

// D6（CSS 与 HTML 同类）：规则嵌套超深时缩进同样封顶。
func TestFormatCSS_DeepNestingIndentIsCapped(t *testing.T) {
	const n = 4000
	out, err := formatCSS(strings.Repeat("a{", n)+strings.Repeat("}", n), "")
	if err != nil {
		t.Fatalf("formatCSS: %v", err)
	}
	bound := 2 * n * (len(cssIndent)*maxIndentDepth + 4)
	if len(out) > bound {
		t.Errorf("output is %d bytes, linear bound is %d bytes", len(out), bound)
	}
	if two, err := formatCSS(out, ""); err != nil || two != out {
		t.Errorf("not idempotent (err=%v, second pass %d bytes)", err, len(two))
	}
}

// 未闭合的 CSS 字符串或块注释：排版器会给语句补分号，每格式化一次就多出一个 ';'
// （"0/*" -> "0/*;" -> "0/*;;"）。这种输入无法安全排版，必须报错，由调用方保留原文。
func TestFormatCSS_UnterminatedStringOrCommentIsRejected(t *testing.T) {
	for _, src := range []string{
		`"0`, "a{content:\"abc", "a{content:'x", "a{content:\"a\\", `a{content:"a;b`,
		"0/*", "/*", "a{/*", "a{color:red}/*",
	} {
		if out, err := formatCSS(src, ""); err == nil {
			t.Errorf("formatCSS(%q) = %q, want an error", src, out)
		}
	}
	// 对照：闭合的字符串（含转义引号与分号）与注释照常排版；<style> 整体保留原文。
	// （CSS 排版器会补上语句末尾的分号，所以这里只比较精确输出。）
	if got, want := mustFormatHTML(t, "<style>/* base */\na{color:red}</style>"),
		"<style>\n    /* base */\n    a {\n        color: red;\n    }\n</style>"; got != want {
		t.Errorf("closed CSS comment:\n got %q\nwant %q", got, want)
	}
	if got, want := mustFormatCSS(t, `a{content:"a\";b"}`), "a {\n    content: \"a\\\";b\";\n}"; got != want {
		t.Errorf("closed CSS string:\n got %q\nwant %q", got, want)
	}
	assertUnchanged(t, "<style>a{content:\"abc</style>")
	assertUnchanged(t, "<style>/* x</style>")
}

// 仓库外的模糊测试发现：<script>/<style> 开始标签里夹着无效的 UTF-8 字节时，
// splitRawElement 用 strings.ToLower 之后的下标去切原字符串（无效字节被替换成
// U+FFFD，长度变了），直接 panic（slice bounds out of range）。
func TestFormatHTML_InvalidUTF8InTagDoesNotPanic(t *testing.T) {
	// 审计到的 panic 输入：开始标签吞掉了后面的闭合标签，原样保留即可。
	assertUnchanged(t, "<sCript \x98\xf4\xf5\xf2\xb0</sCript>")
	// 脚本/样式内容里的无效字节按普通字符处理：不 panic、幂等，内联 JS/CSS 照常排版。
	for _, src := range []string{
		"<div>\xff<script>\xfevar a = 1\xfd</script></div>",
		"<style>\xffa{color:red}</style>",
	} {
		out := mustFormatHTML(t, src)
		if two, err := FormatHTML(out); err != nil || two != out {
			t.Errorf("not idempotent:\n src=%q\n one=%q\n two=%q err=%v", src, out, two, err)
		}
	}
}

// 选择器列表里有空选择器（"0,{}"）时，CSS 排版器以前输出 "0,\n}"——既不是合法 CSS，
// 再格式化一次还会报错。这种输入必须直接报错，由调用方保留原文。
func TestFormatCSS_EmptySelectorGroupIsRejected(t *testing.T) {
	for _, src := range []string{"0,{}", "a,,b{}", ",a{}", "a,{}", "a,\n{}"} {
		if out, err := formatCSS(src, ""); err == nil {
			t.Errorf("formatCSS(%q) = %q, want an error", src, out)
		}
	}
	// 对照：正常的选择器列表照常排版。
	if got, want := mustFormatCSS(t, "a,b{}"), "a,\nb {\n}"; got != want {
		t.Errorf("formatCSS(\"a,b{}\") = %q, want %q", got, want)
	}
	if got, want := mustFormatCSS(t, "@media print{a,b{color:red}}"),
		"@media print {\n    a,\n    b {\n        color: red;\n    }\n}"; got != want {
		t.Errorf("nested selector list = %q, want %q", got, want)
	}
}

func mustFormatCSS(t *testing.T, src string) string {
	t.Helper()
	out, err := formatCSS(src, "")
	if err != nil {
		t.Fatalf("formatCSS(%q): %v", src, err)
	}
	return out
}

// 仓库外的模糊测试发现的幂等性缺口（畸形输入被接受后，输出必须是定点）：
// "/ >" 里的空白第一遍折叠成 " />"，第二遍又变成 "/>"。
func TestFormatHTML_MalformedOutputIsAFixedPoint(t *testing.T) {
	assertFormatted(t, "<Aaaaa00!! / >000000000", "<Aaaaa00!!/>000000000")
	assertFormatted(t, "<br / >", "<br/>")
	assertFormatted(t, "<br />", "<br/>")
	// 引号值里的 "/ >" 是内容，不受影响。
	assertUnchanged(t, `<a href="x / > y">t</a>`)
}

// 畸形输入：不 panic、不死循环。能格式化的必须幂等，且除 ASCII 空白与标签名大小写外不改写内容。
func TestFormatHTML_MalformedInputNeverPanics(t *testing.T) {
	for _, src := range []string{
		"", "<", "<!", "<!-", "<!--", "<!--x", "<!-->", "</", "</ ", "</div", "<?", "<a", "<a ", "<a b='", "<a b=\"",
		"<a b=\"x>", "<![CDATA[", "<![CDATA[x", "<script", "<script>", "<script></", "<script></script", "<script>a</script/>b",
		"<pre", "<pre>", "<pre></pre ", "<pre/>", "<xmp>", "<listing>", "<plaintext>", "<p", "</p", "a<", "a</",
		"\x00", "<\x00>", "<a\x00b>", "<div title=x/>", "<STYLE>a{</STYLE>", "<sCrIpT>x</SCRIPT>", "<pre>a</PRE >",
		"<div class=box title=Don't", "<div title=Don't panic>", "<a href='x>", "<a href=\"x>", "<div title=it's   class='a'>",
		"<textarea>", "<textarea>foo  ", "<textarea>foo\n", "<textarea>foo\n\n", "<svg><g>", "<math><mi>x",
		"<div style=\"white-space:pre>", "<div style=white-space:pre>a  b", "<p style='white-space:pre'>x  y",
		"<div style=\"white-space:pre\">a</div style>", "</div style=\"white-space:pre\">", "<p>\u00A0", "\u00A0",
		"<!-- c --><span>b", "<span>a</span><!-- c", "<!DOCTYPE", "<!doctype html", "<p>a</p><!", "<p>a</p><?",
		strings.Repeat("<div>", 50), strings.Repeat("</div>", 50), strings.Repeat("<span>", 50) + "x",
		strings.Repeat("<!-- c --><span>b</span>", 30), strings.Repeat("<svg/>", 40),
		// 标签里的无效 UTF-8 字节；只有 CR 的行尾（首行的全部 CR 都要去掉，输出才是定点）
		"<sCript \x98\xf4\xf5\xf2\xb0</sCript>", "<script>\xff</script>", "<style>\xc3</style>", "\xff<a>", "<a \xff>",
		"<!--\r\r", "<!--\r", "<pre>\r\rx</pre>", "<textarea>\r\rx", "\r\r<div>a</div>",
	} {
		src := src
		t.Run("", func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic on %q: %v", src, r)
				}
			}()
			out, err := FormatHTML(src)
			if err != nil {
				return
			}
			two, err2 := FormatHTML(out)
			if err2 != nil || two != out {
				t.Errorf("not idempotent:\n src=%q\n one=%q\n two=%q err=%v", src, out, two, err2)
			}
			if a, b := looseContent(src), looseContent(out); !strings.EqualFold(a, b) {
				t.Errorf("content changed beyond ASCII whitespace:\n src=%q\n got=%q", src, out)
			}
		})
	}
}

// looseContent 去掉 ASCII 空白与分号后再比较：内联 CSS 排版器会给声明补上分号。
func looseContent(s string) string {
	return strings.ReplaceAll(stripASCIIWhitespace(s), ";", "")
}

// 固定种子的随机畸形输入：与上面的清单互补，覆盖分词器/排版器的状态组合。
func TestFormatHTML_RandomMalformedInput(t *testing.T) {
	pieces := []string{
		"<", ">", "</", "/>", "<!--", "-->", "<!", "<!'", "<?", "\"", "'", "=", " ", "\n", "\t", "a",
		"div", "p", "pre", "script", "style", "textarea", "svg", "math", "xmp", "listing", "plaintext", "title",
		"style=", "white-space:pre", "type=", "\u00A0", "&amp;", "{", "}", "/*", "*/", "//", "`", "${", ":", "(", ")", ".",
	}
	rng := rand.New(rand.NewSource(20261126))
	for i := 0; i < 6000; i++ {
		var sb strings.Builder
		for n := 1 + rng.Intn(28); n > 0; n-- {
			sb.WriteString(pieces[rng.Intn(len(pieces))])
		}
		src := sb.String()
		if strings.TrimSpace(src) == "" {
			continue
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic on %q: %v", src, r)
				}
			}()
			out, err := FormatHTML(src)
			if err != nil {
				return
			}
			if a, b := looseContent(src), looseContent(out); !strings.EqualFold(a, b) {
				t.Errorf("content changed beyond ASCII whitespace:\n src=%q\n got=%q", src, out)
			}
			if two, err2 := FormatHTML(out); err2 != nil || two != out {
				t.Errorf("not idempotent:\n src=%q\n one=%q\n two=%q err=%v", src, out, two, err2)
			}
		}()
	}
}
