package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
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
		Test: func(string) bool { suiteRan = true; return false /* green */ },
	}

	// Old text is absent → the edit cannot land. This is exactly the #34 case:
	// the box's sed silently no-op'd, the source stayed pristine, the suite
	// went green. Here it MUST become CouldNotMutate, and the suite must not run.
	res, err := h.runOne(dir, Mutation{Name: "phantom", File: "core.txt", Old: "n != p", New: "boom"})
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
		Test: func(string) bool {
			b, _ := os.ReadFile(path)
			return !strings.Contains(string(b), "n == p\n") // red == caught
		},
	}
	res, err := h.runOne(dir, Mutation{Name: "break-invariant", File: "core.txt", Old: "n == p", New: "n == q"})
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
		Test: func(string) bool {
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
		Test:    func(string) bool { return false /* always green — broken instrument */ },
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
		Test:    func(string) bool { return false },
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
		Test:    func(string) bool { return true /* red on pristine source */ },
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

// --- copyTree: an isolated workspace is a real, independent copy ---

func TestCopyTree_CopiesNestedFilesAndIsolatesWrites(t *testing.T) {
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "pkg", "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	files := map[string]string{
		"top.txt":               "top\n",
		"pkg/mid.txt":           "mid\n",
		"pkg/sub/deep.txt":      "deep\n",
		"pkg/sub/executable.sh": "#!/bin/sh\n",
	}
	for name, body := range files {
		mode := os.FileMode(0o644)
		if strings.HasSuffix(name, ".sh") {
			mode = 0o755
		}
		if err := os.WriteFile(filepath.Join(src, name), []byte(body), mode); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	dst := filepath.Join(t.TempDir(), "clone")
	if err := copyTree(src, dst); err != nil {
		t.Fatalf("copyTree: %v", err)
	}
	for name, body := range files {
		got, err := os.ReadFile(filepath.Join(dst, name))
		if err != nil {
			t.Fatalf("read copy %s: %v", name, err)
		}
		if string(got) != body {
			t.Fatalf("copy of %s: got %q want %q", name, got, body)
		}
	}
	if info, err := os.Stat(filepath.Join(dst, "pkg/sub/executable.sh")); err != nil {
		t.Fatalf("stat copied script: %v", err)
	} else if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("executable bit not preserved by the copy: %v", info.Mode())
	}

	// The whole point: mutating the copy must not touch the original. A sweep
	// whose workspaces aliased the source would corrupt every other worker.
	if err := os.WriteFile(filepath.Join(dst, "top.txt"), []byte("MUTATED\n"), 0o644); err != nil {
		t.Fatalf("write into copy: %v", err)
	}
	orig, err := os.ReadFile(filepath.Join(src, "top.txt"))
	if err != nil {
		t.Fatalf("re-read source: %v", err)
	}
	if string(orig) != "top\n" {
		t.Fatalf("writing the copy changed the source — the workspace is not isolated: %q", orig)
	}
}

// mixedHarness builds a harness over a small synthetic tree whose suite is red
// iff any GUARDED token is missing. It deliberately yields all three result
// states: caught guards, one unguarded token (a real NotCaught finding) and one
// mutation whose Old text is absent (CouldNotMutate).
func mixedHarness(t *testing.T) (*Harness, string) {
	t.Helper()
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "pkg"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	const core = "invariant: n == p\nguard: breaker.active\nguard: quorum.reached\n"
	const aux = "guard: signer.verified\nnote: cosmetic.banner\n"
	if err := os.WriteFile(filepath.Join(src, "core.txt"), []byte(core), 0o644); err != nil {
		t.Fatalf("write core: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "pkg", "aux.txt"), []byte(aux), 0o644); err != nil {
		t.Fatalf("write aux: %v", err)
	}

	guarded := []string{"n == p", "breaker.active", "quorum.reached", "signer.verified"}
	h := &Harness{
		Root: src,
		Test: func(root string) bool {
			var all strings.Builder
			for _, f := range []string{"core.txt", filepath.Join("pkg", "aux.txt")} {
				b, err := os.ReadFile(filepath.Join(root, f))
				if err != nil {
					return true // unreadable tree == red, never a silent green
				}
				all.Write(b)
			}
			for _, tok := range guarded {
				if !strings.Contains(all.String(), tok) {
					return true // red == caught
				}
			}
			return false
		},
		Control: Mutation{Name: "value-conservation", File: "core.txt", Old: "n == p", New: "n != p"},
		Mutations: []Mutation{
			{Name: "breaker-active check", File: "core.txt", Old: "breaker.active", New: "true"},
			{Name: "quorum check", File: "core.txt", Old: "quorum.reached", New: "true"},
			{Name: "signer check", File: "pkg/aux.txt", Old: "signer.verified", New: "true"},
			{Name: "cosmetic banner (no discriminating test)", File: "pkg/aux.txt", Old: "cosmetic.banner", New: "changed"},
			{Name: "phantom target", File: "core.txt", Old: "NOT PRESENT ANYWHERE", New: "x"},
		},
	}
	return h, src
}

// tempWorkspace is the same provisioner main.go installs: one throwaway copy of
// src per worker.
func tempWorkspace(t *testing.T, src string) func(int) (string, func(), error) {
	t.Helper()
	return func(id int) (string, func(), error) {
		dir := filepath.Join(t.TempDir(), "ws")
		if err := copyTree(src, dir); err != nil {
			return "", nil, err
		}
		return dir, func() { _ = os.RemoveAll(dir) }, nil
	}
}

// --- the equivalence bar: -j changes the wall time, never the verdicts ---

func TestRun_ParallelVerdictsMatchSequentialExactly(t *testing.T) {
	seqH, _ := mixedHarness(t)
	seq := seqH.Run()

	parH, parSrc := mixedHarness(t)
	parH.Jobs = 4
	parH.Workspace = tempWorkspace(t, parSrc)
	par := parH.Run()

	if seq.Broken || par.Broken {
		t.Fatalf("neither run should be broken: seq=%v/%s par=%v/%s", seq.Broken, seq.BrokenReason, par.Broken, par.BrokenReason)
	}
	if seq.BaselineGreen != par.BaselineGreen || seq.ControlResult != par.ControlResult {
		t.Fatalf("preflight differs: seq baseline=%v control=%s, par baseline=%v control=%s",
			seq.BaselineGreen, seq.ControlResult, par.BaselineGreen, par.ControlResult)
	}
	if len(seq.Results) != len(par.Results) {
		t.Fatalf("result count differs: seq=%d par=%d", len(seq.Results), len(par.Results))
	}
	for i := range seq.Results {
		if seq.Results[i].Mutation.Name != par.Results[i].Mutation.Name {
			t.Fatalf("result %d is a different mutation — parallel reordered the report: seq=%q par=%q",
				i, seq.Results[i].Mutation.Name, par.Results[i].Mutation.Name)
		}
		if seq.Results[i].Result != par.Results[i].Result {
			t.Fatalf("verdict differs for %q: seq=%s par=%s",
				seq.Results[i].Mutation.Name, seq.Results[i].Result, par.Results[i].Result)
		}
	}
	// The gate CI reads is the totals line; it must be byte-identical.
	seqTotals, parTotals := totalsLine(t, seq.Summary()), totalsLine(t, par.Summary())
	if seqTotals != parTotals {
		t.Fatalf("totals line differs:\n  seq: %s\n  par: %s", seqTotals, parTotals)
	}
	// And the fixture must actually exercise all three states, or this proves nothing.
	if !strings.Contains(seqTotals, "3 caught, 1 NOT CAUGHT, 1 could-not-mutate") {
		t.Fatalf("fixture no longer covers all three result states: %s", seqTotals)
	}
}

func totalsLine(t *testing.T, summary string) string {
	t.Helper()
	for _, line := range strings.Split(summary, "\n") {
		if strings.HasPrefix(line, "Totals:") {
			return line
		}
	}
	t.Fatalf("no totals line in summary:\n%s", summary)
	return ""
}

// --- a parallel sweep must really be parallel, and never share a tree ---

func TestRun_ParallelActuallyOverlapsAndKeepsWorkspacesDisjoint(t *testing.T) {
	h, src := mixedHarness(t)
	const jobs = 4
	h.Jobs = jobs
	h.Workspace = tempWorkspace(t, src)

	var mu sync.Mutex
	inFlight, maxInFlight := 0, 0
	seenRoots := map[string]bool{}
	inner := h.Test
	h.Test = func(root string) bool {
		mu.Lock()
		inFlight++
		if inFlight > maxInFlight {
			maxInFlight = inFlight
		}
		seenRoots[root] = true
		mu.Unlock()
		// Hold the slot long enough that genuinely concurrent workers overlap.
		time.Sleep(30 * time.Millisecond)
		res := inner(root)
		mu.Lock()
		inFlight--
		mu.Unlock()
		return res
	}

	rep := h.Run()
	if rep.Broken {
		t.Fatalf("run reported broken: %s", rep.BrokenReason)
	}
	if maxInFlight < 2 {
		t.Fatalf("no two suite runs ever overlapped (max in flight %d) — -j is not actually parallelising", maxInFlight)
	}
	if len(seenRoots) < 2 {
		t.Fatalf("every worker ran in the same root %v — workspaces are not isolated, so the verdicts cannot be trusted", seenRoots)
	}
	if seenRoots[src] {
		t.Fatalf("a parallel worker ran against the ORIGINAL tree %q instead of a copy", src)
	}
}

// --- the broken-harness gates still fire in parallel mode ---

func TestRun_ParallelUncaughtControlStillDiscardsRun(t *testing.T) {
	h, src := mixedHarness(t)
	h.Jobs = 4
	h.Workspace = tempWorkspace(t, src)
	h.Test = func(string) bool { return false } // stuck green light

	rep := h.Run()
	if !rep.Broken {
		t.Fatal("an uncaught positive control must mark a parallel run BROKEN too (#34)")
	}
	if strings.Contains(rep.Summary(), "NOT_CAUGHT") {
		t.Fatalf("a broken parallel run must not publish per-guard verdicts:\n%s", rep.Summary())
	}
}

func TestRun_ParallelRedBaselineDiscardsRun(t *testing.T) {
	h, src := mixedHarness(t)
	h.Jobs = 4
	h.Workspace = tempWorkspace(t, src)
	h.Test = func(string) bool { return true } // red before anything is mutated

	rep := h.Run()
	if !rep.Broken {
		t.Fatal("a red baseline must mark a parallel run BROKEN too")
	}
	if !strings.Contains(rep.BrokenReason, "baseline") {
		t.Fatalf("broken reason should mention the baseline, got: %s", rep.BrokenReason)
	}
}

func TestRun_ParallelWorkspaceFailureIsBrokenNotAGreenSweep(t *testing.T) {
	h, _ := mixedHarness(t)
	h.Jobs = 4
	h.Workspace = func(int) (string, func(), error) {
		return "", nil, errors.New("no space left on device")
	}

	rep := h.Run()
	if !rep.Broken {
		t.Fatal("a workspace that could not be provisioned must mark the run BROKEN — never fall back to a shared tree")
	}
	if !strings.Contains(rep.BrokenReason, "workspace") {
		t.Fatalf("broken reason should name the workspace, got: %s", rep.BrokenReason)
	}
	if strings.Contains(rep.Summary(), "Totals:") {
		t.Fatalf("a broken run must not emit a totals line — CI gates on it:\n%s", rep.Summary())
	}
}

// A Jobs > 1 with no Workspace provisioner must NOT parallelise over a shared
// tree; it falls back to the sequential in-place sweep.
func TestRun_JobsWithoutWorkspaceFallsBackToSequential(t *testing.T) {
	h, src := mixedHarness(t)
	h.Jobs = 8 // no Workspace set

	var mu sync.Mutex
	roots := map[string]bool{}
	inner := h.Test
	h.Test = func(root string) bool {
		mu.Lock()
		roots[root] = true
		mu.Unlock()
		return inner(root)
	}

	rep := h.Run()
	if rep.Broken {
		t.Fatalf("fallback run reported broken: %s", rep.BrokenReason)
	}
	if len(roots) != 1 || !roots[src] {
		t.Fatalf("without a Workspace the sweep must stay in place at %q, ran in %v", src, roots)
	}
}
