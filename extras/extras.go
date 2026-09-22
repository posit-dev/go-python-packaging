// SPDX-License-Identifier: Apache-2.0 OR MIT
//
// Portions of this file are ported from pypa/packaging
// (https://github.com/pypa/packaging), specifically packaging/utils.py's
// canonicalize_name, used under the Apache License, Version 2.0
// (dual-licensed Apache-2.0 OR BSD-2-Clause; see NOTICE for full license and
// copyright detail).
// Changed: translated from a single re.sub regex into an explicit
// rune-by-rune loop, since extra-name normalization sits on a
// per-dependency hot path; behavior (lowercase, collapse -/_/. runs to one
// "-") is unchanged.

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
