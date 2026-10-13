package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestVerifyOutcomes_AdditiveKeys pins that the join keys a verify-outcome record now carries
// (prs, prs_unknown_reason, verifier_tier, started, finished, rows_passed, rows_total,
// row_results, dispatch_ref, dispatch_ref_sha256) are invisible to this reader: a record
// carrying every one of them yields the same latest outcome as the same record without them.
func TestVerifyOutcomes_AdditiveKeys(t *testing.T) {
	const base = `{"ts":"2026-10-06T14:20:00Z","brief":"example-stream/14","outcome":"verify-fail","sha":"0123456789abcdef0123456789abcdef01234567","blocker_kind":"implementation","blocker_ref":"example-org/tracker#7"}`
	const rich = `{"ts":"2026-10-06T14:20:00Z","brief":"example-stream/14","outcome":"verify-fail","sha":"0123456789abcdef0123456789abcdef01234567","blocker_kind":"implementation","blocker_ref":"example-org/tracker#7",` +
		`"prs":[{"repo":"example-org/tracker","number":7,"merge_sha":"89abcdef0123456789abcdef0123456789abcdef"},{"withheld":"` +
		`0000000000000000000000000000000000000000000000000000000000000000"}],"prs_unknown_reason":"no-pr-delivery",` +
		`"verifier_tier":"strong","started":"2026-10-06T14:00:00Z","finished":"2026-10-06T14:10:00Z",` +
		`"rows_passed":0,"rows_total":1,"row_results":[{"row":1,"result":"fail"}],` +
		`"dispatch_ref":"assay--x--1@20261006T141502Z.3fa9c01b7d2e","dispatch_ref_sha256":"` +
		`1111111111111111111111111111111111111111111111111111111111111111"}`

	latest := func(record string) awaitOutcome {
		t.Helper()
		root := t.TempDir()
		dir := filepath.Join(root, filepath.FromSlash(outcomeRecordsDir), "example-stream")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		voWriteFile(t, filepath.Join(dir, "14-20261006T142000Z-aaaaaaaaaaaa.json"), record+"\n")
		idx := loadAwaitOutcomeIndex(root)
		if idx.unreadable != "" {
			t.Fatalf("index unreadable: %s", idx.unreadable)
		}
		o, ok := idx.latest["example-stream/14"]
		if !ok {
			t.Fatalf("no latest outcome for the brief; index = %+v", idx)
		}
		return o
	}
	if got, want := latest(rich), latest(base); got != want {
		t.Errorf("a record carrying every join key reads as %+v, want the same as without them: %+v", got, want)
	}
}
