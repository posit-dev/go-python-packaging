// SPDX-License-Identifier: Apache-2.0 OR MIT

package pep508

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseRequirementString is a small test helper mirroring parseFullMarkerString
// in marker_test.go: parse s as a complete requirement and fail the test
// immediately on error.
func parseRequirementString(t *testing.T, s string) RawRequirement {
	t.Helper()
	tok := NewTokenizer(s)
	req, err := ParseRequirement(tok)
	require.NoError(t, err, "source %q", s)
	return req
}

// --- plain name ---

func TestParseRequirement_PlainName(t *testing.T) {
	req := parseRequirementString(t, "foo")
	assert.Equal(t, "foo", req.Name)
	assert.Empty(t, req.Extras)
	assert.Empty(t, req.Specifier)
	assert.Empty(t, req.URL)
	assert.Nil(t, req.Marker)
}

func TestParseRequirement_NameWithDigitsDotsHyphensUnderscores(t *testing.T) {
	req := parseRequirementString(t, "foo_bar-2.0")
	assert.Equal(t, "foo_bar-2.0", req.Name)
}

func TestParseRequirement_LeadingTrailingWhitespace(t *testing.T) {
	req := parseRequirementString(t, "  foo  ")
	assert.Equal(t, "foo", req.Name)
}

// --- extras ---

func TestParseRequirement_SingleExtra(t *testing.T) {
	req := parseRequirementString(t, "foo[bar]")
	assert.Equal(t, []string{"bar"}, req.Extras)
}

func TestParseRequirement_MultipleExtras(t *testing.T) {
	req := parseRequirementString(t, "foo[bar,baz]")
	assert.Equal(t, []string{"bar", "baz"}, req.Extras)
}

func TestParseRequirement_ExtrasWithWhitespace(t *testing.T) {
	req := parseRequirementString(t, "foo[ bar , baz ]")
	assert.Equal(t, []string{"bar", "baz"}, req.Extras)
}

func TestParseRequirement_EmptyExtras(t *testing.T) {
	req := parseRequirementString(t, "foo[]")
	assert.Empty(t, req.Extras)
}

func TestParseRequirement_ExtrasTrailingCommaIsSyntaxError(t *testing.T) {
	tok := NewTokenizer("foo[bar,]")
	_, err := ParseRequirement(tok)
	require.Error(t, err)
	var synErr *SyntaxError
	require.ErrorAs(t, err, &synErr)
}

func TestParseRequirement_UnclosedExtrasIsSyntaxError(t *testing.T) {
	tok := NewTokenizer("foo[bar")
	_, err := ParseRequirement(tok)
	require.Error(t, err)
	var synErr *SyntaxError
	require.ErrorAs(t, err, &synErr)
}

// --- versionspec ---

func TestParseRequirement_BareVersionSpec(t *testing.T) {
	req := parseRequirementString(t, "foo>=1.0")
	assert.Equal(t, ">=1.0", req.Specifier)
}

func TestParseRequirement_ParenthesizedVersionSpec(t *testing.T) {
	req := parseRequirementString(t, "foo(>=1.0)")
	assert.Equal(t, ">=1.0", req.Specifier)
}

func TestParseRequirement_MultipleVersionSpecClauses(t *testing.T) {
	req := parseRequirementString(t, "foo>=1.0,<2.0")
	assert.Equal(t, ">=1.0,<2.0", req.Specifier)
}

func TestParseRequirement_ParenthesizedMultipleVersionSpecClauses(t *testing.T) {
	req := parseRequirementString(t, "foo(>=1.0,<2.0)")
	assert.Equal(t, ">=1.0,<2.0", req.Specifier)
}

func TestParseRequirement_VersionSpecWithWhitespace(t *testing.T) {
	req := parseRequirementString(t, "foo >= 1.0")
	assert.Equal(t, ">=1.0", req.Specifier)
}

// --- grammar asymmetry: name form allows no wsp before ";" ---

func TestParseRequirement_NoSpaceBeforeMarker_NameForm(t *testing.T) {
	req := parseRequirementString(t, `foo>=1.0;python_version>="3.8"`)
	assert.Equal(t, ">=1.0", req.Specifier)
	require.NotNil(t, req.Marker)
	cmp, ok := req.Marker.(*CompareExpr)
	require.True(t, ok)
	assert.Equal(t, EnvVar{Name: "python_version"}, cmp.Lhs)
}

func TestParseRequirement_NoSpaceBeforeMarker_BareNameForm(t *testing.T) {
	// No extras, no versionspec at all - still legal with zero whitespace
	// before ";" per name_req's "wsp*".
	req := parseRequirementString(t, `foo;python_version>="3.8"`)
	assert.Empty(t, req.Specifier)
	require.NotNil(t, req.Marker)
}

// --- @ url form ---

func TestParseRequirement_URL(t *testing.T) {
	req := parseRequirementString(t, "foo @ https://example.com/foo-1.0.whl")
	assert.Equal(t, "https://example.com/foo-1.0.whl", req.URL)
	assert.Empty(t, req.Specifier)
	assert.Nil(t, req.Marker)
}

func TestParseRequirement_URLNoSpaceAroundAt(t *testing.T) {
	req := parseRequirementString(t, "foo@https://example.com/foo-1.0.whl")
	assert.Equal(t, "https://example.com/foo-1.0.whl", req.URL)
}

func TestParseRequirement_URLWithMarker(t *testing.T) {
	req := parseRequirementString(t, `foo @ https://example.com/foo-1.0.whl ; python_version > "3"`)
	assert.Equal(t, "https://example.com/foo-1.0.whl", req.URL)
	require.NotNil(t, req.Marker)
	cmp, ok := req.Marker.(*CompareExpr)
	require.True(t, ok)
	assert.Equal(t, EnvVar{Name: "python_version"}, cmp.Lhs)
	assert.Equal(t, Literal{Value: "3"}, cmp.Rhs)
}

// --- URL containing ";" (the key robustness case) ---

func TestParseRequirement_URLContainingSemicolon_WithMarker(t *testing.T) {
	// The URL itself contains a literal ";" (e.g. a query string). Because
	// the URL token is greedy non-whitespace, the whole "https://x/y;z.whl"
	// is consumed as ONE token - naive splitting on the first ";" in the
	// source would wrongly cut the URL in half.
	req := parseRequirementString(t, `foo @ https://x/y;z.whl ; python_version > "3"`)
	assert.Equal(t, "https://x/y;z.whl", req.URL)
	require.NotNil(t, req.Marker)
	cmp, ok := req.Marker.(*CompareExpr)
	require.True(t, ok)
	assert.Equal(t, EnvVar{Name: "python_version"}, cmp.Lhs)
	assert.Equal(t, Literal{Value: "3"}, cmp.Rhs)
}

func TestParseRequirement_URLContainingSemicolon_NoMarker(t *testing.T) {
	req := parseRequirementString(t, `foo @ https://x/y;z.whl`)
	assert.Equal(t, "https://x/y;z.whl", req.URL)
	assert.Nil(t, req.Marker)
}

// --- URL terminates at any whitespace, not just space/tab ---

func TestParseRequirement_URLThenNewlineThenMarker_Splits(t *testing.T) {
	// A defensively-multiline value: the "@ url" clause ends in a newline
	// rather than a space before "; marker". The URL rule must treat the
	// newline as a terminator (it is the exact complement of the WS rule,
	// \s+) so the marker clause is not absorbed into the URL token.
	req := parseRequirementString(t, "foo @ https://x/y\n; python_version > \"3\"")
	assert.Equal(t, "https://x/y", req.URL)
	require.NotNil(t, req.Marker)
	cmp, ok := req.Marker.(*CompareExpr)
	require.True(t, ok)
	assert.Equal(t, EnvVar{Name: "python_version"}, cmp.Lhs)
	assert.Equal(t, Literal{Value: "3"}, cmp.Rhs)
}

// --- grammar asymmetry: url form requires wsp+ before ";" marker ---

func TestParseRequirement_URLForm_NoSpaceBeforeMarker_SwallowsIntoURL(t *testing.T) {
	// Without the mandatory whitespace, url_req has no way to tell "URL
	// ends here, marker begins" - so the whole remainder, semicolon
	// included, is just part of the (greedy, non-whitespace) URL token.
	// This is not an error: it is PEP 508's documented behavior.
	req := parseRequirementString(t, `foo@https://x/y;python_version>"3"`)
	assert.Equal(t, `https://x/y;python_version>"3"`, req.URL)
	assert.Nil(t, req.Marker)
}

func TestParseRequirement_URLForm_SpaceBeforeMarker_Splits(t *testing.T) {
	req := parseRequirementString(t, `foo@https://x/y ;python_version>"3"`)
	assert.Equal(t, "https://x/y", req.URL)
	require.NotNil(t, req.Marker)
}

// --- combinations ---

func TestParseRequirement_ExtrasAndVersionSpecAndMarker(t *testing.T) {
	req := parseRequirementString(t, `foo[bar,baz]>=1.0,<2.0; python_version >= "3.8"`)
	assert.Equal(t, "foo", req.Name)
	assert.Equal(t, []string{"bar", "baz"}, req.Extras)
	assert.Equal(t, ">=1.0,<2.0", req.Specifier)
	require.NotNil(t, req.Marker)
}

// --- malformed inputs are syntax errors ---

func TestParseRequirement_Empty_IsSyntaxError(t *testing.T) {
	tok := NewTokenizer("")
	_, err := ParseRequirement(tok)
	require.Error(t, err)
}

func TestParseRequirement_MissingURLAfterAt_IsSyntaxError(t *testing.T) {
	tok := NewTokenizer("foo @ ")
	_, err := ParseRequirement(tok)
	require.Error(t, err)
	var synErr *SyntaxError
	require.ErrorAs(t, err, &synErr)
}

func TestParseRequirement_TrailingGarbage_IsSyntaxError(t *testing.T) {
	tok := NewTokenizer("foo bar")
	_, err := ParseRequirement(tok)
	require.Error(t, err)
}

func TestParseRequirement_MarkerSyntaxError_Propagates(t *testing.T) {
	tok := NewTokenizer(`foo; and`)
	_, err := ParseRequirement(tok)
	require.Error(t, err)
	var synErr *SyntaxError
	require.ErrorAs(t, err, &synErr)
}

func TestParseRequirement_UnknownMarkerVariable_IsSyntaxError(t *testing.T) {
	tok := NewTokenizer(`foo; not_a_real_variable == "1"`)
	_, err := ParseRequirement(tok)
	require.Error(t, err)
}

// --- ParseRequirement does not leave a dangling token cached on success ---

func TestParseRequirement_ConsumesEnd(t *testing.T) {
	tok := NewTokenizer("foo")
	_, err := ParseRequirement(tok)
	require.NoError(t, err)
	// A subsequent check must not panic on "already have pending token" -
	// proving ParseRequirement itself consumed (not merely peeked) END.
	assert.NotPanics(t, func() { tok.check(End) })
}

// --- IDENTIFIER \b boundary caveat (tokenizer.go's tokenRules doc) ---
//
// tokenizer.go documents that a LEADING \b in a rule matched via \A against
// source[pos:] always succeeds at the slice start, regardless of the real
// character preceding pos in the unsliced source - so a rule reached right
// after a word-character token (with no true boundary) could wrongly
// report a match. Identifier has a leading \b, so this must be checked.
//
// In the requirement grammar as implemented here, every position where
// Identifier is checked is reached only after: start-of-input, "[", ",",
// whitespace, or (transitively) another delimiter - never directly after a
// word-character-ending token with nothing between. This directly exercises
// that: a name (word-char-ending) immediately followed, with zero
// separator, by a non-Identifier token (LBracket, OP-shaped Specifier,
// Semicolon, AT) - i.e. exactly "a word-char token immediately followed by
// another token with no separator" - and confirms each parses as the
// grammar intends, never as a spuriously split/merged Identifier.
func TestIdentifier_NeverCheckedAfterWordCharWithNoBoundary(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{"name immediately followed by extras", "foo[bar]"},
		{"name immediately followed by versionspec", "foo>=1.0"},
		{"name immediately followed by url-spec", "foo@https://x"},
		{"name immediately followed by marker", "foo;python_version>=\"3.8\""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tok := NewTokenizer(c.source)
			require.True(t, tok.check(Identifier), "source %q", c.source)
			nameTok := tok.read()
			// The Identifier match must stop exactly at "foo" - the first
			// character outside Identifier's char class - and must not
			// spuriously consume (or refuse to consume) based on what
			// immediately follows.
			assert.Equal(t, "foo", nameTok.Text, "source %q", c.source)
		})
	}
}

// A second, lower-level demonstration: Identifier's own greedy char class
// (which includes digits, ".", "_", "-") means two runs of allowed
// characters with nothing but allowed characters between them are simply
// ONE token, by construction - there is no way to "immediately follow" an
// Identifier match with another Identifier check at a false boundary, since
// the first check would have already consumed everything up to the true
// next delimiter.
func TestIdentifier_GreedyMatchConsumesWholeRunNoFalseSplit(t *testing.T) {
	tok := NewTokenizer("foo123_bar-baz.qux")
	require.True(t, tok.check(Identifier))
	got := tok.read()
	assert.Equal(t, "foo123_bar-baz.qux", got.Text)
}
