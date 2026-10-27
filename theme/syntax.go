package theme

import "github.com/jacksalad/gnote/graphics"

// SyntaxPalette 定义跨语言的语法高亮语义令牌。
//
// 16 个高亮器不再各自维护 Dark/Light 两套硬编码调色板，而是把自身的
// 语法元素映射到这里的语义角色。这样新增一套主题只需定义 UI 令牌与
// 这一份语法令牌，所有语言的高亮风格天然保持一致。
//
// 除核心角色外的字段可以留零值，SyntaxColors() 会按语义回退补齐。
type SyntaxPalette struct {
	Keyword        graphics.Color // if/for/func/class 等关键字
	Type           graphics.Color // int/string/Task 等类型与标签
	Builtin        graphics.Color // len/append/console 等内置对象
	Function       graphics.Color // 函数定义与调用
	Key            graphics.Color // JSON/YAML/TOML 键名
	String         graphics.Color // 代码字符串
	StringStrong   graphics.Color // 配置文件字符串（对比度更高的变体）
	Character      graphics.Color // 字符字面量
	Number         graphics.Color // 数字字面量
	NumberSoft     graphics.Color // 配置文件数字 / 日期等柔和变体
	Constant       graphics.Color // true/false/nil/常量
	Operator       graphics.Color // 运算符与普通正文
	Punctuation    graphics.Color // 代码标点
	PunctuationAlt graphics.Color // 配置文件标点（更柔和的变体）
	Bracket        graphics.Color // 结构括号
	Comment        graphics.Color // 代码注释
	CommentSoft    graphics.Color // 配置文件注释（更柔和的变体）
	Attribute      graphics.Color // HTML 属性 / CSS 属性
	Boolean        graphics.Color // 布尔字面量
	Null           graphics.Color // null / nil 字面量
	Meta           graphics.Color // 预处理指令、注解、装饰器、宏、DOCTYPE、At-Rule
	Escape         graphics.Color // 转义序列
	Entity         graphics.Color // HTML 实体
	Anchor         graphics.Color // YAML 锚点与别名
	Table          graphics.Color // TOML 表头 / YAML 标签
	DateTime       graphics.Color // TOML 日期时间
	Directive      graphics.Color // YAML 指令 / 文档分隔符
	Lifetime       graphics.Color // Rust 生命周期

	Heading1      graphics.Color // Markdown 一级标题
	Heading2      graphics.Color // Markdown 二级标题
	Heading3      graphics.Color // Markdown 三级标题
	Heading4      graphics.Color // Markdown 四级标题
	HeadingMarker graphics.Color // Markdown 标题井号

	CodeBlockText    graphics.Color // Markdown 代码块正文
	CodeBlockBg      graphics.Color // Markdown 代码块底色（可带 alpha）
	CodeBlockMarker  graphics.Color // Markdown 代码块围栏
	CodeLang         graphics.Color // Markdown 代码块语言标记
	InlineCodeText   graphics.Color // Markdown 行内代码正文
	InlineCodeBg     graphics.Color // Markdown 行内代码底色（可带 alpha）
	InlineCodeMarker graphics.Color // Markdown 行内代码反引号
	QuoteText        graphics.Color // Markdown 引用正文
	QuoteMarker      graphics.Color // Markdown 引用标记
	ListMarker       graphics.Color // Markdown 列表标记
	TaskChecked      graphics.Color // Markdown 已完成任务
	TaskUnchecked    graphics.Color // Markdown 未完成任务
	BoldText         graphics.Color // Markdown 粗体正文
	BoldMarker       graphics.Color // Markdown 粗体标记
	ItalicText       graphics.Color // Markdown 斜体正文
	StrikeText       graphics.Color // Markdown 删除线正文
	LinkText         graphics.Color // Markdown 链接文字
	LinkUrl          graphics.Color // Markdown 链接 URL
	ImageMarker      graphics.Color // Markdown 图片标记
	Hr               graphics.Color // Markdown 分割线
}

// SyntaxColors 返回补齐缺省值后的语法令牌。
//
// 主题只需定义核心角色（Keyword/Type/Function/String/Number/Comment/
// Constant/Operator/Punctuation/Meta），其余字段按语义自动派生，
// 避免各语言调色板出现风格漂移。
func (t *Theme) SyntaxColors() SyntaxPalette {
	if t == nil {
		return SyntaxPalette{}
	}
	s := t.Syntax

	if s.Meta == 0 {
		s.Meta = s.Keyword
	}
	if s.Key == 0 {
		s.Key = s.Type
	}
	if s.StringStrong == 0 {
		s.StringStrong = s.String
	}
	if s.Character == 0 {
		s.Character = s.Number
	}
	if s.NumberSoft == 0 {
		s.NumberSoft = s.Number
	}
	if s.CommentSoft == 0 {
		s.CommentSoft = s.Comment
	}
	if s.PunctuationAlt == 0 {
		s.PunctuationAlt = s.Punctuation
	}
	if s.Bracket == 0 {
		s.Bracket = s.Punctuation
	}
	if s.Attribute == 0 {
		s.Attribute = s.Keyword
	}
	if s.Boolean == 0 {
		s.Boolean = s.Keyword
	}
	if s.Null == 0 {
		s.Null = s.Meta
	}
	if s.Escape == 0 {
		s.Escape = s.Number
	}
	if s.Entity == 0 {
		s.Entity = s.Number
	}
	if s.Anchor == 0 {
		s.Anchor = s.Number
	}
	if s.Table == 0 {
		s.Table = s.Meta
	}
	if s.DateTime == 0 {
		s.DateTime = s.Builtin
	}
	if s.Directive == 0 {
		s.Directive = s.Builtin
	}
	if s.Lifetime == 0 {
		s.Lifetime = s.Function
	}

	if s.Heading1 == 0 {
		s.Heading1 = s.Type
	}
	if s.Heading2 == 0 {
		s.Heading2 = s.StringStrong
	}
	if s.Heading3 == 0 {
		s.Heading3 = s.NumberSoft
	}
	if s.Heading4 == 0 {
		s.Heading4 = s.Keyword
	}
	if s.HeadingMarker == 0 {
		s.HeadingMarker = s.Keyword
	}

	if s.CodeBlockText == 0 {
		s.CodeBlockText = s.String
	}
	if s.CodeBlockBg == 0 {
		s.CodeBlockBg = graphics.RGBA(t.SurfaceVariant.R(), t.SurfaceVariant.G(), t.SurfaceVariant.B(), 140)
	}
	if s.CodeBlockMarker == 0 {
		s.CodeBlockMarker = s.CommentSoft
	}
	if s.CodeLang == 0 {
		s.CodeLang = s.Number
	}
	if s.InlineCodeText == 0 {
		s.InlineCodeText = s.NumberSoft
	}
	if s.InlineCodeBg == 0 {
		s.InlineCodeBg = graphics.RGBA(t.SurfaceVariant.R(), t.SurfaceVariant.G(), t.SurfaceVariant.B(), 180)
	}
	if s.InlineCodeMarker == 0 {
		s.InlineCodeMarker = s.Comment
	}
	if s.QuoteText == 0 {
		s.QuoteText = s.Punctuation
	}
	if s.QuoteMarker == 0 {
		s.QuoteMarker = s.Type
	}
	if s.ListMarker == 0 {
		s.ListMarker = s.Number
	}
	if s.TaskChecked == 0 {
		s.TaskChecked = t.Success
	}
	if s.TaskUnchecked == 0 {
		s.TaskUnchecked = t.Warning
	}
	if s.BoldText == 0 {
		s.BoldText = s.Function
	}
	if s.BoldMarker == 0 {
		s.BoldMarker = s.NumberSoft
	}
	if s.ItalicText == 0 {
		s.ItalicText = s.Meta
	}
	if s.StrikeText == 0 {
		s.StrikeText = s.CommentSoft
	}
	if s.LinkText == 0 {
		s.LinkText = s.Type
	}
	if s.LinkUrl == 0 {
		s.LinkUrl = s.CommentSoft
	}
	if s.ImageMarker == 0 {
		s.ImageMarker = s.Meta
	}
	if s.Hr == 0 {
		s.Hr = s.Bracket
	}
	return s
}
