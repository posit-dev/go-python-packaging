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
// Environment holds the 11 PEP 508 marker variables for one concrete
// target; EnvironmentFromTarget builds one from a tags.Target plus the
// InterpreterIdentity fields a Target alone cannot supply (the concrete
// interpreter's implementation name and version). Marker's Evaluate method
// then checks a parsed marker against a single concrete Environment and set
// of active extras, following packaging's _eval_op dispatch: version
// comparison for the four version-typed variables, set membership for
// `extra`, and a generic string-operator fallback otherwise.
package marker
