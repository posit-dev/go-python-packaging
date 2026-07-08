// SPDX-License-Identifier: Apache-2.0 OR MIT
package tags

import (
	"errors"
	"strings"
)

// ErrInvalidTag is returned by ParseTag for a malformed tag component.
var ErrInvalidTag = errors.New("invalid PEP 425 tag")

// Tag is a single PEP 425 (interpreter, abi, platform) triple.
type Tag struct {
	Interpreter string
	ABI         string
	Platform    string
}

func (t Tag) String() string { return t.Interpreter + "-" + t.ABI + "-" + t.Platform }

// ParseTag parses one PEP 425 tag component, expanding dot-compressed
// alternatives across all three fields as a full Cartesian product.
func ParseTag(s string) ([]Tag, error) {
	parts := strings.Split(s, "-")
	if len(parts) != 3 {
		return nil, ErrInvalidTag
	}
	interps, err := splitField(parts[0])
	if err != nil {
		return nil, err
	}
	abis, err := splitField(parts[1])
	if err != nil {
		return nil, err
	}
	plats, err := splitField(parts[2])
	if err != nil {
		return nil, err
	}
	out := make([]Tag, 0, len(interps)*len(abis)*len(plats))
	for _, i := range interps {
		for _, a := range abis {
			for _, p := range plats {
				out = append(out, Tag{i, a, p})
			}
		}
	}
	return out, nil
}

func splitField(f string) ([]string, error) {
	if f == "" {
		return nil, ErrInvalidTag
	}
	elems := strings.Split(f, ".")
	for _, e := range elems {
		if e == "" {
			return nil, ErrInvalidTag
		}
	}
	return elems, nil
}
