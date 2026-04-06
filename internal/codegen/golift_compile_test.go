package codegen

import (
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dylanblakemore/golem/internal/ast"
	"github.com/dylanblakemore/golem/internal/span"
)

// TestGoLiftCompilable verifies that generated Go code using the GoLiftCallExpr
// IIFE pattern compiles correctly with the Go toolchain.
//
// This test constructs a minimal AST directly (bypassing the full pipeline)
// to isolate the codegen behavior for auto-lifted Go calls.
func TestGoLiftCompilable(t *testing.T) {
	// Build an AST representing:
	//   import "os"
	//   pub fn read(path String): Result<Int, Error>
	//     let result = GoLiftCallExpr(os.ReadFile(path))
	//     return Ok { value: 0 }
	//
	// The GoLiftCallExpr wraps os.ReadFile which returns ([]byte, error).
	zero := span.Span{}

	osReadFile := &ast.CallExpr{
		Span: zero,
		Func: &ast.FieldAccessExpr{
			Span:  zero,
			Expr:  &ast.Ident{Span: zero, Name: "os"},
			Field: "readFile", // codegen will exportField → ReadFile
		},
		Args: []ast.Expr{&ast.Ident{Span: zero, Name: "path"}},
	}

	lift := &ast.GoLiftCallExpr{
		Span:        zero,
		Call:        osReadFile,
		ValueGoType: "[]byte",
	}

	fn := &ast.FnDecl{
		Span:       zero,
		Name:       "Read",
		Visibility: ast.VisPub,
		Params: []*ast.Param{
			{Name: "path", Type: &ast.NamedType{Name: "String"}},
		},
		ReturnType: &ast.GenericType{
			Name: "Result",
			TypeArgs: []ast.TypeExpr{
				&ast.NamedType{Name: "Int"},
				&ast.NamedType{Name: "Error"},
			},
		},
		Body: []ast.Expr{
			// let result = GoLiftCallExpr(os.ReadFile(path))
			&ast.LetExpr{
				Span:  zero,
				Name:  "result",
				Value: lift,
			},
			// _ = result  (suppress unused-variable error in generated Go)
			&ast.LetExpr{
				Span:  zero,
				Name:  "_",
				Value: &ast.Ident{Span: zero, Name: "result"},
			},
			// return Ok { value: 0 }
			&ast.ReturnExpr{
				Span: zero,
				Value: &ast.RecordLit{
					Span: zero,
					Name: "ResultOk",
					Fields: []*ast.FieldInit{{
						Span:  zero,
						Name:  "value",
						Value: &ast.IntLit{Span: zero, Value: "0"},
					}},
				},
			},
		},
	}

	mod := &ast.Module{
		File:    "test.golem",
		Imports: []*ast.ImportDecl{{Path: "os"}},
		Decls:   []ast.Decl{fn},
	}

	out, err := Generate(mod, "test.golem")
	if err != nil {
		t.Fatalf("codegen error: %v", err)
	}

	// Verify the IIFE pattern is present.
	if !strings.Contains(string(out), "func() Result[[]byte, error]") {
		t.Errorf("expected IIFE in output, got:\n%s", out)
	}

	// Append a main function so `go build` is satisfied.
	withMain := string(out) + "\nfunc main() { Read(\"\") }\n"

	// Write to a temp dir and try go build to confirm valid Go.
	dir := t.TempDir()
	goVersion := build.Default.ReleaseTags[len(build.Default.ReleaseTags)-1][2:]
	gomodContent := "module testgolem\n\ngo " + goVersion + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomodContent), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(withMain), 0o600); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	cmd := exec.Command("go", "build", ".")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed:\n%s\nGenerated code:\n%s", output, withMain)
	}
}
