package main

// Fail-first audit (verify-integrity/03).
//
// A Verify row that has never been seen to fail is not a check. `verifyrun
// --fail-first` therefore runs each RISK-BEARING row twice: first on the
// merge-base, checked out into a temporary worktree, then on the head. The
// witness row records both in a trailing Base cell —
//
//	base=<sha12> red rc=1 · head=<sha12>
//
// — and a row that is green at base and passes at head is non-discriminating:
// it would have passed without the change it is meant to prove.
//
// SINGLE POINT OF FAILURE, and the layer behind it. The red at base means
// something only if "base" is this branch's history. A run against a stale
// sibling tree reds for reasons that have nothing to do with the change. So the
// witness records the base SHA it ran against, and the closure gate
// (failFirstGateChecks) refuses a fail-first witness whose base is not an
// ancestor of HEAD. That ancestry question is the one place this package asks
// git for commit ancestry, and it is NOT a witness-applicability judgement
// (#2026 forbids judging whether a witness still speaks for a tree by ancestry,
// because a squash merge breaks ancestry without changing a byte). Here the
// base is a commit on main, which a squash merge of the branch leaves in
// main's history, and closures already on main are grandfathered by
// closedAtBase before this check runs.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Base-run states. "unproven" (the row could not run at base) deliberately
// avoids the UNRUN marker words unrunMarkerRe reads in Evidence prose.
const (
	baseRed         = "red"
	baseGreen       = "green"
	baseUnproven    = "unproven"
	baseNotSelected = "not-selected"
)

// witnessCellBase is the Base column's position in a fail-first witness row:
// the 7th cell, after every positional cell an ordinary witness has, so an
// ordinary row's reader (witnessCells needs >= 6 cells) is undisturbed.
const witnessCellBase = 6

// witnessHeaderFailFirst is witnessHeader with the Base column.
const witnessHeaderFailFirst = "| # | Command | Result | Output | Date | Runner | Base |\n" +
	"|---|---------|--------|--------|------|--------|------|"

// failFirst is one row's base-run record.
type failFirst struct {
	Base  string // short base SHA, "" when the row was not run at base
	State string // red | green | unproven | not-selected
	Head  string // the head tree token the same witness row ran on
	Exit  int    // base exit code, -1 when nothing ran
}

// failFirstCellRe reads a Base cell back.
var failFirstCellRe = regexp.MustCompile(`\bbase=([0-9a-f]{7,40}|-)\s+(red|green|unproven|not-selected)\b(?:\s+rc=(-|\d+))?(?:\s*·\s*head=([^\s|]+))?`)

// failFirstCell renders a Base cell.
func failFirstCell(base, state string, exit int, head string) string {
	if base == "" {
		base = "-"
	}
	s := "base=" + base + " " + state
	if state == baseRed || state == baseGreen {
		s += " rc=" + exitCell(exit)
	}
	return s + " · head=" + head
}

// failFirstOf lifts the Base cell of a witness row; ok is false for an
// ordinary witness (no Base cell) or a row that is not a witness.
func failFirstOf(text string) (failFirst, bool) {
	cells := witnessCells(text)
	if len(cells) <= witnessCellBase {
		return failFirst{}, false
	}
	m := failFirstCellRe.FindStringSubmatch(cells[witnessCellBase])
	if m == nil {
		return failFirst{}, false
	}
	f := failFirst{State: m[2], Head: m[4], Exit: -1}
	if m[1] != "-" {
		f.Base = m[1]
	}
	if m[3] != "" && m[3] != "-" {
		fmt.Sscanf(m[3], "%d", &f.Exit)
	}
	return f, true
}

// hexSHARe is an abbreviated-or-full commit id as a witness records it.
var hexSHARe = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

// commitIsAncestor reports whether base names a commit in this clone that is
// an ancestor of HEAD; why says what failed when it is not. The operand is
// validated as hex before git sees it, then resolved to a full object id, so
// no Evidence text reaches git as an option or a revision expression. A
// package var so tests can stand in for git.
var commitIsAncestor = func(root, base string) (bool, string) {
	if !hexSHARe.MatchString(base) {
		return false, "it is not a commit id"
	}
	full, err := gitOut(root, "rev-parse", "--verify", "--quiet", base+"^{commit}")
	if err != nil || !isObjectID(full) {
		return false, "it names no commit in this clone"
	}
	cmd := exec.Command("git", "-C", root, "merge-base", "--is-ancestor", full, "HEAD")
	err = cmd.Run()
	if err == nil {
		return true, ""
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		return false, "it is not in HEAD's history (a sibling or stale tree)"
	}
	return false, "the ancestry could not be checked: " + err.Error()
}

// failFirstRiskyIDs returns the Verify row ids that are risk-bearing: every
// row when the brief-wide risk block has a `yes`, else the rows whose text
// carries a risk-bearing|live|mutating|end-to-end tag (unrun.go's definition).
func failFirstRiskyIDs(risk map[string]string, verify string) map[string]bool {
	all := anyRiskYes(risk)
	out := map[string]bool{}
	for _, it := range parseVerifyItems(verify) {
		if all || riskBearingRowRe.MatchString(it.Text) {
			out[it.ID] = true
		}
	}
	return out
}

// failFirstBaseRev resolves the base the fail-first run checks out: the
// operator's --base revision, or the exact merge-base of HEAD and origin/main.
func failFirstBaseRev(root, rev string) (string, error) {
	if rev != "" {
		if strings.HasPrefix(rev, "-") {
			return "", fmt.Errorf("--base %q is not a revision", rev)
		}
		full, err := resolveCommit(root, rev)
		if err != nil || !isObjectID(full) {
			return "", fmt.Errorf("--base %q names no commit in this clone", rev)
		}
		return full, nil
	}
	if b := mergeBaseExact(root, remoteMainRef); b != "" {
		return b, nil
	}
	return "", errors.New("the merge-base of HEAD and origin/main cannot be resolved (fetch origin/main, or pass --base <rev>)")
}

// runFailFirstBase checks base out into a temporary detached worktree, runs
// the risky rows there, and removes the worktree. Rows not in risky are
// recorded not-selected and never run.
func runFailFirstBase(plan shellPlan, root, base string, rows []verifyRow, risky map[string]bool, timeout time.Duration) (map[string]failFirst, error) {
	short := base
	if len(short) > treeSHALen {
		short = short[:treeSHALen]
	}
	out := map[string]failFirst{}
	var sel []verifyRow
	for _, r := range rows {
		out[r.ID] = failFirst{State: baseNotSelected, Exit: -1}
		if risky[r.ID] {
			sel = append(sel, r)
		}
	}
	if len(sel) == 0 {
		return out, nil
	}
	tmp, err := os.MkdirTemp("", "verifyrun-base-")
	if err != nil {
		return nil, err
	}
	wt := filepath.Join(tmp, "tree")
	defer func() {
		_, _ = gitOut(root, "worktree", "remove", "--force", wt)
		_, _ = gitOut(root, "worktree", "prune")
		_ = os.RemoveAll(tmp)
	}()
	if _, err := gitOut(root, "worktree", "add", "--detach", "--quiet", wt, base); err != nil {
		return nil, fmt.Errorf("checking out the base %s: %w", short, err)
	}
	for _, w := range runWitnessesSandboxed(plan, sandboxUnshare, wt, sel, "", "", "", "", timeout, false) {
		f := failFirst{Base: short, Exit: w.Exit}
		switch w.State {
		case statePass:
			f.State = baseGreen
		case stateFail:
			f.State = baseRed
		default:
			f.State = baseUnproven
		}
		out[w.ID] = f
	}
	return out, nil
}

// foldFailFirst reports each row's base verdict on the console and folds the
// fail-first outcome into the exit code: a risk-bearing row that is green at
// base and passes at head is non-discriminating (exit 1, the row is a defect);
// one that could not run at base is unproven (exit 2, nobody has the answer).
func foldFailFirst(ws []witness, ff map[string]failFirst, risky map[string]bool, worst int, stdout *os.File) int {
	for _, w := range ws {
		f, ok := ff[w.ID]
		if !ok || f.State == baseNotSelected || w.State == stateSkipped {
			continue
		}
		switch {
		case f.State == baseGreen && w.State == statePass:
			fmt.Fprintf(stdout, "row %s: non-discriminating — green at base %s and pass at head; it passes with or without the change\n", w.ID, f.Base)
			if risky[w.ID] && worst == verifyrunExitPass {
				worst = verifyrunExitFail
			}
		case f.State == baseUnproven:
			fmt.Fprintf(stdout, "row %s: unproven at base %s — the row could not run there, so it was never shown to fail\n", w.ID, f.Base)
			if risky[w.ID] {
				worst = verifyrunExitCouldNot
			}
		case f.State == baseRed:
			fmt.Fprintf(stdout, "row %s: fail-first — red at base %s, %s at head\n", w.ID, f.Base, w.State)
		}
	}
	return worst
}
