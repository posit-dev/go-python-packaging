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
		// A file-level option renders as "<name>=<value>" (see renderOption):
		// a bare "<name> <value>" would not round-trip for an option that
		// isn't in knownOptions, since dispatchFileOption only recovers a
		// value for an unrecognized flag from the inline "=value" form — a
		// bare "--unknown" is treated as a boolean and the following token
		// is dispatched as a separate entry. Known arity-1 options (e.g.
		// --index-url) also accept the "=" form on reparse, so using it
		// uniformly doesn't regress them.
		return renderOption(*v)
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
// a single shlex token; the value (or hash algorithm:digest pair) is
// quoted via quoteIfNeeded so embedded whitespace survives shlex
// re-tokenization on reparse.
func writeReqOptions(b *strings.Builder, hashes []Hash, options []OptionEntry) {
	for _, h := range hashes {
		b.WriteString(" --hash=")
		b.WriteString(quoteIfNeeded(h.Algorithm + ":" + h.Digest))
	}
	for _, o := range options {
		b.WriteString(" ")
		b.WriteString(renderOption(o))
	}
}

// renderOption renders an OptionEntry (file-level or per-line) as
// "<name>=<value>" (quoting value via quoteIfNeeded so it survives shlex
// re-tokenization), or bare "<name>" when Value is empty - unless name
// identifies a value-taking option (isValueTakingOption), in which case an
// empty Value is rendered as the explicit "<name>=\"\"" form instead of a
// bare flag. Without this, an empty value on a value-taking option (e.g.
// Parse("--index-url=") or a per-line "--global-option=") would render
// indistinguishably from a boolean flag, and reparsing the bare form would
// hit that option's "requires a value" arity check - breaking the
// Parse(f.String()) round trip. A genuinely boolean/arity-0 option (or an
// unrecognized bare flag) with Value == "" is unaffected and still renders
// bare, matching its actual on-the-wire form.
func renderOption(o OptionEntry) string {
	if o.Value == "" {
		if isValueTakingOption(o.Name) {
			return o.Name + `=""`
		}
		return o.Name
	}
	return o.Name + "=" + quoteIfNeeded(o.Value)
}

// isValueTakingOption reports whether name (an OptionEntry.Name) identifies
// an option that takes a value: a known file-level option with arity 1
// (knownOptions, defined in parse.go), or a per-requirement value option
// (perLineValueOptions, also parse.go). Both tables are keyed by an
// option's canonical spelling, which is exactly what dispatchFileOption and
// attachReqOptions ever store into OptionEntry.Name, so a direct lookup by
// name is sufficient here.
func isValueTakingOption(name string) bool {
	if spec, ok := knownOptions[name]; ok {
		return spec.arity == 1
	}
	return perLineValueOptions[name]
}

// quoteIfNeeded returns s unchanged if it contains no shlex-significant
// whitespace or double quotes; otherwise it wraps s in double quotes,
// backslash-escaping any '"' or '\\' so the result re-tokenizes (via
// shlexSplit's double-quote handling) back to the original value.
func quoteIfNeeded(s string) string {
	if !strings.ContainsAny(s, " \t\"") {
		return s
	}

	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		if r == '"' || r == '\\' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}
