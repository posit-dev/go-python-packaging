// SPDX-License-Identifier: Apache-2.0 OR MIT

package license

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTypesFromLicenseTextAccepts covers the tier that is allowed to resolve:
// the field is either a single known SPDX identifier or an expression whose
// every identifier is known.
func TestTypesFromLicenseTextAccepts(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []string
	}{
		{name: "exact-id", text: "MIT", want: []string{"MIT"}},
		{name: "lowercase-folds", text: "mit", want: []string{"MIT"}},
		{name: "uppercase-folds", text: "APACHE-2.0", want: []string{"Apache-2.0"}},
		{name: "surrounding-space", text: "  MIT  ", want: []string{"MIT"}},
		{name: "apache", text: "Apache-2.0", want: []string{"Apache-2.0"}},
		{name: "bsd-3-clause-is-unambiguous", text: "BSD-3-Clause", want: []string{"BSD-3-Clause"}},
		{name: "gpl-with-version-is-unambiguous", text: "GPL-3.0-or-later", want: []string{"GPL-3.0-or-later"}},
		{name: "or-expression", text: "MIT OR Apache-2.0", want: []string{"MIT", "Apache-2.0"}},
		{name: "and-expression", text: "MIT AND Apache-2.0", want: []string{"MIT", "Apache-2.0"}},
		{
			// An exception is recognized but never returned: the identifiers
			// this function yields are licenses, not exceptions.
			name: "known-exception-is-recognized-not-returned",
			text: "Apache-2.0 WITH LLVM-exception",
			want: []string{"Apache-2.0"},
		},
		{
			// ParseSPDXExpression dedupes by raw token, so a case-varied repeat
			// survives it as two tokens folding to one id.
			name: "case-varied-duplicate-collapses",
			text: "MIT OR mit",
			want: []string{"MIT"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, typesFromLicenseText(tc.text))
		})
	}
}

// TestTypesFromLicenseTextRejects is the load-bearing half of the design: ONE
// unrecognized token rejects the WHOLE field. Returning nil leaves the caller
// on Unknown, which is exactly what the field produced before this tier
// existed, so the tier can only move a package OUT of Unknown, never onto a
// wrong identifier.
//
// Every string below was measured against the live PyPI corpus on 2026-08-18
// and must stay rejected. The counts are the number of (name, version) pairs
// carrying that exact string.
func TestTypesFromLicenseTextRejects(t *testing.T) {
	cases := []struct {
		name string
		text string
		why  string
	}{
		{name: "empty", text: "", why: "no signal at all"},
		{name: "whitespace-only", text: "   ", why: "no signal at all"},
		{
			name: "bare-bsd", text: "BSD",
			why: "44,585 versions; names a family with several incompatible members",
		},
		{
			name: "bare-gpl", text: "GPL",
			why: "31,081 versions; version and variant both unknown",
		},
		{
			name: "gplv3-is-not-an-spdx-id", text: "GPLv3",
			why: "23,697 versions; the identifiers are GPL-3.0-only / GPL-3.0-or-later",
		},
		{
			name: "gplv2-plus-is-not-an-spdx-id", text: "GPLv2+",
			why: "12,789 versions; not a published identifier",
		},
		{
			name: "license-filename", text: "LICENSE",
			why: "9,444 versions; not a license, a filename",
		},
		{
			name: "license-txt-filename", text: "LICENSE.txt",
			why: "12,434 versions; not a license, a filename",
		},
		{
			name: "unknown-sentinel", text: "UNKNOWN",
			why: "69,824 versions; excluded by design, and must never launder into a detected type",
		},
		{
			name: "unknown-sentinel-folded", text: "unknown",
			why: "the fold must not reach the Unknown sentinel either",
		},
		{
			// The alias tier is deliberately deferred: recognizing these
			// requires per-entry human review, and the strict gate defers them
			// without anyone having to adjudicate.
			name: "alias-apache-space-version", text: "Apache 2.0",
			why: "63,176 versions; deferred alias tier",
		},
		{
			name: "alias-apache-license-version", text: "Apache License 2.0",
			why: "57,107 versions; deferred alias tier",
		},
		{
			name: "alias-mit-license", text: "MIT License",
			why: "40,663 versions; deferred alias tier",
		},
		{
			name: "alias-mit-licence-british", text: "MIT Licence",
			why: "11,551 versions; deferred alias tier",
		},
		{
			// This is the case that shows why a partial match can never be
			// allowed: the recognizable-looking part would attribute
			// BSD-3-Clause to a package on the strength of a word.
			name: "partial-match-must-not-resolve", text: "3-Clause BSD License",
			why: "tokenizes to 3-Clause / BSD / License; accepting the plausible part invents a license",
		},
		{
			// Reporting plain MIT here would state the opposite of what the
			// package said.
			name: "unknown-exception-rejects-the-field", text: "MIT with restrictions",
			why: "an unrecognized exception rejects the field like any other unrecognized token",
		},
		{
			name: "proprietary", text: "Proprietary",
			why: "19,257 versions; not an SPDX identifier",
		},
		{
			name: "one-bad-token-in-an-otherwise-valid-expression", text: "MIT OR NotALicense",
			why: "the whole field is rejected, not just the bad token",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Nil(t, typesFromLicenseText(tc.text), tc.why)
		})
	}
}

// TestTypesFromLicenseTextRejectsTextDump guards the length cutoff, which keeps
// a pasted license body from being tokenized at all.
func TestTypesFromLicenseTextRejectsTextDump(t *testing.T) {
	dump := "MIT"
	for len(dump) <= TextDumpMaxLen {
		dump += " permission is hereby granted, free of charge, to any person"
	}
	assert.Greater(t, len(dump), TextDumpMaxLen)
	assert.Nil(t, typesFromLicenseText(dump))
}

// TestTypesFreeformTierIsLast pins the precedence rule: the free-form field is
// consulted ONLY when both structured tiers yield nothing, so it can never
// overrule a license the package declared properly.
func TestTypesFreeformTierIsLast(t *testing.T) {
	t.Run("expression-beats-freeform", func(t *testing.T) {
		assert.Equal(t, []string{"Apache-2.0"}, Types("Apache-2.0", nil, "MIT"))
	})

	t.Run("classifier-beats-freeform", func(t *testing.T) {
		got := Types("", []string{"License :: OSI Approved :: BSD License"}, "MIT")
		assert.Equal(t, []string{"BSD-3-Clause"}, got)
	})

	// A classifier SPDX has no identifier for still says something. It reaches
	// the free-form tier looking identical to a package that declared nothing,
	// because both collapse to Unknown -- so the tier must be suppressed by the
	// presence of the classifier, not by the value it mapped to.
	t.Run("unmappable-classifier-still-suppresses-freeform", func(t *testing.T) {
		for _, classifier := range []string{
			"License :: Other/Proprietary License",
			"License :: Free for non-commercial use",
			"License :: Free To Use But Restricted",
			"License :: Free For Home Use",
			"License :: Freeware",
			// Open source, but the version is ambiguous, so it does not map.
			// A free-form MIT here contradicts the classifier just as plainly.
			"License :: OSI Approved :: GNU General Public License (GPL)",
		} {
			got := Types("", []string{classifier}, "MIT")
			assert.Equal(t, []string{LicenseUnknown}, got,
				"%q must not be overruled by a free-form MIT", classifier)
		}
	})

	// The bare umbrella names no license, so it is a missing declaration rather
	// than an unknown one, and must not suppress the tier.
	t.Run("bare-osi-umbrella-does-not-suppress-freeform", func(t *testing.T) {
		got := Types("", []string{"License :: OSI Approved"}, "MIT")
		assert.Equal(t, []string{"MIT"}, got)
	})

	t.Run("freeform-runs-when-nothing-structured", func(t *testing.T) {
		assert.Equal(t, []string{"MIT"}, Types("", nil, "MIT"))
	})

	t.Run("freeform-rejection-leaves-unknown", func(t *testing.T) {
		assert.Equal(t, []string{LicenseUnknown}, Types("", nil, "Apache 2.0"))
	})

	t.Run("no-signal-at-all-stays-unknown", func(t *testing.T) {
		assert.Equal(t, []string{LicenseUnknown}, Types("", nil, ""))
	})
}
