package format

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// htmlIndent 格式化 HTML 时使用的缩进单位。
// 与编辑器 Tab 的 4 空格缩进保持一致，避免格式化后再次按 Tab 缩进风格跳变。
const htmlIndent = "    "

var errInvalidHTML = errors.New("HTML 标签未闭合，无法安全确定文档结构")

// FormatHTML 对 HTML 文档做保守的缩进重排。
//
// 只调整块级标签的行首缩进与标签之间的空白；行内元素、文本、实体与属性值
// 的内容保持不变。pre/textarea/svg/math 等原始文本元素与内联 style 声明了
// white-space:pre* 的元素内部逐字保留；script/style 在类型可识别时交给内联
// CSS/JS 格式化器，失败则原样保留。遇到到文档末尾都没有 '>' 的标签时返回
// errInvalidHTML，由调用方保持文档不变。
func FormatHTML(src string) (string, error) {
	src = strings.TrimPrefix(src, "\uFEFF")
	if strings.TrimSpace(src) == "" {
		return "", ErrEmptyDocument
	}

	tokens, err := tokenizeHTML(src)
	if err != nil {
		return "", err
	}

	p := &htmlPrinter{}
	knownTo, knownAfter := -1, false // 已算出的“后面是否紧贴行内内容”适用的 Token 下标上界与取值
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		last := i
		if tok.kind == htmlStartTag && hasPreWhitespace(tok.raw) && !isVoidTag(tok.name) {
			// white-space:pre* 元素里的空格与换行都有渲染意义：
			// 从开始标签到结束标签整体并入一个逐字保留的 Token。
			last = findVerbatimEnd(tokens, i, tok.name) - 1
			tok = htmlToken{kind: htmlRawElement, name: tok.name, raw: joinTokens(tokens[i : last+1])}
			i = last
		}
		// 只有行内级的原始元素与注释需要知道后面是否紧贴行内内容；按需计算，
		// 连续的注释共用同一次向后扫描的结果。
		var inlineAfter bool
		if mayNeedInlineAfter(tok) {
			if last <= knownTo {
				inlineAfter = knownAfter
			} else {
				// 判定用的 Token 本身不在此列：它后面的情况还没有算过。
				var decidedBy int
				inlineAfter, decidedBy = htmlInlineAfter(tokens, last)
				knownTo, knownAfter = decidedBy-1, inlineAfter
			}
		}
		p.writeToken(tok, inlineAfter)
	}
	p.flushLine()

	out := p.out.String()
	if !p.rawTail {
		out = strings.TrimRight(out, "\n")
	}
	if endsWithNewline(src) && !strings.HasSuffix(out, "\n") {
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
	kind htmlTokenKind
	name string // 小写标签名，仅标签/原始文本元素使用
	raw  string // 原始源码切片，标签内容与大小写原样保留
}

// tokenizeHTML 把源码切分为标签、注释、声明与文本。
// 它不做 DOM 校验，输入不合法时也只会退化为文本，不会 panic；唯一的例外是到
// 文档末尾都没有 '>' 的标签：浏览器会丢弃这种标签，排版器也无从知道标记从
// 哪里恢复，因此返回 errInvalidHTML 而不是猜测。
func tokenizeHTML(src string) ([]htmlToken, error) {
	tokens := make([]htmlToken, 0, htmlTokenCap(src))

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
				return nil, errUnclosedTag(src, i)
			}
			tokens = append(tokens, htmlToken{kind: htmlEndTag, name: name, raw: rest[:end]})
			i += end

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
				return nil, errUnclosedTag(src, i)
			}
			raw := rest[:end]
			i += end

			if isRawTextTag(name) {
				// 原始文本元素都不是 void：浏览器会忽略 <pre/> 上的斜杠，
				// 内容照常按原文读到结束标签。
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

			// 只有 void 元素承认自闭合写法；<div/> 等标签上的斜杠被浏览器忽略。
			tokens = append(tokens, htmlToken{kind: htmlStartTag, name: name, raw: raw})
		}
	}

	return tokens, nil
}

// htmlTokenCap 估算 Token 数量的预留容量。每个 '<' 至多产生一个标签类 Token，
// 相邻的标签类 Token 之间至多夹一个文本 Token，所以 2×'<' 数 + 1 是上界：
// 大段脚本或纯文本的文档不会再按 len(src)/16 预留出用不上的几十 MB
// （切片里是指针，GC 每轮都要扫描）。
func htmlTokenCap(src string) int {
	n := len(src)/16 + 1
	if bound := 2*strings.Count(src, "<") + 1; bound < n {
		n = bound
	}
	return n
}

// errUnclosedTag 为位于 src[at] 的未闭合标签生成带行列号的错误。
func errUnclosedTag(src string, at int) error {
	line := 1 + strings.Count(src[:at], "\n")
	col := 1 + utf8.RuneCountInString(src[strings.LastIndexByte(src[:at], '\n')+1:at])
	return fmt.Errorf("%w（第 %d 行第 %d 列）", errInvalidHTML, line, col)
}

// scanTagEnd 返回标签结束 '>' 之后的下标（相对 s），引号内的 '>' 不计入。
func scanTagEnd(s string, from int) int {
	if from > len(s) {
		return -1
	}
	var sc tagScanner
	return sc.scan(s, from)
}

// 标签内部的状态，与 HTML 分词器的属性状态一一对应。
const (
	tagBeforeName  byte = iota // 标签名之后、属性名之前
	tagInName                  // 属性名内部
	tagAfterName               // 属性名之后
	tagBeforeValue             // '=' 之后
	tagInQuoted                // 引号属性值内部
	tagInUnquoted              // 无引号属性值内部
)

// tagScanner 复刻 HTML 分词器在标签内部对引号的处理：引号只有紧跟在 '=' 与
// 可选空白之后才开启“引号属性值”，无引号属性值里的引号只是普通字符
// （title=Don't panic）。此前任何引号都会进入引号状态，一个撇号就能让标签
// 永远不结束，其后的整个文档被吞成一个文本 Token。
type tagScanner struct {
	state byte
	quote byte
}

// scan 从 i 起喂入字节，返回标签结束 '>' 之后的下标；到末尾仍未结束返回 -1。
func (s *tagScanner) scan(str string, i int) int {
	for ; i < len(str); i++ {
		if s.step(str[i]) {
			return i + 1
		}
	}
	return -1
}

// step 喂入一个字节，返回它是否是标签结束符 '>'。
func (s *tagScanner) step(ch byte) bool {
	switch s.state {
	case tagInQuoted:
		if ch == s.quote {
			s.state = tagBeforeName
		}
	case tagInUnquoted:
		switch {
		case ch == '>':
			return true
		case isHTMLSpace(ch):
			s.state = tagBeforeName
		}
	case tagBeforeValue:
		switch {
		case isHTMLSpace(ch):
		case ch == '"' || ch == '\'':
			s.quote, s.state = ch, tagInQuoted
		case ch == '>':
			return true // 缺少属性值：浏览器照样在这里结束标签
		default:
			s.state = tagInUnquoted
		}
	default: // tagBeforeName / tagInName / tagAfterName
		switch {
		case ch == '>':
			return true
		case isHTMLSpace(ch):
			if s.state == tagInName {
				s.state = tagAfterName
			}
		case ch == '/':
			s.state = tagBeforeName
		case ch == '=' && s.state != tagBeforeName:
			s.state = tagBeforeValue
		default:
			s.state = tagInName
		}
	}
	return false
}

func isHTMLNameByte(ch byte) bool {
	return ch == '-' || ch == '_' || ch == ':' ||
		(ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
}

// findRawCloseEnd 从 from 起查找 </name ...>，返回闭合标签之后的下标。
// </pre\u00A0> 这类写法在浏览器里不是结束标签（NBSP 不是标签内的空白），
// 所以只接受 HTML 空白。
func findRawCloseEnd(src string, from int, name string) (int, bool) {
	if name == "plaintext" {
		return 0, false // <plaintext> 之后直到文档末尾全是原文，没有结束标签
	}
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
		for k < len(src) && isHTMLSpace(src[k]) {
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
	case "script", "style", "pre", "textarea", "svg", "math",
		"xmp", "listing", "plaintext":
		return true
	}
	return false
}

// isRawBlockName 判断原始文本元素是否在默认样式下独占一行（块级）。
// svg/math/textarea 是行内级元素，见 writeRawElement。
func isRawBlockName(name string) bool {
	switch name {
	case "pre", "script", "style", "xmp", "listing", "plaintext":
		return true
	}
	return false
}

// htmlInlineAfter 报告 tokens[i] 之后（中间没有源码空白）是否紧跟着行内内容，
// 并返回据以判定的 Token 下标。行内级的原始元素只有在一侧紧贴行内内容时才必须
// 留在同一行；注释本身不渲染，对它要越过连续的注释继续往后看。
func htmlInlineAfter(tokens []htmlToken, i int) (bool, int) {
	for j := i + 1; j < len(tokens); j++ {
		next := tokens[j]
		switch {
		case next.kind == htmlComment || next.kind == htmlDoctype:
			continue
		case next.kind == htmlText:
			return !startsWithHTMLSpace(next.raw), j
		case next.kind == htmlRawElement:
			return !isRawBlockName(next.name) && !isBlockTag(next.name), j
		case next.kind == htmlStartTag || next.kind == htmlEndTag:
			return !isBlockTag(next.name), j
		}
	}
	return false, len(tokens)
}

// mayNeedInlineAfter 报告 Token 是否需要知道它后面是否紧贴行内内容：
// 只有行内级的原始元素与注释/声明需要，块级元素总是独占一行。
func mayNeedInlineAfter(tok htmlToken) bool {
	switch tok.kind {
	case htmlComment, htmlDoctype:
		return true
	case htmlRawElement:
		return !isRawBlockName(tok.name) && !isBlockTag(tok.name)
	}
	return false
}

func startsWithHTMLSpace(s string) bool {
	return s != "" && isHTMLSpace(s[0])
}

// hasPreWhitespace 判断开始标签的内联 style 是否声明了 pre 系列的 white-space。
// 这类元素里的空格与换行都有渲染意义，不能折叠，也不能重新缩进。
// 只认内联 style：class 或外部样式表里的 white-space 无法从标签本身得知，不在处理范围内。
func hasPreWhitespace(startTag string) bool {
	style, ok := htmlTagAttr(startTag, "style")
	if !ok {
		return false
	}
	for style != "" {
		var decl string
		decl, style, _ = strings.Cut(style, ";")
		prop, value, found := strings.Cut(decl, ":")
		if !found || !strings.EqualFold(strings.TrimSpace(prop), "white-space") {
			continue
		}
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "pre" || value == "break-spaces" || strings.HasPrefix(value, "pre-") || strings.HasPrefix(value, "pre ") {
			return true
		}
	}
	return false
}

// findVerbatimEnd 返回 white-space:pre* 的 name 元素在 tokens 中占到的下标（不含）。
// 只有 name 自己的结束标签能结束这个范围，并且要扣除嵌套的同名元素；找不到时
// 一律延续到文档末尾。浏览器还会在遇到 <p>、<td> 等元素的同级开始标签时隐式
// 闭合元素，但那之后直到下一个标签之间的空白仍属于这个元素，排版器只要在那里
// 换行或缩进就会写进它的内容，所以宁可少排版，也不越过显式结束标签。
// open 记录范围内尚未闭合的元素，用来区分“自己的结束标签”和“嵌套元素的结束标签”。
func findVerbatimEnd(tokens []htmlToken, start int, name string) int {
	open := []string{name}
	for k := start + 1; k < len(tokens); k++ {
		t := tokens[k]
		switch t.kind {
		case htmlStartTag:
			if !isVoidTag(t.name) {
				open = append(open, t.name)
			}
		case htmlEndTag:
			if isVoidTag(t.name) {
				continue
			}
			nested := false
			for idx := len(open) - 1; idx > 0; idx-- {
				if open[idx] == t.name {
					open = open[:idx] // 嵌套元素闭合，它里面未闭合的元素一并结束
					nested = true
					break
				}
			}
			if t.name == name && !nested {
				return k + 1
			}
		}
	}
	return len(tokens)
}

// joinTokens 按顺序拼接一段 Token 的原文。Token 连续划分了整个源码，
// 结果就是对应的源码切片。
func joinTokens(tokens []htmlToken) string {
	if len(tokens) == 1 {
		return tokens[0].raw
	}
	var b strings.Builder
	for _, t := range tokens {
		b.WriteString(t.raw)
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// 排版
// ---------------------------------------------------------------------------

type htmlPrinter struct {
	out   strings.Builder
	line  strings.Builder
	stack []string // 仅记录块级标签，用于缩进层级

	// 当前行（自上次 flushLine 之后写入的内容）的行内上下文，用来决定能否在
	// 行内级原始元素/注释前后插入换行。插入换行等于插入一个可折叠空格，只有在
	// 源码本来就有空白、或落在块级边界上时才是安全的。
	lineContent bool // 已有行内内容（文本或行内标签）
	lineSpace   bool // 以可折叠空白结尾：源码在这里本来就有空白
	lineRaw     bool // 以逐字保留的内容结尾：行尾空白不能裁剪
	rawTail     bool // 输出以逐字保留的内容结尾：文档末尾的换行不能裁剪
}

// writeToken 输出一个 Token。inlineAfter 表示它之后（中间没有源码空白）紧跟着行内内容。
func (p *htmlPrinter) writeToken(tok htmlToken, inlineAfter bool) {
	switch tok.kind {
	case htmlText:
		p.writeText(tok.raw)
	case htmlStartTag:
		p.writeStartTag(tok)
	case htmlEndTag:
		p.writeEndTag(tok)
	case htmlComment, htmlDoctype:
		// 注释本身不渲染：只要有一侧不是紧贴的行内内容，独占一行就落在
		// 空白会被折叠的位置；两侧都紧贴时必须原样留在行内。
		if p.glued() && inlineAfter {
			p.writeInlineRaw(tok.raw)
			return
		}
		if tok.kind == htmlComment {
			p.writeVerbatim(tok.raw, "-->")
		} else {
			p.writeVerbatim(tok.raw, "")
		}
	case htmlRawElement:
		p.writeRawElement(tok.raw, tok.name, inlineAfter)
	}
}

// glued 报告当前行是否以行内内容结尾、且这个 Token 与它之间没有源码空白。
func (p *htmlPrinter) glued() bool {
	return p.line.Len() > 0 && p.lineContent && !p.lineSpace
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

	if isVoidTag(tok.name) {
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
	p.lineContent = true
	p.lineRaw = false
	p.lineSpace = s[len(s)-1] == ' '
}

func (p *htmlPrinter) writeInline(raw string) {
	if p.line.Len() == 0 {
		p.line.WriteString(p.indent())
	}
	p.line.WriteString(raw)
	p.lineContent, p.lineSpace, p.lineRaw = true, false, false
}

// writeInlineRaw 把必须逐字保留的片段紧接在当前行的内容之后（可以跨多行）。
func (p *htmlPrinter) writeInlineRaw(raw string) {
	if p.line.Len() == 0 {
		p.line.WriteString(p.indent())
	}
	p.line.WriteString(raw)
	p.lineContent, p.lineSpace, p.lineRaw = true, false, true
}

// writeOwnLineRaw 让逐字保留的片段独占若干行：首行按当前层级缩进，其余各行
// 连同末尾的闭合标签原样输出，不做任何重新缩进。
func (p *htmlPrinter) writeOwnLineRaw(raw string) {
	p.flushLine()
	p.line.WriteString(p.indent())
	first, rest, multi := strings.Cut(raw, "\n")
	p.line.WriteString(strings.TrimRight(first, "\r"))
	if multi {
		p.line.WriteString("\n")
		p.line.WriteString(rest)
		p.lineRaw = true
	}
	p.flushLine()
}

// writeRawElement 输出原始文本元素与 white-space:pre* 元素。
//   - script/style 尝试格式化内联 CSS/JS，失败或类型未知时退回逐字保留；
//   - pre 沿用既有布局：内部各行原样保留，只把独占一行的闭合标签对齐到当前层级；
//   - 其余元素完全逐字保留：重新缩进 textarea/svg/math 的闭合标签会把空白写进
//     表单值或 SVG 文本节点，xmp/listing/plaintext 与 pre* 元素同理；
//   - 块级元素独占一行。行内级元素（svg/math/textarea 与行内标签上的 pre* 样式）
//     默认是 inline/inline-block，在它和相邻行内内容之间插入换行会多渲染出一个
//     空格（图标与文字之间出现间隙），所以只有源码本身有空白、或两侧都是块级
//     边界时才断开，否则留在行内。
func (p *htmlPrinter) writeRawElement(raw, name string, inlineAfter bool) {
	if name == "pre" || name == "script" || name == "style" {
		p.writeAlignedRaw(raw, name)
		return
	}
	if !isRawTextTag(name) {
		// white-space:pre* 元素：与普通开始标签一样处理可省略的结束标签，
		// 否则前面未闭合的 <p>/<li> 会让后续内容的缩进多出一层。
		p.autoCloseBeforeStart(name)
	}
	if !isRawBlockName(name) && !isBlockTag(name) && (p.glued() || inlineAfter) {
		p.writeInlineRaw(raw)
		return
	}
	p.writeOwnLineRaw(raw)
}

func (p *htmlPrinter) writeAlignedRaw(raw, name string) {
	p.flushLine()
	if p.writeEmbedded(raw, name) {
		return
	}

	first, rest, multi := strings.Cut(raw, "\n")
	p.out.WriteString(p.indent())
	p.out.WriteString(strings.TrimRight(first, "\r"))
	p.out.WriteByte('\n')
	if !multi {
		p.rawTail = false
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
	p.rawTail = false
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
	p.rawTail = false
	return true
}

// splitRawElement 拆分 <tag ...>content</tag>，保留标签原文。
func splitRawElement(raw, name string) (start, content, end string, ok bool) {
	startEnd := scanTagEnd(raw, 1)
	if startEnd < 0 {
		return "", "", "", false
	}
	closeIdx := lastCloseTag(raw, name)
	if closeIdx < startEnd || !strings.HasSuffix(strings.TrimSpace(raw[closeIdx:]), ">") {
		return "", "", "", false
	}
	return raw[:startEnd], raw[startEnd:closeIdx], raw[closeIdx:], true
}

// lastCloseTag 返回 raw 中最后一个 "</name" 的下标（标签名大小写不敏感），没有返回 -1。
// 不能拿 strings.ToLower 结果里的下标去切原字符串：无效的 UTF-8 字节会被替换成
// U+FFFD，长度随之改变，下标就会越界。
func lastCloseTag(raw, name string) int {
	for i := len(raw); i >= 2; {
		j := strings.LastIndex(raw[:i], "</")
		if j < 0 || j+2+len(name) > len(raw) {
			break
		}
		if strings.EqualFold(raw[j+2:j+2+len(name)], name) {
			return j
		}
		i = j
	}
	return -1
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

// hasHTMLSpace 报告 s 是否含有 HTML 空白；没有时折叠/规范化可以直接跳过，不必分配。
func hasHTMLSpace(s string) bool {
	for i := 0; i < len(s); i++ {
		if isHTMLSpace(s[i]) {
			return true
		}
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

// writeVerbatim 输出独占一行的注释/声明。首行缩进，后续行逐字保留；
// closeMarker 非空且独占末行时，把结束标记对齐到当前层级。
func (p *htmlPrinter) writeVerbatim(raw, closeMarker string) {
	p.flushLine()

	first, rest, multi := strings.Cut(raw, "\n")
	p.out.WriteString(p.indent())
	p.out.WriteString(strings.TrimRight(first, "\r"))
	p.out.WriteByte('\n')
	if !multi {
		p.rawTail = false
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
	p.rawTail = false
}

func (p *htmlPrinter) flushLine() {
	s := p.line.String()
	if p.lineRaw {
		// 逐字保留的内容自己以换行结尾（未闭合的 textarea/pre 一直到文档末尾）：
		// 不能再补一个换行，文档末尾的换行裁剪也必须跳过，否则会改动它的值。
		p.rawTail = strings.HasSuffix(s, "\n")
	} else {
		s = strings.TrimRight(s, " \t")
		p.rawTail = false
	}
	// 这里不能用 TrimSpace 判断空行：它会把只含 NBSP 的行当成空行丢掉。
	if s != "" {
		p.out.WriteString(s)
		if !p.rawTail {
			p.out.WriteByte('\n')
		}
	}
	p.line.Reset()
	p.lineContent, p.lineSpace, p.lineRaw = false, false, false
}

func (p *htmlPrinter) indent() string {
	return indentPrefix(htmlIndent, len(p.stack))
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
// 引号状态的判定与 scanTagEnd 共用同一套规则，否则无引号属性值里的撇号
// 会让后面的引号值被误当成标签结构。
func normalizeTag(raw string) string {
	if !hasHTMLSpace(raw) {
		return raw
	}
	var b strings.Builder
	b.Grow(len(raw))

	var s tagScanner
	space := false
	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if s.state != tagInQuoted && isHTMLSpace(ch) {
			s.step(ch)
			space = true
			continue
		}
		s.step(ch)
		if space && b.Len() > 0 && ch != '>' && !(ch == '/' && closesTag(raw, i)) {
			b.WriteByte(' ')
		}
		space = false
		b.WriteByte(ch)
	}
	return strings.TrimSpace(b.String())
}

// closesTag 报告 i 处的 '/' 之后是否只剩空白与 '>'（自闭合写法结尾的斜杠）。
// "/>" 与 "/ >" 一视同仁：否则 " / >" 第一遍折叠成 " />"、第二遍才变成 "/>"，
// 输出不是定点。
func closesTag(raw string, i int) bool {
	for j := i + 1; j < len(raw); j++ {
		switch raw[j] {
		case '>':
			return true
		case ' ', '\t', '\r', '\n', '\f':
		default:
			return false
		}
	}
	return false
}

// collapseWhitespace 把 HTML 空白（TAB/LF/FF/CR/空格）折叠为单个空格。
// NBSP 等非 ASCII 空白是可见内容，原样保留：它们既不参与折叠，也不会在行首被裁掉。
// 保留首尾空格，让行内元素之间的空白（<b>a</b> <i>b</i>）不被吞掉；
// 行首的多余缩进由 writeText 在空行场景裁剪。
func collapseWhitespace(s string) string {
	if !hasHTMLSpace(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	space := false
	for i := 0; i < len(s); {
		if isHTMLSpace(s[i]) {
			space = true
			i++
			continue
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		j := i
		for j < len(s) && !isHTMLSpace(s[j]) {
			j++
		}
		b.WriteString(s[i:j])
		i = j
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
