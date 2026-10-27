package component

import (
	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/theme"
)

// Switch 开关控件。用于设置页的布尔项，比复选框更接近现代桌面设置界面。
type Switch struct {
	BaseComponent
	on       bool
	hover    bool
	onChange func(bool)

	trackOn  graphics.Color
	trackOff graphics.Color
	thumbOn  graphics.Color
	thumbOff graphics.Color
	border   graphics.Color
	disabled graphics.Color
}

// NewSwitch 创建开关。
func NewSwitch() *Switch {
	s := &Switch{BaseComponent: NewBaseComponent()}
	s.ApplyTheme(theme.Current())
	return s
}

// SetOn 设置状态，不触发回调。
func (s *Switch) SetOn(on bool) {
	s.on = on
}

// IsOn 返回当前状态。
func (s *Switch) IsOn() bool {
	return s.on
}

// SetOnChange 设置切换回调。
func (s *Switch) SetOnChange(fn func(bool)) {
	s.onChange = fn
}

// ApplyTheme 应用主题色。
func (s *Switch) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	s.trackOn = t.Primary
	s.trackOff = t.SurfaceVariant
	s.thumbOn = t.OnPrimary
	s.thumbOff = graphics.ColorWhite
	s.border = t.Border
	s.disabled = t.TextDisabled
}

// Render 绘制开关。
func (s *Switch) Render(canvas *render.Canvas) {
	if !s.visible {
		return
	}
	b := s.bounds
	if b.Width <= 0 || b.Height <= 0 {
		return
	}
	if b.Height > b.Width/2 {
		b.Height = b.Width / 2
	}
	radius := b.Height / 2

	track := s.trackOff
	if s.on {
		track = s.trackOn
	}
	if !s.enabled {
		track = s.disabled
	}
	canvas.FillRoundedRect(b.X, b.Y, b.Width, b.Height, radius, track)
	canvas.DrawRoundedRect(b.X, b.Y, b.Width, b.Height, radius, s.border)

	thumbRadius := b.Height/2 - 3
	if thumbRadius < 2 {
		thumbRadius = 2
	}
	cx := b.X + b.Height/2
	if s.on {
		cx = b.X + b.Width - b.Height/2
	}
	thumb := s.thumbOff
	if s.on {
		thumb = s.thumbOn
	}
	canvas.FillCircle(cx, b.Y+b.Height/2, thumbRadius, thumb)
}

// GetCursor 返回手型光标。
func (s *Switch) GetCursor() int {
	return 32649
}

// OnEvent 处理鼠标点击。
func (s *Switch) OnEvent(evt event.Event) bool {
	if !s.visible || !s.enabled {
		return false
	}
	switch evt.Type {
	case event.EventMouseMove:
		data, ok := evt.Data.(event.MouseData)
		if !ok {
			return false
		}
		was := s.hover
		s.hover = IsInBounds(data.X, data.Y, s.bounds)
		return s.hover != was

	case event.EventMouseDown:
		data, ok := evt.Data.(event.MouseData)
		if !ok || data.Button != event.MouseButtonLeft {
			return false
		}
		if !IsInBounds(data.X, data.Y, s.bounds) {
			return false
		}
		s.on = !s.on
		if s.onChange != nil {
			s.onChange(s.on)
		}
		return true
	}
	return false
}

// Segmented 横向分段选择器。用于主题等少量互斥选项。
type Segmented struct {
	BaseComponent

	options  []string
	selected int
	font     *Font
	onChange func(int)

	track    graphics.Color
	active   graphics.Color
	text     graphics.Color
	textOn   graphics.Color
	border   graphics.Color
	disabled graphics.Color
}

// NewSegmented 创建分段选择器。
func NewSegmented(options []string) *Segmented {
	s := &Segmented{BaseComponent: NewBaseComponent(), options: options, font: DefaultFont, selected: 0}
	s.ApplyTheme(theme.Current())
	return s
}

// SetFont 设置字体。
func (s *Segmented) SetFont(font *Font) {
	if font != nil {
		s.font = font
	}
}

// SetSelected 设置选中项，不触发回调。
func (s *Segmented) SetSelected(index int) {
	if index >= 0 && index < len(s.options) {
		s.selected = index
	}
}

// Selected 返回选中索引。
func (s *Segmented) Selected() int {
	return s.selected
}

// OnChange 设置选中变化回调。
func (s *Segmented) OnChange(fn func(int)) {
	s.onChange = fn
}

// ApplyTheme 应用主题色。
func (s *Segmented) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	s.track = t.SurfaceVariant
	s.active = t.Primary
	s.text = t.TextPrimary
	s.textOn = t.OnPrimary
	s.border = t.Border
	s.disabled = t.TextDisabled
}

// Render 绘制分段选择器。
func (s *Segmented) Render(canvas *render.Canvas) {
	if !s.visible || len(s.options) == 0 {
		return
	}
	b := s.bounds
	if b.Width <= 0 || b.Height <= 0 {
		return
	}
	canvas.FillRoundedRect(b.X, b.Y, b.Width, b.Height, 6, s.track)
	canvas.DrawRoundedRect(b.X, b.Y, b.Width, b.Height, 6, s.border)

	segW := b.Width / int32(len(s.options))
	for i, option := range s.options {
		x := b.X + int32(i)*segW
		w := segW
		if i == len(s.options)-1 {
			w = b.Right() - x
		}
		if i == s.selected {
			canvas.FillRoundedRect(x+2, b.Y+2, w-4, b.Height-4, 4, s.active)
		}
		color := s.text
		if i == s.selected {
			color = s.textOn
		}
		if !s.enabled {
			color = s.disabled
		}
		tw, th := canvas.MeasureText(option, s.font)
		canvas.DrawText(option, x+(w-tw)/2, b.Y+(b.Height-th)/2, s.font, color)
	}
}

// GetCursor 返回手型光标。
func (s *Segmented) GetCursor() int {
	return 32649
}

// OnEvent 处理点击选择。
func (s *Segmented) OnEvent(evt event.Event) bool {
	if !s.visible || !s.enabled {
		return false
	}
	data, ok := evt.Data.(event.MouseData)
	if !ok || data.Button != event.MouseButtonLeft {
		return false
	}
	if evt.Type != event.EventMouseDown || !IsInBounds(data.X, data.Y, s.bounds) {
		return false
	}
	if len(s.options) == 0 {
		return true
	}
	segW := s.bounds.Width / int32(len(s.options))
	if segW <= 0 {
		return true
	}
	index := int((data.X - s.bounds.X) / segW)
	if index < 0 {
		index = 0
	}
	if index >= len(s.options) {
		index = len(s.options) - 1
	}
	if index != s.selected {
		s.selected = index
		if s.onChange != nil {
			s.onChange(index)
		}
	}
	return true
}
