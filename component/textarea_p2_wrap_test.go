package component

import (
	"encoding/binary"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/syntax"
)

// 内嵌 Gnote Mono 没有真实粗体字面：GDI 合成加粗时每个字形步进比常规宽 1px，
// 且 92 个码位（带圈数字、罗马数字、星号……）在 U+2E80 以下却是两格宽。
// 依赖系统字体（Consolas 有真实粗体）的测试覆盖不到这两类排版问题。
var (
	p2FontOnce   sync.Once
	p2FontFamily string
	p2FontData   []byte
	p2FontErr    error
)

// p2EmbeddedFamily 在每个测试进程中注册一次内嵌字体，返回字体族名
func p2EmbeddedFamily(t *testing.T) string {
	t.Helper()
	p2FontOnce.Do(func() {
		data, err := os.ReadFile("../cmd/gnote/font.ttf")
		if err != nil {
			p2FontErr = err
			return
		}
		p2FontData = data
		p2FontFamily, p2FontErr = render.RegisterMemoryFont(data)
	})
	if p2FontErr != nil {
		t.Skipf("embedded font unavailable: %v", p2FontErr)
	}
	return p2FontFamily
}

// p2EmbeddedEditor 返回使用内嵌字体 16px 的编辑器（应用的真实默认配置）
func p2EmbeddedEditor(t *testing.T, width, height int32) *TextArea {
	t.Helper()
	ta := NewTextArea()
	ta.SetBounds(0, 0, width, height)
	ta.SetFontFamily(p2EmbeddedFamily(t))
	ta.SetBaseFontSize(16)
	return ta
}

// p2RightmostInk 返回 [xFrom,xTo)×[yFrom,yTo) 内最右侧的非背景像素列，没有墨迹时返回 -1
func p2RightmostInk(canvas *render.Canvas, xFrom, xTo, yFrom, yTo int32, bg graphics.Color) int32 {
	for x := xTo - 1; x >= xFrom; x-- {
		for y := yFrom; y < yTo; y++ {
			if canvas.GetPixel(x, y) != bg {
				return x
			}
		}
	}
	return -1
}

// p2AssertRowsFit 断言逻辑行 0 的每个视觉子行都不超过折行上限
func p2AssertRowsFit(t *testing.T, ta *TextArea, label string) {
	t.Helper()
	avail := ta.getAvailWrapWidth()
	for _, v := range ta.visualLines {
		if v.logicalRow != 0 {
			continue
		}
		if w := ta.measureStyledVisualX(v.logicalRow, v.startCol, v.endCol); w > avail {
			t.Errorf("%s: visual row [%d:%d) is %d px wide, wrap limit is %d px", label, v.startCol, v.endCol, w, avail)
		}
	}
}

// 回归：折行快路径只累加常规步进，含粗体关键字的行在常规宽度贴近上限时被判定为
// 无需折行，实际却画过了折行上限并被裁剪。每个视觉子行的实际宽度不得超过上限。
func TestTextArea_BoldKeywordsRespectWrapLimit(t *testing.T) {
	for n := 14; n <= 24; n++ {
		line := strings.Repeat("func ", n) // 每组 4 个粗体关键字字符 + 1 个空格
		ta := p2EmbeddedEditor(t, 1000, 400)
		ta.SetHighlighter(syntax.NewGoHighlighter())
		ta.SetText(line)
		if spans := ta.getSpansForLine(0); len(spans) == 0 || !spans[0].Bold {
			t.Fatal("precondition: 'func' must be highlighted as a bold keyword")
		}
		ta.Render(render.NewCanvas(1000, 400))
		p2AssertRowsFit(t, ta, "'func ' x"+strconv.Itoa(n))
	}
}

// 回归：像素级验证——'func ' x18 的常规宽度 900px 在 934px 上限内，
// 加粗后曾被画到画布最右端。折行上限右侧不得出现任何墨迹。
func TestTextArea_BoldLineInkStopsAtWrapLimit(t *testing.T) {
	const width, height = 1000, 400
	ta := p2EmbeddedEditor(t, width, height)
	ta.SetHighlighter(syntax.NewGoHighlighter())
	ta.SetText(strings.Repeat("func ", 18))
	canvas := render.NewCanvas(width, height)
	ta.Render(canvas)

	textStartX := ta.bounds.X + ta.getGutterWidth() + ta.margin
	limit := textStartX + ta.getAvailWrapWidth()
	if x := p2RightmostInk(canvas, limit, width, ta.margin, ta.margin+ta.lineHeight, ta.background); x >= 0 {
		t.Fatalf("bold line is painted past the wrap limit: ink at x=%d, limit x=%d", x, limit)
	}
}

// 回归：关闭折行时水平滚动上界按常规字面计算，粗体行的行尾永远滚不进视口，
// 光标被画到裁剪区之外。滚到最右后必须能看到整行，行尾光标必须落在组件内。
func TestTextArea_BoldLineTailReachableWithoutWrap(t *testing.T) {
	const width, height = 1000, 400
	ta := p2EmbeddedEditor(t, width, height)
	ta.SetWordWrap(false)
	ta.SetHighlighter(syntax.NewGoHighlighter())
	line := strings.Repeat("func ", 40) // 200 列，其中 160 列是粗体关键字
	ta.SetText(line)
	ta.SetFocused(true)
	ta.Render(render.NewCanvas(width, height))

	cols := len([]rune(line))
	styled := ta.measureStyledVisualX(0, 0, cols)
	viewW := ta.bounds.Width - ta.getGutterWidth() - ta.margin*2 - 12
	ta.scrollOffsetX = 1 << 30
	ta.clampScroll()
	if reach := ta.scrollOffsetX + viewW; reach < styled {
		t.Fatalf("unreachable tail: scrolling fully right reveals %d px, the line needs %d px", reach, styled)
	}

	ta.cursorRow, ta.cursorCol = 0, cols
	ta.ScrollCursorIntoView()
	rect, ok := ta.CaretRect()
	if !ok {
		t.Fatal("caret rect unavailable for a focused editor")
	}
	if rect.X+rect.Width > width {
		t.Fatalf("caret at end of a bold line is drawn at x=%d, outside the %d px wide editor", rect.X, width)
	}
}

// 回归：粗体字形必须留在等宽网格上。光标 X、点击命中与插入位置在粗体关键字之后
// 曾逐字偏移 1px，偏移超过半格后点击就落到前一列。
func TestTextArea_BoldKeywordsStayOnMonospaceGrid(t *testing.T) {
	const width, height = 1000, 400
	ta := p2EmbeddedEditor(t, width, height)
	ta.SetHighlighter(syntax.NewGoHighlighter())
	line := "func return x // func return"
	ta.SetText(line)
	cols := len([]rune(line))
	textStartX := ta.bounds.X + ta.getGutterWidth() + ta.margin
	ta.SetFocused(true)

	for col := 0; col <= cols; col++ {
		if got, want := ta.measureStyledVisualX(0, 0, col), int32(col)*ta.monoWidth; got != want {
			t.Fatalf("col %d: styled X = %d, want the grid position %d", col, got, want)
		}

		ta.cursorRow, ta.cursorCol = 0, col
		rect, ok := ta.CaretRect()
		if !ok {
			t.Fatalf("col %d: caret rect unavailable", col)
		}
		if rect.X != textStartX+int32(col)*ta.monoWidth {
			t.Fatalf("col %d: caret drawn at x=%d, want %d", col, rect.X, textStartX+int32(col)*ta.monoWidth)
		}

		// 鼠标点击：格边界归右列；边界左侧 1px 在左格的右半边，同样吸附到本列；
		// 左格中点及其左侧 1px 归左列
		for _, c := range []struct {
			dx   int32
			want int
		}{{0, 0}, {1, 0}, {-1, 0}, {-ta.monoWidth / 2, -1}, {-ta.monoWidth/2 - 1, -1}} {
			if col+c.want < 0 || col+c.want > cols {
				continue
			}
			x := textStartX + int32(col)*ta.monoWidth + c.dx
			y := ta.bounds.Y + ta.margin + ta.lineHeight/2
			ta.lastClickAt = time.Time{} // 同一格的连续点击不应被识别为双击/三击
			ta.OnEvent(event.NewEvent(event.EventMouseDown, event.MouseData{X: x, Y: y, Button: event.MouseButtonLeft}))
			ta.OnEvent(event.NewEvent(event.EventMouseUp, event.MouseData{X: x, Y: y, Button: event.MouseButtonLeft}))
			if ta.cursorCol != col+c.want {
				t.Fatalf("click at x=%d (col %d %+d px): cursorCol = %d, want %d", x, col, c.dx, ta.cursorCol, col+c.want)
			}
		}
	}
}

// 回归：等宽上界把 U+2E80 以下的所有字符都按 1 格计，而内嵌字体的带圈数字、
// 罗马数字、星号是两格宽（U+22EF 等是 1.6 格），这类行不折行就被画出了右边界。
func TestTextArea_WideSymbolsBelowCJKWrapWithinViewport(t *testing.T) {
	const width, height = 800, 400
	for _, c := range []struct{ name, text string }{
		{"circled digits", strings.Repeat("a", 58) + strings.Repeat("\u2460", 15)},   // 880px，按 1 格估算只有 730px
		{"roman numerals", strings.Repeat("\u2160", 80)},                             // 1600px，按 1 格估算 800px
		{"stars", strings.Repeat("a", 66) + strings.Repeat("\u2605", 12)},            // 900px，按 1 格估算 780px
		{"midline ellipsis", strings.Repeat("a", 58) + strings.Repeat("\u22ef", 15)}, // 820px，按 1 格估算 730px
		{"cjk control", strings.Repeat("a", 58) + strings.Repeat("\u4e2d", 15)},      // 一直按 2 格估算的对照组
	} {
		ta := p2EmbeddedEditor(t, width, height)
		ta.SetText(c.text)
		canvas := render.NewCanvas(width, height)
		ta.Render(canvas)
		p2AssertRowsFit(t, ta, c.name)
		if got, want := p2VisualRowCount(ta), 2; got < want {
			t.Errorf("%s: %d visual rows, want at least %d", c.name, got, want)
		}
		textStartX := ta.bounds.X + ta.getGutterWidth() + ta.margin
		limit := textStartX + ta.getAvailWrapWidth()
		if x := p2RightmostInk(canvas, limit, width, ta.margin, ta.margin+ta.lineHeight, ta.background); x >= 0 {
			t.Errorf("%s: painted past the wrap limit: ink at x=%d, limit x=%d", c.name, x, limit)
		}
	}
}

// 随机内容属性测试：粗体关键字、CJK、两格/1.6 格符号与 ASCII 混排时，
// 任何宽度下每个视觉子行都不得超过折行上限。
func TestTextArea_WrappedRowsNeverExceedWrapWidth(t *testing.T) {
	rng := rand.New(rand.NewPCG(20261126, 1))
	tokens := []string{"func", "return", "if", "nil", "main", "(", ")", "{", " ", "  ", "\t", "中", "文，", "①", "Ⅰ", "★", "⋯", "a", "bb", "ccc"}
	for iter := range 300 {
		var line strings.Builder
		for n := 20 + rng.IntN(120); n > 0; n-- {
			line.WriteString(tokens[rng.IntN(len(tokens))])
		}
		ta := p2EmbeddedEditor(t, int32(160+rng.IntN(500)), 300)
		if iter%2 == 0 {
			ta.SetHighlighter(syntax.NewGoHighlighter())
		}
		ta.SetText(line.String())
		ta.recomputeVisualLines()
		p2AssertRowsFit(t, ta, "line "+strconv.Itoa(iter)+" "+line.String())
	}
}

// 回归：等宽上界必须是上界。对内嵌字体 cmap 中的每个码位，真实步进超过可用宽度时
// 上界不得判定“放得下”。U+2000–U+2E7F 一律按宽字符计，保证重新生成字体后仍然成立。
func TestTextArea_LineFitsUpperBoundCoversEmbeddedFont(t *testing.T) {
	ta := p2EmbeddedEditor(t, 800, 400)
	fm := render.GetFontManager()
	for i, r := range p2FontCodePoints(t, p2FontData) {
		if r >= 0x3000 && i%7 != 0 {
			continue // CJK 统一表意文字按步长抽样，其余码位全量检查
		}
		adv := fm.RuneWidthFace(ta.fontFace, r)
		if ta.lineFitsUpperBound([]rune{r}, adv-1) {
			t.Fatalf("U+%04X advances %d px, but the upper bound claims it fits in %d px", r, adv, adv-1)
		}
	}
}

// 命中测试在两格/1.6 格符号上必须可逆：每个字符边界的 X 反查回同一列，
// 格内左半归左列、右半归右列，真实鼠标点击落在同一列。
func TestTextArea_HitTestRoundTripWithWideSymbols(t *testing.T) {
	const width, height = 420, 300
	ta := p2EmbeddedEditor(t, width, height)
	alphabet := []rune("a①Ⅰ★⋯中b")
	line := make([]rune, 0, 120)
	for i := range 120 {
		line = append(line, alphabet[i%len(alphabet)])
	}
	ta.SetText(string(line))
	ta.recomputeVisualLines()

	for _, v := range ta.visualLines {
		for col := v.startCol; col <= v.endCol; col++ {
			x := ta.measureStyledVisualX(0, v.startCol, col)
			if got := ta.getStyledVisualColFromX(0, v.startCol, v.endCol, x); got != col {
				t.Fatalf("row [%d:%d) col %d: X=%d maps back to col %d", v.startCol, v.endCol, col, x, got)
			}
			if col == v.startCol {
				continue
			}
			half := x - (x-ta.measureStyledVisualX(0, v.startCol, col-1))/2
			if got := ta.getStyledVisualColFromX(0, v.startCol, v.endCol, half); got != col-1 {
				t.Fatalf("row [%d:%d) midpoint of col %d (x=%d): got col %d, want %d", v.startCol, v.endCol, col-1, half, got, col-1)
			}
			if got := ta.getStyledVisualColFromX(0, v.startCol, v.endCol, half+1); got != col {
				t.Fatalf("row [%d:%d) just past the midpoint of col %d (x=%d): got col %d, want %d", v.startCol, v.endCol, col-1, half+1, got, col)
			}
		}
	}

	textStartX := ta.bounds.X + ta.getGutterWidth() + ta.margin
	for col := 0; col <= len(line); col++ {
		v := p2VisualRowAt(ta, col)
		x := textStartX + ta.measureStyledVisualX(0, v.startCol, col)
		y := ta.bounds.Y + ta.margin + int32(p2VisualRowIndex(ta, v))*ta.lineHeight + ta.lineHeight/2
		ta.OnEvent(event.NewEvent(event.EventMouseDown, event.MouseData{X: x, Y: y, Button: event.MouseButtonLeft}))
		ta.OnEvent(event.NewEvent(event.EventMouseUp, event.MouseData{X: x, Y: y, Button: event.MouseButtonLeft}))
		if ta.cursorRow != 0 || ta.cursorCol != col {
			t.Fatalf("click at the boundary of col %d (x=%d): cursor = (%d,%d)", col, x, ta.cursorRow, ta.cursorCol)
		}
	}
}

// p2CountingHighlighter 统计分词次数：对超长行每次分词都是 O(行长)
type p2CountingHighlighter struct {
	syntax.Highlighter
	calls int
}

func (h *p2CountingHighlighter) HighlightLine(row int, line []rune, state int) ([]syntax.Span, int) {
	h.calls++
	return h.Highlighter.HighlightLine(row, line, state)
}

// 回归：单行编辑后，失效处理为了得到新的行尾状态已经把编辑行整行分词了一次，
// 随后的光标测量与绘制不得再分词。超长高亮行上每键因此分词两次，开销翻倍。
func TestTextArea_EditedRowIsTokenisedOncePerKey(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		ta := newTestTextArea("func f(x int) int { return x } // note")
		ta.SetWordWrap(wrap)
		hl := &p2CountingHighlighter{Highlighter: syntax.NewGoHighlighter()}
		ta.SetHighlighter(hl)
		canvas := render.NewCanvas(800, 600)
		ta.Render(canvas)

		for i := range 3 {
			calls := hl.calls
			pressRune(ta, 'x')
			ta.Render(canvas)
			ta.Render(canvas)
			if got := hl.calls - calls; got != 1 {
				t.Errorf("wrap=%v key %d: %d tokenisations for one key press and repaint, want 1", wrap, i, got)
			}

			// 缓存里必须是编辑后的新 spans
			fresh, _ := hl.Highlighter.HighlightLine(0, ta.line(0), 0)
			if cached, ok := ta.spansCache[0]; !ok || len(cached) != len(fresh) {
				t.Errorf("wrap=%v key %d: cached spans = %d (present %v), fresh spans = %d", wrap, i, len(cached), ok, len(fresh))
			} else {
				for k := range fresh {
					if cached[k] != fresh[k] {
						t.Errorf("wrap=%v key %d: cached span %d = %+v, want %+v", wrap, i, k, cached[k], fresh[k])
						break
					}
				}
			}
			pressKey(ta, 0x08, false, false)
		}
	}
}

// 超长高亮行上的键入 + 退格 + 渲染：跟踪“编辑行每键只分词一次”的效果。
// 在行尾编辑，前缀检查点的续算可以忽略，剩下的 O(行长) 开销只有分词。
func BenchmarkTextAreaLongHighlightedLineKeystroke(b *testing.B) {
	ta := NewTextArea()
	ta.SetBounds(0, 0, 1560, 920)
	ta.SetWordWrap(false)
	ta.SetText(strings.Repeat("abcdefghij", 20000))
	ta.SetHighlighter(syntax.NewGoHighlighter())
	ta.SetFocused(true)
	ta.cursorRow, ta.cursorCol = 0, len(ta.line(0))
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

// p2VisualRowCount 返回逻辑行 0 的视觉子行数
func p2VisualRowCount(ta *TextArea) int {
	n := 0
	for _, v := range ta.visualLines {
		if v.logicalRow == 0 {
			n++
		}
	}
	return n
}

// p2VisualRowAt 返回包含列 col 的视觉子行（折行点归下一子行）
func p2VisualRowAt(ta *TextArea, col int) visualLine {
	for _, v := range ta.visualLines {
		if v.logicalRow == 0 && col >= v.startCol && (col < v.endCol || v.isLastSub) {
			return v
		}
	}
	return ta.visualLines[len(ta.visualLines)-1]
}

func p2VisualRowIndex(ta *TextArea, v visualLine) int {
	for i, cur := range ta.visualLines {
		if cur == v {
			return i
		}
	}
	return -1
}

// p2FontCodePoints 解析字体 cmap，返回其覆盖的 BMP 码位（升序）
func p2FontCodePoints(t *testing.T, data []byte) []rune {
	t.Helper()
	numTables := int(binary.BigEndian.Uint16(data[4:6]))
	var cmap []byte
	for i := 0; i < numTables; i++ {
		entry := 12 + i*16
		if entry+16 > len(data) {
			break
		}
		if string(data[entry:entry+4]) == "cmap" {
			off := binary.BigEndian.Uint32(data[entry+8 : entry+12])
			length := binary.BigEndian.Uint32(data[entry+12 : entry+16])
			cmap = data[off : off+length]
			break
		}
	}
	if cmap == nil {
		t.Fatal("embedded font has no cmap table")
	}

	var best []rune
	for i := 0; i < int(binary.BigEndian.Uint16(cmap[2:4])); i++ {
		// 子表记录：platformID(2) encodingID(2) offset(4)，相对 cmap 表首
		sub := cmap[binary.BigEndian.Uint32(cmap[8+i*8:12+i*8]):]
		var cps []rune
		switch binary.BigEndian.Uint16(sub[0:2]) {
		case 4: // BMP分段映射
			segX2 := int(binary.BigEndian.Uint16(sub[6:8]))
			end, start, delta, rangeOff := 14, 14+segX2+2, 14+2*segX2+2, 14+3*segX2+2
			for s := 0; s < segX2/2; s++ {
				first := rune(binary.BigEndian.Uint16(sub[start+s*2:]))
				idDelta := binary.BigEndian.Uint16(sub[delta+s*2:])
				idRangeOffset := binary.BigEndian.Uint16(sub[rangeOff+s*2:])
				for c := first; c <= rune(binary.BigEndian.Uint16(sub[end+s*2:])); c++ {
					glyph := uint16(c) + idDelta
					if idRangeOffset != 0 {
						if glyph = binary.BigEndian.Uint16(sub[rangeOff+s*2+int(idRangeOffset)+2*int(c-first):]); glyph != 0 {
							glyph += idDelta
						}
					}
					if glyph != 0 {
						cps = append(cps, c)
					}
				}
			}
		case 12: // 分段连续映射
			for g := 0; g < int(binary.BigEndian.Uint32(sub[12:16])); g++ {
				rec := sub[16+g*12:]
				for c := binary.BigEndian.Uint32(rec[0:4]); c <= binary.BigEndian.Uint32(rec[4:8]) && c <= 0xFFFF; c++ {
					cps = append(cps, rune(c))
				}
			}
		}
		if len(cps) > len(best) {
			best = cps
		}
	}
	if len(best) == 0 {
		t.Fatal("embedded font cmap yields no code points")
	}
	return best
}
