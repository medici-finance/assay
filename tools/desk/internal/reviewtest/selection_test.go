package reviewtest_test

import (
	"encoding/json"
	rf "github.com/medici-finance/assay/tools/desk/internal/reviewflow"
	"reflect"
	"strings"
	"testing"
	"time"
)

func selectionFixture(t *testing.T) (rf.Observation, rf.ObservationPolicy, rf.SelectionPolicy) {
	t.Helper()
	in, fresh := observationFixture()
	base := in.Changes[0]
	other := in.Admitted[0]
	other.Repository = "example-org/other"
	in.Requested = append(in.Requested, other)
	in.Admitted = append(in.Admitted, other)
	in.Changes = nil
	for _, n := range []int{1, 2, 3, 4, 105} {
		c := base
		c.Identity.Number = n
		c.Lanes = append([]rf.LaneConclusion{}, base.Lanes...)
		c.Score = 1000 - n
		if n == 2 {
			c.Identity.Repository = other
		}
		if n == 4 {
			c.Facts.Reviews = rf.ReduceReviews([]rf.ReviewFact{{Reviewer: true, State: "CHANGES_REQUESTED", CommitID: c.Identity.Head}}, c.Identity.Head)
			c.Lanes[0].State = "blocked"
		}
		in.Changes = append(in.Changes, c)
	}
	in.Population[0].Count = 4
	in.Population = append(in.Population, rf.PopulationCoverage{Repository: other, Input: in.Population[0].Input, Count: 1})
	entries := []rf.PriorityEntry{}
	for _, c := range in.Changes {
		if c.Identity.Number == 3 {
			continue
		}
		score := 0
		switch c.Identity.Number {
		case 1:
			score = 10
		case 2:
			score = 2000
		case 4:
			score = 99999
		case 105:
			score = 3000
		}
		entries = append(entries, rf.PriorityEntry{Subject: rf.PrioritySubject{Repository: c.Identity.Repository, Number: c.Identity.Number}, Score: score})
	}
	entries = append(entries, rf.PriorityEntry{Subject: rf.PrioritySubject{Repository: in.Admitted[0], Number: 999}, Score: 999999})
	return observation(t, in, fresh), fresh, rf.SelectionPolicy{Owner: "approved-fixture-policy-owner", Revision: "priorities-r1", Evaluator: rf.SelectionEvaluator, Unmapped: rf.UseObservedScore, Priorities: entries}
}
func selectedNumbers(p rf.SelectionPlan) []int {
	out := []int{}
	for _, c := range p.Candidates {
		if c.Disposition == rf.SelectionSelected {
			out = append(out, c.Row.Identity.Number)
		}
	}
	return out
}
func decision(t *testing.T, p rf.SelectionPlan, n int) rf.CandidateDecision {
	t.Helper()
	for _, c := range p.Candidates {
		if c.Row.Identity.Number == n {
			return c
		}
	}
	t.Fatalf("candidate %d disappeared", n)
	return rf.CandidateDecision{}
}
func selection(t *testing.T, o rf.Observation, f rf.ObservationPolicy, p rf.SelectionPolicy, n int) rf.SelectionPlan {
	t.Helper()
	got, e := rf.PlanCandidates(o, f, p, n)
	if e != nil {
		t.Fatal(e)
	}
	return got
}
func TestReviewPriorityJoin(t *testing.T) {
	t.Run("priority-not-coalescing-key", func(t *testing.T) {
		o, f, p := selectionFixture(t)
		got := selection(t, o, f, p, 2)
		// Literal oracle, independent of the implementation's comparator and coalescer.
		if !reflect.DeepEqual(selectedNumbers(got), []int{105, 2}) {
			t.Fatalf("priority selection = %v; want [105 2]", selectedNumbers(got))
		}
		if len(got.Candidates) != 5 || got.Depths.Total != 5 || len(got.NotObserved) != 1 || got.NotObserved[0].Subject.Number != 999 {
			t.Fatal("candidate/policy union coverage lost")
		}
		if decision(t, got, 4).Disposition != rf.SelectionHeld || len(decision(t, got, 4).Row.Holds) == 0 {
			t.Fatal("high priority bypassed domain hold")
		}
		if c := decision(t, got, 3); c.PriorityMapped || c.Disposition != rf.SelectionDeferred || !strings.Contains(c.Reason, "unmapped") {
			t.Fatal("unmapped candidate hidden", c)
		}
		c := decision(t, got, 2)
		if c.Proposal == nil || c.Proposal.Key.Head != o.Inputs.Changes[1].Identity.Head || len(c.Row.Lanes) != 2 || c.Proposal.Key.Host != "github.com" {
			t.Fatal("selected head/lane/forge binding lost")
		}
		if got.Role != "review" || got.PolicyOwner != p.Owner || got.PolicyRevision != p.Revision || got.Evaluator != p.Evaluator || got.Now != f.Now || got.Capacity != 2 || got.ObservationPolicy != o.Inputs.Policy {
			t.Fatal("missing plan binding")
		}
	})
	t.Run("policy-only-change-and-repeat", func(t *testing.T) {
		o, f, p := selectionFixture(t)
		before, _ := json.Marshal(o)
		first := selection(t, o, f, p, 1)
		p.Revision = "priorities-r2"
		p.Priorities[0].Score = 4000
		got := selection(t, o, f, p, 1)
		if !reflect.DeepEqual(selectedNumbers(first), []int{105}) || !reflect.DeepEqual(selectedNumbers(got), []int{1}) {
			t.Fatal("policy did not reselect on unchanged source")
		}
		for i := 0; i < 20; i++ {
			if !reflect.DeepEqual(got, selection(t, o, f, p, 1)) {
				t.Fatal("same inputs differ")
			}
		}
		after, _ := json.Marshal(o)
		if string(before) != string(after) {
			t.Fatal("selection mutated observation")
		}
		if !reflect.DeepEqual(got.Depths, first.Depths) {
			t.Fatal("priority changed source depths")
		}
	})
	t.Run("permutations", func(t *testing.T) {
		o, f, p := selectionFixture(t)
		want := selection(t, o, f, p, 2)
		for i, j := 0, len(p.Priorities)-1; i < j; i, j = i+1, j-1 {
			p.Priorities[i], p.Priorities[j] = p.Priorities[j], p.Priorities[i]
		}
		in := o.Inputs
		in.Changes = append([]rf.ObservedChange{}, in.Changes...)
		for i, j := 0, len(in.Changes)-1; i < j; i, j = i+1, j-1 {
			in.Changes[i], in.Changes[j] = in.Changes[j], in.Changes[i]
		}
		if got := selection(t, observation(t, in, f), f, p, 2); !reflect.DeepEqual(got, want) {
			t.Fatal("input/key order became priority")
		}
	})
	t.Run("explicit-default-or-hold", func(t *testing.T) {
		o, f, p := selectionFixture(t)
		p.Priorities = []rf.PriorityEntry{}
		got := selection(t, o, f, p, 5)
		if !reflect.DeepEqual(selectedNumbers(got), []int{1, 2, 3, 105}) {
			t.Fatal("existing default not retained", selectedNumbers(got))
		}
		p.Unmapped = rf.HoldUnmapped
		got = selection(t, o, f, p, 5)
		if len(selectedNumbers(got)) != 0 || len(got.Candidates) != 5 || got.Idle.State == rf.IdleYes {
			t.Fatal("unmapped hold lost work or became idle")
		}
		for _, c := range got.Candidates {
			if c.Disposition != rf.SelectionHeld || !strings.Contains(c.Reason, "unmapped") {
				t.Fatal("missing explicit hold", c)
			}
		}
	})
	t.Run("capacity-is-not-coverage-or-idle", func(t *testing.T) {
		o, f, p := selectionFixture(t)
		got := selection(t, o, f, p, 0)
		if len(selectedNumbers(got)) != 0 || len(got.Candidates) != 5 || got.Idle.State == rf.IdleYes {
			t.Fatal("zero slots hid backlog")
		}
		o.Inputs.Changes = o.Inputs.Changes[:2]
		_, e := rf.PlanCandidates(o, f, p, 2)
		expectInputError(t, e, rf.PartialError)
	})
	t.Run("pure-boundary", func(t *testing.T) {
		o, f, p := selectionFixture(t)
		want := selection(t, o, f, p, 2)
		t.Setenv("GH_TOKEN", "invalid-sentinel")
		t.Setenv("GITHUB_TOKEN", "invalid-sentinel")
		t.Setenv("HOME", t.TempDir())
		t.Setenv("PATH", t.TempDir())
		if got := selection(t, o, f, p, 2); !reflect.DeepEqual(got, want) {
			t.Fatal("environment changed pure result")
		}
		assertPureImports(t)
	})
	t.Run("refusals", func(t *testing.T) {
		tests := []struct {
			name string
			kind rf.ErrorKind
			edit func(*rf.Observation, *rf.ObservationPolicy, *rf.SelectionPolicy, *int)
		}{
			{"no-policy", rf.PartialError, func(o *rf.Observation, f *rf.ObservationPolicy, p *rf.SelectionPolicy, n *int) { p.Priorities = nil }},
			{"owner", rf.SchemaError, func(o *rf.Observation, f *rf.ObservationPolicy, p *rf.SelectionPolicy, n *int) { p.Owner = "" }},
			{"revision", rf.SchemaError, func(o *rf.Observation, f *rf.ObservationPolicy, p *rf.SelectionPolicy, n *int) { p.Revision = "" }},
			{"evaluator", rf.SchemaError, func(o *rf.Observation, f *rf.ObservationPolicy, p *rf.SelectionPolicy, n *int) { p.Evaluator = "v999" }},
			{"unmapped", rf.SchemaError, func(o *rf.Observation, f *rf.ObservationPolicy, p *rf.SelectionPolicy, n *int) { p.Unmapped = "" }},
			{"capacity", rf.SchemaError, func(o *rf.Observation, f *rf.ObservationPolicy, p *rf.SelectionPolicy, n *int) { *n = -1 }},
			{"duplicate", rf.SchemaError, func(o *rf.Observation, f *rf.ObservationPolicy, p *rf.SelectionPolicy, n *int) {
				p.Priorities = append(p.Priorities, p.Priorities[0])
			}},
			{"invalid-number", rf.SchemaError, func(o *rf.Observation, f *rf.ObservationPolicy, p *rf.SelectionPolicy, n *int) {
				p.Priorities[0].Subject.Number = 0
			}},
			{"alias", rf.SchemaError, func(o *rf.Observation, f *rf.ObservationPolicy, p *rf.SelectionPolicy, n *int) {
				p.Priorities[0].Subject.Repository.Host = "GITHUB.COM"
			}},
			{"missing-clock", rf.SchemaError, func(o *rf.Observation, f *rf.ObservationPolicy, p *rf.SelectionPolicy, n *int) { f.Now = time.Time{} }},
			{"expired", rf.StaleError, func(o *rf.Observation, f *rf.ObservationPolicy, p *rf.SelectionPolicy, n *int) { f.Oldest = f.Now }},
			{"missing-root", rf.PartialError, func(o *rf.Observation, f *rf.ObservationPolicy, p *rf.SelectionPolicy, n *int) {
				o.Inputs.Admitted = o.Inputs.Admitted[:1]
			}},
			{"moved-head", rf.StaleError, func(o *rf.Observation, f *rf.ObservationPolicy, p *rf.SelectionPolicy, n *int) {
				o.Inputs.Changes[0].Identity.Head = strings.Repeat("f", 40)
			}},
			{"tampered-result", rf.SchemaError, func(o *rf.Observation, f *rf.ObservationPolicy, p *rf.SelectionPolicy, n *int) {
				o.Result.Rows[0].Score++
			}},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				o, f, p := selectionFixture(t)
				n := 2
				tc.edit(&o, &f, &p, &n)
				_, e := rf.PlanCandidates(o, f, p, n)
				expectInputError(t, e, tc.kind)
			})
		}
	})
	t.Run("uncapped-and-qualified", func(t *testing.T) {
		in, fresh := observationFixture()
		base := in.Changes[0]
		in.Changes = []rf.ObservedChange{}
		for n := 1; n <= 110; n++ {
			c := base
			c.Identity.Number = n
			c.Score = 111 - n
			in.Changes = append(in.Changes, c)
		}
		in.Population[0].Count = 110
		other := base.Identity.Repository
		other.Host = "other.example"
		in.Requested = append(in.Requested, other)
		in.Admitted = append(in.Admitted, other)
		c := base
		c.Identity.Repository = other
		in.Changes = append(in.Changes, c)
		in.Population = append(in.Population, rf.PopulationCoverage{Repository: other, Input: in.Population[0].Input, Count: 1})
		p := rf.SelectionPolicy{Owner: "fixture-owner", Revision: "r1", Evaluator: rf.SelectionEvaluator, Unmapped: rf.UseObservedScore,
			Priorities: []rf.PriorityEntry{{Subject: rf.PrioritySubject{Repository: other, Number: 1}, Score: 900},
				{Subject: rf.PrioritySubject{Repository: base.Identity.Repository, Number: 110}, Score: 800}}}
		got := selection(t, observation(t, in, fresh), fresh, p, 2)
		if len(got.Candidates) != 111 || got.Depths.Total != 111 || !reflect.DeepEqual(selectedNumbers(got), []int{1, 110}) {
			t.Fatal("display cap or unqualified priority join dropped candidates", selectedNumbers(got))
		}
		if got.Candidates[0].Row.Identity.Repository.Host != other.Host {
			t.Fatal("host-qualified priority collapsed")
		}
		// A high-priority surfaced approval remains a watch obligation, never an operation.
		in.Changes[109].Lanes = append([]rf.LaneConclusion{}, in.Changes[109].Lanes...)
		approve(&in.Changes[109])
		got = selection(t, observation(t, in, fresh), fresh, p, 2)
		if d := decision(t, got, 110); d.Disposition != rf.SelectionWatch || d.Proposal != nil || len(d.Row.Holds) == 0 {
			t.Fatal("priority overrode a non-operation disposition")
		}
	})
	t.Run("unobserved-priority-is-not-idle", func(t *testing.T) {
		in, fresh := observationFixture()
		in.Changes = []rf.ObservedChange{}
		in.Population[0].Count = 0
		p := rf.SelectionPolicy{Owner: "fixture-owner", Revision: "r1", Evaluator: rf.SelectionEvaluator, Unmapped: rf.UseObservedScore,
			Priorities: []rf.PriorityEntry{{Subject: rf.PrioritySubject{Repository: in.Admitted[0], Number: 1}, Score: 1}}}
		got := selection(t, observation(t, in, fresh), fresh, p, 1)
		if got.Idle.State != rf.IdleCouldNotCheck || len(got.NotObserved) != 1 || len(got.Candidates) != 0 {
			t.Fatal("unobserved priority inferred idle or invented candidate")
		}
		p.Priorities = []rf.PriorityEntry{}
		if selection(t, observation(t, in, fresh), fresh, p, 0).Idle.State != rf.IdleYes {
			t.Fatal("measured empty lists cannot be idle")
		}
	})
	t.Run("equal-priority-qualified-tie", func(t *testing.T) {
		in, fresh := observationFixture()
		// Source scores put github.com first, opposite the selection tie-break.
		// The input order and source order therefore cannot accidentally prove the tie.
		in.Changes[0].Score = 1000
		other := in.Admitted[0]
		other.Host = "a.example"
		c := in.Changes[0]
		c.Identity.Repository = other
		c.Score = 1
		in.Changes = append(in.Changes, c)
		in.Requested = append(in.Requested, other)
		in.Admitted = append(in.Admitted, other)
		in.Population = append(in.Population, rf.PopulationCoverage{Repository: other, Input: in.Population[0].Input, Count: 1})
		p := rf.SelectionPolicy{Owner: "fixture-owner", Revision: "ties-r1", Evaluator: rf.SelectionEvaluator, Unmapped: rf.UseObservedScore,
			Priorities: []rf.PriorityEntry{
				{Subject: rf.PrioritySubject{Repository: in.Admitted[0], Number: 1}, Score: 50},
				{Subject: rf.PrioritySubject{Repository: other, Number: 1}, Score: 50}}}
		for pass := 0; pass < 2; pass++ {
			got := selection(t, observation(t, in, fresh), fresh, p, 1)
			hosts := []string{}
			for _, d := range got.Candidates {
				hosts = append(hosts, d.Row.Identity.Repository.Host)
			}
			if !reflect.DeepEqual(hosts, []string{"a.example", "github.com"}) || got.Candidates[0].Disposition != rf.SelectionSelected || got.Candidates[1].Disposition != rf.SelectionDeferred {
				t.Fatalf("tie selection: hosts=%v decisions=%+v", hosts, got.Candidates)
			}
			in.Changes[0], in.Changes[1] = in.Changes[1], in.Changes[0]
			p.Priorities[0], p.Priorities[1] = p.Priorities[1], p.Priorities[0]
		}
	})
	t.Run("tie-break-mutations", func(t *testing.T) {
		for _, replacement := range []string{"return false", "return observationRowID(a.Row) > observationRowID(b.Row)"} {
			contractMutation(t, "internal/reviewflow/selection.go", "return observationRowID(a.Row) < observationRowID(b.Row)", replacement, "^TestReviewPriorityJoin$/^equal-priority-qualified-tie$", "tie selection")
		}
	})
	t.Run("out-of-scope-priority-gap", func(t *testing.T) {
		o, f, p := selectionFixture(t)
		outside := p.Priorities[0]
		outside.Subject.Repository.Host = "outside.example"
		p.Priorities = append(p.Priorities, outside)
		got := selection(t, o, f, p, 2)
		found := false
		for _, gap := range got.NotObserved {
			if gap.Subject == outside.Subject {
				found = true
				if !strings.Contains(gap.Reason, "outside the admitted") {
					t.Fatal("out-of-scope gap mislabeled", gap)
				}
			}
		}
		if !found || len(got.Candidates) != 5 {
			t.Fatal("outside priority disappeared or invented candidate")
		}
	})
	t.Run("production-mutation", func(t *testing.T) {
		contractMutation(t, "internal/reviewflow/selection.go", "return a.Priority > b.Priority", "return a.Priority < b.Priority", "^TestReviewPriorityJoin$/^priority-not-coalescing-key$", "priority selection")
	})
}
