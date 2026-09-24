// SPDX-License-Identifier: Apache-2.0 OR MIT
//
// Portions of this file port test cases from pypa/packaging
// (https://github.com/pypa/packaging), specifically tests/test_markers.py's
// class TestMarker and TestOperatorEvaluation, used under the Apache License,
// Version 2.0 (dual-licensed Apache-2.0 OR BSD-2-Clause; see NOTICE for full
// license and copyright detail).
// Changed: translated from Python/pytest to Go table-driven tests; see the
// divergence notes below for cases where the Go expectation intentionally
// differs from upstream's literal assertion.
//
// Upstream pinned at 4eb0753dba8fcaaac8eb75463374e448f0931558 (identical at
// release 26.3: the only test_markers.py change since is a type annotation on
// TestNode.test_accepts_value, not ported - see below).
//
// Every upstream dict-valued evaluate() call is a PARTIAL environment: 18 of
// upstream's 40 evaluate( calls in this file pass a dict, and upstream fills
// every key the dict leaves out from the live interpreter (the issue's "~4"
// estimate was wrong). Every one is built here as baseEnvironment().With(dict)
// - never a struct literal, which would zero-fill the other ten fields and
// silently change the answer (see partial_environment_test.go). A bare
// `dict["extra"]` goes to the extraList argument instead, since With rejects
// "extra" as unknown (it is bound from the active-extras list, not the
// environment). A key that names neither a marker variable nor "extra" (e.g.
// `dict(a="a")` in test_new_string_rules) is irrelevant to a marker that never
// references it as an unquoted variable, and is dropped rather than passed to
// With.
//
// Upstream test functions with no counterpart here, and why:
//   - TestNode: an internal AST node type this package does not expose.
//   - TestDefaultEnvironment: reads the live interpreter (sys/platform/os);
//     this package has no live-environment reader to test.
//   - test_str_repr_eq_hash, test_different_markers_different_hashes,
//     test_compare_markers_to_other_objects, test_hash_eq_for_combined_markers,
//     test_str_preserves_nested_group_precedence: __str__/__repr__/__hash__/
//     __eq__ - Marker has String() (exercised by marker_test.go's own
//     round-trip tests) but no Equal/Hash/GoString.
//   - test_operator_rejects_non_marker, test_inplace_operators_fallback,
//     test_right_hand_ops_and_typeerror, test_chaining_associativity_and_str,
//     test_and_operator_str_equality, test_or_operator_str_equality: the
//     operator-overload group (&, &=, __rand__, TypeError) - Marker has no
//     combinator methods; a combined marker is written as one parsed string.
//   - every test_pickle_* (8 of them): no Go serialization equivalent.
//   - test_environment_with_no_extras, test_extras_and_dependency_groups_disallowed:
//     use evaluate(context=...), a parameter Evaluate has no equivalent of.
//   - test_extras_and_dependency_groups(_disallowed),
//     test_missing_environment_key_raises_undefined_environment_name,
//     test_set_valued_marker_on_lhs_raises_undefined_comparison,
//     test_membership_value_str_normalization(_negated): all exercise the
//     set-valued "extras" (plural, PEP 685) or "dependency_groups" (PEP 735)
//     marker variables. Neither is a recognized Variable token in this
//     package's grammar (only singular "extra", bound from Evaluate's
//     extraList) - so these markers do not even parse here.
//   - test_environment_assumes_empty_extra, test_environment_with_extra_none:
//     redundant with test_evaluates' own "extra" rows and with
//     evaluate_test.go's TestEvaluate_Extra_SetMembership.
//   - test_evaluate_setuptools_legacy_markers: redundant with
//     test_evaluate_pep345_markers' platform.python_implementation row and
//     marker_test.go's TestParse_PythonImplementationAlias.
//   - test_python_full_version_untagged_user_provided, test_python_full_version_untagged:
//     the bare-trailing-"+" repair (repairPythonFullVersion) runs only inside
//     EnvironmentFromTarget, not inside Evaluate given a caller-supplied
//     Environment. Porting the first would surface a real gap - Evaluate does
//     not repair "3.11.1+" before a version comparison - but fixing it is out
//     of this issue's scope; the second additionally mocks the live
//     interpreter. Candidate follow-up, not filed.
//   - test_evaluate_does_not_mutate_cached_environment: no live-environment
//     cache exists in Go; Environment is a plain, uncached struct.

package marker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- TestOperatorEvaluation ---

// Ported from pypa/packaging tests/test_markers.py,
// TestOperatorEvaluation.test_prefers_pep440.
// Pinned: 4eb0753dba8fcaaac8eb75463374e448f0931558 (L104-L110 at that commit).
// https://github.com/pypa/packaging/blob/4eb0753dba8fcaaac8eb75463374e448f0931558/tests/test_markers.py#L104-L110
func TestConformance_PrefersPEP440(t *testing.T) {
	env, err := baseEnvironment().With(map[string]string{"python_full_version": "2.7.10"})
	require.NoError(t, err)
	assert.True(t, evalStr(t, `"2.7.9" < python_full_version`, env, nil))

	env, err = baseEnvironment().With(map[string]string{"python_full_version": "2.7.8"})
	require.NoError(t, err)
	assert.False(t, evalStr(t, `"2.7.9" < python_full_version`, env, nil))
}

// Ported from pypa/packaging tests/test_markers.py,
// TestOperatorEvaluation.test_new_string_rules. Every dict upstream passes
// here (dict(a="a")) names a key the marker text never references as an
// unquoted variable - both operands are quoted literals - so it is inert and
// dropped rather than passed to With.
// Pinned: 4eb0753dba8fcaaac8eb75463374e448f0931558 (L112-L123 at that commit).
// https://github.com/pypa/packaging/blob/4eb0753dba8fcaaac8eb75463374e448f0931558/tests/test_markers.py#L112-L123
func TestConformance_NewStringRules(t *testing.T) {
	env, err := baseEnvironment().With(map[string]string{"python_full_version": "c"})
	require.NoError(t, err)
	assert.False(t, evalStr(t, `"b" < python_full_version`, env, nil))

	env, err = baseEnvironment().With(map[string]string{"python_full_version": "a"})
	require.NoError(t, err)
	assert.False(t, evalStr(t, `"b" < python_full_version`, env, nil))

	assert.False(t, evalStr(t, `"b" > "a"`, baseEnvironment(), nil))
	assert.False(t, evalStr(t, `"b" < "a"`, baseEnvironment(), nil))
	assert.False(t, evalStr(t, `"b" >= "a"`, baseEnvironment(), nil))
	assert.False(t, evalStr(t, `"b" <= "a"`, baseEnvironment(), nil))
	assert.True(t, evalStr(t, `"a" <= "a"`, baseEnvironment(), nil))
}

// Ported from pypa/packaging tests/test_markers.py,
// TestOperatorEvaluation.test_fails_when_undefined and
// test_arbitrary_equality_on_non_version_key_is_undefined. Upstream raises
// UndefinedComparison for `~=`/`===` against a non-version-typed key;
// Evaluate's fixed bool signature cannot propagate an error, so it returns
// false instead - the same documented divergence already pinned by
// evaluate_test.go's TestEvaluate_TildeEqualAndArbitraryEqual_NonVersionKey_ReturnsFalse
// (see evaluate.go's evalStringOp doc comment).
// Pinned: 4eb0753dba8fcaaac8eb75463374e448f0931558 (L125-L137 at that commit).
// https://github.com/pypa/packaging/blob/4eb0753dba8fcaaac8eb75463374e448f0931558/tests/test_markers.py#L125-L137
func TestConformance_TildeEqualAndArbitraryEqual_NonVersionKey_Divergence(t *testing.T) {
	tests := []string{
		`'2.7.0' ~= os_name`,
		`os_name === 'posix'`,
		`sys_platform === 'linux'`,
	}
	for _, expr := range tests {
		t.Run(expr, func(t *testing.T) {
			assert.False(t, evalStr(t, expr, baseEnvironment(), nil),
				"upstream raises UndefinedComparison here; we return false (divergence, see evaluate.go)")
		})
	}
}

// Ported from pypa/packaging tests/test_markers.py,
// TestOperatorEvaluation.test_allows_prerelease.
// Pinned: 4eb0753dba8fcaaac8eb75463374e448f0931558 (L139-L142 at that commit).
// https://github.com/pypa/packaging/blob/4eb0753dba8fcaaac8eb75463374e448f0931558/tests/test_markers.py#L139-L142
func TestConformance_AllowsPrerelease(t *testing.T) {
	env, err := baseEnvironment().With(map[string]string{"python_full_version": "3.11.0a5"})
	require.NoError(t, err)
	assert.True(t, evalStr(t, `python_full_version > "3.6.2"`, env, nil))
}

// --- TestMarker.test_parses_valid / _invalid and friends ---

// The three lists upstream's parametrize crosses (VARIABLES, OPERATORS,
// VALUES), transcribed verbatim so test_parses_valid, test_parses_pep345_valid
// and test_parses_setuptools_legacy_valid can each express their own
// cross-product as a Go loop rather than a hand-picked subset.
var (
	confVariables = []string{
		"extra",
		"implementation_name",
		"implementation_version",
		"os_name",
		"platform_machine",
		"platform_release",
		"platform_system",
		"platform_version",
		"python_full_version",
		"python_version",
		"platform_python_implementation",
		"sys_platform",
	}
	confPEP345Variables = []string{
		"os.name",
		"sys.platform",
		"platform.version",
		"platform.machine",
		"platform.python_implementation",
	}
	confSetuptoolsVariables = []string{"python_implementation"}
	confOperators           = []string{"===", "==", ">=", "<=", "!=", "~=", ">", "<", "in", "not in"}
	confValues              = []string{
		"1.0",
		"5.6a0",
		"dog",
		"freebsd",
		"literally any string can go here",
		"things @#4 dsfd (((",
	}
)

// assertCrossProductParses asserts that Parse succeeds for every
// (variable, operator, value) triple in both operand orders, mirroring
// upstream's "{} {} {!r}".format(*i) / "{2!r} {1} {0}".format(*i) parametrize
// pair. None of confValues contains a single quote, so Python's repr() always
// single-quotes them; the Go literal does the same. Returns the case count
// (both orders) for t.Logf.
func assertCrossProductParses(t *testing.T, variables []string) int {
	t.Helper()
	n := 0
	for _, v := range variables {
		for _, op := range confOperators {
			for _, val := range confValues {
				lit := "'" + val + "'"
				n += 2
				if _, err := Parse(v + " " + op + " " + lit); err != nil {
					t.Errorf("Parse(%q) should succeed: %v", v+" "+op+" "+lit, err)
				}
				if _, err := Parse(lit + " " + op + " " + v); err != nil {
					t.Errorf("Parse(%q) should succeed: %v", lit+" "+op+" "+v, err)
				}
			}
		}
	}
	return n
}

// Ported from pypa/packaging tests/test_markers.py, TestMarker.test_parses_valid.
// Pinned: 4eb0753dba8fcaaac8eb75463374e448f0931558 (L204-L216 at that commit).
// https://github.com/pypa/packaging/blob/4eb0753dba8fcaaac8eb75463374e448f0931558/tests/test_markers.py#L204-L216
func TestConformance_ParsesValid(t *testing.T) {
	t.Logf("cross-product cases: %d", assertCrossProductParses(t, confVariables))
}

// Ported from pypa/packaging tests/test_markers.py, TestMarker.test_parses_pep345_valid.
// Pinned: 4eb0753dba8fcaaac8eb75463374e448f0931558 (L428-L440 at that commit).
// https://github.com/pypa/packaging/blob/4eb0753dba8fcaaac8eb75463374e448f0931558/tests/test_markers.py#L428-L440
func TestConformance_ParsesPEP345Valid(t *testing.T) {
	t.Logf("cross-product cases: %d", assertCrossProductParses(t, confPEP345Variables))
}

// Ported from pypa/packaging tests/test_markers.py, TestMarker.test_parses_setuptools_legacy_valid.
// Pinned: 4eb0753dba8fcaaac8eb75463374e448f0931558 (L469-L481 at that commit).
// https://github.com/pypa/packaging/blob/4eb0753dba8fcaaac8eb75463374e448f0931558/tests/test_markers.py#L469-L481
func TestConformance_ParsesSetuptoolsLegacyValid(t *testing.T) {
	t.Logf("cross-product cases: %d", assertCrossProductParses(t, confSetuptoolsVariables))
}

// Ported from pypa/packaging tests/test_markers.py, TestMarker.test_parses_invalid.
// Pinned: 4eb0753dba8fcaaac8eb75463374e448f0931558 (L218-L231 at that commit).
// https://github.com/pypa/packaging/blob/4eb0753dba8fcaaac8eb75463374e448f0931558/tests/test_markers.py#L218-L231
func TestConformance_ParsesInvalid(t *testing.T) {
	tests := []string{
		"this_isnt_a_real_variable >= '1.0'",
		"python_version",
		"(python_version)",
		"python_version >= 1.0 and (python_version)",
		`(python_version == "2.7" and os_name == "linux"`,
		`(python_version == "2.7") with random text`,
	}
	for _, s := range tests {
		t.Run(s, func(t *testing.T) {
			_, err := Parse(s)
			assert.Error(t, err, "input %q should be rejected", s)
		})
	}
}

// Ported from pypa/packaging tests/test_markers.py,
// TestMarker.test_parses_invalid_trailing_line_break.
// Pinned: 4eb0753dba8fcaaac8eb75463374e448f0931558 (L233-L236 at that commit).
// https://github.com/pypa/packaging/blob/4eb0753dba8fcaaac8eb75463374e448f0931558/tests/test_markers.py#L233-L236
func TestConformance_ParsesInvalidTrailingLineBreak(t *testing.T) {
	for _, lb := range []string{"\n", "\r", "\r\n"} {
		t.Run(escapeForName(lb), func(t *testing.T) {
			_, err := Parse(`python_version >= "3"` + lb)
			assert.Error(t, err, "trailing line break should be rejected")
		})
	}
}

// Ported from pypa/packaging tests/test_markers.py,
// TestMarker.test_parses_trailing_horizontal_whitespace.
// Pinned: 4eb0753dba8fcaaac8eb75463374e448f0931558 (L238-L242 at that commit).
// https://github.com/pypa/packaging/blob/4eb0753dba8fcaaac8eb75463374e448f0931558/tests/test_markers.py#L238-L242
func TestConformance_ParsesTrailingHorizontalWhitespace(t *testing.T) {
	want, err := Parse(`python_version >= "3"`)
	require.NoError(t, err)
	for _, ws := range []string{" ", "\t", " \t"} {
		t.Run(escapeForName(ws), func(t *testing.T) {
			got, err := Parse(`python_version >= "3"` + ws)
			require.NoError(t, err, "trailing horizontal whitespace is legal")
			assert.Equal(t, want.String(), got.String())
		})
	}
}

// escapeForName renders whitespace visibly so subtest names stay distinct
// (mirrors internal/pep508's own helper of the same purpose).
func escapeForName(s string) string {
	out := ""
	for _, r := range s {
		switch r {
		case '\n':
			out += "LF"
		case '\r':
			out += "CR"
		case '\t':
			out += "TAB"
		case ' ':
			out += "SP"
		default:
			out += string(r)
		}
	}
	return out
}

// Ported from pypa/packaging tests/test_markers.py,
// TestMarker.test_parses_invalid_malformed_quoted_string. Upstream also
// asserts an exact wrapped message and a tilde-caret rendering specific to
// Python's InvalidMarker text, which this file does not replicate - the
// byte-position-level assertions on malformed quoted strings live in
// internal/pep508/marker_test.go's #19401 escape table (this input overlaps
// it; duplication is fine, see the issue's escape-decoding note). Here we
// only confirm both upstream inputs are rejected.
// Pinned: 4eb0753dba8fcaaac8eb75463374e448f0931558 (L244-L267 at that commit).
// https://github.com/pypa/packaging/blob/4eb0753dba8fcaaac8eb75463374e448f0931558/tests/test_markers.py#L244-L267
func TestConformance_ParsesInvalidMalformedQuotedString(t *testing.T) {
	tests := []string{
		`os_name == "C:\"`,
		`os_name == "\x"`,
	}
	for _, s := range tests {
		t.Run(s, func(t *testing.T) {
			_, err := Parse(s)
			assert.Error(t, err, "input %q should be rejected", s)
		})
	}
}

// --- test_evaluates ---

// Ported from pypa/packaging tests/test_markers.py, TestMarker.test_evaluates.
// Pinned: 4eb0753dba8fcaaac8eb75463374e448f0931558 (L380-L426 at that commit).
// https://github.com/pypa/packaging/blob/4eb0753dba8fcaaac8eb75463374e448f0931558/tests/test_markers.py#L380-L426
func TestConformance_Evaluates(t *testing.T) {
	tests := []struct {
		name      string
		expr      string
		overrides map[string]string
		extra     []string
		want      bool
	}{
		// os.name substituted with the live os.name upstream, always true by
		// construction; baseEnvironment's OsName stands in for os.name here.
		{"os_name == live os.name", `os_name == "posix"`, nil, nil, true},
		{"os_name foo==foo", `os_name == "foo"`, map[string]string{"os_name": "foo"}, nil, true},
		{"os_name foo==bar", `os_name == "foo"`, map[string]string{"os_name": "bar"}, nil, false},
		{"'2.7' in python_version", `"2.7" in python_version`, map[string]string{"python_version": "2.7.5"}, nil, true},
		{"'2.7' not in python_version", `"2.7" not in python_version`, map[string]string{"python_version": "2.7"}, nil, false},
		{
			"os_name and python_version ~=, both true",
			`os_name == "foo" and python_version ~= "2.7.0"`,
			map[string]string{"os_name": "foo", "python_version": "2.7.6"},
			nil, true,
		},
		{
			"~= and (or), os_name foo",
			`python_version ~= "2.7.0" and (os_name == "foo" or os_name == "bar")`,
			map[string]string{"os_name": "foo", "python_version": "2.7.4"},
			nil, true,
		},
		{
			"~= and (or), os_name bar",
			`python_version ~= "2.7.0" and (os_name == "foo" or os_name == "bar")`,
			map[string]string{"os_name": "bar", "python_version": "2.7.4"},
			nil, true,
		},
		{
			"~= and (or), os_name other",
			`python_version ~= "2.7.0" and (os_name == "foo" or os_name == "bar")`,
			map[string]string{"os_name": "other", "python_version": "2.7.4"},
			nil, false,
		},
		// "extra" in the upstream dict goes to extraList, not With.
		{"extra security, active quux", `extra == "security"`, nil, []string{"quux"}, false},
		{"extra security, active security", `extra == "security"`, nil, []string{"security"}, true},
		{"extra SECURITY literal, active security", `extra == "SECURITY"`, nil, []string{"security"}, true},
		{"extra security literal, active SECURITY", `extra == "security"`, nil, []string{"SECURITY"}, true},
		{"extra pep-685-norm literal, active raw", `extra == "pep-685-norm"`, nil, []string{"PEP_685...norm"}, true},
		{
			"extra punctuation literal, active raw",
			`extra == "Different.punctuation..is...equal"`,
			nil, []string{"different__punctuation_is_EQUAL"}, true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := baseEnvironment().With(tt.overrides)
			require.NoError(t, err)
			assert.Equal(t, tt.want, evalStr(t, tt.expr, env, tt.extra))
		})
	}
}

// Ported from pypa/packaging tests/test_markers.py, TestMarker.test_evaluate_pep345_markers.
// Pinned: 4eb0753dba8fcaaac8eb75463374e448f0931558 (L442-L467 at that commit).
// https://github.com/pypa/packaging/blob/4eb0753dba8fcaaac8eb75463374e448f0931558/tests/test_markers.py#L442-L467
func TestConformance_EvaluatePEP345Markers(t *testing.T) {
	tests := []struct {
		name      string
		expr      string
		overrides map[string]string
		want      bool
	}{
		{"os.name == live os.name", `os.name == "posix"`, nil, true},
		{"sys.platform win32 vs linux2", `sys.platform == "win32"`, map[string]string{"sys_platform": "linux2"}, false},
		{"platform.version in Ubuntu", `platform.version in "Ubuntu"`, map[string]string{"platform_version": "#39"}, false},
		{"platform.machine x86_64", `platform.machine=='x86_64'`, map[string]string{"platform_machine": "x86_64"}, true},
		{
			"platform.python_implementation Jython vs CPython",
			`platform.python_implementation=='Jython'`,
			map[string]string{"platform_python_implementation": "CPython"}, false,
		},
		{
			"python_version 2.5 and not Jython",
			`python_version == '2.5' and platform.python_implementation!= 'Jython'`,
			map[string]string{"python_version": "2.7"}, false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := baseEnvironment().With(tt.overrides)
			require.NoError(t, err)
			assert.Equal(t, tt.want, evalStr(t, tt.expr, env, nil))
		})
	}
}

// --- extra normalization (singular "extra" only; see file header for why
// "extras"/"dependency_groups" have no counterpart) ---

// Ported from pypa/packaging tests/test_markers.py, TestMarker.test_extra_str_normalization.
// Pinned: 4eb0753dba8fcaaac8eb75463374e448f0931558 (L488-L495 at that commit).
// https://github.com/pypa/packaging/blob/4eb0753dba8fcaaac8eb75463374e448f0931558/tests/test_markers.py#L488-L495
func TestConformance_ExtraStrNormalization(t *testing.T) {
	lhs, err := Parse(`'S_P__A_M' == extra`)
	require.NoError(t, err)
	assert.Equal(t, `"s-p-a-m" == extra`, lhs.String())

	rhs, err := Parse(`extra == 'S_P__A_M'`)
	require.NoError(t, err)
	assert.Equal(t, `extra == "s-p-a-m"`, rhs.String())
}

// Ported from pypa/packaging tests/test_markers.py,
// TestMarker.test_extra_compared_to_variable_not_normalized.
// Pinned: 4eb0753dba8fcaaac8eb75463374e448f0931558 (L497-L501 at that commit).
// https://github.com/pypa/packaging/blob/4eb0753dba8fcaaac8eb75463374e448f0931558/tests/test_markers.py#L497-L501
func TestConformance_ExtraComparedToVariableNotNormalized(t *testing.T) {
	m1, err := Parse("extra == os_name")
	require.NoError(t, err)
	assert.Equal(t, "extra == os_name", m1.String())

	m2, err := Parse("os_name == extra")
	require.NoError(t, err)
	assert.Equal(t, "os_name == extra", m2.String())
}

// Ported from pypa/packaging tests/test_markers.py,
// TestMarker.test_nested_extra_str_normalization.
// Pinned: 4eb0753dba8fcaaac8eb75463374e448f0931558 (L503-L512 at that commit).
// https://github.com/pypa/packaging/blob/4eb0753dba8fcaaac8eb75463374e448f0931558/tests/test_markers.py#L503-L512
func TestConformance_NestedExtraStrNormalization(t *testing.T) {
	m, err := Parse(`(extra == "Foo_Bar" or extra == "Baz") and python_version >= "3"`)
	require.NoError(t, err)
	assert.Equal(t, `(extra == "foo-bar" or extra == "baz") and python_version >= "3"`, m.String())

	env, err := baseEnvironment().With(map[string]string{"python_version": "3.12"})
	require.NoError(t, err)
	assert.True(t, m.Evaluate(env, []string{"foo-bar"}))
}

// Ported from pypa/packaging tests/test_markers.py, TestMarker.test_version_like_equality.
// Pinned: 4eb0753dba8fcaaac8eb75463374e448f0931558 (L614-L633 at that commit).
// https://github.com/pypa/packaging/blob/4eb0753dba8fcaaac8eb75463374e448f0931558/tests/test_markers.py#L614-L633
func TestConformance_VersionLikeEquality(t *testing.T) {
	tests := []struct {
		name  string
		expr  string
		extra []string
		want  bool
	}{
		{"v2, no active extra", `extra == "v2"`, nil, false},
		{"v2, active empty string", `extra == "v2"`, []string{""}, false},
		{"v2, active v2", `extra == "v2"`, []string{"v2"}, true},
		{"v2, active v2a3", `extra == "v2"`, []string{"v2a3"}, false},
		{"v2a3, active v2", `extra == "v2a3"`, []string{"v2"}, false},
		{"v2a3, active v2a3", `extra == "v2a3"`, []string{"v2a3"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, evalStr(t, tt.expr, baseEnvironment(), tt.extra))
		})
	}
}

// --- module-level combined-marker tests ---
//
// Upstream builds these with Marker.__and__ (m1 & m2); Go's Marker has no
// combinator method (see file header), so each is expressed as one parsed
// "a and b [and c]" string - semantically identical to the combined result,
// which is all these tests exercise (str/hash equality between the two
// spellings is the operator-overload group, not ported).

// Ported from pypa/packaging tests/test_markers.py, test_and_operator_evaluates_true.
// Pinned: 4eb0753dba8fcaaac8eb75463374e448f0931558 (L636-L640 at that commit).
// https://github.com/pypa/packaging/blob/4eb0753dba8fcaaac8eb75463374e448f0931558/tests/test_markers.py#L636-L640
func TestConformance_AndOperatorEvaluatesTrue(t *testing.T) {
	env, err := baseEnvironment().With(map[string]string{"python_version": "3.8", "os_name": "posix"})
	require.NoError(t, err)
	assert.True(t, evalStr(t, `python_version >= "3.6" and os_name == "posix"`, env, nil))
}

// Ported from pypa/packaging tests/test_markers.py, test_or_operator_evaluates_true.
// Pinned: 4eb0753dba8fcaaac8eb75463374e448f0931558 (L650-L654 at that commit).
// https://github.com/pypa/packaging/blob/4eb0753dba8fcaaac8eb75463374e448f0931558/tests/test_markers.py#L650-L654
func TestConformance_OrOperatorEvaluatesTrue(t *testing.T) {
	env, err := baseEnvironment().With(map[string]string{"python_version": "3.7", "os_name": "windows"})
	require.NoError(t, err)
	assert.True(t, evalStr(t, `python_version < "3.6" or os_name == "windows"`, env, nil))
}

// Ported from pypa/packaging tests/test_markers.py, test_evaluation_of_combined_markers.
// Pinned: 4eb0753dba8fcaaac8eb75463374e448f0931558 (L720-L727 at that commit).
// https://github.com/pypa/packaging/blob/4eb0753dba8fcaaac8eb75463374e448f0931558/tests/test_markers.py#L720-L727
func TestConformance_EvaluationOfCombinedMarkers(t *testing.T) {
	env, err := baseEnvironment().With(map[string]string{
		"python_version": "3.8", "os_name": "posix", "platform_system": "Linux",
	})
	require.NoError(t, err)
	assert.True(t, evalStr(t,
		`python_version >= "3.6" and os_name == "posix" and platform_system == "Linux"`,
		env, nil))
}
