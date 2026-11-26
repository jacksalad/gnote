// Package format 提供零依赖的文档格式化能力。
//
// 当前支持 JSON 与 HTML 两种文档：JSON 通过 encoding/json 的 token 级
// 缩进保持键顺序与数字字面量不变；HTML 使用保守的纯 Go 分词器重排块级
// 标签缩进，并对内联 <style>/<script> 尝试格式化 CSS/JS。
// pre/textarea/svg/math 等原始文本元素与 white-space:pre* 元素的内容始终逐字保留；
// CSS/JS 解析失败时也保留原文。无法安全定界的输入（到末尾都没有 '>' 的标签、
// 没有闭合或带空选择器的 CSS）返回错误，由调用方保持文档不变。
package format

import (
	"errors"
	"path/filepath"
	"strings"
)

// Kind 标识可格式化的文档类型。
type Kind uint8

const (
	KindNone Kind = iota
	KindJSON
	KindHTML
)

var (
	// ErrEmptyDocument 文档为空或仅含空白字符。
	ErrEmptyDocument = errors.New("文档为空，没有可格式化的内容")
	// ErrUnsupported 当前文档类型没有注册格式化器。
	ErrUnsupported = errors.New("该文件类型暂不支持格式化")
)

// maxIndentDepth 是缩进层级上限。未闭合的标签或超深嵌套不再线性放大缩进字符串：
// 否则 n 个未闭合的 <div> 会输出约 n²/2 份缩进（8000 个就是 128 MB），而格式化
// 在 UI 线程上同步执行。超过上限的层级沿用最深一层的缩进。
const maxIndentDepth = 32

// indentPrefix 返回 depth 层缩进，超过 maxIndentDepth 的部分不再增加。
func indentPrefix(unit string, depth int) string {
	if depth > maxIndentDepth {
		depth = maxIndentDepth
	}
	return strings.Repeat(unit, depth)
}

// KindForPath 根据文件后缀判断格式化类型。文件名大小写不敏感。
func KindForPath(path string) Kind {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(path))) {
	case ".json":
		return KindJSON
	case ".html", ".htm":
		return KindHTML
	default:
		return KindNone
	}
}

// Format 按文档类型格式化 src。返回的文本统一使用 '\n' 换行，
// 并由 TextArea 在保存时按原文件的 EOL 风格序列化。
func Format(kind Kind, src string) (string, error) {
	switch kind {
	case KindJSON:
		return FormatJSON(src)
	case KindHTML:
		return FormatHTML(src)
	default:
		return "", ErrUnsupported
	}
}
