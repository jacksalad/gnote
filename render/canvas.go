// Package render 提供软件渲染引擎实现
package render

import (
	"math"
	"sync"

	"github.com/jacksalad/gnote/graphics"
)

// Canvas 画布，包含像素缓冲区与裁剪视口栈
type Canvas struct {
	Width, Height int32
	Buffer        []uint32 // ARGB格式像素缓冲
	dirty         bool
	clipStack     []graphics.Rect
}

// NewCanvas 创建新画布
func NewCanvas(width, height int32) *Canvas {
	c := &Canvas{
		Width:  width,
		Height: height,
		Buffer: make([]uint32, width*height),
		dirty:  true,
	}
	c.clipStack = []graphics.Rect{graphics.NewRect(0, 0, width, height)}
	return c
}

// Resize 调整画布大小（复用已有像素缓冲区容量，避免拖拽窗口时反复分配数 MB）
func (c *Canvas) Resize(width, height int32) {
	if width == c.Width && height == c.Height {
		return
	}
	c.Width = width
	c.Height = height
	needed := int(width) * int(height)
	if needed < 0 {
		needed = 0
	}
	if cap(c.Buffer) < needed {
		c.Buffer = make([]uint32, needed)
	} else {
		c.Buffer = c.Buffer[:needed]
	}
	c.dirty = true
	if cap(c.clipStack) > 0 {
		c.clipStack = c.clipStack[:0]
	}
	c.clipStack = append(c.clipStack, graphics.NewRect(0, 0, width, height))
}

// AdoptBuffer 使用外部像素内存作为画布后备缓冲（宽度即行跨距），用于直接渲染到 DIB Section
func (c *Canvas) AdoptBuffer(buffer []uint32, width, height int32) {
	if width <= 0 || height <= 0 || len(buffer) < int(width)*int(height) {
		return
	}
	c.Buffer = buffer[:int(width)*int(height)]
	c.Width = width
	c.Height = height
	c.dirty = true
	c.clipStack = c.clipStack[:0]
	c.clipStack = append(c.clipStack, graphics.NewRect(0, 0, width, height))
}

// PushClip 压入新的裁剪矩形，实际生效裁剪区为与当前区域的交集
func (c *Canvas) PushClip(rect graphics.Rect) {
	cur := c.Clip()
	newClip := cur.Intersect(rect)
	c.clipStack = append(c.clipStack, newClip)
}

// PopClip 弹出当前裁剪矩形
func (c *Canvas) PopClip() {
	if len(c.clipStack) > 1 {
		c.clipStack = c.clipStack[:len(c.clipStack)-1]
	}
}

// Clip 返回当前生效的裁剪矩形
func (c *Canvas) Clip() graphics.Rect {
	if len(c.clipStack) == 0 {
		return graphics.NewRect(0, 0, c.Width, c.Height)
	}
	return c.clipStack[len(c.clipStack)-1]
}

// Clear 清空画布为指定颜色（受当前裁剪区约束）
func (c *Canvas) Clear(color graphics.Color) {
	c.FillRect(0, 0, c.Width, c.Height, color)
}

// FillRect 填充矩形（自动受裁剪栈约束）
func (c *Canvas) FillRect(x, y, width, height int32, color graphics.Color) {
	clip := c.Clip()
	target := graphics.NewRect(x, y, width, height)
	clipped := target.Intersect(clip)
	if clipped.Width <= 0 || clipped.Height <= 0 {
		return
	}

	minY := clipped.Y
	if minY < 0 {
		minY = 0
	}
	maxY := clipped.Y + clipped.Height
	if maxY > c.Height {
		maxY = c.Height
	}
	minX := clipped.X
	if minX < 0 {
		minX = 0
	}
	maxX := clipped.X + clipped.Width
	if maxX > c.Width {
		maxX = c.Width
	}
	if minX >= maxX || minY >= maxY {
		return
	}

	alpha := color.A()
	if alpha == 0 {
		return
	}

	c.dirty = true

	// 完全不透明快速路径：每行用 memmove 倍增填充，避免逐像素存储循环
	if alpha == 255 {
		col := uint32(color)
		for row := minY; row < maxY; row++ {
			offset := row * c.Width
			fillRow(c.Buffer[offset+minX:offset+maxX], col)
		}
		return
	}

	// 半透明 Alpha 混合路径（定点整数混合，消除颜色失真）
	srcR, srcG, srcB := uint32(color.R()), uint32(color.G()), uint32(color.B())
	a := uint32(alpha)
	invA := 255 - a

	for row := minY; row < maxY; row++ {
		offset := row * c.Width
		for colIdx := minX; colIdx < maxX; colIdx++ {
			idx := offset + colIdx
			dst := c.Buffer[idx]
			dstR := (dst >> 16) & 0xFF
			dstG := (dst >> 8) & 0xFF
			dstB := dst & 0xFF

			outR := (srcR*a + dstR*invA + 127) / 255
			outG := (srcG*a + dstG*invA + 127) / 255
			outB := (srcB*a + dstB*invA + 127) / 255

			c.Buffer[idx] = (0xFF << 24) | (outR << 16) | (outG << 8) | outB
		}
	}
}

// DrawRect 绘制矩形边框
func (c *Canvas) DrawRect(x, y, width, height int32, color graphics.Color) {
	if width <= 0 || height <= 0 {
		return
	}
	c.DrawHorizontalLine(x, x+width-1, y, color)
	if height > 1 {
		c.DrawHorizontalLine(x, x+width-1, y+height-1, color)
	}
	c.DrawVerticalLine(x, y, y+height-1, color)
	if width > 1 {
		c.DrawVerticalLine(x+width-1, y, y+height-1, color)
	}
}

// FillRoundedRect 填充圆角矩形（解析几何亚像素距离场平滑抗锯齿）
func (c *Canvas) FillRoundedRect(x, y, width, height, radius int32, color graphics.Color) {
	if width <= 0 || height <= 0 {
		return
	}
	if radius*2 > width {
		radius = width / 2
	}
	if radius*2 > height {
		radius = height / 2
	}
	if radius <= 0 {
		c.FillRect(x, y, width, height, color)
		return
	}

	baseA := color.A()
	if baseA == 0 {
		return
	}

	// 1. 中间主体矩形（极速填充，高度为 height - 2*radius）
	if height-2*radius > 0 {
		c.FillRect(x, y+radius, width, height-2*radius, color)
	}

	// 2. 顶部和底部的中间矩形段（极速填充，宽度为 width - 2*radius）
	if width-2*radius > 0 {
		c.FillRect(x+radius, y, width-2*radius, radius, color)
		c.FillRect(x+radius, y+height-radius, width-2*radius, radius, color)
	}

	// 3. 渲染四个圆角区域（复用按半径缓存的抗锯齿覆盖率，避免每帧重复 Hypot）
	mask := getCornerCoverage(radius)
	corners := [4]struct {
		cx, cy           int32
		mirrorX, mirrorY bool
	}{
		{x, y, false, false},
		{x + width - radius, y, true, false},
		{x, y + height - radius, false, true},
		{x + width - radius, y + height - radius, true, true},
	}

	for _, corn := range corners {
		for py := int32(0); py < radius; py++ {
			my := py
			if corn.mirrorY {
				my = radius - 1 - py
			}
			for px := int32(0); px < radius; px++ {
				mx := px
				if corn.mirrorX {
					mx = radius - 1 - px
				}
				cov := mask.fill[my*radius+mx]
				if cov <= 0 {
					continue
				}
				alpha := uint8(float64(baseA) * float64(cov))
				if alpha > 0 {
					c.BlendPixel(corn.cx+px, corn.cy+py, graphics.RGBA(color.R(), color.G(), color.B(), alpha))
				}
			}
		}
	}
}

// DrawRoundedRect 绘制圆角矩形边框（解析几何亚像素平滑抗锯齿）
func (c *Canvas) DrawRoundedRect(x, y, width, height, radius int32, color graphics.Color) {
	if width <= 0 || height <= 0 {
		return
	}
	if radius*2 > width {
		radius = width / 2
	}
	if radius*2 > height {
		radius = height / 2
	}
	if radius <= 0 {
		c.DrawRect(x, y, width, height, color)
		return
	}

	baseA := color.A()
	if baseA == 0 {
		return
	}

	// 1. 直线段部分
	if width-2*radius > 0 {
		c.DrawHorizontalLine(x+radius, x+width-radius-1, y, color)
		c.DrawHorizontalLine(x+radius, x+width-radius-1, y+height-1, color)
	}
	if height-2*radius > 0 {
		c.DrawVerticalLine(x, y+radius, y+height-radius-1, color)
		c.DrawVerticalLine(x+width-1, y+radius, y+height-radius-1, color)
	}

	// 2. 四个角落平滑抗锯齿弧线（复用按半径缓存的覆盖率）
	mask := getCornerCoverage(radius)
	corners := [4]struct {
		cx, cy           int32
		mirrorX, mirrorY bool
	}{
		{x, y, false, false},
		{x + width - radius, y, true, false},
		{x, y + height - radius, false, true},
		{x + width - radius, y + height - radius, true, true},
	}

	for _, corn := range corners {
		for py := int32(0); py < radius; py++ {
			my := py
			if corn.mirrorY {
				my = radius - 1 - py
			}
			for px := int32(0); px < radius; px++ {
				mx := px
				if corn.mirrorX {
					mx = radius - 1 - px
				}
				cov := mask.ring[my*radius+mx]
				if cov > 0 {
					alpha := uint8(float64(baseA) * float64(cov))
					if alpha > 0 {
						c.BlendPixel(corn.cx+px, corn.cy+py, graphics.RGBA(color.R(), color.G(), color.B(), alpha))
					}
				}
			}
		}
	}
}

// SetPixel 设置单个像素（受 Clip 裁剪与画布物理边界保护）
func (c *Canvas) SetPixel(x, y int32, color graphics.Color) {
	clip := c.Clip()
	if x < clip.X || x >= clip.X+clip.Width || y < clip.Y || y >= clip.Y+clip.Height {
		return
	}
	if x < 0 || x >= c.Width || y < 0 || y >= c.Height {
		return
	}
	c.dirty = true
	c.Buffer[y*c.Width+x] = uint32(color)
}

// GetPixel 获取单个像素
func (c *Canvas) GetPixel(x, y int32) graphics.Color {
	if x < 0 || x >= c.Width || y < 0 || y >= c.Height {
		return 0
	}
	return graphics.Color(c.Buffer[y*c.Width+x])
}

// DrawLine 绘制直线（整数 Bresenham 算法，支持 Clip 裁剪与边界保护）
func (c *Canvas) DrawLine(x1, y1, x2, y2 int32, color graphics.Color) {
	c.dirty = true
	col := uint32(color)
	clip := c.Clip()

	dx := abs(x2 - x1)
	dy := -abs(y2 - y1)
	var sx, sy int32 = 1, 1
	if x1 >= x2 {
		sx = -1
	}
	if y1 >= y2 {
		sy = -1
	}
	err := dx + dy

	maxSteps := abs(x2-x1) + abs(y2-y1) + 4
	steps := int32(0)

	x, y := x1, y1
	for steps < maxSteps {
		steps++
		if x >= clip.X && x < clip.X+clip.Width && y >= clip.Y && y < clip.Y+clip.Height {
			if x >= 0 && x < c.Width && y >= 0 && y < c.Height {
				c.Buffer[y*c.Width+x] = col
			}
		}
		if x == x2 && y == y2 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x += sx
		}
		if e2 <= dx {
			err += dx
			y += sy
		}
	}
}

// DrawCircle 绘制 1px 圆环描边（覆盖率抗锯齿，受 Clip 裁剪）。
//
// 以 radius 为中心线、向内向外各延伸 0.5px；每个像素的覆盖率由 8x8 子采样
// 精确求得并按半径缓存，因此小尺寸单选框的细描边也是连续平滑的圆弧，
// 不会像中点圆算法那样出现断续的单像素颗粒。
func (c *Canvas) DrawCircle(cx, cy, radius int32, color graphics.Color) {
	if radius <= 0 {
		return
	}

	baseA := color.A()
	if baseA == 0 {
		return
	}

	mask := getCircleCoverage(radius)
	if mask == nil {
		return
	}

	n := int(radius*2 + 1)
	left := cx - radius
	top := cy - radius
	srcR, srcG, srcB := color.R(), color.G(), color.B()

	for row := range n {
		ringRow := mask.ring[row*n : (row+1)*n]
		for col := range n {
			cov := ringRow[col]
			if cov <= 0 {
				continue
			}
			alpha := uint8(float64(baseA)*float64(cov) + 0.5)
			if alpha == 0 {
				continue
			}
			c.BlendPixel(left+int32(col), top+int32(row), graphics.RGBA(srcR, srcG, srcB, alpha))
		}
	}
}

// FillCircle 填充圆（覆盖率抗锯齿，受 Clip 裁剪）。
//
// 覆盖率由 8x8 子采样按半径缓存；完全覆盖的连续像素段走矩形快速填充，
// 只有边界像素做 alpha 混合。开关旋钮、滑动条 thumb 等小半径圆的轮廓
// 因此平滑连续，不再出现阶梯状锯齿颗粒。
func (c *Canvas) FillCircle(cx, cy, radius int32, color graphics.Color) {
	if radius <= 0 {
		return
	}

	baseA := color.A()
	if baseA == 0 {
		return
	}

	mask := getCircleCoverage(radius)
	if mask == nil {
		return
	}

	n := int(radius*2 + 1)
	left := cx - radius
	top := cy - radius
	srcR, srcG, srcB := color.R(), color.G(), color.B()

	for row := range n {
		fillRow := mask.fill[row*n : (row+1)*n]

		// 1. 完全覆盖的连续像素段：整行矩形快速填充
		fullStart, fullEnd := -1, -1
		for col := range n {
			if fillRow[col] >= 1 {
				if fullStart < 0 {
					fullStart = col
				}
				fullEnd = col
			}
		}
		if fullStart >= 0 {
			c.FillRect(left+int32(fullStart), top+int32(row), int32(fullEnd-fullStart+1), 1, color)
		}

		// 2. 边界过渡像素：按覆盖率混合
		for col := range n {
			cov := fillRow[col]
			if cov <= 0 || cov >= 1 {
				continue
			}
			alpha := uint8(float64(baseA)*float64(cov) + 0.5)
			if alpha == 0 {
				continue
			}
			c.BlendPixel(left+int32(col), top+int32(row), graphics.RGBA(srcR, srcG, srcB, alpha))
		}
	}
}

// IsDirty 检查画布是否被修改
func (c *Canvas) IsDirty() bool {
	return c.dirty
}

// SetDirty 设置脏标记
func (c *Canvas) SetDirty(dirty bool) {
	c.dirty = dirty
}

// DrawHorizontalLine 绘制水平线（受 Clip 裁剪与物理边界保护极速版本）
func (c *Canvas) DrawHorizontalLine(x1, x2, y int32, color graphics.Color) {
	clip := c.Clip()
	if y < clip.Y || y >= clip.Y+clip.Height || y < 0 || y >= c.Height {
		return
	}
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if x1 < clip.X {
		x1 = clip.X
	}
	if x1 < 0 {
		x1 = 0
	}
	if x2 >= clip.X+clip.Width {
		x2 = clip.X + clip.Width - 1
	}
	if x2 >= c.Width {
		x2 = c.Width - 1
	}
	if x1 > x2 {
		return
	}

	c.dirty = true
	col := uint32(color)
	offset := y * c.Width
	fillRow(c.Buffer[offset+x1:offset+x2+1], col)
}

// fillRow 以 memmove 倍增方式填充一行像素（运行时按 word 宽度拷贝，远快于逐像素赋值）。
func fillRow(dst []uint32, col uint32) {
	if len(dst) == 0 {
		return
	}
	dst[0] = col
	for n := 1; n < len(dst); n <<= 1 {
		copy(dst[n:], dst[:n])
	}
}

// DrawVerticalLine 绘制垂直线（受 Clip 裁剪与物理边界保护极速版本）
func (c *Canvas) DrawVerticalLine(x, y1, y2 int32, color graphics.Color) {
	clip := c.Clip()
	if x < clip.X || x >= clip.X+clip.Width || x < 0 || x >= c.Width {
		return
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	if y1 < clip.Y {
		y1 = clip.Y
	}
	if y1 < 0 {
		y1 = 0
	}
	if y2 >= clip.Y+clip.Height {
		y2 = clip.Y + clip.Height - 1
	}
	if y2 >= c.Height {
		y2 = c.Height - 1
	}
	if y1 > y2 {
		return
	}

	c.dirty = true
	col := uint32(color)
	for y := y1; y <= y2; y++ {
		c.Buffer[y*c.Width+x] = col
	}
}

// BlendPixel 混合像素（支持透明度，受 Clip 裁剪与物理边界保护）
func (c *Canvas) BlendPixel(x, y int32, color graphics.Color) {
	clip := c.Clip()
	if x < clip.X || x >= clip.X+clip.Width || y < clip.Y || y >= clip.Y+clip.Height {
		return
	}
	if x < 0 || x >= c.Width || y < 0 || y >= c.Height {
		return
	}

	alpha := color.A()
	if alpha == 0 {
		return
	}
	if alpha == 255 {
		c.Buffer[y*c.Width+x] = uint32(color)
		c.dirty = true
		return
	}

	c.dirty = true
	idx := y*c.Width + x
	dst := c.Buffer[idx]

	// 定点整数Alpha混合（消除浮点开销与色阶失真）
	srcR, srcG, srcB := uint32(color.R()), uint32(color.G()), uint32(color.B())
	dstR, dstG, dstB := (dst>>16)&0xFF, (dst>>8)&0xFF, dst&0xFF

	a := uint32(alpha)
	invA := 255 - a

	outR := (srcR*a + dstR*invA + 127) / 255
	outG := (srcG*a + dstG*invA + 127) / 255
	outB := (srcB*a + dstB*invA + 127) / 255

	c.Buffer[idx] = (0xFF << 24) | (outR << 16) | (outG << 8) | outB
}

// CopyFrom 从另一个画布复制内容（受目标 Clip 裁剪与物理边界保护）
func (c *Canvas) CopyFrom(src *Canvas, srcX, srcY, srcWidth, srcHeight, dstX, dstY int32) {
	if src == nil {
		return
	}
	c.dirty = true
	clip := c.Clip()

	for y := int32(0); y < srcHeight; y++ {
		sy := srcY + y
		dy := dstY + y
		if sy < 0 || sy >= src.Height || dy < clip.Y || dy >= clip.Y+clip.Height || dy < 0 || dy >= c.Height {
			continue
		}
		for x := int32(0); x < srcWidth; x++ {
			sx := srcX + x
			dx := dstX + x
			if sx < 0 || sx >= src.Width || dx < clip.X || dx >= clip.X+clip.Width || dx < 0 || dx >= c.Width {
				continue
			}
			c.Buffer[dy*c.Width+dx] = src.Buffer[sy*src.Width+sx]
		}
	}
}

func abs(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

// DrawImage 在目标坐标绘制完整图像（1:1 原图，受 Clip 裁剪并支持 Alpha 混合）
func (c *Canvas) DrawImage(img *Image, dstX, dstY int32) {
	if img == nil || img.Width <= 0 || img.Height <= 0 {
		return
	}
	c.DrawImageClip(img, graphics.NewRect(0, 0, img.Width, img.Height), graphics.NewRect(dstX, dstY, img.Width, img.Height))
}

// DrawImageScaled 将完整图像缩放绘制到目标矩形区域（受 Clip 裁剪并支持 Alpha 混合）
func (c *Canvas) DrawImageScaled(img *Image, dstRect graphics.Rect) {
	if img == nil || img.Width <= 0 || img.Height <= 0 {
		return
	}
	c.DrawImageClip(img, graphics.NewRect(0, 0, img.Width, img.Height), dstRect)
}

// DrawImageClip 将源图像的指定切片区域缩放到目标矩形区域（受 Clip 裁剪并支持 Alpha 混合）
func (c *Canvas) DrawImageClip(img *Image, srcRect graphics.Rect, dstRect graphics.Rect) {
	if img == nil || img.Width <= 0 || img.Height <= 0 || dstRect.Width <= 0 || dstRect.Height <= 0 || srcRect.Width <= 0 || srcRect.Height <= 0 {
		return
	}

	clip := c.Clip()
	x1 := dstRect.X
	y1 := dstRect.Y
	x2 := dstRect.X + dstRect.Width
	y2 := dstRect.Y + dstRect.Height

	// 与视口 Clip 求交
	if x1 < clip.X {
		x1 = clip.X
	}
	if y1 < clip.Y {
		y1 = clip.Y
	}
	if x2 > clip.X+clip.Width {
		x2 = clip.X + clip.Width
	}
	if y2 > clip.Y+clip.Height {
		y2 = clip.Y + clip.Height
	}
	// 与画布物理尺寸边界求交
	if x1 < 0 {
		x1 = 0
	}
	if y1 < 0 {
		y1 = 0
	}
	if x2 > c.Width {
		x2 = c.Width
	}
	if y2 > c.Height {
		y2 = c.Height
	}
	if x1 >= x2 || y1 >= y2 {
		return
	}

	c.dirty = true

	for dy := y1; dy < y2; dy++ {
		sy := srcRect.Y + (dy-dstRect.Y)*srcRect.Height/dstRect.Height
		if sy < 0 || sy >= img.Height {
			continue
		}
		imgRowOffset := sy * img.Width
		dstRowOffset := dy * c.Width

		for dx := x1; dx < x2; dx++ {
			sx := srcRect.X + (dx-dstRect.X)*srcRect.Width/dstRect.Width
			if sx < 0 || sx >= img.Width {
				continue
			}

			px := img.Pixels[imgRowOffset+sx]
			alpha := uint8(px >> 24)
			if alpha == 0 {
				continue
			} else if alpha == 255 {
				c.Buffer[dstRowOffset+dx] = px
			} else {
				c.BlendPixel(dx, dy, graphics.Color(px))
			}
		}
	}
}

// cornerCoverage 单个圆角象限的覆盖率掩码（覆盖区间 0..1）
type cornerCoverage struct {
	fill []float32 // 实心圆角覆盖率
	ring []float32 // 1px 圆角描边覆盖率
}

var (
	cornerCoverageMu    sync.Mutex
	cornerCoverageCache = map[int32]*cornerCoverage{}
)

// getCornerCoverage 获取（或构建）指定半径的覆盖率掩码。
// 掩码对四个角共用，通过镜像索引复用；半径过大时不缓存，避免占用过多内存。
func getCornerCoverage(radius int32) *cornerCoverage {
	cornerCoverageMu.Lock()
	if m, ok := cornerCoverageCache[radius]; ok {
		cornerCoverageMu.Unlock()
		return m
	}
	cornerCoverageMu.Unlock()

	m := buildCornerCoverage(radius)
	if radius <= 128 {
		cornerCoverageMu.Lock()
		if len(cornerCoverageCache) >= 64 {
			cornerCoverageCache = map[int32]*cornerCoverage{}
		}
		cornerCoverageCache[radius] = m
		cornerCoverageMu.Unlock()
	}
	return m
}

func buildCornerCoverage(radius int32) *cornerCoverage {
	r := float64(radius)
	m := &cornerCoverage{
		fill: make([]float32, radius*radius),
		ring: make([]float32, radius*radius),
	}
	for py := int32(0); py < radius; py++ {
		dy := float64(py) - (r - 0.5)
		for px := int32(0); px < radius; px++ {
			dx := float64(px) - (r - 0.5)
			dist := math.Hypot(dx, dy)
			switch {
			case dist <= r-0.5:
				m.fill[py*radius+px] = 1
			case dist < r+0.5:
				m.fill[py*radius+px] = float32(r + 0.5 - dist)
			}
			if delta := math.Abs(dist - (r - 0.5)); delta <= 0.5 {
				m.ring[py*radius+px] = float32(1 - delta/0.5)
			}
		}
	}
	return m
}

// circleCoverage 完整圆形（直径 2r+1 像素）的覆盖率掩码，索引 (py*(2r+1)+px)
type circleCoverage struct {
	fill []float32 // 实心圆覆盖率
	ring []float32 // 1px 圆环描边覆盖率（中心线为 radius）
}

// circleCoverageSamples 每轴子采样数，覆盖率精度为 1/(samples*samples)
const circleCoverageSamples = 8

var (
	circleCoverageMu    sync.Mutex
	circleCoverageCache = map[int32]*circleCoverage{}
)

// getCircleCoverage 获取（或构建）指定半径的圆形覆盖率掩码。
// 与圆角掩码共用"按半径缓存"的策略；半径过大时只构建不缓存，避免占用过多内存。
func getCircleCoverage(radius int32) *circleCoverage {
	circleCoverageMu.Lock()
	if m, ok := circleCoverageCache[radius]; ok {
		circleCoverageMu.Unlock()
		return m
	}
	circleCoverageMu.Unlock()

	m := buildCircleCoverage(radius)
	if radius <= 128 {
		circleCoverageMu.Lock()
		if len(circleCoverageCache) >= 64 {
			circleCoverageCache = map[int32]*circleCoverage{}
		}
		circleCoverageCache[radius] = m
		circleCoverageMu.Unlock()
	}
	return m
}

func buildCircleCoverage(radius int32) *circleCoverage {
	n := int(radius*2 + 1)
	m := &circleCoverage{
		fill: make([]float32, n*n),
		ring: make([]float32, n*n),
	}
	r := float64(radius)
	step := 1.0 / circleCoverageSamples
	inv := 1.0 / float64(circleCoverageSamples*circleCoverageSamples)

	for py := range n {
		// 像素中心相对圆心的偏移；子采样点从 -0.5 起均匀铺满该像素
		baseY := float64(py) - r - 0.5
		for px := range n {
			baseX := float64(px) - r - 0.5

			// 快速剔除/填充：像素中心离边界太远时整格只有 0 或 1
			centerDist := math.Hypot(float64(px)-r, float64(py)-r)
			idx := py*n + px
			if centerDist > r+1.25 {
				continue
			}
			if centerDist <= r-1.25 {
				m.fill[idx] = 1
				continue
			}

			fillHits, ringHits := 0, 0
			for sy := range circleCoverageSamples {
				y := baseY + (float64(sy)+0.5)*step
				for sx := range circleCoverageSamples {
					x := baseX + (float64(sx)+0.5)*step
					dist := math.Hypot(x, y)
					if dist <= r {
						fillHits++
					}
					if dist >= r-0.5 && dist <= r+0.5 {
						ringHits++
					}
				}
			}
			m.fill[idx] = float32(fillHits) * float32(inv)
			m.ring[idx] = float32(ringHits) * float32(inv)
		}
	}
	return m
}
