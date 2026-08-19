package main

import (
	"strings"
	"testing"
	"time"
)

// fixedNow is a stable clock for the issue-metric tests.
var issueNow = time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)

func daysAgo(n int) time.Time { return issueNow.AddDate(0, 0, -n) }

// baseIssueCfg is a config with no roster (pure) — team set is explicit so the
// classification math is exercised without an env/roster read.
func baseIssueCfg() issueMetricConfig {
	return issueMetricConfig{
		Now:        issueNow,
		StaleDays:  defaultStaleIssueDays,
		OldestN:    defaultOldestIssueN,
		OrgAccount: "the-org",
		TeamLogins: map[string]bool{"teammate": true},
	}
}

func TestIssueTimeToClose(t *testing.T) {
	recs := []issueMetricRecord{
		{Number: 1, State: "CLOSED", CreatedAt: daysAgo(10), ClosedAt: daysAgo(8), HasClosed: true},  // 2d
		{Number: 2, State: "CLOSED", CreatedAt: daysAgo(20), ClosedAt: daysAgo(16), HasClosed: true}, // 4d
		{Number: 3, State: "CLOSED", CreatedAt: daysAgo(30), ClosedAt: daysAgo(24), HasClosed: true}, // 6d
		{Number: 4, State: "OPEN", CreatedAt: daysAgo(1)},                                            // no close
		// Malformed: closedAt before createdAt — excluded from the median, never negative.
		{Number: 5, State: "CLOSED", CreatedAt: daysAgo(1), ClosedAt: daysAgo(3), HasClosed: true},
	}
	rep := computeIssueMetrics(recs, baseIssueCfg())
	if rep.Open != 1 || rep.Closed != 4 {
		t.Fatalf("open/closed = %d/%d, want 1/4", rep.Open, rep.Closed)
	}
	if rep.TimeToClose.N != 3 {
		t.Fatalf("time-to-close n = %d, want 3 (malformed excluded)", rep.TimeToClose.N)
	}
	// Median of {2d,4d,6d} = 4d.
	if !strings.Contains(rep.TimeToClose.Median, "4.0d") {
		t.Errorf("median = %q, want ~4.0d", rep.TimeToClose.Median)
	}
	if rep.CloseRate != "0.80" {
		t.Errorf("close-rate = %q, want 0.80", rep.CloseRate)
	}
}

func TestIssueAgeBucketing(t *testing.T) {
	recs := []issueMetricRecord{
		{Number: 1, State: "OPEN", CreatedAt: issueNow.Add(-12 * time.Hour)}, // <1d
		{Number: 2, State: "OPEN", CreatedAt: daysAgo(2)},                    // 1-3d
		{Number: 3, State: "OPEN", CreatedAt: daysAgo(5)},                    // 3-7d
		{Number: 4, State: "OPEN", CreatedAt: daysAgo(9)},                    // >7d
		{Number: 5, State: "OPEN", CreatedAt: daysAgo(20)},                   // >7d, oldest
		{Number: 6, State: "CLOSED", CreatedAt: daysAgo(30), ClosedAt: daysAgo(1), HasClosed: true},
	}
	rep := computeIssueMetrics(recs, baseIssueCfg())
	want := map[string]int{"<1d": 1, "1-3d": 1, "3-7d": 1, ">7d": 2}
	for k, v := range want {
		if rep.AgeBuckets[k] != v {
			t.Errorf("age bucket %s = %d, want %d", k, rep.AgeBuckets[k], v)
		}
	}
	// Closed issues never bucket.
	total := 0
	for _, b := range issueAgeBucketOrder {
		total += rep.AgeBuckets[b]
	}
	if total != 5 {
		t.Errorf("bucketed open issues = %d, want 5", total)
	}
	if len(rep.Oldest) == 0 || rep.Oldest[0].Number != 5 {
		t.Errorf("oldest[0] = %+v, want #5", rep.Oldest)
	}
	// Stale: default 7d threshold, #4 (9d) and #5 (20d) are over.
	if rep.Stale.Over != 2 || rep.Stale.Open != 5 {
		t.Errorf("stale over/open = %d/%d, want 2/5", rep.Stale.Over, rep.Stale.Open)
	}
	if rep.Stale.OldestNumber != 5 {
		t.Errorf("stale oldest = #%d, want #5", rep.Stale.OldestNumber)
	}
}

func TestIssueAuthorClassification(t *testing.T) {
	recs := []issueMetricRecord{
		{Number: 1, State: "OPEN", CreatedAt: daysAgo(1), Author: "assay-worker-app[bot]"}, // agent+internal
		{Number: 2, State: "OPEN", CreatedAt: daysAgo(1), Author: "app/assay-desk"},        // agent+internal
		{Number: 3, State: "OPEN", CreatedAt: daysAgo(1), Author: "the-org"},               // agent (org)
		{Number: 4, State: "OPEN", CreatedAt: daysAgo(1), Author: "teammate"},              // human+internal
		{Number: 5, State: "OPEN", CreatedAt: daysAgo(1), Author: "outsider"},              // human+external
	}
	rep := computeIssueMetrics(recs, baseIssueCfg())
	if rep.Agent != 3 {
		t.Errorf("agent = %d, want 3 (two bots + org)", rep.Agent)
	}
	if rep.Human != 2 {
		t.Errorf("human = %d, want 2", rep.Human)
	}
	if rep.Internal != 4 {
		t.Errorf("internal = %d, want 4 (agents + teammate)", rep.Internal)
	}
	if rep.External != 1 {
		t.Errorf("external = %d, want 1 (outsider)", rep.External)
	}
}

func TestIssueByDeskGrouping(t *testing.T) {
	recs := []issueMetricRecord{
		{Number: 1, State: "OPEN", CreatedAt: daysAgo(1), Labels: []string{"raised-by:verify-desk"}},
		{Number: 2, State: "OPEN", CreatedAt: daysAgo(1), Labels: []string{"raised-by:issue-loop"}},
		{Number: 3, State: "OPEN", CreatedAt: daysAgo(1), Labels: []string{"raised-by:verify-desk", "bug"}},
		{Number: 4, State: "OPEN", CreatedAt: daysAgo(1), Labels: []string{"bug"}},        // unattributed
		{Number: 5, State: "OPEN", CreatedAt: daysAgo(1), Labels: []string{}},             // unattributed
		{Number: 6, State: "OPEN", CreatedAt: daysAgo(1), Labels: []string{"raised-by:"}}, // empty desk → unattributed
	}
	rep := computeIssueMetrics(recs, baseIssueCfg())
	if rep.ByDesk["verify-desk"] != 2 {
		t.Errorf("verify-desk = %d, want 2", rep.ByDesk["verify-desk"])
	}
	if rep.ByDesk["issue-loop"] != 1 {
		t.Errorf("issue-loop = %d, want 1", rep.ByDesk["issue-loop"])
	}
	if rep.ByDesk[unattributedDesk] != 3 {
		t.Errorf("unattributed = %d, want 3", rep.ByDesk[unattributedDesk])
	}
}

func TestIssueTypeAndSeverityPartition(t *testing.T) {
	recs := []issueMetricRecord{
		// Process states — NOT bugs, even when also carrying a bug label.
		{Number: 1, State: "OPEN", CreatedAt: daysAgo(1), Labels: []string{"verify-gate"}},
		{Number: 2, State: "OPEN", CreatedAt: daysAgo(1), Labels: []string{"needs-decision"}},
		{Number: 3, State: "OPEN", CreatedAt: daysAgo(1), Labels: []string{"live-verify", "bug"}}, // state wins
		// Defects.
		{Number: 4, State: "OPEN", CreatedAt: daysAgo(1), Labels: []string{"bug"}},                             // normal defect
		{Number: 5, State: "OPEN", CreatedAt: daysAgo(1), Labels: []string{"bug", "critical"}},                 // critical defect
		{Number: 6, State: "OPEN", CreatedAt: daysAgo(1), Labels: []string{"bug"}, Title: "URGENT: prod down"}, // critical via title
		// Other.
		{Number: 7, State: "OPEN", CreatedAt: daysAgo(1), Labels: []string{"question"}},
		{Number: 8, State: "OPEN", CreatedAt: daysAgo(1), Labels: []string{"help wanted"}},
	}
	rep := computeIssueMetrics(recs, baseIssueCfg())
	if rep.ByType.States != 3 {
		t.Errorf("states = %d, want 3 (verify-gate/needs-decision/live-verify — the last NOT a bug)", rep.ByType.States)
	}
	if rep.ByType.Defects != 3 || rep.Defects.Total != 3 {
		t.Errorf("defects = %d (report %d), want 3 (live-verify+bug is excluded)", rep.ByType.Defects, rep.Defects.Total)
	}
	if rep.Defects.Critical != 2 {
		t.Errorf("critical defects = %d, want 2 (label + title)", rep.Defects.Critical)
	}
	if rep.Defects.Normal != 1 {
		t.Errorf("normal defects = %d, want 1", rep.Defects.Normal)
	}
	if rep.ByType.Other["question"] != 1 || rep.ByType.Other["help wanted"] != 1 {
		t.Errorf("other = %+v, want question:1 help wanted:1", rep.ByType.Other)
	}
}

func TestIssueBannerPresent(t *testing.T) {
	rep := computeIssueMetrics(nil, baseIssueCfg())
	if !strings.Contains(rep.Note, "DIAGNOSTIC") {
		t.Errorf("JSON note missing DIAGNOSTIC banner: %q", rep.Note)
	}
	text := renderIssuesText(rep)
	if !strings.Contains(text, "DIAGNOSTIC") {
		t.Errorf("text output missing DIAGNOSTIC banner")
	}
	// The by-desk cut renders even with no issues, naming the raised-by convention.
	if !strings.Contains(text, "raised-by") {
		t.Errorf("text output missing raised-by cut")
	}
	// The type/severity classes render as distinct, named classes (Verify 4b).
	for _, want := range []string{"verify-gate", "defect", "critical"} {
		if !strings.Contains(text, want) {
			t.Errorf("text output missing %q class", want)
		}
	}
}

func TestIssueRenderNamesUnattributed(t *testing.T) {
	recs := []issueMetricRecord{
		{Number: 1, State: "OPEN", CreatedAt: daysAgo(1), Labels: []string{"bug"}},
	}
	rep := computeIssueMetrics(recs, baseIssueCfg())
	text := renderIssuesText(rep)
	if !strings.Contains(text, unattributedDesk) {
		t.Errorf("text output missing %q bucket", unattributedDesk)
	}
}

func TestIssueSeriesWeeklyBuckets(t *testing.T) {
	recs := []issueMetricRecord{
		{Number: 1, State: "CLOSED", CreatedAt: daysAgo(20), ClosedAt: daysAgo(2), HasClosed: true},
		{Number: 2, State: "OPEN", CreatedAt: daysAgo(13)},
		{Number: 3, State: "OPEN", CreatedAt: daysAgo(1)},
	}
	points := computeIssueSeries(recs, issueNow)
	if len(points) < 3 {
		t.Fatalf("expected ≥3 weekly buckets spanning 20d, got %d", len(points))
	}
	// Every opened issue is accounted for across the buckets.
	opened := 0
	closed := 0
	for _, p := range points {
		opened += p.Opened
		closed += p.Closed
	}
	if opened != 3 {
		t.Errorf("total opened across buckets = %d, want 3", opened)
	}
	if closed != 1 {
		t.Errorf("total closed across buckets = %d, want 1", closed)
	}
}

func TestIssueEmptyCorpusDegrades(t *testing.T) {
	rep := computeIssueMetrics(nil, baseIssueCfg())
	if rep.Total != 0 || rep.Open != 0 || rep.Closed != 0 {
		t.Errorf("empty corpus not zeroed: %+v", rep)
	}
	if rep.CloseRate != "n/a" {
		t.Errorf("empty close-rate = %q, want n/a", rep.CloseRate)
	}
	if rep.TimeToClose.Median != "n/a" {
		t.Errorf("empty median = %q, want n/a", rep.TimeToClose.Median)
	}
	// Age buckets still render (seeded at zero).
	for _, b := range issueAgeBucketOrder {
		if _, ok := rep.AgeBuckets[b]; !ok {
			t.Errorf("age bucket %s missing from empty report", b)
		}
	}
}

func TestIssueBotDetection(t *testing.T) {
	cases := map[string]bool{
		"assay-worker-app[bot]": true,
		"app/assay-desk":        true,
		"APP/UPPER":             true,
		"someone[BOT]":          true,
		"a-human":               false,
		"":                      false,
	}
	for login, want := range cases {
		if got := isBotLogin(login); got != want {
			t.Errorf("isBotLogin(%q) = %v, want %v", login, got, want)
		}
	}
}
