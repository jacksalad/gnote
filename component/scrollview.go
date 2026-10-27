package component

import (
	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/theme"
)

// ScrollView 滚动视图容器控件，提供视口裁剪、鼠标滚轮滚动与原生滚动条交互
type ScrollView struct {
	BaseComponent
	content       Component
	scrollOffsetY int32 // 垂直滚动偏移 (>= 0)
	scrollOffsetX int32 // 水平滚动偏移 (>= 0)
	contentWidth  int32 // 内容实际完整宽度
	contentHeight int32 // 内容实际完整高度

	// 滚动条配置
	scrollbarWidth int32          // 滚动条宽度 (默认 10)
	trackColor     graphics.Color // 滚动条轨道颜色
	thumbColor     graphics.Color // 滚动条滑块颜色
	thumbHover     graphics.Color // 滚动条滑块悬停颜色

	// 交互状态
	isDraggingThumb    bool
	dragStartY         int32
	dragStartScrollY   int32
	isThumbHovered     bool
	isScrollBarHovered bool
	// 滚轮 1/120 刻度的像素余数，保证触摸板分数增量也能平滑滚动
	wheelRemainder int64
}

// NewScrollView 创建新滚动视图
func NewScrollView() *ScrollView {
	th := theme.Current()
	return &ScrollView{
		BaseComponent:  NewBaseComponent(),
		scrollbarWidth: 10,
		trackColor:     th.Track,
		thumbColor:     th.Thumb,
		thumbHover:     th.ThumbHover,
	}
}

// SetContent 设置滚动视图容纳的子组件
func (s *ScrollView) SetContent(comp Component) {
	s.content = comp
	if comp != nil {
		cb := comp.GetBounds()
		if cb.Width > s.contentWidth {
			s.contentWidth = cb.Width
		}
		if cb.Height > s.contentHeight {
			s.contentHeight = cb.Height
		}
	}
	s.updateContentBounds()
}

// GetContent 获取内容组件
func (s *ScrollView) GetContent() Component {
	return s.content
}

// SetContentSize 设置内容的完整虚拟尺寸
func (s *ScrollView) SetContentSize(w, h int32) {
	s.contentWidth = w
	s.contentHeight = h
	s.updateContentBounds()
}

// SetBounds 设置视口位置和可见大小
func (s *ScrollView) SetBounds(x, y, width, height int32) {
	s.BaseComponent.SetBounds(x, y, width, height)
	s.updateContentBounds()
}

// updateContentBounds 同步计算并更新内容组件的位置
func (s *ScrollView) updateContentBounds() {
	if s.content == nil {
		return
	}

	w := s.contentWidth
	if w < s.bounds.Width-s.scrollbarWidth {
		w = s.bounds.Width - s.scrollbarWidth
	}
	h := s.contentHeight
	if h < s.bounds.Height {
		h = s.bounds.Height
	}

	// 限制当前 scrollOffsetY 范围
	maxScrollY := s.getMaxScrollY()
	if s.scrollOffsetY > maxScrollY {
		s.scrollOffsetY = maxScrollY
	}
	if s.scrollOffsetY < 0 {
		s.scrollOffsetY = 0
	}

	// 将内容组件的物理坐标偏移为视口原点减去滚动量
	s.content.SetBounds(
		s.bounds.X-s.scrollOffsetX,
		s.bounds.Y-s.scrollOffsetY,
		w,
		h,
	)
}

// getMaxScrollY 计算最大可滚动距离
func (s *ScrollView) getMaxScrollY() int32 {
	if s.contentHeight <= s.bounds.Height {
		return 0
	}
	return s.contentHeight - s.bounds.Height
}

// ScrollTo 滚动到指定偏移坐标
func (s *ScrollView) ScrollTo(x, y int32) {
	maxScrollY := s.getMaxScrollY()
	if y > maxScrollY {
		y = maxScrollY
	}
	if y < 0 {
		y = 0
	}
	s.scrollOffsetY = y
	s.scrollOffsetX = x
	s.updateContentBounds()
}

// ScrollBy 按增量滚动
func (s *ScrollView) ScrollBy(dx, dy int32) {
	s.ScrollTo(s.scrollOffsetX+dx, s.scrollOffsetY+dy)
}

// GetScrollOffset 获取当前滚动偏移
func (s *ScrollView) GetScrollOffset() (int32, int32) {
	return s.scrollOffsetX, s.scrollOffsetY
}

// getThumbRect 计算当前垂直滚动条滑块的几何矩形
func (s *ScrollView) getThumbRect() graphics.Rect {
	maxScrollY := s.getMaxScrollY()
	if maxScrollY <= 0 {
		return graphics.Rect{}
	}

	barX := s.bounds.X + s.bounds.Width - s.scrollbarWidth
	barY := s.bounds.Y
	barH := s.bounds.Height

	thumbH := int32(float64(barH) * float64(s.bounds.Height) / float64(s.contentHeight))
	if thumbH < 24 {
		thumbH = 24
	}
	if thumbH > barH {
		thumbH = barH
	}

	usableH := barH - thumbH
	thumbY := barY
	if maxScrollY > 0 && usableH > 0 {
		thumbY = barY + int32(float64(usableH)*float64(s.scrollOffsetY)/float64(maxScrollY))
	}

	return graphics.NewRect(barX+1, thumbY, s.scrollbarWidth-2, thumbH)
}

// ApplyTheme 应用主题
func (s *ScrollView) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	s.trackColor = t.Track
	s.thumbColor = t.Thumb
	s.thumbHover = t.ThumbHover
	if s.content != nil {
		if th, ok := s.content.(Themeable); ok {
			th.ApplyTheme(t)
		}
	}
}

// OnEvent 处理鼠标滚轮与滚动条拖动事件
func (s *ScrollView) OnEvent(evt event.Event) bool {
	if !s.visible || !s.enabled {
		return false
	}

	maxScrollY := s.getMaxScrollY()

	switch evt.Type {
	case event.EventMouseWheel:
		if mouse, ok := evt.Data.(event.MouseData); ok {
			if s.bounds.Contains(graphics.Point{X: mouse.X, Y: mouse.Y}) {
				if maxScrollY > 0 {
					// 每个标准刻度滚动 40 像素；分数增量按像素累加，避免小增量被丢弃
					pixels := consumeWheelDelta(&s.wheelRemainder, int64(mouse.Delta), 40)
					if pixels != 0 {
						s.ScrollBy(0, -pixels)
					}
					return true
				}
			}
		}

	case event.EventMouseDown:
		if mouse, ok := evt.Data.(event.MouseData); ok {
			if mouse.Button == event.MouseButtonLeft && maxScrollY > 0 {
				barX := s.bounds.X + s.bounds.Width - s.scrollbarWidth
				barRect := graphics.NewRect(barX, s.bounds.Y, s.scrollbarWidth, s.bounds.Height)
				pt := graphics.Point{X: mouse.X, Y: mouse.Y}

				if barRect.Contains(pt) {
					thumbRect := s.getThumbRect()
					if thumbRect.Contains(pt) {
						s.isDraggingThumb = true
						s.dragStartY = mouse.Y
						s.dragStartScrollY = s.scrollOffsetY
						return true
					} else {
						// 点击轨道空白处快速翻页跳转
						if mouse.Y < thumbRect.Y {
							s.ScrollBy(0, -s.bounds.Height)
						} else {
							s.ScrollBy(0, s.bounds.Height)
						}
						return true
					}
				}
			}
		}

	case event.EventMouseMove:
		if mouse, ok := evt.Data.(event.MouseData); ok {
			if s.isDraggingThumb {
				barH := s.bounds.Height
				thumbRect := s.getThumbRect()
				usableH := barH - thumbRect.Height
				if usableH > 0 {
					dy := mouse.Y - s.dragStartY
					newScroll := s.dragStartScrollY + int32(float64(dy)*float64(maxScrollY)/float64(usableH))
					s.ScrollTo(s.scrollOffsetX, newScroll)
				}
				return true
			}

			if maxScrollY > 0 {
				thumbRect := s.getThumbRect()
				isOverThumb := thumbRect.Contains(graphics.Point{X: mouse.X, Y: mouse.Y})
				if isOverThumb != s.isThumbHovered {
					s.isThumbHovered = isOverThumb
					return true
				}
			}
		}

	case event.EventMouseUp:
		if mouse, ok := evt.Data.(event.MouseData); ok {
			if mouse.Button == event.MouseButtonLeft && s.isDraggingThumb {
				s.isDraggingThumb = false
				return true
			}
		}
	}

	// 转发事件给内部内容控件 (只有鼠标在视口范围内才派发)
	if s.content != nil {
		// 检查鼠标类事件是否在当前视口内
		if mouse, ok := evt.Data.(event.MouseData); ok {
			if !s.bounds.Contains(graphics.Point{X: mouse.X, Y: mouse.Y}) {
				return false
			}
		}
		return s.content.OnEvent(evt)
	}

	return false
}

// Render 绘制滚动视图内容与滚动条
func (s *ScrollView) Render(canvas *render.Canvas) {
	if !s.visible || canvas == nil {
		return
	}

	b := s.bounds
	if b.Width <= 0 || b.Height <= 0 {
		return
	}

	// 1. 使用裁剪栈隔离内容渲染，超出视口部分绝对不溢出
	canvas.PushClip(b)
	if s.content != nil {
		s.content.Render(canvas)
	}
	canvas.PopClip()

	// 2. 如果内容高度超出视口，绘制垂直滚动条
	maxScrollY := s.getMaxScrollY()
	if maxScrollY > 0 {
		barX := b.X + b.Width - s.scrollbarWidth
		barY := b.Y
		barH := b.Height

		// 绘制滚动条底轨
		canvas.FillRect(barX, barY, s.scrollbarWidth, barH, s.trackColor)

		// 绘制滑块
		thumbRect := s.getThumbRect()
		col := s.thumbColor
		if s.isDraggingThumb || s.isThumbHovered {
			col = s.thumbHover
		}
		radius := (s.scrollbarWidth - 2) / 2
		if radius < 2 {
			radius = 2
		}
		canvas.FillRoundedRect(thumbRect.X, thumbRect.Y, thumbRect.Width, thumbRect.Height, radius, col)
	}
}
