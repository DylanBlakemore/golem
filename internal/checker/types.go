// Package checker implements type inference and checking for Golem.
package checker

import (
	"fmt"
	"strings"
)

// TypeKind classifies the kind of a type.
type TypeKind int

const (
	KVar    TypeKind = iota // Unresolved type variable
	KCon                    // Concrete type constructor (Int, String, etc.)
	KApp                    // Type application: List<Int> = App(Con("List"), [Con("Int")])
	KFn                     // Function type (params -> return)
	KRecord                 // Record/struct type { field: Type, ... }
	KSum                    // Sum type (algebraic data type)
	KError                  // Poison type for error recovery
)

// Type represents a type in the Golem type system.
type Type struct {
	Kind   TypeKind
	Con    *ConType    // non-nil when Kind == KCon
	App    *AppType    // non-nil when Kind == KApp
	Fn     *FnType     // non-nil when Kind == KFn
	Record *RecordType // non-nil when Kind == KRecord
	Sum    *SumType    // non-nil when Kind == KSum
	Var    *TypeVar    // non-nil when Kind == KVar
}

// ConType represents a concrete type constructor.
type ConType struct {
	Name   string // "Int", "String", "Bool", "Float", "Nil", "Any"
	Module string // "" for builtins, package path for Go types
}

// AppType represents a type application, e.g. List<Int> or Result<T, E>.
type AppType struct {
	Con  *Type   // the type constructor being applied (e.g. Con("Result"))
	Args []*Type // type arguments
}

// FnType represents a function type.
type FnType struct {
	Params []*Type
	Return *Type
}

// RecordType represents a record/struct type.
type RecordType struct {
	Name   string         // type name (e.g. "Point")
	Fields []*RecordField // ordered fields
}

// RecordField represents a field in a record type.
type RecordField struct {
	Name string
	Type *Type
}

// SumType represents a sum type (algebraic data type).
type SumType struct {
	Name     string
	Variants []*SumVariant
}

// SumVariant represents a single variant of a sum type.
type SumVariant struct {
	Name   string
	Fields []*RecordField // nil/empty for unit variants
}

// TypeVar represents a type variable with union-find support.
type TypeVar struct {
	ID     uint64
	parent *TypeVar // nil if root
	rank   int
	Link   *Type // the resolved type, if this variable has been unified
}

// TypeScheme represents a polymorphic type (forall quantification).
// Vars are the IDs of the quantified type variables.
type TypeScheme struct {
	Vars []uint64
	Type *Type
}

// String returns a human-readable representation of the type.
//
//nolint:funlen // type-switch over all type kinds is naturally long
func (t *Type) String() string {
	if t == nil {
		return "<nil>"
	}
	switch t.Kind {
	case KVar:
		resolved := Find(t)
		if resolved.Kind != KVar {
			return resolved.String()
		}
		return fmt.Sprintf("?%d", t.Var.ID)
	case KCon:
		if t.Con.Module != "" {
			return t.Con.Module + "." + t.Con.Name
		}
		return t.Con.Name
	case KApp:
		var b strings.Builder
		b.WriteString(t.App.Con.String())
		b.WriteByte('<')
		for i, arg := range t.App.Args {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(arg.String())
		}
		b.WriteByte('>')
		return b.String()
	case KFn:
		var b strings.Builder
		for i, p := range t.Fn.Params {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(p.String())
		}
		return fmt.Sprintf("Fn(%s) -> %s", b.String(), t.Fn.Return.String())
	case KRecord:
		var b strings.Builder
		for i, f := range t.Record.Fields {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(f.Name)
			b.WriteString(": ")
			b.WriteString(f.Type.String())
		}
		return fmt.Sprintf("%s { %s }", t.Record.Name, b.String())
	case KSum:
		var b strings.Builder
		for i, v := range t.Sum.Variants {
			if i > 0 {
				b.WriteString(" | ")
			}
			b.WriteString(v.Name)
		}
		return fmt.Sprintf("%s(%s)", t.Sum.Name, b.String())
	case KError:
		return "<error>"
	default:
		return "<unknown>"
	}
}

// --- Constructors ---

// NewVar creates a fresh unbound type variable.
func NewVar(id uint64) *Type {
	return &Type{
		Kind: KVar,
		Var:  &TypeVar{ID: id},
	}
}

// NewCon creates a concrete type.
func NewCon(name string) *Type {
	return &Type{Kind: KCon, Con: &ConType{Name: name}}
}

// NewQualifiedCon creates a concrete type with a module qualifier.
func NewQualifiedCon(module, name string) *Type {
	return &Type{Kind: KCon, Con: &ConType{Name: name, Module: module}}
}

// NewApp creates a type application, e.g. List<Int>.
func NewApp(con *Type, args []*Type) *Type {
	return &Type{Kind: KApp, App: &AppType{Con: con, Args: args}}
}

// NewFn creates a function type.
func NewFn(params []*Type, ret *Type) *Type {
	return &Type{Kind: KFn, Fn: &FnType{Params: params, Return: ret}}
}

// NewRecord creates a record type.
func NewRecord(name string, fields []*RecordField) *Type {
	return &Type{Kind: KRecord, Record: &RecordType{Name: name, Fields: fields}}
}

// NewSum creates a sum type.
func NewSum(name string, variants []*SumVariant) *Type {
	return &Type{Kind: KSum, Sum: &SumType{Name: name, Variants: variants}}
}

// TypeError is the poison type that unifies with anything.
var TypeError = &Type{Kind: KError}

// Built-in types.
var (
	TypeInt    = NewCon("Int")
	TypeFloat  = NewCon("Float")
	TypeString = NewCon("String")
	TypeBool   = NewCon("Bool")
	TypeNil    = NewCon("Nil")
	TypeAny    = NewCon("Any")
)

// --- Union-Find ---

// Find follows union-find links to the root type.
// If the root is a type variable that has been linked to a concrete type,
// returns that concrete type. Uses path compression.
func Find(t *Type) *Type {
	if t.Kind != KVar {
		return t
	}
	root := findRoot(t.Var)
	// Path compression: point directly at root
	if t.Var != root {
		t.Var.parent = root
	}
	if root.Link != nil {
		return Find(root.Link)
	}
	// Return the type that owns the root var
	return t
}

// findRoot finds the root TypeVar in the union-find tree.
func findRoot(v *TypeVar) *TypeVar {
	for v.parent != nil {
		// Path compression
		if v.parent.parent != nil {
			v.parent = v.parent.parent
		}
		v = v.parent
	}
	return v
}

// link unifies two type variables using union by rank.
func link(a, b *TypeVar) {
	switch {
	case a.rank < b.rank:
		a.parent = b
	case a.rank > b.rank:
		b.parent = a
	default:
		b.parent = a
		a.rank++
	}
}
