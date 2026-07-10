// SPDX-License-Identifier: Apache-2.0 OR MIT

package reqtxt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreprocess_Continuations(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    []logicalLine
	}{
		{
			name:    "no-space continuation joins with no separator",
			content: "foo==\\\n1.0",
			want:    []logicalLine{{text: "foo==1.0", line: 1}},
		},
		{
			name:    "space-before-backslash continuation preserves the space",
			content: "foo \\\n1.0",
			want:    []logicalLine{{text: "foo 1.0", line: 1}},
		},
		{
			name:    "CRLF continuation",
			content: "foo==\\\r\n1.0",
			want:    []logicalLine{{text: "foo==1.0", line: 1}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := preprocess(c.content, parseConfig{})
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

// TestPreprocess_WholeLineCommentContinuationGuard exercises pip's
// join_lines guard (req_file.py:501): "if not line.endswith('\\') or
// COMMENT_RE.match(line)" - a physical line ending in "\" is NOT a
// continuation when it is itself a whole-line comment (COMMENT_RE matches
// at position 0, i.e. the first non-whitespace character is "#"). Without
// this guard, a trailing "\" on a comment line joins the next physical
// line directly onto the comment text, so by the time comments are
// stripped the entire logical line - including whatever followed the
// comment - is gone: e.g. "# TODO: pin this \" + "django" joins to
// "# TODO: pin this django", which commentRE then strips to nothing,
// silently discarding the "django" requirement pip would still install.
func TestPreprocess_WholeLineCommentContinuationGuard(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    []logicalLine
	}{
		{
			name:    "whole-line comment ending in backslash does not swallow the next line",
			content: "# TODO: pin this \\\ndjango",
			want:    []logicalLine{{text: "django", line: 2}},
		},
		{
			name:    "indented whole-line comment ending in backslash does not swallow the next line",
			content: "  # c \\\nbar",
			want:    []logicalLine{{text: "bar", line: 2}},
		},
		{
			name:    "regression: an ordinary continuation still joins",
			content: "foo==\\\n1.0",
			want:    []logicalLine{{text: "foo==1.0", line: 1}},
		},
		{
			name:    "regression: an inline comment ending in backslash still joins (not a whole-line comment)",
			content: "django # c \\\nfoo",
			want:    []logicalLine{{text: "django", line: 1}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := preprocess(c.content, parseConfig{})
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestPreprocess_Comments(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    []logicalLine
	}{
		{
			name:    "full-line comment is dropped entirely",
			content: "# a comment",
			want:    nil,
		},
		{
			name:    "inline comment preceded by whitespace is stripped",
			content: "foo # a comment",
			want:    []logicalLine{{text: "foo", line: 1}},
		},
		{
			name:    "bare hash not preceded by whitespace is content, not a comment",
			content: "git+https://h/p#egg=n",
			want:    []logicalLine{{text: "git+https://h/p#egg=n", line: 1}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := preprocess(c.content, parseConfig{})
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestPreprocess_BlankLinesAndWhitespace(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    []logicalLine
	}{
		{
			name:    "blank lines are dropped",
			content: "foo\n\nbar",
			want: []logicalLine{
				{text: "foo", line: 1},
				{text: "bar", line: 3},
			},
		},
		{
			name:    "whitespace-only line is dropped",
			content: "foo\n   \nbar",
			want: []logicalLine{
				{text: "foo", line: 1},
				{text: "bar", line: 3},
			},
		},
		{
			name:    "leading whitespace is trimmed",
			content: "  foo",
			want:    []logicalLine{{text: "foo", line: 1}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := preprocess(c.content, parseConfig{})
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestPreprocess_LineNumbers(t *testing.T) {
	// Line 1 is a comment (dropped), line 2 is blank (dropped), line 3 is
	// the kept logical line.
	content := "# c\n\nfoo"
	got, err := preprocess(content, parseConfig{})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "foo", got[0].text)
	assert.Equal(t, 3, got[0].line)
}

func TestPreprocess_LineNumbers_AfterContinuationJoin(t *testing.T) {
	// Line 1: comment (dropped). Line 2: blank (dropped). Lines 3-4: a
	// continuation join whose first segment is physical line 3. Line 5: a
	// trailing plain line, physical line 5.
	content := "# c\n\nfoo==\\\n1.0\nbar"
	got, err := preprocess(content, parseConfig{})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, logicalLine{text: "foo==1.0", line: 3}, got[0])
	assert.Equal(t, logicalLine{text: "bar", line: 5}, got[1])
}

func TestPreprocess_Env(t *testing.T) {
	lookup := func(name string) (string, bool) {
		switch name {
		case "TOKEN":
			return "t", true
		case "EMPTY":
			return "", true
		default:
			return "", false
		}
	}

	cases := []struct {
		name    string
		content string
		cfg     parseConfig
		want    []logicalLine
	}{
		{
			name:    "set-nonempty var is substituted",
			content: "--index-url https://${TOKEN}@h/s",
			cfg:     parseConfig{env: lookup},
			want:    []logicalLine{{text: "--index-url https://t@h/s", line: 1}},
		},
		{
			name:    "unset var is left literal",
			content: "${MISSING}",
			cfg:     parseConfig{env: lookup},
			want:    []logicalLine{{text: "${MISSING}", line: 1}},
		},
		{
			name:    "set-but-empty var is left literal",
			content: "${EMPTY}",
			cfg:     parseConfig{env: lookup},
			want:    []logicalLine{{text: "${EMPTY}", line: 1}},
		},
		{
			name:    "lowercase name is never expanded",
			content: "${token}",
			cfg:     parseConfig{env: lookup},
			want:    []logicalLine{{text: "${token}", line: 1}},
		},
		{
			name:    "no env configured leaves everything literal",
			content: "${TOKEN}",
			cfg:     parseConfig{},
			want:    []logicalLine{{text: "${TOKEN}", line: 1}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := preprocess(c.content, c.cfg)
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestWithEnv(t *testing.T) {
	lookup := func(name string) (string, bool) {
		return "v", true
	}

	var cfg parseConfig
	opt := WithEnv(lookup)
	opt(&cfg)

	require.NotNil(t, cfg.env)
	v, ok := cfg.env("ANYTHING")
	assert.True(t, ok)
	assert.Equal(t, "v", v)

	got, err := preprocess("${X}", cfg)
	require.NoError(t, err)
	assert.Equal(t, []logicalLine{{text: "v", line: 1}}, got)
}
