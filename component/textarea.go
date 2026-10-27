package component

import (
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
	"unsafe"

	"github.com/jacksalad/gnote/clipboard"
	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/syntax"
	"github.com/jacksalad/gnote/theme"
)

// textPos 文档中的行列坐标 (0-indexed)
type textPos struct {
	row, col int
}

// selectionState 选区快照，用于撤销/重做时精确恢复选区
type selectionState struct {
	active    bool
	anchorRow int
	anchorCol int
	cursorRow int
	cursorCol int
}

// editOp 单次区间替换：把 row 行 [col, ...) 处的 removed 替换为 inserted
type editOp struct {
	row, col          int
	removed, inserted []rune
}

// editRecord 一次可撤销编辑。
// 批量替换（全部替换）会包含多个 editOp，但整体只占一个撤销步骤；
// ops 按实际应用顺序记录，撤销时逆序回放。
type editRecord struct {
	ops                       []editOp
	cursorBefore, cursorAfter textPos
	selBefore, selAfter       selectionState
	revBefore, revAfter       uint64
}

const (
	// 超长行阈值：低于该长度时逐 rune 测量足够快，无需构建检查点
	longLinePrefixThreshold = 2048
	// 前缀宽度检查点间隔（列）
	prefixStep = 256
)

// linePrefix 超长行的累计宽度检查点：widths[k] 表示 [0, k*prefixStep) 的像素宽度。
// validTo 之前（含）的检查点有效；文本编辑只截断到编辑列，末尾输入可增量续算。
type linePrefix struct {
	widths  []int32
	validTo int
}

// rowPatch ReplaceAll 批量路径中的单行改动
type rowPatch struct {
	row int
	op  editOp
}

// lineLayout 单个逻辑行的宽度缓存。
// 折行点不再常驻：仅在重建视觉行时按需计算，使每行开销从 40B 降到 8B，
// 行插入/删除时的尾部搬移也同比缩小。
type lineLayout struct {
	totalWidth int32 // 整行文本像素宽度（与折行宽度无关，仅受字体影响）
	widthOK    bool  // totalWidth 是否对当前字体有效
}

// visualLine 表示经过自动换行计算后，在视口中实际排版呈现的单个物理行/视觉行
type visualLine struct {
	logicalRow int  // 对应的逻辑行号 (0-indexed)
	startCol   int  // 该子行在逻辑行中的起始字符列 (0-indexed, inclusive)
	endCol     int  // 该子行在逻辑行中的结束字符列 (0-indexed, exclusive)
	isFirstSub bool // 是否为该逻辑行的第一子行 (用于决定是否渲染行号)
	isLastSub  bool // 是否为该逻辑行的最后子行
}

// TextArea 多行文本编辑器组件
// 支持行虚拟化渲染、自动换行、等宽/变宽排版、行号槽、文本选区、剪贴板交互与撤销/重做
type TextArea struct {
	BaseComponent

	// 文本数据模型：text 是带间隙（gap）的连续 rune 缓冲，
	// lineStart 记录每行起始逻辑偏移（len = 行数+1，末位 = 逻辑 rune 总数）。
	// 行视图零拷贝；间隙始终落在行边界（line() 读取时按需挪动）。
	text      []rune
	lineStart []int
	gapStart  int
	gapEnd    int

	// 自动换行与视觉行模型
	wordWrap        bool                // 是否开启自动换行 (默认 true，单行长文本自动换行不超出窗口右侧)
	visualLines     []visualLine        // 视口排版后的所有视觉行列表
	logicalToVisual []int               // 每个逻辑行对应的首个视觉行索引（长度 = len(lines)+1，末位为总数）
	layouts         []lineLayout        // 与 lines 一一对应的增量排版缓存
	wrappedWidth    int32               // 最近一次折行计算使用的可用宽度
	visualState     int                 // 视觉行索引状态：visualClean / visualPartial / visualFull
	dirtyFrom       int                 // 局部重算起始逻辑行
	dirtyRows       int                 // 局部重算覆盖的逻辑行数（新行号）
	dirtyRowDelta   int                 // 局部重算涉及的行数增量
	dirtyOldStart   int                 // 局部重算前的视觉行起点
	dirtyOldEnd     int                 // 局部重算前的视觉行终点
	visualScratch   []visualLine        // 局部重算临时缓冲
	breakScratch    []int32             // 单行折行点临时缓冲
	longPrefixes    map[int]*linePrefix // 超长行每 prefixStep 列的累计宽度检查点（仅关闭折行时使用）
	preferredX      int32               // 上下移动光标时记忆的物理 X 像素坐标

	// 语法高亮
	highlighter   syntax.Highlighter
	lineStates    []int                 // 记录每行结束时的状态缓存 (用于跨行代码块等)
	statesValidTo int                   // lineStates[0:statesValidTo] 为有效缓存
	spansCache    map[int][]syntax.Span // 语法高亮 spans 行级缓存（按行号缓存可见行）

	// 光标位置（基于行列，0-indexed）
	cursorRow    int
	cursorCol    int
	preferredCol int // 上下移动时记录的期望列

	// 选区管理
	selectionActive bool
	selAnchorRow    int
	selAnchorCol    int
	selCursorRow    int
	selCursorCol    int
	isSelecting     bool // 鼠标左键拖拽选择中

	// 撤销 / 重做栈（差分记录，内存占用只与被改动片段相关）
	undoStack []editRecord
	redoStack []editRecord
	undoBytes int
	redoBytes int

	// 字体与度量
	fontFamily          string
	baseFontSize        int32
	zoomFactor          float64
	font                *Font
	fontBold            *Font // 粗体变体
	fontItalic          *Font // 斜体变体
	fontBoldItalic      *Font // 粗斜体变体
	fontFace            *render.FontFace
	fontFaceBold        *render.FontFace
	fontFaceItalic      *render.FontFace
	fontFaceBoldItalic  *render.FontFace
	lineHeight          int32
	charWidth           int32  // 基础字符宽度（用于等宽估算）
	digitWidth          int32  // '0' 的步进宽度，行号槽用它做零分配宽度计算
	fontMonospace       bool   // 当前字体是否为等宽（用于跳过整行精确测量）
	monoWidth           int32  // 等宽字体的单列步进
	monoWideWidth       int32  // 等宽字体中 CJK/宽字符的步进（通常为 2 列）
	gutterRunes         []rune // 行号绘制复用缓冲，避免每帧分配
	showLineNumbers     bool
	highlightActiveLine bool
	margin              int32 // 文本区左内边距

	// 滚动状态 (像素偏移)
	scrollOffsetY int32
	scrollOffsetX int32
	// 滚轮 1/120 单位增量的余数（精密触摸板会发送 <120 的分数增量，不能直接整除丢弃）
	wheelRemainderY int64
	wheelRemainderX int64
	zoomRemainder   int64

	// 光标 X 的跨帧缓存：闪烁重绘/鼠标移动时无需重走整行字形测量
	measureEpoch uint64
	caretXValid  bool
	caretXRow    int
	caretXCol    int
	caretXEpoch  uint64
	caretXWidth  int32

	// 光标闪烁状态由窗口定时器驱动，配合局部重绘只刷新光标所在单元
	caretOn bool

	// 垂直滚动条拖拽状态
	draggingScrollbar bool
	dragStartY        int32
	dragStartScrollY  int32

	// 序列化换行符（默认 CRLF；打开文件时按原始文件风格设置）
	lineEnding string

	// 输入法预编辑串（组合中的文本，仅参与绘制，不进入文档）
	preedit string

	// 多击检测（双击选词 / 三击选行）
	lastClickAt  time.Time
	lastClickRow int
	lastClickCol int
	clickCount   int

	// 撤销合组：连续输入/退格在时间窗口内合并为一步撤销
	lastEditAt   time.Time
	lastEditKind int // 0 无, 1 插入, 2 删除

	// 非折行模式下的最大内容宽度（用于水平滚动上限）
	maxContentWidth int32
	maxWidthRow     int  // 当前最大值所在逻辑行（用于删除/缩短该行后惰性重算）
	maxWidthDirty   bool // 最大值可能已失效，需在下次 clampScroll 时重算

	// 状态标记
	focused  bool
	readOnly bool
	modified bool // 内容是否有未保存修改

	// 修订号：用于在“保存点”与撤销/重做之间准确判断是否有未保存修改
	revision      uint64 // 当前状态对应的修订号
	savedRevision uint64 // 最近一次保存/加载对应的修订号
	nextRevision  uint64 // 单调递增的修订号分配器

	// 样式与颜色令牌
	background       graphics.Color
	textColor        graphics.Color
	caretColor       graphics.Color
	selectionColor   graphics.Color
	activeLineBg     graphics.Color
	activeLineBorder graphics.Color
	gutterBg         graphics.Color
	gutterText       graphics.Color
	gutterBorder     graphics.Color
	scrollbarTrack   graphics.Color
	scrollbarThumb   graphics.Color
	scrollbarHover   graphics.Color

	// 事件回调
	onChange       func()
	onCursorChange func(row, col int)
	onZoomChange   func(percent int)
}

// NewTextArea 创建多行文本编辑器
func NewTextArea() *TextArea {
	t := &TextArea{
		BaseComponent:       NewBaseComponent(),
		layouts:             make([]lineLayout, 1),
		visualState:         visualFull,
		wordWrap:            true, // 默认开启自动折行
		fontFamily:          "Consolas",
		baseFontSize:        16, // 默认基准字号升级至 16px，大屏与2K下字迹饱满清晰
		zoomFactor:          1.0,
		caretOn:             true,
		showLineNumbers:     true,
		highlightActiveLine: true,
		margin:              8,
		undoStack:           make([]editRecord, 0, 64),
		redoStack:           make([]editRecord, 0, 16),
		spansCache:          make(map[int][]syntax.Span),
		lineEnding:          "\r\n",
	}

	t.ensureTextInitialized()
	t.rebuildFont()
	t.ApplyTheme(theme.Current())
	return t
}

// rebuildFont 根据 fontFamily、baseFontSize 和 zoomFactor 重新生成字体度量
func (t *TextArea) rebuildFont() {
	oldMaxWidth := t.maxContentWidth
	oldFontSize := int32(0)
	if t.font != nil {
		oldFontSize = t.font.Size
	}
	actualSize := int32(float64(t.baseFontSize)*t.zoomFactor + 0.5)
	if actualSize < 8 {
		actualSize = 8
	}
	if actualSize > 72 {
		actualSize = 72
	}
	t.font = graphics.NewFont(t.fontFamily, actualSize)
	t.fontBold = graphics.NewFont(t.fontFamily, actualSize)
	t.fontBold.Bold = true
	t.fontItalic = graphics.NewFont(t.fontFamily, actualSize)
	t.fontItalic.Italic = true
	t.fontBoldItalic = graphics.NewFont(t.fontFamily, actualSize)
	t.fontBoldItalic.Bold = true
	t.fontBoldItalic.Italic = true

	// 编辑器需要列对齐：空格步进固定为 1 个字符（数字 '0'）宽度。
	// 内嵌 Gnote Mono 本身等宽，这里显式开启作为用户替换成比例字体时的兜底。
	t.font.AlignSpace = true
	t.fontBold.AlignSpace = true
	t.fontItalic.AlignSpace = true
	t.fontBoldItalic.AlignSpace = true

	fm := render.GetFontManager()
	t.fontFace = fm.GetFace(t.font)
	t.fontFaceBold = fm.GetFace(t.fontBold)
	t.fontFaceItalic = fm.GetFace(t.fontItalic)
	t.fontFaceBoldItalic = fm.GetFace(t.fontBoldItalic)

	t.measureFontMetrics()
	t.invalidateAllLayouts()
	if oldFontSize > 0 && actualSize > 0 {
		t.maxContentWidth = int32(float64(oldMaxWidth) * float64(actualSize) / float64(oldFontSize))
	}
	t.clampCursor()
	t.ensureCursorVisible()
}

// SetBounds 设置组件空间包围盒，并在宽度发生变更时标记重新折行
func (t *TextArea) SetBounds(x, y, width, height int32) {
	widthChanged := (t.bounds.Width != width)
	t.BaseComponent.SetBounds(x, y, width, height)
	if widthChanged {
		t.clampScroll()
		t.ensureCursorVisible()
	}
}

// SetWordWrap 设置是否启用自动换行 (true: 自动折行不超出窗口右侧; false: 单行水平延续)
func (t *TextArea) SetWordWrap(wrap bool) {
	if t.wordWrap != wrap {
		t.wordWrap = wrap
		t.invalidateAllSegments()
		t.clampScroll()
		t.ensureCursorVisible()
	}
}

// IsWordWrap 检查当前是否开启了自动换行
func (t *TextArea) IsWordWrap() bool {
	return t.wordWrap
}

// getStyledFont 获取粗体/斜体变体字体
func (t *TextArea) getStyledFont(bold, italic bool) *Font {
	if bold && italic && t.fontBoldItalic != nil {
		return t.fontBoldItalic
	}
	if bold && t.fontBold != nil {
		return t.fontBold
	}
	if italic && t.fontItalic != nil {
		return t.fontItalic
	}
	return t.font
}

// getStyledFace 获取粗体/斜体变体字形缓存
func (t *TextArea) getStyledFace(bold, italic bool) *render.FontFace {
	if bold && italic && t.fontFaceBoldItalic != nil {
		return t.fontFaceBoldItalic
	}
	if bold && t.fontFaceBold != nil {
		return t.fontFaceBold
	}
	if italic && t.fontFaceItalic != nil {
		return t.fontFaceItalic
	}
	return t.fontFace
}

// invalidateAllLayouts 字体/缩放变化后，丢弃全部排版与宽度缓存
// ---- 文本存储：gap buffer + 行偏移 ----

// lineCount 返回逻辑行数（至少 1）
func (t *TextArea) lineCount() int { return len(t.lineStart) - 1 }

// logicalLen 返回不含间隙的逻辑 rune 总数
func (t *TextArea) logicalLen() int { return len(t.text) - (t.gapEnd - t.gapStart) }

// ensureTextInitialized 保证文档至少有一个空行
func (t *TextArea) ensureTextInitialized() {
	if len(t.lineStart) == 0 {
		t.text = make([]rune, 64)
		t.gapStart, t.gapEnd = 0, 64
		t.lineStart = []int{0, 0}
	}
}

// moveGap 把间隙移动到逻辑偏移 pos（pos 处的字符紧随机隙之后）
func (t *TextArea) moveGap(pos int) {
	if pos < 0 {
		pos = 0
	}
	if l := t.logicalLen(); pos > l {
		pos = l
	}
	if pos == t.gapStart {
		return
	}
	g := t.gapEnd - t.gapStart
	if pos < t.gapStart {
		copy(t.text[pos+g:], t.text[pos:t.gapStart])
	} else {
		copy(t.text[t.gapStart:], t.text[t.gapEnd:t.gapEnd+(pos-t.gapStart)])
	}
	t.gapStart, t.gapEnd = pos, pos+g
}

// growGap 扩充间隙容量，保留逻辑内容不变
func (t *TextArea) growGap(need int) {
	g := t.gapEnd - t.gapStart
	newGap := g * 2
	if newGap < 4096 {
		newGap = 4096
	}
	if newGap < need {
		newGap = need
	}
	n := t.logicalLen()
	buf := make([]rune, n+newGap)
	copy(buf, t.text[:t.gapStart])
	copy(buf[t.gapStart+newGap:], t.text[t.gapEnd:])
	t.text = buf
	t.gapEnd = t.gapStart + newGap
}

// insertAt 在逻辑偏移 pos 处插入 runes（间隙先移动到 pos）
func (t *TextArea) insertAt(pos int, rs []rune) {
	if len(rs) == 0 {
		return
	}
	t.moveGap(pos)
	if need := len(rs) - (t.gapEnd - t.gapStart); need > 0 {
		t.growGap(need)
	}
	copy(t.text[t.gapStart:], rs)
	t.gapStart += len(rs)
}

// replaceAt 用 rs 替换逻辑区间 [pos, pos+removed)
func (t *TextArea) replaceAt(pos, removed int, rs []rune) {
	t.moveGap(pos)
	t.gapEnd += removed
	t.insertAt(pos, rs)
}

// lineContentEnd 返回第 r 行内容的结束偏移（不含行尾换行符）
func (t *TextArea) lineContentEnd(r int) int {
	if r == t.lineCount()-1 {
		return t.lineStart[r+1]
	}
	return t.lineStart[r+1] - 1
}

// line 返回第 r 行的零拷贝视图。若间隙位于该行内部，先把间隙挪到最近的
// 行边界（代价不超过行长），因此调用方拿到的永远是连续切片。
func (t *TextArea) line(r int) []rune {
	if r < 0 || r >= t.lineCount() {
		return nil
	}
	s, e := t.lineStart[r], t.lineContentEnd(r)
	g := t.gapEnd - t.gapStart
	switch {
	case e <= t.gapStart:
		return t.text[s:e:e]
	case s >= t.gapStart:
		return t.text[s+g : e+g : e+g]
	}
	// 间隙落在行内：挪到较近的一端
	if t.gapStart-s <= e-t.gapStart {
		t.moveGap(s)
		pe := t.gapEnd + (e - s)
		return t.text[t.gapEnd:pe:pe]
	}
	t.moveGap(e)
	return t.text[s:e:e]
}

// lineLen 返回第 r 行的 rune 数（不触碰间隙位置）
func (t *TextArea) lineLen(r int) int { return t.lineContentEnd(r) - t.lineStart[r] }

// cloneLogicalRange 复制逻辑区间 [start,end)
func (t *TextArea) cloneLogicalRange(start, end int) []rune {
	out := make([]rune, 0, end-start)
	return t.appendLogical(out, start, end)
}

// appendLogical 把逻辑区间 [start,end) 追加到 dst，跨间隙时两段拷贝
func (t *TextArea) appendLogical(dst []rune, start, end int) []rune {
	g := t.gapEnd - t.gapStart
	if end <= t.gapStart {
		return append(dst, t.text[start:end]...)
	}
	if start >= t.gapStart {
		return append(dst, t.text[start+g:end+g]...)
	}
	dst = append(dst, t.text[start:t.gapStart]...)
	return append(dst, t.text[t.gapEnd:end+g]...)
}

// installText 用规范化的文本重建存储与行偏移（装载路径）
func (t *TextArea) installText(text string) {
	if strings.ContainsRune(text, '\r') {
		text = strings.ReplaceAll(text, "\r\n", "\n")
		text = strings.ReplaceAll(text, "\r", "\n")
	}
	n := 0
	for range text {
		n++
	}
	spare := n / 16
	if spare < 64 {
		spare = 64
	}
	buf := make([]rune, n+spare)
	i := 0
	for _, r := range text {
		buf[i] = r
		i++
	}
	starts := make([]int, 0, strings.Count(text, "\n")+2)
	starts = append(starts, 0)
	for j := 0; j < n; j++ {
		if buf[j] == '\n' {
			starts = append(starts, j+1)
		}
	}
	starts = append(starts, n)
	t.text = buf
	t.lineStart = starts
	t.gapStart, t.gapEnd = n, len(buf)
}

// spliceLineStarts 在 [startRow] 行内做 offset 编辑后，增量维护行偏移表
func (t *TextArea) spliceLineStarts(startRow, oldCount, startOff, removedLen int, inserted []rune, delta int) {
	newEnt := 0
	for _, r := range inserted {
		if r == '\n' {
			newEnt++
		}
	}
	oldLen := len(t.lineStart)
	newLen := oldLen - (oldCount - 1) + newEnt
	if newLen > oldLen {
		t.lineStart = append(t.lineStart, make([]int, newLen-oldLen)...)
	}
	src := startRow + oldCount
	dst := startRow + 1 + newEnt
	copy(t.lineStart[dst:], t.lineStart[src:oldLen])
	t.lineStart = t.lineStart[:newLen]

	k := 0
	for i, r := range inserted {
		if r == '\n' {
			t.lineStart[startRow+1+k] = startOff + i + 1
			k++
		}
	}
	for i := dst; i < newLen; i++ {
		t.lineStart[i] += delta
	}
}

func (t *TextArea) invalidateAllLayouts() {
	t.measureEpoch++
	t.longPrefixes = nil
	for i := range t.layouts {
		t.layouts[i] = lineLayout{}
	}
	t.visualState = visualFull
	// 保留 maxContentWidth 作为缩放后的估算上界：逐行测量改为按需进行，
	// 可见行会在渲染时修正实际上界，避免缩放瞬间归零导致水平滚动跳变。
}

// invalidateAllSegments 折行宽度变化后重建视觉行；整行宽度缓存（totalWidth）仍然有效
func (t *TextArea) invalidateAllSegments() {
	t.measureEpoch++
	t.visualState = visualFull
}

// SetHighlighter 设置语法高亮器 (传入 nil 即可关闭高亮)
func (t *TextArea) SetHighlighter(h syntax.Highlighter) {
	t.measureEpoch++
	t.longPrefixes = nil
	t.highlighter = h
	t.lineStates = nil
	t.statesValidTo = 0
	if h != nil {
		// 一次性分配每行状态缓存，避免首帧按需扩容时的临时切片与多次拷贝
		t.lineStates = make([]int, t.lineCount())
	}
	t.spansCache = make(map[int][]syntax.Span)
	if t.highlighter != nil {
		t.highlighter.ApplyTheme(theme.Current())
	}
}

// GetHighlighter 获取当前绑定的语法高亮器
func (t *TextArea) GetHighlighter() syntax.Highlighter {
	return t.highlighter
}

// ensureLineStatesLen 保持状态缓存与逻辑行数一致（仅扩展长度，不保证内容有效）
func (t *TextArea) ensureLineStatesLen() {
	if len(t.lineStates) == t.lineCount() {
		return
	}
	if len(t.lineStates) > t.lineCount() {
		t.lineStates = t.lineStates[:t.lineCount()]
		if t.statesValidTo > t.lineCount() {
			t.statesValidTo = t.lineCount()
		}
		return
	}
	t.lineStates = append(t.lineStates, make([]int, t.lineCount()-len(t.lineStates))...)
}

// invalidateHighlightFrom 从指定行开始使语法状态缓存失效（之前的行保持有效）
func (t *TextArea) invalidateHighlightFrom(row int) {
	t.measureEpoch++
	if row < 0 {
		row = 0
	}
	if row < t.statesValidTo {
		t.statesValidTo = row
	}
	for r := range t.spansCache {
		if r >= row {
			delete(t.spansCache, r)
		}
	}
}

// updateLineStatesUpTo 增量计算并缓存从上次有效位置到 maxRow 的每行结束状态
func (t *TextArea) updateLineStatesUpTo(maxRow int) {
	if t.highlighter == nil || t.lineCount() == 0 {
		return
	}
	if maxRow >= t.lineCount() {
		maxRow = t.lineCount() - 1
	}
	if maxRow < t.statesValidTo {
		return
	}
	t.ensureLineStatesLen()
	for i := t.statesValidTo; i <= maxRow; i++ {
		prevState := 0
		if i > 0 {
			prevState = t.lineStates[i-1]
		}
		_, endState := t.highlighter.HighlightLine(i, t.line(i), prevState)
		t.lineStates[i] = endState
	}
	t.statesValidTo = maxRow + 1
}

// invalidateHighlightForEdit 在单次文本编辑后失效高亮缓存。
// 若行数未变化，且编辑后最后一行的结束词法状态与编辑前一致，则下游行的状态与
// span 缓存继续有效，避免每次按键都把视口内的剩余行重新分词。
func (t *TextArea) invalidateHighlightForEdit(startRow, endRow, rowDelta int) {
	if startRow < 0 {
		startRow = 0
	}
	if endRow >= t.lineCount() {
		endRow = t.lineCount() - 1
	}
	if endRow < startRow {
		endRow = startRow
	}
	if t.highlighter == nil || rowDelta != 0 || endRow < 0 {
		t.invalidateHighlightFrom(startRow)
		return
	}

	for r := range t.spansCache {
		if r >= startRow && r <= endRow {
			delete(t.spansCache, r)
		}
	}
	t.measureEpoch++

	// 只有编辑前的行尾状态与编辑行的输入状态都已知时才可比较
	oldEndKnown := endRow < t.statesValidTo && endRow < len(t.lineStates)
	prevKnown := startRow == 0 || startRow-1 < t.statesValidTo
	if !oldEndKnown || !prevKnown {
		t.invalidateHighlightFrom(startRow)
		return
	}
	oldEnd := t.lineStates[endRow]

	t.ensureLineStatesLen()
	state := 0
	if startRow > 0 {
		state = t.lineStates[startRow-1]
	}
	for r := startRow; r <= endRow; r++ {
		_, state = t.highlighter.HighlightLine(r, t.line(r), state)
		t.lineStates[r] = state
	}

	if state == oldEnd {
		// 下游行仍以相同状态开头，缓存保持有效
		return
	}
	t.statesValidTo = endRow + 1
	for r := range t.spansCache {
		if r > endRow {
			delete(t.spansCache, r)
		}
	}
}

// getSpansForLine 获取指定逻辑行 r 的语法高亮 spans（带行级缓存）
func (t *TextArea) getSpansForLine(r int) []syntax.Span {
	if t.highlighter == nil || r < 0 || r >= t.lineCount() {
		return nil
	}
	if spans, ok := t.spansCache[r]; ok {
		return spans
	}
	t.updateLineStatesUpTo(r)
	prevState := 0
	if r > 0 && r-1 < len(t.lineStates) && t.lineStates[r-1] != -1 {
		prevState = t.lineStates[r-1]
	}
	spans, _ := t.highlighter.HighlightLine(r, t.line(r), prevState)
	// 缓存只服务于可见行：上限压到一个视口量级，既避免长会话堆积，
	// 也让“从编辑行向下失效”的 map 扫描保持在微秒级。
	if len(t.spansCache) > maxCachedSpanRows {
		clear(t.spansCache)
	}
	t.spansCache[r] = spans
	return spans
}

// spanFaceAt 返回列 col 使用的样式字体：span 有序时用单调游标，避免逐字符重扫。
func (t *TextArea) spanFaceAt(spans []syntax.Span, idx *int, col int) *render.FontFace {
	for *idx < len(spans) && spans[*idx].EndCol <= col {
		*idx = *idx + 1
	}
	if *idx < len(spans) {
		sp := spans[*idx]
		if col >= sp.StartCol && col < sp.EndCol && (sp.Bold || sp.Italic) {
			return t.getStyledFace(sp.Bold, sp.Italic)
		}
	}
	return t.fontFace
}

// prefixFor 返回超长行的宽度检查点；关闭折行且长度达到阈值时才构建。
func (t *TextArea) prefixFor(row int) *linePrefix {
	if t.wordWrap || row < 0 || row >= t.lineCount() || len(t.line(row)) < longLinePrefixThreshold {
		return nil
	}
	if p := t.longPrefixes[row]; p != nil {
		if p.validTo > len(t.line(row)) {
			// 行尾删除：丢弃越界检查点，剩余前缀仍然有效
			p.validTo = len(t.line(row))
			keep := p.validTo/prefixStep + 1
			if keep < len(p.widths) {
				p.widths = p.widths[:keep]
			}
		}
		if p.validTo < len(t.line(row)) {
			t.extendPrefix(p, row)
		}
		return p
	}
	p := t.buildPrefix(row)
	if t.longPrefixes == nil {
		t.longPrefixes = make(map[int]*linePrefix)
	}
	t.longPrefixes[row] = p
	return p
}

// extendPrefix 从最后一个有效检查点续算到行尾（末尾输入时只需处理最后 <prefixStep 列）。
func (t *TextArea) extendPrefix(p *linePrefix, row int) {
	line := t.line(row)
	k := p.validTo / prefixStep
	if k >= len(p.widths) {
		k = len(p.widths) - 1
	}
	if k < 0 {
		k = 0
	}
	p.widths = p.widths[:k+1]
	start := k * prefixStep
	w := p.widths[k]
	spans := t.getSpansForLine(row)
	fm := render.GetFontManager()
	spanIdx := 0
	for i := start; i < len(line); i++ {
		w += fm.RuneWidthFace(t.spanFaceAt(spans, &spanIdx, i), line[i])
		if (i+1)%prefixStep == 0 {
			p.widths = append(p.widths, w)
		}
	}
	p.validTo = len(line)
}

// truncatePrefixForEdit 文本编辑后使受影响列之后的检查点失效。
// 同列编辑之前的检查点仍然有效；跨行编辑直接丢弃相关行的缓存。
func (t *TextArea) truncatePrefixForEdit(startRow, startCol, endRow, endCol int) {
	if len(t.longPrefixes) == 0 {
		return
	}
	if startRow == endRow {
		if p := t.longPrefixes[startRow]; p != nil && startCol < p.validTo {
			p.validTo = startCol
			keep := startCol/prefixStep + 1
			if keep < len(p.widths) {
				p.widths = p.widths[:keep]
			}
		}
		return
	}
	lo, hi := startRow, endRow
	if hi < lo {
		lo, hi = hi, lo
	}
	for r := lo; r <= hi; r++ {
		delete(t.longPrefixes, r)
	}
}

// buildPrefix 一次扫描构建整行检查点（仅在文本/字体/样式变化后执行一次）。
func (t *TextArea) buildPrefix(row int) *linePrefix {
	line := t.line(row)
	p := &linePrefix{
		widths: make([]int32, 0, len(line)/prefixStep+2),
	}
	p.widths = append(p.widths, 0)
	spans := t.getSpansForLine(row)
	fm := render.GetFontManager()
	spanIdx := 0
	var w int32
	for i := 0; i < len(line); i++ {
		w += fm.RuneWidthFace(t.spanFaceAt(spans, &spanIdx, i), line[i])
		if (i+1)%prefixStep == 0 {
			p.widths = append(p.widths, w)
		}
	}
	p.validTo = len(line)
	return p
}

// prefixWidthAt 返回 [0,col) 的像素宽度；字典区间至多回退 prefixStep 个 rune。
func (t *TextArea) prefixWidthAt(p *linePrefix, row, col int) int32 {
	if col <= 0 || p == nil {
		return 0
	}
	line := t.line(row)
	if col > len(line) {
		col = len(line)
	}
	k := col / prefixStep
	if k >= len(p.widths) {
		k = len(p.widths) - 1
	}
	w := p.widths[k]
	base := k * prefixStep
	if base >= col {
		return w
	}
	spans := t.getSpansForLine(row)
	fm := render.GetFontManager()
	spanIdx := 0
	for i := base; i < col; i++ {
		w += fm.RuneWidthFace(t.spanFaceAt(spans, &spanIdx, i), line[i])
	}
	return w
}

// prefixColAtX 把相对 from 的像素偏移换算为列号，结果限制在 [lower, upper]。
func (t *TextArea) prefixColAtX(p *linePrefix, row, from int, x int32, lower, upper int) int {
	if x <= 0 || upper <= lower {
		return lower
	}
	base := t.prefixWidthAt(p, row, from)
	target := base + x
	if t.prefixWidthAt(p, row, upper) <= target {
		return upper
	}
	k := sort.Search(len(p.widths), func(i int) bool { return p.widths[i] > target }) - 1
	if k < 0 {
		k = 0
	}
	startCol := k * prefixStep
	if startCol < from {
		startCol = from
	}
	if startCol >= upper {
		return upper
	}
	line := t.line(row)
	spans := t.getSpansForLine(row)
	fm := render.GetFontManager()
	spanIdx := 0
	w := t.prefixWidthAt(p, row, startCol)
	for i := startCol; i < upper; i++ {
		if w >= target {
			return i
		}
		w += fm.RuneWidthFace(t.spanFaceAt(spans, &spanIdx, i), line[i])
	}
	return upper
}

// measureStyledVisualX 计算逻辑行 r 中 [subStart : targetCol] 这段文本在考虑语法高亮样式字体下的实际物理像素宽度
func (t *TextArea) measureStyledVisualX(r int, subStart, targetCol int) int32 {
	if targetCol <= subStart || r < 0 || r >= t.lineCount() {
		return 0
	}
	line := t.line(r)
	if subStart >= len(line) {
		return 0
	}
	if targetCol > len(line) {
		targetCol = len(line)
	}

	// 超长行：用检查点把 O(列数) 测量降为 O(prefixStep)
	if targetCol-subStart > prefixStep {
		if p := t.prefixFor(r); p != nil {
			return t.prefixWidthAt(p, r, targetCol) - t.prefixWidthAt(p, r, subStart)
		}
	}

	spans := t.getSpansForLine(r)
	fm := render.GetFontManager()
	if len(spans) == 0 {
		return fm.MeasureRunesFace(t.fontFace, line[subStart:targetCol])
	}

	totalW := int32(0)
	currCol := subStart

	for _, span := range spans {
		if span.EndCol <= subStart || span.StartCol >= targetCol {
			continue
		}

		// 1. 处理 span 之前未被标记的部分
		if span.StartCol > currCol {
			end := span.StartCol
			if end > targetCol {
				end = targetCol
			}
			totalW += fm.MeasureRunesFace(t.fontFace, line[currCol:end])
			currCol = end
			if currCol >= targetCol {
				break
			}
		}

		// 2. 处理 span 内部相交部分
		spanStart := span.StartCol
		if spanStart < currCol {
			spanStart = currCol
		}
		spanEnd := span.EndCol
		if spanEnd > targetCol {
			spanEnd = targetCol
		}

		if spanEnd > spanStart {
			spanFace := t.getStyledFace(span.Bold, span.Italic)
			totalW += fm.MeasureRunesFace(spanFace, line[spanStart:spanEnd])
			currCol = spanEnd
			if currCol >= targetCol {
				break
			}
		}
	}

	// 3. 处理尾部剩余未标记部分
	if currCol < targetCol {
		totalW += fm.MeasureRunesFace(t.fontFace, line[currCol:targetCol])
	}

	return totalW
}

// getStyledVisualColFromX 依据物理像素 X 偏移反算视觉子行内的字符列索引 (返回值在 [subStart, subEnd] 之间)
func (t *TextArea) getStyledVisualColFromX(r int, subStart, subEnd int, targetX int32) int {
	if targetX <= 0 || r < 0 || r >= t.lineCount() || subStart >= subEnd {
		return subStart
	}
	line := t.line(r)
	if subStart >= len(line) {
		return len(line)
	}
	if subEnd > len(line) {
		subEnd = len(line)
	}

	spans := t.getSpansForLine(r)
	fm := render.GetFontManager()

	if len(spans) == 0 {
		currW := int32(0)
		for i := subStart; i < subEnd; i++ {
			rw := fm.RuneWidthFace(t.fontFace, line[i])
			if currW+rw >= targetX {
				if targetX-currW > rw/2 {
					return i + 1
				}
				return i
			}
			currW += rw
		}
		return subEnd
	}

	// 带高亮样式逐字符反查：span 有序且不重叠，用单调游标代替每个字符重扫全部 span
	currW := int32(0)
	spanIdx := 0
	for i := subStart; i < subEnd; i++ {
		for spanIdx < len(spans) && spans[spanIdx].EndCol <= i {
			spanIdx++
		}
		faceToUse := t.fontFace
		if spanIdx < len(spans) {
			span := spans[spanIdx]
			if i >= span.StartCol && i < span.EndCol && (span.Bold || span.Italic) {
				faceToUse = t.getStyledFace(span.Bold, span.Italic)
			}
		}

		rw := fm.RuneWidthFace(faceToUse, line[i])
		if currW+rw >= targetX {
			if targetX-currW > rw/2 {
				return i + 1
			}
			return i
		}
		currW += rw
	}
	return subEnd
}

// trimStaleGlyphCaches 回收其它字号/字体面的字形位图缓存（缩放后旧字号不再使用）。
func (t *TextArea) trimStaleGlyphCaches() {
	render.GetFontManager().ClearGlyphCachesExcept(t.font, t.fontBold, t.fontItalic, t.fontBoldItalic)
}

// measureFontMetrics 预先度量字符宽高
func (t *TextArea) measureFontMetrics() {
	fm := render.GetFontManager()
	face := t.fontFace
	if face == nil {
		face = fm.GetFace(t.font)
	}
	if face != nil && face.Height > 0 {
		t.lineHeight = face.Height + 4
		w0 := fm.RuneWidthFace(face, '0')
		wM := fm.RuneWidthFace(face, 'M')
		if w0 > wM {
			t.charWidth = w0
		} else if wM > 0 {
			t.charWidth = wM
		} else {
			t.charWidth = 8
		}
	} else {
		t.lineHeight = 20
		t.charWidth = 8
	}
	if face != nil {
		t.digitWidth = fm.RuneWidthFace(face, '0')
		// 等宽检测：常用 ASCII 步进一致即可用“列数 × monoWidth”做整行宽度上界，
		// 从而在缩放/折行重建时跳过绝大多数行的逐字形测量。
		w0 := t.digitWidth
		mono := w0 > 0 &&
			fm.RuneWidthFace(face, 'i') == w0 &&
			fm.RuneWidthFace(face, 'W') == w0 &&
			fm.RuneWidthFace(face, 'M') == w0 &&
			fm.RuneWidthFace(face, '.') == w0
		t.fontMonospace = mono
		t.monoWidth = w0
		wide := fm.RuneWidthFace(face, '中')
		if wide < 2*w0 {
			wide = 2 * w0
		}
		t.monoWideWidth = wide
	}
	if t.digitWidth <= 0 {
		t.digitWidth = t.charWidth
	}
}

// lineFitsUpperBound 用等宽列数上界判断整行是否必然不需要折行。
// 只做 rune 扫描，不查字形；返回 false 时调用方再走精确测量。
func (t *TextArea) lineFitsUpperBound(line []rune, availWidth int32) bool {
	if !t.fontMonospace || t.monoWidth <= 0 || t.monoWideWidth <= 0 {
		return false
	}
	var w int32
	for _, r := range line {
		switch {
		case r == '	':
			w += 4 * t.monoWidth
		case r >= 0x2E80:
			w += t.monoWideWidth
		default:
			w += t.monoWidth
		}
		if w > availWidth {
			return false
		}
	}
	return true
}

// ApplyTheme 应用主题调色板
func (t *TextArea) ApplyTheme(th *theme.Theme) {
	if th == nil {
		return
	}
	t.background = th.Surface
	t.textColor = th.TextPrimary
	t.caretColor = th.Primary

	// 选区颜色：主题提供半透明强调色，避免每个主题重新推导
	t.selectionColor = th.Selection

	// 当前光标行与行号槽完全由主题令牌驱动，不再按明暗分支硬编码
	t.activeLineBg = th.ActiveLineBg
	t.activeLineBorder = th.ActiveLineBorder
	t.gutterBg = th.GutterBg
	t.gutterText = th.GutterText
	t.gutterBorder = th.GutterBorder

	t.scrollbarTrack = th.Track
	t.scrollbarThumb = th.Thumb
	t.scrollbarHover = th.ThumbHover

	if t.highlighter != nil {
		t.highlighter.ApplyTheme(th)
	}
	// 主题切换后旧 spans 携带旧调色板，必须整体失效
	t.spansCache = make(map[int][]syntax.Span)
}

// GetCursor 获取鼠标工字光标 (IDC_IBEAM)
func (t *TextArea) GetCursor() int {
	return 32513
}

// ToggleCaretBlink 由窗口定时器调用，翻转光标可见状态。
func (t *TextArea) ToggleCaretBlink() {
	t.caretOn = !t.caretOn
}

// CaretRect 返回光标单元在窗口坐标系中的矩形，用于局部重绘。
// 光标不可见（未聚焦、只读、滚出视口）时返回 false。
func (t *TextArea) CaretRect() (graphics.Rect, bool) {
	if !t.visible || !t.focused || t.readOnly || t.bounds.Width <= 0 || t.bounds.Height <= 0 {
		return graphics.Rect{}, false
	}
	t.recomputeVisualLines()
	if len(t.visualLines) == 0 {
		return graphics.Rect{}, false
	}
	activeIdx := t.getVisualIndexForCursor()
	startVisual := int(t.scrollOffsetY / t.lineHeight)
	if startVisual < 0 {
		startVisual = 0
	}
	visibleCount := int((t.bounds.Height+t.lineHeight-1)/t.lineHeight) + 1
	if activeIdx < startVisual || activeIdx >= startVisual+visibleCount {
		return graphics.Rect{}, false
	}

	v := t.visualLines[activeIdx]
	gutterW := t.getGutterWidth()
	textStartX := t.bounds.X + gutterW + t.margin
	scrollX := t.scrollOffsetX
	if t.wordWrap {
		scrollX = 0
	}
	x := textStartX + t.measureStyledVisualX(v.logicalRow, v.startCol, t.cursorCol) - scrollX
	if t.preedit != "" {
		x += t.measurePreeditWidth()
	}
	y := t.bounds.Y + t.margin + int32(activeIdx)*t.lineHeight - t.scrollOffsetY
	return graphics.NewRect(x, y, 2, t.lineHeight), true
}

// HasBlinkingCursor 供窗口定时器嗅探，驱动光标呼吸重绘
func (t *TextArea) HasBlinkingCursor() bool {
	return t.focused && !t.readOnly
}

// SetShowLineNumbers 设置是否显示行号
func (t *TextArea) SetShowLineNumbers(show bool) {
	if t.showLineNumbers != show {
		t.showLineNumbers = show
		t.invalidateAllSegments()
		t.clampScroll()
		t.ensureCursorVisible()
	}
}

// IsShowLineNumbers 是否显示行号
func (t *TextArea) IsShowLineNumbers() bool {
	return t.showLineNumbers
}

// SetHighlightActiveLine 设置是否高亮当前光标所在行
func (t *TextArea) SetHighlightActiveLine(highlight bool) {
	t.highlightActiveLine = highlight
}

// IsHighlightActiveLine 检查是否高亮当前光标所在行
func (t *TextArea) IsHighlightActiveLine() bool {
	return t.highlightActiveLine
}

// SetActiveLineStyle 自定义设置当前行背景色与边框色
func (t *TextArea) SetActiveLineStyle(bg, border graphics.Color) {
	t.activeLineBg = bg
	t.activeLineBorder = border
}

// IsFocused 检查是否获取焦点
func (t *TextArea) IsFocused() bool {
	return t.focused
}

// SetFocused 设置是否获取焦点
func (t *TextArea) SetFocused(focused bool) {
	t.focused = focused
	if focused {
		t.caretOn = true
	}
}

// SetReadOnly 设置是否只读
func (t *TextArea) SetReadOnly(ro bool) {
	t.readOnly = ro
}

// IsReadOnly 检查是否只读
func (t *TextArea) IsReadOnly() bool {
	return t.readOnly
}

// SetFont 设置编辑器字体族与基准字号（遵循当前缩放），并同步粗体/斜体变体。
func (t *TextArea) SetFont(font *Font) {
	if font == nil || font.Name == "" {
		return
	}
	t.fontFamily = font.Name
	if font.Size > 0 {
		t.baseFontSize = font.Size
	}
	t.rebuildFont()
	t.trimStaleGlyphCaches()
}

// GetFont 获取当前字体
func (t *TextArea) GetFont() *Font {
	return t.font
}

// SetFontFamily 设置字体族（如 "Consolas", "Courier New", "Microsoft YaHei"）
func (t *TextArea) SetFontFamily(family string) {
	if family != "" && family != t.fontFamily {
		t.fontFamily = family
		t.rebuildFont()
		t.trimStaleGlyphCaches()
	}
}

// GetFontFamily 获取当前字体族
func (t *TextArea) GetFontFamily() string {
	return t.fontFamily
}

// SetBaseFontSize 设置基准字号
func (t *TextArea) SetBaseFontSize(size int32) {
	if size > 0 && size != t.baseFontSize {
		t.baseFontSize = size
		t.rebuildFont()
		t.trimStaleGlyphCaches()
	}
}

// GetBaseFontSize 获取基准字号
func (t *TextArea) GetBaseFontSize() int32 {
	return t.baseFontSize
}

// SetZoom 设置缩放系数 (0.5 ~ 3.0)
func (t *TextArea) SetZoom(factor float64) {
	if factor < 0.5 {
		factor = 0.5
	}
	if factor > 3.0 {
		factor = 3.0
	}
	t.zoomFactor = factor
	t.rebuildFont()
	t.trimStaleGlyphCaches()
	if t.onZoomChange != nil {
		t.onZoomChange(t.GetZoomPercent())
	}
}

// GetZoom 获取当前缩放系数
func (t *TextArea) GetZoom() float64 {
	return t.zoomFactor
}

// GetZoomPercent 获取当前缩放百分比 (如 100, 125, 150)
func (t *TextArea) GetZoomPercent() int {
	return int(t.zoomFactor*100 + 0.5)
}

// ZoomIn 放大字体 (步进 10%)
func (t *TextArea) ZoomIn() {
	t.SetZoom(t.zoomFactor + 0.1)
}

// ZoomOut 缩小字体 (步进 10%)
func (t *TextArea) ZoomOut() {
	t.SetZoom(t.zoomFactor - 0.1)
}

// ResetZoom 重置缩放为 1.0 (100%)
func (t *TextArea) ResetZoom() {
	t.SetZoom(1.0)
}

// SetOnZoomChange 注册缩放变动回调 (percent 为 100, 125, 150 等百分比)
func (t *TextArea) SetOnZoomChange(fn func(percent int)) {
	t.onZoomChange = fn
}

// SetOnChange 注册文本变更回调（不传全文，避免每次按键都重建整篇字符串）
func (t *TextArea) SetOnChange(fn func()) {
	t.onChange = fn
}

// SetOnCursorChange 注册光标行列变更回调 (row, col 均为 1-indexed 便于状态栏呈现)
func (t *TextArea) SetOnCursorChange(fn func(row, col int)) {
	t.onCursorChange = fn
}

// IsModified 检查是否有未保存修改
func (t *TextArea) IsModified() bool {
	return t.modified
}

// SetModified 设置修改标记；传 false 表示当前状态已保存（记录保存点修订号）
func (t *TextArea) SetModified(mod bool) {
	t.modified = mod
	if !mod {
		t.savedRevision = t.revision
	}
}

// GetLineCount 获取总行数
func (t *TextArea) GetLineCount() int {
	return t.lineCount()
}

// GetCursorPosition 获取当前光标位置 (1-indexed)
func (t *TextArea) GetCursorPosition() (row, col int) {
	return t.cursorRow + 1, t.cursorCol + 1
}

// GetText 获取整篇文档文本（使用当前配置的换行序列，默认 CRLF）
func (t *TextArea) GetText() string {
	return t.GetTextWithEOL(t.lineEnding)
}

// GetTextWithEOL 以指定换行序列导出整篇文档
func (t *TextArea) GetTextWithEOL(eol string) string {
	if eol == "" {
		eol = "\r\n"
	}
	// 预计算精确 UTF-8 字节数，一次分配、逐 rune 追加，避免每行一次 string 分配
	n := t.lineCount()
	total := 0
	for r := 0; r < n; r++ {
		line := t.line(r)
		for _, ch := range line {
			total += utf8.RuneLen(ch)
		}
		total += len(eol)
	}
	buf := make([]byte, 0, total)
	for r := 0; r < n; r++ {
		for _, ch := range t.line(r) {
			buf = utf8.AppendRune(buf, ch)
		}
		if r < n-1 {
			buf = append(buf, eol...)
		}
	}
	return string(buf)
}

// SetLineEnding 设置 GetText 输出使用的换行序列（打开/保存文件时保持原文件风格）
func (t *TextArea) SetLineEnding(eol string) {
	if eol == "" {
		eol = "\r\n"
	}
	t.lineEnding = eol
}

// GetLineEnding 获取当前序列化换行序列
func (t *TextArea) GetLineEnding() string {
	if t.lineEnding == "" {
		return "\r\n"
	}
	return t.lineEnding
}

// Match 一次查找匹配区间（列基于 rune，EndCol 为开区间）
type Match struct {
	StartRow, StartCol, EndRow, EndCol int
}

// runeMatches 单字符比较（可选忽略大小写）
func runeMatches(a, b rune, caseSensitive bool) bool {
	if caseSensitive {
		return a == b
	}
	return a == b || unicode.ToLower(a) == unicode.ToLower(b)
}

// matchInLine 从 start 列开始查找 q 在 line 中的起点列
func matchInLine(line, q []rune, start int, caseSensitive bool) (int, bool) {
	if len(q) == 0 || len(line) < len(q) {
		return 0, false
	}
	if start < 0 {
		start = 0
	}
	last := len(line) - len(q)
	if start > last {
		return 0, false
	}
	for c := start; c <= last; c++ {
		matched := true
		for i := range q {
			if !runeMatches(line[c+i], q[i], caseSensitive) {
				matched = false
				break
			}
		}
		if matched {
			return c, true
		}
	}
	return 0, false
}

// matchInLineBackward 在 [0, limit) 的起始列范围内反向查找匹配（返回最后一个）
func matchInLineBackward(line, q []rune, limit int, caseSensitive bool) (int, bool) {
	if len(q) == 0 || len(line) < len(q) {
		return 0, false
	}
	last := len(line) - len(q)
	if limit-1 < last {
		last = limit - 1
	}
	for c := last; c >= 0; c-- {
		matched := true
		for i := range q {
			if !runeMatches(line[c+i], q[i], caseSensitive) {
				matched = false
				break
			}
		}
		if matched {
			return c, true
		}
	}
	return 0, false
}

// FindNext 从 (row, col) 起查找 query，找不到时自动回绕到文档首/尾。
// forward=false 时反向查找（仅返回起始列小于 col 的匹配）。
func (t *TextArea) FindNext(query string, row, col int, forward, caseSensitive bool) (Match, bool) {
	q := []rune(query)
	if len(q) == 0 || t.lineCount() == 0 {
		return Match{}, false
	}
	n := t.lineCount()
	if row < 0 {
		row = 0
	}
	if row >= n {
		row = n - 1
	}
	if col < 0 {
		col = 0
	}
	if col > len(t.line(row)) {
		col = len(t.line(row))
	}

	if forward {
		for r := row; r < n; r++ {
			start := 0
			if r == row {
				start = col
			}
			if c, ok := matchInLine(t.line(r), q, start, caseSensitive); ok {
				return Match{StartRow: r, StartCol: c, EndRow: r, EndCol: c + len(q)}, true
			}
		}
		for r := 0; r <= row; r++ {
			if c, ok := matchInLine(t.line(r), q, 0, caseSensitive); ok {
				return Match{StartRow: r, StartCol: c, EndRow: r, EndCol: c + len(q)}, true
			}
		}
		return Match{}, false
	}

	for r := row; r >= 0; r-- {
		limit := len(t.line(r))
		if r == row {
			limit = col
		}
		if c, ok := matchInLineBackward(t.line(r), q, limit, caseSensitive); ok {
			return Match{StartRow: r, StartCol: c, EndRow: r, EndCol: c + len(q)}, true
		}
	}
	for r := n - 1; r >= 0; r-- {
		if c, ok := matchInLineBackward(t.line(r), q, len(t.line(r)), caseSensitive); ok {
			return Match{StartRow: r, StartCol: c, EndRow: r, EndCol: c + len(q)}, true
		}
	}
	return Match{}, false
}

// matchAllInLine 返回行内全部不重叠匹配的起始列（FindAll 与 ReplaceAll 共用同一匹配语义）
func matchAllInLine(line, q []rune, caseSensitive bool) []int {
	return matchAllInLineInto(nil, line, q, caseSensitive)
}

// matchAllInLineInto 复用 dst 收集全部匹配列，避免每行一次切片分配
func matchAllInLineInto(dst []int, line, q []rune, caseSensitive bool) []int {
	if len(q) == 0 || len(line) < len(q) {
		return dst[:0]
	}
	for start := 0; ; {
		c, ok := matchInLine(line, q, start, caseSensitive)
		if !ok {
			return dst
		}
		dst = append(dst, c)
		start = c + len(q)
	}
}

// FindAll 返回全部不重叠匹配（按文档顺序）
func (t *TextArea) FindAll(query string, caseSensitive bool) []Match {
	q := []rune(query)
	if len(q) == 0 {
		return nil
	}
	var matches []Match
	for r := 0; r < t.lineCount(); r++ {
		for _, col := range matchAllInLine(t.line(r), q, caseSensitive) {
			matches = append(matches, Match{StartRow: r, StartCol: col, EndRow: r, EndCol: col + len(q)})
		}
	}
	return matches
}

// ReplaceCurrentSelection 当当前选区文本与 query 匹配时替换为 repl，返回是否发生替换
func (t *TextArea) ReplaceCurrentSelection(query, repl string, caseSensitive bool) bool {
	if t.readOnly || query == "" {
		return false
	}
	sr, sc, er, ec, ok := t.GetSelectionRange()
	if !ok || sr != er {
		return false
	}
	q := []rune(query)
	if ec-sc != len(q) {
		return false
	}
	line := t.line(sr)
	for i := range q {
		if !runeMatches(line[sc+i], q[i], caseSensitive) {
			return false
		}
	}
	return t.applyEdit(sr, sc, er, ec, []rune(repl))
}

// ReplaceAll 以字面量方式替换全部匹配，返回替换次数。
// 所有替换合并为一个撤销步骤。替换文本不含换行时走批量路径：先收集每行改动，
// 再一次性写入 lines/layouts 并只做一次视觉索引重建；匹配稀疏时退回逐行应用，
// 避免为了少量匹配重建整段区间。
func (t *TextArea) ReplaceAll(query, repl string, caseSensitive bool) int {
	if t.readOnly {
		return 0
	}
	q := []rune(query)
	if len(q) == 0 || t.lineCount() == 0 {
		return 0
	}
	t.clampCursor()

	inserted := []rune(repl)
	if runesContain(inserted, '\n') {
		return t.replaceAllPerRow(q, inserted, caseSensitive)
	}

	// 批量路径：先落定旧视觉索引，再收集所有行改动
	t.recomputeVisualLines()
	t.measureEpoch++

	patches := make([]rowPatch, 0, 64)
	count := 0
	firstRow, firstCol := -1, 0
	maxRow := -1
	var cols []int
	removedArena := make([]rune, 0, 256)
	insertedArena := make([]rune, 0, 256)

	for r := t.lineCount() - 1; r >= 0; r-- {
		line := t.line(r)
		cols = matchAllInLineInto(cols[:0], line, q, caseSensitive)
		if len(cols) == 0 {
			continue
		}
		start := cols[0]
		end := cols[len(cols)-1] + len(q)

		r0 := len(removedArena)
		removedArena = append(removedArena, line[start:end]...)
		removed := removedArena[r0:len(removedArena):len(removedArena)]

		i0 := len(insertedArena)
		prev := start
		for _, c := range cols {
			insertedArena = append(insertedArena, line[prev:c]...)
			insertedArena = append(insertedArena, inserted...)
			prev = c + len(q)
		}
		insertedArena = append(insertedArena, line[prev:end]...)
		seg := insertedArena[i0:len(insertedArena):len(insertedArena)]

		patches = append(patches, rowPatch{
			row: r,
			op:  editOp{row: r, col: start, removed: removed, inserted: seg},
		})
		count += len(cols)
		firstRow, firstCol = r, start
		if maxRow < 0 {
			maxRow = r
		}
	}
	if count == 0 {
		return 0
	}

	span := maxRow - firstRow + 1
	if span > len(patches)*8 {
		// 匹配稀疏：逐行提交，保持每次只重建本行视觉条目的增量特性
		return t.applyReplacePatches(patches, count, firstRow, firstCol, true)
	}
	return t.applyReplacePatches(patches, count, firstRow, firstCol, false)
}

// replaceAllPerRow 处理替换文本包含换行的情况。
// 从末行向前原地替换（前面的行偏移保持有效），最后一次性重建行偏移表，
// 把逐行 O(行数) 的偏移更新收敛为一次 O(文档) 扫描。
func (t *TextArea) replaceAllPerRow(q, inserted []rune, caseSensitive bool) int {
	rec := editRecord{
		cursorBefore: textPos{row: t.cursorRow, col: t.cursorCol},
		selBefore:    t.captureSelection(),
		revBefore:    t.revision,
	}
	count := 0
	firstRow, firstCol := -1, 0
	addedLines := 0
	var cols []int
	removedArena := make([]rune, 0, 256)
	insertedArena := make([]rune, 0, 256)
	t.measureEpoch++
	t.longPrefixes = nil

	for r := t.lineCount() - 1; r >= 0; r-- {
		line := t.line(r)
		cols = matchAllInLineInto(cols[:0], line, q, caseSensitive)
		if len(cols) == 0 {
			continue
		}
		start := cols[0]
		end := cols[len(cols)-1] + len(q)

		r0 := len(removedArena)
		removedArena = append(removedArena, line[start:end]...)
		removed := removedArena[r0:len(removedArena):len(removedArena)]

		i0 := len(insertedArena)
		prev := start
		for _, c := range cols {
			insertedArena = append(insertedArena, line[prev:c]...)
			insertedArena = append(insertedArena, inserted...)
			prev = c + len(q)
		}
		insertedArena = append(insertedArena, line[prev:end]...)
		seg := insertedArena[i0:len(insertedArena):len(insertedArena)]

		startOff := t.lineStart[r] + start
		t.replaceAt(startOff, end-start, seg)
		rec.ops = append(rec.ops, editOp{row: r, col: start, removed: removed, inserted: seg})
		addedLines += runesCount(seg, '\n')
		count += len(cols)
		firstRow, firstCol = r, start
	}
	if count == 0 {
		return 0
	}

	t.rebuildLineStarts(t.lineCount() + addedLines)
	t.layouts = make([]lineLayout, t.lineCount())
	t.longPrefixes = nil
	t.maxWidthRow = 0
	t.maxWidthDirty = true
	t.visualState = visualFull
	t.finishReplaceAll(rec, count, firstRow, firstCol)
	return count
}

// rebuildLineStarts 扫描逻辑文本重建行偏移表（hint 为预期行数，用于一次性定容）
func (t *TextArea) rebuildLineStarts(hint int) {
	if hint < 1 {
		hint = 1
	}
	starts := make([]int, 0, hint+1)
	starts = append(starts, 0)
	for i, r := range t.text[:t.gapStart] {
		if r == '\n' {
			starts = append(starts, i+1)
		}
	}
	for i, r := range t.text[t.gapEnd:] {
		if r == '\n' {
			starts = append(starts, t.gapStart+i+1)
		}
	}
	starts = append(starts, t.logicalLen())
	t.lineStart = starts
}

// runesCount 统计切片中目标 rune 的个数
func runesCount(rs []rune, target rune) int {
	n := 0
	for _, r := range rs {
		if r == target {
			n++
		}
	}
	return n
}

// applyReplacePatches 应用已收集的行内替换。
// bulk=true 时一次性写入并只重建 [firstRow,maxRow] 的视觉索引；否则逐行 replaceRange。
func (t *TextArea) applyReplacePatches(patches []rowPatch, count, firstRow, firstCol int, perRow bool) int {
	rec := editRecord{
		cursorBefore: textPos{row: t.cursorRow, col: t.cursorCol},
		selBefore:    t.captureSelection(),
		revBefore:    t.revision,
		ops:          make([]editOp, 0, len(patches)),
	}

	if perRow {
		for _, p := range patches {
			t.replaceRange(p.op.row, p.op.col, p.op.row, p.op.col+len(p.op.removed), p.op.inserted)
			rec.ops = append(rec.ops, p.op)
		}
	} else {
		maxRow := patches[0].row
		minRow := patches[len(patches)-1].row
		oldStart := t.logicalToVisual[minRow]
		oldEnd := t.logicalToVisual[maxRow+1]
		// 从后往前原地替换：较大行号的偏移先失效，较小行号的旧偏移仍有效
		for _, p := range patches {
			startOff := t.lineStart[p.op.row] + p.op.col
			t.replaceAt(startOff, len(p.op.removed), p.op.inserted)
			t.layouts[p.row] = lineLayout{}
			rec.ops = append(rec.ops, p.op)
		}
		// 行数不变，只需把每行的累计长度差加到其后的行起点
		cum, pi := 0, len(patches)-1
		for k := 1; k <= t.lineCount(); k++ {
			for pi >= 0 && patches[pi].row < k {
				cum += len(patches[pi].op.inserted) - len(patches[pi].op.removed)
				pi--
			}
			t.lineStart[k] += cum
		}
		t.maxWidthDirty = true
		t.visualState = visualPartial
		t.dirtyFrom = minRow
		t.dirtyRows = maxRow - minRow + 1
		t.dirtyRowDelta = 0
		t.dirtyOldStart = oldStart
		t.dirtyOldEnd = oldEnd
	}
	t.finishReplaceAll(rec, count, firstRow, firstCol)
	return count
}

// finishReplaceAll 提交替换后的光标、撤销记录、高亮失效与状态通知。
func (t *TextArea) finishReplaceAll(rec editRecord, count, firstRow, firstCol int) {
	t.cursorRow = firstRow
	t.cursorCol = firstCol
	t.preferredCol = t.cursorCol
	t.selectionActive = false
	t.isSelecting = false
	t.selAnchorRow, t.selAnchorCol = t.cursorRow, t.cursorCol
	t.selCursorRow, t.selCursorCol = t.cursorRow, t.cursorCol

	rec.cursorAfter = textPos{row: t.cursorRow, col: t.cursorCol}
	rec.selAfter = selectionState{active: false, anchorRow: t.cursorRow, anchorCol: t.cursorCol, cursorRow: t.cursorRow, cursorCol: t.cursorCol}
	t.nextRevision++
	rec.revAfter = t.nextRevision
	t.pushUndo(rec)

	t.revision = rec.revAfter
	t.modified = true

	t.invalidateHighlightFrom(firstRow)
	t.clampScroll()
	t.updatePreferredX()
	t.ensureCursorVisible()
	t.notifyChange()
	t.notifyCursor()
}

// runesContain 判断 rune 切片是否包含目标字符
func runesContain(runes []rune, target rune) bool {
	for _, r := range runes {
		if r == target {
			return true
		}
	}
	return false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// SetSelection 设置选区（列基于 rune），光标落在选区末尾并滚动进视口
func (t *TextArea) SetSelection(startRow, startCol, endRow, endCol int) {
	if t.lineCount() == 0 {
		return
	}
	clampPos := func(row, col int) (int, int) {
		if row < 0 {
			row = 0
		}
		if row >= t.lineCount() {
			row = t.lineCount() - 1
		}
		if col < 0 {
			col = 0
		}
		if col > len(t.line(row)) {
			col = len(t.line(row))
		}
		return row, col
	}
	startRow, startCol = clampPos(startRow, startCol)
	endRow, endCol = clampPos(endRow, endCol)
	if endRow < startRow || (endRow == startRow && endCol < startCol) {
		startRow, startCol, endRow, endCol = endRow, endCol, startRow, startCol
	}

	t.selAnchorRow, t.selAnchorCol = startRow, startCol
	t.cursorRow, t.cursorCol = endRow, endCol
	t.selCursorRow, t.selCursorCol = endRow, endCol
	t.selectionActive = !(startRow == endRow && startCol == endCol)
	t.preferredCol = endCol
	t.preferredX = 0
	t.updatePreferredX()
	t.ensureCursorVisible()
	t.notifyCursor()
}

// GetSelectionRange 返回规范化后的选区（无选区时 ok=false）
func (t *TextArea) GetSelectionRange() (startRow, startCol, endRow, endCol int, ok bool) {
	if !t.selectionActive {
		return 0, 0, 0, 0, false
	}
	sr, sc, er, ec, ok := t.getNormalizedSelection()
	if !ok {
		return 0, 0, 0, 0, false
	}
	return sr, sc, er, ec, true
}

// ScrollCursorIntoView 将光标滚动进视口
func (t *TextArea) ScrollCursorIntoView() {
	t.clampCursor()
	t.ensureCursorVisible()
	t.notifyCursor()
}

// HasSelection 是否存在活动选区
func (t *TextArea) HasSelection() bool {
	return t.selectionActive
}

// Preedit 返回当前输入法预编辑串（仅供状态展示与测试）
func (t *TextArea) Preedit() string {
	return t.preedit
}

// CanUndo 是否可撤销
func (t *TextArea) CanUndo() bool {
	return len(t.undoStack) > 0
}

// CanRedo 是否可重做
func (t *TextArea) CanRedo() bool {
	return len(t.redoStack) > 0
}

// SelectAll 全选文档
func (t *TextArea) SelectAll() {
	if t.lineCount() == 0 {
		return
	}
	last := t.lineCount() - 1
	t.selAnchorRow, t.selAnchorCol = 0, 0
	t.selCursorRow, t.selCursorCol = last, len(t.line(last))
	t.cursorRow, t.cursorCol = last, len(t.line(last))
	t.selectionActive = true
	t.preferredCol = t.cursorCol
	t.ensureCursorVisible()
	t.notifyCursor()
}

// Copy 复制当前选区到系统剪贴板
func (t *TextArea) Copy() bool {
	if !t.selectionActive {
		return false
	}
	text := t.getSelectedText()
	if text == "" {
		return false
	}
	clipboard.SetText(text)
	return true
}

// Cut 剪切当前选区
func (t *TextArea) Cut() bool {
	if t.readOnly || !t.selectionActive {
		return false
	}
	if !t.Copy() {
		return false
	}
	t.replaceSelectionWithRunes(nil)
	return true
}

// Paste 在光标处粘贴系统剪贴板文本
func (t *TextArea) Paste() bool {
	if t.readOnly {
		return false
	}
	text := clipboard.GetText()
	if text == "" {
		return false
	}
	t.insertText(text)
	return true
}

// prevWordPos 向左跳过非单词字符后再跳过单词字符，返回上一单词起点
func (t *TextArea) prevWordPos(row, col int) (int, int) {
	for {
		line := t.line(row)
		if col > 0 {
			i := col
			for i > 0 && !isWordRune(line[i-1]) {
				i--
			}
			for i > 0 && isWordRune(line[i-1]) {
				i--
			}
			return row, i
		}
		if row == 0 {
			return 0, 0
		}
		row--
		col = len(t.line(row))
	}
}

// nextWordPos 向右跳过非单词字符后再跳过单词字符，返回下一单词终点
func (t *TextArea) nextWordPos(row, col int) (int, int) {
	for {
		line := t.line(row)
		if col < len(line) {
			i := col
			for i < len(line) && !isWordRune(line[i]) {
				i++
			}
			for i < len(line) && isWordRune(line[i]) {
				i++
			}
			return row, i
		}
		if row >= t.lineCount()-1 {
			return row, col
		}
		row++
		col = 0
	}
}

// moveWord 按单词粒度移动光标（不处理选区，由调用方 updateSelection）
func (t *TextArea) moveWord(forward bool) {
	if forward {
		t.cursorRow, t.cursorCol = t.nextWordPos(t.cursorRow, t.cursorCol)
	} else {
		t.cursorRow, t.cursorCol = t.prevWordPos(t.cursorRow, t.cursorCol)
	}
	t.preferredCol = t.cursorCol
	t.updatePreferredX()
}

// deleteWordBackward 删除光标左侧一个单词（Ctrl+Backspace）
func (t *TextArea) deleteWordBackward() bool {
	sr, sc := t.prevWordPos(t.cursorRow, t.cursorCol)
	if sr == t.cursorRow && sc == t.cursorCol {
		return false
	}
	return t.applyEdit(sr, sc, t.cursorRow, t.cursorCol, nil)
}

// deleteWordForward 删除光标右侧一个单词（Ctrl+Delete）
func (t *TextArea) deleteWordForward() bool {
	er, ec := t.nextWordPos(t.cursorRow, t.cursorCol)
	if er == t.cursorRow && ec == t.cursorCol {
		return false
	}
	return t.applyEdit(t.cursorRow, t.cursorCol, er, ec, nil)
}

// enterInsertion 计算回车应插入的内容（换行 + 当前行缩进）
func (t *TextArea) enterInsertion() []rune {
	row, col := t.cursorRow, t.cursorCol
	if sr, sc, _, _, ok := t.getNormalizedSelection(); ok {
		row, col = sr, sc
	}
	if row < 0 || row >= t.lineCount() {
		row = 0
	}
	if row >= t.lineCount() {
		return []rune{'\n'}
	}
	line := t.line(row)
	if col > len(line) {
		col = len(line)
	}

	indent := make([]rune, 0, 8)
	for _, r := range line {
		if r != ' ' && r != '\t' {
			break
		}
		indent = append(indent, r)
	}

	out := make([]rune, 0, len(indent)+5)
	out = append(out, '\n')
	out = append(out, indent...)

	// 前一个非空字符是 '{' 时额外缩进一级
	i := col - 1
	for i >= 0 && (line[i] == ' ' || line[i] == '\t') {
		i--
	}
	if i >= 0 && line[i] == '{' {
		out = append(out, ' ', ' ', ' ', ' ')
	}
	return out
}

// outdentSelection 减少当前行或选区覆盖行的缩进（Shift+Tab）
func (t *TextArea) outdentSelection() bool {
	if t.readOnly {
		return false
	}
	sr, _, er, _, ok := t.getNormalizedSelection()
	if !ok {
		sr, er = t.cursorRow, t.cursorRow
	}
	if sr > er {
		sr, er = er, sr
	}

	type outdent struct{ row, remove int }
	var todo []outdent
	for r := sr; r <= er; r++ {
		line := t.line(r)
		n := 0
		for n < len(line) && n < 4 && line[n] == ' ' {
			n++
		}
		if n == 0 && len(line) > 0 && line[0] == '\t' {
			n = 1
		}
		if n > 0 {
			todo = append(todo, outdent{row: r, remove: n})
		}
	}
	if len(todo) == 0 {
		return false
	}

	rec := editRecord{
		cursorBefore: textPos{row: t.cursorRow, col: t.cursorCol},
		selBefore:    t.captureSelection(),
		revBefore:    t.revision,
	}
	removedByRow := make(map[int]int, len(todo))
	for i := len(todo) - 1; i >= 0; i-- {
		o := todo[i]
		removed := cloneRunes(t.line(o.row)[:o.remove])
		t.replaceRange(o.row, 0, o.row, o.remove, nil)
		rec.ops = append(rec.ops, editOp{row: o.row, col: 0, removed: removed})
		removedByRow[o.row] = o.remove
	}

	adjust := func(row, col int) (int, int) {
		if n, ok := removedByRow[row]; ok {
			col -= n
			if col < 0 {
				col = 0
			}
		}
		return row, col
	}
	t.cursorRow, t.cursorCol = adjust(t.cursorRow, t.cursorCol)
	t.selAnchorRow, t.selAnchorCol = adjust(t.selAnchorRow, t.selAnchorCol)
	t.selCursorRow, t.selCursorCol = adjust(t.selCursorRow, t.selCursorCol)
	t.preferredCol = t.cursorCol

	rec.cursorAfter = textPos{row: t.cursorRow, col: t.cursorCol}
	rec.selAfter = t.captureSelection()
	t.nextRevision++
	rec.revAfter = t.nextRevision
	t.pushUndo(rec)
	t.revision = rec.revAfter
	t.modified = true
	t.lastEditKind = 0
	t.lastEditAt = time.Time{}

	t.invalidateHighlightFrom(sr)
	t.clampScroll()
	t.updatePreferredX()
	t.ensureCursorVisible()
	t.notifyChange()
	t.notifyCursor()
	return true
}

// indentSelection 对选区覆盖的所有逻辑行增加一级缩进（Tab）。
// 仅在选区跨行时生效；单行选区保持“用缩进替换选区”的原有行为。
func (t *TextArea) indentSelection() bool {
	if t.readOnly {
		return false
	}
	sr, _, er, _, ok := t.getNormalizedSelection()
	if !ok || sr >= er {
		return false
	}

	rec := editRecord{
		cursorBefore: textPos{row: t.cursorRow, col: t.cursorCol},
		selBefore:    t.captureSelection(),
		revBefore:    t.revision,
	}
	indent := []rune("    ")
	for r := er; r >= sr; r-- {
		t.replaceRange(r, 0, r, 0, indent)
		rec.ops = append(rec.ops, editOp{row: r, col: 0, inserted: indent})
	}

	adjust := func(row, col int) (int, int) {
		if row >= sr && row <= er {
			col += len(indent)
		}
		return row, col
	}
	t.cursorRow, t.cursorCol = adjust(t.cursorRow, t.cursorCol)
	t.selAnchorRow, t.selAnchorCol = adjust(t.selAnchorRow, t.selAnchorCol)
	t.selCursorRow, t.selCursorCol = adjust(t.selCursorRow, t.selCursorCol)
	t.preferredCol = t.cursorCol

	rec.cursorAfter = textPos{row: t.cursorRow, col: t.cursorCol}
	rec.selAfter = t.captureSelection()
	t.nextRevision++
	rec.revAfter = t.nextRevision
	t.pushUndo(rec)
	t.revision = rec.revAfter
	t.modified = true
	t.lastEditKind = 0
	t.lastEditAt = time.Time{}

	t.invalidateHighlightFrom(sr)
	t.clampScroll()
	t.updatePreferredX()
	t.ensureCursorVisible()
	t.notifyChange()
	t.notifyCursor()
	return true
}

// SetText 设置整篇文档文本并重置光标与历史。
// 整篇文本写入单个 rune backing array，各行以 cap==len 的子切片视图引用它：
// 装载 10 万行从 10 万次分配降为个位数，编辑仍只替换受影响行。
func (t *TextArea) SetText(text string) {
	t.installText(text)
	t.longPrefixes = nil
	t.layouts = make([]lineLayout, t.lineCount())

	t.cursorRow = 0
	t.cursorCol = 0
	t.preferredCol = 0
	t.preferredX = 0
	t.selectionActive = false
	t.isSelecting = false
	t.scrollOffsetY = 0
	t.scrollOffsetX = 0
	t.undoStack = t.undoStack[:0]
	t.redoStack = t.redoStack[:0]
	t.undoBytes = 0
	t.redoBytes = 0
	t.modified = false
	t.revision = 0
	t.savedRevision = 0
	t.nextRevision = 0
	t.lineStates = nil
	t.statesValidTo = 0
	t.spansCache = make(map[int][]syntax.Span)
	t.visualState = visualFull
	// 预分配视觉索引容量：无折行时视觉行数 == 逻辑行数，
	// 避免首帧渲染时 visualLines/logicalToVisual 反复扩容拷贝。
	lineCap := t.lineCount()
	if cap(t.visualLines) < lineCap {
		t.visualLines = make([]visualLine, 0, lineCap)
	} else {
		t.visualLines = t.visualLines[:0]
	}
	if cap(t.logicalToVisual) < lineCap+1 {
		t.logicalToVisual = make([]int, 0, lineCap+1)
	} else {
		t.logicalToVisual = t.logicalToVisual[:0]
	}
	if t.highlighter != nil {
		t.lineStates = make([]int, lineCap)
	}
	t.preedit = ""
	t.lastEditAt = time.Time{}
	t.clickCount = 0
	t.maxContentWidth = 0
	t.maxWidthRow = 0
	t.maxWidthDirty = false
	t.notifyCursor()
}

// ReplaceDocument 用新文本整体替换文档，作为单步可撤销编辑。
// 与 SetText 不同，它保留撤销/重做历史并把文档标记为已修改；
// 光标保持原来的逻辑行列，越界时收敛到新文本末尾。
func (t *TextArea) ReplaceDocument(text string) bool {
	if t.readOnly {
		return false
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	row, col := t.cursorRow, t.cursorCol
	last := t.lineCount() - 1
	if !t.applyEdit(0, 0, last, len(t.line(last)), []rune(text)) {
		return false
	}

	t.cursorRow, t.cursorCol = row, col
	t.clampCursor()
	t.preferredCol = t.cursorCol
	t.selectionActive = false
	t.isSelecting = false
	t.updatePreferredX()
	t.ensureCursorVisible()
	t.notifyCursor()
	return true
}

const (
	maxUndoEntries    = 2000
	maxUndoBytes      = 32 << 20 // 撤销栈最多保留约 32MB 被改动文本
	maxCachedSpanRows = 512      // 语法 span 行缓存上限（约 6 个视口）
)

// editRecordBytes 估算一条撤销记录的真实堆占用：结构体、ops 后备数组与 rune 缓冲区容量。
// 只按 len 计算会低估 header/分配器对齐带来的开销，使 32MB 上限实际占用数倍内存。
func editRecordBytes(rec editRecord) int {
	total := int(unsafe.Sizeof(rec)) + cap(rec.ops)*int(unsafe.Sizeof(editOp{}))
	for _, op := range rec.ops {
		total += cap(op.removed)*4 + cap(op.inserted)*4
	}
	return total
}

// editRecordFirstRow 返回记录中最早受影响的行号（用于高亮状态失效）
func editRecordFirstRow(rec editRecord) int {
	if len(rec.ops) == 0 {
		return 0
	}
	row := rec.ops[0].row
	for _, op := range rec.ops[1:] {
		if op.row < row {
			row = op.row
		}
	}
	return row
}

// pushUndo 压入一条差分编辑记录，并按字节数/条数裁剪历史
func (t *TextArea) pushUndo(rec editRecord) {
	t.undoStack = append(t.undoStack, rec)
	t.undoBytes += editRecordBytes(rec)
	for len(t.undoStack) > maxUndoEntries || (t.undoBytes > maxUndoBytes && len(t.undoStack) > 1) {
		t.undoBytes -= editRecordBytes(t.undoStack[0])
		t.undoStack[0] = editRecord{}
		t.undoStack = t.undoStack[1:]
	}
	// 新的编辑使重做历史失效
	for i := range t.redoStack {
		t.redoStack[i] = editRecord{}
	}
	t.redoStack = t.redoStack[:0]
	t.redoBytes = 0
}

// pushRedo 压入重做记录并限制内存
func (t *TextArea) pushRedo(rec editRecord) {
	t.redoStack = append(t.redoStack, rec)
	t.redoBytes += editRecordBytes(rec)
	for len(t.redoStack) > maxUndoEntries || (t.redoBytes > maxUndoBytes && len(t.redoStack) > 1) {
		t.redoBytes -= editRecordBytes(t.redoStack[0])
		t.redoStack[0] = editRecord{}
		t.redoStack = t.redoStack[1:]
	}
}

// cloneRunes 返回独立的 rune 拷贝，杜绝与文档底层数组共享导致的覆盖损坏
func cloneRunes(src []rune) []rune {
	return append([]rune(nil), src...)
}

// consumeWheelDelta 把 Windows 滚轮消息的 1/120 刻度增量换算成整数像素/步进。
// delta 为原始增量（触摸板会发送远小于 120 的分数值），unit 为一个标准刻度对应的像素数。
// 不足一个像素的余数保留到下一次事件，避免整数除法把精密滚动完全吞掉。
func consumeWheelDelta(remainder *int64, delta, unit int64) int32 {
	if remainder == nil || delta == 0 || unit == 0 {
		return 0
	}
	*remainder += delta * unit
	steps := *remainder / 120
	*remainder -= steps * 120
	return int32(steps)
}

// advancePos 计算从 (row,col) 起插入 text 之后的结束位置
func advancePos(row, col int, text []rune) textPos {
	for _, r := range text {
		if r == '\n' {
			row++
			col = 0
		} else {
			col++
		}
	}
	return textPos{row: row, col: col}
}

// extractRange 取出指定区间的文本（跨行以 '\n' 连接，返回独立拷贝）
func (t *TextArea) extractRange(sr, sc, er, ec int) []rune {
	start := t.lineStart[sr] + sc
	end := t.lineStart[er] + ec
	if end < start {
		return nil
	}
	return t.cloneLogicalRange(start, end)
}

// replaceRange 用 inserted 替换 [start,end) 区间。
// 文本层只在目标偏移处挪动间隙并写入增量，不再克隆整行；
// 行偏移表与排版缓存按行数增量维护。
func (t *TextArea) replaceRange(startRow, startCol, endRow, endCol int, inserted []rune) {
	t.measureEpoch++
	t.truncatePrefixForEdit(startRow, startCol, endRow, endCol)
	// 变更前先落定视觉行索引，保证 spliceLayouts 能捕获正确的旧视觉区间
	t.recomputeVisualLines()

	startOff := t.lineStart[startRow] + startCol
	endOff := t.lineStart[endRow] + endCol
	oldCount := endRow - startRow + 1
	newCount := 1
	for _, r := range inserted {
		if r == '\n' {
			newCount++
		}
	}
	delta := len(inserted) - (endOff - startOff)

	// 单行内编辑且该行宽度已测量：按插入/删除字符的步进宽度增量维护 totalWidth，
	// 避免为了刷新上界而重测整行（20 万字符行从 ~1.6 ms 降到微秒级）。
	presetWidth, hasPreset := int32(0), false
	if startRow == endRow && startCol <= endCol && t.layouts[startRow].widthOK &&
		!runesContain(inserted, '\n') {
		fm := render.GetFontManager()
		removed := t.line(startRow)[startCol:endCol]
		w := t.layouts[startRow].totalWidth -
			fm.MeasureRunesFace(t.fontFace, removed) +
			fm.MeasureRunesFace(t.fontFace, inserted)
		if w < 0 {
			w = 0
		}
		presetWidth, hasPreset = w, true
	}

	t.replaceAt(startOff, endOff-startOff, inserted)
	t.spliceLineStarts(startRow, oldCount, startOff, endOff-startOff, inserted, delta)
	t.spliceLayouts(startRow, oldCount, newCount, presetWidth, hasPreset)
}

// spliceLayouts 在行结构变化后同步增量排版缓存，并把视觉行索引标记为局部脏
func (t *TextArea) spliceLayouts(start, oldCount, newCount int, presetWidth int32, hasPreset bool) {
	if oldCount != newCount {
		t.longPrefixes = nil // 行号整体平移，旧检查点键失效
	}
	oldRows := t.lineCount() - newCount + oldCount
	if len(t.layouts) != oldRows || len(t.logicalToVisual) != oldRows+1 {
		// 防御性重同步：缓存长度明明不一致时退回整体重建
		t.layouts = make([]lineLayout, t.lineCount())
		t.visualState = visualFull
		t.maxContentWidth = 0
		t.maxWidthRow = 0
		t.maxWidthDirty = false
		return
	}

	// 此时 logicalToVisual 仍对应当前变更前的文档，可直接查出旧视觉区间
	oldStart := t.logicalToVisual[start]
	endRow := start + oldCount
	var oldEnd int
	if endRow < len(t.logicalToVisual) {
		oldEnd = t.logicalToVisual[endRow]
	} else {
		oldEnd = len(t.visualLines)
	}

	oldLen := len(t.layouts)
	delta := newCount - oldCount
	if delta > 0 {
		t.layouts = append(t.layouts, make([]lineLayout, delta)...)
		copy(t.layouts[start+newCount:], t.layouts[start+oldCount:oldLen])
	} else if delta < 0 {
		copy(t.layouts[start+newCount:], t.layouts[start+oldCount:])
		t.layouts = t.layouts[:oldLen+delta]
	}
	for i := start; i < start+newCount; i++ {
		t.layouts[i] = lineLayout{}
	}
	if hasPreset && newCount == 1 {
		t.layouts[start] = lineLayout{totalWidth: presetWidth, widthOK: true}
	}

	// 最长行可能被删除、替换或随行号平移：惰性排版下先立即测量替换后的行，
	// 再尝试重算上界，避免删除最长行后仍允许滚进空白区域。
	if start <= t.maxWidthRow && t.maxWidthRow < start+oldCount {
		t.maxWidthDirty = true
		for r := start; r < start+newCount; r++ {
			t.ensureRowWidth(r) // hasPreset 时命中 widthOK，直接返回
		}
		t.refreshMaxContentWidth()
	} else if delta != 0 && t.maxWidthRow >= start+oldCount {
		t.maxWidthRow += delta
	}
	if hasPreset && presetWidth > t.maxContentWidth {
		t.maxContentWidth = presetWidth
		t.maxWidthRow = start
		t.maxWidthDirty = false
	}

	t.visualState = visualPartial
	t.dirtyFrom = start
	t.dirtyRows = newCount
	t.dirtyRowDelta = delta
	t.dirtyOldStart = oldStart
	t.dirtyOldEnd = oldEnd
}

// captureSelection 获取当前选区状态
func (t *TextArea) captureSelection() selectionState {
	return selectionState{
		active:    t.selectionActive,
		anchorRow: t.selAnchorRow,
		anchorCol: t.selAnchorCol,
		cursorRow: t.selCursorRow,
		cursorCol: t.selCursorCol,
	}
}

// restoreSelection 恢复选区状态
func (t *TextArea) restoreSelection(sel selectionState) {
	t.selectionActive = sel.active
	t.selAnchorRow = sel.anchorRow
	t.selAnchorCol = sel.anchorCol
	t.selCursorRow = sel.cursorRow
	t.selCursorCol = sel.cursorCol
}

// applyEdit 用 inserted 替换指定区间，并记录可逆的差分编辑
func (t *TextArea) applyEdit(startRow, startCol, endRow, endCol int, inserted []rune) bool {
	t.clampCursor()
	if startRow < 0 {
		startRow = 0
	}
	if startRow >= t.lineCount() {
		startRow = t.lineCount() - 1
	}
	if startCol < 0 {
		startCol = 0
	}
	if startCol > len(t.line(startRow)) {
		startCol = len(t.line(startRow))
	}
	if endRow < startRow {
		endRow, startRow = startRow, endRow
	}
	if endRow >= t.lineCount() {
		endRow = t.lineCount() - 1
	}
	if endCol < 0 {
		endCol = 0
	}
	if endCol > len(t.line(endRow)) {
		endCol = len(t.line(endRow))
	}
	if startRow == endRow && startCol > endCol {
		startCol, endCol = endCol, startCol
	}

	removed := t.extractRange(startRow, startCol, endRow, endCol)
	if len(removed) == 0 && len(inserted) == 0 {
		return false
	}

	// 判定单字符编辑类型，供撤销合组使用
	kind := 0
	switch {
	case len(removed) == 0 && len(inserted) == 1 && inserted[0] != '\n':
		kind = 1 // 输入（换行单独成步，不与前面的输入合并）
	case len(inserted) == 0 && len(removed) == 1:
		if startRow < t.cursorRow || (startRow == t.cursorRow && startCol < t.cursorCol) {
			kind = 2 // 退格（向左删除）
		} else {
			kind = 3 // Delete（向右删除）
		}
	}

	cursorBefore := textPos{row: t.cursorRow, col: t.cursorCol}
	selBefore := t.captureSelection()
	revBefore := t.revision

	oldLineCount := t.lineCount()
	t.replaceRange(startRow, startCol, endRow, endCol, inserted)
	rowDelta := t.lineCount() - oldLineCount

	after := advancePos(startRow, startCol, inserted)
	selAfter := selectionState{active: false, anchorRow: after.row, anchorCol: after.col, cursorRow: after.row, cursorCol: after.col}
	t.nextRevision++
	revAfter := t.nextRevision

	// 连续单字符输入/删除直接并入上一条撤销记录：
	// 合并路径不构造 rec，也不克隆 inserted，键入热路径零分配。
	merged := t.tryMergeUndo(kind, startRow, startCol, inserted, removed, after, selAfter, revAfter)
	if !merged {
		t.pushUndo(editRecord{
			ops: []editOp{{
				row:      startRow,
				col:      startCol,
				removed:  removed,
				inserted: cloneRunes(inserted),
			}},
			cursorBefore: cursorBefore,
			cursorAfter:  after,
			selBefore:    selBefore,
			selAfter:     selAfter,
			revBefore:    revBefore,
			revAfter:     revAfter,
		})
	}
	t.lastEditAt = time.Now()
	t.lastEditKind = kind

	t.cursorRow = after.row
	t.cursorCol = after.col
	t.preferredCol = after.col
	t.selectionActive = false
	t.isSelecting = false
	t.revision = revAfter
	t.modified = true

	t.invalidateHighlightForEdit(startRow, after.row, rowDelta)
	t.clampScroll()
	t.updatePreferredX()
	t.ensureCursorVisible()
	t.notifyChange()
	t.notifyCursor()
	return true
}

// tryMergeUndo 尝试把本次单字符编辑合并进上一条撤销记录，返回是否已合并。
// 热路径直接追加到已有 inserted/removed 缓冲，不创建新记录。
func (t *TextArea) tryMergeUndo(kind, row, col int, inserted, removed []rune, after textPos, selAfter selectionState, revAfter uint64) bool {
	if kind == 0 || kind != t.lastEditKind || len(t.undoStack) == 0 {
		return false
	}
	if t.selectionActive || time.Since(t.lastEditAt) > 700*time.Millisecond {
		return false
	}
	top := &t.undoStack[len(t.undoStack)-1]
	if len(top.ops) != 1 {
		return false
	}
	// 保存点必须是独立撤销边界，否则一次撤销会越过保存点
	if top.revAfter == t.savedRevision {
		return false
	}
	last := &top.ops[0]
	if row != last.row {
		return false
	}

	oldBytes := editRecordBytes(*top)
	merged := false
	switch kind {
	case 1: // 连续输入
		if len(inserted) == 1 && inserted[0] != '\n' && len(last.removed) == 0 &&
			len(last.inserted) > 0 && len(last.inserted) < 512 &&
			last.inserted[len(last.inserted)-1] != '\n' &&
			col == last.col+len(last.inserted) {
			last.inserted = append(last.inserted, inserted[0])
			merged = true
		}
	case 2: // 连续退格
		if len(removed) == 1 && removed[0] != '\n' && len(inserted) == 0 &&
			len(last.removed) > 0 && len(last.removed) < 512 &&
			last.removed[0] != '\n' &&
			col+1 == last.col {
			last.removed = append([]rune{removed[0]}, last.removed...)
			last.col = col
			merged = true
		}
	case 3: // 连续 Delete
		if len(removed) == 1 && removed[0] != '\n' && len(inserted) == 0 &&
			len(last.removed) > 0 && len(last.removed) < 512 &&
			col == last.col {
			last.removed = append(last.removed, removed[0])
			merged = true
		}
	}
	if !merged {
		return false
	}

	top.cursorAfter = after
	top.selAfter = selAfter
	top.revAfter = revAfter
	t.undoBytes += editRecordBytes(*top) - oldBytes
	return true
}

// replaceSelectionWithRunes 用给定文本替换当前选区（无选区时在光标处插入）
func (t *TextArea) replaceSelectionWithRunes(inserted []rune) bool {
	if sr, sc, er, ec, ok := t.getNormalizedSelection(); ok {
		return t.applyEdit(sr, sc, er, ec, inserted)
	}
	return t.applyEdit(t.cursorRow, t.cursorCol, t.cursorRow, t.cursorCol, inserted)
}

// Undo 撤销操作
func (t *TextArea) Undo() bool {
	if len(t.undoStack) == 0 {
		return false
	}
	rec := t.undoStack[len(t.undoStack)-1]
	t.undoStack[len(t.undoStack)-1] = editRecord{}
	t.undoStack = t.undoStack[:len(t.undoStack)-1]
	t.undoBytes -= editRecordBytes(rec)

	// 反向应用：按记录的应用顺序逆序回放，删除当初插入的内容并恢复被删除的内容
	for i := len(rec.ops) - 1; i >= 0; i-- {
		op := rec.ops[i]
		end := advancePos(op.row, op.col, op.inserted)
		t.replaceRange(op.row, op.col, end.row, end.col, op.removed)
	}

	t.cursorRow = rec.cursorBefore.row
	t.cursorCol = rec.cursorBefore.col
	t.restoreSelection(rec.selBefore)
	t.revision = rec.revBefore
	t.modified = t.revision != t.savedRevision
	t.preferredCol = t.cursorCol
	t.isSelecting = false
	t.lastEditKind = 0
	t.lastEditAt = time.Time{}

	t.invalidateHighlightFrom(editRecordFirstRow(rec))
	t.clampScroll()
	t.updatePreferredX()
	t.ensureCursorVisible()
	t.notifyChange()
	t.notifyCursor()

	t.pushRedo(rec)
	return true
}

// Redo 重做操作
func (t *TextArea) Redo() bool {
	if len(t.redoStack) == 0 {
		return false
	}
	rec := t.redoStack[len(t.redoStack)-1]
	t.redoStack[len(t.redoStack)-1] = editRecord{}
	t.redoStack = t.redoStack[:len(t.redoStack)-1]
	t.redoBytes -= editRecordBytes(rec)

	// 正向回放
	for _, op := range rec.ops {
		end := advancePos(op.row, op.col, op.removed)
		t.replaceRange(op.row, op.col, end.row, end.col, op.inserted)
	}

	t.cursorRow = rec.cursorAfter.row
	t.cursorCol = rec.cursorAfter.col
	t.restoreSelection(rec.selAfter)
	t.revision = rec.revAfter
	t.modified = t.revision != t.savedRevision
	t.preferredCol = t.cursorCol
	t.isSelecting = false
	t.lastEditKind = 0
	t.lastEditAt = time.Time{}

	t.invalidateHighlightFrom(editRecordFirstRow(rec))
	t.clampScroll()
	t.updatePreferredX()
	t.ensureCursorVisible()
	t.notifyChange()
	t.notifyCursor()

	// 放回撤销栈（不触发 redo 清空）
	t.undoStack = append(t.undoStack, rec)
	t.undoBytes += editRecordBytes(rec)
	for len(t.undoStack) > maxUndoEntries || (t.undoBytes > maxUndoBytes && len(t.undoStack) > 1) {
		t.undoBytes -= editRecordBytes(t.undoStack[0])
		t.undoStack[0] = editRecord{}
		t.undoStack = t.undoStack[1:]
	}
	return true
}

// clampCursor 保证光标行列不越界
func (t *TextArea) clampCursor() {
	t.ensureTextInitialized()
	if t.cursorRow < 0 {
		t.cursorRow = 0
	}
	if t.cursorRow >= t.lineCount() {
		t.cursorRow = t.lineCount() - 1
	}
	lineLen := len(t.line(t.cursorRow))
	if t.cursorCol < 0 {
		t.cursorCol = 0
	}
	if t.cursorCol > lineLen {
		t.cursorCol = lineLen
	}
}

// getGutterWidth 计算行号槽实际占用宽度
func (t *TextArea) getGutterWidth() int32 {
	if !t.showLineNumbers {
		return 0
	}
	lineCount := t.lineCount()
	digits := 1
	for lineCount >= 10 {
		digits++
		lineCount /= 10
	}
	if digits < 2 {
		digits = 2
	}
	return int32(digits)*t.charWidth + 18
}

// getNormalizedSelection 返回选区的规范化起点与终点 (start <= end)
func (t *TextArea) getNormalizedSelection() (startRow, startCol, endRow, endCol int, hasSelection bool) {
	if !t.selectionActive {
		return 0, 0, 0, 0, false
	}
	aRow, aCol := t.selAnchorRow, t.selAnchorCol
	cRow, cCol := t.selCursorRow, t.selCursorCol

	if aRow < cRow || (aRow == cRow && aCol <= cCol) {
		return aRow, aCol, cRow, cCol, true
	}
	return cRow, cCol, aRow, aCol, true
}

// deleteSelection 删除选区内容并重置光标
func (t *TextArea) deleteSelection() {
	if !t.selectionActive {
		return
	}
	t.replaceSelectionWithRunes(nil)
}

// selectedRunes 返回选区内容的独立 rune 拷贝（跨行以 '\n' 连接）
func (t *TextArea) selectedRunes() []rune {
	sRow, sCol, eRow, eCol, has := t.getNormalizedSelection()
	if !has {
		return nil
	}
	return t.extractRange(sRow, sCol, eRow, eCol)
}

// getSelectedText 获取选中文本
func (t *TextArea) getSelectedText() string {
	runes := t.selectedRunes()
	if len(runes) == 0 {
		return ""
	}
	// 按文档当前换行风格输出，避免 LF 文件复制到外部编辑器时被强行改写为 CRLF
	return strings.ReplaceAll(string(runes), "\n", t.GetLineEnding())
}

// measureTextX 计算某行第 col 个字符前缀的物理像素宽度 (无堆内存分配)
func (t *TextArea) measureTextX(line []rune, col int) int32 {
	if col <= 0 || len(line) == 0 {
		return 0
	}
	if col > len(line) {
		col = len(line)
	}
	return render.GetFontManager().MeasureRunesFace(t.fontFace, line[:col])
}

// getColFromX 依据物理像素 X 偏移反算行内字符索引 col (无堆内存分配)
func (t *TextArea) getColFromX(line []rune, targetX int32) int {
	if targetX <= 0 || len(line) == 0 {
		return 0
	}
	fm := render.GetFontManager()
	currW := int32(0)
	for i, r := range line {
		rw := fm.RuneWidthFace(t.fontFace, r)
		if currW+rw >= targetX {
			if targetX-currW > rw/2 {
				return i + 1
			}
			return i
		}
		currW += rw
	}
	return len(line)
}

// getAvailWrapWidth 计算自动换行时的最大可用文本行宽度
func (t *TextArea) getAvailWrapWidth() int32 {
	gutterW := t.getGutterWidth()
	avail := t.bounds.Width - gutterW - t.margin*2 - 12
	minW := t.charWidth * 4
	if minW < 32 {
		minW = 32
	}
	if avail < minW {
		avail = minW
	}
	return avail
}

const (
	visualClean   = iota // 视觉行索引与文档完全一致
	visualPartial        // 仅 dirty 区域需要重算
	visualFull           // 需要整体重建
)

// ensureRowWidth 确保第 r 行的整行宽度缓存有效（折行点按需即时计算，不再常驻）
func (t *TextArea) ensureRowWidth(r int) {
	lay := &t.layouts[r]
	if lay.widthOK {
		return
	}
	line := t.line(r)
	lay.totalWidth = render.GetFontManager().MeasureRunesFace(t.fontFace, line)
	lay.widthOK = true
	switch {
	case lay.totalWidth > t.maxContentWidth:
		t.maxContentWidth = lay.totalWidth
		t.maxWidthRow = r
	case r == t.maxWidthRow:
		// 最长行被编辑/替换后可能已变短，旧上界不再可靠；
		// 若所有行都已测量，立即重算，避免惰性排版下上界长期偏高。
		t.maxWidthDirty = true
		t.refreshMaxContentWidth()
	}
}

// wrapBreaks 计算单行在给定宽度下的折行点，结果写入复用缓冲。
func (t *TextArea) wrapBreaks(line []rune, availWidth int32) []int32 {
	breaks := t.breakScratch[:0]
	fm := render.GetFontManager()
	n := len(line)
	start := 0
	for start < n {
		currWidth := int32(0)
		lastBreak := -1
		end := start

		for end < n {
			rw := fm.RuneWidthFace(t.fontFace, line[end])
			if currWidth+rw > availWidth && end > start {
				break
			}
			currWidth += rw
			rChar := line[end]

			// 判断词边界 / 标点 / CJK 断行点
			if rChar == ' ' || rChar == '\t' {
				lastBreak = end + 1
			} else if rChar == ',' || rChar == '.' || rChar == ';' || rChar == ':' ||
				rChar == '!' || rChar == '?' || rChar == '-' || rChar == '/' ||
				rChar == '\\' || rChar == ')' || rChar == ']' || rChar == '}' ||
				rChar == '>' || rChar == '，' || rChar == '。' || rChar == '；' ||
				rChar == '！' || rChar == '？' || rChar == '：' || rChar == '）' ||
				rChar == '】' || rChar == '》' || rChar == '、' {
				lastBreak = end + 1
			} else if rChar >= 0x2E80 { // 中日韩汉字与符号
				lastBreak = end + 1
			}

			end++
		}

		actualEnd := end
		if end < n && lastBreak > start && lastBreak <= end {
			actualEnd = lastBreak
		}
		breaks = append(breaks, int32(actualEnd))
		start = actualEnd
	}
	t.breakScratch = breaks
	return breaks
}

// appendRowVisuals 把逻辑行 r 的视觉行追加到 dst
func (t *TextArea) appendRowVisuals(dst []visualLine, r int, availWidth int32) []visualLine {
	line := t.line(r)
	if len(line) == 0 || !t.wordWrap {
		// 关闭折行时宽度按需测量（可见行渲染时），此处不做字形测量
		return append(dst, visualLine{
			logicalRow: r,
			startCol:   0,
			endCol:     len(line),
			isFirstSub: true,
			isLastSub:  true,
		})
	}
	if t.lineFitsUpperBound(line, availWidth) {
		// 等宽字体下可证明整行不会折行：同样无需精确测量
		return append(dst, visualLine{
			logicalRow: r,
			startCol:   0,
			endCol:     len(line),
			isFirstSub: true,
			isLastSub:  true,
		})
	}
	t.ensureRowWidth(r)
	if t.layouts[r].totalWidth <= availWidth {
		return append(dst, visualLine{
			logicalRow: r,
			startCol:   0,
			endCol:     len(line),
			isFirstSub: true,
			isLastSub:  true,
		})
	}

	breaks := t.wrapBreaks(line, availWidth)
	start := 0
	last := len(breaks) - 1
	for i, breakCol := range breaks {
		end := int(breakCol)
		dst = append(dst, visualLine{
			logicalRow: r,
			startCol:   start,
			endCol:     end,
			isFirstSub: i == 0,
			isLastSub:  i == last,
		})
		start = end
	}
	return dst
}

// rebuildVisualIndexAll 整体重建视觉行索引（文本替换、字体/宽度变化时使用）
func (t *TextArea) rebuildVisualIndexAll(availWidth int32) {
	t.visualLines = t.visualLines[:0]
	t.logicalToVisual = t.logicalToVisual[:0]
	for r := 0; r < t.lineCount(); r++ {
		t.logicalToVisual = append(t.logicalToVisual, len(t.visualLines))
		t.visualLines = t.appendRowVisuals(t.visualLines, r, availWidth)
	}
	// 末位哨兵 = 视觉行总数
	t.logicalToVisual = append(t.logicalToVisual, len(t.visualLines))
	t.visualState = visualClean
}

// rebuildVisualIndexRange 仅重建 spliceLayouts 标记的局部区域，普通按键 O(1)
func (t *TextArea) rebuildVisualIndexRange(availWidth int32) {
	n := t.lineCount()
	start := t.dirtyFrom
	rows := t.dirtyRows
	if start < 0 {
		start = 0
	}
	if start > n {
		start = n
	}
	if start+rows > n {
		rows = n - start
	}
	oldStart, oldEnd := t.dirtyOldStart, t.dirtyOldEnd
	if oldStart < 0 || oldEnd < oldStart || oldStart > len(t.visualLines) || oldEnd > len(t.visualLines) {
		t.rebuildVisualIndexAll(availWidth)
		return
	}

	tmp := t.visualScratch[:0]
	for r := start; r < start+rows; r++ {
		tmp = t.appendRowVisuals(tmp, r, availWidth)
	}
	t.visualScratch = tmp

	oldLen := len(t.visualLines)
	visualDelta := len(tmp) - (oldEnd - oldStart)
	newLen := oldLen + visualDelta
	if visualDelta == 0 {
		copy(t.visualLines[oldStart:], tmp)
	} else if newLen > cap(t.visualLines) {
		grown := make([]visualLine, newLen)
		copy(grown, t.visualLines[:oldStart])
		copy(grown[oldStart:], tmp)
		copy(grown[oldStart+len(tmp):], t.visualLines[oldEnd:])
		t.visualLines = grown
	} else {
		t.visualLines = t.visualLines[:newLen]
		copy(t.visualLines[oldStart+len(tmp):], t.visualLines[oldEnd:oldLen])
		copy(t.visualLines[oldStart:], tmp)
	}

	// 行数变化时，尾部视觉条目的逻辑行号需要整体平移
	rowDelta := t.dirtyRowDelta
	if rowDelta != 0 {
		for i := oldStart + len(tmp); i < newLen; i++ {
			t.visualLines[i].logicalRow += rowDelta
		}
	}

	// 更新行起点索引：块内逐行重填，尾部整体平移（行数或视觉数变化时）
	oldRows := n - rowDelta
	need := n + 1
	if oldRows < 0 {
		oldRows = 0
	}
	if cap(t.logicalToVisual) < need {
		grown := make([]int, need)
		copy(grown, t.logicalToVisual)
		t.logicalToVisual = grown
	} else {
		t.logicalToVisual = t.logicalToVisual[:need]
	}
	sl := t.logicalToVisual

	oldCount := rows - rowDelta
	if oldCount < 0 {
		oldCount = 0
	}
	tailCount := oldRows + 1 - (start + oldCount)
	if tailCount < 0 {
		tailCount = 0
	}
	if rowDelta != 0 && tailCount > 0 {
		copy(sl[start+rows:start+rows+tailCount], sl[start+oldCount:start+oldCount+tailCount])
	}

	idx := oldStart
	pos := 0
	for r := start; r < start+rows; r++ {
		sl[r] = idx
		first := pos
		for pos < len(tmp) && tmp[pos].logicalRow == r {
			pos++
		}
		idx += pos - first
	}
	if visualDelta != 0 {
		for r := start + rows; r < need; r++ {
			sl[r] += visualDelta
		}
	}
	sl[n] = len(t.visualLines)

	t.visualState = visualClean
}

// recomputeVisualLines 按需更新视觉折行模型：脏区域重算，普通按键不再重建整篇索引
func (t *TextArea) recomputeVisualLines() {
	t.ensureTextInitialized()
	availWidth := t.getAvailWrapWidth()
	if t.wrappedWidth != availWidth {
		t.wrappedWidth = availWidth
		t.invalidateAllSegments()
	}
	if len(t.layouts) != t.lineCount() {
		t.layouts = make([]lineLayout, t.lineCount())
		t.visualState = visualFull
		t.maxContentWidth = 0
		t.maxWidthRow = 0
		t.maxWidthDirty = false
	}

	switch t.visualState {
	case visualClean:
		return
	case visualFull:
		t.rebuildVisualIndexAll(availWidth)
	default:
		t.rebuildVisualIndexRange(availWidth)
	}
}

// getVisualIndexForCursor 获取当前光标所在的视觉行索引
func (t *TextArea) getVisualIndexForCursor() int {
	t.recomputeVisualLines()
	if len(t.visualLines) == 0 {
		return 0
	}
	if t.cursorRow < 0 {
		return 0
	}
	if t.cursorRow >= len(t.logicalToVisual) {
		return len(t.visualLines) - 1
	}

	firstV := t.logicalToVisual[t.cursorRow]
	for i := firstV; i < len(t.visualLines); i++ {
		v := t.visualLines[i]
		if v.logicalRow != t.cursorRow {
			break
		}
		if v.isLastSub {
			if t.cursorCol >= v.startCol && t.cursorCol <= v.endCol {
				return i
			}
		} else {
			if t.cursorCol >= v.startCol && t.cursorCol < v.endCol {
				return i
			}
		}
	}
	for i := firstV; i < len(t.visualLines); i++ {
		v := t.visualLines[i]
		if v.logicalRow != t.cursorRow {
			return i - 1
		}
		if v.isLastSub {
			return i
		}
	}
	return firstV
}

// updatePreferredX 更新光标所在位置的物理 X 坐标 (供 Up/Down 跨行对齐记忆)
func (t *TextArea) updatePreferredX() {
	t.recomputeVisualLines()
	vIdx := t.getVisualIndexForCursor()
	if vIdx >= 0 && vIdx < len(t.visualLines) {
		v := t.visualLines[vIdx]
		t.preferredX = t.measureStyledVisualX(v.logicalRow, v.startCol, t.cursorCol)
	}
}

// ensureCursorVisible 保证光标在编辑区域可视范围内
func (t *TextArea) ensureCursorVisible() {
	t.recomputeVisualLines()
	vIdx := t.getVisualIndexForCursor()
	if vIdx < 0 || vIdx >= len(t.visualLines) {
		return
	}
	v := t.visualLines[vIdx]

	gutterW := t.getGutterWidth()
	viewW := t.bounds.Width - gutterW - t.margin*2 - 12
	viewH := t.bounds.Height - t.margin*2
	if viewW <= 0 || viewH <= 0 {
		return
	}

	// 垂直方向对齐：依据视觉行索引 vIdx
	cursorY := int32(vIdx) * t.lineHeight
	if cursorY < t.scrollOffsetY {
		t.scrollOffsetY = cursorY
	} else if cursorY+t.lineHeight > t.scrollOffsetY+viewH {
		t.scrollOffsetY = cursorY + t.lineHeight - viewH
	}

	// 水平方向对齐
	if t.wordWrap {
		t.scrollOffsetX = 0
	} else {
		cursorX := t.measureStyledVisualX(t.cursorRow, v.startCol, t.cursorCol)
		if cursorX < t.scrollOffsetX {
			t.scrollOffsetX = cursorX
		} else if cursorX+t.charWidth > t.scrollOffsetX+viewW {
			t.scrollOffsetX = cursorX + t.charWidth - viewW
		}
	}

	t.clampScroll()
}

// refreshMaxContentWidth 在最长行被编辑/删除后惰性重算水平滚动上界。
// 所有行都按当前字体测量过时才能得到精确值；否则保留现有上界，等下一次全量重建。
func (t *TextArea) refreshMaxContentWidth() {
	if !t.maxWidthDirty {
		return
	}
	for i := range t.layouts {
		if !t.layouts[i].widthOK {
			return
		}
	}
	maxW := int32(0)
	maxRow := 0
	for i := range t.layouts {
		if w := t.layouts[i].totalWidth; w > maxW {
			maxW = w
			maxRow = i
		}
	}
	t.maxContentWidth = maxW
	t.maxWidthRow = maxRow
	t.maxWidthDirty = false
}

// clampScroll 限制滚动偏移范围
func (t *TextArea) clampScroll() {
	t.recomputeVisualLines()
	maxScrollY := int32(len(t.visualLines))*t.lineHeight - (t.bounds.Height - t.margin*2)
	if maxScrollY < 0 {
		maxScrollY = 0
	}
	if t.scrollOffsetY > maxScrollY {
		t.scrollOffsetY = maxScrollY
	}
	if t.scrollOffsetY < 0 {
		t.scrollOffsetY = 0
	}

	if t.wordWrap {
		t.scrollOffsetX = 0
		return
	}
	// 仅非折行模式需要水平上界：惰性重算放在早退之后，避免折行模式的按键/回车
	// 承担最长行失效检查的整表扫描。
	t.refreshMaxContentWidth()
	viewW := t.bounds.Width - t.getGutterWidth() - t.margin*2 - 12
	maxX := t.maxContentWidth - viewW
	if maxX < 0 {
		maxX = 0
	}
	if t.scrollOffsetX > maxX {
		t.scrollOffsetX = maxX
	}
	if t.scrollOffsetX < 0 {
		t.scrollOffsetX = 0
	}
}

// verticalScrollbarRect 返回垂直滚动条滑块矩形；内容不足一屏时 ok=false。
func (t *TextArea) verticalScrollbarRect() (graphics.Rect, bool) {
	b := t.bounds
	contentH := int32(len(t.visualLines)) * t.lineHeight
	maxScrollY := contentH - (b.Height - t.margin*2)
	if b.Width <= 0 || b.Height <= 0 || maxScrollY <= 0 {
		return graphics.Rect{}, false
	}
	barW := int32(6)
	barX := b.X + b.Width - barW - 2
	barH := b.Height
	thumbH := int32(float64(barH) * float64(b.Height) / float64(contentH))
	if thumbH < 20 {
		thumbH = 20
	}
	thumbY := b.Y + int32(float64(barH-thumbH)*float64(t.scrollOffsetY)/float64(maxScrollY))
	return graphics.NewRect(barX, thumbY, barW, thumbH), true
}

// scrollbarPress 处理滚动条按下：命中滑块开始拖拽，点击轨道空白翻一屏。
func (t *TextArea) scrollbarPress(x, y int32) bool {
	thumb, ok := t.verticalScrollbarRect()
	if !ok {
		return false
	}
	if x < thumb.X-2 || x >= thumb.X+thumb.Width+4 {
		return false
	}
	if y >= thumb.Y && y < thumb.Y+thumb.Height {
		t.draggingScrollbar = true
		t.dragStartY = y
		t.dragStartScrollY = t.scrollOffsetY
		return true
	}
	page := t.bounds.Height - t.margin*2
	if y < thumb.Y {
		t.scrollOffsetY -= page
	} else {
		t.scrollOffsetY += page
	}
	t.clampScroll()
	return true
}

// dragScrollbar 按鼠标拖动距离映射滚动偏移。
func (t *TextArea) dragScrollbar(y int32) {
	thumb, ok := t.verticalScrollbarRect()
	if !ok {
		return
	}
	usable := t.bounds.Height - thumb.Height
	maxScrollY := int32(len(t.visualLines))*t.lineHeight - (t.bounds.Height - t.margin*2)
	if usable <= 0 || maxScrollY <= 0 {
		return
	}
	dy := y - t.dragStartY
	t.scrollOffsetY = t.dragStartScrollY + int32(float64(dy)*float64(maxScrollY)/float64(usable))
	t.clampScroll()
}

// notifyChange 触发文本变更
func (t *TextArea) notifyChange() {
	if t.onChange != nil {
		t.onChange()
	}
}

// notifyCursor 触发光标变动通知
func (t *TextArea) notifyCursor() {
	t.caretOn = true
	if t.onCursorChange != nil {
		t.onCursorChange(t.cursorRow+1, t.cursorCol+1)
	}
}

// OnEvent 处理各类输入与交互事件
func (t *TextArea) OnEvent(evt event.Event) bool {
	if !t.visible || !t.enabled {
		return false
	}

	switch evt.Type {
	case event.EventMouseDown:
		data, ok := evt.Data.(event.MouseData)
		if !ok || data.Button != event.MouseButtonLeft {
			return false
		}
		return t.handlePress(data, false)

	case event.EventMouseDoubleClick:
		data, ok := evt.Data.(event.MouseData)
		if !ok {
			return false
		}
		return t.handlePress(data, true)

	case event.EventImeStart:
		if !t.focused || t.readOnly {
			return false
		}
		if t.preedit == "" {
			return false
		}
		t.preedit = ""
		return true

	case event.EventImeUpdate:
		if !t.focused || t.readOnly {
			return false
		}
		data, ok := evt.Data.(event.ImeData)
		if !ok {
			return false
		}
		if t.preedit == data.Text {
			return false
		}
		t.preedit = data.Text
		t.ensureCursorVisible()
		return true

	case event.EventImeCommit:
		if !t.focused || t.readOnly {
			return false
		}
		hadPreedit := t.preedit != ""
		t.preedit = ""
		data, ok := evt.Data.(event.ImeData)
		if !ok || data.Text == "" {
			return hadPreedit
		}
		t.insertText(data.Text)
		return true

	case event.EventImeEnd:
		if t.preedit == "" {
			return false
		}
		t.preedit = ""
		return true

	case event.EventMouseMove:
		data, ok := evt.Data.(event.MouseData)
		if !ok {
			return false
		}
		if t.draggingScrollbar {
			t.dragScrollbar(data.Y)
			return true
		}
		if t.isSelecting {
			t.recomputeVisualLines()
			gutterW := t.getGutterWidth()
			textStartX := t.bounds.X + gutterW + t.margin
			relY := data.Y - (t.bounds.Y + t.margin) + t.scrollOffsetY
			vIdx := int(relY / t.lineHeight)
			if vIdx < 0 {
				vIdx = 0
			}
			if vIdx >= len(t.visualLines) {
				vIdx = len(t.visualLines) - 1
			}

			v := t.visualLines[vIdx]
			relX := data.X - textStartX
			if !t.wordWrap {
				relX += t.scrollOffsetX
			}

			curRow := v.logicalRow
			curCol := t.getStyledVisualColFromX(v.logicalRow, v.startCol, v.endCol, relX)

			t.cursorRow = curRow
			t.cursorCol = curCol
			t.preferredCol = curCol
			t.updatePreferredX()
			t.selCursorRow = curRow
			t.selCursorCol = curCol
			t.selectionActive = !(t.selAnchorRow == curRow && t.selAnchorCol == curCol)

			t.ensureCursorVisible()
			t.notifyCursor()
			return true
		}

	case event.EventMouseUp:
		if t.draggingScrollbar {
			t.draggingScrollbar = false
			return true
		}
		if t.isSelecting {
			t.isSelecting = false
			return true
		}

	case event.EventMouseWheel:
		data, ok := evt.Data.(event.MouseData)
		if !ok {
			return false
		}
		if IsInBounds(data.X, data.Y, t.bounds) {
			if data.Ctrl {
				// Ctrl + 滚轮：按增量比例缩放（一个标准刻度 = 10%），触摸板小增量累加后再步进，
				// 避免每个微小事件都重建整篇排版。
				steps := consumeWheelDelta(&t.zoomRemainder, int64(data.Delta), 1)
				if steps == 0 {
					return true
				}
				t.SetZoom(t.zoomFactor + 0.1*float64(steps))
				return true
			}
			if data.Shift && !t.wordWrap {
				// Shift + 滚轮：关闭折行时横向滚动（每刻度 4 个字符宽）
				pixels := consumeWheelDelta(&t.wheelRemainderX, int64(data.Delta), int64(t.charWidth)*4)
				if pixels == 0 {
					return true
				}
				t.scrollOffsetX -= pixels
				t.clampScroll()
				return true
			}
			// 滚轮滚动：每刻度 3 行，分数增量按像素累加，低精度设备不再完全不响应。
			pixels := consumeWheelDelta(&t.wheelRemainderY, int64(data.Delta), int64(t.lineHeight)*3)
			if pixels == 0 {
				return true
			}
			t.scrollOffsetY -= pixels
			t.clampScroll()
			return true
		}

	case event.EventKeyDown:
		if !t.focused {
			return false
		}
		data, ok := evt.Data.(event.KeyData)
		if !ok {
			return false
		}
		return t.handleKeyDown(data)

	case event.EventKeyPress:
		if !t.focused || t.readOnly {
			return false
		}
		data, ok := evt.Data.(event.KeyData)
		if !ok {
			return false
		}
		return t.handleKeyPress(data)

	case event.EventFocus:
		t.focused = true
		return true

	case event.EventBlur:
		t.focused = false
		t.isSelecting = false
		t.draggingScrollbar = false
		if t.preedit != "" {
			t.preedit = ""
		}
		return true
	}

	return false
}

// isWordRune 判断字符是否属于"单词"（字母/数字/下划线，含中日韩文字）
func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// handlePress 处理一次鼠标按下（单击定位 / 双击选词 / 三击选行）
func (t *TextArea) handlePress(data event.MouseData, isDouble bool) bool {
	if !IsInBounds(data.X, data.Y, t.bounds) {
		// 点击编辑区之外不应清除焦点：工具栏按钮点击后必须仍可继续键入与使用 Ctrl+Z
		return false
	}
	RequestFocus(t)
	t.recomputeVisualLines()
	if !isDouble && t.scrollbarPress(data.X, data.Y) {
		return true
	}

	gutterW := t.getGutterWidth()
	textStartX := t.bounds.X + gutterW + t.margin
	relY := data.Y - (t.bounds.Y + t.margin) + t.scrollOffsetY
	vIdx := int(relY / t.lineHeight)
	if vIdx < 0 {
		vIdx = 0
	}
	if vIdx >= len(t.visualLines) {
		vIdx = len(t.visualLines) - 1
	}
	v := t.visualLines[vIdx]
	relX := data.X - textStartX
	if !t.wordWrap {
		relX += t.scrollOffsetX
	}
	row := v.logicalRow
	col := t.getStyledVisualColFromX(row, v.startCol, v.endCol, relX)

	now := time.Now()
	sameCell := now.Sub(t.lastClickAt) <= 450*time.Millisecond && row == t.lastClickRow && col == t.lastClickCol
	switch {
	case data.Shift:
		// Shift+点击只做选区扩展，不参与双击/三击计数：
		// 否则松开 Shift 后的下一次单击会被误判为双击而选中整词
		t.clickCount = 1
	case isDouble:
		if sameCell && t.clickCount >= 2 {
			t.clickCount++
		} else {
			t.clickCount = 2
		}
	case sameCell:
		t.clickCount++
	default:
		t.clickCount = 1
	}
	t.lastClickAt, t.lastClickRow, t.lastClickCol = now, row, col

	switch {
	case data.Shift:
		// Shift+点击：保留原锚点，仅移动光标扩展选区；
		// 双击/三击在按住 Shift 时同样只扩展选区，不重建词/行选区
		if !t.selectionActive {
			t.selAnchorRow, t.selAnchorCol = t.cursorRow, t.cursorCol
		}
		t.cursorRow, t.cursorCol = row, col
		t.selCursorRow, t.selCursorCol = row, col
		t.selectionActive = !(t.selAnchorRow == row && t.selAnchorCol == col)
		t.preferredCol = col
		t.updatePreferredX()
		t.isSelecting = true
	case t.clickCount >= 3:
		t.selectLineAt(row)
	case t.clickCount == 2:
		t.selectWordAt(row, col)
	default:
		t.cursorRow, t.cursorCol = row, col
		t.selAnchorRow, t.selAnchorCol = row, col
		t.selCursorRow, t.selCursorCol = row, col
		t.selectionActive = false
		t.preferredCol = col
		t.updatePreferredX()
		t.isSelecting = true
	}

	t.preedit = ""
	t.ensureCursorVisible()
	t.notifyCursor()
	return true
}

// selectWordAt 选中 (row, col) 处的单词 / 空白段 / 单个标点
func (t *TextArea) selectWordAt(row, col int) {
	line := t.line(row)
	if len(line) == 0 {
		t.selAnchorRow, t.selAnchorCol = row, 0
		t.cursorRow, t.cursorCol = row, 0
		t.selectionActive = false
		return
	}
	if col >= len(line) {
		col = len(line) - 1
	}
	start, end := col, col+1
	if isWordRune(line[col]) {
		for start > 0 && isWordRune(line[start-1]) {
			start--
		}
		for end < len(line) && isWordRune(line[end]) {
			end++
		}
	} else if line[col] == ' ' || line[col] == '\t' {
		for start > 0 && line[start-1] == line[col] {
			start--
		}
		for end < len(line) && line[end] == line[col] {
			end++
		}
	}
	t.selAnchorRow, t.selAnchorCol = row, start
	t.cursorRow, t.cursorCol = row, end
	t.selCursorRow, t.selCursorCol = row, end
	t.selectionActive = end > start
	t.isSelecting = true
	t.preferredCol = end
	t.updatePreferredX()
}

// selectLineAt 选中整行（三击）
func (t *TextArea) selectLineAt(row int) {
	t.selAnchorRow, t.selAnchorCol = row, 0
	t.cursorRow, t.cursorCol = row, len(t.line(row))
	t.selCursorRow, t.selCursorCol = t.cursorRow, t.cursorCol
	t.selectionActive = t.cursorCol > 0
	t.isSelecting = true
	t.preferredCol = t.cursorCol
	t.updatePreferredX()
}

// handleKeyDown 处理快捷键与功能导航键
func (t *TextArea) handleKeyDown(data event.KeyData) bool {
	// Ctrl 组合快捷键
	if data.Ctrl {
		switch data.KeyCode {
		case 'A', 'a': // 全选
			t.SelectAll()
			return true

		case 'C', 'c': // 复制
			return t.Copy()

		case 'X', 'x': // 剪切
			return t.Cut()

		case 'V', 'v': // 粘贴
			return t.Paste()

		case 'Z', 'z': // 撤销 / Ctrl+Shift+Z 重做
			if !t.readOnly {
				if data.Shift {
					return t.Redo()
				}
				return t.Undo()
			}

		case 'Y', 'y': // 重做
			if !t.readOnly {
				return t.Redo()
			}

		case 187, 107, '=', '+': // 放大字体 (Ctrl + '+' / '=')
			t.ZoomIn()
			return true

		case 189, 109, '-': // 缩小字体 (Ctrl + '-')
			t.ZoomOut()
			return true

		case 0x60, '0': // 重置字体缩放 (Ctrl + '0' 或小键盘 0)
			t.ResetZoom()
			return true
		}
	}

	// 选区与 Shift 键移动结合
	updateSelection := func(oldRow, oldCol int) {
		if data.Shift {
			if !t.selectionActive {
				t.selAnchorRow = oldRow
				t.selAnchorCol = oldCol
				t.selectionActive = true
			}
			t.selCursorRow = t.cursorRow
			t.selCursorCol = t.cursorCol
		} else {
			t.selectionActive = false
		}
	}

	oldRow, oldCol := t.cursorRow, t.cursorCol

	switch data.KeyCode {
	case 0x08: // Backspace
		if t.readOnly {
			return false
		}
		if t.selectionActive {
			t.replaceSelectionWithRunes(nil)
			return true
		}
		if data.Ctrl {
			return t.deleteWordBackward()
		}
		if t.cursorCol > 0 {
			t.applyEdit(t.cursorRow, t.cursorCol-1, t.cursorRow, t.cursorCol, nil)
			return true
		}
		if t.cursorRow > 0 {
			// 删除换行符，将当前行拼入上一行
			t.applyEdit(t.cursorRow-1, len(t.line(t.cursorRow-1)), t.cursorRow, 0, nil)
			return true
		}

	case 0x2E: // Delete
		if t.readOnly {
			return false
		}
		if t.selectionActive {
			t.replaceSelectionWithRunes(nil)
			return true
		}
		if data.Ctrl {
			return t.deleteWordForward()
		}
		if t.cursorCol < len(t.line(t.cursorRow)) {
			t.applyEdit(t.cursorRow, t.cursorCol, t.cursorRow, t.cursorCol+1, nil)
			return true
		}
		if t.cursorRow < t.lineCount()-1 {
			// 删除换行符，将下一行合并入当前行
			t.applyEdit(t.cursorRow, t.cursorCol, t.cursorRow+1, 0, nil)
			return true
		}

	case 0x0D: // Enter (换行 + 智能缩进)
		if t.readOnly {
			return false
		}
		t.replaceSelectionWithRunes(t.enterInsertion())
		return true

	case 0x09: // Tab 缩进 / Shift+Tab 反缩进
		if t.readOnly {
			return false
		}
		if data.Shift {
			return t.outdentSelection()
		}
		if t.indentSelection() {
			return true
		}
		t.replaceSelectionWithRunes([]rune("    "))
		return true

	case 0x25: // Left Arrow
		if data.Ctrl {
			t.moveWord(false)
		} else if t.cursorCol > 0 {
			t.cursorCol--
		} else if t.cursorRow > 0 {
			t.cursorRow--
			t.cursorCol = len(t.line(t.cursorRow))
		}
		t.preferredCol = t.cursorCol
		t.updatePreferredX()
		updateSelection(oldRow, oldCol)
		t.ensureCursorVisible()
		t.notifyCursor()
		return true

	case 0x27: // Right Arrow
		if data.Ctrl {
			t.moveWord(true)
		} else if t.cursorCol < len(t.line(t.cursorRow)) {
			t.cursorCol++
		} else if t.cursorRow < t.lineCount()-1 {
			t.cursorRow++
			t.cursorCol = 0
		}
		t.preferredCol = t.cursorCol
		t.updatePreferredX()
		updateSelection(oldRow, oldCol)
		t.ensureCursorVisible()
		t.notifyCursor()
		return true

	case 0x26: // Up Arrow
		t.recomputeVisualLines()
		vIdx := t.getVisualIndexForCursor()
		if vIdx > 0 {
			targetV := t.visualLines[vIdx-1]
			t.cursorRow = targetV.logicalRow
			t.cursorCol = t.getStyledVisualColFromX(targetV.logicalRow, targetV.startCol, targetV.endCol, t.preferredX)
		} else {
			t.cursorRow = 0
			t.cursorCol = 0
			t.preferredCol = 0
			t.preferredX = 0
		}
		updateSelection(oldRow, oldCol)
		t.ensureCursorVisible()
		t.notifyCursor()
		return true

	case 0x28: // Down Arrow
		t.recomputeVisualLines()
		vIdx := t.getVisualIndexForCursor()
		if vIdx < len(t.visualLines)-1 {
			targetV := t.visualLines[vIdx+1]
			t.cursorRow = targetV.logicalRow
			t.cursorCol = t.getStyledVisualColFromX(targetV.logicalRow, targetV.startCol, targetV.endCol, t.preferredX)
		} else {
			lastV := t.visualLines[len(t.visualLines)-1]
			t.cursorRow = lastV.logicalRow
			t.cursorCol = lastV.endCol
			t.preferredCol = t.cursorCol
		}
		updateSelection(oldRow, oldCol)
		t.ensureCursorVisible()
		t.notifyCursor()
		return true

	case 0x24: // Home / Ctrl+Home
		if data.Ctrl {
			t.cursorRow, t.cursorCol = 0, 0
			t.preferredCol = 0
			t.updatePreferredX()
			updateSelection(oldRow, oldCol)
			t.ensureCursorVisible()
			t.notifyCursor()
			return true
		}
		t.recomputeVisualLines()
		vIdx := t.getVisualIndexForCursor()
		v := t.visualLines[vIdx]
		if t.cursorCol != v.startCol {
			t.cursorCol = v.startCol
		} else {
			t.cursorCol = 0
		}
		t.preferredCol = t.cursorCol
		t.updatePreferredX()
		updateSelection(oldRow, oldCol)
		t.ensureCursorVisible()
		t.notifyCursor()
		return true

	case 0x23: // End / Ctrl+End
		if data.Ctrl {
			last := t.lineCount() - 1
			t.cursorRow, t.cursorCol = last, len(t.line(last))
			t.preferredCol = t.cursorCol
			t.updatePreferredX()
			updateSelection(oldRow, oldCol)
			t.ensureCursorVisible()
			t.notifyCursor()
			return true
		}
		t.recomputeVisualLines()
		vIdx := t.getVisualIndexForCursor()
		v := t.visualLines[vIdx]
		if t.cursorCol != v.endCol {
			t.cursorCol = v.endCol
		} else if !v.isLastSub {
			t.cursorCol = len(t.line(v.logicalRow))
		}
		t.preferredCol = t.cursorCol
		t.updatePreferredX()
		updateSelection(oldRow, oldCol)
		t.ensureCursorVisible()
		t.notifyCursor()
		return true

	case 0x21: // PageUp
		t.recomputeVisualLines()
		visibleLines := int(t.bounds.Height / t.lineHeight)
		vIdx := t.getVisualIndexForCursor() - visibleLines
		if vIdx < 0 {
			vIdx = 0
		}
		v := t.visualLines[vIdx]
		t.cursorRow = v.logicalRow
		subLine := t.line(v.logicalRow)[v.startCol:v.endCol]
		t.cursorCol = v.startCol + t.getColFromX(subLine, t.preferredX)
		updateSelection(oldRow, oldCol)
		t.ensureCursorVisible()
		t.notifyCursor()
		return true

	case 0x22: // PageDown
		t.recomputeVisualLines()
		visibleLines := int(t.bounds.Height / t.lineHeight)
		vIdx := t.getVisualIndexForCursor() + visibleLines
		if vIdx >= len(t.visualLines) {
			vIdx = len(t.visualLines) - 1
		}
		v := t.visualLines[vIdx]
		t.cursorRow = v.logicalRow
		subLine := t.line(v.logicalRow)[v.startCol:v.endCol]
		t.cursorCol = v.startCol + t.getColFromX(subLine, t.preferredX)
		updateSelection(oldRow, oldCol)
		t.ensureCursorVisible()
		t.notifyCursor()
		return true
	}

	return false
}

// handleKeyPress 处理字符输入
func (t *TextArea) handleKeyPress(data event.KeyData) bool {
	ch := data.Char
	if ch < 32 || ch == 127 {
		return false
	}
	var one [1]rune
	one[0] = ch
	return t.replaceSelectionWithRunes(one[:])
}

// insertText 向光标处插入字符串（支持多行换行解析），单次操作即可完整撤销
func (t *TextArea) insertText(text string) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if text == "" {
		return
	}
	t.replaceSelectionWithRunes([]rune(text))
}

// drawStyledRange 以流式画笔绘制 line[from:to]，返回整段物理宽度。
// 关闭折行的超长行会借助前缀检查点跳过视口外的左右片段，只光栅化可见列。
func (t *TextArea) drawStyledRange(canvas *render.Canvas, row int, line []rune, spans []syntax.Span, from, to int, penX, lineY int32) int32 {
	if from >= to || canvas == nil {
		return 0
	}
	startX := penX
	drawFrom, drawTo := from, to
	skippedLeft, skippedRight := int32(0), int32(0)

	if p := t.prefixFor(row); p != nil && to-from > prefixStep {
		clip := canvas.Clip()
		total := t.prefixWidthAt(p, row, to) - t.prefixWidthAt(p, row, from)
		if penX < clip.X || penX+total > clip.X+clip.Width {
			drawFrom = t.prefixColAtX(p, row, from, clip.X-penX, from, to)
			drawTo = t.prefixColAtX(p, row, from, clip.X+clip.Width-penX, from, to)
			if drawFrom > from {
				skippedLeft = t.prefixWidthAt(p, row, drawFrom) - t.prefixWidthAt(p, row, from)
				penX += skippedLeft
			}
			if drawTo < to {
				skippedRight = t.prefixWidthAt(p, row, to) - t.prefixWidthAt(p, row, drawTo)
			}
			if drawFrom >= drawTo {
				return skippedLeft + skippedRight
			}
		}
	}

	if len(spans) == 0 {
		return skippedLeft + canvas.DrawRunes(line[drawFrom:drawTo], penX, lineY, t.font, t.textColor) + skippedRight
	}

	currCol := drawFrom
	for _, span := range spans {
		if span.EndCol <= drawFrom || span.StartCol >= drawTo {
			continue
		}
		if span.StartCol > currCol {
			drawEnd := span.StartCol
			if drawEnd > drawTo {
				drawEnd = drawTo
			}
			seg := line[currCol:drawEnd]
			penX += canvas.DrawRunes(seg, penX, lineY, t.font, t.textColor)
			currCol = drawEnd
			if currCol >= drawTo {
				break
			}
		}

		spanStart := span.StartCol
		if spanStart < currCol {
			spanStart = currCol
		}
		spanEnd := span.EndCol
		if spanEnd > drawTo {
			spanEnd = drawTo
		}
		if spanEnd > spanStart {
			seg := line[spanStart:spanEnd]
			spanFont := t.getStyledFont(span.Bold, span.Italic)
			colorToUse := span.Color
			if colorToUse == 0 {
				colorToUse = t.textColor
			}
			var segW int32
			if span.BgColor != 0 {
				// 底色必须先于字形铺到缓冲上：半透明底色一旦在文字之后混合，
				// 就会盖住已光栅化的字形（Markdown 行内代码/代码块），冲淡正文。
				bgW := render.GetFontManager().MeasureRunes(spanFont, seg)
				if bgW > 0 {
					canvas.FillRect(penX, lineY, bgW, t.lineHeight, span.BgColor)
				}
				segW = canvas.DrawRunes(seg, penX, lineY, spanFont, colorToUse)
				if segW <= 0 {
					segW = bgW
				}
			} else {
				// 无底色的 span 绘制同时返回步进宽度，避免再走一遍字形测量。
				segW = canvas.DrawRunes(seg, penX, lineY, spanFont, colorToUse)
			}
			penX += segW
			currCol = spanEnd
			if currCol >= drawTo {
				break
			}
		}
	}

	if currCol < drawTo {
		penX += canvas.DrawRunes(line[currCol:drawTo], penX, lineY, t.font, t.textColor)
	}
	return skippedLeft + (penX - startX - skippedLeft) + skippedRight
}

// measurePreeditWidth 预编辑串的物理宽度
func (t *TextArea) measurePreeditWidth() int32 {
	if t.preedit == "" {
		return 0
	}
	return render.GetFontManager().MeasureRunesFace(t.fontFace, []rune(t.preedit))
}

// drawPreedit 在光标处绘制输入法预编辑串（带背景与下划线）
func (t *TextArea) drawPreedit(canvas *render.Canvas, x, lineY int32) int32 {
	if t.preedit == "" {
		return 0
	}
	w := t.measurePreeditWidth()
	if w <= 0 {
		return 0
	}
	canvas.FillRect(x, lineY, w, t.lineHeight, t.selectionColor)
	canvas.DrawText(t.preedit, x, lineY, t.font, t.textColor)
	canvas.DrawHorizontalLine(x, x+w, lineY+t.lineHeight-3, t.caretColor)
	canvas.DrawHorizontalLine(x, x+w, lineY+t.lineHeight-2, t.caretColor)
	return w
}

// ImeAnchor 返回输入法组合窗口应贴靠的光标客户区坐标与行高
func (t *TextArea) ImeAnchor() (x, y, lineHeight int32, ok bool) {
	if !t.focused || t.readOnly || !t.visible || t.bounds.Width <= 0 || t.bounds.Height <= 0 {
		return 0, 0, 0, false
	}
	t.recomputeVisualLines()
	vIdx := t.getVisualIndexForCursor()
	if vIdx < 0 || vIdx >= len(t.visualLines) {
		return 0, 0, 0, false
	}
	v := t.visualLines[vIdx]
	scrollX := t.scrollOffsetX
	if t.wordWrap {
		scrollX = 0
	}
	gutterW := t.getGutterWidth()
	x = t.bounds.X + gutterW + t.margin + t.measureStyledVisualX(v.logicalRow, v.startCol, t.cursorCol) - scrollX
	x += t.measurePreeditWidth()
	y = t.bounds.Y + t.margin + int32(vIdx)*t.lineHeight - t.scrollOffsetY
	return x, y, t.lineHeight, true
}

// preeditOnVisualLine 判断输入法预编辑串是否应绘制在该视觉子行上。
// 折行点归属下一子行，避免光标正好在折行点时组合串在上下两行重复绘制。
func (t *TextArea) preeditOnVisualLine(v visualLine) bool {
	if t.preedit == "" || !t.focused || v.logicalRow != t.cursorRow {
		return false
	}
	if t.cursorCol < v.startCol {
		return false
	}
	if v.isLastSub {
		return t.cursorCol <= v.endCol
	}
	return t.cursorCol < v.endCol
}

// Render 渲染多行文本编辑器（行级虚拟化光栅化，支持自动换行与单行水平延续）
func (t *TextArea) Render(canvas *render.Canvas) {
	if !t.visible || canvas == nil {
		return
	}

	b := t.bounds
	if b.Width <= 0 || b.Height <= 0 {
		return
	}

	// 确保视觉折行模型是最新的
	t.recomputeVisualLines()

	// 1. 填充主背景
	canvas.FillRect(b.X, b.Y, b.Width, b.Height, t.background)

	gutterW := t.getGutterWidth()
	textStartX := b.X + gutterW + t.margin

	// 2. 如果开启行号槽，绘制行号背景与分隔线
	if t.showLineNumbers && gutterW > 0 {
		canvas.FillRect(b.X, b.Y, gutterW, b.Height, t.gutterBg)
		canvas.DrawVerticalLine(b.X+gutterW, b.Y, b.Y+b.Height, t.gutterBorder)
	}

	totalVisual := len(t.visualLines)
	if totalVisual == 0 {
		return
	}

	// 3. 计算视口内的视觉行虚拟化范围 [startVisual, endVisual]
	startVisual := int(t.scrollOffsetY / t.lineHeight)
	if startVisual < 0 {
		startVisual = 0
	}
	visibleCount := int((b.Height+t.lineHeight-1)/t.lineHeight) + 1
	endVisual := startVisual + visibleCount
	if endVisual >= totalVisual {
		endVisual = totalVisual - 1
	}

	activeVisualIdx := t.getVisualIndexForCursor()

	// 4. 当前光标行高亮（仅当开启高亮且聚焦无多行选区时）
	if t.highlightActiveLine && t.focused && !t.selectionActive && activeVisualIdx >= startVisual && activeVisualIdx <= endVisual {
		activeY := b.Y + t.margin + int32(activeVisualIdx)*t.lineHeight - t.scrollOffsetY
		if t.activeLineBg != 0 {
			canvas.FillRect(b.X+gutterW+1, activeY, b.Width-gutterW-1, t.lineHeight, t.activeLineBg)
		}
		if t.activeLineBorder != 0 {
			canvas.DrawHorizontalLine(b.X+gutterW+1, b.X+b.Width-1, activeY, t.activeLineBorder)
			canvas.DrawHorizontalLine(b.X+gutterW+1, b.X+b.Width-1, activeY+t.lineHeight-1, t.activeLineBorder)
		}
	}

	// 获取规范化选区
	sRow, sCol, eRow, eCol, hasSel := t.getNormalizedSelection()

	// 5. 绘制视口内的文本（使用裁剪栈严格限制在编辑区内）
	textClip := graphics.NewRect(b.X+gutterW+1, b.Y, b.Width-gutterW-1, b.Height)
	canvas.PushClip(textClip)

	// 更新高亮行状态至当前可视最大逻辑行
	if t.highlighter != nil && endVisual >= 0 {
		maxLogical := t.visualLines[endVisual].logicalRow
		t.updateLineStatesUpTo(maxLogical)
	}

	scrollX := t.scrollOffsetX
	if t.wordWrap {
		scrollX = 0
	}

	for vIdx := startVisual; vIdx <= endVisual; vIdx++ {
		v := t.visualLines[vIdx]
		lineY := b.Y + t.margin + int32(vIdx)*t.lineHeight - t.scrollOffsetY
		r := v.logicalRow
		line := t.line(r)

		if !t.wordWrap {
			// 关闭折行时宽度按需测量：只为当前可见行补算，保证水平滚动上界可用
			t.ensureRowWidth(r)
		}

		// 语法高亮分词（按行缓存，滚动/重绘命中缓存不再重复分词）
		cachedSpans := t.getSpansForLine(r)

		// 选区高亮绘制 (精确匹配样式字体物理宽度)
		if hasSel && r >= sRow && r <= eRow {
			rowSelStart := 0
			if r == sRow {
				rowSelStart = sCol
			}
			rowSelEnd := len(line)
			if r == eRow {
				rowSelEnd = eCol
			}

			selStart := v.startCol
			if rowSelStart > selStart {
				selStart = rowSelStart
			}
			selEnd := v.endCol
			if rowSelEnd < selEnd {
				selEnd = rowSelEnd
			}

			if selStart < selEnd {
				// 一次测量到 selStart，再追加 [selStart, selEnd) 段宽度，避免重复扫描前缀
				selX := t.measureStyledVisualX(r, v.startCol, selStart)
				x1 := textStartX + selX - scrollX
				x2 := x1 + t.measureStyledVisualX(r, selStart, selEnd)
				selW := x2 - x1
				if selEnd == v.endCol && v.isLastSub && r < eRow {
					selW += t.charWidth
				}
				if selW > 0 {
					canvas.FillRect(x1, lineY, selW, t.lineHeight, t.selectionColor)
				}
			} else if selStart == selEnd && v.isLastSub && r < eRow && selStart == len(line) {
				x1 := textStartX + t.measureStyledVisualX(r, v.startCol, v.endCol) - scrollX
				canvas.FillRect(x1, lineY, t.charWidth, t.lineHeight, t.selectionColor)
			}
		}

		// 绘制该视觉子行文本 (流式物理画笔累进，彻底根除字形加粗膨胀带来的重叠错位)
		subLine := line[v.startCol:v.endCol]
		preeditHere := t.preeditOnVisualLine(v)
		if len(subLine) == 0 {
			if len(cachedSpans) > 0 && cachedSpans[0].BgColor != 0 {
				canvas.FillRect(textStartX-scrollX, lineY, b.Width, t.lineHeight, cachedSpans[0].BgColor)
			}
			if preeditHere {
				t.drawPreedit(canvas, textStartX-scrollX, lineY)
			}
		} else if preeditHere {
			// 预编辑串插在光标处：先画光标前文本，再画预编辑串，最后把后半行整体右移
			penX := textStartX - scrollX
			penX += t.drawStyledRange(canvas, v.logicalRow, line, cachedSpans, v.startCol, t.cursorCol, penX, lineY)
			penX += t.drawPreedit(canvas, penX, lineY)
			t.drawStyledRange(canvas, v.logicalRow, line, cachedSpans, t.cursorCol, v.endCol, penX, lineY)
		} else {
			t.drawStyledRange(canvas, v.logicalRow, line, cachedSpans, v.startCol, v.endCol, textStartX-scrollX, lineY)
		}
	}

	// 6. 绘制光标（聚焦且非只读，500ms 周期呼吸闪烁，依据字符样式物理像素精准对齐）
	if t.focused && !t.readOnly {
		if t.caretOn {
			if activeVisualIdx >= startVisual && activeVisualIdx <= endVisual {
				activeV := t.visualLines[activeVisualIdx]
				activeY := b.Y + t.margin + int32(activeVisualIdx)*t.lineHeight - t.scrollOffsetY
				var caretWidth int32
				if t.caretXValid && t.caretXRow == activeV.logicalRow && t.caretXCol == t.cursorCol && t.caretXEpoch == t.measureEpoch {
					caretWidth = t.caretXWidth
				} else {
					caretWidth = t.measureStyledVisualX(activeV.logicalRow, activeV.startCol, t.cursorCol)
					t.caretXValid = true
					t.caretXRow = activeV.logicalRow
					t.caretXCol = t.cursorCol
					t.caretXEpoch = t.measureEpoch
					t.caretXWidth = caretWidth
				}
				cursorX := textStartX + caretWidth - scrollX
				if t.preedit != "" {
					cursorX += t.measurePreeditWidth()
				}
				canvas.DrawVerticalLine(cursorX, activeY, activeY+t.lineHeight, t.caretColor)
				canvas.DrawVerticalLine(cursorX+1, activeY, activeY+t.lineHeight, t.caretColor) // 双像素光标更清晰
			}
		}
	}

	canvas.PopClip()

	// 7. 绘制行号槽上的行号数字 (仅对每个逻辑行的首个视觉行 isFirstSub 绘制)
	if t.showLineNumbers && gutterW > 0 {
		gutterClip := graphics.NewRect(b.X, b.Y, gutterW, b.Height)
		canvas.PushClip(gutterClip)
		var numBuf [20]byte
		for vIdx := startVisual; vIdx <= endVisual; vIdx++ {
			v := t.visualLines[vIdx]
			if !v.isFirstSub {
				continue
			}
			lineY := b.Y + t.margin + int32(vIdx)*t.lineHeight - t.scrollOffsetY
			digits := strconv.AppendInt(numBuf[:0], int64(v.logicalRow+1), 10)
			t.gutterRunes = t.gutterRunes[:0]
			for _, c := range digits {
				t.gutterRunes = append(t.gutterRunes, rune(c))
			}
			numW := int32(len(digits)) * t.digitWidth
			numX := b.X + gutterW - numW - 8 // 右对齐
			canvas.DrawRunes(t.gutterRunes, numX, lineY, t.font, t.gutterText)
		}
		canvas.PopClip()
	}

	// 8. 绘制垂直滚动条滑块（可拖拽，命中区域见 scrollbarPress）
	if thumb, ok := t.verticalScrollbarRect(); ok {
		canvas.FillRect(thumb.X, thumb.Y, thumb.Width, thumb.Height, t.scrollbarThumb)
	}
}
