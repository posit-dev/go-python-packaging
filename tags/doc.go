// SPDX-License-Identifier: Apache-2.0 OR MIT

// Package tags computes and matches platform-compatibility tags
// (PEP 425 / 600 / 656).
//
// A Target describes a Python interpreter/platform combination and Compiles
// into a Matcher holding the ordered set of compatible tags for that target,
// plus a priority index for ranking candidate wheels. Generation is entirely
// server-side and parameterized by the given Target — there is no host
// detection. Notable divergences from CPython's packaging library: a bare
// linux_<arch> tag is accepted but ranked last (below all manylinux and
// musllinux tags), and macOS targets are supported for version 11 and later
// only.
package tags
