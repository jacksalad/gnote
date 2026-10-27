//go:build windows

package window

import (
	"encoding/binary"
	"errors"
	"math"
	"unsafe"

	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/render"
)

// Icon 窗口图标，封装 Win32 原生图标句柄（小图标 16x16 与大图标 32x32 或更高分辨率）
type Icon struct {
	hIconSmall uintptr
	hIconBig   uintptr
}

// GetSmallHandle 获取小图标句柄 (16x16)
func (i *Icon) GetSmallHandle() uintptr {
	if i == nil {
		return 0
	}
	return i.hIconSmall
}

// GetBigHandle 获取大图标句柄 (32x32 或以上)
func (i *Icon) GetBigHandle() uintptr {
	if i == nil {
		return 0
	}
	return i.hIconBig
}

// Destroy 释放图标占用的系统 GDI 句柄
func (i *Icon) Destroy() {
	if i == nil {
		return
	}
	if i.hIconSmall != 0 {
		DestroyIcon(i.hIconSmall)
		if i.hIconBig == i.hIconSmall {
			i.hIconBig = 0
		}
		i.hIconSmall = 0
	}
	if i.hIconBig != 0 {
		DestroyIcon(i.hIconBig)
		i.hIconBig = 0
	}
}

// CreateHIconFromPixels 从 32 位 ARGB 像素数组创建 Windows 原生 32bpp 带 Alpha 通道的 HICON
func CreateHIconFromPixels(width, height int, pixels []graphics.Color) (uintptr, error) {
	if width <= 0 || height <= 0 || len(pixels) < width*height {
		return 0, errors.New("invalid pixel dimensions or insufficient buffer size")
	}

	hdcScreen := GetDC(0)
	if hdcScreen == 0 {
		return 0, errors.New("failed to acquire screen device context")
	}
	defer ReleaseDC(0, hdcScreen)

	// 创建 32bpp DIBSection（Top-Down 布局，BiHeight 为负值）
	var bmi BITMAPINFO
	bmi.BmiHeader.BiSize = uint32(unsafe.Sizeof(BITMAPINFOHEADER{}))
	bmi.BmiHeader.BiWidth = int32(width)
	bmi.BmiHeader.BiHeight = -int32(height)
	bmi.BmiHeader.BiPlanes = 1
	bmi.BmiHeader.BiBitCount = 32
	bmi.BmiHeader.BiCompression = BI_RGB
	bmi.BmiHeader.BiSizeImage = uint32(width * height * 4)

	var pBits unsafe.Pointer
	hbmColor := CreateDIBSection(hdcScreen, &bmi, DIB_RGB_COLORS, &pBits, 0, 0)
	if hbmColor == 0 || pBits == nil {
		return 0, errors.New("failed to create DIB section for icon color bitmap")
	}
	defer DeleteObject(hbmColor)

	// 拷贝像素（graphics.Color 即 0xAARRGGBB，小端序对应 BGRA 内存分布）
	destSlice := unsafe.Slice((*uint32)(pBits), width*height)
	for idx := 0; idx < width*height; idx++ {
		destSlice[idx] = uint32(pixels[idx])
	}

	// 创建 1bpp 单色掩码位图（按 32 位/4 字节每行对齐）
	rowBytes := ((width + 31) / 32) * 4
	maskBits := make([]byte, rowBytes*height)
	hbmMask := CreateBitmap(int32(width), int32(height), 1, 1, uintptr(unsafe.Pointer(&maskBits[0])))
	if hbmMask == 0 {
		return 0, errors.New("failed to create mask bitmap for icon")
	}
	defer DeleteObject(hbmMask)

	// 组装并创建 HICON
	ii := ICONINFO{
		FIcon:    1, // 1 表示图标
		XHotspot: 0,
		YHotspot: 0,
		HbmMask:  hbmMask,
		HbmColor: hbmColor,
	}
	hIcon := CreateIconIndirect(&ii)
	if hIcon == 0 {
		return 0, errors.New("CreateIconIndirect returned NULL handle")
	}

	return hIcon, nil
}

// NewIconFromRGBA 从纯 Go ARGB 像素点阵创建双尺寸原生图标 (小图标与大图标)
func NewIconFromRGBA(smallW, smallH int, smallPixels []graphics.Color, bigW, bigH int, bigPixels []graphics.Color) (*Icon, error) {
	hSmall, err := CreateHIconFromPixels(smallW, smallH, smallPixels)
	if err != nil {
		return nil, err
	}

	hBig, err := CreateHIconFromPixels(bigW, bigH, bigPixels)
	if err != nil {
		DestroyIcon(hSmall)
		return nil, err
	}

	return &Icon{
		hIconSmall: hSmall,
		hIconBig:   hBig,
	}, nil
}

// NewIconFromICO 从标准 .ico 二进制数据中解析并加载图标
func NewIconFromICO(data []byte) (*Icon, error) {
	if len(data) < 6 {
		return nil, errors.New("invalid ico data: length is too short")
	}
	reserved := binary.LittleEndian.Uint16(data[0:2])
	resType := binary.LittleEndian.Uint16(data[2:4])
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	if reserved != 0 || resType != 1 || count <= 0 {
		return nil, errors.New("invalid ico format: signature mismatch")
	}

	if len(data) < 6+count*16 {
		return nil, errors.New("invalid ico data: corrupted directory header")
	}

	type dirEntry struct {
		w, h   int
		offset uint32
		size   uint32
	}
	entries := make([]dirEntry, 0, count)
	for idx := 0; idx < count; idx++ {
		offset := 6 + idx*16
		w := int(data[offset])
		if w == 0 {
			w = 256
		}
		h := int(data[offset+1])
		if h == 0 {
			h = 256
		}
		size := binary.LittleEndian.Uint32(data[offset+8 : offset+12])
		imgOffset := binary.LittleEndian.Uint32(data[offset+12 : offset+16])
		// 用 uint64 做边界运算：imgOffset+size 在 uint32 下会回绕，
		// 回绕后可能把越界的 data 指针交给 CreateIconFromResourceEx。
		if size == 0 || uint64(imgOffset)+uint64(size) > uint64(len(data)) {
			continue
		}
		entries = append(entries, dirEntry{w: w, h: h, offset: imgOffset, size: size})
	}

	if len(entries) == 0 {
		return nil, errors.New("no valid icon entries found in directory")
	}

	// 选取最接近 16x16 的条目
	bestSmall := entries[0]
	bestSmallDiff := abs(bestSmall.w - 16)
	for _, e := range entries {
		diff := abs(e.w - 16)
		if diff < bestSmallDiff {
			bestSmall = e
			bestSmallDiff = diff
		}
	}

	// 选取最接近 32x32 的条目
	bestBig := entries[0]
	bestBigDiff := abs(bestBig.w - 32)
	for _, e := range entries {
		diff := abs(e.w - 32)
		if diff < bestBigDiff {
			bestBig = e
			bestBigDiff = diff
		}
	}

	hSmall := CreateIconFromResourceEx(
		uintptr(unsafe.Pointer(&data[bestSmall.offset])),
		bestSmall.size,
		1,
		0x00030000,
		16, 16,
		LR_DEFAULTCOLOR,
	)

	hBig := CreateIconFromResourceEx(
		uintptr(unsafe.Pointer(&data[bestBig.offset])),
		bestBig.size,
		1,
		0x00030000,
		32, 32,
		LR_DEFAULTCOLOR,
	)

	if hSmall == 0 && hBig == 0 {
		return nil, errors.New("failed to instantiate Windows icon from resource")
	}
	if hSmall == 0 {
		hSmall = hBig
	}
	if hBig == 0 {
		hBig = hSmall
	}

	return &Icon{
		hIconSmall: hSmall,
		hIconBig:   hBig,
	}, nil
}

// NewIconFromImage 从单个 render.Image 高质量缩放并创建图标
func NewIconFromImage(img *render.Image) (*Icon, error) {
	if img == nil || img.Width <= 0 || img.Height <= 0 {
		return nil, errors.New("nil or empty image provided")
	}

	smallPixels := resamplePixels(img, 16, 16)
	bigPixels := resamplePixels(img, 32, 32)

	return NewIconFromRGBA(16, 16, smallPixels, 32, 32, bigPixels)
}

// NewIconFromImages 从专门提供的小尺寸和大尺寸 Image 创建图标
func NewIconFromImages(imgSmall, imgBig *render.Image) (*Icon, error) {
	if imgSmall == nil || imgBig == nil {
		return nil, errors.New("nil image provided")
	}

	smallPixels := resamplePixels(imgSmall, 16, 16)
	bigPixels := resamplePixels(imgBig, 32, 32)

	return NewIconFromRGBA(16, 16, smallPixels, 32, 32, bigPixels)
}

// resamplePixels 使用双线性采样将图像调整为目标尺寸
func resamplePixels(src *render.Image, dstW, dstH int) []graphics.Color {
	out := make([]graphics.Color, dstW*dstH)
	xRatio := float64(src.Width) / float64(dstW)
	yRatio := float64(src.Height) / float64(dstH)

	for dy := 0; dy < dstH; dy++ {
		srcY := (float64(dy)+0.5)*yRatio - 0.5
		if srcY < 0 {
			srcY = 0
		}
		if srcY >= float64(src.Height-1) {
			srcY = float64(src.Height - 1)
		}
		y0 := int(math.Floor(srcY))
		y1 := y0 + 1
		if y1 >= int(src.Height) {
			y1 = y0
		}
		yf := srcY - float64(y0)

		for dx := 0; dx < dstW; dx++ {
			srcX := (float64(dx)+0.5)*xRatio - 0.5
			if srcX < 0 {
				srcX = 0
			}
			if srcX >= float64(src.Width-1) {
				srcX = float64(src.Width - 1)
			}
			x0 := int(math.Floor(srcX))
			x1 := x0 + 1
			if x1 >= int(src.Width) {
				x1 = x0
			}
			xf := srcX - float64(x0)

			c00 := graphics.Color(src.Pixels[y0*int(src.Width)+x0])
			c10 := graphics.Color(src.Pixels[y0*int(src.Width)+x1])
			c01 := graphics.Color(src.Pixels[y1*int(src.Width)+x0])
			c11 := graphics.Color(src.Pixels[y1*int(src.Width)+x1])

			out[dy*dstW+dx] = bilinearInterpolate(c00, c10, c01, c11, xf, yf)
		}
	}
	return out
}

func bilinearInterpolate(c00, c10, c01, c11 graphics.Color, xf, yf float64) graphics.Color {
	interpolateChannel := func(v00, v10, v01, v11 uint8) uint8 {
		top := float64(v00)*(1.0-xf) + float64(v10)*xf
		bot := float64(v01)*(1.0-xf) + float64(v11)*xf
		val := top*(1.0-yf) + bot*yf
		if val < 0 {
			val = 0
		}
		if val > 255 {
			val = 255
		}
		return uint8(math.Round(val))
	}

	a := interpolateChannel(c00.A(), c10.A(), c01.A(), c11.A())
	r := interpolateChannel(c00.R(), c10.R(), c01.R(), c11.R())
	g := interpolateChannel(c00.G(), c10.G(), c01.G(), c11.G())
	b := interpolateChannel(c00.B(), c10.B(), c01.B(), c11.B())

	return graphics.RGBA(r, g, b, a)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
