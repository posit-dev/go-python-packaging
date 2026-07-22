// SPDX-License-Identifier: Apache-2.0 OR MIT

package license

import (
	"strings"
	"unicode"
)

// ParseSPDXExpression parses an SPDX license expression and returns the individual
// license identifiers. This follows the SPDX License Expression syntax specification:
// https://spdx.github.io/spdx-spec/v2.3/SPDX-license-expressions/
//
// The function extracts all license identifiers from expressions like:
//   - Simple: "MIT" -> ["MIT"]
//   - OR (disjunctive): "MIT OR Apache-2.0" -> ["MIT", "Apache-2.0"]
//   - AND (conjunctive): "MIT AND Apache-2.0" -> ["MIT", "Apache-2.0"]
//   - WITH (exception): "GPL-3.0-only WITH Classpath-exception-2.0" -> ["GPL-3.0-only"]
//   - Or-later (+): "GPL-2.0+" -> ["GPL-2.0-or-later"]
//   - Custom: "LicenseRef-Proprietary" -> ["LicenseRef-Proprietary"]
//   - Nested: "(MIT OR Apache-2.0) AND BSD-3-Clause" -> ["MIT", "Apache-2.0", "BSD-3-Clause"]
//
// Note: Per SPDX spec, operators AND, OR, WITH are case-sensitive (must be uppercase).
// This parser is lenient and accepts lowercase for compatibility with real-world usage.
func ParseSPDXExpression(expr string) []string {
	if expr == "" {
		return nil
	}

	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil
	}

	tokens := tokenizeSPDX(expr)
	return extractLicenseIdentifiers(tokens)
}

// tokenizeSPDX splits an SPDX expression into tokens.
// Tokens are: identifiers (including +), AND, OR, WITH, (, )
func tokenizeSPDX(expr string) []string {
	var tokens []string
	var current strings.Builder

	flushCurrent := func() {
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}

	i := 0
	for i < len(expr) {
		ch := rune(expr[i])

		switch {
		case ch == '(' || ch == ')':
			flushCurrent()
			tokens = append(tokens, string(ch))
			i++

		case ch == '+':
			// + suffix attaches to the previous identifier (no whitespace per spec)
			// but we handle it as part of the identifier
			current.WriteRune(ch)
			i++

		case ch == ':':
			// Part of DocumentRef-X:LicenseRef-Y format
			current.WriteRune(ch)
			i++

		case unicode.IsSpace(ch):
			flushCurrent()
			i++

		default:
			current.WriteRune(ch)
			i++
		}
	}

	flushCurrent()
	return tokens
}

// extractLicenseIdentifiers extracts license identifiers from tokens,
// filtering out operators (AND, OR, WITH) and exception identifiers.
func extractLicenseIdentifiers(tokens []string) []string {
	var licenses []string
	seen := make(map[string]bool)
	skipNext := false

	for _, token := range tokens {
		// Skip parentheses
		if token == "(" || token == ")" {
			continue
		}

		// Check for operators - per SPDX spec these are case-sensitive
		// but we accept both for compatibility
		upper := strings.ToUpper(token)
		if upper == "AND" || upper == "OR" {
			continue
		}

		// WITH is followed by an exception identifier, skip both
		if upper == "WITH" {
			skipNext = true
			continue
		}

		if skipNext {
			skipNext = false
			continue
		}

		// Handle + suffix (or-later)
		// Per SPDX spec: "GPL-2.0+" means "GPL-2.0-or-later"
		cleanToken := token
		if strings.HasSuffix(token, "+") {
			base := strings.TrimSuffix(token, "+")
			// Convert to -or-later form if not already
			if !strings.HasSuffix(base, "-or-later") {
				cleanToken = base + "-or-later"
			} else {
				cleanToken = base
			}
		}

		// Deduplicate
		if !seen[cleanToken] {
			seen[cleanToken] = true
			licenses = append(licenses, cleanToken)
		}
	}

	return licenses
}
