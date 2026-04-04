# Phase 2 — Full Pattern Matching & Interop

**Status:** Not Started
**Depends on:** Phase 1 complete
**Goal:** Nested patterns, guards, complete Go interop, escape hatch, project structure, formatter.
**Exit Criteria:** A non-trivial production service built entirely in Golem.

---

## 2.1 — Nested Pattern Matching with Destructuring

Reference: [pattern-matching.md](../architecture/pattern-matching.md)

### Parsing
- [ ] Nested constructor patterns:
  ```golem
  | Ok { value: User { name, role: Admin } } -> ...
  ```
- [ ] Nested record patterns:
  ```golem
  | { config: { host, port } } -> ...
  ```
- [ ] Arbitrary nesting depth

### Type Checking
- [ ] Recursively type-check nested patterns against nested types
- [ ] Variable bindings at any nesting level introduced in scope

### Exhaustiveness
- [ ] Extend pattern matrix to handle nested constructors
- [ ] Specialization descends into sub-patterns
- [ ] Missing pattern reconstruction for nested cases produces readable messages:
  ```
  Non-exhaustive match. Missing: Ok { value: User { role: Member } }
  ```

### Code Generation — Decision Trees
- [ ] Build decision tree from pattern matrix:
  ```go
  type Switch struct {
      Path     AccessPath
      Branches []Branch
      Default  *Decision
  }
  type Leaf struct {
      ArmIndex int
      Bindings []Binding
  }
  ```
- [ ] Nested type assertions with intermediate variable bindings:
  ```go
  switch v1 := response.(type) {
  case ResultOk[User, Error]:
      switch v2 := v1.Value.(type) {
      case UserAdmin:
          grantAccess(v2.Name)
      default:
          denyAccess(v1.Value.Name)
      }
  case ResultErr[User, Error]:
      logError(v1.Error)
  }
  ```
- [ ] Identical subtrees merged (sharing optimization)

### Tests
- [ ] Parse nested patterns (2+ levels deep)
- [ ] Type check nested pattern variable bindings
- [ ] Exhaustiveness on nested patterns
- [ ] Code gen snapshot tests for nested switch chains
- [ ] Test subtree sharing optimization

---

## 2.2 — Guard Clauses in Match Expressions

Reference: [pattern-matching.md](../architecture/pattern-matching.md)

### Parsing
- [ ] Guard syntax: `| pattern if condition -> body`
- [ ] Guard is an arbitrary boolean expression
- [ ] Guard can reference variables bound in the pattern

### Type Checking
- [ ] Guard expression must be Bool
- [ ] Variables bound in pattern are in scope for guard
- [ ] Guarded arms do not count toward exhaustiveness (may fail at runtime)

### Exhaustiveness
- [ ] Arms with guards treated as potentially non-matching
- [ ] Non-guarded wildcard/variable still required for infinite types
- [ ] For finite types: guarded arms don't satisfy constructor coverage

### Code Generation
- [ ] Guard becomes `if` condition within matched branch:
  ```go
  case UserRecord:
      if v.Age >= 18 {
          result = "adult"
      } else {
          // fall through to next arm
      }
  ```
- [ ] Fall-through handling: when guard fails, continue to next matching arm
  - Implementation: flatten to sequential if-else chain rather than Go switch fall-through

### Tests
- [ ] Parse guard clauses
- [ ] Type check guard expressions (must be Bool)
- [ ] Guard variables from pattern are in scope
- [ ] Exhaustiveness with guards
- [ ] Code gen: guard condition in generated Go

---

## 2.3 — List Patterns

Reference: [pattern-matching.md](../architecture/pattern-matching.md)

### Parsing
- [ ] Empty list: `[]`
- [ ] Head/tail: `[head, ..tail]`
- [ ] Fixed length: `[a, b, c]`
- [ ] Nested: `[Ok { value }, ..rest]`

### Type Checking
- [ ] List patterns checked against `List<T>`
- [ ] Head elements have type `T`, tail has type `List<T>`
- [ ] Fixed-length patterns: each element has type `T`

### Exhaustiveness
- [ ] List has two constructors: empty (`[]`) and non-empty (`[_, .._]`)
- [ ] Both must be covered for exhaustiveness
- [ ] Fixed-length patterns don't satisfy non-empty coverage alone

### Code Generation
- [ ] `[]` -> `if len(list) == 0`
- [ ] `[head, ..tail]` -> `head := list[0]; tail := list[1:]`
- [ ] `[a, b, c]` -> `if len(list) == 3 { a := list[0]; ... }`
- [ ] Chained if-else for multiple list patterns

### Tests
- [ ] Parse all list pattern forms
- [ ] Type check list patterns
- [ ] Exhaustiveness: `[]` + `[_, .._]` is complete
- [ ] Code gen for list pattern matching

---

## 2.4 — Decision Tree Compilation

Reference: [pattern-matching.md](../architecture/pattern-matching.md) — Decision Trees section

### Implementation
- [ ] `AccessPath` type: sequence of field accesses / type assertions to reach sub-value
- [ ] `Decision` interface: `Switch`, `Leaf`, `Fail`
- [ ] `Branch`: constructor, bindings, sub-decision
- [ ] Build decision tree from pattern matrix using column selection heuristic
- [ ] Column selection: prefer column with most distinct constructors
- [ ] Leaf nodes carry arm index and accumulated bindings

### Optimizations
- [ ] Identical subtree detection and sharing
- [ ] Minimize type assertions (reuse already-asserted values)
- [ ] Flatten single-branch switches to direct access

### Integration
- [ ] Replace flat code gen from Phase 1 with decision tree-based emission
- [ ] All existing pattern matching tests still pass

### Tests
- [ ] Unit test decision tree construction
- [ ] Test column selection heuristic
- [ ] Test subtree sharing
- [ ] Regression: all Phase 1 pattern matching code gen still correct

---

## 2.5 — Complete Go Interop

Reference: [go-interop.md](../architecture/go-interop.md)

### Generic Go Functions
- [ ] Map Go generic function type parameters to Golem type variables
- [ ] Infer type arguments at Golem call sites
- [ ] Emit explicit Go type arguments in generated code

### Struct Embedding
- [ ] Detect Go struct embedding
- [ ] Promote embedded fields in Golem record type
- [ ] Code gen accesses embedded fields correctly

### Variadic Functions
- [ ] Detect Go variadic parameters (`...T`)
- [ ] Map to `List<T>` as last parameter in Golem
- [ ] Code gen expands with `...` operator in Go call

### Pointer/Nil Handling
- [ ] `*T` -> `Option<T>` at FFI boundary
- [ ] Generate nil check at call sites:
  ```go
  if rawResult == nil {
      golemResult = None[User]{}
  } else {
      golemResult = Some[User]{Value: *rawResult}
  }
  ```
- [ ] Handle double pointers (`**T`)

### Named Interfaces
- [ ] Map Go named interfaces (e.g., `io.Reader`) to opaque Golem types
- [ ] Allow passing Golem values where Go expects an interface (structural typing at boundary)
- [ ] v1 limitation: cannot implement Go interfaces from Golem (document this)

### Tests
- [ ] Test generic Go function calls
- [ ] Test struct embedding access
- [ ] Test variadic function calls
- [ ] Test pointer/nil handling at FFI boundary
- [ ] Test named interface usage
- [ ] Integration: call complex Go APIs (net/http handler, io operations)

---

## 2.6 — `@goraw` Escape Hatch

Reference: [go-interop.md](../architecture/go-interop.md)

- [ ] Parse `@goraw("...")` annotation
- [ ] Emit raw Go expression verbatim in generated code
- [ ] Emit compiler warning when `@goraw` is used
- [ ] `@goraw` expressions have type `Any` (bypass type checking)
- [ ] Document intended use cases: interface implementation, complex type assertions

### Tests
- [ ] Parse `@goraw` annotations
- [ ] Code gen emits raw Go unchanged
- [ ] Warning is emitted
- [ ] Generated code with `@goraw` compiles

---

## 2.7 — Multi-File Project Structure

Reference: [go-interop.md](../architecture/go-interop.md) — Build Integration

### Implementation
- [ ] Discover `.golem` files recursively in project
- [ ] Relative imports (`./handlers`) resolve to Golem modules
- [ ] Package mapping: root -> `main`, subdirectories -> package named after dir
- [ ] Cross-file name resolution (imports between Golem modules)
- [ ] Cross-file type checking (exported types visible to importers)
- [ ] Build directory mirrors source tree:
  ```
  myproject/
  ├── main.golem        -> build/main.golem.go
  ├── handlers/
  │   └── handlers.golem -> build/handlers/handlers.golem.go
  ```

### Visibility Enforcement
- [ ] `pub` declarations visible to other modules
- [ ] `priv` declarations only visible within same module/file
- [ ] Error: accessing private declaration from another module

### Tests
- [ ] Multi-file project compiles correctly
- [ ] Cross-file function calls work
- [ ] Cross-file type references work
- [ ] Private declaration access errors
- [ ] Build directory structure is correct

---

## 2.8 — `golem fmt` (Formatter)

- [ ] Define canonical Golem formatting rules:
  - 2-space indentation
  - Blank line between top-level declarations
  - Consistent spacing around operators
  - Pattern alignment in match expressions
- [ ] Implement formatter: parse -> pretty-print
- [ ] Idempotent: formatting already-formatted code produces same output
- [ ] CLI: `golem fmt` formats all `.golem` files in module
- [ ] CLI: `golem fmt <file>` formats specific file
- [ ] `--check` flag: exit non-zero if any file would change

### Tests
- [ ] Idempotency tests
- [ ] Formatting edge cases (long lines, nested blocks)
- [ ] `--check` flag behavior

---

## 2.9 — `golem check` (Type-Check Only)

- [ ] Run pipeline through type checking and exhaustiveness, skip code gen
- [ ] Report all diagnostics (errors and warnings)
- [ ] Exit code: 0 if no errors, 1 if errors
- [ ] Faster than `golem build` (no code gen, no `go build`)

### Tests
- [ ] `golem check` reports type errors
- [ ] `golem check` reports exhaustiveness errors
- [ ] `golem check` succeeds on valid code
- [ ] Faster than `golem build` (benchmark)

---

## 2.10 — End-to-End Integration Test

**Target:** A non-trivial HTTP service with nested pattern matching, Go interop, error handling.

```golem
import "net/http"
import "encoding/json"
import "fmt"
import "os"

type ApiResponse<T> =
  | Success { data: T }
  | NotFound { message: String }
  | ServerError { cause: String }

type User = { name: String, role: Role }
type Role =
  | Admin
  | Member { team: String }

pub fn handleUser(w: http.ResponseWriter, r: http.Request) do
  let response = loadUser(r.URL.Path)
  match response do
    | Success { data: User { name, role: Admin } } ->
      writeJson(w, 200, "Admin: " <> name)
    | Success { data: User { name, role: Member { team } } } ->
      writeJson(w, 200, "Member: " <> name <> " (" <> team <> ")")
    | NotFound { message } ->
      writeJson(w, 404, message)
    | ServerError { cause } ->
      writeJson(w, 500, cause)
  end
end
```

- [ ] Nested pattern matching generates correctly
- [ ] Guard clauses work in pattern matching
- [ ] List patterns work
- [ ] Multi-file project structure works
- [ ] Go interop covers generics, variadics, embedding
- [ ] `@goraw` escape hatch works
- [ ] `golem fmt` and `golem check` work
- [ ] All Phase 2 tests pass
