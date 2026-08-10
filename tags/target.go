// SPDX-License-Identifier: Apache-2.0 OR MIT
package tags

import (
	"errors"
	"fmt"
)

// ErrUnsupportedTarget is returned by Target.Compile for a Target that is
// malformed or describes a platform combination this package does not (yet)
// support.
var ErrUnsupportedTarget = errors.New("unsupported or invalid target")

// Target describes the interpreter/platform combination a Matcher should be
// compiled for. Not every field is meaningful for every OS; see Compile for
// the exact validation rules.
type Target struct {
	// Implementation is the interpreter family: "cp" (CPython), "pp" (PyPy),
	// or "py" (implementation-agnostic / pure Python).
	Implementation string
	// PyMajor and PyMinor are the target Python version, e.g. 3, 11 for
	// Python 3.11.
	PyMajor int
	PyMinor int

	// FreeThreaded selects the PEP 703 free-threaded ("t"-suffixed) CPython
	// ABI: a free-threaded CPython 3.13 target consumes cp313-cp313t-* wheels
	// rather than cp313-cp313-*. It is only meaningful for Implementation
	// "cp", and only from Python 3.13 on (the first version with a
	// free-threaded build); Compile rejects it elsewhere.
	//
	// A free-threaded target does NOT accept abi3 wheels: the GIL-enabled
	// stable ABI is a different ABI. Its stable-ABI counterpart is PEP 803's
	// abi3t, which this package generates in abi3's place.
	FreeThreaded bool

	// ImplMajor and ImplMinor are the *implementation's* own version, as
	// distinct from the Python language version: PyPy 7.3 for Python 3.10
	// spells its ABI "pypy310_pp73", where the "73" comes from these fields.
	// They are only meaningful for Implementation "pp".
	//
	// Both zero means "implementation version unknown": the target then
	// accepts no implementation-specific ABI wheels, only the
	// "pp<XY>-none-<platform>" tier and the compatible tier below it.
	ImplMajor int
	ImplMinor int

	// OS is one of "linux", "macos", "windows".
	OS string
	// Arch is the target CPU architecture; valid values are OS-dependent.
	Arch string

	// Libc and LibcMajor/LibcMinor are required when OS == "linux".
	// Libc is "glibc" or "musl".
	Libc      string
	LibcMajor int
	LibcMinor int

	// MacMajor and MacMinor are required when OS == "macos".
	//
	// MacMinor is only load-bearing for the legacy major-version-10 numbering
	// (macOS 10.15 is a different target from macOS 10.9). From macOS 11 on,
	// each yearly release bumps the major version and the minor version is a
	// midyear update, so generated tags always use "<major>_0" and MacMinor
	// does not affect the tag set; it is still validated (must be >= 0).
	//
	// MacMajor 10 is accepted for Arch "x86_64" only: Apple silicon shipped
	// with macOS 11, so there is no such thing as a macOS 10.x arm64 host.
	MacMajor int
	MacMinor int
}

// Compile validates the Target and, if valid, builds the ordered set of
// compatible tags into a reusable Matcher.
func (t Target) Compile() (*Matcher, error) {
	if err := t.validate(); err != nil {
		return nil, err
	}
	ordered, err := generateTags(t)
	if err != nil {
		return nil, err
	}
	rank := make(map[Tag]int, len(ordered))
	for i, tag := range ordered {
		if _, exists := rank[tag]; !exists {
			rank[tag] = i
		}
	}
	return &Matcher{tags: ordered, rank: rank}, nil
}

func (t Target) validate() error {
	switch t.Implementation {
	case "cp", "pp", "py":
	default:
		return fmt.Errorf("%w: unsupported implementation %q", ErrUnsupportedTarget, t.Implementation)
	}

	if t.PyMajor <= 0 || t.PyMinor < 0 {
		return fmt.Errorf("%w: invalid Python version %d.%d", ErrUnsupportedTarget, t.PyMajor, t.PyMinor)
	}

	if err := t.validateABI(); err != nil {
		return err
	}

	switch t.OS {
	case "linux":
		if !contains(linuxArchs, t.Arch) {
			return fmt.Errorf("%w: unsupported linux arch %q", ErrUnsupportedTarget, t.Arch)
		}
		if t.Libc != "glibc" && t.Libc != "musl" {
			return fmt.Errorf("%w: linux target requires Libc \"glibc\" or \"musl\", got %q", ErrUnsupportedTarget, t.Libc)
		}
		if t.LibcMajor == 0 && t.LibcMinor == 0 {
			return fmt.Errorf("%w: linux target requires a libc version", ErrUnsupportedTarget)
		}
		if t.LibcMajor <= 0 || t.LibcMinor < 0 {
			return fmt.Errorf("%w: invalid libc version %d.%d", ErrUnsupportedTarget, t.LibcMajor, t.LibcMinor)
		}
	case "macos":
		if !contains(macosArchs, t.Arch) {
			return fmt.Errorf("%w: unsupported macOS arch %q", ErrUnsupportedTarget, t.Arch)
		}
		if t.MacMajor < 10 {
			return fmt.Errorf("%w: macOS target requires major version >= 10, got %d", ErrUnsupportedTarget, t.MacMajor)
		}
		// Apple silicon shipped with macOS 11 (Big Sur), so a macOS 10.x
		// arm64 host has never existed. pypa/packaging would happily generate
		// macosx_10_<n>_arm64 for such a request -- its mac_platforms() puts
		// no version floor on the arm64 binary-format list -- because it is
		// only ever asked about the host it is running on. This package is
		// asked about *declared* targets, where the combination can only be a
		// mistake, so it is rejected rather than answered with tags that
		// cannot describe a real machine.
		if t.MacMajor == 10 && t.Arch != "x86_64" {
			return fmt.Errorf("%w: macOS 10.x targets are x86_64 only, got arch %q (arm64 macOS starts at 11)", ErrUnsupportedTarget, t.Arch)
		}
		if t.MacMinor < 0 {
			return fmt.Errorf("%w: invalid macOS minor version %d", ErrUnsupportedTarget, t.MacMinor)
		}
	case "windows":
		if !contains(windowsArchs, t.Arch) {
			return fmt.Errorf("%w: unsupported windows arch %q", ErrUnsupportedTarget, t.Arch)
		}
	default:
		return fmt.Errorf("%w: unsupported OS %q", ErrUnsupportedTarget, t.OS)
	}
	return nil
}

// freeThreadedMinMinor is the first CPython 3.x minor version with a
// free-threaded build, and the floor pypa/packaging's _cpython_abis applies
// before it will put a "t" in the ABI tag at all.
const freeThreadedMinMinor = 13

// validateABI checks the interpreter-specific ABI selectors (FreeThreaded for
// CPython, ImplMajor/ImplMinor for PyPy) against the Implementation they
// belong to. Setting one on the wrong implementation is silently ignorable,
// which is exactly why it is rejected: a caller who sets FreeThreaded on a
// "py" target believes it is getting free-threaded tags.
func (t Target) validateABI() error {
	if t.FreeThreaded {
		if t.Implementation != "cp" {
			return fmt.Errorf("%w: FreeThreaded requires implementation \"cp\", got %q", ErrUnsupportedTarget, t.Implementation)
		}
		// Note this is an exact-major check, NOT the lexicographic
		// versionAtLeast used for version floors elsewhere in this package.
		// That is deliberate, and it is a statement about what has been
		// measured rather than a prediction about future Pythons.
		//
		// "3.13+" is where a free-threaded build exists and where
		// packaging's _cpython_abis will put a "t" in the ABI tag. There is no
		// Python 4, so there is no reference implementation to check a cp40t
		// spelling against -- and the free-threaded ABI is exactly the kind of
		// thing whose spelling is easy to get subtly wrong. Accepting an
		// unknown major would have this package emit tags it has never
		// validated against anything, which is worse than refusing an input
		// nobody can currently produce. Widen it when there is something real
		// to measure.
		if t.PyMajor != 3 || t.PyMinor < freeThreadedMinMinor {
			return fmt.Errorf("%w: free-threaded CPython starts at 3.%d, got %d.%d", ErrUnsupportedTarget, freeThreadedMinMinor, t.PyMajor, t.PyMinor)
		}
	}

	if t.ImplMajor == 0 && t.ImplMinor == 0 {
		return nil
	}
	if t.Implementation != "pp" {
		return fmt.Errorf("%w: ImplMajor/ImplMinor are only meaningful for implementation \"pp\", got %q", ErrUnsupportedTarget, t.Implementation)
	}
	if t.ImplMajor <= 0 || t.ImplMinor < 0 {
		return fmt.Errorf("%w: invalid implementation version %d.%d", ErrUnsupportedTarget, t.ImplMajor, t.ImplMinor)
	}
	return nil
}

var (
	// linuxArchs and macosArchs are validated by Compile ahead of the
	// generation support landing in later tasks (#18632 Task 3/4), so that
	// Compile's error behavior for malformed linux/macOS targets is already
	// correct.
	linuxArchs   = []string{"x86_64", "i686", "aarch64", "armv7l", "ppc64", "ppc64le", "s390x", "riscv64", "loongarch64"}
	macosArchs   = []string{"x86_64", "arm64"}
	windowsArchs = []string{"amd64", "x86", "arm64"}
)

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// Matcher is a compiled, ordered set of compatible tags for a Target, with
// an index for O(1) priority lookup. The zero value is not usable; obtain a
// Matcher via Target.Compile.
type Matcher struct {
	tags []Tag
	rank map[Tag]int
}

// Tags returns a copy of the full ordered list of compatible tags, most
// preferred first. It is an inspection method, not the hot path (Rank and
// IsCompatible are); the copy protects the Matcher's internal slice from
// mutation by the caller.
func (m *Matcher) Tags() []Tag {
	out := make([]Tag, len(m.tags))
	copy(out, m.tags)
	return out
}

// Rank reports the best (lowest, most preferred) priority among the given
// tags that this Matcher supports. w is typically the output of ParseTag for
// one wheel's tag component, since a single component can expand to several
// Tag alternatives. ok is false if none of w is compatible.
func (m *Matcher) Rank(w []Tag) (rank int, ok bool) {
	best := -1
	found := false
	for _, tag := range w {
		if r, exists := m.rank[tag]; exists && (!found || r < best) {
			best = r
			found = true
		}
	}
	return best, found
}

// IsCompatible reports whether any of the given tags is supported by this
// Matcher.
func (m *Matcher) IsCompatible(w []Tag) bool {
	_, ok := m.Rank(w)
	return ok
}
