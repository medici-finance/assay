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
// is not the merge-base or an ancestor of it.

// ffBrief renders a verified brief with one risk-bearing row (#1, tagged
// `live`) and one plain row (#2), plus the given Evidence body.
func ffBrief(evidence string) string {
	return ffBriefRaw("no",
		"| # | Command | Expect |\n|---|---------|--------|\n"+
			"| 1 | `grep -q new flag.txt` | exit 0 (live check) |\n"+
			"| 2 | `test -f flag.txt` | exit 0 |\n", evidence)
}

// ffBriefRaw renders the same brief with an arbitrary Verify body and a
// brief-wide risk answer (every risk key set to it).
func ffBriefRaw(risk, verify, evidence string) string {
	return "---\nbrief: ff/01\ntitle: Fail-first fixture\nwave: 0\ndepends: []\nunblocks: []\neffort: S\ngate: model\n" +
		"risk: {regulatory: " + risk + ", customer: no, irreversible: no, sensitive-data: no}\nissues: []\nschema: brief-v1\n" +
		"authored: 2026-10-08 by fixture\nsources: [\"fixture\"]\n---\n\n# Fail-first fixture\n\n" +
		"## Verify (executable — no prose-only DoD items)\n" + verify + "\n" +
		"## Evidence\n\n" + evidence + "\n\n## Review\nGate: model.\n"
}

// ffStream writes the ff stream (brief 01 verified) under root and loads it.
func ffStream(t *testing.T, root, evidence string) []*Stream {
	t.Helper()
	return ffStreamBrief(t, root, ffBrief(evidence))
}

// ffStreamBrief writes the ff stream with the given brief-01 text and loads it.
func ffStreamBrief(t *testing.T, root, brief string) []*Stream {
	t.Helper()
	dir := filepath.Join(root, "docs", "streams", "ff")
	mustMkdirAll(t, dir)
	writeTemp(t, dir, "README.md", "---\nstream: ff\nstatus: active\npriority: P2\ntrack: platform\n---\n\n# ff\n\n"+
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n"+
		"|---|-------|------|--------|--------|----------|----------|\n"+
		"| 01 | [FF](./brief-01-ff.md) | 0 | S | verified | 2026-10-08 | model:x |\n")
	writeTemp(t, dir, "brief-01-ff.md", brief)
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

// withAncestry swaps the base-ancestry probe and the merge-base resolver for
// the duration of a test.
func withAncestry(t *testing.T, fn func(root, base, of string) (bool, string)) {
	t.Helper()
	prev, prevMB := commitIsAncestor, failFirstMergeBase
	commitIsAncestor = fn
	failFirstMergeBase = func(string) string { return strings.Repeat("a", 40) }
	// The brief-held rule is exercised by TestFailFirstBaseMustHoldBrief and
	// TestFailFirstBriefHeldRealGit; here every tree holds the brief.
	prevHeld := pathHeldAt
	pathHeldAt = func(_, _, _ string) bool { return true }
	t.Cleanup(func() { commitIsAncestor, failFirstMergeBase, pathHeldAt = prev, prevMB, prevHeld })
}

// TestFailFirstNonDiscriminating — Verify row 1. A risk-bearing row green at
// base and green at head is `non-discriminating`, a closure PROBLEM naming the
// row; perturb the base so the row reds and the closure is clean.
func TestFailFirstNonDiscriminating(t *testing.T) {
	withBase(t, true)
	withAncestry(t, func(string, string, string) (bool, string) { return true, "" })

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

// ffRepo builds a three-commit repo — old (flag.txt "old"), base (the merge-base
// stand-in, flag.txt "old") and head (flag.txt "new") — plus one dangling commit
// that is NOT an ancestor of HEAD and one commit of the branch between base and
// head. It stubs the merge-base resolver to return base.
func ffRepo(t *testing.T) (root, older, base, branch, head, stray string) {
	t.Helper()
	root = t.TempDir()
	gitRun(t, root, "init", "-q")
	writeTemp(t, root, "other.txt", "x\n")
	writeTemp(t, root, "flag.txt", "old\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "older")
	older = gitOutT(t, root, "rev-parse", "HEAD")
	writeTemp(t, root, "other.txt", "y\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "base")
	base = gitOutT(t, root, "rev-parse", "HEAD")
	writeTemp(t, root, "other.txt", "z\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "branch commit")
	branch = gitOutT(t, root, "rev-parse", "HEAD")
	writeTemp(t, root, "flag.txt", "new\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "head")
	head = gitOutT(t, root, "rev-parse", "HEAD")
	tree := gitOutT(t, root, "rev-parse", "HEAD^{tree}")
	stray = gitOutT(t, root, "commit-tree", tree, "-m", "a sibling tree outside this history")
	prev := failFirstMergeBase
	failFirstMergeBase = func(string) string { return base }
	t.Cleanup(func() { failFirstMergeBase = prev })
	return root, older, base, branch, head, stray
}

// TestFailFirstBaseAncestry — Verify row 2. A fail-first witness is accepted
// only when its base is the merge-base of HEAD and the default branch or an
// ancestor of it. HEAD's own commit, a commit of the branch, a sibling tree and
// an object this clone does not have are each a PROBLEM "base is not the
// merge-base" — never a pass.
func TestFailFirstBaseAncestry(t *testing.T) {
	withBase(t, true)
	root, older, base, branch, head, stray := ffRepo(t)
	skip := failFirstCell("", baseNotSelected, -1, head[:12])

	for _, c := range []struct {
		name, base  string
		wantProblem bool
	}{
		{"the merge-base itself", base[:12], false},
		{"an ancestor of the merge-base (post-merge verify: the delivering change's parent)", older[:12], false},
		{"HEAD's own commit", head[:12], true},
		{"a commit of the branch", branch[:12], true},
		{"sibling tree outside this history", stray[:12], true},
		{"object this clone does not have", "deadbeefdead", true},
	} {
		evidence := ffTable(failFirstCell(c.base, baseRed, 1, head[:12]), skip)
		problems, _ := failFirstGateChecks(root, ffStream(t, root, evidence))
		got := containsSub(problems, "not the merge-base or an ancestor of it")
		if got != c.wantProblem {
			t.Errorf("%s: base %s — base PROBLEM=%v, want %v; problems=%v", c.name, c.base, got, c.wantProblem, problems)
		}
	}
}

// TestFailFirstBaseFlagRefusesNonAncestor — the runner refuses at the source
// what the gate would refuse later: --base HEAD records a red that proves
// nothing about the change.
func TestFailFirstBaseFlagRefusesNonAncestor(t *testing.T) {
	root, older, base, branch, head, _ := ffRepo(t)
	withBriefHeld(t, base, older)
	briefPath := filepath.Join(root, "docs", "streams", "ff", "brief-01-ff.md")
	for _, c := range []struct {
		name, rev string
		ok        bool
	}{{"merge-base", base, true}, {"ancestor of the merge-base", older, true}, {"branch commit", branch, false}, {"HEAD", head, false}} {
		_, err := failFirstBaseRev(root, c.rev, briefPath)
		if (err == nil) != c.ok {
			t.Errorf("--base %s: err=%v, want ok=%v", c.name, err, c.ok)
		}
	}
}

// TestFailFirstRecordBoundToLatestWitnessAndCommand — a fail-first record made
// for an earlier command, or followed by a newer ordinary witness, does not
// satisfy the closure gate; the same record with nothing after it does.
func TestFailFirstRecordBoundToLatestWitnessAndCommand(t *testing.T) {
	withBase(t, true)
	withAncestry(t, func(string, string, string) (bool, string) { return true, "" })
	red := failFirstCell("bbbbbbbbbbbb", baseRed, 1, "aaaaaaaaaaaa")
	skip := failFirstCell("", baseNotSelected, -1, "aaaaaaaaaaaa")
	row := func(id, cmd, base string) witness {
		return witness{ID: id, Command: cmd, State: statePass, Exit: 0, OutHash: "0123456789ab",
			Date: "2026-10-08", Runner: "human:fixture", Tree: "aaaaaaaaaaaa", Base: base}
	}
	good := ffTable(red, skip)

	root := t.TempDir()
	if problems, _ := failFirstGateChecks(root, ffStream(t, root, good)); len(problems) != 0 {
		t.Fatalf("control: a current fail-first record is clean; got %v", problems)
	}

	// The row's command is replaced by one that always passes; the old red
	// record stays and a newer ordinary witness for the NEW command follows.
	rewritten := ffBriefRaw("no",
		"| # | Command | Expect |\n|---|---------|--------|\n"+
			"| 1 | `true` | exit 0 (live check) |\n"+
			"| 2 | `test -f flag.txt` | exit 0 |\n",
		good+"\n\n"+witnessTable([]witness{row("1", "true", "")}))
	root = t.TempDir()
	problems, _ := failFirstGateChecks(root, ffStreamBrief(t, root, rewritten))
	if !containsSub(problems, "#1") || !containsSub(problems, "no fail-first witness") {
		t.Fatalf("an old red record behind a newer plain witness must not satisfy the gate; got %v", problems)
	}

	// The command is replaced and no newer witness exists: the record is for a
	// command the row no longer holds.
	stale := ffBriefRaw("no",
		"| # | Command | Expect |\n|---|---------|--------|\n"+
			"| 1 | `true` | exit 0 (live check) |\n"+
			"| 2 | `test -f flag.txt` | exit 0 |\n", good)
	root = t.TempDir()
	problems, _ = failFirstGateChecks(root, ffStreamBrief(t, root, stale))
	if !containsSub(problems, "different command") || !containsSub(problems, "#1") {
		t.Fatalf("a fail-first record for a replaced command must be a PROBLEM; got %v", problems)
	}

	// A later fail-first run for the new command is accepted.
	rerun := ffBriefRaw("no",
		"| # | Command | Expect |\n|---|---------|--------|\n"+
			"| 1 | `true` | exit 0 (live check) |\n"+
			"| 2 | `test -f flag.txt` | exit 0 |\n",
		good+"\n\n"+witnessTable([]witness{row("1", "true", red), row("2", "test -f flag.txt", skip)}))
	root = t.TempDir()
	if problems, _ := failFirstGateChecks(root, ffStreamBrief(t, root, rerun)); len(problems) != 0 {
		t.Fatalf("a fresh fail-first record for the current command is clean; got %v", problems)
	}
}

// TestFailFirstGateSeesEveryRunnerRow — the gate, the risk selection and the
// runner enumerate Verify rows with ONE parser. A short row (no Expect cell), a
// table headed `Expected`, and a table the runner cannot place are each held.
func TestFailFirstGateSeesEveryRunnerRow(t *testing.T) {
	withBase(t, true)
	withAncestry(t, func(string, string, string) (bool, string) { return true, "" })
	red := failFirstCell("bbbbbbbbbbbb", baseRed, 1, "aaaaaaaaaaaa")
	plain := witnessTable([]witness{
		{ID: "1", Command: "grep -q new flag.txt", State: statePass, Exit: 0, OutHash: "0123456789ab", Date: "2026-10-08", Runner: "human:fixture", Tree: "aaaaaaaaaaaa", Base: red},
		{ID: "2", Command: "test -f flag.txt", State: statePass, Exit: 0, OutHash: "0123456789ab", Date: "2026-10-08", Runner: "human:fixture", Tree: "aaaaaaaaaaaa"},
	})

	for _, c := range []struct{ name, verify string }{
		{"a short row", "| # | Command | Expect |\n|---|---------|--------|\n| 1 | `grep -q new flag.txt` | exit 0 |\n| 2 | `test -f flag.txt` |\n"},
		{"a table headed Expected", "| # | Command | Expected |\n|---|---------|----------|\n| 1 | `grep -q new flag.txt` | exit 0 |\n| 2 | `test -f flag.txt` | exit 0 |\n"},
	} {
		// Brief-wide risk yes: every row the runner executes is risk-bearing.
		root := t.TempDir()
		problems, _ := failFirstGateChecks(root, ffStreamBrief(t, root, ffBriefRaw("yes", c.verify, plain)))
		if !containsSub(problems, "#2") || !containsSub(problems, "no fail-first witness") {
			t.Errorf("%s: row 2 is run and witnessed by the runner, so the gate must hold it; got %v", c.name, problems)
		}
		rows := briefVerifyRows(extractSectionByPrefix(ffBriefRaw("yes", c.verify, plain), "Verify"))
		if got := failFirstRiskyIDs(map[string]string{"regulatory": "yes"}, rows); !got["1"] || !got["2"] {
			t.Errorf("%s: the risk selection must see the runner's rows; got %v", c.name, got)
		}
	}

	// A table with no Command column is never run: a PROBLEM, not a skip.
	unplaced := "| # | Cmd | Expect |\n|---|-----|--------|\n| 1 | `true` | exit 0 |\n"
	root := t.TempDir()
	problems, _ := failFirstGateChecks(root, ffStreamBrief(t, root, ffBriefRaw("yes", unplaced, plain)))
	if !containsSub(problems, "cannot place") {
		t.Errorf("a Verify table the runner cannot place must be a PROBLEM on a closure; got %v", problems)
	}
}

// TestFailFirstReadersSkipTheBaseColumn — a fail-first witness table appends
// Base to the right of Runner; the readers that take Date and Runner counted
// from the right (the verify flip and the human close stamp) must still read
// them, not the runner text as the date and Base as the runner.
func TestFailFirstReadersSkipTheBaseColumn(t *testing.T) {
	red := failFirstCell("bbbbbbbbbbbb", baseRed, 1, "aaaaaaaaaaaa")
	for _, tbl := range []string{ffTable(red, red), ffTable("", "")} {
		d, r := evidenceVerifierInfo(tbl)
		if d != "2026-10-08" || !strings.HasPrefix(r, "human:fixture") || strings.Contains(r, "base=") {
			t.Errorf("evidenceVerifierInfo = (%q, %q), want (2026-10-08, human:fixture...) from:\n%s", d, r, tbl)
		}
		rows, _ := flipDateRunnerRows(strings.Split(tbl, "\n"))
		if len(rows) == 0 {
			t.Fatalf("flipDateRunnerRows read no rows from:\n%s", tbl)
		}
		for _, dr := range rows {
			if dr[0] != "2026-10-08" || !strings.HasPrefix(dr[1], "human:fixture") || strings.Contains(dr[1], "base=") {
				t.Errorf("flipDateRunnerRows = %v, want [2026-10-08 human:fixture...]", dr)
			}
		}
	}
}

// TestFailFirstRunsAtBase — the run half: the base run executes in a temp
// worktree checked out at the base, records red/green per risk-bearing row,
// leaves non-risk rows not-selected, and removes the worktree afterwards.
func TestFailFirstRunsAtBase(t *testing.T) {
	root, _, base, _, _, _ := ffRepo(t)
	rows := []verifyRow{
		{ID: "1", Command: "grep -q new flag.txt", Expect: "exit 0"},
		{ID: "2", Command: "test -f flag.txt", Expect: "exit 0"},
		{ID: "3", Command: "test -f flag.txt", Expect: "exit 0"},
	}
	risky := map[string]bool{"1": true, "2": true}
	got, err := runFailFirstBase(resolveShellPlan(), sandboxUnshare, root, base, rows, risky, time.Minute, false)
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

	// The CI flag reaches the base run: an explicitly-classed env-bound `check`
	// row that --ci skips at head is skipped at base, never executed there.
	ciRows := []verifyRow{{ID: "1", Command: "touch ran-at-base.txt", Expect: "exit 0", Class: "check", Classed: true}}
	got2, err := runFailFirstBase(resolveShellPlan(), sandboxUnshare, root, base, ciRows, map[string]bool{"1": true}, time.Minute, true)
	if err != nil {
		t.Fatalf("runFailFirstBase (ci): %v", err)
	}
	if got2["1"].State != baseNotSelected {
		t.Errorf("a row --ci skips at head must not run at base; got %+v", got2["1"])
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

// withBriefHeld stands in for `git cat-file -e <rev>:<path>`: held names the
// revisions (by prefix) whose tree holds the brief file.
func withBriefHeld(t *testing.T, held ...string) {
	t.Helper()
	prev := pathHeldAt
	pathHeldAt = func(_, rev, _ string) bool {
		for _, h := range held {
			if strings.HasPrefix(rev, h) || strings.HasPrefix(h, rev) {
				return true
			}
		}
		return false
	}
	t.Cleanup(func() { pathHeldAt = prev })
}

// TestFailFirstBaseMustHoldBrief — F3 (a). The ancestry rule alone accepts any
// ancestor of the merge-base, the repository's root commit included, where every
// row reds because the files are missing, not because of any change. A base that
// does not hold the brief file while the merge-base does is refused by the gate
// and by the runner; a brief born on the branch (the merge-base lacks it too)
// accepts only the merge-base itself.
func TestFailFirstBaseMustHoldBrief(t *testing.T) {
	withBase(t, true)
	root, older, base, _, head, _ := ffRepo(t)
	skip := failFirstCell("", baseNotSelected, -1, head[:12])
	briefPath := filepath.Join(root, "docs", "streams", "ff", "brief-01-ff.md")

	for _, c := range []struct {
		name        string
		held        []string
		baseRev     string
		wantProblem bool
	}{
		{"older ancestor predates the brief", []string{base}, older, true},
		{"older ancestor holds the brief", []string{base, older}, older, false},
		{"the merge-base holds the brief", []string{base}, base, false},
		{"brief born on the branch: an older ancestor is refused", nil, older, true},
		{"brief born on the branch: the merge-base itself is the one base accepted", nil, base, false},
	} {
		withBriefHeld(t, c.held...)
		evidence := ffTable(failFirstCell(c.baseRev[:12], baseRed, 1, head[:12]), skip)
		problems, _ := failFirstGateChecks(root, ffStream(t, root, evidence))
		if got := containsSub(problems, "predates the brief") || containsSub(problems, "cannot be shown to hold the brief"); got != c.wantProblem {
			t.Errorf("gate, %s: PROBLEM=%v, want %v; problems=%v", c.name, got, c.wantProblem, problems)
		}
		_, err := failFirstBaseRev(root, c.baseRev, briefPath)
		if (err != nil) != c.wantProblem {
			t.Errorf("runner --base, %s: err=%v, want refusal=%v", c.name, err, c.wantProblem)
		}
	}
}

// TestFailFirstBriefHeldRealGit — the review's three scenarios against a real
// repository, no stubbed probe: the root commit has no brief, the next commit
// (the merge-base) adds it, and the branch then (1) leaves it at the same path,
// (2) renames it, (3) holds a brief created on the branch. With the root commit
// as base, (1) was always refused; (2) and (3) used to be accepted. Each must be
// refused now, and the merge-base itself stays accepted in every one.
func TestFailFirstBriefHeldRealGit(t *testing.T) {
	const rel = "docs/streams/ff/brief-01-ff.md"
	const renamed = "docs/streams/ff/brief-02-ff.md"
	setup := func(t *testing.T, briefAtMergeBase bool) (root, rootCommit, mergeBase string) {
		t.Helper()
		root = t.TempDir()
		gitRun(t, root, "init", "-q")
		writeTemp(t, root, "flag.txt", "old\n")
		gitRun(t, root, "add", "-A")
		gitRun(t, root, "commit", "-q", "-m", "root")
		rootCommit = gitOutT(t, root, "rev-parse", "HEAD")
		if briefAtMergeBase {
			mustMkdirAll(t, filepath.Join(root, "docs", "streams", "ff"))
			writeTemp(t, filepath.Join(root, "docs", "streams", "ff"), "brief-01-ff.md", "# brief\n")
		} else {
			writeTemp(t, root, "other.txt", "x\n")
		}
		gitRun(t, root, "add", "-A")
		gitRun(t, root, "commit", "-q", "-m", "merge-base")
		mergeBase = gitOutT(t, root, "rev-parse", "HEAD")
		return root, rootCommit, mergeBase
	}
	for _, c := range []struct {
		name         string
		briefAtMB    bool
		onBranch     func(t *testing.T, root string)
		finalRel     string
		wantRootBase bool // is the root commit accepted as the base
	}{
		{"same path: refused", true, func(t *testing.T, root string) {
			writeTemp(t, filepath.Join(root, "docs", "streams", "ff"), "brief-01-ff.md", "# brief edited\n")
		}, rel, false},
		{"renamed on the branch: refused", true, func(t *testing.T, root string) {
			gitRun(t, root, "mv", rel, renamed)
		}, renamed, false},
		{"created on the branch: refused", false, func(t *testing.T, root string) {
			mustMkdirAll(t, filepath.Join(root, "docs", "streams", "ff"))
			writeTemp(t, filepath.Join(root, "docs", "streams", "ff"), "brief-01-ff.md", "# brief\n")
		}, rel, false},
	} {
		root, rootCommit, mergeBase := setup(t, c.briefAtMB)
		c.onBranch(t, root)
		gitRun(t, root, "add", "-A")
		gitRun(t, root, "commit", "-q", "-m", "branch")
		briefPath := filepath.Join(root, filepath.FromSlash(c.finalRel))

		if why := briefHeldAtBase(root, briefPath, rootCommit, mergeBase); (why == "") != c.wantRootBase {
			t.Errorf("%s: briefHeldAtBase(root commit) = %q, want accepted=%v", c.name, why, c.wantRootBase)
		}
		if why := briefHeldAtBase(root, briefPath, mergeBase, mergeBase); why != "" {
			t.Errorf("%s: the merge-base itself must stay accepted; got %q", c.name, why)
		}
		if why := briefHeldAtBase(root, briefPath, mergeBase[:12], mergeBase); why != "" {
			t.Errorf("%s: an abbreviated merge-base must stay accepted; got %q", c.name, why)
		}
		prev := failFirstMergeBase
		failFirstMergeBase = func(string) string { return mergeBase }
		_, err := failFirstBaseRev(root, rootCommit, briefPath)
		if (err == nil) != c.wantRootBase {
			t.Errorf("%s: runner --base <root commit>: err=%v, want accepted=%v", c.name, err, c.wantRootBase)
		}
		if _, err := failFirstBaseRev(root, mergeBase, briefPath); err != nil {
			t.Errorf("%s: runner --base <merge-base> must be accepted; got %v", c.name, err)
		}
		failFirstMergeBase = prev
	}

	// Fail closed: a brief path outside the clone, or an empty one, cannot be
	// resolved, so only the merge-base itself passes.
	root, rootCommit, mergeBase := setup(t, true)
	for _, bp := range []string{"", filepath.Join(t.TempDir(), "brief-01-ff.md")} {
		if why := briefHeldAtBase(root, bp, rootCommit, mergeBase); why == "" {
			t.Errorf("an unresolvable brief path %q must refuse an ancestor base", bp)
		}
		if why := briefHeldAtBase(root, bp, mergeBase, mergeBase); why != "" {
			t.Errorf("an unresolvable brief path %q still accepts the merge-base itself; got %q", bp, why)
		}
	}
}

// TestVerifyrunFailFirstRefusesAncestorBaseOfBranchBornBrief — the runner
// wiring, end to end through `verifyrun --fail-first --base`: a brief created on
// the branch refuses the root commit as the base before any row runs, and the
// merge-base itself is not refused.
func TestVerifyrunFailFirstRefusesAncestorBaseOfBranchBornBrief(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root, "t", "t@example.com")
	writeTemp(t, root, "flag.txt", "old\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "root")
	rootCommit := gitOutT(t, root, "rev-parse", "HEAD")
	writeTemp(t, root, "other.txt", "x\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "merge-base")
	mergeBase := gitOutT(t, root, "rev-parse", "HEAD")
	writeTemp(t, root, "flag.txt", "new\n")
	verify := "| # | Command | Expect |\n|---|---------|--------|\n| 1 | `grep -q new flag.txt` | exit 0 |\n"
	ffStreamBrief(t, root, ffBriefRaw("yes", verify, ""))
	brief := filepath.Join(root, "docs", "streams", "ff", "brief-01-ff.md")
	prev := failFirstMergeBase
	failFirstMergeBase = func(string) string { return mergeBase }
	t.Cleanup(func() { failFirstMergeBase = prev })

	res, stderr := captureVerifyrun(t, []string{"--fail-first", "--dry-run", "--root", root, "--base", rootCommit, "--brief", brief})
	if res.code != verifyrunExitCouldNot || !strings.Contains(stderr, "cannot be shown to hold the brief") {
		t.Errorf("--base <root commit> for a brief born on the branch must be refused; code=%d stderr=%s", res.code, stderr)
	}
	_, stderr = captureVerifyrun(t, []string{"--fail-first", "--dry-run", "--root", root, "--base", mergeBase, "--brief", brief})
	if strings.Contains(stderr, "cannot be shown to hold the brief") || strings.Contains(stderr, "predates the brief") {
		t.Errorf("--base <merge-base> must not be refused on the brief-held rule; stderr=%s", stderr)
	}
}

// TestPathHeldAtReadsTheTree — the default probe asks git: a file in the
// tree is held, a file added later (or never) is not.
func TestPathHeldAtReadsTheTree(t *testing.T) {
	root, older, _, _, _, _ := ffRepo(t)
	if !pathHeldAt(root, older, "flag.txt") {
		t.Errorf("flag.txt is in the older commit's tree")
	}
	if pathHeldAt(root, older, "docs/streams/ff/brief-01-ff.md") {
		t.Errorf("a file no commit holds must not read as held")
	}
}

// TestVerifyUnplacedRowsOnlyVerifyShaped — F5. The author-brief skill sanctions
// a `Failure mode of the work | Caught by` map beside the Verify table, and
// briefs keep `Row | The mutation that must redden it` maps and legends there.
// None is a Verify table, so none is reported. What stays caught: a row-shaped
// orphan (a prose paragraph split the table, so a numbered row lost its header),
// a Verify-shaped table headed `Expect` with no Command column, and a row too
// short to reach Command.
func TestVerifyUnplacedRowsOnlyVerifyShaped(t *testing.T) {
	table := "| # | Class | Command | Expect |\n|---|---|---|---|\n| 1 | check | `true` | exit 0 |\n"
	for _, c := range []struct {
		name, extra string
		want        int
	}{
		{"B5 failure-mode map", "\n| Failure mode of the work | Caught by |\n|---|---|\n| the flag is ignored | row 1 |\n", 0},
		{"mutation map", "\n| Row | The mutation that must redden it |\n|---|---|\n| 1 | delete the flag |\n", 0},
		{"legend", "\n| Class | Meaning |\n|-------|---------|\n| check | runs in CI |\n", 0},
		{"headerless numbered orphan after prose", "\nA paragraph that splits the table.\n\n| 4 | check:manual | `cat notes.md` | reviewer records agreement |\n", 1},
		{"Verify-shaped table with no Command column", "\n| # | Cmd | Expect |\n|---|-----|--------|\n| 2 | `true` | exit 0 |\n", 1},
		{"row too short to reach Command", "| 2 | check |\n", 1},
	} {
		got := verifyUnplacedRows("\n" + table + c.extra)
		if len(got) != c.want {
			t.Errorf("%s: got %d unplaced (%v), want %d", c.name, len(got), got, c.want)
		}
	}

	// The gate: the sanctioned maps do not block a closure, the orphan does.
	withBase(t, true)
	withAncestry(t, func(string, string, string) (bool, string) { return true, "" })
	red := failFirstCell("bbbbbbbbbbbb", baseRed, 1, "aaaaaaaaaaaa")
	ev := witnessTable([]witness{{ID: "1", Command: "grep -q new flag.txt", State: statePass, Exit: 0, OutHash: "0123456789ab", Date: "2026-10-08", Runner: "human:fixture", Tree: "aaaaaaaaaaaa", Base: red}})
	verify := "| # | Command | Expect |\n|---|---------|--------|\n| 1 | `grep -q new flag.txt` | exit 0 |\n"
	root := t.TempDir()
	problems, _ := failFirstGateChecks(root, ffStreamBrief(t, root, ffBriefRaw("yes", verify+"\n| Failure mode of the work | Caught by |\n|---|---|\n| flag ignored | row 1 |\n", ev)))
	if len(problems) != 0 {
		t.Errorf("a sanctioned Caught-by map must not block a closure; got %v", problems)
	}
	root = t.TempDir()
	problems, _ = failFirstGateChecks(root, ffStreamBrief(t, root, ffBriefRaw("yes", verify+"\nA paragraph.\n\n| 2 | `true` | exit 0 |\n", ev)))
	if !containsSub(problems, "cannot place") {
		t.Errorf("a row-shaped orphan must stay a PROBLEM; got %v", problems)
	}
}
