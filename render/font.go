//go:build windows

// Package render 提供软件渲染引擎与字体管理实现
package render

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/jacksalad/gnote/graphics"
)

var (
	gdi32Dll  = syscall.NewLazyDLL("gdi32.dll")
	user32Dll = syscall.NewLazyDLL("user32.dll")

	createFontWProc             = gdi32Dll.NewProc("CreateFontW")
	selectObjectProc            = gdi32Dll.NewProc("SelectObject")
	deleteObjectProc            = gdi32Dll.NewProc("DeleteObject")
	getGlyphOutlineWProc        = gdi32Dll.NewProc("GetGlyphOutlineW")
	getTextMetricsWProc         = gdi32Dll.NewProc("GetTextMetricsW")
	addFontMemResourceExProc    = gdi32Dll.NewProc("AddFontMemResourceEx")
	removeFontMemResourceExProc = gdi32Dll.NewProc("RemoveFontMemResourceEx")
	getDCProc                   = user32Dll.NewProc("GetDC")
	releaseDCProc               = user32Dll.NewProc("ReleaseDC")

	// gammaTable 矢量字形抗锯齿感知对比度伽马校正表（消除灰白底上的发虚、发灰现象）
	gammaTable [256]uint8
)

func init() {
	// 线性高保真映射：保持矢量字形原始灰度级，消除深色背景上字形边缘因 Gamma 过度提亮而造成的发虚、发胀与毛边晕散
	for i := 0; i < 256; i++ {
		gammaTable[i] = uint8(i)
	}
}

type glyphMetrics struct {
	BlackBoxX    uint32
	BlackBoxY    uint32
	GlyphOriginX int32
	GlyphOriginY int32
	CellIncX     int16
	CellIncY     int16
}

type fixed struct {
	Fract uint16
	Value int16
}

type mat2 struct {
	EM11, EM12, EM21, EM22 fixed
}

type textMetricW struct {
	Height           int32
	Ascent           int32
	Descent          int32
	InternalLeading  int32
	ExternalLeading  int32
	AveCharWidth     int32
	MaxCharWidth     int32
	Weight           int32
	Overhang         int32
	DigitizedAspectX int32
	DigitizedAspectY int32
	FirstChar        uint16
	LastChar         uint16
	DefaultChar      uint16
	BreakChar        uint16
	Italic           byte
	Underlined       byte
	StruckOut        byte
	PitchAndFamily   byte
	CharSet          byte
}

// Glyph 单个字形的抗锯齿灰度信息与空间度量
type Glyph struct {
	Width        int32
	Height       int32
	GlyphOriginX int32
	GlyphOriginY int32
	CellIncX     int32
	AlphaMask    []byte // 尺寸为 Width * Height，值为 0..255 的 Alpha 覆盖率
}

// FontFace 字体实例（缓存特定字体配置下的所有字形）
type FontFace struct {
	hFont   uintptr
	Ascent  int32
	Height  int32
	Descent int32
	cache   map[rune]*Glyph
	// spaceGlyph 非 nil 时 ' ' 使用固定步进（Font.AlignSpace），
	// 在字体面创建完成后写入且不再变更，读取无需加锁。
	spaceGlyph *Glyph
	mu         sync.RWMutex
}

// maxGlyphsPerFace 单个字体面的字形缓存上限，防止超大字符集下无界增长
const maxGlyphsPerFace = 8192

// storeGlyph 写入字形缓存（超出上限时整体回收，未引用的蒙版交由 GC 释放）
func (f *FontFace) storeGlyph(r rune, glyph *Glyph) {
	if len(f.cache) >= maxGlyphsPerFace {
		f.cache = make(map[rune]*Glyph, maxGlyphsPerFace/4)
	}
	f.cache[r] = glyph
}

// GetGlyph 获取指定字符的抗锯齿字形数据（线程安全并自动缓存）
func (f *FontFace) GetGlyph(r rune, hdc uintptr) *Glyph {
	if r == ' ' && f.spaceGlyph != nil {
		return f.spaceGlyph
	}

	f.mu.RLock()
	g, ok := f.cache[r]
	f.mu.RUnlock()
	if ok {
		return g
	}

	// GDI 的 GetGlyphOutlineW 只接受 UTF-16 码元，星平面字符（emoji、CJK 扩展 B）
	// 会被截断成低 16 位从而画出错误字形。这里统一回退到 U+FFFD，
	// 缓存仍以原始 rune 为键，重复查找不会反复走 GDI。
	code := r
	if code > 0xFFFF {
		code = '\uFFFD'
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// 双重检测
	if g, ok := f.cache[r]; ok {
		return g
	}

	selectObjectProc.Call(hdc, f.hFont)

	mat := mat2{
		EM11: fixed{Value: 1},
		EM22: fixed{Value: 1},
	}

	var gm glyphMetrics
	size, _, _ := getGlyphOutlineWProc.Call(
		hdc, uintptr(code), 6, uintptr(unsafe.Pointer(&gm)), 0, 0, uintptr(unsafe.Pointer(&mat)),
	)

	if int32(size) <= 0 {
		// 空白字符（空格等）没有位图，但 GDI 仍会填充有效的 gmCellIncX。
		// 直接丢弃它会让空格退回 Height/2，导致比例字体中空格宽度被放大数倍。
		adv := int32(gm.CellIncX)
		if adv <= 0 {
			adv = f.Height / 2
		}
		glyph := &Glyph{
			CellIncX: adv,
		}
		f.storeGlyph(r, glyph)
		return glyph
	}

	glyph := &Glyph{
		Width:        int32(gm.BlackBoxX),
		Height:       int32(gm.BlackBoxY),
		GlyphOriginX: gm.GlyphOriginX,
		GlyphOriginY: gm.GlyphOriginY,
		CellIncX:     int32(gm.CellIncX),
	}

	buf := make([]byte, size)
	getGlyphOutlineWProc.Call(
		hdc, uintptr(code), 6, uintptr(unsafe.Pointer(&gm)), size, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&mat)),
	)

	pitch := int32((gm.BlackBoxX + 3) &^ 3)
	mask := make([]byte, glyph.Width*glyph.Height)
	for row := int32(0); row < glyph.Height; row++ {
		for col := int32(0); col < glyph.Width; col++ {
			level := buf[row*pitch+col]
			if level > 0 {
				linearAlpha := uint8((uint32(level)*255 + 32) / 64)
				mask[row*glyph.Width+col] = gammaTable[linearAlpha]
			}
		}
	}
	glyph.AlphaMask = mask

	f.storeGlyph(r, glyph)
	return glyph
}

// fontKey 字体缓存键（避免每次测量都做字符串拼接）
type fontKey struct {
	name       string
	size       int32
	bold       bool
	italic     bool
	alignSpace bool
}

// FontManager 字体管理器（管理系统 GDI 字体与字体面缓存）
type FontManager struct {
	hdc   uintptr
	faces map[fontKey]*FontFace
	mu    sync.RWMutex
}

var (
	defaultFontManager     *FontManager
	defaultFontManagerOnce sync.Once
)

// GetFontManager 获取全局单例字体管理器
func GetFontManager() *FontManager {
	defaultFontManagerOnce.Do(func() {
		hdc, _, _ := getDCProc.Call(0)
		defaultFontManager = &FontManager{
			hdc:   hdc,
			faces: make(map[fontKey]*FontFace),
		}
	})
	return defaultFontManager
}

// GetFace 获取或创建指定字体的 FontFace
func (fm *FontManager) GetFace(font *graphics.Font) *FontFace {
	if font == nil {
		font = graphics.DefaultFont
	}
	key := fontKey{name: font.Name, size: font.Size, bold: font.Bold, italic: font.Italic, alignSpace: font.AlignSpace}

	// 读多写少：命中直接走读锁，避免每帧数百次全局互斥。
	fm.mu.RLock()
	face, ok := fm.faces[key]
	fm.mu.RUnlock()
	if ok {
		return face
	}

	fm.mu.Lock()
	defer fm.mu.Unlock()
	if face, ok := fm.faces[key]; ok {
		return face
	}

	weight := 400
	if font.Bold {
		weight = 700
	}
	var italic uint32 = 0
	if font.Italic {
		italic = 1
	}

	fontSize := font.Size
	if fontSize <= 0 {
		fontSize = 14
	}
	// 关键清晰度修复：使用负数高度 (-fontSize) 请求字符字模高度（EM height），彻底消除 Windows GDI Internal Leading 对字形的压缩变形
	cHeight := -int32(fontSize)

	namePtr, _ := syscall.UTF16PtrFromString(font.Name)
	hFont, _, _ := createFontWProc.Call(
		uintptr(int64(cHeight)), 0, 0, 0,
		uintptr(weight), uintptr(italic), 0, 0,
		1, // DEFAULT_CHARSET
		0, 0,
		5, // CLEARTYPE_QUALITY
		0,
		uintptr(unsafe.Pointer(namePtr)),
	)

	oldFont, _, _ := selectObjectProc.Call(fm.hdc, hFont)
	var tm textMetricW
	getTextMetricsWProc.Call(fm.hdc, uintptr(unsafe.Pointer(&tm)))
	selectObjectProc.Call(fm.hdc, oldFont)

	face = &FontFace{
		hFont:   hFont,
		Ascent:  tm.Ascent,
		Height:  tm.Height,
		Descent: tm.Descent,
		cache:   make(map[rune]*Glyph),
	}
	if font.AlignSpace {
		// 比例字体的原生空格通常只有约 0.25em（约为普通字母的一半），代码缩进会明显偏窄。
		// 以数字 '0' 的步进作为 1 列宽度，使空格与普通字母等宽，
		// 制表符（4 列）与 4 空格缩进也随之等宽。
		if digit := face.GetGlyph('0', fm.hdc); digit != nil && digit.CellIncX > 0 {
			face.spaceGlyph = &Glyph{CellIncX: digit.CellIncX}
		}
	}
	fm.faces[key] = face
	return face
}

// ClearGlyphCachesExcept 释放除 keep 之外所有字体面的字形位图缓存（字体句柄与度量保留）。
// 缩放/字号切换后旧字号不再使用，及时回收蒙版内存；被清理的面在再次绘制时会按需重新光栅化。
func (fm *FontManager) ClearGlyphCachesExcept(keep ...*graphics.Font) {
	keepKeys := make(map[fontKey]struct{}, len(keep))
	for _, f := range keep {
		if f == nil {
			continue
		}
		keepKeys[fontKey{
			name:       f.Name,
			size:       f.Size,
			bold:       f.Bold,
			italic:     f.Italic,
			alignSpace: f.AlignSpace,
		}] = struct{}{}
	}

	fm.mu.Lock()
	defer fm.mu.Unlock()
	for key, face := range fm.faces {
		if _, ok := keepKeys[key]; ok {
			continue
		}
		face.mu.Lock()
		face.cache = make(map[rune]*Glyph)
		face.mu.Unlock()
	}
}

// MeasureText 测量文本在指定字体下的排版宽高
func (fm *FontManager) MeasureText(font *graphics.Font, text string) (width, height int32) {
	if text == "" {
		return 0, 0
	}
	face := fm.GetFace(font)
	lines := int32(1)
	var lineWidth int32 = 0
	var maxWidth int32 = 0

	for _, r := range text {
		if r == '\r' {
			continue
		}
		if r == '\n' {
			lines++
			if lineWidth > maxWidth {
				maxWidth = lineWidth
			}
			lineWidth = 0
			continue
		}
		if r == '\t' {
			spaceGlyph := face.GetGlyph(' ', fm.hdc)
			lineWidth += spaceGlyph.CellIncX * 4
			continue
		}
		glyph := face.GetGlyph(r, fm.hdc)
		lineWidth += glyph.CellIncX
	}
	if lineWidth > maxWidth {
		maxWidth = lineWidth
	}
	return maxWidth, lines * face.Height
}

// MeasureRunes 测量 rune 切片在指定字体下的排版物理宽度 (无字符串转换与堆内存分配)
func (fm *FontManager) MeasureRunes(font *graphics.Font, runes []rune) int32 {
	if len(runes) == 0 {
		return 0
	}
	face := fm.GetFace(font)
	if face == nil {
		return 0
	}
	var lineWidth int32 = 0
	for _, r := range runes {
		if r == '\r' || r == '\n' {
			continue
		}
		if r == '\t' {
			spaceGlyph := face.GetGlyph(' ', fm.hdc)
			lineWidth += spaceGlyph.CellIncX * 4
			continue
		}
		glyph := face.GetGlyph(r, fm.hdc)
		lineWidth += glyph.CellIncX
	}
	return lineWidth
}

// GetRuneWidth 获取单个字符的排版物理步进宽度
func (fm *FontManager) GetRuneWidth(font *graphics.Font, r rune) int32 {
	face := fm.GetFace(font)
	if face == nil {
		return 8
	}
	if r == '\t' {
		spaceGlyph := face.GetGlyph(' ', fm.hdc)
		return spaceGlyph.CellIncX * 4
	}
	glyph := face.GetGlyph(r, fm.hdc)
	if glyph != nil {
		return glyph.CellIncX
	}
	return 8
}

// MeasureRunesFace 使用已解析的 FontFace 测量 rune 切片宽度（避免重复查找字体面）
func (fm *FontManager) MeasureRunesFace(face *FontFace, runes []rune) int32 {
	if face == nil || len(runes) == 0 {
		return 0
	}
	var (
		lineWidth int32
		glyphs    [glyphBatch]*Glyph
	)
	for start := 0; start < len(runes); start += glyphBatch {
		end := start + glyphBatch
		if end > len(runes) {
			end = len(runes)
		}
		chunk := runes[start:end]
		dst := glyphs[:len(chunk)]
		face.resolveGlyphs(chunk, fm.hdc, dst)
		for i, r := range chunk {
			switch r {
			case '\r', '\n':
				continue
			case '\t':
				lineWidth += face.GetGlyph(' ', fm.hdc).CellIncX * 4
			default:
				if g := dst[i]; g != nil {
					lineWidth += g.CellIncX
				}
			}
		}
	}
	return lineWidth
}

// RuneWidthFace 使用已解析的 FontFace 获取单个字符步进宽度
func (fm *FontManager) RuneWidthFace(face *FontFace, r rune) int32 {
	if face == nil {
		return 8
	}
	if r == '\t' {
		return face.GetGlyph(' ', fm.hdc).CellIncX * 4
	}
	glyph := face.GetGlyph(r, fm.hdc)
	if glyph != nil {
		return glyph.CellIncX
	}
	return 8
}

const glyphBatch = 256

// resolveGlyphs 在一次读锁内解析一批 rune 的字形指针，未命中的再走 GetGlyph。
// 把每帧数百次 RLock/RUnlock 收敛为每 256 个字符一次。
func (f *FontFace) resolveGlyphs(runes []rune, hdc uintptr, dst []*Glyph) {
	f.mu.RLock()
	for i, r := range runes {
		if r == ' ' && f.spaceGlyph != nil {
			dst[i] = f.spaceGlyph
			continue
		}
		dst[i] = f.cache[r]
	}
	f.mu.RUnlock()

	for i, r := range runes {
		if dst[i] != nil {
			continue
		}
		switch r {
		case '\r', '\n', '\t':
			continue // 控制字符不参与字形解析
		}
		dst[i] = f.GetGlyph(r, hdc)
	}
}

// DrawText 绘制矢量文本（完美支持中文、Unicode、多行折行与任意 Windows 字体，抗锯齿混合，受 Clip 裁剪）
// 返回值是最后一行的笔位宽度（单行文本即整行宽度），供调用方复用测量结果。
func (c *Canvas) DrawText(text string, x, y int32, font *graphics.Font, color graphics.Color) int32 {
	if text == "" {
		return 0
	}
	if font == nil {
		font = graphics.DefaultFont
	}
	fm := GetFontManager()
	face := fm.GetFace(font)
	clip := c.Clip()
	c.dirty = true

	var buf [glyphBatch]rune
	n := 0
	penX, penY := x, y
	for _, r := range text {
		buf[n] = r
		n++
		if n == glyphBatch {
			penX, penY = c.drawRuneChunk(buf[:n], face, fm, penX, penY, x, color, clip)
			n = 0
		}
	}
	if n > 0 {
		penX, penY = c.drawRuneChunk(buf[:n], face, fm, penX, penY, x, color, clip)
	}
	return penX - x
}

// DrawRunes 与 DrawText 等价，但直接消费 rune 切片，避免把高亮片段反复转成 string。
func (c *Canvas) DrawRunes(runes []rune, x, y int32, font *graphics.Font, color graphics.Color) int32 {
	if len(runes) == 0 {
		return 0
	}
	if font == nil {
		font = graphics.DefaultFont
	}
	fm := GetFontManager()
	face := fm.GetFace(font)
	clip := c.Clip()
	c.dirty = true

	penX, penY := x, y
	for start := 0; start < len(runes); start += glyphBatch {
		end := start + glyphBatch
		if end > len(runes) {
			end = len(runes)
		}
		penX, penY = c.drawRuneChunk(runes[start:end], face, fm, penX, penY, x, color, clip)
	}
	return penX - x
}

// drawRuneChunk 绘制至多 glyphBatch 个 rune；penX/penY 通过返回值跨块传递。
func (c *Canvas) drawRuneChunk(runes []rune, face *FontFace, fm *FontManager, penX, penY, originX int32, color graphics.Color, clip graphics.Rect) (int32, int32) {
	var glyphs [glyphBatch]*Glyph
	dst := glyphs[:len(runes)]
	face.resolveGlyphs(runes, fm.hdc, dst)

	baseR, baseG, baseB := color.R(), color.G(), color.B()
	baseA := color.A()

	for i, r := range runes {
		switch r {
		case '\r':
			continue
		case '\n':
			penX = originX
			penY += face.Height
			continue
		case '\t':
			penX += face.GetGlyph(' ', fm.hdc).CellIncX * 4
			continue
		}

		g := dst[i]
		if g == nil || g.Width <= 0 || g.Height <= 0 || len(g.AlphaMask) == 0 {
			if g != nil {
				penX += g.CellIncX
			}
			continue
		}

		// 完全落在裁剪区外的字形只推进笔位，不进入逐像素混合（长行横向滚动时省掉绝大部分工作）
		gx := penX + g.GlyphOriginX
		if gx >= clip.X+clip.Width || gx+g.Width <= clip.X {
			penX += g.CellIncX
			continue
		}

		for row := int32(0); row < g.Height; row++ {
			py := penY + (face.Ascent - g.GlyphOriginY) + row
			if py < clip.Y || py >= clip.Y+clip.Height || py < 0 || py >= c.Height {
				continue
			}
			rowOffset := py * c.Width
			for col := int32(0); col < g.Width; col++ {
				px := penX + g.GlyphOriginX + col
				if px < clip.X || px >= clip.X+clip.Width || px < 0 || px >= c.Width {
					continue
				}
				alpha := g.AlphaMask[row*g.Width+col]
				if baseA < 255 {
					alpha = uint8((uint32(alpha) * uint32(baseA)) / 255)
				}
				if alpha > 0 {
					idx := rowOffset + px
					a := uint32(alpha)
					invA := 255 - a
					dstColor := c.Buffer[idx]
					dstR := (dstColor >> 16) & 0xFF
					dstG := (dstColor >> 8) & 0xFF
					dstB := dstColor & 0xFF

					outR := (uint32(baseR)*a + dstR*invA + 127) / 255
					outG := (uint32(baseG)*a + dstG*invA + 127) / 255
					outB := (uint32(baseB)*a + dstB*invA + 127) / 255

					c.Buffer[idx] = (0xFF << 24) | (outR << 16) | (outG << 8) | outB
				}
			}
		}
		penX += g.CellIncX
	}
	return penX, penY
}

// MeasureText 测量文本在指定字体下的精确像素宽高
func (c *Canvas) MeasureText(text string, font *graphics.Font) (width, height int32) {
	return GetFontManager().MeasureText(font, text)
}

var (
	registeredMemFonts   []uintptr
	registeredMemFontsMu sync.Mutex
)

// ExtractFontFamily 从 TrueType / OpenType 字体二进制数据中自动解析出其 FontFamily 名称
func ExtractFontFamily(data []byte) (string, error) {
	if len(data) < 12 {
		return "", errors.New("font data too short")
	}

	numTables := binary.BigEndian.Uint16(data[4:6])
	var nameOffset, nameLength uint32

	for i := 0; i < int(numTables); i++ {
		entryOffset := 12 + i*16
		if entryOffset+16 > len(data) {
			break
		}
		tag := string(data[entryOffset : entryOffset+4])
		if tag == "name" {
			nameOffset = binary.BigEndian.Uint32(data[entryOffset+8 : entryOffset+12])
			nameLength = binary.BigEndian.Uint32(data[entryOffset+12 : entryOffset+16])
			break
		}
	}

	if nameOffset == 0 || int(nameOffset+nameLength) > len(data) {
		return "", errors.New("name table not found in font")
	}

	nameData := data[nameOffset : nameOffset+nameLength]
	if len(nameData) < 6 {
		return "", errors.New("invalid name table")
	}

	count := int(binary.BigEndian.Uint16(nameData[2:4]))
	stringOffset := int(binary.BigEndian.Uint16(nameData[4:6]))

	var familyName string
	for i := 0; i < count; i++ {
		recOffset := 6 + i*12
		if recOffset+12 > len(nameData) {
			break
		}
		platformID := binary.BigEndian.Uint16(nameData[recOffset : recOffset+2])
		nameID := binary.BigEndian.Uint16(nameData[recOffset+6 : recOffset+8])
		length := int(binary.BigEndian.Uint16(nameData[recOffset+8 : recOffset+10]))
		offset := int(binary.BigEndian.Uint16(nameData[recOffset+10 : recOffset+12]))

		// nameID 1 为 Font Family Name
		if nameID == 1 {
			strStart := stringOffset + offset
			if strStart+length <= len(nameData) {
				strBytes := nameData[strStart : strStart+length]
				var s string
				if platformID == 0 || platformID == 3 { // Unicode / Windows UTF-16BE
					runes := make([]rune, 0, length/2)
					for j := 0; j < length-1; j += 2 {
						r := rune(binary.BigEndian.Uint16(strBytes[j : j+2]))
						runes = append(runes, r)
					}
					s = string(runes)
				} else {
					s = string(strBytes)
				}
				s = strings.TrimSpace(s)
				if len(s) > 0 {
					familyName = s
					if platformID == 3 {
						break
					}
				}
			}
		}
	}

	if familyName == "" {
		return "", errors.New("failed to find font family name in name table")
	}
	return familyName, nil
}

// RegisterMemoryFont 零 CGO 从内存二进制字节流动态注册私有字体到当前进程，并自动解析返回该字体的 FontFamily 名称
func RegisterMemoryFont(fontData []byte) (string, error) {
	if len(fontData) == 0 {
		return "", errors.New("empty font data")
	}

	familyName, err := ExtractFontFamily(fontData)
	if err != nil {
		return "", fmt.Errorf("failed to parse font family: %w", err)
	}

	var numFonts uint32
	hFontRes, _, _ := addFontMemResourceExProc.Call(
		uintptr(unsafe.Pointer(&fontData[0])),
		uintptr(len(fontData)),
		0,
		uintptr(unsafe.Pointer(&numFonts)),
	)
	if hFontRes == 0 || numFonts == 0 {
		return "", errors.New("AddFontMemResourceEx failed to register memory font")
	}

	registeredMemFontsMu.Lock()
	registeredMemFonts = append(registeredMemFonts, hFontRes)
	registeredMemFontsMu.Unlock()

	return familyName, nil
}

// RegisterMemoryFontWithName 使用显式指定的 FontFamily 名称注册内存字体
func RegisterMemoryFontWithName(fontData []byte, name string) (string, error) {
	if len(fontData) == 0 {
		return "", errors.New("empty font data")
	}
	if name == "" {
		return RegisterMemoryFont(fontData)
	}

	var numFonts uint32
	hFontRes, _, _ := addFontMemResourceExProc.Call(
		uintptr(unsafe.Pointer(&fontData[0])),
		uintptr(len(fontData)),
		0,
		uintptr(unsafe.Pointer(&numFonts)),
	)
	if hFontRes == 0 || numFonts == 0 {
		return "", errors.New("AddFontMemResourceEx failed to register memory font")
	}

	registeredMemFontsMu.Lock()
	registeredMemFonts = append(registeredMemFonts, hFontRes)
	registeredMemFontsMu.Unlock()

	return name, nil
}
