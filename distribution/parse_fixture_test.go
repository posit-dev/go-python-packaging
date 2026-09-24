// SPDX-License-Identifier: Apache-2.0 OR MIT

package distribution_test

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	distribution "github.com/posit-dev/go-python-packaging/distribution"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"
)

// These tests synthesize wheels and sdists in-process instead of committing
// real package bytes, so there is no third-party content here (see PR for
// why). All metadata is invented for these tests ("gppfixture-*" names).

// fixtureEntry is one named member of a synthetic archive.
type fixtureEntry struct {
	name     string
	contents string
}

// writeWheelFixture writes a zip archive (a synthetic wheel) at path.
func writeWheelFixture(t *testing.T, path string, entries []fixtureEntry) {
	t.Helper()

	f, err := os.Create(path)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, f.Close())
	}()

	zw := zip.NewWriter(f)
	defer func() {
		require.NoError(t, zw.Close())
	}()

	for _, e := range entries {
		w, err := zw.Create(e.name)
		require.NoError(t, err)
		_, err = w.Write([]byte(e.contents))
		require.NoError(t, err)
	}
}

// writeSDistFixture writes a .tar.gz archive (a synthetic sdist) at path.
func writeSDistFixture(t *testing.T, path string, entries []fixtureEntry) {
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

// fileDigests computes the same two digests NewPackageFile computes over the
// bytes actually written to path, for comparison against PackageFile.SHA2Digest
// and PackageFile.Blake2_256Digest.
func fileDigests(t *testing.T, path string) (sha2Hex string, blake2Hex string) {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	sum := sha256.Sum256(data)

	h, err := blake2b.New256(nil)
	require.NoError(t, err)
	_, err = h.Write(data)
	require.NoError(t, err)

	return hex.EncodeToString(sum[:]), hex.EncodeToString(h.Sum(nil))
}

// TestParse_WheelMultiValueFields pins METADATA header parsing and
// multi-value collection for a Metadata-Version 2.4 wheel, and the body
// (rather than any header) becoming the Description.
func TestParse_WheelMultiValueFields(t *testing.T) {
	dir := t.TempDir()
	wheelPath := filepath.Join(dir, "gppfixture_alpha-1.0.0-py3-none-any.whl")

	header := strings.Join([]string{
		"Metadata-Version: 2.4",
		"Name: gppfixture-alpha",
		"Version: 1.0.0",
		"Classifier: Programming Language :: Python :: 3",
		"Classifier: License :: OSI Approved :: MIT License",
		`Requires-Dist: gppfixture-shared>=1.0; python_version >= "3.8"`,
		"Provides-Extra: extra-one",
		"Provides-Extra: extra-two",
		"Project-URL: Homepage, https://example.invalid/gppfixture-alpha",
		"Project-URL: Source, https://example.invalid/gppfixture-alpha/src",
		"License-Expression: MIT",
		"License-File: LICENSE.txt",
		"Dynamic: requires-dist",
		"Dynamic: license-file",
	}, "\n")
	body := "Fixture alpha description line one.\nFixture alpha description line two.\n"
	metadata := header + "\n\n" + body

	writeWheelFixture(t, wheelPath, []fixtureEntry{
		{name: "gppfixture_alpha-1.0.0.dist-info/METADATA", contents: metadata},
		{name: "gppfixture_alpha-1.0.0.dist-info/WHEEL", contents: "Wheel-Version: 1.0\nGenerator: gppfixture\nRoot-Is-Purelib: true\nTag: py3-none-any\n"},
		{name: "gppfixture_alpha-1.0.0.dist-info/RECORD", contents: ""},
		{name: "gppfixture_alpha/__init__.py", contents: "# fixture module\n"},
	})

	sha2, blake2 := fileDigests(t, wheelPath)

	packages, err := distribution.Parse(wheelPath)
	require.NoError(t, err)
	require.Len(t, packages, 1)
	pkg := packages[0]

	assert.Equal(t, "bdist_wheel", pkg.FileType)
	assert.Equal(t, "py3", pkg.PythonVersion)
	assert.Equal(t, "gppfixture-alpha", pkg.SafeName)
	assert.Equal(t, "gppfixture_alpha-1.0.0-py3-none-any.whl", pkg.BaseFilename)
	assert.Equal(t, sha2, pkg.SHA2Digest)
	assert.Equal(t, blake2, pkg.Blake2_256Digest)

	got := pkg.MetadataMap()

	// The code never stores an md5 digest on PackageFile (NewPackageFile
	// discards HexDigest().md5), so md5_digest can never appear here.
	_, hasMD5 := got["md5_digest"]
	assert.False(t, hasMD5, "md5_digest should never be present")

	want := map[string][]string{
		":action":            {"file_upload"},
		"protocol_version":   {"1"},
		"name":               {"gppfixture-alpha"},
		"metadata_version":   {"2.4"},
		"version":            {"1.0.0"},
		"classifiers":        {"Programming Language :: Python :: 3", "License :: OSI Approved :: MIT License"},
		"requires_dist":      {`gppfixture-shared>=1.0; python_version >= "3.8"`},
		"provides_extra":     {"extra-one", "extra-two"},
		"project_urls":       {"Homepage, https://example.invalid/gppfixture-alpha", "Source, https://example.invalid/gppfixture-alpha/src"},
		"license_expression": {"MIT"},
		"license_file":       {"LICENSE.txt"},
		"dynamic":            {"requires-dist", "license-file"},
		"description":        {body},
		"filetype":           {"bdist_wheel"},
		"pyversion":          {"py3"},
		"sha256_digest":      {sha2},
		"blake2_256_digest":  {blake2},
	}
	assert.Equal(t, want, got)
}

// TestParse_WheelLicenseContinuation pins the pkginfo leading-whitespace
// handling for a Metadata-Version 2.1 header with continuation lines.
//
// The brief for this test asked for a `Description:` header with
// continuation lines, but Description can't show that: BaseDistribution.Parse
// always overwrites description from the message body afterward, even to
// empty (io.ReadAll never returns a nil slice), so a Description header value
// is never observable in MetadataMap(). License hits the same
// collapseLeadingWS code path and does survive to the map, so it is used
// here instead. See the PR description for the same note.
func TestParse_WheelLicenseContinuation(t *testing.T) {
	dir := t.TempDir()
	wheelPath := filepath.Join(dir, "gppfixture_beta-2.0.0-py3-none-any.whl")

	metadata := strings.Join([]string{
		"Metadata-Version: 2.1",
		"Name: gppfixture-beta",
		"Version: 2.0.0",
		"License: MIT License",
		"        Copyright (c) 2026 Example Fixture Author",
		"        Permission is hereby granted, free of charge, to use this fixture.",
	}, "\n") + "\n"

	writeWheelFixture(t, wheelPath, []fixtureEntry{
		{name: "gppfixture_beta-2.0.0.dist-info/METADATA", contents: metadata},
	})

	sha2, blake2 := fileDigests(t, wheelPath)

	packages, err := distribution.Parse(wheelPath)
	require.NoError(t, err)
	require.Len(t, packages, 1)
	pkg := packages[0]

	assert.Equal(t, "bdist_wheel", pkg.FileType)
	assert.Equal(t, "py3", pkg.PythonVersion)
	assert.Equal(t, "gppfixture-beta", pkg.SafeName)
	assert.Equal(t, "gppfixture_beta-2.0.0-py3-none-any.whl", pkg.BaseFilename)

	got := pkg.MetadataMap()
	want := map[string][]string{
		":action":           {"file_upload"},
		"protocol_version":  {"1"},
		"name":              {"gppfixture-beta"},
		"metadata_version":  {"2.1"},
		"version":           {"2.0.0"},
		"license":           {"MIT License Copyright (c) 2026 Example Fixture Author Permission is hereby granted, free of charge, to use this fixture."},
		"filetype":          {"bdist_wheel"},
		"pyversion":         {"py3"},
		"sha256_digest":     {sha2},
		"blake2_256_digest": {blake2},
	}
	assert.Equal(t, want, got)
}

// TestParse_SDistShortestPathSelection pins SDist.read's shortest-path
// PKG-INFO selection: a top-level PKG-INFO (not the tarball's first member)
// must win over a deeper egg-info PKG-INFO whose values differ, so a wrong
// pick is visible on named fields (version, summary).
func TestParse_SDistShortestPathSelection(t *testing.T) {
	dir := t.TempDir()
	sdistPath := filepath.Join(dir, "gppfixture-gamma-3.0.0.tar.gz")

	const prefix = "gppfixture-gamma-3.0.0"
	topLevelPKGINFO := strings.Join([]string{
		"Metadata-Version: 2.1",
		"Name: gppfixture-gamma",
		"Version: 3.0.0",
		"Summary: correct top-level summary",
	}, "\n") + "\n"
	deeperPKGINFO := strings.Join([]string{
		"Metadata-Version: 2.1",
		"Name: gppfixture-gamma",
		"Version: 3.0.0-stale",
		"Summary: WRONG deeper summary that must not be selected",
	}, "\n") + "\n"

	writeSDistFixture(t, sdistPath, []fixtureEntry{
		// The top-level PKG-INFO is deliberately not the first tar member.
		{name: prefix + "/setup.py", contents: "from setuptools import setup\nsetup()\n"},
		{name: prefix + "/src/gppfixture_gamma.egg-info/PKG-INFO", contents: deeperPKGINFO},
		{name: prefix + "/PKG-INFO", contents: topLevelPKGINFO},
	})

	sha2, blake2 := fileDigests(t, sdistPath)

	packages, err := distribution.Parse(sdistPath)
	require.NoError(t, err)
	require.Len(t, packages, 1)
	pkg := packages[0]

	assert.Equal(t, "sdist", pkg.FileType)
	assert.Equal(t, "source", pkg.PythonVersion)
	assert.Equal(t, "gppfixture-gamma", pkg.SafeName)
	assert.Equal(t, "gppfixture-gamma-3.0.0.tar.gz", pkg.BaseFilename)

	got := pkg.MetadataMap()
	want := map[string][]string{
		":action":           {"file_upload"},
		"protocol_version":  {"1"},
		"name":              {"gppfixture-gamma"},
		"metadata_version":  {"2.1"},
		"version":           {"3.0.0"},
		"summary":           {"correct top-level summary"},
		"filetype":          {"sdist"},
		"pyversion":         {"source"},
		"sha256_digest":     {sha2},
		"blake2_256_digest": {blake2},
	}
	assert.Equal(t, want, got)
}

// TestParse_GPGSignaturePairing pins Parse's signature pairing: a wheel plus
// a matching .asc file must attach the raw .asc bytes as the GPG signature,
// without gpg (AddGPGSignature only reads bytes).
func TestParse_GPGSignaturePairing(t *testing.T) {
	dir := t.TempDir()
	wheelPath := filepath.Join(dir, "gppfixture_delta-4.0.0-py3-none-any.whl")
	ascPath := wheelPath + ".asc"

	metadata := strings.Join([]string{
		"Metadata-Version: 2.1",
		"Name: gppfixture-delta",
		"Version: 4.0.0",
	}, "\n") + "\n"

	writeWheelFixture(t, wheelPath, []fixtureEntry{
		{name: "gppfixture_delta-4.0.0.dist-info/METADATA", contents: metadata},
	})

	const signatureBytes = "-----BEGIN PGP SIGNATURE-----\ndummy fixture signature bytes, not a real signature\n-----END PGP SIGNATURE-----\n"
	require.NoError(t, os.WriteFile(ascPath, []byte(signatureBytes), 0o644))

	packages, err := distribution.Parse(wheelPath, ascPath)
	require.NoError(t, err)
	require.Len(t, packages, 1)
	pkg := packages[0]

	require.NotNil(t, pkg.GPGSignature)
	assert.Equal(t, "gppfixture_delta-4.0.0-py3-none-any.whl.asc", pkg.GPGSignature.Filename)
	assert.Equal(t, []byte(signatureBytes), pkg.GPGSignature.Bytes)

	got := pkg.MetadataMap()
	// gpg_signature is an ignored key: the signature is only checked above,
	// directly on the struct.
	_, hasGPGKey := got["gpg_signature"]
	assert.False(t, hasGPGKey)

	want := map[string][]string{
		":action":           {"file_upload"},
		"protocol_version":  {"1"},
		"name":              {"gppfixture-delta"},
		"metadata_version":  {"2.1"},
		"version":           {"4.0.0"},
		"filetype":          {"bdist_wheel"},
		"pyversion":         {"py3"},
		"sha256_digest":     got["sha256_digest"],
		"blake2_256_digest": got["blake2_256_digest"],
	}
	assert.Equal(t, want, got)
}

// TestParse_DirectoryExpansionWheelFirst pins Parse's one-level directory
// expansion and wheel-first grouping: given a directory holding one wheel
// and one sdist, both are parsed and the wheel is ordered before the sdist.
//
// Only one of each is used here, so ordering is unambiguous; do not extend
// this to assert a fixed index among multiple wheels, since
// groupWheelFilesFirst sorts with sort.Slice, which is not stable.
func TestParse_DirectoryExpansionWheelFirst(t *testing.T) {
	dir := t.TempDir()

	wheelPath := filepath.Join(dir, "gppfixture_epsilon-5.0.0-py3-none-any.whl")
	wheelMetadata := strings.Join([]string{
		"Metadata-Version: 2.1",
		"Name: gppfixture-epsilon",
		"Version: 5.0.0",
	}, "\n") + "\n"
	writeWheelFixture(t, wheelPath, []fixtureEntry{
		{name: "gppfixture_epsilon-5.0.0.dist-info/METADATA", contents: wheelMetadata},
	})

	sdistPath := filepath.Join(dir, "gppfixture-zeta-6.0.0.tar.gz")
	sdistMetadata := strings.Join([]string{
		"Metadata-Version: 2.1",
		"Name: gppfixture-zeta",
		"Version: 6.0.0",
	}, "\n") + "\n"
	writeSDistFixture(t, sdistPath, []fixtureEntry{
		{name: "gppfixture-zeta-6.0.0/PKG-INFO", contents: sdistMetadata},
	})

	packages, err := distribution.Parse(dir)
	require.NoError(t, err)
	require.Len(t, packages, 2)

	wheelIdx, sdistIdx := -1, -1
	for i, pkg := range packages {
		switch pkg.BaseFilename {
		case "gppfixture_epsilon-5.0.0-py3-none-any.whl":
			wheelIdx = i
			assert.Equal(t, "bdist_wheel", pkg.FileType)
			assert.Equal(t, "py3", pkg.PythonVersion)
			assert.Equal(t, "gppfixture-epsilon", pkg.SafeName)
		case "gppfixture-zeta-6.0.0.tar.gz":
			sdistIdx = i
			assert.Equal(t, "sdist", pkg.FileType)
			assert.Equal(t, "source", pkg.PythonVersion)
			assert.Equal(t, "gppfixture-zeta", pkg.SafeName)
		default:
			t.Fatalf("unexpected package in results: %s", pkg.BaseFilename)
		}
	}

	require.NotEqual(t, -1, wheelIdx, "wheel not found in results")
	require.NotEqual(t, -1, sdistIdx, "sdist not found in results")
	assert.Less(t, wheelIdx, sdistIdx, "every wheel must come before every sdist")
}
