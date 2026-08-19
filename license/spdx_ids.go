// SPDX-License-Identifier: Apache-2.0 OR MIT

package license

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed data/spdx_licenses.json
var spdxLicenseIDsJSON []byte

// spdxLicenseIDList is the shape of data/spdx_licenses.json. Only the fields
// read here are declared; the file also carries provenance keys for humans
// (see data/gen_spdx_licenses.py).
type spdxLicenseIDList struct {
	// LicenseListVersion is the SPDX License List release the ids come from,
	// e.g. "3.28.0".
	LicenseListVersion string `json:"licenseListVersion"`
	// LicenseIDs are the non-deprecated SPDX license identifiers, sorted.
	LicenseIDs []string `json:"licenseIds"`
	// LicenseExceptionIDs are the non-deprecated SPDX exception identifiers
	// (the right-hand side of a WITH), sorted.
	LicenseExceptionIDs []string `json:"licenseExceptionIds"`
}

// spdxIDByFold maps a lowercased identifier to its canonical SPDX spelling.
// Folding is what lets `mit` resolve to `MIT`; the generator refuses to emit a
// pair of ids that differ only in case, so the fold is unambiguous.
var spdxIDByFold map[string]string

// spdxExceptionByFold is the same table for exception identifiers. Exceptions
// are recognized, never returned: an exception is not a license type.
var spdxExceptionByFold map[string]struct{}

func init() {
	buildSPDXIDDB()
}

func buildSPDXIDDB() {
	var list spdxLicenseIDList
	if err := json.Unmarshal(spdxLicenseIDsJSON, &list); err != nil {
		panic(fmt.Sprintf("license: embedded spdx_licenses.json is malformed: %v", err))
	}
	if len(list.LicenseIDs) == 0 || len(list.LicenseExceptionIDs) == 0 {
		panic("license: embedded spdx_licenses.json carries no identifiers")
	}

	spdxIDByFold = make(map[string]string, len(list.LicenseIDs))
	for _, id := range list.LicenseIDs {
		// The Unknown sentinel must never be reachable through this table, or a
		// free-form field spelling it would be laundered into a "detected" type
		// indistinguishable from a real derivation. The generator drops such an
		// id too; both guards are cheap and neither is allowed to be the only one.
		if strings.EqualFold(id, LicenseUnknown) {
			continue
		}
		spdxIDByFold[strings.ToLower(id)] = id
	}

	spdxExceptionByFold = make(map[string]struct{}, len(list.LicenseExceptionIDs))
	for _, id := range list.LicenseExceptionIDs {
		spdxExceptionByFold[strings.ToLower(id)] = struct{}{}
	}
}

// lookupSPDXID resolves s to its canonical SPDX license identifier, matching
// case-insensitively. ok is false when s is not a known, non-deprecated SPDX
// identifier.
//
// Only spellings SPDX publishes resolve. Aliases in common use -- "Apache 2.0",
// "MIT License", "BSD" -- deliberately do not: a table of aliases is a table of
// judgment calls about which text means which license, and getting one wrong
// attributes a license a package never granted. Callers must treat a false ok
// as "no answer", never as an invitation to guess.
func lookupSPDXID(s string) (string, bool) {
	id, ok := spdxIDByFold[strings.ToLower(s)]
	return id, ok
}

// isSPDXException reports whether s is a known, non-deprecated SPDX license
// exception identifier, matching case-insensitively.
func isSPDXException(s string) bool {
	_, ok := spdxExceptionByFold[strings.ToLower(s)]
	return ok
}
