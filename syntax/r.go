// Package syntax 提供纯 Go 实现的代码语法高亮与词法分词引擎
package syntax

import (
	"unicode"

	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/theme"
)

// R 跨行状态常量
const (
	StateRNormal                = 0
	StateRMultiLineStringDouble = 1 // 处于 "..." 跨行字符串中
	StateRMultiLineStringSingle = 2 // 处于 '...' 跨行字符串中
)

// RColors R 语言语法高亮调色板
type RColors struct {
	Keyword     graphics.Color // if, else, for, while, function, return...
	Special     graphics.Color // <-, <<-, ->, %>%, |>, %in%, %*%...
	Function    graphics.Color // c, list, data.frame, plot, summary, library...
	Type        graphics.Color // numeric, character, logical, factor, matrix...
	Constant    graphics.Color // TRUE, FALSE, NULL, NA, Inf, NaN...
	String      graphics.Color // "str", 'str', `ident`
	Number      graphics.Color // 123L, 3.14, 2i, 0xFF
	Comment     graphics.Color // # comment
	Operator    graphics.Color // $, @, ~, +, -, *, ==, !=, &, |...
	Punctuation graphics.Color // {, }, (, ), [, ], [[, ]], ;, ,, ::...
}

// DefaultDarkRColors 暗色现代调色板
var DefaultDarkRColors = rColorsFromSyntax(theme.Dark().SyntaxColors())

// DefaultLightRColors 浅色主题调色板
var DefaultLightRColors = rColorsFromSyntax(theme.Light().SyntaxColors())

// rColorsFromSyntax 把共享语法令牌映射为 R 调色板。
func rColorsFromSyntax(s theme.SyntaxPalette) RColors {
	return RColors{
		Keyword:     s.Keyword,
		Special:     s.Meta,
		Function:    s.Function,
		Type:        s.Type,
		Constant:    s.Constant,
		String:      s.String,
		Number:      s.Number,
		Comment:     s.Comment,
		Operator:    s.Operator,
		Punctuation: s.Punctuation,
	}
}

var (
	rKeywords = map[string]bool{
		"if": true, "else": true, "repeat": true, "while": true,
		"function": true, "for": true, "in": true, "next": true,
		"break": true, "return": true,
	}

	rConstants = map[string]bool{
		"TRUE": true, "FALSE": true, "T": true, "F": true,
		"NULL": true, "NA": true, "NA_integer_": true, "NA_real_": true,
		"NA_complex_": true, "NA_character_": true, "Inf": true, "NaN": true,
	}

	rTypes = map[string]bool{
		"numeric": true, "integer": true, "double": true, "character": true,
		"logical": true, "complex": true, "raw": true, "list": true,
		"factor": true, "matrix": true, "array": true, "data.frame": true,
		"tibble": true, "vector": true, "environment": true,
	}

	rBuiltinFunctions = map[string]bool{
		"c": true, "print": true, "cat": true, "paste": true, "paste0": true,
		"sprintf": true, "message": true, "warning": true, "stop": true,
		"library": true, "require": true, "source": true, "install.packages": true,
		"summary": true, "str": true, "head": true, "tail": true, "dim": true,
		"nrow": true, "ncol": true, "length": true, "names": true, "colnames": true, "rownames": true,
		"plot": true, "ggplot": true, "aes": true, "mean": true, "median": true,
		"sd": true, "var": true, "cor": true, "lm": true, "glm": true, "predict": true,
		"apply": true, "lapply": true, "sapply": true, "tapply": true, "vapply": true,
		"read.csv": true, "write.csv": true, "readLines": true, "saveRDS": true, "readRDS": true,
		"seq": true, "rep": true, "which": true, "unique": true, "subset": true, "merge": true,
	}
)

// RHighlighter R 语言语法高亮器
type RHighlighter struct {
	isDark bool
	colors RColors
}

// NewRHighlighter 创建默认 R 语言语法高亮器
func NewRHighlighter() *RHighlighter {
	h := &RHighlighter{
		isDark: true,
		colors: DefaultDarkRColors,
	}
	h.ApplyTheme(theme.Current())
	return h
}

// GetColors 获取当前高亮颜色配置
func (h *RHighlighter) GetColors() RColors {
	return h.colors
}

// SetColors 自定义设置高亮颜色
func (h *RHighlighter) SetColors(c RColors) {
	h.colors = c
}

// ApplyTheme 根据当前主题自动适配调色板
func (h *RHighlighter) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	h.colors = rColorsFromSyntax(t.SyntaxColors())
	h.isDark = t.Dark
}

// HighlightLine 对单行 R 源码执行状态机词法分析并生成样式片段
func (h *RHighlighter) HighlightLine(lineIndex int, line []rune, prevLineState int) ([]Span, int) {
	n := len(line)
	if n == 0 {
		return nil, prevLineState
	}

	var spans []Span
	i := 0

	// 1. 处理上一行延续的多行字符串状态
	if prevLineState == StateRMultiLineStringDouble {
		closeIdx := -1
		escaped := false
		for j := 0; j < n; j++ {
			if escaped {
				escaped = false
				continue
			}
			if line[j] == '\\' {
				escaped = true
				continue
			}
			if line[j] == '"' {
				closeIdx = j + 1
				break
			}
		}

		if closeIdx == -1 {
			spans = append(spans, Span{
				StartCol: 0,
				EndCol:   n,
				Color:    h.colors.String,
			})
			return spans, StateRMultiLineStringDouble
		}

		spans = append(spans, Span{
			StartCol: 0,
			EndCol:   closeIdx,
			Color:    h.colors.String,
		})
		i = closeIdx
	} else if prevLineState == StateRMultiLineStringSingle {
		closeIdx := -1
		escaped := false
		for j := 0; j < n; j++ {
			if escaped {
				escaped = false
				continue
			}
			if line[j] == '\\' {
				escaped = true
				continue
			}
			if line[j] == '\'' {
				closeIdx = j + 1
				break
			}
		}

		if closeIdx == -1 {
			spans = append(spans, Span{
				StartCol: 0,
				EndCol:   n,
				Color:    h.colors.String,
			})
			return spans, StateRMultiLineStringSingle
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

		// 2.2 单行注释: # ...
		if r == '#' {
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   n,
				Color:    h.colors.Comment,
				Italic:   true,
			})
			return spans, StateRNormal
		}

		// 2.3 双引号字符串: "..." (支持跨行)
		if r == '"' {
			start := i
			i++
			escaped := false
			closed := false
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
					closed = true
					break
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
				return spans, StateRMultiLineStringDouble
			}
			continue
		}

		// 2.4 单引号字符串: '...' (支持跨行)
		if r == '\'' {
			start := i
			i++
			escaped := false
			closed := false
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
					closed = true
					break
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
				return spans, StateRMultiLineStringSingle
			}
			continue
		}

		// 2.5 反引号标识符: `variable name`
		if r == '`' {
			start := i
			i++
			for i < n && line[i] != '`' {
				i++
			}
			if i < n && line[i] == '`' {
				i++
			}
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.String,
			})
			continue
		}

		// 2.6 特殊二元运算符: %...% (如 %in%, %>%, %*%, %% 等)
		if r == '%' {
			start := i
			i++
			for i < n && line[i] != '%' && !unicode.IsSpace(line[i]) {
				i++
			}
			if i < n && line[i] == '%' {
				i++
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Special,
					Bold:     true,
				})
				continue
			}
			// 单独百分号退回由后续操作符处理
			i = start
		}

		// 2.7 管道符与多字符赋值操作符: |>, <-, <<-, ->, ->>
		if i+2 < n {
			three := string(line[i : i+3])
			if three == "<<-" || three == "->>" {
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   i + 3,
					Color:    h.colors.Special,
					Bold:     true,
				})
				i += 3
				continue
			}
		}
		if i+1 < n {
			two := string(line[i : i+2])
			if two == "<-" || two == "->" || two == "|>" {
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   i + 2,
					Color:    h.colors.Special,
					Bold:     true,
				})
				i += 2
				continue
			}
			if two == "==" || two == "!=" || two == "<=" || two == ">=" ||
				two == "&&" || two == "||" || two == "::" || two == "[[" || two == "]]" {
				color := h.colors.Operator
				if two == "::" || two == "[[" || two == "]]" {
					color = h.colors.Punctuation
				}
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   i + 2,
					Color:    color,
				})
				i += 2
				continue
			}
		}

		// 2.8 数值: 123, 123L, 3.14, 1e-5, 2i, 0xFF
		if unicode.IsDigit(r) || (r == '.' && i+1 < n && unicode.IsDigit(line[i+1])) {
			start := i
			isHex := false
			if r == '0' && i+1 < n && (line[i+1] == 'x' || line[i+1] == 'X') {
				isHex = true
				i += 2
			}
			for i < n {
				curr := line[i]
				if isHex {
					if unicode.IsDigit(curr) || (curr >= 'a' && curr <= 'f') || (curr >= 'A' && curr <= 'F') {
						i++
					} else {
						break
					}
				} else {
					if unicode.IsDigit(curr) || curr == '.' {
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
			// 匹配 R 特有数值后缀: L (integer), i (complex)
			if i < n && (line[i] == 'L' || line[i] == 'i') {
				i++
			}
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Number,
			})
			continue
		}

		// 2.9 标识符与关键字 (R 标识符允许以字母或点号开头，包含字母、数字、点号和下划线)
		if unicode.IsLetter(r) || (r == '.' && i+1 < n && (unicode.IsLetter(line[i+1]) || line[i+1] == '.')) {
			start := i
			for i < n && (unicode.IsLetter(line[i]) || unicode.IsDigit(line[i]) || line[i] == '.' || line[i] == '_') {
				i++
			}
			word := string(line[start:i])

			if rKeywords[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Keyword,
					Bold:     true,
				})
			} else if rConstants[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Constant,
					Bold:     true,
				})
			} else if rBuiltinFunctions[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Function,
				})
			} else if rTypes[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Type,
				})
			} else {
				// 检查后续是否紧跟 '(' 判定为函数调用
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

		// 2.10 单字符操作符与标点符号
		switch r {
		case '<', '>', '=', '+', '-', '*', '/', '^', '!', '&', '|', '~', '$', '@':
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   i + 1,
				Color:    h.colors.Operator,
			})
			i++
		case '(', ')', '{', '}', '[', ']', ';', ',', ':':
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

	return spans, StateRNormal
}
