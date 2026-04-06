// Package ast defines the abstract syntax tree node types for Golem.
package ast

import (
	"github.com/dylanblakemore/golem/internal/span"
)

// Module represents a parsed Golem source file.
type Module struct {
	File    string
	Imports []*ImportDecl
	Decls   []Decl
}

// Visibility represents the visibility of a declaration.
type Visibility int

const (
	VisDefault Visibility = iota
	VisPub
	VisPriv
)

// --- Declarations ---

// Decl is the interface implemented by all declaration nodes.
type Decl interface {
	declNode()
	GetSpan() span.Span
}

// FnDecl represents a function declaration.
type FnDecl struct {
	Span       span.Span
	Visibility Visibility
	Name       string
	TypeParams []string // generic type parameters, e.g. <A, B>
	Params     []*Param
	ReturnType TypeExpr // nil if omitted
	Body       []Expr
}

func (*FnDecl) declNode()            {}
func (d *FnDecl) GetSpan() span.Span { return d.Span }

// TypeDecl represents a type declaration (product types for Phase 0).
type TypeDecl struct {
	Span       span.Span
	Visibility Visibility
	Name       string
	TypeParams []string
	Body       TypeBody
}

func (*TypeDecl) declNode()            {}
func (d *TypeDecl) GetSpan() span.Span { return d.Span }

// TypeBody is the interface for type declaration bodies.
type TypeBody interface {
	typeBodyNode()
}

// RecordTypeBody represents a record/product type body: { x: Int, y: Int }
type RecordTypeBody struct {
	Span   span.Span
	Fields []*FieldDef
}

func (*RecordTypeBody) typeBodyNode() {}

// SumTypeBody represents a sum type (algebraic data type) body:
//
//	| Circle { radius: Float }
//	| Rectangle { width: Float, height: Float }
type SumTypeBody struct {
	Span     span.Span
	Variants []*Variant
}

func (*SumTypeBody) typeBodyNode() {}

// Variant represents a single variant of a sum type.
type Variant struct {
	Span   span.Span
	Name   string
	Fields []*FieldDef // nil or empty for unit variants (e.g., | None)
}

// FieldDef represents a field in a record type.
type FieldDef struct {
	Span span.Span
	Name string
	Type TypeExpr
}

// ImportDecl represents an import declaration.
type ImportDecl struct {
	Span span.Span
	Path string // the import path string, e.g. "net/http"
}

func (*ImportDecl) declNode()            {}
func (d *ImportDecl) GetSpan() span.Span { return d.Span }

// LetDecl represents a top-level let binding (also used as an expression-level statement).
type LetDecl struct {
	Span     span.Span
	Name     string
	TypeAnno TypeExpr // nil if omitted
	Value    Expr
}

func (*LetDecl) declNode()            {}
func (d *LetDecl) GetSpan() span.Span { return d.Span }

// Param represents a function parameter.
type Param struct {
	Span span.Span
	Name string
	Type TypeExpr
}

// --- Type Expressions (syntax nodes, not semantic types) ---

// TypeExpr is the interface for type syntax nodes.
type TypeExpr interface {
	typeExprNode()
	GetSpan() span.Span
}

// NamedType represents a simple named type like Int, String, Bool.
type NamedType struct {
	Span span.Span
	Name string
}

func (*NamedType) typeExprNode()        {}
func (t *NamedType) GetSpan() span.Span { return t.Span }

// QualifiedType represents a qualified type like http.ResponseWriter.
type QualifiedType struct {
	Span      span.Span
	Qualifier string
	Name      string
}

func (*QualifiedType) typeExprNode()        {}
func (t *QualifiedType) GetSpan() span.Span { return t.Span }

// GenericType represents a parameterized type like List<Int> or Result<T, E>.
type GenericType struct {
	Span     span.Span
	Name     string
	TypeArgs []TypeExpr
}

func (*GenericType) typeExprNode()        {}
func (t *GenericType) GetSpan() span.Span { return t.Span }

// PointerType represents a pointer type like *http.Request.
type PointerType struct {
	Span span.Span
	Elem TypeExpr
}

func (*PointerType) typeExprNode()        {}
func (t *PointerType) GetSpan() span.Span { return t.Span }

// FnType represents a function type like Fn(Int, Int): Int.
type FnType struct {
	Span       span.Span
	ParamTypes []TypeExpr
	ReturnType TypeExpr
}

func (*FnType) typeExprNode()        {}
func (t *FnType) GetSpan() span.Span { return t.Span }

// --- Expressions ---

// Expr is the interface implemented by all expression nodes.
type Expr interface {
	exprNode()
	GetSpan() span.Span
}

// IntLit represents an integer literal.
type IntLit struct {
	Span  span.Span
	Value string // raw literal, e.g. "42" or "1_000"
}

func (*IntLit) exprNode()            {}
func (e *IntLit) GetSpan() span.Span { return e.Span }

// FloatLit represents a float literal.
type FloatLit struct {
	Span  span.Span
	Value string
}

func (*FloatLit) exprNode()            {}
func (e *FloatLit) GetSpan() span.Span { return e.Span }

// StringLit represents a string literal.
type StringLit struct {
	Span  span.Span
	Value string
}

func (*StringLit) exprNode()            {}
func (e *StringLit) GetSpan() span.Span { return e.Span }

// BoolLit represents a boolean literal.
type BoolLit struct {
	Span  span.Span
	Value bool
}

func (*BoolLit) exprNode()            {}
func (e *BoolLit) GetSpan() span.Span { return e.Span }

// Ident represents an identifier reference.
type Ident struct {
	Span span.Span
	Name string
}

func (*Ident) exprNode()            {}
func (e *Ident) GetSpan() span.Span { return e.Span }

// BinaryExpr represents a binary operation.
type BinaryExpr struct {
	Span  span.Span
	Op    BinaryOp
	Left  Expr
	Right Expr
}

func (*BinaryExpr) exprNode()            {}
func (e *BinaryExpr) GetSpan() span.Span { return e.Span }

// BinaryOp represents a binary operator kind.
type BinaryOp int

const (
	OpAdd BinaryOp = iota
	OpSub
	OpMul
	OpDiv
	OpMod
	OpEq
	OpNeq
	OpLt
	OpGt
	OpLte
	OpGte
	OpAnd
	OpOr
	OpConcat
	OpPipe
)

// UnaryExpr represents a unary operation.
type UnaryExpr struct {
	Span    span.Span
	Op      UnaryOp
	Operand Expr
}

func (*UnaryExpr) exprNode()            {}
func (e *UnaryExpr) GetSpan() span.Span { return e.Span }

// UnaryOp represents a unary operator kind.
type UnaryOp int

const (
	OpNeg UnaryOp = iota
	OpNot
)

// CallExpr represents a function call.
type CallExpr struct {
	Span span.Span
	Func Expr
	Args []Expr
}

func (*CallExpr) exprNode()            {}
func (e *CallExpr) GetSpan() span.Span { return e.Span }

// FieldAccessExpr represents field access: expr.field
type FieldAccessExpr struct {
	Span  span.Span
	Expr  Expr
	Field string
}

func (*FieldAccessExpr) exprNode()            {}
func (e *FieldAccessExpr) GetSpan() span.Span { return e.Span }

// BlockExpr represents a do/end block expression.
type BlockExpr struct {
	Span  span.Span
	Stmts []Expr
}

func (*BlockExpr) exprNode()            {}
func (e *BlockExpr) GetSpan() span.Span { return e.Span }

// IfExpr represents an if/else expression.
type IfExpr struct {
	Span span.Span
	Cond Expr
	Then []Expr
	Else []Expr // nil if no else branch
}

func (*IfExpr) exprNode()            {}
func (e *IfExpr) GetSpan() span.Span { return e.Span }

// StringInterpolation represents a string with interpolated expressions.
type StringInterpolation struct {
	Span  span.Span
	Parts []StringPart
}

func (*StringInterpolation) exprNode()            {}
func (e *StringInterpolation) GetSpan() span.Span { return e.Span }

// StringPart is either a literal segment or an interpolated expression.
type StringPart interface {
	stringPartNode()
}

// StringText is a literal text segment in an interpolated string.
type StringText struct {
	Span  span.Span
	Value string
}

func (*StringText) stringPartNode() {}

// StringInterpExpr is an interpolated expression #{...} in a string.
type StringInterpExpr struct {
	Span span.Span
	Expr Expr
}

func (*StringInterpExpr) stringPartNode() {}

// LetExpr represents a let binding within a block.
type LetExpr struct {
	Span     span.Span
	Name     string
	TypeAnno TypeExpr // nil if omitted
	Value    Expr
}

func (*LetExpr) exprNode()            {}
func (e *LetExpr) GetSpan() span.Span { return e.Span }

// ReturnExpr represents a return expression.
type ReturnExpr struct {
	Span  span.Span
	Value Expr // nil if bare return
}

func (*ReturnExpr) exprNode()            {}
func (e *ReturnExpr) GetSpan() span.Span { return e.Span }

// RecordLit represents a record/struct literal: Point { x: 1, y: 2 }
type RecordLit struct {
	Span   span.Span
	Name   string
	Fields []*FieldInit
}

func (*RecordLit) exprNode()            {}
func (e *RecordLit) GetSpan() span.Span { return e.Span }

// FieldInit represents a field initialization in a record literal.
type FieldInit struct {
	Span  span.Span
	Name  string
	Value Expr
}

// FnLit represents an anonymous function literal.
type FnLit struct {
	Span       span.Span
	Params     []*Param
	ReturnType TypeExpr // nil if omitted
	Body       []Expr
}

func (*FnLit) exprNode()            {}
func (e *FnLit) GetSpan() span.Span { return e.Span }

// NilLit represents the nil literal.
type NilLit struct {
	Span span.Span
}

func (*NilLit) exprNode()            {}
func (e *NilLit) GetSpan() span.Span { return e.Span }

// MatchExpr represents a match expression:
//
//	match expr do
//	  | Pattern -> body
//	end
type MatchExpr struct {
	Span      span.Span
	Scrutinee Expr
	Arms      []*MatchArm
}

func (*MatchExpr) exprNode()            {}
func (e *MatchExpr) GetSpan() span.Span { return e.Span }

// MatchArm represents a single arm in a match expression.
type MatchArm struct {
	Span    span.Span
	Pattern Pattern
	Body    []Expr
}

// --- Patterns ---

// Pattern is the interface implemented by all pattern nodes.
type Pattern interface {
	patternNode()
	GetSpan() span.Span
}

// ConstructorPattern matches a sum type variant: Circle { radius }
type ConstructorPattern struct {
	Span        span.Span
	Constructor string
	Fields      []*FieldPattern
}

func (*ConstructorPattern) patternNode()         {}
func (p *ConstructorPattern) GetSpan() span.Span { return p.Span }

// FieldPattern represents a field binding in a constructor pattern.
type FieldPattern struct {
	Span    span.Span
	Name    string
	Pattern Pattern // nil means bind to same-name variable
}

// VarPattern matches anything and binds to a variable.
type VarPattern struct {
	Span span.Span
	Name string
}

func (*VarPattern) patternNode()         {}
func (p *VarPattern) GetSpan() span.Span { return p.Span }

// WildcardPattern matches anything without binding: _
type WildcardPattern struct {
	Span span.Span
}

func (*WildcardPattern) patternNode()         {}
func (p *WildcardPattern) GetSpan() span.Span { return p.Span }

// LiteralPattern matches a literal value.
type LiteralPattern struct {
	Span  span.Span
	Value Expr // IntLit, FloatLit, StringLit, BoolLit
}

func (*LiteralPattern) patternNode()         {}
func (p *LiteralPattern) GetSpan() span.Span { return p.Span }

// BadExpr represents a parse error placeholder.
type BadExpr struct {
	Span span.Span
}

func (*BadExpr) exprNode()            {}
func (e *BadExpr) GetSpan() span.Span { return e.Span }
