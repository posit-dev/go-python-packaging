// SPDX-License-Identifier: Apache-2.0 OR MIT
//
// Portions of this file are ported from pypa/packaging
// (https://github.com/pypa/packaging), specifically packaging/utils.py's
// canonicalize_name, used under the Apache License, Version 2.0
// (dual-licensed Apache-2.0 OR BSD-2-Clause; see NOTICE for full license and
// copyright detail). The Final_Sigma rule in finalSigma follows CPython's
// handle_capital_sigma in Objects/unicodeobject.c (Unicode Standard §3.13).
// Changed: upstream's str.lower(), str.replace, and repeated "--" collapse
// (which emulate PEP 503's re.sub(r"[-_.]+", "-", name).lower()) are
// translated into a single loop with an ASCII fast path and a non-allocating
// early return for already-normalized names, since name normalization sits on
// a per-dependency hot path; behavior (lowercase, collapse -/_/. runs to one
// "-") is unchanged. str.lower() is reproduced with Go's unicode tables, so
// results for characters whose case data differs between Go's and the running
// Python's Unicode version can differ. The validate=True option is not ported.

// Package canonname holds the single implementation of Python name
// canonicalization shared by PEP 503 project names (package packagename) and
// PEP 685 extra names (package extras). The two PEPs specify the same
// transformation; keeping one implementation means the public entry points
// cannot drift apart.
package canonname

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	capitalDottedI = 'İ' // U+0130; the only unconditional multi-rune lowercase mapping
	capitalSigma   = 'Σ' // U+03A3; lowercases to ς or σ depending on context
	smallSigma     = 'σ' // U+03C3
	finalSmallSig  = 'ς' // U+03C2
)

// Normalize returns the canonical form of a Python name per PEP 503 (and
// PEP 685, which adopts the same rule): the name is lowercased the way
// Python's str.lower() does it, and any run of "-", "_", or "." collapses to
// a single "-".
//
// Lowercasing covers all of Unicode, not just ASCII, including str.lower()'s
// two special cases: "İ" (U+0130) becomes "i" plus U+0307, and "Σ" becomes
// final "ς" at the end of a word and "σ" elsewhere. Bytes that are not valid
// UTF-8 pass through unchanged.
//
// This mirrors pypa/packaging's canonicalize_name without using a regular
// expression, since it sits on a per-dependency, per-extra hot path during
// dependency resolution. An already-normalized name is returned as-is,
// without allocating.
func Normalize(name string) string {
	// Fast path: most names are already normalized; avoid allocating.
	if isNormalized(name) {
		return name
	}

	// strings.Builder, not a []byte plus string(b): String() does not copy,
	// so the slow path allocates once.
	var b strings.Builder
	b.Grow(len(name) + 1)
	inSep := false
	for i := 0; i < len(name); {
		c := name[i]
		if c < utf8.RuneSelf {
			switch c {
			case '-', '_', '.':
				if !inSep {
					b.WriteByte('-')
					inSep = true
				}
			default:
				inSep = false
				if 'A' <= c && c <= 'Z' {
					c += 'a' - 'A'
				}
				b.WriteByte(c)
			}
			i++
			continue
		}

		// No non-ASCII rune lowercases to a separator, so this always ends a
		// separator run.
		inSep = false
		r, size := utf8.DecodeRuneInString(name[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			b.WriteByte(c) // invalid UTF-8: copy the byte unchanged
		case r == capitalDottedI:
			b.WriteString("i̇")
		case r == capitalSigma:
			if finalSigma(name, i, i+size) {
				b.WriteRune(finalSmallSig)
			} else {
				b.WriteRune(smallSigma)
			}
		default:
			b.WriteRune(unicode.ToLower(r))
		}
		i += size
	}
	return b.String()
}

// isNormalized reports whether Normalize(name) == name: name has no "_" or
// ".", no run of two or more "-", and no rune that lowercasing would change.
func isNormalized(name string) bool {
	prevSep := false
	for i := 0; i < len(name); {
		c := name[i]
		if c < utf8.RuneSelf {
			switch {
			case c == '_' || c == '.', 'A' <= c && c <= 'Z':
				return false
			case c == '-':
				if prevSep {
					return false
				}
				prevSep = true
			default:
				prevSep = false
			}
			i++
			continue
		}

		prevSep = false
		r, size := utf8.DecodeRuneInString(name[i:])
		valid := r != utf8.RuneError || size != 1
		if valid && (r == capitalDottedI || unicode.ToLower(r) != r) {
			return false
		}
		i += size
	}
	return true
}

// finalSigma reports whether the capital sigma at name[start:end] is in the
// Final_Sigma context: preceded by a cased letter (skipping case-ignorable
// runes) and not followed by one (again skipping case-ignorable runes).
//
// The context is evaluated on the original name, before separators are
// replaced, because str.lower() runs first upstream. Invalid UTF-8 bytes,
// which Python strings cannot hold, count as neither cased nor
// case-ignorable.
func finalSigma(name string, start, end int) bool {
	before := false
	for j := start; j > 0; {
		r, size := utf8.DecodeLastRuneInString(name[:j])
		if size == 1 && r == utf8.RuneError {
			break
		}
		if !isCaseIgnorable(r) {
			before = isCased(r)
			break
		}
		j -= size
	}
	if !before {
		return false
	}
	for j := end; j < len(name); {
		r, size := utf8.DecodeRuneInString(name[j:])
		if size == 1 && r == utf8.RuneError {
			return true
		}
		if !isCaseIgnorable(r) {
			return !isCased(r)
		}
		j += size
	}
	return true
}

// isCased reports the Unicode Cased property: Lowercase, Uppercase, or Lt.
func isCased(r rune) bool {
	return unicode.IsLower(r) || unicode.IsUpper(r) || unicode.IsTitle(r) ||
		unicode.Is(unicode.Other_Lowercase, r) || unicode.Is(unicode.Other_Uppercase, r)
}

// isCaseIgnorable reports the Unicode Case_Ignorable property: general
// category Mn, Me, Cf, Lm, or Sk, or Word_Break MidLetter, MidNumLet, or
// Single_Quote. Go has no Word_Break tables, so those runes are listed.
func isCaseIgnorable(r rune) bool {
	switch r {
	case '\'', // Single_Quote
		'.', '‘', '’', '․', '﹒', '＇', '．', // MidNumLet
		':', '·', '·', '՟', '״', '‧', '︓', '﹕', '：': // MidLetter
		return true
	}
	return unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf, unicode.Lm, unicode.Sk)
}
