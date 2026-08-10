// SPDX-License-Identifier: Apache-2.0 OR MIT

// The PEP 440 pre-release / post-release ALIAS and SEPARATOR family, swept
// against pypa/packaging 26.2 for both `~=` and wildcard equality.
//
// PEP 440 defines `alpha`->`a`, `beta`->`b`, `c`/`pre`/`preview`->`rc`,
// `r`/`rev`->`post`, and permits `.`, `-` or `_` before the letter. This table
// covers every alias against every separator, because a fix that handles one
// token and leaves its siblings behind is the recurring failure mode in this
// area.
//
// ⚠️ WHAT THIS TABLE PINS, AND WHY IT IS NOT A BUG REPORT.
//
// `~=1.0c1` and `~=1.0rc1` give DIFFERENT answers -- `1.1` satisfies the second
// and not the first -- even though PEP 440 calls `c` an alias for `rc`. That
// asymmetry is upstream's, and this package reproduces it deliberately:
//
//	packaging 26.2:  SpecifierSet("~=1.0c1").contains("1.1")   -> False
//	                 SpecifierSet("~=1.0rc1").contains("1.1")  -> True
//
// The cause is a mismatch between two upstream tables. `_prefix_regex` is
// `([0-9]+)((?:a|b|c|rc)[0-9]+)` and DOES include `c`, so `0c1` is split into
// `0` + `c1`; but `_is_not_suffix` checks only `("dev","a","b","rc","post")` and
// does NOT include `c`, so the resulting `c1` is treated as part of the release
// segment rather than as a suffix, and the derived prefix is one component
// narrower. `c` is the only alias that falls in that gap: `alpha`, `beta`, `pre`
// and `preview` never match `_prefix_regex` in the first place, so they stay
// whole and are handled consistently.
//
// ⚠️ Normalizing the operand through the parser before deriving the prefix --
// the obvious structural fix, and one this package applies in specifierEqual --
// is WRONG here. Measured: it turns 0 divergences into 6, breaking `~=1.0c1`
// plus the dot-separated `~=1.0.pre1`, `~=1.0.preview1`, `~=1.0.r1`,
// `~=1.0.rev1` and `~=1.0.c1`, because upstream derives the prefix from the RAW
// operand text and those forms do not match `_prefix_regex` un-normalized.
//
// ⚠️ Upstream's own test suite does not cover `c` under `~=`, so the ported
// conformance tables cannot catch a change here -- verified, they stay green
// under the broken variant. Only this independent sweep does, which is why it
// exists separately from the ported tables.

package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConformance_PreReleaseAliasesAndSeparators(t *testing.T) {
	for _, tt := range aliasOracle {
		t.Run(tt.spec+"__"+tt.version, func(t *testing.T) {
			ss, err := NewSpecifiers(tt.spec, WithPreReleases(PreReleasesInclude))
			require.NoError(t, err, "specifier %q must parse", tt.spec)
			v, err := Parse(tt.version)
			require.NoError(t, err, "version %q must parse", tt.version)
			assert.Equal(t, tt.want, ss.Check(v))
		})
	}
}

// The alias tables that produce the asymmetry, pinned directly so a future
// editor sees the coupling rather than discovering it.
//
// isNotSuffix omits "c" ON PURPOSE, mirroring upstream's _is_not_suffix.
// prefixRegexp includes "c", mirroring upstream's _prefix_regex. Changing either
// one alone changes ~= behavior; the test above measures the consequence.
func TestAliasTablesMatchUpstream(t *testing.T) {
	// isNotSuffix recognizes exactly upstream's five prefixes.
	suffixes := []string{"dev", "a", "b", "rc", "post"}
	for _, s := range suffixes {
		assert.False(t, isNotSuffix(s+"1"), "%q1 must be recognized as a suffix", s)
	}
	// ⚠️ The check is a PREFIX match, not an equality test, which is why the
	// alias family splits three ways rather than two. Verified against
	// upstream's _is_not_suffix one token at a time:
	//
	//	recognized as a suffix (prefix-matches one of the five):
	//	    dev1, a1, b1, rc1, post1, alpha1 (via "a"), beta1 (via "b")
	//	NOT recognized:
	//	    c1, pre1, preview1, r1, rev1
	//
	// So "alpha1" and "beta1" are caught incidentally by the "a" and "b"
	// entries, while "c1", "pre1", "preview1", "r1" and "rev1" are not caught
	// at all.
	for _, s := range []string{"alpha1", "beta1"} {
		assert.False(t, isNotSuffix(s),
			"%q must prefix-match a suffix entry, as it does upstream", s)
	}
	for _, s := range []string{"c1", "pre1", "preview1", "r1", "rev1"} {
		assert.True(t, isNotSuffix(s),
			"%q must NOT be recognized as a suffix; upstream's _is_not_suffix does not "+
				"recognize it either, and adding it changes ~= answers away from the reference", s)
	}

	// prefixRegexp DOES include c, which is the other half of the coupling.
	assert.NotNil(t, prefixRegexp.FindStringSubmatch("0c1"),
		`prefixRegexp must split "0c1"; upstream's _prefix_regex includes c`)
	assert.NotNil(t, prefixRegexp.FindStringSubmatch("0rc1"))
	assert.Nil(t, prefixRegexp.FindStringSubmatch("0alpha1"),
		`prefixRegexp must NOT split "0alpha1"; upstream's does not either`)
}

// 410 rows. Expected values measured against pypa/packaging 26.2.
//
// ⚠️ Note the asymmetry these rows PIN rather than fix: `~=1.0c1` and
// `~=1.0rc1` give DIFFERENT answers, even though PEP 440 defines `c` as an
// alias for `rc`. That is upstream's behavior, not this package's choice --
// upstream's _prefix_regex includes `c` while its _is_not_suffix does not, so
// `c1` is split off as its own component and then treated as part of the
// release rather than as a suffix. Conforming to the reference is the point
// of these tables, so the rows record it; see isNotSuffix.
var aliasOracle = []struct {
	spec    string
	version string
	want    bool
}{
	{"~=1.0a1", "1.1", true},
	{"~=1.0a1", "1.0", true},
	{"~=1.0a1", "1.0.1", true},
	{"~=1.0a1", "1.0.5", true},
	{"~=1.0a1", "2.0", false},
	{"~=1.0a1", "1.0c1", true},
	{"~=1.0a1", "1.0rc1", true},
	{"~=1.0a1", "0.9", false},
	{"~=1.0alpha1", "1.1", true},
	{"~=1.0alpha1", "1.0", true},
	{"~=1.0alpha1", "1.0.1", true},
	{"~=1.0alpha1", "1.0.5", true},
	{"~=1.0alpha1", "2.0", false},
	{"~=1.0alpha1", "1.0c1", true},
	{"~=1.0alpha1", "1.0rc1", true},
	{"~=1.0alpha1", "0.9", false},
	{"~=1.0b1", "1.1", true},
	{"~=1.0b1", "1.0", true},
	{"~=1.0b1", "1.0.1", true},
	{"~=1.0b1", "1.0.5", true},
	{"~=1.0b1", "2.0", false},
	{"~=1.0b1", "1.0c1", true},
	{"~=1.0b1", "1.0rc1", true},
	{"~=1.0b1", "0.9", false},
	{"~=1.0beta1", "1.1", true},
	{"~=1.0beta1", "1.0", true},
	{"~=1.0beta1", "1.0.1", true},
	{"~=1.0beta1", "1.0.5", true},
	{"~=1.0beta1", "2.0", false},
	{"~=1.0beta1", "1.0c1", true},
	{"~=1.0beta1", "1.0rc1", true},
	{"~=1.0beta1", "0.9", false},
	{"~=1.0c1", "1.1", false},
	{"~=1.0c1", "1.0", true},
	{"~=1.0c1", "1.0.1", true},
	{"~=1.0c1", "1.0.5", true},
	{"~=1.0c1", "2.0", false},
	{"~=1.0c1", "1.0c1", true},
	{"~=1.0c1", "1.0rc1", true},
	{"~=1.0c1", "0.9", false},
	{"~=1.0rc1", "1.1", true},
	{"~=1.0rc1", "1.0", true},
	{"~=1.0rc1", "1.0.1", true},
	{"~=1.0rc1", "1.0.5", true},
	{"~=1.0rc1", "2.0", false},
	{"~=1.0rc1", "1.0c1", true},
	{"~=1.0rc1", "1.0rc1", true},
	{"~=1.0rc1", "0.9", false},
	{"~=1.0pre1", "1.1", true},
	{"~=1.0pre1", "1.0", true},
	{"~=1.0pre1", "1.0.1", true},
	{"~=1.0pre1", "1.0.5", true},
	{"~=1.0pre1", "2.0", false},
	{"~=1.0pre1", "1.0c1", true},
	{"~=1.0pre1", "1.0rc1", true},
	{"~=1.0pre1", "0.9", false},
	{"~=1.0preview1", "1.1", true},
	{"~=1.0preview1", "1.0", true},
	{"~=1.0preview1", "1.0.1", true},
	{"~=1.0preview1", "1.0.5", true},
	{"~=1.0preview1", "2.0", false},
	{"~=1.0preview1", "1.0c1", true},
	{"~=1.0preview1", "1.0rc1", true},
	{"~=1.0preview1", "0.9", false},
	{"~=1.0.a1", "1.1", true},
	{"~=1.0.a1", "1.0", true},
	{"~=1.0.a1", "1.0.1", true},
	{"~=1.0.a1", "1.0.5", true},
	{"~=1.0.a1", "2.0", false},
	{"~=1.0.a1", "1.0c1", true},
	{"~=1.0.a1", "1.0rc1", true},
	{"~=1.0.a1", "0.9", false},
	{"~=1.0.alpha1", "1.1", true},
	{"~=1.0.alpha1", "1.0", true},
	{"~=1.0.alpha1", "1.0.1", true},
	{"~=1.0.alpha1", "1.0.5", true},
	{"~=1.0.alpha1", "2.0", false},
	{"~=1.0.alpha1", "1.0c1", true},
	{"~=1.0.alpha1", "1.0rc1", true},
	{"~=1.0.alpha1", "0.9", false},
	{"~=1.0.b1", "1.1", true},
	{"~=1.0.b1", "1.0", true},
	{"~=1.0.b1", "1.0.1", true},
	{"~=1.0.b1", "1.0.5", true},
	{"~=1.0.b1", "2.0", false},
	{"~=1.0.b1", "1.0c1", true},
	{"~=1.0.b1", "1.0rc1", true},
	{"~=1.0.b1", "0.9", false},
	{"~=1.0.beta1", "1.1", true},
	{"~=1.0.beta1", "1.0", true},
	{"~=1.0.beta1", "1.0.1", true},
	{"~=1.0.beta1", "1.0.5", true},
	{"~=1.0.beta1", "2.0", false},
	{"~=1.0.beta1", "1.0c1", true},
	{"~=1.0.beta1", "1.0rc1", true},
	{"~=1.0.beta1", "0.9", false},
	{"~=1.0.c1", "1.1", false},
	{"~=1.0.c1", "1.0", true},
	{"~=1.0.c1", "1.0.1", true},
	{"~=1.0.c1", "1.0.5", true},
	{"~=1.0.c1", "2.0", false},
	{"~=1.0.c1", "1.0c1", true},
	{"~=1.0.c1", "1.0rc1", true},
	{"~=1.0.c1", "0.9", false},
	{"~=1.0.rc1", "1.1", true},
	{"~=1.0.rc1", "1.0", true},
	{"~=1.0.rc1", "1.0.1", true},
	{"~=1.0.rc1", "1.0.5", true},
	{"~=1.0.rc1", "2.0", false},
	{"~=1.0.rc1", "1.0c1", true},
	{"~=1.0.rc1", "1.0rc1", true},
	{"~=1.0.rc1", "0.9", false},
	{"~=1.0.pre1", "1.1", false},
	{"~=1.0.pre1", "1.0", true},
	{"~=1.0.pre1", "1.0.1", true},
	{"~=1.0.pre1", "1.0.5", true},
	{"~=1.0.pre1", "2.0", false},
	{"~=1.0.pre1", "1.0c1", true},
	{"~=1.0.pre1", "1.0rc1", true},
	{"~=1.0.pre1", "0.9", false},
	{"~=1.0.preview1", "1.1", false},
	{"~=1.0.preview1", "1.0", true},
	{"~=1.0.preview1", "1.0.1", true},
	{"~=1.0.preview1", "1.0.5", true},
	{"~=1.0.preview1", "2.0", false},
	{"~=1.0.preview1", "1.0c1", true},
	{"~=1.0.preview1", "1.0rc1", true},
	{"~=1.0.preview1", "0.9", false},
	{"~=1.0-a1", "1.1", true},
	{"~=1.0-a1", "1.0", true},
	{"~=1.0-a1", "1.0.1", true},
	{"~=1.0-a1", "1.0.5", true},
	{"~=1.0-a1", "2.0", false},
	{"~=1.0-a1", "1.0c1", true},
	{"~=1.0-a1", "1.0rc1", true},
	{"~=1.0-a1", "0.9", false},
	{"~=1.0-alpha1", "1.1", true},
	{"~=1.0-alpha1", "1.0", true},
	{"~=1.0-alpha1", "1.0.1", true},
	{"~=1.0-alpha1", "1.0.5", true},
	{"~=1.0-alpha1", "2.0", false},
	{"~=1.0-alpha1", "1.0c1", true},
	{"~=1.0-alpha1", "1.0rc1", true},
	{"~=1.0-alpha1", "0.9", false},
	{"~=1.0-b1", "1.1", true},
	{"~=1.0-b1", "1.0", true},
	{"~=1.0-b1", "1.0.1", true},
	{"~=1.0-b1", "1.0.5", true},
	{"~=1.0-b1", "2.0", false},
	{"~=1.0-b1", "1.0c1", true},
	{"~=1.0-b1", "1.0rc1", true},
	{"~=1.0-b1", "0.9", false},
	{"~=1.0-beta1", "1.1", true},
	{"~=1.0-beta1", "1.0", true},
	{"~=1.0-beta1", "1.0.1", true},
	{"~=1.0-beta1", "1.0.5", true},
	{"~=1.0-beta1", "2.0", false},
	{"~=1.0-beta1", "1.0c1", true},
	{"~=1.0-beta1", "1.0rc1", true},
	{"~=1.0-beta1", "0.9", false},
	{"~=1.0-c1", "1.1", true},
	{"~=1.0-c1", "1.0", true},
	{"~=1.0-c1", "1.0.1", true},
	{"~=1.0-c1", "1.0.5", true},
	{"~=1.0-c1", "2.0", false},
	{"~=1.0-c1", "1.0c1", true},
	{"~=1.0-c1", "1.0rc1", true},
	{"~=1.0-c1", "0.9", false},
	{"~=1.0-rc1", "1.1", true},
	{"~=1.0-rc1", "1.0", true},
	{"~=1.0-rc1", "1.0.1", true},
	{"~=1.0-rc1", "1.0.5", true},
	{"~=1.0-rc1", "2.0", false},
	{"~=1.0-rc1", "1.0c1", true},
	{"~=1.0-rc1", "1.0rc1", true},
	{"~=1.0-rc1", "0.9", false},
	{"~=1.0-pre1", "1.1", true},
	{"~=1.0-pre1", "1.0", true},
	{"~=1.0-pre1", "1.0.1", true},
	{"~=1.0-pre1", "1.0.5", true},
	{"~=1.0-pre1", "2.0", false},
	{"~=1.0-pre1", "1.0c1", true},
	{"~=1.0-pre1", "1.0rc1", true},
	{"~=1.0-pre1", "0.9", false},
	{"~=1.0-preview1", "1.1", true},
	{"~=1.0-preview1", "1.0", true},
	{"~=1.0-preview1", "1.0.1", true},
	{"~=1.0-preview1", "1.0.5", true},
	{"~=1.0-preview1", "2.0", false},
	{"~=1.0-preview1", "1.0c1", true},
	{"~=1.0-preview1", "1.0rc1", true},
	{"~=1.0-preview1", "0.9", false},
	{"~=1.0_a1", "1.1", true},
	{"~=1.0_a1", "1.0", true},
	{"~=1.0_a1", "1.0.1", true},
	{"~=1.0_a1", "1.0.5", true},
	{"~=1.0_a1", "2.0", false},
	{"~=1.0_a1", "1.0c1", true},
	{"~=1.0_a1", "1.0rc1", true},
	{"~=1.0_a1", "0.9", false},
	{"~=1.0_alpha1", "1.1", true},
	{"~=1.0_alpha1", "1.0", true},
	{"~=1.0_alpha1", "1.0.1", true},
	{"~=1.0_alpha1", "1.0.5", true},
	{"~=1.0_alpha1", "2.0", false},
	{"~=1.0_alpha1", "1.0c1", true},
	{"~=1.0_alpha1", "1.0rc1", true},
	{"~=1.0_alpha1", "0.9", false},
	{"~=1.0_b1", "1.1", true},
	{"~=1.0_b1", "1.0", true},
	{"~=1.0_b1", "1.0.1", true},
	{"~=1.0_b1", "1.0.5", true},
	{"~=1.0_b1", "2.0", false},
	{"~=1.0_b1", "1.0c1", true},
	{"~=1.0_b1", "1.0rc1", true},
	{"~=1.0_b1", "0.9", false},
	{"~=1.0_beta1", "1.1", true},
	{"~=1.0_beta1", "1.0", true},
	{"~=1.0_beta1", "1.0.1", true},
	{"~=1.0_beta1", "1.0.5", true},
	{"~=1.0_beta1", "2.0", false},
	{"~=1.0_beta1", "1.0c1", true},
	{"~=1.0_beta1", "1.0rc1", true},
	{"~=1.0_beta1", "0.9", false},
	{"~=1.0_c1", "1.1", true},
	{"~=1.0_c1", "1.0", true},
	{"~=1.0_c1", "1.0.1", true},
	{"~=1.0_c1", "1.0.5", true},
	{"~=1.0_c1", "2.0", false},
	{"~=1.0_c1", "1.0c1", true},
	{"~=1.0_c1", "1.0rc1", true},
	{"~=1.0_c1", "0.9", false},
	{"~=1.0_rc1", "1.1", true},
	{"~=1.0_rc1", "1.0", true},
	{"~=1.0_rc1", "1.0.1", true},
	{"~=1.0_rc1", "1.0.5", true},
	{"~=1.0_rc1", "2.0", false},
	{"~=1.0_rc1", "1.0c1", true},
	{"~=1.0_rc1", "1.0rc1", true},
	{"~=1.0_rc1", "0.9", false},
	{"~=1.0_pre1", "1.1", true},
	{"~=1.0_pre1", "1.0", true},
	{"~=1.0_pre1", "1.0.1", true},
	{"~=1.0_pre1", "1.0.5", true},
	{"~=1.0_pre1", "2.0", false},
	{"~=1.0_pre1", "1.0c1", true},
	{"~=1.0_pre1", "1.0rc1", true},
	{"~=1.0_pre1", "0.9", false},
	{"~=1.0_preview1", "1.1", true},
	{"~=1.0_preview1", "1.0", true},
	{"~=1.0_preview1", "1.0.1", true},
	{"~=1.0_preview1", "1.0.5", true},
	{"~=1.0_preview1", "2.0", false},
	{"~=1.0_preview1", "1.0c1", true},
	{"~=1.0_preview1", "1.0rc1", true},
	{"~=1.0_preview1", "0.9", false},
	{"~=1.0post1", "1.1", true},
	{"~=1.0post1", "1.0", false},
	{"~=1.0post1", "1.0.1", true},
	{"~=1.0post1", "1.0.5", true},
	{"~=1.0post1", "2.0", false},
	{"~=1.0post1", "1.0c1", false},
	{"~=1.0post1", "1.0rc1", false},
	{"~=1.0post1", "0.9", false},
	{"~=1.0r1", "1.1", true},
	{"~=1.0r1", "1.0", false},
	{"~=1.0r1", "1.0.1", true},
	{"~=1.0r1", "1.0.5", true},
	{"~=1.0r1", "2.0", false},
	{"~=1.0r1", "1.0c1", false},
	{"~=1.0r1", "1.0rc1", false},
	{"~=1.0r1", "0.9", false},
	{"~=1.0rev1", "1.1", true},
	{"~=1.0rev1", "1.0", false},
	{"~=1.0rev1", "1.0.1", true},
	{"~=1.0rev1", "1.0.5", true},
	{"~=1.0rev1", "2.0", false},
	{"~=1.0rev1", "1.0c1", false},
	{"~=1.0rev1", "1.0rc1", false},
	{"~=1.0rev1", "0.9", false},
	{"~=1.0.post1", "1.1", true},
	{"~=1.0.post1", "1.0", false},
	{"~=1.0.post1", "1.0.1", true},
	{"~=1.0.post1", "1.0.5", true},
	{"~=1.0.post1", "2.0", false},
	{"~=1.0.post1", "1.0c1", false},
	{"~=1.0.post1", "1.0rc1", false},
	{"~=1.0.post1", "0.9", false},
	{"~=1.0.r1", "1.1", false},
	{"~=1.0.r1", "1.0", false},
	{"~=1.0.r1", "1.0.1", true},
	{"~=1.0.r1", "1.0.5", true},
	{"~=1.0.r1", "2.0", false},
	{"~=1.0.r1", "1.0c1", false},
	{"~=1.0.r1", "1.0rc1", false},
	{"~=1.0.r1", "0.9", false},
	{"~=1.0.rev1", "1.1", false},
	{"~=1.0.rev1", "1.0", false},
	{"~=1.0.rev1", "1.0.1", true},
	{"~=1.0.rev1", "1.0.5", true},
	{"~=1.0.rev1", "2.0", false},
	{"~=1.0.rev1", "1.0c1", false},
	{"~=1.0.rev1", "1.0rc1", false},
	{"~=1.0.rev1", "0.9", false},
	{"~=1.0-post1", "1.1", true},
	{"~=1.0-post1", "1.0", false},
	{"~=1.0-post1", "1.0.1", true},
	{"~=1.0-post1", "1.0.5", true},
	{"~=1.0-post1", "2.0", false},
	{"~=1.0-post1", "1.0c1", false},
	{"~=1.0-post1", "1.0rc1", false},
	{"~=1.0-post1", "0.9", false},
	{"~=1.0-r1", "1.1", true},
	{"~=1.0-r1", "1.0", false},
	{"~=1.0-r1", "1.0.1", true},
	{"~=1.0-r1", "1.0.5", true},
	{"~=1.0-r1", "2.0", false},
	{"~=1.0-r1", "1.0c1", false},
	{"~=1.0-r1", "1.0rc1", false},
	{"~=1.0-r1", "0.9", false},
	{"~=1.0-rev1", "1.1", true},
	{"~=1.0-rev1", "1.0", false},
	{"~=1.0-rev1", "1.0.1", true},
	{"~=1.0-rev1", "1.0.5", true},
	{"~=1.0-rev1", "2.0", false},
	{"~=1.0-rev1", "1.0c1", false},
	{"~=1.0-rev1", "1.0rc1", false},
	{"~=1.0-rev1", "0.9", false},
	{"~=1.0_post1", "1.1", true},
	{"~=1.0_post1", "1.0", false},
	{"~=1.0_post1", "1.0.1", true},
	{"~=1.0_post1", "1.0.5", true},
	{"~=1.0_post1", "2.0", false},
	{"~=1.0_post1", "1.0c1", false},
	{"~=1.0_post1", "1.0rc1", false},
	{"~=1.0_post1", "0.9", false},
	{"~=1.0_r1", "1.1", true},
	{"~=1.0_r1", "1.0", false},
	{"~=1.0_r1", "1.0.1", true},
	{"~=1.0_r1", "1.0.5", true},
	{"~=1.0_r1", "2.0", false},
	{"~=1.0_r1", "1.0c1", false},
	{"~=1.0_r1", "1.0rc1", false},
	{"~=1.0_r1", "0.9", false},
	{"~=1.0_rev1", "1.1", true},
	{"~=1.0_rev1", "1.0", false},
	{"~=1.0_rev1", "1.0.1", true},
	{"~=1.0_rev1", "1.0.5", true},
	{"~=1.0_rev1", "2.0", false},
	{"~=1.0_rev1", "1.0c1", false},
	{"~=1.0_rev1", "1.0rc1", false},
	{"~=1.0_rev1", "0.9", false},
	{"~=1.0dev1", "1.1", true},
	{"~=1.0dev1", "1.0", true},
	{"~=1.0dev1", "1.0.1", true},
	{"~=1.0dev1", "1.0.5", true},
	{"~=1.0dev1", "2.0", false},
	{"~=1.0dev1", "1.0c1", true},
	{"~=1.0dev1", "1.0rc1", true},
	{"~=1.0dev1", "0.9", false},
	{"~=1.0.dev1", "1.1", true},
	{"~=1.0.dev1", "1.0", true},
	{"~=1.0.dev1", "1.0.1", true},
	{"~=1.0.dev1", "1.0.5", true},
	{"~=1.0.dev1", "2.0", false},
	{"~=1.0.dev1", "1.0c1", true},
	{"~=1.0.dev1", "1.0rc1", true},
	{"~=1.0.dev1", "0.9", false},
	{"~=1.0-dev1", "1.1", true},
	{"~=1.0-dev1", "1.0", true},
	{"~=1.0-dev1", "1.0.1", true},
	{"~=1.0-dev1", "1.0.5", true},
	{"~=1.0-dev1", "2.0", false},
	{"~=1.0-dev1", "1.0c1", true},
	{"~=1.0-dev1", "1.0rc1", true},
	{"~=1.0-dev1", "0.9", false},
	{"~=1.0_dev1", "1.1", true},
	{"~=1.0_dev1", "1.0", true},
	{"~=1.0_dev1", "1.0.1", true},
	{"~=1.0_dev1", "1.0.5", true},
	{"~=1.0_dev1", "2.0", false},
	{"~=1.0_dev1", "1.0c1", true},
	{"~=1.0_dev1", "1.0rc1", true},
	{"~=1.0_dev1", "0.9", false},
	{"==1.*", "1.0", true},
	{"==1.0.*", "1.0", true},
	{"==1.*", "1.1", true},
	{"==1.0.*", "1.1", false},
	{"==1.*", "1.0a1", true},
	{"==1.0.*", "1.0a1", true},
	{"==1.*", "1.0alpha1", true},
	{"==1.0.*", "1.0alpha1", true},
	{"==1.*", "1.0b1", true},
	{"==1.0.*", "1.0b1", true},
	{"==1.*", "1.0beta1", true},
	{"==1.0.*", "1.0beta1", true},
	{"==1.*", "1.0c1", true},
	{"==1.0.*", "1.0c1", true},
	{"==1.*", "1.0rc1", true},
	{"==1.0.*", "1.0rc1", true},
	{"==1.*", "1.0pre1", true},
	{"==1.0.*", "1.0pre1", true},
	{"==1.*", "1.0preview1", true},
	{"==1.0.*", "1.0preview1", true},
	{"==1.*", "1.0post1", true},
	{"==1.0.*", "1.0post1", true},
	{"==1.*", "1.0r1", true},
	{"==1.0.*", "1.0r1", true},
	{"==1.*", "1.0rev1", true},
	{"==1.0.*", "1.0rev1", true},
}
