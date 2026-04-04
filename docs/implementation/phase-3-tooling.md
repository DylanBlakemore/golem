# Phase 3 — Tooling

**Status:** Not Started
**Depends on:** Phase 2 complete
**Goal:** Language server, VS Code extension, incremental compilation, test syntax.
**Exit Criteria:** End-to-end IDE experience. Public beta release.

---

## 3.1 — Incremental Compilation

Reference: [compiler-pipeline.md](../architecture/compiler-pipeline.md) — Incremental Compilation

### Cache Infrastructure
- [ ] Cache directory: `.golem-cache/`
- [ ] Per-file cache keyed by SHA-256 content hash
- [ ] Cache structure: `.golem-cache/<file-hash>/tokens.bin`, `ast.bin`, `typed_ast.bin`, `fingerprint`
- [ ] Cache serialization format (binary encoding of AST nodes)

### File-Level Caching
- [ ] Skip lexing if source hash matches cached hash
- [ ] Skip parsing if tokens unchanged
- [ ] Skip type checking if AST unchanged AND dependency fingerprints unchanged

### Dependency Tracking
- [ ] Signature fingerprinting: hash of exported type signatures per file
- [ ] Transitive invalidation: if file A's fingerprint changes, re-check all files importing A
- [ ] Topological ordering of file dependency DAG
- [ ] Only re-run code gen for files with changed typed ASTs

### Go Package Caching
- [ ] Cache Go package type info by `(import path, module version)`
- [ ] Invalidate when `go.sum` changes

### CLI Integration
- [ ] `golem build` uses cache by default
- [ ] `golem build --no-cache` flag to force full rebuild
- [ ] Report cache hit/miss stats with `--verbose`

### Tests
- [ ] Cache hit: unchanged file is not re-processed
- [ ] Cache miss: modified file is re-processed
- [ ] Transitive invalidation: changing exported type re-checks dependents
- [ ] Changing private function does NOT invalidate dependents
- [ ] Go package cache works across builds
- [ ] Benchmark: incremental build vs full rebuild speedup

---

## 3.2 — Language Server (`golem-lsp`)

Reference: [compiler-pipeline.md](../architecture/compiler-pipeline.md) — Error Reporting

### LSP Protocol Implementation
- [ ] Separate binary: `cmd/golem-lsp/main.go`
- [ ] JSON-RPC over stdio transport
- [ ] Implement LSP lifecycle: `initialize`, `initialized`, `shutdown`, `exit`
- [ ] `textDocument/didOpen`, `textDocument/didChange`, `textDocument/didClose`
- [ ] Document sync: incremental text changes

### Diagnostics (Inline Type Errors)
- [ ] On file change: run pipeline through type checking + exhaustiveness
- [ ] Publish diagnostics via `textDocument/publishDiagnostics`
- [ ] Map `Span` to LSP `Range` (line/column)
- [ ] Severity mapping: Golem error -> LSP Error, Golem warning -> LSP Warning
- [ ] Debounce: wait for typing pause before re-checking

### Completion
- [ ] `textDocument/completion`
- [ ] Local variable names in scope
- [ ] Function names in module
- [ ] Imported Go package members (after `.`)
- [ ] Sum type variant constructors
- [ ] Record field names in pattern context
- [ ] Type names for annotations

### Hover
- [ ] `textDocument/hover`
- [ ] Show inferred type on hover over any expression
- [ ] Show function signature on hover over function name
- [ ] Show type definition on hover over type name

### Go-to-Definition
- [ ] `textDocument/definition`
- [ ] Jump to Golem function/type declarations
- [ ] Jump to Go source when hovering imported Go symbols
  - Use `go/packages` source position information
- [ ] Cross-file navigation within Golem project

### Exhaustiveness Warnings
- [ ] Non-exhaustive match -> LSP Warning diagnostic
- [ ] Redundant match arm -> LSP Hint diagnostic

### Performance
- [ ] Incremental re-checking: only re-check changed file + transitive dependents
- [ ] Target: p99 response time < 100ms for diagnostics
- [ ] Background type-checking thread (don't block editor)

### Tests
- [ ] Integration test: LSP client sends didOpen, receives diagnostics
- [ ] Test completion in various contexts
- [ ] Test hover shows correct types
- [ ] Test go-to-definition for Golem and Go symbols
- [ ] Performance benchmark: response time on medium project

---

## 3.3 — VS Code Extension

### Syntax Highlighting
- [ ] TextMate grammar for `.golem` files
- [ ] Keywords, operators, literals, comments, string interpolation
- [ ] Semantic token provider (use LSP semantic tokens for type-aware highlighting)

### LSP Client Integration
- [ ] Extension activates on `.golem` files
- [ ] Launches `golem-lsp` binary
- [ ] Configurable path to `golem-lsp` binary

### Commands
- [ ] "Show Generated Go": opens `.golem.go` file side-by-side
- [ ] "Golem: Build": runs `golem build` in terminal
- [ ] "Golem: Check": runs `golem check` in terminal
- [ ] "Golem: Format": runs `golem fmt` on current file

### Configuration
- [ ] `golem.lspPath`: path to `golem-lsp` binary
- [ ] `golem.formatOnSave`: auto-format on save
- [ ] `golem.buildOnSave`: auto-check on save

### Packaging
- [ ] VS Code extension manifest (`package.json`)
- [ ] Extension icon and README
- [ ] Publish to VS Code Marketplace

### Tests
- [ ] Extension activates on `.golem` file open
- [ ] Diagnostics appear in editor
- [ ] Completion works
- [ ] "Show Generated Go" command works
- [ ] Format on save works

---

## 3.4 — Test Syntax and `go test` Integration

Reference: [code-generation.md](../architecture/code-generation.md) — Tests

### Parsing
- [ ] `test` block syntax:
  ```golem
  test "area of a circle" do
    let shape = Circle { radius: 5.0 }
    assert area(shape) == 78.539816
  end
  ```
- [ ] `assert` keyword in test context
- [ ] Test names are string literals

### Type Checking
- [ ] Test blocks type-checked like function bodies
- [ ] `assert` takes a boolean expression
- [ ] Test blocks have no return type (Unit)

### Code Generation
- [ ] `test "name"` -> `func TestName(t *testing.T)`:
  - Sanitize name: spaces to camelCase, remove special characters
  - Prefix with `Test` (Go convention)
- [ ] `assert expr` -> `if !(expr) { t.Fatal(...) }`
- [ ] `assert expr == expected` -> `if expr != expected { t.Fatalf("expected %v, got %v", ...) }`
- [ ] Test files generate `_test.golem.go` files (Go test convention)

### Tests
- [ ] Parse test blocks
- [ ] Code gen for test functions
- [ ] Generated tests run with `go test`
- [ ] Assert failure messages are helpful

---

## 3.5 — `golem test` CLI Command

- [ ] `golem test`: build + run `go test ./build/...`
- [ ] `golem test <file>`: test specific file
- [ ] Pass through flags to `go test` (e.g., `-v`, `-run`)
- [ ] Map Go test output back to Golem source locations where possible

### Tests
- [ ] `golem test` runs all tests
- [ ] `golem test` with flags works
- [ ] Test output references Golem source lines

---

## 3.6 — Documentation Generator

- [ ] Extract doc comments from Golem source (comments above `pub` declarations)
- [ ] Generate HTML or Markdown documentation
- [ ] Include type signatures, function signatures
- [ ] Cross-reference types and functions
- [ ] `golem doc` CLI command
- [ ] `golem doc --serve` for local preview

### Tests
- [ ] Doc extraction from various declaration types
- [ ] Generated docs include type info
- [ ] Cross-references are correct

---

## 3.7 — End-to-End IDE Integration Test

- [ ] Open Golem project in VS Code with extension installed
- [ ] Syntax highlighting works for all constructs
- [ ] Type errors appear inline as you type
- [ ] Exhaustiveness warnings appear on incomplete match
- [ ] Completion suggests local vars, functions, Go package members
- [ ] Hover shows inferred types
- [ ] Go-to-definition works for Golem and Go symbols
- [ ] "Show Generated Go" opens correct file
- [ ] Format on save works
- [ ] `golem test` runs and reports results
- [ ] Incremental build is noticeably faster than full build
- [ ] All Phase 3 tests pass
