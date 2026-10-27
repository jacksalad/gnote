package component

import (
	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/theme"
)

// Button 按钮组件
type Button struct {
	BaseComponent
	text    string
	font    *Font
	onClick func()

	// 状态
	hover   bool
	pressed bool
	focused bool

	// 主题与角色
	isPrimary    bool
	customColors bool

	// 颜色
	normalBg    graphics.Color
	hoverBg     graphics.Color
	pressedBg   graphics.Color
	disabledBg  graphics.Color
	textColor   graphics.Color
	borderColor graphics.Color
	focusRing   graphics.Color

	// 圆角
	borderRadius int32
}

// NewButton 创建按钮（默认次级/标准样式，自动响应主题）
func NewButton(text string) *Button {
	b := &Button{
		BaseComponent: NewBaseComponent(),
		text:          text,
		font:          DefaultFont,
		borderRadius:  6,
	}
	b.ApplyTheme(theme.Current())
	return b
}

// NewPrimaryButton 创建主题核心强调色按钮
func NewPrimaryButton(text string) *Button {
	b := NewButton(text)
	b.SetPrimary(true)
	return b
}

// SetPrimary 设置是否为强调主按钮
func (b *Button) SetPrimary(primary bool) {
	b.isPrimary = primary
	b.ApplyTheme(theme.Current())
}

// IsPrimary 检查是否为主按钮
func (b *Button) IsPrimary() bool {
	return b.isPrimary
}

// ApplyTheme 应用主题样式
func (b *Button) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	b.focusRing = t.FocusRing
	if b.customColors {
		return
	}
	if b.isPrimary {
		b.normalBg = t.Primary
		b.hoverBg = t.PrimaryHover
		b.pressedBg = t.PrimaryActive
		b.disabledBg = t.SurfaceVariant
		b.textColor = t.OnPrimary
		b.borderColor = t.Primary
	} else {
		b.normalBg = t.SurfaceVariant
		b.hoverBg = t.Primary
		b.pressedBg = t.PrimaryActive
		b.disabledBg = t.Surface
		b.textColor = t.TextPrimary
		b.borderColor = t.Border
	}
}

// Render 渲染按钮
func (b *Button) Render(canvas *render.Canvas) {
	if !b.visible {
		return
	}

	bounds := b.bounds

	// 选择背景颜色
	bgColor := b.normalBg
	if !b.enabled {
		bgColor = b.disabledBg
	} else if b.pressed {
		bgColor = b.pressedBg
	} else if b.hover {
		bgColor = b.hoverBg
	}

	// 绘制背景
	if b.borderRadius > 0 {
		canvas.FillRoundedRect(bounds.X, bounds.Y, bounds.Width, bounds.Height, b.borderRadius, bgColor)
	} else {
		canvas.FillRect(bounds.X, bounds.Y, bounds.Width, bounds.Height, bgColor)
	}

	// 绘制边框
	borderColor := b.borderColor
	if b.focused {
		borderColor = b.focusRing
	}
	if b.borderRadius > 0 {
		canvas.DrawRoundedRect(bounds.X, bounds.Y, bounds.Width, bounds.Height, b.borderRadius, borderColor)
	} else {
		canvas.DrawRect(bounds.X, bounds.Y, bounds.Width, bounds.Height, borderColor)
	}

	// 绘制文本
	if b.text != "" {
		b.renderText(canvas, bounds)
	}
}

// renderText 渲染文本
func (b *Button) renderText(canvas *render.Canvas, bounds graphics.Rect) {
	textWidth, textHeight := canvas.MeasureText(b.text, b.font)

	// 居中对齐
	x := bounds.X + (bounds.Width-textWidth)/2
	y := bounds.Y + (bounds.Height-textHeight)/2

	canvas.DrawText(b.text, x, y, b.font, b.textColor)
}

// GetCursor 获取鼠标悬停光标（手型）
func (b *Button) GetCursor() int {
	return 32649 // IDC_HAND
}

// OnEvent 处理事件
func (b *Button) OnEvent(evt event.Event) bool {
	if !b.visible || !b.enabled {
		return false
	}

	switch evt.Type {
	case event.EventMouseMove:
		data, ok := evt.Data.(event.MouseData)
		if !ok {
			return false
		}
		wasHover := b.hover
		b.hover = IsInBounds(data.X, data.Y, b.bounds)
		return b.hover != wasHover

	case event.EventMouseDown:
		data, ok := evt.Data.(event.MouseData)
		if !ok {
			return false
		}
		if data.Button == event.MouseButtonLeft && IsInBounds(data.X, data.Y, b.bounds) {
			b.pressed = true
			return true
		}

	case event.EventMouseUp:
		data, ok := evt.Data.(event.MouseData)
		if !ok {
			return false
		}
		if b.pressed && data.Button == event.MouseButtonLeft {
			b.pressed = false
			if IsInBounds(data.X, data.Y, b.bounds) {
				// 触发点击
				if b.onClick != nil {
					b.onClick()
				}
				return true
			}
		}

	case event.EventFocus:
		b.focused = true
		return true

	case event.EventBlur:
		b.focused = false
		b.pressed = false
		return true
	}

	return false
}

// OnClick 设置点击回调
func (b *Button) OnClick(callback func()) {
	b.onClick = callback
}

// SetHover 设置悬停状态
func (b *Button) SetHover(hover bool) {
	b.hover = hover
}

// IsHover 检查悬停状态
func (b *Button) IsHover() bool {
	return b.hover
}

// SetPressed 设置按下状态
func (b *Button) SetPressed(pressed bool) {
	b.pressed = pressed
}

// IsPressed 检查按下状态
func (b *Button) IsPressed() bool {
	return b.pressed
}

// SetText 设置文本
func (b *Button) SetText(text string) {
	b.text = text
}

// GetText 获取文本
func (b *Button) GetText() string {
	return b.text
}

// SetFont 设置字体
func (b *Button) SetFont(font *Font) {
	b.font = font
}

// GetFont 获取字体
func (b *Button) GetFont() *Font {
	return b.font
}

// SetColors 设置自定义颜色（设置后不再被自动主题覆盖）
func (b *Button) SetColors(normal, hover, pressed, disabled, text, border graphics.Color) {
	b.customColors = true
	b.normalBg = normal
	b.hoverBg = hover
	b.pressedBg = pressed
	b.disabledBg = disabled
	b.textColor = text
	b.borderColor = border
}

// SetBorderRadius 设置圆角半径
func (b *Button) SetBorderRadius(radius int32) {
	b.borderRadius = radius
}
