package deskkit

import (
	"testing"
	"time"
)

// TestVerdictIssueBucketIsAuditVisibleAndMetered — a verdict filing draws on its OWN named
// bucket (VerdictIssueTool), metered repo-wide at the per-PR cap. Under the cap it is
// admitted; over it, refused. This is the audit-visible runaway backstop the lane keeps.
func TestVerdictIssueBucketIsAuditVisibleAndMetered(t *testing.T) {
	dir := setup(t)
	now := time.Now()

	// A handful of recent filings stays under the cap.
	for i := 0; i < 3; i++ {
		appendEntry(t, dir, Entry{Repo: testRepo, Tool: VerdictIssueTool, Verb: "verdict", Result: ResultOK})
	}
	if err := AllowVerdictIssueWriteAt(testRepo, now); err != nil {
		t.Fatalf("AllowVerdictIssueWriteAt under cap = %v, want nil", err)
	}

	// Saturate the rolling-hour cap → refuse. This is the runaway backstop, not a daily cap.
	dir2 := setup(t)
	for i := 0; i < RateLimitPerPRPerHour; i++ {
		appendEntry(t, dir2, Entry{Repo: testRepo, Tool: VerdictIssueTool, Verb: "verdict", Result: ResultOK})
	}
	if err := AllowVerdictIssueWriteAt(testRepo, now); !IsRateLimited(err) {
		t.Fatalf("AllowVerdictIssueWriteAt over the hourly cap = %v, want RateLimited (exit 4)", err)
	}
}

// TestVerdictIssueBucketHasNoDailyCap — the load-bearing property. deskfile refuses a 4th
// `new` issue on a repo within a rolling 24h (its 3/24h session budget). The verdict lane
// must NOT inherit that: a filing cadence spread across the day, far more than 3 filings in
// 24h, is admitted so long as each is within the hourly runaway backstop. Here the seed is
// filings hours apart (outside any rolling hour) — every one is admitted, which a 24h/daily
// cap would forbid.
func TestVerdictIssueBucketHasNoDailyCap(t *testing.T) {
	dir := setup(t)
	now := time.Now()

	// 12 filings across the last 24h, each more than an hour apart, so none share a rolling
	// hour with the write under test. A daily cap of any sane size would already be tripped;
	// the hourly meter sees zero recent charges.
	for i := 1; i <= 12; i++ {
		ts := now.Add(-time.Duration(i) * 2 * time.Hour).UTC().Format(time.RFC3339)
		appendEntry(t, dir, Entry{Repo: testRepo, Tool: VerdictIssueTool, Verb: "verdict", Result: ResultOK, TS: ts})
	}
	if err := AllowVerdictIssueWriteAt(testRepo, now); err != nil {
		t.Fatalf("AllowVerdictIssueWriteAt with 12 filings spread over 24h = %v, want nil "+
			"(the lane has no daily cap — the ~5-minute cadence is the throttle)", err)
	}
}
