// Package syntax 提供纯 Go 实现的代码语法高亮与词法分词引擎
package syntax

import (
	"unicode"

	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/theme"
)

// 跨行状态常量
const (
	StateGoNormal       = 0
	StateGoBlockComment = 1 // 处于 /* ... */ 多行块注释中
	StateGoRawString    = 2 // 处于 `...` 多行原生字符串中
)

// GoColors Go 语言语法高亮调色板
type GoColors struct {
	Keyword     graphics.Color // func, package, import, return, if, else, for...
	Type        graphics.Color // string, int, int64, bool, error, any, float64...
	Builtin     graphics.Color // len, append, make, new, panic, recover, cap...
	Function    graphics.Color // 函数名与方法调用
	String      graphics.Color // "hello" 与 `raw string`
	Character   graphics.Color // 'a', '\n'
	Number      graphics.Color // 123, 0x1F, 3.14, 1e-6, 2i
	Comment     graphics.Color // // 与 /* */
	Constant    graphics.Color // true, false, nil, iota
	Operator    graphics.Color // :=, <-, ==, !=, &&, ||, +, -, *...
	Punctuation graphics.Color // {, }, (, ), [, ], ,, ;, .
}

// DefaultDarkGoColors 暗色现代极客调色板
var DefaultDarkGoColors = goColorsFromSyntax(theme.Dark().SyntaxColors())

// DefaultLightGoColors 浅色主题调色板
var DefaultLightGoColors = goColorsFromSyntax(theme.Light().SyntaxColors())

// goColorsFromSyntax 把共享语法令牌映射为 Go 语言调色板。
func goColorsFromSyntax(s theme.SyntaxPalette) GoColors {
	return GoColors{
		Keyword:     s.Keyword,
		Type:        s.Type,
		Builtin:     s.Builtin,
		Function:    s.Function,
		String:      s.String,
		Character:   s.Character,
		Number:      s.Number,
		Comment:     s.Comment,
		Constant:    s.Constant,
		Operator:    s.Operator,
		Punctuation: s.Punctuation,
	}
}

// Go 语言词典字典
var (
	goKeywords = map[string]bool{
		"break": true, "default": true, "func": true, "interface": true, "select": true,
		"case": true, "defer": true, "go": true, "map": true, "struct": true,
		"chan": true, "else": true, "goto": true, "package": true, "switch": true,
		"const": true, "fallthrough": true, "if": true, "range": true, "type": true,
		"continue": true, "for": true, "import": true, "return": true, "var": true,
	}

	goConstants = map[string]bool{
		"true": true, "false": true, "iota": true, "nil": true,
	}

	goBuiltinTypes = map[string]bool{
		"bool": true, "byte": true, "complex64": true, "complex128": true, "error": true,
		"float32": true, "float64": true, "int": true, "int8": true, "int16": true,
		"int32": true, "int64": true, "rune": true, "string": true, "uint": true,
		"uint8": true, "uint16": true, "uint32": true, "uint64": true, "uintptr": true,
		"any": true, "comparable": true,
	}

	goBuiltins = map[string]bool{
		"append": true, "cap": true, "close": true, "complex": true, "copy": true,
		"delete": true, "imag": true, "len": true, "make": true, "new": true,
		"panic": true, "print": true, "println": true, "real": true, "recover": true,
		"clear": true, "min": true, "max": true,
	}
)

// GoHighlighter 纯 Go 实现的 Go 语言语法高亮器
type GoHighlighter struct {
	colors GoColors
	isDark bool
}

// NewGoHighlighter 创建 Go 语法高亮器
func NewGoHighlighter() *GoHighlighter {
	h := &GoHighlighter{
		isDark: true,
		colors: DefaultDarkGoColors,
	}
	h.ApplyTheme(theme.Current())
	return h
}

// ApplyTheme 跟随编辑器全局主题动态刷新调色板
func (h *GoHighlighter) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	h.colors = goColorsFromSyntax(t.SyntaxColors())
	h.isDark = t.Dark
}

// SetColors 自定义设置高亮颜色
func (h *GoHighlighter) SetColors(colors GoColors) {
	h.colors = colors
}

// GetColors 获取当前高亮颜色
func (h *GoHighlighter) GetColors() GoColors {
	return h.colors
}

// HighlightLine 对指定单行进行词法解析与高亮分词
func (h *GoHighlighter) HighlightLine(lineIndex int, line []rune, prevLineState int) ([]Span, int) {
	n := len(line)
	if n == 0 {
		return nil, prevLineState
	}

	var spans []Span
	i := 0

	// 1. 处理继承自上一行的跨行状态
	if prevLineState == StateGoBlockComment {
		closeIdx := -1
		for j := 0; j+1 < n; j++ {
			if line[j] == '*' && line[j+1] == '/' {
				closeIdx = j
				break
			}
		}
		if closeIdx == -1 {
			// 整行依然处于块注释内
			spans = append(spans, Span{
				StartCol: 0,
				EndCol:   n,
				Color:    h.colors.Comment,
			})
			return spans, StateGoBlockComment
		}
		// 块注释在本行结束
		spans = append(spans, Span{
			StartCol: 0,
			EndCol:   closeIdx + 2,
			Color:    h.colors.Comment,
		})
		i = closeIdx + 2
	} else if prevLineState == StateGoRawString {
		closeIdx := -1
		for j := 0; j < n; j++ {
			if line[j] == '`' {
				closeIdx = j
				break
			}
		}
		if closeIdx == -1 {
			// 整行依然处于原生字符串内
			spans = append(spans, Span{
				StartCol: 0,
				EndCol:   n,
				Color:    h.colors.String,
			})
			return spans, StateGoRawString
		}
		// 原生字符串在本行结束
		spans = append(spans, Span{
			StartCol: 0,
			EndCol:   closeIdx + 1,
			Color:    h.colors.String,
		})
		i = closeIdx + 1
	}

	lastWasFuncKeyword := false

	// 2. 主扫描分词循环
	for i < n {
		r := line[i]

		// 跳过空白字符
		if unicode.IsSpace(r) {
			i++
			continue
		}

		// 单行注释与多行注释
		if r == '/' && i+1 < n {
			if line[i+1] == '/' {
				// 单行注释 //，延续至行尾
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   n,
					Color:    h.colors.Comment,
				})
				return spans, StateGoNormal
			}
			if line[i+1] == '*' {
				// 块注释开启 /*
				closeIdx := -1
				for j := i + 2; j+1 < n; j++ {
					if line[j] == '*' && line[j+1] == '/' {
						closeIdx = j
						break
					}
				}
				if closeIdx != -1 {
					spans = append(spans, Span{
						StartCol: i,
						EndCol:   closeIdx + 2,
						Color:    h.colors.Comment,
					})
					i = closeIdx + 2
					lastWasFuncKeyword = false
					continue
				}
				// 块注释跨至下一行
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   n,
					Color:    h.colors.Comment,
				})
				return spans, StateGoBlockComment
			}
		}

		// 原生多行字符串 `...`
		if r == '`' {
			closeIdx := -1
			for j := i + 1; j < n; j++ {
				if line[j] == '`' {
					closeIdx = j
					break
				}
			}
			if closeIdx != -1 {
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   closeIdx + 1,
					Color:    h.colors.String,
				})
				i = closeIdx + 1
				lastWasFuncKeyword = false
				continue
			}
			// 未在当前行闭合，跨至下一行
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   n,
				Color:    h.colors.String,
			})
			return spans, StateGoRawString
		}

		// 解释型双引号字符串 "..."
		if r == '"' {
			start := i
			i++
			escaped := false
			for i < n {
				ch := line[i]
				if escaped {
					escaped = false
					i++
					continue
				}
				if ch == '\\' {
					escaped = true
					i++
					continue
				}
				if ch == '"' {
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
			lastWasFuncKeyword = false
			continue
		}

		// 单引号字符字面量 'c'
		if r == '\'' {
			start := i
			i++
			escaped := false
			for i < n {
				ch := line[i]
				if escaped {
					escaped = false
					i++
					continue
				}
				if ch == '\\' {
					escaped = true
					i++
					continue
				}
				if ch == '\'' {
					i++
					break
				}
				i++
			}
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Character,
			})
			lastWasFuncKeyword = false
			continue
		}

		// 数字字面量 (支持十进制、0x十六进制、0b二进制、0o八进制、浮点小数、科学计数法、虚数i)
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
				} else if ch == 'e' || ch == 'E' || (isHex && (ch == 'p' || ch == 'P')) {
					i++
					if i < n && (line[i] == '+' || line[i] == '-') {
						i++
					}
				} else if ch == 'i' { // 虚数后缀 2i
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
			lastWasFuncKeyword = false
			continue
		}

		// 标识符与关键字
		if isIdentStart(r) {
			start := i
			for i < n && isIdentPart(line[i]) {
				i++
			}
			word := string(line[start:i])

			// 检查下一个非空白字符是否为 '(' (用于探测函数调用)
			isFollowedByParen := false
			peek := i
			for peek < n && unicode.IsSpace(line[peek]) {
				peek++
			}
			if peek < n && line[peek] == '(' {
				isFollowedByParen = true
			}

			if goKeywords[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Keyword,
					Bold:     true,
				})
				lastWasFuncKeyword = (word == "func")
			} else if goConstants[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Constant,
					Bold:     true,
				})
				lastWasFuncKeyword = false
			} else if goBuiltinTypes[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Type,
				})
				lastWasFuncKeyword = false
			} else if goBuiltins[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Builtin,
				})
				lastWasFuncKeyword = false
			} else if lastWasFuncKeyword || isFollowedByParen {
				// 函数定义或函数调用
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Function,
				})
				lastWasFuncKeyword = false
			} else {
				// 普通标识符 (变量名/包名等)，若首字母大写且非调用，可作为导出的结构体/接口类型感知
				if unicode.IsUpper(r) {
					// 类似 MyStruct 或导出的标识符
					spans = append(spans, Span{
						StartCol: start,
						EndCol:   i,
						Color:    h.colors.Type,
					})
				}
				lastWasFuncKeyword = false
			}
			continue
		}

		// 复合操作符与单字符标点
		// 复合操作符探测
		matchedOp := false
		if i+2 < n {
			three := string(line[i : i+3])
			if three == "..." || three == "<<=" || three == ">>=" || three == "&^=" {
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   i + 3,
					Color:    h.colors.Operator,
				})
				i += 3
				matchedOp = true
			}
		}
		if !matchedOp && i+1 < n {
			two := string(line[i : i+2])
			if two == ":=" || two == "<-" || two == "==" || two == "!=" ||
				two == "<=" || two == ">=" || two == "&&" || two == "||" ||
				two == "++" || two == "--" || two == "<<" || two == ">>" ||
				two == "&^" || two == "+=" || two == "-=" || two == "*=" ||
				two == "/=" || two == "%=" || two == "&=" || two == "|=" || two == "^=" {
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
			lastWasFuncKeyword = false
			continue
		}

		// 单字符操作符与标点符号
		switch r {
		case '+', '-', '*', '/', '%', '&', '|', '^', '!', '<', '>', '=', '~':
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   i + 1,
				Color:    h.colors.Operator,
			})
		case '{', '}', '(', ')', '[', ']', ':', ';', ',', '.':
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   i + 1,
				Color:    h.colors.Punctuation,
			})
			if r != '(' && r != '.' {
				lastWasFuncKeyword = false
			}
		}

		i++
	}

	return spans, StateGoNormal
}

func isIdentStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

func isIdentPart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
