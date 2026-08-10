// SPDX-License-Identifier: Apache-2.0 OR MIT

// Regression tests for one theme with three symptoms: Specifiers is an
// OR-of-ANDs, and treating it as a flat bag of specifiers is wrong.
//
//	HIGH   an empty "||" segment became an empty AND-group, which matches
//	       everything, so ">=1||" silently admitted every version.
//	MEDIUM And concatenated the two operands' members instead of distributing,
//	       so (A||B) AND C became A AND B AND C and matched nothing.
//	MEDIUM Equal compared a flattened member set, so ">=1||<2" (admits
//	       everything) compared equal to ">=1,<2" (admits the intersection).
//
// pypa/packaging cannot arbitrate any of these: it has no "||" operator at all,
// since the OR syntax is this package's extension for the R constraint path. The
// reference here is the operator's own meaning.

package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// identitySanitizer, the no-op sanitizer the R entry point requires, is
// declared in adjacent_constraint_test.go and reused here.

// A malformed "||" expression must be an ERROR. The failure mode being guarded
// against is not "accepts something odd" -- it is that ">=1||" parsed happily
// and then admitted 0.1, a version below its own floor, with nothing logged and
// nothing failing.
func TestEmptyOrSegmentIsRejected(t *testing.T) {
	malformed := []string{
		">=1||",      // trailing
		"||>=1",      // leading
		">=1||||<2",  // doubled
		"||",         // nothing but the operator
		">=1|| ||<2", // doubled with whitespace
		" || >=1",    // leading with whitespace
		">=1,<2||",   // trailing after a multi-member group
		"||||",       // several
	}

	for _, in := range malformed {
		t.Run(in, func(t *testing.T) {
			_, err := NewSpecifiers(in)
			assert.Error(t, err, "NewSpecifiers(%q) must be an error", in)

			// The R entry point shares this code path, and is the one PPM uses
			// for R version constraints.
			_, err = NewRSpecifiers(in, identitySanitizer)
			assert.Error(t, err, "NewRSpecifiers(%q) must be an error", in)
		})
	}
}

// The specific consequence, asserted directly rather than only via the parse
// error: a trailing "||" must never be able to turn a real constraint into one
// that admits everything.
func TestEmptyOrSegmentCannotDisableAConstraint(t *testing.T) {
	below := MustParse("0.1")

	for _, in := range []string{">=1||", "||>=1", ">=1||||<2"} {
		t.Run(in, func(t *testing.T) {
			ss, err := NewSpecifiers(in)
			if err == nil {
				assert.False(t, ss.Check(below),
					"%q parsed, and then admitted 0.1 -- the constraint was disabled, not narrowed", in)
				t.Fatalf("%q must not parse at all", in)
			}

			// A well-formed OR of the same members still behaves like an OR.
			ok, err := NewSpecifiers(">=1||<2")
			require.NoError(t, err)
			assert.True(t, ok.Check(below), "0.1 satisfies the <2 branch")
			assert.True(t, ok.Check(MustParse("99")), "99 satisfies the >=1 branch")
		})
	}
}

// CONTROL: the "||" rule and the comma rule are separate, and each must hold
// without weakening the other.
//
// These interact in a way that is easy to get wrong, which is why they are
// tested together. Blank comma items are DROPPED (upstream's rule), and a group
// left with no items at all is the universal set (also upstream's rule --
// `SpecifierSet(",")` is universal). Compose those two naively and a comma-only
// "||" SEGMENT becomes an empty AND-group, which matches everything, which
// disables the whole constraint -- the same hole a trailing "||" opened, reached
// by a comma instead.
//
// So the rules are: commas are tolerated freely WITHIN a group, and a
// specifier-less group is universal ONLY when it is the entire input.
func TestCommaToleranceDoesNotCrossTheOrBoundary(t *testing.T) {
	t.Run("empty input is still the universal set", func(t *testing.T) {
		for _, in := range []string{"", "   ", "\t"} {
			ss, err := NewSpecifiers(in)
			require.NoError(t, err, "input %q", in)
			assert.Equal(t, 0, ss.Len())
			assert.Equal(t, "", ss.String())
			for _, v := range []string{"0", "0.1", "1.0", "2.0a1", "1!3.0", "1.0+local"} {
				assert.True(t, ss.Check(MustParse(v)), "%q must admit %s", in, v)
			}

			rs, err := NewRSpecifiers(in, identitySanitizer)
			require.NoError(t, err, "R input %q", in)
			assert.True(t, rs.Check(MustParse("0.1")))
		}
	})

	t.Run("blank comma items are dropped within a group", func(t *testing.T) {
		// Every one of these is accepted by packaging 26.2 and now here.
		tests := []struct {
			in      string
			wantLen int
		}{
			{",>=1", 1},
			{",,>=1", 1},
			{">=1,", 1},
			{">=1,,<2", 2},
			{">=1,,,<2", 2},
			{">=1 , , <2", 2},
		}
		for _, tt := range tests {
			ss, err := NewSpecifiers(tt.in)
			require.NoError(t, err, "%q must be accepted, as upstream accepts it", tt.in)
			assert.Equal(t, tt.wantLen, ss.Len(), "input %q", tt.in)
			// The dropped commas must not turn into a wider constraint.
			assert.False(t, ss.Check(MustParse("0.1")), "%q must still enforce >=1", tt.in)
		}
	})

	t.Run("a comma-only input is universal, matching SpecifierSet(\",\")", func(t *testing.T) {
		for _, in := range []string{",", ",,", " , ", " , , "} {
			ss, err := NewSpecifiers(in)
			require.NoError(t, err, "input %q", in)
			assert.Equal(t, 0, ss.Len(), "input %q", in)
			assert.True(t, ss.Check(MustParse("0.1")), "input %q", in)
		}
	})

	t.Run("but a comma-only || SEGMENT is an error, not universal", func(t *testing.T) {
		// ⚠️ THE interaction. Each of these is a real constraint plus a segment
		// that is nothing but punctuation. Treating that segment as universal
		// would make the OR admit every version -- the same failure the "||"
		// fix closed, reached by a comma typo.
		for _, in := range []string{">=1||,", ",||>=1", ">=1||,,", ">=1|| , ||<2", ">=1,<2||,"} {
			ss, err := NewSpecifiers(in)
			if err == nil {
				assert.False(t, ss.Check(MustParse("0.1")),
					"%q parsed, and then admitted 0.1 -- a comma-only segment disabled the constraint", in)
				t.Errorf("%q must be rejected", in)
			}

			// The R path shares this code and is what PPM uses.
			rs, rerr := NewRSpecifiers(in, identitySanitizer)
			if rerr == nil {
				assert.False(t, rs.Check(MustParse("0.1")),
					"NewRSpecifiers(%q) parsed, and then admitted 0.1", in)
				t.Errorf("NewRSpecifiers(%q) must be rejected", in)
			}
		}
	})

	t.Run("commas still work inside every branch of a real OR", func(t *testing.T) {
		ss, err := NewSpecifiers(",>=1.0,<2.0||,>=10.0,<11.0")
		require.NoError(t, err)
		assert.Equal(t, 4, ss.Len())
		assert.True(t, ss.Check(MustParse("1.5")))
		assert.True(t, ss.Check(MustParse("10.5")))
		assert.False(t, ss.Check(MustParse("5.0")))
	})

	t.Run("dropping commas cannot smuggle in an adjacent pair", func(t *testing.T) {
		// The comma between two constraints is still REQUIRED (see
		// TestAdjacentConstraintsRejected). Dropping BLANK items must not
		// weaken that: what survives is re-joined with commas and revalidated.
		for _, in := range []string{">=1<2", ">=1 <2", ",>=1<2", ">=1<2,", ",,>=1 <2,,"} {
			_, err := NewSpecifiers(in)
			assert.Error(t, err, "%q: a missing comma between constraints stays an error", in)
		}
	})

	t.Run("well-formed OR groups still parse", func(t *testing.T) {
		ss, err := NewSpecifiers(">=1.0,<2.0||>=3.0")
		require.NoError(t, err)
		assert.True(t, ss.Check(MustParse("1.5")))
		assert.True(t, ss.Check(MustParse("4.0")))
		assert.False(t, ss.Check(MustParse("2.5")))
	})

	t.Run("the R wildcard segment still works", func(t *testing.T) {
		rs, err := NewRSpecifiers("*", identitySanitizer)
		require.NoError(t, err)
		assert.True(t, rs.Check(MustParse("1.0")))

		rs, err = NewRSpecifiers("*||>=3.0", identitySanitizer)
		require.NoError(t, err)
		assert.True(t, rs.Check(MustParse("1.0")))
	})
}

// And must DISTRIBUTE over the OR-groups. Concatenating the members produced a
// strictly narrower constraint that usually matched nothing.
func TestAndDistributesOverOrGroups(t *testing.T) {
	left, err := NewSpecifiers("==1.0||==2.0")
	require.NoError(t, err)
	right, err := NewSpecifiers("<3.0")
	require.NoError(t, err)

	got, err := left.And(right)
	require.NoError(t, err)

	// (==1.0 || ==2.0) AND <3.0  ==  (==1.0,<3.0) || (==2.0,<3.0)
	assert.True(t, got.Check(MustParse("1.0")), "1.0 satisfies the first branch")
	assert.True(t, got.Check(MustParse("2.0")), "2.0 satisfies the second branch")
	assert.False(t, got.Check(MustParse("5.0")), "5.0 fails the <3.0 conjunct")

	// The concatenating version returned a single unsatisfiable group, so it
	// answered false for every one of the three.
	assert.NotEqual(t, 1, len(got.specifiers),
		"the result must keep both branches, not collapse to one AND-group")
}

func TestAndDistributesBothSides(t *testing.T) {
	left, err := NewSpecifiers(">=1.0,<2.0||>=10.0,<11.0")
	require.NoError(t, err)
	right, err := NewSpecifiers("!=1.5||!=10.5")
	require.NoError(t, err)

	got, err := left.And(right)
	require.NoError(t, err)
	assert.Equal(t, 4, len(got.specifiers), "2 groups AND 2 groups distributes to 4")

	// 1.5 is excluded by !=1.5 but the !=10.5 branch still admits it.
	assert.True(t, got.Check(MustParse("1.5")))
	assert.True(t, got.Check(MustParse("1.2")))
	assert.True(t, got.Check(MustParse("10.2")))
	assert.False(t, got.Check(MustParse("5.0")), "outside both ranges")
}

// The universal set is the identity of And, in both positions. A cartesian
// product against a set with zero groups would otherwise have produced zero
// groups and quietly discarded the other operand's constraints.
func TestAndUniversalIsIdentity(t *testing.T) {
	constrained, err := NewSpecifiers(">=1.0,<2.0")
	require.NoError(t, err)
	empty, err := NewSpecifiers("")
	require.NoError(t, err)

	for name, combine := range map[string]func() (Specifiers, error){
		"universal on the right": func() (Specifiers, error) { return constrained.And(empty) },
		"universal on the left":  func() (Specifiers, error) { return empty.And(constrained) },
		"zero value on the right": func() (Specifiers, error) {
			return constrained.And(Specifiers{})
		},
		"zero value on the left": func() (Specifiers, error) {
			return Specifiers{}.And(constrained)
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := combine()
			require.NoError(t, err)
			assert.True(t, got.Check(MustParse("1.5")), "must still admit what the constraint admits")
			assert.False(t, got.Check(MustParse("2.5")), "must still reject what the constraint rejects")
			assert.True(t, got.Equal(constrained), "universal AND X must equal X")
		})
	}
}

// Equal must respect conjunction boundaries: an OR and an AND of the same two
// members are different predicates.
func TestEqualRespectsOrGroupBoundaries(t *testing.T) {
	or, err := NewSpecifiers(">=1||<2")
	require.NoError(t, err)
	and, err := NewSpecifiers(">=1,<2")
	require.NoError(t, err)

	// They genuinely differ: the OR admits everything, the AND does not.
	assert.True(t, or.Check(MustParse("0.5")), "the OR admits 0.5 via the <2 branch")
	assert.False(t, and.Check(MustParse("0.5")), "the AND rejects 0.5 via >=1")

	assert.False(t, or.Equal(and), `">=1||<2" must not equal ">=1,<2"`)
	assert.False(t, and.Equal(or), "Equal must be symmetric")
}

// What Equal must still ignore, so the group-awareness does not overshoot.
func TestEqualStillIgnoresOrderDuplicatesAndPolicy(t *testing.T) {
	tests := []struct {
		left, right string
		want        bool
	}{
		// Within-group order and duplicates.
		{">=1,<2", "<2,>=1", true},
		{">=1,>=1", ">=1", true},
		{">=1.0.0", ">=1", true},
		// Group order.
		{">=1||<2", "<2||>=1", true},
		{"==1.0||==2.0||==3.0", "==3.0||==1.0||==2.0", true},
		// Duplicated whole groups collapse, because the keys are a SET.
		{">=1||>=1", ">=1", true},
		// Genuinely different.
		{">=1,<2", ">=1", false},
		{">=1||<2", ">=1", false},
		{"==1.0||==2.0", "==1.0||==3.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.left+"__"+tt.right, func(t *testing.T) {
			l, err := NewSpecifiers(tt.left)
			require.NoError(t, err)
			r, err := NewSpecifiers(tt.right)
			require.NoError(t, err)
			assert.Equal(t, tt.want, l.Equal(r))
			assert.Equal(t, tt.want, r.Equal(l), "Equal must be symmetric")
		})
	}

	// The pre-release policy is still not part of equality.
	a, err := NewSpecifiers(">=1||<2", WithPreReleases(PreReleasesInclude))
	require.NoError(t, err)
	b, err := NewSpecifiers(">=1||<2", WithPreReleases(PreReleasesExclude))
	require.NoError(t, err)
	assert.True(t, a.Equal(b))
}

// And and Equal must agree: the distributed result must equal the set written
// out by hand. If they disagreed, one of the two is wrong and neither's own
// tests would say which.
func TestAndAgreesWithEqual(t *testing.T) {
	left, err := NewSpecifiers("==1.0||==2.0")
	require.NoError(t, err)
	right, err := NewSpecifiers("<3.0")
	require.NoError(t, err)

	got, err := left.And(right)
	require.NoError(t, err)

	want, err := NewSpecifiers("==1.0,<3.0||==2.0,<3.0")
	require.NoError(t, err)
	assert.True(t, got.Equal(want), "distributed And must equal the hand-written distribution")

	// And it must NOT equal the concatenation, which is what it used to produce.
	concatenated, err := NewSpecifiers("==1.0,==2.0,<3.0")
	require.NoError(t, err)
	assert.False(t, got.Equal(concatenated),
		"the distributed result must not equal the flattened conjunction")
}

// Contains and Filter already walk the groups; pin that, so a later refactor
// toward List() cannot regress them the way Equal and And were regressed.
func TestContainsAndFilterRespectOrGroups(t *testing.T) {
	ss, err := NewSpecifiers("==1.0||==2.0")
	require.NoError(t, err)

	assert.True(t, ss.Contains("1.0"))
	assert.True(t, ss.Contains("2.0"))
	assert.False(t, ss.Contains("3.0"))

	assert.Equal(t, []string{"1.0", "2.0"}, ss.Filter([]string{"1.0", "2.0", "3.0"}))

	// A flattened reading would be the unsatisfiable ==1.0 AND ==2.0.
	assert.NotEmpty(t, ss.Filter([]string{"1.0"}),
		"a flattened reading would match nothing at all")

	// ContainsInstalled delegates to Contains, so it inherits the group walk.
	assert.True(t, ss.ContainsInstalled("1.0"))
	assert.False(t, ss.ContainsInstalled("3.0"))

	// The OR must survive an explicit pre-release policy too, since that takes
	// a different branch through filterKeep.
	for _, policy := range []PreReleases{PreReleasesAuto, PreReleasesInclude, PreReleasesExclude} {
		assert.True(t, ss.Contains("1.0", WithPreReleases(policy)), "policy %s", policy)
		assert.True(t, ss.Contains("2.0", WithPreReleases(policy)), "policy %s", policy)
		assert.False(t, ss.Contains("3.0", WithPreReleases(policy)), "policy %s", policy)
	}
}

// The audit's other half: the operations that consume a FLATTENED view are
// documented as doing so because they are genuinely structure-independent, and
// this pins that claim rather than leaving it as prose.
//
//	Len   - a COUNT, not a predicate. Documented as the total across groups.
//	        Its one semantic use is `Len() == 0` meaning "constrains nothing",
//	        which stays correct because the universal set is the only thing that
//	        can produce an empty group.
//	List  - ITERATION only; warned against in its doc comment.
//	PreReleases - a global policy hint ("does any member imply pre-releases"),
//	        which is upstream's `any(s.prereleases for s in self._specs)`. It
//	        does not decide what matches, so grouping cannot change it.
func TestFlattenedOperationsAreStructureIndependent(t *testing.T) {
	or, err := NewSpecifiers(">=1.0||<2.0")
	require.NoError(t, err)
	and, err := NewSpecifiers(">=1.0,<2.0")
	require.NoError(t, err)

	t.Run("Len counts members either way", func(t *testing.T) {
		assert.Equal(t, 2, or.Len())
		assert.Equal(t, 2, and.Len())
		assert.Len(t, or.List(), or.Len())
		assert.Len(t, and.List(), and.Len())
	})

	t.Run("Len()==0 means constrains nothing, and only the universal set does", func(t *testing.T) {
		universal, err := NewSpecifiers("")
		require.NoError(t, err)
		assert.Equal(t, 0, universal.Len())
		assert.Equal(t, 0, Specifiers{}.Len())

		// Anything that parsed from a non-blank input has at least one member,
		// so it can never be mistaken for the universal set.
		for _, in := range []string{">=1", ">=1||<2", ">=1,<2", "*", ">=1,"} {
			ss, err := NewSpecifiers(in)
			require.NoError(t, err, in)
			assert.NotZero(t, ss.Len(), "%q must not look like the universal set", in)
		}
	})

	t.Run("PreReleases is a global hint, unaffected by grouping", func(t *testing.T) {
		// A pre-release operand in either position resolves the set to Include,
		// whether the members are ANDed or ORed.
		orPre, err := NewSpecifiers(">=1.0||<2.0.dev1")
		require.NoError(t, err)
		andPre, err := NewSpecifiers(">=1.0,<2.0.dev1")
		require.NoError(t, err)
		assert.Equal(t, PreReleasesInclude, orPre.PreReleases())
		assert.Equal(t, PreReleasesInclude, andPre.PreReleases())

		// And with no pre-release operand, both stay unresolved.
		assert.Equal(t, PreReleasesAuto, or.PreReleases())
		assert.Equal(t, PreReleasesAuto, and.PreReleases())
	})
}

// NewSpecifiersFrom builds a single AND-group, which is correct: it is
// upstream's `SpecifierSet(iterable_of_Specifier)`, and upstream has no OR.
// Pinned so nobody "fixes" it into something that spreads members across groups.
func TestNewSpecifiersFromBuildsOneAndGroup(t *testing.T) {
	lower, err := NewSpecifier(">=1.0")
	require.NoError(t, err)
	upper, err := NewSpecifier("<2.0")
	require.NoError(t, err)

	ss := NewSpecifiersFrom([]Specifier{lower, upper})
	assert.Len(t, ss.specifiers, 1, "one AND-group, not one group per member")

	// Conjunction, so the midpoint matches and the outside does not.
	assert.True(t, ss.Check(MustParse("1.5")))
	assert.False(t, ss.Check(MustParse("2.5")))
	assert.False(t, ss.Check(MustParse("0.5")))

	// If it had built two OR-groups instead, 0.5 would match the <2.0 branch.
	parsed, err := NewSpecifiers(">=1.0,<2.0")
	require.NoError(t, err)
	assert.True(t, ss.Equal(parsed))

	or, err := NewSpecifiers(">=1.0||<2.0")
	require.NoError(t, err)
	assert.False(t, ss.Equal(or))
}
