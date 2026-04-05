package resolver

import (
	"testing"

	"github.com/dylanblakemore/golem/internal/ast"
	"github.com/dylanblakemore/golem/internal/lexer"
	"github.com/dylanblakemore/golem/internal/parser"
)

func resolve(source string) (*Resolution, []Error) {
	l := lexer.New(source, "test.golem")
	tokens := l.Tokenize()
	p := parser.New(tokens, "test.golem")
	mod, perrs := p.Parse()
	if len(perrs) > 0 {
		panic("parse errors: " + perrs[0].Error())
	}
	return Resolve(mod)
}

func expectNoErrors(t *testing.T, errors []Error) {
	t.Helper()
	if len(errors) > 0 {
		for _, e := range errors {
			t.Errorf("resolver error: %s", e)
		}
		t.FailNow()
	}
}

func expectOneError(t *testing.T, errors []Error) {
	t.Helper()
	if len(errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(errors))
		for _, e := range errors {
			t.Logf("  %s", e)
		}
		t.FailNow()
	}
}

func expectErrorContains(t *testing.T, errors []Error, substr string) {
	t.Helper()
	for _, e := range errors {
		if contains(e.Message, substr) {
			return
		}
	}
	t.Errorf("expected error containing %q, got:", substr)
	for _, e := range errors {
		t.Logf("  %s", e)
	}
	t.FailNow()
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- Forward references ---

func TestForwardReferenceBetweenFunctions(t *testing.T) {
	_, errs := resolve(`fn foo() do
  bar()
end

fn bar() do
  42
end`)
	expectNoErrors(t, errs)
}

func TestForwardReferenceToFunction(t *testing.T) {
	_, errs := resolve(`fn main() do
  helper()
end

fn helper() do
  1
end`)
	expectNoErrors(t, errs)
}

func TestMutualRecursion(t *testing.T) {
	_, errs := resolve(`fn even(n: Int): Bool do
  if n == 0 do
    true
  end else do
    odd(n - 1)
  end
end

fn odd(n: Int): Bool do
  if n == 0 do
    false
  end else do
    even(n - 1)
  end
end`)
	expectNoErrors(t, errs)
}

// --- Scope nesting and shadowing ---

func TestLetBindingInScope(t *testing.T) {
	_, errs := resolve(`fn main() do
  let x = 1
  x + 1
end`)
	expectNoErrors(t, errs)
}

func TestNestedScopes(t *testing.T) {
	_, errs := resolve(`fn main() do
  let x = 1
  let y = do
    let z = x + 1
    z
  end
  y
end`)
	expectNoErrors(t, errs)
}

func TestShadowing(t *testing.T) {
	_, errs := resolve(`fn main() do
  let x = 1
  let x = x + 1
  x
end`)
	expectNoErrors(t, errs)
}

func TestIfScopesAreIsolated(t *testing.T) {
	_, errs := resolve(`fn main() do
  let x = true
  if x do
    let y = 1
    y
  end else do
    let y = 2
    y
  end
end`)
	expectNoErrors(t, errs)
}

func TestParameterScope(t *testing.T) {
	_, errs := resolve(`fn add(a: Int, b: Int): Int do
  a + b
end`)
	expectNoErrors(t, errs)
}

// --- Imports ---

func TestImportResolution(t *testing.T) {
	_, errs := resolve(`import "fmt"

fn main() do
  fmt.println("hello")
end`)
	expectNoErrors(t, errs)
}

func TestQualifiedImportCall(t *testing.T) {
	_, errs := resolve(`import "net/http"

fn main() do
  http.listenAndServe(":8080", nil)
end`)
	expectNoErrors(t, errs)
}

func TestMultipleImports(t *testing.T) {
	_, errs := resolve(`import "fmt"
import "net/http"

fn main() do
  fmt.println("starting")
  http.listenAndServe(":8080", nil)
end`)
	expectNoErrors(t, errs)
}

// --- Undefined variable errors ---

func TestUndefinedVariable(t *testing.T) {
	_, errs := resolve(`fn main() do
  x + 1
end`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "undefined variable")
	expectErrorContains(t, errs, "x")
}

func TestUndefinedFunction(t *testing.T) {
	_, errs := resolve(`fn main() do
  missing()
end`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "undefined variable")
	expectErrorContains(t, errs, "missing")
}

func TestUndefinedInNestedScope(t *testing.T) {
	_, errs := resolve(`fn main() do
  let x = do
    y
  end
  x
end`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "undefined variable")
	expectErrorContains(t, errs, "y")
}

func TestLetNotVisibleOutsideBlock(t *testing.T) {
	_, errs := resolve(`fn main() do
  if true do
    let x = 1
    x
  end
  x
end`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "undefined variable")
	expectErrorContains(t, errs, "x")
}

// --- Duplicate declaration errors ---

func TestDuplicateFunction(t *testing.T) {
	_, errs := resolve(`fn foo() do
  1
end

fn foo() do
  2
end`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "duplicate declaration")
	expectErrorContains(t, errs, "foo")
}

func TestDuplicateType(t *testing.T) {
	_, errs := resolve(`type Point = { x: Int, y: Int }
type Point = { a: Float, b: Float }`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "duplicate declaration")
	expectErrorContains(t, errs, "Point")
}

func TestDuplicateImport(t *testing.T) {
	_, errs := resolve(`import "fmt"
import "fmt"`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "duplicate import")
}

func TestDuplicateParameter(t *testing.T) {
	_, errs := resolve(`fn foo(x: Int, x: Int) do
  x
end`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "duplicate parameter")
}

// --- Record literals ---

func TestRecordLitResolution(t *testing.T) {
	_, errs := resolve(`type Point = { x: Int, y: Int }

fn main() do
  Point { x: 1, y: 2 }
end`)
	expectNoErrors(t, errs)
}

func TestUndefinedRecordType(t *testing.T) {
	_, errs := resolve(`fn main() do
  Unknown { x: 1 }
end`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "undefined")
}

// --- Anonymous functions ---

func TestFnLitResolution(t *testing.T) {
	_, errs := resolve(`fn main() do
  let f = fn(x: Int) do
    x + 1
  end
  f(5)
end`)
	expectNoErrors(t, errs)
}

func TestFnLitClosesOverScope(t *testing.T) {
	_, errs := resolve(`fn main() do
  let y = 10
  let f = fn(x: Int) do
    x + y
  end
  f(5)
end`)
	expectNoErrors(t, errs)
}

// --- Resolution lookups ---

func TestResolutionLookup(t *testing.T) {
	res, errs := resolve(`fn main() do
  let x = 1
  x
end`)
	expectNoErrors(t, errs)

	// Check that there are refs recorded
	if len(res.Refs) == 0 {
		t.Error("expected resolved references, got none")
	}
}

func TestResolutionRefKinds(t *testing.T) {
	res, errs := resolve(`import "fmt"

fn helper() do
  1
end

fn main() do
  helper()
  fmt.println("hi")
end`)
	expectNoErrors(t, errs)

	// Verify we have refs of different kinds
	hasFunction := false
	hasImport := false
	hasImportRef := false
	for _, ref := range res.Refs {
		switch ref.Kind { //nolint:exhaustive // only checking for specific kinds
		case DeclFunction:
			hasFunction = true
		case DeclImport:
			hasImport = true
		case DeclImportRef:
			hasImportRef = true
		}
	}
	if !hasFunction {
		t.Error("expected a function reference")
	}
	if !hasImport {
		t.Error("expected an import reference")
	}
	if !hasImportRef {
		t.Error("expected an import member reference")
	}
}

// --- Return expressions ---

func TestReturnResolution(t *testing.T) {
	_, errs := resolve(`fn foo(x: Int): Int do
  return x + 1
end`)
	expectNoErrors(t, errs)
}

// --- String interpolation ---

func TestStringInterpolationResolution(t *testing.T) {
	// String interpolation needs the lexer to produce HASH_LBRACE tokens.
	// For now, just test that variables inside expressions are resolved.
	_, errs := resolve(`fn main() do
  let name = "world"
  name
end`)
	expectNoErrors(t, errs)
}

// --- Complex program ---

func TestFullProgram(t *testing.T) {
	_, errs := resolve(`import "fmt"
import "net/http"

type Config = { port: Int, host: String }

fn makeConfig(): Config do
  Config { port: 8080, host: "localhost" }
end

pub fn main() do
  let config = makeConfig()
  fmt.println(config)
  http.listenAndServe(":8080", nil)
end`)
	expectNoErrors(t, errs)
}

// --- Edge cases ---

func TestEmptyModule(t *testing.T) {
	mod := &ast.Module{File: "empty.golem"}
	_, errs := Resolve(mod)
	expectNoErrors(t, errs)
}

func TestLetBeforeUse(t *testing.T) {
	// let x is used before being defined in the same scope — but
	// since x is a top-level let, it's registered in Phase 1
	_, errs := resolve(`let x = 1

fn main() do
  x
end`)
	expectNoErrors(t, errs)
}
