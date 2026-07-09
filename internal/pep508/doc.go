// SPDX-License-Identifier: Apache-2.0 OR MIT

// Package pep508 implements the shared, position-tracking tokenizer used to
// parse PEP 508 environment markers (package marker/) and PEP 508
// dependency-specifier requirement strings (package requirement/).
//
// It is a Go port of pypa/packaging's _tokenizer.py: a Tokenizer holds the
// source string and a byte offset, and exposes a parser-driven API (check,
// expect, read, consume) rather than pre-scanning the whole input into a
// token slice. Callers ask for a specific token kind at the current
// position, which is what lets requirement-only kinds (added in a later
// package) coexist with marker kinds without any global "first rule wins"
// ordering to maintain.
package pep508
