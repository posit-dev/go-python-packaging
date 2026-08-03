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
