# Golem

Golem is a statically typed, expression-oriented programming language that transpiles to idiomatic Go source code. It provides algebraic data types, exhaustive pattern matching, and ergonomic error handling while giving programs full, unmediated access to the Go ecosystem.

## What Golem Is

Go is an excellent platform. Its concurrency model, single-binary deployment, and standard library have made it a staple of backend and infrastructure development. But its type system and error handling create friction that compounds as a codebase grows. There are no sum types, so discriminated unions require verbose interface boilerplate with no exhaustiveness guarantee. There is no pattern matching worth speaking of. And `if err != nil` repeated at every call site obscures control flow without adding information.

Golem addresses this by sitting one layer above Go. It provides the type system and syntax that Go is missing, while the output is readable, idiomatic Go source code that you can inspect, review, and commit to version control. There is no runtime library, no package manager, and no new ecosystem. It is a better way to write Go-backed services.

Interop is strictly one-directional: Golem calls Go, not the reverse. Any Go package is importable without wrappers or hand-written bindings. This simplification makes Golem appropriate for new projects and new services, not for incrementally migrating existing Go codebases.

## A Quick Look

```golem
import "os"
import "fmt"

type FileResult =
  | TextFile { content: String }
  | EmptyFile
  | ReadError { message: String }

priv fn describeResult(r: FileResult): String do
  match r do
    | TextFile { content } -> "Got content: " <> content
    | EmptyFile            -> "File was empty"
    | ReadError { message } -> "Error: " <> message
  end
end

pub fn processFile(path: String): String do
  let result = os.readFile(path)
  match result do
    | Ok { value } -> describeResult(TextFile { content: fmt.sprintf("%s", value) })
    | Err { error } -> describeResult(ReadError { message: fmt.sprintf("%v", error) })
  end
end

pub fn main() do
  fmt.println(processFile("/etc/hosts"))
end
```

The above compiles to a valid Go program. `os.readFile` returns `([]byte, error)` in Go; Golem automatically lifts this to `Result<List<Int>, Error>`, and the `match result` expression generates a type switch over the two result variants. The match on `r` in `describeResult` generates a type switch over the three variants of `FileResult`. If you add a fourth variant and forget to update that match, the compiler refuses to produce output.

## Language Features

### Sum Types

The primary data modeling tool in Golem is the sum type (also called an algebraic data type or discriminated union):

```golem
type Shape =
  | Circle { radius: Float }
  | Rectangle { width: Float, height: Float }
  | Triangle { base: Float, height: Float }
```

This generates a sealed Go interface and one struct per variant. The generated code is ordinary Go that any Go developer can read and understand. Sum types are generic:

```golem
type Tree<A> =
  | Leaf
  | Node { value: A, left: Tree<A>, right: Tree<A> }
```

### Pattern Matching

The `match` expression is how you work with sum types:

```golem
let area = match shape do
  | Circle { radius }           -> Float.pi * radius * radius
  | Rectangle { width, height } -> width * height
  | Triangle { base, height }   -> 0.5 * base * height
end
```

The compiler enforces exhaustiveness. Every variant must be covered, or the program will not compile. This means adding a new variant to a sum type immediately surfaces every match expression that needs updating. Match is an expression, not a statement, so it can appear anywhere a value is expected: in a `let` binding, as a function argument, or as the implicit return value of a function.

### Error Handling

Golem has built-in `Result<T, E>` and `Option<T>` types. The `?` operator propagates errors early, similar to Rust:

```golem
pub fn readConfig(path: String): Result<Config, Error> do
  let raw = os.readFile(path)?
  parseConfig(raw)?
end
```

When `?` is applied to a `Result`, it desugars to an early return of `Err` if the value is an error, or unwraps the `Ok` value and continues. This removes the need for `if err != nil` chains entirely.

At Go call sites, Golem automatically lifts Go's `(T, error)` return convention into `Result<T, Error>`. The lift happens in generated code at the call boundary, so you interact with a proper `Result` type throughout your Golem code. Go pointer types (`*T`) are similarly lifted to `Option<T>`, with `nil` becoming `None`.

### Go Interoperability

Any Go package is importable directly:

```golem
import "net/http"
import "fmt"

pub fn main() do
  http.handleFunc("/", fn(w: http.ResponseWriter, r: *http.Request) do
    fmt.fprintln(w, "Hello from Golem!")
  end)
  fmt.println("Listening on :8080")
  http.listenAndServe(":8080", nil)
end
```

Golem reads Go package type signatures at compile time using `go/types`, maps them into the Golem type system, and generates the correct Go at each call site. You access Go identifiers with the first letter lowercased (`http.listenAndServe`, `fmt.println`). The generated output uses the original capitalized names.

The type mapping covers the common cases:

| Go type | Golem type |
|---------|-----------|
| `string` | `String` |
| `int`, `int64` | `Int` |
| `float64` | `Float` |
| `bool` | `Bool` |
| `[]T` | `List<T>` |
| `map[K]V` | `Map<K, V>` |
| `*T` | `Option<T>` |
| `(T, error)` | `Result<T, Error>` |
| `interface{}` | `Any` |
| `chan T` | `Chan<T>` |

### Visibility

Golem uses `pub` and `priv` keywords for visibility. Go's naming convention (uppercase for exported, lowercase for unexported) is a code generation detail that Golem handles automatically. You write names however you like, and the compiler capitalizes or lowercases generated Go identifiers accordingly.

```golem
pub fn processRequest(req: Request): Response do
  ...
end

priv fn validateToken(token: String): Bool do
  ...
end
```

`priv` is the default. A declaration with no modifier is private. Writing `pub` must be deliberate, which makes the public surface of a module visible at a glance.

### Functions and Blocks

Functions use `do/end` blocks. The last expression in a block is its value, so explicit `return` is rarely needed:

```golem
pub fn clamp(value: Int, lo: Int, hi: Int): Int do
  if value < lo do
    lo
  else if value > hi do
    hi
  else
    value
  end
end
```

Anonymous functions use a concise arrow form for single-expression bodies:

```golem
let doubled = List.map(numbers, fn(n: Int) -> n * 2)
```

The pipe operator threads a value through a sequence of functions, left to right:

```golem
let result =
  users
  |> List.filter(fn(u) -> u.active)
  |> List.map(fn(u) -> u.name)
  |> String.join(", ")
```

### Generics

Golem generics map directly to Go 1.18+ type parameters. Type arguments are inferred at call sites:

```golem
pub fn identity<A>(x: A): A do
  x
end

fn mapOption<A, B>(opt: Option<A>, f: Fn<A, B>): Option<B> do
  match opt do
    | Some { value } -> Some { value: f(value) }
    | None           -> None
  end
end
```

## Getting Started

Golem requires Go 1.21 or later. There are no pre-built releases yet, so you build the compiler from source:

```bash
git clone https://github.com/dylanblakemore/golem
cd golem
make build
```

This produces a `golem` binary in the project root. Add it to your `PATH`, then create a new project:

```bash
mkdir myproject && cd myproject
go mod init myproject
```

Write a `main.golem` file:

```golem
import "fmt"

pub fn main() do
  fmt.println("Hello from Golem!")
end
```

Build and run it:

```bash
golem run
```

This compiles your `.golem` files, generates `.golem.go` files into `build/`, and runs the resulting binary.

## Project Structure

A Golem project is a standard Go module. Golem source files live in the project root. Generated Go files go into a `build/` directory that mirrors the source tree:

```
myproject/
├── go.mod
├── main.golem
├── handlers/
│   └── handlers.golem
└── build/
    ├── main.golem.go
    └── handlers/
        └── handlers.golem.go
```

The `build/` directory is the boundary between the two languages. Everything above it is Golem source; everything inside it is generated Go. Running `go build ./build/...` is all it takes to produce a binary. Whether to commit `build/` to version control is a team decision. Both workflows are supported.

## CLI Reference

`golem build` compiles all `.golem` files found recursively in the current directory, skipping `build/` itself, and runs `go build` on the generated output.

`golem run` does the same and then executes the compiled binary, forwarding any additional arguments.

Both commands accept `--verbose` to print per-stage timing.

## Project Status

Golem is in active development and is not yet ready for production use.

Phase 0 (bootstrap) and Phase 1 (type system core) are complete. The compiler can lex, parse, resolve, type-check, and generate valid Go for programs using sum types with exhaustiveness checking, flat pattern matching, generics, `Result<T, E>` and `Option<T>`, the `?` operator, and Go package imports with automatic type lifting. All generated code passes `go vet` and `go build`.

Phase 2 covers nested pattern matching with destructuring, guard clauses in match expressions, list patterns, and additional Go interop coverage including variadics and embedding. Phase 3 brings tooling: a language server, VS Code extension, and test syntax. Phase 4 is stabilization toward a v1.0 release.

Detailed per-phase checklists and architecture documentation live in `docs/`.

## Building and Contributing

```bash
make build     # build the golem binary
make test      # run all tests
make lint      # run golangci-lint
make format    # run go fmt
make ci        # run all checks (CI parity)
make clean     # remove build artifacts and binary
```

The compiler is written in Go and organized under `internal/`. Each pipeline stage is a separate package: `internal/lexer`, `internal/parser`, `internal/resolver`, `internal/checker`, `internal/desugar`, `internal/codegen`. Architecture documentation for each stage is in `docs/architecture/`.
