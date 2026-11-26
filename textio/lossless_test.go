//go:build windows

package textio

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
)

// joinBytes 拼接字符串与原始字节。Go 字面量里的 "\xe9" 是码位 U+00E9（UTF-8 为 C3 A9），
// 要构造 Latin-1 / GBK 的原始字节必须逐字节写出；未标类型的 0xE9 传进 any 时是 int。
func joinBytes(parts ...any) []byte {
	var out []byte
	for _, p := range parts {
		switch v := p.(type) {
		case string:
			out = append(out, v...)
		case byte:
			out = append(out, v)
		case int:
			out = append(out, byte(v))
		case []byte:
			out = append(out, v...)
		default:
			panic(fmt.Sprintf("joinBytes: unsupported part %T", p))
		}
	}
	return out
}

func bom16LE(text string) []byte { return append([]byte{0xFF, 0xFE}, encodeUTF16Units(text, false)...) }
func bom16BE(text string) []byte { return append([]byte{0xFE, 0xFF}, encodeUTF16Units(text, true)...) }
func bom8(text string) []byte    { return append([]byte{0xEF, 0xBB, 0xBF}, text...) }
func ansiEnc(cp uint32) Encoding { return Encoding{Kind: KindANSI, CodePage: cp} }
func utf16Enc(kind EncodingKind, noBOM bool) Encoding {
	return Encoding{Kind: kind, NoBOM: noBOM}
}

var (
	envChinese   = decodeEnv{acp: 936, legacy: 936}   // 中文 Windows
	envWestern   = decodeEnv{acp: 1252, legacy: 1252} // 西欧 Windows
	envUTF8Beta  = decodeEnv{acp: 65001, legacy: 936} // 中文系统勾选 “Beta: 使用 Unicode UTF-8”
	envUTF8NoACP = decodeEnv{acp: 65001, legacy: 0}   // 同上但查不到传统代码页

	latin1   = joinBytes("caf", 0xE9, " r", 0xE9, "sum", 0xE9)
	cp1252   = joinBytes("he said ", 0x93, "hi", 0x94, " ", 0x97, " ok ", 0x96)
	gbkWords = []byte{0xD6, 0xD0, 0xCE, 0xC4} // “中文”
	// Big5 “這是繁體中文” 与 Shift-JIS “日本語テキスト” 的原始字节
	big5 = []byte{0xB3, 0x6F, 0xAC, 0x4F, 0xC2, 0x63, 0xC5, 0xE9, 0xA4, 0xA4, 0xA4, 0xE5, 0xB4, 0xFA, 0xB8, 0xD5}
	sjis = []byte{0x93, 0xFA, 0x96, 0x7B, 0x8C, 0xEA, 0x83, 0x65, 0x83, 0x4C, 0x83, 0x58}
	// UTF-8 文本里夹着一个坏字节：以前这一个字节就让整篇文件被当成 GBK 解成乱码
	utf8Bad = joinBytes("第一行 hello\n第二行 world ", 0xFF, " tail\n")
)

// TestDecodeLosslessTable 逐类字节样本核对：识别的编码、解码文本、是否有损，
// 并要求 Lossless 与 “Encode(Decode(x)) == x” 完全一致。
func TestDecodeLosslessTable(t *testing.T) {
	cases := []struct {
		name  string
		env   decodeEnv
		data  []byte
		want  Encoding
		text  string
		lossy bool
	}{
		{"valid utf-8", envChinese, []byte("hello 世界\n"), Encoding{Kind: KindUTF8}, "hello 世界\n", false},
		{"valid utf-8 western", envWestern, []byte("hello 世界\n"), Encoding{Kind: KindUTF8}, "hello 世界\n", false},
		{"utf-8 with bom", envChinese, bom8("hello 世界"), Encoding{Kind: KindUTF8BOM}, "hello 世界", false},
		{"mixed eol utf-8", envChinese, []byte("a\r\nb\nc\r"), Encoding{Kind: KindUTF8}, "a\r\nb\nc\r", false},

		{"utf-8 one bad byte (zh)", envChinese, utf8Bad, Encoding{Kind: KindUTF8}, "第一行 hello\n第二行 world \uFFFD tail\n", true},
		{"utf-8 one bad byte (western)", envWestern, utf8Bad, Encoding{Kind: KindUTF8}, "第一行 hello\n第二行 world \uFFFD tail\n", true},
		{"utf-8 one bad byte (utf-8 beta)", envUTF8Beta, utf8Bad, Encoding{Kind: KindUTF8}, "第一行 hello\n第二行 world \uFFFD tail\n", true},
		{"utf-8 bom + invalid byte", envChinese, joinBytes([]byte{0xEF, 0xBB, 0xBF}, "caf", 0xE9, " ok"), Encoding{Kind: KindUTF8BOM}, "caf\uFFFD ok", true},

		{"latin-1 on zh", envChinese, latin1, ansiEnc(936), "caf?r\u959Fum?", true},
		{"latin-1 on western", envWestern, latin1, ansiEnc(1252), "café résumé", false},
		{"windows-1252 on zh", envChinese, cp1252, ansiEnc(936), "he said \u63FEi??ok ?", true},
		{"windows-1252 on western", envWestern, cp1252, ansiEnc(1252), "he said \u201Chi\u201D \u2014 ok \u2013", false},

		// 误判成 GBK 时显示为乱码，但字节可以原样还原：未编辑的保存不算有损。
		{"big5 on zh", envChinese, big5, ansiEnc(936), "", false},
		{"shift-jis on zh", envChinese, sjis, ansiEnc(936), "", false},

		{"gbk on zh", envChinese, gbkWords, ansiEnc(936), "中文", false},
		{"gbk on utf-8 beta", envUTF8Beta, gbkWords, ansiEnc(936), "中文", false},
		{"gbk on utf-8 beta without legacy cp", envUTF8NoACP, gbkWords, ansiEnc(65001), "\uFFFD\uFFFD\uFFFD\uFFFD", true},
		{"gbk invalid sequence", envChinese, joinBytes(gbkWords, 0x81, 0x30, 0x41), ansiEnc(936), "中文?A", true},
		{"latin-1 on utf-8 beta", envUTF8Beta, latin1, ansiEnc(936), "caf?r\u959Fum?", true},

		{"utf-16 le with bom", envChinese, bom16LE("中文abc"), Encoding{Kind: KindUTF16LE}, "中文abc", false},
		{"utf-16 be with bom", envChinese, bom16BE("中文abc"), Encoding{Kind: KindUTF16BE}, "中文abc", false},
		{"utf-16 le odd trailing byte", envChinese, joinBytes([]byte{0xFF, 0xFE}, encodeUTF16Units("hello", false), byte(0x61)),
			Encoding{Kind: KindUTF16LE}, "hello", true},
		{"utf-16 le lone surrogate", envChinese, joinBytes([]byte{0xFF, 0xFE}, []byte{0x00, 0xD8}, "a\x00"),
			Encoding{Kind: KindUTF16LE}, "\uFFFDa", true},
		{"utf-16 le emoji (surrogate pair)", envChinese, bom16LE("a\uFF04b"),
			Encoding{Kind: KindUTF16LE}, "a\uFF04b", false},

		// 无 BOM 的 UTF-16 必须记住 “没有 BOM”，否则未编辑的保存会多出两个字节。
		{"utf-16 le no bom ascii", envChinese, encodeUTF16Units("hi\r\nthere", false), utf16Enc(KindUTF16LE, true), "hi\r\nthere", false},
		{"utf-16 le no bom cjk", envChinese, encodeUTF16Units("hello 中文\n", false), utf16Enc(KindUTF16LE, true), "hello 中文\n", false},
		{"utf-16 be no bom ascii", envChinese, encodeUTF16Units("hi\r\nthere", true), utf16Enc(KindUTF16BE, true), "hi\r\nthere", false},

		{"empty", envChinese, []byte{}, Encoding{Kind: KindUTF8}, "", false},
		{"bom only", envChinese, []byte{0xEF, 0xBB, 0xBF}, Encoding{Kind: KindUTF8BOM}, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := decodeWithEnv(c.data, c.env)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if res.Encoding != c.want {
				t.Errorf("encoding = %+v (%s), want %+v (%s)", res.Encoding, res.Encoding.Name(), c.want, c.want.Name())
			}
			if c.text != "" && res.Text != c.text {
				t.Errorf("text = %q, want %q", res.Text, c.text)
			}
			if res.Lossless != !c.lossy {
				t.Errorf("lossless = %v, want %v", res.Lossless, !c.lossy)
			}
			out, _, err := Encode(res.Text, res.Encoding)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if same := bytes.Equal(out, c.data); same != !c.lossy || same != res.Lossless {
				t.Errorf("Encode(Decode(x)) == x is %v, but lossless = %v (want lossy = %v)", same, res.Lossless, c.lossy)
			}
		})
	}
}

// 大端小端各一对合法代理项（非 BMP 字符），确认与 utf16.Decode 的结果一致。
func TestDecodeUTF16SurrogatePairs(t *testing.T) {
	const text = "emoji \U0001F600 done"
	for _, c := range []struct {
		name string
		data []byte
		want Encoding
	}{
		{"le", bom16LE(text), Encoding{Kind: KindUTF16LE}},
		{"be", bom16BE(text), Encoding{Kind: KindUTF16BE}},
	} {
		res, err := DecodeDetailed(c.data)
		if err != nil || res.Text != text || res.Encoding != c.want || !res.Lossless {
			t.Fatalf("%s: got %+v lossless=%v err=%v", c.name, res, res.Lossless, err)
		}
	}
}

// TestEncodeKeepsBOMlessUTF16 无 BOM 读入的 UTF-16 保存时不得补写 BOM。
func TestEncodeKeepsBOMlessUTF16(t *testing.T) {
	for _, kind := range []EncodingKind{KindUTF16LE, KindUTF16BE} {
		withBOM, _, _ := Encode("hi", Encoding{Kind: kind})
		noBOM, replaced, err := Encode("hi", Encoding{Kind: kind, NoBOM: true})
		if err != nil || replaced {
			t.Fatalf("kind %d: encode err=%v replaced=%v", kind, err, replaced)
		}
		if want := withBOM[2:]; !bytes.Equal(noBOM, want) {
			t.Fatalf("kind %d: no-BOM encode = % X, want % X", kind, noBOM, want)
		}
	}
}

// TestDecodePrefersUTF8WithIsolatedBadBytes 传统编码的整段文本不能被 “基本是 UTF-8” 误判：
// 顺序扫描时它们每个双字节字符至少产生一个非法字节，合法的多字节序列反而稀少。
func TestDecodePrefersUTF8WithIsolatedBadBytes(t *testing.T) {
	const zh = "这是一段用于测试编码识别的中文文本，包含常见的汉字和标点符号。第一行 hello，第二行 world。"
	const tw = "這是一段用於測試編碼識別的中文文本，包含常見的漢字和標點符號。"
	const jp = "これは文字コード判定をテストするための日本語のテキストです。一般的な漢字とひらがな。"
	const ko = "이것은 인코딩 감지를 테스트하기 위한 한국어 텍스트입니다. 일반적인 한글과 문장 부호."
	const fr = "Café résumé: l'été où il était né à Montréal, très près de la mer. «Bonjour», dit-elle."

	for _, c := range []struct {
		name string
		text string
		cp   uint32
	}{
		{"gbk", zh, 936}, {"big5", tw, 950}, {"shift-jis", jp, 932}, {"euc-kr", ko, 949}, {"cp1252", fr, 1252},
	} {
		data, replaced, ok := encodeCodePage(c.text, c.cp)
		if !ok || replaced {
			t.Fatalf("%s: cannot build sample", c.name)
		}
		res, err := decodeWithEnv(data, envChinese)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if res.Encoding.Kind != KindANSI {
			t.Errorf("%s text misclassified as %s", c.name, res.Encoding.Name())
		}
		if c.cp == 936 && (res.Text != c.text || !res.Lossless) {
			t.Errorf("gbk text should decode exactly: lossless=%v", res.Lossless)
		}
	}

	// ASCII 文本夹带少量 Latin-1 字节：没有任何合法多字节序列，仍走系统代码页。
	asciiLatin1 := joinBytes("plain ascii with one latin1 byte: ", 0xE9)
	if res, _ := decodeWithEnv(asciiLatin1, envChinese); res.Encoding.Kind != KindANSI {
		t.Errorf("latin-1 without any multibyte sequence misclassified as %s", res.Encoding.Name())
	}
	// 单个汉字被破坏：合法序列不足以说明这是 UTF-8，不强行判定。
	for _, data := range [][]byte{joinBytes([]byte{0xE4, 0xB8, 0xAD}, 0xFF), joinBytes([]byte{0xE4, 0xB8}, 0xFF, 0xAD)} {
		if res, _ := decodeWithEnv(data, envChinese); res.Encoding.Kind != KindANSI {
			t.Errorf("% X: a single broken CJK character must not count as mostly-UTF-8, got %s", data, res.Encoding.Name())
		}
	}
}

// TestLosslessFlagAlwaysMatchesRoundTrip 对任意字节串，Lossless 必须与逐字节往返比较一致，
// 无论系统 ACP 是 936、1252 还是 65001（带/不带传统代码页）。
func TestLosslessFlagAlwaysMatchesRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(20261126))
	gbk, _, _ := encodeCodePage("中文测试 abc\r\n", 936)
	seeds := [][]byte{gbk, latin1, cp1252, sjis, big5, []byte("hello 世界"), bom16LE("中文"), bom8("x")}
	envs := []decodeEnv{envChinese, envWestern, envUTF8Beta, envUTF8NoACP}
	for i := 0; i < 3000; i++ {
		data := append([]byte(nil), seeds[rng.Intn(len(seeds))]...)
		for k := rng.Intn(3) + 1; k > 0; k-- {
			data[rng.Intn(len(data))] = byte(rng.Intn(256))
		}
		for _, env := range envs {
			res, err := decodeWithEnv(data, env)
			if err != nil {
				continue
			}
			if res.Lossless != RoundTrips(data, res.Text, res.Encoding) {
				t.Fatalf("data=% X env=%+v: lossless=%v but round trip=%v (%s %q)",
					data, env, res.Lossless, RoundTrips(data, res.Text, res.Encoding), res.Encoding.Name(), res.Text)
			}
		}
	}
}

// TestLegacyANSICodePageFromSystemLocale 系统区域设置总能查出传统 ANSI 代码页（中文系统为 936）。
func TestLegacyANSICodePageFromSystemLocale(t *testing.T) {
	cp := queryLegacyANSICodePage()
	t.Logf("ACP=%d legacy ANSI code page=%d", GetACP(), cp)
	if cp == 0 || cp == 65001 {
		t.Fatalf("legacy ANSI code page = %d, want a real code page", cp)
	}
}

// TestRoundTripsAfterEncodingSwitch 切换编码后的判定：同字节的编码保持可还原，跨编码不可还原。
func TestRoundTripsAfterEncodingSwitch(t *testing.T) {
	data := []byte("hello 世界")
	res, _ := DecodeDetailed(data)
	if RoundTrips(data, res.Text, Encoding{Kind: KindUTF8BOM}) {
		t.Error("UTF-8 BOM output differs from the loaded bytes and must not count as round trip")
	}
	if RoundTrips(data, res.Text, Encoding{Kind: KindUTF16LE}) {
		t.Error("UTF-16 output differs from the loaded bytes")
	}
	if RoundTrips(data, res.Text+"\n", res.Encoding) {
		t.Error("edited text must not reproduce the loaded bytes")
	}
}

// TestDecodeAsRoundTrips 按已知编码解码（不做探测）后重新编码，无损样本必须逐字节还原。
func TestDecodeAsRoundTrips(t *testing.T) {
	gbk, _, _ := Encode("中文 abc\r\n", Encoding{Kind: KindANSI, CodePage: 936})
	utf16NoBOM, _, _ := Encode("plain ascii\r\n", Encoding{Kind: KindUTF16LE, NoBOM: true})
	for _, c := range []struct {
		name string
		data []byte
		enc  Encoding
	}{
		{"utf-8", []byte("hello 世界\r\n"), Encoding{Kind: KindUTF8}},
		{"utf-8 bom", bom8("hello 世界"), Encoding{Kind: KindUTF8BOM}},
		{"utf-16 le", bom16LE("中文abc"), Encoding{Kind: KindUTF16LE}},
		{"utf-16 be", bom16BE("中文abc"), Encoding{Kind: KindUTF16BE}},
		{"utf-16 le no bom", utf16NoBOM, Encoding{Kind: KindUTF16LE, NoBOM: true}},
		{"gbk", gbk, Encoding{Kind: KindANSI, CodePage: 936}},
	} {
		text, err := DecodeAs(c.data, c.enc)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !RoundTrips(c.data, text, c.enc) {
			t.Errorf("%s: DecodeAs + Encode does not reproduce the bytes: %q", c.name, text)
		}
	}
	if text, err := DecodeAs(bom16LE("abc"), Encoding{Kind: KindUTF16LE, NoBOM: true}); err != nil || text != "abc" {
		t.Errorf("a BOM in the bytes is skipped: %q err=%v", text, err)
	}
	if _, err := DecodeAs([]byte("x"), Encoding{Kind: KindANSI, CodePage: 99999}); err == nil {
		t.Errorf("an unknown code page must report an error")
	}
}

// TestPublicDecodeMatchesDetailed 公开入口 Decode 与 DecodeDetailed 给出相同的文本与编码。
func TestPublicDecodeMatchesDetailed(t *testing.T) {
	for _, data := range [][]byte{utf8Bad, latin1, bom16LE("中文"), encodeUTF16Units("abc", true), []byte("plain")} {
		text, enc, err := Decode(data)
		res, err2 := DecodeDetailed(data)
		if err != err2 || text != res.Text || enc != res.Encoding {
			t.Fatalf("Decode and DecodeDetailed disagree on % X", data)
		}
	}
	if _, _, err := Decode([]byte{0x00, 0x01, 0x02}); err != ErrBinary {
		t.Fatalf("binary input: err = %v, want ErrBinary", err)
	}
	if res, err := DecodeDetailed([]byte("x")); err != nil || res.Text != "x" || !res.Lossless {
		t.Fatalf("plain input: %+v err=%v", res, err)
	}
}
