// SPDX-License-Identifier: Apache-2.0 OR MIT

package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An empty set of specifiers admits every version, per PEP 508: a requirement's
// version specifier is optional, and one without it accepts any version. The
// canonical spelling of that is an empty specifier set.
//
// Reference behavior measured against pypa/packaging 26.2:
//
//	Requirement('flask').specifier   -> <SpecifierSet('')>   len 0
//	  contains '3.0.0'    -> True
//	  contains '0.1'      -> True
//	  contains '99!1.0'   -> True
//	  contains '2.0.0rc1' -> True
//
// This package previously answered false to all of those, which inverts every
// gate built on a specifier set: a caller filtering candidates against an
// unconstrained set discarded all of them, and would conclude the package was
// unsatisfiable rather than unconstrained. That is exactly the defect it caused
// downstream in go-pyresolver, where an absent Requires-Python was read as
// "compatible with no interpreter at all".

func TestEmptySpecifiersAdmitEveryVersion(t *testing.T) {
	var unconstrained Specifiers

	// Deliberately spans the shapes most likely to be special-cased somewhere:
	// a plain release, a tiny one, an epoch, a pre-release, a post-release, a dev
	// release, and a local version.
	//
	// The last three are the ones that matter most, and they are not padding.
	// They sort BELOW 0.0.0, so they are what distinguishes "admit everything"
	// from the tempting shortcut of treating the empty set as ">=0.0.0" -- see
	// TestEmptySpecifiersAreNotAFloorOfZero.
	for _, v := range []string{
		"3.0.0", "0.1", "99!1.0", "2.0.0rc1", "1.0.post1", "1.0.dev0", "1.0+ubuntu.1",
		"0.0.0", "0.0.0.dev0", "0.0.0a1",
	} {
		assert.True(t, unconstrained.Check(MustParse(v)),
			"an empty specifier set must admit %s: no constraint means no constraint", v)
	}
}

// TestEmptySpecifiersAgreeWithAndCheck pins the internal consistency that made
// the old behavior indefensible rather than merely debatable.
//
// andCheck returns true for an empty AND-group, because a conjunction over no
// constraints is vacuously true. Zero OR-groups is the same statement one level
// up. The two answered differently, from the same logical shape.
func TestEmptySpecifiersAgreeWithAndCheck(t *testing.T) {
	v := MustParse("1.0")

	assert.True(t, andCheck(v, nil),
		"precondition: an empty AND-group is vacuously true")

	var unconstrained Specifiers
	assert.Equal(t, andCheck(v, nil), unconstrained.Check(v),
		"zero OR-groups and an empty AND-group are the same claim and must agree")
}

// TestEmptySpecifiersStillRenderEmpty guards the other half of the reference's
// behavior: an empty upstream SpecifierSet renders as the empty string, so
// admitting everything must not be implemented by materializing a group into
// ss.specifiers.
func TestEmptySpecifiersStillRenderEmpty(t *testing.T) {
	var unconstrained Specifiers
	assert.Equal(t, "", unconstrained.String(),
		"an empty specifier set renders as the empty string")
}

// TestEmptySpecifiersAreNotAFloorOfZero rules out the plausible-looking shortcut
// of implementing "no constraint" as ">=0.0.0".
//
// It is not equivalent, and the reason is easy to miss: a dev or pre-release
// segment sorts BELOW the release it belongs to, so 0.0.0.dev0 and 0.0.0a1 are
// less than 0.0.0 and a >=0.0.0 floor excludes them. Measured against
// pypa/packaging 26.2, which admits both under an empty SpecifierSet and rejects
// both under SpecifierSet(">=0.0.0").
//
// Note this is NOT about pre-release exclusion. A plain ">=1.0" DOES admit
// 2.0.0rc1 in the reference -- PEP 440's default pre-release exclusion is applied
// by filter(), not by contains(). Conflating the two would be a different bug.
func TestEmptySpecifiersAreNotAFloorOfZero(t *testing.T) {
	floor, err := NewSpecifiers(">=0.0.0")
	require.NoError(t, err)

	var unconstrained Specifiers

	for _, v := range []string{"0.0.0.dev0", "0.0.0a1"} {
		ver := MustParse(v)
		assert.True(t, unconstrained.Check(ver),
			"an empty specifier set must admit %s", v)
		assert.False(t, floor.Check(ver),
			"precondition: %s sorts below 0.0.0, so a >=0.0.0 floor excludes it -- which "+
				"is exactly why the empty set cannot be implemented as that floor", v)
	}
}

// TestNonEmptySpecifiersStillExclude is the direction that keeps the change from
// being a blanket "always true". Without it, "make the empty case lenient" could
// be satisfied by an accessor that never rejects anything, which would be a far
// worse defect than the one being fixed.
func TestNonEmptySpecifiersStillExclude(t *testing.T) {
	for _, tc := range []struct {
		spec, admitted, excluded string
	}{
		{">=3.9", "3.11", "3.8"},
		{"==1.0", "1.0", "1.1"},
		{"!=1.0", "1.1", "1.0"},
		{"~=1.4.5", "1.4.6", "1.5.0"},
		{"<2.0,>=1.0", "1.5", "2.0"},
	} {
		ss, err := NewSpecifiers(tc.spec)
		require.NoError(t, err, "spec %q should parse", tc.spec)

		assert.True(t, ss.Check(MustParse(tc.admitted)),
			"%q should admit %s", tc.spec, tc.admitted)
		assert.False(t, ss.Check(MustParse(tc.excluded)),
			"%q must still EXCLUDE %s -- the empty-set change must not make Check "+
				"unconditionally true", tc.spec, tc.excluded)
	}
}

// TestZeroGroupsIsUnreachableByParsing records the fact that bounds the risk of
// this change, so a future reader can see it was measured rather than assumed.
//
// Flipping the empty case from reject-all to admit-all is a fail-open change, and
// would deserve real suspicion if a hostile or malformed input string could reach
// it. None can: every degenerate input either errors or yields at least one group,
// so the only route to a zero-group Specifiers is Go's zero value -- a caller who
// declared one and never assigned it, which today gets the wrong answer anyway.
//
// If a future change makes some input parse to zero groups, this test fails and
// that risk assessment has to be redone.
func TestZeroGroupsIsUnreachableByParsing(t *testing.T) {
	identity := func(s string) string { return s }

	for _, in := range []string{"", " ", "\t", ",", ",,", "()", "( )", "|", "||", "-", "*"} {
		if ss, err := NewSpecifiers(in); err == nil {
			assert.NotZero(t, len(ss.specifiers),
				"NewSpecifiers(%q) parsed to ZERO groups; the fail-open risk assessment "+
					"for the empty case assumed this was impossible", in)
		}
		if ss, err := NewRSpecifiers(in, identity); err == nil {
			assert.NotZero(t, len(ss.specifiers),
				"NewRSpecifiers(%q) parsed to ZERO groups; the fail-open risk assessment "+
					"for the empty case assumed this was impossible", in)
		}
	}
}
