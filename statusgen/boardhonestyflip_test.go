package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// flipFixtureStreams loads the board under root the way run() does, so the
// planner sees the same streams the reconcile writes.
func flipFixtureStreams(t *testing.T, root string) []*Stream {
	t.Helper()
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	return streams
}

func flipRow(t *testing.T, streams []*Stream, stream, num string) (*Stream, Brief) {
	t.Helper()
	for _, s := range streams {
		if s.Name != stream {
			continue
		}
		for _, b := range s.Briefs {
			if b.Num == num {
				return s, b
			}
		}
	}
	t.Fatalf("fixture drift: no row %s/%s", stream, num)
	return nil, Brief{}
}

// flipNotice returns the board-honesty NOTICE naming id.
func flipNotice(t *testing.T, root string, streams []*Stream, merged []mergedPR, id string) string {
	t.Helper()
	for _, n := range boardHonestyNotices(root, streams, merged, nil) {
		if strings.Contains(n, "): "+id+" — ") {
			return n
		}
	}
	t.Fatalf("no board-honesty NOTICE names %s", id)
	return ""
}

// addDesignRecord gives dg/05 an approved design: record, clearing §4.4.
func addDesignRecord(t *testing.T, root string) {
	t.Helper()
	bp := filepath.Join(root, "docs", "streams", "dg", "brief-05-todo.md")
	b, err := os.ReadFile(bp)
	if err != nil {
		t.Fatal(err)
	}
	with := strings.Replace(string(b), "issues: []\n", "issues: []\ndesign: DR-fixture-approved\n", 1)
	if with == string(b) {
		t.Fatal("fixture shape changed: could not add a design: line to dg/05")
	}
	if err := os.WriteFile(bp, []byte(with), 0o644); err != nil {
		t.Fatal(err)
	}
}

// assertFlipHeld pins one held row: the planner's hold label and NOTICE carry
// the PROBLEM the reconcile's own hold reports for the same row, the fix is
// never "reconcile the row", and the row stays out of dispatch.
func assertFlipHeld(t *testing.T, root, stream, num string, merged []mergedPR, want ...string) {
	t.Helper()
	id := stream + "/" + num
	streams := flipFixtureStreams(t, root)
	applyPhantomHolds(root, streams, merged, nil)
	s, b := flipRow(t, streams, stream, num)

	_, held, err := planReconcileWrites(root, []BriefCell{{ID: id, Cell: "implemented", Source: "pr", Witness: "PR #7"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(held) != 1 {
		t.Fatalf("fixture drift: the reconcile must hold %s, got held=%+v", id, held)
	}
	if !strings.Contains(b.PhantomHeld, "Brief: trailer; flip held: ") || !b.PhantomFlipHeld {
		t.Errorf("a row the reconcile refuses must be labelled flip-held; got %q (flip-held=%v)", b.PhantomHeld, b.PhantomFlipHeld)
	}
	notice := flipNotice(t, root, streams, merged, id)
	for _, p := range held[0].Problems {
		if !strings.Contains(b.PhantomHeld, p) || !strings.Contains(notice, p) {
			t.Errorf("label and NOTICE must name the reconcile hold's own PROBLEM %q;\nlabel:  %s\nnotice: %s", p, b.PhantomHeld, notice)
		}
	}
	for _, w := range want {
		if !strings.Contains(b.PhantomHeld, w) {
			t.Errorf("label must name %q: %s", w, b.PhantomHeld)
		}
	}
	if strings.Contains(notice, "reconcile the row") || !strings.Contains(notice, "`design:`") || !strings.Contains(notice, "`ruling:`") {
		t.Errorf("a flip-held row's fix must cite the record, never \"reconcile the row\": %s", notice)
	}
	if eligibleBase(streams, s, b, map[string]bool{}, nil) {
		t.Errorf("%s must stay held out of dispatch", id)
	}
	nu := nextUp(streams, ClaimView{}, nil)
	if nu.PhantomElsewhere[id] == "" || !nu.PhantomFlipHeld[id] {
		t.Errorf("Next-up must name %s as flip-held; elsewhere=%v flip=%v", id, nu.PhantomElsewhere, nu.PhantomFlipHeld)
	}
}

// regression: #4604
func TestPlannerFlipHeldParity(t *testing.T) {
	dgMerged := []mergedPR{{Number: 7, Subject: "feat: dg/05 (#7)", Briefs: []string{"dg/05"}, Delivers: []string{"dg/05"}}}

	t.Run("§4.4 risk-gated brief with no design: record", func(t *testing.T) {
		root := copyDesignGateFixture(t)
		assertFlipHeld(t, root, "dg", "05", dgMerged, "merged PR #7", "no design: record", "§4.4")
	})

	t.Run("§4.5 gate: human brief with no recorded ruling", func(t *testing.T) {
		dhSeams(t, nil)
		root := dhFixture(t, unruledBase())
		merged := []mergedPR{{Number: 7, Subject: "feat: sdlc/18 (#7)", Briefs: []string{dhBoardID}, Delivers: []string{dhBoardID}}}
		assertFlipHeld(t, root, dhStream, "18", merged, "merged PR #7", "decision-gate hold", "no recorded ruling")
	})

	// The CLASS, not the two instances: a Status-keyed lint rule the planner has
	// never heard of holds the flip, and the planner names it with no change of
	// its own — there is one evaluator, not a copy of the rules.
	t.Run("a planted hold rule is named with no planner change", func(t *testing.T) {
		prev := statusKeyedLintRules
		statusKeyedLintRules = append(append([]statusKeyedLintRule(nil), prev...), statusKeyedLintRule{
			name: "plantedFlipRule",
			problems: func(e *lintEnv) []string {
				for _, s := range e.streams {
					for _, b := range s.Briefs {
						if s.Name == "dg" && b.Num == "05" && b.Status == "implemented" {
							return []string{"planted: dg/05 may not reach implemented"}
						}
					}
				}
				return nil
			},
		})
		t.Cleanup(func() { statusKeyedLintRules = prev })
		root := copyDesignGateFixture(t)
		addDesignRecord(t, root)
		assertFlipHeld(t, root, "dg", "05", dgMerged, "planted: dg/05 may not reach implemented")
	})

	t.Run("with the record the label is plain and --apply writes the row", func(t *testing.T) {
		root := copyDesignGateFixture(t)
		addDesignRecord(t, root)
		streams := flipFixtureStreams(t, root)
		applyPhantomHolds(root, streams, dgMerged, nil)
		s, b := flipRow(t, streams, "dg", "05")
		if want := "already-merged-unflipped (merged PR #7, Brief: trailer)"; b.PhantomHeld != want || b.PhantomFlipHeld {
			t.Errorf("a row the reconcile would write keeps the plain label; got %q (flip-held=%v), want %q", b.PhantomHeld, b.PhantomFlipHeld, want)
		}
		if eligibleBase(streams, s, b, map[string]bool{}, nil) {
			t.Error("a delivered row stays held out of dispatch until it is reconciled")
		}
		if n := flipNotice(t, root, streams, dgMerged, "dg/05"); !strings.Contains(n, "reconcile the row") || strings.Contains(n, "flip held") {
			t.Errorf("an admitted row's fix is the reconcile: %s", n)
		}
		applied, held, err := applyReconcileWrites(root, []BriefCell{{ID: "dg/05", Cell: "implemented", Source: "pr", Witness: "PR #7"}})
		if err != nil || len(held) != 0 || len(applied) != 1 {
			t.Fatalf("the reconcile must write the row the planner calls plain; applied=%+v held=%+v err=%v", applied, held, err)
		}
		if !strings.Contains(readFixtureReadme(t, root), "| 05 | [Risk-gated, todo, no record](./brief-05-todo.md) | 0 | S | implemented |") {
			t.Fatalf("dg/05 was not written to implemented:\n%s", readFixtureReadme(t, root))
		}
	})

	t.Run("the board names flip-held rows apart from reconcile-able ones", func(t *testing.T) {
		nu := NextUp{
			PhantomElsewhere: map[string]string{
				"dg/04": "already-merged-unflipped (merged PR #6, Brief: trailer)",
				"dg/05": "already-merged-unflipped (merged PR #7, Brief: trailer; flip held: dg/05: no design: record)",
			},
			PhantomFlipHeld: map[string]bool{"dg/05": true},
		}
		out := emit(nil, nil, nu, nil, nil, IntakeAlarmResult{}, nil, "")
		var plain, flip string
		for _, l := range strings.Split(out, "\n") {
			switch {
			case strings.Contains(l, "flip held by the reconcile"):
				flip = l
			case strings.Contains(l, "Already delivered or re-homed"):
				plain = l
			}
		}
		if !strings.Contains(plain, "dg/04") || strings.Contains(plain, "dg/05") {
			t.Errorf("the reconcile-or-retire line must carry dg/04 only: %q", plain)
		}
		if !strings.Contains(flip, "dg/05") || strings.Contains(flip, "dg/04") || strings.Contains(flip, "reconcile the row") {
			t.Errorf("the flip-held line must carry dg/05 only, with the record fix: %q", flip)
		}
	})
}
