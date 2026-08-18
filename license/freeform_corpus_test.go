// SPDX-License-Identifier: Apache-2.0 OR MIT

package license

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTypesAgainstRealCorpus pins the derivation against real values of the
// legacy free-form License field taken from PyPI, rather than against strings
// invented to suit the implementation.
//
// ⚠️ This is a REGRESSION FIXTURE, not a specification. Every row whose want is
// not Unknown is a claim about a real package's license. If a change to this
// package moves a row, a human must look at the row and decide whether the new
// answer is right, rather than re-recording the file to make the test pass.
func TestTypesAgainstRealCorpus(t *testing.T) {
	type corpusCase struct {
		License  string   `json:"license"`
		Versions int      `json:"versions"`
		Want     []string `json:"want"`
	}
	var fixture struct {
		Source   string       `json:"source"`
		Captured string       `json:"captured"`
		Coverage string       `json:"coverage"`
		Cases    []corpusCase `json:"cases"`
	}

	raw, err := os.ReadFile("testdata/freeform_corpus.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &fixture))
	require.NotEmpty(t, fixture.Cases)

	var resolved, versionsResolved int
	for _, tc := range fixture.Cases {
		// The corpus selects versions whose ONLY license signal is the
		// free-form field, so both structured tiers are empty by construction.
		got := Types("", nil, tc.License)
		assert.Equal(t, tc.Want, got, "license %q (%d versions)", tc.License, tc.Versions)
		if len(got) != 1 || got[0] != LicenseUnknown {
			resolved++
			versionsResolved += tc.Versions
		}
	}

	t.Logf("corpus %s: %d/%d rows resolved, covering %d versions",
		fixture.Captured, resolved, len(fixture.Cases), versionsResolved)

	// A floor, not an exact figure: a later SPDX list can only resolve more.
	// This catches a change that silently guts the derivation without having to
	// re-record every row.
	assert.Greater(t, versionsResolved, 800000,
		"the derivation should still resolve the bulk of the corpus")
}

// TestCorpusFixtureRejectsTheKnownAmbiguousSpellings guards the specific
// strings the strict gate exists to refuse, straight from the fixture, so a
// future relaxation cannot quietly start resolving them.
func TestCorpusFixtureRejectsTheKnownAmbiguousSpellings(t *testing.T) {
	for _, text := range []string{
		"UNKNOWN",
		"Apache 2.0",
		"Apache License 2.0",
		"BSD",
		"MIT License",
		"GPL",
		"GPLv3",
	} {
		t.Run(text, func(t *testing.T) {
			assert.Equal(t, []string{LicenseUnknown}, Types("", nil, text))
		})
	}
}
