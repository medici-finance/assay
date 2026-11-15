package main

import (
	"reflect"
	"slices"
	"testing"

	"github.com/medici-finance/assay/statusgen/domain"
	"github.com/medici-finance/assay/statusgen/evaluation"
)

func ms07HasPick(n NextUp, id string) bool {
	for _, p := range n.Picks {
		if p.Stream.Name+"/"+p.Brief.Num == id {
			return true
		}
	}
	return false
}

// regression: #6114
func TestNextupMilestoneCoverage(t *testing.T) {
	t.Run("unadvertised and legacy", func(t *testing.T) {
		streams := ms05Load(t, ms05Tree(t, nil))
		checkBriefFiles(streams, streams)
		e := eligibilityForStreams(streams)["build/12"]
		if !hasHold(e, "stream-y:m-01@1", StateCouldNotCheck) || ms07HasPick(nextUp(streams, ClaimView{}, nil), "build/12") {
			t.Fatal("unadvertised requirement offered")
		}
		if !ms07HasPick(nextUp(streams, ClaimView{}, nil), "build/11") {
			t.Fatal("independent contributor hidden")
		}
		if streams[0].MilestoneReadiness == nil || !streams[0].MilestoneReadiness.Coverage.Incomplete {
			t.Fatal("unread root coverage lost")
		}
		_, legacy := eligibilityFixtureRoot(t)
		before := nextUp(legacy, ClaimView{}, nil)
		applyMilestoneStartGates(func(string, ...any) {}, func(string, ...any) {}, legacy, nil, nil, func(string) *graphRepos { return nil })
		if !reflect.DeepEqual(before, nextUp(legacy, ClaimView{}, nil)) {
			t.Fatal("non-milestone Next-up changed")
		}
	})
	for _, status := range []string{"todo", "withdrawn", "done"} {
		t.Run(status, func(t *testing.T) {
			advertiseMilestones(t)
			streams := ms05Load(t, ms05Tree(t, nil))
			ms05SetStatus(streams, status, "stream-y/01")
			for _, s := range streams {
				if s.Name == "stream-y" {
					s.Status = "parked"
				}
			}
			checkBriefFiles(streams, streams)
			n := nextUp(streams, ClaimView{}, nil)
			if ms07HasPick(n, "build/12") || ms07HasPick(n, "stream-y/01") {
				t.Fatal("missing acceptance or parked work offered")
			}
			r := streams[0].MilestoneReadiness
			if r == nil {
				t.Fatal("missing canonical readiness")
			}
			y := r.Milestones[ms05Y]
			if status != "done" && (y.WorkRemaining != 1 || !slices.Contains(y.BlockingHeads, "assay:assay:stream-y:b-01")) {
				t.Fatalf("required parked work not visible: %+v", y)
			}
			if status == "done" && (!y.AcceptanceOutstanding || !y.IncompleteCoverage) {
				t.Fatalf("missing acceptance conflated with completion: %+v", y)
			}
		})
	}
	t.Run("mapping", func(t *testing.T) {
		subject := domain.MilestoneSubject{Artifacts: []string{"artifact"}, Environment: "env", Configuration: "config", Interface: "api", Profile: "batch"}
		ref := "example:widgets:build:m-01@1"
		for _, state := range []evaluation.AcceptanceState{evaluation.AcceptanceAccepted, evaluation.AcceptanceRevoked, evaluation.AcceptanceSuperseded, evaluation.AcceptanceOutstanding, evaluation.AcceptanceUnknown} {
			a := domain.MilestoneAssessment{State: "supported", History: []domain.MilestoneHistory{{}}}
			switch state {
			case evaluation.AcceptanceRevoked:
				a.State = "blocked"
				a.History[0].Revoked = true
			case evaluation.AcceptanceSuperseded:
				a.State = "blocked"
				a.History[0].Superseded = true
			case evaluation.AcceptanceOutstanding:
				a.State = "unknown"
				a.History = nil
				a.AcceptanceOutstanding = true
				a.Unknowns = []string{"acceptance-outstanding"}
			case evaluation.AcceptanceUnknown:
				a.State = "unknown"
				a.Unknowns = []string{"decision-coverage"}
			}
			got := milestoneAcceptanceObservation(ref, subject, a)
			if got.State != state || got.HistoryCount != len(a.History) || got.Milestone != ref || got.Subject.Token == "" {
				t.Fatalf("mapping %s: %+v", state, got)
			}
			if state == evaluation.AcceptanceUnknown && !slices.Contains(got.Unknowns, "decision-coverage") {
				t.Fatal("unread revocation lost")
			}
		}
		for _, change := range []func(*domain.MilestoneSubject){func(s *domain.MilestoneSubject) { s.Configuration = "new" }, func(s *domain.MilestoneSubject) { s.Artifacts = []string{"other"} }, func(s *domain.MilestoneSubject) { s.Profile = "other" }} {
			changed := subject
			change(&changed)
			if milestoneSubject(changed) == milestoneSubject(subject) {
				t.Fatal("exact subject field discarded")
			}
		}
		a := readMilestoneAcceptance(milestoneAcceptanceRequest{}, nil, nil, nil)
		if milestoneAcceptanceObservation(ref, subject, a).State != evaluation.AcceptanceUnknown {
			t.Fatal("absent selector accepted")
		}
	})
	t.Run("accepted gate keeps host exclusions", func(t *testing.T) {
		ref := "example:widgets:build:m-01@1"
		f := msSetup(t)
		a := f.run(t)
		msSupported(t, a)
		observation := milestoneAcceptanceObservation(ref, f.req.Current.CurrentSubject, a)
		if observation.State != evaluation.AcceptanceAccepted || observation.HistoryCount != 1 {
			t.Fatalf("receipt mapping: %+v", observation)
		}
		// The lower consumer sees the actual assessment, not a test-only verdict.
		s := &Stream{Name: "consumer", Status: "active", Briefs: []Brief{{Num: "01", Status: "todo", Schema: "brief-v4", Effort: "M"}}}
		streams := []*Stream{s}
		row := &s.Briefs[0]
		graph := evaluation.BuildMilestoneGraph(evaluation.MilestoneGraphInput{Briefs: []evaluation.GraphBrief{{Key: "consumer", Requires: []evaluation.MilestonePin{{Key: "example:widgets:build:m-01", Version: 1, Spelling: ref}}}}, Milestones: []evaluation.GraphMilestone{{Key: "example:widgets:build:m-01", Version: 1}}})
		in := &evaluation.MilestoneInput{Graph: graph, Subjects: map[string]evaluation.ReadinessSubject{ref: observation.Subject}, Acceptance: []evaluation.AcceptanceObservation{observation}}
		pending := []milestoneStartPending{{stream: s, row: row, requires: []string{ref}}}
		applyMilestoneReadiness(func(string, ...any) {}, streams, pending, map[*Brief]string{row: "consumer"}, nil, in)
		if !ms07HasPick(nextUp(streams, ClaimView{}, nil), "consumer/01") {
			t.Fatal("exact acceptance did not reach Next-up")
		}
		for _, status := range []string{"parked", "paused"} {
			s.Status = status
			if ms07HasPick(nextUp(streams, ClaimView{}, nil), "consumer/01") {
				t.Fatal("inactive stream released")
			}
		}
		s.Status = "active"
		base := *row
		queue := "verification-debt"
		edits := []struct {
			name string
			edit func(*Brief)
		}{
			{"stale", func(b *Brief) { b.StaleRef = "finding" }},
			{"homed", func(b *Brief) { b.HomedIn = "example/elsewhere" }},
			{"sibling", func(b *Brief) { b.MergedInSibling = "example/elsewhere" }},
			{"phantom", func(b *Brief) { b.PhantomHeld = "missing" }},
			{"verification", func(b *Brief) { b.Status = "implemented" }},
			{"approval", func(b *Brief) { b.Status = "verified" }},
			{"human", func(b *Brief) {
				b.Gates = append(append([]GraphEdge(nil), b.Gates...), GraphEdge{Ref: "#17", Type: "behavioural-gate", Reason: "human", Resolved: StateUnsatisfied})
			}},
			{"placeholder", func(b *Brief) { b.Schema = "placeholder-v1"; b.Blocked = "awaiting" }},
			{"measures", func(b *Brief) { b.Measures = &queue }},
		}
		for _, tc := range edits {
			*row = base
			tc.edit(row)
			if eligible(streams, s, *row, nil, map[string]measuredQueue{queue: {Known: true, Breached: true}}, eligibilityForStreams(streams)) {
				t.Fatalf("%s exclusion lost", tc.name)
			}
		}
		*row = base
		if eligible(streams, s, *row, map[string]bool{"consumer/01": true}, nil, eligibilityForStreams(streams)) {
			t.Fatal("claimed work released")
		}
		// A second caller with no resolved value must still fail closed.
		row.Gates = []GraphEdge{{Ref: ref, Type: "behavioural-gate", Reason: "requires-milestones"}}
		if ms07HasPick(nextUp(streams, ClaimView{}, nil), "consumer/01") {
			t.Fatal("absent Resolved released second caller")
		}
		// Current subject differences, unread revocation and historical restrictions
		// are mapped without deriving a fresh acceptance in the host.
		for _, kind := range []string{"revoked", "superseded", "unknown", "outstanding"} {
			assessment := domain.MilestoneAssessment{State: "unknown", AcceptanceOutstanding: true, Unknowns: []string{"acceptance-outstanding"}}
			switch kind {
			case "revoked":
				assessment = domain.MilestoneAssessment{State: "blocked", History: []domain.MilestoneHistory{{Revoked: true}}}
			case "superseded":
				assessment = domain.MilestoneAssessment{State: "blocked", History: []domain.MilestoneHistory{{Superseded: true}}}
			case "unknown":
				assessment.Unknowns = append(assessment.Unknowns, "decision-coverage")
			}
			mapped := milestoneAcceptanceObservation(ref, f.req.Current.CurrentSubject, assessment)
			if string(mapped.State) != kind {
				t.Fatalf("%s mapped as %+v", kind, mapped)
			}
			in.Acceptance = []evaluation.AcceptanceObservation{mapped}
			row.Gates = nil
			applyMilestoneReadiness(func(string, ...any) {}, streams, pending, map[*Brief]string{row: "consumer"}, nil, in)
			if ms07HasPick(nextUp(streams, ClaimView{}, nil), "consumer/01") {
				t.Fatalf("%s offered", kind)
			}
		}
		changed := f.req.Current.CurrentSubject
		changed.Configuration = "other"
		if milestoneSubject(changed) == observation.Subject {
			t.Fatal("configuration dropped from subject")
		}
		missing := readMilestoneAcceptance(f.req, nil, nil, nil)
		if milestoneAcceptanceObservation(ref, f.req.Current.CurrentSubject, missing).State != evaluation.AcceptanceUnknown {
			t.Fatal("missing selector accepted")
		}
	})
}
