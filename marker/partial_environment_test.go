// SPDX-License-Identifier: Apache-2.0 OR MIT

// Tests for the partial-marker-environment override seam added for
// rstudio/package-manager#19383.

package marker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// baseEnvironment is a fully-populated environment to override onto, standing
// in for what upstream gets from the live interpreter.
func baseEnvironment() Environment {
	return Environment{
		OsName:                       "posix",
		SysPlatform:                  "linux",
		PlatformMachine:              "x86_64",
		PlatformPythonImplementation: "CPython",
		PlatformRelease:              "6.1.0",
		PlatformSystem:               "Linux",
		PlatformVersion:              "#1 SMP",
		PythonVersion:                "3.11",
		PythonFullVersion:            "3.11.4",
		ImplementationName:           "cpython",
		ImplementationVersion:        "3.11.4",
	}
}

// ⚠️ THE reason this seam exists. Upstream's Marker.evaluate takes a PARTIAL
// dict and keeps the live interpreter's value for every unmentioned key. The
// naive Go translation is a struct literal, which zero-fills instead -- so
// `python_version` silently becomes "" and a marker that mentions it changes
// its answer with no error reported.
//
// The two halves of this test are the same override expressed both ways, and
// they must NOT agree.
func TestStructLiteralZeroFillIsNotAPartialEnvironment(t *testing.T) {
	m, err := Parse(`os_name == "foo" and python_version >= "3.8"`)
	require.NoError(t, err)

	// The zero-filling trap: python_version is "", so the marker is false.
	naive := Environment{OsName: "foo"}
	assert.False(t, m.Evaluate(naive, nil),
		"a struct literal zero-fills python_version, changing the answer")

	// The override seam: everything the caller did not mention is preserved.
	partial, err := baseEnvironment().With(map[string]string{"os_name": "foo"})
	require.NoError(t, err)
	assert.True(t, m.Evaluate(partial, nil),
		"With must preserve the base value for unmentioned variables")
}

// Every one of the 11 PEP 508 variables must be settable by name and readable
// back. This is a bijection check across VariableNames, With's switch and
// envVarValue's switch: the three are separate switches on the same names, and
// the failure mode of drift is a variable that silently cannot be overridden.
func TestWithAndLookupCoverEveryMarkerVariable(t *testing.T) {
	names := VariableNames()
	assert.Len(t, names, 11, "PEP 508 defines 11 marker variables")

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			const sentinel = "OVERRIDDEN"

			env, err := baseEnvironment().With(map[string]string{name: sentinel})
			require.NoError(t, err)

			got, err := env.Lookup(name)
			require.NoError(t, err)
			assert.Equal(t, sentinel, got, "With must set the variable Lookup reads")

			// No other variable may have moved.
			for _, other := range names {
				if other == name {
					continue
				}
				want, err := baseEnvironment().Lookup(other)
				require.NoError(t, err)
				actual, err := env.Lookup(other)
				require.NoError(t, err)
				assert.Equal(t, want, actual, "overriding %q must not touch %q", name, other)
			}
		})
	}
}

// An override actually reaches evaluation for every variable, not just Lookup.
func TestOverrideReachesEvaluation(t *testing.T) {
	for _, name := range VariableNames() {
		t.Run(name, func(t *testing.T) {
			// A value that no base field holds, and that is not version-shaped,
			// so the comparison takes the string path uniformly.
			const sentinel = "zzsentinelzz"

			m, err := Parse(name + ` == "` + sentinel + `"`)
			require.NoError(t, err)

			base := baseEnvironment()
			assert.False(t, m.Evaluate(base, nil), "base must not already match")

			env, err := base.With(map[string]string{name: sentinel})
			require.NoError(t, err)
			assert.True(t, m.Evaluate(env, nil), "override must reach evaluation")
		})
	}
}

// A typo in a variable name must be an error, not a silent no-op that
// evaluates against the base value and looks like it worked.
func TestWithRejectsUnknownVariable(t *testing.T) {
	for _, name := range []string{"", "os-name", "osname", "OS_NAME", "extra", "platform_machin"} {
		t.Run(name, func(t *testing.T) {
			_, err := baseEnvironment().With(map[string]string{name: "x"})
			assert.ErrorIs(t, err, ErrUnknownMarkerVariable)
		})
	}
}

// `extra` is deliberately NOT a marker-environment variable: it is bound from
// the active-extras list passed to Evaluate, not from the environment.
func TestExtraIsNotAnEnvironmentVariable(t *testing.T) {
	assert.NotContains(t, VariableNames(), "extra")

	_, err := baseEnvironment().Lookup("extra")
	assert.ErrorIs(t, err, ErrUnknownMarkerVariable)

	m, err := Parse(`extra == "docs"`)
	require.NoError(t, err)
	assert.False(t, m.Evaluate(baseEnvironment(), nil))
	assert.True(t, m.Evaluate(baseEnvironment(), []string{"docs"}))
}

// With must not mutate its receiver, and must tolerate an empty or nil map.
func TestWithDoesNotMutateReceiver(t *testing.T) {
	base := baseEnvironment()

	derived, err := base.With(map[string]string{"os_name": "nt", "sys_platform": "win32"})
	require.NoError(t, err)

	assert.Equal(t, baseEnvironment(), base, "receiver must be unchanged")
	assert.Equal(t, "nt", derived.OsName)
	assert.Equal(t, "win32", derived.SysPlatform)

	same, err := base.With(nil)
	require.NoError(t, err)
	assert.Equal(t, base, same)

	same, err = base.With(map[string]string{})
	require.NoError(t, err)
	assert.Equal(t, base, same)
}

// Overriding to the empty string is a real override, distinguishable from
// leaving a variable unmentioned. That distinction is the whole point of the
// seam, so pin it.
func TestOverrideToEmptyStringIsDistinctFromUnmentioned(t *testing.T) {
	m, err := Parse(`platform_release == ""`)
	require.NoError(t, err)

	base := baseEnvironment()
	assert.False(t, m.Evaluate(base, nil))

	cleared, err := base.With(map[string]string{"platform_release": ""})
	require.NoError(t, err)
	assert.True(t, m.Evaluate(cleared, nil))

	// And the unmentioned variables are still their base values, not "".
	assert.Equal(t, "3.11", cleared.PythonVersion)
}
