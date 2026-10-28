package deskkit

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type decisionSweepFixture struct {
	mu      sync.Mutex
	issues  map[int]*Issue
	search  []IssueSearchResult
	events  map[int][]LabelEvent
	trust   map[int]*TrustPayload
	failed  string
	failure error
	calls   map[string]int
}

func newDecisionSweepFixture() *decisionSweepFixture {
	f := &decisionSweepFixture{issues: map[int]*Issue{}, events: map[int][]LabelEvent{}, trust: map[int]*TrustPayload{}, calls: map[string]int{}}
	for i := 1; i <= 8; i++ {
		body := fmt.Sprintf("<!-- decision-gate: s/%02d -->", i)
		f.issues[i] = &Issue{Number: i, State: "open", Labels: []string{DecisionLabel}, Body: body}
		f.search = append(f.search, IssueSearchResult{Number: i, Body: body})
		f.events[i] = []LabelEvent{{Name: DecisionLabel, CreatedAt: "2026-10-01T10:00:00Z"}}
		f.trust[i] = &TrustPayload{Complete: true}
	}
	f.trust[2].Events = []ContentEvent{{Author: "ratifier", AuthorID: 7, CreatedAt: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)}}
	f.trust[3].Complete = false
	f.issues[4].State = "closed"
	f.issues[5].Labels = []string{DecisionLabel, DecidedLabel}
	f.trust[6].Events = []ContentEvent{{Author: "ratifier", AuthorID: 99, CreatedAt: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)}}
	f.events[7] = append(f.events[7], LabelEvent{Name: DecisionLabel, Removed: true}, LabelEvent{Name: DecisionLabel, CreatedAt: "2026-10-03T10:00:00Z"})
	f.trust[7].Events = f.trust[2].Events // ruling predates re-application
	f.events[8] = nil                     // no dated application means unknown
	return f
}

func (f *decisionSweepFixture) call(key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[key]++
	if f.failed == key {
		if f.failure != nil {
			return f.failure
		}
		return errors.New("fixture unavailable")
	}
	return nil
}

// regression: medici-finance/assay#3373
func TestDecisionSweepFailureCategories(t *testing.T) {
	for _, c := range []struct {
		err  error
		want string
	}{
		{Unverifiable("not-for-display", errDecisionSearchCap), "result limit reached"},
		{Unverifiable("not-for-display", errDecisionSearchIncomplete), "response or pagination incomplete"},
		{&ForgeAPIError{Status: 403, Body: "not-for-display"}, "authentication or authorization failed"},
		{&ForgeAPIError{Status: 429, RateLimited: true, Body: "not-for-display"}, "provider rate limit"},
		{errors.New("not-for-display"), "transport or response unavailable"},
		{Refused("not-for-display"), "decision query refused locally"},
	} {
		f := newDecisionSweepFixture()
		f.failed = "search"
		f.failure = c.err
		b := BriefAdmission{ID: "s/01"}
		got := NewDecisionSweep(f, forgeTestRepo, sweepRatifier).Wait(b)
		if got.Waiting || !strings.Contains(got.Unknown, c.want) || strings.Contains(got.Unknown, "not-for-display") {
			t.Fatalf("cause lost or leaked: %+v want %s", got, c.want)
		}
		if want := BriefDecisionWait(f, forgeTestRepo, b, sweepRatifier); got != want {
			t.Fatalf("single and shared causes differ: %+v %+v", got, want)
		}
	}
}
func (f *decisionSweepFixture) GetIssue(_ ForgeRepo, n int) (*Issue, error) {
	if err := f.call(fmt.Sprintf("issue:%d", n)); err != nil {
		return nil, err
	}
	return f.issues[n], nil
}
func (f *decisionSweepFixture) SearchIssues(_ ForgeRepo, in SearchIssuesInput) ([]IssueSearchResult, error) {
	if err := f.call("search"); err != nil {
		return f.search, err
	} // partial content must be discarded
	if in.DecisionGateMarkers && in.DecisionGateBrief == "" {
		return f.search, nil
	}
	var hits []IssueSearchResult
	for _, hit := range f.search {
		for _, m := range decisionMarkerRe.FindAllStringSubmatch(hit.Body, -1) {
			if (in.DecisionGateMarkers && in.DecisionGateBrief == m[1]) || (!in.DecisionGateMarkers && strings.Contains(in.Query, m[1])) {
				hits = append(hits, hit)
				break
			}
		}
	}
	return hits, nil
}
func (f *decisionSweepFixture) ListIssueLabelEvents(_ ForgeRepo, n int) ([]LabelEvent, error) {
	if err := f.call(fmt.Sprintf("labels:%d", n)); err != nil {
		return nil, err
	}
	return f.events[n], nil
}
func (f *decisionSweepFixture) IssueTrustEvents(_ ForgeRepo, n int) (*TrustPayload, error) {
	if err := f.call(fmt.Sprintf("trust:%d", n)); err != nil {
		return nil, err
	}
	return f.trust[n], nil
}
func sweepRatifier(login string, id int64) bool { return login == "ratifier" && id == 7 }

// regression: medici-finance/assay#3373
func TestDecisionSweepCanonicalParity(t *testing.T) {
	for _, failed := range []string{"", "search", "issue:1", "labels:1", "trust:1"} {
		t.Run(failed, func(t *testing.T) {
			f := newDecisionSweepFixture()
			f.failed = failed
			sweep := NewDecisionSweep(f, forgeTestRepo, sweepRatifier)
			for i := 1; i <= 8; i++ {
				b := BriefAdmission{ID: fmt.Sprintf("s/%02d", i)}
				got := sweep.Wait(b)
				plain := newDecisionSweepFixture()
				plain.failed = failed
				want := BriefDecisionWait(plain, forgeTestRepo, b, sweepRatifier)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("brief %+v: shared=%+v plain=%+v", b, got, want)
				}
			}
			if f.calls["search"] != 1 {
				t.Fatal("discovery was not shared", f.calls)
			}
		})
	}
}

// regression: medici-finance/assay#3373
func TestDecisionSweepCoalescesAndRefreshes(t *testing.T) {
	f := newDecisionSweepFixture()
	sweep := NewDecisionSweep(f, forgeTestRepo, sweepRatifier)
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := sweep.Wait(BriefAdmission{ID: "s/01"}); !got.Waiting {
				t.Errorf("lost hold: %+v", got)
			}
		}()
	}
	wg.Wait()
	for _, key := range []string{"search", "issue:1", "labels:1", "trust:1"} {
		if f.calls[key] != 1 {
			t.Fatalf("%s calls=%d, want one", key, f.calls[key])
		}
	}
	// A fresh pass must see a new ruling, even with unchanged local brief bytes.
	f.trust[1] = f.trust[2]
	if got := NewDecisionSweep(f, forgeTestRepo, sweepRatifier).Wait(BriefAdmission{ID: "s/01"}); got.Waiting || got.Unknown != "" {
		t.Fatal("stale ruling crossed sweeps", got)
	}
	if f.calls["search"] != 2 || f.calls["trust:1"] != 2 {
		t.Fatal("fresh pass reused prior observations", f.calls)
	}
}

// regression: medici-finance/assay#3373
func TestDecisionSweepMarkerMembership(t *testing.T) {
	f := newDecisionSweepFixture()
	f.issues[2].Body = "<!-- decision-gate: s/01 -->"
	f.search = []IssueSearchResult{{Number: 2, Body: "<!-- decision-gate: s/01 -->"}, {Number: 1, Body: "<!-- decision-gate: assay:tracker:s:01 -->"}}
	// Lowest issue wins regardless of discovery order or brief-id spelling.
	s := NewDecisionSweep(f, forgeTestRepo, sweepRatifier)
	if got := s.Wait(BriefAdmission{ID: "assay:tracker:s:01"}); got.Issue != 1 || !got.Waiting {
		t.Fatal(got)
	}
	// Search's stale body cannot override current membership. Next candidate's
	// fresh body determines the match, rather than its discovery position.
	f.issues[1].Body = "<!-- decision-gate: s/09 -->"
	f.issues[2].Body = "<!-- decision-gate: s/01 -->"
	if got := NewDecisionSweep(f, forgeTestRepo, sweepRatifier).Wait(BriefAdmission{ID: "assay:tracker:s:01"}); got.Issue != 2 || got.Waiting {
		t.Fatal("search body became authority", got)
	}
	f.failed = "search"
	if got := NewDecisionSweep(f, forgeTestRepo, sweepRatifier).Wait(BriefAdmission{DecisionIssue: 1}); !got.Waiting {
		t.Fatal("explicit ref depends on marker search", got)
	}
	if got := NewDecisionSweep(f, forgeTestRepo, nil).Wait(BriefAdmission{DecisionIssue: 1}); got.Unknown == "" || got.Waiting {
		t.Fatal("missing ratifier treated as a ruling", got)
	}
}

// regression: medici-finance/assay#3373
func TestDecisionSweepCandidateSpellingParity(t *testing.T) {
	for _, id := range []string{"s/01", "assay:tracker:s:01"} {
		for _, foreignClosed := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/foreignClosed=%v", id, foreignClosed), func(t *testing.T) {
				f := newDecisionSweepFixture()
				foreign := "<!-- decision-gate: assay:otherproj:s:01 -->"
				own := "<!-- decision-gate: " + id + " -->"
				f.search = []IssueSearchResult{{Number: 1, Body: foreign}, {Number: 2, Body: own}}
				f.issues[1].Body, f.issues[2].Body = foreign, own
				f.trust[2] = &TrustPayload{Complete: true}
				if foreignClosed {
					f.issues[1].State = "closed"
				} else {
					f.issues[2].State = "closed"
				}
				b := BriefAdmission{ID: id}
				want := BriefDecisionWait(f, forgeTestRepo, b, sweepRatifier)
				got := NewDecisionSweep(f, forgeTestRepo, sweepRatifier).Wait(b)
				if got != want || got.Issue != 2 || got.Waiting != foreignClosed || got.Unknown != "" {
					t.Fatalf("foreign project altered candidate: shared=%+v dispatcher=%+v", got, want)
				}
			})
		}
	}
	// A legacy brief searches only its literal slash spelling. A colon marker
	// must not become a reverse alias that the per-brief reader never queried.
	f := newDecisionSweepFixture()
	f.search = []IssueSearchResult{{Number: 1, Body: "<!-- decision-gate: assay:tracker:s:01 -->"}}
	if got := NewDecisionSweep(f, forgeTestRepo, sweepRatifier).Wait(BriefAdmission{ID: "s/01"}); got != (DecisionWait{}) {
		t.Fatal("legacy id gained an unqueried reverse alias", got)
	}
}

// regression: medici-finance/assay#3373
func TestDecisionSweepRequiredOutcomes(t *testing.T) {
	f := newDecisionSweepFixture()
	s := NewDecisionSweep(f, forgeTestRepo, sweepRatifier)
	for _, c := range []struct {
		issue         int
		held, unknown bool
	}{
		{1, true, false}, {2, false, false}, {3, false, true}, {4, false, false},
		{5, false, false}, {6, true, false}, {7, true, false}, {8, false, true},
	} {
		got := s.Wait(BriefAdmission{DecisionIssue: c.issue})
		if got.Waiting != c.held || (got.Unknown != "") != c.unknown {
			t.Errorf("issue %d: got %+v, held=%v unknown=%v", c.issue, got, c.held, c.unknown)
		}
	}
	if f.calls["search"] != 0 {
		t.Fatal("explicit refs performed marker discovery", f.calls)
	}
}

// regression: medici-finance/assay#3373
func TestDecisionSweepRepoBinding(t *testing.T) {
	f := newDecisionSweepFixture()
	s := NewDecisionSweep(f, forgeTestRepo, sweepRatifier)
	other := ForgeRepo{Owner: "other", Name: "tracker"}
	if _, err := s.GetIssue(other, 1); err == nil {
		t.Fatal("cross-repo issue read")
	}
	if _, err := s.SearchIssues(other, SearchIssuesInput{}); err == nil {
		t.Fatal("cross-repo search")
	}
	if _, err := s.ListIssueLabelEvents(other, 1); err == nil {
		t.Fatal("cross-repo history")
	}
	if _, err := s.IssueTrustEvents(other, 1); err == nil {
		t.Fatal("cross-repo trust")
	}
	if len(f.calls) != 0 {
		t.Fatal("wrong-repo read reached transport", f.calls)
	}
}
