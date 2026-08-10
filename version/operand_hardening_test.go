// SPDX-License-Identifier: Apache-2.0 OR MIT

// Regression tests for the six MustParse calls the matching functions used to
// make on unvalidated operands (rstudio/package-manager#19383).
//
// ⚠️ Honest scope note. These panics were NOT reachable through the public API
// before this change, and are not now: every operand except `===` is validated
// at construction, and `===` never reaches these functions. That is why the
// tests below build a Specifier struct directly -- they can only be written
// from inside the package. What changed is that "unreachable" was a property of
// the callers, and the exported Specifier type plus Filter added new callers
// that drive these functions with whatever operand they hold. The test pins the
// contract (a bad operand yields no match) so a future caller cannot
// reintroduce a panic on remote-supplied input.

package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// operandPanicShapes are the (operator, operand) pairs that reached a MustParse
// with a non-version operand. Each entry names the function it exercises.
func operandPanicShapes() []struct {
	name string
	op   string
	fn   operatorFunc
} {
	return []struct {
		name string
		op   string
		fn   operatorFunc
	}{
		{"specifierEqual", "==", specifierEqual},
		{"specifierNotEqual", "!=", specifierNotEqual},
		{"specifierLessThan", "<", specifierLessThan},
		{"specifierGreaterThan", ">", specifierGreaterThan},
		{"specifierLessThanEqual", "<=", specifierLessThanEqual},
		{"specifierGreaterThanEqual", ">=", specifierGreaterThanEqual},
		{"specifierCompatible", "~=", specifierCompatible},
	}
}

func TestUnparseableOperandDoesNotPanic(t *testing.T) {
	prospective := MustParse("1.2.3")

	for _, shape := range operandPanicShapes() {
		for _, operand := range []string{"lolwat", "", "not a version", "1.2.3.4.5-garbage"} {
			t.Run(shape.name+"__"+operand, func(t *testing.T) {
				s := Specifier{op: shape.op, operand: operand, fn: shape.fn}
				assert.NotPanics(t, func() {
					// A comparison that cannot be made is not a match. It
					// must not be a match, either: admitting a version on the
					// strength of an uninterpretable operand is worse than
					// rejecting it.
					got := s.Check(prospective)
					if shape.op == "!=" {
						// Not-equal negates equal, which returns false, so the
						// negation is true. Documented rather than asserted
						// false, so the inversion is visible.
						assert.True(t, got)
						return
					}
					assert.False(t, got)
				})
			})
		}
	}
}

// Filter drives the matching functions directly, so it is the realistic path a
// bad operand would arrive by.
func TestFilterWithUnparseableOperandDoesNotPanic(t *testing.T) {
	for _, shape := range operandPanicShapes() {
		t.Run(shape.name, func(t *testing.T) {
			s := Specifier{op: shape.op, operand: "lolwat", fn: shape.fn}
			assert.NotPanics(t, func() {
				s.Filter([]string{"1.0", "2.0a1", "also not a version"})
			})
		})
	}
}

// The whole set path, likewise.
func TestSpecifiersFilterWithUnparseableOperandDoesNotPanic(t *testing.T) {
	ss := Specifiers{specifiers: [][]Specifier{{
		{op: ">=", operand: "lolwat", fn: specifierGreaterThanEqual},
		{op: "<", operand: "1.0", fn: specifierLessThan},
	}}}
	assert.NotPanics(t, func() {
		assert.Empty(t, ss.Filter([]string{"0.5", "2.0"}))
	})
}

// A zero-value Version must not panic when it reaches a matching function
// either: it is the other side of the same landmine, and fmt RECOVERS a
// panicking String method, so a crash here would be swallowed at exactly the
// call sites most likely to hit it.
func TestZeroValueProspectiveDoesNotPanic(t *testing.T) {
	for _, shape := range operandPanicShapes() {
		t.Run(shape.name, func(t *testing.T) {
			s := Specifier{op: shape.op, operand: "1.0", fn: shape.fn}
			assert.NotPanics(t, func() {
				s.Check(Version{})
			})
		})
	}
}
