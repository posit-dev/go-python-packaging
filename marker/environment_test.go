// SPDX-License-Identifier: Apache-2.0 OR MIT

package marker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/posit-dev/go-python-packaging/tags"
)

func testIdentity() InterpreterIdentity {
	return InterpreterIdentity{
		ImplementationName:           "cpython",
		PlatformPythonImplementation: "CPython",
		PythonFullVersion:            "3.11.4",
		ImplementationVersion:        "3.11.4",
	}
}

func TestEnvironmentFromTarget_Linux(t *testing.T) {
	target := tags.Target{OS: "linux", Arch: "x86_64"}
	env, err := EnvironmentFromTarget(target, testIdentity())
	require.NoError(t, err)

	assert.Equal(t, "posix", env.OsName)
	assert.Equal(t, "linux", env.SysPlatform)
	assert.Equal(t, "Linux", env.PlatformSystem)
	assert.Equal(t, "x86_64", env.PlatformMachine)
	assert.Equal(t, "", env.PlatformRelease)
	assert.Equal(t, "", env.PlatformVersion)
}

func TestEnvironmentFromTarget_Linux_ArchAarch64Verbatim(t *testing.T) {
	target := tags.Target{OS: "linux", Arch: "aarch64"}
	env, err := EnvironmentFromTarget(target, testIdentity())
	require.NoError(t, err)

	assert.Equal(t, "aarch64", env.PlatformMachine)
}

func TestEnvironmentFromTarget_Macos(t *testing.T) {
	target := tags.Target{OS: "macos", Arch: "arm64"}
	env, err := EnvironmentFromTarget(target, testIdentity())
	require.NoError(t, err)

	assert.Equal(t, "posix", env.OsName)
	assert.Equal(t, "darwin", env.SysPlatform)
	assert.Equal(t, "Darwin", env.PlatformSystem)
	assert.Equal(t, "arm64", env.PlatformMachine)
	assert.Equal(t, "", env.PlatformRelease)
	assert.Equal(t, "", env.PlatformVersion)
}

func TestEnvironmentFromTarget_Macos_ArchX86_64Verbatim(t *testing.T) {
	target := tags.Target{OS: "macos", Arch: "x86_64"}
	env, err := EnvironmentFromTarget(target, testIdentity())
	require.NoError(t, err)

	assert.Equal(t, "x86_64", env.PlatformMachine)
}

func TestEnvironmentFromTarget_Windows(t *testing.T) {
	target := tags.Target{OS: "windows", Arch: "amd64"}
	env, err := EnvironmentFromTarget(target, testIdentity())
	require.NoError(t, err)

	assert.Equal(t, "nt", env.OsName)
	assert.Equal(t, "win32", env.SysPlatform)
	assert.Equal(t, "Windows", env.PlatformSystem)
	assert.Equal(t, "AMD64", env.PlatformMachine)
	assert.Equal(t, "", env.PlatformRelease)
	assert.Equal(t, "", env.PlatformVersion)
}

func TestEnvironmentFromTarget_Windows_PlatformMachineUppercased(t *testing.T) {
	for _, tc := range []struct {
		arch string
		want string
	}{
		{"amd64", "AMD64"},
		{"arm64", "ARM64"},
		{"x86", "x86"},
	} {
		target := tags.Target{OS: "windows", Arch: tc.arch}
		env, err := EnvironmentFromTarget(target, testIdentity())
		require.NoError(t, err)
		assert.Equal(t, tc.want, env.PlatformMachine, "arch %q", tc.arch)
	}
}

func TestEnvironmentFromTarget_UnsupportedOS_Errors(t *testing.T) {
	target := tags.Target{OS: "solaris", Arch: "x86_64"}
	_, err := EnvironmentFromTarget(target, testIdentity())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnsupportedTarget)
}

func TestEnvironmentFromTarget_PythonVersionDerived(t *testing.T) {
	target := tags.Target{OS: "linux", Arch: "x86_64"}
	id := testIdentity()
	id.PythonFullVersion = "3.11.4"
	env, err := EnvironmentFromTarget(target, id)
	require.NoError(t, err)

	assert.Equal(t, "3.11.4", env.PythonFullVersion)
	assert.Equal(t, "3.11", env.PythonVersion)
}

func TestEnvironmentFromTarget_TrailingPlusRepaired(t *testing.T) {
	target := tags.Target{OS: "linux", Arch: "x86_64"}
	id := testIdentity()
	id.PythonFullVersion = "3.11.0+"
	env, err := EnvironmentFromTarget(target, id)
	require.NoError(t, err)

	assert.Equal(t, "3.11.0+local", env.PythonFullVersion)
	assert.Equal(t, "3.11", env.PythonVersion)
}

func TestEnvironmentFromTarget_MalformedPythonFullVersion_Errors(t *testing.T) {
	target := tags.Target{OS: "linux", Arch: "x86_64"}
	id := testIdentity()
	id.PythonFullVersion = "3"
	_, err := EnvironmentFromTarget(target, id)
	require.Error(t, err)
}

// TestEnvironmentFromTarget_BareTrailingPlus_PythonVersionDerived is the
// RoboRev regression case: repairPythonFullVersion turns a bare trailing "+"
// into "+local" (e.g. "3.11+" -> "3.11+local"), and majorMinor must still
// derive "3.11" from that repaired value rather than returning the raw
// "3.11+local" (as a naive strings.SplitN(full, ".", 3) based derivation
// would).
func TestEnvironmentFromTarget_BareTrailingPlus_PythonVersionDerived(t *testing.T) {
	target := tags.Target{OS: "linux", Arch: "x86_64"}
	id := testIdentity()
	id.PythonFullVersion = "3.11+"
	env, err := EnvironmentFromTarget(target, id)
	require.NoError(t, err)

	assert.Equal(t, "3.11+local", env.PythonFullVersion)
	assert.Equal(t, "3.11", env.PythonVersion)
}

func TestEnvironmentFromTarget_PythonVersion_DiscardsLocalSuffix(t *testing.T) {
	target := tags.Target{OS: "linux", Arch: "x86_64"}
	id := testIdentity()
	id.PythonFullVersion = "3.11.0+local"
	env, err := EnvironmentFromTarget(target, id)
	require.NoError(t, err)

	assert.Equal(t, "3.11", env.PythonVersion)
}

func TestEnvironmentFromTarget_PythonVersion_DiscardsPreReleaseSuffix(t *testing.T) {
	target := tags.Target{OS: "linux", Arch: "x86_64"}
	id := testIdentity()
	id.PythonFullVersion = "3.11.0rc1"
	env, err := EnvironmentFromTarget(target, id)
	require.NoError(t, err)

	assert.Equal(t, "3.11", env.PythonVersion)
}

// An epoch prefix ("<N>!") is valid PEP 440 (though it never appears in a real
// interpreter's python_full_version); majorMinor discards it rather than
// erroring on the version it just validated.
func TestEnvironmentFromTarget_PythonVersion_DiscardsEpoch(t *testing.T) {
	target := tags.Target{OS: "linux", Arch: "x86_64"}
	id := testIdentity()
	id.PythonFullVersion = "1!3.11.4"
	env, err := EnvironmentFromTarget(target, id)
	require.NoError(t, err)

	assert.Equal(t, "3.11", env.PythonVersion)
}

func TestEnvironmentFromTarget_EmptyPythonFullVersion_Errors(t *testing.T) {
	target := tags.Target{OS: "linux", Arch: "x86_64"}
	id := testIdentity()
	id.PythonFullVersion = ""
	_, err := EnvironmentFromTarget(target, id)
	require.Error(t, err)
}

func TestEnvironmentFromTarget_NonNumericPythonFullVersion_Errors(t *testing.T) {
	target := tags.Target{OS: "linux", Arch: "x86_64"}
	id := testIdentity()
	id.PythonFullVersion = "abc"
	_, err := EnvironmentFromTarget(target, id)
	require.Error(t, err)
}

func TestEnvironmentFromTarget_InterpreterFieldsCopied(t *testing.T) {
	target := tags.Target{OS: "linux", Arch: "x86_64"}
	id := InterpreterIdentity{
		ImplementationName:           "pypy",
		PlatformPythonImplementation: "PyPy",
		PythonFullVersion:            "3.10.13",
		ImplementationVersion:        "7.3.16",
	}
	env, err := EnvironmentFromTarget(target, id)
	require.NoError(t, err)

	assert.Equal(t, "pypy", env.ImplementationName)
	assert.Equal(t, "PyPy", env.PlatformPythonImplementation)
	assert.Equal(t, "7.3.16", env.ImplementationVersion)
}
