// SPDX-License-Identifier: Apache-2.0 OR MIT

// Package requirement parses PEP 508 dependency-specifier strings: a
// package name, optional bracketed extras, an optional version specifier
// or direct-reference URL, and an optional environment marker clause, e.g.
// `foo[extra]>=1.0,<2.0 ; python_version >= "3.8"`.
//
// Parse drives the shared internal/pep508 tokenizer end-to-end in one pass
// (name, extras, versionspec-or-url, then the marker clause from the
// tokenizer's current position via internal/pep508's ParseRequirement),
// then delegates the version specifier to version.NewSpecifiers,
// normalizes extras via extras.Normalize, and wraps the parsed marker
// expression via marker.FromExpr. Every parse failure - malformed syntax,
// an invalid version specifier, or an invalid marker expression - is
// wrapped in ErrInvalidRequirement, with the specific cause still
// distinguishable via errors.As/errors.Is.
//
// Requirement's fields (Name, Extras, Specifiers, URL, Marker) hold the
// parsed result, and Requirement.String renders it back to canonical PEP
// 508 syntax such that Parse(r.String()) yields an equivalent Requirement.
package requirement
