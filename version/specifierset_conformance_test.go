// SPDX-License-Identifier: Apache-2.0 OR MIT

// Conformance tables ported from the TestSpecifierSet class of
// pypa/packaging's tests/test_specifiers.py, one Go table per upstream test
// function. See specifier_conformance_test.go for the porting conventions and
// the list of intentional divergences.
//
// Upstream tests NOT ported from this class, and why:
//
//	test_is_subset / test_is_superset / test_is_disjoint / TestIsUnsatisfiable
//	              - these rest on packaging 26.2's interval subsystem
//	                (_to_ranges / _intersect_ranges / _LowerBound /
//	                _UpperBound), which has no counterpart in this module.
//	                Set relations and unsatisfiability are their own design
//	                exercise, not accessors; deliberately out of scope.
//	test_pickle_* - Go has no pickle.
//	test_match_args, test_specifiers_hash, *_construction_is_lazy,
//	TestSpecifierInternal.*, TestSpecifierSetToRangeEquivalence.*
//	              - Python-implementation internals (__match_args__, __hash__,
//	                one-element caches, laziness, to_range) with no exported
//	                Go surface to assert against.
//
// Upstream pinned at 4eb0753dba8fcaaac8eb75463374e448f0931558.

package version

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ported from TestSpecifierSet.test_empty_specifier_arbitrary_string.
//
// An empty set admits an arbitrary string unconditionally: there is no way to
// tell whether an opaque token is a pre-release, so the pre-release preference
// cannot apply to it. Note the third block -- with prereleases=False the
// arbitrary string survives and the pre-release does not.
func TestConformance_EmptySetArbitraryString(t *testing.T) {
	tests := []struct {
		pre   PreReleases
		input []string
		want  []string
	}{
		{PreReleasesAuto, []string{"foobar"}, []string{"foobar"}},
		{PreReleasesExclude, []string{"foobar"}, []string{"foobar"}},
		{PreReleasesInclude, []string{"foobar"}, []string{"foobar"}},
		{PreReleasesAuto, []string{"foobar", "1.0"}, []string{"foobar", "1.0"}},
		{PreReleasesExclude, []string{"foobar", "1.0"}, []string{"foobar", "1.0"}},
		{PreReleasesInclude, []string{"foobar", "1.0"}, []string{"foobar", "1.0"}},
		{PreReleasesAuto, []string{"foobar", "1.0a1"}, []string{"foobar", "1.0a1"}},
		{PreReleasesExclude, []string{"foobar", "1.0a1"}, []string{"foobar"}},
		{PreReleasesInclude, []string{"foobar", "1.0a1"}, []string{"foobar", "1.0a1"}},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s__%v", tt.pre, tt.input), func(t *testing.T) {
			ss, err := NewSpecifiers("", WithPreReleases(tt.pre))
			require.NoError(t, err)
			assert.True(t, ss.Contains("foobar"), "an empty set admits any string")
			assert.Equal(t, tt.want, ss.Filter(tt.input))
		})
	}
}

// Ported from TestSpecifierSet.test_empty_specifier_ordering_with_arbitrary:
// the preference must not reorder what it keeps.
func TestConformance_EmptySetOrderingWithArbitrary(t *testing.T) {
	tests := []struct{ input, want []string }{
		{[]string{"1.0a1", "foobar", "1.0", "2.0a1", "bazqux", "2.0"}, []string{"foobar", "1.0", "bazqux", "2.0"}},
		{[]string{"foobar", "1.0", "bazqux", "2.0", "hello"}, []string{"foobar", "1.0", "bazqux", "2.0", "hello"}},
		{[]string{"1.0a1", "foobar", "2.0a1", "bazqux", "3.0a1"}, []string{"1.0a1", "foobar", "2.0a1", "bazqux", "3.0a1"}},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.input), func(t *testing.T) {
			ss, err := NewSpecifiers("")
			require.NoError(t, err)
			assert.Equal(t, tt.want, ss.Filter(tt.input))
		})
	}
}

// Ported from TestSpecifierSet.test_create_from_specifiers.
func TestConformance_CreateFromSpecifiers(t *testing.T) {
	specStrs := []string{">=1.0", "!=1.1", "!=1.2", "<2.0"}
	var specs []Specifier
	for _, s := range specStrs {
		parsed, err := NewSpecifier(s)
		require.NoError(t, err)
		specs = append(specs, parsed)
	}

	ss := NewSpecifiersFrom(specs)
	assert.Equal(t, len(specStrs), ss.Len())

	var got []string
	for _, s := range ss.List() {
		got = append(got, s.String())
	}
	assert.ElementsMatch(t, specStrs, got)
}

// Ported from TestSpecifierSet.test_specifier_prereleases_explicit.
//
// Upstream MUTATES `spec.prereleases` between the two assertions. Specifiers
// is a value type here, so the second half is expressed as a second set built
// with the new policy -- which is the same question, asked without mutation.
func TestConformance_SetPreReleasesExplicit(t *testing.T) {
	tests := []struct {
		spec        string
		initialPre  PreReleases
		finalPre    PreReleases
		version     string
		wantInitial bool
		wantFinal   bool
	}{
		{"", PreReleasesAuto, PreReleasesInclude, "1.0.dev1", true, true},
		{"", PreReleasesExclude, PreReleasesInclude, "1.0.dev1", false, true},
		{"", PreReleasesInclude, PreReleasesExclude, "1.0.dev1", true, false},
		{">=1.0", PreReleasesInclude, PreReleasesExclude, "1.0.dev1", false, false},
		{"==1.*", PreReleasesInclude, PreReleasesExclude, "1.0.dev1", true, false},
		{"", PreReleasesExclude, PreReleasesAuto, "1.0.dev1", false, true},
		{">=1.0", PreReleasesExclude, PreReleasesAuto, "2.0.dev1", false, true},
		{"", PreReleasesInclude, PreReleasesAuto, "1.0.dev1", true, true},
		{">=1.0", PreReleasesInclude, PreReleasesAuto, "2.0.dev1", true, true},
		{"", PreReleasesAuto, PreReleasesInclude, "2.0b1", true, true},
		{"", PreReleasesAuto, PreReleasesExclude, "2.0a1", true, false},
		{"", PreReleasesInclude, PreReleasesExclude, "1.0rc1", true, false},
		{"", PreReleasesExclude, PreReleasesInclude, "1.0.post1.dev1", false, true},
		{"==2.0.dev1", PreReleasesAuto, PreReleasesExclude, "2.0.dev1", true, false},
		{"==1.0.*", PreReleasesInclude, PreReleasesExclude, "1.0.dev1", true, false},
		{"!=2.0", PreReleasesExclude, PreReleasesInclude, "1.0.dev1", false, true},
		{">=1.0,<2.0", PreReleasesAuto, PreReleasesInclude, "1.5a1", true, true},
		{">=1.0,<2.0", PreReleasesExclude, PreReleasesInclude, "1.5b1", false, true},
		{">=1.0,<2.0", PreReleasesInclude, PreReleasesExclude, "1.5rc1", true, false},
		{"", PreReleasesAuto, PreReleasesInclude, "1.0a1", true, true},
		{"", PreReleasesAuto, PreReleasesInclude, "1.0b2", true, true},
		{"", PreReleasesAuto, PreReleasesInclude, "1.0rc3", true, true},
		{"", PreReleasesAuto, PreReleasesInclude, "1.0.dev4", true, true},
		{">=1.0a1", PreReleasesAuto, PreReleasesExclude, "1.0a1", true, false},
		{"<1.0.dev1", PreReleasesAuto, PreReleasesExclude, "0.9.dev0", true, false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q__%s__%s__%s", tt.spec, tt.initialPre, tt.finalPre, tt.version), func(t *testing.T) {
			initial, err := NewSpecifiers(tt.spec, WithPreReleases(tt.initialPre))
			require.NoError(t, err)
			assert.Equal(t, tt.wantInitial, initial.Contains(tt.version), "initial policy")

			final, err := NewSpecifiers(tt.spec, WithPreReleases(tt.finalPre))
			require.NoError(t, err)
			assert.Equal(t, tt.wantFinal, final.Contains(tt.version), "final policy")

			// ⚠️ A per-call EXPLICIT policy reaches the same answer as rebuilding
			// the set with it. A per-call PreReleasesAuto does not, and must not:
			// Auto means "no opinion, defer to the set", which is upstream's
			// `contains(item, prereleases=None)` -- NOT upstream's
			// `spec.prereleases = None`, which resets the set's own policy. Those
			// are different operations, and only the second one can undo an
			// explicit constructor policy. Rebuilding the set is how you do that
			// here, which is exactly what `final` above is.
			if tt.finalPre != PreReleasesAuto {
				assert.Equal(t, tt.wantFinal, initial.Contains(tt.version, WithPreReleases(tt.finalPre)),
					"an explicit per-call policy must agree with a rebuilt set")
			}
		})
	}
}

// Ported from TestSpecifierSet.test_specifier_contains_prereleases.
func TestConformance_SetContainsPreReleases(t *testing.T) {
	auto, err := NewSpecifiers("")
	require.NoError(t, err)
	assert.Equal(t, PreReleasesAuto, auto.PreReleases())
	assert.True(t, auto.Contains("1.0.dev1"))
	assert.True(t, auto.Contains("1.0.dev1", WithPreReleases(PreReleasesInclude)))

	incl, err := NewSpecifiers("", WithPreReleases(PreReleasesInclude))
	require.NoError(t, err)
	assert.Equal(t, PreReleasesInclude, incl.PreReleases())
	assert.True(t, incl.Contains("1.0.dev1"))
	assert.False(t, incl.Contains("1.0.dev1", WithPreReleases(PreReleasesExclude)),
		"a per-call policy must override the constructor policy")
}

// Ported from TestSpecifierSet.test_specifier_contains_installed_prereleases:
// the `installed=True` carve-out, which accepts an already-installed
// pre-release the set would not offer as a NEW candidate.
func TestConformance_SetContainsInstalledPreReleases(t *testing.T) {
	tests := []struct {
		spec      string
		version   string
		specPre   PreReleases
		callPre   PreReleases
		installed bool
		want      bool
	}{
		{"~=1.0", "1.1.0.dev1", PreReleasesAuto, PreReleasesAuto, true, true},
		{"~=1.0", "1.1.0.dev1", PreReleasesExclude, PreReleasesExclude, true, true},
		{"~=1.0", "1.1.0.dev1", PreReleasesInclude, PreReleasesExclude, true, true},
		{"~=1.0", "1.1.0.dev1", PreReleasesAuto, PreReleasesExclude, true, true},
		{"~=1.0", "1.1.0.dev1", PreReleasesAuto, PreReleasesInclude, false, true},
		{"~=1.0", "1.1.0.dev1", PreReleasesExclude, PreReleasesInclude, false, true},
		{"~=1.0", "1.1.0.dev1", PreReleasesExclude, PreReleasesExclude, false, false},
		{"~=1.0", "1.1.0.dev1", PreReleasesAuto, PreReleasesExclude, false, false},
		{"~=1.0", "1.1.0a1", PreReleasesAuto, PreReleasesAuto, true, true},
		{"~=1.0", "1.1.0b1", PreReleasesAuto, PreReleasesAuto, true, true},
		{"~=1.0", "1.1.0rc1", PreReleasesAuto, PreReleasesAuto, true, true},
		{"~=1.0", "1.1.0.post1.dev1", PreReleasesAuto, PreReleasesAuto, true, true},
		{">=1.0", "2.0.dev1", PreReleasesAuto, PreReleasesAuto, true, true},
		{"==1.*", "1.5.0a1", PreReleasesAuto, PreReleasesAuto, true, true},
		{">=1.0,<3.0", "2.0.dev1", PreReleasesAuto, PreReleasesAuto, true, true},
		{"!=2.0", "2.0.dev1", PreReleasesAuto, PreReleasesAuto, true, true},
		{"~=1.0", "3.0.0.dev1", PreReleasesAuto, PreReleasesAuto, true, false},
		{"~=1.0", "3.0.0.dev1", PreReleasesInclude, PreReleasesAuto, true, false},
		{"~=1.0", "3.0.0.dev1", PreReleasesAuto, PreReleasesInclude, true, false},
		{"~=1.0", "3.0.0.dev1", PreReleasesInclude, PreReleasesInclude, true, false},
		{"~=1.0", "3.0.0.dev1", PreReleasesExclude, PreReleasesExclude, true, false},
		{"~=1.0", "3.0.0.dev1", PreReleasesAuto, PreReleasesAuto, false, false},
		{">=2.0", "1.9.0.dev1", PreReleasesInclude, PreReleasesAuto, true, false},
		{">=2.0", "1.9.0.dev1", PreReleasesAuto, PreReleasesInclude, true, false},
		{">=2.0", "1.9.0.dev1", PreReleasesInclude, PreReleasesInclude, true, false},
		{">=2.0", "1.9.0.dev1", PreReleasesAuto, PreReleasesAuto, false, false},
		{">=1.0", "1.0.0.dev1", PreReleasesAuto, PreReleasesAuto, true, false},
		{"<=1.0", "1.0.0.dev1", PreReleasesAuto, PreReleasesAuto, true, true},
		{"<1.0", "1.0.0.dev1", PreReleasesAuto, PreReleasesAuto, true, false},
		{"<1.0", "0.9.0.dev1", PreReleasesAuto, PreReleasesAuto, true, true},
		{">=1.0.dev1", "1.0.0.dev1", PreReleasesAuto, PreReleasesAuto, true, true},
		{">=1.0.dev1", "1.0.0.dev1", PreReleasesExclude, PreReleasesExclude, false, false},
		{"==1.0.0.dev1", "1.0.0.dev1", PreReleasesExclude, PreReleasesExclude, false, false},
		{"~=1.0", "1.1.0", PreReleasesAuto, PreReleasesAuto, true, true},
		{"~=1.0", "1.1.0", PreReleasesExclude, PreReleasesExclude, false, true},
		{"~=1.0", "1.1.0", PreReleasesInclude, PreReleasesExclude, false, true},
		{"~=1.0", "1.1.0.dev1", PreReleasesInclude, PreReleasesAuto, false, true},
		{"~=1.0", "1.1.0.dev1", PreReleasesExclude, PreReleasesAuto, false, false},
		{"~=1.0", "1.1.0.dev1", PreReleasesInclude, PreReleasesExclude, false, false},
		{">=1.0.dev1", "1.0.0.dev1", PreReleasesAuto, PreReleasesExclude, false, false},
		{">=1.0.dev1", "1.0.0.dev1", PreReleasesExclude, PreReleasesAuto, false, false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s__%s__%s__%s__inst=%t", tt.spec, tt.version, tt.specPre, tt.callPre, tt.installed), func(t *testing.T) {
			ss, err := NewSpecifiers(tt.spec, WithPreReleases(tt.specPre))
			require.NoError(t, err)
			if tt.installed {
				assert.Equal(t, tt.want, ss.ContainsInstalled(tt.version, WithPreReleases(tt.callPre)))
				return
			}
			assert.Equal(t, tt.want, ss.Contains(tt.version, WithPreReleases(tt.callPre)))
		})
	}
}

// Ported from TestSpecifierSet.test_specifier_filter.
func TestConformance_SetFilter(t *testing.T) {
	tests := []struct {
		spec    string
		specPre PreReleases
		callPre PreReleases
		input   []string
		want    []string
	}{
		{"", PreReleasesAuto, PreReleasesAuto, []string{"1.0", "2.0a1"}, []string{"1.0"}},
		{">=1.0.dev1", PreReleasesAuto, PreReleasesAuto, []string{"1.0", "2.0a1"}, []string{"1.0", "2.0a1"}},
		{"", PreReleasesAuto, PreReleasesAuto, []string{"1.0a1"}, []string{"1.0a1"}},
		{">=1.2.3", PreReleasesAuto, PreReleasesAuto, []string{"1.2", "1.5a1"}, []string{"1.5a1"}},
		{">=1.2.3", PreReleasesAuto, PreReleasesAuto, []string{"1.3", "1.5a1"}, []string{"1.3"}},
		{"", PreReleasesAuto, PreReleasesAuto, []string{"1.0", "2.0"}, []string{"1.0", "2.0"}},
		{">=1.0", PreReleasesAuto, PreReleasesAuto, []string{"2.0a1"}, []string{"2.0a1"}},
		{"!=2.0a1", PreReleasesAuto, PreReleasesAuto, []string{"1.0a2", "1.0", "2.0a1"}, []string{"1.0"}},
		{"==2.0a1", PreReleasesAuto, PreReleasesAuto, []string{"2.0a1"}, []string{"2.0a1"}},
		{">2.0a1", PreReleasesAuto, PreReleasesAuto, []string{"2.0a1", "3.0a2", "3.0"}, []string{"3.0a2", "3.0"}},
		{"<2.0a1", PreReleasesAuto, PreReleasesAuto, []string{"1.0a2", "1.0", "2.0a1"}, []string{"1.0a2", "1.0"}},
		{"~=2.0a1", PreReleasesAuto, PreReleasesAuto, []string{"1.0", "2.0a1", "3.0a2", "3.0"}, []string{"2.0a1"}},
		{"", PreReleasesAuto, PreReleasesExclude, []string{"1.0a1"}, nil},
		{">=1.0.dev1", PreReleasesAuto, PreReleasesExclude, []string{"1.0", "2.0a1"}, []string{"1.0"}},
		{"", PreReleasesAuto, PreReleasesInclude, []string{"1.0", "2.0a1"}, []string{"1.0", "2.0a1"}},
		{"", PreReleasesInclude, PreReleasesAuto, []string{"1.0", "2.0a1"}, []string{"1.0", "2.0a1"}},
		{"", PreReleasesExclude, PreReleasesAuto, []string{"1.0", "2.0a1"}, []string{"1.0"}},
		{">=1.0.dev1", PreReleasesInclude, PreReleasesAuto, []string{"1.0", "2.0a1"}, []string{"1.0", "2.0a1"}},
		{">=1.0.dev1", PreReleasesExclude, PreReleasesAuto, []string{"1.0", "2.0a1"}, []string{"1.0"}},
		{"", PreReleasesInclude, PreReleasesAuto, []string{"1.0a1"}, []string{"1.0a1"}},
		{"", PreReleasesExclude, PreReleasesAuto, []string{"1.0a1"}, nil},
		{">=1.0", PreReleasesInclude, PreReleasesExclude, []string{"1.0", "2.0a1"}, []string{"1.0"}},
		{">=1.0", PreReleasesExclude, PreReleasesInclude, []string{"1.0", "2.0a1"}, []string{"1.0", "2.0a1"}},
		{">=1.0", PreReleasesInclude, PreReleasesInclude, []string{"1.0", "2.0a1"}, []string{"1.0", "2.0a1"}},
		{">=1.0", PreReleasesExclude, PreReleasesExclude, []string{"1.0", "2.0a1"}, []string{"1.0"}},
		{">=1.0,<=2.0", PreReleasesAuto, PreReleasesAuto, []string{"1.0", "1.5a1"}, []string{"1.0"}},
		{">=1.0,<=2.0dev", PreReleasesAuto, PreReleasesAuto, []string{"1.0", "1.5a1"}, []string{"1.0", "1.5a1"}},
		{">=1.0,<=2.0", PreReleasesInclude, PreReleasesAuto, []string{"1.0", "1.5a1"}, []string{"1.0", "1.5a1"}},
		{">=1.0,<=2.0", PreReleasesExclude, PreReleasesAuto, []string{"1.0", "1.5a1"}, []string{"1.0"}},
		{">=1.0,<=2.0dev", PreReleasesExclude, PreReleasesAuto, []string{"1.0", "1.5a1"}, []string{"1.0"}},
		{">=1.0,<=2.0dev", PreReleasesInclude, PreReleasesAuto, []string{"1.0", "1.5a1"}, []string{"1.0", "1.5a1"}},
		{">=1.0,<=2.0", PreReleasesAuto, PreReleasesExclude, []string{"1.0", "1.5a1"}, []string{"1.0"}},
		{">=1.0,<=2.0", PreReleasesAuto, PreReleasesInclude, []string{"1.0", "1.5a1"}, []string{"1.0", "1.5a1"}},
		{">=1.0,<=2.0dev", PreReleasesAuto, PreReleasesExclude, []string{"1.0", "1.5a1"}, []string{"1.0"}},
		{">=1.0,<=2.0dev", PreReleasesAuto, PreReleasesInclude, []string{"1.0", "1.5a1"}, []string{"1.0", "1.5a1"}},
		{">=1.0,<=2.0", PreReleasesInclude, PreReleasesExclude, []string{"1.0", "1.5a1"}, []string{"1.0"}},
		{">=1.0,<=2.0", PreReleasesExclude, PreReleasesInclude, []string{"1.0", "1.5a1"}, []string{"1.0", "1.5a1"}},
		{">=1.0,<=2.0dev", PreReleasesInclude, PreReleasesExclude, []string{"1.0", "1.5a1"}, []string{"1.0"}},
		{">=1.0,<=2.0dev", PreReleasesExclude, PreReleasesInclude, []string{"1.0", "1.5a1"}, []string{"1.0", "1.5a1"}},
		{"", PreReleasesAuto, PreReleasesAuto, []string{"invalid version"}, []string{"invalid version"}},
		{"", PreReleasesAuto, PreReleasesExclude, []string{"invalid version"}, []string{"invalid version"}},
		{"", PreReleasesExclude, PreReleasesAuto, []string{"invalid version"}, []string{"invalid version"}},
		{"", PreReleasesAuto, PreReleasesAuto, []string{"1.0", "invalid version"}, []string{"1.0", "invalid version"}},
		{"", PreReleasesAuto, PreReleasesExclude, []string{"1.0", "invalid version"}, []string{"1.0", "invalid version"}},
		{"", PreReleasesExclude, PreReleasesAuto, []string{"1.0", "invalid version"}, []string{"1.0", "invalid version"}},
		{"===foobar", PreReleasesAuto, PreReleasesAuto, []string{"foobar", "foo", "bar"}, []string{"foobar"}},
		{"===foobar", PreReleasesAuto, PreReleasesAuto, []string{"foo", "bar"}, nil},
		{"===1.0", PreReleasesAuto, PreReleasesAuto, []string{"1.0", "1.0.0", "2.0"}, []string{"1.0"}},
		{"===1.0", PreReleasesAuto, PreReleasesAuto, []string{"1.0", "1.0+downstream1"}, []string{"1.0"}},
		{">=1.0,===foobar", PreReleasesAuto, PreReleasesAuto, []string{"foobar", "1.0", "2.0"}, nil},
		{"!=2.0,===foobar", PreReleasesAuto, PreReleasesAuto, []string{"foobar", "2.0", "bar"}, nil},
		{">=1.0,===1.5", PreReleasesAuto, PreReleasesAuto, []string{"1.0", "1.5", "2.0"}, []string{"1.5"}},
		{">=2.0,===1.5", PreReleasesAuto, PreReleasesAuto, []string{"1.0", "1.5", "2.0"}, nil},
		{"===foobar", PreReleasesAuto, PreReleasesAuto, []string{"foobar", "1.0", "invalid", "2.0a1"}, []string{"foobar"}},
		{"===1.0", PreReleasesAuto, PreReleasesAuto, []string{"1.0", "foobar", "invalid", "1.0.0"}, []string{"1.0"}},
		{">=1.0,===1.5", PreReleasesAuto, PreReleasesAuto, []string{"1.5", "foobar", "invalid"}, []string{"1.5"}},
		{"!=1.0", PreReleasesAuto, PreReleasesAuto, []string{"invalid", "foobar"}, nil},
		{"!=1.0", PreReleasesAuto, PreReleasesAuto, []string{"1.0", "invalid", "2.0"}, []string{"2.0"}},
		{"!=2.0.*", PreReleasesAuto, PreReleasesAuto, []string{"invalid", "foobar", "2.0"}, nil},
		{"!=2.0.*", PreReleasesAuto, PreReleasesAuto, []string{"1.0", "invalid", "2.0.0"}, []string{"1.0"}},
		{"!=1.0,!=2.0", PreReleasesAuto, PreReleasesAuto, []string{"invalid", "1.0", "2.0", "3.0"}, []string{"3.0"}},
		{">=1.0,!=2.0", PreReleasesAuto, PreReleasesAuto, []string{"invalid", "1.0", "2.0", "3.0"}, []string{"1.0", "3.0"}},
		{"===foobar", PreReleasesAuto, PreReleasesInclude, []string{"foobar", "foo"}, []string{"foobar"}},
		{"===foobar", PreReleasesAuto, PreReleasesExclude, []string{"foobar", "foo"}, []string{"foobar"}},
		{"===foobar", PreReleasesInclude, PreReleasesAuto, []string{"foobar", "foo"}, []string{"foobar"}},
		{"===foobar", PreReleasesExclude, PreReleasesAuto, []string{"foobar", "foo"}, []string{"foobar"}},
		{"===foobar", PreReleasesInclude, PreReleasesInclude, []string{"foobar", "foo"}, []string{"foobar"}},
		{"===foobar", PreReleasesExclude, PreReleasesExclude, []string{"foobar", "foo"}, []string{"foobar"}},
		{"===foobar,===foobar", PreReleasesAuto, PreReleasesAuto, []string{"foobar", "foo", "bar"}, []string{"foobar"}},
		{"==1.*,>=1.0", PreReleasesAuto, PreReleasesAuto, []string{"0.9", "1.0", "1.5", "2.0"}, []string{"1.0", "1.5"}},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q__%s__%s__%v", tt.spec, tt.specPre, tt.callPre, tt.input), func(t *testing.T) {
			ss, err := NewSpecifiers(tt.spec, WithPreReleases(tt.specPre))
			require.NoError(t, err)
			got := ss.Filter(tt.input, WithPreReleases(tt.callPre))
			if len(tt.want) == 0 {
				assert.Empty(t, got)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

// Ported from TestSpecifierSet.test_specifierset_filter_with_key and
// test_empty_specifierset_filter_with_key -- upstream's `key=` parameter,
// which FilterBy provides.
func TestConformance_SetFilterWithKey(t *testing.T) {
	type item struct{ version string }

	nonEmpty := []item{{"1.0"}, {"2.1a1"}}
	emptySet := []item{{"2.0a1"}, {"2.1"}}
	key := func(i item) string { return i.version }

	nonEmptyCases := []struct {
		pre  PreReleases
		want []int
	}{
		{PreReleasesAuto, []int{1}},
		{PreReleasesInclude, []int{1}},
		{PreReleasesExclude, nil},
	}
	for _, tt := range nonEmptyCases {
		t.Run("ge2__"+tt.pre.String(), func(t *testing.T) {
			ss, err := NewSpecifiers(">=2")
			require.NoError(t, err)
			got := FilterBy(ss, nonEmpty, key, WithPreReleases(tt.pre))
			var want []item
			for _, i := range tt.want {
				want = append(want, nonEmpty[i])
			}
			if want == nil {
				assert.Empty(t, got)
				return
			}
			assert.Equal(t, want, got)
		})
	}

	emptyCases := []struct {
		pre  PreReleases
		want []int
	}{
		{PreReleasesAuto, []int{1}},
		{PreReleasesInclude, []int{0, 1}},
		{PreReleasesExclude, []int{1}},
	}
	for _, tt := range emptyCases {
		t.Run("empty__"+tt.pre.String(), func(t *testing.T) {
			ss, err := NewSpecifiers("", WithPreReleases(tt.pre))
			require.NoError(t, err)
			got := FilterBy(ss, emptySet, key)
			var want []item
			for _, i := range tt.want {
				want = append(want, emptySet[i])
			}
			assert.Equal(t, want, got)
		})
	}
}

// Ported from TestSpecifierSet.test_contains_exclusionary_bridges.
//
// When a set excludes whole ranges (!=1.*, !=2.*) the surviving region can
// contain only pre-release, dev or post versions. The pre-release preference
// must still offer those, because there is no final release in the gap to
// prefer -- which is the clearest demonstration that the rule is about
// SELECTION among candidates and not about what the operators match.
func TestConformance_SetContainsExclusionaryBridges(t *testing.T) {
	tests := []struct {
		spec    string
		pre     PreReleases
		version string
		want    bool
	}{
		{">=1,!=1.*,!=2.*,!=3.0,<=3.0", PreReleasesAuto, "3.0.dev0", true},
		{">=1,!=1.*,!=2.*,!=3.0,<=3.0", PreReleasesAuto, "3.0a1", true},
		{">=1,!=1.*,!=2.*,!=3.0,<=3.0", PreReleasesInclude, "3.0.dev0", true},
		{">=1,!=1.*,!=2.*,!=3.0,<=3.0", PreReleasesInclude, "3.0a1", true},
		{">=1,!=1.*,!=2.*,!=3.0,<=3.0", PreReleasesExclude, "3.0.dev0", false},
		{">=1,!=1.*,!=2.*,!=3.0,<=3.0", PreReleasesExclude, "3.0a1", false},
		{">=1,!=1.*,!=2.*,!=3.0,<=3.0", PreReleasesAuto, "0.9", false},
		{">=1,!=1.*,!=2.*,!=3.0,<=3.0", PreReleasesAuto, "1.0", false},
		{">=1,!=1.*,!=2.*,!=3.0,<=3.0", PreReleasesAuto, "2.0", false},
		{">=1,!=1.*,!=2.*,!=3.0,<=3.0", PreReleasesAuto, "3.0", false},
		{">=1,!=1.*,!=2.*,!=3.0,<=3.0", PreReleasesAuto, "4.0", false},
		{">=1.0a1,!=1.*,!=2.*,<3.0", PreReleasesAuto, "1.0a1", false},
		{">=1.0a1,!=1.*,!=2.*,<3.0", PreReleasesAuto, "2.0a1", false},
		{">=1.0a1,!=1.*,!=2.*,<3.0", PreReleasesAuto, "3.0a1", false},
		{">=1.0a1,!=1.*,!=2.*,<3.0", PreReleasesInclude, "1.0a1", false},
		{">=1.0a1,!=1.*,!=2.*,<3.0", PreReleasesInclude, "2.0a1", false},
		{">=1.0a1,!=1.*,!=2.*,<3.0", PreReleasesExclude, "1.0a1", false},
		{">=1.0a1,!=1.*,!=2.*,<3.0", PreReleasesExclude, "2.0a1", false},
		{">=1.0.dev0,!=1.*,!=2.*,<3.0.dev0", PreReleasesAuto, "1.0.dev0", false},
		{">=1.0.dev0,!=1.*,!=2.*,<3.0.dev0", PreReleasesAuto, "2.0.dev0", false},
		{">=1.0.dev0,!=1.*,!=2.*,<3.0.dev0", PreReleasesAuto, "3.0.dev0", false},
		{">=1.0.dev0,!=1.*,!=2.*,<3.0.dev0", PreReleasesInclude, "1.0.dev0", false},
		{">=1.0.dev0,!=1.*,!=2.*,<3.0.dev0", PreReleasesInclude, "2.0.dev0", false},
		{">=1.0.dev0,!=1.*,!=2.*,<3.0.dev0", PreReleasesExclude, "1.0.dev0", false},
		{">=1.0,!=1.0,!=1.1,<2.0", PreReleasesAuto, "1.0.post1", true},
		{">=1.0,!=1.0,!=1.1,<2.0", PreReleasesAuto, "1.1.post1", true},
		{">=1.0,!=1.0,!=1.1,<2.0", PreReleasesAuto, "1.0", false},
		{">=1.0,!=1.0,!=1.1,<2.0", PreReleasesAuto, "1.1", false},
		{">=1.0,!=1.0,!=1.1,<2.0", PreReleasesAuto, "2.0", false},
		{">=1.0,!=1.0,!=1.1,<2.0", PreReleasesInclude, "1.0.post1", true},
		{">=1.0,!=1.0,!=1.1,<2.0", PreReleasesInclude, "1.1.post1", true},
		{">=1.0,!=1.0,!=1.1,<2.0", PreReleasesExclude, "1.0.post1", true},
		{">=1.0,!=1.0,!=1.1,<2.0", PreReleasesExclude, "1.1.post1", true},
		{">=1,!=1.*,!=2.*,!=3.0,!=3.1,<4", PreReleasesAuto, "3.0.dev0", true},
		{">=1,!=1.*,!=2.*,!=3.0,!=3.1,<4", PreReleasesAuto, "3.1.dev0", true},
		{">=1,!=1.*,!=2.*,!=3.0,!=3.1,<4", PreReleasesAuto, "0.5", false},
		{">=1,!=1.*,!=2.*,!=3.0,!=3.1,<4", PreReleasesAuto, "3.0", false},
		{">=1,!=1.*,!=2.*,!=3.0,!=3.1,<4", PreReleasesAuto, "3.1", false},
		{">=1,!=1.*,!=2.*,!=3.0,!=3.1,<4", PreReleasesAuto, "5.0", false},
		{">=1,!=1.*,!=2.*,!=3.0,!=3.1,<4", PreReleasesInclude, "3.0.dev0", true},
		{">=1,!=1.*,!=2.*,!=3.0,!=3.1,<4", PreReleasesInclude, "3.1.dev0", true},
		{">=1,!=1.*,!=2.*,!=3.0,!=3.1,<4", PreReleasesExclude, "3.0.dev0", false},
		{">=1,!=1.*,!=2.*,!=3.0,!=3.1,<4", PreReleasesExclude, "3.1.dev0", false},
		{">=1.0a1,!=1.0,<1.1", PreReleasesAuto, "1.0a1", true},
		{">=1.0a1,!=1.0,<1.1", PreReleasesAuto, "1.0b1", true},
		{">=1.0a1,!=1.0,<1.1", PreReleasesAuto, "0.9", false},
		{">=1.0a1,!=1.0,<1.1", PreReleasesAuto, "1.0", false},
		{">=1.0a1,!=1.0,<1.1", PreReleasesAuto, "1.1", false},
		{">=1.0a1,!=1.0,<1.1", PreReleasesAuto, "1.1.dev0", false},
		{">=1.0a1,!=1.0,<1.1", PreReleasesAuto, "1.1a1", false},
		{">=1.0a1,!=1.0,<1.1", PreReleasesInclude, "1.0a1", true},
		{">=1.0a1,!=1.0,<1.1", PreReleasesInclude, "1.0b1", true},
		{">=1.0a1,!=1.0,<1.1", PreReleasesInclude, "1.1.dev0", false},
		{">=1.0a1,!=1.0,<1.1", PreReleasesInclude, "1.1a1", false},
		{">=1.0a1,!=1.0,<1.1", PreReleasesExclude, "1.0a1", false},
		{">=1.0a1,!=1.0,<1.1", PreReleasesExclude, "1.0b1", false},
		{">=0.9,!=0.9,<=1.0", PreReleasesAuto, "0.9.post1", true},
		{">=0.9,!=0.9,<=1.0", PreReleasesAuto, "1.0", true},
		{">=0.9,!=0.9,<=1.0", PreReleasesAuto, "1.0.dev0", true},
		{">=0.9,!=0.9,<=1.0", PreReleasesAuto, "1.0a1", true},
		{">=0.9,!=0.9,<=1.0", PreReleasesAuto, "1.0.post1", false},
		{">=0.9,!=0.9,<=1.0", PreReleasesInclude, "0.9.post1", true},
		{">=0.9,!=0.9,<=1.0", PreReleasesInclude, "1.0.dev0", true},
		{">=0.9,!=0.9,<=1.0", PreReleasesInclude, "1.0a1", true},
		{">=0.9,!=0.9,<=1.0", PreReleasesInclude, "1.0", true},
		{">=0.9,!=0.9,<=1.0", PreReleasesExclude, "0.9.post1", true},
		{">=0.9,!=0.9,<=1.0", PreReleasesExclude, "1.0.dev0", false},
		{">=0.9,!=0.9,<=1.0", PreReleasesExclude, "1.0a1", false},
		{">=0.9,!=0.9,<=1.0", PreReleasesExclude, "1.0", true},
		{">=1!0,!=1!1.*,!=1!2.*,<1!3", PreReleasesAuto, "1!0.5", true},
		{">=1!0,!=1!1.*,!=1!2.*,<1!3", PreReleasesAuto, "1!2.5", false},
		{">=1!0,!=1!1.*,!=1!2.*,<1!3", PreReleasesAuto, "0!5.0", false},
		{">=1!0,!=1!1.*,!=1!2.*,<1!3", PreReleasesAuto, "2!0.0", false},
		{">=1!0,!=1!1.*,!=1!2.*,<1!3", PreReleasesInclude, "1!0.5", true},
		{">=1!0,!=1!1.*,!=1!2.*,<1!3", PreReleasesInclude, "0!5.0", false},
		{">=1!0,!=1!1.*,!=1!2.*,<1!3", PreReleasesExclude, "1!0.5", true},
		{">=1!0,!=1!1.*,!=1!2.*,<1!3", PreReleasesExclude, "0!5.0", false},
		{">1.0.dev1,==1.0.post0", PreReleasesAuto, "1.0.post0", true},
		{">1.0.dev1,==1.0.post1", PreReleasesAuto, "1.0.post1", true},
		{">1.0a1,==1.0.post0", PreReleasesAuto, "1.0.post0", true},
		{">1.0.dev1,<=2.0", PreReleasesAuto, "1.0.post0", true},
		{">1.0.dev1,<=2.0", PreReleasesAuto, "1.0", true},
		{">1.0a1,<=2.0", PreReleasesInclude, "1.0a1.post0", false},
		{">1.0.dev1,<=1.0.post1", PreReleasesAuto, "1.0.post0", true},
		{">1.0.dev1,<=1.0.post1", PreReleasesAuto, "1.0.post1", true},
		{">1.0.dev1,<=1.0.post1", PreReleasesAuto, "1.0", true},
		{">1.0.dev1,!=1.0,<=2.0", PreReleasesAuto, "1.0.post0", true},
		{">1.0.dev1,!=1.0,!=1.0.post0,<=2.0", PreReleasesAuto, "1.0.post1", true},
		{"==1.0.dev0,<1.0.post1", PreReleasesAuto, "1.0.dev0", true},
		{"==1.0a1,<1.0.post0", PreReleasesAuto, "1.0a1", true},
		{"==1.0.post0.dev0,<1.0.post1", PreReleasesAuto, "1.0.post0.dev0", true},
		{">=1.0,<1.0.post1", PreReleasesAuto, "1.0", true},
		{">=1.0,<1.0.post1", PreReleasesAuto, "1.0.post0", true},
		{">=1.0,<1.0.post1", PreReleasesInclude, "1.0.dev0", false},
		{">=1.0.dev0,<1.0.post1", PreReleasesInclude, "1.0.dev0", true},
		{">=1.0.dev0,<1.0.post1", PreReleasesInclude, "1.0.a1", true},
		{">=1.0.dev0,<1.0.post1", PreReleasesInclude, "1.0.post0.dev0", true},
		{">=1.0.dev0,<1.0.post1,!=1.0,!=1.0.post0", PreReleasesInclude, "1.0.dev0", true},
		{">=1.0.dev0,<1.0.post1,!=1.0,!=1.0.post0", PreReleasesInclude, "1.0.post0.dev0", true},
		{">=1.0.dev0,<1.0.post2,!=1.0,!=1.0.post0", PreReleasesInclude, "1.0.post1", true},
		{">=1.0.dev0,<1.0.post2,!=1.0", PreReleasesInclude, "1.0.post0", true},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s__%s__%s", tt.spec, tt.version, tt.pre), func(t *testing.T) {
			ss, err := NewSpecifiers(tt.spec)
			require.NoError(t, err)
			assert.Equal(t, tt.want, ss.Contains(tt.version, WithPreReleases(tt.pre)))
		})
	}
}

// Ported from TestSpecifierSet.test_filter_exclusionary_bridges.
func TestConformance_SetFilterExclusionaryBridges(t *testing.T) {
	tests := []struct {
		spec  string
		pre   PreReleases
		input []string
		want  []string
	}{
		{">=1,!=1.*,!=2.*,!=3.0,<=3.0", PreReleasesAuto, []string{"3.0.dev0", "3.0a1"}, []string{"3.0.dev0", "3.0a1"}},
		{">=1,!=1.*,!=2.*,!=3.0,<=3.0", PreReleasesAuto, []string{"0.9", "3.0.dev0", "3.0a1", "4.0"}, []string{"3.0.dev0", "3.0a1"}},
		{">=1,!=1.*,!=2.*,!=3.0,<=3.0", PreReleasesInclude, []string{"0.9", "3.0.dev0", "3.0a1", "4.0"}, []string{"3.0.dev0", "3.0a1"}},
		{">=1,!=1.*,!=2.*,!=3.0,<=3.0", PreReleasesExclude, []string{"0.9", "3.0.dev0", "3.0a1", "4.0"}, nil},
		{">=1.0a1,!=1.*,!=2.*,<3.0", PreReleasesAuto, []string{"1.0a1", "2.0a1", "3.0a1"}, nil},
		{">=1.0a1,!=1.*,!=2.*,<3.0", PreReleasesInclude, []string{"1.0a1", "2.0a1", "3.0a1"}, nil},
		{">=1.0a1,!=1.*,!=2.*,<3.0", PreReleasesExclude, []string{"1.0a1", "2.0a1", "3.0a1"}, nil},
		{">=1.0.dev0,!=1.*,!=2.*,<3.0.dev0", PreReleasesAuto, []string{"1.0.dev0", "2.0.dev0", "3.0.dev0"}, nil},
		{">=1.0.dev0,!=1.*,!=2.*,<3.0.dev0", PreReleasesInclude, []string{"1.0.dev0", "2.0.dev0", "3.0.dev0"}, nil},
		{">=1.0.dev0,!=1.*,!=2.*,<3.0.dev0", PreReleasesExclude, []string{"1.0.dev0", "2.0.dev0", "3.0.dev0"}, nil},
		{">=1.0,!=1.0,!=1.1,<2.0", PreReleasesAuto, []string{"1.0.post1", "1.1.post1"}, []string{"1.0.post1", "1.1.post1"}},
		{">=1.0,!=1.0,!=1.1,<2.0", PreReleasesAuto, []string{"0.9", "1.0.post1", "1.1.post1", "2.0"}, []string{"1.0.post1", "1.1.post1"}},
		{">=1.0,!=1.0,!=1.1,<2.0", PreReleasesInclude, []string{"0.9", "1.0.post1", "1.1.post1", "2.0"}, []string{"1.0.post1", "1.1.post1"}},
		{">=1.0,!=1.0,!=1.1,<2.0", PreReleasesExclude, []string{"0.9", "1.0.post1", "1.1.post1", "2.0"}, []string{"1.0.post1", "1.1.post1"}},
		{">=1,!=1.*,!=2.*,!=3.0,!=3.1,<4", PreReleasesAuto, []string{"3.0.dev0", "3.1.dev0"}, []string{"3.0.dev0", "3.1.dev0"}},
		{">=1,!=1.*,!=2.*,!=3.0,!=3.1,<4", PreReleasesAuto, []string{"0.5", "3.0.dev0", "3.1.dev0", "5.0"}, []string{"3.0.dev0", "3.1.dev0"}},
		{">=1,!=1.*,!=2.*,!=3.0,!=3.1,<4", PreReleasesInclude, []string{"0.5", "3.0.dev0", "3.1.dev0", "5.0"}, []string{"3.0.dev0", "3.1.dev0"}},
		{">=1,!=1.*,!=2.*,!=3.0,!=3.1,<4", PreReleasesExclude, []string{"0.5", "3.0.dev0", "3.1.dev0", "5.0"}, nil},
		{">=1.0a1,!=1.0,<1.1", PreReleasesAuto, []string{"1.0a1", "1.0b1"}, []string{"1.0a1", "1.0b1"}},
		{">=1.0a1,!=1.0,<1.1", PreReleasesAuto, []string{"0.9", "1.0a1", "1.0b1", "1.1"}, []string{"1.0a1", "1.0b1"}},
		{">=1.0a1,!=1.0,<1.1", PreReleasesAuto, []string{"1.0a1", "1.0b1", "1.1.dev0", "1.1a1"}, []string{"1.0a1", "1.0b1"}},
		{">=1.0a1,!=1.0,<1.1", PreReleasesInclude, []string{"0.9", "1.0a1", "1.0b1", "1.1"}, []string{"1.0a1", "1.0b1"}},
		{">=1.0a1,!=1.0,<1.1", PreReleasesInclude, []string{"1.0a1", "1.0b1", "1.1.dev0", "1.1a1"}, []string{"1.0a1", "1.0b1"}},
		{">=1.0a1,!=1.0,<1.1", PreReleasesExclude, []string{"0.9", "1.0a1", "1.0b1", "1.1"}, nil},
		{">=0.9,!=0.9,<=1.0", PreReleasesAuto, []string{"0.9.post1", "1.0.dev0", "1.0a1", "1.0"}, []string{"0.9.post1", "1.0"}},
		{">=0.9,!=0.9,<=1.0", PreReleasesAuto, []string{"0.9.post1", "1.0.dev0", "1.0a1", "1.0", "1.0.post1"}, []string{"0.9.post1", "1.0"}},
		{">=0.9,!=0.9,<=1.0", PreReleasesInclude, []string{"0.9.post1", "1.0.dev0", "1.0a1", "1.0", "1.1"}, []string{"0.9.post1", "1.0.dev0", "1.0a1", "1.0"}},
		{">=0.9,!=0.9,<=1.0", PreReleasesExclude, []string{"0.9.post1", "1.0.dev0", "1.0a1", "1.0", "1.1"}, []string{"0.9.post1", "1.0"}},
		{">=1!0,!=1!1.*,!=1!2.*,<1!3", PreReleasesAuto, []string{"1!0.5", "1!2.5"}, []string{"1!0.5"}},
		{">=1!0,!=1!1.*,!=1!2.*,<1!3", PreReleasesAuto, []string{"0!5.0", "1!0.5", "1!2.5", "2!0.0"}, []string{"1!0.5"}},
		{">=1!0,!=1!1.*,!=1!2.*,<1!3", PreReleasesInclude, []string{"0!5.0", "1!0.5", "1!2.5", "2!0.0"}, []string{"1!0.5"}},
		{">=1!0,!=1!1.*,!=1!2.*,<1!3", PreReleasesExclude, []string{"0!5.0", "1!0.5", "1!2.5", "2!0.0"}, []string{"1!0.5"}},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s__%s__%v", tt.spec, tt.pre, tt.input), func(t *testing.T) {
			ss, err := NewSpecifiers(tt.spec)
			require.NoError(t, err)
			got := ss.Filter(tt.input, WithPreReleases(tt.pre))
			if len(tt.want) == 0 {
				assert.Empty(t, got)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

// Ported from TestSpecifierSet.test_contains_post_release_boundary_intersection.
//
// ">V" excludes EVERY post-release of V, and intersecting it with
// "!=V.postN" must not re-admit a different post-release of V. Upstream also
// asserts set membership equals the conjunction of its members; that check is
// carried over, because a set that disagreed with its own members would be a
// bug the table alone would not catch.
func TestConformance_SetPostReleaseBoundaryIntersection(t *testing.T) {
	tests := []struct {
		spec    string
		version string
		want    bool
	}{
		{">1.0,!=1.0.post1", "1.0.post2", false},
		{">1.0,!=1.0.post1", "1.0.post2.dev3", false},
		{">1.0,!=1.0.post1", "1.0.post1", false},
		{">1.0,!=1.0.post1", "1.1", true},
		{">2.3,!=2.3.post1", "2.3.post2", false},
		{">1!1.0,!=1!1.0.post1", "1!1.0.post2", false},
	}
	for _, tt := range tests {
		t.Run(tt.spec+"__"+tt.version, func(t *testing.T) {
			ss, err := NewSpecifiers(tt.spec)
			require.NoError(t, err)
			got := ss.Contains(tt.version, WithPreReleases(PreReleasesInclude))
			assert.Equal(t, tt.want, got)

			conjunction := true
			for _, s := range ss.List() {
				if !s.Contains(tt.version, WithPreReleases(PreReleasesInclude)) {
					conjunction = false
				}
			}
			assert.Equal(t, conjunction, got, "the set must agree with its members")
		})
	}
}

// Ported from TestSpecifierSet.test_filter_post_release_boundary_intersection.
func TestConformance_SetFilterPostReleaseBoundaryIntersection(t *testing.T) {
	ss, err := NewSpecifiers(">1.0,!=1.0.post1")
	require.NoError(t, err)
	got := ss.Filter([]string{"1.0.post2", "1.0.post3", "1.1"}, WithPreReleases(PreReleasesInclude))
	assert.Equal(t, []string{"1.1"}, got)
}

// Ported from TestSpecifierSet.test_contains_arbitrary_equality_contains.
func TestConformance_SetArbitraryEqualityContains(t *testing.T) {
	tests := []struct {
		spec string
		item string
		want bool
	}{
		{"===foobar", "foobar", true},
		{"===foobar", "foo", false},
		{"===foobar", "bar", false},
		{"===1.0", "1.0", true},
		{"===1.0", "1.0.0", false},
		{"===1.0+downstream1", "1.0", false},
		{"===1.0", "1.0+downstream1", false},
		{"===foobar,!=1.0", "foobar", false},
		{"===foobar,!=1.0", "1.0", false},
		{">=1.0,===foobar", "foobar", false},
		{">=1.0,===1.5", "1.5", true},
		{">=2.0,===1.5", "1.5", false},
		{">=1.0,===2.5", "2.5", true},
		{"!=1.0", "invalid", false},
		{"!=1.0", "foobar", false},
		{"!=2.0.*", "invalid", false},
		{"!=1.0,!=2.0", "invalid", false},
		{">=1.0,!=2.0", "foobar", false},
		{">=1.0,!=2.0", "1.5", true},
	}
	for _, tt := range tests {
		t.Run(tt.spec+"__"+tt.item, func(t *testing.T) {
			ss, err := NewSpecifiers(tt.spec)
			require.NoError(t, err)
			assert.Equal(t, tt.want, ss.Contains(tt.item))
		})
	}
}

// Ported from TestSpecifierSet.test_comparison_ignores_local: a local segment
// on the PROSPECTIVE version is ignored by the ordered comparisons, because a
// specifier may not name one.
func TestConformance_SetIgnoresLocalOnProspective(t *testing.T) {
	tests := []struct {
		version string
		spec    string
		want    bool
	}{
		{"1.0.0+local", "==1.0.0", true},
		{"1.0.0+local", "!=1.0.0", false},
		{"1.0.0+local", "<=1.0.0", true},
		{"1.0.0+local", ">=1.0.0", true},
		{"1.0.0+local", "<1.0.0", false},
		{"1.0.0+local", ">1.0.0", false},
	}
	for _, tt := range tests {
		t.Run(tt.version+"__"+tt.spec, func(t *testing.T) {
			ss, err := NewSpecifiers(tt.spec)
			require.NoError(t, err)
			v, err := Parse(tt.version)
			require.NoError(t, err)
			assert.Equal(t, tt.want, ss.ContainsVersion(v))
		})
	}
}

// Ported from TestSpecifierSet.test_contains_with_compatible_operator. Both
// members survive the combination precisely because `~=1.18.0` and `~=1.18`
// are NOT equal, so the narrower one still applies.
func TestConformance_SetContainsWithCompatibleOperator(t *testing.T) {
	narrow, err := NewSpecifiers("~=1.18.0")
	require.NoError(t, err)
	wide, err := NewSpecifiers("~=1.18")
	require.NoError(t, err)

	combination, err := narrow.And(wide)
	require.NoError(t, err)
	assert.False(t, combination.Contains("1.19.5"))
	assert.True(t, combination.Contains("1.18.0"))
}

// Ported from TestSpecifierSet.test_contains_rejects_invalid_specifier: a
// string that is not a PEP 440 version cannot satisfy an ordinary specifier.
func TestConformance_SetContainsRejectsInvalidVersion(t *testing.T) {
	ss, err := NewSpecifiers(">=1.0", WithPreReleases(PreReleasesInclude))
	require.NoError(t, err)
	assert.False(t, ss.Contains("not a valid version"))
}

// Ported from TestSpecifierSet.test_comparison_true / test_comparison_false:
// the SPECIFIERS cross-product at the SET level.
func TestConformance_SetEqualityMatrix(t *testing.T) {
	for i, left := range conformanceValidSpecifiers {
		for j, right := range conformanceValidSpecifiers {
			t.Run(left+"__"+right, func(t *testing.T) {
				l, err := NewSpecifiers(left)
				require.NoError(t, err)
				r, err := NewSpecifiers(right)
				require.NoError(t, err)
				assert.Equal(t, i == j, l.Equal(r))
			})
		}
	}
}

// Ported from TestSpecifierSet.test_specifiers_combine,
// test_specifiers_combine_deduplicates and test_specifiers_duplicate_normalization.
func TestConformance_SetCombine(t *testing.T) {
	left, err := NewSpecifiers(">=1.0.0,!=1.0.1")
	require.NoError(t, err)
	right, err := NewSpecifiers("<=2.0.0,!=2.0.1")
	require.NoError(t, err)

	got, err := left.And(right)
	require.NoError(t, err)
	assert.Equal(t, 4, got.Len())

	// ⚠️ Upstream renders the members SORTED ("!=1.0.1,!=2.0.1,<=2.0.0,>=1.0.0").
	// This package preserves input order, a documented divergence, so the
	// membership is asserted rather than the rendering.
	expect, err := NewSpecifiers(">=1.0.0,!=1.0.1,<=2.0.0,!=2.0.1")
	require.NoError(t, err)
	assert.True(t, got.Equal(expect))

	// Duplicates are kept, exactly as upstream keeps them: in packaging 26.2,
	// len(SpecifierSet(">=1.0") & SpecifierSet(">=1.0")) is 2. Equal is the
	// operation that is blind to them.
	one, err := NewSpecifiers(">=1.0")
	require.NoError(t, err)
	dup, err := one.And(one)
	require.NoError(t, err)
	assert.Equal(t, 2, dup.Len())
	assert.True(t, dup.Equal(one))
}
