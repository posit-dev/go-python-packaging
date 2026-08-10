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
//     versions to cp<X>2-abi3-<plat> -- cp32 for a Python 3 target; the floor is
//     minor 2 within the target's own major; followed by the "compatible tier"
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

	appendPlatform(interp, cpythonExactABI(t))

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

// pymallocMaxMinor and ucs4MaxMinor bound the two legacy CPython 3.x abiflags:
// the pymalloc "m" flag exists for Python 3.x below 3.8 (3.8 removed the
// distinction), and the UCS-4 "u" flag for 3.x below 3.3 (PEP 393 made the
// unicode representation dynamic). Both also apply to every Python 2.x.
const (
	pymallocMaxMinor = 8
	ucs4MaxMinor     = 3
)

// cpythonExactABI is the ABI component of a CPython target's most specific tag,
// mirroring packaging.tags._cpython_abis. It is NOT simply "cp<XY>": the ABI
// carries abiflags, in upstream's order
// "cp<version><threading><debug><pymalloc><ucs4>".
//
//	3.13+ free-threaded   cp313t   (PEP 703; see Target.FreeThreaded)
//	3.3 .. 3.7           cp37m    pymalloc
//	2.x and 3.0 .. 3.2   cp27mu   pymalloc + UCS-4
//	3.8+                 cp312    no flags
//
// Getting this wrong is not cosmetic. Every real CPython 3.7 extension wheel on
// PyPI is tagged cp37-cp37m, so emitting a bare "cp37" ABI matches none of them
// and silently sends a 3.7 client to an sdist.
//
// The debug flag "d" is deliberately absent: upstream infers it from the running
// interpreter (Py_DEBUG, sys.gettotalrefcount, a "_d.pyd" extension suffix),
// which says nothing about a declared target, and debug-build wheels are not
// published.
//
// ⚠️ Known, deliberate limitation of the UCS-4 flag. Read this before "fixing"
// it.
//
// (a) Upstream and this package are answering different questions, and this one
// has no faithful answer. packaging.tags asks "what can THIS RUNNING
// interpreter install", so it simply reads Py_UNICODE_SIZE and falls back to
// sys.maxunicode. This package asks "what can a DECLARED target install", and a
// declared target does not carry its Unicode width -- there is nothing to read.
// So reproducing upstream here is not possible even in principle; every possible
// behavior is a choice, and this one is chosen deliberately: answer as upstream
// would, which on any modern host means "u" for every 2.x target regardless of
// the OS the target names.
//
// (b) The practical consequence. UCS-4 was the Unix default while Windows and
// macOS CPython 2.x were UCS-2 -- that is exactly the cp27mu vs cp27m split
// visible on PyPI. So for Linux 2.x targets this is right, and for a Windows or
// macOS 2.x target it names an ABI that no real wheel carries: such a target
// matches no CPython 2.x extension wheel at all.
//
// (c) If 2.x targets ever matter, the likely fix is to emit BOTH ABI variants
// (cp27m and cp27mu) and let ranking decide, NOT to guess the width from
// Target.OS. Emitting both is complete -- it cannot miss a real wheel, and the
// variant that does not exist for a given host is inert because no wheel carries
// it. Guessing from Target.OS is merely a better guess, and it would still be
// wrong for the UCS-2 Linux builds that CPython's --enable-unicode=ucs2 could
// produce. Note that either choice gives up this package's byte-identical
// conformance with the reference, which is its main safety net, so the trade only
// makes sense once something reachable depends on it.
//
// Why it is not being done now: Python 2 has been end-of-life since 2020 and is
// out of scope for the native resolver this module serves, so the divergence
// would buy nothing reachable. See also doc.go's divergence list.
func cpythonExactABI(t Target) string {
	abi := interpTag("cp", t.PyMajor, t.PyMinor)
	if t.FreeThreaded {
		abi += "t"
	}
	// Upstream's gates are the tuple comparisons `py_version < (3, 8)` and
	// `py_version < (3, 3)`, so these are version comparisons -- the negation of
	// versionAtLeast -- and not bare minor checks. Writing them as
	// "PyMajor < 3 || PyMinor < 8" instead puts an "m" on a Python 4.0 target,
	// which the cp40 golden fixture catches immediately.
	if !versionAtLeast(t.PyMajor, t.PyMinor, 3, pymallocMaxMinor) {
		abi += "m"
	}
	if !versionAtLeast(t.PyMajor, t.PyMinor, 3, ucs4MaxMinor) {
		abi += "u"
	}
	return abi
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
// Use it for every (major, minor) floor in this package, with two deliberate
// exceptions -- listed so that "this one doesn't use the helper" is never by
// itself evidence of a bug:
//
//   - manylinuxTags does not call it because it walks glibc majors explicitly,
//     mirroring upstream's cross-major compatibility assumption. A single
//     boolean floor is the wrong SHAPE there, not the wrong comparison.
//   - Target.validate's free-threaded gate (target.go) really does want an
//     exact-major test. It refuses an unknown major rather than flooring it,
//     because there is no reference implementation to check a Python 4
//     free-threaded ABI spelling against. See the comment there.
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
// architecture supports. Values are uv's (github.com/astral-sh/uv) floor table,
// which matches pypa/packaging for x86_64/i686/aarch64/armv7l/ppc64/ppc64le/
// s390x but floors riscv64 at 2.31 and loongarch64 at 2.36.
//
// ⚠️ Those last two are NARROWER than pypa/packaging 26.2, which floors every
// non-x86 architecture at glibc 2.17 and does list both in its _ALLOWED_ARCHS
// (verified against the installed 26.2; an earlier version of this comment
// claimed packaging did not recognize them at all, which is no longer true, if
// it ever was). So a riscv64 or loongarch64 target here declines manylinux tags
// between 2.17 and its floor that pip on the same host would accept. That is a
// real divergence on real architectures, inherited from #18632, and is
// deliberately left alone here rather than changed as a drive-by: it is the
// opposite direction from the cross-major fix below and deserves its own
// decision. Tracked for follow-up.
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
	// loongarch64's floor is uv's 2.36. pypa/packaging 26.2 does recognize this
	// architecture -- it is in _ALLOWED_ARCHS -- but floors it at 2.17 like
	// every other non-x86 arch; see the divergence note above.
	"loongarch64": {2, 36, nil},
}

// linuxPlatformTags builds the ordered platform-tag list for a linux
// Target: manylinux tags (glibc targets) or musllinux tags (musl targets)
// from newest to oldest -- manylinux down to the architecture's floor within the
// floor's own glibc major and to <major>_0 in any other major, musllinux down to
// musllinux_<major>_0, since musl has no architecture floor -- each accompanied
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

// lastGlibcMinor is the highest minor version assumed for a glibc major older
// than the target's own. It is only consulted when a target declares a glibc
// major above the floor's, since only then is there an older major to
// enumerate.
//
// The value is upstream's, and upstream is explicit that it is a placeholder.
// packaging/_manylinux.py declares it as
// `_LAST_GLIBC_MINOR: dict[int, int] = collections.defaultdict(lambda: 50)`
// under this comment, quoted so its provenance travels with the number:
//
//	# If glibc ever changes its major version, we need to know what the last
//	# minor version was, so we can build the complete list of all versions.
//	# For now, guess what the highest minor version might be, assume it will
//	# be 50 for testing. Once this actually happens, update the dictionary
//	# with the actual value.
//
// Copying the placeholder rather than inventing our own means we inherit
// upstream's correction when glibc 3 actually ships. Upstream pinned at
// 6ce6143ac8eebd91b7b0d38e92618f0702e933af (packaging 26.2).
const lastGlibcMinor = 50

// glibcVersion is a (major, minor) glibc version used while building the
// manylinux walk.
type glibcVersion struct {
	major int
	minor int
}

// manylinuxTags returns the ordered manylinux platform tags for a glibc target,
// newest first, interleaving legacy aliases immediately after the version they
// alias. It mirrors pypa/packaging's _manylinux.platform_tags.
//
// glibc guarantees compatibility across major versions -- upstream's comment is
// "We can assume compatibility across glibc major versions", citing
// https://sourceware.org/bugzilla/show_bug.cgi?id=24636 -- so a target does not
// merely claim its own major. The walk is:
//
//   - the declared version's own major, from the declared minor downward;
//   - then every older major down to the architecture's floor major, each
//     entered at lastGlibcMinor since we cannot know where an unreleased major
//     will stop.
//
// Within the floor's own major the walk bottoms out at that architecture's floor
// minor from manylinuxFloor -- 2.5 on x86_64/i686, 2.17 on most others, but 2.31
// on riscv64 and 2.36 on loongarch64; any other major goes down to <major>_0.
//
// So "below the floor" has two different answers, and conflating them is how this
// function's narrower predecessor looked correct:
//
//   - A declared version in the floor's OWN major but below its floor minor
//     yields nothing at all. glibc 2.12 on aarch64 emits no manylinux tag,
//     because that architecture's manylinux series begins at 2.17.
//   - A declared version in a LOWER major yields that major's own walk,
//     manylinux_<major>_<minor> down to manylinux_<major>_0, and no major-2 tags
//     (upstream only walks *older* majors, never newer ones). glibc 1.5 on
//     x86_64 emits manylinux_1_5 through manylinux_1_0. Measured against
//     packaging 26.2 and pinned by TestLinux_CrossGlibcMajor.
//
// If you are here because tags are being emitted where you expected none, it is
// probably the second case, and it is deliberate -- see the paragraph below on
// why being narrower than upstream is the expensive direction.
//
// Being no NARROWER than upstream is the property that matters here, because the
// consumer of this list is pip's own tag logic. A tag naming a glibc release
// that does not exist is inert -- no wheel is ever tagged with it, so it matches
// nothing and costs only list length. A tag we fail to emit is a false negative:
// we would decline a wheel that pip on that same host would install, and a
// server disagreeing with the client it serves is the failure that reaches
// users. Hence mirroring upstream even where its input is an admitted guess.
//
// No real target is affected: glibc's major has been 2 since 1997, and the
// older-major loop below does not execute for a major-2 target. Pinned in both
// directions by golden fixtures -- glibc 2.x targets are byte-identical, and
// glibc 3.5 x86_64/aarch64 fixtures record the cross-major walk.
func manylinuxTags(arch string, major, minor int) []string {
	floor, ok := manylinuxFloor[arch]
	if !ok {
		// Unreachable: Target.validate restricts linux Arch to linuxArchs,
		// and every entry there has a manylinuxFloor.
		panic("tags: no manylinux floor for arch " + arch)
	}

	// maxima mirrors upstream's glibc_max_list: the declared version, then each
	// older major capped at lastGlibcMinor. For a major-2 target (every real
	// one) this loop adds nothing and the result is the declared major alone.
	maxima := []glibcVersion{{major, minor}}
	for m := major - 1; m >= floor.major; m-- {
		maxima = append(maxima, glibcVersion{m, lastGlibcMinor})
	}

	var out []string
	for _, max := range maxima {
		// Upstream floors the arch-specific "too old" minor only within the
		// major that floor belongs to; for any other major the oldest supported
		// is (x, 0).
		minMinor := 0
		if max.major == floor.major {
			minMinor = floor.minor
		}
		for m := max.minor; m >= minMinor; m-- {
			out = append(out, fmt.Sprintf("manylinux_%d_%d_%s", max.major, m, arch))
			for _, alias := range floor.legacy {
				if alias.major == max.major && alias.minor == m {
					out = append(out, alias.name+"_"+arch)
				}
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
