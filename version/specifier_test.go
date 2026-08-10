// SPDX-License-Identifier: Apache-2.0 OR MIT

package version

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConstraints(t *testing.T) {
	tests := []struct {
		constraint string
		wantErr    bool
	}{
		// https://github.com/pypa/packaging/blob/28d2fa0742747cda4bc4530b2a5bc919b7382039/tests/test_specifiers.py#L31-L42
		{"~=2.0", false},
		{"==2.1.*", false},
		{"==2.1.0.3", false},
		{"!=2.2.*", false},
		{"!=2.2.0.5", false},
		{"<=5", false},
		{">=7.9a1", false},
		{"<1.0.dev1", false},
		{">2.0.post1", false},

		{"===lolwat", false},

		// https://github.com/pypa/packaging/blob/28d2fa0742747cda4bc4530b2a5bc919b7382039/tests/test_specifiers.py#L50-L86
		// Operator-less specifier
		{"2.0", false}, // go-pep-440-version permits this case

		// Invalid operator
		{"=>2.0", true},

		//Version-less specifier
		{"==", true},

		// Local segment on operators which don't support them
		{"~=1.0+5", true},
		{">=1.0+deadbeef", true},
		{"<=1.0+abc123", true},
		{">1.0+watwat", true},
		{"<1.0+1.0", true},

		// Prefix matching on operators which don't support them
		{"~=1.0.*", true},
		{">=1.0.*", true},
		{"<=1.0.*", true},
		{">1.0.*", true},
		{"<1.0.*", true},

		// Combination of local and prefix matching on operators which do
		// support one or the other
		{"==1.0.*+5", true},
		{"!=1.0.*+deadbeef", true},

		// Prefix matching cannot be used inside of a local version
		{"==1.0+5.*", true},
		{"!=1.0+deadbeef.*", true},

		// Prefix matching must appear at the end
		{"==1.0.*.5", true},

		// Compatible operator requires 2 digits in the release operator
		{"~=1", true},

		// Cannot use a prefix matching after a .devN version
		{"==1.0.dev1.*", true},
		{"!=1.0.dev1.*", true},

		// Versions with dashes '-' should return an error
		{"== 3.99-0.14", true},
		{"==3.99-0.14", true},
		{"==3.99.0.14", false},
		{"== 3.99.0.14", false},
	}
	for _, tt := range tests {
		t.Run(tt.constraint, func(t *testing.T) {
			_, err := NewSpecifiers(tt.constraint)
			if tt.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewRSpecifiers(t *testing.T) {
	tests := []struct {
		constraint string
		wantErr    bool
	}{
		// https://github.com/pypa/packaging/blob/28d2fa0742747cda4bc4530b2a5bc919b7382039/tests/test_specifiers.py#L31-L42
		{"~=2.0", false},
		{"==2.1.*", false},
		{"==2.1.0.3", false},
		{"!=2.2.*", false},
		{"!=2.2.0.5", false},
		{"<=5", false},
		{">=7.9a1", false},
		{"<1.0.dev1", false},
		{">2.0.post1", false},

		{"===lolwat", false},

		// https://github.com/pypa/packaging/blob/28d2fa0742747cda4bc4530b2a5bc919b7382039/tests/test_specifiers.py#L50-L86
		// Operator-less specifier
		{"2.0", false}, // go-pep-440-version permits this case

		// Invalid operator
		{"=>2.0", true},

		//Version-less specifier
		{"==", true},

		// Local segment on operators which don't support them
		{"~=1.0+5", true},
		{">=1.0+deadbeef", true},
		{"<=1.0+abc123", true},
		{">1.0+watwat", true},
		{"<1.0+1.0", true},

		// Prefix matching on operators which don't support them
		{"~=1.0.*", true},
		{">=1.0.*", true},
		{"<=1.0.*", true},
		{">1.0.*", true},
		{"<1.0.*", true},

		// Combination of local and prefix matching on operators which do
		// support one or the other
		{"==1.0.*+5", true},
		{"!=1.0.*+deadbeef", true},

		// Prefix matching cannot be used inside of a local version
		{"==1.0+5.*", true},
		{"!=1.0+deadbeef.*", true},

		// Prefix matching must appear at the end
		{"==1.0.*.5", true},

		// Compatible operator requires 2 digits in the release operator
		{"~=1", true},

		// Cannot use a prefix matching after a .devN version
		{"==1.0.dev1.*", true},
		{"!=1.0.dev1.*", true},

		// Versions containing dashes '-' should not return an error
		{"== 3.99-0.14", false},
		{"==3.99-0.14", false},
		{"==3.99.0.14", false},
		{"== 3.99.0.14", false},
	}
	for _, tt := range tests {
		t.Run(tt.constraint, func(t *testing.T) {
			_, err := NewRSpecifiers(tt.constraint, func(s string) string { return s })
			if tt.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVersion_Check(t *testing.T) {
	tests := []struct {
		version string
		spec    string
		want    bool
	}{
		// Test the equality operation
		{"2.0", "==2", true},
		{"2.0", "==2.0", true},
		{"2.0", "==2.0.0", true},
		{"2.0+deadbeef", "==2", true},
		{"2.0+deadbeef", "==2.0", true},
		{"2.0+deadbeef", "==2.0.0", true},
		{"2.0+deadbeef", "==2+deadbeef", true},
		{"2.0+deadbeef", "==2.0+deadbeef", true},
		{"2.0+deadbeef", "==2.0.0+deadbeef", true},
		{"2.0+deadbeef.0", "==2.0.0+deadbeef.00", true},

		// Test the equality operation with a prefix
		{"2.dev1", "==2.*", true},
		{"2a1", "==2.*", true},
		{"2a1.post1", "==2.*", true},
		{"2b1", "==2.*", true},
		{"2b1.dev1", "==2.*", true},
		{"2c1", "==2.*", true},
		{"2c1.post1.dev1", "==2.*", true},
		{"2rc1", "==2.*", true},
		{"2", "==2.*", true},
		{"2.0", "==2.*", true},
		{"2.0.0", "==2.*", true},
		{"2.1+local.version", "==2.1.*", true},

		// Test the in-equality operation
		{"2.1", "!=2", true},
		{"2.1", "!=2.0", true},
		{"2.0.1", "!=2", true},
		{"2.0.1", "!=2.0", true},
		{"2.0.1", "!=2.0.0", true},
		{"2.0", "!=2.0+deadbeef", true},

		// Test the in-equality operation with a prefix
		{"2.0", "!=3.*", true},
		{"2.1", "!=2.0.*", true},

		// Test the greater than equal operation
		{"2.0", ">=2", true},
		{"2.0", ">=2.0", true},
		{"2.0", ">=2.0.0", true},
		{"2.0.post1", ">=2", true},
		{"2.0.post1.dev1", ">=2", true},
		{"3", ">=2", true},

		// Test the less than equal operation
		{"2.0", "<=2", true},
		{"2.0", "<=2.0", true},
		{"2.0", "<=2.0.0", true},
		{"2.0.dev1", "<=2", true},
		{"2.0a1", "<=2", true},
		{"2.0a1.dev1", "<=2", true},
		{"2.0b1", "<=2", true},
		{"2.0b1.post1", "<=2", true},
		{"2.0c1", "<=2", true},
		{"2.0c1.post1.dev1", "<=2", true},
		{"2.0rc1", "<=2", true},
		{"1", "<=2", true},

		// Test the greater than operation
		{"3", ">2", true},
		{"2.1", ">2.0", true},
		{"2.0.1", ">2", true},
		{"2.1.post1", ">2", true},
		{"2.1+local.version", ">2", true},

		// Test the less than operation
		{"1", "<2", true},
		{"2.0", "<2.1", true},
		{"2.0.dev0", "<2.1", true},

		// Test the compatibility operation
		{"1", "~=1.0", true},
		{"1.0.1", "~=1.0", true},
		{"1.1", "~=1.0", true},
		{"1.9999999", "~=1.0", true},

		// Test that epochs are handled sanely
		{"2!1.0", "~=2!1.0", true},
		{"2!1.0", "==2!1.*", true},
		{"2!1.0", "==2!1.0", true},
		{"2!1.0", "!=1.0", true},
		{"1.0", "!=2!1.0", true},
		{"1.0", "<=2!0.1", true},
		{"2!1.0", ">=2.0", true},
		{"1.0", "<2!0.1", true},
		{"2!1.0", ">2.0", true},

		// Test some normalization rules
		{"2.0.5", ">2.0dev", true},

		// Test the equality operation
		{"2.1", "==2", false},
		{"2.1", "==2.0", false},
		{"2.1", "==2.0.0", false},
		{"2.0", "==2.0+deadbeef", false},

		//Test the equality operation with a prefix
		{"2.0", "==3.*", false},
		{"2.1", "==2.0.*", false},

		// Test the in-equality operation
		{"2.0", "!=2", false},
		{"2.0", "!=2.0", false},
		{"2.0", "!=2.0.0", false},
		{"2.0+deadbeef", "!=2", false},
		{"2.0+deadbeef", "!=2.0", false},
		{"2.0+deadbeef", "!=2.0.0", false},
		{"2.0+deadbeef", "!=2+deadbeef", false},
		{"2.0+deadbeef", "!=2.0+deadbeef", false},
		{"2.0+deadbeef", "!=2.0.0+deadbeef", false},
		{"2.0+deadbeef.0", "!=2.0.0+deadbeef.00", false},

		// Test the in-equality operation with a prefix
		{"2.dev1", "!=2.*", false},
		{"2a1", "!=2.*", false},
		{"2a1.post1", "!=2.*", false},
		{"2b1", "!=2.*", false},
		{"2b1.dev1", "!=2.*", false},
		{"2c1", "!=2.*", false},
		{"2c1.post1.dev1", "!=2.*", false},
		{"2rc1", "!=2.*", false},
		{"2", "!=2.*", false},
		{"2.0", "!=2.*", false},
		{"2.0.0", "!=2.*", false},

		//Test the greater than equal operation
		{"2.0.dev1", ">=2", false},
		{"2.0a1", ">=2", false},
		{"2.0a1.dev1", ">=2", false},
		{"2.0b1", ">=2", false},
		{"2.0b1.post1", ">=2", false},
		{"2.0c1", ">=2", false},
		{"2.0c1.post1.dev1", ">=2", false},
		{"2.0rc1", ">=2", false},
		{"1", ">=2", false},

		// Test the less than equal operation
		{"2.0.post1", "<=2", false},
		{"2.0.post1.dev1", "<=2", false},
		{"3", "<=2", false},

		// Test the greater than operation
		{"1", ">2", false},
		{"2.0.dev1", ">2", false},
		{"2.0a1", ">2", false},
		{"2.0a1.post1", ">2", false},
		{"2.0b1", ">2", false},
		{"2.0b1.dev1", ">2", false},
		{"2.0c1", ">2", false},
		{"2.0c1.post1.dev1", ">2", false},
		{"2.0rc1", ">2", false},
		{"2.0", ">2", false},
		{"2.0.post1", ">2", false},
		{"2.0.post1.dev1", ">2", false},
		{"2.0+local.version", ">2", false},

		// Test the less than operation
		{"2.0.dev1", "<2", false},
		{"2.0a1", "<2", false},
		{"2.0a1.post1", "<2", false},
		{"2.0b1", "<2", false},
		{"2.0b2.dev1", "<2", false},
		{"2.0c1", "<2", false},
		{"2.0c1.post1.dev1", "<2", false},
		{"2.0rc1", "<2", false},
		{"2.0", "<2", false},
		{"2.post1", "<2", false},
		{"2.post1.dev1", "<2", false},
		{"3", "<2", false},

		// Test the compatibility operation
		{"2.0", "~=1.0", false},
		{"1.1.0", "~=1.0.0", false},
		{"1.1.post1", "~=1.0.0", false},

		// Test that epochs are handled sanely
		{"1.0", "~=2!1.0", false},
		{"2!1.0", "~=1.0", false},
		{"2!1.0", "==1.0", false},
		{"1.0", "==2!1.0", false},
		{"2!1.0", "==1.*", false},
		{"1.0", "==2!1.*", false},
		{"2!1.0", "!=2!1.0", false},

		// local versions
		{"1.0.0+local", "==1.0.0", true},
		{"1.0.0+local", "!=1.0.0", false},
		{"1.0.0+local", "<=1.0.0", true},
		{"1.0.0+local", ">=1.0.0", true},
		{"1.0.0+local", "<1.0.0", false},
		{"1.0.0+local", ">1.0.0", false},

		// and operators
		{"1.0", ">= 1.0, != 1.3.4.*, < 2.0", true},
		{"1.0", "~= 0.9, >= 1.0, != 1.3.4.*, < 2.0", false},
		{"0.9", "~= 0.9, != 1.3.4.*, < 2.0", true},
		{"2.0", ">= 1.0, != 1.3.4.*, < 2.0", false},
		{"1.3.4", ">= 1.0, != 1.3.4.*, < 2.0", false},

		// or operators
		{"1.0", "~= 0.9, >= 1.0, != 1.3.4.*, < 2.0 || ==1.0", true},
		{"1.0", "~= 0.9, >= 1.0, != 1.3.4.*, < 2.0 || !=1.0", false},

		// Test the equality operation not defined in PEP 440
		{"2.0", "2", true},
		{"2.0", "2.0", true},
		{"2.0", "2.0.0", true},
		{"2.1", "2", false},
		{"2.1", "2.0", false},
		{"2.1", "2.0.0", false},
		{"2.0", "2.0+deadbeef", false},
		{"2.0", "=2", true},
		{"2.0", "=2.0", true},
		{"2.0", "=2.0.0", true},
		{"2.1", "=2", false},
		{"2.1", "=2.0", false},
		{"2.1", "=2.0.0", false},
		{"2.0", "=2.0+deadbeef", false},
		{"2.0", "*", true},

		// Comma-separated, which is what PEP 440 writes and what upstream
		// accepts. These five cases were previously written SPACE-separated
		// (">= 1.0 != 1.3.4.* < 2.0") and asserted as valid, pinning a leniency
		// the reference implementation does not have: measured against
		// pypa/packaging 26.2, that form raises InvalidSpecifier while the comma
		// form parses to '!=1.3.4.*,<2.0,>=1.0'. The intent of each case is the
		// conjunction, which the commas express without relying on the defect.
		{"1.0", ">= 1.0, != 1.3.4.*, < 2.0", true},
		{"1.0", "~= 0.9, >= 1.0, != 1.3.4.*, < 2.0", false},
		{"0.9", "~= 0.9, != 1.3.4.*, < 2.0", true},
		{"2.0", ">= 1.0, != 1.3.4.*, < 2.0", false},
		{"1.3.4", ">= 1.0, != 1.3.4.*, < 2.0", false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s %s", tt.version, tt.spec), func(t *testing.T) {
			c, err := NewSpecifiers(tt.spec)
			require.NoError(t, err)

			v, err := Parse(tt.version)
			require.NoError(t, err)

			assert.Equal(t, tt.want, c.Check(v))
		})
	}
}

// TestLeftPad_InsertsPaddingBeforeTheSuffix replaces the old
// TestPadVersion_RightRestDerivesFromRight.
//
// padVersion is gone: it padded the two sides against EACH OTHER and
// reassembled numeric prefix plus "rest" for both, and the bug it guarded
// against was slicing the left-hand side twice so the right-hand "rest" came
// from the wrong version. leftPad pads ONE side up to a target numeric width,
// with a single slice of a single input, so that class of mix-up is structurally
// impossible.
//
// What still has to hold, and is the substance of the old test, is that the "0"
// padding goes AFTER the numeric prefix and BEFORE any suffix segment, and that
// the suffix survives intact.
func TestLeftPad_InsertsPaddingBeforeTheSuffix(t *testing.T) {
	tests := []struct {
		name   string
		split  []string
		target int
		want   []string
	}{
		{
			// Ported from pypa/packaging 26.2's _left_pad docstring.
			name:   "upstream docstring example",
			split:  []string{"0", "1", "a1"},
			target: 4,
			want:   []string{"0", "1", "0", "0", "a1"},
		},
		{
			name:   "suffix is preserved, not overwritten",
			split:  []string{"0", "2", "rc1"},
			target: 3,
			want:   []string{"0", "2", "0", "rc1"},
		},
		{
			name:   "already wide enough is returned unchanged",
			split:  []string{"0", "1", "2", "3"},
			target: 3,
			want:   []string{"0", "1", "2", "3"},
		},
		{
			name:   "no suffix",
			split:  []string{"0", "2"},
			target: 4,
			want:   []string{"0", "2", "0", "0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, leftPad(tt.split, tt.target))
		})
	}
}

// versionSplit must peel the epoch off as its own leading component, and must
// treat a release-plus-pre-release run in one dot-segment as two components.
// Both were missing before, and both are why `==0!2.*` did not match `2`.
func TestVersionSplit_EpochAndPreRelease(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"2", []string{"0", "2"}},
		{"0!2", []string{"0", "2"}},
		{"2!1.0", []string{"2", "1", "0"}},
		{"2rc1", []string{"0", "2", "rc1"}},
		// The pre-release run is split out of its own dot-segment, so "0a1"
		// becomes two components. Verified against pypa/packaging 26.2's
		// _version_split.
		{"1.0a1", []string{"0", "1", "0", "a1"}},
		{"1.0.post1", []string{"0", "1", "0", "post1"}},
		{"2.0.0", []string{"0", "2", "0", "0"}},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			assert.Equal(t, tt.want, versionSplit(tt.in))
		})
	}
}

// TestNewRSpecifiers_UsesProvidedSanitizer is a regression test for a bug where
// NewRSpecifiers ignored the caller-supplied sanitizer and always forwarded an
// identity function to newRSpecifiers instead.
func TestNewRSpecifiers_UsesProvidedSanitizer(t *testing.T) {
	// A sanitizer that rewrites "9" digits to "1" digits, simulating a caller
	// normalizing a non-standard version token before PEP 440 parsing.
	sanitizer := func(s string) string {
		return strings.ReplaceAll(s, "9", "1")
	}

	specs, err := NewRSpecifiers("==9.0.0", sanitizer)
	require.NoError(t, err)

	v, err := Parse("1.0.0")
	require.NoError(t, err)

	// If the sanitizer is honored, "==9.0.0" is rewritten to "==1.0.0" before
	// being parsed, so 1.0.0 satisfies it. If the sanitizer is ignored (as with
	// the identity-forwarding bug), the specifier stays "==9.0.0" and this
	// version does not satisfy it.
	assert.True(t, specs.Check(v))
}

// TestVersion_CheckWithPreRelease pins the corrected meaning of
// WithPreRelease(true): it is a candidate-SELECTION policy and does not
// suppress the operator-level pre-release guards.
//
// The {"2.0a1", "<2", true} case this table used to assert was a divergence.
// Measured against pypa/packaging 26.2, `<2` does not contain `2.0a1` under
// ANY pre-release policy:
//
//	Specifier("<2", prereleases=True).contains("2.0a1")  -> False
//	Specifier("<2").contains("2.0a1")                    -> False
//	Specifier("<2", prereleases=False).contains("2.0a1") -> False
//
// The old `true` came from WithPreRelease(true) setting a flag that made
// Version.IsPreRelease report false, which disabled the guard in
// specifierLessThan. See rstudio/package-manager#19383.
func TestVersion_CheckWithPreRelease(t *testing.T) {
	tests := []struct {
		version string
		spec    string
		want    bool
	}{
		{"1.3.4", "< 2.0", true},
		{"2.0a1", "<2", false},
		{"2.1a1", "<2", false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s %s", tt.version, tt.spec), func(t *testing.T) {
			c, err := NewSpecifiers(tt.spec, WithPreRelease(true))
			require.NoError(t, err)

			v, err := Parse(tt.version)
			require.NoError(t, err)

			assert.Equal(t, tt.want, c.Check(v))
		})
	}
}
