// SPDX-License-Identifier: Apache-2.0 OR MIT
package tags

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBaselines_KnownValues spot-checks rows whose glibc version is
// independently well known, so a bad regeneration of
// data/distro_baselines.json cannot pass silently. The manylinux-defining
// releases are the load-bearing ones: manylinux2010 and manylinux2014 were
// specified as "whatever CentOS 6 and CentOS 7 ship", so those two rows must
// agree with the legacy alias versions in manylinuxFloor, which they are
// checked against below.
func TestBaselines_KnownValues(t *testing.T) {
	for _, want := range []Baseline{
		{Distro: "centos", Release: "6", Libc: "glibc", LibcMajor: 2, LibcMinor: 12},
		{Distro: "centos", Release: "7", Libc: "glibc", LibcMajor: 2, LibcMinor: 17},
		{Distro: "almalinux", Release: "8", Libc: "glibc", LibcMajor: 2, LibcMinor: 28},
		{Distro: "almalinux", Release: "9", Libc: "glibc", LibcMajor: 2, LibcMinor: 34},
		{Distro: "rocky", Release: "9", Libc: "glibc", LibcMajor: 2, LibcMinor: 34},
		{Distro: "debian", Release: "11", Libc: "glibc", LibcMajor: 2, LibcMinor: 31},
		{Distro: "debian", Release: "12", Libc: "glibc", LibcMajor: 2, LibcMinor: 36},
		{Distro: "ubuntu", Release: "18.04", Libc: "glibc", LibcMajor: 2, LibcMinor: 27},
		{Distro: "ubuntu", Release: "20.04", Libc: "glibc", LibcMajor: 2, LibcMinor: 31},
		{Distro: "ubuntu", Release: "22.04", Libc: "glibc", LibcMajor: 2, LibcMinor: 35},
		{Distro: "ubuntu", Release: "24.04", Libc: "glibc", LibcMajor: 2, LibcMinor: 39},
		{Distro: "amazonlinux", Release: "2", Libc: "glibc", LibcMajor: 2, LibcMinor: 26},
		{Distro: "alpine", Release: "3.20", Libc: "musl", LibcMajor: 1, LibcMinor: 2},
	} {
		got, ok := LookupBaseline(want.Distro, want.Release)
		require.True(t, ok, "no baseline for %s %s", want.Distro, want.Release)
		assert.Equal(t, want, got)
	}
}

// TestBaselines_AgreeWithLegacyAliases ties the table to the generator it is
// meant to explain: manylinux2010 and manylinux2014 are defined as the glibc
// of CentOS 6 and CentOS 7 respectively, so the alias versions this package
// generates tags from and the baseline rows must not drift apart.
func TestBaselines_AgreeWithLegacyAliases(t *testing.T) {
	aliasOf := func(name string) legacyManylinuxAlias {
		for _, alias := range manylinuxFloor["x86_64"].legacy {
			if alias.name == name {
				return alias
			}
		}
		t.Fatalf("no %s alias for x86_64", name)
		return legacyManylinuxAlias{}
	}
	for name, release := range map[string]string{"manylinux2010": "6", "manylinux2014": "7"} {
		alias := aliasOf(name)
		base, ok := LookupBaseline("centos", release)
		require.True(t, ok)
		assert.Equal(t, alias.major, base.LibcMajor, "%s vs centos %s", name, release)
		assert.Equal(t, alias.minor, base.LibcMinor, "%s vs centos %s", name, release)
	}
}

func TestLookupBaseline_Normalization(t *testing.T) {
	want, ok := LookupBaseline("centos-stream", "9")
	require.True(t, ok)
	for _, spelling := range []string{"CentOS-Stream", "centos_stream", "  centos-stream  "} {
		got, ok := LookupBaseline(spelling, "9")
		assert.True(t, ok, "spelling %q", spelling)
		assert.Equal(t, want, got, "spelling %q", spelling)
	}

	// /etc/os-release IDs that differ from the usual name are aliased.
	amzn, ok := LookupBaseline("amzn", "2")
	require.True(t, ok)
	assert.Equal(t, "amazonlinux", amzn.Distro)

	// Releases must be spelled the way the distribution writes them.
	_, ok = LookupBaseline("ubuntu", "22.4")
	assert.False(t, ok)
	_, ok = LookupBaseline("ubuntu", "22")
	assert.False(t, ok)
	// RHEL is deliberately not aliased to its rebuilds.
	_, ok = LookupBaseline("rhel", "9")
	assert.False(t, ok)
}

func TestBaselinesFor(t *testing.T) {
	distros := func(t *testing.T, platformTag string) []string {
		t.Helper()
		got, err := BaselinesFor(platformTag)
		require.NoError(t, err)
		out := make([]string, len(got))
		for i, b := range got {
			out[i] = b.Distro + " " + b.Release
		}
		return out
	}

	// glibc 2.28 is RHEL/AlmaLinux 8; everything at or above it qualifies and
	// everything below does not.
	got := distros(t, "manylinux_2_28_x86_64")
	assert.Contains(t, got, "almalinux 8")
	assert.Contains(t, got, "almalinux 9")
	assert.Contains(t, got, "ubuntu 20.04")
	assert.NotContains(t, got, "ubuntu 18.04") // glibc 2.27, one short
	assert.NotContains(t, got, "centos 7")     // glibc 2.17
	// A glibc tag never selects a musl distro, or vice versa.
	assert.NotContains(t, got, "alpine 3.20")
	assert.Contains(t, distros(t, "musllinux_1_1_x86_64"), "alpine 3.20")
	assert.NotContains(t, distros(t, "musllinux_1_1_x86_64"), "ubuntu 22.04")

	// The legacy aliases resolve to their arch-dependent glibc version:
	// manylinux2014 is glibc 2.17 (CentOS 7), so CentOS 7 itself qualifies,
	// while manylinux1's 2.5 admits every recorded glibc release.
	assert.Contains(t, distros(t, "manylinux2014_x86_64"), "centos 7")
	assert.NotContains(t, distros(t, "manylinux2014_x86_64"), "centos 6")
	assert.Contains(t, distros(t, "manylinux1_x86_64"), "centos 6")

	// A tag newer than anything recorded yields an empty result, which is an
	// answer and not an error.
	none, err := BaselinesFor("manylinux_2_99_x86_64")
	require.NoError(t, err)
	assert.Empty(t, none)

	// A tag whose floor has a LOWER major must select every glibc baseline:
	// glibc 2.x satisfies ">= 1.0". This is a version comparison, not a
	// same-major one. No such tag occurs in practice -- glibc's major has been
	// 2 for the whole manylinux era -- but the comparison has to be a
	// comparison, or it silently answers "none" instead of "all".
	lowerMajor := distros(t, "manylinux_1_0_x86_64")
	var everyGlibc []string
	for _, b := range Baselines() {
		if b.Libc == "glibc" {
			everyGlibc = append(everyGlibc, b.Distro+" "+b.Release)
		}
	}
	require.NotEmpty(t, everyGlibc)
	assert.Equal(t, everyGlibc, lowerMajor,
		"a lower-major floor must select every recorded glibc baseline")
	// ...and symmetrically, a HIGHER-major floor must select none of them.
	assert.Empty(t, distros(t, "manylinux_3_0_x86_64"))
	// The same holds for musl.
	assert.Empty(t, distros(t, "musllinux_2_0_x86_64"))
	assert.Contains(t, distros(t, "musllinux_0_9_x86_64"), "alpine 3.20")
}

func TestBaselinesFor_Errors(t *testing.T) {
	for name, tag := range map[string]string{
		// A bare linux tag carries no libc guarantee to compare against --
		// which is exactly why this package ranks it below every manylinux tag.
		"bare linux":           "linux_x86_64",
		"not a linux tag":      "win_amd64",
		"macOS":                "macosx_11_0_arm64",
		"missing arch":         "manylinux_2_28",
		"unknown arch":         "manylinux_2_28_sparc64",
		"unparseable major":    "manylinux_x_28_x86_64",
		"unparseable minor":    "manylinux_2_y_x86_64",
		"musllinux legacy":     "musllinux1_x86_64",
		"legacy without arch":  "manylinux2014",
		"alias wrong for arch": "manylinux2010_aarch64", // aarch64 only has manylinux2014
		"nonexistent alias":    "manylinux1999_x86_64",
		"empty":                "",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := BaselinesFor(tag)
			require.ErrorIs(t, err, ErrInvalidTag)
		})
	}
}

// TestBaselinesFor_MatchesCompiledTarget is the differential that makes the
// "layered on the core comparison" claim checkable rather than asserted: for
// every recorded glibc baseline, BaselinesFor's verdict on a manylinux tag must
// agree with whether a Target built from that same baseline actually accepts a
// wheel carrying it.
//
// x86_64 is used deliberately. Its manylinux floor (glibc 2.5) is below every
// recorded baseline, so the two computations are comparing the same thing. On
// an architecture whose manylinux series starts later -- aarch64 at 2.17 --
// they part ways by design, which the case at the end of this test pins.
func TestBaselinesFor_MatchesCompiledTarget(t *testing.T) {
	for _, tag := range []string{
		"manylinux_2_5_x86_64", "manylinux_2_17_x86_64", "manylinux_2_28_x86_64",
		"manylinux_2_34_x86_64", "manylinux_2_35_x86_64", "manylinux_2_39_x86_64",
		"manylinux1_x86_64", "manylinux2010_x86_64", "manylinux2014_x86_64",
	} {
		t.Run(tag, func(t *testing.T) {
			selected, err := BaselinesFor(tag)
			require.NoError(t, err)
			inSelection := make(map[string]bool, len(selected))
			for _, b := range selected {
				inSelection[b.String()] = true
			}

			for _, b := range Baselines() {
				if b.Libc != "glibc" {
					continue
				}
				m, err := b.Apply(Target{
					Implementation: "cp", PyMajor: 3, PyMinor: 12,
					OS: "linux", Arch: "x86_64",
				}).Compile()
				require.NoError(t, err)
				compiled := m.IsCompatible([]Tag{{"cp312", "cp312", tag}})
				assert.Equal(t, compiled, inSelection[b.String()],
					"%s: BaselinesFor says %v, compiled target says %v", b, inSelection[b.String()], compiled)
			}
		})
	}

	// The documented divergence. A manylinux_2_12_aarch64 wheel is legal per
	// PEP 600 and runs anywhere aarch64 glibc is >= 2.12, so CentOS 6 is a
	// truthful answer -- but an aarch64 Target claims no manylinux tag older
	// than 2.17, because that is where the aarch64 manylinux series begins.
	selected, err := BaselinesFor("manylinux_2_12_aarch64")
	require.NoError(t, err)
	var names []string
	for _, b := range selected {
		names = append(names, b.Distro+" "+b.Release)
	}
	assert.Contains(t, names, "centos 6")

	m, err := Target{
		Implementation: "cp", PyMajor: 3, PyMinor: 12,
		OS: "linux", Arch: "aarch64", Libc: "glibc", LibcMajor: 2, LibcMinor: 12,
	}.Compile()
	require.NoError(t, err)
	assert.False(t, m.IsCompatible([]Tag{{"cp312", "cp312", "manylinux_2_12_aarch64"}}),
		"an aarch64 target below the aarch64 manylinux floor claims no manylinux tags")
}

// TestBaselines_TableInvariants guards the shape of the generated file: a
// regeneration that drops the provenance fields, emits a duplicate release, or
// records a libc that is not one Target understands must fail here rather than
// become a silently-wrong lookup.
func TestBaselines_TableInvariants(t *testing.T) {
	raw, err := os.ReadFile("data/distro_baselines.json")
	require.NoError(t, err)
	var file distroBaselineFile
	require.NoError(t, json.Unmarshal(raw, &file))

	assert.NotEmpty(t, file.Source, "the table must record where it came from")
	assert.NotEmpty(t, file.Regenerate, "the table must record how to regenerate it")
	assert.NotEmpty(t, file.Notes)

	all := Baselines()
	require.NotEmpty(t, all)
	seen := make(map[string]bool, len(all))
	for _, b := range all {
		assert.Contains(t, []string{"glibc", "musl"}, b.Libc, "%s", b)
		assert.NotEmpty(t, b.Distro)
		assert.NotEmpty(t, b.Release)
		assert.Positive(t, b.LibcMajor, "%s", b)
		assert.GreaterOrEqual(t, b.LibcMinor, 0, "%s", b)

		key := b.Distro + "/" + b.Release
		assert.False(t, seen[key], "duplicate baseline for %s", key)
		seen[key] = true

		// Every row must round-trip through the lookup index, and must produce
		// a Target that Compile accepts.
		got, ok := LookupBaseline(b.Distro, b.Release)
		require.True(t, ok, "%s is not reachable via LookupBaseline", b)
		assert.Equal(t, b, got)
		_, err := b.Apply(Target{
			Implementation: "cp", PyMajor: 3, PyMinor: 12,
			OS: "linux", Arch: "x86_64",
		}).Compile()
		assert.NoError(t, err, "%s", b)
	}

	// Baselines() must hand out a copy: mutating the result cannot corrupt the
	// compiled-in table for the rest of the process.
	all[0].Distro = "mutated"
	assert.NotEqual(t, "mutated", Baselines()[0].Distro)
}
