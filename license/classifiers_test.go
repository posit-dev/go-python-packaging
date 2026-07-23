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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, StandardizeLicensePyPIClassifiers(tc.in))
		})
	}
}
