// SPDX-License-Identifier: Apache-2.0 OR MIT

// Ordering properties, ported from pypa/packaging's
// tests/property/test_version_ordering.py at pinned SHA
// 4eb0753dba8fcaaac8eb75463374e448f0931558.
//
// Upstream's suite also asserts the *shape* of parsed fields — that
// version.release is a tuple of non-negative ints, that version.pre is a
// (letter, int) pair, and so on. Those are not ported, for two reasons: this
// module exports no accessor for epoch, release, pre, post, or dev (only
// String, BaseVersion, Public, Local, Original, IsPreRelease, IsPostRelease
// and the comparison methods), and in Go the field types make most of those
// assertions statically true rather than testable. What remains portable is
// the observable part, which is also the part a resolver depends on: ordering.

package proptest

import (
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/posit-dev/go-python-packaging/version"
)

// parseOrFatal parses a literal version from a plain (non-property) table
// test. Property tests use mustParse, which reports through *rapid.T.
func parseOrFatal(t *testing.T, s string) version.Version {
	t.Helper()
	v, err := version.Parse(s)
	if err != nil {
		t.Fatalf("version.Parse(%q): unexpected error %v", s, err)
	}
	return v
}

func TestEpochSortsByNumericValue(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		a := rapid.IntRange(0, 5).Draw(t, "epochA")
		b := rapid.IntRange(0, 5).Draw(t, "epochB")
		release := releaseSegment().Draw(t, "sharedRelease")

		va := mustParse(t, fmt.Sprintf("%d!%s", a, release))
		vb := mustParse(t, fmt.Sprintf("%d!%s", b, release))

		switch {
		case a < b:
			if !va.LessThan(vb) {
				t.Fatalf("epoch %d should sort below %d at release %s", a, b, release)
			}
		case a > b:
			if !va.GreaterThan(vb) {
				t.Fatalf("epoch %d should sort above %d at release %s", a, b, release)
			}
		default:
			if !va.Equal(vb) {
				t.Fatalf("equal epochs %d should compare equal at release %s", a, release)
			}
		}
	})
}

func TestImplicitEpochIsZero(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		release := releaseSegment().Draw(t, "release")

		implicit := mustParse(t, release)
		explicit := mustParse(t, "0!"+release)

		if !implicit.Equal(explicit) {
			t.Fatalf("%q and %q should be equal, got %q vs %q",
				release, "0!"+release, implicit, explicit)
		}
	})
}

func TestHigherEpochBeatsAnyRelease(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		epoch := rapid.IntRange(1, 5).Draw(t, "higherEpoch")
		lowRelease := releaseSegment().Draw(t, "epochZeroRelease")
		highRelease := releaseSegment().Draw(t, "higherEpochRelease")

		// The epoch dominates: no release value at epoch 0 can outrank a
		// version at a higher epoch, however large.
		zero := mustParse(t, lowRelease)
		higher := mustParse(t, fmt.Sprintf("%d!%s", epoch, highRelease))

		if !higher.GreaterThan(zero) {
			t.Fatalf("%q should outrank %q regardless of release values", higher, zero)
		}
	})
}

func TestReleaseSortsLikeZeroPaddedTuple(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		a := rapid.SliceOfN(rapid.IntRange(0, 50), 1, 4).Draw(t, "releaseA")
		b := rapid.SliceOfN(rapid.IntRange(0, 50), 1, 4).Draw(t, "releaseB")

		va := mustParse(t, joinInts(a))
		vb := mustParse(t, joinInts(b))

		// Independent reference: compare the two release tuples
		// component-wise, treating a missing component as 0. This is the
		// definition PEP 440 gives; it is not derived from the library.
		want := comparePaddedTuples(a, b)

		if got := va.Compare(vb); got != want {
			t.Fatalf("Compare(%q, %q) = %d, want %d (zero-padded tuple order)",
				joinInts(a), joinInts(b), got, want)
		}
	})
}

func comparePaddedTuples(a, b []int) int {
	n := max(len(a), len(b))
	for i := range n {
		av, bv := 0, 0
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if av != bv {
			if av < bv {
				return -1
			}
			return 1
		}
	}
	return 0
}

func TestTrailingZerosAreEqual(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		base := rapid.SliceOfN(smallInt(), 1, 3).Draw(t, "baseRelease")
		extra := rapid.IntRange(1, 3).Draw(t, "trailingZeroCount")

		short := joinInts(base)
		padded := short + strings.Repeat(".0", extra)

		if !mustParse(t, short).Equal(mustParse(t, padded)) {
			t.Fatalf("%q and %q should be equal", short, padded)
		}
	})
}

// TestFullSuffixChain is the composite of upstream's individual
// dev/alpha/beta/rc/final/post ordering properties:
//
//	.devN < aN < bN < rcN < final < .postN
func TestFullSuffixChain(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		release := releaseSegment().Draw(t, "release")
		n := smallInt().Draw(t, "suffixNum")
		post := smallInt().Draw(t, "postNum")

		chain := []string{
			fmt.Sprintf("%s.dev%d", release, n),
			fmt.Sprintf("%sa%d", release, n),
			fmt.Sprintf("%sb%d", release, n),
			fmt.Sprintf("%src%d", release, n),
			release,
			fmt.Sprintf("%s.post%d", release, post),
		}

		for i := range len(chain) - 1 {
			lo, hi := mustParse(t, chain[i]), mustParse(t, chain[i+1])
			if !lo.LessThan(hi) {
				t.Fatalf("expected %q < %q in the suffix chain", chain[i], chain[i+1])
			}
		}
	})
}

func TestCEqualsRC(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		release := releaseSegment().Draw(t, "release")
		n := smallInt().Draw(t, "preNum")

		c := mustParse(t, fmt.Sprintf("%sc%d", release, n))
		rc := mustParse(t, fmt.Sprintf("%src%d", release, n))

		if !c.Equal(rc) {
			t.Fatalf("c%d and rc%d should be equal at release %s, got %q vs %q",
				n, n, release, c, rc)
		}

		// And the "c" spelling must sort above beta, which is what makes the
		// c-is-rc aliasing observable rather than cosmetic.
		b := mustParse(t, fmt.Sprintf("%sb%d", release, n))
		if !c.GreaterThan(b) {
			t.Fatalf("c%d should sort above b%d at release %s", n, n, release)
		}
	})
}

// TestSuffixChainWithinPreRelease ports upstream's
// TestOrderingWithinPreRelease: .devN < <no suffix> < .postN, all sharing one
// pre-release.
func TestSuffixChainWithinPreRelease(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		release := releaseSegment().Draw(t, "release")
		tag := preTag().Draw(t, "preTag")
		preNum := smallInt().Draw(t, "preNum")
		devNum := smallInt().Draw(t, "devNum")
		postNum := smallInt().Draw(t, "postNum")

		pre := fmt.Sprintf("%s%s%d", release, tag, preNum)
		chain := []string{
			fmt.Sprintf("%s.dev%d", pre, devNum),
			pre,
			fmt.Sprintf("%s.post%d", pre, postNum),
		}

		for i := range len(chain) - 1 {
			lo, hi := mustParse(t, chain[i]), mustParse(t, chain[i+1])
			if !lo.LessThan(hi) {
				t.Fatalf("expected %q < %q within a pre-release", chain[i], chain[i+1])
			}
		}
	})
}

// TestNumericOrderingWithinSharedPrefix covers upstream's pre/post/dev numeric
// ordering properties in one place: within a single suffix kind, ordering is
// numeric, not lexicographic. This is the shape that produced the silent wrong
// answer in rstudio/package-manager#19369.
func TestNumericOrderingWithinSharedPrefix(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		release := releaseSegment().Draw(t, "release")
		tag := preTag().Draw(t, "preTag")
		a := smallInt().Draw(t, "numA")
		b := smallInt().Draw(t, "numB")

		forms := map[string]string{
			"pre":  "%s" + tag + "%d",
			"post": "%s.post%d",
			"dev":  "%s.dev%d",
		}

		for kind, format := range forms {
			va := mustParse(t, fmt.Sprintf(format, release, a))
			vb := mustParse(t, fmt.Sprintf(format, release, b))

			want := 0
			if a < b {
				want = -1
			} else if a > b {
				want = 1
			}

			if got := va.Compare(vb); got != want {
				t.Fatalf("%s: Compare(%q, %q) = %d, want %d", kind, va, vb, got, want)
			}
		}
	})
}

// orderedSpecExample is PEP 440's own worked ordering example, quoted verbatim
// from upstream's ORDERED_VERSIONS. Indices 13-15 are the local-label entries;
// upstream excludes them from the strict-ordering pass because a local label
// orders only against versions sharing its public part.
var orderedSpecExample = []string{
	"1.dev0", "1.0.dev456", "1.0a1", "1.0a2.dev456", "1.0a12.dev456", "1.0a12",
	"1.0b1.dev456", "1.0b2", "1.0b2.post345.dev456", "1.0b2.post345",
	"1.0rc1.dev456", "1.0rc1", "1.0", "1.0+abc.5", "1.0+abc.7", "1.0+5",
	"1.0.post456.dev34", "1.0.post456", "1.0.15", "1.1.dev1",
}

var localIndices = map[int]bool{13: true, 14: true, 15: true}

// TestSpecExamplePairwiseOrdering is a plain table test, not a property.
// Upstream's equivalent is likewise not @given-driven: the list is PEP 440's
// normative example, so there is nothing to generate.
func TestSpecExamplePairwiseOrdering(t *testing.T) {
	for i := range orderedSpecExample {
		for j := i + 1; j < len(orderedSpecExample); j++ {
			if localIndices[i] || localIndices[j] {
				continue
			}

			lo := parseOrFatal(t, orderedSpecExample[i])
			hi := parseOrFatal(t, orderedSpecExample[j])

			if !lo.LessThan(hi) {
				t.Errorf("PEP 440 spec example: expected %q (index %d) < %q (index %d)",
					orderedSpecExample[i], i, orderedSpecExample[j], j)
			}
		}
	}
}

// TestSpecExampleLocalOrdering asserts the part upstream's strict pass skips:
// the three local labels order among themselves and sit above their shared
// public version.
func TestSpecExampleLocalOrdering(t *testing.T) {
	// "1.0+5" is numeric and "1.0+abc.N" is lexicographic; PEP 440 orders a
	// numeric local segment above an alphabetic one, so +5 is the greatest of
	// the three despite appearing last in the spec's list. The first pair also
	// covers "a local label sorts above its bare public version".
	for _, tc := range []struct{ lower, higher string }{
		{"1.0", "1.0+abc.5"},
		{"1.0+abc.5", "1.0+abc.7"},
		{"1.0+abc.7", "1.0+5"},
	} {
		lo := parseOrFatal(t, tc.lower)
		hi := parseOrFatal(t, tc.higher)
		if !lo.LessThan(hi) {
			t.Errorf("expected %q < %q", tc.lower, tc.higher)
		}
	}
}

func TestVersionComparisonIsTotalOrder(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Total order is the property a resolver actually depends on, and it
		// has no upstream counterpart. Drawn from versions sharing a release
		// segment as well as fully independent ones, because ties and
		// near-ties are where antisymmetry and transitivity break.
		a := parsedVersion(t, versionString(true, 1), "a")
		b := parsedVersion(t, versionString(true, 1), "b")
		c := parsedVersion(t, versionString(true, 1), "c")

		ab, ba := a.Compare(b), b.Compare(a)

		// Totality: every pair is comparable, and Compare returns one of -1, 0, 1.
		for _, got := range []int{ab, ba} {
			if got < -1 || got > 1 {
				t.Fatalf("Compare returned %d, want one of -1, 0, 1 (%q vs %q)", got, a, b)
			}
		}

		// Antisymmetry.
		if ab != -ba {
			t.Fatalf("antisymmetry: Compare(%q,%q)=%d but Compare(%q,%q)=%d", a, b, ab, b, a, ba)
		}

		// Reflexivity.
		if a.Compare(a) != 0 {
			t.Fatalf("reflexivity: Compare(%q,%q) = %d, want 0", a, a, a.Compare(a))
		}

		// Transitivity of <=.
		if a.LessThanOrEqual(b) && b.LessThanOrEqual(c) && !a.LessThanOrEqual(c) {
			t.Fatalf("transitivity: %q <= %q <= %q but not %q <= %q", a, b, c, a, c)
		}

		// Equal versions must agree with Compare == 0, and the convenience
		// methods must agree with Compare.
		if a.Equal(b) != (ab == 0) {
			t.Fatalf("Equal/Compare disagree for %q vs %q: Equal=%v Compare=%d", a, b, a.Equal(b), ab)
		}
		if a.LessThan(b) != (ab < 0) {
			t.Fatalf("LessThan/Compare disagree for %q vs %q", a, b)
		}
		if a.GreaterThan(b) != (ab > 0) {
			t.Fatalf("GreaterThan/Compare disagree for %q vs %q", a, b)
		}
	})
}

// TestSortedVersionsMatchesCompare checks the exported sort.Interface
// implementation agrees with Compare, since a resolver sorts candidate lists
// through it rather than comparing pairwise.
func TestSortedVersionsMatchesCompare(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		a := parsedVersion(t, versionString(false, 1), "a")
		b := parsedVersion(t, versionString(false, 1), "b")

		sv := version.SortedVersions{a, b}
		if got, want := sv.Less(0, 1), a.Compare(b) < 0; got != want {
			t.Fatalf("SortedVersions.Less(%q, %q) = %v, want %v", a, b, got, want)
		}
		if sv.Len() != 2 {
			t.Fatalf("SortedVersions.Len() = %d, want 2", sv.Len())
		}
	})
}
