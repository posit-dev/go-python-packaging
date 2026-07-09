// SPDX-License-Identifier: Apache-2.0 OR MIT

package marker

import (
	"errors"
	"fmt"
	"strings"

	"github.com/posit-dev/go-python-packaging/tags"
	"github.com/posit-dev/go-python-packaging/version"
)

// ErrUnsupportedTarget is returned by EnvironmentFromTarget when the given
// tags.Target names an OS this package does not know how to map to PEP 508
// marker variables.
var ErrUnsupportedTarget = errors.New("unsupported target for marker environment")

// Environment holds the 11 PEP 508 marker variables for one concrete
// target. Field names correspond to PEP 508 variable names via snake_case
// (e.g. OsName -> os_name); the Evaluate method (a later task) maps between
// them.
type Environment struct {
	OsName                       string // "posix", "nt"
	SysPlatform                  string // "linux", "darwin", "win32"
	PlatformMachine              string // "x86_64", "aarch64", "arm64", "AMD64" (win), ...
	PlatformPythonImplementation string // "CPython", "PyPy"
	PlatformRelease              string // "" when target-derived
	PlatformSystem               string // "Linux", "Darwin", "Windows"
	PlatformVersion              string // "" when target-derived
	PythonVersion                string // "3.11" (major.minor)
	PythonFullVersion            string // "3.11.4"
	ImplementationName           string // "cpython", "pypy"
	ImplementationVersion        string // "3.11.4"
}

// InterpreterIdentity carries the marker fields a tags.Target cannot derive:
// the concrete interpreter identity and its full version.
// tags.Target.Implementation is only "cp"|"py" - a wheel-compatibility
// class, not a runtime interpreter identity - so the caller (which knows
// the real target interpreter) supplies these explicitly.
type InterpreterIdentity struct {
	ImplementationName           string // "cpython", "pypy"
	PlatformPythonImplementation string // "CPython", "PyPy"
	PythonFullVersion            string // "3.11.4" (trailing-'+' repaired; see below)
	ImplementationVersion        string // "3.11.4"
}

// EnvironmentFromTarget builds an Environment for t, filling the platform
// fields (OsName, SysPlatform, PlatformSystem, PlatformMachine,
// PlatformRelease, PlatformVersion) from t and the interpreter-identity
// fields from id.
//
// The platform mapping follows Astral uv's uv-configuration TargetTriple
// (os_name/sys_platform/platform_system/platform_release/platform_version):
// linux -> posix/linux/Linux; macos -> posix/darwin/Darwin; windows ->
// nt/win32/Windows; PlatformRelease and PlatformVersion are "" in all three
// cases, since they are target-derived rather than observed from a live
// process. An unsupported t.OS returns ErrUnsupportedTarget.
//
// PlatformMachine forwards t.Arch verbatim for linux/macos, where
// tags.Target's arch strings ("x86_64", "aarch64", "arm64", ...) already
// match real platform.machine() output. Windows is special-cased instead of
// blindly upper-cased: tags.Target's windows arches are "amd64"/"arm64"/
// "x86", and real Windows platform.machine() values (sourced from the
// PROCESSOR_ARCHITECTURE environment variable) are "AMD64"/"ARM64"/"x86" -
// note "x86" is NOT upper-cased. This deliberately diverges from uv's own
// TargetTriple::platform_machine(), which reports "x86_64" (not "AMD64")
// for its windows/x86_64 triples; we favor fidelity to real-world PEP 508
// markers actually written against platform_machine == 'AMD64' (as seen in
// uv's own MarkerEnvironment test fixtures) over matching uv's Rust-target-
// style naming.
//
// PythonFullVersion is repaired per packaging's
// _pep440_python_full_version/_repair_python_full_version: a bare trailing
// "+" (as reported by some patched/dirty CPython builds) is not a valid PEP
// 440 local version, so it becomes "+local" (e.g. "3.11.0+" ->
// "3.11.0+local") before being stored, so a later version.Parse of it does
// not fail. PythonVersion is then derived as the major.minor prefix of the
// repaired PythonFullVersion (e.g. "3.11.4" -> "3.11"); if that cannot be
// derived, EnvironmentFromTarget returns an error.
func EnvironmentFromTarget(t tags.Target, id InterpreterIdentity) (Environment, error) {
	var env Environment

	switch t.OS {
	case "linux":
		env.OsName = "posix"
		env.SysPlatform = "linux"
		env.PlatformSystem = "Linux"
		env.PlatformMachine = t.Arch
	case "macos":
		env.OsName = "posix"
		env.SysPlatform = "darwin"
		env.PlatformSystem = "Darwin"
		env.PlatformMachine = t.Arch
	case "windows":
		env.OsName = "nt"
		env.SysPlatform = "win32"
		env.PlatformSystem = "Windows"
		env.PlatformMachine = windowsPlatformMachine(t.Arch)
	default:
		return Environment{}, fmt.Errorf("%w: unsupported OS %q", ErrUnsupportedTarget, t.OS)
	}
	env.PlatformRelease = ""
	env.PlatformVersion = ""

	env.PlatformPythonImplementation = id.PlatformPythonImplementation
	env.ImplementationName = id.ImplementationName
	env.ImplementationVersion = id.ImplementationVersion
	env.PythonFullVersion = repairPythonFullVersion(id.PythonFullVersion)

	pythonVersion, err := majorMinor(env.PythonFullVersion)
	if err != nil {
		return Environment{}, err
	}
	env.PythonVersion = pythonVersion

	return env, nil
}

// windowsPlatformMachine maps a tags.Target windows arch ("amd64", "arm64",
// "x86") to the value real Windows Python reports for platform.machine()
// ("AMD64", "ARM64", "x86" - note "x86" stays lower-case). EnvironmentFromTarget
// expects callers to supply a validated Target, so the default case (a plain
// upper-case fallback) only matters for an out-of-band arch value.
func windowsPlatformMachine(arch string) string {
	switch arch {
	case "amd64":
		return "AMD64"
	case "arm64":
		return "ARM64"
	case "x86":
		return "x86"
	default:
		return strings.ToUpper(arch)
	}
}

// repairPythonFullVersion applies packaging's
// _pep440_python_full_version repair: a python_full_version ending in a
// bare "+" is not a valid PEP 440 version (a local version segment requires
// content after "+"), so "local" is appended.
func repairPythonFullVersion(full string) string {
	if strings.HasSuffix(full, "+") {
		return full + "local"
	}
	return full
}

// majorMinor extracts the "major.minor" prefix of a python_full_version
// string such as "3.11.4" -> "3.11". The derivation is based only on the
// leading numeric release segment of full - any pre/post/dev/local suffix
// (e.g. the "+local" appended by repairPythonFullVersion for a bare
// trailing "+", or "rc1", ".post1", ".dev0") is discarded rather than
// naively split on ".", so "3.11+local" correctly yields "3.11" instead of
// the bogus "3.11+local". full is first validated as a real PEP 440 version
// via version.Parse; an error is returned for malformed input (e.g. "" or
// "abc") or if the release segment has fewer than two dot-separated,
// non-empty numeric components (e.g. "3" alone).
func majorMinor(full string) (string, error) {
	if _, err := version.Parse(full); err != nil {
		return "", fmt.Errorf("marker: cannot derive python_version from python_full_version %q: %w", full, err)
	}

	// Cut full at the first rune that is not part of the release segment
	// (i.e. not a digit or "."), discarding any epoch marker ("!") and any
	// pre/post/dev/local suffix.
	end := strings.IndexFunc(full, func(r rune) bool {
		return (r < '0' || r > '9') && r != '.'
	})
	release := full
	if end >= 0 {
		release = full[:end]
	}

	parts := strings.SplitN(release, ".", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("marker: cannot derive python_version from python_full_version %q", full)
	}
	return parts[0] + "." + parts[1], nil
}
