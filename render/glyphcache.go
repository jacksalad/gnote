//go:build windows

package render

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// 字形磁盘缓存：GDI GetGlyphOutlineW 冷启动时每字形约 50–150µs，
// 首帧通常需要数百个字形（60ms 量级）。把已光栅化的 AlphaMask 落盘，
// 下次启动直接反序列化，首帧降到毫秒级。
//
// 缓存有效性由三部分共同决定：
//  1. glyphCacheVersion：光栅化管线（GGO 标志、灰度换算）变更时手动递增；
//  2. 字体指纹：TTF 'head' 表整字体校验和（缺失时回退长度 + 首尾采样 FNV），字体更新后自动失效；
//  3. 字体键：family/size/bold/italic/alignSpace。
//
// 仅缓存通过 RegisterMemoryFont 注册的内嵌字体（指纹可确定），系统字体不落盘。
const (
	glyphCacheVersion = 1
	// maxGlyphCacheFiles 缓存目录保留的文件数上限（按最近使用时间保留），
	// 避免无级缩放遍历大量字号时无限增长。
	maxGlyphCacheFiles = 16
	glyphCacheMagic    = "GNC1"
	// maxGlyphCacheFile 单个缓存文件体积上限，防止异常会话写出超大文件。
	maxGlyphCacheFile = 16 << 20
)

// memFontIDs 记录内存字体的稳定指纹（family -> fingerprint）
var (
	memFontIDsMu sync.RWMutex
	memFontIDs   = make(map[string]string)
)

// fontFingerprint 计算字体身份：优先使用 OpenType 'head' 表的 checkSumAdjustment
// （整字体校验和，字体内容变化必然改变），否则退化为长度 + 首尾采样 FNV。
func fontFingerprint(data []byte) string {
	sum, ok := fontChecksumAdjustment(data)
	if ok && sum != 0 {
		return fmt.Sprintf("fp-%d-%08x", len(data), sum)
	}

	h := fnv.New64a()
	var size [8]byte
	binary.LittleEndian.PutUint64(size[:], uint64(len(data)))
	_, _ = h.Write(size[:])
	head := data
	if len(head) > 64<<10 {
		head = head[:64<<10]
	}
	_, _ = h.Write(head)
	if len(data) > len(head) {
		tail := data
		if len(tail) > 64<<10 {
			tail = tail[len(tail)-(64<<10):]
		}
		_, _ = h.Write(tail)
	}
	return fmt.Sprintf("fp-%d-%016x", len(data), h.Sum64())
}

// fontChecksumAdjustment 从 TTF/OTF 表目录定位 'head' 表并读取 checkSumAdjustment。
func fontChecksumAdjustment(data []byte) (uint32, bool) {
	if len(data) < 12 {
		return 0, false
	}
	// TTC 集合头无法用单表目录直接定位，交给采样回退
	if string(data[:4]) == "ttcf" {
		return 0, false
	}
	numTables := int(binary.BigEndian.Uint16(data[4:6]))
	for i := range numTables {
		off := 12 + i*16
		if off+16 > len(data) {
			return 0, false
		}
		if string(data[off:off+4]) != "head" {
			continue
		}
		headOff := int(binary.BigEndian.Uint32(data[off+8 : off+12]))
		if headOff < 0 || headOff+12 > len(data) {
			return 0, false
		}
		return binary.BigEndian.Uint32(data[headOff+8 : headOff+12]), true
	}
	return 0, false
}

func rememberMemoryFont(family string, data []byte) {
	if family == "" {
		return
	}
	memFontIDsMu.Lock()
	memFontIDs[family] = fontFingerprint(data)
	memFontIDsMu.Unlock()
}

func memoryFontID(family string) (string, bool) {
	memFontIDsMu.RLock()
	id, ok := memFontIDs[family]
	memFontIDsMu.RUnlock()
	return id, ok
}

// glyphCacheKey 组合字体指纹与字体键，作为缓存文件的逻辑标识。
func glyphCacheKey(id string, size int32, bold, italic, alignSpace bool) string {
	return fmt.Sprintf("%s|%d|%t|%t|%t", id, size, bold, italic, alignSpace)
}

// glyphCacheDirOverride 仅用于测试，生产环境为空。
var glyphCacheDirOverride string

func glyphCacheDir() (string, error) {
	if glyphCacheDirOverride != "" {
		dir := filepath.Join(glyphCacheDirOverride, "glyphcache")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		return dir, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "gnote", "glyphcache")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func glyphCachePath(key string) (string, error) {
	dir, err := glyphCacheDir()
	if err != nil {
		return "", err
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return filepath.Join(dir, fmt.Sprintf("%016x.glyphcache", h.Sum64())), nil
}

// loadGlyphCache 尝试把磁盘字形缓存装入字体面。文件缺失或损坏时静默忽略，
// 回退到按需光栅化。
func loadGlyphCache(face *FontFace) {
	path, err := glyphCachePath(face.cacheID)
	if err != nil {
		return
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() <= 0 || info.Size() > maxGlyphCacheFile {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	glyphs, err := decodeGlyphCache(data, face.cacheID)
	if err != nil || len(glyphs) == 0 {
		return
	}
	if len(glyphs) > maxGlyphsPerFace {
		return
	}
	face.cache = glyphs
}

func decodeGlyphCache(data []byte, key string) (map[rune]*Glyph, error) {
	if len(data) < 4+4+4 || string(data[:4]) != glyphCacheMagic {
		return nil, errors.New("bad magic")
	}
	if v := binary.LittleEndian.Uint32(data[4:8]); v != glyphCacheVersion {
		return nil, errors.New("version mismatch")
	}
	keyLen := int(binary.LittleEndian.Uint32(data[8:12]))
	off := 12
	if keyLen < 0 || off+keyLen+4 > len(data) || string(data[off:off+keyLen]) != key {
		return nil, errors.New("key mismatch")
	}
	off += keyLen
	count := int(binary.LittleEndian.Uint32(data[off : off+4]))
	off += 4
	if count < 0 || count > maxGlyphsPerFace {
		return nil, errors.New("bad count")
	}

	glyphs := make(map[rune]*Glyph, count)
	for range count {
		const recHeader = 4 + 5*4
		if off+recHeader > len(data) {
			return nil, errors.New("truncated record")
		}
		r := rune(binary.LittleEndian.Uint32(data[off : off+4]))
		w := int32(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		h := int32(binary.LittleEndian.Uint32(data[off+8 : off+12]))
		ox := int32(binary.LittleEndian.Uint32(data[off+12 : off+16]))
		oy := int32(binary.LittleEndian.Uint32(data[off+16 : off+20]))
		inc := int32(binary.LittleEndian.Uint32(data[off+20 : off+24]))
		off += recHeader
		if w < 0 || h < 0 || w > 512 || h > 512 {
			return nil, errors.New("bad glyph size")
		}
		maskLen := int(w) * int(h)
		if off+maskLen > len(data) {
			return nil, errors.New("truncated mask")
		}
		g := &Glyph{
			Width:        w,
			Height:       h,
			GlyphOriginX: ox,
			GlyphOriginY: oy,
			CellIncX:     inc,
		}
		if maskLen > 0 {
			g.AlphaMask = append([]byte(nil), data[off:off+maskLen]...)
		}
		off += maskLen
		glyphs[r] = g
	}
	return glyphs, nil
}

func encodeGlyphCache(key string, glyphs map[rune]*Glyph) ([]byte, error) {
	size := 12 + len(key) + 4
	for _, g := range glyphs {
		size += 4 + 5*4 + len(g.AlphaMask)
	}
	if size > maxGlyphCacheFile {
		return nil, errors.New("cache too large")
	}
	buf := make([]byte, 0, size)
	var scratch [8]byte
	buf = append(buf, glyphCacheMagic...)
	binary.LittleEndian.PutUint32(scratch[:4], glyphCacheVersion)
	buf = append(buf, scratch[:4]...)
	binary.LittleEndian.PutUint32(scratch[:4], uint32(len(key)))
	buf = append(buf, scratch[:4]...)
	buf = append(buf, key...)
	binary.LittleEndian.PutUint32(scratch[:4], uint32(len(glyphs)))
	buf = append(buf, scratch[:4]...)
	for r, g := range glyphs {
		binary.LittleEndian.PutUint32(scratch[:4], uint32(r))
		buf = append(buf, scratch[:4]...)
		for _, v := range []int32{g.Width, g.Height, g.GlyphOriginX, g.GlyphOriginY, g.CellIncX} {
			binary.LittleEndian.PutUint32(scratch[:4], uint32(v))
			buf = append(buf, scratch[:4]...)
		}
		buf = append(buf, g.AlphaMask...)
	}
	return buf, nil
}

func writeGlyphCacheFile(key string, data []byte) error {
	path, err := glyphCachePath(key)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// SaveGlyphCaches 把已光栅化字形落盘（进程退出前调用一次）并裁剪旧缓存。
// 任何失败都不影响退出流程。
func SaveGlyphCaches() {
	fm := defaultFontManager
	if fm == nil {
		return
	}
	fm.mu.RLock()
	faces := make([]*FontFace, 0, len(fm.faces))
	for _, face := range fm.faces {
		if face.cacheID != "" {
			faces = append(faces, face)
		}
	}
	fm.mu.RUnlock()

	for _, face := range faces {
		face.mu.RLock()
		if len(face.cache) == 0 {
			face.mu.RUnlock()
			continue
		}
		snapshot := make(map[rune]*Glyph, len(face.cache))
		for r, g := range face.cache {
			snapshot[r] = g
		}
		key := face.cacheID
		face.mu.RUnlock()

		data, err := encodeGlyphCache(key, snapshot)
		if err != nil {
			continue
		}
		_ = writeGlyphCacheFile(key, data)
	}
	pruneGlyphCacheFiles()
}

// pruneGlyphCacheFiles 只保留最近写入的 maxGlyphCacheFiles 个缓存文件。
func pruneGlyphCacheFiles() {
	dir, err := glyphCacheDir()
	if err != nil {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) <= maxGlyphCacheFiles {
		return
	}
	type cachedFile struct {
		path    string
		modTime int64
	}
	files := make([]cachedFile, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".glyphcache" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, cachedFile{filepath.Join(dir, e.Name()), info.ModTime().UnixNano()})
	}
	if len(files) <= maxGlyphCacheFiles {
		return
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modTime > files[j].modTime })
	for _, f := range files[maxGlyphCacheFiles:] {
		_ = os.Remove(f.path)
	}
}
