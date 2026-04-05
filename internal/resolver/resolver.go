// Package resolver implements name resolution for the Golem compiler.
//
// It walks the untyped AST and binds every identifier to its declaration site.
// The output is a Resolution containing declaration references for each identifier.
package resolver

import (
	"fmt"

	"github.com/dylanblakemore/golem/internal/ast"
	"github.com/dylanblakemore/golem/internal/span"
)

// DeclKind classifies what a name refers to.
type DeclKind int

const (
	DeclLocal     DeclKind = iota // let binding or function parameter
	DeclFunction                  // top-level function
	DeclType                      // type declaration
	DeclVariant                   // sum type variant constructor
	DeclImport                    // import (Go package)
	DeclImportRef                 // qualified reference to an import member
)

// DeclRef points to the declaration that a name refers to.
type DeclRef struct {
	Kind DeclKind
	Name string
	Span span.Span
}

// Resolution stores the name resolution results for a module.
type Resolution struct {
	// Refs maps AST node spans (as string keys) to their resolved declarations.
	Refs map[string]*DeclRef
}

// Lookup returns the DeclRef for a given span, or nil if unresolved.
func (r *Resolution) Lookup(s span.Span) *DeclRef {
	return r.Refs[spanKey(s)]
}

// Error represents a name resolution error.
type Error struct {
	Span    span.Span
	Message string
}

func (e Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Span, e.Message)
}

// scope represents a lexical scope containing name bindings.
type scope struct {
	parent  *scope
	symbols map[string]*DeclRef
}

func newScope(parent *scope) *scope {
	return &scope{
		parent:  parent,
		symbols: make(map[string]*DeclRef),
	}
}

func (s *scope) define(name string, ref *DeclRef) {
	s.symbols[name] = ref
}

func (s *scope) lookup(name string) *DeclRef {
	if ref, ok := s.symbols[name]; ok {
		return ref
	}
	if s.parent != nil {
		return s.parent.lookup(name)
	}
	return nil
}

func (s *scope) lookupLocal(name string) *DeclRef {
	if ref, ok := s.symbols[name]; ok {
		return ref
	}
	return nil
}

// Resolver performs name resolution on a parsed Golem module.
type Resolver struct {
	module  *ast.Module
	current *scope
	imports map[string]*DeclRef // import alias -> DeclRef (package name from path)
	res     *Resolution
	errors  []Error
}

// Resolve performs name resolution on the given module and returns
// the resolution map and any errors encountered.
func Resolve(mod *ast.Module) (*Resolution, []Error) {
	r := &Resolver{
		module:  mod,
		imports: make(map[string]*DeclRef),
		res: &Resolution{
			Refs: make(map[string]*DeclRef),
		},
	}
	r.resolve()
	return r.res, r.errors
}

func (r *Resolver) resolve() {
	// Phase 1: Build module scope with all top-level declarations.
	// This allows forward references between functions.
	r.current = newScope(nil)

	// Register imports first
	for _, imp := range r.module.Imports {
		name := importName(imp.Path)
		ref := &DeclRef{Kind: DeclImport, Name: imp.Path, Span: imp.Span}
		if existing := r.current.lookupLocal(name); existing != nil {
			r.error(imp.Span, fmt.Sprintf("duplicate import %q", imp.Path))
			continue
		}
		r.current.define(name, ref)
		r.imports[name] = ref
	}

	// Register top-level declarations (forward references allowed)
	for _, decl := range r.module.Decls {
		switch d := decl.(type) {
		case *ast.FnDecl:
			ref := &DeclRef{Kind: DeclFunction, Name: d.Name, Span: d.Span}
			if existing := r.current.lookupLocal(d.Name); existing != nil {
				r.error(d.Span, fmt.Sprintf("duplicate declaration %q", d.Name))
				continue
			}
			r.current.define(d.Name, ref)
		case *ast.TypeDecl:
			ref := &DeclRef{Kind: DeclType, Name: d.Name, Span: d.Span}
			if existing := r.current.lookupLocal(d.Name); existing != nil {
				r.error(d.Span, fmt.Sprintf("duplicate declaration %q", d.Name))
				continue
			}
			r.current.define(d.Name, ref)
			// Register variant constructors for sum types
			if sumBody, ok := d.Body.(*ast.SumTypeBody); ok {
				for _, v := range sumBody.Variants {
					vRef := &DeclRef{Kind: DeclVariant, Name: v.Name, Span: v.Span}
					if existing := r.current.lookupLocal(v.Name); existing != nil {
						r.error(v.Span, fmt.Sprintf("variant %q conflicts with existing declaration", v.Name))
						continue
					}
					r.current.define(v.Name, vRef)
				}
			}
		case *ast.LetDecl:
			ref := &DeclRef{Kind: DeclLocal, Name: d.Name, Span: d.Span}
			if existing := r.current.lookupLocal(d.Name); existing != nil {
				r.error(d.Span, fmt.Sprintf("duplicate declaration %q", d.Name))
				continue
			}
			r.current.define(d.Name, ref)
		}
	}

	// Phase 2: Resolve bodies of all declarations.
	for _, decl := range r.module.Decls {
		r.resolveDecl(decl)
	}
}

func (r *Resolver) resolveDecl(decl ast.Decl) {
	switch d := decl.(type) {
	case *ast.FnDecl:
		r.resolveFnDecl(d)
	case *ast.TypeDecl:
		// Type bodies don't need resolution in Phase 0
	case *ast.LetDecl:
		r.resolveExpr(d.Value)
	}
}

func (r *Resolver) resolveFnDecl(fn *ast.FnDecl) {
	// Create a new scope for the function body
	r.pushScope()
	defer r.popScope()

	// Register parameters
	for _, param := range fn.Params {
		ref := &DeclRef{Kind: DeclLocal, Name: param.Name, Span: param.Span}
		if existing := r.current.lookupLocal(param.Name); existing != nil {
			r.error(param.Span, fmt.Sprintf("duplicate parameter %q", param.Name))
			continue
		}
		r.current.define(param.Name, ref)
	}

	// Resolve body expressions
	for _, expr := range fn.Body {
		r.resolveExpr(expr)
	}
}

func (r *Resolver) resolveExpr(expr ast.Expr) {
	if expr == nil {
		return
	}

	switch e := expr.(type) {
	case *ast.Ident:
		r.resolveIdent(e)
	case *ast.IntLit, *ast.FloatLit, *ast.StringLit, *ast.BoolLit, *ast.NilLit, *ast.BadExpr:
		// Literals need no resolution
	case *ast.BinaryExpr:
		r.resolveExpr(e.Left)
		r.resolveExpr(e.Right)
	case *ast.UnaryExpr:
		r.resolveExpr(e.Operand)
	case *ast.CallExpr:
		r.resolveCallExpr(e)
	case *ast.FieldAccessExpr:
		r.resolveFieldAccess(e)
	case *ast.BlockExpr:
		r.resolveBlock(e.Stmts)
	case *ast.IfExpr:
		r.resolveIfExpr(e)
	case *ast.LetExpr:
		r.resolveExpr(e.Value)
		r.current.define(e.Name, &DeclRef{Kind: DeclLocal, Name: e.Name, Span: e.Span})
	case *ast.ReturnExpr:
		if e.Value != nil {
			r.resolveExpr(e.Value)
		}
	case *ast.StringInterpolation:
		r.resolveStringInterpolation(e)
	case *ast.RecordLit:
		r.resolveRecordLit(e)
	case *ast.FnLit:
		r.resolveFnLit(e)
	}
}

func (r *Resolver) resolveCallExpr(e *ast.CallExpr) {
	r.resolveExpr(e.Func)
	for _, arg := range e.Args {
		r.resolveExpr(arg)
	}
}

func (r *Resolver) resolveFieldAccess(e *ast.FieldAccessExpr) {
	if ident, ok := e.Expr.(*ast.Ident); ok {
		if _, isImport := r.imports[ident.Name]; isImport {
			ref := &DeclRef{Kind: DeclImportRef, Name: ident.Name + "." + e.Field, Span: e.Span}
			r.record(ident.Span, r.imports[ident.Name])
			r.record(e.Span, ref)
			return
		}
	}
	r.resolveExpr(e.Expr)
}

func (r *Resolver) resolveBlock(stmts []ast.Expr) {
	r.pushScope()
	for _, stmt := range stmts {
		r.resolveExpr(stmt)
	}
	r.popScope()
}

func (r *Resolver) resolveIfExpr(e *ast.IfExpr) {
	r.resolveExpr(e.Cond)
	r.resolveBlock(e.Then)
	if e.Else != nil {
		r.resolveBlock(e.Else)
	}
}

func (r *Resolver) resolveStringInterpolation(e *ast.StringInterpolation) {
	for _, part := range e.Parts {
		if interp, ok := part.(*ast.StringInterpExpr); ok {
			r.resolveExpr(interp.Expr)
		}
	}
}

func (r *Resolver) resolveRecordLit(e *ast.RecordLit) {
	ref := r.current.lookup(e.Name)
	if ref != nil {
		r.record(e.Span, ref)
	} else {
		r.error(e.Span, fmt.Sprintf("undefined type %q", e.Name))
	}
	for _, field := range e.Fields {
		r.resolveExpr(field.Value)
	}
}

func (r *Resolver) resolveFnLit(e *ast.FnLit) {
	r.pushScope()
	for _, param := range e.Params {
		r.current.define(param.Name, &DeclRef{Kind: DeclLocal, Name: param.Name, Span: param.Span})
	}
	for _, stmt := range e.Body {
		r.resolveExpr(stmt)
	}
	r.popScope()
}

func (r *Resolver) resolveIdent(ident *ast.Ident) {
	ref := r.current.lookup(ident.Name)
	if ref == nil {
		r.error(ident.Span, fmt.Sprintf("undefined variable %q", ident.Name))
		return
	}
	r.record(ident.Span, ref)
}

func (r *Resolver) record(s span.Span, ref *DeclRef) {
	r.res.Refs[spanKey(s)] = ref
}

func (r *Resolver) pushScope() {
	r.current = newScope(r.current)
}

func (r *Resolver) popScope() {
	r.current = r.current.parent
}

func (r *Resolver) error(s span.Span, msg string) {
	r.errors = append(r.errors, Error{Span: s, Message: msg})
}

// importName extracts the package name from an import path.
// e.g., "net/http" -> "http", "fmt" -> "fmt"
func importName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}

// spanKey creates a unique string key from a Span for map lookups.
func spanKey(s span.Span) string {
	return fmt.Sprintf("%s:%d:%d-%d:%d", s.File, s.Start.Line, s.Start.Column, s.End.Line, s.End.Column)
}
