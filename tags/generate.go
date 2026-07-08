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
// one platform tag per target, e.g. macOS format fallbacks in Task 4).
//
// linux and macOS generation are not yet implemented (see #18632 Task 3/4);
// until they land, both return ErrUnsupportedTarget rather than panicking,
// so that Target.Compile keeps its "validates and returns an error" contract
// for every OS Target.validate currently accepts.
func (t Target) platformTags() ([]string, error) {
	switch t.OS {
	case "windows":
		return windowsPlatformTags(t.Arch), nil
	case "linux":
		// Implemented in #18632 Task 3.
		return nil, fmt.Errorf("%w: linux tag generation not yet implemented", ErrUnsupportedTarget)
	case "macos":
		// Implemented in #18632 Task 4.
		return nil, fmt.Errorf("%w: macos tag generation not yet implemented", ErrUnsupportedTarget)
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
