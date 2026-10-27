//go:build windows

package render

import (
	"os"
	"testing"
	"unsafe"

	"github.com/jacksalad/gnote/graphics"
)

// 空白字符没有位图，但 GDI 仍返回有效的 gmCellIncX。回归测试确保不会退回 Height/2
// （退回时会凭空放大空格步进，破坏缩进与列对齐）。
func TestFont_BlankGlyphUsesGDICellAdvance(t *testing.T) {
	data, err := os.ReadFile("../cmd/gnote/font.ttf")
	if err != nil {
		t.Skipf("embedded font unavailable: %v", err)
	}
	family, err := RegisterMemoryFont(data)
	if err != nil {
		t.Fatalf("register font: %v", err)
	}

	fm := GetFontManager()
	font := graphics.NewFont(family, 16)
	face := fm.GetFace(font)

	selectObjectProc.Call(fm.hdc, face.hFont)
	mat := mat2{EM11: fixed{Value: 1}, EM22: fixed{Value: 1}}
	var gm glyphMetrics
	size, _, _ := getGlyphOutlineWProc.Call(
		fm.hdc, uintptr(' '), 6, uintptr(unsafe.Pointer(&gm)), 0, 0, uintptr(unsafe.Pointer(&mat)),
	)
	if size != 0 {
		t.Fatalf("expected blank glyph to have no bitmap, size=%d", int32(size))
	}
	if gm.CellIncX <= 0 {
		t.Fatalf("GDI did not report a space advance: %d", gm.CellIncX)
	}

	glyph := face.GetGlyph(' ', fm.hdc)
	if glyph.CellIncX != int32(gm.CellIncX) {
		t.Fatalf("space advance=%d, want GDI cell advance %d (Height/2 fallback is a bug)",
			glyph.CellIncX, gm.CellIncX)
	}
}

func TestFont_AlignSpaceMatchesDigitAdvance(t *testing.T) {
	fm := GetFontManager()

	// 找一款比例字体作为对照组：内嵌 Gnote Mono 本身等宽，无法体现 AlignSpace 的差异。
	var family string
	for _, name := range []string{"Microsoft YaHei", "SimSun", "Segoe UI", "Times New Roman"} {
		f := graphics.NewFont(name, 16)
		if fm.RuneWidthFace(fm.GetFace(f), ' ') < fm.RuneWidthFace(fm.GetFace(f), '0') {
			family = name
			break
		}
	}
	if family == "" {
		t.Skip("no proportional system font available")
	}

	natural := graphics.NewFont(family, 16)
	aligned := graphics.NewFont(family, 16)
	aligned.AlignSpace = true

	digit := fm.GetRuneWidth(natural, '0')
	naturalSpace := fm.GetRuneWidth(natural, ' ')
	alignedSpace := fm.GetRuneWidth(aligned, ' ')

	if naturalSpace >= digit {
		t.Fatalf("precondition: proportional space (%d) should be narrower than digit (%d)", naturalSpace, digit)
	}
	if alignedSpace != digit {
		t.Fatalf("aligned space=%d, want digit advance %d", alignedSpace, digit)
	}

	// 回归目标：4 空格缩进 / 制表符必须与 4 个英文字符等宽，而不是约 2 个字符。
	want := digit * 4
	indent, _ := fm.MeasureText(aligned, "    ")
	tab, _ := fm.MeasureText(aligned, "\t")
	if indent != want {
		t.Fatalf("aligned 4-space indent=%d, want %d", indent, want)
	}
	if tab != want {
		t.Fatalf("aligned tab width=%d, want %d", tab, want)
	}

	// 对齐开关必须彼此隔离（字体面缓存键包含 AlignSpace）。
	naturalIndent, _ := fm.MeasureText(natural, "    ")
	if naturalIndent != naturalSpace*4 {
		t.Fatalf("natural 4-space indent=%d, want %d", naturalIndent, naturalSpace*4)
	}
}

func TestCanvasResizeReusesPixelBuffer(t *testing.T) {
	c := NewCanvas(100, 100)
	c.Resize(50, 40)
	if c.Width != 50 || c.Height != 40 {
		t.Fatalf("unexpected size after shrink: %dx%d", c.Width, c.Height)
	}
	if len(c.Buffer) != 50*40 {
		t.Fatalf("buffer length=%d, want %d", len(c.Buffer), 50*40)
	}
	if cap(c.Buffer) < 100*100 {
		t.Fatalf("shrink must keep the original capacity, cap=%d", cap(c.Buffer))
	}
	if clip := c.Clip(); clip.Width != 50 || clip.Height != 40 {
		t.Fatalf("clip not reset after resize: %+v", clip)
	}

	c.FillRect(0, 0, 50, 40, 0xFF112233)
	if c.Buffer[39*50+49] != 0xFF112233 {
		t.Fatalf("pixel write failed after shrink")
	}

	c.Resize(200, 200)
	if len(c.Buffer) != 200*200 {
		t.Fatalf("buffer did not grow: len=%d", len(c.Buffer))
	}
	c.FillRect(199, 199, 1, 1, 0xFF445566)
	if c.Buffer[199*200+199] != 0xFF445566 {
		t.Fatalf("pixel write failed after grow")
	}
}
