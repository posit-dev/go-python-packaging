// SPDX-License-Identifier: Apache-2.0 OR MIT
package tags

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompileAnyLibc_AcceptsBothFamilies is the whole point: one Matcher that
// does not silently drop a wheel family.
func TestCompileAnyLibc_AcceptsBothFamilies(t *testing.T) {
	m, err := linuxTarget(2, 28).CompileAnyLibc()
	require.NoError(t, err)

	for _, tag := range []string{
		"cp313-cp313-manylinux_2_17_x86_64",
		"cp313-cp313-manylinux_2_28_x86_64",
		"cp313-cp313-manylinux2014_x86_64",
		"cp313-cp313-musllinux_1_1_x86_64",
		"cp313-cp313-musllinux_1_2_x86_64",
		"cp313-cp313-linux_x86_64",
		"py3-none-any",
	} {
		t.Run(tag, func(t *testing.T) {
			assert.True(t, m.IsCompatible(mustTags(t, tag)))
		})
	}
}

// TestCompileAnyLibc_SingleFamilyDropsTheOther records the mistake being
// removed, so the value of the helper is executable rather than asserted in prose.
func TestCompileAnyLibc_SingleFamilyDropsTheOther(t *testing.T) {
	glibcOnly := mustCompile(t, linuxTarget(2, 28))
	musl := mustTags(t, "cp313-cp313-musllinux_1_2_x86_64")
	assert.False(t, glibcOnly.IsCompatible(musl),
		"a glibc-only Matcher silently rejects every musllinux wheel")

	any, err := linuxTarget(2, 28).CompileAnyLibc()
	require.NoError(t, err)
	assert.True(t, any.IsCompatible(musl))
}

// TestCompileAnyLibc_NoDuplicateTags guards the union: the bare linux_<arch> tag
// and the whole compatible tier are emitted by both families.
func TestCompileAnyLibc_NoDuplicateTags(t *testing.T) {
	m, err := linuxTarget(2, 28).CompileAnyLibc()
	require.NoError(t, err)

	all := m.Tags()
	seen := make(map[Tag]int, len(all))
	for _, tag := range all {
		seen[tag]++
	}
	for tag, n := range seen {
		assert.Equal(t, 1, n, "tag %s appears %d times", tag, n)
	}

	// And it really is a union, not just one family's list.
	var many, musl int
	for _, tag := range all {
		if strings.HasPrefix(tag.Platform, "manylinux") {
			many++
		}
		if strings.HasPrefix(tag.Platform, "musllinux") {
			musl++
		}
	}
	assert.NotZero(t, many)
	assert.NotZero(t, musl)
}

// TestCompileAnyLibc_GlibcRanksAhead pins the stated ordering, since Rank is a
// published part of the contract.
func TestCompileAnyLibc_GlibcRanksAhead(t *testing.T) {
	m, err := linuxTarget(2, 28).CompileAnyLibc()
	require.NoError(t, err)

	gRank, ok := m.Rank(mustTags(t, "cp313-cp313-manylinux_2_28_x86_64"))
	require.True(t, ok)
	mRank, ok := m.Rank(mustTags(t, "cp313-cp313-musllinux_1_2_x86_64"))
	require.True(t, ok)
	assert.Less(t, gRank, mRank, "glibc tiers are ranked ahead of musl")
}

// TestCompileAnyLibc_NewerWorksForBothFamilies checks the interaction with
// IsCompatibleOrNewer: an any-libc Matcher must treat a too-new tag from EITHER
// family as newer, not just from glibc.
func TestCompileAnyLibc_NewerWorksForBothFamilies(t *testing.T) {
	m, err := linuxTarget(2, 28).CompileAnyLibc()
	require.NoError(t, err)

	newGlibc := mustTags(t, "cp313-cp313-manylinux_2_45_x86_64")
	require.False(t, m.IsCompatible(newGlibc))
	assert.True(t, m.IsCompatibleOrNewer(newGlibc))

	newMusl := mustTags(t, "cp313-cp313-musllinux_2_0_x86_64")
	require.False(t, m.IsCompatible(newMusl))
	assert.True(t, m.IsCompatibleOrNewer(newMusl))

	// The ABI guardrail still holds on the any-libc path.
	wrongABI := mustTags(t, "cp314-cp314-musllinux_2_0_x86_64")
	assert.False(t, m.IsCompatibleOrNewer(wrongABI))
}

func TestCompileAnyLibc_RejectsNonLinux(t *testing.T) {
	for _, tg := range []Target{
		{Implementation: "cp", PyMajor: 3, PyMinor: 13, OS: "windows", Arch: "amd64"},
		{Implementation: "cp", PyMajor: 3, PyMinor: 13, OS: "macos", Arch: "arm64", MacMajor: 14},
	} {
		t.Run(tg.OS, func(t *testing.T) {
			_, err := tg.CompileAnyLibc()
			assert.ErrorIs(t, err, ErrUnsupportedTarget)
		})
	}
}

// TestCompileAnyLibc_IgnoresLibcField documents that t.Libc is not consulted, so
// setting it does not change the result and cannot half-apply.
func TestCompileAnyLibc_IgnoresLibcField(t *testing.T) {
	asGlibc := linuxTarget(2, 28)
	asGlibc.Libc = "glibc"
	a, err := asGlibc.CompileAnyLibc()
	require.NoError(t, err)

	asMusl := linuxTarget(2, 28)
	asMusl.Libc = "musl"
	b, err := asMusl.CompileAnyLibc()
	require.NoError(t, err)

	assert.Equal(t, a.Tags(), b.Tags())
}

// TestCompileAnyLibc_StillValidates confirms the underlying validation is not
// bypassed by filling in Libc internally.
func TestCompileAnyLibc_StillValidates(t *testing.T) {
	bad := linuxTarget(2, 28)
	bad.Arch = "nosucharch"
	_, err := bad.CompileAnyLibc()
	assert.ErrorIs(t, err, ErrUnsupportedTarget)

	noVersion := linuxTarget(0, 0)
	_, err = noVersion.CompileAnyLibc()
	assert.ErrorIs(t, err, ErrUnsupportedTarget)
}
