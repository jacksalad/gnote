package syntax

import (
	"strings"
	"unicode"

	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/theme"
)

// Markdown 跨行解析状态常量
const (
	StateNormal          = 0 // 普通文本行
	StateFencedCodeBlock = 1 // 处于多行围栏代码块内部 (``` 或 ~~~)
)

// MarkdownColors 定义 Markdown 各语法元素的调色板令牌
type MarkdownColors struct {
	H1               graphics.Color
	H2               graphics.Color
	H3               graphics.Color
	H4               graphics.Color
	HMarker          graphics.Color
	CodeBlockText    graphics.Color
	CodeBlockBg      graphics.Color
	CodeBlockMarker  graphics.Color
	CodeLang         graphics.Color
	InlineCodeText   graphics.Color
	InlineCodeBg     graphics.Color
	InlineCodeMarker graphics.Color
	QuoteText        graphics.Color
	QuoteMarker      graphics.Color
	ListMarker       graphics.Color
	TaskChecked      graphics.Color
	TaskUnchecked    graphics.Color
	BoldText         graphics.Color
	BoldMarker       graphics.Color
	ItalicText       graphics.Color
	StrikeText       graphics.Color
	LinkText         graphics.Color
	LinkUrl          graphics.Color
	ImageMarker      graphics.Color
	Hr               graphics.Color
}

// DefaultDarkMarkdownColors 暗色主题预设调色板
var DefaultDarkMarkdownColors = markdownColorsFromSyntax(theme.Dark().SyntaxColors())

// DefaultLightMarkdownColors 浅色主题预设调色板
var DefaultLightMarkdownColors = markdownColorsFromSyntax(theme.Light().SyntaxColors())

// markdownColorsFromSyntax 把共享语法令牌映射为 Markdown 调色板。
func markdownColorsFromSyntax(s theme.SyntaxPalette) MarkdownColors {
	return MarkdownColors{
		H1:               s.Heading1,
		H2:               s.Heading2,
		H3:               s.Heading3,
		H4:               s.Heading4,
		HMarker:          s.HeadingMarker,
		CodeBlockText:    s.CodeBlockText,
		CodeBlockBg:      s.CodeBlockBg,
		CodeBlockMarker:  s.CodeBlockMarker,
		CodeLang:         s.CodeLang,
		InlineCodeText:   s.InlineCodeText,
		InlineCodeBg:     s.InlineCodeBg,
		InlineCodeMarker: s.InlineCodeMarker,
		QuoteText:        s.QuoteText,
		QuoteMarker:      s.QuoteMarker,
		ListMarker:       s.ListMarker,
		TaskChecked:      s.TaskChecked,
		TaskUnchecked:    s.TaskUnchecked,
		BoldText:         s.BoldText,
		BoldMarker:       s.BoldMarker,
		ItalicText:       s.ItalicText,
		StrikeText:       s.StrikeText,
		LinkText:         s.LinkText,
		LinkUrl:          s.LinkUrl,
		ImageMarker:      s.ImageMarker,
		Hr:               s.Hr,
	}
}

// MarkdownHighlighter 基础 Markdown 语法高亮器
type MarkdownHighlighter struct {
	colors MarkdownColors
	isDark bool
}

// NewMarkdownHighlighter 创建 Markdown 语法高亮器
func NewMarkdownHighlighter() *MarkdownHighlighter {
	h := &MarkdownHighlighter{
		isDark: true,
		colors: DefaultDarkMarkdownColors,
	}
	h.ApplyTheme(theme.Current())
	return h
}

// ApplyTheme 跟随编辑器全局主题动态切换高亮色彩
func (h *MarkdownHighlighter) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	h.colors = markdownColorsFromSyntax(t.SyntaxColors())
	h.isDark = t.Dark
}

// SetColors 自定义设置高亮调色板
func (h *MarkdownHighlighter) SetColors(colors MarkdownColors) {
	h.colors = colors
}

// GetColors 获取当前高亮调色板
func (h *MarkdownHighlighter) GetColors() MarkdownColors {
	return h.colors
}

// HighlightLine 对指定单行进行词法解析
func (h *MarkdownHighlighter) HighlightLine(lineIndex int, line []rune, prevLineState int) ([]Span, int) {
	n := len(line)
	if n == 0 {
		return nil, prevLineState
	}

	// 1. 处理多行围栏代码块状态
	if prevLineState == StateFencedCodeBlock {
		// 检查是否为代码块结束标记 (以 ``` 或 ~~~ 开头)
		trimmed := strings.TrimLeft(string(line), " \t")
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			spans := []Span{
				{
					StartCol: 0,
					EndCol:   n,
					Color:    h.colors.CodeBlockMarker,
					BgColor:  h.colors.CodeBlockBg,
					Bold:     true,
				},
			}
			return spans, StateNormal
		}
		// 处于代码块内部
		spans := []Span{
			{
				StartCol: 0,
				EndCol:   n,
				Color:    h.colors.CodeBlockText,
				BgColor:  h.colors.CodeBlockBg,
			},
		}
		return spans, StateFencedCodeBlock
	}

	// 2. 检查多行代码块开启行 (``` 或 ~~~ 开头)
	trimmed := strings.TrimLeft(string(line), " \t")
	if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
		indent := len(line) - len([]rune(trimmed))
		markerLen := 3
		for indent+markerLen < n && (line[indent+markerLen] == '`' || line[indent+markerLen] == '~') {
			markerLen++
		}
		var spans []Span
		// 标记符号
		spans = append(spans, Span{
			StartCol: indent,
			EndCol:   indent + markerLen,
			Color:    h.colors.CodeBlockMarker,
			BgColor:  h.colors.CodeBlockBg,
			Bold:     true,
		})
		// 语言标识符
		if indent+markerLen < n {
			spans = append(spans, Span{
				StartCol: indent + markerLen,
				EndCol:   n,
				Color:    h.colors.CodeLang,
				BgColor:  h.colors.CodeBlockBg,
				Bold:     true,
			})
		}
		return spans, StateFencedCodeBlock
	}

	// 3. 标题行 (# H1, ## H2, ### H3, #### H4 ...)
	if hLevel, prefixLen := parseHeading(line); hLevel > 0 {
		var spans []Span
		var hColor graphics.Color
		switch hLevel {
		case 1:
			hColor = h.colors.H1
		case 2:
			hColor = h.colors.H2
		case 3:
			hColor = h.colors.H3
		default:
			hColor = h.colors.H4
		}

		// # 符号标记
		spans = append(spans, Span{
			StartCol: 0,
			EndCol:   prefixLen,
			Color:    h.colors.HMarker,
			Bold:     true,
		})
		// 标题正文
		if prefixLen < n {
			spans = append(spans, Span{
				StartCol: prefixLen,
				EndCol:   n,
				Color:    hColor,
				Bold:     true,
			})
		}
		return spans, StateNormal
	}

	// 4. 水平分割线 (---, ***, ___)
	if isHorizontalRule(line) {
		spans := []Span{
			{
				StartCol: 0,
				EndCol:   n,
				Color:    h.colors.Hr,
				Bold:     true,
			},
		}
		return spans, StateNormal
	}

	// 5. 引用块 (> Quote)
	if qLevel, prefixLen := parseBlockquote(line); qLevel > 0 {
		var spans []Span
		spans = append(spans, Span{
			StartCol: 0,
			EndCol:   prefixLen,
			Color:    h.colors.QuoteMarker,
			Bold:     true,
		})
		// 引用正文行内解析
		if prefixLen < n {
			inlines := h.parseInlines(line, prefixLen, n, h.colors.QuoteText)
			spans = append(spans, inlines...)
		}
		return spans, StateNormal
	}

	// 6. 列表行 (- / * / + / 1. / - [ ] / - [x])
	if listType, prefixEnd, isTask, taskChecked := parseList(line); listType > 0 {
		var spans []Span
		if isTask {
			// 任务列表
			spans = append(spans, Span{
				StartCol: 0,
				EndCol:   prefixEnd - 4, // 比如 "- "
				Color:    h.colors.ListMarker,
				Bold:     true,
			})
			taskColor := h.colors.TaskUnchecked
			if taskChecked {
				taskColor = h.colors.TaskChecked
			}
			spans = append(spans, Span{
				StartCol: prefixEnd - 4,
				EndCol:   prefixEnd, // "[ ] "
				Color:    taskColor,
				Bold:     true,
			})
		} else {
			// 普通无序/有序列表
			spans = append(spans, Span{
				StartCol: 0,
				EndCol:   prefixEnd,
				Color:    h.colors.ListMarker,
				Bold:     true,
			})
		}

		// 列表项正文行内解析
		if prefixEnd < n {
			inlines := h.parseInlines(line, prefixEnd, n, 0)
			spans = append(spans, inlines...)
		}
		return spans, StateNormal
	}

	// 7. 普通段落行的行内高亮解析 (Inline Code, Bold, Italic, Links, Images, HTML)
	spans := h.parseInlines(line, 0, n, 0)
	return spans, StateNormal
}

// parseHeading 检测行是否为 Markdown 标题 (# 到 ######)
func parseHeading(line []rune) (level int, prefixLen int) {
	i := 0
	for i < len(line) && line[i] == ' ' && i < 3 {
		i++
	}
	start := i
	for i < len(line) && line[i] == '#' {
		i++
	}
	hashes := i - start
	if hashes >= 1 && hashes <= 6 {
		if i == len(line) {
			return hashes, i
		}
		if line[i] == ' ' || line[i] == '\t' {
			// 包含空格分隔符
			for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
				i++
			}
			return hashes, i
		}
	}
	return 0, 0
}

// isHorizontalRule 判定是否为分割线 (---, ***, ___)
func isHorizontalRule(line []rune) bool {
	count := 0
	var char rune = 0
	for _, r := range line {
		if r == ' ' || r == '\t' {
			continue
		}
		if r == '-' || r == '*' || r == '_' {
			if char == 0 {
				char = r
			} else if char != r {
				return false
			}
			count++
		} else {
			return false
		}
	}
	return count >= 3
}

// parseBlockquote 检测引用行 (> text)
func parseBlockquote(line []rune) (level int, prefixLen int) {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	if i < len(line) && line[i] == '>' {
		i++
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		return 1, i
	}
	return 0, 0
}

// parseList 检测列表标记 (无序列表、有序列表、任务列表)
func parseList(line []rune) (listType int, prefixEnd int, isTask bool, taskChecked bool) {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	if i >= len(line) {
		return 0, 0, false, false
	}

	// 1. 无序列表 (-, *, +)
	if (line[i] == '-' || line[i] == '*' || line[i] == '+') && i+1 < len(line) && (line[i+1] == ' ' || line[i+1] == '\t') {
		markerEnd := i + 2
		for markerEnd < len(line) && (line[markerEnd] == ' ' || line[markerEnd] == '\t') {
			markerEnd++
		}

		// 检查任务列表 (- [ ] 或 - [x])
		if markerEnd+3 <= len(line) && line[markerEnd-1] == ' ' && line[markerEnd] == '[' && (markerEnd+3 < len(line) && line[markerEnd+2] == ']') {
			stateR := line[markerEnd+1]
			checked := stateR == 'x' || stateR == 'X'
			afterBox := markerEnd + 3
			if afterBox < len(line) && line[afterBox] == ' ' {
				afterBox++
			}
			return 1, afterBox, true, checked
		}

		return 1, markerEnd, false, false
	}

	// 2. 有序列表 (1. 2. 10.)
	digitStart := i
	for i < len(line) && unicode.IsDigit(line[i]) {
		i++
	}
	if i > digitStart && i < len(line) && (line[i] == '.' || line[i] == ')') && i+1 < len(line) && (line[i+1] == ' ' || line[i+1] == '\t') {
		i += 2
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		return 2, i, false, false
	}

	return 0, 0, false, false
}

// parseInlines 扫描解析行内的代码、强调、链接、图片等标记
func (h *MarkdownHighlighter) parseInlines(line []rune, start, end int, defaultColor graphics.Color) []Span {
	var spans []Span
	i := start

	for i < end {
		r := line[i]

		// 1. 行内代码 (`code`)
		if r == '`' {
			codeEnd := -1
			for j := i + 1; j < end; j++ {
				if line[j] == '`' {
					codeEnd = j
					break
				}
			}
			if codeEnd != -1 {
				// 反引号标记
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   i + 1,
					Color:    h.colors.InlineCodeMarker,
					BgColor:  h.colors.InlineCodeBg,
				})
				// 代码内容
				if codeEnd > i+1 {
					spans = append(spans, Span{
						StartCol: i + 1,
						EndCol:   codeEnd,
						Color:    h.colors.InlineCodeText,
						BgColor:  h.colors.InlineCodeBg,
					})
				}
				// 闭合反引号
				spans = append(spans, Span{
					StartCol: codeEnd,
					EndCol:   codeEnd + 1,
					Color:    h.colors.InlineCodeMarker,
					BgColor:  h.colors.InlineCodeBg,
				})
				i = codeEnd + 1
				continue
			}
		}

		// 2. 图片 (![alt](url))
		if r == '!' && i+1 < end && line[i+1] == '[' {
			closeBracket := -1
			for j := i + 2; j < end; j++ {
				if line[j] == ']' {
					closeBracket = j
					break
				}
			}
			if closeBracket != -1 && closeBracket+1 < end && line[closeBracket+1] == '(' {
				closeParen := -1
				for j := closeBracket + 2; j < end; j++ {
					if line[j] == ')' {
						closeParen = j
						break
					}
				}
				if closeParen != -1 {
					// !
					spans = append(spans, Span{
						StartCol: i,
						EndCol:   i + 1,
						Color:    h.colors.ImageMarker,
						Bold:     true,
					})
					// [alt]
					spans = append(spans, Span{
						StartCol: i + 1,
						EndCol:   closeBracket + 1,
						Color:    h.colors.LinkText,
					})
					// (url)
					spans = append(spans, Span{
						StartCol: closeBracket + 1,
						EndCol:   closeParen + 1,
						Color:    h.colors.LinkUrl,
					})
					i = closeParen + 1
					continue
				}
			}
		}

		// 3. 超链接 ([text](url))
		if r == '[' {
			closeBracket := -1
			for j := i + 1; j < end; j++ {
				if line[j] == ']' {
					closeBracket = j
					break
				}
			}
			if closeBracket != -1 && closeBracket+1 < end && line[closeBracket+1] == '(' {
				closeParen := -1
				for j := closeBracket + 2; j < end; j++ {
					if line[j] == ')' {
						closeParen = j
						break
					}
				}
				if closeParen != -1 {
					// [text]
					spans = append(spans, Span{
						StartCol: i,
						EndCol:   closeBracket + 1,
						Color:    h.colors.LinkText,
						Bold:     true,
					})
					// (url)
					spans = append(spans, Span{
						StartCol: closeBracket + 1,
						EndCol:   closeParen + 1,
						Color:    h.colors.LinkUrl,
					})
					i = closeParen + 1
					continue
				}
			}
		}

		// 4. 粗体 (**bold** 或 __bold__)
		if (r == '*' || r == '_') && i+1 < end && line[i+1] == r {
			marker := r
			closeIdx := -1
			for j := i + 2; j+1 < end; j++ {
				if line[j] == marker && line[j+1] == marker {
					closeIdx = j
					break
				}
			}
			if closeIdx != -1 {
				// ** 开头
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   i + 2,
					Color:    h.colors.BoldMarker,
				})
				// 内容
				if closeIdx > i+2 {
					spans = append(spans, Span{
						StartCol: i + 2,
						EndCol:   closeIdx,
						Color:    h.colors.BoldText,
						Bold:     true,
					})
				}
				// ** 结尾
				spans = append(spans, Span{
					StartCol: closeIdx,
					EndCol:   closeIdx + 2,
					Color:    h.colors.BoldMarker,
				})
				i = closeIdx + 2
				continue
			}
		}

		// 5. 斜体 (*italic* 或 _italic_)
		if r == '*' || r == '_' {
			marker := r
			closeIdx := -1
			for j := i + 1; j < end; j++ {
				if line[j] == marker {
					closeIdx = j
					break
				}
			}
			if closeIdx != -1 && closeIdx > i+1 {
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   closeIdx + 1,
					Color:    h.colors.ItalicText,
					Italic:   true,
				})
				i = closeIdx + 1
				continue
			}
		}

		// 6. 删除线 (~~strike~~)
		if r == '~' && i+1 < end && line[i+1] == '~' {
			closeIdx := -1
			for j := i + 2; j+1 < end; j++ {
				if line[j] == '~' && line[j+1] == '~' {
					closeIdx = j
					break
				}
			}
			if closeIdx != -1 {
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   closeIdx + 2,
					Color:    h.colors.StrikeText,
				})
				i = closeIdx + 2
				continue
			}
		}

		// 7. 自动链接 (<https://example.com>)
		// Markdown 文档不解析行内 HTML，标签一律按正文呈现；此处仅保留
		// CommonMark 自动链接，且扫描不跨越空白、行内代码反引号与行尾。
		if r == '<' {
			closeIdx := -1
			for j := i + 1; j < end; j++ {
				if line[j] == '>' {
					closeIdx = j
					break
				}
				if line[j] == '`' || line[j] == ' ' || line[j] == '\t' {
					break
				}
			}
			if closeIdx != -1 {
				target := string(line[i+1 : closeIdx])
				if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
					spans = append(spans, Span{
						StartCol: i,
						EndCol:   closeIdx + 1,
						Color:    h.colors.LinkText,
					})
					i = closeIdx + 1
					continue
				}
			}
		}

		// 普通未匹配字符：如果指定了默认颜色（如引用块内文字），可以向前聚合
		if defaultColor != 0 {
			segStart := i
			for i < end && line[i] != '`' && line[i] != '*' && line[i] != '_' && line[i] != '[' && line[i] != '!' && line[i] != '<' && line[i] != '~' {
				i++
			}
			if i > segStart {
				spans = append(spans, Span{
					StartCol: segStart,
					EndCol:   i,
					Color:    defaultColor,
				})
			}
			continue
		}

		i++
	}

	return spans
}
