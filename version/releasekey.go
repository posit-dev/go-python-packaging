// SPDX-License-Identifier: Apache-2.0 OR MIT

package version

import (
	"math/big"
	"strings"

	"github.com/rstudio/go-version/pkg/part"
)

// ReleaseKey is a version's position in PEP 440's RELEASE order: its epoch and
// its release segments with trailing zeros stripped, and nothing else.
//
// It is deliberately COARSER than the version order. 1.0, 1.0.0, 1.00,
// 1.0a1, 1.0.post3.dev2 and 1.0+ubuntu1 all carry the same ReleaseKey, because
// PEP 440 gives every one of them epoch 0 and, after stripping, the release
// tuple (1). A consumer that needs the full order wants Compare on the Version;
// this key exists for consumers that need to ORDER or PARTITION versions by
// release, where the pre/post/dev/local suffix must not participate.
//
// ⚠️ "Partition", not "index": a ReleaseKey is not comparable with == and so
// cannot be a map key. Use Compare — sort by it and scan the runs of equal
// keys. String() is a stable scalar form, but it allocates, which is the cost
// this type exists to avoid.
//
// # Why this is not just "read the release segments"
//
// The obvious way to get at a version's release from outside this package is
// BaseVersion(), which renders "1!3.4.5" — and rendering means a bytes.Buffer
// and one math/big decimal conversion per segment, followed by the caller
// splitting the result back apart. go-pyresolver's PEP 440 set algebra did
// exactly that, once per candidate version per containment test.
//
// A key derived from the parsed fields costs no rendering, no parsing and no
// allocation, and answers the same question: BenchmarkReleaseKeyVsBaseVersion-
// Split measures 16 ns and 0 allocations against 220 ns and 10 allocations on
// "2024.10.31". Deriving one costs more when the Version has to be copied out
// of a slice first (~24 ns), because the receiver is by value; see ReleaseKey().
//
// # Ordering
//
// Compare orders keys by epoch, then segment by segment, with the shorter
// release smaller when it is a prefix of the longer. That is PEP 440's release
// ordering, verified against pypa/packaging 26.2's comparison key — its second
// tuple element is precisely the stripped release this type holds.
//
// The order is exact at every magnitude. PEP 440 puts no ceiling on an epoch
// or a release segment (the grammar is `[0-9]+`), and datestamped and calendar
// versions push real segments high, so the general path compares arbitrary-
// precision integers rather than machine words.
//
// # Zero value
//
// The zero ReleaseKey is the key of the zero Version, whose release is empty.
// It compares EQUAL to the key of "0" and "0.0", because those strip to an
// empty release too. This is a deliberate difference from Version.Compare,
// which sorts an uninitialized Version strictly below every real version: that
// distinction lives in the full version order, not in the release grouping,
// and the release grouping genuinely cannot see it.
//
// # Aliasing
//
// A ReleaseKey shares the Version's release segments rather than copying them.
// It never writes to them, its capacity is clipped so an append cannot either,
// and reading a math/big value does not mutate it — so a key is safe to share
// across goroutines exactly as the Version it came from is.
//
// # ⚠️ A key is not a bound on its own group
//
// String() renders the shortest version string carrying the key, but that
// version is NOT the least version in the group: 1.0a1 and 1.0.dev0 carry the
// key of "1" and sort strictly BELOW it. Neither is there a greatest — .postN
// and +local extend the group upward without limit. So
//
//	lo, _ := Parse(k.String())   // ⚠️ NOT a lower bound on k's group
//
// silently excludes every pre-release and dev release of that release. A
// consumer bracketing a release group needs positions between versions, which
// is a different thing from a version; see go-pyresolver's pep440set for one
// treatment of it.
//
// ReleaseKey is NOT comparable with ==; it holds a slice. Use Compare.
type ReleaseKey struct {
	// w is the packed encoding of the stripped release, valid when packed is
	// true: six 32-bit segment fields across three words, most significant
	// segment first, laid out by packRelease. Two packed keys are ordered by
	// three integer comparisons.
	w      [3]uint64
	packed bool

	// epoch and release are the general path, used whenever either side of a
	// comparison did not pack. release is the stripped prefix of the Version's
	// own release slice.
	epoch   part.BigInt
	release []part.BigInt
}

// ReleaseKey returns v's position in the release order. See ReleaseKey.
//
// It allocates nothing: the epoch is a by-value copy of a math/big header and
// the release is a subslice of v's own.
//
// ⚠️ The receiver is by VALUE, so calling this copies the whole Version — a
// large struct, and this method does not inline. That is deliberate, for
// consistency with every other Version method and because a by-value receiver
// cannot write through to the caller's Version; but it means a caller ranging
// over a []Version pays ~50% more per key than one holding a single Version in
// a local. compareVersions takes pointers precisely to dodge that copy in
// sort's inner loop, and a caller with the same problem can hoist the Version
// out of the loop once.
func (v Version) ReleaseKey() ReleaseKey {
	n := releaseLen(v.release)
	k := ReleaseKey{
		epoch: v.epoch,
		// Three-index slicing clips the capacity to n. Without it the key
		// would carry the stripped trailing zeros as spare capacity, and an
		// append by any future holder of the key would write THROUGH into the
		// Version's own release segments. That is the same aliasing hazard
		// that made go-version's Parts.Padding a data race between two copies
		// of one Version; see padParts.
		release: v.release[:n:n],
	}
	// Only an epoch of zero can pack: the packed words hold release segments
	// and nothing else, so a nonzero epoch has nowhere to go. Reusing v.packed
	// when v.packable would save this call, but v.packable is false for
	// reasons that have nothing to do with the release (a local label, a large
	// dev number), and buying a few nanoseconds once per key with a standing
	// assumption about another key's layout is a bad trade.
	//
	// k.release rather than v.release: it is the same segments already
	// stripped, so packRelease's own strip finds nothing left to do instead of
	// rescanning the trailing zeros.
	if bi := big.Int(v.epoch); bi.Sign() == 0 {
		k.w, k.packed = packRelease(k.release)
	}
	return k
}

// Compare reports whether k is before (-1), at (0) or after (+1) o in the
// release order.
//
// Equality here means the two versions share a release group, NOT that they are
// the same version: ReleaseKey ignores the pre/post/dev/local suffix entirely.
func (k ReleaseKey) Compare(o ReleaseKey) int {
	if k.packed && o.packed {
		switch {
		case k.w[0] != o.w[0]:
			return boolCmp(k.w[0] > o.w[0])
		case k.w[1] != o.w[1]:
			return boolCmp(k.w[1] > o.w[1])
		case k.w[2] != o.w[2]:
			return boolCmp(k.w[2] > o.w[2])
		}
		return 0
	}
	return k.compareGeneral(o)
}

// compareGeneral is the arbitrary-precision path, taken whenever either side
// did not pack. It is also the reference the packed path is held to:
// TestReleaseKeyPackedAgreesWithGeneral runs both over every corpus pair, so a
// packing bug cannot hide behind the fast path.
func (k ReleaseKey) compareGeneral(o ReleaseKey) int {
	// (*big.Int)(&…) rather than big.Int(…): part.BigInt IS a big.Int, so the
	// pointer conversion is free, where the value conversion copies a header.
	// That matters HERE, in a per-segment loop; elsewhere in this file the
	// value conversion is used for a single Sign() or String() call, where one
	// header copy reads more plainly than a pointer conversion. Cmp only reads
	// through both pointers.
	if c := (*big.Int)(&k.epoch).Cmp((*big.Int)(&o.epoch)); c != 0 {
		return c
	}
	n := min(len(k.release), len(o.release))
	for i := range n {
		if c := (*big.Int)(&k.release[i]).Cmp((*big.Int)(&o.release[i])); c != 0 {
			return c
		}
	}
	// Both releases are stripped, so a shorter one is genuinely shorter and
	// its missing segments are zeros: it is the smaller release.
	switch {
	case len(k.release) < len(o.release):
		return -1
	case len(k.release) > len(o.release):
		return 1
	}
	return 0
}

// String renders the key in canonical shortest form: the stripped release,
// dot-joined, with an "N!" epoch prefix when the epoch is nonzero. The empty
// release renders as "0", so the result always parses and always round-trips —
// Parse(k.String()).ReleaseKey() compares equal to k.
//
// ⚠️ THE RESULT IS NOT A BOUND ON THE GROUP. It is the shortest spelling
// carrying the key, not the smallest version carrying it: 1.0a1 and 1.0.dev0
// carry the key of "1" and sort strictly below Parse("1"), and .postN and
// +local carry it and sort above with no upper limit. See the type doc.
//
// This exists so that a key appears legibly in a test failure or a %v; it is
// not on any hot path, and nothing in this package's ordering goes through it.
// It allocates, unlike everything else here.
func (k ReleaseKey) String() string {
	var b strings.Builder
	if bi := big.Int(k.epoch); bi.Sign() != 0 {
		b.WriteString(bi.String())
		b.WriteByte('!')
	}
	if len(k.release) == 0 {
		b.WriteByte('0')
		return b.String()
	}
	for i := range k.release {
		if i > 0 {
			b.WriteByte('.')
		}
		b.WriteString((*big.Int)(&k.release[i]).String())
	}
	return b.String()
}
