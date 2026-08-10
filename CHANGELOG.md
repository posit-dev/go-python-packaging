# Changelog

All notable changes to this module are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
While the major version is `0`, breaking changes ship in a **minor** bump rather
than a major one, and are always listed under a `Breaking` heading so they are not
mistaken for a safe patch upgrade.

## [Unreleased]

### Fixed

- `tags`: pre-3.8 CPython targets got the wrong exact ABI, so they matched no
  real extension wheel. The ABI carries abiflags — pymalloc `m` below Python 3.8,
  UCS-4 `u` below 3.3 — which were not emitted, so a CPython 3.7 target claimed
  `cp37-cp37-<platform>` where the correct tag, and the one **every** real
  CPython 3.7 extension wheel on PyPI carries, is `cp37-cp37m-<platform>`.

  This is the reachable, real-world case of the same failure the entries below
  describe: a matcher built for a 3.7 client declined every non-`abi3` binary
  wheel and silently fell back to an sdist. Affected: all of 2.x, and 3.0–3.7
  (`cp27mu`, `cp32mu`, `cp35m`, `cp36m`, `cp37m`, …). 3.8 and later were already
  correct. Now pinned by fixtures generated from `packaging` 26.2 for cp27, cp32
  and cp37.

  ⚠️ One part of this is only right for Linux targets, and is documented in the
  package docs rather than silently fixed. `packaging` derives the UCS-4 flag
  from the *running* interpreter rather than the requested version, so it answers
  `u` for any declared 2.x target; UCS-4 was the Unix default while Windows and
  macOS CPython 2.x were UCS-2, which is the `cp27mu` vs `cp27m` split visible on
  PyPI. This release reproduces `packaging`'s answer, so a Windows or macOS 2.x
  target names an ABI no real wheel carries. Doing better means deliberately
  diverging from the reference.

- `tags`: three `(major, minor)` version floors were written as
  `major == floor.major && minor >= floor.minor`, which is a same-major test
  rather than a version comparison — it answers "below the floor" for every input
  whose major differs, *including* inputs above it. All three now go through one
  lexicographic helper.

  **No currently constructible tag or interpreter reaches any of them**, and the
  fix is not claimed to change a real-world answer. glibc's major has been 2
  since 1997, every musl release is 1.x, and there is no Python 4, so each case
  needs a version that does not exist:

  - `BaselinesFor` returned *no* glibc baselines for a lower-major floor such as
    `manylinux_1_0_x86_64`, when every recorded glibc 2.x satisfies `>= 1.0`.
  - A musl target with a major other than 1 got no `musllinux` tags at all.
    Upstream's `_musllinux.platform_tags` uses the major it is given verbatim, so
    a musl 2.3 target now yields `musllinux_2_3` down to `musllinux_2_0`.
  - A Python 4.0 target got no `abi3` tier, where upstream's `_abi3_applies`
    compares tuples (`>= (3, 2)`) and does emit `cp40-abi3-<platform>`.

  They are fixed because a floor that is not a comparison is wrong on its face,
  and this is the kind of latent defect that becomes reachable the moment a tag
  is built from parsed input rather than a fixed list. Both hypothetical cases
  are pinned by golden fixtures generated from `pypa/packaging` 26.2 rather than
  hand-written, so the expected output is measured even though no such platform
  exists yet.

- `tags`: a glibc target now claims the glibc major versions *below* its own, as
  `pypa/packaging` does. glibc guarantees compatibility across major versions
  ("we can assume compatibility across glibc major versions",
  [sourceware #24636](https://sourceware.org/bugzilla/show_bug.cgi?id=24636)), so
  a glibc 3.5 target yields `manylinux_3_5`…`manylinux_3_0` and then
  `manylinux_2_50`…`manylinux_2_5`, legacy aliases included. Previously it
  yielded no `manylinux` tags at all.

  **No real target changes.** glibc's major has been 2 since 1997, and the
  older-major walk cannot fire for a major-2 target — every glibc 2.x golden
  fixture is byte-identical. The behavior is pinned in both directions: unchanged
  for 2.x, and recorded for 3.5 on x86_64 and aarch64 by fixtures generated from
  `packaging` 26.2.

  Enumerating an unreleased major requires knowing where its minors stop, which
  upstream takes from `_LAST_GLIBC_MINOR` — a `defaultdict` returning 50 that its
  own comment calls a guess to be replaced "once this actually happens". That
  placeholder is copied here **with attribution and the comment quoted**, rather
  than replaced with a judgement of our own, so upstream's eventual correction is
  inherited instead of diverged from. The reason to prefer upstream's guess over
  our own narrower answer: the consumer of this list is pip's own tag logic, and
  a tag naming a nonexistent glibc is inert — no wheel carries it, so it matches
  nothing — whereas a *missing* tag is a false negative that would have us
  decline a wheel pip on the same host would install.

### Added

- `tags`: PEP 703 free-threaded CPython targets, via the new `Target.FreeThreaded`.
  A free-threaded target takes `cp313-cp313t-<platform>` as its exact ABI, and it
  does **not** accept `abi3` wheels — the GIL-enabled stable ABI is a different
  ABI, not a superset. Its stable-ABI tier is PEP 803's `abi3t` instead, both for
  the target's own version and across the whole descending walk down to `cp32`.
  Measured against `pypa/packaging` 26.2, whose `_abi3_applies` is explicitly
  false for threaded builds. Requires CPython 3.13 or later, matching the floor
  upstream puts on the `t` abiflag; `Compile` rejects it elsewhere, and rejects
  it entirely for non-CPython implementations.

- `tags`: PyPy targets, via `Implementation: "pp"` and the new
  `Target.ImplMajor`/`ImplMinor` (the *implementation's* version, as distinct
  from the Python version: PyPy 7.3 on Python 3.10 is `pypy310_pp73`). These
  route through upstream's `generic_tags` path rather than `cpython_tags`:
  `pp310-pypy310_pp73-<platform>`, then `pp310-none-<platform>`, then the
  compatible tier — whose interpreter-any entry is the **major-only**
  `pp3-none-any`, not `pp310-none-any`. There is no `abi3` tier; `abi3` is a
  CPython concept. Leaving `ImplMajor`/`ImplMinor` unset means "PyPy version
  unknown" and simply omits the implementation-ABI tier.

  Previously `Implementation: "pp"` was rejected by `Compile`, so this only
  widens what is accepted.

- `tags`: macOS 10.x targets, and the pre-11 compatibility tail that macOS 11+
  targets should always have carried. **A macOS 11-or-later target now generates
  additional platform tags it did not before** (`macosx_10_16` down to
  `macosx_10_4`, with the full format list on x86_64 and `universal2` alone on
  arm64), ranked below every macOS 11+ tag. A wheel that was previously judged
  incompatible with such a target may now match; nothing that previously matched
  stops matching, and no ordering among the pre-existing tags changed. A declared
  10.x target instead walks the *minor* version, as macOS's pre-11 numbering
  requires, and emits nothing below `macosx_10_4`, where upstream's
  `_mac_binary_formats` defines no x86_64 format at all.

  macOS 10.x is accepted for `x86_64` only, which is a deliberate divergence from
  `pypa/packaging`: it would answer a 10.x arm64 request with
  `macosx_10_<n>_arm64` tags, because it only ever describes the host it is
  running on. Apple silicon shipped with macOS 11, so as a *declared* target that
  combination can only be a mistake, and `Compile` rejects it.

- `tags`: `Baseline`, `Baselines`, `LookupBaseline` and `BaselinesFor` — a
  compiled-in table of the libc version well-known Linux distribution releases
  ship, layered over the `>=` comparison PEP 600 and PEP 656 actually specify.
  `LookupBaseline` returns `(Baseline, bool)`, and `base.Apply(t)` fills a
  `Target`'s libc fields from a distribution name:

  ```go
  base, ok := tags.LookupBaseline("ubuntu", "22.04")
  m, err := base.Apply(target).Compile()
  ```

  `BaselinesFor("manylinux_2_28_x86_64")` reports which
  recorded releases can run a wheel carrying that tag, legacy `manylinux1`/`2010`/
  `2014` aliases included. The version numbers are derived mechanically from the
  distributions' own package repositories; see `tags/data/gen_distro_baselines.py`
  for the source, the filters, and how to regenerate the table. The table is
  embedded, never fetched — nothing in `tags` reaches the network.

## [0.4.0] - 2026-08-10

### Breaking

- A comma is required between version constraints in a specifier set. `>=1.0<2.0`,
  `>= 7 < 10` and `>= 1.0 != 1.3.4.* < 2.0` previously parsed and are now rejected, as
  PEP 440 requires and as upstream does.

  They did not merely parse. `validConstraintRegexp` made the separator optional, and
  combined with the deliberately-admitted empty operator (a bare version is a valid
  constraint) it **rendered a comma the input never contained**: `==0.1dev10.3` became the
  constraint `==0.1dev10` plus a bare-version constraint `3`, rendering `==0.1dev10,3`. That
  is a fabricated constraint boundary, the same defect class as the tokenizer fix in 0.3.0
  reached by a different route, and it completes that fix.

  **Measured over the production PyPI snapshot** (2,804,136 distinct requirement strings):
  exactly **21 newly rejected, 0 newly accepted**. Every one is a version operand whose
  match is a proper prefix leaving a valid second token, so every one previously rendered a
  comma from nowhere. All sampled forms were confirmed rejected by `pypa/packaging` 26.2, so
  this closes a divergence rather than creating one.

  **Measured over real CRAN data for the R path** (`NewRSpecifiers`, which PPM uses for R
  constraint parsing): **no change at all** — 58,656 constraint occurrences, 2,939 distinct
  expressions, identical accept/reject before and after. R constraints in practice are a
  single operator and version; the corpus contains no comma-less pair. The change was
  verified to actually reject six adjacency shapes and to leave twelve load-bearing forms
  untouched, so that null result reflects the data rather than an inert change.

  **Deliberately unchanged:** a bare version is still a valid constraint (PPM passes one
  through `NewRSpecifiers`; narrowing it is tracked separately in
  rstudio/package-manager#18634), and a trailing comma is still tolerated, which upstream
  also accepts. Narrowing one thing at a time is what keeps the blast radius measurable.

  ⚠️ The Python and R entry points diverge on dash-containing input, by design:
  `newRSpecifiers` rewrites `-` to `.` before validating, so `>=0-12.1` is the single
  constraint `>=0.12.1` there, while on the Python path PEP 440 reads `0-12` as the
  post-release `0.post12` and the trailing `.1` makes it an adjacent pair. There is a test
  pinning both halves.

### Fixed

- Five `TestVersion_Check` cases were written space-separated (`>= 1.0 != 1.3.4.* < 2.0`)
  and asserted as valid, pinning a leniency the reference implementation does not have.
  Measured against `pypa/packaging` 26.2, that form raises `InvalidSpecifier` while the
  comma form parses. Rewritten to the comma-separated form PEP 440 actually uses; the intent
  of each case (the conjunction) is unchanged.

## [0.3.1] - 2026-08-07

This section documents a release that was tagged while its entries still sat under
`[Unreleased]`, so `v0.3.1` shipped undated. The entries are reproduced here unchanged.

### Fixed

- No method on a zero-value `Version` panics. Eight of the thirteen exported methods did:
  `String`, `BaseVersion` and `Public` indexed `release[0]` on a nil slice, and all six
  comparison methods inherited it because `Compare` calls `String` as its equality fast
  path. A zero `Version` now renders as `""` and sorts below every parsed version, with two
  zero values comparing equal. No version `Parse` accepts can render as `""` — the grammar
  requires a release segment — so the empty string cannot collide with a real version.

  A panicking `String` method is worse than most panics because `fmt` **recovers** it:
  `fmt.Errorf("%s", v)` returned an error whose message contained
  `%!s(PANIC=String method: runtime error: index out of range [0] with length 0)` and
  reported no failure, so the defect was silently swallowed at the call sites most likely
  to reach it, while a direct `String()` call crashed the process.
- `real.Compare(Version{})` no longer panics with a nil pointer dereference. A zero
  `Version`'s comparison key holds nil `Part` interfaces, and the underlying
  `Parts.IsAny` ranges over elements calling `IsAny()` on each without a nil check, so the
  panic fired before any element was compared — and only when the zero value was the
  *argument*, which is why `Version{}.Compare(real)` returned an answer while the reverse
  crashed. `Compare` now decides from the release segment before building a key.
- `Compare` pads both release segments to the longer of the two. It previously padded the
  second to its own length, which is a no-op. **No behavior changes for parsed versions** —
  the underlying comparison tolerated the resulting length mismatch and returned the right
  answer, verified across nine pairs with differing segment counts in both directions — so
  this was latent rather than wrong. It became reachable only through a nil release.

## [0.3.0] - 2026-08-07

Finishes the conformance work `0.2.0` began, and corrects the module's attribution.

The three behavior changes below were originally written into `0.2.0`'s section on the
belief that its tag had not yet been cut. It had: `v0.2.0` was published on 2026-08-03
at `3dfc5dc` and a Go module tag cannot be moved once the module proxy has seen it. So
they ship here instead, and `0.2.0`'s section no longer lists them. **If you read that
section before 2026-08-07, it claimed these three for `v0.2.0` and `v0.2.0` does not
contain them.**

Two of the three **narrow what parses** and one **changes rendered output**, which is why
this is `0.3.0` rather than `0.2.1`.

### Breaking

- Two version specifiers written adjacently with no comma are rejected, as PEP 508 requires
  and as upstream does. `foo (>=1.0<2.0)` and `foo>=1.0<2.0` previously parsed.
  **They did not merely parse — they re-rendered with a comma the input never contained**,
  turning `urllib3 (>=1.26<2.0)` into `urllib3>=1.26,<2.0` and so fabricating a constraint
  boundary from malformed input. Measured over 2,804,135 distinct requirement strings in a
  production PyPI snapshot, 756 were affected. Arbitrary equality (`===`) is unchanged and
  still takes an opaque operand, which may contain operator characters.
- `Specifiers.Check` returns `true` for a specifier set holding no specifiers, where it
  previously returned `false` for every version. PEP 508 makes a requirement's version
  specifier optional and an omitted one accepts any version, which upstream spells as an
  empty `SpecifierSet` that contains everything — so "no constraint" means "all versions",
  not "no versions". The package was already inconsistent with itself here: an empty
  *conjunction* was vacuously true while zero *groups* was false.
  **This reverses the answer for a zero-value `Specifiers`**, so a consumer using one as a
  gate moves from rejecting everything to admitting everything. Note that no input string
  can produce such a value — every degenerate input errors — so only a deliberately
  constructed or zero-valued `Specifiers` is affected.
- A local version segment consisting entirely of ASCII digits normalizes as an integer,
  so `1.0+007` renders as `1.0+7` and `1.0+ubuntu-007` as `1.0+ubuntu.7`. PEP 440 says
  such a segment "should be considered an integer", and scopes its
  no-normalization carve-out to integers inside an *alphanumeric* segment — so
  `1.0+foo0100` is still left exactly as it is. **Ordering is unaffected**: those
  segments were already compared numerically, so `1.0+007` and `1.0+7` compared equal
  before and after. What changes is the rendered string, and therefore arbitrary
  equality: `===1.0+7` now matches `1.0+007`, as upstream does, where it previously
  did not.

### Notes

- `NOTICE` now credits the Python `pkginfo` package (MIT, Copyright (c) 2009 Agendaless
  Consulting, Inc. and Contributors) and `pypa/twine` (Apache-2.0), whose material
  `distribution/` ports. Neither was credited through `v0.2.0`. It also records that
  `pypa/packaging` is dual-licensed Apache-2.0 **or** two-clause BSD rather than
  Apache-2.0 only, and deletes a stale claim that no source from the cited projects was
  incorporated — false since `tags/` landed on 2026-07-08, and so false in every released
  version to date. **Redistributors of `v0.2.0` or earlier should take the `NOTICE` from
  this release.** No code changed.

## [0.2.0] - 2026-08-03

A conformance release. `requirement/`, `version/`, and the PEP 508 grammar are now
tested against tables ported from `pypa/packaging`'s `tests/test_requirements.py`
(upstream SHA `4eb0753`), and the divergences those tables revealed are fixed.

Several of those fixes **narrow what parses**, and one **changes rendered output**.
Read the Breaking section before upgrading — this is why the release is `0.2.0`
rather than `0.1.3`.

### Breaking

- `Requirement.String()` renders the marker separator as `"; "` when the requirement
  has no URL. It previously always rendered `" ; "`. The `" ; "` form is kept when a
  URL *is* present, because a bare `;` immediately after a URL is ambiguous — a URL
  may itself contain `;`. Consumers that compare, hash, cache, or persist rendered
  requirement strings will see different bytes for the same input.
- Arbitrary equality (`===`) accepts exactly one token. `=== lolwat` is valid and
  normalizes to `===lolwat`; `===any string here` is now rejected. Previously the
  whole operand was rejected regardless of shape.
- A `.*` prefix match is rejected on a post-release (for example `==1.2.3.post4.*`).
  Validation previously covered only dev and local releases.
- A trailing line break (`\n`, `\r`, or `\r\n`) in a requirement string is rejected.
  A trailing space or tab is still accepted.
- Malformed quoted strings in environment markers are rejected: a trailing unpaired
  backslash (`os_name == "C:\"`) and a truncated `\x` escape (`"\x"`). Valid Python
  escapes still parse, including a doubled backslash immediately before an `x`
  (`"C:\\xyz"`).
- Local version labels now normalize their `-` and `_` separators to `.`, as PEP 440
  requires. `1.0+abc-abc` previously stayed verbatim; it now renders as `1.0+abc.abc`
  and compares equal to it. **This also changes ordering**, and fixes a wrong answer:
  a label was compared as a single alphabetic segment, so `1.0+ubuntu-2` sorted
  *above* `1.0+ubuntu-10`. It now sorts below, matching upstream. Consumers that
  persist or compare rendered versions, or that rely on the previous sort order of
  dash- or underscore-separated local labels, will see a difference.

### Added

- `name()` — an empty parenthesized specifier group — is valid, and means "no version
  constraint".
- Conformance tables ported from `pypa/packaging` for PEP 508 requirements and grammar,
  including an invariant that a specifier string can never yield more specifiers than
  it contains operators.
- A vertical tab is accepted as surrounding whitespace around a version, matching
  upstream. Go's `\s` omits `\v` where Python's includes it, so `"1.0\v"` was
  previously rejected.
- Property-based tests (`pgregory.net/rapid`) adapted from upstream's Hypothesis
  suite. This is a test-only dependency and does not enter the module's non-test
  import graph.

### Fixed

- The PEP 440 operator alternation is built in a deterministic, longest-first order.
  It was previously assembled by ranging a Go map, so the compiled pattern varied from
  process to process. Because Go's `regexp` is leftmost-**first** rather than
  leftmost-longest, an operator that is a prefix of another one could win the match.

### Notes

- The trailing-line-break rule matches upstream's current source, not the released
  `packaging` 26.2. Upstream changed its end-of-input rule from `$` to `\Z` after 26.2;
  Python's `$` matches *before* a trailing newline, so 26.2 still accepts `"name>=1\n"`.
  Comparing this module against a `pip install packaging` will show a difference that
  is upstream's version skew, not a divergence here.
- Full Python string-escape decoding is deliberately not implemented. Upstream gets it
  from `ast.literal_eval`; this module validates only the two malformed forms upstream's
  tests assert, and does not decode escapes.
- The operator-less specifier `"2.0"` is still accepted. Tightening it is behavior-
  narrowing and is tracked in rstudio/package-manager#18634.

## [0.1.2] - 2026-07-31

### Fixed

- `version/`: PEP 440 pre-release spellings are ordered longest-first in the compiled
  pattern, so a specifier like `==1.0alpha1` no longer silently matches nothing
  (rstudio/package-manager#19369).

## [0.1.1] - 2026-07-23

### Fixed

- `license/`: the SPDX expression tokenizer decodes runes rather than bytes, so
  multi-byte whitespace separators such as `NBSP` no longer prevent an expression from
  being split.
- `license/`: `GNU Lesser General Public License v3 (LGPLv3)` maps to `LGPL-3.0-only`
  rather than `LGPL-3.0-or-later` — a separate `LGPLv3+` classifier denotes or-later.
- `license/`: the versionless `GNU General Public License (GPL)` and
  `GNU Library or Lesser General Public License (LGPL)` classifiers no longer fabricate
  a version the metadata does not declare, and derive to `Unknown`.

  All three change license-derivation output (rstudio/package-manager#19135).

## [0.1.0] - 2026-07-23

Initial release. One module, one package per PEP concern:

- `version/` — PEP 440 versions and specifiers (migrated from `rstudio/go-pep440-version`).
- `requirement/` — PEP 508 dependency specifiers.
- `marker/` — PEP 508 environment markers and evaluation.
- `extras/` — PEP 685 extra-name normalization.
- `reqtxt/` — pip `requirements.txt` parsing, flattening, and round-trippable rendering.
- `distribution/` — METADATA and wheel parsing (migrated from
  `rstudio/python-distribution-parser`).
- `wheelname/` — PEP 427 wheel filename to name/version/build/tags.
- `tags/` — PEP 425/600/656 compatibility tags, target-parameterized with no host
  detection.
- `license/` — PyPI classifier standardization, SPDX expression parsing, and top-level
  license derivation.

[Unreleased]: https://github.com/posit-dev/go-python-packaging/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/posit-dev/go-python-packaging/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/posit-dev/go-python-packaging/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/posit-dev/go-python-packaging/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/posit-dev/go-python-packaging/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/posit-dev/go-python-packaging/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/posit-dev/go-python-packaging/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/posit-dev/go-python-packaging/releases/tag/v0.1.0
