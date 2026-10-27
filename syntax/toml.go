package syntax

import (
	"unicode"

	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/theme"
)

// TOML 跨行状态常量
const (
	StateTOMLNormal          = 0
	StateTOMLMLBasicString   = 20 // 多行基本字符串 """ ... """
	StateTOMLMLLiteralString = 21 // 多行字面量字符串 ''' ... '''
)

// TOMLColors 定义 TOML 调色板令牌
type TOMLColors struct {
	Table       graphics.Color // 表头 [table] 与 [[array]]
	Key         graphics.Color // 键名
	String      graphics.Color // 字符串
	Number      graphics.Color // 数字
	Boolean     graphics.Color // 布尔值 (true, false)
	DateTime    graphics.Color // 日期与时间
	Comment     graphics.Color // 注释 #
	Punctuation graphics.Color // 分隔符 = , .
	Bracket     graphics.Color // 括号 [ ] { }
	Escape      graphics.Color // 转义字符
}

// DefaultDarkTOMLColors 暗色主题预设调色板
var DefaultDarkTOMLColors = tomlColorsFromSyntax(theme.Dark().SyntaxColors())

// DefaultLightTOMLColors 浅色主题预设调色板
var DefaultLightTOMLColors = tomlColorsFromSyntax(theme.Light().SyntaxColors())

// tomlColorsFromSyntax 把共享语法令牌映射为 TOML 调色板。
func tomlColorsFromSyntax(s theme.SyntaxPalette) TOMLColors {
	return TOMLColors{
		Table:       s.Table,
		Key:         s.Key,
		String:      s.StringStrong,
		Number:      s.NumberSoft,
		Boolean:     s.Boolean,
		DateTime:    s.DateTime,
		Comment:     s.CommentSoft,
		Punctuation: s.PunctuationAlt,
		Bracket:     s.Bracket,
		Escape:      s.Escape,
	}
}

// TOMLHighlighter 纯 Go TOML 词法语法高亮器
type TOMLHighlighter struct {
	colors TOMLColors
	isDark bool
}

// NewTOMLHighlighter 创建 TOML 语法高亮器
func NewTOMLHighlighter() *TOMLHighlighter {
	h := &TOMLHighlighter{
		isDark: true,
		colors: DefaultDarkTOMLColors,
	}
	h.ApplyTheme(theme.Current())
	return h
}

// ApplyTheme 跟随编辑器全局主题动态切换高亮色彩
func (h *TOMLHighlighter) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	h.colors = tomlColorsFromSyntax(t.SyntaxColors())
	h.isDark = t.Dark
}

// SetColors 自定义设置调色板
func (h *TOMLHighlighter) SetColors(c TOMLColors) {
	h.colors = c
}

// HighlightLine 对单行 TOML 文本进行词法扫描分词
func (h *TOMLHighlighter) HighlightLine(lineIndex int, line []rune, prevLineState int) ([]Span, int) {
	n := len(line)
	if n == 0 {
		if prevLineState == StateTOMLMLBasicString || prevLineState == StateTOMLMLLiteralString {
			return nil, prevLineState
		}
		return nil, StateTOMLNormal
	}

	var spans []Span
	i := 0

	// 1. 处理跨行多行基本字符串 """
	if prevLineState == StateTOMLMLBasicString {
		// 寻找闭合的 """
		closeIdx := -1
		for idx := 0; idx+2 < n; idx++ {
			if line[idx] == '\\' {
				idx++
				continue
			}
			if line[idx] == '"' && line[idx+1] == '"' && line[idx+2] == '"' {
				closeIdx = idx
				break
			}
		}

		if closeIdx == -1 {
			// 未闭合，整行继续为多行字符串
			h.appendStringWithEscapes(&spans, line, 0, n)
			return spans, StateTOMLMLBasicString
		}

		// 找到闭合 """
		h.appendStringWithEscapes(&spans, line, 0, closeIdx+3)
		i = closeIdx + 3
		prevLineState = StateTOMLNormal
	}

	// 2. 处理跨行多行字面量字符串 '''
	if prevLineState == StateTOMLMLLiteralString {
		closeIdx := -1
		for idx := 0; idx+2 < n; idx++ {
			if line[idx] == '\'' && line[idx+1] == '\'' && line[idx+2] == '\'' {
				closeIdx = idx
				break
			}
		}

		if closeIdx == -1 {
			spans = append(spans, Span{
				StartCol: 0,
				EndCol:   n,
				Color:    h.colors.String,
			})
			return spans, StateTOMLMLLiteralString
		}

		spans = append(spans, Span{
			StartCol: 0,
			EndCol:   closeIdx + 3,
			Color:    h.colors.String,
		})
		i = closeIdx + 3
		prevLineState = StateTOMLNormal
	}

	// 3. 检查当前行是否为表头定义: [table] 或 [[array_of_tables]]
	if i == 0 {
		leading := 0
		for leading < n && unicode.IsSpace(line[leading]) {
			leading++
		}
		if leading < n && line[leading] == '[' {
			isTable, isArrayTable, closeBracketStart, closeBracketEnd, commentStart := checkTOMLTableHeader(line, leading)
			if isTable {
				bracketLen := 1
				if isArrayTable {
					bracketLen = 2
				}

				// 开括号 [ 或 [[
				spans = append(spans, Span{
					StartCol: leading,
					EndCol:   leading + bracketLen,
					Color:    h.colors.Bracket,
					Bold:     true,
				})

				// 表名称 a.b.c
				nameStart := leading + bracketLen
				nameEnd := closeBracketStart
				// 修剪内侧空格
				for nameStart < nameEnd && unicode.IsSpace(line[nameStart]) {
					nameStart++
				}
				for nameEnd > nameStart && unicode.IsSpace(line[nameEnd-1]) {
					nameEnd--
				}
				if nameEnd > nameStart {
					spans = append(spans, Span{
						StartCol: nameStart,
						EndCol:   nameEnd,
						Color:    h.colors.Table,
						Bold:     true,
					})
				}

				// 闭括号 ] 或 ]]
				spans = append(spans, Span{
					StartCol: closeBracketStart,
					EndCol:   closeBracketEnd,
					Color:    h.colors.Bracket,
					Bold:     true,
				})

				// 注释 (若有)
				if commentStart >= 0 && commentStart < n {
					spans = append(spans, Span{
						StartCol: commentStart,
						EndCol:   n,
						Color:    h.colors.Comment,
						Italic:   true,
					})
				}

				return spans, StateTOMLNormal
			}
		}
	}

	// 4. 普通词法扫描 (Key-Value, 标量, 数组, 行内表等)
	for i < n {
		r := line[i]

		// 空白跳过
		if unicode.IsSpace(r) {
			i++
			continue
		}

		// 注释 #
		if r == '#' {
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   n,
				Color:    h.colors.Comment,
				Italic:   true,
			})
			break
		}

		// 等号 =
		if r == '=' {
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   i + 1,
				Color:    h.colors.Punctuation,
				Bold:     true,
			})
			i++
			continue
		}

		// 分隔符与括号
		if r == ',' || r == '.' {
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   i + 1,
				Color:    h.colors.Punctuation,
			})
			i++
			continue
		}
		if r == '[' || r == ']' || r == '{' || r == '}' {
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   i + 1,
				Color:    h.colors.Bracket,
				Bold:     true,
			})
			i++
			continue
		}

		// 多行基本字符串 """ 或 单行基本字符串 "
		if r == '"' {
			if i+2 < n && line[i+1] == '"' && line[i+2] == '"' {
				// 三双引号 """
				start := i
				i += 3
				closed := false
				for i+2 < n {
					if line[i] == '\\' {
						i += 2
						continue
					}
					if line[i] == '"' && line[i+1] == '"' && line[i+2] == '"' {
						i += 3
						closed = true
						break
					}
					i++
				}
				if !closed {
					h.appendStringWithEscapes(&spans, line, start, n)
					return spans, StateTOMLMLBasicString
				}
				h.appendStringWithEscapes(&spans, line, start, i)
				continue
			}

			// 普通单行双引号字符串 "..."
			start := i
			i++
			for i < n {
				if line[i] == '\\' && i+1 < n {
					i += 2
					continue
				}
				if line[i] == '"' {
					i++
					break
				}
				i++
			}

			// 判断是否为键名 (后跟 =)
			if h.isFollowedByEqual(line, i) {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Key,
					Bold:     true,
				})
			} else {
				h.appendStringWithEscapes(&spans, line, start, i)
			}
			continue
		}

		// 多行字面量字符串 ''' 或 单行字面量字符串 '
		if r == '\'' {
			if i+2 < n && line[i+1] == '\'' && line[i+2] == '\'' {
				// 三单引号 '''
				start := i
				i += 3
				closed := false
				for i+2 < n {
					if line[i] == '\'' && line[i+1] == '\'' && line[i+2] == '\'' {
						i += 3
						closed = true
						break
					}
					i++
				}
				if !closed {
					spans = append(spans, Span{
						StartCol: start,
						EndCol:   n,
						Color:    h.colors.String,
					})
					return spans, StateTOMLMLLiteralString
				}
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.String,
				})
				continue
			}

			// 普通字面量字符串 '...'
			start := i
			i++
			for i < n && line[i] != '\'' {
				i++
			}
			if i < n && line[i] == '\'' {
				i++
			}

			if h.isFollowedByEqual(line, i) {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Key,
					Bold:     true,
				})
			} else {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.String,
				})
			}
			continue
		}

		// 检查当前词段
		start := i
		for i < n && !unicode.IsSpace(line[i]) && line[i] != '=' && line[i] != ',' &&
			line[i] != '[' && line[i] != ']' && line[i] != '{' && line[i] != '}' && line[i] != '#' && line[i] != '"' && line[i] != '\'' {
			i++
		}
		word := string(line[start:i])

		// 检查是否为等号左侧的裸键 (Bare Key)
		if h.isFollowedByEqual(line, i) {
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Key,
				Bold:     true,
			})
			continue
		}

		// 布尔值 true / false
		if word == "true" || word == "false" {
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Boolean,
				Bold:     true,
			})
			continue
		}

		// 日期时间戳 (如 1979-05-27, 07:32:00, 1979-05-27T07:32:00Z)
		if isTOMLDateTime(word) {
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.DateTime,
			})
			continue
		}

		// 数字
		if isTOMLNumber(word) {
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Number,
			})
			continue
		}

		// 默认标量
		spans = append(spans, Span{
			StartCol: start,
			EndCol:   i,
			Color:    h.colors.Punctuation,
		})
	}

	return spans, StateTOMLNormal
}

// isFollowedByEqual 检查在当前位置之后，是否紧跟着赋值符号 '=' (判断键名)
func (h *TOMLHighlighter) isFollowedByEqual(line []rune, idx int) bool {
	n := len(line)
	for idx < n && unicode.IsSpace(line[idx]) {
		idx++
	}
	return idx < n && line[idx] == '='
}

// appendStringWithEscapes 分词双引号基本字符串中的转义字符
func (h *TOMLHighlighter) appendStringWithEscapes(spans *[]Span, line []rune, start, end int) {
	segStart := start
	for idx := start; idx < end; idx++ {
		if line[idx] == '\\' && idx+1 < end {
			if idx > segStart {
				*spans = append(*spans, Span{
					StartCol: segStart,
					EndCol:   idx,
					Color:    h.colors.String,
				})
			}
			escLen := 2
			if line[idx+1] == 'u' && idx+5 < end {
				escLen = 6 // \uXXXX
			} else if line[idx+1] == 'U' && idx+9 < end {
				escLen = 10 // \UXXXXXXXX
			}
			*spans = append(*spans, Span{
				StartCol: idx,
				EndCol:   idx + escLen,
				Color:    h.colors.Escape,
				Bold:     true,
			})
			idx += escLen - 1
			segStart = idx + 1
		}
	}
	if segStart < end {
		*spans = append(*spans, Span{
			StartCol: segStart,
			EndCol:   end,
			Color:    h.colors.String,
		})
	}
}

// checkTOMLTableHeader 检验当前行是否构成合法的 TOML 表头
// 返回: (isTable, isArrayTable, closeBracketStart, closeBracketEnd, commentStart)
func checkTOMLTableHeader(line []rune, openBracketStart int) (bool, bool, int, int, int) {
	n := len(line)
	isArrayTable := (openBracketStart+1 < n && line[openBracketStart+1] == '[')

	// 寻找配对闭括号
	closeStart := -1
	closeEnd := -1

	if isArrayTable {
		for idx := openBracketStart + 2; idx+1 < n; idx++ {
			if line[idx] == ']' && line[idx+1] == ']' {
				closeStart = idx
				closeEnd = idx + 2
				break
			}
		}
	} else {
		for idx := openBracketStart + 1; idx < n; idx++ {
			if line[idx] == ']' {
				closeStart = idx
				closeEnd = idx + 1
				break
			}
		}
	}

	if closeStart == -1 {
		return false, false, -1, -1, -1
	}

	// 验证闭括号之后只能有空白和可选注释
	commentStart := -1
	for idx := closeEnd; idx < n; idx++ {
		r := line[idx]
		if unicode.IsSpace(r) {
			continue
		}
		if r == '#' {
			commentStart = idx
			break
		}
		// 存在其他非空白非注释字符，则不是标准表头
		return false, false, -1, -1, -1
	}

	return true, isArrayTable, closeStart, closeEnd, commentStart
}

// isTOMLNumber 判定词法是否为 TOML 数字
func isTOMLNumber(w string) bool {
	if len(w) == 0 {
		return false
	}
	// 特殊浮点
	if w == "inf" || w == "+inf" || w == "-inf" ||
		w == "nan" || w == "+nan" || w == "-nan" {
		return true
	}

	idx := 0
	if w[idx] == '+' || w[idx] == '-' {
		idx++
		if idx >= len(w) {
			return false
		}
	}

	// 16 进制 0x, 8 进制 0o, 2 进制 0b
	if idx+2 <= len(w) && w[idx] == '0' {
		prefix := w[idx+1]
		if prefix == 'x' || prefix == 'X' || prefix == 'o' || prefix == 'O' || prefix == 'b' || prefix == 'B' {
			return true
		}
	}

	hasDigit := false
	for idx < len(w) {
		ch := w[idx]
		if ch >= '0' && ch <= '9' {
			hasDigit = true
		} else if ch == '.' || ch == 'e' || ch == 'E' || ch == '+' || ch == '-' || ch == '_' {
			// 允许下划线及指数
		} else {
			return false
		}
		idx++
	}
	return hasDigit
}

// isTOMLDateTime 判定词法是否为 TOML 日期时间
func isTOMLDateTime(w string) bool {
	if len(w) < 8 {
		return false
	}
	// 常见格式:
	// 1979-05-27
	// 07:32:00
	// 1979-05-27T07:32:00Z
	// 1979-05-27T00:32:00-07:00
	if len(w) >= 10 && w[4] == '-' && w[7] == '-' {
		return true
	}
	if len(w) >= 8 && w[2] == ':' && w[5] == ':' {
		return true
	}
	return false
}
