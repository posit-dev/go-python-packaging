// SPDX-License-Identifier: Apache-2.0 OR MIT
package tags

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// distroBaselinesJSON is the compiled-in baseline table. It is embedded, never
// fetched: this library is consumed by servers that are frequently air-gapped,
// so no lookup here may depend on the network.
//
// Provenance and regeneration live in data/gen_distro_baselines.py, and the
// data file repeats them in its own "source"/"regenerate"/"notes" fields so
// they travel with the data.
//
//go:embed data/distro_baselines.json
var distroBaselinesJSON []byte

// Baseline records the C library a well-known Linux distribution release
// ships, which is what determines the newest manylinux/musllinux wheel that
// release can run.
//
// This is a convenience layer over the comparison that actually decides
// compatibility, not a replacement for it: a target's libc version is the
// authoritative input to Target, and a real host can deviate from its
// release's stock libc (backports, a hand-built glibc, a container image on a
// different base). Use a Baseline to name a floor -- "we support RHEL 8 and
// newer, so glibc 2.28" -- and pass the version, not the name, to Target.
type Baseline struct {
	// Distro is the canonical lowercase distribution name, e.g. "ubuntu",
	// "almalinux", "centos-stream", "opensuse-leap".
	Distro string `json:"distro"`
	// Release is the distribution's own release identifier as it is normally
	// written: "22.04" for Ubuntu, "9" for AlmaLinux, "3.20" for Alpine.
	Release string `json:"release"`
	// Libc is "glibc" or "musl", matching Target.Libc.
	Libc string `json:"libc"`
	// LibcMajor and LibcMinor are the version of that C library, matching
	// Target.LibcMajor/LibcMinor.
	LibcMajor int `json:"libcMajor"`
	LibcMinor int `json:"libcMinor"`
}

// String renders a Baseline for human-readable messages, e.g.
// "ubuntu 22.04 (glibc 2.35)".
func (b Baseline) String() string {
	return fmt.Sprintf("%s %s (%s %d.%d)", b.Distro, b.Release, b.Libc, b.LibcMajor, b.LibcMinor)
}

// Apply returns a copy of t with the baseline's C library and version filled
// in, leaving every other field alone. It is the intended way to go from a
// distribution name to a Target:
//
//	base, _ := tags.LookupBaseline("ubuntu", "22.04")
//	m, err := base.Apply(tags.Target{
//		Implementation: "cp", PyMajor: 3, PyMinor: 12,
//		OS: "linux", Arch: "x86_64",
//	}).Compile()
func (b Baseline) Apply(t Target) Target {
	t.Libc = b.Libc
	t.LibcMajor = b.LibcMajor
	t.LibcMinor = b.LibcMinor
	return t
}

type distroBaselineFile struct {
	Source     string     `json:"source"`
	Regenerate string     `json:"regenerate"`
	Notes      []string   `json:"notes"`
	Baselines  []Baseline `json:"baselines"`
}

var (
	baselineList  []Baseline
	baselineIndex map[string]Baseline
)

func init() {
	var file distroBaselineFile
	if err := json.Unmarshal(distroBaselinesJSON, &file); err != nil {
		panic("tags: embedded distro_baselines.json is malformed: " + err.Error())
	}
	baselineList = file.Baselines
	baselineIndex = make(map[string]Baseline, len(baselineList))
	for _, b := range baselineList {
		baselineIndex[baselineKey(b.Distro, b.Release)] = b
	}
}

// distroAliases maps alternative spellings to the canonical names used in the
// table. The keys are the /etc/os-release ID values that differ from the name
// the distribution is usually called by.
//
// Deliberately absent: "rhel". Red Hat Enterprise Linux has no public package
// repository to derive a version from, and while AlmaLinux N and Rocky Linux N
// are 1:1 rebuilds of RHEL N shipping the same glibc, silently answering a
// "rhel" lookup with one of them would put a distribution name in the result
// that the caller never asked about. Look up "almalinux" or "rocky" explicitly.
var distroAliases = map[string]string{
	"amzn":         "amazonlinux",
	"amazon":       "amazonlinux",
	"rocky-linux":  "rocky",
	"alma-linux":   "almalinux",
	"opensuse":     "opensuse-leap",
	"alpine-linux": "alpine",
}

func baselineKey(distro, release string) string {
	d := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(distro)), "_", "-")
	if canonical, ok := distroAliases[d]; ok {
		d = canonical
	}
	return d + "/" + strings.TrimSpace(release)
}

// Baselines returns a copy of the full baseline table, ordered by distribution
// name and then ascending release.
func Baselines() []Baseline {
	out := make([]Baseline, len(baselineList))
	copy(out, baselineList)
	return out
}

// LookupBaseline returns the recorded baseline for one distribution release.
// The distro name is matched case-insensitively, with "_" accepted for "-" and
// a few well-known /etc/os-release IDs aliased (see distroAliases); the release
// must be spelled the way the distribution writes it ("22.04", not "22.4").
func LookupBaseline(distro, release string) (Baseline, bool) {
	b, ok := baselineIndex[baselineKey(distro, release)]
	return b, ok
}

// BaselinesFor returns the recorded distribution releases whose C library is
// new enough to run wheels carrying the given manylinux or musllinux platform
// tag, in table order. It is the distro-facing view of the one comparison PEP
// 600 and PEP 656 actually specify: a manylinux_2_Y wheel runs wherever glibc
// is at least 2.Y.
//
// Both the PEP 600/656 forms ("manylinux_2_28_x86_64", "musllinux_1_2_aarch64")
// and the three legacy aliases ("manylinux1_x86_64", "manylinux2010_x86_64",
// "manylinux2014_aarch64") are accepted. The architecture is validated but does
// not filter the result: a distribution release ships the same libc on every
// architecture it is built for, so the answer is per-release, not per-arch.
//
// An empty result is a real answer -- no recorded release is new enough -- and
// is distinct from an error, which means the tag is not a libc-versioned
// platform tag at all. In particular a bare "linux_x86_64" is an error: it
// carries no libc guarantee to compare against, which is exactly why this
// package ranks it below every manylinux tag.
func BaselinesFor(platformTag string) ([]Baseline, error) {
	libc, major, minor, err := parseLibcPlatformTag(platformTag)
	if err != nil {
		return nil, err
	}
	var out []Baseline
	for _, b := range baselineList {
		if b.Libc != libc {
			continue
		}
		if b.LibcMajor == major && b.LibcMinor >= minor {
			out = append(out, b)
		}
	}
	return out, nil
}

// parseLibcPlatformTag decomposes a manylinux/musllinux platform tag into the
// libc family and minimum version it requires.
func parseLibcPlatformTag(platformTag string) (libc string, major, minor int, err error) {
	fail := func(reason string) (string, int, int, error) {
		return "", 0, 0, fmt.Errorf("%w: %q is not a manylinux/musllinux platform tag: %s", ErrInvalidTag, platformTag, reason)
	}

	var family, rest string
	switch {
	case strings.HasPrefix(platformTag, "manylinux"):
		family, rest = "manylinux", platformTag[len("manylinux"):]
		libc = "glibc"
	case strings.HasPrefix(platformTag, "musllinux"):
		family, rest = "musllinux", platformTag[len("musllinux"):]
		libc = "musl"
	default:
		return fail("unknown platform-tag family")
	}

	// PEP 600/656 form: "_<major>_<minor>_<arch>".
	if strings.HasPrefix(rest, "_") {
		parts := strings.SplitN(rest[1:], "_", 3)
		if len(parts) != 3 {
			return fail("expected " + family + "_<major>_<minor>_<arch>")
		}
		major, err = strconv.Atoi(parts[0])
		if err != nil {
			return fail("unparseable libc major version")
		}
		minor, err = strconv.Atoi(parts[1])
		if err != nil {
			return fail("unparseable libc minor version")
		}
		if _, ok := manylinuxFloor[parts[2]]; !ok {
			return fail("unsupported architecture " + strconv.Quote(parts[2]))
		}
		return libc, major, minor, nil
	}

	// Legacy alias form: "1_<arch>", "2010_<arch>", "2014_<arch>". These exist
	// for manylinux only, and each names a glibc version that depends on the
	// architecture, so they are resolved through the same floor table that
	// generates them.
	if family != "manylinux" {
		return fail("musllinux has no legacy aliases")
	}
	name, arch, found := strings.Cut(rest, "_")
	if !found {
		return fail("expected manylinux<alias>_<arch>")
	}
	floor, ok := manylinuxFloor[arch]
	if !ok {
		return fail("unsupported architecture " + strconv.Quote(arch))
	}
	for _, alias := range floor.legacy {
		if alias.name == "manylinux"+name {
			return libc, alias.major, alias.minor, nil
		}
	}
	return fail("architecture " + strconv.Quote(arch) + " has no legacy alias " + strconv.Quote("manylinux"+name))
}
