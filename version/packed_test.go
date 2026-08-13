// SPDX-License-Identifier: Apache-2.0 OR MIT

package version

import (
	"fmt"
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

// gridVersions is a structured grid covering every packed field, its limits,
// and the fallback boundary on both sides.
func gridVersions(tb testing.TB) []Version {
	epochs := []string{"", "1!"}
	releases := []string{
		"0", "1", "1.0", "1.0.0.0", "1.00", "1.2", "1.2.1", "0.0.1",
		"2.31.0.20240406", "2024.1", "20240101.0", "1.2.3.4.5.6",
		"1.2.3.4.5.6.7", "4294967295.1", "4294967296.1", "1.4294967295",
		"1.4294967296", "99999999999999999999.0", "1.99999999999999999999",
	}
	pres := []string{"", "a", "a0", "a1", "b1", "rc1", "rc2", "pre1", "c3",
		"a1048575", "a1048576", "rc20240101"}
	posts := []string{"", ".post", ".post0", ".post1", "-1", ".post16383",
		".post16384", ".post20240101"}
	devs := []string{"", ".dev", ".dev0", ".dev1", ".dev20240101",
		".dev33554431", ".dev33554432"}
	locals := []string{"", "+abc", "+1", "+abc.1", "+ubuntu-2", "+007"}

	var out []Version
	for _, e := range epochs {
		for _, r := range releases {
			for _, p := range pres {
				for _, po := range posts {
					for _, d := range devs {
						for _, l := range locals {
							s := e + r + p + po + d + l
							v, err := Parse(s)
							if err != nil {
								tb.Fatalf("grid version %q failed to parse: %v", s, err)
							}
							out = append(out, v)
						}
					}
				}
			}
		}
	}
	return out
}

// TestPackedAgreesWithSlowPath compares pairs from the grid through both
// paths. The packed path only claims pairs where both sides are packable; for
// those it must agree exactly with the general path.
//
// The full grid is 153,216 versions; all pairs would be 2.3e10 comparisons.
// A deterministic stride keeps every grid dimension represented while
// holding the pair count around 2.4 million, which runs in seconds.
func TestPackedAgreesWithSlowPath(t *testing.T) {
	full := gridVersions(t)
	stride := len(full)/1550 + 1
	var vs []Version
	for i := 0; i < len(full); i += stride {
		vs = append(vs, full[i])
	}
	t.Logf("grid: %d versions, sampled %d", len(full), len(vs))
	packable := 0
	for _, v := range vs {
		if v.packable {
			packable++
		}
	}
	t.Logf("packable: %d/%d", packable, len(vs))
	if packable == 0 {
		t.Fatal("no packable versions in grid; the fast path is untested")
	}
	if packable == len(vs) {
		t.Fatal("every grid version is packable; the fallback boundary is untested")
	}

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
