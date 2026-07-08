// SPDX-License-Identifier: Apache-2.0 OR MIT
package tags

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompile_RejectsUnsupported(t *testing.T) {
	for _, tg := range []Target{
		{Implementation: "pp", PyMajor: 3, PyMinor: 11, OS: "linux", Arch: "x86_64", Libc: "glibc", LibcMajor: 2, LibcMinor: 28},
		{Implementation: "cp", PyMajor: 3, PyMinor: 11, OS: "solaris", Arch: "x86_64"},
		{Implementation: "cp", PyMajor: 3, PyMinor: 11, OS: "macos", Arch: "arm64", MacMajor: 10, MacMinor: 15},                  // <11
		{Implementation: "cp", PyMajor: 3, PyMinor: -1, OS: "linux", Arch: "x86_64", Libc: "glibc", LibcMajor: 2, LibcMinor: 28}, // invalid PyMinor, must error not panic
	} {
		_, err := tg.Compile()
		require.Error(t, err)
	}
}

// tagStrings stringifies a []Tag for easy Contains/NotContains assertions.
func tagStrings(tags []Tag) []string {
	out := make([]string, len(tags))
	for i, tag := range tags {
		out[i] = tag.String()
	}
	return out
}

// TestLinux_ArchDependentFloor exercises the arch-dependent manylinux floor
// table directly (see Global Constraints): x86_64 walks all the way down to
// the manylinux1 (glibc 2.5) alias, while aarch64 floors at glibc 2.17 and
// never claims manylinux_2_5_aarch64 (2.5 predates aarch64 manylinux
// support entirely).
func TestLinux_ArchDependentFloor(t *testing.T) {
	mX, err := Target{Implementation: "cp", PyMajor: 3, PyMinor: 11, OS: "linux", Arch: "x86_64", Libc: "glibc", LibcMajor: 2, LibcMinor: 28}.Compile()
	require.NoError(t, err)
	assert.Contains(t, tagStrings(mX.Tags()), "cp311-cp311-manylinux_2_5_x86_64")
	assert.Contains(t, tagStrings(mX.Tags()), "cp311-cp311-manylinux1_x86_64")

	mA, err := Target{Implementation: "cp", PyMajor: 3, PyMinor: 9, OS: "linux", Arch: "aarch64", Libc: "glibc", LibcMajor: 2, LibcMinor: 17}.Compile()
	require.NoError(t, err)
	assert.NotContains(t, tagStrings(mA.Tags()), "cp39-cp39-manylinux_2_5_aarch64")
	assert.Contains(t, tagStrings(mA.Tags()), "cp39-cp39-manylinux_2_17_aarch64")
}

// TestLinux_BareLinuxRankedLast asserts the deliberate divergence from
// pypa/packaging: a bare "linux_<arch>" tag is accepted, but ranked below
// every manylinux/musllinux tag for the same target.
func TestLinux_BareLinuxRankedLast(t *testing.T) {
	m, err := Target{Implementation: "cp", PyMajor: 3, PyMinor: 11, OS: "linux", Arch: "x86_64", Libc: "glibc", LibcMajor: 2, LibcMinor: 28}.Compile()
	require.NoError(t, err)
	rMany, ok1 := m.Rank([]Tag{{"cp311", "cp311", "manylinux_2_17_x86_64"}})
	rBare, ok2 := m.Rank([]Tag{{"cp311", "cp311", "linux_x86_64"}})
	require.True(t, ok1)
	require.True(t, ok2)
	assert.Less(t, rMany, rBare)
}

func TestMatcher_RankPrefersMoreSpecific(t *testing.T) {
	m, err := Target{Implementation: "cp", PyMajor: 3, PyMinor: 11, OS: "windows", Arch: "amd64"}.Compile()
	require.NoError(t, err)
	// exact cp311-cp311-win_amd64 must outrank the pure py3-none-any universal
	rExact, ok1 := m.Rank([]Tag{{"cp311", "cp311", "win_amd64"}})
	rUniv, ok2 := m.Rank([]Tag{{"py3", "none", "any"}})
	require.True(t, ok1)
	require.True(t, ok2)
	assert.Less(t, rExact, rUniv)
	_, ok := m.Rank([]Tag{{"cp311", "cp311", "manylinux_2_17_x86_64"}})
	assert.False(t, ok) // linux tag incompatible with a windows target
}
