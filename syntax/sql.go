// Package syntax 提供纯 Go 实现的代码语法高亮与词法分词引擎
package syntax

import (
	"strings"
	"unicode"

	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/theme"
)

// SQL 跨行状态常量
const (
	StateSQLNormal       = 0
	StateSQLBlockComment = 1 // 处于 /* ... */ 跨行注释中
)

// SQLColors SQL 语法高亮调色板
type SQLColors struct {
	Keyword     graphics.Color // SELECT, FROM, WHERE, JOIN, GROUP BY, ORDER BY...
	Function    graphics.Color // COUNT, SUM, AVG, COALESCE, NOW, CONCAT...
	Type        graphics.Color // INT, VARCHAR, DATETIME, BOOLEAN, DECIMAL...
	Constant    graphics.Color // TRUE, FALSE, NULL
	String      graphics.Color // 'str', "str", `table`
	Number      graphics.Color // 123, 3.14, 0xFF
	Comment     graphics.Color // -- comment, # comment, /* comment */
	Operator    graphics.Color // =, <>, !=, <=, >=, AND, OR, LIKE, IN...
	Punctuation graphics.Color // (, ), ,, ;, .
}

// DefaultDarkSQLColors 暗色现代调色板
var DefaultDarkSQLColors = sqlColorsFromSyntax(theme.Dark().SyntaxColors())

// DefaultLightSQLColors 浅色主题调色板
var DefaultLightSQLColors = sqlColorsFromSyntax(theme.Light().SyntaxColors())

// sqlColorsFromSyntax 把共享语法令牌映射为 SQL 调色板。
func sqlColorsFromSyntax(s theme.SyntaxPalette) SQLColors {
	return SQLColors{
		Keyword:     s.Keyword,
		Function:    s.Function,
		Type:        s.Type,
		Constant:    s.Constant,
		String:      s.String,
		Number:      s.Number,
		Comment:     s.Comment,
		Operator:    s.Operator,
		Punctuation: s.Punctuation,
	}
}

var (
	sqlKeywords = map[string]bool{
		"select": true, "from": true, "where": true, "insert": true, "into": true,
		"update": true, "delete": true, "create": true, "alter": true, "drop": true,
		"table": true, "view": true, "index": true, "trigger": true, "procedure": true,
		"function": true, "database": true, "schema": true, "truncate": true, "use": true,
		"join": true, "inner": true, "left": true, "right": true, "full": true,
		"outer": true, "cross": true, "natural": true, "on": true, "using": true,
		"group": true, "by": true, "having": true, "order": true, "asc": true,
		"desc": true, "limit": true, "offset": true, "top": true, "distinct": true,
		"union": true, "all": true, "intersect": true, "except": true, "as": true,
		"and": true, "or": true, "not": true, "in": true, "between": true,
		"like": true, "ilike": true, "is": true, "exists": true, "case": true,
		"when": true, "then": true, "else": true, "end": true, "set": true,
		"values": true, "default": true, "primary": true, "key": true, "foreign": true,
		"references": true, "check": true, "unique": true, "constraint": true,
		"auto_increment": true, "cascade": true, "grant": true, "revoke": true,
		"begin": true, "commit": true, "rollback": true, "transaction": true,
		"with": true, "over": true, "partition": true,
	}

	sqlConstants = map[string]bool{
		"true": true, "false": true, "null": true,
	}

	sqlFunctions = map[string]bool{
		"count": true, "sum": true, "avg": true, "min": true, "max": true,
		"coalesce": true, "nullif": true, "concat": true, "substring": true, "length": true,
		"trim": true, "lower": true, "upper": true, "round": true, "now": true,
		"current_timestamp": true, "current_date": true, "current_time": true,
		"dateadd": true, "datediff": true, "cast": true, "convert": true,
		"row_number": true, "rank": true, "dense_rank": true, "lead": true, "lag": true,
		"first_value": true, "last_value": true, "nth_value": true,
	}

	sqlTypes = map[string]bool{
		"int": true, "integer": true, "bigint": true, "smallint": true, "tinyint": true,
		"decimal": true, "numeric": true, "float": true, "double": true, "real": true,
		"boolean": true, "bool": true, "char": true, "varchar": true, "nvarchar": true,
		"text": true, "clob": true, "blob": true, "binary": true, "varbinary": true,
		"date": true, "time": true, "datetime": true, "timestamp": true, "json": true,
		"jsonb": true, "uuid": true,
	}
)

// SQLHighlighter SQL 语法高亮器
type SQLHighlighter struct {
	isDark bool
	colors SQLColors
}

// NewSQLHighlighter 创建默认 SQL 语法高亮器
func NewSQLHighlighter() *SQLHighlighter {
	h := &SQLHighlighter{
		isDark: true,
		colors: DefaultDarkSQLColors,
	}
	h.ApplyTheme(theme.Current())
	return h
}

// GetColors 获取当前高亮颜色配置
func (h *SQLHighlighter) GetColors() SQLColors {
	return h.colors
}

// SetColors 自定义设置高亮颜色
func (h *SQLHighlighter) SetColors(c SQLColors) {
	h.colors = c
}

// ApplyTheme 根据当前主题自动适配调色板
func (h *SQLHighlighter) ApplyTheme(t *theme.Theme) {
	if t == nil {
		return
	}
	h.colors = sqlColorsFromSyntax(t.SyntaxColors())
	h.isDark = t.Dark
}

// HighlightLine 对单行 SQL 源码执行状态机词法分析并生成样式片段
func (h *SQLHighlighter) HighlightLine(lineIndex int, line []rune, prevLineState int) ([]Span, int) {
	n := len(line)
	if n == 0 {
		return nil, prevLineState
	}

	var spans []Span
	i := 0

	// 1. 处理上一行延续的跨行块注释 /* ... */
	if prevLineState == StateSQLBlockComment {
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
			return spans, StateSQLBlockComment
		}

		spans = append(spans, Span{
			StartCol: 0,
			EndCol:   closeIdx,
			Color:    h.colors.Comment,
			Italic:   true,
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

		// 2.2 单行注释: -- ... 或 MySQL # ...
		if (r == '-' && i+1 < n && line[i+1] == '-') || r == '#' {
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   n,
				Color:    h.colors.Comment,
				Italic:   true,
			})
			return spans, StateSQLNormal
		}

		// 2.3 块注释: /* ... */
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
				return spans, StateSQLBlockComment
			}
			continue
		}

		// 2.4 单引号字符串: '...' (标准 SQL 字符串，'' 转义)
		if r == '\'' {
			start := i
			i++
			for i < n {
				if line[i] == '\'' {
					if i+1 < n && line[i+1] == '\'' {
						i += 2 // '' 转义单引号
						continue
					}
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

		// 2.5 双引号字符串/标识符: "..."
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
				Color:    h.colors.String,
			})
			continue
		}

		// 2.6 反引号标识符: `table_name` (MySQL)
		if r == '`' {
			start := i
			i++
			for i < n && line[i] != '`' {
				i++
			}
			if i < n && line[i] == '`' {
				i++
			}
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.String,
			})
			continue
		}

		// 2.7 数值: 123, 3.14, 0xFF
		if unicode.IsDigit(r) || (r == '.' && i+1 < n && unicode.IsDigit(line[i+1])) {
			start := i
			isHex := false
			if r == '0' && i+1 < n && (line[i+1] == 'x' || line[i+1] == 'X') {
				isHex = true
				i += 2
			}
			for i < n {
				curr := line[i]
				if isHex {
					if unicode.IsDigit(curr) || (curr >= 'a' && curr <= 'f') || (curr >= 'A' && curr <= 'F') {
						i++
					} else {
						break
					}
				} else {
					if unicode.IsDigit(curr) || curr == '.' {
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
			spans = append(spans, Span{
				StartCol: start,
				EndCol:   i,
				Color:    h.colors.Number,
			})
			continue
		}

		// 2.8 标识符与关键字 (大小写不敏感匹配)
		if unicode.IsLetter(r) || r == '_' || r == '@' {
			start := i
			for i < n && (unicode.IsLetter(line[i]) || unicode.IsDigit(line[i]) || line[i] == '_') {
				i++
			}
			word := strings.ToLower(string(line[start:i]))

			if sqlKeywords[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Keyword,
					Bold:     true,
				})
			} else if sqlConstants[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Constant,
					Bold:     true,
				})
			} else if sqlFunctions[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Function,
				})
			} else if sqlTypes[word] {
				spans = append(spans, Span{
					StartCol: start,
					EndCol:   i,
					Color:    h.colors.Type,
				})
			}
			continue
		}

		// 2.9 双字符操作符
		if i+1 < n {
			two := string(line[i : i+2])
			if two == "<=" || two == ">=" || two == "<>" || two == "!=" ||
				two == "||" || two == "::" {
				spans = append(spans, Span{
					StartCol: i,
					EndCol:   i + 2,
					Color:    h.colors.Operator,
				})
				i += 2
				continue
			}
		}

		// 2.10 单字符操作符与标点符号
		switch r {
		case '=', '<', '>', '+', '-', '*', '/', '%':
			spans = append(spans, Span{
				StartCol: i,
				EndCol:   i + 1,
				Color:    h.colors.Operator,
			})
			i++
		case '(', ')', ',', ';', '.':
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

	return spans, StateSQLNormal
}
