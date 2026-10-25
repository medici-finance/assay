package main

// flow_50_test.go — the verifyloop half of the run-bound prerequisite record (desk-tools/50).
//
// A root's envelope preflight admits the root's record; every item queued from it is admitted on
// top of it at selection (siblings its brief declares, pinned instruments its rows call). An item
// whose prerequisites cannot be read is held as could-not-check with the exact reason, a second
// root whose prerequisites cannot be read is skipped with the exact reason while the healthy
// root keeps its coverage, and a run whose world moved after admission is refused at dispatch
// and at landing — no prompt emitted, no Evidence written, no bug filed.

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

type world50Probes struct {
	revs map[string]string
}

func (w *world50Probes) probes() deskkit.PreflightProbes {
	return deskkit.PreflightProbes{
		Revision: func(dir string) (string, error) {
			if r, ok := w.revs[dir]; ok {
				return r, nil
			}
			return "", errors.New("not a git checkout")
		},
		Instrument:   func(string) (string, string, error) { return "", "", errors.New("not on PATH") },
		RunningTool:  func() string { return "desk-tools/v9.9.9" },
		Runtime:      func() string { return "linux-amd64" },
		DirExists:    func(d string) (bool, error) { _, ok := w.revs[d]; return ok, nil },
		SiblingRoots: func() ([]deskkit.RootConfig, error) { return nil, nil },
	}
}

const (
	world50RevA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	world50RevS = "cccccccccccccccccccccccccccccccccccccccc"
)

// world50Preflight stands in for the real per-root envelope: the six boot checks are stubbed
// green, and the seventh is the real prerequisite admission over the fake local world.
func world50Preflight(t *testing.T, w *world50Probes) {
	t.Helper()
	stubPreflight(t, func(root string) error {
		adm := deskkit.AdmitWorld(deskkit.WorldSpec{Root: root,
			Capability: deskkit.Check{Name: deskkit.CheckAppScopes, State: deskkit.CheckedClean}}, w.probes())
		if !adm.Ready() {
			recordRootWorld(root, nil)
			return adm.Err()
		}
		recordRootWorld(root, &adm.World)
		return nil
	})
}

func world50Gate(w *world50Probes) *worldGate {
	return &worldGate{root: admittedRootWorld, probes: w.probes()}
}

// regression: desk-tools/50 — the loop dispatched and landed runs whose environment was never
// recorded, so a moved sibling or an unreadable second root read as the code's result.
func TestFlowWorld(t *testing.T) {
	t.Run("second root with unreadable prerequisites is skipped with the exact reason", func(t *testing.T) {
		plantFixtureRoster(t)
		a, b := twoRoots(t)
		w := &world50Probes{revs: map[string]string{absRoot(a): world50RevA}} // b is not a checkout
		t.Setenv(deskkit.RootsEnv, "example-org/tracker="+a+",example-org/agents="+b)
		world50Preflight(t, w)
		var err error
		out := captureStdout(t, func() { err = cmdPlanWorld(nil, verifyPerformance{}, world50Gate(w)) })
		if !strings.Contains(out, "=== DISPATCH example-org/tracker:stream-a/01 (tier=local) root="+a+" ===") {
			t.Fatalf("healthy root lost its coverage (err=%v):\n%s", err, out)
		}
		if strings.Contains(out, "DISPATCH example-org/agents") ||
			!strings.Contains(out, "PREFLIGHT RED — skipped") ||
			!strings.Contains(out, "environment could-not-check: source: cannot read the root revision") {
			t.Fatalf("unreadable root not skipped with its exact reason:\n%s", out)
		}
	})

	t.Run("item admission holds a missing sibling and binds the healthy item", func(t *testing.T) {
		v, _, items := world50Selected(t)
		held, ok := world50Item(items, "fixture/02")
		if !ok || !strings.HasPrefix(payloadValue(held, "could_not_check"), "environment could-not-check: sibling:../absent") {
			t.Fatalf("missing sibling not held with its reason: %+v", held.Payload)
		}
		if _, err := v.Dispatch(held, loopengine.TierLocal); err == nil || !strings.Contains(err.Error(), "could-not-check") {
			t.Fatalf("held item dispatched: %v", err)
		}
		healthy, _ := world50Item(items, "fixture/01")
		if why := payloadValue(healthy, "could_not_check"); why != "" {
			t.Fatalf("healthy item held: %s", why)
		}
		rec, ok := v.World.item("fixture/01")
		if !ok || rec.Inputs["sibling:../tracker"] != world50RevS || rec.Inputs[deskkit.WorldCapability] == "" {
			t.Fatalf("healthy item record incomplete: %+v", rec)
		}
	})

	t.Run("dispatch refuses a world that moved after admission", func(t *testing.T) {
		v, w, items := world50Selected(t)
		healthy, _ := world50Item(items, "fixture/01")
		w.revs[filepath.Join(filepath.Dir(v.Root), "tracker")] = "dddddddddddddddddddddddddddddddddddddddd"
		v.World.probes = w.probes()
		_, err := v.Dispatch(healthy, loopengine.TierLocal)
		if err == nil || !strings.Contains(err.Error(), "environment changed since admission: sibling:../tracker") {
			t.Fatalf("moved sibling dispatched: %v", err)
		}
		if strings.Contains(v.Emit.(*bytes.Buffer).String(), "=== DISPATCH") {
			t.Fatal("a prompt was emitted for a refused world")
		}
		if _, ok := v.takeRun(healthy); ok {
			t.Fatal("a refused world was remembered as an admitted run")
		}
	})

	t.Run("landing consumes the dispatched record and refuses a moved source", func(t *testing.T) {
		v, w, items := world50Selected(t)
		healthy, _ := world50Item(items, "fixture/01")
		h, err := v.Dispatch(healthy, loopengine.TierLocal)
		if err != nil {
			t.Fatalf("healthy dispatch: %v", err)
		}
		r := <-h.Done()
		w.revs[absRoot(v.Root)] = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
		v.World.probes = w.probes()
		rec := v.DurableSink.(*recordingDurable)
		err = v.Land(r)
		if err == nil || !strings.Contains(err.Error(), "environment changed since admission: source") {
			t.Fatalf("moved source landed: %v", err)
		}
		if len(rec.evidence) != 0 || len(rec.bugs) != 0 {
			t.Fatalf("a refused world wrote Evidence %v or filed %v", rec.evidence, rec.bugs)
		}
	})

	t.Run("an unchanged world lands", func(t *testing.T) {
		v, _, items := world50Selected(t)
		healthy, _ := world50Item(items, "fixture/01")
		h, err := v.Dispatch(healthy, loopengine.TierLocal)
		if err != nil {
			t.Fatalf("healthy dispatch: %v", err)
		}
		if err := v.Land(<-h.Done()); err != nil {
			t.Fatalf("unchanged world refused at landing: %v", err)
		}
		if !v.DurableSink.(*recordingDurable).flipped["fixture/01"] {
			t.Fatal("unchanged world did not land its PASS")
		}
	})

	t.Run("a gated loop refuses a result with no record", func(t *testing.T) {
		v, _, _ := world50Selected(t)
		it := loopengine.Item{ID: "fixture/09"}
		admitFixture(v, it)
		if err := v.Land(loopengine.Result{Item: it, Verdict: loopengine.VerdictPass}); err == nil ||
			!strings.Contains(err.Error(), "no admitted prerequisite record") {
			t.Fatalf("unrecorded run landed: %v", err)
		}
	})
}

// world50Selected builds one root with two briefs — 01 declares a materialized sibling, 02 one
// that is absent — admits the root as the boot would, and runs the gated selection.
func world50Selected(t *testing.T) (*VerifyLoop, *world50Probes, []loopengine.Item) {
	t.Helper()
	parent := t.TempDir()
	root := filepath.Join(parent, "home")
	table := "| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | A | 0 | S | implemented | — | — |\n" +
		"| 02 | B | 0 | S | implemented | — | — |\n"
	sibling := func(rel string) string {
		return strings.Replace(briefBody("no", "model", ""), "# Brief\n", "# Brief\n\nout-of-repo files: "+rel+"/docs/x.md\n", 1)
	}
	writeFixtureStream(t, root, "fixture", table, map[string]string{"01": sibling("../tracker"), "02": sibling("../absent")})
	w := &world50Probes{revs: map[string]string{absRoot(root): world50RevA, filepath.Join(parent, "tracker"): world50RevS}}
	world50Preflight(t, w)
	if err := preflightRoot(root); err != nil {
		t.Fatalf("root not admitted: %v", err)
	}
	var out bytes.Buffer
	v := &VerifyLoop{Root: root, TargetSHA: "abc123", Emit: &out, DurableSink: newRec(), Now: fixedNow,
		World: world50Gate(w), Attest: fixtureAttestation,
		Feeder: func(it loopengine.Item, _ loopengine.Tier, _ string) (loopengine.Result, error) {
			return loopengine.Result{Item: it, Verdict: loopengine.VerdictPass, RunnerID: "local:glm",
				Rows: []loopengine.EvidenceRow{{Command: "true", Exit: 0, Output: "ok"}}}, nil
		}}
	items, err := v.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	return v, w, items
}

func world50Item(items []loopengine.Item, id string) (loopengine.Item, bool) {
	for _, it := range items {
		if it.ID == id {
			return it, true
		}
	}
	return loopengine.Item{}, false
}
