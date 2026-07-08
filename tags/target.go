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
	// Implementation is the interpreter family: "cp" (CPython) or "py"
	// (implementation-agnostic / pure Python).
	Implementation string
	// PyMajor and PyMinor are the target Python version, e.g. 3, 11 for
	// Python 3.11.
	PyMajor int
	PyMinor int

	// OS is one of "linux", "macos", "windows".
	OS string
	// Arch is the target CPU architecture; valid values are OS-dependent.
	Arch string

	// Libc and LibcMajor/LibcMinor are required when OS == "linux".
	// Libc is "glibc" or "musl".
	Libc      string
	LibcMajor int
	LibcMinor int

	// MacMajor and MacMinor are required when OS == "macos". Only macOS 11
	// and later is supported. Generated tags always use "<major>_0" (macOS
	// 11+ yearly releases only bump the major version), so MacMinor does not
	// affect the generated tag set; it is validated (must be >= 0) for
	// forward compatibility and symmetry with PyMinor/LibcMinor.
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
	case "cp", "py":
	default:
		return fmt.Errorf("%w: unsupported implementation %q", ErrUnsupportedTarget, t.Implementation)
	}

	if t.PyMajor <= 0 || t.PyMinor < 0 {
		return fmt.Errorf("%w: invalid Python version %d.%d", ErrUnsupportedTarget, t.PyMajor, t.PyMinor)
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
		if t.MacMajor < 11 {
			return fmt.Errorf("%w: macOS target requires major version >= 11, got %d", ErrUnsupportedTarget, t.MacMajor)
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

// Tags returns the full ordered list of compatible tags, most preferred
// first.
func (m *Matcher) Tags() []Tag {
	return m.tags
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
