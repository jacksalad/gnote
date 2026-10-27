package syntax

import (
	"strings"
	"unicode"

	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/theme"
)

// YAML 跨行状态常量
const (
	StateYAMLNormal          = 0
	StateYAMLDoubleQuote     = 10
	StateYAMLSingleQuote     = 11
	StateYAMLBlockScalarBase = 100 // 块标量基础偏移量，实际状态为 StateYAMLBlockScalarBase + indent
)

// YAMLColors 定义 YAML 调色板令牌
type YAMLColors struct {
	Key         graphics.Color // 键名
	String      graphics.Color // 字符串标量 / 块文本
	Number      graphics.Color // 数字
	Boolean     graphics.Color // 布尔值 (true, false, yes, no, on, off)
	Null        graphics.Color // null 与 ~
	Comment     graphics.Color // 注释 #
	Punctuation graphics.Color // 分隔符 : , - ?
	Bracket     graphics.Color // 结构括号 [ ] { }
	Anchor      graphics.Color // 锚点与别名 &anchor *alias
	Tag         graphics.Color // 类型标签 !tag !!type
	Directive   graphics.Color // 文档分隔 --- ... 与 %YAML
	Escape      graphics.Color // 转义字符
}

// DefaultDarkYAMLColors 暗色主题预设调色板
var DefaultDarkYAMLColors = yamlColorsFromSyntax(theme.Dark().SyntaxColors())

// DefaultLightYAMLColors 浅色主题预设调色板
var DefaultLightYAMLColors = yamlColorsFromSyntax(theme.Light().SyntaxColors())

// yamlColorsFromSyntax 把共享语法令牌映射为 YAML 调色板。
func yamlColorsFromSyntax(s theme.SyntaxPalette) YAMLColors {
	return YAMLColors{
		Key:         s.Key,
		String:      s.StringStrong,
		Number:      s.NumberSoft,
		Boolean:     s.Boolean,
		Null:        s.Null,
		Comment:     s.CommentSoft,
		Punctuation: s.PunctuationAlt,
		Bracket:     s.Bracket,
		Anchor:      s.Anchor,
		Tag:         s.Table,
		Directive:   s.Directive,
		Escape:      s.Escape,
	}
}

// YAMLHighlighter 纯 Go YAML 词法语法高亮器
type YAMLHighlighter struct {
	colors YAMLColors
	isDark bool
}

// NewYAMLHighlighter 创建 YAML 语法高亮器
func NewYAMLHighlighter() *YAMLHighlighter {
	h := &YAMLHighlighter{
		isDark: true,
		colors: DefaultDarkYAMLColors,
	}
	h.ApplyTheme(theme.Current())
	return h
}

// ApplyTheme 跟随编辑器全局主题动态切换高亮色彩
func (h *YAMLHighlighter) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	h.colors = yamlColorsFromSyntax(t.SyntaxColors())
	h.isDark = t.Dark
}

// SetColors 自定义设置调色板
func (h *YAMLHighlighter) SetColors(c YAMLColors) {
	h.colors = c
}

// HighlightLine 对单行 YAML 文本进行词法扫描分词
func (h *YAMLHighlighter) HighlightLine(lineIndex int, line []rune, prevLineState int) ([]Span, int) {
	n := len(line)
	if n == 0 {
		// 空行在块标量中保持其状态
		if prevLineState >= StateYAMLBlockScalarBase {
			return nil, prevLineState
		}
		return nil, StateYAMLNormal
	}

	// 1. 处理块标量多行状态 (| 或 >)
	if prevLineState >= StateYAMLBlockScalarBase {
		baseIndent := prevLineState - StateYAMLBlockScalarBase
		// 计算当前行缩进
		indent := 0
		for indent < n && line[indent] == ' ' {
			indent++
		}
		// 若当前行为纯空行，保持块标量状态
		if indent == n {
			return nil, prevLineState
		}
		// 若当前行缩进大于 baseIndent，则整行作为块标量字符串
		if indent >= baseIndent {
			return []Span{
				{
					StartCol: indent,
					EndCol:   n,
					Color:    h.colors.String,
				},
			}, prevLineState
		}
		// 缩进缩回，块标量结束，回退到普通解析
		prevLineState = StateYAMLNormal
	}

	var spans []Span
	i := 0

	// 2. 处理跨行双引号字符串
	if prevLineState == StateYAMLDoubleQuote {
		start := 0
		closed := false
		for i < n {
			if line[i] == '\\' && i+1 < n {
				i += 2
				continue
			}
			if line[i] == '"' {
				i++
				closed = true
				break
			}
			i++
		}
		spans = append(spans, Span{
			StartCol: start,
			EndCol:   i,
			Color:    h.colors.String,
		})
		if !closed {
			return spans, StateYAMLDoubleQuote
		}
		prevLineState = StateYAMLNormal
	}

	// 3. 处理跨行单引号字符串
	if prevLineState == StateYAMLSingleQuote {
		start := 0
		closed := false
		for i < n {
			if line[i] == '\'' {
				if i+1 < n && line[i+1] == '\'' {
					i += 2 // YAML 单引号转义 ''
					continue
				}
				i++
				closed = true
				break
			}
			i++
		}
		spans = append(spans, Span{
			StartCol: start,
			EndCol:   i,
			Color:    h.colors.String,
		})
		if !closed {
			return spans, StateYAMLSingleQuote
		}
		prevLineState = StateYAMLNormal
	}

	// 计算当前行起始缩进
	lineIndent := 0
	for lineIndent < n && line[lineIndent] == ' ' {
		lineIndent++
	}

	// 4. 文档标记与指令 (---, ..., %YAML)
	trimmedLeading := strings.TrimSpace(string(line))
	if strings.HasPrefix(trimmedLeading, "---") || strings.HasPrefix(trimmedLeading, "...") {
		spans = append(spans, Span{
			StartCol: lineIndent,
			EndCol:   lineIndent + 3,
			Color:    h.colors.Directive,
			Bold:     true,
		})
		i = lineIndent + 3
		// 检查行后是否有注释
		for i < n && unicode.IsSpace(line[i]) {
			i++
		}
		if i < n && line[i] == '#' {
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   n,
				Color:    h.colors.Comment,
				Italic:   true,
			})
		}
		return spans, StateYAMLNormal
	}
	if strings.HasPrefix(trimmedLeading, "%") {
		spans = append(spans, Span{
			StartCol: lineIndent,
			EndCol:   n,
			Color:    h.colors.Directive,
			Bold:     true,
		})
		return spans, StateYAMLNormal
	}

	// 5. 扫描各词法单元
	blockScalarIndent := -1

	for i < n {
		r := line[i]

		// 空白跳过
		if unicode.IsSpace(r) {
			i++
			continue
		}

		// 注释: #
		if r == '#' {
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   n,
				Color:    h.colors.Comment,
				Italic:   true,
			})
			break
		}

		// 列表条目符: - (必须后跟空格或处于行尾)
		if r == '-' && (i+1 == n || unicode.IsSpace(line[i+1])) {
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   i + 1,
				Color:    h.colors.Punctuation,
				Bold:     true,
			})
			i++
			continue
		}

		// 显式键标识符: ? (必须后跟空格)
		if r == '?' && (i+1 == n || unicode.IsSpace(line[i+1])) {
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   i + 1,
				Color:    h.colors.Punctuation,
				Bold:     true,
			})
			i++
			continue
		}

		// 流式括号与分隔符: [ ] { } ,
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
		if r == ',' {
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   i + 1,
				Color:    h.colors.Punctuation,
			})
			i++
			continue
		}

		// 锚点 &anchor 与 别名 *alias
		if r == '&' || r == '*' {
			start := i
			i++
			for i < n && (unicode.IsLetter(line[i]) || unicode.IsDigit(line[i]) || line[i] == '_' || line[i] == '-') {
				i++
			}
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Anchor,
				Bold:     true,
			})
			continue
		}

		// 类型标签 !tag 或 !!type
		if r == '!' {
			start := i
			i++
			if i < n && line[i] == '!' {
				i++
			}
			for i < n && !unicode.IsSpace(line[i]) && line[i] != ',' && line[i] != ']' && line[i] != '}' {
				i++
			}
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Tag,
				Italic:   true,
			})
			continue
		}

		// 块标量引导符: | 或 > (如 |, |-, |+, >, >-, >+)
		if (r == '|' || r == '>') && (i+1 == n || unicode.IsSpace(line[i+1]) || line[i+1] == '+' || line[i+1] == '-' || unicode.IsDigit(line[i+1])) {
			start := i
			i++
			for i < n && (line[i] == '+' || line[i] == '-' || unicode.IsDigit(line[i])) {
				i++
			}
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Punctuation,
				Bold:     true,
			})
			// 记录块标量预期的缩进级别 (至少比当前行多 2 个空格)
			blockScalarIndent = lineIndent + 2
			continue
		}

		// 双引号字符串
		if r == '"' {
			start := i
			i++
			closed := false
			for i < n {
				if line[i] == '\\' && i+1 < n {
					i += 2
					continue
				}
				if line[i] == '"' {
					i++
					closed = true
					break
				}
				i++
			}
			// 检查双引号后是否紧跟冒号 (带引号的键名: "key": value)
			isKey := false
			peek := i
			for peek < n && unicode.IsSpace(line[peek]) {
				peek++
			}
			if peek < n && line[peek] == ':' && (peek+1 == n || unicode.IsSpace(line[peek+1]) || line[peek+1] == ',' || line[peek+1] == '}' || line[peek+1] == ']') {
				isKey = true
			}

			if isKey {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Key,
					Bold:     true,
				})
			} else {
				h.appendStringWithEscapes(&spans, line, start, i)
			}

			if !closed {
				return spans, StateYAMLDoubleQuote
			}
			continue
		}

		// 单引号字符串
		if r == '\'' {
			start := i
			i++
			closed := false
			for i < n {
				if line[i] == '\'' {
					if i+1 < n && line[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					closed = true
					break
				}
				i++
			}

			isKey := false
			peek := i
			for peek < n && unicode.IsSpace(line[peek]) {
				peek++
			}
			if peek < n && line[peek] == ':' && (peek+1 == n || unicode.IsSpace(line[peek+1]) || line[peek+1] == ',' || line[peek+1] == '}' || line[peek+1] == ']') {
				isKey = true
			}

			if isKey {
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

			if !closed {
				return spans, StateYAMLSingleQuote
			}
			continue
		}

		// 检查当前位置是否为普通键名或值
		// 探测向前直到下一个关键符号 (:, #, ,, ], })
		start := i
		// 寻找当前词边界
		hasColonMapping := false
		colonPos := -1

		// 向前窥探这一行是否存在作为映射键的冒号 (后跟空白或行尾)
		for scan := i; scan < n; scan++ {
			if line[scan] == '#' || line[scan] == ',' || line[scan] == ']' || line[scan] == '}' {
				break
			}
			if line[scan] == ':' && (scan+1 == n || unicode.IsSpace(line[scan+1]) || line[scan+1] == ',' || line[scan+1] == '}' || line[scan+1] == ']') {
				hasColonMapping = true
				colonPos = scan
				break
			}
		}

		if hasColonMapping && colonPos >= start {
			// 当前片段是键名 (Key)
			keyEnd := colonPos
			// 修剪尾部空格
			for keyEnd > start && unicode.IsSpace(line[keyEnd-1]) {
				keyEnd--
			}
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   keyEnd,
				Color:    h.colors.Key,
				Bold:     true,
			})
			// 冒号作为分隔符
			spans = append(spans, Span{
				StartCol: colonPos,
				EndCol:   colonPos + 1,
				Color:    h.colors.Punctuation,
			})
			i = colonPos + 1
			continue
		}

		// 否则是值 (Value)：可能是数字、布尔、null、或者普通标量字符串
		for i < n && !unicode.IsSpace(line[i]) && line[i] != ',' && line[i] != ']' && line[i] != '}' && line[i] != '#' {
			i++
		}
		word := string(line[start:i])

		// 尝试匹配布尔值
		if isYAMLBoolean(word) {
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Boolean,
				Bold:     true,
			})
			continue
		}

		// 尝试匹配 null
		if isYAMLNull(word) {
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Null,
				Italic:   true,
			})
			continue
		}

		// 尝试匹配数字
		if isYAMLNumber(word) {
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Number,
			})
			continue
		}

		// 普通纯文本标量 (如网址、文件名、纯文本)
		// 向前吃掉到注释或流式分隔符前的内容
		end := i
		for end < n && line[end] != '#' && line[end] != ',' && line[end] != ']' && line[end] != '}' {
			end++
		}
		// 修剪末尾空格
		trimEnd := end
		for trimEnd > start && unicode.IsSpace(line[trimEnd-1]) {
			trimEnd--
		}
		spans = append(spans, Span{
			StartCol: start,
			EndCol:   trimEnd,
			Color:    h.colors.String,
		})
		i = end
	}

	if blockScalarIndent > 0 {
		return spans, StateYAMLBlockScalarBase + blockScalarIndent
	}
	return spans, StateYAMLNormal
}

// appendStringWithEscapes 分词双引号字符串中的转义符
func (h *YAMLHighlighter) appendStringWithEscapes(spans *[]Span, line []rune, start, end int) {
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
			if line[idx+1] == 'x' && idx+3 < end {
				escLen = 4
			} else if line[idx+1] == 'u' && idx+5 < end {
				escLen = 6
			} else if line[idx+1] == 'U' && idx+9 < end {
				escLen = 10
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

// isYAMLBoolean 检查词法是否为 YAML 布尔值
func isYAMLBoolean(w string) bool {
	switch w {
	case "true", "True", "TRUE", "false", "False", "FALSE",
		"yes", "Yes", "YES", "no", "No", "NO",
		"on", "On", "ON", "off", "Off", "OFF":
		return true
	default:
		return false
	}
}

// isYAMLNull 检查词法是否为 YAML null
func isYAMLNull(w string) bool {
	switch w {
	case "null", "Null", "NULL", "~":
		return true
	default:
		return false
	}
}

// isYAMLNumber 检查词法是否为数字标量
func isYAMLNumber(w string) bool {
	if len(w) == 0 {
		return false
	}
	if w == ".nan" || w == ".NaN" || w == ".NAN" ||
		w == ".inf" || w == ".Inf" || w == ".INF" ||
		w == "+.inf" || w == "+.Inf" || w == "+.INF" ||
		w == "-.inf" || w == "-.Inf" || w == "-.INF" {
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
			// 允许浮点与科学记数法
		} else {
			return false
		}
		idx++
	}
	return hasDigit
}
