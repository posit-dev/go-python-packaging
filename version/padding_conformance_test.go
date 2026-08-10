// SPDX-License-Identifier: Apache-2.0 OR MIT

// Regression tests for the release-blocking padding and epoch defects in
// prefix matching (`==X.*`, `!=X.*`) and the compatible operator (`~=`).
//
// Both shipped in v0.4.0 and both were inherited verbatim from the retired
// rstudio/go-pep440-version:
//
//	F1  padVersion's zero-padding loops used `for i := 0; i < len(a)-len(b); i++`
//	    and APPENDED TO b INSIDE THE LOOP, so the bound shrank as the slice grew.
//	    They appended ceil(d/2) zeros for a gap of d, not d.
//	F2  versionSplit did not prepend the epoch the way upstream's _version_split
//	    does, so "1!1.0" split to ["1!1", "0"]; the digit filter then dropped the
//	    "1!1" entirely and the under-padding made the two sides coincidentally
//	    equal, so `==0.0.0.*` MATCHED `1!1.0`.
//
// ⚠️ WHY THIS WENT UNNOTICED, and what this table is shaped to catch.
//
// ceil(1/2) == 1, so a gap of ONE segment is padded correctly by accident. Every
// pre-existing regression case in this area was a 1-segment gap, which is
// exactly why a real arithmetic bug survived a rewrite, a release, and a
// changelog entry claiming the padding worked "as PEP 440 requires". The table
// below sweeps gaps of 1 through 4 in BOTH directions, with zero and non-zero
// tails, pre-release segments, and epochs.
//
// ⚠️ The customer-visible case is `!=7.0.0.*` matching `7`: a repository server
// would SERVE a version pip EXCLUDES. It reaches PPM through arbitrary client
// input on POST /__api__/repos/:id/filter/packages.
//
// Every expected value is measured against pypa/packaging 26.2, not reasoned
// out. Measured before the fix: 30 of the 82 rows diverged, on v0.4.0 and on
// this branch identically.

package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConformance_PaddingAndEpoch(t *testing.T) {
	for _, tt := range paddingOracle {
		t.Run(tt.spec+"__"+tt.version, func(t *testing.T) {
			ss, err := NewSpecifiers(tt.spec, WithPreReleases(PreReleasesInclude))
			require.NoError(t, err, "specifier %q must parse", tt.spec)
			v, err := Parse(tt.version)
			require.NoError(t, err, "version %q must parse", tt.version)
			assert.Equal(t, tt.want, ss.Check(v))

			// The singular specifier must agree with the set.
			s, err := NewSpecifier(tt.spec, WithPreReleases(PreReleasesInclude))
			require.NoError(t, err)
			assert.Equal(t, tt.want, s.ContainsVersion(v), "singular must agree with the set")
		})
	}
}

// The arithmetic itself, independent of any specifier, so a future refactor of
// leftPad is checked directly rather than only through its callers.
//
// ⚠️ A gap of 1 is the case the old shrinking loop got RIGHT, so it cannot be
// the only case tested.
// The arithmetic itself, independent of any specifier, so a future refactor of
// leftPad is checked directly rather than only through its callers.
//
// ⚠️ A gap of 1 is the case the old shrinking loop got RIGHT, so it cannot be
// the only case tested.
func TestLeftPadFillsTheWholeGap(t *testing.T) {
	for gap := 0; gap <= 5; gap++ {
		split := []string{"0", "2"}
		target := numericPrefixLen(split) + gap
		got := leftPad(split, target)
		assert.Equal(t, target, numericPrefixLen(got),
			"gap %d: leftPad must reach the target numeric width", gap)
		assert.Len(t, got, len(split)+gap, "gap %d: exactly gap zeros appended", gap)
	}

	// With a suffix present the zeros go before it and the suffix survives.
	for gap := 1; gap <= 4; gap++ {
		split := []string{"0", "2", "rc1"}
		got := leftPad(split, numericPrefixLen(split)+gap)
		assert.Equal(t, "rc1", got[len(got)-1], "gap %d: suffix must stay last", gap)
		assert.Len(t, got, len(split)+gap, "gap %d", gap)
	}
}

// The epoch must be its own leading component, which is what stops "1!1" from
// being mistaken for a non-numeric token and dropped.
// The epoch must be its own leading component, which is what stops "1!1" from
// being mistaken for a non-numeric token and dropped.
func TestVersionSplitPrependsTheEpoch(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"1.0", []string{"0", "1", "0"}},
		{"0!1.0", []string{"0", "1", "0"}},
		{"1!1.0", []string{"1", "1", "0"}},
		{"2!1", []string{"2", "1"}},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			assert.Equal(t, tt.want, versionSplit(tt.in))
		})
	}
}

// 82 rows. Expected values measured against pypa/packaging 26.2.
var paddingOracle = []struct {
	spec    string
	version string
	want    bool
}{
	{"==2.0.*", "2", true},
	{"==2.0.0.*", "2", true},
	{"!=7.0.0.*", "7", false},
	{"~=2.0.0.0", "2", true},
	{"==2.0.0.0.*", "2", true},
	{"==2.*", "2", true},
	{"!=2.*", "2", false},
	{"==2.*", "2.0", true},
	{"!=2.*", "2.0", false},
	{"==2.*", "2.0.0", true},
	{"!=2.*", "2.0.0", false},
	{"==2.*", "2.0.0.0", true},
	{"!=2.*", "2.0.0.0", false},
	{"==2.*", "2.0.0.0.0", true},
	{"!=2.*", "2.0.0.0.0", false},
	{"!=2.0.*", "2", false},
	{"==2.0.*", "2.0", true},
	{"!=2.0.*", "2.0", false},
	{"==2.0.*", "2.0.0", true},
	{"!=2.0.*", "2.0.0", false},
	{"==2.0.*", "2.0.0.0", true},
	{"!=2.0.*", "2.0.0.0", false},
	{"==2.0.*", "2.0.0.0.0", true},
	{"!=2.0.*", "2.0.0.0.0", false},
	{"!=2.0.0.*", "2", false},
	{"==2.0.0.*", "2.0", true},
	{"!=2.0.0.*", "2.0", false},
	{"==2.0.0.*", "2.0.0", true},
	{"!=2.0.0.*", "2.0.0", false},
	{"==2.0.0.*", "2.0.0.0", true},
	{"!=2.0.0.*", "2.0.0.0", false},
	{"==2.0.0.*", "2.0.0.0.0", true},
	{"!=2.0.0.*", "2.0.0.0.0", false},
	{"!=2.0.0.0.*", "2", false},
	{"==2.0.0.0.*", "2.0", true},
	{"!=2.0.0.0.*", "2.0", false},
	{"==2.0.0.0.*", "2.0.0", true},
	{"!=2.0.0.0.*", "2.0.0", false},
	{"==2.0.0.0.*", "2.0.0.0", true},
	{"!=2.0.0.0.*", "2.0.0.0", false},
	{"==2.0.0.0.*", "2.0.0.0.0", true},
	{"!=2.0.0.0.*", "2.0.0.0.0", false},
	{"==2.0.0.0.0.*", "2", true},
	{"!=2.0.0.0.0.*", "2", false},
	{"==2.0.0.0.0.*", "2.0", true},
	{"!=2.0.0.0.0.*", "2.0", false},
	{"==2.0.0.0.0.*", "2.0.0", true},
	{"!=2.0.0.0.0.*", "2.0.0", false},
	{"==2.0.0.0.0.*", "2.0.0.0", true},
	{"!=2.0.0.0.0.*", "2.0.0.0", false},
	{"==2.0.0.0.0.*", "2.0.0.0.0", true},
	{"!=2.0.0.0.0.*", "2.0.0.0.0", false},
	{"==1.2.0.0.*", "1.2", true},
	{"==1.2.*", "1.2.0.0", true},
	{"==1.0.0.0.0.*", "1", true},
	{"!=1.2.0.0.*", "1.2", false},
	{"==1.2.3.0.0.*", "1.2.3", true},
	{"==1.2.0.0.*", "1.2.0.0.0", true},
	{"==1.*", "1.0.0.0.0", true},
	{"==2.0.0.*", "2rc1", true},
	{"==2.0.0.*", "2.0rc1", true},
	{"==2.0.0.0.*", "2a1", true},
	{"!=2.0.0.*", "2rc1", false},
	{"==1.0.0.*", "1.0.0a1", true},
	{"==0.0.0.*", "1!1.0", false},
	{"~=0.0.0a1", "1!1.0", false},
	{"~=0.0.0rc1", "1!1.0", false},
	{"==0.0.*", "1!1.0", false},
	{"==1!1.0.*", "1!1.0", true},
	{"==1!1.*", "1!1.0", true},
	{"==1!1.0.0.*", "1!1", true},
	{"==2!1.0.0.0.*", "2!1", true},
	{"==1.0.*", "1!1.0", false},
	{"!=0.0.0.*", "1!1.0", true},
	{"==0!2.0.0.*", "2", true},
	{"~=1!1.0.0", "1!1.0.5", true},
	{"~=2.0", "2", true},
	{"~=2.0.0", "2", true},
	{"~=2.0.0.0.0", "2", true},
	{"~=1.2.0.0", "1.2", true},
	{"~=1.2", "1.2.0.0", true},
	{"~=1.2.0.0", "1.2.9.9", false},
}
