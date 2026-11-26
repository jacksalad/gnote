package render

import (
	"bytes"
	"os"
	"testing"

	"github.com/jacksalad/gnote/graphics"
)

// p2LoadEmbeddedFont 注册内嵌 Gnote Mono：该字体没有真实粗体字面，
// 请求字重 700 时 GDI 会合成加粗，并让每个字形的步进比常规字面宽 1px。
func p2LoadEmbeddedFont(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../cmd/gnote/font.ttf")
	if err != nil {
		t.Skipf("embedded font unavailable: %v", err)
	}
	family, err := RegisterMemoryFont(data)
	if err != nil {
		t.Fatalf("register embedded font: %v", err)
	}
	return family
}

// p2NativeBoldDelta 返回 GDI 对内嵌字体合成粗体的步进增量。
// 使用不带 AlignSpace 的字体面（编辑器之外的字面不做步进归一），反映系统原生行为。
func p2NativeBoldDelta(t *testing.T, fm *FontManager, family string, size int32, r rune) int32 {
	t.Helper()
	regular := fm.GetFace(&graphics.Font{Name: family, Size: size})
	bold := fm.GetFace(&graphics.Font{Name: family, Size: size, Bold: true})
	return fm.RuneWidthFace(bold, r) - fm.RuneWidthFace(regular, r)
}

// 编辑器之外（普通 UI 文本）的字体面保持 GDI 原生步进：归一只服务于编辑器的等宽网格。
func TestFont_NativeBoldAdvanceOutsideEditorUnchanged(t *testing.T) {
	family := p2LoadEmbeddedFont(t)
	fm := GetFontManager()
	delta := p2NativeBoldDelta(t, fm, family, 16, 'f')
	if delta == 0 {
		t.Skip("GDI did not synthesise a wider bold face on this machine")
	}
	for _, r := range []rune{'0', 'W', '中', 0x2460} {
		if got := p2NativeBoldDelta(t, fm, family, 16, r); got != delta {
			t.Fatalf("U+%04X: native synthetic bold delta = %d, want %d", r, got, delta)
		}
	}
}

// 回归：合成粗体字形比常规字形宽 1px，而编辑器的折行、行宽、滚动上界都按常规字面计算，
// 含粗体关键字的行因此越过折行上限、行尾无法滚动进视口、光标与命中错位。
// 字体面归一后，粗体/粗斜体字形必须沿用常规字面的步进（内嵌字体的每个字号）。
func TestFont_SyntheticBoldKeepsRegularAdvance(t *testing.T) {
	family := p2LoadEmbeddedFont(t)
	fm := GetFontManager()
	if p2NativeBoldDelta(t, fm, family, 16, 'f') == 0 {
		t.Skip("GDI did not synthesise a wider bold face on this machine")
	}

	const sample = "fA0 iW中①⋯★M.\t"
	for _, size := range []int32{9, 16, 31} {
		regular := fm.GetFace(&graphics.Font{Name: family, Size: size, AlignSpace: true})
		for _, styled := range []*graphics.Font{
			{Name: family, Size: size, Bold: true, AlignSpace: true},
			{Name: family, Size: size, Italic: true, AlignSpace: true},
			{Name: family, Size: size, Bold: true, Italic: true, AlignSpace: true},
		} {
			face := fm.GetFace(styled)
			for _, r := range sample {
				if got, want := fm.RuneWidthFace(face, r), fm.RuneWidthFace(regular, r); got != want {
					t.Errorf("size %d bold=%v italic=%v U+%04X advance = %d, want %d (regular)",
						size, styled.Bold, styled.Italic, r, got, want)
				}
			}
			if got, want := fm.MeasureRunesFace(face, []rune(sample)), fm.MeasureRunesFace(regular, []rune(sample)); got != want {
				t.Errorf("size %d bold=%v italic=%v run width = %d, want %d", size, styled.Bold, styled.Italic, got, want)
			}
		}
	}
}

// 归一的只是步进：合成粗体仍必须画出加粗后的字形，不能退回常规字形的位图。
func TestFont_SyntheticBoldInkStaysEmboldened(t *testing.T) {
	family := p2LoadEmbeddedFont(t)
	fm := GetFontManager()
	if p2NativeBoldDelta(t, fm, family, 16, 'f') == 0 {
		t.Skip("GDI did not synthesise a wider bold face on this machine")
	}
	regular := fm.GetFace(&graphics.Font{Name: family, Size: 16, AlignSpace: true})
	bold := fm.GetFace(&graphics.Font{Name: family, Size: 16, Bold: true, AlignSpace: true})
	for _, r := range []rune{'A', 'M', '中'} {
		reg, emb := regular.GetGlyph(r, fm.hdc), bold.GetGlyph(r, fm.hdc)
		if emb.Width < reg.Width || emb.Height != reg.Height {
			t.Fatalf("U+%04X: bold ink box %dx%d is not emboldened (regular %dx%d)", r, emb.Width, emb.Height, reg.Width, reg.Height)
		}
		if bytes.Equal(emb.AlphaMask, reg.AlphaMask) {
			t.Fatalf("U+%04X: bold glyph mask is identical to the regular one", r)
		}
	}
}

// 绘制与测量共用同一步进：DrawRunes 的返回宽度（画笔实际推进量）必须等于常规字面的排版宽度。
func TestFont_DrawRunesBoldUsesRegularAdvance(t *testing.T) {
	family := p2LoadEmbeddedFont(t)
	fm := GetFontManager()
	if p2NativeBoldDelta(t, fm, family, 16, 'f') == 0 {
		t.Skip("GDI did not synthesise a wider bold face on this machine")
	}
	regular := &graphics.Font{Name: family, Size: 16, AlignSpace: true}
	bold := &graphics.Font{Name: family, Size: 16, Bold: true, AlignSpace: true}
	runes := []rune("func 中=①")

	canvas := NewCanvas(400, 60)
	canvas.Clear(graphics.ColorWhite)
	drawnBold := canvas.DrawRunes(runes, 10, 0, bold, graphics.ColorBlack)
	canvas.Clear(graphics.ColorWhite)
	drawnRegular := canvas.DrawRunes(runes, 10, 0, regular, graphics.ColorBlack)
	if want := fm.MeasureRunes(regular, runes); drawnBold != want || drawnRegular != want {
		t.Fatalf("drawn advance: bold=%d regular=%d, want %d", drawnBold, drawnRegular, want)
	}
}

// 旧版本进程写出的磁盘字形缓存里，合成粗体字形仍带着 +1 步进；
// 装入缓存后必须同样归一，且缓存命中的字形只改步进、不重新光栅化。
func TestFont_SyntheticBoldPinAppliesToLoadedGlyphCache(t *testing.T) {
	family := p2LoadEmbeddedFont(t)
	fm := GetFontManager()
	delta := p2NativeBoldDelta(t, fm, family, 16, 'f')
	if delta == 0 {
		t.Skip("GDI did not synthesise a wider bold face on this machine")
	}

	id, ok := memoryFontID(family)
	if !ok {
		t.Fatal("embedded font has no cache fingerprint")
	}
	regularAdv := fm.RuneWidthFace(fm.GetFace(&graphics.Font{Name: family, Size: 16, AlignSpace: true}), 'f')
	mask := bytes.Repeat([]byte{0x7B}, 9*13)
	stale := &Glyph{Width: 9, Height: 13, GlyphOriginX: 1, CellIncX: regularAdv + delta, AlphaMask: mask}
	data, err := encodeGlyphCache(glyphCacheKey(id, 16, true, false, true), map[rune]*Glyph{'f': stale})
	if err != nil {
		t.Fatal(err)
	}
	glyphCacheDirOverride = t.TempDir()
	defer func() { glyphCacheDirOverride = "" }()
	if err := writeGlyphCacheFile(glyphCacheKey(id, 16, true, false, true), data); err != nil {
		t.Fatal(err)
	}

	// 全新的字体管理器：字体面只能从刚写出的旧缓存装载
	fresh := &FontManager{hdc: fm.hdc, faces: make(map[fontKey]*FontFace)}
	face := fresh.GetFace(&graphics.Font{Name: family, Size: 16, Bold: true, AlignSpace: true})
	if got := fresh.RuneWidthFace(face, 'f'); got != regularAdv {
		t.Fatalf("glyph loaded from a v1.0.2 cache keeps advance %d, want %d", got, regularAdv)
	}
	if loaded := face.GetGlyph('f', fresh.hdc); loaded.Width != stale.Width || !bytes.Equal(loaded.AlphaMask, mask) {
		t.Fatal("cached glyph was not reused (only its advance should be corrected)")
	}
}
