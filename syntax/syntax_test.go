package syntax

import (
	"testing"

	"github.com/jacksalad/gnote/theme"
)

func TestJSONHighlighter(t *testing.T) {
	h := NewJSONHighlighter()

	// 1. 测试基础 Key-Value
	line := []rune(`  "title": "goui", "count": 42, "active": true, "extra": null`)
	spans, state := h.HighlightLine(0, line, StateNormal)
	if state != StateNormal {
		t.Fatalf("expected StateNormal, got %d", state)
	}
	if len(spans) == 0 {
		t.Fatalf("expected spans, got 0")
	}

	// 验证包含 Key, String, Number, Boolean, Null 颜色
	hasKey := false
	hasString := false
	hasNumber := false
	hasBool := false
	hasNull := false

	for _, s := range spans {
		if s.Color == h.colors.Key {
			hasKey = true
		}
		if s.Color == h.colors.String {
			hasString = true
		}
		if s.Color == h.colors.Number {
			hasNumber = true
		}
		if s.Color == h.colors.Boolean {
			hasBool = true
		}
		if s.Color == h.colors.Null {
			hasNull = true
		}
	}

	if !hasKey || !hasString || !hasNumber || !hasBool || !hasNull {
		t.Fatalf("JSON tokens missing: hasKey=%v, hasString=%v, hasNumber=%v, hasBool=%v, hasNull=%v",
			hasKey, hasString, hasNumber, hasBool, hasNull)
	}

	// 2. 测试转义字符
	lineEsc := []rune(`  "msg": "hello \"world\" \u4e2d\u6587"`)
	spansEsc, _ := h.HighlightLine(1, lineEsc, StateNormal)
	hasEscape := false
	for _, s := range spansEsc {
		if s.Color == h.colors.Escape {
			hasEscape = true
			break
		}
	}
	if !hasEscape {
		t.Fatalf("expected escape color in JSON string")
	}

	// 3. 测试主题切换
	h.ApplyTheme(theme.Light())
	if h.isDark {
		t.Fatalf("expected light theme after ApplyTheme(ThemeLight)")
	}
}

func TestYAMLHighlighter(t *testing.T) {
	h := NewYAMLHighlighter()

	// 1. 文档分割
	spansDoc, _ := h.HighlightLine(0, []rune("--- # 开始"), StateYAMLNormal)
	if len(spansDoc) < 2 || spansDoc[0].Color != h.colors.Directive {
		t.Fatalf("expected directive for ---")
	}

	// 2. 键值与标量
	line := []rune("server: 127.0.0.1")
	spans, _ := h.HighlightLine(1, line, StateYAMLNormal)
	if len(spans) < 3 {
		t.Fatalf("expected key, colon, and string spans, got %d", len(spans))
	}
	if spans[0].Color != h.colors.Key {
		t.Fatalf("expected Key color for 'server'")
	}
	if spans[1].Color != h.colors.Punctuation {
		t.Fatalf("expected Punctuation for ':'")
	}

	// 3. 锚点与别名
	lineAnchor := []rune("  tag: &prod_env production")
	spansAnchor, _ := h.HighlightLine(2, lineAnchor, StateYAMLNormal)
	hasAnchor := false
	for _, s := range spansAnchor {
		if s.Color == h.colors.Anchor {
			hasAnchor = true
			break
		}
	}
	if !hasAnchor {
		t.Fatalf("expected Anchor color for &prod_env")
	}

	// 4. 块标量跨行测试 (|)
	lineBlockHeader := []rune("description: |")
	_, stateBlock := h.HighlightLine(3, lineBlockHeader, StateYAMLNormal)
	if stateBlock < StateYAMLBlockScalarBase {
		t.Fatalf("expected BlockScalar state, got %d", stateBlock)
	}

	// 下一行内容 (缩进 > base)
	lineBlockContent := []rune("  line 1 content")
	spansContent, stateCont := h.HighlightLine(4, lineBlockContent, stateBlock)
	if stateCont != stateBlock {
		t.Fatalf("expected continued BlockScalar state, got %d", stateCont)
	}
	if len(spansContent) != 1 || spansContent[0].Color != h.colors.String {
		t.Fatalf("expected string color for block scalar content")
	}

	// 缩进缩回，结束块标量
	lineNextKey := []rune("port: 8080")
	spansNext, stateNext := h.HighlightLine(5, lineNextKey, stateBlock)
	if stateNext != StateYAMLNormal {
		t.Fatalf("expected normal state after unindent, got %d", stateNext)
	}
	if len(spansNext) < 2 || spansNext[0].Color != h.colors.Key {
		t.Fatalf("expected key on unindented line")
	}
}

func TestTOMLHighlighter(t *testing.T) {
	h := NewTOMLHighlighter()

	// 1. 表头测试
	lineTable := []rune("[database.primary]")
	spansTable, stateTable := h.HighlightLine(0, lineTable, StateTOMLNormal)
	if stateTable != StateTOMLNormal {
		t.Fatalf("expected normal state, got %d", stateTable)
	}
	hasTable := false
	for _, s := range spansTable {
		if s.Color == h.colors.Table {
			hasTable = true
			break
		}
	}
	if !hasTable {
		t.Fatalf("expected Table color in [database.primary]")
	}

	// 2. 数组表头 [[bin]]
	lineArr := []rune("[[bin]] # 二进制配置")
	spansArr, _ := h.HighlightLine(1, lineArr, StateTOMLNormal)
	hasArrTable := false
	hasArrComment := false
	for _, s := range spansArr {
		if s.Color == h.colors.Table {
			hasArrTable = true
		}
		if s.Color == h.colors.Comment {
			hasArrComment = true
		}
	}
	if !hasArrTable || !hasArrComment {
		t.Fatalf("expected array table and comment, got %v, %v", hasArrTable, hasArrComment)
	}

	// 3. 赋值语句 (Key = Value, Number, Boolean, DateTime)
	lineKV := []rune(`host = "localhost"`)
	spansKV, _ := h.HighlightLine(2, lineKV, StateTOMLNormal)
	if len(spansKV) < 3 || spansKV[0].Color != h.colors.Key || spansKV[1].Color != h.colors.Punctuation {
		t.Fatalf("expected key and equals sign in TOML KV")
	}

	// 4. 多行基本字符串 """
	lineMLStart := []rune(`multiline = """`)
	_, stateML := h.HighlightLine(3, lineMLStart, StateTOMLNormal)
	if stateML != StateTOMLMLBasicString {
		t.Fatalf("expected StateTOMLMLBasicString, got %d", stateML)
	}

	lineMLMid := []rune(`  multi line content`)
	_, stateMLMid := h.HighlightLine(4, lineMLMid, stateML)
	if stateMLMid != StateTOMLMLBasicString {
		t.Fatalf("expected StateTOMLMLBasicString continued, got %d", stateMLMid)
	}

	lineMLEnd := []rune(`""" # 闭合`)
	spansMLEnd, stateMLEnd := h.HighlightLine(5, lineMLEnd, stateMLMid)
	if stateMLEnd != StateTOMLNormal {
		t.Fatalf("expected StateTOMLNormal after closing, got %d", stateMLEnd)
	}
	if len(spansMLEnd) == 0 || spansMLEnd[0].Color != h.colors.String {
		t.Fatalf("expected string color on closing line")
	}
}

func TestDetectHighlighter(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"main.go", "Go"},
		{"script.py", "Python"},
		{"worker.pyw", "Python"},
		{"main.c", "C/C++"},
		{"header.h", "C/C++"},
		{"app.cpp", "C/C++"},
		{"Program.cs", "C#"},
		{"script.csx", "C#"},
		{"App.java", "Java"},
		{"main.rs", "Rust"},
		{"analysis.r", "R"},
		{"script.R", "R"},
		{"index.js", "JavaScript"},
		{"module.mjs", "JavaScript"},
		{"bundle.cjs", "JavaScript"},
		{"app.ts", "TypeScript"},
		{"page.tsx", "TypeScript"},
		{"setup.typoscript", "TypeScript"},
		{"index.html", "HTML"},
		{"about.htm", "HTML"},
		{"style.css", "CSS"},
		{"query.sql", "SQL"},
		{"migration.dsql", "SQL"},
		{"README.md", "Markdown"},
		{"doc.markdown", "Markdown"},
		{"config.json", "JSON"},
		{"settings.yaml", "YAML"},
		{"deploy.yml", "YAML"},
		{"Cargo.toml", "TOML"},
		{"unknown.txt", "纯文本"},
		{"noext", "纯文本"},
	}

	for _, tc := range cases {
		h := DetectHighlighter(tc.input)
		name := GetLanguageName(h)
		if name != tc.expected {
			t.Errorf("DetectHighlighter(%q) = %q, want %q", tc.input, name, tc.expected)
		}
	}
}
