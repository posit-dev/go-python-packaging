// SPDX-License-Identifier: Apache-2.0 OR MIT

package version

import (
	"bufio"
	"compress/gzip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestPackedConformanceFixtures pins Compare -- both the packed fast path and
// the general path -- against pypa/packaging's reference ordering, frozen
// into fixtures so the check runs in CI without Python.
//
// ⚠️ DO NOT DELETE THESE FIXTURES OR WEAKEN THIS TEST. The packed key is a
// SECOND encoding of PEP 440 ordering truth, alongside the general cmpkey
// path. TestPackedAgreesWithSlowPath holds the two encodings together; THIS
// test is the only thing in CI holding either of them to the reference
// implementation. Delete it and a divergence from pypa/packaging can land
// with every check green.
//
// Two fixtures, both "<version>\t<rank>" lines gzipped, where equal versions
// share a dense rank, produced by sorting with pypa/packaging 26.2:
//
//   - pypa-26.2-grid.ranked.gz: the exact output of gridStrings() -- every
//     combination of the packed fields at and across each field's
//     packability limit (204,288 versions). Set equality with gridStrings()
//     is asserted, so extending the grid without regenerating the fixture
//     fails loudly.
//   - pypa-26.2-corpus-sample.ranked.gz: 10,064 unique version strings
//     sampled (every 63rd, sorted order) from the 634,187 unique versions in
//     the production PyPI index of 2026-08-04. Real data rather than
//     constructed data; 3 sampled strings packaging rejects are excluded.
//
// To regenerate (needs pypa/packaging >= 26.2):
//
//	python3 - <<'EOF' < input.txt | gzip -9 > fixture.ranked.gz
//	import sys
//	from packaging.version import Version, InvalidVersion
//	seen = {}
//	for line in sys.stdin:
//	    s = line.rstrip("\n")
//	    if s and s not in seen:
//	        try: seen[s] = Version(s)
//	        except InvalidVersion: pass
//	rank, prev = 0, None
//	for s, v in sorted(seen.items(), key=lambda kv: kv[1]):
//	    if prev is not None and v > prev: rank += 1
//	    prev = v
//	    print(f"{s}\t{rank}")
//	EOF
//
// The checks: every fixture string must parse; every ADJACENT pair in rank
// order must compare consistently with its ranks (full coverage of the
// order, catching local misorderings anywhere); and every pair of a strided
// sample must too (catching long-range and transitivity-breaking errors).
func TestPackedConformanceFixtures(t *testing.T) {
	grid := readRankedFixture(t, "pypa-26.2-grid.ranked.gz")

	// The grid fixture must describe exactly the strings gridStrings()
	// generates: no more (stale fixture), no fewer (unranked grid entries).
	// Both directions are asserted explicitly, and duplicates rejected --
	// a count comparison alone would let a fixture with duplicates mask
	// missing boundary entries.
	want := map[string]bool{}
	for _, s := range gridStrings() {
		want[s] = true
	}
	seen := make(map[string]bool, len(grid))
	for _, e := range grid {
		if seen[e.s] {
			t.Fatalf("grid fixture entry %q is duplicated; regenerate the fixture", e.s)
		}
		seen[e.s] = true
		if !want[e.s] {
			t.Fatalf("grid fixture entry %q is not produced by gridStrings(); regenerate the fixture", e.s)
		}
	}
	for s := range want {
		if !seen[s] {
			t.Fatalf("gridStrings() generates %q but the fixture does not rank it; regenerate the fixture", s)
		}
	}

	checkAgainstRanks(t, "grid", grid)
	checkAgainstRanks(t, "corpus-sample", readRankedFixture(t, "pypa-26.2-corpus-sample.ranked.gz"))
}

type rankedVersion struct {
	s    string
	rank int
	v    Version
}

func readRankedFixture(t *testing.T, name string) []rankedVersion {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = gz.Close() }()

	var out []rankedVersion
	sc := bufio.NewScanner(gz)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		s, rankStr, ok := strings.Cut(sc.Text(), "\t")
		if !ok {
			t.Fatalf("%s: malformed line %q", name, sc.Text())
		}
		rank, err := strconv.Atoi(rankStr)
		if err != nil {
			t.Fatalf("%s: rank in %q: %v", name, sc.Text(), err)
		}
		v, err := Parse(s)
		if err != nil {
			t.Fatalf("%s: packaging accepts %q, Parse rejects it: %v", name, s, err)
		}
		out = append(out, rankedVersion{s, rank, v})
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(out) < 2 {
		t.Fatalf("%s: fixture is empty or truncated (%d entries)", name, len(out))
	}
	return out
}

func checkAgainstRanks(t *testing.T, name string, es []rankedVersion) {
	t.Helper()
	sign := func(x int) int {
		switch {
		case x < 0:
			return -1
		case x > 0:
			return 1
		}
		return 0
	}

	mismatches := 0
	report := func(a, b rankedVersion, got int) {
		mismatches++
		if mismatches <= 20 {
			t.Errorf("%s: Compare(%q, %q) = %d, packaging ranks %d vs %d",
				name, a.s, b.s, got, a.rank, b.rank)
		}
	}

	for i := 1; i < len(es); i++ {
		a, b := es[i-1], es[i]
		if got := a.v.Compare(b.v); got != sign(a.rank-b.rank) {
			report(a, b, got)
		}
	}

	stride := len(es)/1200 + 1
	var sample []rankedVersion
	for i := 0; i < len(es); i += stride {
		sample = append(sample, es[i])
	}
	for i := range sample {
		for j := range sample {
			if got := sample[i].v.Compare(sample[j].v); got != sign(sample[i].rank-sample[j].rank) {
				report(sample[i], sample[j], got)
			}
		}
	}
	if mismatches > 0 {
		t.Fatalf("%s: %d mismatching pairs against the pypa/packaging reference", name, mismatches)
	}
	t.Logf("%s: %d entries, %d adjacent + %d sampled pairs, all consistent",
		name, len(es), len(es)-1, len(sample)*len(sample))
}
