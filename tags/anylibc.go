// SPDX-License-Identifier: Apache-2.0 OR MIT

package tags

import "fmt"

// CompileAnyLibc compiles a linux Target for BOTH libc families and returns a
// single Matcher accepting either, with the glibc tiers ranked ahead of the musl
// ones. t.Libc is ignored; t.LibcMajor/LibcMinor are used as the glibc version
// and glibcToMusl maps them to a musl version.
//
// Compile requires a Libc and linuxPlatformTags emits manylinux XOR musllinux
// depending on it, so a caller expressing "linux x86_64, either libc" otherwise
// has to compile two Targets and union them by hand. Omitting the musl one is
// silent: every musllinux wheel simply stops matching, with nothing to indicate
// a whole wheel family was dropped. That is the mistake this exists to remove.
//
// Returns ErrUnsupportedTarget for a non-linux Target, since no other OS has a
// libc axis to be agnostic about.
func (t Target) CompileAnyLibc() (*Matcher, error) {
	if t.OS != "linux" {
		return nil, fmt.Errorf("%w: CompileAnyLibc requires OS \"linux\", got %q",
			ErrUnsupportedTarget, t.OS)
	}

	glibc := t
	glibc.Libc = "glibc"
	gm, err := glibc.Compile()
	if err != nil {
		return nil, err
	}

	musl := t
	musl.Libc = "musl"
	musl.LibcMajor, musl.LibcMinor = glibcToMusl(t.LibcMajor, t.LibcMinor)
	mm, err := musl.Compile()
	if err != nil {
		return nil, err
	}

	// Union preserving glibc order first, then musl tags not already present.
	// The overlap is real and must not be double-counted: the bare "linux_<arch>"
	// tag and the whole compatible tier (py3-none-any and friends) are emitted by
	// both families.
	ordered := gm.Tags()
	seen := make(map[Tag]struct{}, len(ordered))
	for _, tag := range ordered {
		seen[tag] = struct{}{}
	}
	for _, tag := range mm.Tags() {
		if _, dup := seen[tag]; dup {
			continue
		}
		seen[tag] = struct{}{}
		ordered = append(ordered, tag)
	}

	rank := make(map[Tag]int, len(ordered))
	abis := make(map[[2]string]struct{})
	for i, tag := range ordered {
		if _, exists := rank[tag]; !exists {
			rank[tag] = i
		}
		abis[[2]string{tag.Interpreter, tag.ABI}] = struct{}{}
	}

	// target keeps the glibc version so IsCompatibleOrNewer compares manylinux
	// tags against it, and muslMajor/Minor carry the musl version separately so a
	// musllinux tag is compared against the right floor. Collapsing the two would
	// make musllinux_2_0 read as older than glibc 2.28.
	return &Matcher{
		tags: ordered, rank: rank, target: glibc, abis: abis,
		anyLibc:   true,
		muslMajor: musl.LibcMajor, muslMinor: musl.LibcMinor,
	}, nil
}

// muslLatest is the newest musl version this package generates tags for. musl
// has no published glibc-equivalence table, and its 1.x series moves slowly
// enough that a single current value is a better answer than a fabricated
// mapping: musllinux tags are floors, so the newest value accepts every older
// musllinux tag as well.
var muslLatest = struct{ major, minor int }{1, 2}

// glibcToMusl chooses the musl version to pair with a declared glibc version.
// There is no meaningful correspondence between the two, so this does not try to
// invent one: it returns the newest musl version, which accepts the widest set
// of musllinux tags. A caller that needs an exact musl floor should compile that
// Target itself rather than use CompileAnyLibc.
func glibcToMusl(_, _ int) (major, minor int) {
	return muslLatest.major, muslLatest.minor
}
