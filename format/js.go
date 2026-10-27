package format

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// jsIndent 与 HTML/JSON/CSS 及编辑器 Tab 保持一致。
const jsIndent = "    "

var errInvalidJS = errors.New("JS 结构不完整")

type jsKind uint8

const (
	jsWord jsKind = iota
	jsNumber
	jsString
	jsTemplate
	jsRegex
	jsLineComment
	jsBlockComment
	jsPunct
)

type jsToken struct {
	kind           jsKind
	text           string
	spaceBefore    bool
	newlinesBefore int
	unary          bool // + / - 处于一元上下文
	postfix        bool // ++ / -- 处于后缀上下文
}

// formatJS 只用于 <script> 内联脚本。base 是每行额外前缀缩进。
//
// 该格式化器只调整缩进、语句级换行与安全的分词间距，不删除任何 token、
// 不插入分号、不重排表达式；模板字符串、正则与注释原样保留，
// 因此不会改变 ASI 语义。
func formatJS(src, base string) (string, error) {
	src = strings.TrimPrefix(src, "\uFEFF")
	if strings.TrimSpace(src) == "" {
		return "", ErrEmptyDocument
	}
	if strings.HasPrefix(src, "#!") {
		return "", errInvalidJS
	}

	tokens, err := tokenizeJS(src)
	if err != nil {
		return "", err
	}

	p := &jsPrinter{base: base}
	if err := p.run(tokens); err != nil {
		return "", err
	}
	return strings.TrimRight(p.out.String(), "\n"), nil
}

// ---------------------------------------------------------------------------
// 分词
// ---------------------------------------------------------------------------

func tokenizeJS(src string) ([]jsToken, error) {
	tokens := make([]jsToken, 0, len(src)/8+1)

	var (
		space    bool
		newlines int
		last     jsToken
		hasLast  bool
		regexOK  = true
		parens   []bool // true = if/for/while 等控制条件括号
		braces   []bool // true = 语句块，false = 对象/解构字面量
	)

	emit := func(tok jsToken) {
		tok.spaceBefore = space || newlines > 0
		tok.newlinesBefore = newlines
		if tok.kind == jsPunct {
			switch tok.text {
			case "+", "-":
				tok.unary = jsUnaryContext(last, hasLast)
			case "++", "--":
				tok.postfix = jsPostfixContext(last, hasLast)
			}
		}
		tokens = append(tokens, tok)
		space = false
		newlines = 0

		switch tok.kind {
		case jsWord:
			regexOK = jsPrefixKeyword(tok.text)
		case jsNumber, jsString, jsTemplate, jsRegex:
			regexOK = false
		case jsLineComment, jsBlockComment:
			// 注释不改变语法上下文
		case jsPunct:
			switch tok.text {
			case "(":
				parens = append(parens, jsControlParen(last, hasLast))
				regexOK = true
			case ")":
				control := false
				if len(parens) > 0 {
					control = parens[len(parens)-1]
					parens = parens[:len(parens)-1]
				}
				regexOK = control
			case "[":
				regexOK = true
			case "]":
				regexOK = false
			case "{":
				braces = append(braces, jsBlockBrace(last, hasLast))
				regexOK = true
			case "}":
				block := true
				if len(braces) > 0 {
					block = braces[len(braces)-1]
					braces = braces[:len(braces)-1]
				}
				regexOK = block
			case "++", "--":
				regexOK = false
			default:
				regexOK = true
			}
		}
		last = tok
		hasLast = true
	}

	for i := 0; i < len(src); {
		ch := src[i]
		switch {
		case ch == ' ' || ch == '\t' || ch == '\f' || ch == '\v':
			space = true
			i++
			continue

		case ch == '\r':
			if i+1 < len(src) && src[i+1] == '\n' {
				i++
			}
			newlines++
			space = false
			i++
			continue

		case ch == '\n':
			newlines++
			space = false
			i++
			continue
		}

		// 注释优先于正则/除法判定
		if (i == 0 || newlines > 0) && (strings.HasPrefix(src[i:], "<!--") || strings.HasPrefix(src[i:], "-->")) {
			end := strings.IndexByte(src[i:], '\n')
			if end < 0 {
				end = len(src)
			} else {
				end += i
			}
			emit(jsToken{kind: jsLineComment, text: src[i:end]})
			i = end
			continue
		}
		if strings.HasPrefix(src[i:], "//") {
			end := strings.IndexByte(src[i+2:], '\n')
			if end < 0 {
				end = len(src)
			} else {
				end += i + 2
			}
			emit(jsToken{kind: jsLineComment, text: src[i:end]})
			i = end
			continue
		}
		if strings.HasPrefix(src[i:], "/*") {
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return nil, errInvalidJS
			}
			end += i + 4
			emit(jsToken{kind: jsBlockComment, text: src[i:end]})
			i = end
			continue
		}

		if ch == '"' || ch == '\'' {
			end := scanJSString(src, i)
			if end < 0 {
				return nil, errInvalidJS
			}
			emit(jsToken{kind: jsString, text: src[i:end]})
			i = end
			continue
		}
		if ch == '`' {
			end := scanJSTemplate(src, i)
			if end < 0 {
				return nil, errInvalidJS
			}
			emit(jsToken{kind: jsTemplate, text: src[i:end]})
			i = end
			continue
		}

		if ch == '/' && regexOK {
			if end := scanJSRegex(src, i); end > 0 {
				emit(jsToken{kind: jsRegex, text: src[i:end]})
				i = end
				continue
			}
		}

		if isJSNumberStart(src, i) {
			end := scanJSNumber(src, i)
			emit(jsToken{kind: jsNumber, text: src[i:end]})
			i = end
			continue
		}

		if isJSIdentStart(src, i) {
			end := scanJSIdent(src, i)
			emit(jsToken{kind: jsWord, text: src[i:end]})
			i = end
			continue
		}

		op := matchJSOperator(src[i:])
		if op == "" {
			// 未知字符按单字符 token 保留，绝不丢源码
			op = src[i : i+1]
		}
		emit(jsToken{kind: jsPunct, text: op})
		i += len(op)
	}

	if !hasLast {
		return nil, ErrEmptyDocument
	}
	if len(parens) != 0 || len(braces) != 0 {
		return nil, errInvalidJS
	}
	return tokens, nil
}

func scanJSString(src string, i int) int {
	quote := src[i]
	for j := i + 1; j < len(src); j++ {
		switch src[j] {
		case '\\':
			j++
		case quote:
			return j + 1
		case '\n', '\r':
			return -1
		}
	}
	return -1
}

// scanJSTemplate 扫描模板字符串，完整跳过 ${...} 插值，返回结束反引号之后的下标。
func scanJSTemplate(src string, i int) int {
	i++ // `
	for i < len(src) {
		switch src[i] {
		case '\\':
			i += 2
		case '`':
			return i + 1
		case '$':
			if i+1 < len(src) && src[i+1] == '{' {
				i = scanTemplateExpr(src, i+2)
			} else {
				i++
			}
		default:
			i++
		}
	}
	return -1
}

// scanTemplateExpr 从 ${ 之后开始，跳过字符串/模板/注释/嵌套花括号，返回匹配 } 之后的下标。
func scanTemplateExpr(src string, i int) int {
	depth := 1
	for i < len(src) {
		switch {
		case src[i] == '{':
			depth++
			i++
		case src[i] == '}':
			depth--
			i++
			if depth == 0 {
				return i
			}
		case src[i] == '"' || src[i] == '\'':
			end := scanJSString(src, i)
			if end < 0 {
				return len(src)
			}
			i = end
		case src[i] == '`':
			end := scanJSTemplate(src, i)
			if end < 0 {
				return len(src)
			}
			i = end
		case strings.HasPrefix(src[i:], "//"):
			end := strings.IndexByte(src[i+2:], '\n')
			if end < 0 {
				return len(src)
			}
			i += end + 2
		case strings.HasPrefix(src[i:], "/*"):
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return len(src)
			}
			i += end + 4
		default:
			i++
		}
	}
	return len(src)
}

// scanJSRegex 扫描正则字面量（含字符类与转义）。失败返回 -1，按除法处理。
func scanJSRegex(src string, i int) int {
	i++ // /
	inClass := false
	for i < len(src) {
		switch src[i] {
		case '\\':
			i += 2
			continue
		case '\n', '\r':
			return -1
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '/':
			if !inClass {
				i++
				for i < len(src) && isJSIdentPart(src, i) {
					_, size := utf8.DecodeRuneInString(src[i:])
					i += size
				}
				return i
			}
		}
		i++
	}
	return -1
}

func isJSNumberStart(src string, i int) bool {
	if src[i] >= '0' && src[i] <= '9' {
		return true
	}
	return src[i] == '.' && i+1 < len(src) && src[i+1] >= '0' && src[i+1] <= '9'
}

func scanJSNumber(src string, i int) int {
	j := i
	for j < len(src) {
		ch := src[j]
		switch {
		case ch >= '0' && ch <= '9', ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z',
			ch == '_', ch == '.':
			j++
		case (ch == '+' || ch == '-') && j > i && (src[j-1] == 'e' || src[j-1] == 'E'):
			j++
		default:
			return j
		}
	}
	return j
}

func isJSIdentStart(src string, i int) bool {
	ch := src[i]
	if ch == '$' || ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
		return true
	}
	if ch < utf8.RuneSelf {
		return false
	}
	r, _ := utf8.DecodeRuneInString(src[i:])
	return unicode.IsLetter(r)
}

func isJSIdentPart(src string, i int) bool {
	ch := src[i]
	if ch == '$' || ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') {
		return true
	}
	if ch < utf8.RuneSelf {
		return false
	}
	r, _ := utf8.DecodeRuneInString(src[i:])
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func scanJSIdent(src string, i int) int {
	for i < len(src) && isJSIdentPart(src, i) {
		_, size := utf8.DecodeRuneInString(src[i:])
		i += size
	}
	return i
}

// jsOperators 必须按长度降序排列，保证最长匹配。
var jsOperators = []string{
	">>>=", "===", "!==", "**=", "&&=", "||=", "??=", ">>>", "<<=", ">>=",
	"...", "=>", "==", "!=", "<=", ">=", "&&", "||", "??", "++", "--",
	"+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "**", "<<", ">>", "?.",
	"!", "~", "+", "-", "*", "/", "%", "&", "|", "^", "<", ">", "=", "?",
	":", ".", ",", ";", "(", ")", "[", "]", "{", "}", "@", "#",
}

func matchJSOperator(rest string) string {
	for _, op := range jsOperators {
		if strings.HasPrefix(rest, op) {
			return op
		}
	}
	return ""
}

func jsPrefixKeyword(word string) bool {
	switch word {
	case "return", "throw", "typeof", "void", "delete", "new", "in", "of",
		"instanceof", "do", "else", "case", "yield", "await", "default", "extends":
		return true
	}
	return false
}

func jsControlParen(prev jsToken, hasPrev bool) bool {
	if !hasPrev || prev.kind != jsWord {
		return false
	}
	switch prev.text {
	case "if", "while", "for", "with", "switch", "catch":
		return true
	}
	return false
}

func jsBlockBrace(prev jsToken, hasPrev bool) bool {
	if !hasPrev {
		return true
	}
	switch prev.text {
	case ")", "=>", ";", "{", "}":
		return true
	}
	if prev.kind == jsWord {
		switch prev.text {
		case "else", "try", "finally", "do", "switch":
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 排版
// ---------------------------------------------------------------------------

type jsPrinter struct {
	out     strings.Builder
	line    strings.Builder
	base    string
	depth   int
	paren   int
	bracket int
	braces  []bool
	last    jsToken
	hasLast bool
	bad     bool
}

func (p *jsPrinter) run(tokens []jsToken) error {
	for i, tok := range tokens {
		var next jsToken
		if i+1 < len(tokens) {
			next = tokens[i+1]
		}

		// 语句级换行是 ASI 语义的一部分。即使当前位于调用参数/数组内部，
		// 嵌套函数体里也可能出现 return/break 等受限产生式，因此一律保留；
		// 只有明确的续行标点才允许并入同一行。
		if tok.newlinesBefore > 0 && p.line.Len() > 0 {
			if !jsLineContinuation(tok) && tok.text != "}" {
				p.flush()
			}
		}

		if tok.kind == jsLineComment {
			if p.needsSpace(tok) {
				p.line.WriteByte(' ')
			}
			p.write(tok.text)
			p.flush()
		} else if tok.text == "}" {
			p.closeBrace(next)
		} else {
			if p.needsSpace(tok) {
				p.line.WriteByte(' ')
			}
			p.write(tok.text)
			p.afterToken(tok, next)
		}

		p.last = tok
		p.hasLast = true
	}

	p.flush()
	if p.bad || len(p.braces) != 0 || p.paren != 0 || p.bracket != 0 {
		return errInvalidJS
	}
	return nil
}

func (p *jsPrinter) indent() string {
	return p.base + strings.Repeat(jsIndent, p.depth)
}

func (p *jsPrinter) write(text string) {
	if p.line.Len() == 0 {
		p.line.WriteString(p.indent())
	}
	p.line.WriteString(text)
}

func (p *jsPrinter) flush() {
	if p.line.Len() == 0 {
		return
	}
	text := strings.TrimRight(p.line.String(), " \t")
	if strings.TrimSpace(text) != "" {
		p.out.WriteString(text)
		p.out.WriteByte('\n')
	}
	p.line.Reset()
}

func (p *jsPrinter) afterToken(tok jsToken, next jsToken) {
	switch tok.text {
	case ";":
		if p.paren == 0 && p.bracket == 0 {
			p.flush()
		}
	case ",":
		// 对象字面量/解构的属性逐行排列；函数参数与数组保持单行。
		if p.paren == 0 && p.bracket == 0 && len(p.braces) > 0 && !p.braces[len(p.braces)-1] {
			p.flush()
		}
	case "(":
		p.paren++
	case ")":
		if p.paren == 0 {
			p.bad = true
		} else {
			p.paren--
		}
	case "[":
		p.bracket++
	case "]":
		if p.bracket == 0 {
			p.bad = true
		} else {
			p.bracket--
		}
	case "{":
		p.braces = append(p.braces, jsBlockBrace(p.last, p.hasLast))
		p.depth++
		if next.text != "}" {
			p.flush()
		}
	}
}

func (p *jsPrinter) closeBrace(next jsToken) {
	empty := p.hasLast && p.last.text == "{"
	if !empty && p.line.Len() > 0 {
		p.flush()
	}
	if len(p.braces) == 0 {
		p.bad = true
		return
	}
	p.braces = p.braces[:len(p.braces)-1]
	if p.depth > 0 {
		p.depth--
	}
	p.write("}")
	if !jsBraceContinuation(next) {
		p.flush()
	}
}

func (p *jsPrinter) needsSpace(tok jsToken) bool {
	if !p.hasLast || p.line.Len() == 0 {
		return false
	}
	prev := p.last

	if (tok.text == "." || tok.text == "?.") && tok.spaceBefore && prev.kind == jsNumber {
		// 1 .toString() 的空格是语法要求；去掉会变成非法的 1.toString。
		return true
	}

	switch tok.text {
	case ")", "]", ",", ";", ".", "?.", ":":
		return false
	}

	switch prev.text {
	case "(", "[", ".", "?.", "...", "#", "!":
		return false
	case "~":
		return false
	case "{":
		return false
	case ",":
		return true
	case ":":
		return true
	case ";":
		return true
	}

	if isJSBinaryOperator(prev.text) && !prev.unary {
		return true
	}

	switch tok.text {
	case "(":
		return jsKeywordBeforeParen(prev)
	case "[":
		if prev.kind == jsWord && jsKeywordBeforeParen(prev) {
			return true
		}
		return false
	case "++", "--":
		return !jsPostfixContext(prev, true)
	case "+", "-":
		if tok.unary {
			return jsWordLike(prev)
		}
		return true
	case "!", "~":
		return jsWordLike(prev)
	case "*":
		return true
	case "{", "}":
		return true
	}

	if jsWordLike(prev) || prev.text == ")" || prev.text == "]" || prev.text == "}" {
		return true
	}
	if prev.kind == jsBlockComment {
		return true
	}
	return tok.spaceBefore
}

func jsWordLike(tok jsToken) bool {
	switch tok.kind {
	case jsWord, jsNumber, jsString, jsTemplate, jsRegex:
		return true
	}
	return false
}

func jsKeywordBeforeParen(tok jsToken) bool {
	if tok.kind != jsWord {
		return false
	}
	switch tok.text {
	case "if", "for", "while", "switch", "catch", "with", "function",
		"return", "throw", "typeof", "void", "delete", "new", "await",
		"yield", "case", "in", "of", "instanceof", "do", "else", "extends", "async":
		return true
	}
	return false
}

func jsPostfixContext(prev jsToken, hasPrev bool) bool {
	if !hasPrev {
		return false
	}
	return jsWordLike(prev) || prev.text == ")" || prev.text == "]"
}

func jsUnaryContext(prev jsToken, hasPrev bool) bool {
	if !hasPrev {
		return true
	}
	if prev.kind == jsPunct {
		switch prev.text {
		case ")", "]", "}":
			return false
		case "++", "--":
			return !prev.postfix
		default:
			return true
		}
	}
	if prev.kind == jsWord {
		return jsPrefixKeyword(prev.text)
	}
	return false
}

func isJSBinaryOperator(text string) bool {
	switch text {
	case "=", "+=", "-=", "*=", "/=", "%=", "**=", "<<=", ">>=", ">>>=",
		"&=", "|=", "^=", "&&=", "||=", "??=", "==", "===", "!=", "!==",
		"<", ">", "<=", ">=", "+", "-", "*", "/", "%", "**", "&&", "||",
		"??", "&", "|", "^", "<<", ">>", ">>>", "=>", "?", "in", "instanceof":
		return true
	}
	return false
}

func jsLineContinuation(tok jsToken) bool {
	switch tok.text {
	case ")", "]", ",", ";", ".", "?.", ":", "else", "catch", "finally", "while":
		return true
	}
	return false
}

func jsBraceContinuation(next jsToken) bool {
	switch next.text {
	case ";", ",", ")", "]", ".", "?.", "(", "[", "?", ":", "=>",
		"else", "catch", "finally", "while", "from", "as", "of", "in", "instanceof",
		"=", "+=", "-=", "*=", "/=", "%=", "**=", "<<=", ">>=", ">>>=",
		"&=", "|=", "^=", "&&=", "||=", "??=", "==", "===", "!=", "!==",
		"<", ">", "<=", ">=", "+", "-", "*", "/", "%", "**",
		"&&", "||", "??", "&", "|", "^", "<<", ">>", ">>>":
		return true
	}
	return false
}
