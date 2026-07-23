// SPDX-License-Identifier: Apache-2.0 OR MIT

package license

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLicensesFromPyPIClassifiers(t *testing.T) {
	got := GetLicensesFromPyPIClassifiers([]string{
		"License :: OSI Approved :: BSD License",
		"Programming Language :: Python :: 3",
		"License :: OSI Approved",
		"License :: OSI Approved :: BSD License",
	})
	assert.Equal(t, []string{"BSD License", "OSI Approved", "BSD License"}, got)
}

func TestStandardizeLicensePyPIClassifiers(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"bsd", []string{"License :: OSI Approved :: BSD License"}, []string{"BSD-3-Clause"}},
		{"umbrella-skipped", []string{"License :: OSI Approved"}, []string{"Unknown"}},
		{"dedup", []string{"License :: OSI Approved :: BSD License", "License :: OSI Approved :: BSD License"}, []string{"BSD-3-Clause"}},
		{"none", []string{"Programming Language :: Python :: 3"}, []string{"Unknown"}},
		{"not-included", []string{"License :: Public Domain"}, []string{"Unknown"}},
		// LGPLv3 (without "+") denotes exactly v3, so it must map to
		// LGPL-3.0-only, not the more permissive -or-later form.
		{"lgplv3-is-v3-only", []string{"License :: OSI Approved :: GNU Lesser General Public License v3 (LGPLv3)"}, []string{"LGPL-3.0-only"}},
		// Versionless GPL/LGPL classifiers state no version, so we must not
		// fabricate one; they derive to Unknown.
		{"versionless-gpl-is-unknown", []string{"License :: OSI Approved :: GNU General Public License (GPL)"}, []string{"Unknown"}},
		{"versionless-lgpl-is-unknown", []string{"License :: OSI Approved :: GNU Library or Lesser General Public License (LGPL)"}, []string{"Unknown"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, StandardizeLicensePyPIClassifiers(tc.in))
		})
	}
}
