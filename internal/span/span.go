// Package span provides source location tracking for the Golem compiler.
package span

import "fmt"

// Position represents a specific point in source code.
type Position struct {
	Offset int // byte offset from start of file
	Line   int // 1-based line number
	Column int // 1-based column number (bytes, not runes)
}

// Span represents a range in source code.
type Span struct {
	File  string
	Start Position
	End   Position
}

// String returns a human-readable representation of the span.
func (s Span) String() string {
	if s.File != "" {
		return fmt.Sprintf("%s:%d:%d", s.File, s.Start.Line, s.Start.Column)
	}
	return fmt.Sprintf("%d:%d", s.Start.Line, s.Start.Column)
}
