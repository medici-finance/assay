package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// The same complete partial-hold receipt must not spend another verifier slot
// reproducing PASS rows, either on the next selection or after a restart.
// regression: #1731
func TestPartialWakeTwoSweeps(t *testing.T) {
	for _, detailed := range []bool{false, true} {
		name := "counts-only"
		if detailed {
			name = "exact-results"
		}
		t.Run(name, func(t *testing.T) {
			root := planFixtureRoot(t, map[string]string{"01": planBriefRows(4)})
			rec := deskkit.NewWakeReceipt("partial-1731", "medici-finance/assay", "example-stream/01",
				"example-verifier", "blocked", "0000abc", []int{1, 2}, map[string]string{"file:pkg/x.go": "rev1"},
				"test-version", deskkit.BlockerEnvironment, "#100", deskkit.WakeRelevantInputChanged, "", "", holdTS)
			reader := fakeWake{revs: map[string]string{"file:pkg/x.go": "rev1"}}
			v := &VerifyLoop{Root: root, WakeReader: reader, Issues: openIssues, Now: fixedClock("2026-09-20T12:00:00Z")}
			writeReceipts(t, root, rec)
			disp, _, _ := dispositionsOf(t, v)
			if disp["example-stream/01"] != dispDispatch {
				t.Fatal("control: unobserved non-held rows must dispatch")
			}
			rec.RowsTotal, rec.RowsPassed = 4, 2
			if detailed {
				if err := json.Unmarshal([]byte(`[{"row":1,"result":"could-not-run"},{"row":2,"result":"could-not-run"},{"row":3,"result":"pass"},{"row":4,"result":"pass"}]`), &rec.RowResults); err != nil {
					t.Fatal(err)
				}
			}
			writeReceipts(t, root, rec)
			for sweep := 0; sweep < 2; sweep++ {
				disp, reason, _ := dispositionsOf(t, v)
				if disp["example-stream/01"] != dispWaitReceipt {
					t.Fatalf("sweep %d: got %v (%s), want WAIT; all non-held rows already passed", sweep+1, disp["example-stream/01"], reason["example-stream/01"])
				}
				if !strings.Contains(reason["example-stream/01"], "#100") || !strings.Contains(reason["example-stream/01"], "next:") {
					t.Fatalf("WAIT loses blocker/next actor: %s", reason["example-stream/01"])
				}
			}
			restarted := &VerifyLoop{Root: root, WakeReader: reader, Issues: openIssues, Now: v.Now}
			disp, _, _ = dispositionsOf(t, restarted)
			if disp["example-stream/01"] != dispWaitReceipt {
				t.Fatal("restart repeated completed rows")
			}
		})
	}
}

// regression: #1731
func TestPartialWakeLetteredReceiptRoundTrip(t *testing.T) {
	raw := []byte(`{"brief":"example-stream/01","rows":[6,"6a"],"row_results":[{"row":6,"result":"pass"},{"row":"6a","result":"could-not-run"}]}`)
	var rec deskkit.WakeReceipt
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatalf("exact row identities must decode: %v", err)
	}
	out, err := rec.MarshalLine()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"rows":[6,"6a"]`) {
		t.Fatalf("numeric and lettered rows aliased/lost: %s", out)
	}
}

// regression: #1731
func TestPartialWakeExactIdentitiesAndFreshness(t *testing.T) {
	brief := strings.Replace(planBriefRows(3), "| 2 |", "| 6 |", 1)
	brief = strings.Replace(brief, "| 3 |", "| 6a |", 1)
	root := planFixtureRoot(t, map[string]string{"01": brief})
	var rec deskkit.WakeReceipt
	raw := `{"brief":"example-stream/01","outcome":"blocked","rows":["6a"],"rows_passed":2,"rows_total":3,"row_results":[{"row":1,"result":"pass"},{"row":6,"result":"pass"},{"row":"6a","result":"could-not-run"}]}`
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		t.Fatal(err)
	}
	base := heldReceipt(holdTS)
	rec.Schema, rec.ID, rec.Inputs = base.Schema, base.ID, base.Inputs
	rec.BlockerKind, rec.BlockerRef, rec.WakePredicate = base.BlockerKind, base.BlockerRef, base.WakePredicate
	rec.TS, rec.Repo, rec.Verifier, rec.SHA = base.TS, base.Repo, base.Verifier, base.SHA
	reader := fakeWake{revs: map[string]string{"file:pkg/x.go": "rev1"}}
	writeReceipts(t, root, rec)
	v := &VerifyLoop{Root: root, WakeReader: reader, Issues: openIssues, Now: fixedClock("2026-09-20T12:00:00Z")}
	for sweep := 0; sweep < 2; sweep++ {
		items, err := v.SelectQueue()
		if err != nil || len(items) != 1 {
			t.Fatalf("select: %v %#v", err, items)
		}
		tier, _ := v.TierPolicy(items[0])
		d, _ := classifyItem(items[0], tier)
		if d != dispWaitReceipt || items[0].Payload["wake_held_rows"] != "6a" {
			t.Fatalf("sweep %d lost exact held row: %v %#v", sweep, d, items[0].Payload)
		}
	}
	for _, tc := range []struct {
		name   string
		reader deskkit.WakeInputs
		issues deskkit.IssueStateSource
		mut    func(*deskkit.WakeReceipt)
		want   disposition
	}{
		{"input-changed", fakeWake{revs: map[string]string{"file:pkg/x.go": "rev2"}}, openIssues, nil, dispDispatch},
		{"blocker-closed", reader, allIssues(deskkit.BlockerClosed), nil, dispDispatch},
		{"explicit-recheck", reader, openIssues, func(r *deskkit.WakeReceipt) {
			r.WakePredicate = deskkit.WakeExplicitRecheck
			r.RecheckReason = "changed toolchain"
		}, dispDispatch},
		{"input-unknown", fakeWake{}, openIssues, nil, dispCouldNotCheck},
		{"blocker-unknown", reader, allIssues(deskkit.BlockerCouldNotCheck), nil, dispCouldNotCheck},
		{"numeric-identity-is-not-lettered", reader, openIssues, func(r *deskkit.WakeReceipt) { r.Rows = []deskkit.VerifyRowID{"6"} }, dispCouldNotCheck},
		{"unknown-held", reader, openIssues, func(r *deskkit.WakeReceipt) { r.Rows = []deskkit.VerifyRowID{"9"} }, dispCouldNotCheck},
		{"duplicate-held", reader, openIssues, func(r *deskkit.WakeReceipt) { r.Rows = []deskkit.VerifyRowID{"6a", "6a"} }, dispCouldNotCheck},
		{"duplicate-result", reader, openIssues, func(r *deskkit.WakeReceipt) {
			r.RowResults = append([]deskkit.RowResult(nil), r.RowResults...)
			r.RowResults[0] = deskkit.RowResult{Row: "6a", Result: deskkit.RowCouldNotRun}
			r.RowsPassed = 1
		}, dispCouldNotCheck},
		{"missing-result", reader, openIssues, func(r *deskkit.WakeReceipt) { r.RowResults = r.RowResults[:2] }, dispCouldNotCheck},
		{"foreign-result", reader, openIssues, func(r *deskkit.WakeReceipt) {
			r.RowResults = append([]deskkit.RowResult(nil), r.RowResults...)
			r.RowResults[0].Row = "9"
		}, dispCouldNotCheck},
		{"counts-mismatch", reader, openIssues, func(r *deskkit.WakeReceipt) { r.RowsPassed = 1 }, dispCouldNotCheck},
		{"ambiguous-aggregate", reader, openIssues, func(r *deskkit.WakeReceipt) { r.RowResults = nil }, dispCouldNotCheck},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := rec
			if tc.mut != nil {
				tc.mut(&changed)
			}
			writeReceipts(t, root, changed)
			vl := &VerifyLoop{Root: root, WakeReader: tc.reader, Issues: tc.issues, Now: v.Now}
			disp, reason, _ := dispositionsOf(t, vl)
			if disp["example-stream/01"] != tc.want {
				t.Fatalf("got %v (%s), want %v", disp["example-stream/01"], reason["example-stream/01"], tc.want)
			}
		})
	}
}

// regression: #1731
func TestPartialWakeDispatchPreservesPassedRows(t *testing.T) {
	root := planFixtureRoot(t, map[string]string{"01": planBriefRows(3)})
	rec := heldReceipt(holdTS)
	if err := json.Unmarshal([]byte(`{"rows":[2],"rows_total":3,"rows_passed":1,"row_results":[{"row":1,"result":"pass"},{"row":2,"result":"could-not-run"},{"row":3,"result":"deferred"}]}`), &rec); err != nil {
		t.Fatal(err)
	}
	writeReceipts(t, root, rec)
	v := &VerifyLoop{Root: root, WakeReader: fakeWake{revs: map[string]string{"file:pkg/x.go": "rev1"}}, Issues: openIssues, Now: fixedClock("2026-09-20T12:00:00Z")}
	items, err := v.SelectQueue()
	if err != nil || len(items) != 1 {
		t.Fatalf("select: %v %#v", err, items)
	}
	it := items[0]
	tier, _ := v.TierPolicy(it)
	d, _ := classifyItem(it, tier)
	if d != dispDispatch || it.Payload["wake_runnable_rows"] != "3" || it.Payload["wake_passed_rows"] != "1" || it.Payload["wake_held_rows"] != "2" {
		t.Fatalf("partial coverage lost: %v %#v", d, it.Payload)
	}
	prompt := renderDispatchPrompt(it, tier)
	if strings.Contains(prompt, "Verify table — run EVERY row") || !strings.Contains(prompt, "account for EVERY row") {
		t.Fatalf("partial execution precedence is ambiguous: %s", prompt)
	}
	if !strings.Contains(prompt, "Newly runnable row identities under the unchanged partial receipt: 3") || !strings.Contains(prompt, "Prior receipt PASS observations") || !strings.Contains(prompt, "never infer a whole-brief PASS") {
		t.Fatalf("dispatch lost observation boundaries: %s", prompt)
	}
}

// regression: #1731
func TestPartialWakeLegacyRequiresUnchangedInputs(t *testing.T) {
	for _, tc := range []struct {
		name string
		revs map[string]string
		want disposition
	}{
		{"same", map[string]string{"file:pkg/x.go": "rev1"}, dispWaitReceipt},
		{"changed", map[string]string{"file:pkg/x.go": "rev2"}, dispDispatch},
		{"unknown", nil, dispCouldNotCheck},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := planFixtureRoot(t, map[string]string{"01": planBriefRows(2)})
			rec := heldReceipt(holdTS)
			rec.Rows = []deskkit.VerifyRowID{"2"}
			rec.RowsPassed = 1
			rec.RowsTotal = 2
			rec.WakePredicate = deskkit.WakeReferencedActionDone
			writeReceipts(t, root, rec)
			v := &VerifyLoop{Root: root, WakeReader: fakeWake{revs: tc.revs, actionDone: map[string]bool{"#100": false}}, Issues: openIssues, Now: fixedClock("2026-09-20T12:00:00Z")}
			disp, reason, _ := dispositionsOf(t, v)
			if disp["example-stream/01"] != tc.want {
				t.Fatalf("got %v (%s),want %v", disp["example-stream/01"], reason["example-stream/01"], tc.want)
			}
		})
	}
}

// regression: #1731
func TestPartialWakeExecutionLexemesStayCompatible(t *testing.T) {
	brief := "## Verify\n\n| # | Command | Expect |\n|---|---|---|\n| 1 | `one` | exit 0 |\n| 06 | `six` | exit 0 |\n| +7 | `seven` | exit 0 |\n| 0 | `zero` | exit 0 |\n| -1 | `minus` | exit 0 |\n| 6a | `lettered` | exit 0 |\n"
	rows := parseVerifyRows(brief)
	want := []int{1, 6, 7, 0, -1}
	if len(rows) != len(want) {
		t.Fatalf("legacy numeric execution rows dropped: got %#v", rows)
	}
	for i, row := range rows {
		if row.Num != want[i] {
			t.Fatalf("row %d: got %d want %d", i, row.Num, want[i])
		}
	}
}

// regression: #1731
func TestPartialWakeInvalidTableRowsAreNotOmitted(t *testing.T) {
	for _, id := range []string{"0", "06", "+7", "-1", "2.1", "R-2", "6a1", "２"} {
		t.Run(id, func(t *testing.T) {
			brief := strings.Replace(planBriefRows(2), "| 2 |", "| "+id+" |", 1)
			root := planFixtureRoot(t, map[string]string{"01": brief})
			rec := heldReceipt(holdTS)
			rec.Rows = []deskkit.VerifyRowID{"1"}
			rec.RowsTotal = 1
			writeReceipts(t, root, rec)
			v := &VerifyLoop{Root: root, WakeReader: fakeWake{revs: map[string]string{"file:pkg/x.go": "rev1"}}, Issues: openIssues, Now: fixedClock("2026-09-20T12:00:00Z")}
			disp, why, _ := dispositionsOf(t, v)
			if disp["example-stream/01"] != dispCouldNotCheck {
				t.Fatalf("table row %q omitted: %v (%s)", id, disp["example-stream/01"], why["example-stream/01"])
			}
		})
	}
}

// regression: #1731
func TestPartialWakeMalformedLatestNeverFallsBack(t *testing.T) {
	root := planFixtureRoot(t, map[string]string{"01": planBriefRows(2), "02": planBriefRows(2)})
	old := heldReceipt(holdTS)
	old.Rows = []deskkit.VerifyRowID{"2"}
	good, err := old.MarshalLine()
	if err != nil {
		t.Fatal(err)
	}
	// Keep the legacy timestamp reducer in the path: the newer malformed receipt,
	// not the older well-formed hold, must determine the visible uncertainty.
	bad := strings.Replace(string(good), `"rows":[2]`, `"rows":[0]`, 1)
	bad = strings.Replace(bad, holdTS, "2026-09-20T06:00:00Z", 1)
	if err := os.WriteFile(filepath.Join(root, "docs/streams/verify-outcomes.jsonl"), []byte(string(good)+"\n"+bad+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	v := &VerifyLoop{Root: root, WakeReader: fakeWake{revs: map[string]string{"file:pkg/x.go": "rev1"}}, Issues: openIssues, Now: fixedClock("2026-09-20T12:00:00Z")}
	disp, why, _ := dispositionsOf(t, v)
	if disp["example-stream/01"] != dispCouldNotCheck {
		t.Fatalf("malformed latest receipt disappeared: %v (%s)", disp["example-stream/01"], why["example-stream/01"])
	}
	if disp["example-stream/02"] != dispDispatch {
		t.Fatal("malformed receipt must not quarantine an unrelated healthy brief")
	}
}

// regression: #1731
func TestPartialWakeCountBounds(t *testing.T) {
	for _, tc := range []struct {
		name          string
		total, passed int
	}{
		{"wrong-total", 3, 2}, {"above-remainder", 4, 3}, {"negative-pass", 4, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := planFixtureRoot(t, map[string]string{"01": planBriefRows(4)})
			rec := heldReceipt(holdTS)
			rec.Rows = []deskkit.VerifyRowID{"1", "2"}
			rec.RowsTotal = tc.total
			rec.RowsPassed = tc.passed
			writeReceipts(t, root, rec)
			v := &VerifyLoop{Root: root, WakeReader: fakeWake{revs: map[string]string{"file:pkg/x.go": "rev1"}}, Issues: openIssues, Now: fixedClock("2026-09-20T12:00:00Z")}
			disp, why, _ := dispositionsOf(t, v)
			if disp["example-stream/01"] != dispCouldNotCheck {
				t.Fatalf("bad count accepted: %v (%s)", disp["example-stream/01"], why["example-stream/01"])
			}
		})
	}
}

// regression: #1731
func TestPartialWakeDeferredRowsNeverRunnable(t *testing.T) {
	for _, allDeferred := range []bool{false, true} {
		brief := strings.Replace(planBriefRows(4), "| 3 |", "| 6a |", 1)
		rows := parseVerifyRowsIn(extractVerify(brief))
		brief = strings.Replace(brief, rows[2].Command, "kubectl get pods", 1)
		if allDeferred {
			brief = strings.Replace(brief, rows[3].Command, "kubectl get nodes", 1)
		}
		root := planFixtureRoot(t, map[string]string{"01": brief})
		rec := heldReceipt(holdTS)
		if err := json.Unmarshal([]byte(`{"rows":[2],"rows_total":4,"rows_passed":1,"row_results":[{"row":1,"result":"pass"},{"row":2,"result":"could-not-run"},{"row":"6a","result":"deferred"},{"row":4,"result":"deferred"}]}`), &rec); err != nil {
			t.Fatal(err)
		}
		writeReceipts(t, root, rec)
		v := &VerifyLoop{Root: root, WakeReader: fakeWake{revs: map[string]string{"file:pkg/x.go": "rev1"}}, Issues: openIssues, Now: fixedClock("2026-09-20T12:00:00Z")}
		items, err := v.SelectQueue()
		if err != nil || len(items) != 1 {
			t.Fatalf("select: %v %#v", err, items)
		}
		it := items[0]
		tier, _ := v.TierPolicy(it)
		disp, _ := classifyItem(it, tier)
		if allDeferred {
			if disp != dispWaitReceipt {
				t.Fatalf("deferred-only remainder dispatched: %v %#v", disp, it.Payload)
			}
			continue
		}
		if disp != dispDispatch || it.Payload["wake_runnable_rows"] != "4" || it.Payload["wake_passed_rows"] != "1" {
			t.Fatalf("deferred identity treated as runnable/PASS: %v %#v", disp, it.Payload)
		}
		prompt := renderDispatchPrompt(it, tier)
		if strings.Contains(prompt, "run every other row") || !strings.Contains(prompt, "explicitly UNRUN (not runnable offline): 6a:") || !strings.Contains(prompt, "newly runnable identities") {
			t.Fatalf("contradictory deferred prompt: %s", prompt)
		}
	}
}

// regression: #1731
func TestPartialWakeMalformedReasonPreservesEarlierUncertainty(t *testing.T) {
	root := planFixtureRoot(t, map[string]string{"01": planBriefRows(2)})
	rec := heldReceipt(holdTS)
	rec.Rows = []deskkit.VerifyRowID{"2"}
	good, err := rec.MarshalLine()
	if err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(string(good), `"rows":[2]`, `"rows_total":"bad"`, 1)
	future := strings.Replace(string(good), holdTS, time.Now().UTC().Add(48*time.Hour).Format(time.RFC3339), 1)
	path := filepath.Join(root, "docs/streams/verify-outcomes.jsonl")
	v := &VerifyLoop{Root: root, WakeReader: fakeWake{revs: map[string]string{"file:pkg/x.go": "rev1"}}, Issues: openIssues, Now: fixedClock("2026-09-20T12:00:00Z")}
	if err := os.WriteFile(path, []byte(bad+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	disp, why, _ := dispositionsOf(t, v)
	if disp["example-stream/01"] != dispCouldNotCheck || !strings.Contains(why["example-stream/01"], "could not decode") || strings.Contains(why["example-stream/01"], "row identities") {
		t.Fatalf("misleading decode reason: %v %#v", disp, why)
	}
	if err := os.WriteFile(path, []byte(bad+"\n"+future+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	disp, why, _ = dispositionsOf(t, v)
	if disp["example-stream/01"] != dispCouldNotCheck || !strings.Contains(why["example-stream/01"], "clock-skew") {
		t.Fatalf("earlier clock uncertainty overwritten: %v %#v", disp, why)
	}
}

// regression: #1731
func TestPartialWakePendingDeferredMatchesScan(t *testing.T) {
	for _, allDeferred := range []bool{false, true} {
		brief := strings.Replace(planBriefRows(4), "| 3 |", "| 6a |", 1)
		brief = strings.Replace(brief, "go test -run T3", "kubectl get pods", 1)
		if allDeferred {
			brief = strings.Replace(brief, "go test -run T4", "kubectl get nodes", 1)
		}
		v, f, r, p := pendingSetupPlan(t, brief)
		pendingRecord(t, v, f, r, p, func(w *deskkit.WakeReceipt) {
			w.Rows = []deskkit.VerifyRowID{"2"}
			w.RowsTotal = 4
			w.RowsPassed = 1
			w.RowResults = []deskkit.RowResult{{Row: "1", Result: deskkit.RowPass}, {Row: "2", Result: deskkit.RowCouldNotRun}, {Row: "6a", Result: deskkit.RowDeferred}, {Row: "4", Result: deskkit.RowDeferred}}
		})
		commitIn(t, v.Root, "unrelated")
		want := "dispatch"
		if allDeferred {
			want = "awaiting-merge"
		}
		for sweep := 0; sweep < 2; sweep++ {
			var it loopengine.Item
			// Exercise the actual admitted forge path, not just a hand-built payload.
			items, err := v.SelectQueue()
			if err != nil {
				t.Fatal(err)
			}
			for _, item := range items {
				if item.ID == "example-stream/01" {
					it = item
				}
			}
			tier, _ := v.TierPolicy(it)
			disp, why := classifyItem(it, tier)
			if disp.String() != want {
				t.Fatalf("allDeferred=%v sweep%d: got%s (%s), want%s", allDeferred, sweep, disp, why, want)
			}
		}
	}
}

// regression: #1731
func TestPartialWakeOutcomeProducerExample(t *testing.T) {
	root := planFixtureRoot(t, map[string]string{"01": planBriefRows(3)})
	it := selectOne(t, root, "example-stream/01")
	tier, _ := (&VerifyLoop{}).TierPolicy(it)
	prompt := renderDispatchPrompt(it, tier)
	_, tail, ok := strings.Cut(prompt, "Outcome row metadata example: ")
	if !ok {
		t.Fatal("producer row contract missing")
	}
	raw, _, _ := strings.Cut(tail, "\n")
	if _, err := deskkit.ParseJoinInput([]byte(raw)); err != nil {
		t.Fatalf("producer guidance violates writer contract: %v", err)
	}
	var record deskkit.WakeReceipt
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		t.Fatal(err)
	}
	if len(record.Rows) != 1 || record.Rows[0] != "2" || record.RowsTotal != 3 || record.RowsPassed != 2 || len(record.RowResults) != 3 || record.RowResults[2].Row != "6a" {
		t.Fatalf("producer lost held/covered identity distinction: %#v", record)
	}
	if !strings.Contains(prompt, "never list every covered row in rows") || !strings.Contains(prompt, "verified record omits rows") {
		t.Fatal("producer semantics missing")
	}
}

// regression: #1731
func TestPartialWakeOutcomeProducerRequiresExactHeldResults(t *testing.T) {
	root := planFixtureRoot(t, map[string]string{"01": planBriefRows(3)})
	it := selectOne(t, root, "example-stream/01")
	tier, _ := (&VerifyLoop{}).TierPolicy(it)
	prompt := renderDispatchPrompt(it, tier)
	if !strings.Contains(prompt, "When rows is non-empty, row_results is required for ALL original Verify IDs and their observed results") {
		t.Fatal("producer must require exact results for every non-empty held set")
	}
	_, tail, ok := strings.Cut(prompt, "Gapped row metadata example: ")
	if !ok {
		t.Fatal("gapped inventory producer example missing")
	}
	raw, _, _ := strings.Cut(tail, "\n")
	if _, err := deskkit.ParseJoinInput([]byte(raw)); err != nil {
		t.Fatalf("gapped producer example violates writer contract: %v", err)
	}
	var rec deskkit.WakeReceipt
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		t.Fatal(err)
	}
	if len(rec.Rows) != 1 || rec.Rows[0] != "10" || rec.RowsTotal != 9 || rec.RowsPassed != 8 || len(rec.RowResults) != 9 || rec.RowResults[8].Row != "10" {
		t.Fatalf("gapped producer lost exact held/inventory identities: %#v", rec)
	}
	// Counts alone cannot bind held row 10 to a nine-row inventory. The producer
	// must keep the exact results; do not relax the writer to accept this shape.
	var countOnly map[string]any
	if err := json.Unmarshal([]byte(raw), &countOnly); err != nil {
		t.Fatal(err)
	}
	delete(countOnly, "row_results")
	withoutResults, err := json.Marshal(countOnly)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deskkit.ParseJoinInput(withoutResults); err == nil {
		t.Fatal("gapped held set accepted without required exact results")
	}
}

// A table labelled with letter IDs schedules by those exact labels, the same
// identities verifyrun records and the outcome-record writer accepts.
// regression: #5013
func TestPartialWakeLetterIdentities(t *testing.T) {
	brief := strings.Replace(planBriefRows(3), "| 2 |", "| R |", 1)
	brief = strings.Replace(brief, "| 3 |", "| S1 |", 1)
	root := planFixtureRoot(t, map[string]string{"01": brief})
	var rec deskkit.WakeReceipt
	raw := `{"brief":"example-stream/01","outcome":"blocked","rows":["R"],"rows_passed":2,"rows_total":3,"row_results":[{"row":1,"result":"pass"},{"row":"R","result":"could-not-run"},{"row":"S1","result":"pass"}]}`
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		t.Fatal(err)
	}
	base := heldReceipt(holdTS)
	rec.Schema, rec.ID, rec.Inputs = base.Schema, base.ID, base.Inputs
	rec.BlockerKind, rec.BlockerRef, rec.WakePredicate = base.BlockerKind, base.BlockerRef, base.WakePredicate
	rec.TS, rec.Repo, rec.Verifier, rec.SHA = base.TS, base.Repo, base.Verifier, base.SHA
	writeReceipts(t, root, rec)
	v := &VerifyLoop{Root: root, WakeReader: fakeWake{revs: map[string]string{"file:pkg/x.go": "rev1"}}, Issues: openIssues, Now: fixedClock("2026-09-20T12:00:00Z")}
	items, err := v.SelectQueue()
	if err != nil || len(items) != 1 {
		t.Fatalf("select: %v %#v", err, items)
	}
	tier, _ := v.TierPolicy(items[0])
	if d, _ := classifyItem(items[0], tier); d != dispWaitReceipt || items[0].Payload["wake_held_rows"] != "R" {
		t.Fatalf("lost exact held label: %v %#v", d, items[0].Payload)
	}
}
