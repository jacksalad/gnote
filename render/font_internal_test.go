package render

import (
	"testing"

	"github.com/jacksalad/gnote/graphics"
)

func TestClearGlyphCachesExcept(t *testing.T) {
	fm := GetFontManager()
	keep := graphics.NewFont("Consolas", 14)
	other := graphics.NewFont("Consolas", 21)

	keepFace := fm.GetFace(keep)
	otherFace := fm.GetFace(other)
	keepFace.GetGlyph('A', fm.hdc)
	otherFace.GetGlyph('A', fm.hdc)
	if len(keepFace.cache) == 0 || len(otherFace.cache) == 0 {
		t.Fatal("glyph caches were not populated")
	}

	fm.ClearGlyphCachesExcept(keep)
	if len(keepFace.cache) == 0 {
		t.Fatal("kept face cache must be preserved")
	}
	if len(otherFace.cache) != 0 {
		t.Fatalf("other face cache should be cleared, still %d glyphs", len(otherFace.cache))
	}
}

// 星平面字符不能把低 16 位直接交给 GDI：应与 U+FFFD 回退字形一致，且按原 rune 缓存。
func TestAstralRuneFallsBackToReplacement(t *testing.T) {
	fm := GetFontManager()
	face := fm.GetFace(graphics.NewFont("Microsoft YaHei", 16))

	emoji := face.GetGlyph(0x1F600, fm.hdc)
	if emoji == nil {
		t.Fatal("astral glyph is nil")
	}
	if emoji.CellIncX <= 0 {
		t.Fatalf("astral glyph advance = %d, want > 0", emoji.CellIncX)
	}
	repl := face.GetGlyph('\uFFFD', fm.hdc)
	if emoji.Width != repl.Width || emoji.Height != repl.Height || emoji.CellIncX != repl.CellIncX {
		t.Fatalf("astral glyph %dx%d/%d != replacement %dx%d/%d",
			emoji.Width, emoji.Height, emoji.CellIncX, repl.Width, repl.Height, repl.CellIncX)
	}
	if cached := face.cache[0x1F600]; cached != emoji {
		t.Fatal("astral glyph not cached under its own rune")
	}
}
