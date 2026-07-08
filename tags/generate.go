// SPDX-License-Identifier: Apache-2.0 OR MIT
package tags

import (
	"fmt"
	"strconv"
)

// generateTags builds the full ordered, most-preferred-first list of
// compatible tags for a (validated) Target. The ordering follows the
// pypa/packaging convention (packaging.tags.cpython_tags /
// packaging.tags.compatible_tags), fixed to a specific declared target
// rather than the running interpreter/host:
//
//   - "cp" targets: cp<XY>-cp<XY>-<plat> (exact ABI), then cp<XY>-abi3-<plat>,
//     then cp<XY>-none-<plat>, then the abi3 walk down through older minor
//     versions to cp32-abi3-<plat>; followed by the "compatible tier"
//     py<XY>-none-<plat> down to py<X>0-none-<plat>; then cp<XY>-none-any;
//     then the universal py*-none-any tail.
//   - "py" targets: the same compatible tier and universal tail, without any
//     interpreter-specific entries (there is no compiled ABI to bind to).
func generateTags(t Target) ([]Tag, error) {
	if t.Implementation == "cp" {
		return cpTags(t)
	}
	return pyTags(t)
}

func cpTags(t Target) ([]Tag, error) {
	plats, err := t.platformTags()
	if err != nil {
		return nil, err
	}
	interp := interpTag("cp", t.PyMajor, t.PyMinor)

	var out []Tag
	appendPlatform := func(abi string) {
		for _, p := range plats {
			out = append(out, Tag{interp, abi, p})
		}
	}

	// Exact ABI.
	appendPlatform(interp)

	// abi3 was introduced in Python 3.2; free-threaded ABIs are out of scope
	// for now (see deferred-features follow-up).
	useAbi3 := t.PyMajor == 3 && t.PyMinor >= 2
	if useAbi3 {
		appendPlatform("abi3")
	}

	appendPlatform("none")

	if useAbi3 {
		for minor := t.PyMinor - 1; minor >= 2; minor-- {
			oldInterp := interpTag("cp", t.PyMajor, minor)
			for _, p := range plats {
				out = append(out, Tag{oldInterp, "abi3", p})
			}
		}
	}

	pyRange := pyInterpreterRange(t.PyMajor, t.PyMinor)
	for _, v := range pyRange {
		for _, p := range plats {
			out = append(out, Tag{v, "none", p})
		}
	}

	out = append(out, Tag{interp, "none", "any"})

	for _, v := range pyRange {
		out = append(out, Tag{v, "none", "any"})
	}

	return out, nil
}

func pyTags(t Target) ([]Tag, error) {
	plats, err := t.platformTags()
	if err != nil {
		return nil, err
	}
	pyRange := pyInterpreterRange(t.PyMajor, t.PyMinor)

	var out []Tag
	for _, v := range pyRange {
		for _, p := range plats {
			out = append(out, Tag{v, "none", p})
		}
	}
	for _, v := range pyRange {
		out = append(out, Tag{v, "none", "any"})
	}
	return out, nil
}

// pyInterpreterRange mirrors packaging._py_interpreter_range: the exact
// "py<major><minor>" version, then the major-only "py<major>" version, then
// every older minor version down to "py<major>0", descending.
func pyInterpreterRange(major, minor int) []string {
	out := make([]string, 0, minor+2)
	out = append(out, interpTag("py", major, minor))
	out = append(out, "py"+strconv.Itoa(major))
	for m := minor - 1; m >= 0; m-- {
		out = append(out, interpTag("py", major, m))
	}
	return out
}

func interpTag(prefix string, major, minor int) string {
	return prefix + strconv.Itoa(major) + strconv.Itoa(minor)
}

// platformTags returns the target's platform component strings (there is
// usually exactly one, but the shape stays a slice for symmetry with the
// underlying pypa/packaging generators, some of which enumerate more than
// one platform tag per target, e.g. macOS's per-major-version format
// fallbacks).
func (t Target) platformTags() ([]string, error) {
	switch t.OS {
	case "windows":
		return windowsPlatformTags(t.Arch), nil
	case "linux":
		return linuxPlatformTags(t), nil
	case "macos":
		return macosPlatformTags(t.Arch, t.MacMajor), nil
	default:
		// Unreachable: Target.validate rejects any other OS before
		// generateTags is ever called.
		panic("tags: unsupported OS " + t.OS)
	}
}

func windowsPlatformTags(arch string) []string {
	switch arch {
	case "amd64":
		return []string{"win_amd64"}
	case "x86":
		return []string{"win32"}
	case "arm64":
		return []string{"win_arm64"}
	default:
		// Unreachable: Target.validate rejects any other windows arch.
		panic("tags: unsupported windows arch " + arch)
	}
}

// legacyManylinuxAlias is a pre-PEP 600 manylinux tag ("manylinux1",
// "manylinux2010", "manylinux2014") that aliases one specific glibc version
// for a given architecture.
type legacyManylinuxAlias struct {
	name  string
	major int
	minor int
}

// manylinuxFloor is the oldest glibc version (major, minor) a given
// architecture's manylinux tags may claim, plus any legacy aliases that
// architecture supports. Values are uv's (github.com/astral-sh/uv) floor
// table, which matches pypa/packaging for x86_64/i686/aarch64/armv7l/
// ppc64/ppc64le/s390x but additionally floors riscv64 and loongarch64 --
// architectures packaging's own manylinux support predates.
//
// Rule (Global Constraints): a target declaring glibc (2, m) accepts a
// manylinux_2_y tag iff m >= y, down to this floor.
var manylinuxFloor = map[string]struct {
	major, minor int
	legacy       []legacyManylinuxAlias
}{
	"x86_64": {2, 5, []legacyManylinuxAlias{
		{"manylinux1", 2, 5}, {"manylinux2010", 2, 12}, {"manylinux2014", 2, 17},
	}},
	"i686": {2, 5, []legacyManylinuxAlias{
		{"manylinux1", 2, 5}, {"manylinux2010", 2, 12}, {"manylinux2014", 2, 17},
	}},
	"aarch64": {2, 17, []legacyManylinuxAlias{{"manylinux2014", 2, 17}}},
	"armv7l":  {2, 17, []legacyManylinuxAlias{{"manylinux2014", 2, 17}}},
	"ppc64":   {2, 17, []legacyManylinuxAlias{{"manylinux2014", 2, 17}}},
	"ppc64le": {2, 17, []legacyManylinuxAlias{{"manylinux2014", 2, 17}}},
	"s390x":   {2, 17, []legacyManylinuxAlias{{"manylinux2014", 2, 17}}},
	"riscv64": {2, 31, nil},
	// loongarch64's floor is uv's; pypa/packaging does not yet recognize
	// this architecture at all.
	"loongarch64": {2, 36, nil},
}

// linuxPlatformTags builds the ordered platform-tag list for a linux
// Target: manylinux tags (glibc targets) or musllinux tags (musl targets)
// from newest to oldest down to the architecture's floor, each accompanied
// by any applicable legacy manylinux alias immediately after the version it
// aliases, followed last by the bare "linux_<arch>" tag -- a deliberate
// divergence from pypa/packaging (which does not emit bare linux_* tags at
// all) to support PPM's local-source/Git-builder wheels, ranked below every
// manylinux/musllinux tag since those carry a documented compatibility
// guarantee that a bare linux tag does not.
func linuxPlatformTags(t Target) []string {
	var plats []string
	switch t.Libc {
	case "glibc":
		plats = manylinuxTags(t.Arch, t.LibcMajor, t.LibcMinor)
	case "musl":
		plats = musllinuxTags(t.Arch, t.LibcMajor, t.LibcMinor)
	default:
		// Unreachable: Target.validate requires Libc to be "glibc" or
		// "musl".
		panic("tags: unsupported linux libc " + t.Libc)
	}
	return append(plats, "linux_"+t.Arch)
}

// manylinuxTags returns "manylinux_<major>_<minor>_<arch>" for every glibc
// version from the target's declared version down to the architecture's
// floor (inclusive), newest first, interleaving legacy aliases immediately
// after the version they alias. Only floors within the target's declared
// glibc major version are considered (per the Global Constraints rule,
// stated only for glibc major 2, which every currently defined floor uses);
// a target whose major doesn't match its floor's, or whose declared version
// is below the floor, yields no manylinux tags.
func manylinuxTags(arch string, major, minor int) []string {
	floor, ok := manylinuxFloor[arch]
	if !ok {
		// Unreachable: Target.validate restricts linux Arch to linuxArchs,
		// and every entry there has a manylinuxFloor.
		panic("tags: no manylinux floor for arch " + arch)
	}
	if major != floor.major || minor < floor.minor {
		return nil
	}
	out := make([]string, 0, (minor-floor.minor+1)+len(floor.legacy))
	for m := minor; m >= floor.minor; m-- {
		out = append(out, fmt.Sprintf("manylinux_%d_%d_%s", major, m, arch))
		for _, alias := range floor.legacy {
			if alias.major == major && alias.minor == m {
				out = append(out, alias.name+"_"+arch)
			}
		}
	}
	return out
}

// musllinuxTags returns "musllinux_<major>_<minor>_<arch>" for every musl
// version from the target's declared version down to musllinux_<major>_0,
// newest first. Unlike manylinux there is no architecture-dependent floor or
// legacy alias.
func musllinuxTags(arch string, major, minor int) []string {
	out := make([]string, 0, minor+1)
	for m := minor; m >= 0; m-- {
		out = append(out, fmt.Sprintf("musllinux_%d_%d_%s", major, m, arch))
	}
	return out
}

// macosBinaryFormats is the ordered list of binary-format suffixes a macOS
// architecture's tags may claim (per pypa/packaging's _mac_binary_formats,
// restricted to the archs this package supports). "intel"/"fat64"/"fat32"
// are legacy 32/64-bit-Intel umbrella formats that only ever applied to
// x86_64 (and, pre-Intel-64, i386/ppc, which are out of scope here);
// "universal"/"universal2" are fat binaries spanning multiple architectures.
var macosBinaryFormats = map[string][]string{
	"x86_64": {"x86_64", "intel", "fat64", "fat32", "universal2", "universal"},
	"arm64":  {"arm64", "universal2"},
}

// macosPlatformTags returns the ordered platform-tag list for a macOS
// Target: "macosx_<M>_0_<fmt>" for every major version M from the target's
// declared major version down to 11 (inclusive), newest first, each paired
// with every binary format the architecture supports. Only macOS 11+ is
// supported (Global Constraints: "macOS: 11+ only" -- pre-11's yearly
// major-version-10 minor-bump numbering, and pypa/packaging's compatibility
// tail of pre-11 "macosx_10_<n>_universal2" fallbacks, are both deferred).
func macosPlatformTags(arch string, major int) []string {
	formats, ok := macosBinaryFormats[arch]
	if !ok {
		// Unreachable: Target.validate restricts macOS Arch to macosArchs,
		// and every entry there has a macosBinaryFormats list.
		panic("tags: no binary formats for macOS arch " + arch)
	}
	out := make([]string, 0, (major-11+1)*len(formats))
	for m := major; m >= 11; m-- {
		for _, f := range formats {
			out = append(out, fmt.Sprintf("macosx_%d_0_%s", m, f))
		}
	}
	return out
}
