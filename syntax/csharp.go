// Package syntax 提供纯 Go 实现的代码语法高亮与词法分词引擎
package syntax

import (
	"unicode"

	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/theme"
)

// C# 跨行状态常量
const (
	StateCSNormal         = 0
	StateCSBlockComment   = 1 // 处于 /* ... */ 跨行注释中
	StateCSVerbatimString = 2 // 处于 @"..." 跨行逐字字符串中
)

// CSharpColors C# 语法高亮调色板
type CSharpColors struct {
	Keyword      graphics.Color // class, public, namespace, return, async, await...
	Type         graphics.Color // int, string, bool, Task, List, Dictionary...
	Preprocessor graphics.Color // #region, #endregion, #if, #pragma...
	Function     graphics.Color // 函数/方法定义与调用
	Constant     graphics.Color // true, false, null, this, base, default
	String       graphics.Color // "str", @"verbatim", $"interp", 'c'
	Number       graphics.Color // 123, 0xFF, 3.14f, 1_000_000m
	Comment      graphics.Color // // comment, /// xml comment, /* comment */
	Operator     graphics.Color // +, -, *, =>, ??, ??=, ?., ==, !=...
	Punctuation  graphics.Color // {, }, (, ), [, ], ;, ,, .
}

// DefaultDarkCSharpColors 暗色现代调色板
var DefaultDarkCSharpColors = csharpColorsFromSyntax(theme.Dark().SyntaxColors())

// DefaultLightCSharpColors 浅色主题调色板
var DefaultLightCSharpColors = csharpColorsFromSyntax(theme.Light().SyntaxColors())

// csharpColorsFromSyntax 把共享语法令牌映射为 C# 调色板。
func csharpColorsFromSyntax(s theme.SyntaxPalette) CSharpColors {
	return CSharpColors{
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
	csKeywords = map[string]bool{
		"abstract": true, "as": true, "async": true, "await": true, "break": true,
		"case": true, "catch": true, "checked": true, "class": true, "const": true,
		"continue": true, "delegate": true, "do": true, "else": true, "enum": true,
		"event": true, "explicit": true, "extern": true, "finally": true, "fixed": true,
		"for": true, "foreach": true, "goto": true, "if": true, "implicit": true,
		"in": true, "interface": true, "internal": true, "is": true, "lock": true,
		"namespace": true, "new": true, "operator": true, "out": true, "override": true,
		"params": true, "private": true, "protected": true, "public": true, "readonly": true,
		"ref": true, "return": true, "sealed": true, "sizeof": true, "stackalloc": true,
		"static": true, "struct": true, "switch": true, "throw": true, "try": true,
		"typeof": true, "unchecked": true, "unsafe": true, "using": true, "virtual": true,
		"volatile": true, "while": true, "yield": true, "partial": true, "record": true,
		"init": true, "required": true, "file": true, "scoped": true,
		// LINQ 查询关键字
		"from": true, "where": true, "select": true, "group": true, "into": true,
		"orderby": true, "join": true, "let": true, "on": true, "equals": true,
		"by": true, "ascending": true, "descending": true, "when": true, "with": true,
	}

	csConstants = map[string]bool{
		"true": true, "false": true, "null": true, "this": true, "base": true, "default": true,
	}

	csBuiltinTypes = map[string]bool{
		// 基础关键字类型
		"bool": true, "byte": true, "sbyte": true, "char": true, "decimal": true,
		"double": true, "float": true, "int": true, "uint": true, "nint": true,
		"nuint": true, "long": true, "ulong": true, "short": true, "ushort": true,
		"object": true, "string": true, "dynamic": true, "void": true, "var": true,
		// BCL 核心常用类型
		"Task": true, "ValueTask": true, "Action": true, "Func": true, "Predicate": true,
		"Span": true, "ReadOnlySpan": true, "Memory": true, "ReadOnlyMemory": true,
		"List": true, "Dictionary": true, "HashSet": true, "Queue": true, "Stack": true,
		"IEnumerable": true, "ICollection": true, "IList": true, "IDictionary": true, "IQueryable": true,
		"Nullable": true, "Guid": true, "DateTime": true, "DateTimeOffset": true, "TimeSpan": true,
		"Exception": true, "Console": true, "StringBuilder": true, "Math": true,
	}
)

// CSharpHighlighter C# 语法高亮器
type CSharpHighlighter struct {
	isDark bool
	colors CSharpColors
}

// NewCSharpHighlighter 创建默认 C# 语法高亮器
func NewCSharpHighlighter() *CSharpHighlighter {
	h := &CSharpHighlighter{
		isDark: true,
		colors: DefaultDarkCSharpColors,
	}
	h.ApplyTheme(theme.Current())
	return h
}

// GetColors 获取当前高亮颜色配置
func (h *CSharpHighlighter) GetColors() CSharpColors {
	return h.colors
}

// SetColors 自定义设置高亮颜色
func (h *CSharpHighlighter) SetColors(c CSharpColors) {
	h.colors = c
}

// ApplyTheme 根据当前主题自动适配调色板
func (h *CSharpHighlighter) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	h.colors = csharpColorsFromSyntax(t.SyntaxColors())
	h.isDark = t.Dark
}

// HighlightLine 对单行 C# 源码执行状态机词法分析并生成样式片段
func (h *CSharpHighlighter) HighlightLine(lineIndex int, line []rune, prevLineState int) ([]Span, int) {
	n := len(line)
	if n == 0 {
		return nil, prevLineState
	}

	var spans []Span
	i := 0

	// 1. 处理上一行延续的跨行状态
	if prevLineState == StateCSBlockComment {
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
			return spans, StateCSBlockComment
		}

		spans = append(spans, Span{
			StartCol: 0,
			EndCol:   closeIdx,
			Color:    h.colors.Comment,
			Italic:   true,
		})
		i = closeIdx
	} else if prevLineState == StateCSVerbatimString {
		// 处于 @"..." 跨行逐字字符串中 (双引号转义为两个 "")
		closeIdx := -1
		for j := 0; j < n; j++ {
			if line[j] == '"' {
				if j+1 < n && line[j+1] == '"' {
					// "" 转义双引号，跳过
					j++
					continue
				}
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
			return spans, StateCSVerbatimString
		}

		spans = append(spans, Span{
			StartCol: 0,
			EndCol:   closeIdx,
			Color:    h.colors.String,
		})
		i = closeIdx
	}

	// 2. 检查是否为预处理指令行 (行首空白后跟 '#')
	leadWs := 0
	for leadWs < n && unicode.IsSpace(line[leadWs]) {
		leadWs++
	}

	if i <= leadWs && leadWs < n && line[leadWs] == '#' {
		i = leadWs
		hashStart := i
		i++ // 跳过 '#'

		for i < n && unicode.IsSpace(line[i]) {
			i++
		}
		for i < n && (unicode.IsLetter(line[i]) || line[i] == '_') {
			i++
		}

		spans = append(spans, Span{
			StartCol: hashStart,
			EndCol:   i,
			Color:    h.colors.Preprocessor,
			Bold:     true,
		})
	}

	// 3. 常规词法扫描
	for i < n {
		r := line[i]

		// 3.1 跳过空白字符
		if unicode.IsSpace(r) {
			i++
			continue
		}

		// 3.2 单行注释与 XML 注释 // ... 或 /// ...
		if r == '/' && i+1 < n && line[i+1] == '/' {
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   n,
				Color:    h.colors.Comment,
				Italic:   true,
			})
			return spans, StateCSNormal
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
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   n,
					Color:    h.colors.Comment,
					Italic:   true,
				})
				return spans, StateCSBlockComment
			}
			continue
		}

		// 3.4 逐字/插值逐字字符串: @"..." 或 $@"..." 或 @$"..."
		isVerbatim := false
		startVerbatim := i
		if r == '@' && i+1 < n && line[i+1] == '"' {
			isVerbatim = true
			i += 2
		} else if (r == '$' && i+2 < n && line[i+1] == '@' && line[i+2] == '"') ||
			(r == '@' && i+2 < n && line[i+1] == '$' && line[i+2] == '"') {
			isVerbatim = true
			i += 3
		}

		if isVerbatim {
			closed := false
			for i < n {
				if line[i] == '"' {
					if i+1 < n && line[i+1] == '"' {
						i += 2
						continue
					}
					i++
					closed = true
					break
				}
				i++
			}
			if closed {
				spans = append(spans, Span{
					StartCol: startVerbatim,
					EndCol:   i,
					Color:    h.colors.String,
				})
			} else {
				spans = append(spans, Span{
					StartCol: startVerbatim,
					EndCol:   n,
					Color:    h.colors.String,
				})
				return spans, StateCSVerbatimString
			}
			continue
		}

		// 3.5 插值字符串与常规双引号字符串: $"..." 或 "..."
		if r == '"' || (r == '$' && i+1 < n && line[i+1] == '"') {
			start := i
			if r == '$' {
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

		// 3.6 字符字面量 '...'
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

		// 3.7 数值: 123, 0xFF, 0b101, 3.14f, 1_000_000m, 100UL
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

			// C# 常见数值类型后缀: f, F, d, D, m, M, u, U, l, L, ul, UL
			for i < n && (line[i] == 'f' || line[i] == 'F' || line[i] == 'd' || line[i] == 'D' ||
				line[i] == 'm' || line[i] == 'M' || line[i] == 'u' || line[i] == 'U' ||
				line[i] == 'l' || line[i] == 'L') {
				i++
			}

			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Number,
			})
			continue
		}

		// 3.8 标识符与关键字
		if unicode.IsLetter(r) || r == '_' || (r == '@' && i+1 < n && unicode.IsLetter(line[i+1])) {
			start := i
			if r == '@' {
				i++ // 原生关键字转义符 @class
			}
			for i < n && (unicode.IsLetter(line[i]) || unicode.IsDigit(line[i]) || line[i] == '_') {
				i++
			}
			word := string(line[start:i])
			if word != "" && word[0] == '@' {
				word = word[1:]
			}

			if csKeywords[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Keyword,
					Bold:     true,
				})
			} else if csConstants[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Constant,
					Bold:     true,
				})
			} else if csBuiltinTypes[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Type,
				})
			} else {
				// 检查后续是否紧随 '(' 判定为函数/方法调用
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

		// 3.9 三字符操作符
		if i+2 < n {
			three := string(line[i : i+3])
			if three == "??=" || three == "<<=" || three == ">>=" {
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   i + 3,
					Color:    h.colors.Operator,
				})
				i += 3
				continue
			}
		}

		// 3.10 双字符操作符
		if i+1 < n {
			two := string(line[i : i+2])
			if two == "=>" || two == "??" || two == "?." || two == "::" ||
				two == "++" || two == "--" || two == "==" || two == "!=" ||
				two == "<=" || two == ">=" || two == "&&" || two == "||" ||
				two == "<<" || two == ">>" || two == "+=" || two == "-=" ||
				two == "*=" || two == "/=" || two == "%=" || two == "&=" ||
				two == "|=" || two == "^=" {
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   i + 2,
					Color:    h.colors.Operator,
				})
				i += 2
				continue
			}
		}

		// 3.11 单字符操作符与标点符号
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

	return spans, StateCSNormal
}
