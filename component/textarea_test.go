package component

import (
	"os"
	"testing"

	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/syntax"
)

func TestTextArea_BasicEditing(t *testing.T) {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 400, 300)

	// 测试设置文本
	ta.SetText("Hello\r\nWorld")
	if ta.GetLineCount() != 2 {
		t.Fatalf("expected 2 lines, got %d", ta.GetLineCount())
	}
	if ta.GetText() != "Hello\r\nWorld" {
		t.Fatalf("expected 'Hello\\r\\nWorld', got %q", ta.GetText())
	}

	// 模拟聚焦
	ta.OnEvent(event.NewEvent(event.EventFocus, nil))
	if !ta.IsFocused() {
		t.Fatalf("expected focused = true")
	}

	// 模拟键盘输入字符
	ta.OnEvent(event.NewEvent(event.EventKeyPress, event.KeyData{Char: '!'}))
	if ta.GetText() != "!Hello\r\nWorld" {
		t.Fatalf("expected '!Hello\\r\\nWorld', got %q", ta.GetText())
	}

	// 模拟回车换行
	ta.OnEvent(event.NewEvent(event.EventKeyDown, event.KeyData{KeyCode: 0x0D})) // Enter
	if ta.GetLineCount() != 3 {
		t.Fatalf("expected 3 lines after enter, got %d", ta.GetLineCount())
	}

	// 模拟 Undo
	if !ta.Undo() {
		t.Fatalf("undo failed")
	}
	if ta.GetLineCount() != 2 {
		t.Fatalf("expected 2 lines after undo, got %d", ta.GetLineCount())
	}

	// 模拟 Redo
	if !ta.Redo() {
		t.Fatalf("redo failed")
	}
	if ta.GetLineCount() != 3 {
		t.Fatalf("expected 3 lines after redo, got %d", ta.GetLineCount())
	}
}

func TestTextArea_SelectionAndDelete(t *testing.T) {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 500, 400)
	ta.SetText("Line 1\r\nLine 2\r\nLine 3")

	ta.OnEvent(event.NewEvent(event.EventFocus, nil))

	// Ctrl+A 全选
	ta.OnEvent(event.NewEvent(event.EventKeyDown, event.KeyData{KeyCode: 'A', Ctrl: true}))
	selected := ta.getSelectedText()
	if selected != "Line 1\r\nLine 2\r\nLine 3" {
		t.Fatalf("expected all text selected, got %q", selected)
	}

	// 按 Backspace 删除选区
	ta.OnEvent(event.NewEvent(event.EventKeyDown, event.KeyData{KeyCode: 0x08}))
	if ta.GetText() != "" {
		t.Fatalf("expected empty text after deleting selection, got %q", ta.GetText())
	}

	// 再次 Undo 恢复
	ta.Undo()
	if ta.GetText() != "Line 1\r\nLine 2\r\nLine 3" {
		t.Fatalf("expected restored text after undo, got %q", ta.GetText())
	}
}

func TestTextArea_Zoom(t *testing.T) {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 500, 400)
	ta.SetText("Hello World")

	var reportedZoom int
	ta.SetOnZoomChange(func(percent int) {
		reportedZoom = percent
	})

	if ta.GetZoomPercent() != 100 {
		t.Fatalf("expected initial zoom 100%%, got %d%%", ta.GetZoomPercent())
	}

	// 放大
	ta.ZoomIn()
	if ta.GetZoomPercent() != 110 {
		t.Fatalf("expected zoom 110%%, got %d%%", ta.GetZoomPercent())
	}
	if reportedZoom != 110 {
		t.Fatalf("expected callback 110, got %d", reportedZoom)
	}

	// 缩小
	ta.ZoomOut()
	if ta.GetZoomPercent() != 100 {
		t.Fatalf("expected zoom 100%%, got %d%%", ta.GetZoomPercent())
	}

	// 滚轮缩放 (Ctrl + MouseWheel)
	ta.OnEvent(event.NewEvent(event.EventMouseWheel, event.MouseData{
		X: 50, Y: 50, Delta: 120, Ctrl: true,
	}))
	if ta.GetZoomPercent() != 110 {
		t.Fatalf("expected zoom 110%% after wheel up, got %d%%", ta.GetZoomPercent())
	}

	// 快捷键缩放 (Ctrl + '+')
	ta.OnEvent(event.NewEvent(event.EventFocus, nil))
	ta.OnEvent(event.NewEvent(event.EventKeyDown, event.KeyData{
		KeyCode: '+', Ctrl: true,
	}))
	if ta.GetZoomPercent() != 120 {
		t.Fatalf("expected zoom 120%% after Ctrl++, got %d%%", ta.GetZoomPercent())
	}

	// 快捷键重置 (Ctrl + '0')
	ta.OnEvent(event.NewEvent(event.EventKeyDown, event.KeyData{
		KeyCode: '0', Ctrl: true,
	}))
	if ta.GetZoomPercent() != 100 {
		t.Fatalf("expected zoom 100%% after Ctrl+0, got %d%%", ta.GetZoomPercent())
	}
}

func TestTextArea_WithHighlighter(t *testing.T) {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 600, 400)
	ta.SetText("# Title\r\n```go\r\ncode\r\n```\r\nRegular text")

	md := syntax.NewMarkdownHighlighter()
	ta.SetHighlighter(md)

	if ta.GetHighlighter() != md {
		t.Fatalf("expected highlighter to match")
	}

	// 渲染触发高亮执行
	canvas := render.NewCanvas(600, 400)
	ta.Render(canvas)

	// 验证行状态缓存已正确建立
	if len(ta.lineStates) != 5 {
		t.Fatalf("expected 5 lineStates, got %d", len(ta.lineStates))
	}
	if ta.lineStates[1] != syntax.StateFencedCodeBlock {
		t.Fatalf("expected line 1 to be inside code block")
	}
	if ta.lineStates[2] != syntax.StateFencedCodeBlock {
		t.Fatalf("expected line 2 to be inside code block")
	}
	if ta.lineStates[3] != syntax.StateNormal {
		t.Fatalf("expected line 3 to be StateNormal")
	}
}

func TestTextArea_WordWrap(t *testing.T) {
	ta := NewTextArea()
	if !ta.IsWordWrap() {
		t.Fatalf("expected WordWrap to be true by default")
	}

	// 设置一个宽度较窄的视口 (例如 250px)
	ta.SetBounds(0, 0, 250, 400)
	longText := "这是一段非常非常长的单行中文测试文本，用户没有手动输入换行符，它应该自动折行显示在多行中，避免超出窗口右侧。"
	ta.SetText(longText)

	canvas := render.NewCanvas(250, 400)
	ta.Render(canvas)

	// 开启换行状态下，单行长文本应该被折为多个视觉行
	if len(ta.visualLines) <= 1 {
		t.Fatalf("expected visualLines > 1 for long single line with word wrap, got %d", len(ta.visualLines))
	}
	visualCountWithWrap := len(ta.visualLines)

	// 验证每个视觉子行都属于同一逻辑行 0
	for _, v := range ta.visualLines {
		if v.logicalRow != 0 {
			t.Fatalf("expected logicalRow 0, got %d", v.logicalRow)
		}
	}
	// 首个子行应标记为 isFirstSub，最后一个标记为 isLastSub
	if !ta.visualLines[0].isFirstSub {
		t.Fatalf("expected first visual line to have isFirstSub == true")
	}
	if !ta.visualLines[visualCountWithWrap-1].isLastSub {
		t.Fatalf("expected last visual line to have isLastSub == true")
	}

	// 测试关闭自动换行
	ta.SetWordWrap(false)
	ta.Render(canvas)
	if len(ta.visualLines) != 1 {
		t.Fatalf("expected exactly 1 visualLine with word wrap disabled, got %d", len(ta.visualLines))
	}

	// 重新开启自动换行
	ta.SetWordWrap(true)
	ta.Render(canvas)
	if len(ta.visualLines) != visualCountWithWrap {
		t.Fatalf("expected %d visualLines after re-enabling wrap, got %d", visualCountWithWrap, len(ta.visualLines))
	}

	// 测试光标在折行子行之间的垂直导航
	ta.OnEvent(event.NewEvent(event.EventFocus, nil))
	// 初始光标在 (0, 0)
	if ta.cursorRow != 0 || ta.cursorCol != 0 {
		t.Fatalf("expected initial cursor at (0, 0), got (%d, %d)", ta.cursorRow, ta.cursorCol)
	}

	// 按下向下键 Down Arrow (0x28)
	ta.OnEvent(event.NewEvent(event.EventKeyDown, event.KeyData{KeyCode: 0x28}))
	// 光标应该移动到第 2 个视觉子行，但逻辑行依然是 0
	if ta.cursorRow != 0 {
		t.Fatalf("expected logical row 0 after Down arrow on wrapped line, got %d", ta.cursorRow)
	}
	v1 := ta.visualLines[1]
	if ta.cursorCol < v1.startCol || ta.cursorCol > v1.endCol {
		t.Fatalf("expected cursorCol within second visual line [%d, %d], got %d", v1.startCol, v1.endCol, ta.cursorCol)
	}

	// 按下向上键 Up Arrow (0x26)，应该回到第 1 个视觉子行
	ta.OnEvent(event.NewEvent(event.EventKeyDown, event.KeyData{KeyCode: 0x26}))
	if ta.cursorRow != 0 {
		t.Fatalf("expected logical row 0 after Up arrow, got %d", ta.cursorRow)
	}
	v0 := ta.visualLines[0]
	if ta.cursorCol < v0.startCol || ta.cursorCol > v0.endCol {
		t.Fatalf("expected cursorCol within first visual line [%d, %d], got %d", v0.startCol, v0.endCol, ta.cursorCol)
	}
}

func TestTextArea_EmbeddedFontAlignment(t *testing.T) {
	data, err := os.ReadFile("../examples/notepad/font.ttf")
	if err != nil {
		t.Skipf("skipping TestTextArea_EmbeddedFontAlignment, font.ttf not found: %v", err)
	}

	family, err := render.RegisterMemoryFont(data)
	if err != nil {
		t.Fatalf("RegisterMemoryFont failed: %v", err)
	}

	ta := NewTextArea()
	ta.SetBounds(0, 0, 1000, 600)
	font16 := graphics.NewFont(family, 16)
	ta.SetFont(font16)

	if ta.lineHeight <= 0 || ta.charWidth <= 0 {
		t.Fatalf("invalid metrics: lineHeight=%d, charWidth=%d", ta.lineHeight, ta.charWidth)
	}

	testStr := "goui 记事本: 零CGO架构, 内存字体内嵌! 1234567890 []{}+=-_ *&^%$#@! 汉字宽度测试。"
	ta.SetText(testStr)

	runes := []rune(testStr)
	fm := render.GetFontManager()

	// 1. 逐字符检验 measureTextX 与累加字符步进的 0 误差一致性
	cumWidth := int32(0)
	for i, r := range runes {
		col := i + 1
		rw := fm.GetRuneWidth(font16, r)
		cumWidth += rw
		measuredX := ta.measureTextX(runes, col)
		if measuredX != cumWidth {
			t.Fatalf("col %d ('%c'): measuredX %d != cumWidth %d (diff %d)", col, r, measuredX, cumWidth, measuredX-cumWidth)
		}

		// 2. 检验 getColFromX 反查精确度
		colExact := ta.getColFromX(runes, cumWidth)
		if colExact != col {
			t.Fatalf("getColFromX at exact char edge %d returned %d, expected %d", cumWidth, colExact, col)
		}

		// 点击右半部分应该对齐到下一个字符（右边缘）
		if rw > 2 {
			colRightHalf := ta.getColFromX(runes, cumWidth-1)
			if colRightHalf != col {
				t.Fatalf("getColFromX at right half (%d) returned %d, expected %d", cumWidth-1, colRightHalf, col)
			}
			colLeftHalf := ta.getColFromX(runes, cumWidth-rw+1)
			if colLeftHalf != i {
				t.Fatalf("getColFromX at left half (%d) returned %d, expected %d", cumWidth-rw+1, colLeftHalf, i)
			}
		}
	}

	// 3. 模拟鼠标在各字符位置点击，验证光标定位完全精准
	gutterW := ta.getGutterWidth()
	textStartX := ta.bounds.X + gutterW + ta.margin

	ta.OnEvent(event.NewEvent(event.EventFocus, nil))

	for i := 0; i <= len(runes); i++ {
		targetX := textStartX + ta.measureTextX(runes, i)
		targetY := ta.bounds.Y + ta.margin + ta.lineHeight/2

		ta.OnEvent(event.NewEvent(event.EventMouseDown, event.MouseData{
			X: targetX,
			Y: targetY,
		}))

		if ta.cursorCol != i {
			t.Fatalf("click at X=%d: expected cursorCol=%d, got %d", targetX, i, ta.cursorCol)
		}
	}

	// 4. 验证缩放（125% / 150%）后光标与字符度量依然一致
	for _, zoom := range []float64{1.25, 1.5} {
		ta.SetZoom(zoom)
		zoomedFont := ta.font
		cumW := int32(0)
		for i, r := range runes {
			col := i + 1
			rw := fm.GetRuneWidth(zoomedFont, r)
			cumW += rw
			measured := ta.measureTextX(runes, col)
			if measured != cumW {
				t.Fatalf("zoom %.2f, col %d ('%c'): measured %d != cumW %d", zoom, col, r, measured, cumW)
			}
		}
	}

	// 5. 渲染测试
	canvas := render.NewCanvas(1000, 600)
	canvas.Clear(graphics.ColorWhite)
	ta.Render(canvas)
	if !canvas.IsDirty() {
		t.Fatalf("expected canvas dirty after render")
	}
}

func TestTextArea_MarkdownBoldAndInlineCodeAlignment(t *testing.T) {
	data, err := os.ReadFile("../examples/notepad/font.ttf")
	if err != nil {
		t.Skip("font.ttf not found")
	}
	fam, err := render.RegisterMemoryFont(data)
	if err != nil {
		t.Fatal(err)
	}

	ta := NewTextArea()
	ta.SetBounds(0, 0, 1000, 600)
	font16 := graphics.NewFont(fam, 16)
	ta.SetFont(font16)
	md := syntax.NewMarkdownHighlighter()
	ta.SetHighlighter(md)

	// 测试行包含 **加粗** 以及 `行内代码`
	testLine := "**内置多语言高亮**: 纯 Go 词法分词状态机，支持 `Ctrl + 滚轮` 与 `Ctrl + +/-/0` 实时缩放"
	ta.SetText(testLine)

	runes := []rune(testLine)
	spans := ta.getSpansForLine(0)
	if len(spans) == 0 {
		t.Fatalf("expected non-empty spans for markdown line")
	}

	// 1. 验证加粗包裹的闭合标记 ** 紧跟在加粗文字之后，0 重叠
	// ** 开头: 0..2
	// 内置多语言高亮: 2..9 (Bold)
	// ** 结尾: 9..11
	boldStartCol := 2
	boldEndCol := 9
	closeMarkerStartCol := 9
	closeMarkerEndCol := 11

	boldStartX := ta.measureStyledVisualX(0, 0, boldStartCol)
	boldEndX := ta.measureStyledVisualX(0, 0, boldEndCol)
	closeMarkerStartX := ta.measureStyledVisualX(0, 0, closeMarkerStartCol)
	closeMarkerEndX := ta.measureStyledVisualX(0, 0, closeMarkerEndCol)

	if boldEndX != closeMarkerStartX {
		t.Fatalf("mismatch at bold boundary: boldEndX=%d != closeMarkerStartX=%d (overlap/gap=%d)",
			boldEndX, closeMarkerStartX, closeMarkerStartX-boldEndX)
	}
	if boldEndX <= boldStartX {
		t.Fatalf("expected boldEndX (%d) > boldStartX (%d)", boldEndX, boldStartX)
	}
	if closeMarkerEndX <= closeMarkerStartX {
		t.Fatalf("expected closeMarkerEndX (%d) > closeMarkerStartX (%d)", closeMarkerEndX, closeMarkerStartX)
	}

	// 2. 验证行内代码 `Ctrl + 滚轮` 的闭合反引号紧跟代码内容之后
	// 找到 `Ctrl + 滚轮` 的 spans
	var codeStartCol, codeEndCol, codeCloseCol int
	for i, sp := range spans {
		if sp.Color == md.GetColors().InlineCodeText {
			codeStartCol = sp.StartCol
			codeEndCol = sp.EndCol
			if i+1 < len(spans) {
				codeCloseCol = spans[i+1].EndCol
			}
			break
		}
	}
	if codeStartCol > 0 && codeEndCol > codeStartCol {
		codeContentEndX := ta.measureStyledVisualX(0, 0, codeEndCol)
		codeCloseStartX := ta.measureStyledVisualX(0, 0, codeEndCol)
		if codeContentEndX != codeCloseStartX {
			t.Fatalf("mismatch at inline code boundary: contentEnd=%d != closeStart=%d", codeContentEndX, codeCloseStartX)
		}
		if codeCloseCol > codeEndCol {
			codeCloseEndX := ta.measureStyledVisualX(0, 0, codeCloseCol)
			if codeCloseEndX <= codeCloseStartX {
				t.Fatalf("expected codeCloseEndX (%d) > codeCloseStartX (%d)", codeCloseEndX, codeCloseStartX)
			}
		}
	}

	// 3. 验证光标在第 9 列（加粗文本之后、右边 ** 之前）的物理位置绝不会插在加粗汉字体内
	gutterW := ta.getGutterWidth()
	textStartX := ta.bounds.X + gutterW + ta.margin
	cursorX := textStartX + ta.measureStyledVisualX(0, 0, 9)
	if cursorX != textStartX+boldEndX {
		t.Fatalf("cursor at col 9: expected X=%d, got %d", textStartX+boldEndX, cursorX)
	}

	// 4. 验证鼠标点击各字符边界的精准反查
	for col := 0; col <= len(runes); col++ {
		targetX := textStartX + ta.measureStyledVisualX(0, 0, col)
		foundCol := ta.getStyledVisualColFromX(0, 0, len(runes), targetX-textStartX)
		if foundCol != col {
			t.Fatalf("col %d: targetX=%d, getStyledVisualColFromX returned %d", col, targetX, foundCol)
		}
	}

	// 5. 渲染测试
	canvas := render.NewCanvas(1000, 600)
	canvas.Clear(graphics.ColorWhite)
	ta.Render(canvas)
	if !canvas.IsDirty() {
		t.Fatalf("expected canvas dirty after render")
	}
}
