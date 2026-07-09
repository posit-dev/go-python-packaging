// SPDX-License-Identifier: Apache-2.0 OR MIT

package marker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_Simple(t *testing.T) {
	m, err := Parse(`python_version >= "3.8"`)
	require.NoError(t, err)
	assert.False(t, m.IsEmpty())
	assert.Equal(t, `python_version >= "3.8"`, m.String())
}

func TestParse_Empty_IsSyntaxError(t *testing.T) {
	_, err := Parse("")
	assert.Error(t, err)
}

func TestParse_WhitespaceOnly_IsSyntaxError(t *testing.T) {
	_, err := Parse("   ")
	assert.Error(t, err)
}

func TestParse_TrailingGarbage_IsSyntaxError(t *testing.T) {
	_, err := Parse(`python_version >= "3.8" or`)
	assert.Error(t, err)
}

func TestParse_NoComparisonChaining_IsSyntaxError(t *testing.T) {
	_, err := Parse(`python_version < "3" < "4"`)
	assert.Error(t, err)
}

func TestParse_UnknownVariable_IsSyntaxError(t *testing.T) {
	_, err := Parse(`unknown_var == "1"`)
	assert.Error(t, err)
}

func TestMarker_ZeroValue_IsAlwaysTrue(t *testing.T) {
	var m Marker
	assert.True(t, m.IsEmpty())
	assert.Equal(t, "", m.String())
}

func TestParse_PythonImplementationAlias_FoldsToCanonicalName(t *testing.T) {
	m, err := Parse(`python_implementation == "CPython"`)
	require.NoError(t, err)
	assert.Equal(t, `platform_python_implementation == "CPython"`, m.String())
}

func TestParse_ExtraNormalization(t *testing.T) {
	m, err := Parse(`extra == "Foo_Bar"`)
	require.NoError(t, err)
	assert.Equal(t, `extra == "foo-bar"`, m.String())
}

// --- round trip: Parse(m.String()) is stable ---

func TestParse_RoundTrip(t *testing.T) {
	markers := []string{
		`python_version >= "3.8"`,
		`os_name == "posix" and sys_platform == "linux"`,
		`os_name == "posix" or sys_platform == "linux"`,
		`os_name == "posix" or sys_platform == "linux" and extra == "foo"`,
		`os_name == "posix" and sys_platform == "linux" or extra == "foo"`,
		`(os_name == "posix" or sys_platform == "linux") and extra == "foo"`,
		`python_version in "2.7 3.5"`,
		`python_version not in "2.7 3.5"`,
		`extra == "Foo_Bar"`,
		`python_implementation == "CPython"`,
	}

	for _, src := range markers {
		t.Run(src, func(t *testing.T) {
			m1, err := Parse(src)
			require.NoError(t, err)
			s1 := m1.String()

			m2, err := Parse(s1)
			require.NoError(t, err, "re-parsing serialized form %q", s1)
			s2 := m2.String()

			assert.Equal(t, s1, s2, "serialization must be stable across a re-parse")
		})
	}
}
