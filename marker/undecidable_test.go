// SPDX-License-Identifier: Apache-2.0 OR MIT
package marker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// linuxEnv is a declared-target environment: everything a wheel-compatibility
// target can supply, with PlatformRelease and PlatformVersion empty exactly as
// EnvironmentFromTarget leaves them, since a declared target has no kernel.
func linuxEnv() Environment {
	return Environment{
		OsName:                       "posix",
		SysPlatform:                  "linux",
		PlatformSystem:               "Linux",
		PlatformMachine:              "x86_64",
		PlatformRelease:              "",
		PlatformVersion:              "",
		PythonVersion:                "3.13",
		PythonFullVersion:            "3.13.0",
		ImplementationName:           "cpython",
		PlatformPythonImplementation: "CPython",
		ImplementationVersion:        "3.13.0",
	}
}

func TestEvaluateUndecidable_Decidable(t *testing.T) {
	for _, src := range []string{
		`sys_platform == "linux"`,
		`sys_platform != "win32"`,
		`python_version >= "3.11"`,
		`python_full_version < "3.14"`,
		// Ordered comparison on string operands: false, but FAITHFUL to
		// pypa/packaging, whose operator table returns False for < and > and
		// equality for <= and >=. Must not be reported as undecidable.
		`sys_platform >= "darwin"`,
		`platform_system > "Darwin"`,
		// A version-typed operand with a parseable version answers via the
		// specifier path, so even ~= is decidable here.
		`python_version ~= "3.13"`,
		`python_full_version ~= "3.13.0"`,
	} {
		t.Run(src, func(t *testing.T) {
			m, err := Parse(src)
			require.NoError(t, err)
			_, und := m.EvaluateUndecidable(linuxEnv(), nil)
			assert.Empty(t, und, "%s must be decidable", src)
		})
	}
}

func TestEvaluateUndecidable_EmptyEnvVar(t *testing.T) {
	// The case that motivates the API: a declared target cannot know the kernel
	// version, so this comparison is meaningless and must not read as a plain
	// false.
	m, err := Parse(`platform_release >= "5.4"`)
	require.NoError(t, err)

	result, und := m.EvaluateUndecidable(linuxEnv(), nil)
	assert.False(t, result, "the bare answer is still false")
	require.Len(t, und, 1)
	assert.Equal(t, "platform_release", und[0].Var)
	assert.Equal(t, "environment variable is empty", und[0].Reason)
	assert.Contains(t, und[0].Expr, "platform_release")

	// Evaluate must be unchanged by the collection path.
	assert.False(t, m.Evaluate(linuxEnv(), nil))
}

func TestEvaluateUndecidable_UndefinedOperator(t *testing.T) {
	// ~= and === have no string semantics; packaging raises UndefinedComparison.
	// Reaching evalStringOp with them is the divergence this reports.
	for _, src := range []string{
		`sys_platform ~= "linux"`,
		`sys_platform === "linux"`,
	} {
		t.Run(src, func(t *testing.T) {
			m, err := Parse(src)
			require.NoError(t, err)
			result, und := m.EvaluateUndecidable(linuxEnv(), nil)
			assert.False(t, result)
			require.NotEmpty(t, und)
			assert.Equal(t, "operator has no string semantics", und[0].Reason)
		})
	}
}

func TestEvaluateUndecidable_ALiteralEmptyStringIsNotReported(t *testing.T) {
	// The marker author wrote "" deliberately; only an empty ENVIRONMENT value
	// is the caller's gap.
	m, err := Parse(`sys_platform == ""`)
	require.NoError(t, err)
	result, und := m.EvaluateUndecidable(linuxEnv(), nil)
	assert.False(t, result)
	assert.Empty(t, und)
}

func TestEvaluateUndecidable_ShortCircuitLimitsReporting(t *testing.T) {
	// An `and` decided false by its first operand needs no further explanation,
	// so the undecidable second operand is never reached. This keeps the caller
	// from over-including on markers whose answer was already determined.
	m, err := Parse(`sys_platform == "win32" and platform_release >= "5.4"`)
	require.NoError(t, err)
	result, und := m.EvaluateUndecidable(linuxEnv(), nil)
	assert.False(t, result)
	assert.Empty(t, und, "the undecidable operand is unreachable")

	// Reversed, the undecidable operand is reached and reported.
	m2, err := Parse(`platform_release >= "5.4" and sys_platform == "win32"`)
	require.NoError(t, err)
	_, und2 := m2.EvaluateUndecidable(linuxEnv(), nil)
	assert.Len(t, und2, 1)
}

func TestEvaluateUndecidable_EmptyMarker(t *testing.T) {
	var m Marker
	result, und := m.EvaluateUndecidable(linuxEnv(), nil)
	assert.True(t, result)
	assert.Empty(t, und)
}

func TestVariables(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want []string
	}{
		{`sys_platform == "linux"`, []string{"sys_platform"}},
		{`extra == "async"`, []string{"extra"}},
		{
			`python_version >= "3.11" and sys_platform == "linux"`,
			[]string{"python_version", "sys_platform"},
		},
		{
			// First-appearance order, deduplicated.
			`sys_platform == "linux" or (sys_platform == "darwin" and python_version > "3.9")`,
			[]string{"sys_platform", "python_version"},
		},
		{
			// The motivating case: the caller invented PythonFullVersion and needs
			// to know this marker depends on it.
			`python_full_version >= "3.13.2"`,
			[]string{"python_full_version"},
		},
	} {
		t.Run(tc.src, func(t *testing.T) {
			m, err := Parse(tc.src)
			require.NoError(t, err)
			assert.Equal(t, tc.want, m.Variables())
		})
	}
}

func TestVariables_EmptyMarker(t *testing.T) {
	var m Marker
	assert.Nil(t, m.Variables())
}

// TestInventedPatchLevelIsDecidableButArbitrary is the reason Variables exists
// alongside EvaluateUndecidable. Nothing here is empty and no operator is
// undefined, so the comparison is decidable -- and the answer is whatever patch
// level the caller invented. Only Variables surfaces the dependency.
func TestInventedPatchLevelIsDecidableButArbitrary(t *testing.T) {
	m, err := Parse(`python_full_version >= "3.13.2"`)
	require.NoError(t, err)

	low := linuxEnv()
	low.PythonFullVersion = "3.13.0"
	resLow, undLow := m.EvaluateUndecidable(low, nil)

	high := linuxEnv()
	high.PythonFullVersion = "3.13.99"
	resHigh, undHigh := m.EvaluateUndecidable(high, nil)

	assert.False(t, resLow)
	assert.True(t, resHigh, "the answer flips on a value the caller invented")
	assert.Empty(t, undLow, "and neither evaluation is undecidable")
	assert.Empty(t, undHigh)

	assert.Contains(t, m.Variables(), "python_full_version",
		"Variables is the only signal a caller has here")
}
