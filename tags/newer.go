// SPDX-License-Identifier: Apache-2.0 OR MIT

package tags

import (
	"fmt"
	"strconv"
	"strings"
)

// IsCompatibleOrNewer reports whether any of w is compatible with this Matcher,
// OR names a platform of the same family that this target is simply too old to
// know about: a manylinux/musllinux tag whose glibc/musl floor is above the
// declared libc version, or a macosx tag whose version is above the declared
// macOS version.
//
// The interpreter and ABI axes are NOT relaxed. A candidate is accepted as
// "newer" only if its (Interpreter, ABI) pair is one this Matcher already
// accepts on some platform, so a cp314 wheel is still rejected by a cp313
// target however new its platform tag is.
//
// Rank is undefined for the newer case and IsCompatibleOrNewer deliberately
// returns no rank: the whole point is that these tags fall outside the ordered
// set this Matcher generated, so there is no defensible priority for them.
//
// # When to use this instead of IsCompatible
//
// IsCompatible answers "can the host I described run this wheel", which is what
// an installer wants. This method answers "should a durable declaration of that
// host collect this wheel", which is what a mirror or an offline bundle wants,
// and the two differ over time.
//
// The platform walks run DOWNWARD from the declared version to a floor
// (manylinuxTags, musllinuxTags, macosPlatformTags), so a set compiled once and
// used for months silently stops matching wheels built for platforms released
// since. For an installer that is correct. For a mirror it means quietly
// collecting less and less, and for an air-gapped mirror a wheel not collected
// is a package the client cannot obtain at all. Accepting a too-new platform
// costs bytes; rejecting one costs availability.
func (m *Matcher) IsCompatibleOrNewer(w []Tag) bool {
	if m.IsCompatible(w) {
		return true
	}
	for _, tag := range w {
		if _, ok := m.abis[[2]string{tag.Interpreter, tag.ABI}]; !ok {
			continue
		}
		if m.platformIsNewer(tag.Platform) {
			return true
		}
	}
	return false
}

// platformIsNewer reports whether platformTag names the same platform family as
// this Matcher's target, for the same architecture, at a version ABOVE the
// declared one.
func (m *Matcher) platformIsNewer(platformTag string) bool {
	switch m.target.OS {
	case "linux":
		libc, major, minor, err := parseLibcPlatformTag(platformTag)
		if err != nil {
			return false
		}
		if !strings.HasSuffix(platformTag, "_"+m.target.Arch) {
			return false
		}
		// Legacy aliases (manylinux1/2010/2014) resolve to their fixed glibc
		// version here, so one whose version exceeds the declared floor counts as
		// newer on the same footing as a PEP 600 tag. That is deliberate: the
		// question is "does this wheel require a newer platform than I declared",
		// not "is this platform recent".
		if !m.libcFamilyMatches(libc) {
			return false
		}
		return versionAbove(major, minor, m.target.LibcMajor, m.target.LibcMinor)

	case "macos":
		major, minor, format, err := parseMacosPlatformTag(platformTag)
		if err != nil {
			return false
		}
		if !contains(macosBinaryFormats[m.target.Arch], format) {
			return false
		}
		return versionAbove(major, minor, m.target.MacMajor, m.target.MacMinor)

	default:
		// Windows platform tags carry no version axis, so "newer" has no meaning.
		return false
	}
}

// libcFamilyMatches reports whether libc is a family this Matcher covers. An
// any-libc Matcher (CompileAnyLibc) covers both.
func (m *Matcher) libcFamilyMatches(libc string) bool {
	if m.anyLibc {
		return libc == "glibc" || libc == "musl"
	}
	return libc == m.target.Libc
}

// versionAbove reports whether (major, minor) is strictly greater than
// (floorMajor, floorMinor), compared as a version rather than per-component --
// the same shape as versionAtLeast, negated and made strict.
func versionAbove(major, minor, floorMajor, floorMinor int) bool {
	if major != floorMajor {
		return major > floorMajor
	}
	return minor > floorMinor
}

// parseMacosPlatformTag decomposes "macosx_<major>_<minor>_<format>", the shape
// macosPlatformTags emits. The format component is returned rather than
// validated, since which formats are legal depends on the target's arch.
func parseMacosPlatformTag(platformTag string) (major, minor int, format string, err error) {
	rest, ok := strings.CutPrefix(platformTag, "macosx_")
	if !ok {
		return 0, 0, "", fmt.Errorf("%w: %q is not a macosx platform tag", ErrInvalidTag, platformTag)
	}
	// The format itself can contain underscores ("fat64", "universal2" do not,
	// but splitting from the left keeps this robust to ones that might).
	parts := strings.SplitN(rest, "_", 3)
	if len(parts) != 3 {
		return 0, 0, "", fmt.Errorf("%w: %q has no <major>_<minor>_<format>", ErrInvalidTag, platformTag)
	}
	if major, err = strconv.Atoi(parts[0]); err != nil {
		return 0, 0, "", fmt.Errorf("%w: %q has a non-numeric major", ErrInvalidTag, platformTag)
	}
	if minor, err = strconv.Atoi(parts[1]); err != nil {
		return 0, 0, "", fmt.Errorf("%w: %q has a non-numeric minor", ErrInvalidTag, platformTag)
	}
	if parts[2] == "" {
		return 0, 0, "", fmt.Errorf("%w: %q has an empty format", ErrInvalidTag, platformTag)
	}
	return major, minor, parts[2], nil
}
