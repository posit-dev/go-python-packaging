// SPDX-License-Identifier: Apache-2.0 OR MIT

package marker

import (
	"github.com/posit-dev/go-python-packaging/internal/pep508"
)

// Undecidable names one comparison that Evaluate answered without being able to
// decide it, and why. See Marker.EvaluateUndecidable.
type Undecidable struct {
	// Expr is the comparison as rendered by the marker's own String(), so it is
	// canonical rather than the caller's original spelling.
	Expr string
	// Reason is a short, stable description suitable for a log line.
	Reason string
	// Var is the environment variable involved, when the reason concerns one.
	// Empty otherwise.
	Var string
}

// note appends u to *und, allocating on first use. A nil und disables
// collection, which is what Evaluate's bare-bool path relies on being cheap.
func note(und *[]Undecidable, u Undecidable) {
	if und == nil {
		return
	}
	*und = append(*und, u)
}

// Variables returns the distinct environment variables this marker references,
// in first-appearance order. An empty marker returns nil.
//
// This exists for callers that construct an Environment with fields they had to
// invent. A declared target names an interpreter as "3.13", so
// PythonFullVersion must be given some patch level; the value is then decidable
// but arbitrary, and `python_full_version >= "3.13.2"` answers false at an
// invented 3.13.0 and true at an invented 3.13.99. EvaluateUndecidable cannot
// help there -- nothing is empty and no operator is undefined -- because only
// the caller knows which fields it fabricated. Variables lets it ask whether a
// marker depends on one of them, without pattern-matching String().
func (m Marker) Variables() []string {
	if m.ast == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	collectVars(m.ast, seen, &out)
	return out
}

func collectVars(e pep508.Expr, seen map[string]struct{}, out *[]string) {
	switch n := e.(type) {
	case *pep508.BoolExpr:
		for _, operand := range n.Operands {
			collectVars(operand, seen, out)
		}
	case *pep508.CompareExpr:
		collectVarOperand(n.Lhs, seen, out)
		collectVarOperand(n.Rhs, seen, out)
	}
}

func collectVarOperand(operand pep508.Operand, seen map[string]struct{}, out *[]string) {
	v, ok := operand.(pep508.EnvVar)
	if !ok {
		return
	}
	if _, dup := seen[v.Name]; dup {
		return
	}
	seen[v.Name] = struct{}{}
	*out = append(*out, v.Name)
}
