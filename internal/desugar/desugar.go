// Package desugar implements the desugaring pass for the Golem compiler.
//
// It transforms the parsed AST into a simpler form by expanding syntactic sugar:
//   - Pipe operator: a |> f(b) -> f(a, b)
//   - String interpolation: "Hello, #{name}" -> fmt.Sprintf("Hello, %v", name)
//   - Implicit priv: bare fn -> explicit VisPriv
//   - Visibility mapping: pub -> capitalize, priv -> lowercase
package desugar

import (
	"strings"
	"unicode"

	"github.com/dylanblakemore/golem/internal/ast"
)

// Result holds the desugared module and metadata about the transformation.
type Result struct {
	Module   *ast.Module
	NeedsFmt bool              // true if string interpolation desugaring introduced fmt usage
	NameMap  map[string]string // original name -> Go name mapping
}

// Desugar transforms a module's AST by expanding syntactic sugar.
func Desugar(mod *ast.Module) *Result {
	d := &desugarer{
		nameMap: make(map[string]string),
	}
	result := d.desugarModule(mod)
	return &Result{
		Module:   result,
		NeedsFmt: d.needsFmt,
		NameMap:  d.nameMap,
	}
}

type desugarer struct {
	needsFmt bool
	nameMap  map[string]string // original fn name -> Go-visible name
}

func (d *desugarer) desugarModule(mod *ast.Module) *ast.Module {
	// Phase 1: Build name map from declarations and normalize visibility.
	for _, decl := range mod.Decls {
		if fn, ok := decl.(*ast.FnDecl); ok {
			if fn.Visibility == ast.VisDefault {
				fn.Visibility = ast.VisPriv
			}
			goName := mapVisibility(fn.Name, fn.Visibility)
			d.nameMap[fn.Name] = goName
		}
		if td, ok := decl.(*ast.TypeDecl); ok {
			if td.Visibility == ast.VisDefault {
				td.Visibility = ast.VisPriv
			}
			goName := mapVisibility(td.Name, td.Visibility)
			d.nameMap[td.Name] = goName
			// Map variant constructors: Circle -> ShapeCircle (prefixed with Go type name)
			if sumBody, ok := td.Body.(*ast.SumTypeBody); ok {
				for _, v := range sumBody.Variants {
					d.nameMap[v.Name] = goName + v.Name
				}
			}
		}
	}

	// Phase 2: Desugar all declaration bodies.
	newDecls := make([]ast.Decl, len(mod.Decls))
	for i, decl := range mod.Decls {
		newDecls[i] = d.desugarDecl(decl)
	}

	// Build new import list, adding "fmt" if needed by string interpolation.
	newImports := make([]*ast.ImportDecl, len(mod.Imports))
	copy(newImports, mod.Imports)
	if d.needsFmt && !hasImport(mod.Imports, "fmt") {
		newImports = append(newImports, &ast.ImportDecl{Path: "fmt"})
	}

	return &ast.Module{
		File:    mod.File,
		Imports: newImports,
		Decls:   newDecls,
	}
}

func (d *desugarer) desugarDecl(decl ast.Decl) ast.Decl {
	switch dc := decl.(type) {
	case *ast.FnDecl:
		return d.desugarFnDecl(dc)
	case *ast.LetDecl:
		return d.desugarLetDecl(dc)
	case *ast.TypeDecl:
		return d.desugarTypeDecl(dc)
	default:
		return decl
	}
}

func (d *desugarer) desugarFnDecl(fn *ast.FnDecl) *ast.FnDecl {
	goName := d.nameMap[fn.Name]
	newBody := d.desugarExprs(fn.Body)
	newParams := d.desugarParams(fn.Params)
	return &ast.FnDecl{
		Span:       fn.Span,
		Visibility: fn.Visibility,
		Name:       goName,
		TypeParams: fn.TypeParams,
		Params:     newParams,
		ReturnType: d.desugarTypeExpr(fn.ReturnType),
		Body:       newBody,
	}
}

func (d *desugarer) desugarTypeDecl(td *ast.TypeDecl) *ast.TypeDecl {
	goName := td.Name
	if n, ok := d.nameMap[td.Name]; ok {
		goName = n
	}

	var body ast.TypeBody
	switch b := td.Body.(type) {
	case *ast.SumTypeBody:
		variants := make([]*ast.Variant, len(b.Variants))
		for i, v := range b.Variants {
			variantGoName := v.Name
			if n, ok := d.nameMap[v.Name]; ok {
				variantGoName = n
			}
			variants[i] = &ast.Variant{
				Span:   v.Span,
				Name:   variantGoName,
				Fields: v.Fields,
			}
		}
		body = &ast.SumTypeBody{
			Span:     b.Span,
			Variants: variants,
		}
	default:
		body = td.Body
	}

	return &ast.TypeDecl{
		Span:       td.Span,
		Visibility: td.Visibility,
		Name:       goName,
		TypeParams: td.TypeParams,
		Body:       body,
	}
}

func (d *desugarer) desugarLetDecl(ld *ast.LetDecl) *ast.LetDecl {
	return &ast.LetDecl{
		Span:     ld.Span,
		Name:     ld.Name,
		TypeAnno: ld.TypeAnno,
		Value:    d.desugarExpr(ld.Value),
	}
}

func (d *desugarer) desugarParams(params []*ast.Param) []*ast.Param {
	result := make([]*ast.Param, len(params))
	for i, p := range params {
		result[i] = &ast.Param{
			Span: p.Span,
			Name: p.Name,
			Type: d.desugarTypeExpr(p.Type),
		}
	}
	return result
}

// desugarTypeExpr applies name mapping to type expressions.
func (d *desugarer) desugarTypeExpr(te ast.TypeExpr) ast.TypeExpr {
	if te == nil {
		return nil
	}
	switch t := te.(type) {
	case *ast.NamedType:
		if goName, ok := d.nameMap[t.Name]; ok {
			return &ast.NamedType{Span: t.Span, Name: goName}
		}
		return t
	case *ast.GenericType:
		name := t.Name
		if goName, ok := d.nameMap[name]; ok {
			name = goName
		}
		args := make([]ast.TypeExpr, len(t.TypeArgs))
		for i, a := range t.TypeArgs {
			args[i] = d.desugarTypeExpr(a)
		}
		return &ast.GenericType{Span: t.Span, Name: name, TypeArgs: args}
	case *ast.FnType:
		params := make([]ast.TypeExpr, len(t.ParamTypes))
		for i, p := range t.ParamTypes {
			params[i] = d.desugarTypeExpr(p)
		}
		return &ast.FnType{
			Span:       t.Span,
			ParamTypes: params,
			ReturnType: d.desugarTypeExpr(t.ReturnType),
		}
	case *ast.PointerType:
		return &ast.PointerType{Span: t.Span, Elem: d.desugarTypeExpr(t.Elem)}
	default:
		return te
	}
}

func (d *desugarer) desugarExprs(exprs []ast.Expr) []ast.Expr {
	result := make([]ast.Expr, len(exprs))
	for i, e := range exprs {
		result[i] = d.desugarExpr(e)
	}
	return result
}

//nolint:funlen // type-switch over AST nodes is naturally long
func (d *desugarer) desugarExpr(expr ast.Expr) ast.Expr {
	if expr == nil {
		return nil
	}

	switch e := expr.(type) {
	case *ast.BinaryExpr:
		if e.Op == ast.OpPipe {
			return d.desugarPipe(e)
		}
		return &ast.BinaryExpr{
			Span:  e.Span,
			Op:    e.Op,
			Left:  d.desugarExpr(e.Left),
			Right: d.desugarExpr(e.Right),
		}

	case *ast.StringInterpolation:
		return d.desugarStringInterpolation(e)

	case *ast.Ident:
		if goName, ok := d.nameMap[e.Name]; ok {
			return &ast.Ident{Span: e.Span, Name: goName}
		}
		return e

	case *ast.UnaryExpr:
		return &ast.UnaryExpr{
			Span:    e.Span,
			Op:      e.Op,
			Operand: d.desugarExpr(e.Operand),
		}

	case *ast.CallExpr:
		return &ast.CallExpr{
			Span: e.Span,
			Func: d.desugarExpr(e.Func),
			Args: d.desugarExprs(e.Args),
		}

	case *ast.FieldAccessExpr:
		return &ast.FieldAccessExpr{
			Span:  e.Span,
			Expr:  d.desugarExpr(e.Expr),
			Field: e.Field,
		}

	case *ast.BlockExpr:
		return &ast.BlockExpr{
			Span:  e.Span,
			Stmts: d.desugarExprs(e.Stmts),
		}

	case *ast.IfExpr:
		return &ast.IfExpr{
			Span: e.Span,
			Cond: d.desugarExpr(e.Cond),
			Then: d.desugarExprs(e.Then),
			Else: d.desugarExprs(e.Else),
		}

	case *ast.LetExpr:
		return &ast.LetExpr{
			Span:     e.Span,
			Name:     e.Name,
			TypeAnno: e.TypeAnno,
			Value:    d.desugarExpr(e.Value),
		}

	case *ast.ReturnExpr:
		return &ast.ReturnExpr{
			Span:  e.Span,
			Value: d.desugarExpr(e.Value),
		}

	case *ast.RecordLit:
		newFields := make([]*ast.FieldInit, len(e.Fields))
		for i, f := range e.Fields {
			newFields[i] = &ast.FieldInit{
				Span:  f.Span,
				Name:  f.Name,
				Value: d.desugarExpr(f.Value),
			}
		}
		name := e.Name
		if goName, ok := d.nameMap[name]; ok {
			name = goName
		}
		return &ast.RecordLit{
			Span:   e.Span,
			Name:   name,
			Fields: newFields,
		}

	case *ast.FnLit:
		return &ast.FnLit{
			Span:       e.Span,
			Params:     d.desugarParams(e.Params),
			ReturnType: e.ReturnType,
			Body:       d.desugarExprs(e.Body),
		}

	case *ast.MatchExpr:
		return d.desugarMatchExpr(e)

	case *ast.IntLit, *ast.FloatLit, *ast.StringLit, *ast.BoolLit, *ast.NilLit, *ast.BadExpr:
		return e

	default:
		return expr
	}
}

func (d *desugarer) desugarMatchExpr(e *ast.MatchExpr) *ast.MatchExpr {
	arms := make([]*ast.MatchArm, len(e.Arms))
	for i, arm := range e.Arms {
		arms[i] = &ast.MatchArm{
			Span:    arm.Span,
			Pattern: d.desugarPattern(arm.Pattern),
			Body:    d.desugarExprs(arm.Body),
		}
	}
	return &ast.MatchExpr{
		Span:      e.Span,
		Scrutinee: d.desugarExpr(e.Scrutinee),
		Arms:      arms,
	}
}

func (d *desugarer) desugarPattern(pat ast.Pattern) ast.Pattern {
	if pat == nil {
		return nil
	}
	switch p := pat.(type) {
	case *ast.ConstructorPattern:
		name := p.Constructor
		if goName, ok := d.nameMap[name]; ok {
			name = goName
		}
		fields := make([]*ast.FieldPattern, len(p.Fields))
		for i, fp := range p.Fields {
			fields[i] = &ast.FieldPattern{
				Span:    fp.Span,
				Name:    fp.Name,
				Pattern: d.desugarPattern(fp.Pattern),
			}
		}
		return &ast.ConstructorPattern{
			Span:        p.Span,
			Constructor: name,
			Fields:      fields,
		}
	default:
		return pat
	}
}

// desugarPipe transforms a |> f(b) into f(a, b), or a |> f into f(a).
func (d *desugarer) desugarPipe(e *ast.BinaryExpr) ast.Expr {
	left := d.desugarExpr(e.Left)
	right := d.desugarExpr(e.Right)

	// If RHS is a function call, prepend LHS as first argument.
	if call, ok := right.(*ast.CallExpr); ok {
		newArgs := make([]ast.Expr, 0, len(call.Args)+1)
		newArgs = append(newArgs, left)
		newArgs = append(newArgs, call.Args...)
		return &ast.CallExpr{
			Span: e.Span,
			Func: call.Func,
			Args: newArgs,
		}
	}

	// Otherwise, wrap RHS as a function call with LHS as the sole argument.
	return &ast.CallExpr{
		Span: e.Span,
		Func: right,
		Args: []ast.Expr{left},
	}
}

// desugarStringInterpolation transforms "Hello, #{name}" into
// fmt.Sprintf("Hello, %v", name).
func (d *desugarer) desugarStringInterpolation(e *ast.StringInterpolation) ast.Expr {
	d.needsFmt = true

	var formatParts []string
	var args []ast.Expr

	for _, part := range e.Parts {
		switch p := part.(type) {
		case *ast.StringText:
			// Escape any existing % characters for Sprintf.
			formatParts = append(formatParts, strings.ReplaceAll(p.Value, "%", "%%"))
		case *ast.StringInterpExpr:
			formatParts = append(formatParts, "%v")
			args = append(args, d.desugarExpr(p.Expr))
		}
	}

	formatStr := strings.Join(formatParts, "")

	// If there are no interpolated expressions, just return a plain string.
	if len(args) == 0 {
		return &ast.StringLit{
			Span:  e.Span,
			Value: formatStr,
		}
	}

	// Build fmt.Sprintf(formatStr, args...)
	sprintfFunc := &ast.FieldAccessExpr{
		Span:  e.Span,
		Expr:  &ast.Ident{Span: e.Span, Name: "fmt"},
		Field: "Sprintf",
	}

	allArgs := make([]ast.Expr, 0, len(args)+1)
	allArgs = append(allArgs, &ast.StringLit{Span: e.Span, Value: formatStr})
	allArgs = append(allArgs, args...)

	return &ast.CallExpr{
		Span: e.Span,
		Func: sprintfFunc,
		Args: allArgs,
	}
}

// mapVisibility applies Go visibility rules to a name.
// pub -> capitalize first letter, priv -> lowercase first letter.
// Special case: "main" is never capitalized (Go entry point).
func mapVisibility(name string, vis ast.Visibility) string {
	if len(name) == 0 {
		return name
	}
	if name == "main" {
		return name
	}
	runes := []rune(name)
	switch vis {
	case ast.VisPub:
		runes[0] = unicode.ToUpper(runes[0])
	case ast.VisPriv, ast.VisDefault:
		runes[0] = unicode.ToLower(runes[0])
	}
	return string(runes)
}

// hasImport checks if an import path is already present.
func hasImport(imports []*ast.ImportDecl, path string) bool {
	for _, imp := range imports {
		if imp.Path == path {
			return true
		}
	}
	return false
}

// GoName returns the Go-visible name for a Golem identifier based on visibility.
// Exported for use by code generation.
func GoName(name string, vis ast.Visibility) string {
	return mapVisibility(name, vis)
}

// FormatString builds the fmt.Sprintf format string and arguments for a
// StringInterpolation node. Exported for testing.
func FormatString(parts []ast.StringPart) (format string, interpCount int) {
	var b strings.Builder
	for _, part := range parts {
		switch p := part.(type) {
		case *ast.StringText:
			b.WriteString(strings.ReplaceAll(p.Value, "%", "%%"))
		case *ast.StringInterpExpr:
			_ = p
			b.WriteString("%v")
			interpCount++
		}
	}
	return b.String(), interpCount
}
