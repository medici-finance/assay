package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPRLinkClassifiesFixtureBodies reads each file under statusgen/testdata/prlink/
// and expects the verdict encoded in the filename: linked-*, unlinked-*, multi-linked-*.
func TestPRLinkClassifiesFixtureBodies(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("testdata", "prlink"))
	if err != nil {
		t.Fatalf("read prlink fixtures: %v", err)
	}
	records := make([]PRLinkRecord, 0, len(entries))
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		b, rerr := os.ReadFile(filepath.Join("testdata", "prlink", e.Name()))
		if rerr != nil {
			t.Fatalf("read %s: %v", e.Name(), rerr)
		}
		records = append(records, PRLinkRecord{Number: len(records) + 1, Body: string(b)})
		names = append(names, e.Name())
	}
	if len(records) == 0 {
		t.Fatal("no prlink fixtures found under statusgen/testdata/prlink/")
	}
	verdicts := ClassifyPRLink(records)
	for i, r := range records {
		prefix := strings.SplitN(names[i], "-", 2)[0]
		want := map[string]PRLinkVerdict{
			"linked": PRLinkLinked, "unlinked": PRLinkUnlinked, "multi": PRLinkMultiLinked,
		}[prefix]
		if want == "" {
			t.Fatalf("fixture %s has no verdict prefix", names[i])
		}
		if verdicts[r.Number] != want {
			t.Errorf("PR %d (%s): got %s, want %s", r.Number, names[i], verdicts[r.Number], want)
		}
	}
}

// TestPRLinkFencedTrailerIgnored is the grammar edge case: a Brief: line inside a
// fenced code block is documentation, not a link.
func TestPRLinkFencedTrailerIgnored(t *testing.T) {
	body := "Docs:\n\n```\nBrief: not/a/real/one\n```\n"
	if got := ClassifyPRLink([]PRLinkRecord{{Number: 1, Body: body}})[1]; got != PRLinkUnlinked {
		t.Fatalf("fenced trailer must not link; got %s", got)
	}
}

// TestPRLinkMulti counts duplicate Brief: lines as multi-linked.
func TestPRLinkMulti(t *testing.T) {
	body := "Brief: a/01\nBrief: b/02\n"
	if got := ClassifyPRLink([]PRLinkRecord{{Number: 1, Body: body}})[1]; got != PRLinkMultiLinked {
		t.Fatalf("two Brief: lines must be multi-linked; got %s", got)
	}
}
