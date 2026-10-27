//go:build windows

package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"unsafe"

	goui "github.com/jacksalad/gnote"
	"github.com/jacksalad/gnote/graphics"
)

// GetGnoteIcon 生成并返回纯 Go 绘制的 gnote 专属现代极客应用图标 (16x16 标题栏 + 32x32 任务栏)
func GetGnoteIcon() *goui.Icon {
	smallPixels := generate16x16Icon()
	bigPixels := generate32x32Icon()

	icon, err := goui.NewIconFromRGBA(16, 16, smallPixels, 32, 32, bigPixels)
	if err != nil {
		return nil
	}

	// 确保本地存在标准 .ico 资产文件，方便用户部署或 PE 编译
	icoTarget := "icon.ico"
	if _, err := os.Stat("cmd/gnote/icon.ico"); err == nil {
		icoTarget = "cmd/gnote/icon.ico"
	}
	_ = EnsureGnoteICOFile(icoTarget)

	return icon
}

// GetNotepadIcon 兼容别名
func GetNotepadIcon() *goui.Icon {
	return GetGnoteIcon()
}

// EnsureGnoteICOFile 检查并在指定路径生成高质量多尺寸标准 .ico 图标文件
func EnsureGnoteICOFile(targetPath string) error {
	if _, err := os.Stat(targetPath); err == nil {
		return nil // 已存在，无需重复生成
	}

	icoData, err := GenerateGnoteICOBytes()
	if err != nil {
		return err
	}

	return os.WriteFile(targetPath, icoData, 0644)
}

// EnsureNotepadICOFile 兼容别名
func EnsureNotepadICOFile(targetPath string) error {
	return EnsureGnoteICOFile(targetPath)
}

// GenerateGnoteICOBytes 将内置设计的 16x16 与 32x32 矢量光栅像素打包为标准 Windows .ico 二进制数据
func GenerateGnoteICOBytes() ([]byte, error) {
	smallPixels := generate16x16Icon()
	bigPixels := generate32x32Icon()

	type layerData struct {
		w, h    int
		xorData []byte
		andData []byte
	}

	buildLayer := func(w, h int, pixels []graphics.Color) layerData {
		xorSize := w * h * 4
		andRowBytes := ((w + 31) / 32) * 4
		andSize := andRowBytes * h

		xor := make([]byte, xorSize)
		// DIB 像素数据自底向上排列 (Bottom-Up)
		for y := 0; y < h; y++ {
			srcY := h - 1 - y
			for x := 0; x < w; x++ {
				c := pixels[srcY*w+x]
				idx := (y*w + x) * 4
				xor[idx+0] = c.B()
				xor[idx+1] = c.G()
				xor[idx+2] = c.R()
				xor[idx+3] = c.A()
			}
		}

		andMask := make([]byte, andSize)
		return layerData{
			w:       w,
			h:       h,
			xorData: xor,
			andData: andMask,
		}
	}

	layers := []layerData{
		buildLayer(16, 16, smallPixels),
		buildLayer(32, 32, bigPixels),
	}

	buf := new(bytes.Buffer)

	// 1. ICONDIR 头 (6 字节)
	binary.Write(buf, binary.LittleEndian, uint16(0))           // Reserved = 0
	binary.Write(buf, binary.LittleEndian, uint16(1))           // Type = 1 (ICO)
	binary.Write(buf, binary.LittleEndian, uint16(len(layers))) // 图标数量

	// 2. ICONDIRENTRY 目录列表
	headerAndDirSize := 6 + len(layers)*16
	currentOffset := uint32(headerAndDirSize)

	for _, l := range layers {
		imgSize := uint32(40 + len(l.xorData) + len(l.andData)) // BITMAPINFOHEADER(40) + XOR + AND
		buf.WriteByte(byte(l.w))
		buf.WriteByte(byte(l.h))
		buf.WriteByte(0)                                   // Color count = 0 (>= 8bpp)
		buf.WriteByte(0)                                   // Reserved
		binary.Write(buf, binary.LittleEndian, uint16(1))  // Color planes
		binary.Write(buf, binary.LittleEndian, uint16(32)) // Bit count = 32bpp
		binary.Write(buf, binary.LittleEndian, imgSize)
		binary.Write(buf, binary.LittleEndian, currentOffset)
		currentOffset += imgSize
	}

	// 3. 各层图像数据体 (BITMAPINFOHEADER + XOR + AND)
	for _, l := range layers {
		bmi := struct {
			BiSize          uint32
			BiWidth         int32
			BiHeight        int32 // 高度必须为 2 倍实际高度
			BiPlanes        uint16
			BiBitCount      uint16
			BiCompression   uint32
			BiSizeImage     uint32
			BiXPelsPerMeter int32
			BiYPelsPerMeter int32
			BiClrUsed       uint32
			BiClrImportant  uint32
		}{
			BiSize:        40,
			BiWidth:       int32(l.w),
			BiHeight:      int32(l.h * 2),
			BiPlanes:      1,
			BiBitCount:    32,
			BiCompression: 0,
			BiSizeImage:   uint32(len(l.xorData) + len(l.andData)),
		}
		bmiBytes := unsafe.Slice((*byte)(unsafe.Pointer(&bmi)), 40)
		buf.Write(bmiBytes)
		buf.Write(l.xorData)
		buf.Write(l.andData)
	}

	return buf.Bytes(), nil
}

// generate16x16Icon 绘制极简、清爽且像素级锐利的 16x16 专属小图标
func generate16x16Icon() []graphics.Color {
	w, h := 16, 16
	pixels := make([]graphics.Color, w*h)

	bgDark := graphics.RGB(15, 23, 42)           // #0F172A 现代极客深蓝底板
	bgBorder := graphics.RGBA(59, 130, 246, 220) // #3B82F6 科技蓝微边框
	pageWhite := graphics.RGB(255, 255, 255)     // #FFFFFF 纯净白文档卡片
	cCode := graphics.RGB(2, 132, 199)           // #0284C7 极简统一科技天蓝行
	cCaret := graphics.RGB(56, 189, 248)         // #38BDF8 终端光标亮蓝

	set := func(x, y int, c graphics.Color) {
		if x >= 0 && x < w && y >= 0 && y < h {
			pixels[y*w+x] = c
		}
	}

	// 1. 底板圆角矩形: (1,1) 到 (14,14)
	for y := 1; y <= 14; y++ {
		for x := 1; x <= 14; x++ {
			// 四角倒角
			if (x == 1 && y == 1) || (x == 14 && y == 1) || (x == 1 && y == 14) || (x == 14 && y == 14) {
				continue
			}
			if x == 1 || x == 14 || y == 1 || y == 14 {
				set(x, y, bgBorder)
			} else {
				set(x, y, bgDark)
			}
		}
	}

	// 2. 极简白文档页面: (3,3) 到 (12,12)
	for y := 3; y <= 12; y++ {
		for x := 3; x <= 12; x++ {
			// 右上角微折角
			if (x >= 11 && y == 3) || (x == 12 && y <= 4) {
				continue
			}
			set(x, y, pageWhite)
		}
	}
	// 折角微阴影
	set(11, 4, graphics.RGB(203, 213, 225))

	// 3. 极简统一科技蓝代码文本条 (整洁明晰，无色彩堆砌)
	for x := 5; x <= 10; x++ {
		set(x, 6, cCode)
	}
	for x := 5; x <= 9; x++ {
		set(x, 8, cCode)
	}
	for x := 5; x <= 7; x++ {
		set(x, 10, cCode)
	}
	set(8, 10, cCaret) // 极简光标点缀

	return pixels
}

// generate32x32Icon 绘制极简、高级质感、抗锯齿平滑的 32x32 任务栏大图标
func generate32x32Icon() []graphics.Color {
	w, h := 32, 32
	pixels := make([]graphics.Color, w*h)

	set := func(x, y int, c graphics.Color) {
		if x >= 0 && x < w && y >= 0 && y < h {
			pixels[y*w+x] = c
		}
	}

	bgDark := graphics.RGB(15, 23, 42)           // #0F172A 深邃极客底色
	bgGrad := graphics.RGB(30, 41, 59)           // #1E293B 平滑渐变
	bgBorder := graphics.RGBA(59, 130, 246, 210) // #3B82F6 科技蓝边框
	pageWhite := graphics.RGB(255, 255, 255)     // #FFFFFF 纯净白卡片
	pageFold := graphics.RGB(241, 245, 249)      // #F1F5F9 折角
	pageShadow := graphics.RGB(203, 213, 225)    // #CBD5E1 柔和折痕
	cCode := graphics.RGB(2, 132, 199)           // #0284C7 极简科技天蓝行
	cCaret := graphics.RGB(56, 189, 248)         // #38BDF8 亮色光标指示

	// 1. 底板圆角矩形: (2,2) 到 (29,29), 半径 5，带抗锯齿边缘
	for y := 2; y <= 29; y++ {
		for x := 2; x <= 29; x++ {
			dx := 0
			if x < 7 {
				dx = 7 - x
			} else if x > 24 {
				dx = x - 24
			}
			dy := 0
			if y < 7 {
				dy = 7 - y
			} else if y > 24 {
				dy = y - 24
			}
			distSq := dx*dx + dy*dy
			if distSq > 25 {
				continue
			}

			alpha := uint8(255)
			if distSq >= 16 {
				alpha = uint8(255 - (distSq-16)*25)
			}

			ratio := float64(x+y-4) / 54.0
			r := uint8(float64(bgDark.R())*(1-ratio) + float64(bgGrad.R())*ratio)
			g := uint8(float64(bgDark.G())*(1-ratio) + float64(bgGrad.G())*ratio)
			b := uint8(float64(bgDark.B())*(1-ratio) + float64(bgGrad.B())*ratio)

			if x == 2 || x == 29 || y == 2 || y == 29 || distSq >= 16 {
				set(x, y, graphics.RGBA(bgBorder.R(), bgBorder.G(), bgBorder.B(), alpha))
			} else {
				set(x, y, graphics.RGBA(r, g, b, alpha))
			}
		}
	}

	// 2. 纸张卡片主体: (6,5) 到 (25,26)
	for y := 5; y <= 26; y++ {
		for x := 6; x <= 25; x++ {
			// 右上角平滑折角: (x - 21) + (9 - y) <= 4
			if x >= 21 && y <= 9 && (x-21)+(9-y) <= 4 {
				continue
			}
			set(x, y, pageWhite)
		}
	}
	// 柔和折角三角形与微折痕
	for fy := 5; fy <= 9; fy++ {
		for fx := 21; fx <= 25; fx++ {
			if (fx-21)+(9-fy) <= 4 {
				if (fx-21)+(9-fy) == 4 {
					set(fx, fy, pageShadow)
				} else {
					set(fx, fy, pageFold)
				}
			}
		}
	}

	// 3. 极简统一科技蓝代码文本条 (律动排版，通透呼吸感)
	// 第 1 行 (长)
	for y := 10; y <= 11; y++ {
		for x := 9; x <= 20; x++ {
			set(x, y, cCode)
		}
	}
	// 第 2 行 (中长)
	for y := 14; y <= 15; y++ {
		for x := 9; x <= 22; x++ {
			set(x, y, cCode)
		}
	}
	// 第 3 行 (中)
	for y := 18; y <= 19; y++ {
		for x := 9; x <= 17; x++ {
			set(x, y, cCode)
		}
	}
	// 第 4 行 (短 + 闪烁光标块)
	for y := 22; y <= 23; y++ {
		for x := 9; x <= 13; x++ {
			set(x, y, cCode)
		}
		set(15, y, cCaret)
		set(16, y, cCaret)
	}

	return pixels
}
