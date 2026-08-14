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

// Bit widths for every w3 field, and shifts DERIVED from them so that a
// width change cannot silently desynchronize the layout. The compile-time
// guards below pin the widths to exactly 64 bits: widening any field without
// shrinking another stops the build instead of quietly shifting preClass off
// the top of the word (4<<62 == 0 for a uint64, which would collapse "no
// pre-release" into the dev-only -Infinity class).
const (
	packedSegBits      = 32
	packedPreClassBits = 3
	packedPreNBits     = 20
	packedPostFlagBits = 1
	packedPostBits     = 14
	packedDevFlagBits  = 1
	packedDevBits      = 25

	// w3 shifts, least significant field first.
	packedDevNShift     = 0
	packedDevFlagShift  = packedDevNShift + packedDevBits
	packedPostNShift    = packedDevFlagShift + packedDevFlagBits
	packedPostFlagShift = packedPostNShift + packedPostBits
	packedPreNShift     = packedPostFlagShift + packedPostFlagBits
	packedPreClassShift = packedPreNShift + packedPreNBits

	packedSegMax  = 1<<packedSegBits - 1
	packedPreNMax = 1<<packedPreNBits - 1
	packedPostMax = 1<<packedPostBits - 1
	packedDevMax  = 1<<packedDevBits - 1

	// ⚠️ packedMaxSegments is NOT a tunable knob: the release-word assembly
	// below is hand-unrolled for exactly six segments (three words of two),
	// so raising this constant alone would admit 7+ segment versions while
	// silently dropping everything past segs[5]. The guards below pin it.
	packedMaxSegments = 6

	// preClass values, in PEP 440 order.
	preClassNegInf = 0 // dev-only version: pre position is -Infinity
	preClassA      = 1
	preClassB      = 2
	preClassRC     = 3
	preClassInf    = 4 // no pre-release: pre position is +Infinity
)

// Compile-time layout guards: a negative array length is a compile error, so
// each pair of complementary lengths forces exact equality.
var (
	// The w3 fields fill the word exactly.
	_ [64 - (packedPreClassShift + packedPreClassBits)]struct{}
	_ [(packedPreClassShift + packedPreClassBits) - 64]struct{}
	// The word assembly in packVersion is unrolled for exactly 6 segments.
	_ [packedMaxSegments - 6]struct{}
	_ [6 - packedMaxSegments]struct{}
	// Every preClass value fits its field.
	_ [(1 << packedPreClassBits) - 1 - preClassInf]struct{}
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
	// The packed layout has no epoch field, so only epoch 0 may pack. This is
	// deliberately a direct sign check rather than smallUint(epoch, 0): with a
	// limit of 0 the value clause of that idiom is unreachable, and it would
	// read as if something other than the limit were rejecting "1!".
	if bi := big.Int(epoch); bi.Sign() != 0 {
		return k, false
	}
	// Parse never produces an empty release -- the grammar requires [0-9]+ --
	// so an empty one means a zero-value Version, which must sort BELOW every
	// real version (see Compare). Packing it would give it exactly version
	// "0"'s key. Today no zero-value Version reaches here (Parse is the only
	// constructor that packs), but the invariant belongs to the packer, not
	// to whoever calls it next.
	if len(release) == 0 {
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

	k.w3 = preClass<<packedPreClassShift |
		preN<<packedPreNShift |
		postFlag<<packedPostFlagShift |
		postN<<packedPostNShift |
		devAbsent<<packedDevFlagShift |
		devN<<packedDevNShift
	return k, true
}
