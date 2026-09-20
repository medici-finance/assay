package main

// stuckflip_test.go — FAIL-FIRST coverage for the stuck-flip bucket (#1309 item 3).
//
// THE DEFECT. A brief with a landed `"outcome":"verified"` sidecar row and FILLED Evidence — a
// `gate: model` row whose CI flip is stuck — was re-listed as a DISPATCH candidate on every plan
// and re-verified each round, reproducing the same verified outcome. These pin that such a row is
// bucketed `stuck-flip` (a finding to file / point the flip at), keyed on the sidecar's latest
// row for the brief plus the Evidence state, and that the two near-misses stay dispatchable: a
// verify-fail outcome (wants a re-run once fixed) and a verified outcome with EMPTY Evidence
// (the Evidence never landed, so the verify must run).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

const sidecarFixture = `{"ts": "2026-09-07T01:19:23Z", "brief": "example-stream/01", "outcome": "verify-fail", "rows_passed": 4, "rows_total": 5, "sha": "0000001"}
{"ts": "2026-09-08T02:00:00Z", "brief": "example-stream/01", "outcome": "verified", "rows_passed": 5, "rows_total": 5, "sha": "0000002"}
not json at all
{"ts": "2026-09-08T03:00:00Z", "brief": "example-stream/02", "outcome": "verify-fail", "rows_passed": 1, "rows_total": 5, "sha": "0000003"}
{"ts": "2026-09-08T04:00:00Z", "brief": "example-stream/03", "outcome": "verified", "rows_passed": 5, "rows_total": 5, "sha": "0000004"}
`

func filledEvidence(brief string) string {
	return strings.Replace(brief, "<!-- appended at verification time -->\n",
		"<!-- appended at verification time -->\n| 1 | `go test ./...` | 0 | ok |\n", 1)
}

func TestStuckFlip_VerifiedSidecarWithFilledEvidenceIsBucketedNotDispatched(t *testing.T) {
	clear := planBrief("model", "no", "no", "no", "no")
	root := planFixtureRoot(t, map[string]string{
		"01": filledEvidence(clear), // latest sidecar row verified + Evidence filled → stuck-flip
		"02": filledEvidence(clear), // latest sidecar row verify-fail → DISPATCH (re-run once fixed)
		"03": clear,                 // sidecar verified but Evidence EMPTY → DISPATCH (Evidence never landed)
		"04": clear,                 // no sidecar row at all → DISPATCH
	})
	if err := os.WriteFile(filepath.Join(root, "docs", "streams", outcomeSidecarName), []byte(sidecarFixture), 0o644); err != nil {
		t.Fatal(err)
	}

	var perr error
	out := captureStdout(t, func() { perr = cmdPlan([]string{"--root", root}) })
	if perr != nil {
		t.Fatalf("cmdPlan: %v", perr)
	}
	if strings.Contains(out, "=== DISPATCH example-stream/01") {
		t.Fatalf("stuck-flip brief was re-dispatched:\n%s", out)
	}
	if !strings.Contains(out, "-- stuck-flip (1): a verified outcome is in the sidecar and Evidence is filled") {
		t.Fatalf("no stuck-flip bucket in the plan:\n%s", out)
	}
	if !strings.Contains(out, "example-stream/01 — sidecar outcome verified 2026-09-08T02:00:00Z (sha 0000002), Evidence filled, status still implemented") {
		t.Fatalf("stuck-flip member line does not name the landed outcome:\n%s", out)
	}
	for _, id := range []string{"example-stream/02", "example-stream/03", "example-stream/04"} {
		if !strings.Contains(out, "=== DISPATCH "+id) {
			t.Fatalf("%s must stay dispatchable:\n%s", id, out)
		}
	}
	if !strings.Contains(out, "3 dispatchable, 1 deferred/bucketed") {
		t.Fatalf("summary count wrong:\n%s", out)
	}
}

// The sidecar read keys on the LATEST row per brief and tolerates a malformed line.
func TestReadOutcomeSidecar_LatestRowWinsAndBadLinesSkip(t *testing.T) {
	p := filepath.Join(t.TempDir(), outcomeSidecarName)
	if err := os.WriteFile(p, []byte(sidecarFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	got := readOutcomeSidecar(p)
	if got["example-stream/01"].Outcome != "verified" || got["example-stream/01"].SHA != "0000002" {
		t.Fatalf("latest row for 01 = %+v, want the verified 0000002 row", got["example-stream/01"])
	}
	if len(got) != 3 {
		t.Fatalf("sidecar map = %d briefs, want 3 (bad line skipped)", len(got))
	}
	if len(readOutcomeSidecar(filepath.Join(t.TempDir(), "absent.jsonl"))) != 0 {
		t.Fatalf("an absent sidecar must read as no outcomes")
	}
}

// Precedence: stuck-flip sits after the human arm (a human-gated stuck row is the human's flip,
// awaiting-human) and before in-repair / online-lane.
func TestClassifyItem_StuckFlipPrecedence(t *testing.T) {
	v := &VerifyLoop{}
	stuck := map[string]string{"sidecar_outcome": "verified", "sidecar_ts": "t", "sidecar_sha": "s", "evidence_empty": "no", "status": "implemented"}
	it := item("s/01", "model", loopengine.RiskFlags{}, stuck)
	tier, _ := v.TierPolicy(it)
	if disp, _ := classifyItem(it, tier); disp != dispStuckFlip {
		t.Fatalf("stuck shape classified %v, want stuck-flip", disp)
	}
	human := item("s/02", "human", loopengine.RiskFlags{}, stuck)
	tier, _ = v.TierPolicy(human)
	if disp, _ := classifyItem(human, tier); disp != dispAwaitingHuman {
		t.Fatalf("human-gated stuck shape classified %v, want awaiting-human", disp)
	}
	lane := map[string]string{"verify_lane": "cluster"}
	for k, val := range stuck {
		lane[k] = val
	}
	it = item("s/03", "model", loopengine.RiskFlags{}, lane)
	tier, _ = v.TierPolicy(it)
	if disp, _ := classifyItem(it, tier); disp != dispStuckFlip {
		t.Fatalf("stuck + online lane classified %v, want stuck-flip (a re-run cannot land the flip on any lane)", disp)
	}
}
