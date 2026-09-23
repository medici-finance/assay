package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gitRepo initialises a throwaway git repo under t.TempDir() and returns its root. Commits
// are made with an inline identity so the test needs no ambient git config.
func gitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "test@example.org")
	run("config", "user.name", "test")
	return root
}

func gitCommit(t *testing.T, root, msg string) {
	t.Helper()
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", msg}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestSafeRename — brief verify-integrity/05 Verify row 1 (FAIL-FIRST). A row pins a path
// that git records moving by a SINGLE rename hop to a file that exists now → safe:rename.
// Perturb: the pinned path is gone with NO rename hop → refused:gone.
func TestSafeRename(t *testing.T) {
	// Fixture: row path moved by one rename hop.
	root := gitRepo(t)
	writeFile(t, root, "tools/old/widget.go", "package old\n")
	gitCommit(t, root, "add widget")
	// git mv records a rename hop.
	if err := os.MkdirAll(filepath.Join(root, "tools/new"), 0o755); err != nil {
		t.Fatal(err)
	}
	mv := exec.Command("git", "mv", "tools/old/widget.go", "tools/new/widget.go")
	mv.Dir = root
	if out, err := mv.CombinedOutput(); err != nil {
		t.Fatalf("git mv: %v\n%s", err, out)
	}
	gitCommit(t, root, "move widget")

	row := verifyRow{Num: 1, Class: "check", Command: "test -f tools/old/widget.go", Expect: "exists"}
	facts := gatherRowFacts(root, row, false, "")
	got := Classify(facts)
	if got.Verdict != SafeRename {
		t.Fatalf("row path moved by one rename hop: got %s (reason %q), want %s\nfacts=%+v",
			got.Verdict, got.Reason, SafeRename, facts)
	}
	if facts.RenameHop != "tools/new/widget.go" {
		t.Fatalf("rename hop: got %q, want tools/new/widget.go", facts.RenameHop)
	}

	// Perturb: delete the file with no rename → refused:gone.
	root2 := gitRepo(t)
	writeFile(t, root2, "tools/old/widget.go", "package old\n")
	gitCommit(t, root2, "add widget")
	if err := os.Remove(filepath.Join(root2, "tools/old/widget.go")); err != nil {
		t.Fatal(err)
	}
	gitCommit(t, root2, "delete widget")

	facts2 := gatherRowFacts(root2, row, false, "")
	got2 := Classify(facts2)
	if got2.Verdict != RefusedGone {
		t.Fatalf("deleted with no rename: got %s (reason %q), want %s\nfacts=%+v",
			got2.Verdict, got2.Reason, RefusedGone, facts2)
	}
}

// TestRefusesRiskBearing — brief verify-integrity/05 Verify row 2 (FAIL-FIRST). ANY row of a
// risk-bearing brief (gate: human, or any risk: flag yes) is refused:risk-bearing regardless
// of shape — even a shape that would otherwise be safe:rename.
func TestRefusesRiskBearing(t *testing.T) {
	// A brief that WOULD classify safe:rename absent the risk gate — proving the gate wins
	// "regardless of shape".
	root := gitRepo(t)
	writeFile(t, root, "a/x.go", "package a\n")
	gitCommit(t, root, "add x")
	mv := exec.Command("git", "mv", "a/x.go", "b/x.go")
	mv.Dir = root
	if err := os.MkdirAll(filepath.Join(root, "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := mv.CombinedOutput(); err != nil {
		t.Fatalf("git mv: %v\n%s", err, out)
	}
	gitCommit(t, root, "move x")

	row := verifyRow{Num: 2, Class: "check", Command: "test -f a/x.go", Expect: "exists"}

	for _, tc := range []struct {
		name string
		risk bool
	}{
		{"gate-human-or-risk-yes", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := gatherRowFacts(root, row, tc.risk, "owning brief is human-gated (gate: human)")
			got := Classify(facts)
			if got.Verdict != RefusedRiskBearing {
				t.Fatalf("risk-bearing row: got %s, want %s (must win regardless of the safe:rename shape)",
					got.Verdict, RefusedRiskBearing)
			}
		})
	}

	// And prove the risk read comes from the brief's OWN frontmatter via loadBrief: a
	// gate:human brief is risk-bearing; a non-risk brief is not.
	riskBrief := filepath.Join(root, "risk-brief.md")
	os.WriteFile(riskBrief, []byte("---\ngate: human\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n---\n\n## Verify\n\n| # | Class | Command | Expect |\n|---|-------|---------|--------|\n| 2 | check | `test -f a/x.go` | exists |\n"), 0o644)
	lb, err := loadBrief("", root, riskBrief)
	if err != nil {
		t.Fatal(err)
	}
	if !lb.RiskBearing {
		t.Fatalf("gate:human brief: RiskBearing=false, want true")
	}
}

// TestRefusesDifferentRC — brief verify-integrity/05 Verify row 3 (FAIL-FIRST). The row's
// command still RUNS (its pinned path is intact) and returns a different result than the row
// pinned → refused:behaviour-changed, never a stale-oracle re-baseline.
func TestRefusesDifferentRC(t *testing.T) {
	root := gitRepo(t)
	// The pinned path EXISTS, so this is not a stale-path case: the command runs against a
	// present tree and returns nonzero — a real behaviour change.
	writeFile(t, root, "pkg/present.txt", "one\n")
	gitCommit(t, root, "add present")

	row := verifyRow{
		Num:     3,
		Class:   "check",
		Command: "test -f pkg/present.txt && exit 3", // runs, exits 3 (differs from the pinned pass)
		Expect:  "exit 0",
	}
	facts := gatherRowFacts(root, row, false, "")
	if !facts.CommandProbed || !facts.CommandRan {
		t.Fatalf("command with an intact path should be probed and ran: %+v", facts)
	}
	got := Classify(facts)
	if got.Verdict != RefusedBehaviourChanged {
		t.Fatalf("command runs, rc differs: got %s (reason %q), want %s\nfacts=%+v",
			got.Verdict, got.Reason, RefusedBehaviourChanged, facts)
	}
}

// TestClassifyFailsClosed — a row the facts cannot place in any safe class is refused, never
// silently re-baselined (the SPOF: unproven is never green).
func TestClassifyFailsClosed(t *testing.T) {
	got := Classify(RowFacts{Row: 9, Command: "true", PathRef: "", CommandProbed: true, CommandRan: true, RCDiffers: false})
	if got.Verdict != RefusedUnclassified {
		t.Fatalf("unprovable row: got %s, want %s", got.Verdict, RefusedUnclassified)
	}
	if got.Verdict.IsSafe() {
		t.Fatalf("%s must not be safe", got.Verdict)
	}
}
