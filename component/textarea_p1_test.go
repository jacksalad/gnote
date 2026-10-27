package component

import (
	"strings"
	"testing"

	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
)

func clickAt(ta *TextArea, row, col int, double bool) {
	ta.recomputeVisualLines()
	vIdx := 0
	if row < len(ta.logicalToVisual) {
		vIdx = ta.logicalToVisual[row]
	}
	x := ta.bounds.X + ta.getGutterWidth() + ta.margin + ta.measureStyledVisualX(row, 0, col)
	y := ta.bounds.Y + ta.margin + int32(vIdx)*ta.lineHeight + ta.lineHeight/2
	data := event.MouseData{X: x, Y: y, Button: event.MouseButtonLeft}
	typ := event.EventMouseDown
	if double {
		typ = event.EventMouseDoubleClick
	}
	ta.OnEvent(event.NewEvent(typ, data))
}

func TestTextArea_TypingCoalescesIntoOneUndo(t *testing.T) {
	ta := newTestTextArea("")
	for _, r := range "abc" {
		pressRune(ta, r)
	}
	if got := ta.GetText(); got != "abc" {
		t.Fatalf("typed text = %q", got)
	}
	if len(ta.undoStack) != 1 {
		t.Fatalf("continuous typing should merge into 1 undo record, got %d", len(ta.undoStack))
	}
	if !ta.Undo() {
		t.Fatal("undo failed")
	}
	if got := ta.GetText(); got != "" {
		t.Fatalf("single undo should remove the whole typing run, got %q", got)
	}
}

func TestTextArea_BackspaceCoalesces(t *testing.T) {
	ta := newTestTextArea("abcdef")
	ta.SetSelection(0, 6, 0, 6)
	for range 3 {
		pressKey(ta, 0x08, false, false)
	}
	if got := ta.GetText(); got != "abc" {
		t.Fatalf("after backspaces = %q", got)
	}
	if len(ta.undoStack) != 1 {
		t.Fatalf("continuous backspace should merge into 1 undo record, got %d", len(ta.undoStack))
	}
	ta.Undo()
	if got := ta.GetText(); got != "abcdef" {
		t.Fatalf("undo after backspaces = %q", got)
	}
}

func TestTextArea_CursorMoveBreaksMerge(t *testing.T) {
	ta := newTestTextArea("")
	pressRune(ta, 'a')
	pressKey(ta, 0x25, false, false) // Left
	pressRune(ta, 'b')
	if got := ta.GetText(); got != "ba" {
		t.Fatalf("text = %q", got)
	}
	if len(ta.undoStack) != 2 {
		t.Fatalf("non-adjacent typing must not merge, got %d records", len(ta.undoStack))
	}
	ta.Undo()
	if got := ta.GetText(); got != "a" {
		t.Fatalf("undo after break = %q", got)
	}
}

func TestTextArea_SmartIndentOnEnter(t *testing.T) {
	ta := newTestTextArea("    foo")
	ta.SetSelection(0, 7, 0, 7)
	pressKey(ta, 0x0D, false, false)
	if got, want := ta.GetText(), "    foo\r\n    "; got != want {
		t.Fatalf("indent after enter = %q want %q", got, want)
	}

	brace := newTestTextArea("func main() {")
	brace.SetSelection(0, 13, 0, 13)
	pressKey(brace, 0x0D, false, false)
	if got, want := brace.GetText(), "func main() {\r\n    "; got != want {
		t.Fatalf("brace indent = %q want %q", got, want)
	}
}

func TestTextArea_OutdentSelectionIsSingleUndo(t *testing.T) {
	ta := newTestTextArea("    a\r\n        b")
	ta.SelectAll()
	if !pressKey(ta, 0x09, true, false) {
		t.Fatal("shift+tab not handled")
	}
	if got, want := ta.GetText(), "a\r\n    b"; got != want {
		t.Fatalf("outdent result = %q want %q", got, want)
	}
	if len(ta.undoStack) != 1 {
		t.Fatalf("outdent should be one undo step, got %d", len(ta.undoStack))
	}
	ta.Undo()
	if got, want := ta.GetText(), "    a\r\n        b"; got != want {
		t.Fatalf("undo outdent = %q want %q", got, want)
	}

	single := newTestTextArea("    foo")
	single.SetSelection(0, 4, 0, 4)
	pressKey(single, 0x09, true, false)
	if got := single.GetText(); got != "foo" {
		t.Fatalf("single line outdent = %q", got)
	}
}

func TestTextArea_WordNavigation(t *testing.T) {
	ta := newTestTextArea("foo bar  baz")

	pressKey(ta, 0x27, false, true) // Ctrl+Right
	if _, col := ta.GetCursorPosition(); col != 4 {
		t.Fatalf("ctrl+right col = %d", col)
	}
	pressKey(ta, 0x27, false, true)
	if _, col := ta.GetCursorPosition(); col != 8 {
		t.Fatalf("second ctrl+right col = %d", col)
	}
	pressKey(ta, 0x27, false, true)
	if _, col := ta.GetCursorPosition(); col != 13 {
		t.Fatalf("third ctrl+right should reach line end, col = %d", col)
	}
	pressKey(ta, 0x25, false, true) // Ctrl+Left
	if _, col := ta.GetCursorPosition(); col != 10 {
		t.Fatalf("ctrl+left col = %d", col)
	}

	pressKey(ta, 0x23, false, true) // Ctrl+End
	if row, col := ta.GetCursorPosition(); row != 1 || col != 13 {
		t.Fatalf("ctrl+end = %d,%d", row, col)
	}
	pressKey(ta, 0x24, false, true) // Ctrl+Home
	if row, col := ta.GetCursorPosition(); row != 1 || col != 1 {
		t.Fatalf("ctrl+home = %d,%d", row, col)
	}
}

func TestTextArea_WordDeletion(t *testing.T) {
	ta := newTestTextArea("foo bar")
	ta.SetSelection(0, 7, 0, 7)
	pressKey(ta, 0x08, false, true) // Ctrl+Backspace
	if got := ta.GetText(); got != "foo " {
		t.Fatalf("ctrl+backspace = %q", got)
	}

	ta2 := newTestTextArea("foo bar")
	pressKey(ta2, 0x2E, false, true) // Ctrl+Delete
	if got := ta2.GetText(); got != " bar" {
		t.Fatalf("ctrl+delete = %q", got)
	}
}

func TestTextArea_DoubleClickSelectsWord(t *testing.T) {
	ta := newTestTextArea("alpha beta gamma")
	clickAt(ta, 0, 7, false)
	clickAt(ta, 0, 7, true)
	if got := ta.getSelectedText(); got != "beta" {
		t.Fatalf("double click selected %q", got)
	}

	cjk := newTestTextArea("你好世界 测试")
	clickAt(cjk, 0, 1, false)
	clickAt(cjk, 0, 1, true)
	if got := cjk.getSelectedText(); got != "你好世界" {
		t.Fatalf("cjk double click selected %q", got)
	}
}

func TestTextArea_TripleClickSelectsLine(t *testing.T) {
	ta := newTestTextArea("alpha beta\r\ngamma")
	clickAt(ta, 0, 3, false)
	clickAt(ta, 0, 3, false)
	clickAt(ta, 0, 3, false)
	sr, sc, er, ec, ok := ta.GetSelectionRange()
	if !ok || sr != 0 || sc != 0 || er != 0 || ec != len("alpha beta") {
		t.Fatalf("triple click selection = %d,%d-%d,%d ok=%v", sr, sc, er, ec, ok)
	}

	// 双击只选词，不应误触整行
	ta2 := newTestTextArea("alpha beta")
	clickAt(ta2, 0, 3, false)
	clickAt(ta2, 0, 3, true)
	if got := ta2.getSelectedText(); got != "alpha" {
		t.Fatalf("double click word = %q", got)
	}
}

func TestTextArea_RightClickDoesNotMoveCaret(t *testing.T) {
	ta := newTestTextArea("alpha beta")
	ta.SetSelection(1, 3, 1, 3)
	beforeRow, beforeCol := ta.GetCursorPosition()
	data := event.MouseData{X: ta.bounds.X + 10, Y: ta.bounds.Y + 5, Button: event.MouseButtonRight}
	if ta.OnEvent(event.NewEvent(event.EventMouseDown, data)) {
		t.Fatal("right button mouse down must not be consumed by editor")
	}
	if row, col := ta.GetCursorPosition(); row != beforeRow || col != beforeCol {
		t.Fatalf("caret moved by right click: %d,%d", row, col)
	}
}

func TestTextArea_ImeCompositionAndCommit(t *testing.T) {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 600, 400)
	ta.SetText("")
	ta.SetFocused(true)
	canvas := render.NewCanvas(600, 400)

	if !ta.OnEvent(event.NewEvent(event.EventImeStart, nil)) {
		// 无预编辑串时开始事件可被忽略，不视为失败
	}
	if !ta.OnEvent(event.NewEvent(event.EventImeUpdate, event.ImeData{Text: "nihao"})) {
		t.Fatal("ime update not handled")
	}
	if ta.Preedit() != "nihao" {
		t.Fatalf("preedit = %q", ta.Preedit())
	}
	ta.Render(canvas) // 预编辑串参与绘制且不得进入文档

	if got := ta.GetText(); got != "" {
		t.Fatalf("preedit must not modify document, got %q", got)
	}
	if !ta.OnEvent(event.NewEvent(event.EventImeCommit, event.ImeData{Text: "你好"})) {
		t.Fatal("ime commit not handled")
	}
	if got := ta.GetText(); got != "你好" {
		t.Fatalf("committed text = %q", got)
	}
	if ta.Preedit() != "" {
		t.Fatalf("preedit should be cleared after commit, got %q", ta.Preedit())
	}

	x, y, h, ok := ta.ImeAnchor()
	if !ok || h <= 0 || x <= 0 || y < 0 {
		t.Fatalf("ime anchor = %d,%d h=%d ok=%v", x, y, h, ok)
	}
}

func TestTextBox_ImeCommitAndAnchor(t *testing.T) {
	tb := NewTextBox()
	tb.SetBounds(10, 10, 240, 32)
	tb.SetFocused(true)

	tb.OnEvent(event.NewEvent(event.EventImeUpdate, event.ImeData{Text: "ni"}))
	if !tb.OnEvent(event.NewEvent(event.EventImeCommit, event.ImeData{Text: "你"})) {
		t.Fatal("textbox ime commit not handled")
	}
	if got := tb.GetText(); got != "你" {
		t.Fatalf("textbox text = %q", got)
	}
	if _, _, h, ok := tb.ImeAnchor(); !ok || h <= 0 {
		t.Fatalf("textbox ime anchor invalid: ok=%v h=%d", ok, h)
	}
}

func TestTextArea_ShiftWheelScrollsHorizontally(t *testing.T) {
	ta := newTestTextArea("short line")
	ta.SetBounds(0, 0, 300, 200)
	ta.SetWordWrap(false)
	lenLong := 400
	long := make([]rune, lenLong)
	for i := range long {
		long[i] = 'x'
	}
	ta.SetText("short line\r\n" + string(long))
	ta.Render(render.NewCanvas(300, 200))

	before := ta.scrollOffsetX
	data := event.MouseData{X: 50, Y: 40, Delta: -120, Shift: true}
	if !ta.OnEvent(event.NewEvent(event.EventMouseWheel, data)) {
		t.Fatal("shift+wheel not handled")
	}
	if ta.scrollOffsetX <= before {
		t.Fatalf("horizontal scroll did not advance: before=%d after=%d", before, ta.scrollOffsetX)
	}
	if ta.scrollOffsetY != 0 {
		t.Fatalf("shift+wheel must not scroll vertically, got %d", ta.scrollOffsetY)
	}
}

// 触摸板的滚轮增量经常小于 120（如 ±1/±30），整数除法会直接丢弃；必须累加成像素。
func TestTextArea_WheelFractionalDeltaAccumulates(t *testing.T) {
	lines := strings.Repeat("line\r\n", 60) + "line"
	ta := newTestTextArea(lines)
	ta.SetBounds(0, 0, 300, 200)
	ta.Render(render.NewCanvas(300, 200))
	notch := ta.lineHeight * 3

	// 半个刻度：应产生部分像素，但不足一整刻度
	ta.OnEvent(event.NewEvent(event.EventMouseWheel, event.MouseData{X: 50, Y: 40, Delta: -60}))
	half := ta.scrollOffsetY
	if half <= 0 || half >= notch {
		t.Fatalf("half notch scrolled %d, want 0 < d < %d", half, notch)
	}

	// 120 个 1 单位事件应恰好补满一个刻度
	before := ta.scrollOffsetY
	for i := 0; i < 120; i++ {
		ta.OnEvent(event.NewEvent(event.EventMouseWheel, event.MouseData{X: 50, Y: 40, Delta: -1}))
	}
	if got := ta.scrollOffsetY - before; got != notch {
		t.Fatalf("120 single-unit deltas scrolled %d, want %d", got, notch)
	}
}

// Ctrl+滚轮应按增量比例缩放：一个刻度 10%，分数增量累加后才步进。
func TestTextArea_CtrlWheelScalesProportionally(t *testing.T) {
	ta := newTestTextArea("hello")
	ta.SetBounds(0, 0, 300, 200)
	ta.Render(render.NewCanvas(300, 200))
	ta.SetZoom(1.0)

	ta.OnEvent(event.NewEvent(event.EventMouseWheel, event.MouseData{
		X: 50, Y: 40, Delta: 60, Ctrl: true,
	}))
	if got := ta.GetZoomPercent(); got != 100 {
		t.Fatalf("half-notch must not zoom yet, got %d%%", got)
	}
	ta.OnEvent(event.NewEvent(event.EventMouseWheel, event.MouseData{
		X: 50, Y: 40, Delta: 60, Ctrl: true,
	}))
	if got := ta.GetZoomPercent(); got != 110 {
		t.Fatalf("accumulated notch should zoom to 110%%, got %d%%", got)
	}
	ta.OnEvent(event.NewEvent(event.EventMouseWheel, event.MouseData{
		X: 50, Y: 40, Delta: -240, Ctrl: true,
	}))
	if got := ta.GetZoomPercent(); got != 90 {
		t.Fatalf("two negative notches should zoom to 90%%, got %d%%", got)
	}
}

// 光标局部重绘接口：矩形跟随光标，滚出视口时返回 false；任意输入/移动都会重新点亮光标。
func TestTextArea_CaretRectAndBlink(t *testing.T) {
	ta := newTestTextArea(strings.Repeat("line\r\n", 40) + "line")
	ta.SetBounds(0, 0, 300, 200)
	canvas := render.NewCanvas(300, 200)
	ta.Render(canvas)

	r1, ok := ta.CaretRect()
	if !ok {
		t.Fatal("caret rect unavailable for focused editor")
	}
	if r1.Width != 2 || r1.Height != ta.lineHeight {
		t.Fatalf("caret rect = %dx%d, want 2x%d", r1.Width, r1.Height, ta.lineHeight)
	}
	if !ta.caretOn {
		t.Fatal("caret must be on after render")
	}
	ta.ToggleCaretBlink()
	if ta.caretOn {
		t.Fatal("toggle should hide caret")
	}
	ta.notifyCursor()
	if !ta.caretOn {
		t.Fatal("cursor notification should re-show caret")
	}

	// 移到视口之外的光标不应触发局部重绘
	ta.cursorRow, ta.cursorCol = ta.lineCount()-1, 0
	ta.scrollOffsetY = 0
	if _, ok := ta.CaretRect(); ok {
		t.Fatal("off-screen caret must not report a rect")
	}
}

// 超长行前缀检查点必须与逐 rune 测量一致，且完全离屏时 drawStyledRange 仍返回整段宽度。
func TestTextArea_LongLinePrefixCache(t *testing.T) {
	long := strings.Repeat("abc中文def", 2000) // 18000 runes，含 CJK
	ta := newTestTextArea(long)
	ta.SetBounds(0, 0, 300, 200)
	ta.SetWordWrap(false)
	ta.Render(render.NewCanvas(300, 200))

	p := ta.prefixFor(0)
	if p == nil {
		t.Fatal("expected prefix cache for long line")
	}
	if len(ta.longPrefixes) != 1 {
		t.Fatalf("prefix map size = %d", len(ta.longPrefixes))
	}

	// 检查点宽度 vs 分段慢速测量（每段 <= prefixStep，绕过前缀快路径）
	for col := 0; col <= len(long); col += prefixStep {
		want := int32(0)
		for start := 0; start < col; start += prefixStep {
			end := start + prefixStep
			if end > col {
				end = col
			}
			want += ta.measureStyledVisualX(0, start, end)
		}
		if got := ta.prefixWidthAt(p, 0, col); got != want {
			t.Fatalf("prefix width at %d = %d, want %d", col, got, want)
		}
	}

	// 完全离屏（视口在整行右侧）时返回值仍是整行宽度
	canvas := render.NewCanvas(300, 200)
	canvas.PushClip(graphics.NewRect(290, 0, 10, 20))
	total := ta.prefixWidthAt(p, 0, len(long))
	got := ta.drawStyledRange(canvas, 0, ta.line(0), nil, 0, len(ta.line(0)), -int32(total)-1000, 0)
	canvas.PopClip()
	if got != total {
		t.Fatalf("clipped draw width = %d, want %d", got, total)
	}

	// 行尾输入：检查点应增量续算而不是整体重建
	ta.cursorRow, ta.cursorCol = 0, len(ta.line(0))
	pressRune(ta, 'z')
	p = ta.prefixFor(0)
	lineWidth := render.GetFontManager().MeasureRunesFace(ta.fontFace, ta.line(0))
	if got := ta.prefixWidthAt(p, 0, len(ta.line(0))); got != lineWidth {
		t.Fatalf("prefix width after append = %d, want %d", got, lineWidth)
	}

	// 行首输入：截断后重新续算，结果仍须正确
	ta.cursorRow, ta.cursorCol = 0, 0
	pressRune(ta, 'q')
	p = ta.prefixFor(0)
	lineWidth = render.GetFontManager().MeasureRunesFace(ta.fontFace, ta.line(0))
	if got := ta.prefixWidthAt(p, 0, len(ta.line(0))); got != lineWidth {
		t.Fatalf("prefix width after prepend = %d, want %d", got, lineWidth)
	}
}

// 等宽上界必须保守：判定“必然不折行”时，精确测量结果不能超过可用宽度。
func TestTextArea_LineFitsUpperBoundIsSafe(t *testing.T) {
	ta := newTestTextArea("")
	lines := []string{
		"abcdefghij",
		"中文换行测试中文换行测试",
		"\tindented line with tab",
		"mix 中文 and ascii and\ttabs",
		strings.Repeat("x", 200),
	}
	fm := render.GetFontManager()
	for _, s := range lines {
		runes := []rune(s)
		for avail := int32(40); avail <= 600; avail += 8 {
			if !ta.lineFitsUpperBound(runes, avail) {
				continue
			}
			if exact := fm.MeasureRunesFace(ta.fontFace, runes); exact > avail {
				t.Fatalf("upper bound claimed fit for %q at %d, exact width %d", s, avail, exact)
			}
		}
	}
}

// 缩放时短行不应触发逐字形测量：等宽上界通过的行保持 widthOK=false。
func TestTextArea_ZoomKeepsShortRowsLazy(t *testing.T) {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 600, 400)
	ta.SetText(strings.Repeat("short line of code\r\n", 2000))
	canvas := render.NewCanvas(600, 400)
	ta.Render(canvas)

	for i := range ta.layouts {
		if ta.layouts[i].widthOK {
			t.Fatalf("row %d measured eagerly after render", i)
		}
	}
	assertVisualInvariants(t, ta)

	ta.SetZoom(1.5)
	ta.Render(canvas)
	for i := range ta.layouts {
		if ta.layouts[i].widthOK {
			t.Fatalf("row %d measured eagerly after zoom", i)
		}
	}
	assertVisualInvariants(t, ta)
}
