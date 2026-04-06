package goloader_test

import (
	"go/types"
	"testing"

	"github.com/dylanblakemore/golem/internal/goloader"
)

func TestLoadFmt(t *testing.T) {
	l := goloader.New()
	pkg := l.Load("fmt")
	if pkg == nil {
		t.Fatal("failed to load fmt package")
	}
	if pkg.Name != "fmt" {
		t.Errorf("expected pkg.Name=fmt, got %s", pkg.Name)
	}
	if pkg.Path != "fmt" {
		t.Errorf("expected pkg.Path=fmt, got %s", pkg.Path)
	}
}

func TestLoadFmtHasExpectedSymbols(t *testing.T) {
	l := goloader.New()
	pkg := l.Load("fmt")
	if pkg == nil {
		t.Fatal("failed to load fmt package")
	}

	// fmt.Println should be accessible as "println" in Golem.
	sym, ok := pkg.Symbols["println"]
	if !ok {
		t.Fatal("expected 'println' symbol in fmt package")
	}
	if sym.GoName != "Println" {
		t.Errorf("expected GoName=Println, got %s", sym.GoName)
	}

	// fmt.Sprintf -> "sprintf"
	sym, ok = pkg.Symbols["sprintf"]
	if !ok {
		t.Fatal("expected 'sprintf' symbol in fmt package")
	}
	if sym.GoName != "Sprintf" {
		t.Errorf("expected GoName=Sprintf, got %s", sym.GoName)
	}
}

func TestLoadOs(t *testing.T) {
	l := goloader.New()
	pkg := l.Load("os")
	if pkg == nil {
		t.Fatal("failed to load os package")
	}
	if pkg.Name != "os" {
		t.Errorf("expected pkg.Name=os, got %s", pkg.Name)
	}

	// os.ReadFile -> "readFile"
	sym, ok := pkg.Symbols["readFile"]
	if !ok {
		t.Fatal("expected 'readFile' symbol in os package")
	}
	if sym.GoName != "ReadFile" {
		t.Errorf("expected GoName=ReadFile, got %s", sym.GoName)
	}
}

func TestLoadNetHttp(t *testing.T) {
	l := goloader.New()
	pkg := l.Load("net/http")
	if pkg == nil {
		t.Fatal("failed to load net/http package")
	}
	if pkg.Name != "http" {
		t.Errorf("expected pkg.Name=http, got %s", pkg.Name)
	}

	// http.ListenAndServe -> "listenAndServe"
	sym, ok := pkg.Symbols["listenAndServe"]
	if !ok {
		t.Fatal("expected 'listenAndServe' symbol in net/http package")
	}
	if sym.GoName != "ListenAndServe" {
		t.Errorf("expected GoName=ListenAndServe, got %s", sym.GoName)
	}
}

func TestLoadUnknownPackageReturnsNil(t *testing.T) {
	l := goloader.New()
	pkg := l.Load("does/not/exist/xyz123golem")
	if pkg != nil {
		t.Error("expected nil for unknown package")
	}
}

func TestCachingReturnsSamePointer(t *testing.T) {
	l := goloader.New()
	pkg1 := l.Load("fmt")
	pkg2 := l.Load("fmt")
	if pkg1 != pkg2 {
		t.Error("expected same package pointer on repeated load (caching)")
	}
}

func TestNegativeCaching(t *testing.T) {
	l := goloader.New()
	pkg1 := l.Load("nonexistent/pkg/xyz")
	pkg2 := l.Load("nonexistent/pkg/xyz")
	// Both nil, and second call should not panic or retry
	if pkg1 != nil || pkg2 != nil {
		t.Error("expected nil for nonexistent package")
	}
}

func TestSymbolIsFunction(t *testing.T) {
	l := goloader.New()
	pkg := l.Load("fmt")
	if pkg == nil {
		t.Fatal("failed to load fmt package")
	}

	sym, ok := pkg.Symbols["println"]
	if !ok {
		t.Fatal("expected println symbol")
	}

	fn, ok := sym.Obj.(*types.Func)
	if !ok {
		t.Fatalf("expected *types.Func for Println, got %T", sym.Obj)
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		t.Fatalf("expected *types.Signature, got %T", fn.Type())
	}
	// fmt.Println is variadic: func(a ...any) (n int, err error)
	if !sig.Variadic() {
		t.Error("expected fmt.Println to be variadic")
	}
	// Two return values: (n int, err error)
	if sig.Results().Len() != 2 {
		t.Errorf("expected 2 results, got %d", sig.Results().Len())
	}
}

func TestStructTypeSymbol(t *testing.T) {
	l := goloader.New()
	pkg := l.Load("net/http")
	if pkg == nil {
		t.Fatal("failed to load net/http package")
	}

	// http.Request should be accessible as "request"
	sym, ok := pkg.Symbols["request"]
	if !ok {
		t.Fatal("expected 'request' symbol in net/http package")
	}
	if sym.GoName != "Request" {
		t.Errorf("expected GoName=Request, got %s", sym.GoName)
	}
}

func TestLowercaseFirst(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Println", "println"},
		{"ListenAndServe", "listenAndServe"},
		{"ReadFile", "readFile"},
		{"Get", "get"},
		{"A", "a"},
		{"", ""},
		{"already", "already"},
	}
	for _, tt := range tests {
		got := goloader.LowercaseFirst(tt.input)
		if got != tt.expected {
			t.Errorf("LowercaseFirst(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
