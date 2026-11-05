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
	if !strings.Contains(b.PhantomHeld, "Brief: trailer; flip held: ") || b.PhantomFlip != phantomFlipHeld {
		t.Errorf("a row the reconcile refuses must be labelled flip-held; got %q (flip state=%v)", b.PhantomHeld, b.PhantomFlip)
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
	if nu.PhantomElsewhere[id] == "" || nu.PhantomFlip[id] != phantomFlipHeld {
		t.Errorf("Next-up must name %s as flip-held; elsewhere=%v flip=%v", id, nu.PhantomElsewhere, nu.PhantomFlip)
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
		if want := "already-merged-unflipped (merged PR #7, Brief: trailer)"; b.PhantomHeld != want || b.PhantomFlip != phantomFlipClean {
			t.Errorf("a row the reconcile would write keeps the plain label; got %q (flip state=%v), want %q", b.PhantomHeld, b.PhantomFlip, want)
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

	t.Run("the board names each flip state on its own line", func(t *testing.T) {
		nu := NextUp{
			PhantomElsewhere: map[string]string{
				"dg/04": "already-merged-unflipped (merged PR #6, Brief: trailer)",
				"dg/05": "already-merged-unflipped (merged PR #7, Brief: trailer; flip held: dg/05: no design: record)",
				"dg/06": "already-merged-unflipped (merged PR #8, Brief: trailer; flip hold could-not-check: no row)",
			},
			PhantomFlip: map[string]phantomFlipState{"dg/05": phantomFlipHeld, "dg/06": phantomFlipUnchecked},
		}
		out := emit(nil, nil, nu, nil, nil, IntakeAlarmResult{}, nil, "")
		var plain, held, unchecked string
		for _, l := range strings.Split(out, "\n") {
			switch {
			case strings.Contains(l, "flip held by the reconcile"):
				held = l
			case strings.Contains(l, "flip hold could-not-check"):
				unchecked = l
			case strings.Contains(l, "Already delivered or re-homed"):
				plain = l
			}
		}
		if !strings.Contains(plain, "dg/04") || strings.Contains(plain, "dg/05") || strings.Contains(plain, "dg/06") {
			t.Errorf("the reconcile-or-retire line must carry dg/04 only: %q", plain)
		}
		if !strings.Contains(held, "dg/05") || strings.Contains(held, "dg/04") || strings.Contains(held, "dg/06") || strings.Contains(held, "reconcile the row") {
			t.Errorf("the flip-held line must carry dg/05 only, with the record fix: %q", held)
		}
		if !strings.Contains(unchecked, "dg/06") || strings.Contains(unchecked, "dg/05") || strings.Contains(unchecked, "`design:`") || !strings.Contains(unchecked, "--backfill") {
			t.Errorf("the could-not-check line must carry dg/06 only, with the evaluation fix: %q", unchecked)
		}
	})
}

// flipBoardLine returns the Next-up board line that names id as held out of
// dispatch, rendered by the real nextUp -> emit path.
func flipBoardLine(t *testing.T, streams []*Stream, id string) string {
	t.Helper()
	nu := nextUp(streams, ClaimView{}, nil)
	out := emit(nil, nil, nu, nil, nil, IntakeAlarmResult{}, nil, "")
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, id+" → ") {
			return l
		}
	}
	t.Fatalf("the board names no held line for %s:\n%s", id, out)
	return ""
}

// regression: #4604
func TestPlannerFlipUncheckedNotHeld(t *testing.T) {
	dgMerged := []mergedPR{{Number: 7, Subject: "feat: dg/05 (#7)", Briefs: []string{"dg/05"}, Delivers: []string{"dg/05"}}}
	dgReadme := func(root string) string { return filepath.Join(root, "docs", "streams", "dg", "README.md") }
	const row05 = "| 05 | [Risk-gated, todo, no record](./brief-05-todo.md) | 0 | S | todo | — | — |"

	cases := []struct {
		name   string
		break_ func(t *testing.T, root string)
		reason string
	}{
		{"the evaluator errors", func(t *testing.T, root string) {
			// A stream directory with no README makes the reconcile's board load fail.
			if err := os.MkdirAll(filepath.Join(root, "docs", "streams", "zz-broken"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, "hold evaluation failed"},
		{"the reconcile finds no row to write", func(t *testing.T, root string) {
			b, _ := os.ReadFile(dgReadme(root))
			if !strings.Contains(string(b), row05) {
				t.Fatal("fixture drift: dg/05 row changed")
			}
			if err := os.WriteFile(dgReadme(root), []byte(strings.Replace(string(b), row05+"\n", "", 1)), 0o644); err != nil {
				t.Fatal(err)
			}
		}, "found no todo row for dg/05"},
		{"the lint has no stream model for the row", func(t *testing.T, root string) {
			// The row survives only in a table the stream loader does not read, so the
			// reconcile finds a README row but the lint model has none.
			b, _ := os.ReadFile(dgReadme(root))
			hdr := "| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n|---|---|---|---|---|---|---|\n"
			moved := strings.Replace(string(b), row05+"\n", "", 1) + "\n## More\n\n" + hdr + row05 + "\n"
			if err := os.WriteFile(dgReadme(root), []byte(moved), 0o644); err != nil {
				t.Fatal(err)
			}
		}, "no row in the lint's stream model"},
		{"a future could-not-check producer (planted)", func(t *testing.T, root string) {
			prev := planFlipWrites
			planFlipWrites = func(root string, cells []BriefCell) ([]appliedRow, []heldRow, error) {
				return nil, []heldRow{{ID: "dg/05", Problems: []string{"could-not-check: a future rule cannot read its input"}}}, nil
			}
			t.Cleanup(func() { planFlipWrites = prev })
		}, "a future rule cannot read its input"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := copyDesignGateFixture(t)
			streams := flipFixtureStreams(t, root) // loaded before the break, as the planner holds them
			c.break_(t, root)
			applyPhantomHolds(root, streams, dgMerged, nil)
			s, b := flipRow(t, streams, "dg", "05")
			if b.PhantomHeld == "" || eligibleBase(streams, s, b, map[string]bool{}, nil) {
				t.Fatalf("a delivered row whose flip could not be checked must stay held out of dispatch; label %q", b.PhantomHeld)
			}
			for where, text := range map[string]string{
				"label":  b.PhantomHeld,
				"NOTICE": flipNotice(t, root, streams, dgMerged, "dg/05"),
				"board":  flipBoardLine(t, streams, "dg/05"),
			} {
				if !strings.Contains(text, "flip hold could-not-check") || !strings.Contains(text, c.reason) {
					t.Errorf("%s must say the flip hold could-not-check (%q): %s", where, c.reason, text)
				}
				if strings.Contains(text, "flip held") || strings.Contains(text, "`design:`") || strings.Contains(text, "`ruling:`") {
					t.Errorf("%s must not render an unchecked flip as a refusal with the design/ruling fix: %s", where, text)
				}
			}
			for _, where := range []string{flipNotice(t, root, streams, dgMerged, "dg/05"), flipBoardLine(t, streams, "dg/05")} {
				if !strings.Contains(where, "statusgen reconcile --backfill") {
					t.Errorf("the fix must send the reader to the evaluation: %s", where)
				}
			}
		})
	}
}

// countFlipEvals counts the reconcile hold evaluations the planner starts.
func countFlipEvals(t *testing.T) *int {
	t.Helper()
	n := 0
	prev := planFlipWrites
	planFlipWrites = func(root string, cells []BriefCell) ([]appliedRow, []heldRow, error) {
		n++
		return prev(root, cells)
	}
	t.Cleanup(func() { planFlipWrites = prev })
	return &n
}

// regression: #4604
func TestFlipVerdictsOncePerRun(t *testing.T) {
	dgMerged := []mergedPR{{Number: 7, Subject: "feat: dg/05 (#7)", Briefs: []string{"dg/05"}, Delivers: []string{"dg/05"}}}

	t.Run("both planner callers share one evaluation inside a run", func(t *testing.T) {
		root := copyDesignGateFixture(t)
		streams := flipFixtureStreams(t, root)
		n := countFlipEvals(t)
		end := beginFlipMemo()
		notices := boardHonestyNotices(root, streams, dgMerged, nil)
		applyPhantomHolds(root, streams, dgMerged, nil)
		phantomFindings(root, streams, dgMerged, nil) // a further caller, added later, must cost nothing either
		end()
		if *n != 1 {
			t.Errorf("the whole-board hold must be evaluated once per run; evaluated %d times", *n)
		}
		_, b := flipRow(t, streams, "dg", "05")
		if !strings.Contains(strings.Join(notices, "\n"), "flip held: ") || !strings.Contains(b.PhantomHeld, "flip held: ") {
			t.Errorf("the shared verdict must reach both callers: %q", b.PhantomHeld)
		}
	})

	t.Run("outside a run nothing is remembered", func(t *testing.T) {
		root := copyDesignGateFixture(t)
		streams := flipFixtureStreams(t, root)
		n := countFlipEvals(t)
		boardHonestyNotices(root, streams, dgMerged, nil)
		addDesignRecord(t, root)
		applyPhantomHolds(root, streams, dgMerged, nil)
		if *n != 2 {
			t.Errorf("a verdict must not outlive a run (the tree may have changed); evaluated %d times", *n)
		}
	})

	t.Run("a verdict is per root", func(t *testing.T) {
		a, b := copyDesignGateFixture(t), copyDesignGateFixture(t)
		addDesignRecord(t, b)
		sa, sb := flipFixtureStreams(t, a), flipFixtureStreams(t, b)
		end := beginFlipMemo()
		defer end()
		applyPhantomHolds(a, sa, dgMerged, nil)
		applyPhantomHolds(b, sb, dgMerged, nil)
		if _, rb := flipRow(t, sb, "dg", "05"); rb.PhantomFlip != phantomFlipClean {
			t.Errorf("root b has the design record; root a's verdict must not leak into it: %q", rb.PhantomHeld)
		}
	})

	// The wiring, end to end: run() itself must open the shared scope, or a
	// future second caller of the planner doubles --lint again.
	t.Run("run evaluates the hold once", func(t *testing.T) {
		root := copyDesignGateFixture(t)
		mustGitInit(t, root)
		if err := os.WriteFile(filepath.Join(root, "landed.txt"), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		mustGitCommitAll(t, root, "feat: dg/05 (#7)\n\nBrief: dg/05")
		n := countFlipEvals(t)
		captureStderr(t, func() { captureStdout(t, func() { run(root, "lint", nil, nil, "") }) })
		if *n != 1 {
			t.Errorf("run(lint) must evaluate the whole-board hold exactly once; evaluated %d times", *n)
		}
	})
}
