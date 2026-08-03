// SPDX-License-Identifier: Apache-2.0 OR MIT

package pep508

import (
	"fmt"
	"strings"

	"github.com/posit-dev/go-python-packaging/extras"
)

// LogicalOp identifies the boolean keyword ("and" or "or") joining a
// BoolExpr's operands. (Named LogicalOp, not BoolOp, to avoid colliding with
// the tokenizer's BoolOp Kind constant.)
type LogicalOp string

const (
	And LogicalOp = "and"
	Or  LogicalOp = "or"
)

// Expr is a node in a parsed PEP 508 marker expression tree: either a
// *BoolExpr (an "and"/"or" of two or more sub-expressions) or a *CompareExpr
// (a single comparison). A parenthesized sub-expression in the source is
// captured structurally - "(...)" parses to the Expr it contains, with no
// distinct node type of its own - so String does not necessarily preserve
// redundant source parentheses, only the grouping they express.
//
// Exported (even though this package is "internal/pep508") so the evaluator
// in the public marker/ package (a later task) can type-switch over the
// tree; parsing has to happen here regardless, since building the tree
// requires the tokenizer's unexported check/expect/read/consume methods.
type Expr interface {
	fmt.Stringer
	markerExpr()
}

// BoolExpr is a boolean "and"/"or" combination of two or more operands.
// "and" binds tighter than "or": parsing "a or b and c" yields
// BoolExpr{Or, [a, BoolExpr{And, [b, c]}]}, never the reverse grouping.
type BoolExpr struct {
	Op       LogicalOp
	Operands []Expr
}

func (*BoolExpr) markerExpr() {}

// String renders e, parenthesizing an operand only where necessary to
// preserve precedence on a re-parse (see formatBoolOperand).
func (e *BoolExpr) String() string {
	parts := make([]string, len(e.Operands))
	for i, operand := range e.Operands {
		parts[i] = formatBoolOperand(e.Op, operand)
	}
	return strings.Join(parts, " "+string(e.Op)+" ")
}

// formatBoolOperand renders operand as a component of a BoolExpr whose
// operator is parentOp. Parentheses are added only when omitting them would
// change the parsed meaning: an "or" nested inside an "and" must be
// parenthesized, since "and" binds tighter and a flat print would let it
// silently re-associate. Every other nesting - "and" inside "or" (the normal
// result of precedence grouping), or same-operator nesting produced by
// redundant source parentheses - prints flat, since both operators are
// associative and the grouping is unaffected either way.
func formatBoolOperand(parentOp LogicalOp, operand Expr) string {
	if nested, ok := operand.(*BoolExpr); ok && parentOp == And && nested.Op == Or {
		return "(" + nested.String() + ")"
	}
	return operand.String()
}

// CompareOp is a marker comparison operator: one of the PEP 440 comparison
// operators (<=, <, !=, ==, >=, >, ~=, ===) or the marker-only "in"/"not in".
type CompareOp string

// CompareExpr is a single marker comparison "lhs op rhs", e.g.
// `python_version >= "3.8"` or `extra == "foo"`.
type CompareExpr struct {
	Lhs Operand
	Op  CompareOp
	Rhs Operand
}

func (*CompareExpr) markerExpr() {}

func (e *CompareExpr) String() string {
	return e.Lhs.String() + " " + string(e.Op) + " " + e.Rhs.String()
}

// Operand is one side of a CompareExpr: either an EnvVar (an environment
// marker variable reference) or a Literal (a quoted string). Distinct
// concrete types per side let the evaluator (package marker) type-switch to
// tell "variable" from "literal" without inspecting a discriminant field.
type Operand interface {
	fmt.Stringer
	markerOperand()
}

// EnvVar is an environment-marker variable reference, e.g. python_version.
// (Named EnvVar, not Variable, to avoid colliding with the tokenizer's
// Variable Kind constant.) Deprecated/legacy spellings - the dotted aliases
// and the bare "python_implementation" alias - are folded to their
// canonical snake_case name by the parser (see foldEnvVarAlias), so Name is
// always canonical.
type EnvVar struct {
	Name string
}

func (EnvVar) markerOperand() {}

func (v EnvVar) String() string { return v.Name }

// Literal is a quoted string operand. Where one side of a comparison is the
// "extra" EnvVar, the parser normalizes the other side's Literal.Value via
// extras.Normalize once, here (see normalizeExtraLiteral), so Evaluate (the
// hot path, a later task) never has to re-normalize.
type Literal struct {
	Value string
}

func (Literal) markerOperand() {}

func (l Literal) String() string { return quoteLiteral(l.Value) }

// quoteLiteral renders a literal's value back into PEP 508 quoted-string
// syntax. We do not EMIT escape sequences, so a value containing a double
// quote is rendered single-quoted instead; a value containing both quote
// characters cannot be round-tripped (packaging has the same limitation).
//
// Note this is about what we emit, not what we accept: upstream runs the token
// through ast.literal_eval, so Python string escapes ARE valid on input (see
// Token.Unquoted and validateQuotedStringContents). Do not use this comment as
// grounds for removing the escape validation.
func quoteLiteral(v string) string {
	if strings.Contains(v, `"`) {
		return "'" + v + "'"
	}
	return `"` + v + `"`
}

// envVarAliases maps every deprecated/legacy Variable-token spelling the
// tokenizer recognizes to its canonical PEP 508 name, mirroring packaging's
// process_env_var (_parser.py). Canonical spellings are absent from the map
// and fold to themselves.
var envVarAliases = map[string]string{
	"python_implementation":          "platform_python_implementation",
	"os.name":                        "os_name",
	"sys.platform":                   "sys_platform",
	"platform.version":               "platform_version",
	"platform.machine":               "platform_machine",
	"platform.python_implementation": "platform_python_implementation",
}

func foldEnvVarAlias(name string) string {
	if canon, ok := envVarAliases[name]; ok {
		return canon
	}
	return name
}

// ParseMarker parses a PEP 508 marker expression from t's current position
// and returns as soon as the marker grammar is exhausted. It does not
// consume or verify End: the caller decides what, if anything, must follow.
//
// This mirrors packaging's _parse_marker (_parser.py), which is reused -
// without an END check - both by parse_marker's END-checking wrapper and by
// _parse_requirement, which keeps parsing requirement grammar after the
// marker clause. Here, ParseFullMarker plays the role of parse_marker's
// wrapper (used by package marker's Parse); the requirement parser (a later
// package, in this same internal/pep508 package) calls ParseMarker
// directly, since more requirement grammar follows the marker clause there.
func ParseMarker(t *Tokenizer) (Expr, error) {
	atoms, ops, err := parseMarkerSequence(t)
	if err != nil {
		return nil, err
	}
	return groupByPrecedence(atoms, ops), nil
}

// ParseFullMarker parses a complete marker expression from t and verifies
// nothing but end-of-input follows, mirroring packaging's parse_marker
// (_parser.py), which wraps _parse_marker with a
// tokenizer.expect("END", ...) check.
func ParseFullMarker(t *Tokenizer) (Expr, error) {
	expr, err := ParseMarker(t)
	if err != nil {
		return nil, err
	}
	if _, err := t.expect(End, "end of marker expression"); err != nil {
		return nil, err
	}
	return expr, nil
}

// parseMarkerSequence parses packaging's _parse_marker: one marker atom,
// then zero or more (LogicalOp, atom) pairs. It returns the flat list of
// atoms and the operator between each consecutive pair, deferring
// and/or-precedence grouping to groupByPrecedence.
func parseMarkerSequence(t *Tokenizer) ([]Expr, []LogicalOp, error) {
	first, err := parseMarkerAtom(t)
	if err != nil {
		return nil, nil, err
	}
	atoms := []Expr{first}
	var ops []LogicalOp
	for t.check(BoolOp) {
		op := LogicalOp(t.read().Text)
		atom, err := parseMarkerAtom(t)
		if err != nil {
			return nil, nil, err
		}
		ops = append(ops, op)
		atoms = append(atoms, atom)
	}
	return atoms, ops, nil
}

// groupByPrecedence assembles the flat (atom, op, atom, ...) sequence
// produced by parseMarkerSequence into a tree where "and" binds tighter than
// "or": consecutive atoms joined by "and" are grouped into a single BoolExpr,
// and those groups are then combined - if there is more than one - into a
// top-level "or" BoolExpr.
//
// This mirrors the grouping packaging's _evaluate_markers performs over its
// flat MarkerList at evaluation time (grouping "and"-joined runs and
// splitting into a new group at each "or"), except performed once here, at
// parse time, so the returned tree already reflects precedence and a later
// evaluator can walk it directly with no grouping logic of its own.
func groupByPrecedence(atoms []Expr, ops []LogicalOp) Expr {
	var orGroups [][]Expr
	current := []Expr{atoms[0]}
	for i, op := range ops {
		if op == Or {
			orGroups = append(orGroups, current)
			current = []Expr{atoms[i+1]}
		} else {
			current = append(current, atoms[i+1])
		}
	}
	orGroups = append(orGroups, current)

	orOperands := make([]Expr, len(orGroups))
	for i, group := range orGroups {
		if len(group) == 1 {
			orOperands[i] = group[0]
		} else {
			orOperands[i] = &BoolExpr{Op: And, Operands: group}
		}
	}
	if len(orOperands) == 1 {
		return orOperands[0]
	}
	return &BoolExpr{Op: Or, Operands: orOperands}
}

// parseMarkerAtom parses packaging's _parse_marker_atom: an optional
// parenthesized sub-expression, or a single marker item (comparison), with
// optional surrounding whitespace.
func parseMarkerAtom(t *Tokenizer) (Expr, error) {
	t.consume(WS)
	var atom Expr
	if t.check(LParen) {
		t.read()
		t.consume(WS)
		inner, err := ParseMarker(t)
		if err != nil {
			return nil, err
		}
		t.consume(WS)
		if _, err := t.expect(RParen, "')'"); err != nil {
			return nil, err
		}
		atom = inner
	} else {
		item, err := parseMarkerItem(t)
		if err != nil {
			return nil, err
		}
		atom = item
	}
	t.consume(WS)
	return atom, nil
}

// parseMarkerItem parses packaging's _parse_marker_item: marker_var OP
// marker_var. It also applies the once-at-parse-time "extra" literal
// normalization (see normalizeExtraLiteral), since both operands are
// available here.
func parseMarkerItem(t *Tokenizer) (*CompareExpr, error) {
	t.consume(WS)
	lhs, err := parseMarkerVar(t)
	if err != nil {
		return nil, err
	}
	t.consume(WS)
	op, err := parseMarkerOp(t)
	if err != nil {
		return nil, err
	}
	t.consume(WS)
	rhs, err := parseMarkerVar(t)
	if err != nil {
		return nil, err
	}
	t.consume(WS)
	lhs, rhs = normalizeExtraLiteral(lhs, rhs)
	return &CompareExpr{Lhs: lhs, Op: op, Rhs: rhs}, nil
}

// parseMarkerVar parses packaging's _parse_marker_var: either an EnvVar
// (folding deprecated aliases to their canonical name) or a Literal (a
// quoted string's unquoted value).
func parseMarkerVar(t *Tokenizer) (Operand, error) {
	if t.check(Variable) {
		name := t.read().Text
		return EnvVar{Name: foldEnvVarAlias(name)}, nil
	}
	if t.check(QuotedString) {
		tok := t.read()
		val := tok.Unquoted()
		// Upstream calls ast.literal_eval on the token, which rejects
		// malformed escape sequences. We validate only the two specific
		// cases upstream asserts in tests: a trailing unpaired backslash
		// (the closing quote is "escaped"), and a truncated \x escape.
		if err := validateQuotedStringContents(val); err != nil {
			return nil, t.NewSyntaxErrorAt(err.Error(), tok.Pos, tok.Pos+len(tok.Text))
		}
		return Literal{Value: val}, nil
	}
	return nil, t.NewSyntaxError("Expected a marker variable or quoted string")
}

// parseMarkerOp parses packaging's _parse_marker_op: a PEP 440 comparison
// OP, or "in"/"not in".
func parseMarkerOp(t *Tokenizer) (CompareOp, error) {
	if t.check(IN) {
		t.read()
		return "in", nil
	}
	if t.check(NOT) {
		t.read()
		if _, err := t.expect(WS, "whitespace after 'not'"); err != nil {
			return "", err
		}
		if _, err := t.expect(IN, "'in' after 'not'"); err != nil {
			return "", err
		}
		return "not in", nil
	}
	if t.check(OP) {
		return CompareOp(t.read().Text), nil
	}
	return "", t.NewSyntaxError("Expected marker operator, one of <=, <, !=, ==, >=, >, ~=, ===, in, not in")
}

// validateQuotedStringContents checks for the two specific malformed escape
// cases that upstream's ast.literal_eval rejects: a trailing unpaired
// backslash, and a truncated \x escape (not followed by two hex digits).
// Returning nil means the string is valid (or contains other escape sequences
// we do not validate). A non-nil error's message is "Invalid quoted string",
// matching upstream.
// It walks the string ONCE, left to right, consuming each escape sequence as a
// unit. A single pass is required, not two independent scans: an escaped
// backslash consumes BOTH of its bytes, so `\\x` is a complete pair followed by
// a literal "x" -- not a truncated \x escape. Scanning for `\x` separately from
// the trailing-backslash check has no way to know which backslashes were
// already consumed, and falsely rejects `"C:\\xyz"`, `"a\\x"`, and `"\\x"` --
// all of which upstream accepts.
func validateQuotedStringContents(s string) error {
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' {
			continue
		}
		// A backslash at the very end is unpaired: the escape is unterminated.
		// This is upstream's `"C:\"` case.
		if i+1 >= len(s) {
			return &SyntaxError{Msg: "Invalid quoted string"}
		}
		if s[i+1] == 'x' {
			// \x requires exactly two hex digits. This is upstream's `"\x"`
			// (and `"\x4"`, `"\xZZ"`) case.
			if i+3 >= len(s) || !isHexDigit(s[i+2]) || !isHexDigit(s[i+3]) {
				return &SyntaxError{Msg: "Invalid quoted string"}
			}
			i += 3 // consume \xNN
			continue
		}
		// Any other escape (including \\, \n, \t, \', \") consumes exactly two
		// bytes. We deliberately do not validate \u/\U/octal -- see the package
		// note on why full Python escape semantics are out of scope.
		i++
	}
	return nil
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// normalizeExtraLiteral normalizes the Literal side of a comparison against
// the "extra" EnvVar via extras.Normalize, once, at parse time - handling
// both operand orders ("extra == 'X'" and "'X' == extra") - so evaluation
// (package marker's Evaluate, a later task) never has to re-normalize on its
// hot path.
func normalizeExtraLiteral(lhs, rhs Operand) (Operand, Operand) {
	if isExtraVar(lhs) {
		if lit, ok := rhs.(Literal); ok {
			rhs = Literal{Value: extras.Normalize(lit.Value)}
		}
	}
	if isExtraVar(rhs) {
		if lit, ok := lhs.(Literal); ok {
			lhs = Literal{Value: extras.Normalize(lit.Value)}
		}
	}
	return lhs, rhs
}

func isExtraVar(o Operand) bool {
	v, ok := o.(EnvVar)
	return ok && v.Name == "extra"
}
