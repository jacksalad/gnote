package component

import (
	"math"

	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/theme"
)

// Slider 滑动条控件
type Slider struct {
	BaseComponent
	min         float64
	max         float64
	value       float64
	step        float64 // 步长，0 表示连续
	trackColor  graphics.Color
	fillColor   graphics.Color
	thumbColor  graphics.Color
	thumbHover  graphics.Color
	thumbRing   graphics.Color
	thumbRadius int32
	trackHeight int32
	isHovered   bool
	isDragging  bool
	onChange    func(val float64)
}

// NewSlider 创建新滑动条控件
func NewSlider(min, max, value float64) *Slider {
	th := theme.Current()
	if min >= max {
		max = min + 100
	}
	if value < min {
		value = min
	}
	if value > max {
		value = max
	}

	return &Slider{
		BaseComponent: NewBaseComponent(),
		min:           min,
		max:           max,
		value:         value,
		step:          0,
		trackColor:    th.Track,
		fillColor:     th.Primary,
		thumbColor:    th.Thumb,
		thumbHover:    th.ThumbHover,
		thumbRing:     th.Background,
		thumbRadius:   8,
		trackHeight:   6,
	}
}

// SetValue 设置滑动条当前值
func (s *Slider) SetValue(val float64) {
	if val < s.min {
		val = s.min
	}
	if val > s.max {
		val = s.max
	}
	if s.step > 0 {
		val = math.Round((val-s.min)/s.step)*s.step + s.min
	}
	if s.value != val {
		s.value = val
		if s.onChange != nil {
			s.onChange(s.value)
		}
	}
}

// GetValue 获取当前数值
func (s *Slider) GetValue() float64 {
	return s.value
}

// SetRange 设置滑动范围
func (s *Slider) SetRange(min, max float64) {
	if min >= max {
		return
	}
	s.min = min
	s.max = max
	s.SetValue(s.value)
}

// SetStep 设置调节步长 (0 为无级连续调节)
func (s *Slider) SetStep(step float64) {
	if step < 0 {
		step = 0
	}
	s.step = step
}

// OnChange 注册数值变动回调函数
func (s *Slider) OnChange(handler func(val float64)) {
	s.onChange = handler
}

// SetTrackHeight 设置轨道粗细
func (s *Slider) SetTrackHeight(h int32) {
	if h < 2 {
		h = 2
	}
	s.trackHeight = h
}

// SetThumbRadius 设置滑块圆形半径
func (s *Slider) SetThumbRadius(r int32) {
	if r < 4 {
		r = 4
	}
	s.thumbRadius = r
}

// ApplyTheme 应用主题
func (s *Slider) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	s.trackColor = t.Track
	s.fillColor = t.Primary
	s.thumbColor = t.Thumb
	s.thumbHover = t.ThumbHover
	s.thumbRing = t.Background
}

// GetCursor 实现 CursorProvider 接口 (返回 IDC_HAND)
func (s *Slider) GetCursor() int {
	return 32649 // IDC_HAND
}

// updateValueFromX 根据鼠标 X 坐标更新滑块数值
func (s *Slider) updateValueFromX(mouseX int32) bool {
	b := s.bounds
	usableWidth := b.Width - 2*s.thumbRadius
	if usableWidth <= 0 {
		return false
	}

	relX := mouseX - (b.X + s.thumbRadius)
	ratio := float64(relX) / float64(usableWidth)
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}

	newVal := s.min + ratio*(s.max-s.min)
	if s.step > 0 {
		newVal = math.Round((newVal-s.min)/s.step)*s.step + s.min
	}
	if newVal < s.min {
		newVal = s.min
	}
	if newVal > s.max {
		newVal = s.max
	}

	if newVal != s.value {
		s.value = newVal
		if s.onChange != nil {
			s.onChange(s.value)
		}
		return true
	}
	return false
}

// OnEvent 处理鼠标滑动与点击事件
func (s *Slider) OnEvent(evt event.Event) bool {
	if !s.visible || !s.enabled {
		return false
	}

	switch evt.Type {
	case event.EventMouseDown:
		if mouse, ok := evt.Data.(event.MouseData); ok {
			if mouse.Button == event.MouseButtonLeft && s.bounds.Contains(graphics.Point{X: mouse.X, Y: mouse.Y}) {
				s.isDragging = true
				s.updateValueFromX(mouse.X)
				return true
			}
		}

	case event.EventMouseMove:
		if mouse, ok := evt.Data.(event.MouseData); ok {
			if s.isDragging {
				s.updateValueFromX(mouse.X)
				return true
			}
			isOver := s.bounds.Contains(graphics.Point{X: mouse.X, Y: mouse.Y})
			if isOver != s.isHovered {
				s.isHovered = isOver
				return true
			}
		}

	case event.EventMouseUp:
		if mouse, ok := evt.Data.(event.MouseData); ok {
			if mouse.Button == event.MouseButtonLeft && s.isDragging {
				s.isDragging = false
				return true
			}
		}
	}

	return false
}

// Render 绘制滑块
func (s *Slider) Render(canvas *render.Canvas) {
	if !s.visible || canvas == nil {
		return
	}

	b := s.bounds
	if b.Width <= 0 || b.Height <= 0 {
		return
	}

	trackY := b.Y + (b.Height-s.trackHeight)/2
	trackX := b.X + s.thumbRadius
	usableWidth := b.Width - 2*s.thumbRadius
	if usableWidth < 0 {
		usableWidth = 0
	}

	// 1. 绘制底轨
	canvas.FillRoundedRect(trackX, trackY, usableWidth, s.trackHeight, s.trackHeight/2, s.trackColor)

	// 2. 绘制已填充部分
	rangeVal := s.max - s.min
	var fillW int32
	if rangeVal > 0 && usableWidth > 0 {
		ratio := (s.value - s.min) / rangeVal
		if ratio < 0 {
			ratio = 0
		}
		if ratio > 1 {
			ratio = 1
		}
		fillW = int32(float64(usableWidth) * ratio)
		if fillW > 0 {
			canvas.FillRoundedRect(trackX, trackY, fillW, s.trackHeight, s.trackHeight/2, s.fillColor)
		}
	}

	// 3. 绘制滑块圆形旋钮 (Thumb)
	thumbX := trackX + fillW
	thumbY := b.Y + b.Height/2

	col := s.thumbColor
	if s.isDragging || s.isHovered {
		col = s.thumbHover
	}

	// 外层微边框：先铺描边色圆盘，再用主体色覆盖内圈。
	// 两次均为距离场抗锯齿填充，得到平滑的 1px 圆环轮廓，不再出现锯齿颗粒。
	canvas.FillCircle(thumbX, thumbY, s.thumbRadius, s.thumbRing)
	canvas.FillCircle(thumbX, thumbY, s.thumbRadius-1, col)
}
