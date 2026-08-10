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
// trailing zeros in the operand are not significant, and the operand's case
// and `v` prefix are normalized:
//
//	==2.8.0  ==  ==2.8      (true)
//	==1.0A1  ==  ==1.0a1    (true)
//	~=1.18.0 ==  ~=1.18     (FALSE — `~=` keeps trailing zeros, because they
//	                         change which versions it matches)
//
// A `===` operand and a `.*` wildcard are compared verbatim, since neither is
// a version that can be canonicalized.
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

// Equal reports whether two sets hold the same specifiers, compared
// canonically and irrespective of order, duplicates, or pre-release policy.
// It is upstream's SpecifierSet.__eq__.
func (ss Specifiers) Equal(other Specifiers) bool {
	left := canonicalKeys(ss)
	right := canonicalKeys(other)
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

// canonicalKeys is the deduplicated set of canonical specifier renderings,
// which is what makes Equal blind to order and duplicates.
func canonicalKeys(ss Specifiers) map[string]struct{} {
	keys := make(map[string]struct{}, ss.Len())
	for _, s := range ss.List() {
		op, operand := s.canonicalSpec()
		keys[op+operand] = struct{}{}
	}
	return keys
}

// And returns the intersection of two specifier sets: every specifier from
// both, which is upstream's `SpecifierSet.__and__`.
//
// The pre-release policies are combined rather than dropped. PreReleasesAuto
// on one side yields to the other; two identical explicit policies survive;
// Include on one side and Exclude on the other is a contradiction and returns
// ErrPreReleaseConflict, as upstream raises ValueError.
func (ss Specifiers) And(other Specifiers) (Specifiers, error) {
	pre, err := combinePreReleases(ss.conf.preReleases, other.conf.preReleases)
	if err != nil {
		return Specifiers{}, err
	}

	combined := append(ss.List(), other.List()...)
	// Re-stamp each member with the combined policy so a specifier read back
	// off the result answers PreReleases consistently with the set.
	for i := range combined {
		combined[i].pre = pre
	}

	return Specifiers{
		specifiers: [][]Specifier{combined},
		conf:       conf{preReleases: pre},
	}, nil
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
func NewSpecifiersFrom(specs []Specifier, opts ...SpecifierOption) Specifiers {
	c := new(conf)
	for _, o := range opts {
		o.apply(c)
	}
	group := make([]Specifier, len(specs))
	copy(group, specs)
	if c.preReleases != PreReleasesAuto {
		for i := range group {
			group[i].pre = c.preReleases
		}
	}
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
