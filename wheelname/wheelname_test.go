// SPDX-License-Identifier: Apache-2.0 OR MIT
package wheelname

import (
	"testing"

	"github.com/posit-dev/go-python-packaging/tags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_Valid(t *testing.T) {
	w, err := Parse("numpy-1.26.0-cp311-cp311-manylinux_2_17_x86_64.whl")
	require.NoError(t, err)
	assert.Equal(t, "numpy", w.Name)
	assert.Equal(t, "1.26.0", w.Version.String())
	assert.Equal(t, "", w.Build)
	assert.Equal(t, []tags.Tag{{Interpreter: "cp311", ABI: "cp311", Platform: "manylinux_2_17_x86_64"}}, w.Tags)
}

func TestParse_BuildTagAndCompressed(t *testing.T) {
	w, err := Parse("foo-1.0-1-py2.py3-none-any.whl")
	require.NoError(t, err)
	assert.Equal(t, "1", w.Build)
	assert.ElementsMatch(t, []tags.Tag{
		{Interpreter: "py2", ABI: "none", Platform: "any"},
		{Interpreter: "py3", ABI: "none", Platform: "any"},
	}, w.Tags)
}

func TestParse_LocalVersionKeepsPlus(t *testing.T) {
	w, err := Parse("torch-2.4.0+cu121-cp311-cp311-linux_x86_64.whl")
	require.NoError(t, err)
	assert.Equal(t, "2.4.0+cu121", w.Version.String())
}

func TestParse_Malformed(t *testing.T) {
	for _, s := range []string{
		"nowhlsuffix-1.0-py3-none-any",     // no .whl
		"too-few-py3-none-any.whl",         // wrong field count vs name-with-dash
		"foo-1.0-x-py3-none-any.whl",       // build tag not digit-led
		"foo-notaversion-py3-none-any.whl", // bad version
	} {
		_, err := Parse(s)
		require.Error(t, err, s)
	}
}

func TestCompareBuildTags(t *testing.T) {
	assert.Negative(t, CompareBuildTags("", "1"))   // absent < present
	assert.Negative(t, CompareBuildTags("2", "10")) // numeric, not lexical
	assert.Negative(t, CompareBuildTags("1abc", "1abd"))
	assert.Zero(t, CompareBuildTags("3x", "3x"))
}
