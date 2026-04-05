package checker

import (
	"testing"

	"github.com/dylanblakemore/golem/internal/lexer"
	"github.com/dylanblakemore/golem/internal/parser"
	"github.com/dylanblakemore/golem/internal/resolver"
)

func check(source string) (*TypeInfo, []Error) {
	l := lexer.New(source, "test.golem")
	tokens := l.Tokenize()
	p := parser.New(tokens, "test.golem")
	mod, perrs := p.Parse()
	if len(perrs) > 0 {
		panic("parse errors: " + perrs[0].Error())
	}
	res, rerrs := resolver.Resolve(mod)
	if len(rerrs) > 0 {
		panic("resolver errors: " + rerrs[0].Error())
	}
	return Check(mod, res)
}

func expectNoErrors(t *testing.T, errors []Error) {
	t.Helper()
	if len(errors) > 0 {
		for _, e := range errors {
			t.Errorf("type error: %s", e)
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
		if containsStr(e.Message, substr) {
			return
		}
	}
	t.Errorf("expected error containing %q, got:", substr)
	for _, e := range errors {
		t.Logf("  %s", e)
	}
	t.FailNow()
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- Literal type inference ---

func TestIntLiteral(t *testing.T) {
	_, errs := check(`fn main() do
  42
end`)
	expectNoErrors(t, errs)
}

func TestFloatLiteral(t *testing.T) {
	_, errs := check(`fn main() do
  3.14
end`)
	expectNoErrors(t, errs)
}

func TestStringLiteral(t *testing.T) {
	_, errs := check(`fn main() do
  "hello"
end`)
	expectNoErrors(t, errs)
}

func TestBoolLiteral(t *testing.T) {
	_, errs := check(`fn main() do
  true
end`)
	expectNoErrors(t, errs)
}

// --- Variable type inference ---

func TestLetBindingInference(t *testing.T) {
	_, errs := check(`fn main() do
  let x = 42
  x
end`)
	expectNoErrors(t, errs)
}

func TestLetBindingWithAnnotation(t *testing.T) {
	_, errs := check(`fn main() do
  let x: Int = 42
  x
end`)
	expectNoErrors(t, errs)
}

func TestLetBindingAnnotationMismatch(t *testing.T) {
	_, errs := check(`fn main() do
  let x: String = 42
  x
end`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "type mismatch")
}

// --- Arithmetic operators ---

func TestArithmeticOps(t *testing.T) {
	_, errs := check(`fn add(a: Int, b: Int): Int do
  a + b
end`)
	expectNoErrors(t, errs)
}

func TestArithmeticMismatch(t *testing.T) {
	_, errs := check(`fn bad(a: Int, b: String): Int do
  a + b
end`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "type mismatch")
}

// --- Comparison operators ---

func TestComparisonOps(t *testing.T) {
	_, errs := check(`fn compare(a: Int, b: Int): Bool do
  a < b
end`)
	expectNoErrors(t, errs)
}

func TestEqualityOps(t *testing.T) {
	_, errs := check(`fn eq(a: Int, b: Int): Bool do
  a == b
end`)
	expectNoErrors(t, errs)
}

// --- Logical operators ---

func TestLogicalOps(t *testing.T) {
	_, errs := check(`fn logic(a: Bool, b: Bool): Bool do
  a && b
end`)
	expectNoErrors(t, errs)
}

func TestLogicalOpTypeMismatch(t *testing.T) {
	_, errs := check(`fn bad(a: Int, b: Bool): Bool do
  a && b
end`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "type mismatch")
}

// --- String concatenation ---

func TestStringConcat(t *testing.T) {
	_, errs := check(`fn greet(name: String): String do
  "Hello, " <> name
end`)
	expectNoErrors(t, errs)
}

func TestStringConcatMismatch(t *testing.T) {
	_, errs := check(`fn bad(a: Int): String do
  "Hello, " <> a
end`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "type mismatch")
}

// --- Function calls ---

func TestFunctionCallTypeCheck(t *testing.T) {
	_, errs := check(`fn double(x: Int): Int do
  x + x
end

fn main() do
  double(5)
end`)
	expectNoErrors(t, errs)
}

func TestFunctionCallArgMismatch(t *testing.T) {
	_, errs := check(`fn double(x: Int): Int do
  x + x
end

fn main() do
  double("hello")
end`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "type mismatch")
}

func TestFunctionCallArityMismatch(t *testing.T) {
	_, errs := check(`fn add(a: Int, b: Int): Int do
  a + b
end

fn main() do
  add(1)
end`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "arity mismatch")
}

// --- If/else type checking ---

func TestIfElseBranchAgreement(t *testing.T) {
	_, errs := check(`fn abs(x: Int): Int do
  if x < 0 do
    0 - x
  end else do
    x
  end
end`)
	expectNoErrors(t, errs)
}

func TestIfElseBranchDisagreement(t *testing.T) {
	_, errs := check(`fn bad(x: Int) do
  if x < 0 do
    "negative"
  end else do
    42
  end
end`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "type mismatch")
}

func TestIfCondMustBeBool(t *testing.T) {
	_, errs := check(`fn bad() do
  if 42 do
    1
  end
end`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "type mismatch")
}

// --- Record types ---

func TestRecordLiteral(t *testing.T) {
	_, errs := check(`type Point = { x: Int, y: Int }

fn main() do
  Point { x: 1, y: 2 }
end`)
	expectNoErrors(t, errs)
}

func TestRecordFieldTypeMismatch(t *testing.T) {
	_, errs := check(`type Point = { x: Int, y: Int }

fn main() do
  Point { x: 1, y: "two" }
end`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "type mismatch")
}

func TestRecordMissingField(t *testing.T) {
	_, errs := check(`type Point = { x: Int, y: Int }

fn main() do
  Point { x: 1 }
end`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "missing field")
}

func TestRecordUnknownField(t *testing.T) {
	_, errs := check(`type Point = { x: Int, y: Int }

fn main() do
  Point { x: 1, y: 2, z: 3 }
end`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "unknown field")
}

func TestRecordFieldAccess(t *testing.T) {
	_, errs := check(`type Point = { x: Int, y: Int }

fn getX(p: Point): Int do
  p.x
end`)
	expectNoErrors(t, errs)
}

func TestRecordFieldAccessInvalid(t *testing.T) {
	_, errs := check(`type Point = { x: Int, y: Int }

fn bad(p: Point) do
  p.z
end`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "no field")
}

// --- Block expressions ---

func TestBlockExprType(t *testing.T) {
	_, errs := check(`fn main(): Int do
  let x = do
    let a = 1
    let b = 2
    a + b
  end
  x
end`)
	expectNoErrors(t, errs)
}

// --- Return expressions ---

func TestReturnType(t *testing.T) {
	_, errs := check(`fn early(x: Int): Int do
  return x + 1
end`)
	expectNoErrors(t, errs)
}

// --- Function return type checking ---

func TestFnReturnTypeMismatch(t *testing.T) {
	_, errs := check(`fn bad(): Int do
  "hello"
end`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "type mismatch")
}

func TestFnReturnTypeMatch(t *testing.T) {
	_, errs := check(`fn good(): String do
  "hello"
end`)
	expectNoErrors(t, errs)
}

// --- Unary operators ---

func TestUnaryNot(t *testing.T) {
	_, errs := check(`fn negate(b: Bool): Bool do
  !b
end`)
	expectNoErrors(t, errs)
}

func TestUnaryNotMismatch(t *testing.T) {
	_, errs := check(`fn bad(x: Int): Bool do
  !x
end`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "type mismatch")
}

// --- Anonymous functions ---

func TestFnLiteral(t *testing.T) {
	_, errs := check(`fn main() do
  let f = fn(x: Int): Int do
    x + 1
  end
  f(5)
end`)
	expectNoErrors(t, errs)
}

// --- Import calls ---

func TestImportCallDoesNotError(t *testing.T) {
	_, errs := check(`import "fmt"

fn main() do
  fmt.println("hello")
end`)
	expectNoErrors(t, errs)
}

// --- Complex programs ---

func TestFullProgram(t *testing.T) {
	_, errs := check(`type Config = { port: Int, host: String }

fn makeConfig(): Config do
  Config { port: 8080, host: "localhost" }
end

fn getPort(c: Config): Int do
  c.port
end

fn main() do
  let config = makeConfig()
  let port = getPort(config)
  port
end`)
	expectNoErrors(t, errs)
}

// --- Sum types ---

func TestSumTypeVariantConstruction(t *testing.T) {
	_, errs := check(`type Shape =
  | Circle { radius: Float }
  | Rectangle { width: Float, height: Float }

fn main() do
  Circle { radius: 1.0 }
end`)
	expectNoErrors(t, errs)
}

func TestSumTypeUnitVariant(t *testing.T) {
	_, errs := check(`type Option =
  | Some { value: Int }
  | None

fn main() do
  None
end`)
	expectNoErrors(t, errs)
}

func TestSumTypeVariantFieldMismatch(t *testing.T) {
	_, errs := check(`type Shape =
  | Circle { radius: Float }

fn main() do
  Circle { radius: "hello" }
end`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "type mismatch")
}

func TestSumTypeVariantMissingField(t *testing.T) {
	_, errs := check(`type Shape =
  | Rectangle { width: Float, height: Float }

fn main() do
  Rectangle { width: 1.0 }
end`)
	expectOneError(t, errs)
	expectErrorContains(t, errs, "missing field")
}

func TestSumTypeAsParam(t *testing.T) {
	_, errs := check(`type Shape =
  | Circle { radius: Float }
  | Square { side: Float }

fn area(s: Shape): Float do
  0.0
end

fn main() do
  area(Circle { radius: 1.0 })
end`)
	expectNoErrors(t, errs)
}

// --- Error recovery ---

func TestErrorRecoveryNoCascade(t *testing.T) {
	// One type error should not cascade into many
	_, errs := check(`fn main() do
  let x: Int = "oops"
  let y = x + 1
  y
end`)
	// Should have exactly 1 error (the annotation mismatch),
	// not cascading errors for x + 1
	expectOneError(t, errs)
}

// --- Type variable inference ---

func TestTypeInferenceAcrossLet(t *testing.T) {
	_, errs := check(`fn main(): Int do
  let x = 1
  let y = x + 2
  y
end`)
	expectNoErrors(t, errs)
}

func TestMultipleFunctions(t *testing.T) {
	_, errs := check(`fn inc(x: Int): Int do
  x + 1
end

fn dec(x: Int): Int do
  x - 1
end

fn main(): Int do
  let a = inc(5)
  dec(a)
end`)
	expectNoErrors(t, errs)
}
