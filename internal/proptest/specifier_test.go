// SPDX-License-Identifier: Apache-2.0 OR MIT

// Specifier matching properties, ported from pypa/packaging's
// tests/property/test_specifier_matching.py, test_specifier_comparison.py, and
// test_specifier_implied.py at pinned SHA
// 4eb0753dba8fcaaac8eb75463374e448f0931558.
//
// Upstream's specifier properties are far broader than what is portable here.
// This module exposes no SpecifierSet algebra (no &, no is_unsatisfiable, no
// filter), so the intersection, unsatisfiability, and filtering families have
// no counterpart -- they belong to go-pubgrub's versionset, tracked separately.
// What is portable is the matching relation itself, which is what Check does.

package proptest

import (
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func TestGreaterEqualMatchesIffOrderingHolds(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Ordered operators reject a local label on the right-hand side.
		bound := nonlocalVersionString().Draw(t, "bound")
		candidate := parsedVersion(t, nonlocalVersionString(), "candidate")

		boundVersion := mustParse(t, bound)
		ss := mustSpecifiers(t, ">="+bound)

		// A pre-release candidate is excluded by default unless the specifier
		// itself mentions a pre-release, which is a separate rule from the
		// ordering relation under test here. Restrict to the case where the
		// two rules cannot interact.
		if candidate.IsPreRelease() || boundVersion.IsPreRelease() {
			return
		}

		want := candidate.GreaterThanOrEqual(boundVersion)
		if got := ss.Check(candidate); got != want {
			t.Fatalf("Check(%q) against %q = %v, want %v (ordering says %v)",
				candidate, ">="+bound, got, want, want)
		}
	})
}

func TestStrictOperatorsExcludeTheBoundItself(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		bound := nonlocalVersionString().Draw(t, "bound")
		v := mustParse(t, bound)

		for _, op := range []string{">", "<"} {
			if mustSpecifiers(t, op+bound).Check(v) {
				t.Fatalf("%q should not match its own bound %q", op+bound, v)
			}
		}

		// And the inclusive forms must match it.
		for _, op := range []string{">=", "<=", "=="} {
			if !mustSpecifiers(t, op+bound).Check(v) {
				t.Fatalf("%q should match its own bound %q", op+bound, v)
			}
		}
	})
}

func TestNotEqualIsExactInverseOfEqual(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		bound := nonlocalVersionString().Draw(t, "bound")
		candidate := parsedVersion(t, versionString(true, 1), "candidate")

		eq := mustSpecifiers(t, "=="+bound)
		ne := mustSpecifiers(t, "!="+bound)

		if eq.Check(candidate) == ne.Check(candidate) {
			t.Fatalf("==%s and !=%s both returned %v for %q",
				bound, bound, eq.Check(candidate), candidate)
		}
	})
}

func TestWildcardIsSupersetOfExact(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// A wildcard operand must be a plain final release; a suffixed operand
		// is rejected outright for dev and post.
		bound := releaseVersionString(1).Draw(t, "bound")
		candidate := parsedVersion(t, versionString(false, 1), "candidate")

		exact := mustSpecifiers(t, "=="+bound)
		wildcard := mustSpecifiers(t, "=="+bound+".*")

		if exact.Check(candidate) && !wildcard.Check(candidate) {
			t.Fatalf("%q matches ==%s but not ==%s.*", candidate, bound, bound)
		}
	})
}

func TestWildcardIgnoresTrailingSegments(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		bound := releaseVersionString(1).Draw(t, "bound")
		extra := smallInt().Draw(t, "extraSegment")

		ss := mustSpecifiers(t, "=="+bound+".*")

		// The prefix match includes the bare bound and any extension of it.
		for _, candidate := range []string{bound, fmt.Sprintf("%s.%d", bound, extra)} {
			if !ss.Check(mustParse(t, candidate)) {
				t.Fatalf("==%s.* should match %q", bound, candidate)
			}
		}
	})
}

// TestExactMatchIsZeroPaddingSymmetric ports upstream's
// test_zero_padding_matches and test_zero_padding_symmetric: == compares
// release tuples with zero padding, so the operand and the candidate may carry
// different numbers of trailing zeros and must still match.
//
// This property was added after mutation testing: deleting the trailing-zero
// Normalize() in cmpkey (version/version.go:218) left every other property in
// this package passing, because Version.Equal pads independently. Only
// specifier matching observes that normalization.
func TestExactMatchIsZeroPaddingSymmetric(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		parts := releaseParts(1).Draw(t, "releaseParts")
		zeros := rapid.IntRange(1, 3).Draw(t, "trailingZeroCount")

		short := joinInts(parts)
		padded := short + strings.Repeat(".0", zeros)

		// ==short must match the padded candidate, and ==padded must match the
		// short one. Both directions, since the padding can be on either side.
		if !mustSpecifiers(t, "=="+short).Check(mustParse(t, padded)) {
			t.Fatalf("==%s should match %q", short, padded)
		}
		if !mustSpecifiers(t, "=="+padded).Check(mustParse(t, short)) {
			t.Fatalf("==%s should match %q", padded, short)
		}

		// And the ordered operators agree: neither is strictly greater.
		if mustSpecifiers(t, ">"+short).Check(mustParse(t, padded)) {
			t.Fatalf(">%s should not match %q, which is equal to it", short, padded)
		}
		if mustSpecifiers(t, "<"+short).Check(mustParse(t, padded)) {
			t.Fatalf("<%s should not match %q, which is equal to it", short, padded)
		}
	})
}

// TestCompatibleReleaseEqualsExpansion ports upstream's
// test_compatible_matches_iff_expansion_matches: ~=X.Y is defined as
// >=X.Y,==X.* -- the two forms must agree on every candidate.
func TestCompatibleReleaseEqualsExpansion(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		parts := releaseParts(2).Draw(t, "boundParts")
		candidate := parsedVersion(t, versionString(false, 2), "candidate")

		bound := joinInts(parts)
		prefix := joinInts(parts[:len(parts)-1])

		compatible := mustSpecifiers(t, "~="+bound)
		expansion := mustSpecifiers(t, fmt.Sprintf(">=%s,==%s.*", bound, prefix))

		if got, want := compatible.Check(candidate), expansion.Check(candidate); got != want {
			t.Fatalf("~=%s = %v but >=%s,==%s.* = %v for %q",
				bound, got, bound, prefix, want, candidate)
		}
	})
}

func TestCommaSeparatedIsConjunction(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		clauses := rapid.SliceOfN(specifierString(), 2, 3).Draw(t, "clauses")
		candidate := parsedVersion(t, versionString(false, 1), "candidate")

		joined := mustSpecifiers(t, strings.Join(clauses, ","))

		want := true
		for _, c := range clauses {
			if !mustSpecifiers(t, c).Check(candidate) {
				want = false
				break
			}
		}

		if got := joined.Check(candidate); got != want {
			t.Fatalf("%q on %q = %v, want %v (conjunction of %v)",
				strings.Join(clauses, ","), candidate, got, want, clauses)
		}
	})
}

// TestAddingClauseCanOnlyNarrow ports upstream's
// test_adding_clause_can_only_narrow: a specifier set is a conjunction, so an
// additional clause can never admit a version the shorter set rejected.
func TestAddingClauseCanOnlyNarrow(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		base := specifierString().Draw(t, "baseClause")
		extra := specifierString().Draw(t, "extraClause")
		candidate := parsedVersion(t, versionString(false, 1), "candidate")

		narrow := mustSpecifiers(t, base+","+extra)

		if narrow.Check(candidate) && !mustSpecifiers(t, base).Check(candidate) {
			t.Fatalf("%q matches the narrower %q but not %q",
				candidate, base+","+extra, base)
		}
	})
}

// TestLocalLabelIgnoredByOrderedSpecifiers ports upstream's
// test_local_ignored_for_ordering_specifiers. This is the property the
// local-normalization fix in version/version.go had to preserve.
func TestLocalLabelIgnoredByOrderedSpecifiers(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		base := nonlocalVersionString().Draw(t, "base")
		label := localLabel().Draw(t, "localLabel")

		bare := mustParse(t, base)
		withLocal := mustParse(t, base+"+"+label)

		// An ordered specifier compares against the candidate's public part,
		// so adding a local label must not change the verdict.
		for _, op := range []string{">=", "<="} {
			spec := mustSpecifiers(t, op+base)
			if spec.Check(bare) != spec.Check(withLocal) {
				t.Fatalf("%q: local label changed the verdict for %q vs %q",
					op+base, bare, withLocal)
			}
		}
	})
}

func TestOrderedSpecifiersRejectLocalOperand(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		base := releaseVersionString(1).Draw(t, "base")
		label := localLabel().Draw(t, "localLabel")

		// PEP 440 forbids a local label on the right-hand side of an ordered
		// operator, because a local label has no meaning across projects.
		for _, op := range []string{">=", "<=", ">", "<"} {
			if _, err := newSpecifiersOrError(op + base + "+" + label); err == nil {
				t.Fatalf("%q should be rejected: local label on an ordered operator",
					op+base+"+"+label)
			}
		}
	})
}

func TestZeroPaddedCandidateMatchesSameSpecifiers(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		bound := nonlocalVersionString().Draw(t, "bound")
		parts := releaseParts(1).Draw(t, "candidateParts")

		short := joinInts(parts)
		padded := short + ".0"

		for _, op := range []string{">=", "<=", ">", "<", "==", "!="} {
			ss := mustSpecifiers(t, op+bound)
			if got, want := ss.Check(mustParse(t, padded)), ss.Check(mustParse(t, short)); got != want {
				t.Fatalf("%q: %q gave %v but zero-padded %q gave %v",
					op+bound, short, want, padded, got)
			}
		}
	})
}
