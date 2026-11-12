package evaluation

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"
)

const ms07P = "cell:repo:cap:m-01@1"
const ms07M = "cell:repo:build:m-02@1"
const ms07B = "cell:repo:cap:b-01"

func ms07Subject() ReadinessSubject {
	return ReadinessSubject{Token: "exact-artifact-and-configuration", Environment: "production", Interface: "api-1", Profile: "batch"}
}

func ms07Input(status string) EligibilityInput {
	g := BuildMilestoneGraph(MilestoneGraphInput{
		Briefs: []GraphBrief{{Key: "b11"}, {Key: "b12", Requires: []MilestonePin{{Key: "cell:repo:cap:m-01", Version: 1, Spelling: "cap:m-01@1"}}}},
		Milestones: []GraphMilestone{
			{Key: "cell:repo:cap:m-01", Version: 1, Contributes: []WorkPin{{Key: ms07B}}},
			{Key: "cell:repo:build:m-02", Version: 1, Contributes: []WorkPin{{Key: "b11"}}, Requires: []MilestonePin{{Key: "cell:repo:cap:m-01", Version: 1}}},
		},
		Lookup: func(p WorkPin) PinObservation {
			s := status
			if p.Key == "b11" {
				s = "done"
			}
			return PinObservation{Found: true, Brief: GraphBrief{Key: p.Key, Status: s}}
		},
	})
	return EligibilityInput{Streams: []Stream{{Name: "build", Briefs: []Brief{{Key: "b11", Num: "11", Status: "todo"}, {Key: "b12", Num: "12", Status: "todo"}}}}, Milestones: &MilestoneInput{Graph: g, Subjects: map[string]ReadinessSubject{ms07P: ms07Subject(), ms07M: ms07Subject()}}}
}

func ms07Accepted(ref string) AcceptanceObservation {
	return AcceptanceObservation{Milestone: ref, Subject: ms07Subject(), State: AcceptanceAccepted, Availability: AvailabilitySupported, HistoryCount: 1}
}

// regression: #6114
func TestMilestoneEligibility(t *testing.T) {
	t.Run("exact acceptance", func(t *testing.T) {
		var cases []struct {
			Name         string
			State        AcceptanceState
			Availability Availability
			Eligible     bool
		}
		raw, err := os.ReadFile("../testdata/milestone-support/07/acceptance.json")
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &cases); err != nil {
			t.Fatal(err)
		}
		for _, tc := range cases {
			t.Run(tc.Name, func(t *testing.T) {
				in := ms07Input("done")
				a := ms07Accepted(ms07P)
				a.State, a.Availability = tc.State, tc.Availability
				switch tc.Name {
				case "wrong-version":
					a.Milestone = "cell:repo:cap:m-01@2"
				case "wrong-subject":
					a.Subject.Token = "other-artifact"
				case "wrong-profile":
					a.Subject.Profile = "interactive"
				case "unknown-revocation":
					a.Unknowns = []string{"decision-coverage"}
				}
				in.Milestones.Acceptance = []AcceptanceObservation{a}
				if tc.Name == "missing" {
					in.Milestones.Acceptance = nil
				}
				if tc.Name == "ambiguous" {
					in.Milestones.Acceptance = append(in.Milestones.Acceptance, a)
				}
				out := EvaluateReadiness(in)
				if got := out.Eligibility["build/12"].Verdict; (got == VerdictEligible) != tc.Eligible || got == VerdictEligibleWithNotice {
					t.Fatalf("start verdict %s, eligible want %v", got, tc.Eligible)
				}
				if out.Eligibility["build/11"].Verdict != VerdictEligible {
					t.Fatal("final-only prerequisite inherited by contributor")
				}
				if out.Milestones[ms07P].AcceptedForSubject != tc.Eligible {
					t.Fatalf("accepted-for-subject: %+v", out.Milestones[ms07P])
				}
				if out.Milestones[ms07M].AcceptanceOutstanding != tc.Eligible {
					t.Fatalf("prerequisite acceptance required for completion: %+v", out.Milestones[ms07M])
				}
				if tc.Eligible && (out.Milestones[ms07P].HistoryCount != 1 || out.Milestones[ms07P].Availability != AvailabilitySupported) {
					t.Fatal("positive acceptance lost")
				}
			})
		}
	})
	t.Run("remaining and coverage", func(t *testing.T) {
		for _, status := range []string{"todo", "implemented", "verified", "withdrawn", ""} {
			in := ms07Input(status)
			in.Milestones.Acceptance = []AcceptanceObservation{ms07Accepted(ms07P)}
			out := EvaluateReadiness(in)
			r := out.Milestones[ms07M]
			if r.WorkComplete || r.AcceptanceOutstanding || out.Eligibility["build/12"].Verdict != VerdictHeld {
				t.Fatalf("%q completed or released: %+v", status, r)
			}
			if status != "" && (r.WorkRemaining != 1 || !slices.Contains(r.Remaining, ms07B)) {
				t.Fatalf("%s prerequisite work count: %+v", status, r)
			}
			if status == "" && !r.IncompleteCoverage {
				t.Fatal("unknown status lost")
			}
		}
		in := ms07Input("done")
		n := in.Milestones.Graph.Milestones[ms07P]
		n.Work[0].Found = false
		in.Milestones.Graph.Milestones[ms07P] = n
		in.Milestones.Acceptance = []AcceptanceObservation{ms07Accepted(ms07P)}
		out := EvaluateReadiness(in)
		if !out.Coverage.Incomplete || out.Milestones[ms07M].WorkComplete || out.Eligibility["build/12"].Verdict != VerdictHeld {
			t.Fatal("unread contributor promoted")
		}
		delete(in.Milestones.Graph.Milestones, ms07P)
		if out := EvaluateReadiness(in); !out.Coverage.Incomplete || out.Eligibility["build/12"].Verdict != VerdictHeld {
			t.Fatal("unread record promoted")
		}
	})
	t.Run("deduplicated closure", func(t *testing.T) {
		in := ms07Input("verified")
		n := in.Milestones.Graph.Milestones[ms07M]
		n.Work = append(n.Work, in.Milestones.Graph.Milestones[ms07P].Work...)
		n.Contributors = append(n.Contributors, ms07B)
		q := "cell:repo:cap:m-03@1"
		pn := in.Milestones.Graph.Milestones[ms07P]
		pn.Ref = q
		in.Milestones.Graph.Milestones[q] = pn
		n.Prerequisites = append(n.Prerequisites, q)
		in.Milestones.Graph.Milestones[ms07M] = n
		out := EvaluateReadiness(in)
		if r := out.Milestones[ms07M]; r.WorkRemaining != 1 || !slices.Equal(r.Remaining, []string{ms07B}) {
			t.Fatalf("shared prerequisite counted more than once: %+v", r)
		}
	})
	t.Run("accepted prerequisite heads", func(t *testing.T) {
		in := ms07Input("done")
		q := "cell:repo:cap:m-03@1"
		in.Milestones.Graph.Milestones[q] = MilestoneNode{Ref: q, Completion: GraphComplete}
		p := in.Milestones.Graph.Milestones[ms07P]
		p.Prerequisites = []string{q}
		p.Completion = GraphUnknown
		in.Milestones.Graph.Milestones[ms07P] = p
		in.Milestones.Subjects[q] = ms07Subject()
		in.Milestones.Acceptance = []AcceptanceObservation{ms07Accepted(ms07P), ms07Accepted(q)}
		r := EvaluateReadiness(in).Milestones[ms07M]
		if !r.AcceptanceOutstanding || len(r.BlockingHeads) != 0 {
			t.Fatalf("accepted prerequisites remain heads: %+v", r)
		}
		in.Milestones.Acceptance = in.Milestones.Acceptance[:1]
		r = EvaluateReadiness(in).Milestones[ms07M]
		if r.WorkComplete || !slices.Contains(r.BlockingHeads, ms07P) {
			t.Fatalf("nearest unaccepted head missing: %+v", r)
		}
	})
	t.Run("scope and cycles", func(t *testing.T) {
		in := ms07Input("done")
		in.Milestones.Acceptance = []AcceptanceObservation{ms07Accepted(ms07P)}
		s := in.Milestones.Subjects[ms07M]
		s.Environment = "staging"
		in.Milestones.Subjects[ms07M] = s
		if EvaluateReadiness(in).Milestones[ms07M].WorkComplete {
			t.Fatal("prerequisite accepted for different environment")
		}
		n := in.Milestones.Graph.Milestones[ms07P]
		n.Completion = GraphCycle
		n.Prerequisites = []string{ms07P}
		in.Milestones.Graph.Milestones[ms07P] = n
		if EvaluateReadiness(in).Eligibility["build/12"].Verdict != VerdictHeld {
			t.Fatal("cycle released")
		}
	})
	t.Run("all other holds survive", func(t *testing.T) {
		for _, reason := range []string{"human", "design", "risk", "verification", "parked"} {
			in := ms07Input("done")
			in.Milestones.Acceptance = []AcceptanceObservation{ms07Accepted(ms07P)}
			in.Streams[0].Briefs[1].Gates = []Edge{{Ref: "#42", Type: "behavioural-gate", Reason: reason, Resolved: StateUnsatisfied}}
			if got := EvaluateReadiness(in).Eligibility["build/12"]; got.Verdict != VerdictHeld || len(got.Holds) != 1 || got.Holds[0].Reason != reason {
				t.Fatalf("%s hold lost: %+v", reason, got)
			}
		}
		in := ms07Input("done")
		in.Milestones.Acceptance = []AcceptanceObservation{ms07Accepted(ms07P)}
		in.Streams[0].Briefs[1].Depends = []string{"other/01"}
		if EvaluateReadiness(in).Eligibility["build/12"].Verdict != VerdictHeld {
			t.Fatal("depends hold lost")
		}
	})
	t.Run("unresolved gates and legacy", func(t *testing.T) {
		in := EligibilityInput{Streams: []Stream{{Name: "legacy", Briefs: []Brief{{Num: "01"}, {Num: "02", Gates: []Edge{{Ref: "cap:m-01@1", Type: "behavioural-gate"}}}}}}}
		got := EvaluateEligibility(in)
		if got["legacy/02"].Verdict != VerdictHeld || len(got["legacy/02"].Holds) != 1 || got["legacy/02"].Holds[0].State != StateCouldNotCheck {
			t.Fatal("absent Resolved became ready")
		}
		raw, _ := json.Marshal(got)
		const want = `{"legacy/01":{"id":"legacy/01","verdict":"eligible"},"legacy/02":{"id":"legacy/02","verdict":"held","holds":[{"ref":"cap:m-01@1","type":"behavioural-gate","state":"could-not-check"}]}}`
		if string(raw) != want {
			t.Fatalf("legacy bytes changed: %s", raw)
		}
		with := ms07Input("done")
		before, _ := json.Marshal(with)
		_ = EvaluateReadiness(with)
		after, _ := json.Marshal(with)
		if !reflect.DeepEqual(before, after) {
			t.Fatal("caller observations mutated")
		}
	})
}
