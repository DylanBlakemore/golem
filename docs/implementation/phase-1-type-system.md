# Phase 1 — Type System Core

**Status:** In Progress
**Depends on:** Phase 0 complete
**Goal:** Sum types, flat pattern matching, exhaustiveness, generics, Result/Option, Go interop.
**Exit Criteria:** Model a domain with ADTs, call Go stdlib, handle errors with `?`.

---

## 1.1 — Sum Types (Algebraic Data Types)

Reference: [type-system.md](../architecture/type-system.md), [code-generation.md](../architecture/code-generation.md)

### Parsing
- [x] Extend parser for sum type declarations:
  ```golem
  type Shape =
    | Circle { radius: Float }
    | Rectangle { width: Float, height: Float }
    | Triangle { base: Float, height: Float }
  ```
- [x] AST nodes: `SumTypeDecl` with list of `Variant` (name + optional fields)
- [x] Support variants with no fields (e.g., `| None`)
- [x] Support `pub`/`priv` visibility on sum type declarations

### Name Resolution
- [x] Register sum type variant constructors as values in module scope
- [x] Variant constructors resolve to their parent sum type
- [x] Error: variant name conflicts with existing declaration

### Type Checking
- [x] Sum type as a `TCon` with variants tracked in type environment
- [x] Variant construction type-checks field types against declaration
- [x] Variant construction produces parent sum type (not variant type)

### Code Generation
- [x] Sum type -> sealed Go interface with unexported marker method:
  ```go
  type Shape interface { isShape() }
  type ShapeCircle struct { Radius float64 }
  func (ShapeCircle) isShape() {}
  type ShapeRectangle struct { Width float64; Height float64 }
  func (ShapeRectangle) isShape() {}
  ```
- [x] Variant construction -> struct literal
- [x] Visibility: `pub` sum type -> exported interface + exported variant structs

### Tests
- [x] Parse sum type declarations (multiple variants, with/without fields)
- [x] Type check variant construction
- [x] Code gen snapshot tests for sum types
- [x] Generated code compiles and passes `go vet`

---

## 1.2 — Flat Pattern Matching

Reference: [pattern-matching.md](../architecture/pattern-matching.md)

### Parsing
- [x] `match` expression with `do`/`end` block:
  ```golem
  match shape do
    | Circle { radius } -> radius * radius
    | Rectangle { width, height } -> width * height
  end
  ```
- [x] Pattern AST nodes: `ConstructorPattern`, `VarPattern`, `WildcardPattern`, `LiteralPattern`, `RecordPattern`
- [x] Match arm: pattern + body expression
- [x] `match` is an expression (has a result type)

### Type Checking
- [x] All match arms must produce the same result type
- [x] Constructor patterns type-checked against sum type variants
- [x] Variable bindings introduced in pattern scope
- [x] Wildcard matches any type
- [x] Literal patterns checked against scrutinee type

### Code Generation (Flat)
- [x] Sum type match -> Go type switch:
  ```go
  switch v := shape.(type) {
  case ShapeCircle:
      result = v.Radius * v.Radius
  case ShapeRectangle:
      result = v.Width * v.Height
  }
  ```
- [x] Expression position: result variable assigned in each branch
- [x] Tail position: `return` in each branch
- [x] Literal match -> Go `switch` statement
- [ ] Bool match -> `if`/`else`

### Tests
- [x] Parse match expressions with various pattern types
- [x] Type check: arm type agreement, pattern variable bindings
- [x] Code gen snapshot tests for flat pattern matching
- [x] Test expression position vs tail position emission

---

## 1.3 — Exhaustiveness Checking

Reference: [pattern-matching.md](../architecture/pattern-matching.md) — Maranget Algorithm

### Implementation
- [x] Pattern matrix representation (rows = arms, columns = sub-positions)
- [x] Type category classification:
  - Finite: sum types, Bool, Option, Result
  - Infinite: Int, Float, String
- [x] Specialization operation: replace pattern column with constructor sub-patterns
- [x] Default matrix operation: keep wildcard/variable rows, remove column
- [x] Recursive exhaustiveness check:
  - Finite types: specialize for each constructor, all must be present
  - Infinite types: require wildcard/variable catch-all
- [x] Missing pattern reconstruction (readable error messages)
- [x] Redundancy detection (unreachable arms -> warning)

### Integration
- [x] Runs after type checking (Phase 5 of pipeline)
- [x] Errors block code generation
- [x] Warnings emitted but don't block

### Tests
- [x] Test exhaustive match on sum types (all variants covered)
- [x] Test non-exhaustive match (missing variant -> error with readable message)
- [x] Test wildcard catch-all satisfies exhaustiveness
- [x] Test redundant arm detection
- [x] Test Bool exhaustiveness (`true` + `false` = exhaustive)
- [x] Test infinite type requires wildcard

---

## 1.4 — Generics

Reference: [type-system.md](../architecture/type-system.md)

### Parsing
- [x] Type parameter syntax: `type Result<T, E> = ...`
- [x] Generic function syntax: `fn map<A, B>(list: List<A>, f: Fn<A, B>): List<B>`
- [x] Generic type application: `List<Int>`, `Result<Config, Error>`

### Type Checking
- [x] `TApp` type kind: `List<Int>` = `TApp(TCon("List"), [TCon("Int")])`
- [x] Type parameter introduction in generic declarations
- [x] Type argument inference at call sites (never written explicitly by user)
- [x] Generalization: `let` bindings and top-level functions get polymorphic type schemes
- [x] Instantiation: fresh type variables per use of a polymorphic binding
- [x] Value restriction: only syntactic values are generalized

### Code Generation
- [x] Generic sum types: all variants carry full type parameter set
  ```go
  type ResultOk[T any, E any] struct { Value T }
  type ResultErr[T any, E any] struct { Error E }
  ```
- [x] Generic functions: Go type parameters
- [x] Emit explicit type arguments at call sites in generated Go

### Tests
- [x] Test generic type declarations
- [x] Test type argument inference at call sites
- [x] Test polymorphic let bindings
- [x] Test value restriction (side-effecting expressions are monomorphic)
- [x] Code gen snapshot tests for generic types and functions

---

## 1.5 — Result<T, E> and Option<T> Built-in Types

Reference: [error-handling.md](../architecture/error-handling.md)

### Implementation
- [x] `Result<T, E>` as built-in sum type:
  ```golem
  type Result<T, E> =
    | Ok { value: T }
    | Err { error: E }
  ```
- [x] `Option<T>` as built-in sum type:
  ```golem
  type Option<T> =
    | Some { value: T }
    | None
  ```
- [x] Pre-registered in type environment (available without import)
- [x] Pattern matchable like any other sum type
- [x] Exhaustiveness checking works on Result and Option

### Code Generation
- [x] Sealed interface pattern (same as user-defined sum types)
- [x] Built-in type definitions emitted in a generated `golem_builtins.go` or inlined per-file

### Tests
- [x] Test Result and Option construction
- [x] Test pattern matching on Result/Option
- [x] Test exhaustiveness checking on Result/Option
- [x] Code gen for Result/Option types

---

## 1.6 — `?` Operator (Error Propagation)

Reference: [error-handling.md](../architecture/error-handling.md)

### Parsing
- [ ] Postfix `?` operator on expressions: `File.read(path)?`
- [ ] Precedence: binds tighter than binary operators

### Type Checking
- [ ] `expr?` requires `expr` to have type `Result<T, E>`
- [ ] Result type of `expr?` is `T` (the unwrapped Ok value)
- [ ] Enclosing function must return `Result<_, E>` with compatible error type
- [ ] Error if `?` used outside a function returning Result

### Desugaring
- [ ] `?` operator hoisting:
  ```golem
  let content = File.read(path)?
  ```
  Desugars to:
  ```golem
  let __tmp = File.read(path)
  let content = match __tmp do
    | Ok { value } -> value
    | Err { error } -> return Err { error: error }
  end
  ```
- [ ] Works in expression position (hoisted to statement level)
- [ ] Supports chaining: `a()?.b()?` — left-to-right with nested match hoisting

### Code Generation
- [ ] Desugared form generates Go `if` with type assertion and early return:
  ```go
  tmp := fileRead(path)
  if err, ok := tmp.(ResultErr[Config, Error]); ok {
      return ResultErr[Config, Error]{Error: err.Error}
  }
  content := tmp.(ResultOk[Config, Error]).Value
  ```

### Tests
- [ ] Test `?` on Result-typed expressions
- [ ] Test `?` chaining
- [ ] Test error: `?` used in non-Result-returning function
- [ ] Test desugaring output
- [ ] Code gen snapshot tests

---

## 1.7 — Go Package Import with Type Mapping

Reference: [go-interop.md](../architecture/go-interop.md)

### Go Package Loader
- [ ] Use `golang.org/x/tools/go/packages.Load()` to load Go package metadata
- [ ] Extract exported type signatures via `go/types`
- [ ] Cache by `(import path, module version)`

### Type Mapper
- [ ] Implement Go -> Golem type mapping:

  | Go Type | Golem Type |
  |---|---|
  | `string` | `String` |
  | `int`, `int8/16/32/64` | `Int` |
  | `uint`, `uint8/16/32/64` | `Int` |
  | `float32`, `float64` | `Float` |
  | `bool` | `Bool` |
  | `[]T` | `List<T>` |
  | `[N]T` | `List<T>` (array -> slice) |
  | `map[K]V` | `Map<K, V>` |
  | `*T` | `Option<T>` (nil -> None) |
  | `chan T` | `Chan<T>` |
  | `interface{}`/`any` | `Any` |
  | `error` | `Error` |
  | Go struct | Golem record (exported fields, lowercased names) |

### Name Mapping
- [ ] Go exported symbols accessed with first letter lowercased in Golem
- [ ] Code gen maps back to capitalized Go names
- [ ] Handle qualified access: `http.listenAndServe` -> `http.ListenAndServe`

### Integration with Name Resolution
- [ ] Import declarations resolved to Go package metadata
- [ ] Qualified identifiers type-checked against Go package signatures

### Tests
- [ ] Test type mapping for each Go type category
- [ ] Test Go stdlib package loading (`fmt`, `net/http`, `os`)
- [ ] Test name mapping (lowercased access -> capitalized output)
- [ ] Test struct field mapping
- [ ] Integration test: call Go stdlib functions from Golem

---

## 1.8 — Auto-Lifting `(T, error)` to `Result<T, Error>`

Reference: [go-interop.md](../architecture/go-interop.md), [error-handling.md](../architecture/error-handling.md)

### Type Mapper Extension
- [ ] Detect Go functions returning `(T, error)` pattern
- [ ] Map return type to `Result<T, Error>` in Golem type system
- [ ] Functions returning only `error` -> `Result<Unit, Error>`

### Code Generation
- [ ] At Go call sites, generate error-checking wrapper:
  ```go
  rawResult, rawErr := os.ReadFile(path)
  var golemResult Result[[]byte, error]
  if rawErr != nil {
      golemResult = ResultErr[[]byte, error]{Error: rawErr}
  } else {
      golemResult = ResultOk[[]byte, error]{Value: rawResult}
  }
  ```
- [ ] Combined with `?` operator for ergonomic usage

### Tests
- [ ] Test `(T, error)` detection in Go function signatures
- [ ] Test auto-lifting for Go stdlib functions (`os.ReadFile`, `os.Open`, etc.)
- [ ] Test `error`-only return mapping
- [ ] End-to-end: Golem calls Go function with `?` and handles errors

---

## 1.9 — End-to-End Integration Test

**Target:** Domain model with ADTs + Go stdlib calls + error handling.

```golem
import "os"
import "fmt"

type FileResult =
  | TextFile { content: String }
  | EmptyFile
  | ReadError { message: String }

pub fn processFile(path: String): String do
  let result = os.readFile(path)
  match result do
    | Ok { value } ->
      if value == "" do
        describeResult(EmptyFile)
      else
        describeResult(TextFile { content: value })
      end
    | Err { error } ->
      describeResult(ReadError { message: error.Error() })
  end
end

priv fn describeResult(r: FileResult): String do
  match r do
    | TextFile { content } -> "Got content: " <> content
    | EmptyFile -> "File was empty"
    | ReadError { message } -> "Error: " <> message
  end
end

pub fn main() do
  fmt.println(processFile("/etc/hosts"))
end
```

- [ ] This compiles without errors
- [ ] Sum type variants generate correctly
- [ ] Pattern matching is exhaustive
- [ ] Go interop types map correctly
- [ ] Error handling works end-to-end
- [ ] All Phase 1 tests pass
