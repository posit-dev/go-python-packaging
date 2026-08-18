// SPDX-License-Identifier: Apache-2.0 OR MIT

package license

import "strings"

// LicenseUnknown is the SPDX-side sentinel returned by Types when a license
// cannot be standardized. Distinct (by case) from Names's "UNKNOWN" literal.
const LicenseUnknown = "Unknown"

// TextDumpMaxLen is the length past which a free-form License field is treated
// as a license-text dump rather than a label. Packages routinely paste an
// entire license (or a 60k-character concatenation of several) into the field,
// and such a value is not a name to be classified: Types will not attempt to
// recognize one, and producers that render licenses for a listing blank it.
//
// Exported so the producers and consumers of a derived license value bind to
// one number instead of re-declaring their own and drifting apart.
const TextDumpMaxLen = 200

// Names reproduces PPM getLicense: display names via the priority ladder
// (License-Expression -> classifiers -> freeform License -> "UNKNOWN").
func Names(expression string, classifiers []string, license string) []string {
	// Priority 1: License-Expression (PEP 639 SPDX format)
	if expression != "" {
		lics := ParseSPDXExpression(expression)
		if len(lics) > 0 {
			return lics
		}
	}
	// Priority 2: Classifiers (raw names; no umbrella skip, no dedup)
	lics := GetLicensesFromPyPIClassifiers(classifiers)
	if len(lics) > 0 {
		return lics
	}
	// Priority 3: freeform License field
	if license != "" {
		return []string{license}
	}
	// Priority 4: unknown
	return []string{"UNKNOWN"}
}

// Types reproduces PPM getLicenseTypes: SPDX-id set via
// (License-Expression -> classifiers->SPDX -> freeform License -> "Unknown").
//
// The freeform tier is strict; see typesFromLicenseText. It runs only when the
// two structured tiers yield nothing, so it can only move a package out of
// "Unknown", never overrule a license the package declared properly.
func Types(expression string, classifiers []string, license string) []string {
	// Priority 1: License-Expression is already SPDX format
	if expression != "" {
		lics := ParseSPDXExpression(expression)
		if len(lics) > 0 {
			return lics
		}
	}
	// Priority 2: map classifiers to SPDX (umbrella skip + dedup)
	types := StandardizeLicensePyPIClassifiers(classifiers)
	if !isUnknownTypes(types) {
		return types
	}
	// Priority 3: the legacy freeform License field, when it is unambiguous
	if fromText := typesFromLicenseText(license); len(fromText) > 0 {
		return fromText
	}
	// Priority 4: unknown, as returned by the classifier tier
	return types
}

// isUnknownTypes reports whether a derived type set says nothing but "Unknown".
// A set holding Unknown ALONGSIDE a real id (a package carrying both a mappable
// and an unmappable classifier) is not unknown: the package did declare a
// license through a structured field, and the freeform field must not be
// consulted to second-guess it.
func isUnknownTypes(types []string) bool {
	return len(types) == 1 && types[0] == LicenseUnknown
}

// typesFromLicenseText returns SPDX ids derived from the legacy free-form
// License field, or nil when the text cannot be resolved to KNOWN ids.
//
// Returning nil leaves the caller on "Unknown", which is what the field
// produced before this tier existed, so the tier can only ever move a package
// OUT of Unknown and never onto a wrong id.
//
// ⚠️ The governing rule is that ONE unrecognized token rejects the WHOLE field.
// That is what makes it impossible to invent a license here. ParseSPDXExpression
// does not validate -- it strips operators and hands back whatever tokens are
// left -- so "3-Clause BSD License" tokenizes to `3-Clause` / `BSD` / `License`,
// and accepting the recognizable-looking part of that would attribute BSD-3-Clause
// to a package on the strength of a word. Every token must be a published SPDX
// identifier or the field yields nothing.
//
// The rule also settles the ambiguous spellings without anyone having to
// adjudicate them. Bare `GPL` and `GPLv3` are not SPDX identifiers (the
// identifiers are `GPL-3.0-only`, `GPL-3.0-or-later`, ...), so they are
// rejected rather than resolved to a guessed version; bare `BSD` names a family
// with several incompatible members and is likewise rejected. Aliases in real
// use -- `Apache 2.0`, `MIT License` -- are rejected for the same reason, and
// recognizing them is a separate decision requiring per-entry human review.
//
// The token after a WITH counts. An exception never becomes a type -- the
// identifiers here are licenses -- but `MIT with restrictions` is a real
// free-form value, and reporting plain `MIT` for it would state the opposite of
// what the package said. So an unrecognized exception rejects the field like
// any other unrecognized token, while `Apache-2.0 WITH LLVM-exception` yields
// Apache-2.0.
func typesFromLicenseText(text string) []string {
	t := strings.TrimSpace(text)
	if t == "" || len(t) > TextDumpMaxLen {
		return nil
	}

	// Tier 1: the whole field IS a known SPDX id (case-insensitive).
	if id, ok := lookupSPDXID(t); ok {
		return []string{id}
	}

	// Tier 2: the field is an SPDX expression whose every identifier is known.
	ids, exceptions := parseSPDXExpressionParts(t)
	if len(ids) == 0 {
		return nil
	}
	for _, exc := range exceptions {
		if !isSPDXException(exc) {
			return nil
		}
	}
	out := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, raw := range ids {
		id, ok := lookupSPDXID(raw)
		if !ok {
			return nil // one unknown token rejects the WHOLE field
		}
		// ParseSPDXExpression dedupes by raw token, so "MIT OR mit" survives it
		// as two tokens that fold to one id. Dedupe again after canonicalizing.
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
