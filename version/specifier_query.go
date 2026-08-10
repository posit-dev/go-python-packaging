// SPDX-License-Identifier: Apache-2.0 OR MIT

// This file holds the query half of the specifier API — the pre-release-aware
// Contains and Filter methods, the resolved PreReleases policy, canonical
// equality, and set combination. It follows the structure of
// pypa/packaging's specifiers.py, where `contains` is defined as
// `bool(list(self.filter([item])))` so the two can never disagree.
//
// Upstream pinned at 4eb0753dba8fcaaac8eb75463374e448f0931558
// (pypa/packaging 26.2).

package version

import (
	"errors"
	"slices"
	"strings"
)

// ErrPreReleaseConflict is returned by Specifiers.And when the two sets carry
// contradictory explicit pre-release policies (one Include, one Exclude).
// Upstream raises ValueError in the same case; PreReleasesAuto on either side
// yields to the other.
var ErrPreReleaseConflict = errors.New("cannot combine specifier sets with conflicting explicit pre-release policies")

//-------------------------------------------------------------------
// Resolved pre-release policy
//-------------------------------------------------------------------

// PreReleases returns the pre-release policy in force for this specifier: the
// policy it was built with, or, for PreReleasesAuto, the one PEP 440 derives
// from the specifier itself.
//
// The derivation matches upstream's Specifier.prereleases property exactly:
//
//   - `!=` never implies pre-releases, even against a pre-release operand.
//   - `==` with a `.*` wildcard never implies pre-releases.
//   - `===` cannot be parsed as a version at all, so the policy stays
//     unresolved (upstream's `None`, PreReleasesAuto here).
//   - otherwise, the policy is Include exactly when the operand is itself a
//     pre-release, so `>=1.0.dev1` offers pre-releases and `>=1.0` does not.
func (s Specifier) PreReleases() PreReleases {
	if s.pre != PreReleasesAuto {
		return s.pre
	}

	switch {
	case s.op == "!=":
		return PreReleasesExclude
	case s.op == "==" && strings.HasSuffix(s.operand, ".*"):
		return PreReleasesExclude
	}

	// `===` gets no special case: upstream tries to parse its operand like any
	// other and only falls through to "unresolved" when that fails. So
	// `===lolwat` is unresolved but `===1.0.dev1` resolves to Include, because
	// its operand does happen to be a pre-release version.
	v, err := Parse(s.operand)
	if err != nil {
		return PreReleasesAuto
	}
	if v.IsPreRelease() {
		return PreReleasesInclude
	}
	return PreReleasesExclude
}

// PreReleases returns the pre-release policy in force for the set: the policy
// it was built with, or, for PreReleasesAuto, Include when any member
// specifier resolves to Include and PreReleasesAuto otherwise.
//
// ⚠️ Note the asymmetry with Specifier.PreReleases, which is upstream's and
// not an oversight: a set of ordinary final-release specifiers resolves to
// PreReleasesAuto (`None`), never to Exclude, even though each member on its
// own resolves to Exclude. That is what lets a set fall back to offering
// pre-releases when nothing else matched, while a single specifier queried
// directly does not.
func (ss Specifiers) PreReleases() PreReleases {
	if ss.conf.preReleases != PreReleasesAuto {
		return ss.conf.preReleases
	}
	if ss.Len() == 0 {
		return PreReleasesAuto
	}
	for _, s := range ss.List() {
		if s.PreReleases() == PreReleasesInclude {
			return PreReleasesInclude
		}
	}
	return PreReleasesAuto
}

//-------------------------------------------------------------------
// Specifier: Contains / Filter
//-------------------------------------------------------------------

// Contains reports whether item satisfies this specifier under the PEP 440
// pre-release selection rules.
//
// item is a raw string, and that matters for `===`: arbitrary equality
// compares the operand against the string as written, so `===1.01` contains
// the string "1.01" but not the string "1.1". Pass a parsed Version to
// ContainsVersion for the normalized comparison instead.
//
// A string that is not a valid PEP 440 version is not an error: it simply
// cannot match anything except a `===` operand spelled the same way.
//
// ⚠️ A pre-release DOES satisfy an ordinary specifier when queried on its own.
// Contains(">=1.0", "1.5a1") is true, because with one candidate under
// consideration there is no final release to prefer. See the PreReleases doc
// comment.
func (s Specifier) Contains(item string, opts ...FilterOption) bool {
	return len(s.Filter([]string{item}, opts...)) > 0
}

// ContainsVersion is Contains for an already-parsed Version. The only
// observable difference is for `===`, which compares against v's normalized
// rendering rather than the caller's original spelling.
func (s Specifier) ContainsVersion(v Version, opts ...FilterOption) bool {
	return len(s.FilterVersions([]Version{v}, opts...)) > 0
}

// Filter returns the members of items this specifier offers as candidates,
// preserving input order.
//
// The PEP 440 rule is a preference, not a hard exclusion: pre-releases are
// held back while any final release matches, and offered when none does.
//
//	Filter(">=1.2.3", ["1.2", "1.3", "1.5a1"])  ->  ["1.3"]
//	Filter(">=1.2.3", ["1.2", "1.5a1"])         ->  ["1.5a1"]
//
// Pass WithPreReleases to override the policy for this call only.
func (s Specifier) Filter(items []string, opts ...FilterOption) []string {
	return filterGeneric(s, items, func(v string) string { return v }, opts)
}

// FilterVersions is Filter over already-parsed versions.
func (s Specifier) FilterVersions(items []Version, opts ...FilterOption) []Version {
	return filterGeneric(s, items, Version.String, opts)
}

// Equal reports whether two specifiers are equal after canonicalization,
// matching upstream's Specifier.__eq__. The pre-release policy is ignored,
// trailing zeros in the operand are not significant, and a `v` prefix, a zero
// epoch and local-segment case are normalized:
//
//	==2.8.0  ==  ==2.8      (true)
//	==v1.0   ==  ==1.0      (true)
//	==0!1.0  ==  ==1.0      (true)
//	~=1.18.0 ==  ~=1.18     (FALSE — `~=` keeps trailing zeros, because they
//	                         change which versions it matches)
//
// A `===` operand and a `.*` wildcard are compared verbatim, since neither is
// a version that can be canonicalized.
//
// ⚠️ Canonicalization would also make `==1.0A1` equal `==1.0a1`, as it does
// upstream, but neither `==1.0A1` nor `==1.0+ABC` can be built here at all:
// validConstraintRegexp lacks the (?i) flag, so both are rejected at parse
// time. See rstudio/package-manager#19391.
func (s Specifier) Equal(other Specifier) bool {
	lop, lv := s.canonicalSpec()
	rop, rv := other.canonicalSpec()
	return lop == rop && lv == rv
}

// canonicalSpec is upstream's Specifier._canonical_spec.
func (s Specifier) canonicalSpec() (op, operand string) {
	if s.op == "===" || strings.HasSuffix(s.operand, ".*") {
		return s.op, s.operand
	}
	v, err := Parse(s.operand)
	if err != nil {
		// Unreachable for a specifier built by this package: every operand
		// except `===` is validated at construction. Degrade to the raw text
		// rather than panicking on a zero value.
		return s.op, s.operand
	}
	if s.op == "~=" {
		// Trailing zeros are load-bearing for the compatible operator:
		// `~=1.18` and `~=1.18.0` accept different version sets.
		return s.op, v.String()
	}
	return s.op, v.trimmedRelease()
}

//-------------------------------------------------------------------
// Specifiers: Contains / Filter / And
//-------------------------------------------------------------------

// Contains reports whether item satisfies every specifier in the set, under
// the PEP 440 pre-release selection rules. See Specifier.Contains for how a
// raw string interacts with `===`.
//
// An empty set contains every version, pre-releases included.
func (ss Specifiers) Contains(item string, opts ...FilterOption) bool {
	return len(ss.Filter([]string{item}, opts...)) > 0
}

// ContainsVersion is Contains for an already-parsed Version.
func (ss Specifiers) ContainsVersion(v Version, opts ...FilterOption) bool {
	return len(ss.FilterVersions([]Version{v}, opts...)) > 0
}

// ContainsInstalled is Contains with upstream's `installed=True` carve-out: an
// already-installed pre-release is accepted even when the set would not offer
// it as a new candidate, because refusing to recognize what is on disk is not
// useful. It has no effect on a version that is not a pre-release.
func (ss Specifiers) ContainsInstalled(item string, opts ...FilterOption) bool {
	if v, err := Parse(item); err == nil && v.IsPreRelease() {
		opts = append(opts, WithPreReleases(PreReleasesInclude))
	}
	return ss.Contains(item, opts...)
}

// Filter returns the members of items the set offers as candidates,
// preserving input order. See Specifier.Filter for the pre-release rule.
func (ss Specifiers) Filter(items []string, opts ...FilterOption) []string {
	return filterGeneric(ss, items, func(v string) string { return v }, opts)
}

// FilterVersions is Filter over already-parsed versions.
func (ss Specifiers) FilterVersions(items []Version, opts ...FilterOption) []Version {
	return filterGeneric(ss, items, Version.String, opts)
}

// Equal reports whether two sets are the same constraint, compared
// canonically: insensitive to the order of the OR-groups, to the order and
// duplication of members within a group, to trailing zeros and letter case in
// an operand, and to the pre-release policy. It is upstream's
// SpecifierSet.__eq__, extended to this type's OR-of-ANDs shape.
//
// ⚠️ It is NOT insensitive to which members share a group, because that is what
// the constraint IS. `">=1||<2"` admits every version and `">=1,<2"` admits
// only their intersection, so they must not compare equal even though they hold
// the same two members. An earlier version compared a flattened List and did
// report them equal — a wrong equality is especially costly, because it makes
// whatever is built on top of it (dedupe, caches, guards, tests that use it as
// an oracle) silently wrong while Equal's own tests still pass.
func (ss Specifiers) Equal(other Specifiers) bool {
	left := canonicalGroupKeys(ss)
	right := canonicalGroupKeys(other)
	if len(left) != len(right) {
		return false
	}
	for k := range left {
		if _, ok := right[k]; !ok {
			return false
		}
	}
	return true
}

// canonicalGroupKeys returns one key per AND-group: the group's canonicalized
// members, deduplicated and sorted, joined with ",". The result is a SET of
// group keys, so comparison is blind to group order and to within-group order
// and duplicates, while conjunction boundaries survive.
//
// For a set with a single group — everything a Python-style comma-separated
// string produces, and so everything the upstream conformance tables exercise —
// this reduces exactly to the deduplicated, sorted member set.
func canonicalGroupKeys(ss Specifiers) map[string]struct{} {
	groups := ss.orGroups()
	keys := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		members := make([]string, 0, len(group))
		seen := make(map[string]struct{}, len(group))
		for _, s := range group {
			op, operand := s.canonicalSpec()
			key := op + operand
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			members = append(members, key)
		}
		slices.Sort(members)
		keys[strings.Join(members, ",")] = struct{}{}
	}
	return keys
}

// orGroups returns the AND-groups, normalizing a set with no groups at all (the
// zero value) to a single EMPTY group.
//
// Those two spellings mean the same thing — a set that constrains nothing, see
// Check — and normalizing here keeps every group-walking operation from having
// to special-case the zero value. It also makes the universal set the identity
// of And, which it must be: `universal AND X` is `X`, and a cartesian product
// against zero groups would instead have produced zero groups.
func (ss Specifiers) orGroups() [][]Specifier {
	if len(ss.specifiers) == 0 {
		return [][]Specifier{nil}
	}
	return ss.specifiers
}

// And returns the intersection of two specifier sets, which is upstream's
// `SpecifierSet.__and__`.
//
// ⚠️ Because this type is an OR of ANDs, intersection DISTRIBUTES; it is not a
// concatenation. `(A||B) AND (C||D)` is `(A,C) || (A,D) || (B,C) || (B,D)`.
// Concatenating the members instead produced `A AND B AND C AND D`, which is a
// strictly narrower and usually unsatisfiable constraint: `(==1.0||==2.0) AND
// <3.0` became `==1.0,==2.0,<3.0` and matched nothing at all, rejecting the two
// versions it should have admitted.
//
// Upstream never has more than one group, so for two Python-style sets this
// collapses to the single concatenated group upstream produces, duplicates
// included.
//
// The pre-release policies of the two SETS are combined rather than dropped.
// PreReleasesAuto on one side yields to the other; two identical explicit
// policies survive; Include on one side and Exclude on the other is a
// contradiction and returns ErrPreReleaseConflict, as upstream raises
// ValueError.
//
// ⚠️ Member policies are left alone, as upstream's `__and__` leaves them. An
// earlier version re-stamped every member with the combined policy, which could
// ERASE a member's explicit policy and then let autodetection reach the opposite
// answer: a member built with PreReleasesExclude on the operand `>=1.0.dev1`
// became Auto, autodetected to Include off the `.dev1`, and flipped the whole
// set's resolved policy from Auto to Include -- so the combined set offered a
// pre-release the original had held back. A member's policy is the member's.
func (ss Specifiers) And(other Specifiers) (Specifiers, error) {
	pre, err := combinePreReleases(ss.conf.preReleases, other.conf.preReleases)
	if err != nil {
		return Specifiers{}, err
	}

	left := ss.orGroups()
	right := other.orGroups()

	groups := make([][]Specifier, 0, len(left)*len(right))
	for _, l := range left {
		for _, r := range right {
			groups = append(groups, dedupeSpecifiers(l, r))
		}
	}

	return Specifiers{
		specifiers: groups,
		conf:       conf{preReleases: pre},
	}, nil
}

// dedupeSpecifiers concatenates AND-groups, dropping any member whose canonical
// form has already been seen. First-occurrence order is preserved (this package
// does not sort; see Specifiers.String).
//
// ⚠️ Deduplicating is a divergence from a LITERAL reading of upstream, and
// agreement with the settled one.
//
// `len(SpecifierSet(">=1.0") & SpecifierSet(">=1.0"))` really is 2 in packaging
// 26.2 -- but only on a fresh object. Ask for `str()` first and it is 1, and
// asking for `len()` again after `str()` also gives 1, because
// `_canonical_specs()` deduplicates LAZILY and every canonicalizing operation
// triggers it. So 2 is a transient pre-canonicalization artifact of upstream's
// caching, not its answer; `str()` is `">=1.0"` under every ordering. An earlier
// version of this package cited that 2 as deliberate upstream parity, which was
// wrong twice over: it anchored a compatibility claim on the one thing this port
// explicitly declares non-portable (lazy caches), and it made the count depend
// on which method a caller happened to call first.
//
// Matching is unaffected either way, because a conjunction is idempotent -- this
// is purely about what Len and String report.
func dedupeSpecifiers(groups ...[]Specifier) []Specifier {
	size := 0
	for _, g := range groups {
		size += len(g)
	}
	out := make([]Specifier, 0, size)
	seen := make(map[string]struct{}, size)
	for _, g := range groups {
		for _, s := range g {
			op, operand := s.canonicalSpec()
			key := op + operand
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}

func combinePreReleases(left, right PreReleases) (PreReleases, error) {
	switch {
	case left == PreReleasesAuto:
		return right, nil
	case right == PreReleasesAuto, left == right:
		return left, nil
	default:
		return PreReleasesAuto, ErrPreReleaseConflict
	}
}

// NewSpecifiersFrom builds a set from already-parsed specifiers, which is
// upstream's `SpecifierSet(iterable_of_Specifier)`. The members are used as
// given; no re-parsing happens.
//
// ⚠️ A policy passed here applies to the SET and does not overwrite the members'
// own, matching upstream: `SpecifierSet(iterable, prereleases=X)` sets its own
// `_prereleases` and leaves each member's alone. An earlier version stamped it
// onto every member, which discarded exactly the per-member policy this
// constructor exists to carry through.
func NewSpecifiersFrom(specs []Specifier, opts ...SpecifierOption) Specifiers {
	c := new(conf)
	for _, o := range opts {
		o.apply(c)
	}
	group := make([]Specifier, len(specs))
	copy(group, specs)
	return Specifiers{specifiers: [][]Specifier{group}, conf: *c}
}

//-------------------------------------------------------------------
// The filter engine
//-------------------------------------------------------------------

// Filterer is the filtering behavior shared by Specifier and Specifiers.
//
// It is a sealed interface — its only method is unexported — because the
// pre-release bookkeeping in the implementations below is subtle enough that a
// third-party implementation would almost certainly get the "offer a
// pre-release only when nothing else matched" rule wrong.
type Filterer interface {
	// filterKeep returns, for each input string, whether it is offered.
	filterKeep(items []string, fc filterConf) []bool
}

// FilterBy filters an arbitrary slice, deriving each element's version string
// with key. It is upstream's `key=` parameter on filter, expressed as a free
// generic function because Go methods cannot take type parameters.
//
//	FilterBy(specs, releases, func(r Release) string { return r.Version })
func FilterBy[T any](f Filterer, items []T, key func(T) string, opts ...FilterOption) []T {
	return filterGeneric(f, items, key, opts)
}

func filterGeneric[T any](f Filterer, items []T, key func(T) string, opts []FilterOption) []T {
	strs := make([]string, len(items))
	for i, it := range items {
		strs[i] = key(it)
	}
	keep := f.filterKeep(strs, newFilterConf(opts))

	out := make([]T, 0, len(items))
	for i, k := range keep {
		if k {
			out = append(out, items[i])
		}
	}
	return out
}

// filterKeep implements upstream's Specifier.filter for a single specifier.
//
// The three-way bookkeeping is upstream's and is easy to get subtly wrong, so
// it is spelled out: a matching pre-release is yielded immediately when the
// policy includes pre-releases, buffered when the policy is unresolved and the
// specifier does not forbid them, and dropped otherwise. The buffer is only
// released if nothing was yielded directly — that is the "offer a pre-release
// when there is nothing else" rule, and it is why Filter's answer for one
// item can differ from its answer for that item alongside a final release.
func (s Specifier) filterKeep(items []string, fc filterConf) []bool {
	keep := make([]bool, len(items))

	// A zero-value Specifier has no operator function and constrains nothing, so
	// it offers everything -- including a string that is not a PEP 440 version.
	//
	// ⚠️ Without this, the two readings of "constrains nothing" disagreed on
	// exactly one input class: Specifier{}.Contains("1.0") was true (Check's nil
	// guard) while Specifier{}.Contains("lolwat") was false, because the
	// unparseable branch below admits a non-version only for "===". Meanwhile
	// NewSpecifiers("").Contains("lolwat") is true. Three spellings of the same
	// idea have to give the same answer.
	if s.fn == nil {
		for i := range keep {
			keep[i] = true
		}
		return keep
	}

	// A per-call policy wins over the one the specifier carries; otherwise the
	// specifier's resolved policy decides.
	effective := fc.preReleases
	if effective == PreReleasesAuto {
		effective = s.PreReleases()
	}
	includePre := effective == PreReleasesInclude

	// callerUnset and specForbids gate the buffer, mirroring upstream's
	// `prereleases is None and self._prereleases is not False`. Note that
	// specForbids reads the RAW policy the specifier was built with, not the
	// resolved one: an autodetected Exclude still buffers, an explicit one
	// does not.
	callerUnset := fc.preReleases == PreReleasesAuto
	specForbids := s.pre == PreReleasesExclude

	var buffered []int
	foundOffered := false

	for i, item := range items {
		v, err := Parse(item)
		if err != nil {
			// Not a PEP 440 version. Only arbitrary equality can match it, and
			// when it does it is offered unconditionally: there is no way to
			// tell whether an opaque token is a pre-release, so the
			// pre-release rules do not apply to it at all.
			if s.op == "===" && strings.EqualFold(item, s.operand) {
				keep[i] = true
			}
			continue
		}

		var match bool
		if s.op == "===" {
			match = strings.EqualFold(item, s.operand)
		} else {
			match = s.Check(v)
		}
		if !match {
			continue
		}

		switch {
		case !v.IsPreRelease() || includePre:
			foundOffered = true
			keep[i] = true
		case callerUnset && !specForbids:
			buffered = append(buffered, i)
		}
	}

	if !foundOffered && callerUnset && !specForbids {
		for _, i := range buffered {
			keep[i] = true
		}
	}

	return keep
}

// filterKeep implements upstream's SpecifierSet.filter.
func (ss Specifiers) filterKeep(items []string, fc filterConf) []bool {
	// An unset per-call policy falls back to the set's resolved policy, which
	// may itself be unresolved.
	policy := fc.preReleases
	if policy == PreReleasesAuto {
		policy = ss.PreReleases()
	}

	if ss.Len() == 0 {
		return ss.filterEmpty(items, policy)
	}

	// With the policy still unresolved, let every match through the
	// per-specifier pass and decide about pre-releases once, at the end, over
	// the versions that satisfied the WHOLE set. Deciding per specifier would
	// release a buffered pre-release that a later specifier rejects.
	inner := policy
	if inner == PreReleasesAuto {
		inner = PreReleasesInclude
	}

	matched := ss.matchAll(items, inner == PreReleasesExclude)

	if policy != PreReleasesAuto {
		return matched
	}
	return applyPEP440Preference(items, matched)
}

// matchAll keeps the items that satisfy the set's operators. Membership is
// OR-of-AND: an item passes if it satisfies every specifier of any one group.
// Upstream has a single group, so for a set built from a Python-style
// comma-separated string this is plain conjunction.
func (ss Specifiers) matchAll(items []string, excludePre bool) []bool {
	keep := make([]bool, len(items))
	for i, item := range items {
		v, err := Parse(item)
		if err != nil {
			// Only a set made entirely of matching `===` specifiers can hold
			// a non-version.
			keep[i] = ss.allArbitraryMatch(item)
			continue
		}
		if excludePre && v.IsPreRelease() {
			continue
		}
		for _, group := range ss.specifiers {
			if groupMatches(group, item, v) {
				keep[i] = true
				break
			}
		}
	}
	return keep
}

func groupMatches(group []Specifier, item string, v Version) bool {
	for _, s := range group {
		var ok bool
		if s.op == "===" {
			ok = strings.EqualFold(item, s.operand)
		} else {
			ok = s.Check(v)
		}
		if !ok {
			return false
		}
	}
	return true
}

func (ss Specifiers) allArbitraryMatch(item string) bool {
	for _, group := range ss.specifiers {
		all := len(group) > 0
		for _, s := range group {
			if s.op != "===" || !strings.EqualFold(item, s.operand) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

// filterEmpty is the empty-set arm of SpecifierSet.filter. An empty set
// constrains nothing, so the pre-release policy is the only thing left to
// apply.
func (ss Specifiers) filterEmpty(items []string, policy PreReleases) []bool {
	keep := make([]bool, len(items))
	switch policy {
	case PreReleasesInclude:
		for i := range keep {
			keep[i] = true
		}
		return keep
	case PreReleasesExclude:
		for i, item := range items {
			v, err := Parse(item)
			keep[i] = err != nil || !v.IsPreRelease()
		}
		return keep
	default:
		for i := range keep {
			keep[i] = true
		}
		return applyPEP440Preference(items, keep)
	}
}

// applyPEP440Preference is upstream's _pep440_filter_prereleases: of the
// candidates that already matched, offer the final releases; offer the
// pre-releases only if there are no final releases at all.
//
// Items that are not parseable versions have already passed every specifier
// (only `===` can admit them) and cannot be classified as pre-release or not,
// so they are always offered.
func applyPEP440Preference(items []string, matched []bool) []bool {
	keep := make([]bool, len(items))
	var nonFinal []int
	foundFinal := false

	for i, item := range items {
		if !matched[i] {
			continue
		}
		v, err := Parse(item)
		if err != nil {
			keep[i] = true
			continue
		}
		if !v.IsPreRelease() {
			foundFinal = true
			keep[i] = true
			continue
		}
		nonFinal = append(nonFinal, i)
	}

	if !foundFinal {
		for _, i := range nonFinal {
			keep[i] = true
		}
	}
	return keep
}

// Compile-time proof that both types implement the filter shape FilterBy
// accepts, so a signature change cannot silently exclude one of them.
var (
	_ Filterer = Specifier{}
	_ Filterer = Specifiers{}
)
