package component

import (
	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/theme"
)

// PanelRole 面板主题角色
type PanelRole int

const (
	PanelRoleNone           PanelRole = iota // 自定义或透明背景，ApplyTheme 不自动覆盖背景
	PanelRoleBackground                      // 背景角色（如窗口根容器，跟随 t.Background）
	PanelRoleSurface                         // 卡片/表面角色（跟随 t.Surface，边框跟随 t.Border）
	PanelRoleSurfaceVariant                  // 次级容器（跟随 t.SurfaceVariant）
)

// Panel 面板容器组件
type Panel struct {
	BaseComponent
	children    []Component
	background  graphics.Color
	borderColor graphics.Color
	borderWidth int32
	padding     int32
	layout      Layout
	role        PanelRole
}

// NewPanel 创建面板
func NewPanel() *Panel {
	return &Panel{
		BaseComponent: NewBaseComponent(),
		background:    graphics.ColorTransparent,
		borderColor:   graphics.ColorTransparent,
		borderWidth:   0,
		padding:       0,
		role:          PanelRoleNone,
	}
}

// NewSurfacePanel 创建具备卡片/面板主题角色的容器
func NewSurfacePanel() *Panel {
	p := NewPanel()
	p.borderWidth = 1
	p.SetRole(PanelRoleSurface)
	return p
}

// NewBackgroundPanel 创建具备背景主题角色的容器（常用于根面板）
func NewBackgroundPanel() *Panel {
	p := NewPanel()
	p.SetRole(PanelRoleBackground)
	return p
}

// NewPanelWithRole 按角色创建面板
func NewPanelWithRole(role PanelRole) *Panel {
	p := NewPanel()
	p.SetRole(role)
	return p
}

// SetRole 设置面板主题角色
func (p *Panel) SetRole(role PanelRole) {
	p.role = role
	p.ApplyTheme(theme.Current())
}

// GetRole 获取面板主题角色
func (p *Panel) GetRole() PanelRole {
	return p.role
}

// ApplyTheme 为面板及所有子组件递归应用主题样式
func (p *Panel) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	switch p.role {
	case PanelRoleBackground:
		p.background = t.Background
	case PanelRoleSurface:
		p.background = t.Surface
		if p.borderWidth > 0 {
			p.borderColor = t.Border
		}
	case PanelRoleSurfaceVariant:
		p.background = t.SurfaceVariant
		if p.borderWidth > 0 {
			p.borderColor = t.Border
		}
	}

	// 递归为所有子组件应用主题
	for _, child := range p.children {
		if th, ok := child.(Themeable); ok {
			th.ApplyTheme(t)
		}
	}
}

// Render 渲染面板
func (p *Panel) Render(canvas *render.Canvas) {
	if !p.visible {
		return
	}

	// 确保若有布局器已计算完毕
	if p.layout != nil && len(p.children) > 0 {
		p.DoLayout()
	}

	bounds := p.bounds

	// 绘制背景
	if p.background != graphics.ColorTransparent {
		canvas.FillRect(bounds.X, bounds.Y, bounds.Width, bounds.Height, p.background)
	}

	// 绘制边框
	if p.borderWidth > 0 && p.borderColor != graphics.ColorTransparent {
		for i := int32(0); i < p.borderWidth; i++ {
			canvas.DrawRect(bounds.X+i, bounds.Y+i, bounds.Width-2*i, bounds.Height-2*i, p.borderColor)
		}
	}

	// 渲染子组件（限制在面板边界内）
	canvas.PushClip(bounds)
	for _, child := range p.children {
		if child.IsVisible() {
			child.Render(canvas)
		}
	}
	canvas.PopClip()
}

// OnEvent 处理事件（事件冒泡）
func (p *Panel) OnEvent(evt event.Event) bool {
	if !p.visible || !p.enabled {
		return false
	}

	// 从后向前遍历（后添加的组件在上层）
	for i := len(p.children) - 1; i >= 0; i-- {
		child := p.children[i]
		if child.IsVisible() && child.IsEnabled() {
			if child.OnEvent(evt) {
				return true
			}
		}
	}

	return false
}

// SetBounds 设置面板边界（若自身位置发生偏移，同步平移子组件；并触发布局计算）
func (p *Panel) SetBounds(x, y, width, height int32) {
	dx := x - p.bounds.X
	dy := y - p.bounds.Y
	p.BaseComponent.SetBounds(x, y, width, height)
	if (dx != 0 || dy != 0) && len(p.children) > 0 {
		for _, child := range p.children {
			cb := child.GetBounds()
			child.SetBounds(cb.X+dx, cb.Y+dy, cb.Width, cb.Height)
		}
	}
	p.DoLayout()
}

// Add 添加子组件
func (p *Panel) Add(child Component) {
	p.children = append(p.children, child)
	if c, ok := child.(interface{ SetParent(Container) }); ok {
		c.SetParent(p)
	}
	if th, ok := child.(Themeable); ok {
		th.ApplyTheme(theme.Current())
	}
	p.DoLayout()
}

// Remove 移除子组件
func (p *Panel) Remove(child Component) {
	for i, c := range p.children {
		if c == child {
			p.children = append(p.children[:i], p.children[i+1:]...)
			if c, ok := child.(interface{ SetParent(Container) }); ok {
				c.SetParent(nil)
			}
			p.DoLayout()
			return
		}
	}
}

// RemoveAll 移除所有子组件
func (p *Panel) RemoveAll() {
	for _, child := range p.children {
		if c, ok := child.(interface{ SetParent(Container) }); ok {
			c.SetParent(nil)
		}
	}
	p.children = nil
}

// GetChildren 获取所有子组件
func (p *Panel) GetChildren() []Component {
	return p.children
}

// GetChildAt 获取指定位置的子组件
func (p *Panel) GetChildAt(x, y int32) Component {
	// 从后向前遍历（后添加的组件在上层）
	for i := len(p.children) - 1; i >= 0; i-- {
		child := p.children[i]
		if child.IsVisible() {
			bounds := child.GetBounds()
			if IsInBounds(x, y, bounds) {
				return child
			}
		}
	}
	return nil
}

// SetBackground 设置背景颜色
func (p *Panel) SetBackground(color graphics.Color) {
	p.background = color
}

// SetBorder 设置边框
func (p *Panel) SetBorder(color graphics.Color, width int32) {
	p.borderColor = color
	p.borderWidth = width
}

// SetPadding 设置内边距
func (p *Panel) SetPadding(padding int32) {
	p.padding = padding
	p.DoLayout()
}

// SetLayout 设置布局
func (p *Panel) SetLayout(layout Layout) {
	p.layout = layout
	p.DoLayout()
}

// DoLayout 执行布局并递归应用到所有子面板
func (p *Panel) DoLayout() {
	if p.layout != nil && len(p.children) > 0 {
		contentRect := graphics.NewRect(
			p.bounds.X+p.padding,
			p.bounds.Y+p.padding,
			p.bounds.Width-2*p.padding,
			p.bounds.Height-2*p.padding,
		)
		p.layout.Calculate(contentRect, p.children)
	}
	// 递归触发所有子面板的布局计算
	for _, child := range p.children {
		if sub, ok := child.(interface{ DoLayout() }); ok {
			sub.DoLayout()
		}
	}
}

// Layout 布局接口
type Layout interface {
	Calculate(container graphics.Rect, children []Component)
}

// FlowLayout 流式布局
type FlowLayout struct {
	Gap    int32
	HGap   int32
	VGap   int32
	HAlign HAlign
	Wrap   bool
}

// HAlign 水平对齐方式
type HAlign int

const (
	HAlignStart HAlign = iota
	HAlignCenter
	HAlignEnd
)

// Calculate 计算流式布局
func (l *FlowLayout) Calculate(container graphics.Rect, children []Component) {
	if len(children) == 0 {
		return
	}

	hGap := l.HGap
	if hGap == 0 {
		hGap = l.Gap
		if hGap == 0 {
			hGap = 4
		}
	}
	vGap := l.VGap
	if vGap == 0 {
		vGap = l.Gap
		if vGap == 0 {
			vGap = 4
		}
	}

	x := container.X
	y := container.Y
	rowHeight := int32(0)
	rowStart := 0

	for i, child := range children {
		bounds := child.GetBounds()

		// 检查是否需要换行
		if l.Wrap && x+bounds.Width > container.X+container.Width && x > container.X {
			// 对齐当前行
			l.alignRow(children, rowStart, i, container, y, rowHeight, hGap)

			// 换行
			x = container.X
			y += rowHeight + vGap
			rowHeight = 0
			rowStart = i
		}

		// 设置位置
		child.SetBounds(x, y, bounds.Width, bounds.Height)

		x += bounds.Width + hGap
		if bounds.Height > rowHeight {
			rowHeight = bounds.Height
		}
	}

	// 对齐最后一行
	l.alignRow(children, rowStart, len(children), container, y, rowHeight, hGap)
}

// alignRow 对齐一行
func (l *FlowLayout) alignRow(children []Component, start, end int, container graphics.Rect, y, rowHeight int32, hGap int32) {
	if end <= start {
		return
	}

	// 计算行的总宽度
	totalWidth := int32(0)
	for i := start; i < end; i++ {
		bounds := children[i].GetBounds()
		totalWidth += bounds.Width
	}
	if end > start+1 {
		totalWidth += int32(end-start-1) * hGap
	}

	// 计算偏移
	var offsetX int32
	switch l.HAlign {
	case HAlignStart:
		offsetX = 0
	case HAlignCenter:
		offsetX = (container.Width - totalWidth) / 2
	case HAlignEnd:
		offsetX = container.Width - totalWidth
	}

	// 应用偏移
	for i := start; i < end; i++ {
		bounds := children[i].GetBounds()
		children[i].SetBounds(container.X+offsetX, y, bounds.Width, bounds.Height)
		offsetX += bounds.Width + hGap
	}
}

// GridLayout 网格布局
type GridLayout struct {
	Rows int
	Cols int
	Gap  int32
	HGap int32
	VGap int32
}

// Calculate 计算网格布局
func (l *GridLayout) Calculate(container graphics.Rect, children []Component) {
	if len(children) == 0 || l.Cols <= 0 {
		return
	}

	hGap := l.HGap
	if hGap == 0 {
		hGap = l.Gap
		if hGap == 0 {
			hGap = 4
		}
	}
	vGap := l.VGap
	if vGap == 0 {
		vGap = l.Gap
		if vGap == 0 {
			vGap = 4
		}
	}

	cols := l.Cols
	rows := l.Rows
	if rows <= 0 {
		rows = (len(children) + cols - 1) / cols
	}

	cellWidth := (container.Width - int32(cols-1)*hGap) / int32(cols)
	cellHeight := (container.Height - int32(rows-1)*vGap) / int32(rows)

	for i, child := range children {
		col := i % cols
		row := i / cols

		x := container.X + int32(col)*(cellWidth+hGap)
		y := container.Y + int32(row)*(cellHeight+vGap)

		child.SetBounds(x, y, cellWidth, cellHeight)
	}
}

// BorderLayout 边界布局
type BorderLayout struct {
	Top    Component
	Bottom Component
	Left   Component
	Right  Component
	Center Component
	Gap    int32
}

// Calculate 计算边界布局
func (l *BorderLayout) Calculate(container graphics.Rect, children []Component) {
	gap := l.Gap

	top, bottom, left, right, center := l.Top, l.Bottom, l.Left, l.Right, l.Center

	topHeight := int32(0)
	bottomHeight := int32(0)
	leftWidth := int32(0)
	rightWidth := int32(0)

	topGap := int32(0)
	bottomGap := int32(0)
	leftGap := int32(0)
	rightGap := int32(0)

	if top != nil && top.IsVisible() {
		_, topHeight = top.GetBounds().Width, top.GetBounds().Height
		topGap = gap
	}
	if bottom != nil && bottom.IsVisible() {
		_, bottomHeight = bottom.GetBounds().Width, bottom.GetBounds().Height
		bottomGap = gap
	}
	if left != nil && left.IsVisible() {
		leftWidth, _ = left.GetBounds().Width, left.GetBounds().Height
		leftGap = gap
	}
	if right != nil && right.IsVisible() {
		rightWidth, _ = right.GetBounds().Width, right.GetBounds().Height
		rightGap = gap
	}

	// 设置各个区域的位置和大小
	if top != nil && top.IsVisible() {
		top.SetBounds(container.X, container.Y, container.Width, topHeight)
	}
	if bottom != nil && bottom.IsVisible() {
		bottom.SetBounds(container.X, container.Y+container.Height-bottomHeight, container.Width, bottomHeight)
	}
	if left != nil && left.IsVisible() {
		left.SetBounds(container.X, container.Y+topHeight+topGap, leftWidth, container.Height-topHeight-bottomHeight-topGap-bottomGap)
	}
	if right != nil && right.IsVisible() {
		right.SetBounds(container.X+container.Width-rightWidth, container.Y+topHeight+topGap, rightWidth, container.Height-topHeight-bottomHeight-topGap-bottomGap)
	}
	if center != nil && center.IsVisible() {
		center.SetBounds(
			container.X+leftWidth+leftGap,
			container.Y+topHeight+topGap,
			container.Width-leftWidth-rightWidth-leftGap-rightGap,
			container.Height-topHeight-bottomHeight-topGap-bottomGap,
		)
	}
}
