// SPDX-License-Identifier: Apache-2.0 OR MIT
package reqtxt

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openMap serves a fixed set of files, so include trees are testable without
// touching disk.
func openMap(files map[string]string) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		content, ok := files[path]
		if !ok {
			return nil, fmt.Errorf("no such file: %s", path)
		}
		return []byte(content), nil
	}
}

func TestSource_LineNumbers(t *testing.T) {
	f, err := Parse("--no-index\nrequests==2.0\n\n-r base.txt\n./local/pkg")
	require.NoError(t, err)
	require.Len(t, f.Entries, 4)

	for i, wantLine := range []int{1, 2, 4, 5} {
		assert.Equal(t, wantLine, SourceOf(f.Entries[i]).Line, "entry %d", i)
	}
}

// TestSource_PathEmptyWithoutWithPath documents the default: Parse takes content,
// not a filename, so it cannot invent a path.
func TestSource_PathEmptyWithoutWithPath(t *testing.T) {
	f, err := Parse("requests==2.0")
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)
	assert.Empty(t, SourceOf(f.Entries[0]).Path)

	f, err = Parse("requests==2.0", WithPath("given.txt"))
	require.NoError(t, err)
	assert.Equal(t, "given.txt", SourceOf(f.Entries[0]).Path)
}

// TestSource_FlattenAttributesEachFile is the case the feature exists for: after
// flattening, every entry still names the file it came from, even though the
// IncludeEntry that pulled it in has been consumed.
func TestSource_FlattenAttributesEachFile(t *testing.T) {
	f, err := Flatten("root.txt", openMap(map[string]string{
		"root.txt": "requests==2.0\n-r base.txt\nflask==3.0",
		"base.txt": "urllib3==2.0\n-r deep.txt",
		"deep.txt": "six==1.16",
	}))
	require.NoError(t, err)

	got := map[string]Source{}
	for _, e := range f.Entries {
		re, ok := e.(*RequirementEntry)
		require.True(t, ok, "want only requirements, got %T", e)
		got[re.Requirement.Name] = re.Source
	}

	assert.Equal(t, Source{Path: "root.txt", Line: 1}, got["requests"])
	assert.Equal(t, Source{Path: "base.txt", Line: 1}, got["urllib3"])
	assert.Equal(t, Source{Path: "deep.txt", Line: 1}, got["six"])
	assert.Equal(t, Source{Path: "root.txt", Line: 3}, got["flask"])

	// No IncludeEntry survives flattening, so Source is the only provenance left.
	assert.Empty(t, f.Includes())
}

// TestSource_PreLeaksFromAConstraintsFile is the diagnostic that motivated this.
// File.Pre() is any-wins across the flattened result, so a single "--pre" nested
// inside a "-c" subtree silently changes what a caller collects. Source is what
// lets the caller say WHERE it came from.
func TestSource_PreLeaksFromAConstraintsFile(t *testing.T) {
	f, err := Flatten("root.txt", openMap(map[string]string{
		"root.txt":  "requests==2.0\n-c pins.txt",
		"pins.txt":  "urllib3<2\n-r extra.txt",
		"extra.txt": "# harmless looking\n--pre\nsix==1.16",
	}))
	require.NoError(t, err)

	require.True(t, f.Pre(), "--pre anywhere in the tree flips it")

	// And now it is attributable.
	var where []Source
	for _, o := range f.Options() {
		if o.Name == optPre {
			where = append(where, o.Source)
		}
	}
	require.Len(t, where, 1)
	assert.Equal(t, Source{Path: "extra.txt", Line: 2}, where[0],
		"line 2 because line 1 of extra.txt is a comment")
}

// TestSource_ConstraintPromotionIsAttributable covers the other warning a
// consumer wants: a "-r" nested inside a "-c" file resets constraint-ness, so
// those pins become requirements. Source names the file responsible.
func TestSource_ConstraintPromotionIsAttributable(t *testing.T) {
	f, err := Flatten("root.txt", openMap(map[string]string{
		"root.txt":     "-c pins.txt",
		"pins.txt":     "urllib3<2\n-r promoted.txt",
		"promoted.txt": "six==1.16",
	}))
	require.NoError(t, err)

	byName := map[string]*RequirementEntry{}
	for _, e := range f.Entries {
		if re, ok := e.(*RequirementEntry); ok {
			byName[re.Requirement.Name] = re
		}
	}

	require.Contains(t, byName, "urllib3")
	assert.True(t, byName["urllib3"].Constraint, "reached directly via -c")
	assert.Equal(t, "pins.txt", byName["urllib3"].Source.Path)

	require.Contains(t, byName, "six")
	assert.False(t, byName["six"].Constraint,
		"a -r inside a -c resets constraint-ness, matching pip")
	assert.Equal(t, "promoted.txt", byName["six"].Source.Path,
		"and Source names the file that did it")
}

// TestSource_PerRequirementOptions pins that options gathered from continuation
// lines are attributed to the logical line their requirement started on.
func TestSource_PerRequirementOptions(t *testing.T) {
	f, err := Parse("# leading comment\nfoo==1.0 \\\n    --config-settings=x=y", WithPath("r.txt"))
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)

	re, ok := f.Entries[0].(*RequirementEntry)
	require.True(t, ok)
	require.Len(t, re.Options, 1)

	assert.Equal(t, Source{Path: "r.txt", Line: 2}, re.Source)
	assert.Equal(t, re.Source, re.Options[0].Source,
		"a continuation's option shares its requirement's anchor")
}

// TestSource_CallerWithPathDoesNotOverrideFlatten guards the append order: a
// caller-supplied WithPath must not attribute every included file to one path.
func TestSource_CallerWithPathDoesNotOverrideFlatten(t *testing.T) {
	f, err := Flatten("root.txt", openMap(map[string]string{
		"root.txt": "-r base.txt",
		"base.txt": "six==1.16",
	}), WithPath("caller-supplied.txt"))
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)

	assert.Equal(t, "base.txt", SourceOf(f.Entries[0]).Path)
}

func TestSourceOf_CoversEveryEntryType(t *testing.T) {
	f, err := Parse("--no-index\nrequests==2.0\n-r base.txt\n./local/pkg", WithPath("r.txt"))
	require.NoError(t, err)
	require.Len(t, f.Entries, 4)

	for i, e := range f.Entries {
		src := SourceOf(e)
		assert.Equal(t, "r.txt", src.Path, "entry %d (%T)", i, e)
		assert.NotZero(t, src.Line, "entry %d (%T)", i, e)
	}
}
