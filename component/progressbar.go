package component

import (
	"fmt"

	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/theme"
)

// ProgressBar 进度条控件
type ProgressBar struct {
	BaseComponent
	min         float64
	max         float64
	value       float64
	trackColor  graphics.Color
	barColor    graphics.Color
	borderColor graphics.Color
	borderWidth int32
	radius      int32
	showText    bool
	textColor   graphics.Color
	font        *Font
}

// NewProgressBar 创建默认进度条控件 (范围 0~100，初始值 0)
func NewProgressBar() *ProgressBar {
	th := theme.Current()
	return &ProgressBar{
		BaseComponent: NewBaseComponent(),
		min:           0,
		max:           100,
		value:         0,
		trackColor:    th.Track,
		barColor:      th.Primary,
		borderColor:   th.Border,
		borderWidth:   1,
		radius:        6,
		showText:      false,
		textColor:     th.TextPrimary,
		font:          DefaultFont,
	}
}

// SetValue 设置当前进度值，自动限制在 [min, max] 范围内
func (p *ProgressBar) SetValue(val float64) {
	if val < p.min {
		val = p.min
	}
	if val > p.max {
		val = p.max
	}
	p.value = val
}

// GetValue 获取当前进度值
func (p *ProgressBar) GetValue() float64 {
	return p.value
}

// SetRange 设置进度条数值范围
func (p *ProgressBar) SetRange(min, max float64) {
	if min >= max {
		return
	}
	p.min = min
	p.max = max
	if p.value < min {
		p.value = min
	}
	if p.value > max {
		p.value = max
	}
}

// SetTrackColor 设置底轨背景颜色
func (p *ProgressBar) SetTrackColor(color graphics.Color) {
	p.trackColor = color
}

// SetBarColor 设置填充指示条颜色
func (p *ProgressBar) SetBarColor(color graphics.Color) {
	p.barColor = color
}

// SetBorder 设置边框颜色与宽度
func (p *ProgressBar) SetBorder(color graphics.Color, width int32) {
	p.borderColor = color
	p.borderWidth = width
}

// SetCornerRadius 设置圆角半径
func (p *ProgressBar) SetCornerRadius(radius int32) {
	p.radius = radius
}

// SetShowText 设置是否在进度条居中显示百分比数值
func (p *ProgressBar) SetShowText(show bool) {
	p.showText = show
}

// SetTextColor 设置百分比文字颜色
func (p *ProgressBar) SetTextColor(color graphics.Color) {
	p.textColor = color
}

// SetFont 设置百分比文字字体
func (p *ProgressBar) SetFont(font *Font) {
	p.font = font
}

// ApplyTheme 应用主题样式
func (p *ProgressBar) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	p.trackColor = t.Track
	p.barColor = t.Primary
	p.borderColor = t.Border
	p.textColor = t.TextPrimary
}

// OnEvent 处理事件
func (p *ProgressBar) OnEvent(evt event.Event) bool {
	// ProgressBar 主要作为展示控件，默认不消耗事件
	return false
}

// Render 绘制进度条
func (p *ProgressBar) Render(canvas *render.Canvas) {
	if !p.visible || canvas == nil {
		return
	}

	b := p.bounds
	if b.Width <= 0 || b.Height <= 0 {
		return
	}

	// 1. 绘制底轨
	if p.radius > 0 {
		canvas.FillRoundedRect(b.X, b.Y, b.Width, b.Height, p.radius, p.trackColor)
	} else {
		canvas.FillRect(b.X, b.Y, b.Width, b.Height, p.trackColor)
	}

	// 2. 绘制有效填充条
	rangeVal := p.max - p.min
	if rangeVal > 0 && p.value > p.min {
		ratio := (p.value - p.min) / rangeVal
		if ratio > 1.0 {
			ratio = 1.0
		}
		barWidth := int32(float64(b.Width) * ratio)
		if barWidth > 0 {
			// 使用裁剪栈保证圆角平滑过渡且不超出底轨边界
			canvas.PushClip(graphics.NewRect(b.X, b.Y, barWidth, b.Height))
			if p.radius > 0 {
				canvas.FillRoundedRect(b.X, b.Y, b.Width, b.Height, p.radius, p.barColor)
			} else {
				canvas.FillRect(b.X, b.Y, barWidth, b.Height, p.barColor)
			}
			canvas.PopClip()
		}
	}

	// 3. 绘制外边框
	if p.borderWidth > 0 {
		if p.radius > 0 {
			canvas.DrawRoundedRect(b.X, b.Y, b.Width, b.Height, p.radius, p.borderColor)
		} else {
			canvas.DrawRect(b.X, b.Y, b.Width, b.Height, p.borderColor)
		}
	}

	// 4. 绘制居中百分比文字
	if p.showText && rangeVal > 0 {
		ratio := (p.value - p.min) / rangeVal
		text := fmt.Sprintf("%.0f%%", ratio*100)
		tw, th := canvas.MeasureText(text, p.font)
		tx := b.X + (b.Width-tw)/2
		ty := b.Y + (b.Height-th)/2
		canvas.DrawText(text, tx, ty, p.font, p.textColor)
	}
}
