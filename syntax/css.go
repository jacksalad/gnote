// Package syntax 提供纯 Go 实现的代码语法高亮与词法分词引擎
package syntax

import (
	"unicode"

	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/theme"
)

// CSS 跨行状态常量
const (
	StateCSSNormal       = 0
	StateCSSBlockComment = 1 // 处于 /* ... */ 跨行注释中
	StateCSSRuleBlock    = 2 // 处于 { ... } 样式声明规则块内部
)

// CSSColors CSS 语法高亮调色板
type CSSColors struct {
	Selector    graphics.Color // .btn, #header, div, :hover, ::after...
	Property    graphics.Color // color, margin, padding, display, --custom-var...
	Value       graphics.Color // block, none, relative, auto, center...
	Number      graphics.Color // 12px, 1.5rem, 100%, #ffffff...
	String      graphics.Color // "font", 'str', url(...)
	AtRule      graphics.Color // @media, @keyframes, @import, @font-face...
	Important   graphics.Color // !important
	Comment     graphics.Color // /* comment */
	Punctuation graphics.Color // {, }, :, ;, ,, .
}

// DefaultDarkCSSColors 暗色现代调色板
var DefaultDarkCSSColors = cssColorsFromSyntax(theme.Dark().SyntaxColors())

// DefaultLightCSSColors 浅色主题调色板
var DefaultLightCSSColors = cssColorsFromSyntax(theme.Light().SyntaxColors())

// cssColorsFromSyntax 把共享语法令牌映射为 CSS 调色板。
func cssColorsFromSyntax(s theme.SyntaxPalette) CSSColors {
	return CSSColors{
		Selector:    s.Type,
		Property:    s.Keyword,
		Value:       s.Builtin,
		Number:      s.Number,
		String:      s.String,
		AtRule:      s.Meta,
		Important:   s.Constant,
		Comment:     s.Comment,
		Punctuation: s.Punctuation,
	}
}

// CSSHighlighter CSS 语法高亮器
type CSSHighlighter struct {
	isDark bool
	colors CSSColors
}

// NewCSSHighlighter 创建默认 CSS 语法高亮器
func NewCSSHighlighter() *CSSHighlighter {
	h := &CSSHighlighter{
		isDark: true,
		colors: DefaultDarkCSSColors,
	}
	h.ApplyTheme(theme.Current())
	return h
}

// GetColors 获取当前高亮颜色配置
func (h *CSSHighlighter) GetColors() CSSColors {
	return h.colors
}

// SetColors 自定义设置高亮颜色
func (h *CSSHighlighter) SetColors(c CSSColors) {
	h.colors = c
}

// ApplyTheme 根据当前主题自动适配调色板
func (h *CSSHighlighter) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	h.colors = cssColorsFromSyntax(t.SyntaxColors())
	h.isDark = t.Dark
}

// HighlightLine 对单行 CSS 源码执行状态机词法分析并生成样式片段
func (h *CSSHighlighter) HighlightLine(lineIndex int, line []rune, prevLineState int) ([]Span, int) {
	n := len(line)
	if n == 0 {
		return nil, prevLineState
	}

	var spans []Span
	i := 0
	inBlock := (prevLineState == StateCSSRuleBlock)

	// 1. 处理上一行延续的块注释状态 /* ... */
	if prevLineState == StateCSSBlockComment {
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
			return spans, StateCSSBlockComment
		}

		spans = append(spans, Span{
			StartCol: 0,
			EndCol:   closeIdx,
			Color:    h.colors.Comment,
			Italic:   true,
		})
		i = closeIdx
	}

	// 2. 状态机分词循环
	for i < n {
		r := line[i]

		// 2.1 空白字符
		if unicode.IsSpace(r) {
			i++
			continue
		}

		// 2.2 块注释 /* ... */
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
				return spans, StateCSSBlockComment
			}
			continue
		}

		// 2.3 At-Rules: @media, @import, @keyframes, @font-face...
		if r == '@' {
			start := i
			i++
			for i < n && (unicode.IsLetter(line[i]) || line[i] == '-') {
				i++
			}
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.AtRule,
				Bold:     true,
			})
			continue
		}

		// 2.4 !important
		if r == '!' {
			start := i
			i++
			for i < n && unicode.IsLetter(line[i]) {
				i++
			}
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Important,
				Bold:     true,
			})
			continue
		}

		// 2.5 字符串字面量 "..." 或 '...'
		if r == '"' || r == '\'' {
			quote := r
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
				if curr == quote {
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

		// 2.6 规则块开启 '{' 与闭合 '}'
		if r == '{' {
			inBlock = true
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   i + 1,
				Color:    h.colors.Punctuation,
				Bold:     true,
			})
			i++
			continue
		}
		if r == '}' {
			inBlock = false
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   i + 1,
				Color:    h.colors.Punctuation,
				Bold:     true,
			})
			i++
			continue
		}

		// 2.7 在规则块内部 ({ ... } 声明部分)
		if inBlock {
			// 冒号与分号
			if r == ':' || r == ';' || r == ',' {
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   i + 1,
					Color:    h.colors.Punctuation,
				})
				i++
				continue
			}

			// 十六进制颜色: #ffffff, #333
			if r == '#' {
				start := i
				i++
				for i < n && (unicode.IsDigit(line[i]) || (line[i] >= 'a' && line[i] <= 'f') || (line[i] >= 'A' && line[i] <= 'F')) {
					i++
				}
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Number,
				})
				continue
			}

			// 数值与带单位尺寸: 10px, 1.5rem, 100%, 0.5s, 45deg
			if unicode.IsDigit(r) || (r == '.' && i+1 < n && unicode.IsDigit(line[i+1])) {
				start := i
				for i < n && (unicode.IsDigit(line[i]) || line[i] == '.') {
					i++
				}
				// 匹配单位后缀: px, rem, em, %, vh, vw, s, ms, deg, fr, pt 等
				for i < n && (unicode.IsLetter(line[i]) || line[i] == '%') {
					i++
				}
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Number,
				})
				continue
			}

			// 标识符: 可能是属性名、属性值、或者函数名 (如 rgb, calc, var, url)
			if unicode.IsLetter(r) || r == '-' || r == '_' {
				start := i
				for i < n && (unicode.IsLetter(line[i]) || unicode.IsDigit(line[i]) || line[i] == '-' || line[i] == '_') {
					i++
				}

				// 检查后续是否紧跟 ':' 且不是伪类，判定为属性名 (如 color:, margin:)
				peek := i
				for peek < n && unicode.IsSpace(line[peek]) {
					peek++
				}

				if peek < n && line[peek] == ':' {
					spans = append(spans, Span{
						StartCol: start,
						EndCol:   i,
						Color:    h.colors.Property,
					})
				} else if peek < n && line[peek] == '(' {
					// 函数调用 (如 rgb, var, calc, url)
					spans = append(spans, Span{
						StartCol: start,
						EndCol:   i,
						Color:    h.colors.AtRule,
					})
				} else {
					// 属性值关键字 (如 block, relative, none, solid, red, inherit)
					spans = append(spans, Span{
						StartCol: start,
						EndCol:   i,
						Color:    h.colors.Value,
					})
				}
				continue
			}

			// 其他单字符标点
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   i + 1,
				Color:    h.colors.Punctuation,
			})
			i++
			continue
		}

		// 2.8 在规则块外部 (选择器部分: .btn, #header, div, :hover 等)
		if r == '.' || r == '#' || r == ':' {
			start := i
			i++
			// 支持 :: 伪元素
			if r == ':' && i < n && line[i] == ':' {
				i++
			}
			for i < n && (unicode.IsLetter(line[i]) || unicode.IsDigit(line[i]) || line[i] == '-' || line[i] == '_' || line[i] == '(' || line[i] == ')') {
				i++
			}
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Selector,
				Bold:     true,
			})
			continue
		}

		// 标签选择器或组合符 (div, span, body, >, +, ~)
		if unicode.IsLetter(r) {
			start := i
			for i < n && (unicode.IsLetter(line[i]) || unicode.IsDigit(line[i]) || line[i] == '-') {
				i++
			}
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Selector,
				Bold:     true,
			})
			continue
		}

		// 标点与组合选择符
		spans = append(spans, Span{
			StartCol: i,
			EndCol:   i + 1,
			Color:    h.colors.Punctuation,
		})
		i++
	}

	if inBlock {
		return spans, StateCSSRuleBlock
	}
	return spans, StateCSSNormal
}
