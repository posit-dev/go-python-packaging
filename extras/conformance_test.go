// SPDX-License-Identifier: Apache-2.0 OR MIT
//
// Portions of this file port test cases from pypa/packaging
// (https://github.com/pypa/packaging), specifically tests/test_utils.py's
// test_canonicalize_name, used under the Apache License, Version 2.0
// (dual-licensed Apache-2.0 OR BSD-2-Clause; see NOTICE for full license and
// copyright detail).
// Changed: translated from Python/pytest to Go table-driven tests; see the
// divergence notes below for cases where the Go expectation intentionally
// differs from upstream's literal assertion.
//
// Of test_utils.py's 11 test functions, only this one maps cleanly onto this
// module: 6 are parse_wheel_filename/parse_sdist_filename (wheelname/'s
// concern, and parse_sdist_filename has no Go counterpart at all), and
// test_canonicalize_version, test_is_normalized_name, and the validating half
// of canonicalize_name(validate=True) have no Go implementation anywhere in
// this module. Only extras.Normalize (canonicalize_name) is ported here.
//
// Upstream pinned at 4eb0753dba8fcaaac8eb75463374e448f0931558 (identical at
// release 26.3: the only test_markers.py/test_utils.py change since is a type
// annotation, not a case).

package extras

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

// Ported from pypa/packaging tests/test_utils.py,
// test_canonicalize_name_invalid - the NON-VALIDATING half only
// (`assert canonicalize_name(name) == expected`). The validating half
// (`canonicalize_name(name, validate=True)` raising InvalidName) has no
// counterpart: Normalize has no validate mode, and PEP 503/685 name-validity
// checking is not implemented anywhere in this module. A half-port, not a
// skip: the assertion this function still makes is real and ported as-is.
// Pinned: 4eb0753dba8fcaaac8eb75463374e448f0931558 (L42-L57 at that commit).
// https://github.com/pypa/packaging/blob/4eb0753dba8fcaaac8eb75463374e448f0931558/tests/test_utils.py#L42-L57
func TestConformance_CanonicalizeNameInvalid_NonValidatingHalf(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"_not_legal", "-not-legal"},
		{"hi\n", "hi\n"},
		{"\nhi", "\nhi"},
		{"h\ni", "h\ni"},
		{"hi\r", "hi\r"},
		{"\rhi", "\rhi"},
		{"h\ri", "h\ri"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Normalize(tt.name))
		})
	}
}
