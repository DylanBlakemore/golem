// Package parser provides a recursive descent parser for Golem source code.
package parser

import (
	"fmt"

	"github.com/dylanblakemore/golem/internal/ast"
	"github.com/dylanblakemore/golem/internal/lexer"
	"github.com/dylanblakemore/golem/internal/span"
)

// Error represents a parse error with source location.
type Error struct {
	Span    span.Span
	Message string
}

func (e Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Span, e.Message)
}

// Parser is a recursive descent parser for Golem source code.
type Parser struct {
	tokens []lexer.Token
	pos    int
	errors []Error
	file   string
}

// New creates a new Parser for the given token stream.
func New(tokens []lexer.Token, file string) *Parser {
	// Strip comments — they're not needed for parsing.
	filtered := make([]lexer.Token, 0, len(tokens))
	for _, t := range tokens {
		if t.Kind != lexer.COMMENT {
			filtered = append(filtered, t)
		}
	}
	return &Parser{
		tokens: filtered,
		file:   file,
	}
}

// Parse parses a complete Golem module and returns the AST and any errors.
func (p *Parser) Parse() (*ast.Module, []Error) {
	mod := &ast.Module{File: p.file}

	p.skipNewlines()

	// Parse imports first
	for p.check(lexer.IMPORT) {
		imp := p.parseImport()
		if imp != nil {
			mod.Imports = append(mod.Imports, imp)
		}
		p.skipNewlines()
	}

	// Parse declarations
	for !p.atEnd() {
		p.skipNewlines()
		if p.atEnd() {
			break
		}
		decl := p.parseDecl()
		if decl != nil {
			mod.Decls = append(mod.Decls, decl)
		}
		p.skipNewlines()
	}

	return mod, p.errors
}

// Errors returns the collected parse errors.
func (p *Parser) Errors() []Error {
	return p.errors
}

// --- Token helpers ---

func (p *Parser) peek() lexer.Token {
	if p.pos >= len(p.tokens) {
		return lexer.Token{Kind: lexer.EOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) advance() lexer.Token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *Parser) check(kind lexer.TokenKind) bool {
	return p.peek().Kind == kind
}

func (p *Parser) match(kinds ...lexer.TokenKind) bool {
	for _, k := range kinds {
		if p.check(k) {
			p.advance()
			return true
		}
	}
	return false
}

func (p *Parser) expect(kind lexer.TokenKind) (lexer.Token, bool) {
	if p.check(kind) {
		return p.advance(), true
	}
	p.error(fmt.Sprintf("expected %s, got %s", kind, p.peek().Kind))
	return p.peek(), false
}

func (p *Parser) atEnd() bool {
	return p.peek().Kind == lexer.EOF
}

func (p *Parser) skipNewlines() {
	for p.check(lexer.NEWLINE) {
		p.advance()
	}
}

func (p *Parser) error(msg string) {
	p.errors = append(p.errors, Error{
		Span:    p.peek().Span,
		Message: msg,
	})
}

// synchronize advances to the next statement boundary for error recovery.
func (p *Parser) synchronize() {
	for !p.atEnd() {
		if p.check(lexer.NEWLINE) {
			p.advance()
			return
		}
		switch p.peek().Kind {
		case lexer.FN, lexer.PUB, lexer.PRIV, lexer.LET, lexer.TYPE, lexer.IMPORT, lexer.END:
			return
		default:
			p.advance()
		}
	}
}

// --- Declarations ---

func (p *Parser) parseImport() *ast.ImportDecl {
	start := p.peek().Span
	p.advance() // consume IMPORT

	pathTok, ok := p.expect(lexer.STRING_LIT)
	if !ok {
		p.synchronize()
		return nil
	}

	p.expectStatementEnd()

	return &ast.ImportDecl{
		Span: spanFromTo(start, pathTok.Span),
		Path: pathTok.Literal,
	}
}

func (p *Parser) parseDecl() ast.Decl {
	switch p.peek().Kind {
	case lexer.PUB, lexer.PRIV:
		return p.parseVisibleDecl()
	case lexer.FN:
		return p.parseFnDecl(ast.VisDefault)
	case lexer.TYPE:
		return p.parseTypeDecl(ast.VisDefault)
	case lexer.LET:
		return p.parseLetDecl()
	default:
		p.error(fmt.Sprintf("expected declaration, got %s", p.peek().Kind))
		p.synchronize()
		return nil
	}
}

func (p *Parser) parseVisibleDecl() ast.Decl {
	visTok := p.advance()
	vis := ast.VisPub
	if visTok.Kind == lexer.PRIV {
		vis = ast.VisPriv
	}

	switch p.peek().Kind {
	case lexer.FN:
		return p.parseFnDecl(vis)
	case lexer.TYPE:
		return p.parseTypeDecl(vis)
	default:
		p.error(fmt.Sprintf("expected fn or type after %s", visTok.Kind))
		p.synchronize()
		return nil
	}
}

func (p *Parser) parseFnDecl(vis ast.Visibility) *ast.FnDecl {
	start := p.peek().Span
	p.advance() // consume FN

	nameTok, ok := p.expect(lexer.IDENT)
	if !ok {
		p.synchronize()
		return nil
	}

	if _, ok := p.expect(lexer.LPAREN); !ok {
		p.synchronize()
		return nil
	}

	params := p.parseParams()

	if _, ok := p.expect(lexer.RPAREN); !ok {
		p.synchronize()
		return nil
	}

	var retType ast.TypeExpr
	if p.check(lexer.COLON) {
		p.advance()
		retType = p.parseTypeExpr()
	}

	body, endSpan := p.parseBlock()

	return &ast.FnDecl{
		Span:       spanFromTo(start, endSpan),
		Visibility: vis,
		Name:       nameTok.Literal,
		Params:     params,
		ReturnType: retType,
		Body:       body,
	}
}

func (p *Parser) parseParams() []*ast.Param {
	var params []*ast.Param
	if p.check(lexer.RPAREN) {
		return params
	}

	param := p.parseParam()
	if param != nil {
		params = append(params, param)
	}

	for p.check(lexer.COMMA) {
		p.advance()
		param = p.parseParam()
		if param != nil {
			params = append(params, param)
		}
	}

	return params
}

func (p *Parser) parseParam() *ast.Param {
	nameTok, ok := p.expect(lexer.IDENT)
	if !ok {
		return nil
	}

	if _, ok := p.expect(lexer.COLON); !ok {
		return nil
	}

	typeExpr := p.parseTypeExpr()

	return &ast.Param{
		Span: spanFromTo(nameTok.Span, typeExpr.GetSpan()),
		Name: nameTok.Literal,
		Type: typeExpr,
	}
}

func (p *Parser) parseTypeDecl(vis ast.Visibility) *ast.TypeDecl {
	start := p.peek().Span
	p.advance() // consume TYPE

	nameTok, ok := p.expect(lexer.UPPER_IDENT)
	if !ok {
		p.synchronize()
		return nil
	}

	if _, ok := p.expect(lexer.ASSIGN); !ok {
		p.synchronize()
		return nil
	}

	body := p.parseTypeBody()

	var endSpan span.Span
	if body != nil {
		if rb, ok := body.(*ast.RecordTypeBody); ok {
			endSpan = rb.Span
		}
	}
	if endSpan.Start.Line == 0 {
		endSpan = nameTok.Span
	}

	p.expectStatementEnd()

	return &ast.TypeDecl{
		Span:       spanFromTo(start, endSpan),
		Visibility: vis,
		Name:       nameTok.Literal,
		Body:       body,
	}
}

func (p *Parser) parseTypeBody() ast.TypeBody {
	if p.check(lexer.LBRACE) {
		return p.parseRecordTypeBody()
	}
	p.error(fmt.Sprintf("expected type body, got %s", p.peek().Kind))
	p.synchronize()
	return nil
}

func (p *Parser) parseRecordTypeBody() *ast.RecordTypeBody {
	start := p.peek().Span
	p.advance() // consume {
	p.skipNewlines()

	var fields []*ast.FieldDef
	p.parseBraceFields("record type", func(nameTok lexer.Token) {
		typeExpr := p.parseTypeExpr()
		fields = append(fields, &ast.FieldDef{
			Span: spanFromTo(nameTok.Span, typeExpr.GetSpan()),
			Name: nameTok.Literal,
			Type: typeExpr,
		})
	})

	end, _ := p.expect(lexer.RBRACE)

	return &ast.RecordTypeBody{
		Span:   spanFromTo(start, end.Span),
		Fields: fields,
	}
}

func (p *Parser) parseLetDecl() *ast.LetDecl {
	start := p.peek().Span
	p.advance() // consume LET

	nameTok, ok := p.expect(lexer.IDENT)
	if !ok {
		p.synchronize()
		return nil
	}

	var typeAnno ast.TypeExpr
	if p.check(lexer.COLON) {
		p.advance()
		typeAnno = p.parseTypeExpr()
	}

	if _, ok := p.expect(lexer.ASSIGN); !ok {
		p.synchronize()
		return nil
	}

	value := p.parseExpr()

	p.expectStatementEnd()

	return &ast.LetDecl{
		Span:     spanFromTo(start, value.GetSpan()),
		Name:     nameTok.Literal,
		TypeAnno: typeAnno,
		Value:    value,
	}
}

// --- Type expressions ---

func (p *Parser) parseTypeExpr() ast.TypeExpr {
	// Handle Fn(params): ReturnType
	if p.check(lexer.FN) {
		return p.parseFnType()
	}

	tok := p.peek()
	switch tok.Kind {
	case lexer.UPPER_IDENT:
		p.advance()
		name := tok.Literal

		// Check for qualified type: Pkg.Type
		if p.check(lexer.DOT) {
			p.advance()
			memberTok, ok := p.expect(lexer.UPPER_IDENT)
			if !ok {
				return &ast.NamedType{Span: tok.Span, Name: name}
			}
			return &ast.QualifiedType{
				Span:      spanFromTo(tok.Span, memberTok.Span),
				Qualifier: name,
				Name:      memberTok.Literal,
			}
		}

		// Check for generic type: Type<Args>
		if p.check(lexer.LT) {
			return p.parseGenericType(tok, name)
		}

		return &ast.NamedType{Span: tok.Span, Name: name}

	case lexer.IDENT:
		// lowercase types allowed for qualified: http.ResponseWriter
		p.advance()
		if p.check(lexer.DOT) {
			p.advance()
			memberTok, ok := p.expect(lexer.UPPER_IDENT)
			if !ok {
				return &ast.NamedType{Span: tok.Span, Name: tok.Literal}
			}
			return &ast.QualifiedType{
				Span:      spanFromTo(tok.Span, memberTok.Span),
				Qualifier: tok.Literal,
				Name:      memberTok.Literal,
			}
		}
		return &ast.NamedType{Span: tok.Span, Name: tok.Literal}

	default:
		p.error(fmt.Sprintf("expected type, got %s", tok.Kind))
		return &ast.NamedType{Span: tok.Span, Name: "<error>"}
	}
}

func (p *Parser) parseGenericType(nameTok lexer.Token, name string) ast.TypeExpr {
	p.advance() // consume <

	var args []ast.TypeExpr
	if !p.check(lexer.GT) {
		args = append(args, p.parseTypeExpr())
		for p.check(lexer.COMMA) {
			p.advance()
			args = append(args, p.parseTypeExpr())
		}
	}

	end, _ := p.expect(lexer.GT)

	return &ast.GenericType{
		Span:     spanFromTo(nameTok.Span, end.Span),
		Name:     name,
		TypeArgs: args,
	}
}

func (p *Parser) parseFnType() *ast.FnType {
	start := p.peek().Span
	p.advance() // consume FN

	if _, ok := p.expect(lexer.LPAREN); !ok {
		return &ast.FnType{Span: start}
	}

	var paramTypes []ast.TypeExpr
	if !p.check(lexer.RPAREN) {
		paramTypes = append(paramTypes, p.parseTypeExpr())
		for p.check(lexer.COMMA) {
			p.advance()
			paramTypes = append(paramTypes, p.parseTypeExpr())
		}
	}

	p.expect(lexer.RPAREN)

	var retType ast.TypeExpr
	if p.check(lexer.COLON) {
		p.advance()
		retType = p.parseTypeExpr()
	}

	var endSpan span.Span
	if retType != nil {
		endSpan = retType.GetSpan()
	} else {
		endSpan = p.tokens[p.pos-1].Span
	}

	return &ast.FnType{
		Span:       spanFromTo(start, endSpan),
		ParamTypes: paramTypes,
		ReturnType: retType,
	}
}

// --- Block parsing ---

func (p *Parser) parseBlock() ([]ast.Expr, span.Span) {
	if _, ok := p.expect(lexer.DO); !ok {
		p.synchronize()
		return nil, p.peek().Span
	}

	p.skipNewlines()

	var stmts []ast.Expr
	for !p.check(lexer.END) && !p.atEnd() {
		stmt := p.parseStmt()
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
		p.skipNewlines()
	}

	end, _ := p.expect(lexer.END)
	return stmts, end.Span
}

// --- Statement / Expression parsing ---

func (p *Parser) parseStmt() ast.Expr {
	if p.check(lexer.LET) {
		return p.parseLetExpr()
	}
	if p.check(lexer.RETURN) {
		return p.parseReturn()
	}
	expr := p.parseExpr()
	p.expectStatementEnd()
	return expr
}

func (p *Parser) parseLetExpr() *ast.LetExpr {
	start := p.peek().Span
	p.advance() // consume LET

	nameTok, ok := p.expect(lexer.IDENT)
	if !ok {
		p.synchronize()
		return nil
	}

	var typeAnno ast.TypeExpr
	if p.check(lexer.COLON) {
		p.advance()
		typeAnno = p.parseTypeExpr()
	}

	if _, ok := p.expect(lexer.ASSIGN); !ok {
		p.synchronize()
		return nil
	}

	value := p.parseExpr()
	p.expectStatementEnd()

	return &ast.LetExpr{
		Span:     spanFromTo(start, value.GetSpan()),
		Name:     nameTok.Literal,
		TypeAnno: typeAnno,
		Value:    value,
	}
}

func (p *Parser) parseReturn() *ast.ReturnExpr {
	start := p.peek().Span
	p.advance() // consume RETURN

	// Check if there's a value to return
	if p.check(lexer.NEWLINE) || p.check(lexer.END) || p.atEnd() {
		return &ast.ReturnExpr{
			Span: start,
		}
	}

	value := p.parseExpr()
	p.expectStatementEnd()

	return &ast.ReturnExpr{
		Span:  spanFromTo(start, value.GetSpan()),
		Value: value,
	}
}

func (p *Parser) expectStatementEnd() {
	if p.check(lexer.NEWLINE) {
		p.advance()
		return
	}
	// Also OK at EOF, END, RPAREN, RBRACE (context-dependent)
	if p.atEnd() || p.check(lexer.END) || p.check(lexer.RPAREN) || p.check(lexer.RBRACE) {
		return
	}
	// Don't error — the next parse call will catch it
}

// --- Expression parsing (Pratt) ---

// Binding power / precedence levels
const (
	bpNone    = 0
	bpPipe    = 1  // |>
	bpOr      = 2  // ||
	bpAnd     = 3  // &&
	bpEqual   = 4  // == !=
	bpCompare = 5  // < > <= >=
	bpConcat  = 6  // <>
	bpAdd     = 7  // + -
	bpMul     = 8  // * / %
	bpUnary   = 9  // ! -
	bpAccess  = 10 // . ()
)

func (p *Parser) parseExpr() ast.Expr {
	return p.parseExprBP(bpNone)
}

func (p *Parser) parseExprBP(minBP int) ast.Expr {
	left := p.parsePrefixExpr()

	for {
		bp, op, isInfix := p.infixBindingPower()
		if !isInfix || bp <= minBP {
			break
		}

		handled := true
		switch op {
		case lexer.DOT:
			left = p.parseFieldAccess(left)
		case lexer.LPAREN:
			left = p.parseCallExpr(left)
		case lexer.LBRACE:
			// Record literal: Name { fields }
			if ident, ok := left.(*ast.Ident); ok && isUpperFirst(ident.Name) {
				left = p.parseRecordLit(ident)
			} else {
				handled = false
			}
		default:
			p.advance() // consume operator
			right := p.parseExprBP(bp)
			binOp := tokenToBinaryOp(op)
			left = &ast.BinaryExpr{
				Span:  spanFromTo(left.GetSpan(), right.GetSpan()),
				Op:    binOp,
				Left:  left,
				Right: right,
			}
		}
		if !handled {
			break
		}
	}

	return left
}

func (p *Parser) parsePrefixExpr() ast.Expr {
	tok := p.peek()

	switch tok.Kind {
	case lexer.BANG:
		p.advance()
		operand := p.parseExprBP(bpUnary)
		return &ast.UnaryExpr{
			Span:    spanFromTo(tok.Span, operand.GetSpan()),
			Op:      ast.OpNot,
			Operand: operand,
		}
	case lexer.MINUS:
		p.advance()
		operand := p.parseExprBP(bpUnary)
		return &ast.UnaryExpr{
			Span:    spanFromTo(tok.Span, operand.GetSpan()),
			Op:      ast.OpNeg,
			Operand: operand,
		}
	default:
		return p.parsePrimaryExpr()
	}
}

func (p *Parser) parsePrimaryExpr() ast.Expr {
	tok := p.peek()

	switch tok.Kind {
	case lexer.INT_LIT:
		p.advance()
		return &ast.IntLit{Span: tok.Span, Value: tok.Literal}

	case lexer.FLOAT_LIT:
		p.advance()
		return &ast.FloatLit{Span: tok.Span, Value: tok.Literal}

	case lexer.STRING_LIT:
		return p.parseStringExpr()

	case lexer.BOOL_LIT:
		p.advance()
		return &ast.BoolLit{Span: tok.Span, Value: tok.Literal == "true"}

	case lexer.IDENT:
		p.advance()
		// Check for "nil" special identifier
		if tok.Literal == "nil" {
			return &ast.NilLit{Span: tok.Span}
		}
		return &ast.Ident{Span: tok.Span, Name: tok.Literal}

	case lexer.UPPER_IDENT:
		p.advance()
		return &ast.Ident{Span: tok.Span, Name: tok.Literal}

	case lexer.LPAREN:
		return p.parseGroupExpr()

	case lexer.IF:
		return p.parseIfExpr()

	case lexer.DO:
		stmts, endSpan := p.parseBlock()
		return &ast.BlockExpr{
			Span:  spanFromTo(tok.Span, endSpan),
			Stmts: stmts,
		}

	case lexer.FN:
		return p.parseFnLit()

	default:
		p.error(fmt.Sprintf("expected expression, got %s", tok.Kind))
		p.advance() // consume the bad token to make progress
		return &ast.BadExpr{Span: tok.Span}
	}
}

func (p *Parser) parseStringExpr() ast.Expr {
	// Check if this is a simple string or an interpolation.
	// If the next non-string token is HASH_LBRACE, it's an interpolation.
	firstTok := p.advance() // consume STRING_LIT

	if !p.check(lexer.HASH_LBRACE) {
		// Simple string
		return &ast.StringLit{Span: firstTok.Span, Value: firstTok.Literal}
	}

	// String interpolation
	var parts []ast.StringPart
	parts = append(parts, &ast.StringText{Span: firstTok.Span, Value: firstTok.Literal})

	for p.check(lexer.HASH_LBRACE) {
		interpStart := p.peek().Span
		p.advance() // consume #{

		expr := p.parseExpr()

		p.expect(lexer.RBRACE)

		parts = append(parts, &ast.StringInterpExpr{
			Span: spanFromTo(interpStart, expr.GetSpan()),
			Expr: expr,
		})

		// There should be a STRING_LIT segment after the interpolation
		if p.check(lexer.STRING_LIT) {
			strTok := p.advance()
			parts = append(parts, &ast.StringText{Span: strTok.Span, Value: strTok.Literal})
		}
	}

	lastPart := parts[len(parts)-1]
	var endSpan span.Span
	switch lp := lastPart.(type) {
	case *ast.StringText:
		endSpan = lp.Span
	case *ast.StringInterpExpr:
		endSpan = lp.Span
	}

	return &ast.StringInterpolation{
		Span:  spanFromTo(firstTok.Span, endSpan),
		Parts: parts,
	}
}

func (p *Parser) parseGroupExpr() ast.Expr {
	p.advance() // consume (
	p.skipNewlines()
	expr := p.parseExpr()
	p.skipNewlines()
	p.expect(lexer.RPAREN)
	return expr
}

func (p *Parser) parseIfExpr() *ast.IfExpr {
	start := p.peek().Span
	p.advance() // consume IF

	cond := p.parseExpr()

	thenBody, thenEnd := p.parseBlock()

	var elseBody []ast.Expr
	var endSpan span.Span

	if p.check(lexer.ELSE) {
		p.advance()
		if p.check(lexer.IF) {
			// else if -> wrap in a single-element else body
			inner := p.parseIfExpr()
			elseBody = []ast.Expr{inner}
			endSpan = inner.GetSpan()
		} else {
			elseBody, endSpan = p.parseBlock()
		}
	} else {
		endSpan = thenEnd
	}

	return &ast.IfExpr{
		Span: spanFromTo(start, endSpan),
		Cond: cond,
		Then: thenBody,
		Else: elseBody,
	}
}

func (p *Parser) parseFnLit() *ast.FnLit {
	start := p.peek().Span
	p.advance() // consume FN

	if _, ok := p.expect(lexer.LPAREN); !ok {
		return &ast.FnLit{Span: start}
	}

	params := p.parseParams()

	if _, ok := p.expect(lexer.RPAREN); !ok {
		return &ast.FnLit{Span: start, Params: params}
	}

	var retType ast.TypeExpr
	if p.check(lexer.COLON) {
		p.advance()
		retType = p.parseTypeExpr()
	}

	body, endSpan := p.parseBlock()

	return &ast.FnLit{
		Span:       spanFromTo(start, endSpan),
		Params:     params,
		ReturnType: retType,
		Body:       body,
	}
}

func (p *Parser) parseFieldAccess(left ast.Expr) ast.Expr {
	p.advance() // consume .
	fieldTok, ok := p.expect(lexer.IDENT)
	if !ok {
		// Also allow upper ident for qualified access
		if p.check(lexer.UPPER_IDENT) {
			fieldTok = p.advance()
		} else {
			return &ast.BadExpr{Span: left.GetSpan()}
		}
	}
	return &ast.FieldAccessExpr{
		Span:  spanFromTo(left.GetSpan(), fieldTok.Span),
		Expr:  left,
		Field: fieldTok.Literal,
	}
}

func (p *Parser) parseCallExpr(fn ast.Expr) ast.Expr {
	p.advance() // consume (
	p.skipNewlines()

	var args []ast.Expr
	if !p.check(lexer.RPAREN) {
		args = append(args, p.parseExpr())
		for p.check(lexer.COMMA) {
			p.advance()
			p.skipNewlines()
			args = append(args, p.parseExpr())
		}
	}
	p.skipNewlines()

	end, _ := p.expect(lexer.RPAREN)

	return &ast.CallExpr{
		Span: spanFromTo(fn.GetSpan(), end.Span),
		Func: fn,
		Args: args,
	}
}

func (p *Parser) parseRecordLit(name *ast.Ident) *ast.RecordLit {
	p.advance() // consume {
	p.skipNewlines()

	var fields []*ast.FieldInit
	p.parseBraceFields("record literal", func(fieldName lexer.Token) {
		value := p.parseExpr()
		fields = append(fields, &ast.FieldInit{
			Span:  spanFromTo(fieldName.Span, value.GetSpan()),
			Name:  fieldName.Literal,
			Value: value,
		})
	})

	end, _ := p.expect(lexer.RBRACE)

	return &ast.RecordLit{
		Span:   spanFromTo(name.GetSpan(), end.Span),
		Name:   name.Name,
		Fields: fields,
	}
}

// parseBraceFields parses comma-or-newline-separated "name: ..." fields within braces.
// The caller provides a callback that receives the name token and parses the value after the colon.
func (p *Parser) parseBraceFields(context string, parseField func(nameTok lexer.Token)) {
	for !p.check(lexer.RBRACE) && !p.atEnd() {
		p.skipNewlines()
		if p.check(lexer.RBRACE) {
			break
		}

		nameTok, ok := p.expect(lexer.IDENT)
		if !ok {
			p.synchronize()
			continue
		}
		if _, ok := p.expect(lexer.COLON); !ok {
			p.synchronize()
			continue
		}

		parseField(nameTok)

		if !p.check(lexer.RBRACE) {
			if !p.match(lexer.COMMA) {
				if !p.check(lexer.NEWLINE) && !p.check(lexer.RBRACE) {
					p.error("expected , or } in " + context)
				}
			}
		}
		p.skipNewlines()
	}
}

// infixBindingPower returns the binding power and token kind for the current token
// if it's a valid infix operator.
func (p *Parser) infixBindingPower() (int, lexer.TokenKind, bool) {
	kind := p.peek().Kind
	switch kind {
	case lexer.PIPE:
		return bpPipe, kind, true
	case lexer.OR:
		return bpOr, kind, true
	case lexer.AND:
		return bpAnd, kind, true
	case lexer.EQ, lexer.NEQ:
		return bpEqual, kind, true
	case lexer.LT, lexer.GT, lexer.LTE, lexer.GTE:
		return bpCompare, kind, true
	case lexer.CONCAT:
		return bpConcat, kind, true
	case lexer.PLUS, lexer.MINUS:
		return bpAdd, kind, true
	case lexer.STAR, lexer.SLASH, lexer.PERCENT:
		return bpMul, kind, true
	case lexer.DOT:
		return bpAccess, kind, true
	case lexer.LPAREN:
		return bpAccess, kind, true
	case lexer.LBRACE:
		return bpAccess, kind, true
	default:
		return bpNone, kind, false
	}
}

func tokenToBinaryOp(kind lexer.TokenKind) ast.BinaryOp {
	switch kind {
	case lexer.PLUS:
		return ast.OpAdd
	case lexer.MINUS:
		return ast.OpSub
	case lexer.STAR:
		return ast.OpMul
	case lexer.SLASH:
		return ast.OpDiv
	case lexer.PERCENT:
		return ast.OpMod
	case lexer.EQ:
		return ast.OpEq
	case lexer.NEQ:
		return ast.OpNeq
	case lexer.LT:
		return ast.OpLt
	case lexer.GT:
		return ast.OpGt
	case lexer.LTE:
		return ast.OpLte
	case lexer.GTE:
		return ast.OpGte
	case lexer.AND:
		return ast.OpAnd
	case lexer.OR:
		return ast.OpOr
	case lexer.CONCAT:
		return ast.OpConcat
	case lexer.PIPE:
		return ast.OpPipe
	default:
		return ast.OpAdd // unreachable
	}
}

// --- Helpers ---

func spanFromTo(start, end span.Span) span.Span {
	return span.Span{
		File:  start.File,
		Start: start.Start,
		End:   end.End,
	}
}

func isUpperFirst(s string) bool {
	if len(s) == 0 {
		return false
	}
	return s[0] >= 'A' && s[0] <= 'Z'
}
