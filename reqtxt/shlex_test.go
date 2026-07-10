// SPDX-License-Identifier: Apache-2.0 OR MIT

package reqtxt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShlexSplit(t *testing.T) {
	cases := []struct {
		name string
		line string
		want []string
	}{
		{
			name: "simple words",
			line: "a b c",
			want: []string{"a", "b", "c"},
		},
		{
			name: "double-quoted argument with embedded space",
			line: `--find-links "/opt/my links"`,
			want: []string{"--find-links", "/opt/my links"},
		},
		{
			name: "single-quoted argument with embedded space",
			line: `-r 'sub dir/reqs.txt'`,
			want: []string{"-r", "sub dir/reqs.txt"},
		},
		{
			name: "single quotes preserve backslash literally",
			line: `'a\b'`,
			want: []string{`a\b`},
		},
		{
			name: "double quotes allow backslash to escape a double quote",
			line: `"a\"b"`,
			want: []string{`a"b`},
		},
		{
			name: "leading, trailing, and multiple whitespace collapse",
			line: "   a    b   c   ",
			want: []string{"a", "b", "c"},
		},
		{
			name: "empty input",
			line: "",
			want: []string{},
		},
		{
			name: "whitespace-only input",
			line: "   ",
			want: []string{},
		},
		{
			name: "adjacent quoted and unquoted runs concatenate",
			line: `a"b"c`,
			want: []string{"abc"},
		},
		{
			name: "backslash outside quotes escapes a space",
			line: `a\ b`,
			want: []string{"a b"},
		},
		{
			name: "double quotes preserve a literal backslash before a non-special char",
			line: `"a\nb"`,
			want: []string{`a\nb`},
		},
		{
			name: "adjacent single- and double-quoted runs concatenate",
			line: `'a b'"c d"e`,
			want: []string{"a bc de"},
		},
		{
			name: "empty quoted string still produces a word",
			line: `''`,
			want: []string{""},
		},
		{
			name: "form feed is not a word separator (unlike unicode.IsSpace)",
			line: "a\x0cb",
			want: []string{"a\x0cb"},
		},
		{
			name: "vertical tab is not a word separator (unlike unicode.IsSpace)",
			line: "a\vb",
			want: []string{"a\vb"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := shlexSplit(c.line)
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestShlexSplit_Errors(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{name: "unterminated double quote", line: `"abc`},
		{name: "unterminated single quote", line: `'abc`},
		{name: "trailing bare backslash after a word", line: `a\`},
		{name: "bare backslash with nothing else", line: `\`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := shlexSplit(c.line)
			require.Error(t, err)
		})
	}
}
