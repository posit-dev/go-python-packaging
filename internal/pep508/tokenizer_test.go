// SPDX-License-Identifier: Apache-2.0 OR MIT
package pep508

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- OP: longest-match ordering ---

func TestCheck_OP_LongestMatch(t *testing.T) {
	cases := []struct {
		source string
		want   string
	}{
		{"===1.0", "==="},
		{"==1.0", "=="},
		{"~=1.0", "~="},
		{"!=1.0", "!="},
		{"<=1.0", "<="},
		{">=1.0", ">="},
		{"<1.0", "<"},
		{">1.0", ">"},
	}
	for _, c := range cases {
		tok := NewTokenizer(c.source)
		ok := tok.check(OP)
		require.True(t, ok, "source %q", c.source)
		got := tok.read()
		assert.Equal(t, OP, got.Kind)
		assert.Equal(t, c.want, got.Text, "source %q", c.source)
		assert.Equal(t, 0, got.Pos)
	}
}

func TestCheck_OP_NoMatch(t *testing.T) {
	tok := NewTokenizer("banana")
	assert.False(t, tok.check(OP))
}

// --- QUOTED_STRING ---

func TestCheck_QuotedString_SingleQuoted(t *testing.T) {
	tok := NewTokenizer(`'posix'`)
	require.True(t, tok.check(QuotedString))
	got := tok.read()
	assert.Equal(t, QuotedString, got.Kind)
	assert.Equal(t, `'posix'`, got.Text)
	assert.Equal(t, "posix", got.Unquoted())
}

func TestCheck_QuotedString_DoubleQuoted(t *testing.T) {
	tok := NewTokenizer(`"posix"`)
	require.True(t, tok.check(QuotedString))
	got := tok.read()
	assert.Equal(t, `"posix"`, got.Text)
	assert.Equal(t, "posix", got.Unquoted())
}

func TestCheck_QuotedString_EmbeddedOtherQuoteChar(t *testing.T) {
	// A double-quoted string may embed single-quote characters, and
	// vice versa, without any escaping.
	tok := NewTokenizer(`"it's fine"`)
	require.True(t, tok.check(QuotedString))
	got := tok.read()
	assert.Equal(t, `"it's fine"`, got.Text)
	assert.Equal(t, "it's fine", got.Unquoted())

	tok2 := NewTokenizer(`'she said "hi"'`)
	require.True(t, tok2.check(QuotedString))
	got2 := tok2.read()
	assert.Equal(t, `'she said "hi"'`, got2.Text)
	assert.Equal(t, `she said "hi"`, got2.Unquoted())
}

func TestCheck_QuotedString_Unterminated(t *testing.T) {
	tok := NewTokenizer(`'posix`)
	assert.False(t, tok.check(QuotedString))
}

// --- WS handling ---

func TestConsume_WS_SkipsWhitespace(t *testing.T) {
	tok := NewTokenizer("   and")
	assert.True(t, tok.consume(WS))
	require.True(t, tok.check(BoolOp))
	got := tok.read()
	assert.Equal(t, "and", got.Text)
	assert.Equal(t, 3, got.Pos)
}

func TestConsume_WS_NoOpWhenAbsent(t *testing.T) {
	tok := NewTokenizer("and")
	assert.False(t, tok.consume(WS))
	assert.True(t, tok.check(BoolOp))
}

// --- VARIABLE recognition ---

func TestCheck_Variable_PythonImplementationAlias(t *testing.T) {
	tok := NewTokenizer("python_implementation")
	require.True(t, tok.check(Variable))
	got := tok.read()
	assert.Equal(t, "python_implementation", got.Text)
	assert.Equal(t, End, kindOfEnd(tok)) // sanity: nothing left but END
}

func TestCheck_Variable_LongestNameFirst(t *testing.T) {
	// python_full_version must tokenize whole, not split into
	// "python_version"-shaped junk (it doesn't share that prefix, but
	// the rule ordering must still be longest-alternative-first per
	// the tokenizer's construction).
	tok := NewTokenizer("python_full_version")
	require.True(t, tok.check(Variable))
	got := tok.read()
	assert.Equal(t, "python_full_version", got.Text)

	tok2 := NewTokenizer("python_version")
	require.True(t, tok2.check(Variable))
	got2 := tok2.read()
	assert.Equal(t, "python_version", got2.Text)
}

func TestCheck_Variable_PlatformPythonImplementation(t *testing.T) {
	tok := NewTokenizer("platform_python_implementation")
	require.True(t, tok.check(Variable))
	got := tok.read()
	assert.Equal(t, "platform_python_implementation", got.Text)
}

func TestCheck_Variable_AllRecognizedNames(t *testing.T) {
	names := []string{
		"os_name", "os.name",
		"sys_platform", "sys.platform",
		"platform_release",
		"platform_system",
		"platform_version", "platform.version",
		"platform_machine", "platform.machine",
		"implementation_name",
		"implementation_version",
		"extra",
	}
	for _, name := range names {
		tok := NewTokenizer(name)
		require.True(t, tok.check(Variable), "name %q", name)
		got := tok.read()
		assert.Equal(t, name, got.Text, "name %q", name)
	}
}

func TestCheck_Variable_RejectsUnknownName(t *testing.T) {
	tok := NewTokenizer("not_a_real_variable")
	assert.False(t, tok.check(Variable))
}

// --- BOOLOP / IN / NOT word-boundary ---

func TestCheck_In_WordBoundary_RejectsInformation(t *testing.T) {
	// "information" must not tokenize as IN ("in") followed by leftover
	// "formation": the IN rule's trailing \b must reject the match
	// entirely because 'n' is directly followed by a word character.
	tok := NewTokenizer("information")
	assert.False(t, tok.check(IN))
	// Nor is it a recognized VARIABLE.
	assert.False(t, tok.check(Variable))
}

func TestCheck_In_MatchesStandalone(t *testing.T) {
	tok := NewTokenizer("in 'foo'")
	require.True(t, tok.check(IN))
	got := tok.read()
	assert.Equal(t, "in", got.Text)
}

func TestCheck_Not_WordBoundary_RejectsNotice(t *testing.T) {
	tok := NewTokenizer("notice")
	assert.False(t, tok.check(NOT))
}

func TestCheck_Not_MatchesStandalone(t *testing.T) {
	tok := NewTokenizer("not in")
	require.True(t, tok.check(NOT))
	got := tok.read()
	assert.Equal(t, "not", got.Text)
}

func TestCheck_BoolOp_WordBoundary(t *testing.T) {
	// "android" must not be mistaken for "and" + "roid".
	tok := NewTokenizer("android")
	assert.False(t, tok.check(BoolOp))

	tok2 := NewTokenizer("and or")
	require.True(t, tok2.check(BoolOp))
	got := tok2.read()
	assert.Equal(t, "and", got.Text)
	tok2.consume(WS)
	require.True(t, tok2.check(BoolOp))
	got2 := tok2.read()
	assert.Equal(t, "or", got2.Text)
}

// --- LPAREN / RPAREN / END ---

func TestCheck_ParensAndEnd(t *testing.T) {
	tok := NewTokenizer("()")
	require.True(t, tok.check(LParen))
	assert.Equal(t, "(", tok.read().Text)
	require.True(t, tok.check(RParen))
	assert.Equal(t, ")", tok.read().Text)
	require.True(t, tok.check(End))
}

func TestCheck_End_FalseWhenSourceRemains(t *testing.T) {
	tok := NewTokenizer("x")
	assert.False(t, tok.check(End))
}

// --- expect() ---

func TestExpect_ReadsMatchingToken(t *testing.T) {
	tok := NewTokenizer("and")
	got, err := tok.expect(BoolOp, "boolean operator")
	require.NoError(t, err)
	assert.Equal(t, "and", got.Text)
}

func TestExpect_SyntaxErrorOnMismatch(t *testing.T) {
	tok := NewTokenizer("banana")
	_, err := tok.expect(BoolOp, "boolean operator")
	require.Error(t, err)
	var synErr *SyntaxError
	require.ErrorAs(t, err, &synErr)
	assert.Equal(t, "Expected boolean operator", synErr.Msg)
	assert.Equal(t, "banana", synErr.Source)
	assert.Equal(t, 0, synErr.Start)
	assert.Equal(t, 0, synErr.End)
}

// --- syntax error position/rendering ---

func TestSyntaxError_PositionAfterConsumedTokens(t *testing.T) {
	tok := NewTokenizer("and banana")
	require.True(t, tok.check(BoolOp))
	tok.read()
	tok.consume(WS)
	_, err := tok.expect(NOT, "'not'")
	require.Error(t, err)
	var synErr *SyntaxError
	require.ErrorAs(t, err, &synErr)
	assert.Equal(t, 4, synErr.Start, "position should be after 'and '")
	assert.Equal(t, 4, synErr.End)
}

func TestSyntaxError_ErrorStringHasCaretAtPosition(t *testing.T) {
	tok := NewTokenizer("banana")
	_, err := tok.expect(BoolOp, "boolean operator")
	require.Error(t, err)
	want := "Expected boolean operator\nbanana\n^"
	assert.Equal(t, want, err.Error())
}

func TestSyntaxError_SpanRendersTildes(t *testing.T) {
	e := &SyntaxError{Msg: "Expected end of input", Source: "and banana", Start: 4, End: 10}
	assert.Equal(t, "Expected end of input\nand banana\n    ~~~~~~^", e.Error())
}

// --- read()/check() invariant panics ---

func TestRead_PanicsWithoutPriorCheck(t *testing.T) {
	tok := NewTokenizer("and")
	assert.Panics(t, func() { tok.read() })
}

func TestCheck_PanicsIfTokenAlreadyPending(t *testing.T) {
	tok := NewTokenizer("and")
	require.True(t, tok.check(BoolOp))
	assert.Panics(t, func() { tok.check(BoolOp) })
}

// --- peek() lookahead without consuming ---

func TestPeek_DoesNotCacheToken(t *testing.T) {
	tok := NewTokenizer("and")
	require.True(t, tok.peek(BoolOp))
	// A subsequent check (not peek) must still succeed - peek did not
	// leave a pending token behind that would trip the "already have a
	// pending token" invariant.
	require.True(t, tok.check(BoolOp))
	got := tok.read()
	assert.Equal(t, "and", got.Text)
}

// kindOfEnd is a tiny test helper asserting the tokenizer is positioned
// at End after consuming the prior token.
func kindOfEnd(t *Tokenizer) Kind {
	if !t.check(End) {
		panic("expected End")
	}
	return End
}
