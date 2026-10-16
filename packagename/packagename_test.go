// SPDX-License-Identifier: Apache-2.0 OR MIT

package packagename

import (
	"testing"

	"github.com/posit-dev/go-python-packaging/extras"
)

// TestNormalizeMatchesExtras checks that the PEP 503 and PEP 685 entry points
// agree, including on non-ASCII and invalid names. The transformation itself is covered by
// extras/extras_test.go and both conformance tables.
func TestNormalizeMatchesExtras(t *testing.T) {
	inputs := []string{
		"", "x", "Django", "zope.interface", "Flask_SQLAlchemy",
		"ruamel.yaml.clib", "A---B", "__foo", "foo__", "..foo..", "___",
		"foo_.-bar", "ÄBC_def", "hi\n",
	}
	for _, in := range inputs {
		if got, want := Normalize(in), extras.Normalize(in); got != want {
			t.Errorf("packagename.Normalize(%q) = %q, extras.Normalize(%q) = %q", in, got, in, want)
		}
	}
}
