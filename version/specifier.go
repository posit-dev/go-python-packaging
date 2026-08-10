// SPDX-License-Identifier: Apache-2.0 OR MIT

package version

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
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

	specifierRegexp       *regexp.Regexp
	validConstraintRegexp *regexp.Regexp
	prefixRegexp          *regexp.Regexp
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

	// ⚠️ The whitespace class must be [\s\v], not \s. Go's \s is [\t\n\f\r ]
	// and EXCLUDES the vertical tab (0x0B); Python's re \s is [ \t\n\r\f\v] and
	// includes it. With a bare \s, the operator/operand separator would not
	// match "\v", so `<=  \r \f \v v1.0` was rejected while pypa/packaging
	// accepts it. versionRegex was already fixed for this; the specifier
	// patterns were not.
	const wsp = `[\s\v]`

	specifierRegexp = regexp.MustCompile(
		fmt.Sprintf(
			`(?i)(?:(?P<arbitraryop>===)%[1]s*(?P<arbitrary>%[2]s)|(?P<operator>(%[3]s))%[1]s*(?P<version>%[4]s(\.\*)?))`,
			wsp,
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
		`(?:===%[1]s*%[2]s|(%[3]s)%[1]s*(%[4]s(\.\*)?))`,
		wsp,
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
	// ⚠️ The (?i) flag is REQUIRED here and was missing.
	//
	// specifierRegexp has it, this gate did not, so the two disagreed about
	// case: `==1.0a1` passed the gate and parsed, while `==1.0A1` was rejected
	// by the gate before specifierRegexp ever saw it. PEP 440 versions are
	// case-insensitive, and pypa/packaging accepts every upper-case spelling
	// (`==1.0DEV`, `==1.0ALPHA1`, `>=7.9A1`, `~=1.0.POST1`, ...). This was the
	// single largest source of conformance failures in
	// tests/test_specifiers.py's normalization table.
	validConstraintRegexp = regexp.MustCompile(
		fmt.Sprintf(`(?i)^%[1]s*(?:%[2]s%[1]s*(?:,%[1]s*%[2]s%[1]s*)*,?)?%[1]s*$`, wsp, constraint),
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
	// the sanitizer applied. It is NOT normalized as a version: upstream's
	// Specifier.version returns the operand as written, so `==1.0A1` keeps
	// its upper-case spelling. Verified against pypa/packaging 26.2.
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

	// A specifier-less group may only be the universal set when the input is a
	// single group. See parseGroup.
	allowEmpty := len(segments) == 1

	var sss [][]Specifier
	for _, vv := range segments {
		if strings.TrimSpace(vv) == "*" {
			vv = ">=0.0.0"
		}
		vv = strings.ReplaceAll(vv, "-", ".")

		specs, err := parseGroup(vv, sanitizer, *c, allowEmpty)
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

	// A specifier-less group may only be the universal set when the input is a
	// single group. See parseGroup.
	allowEmpty := len(segments) == 1

	var sss [][]Specifier
	for _, vv := range segments {
		if strings.TrimSpace(vv) == "*" {
			vv = ">=0.0.0"
		}

		specs, err := parseGroup(vv, sanitizer, *c, allowEmpty)
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
// `Specifier(spec, prereleases=...)` constructor. A comma-separated set is not
// a single specifier: pass those to NewSpecifiers instead.
func NewSpecifier(s string, opts ...SpecifierOption) (Specifier, error) {
	c := new(conf)
	for _, o := range opts {
		o.apply(c)
	}

	// Reject anything the specifier grammar does not accept in full, so that
	// NewSpecifier(">=1,<2") is an error rather than a silent ">=1" with the
	// rest dropped on the floor. specifierRegexp is unanchored (Specifiers
	// scans with it), so a bare FindStringSubmatch would happily match a
	// prefix.
	if !validConstraintRegexp.MatchString(s) {
		return Specifier{}, fmt.Errorf("improper specifier: %s", s)
	}
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
// specifiers.split(",") if s.strip()]`), so `packaging` accepts ",>=1",
// ">=1,,<2" and ">=1,"; this function does the same, by dropping blank items
// before validating. Dropping them cannot smuggle in an adjacent pair, because
// what is left is re-joined with commas and still has to satisfy
// validConstraintRegexp -- ">=1<2" is a single item with no comma to drop and
// stays rejected.
//
// allowEmpty says whether a group that ends up with NO items may be the
// universal set. It is true only for a single-group input, and that is the one
// place this package deliberately stops short of upstream: `SpecifierSet(",")`
// is universal in packaging 26.2, and `NewSpecifiers(",")` is universal here
// too, but `">=1||,"` is an ERROR rather than universal. Upstream has no "||"
// and so has no opinion; letting a comma-only SEGMENT become an empty group
// would re-open the exact hole described above, just reached by a comma typo
// instead of a "||" typo.
func parseGroup(vv string, sanitizer func(string) string, c conf, allowEmpty bool) ([]Specifier, error) {
	// Drop blank comma-separated items, as upstream does.
	items := make([]string, 0, strings.Count(vv, ",")+1)
	for _, item := range strings.Split(vv, ",") {
		if strings.TrimSpace(item) == "" {
			continue
		}
		items = append(items, item)
	}

	if len(items) == 0 {
		if !allowEmpty {
			return nil, fmt.Errorf(
				"improper constraint: %q has no constraints in it (an empty || segment is not allowed)", vv)
		}
		// Every item was blank, so the whole input is punctuation: the
		// universal set, matching SpecifierSet(",").
		return nil, nil
	}

	normalized := strings.Join(items, ",")
	if !validConstraintRegexp.MatchString(normalized) {
		return nil, fmt.Errorf("improper constraint: %s", vv)
	}

	found := specifierRegexp.FindAllString(normalized, -1)
	if found == nil {
		found = append(found, strings.TrimSpace(normalized))
	}

	specs := make([]Specifier, 0, len(found))
	for _, single := range found {
		s, err := newSpecifier(single, sanitizer, c)
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
		// ⚠️ The PRE-release arm was missing. This guard checked dev, post and
		// local but never pre, so `==1.0a1.*` and `!=2.0a1.*` were wrongly
		// ACCEPTED while pypa/packaging rejects them. PEP 440 does not allow a
		// wildcard alongside any of the four, and a prefix match against a
		// pre-release is meaningless: the wildcard covers the release segment,
		// which is where the pre-release marker would have to sit.
		if hasWildcard && (!v.pre.isNull() || !v.dev.isNull() || !v.post.isNull() || v.local != "") {
			return errors.New(
				"the (non)equality operators don't allow to use a wild card and a pre, dev, post, or local version together",
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
// whitespace removed: `Specifier("== 1.0A1").Version()` is "1.0A1", not
// "1.0a1". Upstream does the same — the operand is normalized when it is
// compared, not when it is stored.
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

// versionSplit splits a version string into the components prefix matching and
// the compatible operator compare, ported from pypa/packaging 26.2's
// _version_split.
//
// Two details are load-bearing and were both missing from the earlier
// hand-rolled version:
//
//   - The EPOCH is split off on "!" and becomes the first component, defaulting
//     to "0". Without it, `2` and `0!2` split to different lengths and
//     `==0!2.*` failed to match `2` even though they are the same version.
//   - A release-plus-pre-release run in one dot-segment ("2rc1") is split into
//     two components ("2", "rc1"), so that a pre-release behaves as its own
//     segment for prefix purposes.
//
// The result is for comparison only; joining it back with versionJoin does not
// necessarily reproduce the input.
func versionSplit(version string) []string {
	var result []string

	// rpartition on "!": everything before the LAST "!" is the epoch.
	epoch := "0"
	rest := version
	if i := strings.LastIndex(version, "!"); i >= 0 {
		if version[:i] != "" {
			epoch = version[:i]
		}
		rest = version[i+1:]
	}
	result = append(result, epoch)

	for _, item := range strings.Split(rest, ".") {
		if m := prefixRegexp.FindStringSubmatch(item); m != nil {
			result = append(result, m[1:]...)
		} else {
			result = append(result, item)
		}
	}
	return result
}

// versionJoin rebuilds a version string from a versionSplit result, ported from
// pypa/packaging 26.2's _version_join. The first component is the epoch.
func versionJoin(components []string) string {
	if len(components) == 0 {
		return ""
	}
	return components[0] + "!" + strings.Join(components[1:], ".")
}

// isNotSuffix reports whether a versionSplit component is part of the release
// segment rather than a pre/post/dev suffix. Ported from pypa/packaging 26.2's
// _is_not_suffix; it is the predicate the compatible operator uses to find
// where the release segment ends.
func isNotSuffix(segment string) bool {
	for _, prefix := range []string{"dev", "a", "b", "rc", "post"} {
		if strings.HasPrefix(segment, prefix) {
			return false
		}
	}
	return true
}

// numericPrefixLen counts the leading all-digit components of a versionSplit
// result. Ported from pypa/packaging 26.2's _numeric_prefix_len.
func numericPrefixLen(split []string) int {
	count := 0
	for _, segment := range split {
		if !isASCIIDigits(segment) {
			break
		}
		count++
	}
	return count
}

// leftPad pads a versionSplit result with "0" components until it has
// targetNumericLen numeric components, preserving any suffix components.
// Ported from pypa/packaging 26.2's _left_pad.
//
// ⚠️ The padding is inserted AFTER the numeric prefix and BEFORE the suffix,
// which is what the earlier padVersion got wrong: it padded only to the other
// side's length and reassembled the two halves symmetrically, so `2` compared
// against the spec `2.0.0.*` was never widened to three numeric components and
// the prefix match failed.
func leftPad(split []string, targetNumericLen int) []string {
	numericLen := numericPrefixLen(split)
	padNeeded := targetNumericLen - numericLen
	if padNeeded <= 0 {
		return split
	}
	out := make([]string, 0, len(split)+padNeeded)
	out = append(out, split[:numericLen]...)
	for i := 0; i < padNeeded; i++ {
		out = append(out, "0")
	}
	out = append(out, split[numericLen:]...)
	return out
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

	// Everything but the last item of the RELEASE segment. The loop stops at the
	// first suffix component, so a pre-release in the operand is not mistaken
	// for part of the release.
	//
	// ⚠️ The earlier version broke only on "post" and "dev", so a pre-release
	// operand kept its "a1"/"rc1" component and then had it dropped as "the
	// last item" -- which made `~=1.0a1` derive the prefix `1.*` rather than
	// dropping only the release's last component. versionSplit now also
	// prepends the epoch, so the operand and the prospective version agree on
	// component count.
	var prefixElements []string
	for _, s := range versionSplit(spec) {
		if !isNotSuffix(s) {
			break
		}
		prefixElements = append(prefixElements, s)
	}
	if len(prefixElements) == 0 {
		// Unreachable for a validated "~=" operand: the grammar requires at
		// least two release digits, so there is always an epoch component and
		// at least two release components.
		return false
	}

	prefix := versionJoin(prefixElements[:len(prefixElements)-1]) + ".*"

	return specifierGreaterThanEqual(prospective, spec) && specifierEqual(prospective, prefix)
}

func specifierEqual(prospective Version, spec string) bool {
	// We need special logic to handle prefix matching. Ported from
	// pypa/packaging 26.2's Specifier._compare_equal.
	if strings.HasSuffix(spec, ".*") {
		// In the case of prefix matching we want to ignore the local segment.
		public, ok := parseOperand(prospective.Public())
		if !ok {
			return false
		}
		prospective = public

		// Split the spec out by bangs and dots, pretending there is an implicit
		// dot between a release segment and a pre-release segment. The operand
		// goes through Parse first so that its normalized form (leading zeros,
		// separator spellings, letter case) is what gets split -- otherwise
		// `==1.01.*` and `==1.1.*` would split differently.
		operand := strings.TrimSuffix(spec, ".*")
		specVersion, ok := parseOperand(operand)
		if !ok {
			return false
		}
		splitSpec := versionSplit(specVersion.String())
		specNumericLen := numericPrefixLen(splitSpec)

		splitProspective := versionSplit(prospective.String())

		// ⚠️ Pad BEFORE shortening. Padding the prospective version up to the
		// spec's numeric width first is what lets `2` match `==2.0.0.*`: it
		// becomes ["0","2","0","0"], which then truncates to the spec exactly.
		// Truncating first would have thrown away the room the padding needs.
		paddedProspective := leftPad(splitProspective, specNumericLen)

		// Shorten the prospective version to the spec's length, so that the
		// question becomes whether the spec is a prefix of it.
		if len(paddedProspective) > len(splitSpec) {
			paddedProspective = paddedProspective[:len(splitSpec)]
		}

		return slices.Equal(paddedProspective, splitSpec)
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

	// PEP 440: "<V MUST NOT allow a pre-release of the specified version unless
	// the specified version is itself a pre-release."
	//
	// ⚠️ This guard is MATCHING, not candidate selection: no pre-release policy
	// turns it off. `<3.1` does not contain `3.1.dev0` even with
	// PreReleasesInclude, which is what pypa/packaging 26.2 does.
	//
	// ⚠️ "a pre-release OF the specified version" is a lower bound, not a
	// shared base version. The earlier `sameBaseVersion` test was far too
	// broad: for the spec `1.0.post1` it treated `1.0.dev0` as a pre-release of
	// it, because both have base version 1.0 -- but `1.0.dev0` is a pre-release
	// of `1.0`, not of `1.0.post1`, and upstream accepts it. Comparing against
	// the EARLIEST pre-release of the spec draws the boundary where PEP 440
	// puts it.
	if !s.IsPreRelease() && prospective.IsPreRelease() {
		earliest, ok := s.earliestPreRelease()
		if ok && prospective.GreaterThanOrEqual(earliest) {
			return false
		}
	}
	return true
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

	// PEP 440: ">V MUST NOT allow a post-release of the specified version unless
	// the specified version is itself a post-release."
	//
	// ⚠️ Matching, not candidate selection. See specifierLessThan.
	//
	// ⚠️ "a post-release OF the specified version" means the version this one is
	// a post-release of IS the spec, exactly. The earlier `sameBaseVersion` test
	// was too broad: for the spec `1.0a1` it excluded `1.0.post0`, because both
	// have base version 1.0 -- but `1.0.post0` is a post-release of `1.0`, not
	// of `1.0a1`, and upstream accepts it.
	if !s.IsPostRelease() && prospective.IsPostRelease() {
		if base, ok := prospective.postBase(); ok && base.Equal(s) {
			return false
		}
	}

	// PEP 440: ">V MUST NOT match a local version of the specified version."
	//
	// ⚠️ A "local version of V" is one whose PUBLIC part equals V -- pre, post
	// and dev segments included. `sameBaseVersion` discarded those, so `>1.0a1`
	// wrongly rejected `1.0a2+local`: same base version 1.0, but a different
	// public version, and upstream matches it.
	if prospective.local != "" {
		public, ok := parseOperand(prospective.Public())
		if ok && public.Equal(s) {
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
