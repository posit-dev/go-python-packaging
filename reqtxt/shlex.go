// SPDX-License-Identifier: Apache-2.0 OR MIT

package reqtxt

import (
	"fmt"
	"unicode"
)

// shlexSplit splits line into words the way Python's shlex.split(line,
// comments=False, posix=True) would. pip's requirements-file reader
// (pip/_internal/req/req_file.py, via pip's COMMENT_RE stripping followed
// by shlex.split of the remaining line) uses exactly this POSIX-mode shlex
// behavior to split each requirements-file line into option/value tokens,
// which is why requirements files support quoting values that contain
// spaces (e.g. --find-links "/opt/my links"). Comment handling is not
// performed here: callers are expected to strip comments before calling
// shlexSplit.
//
// Rules (POSIX shlex, whitespace-split mode):
//   - Runs of unquoted whitespace separate words; leading, trailing, and
//     repeated whitespace produce no empty words.
//   - Single quotes ('...') preserve every character inside literally,
//     including backslashes: there is no escape syntax inside single
//     quotes.
//   - Double quotes ("...") preserve whitespace, but a backslash escapes
//     only a following '"' or '\\'; a backslash before any other
//     character is kept as a literal backslash (POSIX sh semantics, not
//     C-string escaping).
//   - A backslash outside of any quoting escapes the next character,
//     making it literal (e.g. "\ " is a literal space within a word).
//   - Quoted and unquoted runs immediately adjacent to each other
//     concatenate into a single word (e.g. a"b"c -> abc).
//   - An unterminated single or double quote is an error.
//
// A word only exists once at least one character (quoted or unquoted) has
// been appended to it, so shlexSplit("") and shlexSplit("   ") both
// return an empty, non-nil slice.
func shlexSplit(line string) ([]string, error) {
	const (
		stateNormal = iota
		stateSingle
		stateDouble
	)

	words := []string{}
	var word []rune
	inWord := false
	state := stateNormal

	appendRune := func(r rune) {
		word = append(word, r)
		inWord = true
	}
	endWord := func() {
		if inWord {
			words = append(words, string(word))
			word = word[:0]
			inWord = false
		}
	}

	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		switch state {
		case stateNormal:
			switch {
			case unicode.IsSpace(r):
				endWord()
			case r == '\'':
				state = stateSingle
				inWord = true
			case r == '"':
				state = stateDouble
				inWord = true
			case r == '\\':
				i++
				if i >= len(runes) {
					return nil, fmt.Errorf("%w: trailing unescaped backslash", ErrInvalidRequirementsFile)
				}
				appendRune(runes[i])
			default:
				appendRune(r)
			}

		case stateSingle:
			switch r {
			case '\'':
				state = stateNormal
			default:
				appendRune(r)
			}

		case stateDouble:
			switch r {
			case '"':
				state = stateNormal
			case '\\':
				if i+1 < len(runes) && (runes[i+1] == '"' || runes[i+1] == '\\') {
					i++
					appendRune(runes[i])
				} else {
					appendRune(r)
				}
			default:
				appendRune(r)
			}
		}
	}

	switch state {
	case stateSingle:
		return nil, fmt.Errorf("%w: unterminated single-quoted string", ErrInvalidRequirementsFile)
	case stateDouble:
		return nil, fmt.Errorf("%w: unterminated double-quoted string", ErrInvalidRequirementsFile)
	}

	endWord()
	return words, nil
}
