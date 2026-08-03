// SPDX-License-Identifier: Apache-2.0 OR MIT

// Round-trip and idempotence properties specific to this module's own API
// contracts. These have no pypa/packaging counterpart: they assert promises
// this module's doc comments make, which nothing else tests.

package proptest

import (
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/posit-dev/go-python-packaging/extras"
	"github.com/posit-dev/go-python-packaging/marker"
	"github.com/posit-dev/go-python-packaging/requirement"
)

// requirementString generates a PEP 508 requirement.
//
// Every requirement is built by rendering a string and handing it to Parse. A
// hand-built Requirement is not an option: its URL and Specifiers fields are
// both exported, but String() selects between them in a switch
// (requirement/requirement.go:113-119), so a value with both set silently
// drops the Specifiers. Parse never produces that state -- the PEP 508 grammar
// makes the two mutually exclusive -- so generating through Parse keeps the
// generator inside the space of values the type actually inhabits.
func requirementString() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		var b strings.Builder

		b.WriteString(rapid.SampledFrom([]string{
			"requests", "flask", "Django", "numpy", "pytest-cov",
			"zope.interface", "ruamel.yaml", "a", "A.B-C_D",
		}).Draw(t, "name"))

		if rapid.Bool().Draw(t, "hasExtras") {
			ex := rapid.SliceOfN(
				rapid.SampledFrom([]string{"security", "tests", "dev", "all", "socks"}),
				1, 3,
			).Draw(t, "extras")
			b.WriteString("[" + strings.Join(ex, ",") + "]")
		}

		// URL and specifiers are mutually exclusive in the grammar.
		hasURL := rapid.Bool().Draw(t, "hasURL")
		if hasURL {
			b.WriteString(" @ ")
			b.WriteString(rapid.SampledFrom([]string{
				"https://example.com/pkg-1.0.tar.gz",
				"file:///tmp/pkg-1.0-py3-none-any.whl",
				"git+https://github.com/example/pkg@main",
			}).Draw(t, "url"))
		} else if rapid.Bool().Draw(t, "hasSpecifiers") {
			b.WriteString(specifierSetString().Draw(t, "specifiers"))
		}

		if rapid.Bool().Draw(t, "hasMarker") {
			// The separator is asymmetric, and this generator has to respect
			// it or it emits strings the grammar rejects: a URL must be
			// followed by " ; " because a bare ";" could be part of the URL,
			// while without a URL the separator is "; ". This mirrors
			// requirement.String() and is the rule pypa/packaging uses.
			if hasURL {
				b.WriteString(" ; ")
			} else {
				b.WriteString("; ")
			}
			b.WriteString(markerString().Draw(t, "marker"))
		}

		return b.String()
	})
}

// markerString generates a PEP 508 environment marker expression.
func markerString() *rapid.Generator[string] {
	atom := rapid.Custom(func(t *rapid.T) string {
		variable := rapid.SampledFrom([]string{
			"python_version", "sys_platform", "os_name", "platform_machine",
			"implementation_name", "extra",
		}).Draw(t, "markerVar")
		op := rapid.SampledFrom([]string{"==", "!=", ">=", "<=", ">", "<", "in", "not in"}).Draw(t, "markerOp")
		value := rapid.SampledFrom([]string{
			"3.12", "linux", "posix", "x86_64", "cpython", "win32", "darwin", "test",
		}).Draw(t, "markerValue")

		return variable + " " + op + " \"" + value + "\""
	})

	return rapid.Custom(func(t *rapid.T) string {
		atoms := rapid.SliceOfN(atom, 1, 3).Draw(t, "markerAtoms")
		if len(atoms) == 1 {
			return atoms[0]
		}
		joiner := rapid.SampledFrom([]string{" and ", " or "}).Draw(t, "markerJoiner")
		return strings.Join(atoms, joiner)
	})
}

// TestRequirementStringRoundTrip checks the promise made verbatim at
// requirement/requirement.go:101-102 -- "Parse(r.String()) yields an
// equivalent Requirement" -- which nothing tested before.
//
// Equivalence is asserted as String() equality, deliberately, and NOT with
// reflect.DeepEqual. DeepEqual reports two Requirements unequal even when
// String() is byte-identical, and even when parsing the same input twice: the
// specifier struct stores its operator as a func field
// (version/specifier.go:100, type operatorFunc at :91), and DeepEqual reports
// any two non-nil funcs unequal. That is a property of reflection, not a defect
// in the round-trip, and it cannot be fixed by canonicalizing the retained
// original string.
func TestRequirementStringRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		raw := requirementString().Draw(t, "requirement")

		r, err := requirement.Parse(raw)
		if err != nil {
			t.Fatalf("generated requirement %q failed to parse: %v", raw, err)
		}

		rendered := r.String()

		again, err := requirement.Parse(rendered)
		if err != nil {
			t.Fatalf("Parse(%q.String()) = Parse(%q) failed: %v", raw, rendered, err)
		}

		if got := again.String(); got != rendered {
			t.Fatalf("round-trip not stable: %q -> %q -> %q", raw, rendered, got)
		}

		// Field-level equivalence for everything that is comparable without
		// reflect.DeepEqual.
		if again.Name != r.Name {
			t.Fatalf("Name changed across round-trip: %q -> %q", r.Name, again.Name)
		}
		if again.URL != r.URL {
			t.Fatalf("URL changed across round-trip: %q -> %q", r.URL, again.URL)
		}
		if strings.Join(again.Extras, ",") != strings.Join(r.Extras, ",") {
			t.Fatalf("Extras changed across round-trip: %v -> %v", r.Extras, again.Extras)
		}
		if again.Specifiers.String() != r.Specifiers.String() {
			t.Fatalf("Specifiers changed across round-trip: %q -> %q",
				r.Specifiers, again.Specifiers)
		}
		if again.Marker.String() != r.Marker.String() {
			t.Fatalf("Marker changed across round-trip: %q -> %q", r.Marker, again.Marker)
		}
	})
}

func TestMarkerStringRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		raw := markerString().Draw(t, "marker")

		m, err := marker.Parse(raw)
		if err != nil {
			t.Fatalf("generated marker %q failed to parse: %v", raw, err)
		}

		rendered := m.String()

		again, err := marker.Parse(rendered)
		if err != nil {
			t.Fatalf("Parse(%q.String()) = Parse(%q) failed: %v", raw, rendered, err)
		}

		if got := again.String(); got != rendered {
			t.Fatalf("marker round-trip not stable: %q -> %q -> %q", raw, rendered, got)
		}

		// A parsed non-empty marker must not report itself empty, or a
		// requirement carrying it would render without it.
		if m.IsEmpty() {
			t.Fatalf("marker %q parsed to an empty Marker", raw)
		}
	})
}

func TestExtrasNormalizeIsIdempotent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// PEP 685 normalization: runs of -, _ and . collapse to a single -,
		// and the name is lowercased. Draw raw names spanning those forms
		// rather than only well-formed ones.
		name := rapid.Custom(func(t *rapid.T) string {
			parts := rapid.SliceOfN(
				rapid.SampledFrom([]string{"Extra", "TESTS", "dev", "a", "Foo9", "bar"}),
				1, 4,
			).Draw(t, "nameParts")
			sep := rapid.SampledFrom([]string{"-", "_", ".", "--", "__", "._-"}).Draw(t, "nameSep")
			return strings.Join(parts, sep)
		}).Draw(t, "extraName")

		once := extras.Normalize(name)
		twice := extras.Normalize(once)

		if once != twice {
			t.Fatalf("Normalize is not idempotent: %q -> %q -> %q", name, once, twice)
		}

		// A normalized name is already in normal form, so it must be
		// lowercase and free of the separators normalization collapses.
		if once != strings.ToLower(once) {
			t.Fatalf("Normalize(%q) = %q is not lowercase", name, once)
		}
		for _, bad := range []string{"_", "."} {
			if strings.Contains(once, bad) {
				t.Fatalf("Normalize(%q) = %q still contains %q", name, once, bad)
			}
		}
	})
}

// TestSpecifiersStringRoundTrip ports upstream's
// test_specifier_set_str_round_trip_general.
func TestSpecifiersStringRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		raw := specifierSetString().Draw(t, "specifierSet")

		ss := mustSpecifiers(t, raw)
		rendered := ss.String()

		again := mustSpecifiers(t, rendered)

		if got := again.String(); got != rendered {
			t.Fatalf("specifier round-trip not stable: %q -> %q -> %q", raw, rendered, got)
		}

		// Behavioral equivalence: the re-parsed set must accept exactly the
		// same versions. String() equality alone would not catch a renderer
		// that dropped a clause it could also not re-read.
		for range 8 {
			v := parsedVersion(t, versionString(false, 1), "candidate")
			if ss.Check(v) != again.Check(v) {
				t.Fatalf("round-tripped specifier %q disagrees with %q on %q",
					rendered, raw, v)
			}
		}
	})
}
