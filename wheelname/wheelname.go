// SPDX-License-Identifier: Apache-2.0 OR MIT
package wheelname

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/posit-dev/go-python-packaging/tags"
	"github.com/posit-dev/go-python-packaging/version"
)

var ErrInvalidWheelFilename = errors.New("invalid wheel filename")

type WheelFilename struct {
	Name    string
	Version version.Version
	Build   string
	Tags    []tags.Tag
}

// WheelTags is Parse's result without the version: the fields a PEP 425
// compatibility decision actually needs. It is a distinct type rather than a
// WheelFilename with an empty Version, so a caller cannot read a zero version
// and mistake it for a real one.
type WheelTags struct {
	Name  string
	Build string
	Tags  []tags.Tag
}

func Parse(filename string) (WheelFilename, error) {
	w, verStr, err := parseExceptVersion(filename)
	if err != nil {
		return WheelFilename{}, err
	}
	// Parsed as-is. PEP 427 escaping is NOT losslessly invertible -- both
	// 2.5.0+post1_cpu and 2.5.0_post1+cpu parse from the escaped
	// 2.5.0_post1_cpu, and only the second is right -- so un-escaping here would
	// return a plausible wrong version with no error. Callers that want the tags
	// of such a wheel should use ParseTags.
	ver, err := version.Parse(verStr)
	if err != nil {
		return WheelFilename{}, fmt.Errorf("wheel %q: version: %w", filename, err)
	}
	return WheelFilename{Name: w.Name, Version: ver, Build: w.Build, Tags: w.Tags}, nil
}

// ParseTags is Parse without the version requirement, for callers that want a
// wheel's compatibility tags. The version segment is not parsed and not
// returned; every other field is validated exactly as Parse validates it.
//
// This exists because the version contributes nothing to a PEP 425 tag, yet a
// version that fails PEP 440 made Parse discard tags it had already read
// successfully. PEP 427 escapes a local version's "+" to "_", so a legal
// 1.0+cpu reaches disk as 1.0_cpu and does not parse.
func ParseTags(filename string) (WheelTags, error) {
	w, _, err := parseExceptVersion(filename)
	return w, err
}

// parseExceptVersion splits and validates everything but the version, returning
// the unparsed version string alongside. Both entry points go through it so they
// cannot disagree about what a wheel filename is.
func parseExceptVersion(filename string) (WheelTags, string, error) {
	base, ok := strings.CutSuffix(filename, ".whl")
	if !ok {
		return WheelTags{}, "", fmt.Errorf("%w: missing .whl suffix", ErrInvalidWheelFilename)
	}
	parts := strings.Split(base, "-")
	if len(parts) != 5 && len(parts) != 6 {
		return WheelTags{}, "", fmt.Errorf("%w: expected 5 or 6 dash fields, got %d", ErrInvalidWheelFilename, len(parts))
	}
	name := strings.ToLower(parts[0])
	verStr := parts[1]
	var build string
	tagStart := 2
	if len(parts) == 6 {
		build = parts[2]
		if build == "" || build[0] < '0' || build[0] > '9' {
			return WheelTags{}, "", fmt.Errorf("%w: build tag %q must start with a digit", ErrInvalidWheelFilename, build)
		}
		tagStart = 3
	}
	tagComponent := strings.Join(parts[tagStart:tagStart+3], "-")
	tg, err := tags.ParseTag(tagComponent)
	if err != nil {
		return WheelTags{}, "", fmt.Errorf("wheel %q: tag: %w", filename, err)
	}
	return WheelTags{Name: name, Build: build, Tags: tg}, verStr, nil
}

// CompareBuildTags orders PEP 427 build tags; absent ("") sorts first; present
// tags compare by (leading-int, remaining-string). Non-digit-led input is
// treated as numeric part 0 (total, never panics).
func CompareBuildTags(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return -1
	}
	if b == "" {
		return 1
	}
	an, ar := splitBuild(a)
	bn, br := splitBuild(b)
	switch {
	case an < bn:
		return -1
	case an > bn:
		return 1
	case ar < br:
		return -1
	case ar > br:
		return 1
	default:
		return 0
	}
}

func splitBuild(s string) (int, string) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	n, _ := strconv.Atoi(s[:i]) // "" -> 0
	return n, s[i:]
}
