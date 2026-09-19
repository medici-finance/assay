package main

import (
	"net/http"
	"testing"
	"time"
)

// flowFixtureHistory mirrors testdata/flow/window-a/docs/streams/.history.jsonl
// (kept in lockstep by hand — a small, well-understood fixture is clearer than
// a file read for these two in-memory tests; the CLI-facing Verify rows read
// the checked-in file directly).
func flowFixtureHistory() []HistoryEntry {
	return []HistoryEntry{
		{Ts: "2026-08-01T00:00:00Z", Brief: "example-app/01", From: "", To: "todo"},
		{Ts: "2026-08-01T00:00:00Z", Brief: "example-app/02", From: "", To: "todo"},
		{Ts: "2026-08-01T00:00:00Z", Brief: "example-app/03", From: "", To: "todo"},
		{Ts: "2026-08-02T00:00:00Z", Brief: "example-app/01", From: "todo", To: "in-progress"},
		{Ts: "2026-08-03T00:00:00Z", Brief: "example-app/01", From: "in-progress", To: "implemented"},
		{Ts: "2026-08-04T00:00:00Z", Brief: "example-app/01", From: "implemented", To: "verified"},
		{Ts: "2026-08-05T00:00:00Z", Brief: "example-app/01", From: "verified", To: "done"},
		{Ts: "2026-08-07T00:00:00Z", Brief: "example-app/02", From: "todo", To: "in-progress"},
		{Ts: "2026-08-08T00:00:00Z", Brief: "example-app/02", From: "in-progress", To: "implemented"},
		{Ts: "2026-08-09T00:00:00Z", Brief: "example-app/02", From: "implemented", To: "verified"},
		{Ts: "2026-08-10T00:00:00Z", Brief: "example-app/03", From: "todo", To: "in-progress"},
	}
}

func flowFixtureStreams() []*Stream {
	return []*Stream{
		{
			Name: "example-app",
			Briefs: []Brief{
				{Num: "01", Status: "done"},
				{Num: "02", Status: "verified", Depends: []string{"example-app/01"}},
				{Num: "03", Status: "in-progress"},
			},
		},
	}
}

// TestFlow (Verify row 1 / row 3's in-process twin): the resolution is
// measured from the historian's own cadence, and example-app/02's
// eligible_to_start reads "measured" — 3 days (measured from 01's earliest
// verified/done instant, 2026-08-04, to 02's first in-progress observation,
// 2026-08-07) sits well above the fixture's 1-day resolution.
func TestFlow(t *testing.T) {
	streams := flowFixtureStreams()
	history := flowFixtureHistory()
	now := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	since, until, err := resolveBFWindow("", "", now)
	if err != nil {
		t.Fatalf("resolveBFWindow: %v", err)
	}

	rep := computeFlowReport(streams, history, "testdata/flow/window-a", false, 0, since, until, now)

	if rep.Resolution.Status != "measured" || rep.Resolution.Seconds != 86400 {
		t.Fatalf("resolution = %+v, want measured/86400s (1-day median regen cadence)", rep.Resolution)
	}
	if rep.Environment.StatusgenVersion == "" {
		t.Fatalf("environment.statusgen_version must never be empty")
	}
	if rep.Environment.ModelVersions != "could-not-check" {
		t.Fatalf("environment.model_versions = %q, want the literal could-not-check (no run-record source supplies it)", rep.Environment.ModelVersions)
	}

	var row *FlowBriefRow
	for i := range rep.Briefs {
		if rep.Briefs[i].ID == "example-app/02" {
			row = &rep.Briefs[i]
		}
	}
	if row == nil {
		t.Fatalf("example-app/02 missing from report briefs")
	}
	if row.EligibleToStart.Status != "measured" {
		t.Fatalf("example-app/02 eligible_to_start = %+v, want measured", row.EligibleToStart)
	}
	const wantSecs = 3 * 24 * 3600
	if row.EligibleToStart.Seconds != wantSecs {
		t.Fatalf("example-app/02 eligible_to_start.seconds = %d, want %d (3 days)", row.EligibleToStart.Seconds, wantSecs)
	}
	if row.EligibleToStart.From != "2026-08-04T00:00:00Z" || row.EligibleToStart.To != "2026-08-07T00:00:00Z" {
		t.Fatalf("example-app/02 eligible_to_start from/to = %q/%q, want the dependency's verified instant to the first in-progress observation", row.EligibleToStart.From, row.EligibleToStart.To)
	}

	// --bottleneck must still render unchanged beside --flow (Verify row 4;
	// exercised at the CLI in the Verify table, asserted structurally here:
	// this report never touches STATUS.md or the bottleneck computation).
}

// TestFlowRefusesOpenInterval (Verify row 2): example-app/03's last historian
// record is `to: "in-progress"` with nothing after it. active_work_time must
// report could-not-check, never a fabricated duration — the same discipline
// briefefficiency.go's completed-dwell walker already keeps, proved here for
// this file's OWN walker (flowTransitionEdges).
func TestFlowRefusesOpenInterval(t *testing.T) {
	streams := flowFixtureStreams()
	history := flowFixtureHistory()
	now := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	since, until, err := resolveBFWindow("", "", now)
	if err != nil {
		t.Fatalf("resolveBFWindow: %v", err)
	}

	rep := computeFlowReport(streams, history, "testdata/flow/window-a", false, 0, since, until, now)

	var row *FlowBriefRow
	for i := range rep.Briefs {
		if rep.Briefs[i].ID == "example-app/03" {
			row = &rep.Briefs[i]
		}
	}
	if row == nil {
		t.Fatalf("example-app/03 missing from report briefs")
	}
	if row.ActiveWorkTime.Status != "could-not-check" {
		t.Fatalf("example-app/03 active_work_time = %+v, want could-not-check (open in-progress interval, no closing record) — never a fabricated duration", row.ActiveWorkTime)
	}
	if row.ActiveWorkTime.Seconds != 0 {
		t.Fatalf("example-app/03 active_work_time.seconds = %d, want 0 — a could-not-check duration must never also carry a number", row.ActiveWorkTime.Seconds)
	}
}

// countingRoundTripper fails the test if ever invoked, and counts calls so a
// test can assert on the count directly rather than relying on t.Fatal never
// having fired (a silently-swallowed panic in a goroutine would not fail the
// test the same way a direct count assertion does).
type countingRoundTripper struct {
	t     *testing.T
	calls int
}

func (c *countingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	c.calls++
	c.t.Errorf("RoundTrip called — --flow must never reach the network without --forge")
	return nil, http.ErrHandlerTimeout
}

// TestFlowForgeGatedOnFlagNotToken (Verify row 6): with GH_TOKEN and
// GITHUB_TOKEN both set in the environment, computeCISlotSaturation with
// forgeMode=false must report could-not-check AND must never construct an
// http request — proved by stubbing http.DefaultTransport (ghfetch.go's
// ghClient uses a plain *http.Client with no Transport override, so it falls
// through to http.DefaultTransport) and asserting zero calls, not just a
// could-not-check status a network-reaching implementation could also print
// after the fact.
func TestFlowForgeGatedOnFlagNotToken(t *testing.T) {
	t.Setenv("GH_TOKEN", "fixture-token")
	t.Setenv("GITHUB_TOKEN", "fixture-token")

	orig := http.DefaultTransport
	stub := &countingRoundTripper{t: t}
	http.DefaultTransport = stub
	defer func() { http.DefaultTransport = orig }()

	rep := computeCISlotSaturation("testdata/flow/window-a", false /* forgeMode */, 24)
	if rep.Status != "could-not-check" {
		t.Fatalf("ci_slot_saturation.status = %q, want could-not-check (forgeMode=false)", rep.Status)
	}
	if stub.calls != 0 {
		t.Fatalf("RoundTrip was called %d time(s) with forgeMode=false — a plain --flow in an environment that happens to carry a token must start no forge process and make no network call", stub.calls)
	}
}
