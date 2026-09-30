package main

// query_test.go — fetchQueue's dedupe/rank/sort against a fake Forge, mirroring the
// fake-Forge harness cmd/issueboard's tests already use (the board reaches the forge
// through deskkit.ForgeFor, so tests inject a recorded fake via the forgeFor seam rather
// than a network).

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

type fakeForge struct {
	deskkit.Forge
	issues []deskkit.IssueSummary
	err    error
}

func (f *fakeForge) ListOpenIssues(deskkit.ForgeRepo) ([]deskkit.IssueSummary, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.issues, nil
}

func withForge(t *testing.T, byRepo map[string]*fakeForge) {
	t.Helper()
	prev := forgeFor
	forgeFor = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		owner, name, _ := splitRepo(repo)
		fr := deskkit.ForgeRepo{Owner: owner, Name: name}
		f, ok := byRepo[repo]
		if !ok {
			return &fakeForge{}, fr, nil
		}
		return f, fr, nil
	}
	t.Cleanup(func() { forgeFor = prev })
}

func TestFetchQueueRankAndAgeSort(t *testing.T) {
	withForge(t, map[string]*fakeForge{
		"example-org/example-repo": {issues: []deskkit.IssueSummary{
			{Number: 1, Title: "old question", Labels: []string{"question"}, CreatedAt: "2026-01-01T00:00:00Z"},
			{Number: 2, Title: "new urgent", Labels: []string{"urgent"}, CreatedAt: "2026-06-01T00:00:00Z"},
			{Number: 3, Title: "old urgent", Labels: []string{"urgent"}, CreatedAt: "2026-01-01T00:00:00Z"},
			{Number: 4, Title: "no escalation label", Labels: []string{"bug"}, CreatedAt: "2025-01-01T00:00:00Z"},
			{Number: 5, Title: "both urgent and needs-decision", Labels: []string{"needs-decision", "urgent"}, CreatedAt: "2026-03-01T00:00:00Z"},
		}},
	})

	items, failures := fetchQueue([]string{"example-org/example-repo"})
	if len(failures) != 0 {
		t.Fatalf("unexpected failures: %v", failures)
	}
	// #4 (no escalation label) must be excluded entirely.
	if len(items) != 4 {
		t.Fatalf("want 4 items (excluding the unlabeled issue), got %d: %+v", len(items), items)
	}
	// #5 carries BOTH needs-decision and urgent, so it ranks at 0 (the MINIMUM index over
	// its labels — urgent wins), tying it with #2 and #3; all three sort ahead of #1
	// (question, rank 2), oldest createdAt first within the tie.
	wantOrder := []int{3, 5, 2, 1}
	for i, w := range wantOrder {
		if items[i].Number != w {
			t.Errorf("position %d: want #%d, got #%d (full order: %v)", i, w, items[i].Number, numbersOf(items))
		}
	}
}

func numbersOf(items []item) []int {
	out := make([]int, len(items))
	for i, it := range items {
		out[i] = it.Number
	}
	return out
}

func TestFetchQueueDedupesRepeatedRepoArg(t *testing.T) {
	withForge(t, map[string]*fakeForge{
		"example-org/example-repo": {issues: []deskkit.IssueSummary{
			{Number: 1, Title: "one", Labels: []string{"urgent"}, CreatedAt: "2026-01-01T00:00:00Z"},
		}},
	})
	items, failures := fetchQueue([]string{"example-org/example-repo", "example-org/example-repo"})
	if len(failures) != 0 {
		t.Fatalf("unexpected failures: %v", failures)
	}
	if len(items) != 1 {
		t.Fatalf("want exactly 1 deduped item, got %d: %+v", len(items), items)
	}
}

func TestFetchQueueReportsRepoFailureWithoutAbortingOthers(t *testing.T) {
	withForge(t, map[string]*fakeForge{
		"example-org/good-repo": {issues: []deskkit.IssueSummary{
			{Number: 1, Title: "one", Labels: []string{"urgent"}, CreatedAt: "2026-01-01T00:00:00Z"},
		}},
		"example-org/bad-repo": {err: deskkit.Unverifiable("simulated read failure", nil)},
	})
	items, failures := fetchQueue([]string{"example-org/good-repo", "example-org/bad-repo"})
	if len(items) != 1 {
		t.Fatalf("want the good repo's item despite the other repo failing, got %d: %+v", len(items), items)
	}
	if len(failures) != 1 || failures[0].Repo != "example-org/bad-repo" {
		t.Fatalf("want exactly one failure naming example-org/bad-repo, got %+v", failures)
	}
}
