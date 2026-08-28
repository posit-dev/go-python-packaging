// SPDX-License-Identifier: Apache-2.0 OR MIT
package reqtxt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKnownOptions_NoFabricatedRequirements is the bug these table entries fix.
// An option missing from knownOptions is assumed boolean, so its argument is
// dispatched as its own line and becomes a requirement. Every one of these is a
// real pip SUPPORTED_OPTIONS entry, so this fired on valid files -- and a
// consumer that resolves the closure of every requirement then hunts for a
// package that does not exist.
func TestKnownOptions_NoFabricatedRequirements(t *testing.T) {
	for _, tc := range []struct {
		content   string
		wantName  string
		wantValue string
	}{
		{"--use-feature 2020-resolver", optUseFeature, "2020-resolver"},
		{"--use-feature=2020-resolver", optUseFeature, "2020-resolver"},
		{"--all-releases :all:", optAllReleases, ":all:"},
		{"--all-releases mypkg,other", optAllReleases, "mypkg,other"},
		{"--only-final :all:", optOnlyFinal, ":all:"},
		{"--only-final=mypkg", optOnlyFinal, "mypkg"},
	} {
		t.Run(tc.content, func(t *testing.T) {
			f, err := Parse(tc.content)
			require.NoError(t, err)

			require.Len(t, f.Entries, 1, "must not also produce a requirement")
			oe, ok := f.Entries[0].(*OptionEntry)
			require.True(t, ok, "want *OptionEntry, got %T", f.Entries[0])
			assert.Equal(t, tc.wantName, oe.Name)
			assert.Equal(t, tc.wantValue, oe.Value)

			assert.Empty(t, f.Requirements(), "no requirement may be fabricated")
		})
	}
}

// TestKnownOptions_CoversPipSupportedOptions guards against the same class of gap
// reappearing. Every long option in pip's SUPPORTED_OPTIONS must be handled
// here, either in knownOptions or by a dedicated dispatch path.
func TestKnownOptions_CoversPipSupportedOptions(t *testing.T) {
	// pip's SUPPORTED_OPTIONS, by long spelling (pip 26.1, req_file.py).
	pipSupported := []string{
		"--index-url", "--extra-index-url", "--no-index",
		"--constraint", "--requirement", "--editable",
		"--find-links", "--no-binary", "--only-binary", "--prefer-binary",
		"--require-hashes", "--pre", "--all-releases", "--only-final",
		"--trusted-host", "--use-feature",
	}
	// These have their own dispatch and never reach knownOptions.
	dispatchedElsewhere := map[string]bool{
		"--constraint": true, "--requirement": true, "--editable": true,
	}

	for _, opt := range pipSupported {
		if dispatchedElsewhere[opt] {
			continue
		}
		_, known := knownOptions[opt]
		assert.True(t, known, "%s is in pip's SUPPORTED_OPTIONS but not knownOptions", opt)
	}
}

// TestStandaloneHash_IsNotAnError pins parity with pip, which logs "line %s has
// --hash but no requirement, and will be ignored" (req_file.py) and carries on.
// Rejecting the file made this package stricter than the thing it models.
func TestStandaloneHash_IsNotAnError(t *testing.T) {
	for _, content := range []string{
		"--hash=sha256:abc",
		"--hash sha256:abc",
		"requests==2.0\n--hash=sha256:abc",
	} {
		t.Run(content, func(t *testing.T) {
			f, err := Parse(content)
			require.NoError(t, err, "pip accepts this file")

			var found bool
			for _, o := range f.Options() {
				if o.Name == "--hash" {
					found = true
				}
			}
			assert.True(t, found,
				"the line is surfaced rather than dropped, so a caller can warn as pip does")
		})
	}
}

// TestStandaloneHash_DoesNotAttachToAPrecedingRequirement guards the thing that
// made this an error in the first place: a file-level --hash must not be mistaken
// for a hash belonging to an earlier requirement.
func TestStandaloneHash_DoesNotAttachToAPrecedingRequirement(t *testing.T) {
	f, err := Parse("requests==2.0\n--hash=sha256:abc")
	require.NoError(t, err)

	reqs := f.Requirements()
	require.Len(t, reqs, 1)
	assert.Empty(t, reqs[0].Hashes, "the standalone --hash belongs to no requirement")
}

// TestFlatten_NilOpenIsAnErrorNotAPanic covers the guard: open is called for the
// root before anything else, so a nil callback used to panic on first deref. In a
// CLI a stack trace is a materially worse failure than an error string.
func TestFlatten_NilOpenIsAnErrorNotAPanic(t *testing.T) {
	require.NotPanics(t, func() {
		_, err := Flatten("root.txt", nil)
		assert.ErrorIs(t, err, ErrInvalidRequirementsFile)
	})
}

// TestTrailingSemicolonStillRejected records a NON-change. A review claimed pip
// accepts "requests;" where this package rejects it. Checked against packaging
// 26.3: Requirement("requests;") raises InvalidRequirement ("Expected a marker
// variable or quoted string"), so the rejection is faithful and stays.
func TestTrailingSemicolonStillRejected(t *testing.T) {
	for _, content := range []string{"requests;", "requests ;"} {
		t.Run(content, func(t *testing.T) {
			_, err := Parse(content)
			assert.ErrorIs(t, err, ErrInvalidRequirementsFile,
				"packaging raises InvalidRequirement here too")
		})
	}
}
