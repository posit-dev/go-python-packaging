// SPDX-License-Identifier: Apache-2.0 OR MIT

package reqtxt

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// reparse is a small test helper: it renders f via String(), re-parses the
// result, and returns the reparsed File, so tests can assert the two Files
// are equivalent entry-by-entry.
func reparse(t *testing.T, f *File) *File {
	t.Helper()
	rendered := f.String()
	f2, err := Parse(rendered)
	require.NoError(t, err, "reparsing rendered content failed: %q", rendered)
	return f2
}

func TestFileString_RequirementWithHashesAndMarker(t *testing.T) {
	f, err := Parse(`requests[security]>=2.0 ; python_version >= "3.8" --hash=sha256:aaa --hash=sha256:bbb`)
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)

	f2 := reparse(t, f)
	require.Len(t, f2.Entries, 1)

	want, ok := f.Entries[0].(*RequirementEntry)
	require.True(t, ok)
	got, ok := f2.Entries[0].(*RequirementEntry)
	require.True(t, ok)

	assert.Equal(t, want.Requirement.Name, got.Requirement.Name)
	assert.Equal(t, want.Requirement.Extras, got.Requirement.Extras)
	assert.Equal(t, want.Requirement.Specifiers.String(), got.Requirement.Specifiers.String())
	assert.Equal(t, want.Requirement.Marker.String(), got.Requirement.Marker.String())
	assert.False(t, got.Requirement.Marker.IsEmpty())
	assert.Equal(t, want.Hashes, got.Hashes)
	require.Len(t, got.Hashes, 2)
}

func TestFileString_EditableVCSWithEgg(t *testing.T) {
	f, err := Parse("-e git+https://h/p#egg=n")
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)

	f2 := reparse(t, f)
	require.Len(t, f2.Entries, 1)

	want, ok := f.Entries[0].(*UnnamedEntry)
	require.True(t, ok)
	got, ok := f2.Entries[0].(*UnnamedEntry)
	require.True(t, ok)

	assert.True(t, got.Editable)
	assert.Equal(t, KindVCS, got.Kind)
	assert.Equal(t, "n", got.EggName)
	assert.Equal(t, want.Raw, got.Raw)
	assert.Equal(t, want.EggName, got.EggName)
	assert.Equal(t, want.Kind, got.Kind)
	assert.Equal(t, want.Editable, got.Editable)
}

func TestFileString_Includes(t *testing.T) {
	f, err := Parse("-r base.txt\n-c constraints.txt")
	require.NoError(t, err)
	require.Len(t, f.Entries, 2)

	f2 := reparse(t, f)
	require.Len(t, f2.Entries, 2)

	incR, ok := f2.Entries[0].(*IncludeEntry)
	require.True(t, ok)
	assert.Equal(t, "base.txt", incR.Path)
	assert.False(t, incR.Constraint)

	incC, ok := f2.Entries[1].(*IncludeEntry)
	require.True(t, ok)
	assert.Equal(t, "constraints.txt", incC.Path)
	assert.True(t, incC.Constraint)
}

func TestFileString_FileOptions(t *testing.T) {
	f, err := Parse("--index-url https://i/simple\n--no-index\n--find-links x\n--find-links y")
	require.NoError(t, err)

	f2 := reparse(t, f)

	indexURL, ok := f2.IndexURL()
	require.True(t, ok)
	assert.Equal(t, "https://i/simple", indexURL)
	assert.True(t, f2.NoIndex())
	assert.Equal(t, []string{"x", "y"}, f2.FindLinks())
}

func TestFileString_UnknownFileOptionWithValue(t *testing.T) {
	// Regression test: an unknown file-level option with a value must
	// round-trip through the "=" form. A bare "--cert /etc/ssl/ca.pem"
	// reparse would silently corrupt this into a boolean "--cert" plus a
	// phantom UnnamedEntry for "/etc/ssl/ca.pem" (dispatchFileOption only
	// recovers a value for an unrecognized flag from the inline "=value"
	// form).
	f, err := Parse("--cert=/etc/ssl/ca.pem")
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)

	f2 := reparse(t, f)
	require.Len(t, f2.Entries, 1, "expected exactly one entry, no phantom entry from misparsed value")

	got, ok := f2.Entries[0].(*OptionEntry)
	require.True(t, ok)
	assert.Equal(t, "--cert", got.Name)
	assert.Equal(t, "/etc/ssl/ca.pem", got.Value)
}

func TestFileString_KnownFileOptionValueWithSpace(t *testing.T) {
	f, err := Parse(`--find-links "/opt/my links"`)
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)

	f2 := reparse(t, f)
	require.Len(t, f2.Entries, 1)

	assert.Equal(t, []string{"/opt/my links"}, f2.FindLinks())
}

func TestFileString_PerLineOptionRoundTrip(t *testing.T) {
	f, err := Parse(`flask --config-settings=key=val`)
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)

	f2 := reparse(t, f)
	require.Len(t, f2.Entries, 1)

	got, ok := f2.Entries[0].(*RequirementEntry)
	require.True(t, ok)
	require.Len(t, got.Options, 1)
	assert.Equal(t, "--config-settings", got.Options[0].Name)
	assert.Equal(t, "key=val", got.Options[0].Value)
}

func TestFileString_EmptyValueOptionRoundTrips(t *testing.T) {
	// Regression test: a value-taking option given an explicitly empty
	// value (e.g. "--index-url=") must round-trip through the explicit
	// '<name>=""' form. Rendering it as a bare "--index-url" would reparse
	// as arity-1 with no value supplied at all, which is an error (the
	// option "requires a value"), so Parse(f.String()) would fail.
	f, err := Parse("--index-url=")
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)

	rendered := f.String()
	f2, err := Parse(rendered)
	require.NoError(t, err, "reparsing rendered content failed: %q", rendered)
	require.Len(t, f2.Entries, 1)

	got, ok := f2.Entries[0].(*OptionEntry)
	require.True(t, ok)
	assert.Equal(t, "--index-url", got.Name)
	assert.Equal(t, "", got.Value)
}

func TestFileString_EmptyValuePerLineOptionRoundTrips(t *testing.T) {
	f, err := Parse("flask --global-option=")
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)

	rendered := f.String()
	f2, err := Parse(rendered)
	require.NoError(t, err, "reparsing rendered content failed: %q", rendered)
	require.Len(t, f2.Entries, 1)

	got, ok := f2.Entries[0].(*RequirementEntry)
	require.True(t, ok)
	require.Len(t, got.Options, 1)
	assert.Equal(t, "--global-option", got.Options[0].Name)
	assert.Equal(t, "", got.Options[0].Value)
}

func TestFileString_BooleanOptionStillRendersBare(t *testing.T) {
	// A genuinely boolean (arity-0) option with an empty Value must still
	// render as a bare flag, not "--no-index=\"\"": only value-taking
	// options need the explicit empty-value form.
	f, err := Parse("--no-index")
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)

	rendered := f.String()
	assert.Equal(t, "--no-index", rendered)

	f2, err := Parse(rendered)
	require.NoError(t, err)
	require.Len(t, f2.Entries, 1)
	assert.True(t, f2.NoIndex())
}

func TestFileString_UnnamedArchive(t *testing.T) {
	f, err := Parse("foo-1.0.tar.gz")
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)

	f2 := reparse(t, f)
	require.Len(t, f2.Entries, 1)

	got, ok := f2.Entries[0].(*UnnamedEntry)
	require.True(t, ok)
	assert.Equal(t, "foo-1.0.tar.gz", got.Raw)
	assert.Equal(t, KindLocalPath, got.Kind)
	assert.False(t, got.Editable)
}

func TestFileString_CommentsDropped(t *testing.T) {
	f, err := Parse("# c\nflask")
	require.NoError(t, err)

	rendered := f.String()
	assert.Contains(t, rendered, "flask")
	assert.NotContains(t, rendered, "# c")
	assert.NotContains(t, rendered, "#")
}

func TestFileString_JoinsWithNewline(t *testing.T) {
	f, err := Parse("flask\nrequests")
	require.NoError(t, err)
	require.Len(t, f.Entries, 2)

	rendered := f.String()
	lines := strings.Split(rendered, "\n")
	require.Len(t, lines, 2)
	assert.True(t, strings.HasPrefix(lines[0], "flask"))
	assert.True(t, strings.HasPrefix(lines[1], "requests"))
}
