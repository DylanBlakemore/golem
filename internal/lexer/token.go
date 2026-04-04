package lexer

import (
	"fmt"

	"github.com/dylanblakemore/golem/internal/span"
)

// TokenKind represents the type of a lexical token.
type TokenKind int

const (
	// Special tokens
	ERROR TokenKind = iota
	EOF
	NEWLINE
	COMMENT

	// Literals
	INT_LIT
	FLOAT_LIT
	STRING_LIT
	BOOL_LIT

	// Identifiers
	IDENT       // lowercase identifier
	UPPER_IDENT // uppercase identifier (types, constructors)

	// Keywords
	FN
	PUB
	PRIV
	LET
	MATCH
	DO
	END
	IF
	ELSE
	TYPE
	IMPORT
	TEST
	ASSERT
	GO
	CHAN
	RETURN

	// Operators
	PLUS       // +
	MINUS      // -
	STAR       // *
	SLASH      // /
	PERCENT    // %
	EQ         // ==
	NEQ        // !=
	LT         // <
	GT         // >
	LTE        // <=
	GTE        // >=
	AND        // &&
	OR         // ||
	BANG       // !
	PIPE       // |>
	ARROW      // ->
	FAT_ARROW  // =>
	QUESTION   // ?
	CONCAT     // <>
	CHAN_SEND  // <-
	ASSIGN     // =
	COLON      // :
	PIPE_CHAR  // |
	DOT        // .
	DOUBLE_DOT // ..

	// Delimiters
	LPAREN   // (
	RPAREN   // )
	LBRACE   // {
	RBRACE   // }
	LBRACKET // [
	RBRACKET // ]
	COMMA    // ,

	// String interpolation
	HASH_LBRACE // #{
)

var tokenNames = map[TokenKind]string{
	ERROR:       "ERROR",
	EOF:         "EOF",
	NEWLINE:     "NEWLINE",
	COMMENT:     "COMMENT",
	INT_LIT:     "INT_LIT",
	FLOAT_LIT:   "FLOAT_LIT",
	STRING_LIT:  "STRING_LIT",
	BOOL_LIT:    "BOOL_LIT",
	IDENT:       "IDENT",
	UPPER_IDENT: "UPPER_IDENT",
	FN:          "FN",
	PUB:         "PUB",
	PRIV:        "PRIV",
	LET:         "LET",
	MATCH:       "MATCH",
	DO:          "DO",
	END:         "END",
	IF:          "IF",
	ELSE:        "ELSE",
	TYPE:        "TYPE",
	IMPORT:      "IMPORT",
	TEST:        "TEST",
	ASSERT:      "ASSERT",
	GO:          "GO",
	CHAN:        "CHAN",
	RETURN:      "RETURN",
	PLUS:        "PLUS",
	MINUS:       "MINUS",
	STAR:        "STAR",
	SLASH:       "SLASH",
	PERCENT:     "PERCENT",
	EQ:          "EQ",
	NEQ:         "NEQ",
	LT:          "LT",
	GT:          "GT",
	LTE:         "LTE",
	GTE:         "GTE",
	AND:         "AND",
	OR:          "OR",
	BANG:        "BANG",
	PIPE:        "PIPE",
	ARROW:       "ARROW",
	FAT_ARROW:   "FAT_ARROW",
	QUESTION:    "QUESTION",
	CONCAT:      "CONCAT",
	CHAN_SEND:   "CHAN_SEND",
	ASSIGN:      "ASSIGN",
	COLON:       "COLON",
	PIPE_CHAR:   "PIPE_CHAR",
	DOT:         "DOT",
	DOUBLE_DOT:  "DOUBLE_DOT",
	LPAREN:      "LPAREN",
	RPAREN:      "RPAREN",
	LBRACE:      "LBRACE",
	RBRACE:      "RBRACE",
	LBRACKET:    "LBRACKET",
	RBRACKET:    "RBRACKET",
	COMMA:       "COMMA",
	HASH_LBRACE: "HASH_LBRACE",
}

func (k TokenKind) String() string {
	if name, ok := tokenNames[k]; ok {
		return name
	}
	return fmt.Sprintf("TokenKind(%d)", int(k))
}

// keywords maps keyword strings to their token kinds.
var keywords = map[string]TokenKind{
	"fn":     FN,
	"pub":    PUB,
	"priv":   PRIV,
	"let":    LET,
	"match":  MATCH,
	"do":     DO,
	"end":    END,
	"if":     IF,
	"else":   ELSE,
	"type":   TYPE,
	"import": IMPORT,
	"test":   TEST,
	"assert": ASSERT,
	"go":     GO,
	"chan":   CHAN,
	"return": RETURN,
	"true":   BOOL_LIT,
	"false":  BOOL_LIT,
}

// Token represents a lexical token with its kind, value, and source location.
type Token struct {
	Kind    TokenKind
	Literal string // the raw text of the token
	Span    span.Span
}

// String returns a human-readable representation of the token.
func (t Token) String() string {
	if t.Literal != "" {
		return fmt.Sprintf("%s(%q)", t.Kind, t.Literal)
	}
	return t.Kind.String()
}
