package lexer

import (
	"testing"
)

// helper to collect non-trivial tokens (skip NEWLINE, COMMENT, EOF)
func significantTokens(source string) []Token {
	l := New(source, "test.golem")
	tokens := l.Tokenize()
	var result []Token
	for _, t := range tokens {
		if t.Kind != NEWLINE && t.Kind != COMMENT && t.Kind != EOF {
			result = append(result, t)
		}
	}
	return result
}

func TestKeywords(t *testing.T) {
	tests := []struct {
		input string
		kind  TokenKind
	}{
		{"fn", FN},
		{"pub", PUB},
		{"priv", PRIV},
		{"let", LET},
		{"match", MATCH},
		{"do", DO},
		{"end", END},
		{"if", IF},
		{"else", ELSE},
		{"type", TYPE},
		{"import", IMPORT},
		{"test", TEST},
		{"assert", ASSERT},
		{"go", GO},
		{"chan", CHAN},
		{"return", RETURN},
	}
	for _, tt := range tests {
		tokens := significantTokens(tt.input)
		if len(tokens) != 1 {
			t.Errorf("keyword %q: expected 1 token, got %d", tt.input, len(tokens))
			continue
		}
		if tokens[0].Kind != tt.kind {
			t.Errorf("keyword %q: expected %s, got %s", tt.input, tt.kind, tokens[0].Kind)
		}
		if tokens[0].Literal != tt.input {
			t.Errorf("keyword %q: expected literal %q, got %q", tt.input, tt.input, tokens[0].Literal)
		}
	}
}

func TestBooleanLiterals(t *testing.T) {
	tokens := significantTokens("true false")
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
	for i, tok := range tokens {
		if tok.Kind != BOOL_LIT {
			t.Errorf("token %d: expected BOOL_LIT, got %s", i, tok.Kind)
		}
	}
	if tokens[0].Literal != "true" {
		t.Errorf("expected literal 'true', got %q", tokens[0].Literal)
	}
	if tokens[1].Literal != "false" {
		t.Errorf("expected literal 'false', got %q", tokens[1].Literal)
	}
}

func TestIntegerLiterals(t *testing.T) {
	tests := []struct {
		input   string
		literal string
	}{
		{"0", "0"},
		{"42", "42"},
		{"1_000_000", "1_000_000"},
	}
	for _, tt := range tests {
		tokens := significantTokens(tt.input)
		if len(tokens) != 1 {
			t.Errorf("int %q: expected 1 token, got %d", tt.input, len(tokens))
			continue
		}
		if tokens[0].Kind != INT_LIT {
			t.Errorf("int %q: expected INT_LIT, got %s", tt.input, tokens[0].Kind)
		}
		if tokens[0].Literal != tt.literal {
			t.Errorf("int %q: expected literal %q, got %q", tt.input, tt.literal, tokens[0].Literal)
		}
	}
}

func TestFloatLiterals(t *testing.T) {
	tests := []struct {
		input   string
		literal string
	}{
		{"3.14", "3.14"},
		{"0.5", "0.5"},
		{"1_000.5", "1_000.5"},
	}
	for _, tt := range tests {
		tokens := significantTokens(tt.input)
		if len(tokens) != 1 {
			t.Errorf("float %q: expected 1 token, got %d", tt.input, len(tokens))
			continue
		}
		if tokens[0].Kind != FLOAT_LIT {
			t.Errorf("float %q: expected FLOAT_LIT, got %s", tt.input, tokens[0].Kind)
		}
		if tokens[0].Literal != tt.literal {
			t.Errorf("float %q: expected literal %q, got %q", tt.input, tt.literal, tokens[0].Literal)
		}
	}
}

func TestStringLiterals(t *testing.T) {
	tests := []struct {
		input   string
		literal string
	}{
		{`"hello"`, "hello"},
		{`""`, ""},
		{`"hello world"`, "hello world"},
		{`"escape: \n\t\\\""`, "escape: \n\t\\\""},
		{`"hash: \#"`, "hash: #"},
	}
	for _, tt := range tests {
		tokens := significantTokens(tt.input)
		if len(tokens) != 1 {
			t.Errorf("string %q: expected 1 token, got %d: %v", tt.input, len(tokens), tokens)
			continue
		}
		if tokens[0].Kind != STRING_LIT {
			t.Errorf("string %q: expected STRING_LIT, got %s", tt.input, tokens[0].Kind)
		}
		if tokens[0].Literal != tt.literal {
			t.Errorf("string %q: expected literal %q, got %q", tt.input, tt.literal, tokens[0].Literal)
		}
	}
}

func TestIdentifiers(t *testing.T) {
	tests := []struct {
		input string
		kind  TokenKind
	}{
		{"foo", IDENT},
		{"bar_baz", IDENT},
		{"x1", IDENT},
		{"_private", IDENT},
		{"Point", UPPER_IDENT},
		{"MyType", UPPER_IDENT},
		{"OK", UPPER_IDENT},
	}
	for _, tt := range tests {
		tokens := significantTokens(tt.input)
		if len(tokens) != 1 {
			t.Errorf("ident %q: expected 1 token, got %d", tt.input, len(tokens))
			continue
		}
		if tokens[0].Kind != tt.kind {
			t.Errorf("ident %q: expected %s, got %s", tt.input, tt.kind, tokens[0].Kind)
		}
		if tokens[0].Literal != tt.input {
			t.Errorf("ident %q: expected literal %q, got %q", tt.input, tt.input, tokens[0].Literal)
		}
	}
}

func TestOperators(t *testing.T) {
	tests := []struct {
		input   string
		kind    TokenKind
		literal string
	}{
		{"+", PLUS, "+"},
		{"-", MINUS, "-"},
		{"*", STAR, "*"},
		{"/", SLASH, "/"},
		{"%", PERCENT, "%"},
		{"==", EQ, "=="},
		{"!=", NEQ, "!="},
		{"<", LT, "<"},
		{">", GT, ">"},
		{"<=", LTE, "<="},
		{">=", GTE, ">="},
		{"&&", AND, "&&"},
		{"||", OR, "||"},
		{"!", BANG, "!"},
		{"|>", PIPE, "|>"},
		{"->", ARROW, "->"},
		{"=>", FAT_ARROW, "=>"},
		{"?", QUESTION, "?"},
		{"<>", CONCAT, "<>"},
		{"<-", CHAN_SEND, "<-"},
		{"=", ASSIGN, "="},
		{":", COLON, ":"},
		{"|", PIPE_CHAR, "|"},
		{".", DOT, "."},
		{"..", DOUBLE_DOT, ".."},
	}
	for _, tt := range tests {
		tokens := significantTokens(tt.input)
		if len(tokens) != 1 {
			t.Errorf("op %q: expected 1 token, got %d: %v", tt.input, len(tokens), tokens)
			continue
		}
		if tokens[0].Kind != tt.kind {
			t.Errorf("op %q: expected %s, got %s", tt.input, tt.kind, tokens[0].Kind)
		}
		if tokens[0].Literal != tt.literal {
			t.Errorf("op %q: expected literal %q, got %q", tt.input, tt.literal, tokens[0].Literal)
		}
	}
}

func TestDelimiters(t *testing.T) {
	tests := []struct {
		input string
		kind  TokenKind
	}{
		{"(", LPAREN},
		{")", RPAREN},
		{"{", LBRACE},
		{"}", RBRACE},
		{"[", LBRACKET},
		{"]", RBRACKET},
		{",", COMMA},
	}
	for _, tt := range tests {
		tokens := significantTokens(tt.input)
		if len(tokens) != 1 {
			t.Errorf("delim %q: expected 1 token, got %d", tt.input, len(tokens))
			continue
		}
		if tokens[0].Kind != tt.kind {
			t.Errorf("delim %q: expected %s, got %s", tt.input, tt.kind, tokens[0].Kind)
		}
	}
}

func TestComments(t *testing.T) {
	tokens := New("-- this is a comment\nfoo", "test.golem").Tokenize()
	// Should produce: COMMENT, NEWLINE, IDENT(foo), EOF
	if len(tokens) != 4 {
		t.Fatalf("expected 4 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[0].Kind != COMMENT {
		t.Errorf("expected COMMENT, got %s", tokens[0].Kind)
	}
	if tokens[0].Literal != "-- this is a comment" {
		t.Errorf("expected comment literal, got %q", tokens[0].Literal)
	}
	if tokens[1].Kind != NEWLINE {
		t.Errorf("expected NEWLINE, got %s", tokens[1].Kind)
	}
	if tokens[2].Kind != IDENT {
		t.Errorf("expected IDENT, got %s", tokens[2].Kind)
	}
}

func TestNewlineSignificance(t *testing.T) {
	tokens := New("a\nb", "test.golem").Tokenize()
	// IDENT(a), NEWLINE, IDENT(b), EOF
	if len(tokens) != 4 {
		t.Fatalf("expected 4 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[0].Kind != IDENT {
		t.Errorf("token 0: expected IDENT, got %s", tokens[0].Kind)
	}
	if tokens[1].Kind != NEWLINE {
		t.Errorf("token 1: expected NEWLINE, got %s", tokens[1].Kind)
	}
	if tokens[2].Kind != IDENT {
		t.Errorf("token 2: expected IDENT, got %s", tokens[2].Kind)
	}
}

func TestStringInterpolation(t *testing.T) {
	// "hello #{name}!"
	tokens := significantTokens(`"hello #{name}!"`)
	// STRING_LIT("hello "), HASH_LBRACE, IDENT(name), RBRACE, STRING_LIT("!")
	expected := []struct {
		kind    TokenKind
		literal string
	}{
		{STRING_LIT, "hello "},
		{HASH_LBRACE, "#{"},
		{IDENT, "name"},
		{RBRACE, "}"},
		{STRING_LIT, "!"},
	}
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
	}
	for i, exp := range expected {
		if tokens[i].Kind != exp.kind {
			t.Errorf("token %d: expected %s, got %s", i, exp.kind, tokens[i].Kind)
		}
		if tokens[i].Literal != exp.literal {
			t.Errorf("token %d: expected literal %q, got %q", i, exp.literal, tokens[i].Literal)
		}
	}
}

func TestNestedStringInterpolation(t *testing.T) {
	// "a #{b + c} d"
	tokens := significantTokens(`"a #{b + c} d"`)
	expected := []struct {
		kind    TokenKind
		literal string
	}{
		{STRING_LIT, "a "},
		{HASH_LBRACE, "#{"},
		{IDENT, "b"},
		{PLUS, "+"},
		{IDENT, "c"},
		{RBRACE, "}"},
		{STRING_LIT, " d"},
	}
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
	}
	for i, exp := range expected {
		if tokens[i].Kind != exp.kind {
			t.Errorf("token %d: expected %s, got %s", i, exp.kind, tokens[i].Kind)
		}
		if tokens[i].Literal != exp.literal {
			t.Errorf("token %d: expected literal %q, got %q", i, exp.literal, tokens[i].Literal)
		}
	}
}

func TestMultipleInterpolations(t *testing.T) {
	// "#{a} and #{b}"
	tokens := significantTokens(`"#{a} and #{b}"`)
	expected := []struct {
		kind    TokenKind
		literal string
	}{
		{STRING_LIT, ""},
		{HASH_LBRACE, "#{"},
		{IDENT, "a"},
		{RBRACE, "}"},
		{STRING_LIT, " and "},
		{HASH_LBRACE, "#{"},
		{IDENT, "b"},
		{RBRACE, "}"},
		{STRING_LIT, ""},
	}
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
	}
	for i, exp := range expected {
		if tokens[i].Kind != exp.kind {
			t.Errorf("token %d: expected %s, got %s", i, exp.kind, tokens[i].Kind)
		}
		if tokens[i].Literal != exp.literal {
			t.Errorf("token %d: expected literal %q, got %q", i, exp.literal, tokens[i].Literal)
		}
	}
}

func TestErrorRecovery(t *testing.T) {
	// Unknown character ~ should produce ERROR token and continue
	tokens := significantTokens("a ~ b")
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[0].Kind != IDENT {
		t.Errorf("token 0: expected IDENT, got %s", tokens[0].Kind)
	}
	if tokens[1].Kind != ERROR {
		t.Errorf("token 1: expected ERROR, got %s", tokens[1].Kind)
	}
	if tokens[1].Literal != "~" {
		t.Errorf("token 1: expected literal '~', got %q", tokens[1].Literal)
	}
	if tokens[2].Kind != IDENT {
		t.Errorf("token 2: expected IDENT, got %s", tokens[2].Kind)
	}
}

func TestUnterminatedString(t *testing.T) {
	tokens := significantTokens(`"hello`)
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d: %v", len(tokens), tokens)
	}
	if tokens[0].Kind != ERROR {
		t.Errorf("expected ERROR, got %s", tokens[0].Kind)
	}
}

func TestSpanAccuracy(t *testing.T) {
	l := New("let x = 42", "test.golem")
	tokens := l.Tokenize()
	// LET, IDENT(x), ASSIGN, INT_LIT(42), EOF

	// LET starts at col 1
	if tokens[0].Span.Start.Line != 1 || tokens[0].Span.Start.Column != 1 {
		t.Errorf("LET: expected 1:1, got %d:%d", tokens[0].Span.Start.Line, tokens[0].Span.Start.Column)
	}
	// IDENT(x) starts at col 5
	if tokens[1].Span.Start.Column != 5 {
		t.Errorf("IDENT: expected col 5, got %d", tokens[1].Span.Start.Column)
	}
	// ASSIGN starts at col 7
	if tokens[2].Span.Start.Column != 7 {
		t.Errorf("ASSIGN: expected col 7, got %d", tokens[2].Span.Start.Column)
	}
	// INT_LIT starts at col 9
	if tokens[3].Span.Start.Column != 9 {
		t.Errorf("INT_LIT: expected col 9, got %d", tokens[3].Span.Start.Column)
	}
}

func TestSpanMultiline(t *testing.T) {
	l := New("a\nb\nc", "test.golem")
	tokens := l.Tokenize()
	// IDENT(a) line 1, NEWLINE, IDENT(b) line 2, NEWLINE, IDENT(c) line 3, EOF

	if tokens[0].Span.Start.Line != 1 {
		t.Errorf("a: expected line 1, got %d", tokens[0].Span.Start.Line)
	}
	if tokens[2].Span.Start.Line != 2 {
		t.Errorf("b: expected line 2, got %d", tokens[2].Span.Start.Line)
	}
	if tokens[4].Span.Start.Line != 3 {
		t.Errorf("c: expected line 3, got %d", tokens[4].Span.Start.Line)
	}
}

func TestSnapshotFnDecl(t *testing.T) {
	source := `pub fn add(a: Int, b: Int): Int do
  a + b
end`
	tokens := significantTokens(source)
	expected := []TokenKind{
		PUB, FN, IDENT, LPAREN, IDENT, COLON, UPPER_IDENT, COMMA,
		IDENT, COLON, UPPER_IDENT, RPAREN, COLON, UPPER_IDENT, DO,
		IDENT, PLUS, IDENT,
		END,
	}
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
	}
	for i, exp := range expected {
		if tokens[i].Kind != exp {
			t.Errorf("token %d: expected %s, got %s (%q)", i, exp, tokens[i].Kind, tokens[i].Literal)
		}
	}
}

func TestSnapshotLetBinding(t *testing.T) {
	source := `let name = "world"
let greeting = "Hello, #{name}!"`
	tokens := significantTokens(source)
	expected := []TokenKind{
		LET, IDENT, ASSIGN, STRING_LIT,
		LET, IDENT, ASSIGN, STRING_LIT, HASH_LBRACE, IDENT, RBRACE, STRING_LIT,
	}
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
	}
	for i, exp := range expected {
		if tokens[i].Kind != exp {
			t.Errorf("token %d: expected %s, got %s (%q)", i, exp, tokens[i].Kind, tokens[i].Literal)
		}
	}
}

func TestSnapshotImportAndCall(t *testing.T) {
	source := `import "fmt"
fmt.println("hello")`
	tokens := significantTokens(source)
	expected := []TokenKind{
		IMPORT, STRING_LIT,
		IDENT, DOT, IDENT, LPAREN, STRING_LIT, RPAREN,
	}
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
	}
	for i, exp := range expected {
		if tokens[i].Kind != exp {
			t.Errorf("token %d: expected %s, got %s (%q)", i, exp, tokens[i].Kind, tokens[i].Literal)
		}
	}
}

func TestSnapshotMatchExpr(t *testing.T) {
	source := `match x do
  | Some(v) -> v
  | None -> 0
end`
	tokens := significantTokens(source)
	expected := []TokenKind{
		MATCH, IDENT, DO,
		PIPE_CHAR, UPPER_IDENT, LPAREN, IDENT, RPAREN, ARROW, IDENT,
		PIPE_CHAR, UPPER_IDENT, ARROW, INT_LIT,
		END,
	}
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
	}
	for i, exp := range expected {
		if tokens[i].Kind != exp {
			t.Errorf("token %d: expected %s, got %s (%q)", i, exp, tokens[i].Kind, tokens[i].Literal)
		}
	}
}

func TestSnapshotPipeOperator(t *testing.T) {
	source := `x |> double |> add(1)`
	tokens := significantTokens(source)
	expected := []TokenKind{
		IDENT, PIPE, IDENT, PIPE, IDENT, LPAREN, INT_LIT, RPAREN,
	}
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
	}
	for i, exp := range expected {
		if tokens[i].Kind != exp {
			t.Errorf("token %d: expected %s, got %s (%q)", i, exp, tokens[i].Kind, tokens[i].Literal)
		}
	}
}

func TestSnapshotTypeDecl(t *testing.T) {
	source := `type Point = { x: Float, y: Float }`
	tokens := significantTokens(source)
	expected := []TokenKind{
		TYPE, UPPER_IDENT, ASSIGN, LBRACE,
		IDENT, COLON, UPPER_IDENT, COMMA,
		IDENT, COLON, UPPER_IDENT,
		RBRACE,
	}
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
	}
	for i, exp := range expected {
		if tokens[i].Kind != exp {
			t.Errorf("token %d: expected %s, got %s (%q)", i, exp, tokens[i].Kind, tokens[i].Literal)
		}
	}
}

func TestNumberDoesNotConsumeDoubleDot(t *testing.T) {
	// "1..10" should be INT_LIT(1), DOUBLE_DOT, INT_LIT(10)
	tokens := significantTokens("1..10")
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[0].Kind != INT_LIT || tokens[0].Literal != "1" {
		t.Errorf("token 0: expected INT_LIT(1), got %s(%q)", tokens[0].Kind, tokens[0].Literal)
	}
	if tokens[1].Kind != DOUBLE_DOT {
		t.Errorf("token 1: expected DOUBLE_DOT, got %s", tokens[1].Kind)
	}
	if tokens[2].Kind != INT_LIT || tokens[2].Literal != "10" {
		t.Errorf("token 2: expected INT_LIT(10), got %s(%q)", tokens[2].Kind, tokens[2].Literal)
	}
}

func TestFieldAccessNotFloat(t *testing.T) {
	// "point.x" should be IDENT, DOT, IDENT — not a float
	tokens := significantTokens("point.x")
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[0].Kind != IDENT {
		t.Errorf("token 0: expected IDENT, got %s", tokens[0].Kind)
	}
	if tokens[1].Kind != DOT {
		t.Errorf("token 1: expected DOT, got %s", tokens[1].Kind)
	}
	if tokens[2].Kind != IDENT {
		t.Errorf("token 2: expected IDENT, got %s", tokens[2].Kind)
	}
}

func TestEOF(t *testing.T) {
	tokens := New("", "test.golem").Tokenize()
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Kind != EOF {
		t.Errorf("expected EOF, got %s", tokens[0].Kind)
	}
}

func TestWhitespaceHandling(t *testing.T) {
	// Tabs and spaces are skipped, carriage returns are skipped
	tokens := significantTokens("a \t b")
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[0].Literal != "a" || tokens[1].Literal != "b" {
		t.Errorf("expected 'a' and 'b', got %q and %q", tokens[0].Literal, tokens[1].Literal)
	}
}
