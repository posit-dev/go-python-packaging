// SPDX-License-Identifier: Apache-2.0 OR MIT

package reqtxt

import (
	"errors"
	"testing"

	"github.com/posit-dev/go-python-packaging/requirement"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_Requirement(t *testing.T) {
	f, err := Parse("flask>=2.0")
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)

	re, ok := f.Entries[0].(*RequirementEntry)
	require.True(t, ok, "want *RequirementEntry, got %T", f.Entries[0])
	assert.Equal(t, "flask", re.Requirement.Name)
}

func TestParse_Include(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    IncludeEntry
	}{
		{name: "-r space form", content: "-r base.txt", want: IncludeEntry{Path: "base.txt"}},
		{name: "-c space form is a constraint", content: "-c c.txt", want: IncludeEntry{Path: "c.txt", Constraint: true}},
		{name: "--requirement= form", content: "--requirement=x.txt", want: IncludeEntry{Path: "x.txt"}},
		{name: "--constraint= form is a constraint", content: "--constraint=x.txt", want: IncludeEntry{Path: "x.txt", Constraint: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, err := Parse(c.content)
			require.NoError(t, err)
			require.Len(t, f.Entries, 1)

			ie, ok := f.Entries[0].(*IncludeEntry)
			require.True(t, ok, "want *IncludeEntry, got %T", f.Entries[0])
			assert.Equal(t, c.want, *ie)
		})
	}
}

func TestParse_Editable(t *testing.T) {
	f, err := Parse("-e ./pkg")
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)

	ue, ok := f.Entries[0].(*UnnamedEntry)
	require.True(t, ok, "want *UnnamedEntry, got %T", f.Entries[0])
	assert.True(t, ue.Editable)
	assert.Equal(t, KindLocalPath, ue.Kind)
	assert.Equal(t, "./pkg", ue.Raw)
	assert.Empty(t, ue.EggName)

	f, err = Parse("-e git+https://h/p#egg=n")
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)

	ue, ok = f.Entries[0].(*UnnamedEntry)
	require.True(t, ok, "want *UnnamedEntry, got %T", f.Entries[0])
	assert.True(t, ue.Editable)
	assert.Equal(t, KindVCS, ue.Kind)
	assert.Equal(t, "n", ue.EggName)
}

func TestParse_FileOptions(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    []OptionEntry
	}{
		{
			name:    "--index-url space form",
			content: "--index-url https://i/simple",
			want:    []OptionEntry{{Name: "--index-url", Value: "https://i/simple"}},
		},
		{
			name:    "-i short form normalizes to --index-url",
			content: "-i https://i/simple",
			want:    []OptionEntry{{Name: "--index-url", Value: "https://i/simple"}},
		},
		{
			name:    "--index-url= form",
			content: "--index-url=https://i",
			want:    []OptionEntry{{Name: "--index-url", Value: "https://i"}},
		},
		{
			name:    "--no-index is boolean",
			content: "--no-index",
			want:    []OptionEntry{{Name: "--no-index", Value: ""}},
		},
		{
			name:    "--find-links repeated across lines",
			content: "--find-links a\n--find-links b",
			want:    []OptionEntry{{Name: "--find-links", Value: "a"}, {Name: "--find-links", Value: "b"}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, err := Parse(c.content)
			require.NoError(t, err)
			require.Len(t, f.Entries, len(c.want))

			for i, e := range f.Entries {
				oe, ok := e.(*OptionEntry)
				require.True(t, ok, "entry %d: want *OptionEntry, got %T", i, e)
				assert.Equal(t, c.want[i], *oe)
			}
		})
	}
}

func TestParse_UnknownFlag(t *testing.T) {
	f, err := Parse("--frobnicate=1")
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)

	oe, ok := f.Entries[0].(*OptionEntry)
	require.True(t, ok, "want *OptionEntry, got %T", f.Entries[0])
	assert.Equal(t, OptionEntry{Name: "--frobnicate", Value: "1"}, *oe)

	f, err = Parse("--frob foo")
	require.NoError(t, err)
	require.Len(t, f.Entries, 2)

	oe, ok = f.Entries[0].(*OptionEntry)
	require.True(t, ok, "entry 0: want *OptionEntry, got %T", f.Entries[0])
	assert.Equal(t, OptionEntry{Name: "--frob", Value: ""}, *oe)

	re, ok := f.Entries[1].(*RequirementEntry)
	require.True(t, ok, "entry 1: want *RequirementEntry, got %T", f.Entries[1])
	assert.Equal(t, "foo", re.Requirement.Name)
}

func TestParse_Hashes(t *testing.T) {
	f, err := Parse("requests==2.0 --hash=sha256:abc --hash sha256:def")
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)

	re, ok := f.Entries[0].(*RequirementEntry)
	require.True(t, ok, "want *RequirementEntry, got %T", f.Entries[0])
	require.Len(t, re.Hashes, 2)
	assert.Equal(t, Hash{Algorithm: "sha256", Digest: "abc"}, re.Hashes[0])
	assert.Equal(t, Hash{Algorithm: "sha256", Digest: "def"}, re.Hashes[1])

	_, err = Parse("foo --hash=bad")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidRequirementsFile)

	_, err = Parse("--hash=sha256:x")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidRequirementsFile)
}

func TestParse_ConfigSettings(t *testing.T) {
	f, err := Parse("foo==1.0 --config-settings=x=y")
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)

	re, ok := f.Entries[0].(*RequirementEntry)
	require.True(t, ok, "want *RequirementEntry, got %T", f.Entries[0])
	require.Len(t, re.Options, 1)
	assert.Equal(t, OptionEntry{Name: "--config-settings", Value: "x=y"}, re.Options[0])
}

func TestParse_BareArchiveIsUnnamed(t *testing.T) {
	f, err := Parse("foo-1.0.tar.gz")
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)

	ue, ok := f.Entries[0].(*UnnamedEntry)
	require.True(t, ok, "want *UnnamedEntry, got %T", f.Entries[0])
	assert.Equal(t, KindLocalPath, ue.Kind)
	assert.Equal(t, "foo-1.0.tar.gz", ue.Raw)
}

func TestParse_Env(t *testing.T) {
	lookup := func(name string) (string, bool) {
		if name == "TOKEN" {
			return "t", true
		}
		return "", false
	}

	f, err := Parse("--index-url https://${TOKEN}@h/s", WithEnv(lookup))
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)
	oe, ok := f.Entries[0].(*OptionEntry)
	require.True(t, ok, "want *OptionEntry, got %T", f.Entries[0])
	assert.Equal(t, "https://t@h/s", oe.Value)

	f, err = Parse("--index-url https://${TOKEN}@h/s")
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)
	oe, ok = f.Entries[0].(*OptionEntry)
	require.True(t, ok, "want *OptionEntry, got %T", f.Entries[0])
	assert.Equal(t, "https://${TOKEN}@h/s", oe.Value)
}

func TestParse_ErrorLineNumber(t *testing.T) {
	// Line 1 is a comment, line 2 is blank, so the bad requirement is the
	// logical line whose *physical* line number is 3.
	content := "# comment\n\nfoo[bad\n"
	_, err := Parse(content)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidRequirementsFile)
	assert.ErrorIs(t, err, requirement.ErrInvalidRequirement)
	assert.Contains(t, err.Error(), "line 3")
}

func TestParse_ErrorIsErrorsIsCompatible(t *testing.T) {
	_, err := Parse("foo[bad")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidRequirementsFile))
	assert.True(t, errors.Is(err, requirement.ErrInvalidRequirement))
}
