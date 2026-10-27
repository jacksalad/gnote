// Package syntax 提供纯 Go 实现的代码语法高亮与词法分词引擎
package syntax

import (
	"strings"
	"unicode"

	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/theme"
)

// Python 跨行状态常量
const (
	StatePyNormal       = 0
	StatePyTripleDouble = 1 // 处于 """...""" 多行三双引号字符串中
	StatePyTripleSingle = 2 // 处于 '''...''' 多行三单引号字符串中
)

// PythonColors Python 语法高亮调色板
type PythonColors struct {
	Keyword     graphics.Color // def, class, return, if, else, for, import...
	Builtin     graphics.Color // print, len, range, enumerate, open...
	Type        graphics.Color // int, str, float, bool, list, dict, set...
	Function    graphics.Color // 函数定义与调用
	Decorator   graphics.Color // @decorator
	String      graphics.Color // "str", 'str', """docstring"""
	Number      graphics.Color // 123, 0x1F, 3.14, 2j
	Comment     graphics.Color // # comment
	Constant    graphics.Color // True, False, None, self, cls
	Operator    graphics.Color // +, -, *, /, ==, !=, in, is, not, and, or...
	Punctuation graphics.Color // {, }, (, ), [, ], :, ,, .
}

// DefaultDarkPythonColors 暗色现代调色板
var DefaultDarkPythonColors = pythonColorsFromSyntax(theme.Dark().SyntaxColors())

// DefaultLightPythonColors 浅色主题调色板
var DefaultLightPythonColors = pythonColorsFromSyntax(theme.Light().SyntaxColors())

// pythonColorsFromSyntax 把共享语法令牌映射为 Python 调色板。
func pythonColorsFromSyntax(s theme.SyntaxPalette) PythonColors {
	return PythonColors{
		Keyword:     s.Keyword,
		Builtin:     s.Builtin,
		Type:        s.Type,
		Function:    s.Function,
		Decorator:   s.Meta,
		String:      s.String,
		Number:      s.Number,
		Comment:     s.Comment,
		Constant:    s.Constant,
		Operator:    s.Operator,
		Punctuation: s.Punctuation,
	}
}

var (
	pythonKeywords = map[string]bool{
		"def": true, "class": true, "import": true, "from": true, "as": true,
		"return": true, "if": true, "elif": true, "else": true, "for": true,
		"while": true, "break": true, "continue": true, "pass": true, "yield": true,
		"lambda": true, "try": true, "except": true, "finally": true, "raise": true,
		"with": true, "async": true, "await": true, "assert": true, "del": true,
		"global": true, "nonlocal": true, "in": true, "is": true, "not": true,
		"and": true, "or": true, "match": true, "case": true,
	}

	pythonConstants = map[string]bool{
		"True": true, "False": true, "None": true, "self": true, "cls": true,
		"Ellipsis": true, "NotImplemented": true,
	}

	pythonBuiltinTypes = map[string]bool{
		"int": true, "float": true, "str": true, "bool": true, "bytes": true,
		"list": true, "dict": true, "set": true, "tuple": true, "object": true,
		"bytearray": true, "memoryview": true, "frozenset": true, "complex": true,
	}

	pythonBuiltins = map[string]bool{
		"print": true, "len": true, "range": true, "enumerate": true, "zip": true,
		"map": true, "filter": true, "open": true, "type": true, "isinstance": true,
		"issubclass": true, "id": true, "sum": true, "min": true, "max": true,
		"abs": true, "round": true, "all": true, "any": true, "input": true,
		"repr": true, "iter": true, "next": true, "super": true, "property": true,
		"staticmethod": true, "classmethod": true, "sorted": true, "reversed": true,
		"hasattr": true, "getattr": true, "setattr": true, "delattr": true,
		"dir": true, "vars": true, "callable": true, "format": true, "hash": true,
		"help": true, "pow": true, "bin": true, "hex": true, "oct": true, "ord": true, "chr": true,
	}
)

// PythonHighlighter Python 语法高亮器
type PythonHighlighter struct {
	colors PythonColors
	isDark bool
}

// NewPythonHighlighter 创建 Python 语法高亮器
func NewPythonHighlighter() *PythonHighlighter {
	h := &PythonHighlighter{
		isDark: true,
		colors: DefaultDarkPythonColors,
	}
	h.ApplyTheme(theme.Current())
	return h
}

// ApplyTheme 动态跟随主题切换
func (h *PythonHighlighter) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	h.colors = pythonColorsFromSyntax(t.SyntaxColors())
	h.isDark = t.Dark
}

// SetColors 自定义设置颜色
func (h *PythonHighlighter) SetColors(colors PythonColors) {
	h.colors = colors
}

// GetColors 获取当前高亮颜色
func (h *PythonHighlighter) GetColors() PythonColors {
	return h.colors
}

// HighlightLine 对指定单行进行词法解析与高亮分词
func (h *PythonHighlighter) HighlightLine(lineIndex int, line []rune, prevLineState int) ([]Span, int) {
	n := len(line)
	if n == 0 {
		return nil, prevLineState
	}

	var spans []Span
	i := 0

	// 1. 处理继承自上一行的跨行三引号多行字符串状态
	if prevLineState == StatePyTripleDouble {
		closeIdx := -1
		for j := 0; j+2 < n; j++ {
			if line[j] == '"' && line[j+1] == '"' && line[j+2] == '"' {
				closeIdx = j
				break
			}
		}
		if closeIdx == -1 {
			spans = append(spans, Span{
				StartCol: 0,
				EndCol:   n,
				Color:    h.colors.String,
			})
			return spans, StatePyTripleDouble
		}
		spans = append(spans, Span{
			StartCol: 0,
			EndCol:   closeIdx + 3,
			Color:    h.colors.String,
		})
		i = closeIdx + 3
	} else if prevLineState == StatePyTripleSingle {
		closeIdx := -1
		for j := 0; j+2 < n; j++ {
			if line[j] == '\'' && line[j+1] == '\'' && line[j+2] == '\'' {
				closeIdx = j
				break
			}
		}
		if closeIdx == -1 {
			spans = append(spans, Span{
				StartCol: 0,
				EndCol:   n,
				Color:    h.colors.String,
			})
			return spans, StatePyTripleSingle
		}
		spans = append(spans, Span{
			StartCol: 0,
			EndCol:   closeIdx + 3,
			Color:    h.colors.String,
		})
		i = closeIdx + 3
	}

	lastWasDef := false
	lastWasClass := false

	// 2. 主分词循环
	for i < n {
		r := line[i]

		// 跳过空白字符
		if unicode.IsSpace(r) {
			i++
			continue
		}

		// 单行注释 #
		if r == '#' {
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   n,
				Color:    h.colors.Comment,
			})
			return spans, StatePyNormal
		}

		// 装饰器 @decorator
		if r == '@' {
			start := i
			i++
			for i < n && (unicode.IsLetter(line[i]) || unicode.IsDigit(line[i]) || line[i] == '_' || line[i] == '.') {
				i++
			}
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Decorator,
				Bold:     true,
			})
			lastWasDef = false
			lastWasClass = false
			continue
		}

		// 三引号与单引号/双引号字符串 (可选带 f, r, b, u, fr 前缀)
		// 探测是否为带前缀的字符串：例如 f"...", r'...'
		prefixLen := 0
		if (r == 'f' || r == 'F' || r == 'r' || r == 'R' || r == 'b' || r == 'B' || r == 'u' || r == 'U') && i+1 < n {
			if line[i+1] == '"' || line[i+1] == '\'' {
				prefixLen = 1
			} else if (line[i+1] == 'r' || line[i+1] == 'R' || line[i+1] == 'b' || line[i+1] == 'B') && i+2 < n && (line[i+2] == '"' || line[i+2] == '\'') {
				prefixLen = 2
			}
		}

		quoteIdx := i + prefixLen
		if quoteIdx < n && (line[quoteIdx] == '"' || line[quoteIdx] == '\'') {
			quoteChar := line[quoteIdx]
			// 检查是否为三引号 """ 或 '''
			if quoteIdx+2 < n && line[quoteIdx+1] == quoteChar && line[quoteIdx+2] == quoteChar {
				// 三引号字符串开启
				isDouble := (quoteChar == '"')
				closeIdx := -1
				for j := quoteIdx + 3; j+2 < n; j++ {
					if line[j] == quoteChar && line[j+1] == quoteChar && line[j+2] == quoteChar {
						closeIdx = j
						break
					}
				}
				if closeIdx != -1 {
					// 当前行闭合
					spans = append(spans, Span{
						StartCol: i,
						EndCol:   closeIdx + 3,
						Color:    h.colors.String,
					})
					i = closeIdx + 3
					lastWasDef = false
					lastWasClass = false
					continue
				}
				// 跨行未闭合
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   n,
					Color:    h.colors.String,
				})
				if isDouble {
					return spans, StatePyTripleDouble
				}
				return spans, StatePyTripleSingle
			}

			// 普通单行引号字符串
			start := i
			curr := quoteIdx + 1
			escaped := false
			for curr < n {
				ch := line[curr]
				if escaped {
					escaped = false
					curr++
					continue
				}
				if ch == '\\' {
					escaped = true
					curr++
					continue
				}
				if ch == quoteChar {
					curr++
					break
				}
				curr++
			}
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   curr,
				Color:    h.colors.String,
			})
			i = curr
			lastWasDef = false
			lastWasClass = false
			continue
		}

		// 数字字面量 (十进制、0x十六进制、0o八进制、0b二进制、浮点小数、科学计数、虚数2j)
		if unicode.IsDigit(r) || (r == '.' && i+1 < n && unicode.IsDigit(line[i+1])) {
			start := i
			i++
			isHex := false
			if r == '0' && i < n && (line[i] == 'x' || line[i] == 'X') {
				isHex = true
				i++
			} else if r == '0' && i < n && (line[i] == 'b' || line[i] == 'B' || line[i] == 'o' || line[i] == 'O') {
				i++
			}

			for i < n {
				ch := line[i]
				if unicode.IsDigit(ch) || ch == '_' {
					i++
				} else if isHex && ((ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
					i++
				} else if ch == '.' {
					i++
				} else if ch == 'e' || ch == 'E' {
					i++
					if i < n && (line[i] == '+' || line[i] == '-') {
						i++
					}
				} else if ch == 'j' || ch == 'J' { // Python 虚数后缀
					i++
					break
				} else {
					break
				}
			}

			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Number,
			})
			lastWasDef = false
			lastWasClass = false
			continue
		}

		// 标识符与关键字
		if isIdentStart(r) {
			start := i
			for i < n && isIdentPart(line[i]) {
				i++
			}
			word := string(line[start:i])

			// 检查后跟字符是否为 '('
			isFollowedByParen := false
			peek := i
			for peek < n && unicode.IsSpace(line[peek]) {
				peek++
			}
			if peek < n && line[peek] == '(' {
				isFollowedByParen = true
			}

			if pythonKeywords[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Keyword,
					Bold:     true,
				})
				lastWasDef = (word == "def")
				lastWasClass = (word == "class")
			} else if pythonConstants[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Constant,
					Bold:     true,
				})
				lastWasDef = false
				lastWasClass = false
			} else if pythonBuiltinTypes[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Type,
				})
				lastWasDef = false
				lastWasClass = false
			} else if pythonBuiltins[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Builtin,
				})
				lastWasDef = false
				lastWasClass = false
			} else if lastWasDef || isFollowedByParen {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Function,
				})
				lastWasDef = false
				lastWasClass = false
			} else if lastWasClass || (unicode.IsUpper(r) && !strings.Contains(word, "_")) {
				// 类定义或大写开头的类名
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Type,
				})
				lastWasDef = false
				lastWasClass = false
			} else {
				lastWasDef = false
				lastWasClass = false
			}
			continue
		}

		// 复合运算符与标点
		matchedOp := false
		if i+1 < n {
			two := string(line[i : i+2])
			if two == "==" || two == "!=" || two == "<=" || two == ">=" ||
				two == "+=" || two == "-=" || two == "*=" || two == "/=" ||
				two == "//" || two == "**" || two == "->" || two == ":=" ||
				two == "%=" || two == "&=" || two == "|=" || two == "^=" ||
				two == "<<" || two == ">>" {
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   i + 2,
					Color:    h.colors.Operator,
				})
				i += 2
				matchedOp = true
			}
		}
		if matchedOp {
			lastWasDef = false
			lastWasClass = false
			continue
		}

		// 单字符操作符与标点
		switch r {
		case '+', '-', '*', '/', '%', '&', '|', '^', '~', '<', '>', '=':
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   i + 1,
				Color:    h.colors.Operator,
			})
		case ':', ';', ',', '.', '(', ')', '[', ']', '{', '}':
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   i + 1,
				Color:    h.colors.Punctuation,
			})
		}

		if r != '(' && r != '.' {
			lastWasDef = false
			lastWasClass = false
		}
		i++
	}

	return spans, StatePyNormal
}
