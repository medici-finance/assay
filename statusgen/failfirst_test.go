package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Tests for verify-integrity/03's fail-first audit: `verifyrun --fail-first`
// runs each risk-bearing row at the merge-base and at head, and the closure
// gate refuses a post-base closure whose risk-bearing row is non-discriminating
// (green at base AND at head), has no fail-first witness, or names a base that
// is not an ancestor of HEAD.

// ffBrief renders a verified brief with one risk-bearing row (#1, tagged
// `live`) and one plain row (#2), plus the given Evidence body.
func ffBrief(evidence string) string {
	return "---\nbrief: ff/01\ntitle: Fail-first fixture\nwave: 0\ndepends: []\nunblocks: []\neffort: S\ngate: model\n" +
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\nissues: []\nschema: brief-v1\n" +
		"authored: 2026-10-08 by fixture\nsources: [\"fixture\"]\n---\n\n# Fail-first fixture\n\n" +
		"## Verify (executable — no prose-only DoD items)\n" +
		"| # | Command | Expect |\n|---|---------|--------|\n" +
		"| 1 | `grep -q new flag.txt` | exit 0 (live check) |\n" +
		"| 2 | `test -f flag.txt` | exit 0 |\n\n" +
		"## Evidence\n\n" + evidence + "\n\n## Review\nGate: model.\n"
}

// ffStream writes the ff stream (brief 01 verified) under root and loads it.
func ffStream(t *testing.T, root, evidence string) []*Stream {
	t.Helper()
	dir := filepath.Join(root, "docs", "streams", "ff")
	mustMkdirAll(t, dir)
	writeTemp(t, dir, "README.md", "---\nstream: ff\nstatus: active\npriority: P2\ntrack: platform\n---\n\n# ff\n\n"+
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n"+
		"|---|-------|------|--------|--------|----------|----------|\n"+
		"| 01 | [FF](./brief-01-ff.md) | 0 | S | verified | 2026-10-08 | model:x |\n")
	writeTemp(t, dir, "brief-01-ff.md", ffBrief(evidence))
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	return streams
}

// ffTable renders a witness table for rows 1 and 2 with the given Base cells
// ("" = an ordinary witness, no fail-first run).
func ffTable(base1, base2 string) string {
	mk := func(id, cmd, base string) witness {
		return witness{ID: id, Command: cmd, State: statePass, Exit: 0, OutHash: "0123456789ab",
			Date: "2026-10-08", Runner: "human:fixture", Tree: "aaaaaaaaaaaa", Base: base}
	}
	return witnessTable([]witness{mk("1", "grep -q new flag.txt", base1), mk("2", "test -f flag.txt", base2)})
}

// withAncestry swaps the base-ancestry probe for the duration of a test.
func withAncestry(t *testing.T, fn func(root, base string) (bool, string)) {
	t.Helper()
	prev := commitIsAncestor
	commitIsAncestor = fn
	t.Cleanup(func() { commitIsAncestor = prev })
}

// TestFailFirstNonDiscriminating — Verify row 1. A risk-bearing row green at
// base and green at head is `non-discriminating`, a closure PROBLEM naming the
// row; perturb the base so the row reds and the closure is clean.
func TestFailFirstNonDiscriminating(t *testing.T) {
	withBase(t, true)
	withAncestry(t, func(string, string) (bool, string) { return true, "" })

	green := failFirstCell("bbbbbbbbbbbb", baseGreen, 0, "aaaaaaaaaaaa")
	red := failFirstCell("bbbbbbbbbbbb", baseRed, 1, "aaaaaaaaaaaa")
	skip := failFirstCell("", baseNotSelected, -1, "aaaaaaaaaaaa")

	root := t.TempDir()
	problems, _ := failFirstGateChecks(root, ffStream(t, root, ffTable(green, skip)))
	if !containsSub(problems, "non-discriminating") || !containsSub(problems, "#1") {
		t.Fatalf("green-at-base + pass-at-head on a risk-bearing row must be a non-discriminating PROBLEM naming #1; got %v", problems)
	}

	root = t.TempDir()
	problems, _ = failFirstGateChecks(root, ffStream(t, root, ffTable(red, skip)))
	if len(problems) != 0 {
		t.Fatalf("a row that reds at base and passes at head discriminates — the closure is clean; got %v", problems)
	}

	// No fail-first witness at all for the risk-bearing row.
	root = t.TempDir()
	problems, _ = failFirstGateChecks(root, ffStream(t, root, ffTable("", "")))
	if !containsSub(problems, "no fail-first witness") || !containsSub(problems, "#1") {
		t.Fatalf("a post-base closure with no fail-first witness for a risk-bearing row is a PROBLEM; got %v", problems)
	}

	// Non-risk-bearing rows are audited, never a PROBLEM on non-discrimination.
	root = t.TempDir()
	problems, notices := failFirstGateChecks(root, ffStream(t, root, ffTable(red, green)))
	if len(problems) != 0 {
		t.Fatalf("a non-discriminating NON-risk row must not be a PROBLEM; got %v", problems)
	}
	if !containsSub(notices, "#2") {
		t.Errorf("a non-discriminating non-risk row is still audited (NOTICE); got %v", notices)
	}

	// A closure already at the merge-base is grandfathered.
	withBase(t, true, "ff/01")
	root = t.TempDir()
	problems, _ = failFirstGateChecks(root, ffStream(t, root, ffTable(green, skip)))
	if len(problems) != 0 {
		t.Fatalf("a closure at the merge-base is grandfathered; got %v", problems)
	}
}

func gitOutT(t *testing.T, root string, args ...string) string {
	t.Helper()
	full := append([]string{"-C", root, "-c", "commit.gpgsign=false"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// ffRepo builds a two-commit repo — base (flag.txt "old") then head (flag.txt
// "new") — plus one dangling commit that is NOT an ancestor of HEAD.
func ffRepo(t *testing.T) (root, base, head, stray string) {
	t.Helper()
	root = t.TempDir()
	gitRun(t, root, "init", "-q")
	writeTemp(t, root, "flag.txt", "old\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "base")
	base = gitOutT(t, root, "rev-parse", "HEAD")
	writeTemp(t, root, "flag.txt", "new\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "head")
	head = gitOutT(t, root, "rev-parse", "HEAD")
	tree := gitOutT(t, root, "rev-parse", "HEAD^{tree}")
	stray = gitOutT(t, root, "commit-tree", tree, "-m", "a sibling tree outside this history")
	return root, base, head, stray
}

// TestFailFirstBaseAncestry — Verify row 2. A fail-first witness whose base SHA
// is not an ancestor of HEAD (a stale sibling tree, or an object this clone does
// not have) is a PROBLEM "base is not an ancestor" — never a pass.
func TestFailFirstBaseAncestry(t *testing.T) {
	withBase(t, true)
	root, base, head, stray := ffRepo(t)
	skip := failFirstCell("", baseNotSelected, -1, head[:12])

	for _, c := range []struct {
		name, base  string
		wantProblem bool
	}{
		{"real merge-base", base[:12], false},
		{"sibling tree outside this history", stray[:12], true},
		{"object this clone does not have", "deadbeefdead", true},
	} {
		evidence := ffTable(failFirstCell(c.base, baseRed, 1, head[:12]), skip)
		problems, _ := failFirstGateChecks(root, ffStream(t, root, evidence))
		got := containsSub(problems, "base is not an ancestor")
		if got != c.wantProblem {
			t.Errorf("%s: base %s — ancestry PROBLEM=%v, want %v; problems=%v", c.name, c.base, got, c.wantProblem, problems)
		}
	}
}

// TestFailFirstRunsAtBase — the run half: the base run executes in a temp
// worktree checked out at the base, records red/green per risk-bearing row,
// leaves non-risk rows not-selected, and removes the worktree afterwards.
func TestFailFirstRunsAtBase(t *testing.T) {
	root, base, _, _ := ffRepo(t)
	rows := []verifyRow{
		{ID: "1", Command: "grep -q new flag.txt", Expect: "exit 0"},
		{ID: "2", Command: "test -f flag.txt", Expect: "exit 0"},
		{ID: "3", Command: "test -f flag.txt", Expect: "exit 0"},
	}
	risky := map[string]bool{"1": true, "2": true}
	got, err := runFailFirstBase(resolveShellPlan(), root, base, rows, risky, time.Minute)
	if err != nil {
		t.Fatalf("runFailFirstBase: %v", err)
	}
	if got["1"].State != baseRed {
		t.Errorf("row 1 checks for the head-only content: must be red at base; got %+v", got["1"])
	}
	if got["2"].State != baseGreen {
		t.Errorf("row 2 passes on both trees: must be green at base; got %+v", got["2"])
	}
	if got["3"].State != baseNotSelected {
		t.Errorf("a non-risk row is not run at base; got %+v", got["3"])
	}
	if wt := gitOutT(t, root, "worktree", "list", "--porcelain"); strings.Count(wt, "worktree ") != 1 {
		t.Errorf("the base worktree must be removed after the run; worktree list:\n%s", wt)
	}

	// The rendered row keeps every positional witness cell and adds Base.
	w := witness{ID: "1", Command: "grep -q new flag.txt", State: statePass, Exit: 0, OutHash: "0123456789ab",
		Date: "2026-10-08", Runner: "human:fixture", Tree: "aaaaaaaaaaaa",
		Base: failFirstCell(base[:12], got["1"].State, got["1"].Exit, "aaaaaaaaaaaa")}
	row := w.row()
	if witnessStateOf(row) != statePass || witnessTreeOf(row) != "aaaaaaaaaaaa" {
		t.Errorf("the Base column must not disturb the positional witness cells: %s", row)
	}
	ff, ok := failFirstOf(row)
	if !ok || ff.Base != base[:12] || ff.State != baseRed {
		t.Errorf("failFirstOf must read back the base and its state; got %+v ok=%v from %s", ff, ok, row)
	}
	if unrunMarkerRe.MatchString(inlineCodeRe.ReplaceAllString(row, "")) {
		t.Errorf("a fail-first row must not carry an UNRUN marker word: %s", row)
	}
}
