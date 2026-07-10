// SPDX-License-Identifier: Apache-2.0 OR MIT

package reqtxt

import (
	"regexp"
	"strings"
)

// commentRE matches pip's COMMENT_RE (pip/_internal/req/req_file.py):
// "(^|\s+)#.*$". A '#' at the start of the line, or preceded by
// whitespace, begins a comment that runs to end-of-line. A '#' that is
// not whitespace-preceded (e.g. the fragment in
// "git+https://h/p#egg=name") is left alone.
var commentRE = regexp.MustCompile(`(^|\s+)#.*$`)

// envVarRE matches pip's ENV_VAR_RE (pip/_internal/req/req_file.py):
// `\$\{([A-Z0-9_]+)\}`. Only the uppercase form is recognized; a
// lowercase or mixed-case name inside ${...} is left as literal text.
var envVarRE = regexp.MustCompile(`\$\{([A-Z0-9_]+)\}`)

// ParseOption configures optional behavior for preprocessing/parsing a
// requirements file, e.g. WithEnv.
type ParseOption func(*parseConfig)

// parseConfig holds the options accumulated from a caller's ParseOption
// values.
type parseConfig struct {
	// env looks up an environment variable by name, returning its value
	// and whether it is set. When nil, ${VAR} references are left
	// literal (no expansion is attempted).
	env func(string) (string, bool)
}

// WithEnv enables "${VAR}" expansion using lookup (e.g. os.LookupEnv).
// Without it, "${VAR}" is left literal. Only the uppercase "${NAME}" form
// (NAME matching [A-Z0-9_]+) is ever expanded; a variable that lookup
// reports as unset, or that resolves to the empty string, is also left
// literal - matching pip's expand_env_variables behavior. Expansion is
// applied to the whole assembled logical line, before tokenization.
func WithEnv(lookup func(string) (string, bool)) ParseOption {
	return func(cfg *parseConfig) {
		cfg.env = lookup
	}
}

// logicalLine is one preprocessed line of a requirements file, ready for
// tokenization: comments and blank lines have already been removed, any
// backslash continuations already joined, and (if configured) any
// "${VAR}" references already expanded.
type logicalLine struct {
	// text is the fully assembled, comment-stripped, trimmed, and
	// (optionally) env-expanded line content.
	text string
	// line is the 1-based physical line number, in the original
	// (post-line-ending-normalization) content, of this logical line's
	// first segment - i.e. the line a continuation join started on.
	line int
}

// preprocess turns raw requirements-file content into an ordered slice of
// logicalLine values, following pip's req_file.preprocess pipeline:
//
//  1. Normalize line endings ("\r\n" and lone "\r" both become "\n"),
//     before anything else, so continuation joining and physical-line
//     counting are CRLF-safe.
//  2. Join backslash continuations: a physical line ending in "\"
//     concatenates directly to the next physical line, with NO separator
//     inserted (the trailing "\" is simply removed). The joined logical
//     line's line number is that of its first physical segment.
//  3. Strip comments per commentRE.
//  4. Drop logical lines that are empty or all-whitespace once comments
//     are stripped.
//  5. Trim remaining surrounding whitespace.
//  6. If cfg.env is set, expand "${NAME}" references (see WithEnv).
//
// The returned error exists for signature stability with future callers;
// preprocess itself has no failure mode today.
func preprocess(content string, cfg parseConfig) ([]logicalLine, error) {
	normalized := normalizeLineEndings(content)
	physicalLines := strings.Split(normalized, "\n")

	var lines []logicalLine
	var (
		joined     strings.Builder
		joinedLine int
		joining    bool
	)

	flush := func() {
		if !joining {
			return
		}
		joining = false

		text := commentRE.ReplaceAllString(joined.String(), "")
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		if cfg.env != nil {
			text = expandEnv(text, cfg.env)
		}
		lines = append(lines, logicalLine{text: text, line: joinedLine})
	}

	for i, physical := range physicalLines {
		physicalLineNum := i + 1

		if !joining {
			joining = true
			joinedLine = physicalLineNum
			joined.Reset()
		}

		if rest, ok := strings.CutSuffix(physical, `\`); ok {
			joined.WriteString(rest)
			continue
		}

		joined.WriteString(physical)
		flush()
	}
	// A trailing continuation with no following physical line still
	// needs to be flushed (there's no next segment to join to).
	flush()

	return lines, nil
}

// normalizeLineEndings converts "\r\n" and lone "\r" to "\n", matching
// Python's universal-newlines splitlines() behavior that pip's preprocess
// relies on.
func normalizeLineEndings(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	return strings.ReplaceAll(content, "\r", "\n")
}

// expandEnv replaces every "${NAME}" reference in text (NAME matching
// [A-Z0-9_]+) with the value lookup(NAME) returns, unless lookup reports
// the variable as unset or reports an empty value - in either case the
// literal "${NAME}" is left in place, matching pip's
// expand_env_variables.
func expandEnv(text string, lookup func(string) (string, bool)) string {
	return envVarRE.ReplaceAllStringFunc(text, func(match string) string {
		name := envVarRE.FindStringSubmatch(match)[1]
		value, ok := lookup(name)
		if !ok || value == "" {
			return match
		}
		return value
	})
}
