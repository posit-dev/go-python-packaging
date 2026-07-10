// SPDX-License-Identifier: Apache-2.0 OR MIT

package reqtxt

import "strings"

// vcsSchemePrefixes are the "<vcs>+" prefixes pip recognizes on a
// requirement target, one per supported VCS backend (pip's
// pip/_internal/vcs/{git,mercurial,subversion,bazaar}.py each register a
// scheme of this form via VersionControl.register).
var vcsSchemePrefixes = []string{"git+", "hg+", "svn+", "bzr+"}

// urlSchemes are the URL schemes classifyTarget treats as a direct URL
// target, mirroring the schemes pip's is_url (pip/_internal/utils/misc.py)
// accepts for a requirement specifier: "file" plus the schemes registered
// by pip's index backends (http/https).
var urlSchemes = []string{"http://", "https://", "file://"}

// archiveExtensions are the local-archive suffixes pip's is_archive_file
// (pip/_internal/utils/misc.py) recognizes by extension. Multi-dot
// suffixes are ordered before their shorter overlapping counterpart (e.g.
// ".tar.gz" before ".tar") so the longer, more specific suffix is checked
// first.
var archiveExtensions = []string{".tar.gz", ".tar.bz2", ".whl", ".tgz", ".tar", ".zip"}

// classifyTarget determines whether tok is shaped like a VCS reference, a
// direct URL, a local archive, or a local path — the non-PEP-508
// requirement forms pip accepts on a requirement line — and if so, which
// Kind it is. It reports (_, false) when tok doesn't match any of those
// shapes, meaning the caller should instead try parsing it as a PEP 508
// requirement (name/version/extras/marker).
//
// This mirrors the target recognition performed by pip's is_url and
// is_archive_file (pip/_internal/utils/misc.py) and the VCS scheme
// dispatch in pip/_internal/vcs/__init__.py, but is a pure, shape-only
// string classifier: unlike pip, it never touches the filesystem. pip
// additionally treats a bare local name as a path if it exists on disk
// (via os.path.exists) even without a "/", "./", or recognized
// extension; this package deliberately does not reproduce that
// filesystem-dependent fallback, so a bare name with no path/URL/archive
// shape (e.g. "flask") always falls through to (_, false) here.
func classifyTarget(tok string) (kind Kind, ok bool) {
	for _, prefix := range vcsSchemePrefixes {
		if strings.HasPrefix(tok, prefix) {
			return KindVCS, true
		}
	}

	lower := strings.ToLower(tok)
	for _, scheme := range urlSchemes {
		if strings.HasPrefix(lower, scheme) {
			return KindURL, true
		}
	}

	for _, ext := range archiveExtensions {
		if strings.HasSuffix(lower, ext) {
			return KindLocalPath, true
		}
	}

	if strings.HasPrefix(tok, "./") || strings.HasPrefix(tok, "../") || strings.HasPrefix(tok, "/") ||
		tok == "." || strings.Contains(tok, "/") {
		return KindLocalPath, true
	}

	return 0, false
}

// eggName returns the value of a "#egg=<name>" fragment on raw (pip's
// legacy way of naming a VCS/URL requirement whose distribution name
// can't otherwise be inferred; see pip's _egg_fragment in
// pip/_internal/req/constructors.py). It returns "" if raw has no such
// fragment. The fragment may be followed by additional "&"-separated
// fragment parameters (e.g. "&subdirectory=..."), which are not part of
// the returned name.
func eggName(raw string) string {
	const marker = "#egg="
	i := strings.Index(raw, marker)
	if i < 0 {
		return ""
	}
	rest := raw[i+len(marker):]
	if j := strings.IndexByte(rest, '&'); j >= 0 {
		return rest[:j]
	}
	return rest
}
