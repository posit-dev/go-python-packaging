// SPDX-License-Identifier: Apache-2.0 OR MIT

package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A ".*" prefix match may not be combined with a dev, local, OR post release.
// The post-release half was missing.
//
// Ported from pypa/packaging tests/test_requirements.py,
// TestRequirementParsing.test_error_when_prefix_match_uses_post_release
// (parametrized over "==" and "!=") and its _without_spaces sibling, plus the
// pre/post entries of tests/test_specifiers.py's test_specifiers_invalid.
// Pinned: 4eb0753dba8fcaaac8eb75463374e448f0931558.
func TestNewSpecifiers_RejectsWildcardOnPostRelease(t *testing.T) {
	invalid := []string{
		"==2.0.post1.*",
		"!=2.0.post1.*",
		"==1.2.3.post4.*",
		"!=1.2.3.post4.*",
	}
	for _, s := range invalid {
		t.Run(s, func(t *testing.T) {
			_, err := NewSpecifiers(s)
			assert.Error(t, err, "input %q should be rejected", s)
		})
	}
}

// Regression guard: the dev and local halves of the same rule must keep working,
// and legitimate wildcards must keep parsing.
func TestNewSpecifiers_WildcardRuleUnchangedCases(t *testing.T) {
	invalid := []string{"==1.0.dev1.*", "!=1.0.dev1.*", "==1.0.*+5", "!=1.0.*+deadbeef"}
	for _, s := range invalid {
		t.Run("invalid_"+s, func(t *testing.T) {
			_, err := NewSpecifiers(s)
			assert.Error(t, err, "input %q should be rejected", s)
		})
	}
	valid := []string{"==1.0.*", "!=2.2.*", "==2.1.*"}
	for _, s := range valid {
		t.Run("valid_"+s, func(t *testing.T) {
			_, err := NewSpecifiers(s)
			assert.NoError(t, err, "input %q should parse", s)
		})
	}
}
