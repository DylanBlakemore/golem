# Golem Implementation Plan

**Status:** Not Started
**Last Updated:** 2026-04-04

This is the master implementation plan for the Golem programming language. Each phase has a dedicated document with granular tasks and checklists.

---

## Overview

Golem is a statically typed, expression-oriented language that transpiles to idiomatic Go. The compiler is written in Go and follows an 8-phase pipeline: Lexer -> Parser -> Name Resolution -> Type Inference -> Exhaustiveness Checking -> Desugaring -> Code Generation -> Go Build.

---

## Phase Checklist

### Phase 0 — Bootstrap (Foundation)
> Lexer, parser, basic type checking, code generation for core constructs, CLI skeleton.
> **Exit Criteria:** Compile a simple HTTP server that delegates to `net/http`.

- [x] [Phase 0: Bootstrap](./phase-0-bootstrap.md)
  - [x] 0.1 — Project scaffolding & build infrastructure
  - [x] 0.2 — Lexer
  - [x] 0.3 — Parser
  - [x] 0.4 — Name resolution (basic)
  - [x] 0.5 — Type checking (monomorphic)
  - [x] 0.6 — Desugaring (basic)
  - [x] 0.7 — Code generation (basic)
  - [x] 0.8 — CLI (`golem build`, `golem run`)
  - [x] 0.9 — End-to-end integration test (HTTP server)

---

### Phase 1 — Type System Core
> Sum types, pattern matching (flat), generics, Result/Option, Go package imports with type mapping.
> **Exit Criteria:** Model a domain with ADTs, call Go stdlib, handle errors with `?`.

- [ ] [Phase 1: Type System Core](./phase-1-type-system.md)
  - [x] 1.1 — Sum types (algebraic data types)
  - [x] 1.2 — Flat pattern matching
  - [x] 1.3 — Exhaustiveness checking (Maranget algorithm)
  - [ ] 1.4 — Generics (mapped to Go 1.18+ generics)
  - [ ] 1.5 — Result<T, E> and Option<T> built-in types
  - [ ] 1.6 — `?` operator (error propagation)
  - [ ] 1.7 — Go package import with type mapping
  - [ ] 1.8 — Auto-lifting `(T, error)` to `Result<T, Error>`
  - [ ] 1.9 — End-to-end integration test (ADT domain model + Go stdlib calls)

---

### Phase 2 — Full Pattern Matching & Interop
> Nested patterns, guards, complete Go interop coverage, escape hatch, project structure, formatting.
> **Exit Criteria:** A non-trivial production service built entirely in Golem.

- [ ] [Phase 2: Full Pattern Matching & Interop](./phase-2-pattern-matching-interop.md)
  - [ ] 2.1 — Nested pattern matching with destructuring
  - [ ] 2.2 — Guard clauses in match expressions
  - [ ] 2.3 — List patterns (`[head, ..tail]`, `[]`)
  - [ ] 2.4 — Decision tree compilation for pattern matching
  - [ ] 2.5 — Complete Go interop (generics, embedding, variadics)
  - [ ] 2.6 — `@goraw` escape hatch
  - [ ] 2.7 — Multi-file project structure support
  - [ ] 2.8 — `golem fmt` (formatter)
  - [ ] 2.9 — `golem check` (type-check only)
  - [ ] 2.10 — End-to-end integration test (production-style service)

---

### Phase 3 — Tooling
> Language server, VS Code extension, incremental compilation, test syntax.
> **Exit Criteria:** End-to-end IDE experience. Public beta.

- [ ] [Phase 3: Tooling](./phase-3-tooling.md)
  - [ ] 3.1 — Incremental compilation (caching, fingerprinting)
  - [ ] 3.2 — Language server (`golem-lsp`)
  - [ ] 3.3 — VS Code extension
  - [ ] 3.4 — Test syntax and `go test` integration
  - [ ] 3.5 — `golem test` CLI command
  - [ ] 3.6 — Documentation generator
  - [ ] 3.7 — End-to-end IDE integration test

---

### Phase 4 — Stabilization & v1.0
> Language spec, standard library coverage, performance, community readiness.
> **Exit Criteria:** v1.0 release with stability guarantees.

- [ ] [Phase 4: Stabilization & v1.0](./phase-4-stabilization.md)
  - [ ] 4.1 — Language specification document
  - [ ] 4.2 — Standard library binding coverage
  - [ ] 4.3 — Performance benchmarking
  - [ ] 4.4 — Error message polish
  - [ ] 4.5 — Community infrastructure
  - [ ] 4.6 — v1.0 release

---

## Architecture Reference

The detailed architecture docs live in [`docs/architecture/`](../architecture/):

| Document | Covers |
|---|---|
| [compiler-pipeline.md](../architecture/compiler-pipeline.md) | Full 8-phase pipeline, token types, AST structure, caching |
| [type-system.md](../architecture/type-system.md) | HM inference, unification, generics, type representation |
| [pattern-matching.md](../architecture/pattern-matching.md) | Maranget algorithm, decision trees, code emission |
| [code-generation.md](../architecture/code-generation.md) | GoEmitter, encoding strategies, deterministic output |
| [go-interop.md](../architecture/go-interop.md) | Type mapping, package loading, nil handling, `@goraw` |
| [error-handling.md](../architecture/error-handling.md) | Result/Option types, `?` operator desugaring |

---

## Key Technical Decisions

1. **Compiler in Go** — single binary distribution, contribution-friendly for target audience
2. **Printf-style code gen** — not `go/ast`, piped through `go/format.Source()`
3. **Constraint-based HM inference** — constraint generation, unification with union-find, generalization
4. **Maranget exhaustiveness** — pattern matrix decomposition, decision tree construction
5. **Sealed interfaces for ADTs** — unexported `is<Type>()` marker method
6. **One-way interop** — Golem calls Go via `go/types` package metadata
7. **Desugaring pass before codegen** — keeps code generator simple
