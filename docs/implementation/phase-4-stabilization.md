# Phase 4 — Stabilization & v1.0

**Status:** Not Started
**Depends on:** Phase 3 complete
**Goal:** Language spec, stdlib coverage, performance, community readiness.
**Exit Criteria:** v1.0 release with stability guarantees.

---

## 4.1 — Language Specification Document

- [ ] Formal grammar (BNF/EBNF)
- [ ] Type system specification:
  - Type syntax and semantics
  - Inference rules
  - Subtyping (if any)
  - Generics and type parameter constraints
- [ ] Pattern matching semantics:
  - Pattern syntax
  - Exhaustiveness rules
  - Guard clause semantics
  - Evaluation order
- [ ] Expression semantics:
  - Evaluation rules for each expression form
  - Expression vs statement position
  - Block evaluation (last expression is result)
- [ ] Declaration semantics:
  - Function declarations and arity dispatch
  - Type declarations (product and sum)
  - Let bindings
  - Import resolution
- [ ] Visibility rules (`pub`/`priv`)
- [ ] Go interop specification:
  - Type mapping rules (complete table)
  - Name mapping rules
  - `(T, error)` auto-lifting rules
  - `@goraw` semantics
- [ ] Concurrency primitives (`go`, channels)
- [ ] Module system and project structure

---

## 4.2 — Standard Library Binding Coverage

- [ ] Audit top 50 most-used Go stdlib packages for type mapping accuracy
- [ ] Test type mapper against top 500 Go packages by usage (target: >95% success)
- [ ] Document known incompatibilities and workarounds
- [ ] Fix type mapping edge cases found during audit:
  - [ ] Complex struct embedding patterns
  - [ ] Interfaces with generic methods
  - [ ] Packages using `unsafe`
  - [ ] C-interop packages (`cgo`)
- [ ] Create Golem-idiomatic wrappers for commonly used Go patterns:
  - [ ] HTTP server setup
  - [ ] JSON encoding/decoding
  - [ ] File I/O
  - [ ] String manipulation
  - [ ] Concurrency patterns

---

## 4.3 — Performance Benchmarking

### Compiler Performance
- [ ] Benchmark: full build time on projects of varying size (100, 1K, 10K lines)
- [ ] Benchmark: incremental build time (single file change)
- [ ] Benchmark: type checking time
- [ ] Benchmark: code generation time
- [ ] Target: < 1s full build for 1K-line project
- [ ] Profile and optimize hot paths

### Generated Code Performance
- [ ] Benchmark generated Go vs hand-written Go for equivalent programs
- [ ] Sum type dispatch overhead (interface method call vs direct)
- [ ] Pattern matching overhead (nested type switches vs hand-written logic)
- [ ] Result type overhead (interface allocation vs raw error return)
- [ ] Target: < 5% overhead vs hand-written Go for equivalent programs
- [ ] Document performance characteristics

### LSP Performance
- [ ] Benchmark: diagnostics response time (p50, p95, p99)
- [ ] Benchmark: completion response time
- [ ] Target: p99 < 100ms for all operations
- [ ] Profile and optimize if needed

---

## 4.4 — Error Message Polish

- [ ] Audit all error messages for clarity and helpfulness
- [ ] Add "did you mean?" suggestions for typos in:
  - [ ] Variable names
  - [ ] Function names
  - [ ] Type names
  - [ ] Import paths
- [ ] Improve type mismatch errors:
  - [ ] Show both expected and actual types
  - [ ] Point to the source of the expected type (e.g., function signature)
  - [ ] Suggest fixes where possible
- [ ] Improve exhaustiveness error messages:
  - [ ] Show missing patterns as Golem code examples
  - [ ] Suggest adding wildcard catch-all
- [ ] Add notes/hints for common mistakes:
  - [ ] Using `?` in non-Result function
  - [ ] Accessing private member from another module
  - [ ] Missing type annotation on `pub` function
- [ ] Source snippets in error output:
  ```
  error[E0308]: type mismatch
   --> handlers.golem:15:12
     |
  15 |   let x: Int = "hello"
     |            --- ^^^^^^^ expected Int, found String
     |            |
     |            expected due to this annotation
  ```

---

## 4.5 — Community Infrastructure

- [ ] Project website with:
  - [ ] Getting started guide
  - [ ] Language tour (interactive examples)
  - [ ] Installation instructions
  - [ ] API reference (generated docs)
- [ ] GitHub repository setup:
  - [ ] Issue templates (bug report, feature request)
  - [ ] Contributing guide
  - [ ] Code of conduct
  - [ ] Release automation (GoReleaser)
- [ ] Package distribution:
  - [ ] Pre-built binaries for Linux, macOS, Windows (amd64, arm64)
  - [ ] Homebrew formula
  - [ ] `go install` support
- [ ] Example projects:
  - [ ] Hello world
  - [ ] HTTP API server
  - [ ] CLI tool
  - [ ] Domain modeling showcase (ADTs, pattern matching)
- [ ] Choose license (MIT or Apache 2.0)

---

## 4.6 — v1.0 Release

### Pre-Release Checklist
- [ ] All Phase 0–3 tests pass
- [ ] Language spec document complete
- [ ] Top 500 Go packages: >95% import success rate
- [ ] Compiler test coverage >85%
- [ ] Generated code passes `go vet`: 100%
- [ ] LSP p99 < 100ms
- [ ] No open high-severity bugs
- [ ] Error messages audited and polished
- [ ] VS Code extension published
- [ ] Website live
- [ ] Getting started guide tested by external users
- [ ] Example projects compile and run

### Release
- [ ] Tag v1.0.0
- [ ] Build and publish binaries
- [ ] Publish VS Code extension update
- [ ] Announce on relevant channels
- [ ] Stability guarantee: no breaking changes until v2.0

### Success Metrics Tracking
- [ ] Set up analytics for:
  - [ ] GitHub stars
  - [ ] Download counts
  - [ ] VS Code extension installs
  - [ ] Issue volume and response time
