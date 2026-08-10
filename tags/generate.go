// SPDX-License-Identifier: Apache-2.0 OR MIT
package tags

import (
	"fmt"
	"strconv"
)

// generateTags builds the full ordered, most-preferred-first list of
// compatible tags for a (validated) Target. The ordering follows the
// pypa/packaging convention (packaging.tags.cpython_tags /
// packaging.tags.generic_tags / packaging.tags.compatible_tags, composed the
// way packaging.tags.sys_tags composes them), fixed to a specific declared
// target rather than the running interpreter/host:
//
//   - "cp" targets: cp<XY>-cp<XY>-<plat> (exact ABI), then cp<XY>-abi3-<plat>,
//     then cp<XY>-none-<plat>, then the abi3 walk down through older minor
//     versions to cp32-abi3-<plat>; followed by the "compatible tier"
//     py<XY>-none-<plat> down to py<X>0-none-<plat>; then cp<XY>-none-any;
//     then the universal py*-none-any tail. A free-threaded target substitutes
//     cp<XY>t for the exact ABI and abi3t for abi3 throughout.
//   - "pp" targets: pp<XY>-pypy<XY>_pp<IJ>-<plat> (the implementation ABI),
//     then pp<XY>-none-<plat>; followed by the same compatible tier, whose
//     "-none-any" entry is the MAJOR-only pp<X>-none-any, then the universal
//     tail. There is no stable-ABI tier: abi3 is a CPython concept.
//   - "py" targets: the same compatible tier and universal tail, without any
//     interpreter-specific entries (there is no compiled ABI to bind to, and
//     no implementation to name in a "-none-any" tag).
func generateTags(t Target) ([]Tag, error) {
	plats, err := t.platformTags()
	if err != nil {
		return nil, err
	}
	switch t.Implementation {
	case "cp":
		return cpTags(t, plats), nil
	case "pp":
		return implTags(t, plats), nil
	default:
		return pyTags(t, plats), nil
	}
}

// cpTags mirrors packaging.tags.cpython_tags followed by
// compatible_tags(interpreter="cp<XY>").
func cpTags(t Target, plats []string) []Tag {
	interp := interpTag("cp", t.PyMajor, t.PyMinor)

	var out []Tag
	appendPlatform := func(tagInterp, abi string) {
		for _, p := range plats {
			out = append(out, Tag{tagInterp, abi, p})
		}
	}

	// Exact ABI: the free-threaded build's abiflags carry a "t"
	// (cp313-cp313t-<plat>), which is what makes it a distinct ABI.
	exactABI := interp
	if t.FreeThreaded {
		exactABI += "t"
	}
	appendPlatform(interp, exactABI)

	// The stable ABI was introduced in Python 3.2 (PEP 384). A free-threaded
	// build does not support it -- packaging's _abi3_applies is explicitly
	// false when threading -- and takes PEP 803's abi3t in its place, both in
	// this slot and in the descending walk below. The two are mutually
	// exclusive, so there is a single stable-ABI name here rather than two
	// tiers.
	//
	// The floor is a version comparison, matching _abi3_applies' own
	// `tuple(python_version) >= (3, 2)`: a hypothetical Python 4.0 target still
	// gets the stable ABI. Pinned by the cp40 golden fixture.
	stableABI := ""
	if versionAtLeast(t.PyMajor, t.PyMinor, 3, 2) {
		if t.FreeThreaded {
			stableABI = "abi3t"
		} else {
			stableABI = "abi3"
		}
	}
	if stableABI != "" {
		appendPlatform(interp, stableABI)
	}

	appendPlatform(interp, "none")

	if stableABI != "" {
		for minor := t.PyMinor - 1; minor >= 2; minor-- {
			appendPlatform(interpTag("cp", t.PyMajor, minor), stableABI)
		}
	}

	return append(out, compatibleTags(t, plats, interp)...)
}

// implTags mirrors packaging.tags.generic_tags followed by the compatible tier
// packaging.tags.sys_tags pairs with it for a non-CPython interpreter, whose
// "-none-any" interpreter is the major-only form ("pp3", not "pp310").
func implTags(t Target, plats []string) []Tag {
	interp := interpTag(t.Implementation, t.PyMajor, t.PyMinor)

	var out []Tag
	if abi := t.implABI(); abi != "" {
		for _, p := range plats {
			out = append(out, Tag{interp, abi, p})
		}
	}
	// generic_tags appends "none" to the ABI list itself, so this tier is
	// present even when the implementation's own version is unknown.
	for _, p := range plats {
		out = append(out, Tag{interp, "none", p})
	}

	majorInterp := t.Implementation + strconv.Itoa(t.PyMajor)
	return append(out, compatibleTags(t, plats, majorInterp)...)
}

// implABI is the implementation-specific ABI tag naming both the Python
// version and the implementation's own version. PyPy 7.3 for Python 3.10
// spells it "pypy310_pp73" (packaging derives this from the interpreter's
// EXT_SUFFIX, ".pypy310-pp73-<plat>.so", in _generic_abi). It is empty when
// the implementation version is unknown, or for an implementation with no
// such ABI spelling.
func (t Target) implABI() string {
	if t.Implementation != "pp" || (t.ImplMajor == 0 && t.ImplMinor == 0) {
		return ""
	}
	return fmt.Sprintf("pypy%d%d_pp%d%d", t.PyMajor, t.PyMinor, t.ImplMajor, t.ImplMinor)
}

func pyTags(t Target, plats []string) []Tag {
	// A bare "py" target names no implementation, so it gets no
	// "<interp>-none-any" entry: packaging's sys_tags passes interpreter=None
	// to compatible_tags for any interpreter it does not recognize.
	return compatibleTags(t, plats, "")
}

// compatibleTags mirrors packaging.tags.compatible_tags: the py<XY>-none-<plat>
// tier over every platform, then "<interp>-none-any" if an interpreter is
// named, then the universal py<XY>-none-any tail.
func compatibleTags(t Target, plats []string, interp string) []Tag {
	pyRange := pyInterpreterRange(t.PyMajor, t.PyMinor)

	out := make([]Tag, 0, len(pyRange)*(len(plats)+1)+1)
	for _, v := range pyRange {
		for _, p := range plats {
			out = append(out, Tag{v, "none", p})
		}
	}
	if interp != "" {
		out = append(out, Tag{interp, "none", "any"})
	}
	for _, v := range pyRange {
		out = append(out, Tag{v, "none", "any"})
	}
	return out
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

// versionAtLeast reports whether (major, minor) >= (floorMajor, floorMinor),
// compared as a version.
//
// It exists because the obvious spelling of a version floor --
// "major == floorMajor && minor >= floorMinor" -- is not one. It silently
// answers "no" for every input whose major differs, including every input
// ABOVE the floor. Upstream compares Python versions as tuples
// (packaging.tags._abi3_applies does `tuple(python_version) >= (3, 2)`), which
// is lexicographic; so does this.
//
// Use it for every (major, minor) floor in this package. The one place that
// deliberately does NOT is manylinuxTags -- see the comment there, which
// records what upstream does across glibc majors and why we do not follow it.
func versionAtLeast(major, minor, floorMajor, floorMinor int) bool {
	if major != floorMajor {
		return major > floorMajor
	}
	return minor >= floorMinor
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
		return macosPlatformTags(t.Arch, t.MacMajor, t.MacMinor), nil
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
// after the version they alias.
//
// This is the one (major, minor) floor in this package that is deliberately NOT
// the lexicographic versionAtLeast comparison: only glibc versions sharing the
// target's declared major are considered, so a target whose major differs from
// its floor's yields no manylinux tags at all.
//
// That IS a divergence from upstream, and it is a considered one rather than an
// oversight. packaging's _manylinux.platform_tags walks older majors too --
// "We can assume compatibility across glibc major versions", citing
// sourceware bug 24636 -- and to enumerate them it needs to know the last minor
// version each older major reached. It gets that from _LAST_GLIBC_MINOR, a
// defaultdict whose fallback is 50 and whose own comment reads "guess what the
// highest minor version might be, assume it will be 50 for testing. Once this
// actually happens, update the dictionary with the actual value."
//
// Mirroring that would mean this library materializing up to ~50 tags per older
// major naming glibc releases that do not exist, on the strength of a
// placeholder upstream flags as a guess. glibc's major has been 2 since 1997,
// so no reachable input distinguishes the two behaviors; between a documented
// narrower answer and a confidently-invented wider one, the narrow answer is
// the safer default for a server deciding which wheel to hand a client. If
// glibc 3 ever ships, this is the function to revisit, and the real
// _LAST_GLIBC_MINOR[2] will be a fact by then rather than a guess.
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
// newest first. Unlike manylinux there is no architecture-dependent floor and
// there are no legacy aliases.
//
// The declared major is used verbatim, which is what upstream does:
// _musllinux.platform_tags yields
// "musllinux_{sys_musl.major}_{minor}" over range(minor, -1, -1), with no
// notion of a single blessed major. Every musl release to date is 1.x, so this
// only matters for a musl 2 that does not exist yet -- but hardcoding "1" here
// meant a musl 2 target silently got NO musllinux tags at all, which is a worse
// answer than the obvious one. Pinned by the musl 2.3 golden fixture.
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

// macosFatBinaryFloorMinor is the oldest macOS 10.x minor version for which
// x86_64 binary formats are defined at all: pypa/packaging's
// _mac_binary_formats returns an empty format list for x86_64 below (10, 4),
// so macosx_10_3 and older yield no tags whatsoever. (10.4 Tiger was the first
// release with Intel support.)
const macosFatBinaryFloorMinor = 4

// macosCompatTailMinor is the highest macOS 10.x minor version an 11-or-later
// target claims compatibility with. macOS 11 reports itself as "10.16" to
// binaries built against an older SDK, so the pre-11 compatibility tail starts
// there rather than at 10.15. Mirrors the range(16, 3, -1) in
// pypa/packaging's mac_platforms.
const macosCompatTailMinor = 16

// macosBinaryFormatsFor is pypa/packaging's _mac_binary_formats restricted to
// the archs this package supports: the format list an arch may claim at a
// given macOS version, which for x86_64 is empty below 10.4.
func macosBinaryFormatsFor(major, minor int, arch string) []string {
	formats, ok := macosBinaryFormats[arch]
	if !ok {
		// Unreachable: Target.validate restricts macOS Arch to macosArchs,
		// and every entry there has a macosBinaryFormats list.
		panic("tags: no binary formats for macOS arch " + arch)
	}
	// Upstream's gate is the tuple comparison `version < (10, 4)`, so it is a
	// version comparison here too rather than a same-major one. (Target.validate
	// rejects a macOS major below 10, so the two agree on every reachable
	// input; spelling it as a version keeps it agreeing if that ever changes.)
	if arch == "x86_64" && !versionAtLeast(major, minor, 10, macosFatBinaryFloorMinor) {
		return nil
	}
	return formats
}

// macosPlatformTags returns the ordered platform-tag list for a macOS Target,
// mirroring pypa/packaging's mac_platforms((major, minor), arch). macOS
// changed its version scheme at 11, and so does the walk:
//
//   - A declared 10.x target walks the MINOR version down, "macosx_10_<m>_<fmt>"
//     from the declared minor to 10.4 (below which no x86_64 format exists).
//     There is no 11+ section and no compatibility tail: a macOS 10 host cannot
//     run an 11+ binary.
//   - A declared 11-or-later target walks the MAJOR version down,
//     "macosx_<M>_0_<fmt>" from the declared major to 11, and then adds the
//     pre-11 compatibility tail (macOS 11+ still runs older binaries):
//     macosx_10_16 down to macosx_10_4. On x86_64 the tail carries the full
//     format list; on any other arch only "universal2", since a
//     single-architecture pre-11 binary cannot contain arm64 code, but the
//     x86_64 half of a universal2 binary can declare a pre-11 minimum.
//
// Each version is paired with every binary format the architecture supports at
// that version.
func macosPlatformTags(arch string, major, minor int) []string {
	var out []string
	appendVersion := func(vMajor, vMinor int, formats []string) {
		for _, f := range formats {
			out = append(out, fmt.Sprintf("macosx_%d_%d_%s", vMajor, vMinor, f))
		}
	}

	if major == 10 {
		for m := minor; m >= 0; m-- {
			appendVersion(10, m, macosBinaryFormatsFor(10, m, arch))
		}
		return out
	}

	for m := major; m >= 11; m-- {
		appendVersion(m, 0, macosBinaryFormatsFor(m, 0, arch))
	}
	for m := macosCompatTailMinor; m >= macosFatBinaryFloorMinor; m-- {
		if arch == "x86_64" {
			appendVersion(10, m, macosBinaryFormatsFor(10, m, arch))
		} else {
			appendVersion(10, m, []string{"universal2"})
		}
	}
	return out
}
