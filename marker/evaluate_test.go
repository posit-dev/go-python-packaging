// SPDX-License-Identifier: Apache-2.0 OR MIT

package marker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// baseEnv is a representative environment used across the truth table:
// CPython 3.8.5 on 64-bit Linux, mirroring a common packaging test fixture.
func baseEnv() Environment {
	return Environment{
		OsName:                       "posix",
		SysPlatform:                  "linux",
		PlatformMachine:              "x86_64",
		PlatformPythonImplementation: "CPython",
		PlatformRelease:              "5.4.0-42-generic",
		PlatformSystem:               "Linux",
		PlatformVersion:              "#1 SMP",
		PythonVersion:                "3.8",
		PythonFullVersion:            "3.8.5",
		ImplementationName:           "cpython",
		ImplementationVersion:        "3.8.5",
	}
}

func evalStr(t *testing.T, expr string, env Environment, extras []string) bool {
	t.Helper()
	m, err := Parse(expr)
	require.NoError(t, err, "parsing %q", expr)
	return m.Evaluate(env, extras)
}

func TestEvaluate_ZeroValueMarker_AlwaysTrue(t *testing.T) {
	var m Marker
	assert.True(t, m.Evaluate(baseEnv(), nil))
	assert.True(t, m.Evaluate(Environment{}, []string{"anything"}))
}

// --- version-key comparisons delegating to version/ ---

func TestEvaluate_VersionKeyComparisons(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want bool
	}{
		{"python_version >= true", `python_version >= "3.8"`, true},
		{"python_version >= false (higher required)", `python_version >= "3.9"`, false},
		{"python_version > false (equal)", `python_version > "3.8"`, false},
		{"python_version < true", `python_version < "3.9"`, true},
		{"python_full_version == true", `python_full_version == "3.8.5"`, true},
		{"python_full_version == false", `python_full_version == "3.8.6"`, false},
		{"reversed operand order, true", `"3.8" <= python_version`, true},
		{"reversed operand order, false", `"3.9" <= python_version`, false},
		{"implementation_version ==", `implementation_version == "3.8.5"`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, evalStr(t, tt.expr, baseEnv(), nil))
		})
	}
}

// --- the `python_version in "2.6 2.7 3.2"` substring idiom ---

func TestEvaluate_VersionKey_InSubstringIdiom(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want bool
	}{
		{"in, present", `python_version in "2.6 2.7 3.2 3.8"`, true},
		{"in, absent", `python_version in "2.6 2.7 3.2"`, false},
		{"not in, present", `python_version not in "2.6 2.7 3.2 3.8"`, false},
		{"not in, absent", `python_version not in "2.6 2.7 3.2"`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, evalStr(t, tt.expr, baseEnv(), nil))
		})
	}
}

// --- string-op quirks on a non-version key ---

func TestEvaluate_StringOpQuirks_NonVersionKey(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want bool
	}{
		{"< always false", `sys_platform < "zzzz"`, false},
		{"< always false (reverse would be true lexicographically)", `sys_platform < "aaaa"`, false},
		{"> always false", `sys_platform > "aaaa"`, false},
		{"> always false (equal case too)", `sys_platform > "linux"`, false},
		{"<= treated as ==, true", `sys_platform <= "linux"`, true},
		{"<= treated as ==, false", `sys_platform <= "darwin"`, false},
		{">= treated as ==, true", `sys_platform >= "linux"`, true},
		{">= treated as ==, false", `sys_platform >= "darwin"`, false},
		{"== true", `sys_platform == "linux"`, true},
		{"== false", `sys_platform == "darwin"`, false},
		{"!= true", `sys_platform != "darwin"`, true},
		{"!= false", `sys_platform != "linux"`, false},
		{"in substring true", `sys_platform in "the linux platform"`, true},
		{"in substring false", `sys_platform in "the darwin platform"`, false},
		{"not in substring true", `sys_platform not in "the darwin platform"`, true},
		{"not in substring false", `sys_platform not in "the linux platform"`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, evalStr(t, tt.expr, baseEnv(), nil))
		})
	}
}

// `~=`/`===` on a non-version string field: no bool-returning entry in
// the fallback string-operator table (packaging raises here; our fixed bool
// signature can't propagate an error, so we return false - a deliberate,
// documented divergence). See evaluate.go and the task report.
func TestEvaluate_TildeEqualAndArbitraryEqual_NonVersionKey_ReturnsFalse(t *testing.T) {
	assert.False(t, evalStr(t, `sys_platform ~= "linux"`, baseEnv(), nil))
	assert.False(t, evalStr(t, `sys_platform === "linux"`, baseEnv(), nil))
}

// On a version-typed key, ~= and === are valid PEP 440 specifier operators
// and are handled by the version-comparison attempt (rule 1), not the
// string fallback table - so they behave normally there, not as "false".
func TestEvaluate_TildeEqualAndArbitraryEqual_VersionKey_DelegatesToVersion(t *testing.T) {
	assert.True(t, evalStr(t, `python_version ~= "3.8"`, baseEnv(), nil))
	assert.False(t, evalStr(t, `python_version ~= "3.9"`, baseEnv(), nil))
	assert.True(t, evalStr(t, `python_full_version === "3.8.5"`, baseEnv(), nil))
	assert.False(t, evalStr(t, `python_full_version === "3.8.6"`, baseEnv(), nil))
}

// --- platform_release: version-typed with string fallback ---

func TestEvaluate_PlatformRelease_VersionTypedWithStringFallback(t *testing.T) {
	// A version-looking platform_release compares as a version.
	env := baseEnv()
	env.PlatformRelease = "5.4.0"
	assert.True(t, evalStr(t, `platform_release >= "5.0.0"`, env, nil))
	assert.False(t, evalStr(t, `platform_release >= "6.0.0"`, env, nil))

	// An empty platform_release (target-derived env) falls through to
	// string ops rather than panicking: version.Parse("") errors.
	env.PlatformRelease = ""
	assert.NotPanics(t, func() {
		evalStr(t, `platform_release == ""`, env, nil)
	})
	assert.True(t, evalStr(t, `platform_release == ""`, env, nil))
	assert.False(t, evalStr(t, `platform_release == "5.4.0"`, env, nil))
	// >= falls back to string-equality semantics per rule 2 when the
	// version attempt fails (empty string is never a valid PEP 440 version).
	assert.False(t, evalStr(t, `platform_release >= "5.0.0"`, env, nil))
}

// --- extra set-membership ---

func TestEvaluate_Extra_SetMembership(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		active []string
		want   bool
	}{
		{"== present", `extra == "foo"`, []string{"foo"}, true},
		{"== absent", `extra == "foo"`, []string{"bar"}, false},
		{"== no active extras", `extra == "foo"`, nil, false},
		{"!= present (false)", `extra != "foo"`, []string{"foo"}, false},
		{"!= absent (true)", `extra != "foo"`, []string{"bar"}, true},
		{"reversed operand order", `"foo" == extra`, []string{"foo"}, true},
		// normalization: literal already normalized at parse time
		// ("Foo_Bar" -> "foo-bar"); active extras normalized at eval time.
		{"normalization, literal side", `extra == "Foo_Bar"`, []string{"foo-bar"}, true},
		{"normalization, active side", `extra == "foo-bar"`, []string{"Foo_Bar"}, true},
		{"normalization, both sides", `extra == "Foo.Bar"`, []string{"foo_bar"}, true},
		// Only == and != are meaningful for extra set-membership; any other
		// operator is inapplicable and evaluates false, even when the extra
		// named on the value side is active.
		{"< present (inapplicable op)", `extra < "foo"`, []string{"foo"}, false},
		{"in present (inapplicable op)", `extra in "foo"`, []string{"foo"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, evalStr(t, tt.expr, baseEnv(), tt.active))
		})
	}
}

// The documented set-membership divergence from packaging: packaging's
// singular `extra` makes `extra == "a" and extra == "b"` unsatisfiable,
// but our active-extras-as-a-set model makes it true when both are active.
func TestEvaluate_Extra_ANDCombinedDistinctExtras_Divergence(t *testing.T) {
	expr := `extra == "a" and extra == "b"`
	assert.True(t, evalStr(t, expr, baseEnv(), []string{"a", "b"}))
	assert.False(t, evalStr(t, expr, baseEnv(), []string{"a"}))
	assert.False(t, evalStr(t, expr, baseEnv(), nil))
}

// --- empty-string LHS on a version key: no panic ---

func TestEvaluate_EmptyStringLHS_OnVersionKey_NoPanic(t *testing.T) {
	env := baseEnv()
	env.PlatformRelease = ""
	assert.NotPanics(t, func() {
		evalStr(t, `platform_release != "5.4.0"`, env, nil)
	})
	assert.True(t, evalStr(t, `platform_release != "5.4.0"`, env, nil))
}

// --- and/or tree evaluation and nesting ---

func TestEvaluate_BoolTree(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want bool
	}{
		{"and, both true", `os_name == "posix" and sys_platform == "linux"`, true},
		{"and, one false", `os_name == "posix" and sys_platform == "darwin"`, false},
		{"or, one true", `os_name == "nt" or sys_platform == "linux"`, true},
		{"or, both false", `os_name == "nt" or sys_platform == "darwin"`, false},
		{
			"and binds tighter than or, right group true",
			`os_name == "nt" or sys_platform == "linux" and platform_system == "Linux"`,
			true,
		},
		{
			"and binds tighter than or, right group false",
			`os_name == "nt" or sys_platform == "linux" and platform_system == "Darwin"`,
			false,
		},
		{
			"parens override precedence, true",
			`(os_name == "nt" or sys_platform == "linux") and platform_system == "Linux"`,
			true,
		},
		{
			"parens override precedence, false",
			`(os_name == "nt" or sys_platform == "darwin") and platform_system == "Linux"`,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, evalStr(t, tt.expr, baseEnv(), nil))
		})
	}
}

// --- env field mapping coverage (all 11 variables) ---

func TestEvaluate_AllEnvVarsResolve(t *testing.T) {
	env := baseEnv()
	tests := []string{
		`os_name == "posix"`,
		`sys_platform == "linux"`,
		`platform_machine == "x86_64"`,
		`platform_python_implementation == "CPython"`,
		`platform_release == "5.4.0-42-generic"`,
		`platform_system == "Linux"`,
		`platform_version == "#1 SMP"`,
		`python_version == "3.8"`,
		`python_full_version == "3.8.5"`,
		`implementation_name == "cpython"`,
		`implementation_version == "3.8.5"`,
	}
	for _, expr := range tests {
		t.Run(expr, func(t *testing.T) {
			assert.True(t, evalStr(t, expr, env, nil))
		})
	}
}

// python_implementation is a legacy alias for
// platform_python_implementation, folded at parse time - Evaluate never
// sees the alias name at all, but this pins the end-to-end behavior.
func TestEvaluate_PythonImplementationAlias(t *testing.T) {
	assert.True(t, evalStr(t, `python_implementation == "CPython"`, baseEnv(), nil))
}
