// Package syntax 提供纯 Go 实现的代码语法高亮与词法分词引擎
package syntax

import (
	"unicode"

	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/theme"
)

// C/C++ 跨行状态常量
const (
	StateCNormal       = 0
	StateCBlockComment = 1 // 处于 /* ... */ 跨行块注释中
)

// CColors C/C++ 语法高亮调色板
type CColors struct {
	Keyword      graphics.Color // if, else, for, while, return, class, struct...
	Type         graphics.Color // int, double, void, size_t, vector, string...
	Preprocessor graphics.Color // #include, #define, #ifdef, #pragma...
	Function     graphics.Color // printf, cout, main, myFunc...
	Constant     graphics.Color // true, false, NULL, nullptr
	String       graphics.Color // "hello", 'c', <stdio.h>
	Number       graphics.Color // 123, 0xFF, 3.14f, 100ULL
	Comment      graphics.Color // // comment, /* comment */
	Operator     graphics.Color // +, -, *, ->, ::, ==, <<, >>...
	Punctuation  graphics.Color // {, }, (, ), [, ], ;, ,, .
}

// DefaultDarkCColors 暗色现代调色板
var DefaultDarkCColors = cColorsFromSyntax(theme.Dark().SyntaxColors())

// DefaultLightCColors 浅色主题调色板
var DefaultLightCColors = cColorsFromSyntax(theme.Light().SyntaxColors())

// cColorsFromSyntax 把共享语法令牌映射为 C/C++ 调色板。
func cColorsFromSyntax(s theme.SyntaxPalette) CColors {
	return CColors{
		Keyword:      s.Keyword,
		Type:         s.Type,
		Preprocessor: s.Meta,
		Function:     s.Function,
		Constant:     s.Constant,
		String:       s.String,
		Number:       s.Number,
		Comment:      s.Comment,
		Operator:     s.Operator,
		Punctuation:  s.Punctuation,
	}
}

var (
	cKeywords = map[string]bool{
		"auto": true, "break": true, "case": true, "catch": true, "class": true,
		"const": true, "constexpr": true, "const_cast": true, "continue": true,
		"decltype": true, "default": true, "delete": true, "do": true, "dynamic_cast": true,
		"else": true, "enum": true, "explicit": true, "export": true, "extern": true,
		"final": true, "for": true, "friend": true, "goto": true, "if": true,
		"inline": true, "mutable": true, "namespace": true, "new": true, "noexcept": true,
		"operator": true, "override": true, "private": true, "protected": true,
		"public": true, "register": true, "reinterpret_cast": true, "return": true,
		"sizeof": true, "static": true, "static_assert": true, "static_cast": true,
		"struct": true, "switch": true, "template": true, "this": true, "throw": true,
		"try": true, "typedef": true, "typeid": true, "typename": true, "union": true,
		"using": true, "virtual": true, "volatile": true, "while": true,
		"concept": true, "requires": true, "co_await": true, "co_yield": true, "co_return": true,
		"restrict": true, "alignas": true, "alignof": true,
	}

	cConstants = map[string]bool{
		"true": true, "false": true, "NULL": true, "nullptr": true, "EOF": true,
	}

	cBuiltinTypes = map[string]bool{
		"void": true, "bool": true, "char": true, "wchar_t": true, "char16_t": true, "char32_t": true,
		"short": true, "int": true, "long": true, "float": true, "double": true,
		"signed": true, "unsigned": true,
		// C/C++ 常见标准类型
		"size_t": true, "ssize_t": true, "int8_t": true, "int16_t": true, "int32_t": true, "int64_t": true,
		"uint8_t": true, "uint16_t": true, "uint32_t": true, "uint64_t": true,
		"intptr_t": true, "uintptr_t": true, "ptrdiff_t": true, "nullptr_t": true,
		"string": true, "wstring": true, "string_view": true,
		"vector": true, "map": true, "unordered_map": true, "set": true, "unordered_set": true,
		"pair": true, "tuple": true, "unique_ptr": true, "shared_ptr": true, "weak_ptr": true,
		"array": true, "list": true, "deque": true, "stack": true, "queue": true,
		"optional": true, "variant": true, "any": true, "function": true,
		"thread": true, "mutex": true, "atomic": true,
	}

	cPreprocessors = map[string]bool{
		"include": true, "define": true, "undef": true, "if": true, "ifdef": true,
		"ifndef": true, "elif": true, "else": true, "endif": true, "line": true,
		"error": true, "pragma": true, "warning": true,
	}
)

// CHighlighter C/C++ 语法高亮器
type CHighlighter struct {
	isDark bool
	colors CColors
}

// NewCHighlighter 创建默认暗色 C/C++ 语法高亮器
func NewCHighlighter() *CHighlighter {
	h := &CHighlighter{
		isDark: true,
		colors: DefaultDarkCColors,
	}
	h.ApplyTheme(theme.Current())
	return h
}

// GetColors 获取当前高亮颜色配置
func (h *CHighlighter) GetColors() CColors {
	return h.colors
}

// SetColors 自定义设置高亮颜色
func (h *CHighlighter) SetColors(c CColors) {
	h.colors = c
}

// ApplyTheme 根据当前主题自动适配调色板
func (h *CHighlighter) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	h.colors = cColorsFromSyntax(t.SyntaxColors())
	h.isDark = t.Dark
}

// HighlightLine 对单行 C/C++ 源码执行状态机词法分析并生成样式片段
func (h *CHighlighter) HighlightLine(lineIndex int, line []rune, prevLineState int) ([]Span, int) {
	n := len(line)
	if n == 0 {
		return nil, prevLineState
	}

	var spans []Span
	i := 0

	// 1. 处理上一行延续的块注释状态 /* ... */
	if prevLineState == StateCBlockComment {
		closeIdx := -1
		for j := 0; j < n-1; j++ {
			if line[j] == '*' && line[j+1] == '/' {
				closeIdx = j + 2
				break
			}
		}

		if closeIdx == -1 {
			// 整行依然在块注释中
			spans = append(spans, Span{
				StartCol: 0,
				EndCol:   n,
				Color:    h.colors.Comment,
				Italic:   true,
			})
			return spans, StateCBlockComment
		}

		// 块注释在本行闭合
		spans = append(spans, Span{
			StartCol: 0,
			EndCol:   closeIdx,
			Color:    h.colors.Comment,
			Italic:   true,
		})
		i = closeIdx
	}

	// 2. 检查是否为预处理指令行 (跳过行首空白后是否为 '#')
	leadWs := 0
	for leadWs < n && unicode.IsSpace(line[leadWs]) {
		leadWs++
	}

	if i <= leadWs && leadWs < n && line[leadWs] == '#' {
		// 预处理器指令行处理
		i = leadWs
		hashStart := i
		i++ // 跳过 '#'

		// 扫描指令名称 (如 include, define 等)
		for i < n && unicode.IsSpace(line[i]) {
			i++
		}
		dirStart := i
		for i < n && (unicode.IsLetter(line[i]) || line[i] == '_') {
			i++
		}
		dirName := string(line[dirStart:i])

		spans = append(spans, Span{
			StartCol: hashStart,
			EndCol:   i,
			Color:    h.colors.Preprocessor,
			Bold:     true,
		})

		// 如果是 #include 指令，后续的 <stdio.h> 或 "myheader.h" 高亮为 String
		if dirName == "include" {
			for i < n && unicode.IsSpace(line[i]) {
				i++
			}
			if i < n && line[i] == '<' {
				incStart := i
				i++
				for i < n && line[i] != '>' {
					i++
				}
				if i < n && line[i] == '>' {
					i++
				}
				spans = append(spans, Span{
					StartCol: incStart,
					EndCol:   i,
					Color:    h.colors.String,
				})
			}
		}
	}

	// 3. 常规词法扫描
	for i < n {
		r := line[i]

		// 3.1 跳过空白字符
		if unicode.IsSpace(r) {
			i++
			continue
		}

		// 3.2 单行注释 // ...
		if r == '/' && i+1 < n && line[i+1] == '/' {
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   n,
				Color:    h.colors.Comment,
				Italic:   true,
			})
			return spans, StateCNormal
		}

		// 3.3 块注释 /* ... */
		if r == '/' && i+1 < n && line[i+1] == '*' {
			start := i
			i += 2
			closed := false
			for i < n-1 {
				if line[i] == '*' && line[i+1] == '/' {
					i += 2
					closed = true
					break
				}
				i++
			}
			if closed {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Comment,
					Italic:   true,
				})
			} else {
				// 未闭合，延续至下一行
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   n,
					Color:    h.colors.Comment,
					Italic:   true,
				})
				return spans, StateCBlockComment
			}
			continue
		}

		// 3.4 字符串字面量 "..."
		if r == '"' {
			start := i
			i++
			escaped := false
			for i < n {
				curr := line[i]
				if escaped {
					escaped = false
					i++
					continue
				}
				if curr == '\\' {
					escaped = true
					i++
					continue
				}
				if curr == '"' {
					i++
					break
				}
				i++
			}
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.String,
			})
			continue
		}

		// 3.5 字符字面量 '...'
		if r == '\'' {
			start := i
			i++
			escaped := false
			for i < n {
				curr := line[i]
				if escaped {
					escaped = false
					i++
					continue
				}
				if curr == '\\' {
					escaped = true
					i++
					continue
				}
				if curr == '\'' {
					i++
					break
				}
				i++
			}
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.String,
			})
			continue
		}

		// 3.6 数值字面量: 123, 0xFF, 0b101, 3.14f, 100ULL, 1e-4
		if unicode.IsDigit(r) || (r == '.' && i+1 < n && unicode.IsDigit(line[i+1])) {
			start := i
			isHex := false
			isBin := false

			if r == '0' && i+1 < n {
				if line[i+1] == 'x' || line[i+1] == 'X' {
					isHex = true
					i += 2
				} else if line[i+1] == 'b' || line[i+1] == 'B' {
					isBin = true
					i += 2
				}
			}

			for i < n {
				curr := line[i]
				if isHex {
					if unicode.IsDigit(curr) || (curr >= 'a' && curr <= 'f') || (curr >= 'A' && curr <= 'F') || curr == '\'' {
						i++
					} else {
						break
					}
				} else if isBin {
					if curr == '0' || curr == '1' || curr == '\'' {
						i++
					} else {
						break
					}
				} else {
					if unicode.IsDigit(curr) || curr == '.' || curr == '\'' {
						i++
					} else if curr == 'e' || curr == 'E' {
						i++
						if i < n && (line[i] == '+' || line[i] == '-') {
							i++
						}
					} else {
						break
					}
				}
			}

			// 匹配类型后缀 (如 u, l, ull, f 等)
			for i < n && (line[i] == 'u' || line[i] == 'U' || line[i] == 'l' || line[i] == 'L' || line[i] == 'f' || line[i] == 'F') {
				i++
			}

			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Number,
			})
			continue
		}

		// 3.7 标识符与关键字
		if unicode.IsLetter(r) || r == '_' {
			start := i
			for i < n && (unicode.IsLetter(line[i]) || unicode.IsDigit(line[i]) || line[i] == '_') {
				i++
			}
			word := string(line[start:i])

			if cKeywords[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Keyword,
					Bold:     true,
				})
			} else if cConstants[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Constant,
					Bold:     true,
				})
			} else if cBuiltinTypes[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Type,
				})
			} else {
				// 检查后续是否紧随 '(' 判定为函数调用/声明
				peek := i
				for peek < n && unicode.IsSpace(line[peek]) {
					peek++
				}
				if peek < n && line[peek] == '(' {
					spans = append(spans, Span{
						StartCol: start,
						EndCol:   i,
						Color:    h.colors.Function,
					})
				}
			}
			continue
		}

		// 3.8 常见多字符操作符
		if i+1 < n {
			two := string(line[i : i+2])
			if two == "->" || two == "::" || two == "++" || two == "--" ||
				two == "==" || two == "!=" || two == "<=" || two == ">=" ||
				two == "&&" || two == "||" || two == "<<" || two == ">>" ||
				two == "+=" || two == "-=" || two == "*=" || two == "/=" ||
				two == "%=" || two == "&=" || two == "|=" || two == "^=" {
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   i + 2,
					Color:    h.colors.Operator,
				})
				i += 2
				continue
			}
		}

		// 3.9 单字符操作符与标点符号
		switch r {
		case '+', '-', '*', '/', '%', '=', '<', '>', '!', '&', '|', '^', '~', '?', ':':
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   i + 1,
				Color:    h.colors.Operator,
			})
			i++
		case '(', ')', '{', '}', '[', ']', ';', ',', '.':
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   i + 1,
				Color:    h.colors.Punctuation,
			})
			i++
		default:
			i++
		}
	}

	return spans, StateCNormal
}
