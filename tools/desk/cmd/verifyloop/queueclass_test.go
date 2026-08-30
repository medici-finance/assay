package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// item is a small constructor for a queue Item carrying the queue-truthfulness markers on its
// Payload the way scanAwaiting populates them.
func item(id, gate string, risk loopengine.RiskFlags, markers map[string]string) loopengine.Item {
	p := map[string]string{}
	for k, v := range markers {
		p[k] = v
	}
	return loopengine.Item{ID: id, Gate: gate, Risk: risk, Payload: p}
}

// TestClassifyItem_Dispositions is the core table: each queue item lands in exactly one
// disposition, and only a plain risk-clear/unmarked brief stays dispatchable. The tier is the
// one the loop's TierPolicy would compute (reused, not re-derived), so the human-gate row goes
// through TierPolicy exactly as the plan does.
func TestClassifyItem_Dispositions(t *testing.T) {
	v := &VerifyLoop{}
	cases := []struct {
		name string
		it   loopengine.Item
		want disposition
	}{
		{
			// (c) plain implemented brief, risk-clear, no markers → DISPATCH (backward-compatible).
			name: "plain risk-clear implemented -> dispatch",
			it:   item("s/01", "model", loopengine.RiskFlags{}, nil),
			want: dispDispatch,
		},
		{
			// (a) blocked-until marker → deferred out of the dispatchable list.
			name: "blocked-until -> deferred",
			it:   item("s/02", "model", loopengine.RiskFlags{}, map[string]string{"blocked_until": "2026-09-15 (shadow window accrues)"}),
			want: dispDeferred,
		},
		{
			// (b) human gate → awaiting-human (via TierPolicy → TierHuman), NOT dispatch.
			name: "gate human -> awaiting-human",
			it:   item("s/03", "human", loopengine.RiskFlags{}, nil),
			want: dispAwaitingHuman,
		},
		{
			// (b) a risk answer yes also routes to the human bucket (gate derived to human).
			name: "risk answer yes -> awaiting-human",
			it:   item("s/04", "human", loopengine.RiskFlags{Customer: true}, nil),
			want: dispAwaitingHuman,
		},
		{
			// (b) online verify lane → awaiting-online-lane, NOT dispatch.
			name: "verify-lane cluster -> awaiting-online-lane",
			it:   item("s/05", "model", loopengine.RiskFlags{}, map[string]string{"verify_lane": "cluster"}),
			want: dispAwaitingOnlineLane,
		},
		{
			name: "verify-lane live-session -> awaiting-online-lane",
			it:   item("s/06", "model", loopengine.RiskFlags{}, map[string]string{"verify_lane": "live-session"}),
			want: dispAwaitingOnlineLane,
		},
		{
			// (b) in-repair marker → in-repair bucket, NOT dispatch.
			name: "in-repair -> in-repair bucket",
			it:   item("s/07", "model", loopengine.RiskFlags{}, map[string]string{"in_repair": "table-repair/12"}),
			want: dispInRepair,
		},
		{
			// An offline (non-online) verify-lane value must NOT bucket the brief.
			name: "verify-lane offline value stays dispatch",
			it:   item("s/08", "model", loopengine.RiskFlags{}, map[string]string{"verify_lane": "offline"}),
			want: dispDispatch,
		},
		{
			// An explicit falsey in-repair value must NOT strand the brief.
			name: "in-repair: no stays dispatch",
			it:   item("s/09", "model", loopengine.RiskFlags{}, map[string]string{"in_repair": "no"}),
			want: dispDispatch,
		},
		{
			// blocked-until is the most specific: it wins even over a human gate.
			name: "blocked-until beats human gate",
			it:   item("s/10", "human", loopengine.RiskFlags{Regulatory: true}, map[string]string{"blocked_until": "2026-12-01"}),
			want: dispDeferred,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tier, err := v.TierPolicy(c.it)
			if err != nil {
				t.Fatalf("TierPolicy: %v", err)
			}
			got, _ := classifyItem(c.it, tier)
			if got != c.want {
				t.Fatalf("classifyItem(%s) = %v; want %v", c.it.ID, got, c.want)
			}
		})
	}
}

// TestClassifyItem_DispatchIsTheActionableSet asserts that exactly the plain briefs reach the
// dispatchable set and every marked/gated brief is bucketed out — the "genuinely-actionable"
// invariant the plan output depends on.
func TestClassifyItem_DispatchIsTheActionableSet(t *testing.T) {
	v := &VerifyLoop{}
	its := []loopengine.Item{
		item("s/01", "model", loopengine.RiskFlags{}, nil),                                         // dispatch
		item("s/02", "model", loopengine.RiskFlags{}, map[string]string{"blocked_until": "later"}), // deferred
		item("s/03", "human", loopengine.RiskFlags{}, nil),                                         // awaiting-human
		item("s/04", "model", loopengine.RiskFlags{}, map[string]string{"verify_lane": "online"}),  // online-lane
		item("s/05", "model", loopengine.RiskFlags{}, map[string]string{"in_repair": "repair/03"}), // in-repair
		item("s/06", "model", loopengine.RiskFlags{}, nil),                                         // dispatch
	}
	var dispatch []string
	buckets := map[disposition]int{}
	for _, it := range its {
		tier, _ := v.TierPolicy(it)
		disp, _ := classifyItem(it, tier)
		if disp == dispDispatch {
			dispatch = append(dispatch, it.ID)
			continue
		}
		buckets[disp]++
	}
	if strings.Join(dispatch, ",") != "s/01,s/06" {
		t.Fatalf("dispatchable set = %v; want [s/01 s/06]", dispatch)
	}
	for _, want := range []disposition{dispDeferred, dispAwaitingHuman, dispAwaitingOnlineLane, dispInRepair} {
		if buckets[want] != 1 {
			t.Fatalf("bucket %v count = %d; want 1", want, buckets[want])
		}
	}
}

// briefBodyMarkers builds a brief with arbitrary extra frontmatter lines (the queue-truthfulness
// markers), so the SelectQueue → Payload propagation is exercised end-to-end.
func briefBodyMarkers(extraFrontmatter string) string {
	return "---\nbrief: x\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\neffort: S\n" +
		extraFrontmatter +
		"---\n\n# Brief\n\n## Verify\n\n| # | Command | Expect |\n\n## Evidence\n<!-- appended at verification time -->\n\n"
}

// TestSelectQueue_CarriesBucketMarkers proves the markers survive the board read onto Item.Payload,
// so classifyItem sees them exactly as the plan does.
func TestSelectQueue_CarriesBucketMarkers(t *testing.T) {
	root := t.TempDir()
	table := "| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | A | 0 | S | implemented | — | — |\n" +
		"| 02 | B | 0 | S | implemented | — | — |\n" +
		"| 03 | C | 0 | S | implemented | — | — |\n"
	briefs := map[string]string{
		"01": briefBodyMarkers("blocked-until: 2026-09-15 (shadow window accrues)\n"),
		"02": briefBodyMarkers("verify-lane: cluster\n"),
		"03": briefBodyMarkers("in-repair: table-repair/12\n"),
	}
	writeFixtureStream(t, root, "fixture", table, briefs)

	v := &VerifyLoop{Root: root, TargetSHA: "abc"}
	items, err := v.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	byID := map[string]loopengine.Item{}
	for _, it := range items {
		byID[it.ID] = it
	}
	if got := byID["fixture/01"].Payload["blocked_until"]; !strings.Contains(got, "shadow window") {
		t.Fatalf("blocked_until not carried: %q", got)
	}
	if got := byID["fixture/02"].Payload["verify_lane"]; got != "cluster" {
		t.Fatalf("verify_lane not carried: %q", got)
	}
	if got := byID["fixture/03"].Payload["in_repair"]; got != "table-repair/12" {
		t.Fatalf("in_repair not carried: %q", got)
	}
	// And end-to-end: each classifies out of DISPATCH.
	v2 := &VerifyLoop{}
	for id, wantDisp := range map[string]disposition{
		"fixture/01": dispDeferred,
		"fixture/02": dispAwaitingOnlineLane,
		"fixture/03": dispInRepair,
	} {
		tier, _ := v2.TierPolicy(byID[id])
		if disp, _ := classifyItem(byID[id], tier); disp != wantDisp {
			t.Fatalf("%s classified %v; want %v", id, disp, wantDisp)
		}
	}
}

// captureStdout runs fn with os.Stdout redirected and returns everything it printed.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// TestPrintBuckets_RendersSectionsWithCounts locks the plan's deferred + bucket rendering: each
// non-empty disposition prints its count and its one-line "why it waits", deferred/bucket items
// carry their reason detail, and awaiting-human (no per-item reason) lists bare IDs.
func TestPrintBuckets_RendersSectionsWithCounts(t *testing.T) {
	bucketed := map[disposition][]bucketMember{
		dispDeferred:           {{ID: "demo/02", Reason: "2026-09-15 (shadow accrual window not yet elapsed)"}},
		dispAwaitingHuman:      {{ID: "demo/03"}},
		dispAwaitingOnlineLane: {{ID: "demo/04", Reason: "cluster"}},
		dispInRepair:           {{ID: "demo/05", Reason: "table-repair/12"}},
	}
	out := captureStdout(t, func() { printBuckets(2, bucketed) })

	for _, must := range []string{
		"2 dispatchable, 4 deferred/bucketed",
		"-- deferred (1):",
		"demo/02 — 2026-09-15 (shadow accrual window not yet elapsed)",
		"-- awaiting-human (1):",
		"-- awaiting-online-lane (1):",
		"demo/04 — cluster",
		"-- in-repair (1):",
		"demo/05 — table-repair/12",
	} {
		if !strings.Contains(out, must) {
			t.Fatalf("plan output missing %q:\n%s", must, out)
		}
	}
	// awaiting-human has no per-item reason: the ID is listed bare, no " — " suffix on its line.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "demo/03") && strings.Contains(line, " — ") {
			t.Fatalf("awaiting-human item should list a bare ID, got: %q", line)
		}
	}
}
