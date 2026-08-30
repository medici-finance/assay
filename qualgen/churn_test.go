package main

import (
	"testing"
	"time"
)

// commitAt builds a Commit with a single measured file diff, at a given time and
// author, whose diff carries the supplied line changes.
func commitAt(sha, author string, when time.Time, lines ...LineChange) (Commit, FileDiff) {
	c := Commit{SHA: sha, AuthorRaw: author, AuthorName: author, AuthorWhen: when}
	fd := FileDiff{CommitSHA: sha, OldPath: "f.go", NewPath: "f.go", Kind: ChangeModified, Lines: Measured([]Hunk{{Lines: lines}})}
	return c, fd
}

// TestChurnWindowBoundary is Verify row 4: a landed line revised (deleted) at
// day 13 counts as `churned`; the identical edit at day 15 does not (14-day
// window). A window with landed lines but zero churn emits `measured-zero`, not
// `could-not-measure`.
func TestChurnWindowBoundary(t *testing.T) {
	day0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	window := time.Duration(DefaultChurnWindowDays) * 24 * time.Hour
	sameClass := func(Commit) string { return IdentityHuman }

	run := func(revisionDay int) ChurnResult {
		landCommit, landDiff := commitAt("land", "dev", day0, addLine("configured := loadConfig()"))
		// The revision deletes the landed line (a rework of it).
		revCommit, revDiff := commitAt("rev", "dev", day0.AddDate(0, 0, revisionDay), delLine("configured := loadConfig()"))
		commits := []Commit{landCommit, revCommit}
		diffs := map[string][]FileDiff{"land": {landDiff}, "rev": {revDiff}}
		return computeChurn(commits, diffs, sameClass, window)
	}

	at13 := run(13)
	if at13.Overall.ChurnedLines != 1 {
		t.Fatalf("day-13 revision must count as churned, got churned=%d new=%d", at13.Overall.ChurnedLines, at13.Overall.NewLines)
	}
	if got := at13.Overall.Rate(); got.State != StateMeasured || got.Value != 1.0 {
		t.Fatalf("day-13 rate: got %+v, want measured 1.0", got)
	}

	at15 := run(15)
	if at15.Overall.ChurnedLines != 0 {
		t.Fatalf("day-15 revision is OUTSIDE the 14-day window, must NOT be churned, got churned=%d", at15.Overall.ChurnedLines)
	}
	// A window with landed lines (new=1) but zero churn is measured-zero, never
	// could-not-measure.
	if got := at15.Overall.Rate(); got.State != StateMeasuredZero {
		t.Fatalf("day-15 rate: got state %q, want measured-zero", got.State)
	}
}

// TestChurnSameIdentityClassOnly pins that churn is attributed within one
// author-identity class: a revision by a DIFFERENT class does not churn the
// landing (spec §4.2 reports rework per identity class).
func TestChurnSameIdentityClassOnly(t *testing.T) {
	day0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	window := time.Duration(DefaultChurnWindowDays) * 24 * time.Hour
	classOf := func(c Commit) string {
		if c.AuthorName == "bot" {
			return IdentityAutomation
		}
		return IdentityHuman
	}
	landCommit, landDiff := commitAt("land", "dev", day0, addLine("configured := loadConfig()"))
	revCommit, revDiff := commitAt("rev", "bot", day0.AddDate(0, 0, 5), delLine("configured := loadConfig()"))
	res := computeChurn([]Commit{landCommit, revCommit}, map[string][]FileDiff{"land": {landDiff}, "rev": {revDiff}}, classOf, window)
	if res.Overall.ChurnedLines != 0 {
		t.Fatalf("a revision by a different identity class must not churn the landing, got churned=%d", res.Overall.ChurnedLines)
	}
}
