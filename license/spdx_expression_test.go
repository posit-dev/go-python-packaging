// SPDX-License-Identifier: Apache-2.0 OR MIT

package license

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSPDXExpression(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		// Empty and whitespace
		{"empty string", "", nil},
		{"whitespace only", "   ", nil},

		// Simple identifiers
		{"single identifier", "MIT", []string{"MIT"}},
		{"single identifier with whitespace", "  MIT  ", []string{"MIT"}},
		{"identifier with version", "GPL-2.0-only", []string{"GPL-2.0-only"}},
		{"identifier with dots", "BSD-3-Clause", []string{"BSD-3-Clause"}},
		{"Apache license", "Apache-2.0", []string{"Apache-2.0"}},

		// Or-later (+) suffix
		{"or-later suffix converts to -or-later", "GPL-2.0+", []string{"GPL-2.0-or-later"}},
		{"LGPL or-later", "LGPL-2.1+", []string{"LGPL-2.1-or-later"}},
		{"already has -or-later suffix", "GPL-3.0-or-later", []string{"GPL-3.0-or-later"}},

		// OR expressions - disjunctive
		{"two licenses with OR", "MIT OR Apache-2.0", []string{"MIT", "Apache-2.0"}},
		{"three licenses with OR", "MIT OR Apache-2.0 OR BSD-3-Clause", []string{"MIT", "Apache-2.0", "BSD-3-Clause"}},
		{"OR is case-sensitive but we accept lowercase", "MIT or Apache-2.0", []string{"MIT", "Apache-2.0"}},

		// AND expressions - conjunctive
		{"two licenses with AND", "MIT AND Apache-2.0", []string{"MIT", "Apache-2.0"}},
		{"three licenses with AND", "MIT AND Apache-2.0 AND BSD-3-Clause", []string{"MIT", "Apache-2.0", "BSD-3-Clause"}},
		{"AND is case-sensitive but we accept lowercase", "MIT and Apache-2.0", []string{"MIT", "Apache-2.0"}},

		// WITH expressions - exceptions
		{"license with exception", "GPL-2.0-only WITH Classpath-exception-2.0", []string{"GPL-2.0-only"}},
		{"GPL-3.0 with exception", "GPL-3.0-or-later WITH Bison-exception-2.2", []string{"GPL-3.0-or-later"}},
		{"WITH is case-sensitive but we accept lowercase", "GPL-2.0-only with Classpath-exception-2.0", []string{"GPL-2.0-only"}},

		// Mixed expressions
		{"AND and OR mixed", "MIT AND Apache-2.0 OR GPL-3.0-only", []string{"MIT", "Apache-2.0", "GPL-3.0-only"}},
		{"OR with exception", "MIT OR GPL-3.0-only WITH Classpath-exception-2.0", []string{"MIT", "GPL-3.0-only"}},

		// Parentheses
		{"parentheses around single license", "(MIT)", []string{"MIT"}},
		{"parentheses around OR expression", "(MIT OR Apache-2.0)", []string{"MIT", "Apache-2.0"}},
		{"nested parentheses", "((MIT OR Apache-2.0) AND BSD-3-Clause)", []string{"MIT", "Apache-2.0", "BSD-3-Clause"}},
		{"complex nested expression", "(MIT AND Apache-2.0) OR (GPL-2.0-only AND BSD-3-Clause)", []string{"MIT", "Apache-2.0", "GPL-2.0-only", "BSD-3-Clause"}},
		{"precedence override with parens", "LGPL-2.1-only OR (MIT AND BSD-2-Clause)", []string{"LGPL-2.1-only", "MIT", "BSD-2-Clause"}},

		// LicenseRef
		{"LicenseRef custom license", "LicenseRef-Proprietary", []string{"LicenseRef-Proprietary"}},
		{"LicenseRef with numbers", "LicenseRef-Company-1.0", []string{"LicenseRef-Company-1.0"}},
		{"LicenseRef with standard license", "MIT OR LicenseRef-Company-License", []string{"MIT", "LicenseRef-Company-License"}},

		// DocumentRef:LicenseRef
		{"DocumentRef with LicenseRef", "DocumentRef-ext:LicenseRef-custom", []string{"DocumentRef-ext:LicenseRef-custom"}},

		// Deduplication
		{"duplicate licenses are deduplicated", "MIT AND MIT", []string{"MIT"}},
		{"duplicates across OR branches", "MIT OR MIT OR Apache-2.0", []string{"MIT", "Apache-2.0"}},

		// Real-world examples
		{"numpy-style expression", "BSD-3-Clause", []string{"BSD-3-Clause"}},
		{"dual-license expression", "Apache-2.0 OR MIT", []string{"Apache-2.0", "MIT"}},
		{"requests-style", "Apache-2.0", []string{"Apache-2.0"}},
		{"GPL with exception (common pattern)", "GPL-2.0-or-later WITH Bison-exception-2.2", []string{"GPL-2.0-or-later"}},
		{"complex real-world", "(Apache-2.0 OR MIT) AND BSD-3-Clause", []string{"Apache-2.0", "MIT", "BSD-3-Clause"}},

		// Edge cases
		{"extra whitespace", "  MIT   OR   Apache-2.0  ", []string{"MIT", "Apache-2.0"}},
		{"mixed case operators", "MIT Or Apache-2.0", []string{"MIT", "Apache-2.0"}},

		// Non-ASCII (Unicode) whitespace separators must split like ASCII spaces.
		// NBSP (U+00A0) is multi-byte in UTF-8, so a byte-wise tokenizer would
		// misread it and fail to split the expression.
		{"NBSP-separated OR expression", "MIT OR Apache-2.0", []string{"MIT", "Apache-2.0"}},
		{"NBSP-separated AND expression", "MIT AND Apache-2.0", []string{"MIT", "Apache-2.0"}},
		{"ideographic-space-separated OR expression", "MIT　OR　Apache-2.0", []string{"MIT", "Apache-2.0"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ParseSPDXExpression(tc.in))
		})
	}
}
