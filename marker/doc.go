// SPDX-License-Identifier: Apache-2.0 OR MIT

// Package marker represents PEP 508 environment markers - the optional
// `; <expression>` suffix of a requirement string, e.g.
// `extra == "foo" and python_version >= "3.8"`.
//
// Marker is an opaque wrapper around the raw AST parsed by
// internal/pep508 (a Go port of pypa/packaging's _parser.py marker
// productions): Parse builds one from marker text, and its zero value
// represents "no marker clause" (always true), so callers never need to
// nil-check before working with a Marker.
//
// TODO(#18637): implement Evaluate and the environment it evaluates
// against.
package marker
