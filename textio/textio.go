//go:build windows

// Package textio 提供带编码与换行符保真的文本文件读写能力。
//
// 设计目标：
//   - 打开文件时识别 BOM/UTF-8/UTF-16/ANSI(系统代码页) 编码，避免把 GBK 文本当成 UTF-8 读入后写回损坏原文件；
//   - 记录文件原始换行风格（CRLF/LF/CR/混合），保存时按原风格与编码回写；
//   - 依赖 Windows kernel32 的代码页转换接口，零 CGO、零外部依赖。
package textio

import (
	"errors"
	"fmt"
	"os"
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

// ErrBinary 文件内容看起来是二进制而非文本
var ErrBinary = errors.New("file contains binary data")

// Decode 自动探测编码并解码为 UTF-8 字符串
func Decode(data []byte) (string, Encoding, error) {
	if text, enc, ok := decodeWithBOM(data); ok {
		return text, enc, nil
	}

	// 无 BOM 时：NUL 字节意味着 UTF-16（无 BOM）或二进制，纯文本编码不会产生 NUL
	if hasNUL(data) {
		if text, enc, ok := decodeUTF16NoBOM(data); ok {
			return text, enc, nil
		}
		return "", Encoding{}, ErrBinary
	}

	if utf8.Valid(data) {
		return string(data), Encoding{Kind: KindUTF8}, nil
	}

	acp := GetACP()
	text, ok := decodeCodePage(data, acp)
	if !ok {
		return "", Encoding{}, errors.New("unsupported encoding: neither valid UTF-8 nor decodable by system code page")
	}
	return text, Encoding{Kind: KindANSI, CodePage: acp}, nil
}

// decodeWithBOM 按 BOM 解码
func decodeWithBOM(data []byte) (string, Encoding, bool) {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		// 畸形 UTF-8 用 U+FFFD 兜底，保证文件仍可打开且不会崩溃
		return string(data[3:]), Encoding{Kind: KindUTF8BOM}, true
	}
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
		return decodeUTF16(data[2:], false), Encoding{Kind: KindUTF16LE}, true
	}
	if len(data) >= 2 && data[0] == 0xFE && data[1] == 0xFF {
		return decodeUTF16(data[2:], true), Encoding{Kind: KindUTF16BE}, true
	}
	return "", Encoding{}, false
}

// decodeUTF16 解码 UTF-16 字节流
func decodeUTF16(data []byte, bigEndian bool) string {
	units := make([]uint16, len(data)/2)
	for i := range units {
		if bigEndian {
			units[i] = uint16(data[i*2])<<8 | uint16(data[i*2+1])
		} else {
			units[i] = uint16(data[i*2]) | uint16(data[i*2+1])<<8
		}
	}
	return string(utf16.Decode(units))
}

// decodeUTF16NoBOM 对无 BOM 的 UTF-16 做保守启发式识别（ASCII 文本高频出现 0x00 字节）
func decodeUTF16NoBOM(data []byte) (string, Encoding, bool) {
	if len(data) < 4 || len(data)%2 != 0 {
		return "", Encoding{}, false
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
		if text := decodeUTF16(data, false); looksLikeText(text) {
			return text, Encoding{Kind: KindUTF16LE}, true
		}
	}
	if zeroEven*10 >= units*7 && zeroOdd*10 <= units*2 {
		if text := decodeUTF16(data, true); looksLikeText(text) {
			return text, Encoding{Kind: KindUTF16BE}, true
		}
	}
	return "", Encoding{}, false
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
	if len(data) == 0 {
		return "", true
	}
	n, _, _ := multiByteToWideChar.Call(
		uintptr(codePage), 0,
		uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)),
		0, 0,
	)
	if n == 0 {
		return "", false
	}
	buf := make([]uint16, n)
	written, _, _ := multiByteToWideChar.Call(
		uintptr(codePage), 0,
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
			out = append(out, 0xFF, 0xFE)
			for _, u := range units {
				out = append(out, byte(u), byte(u>>8))
			}
		} else {
			out = append(out, 0xFE, 0xFF)
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
