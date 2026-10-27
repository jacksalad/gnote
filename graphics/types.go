// Package graphics 提供基础图形类型定义
package graphics

// Color 表示ARGB颜色（32位）
type Color uint32

// 预定义颜色
const (
	ColorTransparent Color = 0x00000000
	ColorBlack       Color = 0xFF000000
	ColorWhite       Color = 0xFFFFFFFF
	ColorRed         Color = 0xFFFF0000
	ColorGreen       Color = 0xFF00FF00
	ColorBlue        Color = 0xFF0000FF
	ColorYellow      Color = 0xFFFFFF00
	ColorCyan        Color = 0xFF00FFFF
	ColorMagenta     Color = 0xFFFF00FF
	ColorGray        Color = 0xFF808080
	ColorLightGray   Color = 0xFFC0C0C0
	ColorDarkGray    Color = 0xFF404040
)

// RGBA 创建颜色（0-255范围）
func RGBA(r, g, b, a uint8) Color {
	return Color(uint32(a)<<24 | uint32(r)<<16 | uint32(g)<<8 | uint32(b))
}

// RGB 创建不透明颜色（0-255范围）
func RGB(r, g, b uint8) Color {
	return Color(0xFF000000 | uint32(r)<<16 | uint32(g)<<8 | uint32(b))
}

// A 返回Alpha分量（0-255）
func (c Color) A() uint8 {
	return uint8(c >> 24)
}

// R 返回Red分量（0-255）
func (c Color) R() uint8 {
	return uint8(c >> 16)
}

// G 返回Green分量（0-255）
func (c Color) G() uint8 {
	return uint8(c >> 8)
}

// B 返回Blue分量（0-255）
func (c Color) B() uint8 {
	return uint8(c)
}

// Point 表示二维点
type Point struct {
	X, Y int32
}

// NewPoint 创建新点
func NewPoint(x, y int32) Point {
	return Point{X: x, Y: y}
}

// Size 表示尺寸
type Size struct {
	Width, Height int32
}

// NewSize 创建新尺寸
func NewSize(width, height int32) Size {
	return Size{Width: width, Height: height}
}

// Rect 表示矩形区域
type Rect struct {
	X, Y, Width, Height int32
}

// NewRect 创建新矩形
func NewRect(x, y, width, height int32) Rect {
	return Rect{X: x, Y: y, Width: width, Height: height}
}

// Contains 检查点是否在矩形内
func (r Rect) Contains(p Point) bool {
	return p.X >= r.X && p.X < r.X+r.Width &&
		p.Y >= r.Y && p.Y < r.Y+r.Height
}

// Left 返回左边界
func (r Rect) Left() int32 {
	return r.X
}

// Top 返回上边界
func (r Rect) Top() int32 {
	return r.Y
}

// Right 返回右边界
func (r Rect) Right() int32 {
	return r.X + r.Width
}

// Bottom 返回下边界
func (r Rect) Bottom() int32 {
	return r.Y + r.Height
}

// Intersect 返回两个矩形的交集
func (r Rect) Intersect(other Rect) Rect {
	x := max(r.X, other.X)
	y := max(r.Y, other.Y)
	right := min(r.Right(), other.Right())
	bottom := min(r.Bottom(), other.Bottom())

	if right <= x || bottom <= y {
		return Rect{}
	}
	return NewRect(x, y, right-x, bottom-y)
}

func min(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func max(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
