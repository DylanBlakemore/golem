# Go Code Generation

## Overview

The code generator is the final phase of the Golem compiler. It walks the desugared, typed Core AST and emits Go source text. The output is piped through `go/format.Source()` to produce `gofmt`-compliant code.

### Design Constraints

Generated Go must satisfy all of:

1. **Readable** — a Go developer can understand it without knowing Golem.
2. **Idiomatic** — passes `go vet` and `staticcheck` clean.
3. **Committable** — diffs are reviewable. Deterministic output (same input always produces identical output).
4. **Self-contained** — no generated runtime library. Only Go stdlib and explicitly imported packages.

---

## Generation Strategy

### String Builder with `go/format`

The code generator uses a printf-style string builder, not `go/ast` construction. Rationale:

- **`go/ast` is designed for parsing, not generation.** Constructing synthetic AST nodes requires setting `token.Pos` values that have no meaningful source location, and the API is extremely verbose.
- **Template-based generation is fragile.** Complex control flow in templates (nested conditionals, loops) becomes unreadable.
- **Printf-style with `go/format`** is the approach used by `protoc-gen-go`, `stringer`, and most production Go generators. The builder produces syntactically valid Go (with relaxed formatting), then `format.Source()` normalizes whitespace and formatting.

```go
type GoEmitter struct {
    buf     strings.Builder
    indent  int
    imports map[string]string  // import path -> alias
}

func (e *GoEmitter) Line(format string, args ...interface{}) {
    fmt.Fprintf(&e.buf, strings.Repeat("\t", e.indent))
    fmt.Fprintf(&e.buf, format, args...)
    e.buf.WriteByte('\n')
}

func (e *GoEmitter) Block(header string, fn func()) {
    e.Line("%s {", header)
    e.indent++
    fn()
    e.indent--
    e.Line("}")
}
```

### Import Management

The emitter tracks imports as they are referenced during generation. At the end, it inserts the import block at the top of the file. Duplicate imports are deduplicated. Conflicting package names are aliased automatically.

```go
func (e *GoEmitter) QualifiedName(pkgPath, name string) string {
    alias := e.ensureImport(pkgPath)
    return alias + "." + name
}
```

---

## Encoding Golem Constructs in Go

### Sum Types (ADTs)

A Golem sum type becomes a Go sealed interface plus one struct per variant.

```golem
type Shape =
  | Circle { radius: Float }
  | Rectangle { width: Float, height: Float }
  | Triangle { base: Float, height: Float }
```

Generated Go:

```go
type Shape interface {
	isShape()
}

type Circle struct {
	Radius float64
}

func (Circle) isShape() {}

type Rectangle struct {
	Width  float64
	Height float64
}

func (Rectangle) isShape() {}

type Triangle struct {
	Base   float64
	Height float64
}

func (Triangle) isShape() {}
```

**Key details:**
- The interface method `isShape()` is unexported, sealing the interface — no type outside the package can implement it.
- Variant struct names are uppercased for export (following `pub`/`priv` rules).
- Field names are uppercased in Go (Golem's `radius` becomes Go's `Radius`). The Golem developer always writes lowercase; the mapping is transparent.

### Generic Sum Types

```golem
type Result<T, E> =
  | Ok { value: T }
  | Err { error: E }
```

Generated Go:

```go
type Result[T any, E any] interface {
	isResult()
}

type Ok[T any, E any] struct {
	Value T
}

func (Ok[T, E]) isResult() {}

type Err[T any, E any] struct {
	Error E
}

func (Err[T, E]) isResult() {}
```

Note: Both `Ok` and `Err` carry the full set of type parameters from the parent type, even if they don't use all of them. This is required by Go's type system — you cannot have `Ok[T]` implement `Result[T, E]` without `Ok` also being parameterized by `E`. This is an unfortunate verbosity in the generated code, but it is correct and idiomatic Go.

### Product Types (Records)

```golem
type Point = { x: Float, y: Float }
```

Generated Go:

```go
type Point struct {
	X float64
	Y float64
}
```

### Functions

```golem
pub fn area(shape: Shape): Float do
  match shape do
    | Circle { radius } -> 3.14159 * radius * radius
    | Rectangle { width, height } -> width * height
    | Triangle { base, height } -> 0.5 * base * height
  end
end
```

Generated Go:

```go
func Area(shape Shape) float64 {
	switch v := shape.(type) {
	case Circle:
		radius := v.Radius
		return 3.14159 * radius * radius
	case Rectangle:
		width := v.Width
		height := v.Height
		return width * height
	case Triangle:
		base := v.Base
		height := v.Height
		return 0.5 * base * height
	default:
		panic("unreachable: exhaustive match")
	}
}
```

### Visibility Mapping

| Golem | Go |
|---|---|
| `pub fn greet` | `func Greet` |
| `priv fn helper` | `func helper` |
| `fn helper` (bare) | `func helper` (priv is default) |
| `pub type User` | `type User struct` |
| `priv type cacheKey` | `type cacheKey struct` |

The code generator applies Go's casing convention mechanically:
- `pub` → first letter uppercased
- `priv` (or default) → first letter lowercased

For multi-word identifiers, Golem preserves the developer's casing for the rest of the name. `pub fn serveHTTP` becomes `func ServeHTTP` — only the first letter changes.

### Multiple Function Clauses (Arity Dispatch)

```golem
pub fn join(items: List<String>): String do
  join(items, ", ")
end

pub fn join(items: List<String>, sep: String): String do
  String.joinWith(items, sep)
end
```

Generated Go:

```go
func Join(items []string) string {
	return Join_2(items, ", ")
}

func Join_2(items []string, sep string) string {
	return strings.Join(items, sep)
}
```

The desugaring phase renames clauses to `name_N` (where N is the arity) and rewrites call sites to the correct variant. The `pub` name without suffix always points to the "primary" clause (lowest arity) for Go consumers, though this is an internal detail — Golem callers use the original name with the compiler selecting the right arity.

### Pipe Operator

Already desugared before code generation. `a |> f(b) |> g(c)` has become `g(f(a, b), c)` in the Core AST.

### String Interpolation

```golem
let msg = "Hello, #{name}! You are #{age} years old."
```

Generated Go:

```go
msg := fmt.Sprintf("Hello, %v! You are %v years old.", name, age)
```

The code generator uses `%v` as the default format verb. For known types, it could use specific verbs (`%s` for strings, `%d` for ints), but `%v` is simpler and more robust.

### Let Bindings

```golem
let x = 42
let y = x + 1
```

Generated Go:

```go
x := 42
y := x + 1
```

Short variable declarations (`:=`) are used for all local bindings. The Go compiler handles shadowing correctly.

### Destructuring Let

```golem
let { x, y } = point
```

Generated Go:

```go
x := point.X
y := point.Y
```

### Concurrency

```golem
go fetchData(url)
let ch = chan<Int>(10)
ch <- 42
let val = <-ch
```

Generated Go:

```go
go fetchData(url)
ch := make(chan int, 10)
ch <- 42
val := <-ch
```

Thin syntax sugar — the mapping is nearly 1:1.

### Test Blocks

```golem
test "area of a circle" do
  let shape = Circle { radius: 5.0 }
  assert area(shape) == 78.539816
end
```

Generated Go:

```go
func TestAreaOfACircle(t *testing.T) {
	shape := Circle{Radius: 5.0}
	if !(Area(shape) == 78.539816) {
		t.Fatal("assertion failed: area(shape) == 78.539816")
	}
}
```

The test name is converted to PascalCase for the Go function name. Spaces and special characters are removed. The `assert` expression compiles to a `t.Fatal` with the original expression as the message.

---

## Expression Position Handling

Go distinguishes statements from expressions more strictly than Golem. Several Golem constructs are expressions that must become Go statements.

### Strategy: Result Variables

When a Golem expression (like `match` or `if`) is used in expression position, the code generator introduces a result variable:

```golem
let x = if condition do 1 else 2 end
```

Generated Go:

```go
var x int
if condition {
    x = 1
} else {
    x = 2
}
```

### Strategy: Tail Position Returns

When an expression is the last thing in a function body, branches use `return` directly:

```golem
pub fn classify(n: Int): String do
  match n do
    | 0 -> "zero"
    | _ -> "nonzero"
  end
end
```

Generated Go:

```go
func Classify(n int) string {
	if n == 0 {
		return "zero"
	}
	return "nonzero"
}
```

The code generator tracks whether the current expression is in "tail position" and uses `return` when it is. This produces more idiomatic Go than result variables.

### Strategy: Immediately Invoked Function (Rare)

For deeply nested expression-position constructs where result variables would be awkward, the code generator may emit an immediately-invoked function literal. This is a last resort — it produces valid but less readable code.

```go
x := func() int {
    if condition {
        return 1
    }
    return 2
}()
```

---

## File Structure

Each `.golem` source file produces one `.golem.go` file in the `build/` directory.

### Generated File Layout

```go
// Code generated by golem. DO NOT EDIT.
// Source: ../main.golem

package main

import (
    "fmt"
    "net/http"
)

// --- Type Definitions ---

type Shape interface { ... }
type Circle struct { ... }
// ...

// --- Functions ---

func Area(shape Shape) float64 { ... }
// ...
```

The `// Code generated by golem. DO NOT EDIT.` comment is the standard Go convention for generated files. It tells editors and tools (like `go vet`) to treat the file differently.

### Package Mapping

Golem files in the project root generate code in `package main`. Golem files in subdirectories generate code in a package named after the directory (matching Go convention).

```
myproject/
  main.golem           -> build/main.golem.go         (package main)
  handlers/
    handlers.golem     -> build/handlers/handlers.golem.go  (package handlers)
```

---

## Deterministic Output

The code generator produces deterministic output for the same input. This means:
- Declaration order in the output matches declaration order in the source.
- Import order is sorted alphabetically.
- No random or time-dependent content in the output.
- Map iteration (if used internally) is sorted by key.

Determinism ensures that re-running the compiler on unchanged source produces byte-identical output, which prevents spurious diffs and unnecessary `go build` invalidation.

---

## Source Mapping

The generated Go files include comments that map back to Golem source locations. These are used for:
- Mapping `go build` errors back to Golem source (when the code generator has a bug).
- Enabling Go debuggers to step through generated code with Golem source context.

```go
// golem:main.golem:15
func Area(shape Shape) float64 {
```

The format is `// golem:<file>:<line>`. These comments are lightweight and do not affect `go vet` or `gofmt`.
