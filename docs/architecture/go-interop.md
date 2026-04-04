# Go Interoperability

## Overview

Golem's interop with Go is strictly **one-directional**: Golem calls Go. Go does not call Golem. This simplification eliminates ABI compatibility concerns, export naming issues, and bidirectional type bridging. The result is a clean FFI boundary where the Golem compiler reads Go type information and maps it into Golem's type system automatically.

---

## Architecture

```
Go ecosystem                    Golem compiler
─────────────                   ──────────────
                                
go/packages.Load() ──────────>  Go Package Loader
  (reads compiled                     |
   package metadata)                  v
                                Type Mapper
                                  (Go types -> Golem types)
                                      |
                                      v
                                Module Type Environment
                                  (available for type checking)
```

The Go Package Loader runs during the name resolution phase (Phase 3) of the compiler pipeline. When the compiler encounters an `import "net/http"` statement, it:

1. Calls `golang.org/x/tools/go/packages.Load()` to read the package's exported type information.
2. Passes the `*types.Package` to the Type Mapper.
3. The Type Mapper converts every exported symbol into a Golem type representation.
4. These types are registered in the module's type environment for use during type checking.

---

## Go Package Loading

### Using `go/packages`

```go
cfg := &packages.Config{
    Mode: packages.NeedTypes | packages.NeedTypesInfo | packages.NeedName,
    Dir:  projectRoot,  // the Go module root
}
pkgs, err := packages.Load(cfg, importPath)
```

This reads the compiled package information from the Go build cache or module cache. It does **not** require the Go source to be present — compiled `.a` files or module cache entries suffice.

### Caching

Package type information is cached by `(import path, module version)`. The cache is invalidated when:
- `go.mod` changes (dependency versions updated)
- `go.sum` changes (integrity hashes updated)
- The Go version changes (stdlib types may differ)

For Go stdlib packages, the cache key is the Go version string.

### Error Handling

If a Go package cannot be loaded (not installed, compilation error, etc.), the compiler reports a diagnostic at the import site and continues with a "poison" module — all references to that module's symbols will produce additional errors, but compilation of the rest of the file continues.

---

## Type Mapping

### Primitive Types

| Go Type | Golem Type | Notes |
|---|---|---|
| `string` | `String` | Direct mapping |
| `int` | `Int` | Default integer |
| `int8/16/32/64` | `Int` | Widened to `Int`; no distinct narrow types in v1 |
| `uint`, `uint8/16/32/64` | `Int` | Widened; no unsigned types in v1 |
| `float32` | `Float` | Widened to `float64` semantics |
| `float64` | `Float` | Direct mapping |
| `bool` | `Bool` | Direct mapping |
| `byte` | `Int` | Alias for uint8 |
| `rune` | `Int` | Alias for int32 |

**Widening rationale**: Golem v1 prioritizes simplicity over precision. A single `Int` and `Float` type avoids a combinatorial explosion of numeric types. The generated Go code uses the specific Go type at the boundary (the code generator knows the original Go type and emits correct casts).

### Composite Types

| Go Type | Golem Type | Notes |
|---|---|---|
| `[]T` | `List<T>` | Backed by Go slices |
| `[N]T` (array) | `List<T>` | Converted to slice at the boundary |
| `map[K]V` | `Map<K, V>` | Direct mapping |
| `chan T` | `Chan<T>` | Direct mapping |
| `chan<- T` | `Chan<T>` | Direction lost in v1 (future: directional channels) |
| `<-chan T` | `Chan<T>` | Direction lost in v1 |

### Pointer and Nil Handling

| Go Type | Golem Type | Rationale |
|---|---|---|
| `*T` (any pointer) | `Option<T>` | nil becomes `None`, non-nil becomes `Some(value)` |
| `T` (non-pointer) | `T` | No wrapping needed |

This is the most important mapping decision. Go's nil is a significant source of bugs; wrapping all pointers in `Option` forces Golem code to handle the nil case explicitly via pattern matching.

**At the FFI boundary**, the code generator inserts nil checks:

```go
// Go function: func FindUser(id int) *User
// Golem sees: fn findUser(id: Int): Option<User>

// Generated Go at the call site:
rawResult := FindUser(id)
var golemResult Option[User]
if rawResult == nil {
    golemResult = None[User]{}
} else {
    golemResult = Some[User]{Value: *rawResult}
}
```

### Function Signatures

Go functions map to Golem function types with special handling for multiple returns:

| Go Signature | Golem Type | Notes |
|---|---|---|
| `func(int) string` | `Fn<Int, String>` | Simple case |
| `func(int) (string, error)` | `Fn<Int, Result<String, Error>>` | Error pair lifting |
| `func(int) (*User, error)` | `Fn<Int, Result<Option<User>, Error>>` | Pointer + error |
| `func(int, ...string)` | Variadic; see below | |
| `func() error` | `Fn<Result<Unit, Error>>` | Error-only return |
| `func()` | `Fn<Unit>` | No return value |

### The `(T, error)` Convention

Go's dominant error pattern is returning `(T, error)`. The type mapper detects this pattern and automatically lifts it to `Result<T, Error>`:

**Detection rule**: If a Go function's return signature has exactly two results, and the last one is the `error` interface type, treat it as `Result<T, Error>` where `T` is the Golem mapping of the first return type.

```go
// Go: func os.ReadFile(name string) ([]byte, error)
// Golem sees: fn readFile(name: String): Result<List<Int>, Error>
```

**Edge cases**:
- `(T, bool)` — not lifted. Only `error` triggers the conversion.
- `(T, S, error)` — three+ returns are not lifted automatically. The Golem developer sees a tuple and must destructure. (Future: named result wrapping.)
- `error` as the sole return (`func() error`) — mapped to `Result<Unit, Error>`.
- Functions returning only `error` with other values — only the two-return `(T, error)` form is lifted.

### Interface Types

| Go Type | Golem Type | Notes |
|---|---|---|
| `interface{}` / `any` | `Any` | Opaque; requires type assertion to use |
| Named interface (e.g., `io.Reader`) | Opaque type `io.Reader` | Golem cannot implement interfaces (v1) |
| `error` interface | `Error` | Built-in Golem type |

Golem v1 does not allow defining types that implement Go interfaces. This is a deliberate limitation — Golem is for writing new services, not for extending Go libraries. The workaround is the `@goraw` escape hatch for the rare cases where interface implementation is needed.

### Struct Types

Go structs encountered through imports are mapped to Golem record types:

```go
// Go: type http.Request struct { Method string; URL *url.URL; ... }
// Golem sees: type Request = { method: String, url: Option<url.URL>, ... }
```

Field names are lowercased in the Golem representation (Golem has no casing convention for field access). The code generator maps back to the correct Go field name.

Only exported fields are visible in Golem. Unexported Go fields are invisible.

### Variadic Functions

Go variadic functions (`func(a int, b ...string)`) are mapped to Golem functions whose last parameter is `List<T>`:

```go
// Go: func fmt.Sprintf(format string, a ...any) string
// Golem: fn sprintf(format: String, a: List<Any>): String
```

The code generator expands the list argument with `...` in the generated Go call:

```go
fmt.Sprintf(format, a...)
```

### Generic Go Functions

When a Go function has type parameters, the type mapper:

1. Creates Golem type variables for each Go type parameter.
2. Maps the Go constraint to a Golem constraint (best-effort; `any` maps to unconstrained, `comparable` maps to a built-in Golem constraint).
3. At call sites, Golem infers the type arguments via its normal inference and emits explicit type arguments in generated Go.

```go
// Go: func slices.Sort[S ~[]E, E cmp.Ordered](s S)
// Golem: fn sort<S, E>(s: S): S  (with constraints from Go)

// Call site: slices.Sort(myList)
// Generated Go: slices.Sort[[]int, int](myList)
```

---

## The `@goraw` Escape Hatch

For cases where the type mapper cannot represent a Go construct:

```golem
@goraw("http.HandleFunc(\"/\", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(\"ok\")) })")
```

This inserts the raw Go expression into the generated code verbatim. The compiler:

1. Emits a warning: "Using @goraw bypasses type checking."
2. Does not type-check the expression.
3. Inserts it as-is into the generated Go output.

`@goraw` is expected to be rare — it exists for implementing Go interfaces (which Golem v1 cannot do natively) and for deeply nested type assertions that the mapper cannot handle.

---

## Import Syntax and Resolution

### Golem Import Statements

```golem
import "net/http"                    -- Go stdlib package
import "github.com/some/library"     -- Third-party Go package
import "./handlers"                  -- Local Golem module (relative path)
```

### Resolution Rules

1. If the import path starts with `./` or `../`, resolve it as a relative Golem module.
2. Otherwise, resolve it as a Go package via `go/packages`.
3. The import is added to the module's type environment for subsequent name resolution.

### Name Access

All access to imported modules is qualified:

```golem
let resp = http.Get("https://example.com")?
let data = json.Marshal(user)?
```

Golem does not support unqualified imports (`from "net/http" import Get`) in v1. Qualified access keeps the code explicit and avoids name collisions.

### Name Mapping for Go Symbols

Go symbols are accessed in Golem using their original name but with the first letter lowercased (since Golem developers don't think about Go's export casing):

```golem
-- Go: http.ListenAndServe(addr string, handler Handler) error
-- Golem usage:
http.listenAndServe(":8080", handler)?
```

The code generator maps `listenAndServe` back to `ListenAndServe` in the emitted Go code. The Golem developer never writes uppercase-initial identifiers for Go functions.

**Ambiguity rule**: If a Go package exports both `Foo` and `foo` (extremely rare), Golem sees only the exported (`Foo`) one. Unexported Go symbols are inaccessible from Golem.

---

## Go Module Integration

A Golem project is a standard Go module:

```
myproject/
  go.mod           -- standard Go module file
  go.sum           -- standard Go checksum file
  main.golem       -- Golem source
  build/
    main.golem.go  -- generated Go
```

### Dependency Management

- `go mod init` creates the module.
- `go get github.com/some/library` adds dependencies.
- There is no Golem-specific package manager. Go modules handle everything.

### Build Integration

`golem build` is equivalent to:

```bash
# 1. Compile all .golem files to .golem.go in build/
golem compile

# 2. Run go build on the generated code
cd build && go build ./...
```

The `golem` CLI wraps both steps. The developer can also run them separately for debugging.
