// SPDX-License-Identifier: Apache-2.0 OR MIT

// Normalization properties, ported from pypa/packaging's
// tests/property/test_version_normalization.py at pinned SHA
// 4eb0753dba8fcaaac8eb75463374e448f0931558.
//
// These are the properties that PEP 440's "Normalization" section defines: the
// many accepted spellings of one version must all parse to the same value, and
// String() must render the single normal form.

package proptest

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// padInt renders n with count extra leading zeros, e.g. padInt(7, 2) == "007".
// Upstream's padded_int strategy does the same.
func padInt(n, zeros int) string {
	return strings.Repeat("0", zeros) + strconv.Itoa(n)
}

func TestPreReleaseCaseInsensitive(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		release := releaseSegment().Draw(t, "release")
		tag := preTag().Draw(t, "preTag")
		n := smallInt().Draw(t, "preNum")

		lower := fmt.Sprintf("%s%s%d", release, tag, n)
		upper := fmt.Sprintf("%s%s%d", release, strings.ToUpper(tag), n)

		assertSameVersion(t, lower, upper)
	})
}

func TestPostAndDevCaseInsensitive(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		release := releaseSegment().Draw(t, "release")
		n := smallInt().Draw(t, "num")

		for _, base := range []string{"post", "dev"} {
			lower := fmt.Sprintf("%s.%s%d", release, base, n)
			// Upper ("POST") and title ("Post") spellings. Built by hand
			// rather than with strings.Title, which is deprecated.
			title := strings.ToUpper(base[:1]) + base[1:]
			for _, variant := range []string{strings.ToUpper(base), title} {
				assertSameVersion(t, lower, fmt.Sprintf("%s.%s%d", release, variant, n))
			}
		}
	})
}

func TestNormalFormIsLowercase(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		release := releaseSegment().Draw(t, "release")
		tag := preTag().Draw(t, "preTag")
		n := smallInt().Draw(t, "preNum")

		v := mustParse(t, fmt.Sprintf("%s%s%d", release, strings.ToUpper(tag), n))

		if got := v.String(); got != strings.ToLower(got) {
			t.Fatalf("normalized form %q is not lowercase", got)
		}
	})
}

func TestLeadingZerosStripped(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := smallInt().Draw(t, "num")
		zeros := rapid.IntRange(1, 3).Draw(t, "zeroCount")
		release := releaseSegment().Draw(t, "release")

		// Release components, and the pre/post/dev numerals, are integers:
		// leading zeros carry no meaning and must normalize away.
		cases := [][2]string{
			{fmt.Sprintf("1.%d", n), fmt.Sprintf("1.%s", padInt(n, zeros))},
			{fmt.Sprintf("%sa%d", release, n), fmt.Sprintf("%sa%s", release, padInt(n, zeros))},
			{fmt.Sprintf("%s.post%d", release, n), fmt.Sprintf("%s.post%s", release, padInt(n, zeros))},
			{fmt.Sprintf("%s.dev%d", release, n), fmt.Sprintf("%s.dev%s", release, padInt(n, zeros))},
		}

		for _, c := range cases {
			assertSameVersion(t, c[0], c[1])
		}
	})
}

func TestPreReleaseSeparatorsNormalizeAway(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		release := releaseSegment().Draw(t, "release")
		tag := preTag().Draw(t, "preTag")
		n := smallInt().Draw(t, "preNum")
		sep := rapid.SampledFrom([]string{".", "-", "_"}).Draw(t, "separator")

		canonical := fmt.Sprintf("%s%s%d", release, tag, n)

		// A separator is permitted before the tag, between the tag and the
		// numeral, or both; all normalize to the bare form.
		assertSameVersion(t, canonical, fmt.Sprintf("%s%s%s%d", release, sep, tag, n))
		assertSameVersion(t, canonical, fmt.Sprintf("%s%s%s%d", release, tag, sep, n))
		assertSameVersion(t, canonical, fmt.Sprintf("%s%s%s%s%d", release, sep, tag, sep, n))

		// And the normal form carries no separator before the tag.
		if got := mustParse(t, canonical).String(); strings.Contains(got, sep+tag) {
			t.Fatalf("normal form %q should not contain separator %q before %q", got, sep, tag)
		}
	})
}

// TestPreReleaseAlternateSpellings is the property that rstudio/package-manager#19369
// was a counterexample to: "alpha" must normalize to "a", not match nothing.
func TestPreReleaseAlternateSpellings(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		release := releaseSegment().Draw(t, "release")
		n := smallInt().Draw(t, "preNum")

		spelling := rapid.SampledFrom([][2]string{
			{"alpha", "a"},
			{"beta", "b"},
			{"c", "rc"},
			{"pre", "rc"},
			{"preview", "rc"},
		}).Draw(t, "spelling")

		long := fmt.Sprintf("%s%s%d", release, spelling[0], n)
		short := fmt.Sprintf("%s%s%d", release, spelling[1], n)

		assertSameVersion(t, short, long)

		// The normal form uses the short spelling, so the long one must be
		// absent from the rendered output.
		//
		// Guarded on the long spelling being strictly longer: "c" normalizes
		// to "rc", which *contains* "c", so a substring check is meaningless
		// for that pair. assertSameVersion above already proves that pair
		// shares a normal form.
		if len(spelling[0]) > len(spelling[1]) {
			if got := mustParse(t, long).String(); strings.Contains(got, spelling[0]) {
				t.Fatalf("normal form of %q is %q, should use short spelling %q", long, got, spelling[1])
			}
		}
	})
}

func TestPostReleaseAlternateSpellings(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		release := releaseSegment().Draw(t, "release")
		n := smallInt().Draw(t, "postNum")
		alt := rapid.SampledFrom([]string{"rev", "r"}).Draw(t, "altSpelling")

		canonical := fmt.Sprintf("%s.post%d", release, n)
		assertSameVersion(t, canonical, fmt.Sprintf("%s.%s%d", release, alt, n))

		if got := mustParse(t, fmt.Sprintf("%s.%s%d", release, alt, n)).String(); !strings.Contains(got, ".post") {
			t.Fatalf("normal form %q should render %q as .post", got, alt)
		}
	})
}

func TestOmittedNumeralEqualsZero(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		release := releaseSegment().Draw(t, "release")
		tag := preTag().Draw(t, "preTag")

		// Pre-release: an omitted numeral means 0, and the normal form
		// makes the 0 explicit.
		assertSameVersion(t, fmt.Sprintf("%s%s0", release, tag), fmt.Sprintf("%s%s", release, tag))

		// Post and dev likewise.
		assertSameVersion(t, release+".post0", release+".post")
		assertSameVersion(t, release+".dev0", release+".dev")

		for _, bare := range []string{fmt.Sprintf("%s%s", release, tag), release + ".post", release + ".dev"} {
			if got := mustParse(t, bare).String(); !strings.HasSuffix(got, "0") {
				t.Fatalf("normal form of %q is %q, should end with an explicit 0", bare, got)
			}
		}
	})
}

// TestDashNumberIsImplicitPost ports upstream's test_dash_number_is_implicit_post:
// "1.0-5" is shorthand for "1.0.post5".
func TestDashNumberIsImplicitPost(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		release := releaseSegment().Draw(t, "release")
		n := rapid.IntRange(1, 999).Draw(t, "postNum")

		assertSameVersion(t, fmt.Sprintf("%s.post%d", release, n), fmt.Sprintf("%s-%d", release, n))
	})
}

func TestVPrefixIgnored(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		release := releaseSegment().Draw(t, "release")
		tag := preTag().Draw(t, "preTag")
		n := smallInt().Draw(t, "preNum")

		for _, prefix := range []string{"v", "V"} {
			assertSameVersion(t, release, prefix+release)

			// Combined with a pre-release, per upstream's
			// test_v_prefix_with_pre_release.
			withPre := fmt.Sprintf("%s%s%d", release, tag, n)
			assertSameVersion(t, withPre, prefix+withPre)
		}

		// The normal form drops the prefix entirely.
		got := mustParse(t, "v"+release).String()
		if strings.HasPrefix(got, "v") || strings.HasPrefix(got, "V") {
			t.Fatalf("normal form %q should not carry a v prefix", got)
		}
	})
}

func TestSurroundingWhitespaceStripped(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		release := releaseSegment().Draw(t, "release")

		// Upstream draws from space, tab, newline, carriage return, form feed
		// and vertical tab. Note this is about a *version* string; a trailing
		// line break in a PEP 508 *requirement* is rejected, which is a
		// different rule tested in requirement/conformance_test.go.
		ws := rapid.SampledFrom([]string{" ", "\t", "\n", "\r", "\f", "\v"}).Draw(t, "whitespace")
		lead := rapid.IntRange(0, 3).Draw(t, "leadCount")
		trail := rapid.IntRange(0, 3).Draw(t, "trailCount")

		padded := strings.Repeat(ws, lead) + release + strings.Repeat(ws, trail)
		assertSameVersion(t, release, padded)

		if got := mustParse(t, padded).String(); got != strings.TrimSpace(got) {
			t.Fatalf("normal form %q retains surrounding whitespace", got)
		}
	})
}

func TestLocalSeparatorsNormalizeToDot(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		release := releaseSegment().Draw(t, "release")
		segments := rapid.SliceOfN(
			rapid.SampledFrom([]string{"abc", "ubuntu", "1", "42", "x9"}),
			2, 4,
		).Draw(t, "localSegments")
		sep := rapid.SampledFrom([]string{"-", "_"}).Draw(t, "localSeparator")

		dotted := release + "+" + strings.Join(segments, ".")
		separated := release + "+" + strings.Join(segments, sep)

		assertSameVersion(t, dotted, separated)

		if got := mustParse(t, separated).String(); strings.Contains(got, sep) {
			t.Fatalf("normal form %q should not retain local separator %q", got, sep)
		}
	})
}

// TestNormalizationIsIdempotent is one of this module's own properties: once
// normalized, re-parsing must be a fixed point. Without it, every other
// normalization property could hold while String() still emitted something
// that drifts on a second pass.
func TestNormalizationIsIdempotent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		v := parsedVersion(t, versionString(true, 1), "version")

		once := v.String()
		twice := mustParse(t, once).String()

		if once != twice {
			t.Fatalf("normalization is not idempotent: %q then %q", once, twice)
		}

		// And the round-trip preserves value, not just rendering.
		if !v.Equal(mustParse(t, once)) {
			t.Fatalf("Parse(%q.String()) is not equal to the original", v)
		}
	})
}

// TestLocalLabelDoesNotAffectPublicPart checks Public() and Local() agree with
// the rendered form. Upstream expresses this against the .public and .local
// attributes; these are the exported equivalents here.
func TestLocalLabelDoesNotAffectPublicPart(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		base := nonlocalVersionString().Draw(t, "base")
		label := localLabel().Draw(t, "localLabel")

		bare := mustParse(t, base)
		withLocal := mustParse(t, base+"+"+label)

		if got, want := withLocal.Public(), bare.String(); got != want {
			t.Fatalf("Public() = %q, want %q", got, want)
		}
		if withLocal.Local() == "" {
			t.Fatalf("Local() is empty for %q", withLocal)
		}
		if got := withLocal.String(); !strings.HasPrefix(got, withLocal.Public()+"+") {
			t.Fatalf("String() = %q should be Public() + \"+\" + Local()", got)
		}

		// A local label never changes the pre/post classification.
		if bare.IsPreRelease() != withLocal.IsPreRelease() {
			t.Fatalf("local label changed IsPreRelease for %q", base)
		}
		if bare.IsPostRelease() != withLocal.IsPostRelease() {
			t.Fatalf("local label changed IsPostRelease for %q", base)
		}
	})
}

// assertSameVersion asserts two spellings parse to the same value AND render
// to the same normal form. Value equality alone would be too weak: two
// spellings could compare equal while String() disagreed.
func assertSameVersion(t *rapid.T, canonical, variant string) {
	a := mustParse(t, canonical)
	b := mustParse(t, variant)

	if !a.Equal(b) {
		t.Fatalf("%q and %q should parse to equal versions, got %q vs %q",
			canonical, variant, a, b)
	}
	if a.String() != b.String() {
		t.Fatalf("%q and %q should share a normal form, got %q vs %q",
			canonical, variant, a.String(), b.String())
	}
}

// TestBaseVersionStripsSuffixes ports upstream's test_base_version_strips_suffixes.
func TestBaseVersionStripsSuffixes(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		v := parsedVersion(t, versionString(true, 1), "version")

		base := mustParse(t, v.BaseVersion())

		if base.IsPreRelease() || base.IsPostRelease() {
			t.Fatalf("BaseVersion() %q of %q still has a pre or post segment", v.BaseVersion(), v)
		}
		if base.Local() != "" {
			t.Fatalf("BaseVersion() %q of %q still has a local label", v.BaseVersion(), v)
		}
		// The base version can never exceed the version it came from.
		if base.GreaterThan(v) && !v.IsPreRelease() {
			t.Fatalf("BaseVersion() %q should not exceed %q", base, v)
		}
	})
}
