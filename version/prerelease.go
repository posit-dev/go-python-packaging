// SPDX-License-Identifier: Apache-2.0 OR MIT

package version

// PreReleases is a three-state pre-release policy, matching the
// `prereleases: bool | None` parameter that pypa/packaging threads through
// `Specifier` and `SpecifierSet`.
//
// # Why three states and not a bool
//
// A bool can say "include pre-releases" and "exclude pre-releases". It cannot
// say "nobody has expressed a preference, so derive one from the specifier
// itself", which is what PEP 440 actually prescribes and what upstream's
// default (`None`) means. All three states behave differently, so collapsing
// `None` into `false` silently answers a question the caller never asked.
//
// # ⚠️ This policy governs SELECTION, not MATCHING
//
// PEP 440's pre-release rule is about which candidates an installer should
// *offer*, not about which versions a specifier *matches*. A pre-release does
// satisfy `>=1.0` — `Specifier(">=1.0").contains("1.5a1")` is true in
// pypa/packaging 26.2, verified — it is merely passed over while a final
// release is also available:
//
//	Filter(">=1.0", ["1.5a1"])          -> ["1.5a1"]   (nothing else on offer)
//	Filter(">=1.0", ["1.5a1", "2.0"])   -> ["2.0"]     (a final release wins)
//
// Conflating the two produces real bugs in both directions: an over-strict
// reading drops a pre-release that is the only candidate, and an over-loose
// reading defeats the operator-level guards that are *not* part of this policy
// at all. Those guards live in the comparison operators — `<2` never matches
// `2.0.dev1` and `>2` never matches `2.0.post1`, with any pre-release policy —
// and no value of PreReleases turns them off.
type PreReleases int8

const (
	// PreReleasesAuto derives the policy from the specifier, per PEP 440. It
	// is the zero value, so a Specifier or Specifiers built without an
	// explicit policy autodetects, matching upstream's `prereleases=None`
	// default. See Specifier.PreReleases for the derivation.
	PreReleasesAuto PreReleases = iota

	// PreReleasesInclude offers pre-releases alongside final releases,
	// matching upstream's `prereleases=True`.
	PreReleasesInclude

	// PreReleasesExclude never offers pre-releases, even when no final
	// release is available, matching upstream's `prereleases=False`.
	PreReleasesExclude
)

// String renders the policy using upstream's spelling of the corresponding
// Python value, so a test failure message lines up with the reference
// implementation's parameter.
func (p PreReleases) String() string {
	switch p {
	case PreReleasesInclude:
		return "true"
	case PreReleasesExclude:
		return "false"
	case PreReleasesAuto:
		return "none"
	default:
		return "unknown"
	}
}

// PreReleaseOption carries a PreReleases policy. It satisfies both
// SpecifierOption (so a policy can be attached when a specifier is built) and
// FilterOption (so a policy can override that one for a single Contains or
// Filter call), which is the same dual role upstream gives its `prereleases`
// parameter on both the constructor and the query methods.
type PreReleaseOption struct{ policy PreReleases }

// WithPreReleases sets the three-state pre-release policy. It may be passed
// to NewSpecifier / NewSpecifiers as a SpecifierOption, or to Contains /
// Filter as a per-call FilterOption that takes precedence over the policy the
// specifier was built with.
func WithPreReleases(p PreReleases) PreReleaseOption {
	return PreReleaseOption{policy: p}
}

func (o PreReleaseOption) apply(c *conf) { c.preReleases = o.policy }

func (o PreReleaseOption) applyFilter(fc *filterConf) { fc.preReleases = o.policy }

// FilterOption configures a single Contains or Filter call.
type FilterOption interface {
	applyFilter(*filterConf)
}

type filterConf struct {
	preReleases PreReleases
}

func newFilterConf(opts []FilterOption) filterConf {
	var fc filterConf
	for _, o := range opts {
		o.applyFilter(&fc)
	}
	return fc
}
