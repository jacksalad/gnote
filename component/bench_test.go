package component

import (
	"strings"
	"testing"

	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/syntax"
)

// benchDoc 生成指定行数的典型代码/文本混合文档，用于性能基线与回归对比。
func benchDoc(lines int) string {
	const line = "func main() { fmt.Println(\"hello world 你好世界\") } // comment"
	var sb strings.Builder
	sb.Grow(lines * (len(line) + 2))
	for i := range lines {
		if i > 0 {
			sb.WriteString("\r\n")
		}
		sb.WriteString(line)
	}
	return sb.String()
}

func newBenchTextArea(b *testing.B, lines int) *TextArea {
	b.Helper()
	ta := NewTextArea()
	ta.SetBounds(0, 0, 1560, 920)
	ta.SetText(benchDoc(lines))
	ta.SetFocused(true)
	return ta
}

func benchCanvas() *render.Canvas {
	return render.NewCanvas(1560, 920)
}

// BenchmarkTextAreaRender 全帧渲染（10 万行文档，视口虚拟化）。
func BenchmarkTextAreaRender(b *testing.B) {
	ta := newBenchTextArea(b, 100000)
	canvas := benchCanvas()
	ta.Render(canvas)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		ta.Render(canvas)
	}
}

// BenchmarkTextAreaKeystrokeRender 单字符输入 + 退格 + 全帧渲染。
// 成对编辑保证行内容不随迭代增长，避免旧基准的 O(N²) 假象。
func BenchmarkTextAreaKeystrokeRender(b *testing.B) {
	ta := newBenchTextArea(b, 100000)
	canvas := benchCanvas()
	ta.Render(canvas)
	chars := event.NewEvent(event.EventKeyPress, event.KeyData{Char: 'x'})
	backspace := event.NewEvent(event.EventKeyDown, event.KeyData{KeyCode: 0x08})
	row := 0
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		row++
		if row >= ta.lineCount() {
			row = 0
		}
		ta.cursorRow, ta.cursorCol = row, 0
		ta.OnEvent(chars)
		ta.OnEvent(backspace)
		ta.Render(canvas)
	}
}

// BenchmarkTextAreaEnterRender 回车拆分 + 退格合并 + 全帧渲染（行数保持稳定）。
func BenchmarkTextAreaEnterRender(b *testing.B) {
	ta := newBenchTextArea(b, 100000)
	canvas := benchCanvas()
	ta.Render(canvas)
	enter := event.NewEvent(event.EventKeyDown, event.KeyData{KeyCode: 0x0D})
	backspace := event.NewEvent(event.EventKeyDown, event.KeyData{KeyCode: 0x08})
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		ta.cursorRow, ta.cursorCol = 0, 0
		ta.OnEvent(enter)
		ta.cursorRow, ta.cursorCol = 1, 0
		ta.OnEvent(backspace)
		ta.Render(canvas)
	}
}

// BenchmarkTextAreaMarkdownRender 语法高亮开启时的全帧渲染。
func BenchmarkTextAreaMarkdownRender(b *testing.B) {
	ta := newBenchTextArea(b, 100000)
	ta.SetHighlighter(syntax.NewMarkdownHighlighter())
	canvas := benchCanvas()
	ta.Render(canvas)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		ta.Render(canvas)
	}
}

// BenchmarkTextAreaLongLineCaret 关闭折行、光标位于 20 万字符长行末端时的稳态渲染。
// 验证光标 X 跨帧缓存，避免每帧从头测量整行。
func BenchmarkTextAreaLongLineCaret(b *testing.B) {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 1560, 920)
	ta.SetWordWrap(false)
	ta.SetText(strings.Repeat("abcdefghij", 20000))
	ta.SetFocused(true)
	ta.cursorRow, ta.cursorCol = 0, len(ta.line(0))
	ta.ScrollCursorIntoView()
	canvas := benchCanvas()
	ta.Render(canvas)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		ta.Render(canvas)
	}
}

// BenchmarkTextAreaGetText 6MB 文档序列化（保存路径）。
func BenchmarkTextAreaGetText(b *testing.B) {
	ta := newBenchTextArea(b, 100000)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = ta.GetText()
	}
}

// BenchmarkTextAreaSetText 10 万行文档装载（打开文件路径）。
func BenchmarkTextAreaSetText(b *testing.B) {
	doc := benchDoc(100000)
	ta := NewTextArea()
	ta.SetBounds(0, 0, 1560, 920)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		ta.SetText(doc)
	}
}

// BenchmarkTextAreaUndoRedo 200 条差分记录的撤销/重做回放。
func BenchmarkTextAreaUndoRedo(b *testing.B) {
	ta := newBenchTextArea(b, 100000)
	ta.cursorRow, ta.cursorCol = 0, 0
	for range 100 {
		pressRune(ta, 'x')
		pressKey(ta, 0x0D, false, false)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		for range 200 {
			ta.Undo()
		}
		for range 200 {
			ta.Redo()
		}
	}
}

// BenchmarkTextAreaReplaceAll 2 万行全部替换（单步撤销的批量编辑）。
func BenchmarkTextAreaReplaceAll(b *testing.B) {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 1560, 920)
	doc := benchDoc(20000)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		b.StopTimer()
		ta.SetText(doc)
		b.StartTimer()
		ta.ReplaceAll("comment", "remark", false)
	}
}

// BenchmarkTextAreaZoom 连续缩放（全部行排版失效后重建）。
func BenchmarkTextAreaZoom(b *testing.B) {
	ta := newBenchTextArea(b, 100000)
	canvas := benchCanvas()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		ta.SetZoom(1.0)
		ta.SetZoom(1.1)
		ta.Render(canvas)
	}
}

// BenchmarkTextAreaKeystrokeHighlight 高亮开启时的输入 + 退格 + 全帧渲染。
// 用于跟踪“编辑行尾状态未变化时保留下游 span 缓存”的效果。
func BenchmarkTextAreaKeystrokeHighlight(b *testing.B) {
	ta := newBenchTextArea(b, 100000)
	ta.SetHighlighter(syntax.NewMarkdownHighlighter())
	canvas := benchCanvas()
	ta.Render(canvas)
	chars := event.NewEvent(event.EventKeyPress, event.KeyData{Char: 'x'})
	backspace := event.NewEvent(event.EventKeyDown, event.KeyData{KeyCode: 0x08})
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		ta.cursorRow, ta.cursorCol = 0, 0
		ta.OnEvent(chars)
		ta.OnEvent(backspace)
		ta.Render(canvas)
	}
}

// BenchmarkTextAreaReplaceAllWrapped 窄视口/折行场景的全部替换（批量应用路径）。
func BenchmarkTextAreaReplaceAllWrapped(b *testing.B) {
	line := strings.Repeat("x", 60) + " comment " + strings.Repeat("y", 60)
	doc := strings.Repeat(line+"\r\n", 20000)
	ta := NewTextArea()
	ta.SetBounds(0, 0, 320, 200)
	ta.SetWordWrap(true)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		b.StopTimer()
		ta.SetText(doc)
		b.StartTimer()
		ta.ReplaceAll("comment", "remark", false)
	}
}

// BenchmarkTextAreaLongLineKeystroke 超长行末端输入 + 退格 + 渲染。
// 每次编辑会使前缀检查点失效，用于观察检查点重建成本。
func BenchmarkTextAreaLongLineKeystroke(b *testing.B) {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 1560, 920)
	ta.SetWordWrap(false)
	ta.SetText(strings.Repeat("abcdefghij", 20000))
	ta.SetFocused(true)
	ta.cursorRow, ta.cursorCol = 0, len(ta.line(0))
	ta.ScrollCursorIntoView()
	canvas := benchCanvas()
	ta.Render(canvas)
	chars := event.NewEvent(event.EventKeyPress, event.KeyData{Char: 'x'})
	backspace := event.NewEvent(event.EventKeyDown, event.KeyData{KeyCode: 0x08})
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		ta.OnEvent(chars)
		ta.OnEvent(backspace)
		ta.Render(canvas)
	}
}

// BenchmarkTextAreaLoadRender 新建编辑器 → 装载 10 万行 → 首帧渲染（打开文件路径）。
func BenchmarkTextAreaLoadRender(b *testing.B) {
	doc := benchDoc(100000)
	canvas := benchCanvas()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		ta := NewTextArea()
		ta.SetBounds(0, 0, 1560, 920)
		ta.SetText(doc)
		ta.Render(canvas)
	}
}

// BenchmarkTextAreaLongLineInsert 在 20 万字符行首连续输入。
// 旧模型每次输入克隆整行（约 800 KB）；gap 模型只移动间隙并写入 1 个 rune。
func BenchmarkTextAreaLongLineInsert(b *testing.B) {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 1560, 920)
	ta.SetWordWrap(false)
	ta.SetText(strings.Repeat("x", 200000))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		ta.applyEdit(0, 0, 0, 0, []rune{'y'})
	}
}

// BenchmarkTextAreaReplaceAllNewlines 2 万行全部替换为含换行文本。
// 行偏移表只重建一次，避免逐行 O(行数) 更新导致的平方级复杂度。
func BenchmarkTextAreaReplaceAllNewlines(b *testing.B) {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 1560, 920)
	ta.SetWordWrap(false)
	doc := strings.Repeat("a\n", 20000)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		ta.SetText(doc)
		ta.ReplaceAll("a", "X\nY", false)
	}
}

// BenchmarkTextAreaTypeContinuous 连续键入的可撤销合并路径（每 512 字符一组）。
func BenchmarkTextAreaTypeContinuous(b *testing.B) {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 1560, 920)
	ta.SetText("")
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		ta.handleKeyPress(event.KeyData{Char: 'a'})
	}
}
