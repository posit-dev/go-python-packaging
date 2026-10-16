// SPDX-License-Identifier: Apache-2.0 OR MIT

package canonname

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNormalize(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty string", "", ""},
		{"single character", "x", "x"},
		{"already normal", "already-normal", "already-normal"},
		{"single hyphen", "-", "-"},
		{"uppercase", "Django", "django"},
		{"all uppercase", "PYYAML", "pyyaml"},
		{"mixed case with underscore", "Foo_Bar", "foo-bar"},
		{"dot", "zope.interface", "zope-interface"},
		{"dot and double underscore", "a.b__c", "a-b-c"},
		{"repeated hyphen", "a--b", "a-b"},
		{"uppercase with repeated hyphen", "A---B", "a-b"},
		{"mixed run of separators", "foo_.-bar", "foo-bar"},
		{"uppercase with dots", "Foo.Bar.Baz", "foo-bar-baz"},
		{"leading separators", "__foo", "-foo"},
		{"trailing separators", "foo__", "foo-"},
		{"leading and trailing separators", "..foo..", "-foo-"},
		{"only separators", "___", "-"},
		{"digits unchanged", "Py3_K", "py3-k"},
		{"ASCII bounds around letters", "@AZ[`az{", "@az[`az{"},
		{"whitespace unchanged", "hi\n", "hi\n"},
		{"non-ASCII already normal", "café-au-lait", "café-au-lait"},
		{"multi-byte runes around separators", "日本__語", "日本-語"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Normalize(c.in); got != c.want {
				t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// Expected values were produced by running pypa/packaging's canonicalize_name
// (pinned 4eb0753dba8fcaaac8eb75463374e448f0931558) under CPython 3.13
// (Unicode 15.1).
func TestNormalizeMatchesPythonLower(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"Latin-1 uppercase", "ÄBC_def", "äbc-def"},
		{"accented uppercase", "ÀÉÎ", "àéî"},
		{"sharp s unchanged", "Straße", "straße"},
		{"capital sharp s", "ẞ", "ß"},
		{"dotted capital I expands", "İstanbul", "i\u0307stanbul"},
		{"Kelvin sign becomes ASCII k", "\u212Aelvin", "kelvin"},
		{"Angstrom sign", "\u212B", "å"},
		{"titlecase digraph", "ǅ", "ǆ"},
		{"fullwidth", "ＦＵＬＬ", "ｆｕｌｌ"},
		{"Roman numeral", "Ⅻ", "ⅻ"},
		{"Odysseus", "ΟΔΥΣΣΕΥΣ", "οδυσσευς"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Normalize(c.in); got != c.want {
				t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// Σ lowercases to final ς only at the end of a word. Python lowercases
// before replacing separators, so the context is the original name: "_" and
// "-" end a word, but "." and ":" are case-ignorable and do not. Expected
// values were produced the same way as in TestNormalizeMatchesPythonLower.
func TestNormalizeFinalSigma(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Σ", "σ"},
		{"1Σ", "1σ"},
		{"ΑΣ", "ας"},
		{"ΣΑΣ", "σας"},
		{"ΑΣΒ", "ασβ"},
		{"ΑΣ_Β", "ας-β"},
		{"ΑΣ-Β", "ας-β"},
		{"ΑΣ.Β", "ασ-β"},
		{"ΑΣ:Β", "ασ:β"},
		{"Α.Σ", "α-ς"},
		{"Α_Σ", "α-σ"},
		{"ΑΣ'", "ας'"},
		{"ΑΣ\u0301", "ας\u0301"},
		{"Α\u0301Σ", "α\u0301ς"},
		{"ΑΣ\u0301Β", "ασ\u0301β"},
		{"ΣΑΣ_ΣΑΣ", "σας-σας"},
	}
	for _, c := range cases {
		if got := Normalize(c.in); got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// Python strings cannot hold invalid UTF-8, so these cases have no upstream
// counterpart; an invalid byte counts as neither cased nor case-ignorable.
func TestNormalizeFinalSigmaInvalidUTF8(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Α\xffΣ", "α\xffσ"},
		{"ΑΣ\xff", "ας\xff"},
	}
	for _, c := range cases {
		if got := Normalize(c.in); got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeInvalidUTF8PassesThrough(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"\xff", "\xff"},
		{"A\xff_B", "a\xff-b"},
		{"\xc3_", "\xc3-"},   // truncated two-byte sequence followed by a separator
		{"\xc3\x84", "ä"},    // the same bytes completed are a valid "Ä"
		{"\uFFFD", "\uFFFD"}, // a valid encoded U+FFFD is not mistaken for invalid input
	}
	for _, c := range cases {
		if got := Normalize(c.in); got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsNormalized(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", true},
		{"foo", true},
		{"foo-bar", true},
		{"-", true},
		{"-foo-", true},
		{"a-b-c", true},
		{"café", true},
		{"σας", true},
		{"i̇", true},
		{"\xff", true},
		{"�", true},
		{"Ä", false},
		{"Σ", false},
		{"İ", false},
		{"K", false}, // Kelvin sign lowercases to ASCII "k"
		{"Foo", false},
		{"fooZ", false},
		{"foo_bar", false},
		{"foo.bar", false},
		{"--", false},
		{"foo--bar", false},
		{"foo-_bar", false},
		{"_", false},
		{".", false},
	}
	for _, c := range cases {
		if got := isNormalized(c.in); got != c.want {
			t.Errorf("isNormalized(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNormalizeFastPathDoesNotAllocate(t *testing.T) {
	for _, in := range []string{"", "requests", "zope-interface", "café-au-lait"} {
		allocs := testing.AllocsPerRun(100, func() { _ = Normalize(in) })
		if allocs != 0 {
			t.Errorf("Normalize(%q) allocated %v times, want 0", in, allocs)
		}
	}
}

// pep503Separators is the separator regex from PEP 503's normalization rule.
var pep503Separators = regexp.MustCompile(`[-_.]+`)

// reference implements PEP 503's re.sub(r"[-_.]+", "-", name).lower(), the
// form pypa/packaging's canonicalize_name emulates without a regex, with
// strings.ToLower standing in for str.lower(). The two agree except on
// invalid UTF-8 (which strings.ToLower replaces with U+FFFD) and on "İ" and
// "Σ", so ok is false for those inputs; the Python-derived tables above cover
// them.
func reference(name string) (want string, ok bool) {
	if !utf8.ValidString(name) || strings.ContainsAny(name, "İΣ") {
		return "", false
	}
	return strings.ToLower(pep503Separators.ReplaceAllString(name, "-")), true
}

func FuzzNormalize(f *testing.F) {
	for _, s := range []string{
		"", "-", "foo", "Foo_Bar", "a.b__c", "A---B", "..foo..", "ÄBC_def",
		"日本__語", "\xff", "A\xff_B", "foo-_bar", "ΑΣ.Β", "İstanbul", "Kelvin",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		got := Normalize(in)
		if want, ok := reference(in); ok && got != want {
			t.Fatalf("Normalize(%q) = %q, reference = %q", in, got, want)
		}
		if again := Normalize(got); again != got {
			t.Fatalf("not idempotent: Normalize(%q) = %q, Normalize(%q) = %q", in, got, got, again)
		}
		if isNormalized(in) != (got == in) {
			t.Fatalf("isNormalized(%q) = %v, but Normalize(%q) == input is %v", in, isNormalized(in), in, got == in)
		}
		if !isNormalized(got) {
			t.Fatalf("isNormalized(Normalize(%q)) = false for %q", in, got)
		}
	})
}
