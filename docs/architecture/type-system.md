# Type System

## Overview

Golem uses a constraint-based Hindley-Milner type system with bidirectional checking at function boundaries. The system provides full type inference within function bodies while requiring explicit annotations on public API surfaces.

The type system enforces:
- Parametric polymorphism (generics) mapped to Go 1.18+ type parameters
- Algebraic data types (sum types and product types)
- Exhaustive pattern matching (see [pattern-matching.md](pattern-matching.md))
- `Result<T, E>` and `Option<T>` as first-class types with special Go interop behavior

---

## Type Representation

All types in the compiler are represented as a recursive `Type` enum backed by a union-find structure for efficient unification.

```go
type Type struct {
    kind TypeKind
}

type TypeKind int

const (
    TVar     TypeKind = iota  // Unresolved type variable
    TCon                       // Concrete type constructor (Int, String, etc.)
    TApp                       // Type application (List<Int>, Result<T, E>)
    TFn                        // Function type (params -> return)
    TRecord                    // Record/struct type { field: Type, ... }
    TAlias                     // Named type alias
)
```

### Type Variables

Type variables are the core of inference. Each starts as `Unbound` and may become `Link`ed to a concrete type through unification.

```go
type TypeVar struct {
    id    uint64
    level int        // nesting depth, for generalization
    state TypeVarState
}

type TypeVarState int

const (
    Unbound TypeVarState = iota  // Not yet determined
    Linked                        // Resolved — follow the link
    Generic                       // Quantified (in a type scheme)
)
```

The `level` field tracks the let-nesting depth at which the variable was created. During generalization, only variables with a level greater than the current scope are quantified. This prevents escaping — a variable introduced inside a `let` body cannot be generalized if it is constrained by the outer scope.

### Concrete Types

```go
type ConcreteType struct {
    Name   string     // "Int", "String", "Bool", "Float", "Any"
    Module string     // "" for builtins, package path for Go types
}
```

### Type Applications

Generics are represented as type applications: `List<Int>` is `TApp(TCon("List"), [TCon("Int")])`.

```go
type TypeApp struct {
    Constructor Type    // The generic type (List, Result, Map, etc.)
    Args        []Type  // Type arguments
}
```

### Function Types

```go
type FnType struct {
    Params []Type
    Return Type
}
```

### Record Types

```go
type RecordType struct {
    Fields []RecordField
}

type RecordField struct {
    Name string
    Type Type
}
```

---

## Built-in Types

| Golem Type | Go Mapping | Notes |
|---|---|---|
| `Int` | `int` | Default integer type |
| `Float` | `float64` | Default float type |
| `String` | `string` | |
| `Bool` | `bool` | |
| `List<T>` | `[]T` | Slice-backed |
| `Map<K, V>` | `map[K]V` | |
| `Option<T>` | `*T` with nil semantics | See [go-interop.md](go-interop.md) |
| `Result<T, E>` | Generated interface | See [error-handling.md](error-handling.md) |
| `Chan<T>` | `chan T` | |
| `Any` | `any` / `interface{}` | Escape hatch |

---

## Type Inference Algorithm

Golem uses a constraint-based approach rather than Algorithm W's direct substitution threading. This decouples constraint generation from solving, producing better error messages.

### Phase 1: Constraint Generation

Walk the typed AST and emit constraints. A constraint is a pair of types that must be equal, tagged with the source location that created the constraint.

```go
type Constraint struct {
    Left  Type
    Right Type
    Span  Span    // where in the source this constraint originated
    Reason string // human-readable explanation ("argument 1 of foo")
}
```

#### Rules

**Literal**: `42` generates no constraint — its type is `Int`. `"hello"` is `String`. `true`/`false` is `Bool`.

**Variable reference**: Look up the variable's type scheme in the environment. Instantiate it (replace each generic variable with a fresh unbound variable). No constraint emitted.

**Function application** `f(a, b)`:
1. Infer the type of `f` — call it `Tf`.
2. Infer the types of `a` and `b` — call them `Ta` and `Tb`.
3. Create a fresh type variable `Tr` for the result.
4. Emit constraint: `Tf == Fn(Ta, Tb) -> Tr`.
5. The expression's type is `Tr`.

**Let binding** `let x = e1`:
1. Infer the type of `e1` — call it `T1`.
2. If `e1` is a value form (no unresolved effects), generalize `T1` into a type scheme.
3. Add `x: Scheme(T1)` to the environment for subsequent expressions.

**Function definition** `fn foo(a: A, b: B): R do body end`:
1. The parameters have annotated types `A` and `B` — these are concrete (no inference needed for annotated params).
2. The return type `R` is annotated — this is the expected type for the body.
3. Infer the type of `body` — call it `Tbody`.
4. Emit constraint: `Tbody == R`.
5. For unannotated parameters (in `priv` functions), create fresh type variables.

**Function definition (unannotated, private)**:
1. Create fresh type variables for each parameter and for the return type.
2. Add the parameters to the local environment.
3. Infer the body type.
4. Emit constraint: `body type == return type variable`.

**Match expression**:
1. Infer the type of the scrutinee — call it `Ts`.
2. For each arm `| pattern -> body`:
   a. Check the pattern against `Ts`, binding pattern variables with their types.
   b. Infer the body type — call it `Tb`.
   c. Emit constraint: `Tb == Tresult` (all arms must agree on result type).

**Record construction** `{ x: 1, y: 2.0 }`:
1. Infer types of all field expressions.
2. The expression type is `TRecord([("x", Int), ("y", Float)])`.
3. If there's an expected type from context (bidirectional checking), emit a constraint between the record type and the expected type.

**Field access** `expr.field`:
1. Infer the type of `expr`.
2. If the type is known (a concrete record or Go struct), look up the field and return its type.
3. If the type is an unbound variable, defer — the constraint solver will resolve it when the variable is unified.

### Phase 2: Constraint Solving

Process the constraint set by unification.

#### Unification Algorithm

```
unify(T1, T2):
  T1 = find(T1)  // follow union-find links
  T2 = find(T2)

  if T1 == T2: return  // same node, nothing to do

  match (T1, T2):
    (TVar(a), _):
      if occursIn(a, T2): error "infinite type"
      link(a, T2)

    (_, TVar(b)):
      if occursIn(b, T1): error "infinite type"
      link(b, T1)

    (TCon(name1), TCon(name2)):
      if name1 != name2: error "type mismatch: name1 vs name2"

    (TApp(c1, args1), TApp(c2, args2)):
      unify(c1, c2)
      for i in 0..len(args1):
        unify(args1[i], args2[i])

    (TFn(params1, ret1), TFn(params2, ret2)):
      if len(params1) != len(params2): error "arity mismatch"
      for i in 0..len(params1):
        unify(params1[i], params2[i])
      unify(ret1, ret2)

    (TRecord(fields1), TRecord(fields2)):
      // match by field name, unify types
      for each common field name:
        unify(fields1[name].type, fields2[name].type)
      // extra fields in either: error (Golem records are not open/extensible)

    _: error "type mismatch"
```

#### Union-Find

Type variables use a union-find (disjoint set) data structure with path compression and union by rank. This makes `find` effectively O(1) amortized.

```go
type TypeVarCell struct {
    parent *TypeVarCell  // nil if root
    rank   int
    value  Type          // the resolved type, if this cell is a root with a known type
}
```

### Phase 3: Generalization

After inferring a `let` binding's type, generalize it:

1. Find all free type variables in the inferred type.
2. Exclude any that are also free in the current environment (they are constrained by the outer scope).
3. Mark the remaining variables as `Generic`.

The result is a type scheme: `forall a b. a -> b -> (a, b)`.

### Bidirectional Checking

At function boundaries, Golem uses bidirectional checking to propagate type information downward:

- **Synthesis mode** (bottom-up): infer the type from the expression structure. Used for variables, literals, function application, field access.
- **Checking mode** (top-down): given an expected type, verify the expression conforms. Used for function bodies (expected type = annotated return type), match arm bodies (expected type = result type), and lambda arguments passed to higher-order functions.

Checking mode is activated when context provides a known type. For example:

```golem
let users: List<User> = [{ name: "Alice", age: 30 }]
```

The annotation `List<User>` pushes the expected element type `User` into the list literal, which pushes it into the record literal. Without bidirectional checking, the record literal would need standalone inference, potentially producing a less specific type or requiring redundant annotation.

**Key benefit**: When a lambda is passed to a higher-order function with a known signature, the lambda's parameter types are inferred from the expected function type — no annotation needed:

```golem
List.map(users, fn(u) -> u.name)
-- u is inferred as User from the expected type (User) -> String
```

---

## Generics

### Golem Generics

Golem generics use angle-bracket syntax and map directly to Go 1.18+ type parameters.

```golem
type Result<T, E> =
  | Ok { value: T }
  | Err { error: E }

pub fn map<A, B>(result: Result<A, E>, f: Fn<A, B>): Result<B, E> do
  match result do
    | Ok { value } -> Ok { value: f(value) }
    | Err { error } -> Err { error: error }
  end
end
```

### Inference of Type Arguments

At call sites, type arguments are inferred — never written explicitly in Golem source:

```golem
let x = map(myResult, fn(v) -> v + 1)
-- A and B are inferred from the argument types
```

The constraint generator creates fresh type variables for `A` and `B`, then unification resolves them from the argument types.

### Mapping to Go Generics

The code generator emits explicit type arguments in Go code, even though Go can sometimes infer them. This makes the generated code unambiguous:

```go
// Golem: map(myResult, fn(v) -> v + 1)
// Generated Go:
Map[int, int, error](myResult, func(v int) int { return v + 1 })
```

### Go Generic Function Imports

When Golem calls a Go generic function, the compiler:

1. Loads the function signature via `go/types`, including type parameter constraints.
2. Creates Golem type variables for each Go type parameter.
3. At the call site, infers the type arguments via normal unification.
4. Validates that the inferred types satisfy the Go constraints (e.g., if Go requires `comparable`, the Golem type must be a type that maps to a Go comparable type).
5. Emits the call with explicit type arguments.

---

## Type Schemes and Polymorphism

A type scheme (polytype) is a type with quantified variables:

```
forall a. a -> a                    -- identity
forall a b. a -> b -> (a, b)        -- pair constructor
forall a e. Result<a, e> -> a -> a  -- unwrapOr
```

Type schemes are stored for:
- `let` bindings (if the bound expression is generalizable)
- Top-level function declarations
- Type constructors (e.g., `Ok` has scheme `forall t e. t -> Result<t, e>`)

Instantiation creates fresh type variables for each quantified variable. Each use of a polymorphic binding gets its own copy:

```golem
let id = fn(x) -> x
let a = id(42)       -- instantiates id as Int -> Int
let b = id("hello")  -- instantiates id as String -> String
```

---

## The Value Restriction

Golem applies the value restriction: only `let` bindings whose right-hand side is a syntactic value (literal, lambda, constructor application) are generalized. This prevents unsoundness with mutable references (relevant if Golem ever adds them) and matches the behavior of OCaml and other ML-family languages.

Expressions with side effects (function calls, Go interop calls) are not generalized — their inferred type is monomorphic.

---

## Type Annotations

### Where Annotations Are Required

- `pub` function parameters and return types
- `pub` type definitions (inherently annotated by their definition)

### Where Annotations Are Optional

- `priv` function parameters and return types
- `let` bindings
- Lambda parameters
- Match arm bindings

### Annotation Syntax

```golem
let x: Int = 42
fn foo(a: Int, b: String): Bool do ... end
let f: Fn<Int, String> = fn(n) -> Int.toString(n)
```

Annotations are checked against inferred types — if both are present, the annotation must agree with inference. The annotation never overrides inference; a mismatch is a type error.

---

## Error Recovery in Type Checking

When a type error is detected (unification failure), the compiler:

1. Records the diagnostic with full span and context information.
2. Assigns a `TError` (poison) type to the problematic expression.
3. `TError` unifies with any type without emitting further errors. This prevents cascading: one mistake in a function body does not produce dozens of downstream errors.
4. Continues type checking the rest of the module.

This "error recovery" approach is critical for the LSP, which must provide useful diagnostics even in incomplete or partially incorrect code.
