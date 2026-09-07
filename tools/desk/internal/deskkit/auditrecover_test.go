package deskkit

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRecoverCarriesCounterAndIdempotency is the fail-first test for hole B: the sanctioned
// corruption recovery must CARRY OVER the rate-limit counter and the idempotency store, not
// reset them the way moving the whole file aside does.
//
// It seeds a log that is load-bearing in both ways — it holds a completed write (idempotency)
// AND fills the per-PR budget (counter) — then corrupts it with one malformed line and
// recovers. After recovery the good state must be intact: the completed write is still
// remembered, and the budget is still spent. A whole-file reset would fail both assertions.
func TestRecoverCarriesCounterAndIdempotency(t *testing.T) {
	dir := setup(t)
	head := "deadbeef"

	// Idempotency marker: a completed review at (repo, pr, head).
	appendEntry(t, dir, Entry{
		Repo: testRepo, PR: testPRPtr, HeadSHA: sptr(head),
		Tool: "deskpost", Verb: "review", Result: ResultOK,
	})
	// Counter state: fill the rest of the per-PR budget with charged writes.
	for i := 0; i < RateLimitPerPRPerHour-1; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "comment", Result: ResultOK})
	}

	// Corrupt the log with a single malformed (kill -9 style) partial append.
	appendLine(t, dir, `{"ts":"2026-01-01T00:00:00Z","tool":"deskpost","verb":"comm`)

	// Precondition: corruption makes the whole log refuse and idempotency forget.
	if _, err := LoadEntries(); !IsUnverifiable(err) {
		t.Fatalf("expected corrupt log to refuse before recovery, got err=%v", err)
	}
	if AlreadyDone(testRepo, testPR, head, "review") {
		t.Fatalf("corrupt log should fail-closed to not-done before recovery")
	}

	// Recover.
	res, err := RecoverCorruptAudit()
	if err != nil {
		t.Fatalf("RecoverCorruptAudit: %v", err)
	}
	if !res.Rewrote || res.Quarantined != 1 || res.Carried != RateLimitPerPRPerHour {
		t.Fatalf("recovery summary = %+v, want Rewrote=true Quarantined=1 Carried=%d", res, RateLimitPerPRPerHour)
	}

	// Carry-over #1 — idempotency: the completed write is remembered again.
	if !AlreadyDone(testRepo, testPR, head, "review") {
		t.Errorf("idempotency was NOT carried over: completed review forgotten after recovery")
	}
	// Carry-over #2 — counter: the budget is still spent (not reset to full).
	if got := meterOf(AllowWrite("deskpost", testRepo, testPR)); got != "pr-budget" {
		t.Errorf("counter was NOT carried over: budget reads %q after recovery (want still pr-budget)", got)
	}
	// The malformed line is quarantined to a sidecar, and the live log parses cleanly.
	if res.QuarantinePath == "" {
		t.Fatalf("no quarantine sidecar recorded")
	}
	if b, rerr := os.ReadFile(res.QuarantinePath); rerr != nil || len(b) == 0 {
		t.Errorf("quarantine sidecar missing/empty: err=%v len=%d", rerr, len(b))
	}
	entries, err := LoadEntries()
	if err != nil {
		t.Fatalf("log did not parse cleanly after recovery: %v", err)
	}
	if len(entries) != RateLimitPerPRPerHour {
		t.Errorf("recovered log has %d entries, want %d", len(entries), RateLimitPerPRPerHour)
	}
}

// TestRecoverNoOpWhenClean — a clean log is left exactly as it was (append-only preserved),
// and recovery reports it did not rewrite.
func TestRecoverNoOpWhenClean(t *testing.T) {
	dir := setup(t)
	appendEntry(t, dir, Entry{Repo: testRepo, PR: testPRPtr, Tool: "deskpost", Verb: "comment", Result: ResultOK})
	before, err := os.ReadFile(filepath.Join(dir, "audit.jsonl"))
	if err != nil {
		t.Fatalf("read before: %v", err)
	}
	res, err := RecoverCorruptAudit()
	if err != nil {
		t.Fatalf("RecoverCorruptAudit: %v", err)
	}
	if res.Rewrote || res.Quarantined != 0 || res.Carried != 1 {
		t.Fatalf("clean recovery = %+v, want Rewrote=false Quarantined=0 Carried=1", res)
	}
	after, err := os.ReadFile(filepath.Join(dir, "audit.jsonl"))
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("clean log was rewritten:\nbefore=%q\nafter=%q", before, after)
	}
}

// TestRecoverMissingFile — no audit log is not corruption: nothing to recover, no error.
func TestRecoverMissingFile(t *testing.T) {
	setup(t)
	res, err := RecoverCorruptAudit()
	if err != nil {
		t.Fatalf("RecoverCorruptAudit on missing file: %v", err)
	}
	if res.Rewrote || res.Carried != 0 || res.Quarantined != 0 {
		t.Fatalf("missing-file recovery = %+v, want zero", res)
	}
}
