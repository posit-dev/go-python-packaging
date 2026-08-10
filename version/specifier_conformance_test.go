// SPDX-License-Identifier: Apache-2.0 OR MIT

// Conformance tables ported from pypa/packaging's tests/test_specifiers.py,
// one Go table per upstream test function. This file contains ONLY cases
// ported from upstream; hand-written cases belong in specifier_test.go,
// specifier_api_test.go or prerelease_policy_test.go.
//
// Where our expected value differs from upstream's literal assertion, the
// difference is annotated inline with a reason. An UNANNOTATED divergence
// from upstream is a bug in this file, NOT a licence to change the library.
//
// Known intentional divergences, all annotated at their table:
//
//	Specifiers.String()  - we preserve input order; upstream sorts its
//	                       members. See TestConformance_SpecifierSetStr.
//	bare version         - we admit an operator-less constraint ("2.0") for
//	                       the R path; upstream rejects it. Tracked as
//	                       rstudio/package-manager#18634.
//
// Every expectation below was cross-checked against the INSTALLED
// pypa/packaging 26.2 as well as the literal in the pinned file, so a stale
// literal cannot be baked in silently.
//
// Upstream pinned at 4eb0753dba8fcaaac8eb75463374e448f0931558.

package version

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ported from TestSpecifier.test_specifiers_valid (SPECIFIERS).
var conformanceValidSpecifiers = []string{
	"~=2.0",
	"==2.1.*",
	"==2.1.0.3",
	"!=2.2.*",
	"!=2.2.0.5",
	"<=5",
	">=7.9a1",
	"<1.0.dev1",
	">2.0.post1",
}

func TestConformance_SpecifiersValid(t *testing.T) {
	for _, in := range conformanceValidSpecifiers {
		t.Run(in, func(t *testing.T) {
			_, err := NewSpecifier(in)
			assert.NoError(t, err)
			_, err = NewSpecifiers(in)
			assert.NoError(t, err)
		})
	}
}

// Ported from TestSpecifier.test_specifiers_invalid.
//
// ⚠️ ONE upstream case is deliberately absent: the operator-less specifier
// "2.0". This package admits a bare version as a constraint on purpose, for
// the R path, and PPM depends on it (see the validConstraintRegexp comment in
// init). Narrowing it is tracked as rstudio/package-manager#18634.
var conformanceInvalidSpecifiers = []string{
	// "2.0", // deliberate divergence: bare version, see above
	"=>2.0",
	"==",
	"~=1.0+5",
	">=1.0+deadbeef",
	"<=1.0+abc123",
	">1.0+watwat",
	"<1.0+1.0",
	"~=1.0.*",
	">=1.0.*",
	"<=1.0.*",
	">1.0.*",
	"<1.0.*",
	"==1.0.*+5",
	"!=1.0.*+deadbeef",
	"==2.0a1.*",
	"!=2.0a1.*",
	"==2.0.post1.*",
	"!=2.0.post1.*",
	"==2.0.dev1.*",
	"!=2.0.dev1.*",
	"==1.0+5.*",
	"!=1.0+deadbeef.*",
	"==1.0.*.5",
	"~=1",
	"==1.0.dev1.*",
	"!=1.0.dev1.*",
	"==1.2+\u0130",
	"==1.2+\u0130\u0131\u017fK",
}

func TestConformance_SpecifiersInvalid(t *testing.T) {
	for _, in := range conformanceInvalidSpecifiers {
		t.Run(fmt.Sprintf("%q", in), func(t *testing.T) {
			_, err := NewSpecifier(in)
			assert.Error(t, err, "NewSpecifier must reject %q", in)
			_, err = NewSpecifiers(in)
			assert.Error(t, err, "NewSpecifiers must reject %q", in)
		})
	}
}

// The bare-version divergence, pinned explicitly so it cannot regress
// silently in either direction.
func TestConformance_BareVersionIsADeliberateDivergence(t *testing.T) {
	_, err := NewSpecifiers("2.0")
	assert.NoError(t, err, "a bare version stays a valid constraint here")
}

// Ported from TestSpecifier.test_specifiers_normalized. Upstream prefixes
// each version with every operator that accepts it and asserts the specifier
// parses; the operator set narrows to ==/!= once a local segment is present.
var conformanceNormalizedVersions = []string{
	"1.0dev",
	"1.0.dev",
	"1.0dev1",
	"1.0-dev",
	"1.0-dev1",
	"1.0DEV",
	"1.0.DEV",
	"1.0DEV1",
	"1.0.DEV1",
	"1.0-DEV",
	"1.0-DEV1",
	"1.0a",
	"1.0.a",
	"1.0.a1",
	"1.0-a",
	"1.0-a1",
	"1.0alpha",
	"1.0.alpha",
	"1.0.alpha1",
	"1.0-alpha",
	"1.0-alpha1",
	"1.0A",
	"1.0.A",
	"1.0.A1",
	"1.0-A",
	"1.0-A1",
	"1.0ALPHA",
	"1.0.ALPHA",
	"1.0.ALPHA1",
	"1.0-ALPHA",
	"1.0-ALPHA1",
	"1.0b",
	"1.0.b",
	"1.0.b1",
	"1.0-b",
	"1.0-b1",
	"1.0beta",
	"1.0.beta",
	"1.0.beta1",
	"1.0-beta",
	"1.0-beta1",
	"1.0B",
	"1.0.B",
	"1.0.B1",
	"1.0-B",
	"1.0-B1",
	"1.0BETA",
	"1.0.BETA",
	"1.0.BETA1",
	"1.0-BETA",
	"1.0-BETA1",
	"1.0c",
	"1.0.c",
	"1.0.c1",
	"1.0-c",
	"1.0-c1",
	"1.0rc",
	"1.0.rc",
	"1.0.rc1",
	"1.0-rc",
	"1.0-rc1",
	"1.0C",
	"1.0.C",
	"1.0.C1",
	"1.0-C",
	"1.0-C1",
	"1.0RC",
	"1.0.RC",
	"1.0.RC1",
	"1.0-RC",
	"1.0-RC1",
	"1.0post",
	"1.0.post",
	"1.0post1",
	"1.0-post",
	"1.0-post1",
	"1.0POST",
	"1.0.POST",
	"1.0POST1",
	"1.0.POST1",
	"1.0-POST",
	"1.0-POST1",
	"1.0-5",
	"1.0+AbC",
	"1.01",
	"1.0a05",
	"1.0b07",
	"1.0c056",
	"1.0rc09",
	"1.0.post000",
	"1.1.dev09000",
	"00!1.2",
	"0100!0.0",
	"v1.0",
	"  \r \f \v v1.0\t\n",
}

func TestConformance_SpecifiersNormalized(t *testing.T) {
	for _, v := range conformanceNormalizedVersions {
		ops := []string{"~=", "==", "!=", "<=", ">=", "<", ">"}
		if strings.Contains(v, "+") {
			// A local version is only legal on the (in)equality operators.
			ops = []string{"==", "!="}
		}
		for _, op := range ops {
			in := op + v
			t.Run(fmt.Sprintf("%q", in), func(t *testing.T) {
				_, err := NewSpecifier(in)
				assert.NoError(t, err)
			})
		}
	}
}

// Ported from TestSpecifier.test_specifiers_str_and_repr (the str() half;
// Go has no repr).
func TestConformance_SpecifierStr(t *testing.T) {
	tests := []struct{ in, want string }{
		{"!=2.0", "!=2.0"},
		{"<2.0", "<2.0"},
		{"<=2.0", "<=2.0"},
		{"==2.0", "==2.0"},
		{">2.0", ">2.0"},
		{">=2.0", ">=2.0"},
		{"~=2.0", "~=2.0"},
		{"< 2", "<2"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			s, err := NewSpecifier(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, s.String())
		})
	}
}

// Ported from TestSpecifier.test_specifier_operator_property.
func TestConformance_SpecifierOperatorProperty(t *testing.T) {
	tests := []struct{ spec, want string }{
		{"~=2.0", "~="},
		{"==2.1.*", "=="},
		{"==2.1.0.3", "=="},
		{"!=2.2.*", "!="},
		{"!=2.2.0.5", "!="},
		{"<=5", "<="},
		{">=7.9a1", ">="},
		{"<1.0.dev1", "<"},
		{">2.0.post1", ">"},
		{"===lolwat", "==="},
	}
	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			s, err := NewSpecifier(tt.spec)
			require.NoError(t, err)
			assert.Equal(t, tt.want, s.Operator())
		})
	}
}

// Ported from TestSpecifier.test_specifier_version_property.
func TestConformance_SpecifierVersionProperty(t *testing.T) {
	tests := []struct{ spec, want string }{
		{"~=2.0", "2.0"},
		{"==2.1.*", "2.1.*"},
		{"==2.1.0.3", "2.1.0.3"},
		{"!=2.2.*", "2.2.*"},
		{"!=2.2.0.5", "2.2.0.5"},
		{"<=5", "5"},
		{">=7.9a1", "7.9a1"},
		{"<1.0.dev1", "1.0.dev1"},
		{">2.0.post1", "2.0.post1"},
		{"===lolwat", "lolwat"},
	}
	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			s, err := NewSpecifier(tt.spec)
			require.NoError(t, err)
			assert.Equal(t, tt.want, s.Version())
		})
	}
}

// Ported from TestSpecifier.test_specifiers -- the core PEP 440 matching
// table. Upstream builds each Specifier with prereleases=True and asserts
// both the string and the Version form of the operand.
//
// ⚠️ prereleases=True does NOT relax the operator-level guards: entries like
// ("2.0a1", "<2", false) and ("2.0.post1", ">2", false) are expected FALSE
// here even though pre-releases are being included. Selection is not matching.
func TestConformance_SpecifierMatching(t *testing.T) {
	tests := []struct {
		version string
		spec    string
		want    bool
	}{
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
		{"2.dev1", "==2.*", true},
		{"2a1", "==2.*", true},
		{"2a1.post1", "==2.*", true},
		{"2b1", "==2.*", true},
		{"2b1.dev1", "==2.*", true},
		{"2c1", "==2.*", true},
		{"2c1.post1.dev1", "==2.*", true},
		{"2c1.post1.dev1", "==2.0.*", true},
		{"2rc1", "==2.*", true},
		{"2rc1", "==2.0.*", true},
		{"2", "==2.*", true},
		{"2", "==2.0.*", true},
		{"2", "==2.0.0.*", true},
		{"2", "==0!2.*", true},
		{"0!2", "==2.*", true},
		{"2.0", "==2.*", true},
		{"2.0.0", "==2.*", true},
		{"2.0.0.0", "==2.0.*", true},
		{"2.1+local.version", "==2.1.*", true},
		{"2.1", "!=2", true},
		{"2.1", "!=2.0", true},
		{"2.0.1", "!=2", true},
		{"2.0.1", "!=2.0", true},
		{"2.0.1", "!=2.0.0", true},
		{"2.0", "!=2.0+deadbeef", true},
		{"2.0", "!=3.*", true},
		{"2.1", "!=2.0.*", true},
		{"3", "!=2.0.0.*", true},
		{"2.1.0.0", "!=2.0.*", true},
		{"2.0", ">=2", true},
		{"2.0", ">=2.0", true},
		{"2.0", ">=2.0.0", true},
		{"2.0.post1", ">=2", true},
		{"2.0.post1.dev1", ">=2", true},
		{"3", ">=2", true},
		{"3.0.0a8", ">=3.0.0a7", true},
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
		{"3.0.0a7", "<=3.0.0a8", true},
		{"3", ">2", true},
		{"2.1", ">2.0", true},
		{"2.0.1", ">2", true},
		{"2.1.post1", ">2", true},
		{"2.1+local.version", ">2", true},
		{"3.0.0a8", ">3.0.0a7", true},
		{"1", "<2", true},
		{"2.0", "<2.1", true},
		{"2.0.dev0", "<2.1", true},
		{"3.0.0a7", "<3.0.0a8", true},
		{"1", "~=1.0", true},
		{"1.0.1", "~=1.0", true},
		{"1.1", "~=1.0", true},
		{"1.9999999", "~=1.0", true},
		{"1.1", "~=1.0a1", true},
		{"2022.01.01", "~=2022.01.01", true},
		{"2!1.0", "~=2!1.0", true},
		{"2!1.0", "==2!1.*", true},
		{"2!1.0", "==2!1.0", true},
		{"2!1.0", "!=1.0", true},
		{"2!1.0.0", "==2!1.0.0.0.*", true},
		{"2!1.0.0", "==2!1.0.*", true},
		{"2!1.0.0", "==2!1.*", true},
		{"1.0", "!=2!1.0", true},
		{"1.0", "<=2!0.1", true},
		{"2!1.0", ">=2.0", true},
		{"1.0", "<2!0.1", true},
		{"2!1.0", ">2.0", true},
		{"2.0.5", ">2.0dev", true},
		{"1.0+local", ">1.0.dev1", true},
		{"4.1.0a2.dev1235+local", ">4.1.0a2.dev1234", true},
		{"1.0a2+local", ">1.0a1", true},
		{"1.0b2+local", ">1.0b1", true},
		{"1.0rc2+local", ">1.0rc1", true},
		{"1.0.post2+local", ">1.0.post1", true},
		{"1.0.dev2+local", ">1.0.dev1", true},
		{"1.0a1.dev2+local", ">1.0a1.dev1", true},
		{"1.0.post1.dev2+local", ">1.0.post1.dev1", true},
		{"2.1", "==2", false},
		{"2.1", "==2.0", false},
		{"2.1", "==2.0.0", false},
		{"2.0", "==2.0+deadbeef", false},
		{"2.0", "==3.*", false},
		{"2.1", "==2.0.*", false},
		{"3", "==2.0.0.*", false},
		{"2.1.0.0", "==2.0.*", false},
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
		{"2.dev1", "!=2.*", false},
		{"2a1", "!=2.*", false},
		{"2a1.post1", "!=2.*", false},
		{"2b1", "!=2.*", false},
		{"2b1.dev1", "!=2.*", false},
		{"2c1", "!=2.*", false},
		{"2c1.post1.dev1", "!=2.*", false},
		{"2c1.post1.dev1", "!=2.0.*", false},
		{"2rc1", "!=2.*", false},
		{"2rc1", "!=2.0.*", false},
		{"2", "!=2.*", false},
		{"2", "!=2.0.*", false},
		{"2", "!=2.0.0.*", false},
		{"2.0", "!=2.*", false},
		{"2.0.0", "!=2.*", false},
		{"2.0.0.0", "!=2.0.*", false},
		{"2.0.dev1", ">=2", false},
		{"2.0a1", ">=2", false},
		{"2.0a1.dev1", ">=2", false},
		{"2.0b1", ">=2", false},
		{"2.0b1.post1", ">=2", false},
		{"2.0c1", ">=2", false},
		{"2.0c1.post1.dev1", ">=2", false},
		{"2.0rc1", ">=2", false},
		{"1", ">=2", false},
		{"2.0.post1", "<=2", false},
		{"2.0.post1.dev1", "<=2", false},
		{"3", "<=2", false},
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
		{"4.1.0a2.dev1234+local", ">4.1.0a2.dev1234", false},
		{"1.0a1+local", ">1.0a1", false},
		{"1.0b1+local", ">1.0b1", false},
		{"1.0rc1+local", ">1.0rc1", false},
		{"1.0.post1+local", ">1.0.post1", false},
		{"1.0.dev1+local", ">1.0.dev1", false},
		{"1.0a1.dev1+local", ">1.0a1.dev1", false},
		{"1.0.post1.dev1+local", ">1.0.post1.dev1", false},
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
		{"2.0", "~=1.0", false},
		{"1.1.0", "~=1.0.0", false},
		{"1.1.post1", "~=1.0.0", false},
		{"1.0", "~=2!1.0", false},
		{"2!1.0", "~=1.0", false},
		{"2!1.0", "==1.0", false},
		{"1.0", "==2!1.0", false},
		{"2!1.0", "==1.0.0.*", false},
		{"1.0", "==2!1.0.0.*", false},
		{"2!1.0", "==1.*", false},
		{"1.0", "==2!1.*", false},
		{"2!1.0", "!=2!1.0", false},
	}
	for _, tt := range tests {
		t.Run(tt.version+"__"+tt.spec, func(t *testing.T) {
			s, err := NewSpecifier(tt.spec, WithPreReleases(PreReleasesInclude))
			require.NoError(t, err)

			assert.Equal(t, tt.want, s.Contains(tt.version), "string operand")

			v, err := Parse(tt.version)
			require.NoError(t, err)
			assert.Equal(t, tt.want, s.ContainsVersion(v), "Version operand")

			// The same answer through the set, and through pure matching.
			ss, err := NewSpecifiers(tt.spec, WithPreReleases(PreReleasesInclude))
			require.NoError(t, err)
			assert.Equal(t, tt.want, ss.ContainsVersion(v), "set, Version operand")
			assert.Equal(t, tt.want, ss.Check(v), "Check")
		})
	}
}

// Ported from TestSpecifier.test_invalid_version. This table was previously
// INEXPRESSIBLE: Check takes a version.Version, and version.Parse of
// "not a valid version" fails, so there was no way to pass the operand in.
func TestConformance_InvalidVersionOperand(t *testing.T) {
	tests := []struct {
		spec    string
		version string
		want    bool
	}{
		{"==1.0", "not a valid version", false},
		{"==1.*", "not a valid version", false},
		{">=1.0", "not a valid version", false},
		{">1.0", "not a valid version", false},
		{"<=1.0", "not a valid version", false},
		{"<1.0", "not a valid version", false},
		{"~=1.0", "not a valid version", false},
		{"!=1.0", "not a valid version", false},
		{"!=1.*", "not a valid version", false},
		{"===invalid", "invalid", true},
		{"===foobar", "invalid", false},
	}
	for _, tt := range tests {
		t.Run(tt.spec+"__"+tt.version, func(t *testing.T) {
			s, err := NewSpecifier(tt.spec, WithPreReleases(PreReleasesInclude))
			require.NoError(t, err)
			assert.Equal(t, tt.want, s.Contains(tt.version))
		})
	}
}

// Ported from TestSpecifier.test_arbitrary_equality.
func TestConformance_ArbitraryEquality(t *testing.T) {
	tests := []struct {
		version string
		spec    string
		want    bool
	}{
		{"1.0.0", "===1.0", false},
		{"1.0.dev0", "===1.0", false},
		{"1.0", "===1.0", true},
		{"1.0.dev0", "===1.0.dev0", true},
		{"1.0+downstream1", "===1.0", false},
		{"1.0", "===1.0+downstream1", false},
		{"foobar", "===foobar", true},
		{"foobar", "===baz", false},
		{"1.0a1", "===1.0a1", true},
		{"1.0A1", "===1.0A1", true},
		{"1.0a1", "===1.0A1", true},
		{"1.0A1", "===1.0a1", true},
		{"1.0b1", "===1.0b1", true},
		{"1.0B1", "===1.0B1", true},
		{"1.0b1", "===1.0B1", true},
		{"1.0B1", "===1.0b1", true},
		{"1.0rc1", "===1.0rc1", true},
		{"1.0RC1", "===1.0RC1", true},
		{"1.0rc1", "===1.0RC1", true},
		{"1.0RC1", "===1.0rc1", true},
		{"1.0.post1", "===1.0.post1", true},
		{"1.0.POST1", "===1.0.POST1", true},
		{"1.0.post1", "===1.0.POST1", true},
		{"1.0.POST1", "===1.0.post1", true},
		{"1.0.dev1", "===1.0.dev1", true},
		{"1.0.DEV1", "===1.0.DEV1", true},
		{"1.0.dev1", "===1.0.DEV1", true},
		{"1.0.DEV1", "===1.0.dev1", true},
		{"1.0+local", "===1.0+local", true},
		{"1.0+LOCAL", "===1.0+LOCAL", true},
		{"1.0+local", "===1.0+LOCAL", true},
		{"1.0+LOCAL", "===1.0+local", true},
		{"1.0+abc.def", "===1.0+abc.def", true},
		{"1.0+ABC.DEF", "===1.0+ABC.DEF", true},
		{"1.0+abc.def", "===1.0+ABC.DEF", true},
		{"1.0+ABC.DEF", "===1.0+abc.def", true},
		{"1.0+AbC", "===1.0+AbC", true},
		{"1.0+AbC", "===1.0+abc", true},
		{"1.0+AbC", "===1.0+ABC", true},
		{"1.0a1.post2.dev3", "===1.0a1.post2.dev3", true},
		{"1.0A1.POST2.DEV3", "===1.0A1.POST2.DEV3", true},
		{"1.0a1.post2.dev3", "===1.0A1.POST2.DEV3", true},
		{"1.0A1.POST2.DEV3", "===1.0a1.post2.dev3", true},
		{"lolwat", "===LOLWAT", true},
		{"lolwat", "===LoLWaT", true},
		{"LOLWAT", "===lolwat", true},
		{"LoLWaT", "===lOlwAt", true},
	}
	for _, tt := range tests {
		t.Run(tt.version+"__"+tt.spec, func(t *testing.T) {
			s, err := NewSpecifier(tt.spec)
			require.NoError(t, err)
			assert.Equal(t, tt.want, s.Contains(tt.version))
		})
	}
}

// Ported from TestSpecifier.test_arbitrary_equality_normalization.
//
// The whole point of this table is the difference between an operand passed
// as a STRING (compared as written) and one passed as a parsed Version
// (compared normalized). Contains covers the first, ContainsVersion the
// second -- which is why both methods exist.
func TestConformance_ArbitraryEqualityNormalization(t *testing.T) {
	tests := []struct {
		spec      string
		item      string
		asVersion bool
		want      bool
	}{
		{"===1.1", "1.01", false, false},
		{"===1.01", "1.1", false, false},
		{"===1.01", "1.01", false, true},
		{"===1.1", "1.1", false, true},
		{"===1.1", "1.1", true, true},
		{"===1.1", "1.1", true, true},
		{"===1.01", "1.1", true, false},
		{"===1.01", "1.1", true, false},
		{"===1.a1", "1.a1", false, true},
		{"===1a1", "1.a1", false, false},
		{"===1.a1", "1a1", false, false},
		{"===1a1", "1a1", false, true},
		{"===1.a1", "1a1", true, false},
		{"===1a1", "1a1", true, true},
		{"===0!1.0", "0!1.0", false, true},
		{"===0!1.0", "1.0", false, false},
		{"===1.0", "0!1.0", false, false},
		{"===0!1.0", "1.0", true, false},
		{"===1.0", "1.0", true, true},
		{"===01.0", "01.0", false, true},
		{"===01.0", "1.0", false, false},
		{"===1.0", "01.0", false, false},
		{"===01.0", "1.0", true, false},
		{"===1.0", "1.0", true, true},
		{"===1.0.post1", "1.0.post1", false, true},
		{"===1.0-1", "1.0-1", false, true},
		{"===1.0-1", "1.0.post1", false, false},
		{"===1.0.post1", "1.0-1", false, false},
		{"===1.0-1", "1.0.post1", true, false},
		{"===1.0.post1", "1.0.post1", true, true},
		{"===1.0.dev01", "1.0.dev01", false, true},
		{"===1.0.dev01", "1.0.dev1", false, false},
		{"===1.0.dev1", "1.0.dev01", false, false},
		{"===1.0.dev01", "1.0.dev1", true, false},
		{"===1.0.dev1", "1.0.dev1", true, true},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s__%s__v=%t", tt.spec, tt.item, tt.asVersion), func(t *testing.T) {
			s, err := NewSpecifier(tt.spec, WithPreReleases(PreReleasesInclude))
			require.NoError(t, err)
			if tt.asVersion {
				v, err := Parse(tt.item)
				require.NoError(t, err)
				assert.Equal(t, tt.want, s.ContainsVersion(v))
				return
			}
			assert.Equal(t, tt.want, s.Contains(tt.item))
		})
	}
}

// Ported from TestSpecifier.test_specifier_prereleases_detection.
func TestConformance_SpecifierPreReleaseDetection(t *testing.T) {
	tests := []struct {
		spec string
		want PreReleases
	}{
		{"==1.0", PreReleasesExclude},
		{">=1.0", PreReleasesExclude},
		{"<=1.0", PreReleasesExclude},
		{"~=1.0", PreReleasesExclude},
		{"<1.0", PreReleasesExclude},
		{">1.0", PreReleasesExclude},
		{"<1.0.dev1", PreReleasesInclude},
		{">1.0.dev1", PreReleasesInclude},
		{"!=1.0.dev1", PreReleasesExclude},
		{"==1.0.*", PreReleasesExclude},
		{"==1.0.dev1", PreReleasesInclude},
		{">=1.0.dev1", PreReleasesInclude},
		{"<=1.0.dev1", PreReleasesInclude},
		{"~=1.0.dev1", PreReleasesInclude},
	}
	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			s, err := NewSpecifier(tt.spec)
			require.NoError(t, err)
			assert.Equal(t, tt.want, s.PreReleases())
		})
	}
}

// Ported from TestSpecifier.test_specifiers_prereleases: the constructor
// policy and the per-call policy, in every combination upstream exercises.
func TestConformance_SpecifierPreReleases(t *testing.T) {
	tests := []struct {
		spec    string
		version string
		specPre PreReleases
		callPre PreReleases
		want    bool
	}{
		{">=1.0", "2.0.dev1", PreReleasesAuto, PreReleasesAuto, true},
		{">=2.0.dev1", "2.0a1", PreReleasesAuto, PreReleasesAuto, true},
		{"==2.0.*", "2.0a1.dev1", PreReleasesAuto, PreReleasesAuto, true},
		{"<=2.0", "1.0.dev1", PreReleasesAuto, PreReleasesAuto, true},
		{"<=2.0.dev1", "1.0a1", PreReleasesAuto, PreReleasesAuto, true},
		{"<2.0", "2.0a1", PreReleasesAuto, PreReleasesAuto, false},
		{"<2.0a2", "2.0a1", PreReleasesAuto, PreReleasesAuto, true},
		{">1.0.dev1", "1.0.dev0", PreReleasesAuto, PreReleasesAuto, false},
		{">1.0.dev1", "1.0.dev2", PreReleasesAuto, PreReleasesAuto, true},
		{">1.0.dev1", "1.0.post0", PreReleasesAuto, PreReleasesAuto, true},
		{">1.0.dev1", "1.0.post1", PreReleasesAuto, PreReleasesAuto, true},
		{">1.0.dev0", "1.0.post0", PreReleasesAuto, PreReleasesAuto, true},
		{">1.0a1", "1.0.post0", PreReleasesAuto, PreReleasesAuto, true},
		{">1.0b1", "1.0.post0", PreReleasesAuto, PreReleasesAuto, true},
		{">1.0rc1", "1.0.post0", PreReleasesAuto, PreReleasesAuto, true},
		{">1.0a1", "1.0a1.post0", PreReleasesAuto, PreReleasesAuto, false},
		{">1.0b2", "1.0b2.post0", PreReleasesAuto, PreReleasesAuto, false},
		{">1.0rc1", "1.0rc1.post0", PreReleasesAuto, PreReleasesAuto, false},
		{">1.0a1", "1.0a2.post0", PreReleasesAuto, PreReleasesAuto, true},
		{">1.0b1", "1.0b2.post0", PreReleasesAuto, PreReleasesAuto, true},
		{">1.0.dev1", "1.0", PreReleasesAuto, PreReleasesAuto, true},
		{">1.0.dev1", "1.0a1", PreReleasesAuto, PreReleasesAuto, true},
		{">1.0.dev1", "1.1", PreReleasesAuto, PreReleasesAuto, true},
		{">1.0", "1.0.post0", PreReleasesAuto, PreReleasesAuto, false},
		{">1.0", "1.0.post1", PreReleasesAuto, PreReleasesAuto, false},
		{">1.0", "2.0.post0", PreReleasesAuto, PreReleasesAuto, true},
		{">1.0", "0.9.post0", PreReleasesAuto, PreReleasesAuto, false},
		{">1.0.dev1", "1.1.post0", PreReleasesAuto, PreReleasesAuto, true},
		{">1.0.dev1", "0.9.post0", PreReleasesAuto, PreReleasesAuto, false},
		{"<1.0.post1", "1.0.post1.dev0", PreReleasesAuto, PreReleasesAuto, false},
		{"<1.0.post0", "1.0.post0.dev0", PreReleasesAuto, PreReleasesAuto, false},
		{"<1.0.post1", "1.0.dev0", PreReleasesAuto, PreReleasesAuto, true},
		{"<1.0.post1", "1.0a1", PreReleasesAuto, PreReleasesAuto, true},
		{"<1.0.post1", "1.0rc1", PreReleasesAuto, PreReleasesAuto, true},
		{"<1.0.post0", "1.0.dev0", PreReleasesAuto, PreReleasesAuto, true},
		{"<1.0.post0", "1.0a1", PreReleasesAuto, PreReleasesAuto, true},
		{"<1.0.post0", "1.0b1", PreReleasesAuto, PreReleasesAuto, true},
		{"<1.0.post0", "1.0rc2", PreReleasesAuto, PreReleasesAuto, true},
		{"<1.0.post1", "1.0.post0.dev0", PreReleasesAuto, PreReleasesAuto, true},
		{"<1.0.post2", "1.0.post1.dev0", PreReleasesAuto, PreReleasesAuto, true},
		{"<1.0.post1", "1.0", PreReleasesAuto, PreReleasesAuto, true},
		{"<1.0.post1", "1.0.post0", PreReleasesAuto, PreReleasesAuto, true},
		{"<1.0.post1", "0.9", PreReleasesAuto, PreReleasesAuto, true},
		{"<1.0.post0", "1.0", PreReleasesAuto, PreReleasesAuto, true},
		{"<1.0.post10", "1.0.dev0", PreReleasesAuto, PreReleasesAuto, true},
		{"<1.0.post10", "1.0.post9.dev0", PreReleasesAuto, PreReleasesAuto, true},
		{"<1.0.post10", "1.0.post9", PreReleasesAuto, PreReleasesAuto, true},
		{"<1.0.post1", "1.0+local", PreReleasesAuto, PreReleasesAuto, true},
		{"<1.0.post1", "1.0.post0+local", PreReleasesAuto, PreReleasesAuto, true},
		{"<1.0.post1", "0.9.dev0", PreReleasesAuto, PreReleasesAuto, true},
		{"<=2.0", "1.0.dev1", PreReleasesExclude, PreReleasesAuto, false},
		{"<=2.0a1", "1.0.dev1", PreReleasesExclude, PreReleasesAuto, false},
		{"<=2.0", "1.0.dev1", PreReleasesAuto, PreReleasesExclude, false},
		{"<=2.0a1", "1.0.dev1", PreReleasesAuto, PreReleasesExclude, false},
		{"<=2.0", "1.0.dev1", PreReleasesInclude, PreReleasesExclude, false},
		{"<=2.0a1", "1.0.dev1", PreReleasesInclude, PreReleasesExclude, false},
		{"<=2.0", "1.0.dev1", PreReleasesExclude, PreReleasesInclude, true},
		{"<=2.0a1", "1.0.dev1", PreReleasesExclude, PreReleasesInclude, true},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s__%s__%s__%s", tt.spec, tt.version, tt.specPre, tt.callPre), func(t *testing.T) {
			s, err := NewSpecifier(tt.spec, WithPreReleases(tt.specPre))
			require.NoError(t, err)
			assert.Equal(t, tt.want, s.Contains(tt.version, WithPreReleases(tt.callPre)))
		})
	}
}

// Ported from TestSpecifier.test_specifier_filter.
func TestConformance_SpecifierFilter(t *testing.T) {
	tests := []struct {
		spec    string
		specPre PreReleases
		callPre PreReleases
		input   []string
		want    []string
	}{
		{">=1.0.dev1", PreReleasesAuto, PreReleasesAuto, []string{"1.0", "2.0a1"}, []string{"1.0", "2.0a1"}},
		{">=1.2.3", PreReleasesAuto, PreReleasesAuto, []string{"1.2", "1.5a1"}, []string{"1.5a1"}},
		{">=1.2.3", PreReleasesAuto, PreReleasesAuto, []string{"1.3", "1.5a1"}, []string{"1.3"}},
		{">=1.0", PreReleasesAuto, PreReleasesAuto, []string{"2.0a1"}, []string{"2.0a1"}},
		{"!=2.0a1", PreReleasesAuto, PreReleasesAuto, []string{"1.0a2", "1.0", "2.0a1"}, []string{"1.0"}},
		{"==2.0a1", PreReleasesAuto, PreReleasesAuto, []string{"2.0a1"}, []string{"2.0a1"}},
		{">2.0a1", PreReleasesAuto, PreReleasesAuto, []string{"2.0a1", "3.0a2", "3.0"}, []string{"3.0a2", "3.0"}},
		{"<2.0a1", PreReleasesAuto, PreReleasesAuto, []string{"1.0a2", "1.0", "2.0a1"}, []string{"1.0a2", "1.0"}},
		{"~=2.0a1", PreReleasesAuto, PreReleasesAuto, []string{"1.0", "2.0a1", "3.0a2", "3.0"}, []string{"2.0a1"}},
		{">=1.0.dev1", PreReleasesAuto, PreReleasesExclude, []string{"1.0", "2.0a1"}, []string{"1.0"}},
		{">=1.0.dev1", PreReleasesInclude, PreReleasesAuto, []string{"1.0", "2.0a1"}, []string{"1.0", "2.0a1"}},
		{">=1.0.dev1", PreReleasesExclude, PreReleasesAuto, []string{"1.0", "2.0a1"}, []string{"1.0"}},
		{">=1.0", PreReleasesInclude, PreReleasesExclude, []string{"1.0", "2.0a1"}, []string{"1.0"}},
		{">=1.0", PreReleasesExclude, PreReleasesInclude, []string{"1.0", "2.0a1"}, []string{"1.0", "2.0a1"}},
		{">=1.0", PreReleasesInclude, PreReleasesInclude, []string{"1.0", "2.0a1"}, []string{"1.0", "2.0a1"}},
		{">=1.0", PreReleasesExclude, PreReleasesExclude, []string{"1.0", "2.0a1"}, []string{"1.0"}},
		{">=1.0", PreReleasesAuto, PreReleasesAuto, []string{"not a valid version"}, nil},
		{">=1.0", PreReleasesAuto, PreReleasesAuto, []string{"1.0", "not a valid version"}, []string{"1.0"}},
		{"===foobar", PreReleasesAuto, PreReleasesAuto, []string{"foobar", "foo", "bar"}, []string{"foobar"}},
		{"===foobar", PreReleasesAuto, PreReleasesAuto, []string{"foo", "bar"}, nil},
		{"===1.0", PreReleasesAuto, PreReleasesAuto, []string{"1.0", "1.0.0", "2.0"}, []string{"1.0"}},
		{"===1.0", PreReleasesAuto, PreReleasesAuto, []string{"1.0", "1.0+downstream1"}, []string{"1.0"}},
		{"===foobar", PreReleasesAuto, PreReleasesAuto, []string{"foobar", "1.0", "2.0a1", "invalid"}, []string{"foobar"}},
		{"===1.0", PreReleasesAuto, PreReleasesAuto, []string{"1.0", "foobar", "invalid", "1.0.0"}, []string{"1.0"}},
		{"!=1.0", PreReleasesAuto, PreReleasesAuto, []string{"invalid", "foobar"}, nil},
		{"!=1.0", PreReleasesAuto, PreReleasesAuto, []string{"1.0", "invalid", "2.0"}, []string{"2.0"}},
		{"!=2.0.*", PreReleasesAuto, PreReleasesAuto, []string{"invalid", "foobar", "2.0"}, nil},
		{"!=2.0.*", PreReleasesAuto, PreReleasesAuto, []string{"1.0", "invalid", "2.0.0"}, []string{"1.0"}},
		{"!=1.0", PreReleasesAuto, PreReleasesInclude, []string{"invalid", "foobar"}, nil},
		{"!=1.0", PreReleasesAuto, PreReleasesExclude, []string{"invalid", "foobar"}, nil},
		{"!=1.0", PreReleasesInclude, PreReleasesAuto, []string{"invalid", "foobar"}, nil},
		{"!=1.0", PreReleasesExclude, PreReleasesAuto, []string{"invalid", "foobar"}, nil},
		{"!=1.0", PreReleasesInclude, PreReleasesInclude, []string{"invalid", "foobar"}, nil},
		{"!=1.0", PreReleasesExclude, PreReleasesExclude, []string{"invalid", "foobar"}, nil},
		{"===foobar", PreReleasesAuto, PreReleasesInclude, []string{"foobar", "foo"}, []string{"foobar"}},
		{"===foobar", PreReleasesAuto, PreReleasesExclude, []string{"foobar", "foo"}, []string{"foobar"}},
		{"===foobar", PreReleasesInclude, PreReleasesAuto, []string{"foobar", "foo"}, []string{"foobar"}},
		{"===foobar", PreReleasesExclude, PreReleasesAuto, []string{"foobar", "foo"}, []string{"foobar"}},
		{"===foobar", PreReleasesInclude, PreReleasesInclude, []string{"foobar", "foo"}, []string{"foobar"}},
		{"===foobar", PreReleasesExclude, PreReleasesExclude, []string{"foobar", "foo"}, []string{"foobar"}},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s__%s__%s__%v", tt.spec, tt.specPre, tt.callPre, tt.input), func(t *testing.T) {
			s, err := NewSpecifier(tt.spec, WithPreReleases(tt.specPre))
			require.NoError(t, err)
			got := s.Filter(tt.input, WithPreReleases(tt.callPre))
			if len(tt.want) == 0 {
				assert.Empty(t, got)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

// Ported from
// TestSpecifier.test_specifier_filter_arbitrary_equality_normalization.
// Upstream mixes plain strings and Version objects in ONE iterable; Go is
// typed, so each case is split into its string half and its Version half.
func TestConformance_SpecifierFilterArbitraryNormalization(t *testing.T) {
	type variant struct {
		input []string
		want  []string
	}
	tests := []struct {
		spec    string
		strings variant
		vers    variant
	}{
		{"===1.01", variant{[]string{"1.01", "1.1", "1.0.1"}, []string{"1.01"}}, variant{nil, nil}},
		{"===1.1", variant{[]string{"1.01", "1.1"}, []string{"1.1"}}, variant{nil, nil}},
		{"===1.1", variant{nil, nil}, variant{[]string{"1.1", "1.1"}, []string{"1.1", "1.1"}}},
		{"===1.01", variant{nil, nil}, variant{[]string{"1.1", "1.1"}, nil}},
		{"===1.1", variant{[]string{"1.01", "1.1"}, []string{"1.1"}}, variant{[]string{"1.1"}, []string{"1.1"}}},
		{"===1.01", variant{[]string{"1.01", "1.1"}, []string{"1.01"}}, variant{[]string{"1.1"}, nil}},
		{"===1.a1", variant{[]string{"1.a1", "1a1"}, []string{"1.a1"}}, variant{nil, nil}},
		{"===1a1", variant{[]string{"1.a1", "1a1"}, []string{"1a1"}}, variant{[]string{"1a1"}, []string{"1a1"}}},
	}
	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			s, err := NewSpecifier(tt.spec, WithPreReleases(PreReleasesInclude))
			require.NoError(t, err)

			gotStrings := s.Filter(tt.strings.input)
			if len(tt.strings.want) == 0 {
				assert.Empty(t, gotStrings)
			} else {
				assert.Equal(t, tt.strings.want, gotStrings)
			}

			var vs []Version
			for _, raw := range tt.vers.input {
				v, err := Parse(raw)
				require.NoError(t, err)
				vs = append(vs, v)
			}
			var gotVersions []string
			for _, v := range s.FilterVersions(vs) {
				gotVersions = append(gotVersions, v.String())
			}
			assert.Equal(t, tt.vers.want, gotVersions)
		})
	}
}

// Ported from TestSpecifier.test_length and TestSpecifier.test_iteration.
func TestConformance_SetLengthAndIteration(t *testing.T) {
	tests := []struct {
		spec  string
		len   int
		items []string
	}{
		{"", 0, nil},
		{"==2.0", 1, []string{"==2.0"}},
		{">=2.0", 1, []string{">=2.0"}},
		{">=2.0,<3", 2, []string{">=2.0", "<3"}},
		{">=2.0,<3,==2.4", 3, []string{">=2.0", "<3", "==2.4"}},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.spec), func(t *testing.T) {
			ss, err := NewSpecifiers(tt.spec)
			require.NoError(t, err)
			assert.Equal(t, tt.len, ss.Len())

			var got []string
			for _, s := range ss.List() {
				got = append(got, s.String())
			}
			assert.ElementsMatch(t, tt.items, got)
		})
	}
}

// Ported from TestSpecifier.test_comparison_true / test_comparison_false /
// test_comparison_canonicalizes / test_specifier_equal_for_compatible_operator.
//
// Upstream's tables are the full SPECIFIERS cross-product asserted through
// operator.eq and operator.ne; expressed here as one Equal matrix, which is
// the same 162 assertions without the Python operator plumbing.
func TestConformance_SpecifierEqualityMatrix(t *testing.T) {
	for i, left := range conformanceValidSpecifiers {
		for j, right := range conformanceValidSpecifiers {
			t.Run(left+"__"+right, func(t *testing.T) {
				l, err := NewSpecifier(left)
				require.NoError(t, err)
				r, err := NewSpecifier(right)
				require.NoError(t, err)
				assert.Equal(t, i == j, l.Equal(r))
			})
		}
	}
}

func TestConformance_SpecifierEqualityCanonicalizes(t *testing.T) {
	tests := []struct {
		left, right string
		want        bool
	}{
		{"==2.8.0", "==2.8", true},
		// From test_specifier_equal_for_compatible_operator: trailing zeros are
		// load-bearing under "~=", so these are NOT equal.
		{"~=1.18.0", "~=1.18", false},
	}
	for _, tt := range tests {
		t.Run(tt.left+"__"+tt.right, func(t *testing.T) {
			l, err := NewSpecifier(tt.left)
			require.NoError(t, err)
			r, err := NewSpecifier(tt.right)
			require.NoError(t, err)
			assert.Equal(t, tt.want, l.Equal(r))
		})
	}
}
