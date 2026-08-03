// SPDX-License-Identifier: Apache-2.0 OR MIT

// Package proptest holds the module's property-based tests, adapted from
// pypa/packaging's Hypothesis suite (tests/property/) at pinned upstream SHA
// 4eb0753dba8fcaaac8eb75463374e448f0931558, plus properties specific to this
// module's own API contracts.
//
// # Why this package contains only _test.go files
//
// The generators below are shared by the version, requirement, marker, and
// extras property tests. Go cannot share _test.go helpers across packages, so
// the obvious alternative is a normal (non-test) file in an internal package —
// but that would put pgregory.net/rapid, which is MPL-2.0, into the non-test
// import graph of a public, dual-Apache-2.0-OR-MIT library.
//
// Keeping every file here a _test.go file keeps rapid strictly test-only:
//
//	go list -deps ./...        # rapid absent
//	go list -deps -test ./...  # rapid present
//
// The cost is that these tests can only use the exported API of the packages
// they exercise. That is not a real limitation for the properties here — they
// are all statements about observable behavior (ordering, normalization,
// round-tripping, specifier matching) rather than about internal structure.
//
// # Determinism
//
// rapid.Check takes no seed option; the seed comes from the -rapid.seed flag
// and defaults to random. That is deliberate and should stay that way. A fixed
// seed would run the same examples on every invocation, which is just a table
// test with extra steps and none of the table's readability. On failure rapid
// prints the reproducing -rapid.seed and the value of every named Draw, so
// every Draw below is named.
//
// # These properties do NOT subsume the conformance tables
//
// Established by mutation testing, not by assumption. Reverting
// Requirement.String()'s conditional marker separator to the unconditional
// " ; " form leaves every property in this package passing, because a
// round-trip property is invariant to a change of canonical form: it detects
// non-idempotence and parse failures, never "the canonical form moved". That
// mutation is caught only by requirement/conformance_test.go, which asserts
// exact bytes against upstream.
//
// The division of labor is therefore load-bearing: properties pin invariants
// over a wide input space, the conformance tables pin the exact output shape
// against pypa/packaging. Deleting either because the other exists will let
// real regressions through.
//
// Mutation testing also found a genuine gap here: dropping the trailing-zero
// Normalize() in version/version.go's cmpkey survived every property until
// TestExactMatchIsZeroPaddingSymmetric was added, because Version.Equal pads
// independently and only specifier matching observes that normalization.
package proptest

import (
	"strconv"
	"strings"

	"pgregory.net/rapid"

	"github.com/posit-dev/go-python-packaging/version"
)

// Generators below mirror pypa/packaging's tests/property/strategies.py. Where
// a generator diverges from upstream, the divergence is annotated — an
// unannotated divergence is a bug in this file, not a licence to change the
// library.

// smallInt mirrors upstream's small_ints: st.integers(min_value=0, max_value=20).
func smallInt() *rapid.Generator[int] { return rapid.IntRange(0, 20) }

// preTag mirrors upstream's pre_tags: st.sampled_from(["a", "b", "rc"]).
//
// Note these are the *normalized* spellings only. The alternate spellings
// (alpha, beta, c, pre, preview) are exercised by the normalization properties,
// which need to pair each spelling with its expected normal form.
func preTag() *rapid.Generator[string] {
	return rapid.SampledFrom([]string{"a", "b", "rc"})
}

// orderedOp and equalityOp mirror upstream's _ordered_ops and _equality_ops.
func orderedOp() *rapid.Generator[string] {
	return rapid.SampledFrom([]string{">=", "<=", ">", "<"})
}

func equalityOp() *rapid.Generator[string] {
	return rapid.SampledFrom([]string{"==", "!="})
}

// releaseParts mirrors upstream's release_segment before the "." join: 1 to 4
// components, each a small_int. minSegments raises the floor, which ~= needs
// because it rejects a single-segment operand.
func releaseParts(minSegments int) *rapid.Generator[[]int] {
	return rapid.SliceOfN(smallInt(), minSegments, 4)
}

// releaseSegment mirrors upstream's release_segment: a dotted release string
// such as "3.14.0".
func releaseSegment() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		return joinInts(releaseParts(1).Draw(t, "release"))
	})
}

func joinInts(parts []int) string {
	s := make([]string, len(parts))
	for i, p := range parts {
		s[i] = strconv.Itoa(p)
	}
	return strings.Join(s, ".")
}

// localLabel mirrors upstream's local_labels: 1 to 3 dot-joined segments, each
// either numeric (0-100) or one of a small pool of words.
func localLabel() *rapid.Generator[string] {
	segment := rapid.OneOf(
		rapid.Custom(func(t *rapid.T) string {
			return strconv.Itoa(rapid.IntRange(0, 100).Draw(t, "localNumeric"))
		}),
		rapid.SampledFrom([]string{"abc", "ubuntu", "local", "patch", "dev"}),
	)
	return rapid.Custom(func(t *rapid.T) string {
		return strings.Join(rapid.SliceOfN(segment, 1, 3).Draw(t, "localSegments"), ".")
	})
}

// versionString mirrors upstream's pep440_versions composite, but returns the
// string rather than a parsed Version.
//
// Returning a string is a deliberate divergence, and it is the safe direction.
// This module's version.Version has no exported fields and no exported
// constructor other than Parse/MustParse, and a zero-value Version{} panics in
// String() (version/version.go:290 indexes release[0] unconditionally). A
// generator that built Version values structurally would therefore be both
// impossible and a landmine; generating the string and routing it through
// Parse is the only correct construction path.
func versionString(includeLocal bool, minSegments int) *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		var b strings.Builder

		// Epoch: upstream draws from [None, 0, 1, 2, 3].
		if epoch := rapid.SampledFrom([]int{-1, 0, 1, 2, 3}).Draw(t, "epoch"); epoch >= 0 {
			b.WriteString(strconv.Itoa(epoch))
			b.WriteString("!")
		}

		b.WriteString(joinInts(releaseParts(minSegments).Draw(t, "releaseParts")))

		if rapid.Bool().Draw(t, "hasPre") {
			b.WriteString(preTag().Draw(t, "preTag"))
			b.WriteString(strconv.Itoa(smallInt().Draw(t, "preNum")))
		}
		if rapid.Bool().Draw(t, "hasPost") {
			b.WriteString(".post")
			b.WriteString(strconv.Itoa(smallInt().Draw(t, "postNum")))
		}
		if rapid.Bool().Draw(t, "hasDev") {
			b.WriteString(".dev")
			b.WriteString(strconv.Itoa(smallInt().Draw(t, "devNum")))
		}
		if includeLocal && rapid.Bool().Draw(t, "hasLocal") {
			b.WriteString("+")
			b.WriteString(localLabel().Draw(t, "localLabel"))
		}

		return b.String()
	})
}

// nonlocalVersionString mirrors upstream's nonlocal_versions. Ordered
// specifiers (>=, <=, >, <) reject a local label on the right-hand side, so
// any property that puts a generated version there must use this.
func nonlocalVersionString() *rapid.Generator[string] {
	return versionString(false, 1)
}

// releaseVersionString mirrors upstream's release_versions: a final release,
// with no pre, post, dev, or local segment. Wildcard (==X.*) and ~= operands
// need this, since a suffixed operand is rejected or reinterpreted.
func releaseVersionString(minSegments int) *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		return joinInts(releaseParts(minSegments).Draw(t, "finalRelease"))
	})
}

// parsedVersion draws a generated version string and parses it.
//
// A parse failure here is a genuine defect, not a filtered-out case: every
// string these generators emit is a valid PEP 440 version by construction.
// This is why no property in this package calls rapid's equivalent of
// Hypothesis's assume() on parseability — heavy filtering would risk rapid's
// "too few valid examples" error, and there is nothing to filter.
func parsedVersion(t *rapid.T, g *rapid.Generator[string], label string) version.Version {
	raw := g.Draw(t, label)
	v, err := version.Parse(raw)
	if err != nil {
		t.Fatalf("generated version %q failed to parse: %v", raw, err)
	}
	return v
}

// mustParse parses a version string that the caller constructed and asserts it
// is valid.
func mustParse(t *rapid.T, s string) version.Version {
	v, err := version.Parse(s)
	if err != nil {
		t.Fatalf("version.Parse(%q): unexpected error %v", s, err)
	}
	return v
}

// mustSpecifiers parses a specifier string and asserts it is valid.
func mustSpecifiers(t *rapid.T, s string) version.Specifiers {
	ss, err := version.NewSpecifiers(s)
	if err != nil {
		t.Fatalf("version.NewSpecifiers(%q): unexpected error %v", s, err)
	}
	return ss
}

// newSpecifiersOrError parses a specifier string that the property expects to
// be INVALID, returning the error for the caller to assert on.
//
// This exists because mustSpecifiers treats an error as a test failure, which
// is the wrong reporting direction for a rejection property.
func newSpecifiersOrError(s string) (version.Specifiers, error) {
	return version.NewSpecifiers(s)
}

// A note on filtering, since it is the one rapid hazard worth stating once:
// Hypothesis's assume() maps to rapid's t.Skip, but rapid ERRORS OUT if a
// property generates too few valid examples, so heavy skipping turns a test
// red rather than merely reducing coverage. Where a property only holds on a
// subset of generated values, the properties in this package therefore
// `return` early -- a trivially-satisfied example -- instead of skipping.
// Nothing here filters aggressively enough to need assume() semantics.

// specifierString mirrors upstream's pep440_specifier_strings: one specifier
// clause spanning the operator shapes PEP 440 allows.
//
// Upstream's include_arbitrary option is deliberately not ported. Its own
// comment records that it skips unparsable === literals because they break the
// range algebra it tests, and this module has no range algebra — so the option
// would add a shape with no property to exercise it.
func specifierString() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		switch rapid.SampledFrom([]string{"ordered", "equality", "wildcard", "compatible"}).Draw(t, "shape") {
		case "ordered":
			// Locals are rejected on the RHS of an ordered operator.
			return orderedOp().Draw(t, "orderedOp") + nonlocalVersionString().Draw(t, "orderedOperand")
		case "equality":
			return equalityOp().Draw(t, "equalityOp") + versionString(true, 1).Draw(t, "equalityOperand")
		case "wildcard":
			return equalityOp().Draw(t, "wildcardOp") + releaseVersionString(1).Draw(t, "wildcardOperand") + ".*"
		default: // compatible
			return "~=" + releaseVersionString(2).Draw(t, "compatibleOperand")
		}
	})
}

// specifierSetString mirrors upstream's rich_specifier_sets: 1 to 3 clauses
// joined by commas.
func specifierSetString() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		return strings.Join(rapid.SliceOfN(specifierString(), 1, 3).Draw(t, "clauses"), ",")
	})
}
