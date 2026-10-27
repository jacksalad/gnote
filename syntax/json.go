package syntax

import (
	"unicode"

	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/theme"
)

// JSONColors 定义 JSON 调色板令牌
type JSONColors struct {
	Key         graphics.Color
	String      graphics.Color
	Number      graphics.Color
	Boolean     graphics.Color
	Null        graphics.Color
	Punctuation graphics.Color
	Bracket     graphics.Color
	Comment     graphics.Color
	Escape      graphics.Color
}

// DefaultDarkJSONColors 暗色主题预设调色板
var DefaultDarkJSONColors = jsonColorsFromSyntax(theme.Dark().SyntaxColors())

// DefaultLightJSONColors 浅色主题预设调色板
var DefaultLightJSONColors = jsonColorsFromSyntax(theme.Light().SyntaxColors())

// jsonColorsFromSyntax 把共享语法令牌映射为 JSON 调色板。
func jsonColorsFromSyntax(s theme.SyntaxPalette) JSONColors {
	return JSONColors{
		Key:         s.Key,
		String:      s.StringStrong,
		Number:      s.NumberSoft,
		Boolean:     s.Boolean,
		Null:        s.Null,
		Punctuation: s.PunctuationAlt,
		Bracket:     s.Bracket,
		Comment:     s.CommentSoft,
		Escape:      s.Escape,
	}
}

// JSONHighlighter 纯 Go JSON 词法语法高亮器
type JSONHighlighter struct {
	colors JSONColors
	isDark bool
}

// NewJSONHighlighter 创建 JSON 语法高亮器
func NewJSONHighlighter() *JSONHighlighter {
	h := &JSONHighlighter{
		isDark: true,
		colors: DefaultDarkJSONColors,
	}
	h.ApplyTheme(theme.Current())
	return h
}

// ApplyTheme 跟随编辑器全局主题动态切换高亮色彩
func (h *JSONHighlighter) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	h.colors = jsonColorsFromSyntax(t.SyntaxColors())
	h.isDark = t.Dark
}

// SetColors 自定义设置调色板
func (h *JSONHighlighter) SetColors(c JSONColors) {
	h.colors = c
}

// HighlightLine 对单行 JSON 文本进行词法扫描分词
func (h *JSONHighlighter) HighlightLine(lineIndex int, line []rune, prevLineState int) ([]Span, int) {
	n := len(line)
	if n == 0 {
		return nil, StateNormal
	}

	var spans []Span
	i := 0

	for i < n {
		r := line[i]

		// 1. 跳过空白字符
		if unicode.IsSpace(r) {
			i++
			continue
		}

		// 2. 单行注释 //
		if r == '/' && i+1 < n && line[i+1] == '/' {
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   n,
				Color:    h.colors.Comment,
				Italic:   true,
			})
			break
		}

		// 3. 结构括号: { } [ ]
		if r == '{' || r == '}' || r == '[' || r == ']' {
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   i + 1,
				Color:    h.colors.Bracket,
				Bold:     true,
			})
			i++
			continue
		}

		// 4. 分隔符: : ,
		if r == ':' || r == ',' {
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   i + 1,
				Color:    h.colors.Punctuation,
			})
			i++
			continue
		}

		// 5. 字符串处理 (包含键名与字符串值的智能区分)
		if r == '"' {
			start := i
			i++
			hasEscapes := false
			for i < n {
				if line[i] == '\\' && i+1 < n {
					hasEscapes = true
					i += 2
					continue
				}
				if line[i] == '"' {
					i++ // 包含闭合双引号
					break
				}
				i++
			}
			strEnd := i

			// 判断是否为对象键名 (Key)：检查闭合引号后下一个非空白字符是否为 ':'
			isKey := false
			peek := strEnd
			for peek < n && unicode.IsSpace(line[peek]) {
				peek++
			}
			if peek < n && line[peek] == ':' {
				isKey = true
			}

			if isKey {
				// 键名高亮
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   strEnd,
					Color:    h.colors.Key,
					Bold:     true,
				})
			} else {
				// 字符串值高亮 (若有转义字符，可高亮区分)
				if !hasEscapes {
					spans = append(spans, Span{
						StartCol: start,
						EndCol:   strEnd,
						Color:    h.colors.String,
					})
				} else {
					// 带有转义字符的字符串细致分词
					segStart := start
					for idx := start; idx < strEnd; idx++ {
						if line[idx] == '\\' && idx+1 < strEnd {
							if idx > segStart {
								spans = append(spans, Span{
									StartCol: segStart,
									EndCol:   idx,
									Color:    h.colors.String,
								})
							}
							escLen := 2
							if line[idx+1] == 'u' && idx+5 < strEnd {
								escLen = 6 // \uXXXX
							}
							spans = append(spans, Span{
								StartCol: idx,
								EndCol:   idx + escLen,
								Color:    h.colors.Escape,
								Bold:     true,
							})
							idx += escLen - 1
							segStart = idx + 1
						}
					}
					if segStart < strEnd {
						spans = append(spans, Span{
							StartCol: segStart,
							EndCol:   strEnd,
							Color:    h.colors.String,
						})
					}
				}
			}
			continue
		}

		// 6. 数字处理 (-0.5, 123, 3.14e+10 等)
		if r == '-' || r == '+' || unicode.IsDigit(r) {
			start := i
			i++
			for i < n && (unicode.IsDigit(line[i]) || line[i] == '.' || line[i] == 'e' || line[i] == 'E' || line[i] == '+' || line[i] == '-') {
				i++
			}
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Number,
			})
			continue
		}

		// 7. 关键字: true, false, null
		if unicode.IsLetter(r) {
			start := i
			for i < n && unicode.IsLetter(line[i]) {
				i++
			}
			word := string(line[start:i])
			switch word {
			case "true", "false":
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Boolean,
					Bold:     true,
				})
			case "null":
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Null,
					Italic:   true,
				})
			default:
				// 其他未定义标识符
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Punctuation,
				})
			}
			continue
		}

		i++
	}

	return spans, StateNormal
}
