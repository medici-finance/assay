//go:build darwin || linux

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellop"
)

var applyLineRE = regexp.MustCompile(`(?m)^apply on a terminal: cellctl up example --apply (\S+)$`)

// applyUp applies plan id on a pseudo-terminal, typing code, and checks the stale role was
// reconciled, the result names the plan, and the plan is consumed under its own id.
func applyUp(t *testing.T, w *upWorld, id, code string) {
	t.Helper()
	exit, out := w.onTTY(t, code, "up", "example", "--cockpit", "tmux", "--apply", id, "--json", "--lease-wait", "20s")
	res := upDocFromTTY(t, out)
	if exit != 0 || res.Outcome != cellop.OK || roleState(res, "worker-desk") != "reconciled+up" {
		t.Fatalf("apply %s: exit %d, outcome %q, rows %+v\n%s", id, exit, res.Outcome, res.Roles, out)
	}
	if res.PlanID == nil || *res.PlanID != id {
		t.Fatalf("result plan_id %v, want %q", res.PlanID, id)
	}
	if _, err := os.Lstat(filepath.Join(w.cellDir, "run", "plans", "consumed", id)); err != nil {
		t.Fatalf("plan %s not consumed under its id: %v", id, err)
	}
}

// `up --plan` names its plan with two words, prints the apply line with that id, stores the plan
// under it, and the plan applies by it with the typed confirm.
// regression: #6266
func TestUpPlanPrintsTwoWordID(t *testing.T) {
	w := newUpWorld(t, "worker-desk", 50*time.Millisecond)
	writeStale(t, w.cell(), "worker-desk", deadPID(t), "", 0, "")
	r := w.run(t, nil, "up", "example", "--plan")
	m := applyLineRE.FindStringSubmatch(r.Stdout)
	if r.Code != 0 || m == nil {
		t.Fatalf("up --plan: exit %d, no apply line\n%s%s", r.Code, r.Stdout, r.Stderr)
	}
	id := m[1]
	if !regexp.MustCompile(`^[a-z]{3,8}-[a-z]{3,8}$`).MatchString(id) {
		t.Fatalf("printed plan id %q is not two words joined by a hyphen", id)
	}
	if !strings.Contains(r.Stdout, "plan "+id) {
		t.Fatalf("the rendering does not name plan %s:\n%s", id, r.Stdout)
	}
	p, err := cellop.StoreFor(w.cellDir).Load(id)
	if err != nil {
		t.Fatalf("the printed id does not load: %v", err)
	}
	applyUp(t, w, id, p.ConfirmCode)
}

// A plan stored under a 24-hex id, as cellctl wrote them before two-word ids, still applies by
// that id on a terminal with the typed confirm.
// regression: #6266
func TestUpApplyLegacyHexID(t *testing.T) {
	w := newUpWorld(t, "worker-desk", 50*time.Millisecond)
	writeStale(t, w.cell(), "worker-desk", deadPID(t), "", 0, "")
	r := w.run(t, nil, "up", "example", "--plan", "--json")
	p, err := cellop.DecodePlan([]byte(r.Stdout))
	if r.Code != 0 || err != nil {
		t.Fatalf("up --plan --json: exit %d, %v\n%s%s", r.Code, err, r.Stdout, r.Stderr)
	}
	legacy, err := cellop.NewPlan("4f1c2a9e7b3d5c8e0a6f2b1d", p.Verb, p.Cells, p.Actions, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cellop.StoreFor(w.cellDir).Save(legacy); err != nil {
		t.Fatal(err)
	}
	applyUp(t, w, legacy.ID, legacy.ConfirmCode)
}
