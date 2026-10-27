// Package syntax 提供纯 Go 实现的代码语法高亮与词法分词引擎
package syntax

import (
	"unicode"

	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/theme"
)

// Java 跨行状态常量
const (
	StateJavaNormal       = 0
	StateJavaBlockComment = 1 // 处于 /* ... */ 或 /** ... */ 跨行注释中
	StateJavaTextBlock    = 2 // 处于 """...""" 多行文本块中
)

// JavaColors Java 语法高亮调色板
type JavaColors struct {
	Keyword     graphics.Color // class, public, return, if, else, for, import...
	Annotation  graphics.Color // @Override, @Deprecated, @Test...
	Type        graphics.Color // int, boolean, String, List, Map, Object...
	Function    graphics.Color // 函数/方法定义与调用
	Constant    graphics.Color // true, false, null, this, super
	String      graphics.Color // "str", 'c', """text block"""
	Number      graphics.Color // 123, 0xFF, 3.14f, 1_000_000L
	Comment     graphics.Color // // comment, /* comment */, /** javadoc */
	Operator    graphics.Color // +, -, *, ->, ::, ==, !=, &&, ||...
	Punctuation graphics.Color // {, }, (, ), [, ], ;, ,, .
}

// DefaultDarkJavaColors 暗色现代调色板
var DefaultDarkJavaColors = javaColorsFromSyntax(theme.Dark().SyntaxColors())

// DefaultLightJavaColors 浅色主题调色板
var DefaultLightJavaColors = javaColorsFromSyntax(theme.Light().SyntaxColors())

// javaColorsFromSyntax 把共享语法令牌映射为 Java 调色板。
func javaColorsFromSyntax(s theme.SyntaxPalette) JavaColors {
	return JavaColors{
		Keyword:     s.Keyword,
		Annotation:  s.Meta,
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
	javaKeywords = map[string]bool{
		"abstract": true, "assert": true, "break": true, "case": true,
		"catch": true, "class": true, "const": true, "continue": true,
		"default": true, "do": true, "else": true, "enum": true,
		"extends": true, "final": true, "finally": true, "for": true,
		"goto": true, "if": true, "implements": true, "import": true,
		"instanceof": true, "interface": true, "native": true, "new": true,
		"package": true, "private": true, "protected": true, "public": true,
		"return": true, "static": true, "strictfp": true, "switch": true,
		"synchronized": true, "throw": true, "throws": true, "transient": true,
		"try": true, "volatile": true, "while": true, "yield": true,
		"record": true, "sealed": true, "non-sealed": true, "permits": true,
		"var": true,
	}

	javaConstants = map[string]bool{
		"true": true, "false": true, "null": true, "this": true, "super": true,
	}

	javaBuiltinTypes = map[string]bool{
		// 基础类型
		"byte": true, "short": true, "int": true, "long": true,
		"float": true, "double": true, "char": true, "boolean": true, "void": true,
		// 核心常用类
		"String": true, "Object": true, "Class": true, "System": true,
		"Integer": true, "Long": true, "Double": true, "Float": true,
		"Boolean": true, "Character": true, "Byte": true, "Short": true, "Number": true,
		// 集合
		"List": true, "ArrayList": true, "LinkedList": true,
		"Map": true, "HashMap": true, "LinkedHashMap": true, "TreeMap": true,
		"Set": true, "HashSet": true, "TreeSet": true,
		"Queue": true, "Deque": true, "ArrayDeque": true, "Stack": true, "Vector": true,
		"Collections": true, "Arrays": true, "Objects": true,
		// 并发与函数式
		"Thread": true, "Runnable": true, "Callable": true, "Future": true,
		"CompletableFuture": true, "Executor": true, "ExecutorService": true,
		"Optional": true, "Stream": true, "Function": true, "Consumer": true,
		"Supplier": true, "Predicate": true, "BiFunction": true, "BiConsumer": true, "BiPredicate": true,
		// 常见异常
		"Exception": true, "Throwable": true, "Error": true, "RuntimeException": true,
		"IllegalArgumentException": true, "NullPointerException": true,
		"IllegalStateException": true, "IOException": true,
	}
)

// JavaHighlighter Java 语法高亮器
type JavaHighlighter struct {
	isDark bool
	colors JavaColors
}

// NewJavaHighlighter 创建默认暗色 Java 语法高亮器
func NewJavaHighlighter() *JavaHighlighter {
	h := &JavaHighlighter{
		isDark: true,
		colors: DefaultDarkJavaColors,
	}
	h.ApplyTheme(theme.Current())
	return h
}

// GetColors 获取当前高亮颜色配置
func (h *JavaHighlighter) GetColors() JavaColors {
	return h.colors
}

// SetColors 自定义设置高亮颜色
func (h *JavaHighlighter) SetColors(c JavaColors) {
	h.colors = c
}

// ApplyTheme 根据当前主题自动适配调色板
func (h *JavaHighlighter) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	h.colors = javaColorsFromSyntax(t.SyntaxColors())
	h.isDark = t.Dark
}

// HighlightLine 对单行 Java 源码执行状态机词法分析并生成样式片段
func (h *JavaHighlighter) HighlightLine(lineIndex int, line []rune, prevLineState int) ([]Span, int) {
	n := len(line)
	if n == 0 {
		return nil, prevLineState
	}

	var spans []Span
	i := 0

	// 1. 处理上一行延续的跨行状态
	if prevLineState == StateJavaBlockComment {
		// 跨行注释 /* ... */ 或 /** ... */
		closeIdx := -1
		for j := 0; j < n-1; j++ {
			if line[j] == '*' && line[j+1] == '/' {
				closeIdx = j + 2
				break
			}
		}

		if closeIdx == -1 {
			// 整行依然处于块注释中
			spans = append(spans, Span{
				StartCol: 0,
				EndCol:   n,
				Color:    h.colors.Comment,
				Italic:   true,
			})
			return spans, StateJavaBlockComment
		}

		// 块注释在本行闭合
		spans = append(spans, Span{
			StartCol: 0,
			EndCol:   closeIdx,
			Color:    h.colors.Comment,
			Italic:   true,
		})
		i = closeIdx
	} else if prevLineState == StateJavaTextBlock {
		// 跨行多行文本块 """..."""
		closeIdx := -1
		for j := 0; j < n-2; j++ {
			if line[j] == '"' && line[j+1] == '"' && line[j+2] == '"' {
				// 检查前面反斜杠是否为奇数个 (转义)
				slashCount := 0
				for k := j - 1; k >= 0 && line[k] == '\\'; k-- {
					slashCount++
				}
				if slashCount%2 == 0 {
					closeIdx = j + 3
					break
				}
			}
		}

		if closeIdx == -1 {
			// 整行依然处于多行文本块中
			spans = append(spans, Span{
				StartCol: 0,
				EndCol:   n,
				Color:    h.colors.String,
			})
			return spans, StateJavaTextBlock
		}

		// 文本块在本行闭合
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

		// 2.1 跳过空白字符
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
			return spans, StateJavaNormal
		}

		// 2.3 块注释 /* ... */ 或 /** ... */
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
				return spans, StateJavaBlockComment
			}
			continue
		}

		// 2.4 Java 15+ 多行文本块 """..."""
		if r == '"' && i+2 < n && line[i+1] == '"' && line[i+2] == '"' {
			start := i
			i += 3
			closed := false
			for i < n-2 {
				if line[i] == '"' && line[i+1] == '"' && line[i+2] == '"' {
					slashCount := 0
					for k := i - 1; k >= start && line[k] == '\\'; k-- {
						slashCount++
					}
					if slashCount%2 == 0 {
						i += 3
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
				return spans, StateJavaTextBlock
			}
			continue
		}

		// 2.5 常规单行字符串 "..."
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

		// 2.6 字符字面量 '...'
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

		// 2.7 注解 @AnnotationName
		if r == '@' {
			start := i
			i++
			for i < n && (unicode.IsLetter(line[i]) || unicode.IsDigit(line[i]) || line[i] == '_' || line[i] == '.') {
				i++
			}
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Annotation,
				Bold:     true,
			})
			continue
		}

		// 2.8 数值字面量: 123, 0xFF, 0b101, 3.14f, 100L, 1_000_000
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

			// 匹配类型后缀: L, l, f, F, d, D
			for i < n && (line[i] == 'L' || line[i] == 'l' || line[i] == 'f' || line[i] == 'F' || line[i] == 'd' || line[i] == 'D') {
				i++
			}

			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Number,
			})
			continue
		}

		// 2.9 标识符与关键字
		if unicode.IsLetter(r) || r == '_' || r == '$' {
			start := i
			for i < n && (unicode.IsLetter(line[i]) || unicode.IsDigit(line[i]) || line[i] == '_' || line[i] == '$') {
				i++
			}
			word := string(line[start:i])

			if javaKeywords[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Keyword,
					Bold:     true,
				})
			} else if javaConstants[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Constant,
					Bold:     true,
				})
			} else if javaBuiltinTypes[word] {
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

		// 2.10 三字符操作符
		if i+2 < n {
			three := string(line[i : i+3])
			if three == ">>>" || three == "<<=" || three == ">>=" {
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

		// 2.12 单字符操作符与标点符号
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

	return spans, StateJavaNormal
}
