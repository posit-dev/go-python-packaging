// SPDX-License-Identifier: Apache-2.0 OR MIT

package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PEP 440 permits "alpha", "beta", "pre", "preview" and "c" as alternate
// spellings of "a", "b", "rc" and "rc".
//
// These cases pass both before and after the pre_l alternation reorder, because
// versionRegex is ANCHORED (`^\s*...\s*$`, see version.go's init): the anchor
// forces backtracking until the whole string is consumed, so a short "a" match
// on "alpha" cannot succeed and the engine retries with the long alternative.
// They are kept as characterization tests -- they pin the alias mapping and
// document that Parse was never the broken path. The regression that motivated
// the reorder is in TestNewSpecifiers_LongPreReleaseSpellingIsOneSpecifier
// below, where the governing regex is UNANCHORED.
func TestParse_LongPreReleaseSpellings(t *testing.T) {
	tests := []struct {
		version string
		want    string
	}{
		{"1.0alpha", "1.0a0"},
		{"1.0alpha1", "1.0a1"},
		{"1.0.alpha1", "1.0a1"},
		{"1.0-alpha1", "1.0a1"},
		{"1.0ALPHA1", "1.0a1"},
		{"1.0beta", "1.0b0"},
		{"1.0beta2", "1.0b2"},
		{"1.0-beta2", "1.0b2"},
		{"1.0BETA2", "1.0b2"},
		{"1.0pre", "1.0rc0"},
		{"1.0pre1", "1.0rc1"},
		{"1.0preview", "1.0rc0"},
		{"1.0preview1", "1.0rc1"},
		{"1.0PREVIEW1", "1.0rc1"},
		{"1.0c1", "1.0rc1"},
		{"1.0rc1", "1.0rc1"},
		{"1.0a1", "1.0a1"},
		{"1.0b2", "1.0b2"},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			v, err := Parse(tt.version)
			require.NoError(t, err)
			assert.Equal(t, tt.want, v.String())
		})
	}
}

// A specifier string must never yield more specifiers than it has operators.
//
// specifierRegexp is UNANCHORED (see specifier.go), so nothing forces the pre_l
// alternation to consume the whole token: with the shortest-first ordering,
// Go's leftmost-first matching accepted "a" for "alpha", leaving "lpha1", and
// FindAllString then harvested the trailing "1" as a SECOND, operator-less
// specifier. Both were ANDed together, so "==1.0alpha1" parsed without error
// and then matched nothing at all -- a silent wrong answer:
//
//	"==1.0alpha1"   -> "==1.0a,1"
//	"==1.0-beta2"   -> "==1.0-b,2"
//	"==1.0preview1" -> "==1.0pre,1"
//
// pypa/packaging orders its equivalent alternation longest-before-prefix for
// this same reason (Python's re is also leftmost-first):
// https://github.com/pypa/packaging/blob/4eb0753dba8fcaaac8eb75463374e448f0931558/src/packaging/version.py#L199
func TestNewSpecifiers_LongPreReleaseSpellingIsOneSpecifier(t *testing.T) {
	tests := []struct {
		specifier string
		matches   string
	}{
		{"==1.0alpha1", "1.0a1"},
		{"==1.0-beta2", "1.0b2"},
		{"==1.0preview1", "1.0rc1"},
		{"==1.0pre1", "1.0rc1"},
		{"==1.0c1", "1.0rc1"},
		// post_l is `post|rev|r`. "r" IS a prefix of "rev", but it is listed
		// last, so leftmost-first already picks the long form. Verified against
		// the unfixed baseline; these are guard cases against a future reorder.
		{"==1.0rev1", "1.0.post1"},
		{"==1.0post1", "1.0.post1"},
		{"==1.0r1", "1.0.post1"},
	}
	for _, tt := range tests {
		t.Run(tt.specifier, func(t *testing.T) {
			specs, err := NewSpecifiers(tt.specifier)
			require.NoError(t, err)

			count := 0
			for _, group := range specs.specifiers {
				count += len(group)
			}
			assert.Equal(t, 1, count, "expected exactly one specifier, got %q", specs.String())

			v, err := Parse(tt.matches)
			require.NoError(t, err)
			assert.True(t, specs.Check(v), "%q should match %q", tt.specifier, tt.matches)
		})
	}
}
