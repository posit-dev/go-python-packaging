// SPDX-License-Identifier: Apache-2.0 OR MIT

package reqtxt

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
)

// utf8BOM is the UTF-8 encoding of U+FEFF (byte order mark), which some
// requirements files (and pip itself, via its charset-detection fallback)
// tolerate as a leading marker. Flatten contracts UTF-8 input and strips a
// leading BOM before handing content to Parse, since Parse has no charset
// awareness of its own.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// schemeRE matches a URL scheme prefix (RFC 3986 ALPHA *( ALPHA / DIGIT /
// "+" / "-" / "." ) "://"), e.g. "https://", "file://", "git+https://".
// Flatten uses this (alongside a leading "/") to decide whether an include
// target is passed to open verbatim rather than resolved against the
// referencing file's directory.
var schemeRE = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*://`)

// Flatten parses the file at root and follows its "-r"/"-c" includes
// (pip's parse_requirements / RequirementsFileParser._parse_and_recurse in
// pip._internal.req.req_file) to produce a single File with every include
// expanded inline, in the order encountered by a depth-first traversal.
// Each IncludeEntry is consumed and dropped; the entries of the file it
// references are spliced into its place. All file content is obtained via
// open, which Flatten decodes as UTF-8, stripping a leading byte-order
// mark. root is passed to open verbatim; relative include targets nested
// inside it (and inside whatever it includes) are resolved against the
// referencing file's directory using slash-based path semantics -
// deliberately not the OS-dependent filepath package, so behavior doesn't
// depend on host CWD or path separator. opts is threaded to every
// recursive Parse call, so e.g. WithEnv expansion is applied consistently
// at every level of the include tree.
func Flatten(root string, open func(path string) ([]byte, error), opts ...ParseOption) (*File, error) {
	entries, err := flattenWalk(root, false, map[string]bool{}, open, opts)
	if err != nil {
		return nil, err
	}
	return &File{Entries: entries}, nil
}

// flattenWalk parses the file at p and recursively expands its includes,
// returning the flattened entries it (transitively) contributes.
// constraintCtx is true if p was itself reached via a "-c"/"--constraint"
// include - specifically the immediate include directive that pulled p in,
// not any ancestor further up the include chain (see childConstraint below);
// visited is the set of resolved
// paths on the CURRENT recursion stack, used for cycle detection - a
// diamond (the same file reached via two different, non-overlapping
// branches) is fine and yields its entries twice (see the double-inclusion
// case below); a path already on the active stack is a true cycle.
func flattenWalk(p string, constraintCtx bool, visited map[string]bool, open func(string) ([]byte, error), opts []ParseOption) ([]Entry, error) {
	if visited[p] {
		return nil, errors.Join(ErrInvalidRequirementsFile, fmt.Errorf("%q recursively includes itself", p))
	}

	raw, err := open(p)
	if err != nil {
		return nil, errors.Join(ErrInvalidRequirementsFile, fmt.Errorf("opening %q: %w", p, err))
	}

	content := decodeUTF8(raw)

	file, err := Parse(content, opts...)
	if err != nil {
		return nil, err
	}

	visited[p] = true
	defer delete(visited, p)

	var out []Entry
	for _, e := range file.Entries {
		switch v := e.(type) {
		case *IncludeEntry:
			target := resolveInclude(p, v.Path)

			// childConstraint is set solely by the immediate include
			// directive that reached the nested file, matching pip:
			// pip._internal.req.req_file.RequirementsFileParser
			// ._parse_and_recurse sets nested_constraint = False for a
			// "-r"/"--requirement" directive and nested_constraint = True
			// for a "-c"/"--constraint" directive, then recurses with that
			// value alone - it is never OR'd with the ambient constraint
			// flag of the referencing file. So a "-r" nested inside a
			// "-c"-included file RESETS constraint-ness to false for
			// everything below it, while a "-c" nested inside a
			// "-r"-included file sets it to true regardless of the outer
			// context.
			childConstraint := v.Constraint

			children, err := flattenWalk(target, childConstraint, visited, open, opts)
			if err != nil {
				return nil, err
			}
			out = append(out, children...)

		case *RequirementEntry:
			cp := *v
			cp.Constraint = constraintCtx || v.Constraint
			out = append(out, &cp)

		case *UnnamedEntry:
			cp := *v
			cp.Constraint = constraintCtx || v.Constraint
			out = append(out, &cp)

		default:
			out = append(out, e)
		}
	}

	return out, nil
}

// decodeUTF8 decodes raw as UTF-8 text, stripping a leading byte-order
// mark if present.
func decodeUTF8(raw []byte) string {
	raw = bytes.TrimPrefix(raw, utf8BOM)
	return string(raw)
}

// resolveInclude resolves an include directive's target path, as found in
// the file at referencedFrom, into the path Flatten will pass to open.
// An absolute path (leading "/") or a scheme-bearing reference (e.g.
// "https://...", "file://...") is passed through verbatim - open decides
// how to fetch it. Otherwise, target is relative. If referencedFrom is
// itself a URL (scheme-bearing), target is resolved against it using
// net/url reference resolution (RFC 3986), since referencedFrom's
// "directory" isn't a filesystem path: slash-based path.Dir/path.Join
// would mangle the scheme (e.g. turning "https://host/reqs/root.txt" into
// "https:/host/reqs", collapsing the double slash). Otherwise target is
// joined to referencedFrom's directory using slash-based (path package)
// semantics and cleaned, mirroring pip's
// os.path.join(os.path.dirname(filename), req_path) but OS-independent.
func resolveInclude(referencedFrom, target string) string {
	if strings.HasPrefix(target, "/") || schemeRE.MatchString(target) {
		return target
	}
	if schemeRE.MatchString(referencedFrom) {
		if base, err := url.Parse(referencedFrom); err == nil {
			if rel, err := url.Parse(target); err == nil {
				return base.ResolveReference(rel).String()
			}
		}
	}
	return path.Clean(path.Join(path.Dir(referencedFrom), target))
}
