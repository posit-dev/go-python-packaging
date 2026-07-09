// SPDX-License-Identifier: Apache-2.0 OR MIT

// Package requirement parses PEP 508 dependency-specifier strings: a
// package name, optional bracketed extras, an optional version specifier
// or direct-reference URL, and an optional environment marker clause, e.g.
// `foo[extra]>=1.0,<2.0 ; python_version >= "3.8"`.
//
// Parse drives the shared internal/pep508 tokenizer end-to-end in one pass
// (name, extras, versionspec-or-url, then the marker clause from the
// tokenizer's current position via internal/pep508's ParseMarker), then
// delegates the version specifier to version.NewSpecifiers, normalizes
// extras via extras.Normalize, and wraps the parsed marker expression via
// marker.FromExpr.
package requirement
