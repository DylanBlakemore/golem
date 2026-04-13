// Package golifter inserts GoLiftCallExpr wrappers around calls to Go functions
// that return (T, error) or error, converting them to Golem Result values.
//
// It runs after type-checking (which validates that the return types map correctly)
// and before desugaring. The pass uses the loader to inspect Go function signatures
// and the resolver to identify which call sites target imported Go packages.
package golifter

import (
	"go/types"

	"github.com/dylanblakemore/golem/internal/ast"
	"github.com/dylanblakemore/golem/internal/goloader"
	"github.com/dylanblakemore/golem/internal/resolver"
)

// Lift walks the module and wraps qualifying Go call expressions in GoLiftCallExpr.
// Returns the module unchanged if loader is nil (no Go type information available).
func Lift(mod *ast.Module, res *resolver.Resolution, loader *goloader.Loader) *ast.Module {
	if loader == nil {
		return mod
	}
	l := &lifter{res: res, loader: loader}
	return l.liftModule(mod)
}

type lifter struct {
	res    *resolver.Resolution
	loader *goloader.Loader
}

func (l *lifter) liftModule(mod *ast.Module) *ast.Module {
	newDecls := make([]ast.Decl, len(mod.Decls))
	for i, decl := range mod.Decls {
		newDecls[i] = l.liftDecl(decl)
	}
	return &ast.Module{
		File:    mod.File,
		Imports: mod.Imports,
		Decls:   newDecls,
	}
}

func (l *lifter) liftDecl(decl ast.Decl) ast.Decl {
	switch d := decl.(type) {
	case *ast.FnDecl:
		newBody := l.liftExprs(d.Body)
		return &ast.FnDecl{
			Span:       d.Span,
			Name:       d.Name,
			Visibility: d.Visibility,
			TypeParams: d.TypeParams,
			Params:     d.Params,
			ReturnType: d.ReturnType,
			Body:       newBody,
		}
	case *ast.LetDecl:
		return &ast.LetDecl{
			Span:  d.Span,
			Name:  d.Name,
			Value: l.liftExpr(d.Value),
		}
	default:
		return decl
	}
}

func (l *lifter) liftExprs(exprs []ast.Expr) []ast.Expr {
	result := make([]ast.Expr, len(exprs))
	for i, e := range exprs {
		result[i] = l.liftExpr(e)
	}
	return result
}

//nolint:cyclop,funlen // expression type-switch is necessarily exhaustive
func (l *lifter) liftExpr(expr ast.Expr) ast.Expr {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case *ast.CallExpr:
		return l.liftCallExpr(e)
	case *ast.LetExpr:
		return &ast.LetExpr{
			Span:     e.Span,
			Name:     e.Name,
			TypeAnno: e.TypeAnno,
			Value:    l.liftExpr(e.Value),
		}
	case *ast.BinaryExpr:
		return &ast.BinaryExpr{
			Span:  e.Span,
			Op:    e.Op,
			Left:  l.liftExpr(e.Left),
			Right: l.liftExpr(e.Right),
		}
	case *ast.UnaryExpr:
		return &ast.UnaryExpr{
			Span:    e.Span,
			Op:      e.Op,
			Operand: l.liftExpr(e.Operand),
		}
	case *ast.IfExpr:
		return &ast.IfExpr{
			Span: e.Span,
			Cond: l.liftExpr(e.Cond),
			Then: l.liftExprs(e.Then),
			Else: l.liftExprs(e.Else),
		}
	case *ast.MatchExpr:
		newArms := make([]*ast.MatchArm, len(e.Arms))
		for i, arm := range e.Arms {
			var guard ast.Expr
			if arm.Guard != nil {
				guard = l.liftExpr(arm.Guard)
			}
			newArms[i] = &ast.MatchArm{
				Span:    arm.Span,
				Pattern: arm.Pattern,
				Guard:   guard,
				Body:    l.liftExprs(arm.Body),
			}
		}
		return &ast.MatchExpr{
			Span:      e.Span,
			Scrutinee: l.liftExpr(e.Scrutinee),
			Arms:      newArms,
		}
	case *ast.BlockExpr:
		return &ast.BlockExpr{
			Span:  e.Span,
			Stmts: l.liftExprs(e.Stmts),
		}
	case *ast.ReturnExpr:
		return &ast.ReturnExpr{
			Span:  e.Span,
			Value: l.liftExpr(e.Value),
		}
	case *ast.FieldAccessExpr:
		return &ast.FieldAccessExpr{
			Span:  e.Span,
			Expr:  l.liftExpr(e.Expr),
			Field: e.Field,
		}
	case *ast.ErrorPropagationExpr:
		return &ast.ErrorPropagationExpr{
			Span: e.Span,
			Expr: l.liftExpr(e.Expr),
		}
	case *ast.RecordLit:
		newFields := make([]*ast.FieldInit, len(e.Fields))
		for i, f := range e.Fields {
			newFields[i] = &ast.FieldInit{
				Span:  f.Span,
				Name:  f.Name,
				Value: l.liftExpr(f.Value),
			}
		}
		return &ast.RecordLit{Span: e.Span, Name: e.Name, Fields: newFields}
	case *ast.FnLit:
		return &ast.FnLit{
			Span:       e.Span,
			Params:     e.Params,
			ReturnType: e.ReturnType,
			Body:       l.liftExprs(e.Body),
		}
	default:
		// Atoms (Ident, literals, etc.) pass through unchanged.
		return expr
	}
}

// liftCallExpr checks whether a call targets a Go function returning (T, error)
// or error, and wraps it in a GoLiftCallExpr if so.
func (l *lifter) liftCallExpr(ce *ast.CallExpr) ast.Expr {
	// Also lift the arguments recursively.
	newArgs := make([]ast.Expr, len(ce.Args))
	for i, a := range ce.Args {
		newArgs[i] = l.liftExpr(a)
	}
	lifted := &ast.CallExpr{Span: ce.Span, Func: l.liftExpr(ce.Func), Args: newArgs}

	fa, ok := ce.Func.(*ast.FieldAccessExpr)
	if !ok {
		return lifted
	}
	ident, ok := fa.Expr.(*ast.Ident)
	if !ok {
		return lifted
	}
	ref := l.res.Lookup(ident.Span)
	if ref == nil || ref.Kind != resolver.DeclImport {
		return lifted
	}
	pkg := l.loader.Load(ref.Name)
	if pkg == nil {
		return lifted
	}
	sym := pkg.Symbols[fa.Field]
	if sym == nil {
		return lifted
	}
	fn, ok := sym.Obj.(*types.Func)
	if !ok {
		return lifted
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return lifted
	}

	valueType, needsLift := goErrorConvention(sig.Results())
	if !needsLift {
		return lifted
	}
	return &ast.GoLiftCallExpr{
		Span:        ce.Span,
		Call:        lifted,
		ValueGoType: valueType,
	}
}

// twoResultCount is the number of return values in the (T, error) Go convention.
const twoResultCount = 2

// goErrorConvention checks if a Go result tuple follows the (T, error) or error
// convention. Returns the Go type string for T (or "" for error-only) and true
// if lifting is needed.
func goErrorConvention(results *types.Tuple) (valueType string, ok bool) {
	switch results.Len() {
	case 1:
		if isGoErrorType(results.At(0).Type()) {
			return "", true
		}
	case twoResultCount:
		if isGoErrorType(results.At(1).Type()) {
			return types.TypeString(results.At(0).Type(), nil), true
		}
	}
	return "", false
}

// isGoErrorType reports whether t is the predeclared Go error interface.
func isGoErrorType(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	return named.Obj().Name() == "error" && named.Obj().Pkg() == nil
}
