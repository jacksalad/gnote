package component

import (
	"time"

	"github.com/jacksalad/gnote/clipboard"
	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/theme"
)

// TextBox 文本输入框组件（支持中文、Unicode、剪贴板与光标闪烁）
type TextBox struct {
	BaseComponent
	text             string
	placeholder      string
	font             *Font
	textColor        graphics.Color
	placeholderColor graphics.Color
	background       graphics.Color
	borderColor      graphics.Color
	focusBorderColor graphics.Color
	focused          bool
	cursorPos        int   // 以 rune 字符为单位的索引
	scrollOffsetX    int32 // 长文本水平滚动偏移，保证光标可见
	readOnly         bool
	maxLength        int
	onChange         func(text string)
	onSubmit         func(text string)
	preedit          string // 输入法预编辑串（不进入文本模型）
}

// NewTextBox 创建文本框
func NewTextBox() *TextBox {
	t := &TextBox{
		BaseComponent: NewBaseComponent(),
		font:          DefaultFont,
		maxLength:     512,
	}
	t.ApplyTheme(theme.Current())
	return t
}

// ApplyTheme 应用主题样式
func (t *TextBox) ApplyTheme(th *theme.Theme) {
	if th == nil {
		return
	}
	t.background = th.Surface
	t.textColor = th.TextPrimary
	t.placeholderColor = th.TextSecondary
	t.borderColor = th.Border
	t.focusBorderColor = th.BorderFocused
}

// GetCursor 获取鼠标悬停光标（工字光标）
func (t *TextBox) GetCursor() int {
	return 32513 // IDC_IBEAM
}

// Render 渲染文本框
func (t *TextBox) Render(canvas *render.Canvas) {
	if !t.visible {
		return
	}

	bounds := t.bounds

	// 绘制背景
	canvas.FillRect(bounds.X, bounds.Y, bounds.Width, bounds.Height, t.background)

	// 绘制边框（聚焦时显示高亮色）
	borderColor := t.borderColor
	if t.focused {
		borderColor = t.focusBorderColor
	}
	canvas.DrawRect(bounds.X, bounds.Y, bounds.Width, bounds.Height, borderColor)

	// 文本内容或占位符
	displayText := t.text
	textColor := t.textColor
	if displayText == "" && t.preedit == "" {
		displayText = t.placeholder
		textColor = t.placeholderColor
	}

	// 绘制文本（限制在输入框内，长文本随光标水平滚动）
	paddingX := int32(6)
	textX := bounds.X + paddingX - t.scrollOffsetX
	clip := graphics.NewRect(bounds.X+2, bounds.Y+2, bounds.Width-4, bounds.Height-4)
	canvas.PushClip(clip)

	fm := render.GetFontManager()
	_, textHeight := canvas.MeasureText("Ag", t.font)
	textY := bounds.Y + (bounds.Height-textHeight)/2

	runes := []rune(t.text)
	cursorPos := t.cursorPos
	if cursorPos > len(runes) {
		cursorPos = len(runes)
	}
	if cursorPos < 0 {
		cursorPos = 0
	}

	if t.focused && t.preedit != "" {
		// 光标处插入预编辑串，后半段整体右移
		prefix := runes[:cursorPos]
		tail := runes[cursorPos:]
		prefixW := fm.MeasureRunes(t.font, prefix)
		canvas.DrawText(string(prefix), textX, textY, t.font, t.textColor)
		preeditX := textX + prefixW
		preeditW := fm.MeasureRunes(t.font, []rune(t.preedit))
		if preeditW > 0 {
			canvas.FillRect(preeditX, textY, preeditW, textHeight, t.focusBorderColor)
			canvas.DrawText(t.preedit, preeditX, textY, t.font, t.textColor)
			canvas.DrawHorizontalLine(preeditX, preeditX+preeditW, textY+textHeight-2, t.textColor)
		}
		canvas.DrawText(string(tail), preeditX+preeditW, textY, t.font, textColor)
	} else if displayText != "" {
		canvas.DrawText(displayText, textX, textY, t.font, textColor)
	}

	// 绘制光标（聚焦且非只读，500ms 周期闪烁）
	if t.focused && !t.readOnly {
		prefixWidth := fm.MeasureRunes(t.font, runes[:cursorPos])
		if t.preedit != "" {
			prefixWidth += fm.MeasureRunes(t.font, []rune(t.preedit))
		}
		cursorX := textX + prefixWidth

		if (time.Now().UnixMilli()/500)%2 == 0 {
			canvas.DrawVerticalLine(cursorX, bounds.Y+4, bounds.Y+bounds.Height-4, t.textColor)
		}
	}

	canvas.PopClip()
}

// OnEvent 处理事件
func (t *TextBox) OnEvent(evt event.Event) bool {
	if !t.visible || !t.enabled {
		return false
	}

	switch evt.Type {
	case event.EventMouseDown:
		data, ok := evt.Data.(event.MouseData)
		if !ok {
			return false
		}
		if data.Button != event.MouseButtonLeft && data.Button != event.MouseButtonNone {
			return false
		}
		if IsInBounds(data.X, data.Y, t.bounds) {
			RequestFocus(t)
			t.preedit = ""
			// 基于字体宽度精确定位点击的字符索引（单次 O(n) 扫描，取最近的字符边界）
			clickX := data.X - t.bounds.X - 6 + t.scrollOffsetX
			runes := []rune(t.text)
			fm := render.GetFontManager()
			face := fm.GetFace(t.font)
			bestPos := len(runes)
			acc := int32(0)
			for i, r := range runes {
				rw := fm.RuneWidthFace(face, r)
				if clickX < acc+rw/2 {
					bestPos = i
					break
				}
				acc += rw
			}
			if clickX <= 0 {
				bestPos = 0
			}
			t.cursorPos = bestPos
			t.ensureCaretVisible()
			return true
		} else {
			t.focused = false
			t.preedit = ""
			return false
		}

	case event.EventKeyDown:
		if !t.focused || t.readOnly {
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
		t.preedit = ""
		return true

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
		if !ok || t.preedit == data.Text {
			return false
		}
		t.preedit = data.Text
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
		t.insertRunes([]rune(data.Text))
		return true

	case event.EventImeEnd:
		if t.preedit == "" {
			return false
		}
		t.preedit = ""
		return true
	}

	return false
}

// handleKeyDown 处理快捷键与光标移动
func (t *TextBox) handleKeyDown(data event.KeyData) bool {
	defer t.ensureCaretVisible()
	runes := []rune(t.text)

	// Ctrl 组合快捷键（剪贴板交互）
	if data.Ctrl {
		switch data.KeyCode {
		case 'A', 'a': // 全选（暂定移动到末尾）
			t.cursorPos = len(runes)
			return true
		case 'C', 'c': // 复制全量文本
			if t.text != "" {
				clipboard.SetText(t.text)
			}
			return true
		case 'X', 'x': // 剪切
			if t.text != "" {
				clipboard.SetText(t.text)
				t.text = ""
				t.cursorPos = 0
				t.notifyChange()
			}
			return true
		case 'V', 'v': // 粘贴
			clipText := clipboard.GetText()
			if clipText != "" {
				if t.cursorPos > len(runes) {
					t.cursorPos = len(runes)
				}
				// SetText 不受 maxLength 约束，maxLength 也可能被下调到小于既有文本；
				// 此时 room <= 0，必须直接忽略粘贴，否则会切出负长度。
				room := t.maxLength - len(runes)
				if room <= 0 {
					return true
				}
				clipRunes := []rune(clipText)
				if len(clipRunes) > room {
					clipRunes = clipRunes[:room]
				}
				newRunes := append(runes[:t.cursorPos], append(clipRunes, runes[t.cursorPos:]...)...)
				t.text = string(newRunes)
				t.cursorPos += len(clipRunes)
				t.notifyChange()
				return true
			}
		}
	}

	switch data.KeyCode {
	case 0x08: // Backspace
		if t.cursorPos > 0 && len(runes) > 0 {
			newRunes := append(runes[:t.cursorPos-1], runes[t.cursorPos:]...)
			t.text = string(newRunes)
			t.cursorPos--
			t.notifyChange()
		}
		return true

	case 0x2E: // Delete
		if t.cursorPos < len(runes) {
			newRunes := append(runes[:t.cursorPos], runes[t.cursorPos+1:]...)
			t.text = string(newRunes)
			t.notifyChange()
		}
		return true

	case 0x25: // Left Arrow
		if t.cursorPos > 0 {
			t.cursorPos--
		}
		return true

	case 0x27: // Right Arrow
		if t.cursorPos < len(runes) {
			t.cursorPos++
		}
		return true

	case 0x24: // Home
		t.cursorPos = 0
		return true

	case 0x23: // End
		t.cursorPos = len(runes)
		return true

	case 0x0D: // Enter
		if t.onSubmit != nil {
			t.onSubmit(t.text)
		}
		return true
	}

	return false
}

// ImeAnchor 返回输入法组合窗口应贴靠的光标客户区坐标与行高
func (t *TextBox) ImeAnchor() (x, y, lineHeight int32, ok bool) {
	if !t.focused || t.readOnly || !t.visible || t.bounds.Width <= 0 {
		return 0, 0, 0, false
	}
	fm := render.GetFontManager()
	face := fm.GetFace(t.font)
	if face == nil || face.Height <= 0 {
		return 0, 0, 0, false
	}
	runes := []rune(t.text)
	pos := t.cursorPos
	if pos > len(runes) {
		pos = len(runes)
	}
	if pos < 0 {
		pos = 0
	}
	w := fm.MeasureRunesFace(face, runes[:pos])
	if t.preedit != "" {
		w += fm.MeasureRunesFace(face, []rune(t.preedit))
	}
	x = t.bounds.X + 6 + w - t.scrollOffsetX
	y = t.bounds.Y + (t.bounds.Height-face.Height)/2
	return x, y, face.Height, true
}

// insertRunes 在光标处插入字符（自动遵守最大长度）
func (t *TextBox) insertRunes(add []rune) {
	if len(add) == 0 {
		return
	}
	runes := []rune(t.text)
	if t.cursorPos > len(runes) {
		t.cursorPos = len(runes)
	}
	if t.cursorPos < 0 {
		t.cursorPos = 0
	}
	if room := t.maxLength - len(runes); room < len(add) {
		if room <= 0 {
			return
		}
		add = add[:room]
	}
	newRunes := make([]rune, 0, len(runes)+len(add))
	newRunes = append(newRunes, runes[:t.cursorPos]...)
	newRunes = append(newRunes, add...)
	newRunes = append(newRunes, runes[t.cursorPos:]...)
	t.text = string(newRunes)
	t.cursorPos += len(add)
	t.notifyChange()
	t.ensureCaretVisible()
}

// handleKeyPress 处理字符输入（完美支持中文与所有 Unicode 字符）
func (t *TextBox) handleKeyPress(data event.KeyData) bool {
	ch := data.Char
	// 忽略控制字符与 DEL
	if ch < 32 || ch == 127 {
		return false
	}
	runes := []rune(t.text)
	if len(runes) >= t.maxLength {
		return false
	}
	if t.cursorPos > len(runes) {
		t.cursorPos = len(runes)
	}

	// 插入字符
	newRunes := append(runes[:t.cursorPos], append([]rune{ch}, runes[t.cursorPos:]...)...)
	t.text = string(newRunes)
	t.cursorPos++
	t.notifyChange()
	t.ensureCaretVisible()
	return true
}

// ensureCaretVisible 调整水平滚动偏移，保证光标与前后的输入内容在可视区内。
func (t *TextBox) ensureCaretVisible() {
	if t.bounds.Width <= 0 {
		t.scrollOffsetX = 0
		return
	}
	runes := []rune(t.text)
	pos := t.cursorPos
	if pos < 0 {
		pos = 0
	}
	if pos > len(runes) {
		pos = len(runes)
	}
	fm := render.GetFontManager()
	face := fm.GetFace(t.font)
	caret := fm.MeasureRunesFace(face, runes[:pos])
	total := fm.MeasureRunesFace(face, runes)
	if t.preedit != "" {
		w := fm.MeasureRunesFace(face, []rune(t.preedit))
		caret += w
		total += w
	}
	viewW := t.bounds.Width - 16
	if viewW < 1 {
		viewW = 1
	}
	if caret-t.scrollOffsetX > viewW {
		t.scrollOffsetX = caret - viewW
	}
	if t.scrollOffsetX > caret {
		t.scrollOffsetX = caret
	}
	if limit := total - viewW; t.scrollOffsetX > limit {
		t.scrollOffsetX = limit
	}
	if t.scrollOffsetX < 0 {
		t.scrollOffsetX = 0
	}
}

// notifyChange 通知文本改变
func (t *TextBox) notifyChange() {
	if t.onChange != nil {
		t.onChange(t.text)
	}
}

// SetText 设置文本
func (t *TextBox) SetText(text string) {
	t.text = text
	t.cursorPos = len([]rune(text))
	t.ensureCaretVisible()
}

// SetBounds 调整尺寸后重新保证光标可见（宽度变化会影响水平滚动上限）。
func (t *TextBox) SetBounds(x, y, width, height int32) {
	t.BaseComponent.SetBounds(x, y, width, height)
	t.ensureCaretVisible()
}

// GetText 获取文本
func (t *TextBox) GetText() string {
	return t.text
}

// SetFont 设置文本字体
func (t *TextBox) SetFont(font *Font) {
	if font != nil {
		t.font = font
		t.ensureCaretVisible()
	}
}

// SetPlaceholder 设置占位符
func (t *TextBox) SetPlaceholder(placeholder string) {
	t.placeholder = placeholder
}

// SetReadOnly 设置只读
func (t *TextBox) SetReadOnly(readOnly bool) {
	t.readOnly = readOnly
}

// IsReadOnly 检查是否只读
func (t *TextBox) IsReadOnly() bool {
	return t.readOnly
}

// SetMaxLength 设置最大字符长度。下调上限不会截断既有文本或移动光标，
// 只限制后续输入/粘贴；粘贴时按剩余空间裁剪。
func (t *TextBox) SetMaxLength(maxLength int) {
	t.maxLength = maxLength
}

// SetOnChange 设置改变回调
func (t *TextBox) SetOnChange(callback func(text string)) {
	t.onChange = callback
}

// SetOnSubmit 设置提交回调
func (t *TextBox) SetOnSubmit(callback func(text string)) {
	t.onSubmit = callback
}

// IsFocused 检查是否聚焦
func (t *TextBox) IsFocused() bool {
	return t.focused
}

// SetFocused 设置聚焦
func (t *TextBox) SetFocused(focused bool) {
	t.focused = focused
}
