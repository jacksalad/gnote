// Package syntax 提供纯 Go 实现的代码语法高亮与词法分词引擎
package syntax

import (
	"unicode"

	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/theme"
)

// CSS 跨行状态常量。状态值 = 2 × 花括号深度 + 奇偶位 (1 表示处于 /* ... */ 块注释中)：
// 规则块内的注释因此记得自己还在块里，嵌套规则闭合之后也仍然留在上一层声明块中。
const (
	StateCSSNormal       = 0
	StateCSSBlockComment = 1 // 规则块外，处于 /* ... */ 跨行注释中
	StateCSSRuleBlock    = 2 // 处于 { ... } 样式声明规则块内部 (第一层；每深一层加 2)
)

// cssMaxDepth 花括号深度的编码上限，只用于防止外来的极大状态值相加后溢出
const cssMaxDepth = 1 << 20

// cssSelectorUnknown 表示尚未判断当前这条语句是不是嵌套规则的选择器
const cssSelectorUnknown = -2

// cssState 把花括号深度与“是否处于块注释”编码为跨行状态
func cssState(depth int, inComment bool) int {
	if depth > cssMaxDepth {
		depth = cssMaxDepth
	}
	if inComment {
		return depth*2 + 1
	}
	return depth * 2
}

// cssSelectorEnd 从 i 起寻找这条语句的终结符 '{' ';' '}' (越过字符串与注释)。
// 是 '{' 说明这条语句是嵌套规则的选择器，返回它的位置；否则返回 -1。
func cssSelectorEnd(line []rune, i, n int) int {
	for i < n {
		switch c := line[i]; c {
		case '{':
			return i
		case ';', '}':
			return -1
		case '"', '\'':
			for i++; i < n && line[i] != c; i++ {
				if line[i] == '\\' && i+1 < n {
					i++
				}
			}
			i++
		case '/':
			if i+1 >= n || line[i+1] != '*' {
				i++
				continue
			}
			for i += 2; i < n; i++ {
				if line[i] == '*' && i+1 < n && line[i+1] == '/' {
					i += 2
					break
				}
			}
		default:
			i++
		}
	}
	return -1
}

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
	// 负数不是合法状态，按规则块外的普通文本处理
	if prevLineState < 0 {
		prevLineState = StateCSSNormal
	}
	depth := prevLineState / 2
	selEnd := cssSelectorUnknown

	// 1. 处理上一行延续的块注释状态 /* ... */ (规则块内外的注释都带着当时的花括号深度)
	if prevLineState%2 == 1 {
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
			return spans, prevLineState
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
				return spans, cssState(depth, true)
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

		// 2.6 规则块开启 '{' 与闭合 '}': 是否在规则块内按花括号深度判断，
		// 嵌套的内层规则闭合之后，外层声明块仍在继续
		if r == '{' {
			depth++
			selEnd = cssSelectorUnknown
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
			if depth > 0 {
				depth--
			}
			selEnd = cssSelectorUnknown
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   i + 1,
				Color:    h.colors.Punctuation,
				Bold:     true,
			})
			i++
			continue
		}

		// 2.7 在规则块内部 ({ ... } 声明部分)。块内一条语句若以 '{' 收尾 (之前没有 ';' 或 '}')，
		// 它是嵌套规则或 @media 块内那条规则的选择器，改按 2.8 的选择器规则着色
		inDeclaration := depth > 0
		if inDeclaration {
			if selEnd == cssSelectorUnknown {
				selEnd = cssSelectorEnd(line, i, n)
			}
			inDeclaration = i >= selEnd
		}

		// 数值与带单位尺寸: 10px, 1.5rem, 100%, 0.5s, 45deg。
		// 规则块外只认数字起始 ('.' 在那里是类选择器)，@media (max-width: 768px) 这样的条件由此着色
		if unicode.IsDigit(r) || (inDeclaration && r == '.' && i+1 < n && unicode.IsDigit(line[i+1])) {
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

		if inDeclaration {
			// 冒号与分号
			if r == ':' || r == ';' || r == ',' {
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   i + 1,
					Color:    h.colors.Punctuation,
				})
				if r == ';' {
					selEnd = cssSelectorUnknown // 一条声明结束，下一条语句重新判断
				}
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

		// 2.8 在规则块外部 (选择器部分: .btn, #header, div, :hover, 嵌套的 & 等)
		if r == '.' || r == '#' || r == ':' || r == '&' {
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

	return spans, cssState(depth, false)
}
