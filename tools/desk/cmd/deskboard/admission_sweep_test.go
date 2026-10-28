package main

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

type boundedDecisionFixture struct {
	boardDecisionForge
	active, peak, search, reads atomic.Int64
}

func (f *boundedDecisionFixture) GetIssue(repo deskkit.ForgeRepo, n int) (*deskkit.Issue, error) {
	a := f.active.Add(1)
	for old := f.peak.Load(); a > old; old = f.peak.Load() {
		if f.peak.CompareAndSwap(old, a) {
			break
		}
	}
	defer f.active.Add(-1)
	f.reads.Add(1)
	time.Sleep(time.Duration(13-n) * time.Millisecond)
	return f.boardDecisionForge.GetIssue(repo, n)
}
func (f *boundedDecisionFixture) SearchIssues(deskkit.ForgeRepo, deskkit.SearchIssuesInput) ([]deskkit.IssueSearchResult, error) {
	f.search.Add(1)
	var hits []deskkit.IssueSearchResult
	for n, item := range f.issues {
		hits = append(hits, deskkit.IssueSearchResult{Number: n, Body: item.Body})
	}
	return hits, nil
}

// regression: medici-finance/assay#3373
func TestBoardAdmissionSharedBoundedOrdered(t *testing.T) {
	briefs := map[string]string{}
	f := &boundedDecisionFixture{boardDecisionForge: boardDecisionForge{issues: map[int]*deskkit.Issue{}}}
	var nums []string
	for n := 1; n <= 12; n++ {
		num := fmt.Sprintf("%02d", n)
		nums = append(nums, num)
		briefs[num] = "gate: human\n"
		f.issues[n] = &deskkit.Issue{Number: n, State: "open", Labels: []string{deskkit.DecisionLabel}, Body: "<!-- decision-gate: s/" + num + " -->"}
	}
	root := admissionRoot(t, briefs)
	useBoardDecisions(t, &f.boardDecisionForge)
	var resolutions atomic.Int64
	decisionForgeFor = func(string) (deskkit.DecisionForge, deskkit.ForgeRepo, error) {
		resolutions.Add(1)
		return f, deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}, nil
	}
	rep := admissionReport(root, nums...)
	applyBriefAdmission(rep, "strong")
	if rep.HeldByDecision != 12 || len(rep.Rows) != 0 || len(rep.AdmissionUnknown) != 0 {
		t.Fatalf("outcomes changed: %+v", rep)
	}
	for i, h := range rep.AdmissionHeld {
		if h.Brief != "s/"+nums[i] {
			t.Fatal("finish order changed output", rep.AdmissionHeld)
		}
	}
	if resolutions.Load() != 1 || f.search.Load() != 1 || f.reads.Load() != 12 {
		t.Fatalf("duplicated observations: resolve=%d search=%d reads=%d", resolutions.Load(), f.search.Load(), f.reads.Load())
	}
	if peak := f.peak.Load(); peak < 2 || peak > sweepConcurrency {
		t.Fatalf("read concurrency=%d, want 2..%d", peak, sweepConcurrency)
	}
}

// regression: medici-finance/assay#3373
func TestBoardAdmissionTierBeforeDecision(t *testing.T) {
	root := admissionRoot(t, map[string]string{"01": "gate: human\nexec-tier: strong\n", "02": ""})
	useBoardDecisions(t, &boardDecisionForge{})
	decisionForgeFor = func(string) (deskkit.DecisionForge, deskkit.ForgeRepo, error) {
		t.Fatal("tier-held row reached forge")
		return nil, deskkit.ForgeRepo{}, nil
	}
	rep := admissionReport(root, "01", "02")
	applyBriefAdmission(rep, "any")
	if rowBriefs(rep) != "s/02" || rep.HeldByExecTier != 1 || !strings.Contains(rep.AdmissionHeld[0].Reason, "exec-tier") {
		t.Fatal(rep)
	}
}
