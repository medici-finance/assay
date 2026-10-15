//go:build cellopfixture && (darwin || linux)

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/cellop"
)

// Row 10 of desk-containers/15: the fixture verb's plan and apply, end to end, with the built
// binary on a pseudo-terminal.

// fixtureResult decodes the last cell-op-result line of a run.
func fixtureResult(t *testing.T, r cellopRun) cellop.Result {
	t.Helper()
	line := lastJSON(r.stdout)
	if line == "" {
		t.Fatalf("no cell-op-v1 result on stdout (exit %d)\n%s", r.code, r.out)
	}
	var res cellop.Result
	if err := json.Unmarshal([]byte(line), &res); err != nil {
		t.Fatalf("decoding the result: %v\n%s", err, line)
	}
	return res
}

func reasonOf(r cellop.Result) string {
	if r.Reason == nil {
		return ""
	}
	return *r.Reason
}

// A plan applied twice, applied stale or applied with a wrong code ran its action (desk-containers/15).
// regression: #2890
func TestPlanApplyRoundTrip(t *testing.T) {
	w := newCellopWorld(t)
	env := w.env("DRY_RUN=1")
	plan := func() cellop.Plan {
		t.Helper()
		r := runCellctl(t, w, ttyNone, env, nil, "cellop-fixture", "example", "--plan", "--json")
		if r.code != 0 {
			t.Fatalf("--plan --json off a terminal: exit %d\n%s", r.code, r.out)
		}
		p, err := cellop.DecodePlan([]byte(r.stdout))
		if err != nil {
			t.Fatalf("--plan --json printed no valid plan: %v\n%s", err, r.stdout)
		}
		return p
	}
	typeCode := func(p cellop.Plan) func(string) string {
		return func(string) string { return p.ConfirmCode }
	}

	p := plan()
	if p.Verb != "cellop-fixture" || len(p.Actions) != 1 || p.Actions[0].Preconditions[0].Value != "absent" {
		t.Fatalf("unexpected plan: %+v", p)
	}
	if len(w.effects(t)) != 0 {
		t.Fatal("--plan ran an effect")
	}

	r := runCellctl(t, w, ttyBoth, env, typeCode(p), "cellop-fixture", "example", "--apply", p.ID, "--json")
	res := fixtureResult(t, r)
	if r.code != 0 || res.Outcome != cellop.OK || res.PlanID == nil || *res.PlanID != p.ID {
		t.Fatalf("apply with the typed code: exit %d, result %+v\n%s", r.code, res, r.out)
	}
	if !strings.Contains(r.out, p.ConfirmCode) {
		t.Fatalf("the terminal was not shown the confirm code\n%s", r.out)
	}
	if got := w.effects(t); len(got) != 1 {
		t.Fatalf("effects after one apply: %v", got)
	}

	r = runCellctl(t, w, ttyBoth, env, typeCode(p), "cellop-fixture", "example", "--apply", p.ID, "--json")
	res = fixtureResult(t, r)
	if r.code != 5 || reasonOf(res) != cellop.ReasonAlreadyConsumed {
		t.Fatalf("second apply: exit %d, reason %q, want 5 %s\n%s", r.code, reasonOf(res), cellop.ReasonAlreadyConsumed, r.out)
	}
	if strings.Contains(r.out, confirmPrompt) {
		t.Fatal("a consumed plan prompted for its code again")
	}
	if got := w.effects(t); len(got) != 1 {
		t.Fatalf("the second apply ran an effect: %v", got)
	}

	// A wrong code changes nothing and leaves the plan applicable.
	q := plan()
	r = runCellctl(t, w, ttyBoth, env, func(string) string { return "00000000" }, "cellop-fixture", "example", "--apply", q.ID, "--json")
	if res = fixtureResult(t, r); r.code != 5 || reasonOf(res) != cellop.ReasonConfirmMismatch {
		t.Fatalf("wrong code: exit %d, reason %q\n%s", r.code, reasonOf(res), r.out)
	}
	if done, _ := w.store().Consumed(q.ID); done || len(w.effects(t)) != 1 {
		t.Fatalf("a wrong code consumed the plan (%v) or ran an effect (%v)", done, w.effects(t))
	}

	// The world moved after the plan was written: the re-measure refuses and consumes nothing.
	dir := filepath.Join(w.cellDir, "run", "cellop-fixture")
	if err := os.WriteFile(filepath.Join(dir, "state"), []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r = runCellctl(t, w, ttyBoth, env, typeCode(q), "cellop-fixture", "example", "--apply", q.ID, "--json")
	if res = fixtureResult(t, r); r.code != 5 || reasonOf(res) != cellop.ReasonPlanStale {
		t.Fatalf("stale plan: exit %d, reason %q\n%s", r.code, reasonOf(res), r.out)
	}
	if done, _ := w.store().Consumed(q.ID); done || len(w.effects(t)) != 1 {
		t.Fatalf("a stale plan was consumed (%v) or ran (%v)", done, w.effects(t))
	}
}
