//go:build windows

package render

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jacksalad/gnote/graphics"
)

func TestGlyphCacheEncodeDecodeRoundTrip(t *testing.T) {
	key := "fp-123-abcdef|16|false|false|true"
	in := map[rune]*Glyph{
		' ': {CellIncX: 9},
		'A': {Width: 2, Height: 2, GlyphOriginX: -1, GlyphOriginY: 7, CellIncX: 8, AlphaMask: []byte{0, 32, 128, 255}},
		'中': {Width: 3, Height: 1, CellIncX: 16, AlphaMask: []byte{1, 2, 3}},
	}
	data, err := encodeGlyphCache(key, in)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	out, err := decodeGlyphCache(data, key)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != len(in) {
		t.Fatalf("decoded %d glyphs, want %d", len(out), len(in))
	}
	for r, want := range in {
		got, ok := out[r]
		if !ok {
			t.Fatalf("rune %q missing after round trip", r)
		}
		if got.Width != want.Width || got.Height != want.Height ||
			got.GlyphOriginX != want.GlyphOriginX || got.GlyphOriginY != want.GlyphOriginY ||
			got.CellIncX != want.CellIncX || !bytes.Equal(got.AlphaMask, want.AlphaMask) {
			t.Fatalf("rune %q round trip mismatch: %+v vs %+v", r, got, want)
		}
	}
}

func TestGlyphCacheDecodeRejectsCorruption(t *testing.T) {
	key := "fp-1-x|16|false|false|false"
	valid, err := encodeGlyphCache(key, map[rune]*Glyph{'A': {Width: 1, Height: 1, CellIncX: 8, AlphaMask: []byte{255}}})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	bad := map[string][]byte{
		"empty":       nil,
		"bad magic":   append([]byte("XXXX"), valid[4:]...),
		"truncated":   valid[:len(valid)-1],
		"wrong key":   valid,
		"header only": valid[:12],
	}
	for name, data := range bad {
		if name == "wrong key" {
			if _, err := decodeGlyphCache(data, "different-key"); err == nil {
				t.Fatalf("%s: expected error", name)
			}
			continue
		}
		if _, err := decodeGlyphCache(data, key); err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}

	// 超大字形的畸形记录必须被拒绝
	oversize := append([]byte(nil), valid...)
	// 记录头紧跟在 magic/version/keyLen/key/count 之后
	off := 12 + len(key) + 4
	copy(oversize[off+4:off+8], []byte{0xFF, 0xFF, 0xFF, 0x7F})
	if _, err := decodeGlyphCache(oversize, key); err == nil {
		t.Fatal("oversize glyph: expected error")
	}
}

func TestGlyphCacheSaveAndReloadFromDisk(t *testing.T) {
	fontData, err := os.ReadFile("../cmd/gnote/font.ttf")
	if err != nil {
		t.Skipf("embedded font not available: %v", err)
	}
	family, err := RegisterMemoryFont(fontData)
	if err != nil {
		t.Fatalf("RegisterMemoryFont: %v", err)
	}

	prevOverride := glyphCacheDirOverride
	glyphCacheDirOverride = t.TempDir()
	t.Cleanup(func() {
		glyphCacheDirOverride = prevOverride
		// 清理本次注册的字体面，避免影响其它测试的字形缓存状态
		if fm := GetFontManager(); fm != nil {
			fm.mu.Lock()
			delete(fm.faces, fontKey{name: family, size: 23, alignSpace: true})
			fm.mu.Unlock()
		}
	})

	font := graphics.NewFont(family, 23)
	font.AlignSpace = true
	fm := GetFontManager()
	face := fm.GetFace(font)
	runes := []rune("Ag中")
	want := make(map[rune]*Glyph, len(runes))
	for _, r := range runes {
		want[r] = face.GetGlyph(r, fm.hdc)
	}
	SaveGlyphCaches()

	files, err := filepath.Glob(filepath.Join(glyphCacheDirOverride, "glyphcache", "*.glyphcache"))
	if err != nil || len(files) == 0 {
		t.Fatalf("expected cache file written, got %v (err %v)", files, err)
	}

	// 用全新的字体管理器再取同一字体面，必须命中磁盘缓存且蒙版逐字节一致
	fm2 := &FontManager{hdc: fm.hdc, faces: make(map[fontKey]*FontFace)}
	face2 := fm2.GetFace(font)
	for _, r := range runes {
		got := face2.GetGlyph(r, fm2.hdc)
		w := want[r]
		if got.Width != w.Width || got.Height != w.Height ||
			got.GlyphOriginX != w.GlyphOriginX || got.GlyphOriginY != w.GlyphOriginY ||
			got.CellIncX != w.CellIncX || !bytes.Equal(got.AlphaMask, w.AlphaMask) {
			t.Fatalf("rune %q reload mismatch", r)
		}
	}
}

func TestPruneGlyphCacheFiles(t *testing.T) {
	prev := glyphCacheDirOverride
	glyphCacheDirOverride = t.TempDir()
	t.Cleanup(func() { glyphCacheDirOverride = prev })

	dir, err := glyphCacheDir()
	if err != nil {
		t.Fatalf("glyphCacheDir: %v", err)
	}
	const total = maxGlyphCacheFiles + 5
	for i := range total {
		path := filepath.Join(dir, "cache-"+string(rune('a'+i))+".glyphcache")
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		// 让编号越大越新
		stamp := time.Unix(int64(1700000000+i*60), 0)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatalf("chtimes: %v", err)
		}
	}
	// 非缓存文件不得被清理
	keep := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(keep, []byte("keep"), 0o644); err != nil {
		t.Fatalf("write keep: %v", err)
	}

	pruneGlyphCacheFiles()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	cached := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".glyphcache" {
			cached++
		}
	}
	if cached != maxGlyphCacheFiles {
		t.Fatalf("kept %d cache files, want %d", cached, maxGlyphCacheFiles)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("non-cache file was removed: %v", err)
	}
	// 最旧的 5 个必须被删除，最新的保留
	for i := range 5 {
		old := filepath.Join(dir, "cache-"+string(rune('a'+i))+".glyphcache")
		if _, err := os.Stat(old); !os.IsNotExist(err) {
			t.Fatalf("old cache file %q should be pruned", filepath.Base(old))
		}
	}
}

func TestFontFingerprintFromChecksum(t *testing.T) {
	data, err := os.ReadFile("../cmd/gnote/font.ttf")
	if err != nil {
		t.Skipf("embedded font not available: %v", err)
	}
	sum, ok := fontChecksumAdjustment(data)
	if !ok || sum == 0 {
		t.Fatalf("expected head checkSumAdjustment, got ok=%v sum=%#x", ok, sum)
	}
	fp1, fp2 := fontFingerprint(data), fontFingerprint(data)
	if fp1 != fp2 {
		t.Fatalf("fingerprint not stable: %q vs %q", fp1, fp2)
	}
	if fp1 == "" {
		t.Fatal("empty fingerprint")
	}
}
