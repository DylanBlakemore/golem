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
	_, errs := check(`type Maybe =
  | Just { value: Int }
  | Nothing

fn main() do
  Nothing
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

// --- Match expression type checking ---

func TestMatchSumTypeConstructors(t *testing.T) {
	_, errs := check(`type Shape =
  | Circle { radius: Float }
  | Rectangle { width: Float, height: Float }

fn area(s: Shape): Float do
  match s do
    | Circle { radius } -> radius * radius
    | Rectangle { width, height } -> width * height
  end
end`)
	expectNoErrors(t, errs)
}

func TestMatchUnitVariants(t *testing.T) {
	_, errs := check(`type Color =
  | Red
  | Green
  | Blue

fn name(c: Color): String do
  match c do
    | Red -> "red"
    | Green -> "green"
    | Blue -> "blue"
  end
end`)
	expectNoErrors(t, errs)
}

func TestMatchArmTypeMismatch(t *testing.T) {
	_, errs := check(`type Shape =
  | Circle { radius: Float }
  | Square { side: Float }

fn area(s: Shape): Float do
  match s do
    | Circle { radius } -> radius * radius
    | Square { side } -> "not a float"
  end
end`)
	if len(errs) == 0 {
		t.Fatal("expected type error for mismatched arm types")
	}
	expectErrorContains(t, errs, "type mismatch")
}

func TestMatchVariableBinding(t *testing.T) {
	_, errs := check(`type Wrapper =
  | Val { inner: Int }

fn unwrap(w: Wrapper): Int do
  match w do
    | Val { inner } -> inner + 1
  end
end`)
	expectNoErrors(t, errs)
}

func TestMatchWildcardPattern(t *testing.T) {
	_, errs := check(`type Color =
  | Red
  | Green
  | Blue

fn isRed(c: Color): String do
  match c do
    | Red -> "yes"
    | _ -> "no"
  end
end`)
	expectNoErrors(t, errs)
}

func TestMatchVarPattern(t *testing.T) {
	_, errs := check(`type Color =
  | Red
  | Green
  | Blue

fn show(c: Color): String do
  match c do
    | Red -> "red"
    | other -> "not red"
  end
end`)
	expectNoErrors(t, errs)
}

// --- Generics ---

func TestGenericSumTypeDecl(t *testing.T) {
	_, errs := check(`type Box<T> =
  | Full { value: T }
  | Empty

fn main() do
  Full { value: 42 }
end`)
	expectNoErrors(t, errs)
}

func TestGenericSumTypeFieldInference(t *testing.T) {
	_, errs := check(`type Box<T> =
  | Full { value: T }
  | Empty

fn main(): Int do
  let b = Full { value: 42 }
  match b do
    | Full { value } -> value
    | Empty -> 0
  end
end`)
	expectNoErrors(t, errs)
}

func TestGenericFunction(t *testing.T) {
	_, errs := check(`fn identity<A>(x: A): A do
  x
end

fn main(): Int do
  identity(42)
end`)
	expectNoErrors(t, errs)
}

func TestGenericFunctionTypeInference(t *testing.T) {
	_, errs := check(`fn identity<A>(x: A): A do
  x
end

fn main(): String do
  identity("hello")
end`)
	expectNoErrors(t, errs)
}

func TestGenericFunctionMultipleTypeParams(t *testing.T) {
	_, errs := check(`fn first<A, B>(a: A, b: B): A do
  a
end

fn main(): Int do
  first(42, "hello")
end`)
	expectNoErrors(t, errs)
}

func TestGenericSumTypeExhaustive(t *testing.T) {
	_, errs := check(`type Box<T> =
  | Full { value: T }
  | Empty

fn unbox(b: Box<Int>): Int do
  match b do
    | Full { value } -> value
    | Empty -> 0
  end
end`)
	expectNoErrors(t, errs)
}

func TestGenericSumTypeNonExhaustive(t *testing.T) {
	_, errs := check(`type Box<T> =
  | Full { value: T }
  | Empty

fn unbox(b: Box<Int>): Int do
  match b do
    | Full { value } -> value
  end
end`)
	if len(errs) == 0 {
		t.Fatal("expected exhaustiveness error")
	}
	expectErrorContains(t, errs, "non-exhaustive")
}

func TestPolymorphicLetBinding(t *testing.T) {
	_, errs := check(`fn main() do
  let id = fn(x: Int): Int do x end
  id(42)
end`)
	expectNoErrors(t, errs)
}

func TestGenericTwoParamSumType(t *testing.T) {
	_, errs := check(`type Either<L, R> =
  | Left { value: L }
  | Right { value: R }

fn main() do
  Left { value: 42 }
end`)
	expectNoErrors(t, errs)
}

func TestGenericMatchTwoParams(t *testing.T) {
	_, errs := check(`type Either<L, R> =
  | Left { value: L }
  | Right { value: R }

fn unwrap(e: Either<Int, String>): Int do
  match e do
    | Left { value } -> value
    | Right { value } -> 0
  end
end`)
	expectNoErrors(t, errs)
}

// --- Built-in Result and Option types ---

func TestBuiltinOptionSomeConstruction(t *testing.T) {
	_, errs := check(`fn main() do
  Some { value: 42 }
end`)
	expectNoErrors(t, errs)
}

func TestBuiltinOptionNoneConstruction(t *testing.T) {
	_, errs := check(`fn main() do
  None
end`)
	expectNoErrors(t, errs)
}

func TestBuiltinOptionPatternMatch(t *testing.T) {
	_, errs := check(`fn unwrap(o: Option<Int>): Int do
  match o do
    | Some { value } -> value
    | None -> 0
  end
end`)
	expectNoErrors(t, errs)
}

func TestBuiltinOptionNonExhaustive(t *testing.T) {
	_, errs := check(`fn unwrap(o: Option<Int>): Int do
  match o do
    | Some { value } -> value
  end
end`)
	if len(errs) == 0 {
		t.Fatal("expected exhaustiveness error")
	}
	expectErrorContains(t, errs, "non-exhaustive")
}

func TestBuiltinResultOkConstruction(t *testing.T) {
	_, errs := check(`fn main() do
  Ok { value: 42 }
end`)
	expectNoErrors(t, errs)
}

func TestBuiltinResultErrConstruction(t *testing.T) {
	_, errs := check(`fn main() do
  Err { error: "something failed" }
end`)
	expectNoErrors(t, errs)
}

func TestBuiltinResultPatternMatch(t *testing.T) {
	_, errs := check(`fn unwrap(r: Result<Int, String>): Int do
  match r do
    | Ok { value } -> value
    | Err { error } -> 0
  end
end`)
	expectNoErrors(t, errs)
}

func TestBuiltinResultNonExhaustive(t *testing.T) {
	_, errs := check(`fn unwrap(r: Result<Int, String>): Int do
  match r do
    | Ok { value } -> value
  end
end`)
	if len(errs) == 0 {
		t.Fatal("expected exhaustiveness error")
	}
	expectErrorContains(t, errs, "non-exhaustive")
}

// --- Error propagation (? operator) ---

func TestErrorPropagationOnResult(t *testing.T) {
	_, errs := check(`fn process(path: String): Result<String, String> do
  let content = readFile(path)?
  Ok { value: content }
end

fn readFile(path: String): Result<String, String> do
  Ok { value: path }
end`)
	expectNoErrors(t, errs)
}

func TestErrorPropagationChained(t *testing.T) {
	_, errs := check(`fn process(path: String): Result<String, String> do
  let content = readFile(path)?
  let parsed = transform(content)?
  Ok { value: parsed }
end

fn readFile(path: String): Result<String, String> do
  Ok { value: path }
end

fn transform(s: String): Result<String, String> do
  Ok { value: s }
end`)
	expectNoErrors(t, errs)
}

func TestErrorPropagationRequiresResult(t *testing.T) {
	_, errs := check(`fn process(n: Int): String do
  let x = n?
  x
end`)
	if len(errs) == 0 {
		t.Fatal("expected type error for ? on non-Result type")
	}
	expectErrorContains(t, errs, "requires Result")
}

func TestErrorPropagationOutsideResultFunction(t *testing.T) {
	_, errs := check(`fn process(path: String): String do
  let content = readFile(path)?
  content
end

fn readFile(path: String): Result<String, String> do
  Ok { value: path }
end`)
	if len(errs) == 0 {
		t.Fatal("expected type error for ? in non-Result-returning function")
	}
	expectErrorContains(t, errs, "not returning Result")
}
