// SPDX-License-Identifier: Apache-2.0 OR MIT

package marker

import "github.com/posit-dev/go-python-packaging/internal/pep508"

// Marker represents a parsed PEP 508 environment marker expression - the
// part of a requirement string following ";", e.g.
// `extra == "foo" and python_version >= "3.8"`.
//
// The zero value represents the absence of a marker clause (e.g. a
// requirement with no ";" section at all) and always evaluates as true, so
// callers never need to nil-check or special-case "no marker" before using
// a Marker.
type Marker struct {
	// ast is the parsed expression tree, or nil for the zero-value
	// "always true" marker. Left unexported: only this package and the
	// evaluator that will live alongside it (a later task) need to walk
	// it directly.
	ast pep508.Expr
}

// Parse parses s - the marker text with no leading ";" - as a complete PEP
// 508 marker expression.
//
// An empty or whitespace-only s is a syntax error, matching packaging's
// behavior for Marker(""): a marker expression always requires at least one
// comparison. To represent "no marker clause", use the zero-value Marker
// rather than calling Parse.
func Parse(s string) (Marker, error) {
	t := pep508.NewTokenizer(s)
	ast, err := pep508.ParseFullMarker(t)
	if err != nil {
		return Marker{}, err
	}
	return Marker{ast: ast}, nil
}

// FromExpr wraps an already-parsed internal marker expression into a
// Marker. It is the seam the requirement/ parser uses to build the Marker
// for a requirement's "; marker" clause without re-lexing: requirement/
// drives the shared internal/pep508 tokenizer end-to-end itself (parsing
// the marker clause mid-stream via pep508.ParseMarker), so it already has a
// pep508.Expr in hand and has no need - and, since Marker.ast is
// unexported, no ability - to go through Parse's string-based entry point.
//
// A nil expr yields the zero-value "always true" Marker, matching what
// Parse returns for the absence of a "; marker" clause.
func FromExpr(e pep508.Expr) Marker {
	return Marker{ast: e}
}

// IsEmpty reports whether m is the zero-value "always true" marker - i.e.
// there is no marker clause to evaluate at all, as opposed to a parsed
// marker expression that merely evaluates to true in some environment.
func (m Marker) IsEmpty() bool {
	return m.ast == nil
}

// String renders m in canonical PEP 508 marker syntax; Parse(m.String())
// yields an equivalent marker. The zero-value marker renders as "".
func (m Marker) String() string {
	if m.ast == nil {
		return ""
	}
	return m.ast.String()
}
