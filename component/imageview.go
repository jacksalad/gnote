package component

import (
	"github.com/jacksalad/gnote/event"
	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
	"github.com/jacksalad/gnote/theme"
)

// ImageScaleMode 图像缩放显示模式
type ImageScaleMode int

const (
	// ScaleStretch 拉伸填满组件边界，不保持宽高比
	ScaleStretch ImageScaleMode = iota
	// ScaleFit 等比例缩放，完整显示并居中（若比例不一致则留空）
	ScaleFit
	// ScaleFill 等比例缩放并填满组件边界，裁剪超出部分
	ScaleFill
	// ScaleCenter 保持原图尺寸 1:1 居中显示
	ScaleCenter
)

// ImageView 图像展示控件
type ImageView struct {
	BaseComponent
	image       *render.Image
	scaleMode   ImageScaleMode
	bgColor     graphics.Color
	hasBg       bool
	borderColor graphics.Color
	borderWidth int32
	radius      int32
}

// NewImageView 创建图像控件
func NewImageView(img *render.Image) *ImageView {
	iv := &ImageView{
		BaseComponent: NewBaseComponent(),
		image:         img,
		scaleMode:     ScaleFit,
		bgColor:       0,
		hasBg:         false,
		borderColor:   0,
		borderWidth:   0,
		radius:        0,
	}
	iv.ApplyTheme(theme.Current())
	return iv
}

// ApplyTheme 应用主题样式
func (v *ImageView) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	if v.borderWidth > 0 {
		v.borderColor = t.Border
	}
}

// NewImageViewFromFile 从本地磁盘文件加载并创建图像控件
func NewImageViewFromFile(path string) (*ImageView, error) {
	img, err := render.LoadImageFromFile(path)
	if err != nil {
		return nil, err
	}
	return NewImageView(img), nil
}

// SetImage 设置显示的图像
func (v *ImageView) SetImage(img *render.Image) {
	v.image = img
}

// GetImage 获取当前图像指针
func (v *ImageView) GetImage() *render.Image {
	return v.image
}

// SetScaleMode 设置图像缩放适配模式
func (v *ImageView) SetScaleMode(mode ImageScaleMode) {
	v.scaleMode = mode
}

// SetBackgroundColor 设置图像控件背景底色
func (v *ImageView) SetBackgroundColor(c graphics.Color) {
	v.bgColor = c
	v.hasBg = true
}

// SetBorder 设置边框颜色与粗细
func (v *ImageView) SetBorder(c graphics.Color, width int32) {
	v.borderColor = c
	v.borderWidth = width
}

// SetCornerRadius 设置圆角半径
func (v *ImageView) SetCornerRadius(radius int32) {
	v.radius = radius
}

// OnEvent 处理事件
func (v *ImageView) OnEvent(evt event.Event) bool {
	return false
}

// Render 渲染图像
func (v *ImageView) Render(canvas *render.Canvas) {
	if !v.visible || canvas == nil {
		return
	}

	b := v.bounds
	if b.Width <= 0 || b.Height <= 0 {
		return
	}

	// 1. 绘制背景色
	if v.hasBg {
		if v.radius > 0 {
			canvas.FillRoundedRect(b.X, b.Y, b.Width, b.Height, v.radius, v.bgColor)
		} else {
			canvas.FillRect(b.X, b.Y, b.Width, b.Height, v.bgColor)
		}
	}

	// 2. 绘制图像内容
	if v.image != nil && v.image.Width > 0 && v.image.Height > 0 {
		// 如果有圆角，开启裁剪隔离
		if v.radius > 0 {
			canvas.PushClip(b)
		}

		imgW := float64(v.image.Width)
		imgH := float64(v.image.Height)

		switch v.scaleMode {
		case ScaleStretch:
			canvas.DrawImageScaled(v.image, b)

		case ScaleFit:
			scaleX := float64(b.Width) / imgW
			scaleY := float64(b.Height) / imgH
			scale := scaleX
			if scaleY < scale {
				scale = scaleY
			}
			newW := int32(imgW * scale)
			newH := int32(imgH * scale)
			dstRect := graphics.NewRect(
				b.X+(b.Width-newW)/2,
				b.Y+(b.Height-newH)/2,
				newW,
				newH,
			)
			canvas.DrawImageScaled(v.image, dstRect)

		case ScaleFill:
			scaleX := float64(b.Width) / imgW
			scaleY := float64(b.Height) / imgH
			scale := scaleX
			if scaleY > scale {
				scale = scaleY
			}
			newW := int32(imgW * scale)
			newH := int32(imgH * scale)
			dstRect := graphics.NewRect(
				b.X+(b.Width-newW)/2,
				b.Y+(b.Height-newH)/2,
				newW,
				newH,
			)
			canvas.PushClip(b)
			canvas.DrawImageScaled(v.image, dstRect)
			canvas.PopClip()

		case ScaleCenter:
			dstRect := graphics.NewRect(
				b.X+(b.Width-v.image.Width)/2,
				b.Y+(b.Height-v.image.Height)/2,
				v.image.Width,
				v.image.Height,
			)
			canvas.PushClip(b)
			canvas.DrawImageScaled(v.image, dstRect)
			canvas.PopClip()
		}

		if v.radius > 0 {
			canvas.PopClip()
		}
	}

	// 3. 绘制边框
	if v.borderWidth > 0 {
		if v.radius > 0 {
			canvas.DrawRoundedRect(b.X, b.Y, b.Width, b.Height, v.radius, v.borderColor)
		} else {
			canvas.DrawRect(b.X, b.Y, b.Width, b.Height, v.borderColor)
		}
	}
}
