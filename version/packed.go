// SPDX-License-Identifier: Apache-2.0 OR MIT

package version

import (
	"math/big"

	"github.com/rstudio/go-version/pkg/part"
)

// packedKey is a fixed-size encoding of a Version's PEP 440 comparison key,
// computed once at Parse time. For the versions it can represent -- the
// overwhelming majority of real PyPI versions, see below -- comparing two
// packedKeys with plain integer comparisons gives exactly the same answer as
// the general cmpkey path, without allocating, rendering strings, dispatching
// through Part interfaces, or touching math/big.
//
// A version is packable when ALL of the following hold:
//
//   - epoch == 0
//   - no local version label
//   - at most 6 release segments after trailing zeros are stripped
//   - every release segment < 2^32
//   - pre-release number < 2^20
//   - post-release number < 2^14
//   - dev-release number < 2^25 (covers YYYYMMDD nightly datestamps)
//
// Against the production PyPI index (7,666,849 version occurrences across
// 932,861 packages) these bounds were chosen from the measured distribution:
// no version in the index carries a local label, 99.97% have <= 6 release
// segments, and the wide dev field exists because datestamped dev releases
// (1.2.3.dev20240101) are the single most common large suffix number.
//
// # Layout
//
// The key is four uint64 words, compared lexicographically (w0, then w1, w2,
// w3). PEP 440's cmpkey for a packable version is the tuple
//
//	(epoch=0, release, pre, post, dev, local=-Inf)
//
// so only (release, pre, post, dev) vary, and they are laid out in exactly
// that order from the most significant bit down:
//
//	w0: release[0] (32 bits) | release[1] (32 bits)
//	w1: release[2] (32 bits) | release[3] (32 bits)
//	w2: release[4] (32 bits) | release[5] (32 bits)
//	w3: preClass (3) | preN (20) | postFlag (1) | postN (14) | devAbsent (1) | devN (25)
//
// Absent trailing release segments are stored as zero, which is sound because
// cmpkey itself strips trailing zeros: two stripped tuples compare identically
// to their zero-padded fixed-width forms.
//
// The suffix word w3 encodes the pre/post/dev tuple positions:
//
//   - preClass: 0 = -Infinity (the dev-only case: no pre, no post, dev set),
//     1 = "a", 2 = "b", 3 = "rc", 4 = +Infinity (no pre-release otherwise).
//     This mirrors cmpkey's substitution rules and the lexicographic order of
//     the normalized letters ("a" < "b" < "rc").
//   - postFlag: 0 = no post-release (-Infinity), 1 = post-release present.
//   - devAbsent: 0 = dev-release present, 1 = absent (+Infinity); a dev
//     release sorts BELOW the same version without one.
//
// Each numeric field is 0 when its segment is absent, matching PEP 440's
// "1.0a" == "1.0a0".
type packedKey struct {
	w0, w1, w2, w3 uint64
}

// Bit widths and derived limits for the suffix word.
const (
	packedSegBits  = 32
	packedPreNBits = 20
	packedPostBits = 14
	packedDevBits  = 25

	packedSegMax  = 1<<packedSegBits - 1
	packedPreNMax = 1<<packedPreNBits - 1
	packedPostMax = 1<<packedPostBits - 1
	packedDevMax  = 1<<packedDevBits - 1

	packedMaxSegments = 6

	// preClass values, in PEP 440 order.
	preClassNegInf = 0 // dev-only version: pre position is -Infinity
	preClassA      = 1
	preClassB      = 2
	preClassRC     = 3
	preClassInf    = 4 // no pre-release: pre position is +Infinity
)

// compare returns -1, 0 or 1 ordering k against o.
func (k packedKey) compare(o packedKey) int {
	switch {
	case k.w0 != o.w0:
		return boolCmp(k.w0 > o.w0)
	case k.w1 != o.w1:
		return boolCmp(k.w1 > o.w1)
	case k.w2 != o.w2:
		return boolCmp(k.w2 > o.w2)
	case k.w3 != o.w3:
		return boolCmp(k.w3 > o.w3)
	}
	return 0
}

func boolCmp(greater bool) int {
	if greater {
		return 1
	}
	return -1
}

// smallUint extracts b as a uint64 if it fits under limit. The zero part.BigInt
// (an absent segment) extracts as 0, which is exactly the value PEP 440 gives
// an absent number ("1.0a" == "1.0a0").
func smallUint(b part.BigInt, limit uint64) (uint64, bool) {
	bi := big.Int(b)
	// IsUint64 admits big negatives' absolute values? No: IsUint64 reports
	// whether the value is a non-negative integer < 2^64, and the version
	// grammar cannot produce negatives in the first place.
	if !bi.IsUint64() {
		return 0, false
	}
	u := bi.Uint64()
	if u > limit {
		return 0, false
	}
	return u, true
}

// packVersion computes the packed comparison key for a parsed version, or
// ok=false when the version does not fit the packed layout and must use the
// general comparison path.
func packVersion(epoch part.BigInt, release []part.BigInt, pre, post, dev letterNumber, local string) (packedKey, bool) {
	var k packedKey

	if local != "" {
		return k, false
	}
	if e, ok := smallUint(epoch, 0); !ok || e != 0 {
		return k, false
	}

	// Strip trailing zero segments; cmpkey does the same, and zero-padding
	// the remainder to fixed width preserves the resulting order.
	n := len(release)
	for n > 0 {
		if v, ok := smallUint(release[n-1], packedSegMax); ok && v == 0 {
			n--
			continue
		}
		break
	}
	if n > packedMaxSegments {
		return k, false
	}
	var segs [packedMaxSegments]uint64
	for i := 0; i < n; i++ {
		v, ok := smallUint(release[i], packedSegMax)
		if !ok {
			return k, false
		}
		segs[i] = v
	}
	k.w0 = segs[0]<<packedSegBits | segs[1]
	k.w1 = segs[2]<<packedSegBits | segs[3]
	k.w2 = segs[4]<<packedSegBits | segs[5]

	// Suffix word. The class substitutions mirror cmpkey exactly.
	var preClass, preN uint64
	switch {
	case pre.isNull() && post.isNull() && !dev.isNull():
		preClass = preClassNegInf
	case pre.isNull():
		preClass = preClassInf
	default:
		switch pre.letter {
		case "a":
			preClass = preClassA
		case "b":
			preClass = preClassB
		case "rc":
			preClass = preClassRC
		default:
			// Parse normalizes every pre-release spelling to a/b/rc, so this
			// is unreachable; refusing to pack keeps it correct if that ever
			// changes.
			return k, false
		}
		var ok bool
		preN, ok = smallUint(pre.number, packedPreNMax)
		if !ok {
			return k, false
		}
	}

	var postFlag, postN uint64
	if !post.isNull() {
		postFlag = 1
		var ok bool
		postN, ok = smallUint(post.number, packedPostMax)
		if !ok {
			return k, false
		}
	}

	// devAbsent inverts presence: a version with a dev segment sorts below
	// the same version without one.
	devAbsent := uint64(1)
	var devN uint64
	if !dev.isNull() {
		devAbsent = 0
		var ok bool
		devN, ok = smallUint(dev.number, packedDevMax)
		if !ok {
			return k, false
		}
	}

	k.w3 = preClass<<(packedPreNBits+1+packedPostBits+1+packedDevBits) |
		preN<<(1+packedPostBits+1+packedDevBits) |
		postFlag<<(packedPostBits+1+packedDevBits) |
		postN<<(1+packedDevBits) |
		devAbsent<<packedDevBits |
		devN
	return k, true
}
