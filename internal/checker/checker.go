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
	// declSchemes caches polymorphic type schemes for top-level declarations
	declSchemes map[string]*TypeScheme
	// recordDefs caches record type definitions
	recordDefs map[string]*RecordType
	// sumDefs caches sum type definitions
	sumDefs map[string]*SumType
	// variantToSum maps variant name -> parent sum type name
	variantToSum map[string]string
	// typeParamDefs maps generic type name -> list of type param names
	typeParamDefs map[string][]string
	// typeParamEnv maps type param name -> type var during generic type registration
	typeParamEnv map[string]*Type
}

// Check performs type checking on the given module with its resolution.
func Check(mod *ast.Module, res *resolver.Resolution) (*TypeInfo, []Error) {
	c := &Checker{
		module: mod,
		res:    res,
		info: &TypeInfo{
			Types: make(map[string]*Type),
		},
		env:           newTypeEnv(nil),
		declTypes:     make(map[string]*Type),
		declSchemes:   make(map[string]*TypeScheme),
		recordDefs:    make(map[string]*RecordType),
		sumDefs:       make(map[string]*SumType),
		variantToSum:  make(map[string]string),
		typeParamDefs: make(map[string][]string),
		typeParamEnv:  make(map[string]*Type),
	}
	c.check()
	c.info.Warnings = c.warnings
	return c.info, c.errors
}

func (c *Checker) check() {
	// Phase 0: Register built-in types (Result, Option).
	c.registerBuiltinTypes()

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
// For generic functions, it also registers a type scheme.
func (c *Checker) fnDeclType(fn *ast.FnDecl) *Type {
	// If generic, create fresh type vars for type parameters
	var typeParamVars []*Type
	if len(fn.TypeParams) > 0 {
		typeParamVars = make([]*Type, len(fn.TypeParams))
		for i, tp := range fn.TypeParams {
			tv := c.freshVar()
			typeParamVars[i] = tv
			c.typeParamEnv[tp] = tv
		}
	}

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

	fnType := NewFn(params, ret)

	// Build type scheme for generic functions
	if len(fn.TypeParams) > 0 {
		varIDs := make([]uint64, len(typeParamVars))
		for i, tv := range typeParamVars {
			varIDs[i] = tv.Var.ID
		}
		c.declSchemes[fn.Name] = &TypeScheme{Vars: varIDs, Type: fnType}
		// Clean up type param env
		for _, tp := range fn.TypeParams {
			delete(c.typeParamEnv, tp)
		}
	}

	return fnType
}

// registerBuiltinTypes pre-registers Result<T, E> and Option<T> as built-in
// sum types so they are available without user declarations.
func (c *Checker) registerBuiltinTypes() {
	// Option<T> = Some { value: T } | None
	{
		tv := c.freshVar()
		optionVariants := []*SumVariant{
			{Name: "Some", Fields: []*RecordField{{Name: "value", Type: tv}}},
			{Name: "None", Fields: nil},
		}
		optionDef := &SumType{Name: "Option", Variants: optionVariants}
		c.sumDefs["Option"] = optionDef
		c.typeParamDefs["Option"] = []string{"T"}
		c.variantToSum["Some"] = "Option"
		c.variantToSum["None"] = "Option"

		optionType := NewApp(NewSum("Option", optionVariants), []*Type{tv})
		c.env.define("Option", optionType)
		c.env.define("Some", optionType)
		c.env.define("None", optionType)
	}

	// Result<T, E> = Ok { value: T } | Err { error: E }
	{
		tvT := c.freshVar()
		tvE := c.freshVar()
		resultVariants := []*SumVariant{
			{Name: "Ok", Fields: []*RecordField{{Name: "value", Type: tvT}}},
			{Name: "Err", Fields: []*RecordField{{Name: "error", Type: tvE}}},
		}
		resultDef := &SumType{Name: "Result", Variants: resultVariants}
		c.sumDefs["Result"] = resultDef
		c.typeParamDefs["Result"] = []string{"T", "E"}
		c.variantToSum["Ok"] = "Result"
		c.variantToSum["Err"] = "Result"

		resultType := NewApp(NewSum("Result", resultVariants), []*Type{tvT, tvE})
		c.env.define("Result", resultType)
		c.env.define("Ok", resultType)
		c.env.define("Err", resultType)
	}
}

func (c *Checker) registerTypeDecl(td *ast.TypeDecl) {
	// If generic, create fresh type vars for type parameters
	if len(td.TypeParams) > 0 {
		c.typeParamDefs[td.Name] = td.TypeParams
		for _, tp := range td.TypeParams {
			c.typeParamEnv[tp] = c.freshVar()
		}
	}

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

		// For generic sum types, build a KApp type with the type param vars
		var sumType *Type
		if len(td.TypeParams) > 0 {
			args := make([]*Type, len(td.TypeParams))
			for i, tp := range td.TypeParams {
				args[i] = c.typeParamEnv[tp]
			}
			sumType = NewApp(NewSum(td.Name, variants), args)
		} else {
			sumType = NewSum(td.Name, variants)
		}

		c.env.define(td.Name, sumType)
		// Register each variant constructor in the env
		for _, v := range variants {
			c.env.define(v.Name, sumType)
		}
	}

	// Clear type param env after registration
	if len(td.TypeParams) > 0 {
		for _, tp := range td.TypeParams {
			delete(c.typeParamEnv, tp)
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
		// Check if there's a scheme for this binding
		if scheme := env.lookupScheme(e.Name); scheme != nil {
			return c.instantiate(scheme)
		}
		return t
	}
	// Check resolver for import refs
	ref := c.res.Lookup(e.Span)
	if ref != nil {
		switch ref.Kind { //nolint:exhaustive // only handling relevant cases
		case resolver.DeclFunction:
			if scheme, ok := c.declSchemes[ref.Name]; ok {
				return c.instantiate(scheme)
			}
			if t := c.declTypes[ref.Name]; t != nil {
				return t
			}
		case resolver.DeclImport:
			return NewQualifiedCon(ref.Name, ref.Name)
		case resolver.DeclVariant:
			// Unit variant constructor — produces the parent sum type
			if sumName, ok := c.variantToSum[ref.Name]; ok {
				sumDef := c.sumDefs[sumName]
				sumType := NewSum(sumName, sumDef.Variants)
				// For generic sum types, instantiate with fresh vars
				if tps, ok := c.typeParamDefs[sumName]; ok && len(tps) > 0 {
					args := make([]*Type, len(tps))
					for i := range tps {
						args[i] = c.freshVar()
					}
					return NewApp(sumType, args)
				}
				return sumType
			}
		}
	}
	c.error(e.Span, fmt.Sprintf("undefined variable %q", e.Name))
	return TypeError
}

// instantiate creates a fresh copy of a type scheme by replacing each
// quantified variable with a fresh type variable.
func (c *Checker) instantiate(scheme *TypeScheme) *Type {
	if len(scheme.Vars) == 0 {
		return scheme.Type
	}
	subst := make(map[uint64]*Type, len(scheme.Vars))
	for _, vid := range scheme.Vars {
		subst[vid] = c.freshVar()
	}
	return c.applySubst(scheme.Type, subst)
}

// applySubst recursively substitutes type variables in a type.
func (c *Checker) applySubst(t *Type, subst map[uint64]*Type) *Type {
	t = Find(t)
	switch t.Kind {
	case KVar:
		if replacement, ok := subst[findRoot(t.Var).ID]; ok {
			return replacement
		}
		return t
	case KCon:
		return t
	case KApp:
		newCon := c.applySubst(t.App.Con, subst)
		newArgs := make([]*Type, len(t.App.Args))
		for i, arg := range t.App.Args {
			newArgs[i] = c.applySubst(arg, subst)
		}
		return NewApp(newCon, newArgs)
	case KFn:
		newParams := make([]*Type, len(t.Fn.Params))
		for i, p := range t.Fn.Params {
			newParams[i] = c.applySubst(p, subst)
		}
		newRet := c.applySubst(t.Fn.Return, subst)
		return NewFn(newParams, newRet)
	case KRecord:
		newFields := make([]*RecordField, len(t.Record.Fields))
		for i, f := range t.Record.Fields {
			newFields[i] = &RecordField{Name: f.Name, Type: c.applySubst(f.Type, subst)}
		}
		return NewRecord(t.Record.Name, newFields)
	case KSum:
		newVariants := make([]*SumVariant, len(t.Sum.Variants))
		for i, v := range t.Sum.Variants {
			newFields := make([]*RecordField, len(v.Fields))
			for j, f := range v.Fields {
				newFields[j] = &RecordField{Name: f.Name, Type: c.applySubst(f.Type, subst)}
			}
			newVariants[i] = &SumVariant{Name: v.Name, Fields: newFields}
		}
		return NewSum(t.Sum.Name, newVariants)
	default:
		return t
	}
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
		bindType = annoType
	}
	// Generalize: if the value is a syntactic value (fn literal), generalize its type
	if c.isSyntacticValue(e.Value) {
		scheme := c.generalize(bindType, env)
		if len(scheme.Vars) > 0 {
			env.define(e.Name, bindType)
			env.defineScheme(e.Name, scheme)
			return TypeNil
		}
	}
	env.define(e.Name, bindType)
	return TypeNil
}

// isSyntacticValue returns true if the expression is a syntactic value
// (safe to generalize under the value restriction).
func (c *Checker) isSyntacticValue(expr ast.Expr) bool {
	switch expr.(type) {
	case *ast.FnLit, *ast.IntLit, *ast.FloatLit, *ast.StringLit, *ast.BoolLit, *ast.NilLit:
		return true
	default:
		return false
	}
}

// generalize collects free type variables in t that are not bound in env,
// and returns a TypeScheme quantifying over them.
func (c *Checker) generalize(t *Type, env *typeEnv) *TypeScheme {
	freeVars := c.freeTypeVars(t)
	envVars := c.envTypeVars(env)
	var quantified []uint64
	for vid := range freeVars {
		if !envVars[vid] {
			quantified = append(quantified, vid)
		}
	}
	return &TypeScheme{Vars: quantified, Type: t}
}

// freeTypeVars collects all unresolved type variable IDs in a type.
func (c *Checker) freeTypeVars(t *Type) map[uint64]bool {
	vars := make(map[uint64]bool)
	c.collectFreeVars(t, vars)
	return vars
}

func (c *Checker) collectFreeVars(t *Type, vars map[uint64]bool) {
	t = Find(t)
	switch t.Kind { //nolint:exhaustive // only collecting from structured types
	case KVar:
		vars[findRoot(t.Var).ID] = true
	case KApp:
		c.collectFreeVars(t.App.Con, vars)
		for _, arg := range t.App.Args {
			c.collectFreeVars(arg, vars)
		}
	case KFn:
		for _, p := range t.Fn.Params {
			c.collectFreeVars(p, vars)
		}
		c.collectFreeVars(t.Fn.Return, vars)
	case KRecord:
		for _, f := range t.Record.Fields {
			c.collectFreeVars(f.Type, vars)
		}
	case KSum:
		for _, v := range t.Sum.Variants {
			for _, f := range v.Fields {
				c.collectFreeVars(f.Type, vars)
			}
		}
	}
}

// envTypeVars collects all type variable IDs referenced in the environment.
func (c *Checker) envTypeVars(env *typeEnv) map[uint64]bool {
	vars := make(map[uint64]bool)
	for e := env; e != nil; e = e.parent {
		for _, t := range e.symbols {
			c.collectFreeVars(t, vars)
		}
	}
	return vars
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

//nolint:funlen // generic variant instantiation requires multi-step logic
func (c *Checker) inferVariantLit(e *ast.RecordLit, sumName string, env *typeEnv) *Type {
	sumDef := c.sumDefs[sumName]

	// For generic sum types, instantiate with fresh type vars
	var subst map[uint64]*Type
	isGeneric := false
	if tps, ok := c.typeParamDefs[sumName]; ok && len(tps) > 0 {
		isGeneric = true
		subst = make(map[uint64]*Type, len(tps))
		// We need to find the original type vars used during registration.
		// Re-read the env's type for the sum name and extract args from KApp.
		if envType := c.env.lookup(sumName); envType != nil {
			resolved := Find(envType)
			if resolved.Kind == KApp && len(resolved.App.Args) == len(tps) {
				for _, arg := range resolved.App.Args {
					resolved := Find(arg)
					if resolved.Kind == KVar {
						subst[findRoot(resolved.Var).ID] = c.freshVar()
					}
				}
			}
		}
	}

	// Find the variant definition
	var variantDef *SumVariant
	for _, v := range sumDef.Variants {
		if v.Name == e.Name {
			variantDef = v
			break
		}
	}

	// Build a map of expected field types (with substitution for generic types)
	expected := make(map[string]*Type, len(variantDef.Fields))
	for _, f := range variantDef.Fields {
		if isGeneric && len(subst) > 0 {
			expected[f.Name] = c.applySubst(f.Type, subst)
		} else {
			expected[f.Name] = f.Type
		}
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

	// Variant construction produces the parent sum type (possibly generic)
	baseSumType := NewSum(sumName, sumDef.Variants)
	if isGeneric {
		if tps, ok := c.typeParamDefs[sumName]; ok {
			args := make([]*Type, len(tps))
			for i := range tps {
				// Get the fresh vars we used for substitution
				found := false
				if envType := c.env.lookup(sumName); envType != nil {
					resolved := Find(envType)
					if resolved.Kind == KApp && i < len(resolved.App.Args) {
						origVar := Find(resolved.App.Args[i])
						if origVar.Kind == KVar {
							if replacement, ok := subst[findRoot(origVar.Var).ID]; ok {
								args[i] = replacement
								found = true
							}
						}
					}
				}
				if !found {
					args[i] = c.freshVar()
				}
			}
			return NewApp(baseSumType, args)
		}
	}
	return baseSumType
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
//
//nolint:funlen // pattern checking with generic type support is naturally long
func (c *Checker) checkPattern(pat ast.Pattern, expected *Type, env *typeEnv) {
	if pat == nil {
		return
	}

	switch p := pat.(type) {
	case *ast.ConstructorPattern:
		resolved := Find(expected)
		// Unwrap KApp to get the inner sum type for pattern matching
		var sumType *SumType
		switch resolved.Kind { //nolint:exhaustive // only handling sum-like types
		case KSum:
			sumType = resolved.Sum
		case KApp:
			inner := Find(resolved.App.Con)
			if inner.Kind == KSum {
				sumType = inner.Sum
			}
		}
		if sumType == nil {
			c.error(p.Span, fmt.Sprintf("cannot match constructor pattern against non-sum type %s", resolved))
			return
		}
		// Find the variant
		var variantDef *SumVariant
		for _, v := range sumType.Variants {
			if v.Name == p.Constructor {
				variantDef = v
				break
			}
		}
		if variantDef == nil {
			c.error(p.Span, fmt.Sprintf("unknown variant %q for type %s", p.Constructor, sumType.Name))
			return
		}
		// For generic types, build substitution from type params to actual args
		var fieldSubst map[uint64]*Type
		if resolved.Kind == KApp {
			if tps, ok := c.typeParamDefs[sumType.Name]; ok && len(tps) == len(resolved.App.Args) {
				fieldSubst = make(map[uint64]*Type, len(tps))
				// Get the original type vars from the registered type
				if envType := c.env.lookup(sumType.Name); envType != nil {
					envResolved := Find(envType)
					if envResolved.Kind == KApp && len(envResolved.App.Args) == len(tps) {
						for i, arg := range envResolved.App.Args {
							origVar := Find(arg)
							if origVar.Kind == KVar {
								fieldSubst[findRoot(origVar.Var).ID] = resolved.App.Args[i]
							}
						}
					}
				}
			}
		}
		// Bind field patterns
		fieldTypes := make(map[string]*Type, len(variantDef.Fields))
		for _, f := range variantDef.Fields {
			ft := f.Type
			if len(fieldSubst) > 0 {
				ft = c.applySubst(ft, fieldSubst)
			}
			fieldTypes[f.Name] = ft
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

//nolint:funlen // type-switch unification is naturally long
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
	case a.Kind == KApp && b.Kind == KApp:
		c.unify(a.App.Con, b.App.Con, s)
		if len(a.App.Args) != len(b.App.Args) {
			c.error(s, fmt.Sprintf("type argument count mismatch: %s vs %s", a, b))
			return
		}
		for i := range a.App.Args {
			c.unify(a.App.Args[i], b.App.Args[i], s)
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
	case KApp:
		if c.occursIn(v, t.App.Con) {
			return true
		}
		for _, arg := range t.App.Args {
			if c.occursIn(v, arg) {
				return true
			}
		}
		return false
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
		con := c.resolveNamedType(t.Name)
		args := make([]*Type, len(t.TypeArgs))
		for i, a := range t.TypeArgs {
			args[i] = c.resolveTypeExpr(a)
		}
		return NewApp(con, args)
	default:
		return c.freshVar()
	}
}

func (c *Checker) resolveNamedType(name string) *Type {
	// Check if it's a type parameter in scope
	if tv, ok := c.typeParamEnv[name]; ok {
		return tv
	}

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
	schemes map[string]*TypeScheme
}

func newTypeEnv(parent *typeEnv) *typeEnv {
	return &typeEnv{
		parent:  parent,
		symbols: make(map[string]*Type),
		schemes: make(map[string]*TypeScheme),
	}
}

func (e *typeEnv) child() *typeEnv {
	return newTypeEnv(e)
}

func (e *typeEnv) define(name string, t *Type) {
	e.symbols[name] = t
}

func (e *typeEnv) defineScheme(name string, scheme *TypeScheme) {
	e.schemes[name] = scheme
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

func (e *typeEnv) lookupScheme(name string) *TypeScheme {
	if s, ok := e.schemes[name]; ok {
		return s
	}
	if e.parent != nil {
		return e.parent.lookupScheme(name)
	}
	return nil
}
