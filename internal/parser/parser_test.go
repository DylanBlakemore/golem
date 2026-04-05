package parser

import (
	"testing"

	"github.com/dylanblakemore/golem/internal/ast"
	"github.com/dylanblakemore/golem/internal/lexer"
)

func parse(source string) (*ast.Module, []Error) {
	l := lexer.New(source, "test.golem")
	tokens := l.Tokenize()
	p := New(tokens, "test.golem")
	return p.Parse()
}

func expectNoErrors(t *testing.T, errors []Error) {
	t.Helper()
	if len(errors) > 0 {
		for _, e := range errors {
			t.Errorf("parse error: %s", e)
		}
		t.FailNow()
	}
}

// --- Import declarations ---

func TestParseImport(t *testing.T) {
	mod, errs := parse(`import "fmt"`)
	expectNoErrors(t, errs)

	if len(mod.Imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(mod.Imports))
	}
	if mod.Imports[0].Path != "fmt" {
		t.Errorf("expected path 'fmt', got %q", mod.Imports[0].Path)
	}
}

func TestParseMultipleImports(t *testing.T) {
	mod, errs := parse(`import "fmt"
import "net/http"`)
	expectNoErrors(t, errs)

	if len(mod.Imports) != 2 {
		t.Fatalf("expected 2 imports, got %d", len(mod.Imports))
	}
	if mod.Imports[0].Path != "fmt" {
		t.Errorf("import 0: expected 'fmt', got %q", mod.Imports[0].Path)
	}
	if mod.Imports[1].Path != "net/http" {
		t.Errorf("import 1: expected 'net/http', got %q", mod.Imports[1].Path)
	}
}

// --- Function declarations ---

func TestParseFnDecl(t *testing.T) {
	mod, errs := parse(`fn greet() do
  42
end`)
	expectNoErrors(t, errs)

	if len(mod.Decls) != 1 {
		t.Fatalf("expected 1 decl, got %d", len(mod.Decls))
	}
	fn, ok := mod.Decls[0].(*ast.FnDecl)
	if !ok {
		t.Fatalf("expected FnDecl, got %T", mod.Decls[0])
	}
	if fn.Name != "greet" {
		t.Errorf("expected name 'greet', got %q", fn.Name)
	}
	if fn.Visibility != ast.VisDefault {
		t.Errorf("expected default visibility")
	}
	if len(fn.Params) != 0 {
		t.Errorf("expected 0 params, got %d", len(fn.Params))
	}
	if fn.ReturnType != nil {
		t.Errorf("expected no return type")
	}
	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 body expr, got %d", len(fn.Body))
	}
}

func TestParseFnWithParams(t *testing.T) {
	mod, errs := parse(`pub fn add(a: Int, b: Int): Int do
  a + b
end`)
	expectNoErrors(t, errs)

	fn := mod.Decls[0].(*ast.FnDecl)
	if fn.Name != "add" {
		t.Errorf("expected name 'add', got %q", fn.Name)
	}
	if fn.Visibility != ast.VisPub {
		t.Errorf("expected pub visibility")
	}
	if len(fn.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(fn.Params))
	}
	if fn.Params[0].Name != "a" {
		t.Errorf("param 0: expected 'a', got %q", fn.Params[0].Name)
	}
	if named, ok := fn.Params[0].Type.(*ast.NamedType); !ok || named.Name != "Int" {
		t.Errorf("param 0: expected type Int")
	}
	if fn.ReturnType == nil {
		t.Fatal("expected return type")
	}
	if named, ok := fn.ReturnType.(*ast.NamedType); !ok || named.Name != "Int" {
		t.Errorf("expected return type Int")
	}
}

func TestParsePrivFn(t *testing.T) {
	mod, errs := parse(`priv fn helper(): Bool do
  true
end`)
	expectNoErrors(t, errs)

	fn := mod.Decls[0].(*ast.FnDecl)
	if fn.Visibility != ast.VisPriv {
		t.Errorf("expected priv visibility")
	}
}

// --- Type declarations ---

func TestParseProductType(t *testing.T) {
	mod, errs := parse(`type Point = { x: Float, y: Float }`)
	expectNoErrors(t, errs)

	if len(mod.Decls) != 1 {
		t.Fatalf("expected 1 decl, got %d", len(mod.Decls))
	}
	td, ok := mod.Decls[0].(*ast.TypeDecl)
	if !ok {
		t.Fatalf("expected TypeDecl, got %T", mod.Decls[0])
	}
	if td.Name != "Point" {
		t.Errorf("expected name 'Point', got %q", td.Name)
	}
	body, ok := td.Body.(*ast.RecordTypeBody)
	if !ok {
		t.Fatalf("expected RecordTypeBody, got %T", td.Body)
	}
	if len(body.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(body.Fields))
	}
	if body.Fields[0].Name != "x" {
		t.Errorf("field 0: expected 'x', got %q", body.Fields[0].Name)
	}
	if body.Fields[1].Name != "y" {
		t.Errorf("field 1: expected 'y', got %q", body.Fields[1].Name)
	}
}

// --- Let declarations ---

func TestParseLetDecl(t *testing.T) {
	mod, errs := parse(`let x = 42`)
	expectNoErrors(t, errs)

	if len(mod.Decls) != 1 {
		t.Fatalf("expected 1 decl, got %d", len(mod.Decls))
	}
	letD, ok := mod.Decls[0].(*ast.LetDecl)
	if !ok {
		t.Fatalf("expected LetDecl, got %T", mod.Decls[0])
	}
	if letD.Name != "x" {
		t.Errorf("expected name 'x', got %q", letD.Name)
	}
	if letD.TypeAnno != nil {
		t.Errorf("expected no type annotation")
	}
	intLit, ok := letD.Value.(*ast.IntLit)
	if !ok {
		t.Fatalf("expected IntLit, got %T", letD.Value)
	}
	if intLit.Value != "42" {
		t.Errorf("expected value '42', got %q", intLit.Value)
	}
}

func TestParseLetDeclWithType(t *testing.T) {
	mod, errs := parse(`let name: String = "hello"`)
	expectNoErrors(t, errs)

	letD := mod.Decls[0].(*ast.LetDecl)
	if letD.TypeAnno == nil {
		t.Fatal("expected type annotation")
	}
	named, ok := letD.TypeAnno.(*ast.NamedType)
	if !ok {
		t.Fatalf("expected NamedType, got %T", letD.TypeAnno)
	}
	if named.Name != "String" {
		t.Errorf("expected type String, got %q", named.Name)
	}
}

// --- Expression tests ---

func TestParseIntLit(t *testing.T) {
	mod, errs := parse(`let x = 42`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	if _, ok := letD.Value.(*ast.IntLit); !ok {
		t.Errorf("expected IntLit, got %T", letD.Value)
	}
}

func TestParseFloatLit(t *testing.T) {
	mod, errs := parse(`let x = 3.14`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	fl, ok := letD.Value.(*ast.FloatLit)
	if !ok {
		t.Fatalf("expected FloatLit, got %T", letD.Value)
	}
	if fl.Value != "3.14" {
		t.Errorf("expected '3.14', got %q", fl.Value)
	}
}

func TestParseBoolLit(t *testing.T) {
	mod, errs := parse(`let x = true`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	bl, ok := letD.Value.(*ast.BoolLit)
	if !ok {
		t.Fatalf("expected BoolLit, got %T", letD.Value)
	}
	if !bl.Value {
		t.Errorf("expected true")
	}
}

func TestParseStringLit(t *testing.T) {
	mod, errs := parse(`let x = "hello"`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	sl, ok := letD.Value.(*ast.StringLit)
	if !ok {
		t.Fatalf("expected StringLit, got %T", letD.Value)
	}
	if sl.Value != "hello" {
		t.Errorf("expected 'hello', got %q", sl.Value)
	}
}

func TestParseIdent(t *testing.T) {
	mod, errs := parse(`let x = y`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	id, ok := letD.Value.(*ast.Ident)
	if !ok {
		t.Fatalf("expected Ident, got %T", letD.Value)
	}
	if id.Name != "y" {
		t.Errorf("expected 'y', got %q", id.Name)
	}
}

// --- Binary expressions ---

func TestParseBinaryAdd(t *testing.T) {
	mod, errs := parse(`let x = 1 + 2`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	bin, ok := letD.Value.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", letD.Value)
	}
	if bin.Op != ast.OpAdd {
		t.Errorf("expected OpAdd, got %d", bin.Op)
	}
}

func TestParseBinarySub(t *testing.T) {
	mod, errs := parse(`let x = a - b`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	bin := letD.Value.(*ast.BinaryExpr)
	if bin.Op != ast.OpSub {
		t.Errorf("expected OpSub, got %d", bin.Op)
	}
}

func TestParseBinaryMul(t *testing.T) {
	mod, errs := parse(`let x = a * b`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	bin := letD.Value.(*ast.BinaryExpr)
	if bin.Op != ast.OpMul {
		t.Errorf("expected OpMul, got %d", bin.Op)
	}
}

func TestParseBinaryComparison(t *testing.T) {
	tests := []struct {
		source string
		op     ast.BinaryOp
	}{
		{`let x = a == b`, ast.OpEq},
		{`let x = a != b`, ast.OpNeq},
		{`let x = a < b`, ast.OpLt},
		{`let x = a > b`, ast.OpGt},
		{`let x = a <= b`, ast.OpLte},
		{`let x = a >= b`, ast.OpGte},
	}
	for _, tt := range tests {
		mod, errs := parse(tt.source)
		expectNoErrors(t, errs)
		letD := mod.Decls[0].(*ast.LetDecl)
		bin, ok := letD.Value.(*ast.BinaryExpr)
		if !ok {
			t.Fatalf("%s: expected BinaryExpr, got %T", tt.source, letD.Value)
		}
		if bin.Op != tt.op {
			t.Errorf("%s: expected op %d, got %d", tt.source, tt.op, bin.Op)
		}
	}
}

func TestParseBinaryLogical(t *testing.T) {
	tests := []struct {
		source string
		op     ast.BinaryOp
	}{
		{`let x = a && b`, ast.OpAnd},
		{`let x = a || b`, ast.OpOr},
	}
	for _, tt := range tests {
		mod, errs := parse(tt.source)
		expectNoErrors(t, errs)
		letD := mod.Decls[0].(*ast.LetDecl)
		bin := letD.Value.(*ast.BinaryExpr)
		if bin.Op != tt.op {
			t.Errorf("%s: expected op %d, got %d", tt.source, tt.op, bin.Op)
		}
	}
}

func TestParseStringConcat(t *testing.T) {
	mod, errs := parse(`let x = "a" <> "b"`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	bin := letD.Value.(*ast.BinaryExpr)
	if bin.Op != ast.OpConcat {
		t.Errorf("expected OpConcat, got %d", bin.Op)
	}
}

// --- Operator precedence ---

func TestPrecedenceMulOverAdd(t *testing.T) {
	// 1 + 2 * 3 should parse as 1 + (2 * 3)
	mod, errs := parse(`let x = 1 + 2 * 3`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	bin := letD.Value.(*ast.BinaryExpr)
	if bin.Op != ast.OpAdd {
		t.Fatalf("top level should be +, got %d", bin.Op)
	}
	right, ok := bin.Right.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("right should be BinaryExpr, got %T", bin.Right)
	}
	if right.Op != ast.OpMul {
		t.Errorf("right should be *, got %d", right.Op)
	}
}

func TestPrecedenceCompareOverLogical(t *testing.T) {
	// a && b == c should parse as a && (b == c)
	mod, errs := parse(`let x = a && b == c`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	bin := letD.Value.(*ast.BinaryExpr)
	if bin.Op != ast.OpAnd {
		t.Fatalf("top level should be &&, got %d", bin.Op)
	}
	right, ok := bin.Right.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("right should be BinaryExpr, got %T", bin.Right)
	}
	if right.Op != ast.OpEq {
		t.Errorf("right should be ==, got %d", right.Op)
	}
}

func TestPrecedencePipeLowest(t *testing.T) {
	// x |> f + 1 should parse as x |> (f + 1)
	mod, errs := parse(`let y = x |> f + 1`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	bin := letD.Value.(*ast.BinaryExpr)
	if bin.Op != ast.OpPipe {
		t.Fatalf("top level should be |>, got %d", bin.Op)
	}
}

func TestPrecedenceGrouping(t *testing.T) {
	// (1 + 2) * 3 should parse as (1 + 2) * 3
	mod, errs := parse(`let x = (1 + 2) * 3`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	bin := letD.Value.(*ast.BinaryExpr)
	if bin.Op != ast.OpMul {
		t.Fatalf("top level should be *, got %d", bin.Op)
	}
	left, ok := bin.Left.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("left should be BinaryExpr, got %T", bin.Left)
	}
	if left.Op != ast.OpAdd {
		t.Errorf("left should be +, got %d", left.Op)
	}
}

// --- Unary expressions ---

func TestParseUnaryNeg(t *testing.T) {
	mod, errs := parse(`let x = -42`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	un, ok := letD.Value.(*ast.UnaryExpr)
	if !ok {
		t.Fatalf("expected UnaryExpr, got %T", letD.Value)
	}
	if un.Op != ast.OpNeg {
		t.Errorf("expected OpNeg")
	}
}

func TestParseUnaryNot(t *testing.T) {
	mod, errs := parse(`let x = !true`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	un, ok := letD.Value.(*ast.UnaryExpr)
	if !ok {
		t.Fatalf("expected UnaryExpr, got %T", letD.Value)
	}
	if un.Op != ast.OpNot {
		t.Errorf("expected OpNot")
	}
}

// --- Call expressions ---

func TestParseCallNoArgs(t *testing.T) {
	mod, errs := parse(`let x = foo()`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	call, ok := letD.Value.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected CallExpr, got %T", letD.Value)
	}
	fn, ok := call.Func.(*ast.Ident)
	if !ok {
		t.Fatalf("expected Ident, got %T", call.Func)
	}
	if fn.Name != "foo" {
		t.Errorf("expected 'foo', got %q", fn.Name)
	}
	if len(call.Args) != 0 {
		t.Errorf("expected 0 args, got %d", len(call.Args))
	}
}

func TestParseCallWithArgs(t *testing.T) {
	mod, errs := parse(`let x = add(1, 2)`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	call := letD.Value.(*ast.CallExpr)
	if len(call.Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(call.Args))
	}
}

func TestParseQualifiedCall(t *testing.T) {
	mod, errs := parse(`let x = fmt.println("hello")`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	call, ok := letD.Value.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected CallExpr, got %T", letD.Value)
	}
	fa, ok := call.Func.(*ast.FieldAccessExpr)
	if !ok {
		t.Fatalf("expected FieldAccessExpr, got %T", call.Func)
	}
	if fa.Field != "println" {
		t.Errorf("expected field 'println', got %q", fa.Field)
	}
}

// --- Field access ---

func TestParseFieldAccess(t *testing.T) {
	mod, errs := parse(`let x = point.x`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	fa, ok := letD.Value.(*ast.FieldAccessExpr)
	if !ok {
		t.Fatalf("expected FieldAccessExpr, got %T", letD.Value)
	}
	if fa.Field != "x" {
		t.Errorf("expected field 'x', got %q", fa.Field)
	}
}

func TestParseChainedFieldAccess(t *testing.T) {
	mod, errs := parse(`let x = a.b.c`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	fa, ok := letD.Value.(*ast.FieldAccessExpr)
	if !ok {
		t.Fatalf("expected FieldAccessExpr, got %T", letD.Value)
	}
	if fa.Field != "c" {
		t.Errorf("expected field 'c', got %q", fa.Field)
	}
	inner, ok := fa.Expr.(*ast.FieldAccessExpr)
	if !ok {
		t.Fatalf("expected inner FieldAccessExpr, got %T", fa.Expr)
	}
	if inner.Field != "b" {
		t.Errorf("expected field 'b', got %q", inner.Field)
	}
}

// --- If/else ---

func TestParseIfExpr(t *testing.T) {
	mod, errs := parse(`fn check() do
  if true do
    1
  end
end`)
	expectNoErrors(t, errs)
	fn := mod.Decls[0].(*ast.FnDecl)
	ifExpr, ok := fn.Body[0].(*ast.IfExpr)
	if !ok {
		t.Fatalf("expected IfExpr, got %T", fn.Body[0])
	}
	if len(ifExpr.Then) != 1 {
		t.Errorf("expected 1 then expr, got %d", len(ifExpr.Then))
	}
	if ifExpr.Else != nil {
		t.Errorf("expected no else branch")
	}
}

func TestParseIfElseExpr(t *testing.T) {
	mod, errs := parse(`fn check() do
  if x > 0 do
    1
  end else do
    2
  end
end`)
	expectNoErrors(t, errs)
	fn := mod.Decls[0].(*ast.FnDecl)
	ifExpr := fn.Body[0].(*ast.IfExpr)
	if len(ifExpr.Then) != 1 {
		t.Errorf("expected 1 then expr, got %d", len(ifExpr.Then))
	}
	if len(ifExpr.Else) != 1 {
		t.Errorf("expected 1 else expr, got %d", len(ifExpr.Else))
	}
}

func TestParseIfElseIfExpr(t *testing.T) {
	mod, errs := parse(`fn check() do
  if x > 0 do
    1
  end else if x < 0 do
    2
  end else do
    0
  end
end`)
	expectNoErrors(t, errs)
	fn := mod.Decls[0].(*ast.FnDecl)
	ifExpr := fn.Body[0].(*ast.IfExpr)
	if len(ifExpr.Else) != 1 {
		t.Fatalf("expected 1 else expr (nested if), got %d", len(ifExpr.Else))
	}
	innerIf, ok := ifExpr.Else[0].(*ast.IfExpr)
	if !ok {
		t.Fatalf("expected nested IfExpr, got %T", ifExpr.Else[0])
	}
	if len(innerIf.Else) != 1 {
		t.Errorf("expected 1 inner else expr, got %d", len(innerIf.Else))
	}
}

// --- Block expressions ---

func TestParseBlockExpr(t *testing.T) {
	mod, errs := parse(`fn check() do
  do
    1
    2
  end
end`)
	expectNoErrors(t, errs)
	fn := mod.Decls[0].(*ast.FnDecl)
	block, ok := fn.Body[0].(*ast.BlockExpr)
	if !ok {
		t.Fatalf("expected BlockExpr, got %T", fn.Body[0])
	}
	if len(block.Stmts) != 2 {
		t.Errorf("expected 2 stmts, got %d", len(block.Stmts))
	}
}

// --- String interpolation ---

func TestParseStringInterpolation(t *testing.T) {
	mod, errs := parse(`let x = "hello #{name}!"`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	interp, ok := letD.Value.(*ast.StringInterpolation)
	if !ok {
		t.Fatalf("expected StringInterpolation, got %T", letD.Value)
	}
	if len(interp.Parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(interp.Parts))
	}
	text0, ok := interp.Parts[0].(*ast.StringText)
	if !ok {
		t.Fatalf("part 0: expected StringText, got %T", interp.Parts[0])
	}
	if text0.Value != "hello " {
		t.Errorf("part 0: expected 'hello ', got %q", text0.Value)
	}
	interpExpr, ok := interp.Parts[1].(*ast.StringInterpExpr)
	if !ok {
		t.Fatalf("part 1: expected StringInterpExpr, got %T", interp.Parts[1])
	}
	ident, ok := interpExpr.Expr.(*ast.Ident)
	if !ok {
		t.Fatalf("interp expr: expected Ident, got %T", interpExpr.Expr)
	}
	if ident.Name != "name" {
		t.Errorf("expected 'name', got %q", ident.Name)
	}
	text2, ok := interp.Parts[2].(*ast.StringText)
	if !ok {
		t.Fatalf("part 2: expected StringText, got %T", interp.Parts[2])
	}
	if text2.Value != "!" {
		t.Errorf("part 2: expected '!', got %q", text2.Value)
	}
}

func TestParseStringInterpolationWithExpr(t *testing.T) {
	mod, errs := parse(`let x = "result: #{1 + 2}"`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	interp := letD.Value.(*ast.StringInterpolation)
	interpExpr := interp.Parts[1].(*ast.StringInterpExpr)
	_, ok := interpExpr.Expr.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr in interpolation, got %T", interpExpr.Expr)
	}
}

// --- Pipe operator ---

func TestParsePipeChain(t *testing.T) {
	mod, errs := parse(`let x = a |> double |> add(1)`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	// Should be: (a |> double) |> add(1)  — left-associative
	bin := letD.Value.(*ast.BinaryExpr)
	if bin.Op != ast.OpPipe {
		t.Fatalf("top level should be |>, got %d", bin.Op)
	}
	// Right side should be add(1)
	_, ok := bin.Right.(*ast.CallExpr)
	if !ok {
		t.Fatalf("right should be CallExpr, got %T", bin.Right)
	}
	// Left should be another pipe
	leftBin, ok := bin.Left.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("left should be BinaryExpr, got %T", bin.Left)
	}
	if leftBin.Op != ast.OpPipe {
		t.Errorf("left should be |>, got %d", leftBin.Op)
	}
}

// --- Record literals ---

func TestParseRecordLit(t *testing.T) {
	mod, errs := parse(`let p = Point { x: 1, y: 2 }`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	rec, ok := letD.Value.(*ast.RecordLit)
	if !ok {
		t.Fatalf("expected RecordLit, got %T", letD.Value)
	}
	if rec.Name != "Point" {
		t.Errorf("expected name 'Point', got %q", rec.Name)
	}
	if len(rec.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(rec.Fields))
	}
	if rec.Fields[0].Name != "x" {
		t.Errorf("field 0: expected 'x', got %q", rec.Fields[0].Name)
	}
}

// --- Let expression inside fn body ---

func TestParseLetExprInBody(t *testing.T) {
	mod, errs := parse(`fn check() do
  let x = 42
  x
end`)
	expectNoErrors(t, errs)
	fn := mod.Decls[0].(*ast.FnDecl)
	if len(fn.Body) != 2 {
		t.Fatalf("expected 2 body stmts, got %d", len(fn.Body))
	}
	letExpr, ok := fn.Body[0].(*ast.LetExpr)
	if !ok {
		t.Fatalf("expected LetExpr, got %T", fn.Body[0])
	}
	if letExpr.Name != "x" {
		t.Errorf("expected name 'x', got %q", letExpr.Name)
	}
}

// --- Return expression ---

func TestParseReturn(t *testing.T) {
	mod, errs := parse(`fn check() do
  return 42
end`)
	expectNoErrors(t, errs)
	fn := mod.Decls[0].(*ast.FnDecl)
	ret, ok := fn.Body[0].(*ast.ReturnExpr)
	if !ok {
		t.Fatalf("expected ReturnExpr, got %T", fn.Body[0])
	}
	if ret.Value == nil {
		t.Fatal("expected return value")
	}
}

func TestParseReturnBare(t *testing.T) {
	mod, errs := parse(`fn check() do
  return
end`)
	expectNoErrors(t, errs)
	fn := mod.Decls[0].(*ast.FnDecl)
	ret, ok := fn.Body[0].(*ast.ReturnExpr)
	if !ok {
		t.Fatalf("expected ReturnExpr, got %T", fn.Body[0])
	}
	if ret.Value != nil {
		t.Errorf("expected no return value")
	}
}

// --- Anonymous function ---

func TestParseFnLit(t *testing.T) {
	mod, errs := parse(`let f = fn(x: Int) do
  x + 1
end`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	fnLit, ok := letD.Value.(*ast.FnLit)
	if !ok {
		t.Fatalf("expected FnLit, got %T", letD.Value)
	}
	if len(fnLit.Params) != 1 {
		t.Errorf("expected 1 param, got %d", len(fnLit.Params))
	}
	if len(fnLit.Body) != 1 {
		t.Errorf("expected 1 body expr, got %d", len(fnLit.Body))
	}
}

// --- Nil ---

func TestParseNil(t *testing.T) {
	mod, errs := parse(`let x = nil`)
	expectNoErrors(t, errs)
	letD := mod.Decls[0].(*ast.LetDecl)
	_, ok := letD.Value.(*ast.NilLit)
	if !ok {
		t.Fatalf("expected NilLit, got %T", letD.Value)
	}
}

// --- Qualified type ---

func TestParseQualifiedType(t *testing.T) {
	mod, errs := parse(`fn handler(w: http.ResponseWriter, r: http.Request) do
  42
end`)
	expectNoErrors(t, errs)
	fn := mod.Decls[0].(*ast.FnDecl)
	qt, ok := fn.Params[0].Type.(*ast.QualifiedType)
	if !ok {
		t.Fatalf("expected QualifiedType, got %T", fn.Params[0].Type)
	}
	if qt.Qualifier != "http" || qt.Name != "ResponseWriter" {
		t.Errorf("expected http.ResponseWriter, got %s.%s", qt.Qualifier, qt.Name)
	}
}

// --- Error recovery ---

func TestErrorRecoveryMultipleErrors(t *testing.T) {
	// Multiple broken declarations should produce multiple errors but not crash
	_, errs := parse(`fn check() do
  let = 42
  let y =
end`)
	if len(errs) == 0 {
		t.Fatal("expected parse errors")
	}
}

func TestErrorRecoveryContinuesParsing(t *testing.T) {
	// After an error in one declaration, should still parse the next
	mod, errs := parse(`let = 42
fn valid() do
  1
end`)
	if len(errs) == 0 {
		t.Fatal("expected at least one error")
	}
	// Should still have parsed the valid fn
	found := false
	for _, d := range mod.Decls {
		if fn, ok := d.(*ast.FnDecl); ok && fn.Name == "valid" {
			found = true
		}
	}
	if !found {
		t.Error("expected to find 'valid' function after error recovery")
	}
}

// --- Snapshot / integration tests ---

func TestSnapshotFullProgram(t *testing.T) {
	source := `import "fmt"
import "net/http"

pub fn main() do
  let greeting = "Hello from Golem!"
  fmt.println(greeting)
end`

	mod, errs := parse(source)
	expectNoErrors(t, errs)

	if len(mod.Imports) != 2 {
		t.Errorf("expected 2 imports, got %d", len(mod.Imports))
	}
	if len(mod.Decls) != 1 {
		t.Fatalf("expected 1 decl, got %d", len(mod.Decls))
	}
	fn := mod.Decls[0].(*ast.FnDecl)
	if fn.Name != "main" {
		t.Errorf("expected 'main', got %q", fn.Name)
	}
	if fn.Visibility != ast.VisPub {
		t.Errorf("expected pub visibility")
	}
	if len(fn.Body) != 2 {
		t.Errorf("expected 2 body stmts, got %d", len(fn.Body))
	}
}

func TestSnapshotFnWithCallback(t *testing.T) {
	source := `pub fn main() do
  http.handleFunc("/", fn(w: http.ResponseWriter, r: http.Request) do
    fmt.fprintln(w, "Hello from Golem!")
  end)
end`

	mod, errs := parse(source)
	expectNoErrors(t, errs)

	fn := mod.Decls[0].(*ast.FnDecl)
	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 body stmt, got %d", len(fn.Body))
	}
	// Body should be a call to http.handleFunc
	call, ok := fn.Body[0].(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected CallExpr, got %T", fn.Body[0])
	}
	if len(call.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(call.Args))
	}
	// Second arg should be a FnLit
	fnLit, ok := call.Args[1].(*ast.FnLit)
	if !ok {
		t.Fatalf("expected FnLit arg, got %T", call.Args[1])
	}
	if len(fnLit.Params) != 2 {
		t.Errorf("expected 2 params, got %d", len(fnLit.Params))
	}
}

func TestSnapshotTypeAndConstruction(t *testing.T) {
	source := `type Point = { x: Float, y: Float }

let origin = Point { x: 0, y: 0 }

fn distance(p: Point): Float do
  p.x + p.y
end`

	mod, errs := parse(source)
	expectNoErrors(t, errs)

	if len(mod.Decls) != 3 {
		t.Fatalf("expected 3 decls, got %d", len(mod.Decls))
	}

	// First: TypeDecl
	td, ok := mod.Decls[0].(*ast.TypeDecl)
	if !ok {
		t.Fatalf("decl 0: expected TypeDecl, got %T", mod.Decls[0])
	}
	if td.Name != "Point" {
		t.Errorf("expected 'Point', got %q", td.Name)
	}

	// Second: LetDecl with RecordLit
	letD, ok := mod.Decls[1].(*ast.LetDecl)
	if !ok {
		t.Fatalf("decl 1: expected LetDecl, got %T", mod.Decls[1])
	}
	rec, ok := letD.Value.(*ast.RecordLit)
	if !ok {
		t.Fatalf("expected RecordLit, got %T", letD.Value)
	}
	if rec.Name != "Point" {
		t.Errorf("expected 'Point', got %q", rec.Name)
	}

	// Third: FnDecl
	fn, ok := mod.Decls[2].(*ast.FnDecl)
	if !ok {
		t.Fatalf("decl 2: expected FnDecl, got %T", mod.Decls[2])
	}
	if fn.Name != "distance" {
		t.Errorf("expected 'distance', got %q", fn.Name)
	}
}

func TestSnapshotPipeWithCalls(t *testing.T) {
	source := `let result = data |> transform() |> filter(pred) |> collect()`

	mod, errs := parse(source)
	expectNoErrors(t, errs)

	letD := mod.Decls[0].(*ast.LetDecl)
	// Top level should be a pipe
	bin, ok := letD.Value.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", letD.Value)
	}
	if bin.Op != ast.OpPipe {
		t.Errorf("expected pipe op, got %d", bin.Op)
	}
}
