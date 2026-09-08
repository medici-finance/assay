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

// TestClassifyItemRiskGateTable is the fail-safe truth table: every one of the four risk keys
// individually yes (plus the all-no row) crossed with every gate form (model, human, absent,
// Human, and a human value carrying a trailing qualifier). It asserts BOTH the disposition and
// the tier the policy computed for the SAME input, so the two independent layers cannot silently
// disagree — a divergence is a test failure, not a quiet fail-open.
//
//   - awaiting-human whenever ANY risk answer is yes OR the gate reads human;
//   - dispatch ONLY for the all-no + non-human-gate rows.
//
// The tier and the disposition agree by construction with the middle-rung flag off: a row is
// TierHuman iff it is awaiting-human.
func TestClassifyItemRiskGateTable(t *testing.T) {
	v := &VerifyLoop{}
	riskForms := []struct {
		name string
		r    loopengine.RiskFlags
	}{
		{"all-no", loopengine.RiskFlags{}},
		{"regulatory", loopengine.RiskFlags{Regulatory: true}},
		{"customer", loopengine.RiskFlags{Customer: true}},
		{"irreversible", loopengine.RiskFlags{Irreversible: true}},
		{"sensitive-data", loopengine.RiskFlags{SensitiveData: true}},
	}
	gateForms := []struct {
		name  string
		gate  string
		human bool
	}{
		{"model", "model", false},
		{"human", "human", true},
		{"absent", "", false},
		{"Human", "Human", true},
		{"human-qualified", "human — maintainer sign-off", true},
	}
	for _, rf := range riskForms {
		for _, gf := range gateForms {
			name := rf.name + "/" + gf.name
			t.Run(name, func(t *testing.T) {
				it := item("t/01", gf.gate, rf.r, nil)
				anyRisk := rf.r.Any()
				wantHuman := anyRisk || gf.human

				wantDisp := dispDispatch
				wantTier := loopengine.TierLocal
				if wantHuman {
					wantDisp = dispAwaitingHuman
					wantTier = loopengine.TierHuman
				}

				tier, err := v.TierPolicy(it)
				if err != nil {
					t.Fatalf("TierPolicy: %v", err)
				}
				if tier != wantTier {
					t.Fatalf("tier = %v; want %v (the two layers must agree on the same input)", tier, wantTier)
				}
				disp, reason := classifyItem(it, tier)
				if disp != wantDisp {
					t.Fatalf("disposition = %v; want %v", disp, wantDisp)
				}
				// The reason names the risk answers when any is yes; it is empty for a
				// gate:human-only (or purely dispatchable) row.
				if anyRisk {
					if !strings.HasPrefix(reason, "risk: ") || !strings.Contains(reason, rf.name) {
						t.Fatalf("reason = %q; want it to name risk %q", reason, rf.name)
					}
				} else if reason != "" {
					t.Fatalf("reason = %q; want empty for a non-risk row", reason)
				}
			})
		}
	}
}

// TestRiskClearBriefStillDispatches is the NEGATIVE control: the fix moves items OUT of the
// dispatchable list only — it must never sweep a risk-clear, non-human-gate brief out with them.
// A table on which every case is risk-flagged proves the bucket works and says nothing about
// whether the queue still has anything in it; this asserts the queue is not emptied.
func TestRiskClearBriefStillDispatches(t *testing.T) {
	v := &VerifyLoop{}
	// The canonical dispatchable shape: all-no risk, gate:model, no markers.
	clear := item("neg/01", "model", loopengine.RiskFlags{}, nil)
	tier, err := v.TierPolicy(clear)
	if err != nil {
		t.Fatalf("TierPolicy: %v", err)
	}
	if tier != loopengine.TierLocal {
		t.Fatalf("risk-clear tier = %v; want TierLocal", tier)
	}
	if disp, _ := classifyItem(clear, tier); disp != dispDispatch {
		t.Fatalf("risk-clear gate:model classified %v; want dispatch — the fix must not empty the queue", disp)
	}
	// Other previously-dispatchable shapes must also stay dispatchable: an offline verify-lane
	// value and an explicit falsey in-repair value are NOT risk signals and must not bucket.
	for _, it := range []loopengine.Item{
		item("neg/02", "model", loopengine.RiskFlags{}, map[string]string{"verify_lane": "offline"}),
		item("neg/03", "model", loopengine.RiskFlags{}, map[string]string{"in_repair": "no"}),
	} {
		tr, _ := v.TierPolicy(it)
		if disp, _ := classifyItem(it, tr); disp != dispDispatch {
			t.Fatalf("%s classified %v; want dispatch (no risk signal — must not be swept out)", it.ID, disp)
		}
	}
}

// TestDormantReversibleFlagNeverDivertsIrreversible pins that enabling the dormant
// reversible-risk middle rung does NOT divert an irreversible item away from the human: the
// fail-safe arm runs FIRST, ahead of the branch the flag controls. The flag still does its job
// for a REVERSIBLE risk-flagged item (routed to session), so the test also proves the flag is
// genuinely on.
func TestDormantReversibleFlagNeverDivertsIrreversible(t *testing.T) {
	on := &VerifyLoop{F16ReversibleRiskToSession: true}

	irr := item("dorm/01", "model", loopengine.RiskFlags{Irreversible: true}, nil)
	tier, err := on.TierPolicy(irr)
	if err != nil {
		t.Fatalf("TierPolicy: %v", err)
	}
	if tier != loopengine.TierHuman {
		t.Fatalf("irreversible with the reversible-risk flag ON routed to %v; want TierHuman (the flag must not divert irreversible work)", tier)
	}
	if disp, _ := classifyItem(irr, tier); disp != dispAwaitingHuman {
		t.Fatalf("irreversible with flag ON classified %v; want awaiting-human", disp)
	}

	// The flag IS genuinely on: a reversible risk-flagged item routes to session.
	rev := item("dorm/02", "model", loopengine.RiskFlags{Customer: true}, nil)
	if tr, _ := on.TierPolicy(rev); tr != loopengine.TierSession {
		t.Fatalf("reversible risk with flag ON routed to %v; want TierSession (proves the flag is active)", tr)
	}
}

// planFixtureRoot lays down a one-stream board whose briefs are all implemented, so cmdPlan's
// SelectQueue picks every one of them up. briefs maps a two-digit num to its full frontmatter+body.
func planFixtureRoot(t *testing.T, briefs map[string]string) string {
	t.Helper()
	root := t.TempDir()
	var rows strings.Builder
	rows.WriteString("| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n")
	rows.WriteString("|---|-------|------|--------|--------|----------|----------|\n")
	for num := range briefs {
		rows.WriteString("| " + num + " | b" + num + " | 0 | S | implemented | — | — |\n")
	}
	writeFixtureStream(t, root, "example-stream", rows.String(), briefs)
	return root
}

func planBrief(gate string, reg, cust, irr, sens string) string {
	return "---\nbrief: x\ngate: " + gate + "\n" +
		"risk: {regulatory: " + reg + ", customer: " + cust + ", irreversible: " + irr + ", sensitive-data: " + sens + "}\neffort: S\n---\n\n" +
		"# Brief\n\n## Verify\n\n| # | Command | Expect |\n| 1 | `go test ./...` | exit 0 |\n\n" +
		"## Evidence\n<!-- appended at verification time -->\n"
}

// TestIrreversibleBriefIsRouteHumanNotDispatch runs the actual plan output: an irreversible,
// gate:model brief appears under the ROUTE-HUMAN heading and its ID NEVER appears after
// "=== DISPATCH". A risk-clear sibling proves DISPATCH lines are produced at all, so the absence
// of the irreversible ID from a DISPATCH line is meaningful, not an empty-plan artifact.
func TestIrreversibleBriefIsRouteHumanNotDispatch(t *testing.T) {
	root := planFixtureRoot(t, map[string]string{
		"01": planBrief("model", "no", "no", "yes", "no"), // irreversible, gate:model — the fail-open
		"02": planBrief("model", "no", "no", "no", "no"),  // risk-clear — genuinely dispatchable
	})

	var perr error
	out := captureStdout(t, func() { perr = cmdPlan([]string{"--root", root}) })
	if perr != nil {
		t.Fatalf("cmdPlan: %v", perr)
	}

	if !strings.Contains(out, "ROUTE-HUMAN") {
		t.Fatalf("plan output has no ROUTE-HUMAN heading:\n%s", out)
	}
	if !strings.Contains(out, "example-stream/01") {
		t.Fatalf("irreversible brief not listed in the plan:\n%s", out)
	}
	if strings.Contains(out, "=== DISPATCH example-stream/01") {
		t.Fatalf("irreversible gate:model brief was printed as a DISPATCH candidate (fail-open):\n%s", out)
	}
	// The risk-clear sibling IS dispatched — so a DISPATCH section exists and 01's absence is real.
	if !strings.Contains(out, "=== DISPATCH example-stream/02") {
		t.Fatalf("risk-clear sibling was not dispatched — the negative anchor is void:\n%s", out)
	}
}

// TestRouteHumanLineCarriesEvidenceOnlyMarker asserts the member line of a risk-flagged item
// carries BOTH the risk reason (in canonical key order) AND the literal
// "Evidence-only (never flip-eligible)" — the permission and its limit on one line.
func TestRouteHumanLineCarriesEvidenceOnlyMarker(t *testing.T) {
	root := planFixtureRoot(t, map[string]string{
		"01": planBrief("model", "no", "yes", "yes", "no"), // customer + irreversible
	})

	var perr error
	out := captureStdout(t, func() { perr = cmdPlan([]string{"--root", root}) })
	if perr != nil {
		t.Fatalf("cmdPlan: %v", perr)
	}

	var memberLine string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "example-stream/01") {
			memberLine = line
			break
		}
	}
	if memberLine == "" {
		t.Fatalf("risk-flagged brief not listed:\n%s", out)
	}
	if !strings.Contains(memberLine, "risk: customer, irreversible") {
		t.Fatalf("member line missing the canonical-order risk reason: %q", memberLine)
	}
	if !strings.Contains(memberLine, "Evidence-only (never flip-eligible)") {
		t.Fatalf("member line missing the Evidence-only marker: %q", memberLine)
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
		"-- awaiting-human / ROUTE-HUMAN (1):",
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
