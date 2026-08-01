// SPDX-License-Identifier: Apache-2.0 OR MIT

package requirement

import (
	"errors"
	"fmt"
	"strings"

	"github.com/posit-dev/go-python-packaging/extras"
	"github.com/posit-dev/go-python-packaging/internal/pep508"
	"github.com/posit-dev/go-python-packaging/marker"
	"github.com/posit-dev/go-python-packaging/version"
)

// ErrInvalidRequirement is the sentinel wrapped by every error Parse
// returns, regardless of which stage of parsing failed: malformed
// requirement syntax, an invalid version specifier, and an invalid marker
// expression are all distinct underlying causes, but all satisfy
// errors.Is(err, ErrInvalidRequirement). The specific cause remains
// distinguishable in the error chain:
//
//   - malformed syntax (bad name/extras/versionspec/url grammar): the chain
//     contains a *pep508.SyntaxError, but NOT a *pep508.MarkerClauseError.
//   - invalid marker expression: the chain contains a
//     *pep508.MarkerClauseError (itself wrapping a *pep508.SyntaxError from
//     the marker grammar specifically).
//   - invalid version specifier: the chain contains neither of the above -
//     the error instead comes from version.NewSpecifiers.
var ErrInvalidRequirement = errors.New("invalid PEP 508 requirement")

// Requirement is a parsed PEP 508 dependency-specifier requirement:
//
//	name [extras] (versionspec | "@" url) [";" marker]
type Requirement struct {
	// Name is the requirement's package name exactly as parsed - not
	// canonicalized. Name canonicalization is a naming concern left to
	// the caller (consistent with package wheelname's Name field).
	Name string
	// Extras are the requirement's bracketed extras, each normalized via
	// extras.Normalize. Nil if there was no "[...]" clause or it was
	// empty ("[]").
	Extras []string
	// Specifiers is the requirement's version specifier, parsed via
	// version.NewSpecifiers. It is the zero value if the requirement is
	// a URL requirement, or has no version specifier at all.
	Specifiers version.Specifiers
	// URL is the requirement's direct-reference URL ("@ url" form). Empty
	// unless that form was used.
	URL string
	// Marker is the requirement's "; marker" clause. It is the zero
	// value (always-true) if there was no such clause.
	Marker marker.Marker
}

// Parse parses s as a PEP 508 dependency-specifier requirement string:
//
//	name [extras] (versionspec | "@" url) [";" marker]
//
// Parsing drives the shared internal/pep508 tokenizer end-to-end in one
// pass (see pep508.ParseRequirement): the marker clause, if present, is
// parsed from the tokenizer's position immediately after the name/extras/
// versionspec-or-url, not by splitting the string on ";" - which would
// mishandle a URL requirement whose URL itself contains a ";".
func Parse(s string) (Requirement, error) {
	t := pep508.NewTokenizer(s)
	raw, err := pep508.ParseRequirement(t)
	if err != nil {
		var markerErr *pep508.MarkerClauseError
		if errors.As(err, &markerErr) {
			return Requirement{}, fmt.Errorf("%w: invalid marker expression: %w", ErrInvalidRequirement, err)
		}
		return Requirement{}, fmt.Errorf("%w: %w", ErrInvalidRequirement, err)
	}

	req := Requirement{
		Name: raw.Name,
		URL:  raw.URL,
	}

	if len(raw.Extras) > 0 {
		req.Extras = make([]string, len(raw.Extras))
		for i, e := range raw.Extras {
			req.Extras[i] = extras.Normalize(e)
		}
	}

	if raw.Specifier != "" {
		specs, err := version.NewSpecifiers(raw.Specifier)
		if err != nil {
			return Requirement{}, fmt.Errorf("%w: invalid version specifier %q: %w", ErrInvalidRequirement, raw.Specifier, err)
		}
		req.Specifiers = specs
	}

	req.Marker = marker.FromExpr(raw.Marker)

	return req, nil
}

// String renders r in canonical PEP 508 requirement syntax;
// Parse(r.String()) yields an equivalent Requirement.
func (r Requirement) String() string {
	var b strings.Builder
	b.WriteString(r.Name)

	if len(r.Extras) > 0 {
		b.WriteString("[")
		b.WriteString(strings.Join(r.Extras, ","))
		b.WriteString("]")
	}

	switch {
	case r.URL != "":
		b.WriteString(" @ ")
		b.WriteString(r.URL)
	case r.Specifiers.String() != "":
		b.WriteString(r.Specifiers.String())
	}

	if !r.Marker.IsEmpty() {
		// pypa/packaging renders "; " normally but " ; " when a URL is
		// present, because a bare ";" immediately after a URL would be
		// ambiguous - a URL may itself contain ";". Match that exactly.
		if r.URL != "" {
			b.WriteString(" ; ")
		} else {
			b.WriteString("; ")
		}
		b.WriteString(r.Marker.String())
	}

	return b.String()
}
