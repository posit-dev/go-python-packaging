// SPDX-License-Identifier: Apache-2.0 OR MIT

package version

import (
	"strings"
	"testing"
)

func identitySanitizer(s string) string { return s }

// Two constraints written with no comma between them are rejected.
//
// validConstraintRegexp made the separator optional, so adjacency validated and
// then re-rendered WITH a comma the input never contained. Combined with the
// deliberately-admitted empty operator (a bare version is a valid constraint),
// that turned "==0.1dev10.3" into "==0.1dev10" plus a bare-version constraint
// "3" and rendered "==0.1dev10,3" -- a fabricated constraint boundary, the same
// defect class as the tokenizer fix in #22 reached by a different route.
//
// Every case here was verified REJECTED by pypa/packaging 26.2.
func TestAdjacentConstraintsRejected(t *testing.T) {
	for _, spec := range []string{
		">=1.0<2.0",
		">= 7 < 10",
		">= 1.0 != 1.3.4.* < 2.0",
		">= 3.5.0 >= 4.0.0",
		"> 1.0 2.0", // operator form followed by a bare version

		// Drawn from the production PyPI snapshot. Each is a single operand whose
		// version match is a PROPER PREFIX, leaving a valid second token behind:
		// the fabricated comma appears between the two.
		"==0.1dev10.3",     // -> "==0.1dev10" + "3"
		">=9.0.0a1.0",      // -> ">=9.0.0a1"  + "0"
		"==1.1b02.1",       // -> "==1.1b02"   + "1"
		">=0.2.1dev4.dev0", // -> ">=0.2.1dev4" + ".dev0"
	} {
		t.Run(spec, func(t *testing.T) {
			if _, err := NewSpecifiers(spec); err == nil {
				t.Errorf("NewSpecifiers(%q) accepted an adjacent pair with no comma", spec)
			}
			if _, err := NewRSpecifiers(spec, identitySanitizer); err == nil {
				t.Errorf("NewRSpecifiers(%q) accepted an adjacent pair with no comma", spec)
			}
		})
	}
}

// ⚠️ The two entry points DIVERGE on dash-containing input, by design, and it is
// easy to write a test that conflates them.
//
// newRSpecifiers replaces "-" with "." BEFORE validating, because that is R's
// version convention. So ">=0-12.1" is not an adjacent pair on the R path at
// all: it becomes ">=0.12.1", a single valid constraint. On the Python path the
// same string IS an adjacent pair -- PEP 440 reads "0-12" as the post-release
// "0.post12", leaving ".1" behind -- and is now rejected, which matches
// pypa/packaging 26.2.
//
// Asserting both paths reject it would have been wrong, and asserting neither
// does would have missed the Python fix.
func TestDashInputDivergesBetweenPythonAndRPaths(t *testing.T) {
	const spec = ">=0-12.1"

	if _, err := NewSpecifiers(spec); err == nil {
		t.Errorf("NewSpecifiers(%q) should reject: PEP 440 reads it as "+
			"\">=0.post12\" plus a bare \"1\"", spec)
	}
	if _, err := NewRSpecifiers(spec, identitySanitizer); err != nil {
		t.Errorf("NewRSpecifiers(%q) should accept: the dash becomes a dot before "+
			"validation, making it the single constraint \">=0.12.1\": %v", spec, err)
	}
}

// The narrowing must not reach anything else. These are the forms measured as
// load-bearing: real CRAN constraint syntax, PPM's R inputs, and the two
// leniencies deliberately left in place.
func TestCommaRequirementDoesNotNarrowValidForms(t *testing.T) {
	cases := []struct{ spec, why string }{
		{">= 3.5.0", "the overwhelmingly common CRAN form (99.8% of real occurrences)"},
		{">= 1.2-18", "R dash form"},
		{">= 1.1-18-1", "real CRAN two-dash form"},
		{">= 2016-12-12", "real CRAN date-shaped version"},
		{"1.2-18", "bare R version"},
		{"3", "bare version: PPM's ?version= passes one, tracked in #18634"},
		{">= 1.0, != 1.3.4.*, < 2.0", "PEP 440's own comma-separated example"},
		{"!=3.2-2, >3.0.0", "PPM's R multi-constraint test shape"},
		{"> 2.0.0, < 3.0.0, != 2.4.2", "three comma-separated constraints"},
		{">= 1.0,", "trailing comma, which upstream also accepts"},
		// Upstream drops every blank comma-split item, not just a trailing one:
		// `[s.strip() for s in specifiers.split(",") if s.strip()]` in
		// SpecifierSet.__init__ (pypa/packaging @ 4eb0753, release 26.2).
		// Verified: SpecifierSet(",>=1") and SpecifierSet(">=1,,<2") both parse,
		// to lengths 1 and 2.
		{",>= 1.0", "leading comma, which upstream also accepts"},
		{",,>= 1.0", "doubled leading comma"},
		{">= 1.0,, < 2.0", "doubled comma between constraints"},
		{">= 1.0 , , < 2.0", "doubled comma with surrounding spaces"},
		{",", "comma-only input: the universal set, as SpecifierSet(\",\") is"},
		{"*", "the any-version shorthand"},
		{"===lolwat", "arbitrary equality takes an opaque operand"},
		{">=1.0 , <2.0", "spaces around the comma"},
		{">=2.3 || < 3", "OR groups"},
	}
	for _, c := range cases {
		t.Run(c.spec, func(t *testing.T) {
			if _, err := NewRSpecifiers(c.spec, identitySanitizer); err != nil {
				t.Errorf("NewRSpecifiers(%q) rejected a valid form (%s): %v", c.spec, c.why, err)
			}
		})
	}
}

// ⚠️ The empty operator stays admissible. It is what makes a bare version a
// valid constraint, PPM depends on it, and narrowing it is separately tracked in
// rstudio/package-manager#18634. A comma requirement must not remove it by
// accident -- this pins the two changes apart.
func TestBareVersionStillAcceptedAfterCommaRequirement(t *testing.T) {
	for _, spec := range []string{"2.0", "3", "1.2.3", "1!2.0", "1.0+local"} {
		if _, err := NewSpecifiers(spec); err != nil {
			t.Errorf("NewSpecifiers(%q) rejected a bare version: %v", spec, err)
		}
	}
}

// No fabricated comma survives in rendered output. This is the actual harm the
// change prevents, so it is asserted on the RENDERING rather than only on
// accept/reject: a constraint set must never render a comma its input lacked.
func TestNoFabricatedCommaInRenderedOutput(t *testing.T) {
	// Inputs with no comma must either be rejected or render without one.
	for _, spec := range []string{">=1.26", "1.0", ">= 3.5.0", "===lolwat"} {
		specs, err := NewSpecifiers(spec)
		if err != nil {
			t.Fatalf("NewSpecifiers(%q): unexpected error %v", spec, err)
		}
		if got := specs.String(); strings.Contains(got, ",") {
			t.Errorf("NewSpecifiers(%q).String() = %q, which invents a comma", spec, got)
		}
	}
}
