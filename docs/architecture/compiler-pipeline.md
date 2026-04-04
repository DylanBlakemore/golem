# Compiler Pipeline

## Overview

The Golem compiler is a multi-phase transpiler written in Go. It transforms `.golem` source files into idiomatic `.golem.go` files, then delegates to `go build` for final compilation. Each phase produces a well-defined intermediate representation consumed by the next.

```
.golem source
    |
    v
[1. Lexer] --> Token stream
    |
    v
[2. Parser] --> Untyped AST
    |
    v
[3. Name Resolution] --> Resolved AST (names bound to declarations)
    |
    v
[4. Type Inference & Checking] --> Typed AST (every node has a type)
    |
    v
[5. Exhaustiveness Checker] --> Validated Typed AST
    |
    v
[6. Desugaring] --> Core AST (pipes, ?, arity dispatch expanded)
    |
    v
[7. Go Code Generation] --> .golem.go source text
    |
    v
[8. go build] --> Binary
```

---

## Phase 1: Lexer

The lexer converts raw source text into a stream of tokens with source positions.

### Token Types

```
// Keywords
FN, PUB, PRIV, LET, MATCH, DO, END, IF, ELSE, TYPE, IMPORT,
TEST, ASSERT, GO, CHAN, RETURN

// Literals
INT_LIT, FLOAT_LIT, STRING_LIT, BOOL_LIT

// Identifiers
IDENT, UPPER_IDENT  (constructors and type names start uppercase)

// Operators
PIPE,        // |>
ARROW,       // ->
FAT_ARROW,   // =>
QUESTION,    // ?
DOT,         // .
DOUBLE_DOT,  // ..
LANGLE,      // <
RANGLE,      // >
CHAN_SEND,    // <-
PIPE_CHAR,   // |  (in match arms)
EQUALS,      // =
DOUBLE_EQ,   // ==
BANG_EQ,     // !=
LT, GT, LTE, GTE,
PLUS, MINUS, STAR, SLASH, PERCENT,
CONCAT,      // <>
HASH_LBRACE, // #{  (interpolation start)

// Delimiters
LPAREN, RPAREN, LBRACE, RBRACE, LBRACKET, RBRACKET,
COMMA, COLON, NEWLINE

// Special
COMMENT, EOF
```

### Source Positions

Every token carries a `Span` recording byte offsets and line/column positions. These propagate through all subsequent phases for error reporting and LSP integration.

```go
type Span struct {
    File   string
    Start  Position  // byte offset, line, column
    End    Position
}
```

### Design Decisions

- **Newline significance**: Newlines are tokens, used by the parser for statement termination (similar to Go). Semicolons are not part of the language.
- **String interpolation**: The lexer emits `STRING_LIT` for plain segments and `HASH_LBRACE` / `RBRACE` around interpolated expressions. The parser reassembles these into an interpolation AST node.
- **Comments**: Single-line comments use `--`. Comments are preserved as tokens for the formatter but stripped before parsing.

---

## Phase 2: Parser

A recursive-descent parser with Pratt parsing for expressions. Produces an untyped AST — the tree structure is complete but no type information is attached.

### Grammar Highlights

```
module       = import* declaration*
declaration  = type_decl | fn_decl | test_decl
type_decl    = "type" UPPER_IDENT type_params? "=" type_body
type_body    = sum_type | record_type
sum_type     = ("|" UPPER_IDENT record_type?)+
record_type  = "{" (IDENT ":" type_expr ",")* "}"
fn_decl      = visibility? "fn" IDENT "(" params ")" (":" type_expr)? block
visibility   = "pub" | "priv"
block        = "do" expr* "end" | "{" expr* "}"
match_expr   = "match" expr block_start match_arm+ block_end
match_arm    = "|" pattern guard? "->" expr
guard        = "if" expr
pattern      = constructor_pat | record_pat | list_pat | literal_pat | wildcard | bind
```

### AST Node Structure

All AST nodes embed a `Span` for error reporting. The untyped AST uses `Expr` and `Pattern` as the top-level node types.

```go
type UntypedModule struct {
    File     string
    Imports  []Import
    Decls    []Declaration
}

type Declaration interface{ declNode() }

type FnDecl struct {
    Span       Span
    Visibility Visibility  // Pub, Priv, Default
    Name       string
    Params     []Param
    ReturnType *TypeExpr   // nil if omitted
    Body       []Expr
}

type TypeDecl struct {
    Span       Span
    Visibility Visibility
    Name       string
    TypeParams []string
    Body       TypeBody    // SumType or RecordType
}
```

### Operator Precedence (Pratt Parser)

From lowest to highest binding power:

| Precedence | Operators | Associativity |
|---|---|---|
| 1 | `\|>` (pipe) | Left |
| 2 | `\|\|` (or) | Left |
| 3 | `&&` (and) | Left |
| 4 | `==`, `!=` | Left |
| 5 | `<`, `>`, `<=`, `>=` | Left |
| 6 | `<>` (concat) | Left |
| 7 | `+`, `-` | Left |
| 8 | `*`, `/`, `%` | Left |
| 9 | Unary `-`, `!` | Prefix |
| 10 | `.` (field access), `()` (call) | Left |

### Error Recovery

The parser collects multiple errors rather than aborting on the first failure. On encountering an unexpected token, it synchronizes by scanning to the next statement boundary (newline at top-level indentation or a keyword like `fn`, `type`, `let`). This enables the LSP to report multiple diagnostics per file.

---

## Phase 3: Name Resolution

Walks the untyped AST and binds every identifier to its declaration site. This phase:

1. **Builds the module scope** — registers all top-level declarations (types, functions) before resolving any bodies. This allows forward references.
2. **Resolves imports** — maps import paths to loaded Go package type information (via `go/types`) or to other Golem module scopes.
3. **Resolves local scopes** — walks function bodies, creating nested scopes for `let` bindings, `match` arms, and blocks. Detects undefined variables and shadowing.
4. **Groups function clauses** — multiple `fn` declarations with the same name but different arities are collected into a `FnClauseGroup`. Validates that all clauses share the same visibility and return type annotation.
5. **Registers constructors** — sum type variant constructors (e.g., `Circle`, `Ok`) are registered as values in the module scope so they can be used in expressions and patterns.

### Output

A `ResolvedAST` where every `Ident` node carries a reference to its `Declaration` (a pointer or unique ID), and every import is resolved to either a `GoPackage` or a `GolemModule`.

```go
type ResolvedIdent struct {
    Name string
    Span Span
    Ref  DeclRef  // points to the declaration this name refers to
}

type DeclRef struct {
    Kind    DeclKind  // Local, Module, GoPackage, Constructor, TypeParam
    DeclID  uint64    // unique within the compilation
}
```

---

## Phase 4: Type Inference and Checking

The heart of the compiler. Uses a constraint-based Hindley-Milner algorithm with bidirectional checking at function boundaries.

This phase is documented in detail in [type-system.md](type-system.md).

### Summary

1. **Constraint generation**: Walk the resolved AST, emitting type equality constraints for every expression.
2. **Constraint solving**: Unify constraints using a union-find structure. Detect and report type errors.
3. **Generalization**: At `let` bindings, generalize unconstrained type variables into polymorphic type schemes.
4. **Output**: A `TypedAST` where every expression node carries its resolved `Type`.

### Interaction with Go Types

When a resolved identifier refers to a Go package symbol, the type checker reads its Go type from the cached `go/types` information and maps it into Golem's type representation (see [go-interop.md](go-interop.md)).

---

## Phase 5: Exhaustiveness Checker

Runs over every `match` expression in the typed AST. Uses the Maranget algorithm to verify that all possible values of the scrutinee type are covered by the match arms.

This phase is documented in detail in [pattern-matching.md](pattern-matching.md).

### Summary

- Builds a pattern matrix from the match arms.
- Recursively decomposes the matrix by constructor.
- Reports missing patterns as compile errors.
- Reports redundant (unreachable) arms as warnings.

---

## Phase 6: Desugaring

Transforms the typed AST into a simpler "Core AST" by expanding syntactic sugar. This phase exists to keep the code generator simple — it only needs to handle core constructs.

### Desugaring Passes

| Sugar | Desugared Form |
|---|---|
| `a \|> f(b)` | `f(a, b)` |
| `expr?` | `let __tmp = expr; if __tmp is Err { return Err(__tmp.error) }; __tmp.value` |
| `"hello #{name}"` | `String.concat("hello ", String.from(name))` |
| Multi-arity `fn foo` | Renamed to `foo_1`, `foo_2`, etc. with call sites rewritten |
| `do/end` blocks | Normalized to a single block representation |
| Implicit `priv` | Resolved to explicit `Priv` visibility |
| `let { x, y } = point` | Destructuring expanded to field access |

### The `?` Operator Desugaring (Detail)

The `?` operator is the most complex desugaring because it introduces control flow (early return) into expression position.

The desugarer walks expressions bottom-up. When it encounters `expr?`:

1. Lifts `expr` to a `let` binding with a fresh temporary name.
2. Inserts an `if` check: if the result is an `Err` variant, emit an early return wrapping the error.
3. Replaces the `?` node with a reference to the unwrapped value.

This hoisting is necessary because Go cannot express early returns inside expressions. All `?` sites become statement-level checks in the desugared form.

---

## Phase 7: Go Code Generation

Walks the core AST and emits Go source text. Uses a printf-style string builder (not `go/ast` construction) piped through `go/format.Source()` for consistent formatting.

This phase is documented in detail in [code-generation.md](code-generation.md).

---

## Phase 8: Go Build

The compiler invokes `go build ./build/...` as a subprocess. This is a straightforward delegation — Golem produces Go source and Go compiles it.

### Error Mapping

If `go build` fails (which should be rare if the code generator is correct), the compiler attempts to map Go error locations back to Golem source locations using the generated source map metadata (comments or a sidecar mapping file). This is best-effort — the primary error reporting path is through the Golem type checker.

---

## Incremental Compilation

The compiler caches intermediate results to avoid redundant work on subsequent builds.

### Cache Structure

```
.golem-cache/
  <file-hash>/
    tokens.bin        # serialized token stream
    ast.bin           # serialized typed AST
    fingerprint       # hash of the file's exported type signatures
```

### Invalidation Strategy

1. **Content hashing**: Compute SHA-256 of each `.golem` source file. If the hash matches the cache, skip re-parsing and re-type-checking.
2. **Signature fingerprinting**: After type checking, compute a fingerprint of each file's exported declarations (function signatures, type definitions). If a dependency's fingerprint is unchanged, dependents do not need re-type-checking even if the dependency's internal implementation changed.
3. **Go package caching**: Cache `go/types` load results keyed by `(import path, module version)`. Invalidate only when `go.mod` or `go.sum` changes.
4. **Transitive invalidation**: If a file's fingerprint changes, all files that import it are marked for re-checking. This forms a DAG — the compiler processes it in topological order, stopping propagation when a fingerprint is stable.

### Code Generation

Code generation is re-run only for files whose typed AST (post-desugaring) differs from the cached version. The output `.golem.go` file is written only if its content would change, avoiding unnecessary `go build` invalidation.

---

## Error Reporting

All phases accumulate errors into a shared `Diagnostics` collection. Each diagnostic carries:

- A `Severity` (Error, Warning, Hint)
- A primary `Span` (the source location of the problem)
- A message string
- Optional secondary spans with labels (e.g., "expected this type because of ..." pointing to a function signature)
- An optional fix suggestion

Diagnostics are rendered to the terminal with source context (the relevant line with an underline caret) and colors. The same diagnostics are served via the LSP for IDE integration.

The compiler continues through all phases even in the presence of errors (where safe to do so), collecting as many diagnostics as possible in a single run. Type checking uses a "poison" type for expressions with errors — this prevents cascading errors from a single root cause.
