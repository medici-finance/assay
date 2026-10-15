package deskkit

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// fixturePerfDir copies the committed fixture day files into a temp dir and appends one
// truncated line to the newest, as a crash mid-write would leave it.
func fixturePerfDir(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	entries, err := os.ReadDir(filepath.Join("testdata", "perf"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join("testdata", "perf", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if e.Name() == "2026-10-12.jsonl" {
			b = append(b, []byte(`{"schema":"perf-v1","ts":"2026-10-12T11:00:00Z","tool":"desk`)...)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dst
}

// TestSummarisePerfReproducesFixtureAggregates checks every aggregate against values worked by
// hand from testdata/perf (clock fixed at 2026-10-13, so the 2026-10-05 file is outside the
// 7-day window and its 99999 ms outlier must not appear anywhere).
//
// Hand-check, verb-level records only (step:* records excluded):
//
//	deskpr   walls 100 300 200 1000 -> sorted 100 200 300 1000; nearest-rank p50 = ceil(2)=200,
//	         p90 = ceil(3.6)=4th = 1000; max 1000; total 1600; refusals refused+unverifiable = 2 -> 0.5
//	deskboot 1500 (one verb-level record) -> 1500/1500/1500, total 1500, 0 refusals
//	deskpost 50 150 250 -> p50 = ceil(1.5)=2nd = 150, p90 = ceil(2.7)=3rd = 250, total 450
//	grand total 3550: shares 1600/3550 = 45.1, 1500/3550 = 42.3, 450/3550 = 12.7
func TestSummarisePerfReproducesFixtureAggregates(t *testing.T) {
	perfFixture(t)
	h, err := loadPerfHistoryFrom(fixturePerfDir(t), 7)
	if err != nil {
		t.Fatal(err)
	}
	if h.DayFiles != 2 {
		t.Fatalf("day files in window = %d, want 2 (the 2026-10-05 file is outside it)", h.DayFiles)
	}
	if h.Skipped != 1 {
		t.Fatalf("skipped = %d, want 1 (the truncated final line)", h.Skipped)
	}
	if len(h.Records) != 11 {
		t.Fatalf("records = %d, want 11", len(h.Records))
	}

	s := SummarisePerf(h.Records, 3)
	want := PerfSummary{
		Records: 8,
		TotalMs: 3550,
		Tools: []PerfToolStats{
			{Tool: "deskpr", Count: 4, P50Ms: 200, P90Ms: 1000, MaxMs: 1000, TotalMs: 1600, SharePct: 45.1, Refusals: 2, RefusalRatio: 0.5},
			{Tool: "deskboot", Count: 1, P50Ms: 1500, P90Ms: 1500, MaxMs: 1500, TotalMs: 1500, SharePct: 42.3},
			{Tool: "deskpost", Count: 3, P50Ms: 150, P90Ms: 250, MaxMs: 250, TotalMs: 450, SharePct: 12.7},
		},
		Slowest: []PerfSlow{
			{TS: "2026-10-12T10:10:02Z", Tool: "deskboot", Verb: "the-desk", WallMs: 1500},
			{TS: "2026-10-11T09:15:00Z", Tool: "deskpr", Verb: "create", WallMs: 1000},
			{TS: "2026-10-11T09:05:00Z", Tool: "deskpr", Verb: "create", WallMs: 300},
		},
	}
	if !reflect.DeepEqual(s, want) {
		t.Fatalf("SummarisePerf drifted from the hand-checked values:\n got  %+v\n want %+v", s, want)
	}

	b := SummariseBootSteps(h.Records)
	wantBoot := PerfBootSummary{
		Records: 3,
		Roles: []PerfBootRole{
			{Role: "the-desk", TotalMs: 1000, Steps: []PerfStepStats{
				{Step: "token-mint", Count: 1, P50Ms: 400, P90Ms: 400, MaxMs: 400},
				{Step: "board-fetch", Count: 1, P50Ms: 600, P90Ms: 600, MaxMs: 600},
			}},
			{Role: "worker-desk", TotalMs: 200, Steps: []PerfStepStats{
				{Step: "token-mint", Count: 1, P50Ms: 200, P90Ms: 200, MaxMs: 200},
			}},
		},
	}
	if !reflect.DeepEqual(b, wantBoot) {
		t.Fatalf("SummariseBootSteps drifted:\n got  %+v\n want %+v", b, wantBoot)
	}

	// Filters: by tool, by role, and the 1-day window (only 2026-10-13 — no file, so empty).
	if got := FilterPerf(h.Records, "deskpost", ""); len(got) != 3 {
		t.Errorf("FilterPerf(tool=deskpost) = %d records, want 3", len(got))
	}
	if got := FilterPerf(h.Records, "", "the-desk"); len(got) != 3 {
		t.Errorf("FilterPerf(role=the-desk) = %d records, want 3", len(got))
	}
	one, err := loadPerfHistoryFrom(fixturePerfDir(t), 1)
	if err != nil || one.DayFiles != 0 || len(one.Records) != 0 {
		t.Errorf("1-day window = %d files / %d records (%v), want empty", one.DayFiles, len(one.Records), err)
	}
	// A missing directory is an empty history, not an error.
	if none, err := loadPerfHistoryFrom(filepath.Join(t.TempDir(), "absent"), 7); err != nil || none.DayFiles != 0 {
		t.Errorf("missing dir = %+v, %v; want empty, nil", none, err)
	}
}
