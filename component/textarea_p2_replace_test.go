package component

import (
	"strings"
	"testing"
	"time"

	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/render"
)

// assertNoBareCR 断言文档的任意一行都不含裸 CR（文档内部只以 \n 分行）
func assertNoBareCR(t *testing.T, label string, ta *TextArea) {
	t.Helper()
	for r := range ta.lineCount() {
		if runesContain(ta.line(r), '\r') {
			t.Errorf("%s: bare CR left in line %d: %q", label, r, string(ta.line(r)))
		}
	}
}

// 回归：ReplaceAll 曾直接使用 []rune(repl)，与 insertText 不同，不做换行归一。
// Windows 剪贴板给出的 CRLF 替换文本把裸 CR 写进文档，CRLF 文件因此被保存成 \r\r\n。
func TestTextArea_ReplaceAllNormalizesReplacementLineBreaks(t *testing.T) {
	for _, c := range []struct{ name, repl string }{
		{"CRLF", "x\r\ny"},
		{"CR", "x\ry"},
		{"LF control", "x\ny"},
	} {
		ta := newTestTextArea("a b a\n")
		if n := ta.ReplaceAll("a", c.repl, false); n != 2 {
			t.Fatalf("%s: replaced %d, want 2", c.name, n)
		}
		if got, want := ta.GetTextWithEOL("\r\n"), "x\r\ny b x\r\ny\r\n"; got != want {
			t.Errorf("%s: CRLF document is saved as %q, want %q", c.name, got, want)
		}
		if got := ta.lineCount(); got != 4 {
			t.Errorf("%s: lineCount = %d, want 4", c.name, got)
		}
		assertNoBareCR(t, c.name, ta)

		if !ta.Undo() || ta.GetTextWithEOL("\n") != "a b a\n" {
			t.Errorf("%s: undo did not restore the document: %q", c.name, ta.GetTextWithEOL("\n"))
		}
		if !ta.Redo() {
			t.Fatalf("%s: redo failed", c.name)
		}
		assertNoBareCR(t, c.name+" redo", ta)
	}
}

// 回归：ReplaceCurrentSelection 与 ReplaceAll 相同，替换文本里的 CRLF/CR 必须归一为 LF。
func TestTextArea_ReplaceCurrentSelectionNormalizesReplacementLineBreaks(t *testing.T) {
	for _, c := range []struct{ name, repl string }{
		{"CRLF", "x\r\ny"},
		{"CR", "x\ry"},
		{"LF control", "x\ny"},
	} {
		ta := newTestTextArea("a b\n")
		ta.SetSelection(0, 0, 0, 1)
		if !ta.ReplaceCurrentSelection("a", c.repl, false) {
			t.Fatalf("%s: replacement did not happen", c.name)
		}
		if got, want := ta.GetTextWithEOL("\r\n"), "x\r\ny b\r\n"; got != want {
			t.Errorf("%s: CRLF document is saved as %q, want %q", c.name, got, want)
		}
		if got := ta.lineCount(); got != 3 {
			t.Errorf("%s: lineCount = %d, want 3", c.name, got)
		}
		assertNoBareCR(t, c.name, ta)

		ta.Undo()
		ta.Redo()
		assertNoBareCR(t, c.name+" redo", ta)
	}
}

// 归一化不得带来堆分配：不含 CR 的替换路径保持 v1.0.2 实测的 19 次分配，
// 含 CRLF 的替换与同样走逐行路径的 LF 替换分配数相同。
func TestTextArea_ReplaceNormalizationAddsNoAllocs(t *testing.T) {
	doc := strings.Repeat("a b a\n", 200)
	ta := newTestTextArea(doc)
	ta.ReplaceAll("a", "XY", false) // 预热
	allocs := testing.AllocsPerRun(5, func() {
		ta.SetText(doc)
		ta.ReplaceAll("a", "XY", false)
	})
	if allocs > 19 {
		t.Fatalf("ReplaceAll without CR allocated %.0f times, want <= 19 (the v1.0.2 baseline)", allocs)
	}

	crlf := testing.AllocsPerRun(5, func() { ta.ReplaceAll("a", "x\r\ny", false); ta.Undo() })
	lf := testing.AllocsPerRun(5, func() { ta.ReplaceAll("a", "x\ny", false); ta.Undo() })
	if crlf > lf {
		t.Fatalf("CRLF replacement allocated %.0f times, LF replacement %.0f times", crlf, lf)
	}
}

// 回归：ReplaceAll 的批量路径直接调用 replaceAt，绕过了 truncatePrefixForEdit，
// 被替换行的前缀宽度检查点随之失效。关闭折行时 2048 列以上的行在替换后
// 光标 X、选区与绘制偏移仍按旧宽度计算：光标画错位置，点它却落到别的列。
func TestTextArea_BulkReplaceAllInvalidatesLongLinePrefix(t *testing.T) {
	fm := render.GetFontManager()
	for _, c := range []struct {
		name, text, query, repl string
		cols                    []int
	}{
		{"edit from column 0", strings.Repeat("aX", 1500), "a", "中", []int{2048, 2304, 2560, 2999, 3000}},
		{"edit from column 2500", strings.Repeat("b", 2500) + strings.Repeat("aX", 300), "a", "中", []int{2048, 2560, 2700, 3099, 3100}},
	} {
		ta := p2EmbeddedEditor(t, 800, 400)
		ta.SetWordWrap(false)
		ta.SetFocused(true)
		ta.SetText(c.text)
		ta.Render(render.NewCanvas(800, 400)) // 可见超长行：渲染时构建宽度检查点
		if len(ta.longPrefixes) != 1 {
			t.Fatalf("%s: precondition: prefix checkpoints must be built", c.name)
		}

		if n := ta.ReplaceAll(c.query, c.repl, false); n == 0 {
			t.Fatalf("%s: nothing replaced", c.name)
		}
		check := func(step string) {
			t.Helper()
			line := ta.line(0)
			for _, col := range c.cols {
				if got, want := ta.measureStyledVisualX(0, 0, col), fm.MeasureRunesFace(ta.fontFace, line[:col]); got != want {
					t.Errorf("%s %s: col %d is at %d px, want %d px", c.name, step, col, got, want)
				}
			}
		}
		check("after ReplaceAll")

		// 光标停在第 2048 列：点击光标所在位置必须回到同一列
		ta.cursorRow, ta.cursorCol = 0, 2048
		ta.ScrollCursorIntoView()
		rect, ok := ta.CaretRect()
		if !ok {
			t.Fatalf("%s: caret rect unavailable", c.name)
		}
		ta.lastClickAt = time.Time{}
		ta.OnEvent(event.NewEvent(event.EventMouseDown, event.MouseData{X: rect.X, Y: rect.Y + rect.Height/2, Button: event.MouseButtonLeft}))
		ta.OnEvent(event.NewEvent(event.EventMouseUp, event.MouseData{X: rect.X, Y: rect.Y + rect.Height/2, Button: event.MouseButtonLeft}))
		if ta.cursorCol != 2048 {
			t.Errorf("%s: clicking the caret of column 2048 (x=%d) lands on column %d", c.name, rect.X, ta.cursorCol)
		}

		ta.Undo()
		check("after undo")
		ta.Redo()
		check("after redo")
	}
}
