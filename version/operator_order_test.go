// SPDX-License-Identifier: Apache-2.0 OR MIT

package version

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The operator alternation must be built in a deterministic, longest-first
// order. Go's regexp is leftmost-FIRST, not leftmost-longest, so an
// alternative that is a prefix of a later one wins: "=" would swallow the
// first character of "==" and "===". The empty operator matches anything and
// must come last.
//
// This is pinned rather than merely sorted because Task 2 relaxes the "==="
// operand pattern, removing the trailing PEP-440-shaped group that currently
// forces backtracking to the correct operator. See rstudio/package-manager#19369
// for the same bug in the pre-release alternation.
func TestOperatorAlternationIsDeterministic(t *testing.T) {
	// The empty operator is the last alternative and contributes an empty
	// string, so the joined form ends with a trailing "|". That is correct and
	// intentional: an empty trailing alternative is how the operator-less
	// specifier form is expressed in the regexp.
	want := "===|==|!=|<=|>=|~=|<|>|=|"
	got := strings.Join(orderedOperatorPatterns(), "|")
	assert.Equal(t, want, got, "operator alternation order changed")
}

func TestOperatorAlternationLongestFirst(t *testing.T) {
	ops := orderedOperatorPatterns()
	for i, a := range ops {
		for j := i + 1; j < len(ops); j++ {
			b := ops[j]
			assert.False(t, strings.HasPrefix(b, a),
				"%q at index %d is a prefix of %q at index %d; longer alternatives must come first",
				a, i, b, j)
		}
	}
}

// The empty operator matches anything, so it must be last.
func TestEmptyOperatorIsLast(t *testing.T) {
	ops := orderedOperatorPatterns()
	require.NotEmpty(t, ops)
	// The empty operator is represented by the empty string; QuoteMeta("") is "".
	for i, o := range ops {
		if o == "" {
			assert.Equal(t, len(ops)-1, i, "the empty operator must be the last alternative")
		}
	}
}

// A specifier string must never yield more specifiers than it has operators.
// This is the invariant that generalizes rstudio/package-manager#19369: there,
// a truncated pre-release token left a trailing fragment that was harvested as
// a second, operator-less specifier, so "==1.0alpha1" parsed cleanly and then
// matched nothing.
func TestSpecifierCountMatchesOperatorCount(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"==1.0", 1},
		{"==1.0alpha1", 1},
		{"==1.0-beta2", 1},
		{"==1.0preview1", 1},
		{"==1.0rev1", 1},
		{">1.0,<2.0", 2},
		{">=1.0,!=1.5,<2.0", 3},
		{"~=1.0", 1},
		{"===1.0", 1},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			specs, err := NewSpecifiers(tt.in)
			require.NoError(t, err, "input %q", tt.in)
			count := 0
			for _, group := range specs.specifiers {
				count += len(group)
			}
			assert.Equal(t, tt.want, count,
				"input %q parsed to %d specifiers (String()=%q)", tt.in, count, specs.String())
		})
	}
}
