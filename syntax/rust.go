// Package syntax 提供纯 Go 实现的代码语法高亮与词法分词引擎
package syntax

import (
	"unicode"

	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/theme"
)

// Rust 跨行状态常量
const (
	StateRustNormal       = 0
	StateRustBlockComment = 1 // 处于 /* ... */ 跨行注释中
	StateRustRawString    = 2 // 处于 r"..." 或 r#"..."# 跨行原生字符串中
)

// RustColors Rust 语法高亮调色板
type RustColors struct {
	Keyword     graphics.Color // fn, let, mut, struct, impl, trait, match, pub...
	Type        graphics.Color // i32, u64, f64, String, Vec, Option, Result...
	Macro       graphics.Color // println!, format!, vec!, #[derive]...
	Lifetime    graphics.Color // 'a, 'static...
	Function    graphics.Color // 函数定义与调用
	Constant    graphics.Color // true, false, Some, None, Ok, Err...
	String      graphics.Color // "str", r#"raw"#, 'c', b"bytes"
	Number      graphics.Color // 123, 0xFF, 3.14f32, 1_000_000usize
	Comment     graphics.Color // // comment, /// doc comment, /* comment */
	Operator    graphics.Color // ->, =>, ::, .., ..=, +, -, *, &, |...
	Punctuation graphics.Color // {, }, (, ), [, ], ;, ,, .
}

// DefaultDarkRustColors 暗色现代调色板
var DefaultDarkRustColors = rustColorsFromSyntax(theme.Dark().SyntaxColors())

// DefaultLightRustColors 浅色主题调色板
var DefaultLightRustColors = rustColorsFromSyntax(theme.Light().SyntaxColors())

// rustColorsFromSyntax 把共享语法令牌映射为 Rust 调色板。
func rustColorsFromSyntax(s theme.SyntaxPalette) RustColors {
	return RustColors{
		Keyword:     s.Keyword,
		Type:        s.Type,
		Macro:       s.Meta,
		Lifetime:    s.Lifetime,
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
	rustKeywords = map[string]bool{
		"as": true, "async": true, "await": true, "break": true, "const": true,
		"continue": true, "crate": true, "dyn": true, "else": true, "enum": true,
		"extern": true, "fn": true, "for": true, "if": true, "impl": true,
		"in": true, "let": true, "loop": true, "match": true, "mod": true,
		"move": true, "mut": true, "pub": true, "ref": true, "return": true,
		"self": true, "Self": true, "static": true, "struct": true, "super": true,
		"trait": true, "type": true, "unsafe": true, "use": true, "where": true,
		"while": true, "try": true, "yield": true, "macro_rules": true,
	}

	rustConstants = map[string]bool{
		"true": true, "false": true, "Some": true, "None": true, "Ok": true, "Err": true,
	}

	rustTypes = map[string]bool{
		// 标量类型
		"bool": true, "char": true, "str": true,
		"i8": true, "i16": true, "i32": true, "i64": true, "i128": true, "isize": true,
		"u8": true, "u16": true, "u32": true, "u64": true, "u128": true, "usize": true,
		"f32": true, "f64": true,
		// 核心集合与指针类型
		"String": true, "Vec": true, "Option": true, "Result": true,
		"Box": true, "Rc": true, "Arc": true, "RefCell": true, "Cell": true, "Mutex": true,
		"HashMap": true, "HashSet": true, "BTreeMap": true, "BTreeSet": true,
	}
)

// RustHighlighter Rust 语法高亮器
type RustHighlighter struct {
	isDark bool
	colors RustColors
}

// NewRustHighlighter 创建默认 Rust 语法高亮器
func NewRustHighlighter() *RustHighlighter {
	h := &RustHighlighter{
		isDark: true,
		colors: DefaultDarkRustColors,
	}
	h.ApplyTheme(theme.Current())
	return h
}

// GetColors 获取当前高亮颜色配置
func (h *RustHighlighter) GetColors() RustColors {
	return h.colors
}

// SetColors 自定义设置高亮颜色
func (h *RustHighlighter) SetColors(c RustColors) {
	h.colors = c
}

// ApplyTheme 根据当前主题自动适配调色板
func (h *RustHighlighter) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	h.colors = rustColorsFromSyntax(t.SyntaxColors())
	h.isDark = t.Dark
}

// HighlightLine 对单行 Rust 源码执行状态机词法分析并生成样式片段
func (h *RustHighlighter) HighlightLine(lineIndex int, line []rune, prevLineState int) ([]Span, int) {
	n := len(line)
	if n == 0 {
		return nil, prevLineState
	}

	var spans []Span
	i := 0

	// 1. 处理上一行延续的跨行状态
	if prevLineState == StateRustBlockComment {
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
			return spans, StateRustBlockComment
		}

		spans = append(spans, Span{
			StartCol: 0,
			EndCol:   closeIdx,
			Color:    h.colors.Comment,
			Italic:   true,
		})
		i = closeIdx
	} else if prevLineState == StateRustRawString {
		// 原生字符串在下一行寻找闭合引号 '"' 与 '#'
		closeIdx := -1
		for j := 0; j < n; j++ {
			if line[j] == '"' {
				if j+1 < n && line[j+1] == '#' {
					k := j + 1
					for k < n && line[k] == '#' {
						k++
					}
					closeIdx = k
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
			return spans, StateRustRawString
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

		// 2.2 单行注释与文档注释 // ... 或 /// ... 或 //! ...
		if r == '/' && i+1 < n && line[i+1] == '/' {
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   n,
				Color:    h.colors.Comment,
				Italic:   true,
			})
			return spans, StateRustNormal
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
				return spans, StateRustBlockComment
			}
			continue
		}

		// 2.4 属性注解: #[derive(...)] 或 #![no_std]
		if r == '#' && i+1 < n && (line[i+1] == '[' || (line[i+1] == '!' && i+2 < n && line[i+2] == '[')) {
			start := i
			for i < n && line[i] != ']' {
				i++
			}
			if i < n && line[i] == ']' {
				i++
			}
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Macro,
				Bold:     true,
			})
			continue
		}

		// 2.5 原生字符串: r"..." 或 r#"..."#
		if (r == 'r' || (r == 'b' && i+1 < n && line[i+1] == 'r')) && i+1 < n {
			start := i
			temp := i
			if line[temp] == 'b' {
				temp++
			}
			if temp < n && line[temp] == 'r' {
				temp++
				hashes := 0
				for temp < n && line[temp] == '#' {
					hashes++
					temp++
				}
				if temp < n && line[temp] == '"' {
					// 确认是原生字符串
					i = temp + 1
					closed := false
					for i < n {
						if line[i] == '"' {
							match := true
							for k := 0; k < hashes; k++ {
								if i+1+k >= n || line[i+1+k] != '#' {
									match = false
									break
								}
							}
							if match {
								i += 1 + hashes
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
						return spans, StateRustRawString
					}
					continue
				}
			}
		}

		// 2.6 字节字符串与常规字符串: b"..." 或 "..."
		if r == '"' || (r == 'b' && i+1 < n && line[i+1] == '"') {
			start := i
			if r == 'b' {
				i += 2
			} else {
				i++
			}
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

		// 2.7 字符字面量与生命周期: 'a' 或 'static
		if r == '\'' || (r == 'b' && i+1 < n && line[i+1] == '\'') {
			start := i
			if r == 'b' {
				i += 2
			} else {
				i++
			}

			// 检查是否为生命周期 (如 'a, 'static)
			if r == '\'' && i < n && (unicode.IsLetter(line[i]) || line[i] == '_') {
				temp := i
				for temp < n && (unicode.IsLetter(line[temp]) || unicode.IsDigit(line[temp]) || line[temp] == '_') {
					temp++
				}
				// 若不是紧跟着闭合单引号 '\''，则判定为生命周期
				if temp >= n || line[temp] != '\'' {
					spans = append(spans, Span{
						StartCol: start,
						EndCol:   temp,
						Color:    h.colors.Lifetime,
						Italic:   true,
					})
					i = temp
					continue
				}
			}

			// 常规字符字面量 'c' 或 '\n'
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

		// 2.8 数值: 123, 0xFF, 0b101, 0o77, 3.14f32, 100usize
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

			// 匹配类型后缀: u8, i32, usize, f64 等
			for i < n && (unicode.IsLetter(line[i]) || unicode.IsDigit(line[i])) {
				i++
			}

			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Number,
			})
			continue
		}

		// 2.9 标识符、关键字、类型与宏调用
		if unicode.IsLetter(r) || r == '_' {
			start := i
			for i < n && (unicode.IsLetter(line[i]) || unicode.IsDigit(line[i]) || line[i] == '_') {
				i++
			}
			word := string(line[start:i])

			// 检查是否紧随 '!' (判定为宏调用，如 println!, vec!)
			if i < n && line[i] == '!' && (i+1 >= n || line[i+1] != '=') {
				i++
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Macro,
					Bold:     true,
				})
				continue
			}

			if rustKeywords[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Keyword,
					Bold:     true,
				})
			} else if rustConstants[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Constant,
					Bold:     true,
				})
			} else if rustTypes[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Type,
				})
			} else {
				// 检查后续是否跟随 '(' 或泛型 '<...>' 判定为函数调用/声明
				peek := i
				for peek < n && unicode.IsSpace(line[peek]) {
					peek++
				}
				isFunc := false
				if peek < n && line[peek] == '(' {
					isFunc = true
				} else if peek < n && line[peek] == '<' {
					depth := 0
					k := peek
					for k < n {
						if line[k] == '<' {
							depth++
						} else if line[k] == '>' {
							depth--
							if depth == 0 {
								k++
								break
							}
						}
						k++
					}
					for k < n && unicode.IsSpace(line[k]) {
						k++
					}
					if k < n && line[k] == '(' {
						isFunc = true
					}
				}
				if isFunc {
					spans = append(spans, Span{
						StartCol: start,
						EndCol:   i,
						Color:    h.colors.Function,
					})
				}
			}
			continue
		}

		// 2.10 三字符操作符
		if i+2 < n {
			three := string(line[i : i+3])
			if three == "..=" || three == "<<=" || three == ">>=" {
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   i + 3,
					Color:    h.colors.Operator,
				})
				i += 3
				continue
			}
		}

		// 2.11 双字符操作符
		if i+1 < n {
			two := string(line[i : i+2])
			if two == "->" || two == "=>" || two == "::" || two == ".." ||
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

		// 2.12 单字符操作符与标点符号
		switch r {
		case '+', '-', '*', '/', '%', '=', '<', '>', '!', '&', '|', '^', '~', '?', '@':
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

	return spans, StateRustNormal
}
