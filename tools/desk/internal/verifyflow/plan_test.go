package verifyflow

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/statusgen/verifyqueue"
)

var t0 = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

func row(brief, class string) verifyqueue.Row {
	return verifyqueue.Row{Brief: brief, Repo: "example-org/one", QueueVersion: verifyqueue.Version, QueueClass: class,
		QueueValid: true, QueueNextAct: "next act"}
}

func cand(brief, class, overlay string, order int) Candidate {
	return Candidate{Subject: Subject{Repository: "example-org/one", Brief: brief, Revision: strings.Repeat("c", 40)},
		Sources: []string{Awaiting}, Base: row(brief, class), Overlay: overlay, Order: order}
}

// obs is a sealed one-root observation with complete coverage on both lanes.
func obs(cs ...Candidate) Observation {
	o := Observation{Schema: Schema, Evaluator: Evaluator, Provenance: "unit", Collected: t0, WholeScope: true,
		Requested: []string{"example-org/one"}, Candidates: cs,
		Coverage: []Coverage{{Repository: "example-org/one", Lane: Awaiting, State: "complete"},
			{Repository: "example-org/one", Lane: Repair, State: "complete"}}}
	o.Revision = ContentRevision(o)
	return o
}

func policy(unmapped string, ps ...Priority) Policy {
	return Policy{Owner: "unit", Revision: "p1", Evaluator: Evaluator, Unmapped: unmapped, Priorities: append([]Priority{}, ps...)}
}

func plan(t *testing.T, o Observation, p Policy, capacity int) Plan {
	t.Helper()
	got, err := PlanCandidates(o, p, t0, t0.Add(-time.Minute), capacity)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func byBrief(p Plan) map[string]Decision {
	m := map[string]Decision{}
	for _, d := range p.Candidates {
		m[d.Candidate.Subject.Brief] = d
	}
	return m
}

// regression: F-verify-candidate-parity
func TestHoldPolicyKeepsWatch(t *testing.T) {
	o := obs(cand("s/01", verifyqueue.Paused, Dispatch, 0), cand("s/02", verifyqueue.Parked, Evidence, 1))
	for _, unmapped := range []string{"observed-score", "hold"} {
		got := plan(t, o, policy(unmapped), 2)
		for b, d := range byBrief(got) {
			if d.Disposition != "watch" || d.Authority != AuthorityNone {
				t.Errorf("%s %s: disposition=%s authority=%s, want watch/none", unmapped, b, d.Disposition, d.Authority)
			}
		}
		if got.Idle != "yes" {
			t.Errorf("%s: paused/parked-only desk idle=%s gaps=%v, want yes", unmapped, got.Idle, got.Gaps)
		}
	}
}

// regression: F-verify-candidate-parity
func TestHoldPolicyHoldsUnmapped(t *testing.T) {
	o := obs(cand("s/01", verifyqueue.Judgment, Dispatch, 0), cand("s/02", verifyqueue.Judgment, Dispatch, 1))
	got := byBrief(plan(t, o, policy("hold", Priority{Repository: "example-org/one", Brief: "s/02", Score: 5}), 2))
	if d := got["s/01"]; d.Disposition != "held" || d.Authority != AuthorityNone || !strings.HasSuffix(d.Reason, "; priority unmapped") {
		t.Errorf("unmapped judgment = %+v, want held/none with the unmapped reason", d)
	}
	if d := got["s/02"]; d.Disposition != "selected" || d.Authority != AuthorityFlip {
		t.Errorf("mapped judgment = %s/%s, want selected/flip", d.Disposition, d.Authority)
	}
}

// regression: F-verify-candidate-parity
func TestDecisionAuthority(t *testing.T) {
	o := obs(cand("s/01", verifyqueue.Judgment, Dispatch, 0), cand("s/02", verifyqueue.Human, Evidence, 1),
		cand("s/03", verifyqueue.Human, Dispatch, 2), cand("s/04", verifyqueue.Judgment, Evidence, 3))
	got := byBrief(plan(t, o, policy("observed-score"), 1))
	want := map[string][2]string{"s/01": {"selected", AuthorityFlip}, "s/02": {"deferred", AuthorityEvidence},
		"s/03": {"held", AuthorityNone}, "s/04": {"deferred", AuthorityEvidence}}
	for b, w := range want {
		if d := got[b]; d.Disposition != w[0] || d.Authority != w[1] {
			t.Errorf("%s = %s/%s, want %s/%s", b, d.Disposition, d.Authority, w[0], w[1])
		}
	}
	got = byBrief(plan(t, o, policy("observed-score"), 4))
	if d := got["s/02"]; d.Disposition != "selected" || d.Authority != AuthorityEvidence {
		t.Errorf("selected Evidence-only = %s/%s, want evidence-only authority", d.Disposition, d.Authority)
	}
}

// regression: F-verify-candidate-parity
func TestComposeReasonText(t *testing.T) {
	r := row("s/01", verifyqueue.Unreadable)
	r.QueueValid = false
	if _, why := Compose(Dispatch, "", &r); why != "canonical queue: unreadable — next act" {
		t.Errorf("unreadable without a why = %q, want the next-act fallback", why)
	}
	r.QueueWhy = "row hash mismatch"
	if _, why := Compose(Dispatch, "", &r); why != "canonical queue: unreadable — row hash mismatch" {
		t.Errorf("unreadable with a why = %q", why)
	}
	p := row("s/02", verifyqueue.Paused)
	if _, why := Compose(Dispatch, "", &p); why != "canonical queue: paused (-, owner -): next act" {
		t.Errorf("paused without bucket/owner = %q, want - placeholders", why)
	}
	p.QueueBucket, p.QueueOwner = "b", "o"
	if _, why := Compose(Dispatch, "", &p); why != "canonical queue: paused (b, owner o): next act" {
		t.Errorf("paused with bucket/owner = %q", why)
	}
}

// regression: F-verify-candidate-parity
func TestGapsNoTrailingSep(t *testing.T) {
	o := obs(cand("s/01", verifyqueue.Judgment, Dispatch, 0))
	o.Coverage = o.Coverage[:1]
	o.Candidates[0].Subject.Revision = ""
	o.Revision = ContentRevision(o)
	got := plan(t, o, policy("observed-score"), 1)
	if len(got.Gaps) != 2 {
		t.Fatalf("gaps = %q, want the repair-lane gap and the unknown candidate", got.Gaps)
	}
	for _, g := range got.Gaps {
		if strings.HasSuffix(g, "; ") || strings.HasSuffix(g, ";") || strings.HasSuffix(g, ": ") {
			t.Errorf("gap %q ends with a dangling separator", g)
		}
	}
	if d := got.Candidates[0]; d.Disposition != Unknown || strings.HasSuffix(d.Reason, "; ") {
		t.Errorf("unknown reason = %q", d.Reason)
	}
}

// regression: F-verify-candidate-parity
func TestRevisionIsVerified(t *testing.T) {
	o := obs(cand("s/01", verifyqueue.Judgment, Dispatch, 0))
	raw, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	var back Observation
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanCandidates(back, policy("observed-score"), t0, t0.Add(-time.Minute), 1); err != nil {
		t.Fatalf("JSON round trip refused: %v", err)
	}
	later := o
	later.Collected = t0.Add(-time.Second)
	if ContentRevision(later) != o.Revision {
		t.Error("collection time changed the content revision")
	}
	tampered := o
	tampered.Candidates = append([]Candidate{}, o.Candidates...)
	tampered.Candidates[0].Base.QueueClass = verifyqueue.Judgment
	tampered.Candidates[0].Overlay = Evidence
	if _, err := PlanCandidates(tampered, policy("observed-score"), t0, t0.Add(-time.Minute), 1); err == nil ||
		!strings.Contains(err.Error(), "does not match") {
		t.Fatalf("edited observation accepted under its old revision: %v", err)
	}
}
