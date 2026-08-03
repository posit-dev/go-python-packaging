// SPDX-License-Identifier: Apache-2.0 OR MIT

package pep508

import "strings"

// RawRequirement is the raw, structurally-parsed result of a PEP 508
// requirement string, before the public requirement/ package applies name
// canonicalization (none - the caller's concern), extras normalization
// (extras.Normalize), version-specifier delegation (version.NewSpecifiers),
// and marker wrapping (marker.FromExpr).
//
// Fields are populated exactly as parsed: Name is not canonicalized, Extras
// are not normalized, Specifier is the raw (un-parenthesized) specifier
// text handed verbatim to version.NewSpecifiers, and Marker is the
// pep508.Expr returned by ParseMarker (nil if there was no "; marker"
// clause at all - as opposed to a Marker.ast of nil, which package marker's
// zero value also represents "no clause").
type RawRequirement struct {
	Name      string
	Extras    []string
	Specifier string
	URL       string
	Marker    Expr
}

// MarkerClauseError wraps an error that occurred while parsing a
// requirement's "; marker" clause (via ParseMarker), distinguishing it from
// a *SyntaxError raised anywhere else in the requirement grammar (name,
// extras, versionspec, url). Both are *SyntaxError under the hood - this
// wrapper is what lets a caller (package requirement's Parse) tell "bad
// marker" apart from "malformed requirement syntax" via errors.As, without
// requiring a second, unrelated error type from the marker grammar itself.
type MarkerClauseError struct {
	Err error
}

func (e *MarkerClauseError) Error() string { return e.Err.Error() }
func (e *MarkerClauseError) Unwrap() error { return e.Err }

// ParseRequirement parses s (via t) as a complete PEP 508 dependency
// specifier and verifies nothing but end-of-input follows, mirroring
// packaging's _parse_requirement (_parser.py) and its wrapper parse_requirement.
//
// Grammar (PEP 508's formal grammar, as packaging implements it):
//
//	specification = wsp* ( url_req | name_req ) wsp*
//	name_req      = name wsp* extras? wsp* versionspec? wsp* quoted_marker?
//	url_req       = name wsp* extras? wsp* urlspec wsp+ quoted_marker?
//	name          = identifier
//	extras        = '[' wsp* extras_list? wsp* ']'
//	extras_list   = identifier (wsp* ',' wsp* identifier)*
//	versionspec   = ( '(' version_many ')' ) | version_many
//	version_many  = version_one (wsp* ',' version_one)*
//	urlspec       = '@' wsp* <URI_reference>
//	quoted_marker = ';' wsp* marker
//
// This drives the SHARED tokenizer end-to-end in one pass - name, then
// optional extras, then optional (versionspec | "@" url), then, if a ";"
// follows, the marker clause via ParseMarker from the tokenizer's current
// position - rather than pre-splitting the string on ";" (which would
// mishandle a URL requirement whose URL itself contains ";") or re-lexing a
// marker substring with a second tokenizer.
func ParseRequirement(t *Tokenizer) (RawRequirement, error) {
	t.consume(WS)
	nameTok, err := t.expect(Identifier, "package name")
	if err != nil {
		return RawRequirement{}, err
	}
	req := RawRequirement{Name: nameTok.Text}
	t.consume(WS)

	extras, err := parseExtras(t)
	if err != nil {
		return RawRequirement{}, err
	}
	req.Extras = extras
	t.consume(WS)

	if err := parseRequirementDetails(t, &req); err != nil {
		return RawRequirement{}, err
	}
	t.consume(WS)

	if _, err := t.expect(End, "end of requirement string"); err != nil {
		return RawRequirement{}, err
	}
	return req, nil
}

// parseExtras parses packaging's "extras" production. A missing "[" is not
// an error - extras are entirely optional - and an empty "[]" is valid
// (returning a nil/empty slice, not an error).
func parseExtras(t *Tokenizer) ([]string, error) {
	if !t.check(LBracket) {
		return nil, nil
	}
	t.read()
	t.consume(WS)

	var names []string
	if t.check(Identifier) {
		names = append(names, t.read().Text)
		t.consume(WS)
		for t.check(Comma) {
			t.read()
			t.consume(WS)
			idTok, err := t.expect(Identifier, "extra name after ','")
			if err != nil {
				return nil, err
			}
			names = append(names, idTok.Text)
			t.consume(WS)
		}
	}

	if _, err := t.expect(RBracket, "']'"); err != nil {
		return nil, err
	}
	return names, nil
}

// parseRequirementDetails parses packaging's "requirement_details":
// optionally either a version specifier or an "@ url" clause, followed by
// an optional "; marker" clause, and stores the results into req.
//
// The "@ url" branch enforces PEP 508's url_req asymmetry: mandatory
// whitespace (or end-of-input) must follow the URL before anything else can
// be parsed - including a following marker clause. Without that whitespace,
// there is no way to tell "the URL ends here" from "the URL continues", so
// (correctly, per PEP 508) everything up to the next whitespace or
// end-of-input is simply part of the URL, semicolons included.
func parseRequirementDetails(t *Tokenizer, req *RawRequirement) error {
	if t.check(AT) {
		t.read()
		t.consume(WS)
		urlTok, err := t.expect(URL, "URL after '@'")
		if err != nil {
			return err
		}
		req.URL = urlTok.Text

		if t.peek(End) {
			return nil
		}
		// Consume horizontal whitespace and any immediately following line
		// breaks. PEP 508 requires whitespace after a URL; a newline is valid
		// (common in "defensively multiline" Requires-Dist metadata) and must
		// be consumed here so a following "; marker" clause is recognized.
		if !t.consume(WS) {
			// No horizontal whitespace found; check for a line break.
			if t.pos < len(t.source) && (t.source[t.pos] == '\n' || t.source[t.pos] == '\r') {
				// Consume the line break(s).
				for t.pos < len(t.source) && (t.source[t.pos] == '\n' || t.source[t.pos] == '\r') {
					t.pos++
				}
			} else {
				return t.NewSyntaxError("Expected whitespace after URL")
			}
		} else {
			// Consumed horizontal whitespace; also consume any following line breaks.
			for t.pos < len(t.source) && (t.source[t.pos] == '\n' || t.source[t.pos] == '\r') {
				t.pos++
			}
		}
		if t.peek(End) {
			return nil
		}
		return parseMarkerClause(t, req)
	}

	spec, err := parseVersionSpec(t)
	if err != nil {
		return err
	}
	req.Specifier = spec
	t.consume(WS)
	return parseMarkerClause(t, req)
}

// parseVersionSpec parses packaging's "versionspec": an optionally
// parenthesized, comma-separated run of version-specifier clauses. It
// returns the raw specifier text (parens stripped, clauses rejoined with
// ","), or "" if no version specifier is present at all - which is legal,
// since versionspec is entirely optional in name_req. An empty parenthesized
// group ("name()") is also legal and likewise yields "", matching
// pypa/packaging's test_empty_specifier.
func parseVersionSpec(t *Tokenizer) (string, error) {
	t.consume(WS)
	parenthesized := t.check(LParen)
	if parenthesized {
		t.read()
		t.consume(WS)
	}

	var clauses []string
	if t.check(Specifier) {
		clauses = append(clauses, normalizeSpecifierClause(t.read().Text))
		t.consume(WS)
		for t.check(Comma) {
			t.read()
			t.consume(WS)
			specTok, err := t.expect(Specifier, "version specifier after ','")
			if err != nil {
				return "", err
			}
			clauses = append(clauses, normalizeSpecifierClause(specTok.Text))
			t.consume(WS)
		}
	}

	if parenthesized {
		if _, err := t.expect(RParen, "')'"); err != nil {
			return "", err
		}
	}

	return strings.Join(clauses, ","), nil
}

// normalizeSpecifierClause strips whitespace from a single tokenized
// Specifier clause's text (the Specifier rule permissively allows "\s*"
// between the operator and the version, e.g. ">= 1.0"), producing a
// canonical, whitespace-free clause for joining into the raw specifier
// string handed to version.NewSpecifiers - which is agnostic to whether
// the input had that whitespace, but round-tripping a canonical form is
// cleaner than preserving arbitrary user spacing verbatim.
func normalizeSpecifierClause(s string) string {
	return strings.Join(strings.Fields(s), "")
}

// parseMarkerClause parses packaging's "quoted_marker" (';' wsp* marker) if
// a ';' is present at the current position; it is a no-op otherwise, since
// the marker clause is always optional. The marker itself is parsed by
// ParseMarker directly from t's current (mid-stream) position - the same
// shared tokenizer, no substring, no second lexer - and any error it
// returns is wrapped in *MarkerClauseError so the caller can distinguish a
// bad marker from a bad requirement syntax error elsewhere.
func parseMarkerClause(t *Tokenizer, req *RawRequirement) error {
	if !t.check(Semicolon) {
		return nil
	}
	t.read()
	t.consume(WS)

	expr, err := ParseMarker(t)
	if err != nil {
		return &MarkerClauseError{Err: err}
	}
	req.Marker = expr
	t.consume(WS)
	return nil
}
