# Changelog

All notable changes to this module are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
While the major version is `0`, breaking changes ship in a **minor** bump rather
than a major one, and are always listed under a `Breaking` heading so they are not
mistaken for a safe patch upgrade.

## [Unreleased]

## [0.2.0] - 2026-08-03

A conformance release. `requirement/`, `version/`, and the PEP 508 grammar are now
tested against tables ported from `pypa/packaging`'s `tests/test_requirements.py`
(upstream SHA `4eb0753`), and the divergences those tables revealed are fixed.

Several of those fixes **narrow what parses**, and one **changes rendered output**.
Read the Breaking section before upgrading — this is why the release is `0.2.0`
rather than `0.1.3`.

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
- A local version segment consisting entirely of ASCII digits normalizes as an integer,
  so `1.0+007` renders as `1.0+7` and `1.0+ubuntu-007` as `1.0+ubuntu.7`. PEP 440 says
  such a segment "should be considered an integer", and scopes its
  no-normalization carve-out to integers inside an *alphanumeric* segment — so
  `1.0+foo0100` is still left exactly as it is. **Ordering is unaffected**: those
  segments were already compared numerically, so `1.0+007` and `1.0+7` compared equal
  before and after. What changes is the rendered string, and therefore arbitrary
  equality: `===1.0+7` now matches `1.0+007`, as upstream does, where it previously
  did not.

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

[Unreleased]: https://github.com/posit-dev/go-python-packaging/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/posit-dev/go-python-packaging/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/posit-dev/go-python-packaging/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/posit-dev/go-python-packaging/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/posit-dev/go-python-packaging/releases/tag/v0.1.0
