package main

// wake_test.go — the desk-supervision/16 acceptance tests (verification wake conditions).
//
// THE DEFECT. A failed or blocked verification was re-listed as a DISPATCH candidate every plan
// and re-run each pass, reproducing the identical non-verdict, even though nothing its outcome
// depends on had changed. These four tests pin the fix: a complete, still-UNCHANGED wake receipt
// keeps the failure visible as a WAIT row but excludes it from dispatch (survives a restart); a
// changed relevant input / tool / Verify definition / completed action wakes it while an
// unrelated change does not; unreadable inputs stay could-not-check and legacy/incomplete
// receipts stay unclassified (never a fabricated unchanged claim); and a partial run of one
// newly-runnable row never closes the whole brief.
//
// FAIL-FIRST: these assert behaviour introduced by this PR (the WAKE disposition and the receipt
// evaluator), so a pre-fix checkout does not compile them. The observed red is produced by
// MUTATION instead — see the PR body's ## Fail-first section, which quotes the failing runs from
// neutering the classifier's wake switch (reddens 1/3/4) and the reader's change detection
// (reddens 2).

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// fakeWake is an injected WakeInputs reader: revs maps an input key to its CURRENT revision (an
// absent key, or one listed in unreadable, is could-not-check); actionDone maps a referenced
// action to whether it has completed (an absent ref is unobservable — could-not-check).
type fakeWake struct {
	revs       map[string]string
	unreadable map[string]bool
	actionDone map[string]bool
}

func (f fakeWake) Revision(in string) (string, bool) {
	if f.unreadable[in] {
		return "", false
	}
	r, ok := f.revs[in]
	return r, ok
}

func (f fakeWake) ActionCompleted(ref string) (bool, bool) {
	d, ok := f.actionDone[ref]
	return d, ok
}

func fixedClock(rfc3339 string) func() time.Time {
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		panic(err)
	}
	return func() time.Time { return t }
}

// writeReceipts writes the append-only verify-outcomes sidecar from the given receipts, one JSONL
// line each — the same shape the verifier Land path appends.
func writeReceipts(t *testing.T, root string, recs ...deskkit.WakeReceipt) {
	t.Helper()
	var b strings.Builder
	for _, r := range recs {
		line, err := r.MarshalLine()
		if err != nil {
			t.Fatal(err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	p := filepath.Join(root, "docs", "streams", outcomeSidecarName)
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// dispositionsOf runs the loop's real board read + classifier for every awaiting item and returns
// each item's disposition plus the count that reached DISPATCH. It uses the SAME SelectQueue and
// classifyItem the plan does, with the loop's injected reader + clock.
func dispositionsOf(t *testing.T, v *VerifyLoop) (map[string]disposition, map[string]string, int) {
	t.Helper()
	items, err := v.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	disp := map[string]disposition{}
	reason := map[string]string{}
	dispatch := 0
	for _, it := range items {
		tier, err := v.TierPolicy(it)
		if err != nil {
			t.Fatalf("TierPolicy(%s): %v", it.ID, err)
		}
		d, r := classifyItem(it, tier)
		disp[it.ID] = d
		reason[it.ID] = r
		if d == dispDispatch {
			dispatch++
		}
	}
	return disp, reason, dispatch
}

// planBriefRows builds a risk-clear, gate:model brief with nrows Verify rows (all offline-runnable)
// and an empty Evidence section, so it is a tier-1 DISPATCH candidate absent a wake receipt.
func planBriefRows(nrows int) string {
	var b strings.Builder
	b.WriteString("---\nbrief: x\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\neffort: S\n---\n\n# Brief\n\n## Verify\n\n| # | Command | Expect |\n|---|---------|--------|\n")
	for i := 1; i <= nrows; i++ {
		n := strconv.Itoa(i)
		b.WriteString("| " + n + " | `go test -run T" + n + "` | exit 0 |\n")
	}
	b.WriteString("\n## Evidence\n<!-- appended at verification time -->\n")
	return b.String()
}

// Test 1 — a complete, unchanged wake receipt is a VISIBLE WAIT, never a dispatch, and the hold
// survives a process restart (the receipt lives in the append-only sidecar, the classifier is
// stateless — a fresh VerifyLoop re-reading the same file reaches the same verdict).
func TestVerifyWakeUnchangedIsVisibleWait(t *testing.T) {
	root := planFixtureRoot(t, map[string]string{
		"01": planBrief("model", "no", "no", "no", "no"), // no receipt → would DISPATCH
	})
	writeReceipts(t, root, deskkit.NewWakeReceipt(
		"r-01", "medici-finance/assay", "example-stream/01", "assay-verifier-app[bot]",
		"verify-fail", "0000abc", nil, map[string]string{"file:pkg/x.go": "rev1"},
		"desk-tools/v1.0.16", deskkit.BlockerImplementation, "tracker#100",
		deskkit.WakeRelevantInputChanged, "", "", "2026-09-20T00:00:00Z"))
	reader := fakeWake{revs: map[string]string{"file:pkg/x.go": "rev1"}} // unchanged

	for _, pass := range []string{"first pass", "after process restart (fresh loop, re-read sidecar)"} {
		v := &VerifyLoop{Root: root, WakeReader: reader, Now: fixedClock("2026-09-20T12:00:00Z")}
		disp, reason, dispatchN := dispositionsOf(t, v)
		if disp["example-stream/01"] != dispWaitReceipt {
			t.Fatalf("%s: brief classified %v (reason %q); want a WAIT", pass, disp["example-stream/01"], reason["example-stream/01"])
		}
		if dispatchN != 0 {
			t.Fatalf("%s: %d dispatch(es); want 0 — an unchanged blocked receipt must not consume a verifier run", pass, dispatchN)
		}
		if !strings.Contains(reason["example-stream/01"], "blocker: implementation") ||
			!strings.Contains(reason["example-stream/01"], "next: worker") {
			t.Fatalf("%s: WAIT line does not name the blocker and next actor: %q", pass, reason["example-stream/01"])
		}
	}
}

// Test 2 — a changed relevant input, tool version, Verify definition, or completed action WAKES
// the affected work; an unrelated change (a declared input still unchanged) does NOT.
func TestVerifyWakeRelevantChange(t *testing.T) {
	root := planFixtureRoot(t, map[string]string{
		"01": planBrief("model", "no", "no", "no", "no"), // relevant file changed → wake
		"02": planBrief("model", "no", "no", "no", "no"), // tool version changed → wake
		"03": planBrief("model", "no", "no", "no", "no"), // Verify definition changed → wake
		"04": planBrief("model", "no", "no", "no", "no"), // referenced action completed → wake
		"05": planBrief("model", "no", "no", "no", "no"), // declared input unchanged → HOLD (unrelated commit)
	})
	rel := func(num, pred string, inputs map[string]string, ref string) deskkit.WakeReceipt {
		return deskkit.NewWakeReceipt("r-"+num, "medici-finance/assay", "example-stream/"+num,
			"assay-verifier-app[bot]", "verify-fail", "0000abc", nil, inputs,
			"desk-tools/v1.0.16", deskkit.BlockerCheckDef, ref, pred, "", "", "2026-09-20T00:00:00Z")
	}
	writeReceipts(t, root,
		rel("01", deskkit.WakeRelevantInputChanged, map[string]string{"file:pkg/a.go": "rev1"}, ""),
		rel("02", deskkit.WakeRelevantInputChanged, map[string]string{"tool": "desk-tools/v1.0.15"}, ""),
		rel("03", deskkit.WakeRelevantInputChanged, map[string]string{"verify-def:example-stream/03": "vdef1"}, ""),
		rel("04", deskkit.WakeReferencedActionDone, nil, "tracker#200"),
		rel("05", deskkit.WakeRelevantInputChanged, map[string]string{"file:pkg/b.go": "rev1"}, ""),
	)
	reader := fakeWake{
		revs: map[string]string{
			"file:pkg/a.go":                "rev2",               // CHANGED → wake 01
			"tool":                         "desk-tools/v1.0.16", // CHANGED → wake 02
			"verify-def:example-stream/03": "vdef2",              // CHANGED → wake 03
			"file:pkg/b.go":                "rev1",               // UNCHANGED → hold 05 (an unrelated file moved)
		},
		actionDone: map[string]bool{"tracker#200": true}, // COMPLETED → wake 04
	}
	v := &VerifyLoop{Root: root, WakeReader: reader, Now: fixedClock("2026-09-20T12:00:00Z")}
	disp, reason, dispatchN := dispositionsOf(t, v)

	for _, id := range []string{"example-stream/01", "example-stream/02", "example-stream/03", "example-stream/04"} {
		if disp[id] != dispDispatch {
			t.Fatalf("%s classified %v (reason %q); want DISPATCH — its wake condition was met", id, disp[id], reason[id])
		}
	}
	if disp["example-stream/05"] != dispWaitReceipt {
		t.Fatalf("example-stream/05 classified %v; want WAIT — its declared input is unchanged, an unrelated change must not wake it", disp["example-stream/05"])
	}
	if dispatchN != 4 {
		t.Fatalf("%d dispatch(es); want exactly 4 (the four woken briefs, never the held one)", dispatchN)
	}
}

// Test 3 — unreadable declared inputs are COULD-NOT-CHECK (never rounded up to unchanged), and a
// legacy row / incomplete receipt stays UNCLASSIFIED and eligible for one classification pass —
// both distinct from an empty queue or a pass. No path fabricates an "unchanged" claim.
func TestVerifyWakeUnknownAndLegacy(t *testing.T) {
	root := planFixtureRoot(t, map[string]string{
		"01": planBrief("model", "no", "no", "no", "no"), // complete receipt, input UNREADABLE → could-not-check
		"02": planBrief("model", "no", "no", "no", "no"), // legacy outcome row (no wake fields) → unclassified → one pass
		"03": planBrief("model", "no", "no", "no", "no"), // incomplete receipt (empty input scope) → unclassified → one pass
	})
	complete := deskkit.NewWakeReceipt("r-01", "medici-finance/assay", "example-stream/01",
		"assay-verifier-app[bot]", "verify-fail", "0000abc", nil, map[string]string{"file:pkg/c.go": "rev1"},
		"desk-tools/v1.0.16", deskkit.BlockerImplementation, "", deskkit.WakeRelevantInputChanged, "", "", "2026-09-20T00:00:00Z")
	// A LEGACY row: only the four original verify-outcomes fields, no schema, no wake fields.
	legacy := deskkit.WakeReceipt{TS: "2026-09-20T00:00:00Z", Brief: "example-stream/02", Outcome: "verify-fail", SHA: "0000def"}
	// An INCOMPLETE receipt: schema + ids present, but relevant-input-changed with an EMPTY input
	// scope cannot establish unchangedness — it is not complete.
	incomplete := deskkit.WakeReceipt{
		TS: "2026-09-20T00:00:00Z", Brief: "example-stream/03", Outcome: "verify-fail", SHA: "0000ghi",
		Schema: deskkit.SchemaWakeV1, ID: "r-03", BlockerKind: deskkit.BlockerImplementation,
		WakePredicate: deskkit.WakeRelevantInputChanged, Inputs: nil,
	}
	writeReceipts(t, root, complete, legacy, incomplete)
	reader := fakeWake{unreadable: map[string]bool{"file:pkg/c.go": true}} // the declared input cannot be read

	v := &VerifyLoop{Root: root, WakeReader: reader, Now: fixedClock("2026-09-20T12:00:00Z")}
	disp, reason, _ := dispositionsOf(t, v)

	if disp["example-stream/01"] != dispCouldNotCheck {
		t.Fatalf("unreadable-input brief classified %v (reason %q); want could-not-check", disp["example-stream/01"], reason["example-stream/01"])
	}
	if disp["example-stream/01"] == dispWaitReceipt {
		t.Fatalf("unreadable input must NOT be rounded up to an unchanged WAIT")
	}
	for _, id := range []string{"example-stream/02", "example-stream/03"} {
		if disp[id] != dispDispatch {
			t.Fatalf("%s classified %v; want DISPATCH — a legacy/incomplete receipt gets one classification pass, never a fabricated hold", id, disp[id])
		}
		if disp[id] == dispWaitReceipt {
			t.Fatalf("%s must not be held on a legacy/incomplete receipt (no fabricated unchanged claim)", id)
		}
	}
}

// Test 4 — a receipt that holds only SOME rows lets the newly-runnable row execute (recorded so
// the held rows are not repeated), and no partial result closes the whole brief: Land flips ONLY
// on a whole-brief PASS, never on a FAIL/BLOCKED partial.
func TestVerifyWakePartialRows(t *testing.T) {
	root := planFixtureRoot(t, map[string]string{
		"01": planBriefRows(3), // rows 1,2,3
	})
	// The receipt holds rows 2 and 3; its declared input is unchanged, so those rows stay held.
	writeReceipts(t, root, deskkit.NewWakeReceipt(
		"r-01", "medici-finance/assay", "example-stream/01", "assay-verifier-app[bot]",
		"verify-fail", "0000abc", []int{2, 3}, map[string]string{"file:pkg/x.go": "rev1"},
		"desk-tools/v1.0.16", deskkit.BlockerImplementation, "tracker#100",
		deskkit.WakeRelevantInputChanged, "", "", "2026-09-20T00:00:00Z"))
	reader := fakeWake{revs: map[string]string{"file:pkg/x.go": "rev1"}} // unchanged → rows 2,3 stay held

	v := &VerifyLoop{Root: root, WakeReader: reader, Now: fixedClock("2026-09-20T12:00:00Z")}
	items, err := v.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want one item, got %d", len(items))
	}
	it := items[0]
	tier, _ := v.TierPolicy(it)
	if d, _ := classifyItem(it, tier); d != dispDispatch {
		t.Fatalf("partial-hold brief classified %v; want DISPATCH — row 1 is newly runnable", d)
	}
	if held := it.Payload["wake_held_rows"]; held != "2,3" {
		t.Fatalf("held rows = %q; want \"2,3\" recorded so they are not repeated", held)
	}
	prompt := renderDispatchPrompt(it, tier)
	if !strings.Contains(prompt, "WAKE-HELD") || !strings.Contains(prompt, "2,3") {
		t.Fatalf("dispatch prompt must record the held rows as explicitly unrun:\n%s", prompt)
	}

	// No partial result closes the whole brief: Land flips ONLY on a whole-brief PASS. A partial
	// run of the newly-runnable row alone is never a whole-brief PASS — Land sees FAIL/BLOCKED and
	// never flips. Prove Land takes no flip on those verdicts.
	for _, verdict := range []string{loopengine.VerdictBlocked, loopengine.VerdictFail} {
		cap := &captureDurable{}
		vl := &VerifyLoop{DurableSink: cap, Now: fixedClock("2026-09-20T12:00:00Z"), RunnerID: "assay-verifier-app[bot]"}
		if err := vl.Land(loopengine.Result{Item: it, Verdict: verdict, RunnerID: "assay-verifier-app[bot]",
			Rows: []loopengine.EvidenceRow{{Command: "go test -run T1", Exit: 0, Output: "ok"}}}); err != nil {
			t.Fatalf("Land(%s): %v", verdict, err)
		}
		if cap.flipped {
			t.Fatalf("Land(%s) flipped the brief — a partial/failed result must never close the whole brief", verdict)
		}
	}
}

// captureDurable records whether a status flip was ever requested, so a test can assert a partial
// or failed result never closes a brief. It is a no-op sink otherwise.
type captureDurable struct{ flipped bool }

func (c *captureDurable) WriteEvidenceAndFlip(_ loopengine.Item, _ string, flip bool) error {
	if flip {
		c.flipped = true
	}
	return nil
}
func (c *captureDurable) Checkpoint(loopengine.Item, string) (string, error) {
	return "checkpoint://x", nil
}
func (c *captureDurable) RouteHuman(loopengine.Item) (string, error)      { return "issue://x", nil }
func (c *captureDurable) FileBug(loopengine.Item, string) (string, error) { return "issue://bug", nil }
