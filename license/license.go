// SPDX-License-Identifier: Apache-2.0 OR MIT

package license

// LicenseUnknown is the SPDX-side sentinel returned by Types when a license
// cannot be standardized. Distinct (by case) from Names's "UNKNOWN" literal.
const LicenseUnknown = "Unknown"

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
// (License-Expression -> classifiers->SPDX -> "Unknown").
func Types(expression string, classifiers []string) []string {
	// Priority 1: License-Expression is already SPDX format
	if expression != "" {
		lics := ParseSPDXExpression(expression)
		if len(lics) > 0 {
			return lics
		}
	}
	// Priority 2: map classifiers to SPDX (umbrella skip + dedup)
	return StandardizeLicensePyPIClassifiers(classifiers)
}
