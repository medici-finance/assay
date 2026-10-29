package main

// flow_45_test.go — the verifyloop half of the verification-queue contract (brief 45).
//
// The classification is statusgen's. The fixture is the one the statusgen and deskboard tests
// share: the tree statusgen/testdata/flowqueue and the rows statusgen printed for it
// (gate-scores.golden.json), served here through the baseQueueRows seam. One fixture, so the
// three consumers are checked against the same briefs and cannot drift apart.

import (
	"bytes"
	"encoding/json"
	vf "github.com/medici-finance/assay/tools/desk/internal/verifyflow"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

const (
	flowFixtureDir = "../../../../statusgen/testdata/flowqueue"
	flowGoldenFile = flowFixtureDir + "/gate-scores.golden.json"

	flowRoster = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_TRUSTED_BOT_SLUGS=verifier=assay-verifier-app:300000005
ASSAY_ALLOWED_REPOS=example-org/tracker:ci:private,example-org/agents:ci:private,example-org/examples:no-ci:private,example-org/console:ci:private
`
)

func flowGolden(t *testing.T) []baseRow {
	t.Helper()
	b, err := os.ReadFile(flowGoldenFile)
	if err != nil {
		t.Fatalf("shared queue fixture: %v", err)
	}
	var rows []baseRow
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatalf("shared queue fixture is not JSON: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("shared queue fixture is empty")
	}
	return rows
}

// serveBase points the canonical-queue seam at per-root rows (keyed by absolute root path); a
// root with no entry fails the way an unrunnable statusgen does.
func serveBase(t *testing.T, byRoot map[string][]baseRow) {
	t.Helper()
	old := baseQueueRows
	baseQueueRows = func(root string) ([]baseRow, error) {
		abs, _ := filepath.Abs(root)
		rows, ok := byRoot[abs]
		if !ok {
			return nil, os.ErrNotExist
		}
		return rows, nil
	}
	t.Cleanup(func() { baseQueueRows = old })
}

func flowRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(flowFixtureDir)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func planJSON(t *testing.T, args ...string) (planDoc, string) {
	t.Helper()
	var perr error
	out := captureStdout(t, func() { perr = cmdPlan(append(args, "--json")) })
	var doc planDoc
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("--json stdout is not exactly one JSON document (err=%v, plan err=%v):\n%s", err, perr, out)
	}
	return doc, out
}

func idsOf(doc planDoc, kind string) []string {
	var ids []string
	for _, it := range doc.Items {
		if it.Kind == kind {
			ids = append(ids, it.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

// regression: F-queue-contract
func TestFlowQueue(t *testing.T) {
	golden := flowGolden(t)
	root := flowRoot(t)
	stubPreflight(t, func(string) error { return nil })
	t.Setenv(deskkit.RootsEnv, "")

	// A brief the canonical queue calls judgment-dispatch is the only work the planner hands
	// a model judge, and every other class (runner, continuation, human, paused, parked,
	// unreadable) is held. deskboard's verify depth counts the same set.
	t.Run("one fixture same classes", func(t *testing.T) {
		serveBase(t, map[string][]baseRow{root: golden})
		doc, _ := planJSON(t, "--root", root, "--base-queue")

		wantDispatch := []string{}
		byBrief := map[string]baseRow{}
		for _, r := range golden {
			byBrief[r.Brief] = r
			if r.QueueValid && r.QueueClass == "judgment-dispatch" && r.Status == "implemented" {
				wantDispatch = append(wantDispatch, r.Brief)
			}
		}
		sort.Strings(wantDispatch)
		if len(wantDispatch) == 0 {
			t.Fatal("fixture has no judgment-dispatch row; the test is void")
		}
		if got := idsOf(doc, kindDispatch); strings.Join(got, ",") != strings.Join(wantDispatch, ",") {
			t.Fatalf("dispatched %v, want exactly the judgment-dispatch rows %v", got, wantDispatch)
		}
		for _, it := range doc.Items {
			row, ok := byBrief[it.ID]
			if !ok {
				t.Errorf("%s is in the plan but not in the canonical rows", it.ID)
				continue
			}
			if row.QueueValid && (it.Base == nil || it.Base.Class != row.QueueClass) {
				t.Errorf("%s base = %+v, want class %s", it.ID, it.Base, row.QueueClass)
			}
			if !row.QueueValid && it.Disposition != dispCouldNotCheck.String() {
				t.Errorf("%s is unreadable canonically but planned as %s, want could-not-check", it.ID, it.Disposition)
			}
			if it.Kind == kindDispatch && it.Authority != authFlip {
				t.Errorf("%s dispatch authority = %s", it.ID, it.Authority)
			}
		}
		if len(doc.Items) != len(golden) || !doc.Complete {
			t.Fatalf("items = %d (want %d), complete = %v, gaps = %v", len(doc.Items), len(golden), doc.Complete, doc.Gaps)
		}
	})

	// The base can only demote. Served a lie (every row judgment-dispatch), the human-gated
	// and risk-flagged briefs are still not dispatched: the overlays hold them whatever the
	// canonical queue says.
	t.Run("base never promotes", func(t *testing.T) {
		lie := make([]baseRow, len(golden))
		for i, r := range golden {
			r.QueueClass, r.QueueValid, r.QueueVersion = "judgment-dispatch", true, queueContractVersion
			lie[i] = r
		}
		serveBase(t, map[string][]baseRow{root: lie})
		doc, _ := planJSON(t, "--root", root, "--base-queue")
		for _, it := range doc.Items {
			if (it.ID == "flowq/01" || it.ID == "flowq/10") && (it.Kind == kindDispatch || it.Authority == authFlip) {
				t.Errorf("%s (human gate / irreversible risk) became model dispatch with a lying base", it.ID)
			}
		}
	})

	// The precedence table, asserted on the composer itself.
	t.Run("base composes under overlays", func(t *testing.T) {
		it := loopengine.Item{ID: "flowq/01"}
		look := func(class string) *baseLookup {
			return &baseLookup{state: "canonical", rows: map[string]baseRow{
				"flowq/01": {Brief: "flowq/01", QueueVersion: queueContractVersion, QueueClass: class, QueueValid: true},
			}}
		}
		// An overlay hold is never overridden or demoted by the base.
		for _, d := range []disposition{dispStuckFlip, dispAwaitingHuman} {
			got, _, pb := applyBase(it, d, "overlay", look("human-action"))
			if got != d || pb == nil {
				t.Errorf("overlay %v under a human-action base = %v (base %v), want it kept with the base recorded", d, got, pb)
			}
		}
		// A plain dispatch survives only a judgment-dispatch base.
		if got, _, _ := applyBase(it, dispDispatch, "r", look("judgment-dispatch")); got != dispDispatch {
			t.Errorf("dispatch under a judgment base = %v, want dispatch", got)
		}
		if got, _, _ := applyBase(it, dispDispatch, "r", look("human-action")); got != dispBaseHeld {
			t.Errorf("dispatch under a human-action base = %v, want queue-held", got)
		}
		// Evidence-only dispatch may stay under human action, and no further.
		if got, _, _ := applyBase(it, dispDispatchForEvidence, "r", look("human-action")); got != dispDispatchForEvidence {
			t.Errorf("evidence dispatch under a human-action base = %v, want it kept", got)
		}
		if got, _, _ := applyBase(it, dispDispatchForEvidence, "r", look("continuation")); got != dispBaseHeld {
			t.Errorf("evidence dispatch under a continuation base = %v, want queue-held", got)
		}
	})

	// Text and JSON are two renderings of ONE plan value.
	t.Run("text and json are one plan", func(t *testing.T) {
		serveBase(t, map[string][]baseRow{root: golden})
		doc, out := planJSON(t, "--root", root, "--base-queue")
		if strings.Contains(out, "verify-desk plan:") || strings.Contains(out, "=== DISPATCH") {
			t.Fatalf("--json stdout carries text-mode lines:\n%s", out)
		}
		var perr error
		text := captureStdout(t, func() { perr = cmdPlan([]string{"--root", root, "--base-queue"}) })
		if perr != nil {
			t.Fatalf("cmdPlan: %v", perr)
		}
		for _, id := range idsOf(doc, kindDispatch) {
			if !strings.Contains(text, "=== DISPATCH "+id+" ") {
				t.Errorf("text has no DISPATCH for %s that the JSON lists", id)
			}
		}
		if n := strings.Count(text, "=== DISPATCH "); n != doc.Counts.Dispatchable {
			t.Errorf("text dispatches %d, JSON counts %d", n, doc.Counts.Dispatchable)
		}
		for _, it := range doc.Items {
			if it.Kind == kindBucketed && !strings.Contains(text, it.ID) {
				t.Errorf("text does not list bucketed %s", it.ID)
			}
		}
		// The same value rendered both ways, in one process.
		built, err := buildPlan(&VerifyLoop{Root: root, RootRepo: ""}, planOpts{root: root, baseQueue: true})
		if err != nil {
			t.Fatal(err)
		}
		var viaText bytes.Buffer
		viaText.WriteString(captureStdout(t, func() { renderPlanText(built) }))
		viaJSON := captureStdout(t, func() { _ = renderPlanJSON(built) })
		var back planDoc
		if err := json.Unmarshal([]byte(viaJSON), &back); err != nil {
			t.Fatal(err)
		}
		if strings.Count(viaText.String(), "=== DISPATCH ") != len(idsOf(back, kindDispatch)) {
			t.Error("one plan value renders different dispatch sets as text and as JSON")
		}
	})

	// Mixed roots: healthy and canonical; healthy but its statusgen predates the contract;
	// preflight red; unreadable. Each is named, none is a zero, and the exit is 6.
	t.Run("mixed roots keep partial coverage", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		plantFlowRoster(t)
		old := t.TempDir()
		writeFixtureStream(t, old, "stream-old", briefTable("01"), map[string]string{"01": briefBody("no", "model", "")})
		red := t.TempDir()
		writeFixtureStream(t, red, "stream-red", briefTable("01"), map[string]string{"01": briefBody("no", "model", "")})
		gone := filepath.Join(t.TempDir(), "absent")
		t.Setenv(deskkit.RootsEnv, "example-org/tracker="+root+",example-org/agents="+old+
			",example-org/examples="+red+",example-org/console="+gone)
		stubPreflight(t, func(r string) error {
			if r == red {
				return deskkit.Unverifiable("preflight: could-not-check", nil)
			}
			return nil
		})
		oldAbs, _ := filepath.Abs(old)
		serveBase(t, map[string][]baseRow{
			root:   golden,
			oldAbs: {{Brief: "stream-old/01", Status: "implemented"}}, // no queueVersion
		})

		var err error
		out := captureStdout(t, func() { err = cmdPlan([]string{"--base-queue", "--json"}) })
		var doc planDoc
		if jerr := json.Unmarshal([]byte(out), &doc); jerr != nil {
			t.Fatalf("not JSON: %v\n%s", jerr, out)
		}
		if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
			t.Fatalf("a skipped/unreadable root must make the plan exit 6, got %v", err)
		}
		states := map[string]planRoot{}
		for _, r := range doc.Roots {
			states[r.Repo] = r
		}
		if states["example-org/tracker"].State != rootHealthy || states["example-org/tracker"].Queue != baseCanonical ||
			states["example-org/agents"].State != rootHealthy || states["example-org/agents"].Queue != baseUnsupported ||
			states["example-org/examples"].State != rootSkipped ||
			states["example-org/console"].State != rootUnreadable {
			t.Fatalf("root coverage = %+v", doc.Roots)
		}
		if doc.Complete || doc.Skipped != 2 || len(doc.Gaps) < 3 {
			t.Fatalf("complete=%v skipped=%d gaps=%v: the gaps must be named, not rounded to clean", doc.Complete, doc.Skipped, doc.Gaps)
		}
		// The healthy canonical root still plans in full ...
		want := 0
		for _, r := range golden {
			if r.QueueValid && r.QueueClass == "judgment-dispatch" {
				want++
			}
		}
		if got := len(idsOf(doc, kindDispatch)); got != want {
			t.Fatalf("dispatched %d, want the canonical root's %d", got, want)
		}
		// ... and the root whose statusgen predates the contract dispatches nothing: a brief
		// the canonical queue could not classify is could-not-check, never widened to work.
		for _, it := range doc.Items {
			if strings.HasSuffix(it.ID, ":stream-old/01") && it.Kind != kindBucketed {
				t.Errorf("%s was planned as %s without a canonical classification", it.ID, it.Kind)
			}
			if strings.HasSuffix(it.ID, ":stream-old/01") && it.Disposition != dispCouldNotCheck.String() {
				t.Errorf("%s disposition = %s, want could-not-check", it.ID, it.Disposition)
			}
		}
	})

	// A healthy root whose canonical queue is partial or unsupported makes the plan
	// incomplete (Complete=false plus a named gap), and an incomplete read never exits 0:
	// the repo's incomplete-read code (6, unverifiable) is what a scripted caller keys on.
	t.Run("an incomplete canonical read exits 6", func(t *testing.T) {
		old := make([]baseRow, len(golden))
		copy(old, golden)
		for i := range old {
			old[i].QueueVersion = 0 // a statusgen that predates the contract
		}
		serveBase(t, map[string][]baseRow{root: old})
		var err error
		out := captureStdout(t, func() { err = cmdPlan([]string{"--root", root, "--base-queue", "--json"}) })
		var doc planDoc
		if jerr := json.Unmarshal([]byte(out), &doc); jerr != nil {
			t.Fatalf("not JSON: %v\n%s", jerr, out)
		}
		if doc.Complete || len(doc.Gaps) == 0 {
			t.Fatalf("complete=%v gaps=%v: an unsupported canonical queue must be a named gap", doc.Complete, doc.Gaps)
		}
		if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
			t.Fatalf("complete=false must not exit 0; want exit 6, got %v", err)
		}

		// Control: a fully canonical reading is complete and exits 0.
		serveBase(t, map[string][]baseRow{root: golden})
		out = captureStdout(t, func() { err = cmdPlan([]string{"--root", root, "--base-queue", "--json"}) })
		if err != nil {
			t.Fatalf("a complete canonical plan must exit 0, got %v\n%s", err, out)
		}
	})

	// With the planner BYPASSED the lower landing guard still refuses to turn a risk-flagged
	// brief into model acceptance: an irreversible PASS is written as Evidence with no flip
	// and a human checkpoint, and a ROUTE_HUMAN verdict is routed, never flipped.
	t.Run("landing guard holds with planner bypassed", func(t *testing.T) {
		rec := newRec()
		v := &VerifyLoop{DurableSink: rec, Now: fixedNow}
		it := loopengine.Item{ID: "flowq/10", Risk: loopengine.RiskFlags{Irreversible: true}}
		admitFixture(v, it)
		if err := v.Land(loopengine.Result{Item: it, Verdict: loopengine.VerdictPass, RunnerID: "local:judge",
			Rows: []loopengine.EvidenceRow{{Command: "go test", Exit: 0, Output: "ok"}}}); err != nil {
			t.Fatalf("Land: %v", err)
		}
		if rec.flipped["flowq/10"] {
			t.Fatal("a risk-flagged brief was flipped by a model verdict")
		}
		if len(rec.checkpoint) != 1 {
			t.Fatalf("no human checkpoint opened: %v", rec.checkpoint)
		}
		human := loopengine.Item{ID: "flowq/01"}
		if err := v.Land(loopengine.Result{Item: human, Verdict: loopengine.VerdictRouteHuman}); err != nil {
			t.Fatal(err)
		}
		if rec.flipped["flowq/01"] || len(rec.routed) != 1 {
			t.Fatalf("a route-human verdict must route and never flip: routed=%v flipped=%v", rec.routed, rec.flipped)
		}
		// And a verdict with no dispatcher attestation is refused outright.
		un := loopengine.Item{ID: "flowq/06"}
		if err := (&VerifyLoop{DurableSink: rec, Now: fixedNow}).Land(loopengine.Result{Item: un, Verdict: loopengine.VerdictPass}); err == nil {
			t.Fatal("an unattested verdict landed")
		}
	})
}

func briefTable(nums ...string) string {
	var b strings.Builder
	b.WriteString("| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n")
	b.WriteString("|---|-------|------|--------|--------|----------|----------|\n")
	for _, n := range nums {
		b.WriteString("| " + n + " | b | 0 | S | implemented | — | — |\n")
	}
	return b.String()
}

func plantFlowRoster(t *testing.T) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(flowRoster), 0o600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
}

// regression: F-verify-candidate-parity
func TestVerifyPlanParity(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	golden := flowGolden(t)
	root := flowRoot(t)
	serveBase(t, map[string][]baseRow{root: golden})
	v := &VerifyLoop{Root: root, RootRepo: "example-org/first", TargetSHA: strings.Repeat("a", 40), Now: func() time.Time { return now }}
	doc, err := buildPlan(v, planOpts{root: root, baseQueue: true})
	if err != nil {
		t.Fatal(err)
	}
	if doc.CandidateObservation == nil {
		t.Fatal("production plan omitted the candidate observation")
	}
	original := *doc.CandidateObservation
	if original.WholeScope {
		t.Fatal("one explicit root established whole-desk coverage")
	}
	if len(original.Candidates) != len(golden) {
		t.Fatal("observation lost held candidates")
	}
	// The adapter supplies actual queue rows and observed overlays, not a second
	// classifier; source and repair coverage remain independently visible.
	for _, c := range original.Candidates {
		if c.Subject.Revision == "" || c.Base.Brief != c.Subject.Brief {
			t.Fatalf("lost exact source or base: %+v", c)
		}
	}
	policy := vf.Policy{Owner: "verify-desk", Revision: "approved-policy-1", Evaluator: vf.Evaluator, Unmapped: "observed-score", Priorities: []vf.Priority{}}
	plan, err := vf.PlanCandidates(original, policy, now, now.Add(-time.Minute), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Gaps) == 0 || plan.Idle == "yes" {
		t.Fatal("missing repair/all-root coverage looked idle")
	}
	// Independent base oracle: these are the only judgment rows in the shared
	// source fixture, irrespective of its derived golden's class labels.
	expected := map[string]bool{"flowq/06": true, "flowq/07": true}
	for _, d := range plan.Candidates {
		if (d.Disposition == "selected") != expected[d.Candidate.Subject.Brief] {
			t.Errorf("oracle mismatch %s: %s", d.Candidate.Subject.Brief, d.Disposition)
		}
	}
	// The same immutable value reruns deterministically after a slot completion;
	// there is no reader/process/provider in the shared pure function.
	again, err := vf.PlanCandidates(original, policy, now, now.Add(-time.Minute), 100)
	if err != nil || !reflect.DeepEqual(plan, again) {
		t.Fatal("identical inputs changed the plan", err)
	}
	// Second root work remains visible even when the first root is empty. Explicit
	// repair coverage is needed to establish whole-scope idle, even with no rows.
	empty := vf.Observation{Schema: vf.Schema, Evaluator: vf.Evaluator, Revision: "obs-1", Provenance: "fixture", Collected: now, WholeScope: true,
		Requested: []string{"example-org/first", "example-org/second"}, Candidates: []vf.Candidate{}}
	for _, repo := range empty.Requested {
		for _, lane := range []string{vf.Awaiting, vf.Repair} {
			empty.Coverage = append(empty.Coverage, vf.Coverage{Repository: repo, Lane: lane, State: "complete"})
		}
	}
	idle, err := vf.PlanCandidates(empty, policy, now, now.Add(-time.Minute), 0)
	if err != nil || idle.Idle != "yes" {
		t.Fatal("complete empty observation not idle", err, idle)
	}
	missing := empty
	missing.Coverage = append([]vf.Coverage{}, empty.Coverage[:3]...)
	unknown, _ := vf.PlanCandidates(missing, policy, now, now.Add(-time.Minute), 2)
	if unknown.Idle != "unknown" {
		t.Fatal("missing second repair lane became empty")
	}
	one := original.Candidates[0]
	for _, c := range original.Candidates {
		if c.Subject.Brief == "flowq/06" {
			one = c
		}
	}
	one.Subject.Repository = "example-org/second"
	one.Base.Repo = ""
	second := empty
	second.Candidates = []vf.Candidate{one}
	result, err := vf.PlanCandidates(second, policy, now, now.Add(-time.Minute), 1)
	if err != nil || len(result.Candidates) != 1 || result.Candidates[0].Disposition != "selected" || result.Idle != "no" {
		t.Fatal("second root work vanished", err, result)
	}
	// Capacity is selection, never an empty-candidate claim.
	zero, _ := vf.PlanCandidates(second, policy, now, now.Add(-time.Minute), 0)
	if zero.Candidates[0].Disposition != "deferred" || zero.Idle != "no" {
		t.Fatal("full pool became idle")
	}
	stale, _ := vf.PlanCandidates(second, policy, now.Add(time.Hour), now.Add(time.Minute), 1)
	if stale.Candidates[0].Disposition != vf.Unknown {
		t.Fatal("stale observation selected")
	}
	bad := one
	bad.Base.QueueClass = "future-class"
	second.Candidates = []vf.Candidate{bad}
	unsupported, _ := vf.PlanCandidates(second, policy, now, now.Add(-time.Minute), 1)
	if unsupported.Candidates[0].Disposition != vf.Unknown {
		t.Fatal("unsupported contract was usable")
	}
	// High priority cannot erase an overlay hold. An unobserved priority entry
	// remains a gap rather than a phantom work item or a silently dropped join.
	held := one
	held.Subject.Brief = "flowq/hold"
	held.Base.Brief = held.Subject.Brief
	held.Overlay = "awaiting-human"
	second.Candidates = []vf.Candidate{one, held}
	priority := policy
	priority.Priorities = []vf.Priority{{Repository: one.Subject.Repository, Brief: held.Subject.Brief, Score: 999},
		{Repository: one.Subject.Repository, Brief: "absent/01", Score: 1000}}
	ranked, _ := vf.PlanCandidates(second, priority, now, now.Add(-time.Minute), 1)
	if ranked.Candidates[0].Candidate.Subject.Brief != held.Subject.Brief || ranked.Candidates[0].Disposition != "held" ||
		ranked.Candidates[1].Disposition != "selected" || len(ranked.NotObserved) != 1 {
		t.Fatalf("ranking erased hold or coverage: %+v", ranked)
	}
	// Same brief alias in another root remains a separate qualified subject.
	first := one
	first.Subject.Repository = "example-org/first"
	first.Base.Score = 100
	second.Candidates = []vf.Candidate{one, first}
	both, _ := vf.PlanCandidates(second, policy, now, now.Add(-time.Minute), 2)
	if len(both.Candidates) != 2 || both.Candidates[0].Disposition != "selected" || both.Candidates[1].Disposition != "selected" {
		t.Fatal("cross-root identity collapsed")
	}
	// A repair-only obligation remains visible and cannot become judge work.
	repair := held
	repair.Sources = []string{vf.Repair}
	repair.Overlay = "in-repair"
	second.Candidates = []vf.Candidate{repair}
	debt, _ := vf.PlanCandidates(second, policy, now, now.Add(-time.Minute), 2)
	if debt.Idle != "no" || debt.Candidates[0].Disposition != "held" {
		t.Fatal("repair debt disappeared")
	}
	duplicate := one
	duplicate.Aliases = []string{"alias/06"}
	duplicate.Sources = []string{vf.Repair}
	second.Candidates = []vf.Candidate{one, duplicate}
	joined, err := vf.PlanCandidates(second, policy, now, now.Add(-time.Minute), 2)
	if err != nil || len(joined.Candidates) != 1 || len(joined.Candidates[0].Candidate.Sources) != 2 {
		t.Fatal("lane union lost or duplicated a candidate", err)
	}
	duplicate.Subject.Revision = "different-head"
	second.Candidates = []vf.Candidate{one, duplicate}
	if _, err := vf.PlanCandidates(second, policy, now, now.Add(-time.Minute), 2); err == nil {
		t.Fatal("conflicting heads silently reconciled")
	}
	watched := one
	watched.Base.QueueClass = "paused"
	second.Candidates = []vf.Candidate{watched}
	watch, _ := vf.PlanCandidates(second, policy, now, now.Add(-time.Minute), 2)
	if watch.Candidates[0].Disposition != "watch" || watch.Idle != "yes" {
		t.Fatal("paused placement became owed work", watch)
	}
	missingPolicy := policy
	missingPolicy.Priorities = nil
	if _, err := vf.PlanCandidates(empty, missingPolicy, now, now.Add(-time.Minute), 1); err == nil {
		t.Fatal("missing policy became empty")
	}
}
