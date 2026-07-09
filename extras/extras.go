// SPDX-License-Identifier: Apache-2.0 OR MIT

package extras

import "strings"

// Normalize canonicalizes a Python extra name per PEP 685: lowercase the
// name, then collapse any run of "-", "_", or "." into a single "-".
//
// This mirrors pypa/packaging's canonicalize_name (PEP 503/685) without
// using a regular expression, since extra-name normalization sits on a
// per-dependency, per-extra hot path during dependency resolution.
func Normalize(name string) string {
	var b strings.Builder
	b.Grow(len(name))

	inSep := false
	for _, r := range name {
		switch r {
		case '-', '_', '.':
			if !inSep {
				b.WriteByte('-')
				inSep = true
			}
		default:
			inSep = false
			if r >= 'A' && r <= 'Z' {
				r += 'a' - 'A'
			}
			b.WriteRune(r)
		}
	}

	return b.String()
}
