# Phase 0 — Bootstrap

**Status:** In Progress
**Goal:** Lexer, parser, basic type checking, code generation for core constructs, CLI skeleton.
**Exit Criteria:** Compile a simple HTTP server written in Golem that delegates to `net/http`.

---

## 0.1 — Project Scaffolding & Build Infrastructure

- [x] Initialize Go module (`go mod init`)
- [x] Set up directory structure:
  ```
  cmd/golem/         — CLI entrypoint
  internal/
    lexer/           — tokenizer
    parser/          — recursive descent parser
    ast/             — AST node definitions
    resolver/        — name resolution
    checker/         — type inference & checking
    desugar/         — desugaring pass
    codegen/         — Go code emitter
    diagnostic/      — error reporting
    span/            — source location tracking
  ```
- [x] Set up test infrastructure (`go test ./...`)
- [x] Set up CI (GitHub Actions: build, test, lint)
- [x] Add `Makefile` with `build`, `test`, `lint` targets

---

## 0.2 — Lexer

Reference: [compiler-pipeline.md](../architecture/compiler-pipeline.md) Phase 1

### Token Types
- [x] Keywords: `fn`, `pub`, `priv`, `let`, `match`, `do`, `end`, `type`, `import`, `if`, `else`, `go`, `return`, `test`, `assert`
- [x] Literals: integers, floats, strings (with interpolation tracking), booleans
- [x] Identifiers and type identifiers (capitalized)
- [x] Operators: `+`, `-`, `*`, `/`, `%`, `==`, `!=`, `<`, `>`, `<=`, `>=`, `&&`, `||`, `!`, `|>`, `->`, `?`, `<>`, `<-`, `=`, `:`, `|`
- [x] Delimiters: `(`, `)`, `{`, `}`, `[`, `]`, `,`, `.`
- [x] String interpolation tokens: `#{` open, `}` close
- [x] Comments: `--` line comments

### Core Implementation
- [x] `Span` type with byte offsets, line, and column
- [x] `Token` type carrying kind, literal value, and `Span`
- [x] Lexer struct with source input, position tracking
- [x] Newline significance for statement termination
- [x] String interpolation state machine (track nesting depth)
- [x] Error recovery: emit `ERROR` token and continue

### Tests
- [x] Unit tests for each token category
- [x] Test string interpolation (including nested)
- [x] Test error recovery on malformed input
- [x] Test span accuracy (line/column numbers)
- [x] Snapshot tests for representative Golem source files

---

## 0.3 — Parser

Reference: [compiler-pipeline.md](../architecture/compiler-pipeline.md) Phase 2

### AST Node Definitions (`internal/ast/`)
- [x] **Declarations**: `FnDecl` (name, params, return type, body, visibility), `TypeDecl` (product types only for Phase 0), `ImportDecl`, `LetDecl`
- [x] **Expressions**: `IntLit`, `FloatLit`, `StringLit`, `BoolLit`, `Ident`, `BinaryExpr`, `UnaryExpr`, `CallExpr`, `FieldAccessExpr`, `BlockExpr`, `IfExpr`, `StringInterpolation`
- [x] **Types** (syntax nodes): `NamedType`, `GenericType` (placeholder for later)
- [x] **Params**: `Param` with name, type annotation
- [x] All nodes carry `Span`

### Parser Implementation
- [x] Recursive descent parser struct with token stream, position, error list
- [x] Pratt parsing for expressions with operator precedence:
  - Lowest: `|>` (pipe)
  - `||` (logical or)
  - `&&` (logical and)
  - `==`, `!=` (equality)
  - `<`, `>`, `<=`, `>=` (comparison)
  - `<>` (string concatenation)
  - `+`, `-` (additive)
  - `*`, `/`, `%` (multiplicative)
  - Unary: `!`, `-`
  - Highest: field access (`.`), function call (`(`)
- [x] Parse `do`/`end` blocks
- [x] Parse `fn` declarations with `pub`/`priv` visibility
- [x] Parse `let` bindings with optional type annotation
- [x] Parse `if`/`else` expressions
- [x] Parse `import` declarations (string literal path)
- [x] Parse product `type` declarations (`type Point = { x: Float, y: Float }`)
- [x] Parse function calls (including qualified: `http.listenAndServe(...)`)
- [x] Parse string interpolation expressions
- [x] Parse pipe operator chains
- [x] Error recovery: synchronize to next statement boundary on parse error

### Tests
- [x] Unit tests for each declaration type
- [x] Unit tests for each expression type
- [x] Operator precedence tests
- [x] Error recovery tests (multiple errors per file)
- [x] Round-trip snapshot tests (parse -> pretty print -> compare)

---

## 0.4 — Name Resolution (Basic)

Reference: [compiler-pipeline.md](../architecture/compiler-pipeline.md) Phase 3

- [x] Build module scope: collect all top-level declarations (forward references allowed)
- [x] Resolve local variable references (`let` bindings)
- [x] Resolve function references (within module)
- [x] Resolve import declarations (store as unresolved Go package references for now)
- [x] Resolve qualified identifiers (`http.get` -> import ref + member)
- [x] Nested scoping for `let` bindings and blocks
- [x] Error: undefined variable, duplicate declaration
- [x] Output: `ResolvedAST` with `DeclRef` pointers

### Tests
- [x] Test forward references between functions
- [x] Test scope nesting and shadowing
- [x] Test undefined variable errors
- [x] Test duplicate declaration errors

---

## 0.5 — Type Checking (Monomorphic)

Reference: [type-system.md](../architecture/type-system.md)

### Type Representation
- [x] `Type` struct with `TypeKind` enum
- [x] `TCon` (concrete): Int, Float, String, Bool, Any
- [x] `TFn` (function): params + return type
- [x] `TRecord` (product type): named fields
- [x] `TVar` (type variable): Unbound/Linked, level tracking
- [x] Union-find with path compression for type variables

### Constraint Generation
- [x] Literal expressions -> concrete types
- [x] Variable references -> look up type in environment
- [x] Function calls -> emit `f == Fn(args) -> freshVar` constraint
- [x] Binary operators -> emit type constraints for operands and result
- [x] Let bindings -> infer RHS type, bind in environment
- [x] Function declarations -> check body type matches return annotation
- [x] Record construction -> infer field types
- [x] Field access -> look up field type from record type
- [x] If/else -> both branches must agree on result type
- [x] Block -> type of last expression

### Constraint Solving (Unification)
- [x] Unify two types recursively
- [x] Type variable unification with occur check
- [x] Concrete type matching (names must match)
- [x] Function type unification (params + return)
- [x] Record type unification (field-by-field)

### Error Recovery
- [x] On unification failure: record diagnostic, assign `TError` poison type
- [x] `TError` unifies with anything (prevents cascading errors)

### Tests
- [x] Test type inference for each expression form
- [x] Test type error messages
- [x] Test record field access type checking
- [x] Test function call type checking
- [x] Test if/else branch type agreement

---

## 0.6 — Desugaring (Basic)

Reference: [compiler-pipeline.md](../architecture/compiler-pipeline.md) Phase 6

- [x] Core AST definition (simplified typed AST)
- [x] Pipe operator: `a |> f(b)` -> `f(a, b)`
- [x] String interpolation: `"Hello, #{name}"` -> `fmt.Sprintf("Hello, %v", name)`
- [x] Implicit `priv` on bare `fn` declarations
- [x] Visibility mapping: `pub` -> capitalize first letter, `priv` -> lowercase

### Tests
- [x] Test pipe operator desugaring
- [x] Test string interpolation desugaring
- [x] Test visibility mapping

---

## 0.7 — Code Generation (Basic)

Reference: [code-generation.md](../architecture/code-generation.md)

### GoEmitter
- [x] `GoEmitter` struct: `strings.Builder`, indent level, import map
- [x] Indentation management (indent/dedent helpers)
- [x] Import collection and deduplication
- [x] Pipe output through `go/format.Source()` for canonical formatting

### Emission
- [x] File header: `// Code generated by golem. DO NOT EDIT.`
- [x] Package declaration (root -> `main`, subdirs -> dir name)
- [x] Import block generation (sorted alphabetically)
- [x] Function declarations -> `func` with params and return type
- [x] Let bindings -> `:=` short variable declarations
- [x] Literals: int, float, string, bool
- [x] Binary expressions -> Go operators
- [x] Function calls (including qualified: `http.ListenAndServe(...)`)
- [x] String concatenation (`<>`) -> `+` in Go
- [x] If/else -> Go `if`/`else` (handle expression position with result variable)
- [x] Product type declarations -> Go `struct`
- [x] Record construction -> struct literal
- [x] Field access -> `.` notation
- [x] Block expressions -> last expression is return value
- [ ] Source mapping comments: `// golem:<file>:<line>`
- [x] Deterministic output: declaration order matches source, sorted imports

### Output Structure
- [x] Write to `build/` directory mirroring source tree
- [x] `.golem.go` extension for generated files

### Tests
- [x] Snapshot tests: Golem source -> generated Go source
- [x] Verify generated code compiles with `go build`
- [x] Verify generated code passes `go vet`
- [x] Test import collection and deduplication
- [x] Test expression position handling (result variables)

---

## 0.8 — CLI (`golem build`, `golem run`)

- [x] CLI entrypoint in `cmd/golem/main.go`
- [x] `golem build` command:
  - Discover all `.golem` files in module
  - Run full pipeline (lex -> parse -> resolve -> check -> desugar -> codegen)
  - Write generated files to `build/`
  - Invoke `go build ./build/...`
  - Report errors with source locations
- [x] `golem run <file>` command:
  - `golem build` + execute the binary
- [x] Error output formatting:
  - File path, line, column
  - Source line with caret pointing to error
  - Error message
- [x] `--verbose` flag for pipeline timing info

### Tests
- [x] Integration test: `golem build` on a simple project
- [x] Integration test: `golem run` produces expected output
- [x] Test error output formatting

---

## 0.9 — End-to-End Integration Test

**Target:** Compile and run a simple HTTP server written in Golem.

```golem
import "net/http"
import "fmt"

pub fn main() do
  http.handleFunc("/", fn(w: http.ResponseWriter, r: http.Request) do
    fmt.fprintln(w, "Hello from Golem!")
  end)
  fmt.println("Listening on :8080")
  http.listenAndServe(":8080", nil)
end
```

- [ ] This compiles without errors
- [ ] Generated Go code is readable and idiomatic
- [ ] Generated code passes `go vet`
- [ ] Server runs and responds to HTTP requests
- [ ] All Phase 0 tests pass
