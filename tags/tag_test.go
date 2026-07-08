// SPDX-License-Identifier: Apache-2.0 OR MIT
package tags

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTagString(t *testing.T) {
	assert.Equal(t, "cp311-cp311-manylinux_2_17_x86_64",
		Tag{"cp311", "cp311", "manylinux_2_17_x86_64"}.String())
}

func TestParseTag_Simple(t *testing.T) {
	got, err := ParseTag("cp311-cp311-manylinux_2_17_x86_64")
	require.NoError(t, err)
	assert.Equal(t, []Tag{{"cp311", "cp311", "manylinux_2_17_x86_64"}}, got)
}

func TestParseTag_CartesianAllThreeFields(t *testing.T) {
	got, err := ParseTag("cp36.cp37-abi3-manylinux1_x86_64.manylinux2010_x86_64")
	require.NoError(t, err)
	assert.ElementsMatch(t, []Tag{
		{"cp36", "abi3", "manylinux1_x86_64"},
		{"cp36", "abi3", "manylinux2010_x86_64"},
		{"cp37", "abi3", "manylinux1_x86_64"},
		{"cp37", "abi3", "manylinux2010_x86_64"},
	}, got)
}

func TestParseTag_PurePython(t *testing.T) {
	got, err := ParseTag("py2.py3-none-any")
	require.NoError(t, err)
	assert.ElementsMatch(t, []Tag{{"py2", "none", "any"}, {"py3", "none", "any"}}, got)
}

func TestParseTag_Invalid(t *testing.T) {
	for _, s := range []string{"cp311-cp311", "a-b-c-d", "cp311--any", "cp311-none-"} {
		_, err := ParseTag(s)
		require.Error(t, err, s)
	}
}
