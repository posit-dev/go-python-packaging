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
// Conformance with pypa/packaging is the design goal, and it is verified rather
// than asserted: the golden fixtures in testdata are generated from packaging
// itself and compared in order, including one produced by an unpatched
// sys_tags() on a real host. Outside the divergences listed below, generated tag
// sets match tag for tag and in order.
//
// Notable divergences from pypa/packaging:
//
//   - A bare linux_<arch> tag is accepted, but ranked last, below every
//     manylinux and musllinux tag. packaging emits it too, in the same last
//     position; the divergence is that this package treats it as usable at all
//     for locally built wheels.
//
//   - A macOS 10.x target is accepted for x86_64 only. packaging would answer a
//     10.x arm64 request with macosx_10_<n>_arm64 tags; Apple silicon shipped
//     with macOS 11, so as a *declared* target that combination can only be a
//     mistake.
//
//   - riscv64 and loongarch64 targets get NARROWER manylinux coverage than
//     packaging, and the gap is large. Two independent causes, both in
//     manylinuxFloor: this package floors those architectures at glibc 2.31 and
//     2.36 respectively (uv's values) where packaging floors every non-x86
//     architecture at 2.17; and it records no legacy alias for them, where
//     packaging's legacy map is keyed by glibc version ALONE and is therefore
//     architecture-independent, so pip does emit manylinux2014_riscv64 and
//     manylinux2014_loongarch64. Fixing only the floor number would still not
//     produce the alias.
//
//     Because a target below its floor yields no manylinux tag at all rather
//     than a shortened list, the effect is not marginal. Measured against
//     packaging 26.2 for a CPython 3.12 target, in order: loongarch64 on glibc
//     2.35 gives packaging 582 tags and this package 42, missing 540;
//     riscv64 on glibc 2.28 gives 393 against 42. glibc 2.35 loongarch64 is a
//     real configuration (Loongnix, Debian), and such a host would be offered no
//     manylinux wheel whatsoever. The current behavior is pinned by
//     TestLinux_NarrowNonX86Floors so that changing the floors is a deliberate
//     test update; those tags are not otherwise covered by the golden fixtures,
//     since a generated fixture would record packaging's wider answer.
//
//   - The exact ABI of a pre-3.3 CPython target always carries the UCS-4 "u"
//     flag, e.g. cp27mu, whatever OS the target names. That reproduces
//     packaging, which derives the flag from the running interpreter rather than
//     the requested version, but UCS-4 was the Unix default while Windows and
//     macOS CPython 2.x were UCS-2 -- the cp27mu vs cp27m split on PyPI. A
//     Windows or macOS 2.x target therefore names an ABI no real wheel carries.
//     See cpythonExactABI.
//
// Baseline and its lookups map well-known Linux distribution releases to the
// libc version they ship, as a convenience for naming a compatibility floor.
// That table is compiled in, never fetched: this library is consumed by servers
// that are frequently air-gapped, so nothing here reaches the network.
package tags
