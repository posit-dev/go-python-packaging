// SPDX-License-Identifier: Apache-2.0 OR MIT

package packagename

import "github.com/posit-dev/go-python-packaging/internal/canonname"

// Normalize canonicalizes a Python project name per PEP 503: lowercase the
// name, then collapse any run of "-", "_", or "." into a single "-".
//
// Lowercasing follows Python's str.lower() across all of Unicode, so the
// result matches pypa/packaging for non-ASCII input too (up to differences
// between Go's and Python's Unicode versions). Bytes that are not valid UTF-8
// pass through unchanged.
//
// It mirrors pypa/packaging's canonicalize_name (without validate=True) and
// uses no regular expression. It does not check that name is a valid project
// name.
func Normalize(name string) string {
	return canonname.Normalize(name)
}
