package golifter_test

import (
	"testing"

	"github.com/dylanblakemore/golem/internal/ast"
	"github.com/dylanblakemore/golem/internal/golifter"
	"github.com/dylanblakemore/golem/internal/goloader"
	"github.com/dylanblakemore/golem/internal/lexer"
	"github.com/dylanblakemore/golem/internal/parser"
	"github.com/dylanblakemore/golem/internal/resolver"
)

func parseAndResolve(t *testing.T, src string) (*ast.Module, *resolver.Resolution) {
	t.Helper()
	l := lexer.New(src, "test.golem")
	tokens := l.Tokenize()
	p := parser.New(tokens, "test.golem")
	mod, perrs := p.Parse()
	if len(perrs) > 0 {
		t.Fatalf("parse errors: %v", perrs)
	}
	res, rerrs := resolver.Resolve(mod)
	if len(rerrs) > 0 {
		t.Fatalf("resolver errors: %v", rerrs)
	}
	return mod, res
}

func TestLiftNilLoader(t *testing.T) {
	// Lift with nil loader is a no-op.
	mod, res := parseAndResolve(t, `import "os"

fn readContent(path: String): String do
  path
end`)
	result := golifter.Lift(mod, res, nil)
	if result != mod {
		// Should return the same module unchanged.
		t.Error("expected same module with nil loader")
	}
}

func TestLiftDetectsErrorTupleReturn(t *testing.T) {
	// os.ReadFile returns ([]byte, error) — should be wrapped in GoLiftCallExpr.
	loader := goloader.New()
	mod, res := parseAndResolve(t, `import "os"

fn readContent(path: String): String do
  os.readFile(path)
  path
end`)
	lifted := golifter.Lift(mod, res, loader)

	fn, ok := lifted.Decls[0].(*ast.FnDecl)
	if !ok {
		t.Fatalf("expected FnDecl, got %T", lifted.Decls[0])
	}

	// First statement: os.readFile(path) — should be wrapped.
	callStmt := fn.Body[0]
	lift, ok := callStmt.(*ast.GoLiftCallExpr)
	if !ok {
		t.Fatalf("expected GoLiftCallExpr, got %T", callStmt)
	}
	if lift.ValueGoType == "" {
		t.Error("expected non-empty ValueGoType for ([]byte, error) return")
	}
	// os.ReadFile returns []byte as the success type.
	if lift.ValueGoType != "[]byte" {
		t.Errorf("expected ValueGoType=[]byte, got %q", lift.ValueGoType)
	}
}

func TestLiftDetectsErrorOnlyReturn(t *testing.T) {
	// net/http.ListenAndServe returns error — should be wrapped with empty ValueGoType.
	loader := goloader.New()
	mod, res := parseAndResolve(t, `import "net/http"

fn serve(): String do
  http.listenAndServe(":8080", nil)
  "done"
end`)
	lifted := golifter.Lift(mod, res, loader)

	fn, ok := lifted.Decls[0].(*ast.FnDecl)
	if !ok {
		t.Fatalf("expected FnDecl, got %T", lifted.Decls[0])
	}

	callStmt := fn.Body[0]
	lift, ok := callStmt.(*ast.GoLiftCallExpr)
	if !ok {
		t.Fatalf("expected GoLiftCallExpr, got %T", callStmt)
	}
	if lift.ValueGoType != "" {
		t.Errorf("expected empty ValueGoType for error-only return, got %q", lift.ValueGoType)
	}
}

func TestLiftDoesNotWrapNonErrorReturn(t *testing.T) {
	// fmt.Println returns (int, error) — but second return is error, so it IS wrapped.
	// fmt.Sprintf returns string — NOT wrapped.
	loader := goloader.New()
	mod, res := parseAndResolve(t, `import "fmt"

fn greet(name: String): String do
  fmt.sprintf("Hello, %v", name)
end`)
	lifted := golifter.Lift(mod, res, loader)

	fn, ok := lifted.Decls[0].(*ast.FnDecl)
	if !ok {
		t.Fatalf("expected FnDecl, got %T", lifted.Decls[0])
	}

	// fmt.Sprintf returns string, not (T, error) — should NOT be wrapped.
	callStmt := fn.Body[0]
	if _, ok := callStmt.(*ast.GoLiftCallExpr); ok {
		t.Error("expected plain CallExpr for fmt.Sprintf (no error return), got GoLiftCallExpr")
	}
}

func TestLiftPreservesNonGoCall(t *testing.T) {
	// Calls to local Golem functions should never be wrapped.
	loader := goloader.New()
	mod, res := parseAndResolve(t, `fn helper(): String do
  "hello"
end

fn main(): String do
  helper()
end`)
	lifted := golifter.Lift(mod, res, loader)

	fn, ok := lifted.Decls[1].(*ast.FnDecl)
	if !ok {
		t.Fatalf("expected second FnDecl, got %T", lifted.Decls[1])
	}

	callStmt := fn.Body[0]
	if _, ok := callStmt.(*ast.GoLiftCallExpr); ok {
		t.Error("local Golem function call should not be wrapped in GoLiftCallExpr")
	}
}
