package component

import (
	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/theme"
)

// CheckBox 复选框组件
type CheckBox struct {
	BaseComponent
	text        string
	font        *Font
	checked     bool
	textColor   graphics.Color
	boxBg       graphics.Color
	borderColor graphics.Color
	activeColor graphics.Color
	checkColor  graphics.Color
	onChange    func(checked bool)
}

// NewCheckBox 创建复选框（自动跟随当前主题）
func NewCheckBox(text string) *CheckBox {
	c := &CheckBox{
		BaseComponent: NewBaseComponent(),
		text:          text,
		font:          DefaultFont,
	}
	c.ApplyTheme(theme.Current())
	return c
}

// ApplyTheme 应用主题样式
func (c *CheckBox) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	c.textColor = t.TextPrimary
	c.activeColor = t.Primary
	c.boxBg = t.Surface
	c.borderColor = t.Border
	c.checkColor = t.OnPrimary
}

// Render 渲染复选框
func (c *CheckBox) Render(canvas *render.Canvas) {
	if !c.visible {
		return
	}

	bounds := c.bounds
	boxSize := int32(16)
	boxX := bounds.X
	boxY := bounds.Y + (bounds.Height-boxSize)/2

	// 绘制复选框
	if c.checked {
		canvas.FillRect(boxX, boxY, boxSize, boxSize, c.activeColor)
		// 绘制勾选标记
		c.drawCheckMark(canvas, boxX, boxY, boxSize)
	} else {
		canvas.FillRect(boxX, boxY, boxSize, boxSize, c.boxBg)
		canvas.DrawRect(boxX, boxY, boxSize, boxSize, c.borderColor)
	}

	// 绘制文本
	if c.text != "" {
		_, textHeight := canvas.MeasureText(c.text, c.font)
		textX := boxX + boxSize + 6
		textY := bounds.Y + (bounds.Height-textHeight)/2
		canvas.DrawText(c.text, textX, textY, c.font, c.textColor)
	}
}

// drawCheckMark 绘制勾选标记
func (c *CheckBox) drawCheckMark(canvas *render.Canvas, x, y, size int32) {
	padding := int32(3)
	canvas.SetPixel(x+padding+1, y+size/2, c.checkColor)
	canvas.SetPixel(x+padding+2, y+size/2+1, c.checkColor)
	canvas.SetPixel(x+padding+2, y+size/2+2, c.checkColor)
	canvas.SetPixel(x+padding+3, y+size/2+3, c.checkColor)
	canvas.SetPixel(x+padding+4, y+size/2+2, c.checkColor)
	canvas.SetPixel(x+padding+5, y+size/2+1, c.checkColor)
	canvas.SetPixel(x+padding+6, y+size/2, c.checkColor)
	canvas.SetPixel(x+padding+7, y+size/2-1, c.checkColor)
	canvas.SetPixel(x+padding+8, y+size/2-2, c.checkColor)
}

// GetCursor 获取鼠标悬停光标（手型）
func (c *CheckBox) GetCursor() int {
	return 32649 // IDC_HAND
}

// OnEvent 处理事件
func (c *CheckBox) OnEvent(evt event.Event) bool {
	if !c.visible || !c.enabled {
		return false
	}

	switch evt.Type {
	case event.EventMouseDown:
		data, ok := evt.Data.(event.MouseData)
		if !ok {
			return false
		}
		// 右键/中键不应切换勾选状态
		if data.Button != event.MouseButtonRight && data.Button != event.MouseButtonMiddle &&
			IsInBounds(data.X, data.Y, c.bounds) {
			c.checked = !c.checked
			if c.onChange != nil {
				c.onChange(c.checked)
			}
			return true
		}
	}

	return false
}

// SetChecked 设置选中状态
func (c *CheckBox) SetChecked(checked bool) {
	c.checked = checked
}

// IsChecked 检查选中状态
func (c *CheckBox) IsChecked() bool {
	return c.checked
}

// SetText 设置文本
func (c *CheckBox) SetText(text string) {
	c.text = text
}

// GetText 获取文本
func (c *CheckBox) GetText() string {
	return c.text
}

// SetOnChange 设置改变回调
func (c *CheckBox) SetOnChange(callback func(checked bool)) {
	c.onChange = callback
}
