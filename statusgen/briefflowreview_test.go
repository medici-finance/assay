package main

import "testing"

// --- extractBriefTrailer -----------------------------------------------------

func TestExtractBrief_TrailerBasic(t *testing.T) {
	body := "Implements the thing.\n\nBrief: example-stream/02\n\nMore prose."
	id, ok := extractBriefTrailer(body)
	if !ok || id != "example-stream/02" {
		t.Errorf("got id=%q ok=%v, want example-stream/02/true", id, ok)
	}
}

func TestExtractBrief_TrailerIgnoresFenced_Example(t *testing.T) {
	body := "Docs example:\n\n```\nBrief: not-a-real-link/01\n```\n\nBrief: example-stream/03\n"
	id, ok := extractBriefTrailer(body)
	if !ok || id != "example-stream/03" {
		t.Errorf("got id=%q ok=%v, want the trailer OUTSIDE the fence (example-stream/03)", id, ok)
	}
}

func TestExtractBrief_TrailerNoneFound(t *testing.T) {
	if _, ok := extractBriefTrailer("just some PR body with no trailer"); ok {
		t.Error("expected ok=false for a body with no Brief: trailer")
	}
}

func TestExtractBrief_TrailerTrimsTrailing_Punctuation(t *testing.T) {
	id, ok := extractBriefTrailer("Brief: example-stream/02.")
	if !ok || id != "example-stream/02" {
		t.Errorf("got id=%q ok=%v, want example-stream/02 with trailing '.' stripped", id, ok)
	}
}

// --- resolvePRsByBrief --------------------------------------------------------

func TestResolvePRsBy_BriefLatestWins(t *testing.T) {
	prs := []bfPRRecord{
		{Number: 10, Body: "Brief: s/01", MergedAt: "2026-08-01T00:00:00Z"},
		{Number: 20, Body: "Brief: s/01", MergedAt: "2026-08-10T00:00:00Z"}, // later merge — should win
		{Number: 30, Body: "no trailer here"},
	}
	byBrief, unlinked := resolvePRsByBrief(prs)
	if unlinked != 1 {
		t.Errorf("unlinked = %d, want 1", unlinked)
	}
	got, ok := byBrief["s/01"]
	if !ok || got.Number != 20 {
		t.Errorf("got PR #%d, want the later-merged #20", got.Number)
	}
}

// --- countChangesRequested ----------------------------------------------------

func TestCountChanges_Requested(t *testing.T) {
	reviews := []ghReview{
		{State: "CHANGES_REQUESTED"},
		{State: "COMMENTED"},
		{State: "CHANGES_REQUESTED"},
		{State: "APPROVED"},
	}
	if got := countChangesRequested(reviews); got != 2 {
		t.Errorf("got %d, want 2", got)
	}
}

// --- computeReviewRework ------------------------------------------------------

func TestComputeReview_ReworkDistribution(t *testing.T) {
	byBrief := map[string]bfPRRecord{
		"s/01": {Number: 1},
		"s/02": {Number: 2},
		"s/03": {Number: 3},
	}
	reviewsByPR := map[int][]ghReview{
		1: {}, // 0 rounds — a clean first-pass PR
		2: {{State: "CHANGES_REQUESTED"}},
		3: {{State: "CHANGES_REQUESTED"}, {State: "CHANGES_REQUESTED"}, {State: "CHANGES_REQUESTED"}},
	}
	rep := computeReviewRework(byBrief, 5, func(pr int) ([]ghReview, error) { return reviewsByPR[pr], nil })
	if rep.State != "ok" {
		t.Fatalf("state = %q, want ok", rep.State)
	}
	if rep.N != 3 {
		t.Errorf("n = %d, want 3", rep.N)
	}
	if rep.UnlinkedExcluded != 5 {
		t.Errorf("unlinked_excluded = %d, want 5", rep.UnlinkedExcluded)
	}
	if rep.Distribution["0"] != 1 || rep.Distribution["1"] != 1 || rep.Distribution["3+"] != 1 {
		t.Errorf("distribution = %+v, want {0:1,1:1,3+:1}", rep.Distribution)
	}
	// mean rounds = (0+1+3)/3 = 1.3...
	if rep.MeanRounds < 1.3 || rep.MeanRounds > 1.4 {
		t.Errorf("mean_rounds = %v, want ~1.33", rep.MeanRounds)
	}
}

func TestComputeReview_ReworkNoLinkedPRsIs_CouldNotCheck(t *testing.T) {
	rep := computeReviewRework(map[string]bfPRRecord{}, 7, func(pr int) ([]ghReview, error) { return nil, nil })
	if rep.State != "could-not-check" {
		t.Errorf("state = %q, want could-not-check", rep.State)
	}
	if rep.UnlinkedExcluded != 7 {
		t.Errorf("unlinked_excluded = %d, want 7 (still reported even when could-not-check)", rep.UnlinkedExcluded)
	}
}

func TestComputeReview_ReworkOneUnreadableP_RExcludedNotFatal(t *testing.T) {
	byBrief := map[string]bfPRRecord{
		"s/01": {Number: 1},
		"s/02": {Number: 2}, // this one's reviews read fails
	}
	rep := computeReviewRework(byBrief, 0, func(pr int) ([]ghReview, error) {
		if pr == 2 {
			return nil, errTestReviews
		}
		return nil, nil
	})
	if rep.N != 1 {
		t.Errorf("n = %d, want 1 (the unreadable PR excluded, not fatal)", rep.N)
	}
	if rep.State != "ok" {
		t.Errorf("state = %q, want ok", rep.State)
	}
}

var errTestReviews = &testError{"reviews unreadable"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

// --- computeFirstPassYield -----------------------------------------------------

func TestComputeFirstPass_YieldAllThreeLegs(t *testing.T) {
	doneIDs := []string{"s/01", "s/02", "s/03", "s/04"}
	byBrief := map[string]bfPRRecord{
		"s/01": {Number: 1}, // clean: 0 CR, no verify-fail, no finding -> first pass
		"s/02": {Number: 2}, // CHANGES_REQUESTED present -> not first pass
		"s/03": {Number: 3}, // clean reviews but VERIFY:FAIL in Evidence -> not first pass
		// s/04 has no resolvable PR -> excluded entirely
	}
	reviewsByPR := map[int][]ghReview{
		1: {{State: "APPROVED"}},
		2: {{State: "CHANGES_REQUESTED"}},
		3: {{State: "APPROVED"}},
	}
	evidenceByID := map[string]string{
		"s/01": "## Evidence\nVERIFY: PASS\n",
		"s/03": "## Evidence\nVERIFY: FAIL\n",
	}
	findings := []Finding{} // none — the finding leg is exercised by the next test
	rep := computeFirstPassYield(doneIDs, byBrief, 1,
		func(pr int) ([]ghReview, error) { return reviewsByPR[pr], nil },
		evidenceByID, findings)
	if rep.State != "ok" {
		t.Fatalf("state = %q, want ok", rep.State)
	}
	if rep.N != 3 {
		t.Errorf("n = %d, want 3 (s/04 excluded — no resolvable PR)", rep.N)
	}
	if rep.FirstPass != 1 {
		t.Errorf("first_pass = %d, want 1 (only s/01)", rep.FirstPass)
	}
}

func TestComputeFirstPass_YieldFindingNamed_ExcludesIt(t *testing.T) {
	doneIDs := []string{"s/01"}
	byBrief := map[string]bfPRRecord{"s/01": {Number: 1}}
	reviewsByPR := map[int][]ghReview{1: {{State: "APPROVED"}}} // clean reviews, no verify-fail
	findings := []Finding{{ID: "F-01", Affects: []string{"s/01"}, Resolved: false}}
	rep := computeFirstPassYield(doneIDs, byBrief, 0,
		func(pr int) ([]ghReview, error) { return reviewsByPR[pr], nil },
		map[string]string{}, findings)
	if rep.FirstPass != 0 {
		t.Errorf("first_pass = %d, want 0 (an unresolved finding names s/01)", rep.FirstPass)
	}
	if rep.N != 1 {
		t.Errorf("n = %d, want 1", rep.N)
	}
}

func TestComputeFirstPass_YieldNoLinkedDone_BriefsIsCouldNot_Check(t *testing.T) {
	rep := computeFirstPassYield([]string{"s/01"}, map[string]bfPRRecord{}, 0,
		func(pr int) ([]ghReview, error) { return nil, nil }, map[string]string{}, nil)
	if rep.State != "could-not-check" {
		t.Errorf("state = %q, want could-not-check", rep.State)
	}
}
