// SPDX-License-Identifier: Apache-2.0 OR MIT

package archiver_test

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/posit-dev/go-python-packaging/distribution/internal/archiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildTarGz writes a .tar.gz file at path whose entries are, in order,
// name -> contents from the given slice of pairs.
func buildTarGz(t *testing.T, path string, entries []struct {
	name     string
	contents string
}) {
	t.Helper()

	f, err := os.Create(path)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, f.Close())
	}()

	gzw := gzip.NewWriter(f)
	defer func() {
		require.NoError(t, gzw.Close())
	}()

	tw := tar.NewWriter(gzw)
	defer func() {
		require.NoError(t, tw.Close())
	}()

	for _, e := range entries {
		hdr := &tar.Header{
			Name: e.name,
			Mode: 0o644,
			Size: int64(len(e.contents)),
		}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err := tw.Write([]byte(e.contents))
		require.NoError(t, err)
	}
}

// TestTarReader_ReadFile_FirstEntry is a regression test for a bug where
// NewArchiveReader consumed the first tar header during validation and
// stored the already-advanced reader, causing ReadFile to miss an sdist
// whose PKG-INFO is the very first member of the tarball.
func TestTarReader_ReadFile_FirstEntry(t *testing.T) {
	const pkgInfoContents = "Metadata-Version: 2.1\nName: example\nVersion: 1.0.0\n"

	dir := t.TempDir()
	path := filepath.Join(dir, "example-1.0.0.tar.gz")

	buildTarGz(t, path, []struct {
		name     string
		contents string
	}{
		{name: "PKG-INFO", contents: pkgInfoContents},
		{name: "setup.py", contents: "from setuptools import setup\nsetup()\n"},
	})

	reader, err := archiver.NewArchiveReader(path)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, reader.Close())
	}()

	got, err := reader.ReadFile("PKG-INFO")
	require.NoError(t, err)
	assert.Equal(t, pkgInfoContents, string(got))
}
