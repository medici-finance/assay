package main

import (
	"errors"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestPendingChangePreparationKeepsTrustAndFreshHeads(t *testing.T) {
	v, f, _, brief := pendingSetup(t)
	s := v.Issues.(*forgeIssueSource)
	repo := deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}
	items, err := v.SelectQueue()
	if err != nil {
		t.Fatal(err)
	}
	var itemIndex int
	for i, it := range items {
		if it.BriefPath == brief {
			itemIndex = i
		}
	}
	item := items[itemIndex]
	item.Payload = map[string]string{}
	prepare := func(fail bool) {
		s.beginPlanningPass()
		cache := s.pendingChanges(f, repo)
		// A look-alike/foreign author never enters the read population.
		cache.list.Changes = append(cache.list.Changes, deskkit.ChangeRef{Number: 99, Author: deskkit.Account{Login: "stranger"}})
		preparePendingChangeReads(cache, repo, "example-verifier[bot]", func(r deskkit.ForgeRepo, ns []int) map[int]deskkit.PendingChangeObservation {
			if r != repo || len(ns) != 1 || ns[0] != 51 {
				t.Fatalf("untrusted target reached batch: %v %v", r, ns)
			}
			pr, files, e := deskkit.CompleteChangedFilesOf(f, repo, 51)
			if fail {
				e = errors.New("batch unavailable")
			}
			return map[int]deskkit.PendingChangeObservation{51: {PR: pr, Files: files, Err: e}}
		})
	}
	prepare(false)
	before := f.prReads[51]
	for range 2 {
		ref, e := s.PendingEvidence(v.Root, v.RootRepo, item, v.wakeNow(), nil)
		if e != nil || ref != "#51" {
			t.Fatalf("prepared hold %q %v", ref, e)
		}
	}
	if f.prReads[51] != before+2 {
		t.Fatal("prepared metadata suppressed consequential fresh head checks")
	}
	f.move = true
	if ref, e := s.PendingEvidence(v.Root, v.RootRepo, item, v.wakeNow(), nil); e == nil || ref != "" {
		t.Fatal("moved head retained prepared hold")
	}
	f.move = false
	prepare(true)
	before = f.prReads[51]
	for range 2 {
		if _, e := s.PendingEvidence(v.Root, v.RootRepo, item, v.wakeNow(), nil); e == nil {
			t.Fatal("prepared failure became absence")
		}
	}
	if f.prReads[51] != before {
		t.Fatal("failed batch was retried through a lazy side door")
	}
	prepare(false)
	if ref, e := s.PendingEvidence(v.Root, v.RootRepo, item, v.wakeNow(), nil); e != nil || ref != "#51" {
		t.Fatal("fresh pass did not recover")
	}
}
func TestPendingChangePreparationDoesNotGrantTrust(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*deskkit.PullRequest)
	}{
		{"recycled-id", func(p *deskkit.PullRequest) { p.Author.ID = 999 }},
		{"foreign-author", func(p *deskkit.PullRequest) { p.Author.Login = "stranger" }},
		{"closed", func(p *deskkit.PullRequest) { p.State = "closed" }},
		{"merged", func(p *deskkit.PullRequest) { p.Merged = true }},
		{"wrong-base", func(p *deskkit.PullRequest) { p.BaseRef = "elsewhere" }},
		{"fork", func(p *deskkit.PullRequest) { p.CrossRepo = deskkit.CrossRepoFork }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, f, _, _ := pendingSetup(t)
			s := v.Issues.(*forgeIssueSource)
			repo := deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}
			items, e := v.SelectQueue()
			if e != nil {
				t.Fatal(e)
			}
			item := items[0]
			item.Payload = map[string]string{}
			s.beginPlanningPass()
			cache := s.pendingChanges(f, repo)
			preparePendingChangeReads(cache, repo, "example-verifier[bot]", func(deskkit.ForgeRepo, []int) map[int]deskkit.PendingChangeObservation {
				pr, files, e := deskkit.CompleteChangedFilesOf(f, repo, 51)
				tc.change(pr)
				return map[int]deskkit.PendingChangeObservation{51: {PR: pr, Files: files, Err: e}}
			})
			if ref, e := s.PendingEvidence(v.Root, v.RootRepo, item, v.wakeNow(), nil); e == nil || ref != "" {
				t.Fatalf("untrusted prepared hold %q %v", ref, e)
			}
		})
	}
}
func TestPendingChangePreparationMissingResult(t *testing.T) {
	v, f, _, _ := pendingSetup(t)
	s := v.Issues.(*forgeIssueSource)
	repo := deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}
	cache := s.pendingChanges(f, repo)
	preparePendingChangeReads(cache, repo, "example-verifier[bot]", func(deskkit.ForgeRepo, []int) map[int]deskkit.PendingChangeObservation { return nil })
	if cache.reads[51] == nil || cache.reads[51].err == nil {
		t.Fatal("missing batch target became absence")
	}
}
