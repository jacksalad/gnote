package component

import (
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"testing"

	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/syntax"
)

func newTestTextArea(text string) *TextArea {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 800, 600)
	ta.SetText(text)
	ta.SetFocused(true)
	return ta
}

func pressKey(ta *TextArea, code int, shift, ctrl bool) bool {
	return ta.OnEvent(event.NewEvent(event.EventKeyDown, event.KeyData{
		KeyCode: code, Shift: shift, Ctrl: ctrl,
	}))
}

func pressRune(ta *TextArea, ch rune) bool {
	return ta.OnEvent(event.NewEvent(event.EventKeyPress, event.KeyData{Char: ch}))
}

func moveRight(t *testing.T, ta *TextArea, n int) {
	t.Helper()
	for range n {
		pressKey(ta, 0x27, false, false)
	}
}

func TestTextArea_MultiLineInsertInMiddleDoesNotCorrupt(t *testing.T) {
	ta := newTestTextArea("HELLOWORLD")
	moveRight(t, ta, 5)
	ta.insertText("a\r\nb")

	const want = "HELLOa\r\nbWORLD"
	if got := ta.GetText(); got != want {
		t.Fatalf("multi-line insert corrupted text: got %q want %q", got, want)
	}
	if r, c := ta.GetCursorPosition(); r != 2 || c != 2 {
		t.Fatalf("unexpected cursor after insert: row=%d col=%d, want row=2 col=2", r, c)
	}

	ta.Undo()
	if got := ta.GetText(); got != "HELLOWORLD" {
		t.Fatalf("undo after multi-line insert failed: %q", got)
	}
	ta.Redo()
	if got := ta.GetText(); got != want {
		t.Fatalf("redo after multi-line insert failed: %q", got)
	}
}

func TestTextArea_UndoRedoAcrossEditKinds(t *testing.T) {
	ta := newTestTextArea("abc\r\ndef")
	setCursor := func(row, col int) {
		ta.cursorRow = row
		ta.cursorCol = col
		ta.selectionActive = false
		ta.updatePreferredX()
		ta.ensureCursorVisible()
	}

	// Backspace at document start is a no-op
	pressKey(ta, 0x08, false, false)
	if got := ta.GetText(); got != "abc\r\ndef" {
		t.Fatalf("backspace at start changed text: %q", got)
	}

	// Delete joins the next line into the current one
	setCursor(0, 3)
	pressKey(ta, 0x2E, false, false)
	if got := ta.GetText(); got != "abcdef" {
		t.Fatalf("delete join failed: %q", got)
	}
	ta.Undo()
	if got := ta.GetText(); got != "abc\r\ndef" {
		t.Fatalf("undo delete join failed: %q", got)
	}
	ta.Redo()
	if got := ta.GetText(); got != "abcdef" {
		t.Fatalf("redo delete join failed: %q", got)
	}
	ta.Undo()

	// Enter splits the line
	setCursor(0, 3)
	pressKey(ta, 0x0D, false, false)
	if got := ta.GetText(); got != "abc\r\n\r\ndef" {
		t.Fatalf("enter split failed: %q", got)
	}
	ta.Undo()
	if got := ta.GetText(); got != "abc\r\ndef" {
		t.Fatalf("undo enter failed: %q", got)
	}

	// Tab inserts four spaces
	setCursor(0, 0)
	pressKey(ta, 0x09, false, false)
	if got := ta.GetText(); got != "    abc\r\ndef" {
		t.Fatalf("tab insert failed: %q", got)
	}
	ta.Undo()

	// Shift+Right selects 'd' on the second line, then typing replaces it as one undo step
	setCursor(1, 0)
	pressKey(ta, 0x27, true, false)
	pressRune(ta, 'X')
	if got := ta.GetText(); got != "abc\r\nXef" {
		t.Fatalf("selection replacement failed: %q", got)
	}
	ta.Undo()
	if got := ta.GetText(); got != "abc\r\ndef" {
		t.Fatalf("undo selection replacement failed: %q", got)
	}
}

func TestTextArea_UndoMemoryBounded(t *testing.T) {
	const lines = 20000
	original := strings.Repeat("line\r\n", lines)
	ta := newTestTextArea(original)

	const edits = 100
	// 交替输入与换行：换行会打断撤销合组，确保生成等量的独立撤销记录
	for range edits {
		pressRune(ta, 'x')
		pressKey(ta, 0x0D, false, false)
	}
	if len(ta.undoStack) != edits*2 {
		t.Fatalf("expected %d undo records, got %d", edits*2, len(ta.undoStack))
	}
	// Full-document snapshots would need ~700KB+ here; delta records must stay tiny.
	if ta.undoBytes > 64*1024 {
		t.Fatalf("undo history stores too much data: %d bytes for %d single-rune edits", ta.undoBytes, edits)
	}

	for range edits * 2 {
		if !ta.Undo() {
			t.Fatalf("undo failed before restoring all edits")
		}
	}
	if got := ta.GetText(); got != original {
		t.Fatalf("text after undoing all edits mismatch: got %d bytes want %d bytes", len(got), len(original))
	}
	if len(ta.redoStack) != edits*2 {
		t.Fatalf("expected %d redo records, got %d", edits*2, len(ta.redoStack))
	}
}

func TestTextArea_VisualIndexInvariants(t *testing.T) {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 260, 200) // 窄窗口强制折行
	ta.SetText("short\r\n" + strings.Repeat("verylongsegment", 40) + "\r\n中文换行测试" + strings.Repeat("汉", 120))
	canvas := render.NewCanvas(260, 200)
	ta.Render(canvas)
	assertVisualInvariants(t, ta)

	moveRight(t, ta, 3)
	pressRune(ta, 'Z')
	ta.Render(canvas)
	assertVisualInvariants(t, ta)

	ta.SetBounds(0, 0, 700, 200)
	ta.Render(canvas)
	assertVisualInvariants(t, ta)

	ta.Undo()
	ta.Render(canvas)
	assertVisualInvariants(t, ta)
}

func assertVisualInvariants(t *testing.T, ta *TextArea) {
	t.Helper()
	ta.recomputeVisualLines()
	if len(ta.logicalToVisual) != ta.lineCount()+1 {
		t.Fatalf("logicalToVisual len=%d, lines=%d (expected lines+1 sentinel)", len(ta.logicalToVisual), ta.lineCount())
	}
	if got := ta.logicalToVisual[ta.lineCount()]; got != len(ta.visualLines) {
		t.Fatalf("logicalToVisual sentinel=%d, visualLines=%d", got, len(ta.visualLines))
	}
	for r := 0; r < ta.lineCount(); r++ {
		line := ta.line(r)
		startIdx := ta.logicalToVisual[r]
		if startIdx < 0 || startIdx >= len(ta.visualLines) {
			t.Fatalf("row %d has invalid first visual index %d (total %d)", r, startIdx, len(ta.visualLines))
		}
		col := 0
		idx := startIdx
		first := true
		for idx < len(ta.visualLines) && ta.visualLines[idx].logicalRow == r {
			v := ta.visualLines[idx]
			if v.startCol != col {
				t.Fatalf("row %d visual %d starts at %d, expected %d", r, idx, v.startCol, col)
			}
			if v.endCol < v.startCol {
				t.Fatalf("row %d visual %d has negative width", r, idx)
			}
			if v.isFirstSub != first {
				t.Fatalf("row %d visual %d isFirstSub=%v, expected %v", r, idx, v.isFirstSub, first)
			}
			col = v.endCol
			first = false
			idx++
		}
		if col != len(line) {
			t.Fatalf("row %d visual segments cover %d cols, line has %d", r, col, len(line))
		}
		if idx < len(ta.visualLines) && ta.visualLines[idx].logicalRow <= r {
			t.Fatalf("row %d visual blocks are not ordered", r)
		}
	}
}

func TestTextArea_OutsideClickKeepsFocus(t *testing.T) {
	ta := NewTextArea()
	ta.SetBounds(10, 10, 200, 100)
	ta.SetFocused(true)

	consumed := ta.OnEvent(event.NewEvent(event.EventMouseDown, event.MouseData{
		X: 500, Y: 500, Button: event.MouseButtonLeft,
	}))
	if consumed {
		t.Fatalf("click outside the editor must not be consumed")
	}
	if !ta.IsFocused() {
		t.Fatalf("clicking outside must not silently remove editor focus")
	}
	if !pressRune(ta, 'q') {
		t.Fatalf("editor stopped accepting input after an outside click")
	}
	if ta.GetText() != "q" {
		t.Fatalf("typed text missing after outside click: %q", ta.GetText())
	}
}

func TestTextArea_SetTextResetsHistory(t *testing.T) {
	ta := newTestTextArea("abc")
	pressRune(ta, 'x')
	if len(ta.undoStack) != 1 {
		t.Fatalf("expected one undo record")
	}
	ta.SetText("fresh")
	if len(ta.undoStack) != 0 || len(ta.redoStack) != 0 {
		t.Fatalf("SetText must clear undo/redo history")
	}
	if ta.Undo() {
		t.Fatalf("Undo must fail after SetText")
	}
	if got := ta.GetText(); got != "fresh" {
		t.Fatalf("unexpected text after SetText: %q", got)
	}
}

func TestTextArea_ModifiedTracksSavePointAcrossUndo(t *testing.T) {
	ta := newTestTextArea("")
	pressRune(ta, 'A')
	ta.SetModified(false) // 模拟保存：当前状态（内容 "A"）即保存点

	pressRune(ta, 'B')
	if !ta.IsModified() {
		t.Fatalf("typing after save must mark modified")
	}

	ta.Undo() // 回到保存点
	if got := ta.GetText(); got != "A" {
		t.Fatalf("undo should return to saved content, got %q", got)
	}
	if ta.IsModified() {
		t.Fatalf("state at save point must not be marked modified")
	}

	ta.Undo() // 越过保存点
	if got := ta.GetText(); got != "" {
		t.Fatalf("second undo should reach empty document, got %q", got)
	}
	if !ta.IsModified() {
		t.Fatalf("state before save point must be marked modified")
	}

	ta.Redo() // 再次回到保存点
	if ta.IsModified() {
		t.Fatalf("redo back to save point must clear modified")
	}
}

func TestTextArea_RandomEditUndoRedoConsistency(t *testing.T) {
	ta := newTestTextArea("seed")
	ref := []rune("seed")
	cur := 0
	rng := rand.New(rand.NewPCG(7, 7))
	canvas := render.NewCanvas(320, 240)

	insert := func(pos int, rs ...rune) {
		ref = append(ref, make([]rune, len(rs))...)
		copy(ref[pos+len(rs):], ref[pos:])
		copy(ref[pos:], rs)
	}
	remove := func(pos int) {
		ref = append(ref[:pos], ref[pos+1:]...)
	}
	normalized := func(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }

	for step := range 300 {
		switch rng.IntN(8) {
		case 0, 1, 2:
			ch := rune('a' + rng.IntN(26))
			insert(cur, ch)
			cur++
			pressRune(ta, ch)
		case 3:
			// 参考模型同步 Enter 的智能缩进：复制当前行前缀空白
			lineStart := cur
			for lineStart > 0 && ref[lineStart-1] != '\n' {
				lineStart--
			}
			indent := 0
			for lineStart+indent < cur && ref[lineStart+indent] == ' ' {
				indent++
			}
			ins := append([]rune{'\n'}, ref[lineStart:lineStart+indent]...)
			insert(cur, ins...)
			cur += len(ins)
			pressKey(ta, 0x0D, false, false)
		case 4:
			if cur > 0 {
				remove(cur - 1)
				cur--
			}
			pressKey(ta, 0x08, false, false)
		case 5:
			if cur < len(ref) {
				remove(cur)
			}
			pressKey(ta, 0x2E, false, false)
		case 6:
			if rng.IntN(2) == 0 {
				if cur < len(ref) {
					cur++
				}
				pressKey(ta, 0x27, false, false)
			} else {
				if cur > 0 {
					cur--
				}
				pressKey(ta, 0x25, false, false)
			}
		case 7:
			insert(cur, ' ', ' ', ' ', ' ')
			cur += 4
			pressKey(ta, 0x09, false, false)
		}
		if got, want := normalized(ta.GetText()), string(ref); got != want {
			t.Fatalf("mismatch after step %d (cursor=%d):\n got=%q\nwant=%q", step, cur, got, want)
		}
		assertTextStorageInvariants(t, ta)
	}

	ta.Render(canvas)
	assertVisualInvariants(t, ta)

	finalText := string(ref)
	for ta.Undo() {
	}
	if got := normalized(ta.GetText()); got != "seed" {
		t.Fatalf("undo-all mismatch: got %q want seed", got)
	}
	for ta.Redo() {
	}
	if got := normalized(ta.GetText()); got != finalText {
		t.Fatalf("redo-all mismatch: got %q want %q", got, finalText)
	}
	ta.Render(canvas)
	assertVisualInvariants(t, ta)
}

// 回归：内嵌等宽字体下，编辑器 1 空格 = 1 个字符宽度、ASCII 全等宽、中文严格 2 格，
// 因此 4 空格缩进与制表符等于 4 个英文字符宽度（比例字体原生空格仅约半字符）。
func TestTextArea_SpaceAdvancesOneCharacter(t *testing.T) {
	data, err := os.ReadFile("../cmd/gnote/font.ttf")
	if err != nil {
		t.Skipf("embedded font unavailable: %v", err)
	}
	family, err := render.RegisterMemoryFont(data)
	if err != nil {
		t.Fatalf("register font: %v", err)
	}

	ta := NewTextArea()
	ta.SetFontFamily(family)
	ta.SetBaseFontSize(16)
	face := ta.fontFace
	if face == nil {
		t.Fatal("editor font face not initialized")
	}

	fm := render.GetFontManager()
	cell := fm.RuneWidthFace(face, '0')
	for _, r := range []rune{' ', 'a', 'i', 'm', 'W', '_', '{', '}'} {
		if got := fm.RuneWidthFace(face, r); got != cell {
			t.Fatalf("%q advance=%d, want one cell (%d)", r, got, cell)
		}
	}
	if got := fm.RuneWidthFace(face, '中'); got != cell*2 {
		t.Fatalf("CJK advance=%d, want two cells (%d)", got, cell*2)
	}
	if got := fm.MeasureRunesFace(face, []rune("    ")); got != cell*4 {
		t.Fatalf("editor 4-space indent=%d, want %d", got, cell*4)
	}
	if got := fm.MeasureRunesFace(face, []rune("\t")); got != cell*4 {
		t.Fatalf("editor tab=%d, want %d", got, cell*4)
	}
}

func TestTextArea_IncrementalVisualIndexStaysConsistent(t *testing.T) {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 320, 240)
	ta.SetText("alpha beta gamma\r\n" +
		strings.Repeat("wrap ", 20) + "\r\n" +
		"中文折行测试" + strings.Repeat("汉", 60) + "\r\n" +
		"tail")
	ta.SetFocused(true)
	canvas := render.NewCanvas(320, 240)

	check := func(step int) {
		t.Helper()
		ta.Render(canvas)
		assertVisualInvariants(t, ta)
	}
	check(0)

	for i := range 40 {
		pressKey(ta, 0x0D, false, false) // Enter: 行数变化 + 尾部平移
		check(i*4 + 1)
		pressRune(ta, 'x') // 行内编辑：局部视觉段重建
		check(i*4 + 2)
		pressKey(ta, 0x08, false, false) // Backspace
		check(i*4 + 3)
		pressKey(ta, 0x2E, false, false) // Delete
		check(i*4 + 4)
		if i%7 == 0 {
			ta.SetBounds(0, 0, 200+int32(i)*8, 240) // 改宽触发整体重排
			check(i)
		}
	}

	ta.SetWordWrap(false)
	check(1)
	ta.SetWordWrap(true)
	check(2)
}

// 回归：maxContentWidth 曾只增不减，删除最长行/缩小字号后仍允许水平滚动到空白区域。
func TestTextArea_MaxContentWidthShrinks(t *testing.T) {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 400, 300)
	ta.SetWordWrap(false)
	ta.SetFocused(true)
	ta.SetText(strings.Repeat("a", 3000) + "\nshort")
	// 惰性排版：关闭折行时宽度由可见行渲染时按需测量
	ta.Render(render.NewCanvas(400, 300))
	if ta.maxContentWidth == 0 {
		t.Fatal("long line width was not measured")
	}

	// 全选删除：最长行被移除，水平滚动上界必须归零
	pressKey(ta, 'A', false, true)
	pressKey(ta, 0x08, false, false)
	ta.clampScroll()
	if got := ta.GetText(); got != "" {
		t.Fatalf("delete all failed, text len=%d", len(got))
	}
	if ta.maxContentWidth != 0 {
		t.Fatalf("stale maxContentWidth=%d after deleting all text", ta.maxContentWidth)
	}
	ta.scrollOffsetX = 1 << 30
	ta.clampScroll()
	if ta.scrollOffsetX != 0 {
		t.Fatalf("still scrollable into empty space: scrollOffsetX=%d", ta.scrollOffsetX)
	}

	// 缩放变小后，上界必须按新字体重新测量，而不是保留旧字号的上界
	ta.SetText(strings.Repeat("a", 3000))
	ta.Render(render.NewCanvas(400, 300))
	wide := ta.maxContentWidth
	if wide == 0 {
		t.Fatal("wide line was not measured before zoom")
	}
	ta.SetZoom(0.5)
	ta.clampScroll()
	ta.Render(render.NewCanvas(400, 300))
	if ta.maxContentWidth >= wide {
		t.Fatalf("maxContentWidth not recomputed after zoom: %d >= %d", ta.maxContentWidth, wide)
	}
	if got, want := ta.maxContentWidth, ta.layouts[0].totalWidth; got != want {
		t.Fatalf("maxContentWidth=%d want measured width %d", got, want)
	}
}

// Tab 在多行选区上应整行缩进，而不是用 4 个空格替换掉整段选区。
func TestTextArea_TabIndentsMultiLineSelection(t *testing.T) {
	ta := newTestTextArea("aa\nbb\ncc")
	ta.SetSelection(0, 1, 2, 1)

	if !pressKey(ta, 0x09, false, false) {
		t.Fatal("Tab should be handled")
	}
	if got, want := ta.GetTextWithEOL("\n"), "    aa\n    bb\n    cc"; got != want {
		t.Fatalf("indent result: %q want %q", got, want)
	}
	sr, sc, er, ec, ok := ta.GetSelectionRange()
	if !ok || sr != 0 || sc != 5 || er != 2 || ec != 5 {
		t.Fatalf("selection after indent = %d,%d-%d,%d ok=%v", sr, sc, er, ec, ok)
	}

	if !ta.Undo() {
		t.Fatal("undo failed")
	}
	if got, want := ta.GetTextWithEOL("\n"), "aa\nbb\ncc"; got != want {
		t.Fatalf("undo result: %q want %q", got, want)
	}
	if !ta.Redo() {
		t.Fatal("redo failed")
	}
	if got, want := ta.GetTextWithEOL("\n"), "    aa\n    bb\n    cc"; got != want {
		t.Fatalf("redo result: %q want %q", got, want)
	}

	// 单行选区保持原有语义：用缩进替换选中文本
	single := newTestTextArea("hello")
	single.SetSelection(0, 1, 0, 3)
	pressKey(single, 0x09, false, false)
	if got, want := single.GetText(), "h    lo"; got != want {
		t.Fatalf("single-line tab: %q want %q", got, want)
	}
}

func TestTextArea_ShiftClickExtendsSelection(t *testing.T) {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 800, 600)
	ta.SetText("hello\nworld")
	ta.SetFocused(true)

	textStartX := ta.getGutterWidth() + ta.margin
	click := func(row int, shift bool) {
		ta.OnEvent(event.NewEvent(event.EventMouseDown, event.MouseData{
			X: textStartX, Y: ta.margin + int32(row)*ta.lineHeight,
			Button: event.MouseButtonLeft, Shift: shift,
		}))
	}
	click(0, false)
	click(1, true)

	sr, sc, er, ec, ok := ta.GetSelectionRange()
	if !ok || sr != 0 || sc != 0 || er != 1 || ec != 0 {
		t.Fatalf("shift-click selection = %d,%d-%d,%d ok=%v", sr, sc, er, ec, ok)
	}
}

func TestTextArea_CtrlShiftZRedo(t *testing.T) {
	ta := newTestTextArea("abc")
	if !pressRune(ta, 'x') {
		t.Fatal("insert failed")
	}
	if !pressKey(ta, 'Z', false, true) {
		t.Fatal("undo failed")
	}
	if got := ta.GetText(); got != "abc" {
		t.Fatalf("after undo: %q", got)
	}
	if !pressKey(ta, 'Z', true, true) {
		t.Fatal("Ctrl+Shift+Z redo failed")
	}
	if got := ta.GetText(); got != "xabc" {
		t.Fatalf("after redo: %q", got)
	}
}

func TestTextArea_ScrollbarDrag(t *testing.T) {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 240, 120)
	ta.SetFocused(true)
	ta.SetText(strings.Repeat("line\n", 100))
	ta.recomputeVisualLines()

	thumb, ok := ta.verticalScrollbarRect()
	if !ok {
		t.Fatalf("expected a vertical scrollbar, visuals=%d", len(ta.visualLines))
	}
	cx, cy := thumb.X+thumb.Width/2, thumb.Y+thumb.Height/2
	ta.OnEvent(event.NewEvent(event.EventMouseDown, event.MouseData{
		X: cx, Y: cy, Button: event.MouseButtonLeft,
	}))
	if !ta.draggingScrollbar {
		t.Fatal("mouse down on thumb should start dragging")
	}

	ta.OnEvent(event.NewEvent(event.EventMouseMove, event.MouseData{X: cx, Y: cy + 20}))
	if ta.scrollOffsetY <= 0 {
		t.Fatalf("drag did not scroll, offset=%d", ta.scrollOffsetY)
	}
	before := ta.scrollOffsetY

	ta.OnEvent(event.NewEvent(event.EventMouseUp, event.MouseData{
		X: cx, Y: cy + 20, Button: event.MouseButtonLeft,
	}))
	if ta.draggingScrollbar {
		t.Fatal("mouse up should stop dragging")
	}
	ta.OnEvent(event.NewEvent(event.EventMouseMove, event.MouseData{X: cx, Y: cy + 40}))
	if ta.scrollOffsetY != before {
		t.Fatalf("scroll changed after release: %d -> %d", before, ta.scrollOffsetY)
	}
}

func TestTextArea_PreeditOnlyOnOneVisualLineAtWrapBoundary(t *testing.T) {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 120, 200)
	ta.SetWordWrap(true)
	ta.SetFocused(true)
	ta.SetText(strings.Repeat("a", 200))
	ta.recomputeVisualLines()

	var boundary visualLine
	found := false
	for _, vl := range ta.visualLines {
		if !vl.isLastSub {
			boundary = vl
			found = true
			break
		}
	}
	if !found {
		t.Skip("line did not wrap at this width")
	}

	ta.cursorRow, ta.cursorCol = boundary.logicalRow, boundary.endCol
	ta.preedit = "中"
	count := 0
	for _, vl := range ta.visualLines {
		if ta.preeditOnVisualLine(vl) {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("preedit drawn on %d visual lines at wrap boundary, want 1", count)
	}
}

func TestTextArea_CopyPreservesFileEOL(t *testing.T) {
	ta := NewTextArea()
	ta.SetText("aa\nbb\ncc")
	ta.SetSelection(0, 0, 2, 2)

	ta.SetLineEnding("\n")
	if got, want := ta.getSelectedText(), "aa\nbb\ncc"; got != want {
		t.Fatalf("LF copy: %q want %q", got, want)
	}

	ta.SetLineEnding("\r\n")
	if got, want := ta.getSelectedText(), "aa\r\nbb\r\ncc"; got != want {
		t.Fatalf("CRLF copy: %q want %q", got, want)
	}
}

func TestTextArea_SetFontUpdatesStyledVariants(t *testing.T) {
	ta := newTestTextArea("x")
	ta.SetFont(graphics.NewFont("Courier New", 20))

	if ta.font.Name != "Courier New" || ta.font.Size != 20 {
		t.Fatalf("base font = %s %d", ta.font.Name, ta.font.Size)
	}
	if ta.fontBold.Name != "Courier New" || ta.fontBold.Size != 20 ||
		ta.fontItalic.Name != "Courier New" || ta.fontBoldItalic.Size != 20 {
		t.Fatalf("styled variants not rebuilt: bold=%s/%d italic=%s/%d bolditalic=%s/%d",
			ta.fontBold.Name, ta.fontBold.Size, ta.fontItalic.Name, ta.fontItalic.Size,
			ta.fontBoldItalic.Name, ta.fontBoldItalic.Size)
	}
}

func TestTextArea_ReplaceRespectsReadOnly(t *testing.T) {
	ta := newTestTextArea("foo foo")
	ta.SetSelection(0, 0, 0, 3)
	ta.SetReadOnly(true)

	if n := ta.ReplaceAll("foo", "bar", true); n != 0 {
		t.Fatalf("read-only ReplaceAll replaced %d matches", n)
	}
	if ta.ReplaceCurrentSelection("foo", "bar", true) {
		t.Fatal("read-only ReplaceCurrentSelection must not replace")
	}
	if got := ta.GetText(); got != "foo foo" {
		t.Fatalf("read-only document changed: %q", got)
	}
	if ta.CanUndo() {
		t.Fatal("read-only replacement must not create undo history")
	}
}

// 多行选区必须一次性预留精确长度，避免复制/删除大选区时反复扩容。
func TestTextArea_ExtractRangePreallocatesExactly(t *testing.T) {
	ta := newTestTextArea("aaaa\r\nbbbbbbbb\r\ncccccccccc\r\ndddd")
	out := ta.extractRange(0, 1, 2, 3)
	want := "aaa\nbbbbbbbb\nccc"
	if string(out) != want {
		t.Fatalf("extractRange = %q, want %q", string(out), want)
	}
	if cap(out) != len(out) {
		t.Fatalf("extractRange capacity = %d, len = %d; want exact preallocation", cap(out), len(out))
	}
}

// 编辑不改变行尾词法状态时，下游行的 span 缓存必须保留；一旦状态改变则整体失效。
func TestTextArea_HighlightStateShortCircuit(t *testing.T) {
	text := strings.Repeat("plain text line\r\n", 80) + "tail"
	ta := newTestTextArea(text)
	ta.SetBounds(0, 0, 400, 320)
	ta.SetHighlighter(syntax.NewMarkdownHighlighter())
	ta.Render(render.NewCanvas(400, 320))

	if ta.statesValidTo < 2 {
		t.Fatalf("highlight states not warmed: %d", ta.statesValidTo)
	}
	if _, ok := ta.spansCache[1]; !ok {
		t.Fatal("expected cached spans for row 1")
	}
	validBefore := ta.statesValidTo

	// 插入普通字符：行尾状态不变，下游缓存保持
	ta.cursorRow, ta.cursorCol = 0, 0
	pressRune(ta, 'x')
	if ta.statesValidTo != validBefore {
		t.Fatalf("plain edit invalidated downstream states: %d -> %d", validBefore, ta.statesValidTo)
	}
	if _, ok := ta.spansCache[1]; !ok {
		t.Fatal("plain edit dropped downstream span cache")
	}

	// 打开代码围栏：行尾状态改变，下游必须失效
	ta.cursorRow, ta.cursorCol = 0, 0
	for range 3 {
		pressRune(ta, '`')
	}
	if ta.statesValidTo != 1 {
		t.Fatalf("state-changing edit should invalidate from row 1, got validTo=%d", ta.statesValidTo)
	}
	if _, ok := ta.spansCache[1]; ok {
		t.Fatal("state-changing edit left stale downstream spans")
	}
}

// ReplaceAll 批量路径必须保持视觉索引一致，并仍然只占一步撤销。
func TestTextArea_ReplaceAllBulkKeepsVisualIndexAndUndo(t *testing.T) {
	const line = "aaaa foo bbbb foo cccc dddd eeee ffff"
	original := strings.Repeat(line+"\r\n", 200) + line
	ta := newTestTextArea(original)
	ta.SetBounds(0, 0, 180, 240)
	ta.SetWordWrap(true)
	canvas := render.NewCanvas(180, 240)
	ta.Render(canvas)
	assertVisualInvariants(t, ta)

	n := ta.ReplaceAll("foo", "foofoo", false)
	if n != 402 {
		t.Fatalf("replace count = %d, want 402", n)
	}
	ta.Render(canvas)
	assertVisualInvariants(t, ta)

	if !ta.Undo() {
		t.Fatal("ReplaceAll should be undoable")
	}
	if got := ta.GetText(); got != original {
		t.Fatalf("undo after batch ReplaceAll did not restore document")
	}
	if ta.CanUndo() {
		t.Fatal("batch ReplaceAll must occupy exactly one undo step")
	}
}

// 装载必须把整篇文本放进单个 backing array，行切片只做视图。
func TestTextArea_SetTextUsesGapBuffer(t *testing.T) {
	ta := NewTextArea()
	ta.SetText("alpha\r\nbeta\r\ngamma")
	if ta.lineCount() != 3 {
		t.Fatalf("lines = %d, want 3", ta.lineCount())
	}
	for i := 0; i < ta.lineCount(); i++ {
		line := ta.line(i)
		if cap(line) != len(line) {
			t.Fatalf("line %d cap=%d len=%d, want exact capacity", i, cap(line), len(line))
		}
	}
	// 行视图共享底层数组，但 append 不能写穿到下一行
	_ = append(ta.line(0), 'x')
	if got := string(ta.line(1)); got != "beta" {
		t.Fatalf("append leaked into next line: %q", got)
	}

	if ta.lineStart[ta.lineCount()] != len([]rune("alpha\nbeta\ngamma")) {
		t.Fatalf("lineStart sentinel = %d", ta.lineStart[ta.lineCount()])
	}
	if spare := ta.gapEnd - ta.gapStart; spare < 64 {
		t.Fatalf("gap spare = %d, want >= 64", spare)
	}

	text := strings.Repeat("line one\r\n", 10000)
	allocs := testing.AllocsPerRun(3, func() { ta.SetText(text) })
	if allocs > 16 {
		t.Fatalf("SetText allocated %.0f times for 10k lines, want <= 16", allocs)
	}
}

// 编辑发生在行长中间时，line() 必须在间隙两侧都给出一致的行内容。
func TestTextArea_GapBufferLineViews(t *testing.T) {
	ta := NewTextArea()
	ta.SetText("alpha\nbeta\ngamma")
	ta.SetBounds(0, 0, 800, 600)
	if !ta.applyEdit(1, 2, 1, 2, []rune("XY")) {
		t.Fatal("applyEdit failed")
	}
	if got := ta.GetTextWithEOL("\n"); got != "alpha\nbeXYta\ngamma" {
		t.Fatalf("text = %q", got)
	}
	want := []string{"alpha", "beXYta", "gamma"}
	for r, w := range want {
		if got := string(ta.line(r)); got != w {
			t.Fatalf("line %d = %q, want %q", r, got, w)
		}
	}
	if ta.lineStart[0] != 0 || ta.lineStart[1] != 6 || ta.lineStart[2] != 13 || ta.lineStart[3] != 18 {
		t.Fatalf("lineStart = %v", ta.lineStart)
	}
	if ta.lineStart[ta.lineCount()] != ta.logicalLen() {
		t.Fatalf("sentinel=%d logicalLen=%d", ta.lineStart[ta.lineCount()], ta.logicalLen())
	}
	if !ta.Undo() {
		t.Fatal("undo failed")
	}
	if got := ta.GetTextWithEOL("\n"); got != "alpha\nbeta\ngamma" {
		t.Fatalf("after undo = %q", got)
	}
}

// assertTextStorageInvariants 校验 gap buffer 与行偏移表的结构不变量。
func assertTextStorageInvariants(t *testing.T, ta *TextArea) {
	t.Helper()
	n := ta.lineCount()
	if n < 1 {
		t.Fatalf("lineCount = %d", n)
	}
	if ta.gapStart < 0 || ta.gapEnd < ta.gapStart || ta.gapEnd > len(ta.text) {
		t.Fatalf("gap [%d,%d) out of text len %d", ta.gapStart, ta.gapEnd, len(ta.text))
	}
	if got, want := ta.lineStart[n], ta.logicalLen(); got != want {
		t.Fatalf("lineStart sentinel=%d logicalLen=%d", got, want)
	}
	if got, want := ta.logicalLen(), len([]rune(ta.GetTextWithEOL("\n"))); got != want {
		t.Fatalf("logicalLen=%d serialized=%d", got, want)
	}
	for r := 0; r < n; r++ {
		if ta.lineStart[r] > ta.lineStart[r+1] {
			t.Fatalf("lineStart not monotonic at %d: %v", r, ta.lineStart)
		}
		if r < n-1 && ta.lineStart[r+1]-ta.lineStart[r] < 1 {
			t.Fatalf("row %d has no newline separator: %v", r, ta.lineStart)
		}
		if got, want := len(ta.line(r)), ta.lineLen(r); got != want {
			t.Fatalf("line(%d) len=%d lineLen=%d", r, got, want)
		}
	}
}

// 单行编辑必须按 rune 宽度增量维护 totalWidth，并与整行重新测量完全一致。
func TestTextArea_IncrementalWidthMatchesFullMeasure(t *testing.T) {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 800, 600)
	ta.SetWordWrap(false)
	ta.SetText("abc\t中文 def")
	ta.ensureRowWidth(0)
	fm := render.GetFontManager()
	check := func(step string) {
		t.Helper()
		full := fm.MeasureRunesFace(ta.fontFace, ta.line(0))
		if !ta.layouts[0].widthOK || ta.layouts[0].totalWidth != full {
			t.Fatalf("%s: cached=%d widthOK=%v, full=%d", step, ta.layouts[0].totalWidth, ta.layouts[0].widthOK, full)
		}
	}
	check("initial")
	for i, edit := range []struct {
		sc, ec int
		ins    string
	}{
		{3, 3, "XY"},
		{1, 3, ""},
		{0, 0, "\t中"},
		{2, 2, "  "},
	} {
		ta.applyEdit(0, edit.sc, 0, edit.ec, []rune(edit.ins))
		check(fmt.Sprintf("edit %d", i))
	}
	ta.Undo()
	check("undo")
	ta.Redo()
	check("redo")
}

// 含换行的全部替换必须批量重建行偏移，并保持撤销/重做与视觉索引一致。
func TestTextArea_ReplaceAllWithNewlinesBatchesLineStarts(t *testing.T) {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 800, 600)
	ta.SetText("aXb\ncXd\neXf")
	if n := ta.ReplaceAll("X", "1\n2", false); n != 3 {
		t.Fatalf("replaced %d, want 3", n)
	}
	const want = "a1\n2b\nc1\n2d\ne1\n2f"
	if got := ta.GetTextWithEOL("\n"); got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
	if ta.lineCount() != 6 {
		t.Fatalf("lineCount = %d, want 6", ta.lineCount())
	}
	assertTextStorageInvariants(t, ta)
	ta.Render(render.NewCanvas(800, 600))
	assertVisualInvariants(t, ta)
	if !ta.Undo() {
		t.Fatal("undo failed")
	}
	if got := ta.GetTextWithEOL("\n"); got != "aXb\ncXd\neXf" {
		t.Fatalf("after undo = %q", got)
	}
	assertTextStorageInvariants(t, ta)
	if !ta.Redo() {
		t.Fatal("redo failed")
	}
	if got := ta.GetTextWithEOL("\n"); got != want {
		t.Fatalf("after redo = %q", got)
	}
	assertTextStorageInvariants(t, ta)
}

// ReplaceAll 的匹配列、removed/inserted 缓冲必须复用，避免每处匹配分配切片。
func TestTextArea_ReplaceAllNewlineAllocationsBounded(t *testing.T) {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 800, 600)
	ta.SetWordWrap(false)
	doc := strings.Repeat("a\n", 2000)
	ta.SetText(doc)
	allocs := testing.AllocsPerRun(3, func() {
		ta.SetText(doc)
		ta.ReplaceAll("a", "X\nY", false)
	})
	if allocs > 200 {
		t.Fatalf("ReplaceAll allocated %.0f times for 2000 matches, want <= 200", allocs)
	}
}

// 连续单字符输入必须并入上一条撤销记录且不产生堆分配。
func TestTextArea_ContinuousTypingAllocations(t *testing.T) {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 800, 600)
	ta.SetText("")
	ta.handleKeyPress(event.KeyData{Char: 'a'}) // 预热首字符与字形缓存
	allocs := testing.AllocsPerRun(1000, func() {
		ta.handleKeyPress(event.KeyData{Char: 'b'})
	})
	if allocs > 0.2 {
		t.Fatalf("continuous typing allocated %.2f times per key, want ~0", allocs)
	}
	if got := len(ta.line(0)); got < 1001 {
		t.Fatalf("typed text length = %d", got)
	}
}
