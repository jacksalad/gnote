package format

import (
	"math/rand"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// 回归测试：格式化器第二轮审计（D3，以及 D6 的 JS 同类问题）
// ---------------------------------------------------------------------------

func mustFormatJS(t *testing.T, src string) string {
	t.Helper()
	out, err := formatJS(src, "")
	if err != nil {
		t.Fatalf("formatJS(%q): %v", src, err)
	}
	return out
}

// assertNoLineBreakRemoved 用分词器本身核对 ASI 语义没有被改掉：
// 输出保留源码的全部 Token，并且源码里出现过的换行在输出里仍然是换行。
// （格式化器允许在 ; { } 之后新增换行，所以只检查“不删除”这一个方向。）
func assertNoLineBreakRemoved(t *testing.T, src, out string) {
	t.Helper()
	a, err := tokenizeJS(src)
	if err != nil {
		t.Fatalf("tokenizeJS(%q): %v", src, err)
	}
	b, err := tokenizeJS(out)
	if err != nil {
		t.Fatalf("output does not tokenize (%v):\n out=%q", err, out)
	}
	if len(a) != len(b) {
		t.Errorf("token count changed: %d -> %d\n src=%q\n out=%q", len(a), len(b), src, out)
		return
	}
	for i := range a {
		if a[i].text != b[i].text {
			t.Errorf("token %d changed: %q -> %q\n src=%q\n out=%q", i, a[i].text, b[i].text, src, out)
			return
		}
		if a[i].newlinesBefore > 0 && b[i].newlinesBefore == 0 {
			t.Errorf("line break before token %d (%q) was removed:\n src=%q\n out=%q", i, a[i].text, src, out)
			return
		}
	}
}

// assertJSOutput 断言输出恰好为 want 且输出幂等（不检查换行，用于续行规则会并入上一行的写法）。
func assertJSOutput(t *testing.T, src, want string) {
	t.Helper()
	out := mustFormatJS(t, src)
	if out != want {
		t.Errorf("unexpected output:\n src =%q\n got =%q\n want=%q", src, out, want)
	}
	if two, err := formatJS(out, ""); err != nil || two != out {
		t.Errorf("not idempotent:\n src=%q\n one=%q\n two=%q err=%v", src, out, two, err)
	}
}

// assertJSFormatted 断言输出恰好为 want、ASI 换行未被删除，且输出幂等。
func assertJSFormatted(t *testing.T, src, want string) {
	t.Helper()
	out := mustFormatJS(t, src)
	if out != want {
		t.Errorf("unexpected output:\n src =%q\n got =%q\n want=%q", src, out, want)
	}
	assertNoLineBreakRemoved(t, src, out)
	if two, err := formatJS(out, ""); err != nil || two != out {
		t.Errorf("not idempotent:\n src=%q\n one=%q\n two=%q err=%v", src, out, two, err)
	}
}

// D3：能结束语句的 '}' 之后，行首的 '/' 是正则字面量。以前下列输入里的正则被
// 拆成两个除号（/re/ -> / re /），模式从 "re" 变成了 " re "，程序含义随之改变。
// 这些输入在 Node (V8) 与 bun (JavaScriptCore) 中都能正常解析并按正则求值。
func TestFormatJS_RegexAfterStatementBlocks(t *testing.T) {
	for _, src := range []string{
		// catch 省略参数绑定时 '{' 之前是 catch 而不是 ')'
		"try {\n  a()\n} catch {\n  b()\n}\n/re/.test(s)\n",
		"try {\n  a()\n} catch (e) {\n  b()\n}\n/re/.test(s)\n",
		// '{' 之前夹着注释时，语法上下文应当跳过注释往前看
		"if (a) /* c */ {\n  b()\n}\n/re/.test(s)\n",
		"function f() /* body */ {\n  b()\n}\n/re/.test(s)\n",
		// class 主体、static 块
		"class A {}\n/re/.test(s)\n",
		"class A extends B {\n  m() {}\n}\n/re/.test(s)\n",
		"class A extends mixin(B) {\n  m() {}\n}\n/re/.test(s)\n",
		"class A {\n  get x() { return 1 }\n  set y(v) { return v }\n  static m() {}\n}\n/re/.test(s)\n",
		"class A {\n  static {\n    init()\n  }\n}\n/re/.test(A.name)\n",
		"class A {} /re/.test(s)",
		// 标签语句 / case 子句后的块
		"lab: { }\n/re/.test(s)",
		"switch (x) {\n  case 1: {\n    let y = 1\n  }\n  /re/.test(y)\n}",
		// 对照：这些本来就正确，不得回退
		"function f() {}\n/re/.test(s)\n",
		"const f = () => {}\n/re/.test(s)\n",
		"if (a) {\n  b()\n}\n/re/.test(s)\n",
		"if (a) /re/.test(s)\n",
	} {
		out := mustFormatJS(t, src)
		if !strings.Contains(out, "/re/") || strings.Contains(out, "/ re /") {
			t.Errorf("regex literal was rewritten as division:\n src=%q\n got=%q", src, out)
		}
		assertNoLineBreakRemoved(t, src, out)
		if two, err := formatJS(out, ""); err != nil || two != out {
			t.Errorf("not idempotent:\n src=%q\n one=%q\n two=%q err=%v", src, out, two, err)
		}
	}
	assertJSFormatted(t, "try {\n  a()\n} catch {\n  b()\n}\n/re/.test(s)\n",
		"try {\n    a()\n} catch {\n    b()\n}\n/re/.test(s)")
	// 审计列出的三种有效写法（同一个语句块，注释位置不同）：'/' 之前的有效 Token
	// 是块的 '}'，引擎按正则读取；以前 catch { 被当成对象字面量、注释隔断了上下文。
	assertJSFormatted(t, "try { f(); } catch { g(); } /re/.test(s)",
		"try {\n    f();\n} catch {\n    g();\n}\n/re/.test(s)")
	assertJSFormatted(t, "try {\n  f();\n} catch {\n  g();\n}\n/re/.test(s)\n",
		"try {\n    f();\n} catch {\n    g();\n}\n/re/.test(s)")
	assertJSFormatted(t, "if (a) /* c */ { f(); } /re/.test(s)",
		"if (a) /* c */ {\n    f();\n}\n/re/.test(s)")
	assertJSFormatted(t, "if (a) // c\n{ f(); } /re/.test(s)",
		"if (a) // c\n{\n    f();\n}\n/re/.test(s)")
}

// D3：'in' / 'of' 等前缀关键字只有在真正的关键字位置才算数。属性访问 x.in 里的 in、
// 普通标识符 of 后面的 '/' 是除号；以前它被当成正则起点，把行尾的 // 注释拆成了
// “除号 除号 注释”，输出直接变成语法错误（Invalid regular expression: missing /）。
func TestFormatJS_PrefixKeywordMistakenForIdentifier(t *testing.T) {
	cases := []struct{ src, want string }{
		{"const r = x.in / 2 // comment\n", "const r = x.in / 2 // comment"},
		{"const r = x.default / 2 // comment\n", "const r = x.default / 2 // comment"},
		{"const r = x?.in / 2 // comment\n", "const r = x?.in / 2 // comment"},
		{"const r = a.b().in / 2 // comment\n", "const r = a.b().in / 2 // comment"},
		{"const of = 2\nx = of / 4 // c\n", "const of = 2\nx = of / 4 // c"},
		{"let of = 6\nlet q = of / 2 // half\n", "let of = 6\nlet q = of / 2 // half"},
		// 真正的 for-of 头部里 of 仍然是关键字
		{"for (const v of /a b/.exec(s)) {\n  log(v)\n}\n", "for (const v of /a b/.exec(s)) {\n    log(v)\n}"},
		{"for (x of /a/.exec(s)) x = x / 2 // half\n", "for (x of /a/.exec(s)) x = x / 2 // half"},
		{"for (const [a] of /b/.exec(s)) {\n  use(a)\n}\n", "for (const[a] of /b/.exec(s)) {\n    use(a)\n}"},
		// 对照：关键字位置上的行为不变
		{"if (k in o) {\n  o[k] = 1\n}\n", "if (k in o) {\n    o[k] = 1\n}"},
		{"switch (x) {\n  case 1 / 2:\n    break\n}\n", "switch (x) {\n    case 1 / 2:\n    break\n}"},
		{"x = typeof y / 2 // half\n", "x = typeof y / 2 // half"},
		{"x = 'a' in o // has\n", "x = 'a' in o // has"},
		{"x = { in: 1 / 2 }\n", "x = {\n    in: 1 / 2\n}"},
		{"const { in: i2, default: d2 } = x\nconst y = i2 / d2 // ratio\n",
			"const {\n    in: i2,\n    default: d2\n} = x\nconst y = i2 / d2 // ratio"},
	}
	for _, tc := range cases {
		assertJSFormatted(t, tc.src, tc.want)
	}
}

// D3 的审计复现：换行之后、前一个 Token 能结束表达式时出现的 '/'，如
//
//	foo()\n/re/.test(s)
//
// 引擎只看 '/' 之前的 Token 区分除号与正则，不做 ASI 回溯，所以这类位置上的 '/'
// 默认是除号。但只要“除法读法”在语法上不成立——候选字面量不带修饰位，且第二个
// '/' 之后开不了操作数（".x"、"?."、")" "}"、EOF 等）——它就只可能是作者写的正则，
// 排版器不能把它拆成两个除号：/re/ -> / re / 改写的是正则本身的内容。
// 判定依据是 ECMAScript 词法规则，并在 Node (V8) 与 bun (JavaScriptCore) 上核对过。
func TestFormatJS_RegexAtLineStartAfterExpression(t *testing.T) {
	for _, src := range []string{
		"foo()\n/re/.test(s)\n",
		"const a = 1\n/re/.test(a)\n",
		"arr[0]\n/re/.test(s)\n",
		"const o = {a: 1}\n/re/.test(o)\n",
		"const t = `x`\n/re/.test(t)\n",
		"let i = 1\ni++\n/re/.test(i)\n",
		"foo() // note\n/re/.test(s)\n",
		"foo()\n/* c */ /re/.test(s)\n",
		"foo()\n/re/?.test(s)\n",
		"function w() {\n  foo()\n  /re/\n}\n",
	} {
		out := mustFormatJS(t, src)
		if !strings.Contains(out, "/re/") || strings.Contains(out, "/ re /") {
			t.Errorf("regex literal was rewritten as division:\n src=%q\n got=%q", src, out)
		}
		assertNoLineBreakRemoved(t, src, out)
		if two, err := formatJS(out, ""); err != nil || two != out {
			t.Errorf("not idempotent:\n src=%q\n one=%q\n two=%q err=%v", src, out, two, err)
		}
	}
	// 成员访问的 '.' 跨行时按既有的续行规则并入上一行（'.' 之前的换行没有语义）；
	// '/' 之前的换行保留，字面量逐字输出。
	assertJSOutput(t, "foo()\n/re/\n.test(s)\n", "foo()\n/re/.test(s)")
	// 审计复现的精确输出：正则字面量逐字保留。
	assertJSFormatted(t, "foo()\n/re/.test(s)\n", "foo()\n/re/.test(s)")
	assertJSFormatted(t, "const a = 1\n/re/.test(a)", "const a = 1\n/re/.test(a)")
	// 经由 HTML 的同一条路径（<script> 内联脚本）。
	assertFormatted(t, "<script>\nfoo()\n/re/.test(s)\n</script>",
		"<script>\n    foo()\n    /re/.test(s)\n</script>")
}

// 行首的 '/' 并不总是正则：除法读法成立时它就是除号，必须保持原样。
// foo()\n/re/g.test(s) 是合法代码，引擎读作 foo() / re / g.test(s)
// （ReferenceError: re is not defined）；x = y\n/ 2 // comment 里的 '//' 是注释。
// 一律把行首 '/' 当正则会把这些合法代码改写成另一个程序（或语法错误）。
func TestFormatJS_DivisionAfterExpressionStaysDivision(t *testing.T) {
	cases := []struct{ src, want string }{
		{"foo()\n/re/g.test(s)\n", "foo()\n/ re / g.test(s)"},
		{"const a = 1\n/re/g.test(a)\n", "const a = 1\n/ re / g.test(a)"},
		{"a = b\n/ c / d\n", "a = b\n/ c / d"},
		{"x = y\n/ 2 // comment\n", "x = y\n/ 2 // comment"},
		{"x = y\n/ 2 /* c */\n", "x = y\n/ 2 /* c */"},
		{"let v = a\n/ b\n/ c\n", "let v = a\n/ b\n/ c"},
		{"foo()\n/re/\nfoo()\n", "foo()\n/ re /\nfoo()"},
		{"foo()\n/re/ + 1\n", "foo()\n/ re / + 1"},
		{"a = b\n/ (c) / d\n", "a = b\n/ (c) / d"},
		{"a = b\n/ c /.5\n", "a = b\n/ c / .5"},
		{"a = b\n/ c /* x */ / d\n", "a = b\n/ c /* x */ / d"},
		{"a = b\n/ /re/.source\n", "a = b\n/ /re/.source"},
	}
	for _, tc := range cases {
		assertJSFormatted(t, tc.src, tc.want)
	}
}

// 既有的 ASI 防护不得回退：换行本身就是语义的一部分，源码里的换行一个都不能少。
func TestFormatJS_ASIGuardsStayIntact(t *testing.T) {
	for _, src := range []string{
		"function f(){return\n1}",
		"const a = b\n(function () {})",
		"const a = b\n[1, 2].forEach(f)",
		"const a = b\n`x`.trim()",
		"a = b\n++c",
		"a = b\n--c",
		"if (x) {\n  a()\n}\n-b\n",
		"if (x) {\n  a()\n}\nfrom = 10\n",
		"function f() {}\nof = 1\n",
		"try {\n  a()\n} catch (e) {\n  b()\n}\nas = 1\n",
		"const x = 1\n+ 2",
		"const s = `a   `\nfoo()\n",
		"const o = {a: 1}\n[1].map(f)",
		"foo()\n(function () {})",
		"x = y\n`x`\n",
	} {
		out := mustFormatJS(t, src)
		assertNoLineBreakRemoved(t, src, out)
	}
	assertJSFormatted(t, "function f(){return\n1}", "function f() {\n    return\n    1\n}")
}

// 仓库外的模糊测试发现的幂等性缺口：一元 '!' / '~' 与其后的 '=' 之间的空格被去掉后，
// 两个 Token 合并成 "!=" / "~="，再格式化一次 Token 数就变了。合法脚本里不会出现
// 这种相邻关系，但被接受的输出仍必须是定点。
func TestFormatJS_AdjacentTokensDoNotMerge(t *testing.T) {
	assertJSFormatted(t, "0! =000000", "0 ! = 000000")
	assertJSFormatted(t, "a ~ = b", "a ~ = b")
	// 同类问题：两个正负号相邻时会被并成 "++" / "--"，(+ +x) 变成 (++x)。
	assertJSFormatted(t, "const y = f(+ +x) + [+ -x] - -x", "const y = f(+ +x) + [+ -x] - -x")
	// 对照：一元运算符后面照常不留空格；二元加减两侧的空格规则不变。
	assertJSFormatted(t, "const ok = !a", "const ok = !a")
	assertJSFormatted(t, "const n = ~a", "const n = ~a")
	assertJSFormatted(t, "const x=-1;const y=a + +b;const z=i++ + j;", "const x = -1;\nconst y = a + +b;\nconst z = i++ + j;")
}

// D6（JS 与 HTML 同类问题）：超深嵌套的缩进同样封顶，避免输出体积随深度平方增长。
func TestFormatJS_DeepNestingIndentIsCapped(t *testing.T) {
	const n = 4000
	src := strings.Repeat("{", n) + strings.Repeat("}", n)
	out, err := formatJS(src, "")
	if err != nil {
		t.Fatalf("formatJS: %v", err)
	}
	// 2n 行，每行 = 缩进（封顶后 ≤ maxIndentDepth 个单位）+ 括号 + 换行。
	bound := 2 * n * (len(jsIndent)*maxIndentDepth + 2)
	if len(out) > bound {
		t.Errorf("output is %d bytes, linear bound is %d bytes", len(out), bound)
	}
	assertNoLineBreakRemoved(t, src, out)
	if two, err := formatJS(out, ""); err != nil || two != out {
		t.Errorf("not idempotent (err=%v, second pass %d bytes)", err, len(two))
	}
}

// 畸形输入：不 panic、不死循环；能格式化的必须幂等。
func TestFormatJS_MalformedInputNeverPanics(t *testing.T) {
	for _, src := range []string{
		"", " ", "//", "/*", "`", "${", "`${", "`${`", "`${}`", "'\\", "\"\\", "/[", "/\\", "a\n-->", "-->x", "<!--",
		"#!/x", "#{", "@", "1.", "1..", ".5.5", "a??b", "a?.5:b", "\\u0061", "\x00", "\xff", "a\x00b", "a\xffb",
		"x = `a${`${b}`}`;", "{", "(", "[", "}", ")", "]", "a /* b */ / c", "return\n", "a\n++\n++b",
		"/* c */", "/* c */ {", "/* c */ /", "// c\n/", "x.in", "x.in /", "of /", "for (of /", "class {",
		"class", "class A", "class A extends", "case", "case 1:", "a:", "a: {", "catch", "try { } catch",
		"try { } catch { } /", "x = { a: {", "x = { a: {} } /", "switch (x) { case 1: { } /", "/re/ /re/",
		strings.Repeat("{", 3000), strings.Repeat("/* c */ {", 2000), strings.Repeat("x.in / ", 2000),
	} {
		src := src
		t.Run("", func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic on %q: %v", src, r)
				}
			}()
			out, err := formatJS(src, "")
			if err != nil {
				return
			}
			if two, err2 := formatJS(out, ""); err2 != nil || two != out {
				t.Errorf("not idempotent:\n src=%q\n one=%q\n two=%q err=%v", src, out, two, err2)
			}
		})
	}
}

// 固定种子的随机输入：分词器的除号/正则判定与注释回看逻辑不得 panic 或死循环。
func TestFormatJS_RandomInputNeverPanics(t *testing.T) {
	pieces := []string{
		" ", "\n", "\t", "a", "x", "of", "in", "class", "extends", "static", "case", "default", "return", "else", "do",
		"catch", "try", "if", "for", "(", ")", "{", "}", "[", "]", ";", ":", ",", ".", "?.", "=", "?", "=>", "/", "// c",
		"/* c */", "/re/", "/ re /", "1", "'s'", "`t`", "++", "<!--", "-->", "\\", "\x00",
	}
	rng := rand.New(rand.NewSource(20261126))
	for i := 0; i < 6000; i++ {
		var sb strings.Builder
		for n := 1 + rng.Intn(24); n > 0; n-- {
			sb.WriteString(pieces[rng.Intn(len(pieces))])
		}
		src := sb.String()
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic on %q: %v", src, r)
				}
			}()
			out, err := formatJS(src, "")
			if err != nil {
				return
			}
			if two, err2 := formatJS(out, ""); err2 != nil || two != out {
				t.Errorf("not idempotent:\n src=%q\n one=%q\n two=%q err=%v", src, out, two, err2)
			}
		}()
	}
}
