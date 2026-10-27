// Package component 提供UI组件定义和管理
package component

import (
	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/theme"
)

// Themeable 支持动态应用主题样式的组件接口
type Themeable interface {
	ApplyTheme(t *theme.Theme)
}

// Component 组件接口
type Component interface {
	// Render 渲染组件
	Render(canvas *render.Canvas)
	// OnEvent 处理事件
	OnEvent(evt event.Event) bool
	// SetBounds 设置边界
	SetBounds(x, y, width, height int32)
	// GetBounds 获取边界
	GetBounds() graphics.Rect
	// SetVisible 设置可见性
	SetVisible(visible bool)
	// IsVisible 检查可见性
	IsVisible() bool
	// SetEnabled 设置启用状态
	SetEnabled(enabled bool)
	// IsEnabled 检查启用状态
	IsEnabled() bool
}

// Focusable 表示可接收键盘焦点的组件
type Focusable interface {
	SetFocused(bool)
	IsFocused() bool
}

// RequestFocus 将键盘焦点独占交给 target，并清除整棵组件树中其它可聚焦组件的焦点。
// 避免查找条多个输入框同时持有焦点（双光标、按键串入正文）。
func RequestFocus(target Component) {
	if target == nil {
		return
	}
	clearFocusTree(rootOf(target))
	if f, ok := target.(Focusable); ok {
		f.SetFocused(true)
	}
}

// rootOf 沿父容器链找到组件树根
func rootOf(c Component) Component {
	for {
		holder, ok := c.(interface{ GetParent() Container })
		if !ok {
			return c
		}
		parent := holder.GetParent()
		if parent == nil {
			return c
		}
		c = parent
	}
}

// clearFocusTree 递归清除子树中所有可聚焦组件的焦点
func clearFocusTree(c Component) {
	if c == nil {
		return
	}
	if f, ok := c.(Focusable); ok {
		f.SetFocused(false)
	}
	if cont, ok := c.(Container); ok {
		for _, child := range cont.GetChildren() {
			clearFocusTree(child)
		}
	}
}

// BaseComponent 基础组件实现
type BaseComponent struct {
	bounds  graphics.Rect
	visible bool
	enabled bool
	parent  Container
}

// NewBaseComponent 创建基础组件
func NewBaseComponent() BaseComponent {
	return BaseComponent{
		visible: true,
		enabled: true,
	}
}

// SetBounds 设置边界
func (b *BaseComponent) SetBounds(x, y, width, height int32) {
	b.bounds = graphics.NewRect(x, y, width, height)
}

// GetBounds 获取边界
func (b *BaseComponent) GetBounds() graphics.Rect {
	return b.bounds
}

// SetVisible 设置可见性
func (b *BaseComponent) SetVisible(visible bool) {
	b.visible = visible
}

// IsVisible 检查可见性
func (b *BaseComponent) IsVisible() bool {
	return b.visible
}

// SetEnabled 设置启用状态
func (b *BaseComponent) SetEnabled(enabled bool) {
	b.enabled = enabled
}

// IsEnabled 检查启用状态
func (b *BaseComponent) IsEnabled() bool {
	return b.enabled
}

// SetParent 设置父容器
func (b *BaseComponent) SetParent(parent Container) {
	b.parent = parent
}

// GetParent 获取父容器
func (b *BaseComponent) GetParent() Container {
	return b.parent
}

// Container 容器接口
type Container interface {
	Component
	// Add 添加子组件
	Add(child Component)
	// Remove 移除子组件
	Remove(child Component)
	// RemoveAll 移除所有子组件
	RemoveAll()
	// GetChildren 获取所有子组件
	GetChildren() []Component
	// GetChildAt 获取指定位置的子组件
	GetChildAt(x, y int32) Component
}

// Clickable 可点击接口
type Clickable interface {
	Component
	// OnClick 设置点击回调
	OnClick(callback func())
	// SetHover 设置悬停状态
	SetHover(hover bool)
	// IsHover 检查悬停状态
	IsHover() bool
	// SetPressed 设置按下状态
	SetPressed(pressed bool)
	// IsPressed 检查按下状态
	IsPressed() bool
}

// TextComponent 文本组件接口
type TextComponent interface {
	Component
	// SetText 设置文本
	SetText(text string)
	// GetText 获取文本
	GetText() string
	// SetFont 设置字体
	SetFont(font *Font)
	// GetFont 获取字体
	GetFont() *Font
}

// Font 字体类型别名
type Font = graphics.Font

// NewFont 创建字体
func NewFont(name string, size int32) *Font {
	return graphics.NewFont(name, size)
}

// DefaultFont 默认字体
var DefaultFont = graphics.DefaultFont

// 事件处理辅助函数

// IsMouseEvent 检查是否是鼠标事件
func IsMouseEvent(evt event.Event) bool {
	switch evt.Type {
	case event.EventMouseMove, event.EventMouseDown, event.EventMouseUp,
		event.EventMouseClick, event.EventMouseWheel:
		return true
	}
	return false
}

// IsKeyEvent 检查是否是键盘事件
func IsKeyEvent(evt event.Event) bool {
	switch evt.Type {
	case event.EventKeyDown, event.EventKeyUp, event.EventKeyPress:
		return true
	}
	return false
}

// GetMousePosition 从事件获取鼠标位置
func GetMousePosition(evt event.Event) (x, y int32, ok bool) {
	if data, ok := evt.Data.(event.MouseData); ok {
		return data.X, data.Y, true
	}
	return 0, 0, false
}

// IsInBounds 检查点是否在组件边界内
func IsInBounds(x, y int32, bounds graphics.Rect) bool {
	return x >= bounds.X && x < bounds.X+bounds.Width &&
		y >= bounds.Y && y < bounds.Y+bounds.Height
}
