// SPDX-License-Identifier: Apache-2.0 OR MIT

// Package tags computes and matches platform-compatibility tags
// (PEP 425 / 600 / 656).
//
// A Target describes a Python interpreter/platform combination and Compiles
// into a Matcher holding the ordered set of compatible tags for that target,
// plus a priority index for ranking candidate wheels. Generation is entirely
// server-side and parameterized by the given Target — there is no host
// detection.
//
// Supported interpreters are CPython ("cp", including PEP 703 free-threaded
// builds via Target.FreeThreaded), PyPy ("pp"), and the
// implementation-agnostic pure-Python "py".
//
// Notable divergences from CPython's packaging library, which is otherwise
// matched tag for tag and in order:
//
//   - A bare linux_<arch> tag is accepted, but ranked last, below every
//     manylinux and musllinux tag.
//   - A macOS 10.x target is accepted for x86_64 only. packaging would answer a
//     10.x arm64 request with macosx_10_<n>_arm64 tags; Apple silicon shipped
//     with macOS 11, so as a *declared* target that combination can only be a
//     mistake.
//
// Baseline and its lookups map well-known Linux distribution releases to the
// libc version they ship, as a convenience for naming a compatibility floor.
// That table is compiled in, never fetched: this library is consumed by servers
// that are frequently air-gapped, so nothing here reaches the network.
package tags
