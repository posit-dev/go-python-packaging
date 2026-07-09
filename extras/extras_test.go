// SPDX-License-Identifier: Apache-2.0 OR MIT

package extras

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"mixed case with underscore", "Foo_Bar", "foo-bar"},
		{"dot and double underscore", "a.b__c", "a-b-c"},
		{"uppercase with repeated hyphen", "A---B", "a-b"},
		{"already normal is idempotent", "already-normal", "already-normal"},
		{"empty string", "", ""},
		{"leading separators", "__foo", "-foo"},
		{"trailing separators", "foo__", "foo-"},
		{"leading and trailing separators", "..foo..", "-foo-"},
		{"mixed run of separators", "foo_.-bar", "foo-bar"},
		{"uppercase with dots", "Foo.Bar.Baz", "foo-bar-baz"},
		{"single character", "x", "x"},
		{"only separators", "___", "-"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Normalize(c.in)
			if got != c.want {
				t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestNormalizeIdempotent(t *testing.T) {
	inputs := []string{"Foo_Bar", "a.b__c", "A---B", "already-normal", "", "__foo", "foo__"}
	for _, in := range inputs {
		once := Normalize(in)
		twice := Normalize(once)
		if once != twice {
			t.Errorf("Normalize not idempotent for %q: Normalize(%q) = %q, Normalize(%q) = %q", in, in, once, once, twice)
		}
	}
}
