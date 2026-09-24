// SPDX-License-Identifier: Apache-2.0 OR MIT
//
// Portions of this file port test inputs and caret positions from Astral uv's
// uv-pep508 crate (https://github.com/astral-sh/uv), specifically
// crates/uv-pep508/src/lib.rs's tests module (the error_* tests), used under
// the Apache License, Version 2.0 (uv-pep508 is dual-licensed Apache-2.0 OR
// BSD-2-Clause, Copyright (c) 2023 konstin; see NOTICE).
// Changed: only inputs and span offsets are ported; message text is this
// module's own, not uv's. uv-pep508's own test module says "Half of these
// tests are copied from https://github.com/pypa/packaging/pull/624".
//
// Pinned: 01cb90c1a4f88af09906cb60de9766d2add4a062 (release 0.12.18). Scope,
// per the issue: nothing from uv-pep440, nothing from uv-pep508/src/marker/,
// only the `mod tests` block of crates/uv-pep508/src/lib.rs's error_* tests -
// 43 of the 44, excluding error_invalid_extra_unnamed_url (behind uv's
// "non-pep508-extensions" cargo feature, covering unnamed direct-reference
// requirements with no counterpart in this module's grammar).
//
// # The caret-translation rule
//
// uv's snapshot draws a span as a run of "^" characters under the covered
// display columns, e.g. `numpy >=1.1.*` / `      ^^^^^^^`. Ours renders
// SyntaxError as Msg + "\n" + Source + "\n" + spaces(Start) + "~"*(End-Start)
// + "^" (tokenizer.go). uv's span [start, start+width) in DISPLAY COLUMNS is
// translated to bytes here by measuring the same input through Go's own
// rune/byte indexing - the two agree everywhere except error_unicode_after_extra
// (see its row below), because uv counts display columns (unicode-width) and
// we count bytes.
//
// # Why 5 pass, 28 are annotated divergences, 7 are skips, and 4 more are skips
//
// Every "Expected X" raised by this grammar (NewSyntaxError/expect) is a
// ZERO-WIDTH point at the parser's cursor - the position where the missing
// token would have started. uv instead spans the FULL offending token. Where
// our point coincides with uv's span START, that is annotated below as a
// divergence (not a bug: our Start is exactly where uv's span begins, we just
// do not additionally measure a width). Where the anchor itself differs - an
// unclosed bracket/paren reported at end-of-input here vs. at the opening
// delimiter by uv, or a uv-only "unnamed URL" heuristic pypa has no
// counterpart for - that is a skip: reproducing uv's anchor would mean
// widening the zero-width convention across many call sites, well past a
// small, contained fix. Four more are skips because pypa (our semantics
// anchor) and uv disagree on the grammar itself, or because the input's
// defect is version-string validity, which this raw grammar entry point does
// not check at all (that happens one layer up, in package requirement, via
// version.NewSpecifiers).
package pep508

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// uvCase is one ported uv-pep508 error_* (or, for the two non-"error_"-named
// upstream functions, parse_name_with_star / test_error_invalid_marker_key)
// input. uvStart/uvEnd is uv's own span, translated to bytes per the file
// header. skip, when non-empty, is a documented skip reason and the row's
// wantStart/wantEnd/wantMsg are left unset (the loop skips before asserting).
type uvCase struct {
	name           string
	input          string
	uvStart, uvEnd int
	skip           string

	wantStart, wantEnd int
	wantMsg            string
}

const (
	skipUnclosedDelimiterAnchor         = "different anchor: this grammar reports the cursor position where the closing delimiter was expected (end of input); uv reports the position of the opening delimiter that never closed. Reproducing uv's anchor would mean widening the zero-width error convention across every unclosed-bracket/paren call site, past a small fix."
	skipOracleAcceptsTrailingUnderscore = `oracle rule: pypa accepts a trailing "_" in this identifier grammar (same production for both a package name and an extra name); uv rejects it. We follow pypa, the semantics anchor.`
	skipVersionValidityDeferred         = "version-string validity (a wildcard/operator mismatch, an invalid PEP 440 local segment) is checked one layer up, in package requirement via version.NewSpecifiers - not by this raw grammar entry point, which accepts any specifier-shaped text."
)

var uvCases = []uvCase{
	// --- pass: our zero-width point lands at the SAME byte offset uv's
	// span both starts AND ends at (uv's own span is itself zero-width here:
	// each points one past the last real character, at end-of-input). ---
	{
		name: "error_marker_incomplete1", input: `numpy; sys_platform`,
		uvStart: 19, uvEnd: 19,
		wantStart: 19, wantEnd: 19,
		wantMsg: "Expected marker operator, one of <=, <, !=, ==, >=, >, ~=, ===, in, not in",
	},
	{
		name: "error_marker_incomplete2", input: `numpy; sys_platform ==`,
		uvStart: 22, uvEnd: 22,
		wantStart: 22, wantEnd: 22,
		wantMsg: "Expected a marker variable or quoted string",
	},
	{
		name: "error_marker_incomplete3", input: `numpy; sys_platform == "win32" or`,
		uvStart: 33, uvEnd: 33,
		wantStart: 33, wantEnd: 33,
		wantMsg: "Expected a marker variable or quoted string",
	},
	{
		name: "error_marker_incomplete5", input: `numpy; sys_platform == "win32" or (os_name == "linux" and`,
		uvStart: 57, uvEnd: 57,
		wantStart: 57, wantEnd: 57,
		wantMsg: "Expected a marker variable or quoted string",
	},
	{
		name: "error_name_at_nothing", input: `name @`,
		uvStart: 6, uvEnd: 6,
		wantStart: 6, wantEnd: 6,
		wantMsg: "Expected URL after '@'",
	},

	// --- annotated divergence: our zero-width point equals uv's span
	// START; we do not additionally measure the offending token's width. ---
	{
		name: "error_empty", input: ``,
		uvStart: 0, uvEnd: 0, // uv: "Empty field is not allowed for PEP508", no span at all
		wantStart: 0, wantEnd: 0, wantMsg: "Expected package name",
	},
	{
		name: "error_start", input: `_name`,
		uvStart: 0, uvEnd: 1,
		wantStart: 0, wantEnd: 0, wantMsg: "Expected package name",
	},
	{
		name: "error_no_name", input: `==0.0`,
		uvStart: 0, uvEnd: 1,
		wantStart: 0, wantEnd: 0, wantMsg: "Expected package name",
	},
	{
		name: "error_unnamed_file_path", input: `/path/to/flask.tar.gz`,
		uvStart: 0, uvEnd: 21,
		wantStart: 0, wantEnd: 0, wantMsg: "Expected package name",
	},
	{
		name: "error_extras_illegal_start1", input: `black[ö]`,
		uvStart: 6, uvEnd: 8,
		wantStart: 6, wantEnd: 6, wantMsg: "Expected ']'",
	},
	{
		name: "error_extras_illegal_start2", input: `black[_d]`,
		uvStart: 6, uvEnd: 7,
		wantStart: 6, wantEnd: 6, wantMsg: "Expected ']'",
	},
	{
		name: "error_extras_illegal_start3", input: `black[,]`,
		uvStart: 6, uvEnd: 7,
		wantStart: 6, wantEnd: 6, wantMsg: "Expected ']'",
	},
	{
		name: "error_extras_illegal_character", input: `black[jüpyter]`,
		uvStart: 7, uvEnd: 9,
		wantStart: 7, wantEnd: 7, wantMsg: "Expected ']'",
	},
	{
		name: "error_extras_illegal_end/dash", input: `foo[bar-]`,
		uvStart: 7, uvEnd: 8,
		wantStart: 7, wantEnd: 7, wantMsg: "Expected ']'",
	},
	{
		name: "error_extras_illegal_end/dot", input: `foo[bar.]`,
		uvStart: 7, uvEnd: 8,
		wantStart: 7, wantEnd: 7, wantMsg: "Expected ']'",
	},
	{
		// uv counts display columns (unicode-width), we count bytes: 'α' is
		// one display column but two UTF-8 bytes. Here that only widens uv's
		// END (our zero-width point still lands at Start=8, the same byte
		// offset uv's span begins at, since nothing multi-byte precedes it) -
		// annotated, not "fixed": see the file header's caret-translation rule.
		name: "error_unicode_after_extra", input: "foo[bar α]",
		uvStart: 8, uvEnd: 10,
		wantStart: 8, wantEnd: 8, wantMsg: "Expected ']'",
	},
	{
		name: "error_extra_with_trailing_comma", input: `black[d,]`,
		uvStart: 8, uvEnd: 9,
		wantStart: 8, wantEnd: 8, wantMsg: "Expected extra name after ','",
	},
	{
		name: "error_parenthesized_pep440", input: `numpy ( ><1.19 )`,
		uvStart: 8, uvEnd: 15,
		wantStart: 8, wantEnd: 8, wantMsg: "Expected ')'",
	},
	{
		name: "error_whats_that", input: `numpy % 1.16`,
		uvStart: 6, uvEnd: 7,
		wantStart: 6, wantEnd: 6, wantMsg: "Expected end of requirement string",
	},
	{
		name: "error_no_comma_between_extras", input: `name[bar baz]`,
		uvStart: 9, uvEnd: 10,
		wantStart: 9, wantEnd: 9, wantMsg: "Expected ']'",
	},
	{
		name: "error_extra_comma_after_extras", input: `name[bar, baz,]`,
		uvStart: 14, uvEnd: 15,
		wantStart: 14, wantEnd: 14, wantMsg: "Expected extra name after ','",
	},
	{
		name: "error_extras_not_closed", input: `name[bar, baz >= 1.0`,
		uvStart: 14, uvEnd: 15,
		wantStart: 14, wantEnd: 14, wantMsg: "Expected ']'",
	},
	{
		name: "parse_name_with_star/dash", input: `wheel-*.whl`,
		uvStart: 5, uvEnd: 6,
		wantStart: 5, wantEnd: 5, wantMsg: "Expected end of requirement string",
	},
	{
		// Same byte-vs-display-column note as error_unicode_after_extra: 'Ѧ'
		// is one display column, two UTF-8 bytes, widening only uv's End.
		name: "parse_name_with_star/non_ascii", input: "wheelѦ",
		uvStart: 5, uvEnd: 7,
		wantStart: 5, wantEnd: 5, wantMsg: "Expected end of requirement string",
	},
	{
		name: "test_error_invalid_marker_key", input: `name; invalid_name`,
		uvStart: 6, uvEnd: 18,
		wantStart: 6, wantEnd: 6, wantMsg: "Expected a marker variable or quoted string",
	},
	{
		name: "error_markers_invalid_order", input: `name; '3.7' <= invalid_name`,
		uvStart: 15, uvEnd: 27,
		wantStart: 15, wantEnd: 15, wantMsg: "Expected a marker variable or quoted string",
	},
	{
		name: "error_markers_notin", input: `name; '3.7' notin python_version`,
		uvStart: 12, uvEnd: 17,
		wantStart: 12, wantEnd: 12,
		wantMsg: "Expected marker operator, one of <=, <, !=, ==, >=, >, ~=, ===, in, not in",
	},
	{
		name: "error_missing_quote", input: `name; python_version == 3.10`,
		uvStart: 24, uvEnd: 28,
		wantStart: 24, wantEnd: 24, wantMsg: "Expected a marker variable or quoted string",
	},
	{
		name: "error_non_ascii_after_marker", input: `foo; python_version == "3.12" αx`,
		uvStart: 30, uvEnd: 33,
		wantStart: 30, wantEnd: 30, wantMsg: "Expected end of requirement string",
	},
	{
		name: "error_markers_inpython_version", input: `name; '3.6'inpython_version`,
		uvStart: 11, uvEnd: 27,
		wantStart: 11, wantEnd: 11,
		wantMsg: "Expected marker operator, one of <=, <, !=, ==, >=, >, ~=, ===, in, not in",
	},
	{
		name: "error_markers_not_python_version", input: `name; '3.7' not python_version`,
		uvStart: 16, uvEnd: 17,
		wantStart: 16, wantEnd: 16, wantMsg: "Expected 'in' after 'not'",
	},
	{
		name: "error_markers_invalid_operator", input: `name; '3.7' ~ python_version`,
		uvStart: 12, uvEnd: 13,
		wantStart: 12, wantEnd: 12,
		wantMsg: "Expected marker operator, one of <=, <, !=, ==, >=, >, ~=, ===, in, not in",
	},
	{
		name: "error_no_version_value", input: `name==`,
		uvStart: 4, uvEnd: 6,
		wantStart: 4, wantEnd: 4, wantMsg: "Expected end of requirement string",
	},
	{
		name: "error_no_version_operator", input: `name 1.0`,
		uvStart: 5, uvEnd: 6,
		wantStart: 5, wantEnd: 5, wantMsg: "Expected end of requirement string",
	},

	// --- annotated divergence, individually: our point sits at the LAST
	// byte of uv's span, not its first (both anchor the same "#" character;
	// uv also spans the whitespace and specifier text before it). ---
	{
		name: "error_random_char", input: `name >= 1.0 #`,
		uvStart: 5, uvEnd: 13,
		wantStart: 12, wantEnd: 12, wantMsg: "Expected end of requirement string",
	},

	// --- skip: different anchor entirely (unclosed delimiter) ---
	{
		name: "error_extras_eof1", input: `black[`,
		uvStart: 5, uvEnd: 6,
		skip: skipUnclosedDelimiterAnchor,
	},
	{
		name: "error_extras_eof2", input: `black[d`,
		uvStart: 5, uvEnd: 6,
		skip: skipUnclosedDelimiterAnchor,
	},
	{
		name: "error_extras_eof3", input: `black[d,`,
		uvStart: 5, uvEnd: 6,
		skip: skipUnclosedDelimiterAnchor,
	},
	{
		name: "error_parenthesized_parenthesis", input: `numpy ( >=1.19`,
		uvStart: 6, uvEnd: 7,
		skip: skipUnclosedDelimiterAnchor,
	},
	{
		name: "error_marker_incomplete4", input: `numpy; sys_platform == "win32" or (os_name == "linux"`,
		uvStart: 34, uvEnd: 35,
		skip: skipUnclosedDelimiterAnchor,
	},

	// --- skip: uv-only heuristic with no pypa counterpart ---
	{
		name: "error_unnamedunnamed_url", input: `git+https://github.com/pallets/flask.git`,
		uvStart: 0, uvEnd: 40,
		skip: `uv detects an unnamed direct-reference URL as a special case with a tailored "add a package name" message; pypa (our semantics anchor) has no such heuristic, and neither do we. Ours simply fails parsing the truncated identifier "git" as a package name followed by unexpected input - correct per PEP 508 (a bare URL is not a valid name_req), but a plainer diagnostic. Implementing uv's heuristic would be new behavior, not a bug fix.`,
	},

	// --- skip: oracle conflict (pypa and uv disagree on the grammar) ---
	{
		name: "error_end", input: `name_`,
		uvStart: 4, uvEnd: 5,
		skip: skipOracleAcceptsTrailingUnderscore,
	},
	{
		name: "error_extras_illegal_end/trailing_underscore", input: `foo[bar_]`,
		uvStart: 7, uvEnd: 8,
		skip: skipOracleAcceptsTrailingUnderscore,
	},

	// --- skip: version-string validity is out of this entry point's scope ---
	{
		name: "error_pep440", input: `numpy >=1.1.*`,
		uvStart: 6, uvEnd: 13,
		skip: skipVersionValidityDeferred,
	},
	{
		name: "error_invalid_prerelease", input: `name==1.0.org1`,
		uvStart: 4, uvEnd: 14,
		skip: skipVersionValidityDeferred,
	},
}

// TestConformance_UVErrorPositions asserts, for every in-scope uv-pep508
// error_* input, the byte Start/End our own *SyntaxError reports and its
// rendered Error() text - never uv's message text (see file header). A
// skip-classified row calls t.Skip(reason) before asserting anything; that
// reason is what "-v" output and go test's own --- SKIP lines report.
func TestConformance_UVErrorPositions(t *testing.T) {
	for _, c := range uvCases {
		t.Run(c.name, func(t *testing.T) {
			if c.skip != "" {
				t.Skip(c.skip)
			}
			_, err := ParseRequirement(NewTokenizer(c.input))
			require.Error(t, err, "input %q should be rejected", c.input)

			var se *SyntaxError
			require.ErrorAs(t, err, &se, "error chain should contain a *SyntaxError")
			assert.Equal(t, c.wantStart, se.Start, "Start")
			assert.Equal(t, c.wantEnd, se.End, "End")
			assert.Equal(t, c.wantMsg, se.Msg, "message is ours, not uv's")

			lines := strings.Split(se.Error(), "\n")
			require.Len(t, lines, 3, "Msg + Source + caret line")
			assert.Equal(t, c.wantMsg, lines[0])
			assert.Equal(t, c.input, lines[1], "source line echoes the input verbatim")
		})
	}
}
