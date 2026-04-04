# Pattern Matching and Exhaustiveness Checking

## Overview

Pattern matching is Golem's primary conditional mechanism. The compiler is responsible for two tasks:

1. **Exhaustiveness checking** — verify at compile time that every possible value is handled.
2. **Compilation to Go** — translate pattern matches into efficient Go type switches, if-else chains, and variable bindings.

Both tasks use the same underlying algorithm: Maranget's decision tree construction.

---

## Pattern Language

### Pattern Types

```
pattern ::=
  | constructor_pattern     Circle { radius }
  | record_pattern          { x, y }
  | list_pattern            [head, ..tail]  |  []
  | literal_pattern         42  |  "hello"  |  true
  | wildcard                _
  | variable_binding        name
  | nested_pattern          Ok { value: User { name, role: Admin } }
  | or_pattern              Circle { .. } | Square { .. }   (future)
```

### Pattern AST

```go
type Pattern interface{ patternNode() }

type ConstructorPattern struct {
    Span        Span
    Constructor string           // "Circle", "Ok", "None"
    Fields      []FieldPattern   // named fields
    Type        Type             // resolved type (filled during type checking)
}

type FieldPattern struct {
    Name    string   // the field name
    Pattern Pattern  // the pattern for this field (can be nested)
}

type RecordPattern struct {
    Span   Span
    Fields []FieldPattern
    Rest   bool    // true if `..` is present (ignore remaining fields)
}

type ListPattern struct {
    Span     Span
    Elements []Pattern
    Tail     Pattern   // nil for exact match, variable for `..tail`
}

type LiteralPattern struct {
    Span  Span
    Value interface{}  // int64, float64, string, bool
}

type WildcardPattern struct {
    Span Span
}

type VarPattern struct {
    Span Span
    Name string
}
```

### Guard Clauses

A match arm may have an optional guard:

```golem
match user do
  | { age } if age >= 18 -> "adult"
  | { age } -> "minor"
end
```

Guards are semantically transparent to exhaustiveness checking — the compiler treats guarded arms as potentially not matching (since the guard may fail at runtime). A guarded arm does not satisfy exhaustiveness on its own; a fallback arm is required.

---

## Exhaustiveness Checking

### The Maranget Algorithm

Golem implements the algorithm from Maranget's "Warnings for Pattern Matching" (JFP 2007). The algorithm operates on a **pattern matrix** where each row is a match arm and each column corresponds to a sub-position of the scrutinee.

#### Type Categories

Different types have different constructor sets:

| Type Category | Constructors | Finite? | Requires Wildcard? |
|---|---|---|---|
| Sum type (ADT) | Variant names (Circle, Rectangle, ...) | Yes | No (if all listed) |
| Bool | `true`, `false` | Yes | No (if both listed) |
| List | `[]`, `[head, ..tail]` | Yes (2 shapes) | No (if both listed) |
| Int | All integers | No | Yes |
| Float | All floats | No | Yes |
| String | All strings | No | Yes |
| Record | Single constructor (the record itself) | Yes | No |
| Option | `Some`, `None` | Yes | No (if both listed) |
| Result | `Ok`, `Err` | Yes | No (if both listed) |

#### Algorithm

```
checkExhaustive(patternMatrix, typeVector):
  if patternMatrix is empty:
    // No arms cover this case — report the missing pattern
    return MissingPattern(reconstruct from typeVector)

  if typeVector is empty:
    // All positions matched — this case is covered
    return Covered

  // Pick a column to decompose (heuristic: leftmost with constructors)
  col = selectColumn(patternMatrix)
  type = typeVector[col]

  if type is finite (sum type, Bool, Option, Result, List):
    constructors = allConstructors(type)
    presentConstructors = constructorsInColumn(patternMatrix, col)

    if presentConstructors == constructors:
      // All constructors present — specialize for each
      for each constructor C in constructors:
        subMatrix = specialize(patternMatrix, col, C)
        subTypes = expandTypeVector(typeVector, col, C)
        checkExhaustive(subMatrix, subTypes)
    else:
      // Some constructors missing — check if wildcard/var covers the rest
      defaultMatrix = defaultRows(patternMatrix, col)
      missingConstructors = constructors - presentConstructors
      if defaultMatrix is empty:
        for each missing C:
          report MissingPattern(C with wildcards for fields)
      else:
        checkExhaustive(defaultMatrix, typeVector without col)

  if type is infinite (Int, Float, String):
    // Must have a wildcard/variable catch-all
    defaultMatrix = defaultRows(patternMatrix, col)
    if defaultMatrix is empty:
      report MissingPattern(_ for this column)
    else:
      checkExhaustive(defaultMatrix, typeVector without col)
```

#### Specialization

`specialize(matrix, col, constructor)`: For each row in the matrix:
- If the pattern in `col` matches `constructor`: replace it with the constructor's sub-patterns (fields).
- If the pattern in `col` is a wildcard/variable: expand it to wildcards for each of the constructor's fields.
- If the pattern in `col` is a different constructor: discard the row.

Example — specializing for `Circle` on a `Shape` match:

```
Before:                          After specialize(_, 0, Circle):
| Circle { radius }  -> ...     | radius  -> ...
| Rectangle { w, h } -> ...     (discarded)
| _                   -> ...     | _       -> ...
```

#### Default Matrix

`defaultRows(matrix, col)`: Keep only rows where column `col` is a wildcard or variable. Remove column `col`.

### Missing Pattern Reconstruction

When exhaustiveness fails, the algorithm must produce a human-readable description of the missing case. This is done by tracking the path of constructors chosen during decomposition:

```
Missing pattern for Shape match:
  - Triangle { base: _, height: _ }
```

For nested patterns:

```
Missing pattern for Result<User, Error> match:
  - Ok { value: User { name: _, role: Guest } }
```

### Redundancy Detection

After checking exhaustiveness, a second pass identifies unreachable arms. An arm is redundant if removing it does not change the set of covered patterns. This is detected during the decision tree construction — if a branch in the tree can never be reached, the corresponding arm is redundant.

Redundant arms are reported as warnings, not errors.

---

## Compilation to Go

### Decision Tree Construction

The same pattern matrix decomposition used for exhaustiveness checking produces a **decision tree** that drives code generation.

```go
type Decision interface{ decisionNode() }

type Switch struct {
    // Test the scrutinee at this position
    Path     AccessPath   // how to reach this sub-value
    Branches []Branch     // one per constructor
    Default  *Decision    // fallback (for infinite types or catch-all)
}

type Branch struct {
    Constructor string
    Bindings    []Binding   // variables bound by this constructor's fields
    SubDecision Decision    // what to test next
}

type Leaf struct {
    ArmIndex int  // which match arm to execute
    Bindings []Binding
}

type Fail struct{}  // unreachable if exhaustiveness passed

type Binding struct {
    VarName  string
    Path     AccessPath
}

type AccessPath struct {
    Steps []AccessStep  // e.g., [TypeAssert("Circle"), Field("radius")]
}
```

### Go Code Emission

The decision tree is walked to emit Go code:

#### Flat ADT Match

```golem
match shape do
  | Circle { radius } -> 3.14 * radius * radius
  | Rectangle { width, height } -> width * height
  | Triangle { base, height } -> 0.5 * base * height
end
```

Generated Go:

```go
func matchShape(shape Shape) float64 {
    switch v := shape.(type) {
    case Circle:
        radius := v.Radius
        return 3.14 * radius * radius
    case Rectangle:
        width := v.Width
        height := v.Height
        return width * height
    case Triangle:
        base := v.Base
        height := v.Height
        return 0.5 * base * height
    default:
        panic("unreachable: exhaustive match")
    }
}
```

#### Nested Pattern Match

```golem
match response do
  | Ok { value: User { name, role: Admin } } -> grantAccess(name)
  | Ok { value: User { name } } -> denyAccess(name)
  | Err { error } -> logError(error)
end
```

Generated Go:

```go
switch v1 := response.(type) {
case Ok[User]:
    v2 := v1.Value
    switch v2.Role.(type) {
    case Admin:
        name := v2.Name
        return grantAccess(name)
    default:
        name := v2.Name
        return denyAccess(name)
    }
case Err[User]:
    err := v1.Error
    return logError(err)
default:
    panic("unreachable: exhaustive match")
}
```

#### List Pattern Match

```golem
match items do
  | [] -> "empty"
  | [only] -> "single: #{only}"
  | [first, ..rest] -> "multiple, starting with #{first}"
end
```

Generated Go:

```go
if len(items) == 0 {
    return "empty"
} else if len(items) == 1 {
    only := items[0]
    return fmt.Sprintf("single: %v", only)
} else {
    first := items[0]
    rest := items[1:]
    return fmt.Sprintf("multiple, starting with %v", first)
}
```

#### Guard Clause Handling

Guards become `if` conditions within the matched branch. If the guard fails, control falls through to the next arm that matches the same pattern:

```golem
match user do
  | { age } if age >= 18 -> "adult"
  | { age } -> "minor"
end
```

Generated Go:

```go
age := user.Age
if age >= 18 {
    return "adult"
}
return "minor"
```

### Optimization: Sharing and Deduplication

When multiple arms have the same constructor test, the decision tree merges them into a single branch with sub-decisions. The code generator detects when two subtrees are identical and emits the code once, avoiding duplication.

For flat matches on sum types (the most common case), the decision tree degenerates to a simple flat switch — no optimization needed beyond what Go's type switch already provides.

---

## Match Expressions vs. Match Statements

In Golem, `match` is an expression — it returns a value. In Go, `switch` is a statement. The code generator handles this by:

1. If the match is in statement position (the result is not used), emit a normal `switch`.
2. If the match is in expression position, either:
   a. Emit the switch with a result variable assigned in each branch, or
   b. If the match is the last expression in a function body, use `return` in each branch.

```go
// Expression position: result variable
var result float64
switch v := shape.(type) {
case Circle:
    result = 3.14 * v.Radius * v.Radius
case Rectangle:
    result = v.Width * v.Height
// ...
}
// result is now available
```

---

## Integration with Type Checker

The exhaustiveness checker runs after type checking, so it has full type information. It knows:
- The complete set of constructors for every sum type
- The types of nested sub-patterns
- Whether a guard clause is present (making the arm non-exhaustive on its own)

Type errors in patterns are caught during type checking (Phase 4), not during exhaustiveness checking (Phase 5). The exhaustiveness checker assumes all patterns are well-typed.
