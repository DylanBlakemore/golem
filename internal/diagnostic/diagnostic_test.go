package diagnostic

import (
	"strings"
	"testing"

	"github.com/dylanblakemore/golem/internal/span"
)

func TestFormatDiagnosticWithSource(t *testing.T) {
	source := "let x = 42\nlet y = x + true\nlet z = 1"
	d := Diagnostic{
		Span: span.Span{
			File:  "test.golem",
			Start: span.Position{Line: 2, Column: 13},
		},
		Message: "type mismatch: expected Int, got Bool",
		Phase:   "type",
	}

	out := FormatDiagnostic(d, source)

	if !strings.Contains(out, "test.golem:2:13") {
		t.Errorf("expected location header, got:\n%s", out)
	}
	if !strings.Contains(out, "type error:") {
		t.Errorf("expected phase in output, got:\n%s", out)
	}
	if !strings.Contains(out, "let y = x + true") {
		t.Errorf("expected source line, got:\n%s", out)
	}
	if !strings.Contains(out, "            ^") {
		t.Errorf("expected caret at column 13, got:\n%s", out)
	}
}

func TestFormatDiagnosticWithoutSource(t *testing.T) {
	d := Diagnostic{
		Span: span.Span{
			File:  "test.golem",
			Start: span.Position{Line: 5, Column: 1},
		},
		Message: "unexpected token",
		Phase:   "parse",
	}

	out := FormatDiagnostic(d, "")

	if !strings.Contains(out, "test.golem:5:1: parse error: unexpected token") {
		t.Errorf("unexpected output:\n%s", out)
	}
	// Should not have source line or caret.
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line without source, got %d:\n%s", len(lines), out)
	}
}

func TestFormatDiagnosticNoPhase(t *testing.T) {
	d := Diagnostic{
		Span: span.Span{
			File:  "test.golem",
			Start: span.Position{Line: 1, Column: 1},
		},
		Message: "something went wrong",
	}

	out := FormatDiagnostic(d, "some code here")

	if !strings.Contains(out, "error: something went wrong") {
		t.Errorf("expected generic error format, got:\n%s", out)
	}
	if strings.Contains(out, " error:") && strings.Contains(out, "  error:") {
		t.Errorf("should not have double space before error")
	}
}

func TestFormatDiagnostics(t *testing.T) {
	source := "let x = 1\nlet y = 2"
	diags := []Diagnostic{
		{
			Span:    span.Span{File: "test.golem", Start: span.Position{Line: 1, Column: 5}},
			Message: "first error",
			Phase:   "parse",
		},
		{
			Span:    span.Span{File: "test.golem", Start: span.Position{Line: 2, Column: 5}},
			Message: "second error",
			Phase:   "parse",
		},
	}

	out := FormatDiagnostics(diags, source)

	if !strings.Contains(out, "first error") {
		t.Error("missing first error")
	}
	if !strings.Contains(out, "second error") {
		t.Error("missing second error")
	}
}

func TestFormatDiagnosticsEmpty(t *testing.T) {
	out := FormatDiagnostics(nil, "source")
	if out != "" {
		t.Errorf("expected empty string for no diagnostics, got: %q", out)
	}
}
