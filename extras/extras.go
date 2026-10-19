// SPDX-License-Identifier: Apache-2.0 OR MIT

package extras

import "github.com/posit-dev/go-python-packaging/internal/canonname"

// Normalize canonicalizes a Python extra name per PEP 685: lowercase the
// name, then collapse any run of "-", "_", or "." into a single "-".
//
// Lowercasing follows Python's str.lower() across all of Unicode, so the
// result matches pypa/packaging for non-ASCII input too (up to differences
// between Go's and Python's Unicode versions). Bytes that are not valid UTF-8
// pass through unchanged.
//
// PEP 685 adopts the PEP 503 project-name rule unchanged, so this returns
// exactly what packagename.Normalize returns; both delegate to one shared
// implementation. It mirrors pypa/packaging's canonicalize_name and uses no
// regular expression.
func Normalize(name string) string {
	return canonname.Normalize(name)
}
