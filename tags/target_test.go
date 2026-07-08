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

// TestCompile_LinuxNotYetImplemented asserts that a well-formed linux Target
// returns ErrUnsupportedTarget from Compile rather than panicking, since
// linux platform tag generation is not implemented until #18632 Task 3. This
// case is expected to start succeeding once that task lands; at that point it
// should be replaced with a real linux golden-file assertion.
func TestCompile_LinuxNotYetImplemented(t *testing.T) {
	tg := Target{Implementation: "cp", PyMajor: 3, PyMinor: 11, OS: "linux", Arch: "x86_64", Libc: "glibc", LibcMajor: 2, LibcMinor: 28}
	_, err := tg.Compile()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrUnsupportedTarget)
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
