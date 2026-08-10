// SPDX-License-Identifier: Apache-2.0 OR MIT

package version

type conf struct {
	preReleases PreReleases
}

type SpecifierOption interface {
	apply(*conf)
}

// WithPreRelease is the original two-state pre-release switch.
//
// Deprecated: use WithPreReleases, which can express all three PEP 440 states.
// WithPreRelease(true) maps to PreReleasesInclude and WithPreRelease(false) to
// PreReleasesAuto — false has never meant "exclude" here, it meant "no
// override", which is what PreReleasesAuto spells out.
//
// ⚠️ WithPreRelease(true) also used to suppress the operator-level pre-release
// guards, by setting a flag that made Version.IsPreRelease report false for a
// version that plainly is one. That conflated candidate SELECTION with
// MATCHING: it made `<2` match `2.0.dev1`, which pypa/packaging 26.2 rejects
// with `prereleases=True` just as it does without it (verified against the
// reference implementation). The flag is gone, so `<2` no longer matches
// `2.0.dev1` under this option. See the PreReleases doc comment.
type WithPreRelease bool

func (o WithPreRelease) apply(c *conf) {
	if bool(o) {
		c.preReleases = PreReleasesInclude
		return
	}
	c.preReleases = PreReleasesAuto
}
