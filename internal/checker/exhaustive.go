package checker

import (
	"fmt"
	"strings"

	"github.com/dylanblakemore/golem/internal/ast"
	"github.com/dylanblakemore/golem/internal/span"
)

// Warning represents a non-fatal compiler diagnostic.
type Warning struct {
	Span    span.Span
	Message string
}

func (w Warning) Error() string {
	return fmt.Sprintf("%s: %s", w.Span, w.Message)
}

// constructor represents a type constructor for the exhaustiveness algorithm.
type constructor struct {
	name       string
	arity      int
	fieldTypes []*Type
	fieldNames []string
}

// checkMatchExhaustive validates that a match expression covers all possible
// values of the scrutinee type (Maranget algorithm) and detects redundant arms.
func (c *Checker) checkMatchExhaustive(e *ast.MatchExpr, scrutineeType *Type) {
	resolved := Find(scrutineeType)
	if resolved.Kind == KError || resolved.Kind == KVar {
		return
	}

	matrix := make([][]ast.Pattern, len(e.Arms))
	for i, arm := range e.Arms {
		matrix[i] = []ast.Pattern{arm.Pattern}
	}
	types := []*Type{resolved}

	missing := c.findMissing(matrix, types)
	if len(missing) > 0 {
		c.error(e.Span, fmt.Sprintf("non-exhaustive match, missing patterns: %s",
			strings.Join(missing, ", ")))
	}

	for i := 1; i < len(e.Arms); i++ {
		if !c.isUseful(matrix[:i], matrix[i], types) {
			c.warning(e.Arms[i].Span, "unreachable match arm")
		}
	}
}

// typeConstructors returns the complete constructor set for finite types.
// Returns nil for infinite types (Int, Float, String).
func (c *Checker) typeConstructors(t *Type) []constructor {
	t = Find(t)
	switch t.Kind {
	case KSum:
		ctors := make([]constructor, len(t.Sum.Variants))
		for i, v := range t.Sum.Variants {
			ft := make([]*Type, len(v.Fields))
			fn := make([]string, len(v.Fields))
			for j, f := range v.Fields {
				ft[j] = f.Type
				fn[j] = f.Name
			}
			ctors[i] = constructor{
				name:       v.Name,
				arity:      len(v.Fields),
				fieldTypes: ft,
				fieldNames: fn,
			}
		}
		return ctors
	case KCon:
		if t.Con.Name == "Bool" {
			return []constructor{
				{name: "true", arity: 0},
				{name: "false", arity: 0},
			}
		}
		return nil
	default:
		return nil
	}
}

// patternInfo extracts the constructor name and wildcard status from a pattern.
func patternInfo(p ast.Pattern) (name string, isWild bool) {
	switch pat := p.(type) {
	case *ast.ConstructorPattern:
		return pat.Constructor, false
	case *ast.LiteralPattern:
		return literalPatternName(pat), false
	case *ast.VarPattern:
		return "", true
	case *ast.WildcardPattern:
		return "", true
	default:
		return "", true
	}
}

func literalPatternName(p *ast.LiteralPattern) string {
	switch v := p.Value.(type) {
	case *ast.BoolLit:
		if v.Value {
			return "true"
		}
		return "false"
	case *ast.IntLit:
		return v.Value
	case *ast.FloatLit:
		return v.Value
	case *ast.StringLit:
		return fmt.Sprintf("%q", v.Value)
	default:
		return "_"
	}
}

// subPatterns returns the sub-patterns for a pattern matching a constructor.
// For constructor patterns, field sub-patterns are extracted (or wildcards for bindings).
// For wildcards/vars, returns arity wildcards.
func subPatterns(p ast.Pattern, ctor constructor) []ast.Pattern {
	if cp, ok := p.(*ast.ConstructorPattern); ok && cp.Constructor == ctor.name {
		fieldPats := make(map[string]ast.Pattern, len(cp.Fields))
		for _, fp := range cp.Fields {
			if fp.Pattern != nil {
				fieldPats[fp.Name] = fp.Pattern
			}
		}
		result := make([]ast.Pattern, ctor.arity)
		for i, name := range ctor.fieldNames {
			if nested, ok := fieldPats[name]; ok {
				result[i] = nested
			} else {
				result[i] = &ast.WildcardPattern{}
			}
		}
		return result
	}
	result := make([]ast.Pattern, ctor.arity)
	for i := range result {
		result[i] = &ast.WildcardPattern{}
	}
	return result
}

// specialize produces the specialized matrix for constructor ctor at column 0.
// Rows matching ctor have column 0 replaced by sub-patterns.
// Wildcard/var rows are expanded with arity wildcards.
// Rows with a different constructor are discarded.
func specialize(matrix [][]ast.Pattern, ctor constructor) [][]ast.Pattern {
	var result [][]ast.Pattern
	for _, row := range matrix {
		name, isWild := patternInfo(row[0])
		if isWild || name == ctor.name {
			sub := subPatterns(row[0], ctor)
			newRow := make([]ast.Pattern, 0, len(sub)+len(row)-1)
			newRow = append(newRow, sub...)
			newRow = append(newRow, row[1:]...)
			result = append(result, newRow)
		}
	}
	return result
}

// defaultMatrix returns rows where column 0 is a wildcard/variable, with column 0 removed.
func defaultMatrix(matrix [][]ast.Pattern) [][]ast.Pattern {
	var result [][]ast.Pattern
	for _, row := range matrix {
		_, isWild := patternInfo(row[0])
		if isWild {
			newRow := make([]ast.Pattern, len(row)-1)
			copy(newRow, row[1:])
			result = append(result, newRow)
		}
	}
	return result
}

// expandDefault prepends arity wildcards to each row of the default matrix
// for checking a missing constructor's sub-patterns.
func expandDefault(def [][]ast.Pattern, ctor constructor) [][]ast.Pattern {
	result := make([][]ast.Pattern, len(def))
	for i, row := range def {
		newRow := make([]ast.Pattern, 0, ctor.arity+len(row))
		for range ctor.arity {
			newRow = append(newRow, &ast.WildcardPattern{})
		}
		newRow = append(newRow, row...)
		result[i] = newRow
	}
	return result
}

// findMissing returns human-readable descriptions of uncovered patterns,
// or nil if the match is exhaustive.
func (c *Checker) findMissing(matrix [][]ast.Pattern, types []*Type) []string {
	if len(types) == 0 {
		if len(matrix) == 0 {
			return []string{""}
		}
		return nil
	}

	typ := Find(types[0])
	ctors := c.typeConstructors(typ)
	restTypes := types[1:]

	if ctors == nil {
		// Infinite type: must have a wildcard/variable catch-all
		def := defaultMatrix(matrix)
		if len(def) == 0 {
			return []string{"_"}
		}
		return c.findMissing(def, restTypes)
	}

	// Finite type
	present := make(map[string]bool)
	hasWild := false
	for _, row := range matrix {
		name, wild := patternInfo(row[0])
		if wild {
			hasWild = true
		} else {
			present[name] = true
		}
	}

	allPresent := hasWild
	if !allPresent {
		allPresent = true
		for _, ctor := range ctors {
			if !present[ctor.name] {
				allPresent = false
				break
			}
		}
	}

	if allPresent {
		var missing []string
		for _, ctor := range ctors {
			spec := specialize(matrix, ctor)
			subTypes := concatTypes(ctor.fieldTypes, restTypes)
			sub := c.findMissing(spec, subTypes)
			for _, m := range sub {
				missing = append(missing, prependCtorName(ctor, m))
			}
		}
		return missing
	}

	// Some constructors missing
	return c.findMissingIncomplete(matrix, ctors, present, restTypes)
}

// findMissingIncomplete handles the case where not all constructors are present.
func (c *Checker) findMissingIncomplete(
	matrix [][]ast.Pattern, ctors []constructor, present map[string]bool, restTypes []*Type,
) []string {
	def := defaultMatrix(matrix)
	var missing []string
	for _, ctor := range ctors {
		if present[ctor.name] {
			continue
		}
		if len(def) == 0 {
			missing = append(missing, formatCtorPattern(ctor))
		} else {
			expanded := expandDefault(def, ctor)
			subTypes := concatTypes(ctor.fieldTypes, restTypes)
			sub := c.findMissing(expanded, subTypes)
			for _, m := range sub {
				missing = append(missing, prependCtorName(ctor, m))
			}
		}
	}
	return missing
}

// isUseful returns true if pattern row can match a value not covered by matrix.
func (c *Checker) isUseful(matrix [][]ast.Pattern, row []ast.Pattern, types []*Type) bool {
	if len(types) == 0 {
		return len(matrix) == 0
	}

	typ := Find(types[0])
	ctors := c.typeConstructors(typ)
	restTypes := types[1:]

	name, isWild := patternInfo(row[0])

	if !isWild {
		ctor := c.findCtor(ctors, name)
		spec := specialize(matrix, ctor)
		sub := subPatterns(row[0], ctor)
		newRow := make([]ast.Pattern, 0, len(sub)+len(row)-1)
		newRow = append(newRow, sub...)
		newRow = append(newRow, row[1:]...)
		subTypes := concatTypes(ctor.fieldTypes, restTypes)
		return c.isUseful(spec, newRow, subTypes)
	}

	// Wildcard/var pattern
	if ctors == nil {
		def := defaultMatrix(matrix)
		return c.isUseful(def, row[1:], restTypes)
	}

	// Finite type: wildcard is useful if useful for at least one constructor
	for _, ctor := range ctors {
		spec := specialize(matrix, ctor)
		sub := make([]ast.Pattern, ctor.arity)
		for i := range sub {
			sub[i] = &ast.WildcardPattern{}
		}
		newRow := make([]ast.Pattern, 0, len(sub)+len(row)-1)
		newRow = append(newRow, sub...)
		newRow = append(newRow, row[1:]...)
		subTypes := concatTypes(ctor.fieldTypes, restTypes)
		if c.isUseful(spec, newRow, subTypes) {
			return true
		}
	}
	return false
}

// findCtor finds the constructor definition by name, or creates an arity-0
// constructor for literals on infinite types.
func (c *Checker) findCtor(ctors []constructor, name string) constructor {
	for _, ct := range ctors {
		if ct.name == name {
			return ct
		}
	}
	return constructor{name: name, arity: 0}
}

// --- formatting helpers ---

func formatCtorPattern(ctor constructor) string {
	if ctor.arity == 0 {
		return ctor.name
	}
	fields := make([]string, ctor.arity)
	for i, name := range ctor.fieldNames {
		fields[i] = name + ": _"
	}
	return fmt.Sprintf("%s { %s }", ctor.name, strings.Join(fields, ", "))
}

func prependCtorName(ctor constructor, subMissing string) string {
	if subMissing == "" {
		return formatCtorPattern(ctor)
	}
	return formatCtorPattern(ctor)
}

func concatTypes(a, b []*Type) []*Type {
	result := make([]*Type, 0, len(a)+len(b))
	result = append(result, a...)
	result = append(result, b...)
	return result
}
