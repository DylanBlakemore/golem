# Product Requirements Document
## Golem — A Expressive, Go-Transpiled Programming Language

**Version:** 0.1 Draft  
**Status:** Pre-Inception  
**Author:** [TBD]  
**Last Updated:** April 2026

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Problem Statement](#2-problem-statement)
3. [Goals & Non-Goals](#3-goals--non-goals)
4. [Target Users](#4-target-users)
5. [Language Design](#5-language-design)
6. [Go Interoperability](#6-go-interoperability)
7. [Compiler & Toolchain Architecture](#7-compiler--toolchain-architecture)
8. [Developer Experience](#8-developer-experience)
9. [Phased Roadmap](#9-phased-roadmap)
10. [Success Metrics](#10-success-metrics)
11. [Risks & Mitigations](#11-risks--mitigations)
12. [Open Questions](#12-open-questions)

---

## 1. Executive Summary

**Golem** is a statically typed, expression-oriented programming language that transpiles to idiomatic Go source code. It provides algebraic data types, pattern matching, and a cleaner syntax — while giving Golem programs full access to the entire Go ecosystem and standard library. Think of it as what CoffeeScript was to JavaScript: a language that inherits an entire platform's muscle while replacing the parts that frustrate.

The thesis is simple: Go is an excellent platform — fast, concurrent, well-deployed — but its type system and error handling create unnecessary friction. Golem removes that friction without abandoning the platform. Golem is a **one-way lens** onto Go: Golem code can call any Go package freely, but Go calling into Golem is explicitly out of scope. Golem is a new-project or new-service language, not a migration tool for existing Go codebases.

---

## 2. Problem Statement

### 2.1 Go's Strengths Are Real

Go has won in a meaningful slice of infrastructure and backend development. Its concurrency model, compilation speed, deployment story (single binary), and ecosystem are genuinely excellent. Any alternative targeting Go's users must inherit these properties, not replace them.

### 2.2 Go's Pain Points Are Also Real

| Pain Point | Impact |
|---|---|
| No sum types / discriminated unions | Requires interface + struct boilerplate; no exhaustiveness checking |
| Error handling (`if err != nil`) | Repeated at every call site; obscures control flow |
| No pattern matching | Switch statements are limited; type switches are verbose |
| Verbose struct manipulation | No destructuring; no concise record updates |
| No pipe operator | Nested function calls are hard to read |

### 2.3 The Gap This Fills

Existing alternatives either abandon the Go platform entirely (Rust, Zig) or add syntax sugar without addressing type-level expressiveness (some Go preprocessors). No existing tool provides the combination of algebraic types, pattern matching, and seamless Go package consumption in a production-quality transpiler. Golem's scope is deliberately narrow: it is a better language for writing Go-backed services from scratch, not a tool for migrating existing Go code.

---

## 3. Goals & Non-Goals

### 3.1 Goals

- **G1:** Algebraic data types (sum types + product types) with exhaustiveness checking at the Golem compiler level.
- **G2:** First-class pattern matching on all types, including nested destructuring.
- **G3:** Transpile to readable, idiomatic Go source that a Go developer could review, debug, and commit.
- **G4:** Import any Go package from Golem code with no friction.
- **G5:** A `Result<T, E>` type with ergonomic unwrapping at Go stdlib call sites.
- **G6:** `do/end` as the sole block syntax, keeping the grammar simple and unambiguous.
- **G7:** A pipe operator (`|>`) for function composition.
- **G8:** A language server (LSP) providing completion, diagnostics, and go-to-definition.
- **G9:** Explicit `pub`/`priv` visibility keywords. Go's upper/lowercase convention is a code generation detail, invisible to Golem developers.
- **G10:** Multiple function clauses dispatched by arity, enabling clean default-argument patterns and recursive base cases without overloading syntax.

### 3.2 Non-Goals (v1.0)

- **NG1:** A new runtime. Golem produces Go source; the Go runtime is the runtime.
- **NG2:** A new package manager. Golem uses Go modules (`go.mod`) directly.
- **NG3:** Compile-time macros or metaprogramming.
- **NG4:** Hot code reloading or a REPL (deferred to v2).
- **NG5:** Targeting non-Go backends (WASM, JS, LLVM). Go handles WASM already.
- **NG6:** Breaking changes to Go's concurrency model. Goroutines and channels are surfaced as-is.
- **NG7:** Go calling into Golem. Interop is strictly one-directional — Golem calls Go, not the reverse. Golem is a language for writing new services, not for incrementally migrating existing Go code.
- **NG8:** Pattern matching in function heads (Elixir-style). Function clauses are dispatched by arity only. Full pattern matching belongs in `match` expressions inside the body.

---

## 4. Target Users

### 4.1 Primary: Go Developers Frustrated with Boilerplate

Engineers who are already productive in Go but find themselves writing repetitive `if err != nil` chains, wrestling with the lack of sum types, or wishing for more expressive data modeling. They don't want to leave the Go ecosystem — they want a better front-end for it.

**Needs:** Familiar semantics, zero-friction Go interop, readable generated code, no magic.

### 4.2 Secondary: Functional Language Developers Entering Go Ecosystems

Developers coming from Elixir, Haskell, F#, or Rust who need to write a new Go-backed service. They're productive in typed functional paradigms and find Go's type system limiting, but they need the deployment profile and ecosystem Go provides.

**Needs:** Pattern matching, ADTs, Result types. They are starting fresh — Go interop going the other direction is not a concern.

### 4.3 Tertiary: New Projects Choosing a Backend Language

Teams starting a new Go-adjacent backend service who would prefer an expressive language but need the deployment properties (single binary, Docker-friendly, low memory footprint) that Go provides.

**Needs:** Full language feature set, good tooling, confidence that the generated Go binary behaves identically to hand-written Go.

---

## 5. Language Design

### 5.1 Type System

#### Sum Types (Algebraic / Discriminated Unions)

```golem
type Shape =
  | Circle { radius: Float }
  | Rectangle { width: Float, height: Float }
  | Triangle { base: Float, height: Float }
```

Transpiles to Go interface + struct pattern, with generated type tags enabling exhaustive switches.

#### Product Types

Golem structs are Go structs with destructuring support:

```golem
type Point = { x: Float, y: Float }

let origin: Point = { x: 0.0, y: 0.0 }
let { x, y } = origin  -- destructuring
```

#### Generics

Golem generics map directly to Go 1.18+ generics:

```golem
type Result<T, E> =
  | Ok { value: T }
  | Err { error: E }
```

#### Type Inference

Full Hindley-Milner style inference within function bodies. Explicit type annotations required on `pub` functions and `pub` types — the public API surface must be self-documenting. Annotations are optional but encouraged on `priv` functions.

---

### 5.2 Pattern Matching

Pattern matching is the primary conditional mechanism:

```golem
let area = match shape do
  | Circle { radius } -> Float.pi * radius * radius
  | Rectangle { width, height } -> width * height
  | Triangle { base, height } -> 0.5 * base * height
end
```

**Exhaustiveness:** The Golem compiler enforces exhaustiveness on sum type matches. Missing branches are a compile error — the generated Go will not be produced until the match is complete.

**Guard clauses:**

```golem
match user do
  | { age } if age >= 18 -> "adult"
  | { age } -> "minor"
end
```

**Nested destructuring:**

```golem
match response do
  | Ok { value: User { name, role: Admin } } -> grantAccess(name)
  | Ok { value: User { name } } -> denyAccess(name)
  | Err { error } -> logError(error)
end
```

Transpiles to nested Go type switches with intermediate variable binding.

---

### 5.3 Error Handling

Golem introduces `Result<T, E>` as a first-class type. The `?` operator propagates errors in the same style as Rust:

```golem
pub fn readConfig(path: String): Result<Config, IOError> do
  let content = File.read(path)?   -- propagates Err early
  let config = Json.parse(content)?
  Ok { value: config }
end
```

At Go stdlib call sites, Golem auto-wraps `(T, error)` returns into `Result<T, error>`:

```golem
-- Go's os.ReadFile(path) returns ([]byte, error)
-- Golem sees it as Result<Bytes, Error>
let bytes = os.ReadFile(path)?
```

---

### 5.4 Syntax

#### Visibility: `pub` and `priv`

Golem uses explicit keywords for visibility. There is no naming convention — upper or lower case has no semantic meaning in Golem source.

```golem
pub fn greet(name: String): String do
  "Hello, " <> name
end

priv fn formatName(name: String): String do
  String.trim(name) |> String.capitalize
end
```

`priv` is the default — a bare `fn` with no modifier is private. `pub` must be written explicitly. This matches common intuition: public surface area should require a deliberate act.

**Code generation:** The Golem compiler maps `pub` → uppercase Go identifier and `priv` → lowercase. This is entirely a code generation detail. Golem developers never write or think about Go's casing convention.

```golem
-- Golem source
pub fn ServeHTTP ...   -- ❌ unnecessary, pub already handles capitalisation
pub fn serveHTTP ...   -- ✅ generates func ServeHTTP in Go
priv fn helper ...     -- generates func helper in Go
```

The same applies to types:

```golem
pub type User = { name: String, email: String }   -- exported
priv type cacheKey = { host: String, port: Int }  -- unexported
```

---

#### Multiple Function Clauses (Arity Dispatch)

A function may be defined multiple times with different arities. The compiler selects the correct clause at the call site based on argument count. This is resolved entirely at compile time — there is no runtime dispatch.

```golem
-- Base case: no separator
pub fn join(items: List<String>): String do
  join(items, ", ")
end

-- Full form: custom separator
pub fn join(items: List<String>, sep: String): String do
  String.joinWith(items, sep)
end
```

Callers use whichever form fits:

```golem
join(names)           -- calls the 1-argument clause
join(names, " | ")    -- calls the 2-argument clause
```

**Recursive base cases:** Arity dispatch also enables clean recursion patterns without sentinel values:

```golem
-- Entry point (no accumulator exposed)
pub fn sum(nums: List<Int>): Int do
  sum(nums, 0)
end

-- Worker clause (private, carries accumulator)
priv fn sum(nums: List<Int>, acc: Int): Int do
  match nums do
    | [] -> acc
    | [head, ..tail] -> sum(tail, acc + head)
  end
end
```

**Constraints:**
- All clauses of the same name must share the same visibility. Mixing `pub` and `priv` across arities of the same function is a compile error.
- All clauses of the same name must share the same return type.
- Clauses must have distinct arities — two clauses with identical argument counts for the same name is a compile error.
- No pattern matching on argument values in function heads. Use `match` inside the body.

**Code generation:** Each arity clause generates a separate Go function with a disambiguating suffix (`greet_1`, `greet_2`, etc.), and Golem's call sites reference the correct one. The suffix is not visible to the Golem developer.

---

#### Pipe Operator

```golem
let result =
  users
  |> List.filter(fn(u) -> u.active)
  |> List.map(fn(u) -> u.name)
  |> String.join(", ")
```

#### `do/end` Blocks

`do/end` is the sole block syntax in Golem. Curly braces are not supported. This is a deliberate simplification — one block style means one mental model, and it keeps the grammar unambiguous.

```golem
pub fn greet(name: String): String do
  "Hello, " <> name
end
```

The last expression in a block is the return value. Explicit `return` is supported but discouraged.

#### String Interpolation

```golem
let msg = "Hello, #{name}! You are #{age} years old."
```

#### Concurrency

Golem does not abstract over goroutines or channels. They are surfaced with thin syntax sugar:

```golem
go fetchData(url)           -- spawns goroutine
let ch = chan<Int>(10)      -- buffered channel
ch <- 42                    -- send
let val = <-ch              -- receive
```

---

## 6. Go Interoperability

Interop is strictly **one-directional**: Golem calls Go. Go does not call Golem. This simplification removes an entire class of design problems (ABI compatibility, exported symbol naming conventions, generated header files) and lets the team focus on making the inbound direction excellent.

### 6.1 Importing Go Packages

Any Go package is importable in Golem with no wrapper layer:

```golem
import "net/http"
import "github.com/some/library"

let resp = http.Get("https://example.com")?
```

Golem reads Go type signatures from compiled package metadata (via `go/types`) and maps them into the Golem type system at compile time. No hand-written bindings are required.

### 6.2 Auto-lifting Go Conventions

Go's `(T, error)` return convention is automatically lifted to `Result<T, Error>` at every call site. Nil pointer returns are lifted to `Option<T>`. This happens transparently — the Golem developer never sees raw Go error pairs.

```golem
-- os.ReadFile returns ([]byte, error) in Go
-- Golem sees Result<Bytes, Error>
let content = os.ReadFile("/etc/hosts")?
```

### 6.3 Project Structure

A Golem project is a standard Go module. Golem source files live in the project root. The compiler emits `.golem.go` files into a `/build` directory that mirrors the source tree exactly — making it trivial to distinguish Golem source from generated Go at a glance.

```
myproject/
├── go.mod
├── go.sum
├── main.golem
├── handlers/
│   └── handlers.golem
└── build/                        ← generated, committed to repo
    ├── main.golem.go
    └── handlers/
        └── handlers.golem.go
```

The `/build` directory is the boundary between the two languages. Everything above it is Golem; everything inside it is Go. `go build` is pointed at `./build/...` and never touches the source tree directly.

This also makes `.gitignore` hygiene unambiguous — teams that prefer not to commit generated files exclude `/build` in one line.

### 6.4 Type Mapping

| Go Type | Golem Type |
|---|---|
| `string` | `String` |
| `int`, `int64` | `Int` |
| `float64` | `Float` |
| `bool` | `Bool` |
| `[]T` | `List<T>` |
| `map[K]V` | `Map<K, V>` |
| `(T, error)` | `Result<T, Error>` |
| `*T` | `Option<T>` (nil → None) |
| `interface{}` / `any` | `Any` |
| `chan T` | `Chan<T>` |

### 6.5 Escape Hatch

When the type mapper cannot represent a Go type accurately (rare edge cases involving unsafe pointers, C interop, or deeply nested interface embedding), a `@goraw` annotation allows passing a raw Go expression through to generated code unmodified. This is explicitly an escape hatch and emits a compiler warning.

---

## 7. Compiler & Toolchain Architecture

### 7.1 Pipeline

```
.golem source
    ↓
Lexer → Token stream
    ↓
Parser → AST
    ↓
Name Resolution → Resolved AST
    ↓
Type Inference + Checking → Typed AST
    ↓
Exhaustiveness Checker
    ↓
Go Code Generator → .golem.go source
    ↓
go build → Binary
```

### 7.2 Implementation Language

The Golem compiler is written in Go. This is intentional: it makes contributions easier for the target audience, and the compiler can be distributed as a single binary with no external dependencies.

### 7.3 Generated Code Philosophy

Generated Go must be:

- **Readable.** A Go developer reviewing the generated output should understand what it does without studying Golem.
- **Idiomatic.** Passes `go vet` and `staticcheck` clean. Formatted with `gofmt`.
- **Committable.** Teams should be able to commit generated `.golem.go` files to version control and review diffs like any other Go code.
- **Not abstract.** No generated runtime library is required. The generated code uses only the Go standard library and the packages the Golem source explicitly imported.

### 7.4 Incremental Compilation

The compiler caches type-checked ASTs per file. Unchanged files are not re-type-checked. Code generation is only re-run for files with changed or transitively changed ASTs.

---

## 8. Developer Experience

### 8.1 CLI

```bash
golem build             # Compile all .golem files in module
golem check             # Type-check only, no code gen
golem fmt               # Format .golem source files
golem run main.golem    # Build and run
golem test              # Run tests (delegates to go test on generated code)
```

### 8.2 Language Server (LSP)

The `golem-lsp` binary implements the Language Server Protocol. Editors supporting LSP (VS Code, Neovim, Helix, Zed) get:

- Inline type errors
- Exhaustiveness warnings
- Auto-complete for imported Go packages
- Go-to-definition (crosses language boundary — jumps to Go source when applicable)
- Hover types

### 8.3 VS Code Extension

A first-party VS Code extension ships with syntax highlighting, LSP integration, and a "Show Generated Go" command that opens the generated `.golem.go` side-by-side.

### 8.4 Testing

Golem test files use a `test` block syntax:

```golem
test "area of a circle" do
  let shape = Circle { radius: 5.0 }
  assert area(shape) == 78.539816
end
```

These transpile to standard Go test functions (`func TestAreaOfACircle(t *testing.T)`), so `go test ./...` works without any special runner.

---

## 9. Phased Roadmap

### Phase 0 — Bootstrap (Months 1–3)
- Lexer and parser for core syntax
- Basic type inference (monomorphic)
- Go code generation for functions, structs, and basic expressions
- `golem build` CLI (no LSP)

**Exit Criteria:** Can compile a simple HTTP server written in Golem that delegates to `net/http`.

---

### Phase 1 — Type System Core (Months 4–7)
- Sum types with exhaustiveness checking
- Pattern matching (flat, no nesting yet)
- Generics (mapped to Go generics)
- `Result<T, E>` and `?` operator
- Go package import with type mapping

**Exit Criteria:** Can model a domain with ADTs, call Go stdlib, and handle errors ergonomically.

---

### Phase 2 — Full Pattern Matching & Interop (Months 8–11)
- Nested pattern matching with destructuring
- Guard clauses
- Complete Go package import coverage (generics, embedding, variadic functions)
- `@goraw` escape hatch
- Mixed `.golem` project structure support
- `golem fmt` and `golem check`

**Exit Criteria:** A non-trivial production service can be built entirely in Golem, calling Go stdlib and third-party Go packages freely.

---

### Phase 3 — Tooling (Months 12–15)
- `golem-lsp` language server
- VS Code extension
- Incremental compilation
- Test syntax and `go test` integration
- Documentation generator

**Exit Criteria:** End-to-end IDE experience. Public beta release.

---

### Phase 4 — Stabilization & v1.0 (Months 16–18)
- Language spec document
- Comprehensive standard library bindings
- Performance benchmarking of generated code
- Community feedback integration
- v1.0 release with stability guarantees

---

## 10. Success Metrics

| Metric | Target (12 months post-launch) |
|---|---|
| GitHub stars | 2,000+ |
| Compiler test coverage | >85% |
| Generated code passes `go vet` | 100% |
| LSP response time (p99) | <100ms |
| Go packages successfully importable (from top 500 by usage) | >95% |
| Community-authored packages | 10+ |
| Open bugs (severity: high) | <5 |

---

## 11. Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Go type mapping is incomplete or breaks on edge cases | High | High | Extensive property-based testing of the type mapper; `@goraw` escape hatch for truly unmappable types |
| Generated code is unreadable / unmaintainable | Medium | High | Generated code review as part of CI; "Show Generated Go" tooling so developers stay honest |
| Exhaustiveness checking doesn't survive Go generics | Medium | Medium | Limit exhaustiveness checking to non-generic sum types in v1; add generic support in v1.1 |
| Community doesn't adopt due to "yet another language" fatigue | High | High | One-way interop simplifies the pitch: "use any Go package, no Go changes required." Lower bar than a full new ecosystem. |
| Go language changes break the transpiler | Low | Medium | Pin to Go version in `go.mod`; test against Go release candidates in CI |
| nil edge cases in Go APIs cause subtle runtime bugs | Medium | High | Fuzz testing of the type mapper; explicit policy on concrete-typed nil (see Open Questions) |

---

## 12. Open Questions

**Q1: What is the language called?**
Working name is "Golem." Needs trademark search and community input.

**Q2: Should generated `.golem.go` files be committed to version control?**
Recommendation: yes — it makes the repo self-contained and diffs reviewable without requiring every contributor to have the Golem compiler installed. The compiler should support either workflow.

**Q3: How do we handle Go's nil vs Golem's Option<T>?**
The type mapper converts `*T` to `Option<T>`, but Go APIs that return concrete types and set them to nil (anti-pattern but common in the wild) are harder. Needs a defined policy — likely: the type mapper treats any pointer-typed return as `Option<T>`, and concrete nil is surfaced as a runtime `None`. Needs validation.

**Q4: Do we support Golem-to-Golem packages?**
v1 should be Go-module-native: Golem packages are Go modules. Golem-specific package tooling is deferred.

**Q5: What is the license?**
Open source is strongly recommended for adoption. MIT or Apache 2.0 are the conventional choices in the Go ecosystem.

**Q6: Does Golem need its own `go.sum`-equivalent?**
No. Because Golem uses Go modules, `go.sum` covers all dependencies.

---

*This document is a living draft. All syntax examples are illustrative and subject to change during language design phases.*
