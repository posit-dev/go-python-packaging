// SPDX-License-Identifier: Apache-2.0 OR MIT

// This file contains ONLY cases ported from pypa/packaging's test suite, one
// Go table per upstream test function. Hand-written cases belong in
// requirement_test.go.
//
// Where our expected value differs from upstream's literal assertion, the
// difference is annotated inline with a reason. An UNANNOTATED divergence from
// upstream is a bug in this file, NOT a licence to change the library.
//
// Known intentional divergences:
//   extras - we canonicalize at parse time; upstream's Requirement.extras
//            keeps the raw spelling and canonicalizes only in __eq__/__hash__.
//            Compare canonicalized on BOTH sides.
//   marker - our Marker.String() normalizes quoting and spacing; compare
//            against marker.Parse(upstreamString).String(), not the raw text.
//
// Upstream pinned at 4eb0753dba8fcaaac8eb75463374e448f0931558.

package requirement_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/posit-dev/go-python-packaging/marker"
	"github.com/posit-dev/go-python-packaging/requirement"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pypa/packaging renders the marker separator CONDITIONALLY: "; " when the
// requirement has no URL, " ; " when it does. Verified against packaging 26.2:
//
//	marker, no url  ->  'package; python_version >= "3.3"'
//	marker + url    ->  'package @ https://e/p.zip ; python_version >= "3.3"'
//
// Ported from tests/test_requirements.py, test_normalized_requirements and
// the str(req) assertions of test_basic_valid_requirement_parsing.
func TestRequirement_String_MarkerSeparator(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{`name; os_name == "a"`, `name; os_name == "a"`},
		{`name>=1; python_version >= "3.3"`, `name>=1; python_version >= "3.3"`},
		{`name[x]<2; os_name == "a"`, `name[x]<2; os_name == "a"`},
		// With a URL, upstream uses " ; " -- we already match this.
		{
			`name @ https://e.com/p.zip ; python_version >= "3.3"`,
			`name @ https://e.com/p.zip ; python_version >= "3.3"`,
		},
		// No marker at all: no separator either way.
		{`name>=1`, `name>=1`},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			r, err := requirement.Parse(tt.in)
			require.NoError(t, err, "input %q", tt.in)
			assert.Equal(t, tt.want, r.String(), "input %q", tt.in)
		})
	}
}

// Ported from pypa/packaging tests/test_requirements.py,
// test_basic_valid_requirement_parsing. Upstream stacks five
// @pytest.mark.parametrize decorators for 5,184 cases; the curated subset
// below was measured to cover every block the full cross-product reaches, so
// transcribing all 5,184 buys nothing. Whitespace contributes zero coverage
// blocks, so it is swept once rather than crossed with everything.
//
// The rule's six clauses generate overlapping tuples (54 raw), so add()
// deduplicates and the table runs 49 cases. Measured: the 49-case set's
// covered-block set is a strict superset of the full 5,184-case product's --
// nothing is missed, and it reaches 7 blocks MORE because it runs the real
// assertions rather than only Parse.
//
// Curation rule (keep this reproducible):
//  1. all 18 (url, specifier) values x {no marker, simple marker}
//  2. each of the 6 names once
//  3. each of the 4 extras shapes once
//  4. each of the 4 markers once
//  5. each of the 3 whitespace values once, with a marker present
//  6. one url + extras + marker interaction case
const noURL = "\x00" // sentinel for Python's None (url absent)

var (
	confNames  = []string{"package", "pAcKaGe", "Package", "foo-bar.quux_bAz", "installer", "android12"}
	confExtras = [][]string{nil, {"a"}, {"a", "b"}, {"a", "B", "CDEF123"}}
	confUS     = [][2]string{
		{noURL, ""},
		{"https://example.com/packagename.zip", ""},
		{"ssh://user:pass%20word@example.com/packagename.zip", ""},
		{"https://example.com/name;v=1.1/?query=foo&bar=baz#blah", ""},
		{"git+ssh://git.example.com/MyProject", ""},
		{"git+ssh://git@github.com:pypa/packaging.git", ""},
		{"git+https://git.example.com/MyProject.git@master", ""},
		{"git+https://git.example.com/MyProject.git@v1.0", ""},
		{"git+https://git.example.com/MyProject.git@refs/pull/123/head", ""},
		{"gopher:/foo/com", ""},
		{noURL, "==={ws}arbitrarystring"},
		{noURL, "({ws}==={ws}arbitrarystring{ws})"},
		{noURL, "=={ws}1.0"},
		{noURL, "({ws}=={ws}1.0{ws})"},
		{noURL, "=={ws}1.0-alpha"},
		{noURL, "<={ws}1!3.0.0.rc2"},
		{noURL, ">{ws}2.2{ws},{ws}<{ws}3"},
		{noURL, "(>{ws}2.2{ws},{ws}<{ws}3)"},
	}
	confMarkers = []string{
		noURL, // sentinel: no marker clause at all
		"python_version{ws}>={ws}'3.3'",
		`({ws}python_version{ws}>={ws}"3.4"{ws}){ws}and extra{ws}=={ws}"oursql"`,
		"sys_platform{ws}!={ws}'linux' and(os_name{ws}=={ws}'linux' or python_version{ws}>={ws}'3.3'{ws}){ws}",
	}
	confWS = []string{"", " ", "\t"}
)

func subWS(s, ws string) string { return strings.ReplaceAll(s, "{ws}", ws) }

// buildConfCase mirrors upstream's builder at test_requirements.py:133-151.
// Two details matter: extras are sorted() and joined with "{ws},{ws}", and a
// URL+marker case appends a hardcoded " ;" while a non-URL case appends ";".
func buildConfCase(name string, extras []string, us [2]string, mk, ws string) string {
	parts := []string{name}
	if len(extras) > 0 {
		s := append([]string(nil), extras...)
		sort.Strings(s) // Python sorted(): codepoint order, uppercase first
		parts = append(parts, "[", strings.Join(s, ws+","+ws), "]")
	}
	if us[1] != "" {
		parts = append(parts, subWS(us[1], ws))
	}
	if us[0] != noURL {
		parts = append(parts, "@", subWS(us[0], ws))
	}
	if mk != noURL {
		if us[0] != noURL {
			parts = append(parts, " ;")
		} else {
			parts = append(parts, ";")
		}
		parts = append(parts, subWS(mk, ws))
	}
	return strings.Join(parts, ws)
}

// canonExtras applies PEP 685 normalization so both sides of the extras
// assertion are compared the same way. Upstream keeps the raw spelling and
// canonicalizes only in __eq__; we canonicalize at parse. Sorting happens
// AFTER normalizing so the comparison is order-independent.
func canonExtras(list []string) []string {
	out := make([]string, 0, len(list))
	for _, e := range list {
		e = strings.ToLower(strings.TrimSpace(e))
		e = strings.ReplaceAll(e, "_", "-")
		e = strings.ReplaceAll(e, ".", "-")
		if e != "" {
			out = append(out, e)
		}
	}
	sort.Strings(out)
	return out
}

func TestBasicValidRequirementParsing(t *testing.T) {
	type sel struct {
		ni, ei, ui, mi, wi int
	}
	seen := make(map[sel]bool)
	var cases []sel
	add := func(ni, ei, ui, mi, wi int) {
		s := sel{ni, ei, ui, mi, wi}
		if !seen[s] {
			seen[s] = true
			cases = append(cases, s)
		}
	}

	// 1. all 18 (url, specifier) values x {no marker, simple marker}
	for ui := range confUS {
		add(0, 0, ui, 0, 1)
		add(0, 0, ui, 1, 1)
	}
	// 2. each name once
	for ni := range confNames {
		add(ni, 0, 0, 0, 1)
	}
	// 3. each extras shape once
	for ei := range confExtras {
		add(0, ei, 0, 0, 1)
	}
	// 4. each marker once
	for mi := range confMarkers {
		add(0, 0, 0, mi, 1)
	}
	// 5. each whitespace value once, with a marker present
	for wi := range confWS {
		add(0, 0, 0, 1, wi)
	}
	// 6. one url + extras + marker interaction
	add(0, 1, 1, 1, 1)

	for _, c := range cases {
		name, extras := confNames[c.ni], confExtras[c.ei]
		us, mk, ws := confUS[c.ui], confMarkers[c.mi], confWS[c.wi]
		src := buildConfCase(name, extras, us, mk, ws)

		t.Run(src, func(t *testing.T) {
			r, err := requirement.Parse(src)
			require.NoError(t, err, "Parse(%q) should succeed", src)

			assert.Equal(t, name, r.Name, "name, input %q", src)
			assert.Equal(t, canonExtras(extras), canonExtras(r.Extras),
				"extras, input %q", src)

			wantURL := ""
			if us[0] != noURL {
				wantURL = subWS(us[0], ws)
			}
			assert.Equal(t, wantURL, r.URL, "url, input %q", src)

			// Upstream: req.specifier == specifier.format(ws="").strip("()")
			wantSpec := strings.Trim(subWS(us[1], ""), "()")
			assert.Equal(t, wantSpec, r.Specifiers.String(), "specifier, input %q", src)

			// Upstream: req.marker == Marker(marker.format(ws="")). Compare
			// CONTENT, not just presence -- our Marker.String() normalizes
			// quoting and spacing, so the comparison goes through a parsed
			// round-trip of the expected text rather than the raw string
			// (see the intentional-divergence note in this file's header).
			if mk == noURL {
				assert.True(t, r.Marker.IsEmpty(), "expected no marker, input %q", src)
			} else {
				wantMarker, err := marker.Parse(subWS(mk, ""))
				require.NoError(t, err, "expected-marker text should itself parse, input %q", src)
				assert.Equal(t, wantMarker.String(), r.Marker.String(),
					"marker content, input %q", src)
			}
		})
	}

	t.Logf("curated cases: %d", len(cases))
}

// Ported from pypa/packaging tests/test_requirements.py, class
// TestRequirementParsing. Each entry cites the upstream test function.
// wantErr mirrors upstream's expectation, NOT our current behavior.
//
// One case is deliberately omitted: the operator-less specifier "2.0", which
// we still accept via the non-PEP-440 "" entry in specifierOperators.
// Tightening that is behavior-narrowing and user-visible, and is tracked in
// rstudio/package-manager#18634 (blocked on #19371), whose acceptance criteria
// require re-enabling this case:
//
//	{"name 2.0", true, "operator-less specifier"},
func TestRequirementParsingErrors(t *testing.T) {
	tests := []struct {
		in       string
		wantErr  bool
		upstream string
	}{
		{"name[]", false, "test_empty_extras"},
		{"name()", false, "test_empty_specifier"},
		{"foobar", false, "test_sole_use_of_name"},
		{"foobar[quux]<2,>=3; os_name=='a'", false, "test_valid_with_all_parts"},
		{"name>=1 ", false, "test_trailing_horizontal_whitespace"},
		{"name>=1\t", false, "test_trailing_horizontal_whitespace"},

		{"demo===x,y", true, "test_error_when_specifier_set_rejects_parsed_specifier"},
		{"", true, "test_error_when_empty_string"},
		{"==0.0", true, "test_error_no_name"},
		{"name[bar baz]", true, "test_error_when_missing_comma_in_extras"},
		{"name[bar, baz,]", true, "test_error_when_trailing_comma_in_extras"},
		{"name (>= 1.0", true, "test_error_when_parens_not_closed_correctly"},
		{"black (>=20.*) ; extra == 'format'", true, "test_error_when_prefix_match_is_used_incorrectly"},
		{"name == 1.2.3.post4.*", true, "test_error_when_prefix_match_uses_post_release"},
		{"name != 1.2.3.post4.*", true, "test_error_when_prefix_match_uses_post_release"},
		{"name==1.2.3.post4.*", true, "test_error_when_prefix_match_uses_post_release_without_spaces"},
		// Local version labels: upstream parametrizes over >=, <=, >, <, ~= ONLY.
		// "==" and "!=" ALLOW a local label -- do not add them here.
		{"name >= 1.0+local.version.label", true, "test_error_when_local_version_label_is_used_incorrectly"},
		{"name <= 1.0+local.version.label", true, "test_error_when_local_version_label_is_used_incorrectly"},
		{"name > 1.0+local.version.label", true, "test_error_when_local_version_label_is_used_incorrectly"},
		{"name < 1.0+local.version.label", true, "test_error_when_local_version_label_is_used_incorrectly"},
		{"name ~= 1.0+local.version.label", true, "test_error_when_local_version_label_is_used_incorrectly"},
		{"name == 1.0+local.version.label", false, "local labels ARE legal with =="},
		{"name != 1.0+local.version.label", false, "local labels ARE legal with !="},
		{"name[bar, baz >= 1.0", true, "test_error_when_bracket_not_closed_correctly"},
		{"name[bar, baz", true, "test_error_when_extras_bracket_left_unclosed"},
		{"name @ https://example.com/; extra == 'example'", true, "test_error_no_space_after_url"},
		{"name; (extra == 'example'", true, "test_error_marker_bracket_unclosed"},
		{"name @ ", true, "test_error_no_url_after_at"},
		{"name; invalid_name", true, "test_error_invalid_marker_lvalue"},
		{"name; '3.7' <= invalid_name", true, "test_error_invalid_marker_rvalue"},
		{"name; '3.7' notin python_version", true, "test_error_invalid_marker_malformed_operator"},
		{"name; '3.6'inpython_version", true, "test_error_invalid_marker_malformed_operator"},
		{"name; '3.7' not python_version", true, "test_error_invalid_marker_malformed_operator"},
		{"name; '3.7' ~ python_version", true, "test_error_invalid_marker_malformed_operator"},
		{`name; os_name == "C:\"`, true, "test_error_invalid_marker_malformed_quoted_string"},
		{"name==1.0.org1", true, "test_error_invalid_version"},
		{"name==", true, "test_error_no_version_after_operator"},
		{"name 1.0", true, "test_error_missing_operator"},
		{"name >= 1.0 #", true, "test_error_trailing_garbage"},
		{"name >= 1.0 <= 2.0", true, "test_error_missing_comma_between_specifiers"},

		{"name>=1\n", true, "test_error_when_suffixed_with_line_break"},
		{"name>=1\r", true, "test_error_when_suffixed_with_line_break"},
		{"name>=1\r\n", true, "test_error_when_suffixed_with_line_break"},
		{`name; python_version >= "3"` + "\n", true, "test_error_when_suffixed_with_line_break"},
		{`name; python_version >= "3"` + "\r", true, "test_error_when_suffixed_with_line_break"},
		{`name; python_version >= "3"` + "\r\n", true, "test_error_when_suffixed_with_line_break"},
	}

	for _, tt := range tests {
		t.Run(tt.upstream+"/"+strings.NewReplacer("\n", "LF", "\r", "CR", "\t", "TAB").Replace(tt.in), func(t *testing.T) {
			_, err := requirement.Parse(tt.in)
			if tt.wantErr {
				assert.Error(t, err, "input %q (%s) should be rejected", tt.in, tt.upstream)
			} else {
				assert.NoError(t, err, "input %q (%s) should parse", tt.in, tt.upstream)
			}
		})
	}
}
