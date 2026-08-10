# Changelog

All notable changes to this module are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
While the major version is `0`, breaking changes ship in a **minor** bump rather
than a major one, and are always listed under a `Breaking` heading so they are not
mistaken for a safe patch upgrade.

## [Unreleased]

### Breaking

- **A wildcard may no longer be combined with a pre-release.** `==1.0a1.*`,
  `!=2.0rc1.*` and `==1.0alpha3.*` previously parsed and are now rejected, as PEP 440
  requires and as upstream does.

  `validate()` guarded a wildcard against a dev, post or local version but never against
  a **pre-release**, so a prefix match on a pre-release was silently accepted. It cannot
  mean anything: the wildcard covers the release segment, which is exactly where the
  pre-release marker would have to sit.

  **Measured over the production PyPI snapshot** (25,457 distinct specifier tokens
  extracted from 932,861 packages): **0 newly rejected**. 1,664 wildcard specifiers occur
  in that corpus and not one combines a wildcard with a pre-release. **Measured over real
  CRAN data for the R path** (`NewRSpecifiers`): **0 newly rejected** out of 59,139
  constraint occurrences. Both null results were instrumented — the extraction pattern
  was proved to find the four synthetic pre-release-wildcard shapes it is looking for, and
  the differential harness was proved to report these three specifiers flipping from
  accepted to rejected when they are appended to the corpus.

- `WithPreRelease(true)` no longer suppresses the operator-level pre-release guards.

  It used to set a flag on the `Version` being tested that made
  `Version.IsPreRelease()` report **false for a version that plainly is a pre-release**,
  which disabled the guards inside `specifierLessThan` and `specifierGreaterThan`. The
  observable effect was that `<2` matched `2.0.dev1`. Measured against
  `pypa/packaging` 26.2, `Specifier("<2").contains("2.0.dev1")` is `False` with
  `prereleases=True`, with `prereleases=False`, and with the default — the guard is not
  a pre-release *policy* at all, it is part of what `<` means.

  That flag conflated two different things. PEP 440's pre-release rule governs which
  candidates an installer **offers**; the operator guards govern which versions a
  specifier **matches**. A pre-release does satisfy `>=1.0` — verified against the
  reference implementation — it is merely passed over while a final release is also
  available. The flag has been removed, so `Version.IsPreRelease()` is now a property of
  the version alone and nothing can override it.

  One existing assertion pinned the old behavior (`TestVersion_CheckWithPreRelease`
  asserted `2.0a1` satisfies `<2` under `WithPreRelease(true)`) and has been corrected.

  `WithPreRelease` is now deprecated in favor of `WithPreReleases`. `WithPreRelease(true)`
  maps to `PreReleasesInclude` and `WithPreRelease(false)` to `PreReleasesAuto` — `false`
  never meant "exclude" here, it meant "no override".

- `Specifiers.String()` now renders each specifier normalized instead of echoing the
  input text. `NewSpecifiers("< 2").String()` was `"< 2"` and is now `"<2"`, matching
  upstream's `str(SpecifierSet("< 2"))`.

  `Requirement.String()` is unaffected: `internal/pep508`'s `normalizeSpecifierClause`
  already strips that whitespace before the string reaches `version.NewSpecifiers`, so
  the two levels previously disagreed and now agree. The `requirement/` conformance
  suite passes unchanged.

  ⚠️ Input **order** is still preserved, which upstream does not do — it sorts its
  members, so `SpecifierSet(">=1,<2")` renders as `"<2,>=1"`. Left as a deliberate
  divergence: the OR-of-ANDs shape this type carries for the R path (`a || b`) has no
  upstream counterpart to sort within.

### Added

- `Specifier`, the exported **singular** specifier, with `Operator()`, `Version()`,
  `Original()`, `String()`, `Check()`, `Contains()`, `ContainsVersion()`, `Filter()`,
  `FilterVersions()`, `PreReleases()` and `Equal()`. Built with `NewSpecifier`, or read
  off a set with `Specifiers.List()`. Previously only the *set* was public, exposing
  just `Check` and `String`.

  `Specifier.Version()` returns the operand **as written** rather than normalized
  (`NewSpecifier(">=  v1.0  ").Version()` is `"v1.0"`), which is what upstream does.
  `Equal` canonicalizes instead: `==2.8.0` equals `==2.8`, but `~=1.18.0` does **not**
  equal `~=1.18`, because trailing zeros change which versions `~=` accepts.

- `PreReleases`, a three-state pre-release policy (`PreReleasesAuto`,
  `PreReleasesInclude`, `PreReleasesExclude`) with the `WithPreReleases` option, which
  works both as a `SpecifierOption` at construction and as a `FilterOption` overriding it
  for a single call. `PreReleasesAuto` is the zero value and derives the policy from the
  specifier per PEP 440, which a bool cannot express.

  ⚠️ The set-level and specifier-level derivations are deliberately asymmetric, as
  upstream's are: `Specifier(">=1.0").PreReleases()` is `PreReleasesExclude` while
  `NewSpecifiers(">=1.0").PreReleases()` is `PreReleasesAuto`. That is what lets a set
  fall back to offering a pre-release when nothing else matched.

- `Filter` / `Contains` on both types, implementing PEP 440 candidate **selection**:
  pre-releases are held back while any final release matches, and offered when none
  does. `Contains` is defined as "`Filter` of one item is non-empty", exactly as
  upstream defines it, so the two cannot disagree.

- A **string-operand** query path. `Contains` takes a raw `string`, so a non-PEP 440
  operand is finally expressible: `NewSpecifier("===lolwat").Contains("lolwat")` is
  true. This was previously impossible — `Check` takes a `version.Version` and
  `version.Parse("lolwat")` fails, so there was no way to pass the value in.

  `Contains` (string) and `ContainsVersion` (parsed) differ only for `===`, which
  compares the caller's spelling: `===1.1` contains `Version("1.01")` but not the
  *string* `"1.01"`. That distinction is upstream's and is now testable.

- `FilterBy`, a generic free function providing upstream's `key=` parameter for
  filtering a slice of arbitrary values by a version field. It is a function rather than
  a method because Go methods cannot take type parameters.

- `Specifiers.Len()`, `Specifiers.List()`, `Specifiers.Equal()`, `Specifiers.And()`,
  `Specifiers.ContainsInstalled()` and `NewSpecifiersFrom()`.

  `And` combines the two sets' pre-release policies rather than dropping them, and
  returns `ErrPreReleaseConflict` when they contradict (one `Include`, one `Exclude`),
  where upstream raises `ValueError`. Like upstream it keeps duplicates —
  `len(SpecifierSet(">=1.0") & SpecifierSet(">=1.0"))` is 2 in packaging 26.2 — while
  `Equal` is blind to order and duplicates.

- `marker.Environment.With(map[string]string)`, `Environment.Lookup(name)` and
  `marker.VariableNames()`: the override seam a **partial** marker environment needs.

  Upstream's `Marker.evaluate` takes a partial dict and keeps the live interpreter's
  value for every key the caller left out. The naive Go translation is a struct literal,
  which zero-fills instead — so `Environment{OsName: "foo"}` silently sets
  `python_version` to `""` and **changes the answer** of any marker touching it, with no
  error reported. `With` overrides onto an explicit base, preserving the distinction
  between "set to the empty string" and "never mentioned". An unrecognized variable name
  is an error rather than a silent no-op.

### Fixed

- `NewSpecifiers("")` is now the universal set instead of an error. Upstream treats
  `SpecifierSet('')` as valid, of length zero, and containing every version. The 0.3.0
  fix covered only the zero-value/no-groups arm; this is the empty-input arm that
  survived it.

- The six `MustParse` calls on the matching hot path (`version/specifier.go`) no longer
  panic on an operand that is not a valid version; a comparison that cannot be made
  yields no match.

  ⚠️ Honest scope: these were **not** reachable through the public API before this
  change, and are not now — every operand except `===` is validated at construction, and
  `===` never reaches those functions. But "unreachable" was a property of the *callers*,
  and the exported `Specifier` plus `Filter` are new callers that drive them with
  whatever operand they hold. The regression test builds the shape directly and was
  verified to panic without the fix.

- **Specifier parsing is now case-insensitive, matching version parsing.**
  `specifierRegexp` carried the `(?i)` flag and the `validConstraintRegexp` gate did not,
  so the two disagreed: `==1.0a1` parsed while `==1.0A1` was rejected before the parser
  ever saw it. PEP 440 versions are case-insensitive, and upstream accepts every
  upper-case spelling (`==1.0DEV`, `==1.0ALPHA1`, `>=7.9A1`, `~=1.0.POST1`, …). This was
  the single largest source of conformance failures — 303 of 358.

  This is a pure widening: **874** of the production corpus's 25,457 distinct specifier
  tokens are newly accepted and **0** are newly rejected.

- **A vertical tab is accepted as surrounding whitespace in a specifier.** Go's `\s` is
  `[\t\n\f\r ]` and excludes `\v` (0x0B) where Python's includes it, so `<=  \r \f \v
  v1.0` was rejected. `versionRegex` had already been fixed for this; the two specifier
  patterns had not.

- **Four PEP 440 matching rules corrected**, all found by the ported tables and all now
  agreeing with `pypa/packaging` 26.2:

  - **`<V` and a pre-release of V.** The guard compared *base versions*, so for the spec
    `1.0.post1` it treated `1.0.dev0` as "a pre-release of the specified version" —
    both have base version 1.0. But `1.0.dev0` is a pre-release of `1.0`, not of
    `1.0.post1`. Now compared against the earliest pre-release of the spec
    (upstream's `_earliest_prerelease`), which is where PEP 440 draws the boundary.
  - **`>V` and a post-release of V.** Same base-version error in the other direction: for
    the spec `1.0a1` it excluded `1.0.post0`, which is a post-release of `1.0`, not of
    `1.0a1`. Now uses upstream's `_post_base`.
  - **`>V` and a local version of V.** "A local version of V" means one whose *public*
    part equals V, pre/post/dev included. Comparing base versions made `>1.0a1` wrongly
    reject `1.0a2+local`.
  - **Prefix matching and `~=`.** `versionSplit` did not split the epoch off as its own
    component, so `==0!2.*` could not match `2`; and padding happened between the two
    sides rather than up to the spec's numeric width, so `2` did not match `==2.0.0.*`.
    `~=` derived its prefix by breaking only on `post`/`dev`, so a pre-release operand
    (`~=1.0a1`) produced the wrong prefix. The upstream helpers (`_version_split`,
    `_version_join`, `_is_not_suffix`, `_numeric_prefix_len`, `_left_pad`) are now ported
    directly.

  **Measured over the production corpus:** 12 of 25,457 distinct specifier tokens change
  their match results — all `>`/`<` against a pre-release or post-release operand
  (`>1a2`, `>1rc1`, `<1r`, …). Every one of those 12, plus 5 synthetic controls, was
  checked against `pypa/packaging` 26.2 over a 19-version panel: **0 of 17 agreed with the
  reference before, 17 of 17 agree after.** No rendered form changed.

### Added

- **Conformance tables ported from `pypa/packaging`'s `tests/test_specifiers.py`**
  (`version/specifier_conformance_test.go`, `version/specifierset_conformance_test.go`),
  pinned at `4eb0753dba8fcaaac8eb75463374e448f0931558`: **1,649 assertions**, of which
  **358 failed against the pre-fix library**. Attribution is in `NOTICE`.

  Every expectation was cross-checked against two independent sources — the literal in
  the pinned upstream file *and* what the installed `packaging` 26.2 actually answers — so
  a stale literal cannot be baked in silently. All of them agreed.

  Not ported, with the reason recorded in each file's header: the interval subsystem
  behind `is_subset`/`is_superset`/`is_disjoint`/`is_unsatisfiable`, pickling, and the
  Python-implementation internals (`__match_args__`, `__hash__`, one-element caches,
  construction laziness, `to_range` equivalence).

### Notes

- The 163 rows of the hand-written `TestVersion_Check` table were audited against
  `packaging` 26.2. **No row disagrees with the reference** where both implementations
  parse the input; the only flagged rows are this package's four documented deliberate
  extensions (a bare version, the `=` operator, `*` for "any", and `||` for the R path).

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
