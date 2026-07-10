// SPDX-License-Identifier: Apache-2.0 OR MIT

package reqtxt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifyTarget(t *testing.T) {
	cases := []struct {
		name     string
		tok      string
		wantKind Kind
		wantOK   bool
	}{
		{
			name:     "vcs git+https with egg fragment",
			tok:      "git+https://h/p@v1#egg=n",
			wantKind: KindVCS,
			wantOK:   true,
		},
		{
			name:     "url https to a wheel",
			tok:      "https://h/x-1.0.whl",
			wantKind: KindURL,
			wantOK:   true,
		},
		{
			name:     "url file scheme",
			tok:      "file:///a/b",
			wantKind: KindURL,
			wantOK:   true,
		},
		{
			name:     "bare tar.gz archive",
			tok:      "foo-1.0.tar.gz",
			wantKind: KindLocalPath,
			wantOK:   true,
		},
		{
			name:     "bare wheel archive",
			tok:      "x.whl",
			wantKind: KindLocalPath,
			wantOK:   true,
		},
		{
			name:     "relative path with leading dot-slash",
			tok:      "./local",
			wantKind: KindLocalPath,
			wantOK:   true,
		},
		{
			name:     "relative path with leading dot-dot-slash",
			tok:      "../up",
			wantKind: KindLocalPath,
			wantOK:   true,
		},
		{
			name:     "absolute path",
			tok:      "/abs/pkg",
			wantKind: KindLocalPath,
			wantOK:   true,
		},
		{
			name:     "path shape via embedded slash",
			tok:      "subdir/pkg",
			wantKind: KindLocalPath,
			wantOK:   true,
		},
		{
			name:     "current directory",
			tok:      ".",
			wantKind: KindLocalPath,
			wantOK:   true,
		},
		{
			name:   "plain requirement name",
			tok:    "flask",
			wantOK: false,
		},
		{
			name:   "plain requirement with version specifier",
			tok:    "flask>=2.0",
			wantOK: false,
		},
		{
			name:   "plain requirement with extras",
			tok:    "requests[security]",
			wantOK: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotKind, gotOK := classifyTarget(c.tok)
			assert.Equal(t, c.wantOK, gotOK, "ok mismatch for %q", c.tok)
			if c.wantOK {
				assert.Equal(t, c.wantKind.String(), gotKind.String(), "kind mismatch for %q", c.tok)
			}
		})
	}
}

func TestClassifyTargetVCSSchemePrefixes(t *testing.T) {
	for _, prefix := range []string{"git+", "hg+", "svn+", "bzr+"} {
		tok := prefix + "https://h/p"
		kind, ok := classifyTarget(tok)
		assert.True(t, ok, "expected %q to classify as VCS", tok)
		assert.Equal(t, KindVCS.String(), kind.String(), "expected %q to be KindVCS", tok)
	}
}

func TestClassifyTargetVCSSchemePrefixCaseInsensitive(t *testing.T) {
	kind, ok := classifyTarget("Git+https://h/p")
	assert.True(t, ok, `expected "Git+https://h/p" to classify as VCS`)
	assert.Equal(t, KindVCS.String(), kind.String(), `expected "Git+https://h/p" to be KindVCS`)
}

func TestClassifyTargetArchiveExtensions(t *testing.T) {
	for _, ext := range []string{".whl", ".tar.gz", ".tgz", ".tar.bz2", ".tar", ".zip"} {
		tok := "pkg-1.0" + ext
		kind, ok := classifyTarget(tok)
		assert.True(t, ok, "expected %q to classify as an archive", tok)
		assert.Equal(t, KindLocalPath.String(), kind.String(), "expected %q to be KindLocalPath", tok)
	}
}

func TestEggName(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "vcs fragment with egg only",
			raw:  "git+https://h/p#egg=name",
			want: "name",
		},
		{
			name: "vcs fragment with egg and subdirectory",
			raw:  "git+https://h/p#egg=name&subdirectory=x",
			want: "name",
		},
		{
			name: "no fragment at all",
			raw:  "./x",
			want: "",
		},
		{
			name: "vcs fragment with subdirectory before egg",
			raw:  "git+https://h/p#subdirectory=src&egg=name",
			want: "name",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, eggName(c.raw))
		})
	}
}
