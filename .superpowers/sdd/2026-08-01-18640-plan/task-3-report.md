# Task 3 Report: Fix `internal/pep508/` — `name()`, trailing line breaks, malformed quoted strings

## Status: DONE

**Commit:** 374af55

## Summary

Fixed three PEP 508 grammar divergences from pypa/packaging 26.2, covering 8 of the 11 measured error-case divergences. All tests pass, including with `-race`. The tokenizer changes affected `marker/` and `reqtxt/` packages as expected, but both passed all tests — no behavior was broken. One test in the `requirement/` package had the wrong expectation and was corrected.

## Changes Made

### 1. Allow empty parenthesised specifier (`name()`)

**Files modified:**
- `internal/pep508/requirement.go`

**Changes:**
- Deleted the error branch (lines 191-193) that rejected empty parentheses
- Updated the doc comment on `parseVersionSpec` to document that `name()` is valid

**Rationale:** According to pypa/packaging's `test_empty_specifier`, an empty parenthesized group is legal and means "no constraint". The function already returns `""` (empty string) when `clauses` is empty, which is the correct behavior.

**Test verification:** Without this fix, all 4 test cases in `TestParseRequirement_EmptyParenthesisedSpecifier` fail with "Expected version specifier inside parentheses".

### 2. Reject trailing line breaks

**Files modified:**
- `internal/pep508/tokenizer.go`
- `internal/pep508/requirement.go`

**Changes in tokenizer.go:**
- Changed the `WS` token pattern from `\A\s+` to `\A[ \t]+`
  - This restricts whitespace matching to horizontal whitespace only (space and tab)
  - Line breaks (`\n`, `\r`) are no longer consumed as whitespace

**Changes in requirement.go:**
- Modified the URL whitespace handling (lines 146-168) to explicitly accept and consume line breaks after a URL
- This preserves the existing behavior where `name @ url\n; marker` is valid (defensively multiline format)
- The final `consume(WS)` before `expect(End)` only consumes horizontal whitespace, so a trailing line break at the end causes the `expect(End)` check to fail with "Expected end of requirement string"

**Rationale:** Upstream rejects trailing line breaks in all 6 variants tested (3 line breaks × 2 bases). By restricting `WS` to horizontal whitespace, a trailing line break stays unconsumed and triggers the existing end-of-input error. The URL case needed special handling because line breaks are valid separators there (common in Requires-Dist metadata).

**Test verification:** Without the tokenizer change, all 6 test cases in `TestParseRequirement_RejectsTrailingLineBreak` fail with "An error is expected but got nil" (the line breaks were accepted).

**Horizontal whitespace still valid:** `TestParseRequirement_AllowsTrailingHorizontalWhitespace` passes, confirming that trailing spaces and tabs remain valid.

### 3. Reject malformed quoted strings

**Files modified:**
- `internal/pep508/tokenizer.go`

**Changes:**
- Changed the `QuotedString` token pattern from `\A(?:'[^']*'|"[^"]*")` to `\A(?:'[^'\\]*'|"[^"\\]*")`
- This rejects any string containing a backslash

**Rationale:** PEP 508 marker strings have **no escape sequences**. A backslash before the closing quote leaves the string unterminated. The simplest correct fix is to reject backslashes entirely, which matches upstream behavior for both `"C:\"` and `"\x"`.

**Test verification:** Without this fix, both test cases in `TestParseRequirement_RejectsMalformedQuotedString` fail with "An error is expected but got nil" (the malformed strings were accepted).

## Impact on Other Packages

### `marker/` package
- All tests passed (0.652s without race, 1.998s with race)
- No changes needed
- The tokenizer is shared, but the whitespace and quoted string changes are correct for markers too

### `reqtxt/` package
- All tests passed (0.603s without race, 1.677s with race)
- No changes needed
- This package handles its own line splitting before tokenizing, so the WS change doesn't affect it

### `requirement/` package
- One test updated: `TestParse_VersionSpec_EmptyParens_IsError` → `TestParse_VersionSpec_EmptyParens_IsValid`
- The old test expected `foo()` to be an error, but according to upstream it should be valid
- This is a correction of a wrong expectation, not a behavior regression

## Commands Run

### Test execution
```bash
# Initial test run to verify failures
cd /Users/jonyoder/Dev/gpp-worktrees/conformance-18640
go test ./internal/pep508/ -run 'TestParseRequirement_' -v
# Result: 4 failures in EmptyParenthesisedSpecifier, 6 in RejectsTrailingLineBreak, 
#         2 in RejectsMalformedQuotedString (as expected)

# After fixes, run pep508 tests
go test ./internal/pep508/ -run 'TestParseRequirement_' -v
# Result: PASS, all 4 new test functions pass

# Full module test
go test ./...
# Result: PASS all packages except requirement (test expectation bug)

# After fixing requirement test
go test ./...
# Result: PASS all 14 packages

# Race detector
go test -race ./...
# Result: PASS all 14 packages
```

### Individual fix verification (stash and test)
```bash
# Fix 1: Empty parenthesised specifier
git stash push internal/pep508/requirement.go requirement/requirement_test.go
go test ./internal/pep508/ -run TestParseRequirement_EmptyParenthesisedSpecifier -v
# Result: FAIL - all 4 cases fail with "Expected version specifier inside parentheses"
git stash pop

# Fix 2: Trailing line breaks
git stash push internal/pep508/tokenizer.go
go test ./internal/pep508/ -run TestParseRequirement_RejectsTrailingLineBreak -v
# Result: FAIL - all 6 cases fail with "An error is expected but got nil"

# Fix 3: Malformed quoted strings
go test ./internal/pep508/ -run TestParseRequirement_RejectsMalformedQuotedString -v
# Result: FAIL - both cases fail with "An error is expected but got nil"
git stash pop
```

### Formatting and linting
```bash
gofmt -l .
# Result: (empty output - all files correctly formatted)

golangci-lint config verify
golangci-lint run ./... --timeout 10m
# Result: 0 issues
```

## Test Results Summary

**New tests added:** 4 test functions in `internal/pep508/grammar_conformance_test.go`
- `TestParseRequirement_EmptyParenthesisedSpecifier` (4 subtests)
- `TestParseRequirement_RejectsTrailingLineBreak` (6 subtests)
- `TestParseRequirement_AllowsTrailingHorizontalWhitespace` (3 subtests)
- `TestParseRequirement_RejectsMalformedQuotedString` (2 subtests)

**Tests modified:** 1 in `requirement/requirement_test.go`
- `TestParse_VersionSpec_EmptyParens_IsError` → `TestParse_VersionSpec_EmptyParens_IsValid`

**All module tests:** 14 packages, all PASS
**Race detector:** All PASS
**Lint:** 0 issues

## Notes

1. **URL whitespace handling:** The most complex part of this task was handling the `name @ url\n; marker` case. A newline after a URL is valid as a separator (defensively multiline format), but a trailing newline at the end of the requirement should be rejected. The fix adds explicit line-break consumption after a URL while keeping the final `consume(WS)` horizontal-only.

2. **No tokenizer breakage:** The tokenizer is shared with `marker/` and `reqtxt/`, but both packages passed all tests. The whitespace and quoted-string changes are correct for all consumers.

3. **Upstream citation:** The test file includes the upstream commit pinned at `4eb0753dba8fcaaac8eb75463374e448f0931558` (pypa/packaging).

4. **No provenance blocks added:** As instructed, I did not add per-file provenance blocks (separate issues #19394, #19395).

## Fix round 1

### Status: DONE

**What was wrong:** The brief's premise that "PEP 508 marker strings have no escape sequences" was incorrect. Upstream `pypa/packaging` at the pinned SHA (`4eb0753dba8fcaaac8eb75463374e448f0931558`) does NOT reject all backslashes. The tokenizer pattern is `('[^']*')|("[^"]*")` (no backslash exclusion), and the parser calls `ast.literal_eval(token.text)` to decode Python string-literal escapes. Only *invalid* escapes are rejected.

The prior implementation's `QuotedString` pattern `\A(?:'[^'\\]*'|"[^"\\]*")` rejected all backslashes, which was over-strict.

### Changes Made

**Files modified:**
- `internal/pep508/tokenizer.go`
- `internal/pep508/marker.go`
- `internal/pep508/grammar_conformance_test.go`

### 1. Reverted tokenizer pattern to upstream's form

**File:** `internal/pep508/tokenizer.go:191`

Changed `QuotedString` pattern from:
```go
QuotedString: regexp.MustCompile(`\A(?:'[^'\\]*'|"[^"\\]*")`),
```

To upstream's exact form:
```go
QuotedString: regexp.MustCompile(`\A(?:'[^']*'|"[^"]*")`),
```

This allows backslashes to tokenize normally, matching upstream behavior.

### 2. Fixed wrong comment about escape handling

**File:** `internal/pep508/tokenizer.go:140-146`

The comment previously claimed packaging "takes `token.text[1:-1]` directly" — it does not. Updated to describe what upstream actually does (`ast.literal_eval`), and note that we deliberately do not implement full escape decoding (out of scope), but validate only the two specific malformed cases upstream asserts.

### 3. Added targeted validation logic in marker.go

**File:** `internal/pep508/marker.go:315-325, 352-394`

Added validation where the marker literal is produced (`parseMarkerVar`), mirroring upstream's structure (tokenize permissively, then validate):

- New function `validateQuotedStringContents(s string)` checks for:
  1. **Trailing unpaired backslash** — counts consecutive backslashes at end, rejects if odd
  2. **Truncated `\x` escape** — finds any `\x` not followed by two hex digits
  
- Returns `&SyntaxError{Msg: "Invalid quoted string"}` — matching upstream's wording

- New helper `isHexDigit(c byte)` checks if a character is 0-9, a-f, or A-F

### 4. Updated test to accept valid escapes

**File:** `internal/pep508/grammar_conformance_test.go:55-82`

- Split `TestParseRequirement_RejectsMalformedQuotedString` into two sections:
  - Still **REJECTS** `"C:\"` (trailing unpaired backslash) and `"\x"` (truncated escape)
  - Newly **ACCEPTS** `"C:\\U"` (doubled backslash, paired) and `"\n"` (newline escape)
  
- Added comments noting we accept the token but do not decode the escape

### Verification

All commands run from `/Users/jonyoder/Dev/gpp-worktrees/conformance-18640`:

**1. Test the new logic:**
```bash
go test ./internal/pep508/ -v -run TestParseRequirement_RejectsMalformedQuotedString
# Result: PASS (4 subtests: 2 rejects, 2 accepts)
```

**2. All conformance tests:**
```bash
go test ./internal/pep508/ -v -run TestParseRequirement_
# Result: PASS (all 4 test functions, 33 total subtests)
```

**3. Full module test:**
```bash
go test ./...
# Result: PASS all 14 packages
```

**4. Race detector:**
```bash
go test -race ./...
# Result: PASS all 14 packages
```

**5. Verify rejections fail without validation:**
```bash
git stash push internal/pep508/marker.go
go test ./internal/pep508/ -run "TestParseRequirement_RejectsMalformedQuotedString/reject" -v
# Result: FAIL - both rejection cases fail with "An error is expected but got nil"
git stash pop
```

This proves the validation logic is necessary.

**6. Formatting and linting:**
```bash
gofmt -l .
# Result: (empty - all files correctly formatted after gofmt -w .)

golangci-lint config verify
golangci-lint run ./... --timeout 10m
# Result: 0 issues
```

### Summary

The fix correctly implements upstream's behavior:
- **Accepts** valid Python string escapes (`"C:\\U"`, `"\n"`)
- **Rejects** the two specific malformed cases upstream asserts (`"C:\"`, `"\x"`)
- **Does not decode** escapes (out of scope, left for future work)

The tokenizer pattern now matches upstream exactly, and validation logic mirrors upstream's structure (permissive tokenization + targeted validation).
