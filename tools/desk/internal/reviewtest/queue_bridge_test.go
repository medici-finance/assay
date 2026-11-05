package reviewtest_test

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	rf "github.com/medici-finance/assay/tools/desk/internal/reviewflow"
	q "github.com/medici-finance/assay/tools/desk/internal/reviewtest"
)

func collectQueue(t *testing.T, name string, size int) q.Response {
	t.Helper()
	got, _ := probe(t, build(t), "deskboard", q.Request{Case: "queue-" + name, Size: size}, t.TempDir())
	return got
}
func queueRequest(t *testing.T, raw json.RawMessage, capacity int) q.Request {
	t.Helper()
	o, err := rf.DecodeObservation(raw)
	if err != nil {
		t.Fatal(err)
	}
	selection := map[string]any{"Policy": rf.SelectionPolicy{Owner: "fixture-policy-owner", Revision: "fixture-policy-v1", Evaluator: rf.SelectionEvaluator, Unmapped: rf.UseObservedScore, Priorities: []rf.PriorityEntry{}}, "Scope": o.Inputs.Requested, "ObservationPolicy": o.Inputs.Policy, "Oldest": o.Inputs.Collection.Start.Add(-time.Second), "Capacity": capacity}
	b, _ := json.Marshal(selection)
	return q.Request{Case: "snapshot", Size: 1, Snapshot: raw, Selection: string(b), Now: o.Inputs.Collection.End.Add(time.Second).Format(time.RFC3339Nano)}
}
func planQueue(t *testing.T, req q.Request) (rf.SelectionPlan, q.Response) {
	t.Helper()
	got, _ := probe(t, build(t), "reviewloop", req, t.TempDir())
	if !got.HTTPComplete || got.Facts["networkAttempts"] != "0" {
		t.Fatal("offline planner fetched", got)
	}
	var p rf.SelectionPlan
	if got.Exit == 0 {
		if err := json.Unmarshal([]byte(got.Output), &p); err != nil {
			t.Fatal(err, got.Output)
		}
	}
	return p, got
}
func TestQueuePlanBridge(t *testing.T) {
	got := collectQueue(t, "normal", 3)
	if got.Exit != 0 {
		t.Fatal(got.Exit, got.Output)
	}
	req := queueRequest(t, got.Snapshot, 2)
	p, r := planQueue(t, req)
	if r.Exit != 0 || p.Depths.Total != 3 || p.Depths.Actions["NEEDS-REVIEW"] != 3 || len(p.Candidates) != 3 {
		t.Fatal(p, r)
	}
	for i, c := range p.Candidates {
		want := rf.SelectionSelected
		if i == 2 {
			want = rf.SelectionDeferred
		}
		if c.Row.Identity.Number != i+1 || c.Row.Identity.Head != strings.Repeat("a", 40) || c.Row.Action != "NEEDS-REVIEW" || c.Disposition != want || c.Proposal == nil {
			t.Fatalf("candidate oracle %d: %+v", i, c)
		}
	}
	lanes := collectQueue(t, "lanes", 4)
	if lanes.Exit != 0 {
		t.Fatal(lanes.Output)
	}
	observed, err := rf.DecodeObservation(lanes.Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range observed.Result.Rows {
		correctness, security := row.Lanes[0].Satisfied, row.Lanes[1].Satisfied
		switch row.Identity.Number {
		case 1:
			if correctness || !security {
				t.Fatal("security approval leaked into correctness", row)
			}
		case 2:
			if correctness || !security || observed.Inputs.Changes[1].Lanes[0].State != "blocked" {
				t.Fatal("security approval cleared correctness block", row)
			}
		case 3:
			if !correctness || security {
				t.Fatal("correctness approval leaked into security", row)
			}
		case 4:
			if correctness || security || observed.Inputs.Changes[3].Lanes[1].State != "blocked" {
				t.Fatal("security fail was lost", row)
			}
		}
	}
	// Delete required inputs/results from the actual command artifact. No field
	// default may turn a malformed or out-of-scope observation into idle success.
	for _, kind := range []string{"head", "scope", "action"} {
		t.Run(kind, func(t *testing.T) {
			var v map[string]any
			_ = json.Unmarshal(got.Snapshot, &v)
			switch kind {
			case "head":
				delete(v["inputs"].(map[string]any)["changes"].([]any)[0].(map[string]any)["identity"].(map[string]any), "head")
			case "scope":
				delete(v["inputs"].(map[string]any), "requested")
			case "action":
				delete(v["result"].(map[string]any)["rows"].([]any)[0].(map[string]any), "action")
			}
			b, _ := json.Marshal(v)
			bad := req
			bad.Snapshot = b
			_, r := planQueue(t, bad)
			if r.Exit != 6 || r.Output != "" {
				t.Fatal("invalid observation looked successful", r)
			}
		})
	}
}
func TestQueuePartialAndDepth(t *testing.T) {
	for _, tc := range []struct {
		name string
		size int
	}{{"denied", 2}, {"partial", 2}, {"missing-head", 2}, {"normal", 100}, {"untrusted", 2}, {"moved-head", 2}} {
		t.Run(fmt.Sprint(tc), func(t *testing.T) {
			r := collectQueue(t, tc.name, tc.size)
			if r.Exit != 6 || len(r.Snapshot) != 0 {
				t.Fatal("incomplete collector emitted complete artifact", r)
			}
		})
	}
	r := collectQueue(t, "normal", 80)
	if r.Exit != 0 {
		t.Fatal(r.Output)
	}
	req := queueRequest(t, r.Snapshot, 1)
	p, res := planQueue(t, req)
	if res.Exit != 0 || len(p.Candidates) != 80 || p.Depths.Total != 80 || p.Depths.Actions["NEEDS-REVIEW"] != 80 {
		t.Fatal("display/capacity trimmed population", p, res)
	}
	bad := req
	observed, _ := rf.DecodeObservation(r.Snapshot)
	bad.Now = observed.Inputs.Collection.End.Add(time.Hour).Format(time.RFC3339Nano)
	_, advanced := planQueue(t, bad)
	if advanced.Exit != 0 {
		t.Fatal("advancing now changed the explicit absolute cutoff", advanced)
	}
	bad = req
	var selection map[string]any
	_ = json.Unmarshal([]byte(req.Selection), &selection)
	selection["Oldest"] = observed.Inputs.Collection.End
	b, _ := json.Marshal(selection)
	bad.Selection = string(b)
	_, res = planQueue(t, bad)
	if res.Exit != 6 {
		t.Fatal("stale became idle", res)
	}
	bad = req
	bad.Snapshot = json.RawMessage(strings.Replace(string(r.Snapshot), rf.ObservationSchema, "unsupported-v2", 1))
	_, res = planQueue(t, bad)
	if res.Exit != 6 {
		t.Fatal("schema accepted", res)
	}
	// Independently captured legacy wire fixtures still pass through the real plan
	// adapter, including its old human-readable stdout and incomplete exit shape.
	for _, complete := range []bool{true, false} {
		actions := fmt.Sprintf(`{"asOf":"2026-10-17T12:00:00Z","scope":{"repos":["example-org/tracker"],"count":1},"prPopulation":{"complete":%v,"cap":100},"rows":[{"repo":"example-org/tracker","number":7,"title":"legacy","action":"NEEDS-REVIEW","score":10}]}`, complete)
		out, _ := probe(t, build(t), "reviewloop", q.Request{Case: "legacy", Size: 1, Actions: json.RawMessage(actions), PRs: json.RawMessage(`{"prs":[{"repo":"example-org/tracker","number":7,"headSHA":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}`), Now: "2026-10-17T12:01:00Z"}, t.TempDir())
		want := 0
		if !complete {
			want = 6
		}
		if out.Exit != want || !strings.Contains(out.Output, "OUTWARD VERBS, coalesced") {
			t.Fatal("legacy contract changed", out)
		}
	}
}
func TestReviewPlanNoFetch(t *testing.T) {
	collected := collectQueue(t, "normal", 3)
	if collected.Exit != 0 {
		t.Fatal(collected.Output)
	}
	graphql, files, metadata := 0, 0, 0
	for _, a := range collected.Attempts {
		switch {
		case a.Path == "/graphql":
			graphql++
		case strings.HasSuffix(a.Path, "/files"):
			files++
		case strings.Contains(a.Path, "/pulls/"):
			metadata++
		default:
			t.Fatal("unexpected read", a)
		}
	}
	if graphql != 1 || files != 3 || metadata != 3 {
		t.Fatalf("collection calls graphql=%d files=%d; want one shared query plus N files and N metadata reads", graphql, files)
	}
	req := queueRequest(t, collected.Snapshot, 1)
	p, r := planQueue(t, req)
	if r.Exit != 0 || p.Candidates[0].Disposition != rf.SelectionSelected {
		t.Fatal("offline selection failed on valid observation", p, r)
	}
	// Slot completion changes advisory capacity locally, without recollection.
	wider := queueRequest(t, collected.Snapshot, 2)
	p, r = planQueue(t, wider)
	if r.Exit != 0 || p.Candidates[1].Disposition != rf.SelectionSelected {
		t.Fatal(p, r)
	}
	var invocation map[string]any
	_ = json.Unmarshal([]byte(req.Selection), &invocation)
	o, _ := rf.DecodeObservation(collected.Snapshot)
	invocation["Policy"] = rf.SelectionPolicy{Owner: "fixture-policy-owner", Revision: "fixture-policy-v2", Evaluator: rf.SelectionEvaluator, Unmapped: rf.HoldUnmapped, Priorities: []rf.PriorityEntry{{Subject: rf.PrioritySubject{Repository: o.Inputs.Requested[0], Number: 3}, Score: 100}}}
	b, _ := json.Marshal(invocation)
	reorder := req
	reorder.Selection = string(b)
	p, r = planQueue(t, reorder)
	if r.Exit != 0 || p.Candidates[0].Row.Identity.Number != 3 || p.Candidates[0].Disposition != rf.SelectionSelected || !reflect.DeepEqual(p.Depths, o.Result.Depths) {
		t.Fatal("policy-only reorder lost rows/depths", p, r)
	}
	for _, field := range []string{"ObservationPolicy", "Scope", "Oldest"} {
		t.Run(field, func(t *testing.T) {
			var inv map[string]any
			_ = json.Unmarshal([]byte(req.Selection), &inv)
			switch field {
			case "ObservationPolicy":
				inv[field] = "different-source-revision"
			case "Scope":
				inv[field] = []rf.Repository{{Provider: "github", Host: "github.com", Repository: "example-org/other"}}
			case "Oldest":
				inv[field] = o.Inputs.Collection.End.Add(time.Second)
			}
			b, _ := json.Marshal(inv)
			bad := req
			bad.Selection = string(b)
			_, r := planQueue(t, bad)
			if r.Exit != 6 || r.Output != "" {
				t.Fatal("invalidation became idle", r)
			}
		})
	}
	req.Case = "snapshot-conflict"
	_, r = planQueue(t, req)
	if r.Exit != 5 {
		t.Fatal("mixed snapshot and legacy reread accepted", r)
	}

	for _, value := range []string{"", " ", "\t"} {
		bad := req
		bad.Case = "snapshot"
		bad.Now = value
		_, r := planQueue(t, bad)
		if r.Exit != 5 {
			t.Fatal("invalid-now accepted", value, r)
		}
	}
	omitted := req
	omitted.Case = "snapshot-no-now"
	_, r = planQueue(t, omitted)
	if r.Exit != 5 {
		t.Fatal("invalid-now accepted: absent", r)
	}
	empty := collectQueue(t, "empty", 1)
	if empty.Exit != 0 {
		t.Fatal(empty.Output)
	}
	gapReq := queueRequest(t, empty.Snapshot, 1)
	var gapInvocation map[string]any
	_ = json.Unmarshal([]byte(gapReq.Selection), &gapInvocation)
	gapObservation, _ := rf.DecodeObservation(empty.Snapshot)
	gapInvocation["Policy"] = rf.SelectionPolicy{Owner: "fixture-policy-owner", Revision: "gap-v1", Evaluator: rf.SelectionEvaluator, Unmapped: rf.HoldUnmapped, Priorities: []rf.PriorityEntry{{Subject: rf.PrioritySubject{Repository: gapObservation.Inputs.Requested[0], Number: 99}, Score: 10}}}
	gapJSON, _ := json.Marshal(gapInvocation)
	gapReq.Selection = string(gapJSON)
	_, gapResult := planQueue(t, gapReq)
	if gapResult.Exit != 6 {
		t.Fatal("unobserved priority looked idle", gapResult)
	}
	var gapPlan rf.SelectionPlan
	if err := json.Unmarshal([]byte(gapResult.Output), &gapPlan); err != nil || gapPlan.Idle.State != rf.IdleCouldNotCheck || len(gapPlan.NotObserved) != 1 {
		t.Fatal("gap lost could-not-check verdict", gapPlan, err)
	}
	if os.Getenv("ASSAY_REVIEW_CONTRACT_MUTANT") != "1" {
		t.Run("mutation-controls", func(t *testing.T) {
			contractMutation(t, "cmd/deskboard/reviewobservation.go", `encodeErr := json.NewEncoder(f).Encode(value)`, `_ = os.Chmod(f.Name(),0644); encodeErr := json.NewEncoder(f).Encode(value)`, "^TestReviewObservationPrivateAtomicOutput$", "artifact not a private regular file", "./cmd/deskboard")
			contractMutation(t, "cmd/deskboard/reviewobservation.go", `security = "blocked"`, `security = "missing"`, "^TestQueuePlanBridge$", "security fail was lost")
			contractMutation(t, "cmd/reviewloop/plan.go", `strings.TrimSpace(*nowStr) == ""`, `*nowStr == ""`, "^TestReviewPlanNoFetch$", "invalid-now accepted")
			contractMutation(t, "cmd/deskboard/reviewobservation.go", `part.truncated || len(part.external) != 0`, `part.truncated`, "^TestQueuePartialAndDepth$", "incomplete collector emitted complete artifact")
			contractMutation(t, "cmd/deskboard/board.go", `pr.HeadSHA != expectedHead[0]`, `false && pr.HeadSHA != expectedHead[0]`, "^TestQueuePartialAndDepth$", "incomplete collector emitted complete artifact")
			contractMutation(t, "cmd/reviewloop/snapshot.go", `if p.Idle.State == rf.IdleCouldNotCheck {`, `if false && p.Idle.State == rf.IdleCouldNotCheck {`, "^TestReviewPlanNoFetch$", "unobserved priority looked idle")
			contractMutation(t, "cmd/reviewloop/plan.go", ` || strings.TrimSpace(*nowStr) == ""`, ``, "^TestReviewPlanNoFetch$", "invalid-now accepted")
			contractMutation(t, "cmd/deskboard/reviewobservation.go", `state = "blocked"`, `state = "missing"`, "^TestQueuePlanBridge$", "security approval cleared correctness block")
			contractMutation(t, "cmd/reviewloop/snapshot.go", `return deskkit.Unverifiable("reviewloop plan: snapshot cannot support selection; refresh or hold", err)`, `p.Idle.State = rf.IdleYes; return json.NewEncoder(stdout).Encode(p)`, "^TestReviewPlanNoFetch$", "invalidation became idle")
			contractMutation(t, "cmd/reviewloop/snapshot.go", `b, err := readInput(path)`, `if _,err := readInput("actions.json"); err!=nil {return deskkit.Unverifiable("hidden legacy reread",err)}; b, err := readInput(path)`, "^TestReviewPlanNoFetch$", "offline selection failed on valid observation")
		})
	}

}
