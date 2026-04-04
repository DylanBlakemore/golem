# Error Handling

## Overview

Golem replaces Go's `if err != nil` pattern with a `Result<T, E>` type and a `?` operator for ergonomic error propagation. This design is directly inspired by Rust and Gleam, adapted for a Go transpilation target.

---

## The Result Type

### Definition

`Result<T, E>` is a built-in sum type:

```golem
type Result<T, E> =
  | Ok { value: T }
  | Err { error: E }
```

It is defined in the Golem prelude (always available, no import needed) and has special compiler support for the `?` operator and Go interop auto-lifting.

### Generated Go

```go
type Result[T any, E any] interface {
	isResult()
}

type Ok[T any, E any] struct {
	Value T
}

func (Ok[T, E]) isResult() {}

type Err[T any, E any] struct {
	Error E
}

func (Err[T, E]) isResult() {}
```

### Usage

```golem
pub fn divide(a: Float, b: Float): Result<Float, String> do
  if b == 0.0 do
    Err { error: "division by zero" }
  else
    Ok { value: a / b }
  end
end

-- Pattern matching on Result
match divide(10.0, 3.0) do
  | Ok { value } -> "Result: #{value}"
  | Err { error } -> "Error: #{error}"
end
```

---

## The Option Type

### Definition

```golem
type Option<T> =
  | Some { value: T }
  | None
```

Also defined in the prelude. Used primarily at the Go interop boundary where `*T` (nullable pointer) is mapped to `Option<T>`.

### Generated Go

```go
type Option[T any] interface {
	isOption()
}

type Some[T any] struct {
	Value T
}

func (Some[T]) isOption() {}

type None[T any] struct{}

func (None[T]) isOption() {}
```

---

## The `?` Operator

### Semantics

`expr?` is syntactic sugar for: "if `expr` evaluates to `Err`, return that error immediately from the enclosing function; otherwise, unwrap the `Ok` value."

The enclosing function must return a `Result` type. The error types must be compatible (same type, or convertible).

### Desugaring

The `?` operator is expanded during the desugaring phase (Phase 6) of the compiler pipeline.

**Source:**

```golem
pub fn readConfig(path: String): Result<Config, Error> do
  let content = File.read(path)?
  let config = Json.parse(content)?
  Ok { value: config }
end
```

**After desugaring (conceptual):**

```golem
pub fn readConfig(path: String): Result<Config, Error> do
  let __tmp1 = File.read(path)
  let content = match __tmp1 do
    | Ok { value } -> value
    | Err { error } -> return Err { error: error }
  end
  let __tmp2 = Json.parse(content)
  let config = match __tmp2 do
    | Ok { value } -> value
    | Err { error } -> return Err { error: error }
  end
  Ok { value: config }
end
```

**Generated Go:**

```go
func ReadConfig(path string) Result[Config, error] {
	tmp1, err1 := os.ReadFile(path)
	if err1 != nil {
		return Err[Config, error]{Error: err1}
	}
	content := tmp1

	tmp2 := JsonParse(content)
	if err2, ok := tmp2.(Err[Config, error]); ok {
		return err2
	}
	config := tmp2.(Ok[Config, error]).Value

	return Ok[Config, error]{Value: config}
}
```

### Expression Position

The `?` operator can appear anywhere an expression is expected, including nested inside other expressions:

```golem
let name = getUserName(fetchUser(id)?)
```

The desugarer hoists the `?` to statement level:

```golem
let __tmp = fetchUser(id)
match __tmp do
  | Err { error } -> return Err { error: error }
  | Ok { value } ->
    let name = getUserName(value)
end
```

### Chained `?`

Multiple `?` operators in a single expression are hoisted in left-to-right evaluation order:

```golem
let result = process(read(a)?, read(b)?)
```

Desugars to:

```golem
let __tmp1 = read(a)
match __tmp1 do
  | Err { error } -> return Err { error }
  | Ok { value: v1 } ->
    let __tmp2 = read(b)
    match __tmp2 do
      | Err { error } -> return Err { error }
      | Ok { value: v2 } ->
        let result = process(v1, v2)
    end
end
```

---

## Go Interop: Automatic `(T, error)` Lifting

When Golem code calls a Go function that returns `(T, error)`, the compiler automatically wraps the result as `Result<T, Error>`.

### Detection

The type mapper (see [go-interop.md](go-interop.md)) identifies Go functions with the `(T, error)` return pattern and assigns them a Golem return type of `Result<T, Error>`.

### Code Generation

The generated Go code at the call site converts Go's two-return convention to Golem's `Result`:

```golem
let content = os.ReadFile("/etc/hosts")?
```

Generated Go:

```go
rawContent, rawErr := os.ReadFile("/etc/hosts")
if rawErr != nil {
	return Err[[]byte, error]{Error: rawErr}
}
content := rawContent
```

When the `?` is not used (the developer wants to handle the Result manually):

```golem
let result = os.ReadFile("/etc/hosts")
match result do
  | Ok { value } -> processBytes(value)
  | Err { error } -> handleError(error)
end
```

Generated Go:

```go
rawContent, rawErr := os.ReadFile("/etc/hosts")
var result Result[[]byte, error]
if rawErr != nil {
	result = Err[[]byte, error]{Error: rawErr}
} else {
	result = Ok[[]byte, error]{Value: rawContent}
}
switch v := result.(type) {
case Ok[[]byte, error]:
	processBytes(v.Value)
case Err[[]byte, error]:
	handleError(v.Error)
}
```

### Functions Returning Only `error`

Go functions that return only `error` (e.g., `func Close() error`) are mapped to `Result<Unit, Error>`:

```golem
file.Close()?
```

Generated Go:

```go
if err := file.Close(); err != nil {
	return Err[Unit, error]{Error: err}
}
```

---

## Error Type Compatibility

### The `Error` Type

Golem's built-in `Error` type maps to Go's `error` interface. It is the default error type for Go interop.

### Custom Error Types

Golem code can define custom error types and use them with `Result`:

```golem
type AppError =
  | NotFound { resource: String }
  | Unauthorized { user: String }
  | Internal { message: String }

pub fn getUser(id: Int): Result<User, AppError> do
  if id < 0 do
    Err { error: NotFound { resource: "user" } }
  else
    Ok { value: lookupUser(id) }
  end
end
```

### Error Propagation Across Types

When `?` propagates an error, the error types of the inner and outer `Result` must match. If they don't, the compiler reports a type error:

```golem
-- This is a type error: IOError != AppError
pub fn loadConfig(path: String): Result<Config, AppError> do
  let content = File.read(path)?  -- File.read returns Result<String, IOError>
  ...
end
```

The developer must explicitly convert:

```golem
pub fn loadConfig(path: String): Result<Config, AppError> do
  let content = match File.read(path) do
    | Ok { value } -> value
    | Err { error } -> return Err { error: Internal { message: error.message } }
  end
  ...
end
```

Future versions may add a `From` trait or conversion mechanism (similar to Rust's `From` for `?`), but v1 keeps it explicit.

---

## Design Rationale

### Why Not Exceptions?

Go does not have exceptions (only `panic`/`recover`, which are not idiomatic for error handling). Golem follows this philosophy — errors are values, not control flow mechanisms. `Result` makes the error path visible in the type signature and forces the developer to handle it.

### Why Not `use` (Gleam-style)?

Gleam uses `use` expressions with callbacks for error propagation. This is elegant but unfamiliar to Go developers and produces more complex generated code (closures instead of early returns). The `?` operator is more familiar (from Rust), produces simpler generated Go (if-return), and reads more naturally in imperative code.

### Why Require Matching Error Types?

Implicit error conversion (Rust's `From` trait on `?`) is powerful but adds complexity to both the type system and the generated code. For v1, explicit conversion keeps the mental model simple and the generated Go transparent. The developer always knows exactly what error type they're propagating.
