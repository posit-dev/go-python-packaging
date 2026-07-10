// SPDX-License-Identifier: Apache-2.0 OR MIT

package reqtxt

import (
	"testing"

	"github.com/posit-dev/go-python-packaging/requirement"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildMixedFile constructs a File with one of each Entry kind, plus extra
// OptionEntry/RequirementEntry/IncludeEntry values interleaved, so ordering
// and filtering can be asserted precisely.
func buildMixedFile(t *testing.T) *File {
	t.Helper()

	flaskReq, err := requirement.Parse("flask")
	require.NoError(t, err)
	requestsReq, err := requirement.Parse("requests")
	require.NoError(t, err)

	reqA := &RequirementEntry{Requirement: flaskReq}
	incA := &IncludeEntry{Path: "base.txt"}
	optIndex1 := &OptionEntry{Name: "--index-url", Value: "https://example.com/simple"}
	unnamed := &UnnamedEntry{Raw: "./local-pkg", Kind: KindLocalPath}
	reqB := &RequirementEntry{Requirement: requestsReq, Constraint: true}
	incB := &IncludeEntry{Path: "constraints.txt", Constraint: true}
	optIndex2 := &OptionEntry{Name: "--index-url", Value: "https://example.com/simple2"}
	optExtra1 := &OptionEntry{Name: "--extra-index-url", Value: "https://extra1.example.com"}
	optExtra2 := &OptionEntry{Name: "--extra-index-url", Value: "https://extra2.example.com"}
	optFindLinks1 := &OptionEntry{Name: "--find-links", Value: "./wheels"}
	optFindLinks2 := &OptionEntry{Name: "--find-links", Value: "./more-wheels"}
	optTrusted1 := &OptionEntry{Name: "--trusted-host", Value: "example.com"}
	optTrusted2 := &OptionEntry{Name: "--trusted-host", Value: "example.org"}
	optNoBinary1 := &OptionEntry{Name: "--no-binary", Value: ":all:"}
	optNoBinary2 := &OptionEntry{Name: "--no-binary", Value: "foo"}
	optOnlyBinary1 := &OptionEntry{Name: "--only-binary", Value: "bar"}
	optOnlyBinary2 := &OptionEntry{Name: "--only-binary", Value: ":none:"}
	optNoIndex := &OptionEntry{Name: "--no-index"}
	optRequireHashes := &OptionEntry{Name: "--require-hashes"}
	optPre := &OptionEntry{Name: "--pre"}
	optPreferBinary := &OptionEntry{Name: "--prefer-binary"}

	return &File{
		Entries: []Entry{
			reqA,
			incA,
			optIndex1,
			unnamed,
			reqB,
			incB,
			optIndex2,
			optExtra1,
			optExtra2,
			optFindLinks1,
			optFindLinks2,
			optTrusted1,
			optTrusted2,
			optNoBinary1,
			optNoBinary2,
			optOnlyBinary1,
			optOnlyBinary2,
			optNoIndex,
			optRequireHashes,
			optPre,
			optPreferBinary,
		},
	}
}

func TestFile_Requirements(t *testing.T) {
	f := buildMixedFile(t)
	got := f.Requirements()
	require.Len(t, got, 2)
	assert.Equal(t, "flask", got[0].Requirement.Name)
	assert.False(t, got[0].Constraint)
	assert.Equal(t, "requests", got[1].Requirement.Name)
	assert.True(t, got[1].Constraint)
}

func TestFile_Includes(t *testing.T) {
	f := buildMixedFile(t)
	got := f.Includes()
	require.Len(t, got, 2)
	assert.Equal(t, "base.txt", got[0].Path)
	assert.False(t, got[0].Constraint)
	assert.Equal(t, "constraints.txt", got[1].Path)
	assert.True(t, got[1].Constraint)
}

func TestFile_Options(t *testing.T) {
	f := buildMixedFile(t)
	got := f.Options()
	// 16 OptionEntry values were added to the mixed file.
	require.Len(t, got, 16)
	assert.Equal(t, "--index-url", got[0].Name)
	assert.Equal(t, "https://example.com/simple", got[0].Value)
	assert.Equal(t, "--prefer-binary", got[len(got)-1].Name)
}

func TestFile_Requirements_Empty(t *testing.T) {
	f := &File{}
	assert.Empty(t, f.Requirements())
}

func TestFile_Includes_Empty(t *testing.T) {
	f := &File{}
	assert.Empty(t, f.Includes())
}

func TestFile_Options_Empty(t *testing.T) {
	f := &File{}
	assert.Empty(t, f.Options())
}

func TestFile_IndexURL_LastWins(t *testing.T) {
	f := buildMixedFile(t)
	got, ok := f.IndexURL()
	assert.True(t, ok)
	assert.Equal(t, "https://example.com/simple2", got)
}

func TestFile_IndexURL_Absent(t *testing.T) {
	f := &File{}
	got, ok := f.IndexURL()
	assert.False(t, ok)
	assert.Empty(t, got)
}

func TestFile_ExtraIndexURLs_Accumulates(t *testing.T) {
	f := buildMixedFile(t)
	assert.Equal(t, []string{"https://extra1.example.com", "https://extra2.example.com"}, f.ExtraIndexURLs())
}

func TestFile_ExtraIndexURLs_Absent(t *testing.T) {
	f := &File{}
	assert.Empty(t, f.ExtraIndexURLs())
}

func TestFile_FindLinks_Accumulates(t *testing.T) {
	f := buildMixedFile(t)
	assert.Equal(t, []string{"./wheels", "./more-wheels"}, f.FindLinks())
}

func TestFile_TrustedHosts_Accumulates(t *testing.T) {
	f := buildMixedFile(t)
	assert.Equal(t, []string{"example.com", "example.org"}, f.TrustedHosts())
}

func TestFile_NoBinary_Accumulates(t *testing.T) {
	f := buildMixedFile(t)
	assert.Equal(t, []string{":all:", "foo"}, f.NoBinary())
}

func TestFile_OnlyBinary_Accumulates(t *testing.T) {
	f := buildMixedFile(t)
	assert.Equal(t, []string{"bar", ":none:"}, f.OnlyBinary())
}

func TestFile_NoIndex(t *testing.T) {
	assert.True(t, buildMixedFile(t).NoIndex())
	assert.False(t, (&File{}).NoIndex())
}

func TestFile_RequireHashes(t *testing.T) {
	assert.True(t, buildMixedFile(t).RequireHashes())
	assert.False(t, (&File{}).RequireHashes())
}

func TestFile_Pre(t *testing.T) {
	assert.True(t, buildMixedFile(t).Pre())
	assert.False(t, (&File{}).Pre())
}

func TestFile_PreferBinary(t *testing.T) {
	assert.True(t, buildMixedFile(t).PreferBinary())
	assert.False(t, (&File{}).PreferBinary())
}

func TestKind_String(t *testing.T) {
	cases := []struct {
		kind Kind
		want string
	}{
		{KindVCS, "vcs"},
		{KindURL, "url"},
		{KindLocalPath, "local-path"},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			assert.Equal(t, c.want, c.kind.String())
		})
	}
}

func TestEntry_PointerReceivers(t *testing.T) {
	// Compile-time-ish assertion: each concrete entry type satisfies Entry
	// only via a pointer receiver.
	var entries []Entry
	entries = append(entries, &RequirementEntry{})
	entries = append(entries, &IncludeEntry{})
	entries = append(entries, &UnnamedEntry{})
	entries = append(entries, &OptionEntry{})
	assert.Len(t, entries, 4)
}

func TestErrInvalidRequirementsFile(t *testing.T) {
	require.Error(t, ErrInvalidRequirementsFile)
	assert.Equal(t, "invalid requirements file", ErrInvalidRequirementsFile.Error())
}
