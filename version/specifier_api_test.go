// SPDX-License-Identifier: Apache-2.0 OR MIT

// Unit tests for the public specifier API added for
// rstudio/package-manager#19383: the singular Specifier, its accessors and
// canonical equality, set length/iteration/combination, and the empty set.
//
// Every expected value in this file was measured against pypa/packaging 26.2,
// not derived from the PEP text. The conformance tables ported from upstream's
// own test suite live separately, in specifier_conformance_test.go.

package version

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSpecifier_Accessors(t *testing.T) {
	tests := []struct {
		in      string
		op      string
		operand string
		str     string
	}{
		{"~=2.0", "~=", "2.0", "~=2.0"},
		{"==2.1.*", "==", "2.1.*", "==2.1.*"},
		{"==2.1.0.3", "==", "2.1.0.3", "==2.1.0.3"},
		{"!=2.2.*", "!=", "2.2.*", "!=2.2.*"},
		{"<=5", "<=", "5", "<=5"},
		{">=7.9a1", ">=", "7.9a1", ">=7.9a1"},
		{"<1.0.dev1", "<", "1.0.dev1", "<1.0.dev1"},
		{">2.0.post1", ">", "2.0.post1", ">2.0.post1"},
		// === is PEP 440's escape hatch: the operand is an opaque token.
		{"===lolwat", "===", "lolwat", "===lolwat"},
		// Whitespace between operator and operand is removed by String, which
		// is the whole of upstream's normalization at this level.
		{"< 2", "<", "2", "<2"},
		// ⚠️ The operand is NOT normalized as a version. Upstream's
		// Specifier.version returns it as written, so the "v" prefix survives:
		//   Specifier(">=  v1.0  ").version -> 'v1.0'
		//
		// The upper-case counterpart ("== 1.0A1" -> '1.0A1') is a separate,
		// still-open divergence: validConstraintRegexp lacks the (?i) flag
		// specifierRegexp has, so an upper-case operand is rejected before it
		// reaches this code. Tracked with the other parse divergences in
		// rstudio/package-manager#19391.
		{">=  v1.0  ", ">=", "v1.0", ">=v1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			s, err := NewSpecifier(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.op, s.Operator(), "Operator()")
			assert.Equal(t, tt.operand, s.Version(), "Version()")
			assert.Equal(t, tt.str, s.String(), "String()")
		})
	}
}

// NewSpecifier is the SINGULAR constructor. A comma-separated set is not one
// specifier, and must not quietly parse as its first member with the rest
// discarded -- the failure mode would be a constraint that matches far more
// than the caller wrote.
func TestNewSpecifier_RejectsASet(t *testing.T) {
	for _, in := range []string{">=1,<2", "==1.0,==2.0", ">=1.0 <2.0"} {
		t.Run(in, func(t *testing.T) {
			_, err := NewSpecifier(in)
			assert.Error(t, err)
		})
	}
}

func TestNewSpecifier_Invalid(t *testing.T) {
	for _, in := range []string{"", "=>2.0", "==", "~=1", ">=1.0+deadbeef", "<1.0.*"} {
		t.Run(fmt.Sprintf("%q", in), func(t *testing.T) {
			_, err := NewSpecifier(in)
			assert.Error(t, err)
		})
	}
}

// Specifier.Equal canonicalizes the operand rather than comparing text, so
// trailing zeros, case, a "v" prefix, a zero epoch and local-segment case are
// all insignificant -- except under "~=", where trailing zeros change which
// versions match and so must stay significant.
func TestSpecifier_Equal_Canonicalizes(t *testing.T) {
	tests := []struct {
		left, right string
		want        bool
	}{
		{"==2.8.0", "==2.8", true},
		{"==1.0.0.0", "==1", true},
		{"< 2", "<2", true},
		{"==1.01", "==1.1", true},
		{"==v1.0", "==1.0", true},
		{"==0!1.0", "==1.0", true},
		// The case-insensitive pairs upstream also treats as equal
		// (`==1.0A1`/`==1.0a1`, `==1.0+abc`/`==1.0+ABC`) cannot be built yet:
		// validConstraintRegexp rejects an upper-case operand. See
		// rstudio/package-manager#19391.
		{"==2.1.*", "==2.1.*", true},
		{"~=1.2.0", "~=1.2.0", true},
		// ⚠️ `~=1.18` and `~=1.18.0` accept different sets, so the trailing
		// zero is load-bearing and they are NOT equal. Upstream keeps trailing
		// zeros for `~=` alone (strip_trailing_zero=(operator != "~=")).
		{"~=1.18.0", "~=1.18", false},
		{"==1.0", "!=1.0", false},
		// A wildcard operand is compared verbatim: it is not a version, so
		// there is nothing to canonicalize, and "2.1.*" and "2.1.0.*" are
		// genuinely different prefixes.
		{"==2.1.*", "==2.1.0.*", false},
		// An arbitrary-equality operand is likewise verbatim, case included.
		{"===lolwat", "===LOLWAT", false},
	}

	for _, tt := range tests {
		t.Run(tt.left+"__"+tt.right, func(t *testing.T) {
			l, err := NewSpecifier(tt.left)
			require.NoError(t, err)
			r, err := NewSpecifier(tt.right)
			require.NoError(t, err)
			assert.Equal(t, tt.want, l.Equal(r))
			assert.Equal(t, tt.want, r.Equal(l), "Equal must be symmetric")
		})
	}
}

// A pre-release policy is explicitly NOT part of equality: two specifiers that
// differ only in policy are the same constraint.
func TestSpecifier_Equal_IgnoresPreReleasePolicy(t *testing.T) {
	a, err := NewSpecifier(">=1.2.3", WithPreReleases(PreReleasesInclude))
	require.NoError(t, err)
	b, err := NewSpecifier(">=1.2.3", WithPreReleases(PreReleasesExclude))
	require.NoError(t, err)
	assert.True(t, a.Equal(b))
}

func TestSpecifiers_LenAndList(t *testing.T) {
	tests := []struct {
		in    string
		len   int
		items []string
	}{
		{"", 0, nil},
		{"==2.0", 1, []string{"==2.0"}},
		{">=2.0", 1, []string{">=2.0"}},
		{">=2.0,<3", 2, []string{">=2.0", "<3"}},
		{">=2.0,<3,==2.4", 3, []string{">=2.0", "<3", "==2.4"}},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.in), func(t *testing.T) {
			ss, err := NewSpecifiers(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.len, ss.Len())

			var got []string
			for _, s := range ss.List() {
				got = append(got, s.String())
			}
			assert.ElementsMatch(t, tt.items, got)
			assert.Len(t, ss.List(), ss.Len(), "List and Len must agree")
		})
	}
}

// NewSpecifiers("") is the universal set, not an error and not an empty one.
// Upstream's SpecifierSet(”) has length zero and contains every version; this
// package used to reject the empty string outright (the residue noted on
// rstudio/package-manager#19391 after #19366/#19465 fixed the zero-value arm).
func TestNewSpecifiers_EmptyStringIsUniversal(t *testing.T) {
	ss, err := NewSpecifiers("")
	require.NoError(t, err)
	assert.Equal(t, 0, ss.Len())
	assert.Equal(t, "", ss.String())

	for _, v := range []string{"0", "1.0", "2.0a1", "1!3.0", "1.0.dev0", "1.0+local"} {
		assert.True(t, ss.Check(MustParse(v)), "Check(%s)", v)
	}
}

func TestSpecifiers_And(t *testing.T) {
	left, err := NewSpecifiers(">=1.0.0,!=1.0.1")
	require.NoError(t, err)
	right, err := NewSpecifiers("<=2.0.0,!=2.0.1")
	require.NoError(t, err)

	combined, err := left.And(right)
	require.NoError(t, err)
	assert.Equal(t, 4, combined.Len())

	// The intersection must actually intersect.
	assert.True(t, combined.Check(MustParse("1.5")))
	assert.False(t, combined.Check(MustParse("1.0.1")))
	assert.False(t, combined.Check(MustParse("2.0.1")))
	assert.False(t, combined.Check(MustParse("3.0")))
}

// And deduplicates, which is where upstream SETTLES even though a fresh object
// reports otherwise.
//
// ⚠️ Measured against packaging 26.2, `len(SpecifierSet(">=1.0") &
// SpecifierSet(">=1.0"))` is 2 on a fresh object, 1 after any canonicalizing
// call, and `str()` is ">=1.0" always:
//
//	c = SpecifierSet(">=1.0") & SpecifierSet(">=1.0")
//	len(c)      -> 2
//	str(c)      -> '>=1.0'
//	len(c)      -> 1      # _canonical_specs() dedupes LAZILY
//
// So the 2 is an artifact of upstream's lazy cache -- the very thing this port
// declares non-portable -- and it makes the count depend on call order. An
// earlier version of this test asserted the 2 and cited it as deliberate
// upstream parity, which was wrong on both counts.
//
// Matching is unaffected either way: a conjunction is idempotent.
func TestSpecifiers_And_Deduplicates(t *testing.T) {
	one, err := NewSpecifiers(">=1.0")
	require.NoError(t, err)

	combined, err := one.And(one)
	require.NoError(t, err)
	assert.Equal(t, 1, combined.Len(), "a repeated member collapses")
	assert.Equal(t, ">=1.0", combined.String())
	assert.True(t, combined.Equal(one))

	// Matching is the same either way, which is why this is safe.
	assert.True(t, combined.Check(MustParse("1.5")))
	assert.False(t, combined.Check(MustParse("0.5")))

	// Deduplication is CANONICAL, not textual: these are the same constraint.
	a, err := NewSpecifiers(">=1.0.0")
	require.NoError(t, err)
	b, err := NewSpecifiers(">=1")
	require.NoError(t, err)
	collapsed, err := a.And(b)
	require.NoError(t, err)
	assert.Equal(t, 1, collapsed.Len())

	// But genuinely distinct members are all kept, including the `~=` pair whose
	// trailing zero is load-bearing.
	narrow, err := NewSpecifiers("~=1.18.0")
	require.NoError(t, err)
	wide, err := NewSpecifiers("~=1.18")
	require.NoError(t, err)
	both, err := narrow.And(wide)
	require.NoError(t, err)
	assert.Equal(t, 2, both.Len(), "~=1.18.0 and ~=1.18 are different constraints")
	assert.False(t, both.Contains("1.19.5"))
	assert.True(t, both.Contains("1.18.0"))
}

func TestSpecifiers_And_PreReleasePolicyCombination(t *testing.T) {
	tests := []struct {
		left, right PreReleases
		want        PreReleases
		wantErr     bool
	}{
		{PreReleasesAuto, PreReleasesAuto, PreReleasesAuto, false},
		{PreReleasesAuto, PreReleasesInclude, PreReleasesInclude, false},
		{PreReleasesAuto, PreReleasesExclude, PreReleasesExclude, false},
		{PreReleasesInclude, PreReleasesAuto, PreReleasesInclude, false},
		{PreReleasesInclude, PreReleasesInclude, PreReleasesInclude, false},
		{PreReleasesExclude, PreReleasesAuto, PreReleasesExclude, false},
		{PreReleasesExclude, PreReleasesExclude, PreReleasesExclude, false},
		// A contradiction is an error, not a silent winner.
		{PreReleasesInclude, PreReleasesExclude, PreReleasesAuto, true},
		{PreReleasesExclude, PreReleasesInclude, PreReleasesAuto, true},
	}

	for _, tt := range tests {
		t.Run(tt.left.String()+"__"+tt.right.String(), func(t *testing.T) {
			l, err := NewSpecifiers(">=1", WithPreReleases(tt.left))
			require.NoError(t, err)
			r, err := NewSpecifiers("<2", WithPreReleases(tt.right))
			require.NoError(t, err)

			combined, err := l.And(r)
			if tt.wantErr {
				assert.ErrorIs(t, err, ErrPreReleaseConflict)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, combined.PreReleases())
		})
	}
}

func TestSpecifiers_Equal_IgnoresOrderAndDuplicates(t *testing.T) {
	tests := []struct {
		left, right string
		want        bool
	}{
		{">=1,<2", "<2,>=1", true},
		{">=1,>=1", ">=1", true},
		{">=1.0.0", ">=1", true},
		{">=1,<2", ">=1", false},
		{">=1,<2", ">=1,<3", false},
	}

	for _, tt := range tests {
		t.Run(tt.left+"__"+tt.right, func(t *testing.T) {
			l, err := NewSpecifiers(tt.left)
			require.NoError(t, err)
			r, err := NewSpecifiers(tt.right)
			require.NoError(t, err)
			assert.Equal(t, tt.want, l.Equal(r))
			assert.Equal(t, tt.want, r.Equal(l))
		})
	}
}

func TestNewSpecifiersFrom(t *testing.T) {
	lower, err := NewSpecifier(">=1.0")
	require.NoError(t, err)
	upper, err := NewSpecifier("<2.0")
	require.NoError(t, err)

	ss := NewSpecifiersFrom([]Specifier{lower, upper})
	assert.Equal(t, 2, ss.Len())
	assert.Equal(t, ">=1.0,<2.0", ss.String())
	assert.True(t, ss.Check(MustParse("1.5")))
	assert.False(t, ss.Check(MustParse("2.5")))

	parsed, err := NewSpecifiers(">=1.0,<2.0")
	require.NoError(t, err)
	assert.True(t, ss.Equal(parsed))
}

// NewSpecifiersFrom must copy: mutating the caller's slice afterwards cannot
// be allowed to change the set.
func TestNewSpecifiersFrom_CopiesInput(t *testing.T) {
	lower, err := NewSpecifier(">=1.0")
	require.NoError(t, err)
	other, err := NewSpecifier("==9.9")
	require.NoError(t, err)

	input := []Specifier{lower}
	ss := NewSpecifiersFrom(input)
	input[0] = other

	assert.Equal(t, ">=1.0", ss.String())
}

// Specifiers.String renders each member normalized. Before this change it
// returned the raw input text, so "< 2" round-tripped as "< 2" while upstream
// gives "<2".
func TestSpecifiers_String_IsNormalized(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"< 2", "<2"},
		{"== 1.0", "==1.0"},
		{">=  v1.0  ", ">=v1.0"},
		{">= 1.0, < 2.0", ">=1.0,<2.0"},
		{"==2.0", "==2.0"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			ss, err := NewSpecifiers(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, ss.String())
		})
	}
}

// A zero-value Specifier has no operator function. It must constrain nothing
// rather than panic on a nil call -- the same reading that makes an empty
// Specifiers universal (rstudio/package-manager#19366).
func TestZeroValueSpecifierIsHarmless(t *testing.T) {
	var s Specifier
	assert.Equal(t, "", s.Operator())
	assert.Equal(t, "", s.Version())
	assert.Equal(t, "", s.String())
	assert.NotPanics(t, func() {
		assert.True(t, s.Check(MustParse("1.0")))
	})
}
