//go:build windows

// Package textio 提供带编码与换行符保真的文本文件读写能力。
//
// 设计目标：
//   - 打开文件时识别 BOM/UTF-8/UTF-16/ANSI(系统代码页) 编码，避免把 GBK 文本当成 UTF-8 读入后写回损坏原文件；
//   - 记录文件原始换行风格（CRLF/LF/CR/混合），保存时按原风格与编码回写；
//   - 依赖 Windows kernel32 的代码页转换接口，零 CGO、零外部依赖。
package textio

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"
)

// EncodingKind 文本编码类别
type EncodingKind int

const (
	KindUTF8    EncodingKind = iota // 无 BOM 的 UTF-8
	KindUTF8BOM                     // 带 BOM 的 UTF-8
	KindUTF16LE                     // 带 BOM 的小端 UTF-16
	KindUTF16BE                     // 带 BOM 的大端 UTF-16
	KindANSI                        // 系统代码页（中文 Windows 上即 GBK/CP936）
)

// Encoding 文本编码。ANSI 时 CodePage 记录实际代码页。
type Encoding struct {
	Kind     EncodingKind
	CodePage uint32
	// NoBOM 仅对 UTF-16 有意义：输入没有 BOM 时置位，保存时同样不补写 BOM，
	// 保证未编辑的保存能逐字节还原。
	NoBOM bool
}

// Name 返回适合状态栏展示的编码名称
func (e Encoding) Name() string {
	switch e.Kind {
	case KindUTF8:
		return "UTF-8"
	case KindUTF8BOM:
		return "UTF-8 BOM"
	case KindUTF16LE:
		return "UTF-16 LE"
	case KindUTF16BE:
		return "UTF-16 BE"
	default:
		return codePageName(e.CodePage)
	}
}

// IsANSI 判断是否为系统代码页编码
func (e Encoding) IsANSI() bool {
	return e.Kind == KindANSI
}

// DefaultEncoding 新文档默认编码
func DefaultEncoding() Encoding {
	return Encoding{Kind: KindUTF8}
}

// codePageName 常见代码页的友好名称
func codePageName(cp uint32) string {
	switch cp {
	case 936:
		return "GBK"
	case 54936:
		return "GB18030"
	case 950:
		return "Big5"
	case 932:
		return "Shift-JIS"
	case 949:
		return "EUC-KR"
	case 1252:
		return "Windows-1252"
	case 65001:
		return "UTF-8"
	case 0:
		return "ANSI"
	default:
		return fmt.Sprintf("ANSI (CP%d)", cp)
	}
}

var (
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	multiByteToWideChar        = kernel32.NewProc("MultiByteToWideChar")
	wideCharToMultiByte        = kernel32.NewProc("WideCharToMultiByte")
	getACPProc                 = kernel32.NewProc("GetACP")
	cpACP               uint32 = 0
	cpUTF8              uint32 = 65001

	// wcNoBestFitChars 禁止最佳拟合替换，配合 lpUsedDefaultChar 精确检测不可表示字符
	wcNoBestFitChars uintptr = 0x00000400

	// MB_ERR_INVALID_CHARS：输入含非法序列时让 MultiByteToWideChar 失败而不是替换成 '?'
	mbErrInvalidChars uintptr = 0x00000008

	// LOCALE_IDEFAULTANSICODEPAGE：区域设置对应的传统 ANSI 代码页
	localeIDefaultANSICodePage uintptr = 0x1004

	getSystemDefaultLocaleName = kernel32.NewProc("GetSystemDefaultLocaleName")
	getLocaleInfoEx            = kernel32.NewProc("GetLocaleInfoEx")
	legacyACP                  uint32
)

// GetACP 返回当前系统 ANSI 代码页（中文 Windows 为 936）
func GetACP() uint32 {
	if cpACP != 0 {
		return cpACP
	}
	ret, _, _ := getACPProc.Call()
	cpACP = uint32(ret)
	if cpACP == 0 {
		cpACP = 1252
	}
	return cpACP
}

// LegacyANSICodePage 返回系统区域设置对应的传统 ANSI 代码页（中文 Windows 为 936）。
// 勾选 “Beta: 使用 Unicode UTF-8 提供全球语言支持” 后 GetACP 返回 65001，
// 旧的 GBK / Shift-JIS 文件仍要靠这里的代码页才能解码。
func LegacyANSICodePage() uint32 {
	if legacyACP != 0 {
		return legacyACP
	}
	legacyACP = queryLegacyANSICodePage()
	return legacyACP
}

// queryLegacyANSICodePage 经 GetSystemDefaultLocaleName + GetLocaleInfoEx 查询传统 ANSI 代码页；失败返回 0。
func queryLegacyANSICodePage() uint32 {
	name := make([]uint16, 85) // LOCALE_NAME_MAX_LENGTH
	n, _, _ := getSystemDefaultLocaleName.Call(uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
	if n <= 1 {
		return 0
	}
	buf := make([]uint16, 16)
	m, _, _ := getLocaleInfoEx.Call(uintptr(unsafe.Pointer(&name[0])), localeIDefaultANSICodePage,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if m <= 1 {
		return 0
	}
	cp, err := strconv.ParseUint(syscall.UTF16ToString(buf[:m]), 10, 32)
	if err != nil {
		return 0
	}
	return uint32(cp)
}

// ErrBinary 文件内容看起来是二进制而非文本
var ErrBinary = errors.New("file contains binary data")

// DecodeResult 解码结果与保真信息。
type DecodeResult struct {
	Text     string
	Encoding Encoding
	// Lossless 为 true 表示把 Text 按 Encoding 重新编码可逐字节还原输入。
	// 为 false 说明解码时替换或丢弃了字节（非法序列、落单代理项、奇数尾字节），
	// 原样保存会改写用户没有见过的内容，调用方应提示确认。
	Lossless bool
}

// Decode 自动探测编码并解码为 UTF-8 字符串
func Decode(data []byte) (string, Encoding, error) {
	res, err := DecodeDetailed(data)
	if err != nil {
		return "", Encoding{}, err
	}
	return res.Text, res.Encoding, nil
}

// DecodeDetailed 同 Decode，并报告解码结果能否逐字节还原。
func DecodeDetailed(data []byte) (DecodeResult, error) {
	return decodeWithEnv(data, systemDecodeEnv())
}

// decodeEnv 解码所依赖的系统代码页环境；抽成参数便于测试模拟不同语言版本的 Windows。
type decodeEnv struct {
	acp    uint32 // 系统 ACP
	legacy uint32 // 传统 ANSI 代码页，ACP 为 65001 时代替它解码；0 表示未知
}

func systemDecodeEnv() decodeEnv {
	return decodeEnv{acp: GetACP(), legacy: LegacyANSICodePage()}
}

// RoundTrips 判断把 text 按 enc 编码能否逐字节还原 data。
// 用户切换编码后用它重新判定保存是否会改写载入时的字节。
func RoundTrips(data []byte, text string, enc Encoding) bool {
	out, _, err := Encode(text, enc)
	return err == nil && bytes.Equal(out, data)
}

// DecodeAs 按已知编码解码，不做探测（UTF-16 开头的 BOM 会被跳过）。
// 用来重新得到 “文件现状” 对应的文本，再配合 RoundTrips 判断换一种编码写出是否改动字节。
func DecodeAs(data []byte, enc Encoding) (string, error) {
	switch enc.Kind {
	case KindUTF8:
		return string(data), nil
	case KindUTF8BOM:
		return string(bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})), nil
	case KindUTF16LE, KindUTF16BE:
		body := data
		if len(body) >= 2 && ((enc.Kind == KindUTF16LE && body[0] == 0xFF && body[1] == 0xFE) ||
			(enc.Kind == KindUTF16BE && body[0] == 0xFE && body[1] == 0xFF)) {
			body = body[2:]
		}
		text, _ := decodeUTF16(body, enc.Kind == KindUTF16BE)
		return text, nil
	case KindANSI:
		cp := enc.CodePage
		if cp == 0 {
			cp = GetACP()
		}
		text, _, ok := decodeCodePageChecked(data, cp)
		if !ok {
			return "", fmt.Errorf("cannot decode text with code page %d", cp)
		}
		return text, nil
	}
	return string(data), nil
}

// decodeWithEnv 自动探测编码并解码；系统代码页来自 env。
func decodeWithEnv(data []byte, env decodeEnv) (DecodeResult, error) {
	if res, ok := decodeWithBOM(data); ok {
		return res, nil
	}

	// 无 BOM 时：NUL 字节意味着 UTF-16（无 BOM）或二进制，纯文本编码不会产生 NUL
	if hasNUL(data) {
		if text, enc, lossless, ok := decodeUTF16NoBOM(data); ok {
			return DecodeResult{Text: text, Encoding: enc, Lossless: lossless}, nil
		}
		return DecodeResult{}, ErrBinary
	}

	if utf8.Valid(data) {
		return DecodeResult{Text: string(data), Encoding: Encoding{Kind: KindUTF8}, Lossless: true}, nil
	}
	// 含合法多字节序列、只夹带个别非法字节的 UTF-8：坏字节替换为 U+FFFD 后仍按 UTF-8 读入。
	// utf8.Valid 是全有或全无的，以前一个坏字节就会把整篇 UTF-8 送去代码页解码成乱码。
	if mostlyUTF8(data) {
		return DecodeResult{Text: replaceInvalidUTF8(data), Encoding: Encoding{Kind: KindUTF8}, Lossless: false}, nil
	}

	codePage := env.acp
	if codePage == cpUTF8 && env.legacy != 0 && env.legacy != cpUTF8 {
		// ACP 为 65001 的系统：非 UTF-8 字节按 65001 解码只会得到满篇 U+FFFD，
		// 改用系统区域设置的传统代码页（中文即 GBK）。
		codePage = env.legacy
	}
	text, lossless, ok := decodeCodePageChecked(data, codePage)
	if !ok {
		return DecodeResult{}, errors.New("unsupported encoding: neither valid UTF-8 nor decodable by system code page")
	}
	return DecodeResult{Text: text, Encoding: Encoding{Kind: KindANSI, CodePage: codePage}, Lossless: lossless}, nil
}

// decodeWithBOM 按 BOM 解码
func decodeWithBOM(data []byte) (DecodeResult, bool) {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		// 畸形 UTF-8 用 U+FFFD 兜底，保证文件仍可打开且不会崩溃；
		// 是否真的发生过替换由 Lossless 报告，保存前据此提示用户。
		body := data[3:]
		if utf8.Valid(body) {
			return DecodeResult{Text: string(body), Encoding: Encoding{Kind: KindUTF8BOM}, Lossless: true}, true
		}
		return DecodeResult{Text: replaceInvalidUTF8(body), Encoding: Encoding{Kind: KindUTF8BOM}, Lossless: false}, true
	}
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
		text, lossless := decodeUTF16(data[2:], false)
		return DecodeResult{Text: text, Encoding: Encoding{Kind: KindUTF16LE}, Lossless: lossless}, true
	}
	if len(data) >= 2 && data[0] == 0xFE && data[1] == 0xFF {
		text, lossless := decodeUTF16(data[2:], true)
		return DecodeResult{Text: text, Encoding: Encoding{Kind: KindUTF16BE}, Lossless: lossless}, true
	}
	return DecodeResult{}, false
}

// isUTF8Cont 判断 b 是否为 UTF-8 的后续字节（10xxxxxx）。
func isUTF8Cont(b byte) bool {
	return b&0xC0 == 0x80
}

// utf8SeqLen 返回 data[i:] 处一条合法 UTF-8 序列的长度；非法（孤立的后续字节、越界或截断）返回 0。
func utf8SeqLen(data []byte, i int) int {
	b := data[i]
	switch {
	case b < 0x80:
		return 1
	case b < 0xC2: // 0x80–0xBF 是孤立的后续字节，0xC0/0xC1 永远非法
	case b < 0xE0:
		if i+1 < len(data) && isUTF8Cont(data[i+1]) {
			return 2
		}
	case b < 0xF0:
		if i+2 < len(data) && isUTF8Cont(data[i+1]) && isUTF8Cont(data[i+2]) &&
			!(b == 0xE0 && data[i+1] < 0xA0) && !(b == 0xED && data[i+1] > 0x9F) {
			return 3
		}
	case b <= 0xF4:
		if i+3 < len(data) && isUTF8Cont(data[i+1]) && isUTF8Cont(data[i+2]) && isUTF8Cont(data[i+3]) &&
			!(b == 0xF0 && data[i+1] < 0x90) && !(b == 0xF4 && data[i+1] > 0x8F) {
			return 4
		}
	}
	return 0
}

// utf8ProbeLimit 判定“是否基本是 UTF-8”时最多扫描文件开头的字节数。
// 传统编码的顺序扫描约为 1.5ns/字节，10MB 的 GBK 文件要多花 15ms；只看开头 64KB 可忽略不计。
const utf8ProbeLimit = 64 << 10

// mostlyUTF8 判断输入是否为“夹带个别非法字节的 UTF-8”：至少有两条合法的多字节序列，
// 且非法字节不超过合法序列的一半。传统双字节编码（GBK/Big5/Shift-JIS/EUC-KR）顺序扫描时
// 每个汉字至少产生一个非法字节、合法的多字节序列反而稀少（实测非法字节约为合法序列的 3～7 倍），
// Latin-1 / Windows-1252 文本则几乎没有合法的多字节序列，因此都不会被误判。
func mostlyUTF8(data []byte) bool {
	if len(data) > utf8ProbeLimit {
		// 截断点退回到 ASCII 边界，避免把一条多字节序列从中间切开算作非法字节
		limit := utf8ProbeLimit
		for n := 0; n < 4 && limit > 0 && data[limit-1] >= utf8.RuneSelf; n++ {
			limit--
		}
		data = data[:limit]
	}
	valid, invalid := 0, 0
	for i := 0; i < len(data); {
		b := data[i]
		size := 0
		switch {
		case b < 0x80: // ASCII
			i++
			continue
		case b < 0xE0: // 孤立的后续字节 / 0xC0 / 0xC1，或一条双字节序列
			if b >= 0xC2 && i+1 < len(data) && isUTF8Cont(data[i+1]) {
				size = 2
			}
		case b <= 0xF4:
			size = utf8SeqLen(data, i)
		}
		if size == 0 {
			invalid++
			i++
		} else {
			valid++
			i += size
		}
	}
	return valid >= 2 && invalid*2 <= valid
}

// replaceInvalidUTF8 把夹带非法字节的 UTF-8 转成合法字符串，非法字节替换为 U+FFFD。
// string([]byte) 会原样保留非法字节（直到编辑器转成 []rune 时才变成 U+FFFD），
// 解码结果必须自己保证是合法 UTF-8，Lossless 标志才有意义。
func replaceInvalidUTF8(data []byte) string {
	first := -1
	for i := 0; i < len(data); {
		size := utf8SeqLen(data, i)
		if size == 0 {
			first = i
			break
		}
		i += size
	}
	if first < 0 {
		return string(data)
	}
	var b strings.Builder
	b.Grow(len(data) + 3)
	b.Write(data[:first])
	// 合法区间整段拷贝，只在非法字节处停下写入 U+FFFD
	for start := first; start < len(data); {
		i := start
		for i < len(data) {
			size := utf8SeqLen(data, i)
			if size == 0 {
				break
			}
			i += size
		}
		b.Write(data[start:i])
		if i < len(data) {
			b.WriteRune(utf8.RuneError)
			i++
		}
		start = i
	}
	return b.String()
}

// decodeUTF16 解码 UTF-16 字节流。合法输入与 utf16.Decode 结果一致；
// 落单的代理项和末尾多出的奇数个字节替换为 U+FFFD，此时 lossless 为 false。
func decodeUTF16(data []byte, bigEndian bool) (text string, lossless bool) {
	n := len(data) / 2
	unit := func(i int) uint16 {
		if bigEndian {
			return uint16(data[i*2])<<8 | uint16(data[i*2+1])
		}
		return uint16(data[i*2]) | uint16(data[i*2+1])<<8
	}
	runes := make([]rune, 0, n)
	lossless = len(data)%2 == 0
	for i := 0; i < n; i++ {
		u := unit(i)
		switch {
		case u >= 0xD800 && u <= 0xDBFF && i+1 < n && unit(i+1) >= 0xDC00 && unit(i+1) <= 0xDFFF:
			runes = append(runes, utf16.DecodeRune(rune(u), rune(unit(i+1))))
			i++
		case u >= 0xD800 && u <= 0xDFFF:
			runes = append(runes, utf8.RuneError)
			lossless = false
		default:
			runes = append(runes, rune(u))
		}
	}
	return string(runes), lossless
}

// decodeUTF16NoBOM 对无 BOM 的 UTF-16 做保守启发式识别（ASCII 文本高频出现 0x00 字节）。
// 识别成功时 Encoding.NoBOM 置位，保存不补写 BOM，未编辑的保存才能逐字节还原。
func decodeUTF16NoBOM(data []byte) (string, Encoding, bool, bool) {
	if len(data) < 4 || len(data)%2 != 0 {
		return "", Encoding{}, false, false
	}
	units := len(data) / 2
	var zeroEven, zeroOdd int
	for i := 0; i < units; i++ {
		if data[i*2] == 0 {
			zeroEven++
		}
		if data[i*2+1] == 0 {
			zeroOdd++
		}
	}
	// UTF-16LE 的 ASCII 文本：低字节非零、高字节为零
	if zeroOdd*10 >= units*7 && zeroEven*10 <= units*2 {
		if text, lossless := decodeUTF16(data, false); looksLikeText(text) {
			return text, Encoding{Kind: KindUTF16LE, NoBOM: true}, lossless, true
		}
	}
	if zeroEven*10 >= units*7 && zeroOdd*10 <= units*2 {
		if text, lossless := decodeUTF16(data, true); looksLikeText(text) {
			return text, Encoding{Kind: KindUTF16BE, NoBOM: true}, lossless, true
		}
	}
	return "", Encoding{}, false, false
}

// looksLikeText 判断解码结果是否像正常文本（控制字符占比不超过 2%），用于否决错误的 UTF-16 猜测
func looksLikeText(s string) bool {
	if s == "" {
		return true
	}
	bad, total := 0, 0
	for _, r := range s {
		total++
		if r == '\t' || r == '\r' || r == '\n' {
			continue
		}
		if !unicode.IsPrint(r) {
			bad++
		}
	}
	return bad*50 <= total
}

// decodeCodePage 使用指定 Windows 代码页把字节流解码为 UTF-8
func decodeCodePage(data []byte, codePage uint32) (string, bool) {
	text, _, ok := decodeCodePageChecked(data, codePage)
	return text, ok
}

// codePagesWithDuplicateMappings 这些代码页里有多组字节映射到同一个 Unicode 字符
// （CP932 的 NEC 选定 IBM 扩展区、CP950 的重复符号），解码合法也不保证往返后字节一致。
func codePagesWithDuplicateMappings(codePage uint32) bool {
	return codePage == 932 || codePage == 950
}

// decodeCodePageChecked 解码并报告能否逐字节还原。
// 先带 MB_ERR_INVALID_CHARS 探测：它同时充当长度查询，输入全部合法时没有额外开销；
// 探测失败或代码页存在重复映射时，再退回宽松解码（非法序列变成 '?'）并做一次往返比较，
// 因为个别非法字节（如 CP936 中单独的 0xFF）经用户自定义区映射其实可以原样还原。
func decodeCodePageChecked(data []byte, codePage uint32) (text string, lossless bool, ok bool) {
	if len(data) == 0 {
		return "", true, true
	}
	strict, strictOK := multiByteToWide(data, codePage, mbErrInvalidChars)
	if strictOK && !codePagesWithDuplicateMappings(codePage) {
		return strict, true, true
	}
	text = strict
	if !strictOK {
		if text, ok = multiByteToWide(data, codePage, 0); !ok {
			return "", false, false
		}
	}
	out, _, encOK := encodeCodePage(text, codePage)
	return text, encOK && bytes.Equal(out, data), true
}

// multiByteToWide 调用 MultiByteToWideChar 把字节流解码为 UTF-8 字符串
func multiByteToWide(data []byte, codePage uint32, flags uintptr) (string, bool) {
	cp := uintptr(codePage)
	n, _, _ := multiByteToWideChar.Call(
		cp, flags,
		uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)),
		0, 0,
	)
	if n == 0 {
		return "", false
	}
	buf := make([]uint16, n)
	written, _, _ := multiByteToWideChar.Call(
		cp, flags,
		uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)),
	)
	if written == 0 {
		return "", false
	}
	return string(utf16.Decode(buf[:written])), true
}

// encodeCodePage 使用指定 Windows 代码页编码 UTF-8 字符串。
// replaced 为 true 表示存在无法表示的字符，已用 '?' 代替。
func encodeCodePage(text string, codePage uint32) (out []byte, replaced bool, ok bool) {
	if text == "" {
		return []byte{}, false, true
	}
	// 非 BMP 字符（emoji、扩展区汉字）在 ANSI 代码页中一律不可表示。
	// Windows 会把代理对拆成两个默认字符，这里先按 rune 替换为单个 '?'，保持替换语义清晰。
	text, hadNonBMP := replaceNonBMP(text)
	units := utf16.Encode([]rune(text))
	if len(units) == 0 {
		return []byte{}, hadNonBMP, true
	}

	var defaultChar byte = '?'
	var usedDefault int32
	n, _, _ := wideCharToMultiByte.Call(
		uintptr(codePage), wcNoBestFitChars,
		uintptr(unsafe.Pointer(&units[0])), uintptr(len(units)),
		0, 0,
		uintptr(unsafe.Pointer(&defaultChar)), uintptr(unsafe.Pointer(&usedDefault)),
	)
	if n == 0 {
		return nil, false, false
	}
	buf := make([]byte, n)
	written, _, _ := wideCharToMultiByte.Call(
		uintptr(codePage), wcNoBestFitChars,
		uintptr(unsafe.Pointer(&units[0])), uintptr(len(units)),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)),
		uintptr(unsafe.Pointer(&defaultChar)), uintptr(unsafe.Pointer(&usedDefault)),
	)
	if written == 0 {
		return nil, false, false
	}
	return buf[:written], usedDefault != 0 || hadNonBMP, true
}

// replaceNonBMP 将非 BMP 字符替换为 '?'，返回替换后的文本以及是否发生过替换。
func replaceNonBMP(text string) (string, bool) {
	first := -1
	for i, r := range text {
		if r > 0xFFFF {
			first = i
			break
		}
	}
	if first < 0 {
		return text, false
	}
	out := make([]rune, 0, len(text))
	for _, r := range text {
		if r > 0xFFFF {
			out = append(out, '?')
		} else {
			out = append(out, r)
		}
	}
	return string(out), true
}

// Encode 按指定编码把文本编码为字节流。
// replaced 为 true 表示有字符无法用该编码表示（已用 '?' 代替），调用方应提示用户。
func Encode(text string, enc Encoding) (out []byte, replaced bool, err error) {
	switch enc.Kind {
	case KindUTF8:
		return []byte(text), false, nil
	case KindUTF8BOM:
		out = make([]byte, 0, len(text)+3)
		out = append(out, 0xEF, 0xBB, 0xBF)
		return append(out, text...), false, nil
	case KindUTF16LE, KindUTF16BE:
		units := utf16.Encode([]rune(text))
		out = make([]byte, 0, len(units)*2+2)
		if enc.Kind == KindUTF16LE {
			if !enc.NoBOM {
				out = append(out, 0xFF, 0xFE)
			}
			for _, u := range units {
				out = append(out, byte(u), byte(u>>8))
			}
		} else {
			if !enc.NoBOM {
				out = append(out, 0xFE, 0xFF)
			}
			for _, u := range units {
				out = append(out, byte(u>>8), byte(u))
			}
		}
		return out, false, nil
	case KindANSI:
		cp := enc.CodePage
		if cp == 0 {
			cp = GetACP()
		}
		data, replaced, ok := encodeCodePage(text, cp)
		if !ok {
			return nil, false, fmt.Errorf("cannot encode text with code page %d", cp)
		}
		return data, replaced, nil
	default:
		return []byte(text), false, nil
	}
}

// hasNUL 检测 NUL 字节
func hasNUL(data []byte) bool {
	for _, b := range data {
		if b == 0 {
			return true
		}
	}
	return false
}

// EOLKind 换行风格类别
type EOLKind int

const (
	EOLCRLF  EOLKind = iota // \r\n
	EOLLF                   // \n
	EOLCR                   // \r
	EOLMixed                // 混合风格
	EOLNone                 // 无换行符
)

// EOL 换行符统计信息
type EOL struct {
	CRLF int
	LF   int
	CR   int
}

// AnalyzeEOL 统计文本中的各类换行符
func AnalyzeEOL(text string) EOL {
	var e EOL
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '\r':
			if i+1 < len(text) && text[i+1] == '\n' {
				e.CRLF++
				i++
			} else {
				e.CR++
			}
		case '\n':
			e.LF++
		}
	}
	return e
}

// Kind 返回换行风格类别
func (e EOL) Kind() EOLKind {
	nonZero := 0
	for _, n := range []int{e.CRLF, e.LF, e.CR} {
		if n > 0 {
			nonZero++
		}
	}
	switch {
	case nonZero == 0:
		return EOLNone
	case nonZero > 1:
		return EOLMixed
	case e.CRLF > 0:
		return EOLCRLF
	case e.LF > 0:
		return EOLLF
	default:
		return EOLCR
	}
}

// Name 返回适合状态栏展示的换行风格名称
func (e EOL) Name() string {
	switch e.Kind() {
	case EOLCRLF:
		return "CRLF"
	case EOLLF:
		return "LF"
	case EOLCR:
		return "CR"
	case EOLMixed:
		return "混合"
	default:
		return "无换行"
	}
}

// DefaultEOL 新文档默认换行风格
func DefaultEOL() EOL {
	return EOL{CRLF: 1}
}

// Label 返回状态栏展示文本；混合风格额外标注保存时归一的目标
func (e EOL) Label() string {
	if e.Kind() == EOLMixed {
		if e.Sequence() == "\n" {
			return "混合→LF"
		}
		return "混合→CRLF"
	}
	return e.Name()
}

// Sequence 返回保存时应使用的换行序列（混合风格按出现次数最多者归一）
func (e EOL) Sequence() string {
	if e.CRLF >= e.LF && e.CRLF >= e.CR && e.CRLF > 0 {
		return "\r\n"
	}
	if e.LF >= e.CR && e.LF > 0 {
		return "\n"
	}
	if e.CR > 0 {
		return "\r"
	}
	return "\r\n"
}

// Stamp 文件身份戳，用于检测打开期间的外部修改
type Stamp struct {
	Exists  bool
	Size    int64
	ModTime time.Time
}

// Stat 获取文件的当前身份戳
func Stat(path string) Stamp {
	fi, err := os.Stat(path)
	if err != nil {
		return Stamp{}
	}
	return Stamp{Exists: true, Size: fi.Size(), ModTime: fi.ModTime()}
}

// Equal 判断两个文件身份戳是否一致
func (s Stamp) Equal(other Stamp) bool {
	return s.Exists == other.Exists && s.Size == other.Size && s.ModTime.Equal(other.ModTime)
}

// Same 判断文件当前是否仍与戳一致（用于保存前的外部修改检测）
func (s Stamp) Same(path string) bool {
	return s.Equal(Stat(path))
}
