// SPDX-License-Identifier: Apache-2.0 OR MIT

package requirement

import (
	"strings"
	"testing"
)

// PEP 508 requires a comma between version specifiers. Two specifiers written
// adjacently, with no comma, is invalid input — and accepting it was worse than
// a conformance gap, because we did not merely admit it, we RENDERED A COMMA THE
// INPUT NEVER CONTAINED.
//
// Measured on a production PyPI snapshot of 2,804,135 distinct requirement
// strings: 756 were accepted and re-rendered with a fabricated comma, e.g.
// "urllib3 (>=1.26<2.0)" became "urllib3>=1.26,<2.0". Fabricating a constraint
// boundary is a wrong answer delivered confidently — the publisher's intent is
// unknowable from malformed input, and guessing it silently changes which
// versions resolve.
//
// Expected verdicts below are the MEASURED behaviour of pypa/packaging 26.2,
// which rejects every one of these.
func TestAdjacentSpecifiersRequireAComma(t *testing.T) {
	for _, tc := range []struct {
		in     string
		reason string
	}{
		{"azure-identity (<1.5.0>=1.2.0)", "parenthesized, two specifiers, no comma"},
		{"azure-identity<1.5.0>=1.2.0", "unparenthesized, two specifiers, no comma"},
		{"foo (>=1.0<2.0)", "the shape seen most often in the real corpus"},
		{"urllib3 (>=1.26<2.0)", "a real example from the snapshot"},
		{"boto3 (>=1.23.10<1.24.0)", "a real example from the snapshot"},
		{"textual>=0.2.1<1", "a real example, unparenthesized"},
		{"locate (>=1.0==1.*)", "a real example mixing operators"},
		{"foo >=1.0 <2.0", "whitespace is not a separator either"},
	} {
		if _, err := Parse(tc.in); err == nil {
			t.Errorf("Parse(%q) was accepted; PEP 508 requires a comma between specifiers "+
				"and upstream rejects it (%s)", tc.in, tc.reason)
		}
	}
}

// TestCommaSeparatedSpecifiersStillParse is the other direction, and it is not a
// formality: the fix narrows a character class in the tokenizer, so the risk is
// rejecting valid input rather than accepting invalid input.
func TestCommaSeparatedSpecifiersStillParse(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"azure-identity (<1.5.0,>=1.2.0)", "azure-identity<1.5.0,>=1.2.0"},
		{"foo >=1.0,<2.0", "foo>=1.0,<2.0"},
		{"foo (>=1.0)", "foo>=1.0"},
		{"foo", "foo"},
		{"foo == 1.0", "foo==1.0"},
		{"foo ~= 1.4.5", "foo~=1.4.5"},
		{"foo != 1.0", "foo!=1.0"},
		{"foo == 1.0.*", "foo==1.0.*"},
		// An epoch contains "!", which the narrowed class must still admit.
		{"foo ==1!2.0", "foo==1!2.0"},
		{"foo >=1!2.0,<1!3.0", "foo>=1!2.0,<1!3.0"},
		// A local version contains "+" and "-", both of which the narrowed class
		// must still admit.
		//
		// ⚠️ Rendered VERBATIM, not normalized to "ubuntu.1". A bare Version does
		// normalize its local label, but a specifier's operand does not — measured
		// against pypa/packaging 26.2, where SpecifierSet("==1.0+ubuntu-1") renders
		// unchanged while Version("1.0+ubuntu-1") becomes "1.0+ubuntu.1". My first
		// draft of this case asserted the normalized form and was simply wrong
		// about the reference.
		{"foo ==1.0+ubuntu-1", "foo==1.0+ubuntu-1"},
	} {
		r, err := Parse(tc.in)
		if err != nil {
			t.Errorf("Parse(%q): unexpected error %v", tc.in, err)
			continue
		}
		if got := r.String(); got != tc.want {
			t.Errorf("Parse(%q).String() = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ⚠️ TWO SEPARATE DIVERGENCES FROM THE REFERENCE WERE FOUND WHILE WRITING THESE
// TESTS AND ARE DELIBERATELY NOT FIXED HERE. Both predate this change — each was
// confirmed to behave identically on unmodified main — and each needs a decision
// this change should not make on its own.
//
//  1. An uppercase or underscored local label in a SPECIFIER is rejected:
//     "foo ==1.0+UBUNTU_1" errors with "improper constraint", while
//     pypa/packaging accepts it and renders it verbatim. The version regex behind
//     validConstraintRegexp is stricter than PEP 440's raw local-label grammar.
//
//  2. The comma between constraints is OPTIONAL in validConstraintRegexp
//     (a trailing `\,?` inside a repeated group), and the operator alternation
//     admits an empty operator so a bare version is a valid constraint. Together
//     those let adjacent constraints run together: "==0.1dev10.3" parses as
//     "==0.1dev10" followed by a bare-version constraint "3", and renders
//     "==0.1dev10,3" — the same fabricated-comma failure as above, arriving by a
//     different route. It accounts for the 20 cases that remain after this fix,
//     down from 756.
//
// The second is the one to be careful with: that regex is shared with
// NewRSpecifiers, which Package Manager uses for R constraint parsing, so
// requiring a comma there needs to be measured against R data rather than
// assumed safe from the Python side.

// TestArbitraryEqualityKeepsItsPermissiveOperand guards the one operator that
// must NOT be narrowed.
//
// PEP 440 defines "===" as an escape hatch comparing an opaque string that is
// never interpreted as a version, so its operand may legitimately contain the
// characters excluded for every other operator. Narrowing it along with the rest
// would have been an easy and invisible mistake — the corpus has few such
// requirements, so nothing else here would have caught it.
func TestArbitraryEqualityKeepsItsPermissiveOperand(t *testing.T) {
	for _, in := range []string{
		"foo ===lolwat",
		"foo ===1.0>=2.0",
		"foo ===<>=~",
		"foo (===anything-at-all)",
	} {
		r, err := Parse(in)
		if err != nil {
			t.Errorf("Parse(%q): arbitrary equality takes an opaque operand, got error %v", in, err)
			continue
		}
		if !strings.Contains(r.String(), "===") {
			t.Errorf("Parse(%q).String() = %q, expected it to retain ===", in, r.String())
		}
	}
}
