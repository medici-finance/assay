package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// dgProblems copies the shared design-gate fixture tree into a temp root, loads
// the streams, and returns the design-approval gate problems.
func dgProblems(t *testing.T) []string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/designgate")); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	return designGateProblems(root, streams)
}

// TestDesignGate exercises the whole design-approval gate over the committed
// fixture stream: the positive path, the mutation (negative) path the transition
// must actually gate on, the scope skip, the grandfather, the todo exemption and
// the dangling-reference case — one subtest each, all under the single name the
// brief's Verify row 3 selects.
func TestDesignGate(t *testing.T) {
	problems := dgProblems(t)

	t.Run("RecordPresentIsClean", func(t *testing.T) {
		// dg/01: risk-gated, in-progress, cites an existing DR record → no problem.
		if hasProblem(problems, "dg/brief-01") {
			t.Errorf("a risk-gated in-progress brief citing an approved design record must pass; got:\n%s", strings.Join(problems, "\n"))
		}
	})

	t.Run("NoRecordGatesTheTransition", func(t *testing.T) {
		// dg/02: risk-gated, in-progress, NO design record → PROBLEM naming it.
		// This is the negative-path row (Verify row 4): the transition must gate.
		if !hasProblem(problems, "dg/brief-02", "no design: record") {
			t.Errorf("a risk-gated in-progress brief with no design record must be flagged (the gate must actually gate); got:\n%s", strings.Join(problems, "\n"))
		}
	})

	t.Run("ModelGateIsScopedOut", func(t *testing.T) {
		// dg/03: gate: model, all risks no, in-progress, NO record → NOT flagged.
		// This is the scope row (Verify row 5): the gate is not a blanket new
		// obligation on every brief in every corpus.
		if hasProblem(problems, "dg/brief-03") {
			t.Errorf("a gate: model all-risks-no brief must be scoped OUT of the design gate; got:\n%s", strings.Join(problems, "\n"))
		}
	})

	t.Run("PreCutoverIsGrandfathered", func(t *testing.T) {
		// dg/04: risk-gated, in-progress, authored before the cutover → exempt, so
		// a pin bump reds nothing already in flight.
		if hasProblem(problems, "dg/brief-04") {
			t.Errorf("a risk-gated brief authored on or before the cutover must be grandfathered; got:\n%s", strings.Join(problems, "\n"))
		}
	})

	t.Run("TodoIsNotYetGated", func(t *testing.T) {
		// dg/05: risk-gated, still todo → the transition has not happened, so the
		// gate does not fire yet.
		if hasProblem(problems, "dg/brief-05") {
			t.Errorf("a brief still at todo has not made the guarded move and must not be gated; got:\n%s", strings.Join(problems, "\n"))
		}
	})

	t.Run("DanglingReferenceIsFlagged", func(t *testing.T) {
		// dg/06: risk-gated, in-progress, design: to a record that does not exist →
		// a dangling reference gates nothing and is flagged.
		if !hasProblem(problems, "dg/brief-06", "dereferences to no record") {
			t.Errorf("a design: reference that resolves to no record must be flagged; got:\n%s", strings.Join(problems, "\n"))
		}
	})
}

// TestDesignGateMutationReversion pins the Verify-row-4 revert step: with dg/02's
// row status backed off to todo, the gate goes quiet — proving the flag in the
// mutation subtest above is caused by the transition, not by the fixture's mere
// presence.
func TestDesignGateMutationReversion(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/designgate")); err != nil {
		t.Fatal(err)
	}
	readme := filepath.Join(root, "docs", "streams", "dg", "README.md")
	raw, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	// Back dg/02 off from in-progress to todo (the revert of the row-4 mutation).
	mutated := strings.Replace(string(raw),
		"| 02 | [Risk-gated, in-progress, no record](./brief-02-no-record.md) | 0 | S | in-progress |",
		"| 02 | [Risk-gated, in-progress, no record](./brief-02-no-record.md) | 0 | S | todo |", 1)
	if mutated == string(raw) {
		t.Fatal("fixture README row for dg/02 not found — test is stale")
	}
	if err := os.WriteFile(readme, []byte(mutated), 0o644); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	if hasProblem(designGateProblems(root, streams), "dg/brief-02") {
		t.Error("with dg/02 reverted to todo the gate must be silent — the flag must be caused by the transition, not the fixture")
	}
}

// TestDesignGateRegisterShape validates the DECISIONS register's own entry-shape
// rules: the ordered consequence axis (registers-v1 §3.5), a human decided-by
// stamp, and the enumerated alternatives/accepted lists.
func TestDesignGateRegisterShape(t *testing.T) {
	write := func(t *testing.T, root, name, body string) {
		t.Helper()
		dir := filepath.Join(root, "docs", "streams", "decisions")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("ValidEntryIsClean", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "DR-good-record.md", "---\nid: DR-good-record\ndate: \"2026-09-10\"\ntitle: t\nconsequence: minor\ndecided-by: \"human:ada\"\nalternatives: [\"a — ruled out\"]\naccepted: [\"one cost\"]\n---\nbody\n")
		if p := decisionRegisterProblems(root); len(p) != 0 {
			t.Errorf("a well-formed decision record must be clean; got:\n%s", strings.Join(p, "\n"))
		}
	})

	t.Run("MissingConsequenceAxisFlagged", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "DR-no-axis-here.md", "---\nid: DR-no-axis-here\ndate: \"2026-09-10\"\ntitle: t\ndecided-by: \"human:ada\"\nalternatives: [\"a\"]\naccepted: [\"c\"]\n---\nbody\n")
		if !hasProblem(decisionRegisterProblems(root), "consequence", "registers-v1 §3.5") {
			t.Error("a decision record with no consequence axis must be flagged (registers-v1 §3.5)")
		}
	})

	t.Run("NonHumanDecidedByFlagged", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "DR-model-signed.md", "---\nid: DR-model-signed\ndate: \"2026-09-10\"\ntitle: t\nconsequence: major\ndecided-by: \"model:sonnet\"\nalternatives: [\"a\"]\naccepted: [\"c\"]\n---\nbody\n")
		if !hasProblem(decisionRegisterProblems(root), "decided-by", "human") {
			t.Error("a decision record signed by a model rather than a human must be flagged")
		}
	})

	t.Run("NoAlternativesFlagged", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "DR-no-alternatives.md", "---\nid: DR-no-alternatives\ndate: \"2026-09-10\"\ntitle: t\nconsequence: major\ndecided-by: \"human:ada\"\nalternatives: []\naccepted: [\"c\"]\n---\nbody\n")
		if !hasProblem(decisionRegisterProblems(root), "alternatives") {
			t.Error("a decision record enumerating no alternatives must be flagged")
		}
	})
}

// TestDesignGateThreeState pins the could-not-check posture: an absent register is
// a legitimate empty (no problem, no notice), while an UNREADABLE one is a
// could-not-check surfaced as a PROBLEM and a NOTICE, never a silent clean pass.
func TestDesignGateThreeState(t *testing.T) {
	t.Run("AbsentRegisterIsClean", func(t *testing.T) {
		root := t.TempDir() // no docs/streams/decisions/ at all
		if p := decisionRegisterProblems(root); len(p) != 0 {
			t.Errorf("an absent decisions register is a legitimate empty, not a problem; got:\n%s", strings.Join(p, "\n"))
		}
		if n := designGateNotices(root, nil); len(n) != 0 {
			t.Errorf("an absent register emits no notice; got:\n%s", strings.Join(n, "\n"))
		}
	})

	t.Run("UnreadableRegisterIsCouldNotCheck", func(t *testing.T) {
		root := t.TempDir()
		// A FILE where the register directory is expected → os.ReadDir returns a
		// non-IsNotExist error, the unreadable branch.
		if err := os.MkdirAll(filepath.Join(root, "docs", "streams"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "docs", "streams", "decisions"), []byte("not a dir"), 0o644); err != nil {
			t.Fatal(err)
		}
		if !hasProblem(decisionRegisterProblems(root), "decisions register unreadable") {
			t.Error("an unreadable register must be a could-not-check PROBLEM, never a silent clean pass")
		}
		if n := designGateNotices(root, nil); !strings.Contains(strings.Join(n, "\n"), "COULD-NOT-CHECK") {
			t.Errorf("an unreadable register must surface a COULD-NOT-CHECK notice; got:\n%s", strings.Join(n, "\n"))
		}
	})
}
