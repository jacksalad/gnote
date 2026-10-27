package syntax

import "testing"

// allHighlighters 返回全部 16 种语言的高亮器（用于模糊测试统一覆盖）
func allHighlighters() []Highlighter {
	return []Highlighter{
		NewGoHighlighter(),
		NewPythonHighlighter(),
		NewCHighlighter(),
		NewCSharpHighlighter(),
		NewJavaHighlighter(),
		NewRustHighlighter(),
		NewRHighlighter(),
		NewJSHighlighter(),
		NewTypeScriptHighlighter(),
		NewHTMLHighlighter(),
		NewCSSHighlighter(),
		NewSQLHighlighter(),
		NewMarkdownHighlighter(),
		NewJSONHighlighter(),
		NewYAMLHighlighter(),
		NewTOMLHighlighter(),
	}
}

// FuzzHighlightLine 校验所有语言分词器在任意输入下：
//   - 不 panic、不修改输入
//   - 结束状态非负
//   - span 边界合法、按起点有序且互不重叠
func FuzzHighlightLine(f *testing.F) {
	seeds := []string{
		"",
		"package main\nfunc main() {}",
		`{ "a": [1, 2, 3], "b": null }`,
		"key: value  # comment",
		"---\n- item\nanchor: &a *b\n",
		"[server]\nport = 8080\nwhen = 2024-01-02T03:04:05Z\n",
		`<div class="x">&amp;</div>`,
		"SELECT * FROM t WHERE a = 'x';",
		"# Title\n```go\ncode\n```\n",
		"a /* multi\n line */ b",
		"fn main<'a>() { let x = 1i64; }",
		"obj.foo <- bar %>% baz",
		"const x: number = 1; // c",
		"class A { public void f() {} }",
		`a = "\u4f60\U0001F600"`,
		"中文注释 // 尾随",
	}
	for _, s := range seeds {
		f.Add(s, uint8(0), 0)
	}

	f.Fuzz(func(t *testing.T, text string, langSel uint8, prev int) {
		if prev < 0 {
			prev = -prev
		}
		if prev > 400 {
			prev %= 401
		}
		line := []rune(text)
		saved := string(line)

		hs := allHighlighters()
		h := hs[int(langSel)%len(hs)]
		spans, state := h.HighlightLine(0, line, prev)

		if state < 0 {
			t.Fatalf("negative end state %d", state)
		}
		if string(line) != saved {
			t.Fatal("highlighter mutated its input")
		}

		lastEnd := 0
		for i, sp := range spans {
			if sp.StartCol < 0 || sp.EndCol < sp.StartCol || sp.EndCol > len(line) {
				t.Fatalf("invalid span %d: %+v (line len %d)", i, sp, len(line))
			}
			if i > 0 && sp.StartCol < lastEnd {
				t.Fatalf("overlapping/unsorted span %d: %+v (previous end %d)", i, sp, lastEnd)
			}
			lastEnd = sp.EndCol
		}
	})
}
