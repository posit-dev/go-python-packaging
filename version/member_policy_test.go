// SPDX-License-Identifier: Apache-2.0 OR MIT

// Regression tests for one theme with three symptoms: a member's pre-release
// policy belongs to the MEMBER, and a container must not overwrite it.
//
//	newSpecifier stamped the SET's policy onto every member, so a member read
//	           back off a set reported the set's policy instead of its own.
//	And        re-stamped members with the combined policy, which could ERASE an
//	           explicit member policy and let autodetection reach the opposite
//	           answer -- flipping the whole set's resolved policy.
//	NewSpecifiersFrom stamped too, discarding exactly the per-member policy that
//	           constructor exists to carry through.
//
// This is the same defect shape as the flat-bag bug in or_group_structure_test.go
// -- state that belongs to the member being taken over by the container -- which
// is why the container->member audit below is part of this file.
//
// Upstream is the reference: SpecifierSet keeps its own _prereleases and never
// touches its members'. Verified against pypa/packaging 26.2 throughout.

package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FINDING 7. A member read off a set reports ITS OWN policy.
//
// Measured against packaging 26.2:
//
//	ss = SpecifierSet(">=1.0.dev1", prereleases=False)
//	ss.prereleases           -> False   (the set's own)
//	list(ss)[0].prereleases  -> True    (the member's autodetect off ".dev1")
//
// This matters concretely, not just theoretically: List() is documented as
// upstream's `iter(SpecifierSet)` and the ported conformance tables iterate it.
func TestMemberReportsItsOwnPolicyNotTheSets(t *testing.T) {
	// wantSet is the set's RESOLVED policy, which is not simply the policy it was
	// given: with Auto the set resolves through its members, so a member that
	// autodetects Include makes the set Include. Measured against packaging 26.2:
	//
	//	SpecifierSet('>=1.0.dev1', prereleases=None).prereleases  -> True
	//	SpecifierSet('>=1.0.dev1', prereleases=True).prereleases  -> True
	//	SpecifierSet('>=1.0.dev1', prereleases=False).prereleases -> False
	//
	// The member is True in all three.
	cases := []struct {
		setPolicy PreReleases
		wantSet   PreReleases
	}{
		{PreReleasesAuto, PreReleasesInclude},
		{PreReleasesInclude, PreReleasesInclude},
		{PreReleasesExclude, PreReleasesExclude},
	}

	for _, tc := range cases {
		setPolicy := tc.setPolicy
		t.Run("set="+setPolicy.String(), func(t *testing.T) {
			ss, err := NewSpecifiers(">=1.0.dev1", WithPreReleases(setPolicy))
			require.NoError(t, err)

			assert.Equal(t, tc.wantSet, ss.PreReleases(), "the set's resolved policy")

			// The member autodetects from its operand, regardless.
			require.Len(t, ss.List(), 1)
			assert.Equal(t, PreReleasesInclude, ss.List()[0].PreReleases(),
				"the member autodetects Include off .dev1, whatever the set says")

			// And it agrees with the same specifier built standalone, which is
			// the invariant that makes List() usable at all.
			standalone, err := NewSpecifier(">=1.0.dev1")
			require.NoError(t, err)
			assert.Equal(t, standalone.PreReleases(), ss.List()[0].PreReleases(),
				"a member off a set must match the same specifier built alone")
		})
	}
}

// The same for a member with no pre-release in its operand, so the test cannot
// pass merely because Include happens to be the answer everywhere.
func TestMemberAutodetectIsNotOverriddenByAnExplicitSetPolicy(t *testing.T) {
	ss, err := NewSpecifiers(">=1.0", WithPreReleases(PreReleasesInclude))
	require.NoError(t, err)

	assert.Equal(t, PreReleasesInclude, ss.PreReleases(), "set")
	assert.Equal(t, PreReleasesExclude, ss.List()[0].PreReleases(),
		`>=1.0 autodetects Exclude; upstream's Specifier(">=1.0").prereleases is False`)
}

// A policy passed to NewSpecifier (the SINGULAR constructor) is that
// specifier's own, so it must still stick. This is the boundary of the fix: the
// distinction is set-policy vs member-policy, not "never stamp".
func TestSingularConstructorPolicyStillSticks(t *testing.T) {
	for _, policy := range []PreReleases{PreReleasesInclude, PreReleasesExclude} {
		s, err := NewSpecifier(">=1.0.dev1", WithPreReleases(policy))
		require.NoError(t, err)
		assert.Equal(t, policy, s.PreReleases(), "policy %s", policy)
	}
}

// FINDING 6. And must not re-stamp member policies.
//
// The reproduction is specific: a member carrying an explicit Exclude, in a set
// whose own policy is unset, combined with a plain set. Re-stamping wrote the
// combined policy (Auto) over the member's Exclude; the member then autodetected
// Include off its ".dev1" operand; and the set's resolved policy went from Auto
// to Include, so the combined set offered a pre-release the original held back.
func TestAndDoesNotRestampMemberPolicies(t *testing.T) {
	excluded, err := NewSpecifier(">=1.0.dev1", WithPreReleases(PreReleasesExclude))
	require.NoError(t, err)
	withExplicitMember := NewSpecifiersFrom([]Specifier{excluded})
	plain, err := NewSpecifiers("<2.0")
	require.NoError(t, err)

	// Baseline: the member's explicit Exclude keeps the set from resolving to
	// Include.
	require.Equal(t, PreReleasesExclude, withExplicitMember.List()[0].PreReleases())
	require.Equal(t, PreReleasesAuto, withExplicitMember.PreReleases())

	combined, err := withExplicitMember.And(plain)
	require.NoError(t, err)

	assert.Equal(t, PreReleasesExclude, combined.List()[0].PreReleases(),
		"And must not erase the member's explicit policy")
	assert.Equal(t, PreReleasesAuto, combined.PreReleases(),
		"and so the set's resolved policy must not flip to Include")
}

// And must not mutate its operands at all -- the property the re-stamp violated.
func TestAndDoesNotMutateItsOperands(t *testing.T) {
	excluded, err := NewSpecifier(">=1.0.dev1", WithPreReleases(PreReleasesExclude))
	require.NoError(t, err)
	left := NewSpecifiersFrom([]Specifier{excluded})
	right, err := NewSpecifiers("<2.0", WithPreReleases(PreReleasesInclude))
	require.NoError(t, err)

	leftBefore := left.List()[0].PreReleases()
	rightBefore := right.List()[0].PreReleases()
	leftStr, rightStr := left.String(), right.String()

	_, err = left.And(right)
	require.NoError(t, err)

	assert.Equal(t, leftBefore, left.List()[0].PreReleases(), "left member unchanged")
	assert.Equal(t, rightBefore, right.List()[0].PreReleases(), "right member unchanged")
	assert.Equal(t, leftStr, left.String(), "left rendering unchanged")
	assert.Equal(t, rightStr, right.String(), "right rendering unchanged")
}

// FINDING 6, second half. NewSpecifiersFrom carries per-member policies through
// rather than overwriting them -- which is the whole reason to accept
// already-parsed specifiers.
func TestNewSpecifiersFromPreservesMemberPolicies(t *testing.T) {
	incl, err := NewSpecifier(">=1.0", WithPreReleases(PreReleasesInclude))
	require.NoError(t, err)
	excl, err := NewSpecifier("<2.0.dev1", WithPreReleases(PreReleasesExclude))
	require.NoError(t, err)

	// A set-level policy is the SET's, and must not reach the members.
	ss := NewSpecifiersFrom([]Specifier{incl, excl}, WithPreReleases(PreReleasesExclude))

	assert.Equal(t, PreReleasesExclude, ss.PreReleases(), "the set's own policy")
	require.Len(t, ss.List(), 2)
	assert.Equal(t, PreReleasesInclude, ss.List()[0].PreReleases(), "member 0 keeps Include")
	assert.Equal(t, PreReleasesExclude, ss.List()[1].PreReleases(), "member 1 keeps Exclude")
}

// FINDING 8. The zero-value Specifier constrains nothing, consistently.
//
// Before the fix the two readings disagreed on exactly one input class:
// Contains("1.0") was true via Check's nil guard, while Contains("lolwat") was
// false because the unparseable branch admits a non-version only for "===".
// Meanwhile NewSpecifiers("").Contains("lolwat") is true. Three spellings of
// "no constraint" have to agree.
func TestZeroValueSpecifierConstrainsNothingConsistently(t *testing.T) {
	var zero Specifier

	items := []string{"1.0", "0", "2.0a1", "1!3.0", "1.0+local", "lolwat", "not a version", ""}

	for _, item := range items {
		assert.True(t, zero.Contains(item), "zero Specifier must admit %q", item)
	}

	// Filter keeps everything, in order.
	assert.Equal(t, items, zero.Filter(items))

	// And it agrees with the other two spellings of "no constraint".
	emptySet, err := NewSpecifiers("")
	require.NoError(t, err)
	zeroSet := Specifiers{}
	for _, item := range items {
		assert.Equal(t, emptySet.Contains(item), zero.Contains(item),
			`zero Specifier and NewSpecifiers("") must agree on %q`, item)
		assert.Equal(t, emptySet.Contains(item), zeroSet.Contains(item),
			`zero Specifiers and NewSpecifiers("") must agree on %q`, item)
	}

	// Check already agreed and must keep agreeing.
	assert.True(t, zero.Check(MustParse("1.0")))
}

// A REAL specifier must not be caught by the zero-value shortcut. The guard is
// on the nil operator function, and a parsed bare version has an empty operator
// string but a non-nil function -- so gating on the operator string instead
// would have turned every bare-version constraint into a no-op.
func TestZeroValueShortcutDoesNotSwallowRealSpecifiers(t *testing.T) {
	bare, err := NewSpecifier("2.0")
	require.NoError(t, err)
	assert.Equal(t, "", bare.Operator(), "a bare version has an empty operator")
	assert.True(t, bare.Contains("2.0"))
	assert.False(t, bare.Contains("3.0"), "a bare version still constrains")
	assert.False(t, bare.Contains("lolwat"), "and still rejects a non-version")

	normal, err := NewSpecifier(">=2.0")
	require.NoError(t, err)
	assert.False(t, normal.Contains("1.0"))
	assert.False(t, normal.Contains("lolwat"))
}

// CONTAINER -> MEMBER STAMPING AUDIT.
//
// The theme above is container state overwriting member state. This pins the
// invariant directly, for every constructor that produces a set, so a future
// change that reintroduces a stamp fails here rather than in a subtle
// policy-resolution assertion.
//
// Sites checked and found CLEAN, recorded so the audit is legible rather than
// merely asserted:
//
//	version.go cmpkey (k.pre = part.Infinity / NegativeInfinity)
//	              writes the comparison KEY's pre field from the version's OWN
//	              pre segment. Not container state, and not a Specifier.
//	version.go normalizeLocal (segments[i] = ...)
//	              rewrites the version's own local label in place. Clean.
//	filterGeneric (strs[i] = key(it))
//	              derives a string from the caller's item via the caller's key
//	              function; touches no Specifier. Clean.
//	filterKeep / Specifiers.filterKeep / matchAll / groupMatches /
//	allArbitraryMatch / applyPEP440Preference
//	              every write is to the boolean result mask. All read-only over
//	              members. Clean.
//	dedupeSpecifiers
//	              copies member VALUES into a fresh slice and writes no field.
//	              Clean.
//	orGroups
//	              returns the existing groups or a synthetic empty one; no
//	              mutation. Clean.
//	String / Len / List / Equal / canonicalGroupKeys / PreReleases
//	              read-only. Clean.
//	marker.Environment.With
//	              copies the receiver and writes only the named fields on the
//	              COPY; already covered by TestWithDoesNotMutateReceiver. Clean.
//
// After the fix, `newSpecifier` is the ONLY place a member's policy is set from
// a conf, and it is reached with a real conf only from NewSpecifier, where the
// conf is that specifier's own.
func TestNoConstructorStampsMemberPolicyFromSetPolicy(t *testing.T) {
	// A pre-release operand so that Auto autodetects Include, and a plain one so
	// that Auto autodetects Exclude. If a set policy leaked onto the members,
	// one of the two would report the set's value instead.
	const preOperand = ">=1.0.dev1"
	const finalOperand = ">=1.0"

	for _, setPolicy := range []PreReleases{PreReleasesAuto, PreReleasesInclude, PreReleasesExclude} {
		t.Run("NewSpecifiers__"+setPolicy.String(), func(t *testing.T) {
			ss, err := NewSpecifiers(preOperand+","+finalOperand, WithPreReleases(setPolicy))
			require.NoError(t, err)
			require.Len(t, ss.List(), 2)
			assert.Equal(t, PreReleasesInclude, ss.List()[0].PreReleases(), preOperand)
			assert.Equal(t, PreReleasesExclude, ss.List()[1].PreReleases(), finalOperand)
		})

		t.Run("NewRSpecifiers__"+setPolicy.String(), func(t *testing.T) {
			rs, err := NewRSpecifiers(preOperand+","+finalOperand, identitySanitizer,
				WithPreReleases(setPolicy))
			require.NoError(t, err)
			require.Len(t, rs.List(), 2)
			assert.Equal(t, PreReleasesInclude, rs.List()[0].PreReleases(), preOperand)
			assert.Equal(t, PreReleasesExclude, rs.List()[1].PreReleases(), finalOperand)
		})

		t.Run("NewSpecifiersWithSanitizer__"+setPolicy.String(), func(t *testing.T) {
			ws, err := NewSpecifiersWithSanitizer(preOperand+","+finalOperand, identitySanitizer,
				WithPreReleases(setPolicy))
			require.NoError(t, err)
			require.Len(t, ws.List(), 2)
			assert.Equal(t, PreReleasesInclude, ws.List()[0].PreReleases(), preOperand)
			assert.Equal(t, PreReleasesExclude, ws.List()[1].PreReleases(), finalOperand)
		})

		t.Run("OR_groups__"+setPolicy.String(), func(t *testing.T) {
			or, err := NewSpecifiers(preOperand+"||"+finalOperand, WithPreReleases(setPolicy))
			require.NoError(t, err)
			require.Len(t, or.List(), 2)
			assert.Equal(t, PreReleasesInclude, or.List()[0].PreReleases())
			assert.Equal(t, PreReleasesExclude, or.List()[1].PreReleases())
		})

		// And the deprecated two-state option must not stamp either.
		t.Run("WithPreRelease__"+setPolicy.String(), func(t *testing.T) {
			legacy, err := NewSpecifiers(preOperand+","+finalOperand,
				WithPreRelease(setPolicy == PreReleasesInclude))
			require.NoError(t, err)
			require.Len(t, legacy.List(), 2)
			assert.Equal(t, PreReleasesInclude, legacy.List()[0].PreReleases())
			assert.Equal(t, PreReleasesExclude, legacy.List()[1].PreReleases())
		})
	}
}
