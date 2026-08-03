# go-python-packaging — Claude Code guide

Go primitives for Python packaging metadata: PEP 440 versions, PEP 508 requirement
specifiers and markers, PEP 685 extras, PEP 425/600/656 compatibility tags,
wheel/METADATA parsing, and `requirements.txt`. Consumed by Posit Package Manager
(PPM) as part of [RFD 0001 — Native Go PyPI Dependency Resolution](https://github.com/rstudio/package-manager/blob/main/docs/rfds/0001-pypi-native-resolver/README.md).

> **Status: scaffolding.** Packages are populated per RFD 0001 Phase 0–1. `version/`
> and `distribution/` are created by the history-preserving migrations (#18629/#18630),
> not by hand.

## Module layout

One module, one package per PEP concern (single import path for downstream convenience):

| Package | Scope |
|---|---|
| `version/` | PEP 440 versions and specifiers *(added by migration #18629)* |
| `requirement/` | PEP 508 dependency specifiers |
| `marker/` | PEP 508 environment markers |
| `extras/` | PEP 685 extra-name normalization |
| `reqtxt/` | pip `requirements.txt` format |
| `distribution/` | METADATA / wheel parsing *(added by migration #18630)* |
| `wheelname/` | wheel filename → structured fields |
| `tags/` | PEP 425 / 600 / 656 compatibility tags |

## Build & test

```bash
go build ./...
go test ./...
go vet ./...
```

Module floor is **Go 1.25** (conservative for a public library; PPM consumes it fine at 1.26).

## Code Style

- Follow standard Go conventions.
- **Formatting:** always run `gofmt` before committing. `gofmt -w .` to format in place, or
  `gofmt -l .` to list unformatted files (must print nothing).
- **License header:** every `.go` file begins with, as its first line:
  ```go
  // SPDX-License-Identifier: Apache-2.0 OR MIT
  ```
  This module is dual-licensed Apache-2.0 OR MIT (see `LICENSE-APACHE`, `LICENSE-MIT`, `NOTICE`).
  When code is ported from another project, preserve its upstream attribution in `NOTICE` at
  the time it lands (e.g. Aqua Security's Apache-2.0 notice when `version/` migrates).

### Verify lint matches CI before claiming "lint clean"

CI runs `golangci-lint` via `golangci/golangci-lint-action@v9` (pinned to golangci-lint
**v2.11.2**). Reproduce it exactly from the repo root:

```bash
golangci-lint config verify   # config must load (see the v2 note below)
golangci-lint run ./...
```

A scoped or stale run is not a substitute — run it from the repo root against `./...`, matching CI.

**golangci-lint v2 config note.** `.golangci.yml` uses the v2 schema. In v2, `gofmt` is a
**formatter**, not a linter: it lives under the top-level `formatters:` block, never under
`linters:`. Listing it under `linters:` fails config load (`gofmt is a formatter`) and turns CI
red before any file is scanned. `linters.default: none` keeps the enabled set explicit. If you add
or change linters, run `golangci-lint config verify` and confirm CI is green.

## Automated Reviews (RoboRev)

This repo is set up for **automatic RoboRev reviews**: a git post-commit hook enqueues a
background AI review of every commit through the local RoboRev daemon. Reviews are not gated on a
GitHub workflow — they run locally as you commit.

- **View findings:** `roborev show HEAD` (current commit), `roborev list` (recent jobs), or
  `roborev tui` (interactive). `roborev wait` blocks until the in-flight review finishes.
- **Before opening a PR:** get the whole branch review-clean with the refine loop — invoke the
  `/roborev-refine` skill, which reviews every commit on the branch, fixes findings, and
  re-reviews until it passes.
- **Fix / respond:** `/roborev-fix` fixes all open failing reviews in one pass; `/roborev-respond`
  comments on a review and closes it. `/roborev-design-review` runs a design-focused pass.
- Treat RoboRev findings the way PPM treats its automated PR review: address Critical/Important
  findings before merge; record or dismiss Minor ones deliberately, not silently.

## Changelog

`CHANGELOG.md` is the only signal a downstream consumer of this **public** module has.
Any change that alters what parses, what a `String()` method renders, or what a
derivation returns needs an entry under `## [Unreleased]` in the same PR — behavior
narrowing especially, because a consumer's previously-valid input starts erroring.

- Sections: `Breaking`, `Added`, `Fixed`, `Notes`. Put narrowing changes under
  `Breaking`, not `Fixed`, even when the old behavior was a bug.
- While the major version is `0`, breaking changes ship in a **minor** bump (`0.1.x`
  → `0.2.0`), never a patch. A consumer pinning `^0.1` would otherwise pick up a
  narrowing change silently.
- Internal refactors, test-only changes, and dependency bumps do not need entries.
- CI does not validate the changelog, so this is a review concern.

## Committing

Commit early and often. After each self-contained change that builds and passes tests, make a
commit rather than batching many changes together — RoboRev reviews per commit, so small,
focused commits get tighter, more useful automated review.
