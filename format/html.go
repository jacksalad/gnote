package format

import (
	"strings"
	"unicode"
)

// htmlIndent 格式化 HTML 时使用的缩进单位。
// 与编辑器 Tab 的 4 空格缩进保持一致，避免格式化后再次按 Tab 缩进风格跳变。
const htmlIndent = "    "

// FormatHTML 对 HTML 文档做保守的缩进重排。
//
// 只调整块级标签的行首缩进与标签之间的空白；行内元素、文本、实体与属性值
// 的内容保持不变。pre/textarea/svg/math 的内部文本逐字保留；script/style
// 在类型可识别时交给内联 CSS/JS 格式化器，失败则原样保留。
func FormatHTML(src string) (string, error) {
	src = strings.TrimPrefix(src, "\uFEFF")
	if strings.TrimSpace(src) == "" {
		return "", ErrEmptyDocument
	}

	p := &htmlPrinter{}
	for _, tok := range tokenizeHTML(src) {
		p.writeToken(tok)
	}
	p.flushLine()

	out := strings.TrimRight(p.out.String(), "\n")
	if endsWithNewline(src) {
		out += "\n"
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// 分词
// ---------------------------------------------------------------------------

type htmlTokenKind uint8

const (
	htmlText htmlTokenKind = iota
	htmlStartTag
	htmlEndTag
	htmlComment
	htmlDoctype
	htmlRawElement
)

type htmlToken struct {
	kind        htmlTokenKind
	name        string // 小写标签名，仅标签/原始文本元素使用
	raw         string // 原始源码切片，标签内容与大小写原样保留
	selfClosing bool
}

// tokenizeHTML 把源码切分为标签、注释、声明与文本。
// 它不做 DOM 校验，输入不合法时也只会退化为文本，不会 panic。
func tokenizeHTML(src string) []htmlToken {
	tokens := make([]htmlToken, 0, len(src)/16+1)

	for i := 0; i < len(src); {
		if src[i] != '<' {
			j := strings.IndexByte(src[i:], '<')
			if j < 0 {
				j = len(src) - i
			}
			tokens = append(tokens, htmlToken{kind: htmlText, raw: src[i : i+j]})
			i += j
			continue
		}

		rest := src[i:]
		switch {
		case strings.HasPrefix(rest, "<!--"):
			end := strings.Index(rest[4:], "-->")
			if end < 0 {
				tokens = append(tokens, htmlToken{kind: htmlComment, raw: rest})
				i = len(src)
			} else {
				end += 4 + 3
				tokens = append(tokens, htmlToken{kind: htmlComment, raw: rest[:end]})
				i += end
			}

		case strings.HasPrefix(rest, "<![CDATA["):
			end := strings.Index(rest[9:], "]]>")
			if end < 0 {
				tokens = append(tokens, htmlToken{kind: htmlDoctype, raw: rest})
				i = len(src)
			} else {
				end += 9 + 3
				tokens = append(tokens, htmlToken{kind: htmlDoctype, raw: rest[:end]})
				i += end
			}

		case strings.HasPrefix(rest, "<!") || strings.HasPrefix(rest, "<?"):
			end := scanTagEnd(rest, 2)
			if end < 0 {
				tokens = append(tokens, htmlToken{kind: htmlText, raw: rest})
				i = len(src)
			} else {
				tokens = append(tokens, htmlToken{kind: htmlDoctype, raw: rest[:end]})
				i += end
			}

		case strings.HasPrefix(rest, "</"):
			nameEnd := 2
			for nameEnd < len(rest) && isHTMLNameByte(rest[nameEnd]) {
				nameEnd++
			}
			name := strings.ToLower(rest[2:nameEnd])
			end := scanTagEnd(rest, nameEnd)
			if end < 0 {
				tokens = append(tokens, htmlToken{kind: htmlText, raw: rest})
				i = len(src)
			} else {
				tokens = append(tokens, htmlToken{kind: htmlEndTag, name: name, raw: rest[:end]})
				i += end
			}

		default:
			nameEnd := 1
			for nameEnd < len(rest) && isHTMLNameByte(rest[nameEnd]) {
				nameEnd++
			}
			if nameEnd == 1 {
				// 单独的 '<'，按文本处理。
				tokens = append(tokens, htmlToken{kind: htmlText, raw: "<"})
				i++
				continue
			}
			name := strings.ToLower(rest[1:nameEnd])
			end := scanTagEnd(rest, nameEnd)
			if end < 0 {
				tokens = append(tokens, htmlToken{kind: htmlText, raw: rest})
				i = len(src)
				continue
			}
			raw := rest[:end]
			selfClosing := isSelfClosingTag(raw)
			i += end

			if isRawTextTag(name) && !selfClosing {
				if closeEnd, ok := findRawCloseEnd(src, i, name); ok {
					raw = src[i-end : closeEnd]
					i = closeEnd
				} else {
					raw = src[i-end:]
					i = len(src)
				}
				tokens = append(tokens, htmlToken{kind: htmlRawElement, name: name, raw: raw})
				continue
			}

			tokens = append(tokens, htmlToken{
				kind:        htmlStartTag,
				name:        name,
				raw:         raw,
				selfClosing: selfClosing,
			})
		}
	}

	return tokens
}

// scanTagEnd 返回标签结束 '>' 之后的下标（相对 s），引号内的 '>' 不计入。
func scanTagEnd(s string, from int) int {
	if from > len(s) {
		return -1
	}
	var quote byte
	for i := from; i < len(s); i++ {
		ch := s[i]
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '"', '\'':
			quote = ch
		case '>':
			return i + 1
		}
	}
	return -1
}

func isHTMLNameByte(ch byte) bool {
	return ch == '-' || ch == '_' || ch == ':' ||
		(ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
}

func isSelfClosingTag(raw string) bool {
	body := strings.TrimSuffix(strings.TrimSpace(raw), ">")
	return strings.HasSuffix(body, "/")
}

// findRawCloseEnd 从 from 起查找 </name ...>，返回闭合标签之后的下标。
func findRawCloseEnd(src string, from int, name string) (int, bool) {
	for i := from; i+2 < len(src); i++ {
		if src[i] != '<' || src[i+1] != '/' {
			continue
		}
		j := i + 2
		for j < len(src) && isHTMLNameByte(src[j]) {
			j++
		}
		if !strings.EqualFold(src[i+2:j], name) {
			continue
		}
		k := j
		for k < len(src) && unicode.IsSpace(rune(src[k])) {
			k++
		}
		if k < len(src) && src[k] == '>' {
			return k + 1, true
		}
	}
	return 0, false
}

func isRawTextTag(name string) bool {
	switch name {
	case "script", "style", "pre", "textarea", "svg", "math":
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// 排版
// ---------------------------------------------------------------------------

type htmlPrinter struct {
	out   strings.Builder
	line  strings.Builder
	stack []string // 仅记录块级标签，用于缩进层级
}

func (p *htmlPrinter) writeToken(tok htmlToken) {
	switch tok.kind {
	case htmlText:
		p.writeText(tok.raw)
	case htmlStartTag:
		p.writeStartTag(tok)
	case htmlEndTag:
		p.writeEndTag(tok)
	case htmlComment:
		p.writeVerbatim(tok.raw, "-->")
	case htmlDoctype:
		p.writeVerbatim(tok.raw, "")
	case htmlRawElement:
		p.writeRawElement(tok.raw, tok.name)
	}
}

func (p *htmlPrinter) writeStartTag(tok htmlToken) {
	p.autoCloseBeforeStart(tok.name)
	raw := normalizeTag(tok.raw)

	if !isBlockTag(tok.name) {
		p.writeInline(raw)
		return
	}

	p.flushLine()
	p.line.WriteString(p.indent())
	p.line.WriteString(raw)

	if tok.selfClosing || isVoidTag(tok.name) {
		p.flushLine()
		return
	}
	p.stack = append(p.stack, tok.name)
}

func (p *htmlPrinter) writeEndTag(tok htmlToken) {
	raw := normalizeTag(tok.raw)
	if !isBlockTag(tok.name) {
		p.writeInline(raw)
		return
	}

	if idx := p.findStack(tok.name); idx >= 0 {
		p.stack = p.stack[:idx]
	}
	// 行内尾部内容（如 <img>）不与块级闭合标签挤在同一行；
	// 若当前行正是该块级标签自身的起始行（<p>text），则闭合标签紧跟其后。
	if p.line.Len() > 0 && !p.lineStartsWithOpenTag(tok.name) {
		p.flushLine()
	}
	if p.line.Len() == 0 {
		p.line.WriteString(p.indent())
	}
	p.line.WriteString(raw)
	p.flushLine()
}

func (p *htmlPrinter) writeText(raw string) {
	s := collapseWhitespace(raw)
	if s == "" {
		return
	}
	if p.line.Len() == 0 {
		s = strings.TrimLeft(s, " ")
		if s == "" {
			return
		}
		p.line.WriteString(p.indent())
	}
	p.line.WriteString(s)
}

func (p *htmlPrinter) writeInline(raw string) {
	if p.line.Len() == 0 {
		p.line.WriteString(p.indent())
	}
	p.line.WriteString(raw)
}

// writeRawElement 输出 script/style/pre/textarea/svg/math。
// script/style 会尝试格式化内联 CSS/JS；失败或类型未知时退回逐字保留。
// pre/textarea/svg/math 始终逐字保留，仅将首行与独占一行的闭合标签对齐到当前层级。
func (p *htmlPrinter) writeRawElement(raw, name string) {
	p.flushLine()
	if p.writeEmbedded(raw, name) {
		return
	}

	first, rest, multi := strings.Cut(raw, "\n")
	p.out.WriteString(p.indent())
	p.out.WriteString(strings.TrimSuffix(first, "\r"))
	p.out.WriteByte('\n')
	if !multi {
		return
	}

	lines := strings.Split(rest, "\n")
	closeOnly := strings.EqualFold(strings.TrimSpace(lines[len(lines)-1]), "</"+name+">")
	if closeOnly {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > 0 {
		p.out.WriteString(strings.Join(lines, "\n"))
		p.out.WriteByte('\n')
	}
	if closeOnly {
		p.out.WriteString(p.indent())
		p.out.WriteString("</" + name + ">")
		p.out.WriteByte('\n')
	}
}

// writeEmbedded 格式化 <style> 与 <script> 的内联内容。
// 只在类型明确且内容非空时执行；任何解析失败都返回 false，由调用方保留原文。
func (p *htmlPrinter) writeEmbedded(raw, name string) bool {
	start, content, end, ok := splitRawElement(raw, name)
	if !ok || strings.TrimSpace(content) == "" {
		return false
	}

	base := p.indent() + htmlIndent
	var (
		formatted string
		err       error
	)
	switch name {
	case "style":
		if !isCSSStyleTag(start) {
			return false
		}
		formatted, err = formatCSS(content, base)
	case "script":
		if !isJSTypeTag(start) {
			return false
		}
		formatted, err = formatJS(content, base)
	default:
		return false
	}
	if err != nil || strings.TrimSpace(formatted) == "" {
		return false
	}

	p.out.WriteString(p.indent())
	p.out.WriteString(normalizeTag(start))
	p.out.WriteByte('\n')
	p.out.WriteString(strings.TrimRight(formatted, "\n"))
	p.out.WriteByte('\n')
	p.out.WriteString(p.indent())
	p.out.WriteString(normalizeTag(end))
	p.out.WriteByte('\n')
	return true
}

// splitRawElement 拆分 <tag ...>content</tag>，保留标签原文。
func splitRawElement(raw, name string) (start, content, end string, ok bool) {
	startEnd := scanTagEnd(raw, 1)
	if startEnd < 0 {
		return "", "", "", false
	}
	closeIdx := strings.LastIndex(strings.ToLower(raw), "</"+name)
	if closeIdx < startEnd || !strings.HasSuffix(strings.TrimSpace(raw[closeIdx:]), ">") {
		return "", "", "", false
	}
	return raw[:startEnd], raw[startEnd:closeIdx], raw[closeIdx:], true
}

func htmlTagAttr(tag, want string) (string, bool) {
	i := 1
	for i < len(tag) && isHTMLNameByte(tag[i]) {
		i++
	}
	for i < len(tag) {
		for i < len(tag) && isHTMLSpace(tag[i]) {
			i++
		}
		if i >= len(tag) || tag[i] == '>' || tag[i] == '/' {
			break
		}
		start := i
		for i < len(tag) && isHTMLNameByte(tag[i]) {
			i++
		}
		if i == start {
			i++
			continue
		}
		name := strings.ToLower(tag[start:i])
		for i < len(tag) && isHTMLSpace(tag[i]) {
			i++
		}
		value := ""
		if i < len(tag) && tag[i] == '=' {
			i++
			for i < len(tag) && isHTMLSpace(tag[i]) {
				i++
			}
			if i < len(tag) && (tag[i] == '"' || tag[i] == '\'') {
				quote := tag[i]
				i++
				valueStart := i
				for i < len(tag) && tag[i] != quote {
					i++
				}
				value = tag[valueStart:i]
				if i < len(tag) {
					i++
				}
			} else {
				valueStart := i
				for i < len(tag) && !isHTMLSpace(tag[i]) && tag[i] != '>' {
					i++
				}
				value = tag[valueStart:i]
			}
		}
		if name == want {
			return value, true
		}
	}
	return "", false
}

func isHTMLSpace(ch byte) bool {
	switch ch {
	case ' ', '\t', '\r', '\n', '\f':
		return true
	}
	return false
}

func isJSTypeTag(startTag string) bool {
	if typ, ok := htmlTagAttr(startTag, "type"); ok {
		typ = strings.ToLower(strings.TrimSpace(strings.SplitN(typ, ";", 2)[0]))
		switch typ {
		case "", "module", "text/javascript", "application/javascript",
			"text/ecmascript", "application/ecmascript",
			"application/x-javascript", "text/jscript":
			return true
		}
		return false
	}
	if lang, ok := htmlTagAttr(startTag, "language"); ok {
		lang = strings.ToLower(strings.TrimSpace(lang))
		switch {
		case strings.HasPrefix(lang, "javascript"), strings.HasPrefix(lang, "ecmascript"), lang == "js":
			return true
		}
		return false
	}
	return true
}

func isCSSStyleTag(startTag string) bool {
	if typ, ok := htmlTagAttr(startTag, "type"); ok {
		typ = strings.ToLower(strings.TrimSpace(strings.SplitN(typ, ";", 2)[0]))
		return typ == "" || typ == "text/css"
	}
	return true
}

// writeVerbatim 输出注释/声明。首行缩进，后续行逐字保留；
// closeMarker 非空且独占末行时，把结束标记对齐到当前层级。
func (p *htmlPrinter) writeVerbatim(raw, closeMarker string) {
	p.flushLine()

	first, rest, multi := strings.Cut(raw, "\n")
	p.out.WriteString(p.indent())
	p.out.WriteString(strings.TrimSuffix(first, "\r"))
	p.out.WriteByte('\n')
	if !multi {
		return
	}

	lines := strings.Split(rest, "\n")
	closeOnly := closeMarker != "" &&
		strings.EqualFold(strings.TrimSpace(lines[len(lines)-1]), closeMarker)
	if closeOnly {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > 0 {
		p.out.WriteString(strings.Join(lines, "\n"))
		p.out.WriteByte('\n')
	}
	if closeOnly {
		p.out.WriteString(p.indent())
		p.out.WriteString(closeMarker)
		p.out.WriteByte('\n')
	}
}

func (p *htmlPrinter) flushLine() {
	s := strings.TrimRight(p.line.String(), " \t")
	if strings.TrimSpace(s) != "" {
		p.out.WriteString(s)
		p.out.WriteByte('\n')
	}
	p.line.Reset()
}

func (p *htmlPrinter) indent() string {
	return strings.Repeat(htmlIndent, len(p.stack))
}

// lineStartsWithOpenTag 判断当前行（去掉缩进后）是否以指定块级开始标签开头。
func (p *htmlPrinter) lineStartsWithOpenTag(name string) bool {
	s := strings.TrimLeft(p.line.String(), " \t")
	if len(s) < 2 || s[0] != '<' || s[1] == '/' || s[1] == '!' || s[1] == '?' {
		return false
	}
	s = s[1:]
	k := 0
	for k < len(s) && isHTMLNameByte(s[k]) {
		k++
	}
	if k == 0 || !strings.EqualFold(s[:k], name) {
		return false
	}
	return k == len(s) || s[k] == ' ' || s[k] == '\t' || s[k] == '>' || s[k] == '/'
}

func (p *htmlPrinter) findStack(name string) int {
	for i := len(p.stack) - 1; i >= 0; i-- {
		if p.stack[i] == name {
			return i
		}
	}
	return -1
}

// autoCloseBeforeStart 处理 HTML 可选结束标签，避免未闭合的 li/td/p
// 让缩进层级无限增长。
func (p *htmlPrinter) autoCloseBeforeStart(name string) {
	switch name {
	case "li":
		p.popTop("li")
	case "dt", "dd":
		p.popTop("dt", "dd")
	case "tr":
		p.popTop("td", "th")
		p.popTop("tr")
	case "td", "th":
		p.popTop("td", "th")
	case "thead", "tbody", "tfoot":
		p.popTop("td", "th")
		p.popTop("tr")
		p.popTop("thead", "tbody", "tfoot")
	case "option":
		p.popTop("option")
	case "optgroup":
		p.popTop("option")
		p.popTop("optgroup")
	case "p":
		p.popTop("p")
	}
	if isBlockTag(name) && name != "p" {
		for len(p.stack) > 0 && p.stack[len(p.stack)-1] == "p" {
			p.stack = p.stack[:len(p.stack)-1]
		}
	}
}

func (p *htmlPrinter) popTop(names ...string) {
	if len(p.stack) == 0 {
		return
	}
	top := p.stack[len(p.stack)-1]
	for _, name := range names {
		if top == name {
			p.stack = p.stack[:len(p.stack)-1]
			return
		}
	}
}

// ---------------------------------------------------------------------------
// 标签属性
// ---------------------------------------------------------------------------

// normalizeTag 折叠标签内引号外的空白，属性值逐字保留。
func normalizeTag(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))

	var quote byte
	space := false
	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if quote != 0 {
			b.WriteByte(ch)
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '"', '\'':
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			quote = ch
			b.WriteByte(ch)
		case ' ', '\t', '\r', '\n', '\f':
			space = true
		default:
			if space && b.Len() > 0 && ch != '>' && !(ch == '/' && i+1 < len(raw) && raw[i+1] == '>') {
				b.WriteByte(' ')
			}
			space = false
			b.WriteByte(ch)
		}
	}
	return strings.TrimSpace(b.String())
}

// collapseWhitespace 把 rune 空白折叠为单个空格。
// 保留首尾空格，让行内元素之间的空白（<b>a</b> <i>b</i>）不被吞掉；
// 行首的多余缩进由 writeText 在空行场景裁剪。
func collapseWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	space := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			space = true
			continue
		}
		if space {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	if space {
		b.WriteByte(' ')
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// 标签分类
// ---------------------------------------------------------------------------

var blockTagSet = map[string]bool{
	"html": true, "head": true, "body": true, "title": true, "base": true,
	"link": true, "meta": true, "style": true, "script": true, "noscript": true,
	"template": true, "div": true, "p": true, "section": true, "article": true,
	"aside": true, "header": true, "footer": true, "nav": true, "main": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"ul": true, "ol": true, "li": true, "dl": true, "dt": true, "dd": true,
	"table": true, "thead": true, "tbody": true, "tfoot": true, "tr": true,
	"td": true, "th": true, "caption": true, "colgroup": true, "col": true,
	"form": true, "fieldset": true, "legend": true, "figure": true,
	"figcaption": true, "blockquote": true, "hr": true, "pre": true,
	"address": true, "details": true, "summary": true, "dialog": true,
	"hgroup": true, "menu": true, "search": true, "select": true, "optgroup": true, "option": true,
}

var voidTagSet = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}

func isBlockTag(name string) bool { return blockTagSet[name] }

func isVoidTag(name string) bool { return voidTagSet[name] }
