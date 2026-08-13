package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSrc writes a source file into a fresh temp dir and returns dir + path.
func writeSrc(t *testing.T, name, content string) (dir, path string) {
	t.Helper()
	dir = t.TempDir()
	path = filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	return dir, path
}

// --- applyEdit: a silent no-op must ERROR, never silently succeed ---

func TestApplyEdit_NoopVariantsError(t *testing.T) {
	const src = "invariant: n == p\n"
	cases := []struct {
		name string
		m    Mutation
	}{
		{"absent Old (the sed -i '' no-op shape)", Mutation{Name: "x", Old: "n != p", New: "n == p+1"}},
		{"Old == New", Mutation{Name: "x", Old: "n == p", New: "n == p"}},
		{"empty Old", Mutation{Name: "x", Old: "", New: "whatever"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := applyEdit(src, tc.m); err == nil {
				t.Fatalf("expected an error for a no-op edit, got nil — a silent no-op would be read as a mutated file")
			}
		})
	}
}

func TestApplyEdit_RealEditSucceeds(t *testing.T) {
	const src = "invariant: n == p\n"
	got, err := applyEdit(src, Mutation{Name: "x", Old: "n == p", New: "n == p + 1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == src {
		t.Fatalf("edit did not change the content")
	}
	if !strings.Contains(got, "n == p + 1") {
		t.Fatalf("mutated content missing the replacement: %q", got)
	}
}

// --- runOne: a no-op mutation is CouldNotMutate, and it NEVER runs the suite ---

func TestRunOne_SilentNoopIsCouldNotMutateNotNotCaught(t *testing.T) {
	dir, _ := writeSrc(t, "core.txt", "invariant: n == p\n")

	suiteRan := false
	h := &Harness{
		Root: dir,
		Test: func() bool { suiteRan = true; return false /* green */ },
	}

	// Old text is absent → the edit cannot land. This is exactly the #34 case:
	// the box's sed silently no-op'd, the source stayed pristine, the suite
	// went green. Here it MUST become CouldNotMutate, and the suite must not run.
	res, err := h.runOne(Mutation{Name: "phantom", File: "core.txt", Old: "n != p", New: "boom"})
	if res != CouldNotMutate {
		t.Fatalf("a no-op mutation must be CouldNotMutate, got %s", res)
	}
	if res == NotCaught {
		t.Fatal("a no-op mutation collapsed into NotCaught — the #34 false-clean bug")
	}
	if err == nil {
		t.Fatal("expected an explanatory error for the failed edit")
	}
	if suiteRan {
		t.Fatal("the suite ran against pristine source for an unlanded mutation — the exact #34 trap")
	}
}

func TestRunOne_LandedMutationRestoresFileAndReportsCaught(t *testing.T) {
	const src = "invariant: n == p\n"
	dir, path := writeSrc(t, "core.txt", src)

	// The suite is "red iff the invariant is broken": it reads the file from
	// disk, so it genuinely proves the mutation was present when it ran.
	h := &Harness{
		Root: dir,
		Test: func() bool {
			b, _ := os.ReadFile(path)
			return !strings.Contains(string(b), "n == p\n") // red == caught
		},
	}
	res, err := h.runOne(Mutation{Name: "break-invariant", File: "core.txt", Old: "n == p", New: "n == q"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != Caught {
		t.Fatalf("a landed, test-detected mutation must be Caught, got %s", res)
	}
	// File must be restored byte-for-byte after the run.
	after, _ := os.ReadFile(path)
	if string(after) != src {
		t.Fatalf("file not restored after mutation: got %q want %q", string(after), src)
	}
}

// --- Run: the positive control catches its planted mutation (healthy run) ---

func TestRun_PositiveControlCatchesPlantedMutation(t *testing.T) {
	// core.txt holds a structural invariant (the control target) AND a guard
	// under test. The suite is red iff EITHER the invariant is broken OR the
	// guarded line is broken — i.e. both have discriminating tests.
	const src = "invariant: n == p\nguard: breaker.active\n"
	dir, path := writeSrc(t, "core.txt", src)

	h := &Harness{
		Root: dir,
		Test: func() bool {
			b, _ := os.ReadFile(path)
			s := string(b)
			brokenInvariant := !strings.Contains(s, "n == p")
			brokenGuard := !strings.Contains(s, "breaker.active")
			return brokenInvariant || brokenGuard // red == caught
		},
		// Replacement must not itself contain the original substring, or the
		// invariant check would still see "n == p" and think nothing broke.
		Control: Mutation{Name: "value-conservation", File: "core.txt", Old: "n == p", New: "n != p"},
		Mutations: []Mutation{
			{Name: "breaker-active check", File: "core.txt", Old: "breaker.active", New: "true"},
		},
	}

	rep := h.Run()
	if rep.Broken {
		t.Fatalf("healthy run reported broken: %s", rep.BrokenReason)
	}
	if !rep.BaselineGreen {
		t.Fatal("baseline should be green on pristine source")
	}
	if rep.ControlResult != Caught {
		t.Fatalf("positive control must be Caught in a healthy run, got %s", rep.ControlResult)
	}
	if len(rep.Results) != 1 || rep.Results[0].Result != Caught {
		t.Fatalf("guard should be Caught, got %+v", rep.Results)
	}
	// File restored after the whole run.
	after, _ := os.ReadFile(path)
	if string(after) != src {
		t.Fatalf("file not restored after run: got %q", string(after))
	}
}

// --- Run: a control that is NOT caught flags the whole harness as broken ---

func TestRun_UncaughtControlDiscardsRun(t *testing.T) {
	// The suite is a stuck green light — it NEVER goes red, whatever the source.
	// This models the #34 box where mutations never landed: the control cannot
	// be caught, so the run must be discarded, NOT reported as a clean gate.
	const src = "invariant: n == p\nguard: breaker.active\n"
	dir, _ := writeSrc(t, "core.txt", src)

	h := &Harness{
		Root:    dir,
		Test:    func() bool { return false /* always green — broken instrument */ },
		Control: Mutation{Name: "value-conservation", File: "core.txt", Old: "n == p", New: "n == p + 1"},
		Mutations: []Mutation{
			{Name: "breaker-active check", File: "core.txt", Old: "breaker.active", New: "true"},
		},
	}

	rep := h.Run()
	if !rep.Broken {
		t.Fatal("an uncaught positive control must mark the run BROKEN — otherwise a broken harness reads as a clean gate (#34)")
	}
	if rep.ControlResult == Caught {
		t.Fatal("control cannot be Caught against an always-green suite")
	}
	// The broken report must NOT publish per-guard verdicts.
	if strings.Contains(rep.Summary(), "NOT_CAUGHT") {
		t.Fatalf("a broken run must not emit per-guard NOT_CAUGHT verdicts:\n%s", rep.Summary())
	}
	if !strings.Contains(rep.Summary(), "HARNESS BROKEN") {
		t.Fatalf("broken run summary must say so:\n%s", rep.Summary())
	}
}

func TestRun_ControlThatCannotBePlantedDiscardsRun(t *testing.T) {
	// The control's Old text is absent → it can never be planted. That is the
	// literal #34 mechanism (sed no-op). The run must be discarded as broken,
	// with a "could not plant" reason — never treated as a clean gate.
	const src = "invariant: n == p\n"
	dir, _ := writeSrc(t, "core.txt", src)

	h := &Harness{
		Root:    dir,
		Test:    func() bool { return false },
		Control: Mutation{Name: "value-conservation", File: "core.txt", Old: "THIS TEXT IS NOT PRESENT", New: "x"},
	}
	rep := h.Run()
	if !rep.Broken {
		t.Fatal("a control that cannot be planted must mark the run BROKEN")
	}
	if !strings.Contains(rep.BrokenReason, "could not be planted") {
		t.Fatalf("broken reason should explain the control could not be planted, got: %s", rep.BrokenReason)
	}
}

func TestRun_RedBaselineDiscardsRun(t *testing.T) {
	// If pristine source is already red, no later "caught" means anything.
	dir, _ := writeSrc(t, "core.txt", "invariant: n == p\n")
	h := &Harness{
		Root:    dir,
		Test:    func() bool { return true /* red on pristine source */ },
		Control: Mutation{Name: "c", File: "core.txt", Old: "n == p", New: "n == q"},
	}
	rep := h.Run()
	if !rep.Broken {
		t.Fatal("a red baseline must mark the run BROKEN")
	}
	if !strings.Contains(rep.BrokenReason, "baseline") {
		t.Fatalf("broken reason should mention the baseline, got: %s", rep.BrokenReason)
	}
}
