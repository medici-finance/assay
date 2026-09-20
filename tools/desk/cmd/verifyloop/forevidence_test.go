package main

// forevidence_test.go — FAIL-FIRST coverage for the DISPATCH-FOR-EVIDENCE class (#1309 item 2).
//
// THE DEFECT. classifyItem made DISPATCH the only disposition that reached the dispatchable
// list, while the ROUTE-HUMAN bucket's own text said "a model MAY gather Evidence for it, and
// never flips it". The Evidence-gathering worklist therefore had to be hand-computed from the
// bucket every pass. These pin the second dispatchable class: a human-gated brief with EMPTY
// Evidence (and nothing else withholding an offline run) is emitted as DISPATCH-FOR-EVIDENCE
// after the DISPATCH set; one whose Evidence is already gathered stays awaiting-human.

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

func TestClassifyItem_HumanGatedEmptyEvidenceIsDispatchForEvidence(t *testing.T) {
	v := &VerifyLoop{}
	cases := []struct {
		name string
		it   loopengine.Item
		want disposition
	}{
		{"gate:human + empty Evidence -> for-evidence",
			item("s/01", "human", loopengine.RiskFlags{}, map[string]string{"evidence_empty": "yes"}), dispDispatchForEvidence},
		{"irreversible + empty Evidence -> for-evidence",
			item("s/02", "model", loopengine.RiskFlags{Irreversible: true}, map[string]string{"evidence_empty": "yes"}), dispDispatchForEvidence},
		{"gate:human + Evidence gathered -> awaiting-human (no re-gather treadmill)",
			item("s/03", "human", loopengine.RiskFlags{}, map[string]string{"evidence_empty": "no"}), dispAwaitingHuman},
		{"gate:human + unknown Evidence state -> awaiting-human (fail-safe)",
			item("s/04", "human", loopengine.RiskFlags{}, nil), dispAwaitingHuman},
		{"gate:human + online lane -> awaiting-human (no offline Evidence possible)",
			item("s/05", "human", loopengine.RiskFlags{}, map[string]string{"evidence_empty": "yes", "verify_lane": "cluster"}), dispAwaitingHuman},
		{"gate:human + in-repair -> awaiting-human",
			item("s/06", "human", loopengine.RiskFlags{}, map[string]string{"evidence_empty": "yes", "in_repair": "repair/1"}), dispAwaitingHuman},
		{"risk-clear + empty Evidence -> plain dispatch, never for-evidence",
			item("s/07", "model", loopengine.RiskFlags{}, map[string]string{"evidence_empty": "yes"}), dispDispatch},
	}
	for _, c := range cases {
		tier, _ := v.TierPolicy(c.it)
		if got, _ := classifyItem(c.it, tier); got != c.want {
			t.Errorf("%s: classified %v, want %v", c.name, got, c.want)
		}
	}
	if _, reason := classifyItem(cases[0].it, loopengine.TierHuman); reason != "gate: human" {
		t.Errorf("gate:human-only for-evidence reason = %q, want \"gate: human\"", reason)
	}
	if _, reason := classifyItem(cases[1].it, loopengine.TierHuman); reason != "risk: irreversible" {
		t.Errorf("risk for-evidence reason = %q, want \"risk: irreversible\"", reason)
	}
}

// The plan EMITS the class: after the DISPATCH set, each for-evidence item gets its own block
// headed with the human-gate reason and the Evidence-only limit, at the local session tier, and
// the prompt opens with the no-flip clause. Before the fix the item sat in the ROUTE-HUMAN
// bucket with no dispatch block at all.
func TestPlan_EmitsDispatchForEvidenceAfterDispatchSet(t *testing.T) {
	root := planFixtureRoot(t, map[string]string{
		"01": planBrief("human", "no", "no", "no", "no"), // gate:human, empty Evidence
		"02": planBrief("model", "no", "no", "no", "no"), // risk-clear — plain DISPATCH
	})
	var perr error
	out := captureStdout(t, func() { perr = cmdPlan([]string{"--root", root}) })
	if perr != nil {
		t.Fatalf("cmdPlan: %v", perr)
	}
	header := "=== DISPATCH-FOR-EVIDENCE example-stream/01 (tier=local) — ROUTE-HUMAN: gate: human — " + evidenceOnlyMarker + " ==="
	hi := strings.Index(out, header)
	if hi < 0 {
		t.Fatalf("plan output has no DISPATCH-FOR-EVIDENCE block for the gate:human brief:\n%s", out)
	}
	di := strings.Index(out, "=== DISPATCH example-stream/02")
	if di < 0 || di > hi {
		t.Fatalf("DISPATCH-FOR-EVIDENCE must be emitted AFTER the DISPATCH set (dispatch at %d, for-evidence at %d):\n%s", di, hi, out)
	}
	if !strings.Contains(out[hi:], "EVIDENCE-ONLY (gate: human): gather and record Evidence rows plus the outcome sidecar row") ||
		!strings.Contains(out[hi:], "NEVER flip its status") {
		t.Fatalf("for-evidence prompt lacks the no-flip clause:\n%s", out[hi:])
	}
	if !strings.Contains(out, "1 dispatch-for-evidence (Evidence rows + outcome sidecar only; flip never)") {
		t.Fatalf("plan summary does not count the for-evidence set:\n%s", out)
	}
	if strings.Contains(out, "-- awaiting-human / ROUTE-HUMAN") {
		t.Fatalf("a for-evidence item must not also be listed in the awaiting-human bucket:\n%s", out)
	}
}

// A human-gated brief whose Evidence is already gathered is NOT re-dispatched: it stays in the
// awaiting-human bucket, waiting on the human flip.
func TestPlan_GatheredEvidenceStaysAwaitingHuman(t *testing.T) {
	filled := strings.Replace(planBrief("human", "no", "no", "no", "no"),
		"<!-- appended at verification time -->\n", "<!-- appended at verification time -->\n| 1 | `go test ./...` | 0 | ok |\n", 1)
	root := planFixtureRoot(t, map[string]string{"01": filled})
	out := captureStdout(t, func() { _ = cmdPlan([]string{"--root", root}) })
	if strings.Contains(out, "DISPATCH-FOR-EVIDENCE") {
		t.Fatalf("gathered-Evidence human brief was re-dispatched for Evidence:\n%s", out)
	}
	if !strings.Contains(out, "-- awaiting-human / ROUTE-HUMAN (1)") {
		t.Fatalf("gathered-Evidence human brief missing from the awaiting-human bucket:\n%s", out)
	}
}
