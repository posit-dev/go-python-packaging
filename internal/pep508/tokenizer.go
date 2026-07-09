// SPDX-License-Identifier: Apache-2.0 OR MIT

package pep508

import (
	"fmt"
	"regexp"
	"strings"
)

// Kind identifies a lexical token kind recognized by the Tokenizer.
//
// This is pypa/packaging's DEFAULT_RULES keys (_tokenizer.py): the
// marker-grammar subset (WS, LPAREN, RPAREN, OP, BOOLOP, IN, NOT, VARIABLE,
// QUOTED_STRING, END), plus the requirement-only kinds needed by the
// requirement parser (IDENTIFIER, LBRACKET, RBRACKET, COMMA, SEMICOLON, AT,
// SPECIFIER, URL). The requirement-only kinds coexist with the marker kinds
// without any ordering conflict, since matching is parser-driven (a given
// Kind is only ever attempted where the calling grammar expects it).
type Kind int

const (
	// WS matches one or more whitespace characters.
	WS Kind = iota
	// LParen matches a literal "(".
	LParen
	// RParen matches a literal ")".
	RParen
	// OP matches a PEP 440 comparison operator. Alternatives are ordered
	// longest-first so e.g. "===" is not truncated to "==".
	OP
	// BoolOp matches the marker boolean keywords "and"/"or".
	BoolOp
	// IN matches the marker keyword "in".
	IN
	// NOT matches the marker keyword "not".
	NOT
	// Variable matches a PEP 508 environment-marker variable name,
	// including the deprecated "python_implementation" alias for
	// "platform_python_implementation". Folding the alias to its
	// canonical name is the marker parser's job, not the tokenizer's.
	Variable
	// QuotedString matches a single- or double-quoted string literal.
	// PEP 508 quoted strings have no escape syntax; a double-quoted
	// string may embed unescaped single quotes and vice versa.
	QuotedString
	// End matches only at the end of the source (zero-width).
	End

	// --- requirement-only kinds below: used by the requirement parser
	// (internal/pep508/requirement.go), never by the marker parser. ---

	// Identifier matches a PEP 508 "identifier" production: a package name
	// or extra name. Per packaging's IDENTIFIER rule, the ground truth for
	// this grammar (PEP 508's prose and formal grammar disagree).
	Identifier
	// LBracket matches a literal "[", opening an extras list.
	LBracket
	// RBracket matches a literal "]", closing an extras list.
	RBracket
	// Comma matches a literal ",", separating extras or specifier clauses.
	Comma
	// Semicolon matches a literal ";", introducing a "; marker" clause.
	Semicolon
	// AT matches a literal "@", introducing a "@ url" clause.
	AT
	// Specifier matches one version-specifier clause: a PEP 440
	// comparison operator followed by a version-shaped token. The raw
	// text is handed to version.NewSpecifiers for authoritative parsing
	// and validation; the tokenizer's own version-shape match need only
	// be permissive enough to bound the clause (up to the next
	// whitespace, comma, semicolon, or closing paren).
	Specifier
	// URL matches a greedy run of non-whitespace characters following
	// "@". It intentionally has no upper bound other than whitespace (not
	// even ";"), since a URL requirement's URL can itself contain a
	// semicolon (e.g. a direct-reference wheel URL with a query string);
	// PEP 508 relies on mandatory whitespace, not "no semicolons in a
	// URL", to separate a URL from a following marker clause.
	URL
)

// String renders a Kind by its packaging DEFAULT_RULES name, primarily for
// panic/error messages.
func (k Kind) String() string {
	switch k {
	case WS:
		return "WS"
	case LParen:
		return "LPAREN"
	case RParen:
		return "RPAREN"
	case OP:
		return "OP"
	case BoolOp:
		return "BOOLOP"
	case IN:
		return "IN"
	case NOT:
		return "NOT"
	case Variable:
		return "VARIABLE"
	case QuotedString:
		return "QUOTED_STRING"
	case End:
		return "END"
	case Identifier:
		return "IDENTIFIER"
	case LBracket:
		return "LBRACKET"
	case RBracket:
		return "RBRACKET"
	case Comma:
		return "COMMA"
	case Semicolon:
		return "SEMICOLON"
	case AT:
		return "AT"
	case Specifier:
		return "SPECIFIER"
	case URL:
		return "URL"
	default:
		return fmt.Sprintf("Kind(%d)", int(k))
	}
}

// Token is a single lexed token: its Kind, the exact matched source text,
// and the byte offset in the source where it begins.
//
// For QuotedString, Text retains the surrounding quote characters (matching
// packaging's Token.text, which is later sliced [1:-1] by the parser); use
// Unquoted for the string literal's value.
type Token struct {
	Kind Kind
	Text string
	Pos  int
}

// Unquoted returns a QuotedString token's value with its surrounding quote
// characters stripped. No escape processing is performed, since PEP 508
// quoted strings support none; this mirrors packaging's _parser.py, which
// takes token.text[1:-1] directly.
func (t Token) Unquoted() string {
	if len(t.Text) < 2 {
		return t.Text
	}
	return t.Text[1 : len(t.Text)-1]
}

// tokenRules mirrors pypa/packaging's _tokenizer.py DEFAULT_RULES, restricted
// to the kinds needed by the marker grammar. Each pattern is compiled once,
// at package init, and wrapped in \A so a match is only ever accepted when
// it starts at position 0 of the (position-sliced) source - see check.
//
// Rules are matched with \A against source[pos:], so a leading \b in a rule
// (BoolOp, IN, NOT, Variable, Identifier) sees the start of that slice as a
// word boundary, regardless of what precedes source[pos] in the full
// source. This is safe because every reachable pos follows either
// start-of-input or a token ending in a non-word character (WS, LParen,
// OP, QuotedString, LBracket, RBracket, Comma, Semicolon, AT, ...), so the
// synthetic boundary at the slice start always agrees with the true
// boundary in the unsliced source. Identifier (added for the requirement
// parser) was checked against this invariant when it was introduced: every
// grammar position that checks Identifier is reached only after such a
// delimiter, start-of-input, or whitespace - never directly after another
// word-character token with no separator - see
// TestIdentifier_NeverCheckedAfterWordCharWithNoBoundary in
// requirement_test.go.
var tokenRules = map[Kind]*regexp.Regexp{
	WS:     regexp.MustCompile(`\A\s+`),
	LParen: regexp.MustCompile(`\A\(`),
	RParen: regexp.MustCompile(`\A\)`),
	OP:     regexp.MustCompile(`\A(?:===|==|~=|!=|<=|>=|<|>)`),
	BoolOp: regexp.MustCompile(`\A\b(?:and|or)\b`),
	IN:     regexp.MustCompile(`\A\bin\b`),
	NOT:    regexp.MustCompile(`\A\bnot\b`),
	// Longest-alternative-first: python_full_version before python_version,
	// and the platform_python_implementation forms before the bare
	// python_implementation alias, so a longer name is never truncated to
	// a shorter one that happens to prefix it.
	Variable: regexp.MustCompile(`\A\b(?:` +
		`python_full_version|python_version|` +
		`os[._]name|sys[._]platform|` +
		`platform_(?:release|system)|` +
		`platform[._](?:version|machine|python_implementation)|` +
		`python_implementation|` +
		`implementation_(?:name|version)|` +
		`extra` +
		`)\b`),
	QuotedString: regexp.MustCompile(`\A(?:'[^']*'|"[^"]*")`),
	End:          regexp.MustCompile(`\A$`),

	// --- requirement-only rules below (see the requirement-only Kind
	// constants for what each matches and why). ---

	Identifier: regexp.MustCompile(`\A\b[a-zA-Z0-9][a-zA-Z0-9._-]*\b`),
	LBracket:   regexp.MustCompile(`\A\[`),
	RBracket:   regexp.MustCompile(`\A\]`),
	Comma:      regexp.MustCompile(`\A,`),
	Semicolon:  regexp.MustCompile(`\A;`),
	AT:         regexp.MustCompile(`\A@`),
	// Specifier: an OP (reusing the same longest-first alternation as the
	// OP rule) followed by optional whitespace and a version-shaped run
	// of characters, bounded by whitespace/comma/semicolon/closing-paren
	// rather than a precise PEP 440 grammar - version.NewSpecifiers is
	// the authoritative parser for the accumulated raw text.
	Specifier: regexp.MustCompile(`\A(?:===|==|~=|!=|<=|>=|<|>)\s*[^\s,;)]+`),
	// URL: a greedy run of non-whitespace characters - the exact
	// complement of the WS rule above (\s+) - deliberately not bounded by
	// ";" (see the URL Kind doc comment). Upstream packaging's rule
	// excludes only space ([^ ]+); this port is stricter, treating all
	// whitespace (including tab and newlines) as a terminator. That's
	// safe because a valid URL never contains whitespace, and it avoids
	// a stray newline (e.g. in a defensively-multiline Requires-Dist
	// value) being absorbed into the URL along with a following "; marker"
	// clause.
	URL: regexp.MustCompile(`\A\S+`),
}

// Tokenizer performs context-sensitive lexing over a PEP 508 source string.
// It holds a single-token lookahead: callers ask whether the current
// position matches a given Kind (check), then read it, mirroring
// packaging's Tokenizer class. There is no whole-input pre-scan, since the
// set of valid token kinds at a given position is determined by the caller
// (the marker or requirement parser), not by a single global grammar.
type Tokenizer struct {
	source string
	pos    int
	next   *Token
}

// NewTokenizer returns a Tokenizer positioned at the start of source.
func NewTokenizer(source string) *Tokenizer {
	return &Tokenizer{source: source}
}

// check reports whether the rule for kind matches at the current position.
// On a successful non-peeking check, the matched token is cached and must
// be consumed via read before another check is made (panics otherwise,
// mirroring packaging's assert).
func (t *Tokenizer) check(kind Kind) bool {
	return t.checkOrPeek(kind, false)
}

// peek is like check, but never caches the match: the position is
// unaffected and the same (or a different) kind may be checked again
// immediately after.
func (t *Tokenizer) peek(kind Kind) bool {
	return t.checkOrPeek(kind, true)
}

func (t *Tokenizer) checkOrPeek(kind Kind, peek bool) bool {
	if t.next != nil {
		panic(fmt.Sprintf("pep508: cannot check %s, already have pending token %s %q", kind, t.next.Kind, t.next.Text))
	}
	re, ok := tokenRules[kind]
	if !ok {
		panic(fmt.Sprintf("pep508: unknown token kind %s", kind))
	}

	// re is anchored with \A, so a non-nil match always starts at index 0
	// of source[pos:] - i.e., exactly at the current position.
	loc := re.FindStringIndex(t.source[t.pos:])
	if loc == nil {
		return false
	}
	if !peek {
		t.next = &Token{Kind: kind, Text: t.source[t.pos : t.pos+loc[1]], Pos: t.pos}
	}
	return true
}

// expect requires that kind matches at the current position, reading and
// returning the token if so. Otherwise, it returns a *SyntaxError whose
// message is "Expected " + expected, positioned at the current (unmoved)
// position.
func (t *Tokenizer) expect(kind Kind, expected string) (Token, error) {
	if !t.check(kind) {
		return Token{}, t.NewSyntaxError("Expected " + expected)
	}
	return t.read(), nil
}

// read consumes and returns the token cached by the most recent successful
// (non-peeking) check, advancing the position past it. It panics if no
// token is pending, mirroring packaging's assert next_token is not None.
func (t *Tokenizer) read() Token {
	if t.next == nil {
		panic("pep508: read called with no pending token; check must succeed first")
	}
	tok := *t.next
	t.pos += len(tok.Text)
	t.next = nil
	return tok
}

// consume reads past kind if it matches at the current position, discarding
// the token; it is a no-op (returning false) otherwise. It is used to skip
// optional tokens such as WS.
func (t *Tokenizer) consume(kind Kind) bool {
	if t.check(kind) {
		t.read()
		return true
	}
	return false
}

// SyntaxError reports that the source could not be tokenized/parsed
// correctly at a given span. It mirrors packaging's ParserSyntaxError,
// including its message/source/marker-line rendering (a run of spaces,
// then tildes spanning [Start,End), then a caret), so error output points
// at the offending span the same way packaging's does.
type SyntaxError struct {
	Msg    string
	Source string
	Start  int
	End    int
}

func (e *SyntaxError) Error() string {
	width := e.End - e.Start
	if width < 0 {
		width = 0
	}
	marker := strings.Repeat(" ", max(e.Start, 0)) + strings.Repeat("~", width) + "^"
	return e.Msg + "\n" + e.Source + "\n" + marker
}

// NewSyntaxError builds a *SyntaxError for message at the tokenizer's
// current position (a zero-width span), mirroring a bare call to
// packaging's Tokenizer.raise_syntax_error(message).
//
// Exported (rather than kept alongside the unexported check/expect/read/
// consume methods) so the marker and requirement parsers - which live in
// this package but are handed a *Tokenizer constructed by their public
// wrapper packages - have a stable, documented way to raise it.
func (t *Tokenizer) NewSyntaxError(message string) *SyntaxError {
	return t.NewSyntaxErrorAt(message, t.pos, t.pos)
}

// NewSyntaxErrorAt is like NewSyntaxError but with an explicit [start, end)
// span, mirroring packaging's Tokenizer.raise_syntax_error(message,
// span_start=..., span_end=...) - used e.g. to point at the opening "(" of
// an unclosed parenthesized expression rather than at the current position.
func (t *Tokenizer) NewSyntaxErrorAt(message string, start, end int) *SyntaxError {
	return &SyntaxError{Msg: message, Source: t.source, Start: start, End: end}
}
