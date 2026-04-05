package checker

import (
	"fmt"

	"github.com/dylanblakemore/golem/internal/ast"
	"github.com/dylanblakemore/golem/internal/resolver"
	"github.com/dylanblakemore/golem/internal/span"
)

// Error represents a type checking error.
type Error struct {
	Span    span.Span
	Message string
}

func (e Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Span, e.Message)
}

// TypeInfo stores the inferred types for AST nodes, keyed by span string.
type TypeInfo struct {
	Types    map[string]*Type
	Warnings []Warning
}

// Lookup returns the type for a given span, or nil.
func (ti *TypeInfo) Lookup(s span.Span) *Type {
	return ti.Types[spanKey(s)]
}

// Checker performs type inference and checking on a resolved Golem module.
type Checker struct {
	module   *ast.Module
	res      *resolver.Resolution
	info     *TypeInfo
	env      *typeEnv
	errors   []Error
	warnings []Warning
	nextID   uint64

	// declTypes caches the types of top-level declarations
	declTypes map[string]*Type
	// recordDefs caches record type definitions
	recordDefs map[string]*RecordType
	// sumDefs caches sum type definitions
	sumDefs map[string]*SumType
	// variantToSum maps variant name -> parent sum type name
	variantToSum map[string]string
}

// Check performs type checking on the given module with its resolution.
func Check(mod *ast.Module, res *resolver.Resolution) (*TypeInfo, []Error) {
	c := &Checker{
		module: mod,
		res:    res,
		info: &TypeInfo{
			Types: make(map[string]*Type),
		},
		env:          newTypeEnv(nil),
		declTypes:    make(map[string]*Type),
		recordDefs:   make(map[string]*RecordType),
		sumDefs:      make(map[string]*SumType),
		variantToSum: make(map[string]string),
	}
	c.check()
	c.info.Warnings = c.warnings
	return c.info, c.errors
}

func (c *Checker) check() {
	// Phase 1: Register all top-level declarations and record types.
	for _, decl := range c.module.Decls {
		switch d := decl.(type) {
		case *ast.FnDecl:
			fnType := c.fnDeclType(d)
			c.declTypes[d.Name] = fnType
			c.env.define(d.Name, fnType)
		case *ast.TypeDecl:
			c.registerTypeDecl(d)
		case *ast.LetDecl:
			// Top-level lets are deferred to phase 2
		}
	}

	// Phase 2: Type-check all declaration bodies.
	for _, decl := range c.module.Decls {
		switch d := decl.(type) {
		case *ast.FnDecl:
			c.checkFnDecl(d)
		case *ast.LetDecl:
			c.checkLetDecl(d)
		case *ast.TypeDecl:
			// Already registered
		}
	}
}

// fnDeclType builds the function type from a declaration's annotations.
func (c *Checker) fnDeclType(fn *ast.FnDecl) *Type {
	params := make([]*Type, len(fn.Params))
	for i, p := range fn.Params {
		if p.Type != nil {
			params[i] = c.resolveTypeExpr(p.Type)
		} else {
			params[i] = c.freshVar()
		}
	}
	var ret *Type
	if fn.ReturnType != nil {
		ret = c.resolveTypeExpr(fn.ReturnType)
	} else {
		ret = c.freshVar()
	}
	return NewFn(params, ret)
}

func (c *Checker) registerTypeDecl(td *ast.TypeDecl) {
	switch body := td.Body.(type) {
	case *ast.RecordTypeBody:
		fields := make([]*RecordField, len(body.Fields))
		for i, f := range body.Fields {
			fields[i] = &RecordField{
				Name: f.Name,
				Type: c.resolveTypeExpr(f.Type),
			}
		}
		rec := &RecordType{Name: td.Name, Fields: fields}
		c.recordDefs[td.Name] = rec
		c.env.define(td.Name, NewRecord(td.Name, fields))
	case *ast.SumTypeBody:
		variants := make([]*SumVariant, len(body.Variants))
		for i, v := range body.Variants {
			fields := make([]*RecordField, len(v.Fields))
			for j, f := range v.Fields {
				fields[j] = &RecordField{
					Name: f.Name,
					Type: c.resolveTypeExpr(f.Type),
				}
			}
			variants[i] = &SumVariant{Name: v.Name, Fields: fields}
			c.variantToSum[v.Name] = td.Name
		}
		sumDef := &SumType{Name: td.Name, Variants: variants}
		c.sumDefs[td.Name] = sumDef
		sumType := NewSum(td.Name, variants)
		c.env.define(td.Name, sumType)
		// Register each variant constructor in the env
		for _, v := range variants {
			c.env.define(v.Name, sumType)
		}
	}
}

func (c *Checker) checkFnDecl(fn *ast.FnDecl) {
	fnType := c.declTypes[fn.Name]
	if fnType == nil {
		return
	}

	env := c.env.child()
	// Bind parameters
	for i, p := range fn.Params {
		env.define(p.Name, fnType.Fn.Params[i])
	}

	bodyType := c.checkBody(fn.Body, env)
	if bodyType != nil {
		c.unify(fnType.Fn.Return, bodyType, fn.Span)
	}
	c.record(fn.Span, fnType)
}

func (c *Checker) checkLetDecl(ld *ast.LetDecl) {
	valType := c.inferExpr(ld.Value, c.env)
	bindType := valType
	if ld.TypeAnno != nil {
		annoType := c.resolveTypeExpr(ld.TypeAnno)
		c.unify(annoType, valType, ld.Span)
		bindType = annoType
	}
	c.env.define(ld.Name, bindType)
	c.record(ld.Span, bindType)
}

// checkBody infers the type of a body (sequence of expressions).
// The type of the body is the type of the last expression.
func (c *Checker) checkBody(body []ast.Expr, env *typeEnv) *Type {
	var last *Type
	for _, expr := range body {
		last = c.inferExpr(expr, env)
	}
	return last
}

// inferExpr infers the type of an expression.
func (c *Checker) inferExpr(expr ast.Expr, env *typeEnv) *Type {
	if expr == nil {
		return TypeError
	}
	t := c.inferExprKind(expr, env)
	c.record(expr.GetSpan(), t)
	return t
}

//nolint:funlen // type-switch over AST nodes is naturally long
func (c *Checker) inferExprKind(expr ast.Expr, env *typeEnv) *Type {
	switch e := expr.(type) {
	case *ast.IntLit:
		return TypeInt
	case *ast.FloatLit:
		return TypeFloat
	case *ast.StringLit:
		return TypeString
	case *ast.BoolLit:
		return TypeBool
	case *ast.NilLit:
		return TypeNil
	case *ast.Ident:
		return c.inferIdent(e, env)
	case *ast.BinaryExpr:
		return c.inferBinary(e, env)
	case *ast.UnaryExpr:
		return c.inferUnary(e, env)
	case *ast.CallExpr:
		return c.inferCall(e, env)
	case *ast.FieldAccessExpr:
		return c.inferFieldAccess(e, env)
	case *ast.BlockExpr:
		return c.inferBlock(e, env)
	case *ast.IfExpr:
		return c.inferIf(e, env)
	case *ast.LetExpr:
		return c.inferLet(e, env)
	case *ast.ReturnExpr:
		return c.inferReturn(e, env)
	case *ast.StringInterpolation:
		return c.inferStringInterp(e, env)
	case *ast.MatchExpr:
		return c.inferMatch(e, env)
	case *ast.RecordLit:
		return c.inferRecordLit(e, env)
	case *ast.FnLit:
		return c.inferFnLit(e, env)
	case *ast.BadExpr:
		return TypeError
	default:
		return TypeError
	}
}

func (c *Checker) inferIdent(e *ast.Ident, env *typeEnv) *Type {
	// Check local env first
	if t := env.lookup(e.Name); t != nil {
		return t
	}
	// Check resolver for import refs
	ref := c.res.Lookup(e.Span)
	if ref != nil {
		switch ref.Kind { //nolint:exhaustive // only handling relevant cases
		case resolver.DeclFunction:
			if t := c.declTypes[ref.Name]; t != nil {
				return t
			}
		case resolver.DeclImport:
			return NewQualifiedCon(ref.Name, ref.Name)
		case resolver.DeclVariant:
			// Unit variant constructor — produces the parent sum type
			if sumName, ok := c.variantToSum[ref.Name]; ok {
				sumDef := c.sumDefs[sumName]
				return NewSum(sumName, sumDef.Variants)
			}
		}
	}
	c.error(e.Span, fmt.Sprintf("undefined variable %q", e.Name))
	return TypeError
}

func (c *Checker) inferBinary(e *ast.BinaryExpr, env *typeEnv) *Type {
	left := c.inferExpr(e.Left, env)
	right := c.inferExpr(e.Right, env)

	switch e.Op {
	case ast.OpAdd, ast.OpSub, ast.OpMul, ast.OpDiv, ast.OpMod:
		// Arithmetic: both operands must agree, result is the same type.
		// For now, allow Int or Float.
		c.unify(left, right, e.Span)
		return left
	case ast.OpEq, ast.OpNeq:
		c.unify(left, right, e.Span)
		return TypeBool
	case ast.OpLt, ast.OpGt, ast.OpLte, ast.OpGte:
		c.unify(left, right, e.Span)
		return TypeBool
	case ast.OpAnd, ast.OpOr:
		c.unify(left, TypeBool, e.Span)
		c.unify(right, TypeBool, e.Span)
		return TypeBool
	case ast.OpConcat:
		c.unify(left, TypeString, e.Span)
		c.unify(right, TypeString, e.Span)
		return TypeString
	case ast.OpPipe:
		// a |> f  is  f(a) — the right side must be a function
		result := c.freshVar()
		expected := NewFn([]*Type{left}, result)
		c.unify(right, expected, e.Span)
		return result
	default:
		return TypeError
	}
}

func (c *Checker) inferUnary(e *ast.UnaryExpr, env *typeEnv) *Type {
	operand := c.inferExpr(e.Operand, env)
	switch e.Op {
	case ast.OpNeg:
		// Negation: operand must be numeric, result is same type
		return operand
	case ast.OpNot:
		c.unify(operand, TypeBool, e.Span)
		return TypeBool
	default:
		return TypeError
	}
}

func (c *Checker) inferCall(e *ast.CallExpr, env *typeEnv) *Type {
	// Special case: qualified call like fmt.println(...)
	if fa, ok := e.Func.(*ast.FieldAccessExpr); ok {
		if ident, ok := fa.Expr.(*ast.Ident); ok {
			ref := c.res.Lookup(ident.Span)
			if ref != nil && ref.Kind == resolver.DeclImport {
				// Go import call — we don't type-check Go functions in Phase 0.
				// Infer arguments but return Any.
				for _, arg := range e.Args {
					c.inferExpr(arg, env)
				}
				c.record(fa.Span, TypeAny)
				c.record(ident.GetSpan(), TypeAny)
				return TypeAny
			}
		}
	}

	funcType := c.inferExpr(e.Func, env)
	argTypes := make([]*Type, len(e.Args))
	for i, arg := range e.Args {
		argTypes[i] = c.inferExpr(arg, env)
	}

	result := c.freshVar()
	expected := NewFn(argTypes, result)
	c.unify(funcType, expected, e.Span)
	return result
}

func (c *Checker) inferFieldAccess(e *ast.FieldAccessExpr, env *typeEnv) *Type {
	// Check if this is an import member access (already resolved by resolver)
	if ident, ok := e.Expr.(*ast.Ident); ok {
		ref := c.res.Lookup(ident.Span)
		if ref != nil && ref.Kind == resolver.DeclImport {
			c.record(ident.GetSpan(), TypeAny)
			return TypeAny
		}
	}

	exprType := c.inferExpr(e.Expr, env)
	resolved := Find(exprType)

	if resolved.Kind == KRecord {
		for _, f := range resolved.Record.Fields {
			if f.Name == e.Field {
				return f.Type
			}
		}
		c.error(e.Span, fmt.Sprintf("no field %q on type %s", e.Field, resolved))
		return TypeError
	}

	if resolved.Kind == KError {
		return TypeError
	}

	// If it's an unresolved variable, we can't look up fields yet.
	// For Phase 0, emit an error.
	if resolved.Kind == KVar {
		c.error(e.Span, fmt.Sprintf("cannot access field %q on unresolved type", e.Field))
		return TypeError
	}

	c.error(e.Span, fmt.Sprintf("cannot access field %q on type %s", e.Field, resolved))
	return TypeError
}

func (c *Checker) inferBlock(e *ast.BlockExpr, env *typeEnv) *Type {
	childEnv := env.child()
	result := c.checkBody(e.Stmts, childEnv)
	if result == nil {
		return TypeNil
	}
	return result
}

func (c *Checker) inferIf(e *ast.IfExpr, env *typeEnv) *Type {
	condType := c.inferExpr(e.Cond, env)
	c.unify(condType, TypeBool, e.Cond.GetSpan())

	thenType := c.checkBody(e.Then, env.child())
	if e.Else != nil {
		elseType := c.checkBody(e.Else, env.child())
		if thenType != nil && elseType != nil {
			c.unify(thenType, elseType, e.Span)
		}
	}
	if thenType == nil {
		return TypeNil
	}
	return thenType
}

func (c *Checker) inferLet(e *ast.LetExpr, env *typeEnv) *Type {
	valType := c.inferExpr(e.Value, env)
	bindType := valType
	if e.TypeAnno != nil {
		annoType := c.resolveTypeExpr(e.TypeAnno)
		c.unify(annoType, valType, e.Span)
		// Use annotation type for the binding (programmer's intent)
		bindType = annoType
	}
	env.define(e.Name, bindType)
	// Let expressions don't produce a value
	return TypeNil
}

func (c *Checker) inferReturn(e *ast.ReturnExpr, env *typeEnv) *Type {
	if e.Value != nil {
		return c.inferExpr(e.Value, env)
	}
	return TypeNil
}

func (c *Checker) inferStringInterp(e *ast.StringInterpolation, env *typeEnv) *Type {
	for _, part := range e.Parts {
		if interp, ok := part.(*ast.StringInterpExpr); ok {
			c.inferExpr(interp.Expr, env)
		}
	}
	return TypeString
}

func (c *Checker) inferRecordLit(e *ast.RecordLit, env *typeEnv) *Type {
	// Check if this is a variant constructor
	if sumName, ok := c.variantToSum[e.Name]; ok {
		return c.inferVariantLit(e, sumName, env)
	}

	recDef, ok := c.recordDefs[e.Name]
	if !ok {
		c.error(e.Span, fmt.Sprintf("undefined record type %q", e.Name))
		// Still infer field values
		for _, f := range e.Fields {
			c.inferExpr(f.Value, env)
		}
		return TypeError
	}

	// Build a map of expected field types
	expected := make(map[string]*Type, len(recDef.Fields))
	for _, f := range recDef.Fields {
		expected[f.Name] = f.Type
	}

	// Check provided fields
	provided := make(map[string]bool, len(e.Fields))
	for _, f := range e.Fields {
		provided[f.Name] = true
		valType := c.inferExpr(f.Value, env)
		if expType, ok := expected[f.Name]; ok {
			c.unify(expType, valType, f.Span)
		} else {
			c.error(f.Span, fmt.Sprintf("unknown field %q on type %s", f.Name, e.Name))
		}
	}

	// Check for missing fields
	for _, f := range recDef.Fields {
		if !provided[f.Name] {
			c.error(e.Span, fmt.Sprintf("missing field %q in %s literal", f.Name, e.Name))
		}
	}

	return NewRecord(e.Name, recDef.Fields)
}

func (c *Checker) inferVariantLit(e *ast.RecordLit, sumName string, env *typeEnv) *Type {
	sumDef := c.sumDefs[sumName]

	// Find the variant definition
	var variantDef *SumVariant
	for _, v := range sumDef.Variants {
		if v.Name == e.Name {
			variantDef = v
			break
		}
	}

	// Build a map of expected field types
	expected := make(map[string]*Type, len(variantDef.Fields))
	for _, f := range variantDef.Fields {
		expected[f.Name] = f.Type
	}

	// Check provided fields
	provided := make(map[string]bool, len(e.Fields))
	for _, f := range e.Fields {
		provided[f.Name] = true
		valType := c.inferExpr(f.Value, env)
		if expType, ok := expected[f.Name]; ok {
			c.unify(expType, valType, f.Span)
		} else {
			c.error(f.Span, fmt.Sprintf("unknown field %q on variant %s", f.Name, e.Name))
		}
	}

	// Check for missing fields
	for _, f := range variantDef.Fields {
		if !provided[f.Name] {
			c.error(e.Span, fmt.Sprintf("missing field %q in %s literal", f.Name, e.Name))
		}
	}

	// Variant construction produces the parent sum type
	return NewSum(sumName, sumDef.Variants)
}

func (c *Checker) inferMatch(e *ast.MatchExpr, env *typeEnv) *Type {
	scrutineeType := c.inferExpr(e.Scrutinee, env)

	resultType := c.freshVar()

	for _, arm := range e.Arms {
		armEnv := env.child()
		c.checkPattern(arm.Pattern, scrutineeType, armEnv)
		armType := c.checkBody(arm.Body, armEnv)
		if armType != nil {
			c.unify(resultType, armType, arm.Span)
		}
	}

	c.checkMatchExhaustive(e, scrutineeType)

	return resultType
}

// checkPattern type-checks a pattern against the expected type and introduces bindings.
func (c *Checker) checkPattern(pat ast.Pattern, expected *Type, env *typeEnv) {
	if pat == nil {
		return
	}

	switch p := pat.(type) {
	case *ast.ConstructorPattern:
		resolved := Find(expected)
		if resolved.Kind != KSum {
			c.error(p.Span, fmt.Sprintf("cannot match constructor pattern against non-sum type %s", resolved))
			return
		}
		// Find the variant
		var variantDef *SumVariant
		for _, v := range resolved.Sum.Variants {
			if v.Name == p.Constructor {
				variantDef = v
				break
			}
		}
		if variantDef == nil {
			c.error(p.Span, fmt.Sprintf("unknown variant %q for type %s", p.Constructor, resolved.Sum.Name))
			return
		}
		// Bind field patterns
		fieldTypes := make(map[string]*Type, len(variantDef.Fields))
		for _, f := range variantDef.Fields {
			fieldTypes[f.Name] = f.Type
		}
		for _, fp := range p.Fields {
			ft, ok := fieldTypes[fp.Name]
			if !ok {
				c.error(fp.Span, fmt.Sprintf("unknown field %q in variant %s", fp.Name, p.Constructor))
				continue
			}
			if fp.Pattern != nil {
				c.checkPattern(fp.Pattern, ft, env)
			} else {
				// Shorthand: bind field name as variable
				env.define(fp.Name, ft)
			}
		}

	case *ast.VarPattern:
		env.define(p.Name, expected)

	case *ast.WildcardPattern:
		// matches anything, no bindings

	case *ast.LiteralPattern:
		litType := c.inferExpr(p.Value, env)
		c.unify(litType, expected, p.Span)
	}
}

func (c *Checker) inferFnLit(e *ast.FnLit, env *typeEnv) *Type {
	childEnv := env.child()
	params := make([]*Type, len(e.Params))
	for i, p := range e.Params {
		if p.Type != nil {
			params[i] = c.resolveTypeExpr(p.Type)
		} else {
			params[i] = c.freshVar()
		}
		childEnv.define(p.Name, params[i])
	}

	var retType *Type
	if e.ReturnType != nil {
		retType = c.resolveTypeExpr(e.ReturnType)
	}

	bodyType := c.checkBody(e.Body, childEnv)
	if retType != nil && bodyType != nil {
		c.unify(retType, bodyType, e.Span)
	} else if retType == nil {
		retType = bodyType
	}
	if retType == nil {
		retType = TypeNil
	}

	return NewFn(params, retType)
}

// --- Unification ---

func (c *Checker) unify(a, b *Type, s span.Span) {
	a = Find(a)
	b = Find(b)

	if a == b {
		return
	}

	// Error types unify with anything (poison)
	if a.Kind == KError || b.Kind == KError {
		return
	}

	switch {
	case a.Kind == KVar && b.Kind == KVar:
		link(a.Var, b.Var)
	case a.Kind == KVar:
		if c.occursIn(a.Var, b) {
			c.error(s, "infinite type detected")
			return
		}
		a.Var.Link = b
	case b.Kind == KVar:
		if c.occursIn(b.Var, a) {
			c.error(s, "infinite type detected")
			return
		}
		b.Var.Link = a
	case a.Kind == KCon && b.Kind == KCon:
		if a.Con.Name != b.Con.Name || a.Con.Module != b.Con.Module {
			c.error(s, fmt.Sprintf("type mismatch: expected %s, got %s", a, b))
		}
	case a.Kind == KFn && b.Kind == KFn:
		if len(a.Fn.Params) != len(b.Fn.Params) {
			c.error(s, fmt.Sprintf("function arity mismatch: expected %d params, got %d",
				len(a.Fn.Params), len(b.Fn.Params)))
			return
		}
		for i := range a.Fn.Params {
			c.unify(a.Fn.Params[i], b.Fn.Params[i], s)
		}
		c.unify(a.Fn.Return, b.Fn.Return, s)
	case a.Kind == KRecord && b.Kind == KRecord:
		c.unifyRecords(a, b, s)
	case a.Kind == KSum && b.Kind == KSum:
		if a.Sum.Name != b.Sum.Name {
			c.error(s, fmt.Sprintf("type mismatch: expected %s, got %s", a.Sum.Name, b.Sum.Name))
		}
	default:
		c.error(s, fmt.Sprintf("type mismatch: expected %s, got %s", a, b))
	}
}

func (c *Checker) unifyRecords(a, b *Type, s span.Span) {
	aFields := make(map[string]*Type, len(a.Record.Fields))
	for _, f := range a.Record.Fields {
		aFields[f.Name] = f.Type
	}
	bFields := make(map[string]*Type, len(b.Record.Fields))
	for _, f := range b.Record.Fields {
		bFields[f.Name] = f.Type
	}

	// Unify common fields
	for name, at := range aFields {
		if bt, ok := bFields[name]; ok {
			c.unify(at, bt, s)
		}
	}

	// Check for extra fields in either record
	if a.Record.Name != "" && b.Record.Name != "" && a.Record.Name != b.Record.Name {
		c.error(s, fmt.Sprintf("type mismatch: expected %s, got %s", a.Record.Name, b.Record.Name))
	}
}

// occursIn checks if a type variable occurs in a type (prevents infinite types).
func (c *Checker) occursIn(v *TypeVar, t *Type) bool {
	t = Find(t)
	switch t.Kind {
	case KVar:
		return findRoot(v) == findRoot(t.Var)
	case KFn:
		for _, p := range t.Fn.Params {
			if c.occursIn(v, p) {
				return true
			}
		}
		return c.occursIn(v, t.Fn.Return)
	case KRecord:
		for _, f := range t.Record.Fields {
			if c.occursIn(v, f.Type) {
				return true
			}
		}
		return false
	case KSum:
		for _, variant := range t.Sum.Variants {
			for _, f := range variant.Fields {
				if c.occursIn(v, f.Type) {
					return true
				}
			}
		}
		return false
	default:
		return false
	}
}

// --- Type resolution ---

// resolveTypeExpr converts a syntax type expression to a semantic type.
func (c *Checker) resolveTypeExpr(te ast.TypeExpr) *Type {
	if te == nil {
		return c.freshVar()
	}
	switch t := te.(type) {
	case *ast.NamedType:
		return c.resolveNamedType(t.Name)
	case *ast.QualifiedType:
		return NewQualifiedCon(t.Qualifier, t.Name)
	case *ast.FnType:
		params := make([]*Type, len(t.ParamTypes))
		for i, p := range t.ParamTypes {
			params[i] = c.resolveTypeExpr(p)
		}
		ret := c.resolveTypeExpr(t.ReturnType)
		return NewFn(params, ret)
	case *ast.PointerType:
		// Pointer types pass through to codegen; treat as Any for type checking.
		return TypeAny
	case *ast.GenericType:
		// Phase 0: treat generic types as Any
		return TypeAny
	default:
		return c.freshVar()
	}
}

func (c *Checker) resolveNamedType(name string) *Type {
	switch name {
	case "Int":
		return TypeInt
	case "Float":
		return TypeFloat
	case "String":
		return TypeString
	case "Bool":
		return TypeBool
	case "Any":
		return TypeAny
	default:
		// Check if it's a user-defined record type
		if rec, ok := c.recordDefs[name]; ok {
			return NewRecord(name, rec.Fields)
		}
		// Check if it's a user-defined sum type
		if sum, ok := c.sumDefs[name]; ok {
			return NewSum(name, sum.Variants)
		}
		return NewQualifiedCon("", name)
	}
}

// --- Helpers ---

func (c *Checker) freshVar() *Type {
	c.nextID++
	return NewVar(c.nextID)
}

func (c *Checker) record(s span.Span, t *Type) {
	c.info.Types[spanKey(s)] = t
}

func (c *Checker) error(s span.Span, msg string) {
	c.errors = append(c.errors, Error{Span: s, Message: msg})
}

func (c *Checker) warning(s span.Span, msg string) {
	c.warnings = append(c.warnings, Warning{Span: s, Message: msg})
}

func spanKey(s span.Span) string {
	return fmt.Sprintf("%s:%d:%d-%d:%d", s.File, s.Start.Line, s.Start.Column, s.End.Line, s.End.Column)
}

// --- Type environment ---

type typeEnv struct {
	parent  *typeEnv
	symbols map[string]*Type
}

func newTypeEnv(parent *typeEnv) *typeEnv {
	return &typeEnv{
		parent:  parent,
		symbols: make(map[string]*Type),
	}
}

func (e *typeEnv) child() *typeEnv {
	return newTypeEnv(e)
}

func (e *typeEnv) define(name string, t *Type) {
	e.symbols[name] = t
}

func (e *typeEnv) lookup(name string) *Type {
	if t, ok := e.symbols[name]; ok {
		return t
	}
	if e.parent != nil {
		return e.parent.lookup(name)
	}
	return nil
}
