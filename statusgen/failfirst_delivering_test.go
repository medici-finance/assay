package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ffHermeticGitEnv unsets, for the test, every variable that sends a library
// read to the native profile (nativeGitEnv), so the delivering tests do not
// depend on the shell they run in (a GIT_CONFIG_COUNT set by a CI runner or a
// harness made the delivering arm refuse). Not for parallel tests.
func ffHermeticGitEnv(t *testing.T) {
	t.Helper()
	for _, k := range nativeGitEnv {
		if v, ok := os.LookupEnv(k); ok {
			os.Unsetenv(k)
			t.Cleanup(func() { os.Setenv(k, v) })
		}
	}
}

// ffDeliveredVerify is the risk-bearing row the delivered fixtures run: it
// passes only once the delivering commit has flipped flag.txt to "new".
const ffDeliveredVerify = "| # | Command | Expect |\n|---|---------|--------|\n| 1 | `grep -q new flag.txt` | exit 0 |\n"

// ffDeliveredRepo builds a post-merge history on real git, with origin/main at
// HEAD so the merge-base already holds the change:
//
//	rootC (flag old) → prep (the brief, unless briefInDelivery) → deliv (flag
//	new, and the brief when briefInDelivery) → head (unrelated)
//
// plus side, a child of prep that is not on HEAD's history.
func ffDeliveredRepo(t *testing.T, briefInDelivery bool) (root, rootC, prep, deliv, head, side, briefPath string) {
	t.Helper()
	root = t.TempDir()
	gitInit(t, root, "t", "t@example.com")
	briefDir := filepath.Join(root, "docs", "streams", "ff")
	briefPath = filepath.Join(briefDir, "brief-01-ff.md")
	commit := func(msg string) string {
		gitRun(t, root, "add", "-A")
		gitRun(t, root, "commit", "-q", "-m", msg)
		return gitOutT(t, root, "rev-parse", "HEAD")
	}
	ffHermeticGitEnv(t)
	writeTemp(t, root, "flag.txt", "old\n")
	rootC = commit("root")
	writeTemp(t, root, "other.txt", "x\n")
	if !briefInDelivery {
		mustMkdirAll(t, briefDir)
		writeTemp(t, briefDir, "brief-01-ff.md", ffBriefRaw("yes", ffDeliveredVerify, ""))
	}
	prep = commit("prep")
	writeTemp(t, root, "flag.txt", "new\n")
	if briefInDelivery {
		mustMkdirAll(t, briefDir)
		writeTemp(t, briefDir, "brief-01-ff.md", ffBriefRaw("yes", ffDeliveredVerify, ""))
	}
	deliv = commit("delivering change")
	writeTemp(t, root, "other.txt", "y\n")
	head = commit("later, unrelated")
	gitRun(t, root, "update-ref", remoteMainRef, head)
	gitRun(t, root, "checkout", "-q", "-b", "side", prep)
	writeTemp(t, root, "other.txt", "side\n")
	side = commit("side")
	gitRun(t, root, "checkout", "-q", head)
	return root, rootC, prep, deliv, head, side, briefPath
}

// ffDeliveredCell is a red Base cell for base, recording delivering when set.
func ffDeliveredCell(base, delivering, head string) string {
	c := failFirstCell(base[:12], baseRed, 1, head[:12])
	if delivering != "" {
		c += " · delivering=" + delivering[:12]
	}
	return c
}

// ffDeliveredGate runs the closure gate over one witnessed row with cell.
func ffDeliveredGate(t *testing.T, root, cell string) []string {
	t.Helper()
	ev := witnessTable([]witness{{ID: "1", Command: "grep -q new flag.txt", State: statePass, Exit: 0, OutHash: "0123456789ab", Date: "2026-11-15", Runner: "human:fixture", Tree: "aaaaaaaaaaaa", Base: cell}})
	problems, _ := failFirstGateChecks(root, ffStreamBrief(t, root, ffBriefRaw("yes", ffDeliveredVerify, ev)))
	return problems
}

// TestFailFirstDeliveringBindsBase — medici-finance/assay#2721 option 2. After
// the change has merged the merge-base already holds it, so the verifier names
// the delivering commit and the base is its parent. The runner and the closure
// gate both check, on real git, that the delivering commit is HEAD or an
// ancestor of HEAD and that the recorded base is exactly its first parent; an
// older ancestor with no delivering commit recorded (the retired option-3
// strength) is refused.
// regression: #2721
func TestFailFirstDeliveringBindsBase(t *testing.T) {
	withBase(t, true)
	root, rootC, prep, deliv, head, side, briefPath := ffDeliveredRepo(t, false)
	for _, c := range []struct {
		name, base, deliver, want string // want: a PROBLEM substring, "" for accepted
	}{
		{"the delivering commit's parent", prep, deliv, ""},
		{"the merge-base, which after the merge is HEAD (F4)", head, "", "the base is the head the row ran on"},
		{"an older ancestor, no delivering commit (option 3)", prep, "", "an older base is accepted only as the parent of a recorded --delivering-commit"},
		{"a base that is not the delivering commit's parent", rootC, deliv, "is not the parent of the delivering commit"},
		{"a delivering commit off HEAD's history", prep, side, "is not HEAD or an ancestor of HEAD"},
		{"the root commit as the delivering commit", rootC, rootC, "has no parent"},
	} {
		problems := ffDeliveredGate(t, root, ffDeliveredCell(c.base, c.deliver, head))
		if c.want == "" && len(problems) != 0 || c.want != "" && !containsSub(problems, c.want) {
			t.Errorf("gate, %s: want %q; problems=%v", c.name, c.want, problems)
		}
		_, got, err := failFirstBaseRev(root, c.base, c.deliver, briefPath)
		if (err != nil) != (c.want != "") || err != nil && !strings.Contains(err.Error(), c.want) {
			t.Errorf("runner, %s: err=%v, want %q", c.name, err, c.want)
		}
		if err == nil && got != c.deliver {
			t.Errorf("runner, %s: recorded delivering commit %q, want %q", c.name, got, c.deliver)
		}
	}
	// Without --base the runner derives the base: the delivering commit's parent.
	if base, _, err := failFirstBaseRev(root, "", deliv, briefPath); err != nil || base != prep {
		t.Errorf("--delivering-commit alone: base=%s err=%v, want %s", base, err, prep)
	}
}

// TestFailFirstDeliveringBriefBornInDelivery — the review's F8 case: the brief
// was added by its own delivering change, so the delivering commit's parent
// does not hold it. Recording the delivering commit keeps that brief closable
// after merge; held at neither commit (the root commit named as delivering
// for a later brief) stays refused.
// regression: #2721
func TestFailFirstDeliveringBriefBornInDelivery(t *testing.T) {
	withBase(t, true)
	root, _, prep, deliv, head, _, briefPath := ffDeliveredRepo(t, true)
	if problems := ffDeliveredGate(t, root, ffDeliveredCell(prep, deliv, head)); len(problems) != 0 {
		t.Errorf("gate: a brief its delivering change added must stay closable; problems=%v", problems)
	}
	if _, _, err := failFirstBaseRev(root, "", deliv, briefPath); err != nil {
		t.Errorf("runner: a brief its delivering change added must stay closable; err=%v", err)
	}
	// prep is a legitimate commit with a parent, but neither it nor its parent
	// holds the brief: that red is about missing files, not the change.
	const want = "neither the base nor the delivering commit"
	rootC := gitOutT(t, root, "rev-parse", prep+"^")
	if problems := ffDeliveredGate(t, root, ffDeliveredCell(rootC, prep, head)); !containsSub(problems, want) {
		t.Errorf("gate: a delivering commit older than the brief must be refused; problems=%v", problems)
	}
	if _, _, err := failFirstBaseRev(root, "", prep, briefPath); err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("runner: a delivering commit older than the brief must be refused; err=%v", err)
	}
}

// TestVerifyrunRecordsDeliveringCommit — the runner wiring, end to end through
// `verifyrun --fail-first --delivering-commit`: the base run is the delivering
// commit's parent, the Base cell records the delivering commit, the gate's
// reader and the held-scope grammar both read that cell back, and the flag is
// refused outside --fail-first.
// regression: #2721
func TestVerifyrunRecordsDeliveringCommit(t *testing.T) {
	root, _, prep, deliv, _, _, briefPath := ffDeliveredRepo(t, false)
	res, out := captureVerifyrunOut(t, []string{"--fail-first", "--dry-run", "--root", root, "--delivering-commit", deliv, "--brief", briefPath})
	want := "base=" + prep[:12] + " red rc=1 · head="
	if res.code != verifyrunExitPass || !strings.Contains(out, want) || !strings.Contains(out, "· delivering="+deliv[:12]) {
		t.Fatalf("want a red at the delivering commit's parent with the delivering commit recorded; code=%d out=%s", res.code, out)
	}
	var cell string
	for _, line := range strings.Split(out, "\n") {
		if f, ok := failFirstOf(line); ok && f.Base != "" {
			if f.Delivering != deliv[:12] || f.Base != prep[:12] {
				t.Errorf("failFirstOf read base=%q delivering=%q; want %q %q", f.Base, f.Delivering, prep[:12], deliv[:12])
			}
			cell = strings.TrimSpace(witnessCells(line)[witnessCellBase])
		}
	}
	if cell == "" || !heldBaseCellRe.MatchString(cell) {
		t.Errorf("the held-scope grammar must read the recorded Base cell %q", cell)
	}
	res, stderr := captureVerifyrun(t, []string{"--dry-run", "--root", root, "--delivering-commit", deliv, "--brief", briefPath})
	if res.code != verifyrunExitUsageError || !strings.Contains(stderr, "--delivering-commit only applies to --fail-first") {
		t.Errorf("--delivering-commit without --fail-first must be a usage error; code=%d stderr=%s", res.code, stderr)
	}
}

// TestFailFirstDeliveringCellGrammar — the delivering token is read once, only
// after a red or green base run, and the held-scope grammar admits exactly the
// cell the runner writes, with or without retained base output.
// regression: #2721
func TestFailFirstDeliveringCellGrammar(t *testing.T) {
	row := func(cell string) string {
		return "| 1 | `true` | pass exit=0 | sha256:0123456789ab | 2026-11-15 | human:fixture | " + cell + " |"
	}
	red := failFirstCell("bbbbbbbbbbbb", baseRed, 1, "aaaaaaaaaaaa")
	digest := " output=sha256:" + strings.Repeat("c", 64) + " log=row-000001.log"
	for _, c := range []struct {
		name, cell, want string
		ok, held         bool
	}{
		{"red with delivering", red + " · delivering=dddddddddddd", "dddddddddddd", true, true},
		{"red with delivering and retained output", red + " · delivering=dddddddddddd" + digest, "dddddddddddd", true, true},
		{"red without delivering", red, "", true, true},
		{"delivering twice", red + " · delivering=dddddddddddd · delivering=eeeeeeeeeeee", "", false, false},
		{"delivering on an unproven base", failFirstCell("bbbbbbbbbbbb", baseUnproven, -1, "aaaaaaaaaaaa") + " · delivering=dddddddddddd", "", false, false},
		{"delivering not hex", red + " · delivering=HEAD~1", "", false, false},
	} {
		f, ok := failFirstOf(row(c.cell))
		if ok != c.ok || ok && f.Delivering != c.want {
			t.Errorf("%s: failFirstOf ok=%v delivering=%q, want ok=%v %q", c.name, ok, f.Delivering, c.ok, c.want)
		}
		if got := heldBaseCellRe.MatchString(c.cell); got != c.held {
			t.Errorf("%s: heldBaseCellRe=%v, want %v", c.name, got, c.held)
		}
	}
}

// TestFailFirstDeliveringNoMergeBase — ruling A (medici-finance/assay#5544):
// with origin/main unresolvable the runner fails closed on every arm, a
// recorded --delivering-commit included, and the shared binding refuses it
// for the closure gate too. Before this, the delivering arm checked only that
// the commit was on HEAD's history and had a parent.
// regression: #2721
func TestFailFirstDeliveringNoMergeBase(t *testing.T) {
	root, _, prep, deliv, _, _, briefPath := ffDeliveredRepo(t, false)
	gitRun(t, root, "update-ref", "-d", remoteMainRef)
	for _, c := range []struct{ name, base, delivering string }{
		{"delivering", "", deliv},
		{"delivering+base", prep, deliv},
		{"base", prep, ""},
	} {
		base, d, err := failFirstBaseRev(root, c.base, c.delivering, briefPath)
		if err == nil || !strings.Contains(err.Error(), "merge-base") {
			t.Errorf("%s: unresolvable merge-base accepted: base=%q delivering=%q err=%v", c.name, base, d, err)
		}
	}
	if ancestry, held := failFirstBaseProblem(root, briefPath, prep, deliv, "", gitOutT(t, root, "rev-parse", "HEAD")); ancestry == "" && held == "" {
		t.Error("gate binding accepted a delivering witness with no merge-base")
	}
}

// TestFailFirstDeliveringOffMain — review SEC-1. A commit X off main (its parent
// the root commit, adding the brief) merged into an Evidence branch with
// `-s ours` is on HEAD's history but not on the default branch's. Named as the
// delivering commit it made the base the root commit, accepted by both
// surfaces. The delivering commit must be the merge-base or an ancestor of it.
// regression: #2721
func TestFailFirstDeliveringOffMain(t *testing.T) {
	withBase(t, true)
	root, rootC, _, _, head, _, briefPath := ffDeliveredRepo(t, false)
	gitRun(t, root, "checkout", "-q", "--detach", rootC)
	mustMkdirAll(t, filepath.Dir(briefPath))
	writeTemp(t, filepath.Dir(briefPath), filepath.Base(briefPath), ffBriefRaw("yes", ffDeliveredVerify, ""))
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "off main")
	x := gitOutT(t, root, "rev-parse", "HEAD")
	gitRun(t, root, "checkout", "-q", "-b", "evidence", head)
	gitRun(t, root, "merge", "-q", "-s", "ours", "--no-edit", x)
	ev := gitOutT(t, root, "rev-parse", "HEAD")
	const want = "must be on the default branch's history"
	if problems := ffDeliveredGate(t, root, ffDeliveredCell(rootC, x, ev)); !containsSub(problems, want) {
		t.Errorf("gate: a delivering commit off the default branch must be refused; problems=%v", problems)
	}
	if base, _, err := failFirstBaseRev(root, "", x, briefPath); err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("runner: a delivering commit off the default branch must be refused; base=%s err=%v", base, err)
	}
}

// TestFailFirstBaseIsHeadAfterMerge — review F4. After the merge, with no
// delivering commit, the merge-base is HEAD: the default base and --base HEAD
// both ran the row on HEAD itself, and the gate accepted a witness whose base
// is its own head. Before the merge (origin/main behind HEAD) the default base
// is still accepted.
// regression: #2721
func TestFailFirstBaseIsHeadAfterMerge(t *testing.T) {
	withBase(t, true)
	root, _, _, deliv, head, _, briefPath := ffDeliveredRepo(t, false)
	const want = "the base is the head the row ran on"
	for _, rev := range []string{"", "HEAD", head} {
		if base, _, err := failFirstBaseRev(root, rev, "", briefPath); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("runner, --base %q after the merge: base=%s err=%v, want %q", rev, base, err, want)
		}
	}
	if problems := ffDeliveredGate(t, root, ffDeliveredCell(head, "", head)); !containsSub(problems, want) {
		t.Errorf("gate: a base equal to its own head must be refused; problems=%v", problems)
	}
	// Positive control: before the merge the merge-base is behind HEAD.
	gitRun(t, root, "update-ref", remoteMainRef, deliv)
	if base, _, err := failFirstBaseRev(root, "", "", briefPath); err != nil || base != deliv {
		t.Errorf("runner, before the merge: base=%s err=%v, want %s", base, err, deliv)
	}
	if problems := ffDeliveredGate(t, root, ffDeliveredCell(deliv, "", head)); len(problems) != 0 {
		t.Errorf("gate, before the merge: problems=%v", problems)
	}
}

// TestFailFirstDeliveringNamesEnv — review F5. A variable that selects the
// native Git profile makes the delivering arm refuse; the refusal names the
// variable and the remedy. A count of 0 selects none (#7007), so the case
// injects one key.
// regression: #2721
func TestFailFirstDeliveringNamesEnv(t *testing.T) {
	root, _, _, deliv, _, _, briefPath := ffDeliveredRepo(t, false)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.worktree")
	t.Setenv("GIT_CONFIG_VALUE_0", "/")
	_, _, err := failFirstBaseRev(root, "", deliv, briefPath)
	if err == nil || !strings.Contains(err.Error(), "GIT_CONFIG_COUNT is set") || !strings.Contains(err.Error(), "unset it") {
		t.Errorf("want a refusal naming GIT_CONFIG_COUNT and the remedy; err=%v", err)
	}
}
