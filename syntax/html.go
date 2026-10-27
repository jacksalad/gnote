// Package syntax 提供纯 Go 实现的代码语法高亮与词法分词引擎
package syntax

import (
	"strings"
	"unicode"

	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/theme"
)

// HTML 跨行状态常量
const (
	StateHTMLNormal  = 0
	StateHTMLComment = 1 // 处于 <!-- ... --> 跨行注释中
	StateHTMLTag     = 2 // 处于 <tag attr="val"... 跨行标签内部
)

// HTMLColors HTML 语法高亮调色板
type HTMLColors struct {
	Tag       graphics.Color // div, span, a, body, html...
	Bracket   graphics.Color // <, >, </, />
	Attribute graphics.Color // class, id, href, src, style, data-*...
	Value     graphics.Color // "value", 'value'
	Comment   graphics.Color // <!-- comment -->
	Doctype   graphics.Color // <!DOCTYPE html>
	Entity    graphics.Color // &nbsp;, &lt;, &gt;, &#123;...
	Text      graphics.Color // 标签间常规文本
}

// DefaultDarkHTMLColors 暗色现代调色板
var DefaultDarkHTMLColors = htmlColorsFromSyntax(theme.Dark().SyntaxColors())

// DefaultLightHTMLColors 浅色主题调色板
var DefaultLightHTMLColors = htmlColorsFromSyntax(theme.Light().SyntaxColors())

// htmlColorsFromSyntax 把共享语法令牌映射为 HTML 调色板。
func htmlColorsFromSyntax(s theme.SyntaxPalette) HTMLColors {
	return HTMLColors{
		Tag:       s.Type,
		Bracket:   s.Punctuation,
		Attribute: s.Attribute,
		Value:     s.String,
		Comment:   s.Comment,
		Doctype:   s.Meta,
		Entity:    s.Entity,
		Text:      s.Operator,
	}
}

// HTMLHighlighter HTML 语法高亮器
type HTMLHighlighter struct {
	isDark bool
	colors HTMLColors
}

// NewHTMLHighlighter 创建默认 HTML 语法高亮器
func NewHTMLHighlighter() *HTMLHighlighter {
	h := &HTMLHighlighter{
		isDark: true,
		colors: DefaultDarkHTMLColors,
	}
	h.ApplyTheme(theme.Current())
	return h
}

// GetColors 获取当前高亮颜色配置
func (h *HTMLHighlighter) GetColors() HTMLColors {
	return h.colors
}

// SetColors 自定义设置高亮颜色
func (h *HTMLHighlighter) SetColors(c HTMLColors) {
	h.colors = c
}

// ApplyTheme 根据当前主题自动适配调色板
func (h *HTMLHighlighter) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	h.colors = htmlColorsFromSyntax(t.SyntaxColors())
	h.isDark = t.Dark
}

// HighlightLine 对单行 HTML 源码执行状态机词法分析并生成样式片段
func (h *HTMLHighlighter) HighlightLine(lineIndex int, line []rune, prevLineState int) ([]Span, int) {
	n := len(line)
	if n == 0 {
		return nil, prevLineState
	}

	var spans []Span
	i := 0
	inTag := (prevLineState == StateHTMLTag)

	// 1. 处理上一行延续的注释状态 <!-- ... -->
	if prevLineState == StateHTMLComment {
		closeIdx := -1
		for j := 0; j < n-2; j++ {
			if line[j] == '-' && line[j+1] == '-' && line[j+2] == '>' {
				closeIdx = j + 3
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
			return spans, StateHTMLComment
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

		// 2.1 处于 HTML 标签内部 (<tag attr="val"...>)
		if inTag {
			if unicode.IsSpace(r) {
				i++
				continue
			}

			// 检查标签闭合: > 或 />
			if r == '>' {
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   i + 1,
					Color:    h.colors.Bracket,
				})
				i++
				inTag = false
				continue
			}
			if r == '/' && i+1 < n && line[i+1] == '>' {
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   i + 2,
					Color:    h.colors.Bracket,
				})
				i += 2
				inTag = false
				continue
			}

			// 属性值: 双引号字符串 "..."
			if r == '"' {
				start := i
				i++
				for i < n && line[i] != '"' {
					i++
				}
				if i < n && line[i] == '"' {
					i++
				}
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Value,
				})
				continue
			}

			// 属性值: 单引号字符串 '...'
			if r == '\'' {
				start := i
				i++
				for i < n && line[i] != '\'' {
					i++
				}
				if i < n && line[i] == '\'' {
					i++
				}
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Value,
				})
				continue
			}

			// 等号 '='
			if r == '=' {
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   i + 1,
					Color:    h.colors.Bracket,
				})
				i++
				continue
			}

			// 属性名: 如 class, id, data-*, @click, v-model, :href
			if unicode.IsLetter(r) || r == '_' || r == '@' || r == ':' || r == 'v' || r == '-' {
				start := i
				for i < n && (unicode.IsLetter(line[i]) || unicode.IsDigit(line[i]) ||
					line[i] == '_' || line[i] == '-' || line[i] == ':' || line[i] == '@' || line[i] == '.') {
					i++
				}
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Attribute,
				})
				continue
			}

			i++
			continue
		}

		// 2.2 处于常规区域 (标签外)
		// 检查注释开始: <!--
		if r == '<' && i+3 < n && line[i+1] == '!' && line[i+2] == '-' && line[i+3] == '-' {
			start := i
			i += 4
			closed := false
			for i < n-2 {
				if line[i] == '-' && line[i+1] == '-' && line[i+2] == '>' {
					i += 3
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
				return spans, StateHTMLComment
			}
			continue
		}

		// 检查 <!DOCTYPE ...>
		if r == '<' && i+8 < n && line[i+1] == '!' {
			sub := strings.ToLower(string(line[i : i+9]))
			if sub == "<!doctype" {
				start := i
				for i < n && line[i] != '>' {
					i++
				}
				if i < n && line[i] == '>' {
					i++
				}
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Doctype,
					Bold:     true,
				})
				continue
			}
		}

		// 检查闭合标签: </tag>
		if r == '<' && i+1 < n && line[i+1] == '/' {
			start := i
			i += 2 // 跳过 '</'
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Bracket,
			})

			// 标签名
			tagStart := i
			for i < n && (unicode.IsLetter(line[i]) || unicode.IsDigit(line[i]) || line[i] == '-') {
				i++
			}
			if i > tagStart {
				spans = append(spans, Span{
					StartCol: tagStart,
					EndCol:   i,
					Color:    h.colors.Tag,
					Bold:     true,
				})
			}

			// 跳过可能存在的空格后匹配 '>'
			for i < n && line[i] != '>' {
				i++
			}
			if i < n && line[i] == '>' {
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   i + 1,
					Color:    h.colors.Bracket,
				})
				i++
			}
			continue
		}

		// 检查开始标签: <tag ...>
		if r == '<' && i+1 < n && (unicode.IsLetter(line[i+1]) || line[i+1] == '!') {
			start := i
			i++ // 跳过 '<'
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Bracket,
			})

			// 扫描标签名
			tagStart := i
			for i < n && (unicode.IsLetter(line[i]) || unicode.IsDigit(line[i]) || line[i] == '-') {
				i++
			}
			if i > tagStart {
				spans = append(spans, Span{
					StartCol: tagStart,
					EndCol:   i,
					Color:    h.colors.Tag,
					Bold:     true,
				})
			}

			// 进入标签内部解析状态
			inTag = true
			continue
		}

		// 检查 HTML 实体: &name; 或 &#123;
		if r == '&' {
			start := i
			i++
			for i < n && i-start < 10 && (unicode.IsLetter(line[i]) || unicode.IsDigit(line[i]) || line[i] == '#') {
				i++
			}
			if i < n && line[i] == ';' {
				i++
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Entity,
					Bold:     true,
				})
				continue
			}
			// 若非标准实体，按常规文本推进
			i = start + 1
			continue
		}

		// 常规正文文本
		textStart := i
		for i < n && line[i] != '<' && line[i] != '&' {
			i++
		}
		if i > textStart {
			spans = append(spans, Span{
				StartCol: textStart,
				EndCol:   i,
				Color:    h.colors.Text,
			})
		}
	}

	if inTag {
		return spans, StateHTMLTag
	}
	return spans, StateHTMLNormal
}
