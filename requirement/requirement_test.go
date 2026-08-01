// SPDX-License-Identifier: Apache-2.0 OR MIT

package requirement

import (
	"errors"
	"testing"

	"github.com/posit-dev/go-python-packaging/internal/pep508"
	"github.com/posit-dev/go-python-packaging/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- plain name ---

func TestParse_PlainName(t *testing.T) {
	r, err := Parse("foo")
	require.NoError(t, err)
	assert.Equal(t, "foo", r.Name)
	assert.Empty(t, r.Extras)
	assert.Empty(t, r.URL)
	assert.True(t, r.Marker.IsEmpty())
	assert.Empty(t, r.Specifiers.String())
}

// --- extras ---

func TestParse_Extras_Multiple(t *testing.T) {
	r, err := Parse("foo[bar,baz]")
	require.NoError(t, err)
	assert.Equal(t, []string{"bar", "baz"}, r.Extras)
}

func TestParse_Extras_Empty(t *testing.T) {
	r, err := Parse("foo[]")
	require.NoError(t, err)
	assert.Empty(t, r.Extras)
}

func TestParse_Extras_WhitespaceInside(t *testing.T) {
	r, err := Parse("foo[ bar , baz ]")
	require.NoError(t, err)
	assert.Equal(t, []string{"bar", "baz"}, r.Extras)
}

func TestParse_Extras_Normalized(t *testing.T) {
	r, err := Parse("foo[Foo_Bar]")
	require.NoError(t, err)
	assert.Equal(t, []string{"foo-bar"}, r.Extras)
}

// --- version specifier ---

func TestParse_VersionSpec_Bare(t *testing.T) {
	r, err := Parse("foo>=1.0")
	require.NoError(t, err)
	want, err := version.NewSpecifiers(">=1.0")
	require.NoError(t, err)
	assert.Equal(t, want.String(), r.Specifiers.String())
}

func TestParse_VersionSpec_Parenthesized(t *testing.T) {
	r, err := Parse("foo(>=1.0)")
	require.NoError(t, err)
	want, err := version.NewSpecifiers(">=1.0")
	require.NoError(t, err)
	assert.Equal(t, want.String(), r.Specifiers.String())
}

func TestParse_VersionSpec_EmptyParens_IsValid(t *testing.T) {
	// An empty parenthesized specifier is valid and means "no constraint"
	// (pypa/packaging test_empty_specifier).
	r, err := Parse("foo()")
	require.NoError(t, err)
	assert.Equal(t, "foo", r.Name)
	assert.Equal(t, "", r.Specifiers.String(), "empty parens mean no specifier")
}

func TestParse_VersionSpec_MultipleClauses(t *testing.T) {
	r, err := Parse("foo>=1.0,<2.0")
	require.NoError(t, err)
	v, err := version.Parse("1.5")
	require.NoError(t, err)
	assert.True(t, r.Specifiers.Check(v))
	v2, err := version.Parse("2.5")
	require.NoError(t, err)
	assert.False(t, r.Specifiers.Check(v2))
}

// --- grammar asymmetry: no space required before ";" after a name/versionspec ---

func TestParse_NoSpaceBeforeMarker(t *testing.T) {
	r, err := Parse(`foo>=1.0;python_version>="3.8"`)
	require.NoError(t, err)
	assert.False(t, r.Marker.IsEmpty())
	assert.Equal(t, `python_version >= "3.8"`, r.Marker.String())
}

// --- @ url form ---

func TestParse_URL(t *testing.T) {
	r, err := Parse("foo @ https://example.com/foo-1.0.whl")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/foo-1.0.whl", r.URL)
	assert.Empty(t, r.Specifiers.String())
	assert.True(t, r.Marker.IsEmpty())
}

func TestParse_URL_WithMarker(t *testing.T) {
	r, err := Parse(`foo @ https://example.com/foo-1.0.whl ; python_version > "3"`)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/foo-1.0.whl", r.URL)
	assert.False(t, r.Marker.IsEmpty())
}

// --- URL containing ";" (key robustness case) ---

func TestParse_URLContainingSemicolon(t *testing.T) {
	r, err := Parse(`foo @ https://x/y;z.whl ; python_version > "3"`)
	require.NoError(t, err)
	assert.Equal(t, "https://x/y;z.whl", r.URL)
	require.False(t, r.Marker.IsEmpty())
	assert.Equal(t, `python_version > "3"`, r.Marker.String())
}

// --- grammar asymmetry: URL form needs wsp+ (or end) before a marker ---

func TestParse_URLForm_NoSpaceBeforeMarker_SwallowsIntoURL(t *testing.T) {
	r, err := Parse(`foo@https://x/y;python_version>"3"`)
	require.NoError(t, err)
	assert.Equal(t, `https://x/y;python_version>"3"`, r.URL)
	assert.True(t, r.Marker.IsEmpty())
}

// --- distinct error categories ---

func TestParse_MalformedSyntax_IsErrInvalidRequirement(t *testing.T) {
	_, err := Parse("foo[bar")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidRequirement))

	var syntaxErr *pep508.SyntaxError
	assert.True(t, errors.As(err, &syntaxErr), "expected a *pep508.SyntaxError in the chain")
	var markerErr *pep508.MarkerClauseError
	assert.False(t, errors.As(err, &markerErr), "malformed (non-marker) syntax must NOT be a MarkerClauseError")
}

func TestParse_InvalidVersionSpecifier_IsErrInvalidRequirement(t *testing.T) {
	_, err := Parse("foo~=1")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidRequirement))

	var syntaxErr *pep508.SyntaxError
	assert.False(t, errors.As(err, &syntaxErr), "a bad-version error must not be a pep508.SyntaxError")
	var markerErr *pep508.MarkerClauseError
	assert.False(t, errors.As(err, &markerErr))
}

func TestParse_InvalidMarker_IsErrInvalidRequirement(t *testing.T) {
	_, err := Parse(`foo; not_a_real_variable == "1"`)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidRequirement))

	var markerErr *pep508.MarkerClauseError
	require.True(t, errors.As(err, &markerErr), "expected a *pep508.MarkerClauseError in the chain")
	var syntaxErr *pep508.SyntaxError
	assert.True(t, errors.As(err, &syntaxErr), "the marker error should still wrap a *pep508.SyntaxError")
}

func TestParse_ThreeErrorCategoriesAreDistinguishable(t *testing.T) {
	_, malformedErr := Parse("foo[bar")
	_, versionErr := Parse("foo~=1")
	_, markerErr := Parse(`foo; not_a_real_variable == "1"`)

	var mce *pep508.MarkerClauseError
	assert.False(t, errors.As(malformedErr, &mce))
	assert.False(t, errors.As(versionErr, &mce))
	assert.True(t, errors.As(markerErr, &mce))

	var se *pep508.SyntaxError
	assert.True(t, errors.As(malformedErr, &se))
	assert.False(t, errors.As(versionErr, &se))
	assert.True(t, errors.As(markerErr, &se))
}

// --- round trip: Parse(r.String()) is stable ---

func TestParse_RoundTrip(t *testing.T) {
	sources := []string{
		"foo",
		"foo[bar]",
		"foo[bar,baz]",
		"foo>=1.0",
		"foo>=1.0,<2.0",
		`foo>=1.0 ; python_version >= "3.8"`,
		"foo @ https://example.com/foo-1.0.whl",
		`foo @ https://example.com/foo-1.0.whl ; python_version > "3"`,
		`foo @ https://x/y;z.whl ; python_version > "3"`,
		"foo[Foo_Bar]>=1.0",
	}

	for _, src := range sources {
		t.Run(src, func(t *testing.T) {
			r1, err := Parse(src)
			require.NoError(t, err, "parsing %q", src)
			s1 := r1.String()

			r2, err := Parse(s1)
			require.NoError(t, err, "re-parsing serialized form %q", s1)
			s2 := r2.String()

			assert.Equal(t, s1, s2, "serialization must be stable across a re-parse")
			assert.Equal(t, r1.Name, r2.Name)
			assert.Equal(t, r1.Extras, r2.Extras)
			assert.Equal(t, r1.URL, r2.URL)
			assert.Equal(t, r1.Specifiers.String(), r2.Specifiers.String())
			assert.Equal(t, r1.Marker.String(), r2.Marker.String())
		})
	}
}

// --- Marker zero value is always-true ---

func TestParse_NoMarker_IsAlwaysTrueZeroValue(t *testing.T) {
	r, err := Parse("foo")
	require.NoError(t, err)
	assert.True(t, r.Marker.IsEmpty())
	assert.Equal(t, "", r.Marker.String())
}
