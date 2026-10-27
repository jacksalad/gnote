//go:build windows

package textio

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"
)

func encodeUTF16Units(text string, bigEndian bool) []byte {
	units := utf16.Encode([]rune(text))
	out := make([]byte, 0, len(units)*2)
	for _, u := range units {
		if bigEndian {
			out = append(out, byte(u>>8), byte(u))
		} else {
			out = append(out, byte(u), byte(u>>8))
		}
	}
	return out
}

func TestDecodeUTF8AndBOM(t *testing.T) {
	text, enc, err := Decode([]byte("hello 世界"))
	if err != nil {
		t.Fatalf("decode plain utf-8: %v", err)
	}
	if text != "hello 世界" || enc.Kind != KindUTF8 {
		t.Fatalf("unexpected decode result: %q %v", text, enc)
	}

	withBOM := append([]byte{0xEF, 0xBB, 0xBF}, []byte("hello")...)
	text, enc, err = Decode(withBOM)
	if err != nil {
		t.Fatalf("decode utf-8 bom: %v", err)
	}
	if text != "hello" {
		t.Fatalf("BOM should be stripped, got %q", text)
	}
	if enc.Kind != KindUTF8BOM || enc.Name() != "UTF-8 BOM" {
		t.Fatalf("unexpected encoding: %v %q", enc.Kind, enc.Name())
	}
}

func TestDecodeUTF16WithBOM(t *testing.T) {
	body := encodeUTF16Units("中文abc", false)
	data := append([]byte{0xFF, 0xFE}, body...)
	text, enc, err := Decode(data)
	if err != nil {
		t.Fatalf("decode utf-16le: %v", err)
	}
	if text != "中文abc" || enc.Kind != KindUTF16LE {
		t.Fatalf("unexpected utf-16le decode: %q %v", text, enc)
	}

	body = encodeUTF16Units("中文abc", true)
	data = append([]byte{0xFE, 0xFF}, body...)
	text, enc, err = Decode(data)
	if err != nil {
		t.Fatalf("decode utf-16be: %v", err)
	}
	if text != "中文abc" || enc.Kind != KindUTF16BE {
		t.Fatalf("unexpected utf-16be decode: %q %v", text, enc)
	}
}

func TestDecodeUTF16WithoutBOMHeuristic(t *testing.T) {
	data := encodeUTF16Units("plain ascii text", false)
	text, enc, err := Decode(data)
	if err != nil {
		t.Fatalf("decode bom-less utf-16le: %v", err)
	}
	if text != "plain ascii text" || enc.Kind != KindUTF16LE {
		t.Fatalf("unexpected bom-less decode: %q %v", text, enc)
	}
}

func TestDecodeRejectsBinary(t *testing.T) {
	// 截断的 PNG 头：包含 NUL 但不符合 UTF-16 文本特征
	png := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 'I', 'H', 'D', 'R'}
	if _, _, err := Decode(png); !errors.Is(err, ErrBinary) {
		t.Fatalf("expected ErrBinary, got %v", err)
	}
}

func TestANSICodePageRoundTrip(t *testing.T) {
	// "中文测试" 的 GBK 字节序列（CP936）
	gbk := []byte{0xD6, 0xD0, 0xCE, 0xC4, 0xB2, 0xE2, 0xCA, 0xD4}
	text, ok := decodeCodePage(gbk, 936)
	if !ok {
		t.Fatal("decode cp936 failed")
	}
	if text != "中文测试" {
		t.Fatalf("unexpected cp936 decode: %q", text)
	}

	back, replaced, ok := encodeCodePage(text, 936)
	if !ok || replaced {
		t.Fatalf("encode cp936 failed: ok=%v replaced=%v", ok, replaced)
	}
	if string(back) != string(gbk) {
		t.Fatalf("cp936 roundtrip mismatch: % X != % X", back, gbk)
	}

	// emoji 无法用 GBK 表示，必须被报告为替换
	out, replaced, ok := encodeCodePage("abc😀", 936)
	if !ok {
		t.Fatal("encode cp936 with fallback failed")
	}
	if !replaced {
		t.Fatal("expected unrepresentable char to be reported")
	}
	if string(out) != "abc?" {
		t.Fatalf("expected fallback '?', got %q", out)
	}
}

func TestEncodeAllEncodings(t *testing.T) {
	const sample = "a中😀"

	cases := []Encoding{
		{Kind: KindUTF8},
		{Kind: KindUTF8BOM},
		{Kind: KindUTF16LE},
		{Kind: KindUTF16BE},
	}
	for _, enc := range cases {
		data, replaced, err := Encode(sample, enc)
		if err != nil || replaced {
			t.Fatalf("%s: encode failed: err=%v replaced=%v", enc.Name(), err, replaced)
		}
		text, got, err := Decode(data)
		if err != nil {
			t.Fatalf("%s: decode failed: %v", enc.Name(), err)
		}
		if text != sample {
			t.Fatalf("%s: roundtrip mismatch: %q", enc.Name(), text)
		}
		if got.Kind != enc.Kind {
			t.Fatalf("%s: detected %v", enc.Name(), got.Kind)
		}
	}
}

func TestAnalyzeEOL(t *testing.T) {
	cases := []struct {
		text string
		kind EOLKind
		name string
		seq  string
	}{
		{"a\r\nb\r\n", EOLCRLF, "CRLF", "\r\n"},
		{"a\nb\n", EOLLF, "LF", "\n"},
		{"a\rb\r", EOLCR, "CR", "\r"},
		{"a\r\nb\nc\r\n", EOLMixed, "混合", "\r\n"},
		{"single line", EOLNone, "无换行", "\r\n"},
		{"a\nb\nc\r\n", EOLMixed, "混合", "\n"},
	}
	for _, c := range cases {
		e := AnalyzeEOL(c.text)
		if e.Kind() != c.kind || e.Name() != c.name || e.Sequence() != c.seq {
			t.Fatalf("AnalyzeEOL(%q) = kind %v name %q seq %q, want %v %q %q",
				c.text, e.Kind(), e.Name(), e.Sequence(), c.kind, c.name, c.seq)
		}
	}
}

func TestStampDetectsExternalChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.txt")
	if err := os.WriteFile(path, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	stamp := Stat(path)
	if !stamp.Exists || !stamp.Same(path) {
		t.Fatal("stamp should match freshly written file")
	}

	if err := os.WriteFile(path, []byte("one plus more"), 0o644); err != nil {
		t.Fatal(err)
	}
	if stamp.Same(path) {
		t.Fatal("stamp should detect size change")
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if stamp.Same(path) {
		t.Fatal("stamp should detect deletion")
	}

	if Stat(path).Exists {
		t.Fatal("missing file must report Exists=false")
	}
}

func TestDecodeFallsBackToSystemCodePage(t *testing.T) {
	if GetACP() != 936 {
		t.Skip("system ACP is not GBK, fallback semantics differ")
	}
	text, enc, err := Decode([]byte{0xD6, 0xD0, 0xCE, 0xC4})
	if err != nil {
		t.Fatalf("decode gbk: %v", err)
	}
	if text != "中文" || enc.Kind != KindANSI || enc.Name() != "GBK" {
		t.Fatalf("unexpected gbk fallback: %q %v %q", text, enc.Kind, enc.Name())
	}
}
