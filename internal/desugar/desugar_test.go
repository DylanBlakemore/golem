package desugar

import (
	"testing"

	"github.com/dylanblakemore/golem/internal/ast"
	"github.com/dylanblakemore/golem/internal/lexer"
	"github.com/dylanblakemore/golem/internal/parser"
)

func parse(source string) *ast.Module {
	l := lexer.New(source, "test.golem")
	tokens := l.Tokenize()
	p := parser.New(tokens, "test.golem")
	mod, perrs := p.Parse()
	if len(perrs) > 0 {
		panic("parse errors: " + perrs[0].Error())
	}
	return mod
}

// --- Pipe operator desugaring ---

func TestPipeToIdent(t *testing.T) {
	mod := parse(`fn main() do
  42 |> foo
end`)
	result := Desugar(mod)
	body := fnBody(t, result.Module, "main")
	call, ok := body[0].(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected CallExpr, got %T", body[0])
	}
	ident, ok := call.Func.(*ast.Ident)
	if !ok {
		t.Fatalf("expected Ident func, got %T", call.Func)
	}
	if ident.Name != "foo" {
		t.Errorf("expected func name foo, got %s", ident.Name)
	}
	if len(call.Args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(call.Args))
	}
	lit, ok := call.Args[0].(*ast.IntLit)
	if !ok {
		t.Fatalf("expected IntLit arg, got %T", call.Args[0])
	}
	if lit.Value != "42" {
		t.Errorf("expected 42, got %s", lit.Value)
	}
}

func TestPipeToCall(t *testing.T) {
	mod := parse(`fn main() do
  1 |> add(2)
end`)
	result := Desugar(mod)
	body := fnBody(t, result.Module, "main")
	call, ok := body[0].(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected CallExpr, got %T", body[0])
	}
	ident, ok := call.Func.(*ast.Ident)
	if !ok {
		t.Fatalf("expected Ident func, got %T", call.Func)
	}
	if ident.Name != "add" {
		t.Errorf("expected func name add, got %s", ident.Name)
	}
	if len(call.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(call.Args))
	}
	first, ok := call.Args[0].(*ast.IntLit)
	if !ok {
		t.Fatalf("expected IntLit first arg, got %T", call.Args[0])
	}
	if first.Value != "1" {
		t.Errorf("expected 1, got %s", first.Value)
	}
	second, ok := call.Args[1].(*ast.IntLit)
	if !ok {
		t.Fatalf("expected IntLit second arg, got %T", call.Args[1])
	}
	if second.Value != "2" {
		t.Errorf("expected 2, got %s", second.Value)
	}
}

func TestPipeChain(t *testing.T) {
	mod := parse(`fn main() do
  1 |> double |> add(10)
end`)
	result := Desugar(mod)
	body := fnBody(t, result.Module, "main")

	// The outer expression should be add(double(1), 10)
	outer, ok := body[0].(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected outer CallExpr, got %T", body[0])
	}
	outerIdent, ok := outer.Func.(*ast.Ident)
	if !ok {
		t.Fatalf("expected Ident func, got %T", outer.Func)
	}
	if outerIdent.Name != "add" {
		t.Errorf("expected add, got %s", outerIdent.Name)
	}
	if len(outer.Args) != 2 {
		t.Fatalf("expected 2 args for outer, got %d", len(outer.Args))
	}

	// First arg of outer should be double(1)
	inner, ok := outer.Args[0].(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected inner CallExpr, got %T", outer.Args[0])
	}
	innerIdent, ok := inner.Func.(*ast.Ident)
	if !ok {
		t.Fatalf("expected Ident func, got %T", inner.Func)
	}
	if innerIdent.Name != "double" {
		t.Errorf("expected double, got %s", innerIdent.Name)
	}
	if len(inner.Args) != 1 {
		t.Fatalf("expected 1 arg for inner, got %d", len(inner.Args))
	}
}

func TestPipeToQualifiedCall(t *testing.T) {
	mod := parse(`import "fmt"

fn main() do
  "hello" |> fmt.println
end`)
	result := Desugar(mod)
	body := fnBody(t, result.Module, "main")
	call, ok := body[0].(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected CallExpr, got %T", body[0])
	}
	fa, ok := call.Func.(*ast.FieldAccessExpr)
	if !ok {
		t.Fatalf("expected FieldAccessExpr func, got %T", call.Func)
	}
	if fa.Field != "println" {
		t.Errorf("expected field println, got %s", fa.Field)
	}
	if len(call.Args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(call.Args))
	}
}

// --- String interpolation desugaring ---

func TestStringInterpolationSimple(t *testing.T) {
	mod := parse(`fn main() do
  let name = "world"
  "Hello, #{name}!"
end`)
	result := Desugar(mod)
	body := fnBody(t, result.Module, "main")

	// Second expression should be fmt.Sprintf call
	call, ok := body[1].(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected CallExpr, got %T", body[1])
	}

	// Func should be fmt.Sprintf
	fa, ok := call.Func.(*ast.FieldAccessExpr)
	if !ok {
		t.Fatalf("expected FieldAccessExpr func, got %T", call.Func)
	}
	fmtIdent, ok := fa.Expr.(*ast.Ident)
	if !ok {
		t.Fatalf("expected Ident, got %T", fa.Expr)
	}
	if fmtIdent.Name != "fmt" {
		t.Errorf("expected fmt, got %s", fmtIdent.Name)
	}
	if fa.Field != "Sprintf" {
		t.Errorf("expected Sprintf, got %s", fa.Field)
	}

	// First arg should be format string
	if len(call.Args) < 2 {
		t.Fatalf("expected at least 2 args, got %d", len(call.Args))
	}
	formatStr, ok := call.Args[0].(*ast.StringLit)
	if !ok {
		t.Fatalf("expected StringLit format, got %T", call.Args[0])
	}
	if formatStr.Value != "Hello, %v!" {
		t.Errorf("expected format 'Hello, %%v!', got %q", formatStr.Value)
	}

	// Second arg should be the name ident
	nameIdent, ok := call.Args[1].(*ast.Ident)
	if !ok {
		t.Fatalf("expected Ident arg, got %T", call.Args[1])
	}
	if nameIdent.Name != "name" {
		t.Errorf("expected name, got %s", nameIdent.Name)
	}
}

func TestStringInterpolationMultiple(t *testing.T) {
	mod := parse(`fn main() do
  let x = 1
  let y = 2
  "#{x} + #{y}"
end`)
	result := Desugar(mod)
	body := fnBody(t, result.Module, "main")

	call, ok := body[2].(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected CallExpr, got %T", body[2])
	}
	if len(call.Args) != 3 {
		t.Fatalf("expected 3 args (format + 2 values), got %d", len(call.Args))
	}
	formatStr, ok := call.Args[0].(*ast.StringLit)
	if !ok {
		t.Fatalf("expected StringLit format, got %T", call.Args[0])
	}
	if formatStr.Value != "%v + %v" {
		t.Errorf("expected '%%v + %%v', got %q", formatStr.Value)
	}
}

func TestStringInterpolationNeedsFmt(t *testing.T) {
	mod := parse(`fn main() do
  "Hello, #{42}!"
end`)
	result := Desugar(mod)
	if !result.NeedsFmt {
		t.Error("expected NeedsFmt to be true")
	}

	// Check that fmt import was added
	found := false
	for _, imp := range result.Module.Imports {
		if imp.Path == "fmt" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected fmt import to be added")
	}
}

func TestStringInterpolationExistingFmt(t *testing.T) {
	mod := parse(`import "fmt"

fn main() do
  "Hello, #{42}!"
end`)
	result := Desugar(mod)

	// Should not duplicate the fmt import
	count := 0
	for _, imp := range result.Module.Imports {
		if imp.Path == "fmt" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 fmt import, got %d", count)
	}
}

func TestStringInterpolationEscapesPercent(t *testing.T) {
	mod := parse(`fn main() do
  "100% of #{name}"
end`)
	result := Desugar(mod)
	body := fnBody(t, result.Module, "main")

	call, ok := body[0].(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected CallExpr, got %T", body[0])
	}
	formatStr, ok := call.Args[0].(*ast.StringLit)
	if !ok {
		t.Fatalf("expected StringLit format, got %T", call.Args[0])
	}
	if formatStr.Value != "100%% of %v" {
		t.Errorf("expected '100%%%% of %%v', got %q", formatStr.Value)
	}
}

func TestStringInterpolationNoExprs(t *testing.T) {
	// A StringInterpolation with only text parts should become a StringLit
	mod := &ast.Module{
		File: "test.golem",
		Decls: []ast.Decl{
			&ast.FnDecl{
				Name:       "main",
				Visibility: ast.VisPub,
				Body: []ast.Expr{
					&ast.StringInterpolation{
						Parts: []ast.StringPart{
							&ast.StringText{Value: "no interpolation"},
						},
					},
				},
			},
		},
	}
	result := Desugar(mod)
	body := fnBody(t, result.Module, "main")
	lit, ok := body[0].(*ast.StringLit)
	if !ok {
		t.Fatalf("expected StringLit, got %T", body[0])
	}
	if lit.Value != "no interpolation" {
		t.Errorf("expected 'no interpolation', got %q", lit.Value)
	}
}

// --- Visibility mapping ---

func TestImplicitPriv(t *testing.T) {
	mod := parse(`fn helper() do
  42
end`)
	result := Desugar(mod)
	fn := findFn(t, result.Module, "helper")
	if fn.Visibility != ast.VisPriv {
		t.Errorf("expected VisPriv, got %d", fn.Visibility)
	}
}

func TestPubCapitalize(t *testing.T) {
	mod := parse(`pub fn myHandler() do
  42
end`)
	result := Desugar(mod)
	fn := findFn(t, result.Module, "MyHandler")
	if fn == nil {
		t.Fatal("expected fn named MyHandler")
	}
	if fn.Visibility != ast.VisPub {
		t.Errorf("expected VisPub, got %d", fn.Visibility)
	}
}

func TestPrivLowercase(t *testing.T) {
	mod := parse(`priv fn helper() do
  42
end`)
	result := Desugar(mod)
	fn := findFn(t, result.Module, "helper")
	if fn == nil {
		t.Fatal("expected fn named helper")
	}
}

func TestVisibilityNameMap(t *testing.T) {
	mod := parse(`pub fn greet() do
  42
end

fn callGreet() do
  greet()
end`)
	result := Desugar(mod)

	if result.NameMap["greet"] != "Greet" {
		t.Errorf("expected greet -> Greet, got %s", result.NameMap["greet"])
	}
	if result.NameMap["callGreet"] != "callGreet" {
		t.Errorf("expected callGreet -> callGreet, got %s", result.NameMap["callGreet"])
	}

	// Check that call site references are updated
	callerBody := fnBody(t, result.Module, "callGreet")
	call, ok := callerBody[0].(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected CallExpr, got %T", callerBody[0])
	}
	ident, ok := call.Func.(*ast.Ident)
	if !ok {
		t.Fatalf("expected Ident, got %T", call.Func)
	}
	if ident.Name != "Greet" {
		t.Errorf("expected call to Greet, got %s", ident.Name)
	}
}

func TestMapVisibility(t *testing.T) {
	tests := []struct {
		name string
		vis  ast.Visibility
		want string
	}{
		{"foo", ast.VisPub, "Foo"},
		{"foo", ast.VisPriv, "foo"},
		{"Foo", ast.VisPriv, "foo"},
		{"Foo", ast.VisPub, "Foo"},
		{"main", ast.VisPub, "main"},
		{"main", ast.VisDefault, "main"},
		{"", ast.VisPub, ""},
	}
	for _, tt := range tests {
		got := GoName(tt.name, tt.vis)
		if got != tt.want {
			t.Errorf("GoName(%q, %d) = %q, want %q", tt.name, tt.vis, got, tt.want)
		}
	}
}

// --- Helpers ---

func fnBody(t *testing.T, mod *ast.Module, name string) []ast.Expr {
	t.Helper()
	fn := findFn(t, mod, name)
	if fn == nil {
		t.Fatalf("function %q not found", name)
	}
	return fn.Body
}

func findFn(t *testing.T, mod *ast.Module, name string) *ast.FnDecl {
	t.Helper()
	for _, decl := range mod.Decls {
		if fn, ok := decl.(*ast.FnDecl); ok && fn.Name == name {
			return fn
		}
	}
	return nil
}
