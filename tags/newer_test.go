// SPDX-License-Identifier: Apache-2.0 OR MIT
package tags

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustCompile(t *testing.T, tg Target) *Matcher {
	t.Helper()
	m, err := tg.Compile()
	require.NoError(t, err)
	return m
}

func linuxTarget(libcMajor, libcMinor int) Target {
	return Target{
		Implementation: "cp", PyMajor: 3, PyMinor: 13,
		OS: "linux", Arch: "x86_64",
		Libc: "glibc", LibcMajor: libcMajor, LibcMinor: libcMinor,
	}
}

// mustTags parses a wheel's tag component the way a caller would.
func mustTags(t *testing.T, s string) []Tag {
	t.Helper()
	parsed, err := ParseTag(s)
	require.NoError(t, err)
	return parsed
}

func TestIsCompatibleOrNewer_Linux(t *testing.T) {
	m := mustCompile(t, linuxTarget(2, 28))

	for _, tc := range []struct {
		tag           string
		compatible    bool
		compatOrNewer bool
		why           string
	}{
		{"cp313-cp313-manylinux_2_17_x86_64", true, true, "at or below the declared floor"},
		{"cp313-cp313-manylinux_2_28_x86_64", true, true, "exactly the declared version"},
		{"cp313-cp313-manylinux_2_45_x86_64", false, true, "newer glibc than declared"},
		{"cp313-cp313-manylinux_3_0_x86_64", false, true, "newer glibc major"},
		{"cp313-cp313-musllinux_1_2_x86_64", false, false, "wrong libc family"},
		{"cp313-cp313-manylinux_2_45_aarch64", false, false, "wrong arch"},
		{"cp313-cp313-win_amd64", false, false, "wrong OS"},
		{"py3-none-any", true, true, "pure python"},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			w := mustTags(t, tc.tag)
			assert.Equal(t, tc.compatible, m.IsCompatible(w), "IsCompatible: %s", tc.why)
			assert.Equal(t, tc.compatOrNewer, m.IsCompatibleOrNewer(w), "IsCompatibleOrNewer: %s", tc.why)
		})
	}
}

// TestIsCompatibleOrNewer_DoesNotRelaxTheABI is the guardrail: relaxing the
// platform axis must not smuggle in a wheel built for an interpreter this target
// cannot run.
func TestIsCompatibleOrNewer_DoesNotRelaxTheABI(t *testing.T) {
	m := mustCompile(t, linuxTarget(2, 28))

	for _, tag := range []string{
		"cp314-cp314-manylinux_2_45_x86_64", // newer platform AND newer interpreter
		"cp312-cp312-manylinux_2_45_x86_64", // newer platform, older interpreter ABI
		"pp310-pypy310_pp73-manylinux_2_45_x86_64",
	} {
		t.Run(tag, func(t *testing.T) {
			w := mustTags(t, tag)
			require.False(t, m.IsCompatible(w))
			assert.False(t, m.IsCompatibleOrNewer(w),
				"a newer platform must not excuse an ABI this target does not accept")
		})
	}
}

// TestIsCompatibleOrNewer_FreeThreadedKeepsItsABI checks the interaction with
// the abi3/abi3t substitution: a free-threaded target must not gain abi3 wheels
// just because their platform is newer.
func TestIsCompatibleOrNewer_FreeThreadedKeepsItsABI(t *testing.T) {
	ft := linuxTarget(2, 28)
	ft.FreeThreaded = true
	m := mustCompile(t, ft)

	abi3 := mustTags(t, "cp37-abi3-manylinux_2_45_x86_64")
	assert.False(t, m.IsCompatible(abi3))
	assert.False(t, m.IsCompatibleOrNewer(abi3),
		"a free-threaded target does not accept abi3 at any platform version")

	abi3t := mustTags(t, "cp313-abi3t-manylinux_2_45_x86_64")
	assert.False(t, m.IsCompatible(abi3t), "the platform is above the declared floor")
	assert.True(t, m.IsCompatibleOrNewer(abi3t), "but the ABI is one this target accepts")
}

func TestIsCompatibleOrNewer_MacOS(t *testing.T) {
	m := mustCompile(t, Target{
		Implementation: "cp", PyMajor: 3, PyMinor: 13,
		OS: "macos", Arch: "arm64", MacMajor: 14, MacMinor: 0,
	})

	for _, tc := range []struct {
		tag           string
		compatible    bool
		compatOrNewer bool
		why           string
	}{
		{"cp313-cp313-macosx_14_0_arm64", true, true, "exactly the declared version"},
		{"cp313-cp313-macosx_11_0_arm64", true, true, "older, inside the walk"},
		{"cp313-cp313-macosx_15_0_arm64", false, true, "newer macOS"},
		{"cp313-cp313-macosx_26_0_arm64", false, true, "much newer macOS"},
		{"cp313-cp313-macosx_15_0_universal2", false, true, "newer, arm64 accepts universal2"},
		{"cp313-cp313-macosx_15_0_x86_64", false, false, "newer but arm64 does not accept x86_64"},
		// The trap: a naive "_x86_64" suffix test on the whole tag would treat
		// this as an x86_64 linux tag. It is a macOS tag, and for an arm64
		// target it is not acceptable at any version.
		{"cp313-cp313-macosx_10_9_x86_64", false, false, "macOS 10.9 x86_64 on an arm64 target"},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			w := mustTags(t, tc.tag)
			assert.Equal(t, tc.compatible, m.IsCompatible(w), "IsCompatible: %s", tc.why)
			assert.Equal(t, tc.compatOrNewer, m.IsCompatibleOrNewer(w), "IsCompatibleOrNewer: %s", tc.why)
		})
	}
}

// TestIsCompatibleOrNewer_WindowsHasNoVersionAxis records that "newer" is
// meaningless for windows platform tags, so the method degrades to IsCompatible.
func TestIsCompatibleOrNewer_WindowsHasNoVersionAxis(t *testing.T) {
	m := mustCompile(t, Target{
		Implementation: "cp", PyMajor: 3, PyMinor: 13,
		OS: "windows", Arch: "amd64",
	})

	ok := mustTags(t, "cp313-cp313-win_amd64")
	assert.True(t, m.IsCompatibleOrNewer(ok))

	wrong := mustTags(t, "cp313-cp313-win32")
	assert.Equal(t, m.IsCompatible(wrong), m.IsCompatibleOrNewer(wrong))
}

// TestIsCompatibleOrNewer_LegacyAliasesResolveToTheirGlibc pins that a pre-PEP
// 600 alias is judged by the glibc version it names, not by being "old". The
// question this method answers is whether a wheel requires a newer platform than
// was declared, so manylinux2014 (glibc 2.17) is newer than a 2.12 declaration
// and merely compatible with a 2.28 one.
func TestIsCompatibleOrNewer_LegacyAliasesResolveToTheirGlibc(t *testing.T) {
	below := mustCompile(t, linuxTarget(2, 12))
	w := mustTags(t, "cp313-cp313-manylinux2014_x86_64")
	require.False(t, below.IsCompatible(w), "2.17 is above a 2.12 declaration")
	assert.True(t, below.IsCompatibleOrNewer(w),
		"it requires more glibc than declared, which is what newer means here")

	above := mustCompile(t, linuxTarget(2, 28))
	assert.True(t, above.IsCompatible(w), "and at 2.28 it is simply compatible")
}

func TestParseMacosPlatformTag(t *testing.T) {
	for _, tc := range []struct {
		in     string
		major  int
		minor  int
		format string
		bad    bool
	}{
		{in: "macosx_14_0_arm64", major: 14, minor: 0, format: "arm64"},
		{in: "macosx_10_9_x86_64", major: 10, minor: 9, format: "x86_64"},
		{in: "macosx_11_0_universal2", major: 11, minor: 0, format: "universal2"},
		{in: "manylinux_2_28_x86_64", bad: true},
		{in: "macosx_14_arm64", bad: true},
		{in: "macosx_x_0_arm64", bad: true},
		{in: "macosx_14_y_arm64", bad: true},
		{in: "macosx_14_0_", bad: true},
	} {
		t.Run(tc.in, func(t *testing.T) {
			major, minor, format, err := parseMacosPlatformTag(tc.in)
			if tc.bad {
				assert.ErrorIs(t, err, ErrInvalidTag)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.major, major)
			assert.Equal(t, tc.minor, minor)
			assert.Equal(t, tc.format, format)
		})
	}
}

func TestVersionAbove(t *testing.T) {
	assert.True(t, versionAbove(2, 45, 2, 28))
	assert.True(t, versionAbove(3, 0, 2, 99), "a version comparison, not per-component")
	assert.False(t, versionAbove(2, 28, 2, 28), "strict")
	assert.False(t, versionAbove(2, 17, 2, 28))
	assert.False(t, versionAbove(1, 99, 2, 0))
}
