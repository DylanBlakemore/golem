// Package diagnostic provides error reporting for the Golem compiler.
package diagnostic

import (
	"fmt"
	"strings"

	"github.com/dylanblakemore/golem/internal/span"
)

// Diagnostic represents a compiler error or warning with source location information.
type Diagnostic struct {
	Span     span.Span
	Message  string
	Phase    string // e.g. "parse", "resolve", "type"
	Severity string // "error" (default) or "warning"
}

// FormatDiagnostic formats a single diagnostic with source context.
// source is the full source text of the file. If empty, only the location and message are shown.
func FormatDiagnostic(d Diagnostic, source string) string {
	var b strings.Builder

	loc := d.Span.String()
	sev := d.Severity
	if sev == "" {
		sev = "error"
	}
	if d.Phase != "" {
		fmt.Fprintf(&b, "%s: %s %s: %s\n", loc, d.Phase, sev, d.Message)
	} else {
		fmt.Fprintf(&b, "%s: %s: %s\n", loc, sev, d.Message)
	}

	if source == "" || d.Span.Start.Line <= 0 {
		return b.String()
	}

	// Extract the source line.
	lines := strings.Split(source, "\n")
	lineIdx := d.Span.Start.Line - 1
	if lineIdx >= len(lines) {
		return b.String()
	}

	srcLine := lines[lineIdx]
	fmt.Fprintf(&b, "  %s\n", srcLine)

	// Caret pointing to the error column.
	col := max(d.Span.Start.Column, 1)
	fmt.Fprintf(&b, "  %s^\n", strings.Repeat(" ", col-1))

	return b.String()
}

// FormatDiagnostics formats multiple diagnostics, separated by blank lines.
func FormatDiagnostics(diags []Diagnostic, source string) string {
	if len(diags) == 0 {
		return ""
	}
	parts := make([]string, len(diags))
	for i, d := range diags {
		parts[i] = FormatDiagnostic(d, source)
	}
	return strings.Join(parts, "\n")
}
