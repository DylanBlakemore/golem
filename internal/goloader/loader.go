// Package goloader loads Go package type information for use in Golem type checking.
//
// It uses golang.org/x/tools/go/packages to load compiled Go package
// type signatures and exposes them in a form suitable for Golem's type checker.
// Package metadata is cached per import path so repeated lookups are cheap.
package goloader

import (
	"go/token"
	"go/types"
	"unicode"

	"golang.org/x/tools/go/packages"
)

// Symbol represents an exported symbol from a Go package.
type Symbol struct {
	GoName string       // original Go name (e.g. "Println")
	Obj    types.Object // the underlying Go type object
}

// Package holds the loaded type information for a Go package.
type Package struct {
	Name    string             // package name (e.g. "fmt")
	Path    string             // import path (e.g. "fmt" or "net/http")
	Symbols map[string]*Symbol // lowercased Golem name -> symbol
}

// Loader loads and caches Go packages by import path.
type Loader struct {
	cache map[string]*Package // nil entry means load was attempted but failed
	fset  *token.FileSet
}

// New creates a new Loader.
func New() *Loader {
	return &Loader{
		cache: make(map[string]*Package),
		fset:  token.NewFileSet(),
	}
}

// Load loads the Go package at the given import path and returns its symbols.
// Returns nil if the package cannot be loaded (unknown or not compiled).
// Results are cached: repeated calls for the same path are cheap.
func (l *Loader) Load(importPath string) *Package {
	if pkg, ok := l.cache[importPath]; ok {
		return pkg // may be nil if previously failed
	}

	cfg := &packages.Config{
		Mode: packages.NeedTypes | packages.NeedName,
		Fset: l.fset,
	}
	pkgs, err := packages.Load(cfg, importPath)
	if err != nil || len(pkgs) == 0 {
		l.cache[importPath] = nil
		return nil
	}

	loaded := pkgs[0]
	if loaded.Types == nil || len(loaded.Errors) > 0 {
		l.cache[importPath] = nil
		return nil
	}

	pkg := l.buildPackage(importPath, loaded)
	l.cache[importPath] = pkg
	return pkg
}

func (l *Loader) buildPackage(path string, loaded *packages.Package) *Package {
	pkg := &Package{
		Name:    loaded.Types.Name(),
		Path:    path,
		Symbols: make(map[string]*Symbol),
	}

	scope := loaded.Types.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if !obj.Exported() {
			continue
		}
		golemName := LowercaseFirst(name)
		pkg.Symbols[golemName] = &Symbol{
			GoName: name,
			Obj:    obj,
		}
	}

	return pkg
}

// LowercaseFirst returns s with the first Unicode letter lowercased.
// This is used to convert Go exported names (e.g. "Println") to
// Golem access names (e.g. "println").
func LowercaseFirst(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}
