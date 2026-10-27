package syntax

import (
	"github.com/jacksalad/gnote/graphics"
	"github.com/jacksalad/gnote/theme"
)

// Span 表示一行文本内的一个语法高亮样式片段
type Span struct {
	StartCol int            // 起始字符列索引 (基于 rune, 0-indexed, inclusive)
	EndCol   int            // 结束字符列索引 (基于 rune, 0-indexed, exclusive)
	Color    graphics.Color // 前景色
	BgColor  graphics.Color // 背景底色 (0 表示透明无背景)
	Bold     bool           // 是否加粗显示
	Italic   bool           // 是否斜体显示
}

// Highlighter 语法高亮器抽象接口
type Highlighter interface {
	// HighlightLine 对指定单行文本进行词法扫描与高亮分词
	// lineIndex: 行号 (0-indexed)
	// line: 该行的 Unicode rune 切片
	// prevLineState: 前一行的结束状态 (用于跨行代码块等状态传递)
	// 返回: 该行的样式片段切片 spans，以及该行的结束状态 endState
	HighlightLine(lineIndex int, line []rune, prevLineState int) (spans []Span, endState int)

	// ApplyTheme 当 UI 主题 (Dark/Light) 发生变化时调用以刷新调色板令牌
	ApplyTheme(t *theme.Theme)
}
