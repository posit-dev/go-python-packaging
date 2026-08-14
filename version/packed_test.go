// SPDX-License-Identifier: Apache-2.0 OR MIT

package version

import (
	"math/rand"

	"fmt"
	"github.com/rstudio/go-version/pkg/part"
	"sort"
	"sync"
	"testing"
)

// compareSlow forces the general (pre-packed-key) comparison path, so the
// packed path can be checked against it.
func compareSlow(a, b Version) int {
	a.packable = false
	b.packable = false
	return a.Compare(b)
}

// gridStrings is a structured grid covering every packed field, its limits,
// and the fallback boundary on both sides.
//
// ⚠️ This generator and the frozen fixture testdata/pypa-26.2-grid.ranked.gz
// must describe the same set of strings; TestPackedConformanceFixtures
// enforces that. If you extend the grid, regenerate the fixture (see that
// test's doc comment) with pypa/packaging 26.2 or newer.
func gridStrings() []string {
	// "0!" is an explicitly written zero epoch: it must pack (and order
	// identically to the same version without it). The two trailing-zero
	// releases below the 7-segment one exercise the strip-then-count order in
	// packVersion: more than 6 written segments that strip back into range.
	epochs := []string{"", "0!", "1!"}
	releases := []string{
		"0", "1", "1.0", "1.0.0.0", "1.00", "1.2", "1.2.1", "0.0.1",
		"2.31.0.20240406", "2024.1", "20240101.0", "1.2.3.4.5.6",
		"1.2.3.4.5.6.7", "1.2.3.4.5.6.0.0", "1.2.3.4.5.0.0",
		"4294967295.1", "4294967296.1", "1.4294967295",
		"1.4294967296", "99999999999999999999.0", "1.99999999999999999999",
	}
	pres := []string{"", "a", "a0", "a1", "b1", "rc1", "rc2", "pre1", "c3",
		"a1048575", "a1048576", "rc20240101"}
	posts := []string{"", ".post", ".post0", ".post1", "-1", ".post16383",
		".post16384", ".post20240101"}
	devs := []string{"", ".dev", ".dev0", ".dev1", ".dev20240101",
		".dev33554431", ".dev33554432"}
	locals := []string{"", "+abc", "+1", "+abc.1", "+ubuntu-2", "+007", "+abc.10", "+abc.2"}

	var out []string
	for _, e := range epochs {
		for _, r := range releases {
			for _, p := range pres {
				for _, po := range posts {
					for _, d := range devs {
						for _, l := range locals {
							out = append(out, e+r+p+po+d+l)
						}
					}
				}
			}
		}
	}
	return out
}

func gridVersions(tb testing.TB) []Version {
	strs := gridStrings()
	out := make([]Version, 0, len(strs))
	for _, s := range strs {
		v, err := Parse(s)
		if err != nil {
			tb.Fatalf("grid version %q failed to parse: %v", s, err)
		}
		out = append(out, v)
	}
	return out
}

// TestPackedAgreesWithSlowPath compares pairs from the grid through both
// paths and requires exact agreement.
//
// Only a PACKABLE x PACKABLE pair actually discriminates the two encodings:
// for any pair with an unpackable side, Compare and compareSlow run the same
// code and the assertion is x == x. So the sample is drawn packable-first --
// the packable subset carries the information -- topped up with unpackable
// versions to exercise the boundary and the mixed-pair fallback.
//
// Sampling is a seeded SHUFFLE, not a stride. The grid is a nested cross
// product, so any fixed stride whose gcd with an inner dimension's period
// exceeds 1 silently collapses that dimension: at 204,288 versions the old
// stride of 132 shared a factor of 4 with the 8-value locals dimension and
// sampled only 2 of its 8 values. A shuffle has no phase to lock onto.
func TestPackedAgreesWithSlowPath(t *testing.T) {
	full := gridVersions(t)
	rng := rand.New(rand.NewSource(1))
	rng.Shuffle(len(full), func(i, j int) { full[i], full[j] = full[j], full[i] })

	var packables, unpackables []Version
	for _, v := range full {
		if v.packable {
			packables = append(packables, v)
		} else {
			unpackables = append(unpackables, v)
		}
	}
	if len(packables) == 0 {
		t.Fatal("no packable versions in grid; the fast path is untested")
	}
	if len(unpackables) == 0 {
		t.Fatal("every grid version is packable; the fallback boundary is untested")
	}

	vs := append([]Version{}, packables[:min(1200, len(packables))]...)
	vs = append(vs, unpackables[:min(400, len(unpackables))]...)
	t.Logf("grid: %d versions (%d packable); sampled %d packable + %d unpackable",
		len(full), len(packables), min(1200, len(packables)), min(400, len(unpackables)))

	mismatches := 0
	for i := range vs {
		for j := range vs {
			got := vs[i].Compare(vs[j])
			want := compareSlow(vs[i], vs[j])
			if got != want {
				mismatches++
				if mismatches <= 20 {
					t.Errorf("Compare(%q, %q) = %d, slow path says %d",
						vs[i].Original(), vs[j].Original(), got, want)
				}
			}
		}
	}
	if mismatches > 0 {
		t.Fatalf("%d mismatching pairs", mismatches)
	}
}

// TestPackedLimits pins the exact packability boundary of every field.
func TestPackedLimits(t *testing.T) {
	cases := []struct {
		v    string
		want bool
	}{
		{"1.2.3", true},
		{"0", true},
		{"0!1.2.3", true},         // a WRITTEN zero epoch packs
		{"1.2.3.4.5.6.0.0", true}, // 8 written segments strip to 6
		{"1.2.3.4.5.0.0", true},   // 7 written segments strip to 5
		{"1.0a", true},
		{"1.0rc1", true},
		{"1.0.post1", true},
		{"1.0.dev20240101", true},
		{"1.2.3.4.5.6", true},
		{"1.2.3.4.5.6.7", false},          // 7 segments
		{"1.2.3.4.5.6.0", true},           // trailing zero strips to 6
		{"4294967295.0", true},            // 2^32-1 fits
		{"4294967296.0", false},           // 2^32 does not
		{"1!1.0", false},                  // epoch
		{"1.0+local", false},              // local
		{"1.0a1048575", true},             // preN 2^20-1
		{"1.0a1048576", false},            // preN 2^20
		{"1.0.post16383", true},           // postN 2^14-1
		{"1.0.post16384", false},          // postN 2^14
		{"1.0.dev33554431", true},         // devN 2^25-1
		{"1.0.dev33554432", false},        // devN 2^25
		{"99999999999999999999.0", false}, // > 2^64 segment
	}
	for _, c := range cases {
		v := MustParse(c.v)
		if v.packable != c.want {
			t.Errorf("packable(%q) = %v, want %v", c.v, v.packable, c.want)
		}
	}
}

// TestPackVersionRefusesEmptyRelease pins the packer-local invariant: an
// empty release means a zero-value Version, which must sort below every real
// version, and packing it would instead give it version "0"'s key. Parse is
// currently the only caller and never passes one, so this can only be tested
// by calling the packer directly -- which is the point: the invariant must
// not depend on who calls it next.
func TestPackVersionRefusesEmptyRelease(t *testing.T) {
	var ln letterNumber
	if _, ok := packVersion(part.BigInt{}, nil, ln, ln, ln, ""); ok {
		t.Fatal("packVersion packed an empty release; a zero-value Version would compare equal to version 0")
	}
}

// TestCompareSharedVersionCopiesRace exercises the historical data race:
// go-version v0.0.2's Parts.Normalize leaves len < cap on the key's release
// slice, and Parts.Padding appended into that spare capacity in place, so two
// goroutines comparing by-value COPIES of one Version raced on its backing
// array. Compare now pads into fresh slices (padParts), making a Version
// safely shareable. Run under -race; the unpackable locals force the general
// path where the race lived.
func TestCompareSharedVersionCopiesRace(t *testing.T) {
	// Trailing zero stripped by Normalize: key release len 2, cap 3.
	shared := MustParse("1.2.0+shared")
	// Longer release forces Padding of shared's key from 2 toward 4.
	longer := MustParse("1.2.3.4+other")

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v := shared // by-value copy, shares the key's backing array
			o := longer
			for i := 0; i < 200; i++ {
				if got := v.Compare(o); got != -1 {
					t.Errorf("Compare = %d, want -1", got)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// Benchmarks. The "Packed" benchmarks take the integer fast path; the "Slow"
// ones force the general path over the same versions, which is byte-for-byte
// the pre-spike code (modulo padParts vs Parts.Padding).
func benchPairs() (a, b Version) {
	return MustParse("2.31.0"), MustParse("2.30.5")
}

func BenchmarkComparePacked(b *testing.B) {
	x, y := benchPairs()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if x.Compare(y) != 1 {
			b.Fatal("wrong answer")
		}
	}
}

func BenchmarkCompareSlow(b *testing.B) {
	x, y := benchPairs()
	x.packable = false
	y.packable = false
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if x.Compare(y) != 1 {
			b.Fatal("wrong answer")
		}
	}
}

func BenchmarkComparePackedEqual(b *testing.B) {
	x := MustParse("1.24.0")
	y := MustParse("1.24")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if x.Compare(y) != 0 {
			b.Fatal("wrong answer")
		}
	}
}

func BenchmarkCompareSlowEqual(b *testing.B) {
	x := MustParse("1.24.0")
	y := MustParse("1.24")
	x.packable = false
	y.packable = false
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if x.Compare(y) != 0 {
			b.Fatal("wrong answer")
		}
	}
}

// BenchmarkSortVersions sorts a realistic mixed slice, the shape consumers
// actually pay for.
func sortInput(b *testing.B) []Version {
	var vs []Version
	for maj := 0; maj < 4; maj++ {
		for min := 0; min < 25; min++ {
			vs = append(vs, MustParse(fmt.Sprintf("%d.%d.%d", maj, min, (maj*7+min)%5)))
			vs = append(vs, MustParse(fmt.Sprintf("%d.%d.0rc%d", maj, min+1, min%3+1)))
			vs = append(vs, MustParse(fmt.Sprintf("%d.%d.0.dev%d", maj, min+1, 20240100+min)))
		}
	}
	return vs
}

func BenchmarkSortVersionsPacked(b *testing.B) {
	src := sortInput(b)
	buf := make([]Version, len(src))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy(buf, src)
		sort.Sort(SortedVersions(buf))
	}
}

func BenchmarkSortVersionsSlow(b *testing.B) {
	src := sortInput(b)
	for i := range src {
		src[i].packable = false
	}
	buf := make([]Version, len(src))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy(buf, src)
		sort.Sort(SortedVersions(buf))
	}
}

func BenchmarkParse(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Parse("2.31.0.post1"); err != nil {
			b.Fatal(err)
		}
	}
}
