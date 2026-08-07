// SPDX-License-Identifier: Apache-2.0 OR MIT

package version

import (
	"fmt"
	"strings"
	"testing"
)

// A zero-value Version is reachable by accident all over Go -- an unset struct
// field, a map miss, a slice grown with make -- so no method on it may panic.
// Eight of the thirteen exported methods used to: String, BaseVersion and
// Public indexed release[0] on a nil slice, and all six comparison methods
// inherited it because Compare calls String as its fast path.
//
// This table is deliberately exhaustive over the exported surface rather than
// testing only the methods known to have been broken, because the failure mode
// is "somebody adds a method that indexes release[0]" and a targeted test would
// not catch that.
func TestZeroValueMethodsDoNotPanic(t *testing.T) {
	real := mustParse(t, "1.2.3")

	cases := []struct {
		name string
		call func()
	}{
		{"String", func() { _ = Version{}.String() }},
		{"BaseVersion", func() { _ = Version{}.BaseVersion() }},
		{"Original", func() { _ = Version{}.Original() }},
		{"Local", func() { _ = Version{}.Local() }},
		{"Public", func() { _ = Version{}.Public() }},
		{"IsPreRelease", func() { _ = Version{}.IsPreRelease() }},
		{"IsPostRelease", func() { _ = Version{}.IsPostRelease() }},
		{"Compare(zero)", func() { _ = Version{}.Compare(Version{}) }},
		{"Equal(zero)", func() { _ = Version{}.Equal(Version{}) }},
		{"GreaterThan(zero)", func() { _ = Version{}.GreaterThan(Version{}) }},
		{"GreaterThanOrEqual(zero)", func() { _ = Version{}.GreaterThanOrEqual(Version{}) }},
		{"LessThan(zero)", func() { _ = Version{}.LessThan(Version{}) }},
		{"LessThanOrEqual(zero)", func() { _ = Version{}.LessThanOrEqual(Version{}) }},

		// ⚠️ Both directions. The original defect was asymmetric: the panic
		// came from go-version's Parts.IsAny ranging over nil Part interfaces
		// on the *argument* side, so real.Compare(Version{}) crashed while
		// Version{}.Compare(real) returned an answer. Testing one direction
		// would have missed it.
		{"zero.Compare(real)", func() { _ = Version{}.Compare(real) }},
		{"real.Compare(zero)", func() { _ = real.Compare(Version{}) }},
		{"zero.Equal(real)", func() { _ = Version{}.Equal(real) }},
		{"real.Equal(zero)", func() { _ = real.Equal(Version{}) }},
		{"real.LessThan(zero)", func() { _ = real.LessThan(Version{}) }},
		{"real.GreaterThan(zero)", func() { _ = real.GreaterThan(Version{}) }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panicked: %v", r)
				}
			}()
			c.call()
		})
	}
}

// The empty string is only a safe rendering for an uninitialized Version if no
// version Parse accepts can render as "". The grammar requires a release
// segment, so it cannot -- but that is the property the choice rests on, so it
// is asserted rather than assumed.
func TestNoParsedVersionRendersEmpty(t *testing.T) {
	for _, s := range []string{
		"0", "0.0", "1", "1.0", "0!0", "1.0a1", "1.0.dev0", "1.0+local",
		"0.0.0.0.0", "1!0.0", "  1.0  ", "v1.0", "1.0.post0",
	} {
		v, err := Parse(s)
		if err != nil {
			t.Errorf("Parse(%q): unexpected error %v", s, err)
			continue
		}
		if v.String() == "" {
			t.Errorf("Parse(%q).String() == \"\", which collides with the "+
				"uninitialized-Version rendering", s)
		}
		if v.BaseVersion() == "" {
			t.Errorf("Parse(%q).BaseVersion() == \"\", same collision", s)
		}
	}
}

// A zero Version sorts below every real version, and two of them are equal.
func TestZeroValueOrdering(t *testing.T) {
	real := mustParse(t, "0")

	if got := (Version{}).Compare(Version{}); got != 0 {
		t.Errorf("zero.Compare(zero) = %d, want 0", got)
	}
	if got := (Version{}).Compare(real); got != -1 {
		t.Errorf("zero.Compare(0) = %d, want -1", got)
	}
	if got := real.Compare(Version{}); got != 1 {
		t.Errorf("0.Compare(zero) = %d, want 1", got)
	}
	if !(Version{}).Equal(Version{}) {
		t.Error("two zero Versions should be Equal")
	}
	if (Version{}).Equal(real) {
		t.Error("a zero Version should not Equal 0")
	}
}

// ⚠️ A panicking String method is worse than an ordinary panic, because fmt
// recovers it and substitutes a "%!s(PANIC=...)" placeholder. Before the fix,
// fmt.Errorf("%s", Version{}) produced an error whose message contained that
// placeholder and reported no failure -- so the bug was invisible at exactly
// the call sites most likely to hit it, while a direct .String() crashed.
// This pins that a formatted zero Version is now clean.
func TestZeroValueFormattingHasNoPanicPlaceholder(t *testing.T) {
	msg := fmt.Sprintf("version=%s base=%s", Version{}, Version{}.BaseVersion())
	if strings.Contains(msg, "PANIC") {
		t.Errorf("formatting a zero Version still embeds a recovered panic: %q", msg)
	}
	if msg != "version= base=" {
		t.Errorf("formatted zero Version = %q, want %q", msg, "version= base=")
	}
}

// Release segments of differing lengths must compare as though the shorter were
// zero-padded, in both directions. Compare pads both sides to the longer of the
// two; it previously padded the second to its own length, which is a no-op.
// key.compare tolerated the resulting mismatch for parsed versions, so this was
// latent -- these cases passed before the fix and must keep passing after it.
func TestCompareUnequalReleaseLengths(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.2", "1.2.3", -1},
		{"1.2.3", "1.2", 1},
		{"1", "1.0.0", 0},
		{"1.0.0", "1", 0},
		{"1.0", "1.0.0.0", 0},
		{"2", "1.9.9.9", 1},
		{"1.9.9.9", "2", -1},
		{"1.2.3.4.5", "1.2", 1},
		{"1.2", "1.2.3.4.5", -1},
	}

	for _, c := range cases {
		a := mustParse(t, c.a)
		b := mustParse(t, c.b)
		if got := a.Compare(b); got != c.want {
			t.Errorf("%s.Compare(%s) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func mustParse(t *testing.T, s string) Version {
	t.Helper()
	v, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return v
}
