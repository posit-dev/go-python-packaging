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

func Parse(filename string) (WheelFilename, error) {
	base, ok := strings.CutSuffix(filename, ".whl")
	if !ok {
		return WheelFilename{}, fmt.Errorf("%w: missing .whl suffix", ErrInvalidWheelFilename)
	}
	parts := strings.Split(base, "-")
	if len(parts) != 5 && len(parts) != 6 {
		return WheelFilename{}, fmt.Errorf("%w: expected 5 or 6 dash fields, got %d", ErrInvalidWheelFilename, len(parts))
	}
	name := strings.ToLower(parts[0])
	verStr := parts[1]
	var build string
	tagStart := 2
	if len(parts) == 6 {
		build = parts[2]
		if build == "" || build[0] < '0' || build[0] > '9' {
			return WheelFilename{}, fmt.Errorf("%w: build tag %q must start with a digit", ErrInvalidWheelFilename, build)
		}
		tagStart = 3
	}
	ver, err := version.Parse(verStr) // as-is; no _->+ un-escape
	if err != nil {
		return WheelFilename{}, fmt.Errorf("wheel %q: version: %w", filename, err)
	}
	tagComponent := strings.Join(parts[tagStart:tagStart+3], "-")
	tg, err := tags.ParseTag(tagComponent)
	if err != nil {
		return WheelFilename{}, fmt.Errorf("wheel %q: tag: %w", filename, err)
	}
	return WheelFilename{Name: name, Version: ver, Build: build, Tags: tg}, nil
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
