// SPDX-License-Identifier: Apache-2.0 OR MIT

package reqtxt

import (
	"errors"

	"github.com/posit-dev/go-python-packaging/requirement"
)

// ErrInvalidRequirementsFile is the sentinel wrapped by errors returned
// while parsing a malformed requirements.txt file.
var ErrInvalidRequirementsFile = errors.New("invalid requirements file")

// File is a parsed requirements.txt (or constraints) file: an ordered list
// of the entries it contains, in file order.
type File struct {
	Entries []Entry
}

// Entry is one line's worth of parsed requirements.txt content. It is
// implemented by *RequirementEntry, *IncludeEntry, *UnnamedEntry, and
// *OptionEntry.
type Entry interface {
	entry()
}

// RequirementEntry is a PEP 508 requirement line (e.g. "flask==2.0 ;
// python_version >= '3.8'"), optionally followed by "--hash" and other
// per-requirement options on continuation lines.
type RequirementEntry struct {
	Requirement requirement.Requirement
	Hashes      []Hash
	Options     []OptionEntry
	// Constraint is true when the file this entry appears in was included
	// via "-c"/"--constraint". Constraint-ness is set by the immediate
	// include directive (per pip's req_file._parse_and_recurse); it is NOT
	// inherited through nested includes, so a "-r" nested inside a "-c" file
	// yields non-constraint entries. Set by Flatten; false after Parse alone.
	Constraint bool
}

func (*RequirementEntry) entry() {}

// IncludeEntry is a "-r"/"--requirement" or "-c"/"--constraint" directive
// referencing another requirements/constraints file.
type IncludeEntry struct {
	Path string
	// Constraint is true if this include used "-c"/"--constraint" rather
	// than "-r"/"--requirement".
	Constraint bool
}

func (*IncludeEntry) entry() {}

// UnnamedEntry is a requirement line that isn't a plain PEP 508
// requirement: a VCS reference, a direct URL, a local path, or an editable
// install ("-e") of any of those. Its Raw field preserves the original
// requirement specifier text for later classification/parsing.
type UnnamedEntry struct {
	Raw      string
	Kind     Kind
	Editable bool
	// EggName is the "#egg=name" fragment, if present.
	EggName string
	Hashes  []Hash
	Options []OptionEntry
	// Constraint is true when the file this entry appears in was included
	// via "-c"/"--constraint". Constraint-ness is set by the immediate
	// include directive (per pip's req_file._parse_and_recurse); it is NOT
	// inherited through nested includes, so a "-r" nested inside a "-c" file
	// yields non-constraint entries. Set by Flatten; false after Parse alone.
	Constraint bool
}

func (*UnnamedEntry) entry() {}

// OptionEntry is a global pip option line (e.g. "--index-url
// https://example.com/simple"), as opposed to a per-requirement option
// attached to a RequirementEntry or UnnamedEntry.
type OptionEntry struct {
	Name  string
	Value string
}

func (*OptionEntry) entry() {}

// Kind identifies the flavor of requirement specifier an UnnamedEntry
// represents.
type Kind int

const (
	// KindVCS is a VCS reference (e.g. "git+https://...").
	KindVCS Kind = iota
	// KindURL is a direct URL to a source archive or wheel.
	KindURL
	// KindLocalPath is a path to a local directory or archive.
	KindLocalPath
)

// String renders k as a readable name, primarily so test failures are
// legible.
func (k Kind) String() string {
	switch k {
	case KindVCS:
		return "vcs"
	case KindURL:
		return "url"
	case KindLocalPath:
		return "local-path"
	default:
		return "unknown"
	}
}

// Hash is a "--hash" value attached to a requirement, used for
// hash-checking mode.
type Hash struct {
	Algorithm string
	Digest    string
}

// Canonical long option names, as used by OptionEntry.Name.
const (
	optIndexURL      = "--index-url"
	optExtraIndexURL = "--extra-index-url"
	optNoIndex       = "--no-index"
	optFindLinks     = "--find-links"
	optRequireHashes = "--require-hashes"
	optPre           = "--pre"
	optTrustedHost   = "--trusted-host"
	optNoBinary      = "--no-binary"
	optOnlyBinary    = "--only-binary"
	optPreferBinary  = "--prefer-binary"
)

// Requirements returns the RequirementEntry values in f.Entries, in file
// order.
func (f *File) Requirements() []RequirementEntry {
	var out []RequirementEntry
	for _, e := range f.Entries {
		if r, ok := e.(*RequirementEntry); ok {
			out = append(out, *r)
		}
	}
	return out
}

// Includes returns the IncludeEntry values in f.Entries, in file order.
func (f *File) Includes() []IncludeEntry {
	var out []IncludeEntry
	for _, e := range f.Entries {
		if i, ok := e.(*IncludeEntry); ok {
			out = append(out, *i)
		}
	}
	return out
}

// Options returns the OptionEntry values in f.Entries, in file order.
func (f *File) Options() []OptionEntry {
	var out []OptionEntry
	for _, e := range f.Entries {
		if o, ok := e.(*OptionEntry); ok {
			out = append(out, *o)
		}
	}
	return out
}

// IndexURL returns the value of the last "--index-url" option in f, and
// whether one was present at all.
func (f *File) IndexURL() (string, bool) {
	var (
		value string
		found bool
	)
	for _, o := range f.Options() {
		if o.Name == optIndexURL {
			value = o.Value
			found = true
		}
	}
	return value, found
}

// ExtraIndexURLs returns the values of all "--extra-index-url" options in
// f, in file order.
func (f *File) ExtraIndexURLs() []string {
	return optionValues(f.Options(), optExtraIndexURL)
}

// FindLinks returns the values of all "--find-links" options in f, in file
// order.
func (f *File) FindLinks() []string {
	return optionValues(f.Options(), optFindLinks)
}

// TrustedHosts returns the values of all "--trusted-host" options in f, in
// file order.
func (f *File) TrustedHosts() []string {
	return optionValues(f.Options(), optTrustedHost)
}

// NoBinary returns the values of all "--no-binary" options in f, in file
// order.
func (f *File) NoBinary() []string {
	return optionValues(f.Options(), optNoBinary)
}

// OnlyBinary returns the values of all "--only-binary" options in f, in
// file order.
func (f *File) OnlyBinary() []string {
	return optionValues(f.Options(), optOnlyBinary)
}

// NoIndex reports whether f has a "--no-index" option.
func (f *File) NoIndex() bool {
	return hasOption(f.Options(), optNoIndex)
}

// RequireHashes reports whether f has a "--require-hashes" option.
func (f *File) RequireHashes() bool {
	return hasOption(f.Options(), optRequireHashes)
}

// Pre reports whether f has a "--pre" option.
func (f *File) Pre() bool {
	return hasOption(f.Options(), optPre)
}

// PreferBinary reports whether f has a "--prefer-binary" option.
func (f *File) PreferBinary() bool {
	return hasOption(f.Options(), optPreferBinary)
}

// optionValues returns the Value of every OptionEntry in opts whose Name
// matches name, in order.
func optionValues(opts []OptionEntry, name string) []string {
	var out []string
	for _, o := range opts {
		if o.Name == name {
			out = append(out, o.Value)
		}
	}
	return out
}

// hasOption reports whether any OptionEntry in opts has the given Name.
func hasOption(opts []OptionEntry, name string) bool {
	for _, o := range opts {
		if o.Name == name {
			return true
		}
	}
	return false
}
