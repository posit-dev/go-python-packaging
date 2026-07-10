// SPDX-License-Identifier: Apache-2.0 OR MIT

package reqtxt

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mapOpener returns an opener (Flatten's "open" parameter) backed by m: a
// lookup for path returns m[path], or an error if path isn't a key.
func mapOpener(m map[string][]byte) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		content, ok := m[path]
		if !ok {
			return nil, fmt.Errorf("mapOpener: no such file %q", path)
		}
		return content, nil
	}
}

func TestFlatten_NestedRequirement(t *testing.T) {
	f, err := Flatten("root.txt", mapOpener(map[string][]byte{
		"root.txt": []byte("-r a.txt\n"),
		"a.txt":    []byte("foo\n"),
	}))
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)

	re, ok := f.Entries[0].(*RequirementEntry)
	require.True(t, ok, "want *RequirementEntry, got %T", f.Entries[0])
	assert.Equal(t, "foo", re.Requirement.Name)
	assert.False(t, re.Constraint)

	assert.Empty(t, f.Includes(), "IncludeEntry should be dropped, not spliced in")
}

func TestFlatten_RelativeSubdir(t *testing.T) {
	var openedPaths []string
	base := mapOpener(map[string][]byte{
		"root.txt":  []byte("-r sub/a.txt\n"),
		"sub/a.txt": []byte("-r b.txt\n"),
		"sub/b.txt": []byte("bar\n"),
	})
	open := func(path string) ([]byte, error) {
		openedPaths = append(openedPaths, path)
		return base(path)
	}

	f, err := Flatten("root.txt", open)
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)

	re, ok := f.Entries[0].(*RequirementEntry)
	require.True(t, ok, "want *RequirementEntry, got %T", f.Entries[0])
	assert.Equal(t, "bar", re.Requirement.Name)

	// b.txt is referenced bare from sub/a.txt; it must resolve against
	// sub/a.txt's directory ("sub"), not root.txt's directory (".").
	assert.Contains(t, openedPaths, "sub/b.txt")
}

func TestFlatten_AbsoluteIncludeVerbatim(t *testing.T) {
	var openedPaths []string
	base := mapOpener(map[string][]byte{
		"root.txt":   []byte("-r /abs/x.txt\n"),
		"/abs/x.txt": []byte("foo\n"),
	})
	open := func(path string) ([]byte, error) {
		openedPaths = append(openedPaths, path)
		return base(path)
	}

	f, err := Flatten("root.txt", open)
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)
	assert.Contains(t, openedPaths, "/abs/x.txt")
}

func TestFlatten_URLIncludeVerbatim(t *testing.T) {
	var openedPaths []string
	base := mapOpener(map[string][]byte{
		"root.txt":        []byte("-r https://h/x.txt\n"),
		"https://h/x.txt": []byte("foo\n"),
	})
	open := func(path string) ([]byte, error) {
		openedPaths = append(openedPaths, path)
		return base(path)
	}

	f, err := Flatten("root.txt", open)
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)
	assert.Contains(t, openedPaths, "https://h/x.txt")
}

func TestFlatten_ConstraintPropagation(t *testing.T) {
	f, err := Flatten("root.txt", mapOpener(map[string][]byte{
		"root.txt":        []byte("-c constraints.txt\n-r r.txt\n"),
		"constraints.txt": []byte("bar==1\n"),
		"r.txt":           []byte("baz\n"),
	}))
	require.NoError(t, err)
	require.Len(t, f.Entries, 2)

	byName := map[string]*RequirementEntry{}
	for _, e := range f.Entries {
		re, ok := e.(*RequirementEntry)
		require.True(t, ok, "want *RequirementEntry, got %T", e)
		byName[re.Requirement.Name] = re
	}

	require.Contains(t, byName, "bar")
	assert.True(t, byName["bar"].Constraint, "bar came in via -c, should be a constraint")

	require.Contains(t, byName, "baz")
	assert.False(t, byName["baz"].Constraint, "baz came in via -r, should not be a constraint")
}

func TestFlatten_NestedRequirementUnderConstraint(t *testing.T) {
	// Design/task-spec default: transitive closure. Once the walk crosses
	// a -c edge, constraint-ness stays true for everything below,
	// including a further -r nested inside the -c-included file. See the
	// code comment on walk's childConstraint computation in flatten.go
	// for the verified pip divergence.
	f, err := Flatten("root.txt", mapOpener(map[string][]byte{
		"root.txt": []byte("-c c.txt\n"),
		"c.txt":    []byte("-r more.txt\n"),
		"more.txt": []byte("qux\n"),
	}))
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)

	re, ok := f.Entries[0].(*RequirementEntry)
	require.True(t, ok, "want *RequirementEntry, got %T", f.Entries[0])
	assert.Equal(t, "qux", re.Requirement.Name)
	assert.True(t, re.Constraint, "qux is nested under a -c include, transitive-closure default => Constraint true")
}

func TestFlatten_DoubleInclusionNoDedup(t *testing.T) {
	f, err := Flatten("root.txt", mapOpener(map[string][]byte{
		"root.txt": []byte("-r a.txt\n-c a.txt\n"),
		"a.txt":    []byte("foo\n"),
	}))
	require.NoError(t, err)
	require.Len(t, f.Entries, 2, "no dedup: the same file included twice yields two entries")

	var sawFalse, sawTrue bool
	for _, e := range f.Entries {
		re, ok := e.(*RequirementEntry)
		require.True(t, ok, "want *RequirementEntry, got %T", e)
		assert.Equal(t, "foo", re.Requirement.Name)
		if re.Constraint {
			sawTrue = true
		} else {
			sawFalse = true
		}
	}
	assert.True(t, sawFalse, "expected one non-constraint foo (via -r)")
	assert.True(t, sawTrue, "expected one constraint foo (via -c)")
}

func TestFlatten_Cycle(t *testing.T) {
	_, err := Flatten("root.txt", mapOpener(map[string][]byte{
		"root.txt": []byte("-r a.txt\n"),
		"a.txt":    []byte("-r b.txt\n"),
		"b.txt":    []byte("-r a.txt\n"),
	}))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidRequirementsFile))
}

func TestFlatten_OpenerErrorWrapped(t *testing.T) {
	_, err := Flatten("root.txt", mapOpener(map[string][]byte{
		"root.txt": []byte("-r missing.txt\n"),
	}))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidRequirementsFile))
}

func TestFlatten_BOMStripped(t *testing.T) {
	content := append([]byte{0xEF, 0xBB, 0xBF}, []byte("flask\n")...)
	f, err := Flatten("root.txt", mapOpener(map[string][]byte{
		"root.txt": content,
	}))
	require.NoError(t, err)
	require.Len(t, f.Entries, 1)

	re, ok := f.Entries[0].(*RequirementEntry)
	require.True(t, ok, "want *RequirementEntry, got %T", f.Entries[0])
	assert.Equal(t, "flask", re.Requirement.Name)
}

func TestFlatten_RootLevelOpenerError(t *testing.T) {
	_, err := Flatten("nonexistent.txt", mapOpener(map[string][]byte{}))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidRequirementsFile))
}
