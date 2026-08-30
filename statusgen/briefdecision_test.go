package main

import "testing"

func TestComputeDecision_LatencyWindowsOnClosedAt(t *testing.T) {
	now := mustTime(t, "2026-08-26T00:00:00Z")
	since := mustTime(t, "2026-08-01T00:00:00Z")
	until := mustTime(t, "2026-08-20T00:00:00Z")
	closed := []dqIssue{
		{Number: 1, CreatedAt: "2026-08-05T00:00:00Z", ClosedAt: "2026-08-10T00:00:00Z"}, // 5 days, in window
		{Number: 2, CreatedAt: "2026-07-01T00:00:00Z", ClosedAt: "2026-07-05T00:00:00Z"}, // closed before window — excluded
		{Number: 3, CreatedAt: "2026-08-01T00:00:00Z", ClosedAt: "2026-08-25T00:00:00Z"}, // closed AFTER window — excluded
	}
	open := []dqIssue{
		{Number: 4, CreatedAt: "2026-08-20T00:00:00Z"}, // 6 days old
		{Number: 5, CreatedAt: "2026-08-10T00:00:00Z"}, // 16 days old — the oldest
	}
	rep := computeDecisionLatency(open, closed, since, until, now)
	if rep.Latency.N != 1 {
		t.Fatalf("latency.n = %d, want 1 (only issue #1 closes inside the window)", rep.Latency.N)
	}
	if rep.Latency.P50 == nil || *rep.Latency.P50 != 120 { // 5 days = 120 hours
		t.Errorf("latency p50 = %v, want 120h", rep.Latency.P50)
	}
	if rep.WIP != 2 {
		t.Errorf("wip = %d, want 2 (both open issues, unwindowed)", rep.WIP)
	}
	if rep.OldestOpenIssue != 5 {
		t.Errorf("oldest_open_issue = %d, want 5", rep.OldestOpenIssue)
	}
	wantAge := now.Sub(mustTime(t, "2026-08-10T00:00:00Z")).Hours()
	if diff := rep.OldestOpenAgeHours - wantAge; diff > 0.1 || diff < -0.1 {
		t.Errorf("oldest_open_age_hours = %v, want ~%v", rep.OldestOpenAgeHours, wantAge)
	}
}

// An empty-but-successfully-read queue is a legitimate zero (gatetelemetry.go's
// documented distinction: "[] is an affirmative zero rows"), so the NESTED
// latency metric renders could-not-check (aggregateSeconds' own n==0 rule)
// while WIP/oldest render honest zeros — this function itself carries no top
// -level state; runDecisionLatency sets that from whether the gh reads
// succeeded, independent of the counts (see briefdecision.go).
func TestComputeDecisionLatencyEmpty_QueueIsHonestZeroNotFabricated(t *testing.T) {
	now := mustTime(t, "2026-08-26T00:00:00Z")
	since := mustTime(t, "2026-08-01T00:00:00Z")
	until := mustTime(t, "2026-08-20T00:00:00Z")
	rep := computeDecisionLatency(nil, nil, since, until, now)
	if rep.Latency.State != "could-not-check" || rep.Latency.N != 0 {
		t.Errorf("latency = %+v, want could-not-check/n=0 for an empty closed set", rep.Latency)
	}
	if rep.WIP != 0 {
		t.Errorf("wip = %d, want 0", rep.WIP)
	}
	if rep.OldestOpenIssue != 0 || rep.OldestOpenAgeHours != 0 {
		t.Errorf("oldest fields should be zero-valued for an empty open set, got issue=%d age=%v", rep.OldestOpenIssue, rep.OldestOpenAgeHours)
	}
}

func TestComputeDecisionLatency_SkipsUnparseableTimestamps(t *testing.T) {
	now := mustTime(t, "2026-08-26T00:00:00Z")
	since := mustTime(t, "2026-08-01T00:00:00Z")
	until := mustTime(t, "2026-08-20T00:00:00Z")
	closed := []dqIssue{
		{Number: 1, CreatedAt: "not-a-time", ClosedAt: "2026-08-10T00:00:00Z"},
		{Number: 2, CreatedAt: "2026-08-01T00:00:00Z", ClosedAt: "not-a-time"},
	}
	rep := computeDecisionLatency(nil, closed, since, until, now)
	if rep.Latency.N != 0 {
		t.Errorf("latency.n = %d, want 0 (both rows have an unparseable timestamp)", rep.Latency.N)
	}
}

// runDecisionLatency must not panic and must exit 0 regardless of how
// doraTargetRepo (doratiming.go, unmodified here) resolves the repo — same
// caveat doratiming_test.go's own no-repo-is-could-not-check test for
// recordDoraTiming documents: a bare temp dir has no git remote, but the gh-default fallback
// is a real `gh repo view` call this test cannot control in every
// environment, so it asserts the one thing that must hold everywhere: a
// could-not-check (unreadable repo) or an "ok" (a resolvable repo whose fake
// source returns nothing) report both exit 0, never a crash or a non-zero
// usage error.
func TestRunDecisionLatencyNo_RepoDoesNotPanicOrError(t *testing.T) {
	dir := t.TempDir() // no .git
	src := &countingDecisionSource{}
	rc := runDecisionLatency(dir, "2026-08-01", "2026-08-20", true, src)
	if rc != 0 {
		t.Errorf("rc = %d, want 0", rc)
	}
}

type countingDecisionSource struct{ calls int }

func (s *countingDecisionSource) Issues(repo, label, state string) ([]dqIssue, error) {
	s.calls++
	return nil, nil
}
