package main

import (
	"encoding/json"
	"testing"
)

// gateScoreRow mirrors the anonymous row struct runGateScores marshals to
// stdout. It is the deskboard's cross-repo prioritization input.
type gateScoreRow struct {
	Brief        string `json:"brief"`
	Score        int    `json:"score"`
	BlockedCount int    `json:"blockedCount"`
	Stream       string `json:"stream"`
	Status       string `json:"status"`
	Repo         string `json:"repo,omitempty"`
}

// TestRunGateScoresHydratesFrontmatter is the end-to-end regression test for
// issue #266. It runs runGateScores against a real fixture root (brief files on
// disk, README table, no in-memory Brief literals) and asserts the emitted JSON
// carries the value weight and the unblocks term.
//
// Before the fix, runGateScores called loadStreams + attachPlaceholders but not
// checkBriefFiles, so Brief.Value/Brief.Depends were never hydrated from
// frontmatter: this row came out 2000 / blockedCount 0 instead of 2700 /
// blockedCount 1. The fixture reproduces the reviewer's exact repro numbers.
func TestRunGateScoresHydratesFrontmatter(t *testing.T) {
	root := "testdata/gatescores"

	out := captureStdout(t, func() {
		if code := runGateScores(root); code != 0 {
			t.Fatalf("runGateScores returned non-zero exit %d", code)
		}
	})

	var rows []gateScoreRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("gate-scores did not emit valid JSON: %v\noutput: %q", err, out)
	}

	// Only the implemented brief 01 is scored; the todo brief 02 is not
	// awaiting and must not appear.
	if len(rows) != 1 {
		t.Fatalf("want exactly 1 scored (awaiting) row, got %d: %+v", len(rows), rows)
	}
	got := rows[0]

	if got.Brief != "gatehydration/01" {
		t.Errorf("brief = %q, want gatehydration/01", got.Brief)
	}
	// priorityWeight(P1)=2000 + valueWeightHigh=200 + unblocksWeight×1=500.
	// A missing hydration collapses this to 2000 (value→med, depends→empty).
	const wantScore = weightP1 + valueWeightHigh + unblocksWeight
	if got.Score != wantScore {
		t.Errorf("score = %d, want %d (frontmatter value:high + unblocks not hydrated?)", got.Score, wantScore)
	}
	if got.BlockedCount != 1 {
		t.Errorf("blockedCount = %d, want 1 (depends graph not hydrated from frontmatter?)", got.BlockedCount)
	}
	if got.Status != "implemented" {
		t.Errorf("status = %q, want implemented", got.Status)
	}
}

// NOTE: the companion TestRunRoadmapHydratesFrontmatter was removed with
// roadmap.go in oss-replacement/06 (the roadmap deck is now DevLake's). The
// issue-266 hydration regression stays covered here by
// TestRunGateScoresHydratesFrontmatter, which exercises the same
// loadHydratedStreams path through a RETAINED surface (--gate-scores).
