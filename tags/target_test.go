// SPDX-License-Identifier: Apache-2.0 OR MIT
package tags

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompile_RejectsUnsupported(t *testing.T) {
	for name, tg := range map[string]Target{
		"unknown implementation": {Implementation: "jy", PyMajor: 3, PyMinor: 11, OS: "windows", Arch: "amd64"},
		"unknown OS":             {Implementation: "cp", PyMajor: 3, PyMinor: 11, OS: "solaris", Arch: "x86_64"},
		// Apple silicon shipped with macOS 11, so 10.x arm64 is not a machine
		// that can exist -- unlike 10.x x86_64, which is now supported.
		"macOS 10.x on arm64":      {Implementation: "cp", PyMajor: 3, PyMinor: 11, OS: "macos", Arch: "arm64", MacMajor: 10, MacMinor: 15},
		"macOS major below 10":     {Implementation: "cp", PyMajor: 3, PyMinor: 11, OS: "macos", Arch: "x86_64", MacMajor: 9, MacMinor: 2},
		"negative PyMinor":         {Implementation: "cp", PyMajor: 3, PyMinor: -1, OS: "linux", Arch: "x86_64", Libc: "glibc", LibcMajor: 2, LibcMinor: 28},
		"negative LibcMinor":       {Implementation: "cp", PyMajor: 3, PyMinor: 11, OS: "linux", Arch: "x86_64", Libc: "musl", LibcMajor: 1, LibcMinor: -2},
		"negative MacMinor":        {Implementation: "cp", PyMajor: 3, PyMinor: 11, OS: "macos", Arch: "arm64", MacMajor: 12, MacMinor: -1},
		"free-threaded pre-3.13":   {Implementation: "cp", PyMajor: 3, PyMinor: 12, FreeThreaded: true, OS: "windows", Arch: "amd64"},
		"free-threaded non-cp":     {Implementation: "py", PyMajor: 3, PyMinor: 13, FreeThreaded: true, OS: "windows", Arch: "amd64"},
		"free-threaded PyPy":       {Implementation: "pp", PyMajor: 3, PyMinor: 13, FreeThreaded: true, OS: "windows", Arch: "amd64"},
		"impl version on cp":       {Implementation: "cp", PyMajor: 3, PyMinor: 11, ImplMajor: 7, ImplMinor: 3, OS: "windows", Arch: "amd64"},
		"impl version on py":       {Implementation: "py", PyMajor: 3, PyMinor: 11, ImplMajor: 7, ImplMinor: 3, OS: "windows", Arch: "amd64"},
		"negative impl minor":      {Implementation: "pp", PyMajor: 3, PyMinor: 11, ImplMajor: 7, ImplMinor: -3, OS: "windows", Arch: "amd64"},
		"impl minor without major": {Implementation: "pp", PyMajor: 3, PyMinor: 11, ImplMinor: 3, OS: "windows", Arch: "amd64"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := tg.Compile()
			require.ErrorIs(t, err, ErrUnsupportedTarget)
		})
	}
}

// TestCompile_AcceptsImplementations pins the implementations Compile accepts,
// including the two added in #18766: "pp" (with or without a known PyPy-side
// version) and free-threaded "cp".
func TestCompile_AcceptsImplementations(t *testing.T) {
	for name, tg := range map[string]Target{
		"cp":                {Implementation: "cp", PyMajor: 3, PyMinor: 11, OS: "windows", Arch: "amd64"},
		"py":                {Implementation: "py", PyMajor: 3, PyMinor: 11, OS: "windows", Arch: "amd64"},
		"pp with version":   {Implementation: "pp", PyMajor: 3, PyMinor: 11, ImplMajor: 7, ImplMinor: 3, OS: "windows", Arch: "amd64"},
		"pp without":        {Implementation: "pp", PyMajor: 3, PyMinor: 11, OS: "windows", Arch: "amd64"},
		"free-threaded 313": {Implementation: "cp", PyMajor: 3, PyMinor: 13, FreeThreaded: true, OS: "windows", Arch: "amd64"},
		"macOS 10.x x86_64": {Implementation: "cp", PyMajor: 3, PyMinor: 9, OS: "macos", Arch: "x86_64", MacMajor: 10, MacMinor: 15},
	} {
		t.Run(name, func(t *testing.T) {
			m, err := tg.Compile()
			require.NoError(t, err)
			require.NotEmpty(t, m.Tags())
		})
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

// TestMacOS_ArchFormats exercises the arch-dependent macOS binary-format list:
// arm64 only ever claims "arm64"/"universal2" (no legacy Intel formats),
// including across the pre-11 compatibility tail, where it claims universal2
// alone; x86_64 claims the full legacy format list at every version.
func TestMacOS_ArchFormats(t *testing.T) {
	mArm, err := Target{Implementation: "cp", PyMajor: 3, PyMinor: 10, OS: "macos", Arch: "arm64", MacMajor: 12, MacMinor: 0}.Compile()
	require.NoError(t, err)
	ss := tagStrings(mArm.Tags())
	assert.Contains(t, ss, "cp310-cp310-macosx_12_0_arm64")
	assert.Contains(t, ss, "cp310-cp310-macosx_12_0_universal2")
	assert.Contains(t, ss, "cp310-cp310-macosx_11_0_arm64")
	for _, s := range ss {
		assert.NotContains(t, s, "_intel")
		assert.NotContains(t, s, "_fat")
		// Arm64 support arrived in macOS 11, so no pre-11 tag may name arm64
		// directly -- only the universal2 fat format, whose x86_64 half can
		// legitimately declare a pre-11 minimum.
		if strings.Contains(s, "macosx_10_") {
			assert.True(t, strings.HasSuffix(s, "_universal2"), "pre-11 arm64 tag %q must be universal2", s)
		}
	}

	mX, err := Target{Implementation: "cp", PyMajor: 3, PyMinor: 12, OS: "macos", Arch: "x86_64", MacMajor: 14, MacMinor: 0}.Compile()
	require.NoError(t, err)
	xs := tagStrings(mX.Tags())
	for _, fmtSuffix := range []string{"x86_64", "intel", "fat64", "fat32", "universal2", "universal"} {
		assert.Contains(t, xs, "cp312-cp312-macosx_14_0_"+fmtSuffix)
		assert.Contains(t, xs, "cp312-cp312-macosx_11_0_"+fmtSuffix)
		assert.Contains(t, xs, "cp312-cp312-macosx_10_16_"+fmtSuffix)
		assert.Contains(t, xs, "cp312-cp312-macosx_10_4_"+fmtSuffix)
	}
}

// TestMacOS_PreElevenWalks pins the two different version walks and the
// ordering between them. A declared 10.x target walks the MINOR version and
// gets no 11+ tags at all; a declared 11+ target walks the MAJOR version and
// then appends the pre-11 tail, strictly below every 11+ tag.
func TestMacOS_PreElevenWalks(t *testing.T) {
	m10, err := Target{Implementation: "cp", PyMajor: 3, PyMinor: 9, OS: "macos", Arch: "x86_64", MacMajor: 10, MacMinor: 15}.Compile()
	require.NoError(t, err)
	ss := tagStrings(m10.Tags())
	assert.Contains(t, ss, "cp39-cp39-macosx_10_15_x86_64")
	assert.Contains(t, ss, "cp39-cp39-macosx_10_4_x86_64")
	for _, s := range ss {
		assert.NotContains(t, s, "macosx_11_")
		assert.NotContains(t, s, "macosx_10_16_")
		// _mac_binary_formats has no x86_64 format below (10, 4): 10.3 and
		// older yield nothing rather than a bare "macosx_10_3_x86_64".
		for _, tooOld := range []string{"macosx_10_3_", "macosx_10_2_", "macosx_10_1_", "macosx_10_0_"} {
			assert.NotContains(t, s, tooOld)
		}
	}

	m12, err := Target{Implementation: "cp", PyMajor: 3, PyMinor: 9, OS: "macos", Arch: "x86_64", MacMajor: 12, MacMinor: 0}.Compile()
	require.NoError(t, err)
	rEleven, ok1 := m12.Rank([]Tag{{"cp39", "cp39", "macosx_11_0_x86_64"}})
	rTail, ok2 := m12.Rank([]Tag{{"cp39", "cp39", "macosx_10_16_x86_64"}})
	require.True(t, ok1)
	require.True(t, ok2)
	assert.Less(t, rEleven, rTail, "the pre-11 compatibility tail must rank below every macOS 11+ tag")
}

// TestFreeThreaded_SubstitutesABI3T asserts the load-bearing consequence of
// PEP 703: a free-threaded target takes cp<XY>t as its exact ABI and PEP 803's
// abi3t as its stable ABI, and does NOT accept abi3 wheels at all -- the
// GIL-enabled stable ABI is a different ABI, not a subset.
func TestFreeThreaded_SubstitutesABI3T(t *testing.T) {
	ft, err := Target{Implementation: "cp", PyMajor: 3, PyMinor: 13, FreeThreaded: true, OS: "windows", Arch: "amd64"}.Compile()
	require.NoError(t, err)
	ss := tagStrings(ft.Tags())
	assert.Contains(t, ss, "cp313-cp313t-win_amd64")
	assert.Contains(t, ss, "cp313-abi3t-win_amd64")
	assert.Contains(t, ss, "cp312-abi3t-win_amd64")
	assert.Contains(t, ss, "cp32-abi3t-win_amd64")
	assert.NotContains(t, ss, "cp313-cp313-win_amd64")
	for _, s := range ss {
		assert.NotContains(t, s, "-abi3-")
	}

	// ...and the converse: a GIL-enabled target of the same version accepts
	// abi3 and never abi3t or a "t" ABI.
	gil, err := Target{Implementation: "cp", PyMajor: 3, PyMinor: 13, OS: "windows", Arch: "amd64"}.Compile()
	require.NoError(t, err)
	gs := tagStrings(gil.Tags())
	assert.Contains(t, gs, "cp313-cp313-win_amd64")
	assert.Contains(t, gs, "cp313-abi3-win_amd64")
	for _, s := range gs {
		assert.NotContains(t, s, "-abi3t-")
		assert.NotContains(t, s, "-cp313t-")
	}
}

// TestPyPy_ImplementationABI asserts the PyPy-specific spellings: the exact ABI
// names both versions ("pypy310_pp73"), the compatible tier's interpreter-any
// entry is the MAJOR-only "pp3-none-any" (not "pp310-none-any"), and there is
// no abi3 tier, which is a CPython concept.
func TestPyPy_ImplementationABI(t *testing.T) {
	m, err := Target{
		Implementation: "pp", PyMajor: 3, PyMinor: 10, ImplMajor: 7, ImplMinor: 3,
		OS: "windows", Arch: "amd64",
	}.Compile()
	require.NoError(t, err)
	ss := tagStrings(m.Tags())
	assert.Contains(t, ss, "pp310-pypy310_pp73-win_amd64")
	assert.Contains(t, ss, "pp310-none-win_amd64")
	assert.Contains(t, ss, "pp3-none-any")
	assert.NotContains(t, ss, "pp310-none-any")
	for _, s := range ss {
		assert.NotContains(t, s, "abi3")
		assert.NotContains(t, s, "cp")
	}

	// The implementation ABI outranks the generic "pp<XY>-none" tier, which in
	// turn outranks the pure-Python universal tail.
	rABI, ok1 := m.Rank([]Tag{{"pp310", "pypy310_pp73", "win_amd64"}})
	rNone, ok2 := m.Rank([]Tag{{"pp310", "none", "win_amd64"}})
	rAny, ok3 := m.Rank([]Tag{{"py3", "none", "any"}})
	require.True(t, ok1)
	require.True(t, ok2)
	require.True(t, ok3)
	assert.Less(t, rABI, rNone)
	assert.Less(t, rNone, rAny)

	// A cp wheel is not compatible with a PyPy target.
	assert.False(t, m.IsCompatible([]Tag{{"cp310", "cp310", "win_amd64"}}))
}

// TestPyPy_UnknownImplVersion: with no PyPy-side version the target simply has
// no implementation-ABI tier. It must NOT fabricate one (e.g. "pypy310_pp00").
func TestPyPy_UnknownImplVersion(t *testing.T) {
	m, err := Target{Implementation: "pp", PyMajor: 3, PyMinor: 10, OS: "windows", Arch: "amd64"}.Compile()
	require.NoError(t, err)
	ss := tagStrings(m.Tags())
	assert.Equal(t, "pp310-none-win_amd64", ss[0])
	for _, s := range ss {
		assert.NotContains(t, s, "pypy")
	}
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
