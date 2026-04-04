# Golem

Golem is a statically typed, expression-oriented language that transpiles to idiomatic Go.

## Development Commands

- `make build` — Build the golem binary
- `make format` — Format code
- `make lint` — Lint (golangci-lint)
- `make test` — Run all tests
- `make security` — Security + dependency checks
- `make ci` — Run everything (CI parity)

## Project Structure

- `cmd/golem/` — CLI entrypoint
- `internal/lexer/` — Tokenizer
- `internal/parser/` — Recursive descent parser
- `internal/ast/` — AST node definitions
- `internal/resolver/` — Name resolution
- `internal/checker/` — Type inference & checking
- `internal/desugar/` — Desugaring pass
- `internal/codegen/` — Go code emitter
- `internal/diagnostic/` — Error reporting
- `internal/span/` — Source location tracking

## Conventions

- All compiler internals live under `internal/`
- Generated Go output uses `.golem.go` extension
- Printf-style code generation piped through `go/format.Source()`
