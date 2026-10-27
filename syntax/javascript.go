// Package syntax 提供纯 Go 实现的代码语法高亮与词法分词引擎
package syntax

import (
	"unicode"

	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/theme"
)

// JavaScript 跨行状态常量
const (
	StateJSNormal         = 0
	StateJSBlockComment   = 1 // 处于 /* ... */ 跨行注释中
	StateJSTemplateString = 2 // 处于 `...` 跨行模板字符串中
)

// JSColors JavaScript 语法高亮调色板
type JSColors struct {
	Keyword     graphics.Color // function, class, const, let, return, if, else, async, await...
	Builtin     graphics.Color // console, window, document, Promise, Object, Array, Math...
	Type        graphics.Color // String, Number, Boolean, BigInt, Symbol...
	Function    graphics.Color // 函数定义与调用
	Constant    graphics.Color // true, false, null, undefined, NaN, Infinity
	String      graphics.Color // "str", 'str', `template`
	Number      graphics.Color // 123, 0xFF, 3.14, 100n
	Comment     graphics.Color // // comment, /* comment */
	Operator    graphics.Color // +, -, *, ===, !==, =>, ??, ?., &&, ||...
	Punctuation graphics.Color // {, }, (, ), [, ], ;, ,, .
}

// DefaultDarkJSColors 暗色现代调色板
var DefaultDarkJSColors = jsColorsFromSyntax(theme.Dark().SyntaxColors())

// DefaultLightJSColors 浅色主题调色板
var DefaultLightJSColors = jsColorsFromSyntax(theme.Light().SyntaxColors())

// jsColorsFromSyntax 把共享语法令牌映射为 JavaScript 调色板。
func jsColorsFromSyntax(s theme.SyntaxPalette) JSColors {
	return JSColors{
		Keyword:     s.Keyword,
		Builtin:     s.Builtin,
		Type:        s.Type,
		Function:    s.Function,
		Constant:    s.Constant,
		String:      s.String,
		Number:      s.Number,
		Comment:     s.Comment,
		Operator:    s.Operator,
		Punctuation: s.Punctuation,
	}
}

var (
	jsKeywords = map[string]bool{
		"async": true, "await": true, "break": true, "case": true, "catch": true,
		"class": true, "const": true, "continue": true, "debugger": true, "default": true,
		"delete": true, "do": true, "else": true, "export": true, "extends": true,
		"finally": true, "for": true, "from": true, "function": true, "if": true,
		"import": true, "in": true, "instanceof": true, "let": true, "new": true,
		"of": true, "return": true, "static": true, "super": true, "switch": true,
		"this": true, "throw": true, "try": true, "typeof": true, "var": true,
		"void": true, "while": true, "with": true, "yield": true, "as": true,
	}

	jsConstants = map[string]bool{
		"true": true, "false": true, "null": true, "undefined": true,
		"NaN": true, "Infinity": true,
	}

	jsBuiltinObjects = map[string]bool{
		"console": true, "window": true, "document": true, "global": true, "globalThis": true,
		"process": true, "module": true, "exports": true, "require": true,
		"Math": true, "JSON": true, "Reflect": true, "Proxy": true, "Intl": true,
		"Promise": true, "Map": true, "Set": true, "WeakMap": true, "WeakSet": true,
		"Array": true, "Object": true, "Function": true, "Date": true, "RegExp": true,
		"Error": true, "TypeError": true, "RangeError": true, "SyntaxError": true,
	}

	jsBuiltinTypes = map[string]bool{
		"String": true, "Number": true, "Boolean": true, "Symbol": true, "BigInt": true,
		"ArrayBuffer": true, "SharedArrayBuffer": true, "DataView": true,
		"Int8Array": true, "Uint8Array": true, "Uint8ClampedArray": true,
		"Int16Array": true, "Uint16Array": true, "Int32Array": true, "Uint32Array": true,
		"Float32Array": true, "Float64Array": true, "BigInt64Array": true, "BigUint64Array": true,
	}
)

// JSHighlighter JavaScript 语法高亮器
type JSHighlighter struct {
	isDark bool
	colors JSColors
}

// NewJSHighlighter 创建默认 JavaScript 语法高亮器
func NewJSHighlighter() *JSHighlighter {
	h := &JSHighlighter{
		isDark: true,
		colors: DefaultDarkJSColors,
	}
	h.ApplyTheme(theme.Current())
	return h
}

// GetColors 获取当前高亮颜色配置
func (h *JSHighlighter) GetColors() JSColors {
	return h.colors
}

// SetColors 自定义设置高亮颜色
func (h *JSHighlighter) SetColors(c JSColors) {
	h.colors = c
}

// ApplyTheme 根据当前主题自动适配调色板
func (h *JSHighlighter) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	h.colors = jsColorsFromSyntax(t.SyntaxColors())
	h.isDark = t.Dark
}

// HighlightLine 对单行 JavaScript 源码执行状态机词法分析并生成样式片段
func (h *JSHighlighter) HighlightLine(lineIndex int, line []rune, prevLineState int) ([]Span, int) {
	n := len(line)
	if n == 0 {
		return nil, prevLineState
	}

	var spans []Span
	i := 0

	// 1. 处理上一行延续的跨行状态
	if prevLineState == StateJSBlockComment {
		// 处于 /* ... */ 跨行注释中
		closeIdx := -1
		for j := 0; j < n-1; j++ {
			if line[j] == '*' && line[j+1] == '/' {
				closeIdx = j + 2
				break
			}
		}

		if closeIdx == -1 {
			spans = append(spans, Span{
				StartCol: 0,
				EndCol:   n,
				Color:    h.colors.Comment,
				Italic:   true,
			})
			return spans, StateJSBlockComment
		}

		spans = append(spans, Span{
			StartCol: 0,
			EndCol:   closeIdx,
			Color:    h.colors.Comment,
			Italic:   true,
		})
		i = closeIdx
	} else if prevLineState == StateJSTemplateString {
		// 处于 `...` 跨行模板字符串中
		closeIdx := -1
		for j := 0; j < n; j++ {
			if line[j] == '`' {
				slashCount := 0
				for k := j - 1; k >= 0 && line[k] == '\\'; k-- {
					slashCount++
				}
				if slashCount%2 == 0 {
					closeIdx = j + 1
					break
				}
			}
		}

		if closeIdx == -1 {
			spans = append(spans, Span{
				StartCol: 0,
				EndCol:   n,
				Color:    h.colors.String,
			})
			return spans, StateJSTemplateString
		}

		spans = append(spans, Span{
			StartCol: 0,
			EndCol:   closeIdx,
			Color:    h.colors.String,
		})
		i = closeIdx
	}

	// 2. 常规词法扫描
	for i < n {
		r := line[i]

		// 2.1 空白字符
		if unicode.IsSpace(r) {
			i++
			continue
		}

		// 2.2 单行注释 // ...
		if r == '/' && i+1 < n && line[i+1] == '/' {
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   n,
				Color:    h.colors.Comment,
				Italic:   true,
			})
			return spans, StateJSNormal
		}

		// 2.3 块注释 /* ... */
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
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   n,
					Color:    h.colors.Comment,
					Italic:   true,
				})
				return spans, StateJSBlockComment
			}
			continue
		}

		// 2.4 反引号模板字符串 `...`
		if r == '`' {
			start := i
			i++
			closed := false
			for i < n {
				if line[i] == '`' {
					slashCount := 0
					for k := i - 1; k >= start && line[k] == '\\'; k-- {
						slashCount++
					}
					if slashCount%2 == 0 {
						i++
						closed = true
						break
					}
				}
				i++
			}
			if closed {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.String,
				})
			} else {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   n,
					Color:    h.colors.String,
				})
				return spans, StateJSTemplateString
			}
			continue
		}

		// 2.5 双引号字符串 "..."
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

		// 2.6 单引号字符串 '...'
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

		// 2.7 数值: 123, 0xFF, 0b101, 0o755, 3.14, 100n, 1_000_000
		if unicode.IsDigit(r) || (r == '.' && i+1 < n && unicode.IsDigit(line[i+1])) {
			start := i
			isHex := false
			isBin := false
			isOct := false

			if r == '0' && i+1 < n {
				if line[i+1] == 'x' || line[i+1] == 'X' {
					isHex = true
					i += 2
				} else if line[i+1] == 'b' || line[i+1] == 'B' {
					isBin = true
					i += 2
				} else if line[i+1] == 'o' || line[i+1] == 'O' {
					isOct = true
					i += 2
				}
			}

			for i < n {
				curr := line[i]
				if isHex {
					if unicode.IsDigit(curr) || (curr >= 'a' && curr <= 'f') || (curr >= 'A' && curr <= 'F') || curr == '_' {
						i++
					} else {
						break
					}
				} else if isBin {
					if curr == '0' || curr == '1' || curr == '_' {
						i++
					} else {
						break
					}
				} else if isOct {
					if (curr >= '0' && curr <= '7') || curr == '_' {
						i++
					} else {
						break
					}
				} else {
					if unicode.IsDigit(curr) || curr == '.' || curr == '_' {
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

			// BigInt 后缀 'n'
			if i < n && line[i] == 'n' {
				i++
			}

			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Number,
			})
			continue
		}

		// 2.8 标识符与关键字
		if unicode.IsLetter(r) || r == '_' || r == '$' {
			start := i
			for i < n && (unicode.IsLetter(line[i]) || unicode.IsDigit(line[i]) || line[i] == '_' || line[i] == '$') {
				i++
			}
			word := string(line[start:i])

			if jsKeywords[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Keyword,
					Bold:     true,
				})
			} else if jsConstants[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Constant,
					Bold:     true,
				})
			} else if jsBuiltinObjects[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Builtin,
				})
			} else if jsBuiltinTypes[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Type,
				})
			} else {
				// 检查后续是否紧随 '(' 判定为函数调用
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

		// 2.9 三字符操作符
		if i+2 < n {
			three := string(line[i : i+3])
			if three == "===" || three == "!==" || three == ">>>" ||
				three == "**=" || three == "&&=" || three == "||=" ||
				three == "??=" || three == "<<=" || three == ">>=" ||
				three == "..." {
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   i + 3,
					Color:    h.colors.Operator,
				})
				i += 3
				continue
			}
		}

		// 2.10 双字符操作符
		if i+1 < n {
			two := string(line[i : i+2])
			if two == "==" || two == "!=" || two == "<=" || two == ">=" ||
				two == "=>" || two == "++" || two == "--" ||
				two == "&&" || two == "||" || two == "??" || two == "?." ||
				two == "**" || two == "<<" || two == ">>" ||
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

		// 2.11 单字符操作符与标点符号
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

	return spans, StateJSNormal
}
