package syntax

import (
	"strings"
	"testing"

	"github.com/jacksalad/gnote/graphics"
)

// 回归：引号字符串的接续行若恰好以文档标记或指令开头 (text: "first line 的下一行是 --- second")，
// 高亮器先给出接续的 String 片段，又把行首的 --- 当作文档分隔符追加一段 Directive，两个片段重叠。
// 项目自带的 FuzzHighlightLine 几秒内就会报 “overlapping/unsorted span”。
// 接续行的行首只是字符串内容，不能再识别文档标记与指令。
func TestYAMLHighlighter_QuotedContinuationIsNotADocumentMarker(t *testing.T) {
	h := NewYAMLHighlighter()

	for _, tc := range []struct {
		name     string
		second   string
		strState int
	}{
		{"document start", `--- second"`, StateYAMLDoubleQuote},
		{"document end", `... second"`, StateYAMLDoubleQuote},
		{"directive", `%APPDATA% continues"`, StateYAMLDoubleQuote},
		{"indented document start", `   --- second"`, StateYAMLDoubleQuote},
		{"single-quoted", `--- second'`, StateYAMLSingleQuote},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first := `text: "first line`
			if tc.strState == StateYAMLSingleQuote {
				first = `text: 'first line`
			}
			spans, states := p2HighlightDoc(t, h, []string{first, tc.second})
			if states[0] != tc.strState {
				t.Fatalf("end state after %q = %d, want %d", first, states[0], tc.strState)
			}
			p2AssertWholeLine(t, tc.second, spans[1], h.colors.String)
			if states[1] != StateYAMLNormal {
				t.Fatalf("end state after the closing quote = %d, want %d", states[1], StateYAMLNormal)
			}

			// 模糊测试的调用方式：单行，前置状态直接传入
			runes := []rune(tc.second)
			s, state := h.HighlightLine(0, runes, tc.strState)
			assertSpansWellFormed(t, runes, s)
			if state != StateYAMLNormal {
				t.Fatalf("single call: end state = %d, want %d", state, StateYAMLNormal)
			}
			p2AssertWholeLine(t, tc.second, s, h.colors.String)
		})
	}

	// 接续的字符串在本行闭合之后，其后的内容照常解析
	second := `--- x" key: value`
	spans, states := p2HighlightDoc(t, h, []string{`text: "first line`, second})
	if states[1] != StateYAMLNormal {
		t.Fatalf("end state = %d, want %d", states[1], StateYAMLNormal)
	}
	if s, ok := spanAt(spans[1], strings.Index(second, "key")); !ok || s.Color != h.colors.Key {
		t.Fatalf("'key' after the closing quote = %+v, want key color", s)
	}

	// 不是接续行时，文档标记与指令照常识别
	for _, line := range []string{"---", "--- # start", "--- x", "...", "%YAML 1.2", "  --- x"} {
		runes := []rune(line)
		spans, _ := h.HighlightLine(0, runes, StateYAMLNormal)
		assertSpansWellFormed(t, runes, spans)
		if len(spans) == 0 || spans[0].Color != h.colors.Directive {
			t.Errorf("%q: spans = %+v, want a leading directive span", line, spans)
		}
	}
}

// 回归：每个普通标量都向右扫到行尾，看后面有没有映射冒号，然后只吃掉一个词。
// "m: " 后面跟 10 万个 "1 " 时，每个数字都把剩下的半行重扫一遍，共 11 s。
func TestYAMLHighlighter_ScalarRunsStayLinear(t *testing.T) {
	h := NewYAMLHighlighter()

	for _, word := range []string{"1 ", "true ", "null ", "~ ", "0x1f ", ".inf ", "word "} {
		t.Run(strings.TrimSpace(word), func(t *testing.T) {
			t.Logf("%d runes: %v", len("m: "+p2LongLine(word)), p2HighlightTimed(t, h, "numbers", "m: "+p2LongLine(word)))
		})
	}

	// 标量类别照常识别
	line := []rune("m: 1 true null ~ 0x1f 2.5e3 text")
	spans, state := h.HighlightLine(0, line, StateYAMLNormal)
	assertSpansWellFormed(t, line, spans)
	if state != StateYAMLNormal {
		t.Fatalf("end state = %d, want %d", state, StateYAMLNormal)
	}
	for _, tc := range []struct {
		text  string
		color graphics.Color
	}{
		{"1", h.colors.Number}, {"true", h.colors.Boolean}, {"null", h.colors.Null},
		{"~", h.colors.Null}, {"0x1f", h.colors.Number}, {"2.5e3", h.colors.Number},
		{"text", h.colors.String},
	} {
		s, ok := spanAt(spans, strings.Index(string(line), tc.text))
		if !ok || s.Color != tc.color || string(line[s.StartCol:s.EndCol]) != tc.text {
			t.Errorf("%q coloured %+v, want %08X", tc.text, s, uint32(tc.color))
		}
	}

	// 键名识别 (标量后面跟着映射冒号) 与各词条法不受影响
	for _, l := range []string{
		"name: value", "my key: value", "server: 127.0.0.1", "list: [1, 2, {a: b}]",
		"http://example.com/x", "key: value # comment", "title: C# 入门", "- item", "? key",
		`tag: &anchor text`, `ref: *alias`, `type: !!str text`, "text: |", "text: >-",
	} {
		runes := []rune(l)
		spans, _ := h.HighlightLine(0, runes, StateYAMLNormal)
		assertSpansWellFormed(t, runes, spans)
	}
}

// 空键 (如 “- : value”) 的键名片段起止相同，违反“片段非空”的约定。
func TestYAMLHighlighter_EmptyKeyHasNoZeroLengthSpan(t *testing.T) {
	h := NewYAMLHighlighter()
	for _, line := range []string{"- : value", "key: : v", "{: a}", "?: : b"} {
		runes := []rune(line)
		spans, _ := h.HighlightLine(0, runes, StateYAMLNormal)
		assertSpansWellFormed(t, runes, spans)
	}
}

func BenchmarkYAMLHighlighter_NumberRun100k(b *testing.B) {
	h := NewYAMLHighlighter()
	line := []rune("m: " + p2LongLine("1 "))
	b.ReportAllocs()
	b.SetBytes(int64(len(line)))
	for i := 0; i < b.N; i++ {
		h.HighlightLine(0, line, 0)
	}
}
