// SPDX-License-Identifier: Apache-2.0 OR MIT

package pep508

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ported from pypa/packaging tests/test_requirements.py, class
// TestRequirementParsing. Pinned:
// 4eb0753dba8fcaaac8eb75463374e448f0931558.

// test_empty_specifier: "name()" is VALID and means no constraint.
func TestParseRequirement_EmptyParenthesisedSpecifier(t *testing.T) {
	for _, s := range []string{"name()", "name ()", "name( )", "name (  )"} {
		t.Run(s, func(t *testing.T) {
			raw, err := ParseRequirement(NewTokenizer(s))
			require.NoError(t, err, "input %q should parse", s)
			assert.Equal(t, "name", raw.Name)
			assert.Equal(t, "", raw.Specifier, "empty parens mean no specifier")
		})
	}
}

// test_error_when_suffixed_with_line_break: parametrized over "\n", "\r",
// "\r\n" and over two bases.
func TestParseRequirement_RejectsTrailingLineBreak(t *testing.T) {
	bases := []string{"name>=1", `name; python_version >= "3"`}
	breaks := []string{"\n", "\r", "\r\n"}
	for _, base := range bases {
		for _, lb := range breaks {
			in := base + lb
			t.Run(base+"+"+escapeForName(lb), func(t *testing.T) {
				_, err := ParseRequirement(NewTokenizer(in))
				assert.Error(t, err, "input %q should be rejected", in)
			})
		}
	}
}

// A trailing horizontal space or tab remains VALID
// (test_trailing_horizontal_whitespace).
func TestParseRequirement_AllowsTrailingHorizontalWhitespace(t *testing.T) {
	for _, ws := range []string{" ", "\t", " \t"} {
		t.Run(escapeForName(ws), func(t *testing.T) {
			_, err := ParseRequirement(NewTokenizer("name>=1" + ws))
			assert.NoError(t, err, "trailing horizontal whitespace is legal")
		})
	}
}

// test_error_invalid_marker_malformed_quoted_string: the closing quote is
// escaped, so the string is unterminated.
func TestParseRequirement_RejectsMalformedQuotedString(t *testing.T) {
	for _, s := range []string{`name; os_name == "C:\"`, `name; os_name == "\x"`} {
		t.Run(s, func(t *testing.T) {
			_, err := ParseRequirement(NewTokenizer(s))
			assert.Error(t, err, "input %q should be rejected", s)
		})
	}
}

// escapeForName renders whitespace visibly so subtest names stay distinct.
// Go replaces a space with "_" in subtest names, which would otherwise make
// the space and tab cases indistinguishable in test output.
func escapeForName(s string) string {
	out := ""
	for _, r := range s {
		switch r {
		case '\n':
			out += "LF"
		case '\r':
			out += "CR"
		case '\t':
			out += "TAB"
		case ' ':
			out += "SP"
		default:
			out += string(r)
		}
	}
	return out
}
