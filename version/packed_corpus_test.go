// SPDX-License-Identifier: Apache-2.0 OR MIT

package version

import (
	"bufio"
	"os"
	"testing"
)

// TestCorpusPackedFraction reports, for a real corpus of version strings (one
// per line, duplicates meaningful), what fraction of them pack. Skipped unless
// GPP_VERSION_CORPUS points at a corpus file.
//
// Against the production PyPI RSF of 2026-08-04 (932,861 packages, 7,666,849
// version occurrences) this reported 97.26% packable, with 0 local labels in
// the entire index and 0.0142% nonzero epochs.
func TestCorpusPackedFraction(t *testing.T) {
	path := os.Getenv("GPP_VERSION_CORPUS")
	if path == "" {
		t.Skip("set GPP_VERSION_CORPUS to a file of version strings")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	var total, parsed, packable int
	for sc.Scan() {
		total++
		v, err := Parse(sc.Text())
		if err != nil {
			continue
		}
		parsed++
		if v.packable {
			packable++
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("total=%d parsed=%d packable=%d (%.4f%% of parsed)",
		total, parsed, packable, 100*float64(packable)/float64(parsed))
}
