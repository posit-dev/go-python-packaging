// SPDX-License-Identifier: Apache-2.0 OR MIT

package reqtxt

import "strings"

// String renders f as a canonical requirements.txt document: one line per
// entry, in file order, joined with "\n". The rendering is not
// byte-identical to whatever produced f (comments and blank lines, which
// Parse drops, are never reconstructed), but Parse(f.String()) yields a
// File equivalent to f: every entry's semantic fields (name, extras,
// specifiers, marker, hashes, options, paths, ...) survive the round trip.
func (f *File) String() string {
	lines := make([]string, len(f.Entries))
	for i, e := range f.Entries {
		lines[i] = renderEntry(e)
	}
	return strings.Join(lines, "\n")
}

// renderEntry renders a single Entry to its canonical line, per the
// per-type rules documented on File.String.
func renderEntry(e Entry) string {
	switch v := e.(type) {
	case *RequirementEntry:
		var b strings.Builder
		b.WriteString(v.Requirement.String())
		writeReqOptions(&b, v.Hashes, v.Options)
		return b.String()
	case *UnnamedEntry:
		var b strings.Builder
		if v.Editable {
			b.WriteString("-e ")
		}
		b.WriteString(v.Raw)
		writeReqOptions(&b, v.Hashes, v.Options)
		return b.String()
	case *IncludeEntry:
		if v.Constraint {
			return "-c " + v.Path
		}
		return "-r " + v.Path
	case *OptionEntry:
		return renderFileOption(*v)
	default:
		// Entry is a closed interface (only the four types above
		// implement entry()), so this is unreachable barring a new
		// implementation added without updating this switch.
		return ""
	}
}

// writeReqOptions appends the canonical rendering of a RequirementEntry's
// or UnnamedEntry's per-line "--hash" values and other options to b: each
// as " --hash=<algorithm>:<digest>" or " <name>=<value>" (bare " <name>"
// when Value is empty), in that order, matching the fields' declaration
// order on the struct. The "=" form (rather than a separate token) is used
// for per-line options so a value containing spaces still round-trips as
// a single shlex token.
func writeReqOptions(b *strings.Builder, hashes []Hash, options []OptionEntry) {
	for _, h := range hashes {
		b.WriteString(" --hash=")
		b.WriteString(h.Algorithm)
		b.WriteString(":")
		b.WriteString(h.Digest)
	}
	for _, o := range options {
		b.WriteString(" ")
		b.WriteString(o.Name)
		if o.Value != "" {
			b.WriteString("=")
			b.WriteString(o.Value)
		}
	}
}

// renderFileOption renders a file-level OptionEntry as "<name> <value>",
// using a space (rather than "=") to separate name and value: this matches
// the predominant form pip's own documentation and generated files use for
// file-level options (e.g. "--index-url https://..."). A bare boolean
// option (Value == "") renders as just its name (e.g. "--no-index").
func renderFileOption(o OptionEntry) string {
	if o.Value == "" {
		return o.Name
	}
	return o.Name + " " + o.Value
}
