// SPDX-License-Identifier: Apache-2.0 OR MIT
package tags

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestArchs_AgreesWithValidate is the point of exporting these lists: the
// exported view and the value Compile actually accepts must not drift. Every
// arch Archs reports must compile, and a value it does not report must not.
func TestArchs_AgreesWithValidate(t *testing.T) {
	for _, os := range OSes() {
		archs := Archs(os)
		require.NotEmpty(t, archs, "%s must report at least one arch", os)

		for _, arch := range archs {
			t.Run(os+"/"+arch, func(t *testing.T) {
				_, err := targetFor(os, arch).Compile()
				assert.NoError(t, err, "Archs reported %s/%s, so it must compile", os, arch)
			})
		}

		t.Run(os+"/unsupported", func(t *testing.T) {
			_, err := targetFor(os, "nosucharch").Compile()
			assert.ErrorIs(t, err, ErrUnsupportedTarget)
		})
	}
}

// targetFor fills in the per-OS mandatory version axes so the only thing under
// test is the arch.
func targetFor(os, arch string) Target {
	t := Target{Implementation: "cp", PyMajor: 3, PyMinor: 12, OS: os, Arch: arch}
	switch os {
	case "linux":
		t.Libc, t.LibcMajor, t.LibcMinor = "glibc", 2, 28
	case "macos":
		t.MacMajor, t.MacMinor = 14, 0
		if arch == "x86_64" {
			t.MacMajor = 11
		}
	}
	return t
}

func TestArchs_UnknownOS(t *testing.T) {
	assert.Nil(t, Archs("solaris"))
	assert.Nil(t, Archs(""))
}

// TestArchs_ReturnsACopy guards the stated contract: a caller must not be able
// to widen what Compile accepts by mutating the returned slice.
func TestArchs_ReturnsACopy(t *testing.T) {
	first := Archs("windows")
	require.NotEmpty(t, first)
	first[0] = "tampered"

	second := Archs("windows")
	assert.NotContains(t, second, "tampered")

	_, err := targetFor("windows", "tampered").Compile()
	assert.ErrorIs(t, err, ErrUnsupportedTarget)
}

// TestArchs_SpellingsDiffer records the reason a caller needs to enumerate
// rather than guess.
func TestArchs_SpellingsDiffer(t *testing.T) {
	assert.Contains(t, Archs("linux"), "x86_64")
	assert.NotContains(t, Archs("windows"), "x86_64", "windows spells it amd64")
	assert.Contains(t, Archs("windows"), "amd64")

	assert.Contains(t, Archs("linux"), "aarch64")
	assert.NotContains(t, Archs("macos"), "aarch64", "macOS spells it arm64")
	assert.Contains(t, Archs("macos"), "arm64")
}
