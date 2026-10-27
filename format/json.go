package format

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// jsonIndent 格式化 JSON 时使用的缩进单位。
// 与编辑器 Tab 的 4 空格缩进保持一致，避免格式化后再次按 Tab 缩进风格跳变。
const jsonIndent = "    "

// FormatJSON 对 JSON 文档做 token 级缩进。
//
// 使用 json.Indent 而不是 Unmarshal/Marshal：键顺序、数字字面量（含大整数
// 与高精度小数）、字符串转义与 \u 序列都原样保留，仅重排空白。
// 原文档以换行结尾时，输出同样以单个 '\n' 结尾。
func FormatJSON(src string) (string, error) {
	src = strings.TrimPrefix(src, "\uFEFF")
	trimmed := strings.TrimSpace(src)
	if trimmed == "" {
		return "", ErrEmptyDocument
	}

	var buf bytes.Buffer
	buf.Grow(len(trimmed) + len(trimmed)/4)
	if err := json.Indent(&buf, []byte(trimmed), "", jsonIndent); err != nil {
		if syn, ok := err.(*json.SyntaxError); ok {
			line, col := jsonErrorPosition(trimmed, syn.Offset)
			return "", fmt.Errorf("JSON 语法错误（第 %d 行第 %d 列）：%v", line, col, err)
		}
		return "", fmt.Errorf("JSON 解析失败：%w", err)
	}

	out := buf.String()
	if endsWithNewline(src) {
		out += "\n"
	}
	return out, nil
}

// jsonErrorPosition 将 1-based 字节偏移转换为 1-based 行列号，
// 列号按 rune 计算，保证中文与 emoji 前的错误定位仍然直观。
func jsonErrorPosition(src string, offset int64) (line, col int) {
	line, col = 1, 1
	if offset <= 1 {
		return line, col
	}
	limit := int(offset - 1)
	if limit > len(src) {
		limit = len(src)
	}
	for i := 0; i < limit; {
		r, size := utf8.DecodeRuneInString(src[i:])
		i += size
		if r == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}

func endsWithNewline(s string) bool {
	return strings.HasSuffix(s, "\n") || strings.HasSuffix(s, "\r")
}
