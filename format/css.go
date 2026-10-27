package format

import (
	"errors"
	"strings"
)

// cssIndent 与 HTML/JSON 及编辑器 Tab 保持一致。
const cssIndent = "    "

var errInvalidCSS = errors.New("CSS 结构不完整")

type cssKind uint8

const (
	cssWord cssKind = iota
	cssString
	cssComment
	cssOpen
	cssClose
	cssSemi
	cssColon
	cssComma
	cssParenOpen
	cssParenClose
	cssBracketOpen
	cssBracketClose
)

type cssToken struct {
	kind        cssKind
	text        string
	spaceBefore bool
}

// formatCSS 只用于 <style> 内联样式。base 是每行额外的前缀缩进，
// 便于嵌入 HTML 时直接对齐，同时保证 url(...) 内容不被改写。
func formatCSS(src, base string) (string, error) {
	src = strings.TrimPrefix(src, "\uFEFF")
	if strings.TrimSpace(src) == "" {
		return "", ErrEmptyDocument
	}

	p := &cssPrinter{base: base}
	if err := p.run(tokenizeCSS(src)); err != nil {
		return "", err
	}
	return strings.TrimRight(p.out.String(), "\n"), nil
}

func tokenizeCSS(src string) []cssToken {
	tokens := make([]cssToken, 0, len(src)/8+1)
	space := false

	for i := 0; i < len(src); {
		ch := src[i]
		if isCSSSpace(ch) {
			space = true
			i++
			continue
		}

		if strings.HasPrefix(src[i:], "/*") {
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				end = len(src)
			} else {
				end += i + 4
			}
			tokens = append(tokens, cssToken{kind: cssComment, text: src[i:end], spaceBefore: space})
			space = false
			i = end
			continue
		}

		if ch == '"' || ch == '\'' {
			j := i + 1
			for j < len(src) {
				if src[j] == '\\' {
					j += 2
					continue
				}
				if src[j] == ch {
					j++
					break
				}
				j++
			}
			if j > len(src) {
				j = len(src)
			}
			tokens = append(tokens, cssToken{kind: cssString, text: src[i:j], spaceBefore: space})
			space = false
			i = j
			continue
		}

		kind := cssWord
		switch ch {
		case '{':
			kind = cssOpen
		case '}':
			kind = cssClose
		case ';':
			kind = cssSemi
		case ':':
			kind = cssColon
		case ',':
			kind = cssComma
		case '(':
			kind = cssParenOpen
		case ')':
			kind = cssParenClose
		case '[':
			kind = cssBracketOpen
		case ']':
			kind = cssBracketClose
		}
		if kind != cssWord {
			tokens = append(tokens, cssToken{kind: kind, text: string(ch), spaceBefore: space})
			space = false
			i++
			continue
		}

		j := i
		for j < len(src) {
			c := src[j]
			if isCSSSpace(c) || strings.ContainsRune("{};:,()[]'\"", rune(c)) {
				break
			}
			if c == '/' && j+1 < len(src) && src[j+1] == '*' {
				break
			}
			j++
		}
		if j == i {
			j++
		}
		tokens = append(tokens, cssToken{kind: cssWord, text: src[i:j], spaceBefore: space})
		space = false
		i = j
	}

	return tokens
}

func isCSSSpace(ch byte) bool {
	switch ch {
	case ' ', '\t', '\r', '\n', '\f', '\v':
		return true
	}
	return false
}

type cssPrinter struct {
	out     strings.Builder
	base    string
	depth   int
	buf     []cssToken
	paren   int
	bracket int
}

func (p *cssPrinter) run(tokens []cssToken) error {
	for _, tok := range tokens {
		switch tok.kind {
		case cssComment:
			if len(p.buf) == 0 {
				p.writeComment(tok.text)
			} else {
				p.buf = append(p.buf, tok)
			}

		case cssOpen:
			if p.paren > 0 || p.bracket > 0 {
				p.buf = append(p.buf, tok)
				continue
			}
			if len(p.buf) == 0 {
				return errInvalidCSS
			}
			p.writePrelude(p.buf)
			p.buf = p.buf[:0]
			p.depth++

		case cssSemi:
			if p.paren > 0 || p.bracket > 0 {
				p.buf = append(p.buf, tok)
				continue
			}
			if len(p.buf) == 0 {
				continue
			}
			p.writeStatement(p.buf)
			p.buf = p.buf[:0]

		case cssClose:
			if p.paren > 0 || p.bracket > 0 {
				p.buf = append(p.buf, tok)
				continue
			}
			if p.depth == 0 {
				return errInvalidCSS
			}
			if len(p.buf) > 0 {
				p.writeStatement(p.buf)
				p.buf = p.buf[:0]
			}
			p.depth--
			p.line("}")

		case cssParenOpen:
			p.paren++
			p.buf = append(p.buf, tok)

		case cssParenClose:
			if p.paren > 0 {
				p.paren--
			}
			p.buf = append(p.buf, tok)

		case cssBracketOpen:
			p.bracket++
			p.buf = append(p.buf, tok)

		case cssBracketClose:
			if p.bracket > 0 {
				p.bracket--
			}
			p.buf = append(p.buf, tok)

		default:
			p.buf = append(p.buf, tok)
		}
	}

	if len(p.buf) > 0 {
		p.writeStatement(p.buf)
		p.buf = p.buf[:0]
	}
	if p.depth != 0 || p.paren != 0 || p.bracket != 0 {
		return errInvalidCSS
	}
	return nil
}

func (p *cssPrinter) indent() string {
	return p.base + strings.Repeat(cssIndent, p.depth)
}

func (p *cssPrinter) line(text string) {
	p.out.WriteString(p.indent())
	p.out.WriteString(text)
	p.out.WriteByte('\n')
}

func (p *cssPrinter) writePrelude(tokens []cssToken) {
	groups := splitCSSGroups(tokens)
	if len(groups) == 0 {
		p.line("{")
		return
	}
	for i, group := range groups {
		text := joinCSS(group)
		if text == "" {
			continue
		}
		if i < len(groups)-1 {
			p.line(text + ",")
		} else {
			p.line(text + " {")
		}
	}
}

func (p *cssPrinter) writeStatement(tokens []cssToken) {
	if len(tokens) == 0 {
		return
	}

	depth := 0
	colon := -1
	for i, tok := range tokens {
		switch tok.kind {
		case cssParenOpen, cssBracketOpen:
			depth++
		case cssParenClose, cssBracketClose:
			if depth > 0 {
				depth--
			}
		case cssColon:
			if depth == 0 && colon < 0 {
				colon = i
			}
		}
	}

	if colon > 0 && colon < len(tokens)-1 {
		prop := joinCSS(tokens[:colon])
		value := joinCSS(tokens[colon+1:])
		if prop != "" && value != "" {
			p.line(prop + ": " + value + ";")
			return
		}
	}
	p.line(joinCSS(tokens) + ";")
}

func (p *cssPrinter) writeComment(text string) {
	lines := strings.Split(text, "\n")
	for _, ln := range lines {
		ln = strings.TrimRight(ln, " \t\r")
		if strings.TrimSpace(ln) == "" {
			p.out.WriteByte('\n')
			continue
		}
		p.line(ln)
	}
}

// splitCSSGroups 按顶层逗号切分选择器列表。
func splitCSSGroups(tokens []cssToken) [][]cssToken {
	var groups [][]cssToken
	start := 0
	depth := 0
	for i, tok := range tokens {
		switch tok.kind {
		case cssParenOpen, cssBracketOpen:
			depth++
		case cssParenClose, cssBracketClose:
			if depth > 0 {
				depth--
			}
		case cssComma:
			if depth == 0 {
				groups = append(groups, tokens[start:i])
				start = i + 1
			}
		}
	}
	groups = append(groups, tokens[start:])
	return groups
}

// joinCSS 将 token 还原为单行文本，只折叠空白并补上逗号后的空格。
// url(...) 内部按原文保留，避免破坏 data URI。
func joinCSS(tokens []cssToken) string {
	var b strings.Builder
	var parens []bool // true 表示当前括号是 url(...)

	for i, tok := range tokens {
		inURL := len(parens) > 0 && parens[len(parens)-1]
		space := false
		if i > 0 {
			prev := tokens[i-1]
			switch {
			case inURL:
				space = tok.spaceBefore && prev.kind != cssParenOpen
			case tok.kind == cssComma || tok.kind == cssParenClose || tok.kind == cssBracketClose:
				space = tok.spaceBefore && tok.kind == cssComma
			case prev.kind == cssParenOpen || prev.kind == cssBracketOpen:
				space = false
			case prev.kind == cssComma:
				space = true
			default:
				space = tok.spaceBefore
			}
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(tok.text)

		switch tok.kind {
		case cssParenOpen:
			isURL := i > 0 && strings.EqualFold(tokens[i-1].text, "url")
			parens = append(parens, isURL)
		case cssParenClose:
			if len(parens) > 0 {
				parens = parens[:len(parens)-1]
			}
		}
	}
	return strings.TrimSpace(b.String())
}
