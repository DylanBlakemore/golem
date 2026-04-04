// Package lexer provides the tokenizer for Golem source code.
package lexer

import (
	"unicode"
	"unicode/utf8"

	"github.com/dylanblakemore/golem/internal/span"
)

// lexMode tracks whether the lexer is in normal mode or inside a string.
type lexMode int

const (
	modeNormal lexMode = iota
	modeString         // inside a string, lexing literal characters
)

// Lexer tokenizes Golem source code into a stream of tokens.
type Lexer struct {
	source string
	file   string
	pos    int // current byte position
	line   int // current line (1-based)
	col    int // current column (1-based)

	// String interpolation state: a stack of modes.
	// When we enter #{, we push modeNormal. When we hit } we pop back to modeString.
	modeStack []lexMode

	// pending holds a token that needs to be returned before continuing normal lexing.
	// Used for string interpolation: when we encounter #{, we return the STRING_LIT
	// segment first, then return HASH_LBRACE on the next call.
	pending *Token
}

// New creates a new Lexer for the given source code.
func New(source, file string) *Lexer {
	return &Lexer{
		source:    source,
		file:      file,
		pos:       0,
		line:      1,
		col:       1,
		modeStack: []lexMode{modeNormal},
	}
}

// Tokenize lexes the entire source and returns all tokens including EOF.
func (l *Lexer) Tokenize() []Token {
	var tokens []Token
	for {
		tok := l.Next()
		tokens = append(tokens, tok)
		if tok.Kind == EOF {
			break
		}
	}
	return tokens
}

func (l *Lexer) mode() lexMode {
	return l.modeStack[len(l.modeStack)-1]
}

func (l *Lexer) pushMode(m lexMode) {
	l.modeStack = append(l.modeStack, m)
}

func (l *Lexer) popMode() {
	if len(l.modeStack) > 1 {
		l.modeStack = l.modeStack[:len(l.modeStack)-1]
	}
}

// Next returns the next token from the source.
func (l *Lexer) Next() Token {
	// Return any pending token first (e.g., HASH_LBRACE after string segment)
	if l.pending != nil {
		tok := *l.pending
		l.pending = nil
		return tok
	}

	if l.mode() == modeString {
		return l.lexStringContent()
	}

	l.skipWhitespace()

	// Check for } that closes an interpolation
	if len(l.modeStack) > 1 && !l.atEnd() && l.peek() == '}' {
		// Pop back to string mode — the } closes the interpolation expression
		l.popMode() // back to modeString
		return l.singleChar(RBRACE)
	}

	if l.atEnd() {
		return l.makeToken(EOF, "")
	}

	ch := l.peek()

	// Comments
	if ch == '-' && l.peekNext() == '-' {
		return l.lexComment()
	}

	// String literals
	if ch == '"' {
		return l.lexStringOpen()
	}

	// Numbers
	if isDigit(ch) {
		return l.lexNumber()
	}

	// Identifiers and keywords
	if isIdentStart(ch) {
		return l.lexIdentifier()
	}

	// Newlines
	if ch == '\n' {
		return l.lexNewline()
	}

	// Operators and delimiters
	return l.lexOperator()
}

func (l *Lexer) lexComment() Token {
	start := l.position()
	startPos := l.pos
	l.advance() // first -
	l.advance() // second -
	for !l.atEnd() && l.peek() != '\n' {
		l.advance()
	}
	return Token{
		Kind:    COMMENT,
		Literal: l.source[startPos:l.pos],
		Span:    l.spanFrom(start),
	}
}

// lexStringOpen handles the opening " of a string and enters string mode.
func (l *Lexer) lexStringOpen() Token {
	l.advance() // consume opening "
	l.pushMode(modeString)
	return l.lexStringContent()
}

// lexStringContent lexes inside a string literal, producing STRING_LIT tokens
// for text segments and switching modes for interpolation.
func (l *Lexer) lexStringContent() Token {
	start := l.position()
	var literal []byte

	for !l.atEnd() {
		ch := l.peek()

		if ch == '"' {
			// End of string
			tok := Token{
				Kind:    STRING_LIT,
				Literal: string(literal),
				Span:    l.spanFrom(start),
			}
			l.advance() // consume closing "
			l.popMode() // back to normal/previous mode
			return tok
		}

		if ch == '#' && l.peekNext() == '{' {
			// Emit the string content accumulated so far
			tok := Token{
				Kind:    STRING_LIT,
				Literal: string(literal),
				Span:    l.spanFrom(start),
			}
			// Consume #{
			interpStart := l.position()
			l.advance() // #
			l.advance() // {
			// Push normal mode for the interpolation expression
			l.pushMode(modeNormal)
			// We need to also emit the HASH_LBRACE token. Queue it by
			// storing the string token and returning. Actually, we return
			// the string segment first. The next call to Next() will be in
			// modeNormal and will lex the expression. But we still need
			// to emit HASH_LBRACE.
			// Solution: return the string segment, and prepend HASH_LBRACE
			// to next. Let's use a pending token approach.
			l.pending = &Token{
				Kind:    HASH_LBRACE,
				Literal: "#{",
				Span:    l.spanFrom(interpStart),
			}
			return tok
		}

		if ch == '\\' {
			literal = append(literal, l.lexEscape()...)
			continue
		}

		if ch == '\n' {
			l.line++
			l.col = 1
			literal = append(literal, '\n')
			l.pos++
			continue
		}

		_, size := utf8.DecodeRuneInString(l.source[l.pos:])
		literal = append(literal, l.source[l.pos:l.pos+size]...)
		l.pos += size
		l.col++
	}

	// Unterminated string
	l.popMode()
	return Token{
		Kind:    ERROR,
		Literal: "unterminated string literal",
		Span:    l.spanFrom(start),
	}
}

func (l *Lexer) lexEscape() []byte {
	l.advance() // backslash
	if l.atEnd() {
		return []byte{'\\'}
	}
	ch := l.peek()
	l.advance()
	switch ch {
	case 'n':
		return []byte{'\n'}
	case 't':
		return []byte{'\t'}
	case 'r':
		return []byte{'\r'}
	case '\\':
		return []byte{'\\'}
	case '"':
		return []byte{'"'}
	case '#':
		return []byte{'#'}
	default:
		return []byte{'\\', ch}
	}
}

func (l *Lexer) lexNumber() Token {
	start := l.position()
	startPos := l.pos
	isFloat := false

	for !l.atEnd() && (isDigit(l.peek()) || l.peek() == '_') {
		l.advance()
	}

	if !l.atEnd() && l.peek() == '.' && l.peekNext() != '.' {
		if next := l.peekNext(); isDigit(next) {
			isFloat = true
			l.advance() // consume .
			for !l.atEnd() && (isDigit(l.peek()) || l.peek() == '_') {
				l.advance()
			}
		}
	}

	kind := INT_LIT
	if isFloat {
		kind = FLOAT_LIT
	}
	return Token{
		Kind:    kind,
		Literal: l.source[startPos:l.pos],
		Span:    l.spanFrom(start),
	}
}

func (l *Lexer) lexIdentifier() Token {
	start := l.position()
	startPos := l.pos
	isUpper := unicode.IsUpper(rune(l.peek()))

	for !l.atEnd() && isIdentContinue(l.peek()) {
		l.advance()
	}

	literal := l.source[startPos:l.pos]

	if kind, ok := keywords[literal]; ok {
		return Token{
			Kind:    kind,
			Literal: literal,
			Span:    l.spanFrom(start),
		}
	}

	kind := IDENT
	if isUpper {
		kind = UPPER_IDENT
	}
	return Token{
		Kind:    kind,
		Literal: literal,
		Span:    l.spanFrom(start),
	}
}

func (l *Lexer) lexNewline() Token {
	start := l.position()
	l.pos++
	l.line++
	l.col = 1
	return Token{
		Kind:    NEWLINE,
		Literal: "\n",
		Span:    l.spanFrom(start),
	}
}

func (l *Lexer) lexOperator() Token {
	ch := l.peek()
	next := l.peekNext()

	// Try two-character operators first
	if tok, ok := l.tryTwoCharOp(ch, next); ok {
		return tok
	}

	// Single-character operators and delimiters
	if kind, ok := singleCharOps[ch]; ok {
		return l.singleChar(kind)
	}

	// Unknown character — emit error and continue
	start := l.position()
	r, size := utf8.DecodeRuneInString(l.source[l.pos:])
	l.pos += size
	l.col++
	return Token{
		Kind:    ERROR,
		Literal: string(r),
		Span:    l.spanFrom(start),
	}
}

func (l *Lexer) tryTwoCharOp(ch, next byte) (Token, bool) {
	switch {
	case ch == '|' && next == '>':
		return l.twoChar(PIPE), true
	case ch == '-' && next == '>':
		return l.twoChar(ARROW), true
	case ch == '=' && next == '>':
		return l.twoChar(FAT_ARROW), true
	case ch == '=' && next == '=':
		return l.twoChar(EQ), true
	case ch == '!' && next == '=':
		return l.twoChar(NEQ), true
	case ch == '<' && next == '=':
		return l.twoChar(LTE), true
	case ch == '>' && next == '=':
		return l.twoChar(GTE), true
	case ch == '<' && next == '>':
		return l.twoChar(CONCAT), true
	case ch == '<' && next == '-':
		return l.twoChar(CHAN_SEND), true
	case ch == '&' && next == '&':
		return l.twoChar(AND), true
	case ch == '|' && next == '|':
		return l.twoChar(OR), true
	case ch == '.' && next == '.':
		return l.twoChar(DOUBLE_DOT), true
	default:
		return Token{}, false
	}
}

var singleCharOps = map[byte]TokenKind{
	'+': PLUS,
	'-': MINUS,
	'*': STAR,
	'/': SLASH,
	'%': PERCENT,
	'<': LT,
	'>': GT,
	'!': BANG,
	'?': QUESTION,
	'=': ASSIGN,
	':': COLON,
	'|': PIPE_CHAR,
	'.': DOT,
	'(': LPAREN,
	')': RPAREN,
	'{': LBRACE,
	'}': RBRACE,
	'[': LBRACKET,
	']': RBRACKET,
	',': COMMA,
}

// Helper methods

func (l *Lexer) atEnd() bool {
	return l.pos >= len(l.source)
}

func (l *Lexer) peek() byte {
	if l.atEnd() {
		return 0
	}
	return l.source[l.pos]
}

func (l *Lexer) peekNext() byte {
	pos := l.pos + 1
	if pos >= len(l.source) {
		return 0
	}
	return l.source[pos]
}

func (l *Lexer) advance() {
	if !l.atEnd() {
		if l.source[l.pos] == '\n' {
			l.line++
			l.col = 1
		} else {
			l.col++
		}
		l.pos++
	}
}

func (l *Lexer) skipWhitespace() {
	for !l.atEnd() {
		ch := l.peek()
		if ch == ' ' || ch == '\t' || ch == '\r' {
			l.advance()
		} else {
			break
		}
	}
}

func (l *Lexer) position() span.Position {
	return span.Position{
		Offset: l.pos,
		Line:   l.line,
		Column: l.col,
	}
}

func (l *Lexer) spanFrom(start span.Position) span.Span {
	return span.Span{
		File:  l.file,
		Start: start,
		End:   l.position(),
	}
}

func (l *Lexer) makeToken(kind TokenKind, literal string) Token {
	pos := l.position()
	return Token{
		Kind:    kind,
		Literal: literal,
		Span: span.Span{
			File:  l.file,
			Start: pos,
			End:   pos,
		},
	}
}

func (l *Lexer) singleChar(kind TokenKind) Token {
	start := l.position()
	ch := l.source[l.pos]
	l.advance()
	return Token{
		Kind:    kind,
		Literal: string(ch),
		Span:    l.spanFrom(start),
	}
}

func (l *Lexer) twoChar(kind TokenKind) Token {
	start := l.position()
	literal := l.source[l.pos : l.pos+2]
	l.advance()
	l.advance()
	return Token{
		Kind:    kind,
		Literal: literal,
		Span:    l.spanFrom(start),
	}
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIdentContinue(ch byte) bool {
	return isIdentStart(ch) || isDigit(ch)
}
