// SPDX-License-Identifier: Apache-2.0 OR MIT
//
// Portions of this file port test cases from pypa/packaging
// (https://github.com/pypa/packaging), specifically tests/test_utils.py's
// test_canonicalize_name, used under the Apache License, Version 2.0
// (dual-licensed Apache-2.0 OR BSD-2-Clause; see NOTICE for full license and
// copyright detail).
// Changed: translated from Python/pytest to a Go table-driven test.
//
// These are the same cases extras/conformance_test.go ports. They are repeated
// here under the PEP 503 project-name framing that canonicalize_name was
// written for, so this entry point is pinned to upstream on its own.
//
// Upstream pinned at 4eb0753dba8fcaaac8eb75463374e448f0931558.

package packagename

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Ported from pypa/packaging tests/test_utils.py, test_canonicalize_name.
// Pinned: 4eb0753dba8fcaaac8eb75463374e448f0931558 (L23-L39 at that commit).
// https://github.com/pypa/packaging/blob/4eb0753dba8fcaaac8eb75463374e448f0931558/tests/test_utils.py#L23-L39
func TestConformance_CanonicalizeName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"foo", "foo"},
		{"Foo", "foo"},
		{"fOo", "foo"},
		{"foo.bar", "foo-bar"},
		{"Foo.Bar", "foo-bar"},
		{"Foo.....Bar", "foo-bar"},
		{"foo_bar", "foo-bar"},
		{"foo___bar", "foo-bar"},
		{"foo-bar", "foo-bar"},
		{"foo----bar", "foo-bar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Normalize(tt.name))
		})
	}
}
