package component

import (
	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/theme"
)

// RadioButton 单选按钮组件
type RadioButton struct {
	BaseComponent
	text        string
	font        *Font
	selected    bool
	group       *RadioGroup
	textColor   graphics.Color
	radioBg     graphics.Color
	borderColor graphics.Color
	activeColor graphics.Color
	onChange    func(selected bool)
}

// RadioGroup 单选按钮组
type RadioGroup struct {
	buttons []*RadioButton
}

// NewRadioGroup 创建单选按钮组
func NewRadioGroup() *RadioGroup {
	return &RadioGroup{}
}

// Add 添加单选按钮
func (g *RadioGroup) Add(button *RadioButton) {
	button.group = g
	g.buttons = append(g.buttons, button)
}

// Select 选择按钮
func (g *RadioGroup) Select(button *RadioButton) {
	for _, b := range g.buttons {
		b.selected = (b == button)
		if b.onChange != nil {
			b.onChange(b.selected)
		}
	}
}

// GetSelected 获取选中的按钮
func (g *RadioGroup) GetSelected() *RadioButton {
	for _, b := range g.buttons {
		if b.selected {
			return b
		}
	}
	return nil
}

// GetSelectedIndex 获取选中索引
func (g *RadioGroup) GetSelectedIndex() int {
	for i, b := range g.buttons {
		if b.selected {
			return i
		}
	}
	return -1
}

// NewRadioButton 创建单选按钮（自动跟随当前主题）
func NewRadioButton(text string) *RadioButton {
	r := &RadioButton{
		BaseComponent: NewBaseComponent(),
		text:          text,
		font:          DefaultFont,
	}
	r.ApplyTheme(theme.Current())
	return r
}

// ApplyTheme 应用主题样式
func (r *RadioButton) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	r.textColor = t.TextPrimary
	r.activeColor = t.Primary
	r.radioBg = t.Surface
	r.borderColor = t.Border
}

// Render 渲染单选按钮
func (r *RadioButton) Render(canvas *render.Canvas) {
	if !r.visible {
		return
	}

	bounds := r.bounds
	radius := int32(8)
	centerX := bounds.X + radius
	centerY := bounds.Y + (bounds.Height)/2

	// 绘制内圆底色
	canvas.FillCircle(centerX, centerY, radius-1, r.radioBg)

	// 绘制外圆边框
	borderCol := r.borderColor
	if r.selected {
		borderCol = r.activeColor
	}
	canvas.DrawCircle(centerX, centerY, radius, borderCol)

	// 绘制内圆标记（选中时）
	if r.selected {
		canvas.FillCircle(centerX, centerY, radius-3, r.activeColor)
	}

	// 绘制文本
	if r.text != "" {
		_, textHeight := canvas.MeasureText(r.text, r.font)
		textX := bounds.X + radius*2 + 6
		textY := bounds.Y + (bounds.Height-textHeight)/2
		canvas.DrawText(r.text, textX, textY, r.font, r.textColor)
	}
}

// GetCursor 获取鼠标悬停光标（手型）
func (r *RadioButton) GetCursor() int {
	return 32649 // IDC_HAND
}

// OnEvent 处理事件
func (r *RadioButton) OnEvent(evt event.Event) bool {
	if !r.visible || !r.enabled {
		return false
	}

	switch evt.Type {
	case event.EventMouseDown:
		data, ok := evt.Data.(event.MouseData)
		if !ok {
			return false
		}
		// 右键/中键不应改变选中项
		if data.Button != event.MouseButtonRight && data.Button != event.MouseButtonMiddle &&
			IsInBounds(data.X, data.Y, r.bounds) {
			if r.group != nil {
				r.group.Select(r)
			} else {
				r.selected = true
				if r.onChange != nil {
					r.onChange(true)
				}
			}
			return true
		}
	}

	return false
}

// SetSelected 设置选中状态
func (r *RadioButton) SetSelected(selected bool) {
	r.selected = selected
}

// IsSelected 检查选中状态
func (r *RadioButton) IsSelected() bool {
	return r.selected
}

// SetText 设置文本
func (r *RadioButton) SetText(text string) {
	r.text = text
}

// GetText 获取文本
func (r *RadioButton) GetText() string {
	return r.text
}

// SetOnChange 设置改变回调
func (r *RadioButton) SetOnChange(callback func(selected bool)) {
	r.onChange = callback
}
