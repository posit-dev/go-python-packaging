// SPDX-License-Identifier: Apache-2.0 OR MIT

package license

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed data/pypi_licenses.json
var pypiLicensesJSON []byte

// PyPILicense represents a license from PyPI's Trove classifiers.
// See: https://pypi.org/classifiers/
type PyPILicense struct {
	// Full license classifier. E.g., "License :: OSI Approved :: MIT License"
	Classifier string `json:"Classifier"`
	// License name. The last component of the classifier, e.g., "MIT License"
	Name string `json:"Name"`

	// SPDX identifier mapping. If the license is not in SPDX, SPDXIdentifier should
	// be empty with SPDXNotIncluded set to true. SPDXNotIncluded distinguishes
	// newly added, unreviewed licenses from licenses without an SPDX identifier.
	SPDXIdentifier  string `json:"SPDXIdentifier"`
	SPDXNotIncluded bool   `json:"SPDXNotIncluded"`
}

func init() {
	buildPyPILicenseDB()
}

var pypiLicenseList []*PyPILicense
var pypiLicenseByName map[string]*PyPILicense

func buildPyPILicenseDB() {
	if err := json.Unmarshal(pypiLicensesJSON, &pypiLicenseList); err != nil {
		panic(fmt.Sprintf("license: embedded pypi_licenses.json is malformed: %v", err))
	}

	pypiLicenseByName = make(map[string]*PyPILicense)

	for _, license := range pypiLicenseList {
		if license.SPDXNotIncluded {
			license.SPDXIdentifier = LicenseUnknown
		}
		pypiLicenseByName[license.Name] = license
	}
}

func ListPyPILicenseTypes() []string {
	var types []string
	for _, lic := range pypiLicenseList {
		if lic.SPDXIdentifier != LicenseUnknown {
			types = append(types, lic.SPDXIdentifier)
		}
	}
	return types
}

// StandardizeLicensePyPIClassifiers standardizes PyPI classifiers into SPDX license identifiers.
// LicenseUnknown is returned if a license can't be standardized.
//
// Examples of standardizable PyPI license classifiers:
//
// - License :: OSI Approved :: BSD License
// - License :: OSI Approved :: GNU General Public License v2 or later (GPLv2+)
func StandardizeLicensePyPIClassifiers(classifiers []string) []string {
	var standardized []string
	dedupLicenses := make(map[string]bool)

	licenses := GetLicensesFromPyPIClassifiers(classifiers)
	for _, license := range licenses {
		// "License :: OSI Approved" is a special case. It's an umbrella classifier
		// for valid and standardizable licenses, rather than a license itself, and
		// should not be classified as an unknown license. Just ignore these, as
		// "License :: OSI Approved" alone would be considered a missing license.
		// Also see: https://pypi.org/search/?c=License+%3A%3A+OSI+Approved
		if license == "OSI Approved" {
			continue
		}
		if foundLic, ok := pypiLicenseByName[license]; ok {
			if _, duplicated := dedupLicenses[foundLic.SPDXIdentifier]; duplicated {
				continue
			}
			dedupLicenses[foundLic.SPDXIdentifier] = true
			standardized = append(standardized, foundLic.SPDXIdentifier)
		}
	}
	if len(standardized) == 0 {
		standardized = []string{LicenseUnknown}
	}
	return standardized
}

// GetLicensesFromPyPIClassifiers filters PyPI classifiers for license classifiers
// and returns just the license components of the classifiers.
//
// For example:
//   - "License :: OSI Approved :: BSD License" -> "BSD License"
//   - "License :: Free For Home Use" -> "Free For Home Use"
func GetLicensesFromPyPIClassifiers(classifiers []string) []string {
	var licenses []string
	for _, classifier := range classifiers {
		if strings.HasPrefix(classifier, "License ::") {
			fields := strings.Split(classifier, "::")
			licenses = append(licenses, strings.TrimSpace(fields[len(fields)-1]))
		}
	}
	return licenses
}
