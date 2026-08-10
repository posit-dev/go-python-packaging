// SPDX-License-Identifier: Apache-2.0 OR MIT

package version

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

var (
	specifierOperators = map[string]operatorFunc{
		"":    specifierEqual, // not defined in PEP 440
		"=":   specifierEqual, // not defined in PEP 440
		"==":  specifierEqual,
		"!=":  specifierNotEqual,
		">":   specifierGreaterThan,
		"<":   specifierLessThan,
		">=":  specifierGreaterThanEqual,
		"<=":  specifierLessThanEqual,
		"~=":  specifierCompatible,
		"===": specifierArbitrary,
	}

	specifierRegexp        *regexp.Regexp
	validConstraintRegexp  *regexp.Regexp
	singleConstraintRegexp *regexp.Regexp
	prefixRegexp           *regexp.Regexp
)

// orderedOperatorPatterns returns the regexp-quoted comparison operators in a
// deterministic order suitable for a regexp alternation.
//
// Go's regexp is leftmost-FIRST, not leftmost-longest, so an alternative that
// is a prefix of a later one wins the match: listing "=" before "==" makes
// "==1.0" parse with operator "=" and version "=1.0". Order longest-first, and
// keep the empty operator (which matches anything) last. Ranging over the
// specifierOperators map directly would randomize this per process.
func orderedOperatorPatterns() []string {
	// Longest-first; "" last because it matches anything.
	ordered := []string{"===", "==", "!=", "<=", ">=", "~=", "<", ">", "="}
	out := make([]string, 0, len(ordered)+1)
	for _, op := range ordered {
		if _, ok := specifierOperators[op]; !ok {
			panic("orderedOperatorPatterns: unknown operator " + op)
		}
		out = append(out, regexp.QuoteMeta(op))
	}
	if _, ok := specifierOperators[""]; ok {
		out = append(out, regexp.QuoteMeta(""))
	}
	if len(out) != len(specifierOperators) {
		panic("orderedOperatorPatterns: operator list is out of sync with specifierOperators")
	}
	return out
}

func init() {
	ops := orderedOperatorPatterns()

	// Arbitrary equality ("===") compares an opaque token, which need not be a
	// valid PEP 440 version. Give it its own branch with a non-whitespace-run
	// operand; every other operator keeps the version-shaped operand. The
	// arbitrary branch is FIRST because Go's regexp is leftmost-first and
	// "===" must not be decomposed into "==" + "=".
	const arbitraryOperand = `[^\s,;)]+`

	specifierRegexp = regexp.MustCompile(
		fmt.Sprintf(
			`(?i)(?:(?P<arbitraryop>===)\s*(?P<arbitrary>%s)|(?P<operator>(%s))\s*(?P<version>%s(\.\*)?))`,
			arbitraryOperand,
			strings.Join(ops, "|"),
			regex,
		),
	)

	// One constraint, either arbitrary equality or an operator plus a version.
	//
	// ⚠️ The operator alternation admits the EMPTY operator, so a bare version
	// ("2.0") is a valid constraint. That is deliberate and separately tracked
	// (rstudio/package-manager#18634), and it is load-bearing: PPM's
	// GET /repos/:repo/packages/:key/doc?version=<raw> passes a bare version
	// through NewRSpecifiers. Do not remove it while tightening the comma.
	constraint := fmt.Sprintf(
		`(?:===\s*%s|(%s)\s*(%s(\.\*)?))`,
		arbitraryOperand,
		strings.Join(ops, "|"),
		regex,
	)

	// A COMMA IS REQUIRED BETWEEN CONSTRAINTS.
	//
	// This previously ended each repetition with `\s*\,?`, making the separator
	// optional, so two constraints written adjacently validated and then
	// re-rendered WITH a comma the input never contained. Combined with the
	// empty operator above, that turned "==0.1dev10.3" into the constraint
	// "==0.1dev10" plus a bare-version constraint "3", rendering
	// "==0.1dev10,3" -- a fabricated constraint boundary, the same defect class
	// as #22 by a different route.
	//
	// A trailing comma is still tolerated, deliberately: that is a separate
	// leniency from the missing separator, nothing observed depends on
	// tightening it, and narrowing one thing at a time keeps the blast radius
	// measurable.
	//
	// Go's regexp permits a capture-group name to repeat, so embedding the
	// version pattern twice needs no name stripping. Verified rather than
	// assumed -- it is the opposite of Python's rule.
	validConstraintRegexp = regexp.MustCompile(
		fmt.Sprintf(`^\s*(?:%s\s*(?:,\s*%s\s*)*,?)?\s*$`, constraint, constraint),
	)

	// EXACTLY ONE constraint, with no comma anywhere: the grammar for the
	// SINGULAR NewSpecifier.
	//
	// ⚠️ This exists because NewSpecifier must NOT borrow the set grammar above.
	// The set grammar tolerates a trailing comma on purpose, and that leniency
	// leaked straight into the singular constructor: `NewSpecifier(">=1,")` was
	// accepted and silently yielded the single specifier `>=1`, contradicting
	// NewSpecifier's own documented contract. Upstream draws the line sharply --
	// measured against packaging 26.2, `Specifier(">=1,")`, `Specifier(",>=1")`
	// and `Specifier(",")` all raise InvalidSpecifier while the corresponding
	// SpecifierSet calls all succeed.
	//
	// One rule, one copy. A singular constructor validating with the plural
	// grammar is a coupling that breaks silently whenever the plural grammar
	// moves, which is exactly how this was introduced.
	singleConstraintRegexp = regexp.MustCompile(
		fmt.Sprintf(`^\s*%s\s*$`, constraint),
	)

	prefixRegexp = regexp.MustCompile(`^([0-9]+)((?:a|b|c|rc)[0-9]+)$`)
}

type operatorFunc func(v Version, c string) bool

type Specifiers struct {
	specifiers [][]Specifier
	conf       conf
}

// Specifier is a single PEP 440 version specifier: one comparison operator
// and one operand, such as `>=1.2.3` or `===lolwat`. It is the unit
// pypa/packaging calls `Specifier`; the comma-separated set is Specifiers.
//
// The zero value is not a usable specifier. Build one with NewSpecifier, or
// read them off a Specifiers with Specifiers.List.
type Specifier struct {
	// op is the comparison operator exactly as PEP 440 spells it ("==",
	// "!=", "<", "<=", ">", ">=", "~=", "==="), or "" for the operator-less
	// bare-version form this package deliberately admits (see the
	// validConstraintRegexp comment in init).
	op string
	// operand is the right-hand side with surrounding whitespace removed and
	// the sanitizer applied. It is NOT normalized as a version: like upstream's
	// Specifier.version it is the operand as written, so `>=  v1.0  ` keeps its
	// "v" prefix.
	//
	// ⚠️ Upstream also preserves an upper-case spelling (`==1.0A1` keeps
	// `1.0A1`), but this package cannot reach that case: validConstraintRegexp
	// lacks the (?i) flag specifierRegexp has, so `==1.0A1` is rejected at
	// parse time. Tracked in rstudio/package-manager#19391.
	operand string
	// original is the specifier as matched in the input, whitespace and all.
	original string
	// fn is the matching function for op.
	fn operatorFunc
	// pre is the pre-release policy this specifier was built with, before
	// autodetection. PreReleasesAuto means "not set"; PreReleases resolves it.
	pre PreReleases
}

// NewSpecifiersWithSanitizer parses a given specifier and returns a new instance of Specifiers
// it sanitizes the version string before parsing it with the given function.
func NewSpecifiersWithSanitizer(v string, sanitizer func(string) string, opts ...SpecifierOption) (Specifiers, error) {
	return newSpecifiers(v, sanitizer, opts...)
}

// NewRSpecifiers parses a given specifier and returns a new instance of Specifiers intended for
// working with R package versions.
func NewRSpecifiers(v string, sanitizer func(string) string, opts ...SpecifierOption) (Specifiers, error) {
	return newRSpecifiers(v, sanitizer, opts...)
}

// NewSpecifiers parses a given specifier and returns a new instance of Specifiers
func NewSpecifiers(v string, opts ...SpecifierOption) (Specifiers, error) {
	return newSpecifiers(v, func(s string) string { return s }, opts...)
}

// NewSpecifiers parses a given specifier and returns a new instance of Specifiers
func newRSpecifiers(v string, sanitizer func(string) string, opts ...SpecifierOption) (Specifiers, error) {
	c := new(conf)

	// Apply options
	for _, o := range opts {
		o.apply(c)
	}

	segments, universal, err := splitOrSegments(v)
	if err != nil {
		return Specifiers{}, err
	}
	if universal {
		return universalSpecifiers(*c), nil
	}

	var sss [][]Specifier
	for _, vv := range segments {
		if strings.TrimSpace(vv) == "*" {
			vv = ">=0.0.0"
		}
		vv = strings.ReplaceAll(vv, "-", ".")

		specs, err := parseGroup(vv, sanitizer)
		if err != nil {
			return Specifiers{}, err
		}
		sss = append(sss, specs)
	}

	return Specifiers{
		specifiers: sss,
		conf:       *c,
	}, nil

}

// NewSpecifiers parses a given specifier and returns a new instance of Specifiers
func newSpecifiers(v string, sanitizer func(string) string, opts ...SpecifierOption) (Specifiers, error) {
	c := new(conf)

	// Apply options
	for _, o := range opts {
		o.apply(c)
	}

	segments, universal, err := splitOrSegments(v)
	if err != nil {
		return Specifiers{}, err
	}
	if universal {
		return universalSpecifiers(*c), nil
	}

	var sss [][]Specifier
	for _, vv := range segments {
		if strings.TrimSpace(vv) == "*" {
			vv = ">=0.0.0"
		}

		specs, err := parseGroup(vv, sanitizer)
		if err != nil {
			return Specifiers{}, err
		}
		sss = append(sss, specs)
	}

	return Specifiers{
		specifiers: sss,
		conf:       *c,
	}, nil

}

// splitOrSegments splits the input on the "||" OR operator.
//
// universal is true when the ENTIRE input is empty or whitespace, which is the
// one and only way to get a set that constrains nothing. In that case segments
// is nil and the caller must not parse anything.
//
// ⚠️ An empty "||" segment is an ERROR, not the universal set.
//
// This is the whole point of the function, and it is worth being explicit about
// why, because the obvious-looking alternative is a silent, invisible failure.
// "||" is an OR, and Check admits a version if ANY group matches; a group with
// no specifiers in it matches everything. So if ">=1||" were allowed to parse
// as `>=1 OR (nothing)`, the trailing typo would not narrow the constraint or
// raise an error -- it would DISABLE it, and ">=1||" would admit 0.1. The R
// entry point shares this path and is what PPM uses for R version constraints,
// so the visible symptom would be "the wrong versions were allowed", with
// nothing logged and nothing failing.
//
// A leading, trailing or doubled "||" is therefore rejected. Note that this is
// specifically about the "||" boundary: comma handling inside a group is a
// separate question, decided in parseGroup.
func splitOrSegments(v string) (segments []string, universal bool, err error) {
	if strings.TrimSpace(v) == "" {
		return nil, true, nil
	}

	segments = strings.Split(v, "||")
	for _, segment := range segments {
		if strings.TrimSpace(segment) == "" {
			return nil, false, fmt.Errorf(
				"improper constraint: empty || segment in %q (a leading, trailing or doubled || is not allowed)", v)
		}
	}
	return segments, false, nil
}

// universalSpecifiers returns the set that constrains nothing: exactly one
// AND-group, containing no specifiers.
//
// It is spelled as one EMPTY group rather than as zero groups so that the two
// readings of "no constraints" cannot drift apart -- Check treats zero groups
// and an empty group the same way, and both mean "admits every version" (see
// Check, and rstudio/package-manager#19366).
func universalSpecifiers(c conf) Specifiers {
	return Specifiers{
		specifiers: [][]Specifier{nil},
		conf:       c,
	}
}

// NewSpecifier parses a single PEP 440 version specifier, such as `>=1.2.3`.
//
// It is the singular counterpart to NewSpecifiers and mirrors upstream's
// `Specifier(spec, prereleases=...)` constructor.
//
// A COMMA is rejected outright, in any position — `">=1,<2"`, `">=1,"`,
// `",>=1"` and `","` are all errors. A comma is set punctuation and has no
// meaning inside one specifier, and upstream agrees: measured against
// packaging 26.2, `Specifier` raises `InvalidSpecifier` for every one of those
// while the corresponding `SpecifierSet` calls all succeed. Pass a set to
// NewSpecifiers instead.
//
// Whitespace is permitted and stripped (`">= 1.0"`, `">=  v1.0  "`), and a bare
// version with no operator is accepted as this package's deliberate extension
// for the R path (see the validConstraintRegexp comment in init).
func NewSpecifier(s string, opts ...SpecifierOption) (Specifier, error) {
	c := new(conf)
	for _, o := range opts {
		o.apply(c)
	}

	// Validate against the SINGLE-constraint grammar, not the set grammar, so
	// that NewSpecifier(">=1,<2") is an error rather than a silent ">=1" with the
	// rest dropped on the floor -- and so that a comma is rejected outright
	// rather than inherited from the set grammar's deliberate trailing-comma
	// leniency. See singleConstraintRegexp.
	//
	// specifierRegexp is unanchored (Specifiers scans with it), so a bare
	// FindStringSubmatch would happily match a prefix; the anchored grammar is
	// what makes the whole input have to be one specifier.
	if !singleConstraintRegexp.MatchString(s) {
		return Specifier{}, fmt.Errorf("improper specifier: %s", s)
	}
	// Belt and braces: the anchored grammar already guarantees one constraint, so
	// this only fires if the grammar and the scanner ever disagree about where a
	// constraint ends. Cheap, and the failure it would catch is a silent wrong
	// parse rather than an error.
	found := specifierRegexp.FindAllString(s, -1)
	if len(found) != 1 {
		return Specifier{}, fmt.Errorf("improper specifier: %s", s)
	}

	return newSpecifier(found[0], func(v string) string { return v }, *c)
}

// parseGroup parses one AND-group: a comma-separated run of specifiers.
//
// ⚠️ parseGroup NEVER returns an empty group, and must not be changed to.
//
// An empty AND-group matches every version (a conjunction over no constraints
// is vacuously true), so inside an OR it does not widen the set a little -- it
// makes the whole set universal. Returning one from here for a blank input is
// what made ">=1||" admit 0.1: the trailing "||" produced a blank segment, the
// blank segment produced an empty group, and the empty group swallowed the
// ">=1". A malformed constraint went from "error" to "silently allows
// everything".
//
// The universal set has exactly one producer, and it is not this function: see
// universalSpecifiers, reached only when the ENTIRE input is blank. Callers
// guarantee that by rejecting blank "||" segments in splitOrSegments, so a
// blank input reaching here is a caller bug and is reported as an error.
//
// A blank group is not the same question as a blank comma-separated ITEM.
// Upstream drops empty comma-split items (`[s.strip() for s in
// specifiers.split(",") if s.strip()]`), which is why `packaging` accepts
// ",>=1" and ">=1,,<2"; this package still rejects both, via
// validConstraintRegexp, and that remaining divergence is tracked in
// rstudio/package-manager#19391. Either way it is a question about members
// within a group, never about whether the group itself may be empty.
// ⚠️ parseGroup deliberately takes no conf. The pre-release policy passed to
// NewSpecifiers belongs to the SET, not to its members, and stamping it onto
// each member conflates the two: upstream's SpecifierSet keeps its own
// _prereleases and leaves every member's alone, so
// `list(SpecifierSet(">=1.0.dev1", prereleases=False))[0].prereleases` is True
// (the member's own autodetect) while the set's is False. Members are built
// with the zero policy and autodetect individually; the set's policy is applied
// at query time from ss.conf.
func parseGroup(vv string, sanitizer func(string) string) ([]Specifier, error) {
	if strings.TrimSpace(vv) == "" {
		return nil, fmt.Errorf("improper constraint: empty constraint group")
	}
	if !validConstraintRegexp.MatchString(vv) {
		return nil, fmt.Errorf("improper constraint: %s", vv)
	}

	found := specifierRegexp.FindAllString(vv, -1)
	if found == nil {
		found = append(found, strings.TrimSpace(vv))
	}

	specs := make([]Specifier, 0, len(found))
	for _, single := range found {
		s, err := newSpecifier(single, sanitizer, conf{})
		if err != nil {
			return nil, err
		}
		specs = append(specs, s)
	}
	return specs, nil
}

func newSpecifier(s string, sanitizer func(s string) string, c conf) (Specifier, error) {
	m := specifierRegexp.FindStringSubmatch(s)
	if m == nil {
		return Specifier{}, fmt.Errorf("improper specifier: %s", s)
	}

	operator := m[specifierRegexp.SubexpIndex("operator")]
	version := m[specifierRegexp.SubexpIndex("version")]
	if arbitrary := m[specifierRegexp.SubexpIndex("arbitrary")]; arbitrary != "" {
		// Arbitrary equality: the operand is an opaque token, used verbatim.
		operator = m[specifierRegexp.SubexpIndex("arbitraryop")]
		version = arbitrary
	} else {
		version = sanitizer(version)
	}

	if operator != "===" {
		if err := validate(operator, version); err != nil {
			return Specifier{}, err
		}
	}

	return Specifier{
		op:       operator,
		operand:  version,
		original: s,
		fn:       specifierOperators[operator],
		pre:      c.preReleases,
	}, nil
}

func validate(operator, version string) error {
	hasWildcard := false
	if strings.HasSuffix(version, ".*") {
		hasWildcard = true
		version = strings.TrimSuffix(version, ".*")
	}
	v, err := Parse(version)
	if err != nil {
		return fmt.Errorf("version parse error (%s): %w", v, err)
	}

	switch operator {
	case "", "=", "==", "!=":
		if hasWildcard && (!v.dev.isNull() || !v.post.isNull() || v.local != "") {
			return errors.New(
				"the (non)equality operators don't allow to use a wild card and a dev, post, or local version together",
			)
		}
	case "~=":
		if hasWildcard {
			return errors.New("a wild card is not allowed")
		} else if len(v.release) < 2 {
			return errors.New("the compatible operator requires at least two digits in the release segment")
		} else if v.local != "" {
			return errors.New("local versions cannot be specified")
		}
	default:
		if hasWildcard {
			return errors.New("a wild card is not allowed")
		} else if v.local != "" {
			return errors.New("local versions cannot be specified")
		}
	}
	return nil
}

// Check tests if a version satisfies all the specifiers.
//
// # An empty set of specifiers admits EVERY version
//
// PEP 508 makes a requirement's version specifier optional, and a requirement
// without one accepts any version. The canonical spelling of that is an empty
// specifier set: pypa/packaging parses `flask` to a `SpecifierSet(”)` of length
// zero that contains every version, epochs and pre-releases included.
//
// So "no constraint" means "all versions", not "no versions". Reading it the
// other way inverts every gate built on it — a caller filtering candidates
// against an unconstrained set would discard all of them, and would be told the
// package is unsatisfiable rather than unconstrained.
//
// Note that andCheck below already returns true for an empty AND-group, for the
// same reason: a conjunction over no constraints is vacuously true. Zero
// OR-groups is the same statement one level up, and the two must agree.
// Check is pure MATCHING: it ignores the pre-release policy entirely and asks
// only whether v satisfies the operators. Use Contains for the PEP 440
// selection semantics, where the policy applies.
func (ss Specifiers) Check(v Version) bool {
	if len(ss.specifiers) == 0 {
		return true
	}

	for _, s := range ss.specifiers {
		if andCheck(v, s) {
			return true
		}
	}

	return false
}

// Check reports whether v satisfies this single specifier's operator. Like
// Specifiers.Check it is pure matching, with no pre-release selection.
func (s Specifier) Check(v Version) bool {
	if s.fn == nil {
		// A zero-value Specifier constrains nothing, for the same reason an
		// empty Specifiers admits every version.
		return true
	}
	return s.fn(v, s.operand)
}

// Operator returns the comparison operator as PEP 440 spells it: "==", "!=",
// "<", "<=", ">", ">=", "~=" or "===".
//
// It returns "" for the operator-less bare-version form, which PEP 440 does
// not define and upstream rejects; this package admits it deliberately for the
// R constraint path (see the validConstraintRegexp comment in init).
func (s Specifier) Operator() string { return s.op }

// Version returns the specifier's operand as written, with surrounding
// whitespace removed: `NewSpecifier(">=  v1.0  ").Version()` is "v1.0", not
// "1.0". Like upstream, the operand is normalized when it is compared, not when
// it is stored.
//
// ⚠️ Upstream demonstrates this with case as well (`Specifier("== 1.0A1").version`
// is `1.0A1`). That example does not work here: `==1.0A1` is rejected at parse
// time because validConstraintRegexp lacks the (?i) flag. See
// rstudio/package-manager#19391.
func (s Specifier) Version() string { return s.operand }

// Original returns the specifier exactly as it appeared in the input,
// whitespace included.
func (s Specifier) Original() string { return s.original }

// String returns the normalized form: the operator immediately followed by the
// operand, with the whitespace PEP 440 permits between them removed. So
// `NewSpecifier("< 2").String()` is "<2", matching upstream's `str()`.
func (s Specifier) String() string {
	return s.op + s.operand
}

// String returns the string format of the specifiers.
//
// Each specifier is rendered normalized (see Specifier.String), so
// `NewSpecifiers("< 2").String()` is "<2".
//
// ⚠️ Input order is preserved. Upstream's SpecifierSet.__str__ sorts its
// members, so `SpecifierSet(">=1,<2")` renders as "<2,>=1"; this package
// renders ">=1,<2". That divergence is deliberate and left in place: the
// OR-of-ANDs shape this type carries for the R path (`a || b`) has no upstream
// counterpart to sort within, and reordering would churn Requirement.String
// for no conformance gain.
func (ss Specifiers) String() string {
	var ssStr []string
	for _, orS := range ss.specifiers {
		var sstr []string
		for _, andS := range orS {
			sstr = append(sstr, andS.String())
		}
		ssStr = append(ssStr, strings.Join(sstr, ","))
	}

	return strings.Join(ssStr, "||")
}

// Len returns the number of specifiers in the set, matching upstream's
// `len(SpecifierSet)`. For a set carrying more than one OR-group (the R-only
// `||` syntax, which upstream has no equivalent for) it is the total across
// every group.
func (ss Specifiers) Len() int {
	n := 0
	for _, group := range ss.specifiers {
		n += len(group)
	}
	return n
}

// List returns the individual specifiers, flattened across OR-groups in input
// order. It is the iteration upstream gets from `iter(SpecifierSet)`.
//
// ⚠️ List is for ITERATION, not for SEMANTICS. It discards the OR-of-ANDs
// structure, so `">=1||<2"` and `">=1,<2"` flatten to the same two members
// while meaning opposite things: the first admits every version, the second
// admits only their intersection. Anything that decides whether two sets are
// the same, or what a set matches, must walk the groups instead — see
// orGroups, Equal and matchAll. Deciding a predicate from a flattened List is
// how Equal came to report `">=1||<2"` equal to `">=1,<2"`, and how And came
// to collapse `(A||B) AND C` into `A AND B AND C`.
func (ss Specifiers) List() []Specifier {
	out := make([]Specifier, 0, ss.Len())
	for _, group := range ss.specifiers {
		out = append(out, group...)
	}
	return out
}

func andCheck(v Version, specifiers []Specifier) bool {
	for _, c := range specifiers {
		if !c.Check(v) {
			return false
		}
	}
	return true
}

func versionSplit(version string) []string {
	var result []string
	for _, v := range strings.Split(version, ".") {
		m := prefixRegexp.FindStringSubmatch(v)
		if m != nil {
			result = append(result, m[1:]...)
		} else {
			result = append(result, v)
		}
	}
	return result
}

func isDigist(s string) bool {
	if _, err := strconv.Atoi(s); err == nil {
		return true
	}
	return false
}

func padVersion(left, right []string) ([]string, []string) {
	var leftRelease, rightRelease []string
	for _, l := range left {
		if isDigist(l) {
			leftRelease = append(leftRelease, l)
		}
	}

	for _, r := range right {
		if isDigist(r) {
			rightRelease = append(rightRelease, r)
		}
	}

	// Get the rest of our versions
	leftRest := left[len(leftRelease):]
	rightRest := right[len(rightRelease):]

	for i := 0; i < len(leftRelease)-len(rightRelease); i++ {
		rightRelease = append(rightRelease, "0")
	}
	for i := 0; i < len(rightRelease)-len(leftRelease); i++ {
		leftRelease = append(leftRelease, "0")
	}

	return append(leftRelease, leftRest...), append(rightRelease, rightRest...)
}

//-------------------------------------------------------------------
// Specifier functions
//-------------------------------------------------------------------

// parseOperand parses a matching operand, reporting failure instead of
// panicking.
//
// The comparison functions below used to call MustParse at six sites. Every
// one of them was, at the time, only reachable with an operand the specifier
// grammar had already validated, so none could actually panic through the
// public API — but "unreachable" was a property of the CALLERS, not of these
// functions, and it stopped being true the moment a new caller appeared. The
// exported Specifier type is exactly that new caller: it can be built inside
// this package with any operand at all, and Filter drives these functions
// directly. A panic on the matching path of a library that parses
// remote-supplied requirement strings is not a landmine worth keeping for the
// sake of two fewer lines.
//
// A failed parse means "this comparison cannot be made", which yields no
// match. It cannot mean "match", because that would admit versions on the
// strength of an operand nobody could interpret.
func parseOperand(s string) (Version, bool) {
	v, err := Parse(s)
	if err != nil {
		return Version{}, false
	}
	return v, true
}

func specifierCompatible(prospective Version, spec string) bool {
	// Compatible releases have an equivalent combination of >= and ==. That is that ~=2.2 is equivalent to >=2.2,==2.*.
	// This allows us to implement this in terms of the other specifiers instead of implementing it ourselves.
	// The only thing we need to do is construct the other specifiers.

	var prefixElements []string
	for _, s := range versionSplit(spec) {
		if strings.HasPrefix(s, "post") || strings.HasPrefix(s, "dev") {
			break
		}
		prefixElements = append(prefixElements, s)
	}

	// ⚠️ An operand whose FIRST dot-segment already starts with "post" or "dev"
	// breaks the loop on iteration one, leaving prefixElements empty -- and
	// slicing [:len-1] of an empty slice panics with "slice bounds out of range
	// [:-1]". Reachable with the operand "dev1", "post1", "dev" or "post".
	//
	// This is the same landmine class as the MustParse sites, and note which
	// operands DO NOT trigger it: "lolwat", "", "not a version" and "1.2.3.4.5-garbage"
	// all pass through harmlessly, because they produce a non-empty first
	// segment. A hardening test stocked only with those shapes therefore proves
	// nothing about this line, which is exactly how it was missed.
	if len(prefixElements) == 0 {
		return false
	}

	// We want everything but the last item in the version, but we want to ignore post and dev releases, and
	// we want to treat the pre-release as its own separate segment.
	prefix := strings.Join(prefixElements[:len(prefixElements)-1], ".")

	// Add the prefix notation to the end of our string
	prefix += ".*"

	return specifierGreaterThanEqual(prospective, spec) && specifierEqual(prospective, prefix)
}

func specifierEqual(prospective Version, spec string) bool {
	// https://github.com/pypa/packaging/blob/a6407e3a7e19bd979e93f58cfc7f6641a7378c46/packaging/specifiers.py#L476
	// We need special logic to handle prefix matching
	if strings.HasSuffix(spec, ".*") {
		// In the case of prefix matching we want to ignore local segment.
		public, ok := parseOperand(prospective.Public())
		if !ok {
			return false
		}
		prospective = public

		// Split the spec out by dots, and pretend that there is an implicit
		// dot in between a release segment and a pre-release segment.
		splitSpec := versionSplit(strings.TrimSuffix(spec, ".*"))

		// Split the prospective version out by dots, and pretend that there is an implicit dot
		//  in between a release segment and a pre-release segment.
		splitProspective := versionSplit(prospective.String())

		// Shorten the prospective version to be the same length as the spec
		// so that we can determine if the specifier is a prefix of the
		// prospective version or not.
		if len(splitProspective) > len(splitSpec) {
			splitProspective = splitProspective[:len(splitSpec)]
		}

		paddedSpec, paddedProspective := padVersion(splitSpec, splitProspective)
		return reflect.DeepEqual(paddedSpec, paddedProspective)
	}

	specVersion, ok := parseOperand(spec)
	if !ok {
		return false
	}
	if specVersion.local == "" {
		public, ok := parseOperand(prospective.Public())
		if !ok {
			return false
		}
		prospective = public
	}

	return specVersion.Equal(prospective)
}

func specifierNotEqual(prospective Version, spec string) bool {
	return !specifierEqual(prospective, spec)
}

func specifierLessThan(prospective Version, spec string) bool {
	// Convert our spec to a Version instance, since we'll want to work with it as a version.
	s, ok := parseOperand(spec)
	if !ok {
		return false
	}

	// Check to see if the prospective version is less than the spec version.
	// If it's not we can short circuit and just return False now instead of doing extra unneeded work.
	if !prospective.LessThan(s) {
		return false
	}

	// This special case is here so that, unless the specifier itself includes is a pre-release version,
	// that we do not accept pre-release versions for the version mentioned in the specifier
	// (e.g. <3.1 should not match 3.1.dev0, but should match 3.0.dev0).
	//
	// ⚠️ This guard is MATCHING, not candidate selection: no pre-release
	// policy turns it off. `<3.1` does not contain `3.1.dev0` even with
	// PreReleasesInclude, which is what pypa/packaging 26.2 does.
	if !s.IsPreRelease() && prospective.IsPreRelease() {
		if sameBaseVersion(prospective, s) {
			return false
		}
	}
	return true
}

// sameBaseVersion reports whether two versions share a base version (release
// segment and epoch, with pre/post/dev/local discarded).
func sameBaseVersion(a, b Version) bool {
	av, ok := parseOperand(a.BaseVersion())
	if !ok {
		return false
	}
	bv, ok := parseOperand(b.BaseVersion())
	if !ok {
		return false
	}
	return av.Equal(bv)
}

func specifierGreaterThan(prospective Version, spec string) bool {
	// Convert our spec to a Version instance, since we'll want to work with it as a version.
	s, ok := parseOperand(spec)
	if !ok {
		return false
	}

	// Check to see if the prospective version is greater than the spec version.
	// If it's not we can short circuit and just return False now instead of doing extra unneeded work.
	if !prospective.GreaterThan(s) {
		return false
	}

	// This special case is here so that, unless the specifier itself includes is a post-release version,
	// that we do not accept post-release versions for the version mentioned in the specifier
	// (e.g. >3.1 should not match 3.0.post0, but should match 3.2.post0).
	//
	// ⚠️ Matching, not candidate selection. See specifierLessThan.
	if !s.IsPostRelease() && prospective.IsPostRelease() {
		if sameBaseVersion(prospective, s) {
			return false
		}
	}

	// Ensure that we do not allow a local version of the version mentioned
	//  in the specifier, which is technically greater than, to match.
	if prospective.local != "" {
		if sameBaseVersion(prospective, s) {
			return false
		}
	}
	return true
}

func specifierArbitrary(prospective Version, spec string) bool {
	return strings.EqualFold(prospective.String(), spec)
}

func specifierLessThanEqual(prospective Version, spec string) bool {
	p, pok := parseOperand(prospective.Public())
	s, sok := parseOperand(spec)
	if !pok || !sok {
		return false
	}
	return p.LessThanOrEqual(s)
}

func specifierGreaterThanEqual(prospective Version, spec string) bool {
	p, pok := parseOperand(prospective.Public())
	s, sok := parseOperand(spec)
	if !pok || !sok {
		return false
	}
	return p.GreaterThanOrEqual(s)
}
