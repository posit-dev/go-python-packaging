// SPDX-License-Identifier: Apache-2.0 OR MIT

package reqtxt

import (
	"errors"
	"fmt"
	"strings"

	"github.com/posit-dev/go-python-packaging/requirement"
)

// knownOption describes one of pip's recognized global requirements-file
// options (pip's SUPPORTED_OPTIONS in pip/_internal/req/req_file.py): its
// canonical long name (OptionEntry.Name always uses this, even when a
// short form was used on the line) and how many values it takes.
type knownOption struct {
	long  string
	arity int
}

// knownOptions maps every short and long spelling of a recognized
// file-level option to its canonical long name and arity. A flag not in
// this table is still accepted (see dispatchFileOption) but captured
// generically, without normalization.
var knownOptions = map[string]knownOption{
	"-i":                {optIndexURL, 1},
	"--index-url":       {optIndexURL, 1},
	"--extra-index-url": {optExtraIndexURL, 1},
	"--no-index":        {optNoIndex, 0},
	"-f":                {optFindLinks, 1},
	"--find-links":      {optFindLinks, 1},
	"--require-hashes":  {optRequireHashes, 0},
	"--pre":             {optPre, 0},
	"--trusted-host":    {optTrustedHost, 1},
	"--no-binary":       {optNoBinary, 1},
	"--only-binary":     {optOnlyBinary, 1},
	"--prefer-binary":   {optPreferBinary, 0},
}

// perLineValueOptions are the per-requirement options (other than
// "--hash") that pip's SUPPORTED_OPTIONS_REQ recognizes and that this
// package captures onto RequirementEntry.Options / UnnamedEntry.Options.
var perLineValueOptions = map[string]bool{
	"--config-settings": true,
	"--global-option":   true,
	"--install-option":  true,
}

// Parse parses content as a requirements.txt (or constraints) file. It
// follows pip's req_file preprocessing pipeline (see preprocess) to
// produce logical lines, then dispatches each one by its leading token
// (see pip's req_file.break_args_options / process_line) into the
// Entry it represents.
//
// Parse does not follow "-r"/"-c" includes; that is Flatten's job.
func Parse(content string, opts ...ParseOption) (*File, error) {
	var cfg parseConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	lines, err := preprocess(content, cfg)
	if err != nil {
		return nil, err
	}

	file := &File{}
	for _, ll := range lines {
		tokens, err := shlexSplit(ll.text)
		if err != nil {
			return nil, wrapLineErr(ll.line, err)
		}

		entries, err := dispatchLine(tokens, ll.line)
		if err != nil {
			return nil, err
		}
		file.Entries = append(file.Entries, entries...)
	}

	return file, nil
}

// dispatchLine dispatches the tokens of one (recursive tail of a) logical
// line into zero or more Entry values, per pip's break_args_options: the
// leading token determines the entry kind. For the plain file-option
// branch only, any tokens left over after that flag's own value (if any)
// are dispatched again as if they began a new line — so e.g. an
// unrecognized boolean flag followed by a requirement ("--frob foo")
// yields two entries, rather than the flag swallowing "foo" as its value.
// The include/editable/package branches instead consume the *entire*
// remaining token stream for their own per-line options (hashes, etc.)
// and never recurse.
func dispatchLine(tokens []string, lineNum int) ([]Entry, error) {
	if len(tokens) == 0 {
		return nil, nil
	}

	first := tokens[0]
	rest := tokens[1:]
	name, eqValue, hasEq := splitFlagValue(first)

	switch name {
	case "-r", "--requirement":
		return dispatchInclude(false, hasEq, eqValue, rest, lineNum)
	case "-c", "--constraint":
		return dispatchInclude(true, hasEq, eqValue, rest, lineNum)
	case "-e", "--editable":
		return dispatchEditable(hasEq, eqValue, rest, lineNum)
	}

	if strings.HasPrefix(first, "-") {
		return dispatchFileOption(name, hasEq, eqValue, rest, lineNum)
	}

	return dispatchPackage(first, rest, lineNum)
}

// dispatchInclude handles a leading "-r"/"--requirement" or
// "-c"/"--constraint" directive, producing an *IncludeEntry.
func dispatchInclude(constraint, hasEq bool, eqValue string, rest []string, lineNum int) ([]Entry, error) {
	label := "-r/--requirement"
	if constraint {
		label = "-c/--constraint"
	}

	path, remaining, err := takeValue(label, hasEq, eqValue, rest, lineNum)
	if err != nil {
		return nil, err
	}

	following, err := dispatchLine(remaining, lineNum)
	if err != nil {
		return nil, err
	}

	entry := &IncludeEntry{Path: path, Constraint: constraint}
	return append([]Entry{entry}, following...), nil
}

// dispatchEditable handles a leading "-e"/"--editable" directive: the
// next token is the install target, classified the same way a package
// line's target would be, plus any trailing per-line options.
func dispatchEditable(hasEq bool, eqValue string, rest []string, lineNum int) ([]Entry, error) {
	target, remaining, err := takeValue("-e/--editable", hasEq, eqValue, rest, lineNum)
	if err != nil {
		return nil, err
	}

	kind, ok := classifyTarget(target)
	if !ok {
		return nil, lineError(lineNum, "editable requirement %q is not a recognized VCS/URL/local-path target", target)
	}

	entry := &UnnamedEntry{
		Raw:      target,
		Kind:     kind,
		Editable: true,
		EggName:  eggName(target),
	}
	if err := attachReqOptions(&entry.Hashes, &entry.Options, remaining, lineNum); err != nil {
		return nil, err
	}

	return []Entry{entry}, nil
}

// dispatchFileOption handles a leading "-"/"--" token that isn't
// -r/-c/-e: a file-level pip option, matched against knownOptions when
// recognized, or captured generically otherwise. A standalone "--hash"
// (with no preceding requirement to attach to) is rejected: "--hash" is
// only ever a per-requirement option (see attachReqOptions).
func dispatchFileOption(name string, hasEq bool, eqValue string, rest []string, lineNum int) ([]Entry, error) {
	if name == "--hash" {
		return nil, lineError(lineNum, "--hash has no associated requirement")
	}

	var (
		value     string
		remaining []string
	)

	if spec, known := knownOptions[name]; known {
		name = spec.long
		if spec.arity == 1 {
			var err error
			value, remaining, err = takeValue(name, hasEq, eqValue, rest, lineNum)
			if err != nil {
				return nil, err
			}
		} else {
			// A boolean known option never consumes a following token,
			// even if it happened to be written with "=".
			value, remaining = eqValue, rest
		}
	} else {
		// Unknown flag: "--opt=value" takes the value; a bare "--opt" is
		// boolean and never consumes the next token — it is left to be
		// dispatched as its own, separate entry.
		value, remaining = eqValue, rest
	}

	following, err := dispatchLine(remaining, lineNum)
	if err != nil {
		return nil, err
	}

	entry := &OptionEntry{Name: name, Value: value}
	return append([]Entry{entry}, following...), nil
}

// dispatchPackage handles a package/requirement line: first is either a
// VCS/URL/local-path target (classifyTarget) or a plain PEP 508
// requirement (requirement.Parse). Unlike the file-option branch, every
// remaining token on the line is a per-requirement option attached to
// this single entry (attachReqOptions) — none of it is dispatched as a
// separate entry.
func dispatchPackage(first string, rest []string, lineNum int) ([]Entry, error) {
	var (
		entry   Entry
		hashes  *[]Hash
		options *[]OptionEntry
	)

	if kind, ok := classifyTarget(first); ok {
		e := &UnnamedEntry{Raw: first, Kind: kind, EggName: eggName(first)}
		entry, hashes, options = e, &e.Hashes, &e.Options
	} else {
		req, err := requirement.Parse(first)
		if err != nil {
			return nil, wrapLineErr(lineNum, err)
		}
		e := &RequirementEntry{Requirement: req}
		entry, hashes, options = e, &e.Hashes, &e.Options
	}

	if err := attachReqOptions(hashes, options, rest, lineNum); err != nil {
		return nil, err
	}

	return []Entry{entry}, nil
}

// attachReqOptions consumes every token in tokens as a per-requirement
// option (pip's SUPPORTED_OPTIONS_REQ), appending to *hashes or *options
// as appropriate. Unlike dispatchFileOption, this never leaves tokens for
// a separate entry: every token on a requirement/editable line's tail
// belongs to that one requirement.
func attachReqOptions(hashes *[]Hash, options *[]OptionEntry, tokens []string, lineNum int) error {
	i := 0
	for i < len(tokens) {
		tok := tokens[i]
		if !strings.HasPrefix(tok, "-") {
			return lineError(lineNum, "unexpected token %q (expected a per-requirement option)", tok)
		}

		name, eqValue, hasEq := splitFlagValue(tok)
		i++

		var value string
		switch {
		case hasEq:
			value = eqValue
		case name == "--hash" || perLineValueOptions[name]:
			if i >= len(tokens) {
				return lineError(lineNum, "%s requires a value", name)
			}
			value = tokens[i]
			i++
		}
		// An unknown bare "--flag" (no "=", not --hash/perLineValueOptions)
		// falls through with value == "", matching the generic
		// unknown-flag boolean rule.

		if name == "--hash" {
			h, err := parseHash(value, lineNum)
			if err != nil {
				return err
			}
			*hashes = append(*hashes, h)
			continue
		}

		*options = append(*options, OptionEntry{Name: name, Value: value})
	}

	return nil
}

// parseHash parses a "--hash" value ("algorithm:digest") into a Hash,
// splitting on the first ':' only (a digest can't itself contain a
// leading algorithm-style prefix, but be conservative and only split
// once regardless).
func parseHash(raw string, lineNum int) (Hash, error) {
	algo, digest, ok := strings.Cut(raw, ":")
	if !ok {
		return Hash{}, lineError(lineNum, "malformed --hash value %q: expected \"algorithm:digest\"", raw)
	}
	return Hash{Algorithm: algo, Digest: digest}, nil
}

// splitFlagValue splits tok on its first '=', if any, into a flag name
// and inline value (e.g. "--index-url=https://x" -> "--index-url",
// "https://x", true). A token with no '=' returns (tok, "", false).
func splitFlagValue(tok string) (name, value string, hasValue bool) {
	if i := strings.Index(tok, "="); i >= 0 {
		return tok[:i], tok[i+1:], true
	}
	return tok, "", false
}

// takeValue resolves the value for a single-value directive (-r/-c/-e, or
// a known arity-1 file option): the inline "=value" form if present,
// otherwise the next token in rest. label names the directive, for the
// error produced when neither form supplies a value.
func takeValue(label string, hasEq bool, eqValue string, rest []string, lineNum int) (value string, remaining []string, err error) {
	if hasEq {
		return eqValue, rest, nil
	}
	if len(rest) == 0 {
		return "", nil, lineError(lineNum, "%s requires a value", label)
	}
	return rest[0], rest[1:], nil
}

// lineError builds a line-numbered error wrapping ErrInvalidRequirementsFile.
func lineError(lineNum int, format string, args ...any) error {
	msg := fmt.Errorf(format, args...)
	return errors.Join(ErrInvalidRequirementsFile, fmt.Errorf("line %d: %w", lineNum, msg))
}

// wrapLineErr wraps an existing error (e.g. from shlexSplit or
// requirement.Parse) with a line number, ensuring the result still
// satisfies errors.Is against both ErrInvalidRequirementsFile and
// whatever sentinel err itself wraps (e.g. requirement.ErrInvalidRequirement).
func wrapLineErr(lineNum int, err error) error {
	return errors.Join(ErrInvalidRequirementsFile, fmt.Errorf("line %d: %w", lineNum, err))
}
