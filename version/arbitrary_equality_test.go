// SPDX-License-Identifier: Apache-2.0 OR MIT

package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PEP 440 arbitrary equality ("===") compares against an opaque single token,
// which need not be a valid version.
//
// Ported from pypa/packaging tests/test_specifiers.py, the "===" entries of
// TestSpecifier.test_specifiers_valid and test_arbitrary_equality.
// Pinned: 4eb0753dba8fcaaac8eb75463374e448f0931558.
func TestNewSpecifiers_ArbitraryEqualityAcceptsSingleToken(t *testing.T) {
	valid := []string{
		"===lolwat",
		"=== lolwat",
		"===1.0",
		"===v1.0",
		"===1.0+abc",
		"===arbitrarystring",
	}
	for _, s := range valid {
		t.Run(s, func(t *testing.T) {
			specs, err := NewSpecifiers(s)
			require.NoError(t, err, "input %q should parse", s)
			count := 0
			for _, g := range specs.specifiers {
				count += len(g)
			}
			assert.Equal(t, 1, count, "input %q yielded %d specifiers (String()=%q)",
				s, count, specs.String())
		})
	}
}

// Arbitrary equality takes ONE token. An operand containing internal
// whitespace is rejected by pypa/packaging 26.2 as well.
func TestNewSpecifiers_ArbitraryEqualityRejectsMultipleTokens(t *testing.T) {
	invalid := []string{
		"===any string here",
		"===lolwat and stuff",
	}
	for _, s := range invalid {
		t.Run(s, func(t *testing.T) {
			_, err := NewSpecifiers(s)
			assert.Error(t, err, "input %q should be rejected", s)
		})
	}
}
