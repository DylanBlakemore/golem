# Phase 0 — Bootstrap

**Status:** Not Started
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
- [ ] Keywords: `fn`, `pub`, `priv`, `let`, `match`, `do`, `end`, `type`, `import`, `if`, `else`, `go`, `return`, `test`, `assert`
- [ ] Literals: integers, floats, strings (with interpolation tracking), booleans
- [ ] Identifiers and type identifiers (capitalized)
- [ ] Operators: `+`, `-`, `*`, `/`, `%`, `==`, `!=`, `<`, `>`, `<=`, `>=`, `&&`, `||`, `!`, `|>`, `->`, `?`, `<>`, `<-`, `=`, `:`, `|`
- [ ] Delimiters: `(`, `)`, `{`, `}`, `[`, `]`, `,`, `.`
- [ ] String interpolation tokens: `#{` open, `}` close
- [ ] Comments: `--` line comments

### Core Implementation
- [ ] `Span` type with byte offsets, line, and column
- [ ] `Token` type carrying kind, literal value, and `Span`
- [ ] Lexer struct with source input, position tracking
- [ ] Newline significance for statement termination
- [ ] String interpolation state machine (track nesting depth)
- [ ] Error recovery: emit `ERROR` token and continue

### Tests
- [ ] Unit tests for each token category
- [ ] Test string interpolation (including nested)
- [ ] Test error recovery on malformed input
- [ ] Test span accuracy (line/column numbers)
- [ ] Snapshot tests for representative Golem source files

---

## 0.3 — Parser

Reference: [compiler-pipeline.md](../architecture/compiler-pipeline.md) Phase 2

### AST Node Definitions (`internal/ast/`)
- [ ] **Declarations**: `FnDecl` (name, params, return type, body, visibility), `TypeDecl` (product types only for Phase 0), `ImportDecl`, `LetDecl`
- [ ] **Expressions**: `IntLit`, `FloatLit`, `StringLit`, `BoolLit`, `Ident`, `BinaryExpr`, `UnaryExpr`, `CallExpr`, `FieldAccessExpr`, `BlockExpr`, `IfExpr`, `StringInterpolation`
- [ ] **Types** (syntax nodes): `NamedType`, `GenericType` (placeholder for later)
- [ ] **Params**: `Param` with name, type annotation
- [ ] All nodes carry `Span`

### Parser Implementation
- [ ] Recursive descent parser struct with token stream, position, error list
- [ ] Pratt parsing for expressions with operator precedence:
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
- [ ] Parse `do`/`end` blocks
- [ ] Parse `fn` declarations with `pub`/`priv` visibility
- [ ] Parse `let` bindings with optional type annotation
- [ ] Parse `if`/`else` expressions
- [ ] Parse `import` declarations (string literal path)
- [ ] Parse product `type` declarations (`type Point = { x: Float, y: Float }`)
- [ ] Parse function calls (including qualified: `http.listenAndServe(...)`)
- [ ] Parse string interpolation expressions
- [ ] Parse pipe operator chains
- [ ] Error recovery: synchronize to next statement boundary on parse error

### Tests
- [ ] Unit tests for each declaration type
- [ ] Unit tests for each expression type
- [ ] Operator precedence tests
- [ ] Error recovery tests (multiple errors per file)
- [ ] Round-trip snapshot tests (parse -> pretty print -> compare)

---

## 0.4 — Name Resolution (Basic)

Reference: [compiler-pipeline.md](../architecture/compiler-pipeline.md) Phase 3

- [ ] Build module scope: collect all top-level declarations (forward references allowed)
- [ ] Resolve local variable references (`let` bindings)
- [ ] Resolve function references (within module)
- [ ] Resolve import declarations (store as unresolved Go package references for now)
- [ ] Resolve qualified identifiers (`http.get` -> import ref + member)
- [ ] Nested scoping for `let` bindings and blocks
- [ ] Error: undefined variable, duplicate declaration
- [ ] Output: `ResolvedAST` with `DeclRef` pointers

### Tests
- [ ] Test forward references between functions
- [ ] Test scope nesting and shadowing
- [ ] Test undefined variable errors
- [ ] Test duplicate declaration errors

---

## 0.5 — Type Checking (Monomorphic)

Reference: [type-system.md](../architecture/type-system.md)

### Type Representation
- [ ] `Type` struct with `TypeKind` enum
- [ ] `TCon` (concrete): Int, Float, String, Bool, Any
- [ ] `TFn` (function): params + return type
- [ ] `TRecord` (product type): named fields
- [ ] `TVar` (type variable): Unbound/Linked, level tracking
- [ ] Union-find with path compression for type variables

### Constraint Generation
- [ ] Literal expressions -> concrete types
- [ ] Variable references -> look up type in environment
- [ ] Function calls -> emit `f == Fn(args) -> freshVar` constraint
- [ ] Binary operators -> emit type constraints for operands and result
- [ ] Let bindings -> infer RHS type, bind in environment
- [ ] Function declarations -> check body type matches return annotation
- [ ] Record construction -> infer field types
- [ ] Field access -> look up field type from record type
- [ ] If/else -> both branches must agree on result type
- [ ] Block -> type of last expression

### Constraint Solving (Unification)
- [ ] Unify two types recursively
- [ ] Type variable unification with occur check
- [ ] Concrete type matching (names must match)
- [ ] Function type unification (params + return)
- [ ] Record type unification (field-by-field)

### Error Recovery
- [ ] On unification failure: record diagnostic, assign `TError` poison type
- [ ] `TError` unifies with anything (prevents cascading errors)

### Tests
- [ ] Test type inference for each expression form
- [ ] Test type error messages
- [ ] Test record field access type checking
- [ ] Test function call type checking
- [ ] Test if/else branch type agreement

---

## 0.6 — Desugaring (Basic)

Reference: [compiler-pipeline.md](../architecture/compiler-pipeline.md) Phase 6

- [ ] Core AST definition (simplified typed AST)
- [ ] Pipe operator: `a |> f(b)` -> `f(a, b)`
- [ ] String interpolation: `"Hello, #{name}"` -> `fmt.Sprintf("Hello, %v", name)`
- [ ] Implicit `priv` on bare `fn` declarations
- [ ] Visibility mapping: `pub` -> capitalize first letter, `priv` -> lowercase

### Tests
- [ ] Test pipe operator desugaring
- [ ] Test string interpolation desugaring
- [ ] Test visibility mapping

---

## 0.7 — Code Generation (Basic)

Reference: [code-generation.md](../architecture/code-generation.md)

### GoEmitter
- [ ] `GoEmitter` struct: `strings.Builder`, indent level, import map
- [ ] Indentation management (indent/dedent helpers)
- [ ] Import collection and deduplication
- [ ] Pipe output through `go/format.Source()` for canonical formatting

### Emission
- [ ] File header: `// Code generated by golem. DO NOT EDIT.`
- [ ] Package declaration (root -> `main`, subdirs -> dir name)
- [ ] Import block generation (sorted alphabetically)
- [ ] Function declarations -> `func` with params and return type
- [ ] Let bindings -> `:=` short variable declarations
- [ ] Literals: int, float, string, bool
- [ ] Binary expressions -> Go operators
- [ ] Function calls (including qualified: `http.ListenAndServe(...)`)
- [ ] String concatenation (`<>`) -> `+` in Go
- [ ] If/else -> Go `if`/`else` (handle expression position with result variable)
- [ ] Product type declarations -> Go `struct`
- [ ] Record construction -> struct literal
- [ ] Field access -> `.` notation
- [ ] Block expressions -> last expression is return value
- [ ] Source mapping comments: `// golem:<file>:<line>`
- [ ] Deterministic output: declaration order matches source, sorted imports

### Output Structure
- [ ] Write to `build/` directory mirroring source tree
- [ ] `.golem.go` extension for generated files

### Tests
- [ ] Snapshot tests: Golem source -> generated Go source
- [ ] Verify generated code compiles with `go build`
- [ ] Verify generated code passes `go vet`
- [ ] Test import collection and deduplication
- [ ] Test expression position handling (result variables)

---

## 0.8 — CLI (`golem build`, `golem run`)

- [ ] CLI entrypoint in `cmd/golem/main.go`
- [ ] `golem build` command:
  - Discover all `.golem` files in module
  - Run full pipeline (lex -> parse -> resolve -> check -> desugar -> codegen)
  - Write generated files to `build/`
  - Invoke `go build ./build/...`
  - Report errors with source locations
- [ ] `golem run <file>` command:
  - `golem build` + execute the binary
- [ ] Error output formatting:
  - File path, line, column
  - Source line with caret pointing to error
  - Error message
- [ ] `--verbose` flag for pipeline timing info

### Tests
- [ ] Integration test: `golem build` on a simple project
- [ ] Integration test: `golem run` produces expected output
- [ ] Test error output formatting

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
