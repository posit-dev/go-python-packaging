// SPDX-License-Identifier: Apache-2.0 OR MIT

package version

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/rstudio/go-version/pkg/part"
)

var (
	// The compiled regular expression used to test the validity of a version.
	versionRegex *regexp.Regexp

	// https://github.com/pypa/packaging/blob/a6407e3a7e19bd979e93f58cfc7f6641a7378c46/packaging/version.py#L459-L464
	preReleaseAliases = map[string]string{
		"a":       "a",
		"alpha":   "a",
		"b":       "b",
		"beta":    "b",
		"rc":      "rc",
		"c":       "rc",
		"pre":     "rc",
		"preview": "rc",
	}

	// https://github.com/pypa/packaging/blob/a6407e3a7e19bd979e93f58cfc7f6641a7378c46/packaging/version.py#L465-L466
	postReleaseAliases = map[string]string{
		"post": "post",
		"rev":  "post",
		"r":    "post",
	}
)

const (
	// The raw regular expression string used for testing the validity of a version.
	regex = `v?` +
		`(?:` +
		`(?:(?P<epoch>[0-9]+)!)?` + // epoch
		`(?P<release>[0-9]+(?:\.[0-9]+)*)` + // release segment
		// The pre-release spellings MUST be ordered so that no alternative is
		// preceded by one of its own prefixes: Go's regexp is leftmost-first,
		// so listing "a" before "alpha" makes "alpha" match as just "a",
		// truncating the version mid-token. pypa/packaging orders its
		// equivalent alternation the same way, for the same reason.
		`(?P<pre>[-_\.]?(?P<pre_l>(alpha|a|beta|b|preview|pre|c|rc))[-_\.]?(?P<pre_n>[0-9]+)?)?` + // pre-release
		`(?P<post>(?:-(?P<post_n1>[0-9]+))|(?:[-_\.]?(?P<post_l>post|rev|r)[-_\.]?(?P<post_n2>[0-9]+)?))?` + // post release
		`(?P<dev>[-_\.]?(?P<dev_l>dev)[-_\.]?(?P<dev_n>[0-9]+)?)?)` + // dev release
		`(?:\+(?P<local>[a-z0-9]+(?:[-_\.][a-z0-9]+)*))?` // local version`
)

// Version represents a single version.
type Version struct {
	epoch    part.BigInt
	release  []part.BigInt
	pre      letterNumber
	post     letterNumber
	dev      letterNumber
	local    string
	key      key
	original string

	// packed is a fixed-size integer encoding of key, valid when packable is
	// true. See packed.go. Compare uses it to order two packable versions
	// with a handful of integer comparisons instead of the allocating
	// interface-driven path through key.
	packed   packedKey
	packable bool
}

type key struct {
	epoch   part.BigInt
	release part.Parts
	pre     part.Part
	post    part.Part
	dev     part.Part
	local   part.Part
}

func (k key) compare(o key) int {
	p1 := part.Parts{k.epoch, k.release, k.pre, k.post, k.dev, k.local}
	p2 := part.Parts{o.epoch, o.release, o.pre, o.post, o.dev, o.local}
	return p1.Compare(p2)
}

type letterNumber struct {
	letter part.String
	number part.BigInt
}

func (ln letterNumber) isNull() bool {
	return ln.letter.IsNull() && ln.number.IsNull()
}

func init() {
	// The surrounding-whitespace class must include the vertical tab. Go's
	// \s is [\t\n\f\r ] and excludes \v, while Python's re \s is
	// [ \t\n\r\f\v] and includes it -- so anchoring with a bare \s* would
	// reject "1.0\v", which pypa/packaging accepts.
	versionRegex = regexp.MustCompile(`(?i)^[\s\v]*` + regex + `[\s\v]*$`)
}

// localVersionSeparators matches the separators PEP 440 permits inside a local
// version label. It mirrors pypa/packaging's _local_version_separators.
var localVersionSeparators = regexp.MustCompile(`[._-]`)

// normalizeLocal normalizes a local version label: PEP 440 requires "-" and
// "_" separators to be rewritten as ".", the label to be lowercased, and any
// segment consisting entirely of ASCII digits to be normalized as an integer.
//
// Normalizing the separator is not cosmetic. cmpkey splits the label on "."
// to compare it segment by segment, with numeric segments ordering above
// alphabetic ones, so an un-normalized "ubuntu-2" is treated as a single
// alphabetic segment and compared lexicographically. That makes
// "1.0+ubuntu-10" sort BELOW "1.0+ubuntu-2" -- a silent wrong answer of the
// same kind as rstudio/package-manager#19369.
//
// # Why all-digit segments have their leading zeros stripped
//
// PEP 440: "If a segment consists entirely of ASCII digits then that section
// should be considered an integer" — and its integer-normalization rule ("an
// integer version of 00 would normalize to 0") therefore reaches it. The rule's
// one carve-out is scoped to integers inside an ALPHANUMERIC local segment,
// "such as 1.0+foo0100 which is already in its normalized form", so "foo0100"
// is left exactly as it is while "007" becomes "7".
//
// That asymmetry is easy to get backwards, so note what it is NOT about:
// ordering was already correct without this, because cmpkey compares all-digit
// segments numerically either way. What it changes is the normalized STRING —
// and PEP 440 defines the "===" arbitrary-equality operator as string equality
// on that form, so without this "===1.0+7" fails to match "1.0+007" while the
// reference implementation matches it.
func normalizeLocal(local string) string {
	lowered := localVersionSeparators.ReplaceAllString(strings.ToLower(local), ".")

	segments := strings.Split(lowered, ".")
	for i, seg := range segments {
		if isASCIIDigits(seg) {
			segments[i] = strings.TrimLeft(seg, "0")
			if segments[i] == "" {
				// The segment was all zeros, which is the integer 0.
				segments[i] = "0"
			}
		}
	}
	return strings.Join(segments, ".")
}

// isASCIIDigits reports whether s is non-empty and every byte is an ASCII digit.
//
// Deliberately not unicode.IsDigit: PEP 440 says "entirely of ASCII digits", and
// admitting other Unicode digit forms here would accept labels the grammar
// rejects and hand strconv-shaped input to a numeric comparison.
func isASCIIDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// MustParse is like Parse but panics if the version cannot be parsed.
func MustParse(v string) Version {
	ver, err := Parse(v)
	if err != nil {
		panic(err)
	}
	return ver
}

// Parse parses the given version and returns a new Version.
func Parse(v string) (Version, error) {
	matches := versionRegex.FindStringSubmatch(v)
	if matches == nil {
		return Version{}, fmt.Errorf("malformed version: %s", v)
	}

	var epoch, preN, postN, devN part.BigInt
	var preL, postL, devL part.String
	var release []part.BigInt
	var local string
	var err error

	for i, name := range versionRegex.SubexpNames() {
		m := matches[i]
		if m == "" {
			continue
		}

		switch name {
		case "epoch":
			epoch, err = part.NewBigInt(m)
		case "release":
			for _, str := range strings.Split(m, ".") {
				val, err := part.NewBigInt(str)
				if err != nil {
					return Version{}, fmt.Errorf("error parsing version: %w", err)
				}

				release = append(release, val)
			}
		case "pre_l":
			preL = part.String(preReleaseAliases[strings.ToLower(m)])
		case "pre_n":
			preN, err = part.NewBigInt(m)
		case "post_l":
			postL = part.String(postReleaseAliases[strings.ToLower(m)])
		case "post_n1", "post_n2":
			// https://github.com/pypa/packaging/blob/a6407e3a7e19bd979e93f58cfc7f6641a7378c46/packaging/version.py#L469-L472
			if postL == "" {
				postL = "post"
			}
			postN, err = part.NewBigInt(m)
		case "dev_l":
			devL = part.String(strings.ToLower(m))
		case "dev_n":
			devN, err = part.NewBigInt(m)
		case "local":
			local = normalizeLocal(m)
		}
		if err != nil {
			return Version{}, fmt.Errorf("failed to parse version (%s): %w", v, err)
		}
	}

	pre := letterNumber{
		letter: preL,
		number: preN,
	}
	post := letterNumber{
		letter: postL,
		number: postN,
	}
	dev := letterNumber{
		letter: devL,
		number: devN,
	}

	packed, packable := packVersion(epoch, release, pre, post, dev, local)

	return Version{
		epoch:    epoch,
		release:  release,
		pre:      pre,
		post:     post,
		dev:      dev,
		local:    local,
		key:      cmpkey(epoch, release, pre, post, dev, local),
		original: v,
		packed:   packed,
		packable: packable,
	}, nil
}

// ref. https://github.com/pypa/packaging/blob/a6407e3a7e19bd979e93f58cfc7f6641a7378c46/packaging/version.py#L495
func cmpkey(epoch part.BigInt, release []part.BigInt, pre, post, dev letterNumber, local string) key {
	// Set default values
	k := key{
		epoch: epoch,
		pre:   part.Parts{pre.letter, pre.number},
		post:  part.Parts{post.letter, post.number},
		dev:   part.Parts{dev.letter, dev.number},
		local: part.NegativeInfinity,
	}

	// Remove trailing zeros
	k.release = part.BigIntSliceToParts(release).Normalize()

	// https://github.com/pypa/packaging/blob/a6407e3a7e19bd979e93f58cfc7f6641a7378c46/packaging/version.py#L514-L517
	if pre.isNull() && post.isNull() && !dev.isNull() {
		k.pre = part.NegativeInfinity
	} else if pre.isNull() {
		k.pre = part.Infinity
	}

	// Versions without a post segment should sort before those with one.
	if post.isNull() {
		k.post = part.NegativeInfinity
	}

	// Versions without a development segment should sort after those with one.
	if dev.isNull() {
		k.dev = part.Infinity
	}

	// Versions with a local segment need that segment parsed to implement the sorting rules in PEP440.
	//   - Alpha numeric segments sort before numeric segments
	//   - Alpha numeric segments sort lexicographically
	//   - Numeric segments sort numerically
	//   - Shorter versions sort before longer versions when the prefixes match exactly
	if local != "" {
		var parts part.Parts
		for _, l := range strings.Split(local, ".") {
			if p, err := part.NewBigInt(l); err == nil {
				parts = append(parts, p)
			} else {
				parts = append(parts, part.NewPreString(l))
			}
		}
		k.local = parts
	}

	return k
}

// Compare compares this version to another version. This
// returns -1, 0, or 1 if this version is smaller, equal,
// or larger than the other version, respectively.
func (v Version) Compare(other Version) int {
	return compareVersions(&v, &other)
}

// compareVersions is Compare's whole implementation, on pointers. Version is
// a large struct (~392 bytes), so internal callers that already hold two
// Versions in a slice -- SortedVersions.Less above all, which sits inside
// sort's O(n log n) comparison loop -- go through this directly rather than
// copying both structs per comparison just to read a 33-byte packed key.
// It never writes through either pointer; that read-only property is what
// keeps a parsed Version safely shareable across goroutines (see padParts).
func compareVersions(v, other *Version) int {
	// The packed fast path: when both versions carry a packed key, a few
	// integer comparisons decide the whole ordering. The packed key is a
	// complete encoding of the comparison key for packable versions -- see
	// packed.go -- so this needs no fallback tie-break.
	if v.packable && other.packable {
		return v.packed.compare(other.packed)
	}

	// There is deliberately NO String()==String() equality fast path here.
	// It rendered both sides to fresh strings on every call -- two
	// bytes.Buffer round trips through fmt -- and with the packed path in
	// place it could only ever serve pairs where at least one side is
	// unpackable. For those pairs the key comparison below returns 0 for
	// exactly the same inputs, without the two allocations. Removing it was
	// measured alone (against go-pyresolver's resolver benchmark, production
	// snapshot) at 1.18-1.51x warm, and is pure simplification once the
	// packed key exists.

	// ⚠️ An uninitialized Version cannot go through key comparison at all. Its
	// key's pre/post/dev/local are NIL Part interfaces, and go-version's
	// Parts.IsAny ranges over the elements calling p.IsAny() with no nil check
	// (part/list.go:101), so merely asking whether the Parts is "any"
	// dereferences nil. That call sits at part/list.go:54, before any element
	// is compared, and only on the *argument* side -- which is exactly why
	// real.Compare(Version{}) crashed while Version{}.Compare(real) returned
	// -1 perfectly well. The asymmetry was the tell that this is not about
	// release lengths.
	//
	// Decide from the release segment instead. Parse cannot produce an empty
	// release, so an empty one means an uninitialized Version: it sorts below
	// every real version, and two of them are equal.
	switch {
	case len(v.release) == 0 && len(other.release) == 0:
		return 0
	case len(v.release) == 0:
		return -1
	case len(other.release) == 0:
		return 1
	}

	k1 := v.key
	k2 := other.key

	// Pad both release segments to the longer of the two, so an absent
	// trailing segment compares as zero and 1.2 equals 1.2.0.
	//
	// ⚠️ The second line padded k2 to its OWN length, which is a no-op, where
	// it means the longer of the two. For two parsed versions key.compare
	// tolerates the unequal lengths that left behind and still returns the
	// right answer -- verified across ten pairs with differing segment counts
	// in both directions -- so the defect was latent rather than wrong.
	//
	// It stopped being latent for a zero-value Version, whose release is nil:
	// real.Compare(Version{}) dereferenced a nil part and crashed while
	// Version{}.Compare(real) returned -1, because only the k1 side was ever
	// actually padded. That asymmetry is the tell.
	// ⚠️ padParts, not Parts.Padding. go-version v0.0.2's Parts.Normalize
	// reslices (ret = ret[:i]) leaving len < cap, and Parts.Padding appends
	// into that spare capacity IN PLACE -- so a by-value copy of a Version
	// shares a backing array with its original, and two goroutines comparing
	// copies of the same version race on it. Padding into a fresh slice makes
	// Compare read-only on both receivers, which is what lets a Version be
	// shared across goroutines.
	n := max(len(k1.release), len(k2.release))
	k1.release = padParts(k1.release, n)
	k2.release = padParts(k2.release, n)

	return k1.compare(k2)
}

// padParts returns parts extended with part.Zero to size elements, never
// writing through parts' own backing array. See the caller for why appending
// in place (Parts.Padding) is a data race.
func padParts(parts part.Parts, size int) part.Parts {
	if len(parts) >= size {
		return parts
	}
	padded := make(part.Parts, size)
	copy(padded, parts)
	for i := len(parts); i < size; i++ {
		padded[i] = part.Zero
	}
	return padded
}

// Equal tests if two versions are equal.
func (v Version) Equal(o Version) bool {
	return v.Compare(o) == 0
}

// GreaterThan tests if this version is greater than another version.
func (v Version) GreaterThan(o Version) bool {
	return v.Compare(o) > 0
}

// GreaterThanOrEqual tests if this version is greater than or equal to another version.
func (v Version) GreaterThanOrEqual(o Version) bool {
	return v.Compare(o) >= 0
}

// LessThan tests if this version is less than another version.
func (v Version) LessThan(o Version) bool {
	return v.Compare(o) < 0
}

// LessThanOrEqual tests if this version is less than or equal to another version.
func (v Version) LessThanOrEqual(o Version) bool {
	return v.Compare(o) <= 0
}

// String returns the full version string included pre-release
// and metadata information.
func (v Version) String() string {
	var buf bytes.Buffer

	// Epoch
	if v.epoch.Compare(part.Zero) == 1 {
		fmt.Fprintf(&buf, "%s!", v.epoch)
	}

	// Release segment
	//
	// ⚠️ A zero-value Version has no release segments, and indexing release[0]
	// unconditionally made every rendering path panic on one. Parse cannot
	// produce an empty release -- the grammar requires `[0-9]+` -- so an empty
	// one always means an uninitialized Version is being rendered, and the
	// empty string cannot collide with any version Parse accepts.
	//
	// A panicking String method is worse than most panics, because fmt
	// *recovers* it: `fmt.Errorf("%s", v)` yields a message containing
	// "%!s(PANIC=String method: ...)" and reports no error, so the bug is
	// swallowed at exactly the call sites most likely to hit it. A direct call
	// crashes instead. (Compare() historically called String() as its fast
	// path, which is how this one line once made all six comparison methods
	// panic as well; that fast path is gone, but the guard here still matters
	// for every direct rendering call.)
	writeRelease(&buf, v.release)

	// Pre-release
	if !v.pre.isNull() {
		fmt.Fprintf(&buf, "%s%s", v.pre.letter, v.pre.number)
	}

	// Post-release
	if !v.post.isNull() {
		fmt.Fprintf(&buf, ".post%s", v.post.number)
	}

	// Development release
	if !v.dev.isNull() {
		fmt.Fprintf(&buf, ".dev%s", v.dev.number)
	}

	// Local version segment
	if v.local != "" {
		fmt.Fprintf(&buf, "+%s", v.local)
	}

	return buf.String()
}

// BaseVersion returns the base version
func (v Version) BaseVersion() string {
	var buf bytes.Buffer

	// Epoch
	if v.epoch.Compare(part.Zero) == 1 {
		fmt.Fprintf(&buf, "%s!", v.epoch.String())
	}

	// Release segment. See String() for why an empty release is rendered as
	// nothing rather than indexed into.
	writeRelease(&buf, v.release)

	return buf.String()
}

// writeRelease renders a dot-joined release segment, or nothing at all when
// there are no segments. Shared by String and BaseVersion so the two cannot
// drift on the empty case, which is how only one of them getting a guard would
// leave the other panicking.
func writeRelease(buf *bytes.Buffer, release []part.BigInt) {
	for i, r := range release {
		if i > 0 {
			buf.WriteByte('.')
		}
		buf.WriteString(r.String())
	}
}

// Original returns the original parsed version as-is, including any
// potential whitespace, `v` prefix, etc.
func (v Version) Original() string {
	return v.original
}

// Local returns the local version
func (v Version) Local() string {
	return v.local
}

// Public returns the public version
func (v Version) Public() string {
	return strings.SplitN(v.String(), "+", 2)[0]
}

// IsPreRelease reports whether this is a pre-release: it has a pre-release
// segment (a1, b2, rc3) or a dev segment.
//
// ⚠️ This is a property of the version, and nothing else may override it. A
// pre-release policy (see PreReleases) decides whether a pre-release is
// *offered* as a candidate; it does not make a pre-release stop being one. An
// earlier version of this package let WithPreRelease(true) flip this method to
// false, which silently disabled the operator-level guards in the comparison
// functions below and made `<2` match `2.0.dev1` — a result pypa/packaging
// 26.2 rejects under every pre-release policy.
func (v Version) IsPreRelease() bool {
	return !v.pre.isNull() || !v.dev.isNull()
}

// postBase returns the version this one is a post-release of: the same version
// with its post, dev and local segments dropped. Ported from pypa/packaging
// 26.2's _post_base.
//
//	1.0.post1       -> 1.0
//	1.0a1.post0     -> 1.0a1
//	1.0.post0.dev1  -> 1.0
//
// It is derived by re-parsing the rendered prefix rather than by editing the
// struct, because the comparison key is precomputed at Parse time and a
// hand-edited copy would carry a key describing the version it used to be.
func (v Version) postBase() (Version, bool) {
	var buf bytes.Buffer
	if v.epoch.Compare(part.Zero) == 1 {
		fmt.Fprintf(&buf, "%s!", v.epoch)
	}
	writeRelease(&buf, v.release)
	if !v.pre.isNull() {
		fmt.Fprintf(&buf, "%s%s", v.pre.letter, v.pre.number)
	}
	return parseOperand(buf.String())
}

// earliestPreRelease returns the earliest pre-release of this version: the same
// version with dev set to 0 and the local segment dropped. Ported from
// pypa/packaging 26.2's _earliest_prerelease.
//
//	1.2         -> 1.2.dev0
//	1.2.post1   -> 1.2.post1.dev0
//
// It is the lower bound of "a pre-release of V", which is what PEP 440's "<V
// MUST NOT allow a pre-release of the specified version" actually means. See
// specifierLessThan.
func (v Version) earliestPreRelease() (Version, bool) {
	var buf bytes.Buffer
	if v.epoch.Compare(part.Zero) == 1 {
		fmt.Fprintf(&buf, "%s!", v.epoch)
	}
	writeRelease(&buf, v.release)
	if !v.pre.isNull() {
		fmt.Fprintf(&buf, "%s%s", v.pre.letter, v.pre.number)
	}
	if !v.post.isNull() {
		fmt.Fprintf(&buf, ".post%s", v.post.number)
	}
	buf.WriteString(".dev0")
	return parseOperand(buf.String())
}

// trimmedRelease renders the version like String, but with trailing zero
// components dropped from the release segment (always keeping at least one),
// which is the canonical form upstream's canonicalize_version produces via
// _TrimmedRelease. Used by Specifier equality.
func (v Version) trimmedRelease() string {
	trimmed := v
	rel := v.release
	i := len(rel)
	for i > 1 && rel[i-1].Compare(part.Zero) == 0 {
		i--
	}
	trimmed.release = rel[:i]
	return trimmed.String()
}

// IsPostRelease returns if it is a post-release
func (v Version) IsPostRelease() bool {
	return !v.post.isNull()
}

type SortedVersions []Version

func (s SortedVersions) Len() int {
	return len(s)
}
func (s SortedVersions) Less(i, j int) bool {
	// Compare through pointers: this sits inside sort's comparison loop, and
	// going through the value-receiver methods would copy four ~392-byte
	// structs per comparison to read 33 bytes of packed key.
	return compareVersions(&s[i], &s[j]) < 0
}
func (s SortedVersions) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
