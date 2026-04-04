# Golem Architecture Documentation

This directory contains the detailed architecture and design documents for the Golem compiler and language. These documents serve as the technical blueprint for implementation.

## Documents

| Document | Description |
|---|---|
| [Compiler Pipeline](compiler-pipeline.md) | End-to-end compiler architecture: phases, data structures, and data flow from `.golem` source to `.golem.go` output |
| [Type System](type-system.md) | Type inference algorithm, type representation, generics, and the constraint-based checking approach |
| [Pattern Matching](pattern-matching.md) | Pattern matching compilation, exhaustiveness checking, and decision tree generation |
| [Go Code Generation](code-generation.md) | Code generation strategy, Go encoding of Golem constructs, and output quality guarantees |
| [Go Interoperability](go-interop.md) | Go package import, type mapping, `(T, error)` lifting, and the FFI boundary |
| [Error Handling](error-handling.md) | `Result<T, E>`, the `?` operator, and how error propagation compiles to Go |

## Design Principles

1. **Golem is a lens, not a platform.** The Go runtime, Go modules, and `go build` are the platform. Golem adds expressiveness on top without replacing any of that machinery.

2. **The compiler is the source of truth.** Exhaustiveness, type safety, and visibility are enforced by the Golem compiler. The generated Go need not be independently safe — it is correct by construction.

3. **Generated code is for humans.** A Go developer should be able to read, review, and debug generated `.golem.go` files without understanding Golem. This constrains code generation: no runtime library, no opaque encoding, no clever tricks.

4. **Type inference serves the developer, annotations serve the team.** Full inference within function bodies; explicit annotations on public API surfaces. This is the same trade-off Gleam, Rust, and Haskell make.

5. **One-way interop simplifies everything.** Golem calls Go. Go does not call Golem. This eliminates ABI concerns, export naming conventions, and bidirectional type bridging.
