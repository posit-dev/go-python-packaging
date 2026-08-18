// SPDX-License-Identifier: Apache-2.0 OR MIT

package license

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNamesAndTypes(t *testing.T) {
	cases := []struct {
		name        string
		expression  string
		classifiers []string
		license     string
		wantNames   []string
		wantTypes   []string
	}{
		{
			name: "expression-wins", expression: "MIT OR Apache-2.0",
			classifiers: []string{"License :: OSI Approved :: BSD License"}, license: "ignored",
			wantNames: []string{"MIT", "Apache-2.0"}, wantTypes: []string{"MIT", "Apache-2.0"},
		},
		{
			name: "classifiers-second", classifiers: []string{"License :: OSI Approved :: BSD License"},
			wantNames: []string{"BSD License"}, wantTypes: []string{"BSD-3-Clause"},
		},
		{
			name:        "names-vs-types-classifiers",
			classifiers: []string{"License :: OSI Approved", "License :: OSI Approved :: BSD License", "License :: OSI Approved :: BSD License"},
			wantNames:   []string{"OSI Approved", "BSD License", "BSD License"},
			wantTypes:   []string{"BSD-3-Clause"},
		},
		{
			name: "freeform-third", license: "My Custom License 1.0",
			wantNames: []string{"My Custom License 1.0"}, wantTypes: []string{"Unknown"},
		},
		{
			name:      "none-declared",
			wantNames: []string{"UNKNOWN"}, wantTypes: []string{"Unknown"},
		},
		{
			name: "long-dump-freeform", license: "Copyright (c) 2020 ... 400 chars of license text ... permission is hereby granted",
			wantNames: []string{"Copyright (c) 2020 ... 400 chars of license text ... permission is hereby granted"},
			wantTypes: []string{"Unknown"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantNames, Names(tc.expression, tc.classifiers, tc.license))
			assert.Equal(t, tc.wantTypes, Types(tc.expression, tc.classifiers, tc.license))
		})
	}
}
