// SPDX-License-Identifier: Apache-2.0 OR MIT

// Unit tests for the three-state pre-release policy added for
// rstudio/package-manager#19383.
//
// Every expected value was measured against pypa/packaging 26.2.

package version

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSpecifier_PreReleases_Autodetect pins the PEP 440 derivation upstream
// performs when no policy is set. Note the two carve-outs that are easy to get
// backwards: `!=` never implies pre-releases even against a pre-release
// operand, and `===` stays UNRESOLVED because its operand is not a version at
// all.
func TestSpecifier_PreReleases_Autodetect(t *testing.T) {
	tests := []struct {
		in   string
		want PreReleases
	}{
		{"==1.0", PreReleasesExclude},
		{">=1.0", PreReleasesExclude},
		{"<=1.0", PreReleasesExclude},
		{"~=1.0", PreReleasesExclude},
		{"<1.0", PreReleasesExclude},
		{">1.0", PreReleasesExclude},
		{"!=1.0", PreReleasesExclude},
		{"<1.0.dev1", PreReleasesInclude},
		{">1.0.dev1", PreReleasesInclude},
		{"==1.0.dev1", PreReleasesInclude},
		{">=1.0.dev1", PreReleasesInclude},
		{"<=1.0.dev1", PreReleasesInclude},
		{"~=1.0.dev1", PreReleasesInclude},
		{"==1.0a1.dev1", PreReleasesInclude},
		// `!=` against a pre-release operand still excludes.
		{"!=1.0.dev1", PreReleasesExclude},
		// A wildcard `==` cannot name a pre-release, so it excludes.
		{"==1.0.*", PreReleasesExclude},
		// An opaque `===` operand leaves the policy unresolved.
		{"===lolwat", PreReleasesAuto},
		// ...unless the operand happens to parse as a pre-release version.
		{"===1.0.dev1", PreReleasesInclude},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			s, err := NewSpecifier(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, s.PreReleases())
		})
	}
}

// An explicit policy short-circuits the derivation entirely.
func TestSpecifier_PreReleases_ExplicitWins(t *testing.T) {
	for _, policy := range []PreReleases{PreReleasesInclude, PreReleasesExclude} {
		t.Run(policy.String(), func(t *testing.T) {
			// ">=1.0" would autodetect Exclude, "<1.0.dev1" Include.
			for _, spec := range []string{">=1.0", "<1.0.dev1"} {
				s, err := NewSpecifier(spec, WithPreReleases(policy))
				require.NoError(t, err)
				assert.Equal(t, policy, s.PreReleases(), "spec %q", spec)
			}
		})
	}
}

// TestSpecifiers_PreReleases_Autodetect pins the SET-level derivation, which is
// deliberately asymmetric with the single-specifier one: a set of ordinary
// final-release specifiers resolves to PreReleasesAuto, never to Exclude, even
// though each member alone resolves to Exclude. That asymmetry is upstream's,
// and it is what lets a set fall back to offering a pre-release when nothing
// else matched.
func TestSpecifiers_PreReleases_Autodetect(t *testing.T) {
	tests := []struct {
		in   string
		want PreReleases
	}{
		{"", PreReleasesAuto},
		{"==1.0", PreReleasesAuto},
		{">=1.0,<2", PreReleasesAuto},
		{"!=1.0.dev1", PreReleasesAuto},
		{"==1.0.*", PreReleasesAuto},
		{"===lolwat", PreReleasesAuto},
		{"<1.0.dev1", PreReleasesInclude},
		{">=1.0,<2.0.dev1", PreReleasesInclude},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.in), func(t *testing.T) {
			ss, err := NewSpecifiers(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, ss.PreReleases())
		})
	}
}

// ⚠️ THE central distinction. A pre-release DOES satisfy an ordinary
// specifier; PEP 440's rule is about which candidate an installer offers when
// several match, not about which versions the operator matches. Conflating the
// two is what produced the bug this API exists to prevent.
func TestPreReleaseExclusionIsSelectionNotMatching(t *testing.T) {
	tests := []struct {
		spec string
		item string
		want bool
	}{
		// Queried alone, a pre-release satisfies >= and > -- there is no final
		// release to prefer over it.
		{">=1.0", "1.5a1", true},
		{">=1.0", "2.0.dev1", true},
		{"==2.0.*", "2.0.dev1", true},
		{"!=1.0.dev1", "1.5a1", true},
		// These false answers come from the OPERATORS, not from the policy:
		// `<2` excludes a pre-release of 2, and `>2` excludes a post-release
		// of 2, at every policy setting.
		{"<2", "2.0.dev1", false},
		{">2", "2.0.post1", false},
		{"!=2.0a1", "2.0a1", false},
	}

	for _, tt := range tests {
		t.Run(tt.spec+"__"+tt.item, func(t *testing.T) {
			s, err := NewSpecifier(tt.spec)
			require.NoError(t, err)
			assert.Equal(t, tt.want, s.Contains(tt.item), "Specifier.Contains")

			ss, err := NewSpecifiers(tt.spec)
			require.NoError(t, err)
			assert.Equal(t, tt.want, ss.Contains(tt.item), "Specifiers.Contains")
		})
	}
}

// No pre-release policy can defeat an operator-level guard. This is the
// regression test for the conflation WithPreRelease(true) used to introduce:
// it set a flag that made Version.IsPreRelease lie, so `<2` matched
// `2.0.dev1`. Measured against packaging 26.2, `<2` contains `2.0.dev1` under
// none of the three policies.
func TestNoPolicyDefeatsTheOperatorGuards(t *testing.T) {
	guards := []struct{ spec, item string }{
		{"<2", "2.0.dev1"},
		{"<2", "2.0a1"},
		{"<3.1", "3.1.dev0"},
		{">2", "2.0.post1"},
		{">=2", "2.0.dev1"},
	}

	for _, g := range guards {
		for _, policy := range []PreReleases{PreReleasesAuto, PreReleasesInclude, PreReleasesExclude} {
			t.Run(g.spec+"__"+g.item+"__"+policy.String(), func(t *testing.T) {
				s, err := NewSpecifier(g.spec, WithPreReleases(policy))
				require.NoError(t, err)
				assert.False(t, s.Contains(g.item), "constructor policy")

				plain, err := NewSpecifier(g.spec)
				require.NoError(t, err)
				assert.False(t, plain.Contains(g.item, WithPreReleases(policy)), "per-call policy")
			})
		}
	}

	// The same, through the deprecated two-state option.
	legacy, err := NewSpecifiers("<2", WithPreRelease(true))
	require.NoError(t, err)
	assert.False(t, legacy.Check(MustParse("2.0.dev1")))
}

func TestSpecifier_Filter(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		specPre PreReleases
		callPre PreReleases
		items   []string
		want    []string
	}{
		// The PEP 440 preference: a final release wins, a pre-release is
		// offered only when nothing else matched.
		{"final wins", ">=1.2.3", PreReleasesAuto, PreReleasesAuto,
			[]string{"1.2", "1.3", "1.5a1"}, []string{"1.3"}},
		{"pre offered when alone", ">=1.2.3", PreReleasesAuto, PreReleasesAuto,
			[]string{"1.2", "1.5a1"}, []string{"1.5a1"}},
		// An operand that is itself a pre-release autodetects Include, so both
		// come through.
		{"prerelease operand includes", ">=1.0.dev1", PreReleasesAuto, PreReleasesAuto,
			[]string{"1.0", "2.0a1"}, []string{"1.0", "2.0a1"}},
		{"per-call exclude overrides", ">=1.0.dev1", PreReleasesAuto, PreReleasesExclude,
			[]string{"1.0", "2.0a1"}, []string{"1.0"}},
		{"constructor include", ">=1.0.dev1", PreReleasesInclude, PreReleasesAuto,
			[]string{"1.0", "2.0a1"}, []string{"1.0", "2.0a1"}},
		{"constructor exclude", ">=1.0.dev1", PreReleasesExclude, PreReleasesAuto,
			[]string{"1.0", "2.0a1"}, []string{"1.0"}},
		{"not-equal drops the pre", "!=2.0a1", PreReleasesAuto, PreReleasesAuto,
			[]string{"1.0a2", "1.0", "2.0a1"}, []string{"1.0"}},
		{"compatible with pre operand", "~=2.0a1", PreReleasesAuto, PreReleasesAuto,
			[]string{"1.0", "2.0a1", "3.0a2", "3.0"}, []string{"2.0a1"}},
		// An unparseable string is silently discarded, not an error.
		{"invalid alone", ">=1.0", PreReleasesAuto, PreReleasesAuto,
			[]string{"not a valid version"}, nil},
		{"invalid alongside valid", ">=1.0", PreReleasesAuto, PreReleasesAuto,
			[]string{"1.0", "not a valid version"}, []string{"1.0"}},
		// Arbitrary equality is the only operator that can match a non-version,
		// and it compares the string as written: "1.0" does not match "1.0.0".
		{"arbitrary matches token", "===foobar", PreReleasesAuto, PreReleasesAuto,
			[]string{"foobar", "foo", "bar"}, []string{"foobar"}},
		{"arbitrary is not zero-padded", "===1.0", PreReleasesAuto, PreReleasesAuto,
			[]string{"1.0", "1.0.0", "2.0"}, []string{"1.0"}},
		{"not-equal rejects invalid", "!=1.0", PreReleasesAuto, PreReleasesAuto,
			[]string{"1.0", "invalid", "2.0"}, []string{"2.0"}},
		{"wildcard not-equal rejects invalid", "!=2.0.*", PreReleasesAuto, PreReleasesAuto,
			[]string{"invalid", "foobar", "2.0"}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewSpecifier(tt.spec, WithPreReleases(tt.specPre))
			require.NoError(t, err)

			got := s.Filter(tt.items, WithPreReleases(tt.callPre))
			assert.Equal(t, tt.want, emptyToNil(got))
		})
	}
}

func TestSpecifiers_Filter(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		specPre PreReleases
		callPre PreReleases
		items   []string
		want    []string
	}{
		// An empty set constrains nothing, so only the preference applies.
		{"empty prefers final", "", PreReleasesAuto, PreReleasesAuto,
			[]string{"1.3", "1.5a1"}, []string{"1.3"}},
		{"empty offers lone pre", "", PreReleasesAuto, PreReleasesAuto,
			[]string{"1.5a1"}, []string{"1.5a1"}},
		{"empty include", "", PreReleasesInclude, PreReleasesAuto,
			[]string{"1.3", "1.5a1"}, []string{"1.3", "1.5a1"}},
		{"empty per-call include", "", PreReleasesAuto, PreReleasesInclude,
			[]string{"1.3", "1.5a1"}, []string{"1.3", "1.5a1"}},
		{"empty exclude", "", PreReleasesAuto, PreReleasesExclude,
			[]string{"1.3", "1.5a1"}, []string{"1.3"}},
		{"single spec prefers final", ">=1.2.3", PreReleasesAuto, PreReleasesAuto,
			[]string{"1.2", "1.3", "1.5a1"}, []string{"1.3"}},
		{"single spec offers lone pre", ">=1.2.3", PreReleasesAuto, PreReleasesAuto,
			[]string{"1.2", "1.5a1"}, []string{"1.5a1"}},
		// Two specifiers: the preference is decided over the versions that
		// satisfied the WHOLE set, not per specifier.
		{"conjunction prefers final", ">=1.0,<3", PreReleasesAuto, PreReleasesAuto,
			[]string{"1.0", "2.0a1"}, []string{"1.0"}},
		{"conjunction offers lone pre", ">=1.0,<3", PreReleasesAuto, PreReleasesAuto,
			[]string{"2.0a1"}, []string{"2.0a1"}},
		{"conjunction with pre operand", ">=1.0.dev1,<2", PreReleasesAuto, PreReleasesAuto,
			[]string{"1.0", "1.5a1"}, []string{"1.0", "1.5a1"}},
		{"arbitrary in a set", "===foobar", PreReleasesAuto, PreReleasesAuto,
			[]string{"foobar", "1.0"}, []string{"foobar"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ss, err := NewSpecifiers(tt.spec, WithPreReleases(tt.specPre))
			require.NoError(t, err)

			got := ss.Filter(tt.items, WithPreReleases(tt.callPre))
			assert.Equal(t, tt.want, emptyToNil(got))
		})
	}
}

// Filter must preserve input order, not the order the buffering logic happens
// to release items in.
func TestFilterPreservesInputOrder(t *testing.T) {
	ss, err := NewSpecifiers(">=1.0", WithPreReleases(PreReleasesInclude))
	require.NoError(t, err)
	items := []string{"3.0", "1.5a1", "2.0", "1.0"}
	assert.Equal(t, items, ss.Filter(items))
}

// FilterBy is the `key=` seam: filtering a slice of arbitrary values by a
// version field. It must work for both Specifier and Specifiers.
func TestFilterBy(t *testing.T) {
	type release struct {
		name string
		ver  string
	}
	items := []release{{"a", "1.0"}, {"b", "2.1a1"}, {"c", "3.0"}}
	key := func(r release) string { return r.ver }

	spec, err := NewSpecifier(">=2.0")
	require.NoError(t, err)
	assert.Equal(t, []release{{"c", "3.0"}}, FilterBy(spec, items, key))
	assert.Equal(t,
		[]release{{"b", "2.1a1"}, {"c", "3.0"}},
		FilterBy(spec, items, key, WithPreReleases(PreReleasesInclude)))
	assert.Equal(t, []release{{"c", "3.0"}},
		FilterBy(spec, items, key, WithPreReleases(PreReleasesExclude)))

	// Only a pre-release matches: it is offered.
	onlyPre := []release{{"b", "2.1a1"}}
	assert.Equal(t, onlyPre, FilterBy(spec, onlyPre, key))
	assert.Empty(t, FilterBy(spec, onlyPre, key, WithPreReleases(PreReleasesExclude)))

	set, err := NewSpecifiers(">=2.0,<4")
	require.NoError(t, err)
	assert.Equal(t, []release{{"c", "3.0"}}, FilterBy(set, items, key))

	// An empty set filters on the preference alone.
	empty, err := NewSpecifiers("")
	require.NoError(t, err)
	assert.Equal(t, []release{{"a", "1.0"}, {"c", "3.0"}}, FilterBy(empty, items, key))
}

// ContainsVersion differs from Contains only for `===`, which compares the
// caller's spelling: a parsed Version has already been normalized, so
// `===1.1` contains Version("1.01") but not the string "1.01".
func TestArbitraryEqualityStringVersusVersion(t *testing.T) {
	tests := []struct {
		spec     string
		item     string
		wantStr  bool
		wantVers bool
	}{
		{"===1.1", "1.01", false, true},
		{"===1.01", "1.01", true, false},
		{"===1.1", "1.1", true, true},
		{"===1.0", "01.0", false, true},
		{"===01.0", "01.0", true, false},
		{"===1.0", "0!1.0", false, true},
		{"===1.0.post1", "1.0-1", false, true},
		{"===1.0.dev1", "1.0.dev01", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.spec+"__"+tt.item, func(t *testing.T) {
			s, err := NewSpecifier(tt.spec)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStr, s.Contains(tt.item), "string operand")

			v, err := Parse(tt.item)
			require.NoError(t, err)
			assert.Equal(t, tt.wantVers, s.ContainsVersion(v), "Version operand")
		})
	}
}

// A non-PEP 440 string is reachable through Contains, which is what makes
// `===lolwat` testable end to end. Specifiers.Check cannot express this at
// all: version.Parse("lolwat") fails, so there is no Version to pass.
func TestArbitraryEqualityAcceptsNonVersionStrings(t *testing.T) {
	tests := []struct {
		spec string
		item string
		want bool
	}{
		{"===lolwat", "lolwat", true},
		{"===lolwat", "LOLWAT", true},
		{"===LoLWaT", "lOlwAt", true},
		{"===lolwat", "notlolwat", false},
		{"===foobar", "invalid", false},
		{"===invalid", "invalid", true},
		// No other operator can match a non-version.
		{"==1.0", "not a valid version", false},
		{">=1.0", "not a valid version", false},
		{"!=1.0", "not a valid version", false},
		{"~=1.0", "not a valid version", false},
	}

	for _, tt := range tests {
		t.Run(tt.spec+"__"+tt.item, func(t *testing.T) {
			s, err := NewSpecifier(tt.spec)
			require.NoError(t, err)
			assert.Equal(t, tt.want, s.Contains(tt.item))

			ss, err := NewSpecifiers(tt.spec)
			require.NoError(t, err)
			assert.Equal(t, tt.want, ss.Contains(tt.item))
		})
	}
}

// ContainsInstalled is upstream's installed=True carve-out: what is already on
// disk is recognized even when the set would not offer it as a new candidate.
func TestSpecifiers_ContainsInstalled(t *testing.T) {
	tests := []struct {
		spec          string
		item          string
		wantContains  bool
		wantInstalled bool
	}{
		{">=1.0,<2", "1.5a1", false, true},
		{">=1.0", "2.0a1", false, true},
		{"<1.0", "0.9a1", false, true},
		// A final release is unaffected either way.
		{"==1.0", "1.0", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.spec+"__"+tt.item, func(t *testing.T) {
			ss, err := NewSpecifiers(tt.spec, WithPreReleases(PreReleasesExclude))
			require.NoError(t, err)
			assert.Equal(t, tt.wantContains, ss.Contains(tt.item), "Contains")
			assert.Equal(t, tt.wantInstalled, ss.ContainsInstalled(tt.item), "ContainsInstalled")
		})
	}
}

// TestContainsMatchesTheReference drives Contains against an INDEPENDENT oracle:
// 300 (specifier, policy, item) combinations whose expected values were measured
// against pypa/packaging 26.2.
//
// ⚠️ This replaces a test called TestContainsAgreesWithFilterOfOne, which was
// TAUTOLOGICAL and could not fail for any input. Contains IS
// `len(Filter([]string{item})) > 0` -- that is its implementation -- and the
// test asserted that expression against itself. It was billed as a cross-check
// and its comment even conceded the two "can never disagree", which should have
// been the tell: a check whose two sides are the same expression measures
// nothing. A real cross-check needs an oracle that is not the implementation.
//
// The Contains/Filter consistency the old test was reaching for is now a
// property of the code rather than of a test: Contains has no logic of its own,
// it delegates. What is worth testing is whether the delegation gives the
// answers the reference gives, which is what this does.
func TestContainsMatchesTheReference(t *testing.T) {
	for _, tt := range containsOracle {
		t.Run(fmt.Sprintf("%q__%s__%q", tt.spec, tt.policy, tt.item), func(t *testing.T) {
			ss, err := NewSpecifiers(tt.spec, WithPreReleases(tt.policy))
			require.NoError(t, err)
			assert.Equal(t, tt.want, ss.Contains(tt.item))
		})
	}
}

func emptyToNil(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}

// 300 rows, expected values measured against pypa/packaging 26.2.
var containsOracle = []struct {
	spec   string
	policy PreReleases
	item   string
	want   bool
}{
	{"", PreReleasesAuto, "1.0", true},
	{"", PreReleasesAuto, "1.5", true},
	{"", PreReleasesAuto, "2.0", true},
	{"", PreReleasesAuto, "1.5a1", true},
	{"", PreReleasesAuto, "2.0.dev1", true},
	{"", PreReleasesAuto, "lolwat", true},
	{"", PreReleasesAuto, "not a version", true},
	{"", PreReleasesAuto, "0.5", true},
	{"", PreReleasesAuto, "1.4.9", true},
	{"", PreReleasesAuto, "1!1.0", true},
	{"", PreReleasesInclude, "1.0", true},
	{"", PreReleasesInclude, "1.5", true},
	{"", PreReleasesInclude, "2.0", true},
	{"", PreReleasesInclude, "1.5a1", true},
	{"", PreReleasesInclude, "2.0.dev1", true},
	{"", PreReleasesInclude, "lolwat", true},
	{"", PreReleasesInclude, "not a version", true},
	{"", PreReleasesInclude, "0.5", true},
	{"", PreReleasesInclude, "1.4.9", true},
	{"", PreReleasesInclude, "1!1.0", true},
	{"", PreReleasesExclude, "1.0", true},
	{"", PreReleasesExclude, "1.5", true},
	{"", PreReleasesExclude, "2.0", true},
	{"", PreReleasesExclude, "1.5a1", false},
	{"", PreReleasesExclude, "2.0.dev1", false},
	{"", PreReleasesExclude, "lolwat", true},
	{"", PreReleasesExclude, "not a version", true},
	{"", PreReleasesExclude, "0.5", true},
	{"", PreReleasesExclude, "1.4.9", true},
	{"", PreReleasesExclude, "1!1.0", true},
	{">=1.0", PreReleasesAuto, "1.0", true},
	{">=1.0", PreReleasesAuto, "1.5", true},
	{">=1.0", PreReleasesAuto, "2.0", true},
	{">=1.0", PreReleasesAuto, "1.5a1", true},
	{">=1.0", PreReleasesAuto, "2.0.dev1", true},
	{">=1.0", PreReleasesAuto, "lolwat", false},
	{">=1.0", PreReleasesAuto, "not a version", false},
	{">=1.0", PreReleasesAuto, "0.5", false},
	{">=1.0", PreReleasesAuto, "1.4.9", true},
	{">=1.0", PreReleasesAuto, "1!1.0", true},
	{">=1.0", PreReleasesInclude, "1.0", true},
	{">=1.0", PreReleasesInclude, "1.5", true},
	{">=1.0", PreReleasesInclude, "2.0", true},
	{">=1.0", PreReleasesInclude, "1.5a1", true},
	{">=1.0", PreReleasesInclude, "2.0.dev1", true},
	{">=1.0", PreReleasesInclude, "lolwat", false},
	{">=1.0", PreReleasesInclude, "not a version", false},
	{">=1.0", PreReleasesInclude, "0.5", false},
	{">=1.0", PreReleasesInclude, "1.4.9", true},
	{">=1.0", PreReleasesInclude, "1!1.0", true},
	{">=1.0", PreReleasesExclude, "1.0", true},
	{">=1.0", PreReleasesExclude, "1.5", true},
	{">=1.0", PreReleasesExclude, "2.0", true},
	{">=1.0", PreReleasesExclude, "1.5a1", false},
	{">=1.0", PreReleasesExclude, "2.0.dev1", false},
	{">=1.0", PreReleasesExclude, "lolwat", false},
	{">=1.0", PreReleasesExclude, "not a version", false},
	{">=1.0", PreReleasesExclude, "0.5", false},
	{">=1.0", PreReleasesExclude, "1.4.9", true},
	{">=1.0", PreReleasesExclude, "1!1.0", true},
	{"<2", PreReleasesAuto, "1.0", true},
	{"<2", PreReleasesAuto, "1.5", true},
	{"<2", PreReleasesAuto, "2.0", false},
	{"<2", PreReleasesAuto, "1.5a1", true},
	{"<2", PreReleasesAuto, "2.0.dev1", false},
	{"<2", PreReleasesAuto, "lolwat", false},
	{"<2", PreReleasesAuto, "not a version", false},
	{"<2", PreReleasesAuto, "0.5", true},
	{"<2", PreReleasesAuto, "1.4.9", true},
	{"<2", PreReleasesAuto, "1!1.0", false},
	{"<2", PreReleasesInclude, "1.0", true},
	{"<2", PreReleasesInclude, "1.5", true},
	{"<2", PreReleasesInclude, "2.0", false},
	{"<2", PreReleasesInclude, "1.5a1", true},
	{"<2", PreReleasesInclude, "2.0.dev1", false},
	{"<2", PreReleasesInclude, "lolwat", false},
	{"<2", PreReleasesInclude, "not a version", false},
	{"<2", PreReleasesInclude, "0.5", true},
	{"<2", PreReleasesInclude, "1.4.9", true},
	{"<2", PreReleasesInclude, "1!1.0", false},
	{"<2", PreReleasesExclude, "1.0", true},
	{"<2", PreReleasesExclude, "1.5", true},
	{"<2", PreReleasesExclude, "2.0", false},
	{"<2", PreReleasesExclude, "1.5a1", false},
	{"<2", PreReleasesExclude, "2.0.dev1", false},
	{"<2", PreReleasesExclude, "lolwat", false},
	{"<2", PreReleasesExclude, "not a version", false},
	{"<2", PreReleasesExclude, "0.5", true},
	{"<2", PreReleasesExclude, "1.4.9", true},
	{"<2", PreReleasesExclude, "1!1.0", false},
	{"==2.0.*", PreReleasesAuto, "1.0", false},
	{"==2.0.*", PreReleasesAuto, "1.5", false},
	{"==2.0.*", PreReleasesAuto, "2.0", true},
	{"==2.0.*", PreReleasesAuto, "1.5a1", false},
	{"==2.0.*", PreReleasesAuto, "2.0.dev1", true},
	{"==2.0.*", PreReleasesAuto, "lolwat", false},
	{"==2.0.*", PreReleasesAuto, "not a version", false},
	{"==2.0.*", PreReleasesAuto, "0.5", false},
	{"==2.0.*", PreReleasesAuto, "1.4.9", false},
	{"==2.0.*", PreReleasesAuto, "1!1.0", false},
	{"==2.0.*", PreReleasesInclude, "1.0", false},
	{"==2.0.*", PreReleasesInclude, "1.5", false},
	{"==2.0.*", PreReleasesInclude, "2.0", true},
	{"==2.0.*", PreReleasesInclude, "1.5a1", false},
	{"==2.0.*", PreReleasesInclude, "2.0.dev1", true},
	{"==2.0.*", PreReleasesInclude, "lolwat", false},
	{"==2.0.*", PreReleasesInclude, "not a version", false},
	{"==2.0.*", PreReleasesInclude, "0.5", false},
	{"==2.0.*", PreReleasesInclude, "1.4.9", false},
	{"==2.0.*", PreReleasesInclude, "1!1.0", false},
	{"==2.0.*", PreReleasesExclude, "1.0", false},
	{"==2.0.*", PreReleasesExclude, "1.5", false},
	{"==2.0.*", PreReleasesExclude, "2.0", true},
	{"==2.0.*", PreReleasesExclude, "1.5a1", false},
	{"==2.0.*", PreReleasesExclude, "2.0.dev1", false},
	{"==2.0.*", PreReleasesExclude, "lolwat", false},
	{"==2.0.*", PreReleasesExclude, "not a version", false},
	{"==2.0.*", PreReleasesExclude, "0.5", false},
	{"==2.0.*", PreReleasesExclude, "1.4.9", false},
	{"==2.0.*", PreReleasesExclude, "1!1.0", false},
	{"!=1.5", PreReleasesAuto, "1.0", true},
	{"!=1.5", PreReleasesAuto, "1.5", false},
	{"!=1.5", PreReleasesAuto, "2.0", true},
	{"!=1.5", PreReleasesAuto, "1.5a1", true},
	{"!=1.5", PreReleasesAuto, "2.0.dev1", true},
	{"!=1.5", PreReleasesAuto, "lolwat", false},
	{"!=1.5", PreReleasesAuto, "not a version", false},
	{"!=1.5", PreReleasesAuto, "0.5", true},
	{"!=1.5", PreReleasesAuto, "1.4.9", true},
	{"!=1.5", PreReleasesAuto, "1!1.0", true},
	{"!=1.5", PreReleasesInclude, "1.0", true},
	{"!=1.5", PreReleasesInclude, "1.5", false},
	{"!=1.5", PreReleasesInclude, "2.0", true},
	{"!=1.5", PreReleasesInclude, "1.5a1", true},
	{"!=1.5", PreReleasesInclude, "2.0.dev1", true},
	{"!=1.5", PreReleasesInclude, "lolwat", false},
	{"!=1.5", PreReleasesInclude, "not a version", false},
	{"!=1.5", PreReleasesInclude, "0.5", true},
	{"!=1.5", PreReleasesInclude, "1.4.9", true},
	{"!=1.5", PreReleasesInclude, "1!1.0", true},
	{"!=1.5", PreReleasesExclude, "1.0", true},
	{"!=1.5", PreReleasesExclude, "1.5", false},
	{"!=1.5", PreReleasesExclude, "2.0", true},
	{"!=1.5", PreReleasesExclude, "1.5a1", false},
	{"!=1.5", PreReleasesExclude, "2.0.dev1", false},
	{"!=1.5", PreReleasesExclude, "lolwat", false},
	{"!=1.5", PreReleasesExclude, "not a version", false},
	{"!=1.5", PreReleasesExclude, "0.5", true},
	{"!=1.5", PreReleasesExclude, "1.4.9", true},
	{"!=1.5", PreReleasesExclude, "1!1.0", true},
	{"~=1.4", PreReleasesAuto, "1.0", false},
	{"~=1.4", PreReleasesAuto, "1.5", true},
	{"~=1.4", PreReleasesAuto, "2.0", false},
	{"~=1.4", PreReleasesAuto, "1.5a1", true},
	{"~=1.4", PreReleasesAuto, "2.0.dev1", false},
	{"~=1.4", PreReleasesAuto, "lolwat", false},
	{"~=1.4", PreReleasesAuto, "not a version", false},
	{"~=1.4", PreReleasesAuto, "0.5", false},
	{"~=1.4", PreReleasesAuto, "1.4.9", true},
	{"~=1.4", PreReleasesAuto, "1!1.0", false},
	{"~=1.4", PreReleasesInclude, "1.0", false},
	{"~=1.4", PreReleasesInclude, "1.5", true},
	{"~=1.4", PreReleasesInclude, "2.0", false},
	{"~=1.4", PreReleasesInclude, "1.5a1", true},
	{"~=1.4", PreReleasesInclude, "2.0.dev1", false},
	{"~=1.4", PreReleasesInclude, "lolwat", false},
	{"~=1.4", PreReleasesInclude, "not a version", false},
	{"~=1.4", PreReleasesInclude, "0.5", false},
	{"~=1.4", PreReleasesInclude, "1.4.9", true},
	{"~=1.4", PreReleasesInclude, "1!1.0", false},
	{"~=1.4", PreReleasesExclude, "1.0", false},
	{"~=1.4", PreReleasesExclude, "1.5", true},
	{"~=1.4", PreReleasesExclude, "2.0", false},
	{"~=1.4", PreReleasesExclude, "1.5a1", false},
	{"~=1.4", PreReleasesExclude, "2.0.dev1", false},
	{"~=1.4", PreReleasesExclude, "lolwat", false},
	{"~=1.4", PreReleasesExclude, "not a version", false},
	{"~=1.4", PreReleasesExclude, "0.5", false},
	{"~=1.4", PreReleasesExclude, "1.4.9", true},
	{"~=1.4", PreReleasesExclude, "1!1.0", false},
	{">=1.0.dev1", PreReleasesAuto, "1.0", true},
	{">=1.0.dev1", PreReleasesAuto, "1.5", true},
	{">=1.0.dev1", PreReleasesAuto, "2.0", true},
	{">=1.0.dev1", PreReleasesAuto, "1.5a1", true},
	{">=1.0.dev1", PreReleasesAuto, "2.0.dev1", true},
	{">=1.0.dev1", PreReleasesAuto, "lolwat", false},
	{">=1.0.dev1", PreReleasesAuto, "not a version", false},
	{">=1.0.dev1", PreReleasesAuto, "0.5", false},
	{">=1.0.dev1", PreReleasesAuto, "1.4.9", true},
	{">=1.0.dev1", PreReleasesAuto, "1!1.0", true},
	{">=1.0.dev1", PreReleasesInclude, "1.0", true},
	{">=1.0.dev1", PreReleasesInclude, "1.5", true},
	{">=1.0.dev1", PreReleasesInclude, "2.0", true},
	{">=1.0.dev1", PreReleasesInclude, "1.5a1", true},
	{">=1.0.dev1", PreReleasesInclude, "2.0.dev1", true},
	{">=1.0.dev1", PreReleasesInclude, "lolwat", false},
	{">=1.0.dev1", PreReleasesInclude, "not a version", false},
	{">=1.0.dev1", PreReleasesInclude, "0.5", false},
	{">=1.0.dev1", PreReleasesInclude, "1.4.9", true},
	{">=1.0.dev1", PreReleasesInclude, "1!1.0", true},
	{">=1.0.dev1", PreReleasesExclude, "1.0", true},
	{">=1.0.dev1", PreReleasesExclude, "1.5", true},
	{">=1.0.dev1", PreReleasesExclude, "2.0", true},
	{">=1.0.dev1", PreReleasesExclude, "1.5a1", false},
	{">=1.0.dev1", PreReleasesExclude, "2.0.dev1", false},
	{">=1.0.dev1", PreReleasesExclude, "lolwat", false},
	{">=1.0.dev1", PreReleasesExclude, "not a version", false},
	{">=1.0.dev1", PreReleasesExclude, "0.5", false},
	{">=1.0.dev1", PreReleasesExclude, "1.4.9", true},
	{">=1.0.dev1", PreReleasesExclude, "1!1.0", true},
	{"===lolwat", PreReleasesAuto, "1.0", false},
	{"===lolwat", PreReleasesAuto, "1.5", false},
	{"===lolwat", PreReleasesAuto, "2.0", false},
	{"===lolwat", PreReleasesAuto, "1.5a1", false},
	{"===lolwat", PreReleasesAuto, "2.0.dev1", false},
	{"===lolwat", PreReleasesAuto, "lolwat", true},
	{"===lolwat", PreReleasesAuto, "not a version", false},
	{"===lolwat", PreReleasesAuto, "0.5", false},
	{"===lolwat", PreReleasesAuto, "1.4.9", false},
	{"===lolwat", PreReleasesAuto, "1!1.0", false},
	{"===lolwat", PreReleasesInclude, "1.0", false},
	{"===lolwat", PreReleasesInclude, "1.5", false},
	{"===lolwat", PreReleasesInclude, "2.0", false},
	{"===lolwat", PreReleasesInclude, "1.5a1", false},
	{"===lolwat", PreReleasesInclude, "2.0.dev1", false},
	{"===lolwat", PreReleasesInclude, "lolwat", true},
	{"===lolwat", PreReleasesInclude, "not a version", false},
	{"===lolwat", PreReleasesInclude, "0.5", false},
	{"===lolwat", PreReleasesInclude, "1.4.9", false},
	{"===lolwat", PreReleasesInclude, "1!1.0", false},
	{"===lolwat", PreReleasesExclude, "1.0", false},
	{"===lolwat", PreReleasesExclude, "1.5", false},
	{"===lolwat", PreReleasesExclude, "2.0", false},
	{"===lolwat", PreReleasesExclude, "1.5a1", false},
	{"===lolwat", PreReleasesExclude, "2.0.dev1", false},
	{"===lolwat", PreReleasesExclude, "lolwat", true},
	{"===lolwat", PreReleasesExclude, "not a version", false},
	{"===lolwat", PreReleasesExclude, "0.5", false},
	{"===lolwat", PreReleasesExclude, "1.4.9", false},
	{"===lolwat", PreReleasesExclude, "1!1.0", false},
	{">=1.0,<2.0", PreReleasesAuto, "1.0", true},
	{">=1.0,<2.0", PreReleasesAuto, "1.5", true},
	{">=1.0,<2.0", PreReleasesAuto, "2.0", false},
	{">=1.0,<2.0", PreReleasesAuto, "1.5a1", true},
	{">=1.0,<2.0", PreReleasesAuto, "2.0.dev1", false},
	{">=1.0,<2.0", PreReleasesAuto, "lolwat", false},
	{">=1.0,<2.0", PreReleasesAuto, "not a version", false},
	{">=1.0,<2.0", PreReleasesAuto, "0.5", false},
	{">=1.0,<2.0", PreReleasesAuto, "1.4.9", true},
	{">=1.0,<2.0", PreReleasesAuto, "1!1.0", false},
	{">=1.0,<2.0", PreReleasesInclude, "1.0", true},
	{">=1.0,<2.0", PreReleasesInclude, "1.5", true},
	{">=1.0,<2.0", PreReleasesInclude, "2.0", false},
	{">=1.0,<2.0", PreReleasesInclude, "1.5a1", true},
	{">=1.0,<2.0", PreReleasesInclude, "2.0.dev1", false},
	{">=1.0,<2.0", PreReleasesInclude, "lolwat", false},
	{">=1.0,<2.0", PreReleasesInclude, "not a version", false},
	{">=1.0,<2.0", PreReleasesInclude, "0.5", false},
	{">=1.0,<2.0", PreReleasesInclude, "1.4.9", true},
	{">=1.0,<2.0", PreReleasesInclude, "1!1.0", false},
	{">=1.0,<2.0", PreReleasesExclude, "1.0", true},
	{">=1.0,<2.0", PreReleasesExclude, "1.5", true},
	{">=1.0,<2.0", PreReleasesExclude, "2.0", false},
	{">=1.0,<2.0", PreReleasesExclude, "1.5a1", false},
	{">=1.0,<2.0", PreReleasesExclude, "2.0.dev1", false},
	{">=1.0,<2.0", PreReleasesExclude, "lolwat", false},
	{">=1.0,<2.0", PreReleasesExclude, "not a version", false},
	{">=1.0,<2.0", PreReleasesExclude, "0.5", false},
	{">=1.0,<2.0", PreReleasesExclude, "1.4.9", true},
	{">=1.0,<2.0", PreReleasesExclude, "1!1.0", false},
	{"!=1.0,>=0.5", PreReleasesAuto, "1.0", false},
	{"!=1.0,>=0.5", PreReleasesAuto, "1.5", true},
	{"!=1.0,>=0.5", PreReleasesAuto, "2.0", true},
	{"!=1.0,>=0.5", PreReleasesAuto, "1.5a1", true},
	{"!=1.0,>=0.5", PreReleasesAuto, "2.0.dev1", true},
	{"!=1.0,>=0.5", PreReleasesAuto, "lolwat", false},
	{"!=1.0,>=0.5", PreReleasesAuto, "not a version", false},
	{"!=1.0,>=0.5", PreReleasesAuto, "0.5", true},
	{"!=1.0,>=0.5", PreReleasesAuto, "1.4.9", true},
	{"!=1.0,>=0.5", PreReleasesAuto, "1!1.0", true},
	{"!=1.0,>=0.5", PreReleasesInclude, "1.0", false},
	{"!=1.0,>=0.5", PreReleasesInclude, "1.5", true},
	{"!=1.0,>=0.5", PreReleasesInclude, "2.0", true},
	{"!=1.0,>=0.5", PreReleasesInclude, "1.5a1", true},
	{"!=1.0,>=0.5", PreReleasesInclude, "2.0.dev1", true},
	{"!=1.0,>=0.5", PreReleasesInclude, "lolwat", false},
	{"!=1.0,>=0.5", PreReleasesInclude, "not a version", false},
	{"!=1.0,>=0.5", PreReleasesInclude, "0.5", true},
	{"!=1.0,>=0.5", PreReleasesInclude, "1.4.9", true},
	{"!=1.0,>=0.5", PreReleasesInclude, "1!1.0", true},
	{"!=1.0,>=0.5", PreReleasesExclude, "1.0", false},
	{"!=1.0,>=0.5", PreReleasesExclude, "1.5", true},
	{"!=1.0,>=0.5", PreReleasesExclude, "2.0", true},
	{"!=1.0,>=0.5", PreReleasesExclude, "1.5a1", false},
	{"!=1.0,>=0.5", PreReleasesExclude, "2.0.dev1", false},
	{"!=1.0,>=0.5", PreReleasesExclude, "lolwat", false},
	{"!=1.0,>=0.5", PreReleasesExclude, "not a version", false},
	{"!=1.0,>=0.5", PreReleasesExclude, "0.5", true},
	{"!=1.0,>=0.5", PreReleasesExclude, "1.4.9", true},
	{"!=1.0,>=0.5", PreReleasesExclude, "1!1.0", true},
}
