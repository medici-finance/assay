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
// SINGLE POINT OF FAILURE, and the layers behind it. The red at base means
// something only if "base" is the tree WITHOUT this change. A run against a
// sibling tree, against HEAD's own commit, or against a commit of the branch
// reds (or greens) for reasons that have nothing to do with the change. So the
// witness records the base SHA it ran against, and the closure gate
// (failFirstGateChecks) accepts a fail-first witness only when
//   - its base is the merge-base of HEAD and the default branch, or an ancestor
//     of that merge-base (so HEAD's own commit and any branch commit are
//     refused, and a post-merge verifier can pass --base <the delivering
//     change's parent>); and
//   - it is the LATEST witness for the row and was run for the row's CURRENT
//     command (the sibling check checkWitnesses applies the same two rules).
//
// The ancestry question lives in failfirst_ancestry.go, the one file in this
// package that asks git for commit ancestry; it is NOT a witness-applicability
// judgement (#2026 forbids judging whether a witness still speaks for a tree by
// ancestry, because a squash merge breaks ancestry without changing a byte).
// Closures already on main are grandfathered by closedAtBase before it runs.

import (
	"errors"
	"fmt"
	"os"
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

// failFirstRiskyIDs returns the Verify row ids that are risk-bearing: every
// row when the brief-wide risk block has a `yes`, else the rows whose text
// carries a risk-bearing|live|mutating|end-to-end tag (unrun.go's definition).
// The rows are briefVerifyRows' — the SAME enumeration the runner executes and
// the closure gate holds, so no row is run and witnessed that this selection
// never saw.
func failFirstRiskyIDs(risk map[string]string, rows []verifyRow) map[string]bool {
	all := anyRiskYes(risk)
	out := map[string]bool{}
	for _, r := range rows {
		if all || riskBearingRowRe.MatchString(r.RiskText) {
			out[r.ID] = true
		}
	}
	return out
}

// verifyUnplacedRows describes what a Verify section holds that verifyRowTable
// (the runner's enumeration) cannot place: a table whose header names no
// Command column, and a data row too short to reach the Command cell. Such
// text is never run and never witnessed, so a closure the branch makes must not
// pass over it in silence (failFirstGateChecks reports each as a PROBLEM).
func verifyUnplacedRows(section string) []string {
	var out []string
	cmdIdx := -1
	first := ""
	flush := func() {
		if cmdIdx < 0 && first != "" {
			out = append(out, "a table whose header names no `Command` column ("+truncateRunes(first, 60)+")")
		}
		cmdIdx, first = -1, ""
	}
	for _, raw := range strings.Split(section, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "|") {
			flush()
			continue
		}
		if separatorRowRe.MatchString(strings.Trim(line, "|")) {
			continue
		}
		cells := splitRowEscaped(line)
		if cmdIdx < 0 {
			if first == "" {
				first = line
			}
			for j, c := range cells {
				if strings.ToLower(strings.TrimSpace(c)) == "command" {
					cmdIdx = j
				}
			}
			continue
		}
		if cmdIdx >= len(cells) {
			out = append(out, "a table row with no `Command` cell ("+truncateRunes(line, 60)+")")
		}
	}
	flush()
	return out
}

// truncateRunes shortens s to at most n runes, marking the cut.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// failFirstBaseRev resolves the base the fail-first run checks out: the
// operator's --base revision (the merge-base of HEAD and origin/main, or an
// ancestor of it: in a post-merge verify the merge-base already holds the
// change, so the verifier passes the delivering change's parent), or by
// default the exact merge-base of HEAD and origin/main.
func failFirstBaseRev(root, rev string) (string, error) {
	if rev != "" {
		if strings.HasPrefix(rev, "-") {
			return "", fmt.Errorf("--base %q is not a revision", rev)
		}
		full, err := resolveCommit(root, rev)
		if err != nil || !isObjectID(full) {
			return "", fmt.Errorf("--base %q names no commit in this clone", rev)
		}
		// Refuse here what the closure gate would refuse later: a base that is
		// neither the merge-base nor an ancestor of it was never "without this
		// change". Unresolvable merge-base: the gate runs degraded and says so.
		if mb := failFirstMergeBase(root); mb != "" {
			if ok, why := commitIsAncestor(root, full, mb); !ok {
				return "", fmt.Errorf("--base %q is refused: %s", rev, why)
			}
		}
		return full, nil
	}
	if b := mergeBaseExact(root, remoteMainRef); b != "" {
		return b, nil
	}
	return "", errors.New("the merge-base of HEAD and origin/main cannot be resolved (fetch origin/main, or pass --base <rev>)")
}

// runFailFirstBase checks base out into a temporary detached worktree, runs
// the risky rows there under the same sandbox mode and the same CI flag as the
// head run (a row --ci skips at head is skipped at base too, and recorded
// not-selected), and removes the worktree. Rows not in risky are recorded not-selected and never run.
func runFailFirstBase(plan shellPlan, sandbox, root, base string, rows []verifyRow, risky map[string]bool, timeout time.Duration, ci bool) (map[string]failFirst, error) {
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
	for _, w := range runWitnessesSandboxed(plan, sandbox, wt, sel, "", "", "", "", timeout, ci) {
		f := failFirst{Base: short, Exit: w.Exit}
		switch w.State {
		case stateSkipped:
			out[w.ID] = failFirst{State: baseNotSelected, Exit: -1}
			continue
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
			fmt.Fprintf(stdout, "row %s: non-discriminating — green at base %s and pass at head; it passes with or without the change (if the change is already merged, pass --base <the parent of the delivering merge>)\n", w.ID, f.Base)
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
