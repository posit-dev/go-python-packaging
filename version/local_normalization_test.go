// SPDX-License-Identifier: Apache-2.0 OR MIT

package version

import "testing"

// Local-label normalization, per PEP 440: "-" and "_" separators inside a
// local version label MUST be normalized to ".".
//
// Reference behavior measured against pypa/packaging at pinned SHA
// 4eb0753dba8fcaaac8eb75463374e448f0931558 (reporting itself as 26.3.dev0),
// read from source rather than a pip-installed copy -- the released 26.2
// differs from the pinned tree in unrelated ways.
//
// Found by the property tests in internal/proptest.

func TestLocalLabelSeparatorsNormalize(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"1.0+abc.abc", "1.0+abc.abc"},
		{"1.0+abc-abc", "1.0+abc.abc"},
		{"1.0+abc_abc", "1.0+abc.abc"},
		{"1.0+abc-def.ghi_jkl", "1.0+abc.def.ghi.jkl"},
		{"1.0+ubuntu-1", "1.0+ubuntu.1"},
		// Case is lowered as well, and the two rules compose.
		{"1.0+Ubuntu-1", "1.0+ubuntu.1"},
		{"1.0+ABC_DEF", "1.0+abc.def"},
	} {
		v, err := Parse(tc.in)
		if err != nil {
			t.Errorf("Parse(%q): unexpected error %v", tc.in, err)
			continue
		}
		if got := v.String(); got != tc.want {
			t.Errorf("Parse(%q).String() = %q, want %q", tc.in, got, tc.want)
		}
		if got := v.Local(); got != tc.want[len("1.0+"):] {
			t.Errorf("Parse(%q).Local() = %q, want %q", tc.in, got, tc.want[len("1.0+"):])
		}
	}
}

func TestLocalLabelSeparatorFormsAreEqual(t *testing.T) {
	for _, tc := range [][2]string{
		{"1.0+abc.abc", "1.0+abc-abc"},
		{"1.0+abc.abc", "1.0+abc_abc"},
		{"1.0+a.b.c", "1.0+a-b_c"},
		{"1.0+ubuntu.1", "1.0+ubuntu-1"},
	} {
		a, err := Parse(tc[0])
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc[0], err)
		}
		b, err := Parse(tc[1])
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc[1], err)
		}
		if !a.Equal(b) {
			t.Errorf("%q and %q should be equal, got %q vs %q", tc[0], tc[1], a, b)
		}
	}
}

// TestLocalLabelSeparatorOrdering is the regression test for the silent wrong
// answer the un-normalized label produced.
//
// cmpkey splits a local label on "." to compare it segment by segment, with a
// numeric segment ordering above an alphabetic one. Without separator
// normalization, "ubuntu-2" is one alphabetic segment rather than two, so the
// comparison falls back to lexicographic ordering of the whole label and
// "ubuntu-10" sorts BELOW "ubuntu-2".
func TestLocalLabelSeparatorOrdering(t *testing.T) {
	for _, tc := range []struct {
		lower, higher string
	}{
		// The dotted form was always correct; the dash and underscore forms
		// were not, and must now agree with it.
		{"1.0+ubuntu.2", "1.0+ubuntu.10"},
		{"1.0+ubuntu-2", "1.0+ubuntu-10"},
		{"1.0+ubuntu_2", "1.0+ubuntu_10"},
		{"1.0+abc-1", "1.0+abc-2"},
		{"1.0+deb-9", "1.0+deb-11"},
	} {
		lo, err := Parse(tc.lower)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc.lower, err)
		}
		hi, err := Parse(tc.higher)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc.higher, err)
		}
		if !lo.LessThan(hi) {
			t.Errorf("expected %q < %q (numeric local segment ordering)", tc.lower, tc.higher)
		}
	}
}

// TestSurroundingWhitespaceIncludesVerticalTab covers the second divergence
// the property tests surfaced. Go's \s is [\t\n\f\r ] and omits the vertical
// tab; Python's re \s is [ \t\n\r\f\v] and includes it, so pypa/packaging
// accepts "1.0\v" where a bare \s* anchor would reject it.
func TestSurroundingWhitespaceIncludesVerticalTab(t *testing.T) {
	for _, ws := range []string{" ", "\t", "\n", "\r", "\f", "\v"} {
		for _, in := range []string{ws + "1.0", "1.0" + ws, ws + "1.0" + ws} {
			v, err := Parse(in)
			if err != nil {
				t.Errorf("Parse(%q): unexpected error %v", in, err)
				continue
			}
			if got := v.String(); got != "1.0" {
				t.Errorf("Parse(%q).String() = %q, want %q", in, got, "1.0")
			}
		}
	}
}

// TestAllDigitLocalSegmentsNormalizeAsIntegers covers a third divergence, found
// by a differential run over 634,187 distinct real PyPI version strings — which
// is worth stating plainly, because that corpus could not possibly have found
// it: PEP 440 forbids PyPI from accepting local versions at all, so the corpus
// contained ZERO of them and this came out of the synthetic supplement.
//
// PEP 440: "If a segment consists entirely of ASCII digits then that section
// should be considered an integer", and its integer-normalization rule ("an
// integer version of 00 would normalize to 0") therefore reaches such a
// segment. The rule's carve-out is scoped to integers inside an ALPHANUMERIC
// segment, "such as 1.0+foo0100 which is already in its normalized form".
//
// Expected values below are the measured output of str(packaging.version.Version(s)),
// not a reading of the spec — the spec settles which implementation is right, the
// reference settles what the answer looks like.
func TestAllDigitLocalSegmentsNormalizeAsIntegers(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"1.0+007", "1.0+7"},
		{"1.0+7", "1.0+7"},
		{"1.0+00", "1.0+0"}, // All zeros is the integer 0, not the empty string.
		{"1.0+0", "1.0+0"},
		{"1.0+ubuntu.007", "1.0+ubuntu.7"},
		{"1.0+ubuntu-007", "1.0+ubuntu.7"}, // Separator and integer normalization compose.
		{"1.0+007.ubuntu", "1.0+7.ubuntu"},
		{"1.0+foo0100", "1.0+foo0100"}, // Alphanumeric: the carve-out, left alone.
		{"1.0+0foo", "1.0+0foo"},       // Also alphanumeric, despite the leading zero.
		{"1.0+09000", "1.0+9000"},
	} {
		v, err := Parse(tc.in)
		if err != nil {
			t.Errorf("Parse(%q): unexpected error %v", tc.in, err)
			continue
		}
		if got := v.String(); got != tc.want {
			t.Errorf("Parse(%q).String() = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestAllDigitLocalNormalizationIsObservableOnlyViaArbitraryEquality pins WHY
// the normalization above matters, so a future reader does not conclude it is
// cosmetic and drop it.
//
// Ordering was already correct without it, because cmpkey compares all-digit
// segments numerically either way — so no comparison test could have caught the
// bug. PEP 440 defines "===" as string equality on the normalized form, which is
// the one place the rendering is load-bearing.
func TestAllDigitLocalNormalizationIsObservableOnlyViaArbitraryEquality(t *testing.T) {
	padded, err := Parse("1.0+007")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	bare, err := Parse("1.0+7")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// The half that was always right: they compare equal.
	if got := padded.Compare(bare); got != 0 {
		t.Errorf("Compare(1.0+007, 1.0+7) = %d, want 0 — all-digit segments compare "+
			"numerically, which was never the defect", got)
	}

	// The half that was wrong: "===" is string equality, so the strings must match.
	if padded.String() != bare.String() {
		t.Errorf("String() forms differ (%q vs %q), so \"===1.0+7\" would fail to match "+
			"\"1.0+007\" while the reference implementation matches it",
			padded.String(), bare.String())
	}

	spec, err := NewSpecifiers("===1.0+7")
	if err != nil {
		t.Fatalf("NewSpecifiers: %v", err)
	}
	if !spec.Check(padded) {
		t.Error("\"===1.0+7\" must match 1.0+007: PEP 440 defines === as string equality " +
			"on the normalized form, and 007 normalizes to 7")
	}
}
