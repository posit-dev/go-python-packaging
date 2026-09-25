// SPDX-License-Identifier: Apache-2.0 OR MIT
//
// Portions of this file are ported from pypa/packaging
// (https://github.com/pypa/packaging), specifically packaging/markers.py's
// _eval_op and its operator/specifier dispatch (including
// MARKERS_REQUIRING_VERSION, reflected below as versionTypedVars), used
// under the Apache License, Version 2.0 (dual-licensed Apache-2.0 OR
// BSD-2-Clause; see NOTICE for full license and copyright detail).
// Changed: translated from Python to Go; an unresolvable comparison records
// an Undecidable value instead of raising UndefinedEnvironmentName, and
// version-typed comparisons delegate to package version instead of
// packaging.specifiers.

package marker

import (
	"strings"

	"github.com/posit-dev/go-python-packaging/extras"
	"github.com/posit-dev/go-python-packaging/internal/pep508"
	"github.com/posit-dev/go-python-packaging/version"
)

// versionTypedVars is the set of marker variables packaging attempts a PEP
// 440 version comparison against before falling back to string semantics
// (packaging's MARKERS_REQUIRING_VERSION). Note platform_version is NOT a
// member - only these four are - and this set differs from uv's (uv omits
// platform_release); we follow packaging, our correctness anchor.
var versionTypedVars = map[string]struct{}{
	"python_version":         {},
	"python_full_version":    {},
	"implementation_version": {},
	"platform_release":       {},
}

// Evaluate reports whether the marker is satisfied in env, with the given
// set of active extras bound to the `extra` variable. Pass nil extras for a
// metadata-context evaluation with no active extras.
func (m Marker) Evaluate(env Environment, extraList []string) bool {
	result, _ := m.EvaluateUndecidable(env, extraList)
	return result
}

// EvaluateUndecidable reports whether the marker is satisfied in env, and which
// of its comparisons could not be decided. When the returned slice is empty the
// bool is authoritative.
//
// Two things make a comparison undecidable, and neither is recoverable from
// Evaluate's bare bool:
//
//   - An environment variable resolves to "". EnvironmentFromTarget legitimately
//     cannot know PlatformRelease or PlatformVersion for a DECLARED target and
//     leaves them empty, and comparing "" against anything is meaningless.
//
//   - `~=` or `===` reaches the generic string-operator table, which has no
//     semantics for them (evalStringOp returns false; packaging raises
//     UndefinedComparison). Note this cannot happen when either operand is
//     version-typed and both sides parse as PEP 440 versions, because the
//     specifier path answers first and answers correctly.
//
// Ordered comparisons on string operands are NOT undecidable: `<` and `>`
// returning false and `<=`/`>=` collapsing to equality is faithful to
// pypa/packaging, whose operator table is the same.
//
// A caller that must not discard a dependency edge -- building a mirror, an
// offline bundle -- should treat a non-empty slice as "include regardless of the
// bool". Short-circuiting means only the comparisons actually reached are
// reported, which is what the caller wants: an `and` that was decided false by
// its first operand needs no further explanation.
func (m Marker) EvaluateUndecidable(env Environment, extraList []string) (bool, []Undecidable) {
	if m.ast == nil {
		return true, nil
	}
	active := normalizeExtraSet(extraList)
	var und []Undecidable
	result := evalExpr(m.ast, env, active, &und)
	return result, und
}

// normalizeExtraSet normalizes each active extra name (extras.Normalize) at
// evaluation time and returns it as a membership set. The literal side of an
// `extra` comparison is already normalized once at parse time (see
// internal/pep508's normalizeExtraLiteral); only the caller-supplied active
// list needs normalizing here, on Evaluate's hot path.
func normalizeExtraSet(extraList []string) map[string]struct{} {
	set := make(map[string]struct{}, len(extraList))
	for _, e := range extraList {
		set[extras.Normalize(e)] = struct{}{}
	}
	return set
}

// evalExpr walks the marker AST: a *pep508.BoolExpr short-circuits its
// "and"/"or" operands, a *pep508.CompareExpr evaluates a single comparison.
func evalExpr(e pep508.Expr, env Environment, active map[string]struct{}, und *[]Undecidable) bool {
	switch n := e.(type) {
	case *pep508.BoolExpr:
		return evalBool(n, env, active, und)
	case *pep508.CompareExpr:
		return evalCompare(n, env, active, und)
	default:
		// Unreachable: pep508.Expr has exactly these two implementations.
		return false
	}
}

func evalBool(n *pep508.BoolExpr, env Environment, active map[string]struct{}, und *[]Undecidable) bool {
	switch n.Op {
	case pep508.And:
		for _, operand := range n.Operands {
			if !evalExpr(operand, env, active, und) {
				return false
			}
		}
		return true
	case pep508.Or:
		for _, operand := range n.Operands {
			if evalExpr(operand, env, active, und) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// evalCompare implements the unified per-comparison dispatch ported from
// packaging's _eval_op: `extra` comparisons are set-membership (rule 3);
// otherwise a version comparison is attempted whenever either operand names
// one of the four version-typed variables (rule 1), falling back on any
// failure - or immediately, for any other variable - to a single generic
// string-operator table (rule 2).
func evalCompare(n *pep508.CompareExpr, env Environment, active map[string]struct{}, und *[]Undecidable) bool {
	if isExtraVar(n.Lhs) {
		return evalExtra(n.Op, n.Rhs, env, active)
	}
	if isExtraVar(n.Rhs) {
		return evalExtra(n.Op, n.Lhs, env, active)
	}

	lhsVal := resolveOperand(n.Lhs, env)
	rhsVal := resolveOperand(n.Rhs, env)

	// An environment variable with no value cannot be compared meaningfully. This
	// is reported rather than silently answered, because the empty value is often
	// the caller's own gap (a declared target has no kernel version to supply)
	// rather than a property of the environment.
	noteEmptyEnvVar(n, n.Lhs, lhsVal, und)
	noteEmptyEnvVar(n, n.Rhs, rhsVal, und)

	if isVersionTypedOperand(n.Lhs) || isVersionTypedOperand(n.Rhs) {
		if result, ok := tryVersionCompare(n.Op, lhsVal, rhsVal); ok {
			return result
		}
	}
	if n.Op == "~=" || n.Op == "===" {
		note(und, Undecidable{
			Expr:   n.String(),
			Reason: "operator has no string semantics",
		})
	}
	return evalStringOp(n.Op, lhsVal, rhsVal)
}

// noteEmptyEnvVar records an undecidable when operand names an environment
// variable that resolved to the empty string. A literal "" is not reported: the
// marker author wrote it deliberately.
func noteEmptyEnvVar(n *pep508.CompareExpr, operand pep508.Operand, value string, und *[]Undecidable) {
	if value != "" {
		return
	}
	v, ok := operand.(pep508.EnvVar)
	if !ok {
		return
	}
	note(und, Undecidable{
		Expr:   n.String(),
		Reason: "environment variable is empty",
		Var:    v.Name,
	})
}

// evalExtra evaluates a comparison where one side is the `extra` variable
// and other is the opposite operand (the value side, whichever operand
// order the marker was written in). Only == and != are meaningful for
// set-membership against the active-extras set; any other operator is
// inapplicable and evaluates false.
func evalExtra(op pep508.CompareOp, other pep508.Operand, env Environment, active map[string]struct{}) bool {
	lit := resolveOperand(other, env)
	_, present := active[lit]
	switch op {
	case "==":
		return present
	case "!=":
		return !present
	default:
		return false
	}
}

// isVersionTypedOperand reports whether operand is an EnvVar naming one of
// the four version-typed marker variables.
func isVersionTypedOperand(operand pep508.Operand) bool {
	v, ok := operand.(pep508.EnvVar)
	if !ok {
		return false
	}
	_, isVersionTyped := versionTypedVars[v.Name]
	return isVersionTyped
}

func isExtraVar(operand pep508.Operand) bool {
	v, ok := operand.(pep508.EnvVar)
	return ok && v.Name == "extra"
}

// resolveOperand resolves an operand (in source position, lhs or rhs) to its
// string value: a Literal's Value verbatim, or an EnvVar's value looked up
// in env.
func resolveOperand(operand pep508.Operand, env Environment) string {
	switch v := operand.(type) {
	case pep508.Literal:
		return v.Value
	case pep508.EnvVar:
		return envVarValue(v.Name, env)
	default:
		return ""
	}
}

// envVarValue maps a canonical PEP 508 marker variable name to its value in
// env. `extra` is handled separately (evalExtra) and never reaches here;
// every other name is guaranteed valid by Parse (unknown variables are a
// parse-time syntax error), so the default case is unreachable in practice.
func envVarValue(name string, env Environment) string {
	switch name {
	case "os_name":
		return env.OsName
	case "sys_platform":
		return env.SysPlatform
	case "platform_machine":
		return env.PlatformMachine
	case "platform_python_implementation":
		return env.PlatformPythonImplementation
	case "platform_release":
		return env.PlatformRelease
	case "platform_system":
		return env.PlatformSystem
	case "platform_version":
		return env.PlatformVersion
	case "python_version":
		return env.PythonVersion
	case "python_full_version":
		return env.PythonFullVersion
	case "implementation_name":
		return env.ImplementationName
	case "implementation_version":
		return env.ImplementationVersion
	default:
		return ""
	}
}

// tryVersionCompare attempts a PEP 440 version comparison of lhsVal op
// rhsVal: it builds a specifier from op+rhsVal and checks it against
// version.Parse(lhsVal). ok is false whenever either step fails to parse
// (e.g. op is "in"/"not in", or either side is not a valid PEP 440 version,
// including ""), signaling the caller to fall back to string semantics; the
// version.Parse/NewSpecifiers failure path never panics.
func tryVersionCompare(op pep508.CompareOp, lhsVal, rhsVal string) (result bool, ok bool) {
	spec, err := version.NewSpecifiers(string(op) + rhsVal)
	if err != nil {
		return false, false
	}
	pv, err := version.Parse(lhsVal)
	if err != nil {
		return false, false
	}
	return spec.Check(pv), true
}

// evalStringOp is the single generic operator table used whenever a version
// comparison was not attempted (neither operand names a version-typed
// variable) or was attempted and failed to parse. `~=` and `===` have no
// meaningful string semantics - packaging raises UndefinedComparison here;
// since Evaluate's signature is a fixed bool with no error channel, this is
// a deliberate, documented divergence: they evaluate false rather than
// erroring. See the task report for this judgment call.
func evalStringOp(op pep508.CompareOp, lhsVal, rhsVal string) bool {
	switch op {
	case "==":
		return lhsVal == rhsVal
	case "!=":
		return lhsVal != rhsVal
	case "in":
		return strings.Contains(rhsVal, lhsVal)
	case "not in":
		return !strings.Contains(rhsVal, lhsVal)
	case "<", ">":
		return false
	case "<=", ">=":
		return lhsVal == rhsVal
	case "~=", "===":
		return false
	default:
		return false
	}
}
