package render

import (
	"bytes"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"

	"github.com/jacksalad/gnote/graphics"
)

// Image 存储 32 位连续 ARGB 像素的软件位图，兼容 Go 标准库 image.Image
type Image struct {
	Width  int32
	Height int32
	Pixels []uint32 // ARGB 格式: 0xAARRGGBB
}

// NewImage 创建指定宽高的新空白图像（默认全透明黑色 0x00000000）
func NewImage(width, height int32) *Image {
	if width <= 0 || height <= 0 {
		return &Image{Width: 0, Height: 0, Pixels: nil}
	}
	return &Image{
		Width:  width,
		Height: height,
		Pixels: make([]uint32, width*height),
	}
}

// FromGoImage 将 Go 标准库的 image.Image 转换为 *Image
func FromGoImage(src image.Image) *Image {
	if src == nil {
		return nil
	}
	b := src.Bounds()
	w := int32(b.Dx())
	h := int32(b.Dy())
	if w <= 0 || h <= 0 {
		return &Image{Width: 0, Height: 0, Pixels: nil}
	}

	img := NewImage(w, h)
	for y := int32(0); y < h; y++ {
		rowOffset := y * w
		srcY := b.Min.Y + int(y)
		for x := int32(0); x < w; x++ {
			c := src.At(b.Min.X+int(x), srcY)
			r, g, bVal, a := c.RGBA()
			// RGBA() 返回 0..0xffff，右移 8 位转为 0..0xff
			r8 := uint8(r >> 8)
			g8 := uint8(g >> 8)
			b8 := uint8(bVal >> 8)
			a8 := uint8(a >> 8)
			img.Pixels[rowOffset+x] = (uint32(a8) << 24) | (uint32(r8) << 16) | (uint32(g8) << 8) | uint32(b8)
		}
	}
	return img
}

// LoadImageFromFile 从指定路径加载图像文件 (自动支持 PNG, JPEG, GIF)
func LoadImageFromFile(path string) (*Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	src, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}
	return FromGoImage(src), nil
}

// LoadImageFromReader 从 Reader 加载并解码图像
func LoadImageFromReader(r io.Reader) (*Image, error) {
	src, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}
	return FromGoImage(src), nil
}

// LoadImageFromBytes 从字节切片解码图像
func LoadImageFromBytes(data []byte) (*Image, error) {
	return LoadImageFromReader(bytes.NewReader(data))
}

// ColorModel 实现 image.Image 接口
func (img *Image) ColorModel() color.Model {
	return color.NRGBAModel
}

// Bounds 实现 image.Image 接口
func (img *Image) Bounds() image.Rectangle {
	return image.Rect(0, 0, int(img.Width), int(img.Height))
}

// At 实现 image.Image 接口
func (img *Image) At(x, y int) color.Color {
	if x < 0 || x >= int(img.Width) || y < 0 || y >= int(img.Height) {
		return color.Transparent
	}
	px := img.Pixels[int32(y)*img.Width+int32(x)]
	a := uint8(px >> 24)
	r := uint8(px >> 16)
	g := uint8(px >> 8)
	b := uint8(px)
	return color.NRGBA{R: r, G: g, B: b, A: a}
}

// GetPixel 获取指定坐标像素颜色
func (img *Image) GetPixel(x, y int32) graphics.Color {
	if x < 0 || x >= img.Width || y < 0 || y >= img.Height {
		return 0
	}
	return graphics.Color(img.Pixels[y*img.Width+x])
}

// SetPixel 设置指定坐标像素颜色
func (img *Image) SetPixel(x, y int32, c graphics.Color) {
	if x >= 0 && x < img.Width && y >= 0 && y < img.Height {
		img.Pixels[y*img.Width+x] = uint32(c)
	}
}

// SubImage 截取图像子区域
func (img *Image) SubImage(rect graphics.Rect) *Image {
	if rect.Width <= 0 || rect.Height <= 0 {
		return NewImage(0, 0)
	}
	sub := NewImage(rect.Width, rect.Height)
	for y := int32(0); y < rect.Height; y++ {
		sy := rect.Y + y
		if sy < 0 || sy >= img.Height {
			continue
		}
		for x := int32(0); x < rect.Width; x++ {
			sx := rect.X + x
			if sx < 0 || sx >= img.Width {
				continue
			}
			sub.Pixels[y*rect.Width+x] = img.Pixels[sy*img.Width+sx]
		}
	}
	return sub
}
