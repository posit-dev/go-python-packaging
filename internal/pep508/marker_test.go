// SPDX-License-Identifier: Apache-2.0 OR MIT

package pep508

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseFullMarkerString is a small test helper: parse s as a complete marker
// (mirroring what package marker's Parse does) and fail the test immediately
// on error, since most AST-shape assertions below don't want to repeat the
// success-path boilerplate.
func parseFullMarkerString(t *testing.T, s string) Expr {
	t.Helper()
	tok := NewTokenizer(s)
	expr, err := ParseFullMarker(tok)
	require.NoError(t, err)
	return expr
}

// --- basic comparison ---

func TestParseMarker_SimpleComparison(t *testing.T) {
	expr := parseFullMarkerString(t, `python_version >= "3.8"`)
	cmp, ok := expr.(*CompareExpr)
	require.True(t, ok, "expected *CompareExpr, got %T", expr)
	assert.Equal(t, EnvVar{Name: "python_version"}, cmp.Lhs)
	assert.Equal(t, CompareOp(">="), cmp.Op)
	assert.Equal(t, Literal{Value: "3.8"}, cmp.Rhs)
}

func TestParseMarker_QuotedStringLiteralOnLhs(t *testing.T) {
	expr := parseFullMarkerString(t, `"3.8" <= python_version`)
	cmp, ok := expr.(*CompareExpr)
	require.True(t, ok)
	assert.Equal(t, Literal{Value: "3.8"}, cmp.Lhs)
	assert.Equal(t, EnvVar{Name: "python_version"}, cmp.Rhs)
}

func TestParseMarker_InOperator(t *testing.T) {
	expr := parseFullMarkerString(t, `python_version in "2.7 3.5"`)
	cmp, ok := expr.(*CompareExpr)
	require.True(t, ok)
	assert.Equal(t, CompareOp("in"), cmp.Op)
}

func TestParseMarker_NotInOperator(t *testing.T) {
	expr := parseFullMarkerString(t, `python_version not in "2.7 3.5"`)
	cmp, ok := expr.(*CompareExpr)
	require.True(t, ok)
	assert.Equal(t, CompareOp("not in"), cmp.Op)
}

// --- and/or precedence ---

func TestParseMarker_AndBindsTighterThanOr(t *testing.T) {
	// "a or b and c" must group as Or{a, And{b, c}}, not And{Or{a,b}, c}.
	expr := parseFullMarkerString(t, `os_name == "posix" or sys_platform == "linux" and extra == "foo"`)
	or, ok := expr.(*BoolExpr)
	require.True(t, ok, "expected top-level *BoolExpr, got %T", expr)
	require.Equal(t, Or, or.Op)
	require.Len(t, or.Operands, 2)

	a, ok := or.Operands[0].(*CompareExpr)
	require.True(t, ok, "operand 0 should be a bare comparison, got %T", or.Operands[0])
	assert.Equal(t, EnvVar{Name: "os_name"}, a.Lhs)

	and, ok := or.Operands[1].(*BoolExpr)
	require.True(t, ok, "operand 1 should be a nested And, got %T", or.Operands[1])
	assert.Equal(t, And, and.Op)
	require.Len(t, and.Operands, 2)
}

func TestParseMarker_AndBindsTighterThanOr_OtherOrder(t *testing.T) {
	// "a and b or c" must group as Or{And{a,b}, c}.
	expr := parseFullMarkerString(t, `os_name == "posix" and sys_platform == "linux" or extra == "foo"`)
	or, ok := expr.(*BoolExpr)
	require.True(t, ok)
	require.Equal(t, Or, or.Op)
	require.Len(t, or.Operands, 2)

	and, ok := or.Operands[0].(*BoolExpr)
	require.True(t, ok, "operand 0 should be a nested And, got %T", or.Operands[0])
	assert.Equal(t, And, and.Op)

	c, ok := or.Operands[1].(*CompareExpr)
	require.True(t, ok, "operand 1 should be a bare comparison, got %T", or.Operands[1])
	assert.Equal(t, EnvVar{Name: "extra"}, c.Lhs)
}

func TestParseMarker_ParenthesesOverridePrecedence(t *testing.T) {
	// "(a or b) and c" must group as And{Or{a,b}, c} - the parens force an
	// Or to nest inside an And, which precedence alone would never produce.
	expr := parseFullMarkerString(t, `(os_name == "posix" or sys_platform == "linux") and extra == "foo"`)
	and, ok := expr.(*BoolExpr)
	require.True(t, ok, "expected top-level *BoolExpr, got %T", expr)
	require.Equal(t, And, and.Op)
	require.Len(t, and.Operands, 2)

	or, ok := and.Operands[0].(*BoolExpr)
	require.True(t, ok, "operand 0 should be a nested Or, got %T", and.Operands[0])
	assert.Equal(t, Or, or.Op)
}

func TestParseMarker_ChainedAndIsOneGroup(t *testing.T) {
	expr := parseFullMarkerString(t, `os_name == "posix" and sys_platform == "linux" and extra == "foo"`)
	and, ok := expr.(*BoolExpr)
	require.True(t, ok)
	assert.Equal(t, And, and.Op)
	assert.Len(t, and.Operands, 3)
}

// --- no comparison chaining ---

func TestParseMarker_NoComparisonChaining_IsSyntaxError(t *testing.T) {
	tok := NewTokenizer(`python_version < "3" < "4"`)
	_, err := ParseFullMarker(tok)
	require.Error(t, err)
	var synErr *SyntaxError
	require.ErrorAs(t, err, &synErr)
}

func TestParseMarker_DanglingBoolOp_IsSyntaxError(t *testing.T) {
	tok := NewTokenizer(`python_version >= "3.8" or`)
	_, err := ParseFullMarker(tok)
	require.Error(t, err)
}

// --- unknown variable ---

func TestParseMarker_UnknownVariable_IsSyntaxError(t *testing.T) {
	tok := NewTokenizer(`foo == "1"`)
	_, err := ParseFullMarker(tok)
	require.Error(t, err)
	var synErr *SyntaxError
	require.ErrorAs(t, err, &synErr)
}

// --- alias folding ---

func TestParseMarker_AliasFolding(t *testing.T) {
	cases := []struct {
		source string
		want   string
	}{
		{"python_implementation", "platform_python_implementation"},
		{"platform_python_implementation", "platform_python_implementation"},
		{"os.name", "os_name"},
		{"os_name", "os_name"},
		{"sys.platform", "sys_platform"},
		{"sys_platform", "sys_platform"},
		{"platform.version", "platform_version"},
		{"platform_version", "platform_version"},
		{"platform.machine", "platform_machine"},
		{"platform_machine", "platform_machine"},
		{"platform.python_implementation", "platform_python_implementation"},
	}
	for _, c := range cases {
		expr := parseFullMarkerString(t, c.source+` == "x"`)
		cmp, ok := expr.(*CompareExpr)
		require.True(t, ok, "source %q", c.source)
		assert.Equal(t, EnvVar{Name: c.want}, cmp.Lhs, "source %q", c.source)
	}
}

// --- extra literal normalization ---

func TestParseMarker_ExtraNormalization_VariableOnLhs(t *testing.T) {
	expr := parseFullMarkerString(t, `extra == "Foo_Bar"`)
	cmp, ok := expr.(*CompareExpr)
	require.True(t, ok)
	assert.Equal(t, Literal{Value: "foo-bar"}, cmp.Rhs)
}

func TestParseMarker_ExtraNormalization_VariableOnRhs(t *testing.T) {
	expr := parseFullMarkerString(t, `"Foo_Bar" == extra`)
	cmp, ok := expr.(*CompareExpr)
	require.True(t, ok)
	assert.Equal(t, Literal{Value: "foo-bar"}, cmp.Lhs)
}

func TestParseMarker_ExtraNormalization_NotAppliedToNonExtraComparisons(t *testing.T) {
	expr := parseFullMarkerString(t, `os_name == "Foo_Bar"`)
	cmp, ok := expr.(*CompareExpr)
	require.True(t, ok)
	assert.Equal(t, Literal{Value: "Foo_Bar"}, cmp.Rhs)
}

// --- ParseMarker does not consume/verify END ---

func TestParseMarker_DoesNotCheckEnd(t *testing.T) {
	tok := NewTokenizer(`python_version >= "3.8" garbage`)
	expr, err := ParseMarker(tok)
	require.NoError(t, err, "ParseMarker must not itself verify END")
	_, ok := expr.(*CompareExpr)
	assert.True(t, ok)
	// Confirm trailing input was left unconsumed for the caller to notice.
	assert.False(t, tok.check(End))
}

// --- String() rendering ---

func TestBoolExpr_String_ParenthesizesOrInsideAnd(t *testing.T) {
	expr := parseFullMarkerString(t, `(os_name == "posix" or sys_platform == "linux") and extra == "foo"`)
	assert.Equal(t, `(os_name == "posix" or sys_platform == "linux") and extra == "foo"`, expr.String())
}

func TestBoolExpr_String_NoParensNeededForAndInsideOr(t *testing.T) {
	expr := parseFullMarkerString(t, `os_name == "posix" or sys_platform == "linux" and extra == "foo"`)
	assert.Equal(t, `os_name == "posix" or sys_platform == "linux" and extra == "foo"`, expr.String())
}

func TestCompareExpr_String(t *testing.T) {
	expr := parseFullMarkerString(t, `python_version >= "3.8"`)
	assert.Equal(t, `python_version >= "3.8"`, expr.String())
}

func TestLiteral_String_UsesSingleQuoteWhenValueContainsDoubleQuote(t *testing.T) {
	lit := Literal{Value: `has "quote"`}
	assert.Equal(t, `'has "quote"'`, lit.String())
}

// --- Escape decoding (#19401) ---
//
// Boundary cases below were verified by hand against packaging 26.2's
// process_python_str (_parser.py) and QUOTED_STRING (_tokenizer.py) - via
// ast.literal_eval, which process_python_str essentially is - since this
// task was scoped without direct access to packaging's own test suite
// (tests/test_markers.py / test_tokenizer.py). No pinned SHA is cited for
// this specific escape-handling table for that reason; contrast
// grammar_conformance_test.go, whose requirement-parsing ports ARE pinned to
// a verified SHA.

func TestDecodeQuotedStringContents_Accepts(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"doubled_backslash_then_U", `C:\\U`, `C:\U`},
		{"newline", `\n`, "\n"},
		{"tab", `\t`, "\t"},
		{"carriage_return", `\r`, "\r"},
		{"nul_octal", `\0`, "\x00"},
		{"escaped_single_quote", `\'`, `'`},
		{"escaped_double_quote", `\"`, `"`},
		{"alert", `\a`, "\a"},
		{"backspace", `\b`, "\b"},
		{"form_feed", `\f`, "\f"},
		{"vertical_tab", `\v`, "\v"},
		{"hex_escape", `\x41`, "A"},
		{"hex_escape_high_byte", `\xFF`, "\u00FF"},
		{"unicode_escape", `\u0041`, "A"},
		{"unicode_escape_wide", `\U0001F600`, "\U0001F600"},
		{"octal_two_digit", `\101`, "A"},             // 0o101 == 65 == 'A'
		{"octal_three_digit_wide", `\401`, "\u0101"}, // 0o401 == 257
		{"octal_single_digit", `\7`, "\a"},           // 0o7 == 7 == BEL
		{"octal_greedy_stops_at_non_octal", `\1a`, "\x01a"},
		// Regression guard: an escaped backslash ("\\") consumes BOTH its
		// source bytes as one unit, so the "x41"/"x" that follows is
		// literal characters, not a fresh \x escape. A validator that scans
		// for "\x" independently of consumed "\\" pairs misreads this as a
		// truncated \x escape and wrongly rejects it - the regression
		// #18640's review caught; see decodeQuotedStringContents's doc
		// comment in marker.go for why a single pass avoids it.
		{"doubled_backslash_before_x_regression", `C:\\x41`, `C:\x41`},
		{"doubled_backslash_before_x_short", `\\x`, `\x`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := decodeQuotedStringContents(c.in)
			require.NoError(t, err, "input %q should decode", c.in)
			assert.Equal(t, c.want, got, "input %q", c.in)
		})
	}
}

func TestDecodeQuotedStringContents_Rejects(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"trailing_unpaired_backslash", `C:\`},
		{"truncated_hex_empty", `\x`},
		{"truncated_hex_one_digit", `\x4`},
		{"truncated_hex_bad_digit", `\xZZ`},
		{"truncated_unicode_empty", `\u`},
		{"truncated_unicode_short", `\u123`},
		{"truncated_unicode_wide_empty", `\U`},
		{"truncated_unicode_wide_short", `\U1234567`},
		{"unicode_wide_out_of_range", `\U00110000`},
		{"invalid_octal_digit_8", `\8`},
		{"invalid_octal_digit_9", `\9`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := decodeQuotedStringContents(c.in)
			require.Error(t, err, "input %q should be rejected", c.in)
			assert.Contains(t, err.Error(), "Invalid quoted string")
		})
	}
}

// TestDecodeQuotedStringContents_RoundTripIsIdempotent checks that decoding
// is a pure, deterministic function: re-encoding a decoded value back into
// an escaped literal body and decoding it again reproduces the same value,
// and decoding the same input twice never differs.
func TestDecodeQuotedStringContents_RoundTripIsIdempotent(t *testing.T) {
	cases := []string{
		`C:\\U`,
		`\n\t\r`,
		`\x41\u0041\U0001F600`,
		`\101\7`,
		`C:\\x41`,
		`it's "fine"`,
		`plain text, no escapes`,
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			decoded1, err := decodeQuotedStringContents(in)
			require.NoError(t, err)

			reencoded := escapeBackslashesAndQuotes(decoded1)
			decoded2, err := decodeQuotedStringContents(reencoded)
			require.NoError(t, err)
			assert.Equal(t, decoded1, decoded2, "decoding must be stable across a round trip")

			decodedAgain, err := decodeQuotedStringContents(in)
			require.NoError(t, err)
			assert.Equal(t, decoded1, decodedAgain, "decoding the same input twice must agree")
		})
	}
}

// escapeBackslashesAndQuotes re-encodes a decoded string value into a
// Python-style escaped literal body, for
// TestDecodeQuotedStringContents_RoundTripIsIdempotent. It only needs to
// handle what decodeQuotedStringContents can itself produce (a bare
// backslash or quote character), since every other decoded rune requires no
// escape to round-trip.
func escapeBackslashesAndQuotes(s string) string {
	out := ""
	for _, r := range s {
		switch r {
		case '\\':
			out += `\\`
		case '\'':
			out += `\'`
		case '"':
			out += `\"`
		default:
			out += string(r)
		}
	}
	return out
}

// --- Escape decoding, via the public marker-parsing surface ---

func TestParseMarker_QuotedStringLiteralDecodesEscapes(t *testing.T) {
	expr := parseFullMarkerString(t, `os_name == "line\nbreak"`)
	cmp, ok := expr.(*CompareExpr)
	require.True(t, ok)
	assert.Equal(t, Literal{Value: "line\nbreak"}, cmp.Rhs)
}

func TestParseMarker_QuotedStringRejectsMalformedUnicodeEscape(t *testing.T) {
	tok := NewTokenizer(`os_name == "\u12"`)
	_, err := ParseMarker(tok)
	assert.Error(t, err)
}
