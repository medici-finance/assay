package main

// Fail-first audit (verify-integrity/03).
//
// A Verify row that has never been seen to fail is not a check. `verifyrun
// --fail-first` therefore runs each RISK-BEARING row twice: first on the
// merge-base, checked out into a temporary worktree, then on the head. The
// witness row records both in a trailing Base cell —
//
//	base=<sha12> red rc=1 · head=<sha12>
//	base=<sha12> red rc=1 · head=<sha12> · delivering=<sha12>
//
// — the second form when the verifier named the delivering commit — and a row that is green at base and passes at head is non-discriminating:
// it would have passed without the change it is meant to prove.
//
// SINGLE POINT OF FAILURE, and the layers behind it. The red at base means
// something only if "base" is the tree WITHOUT this change. A run against a
// sibling tree, against HEAD's own commit, or against a commit of the branch
// reds (or greens) for reasons that have nothing to do with the change. So the
// witness records the base SHA it ran against, and the closure gate
// (failFirstGateChecks) accepts a fail-first witness only when
//   - with no delivering commit recorded, its base is EXACTLY the merge-base of
//     HEAD and the default branch (so HEAD's own commit, any branch commit and
//     any older ancestor are refused);
//   - with a delivering commit recorded (verifyrun --delivering-commit, the
//     post-merge case where the merge-base already holds the change), that
//     commit is HEAD or an ancestor of HEAD, the base is exactly its first
//     parent, and the base is the merge-base or an ancestor of it
//     (medici-finance/assay#2721, option 2);
//   - the brief file is held at its current path by the base, or by the
//     delivering commit when one is recorded (a brief the delivering change
//     itself added), so a base older than the brief, such as the repository's
//     root commit, where every row reds for reasons that have nothing to do
//     with any change, is refused; and
//   - it is the LATEST witness for the row and was run for the row's CURRENT
//     command (the sibling check checkWitnesses applies the same two rules).
//
// THE LIMIT, stated here and wherever the guarantee is claimed (the doc's
// Fail-first section, the verify-desk step, the PR body): the gate binds the
// base to the delivering commit the witness records, in code. It does NOT
// prove that the recorded commit is the one that delivered THIS change: which
// commit on HEAD's history is "the delivering commit" is the verifier's
// attestation, read from the forge, and the Base cell is self-attested
// Evidence text like every witness cell.
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
	Base         string // short base SHA, "" when the row was not run at base
	State        string // red | green | unproven | not-selected
	Head         string // the head tree token the same witness row ran on
	Exit         int    // base exit code, -1 when nothing ran
	Delivering   string // short delivering commit SHA, "" when none was recorded
	OutputDigest string // full sha256 of opt-in retained primary output
	LogName      string // generated filename within the caller's base-log directory
}

// failFirstCellRe reads a Base cell back.
var failFirstCellRe = regexp.MustCompile(`\bbase=([0-9a-f]{7,40}|-)\s+(red|green|unproven|not-selected)\b(?:\s+rc=(-|\d+))?(?:\s*·\s*head=([^\s|]+))?`)

// deliveringCellRe reads the delivering commit a Base cell records after its
// head token.
var deliveringCellRe = regexp.MustCompile(`·\s*delivering=([0-9a-f]{7,40})(?:\s|$)`)

var baseOutputCellRe = regexp.MustCompile(`\soutput=(sha256:[0-9a-f]{64}) log=(row-[0-9]{6}\.log)$`)

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
	if strings.Contains(cells[witnessCellBase], "output=") || strings.Contains(cells[witnessCellBase], "log=") {
		metadata := baseOutputCellRe.FindStringSubmatch(strings.TrimSpace(cells[witnessCellBase]))
		if metadata == nil || (f.State != baseRed && f.State != baseGreen) || strings.Count(cells[witnessCellBase], "output=") != 1 || strings.Count(cells[witnessCellBase], "log=") != 1 {
			return failFirst{}, false
		}
		f.OutputDigest, f.LogName = metadata[1], metadata[2]
	}
	if strings.Contains(cells[witnessCellBase], "delivering=") {
		d := deliveringCellRe.FindStringSubmatch(cells[witnessCellBase])
		if d == nil || (f.State != baseRed && f.State != baseGreen) || strings.Count(cells[witnessCellBase], "delivering=") != 1 {
			return failFirst{}, false
		}
		f.Delivering = d[1]
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
// (the runner's enumeration) cannot place AND that is shaped like Verify rows:
// a data row too short to reach the Command cell, a Verify-shaped table whose
// header names no Command column (it has an Expect column, e.g. `# | Cmd |
// Expect`), and a headerless table whose first line is already a numbered row
// (a prose paragraph split the Verify table, so its later rows lost the header
// and the runner takes the first of them for one). Such text is never run and
// never witnessed, so a closure the branch makes must not pass over it in
// silence (failFirstGateChecks reports each as a PROBLEM).
//
// A table that is NOT Verify-shaped is not reported: the author-brief skill's
// B5 step sanctions a `Failure mode of the work | Caught by` map "beside the
// table", and a brief may keep a `Row | The mutation that must redden it` map or
// a legend in ## Verify. A table is Verify-shaped when its header has an
// Expect/Expected column, or its first line is a data row whose first cell is a
// row id (`1`, `#1`, `R1`); the map and legend headers have neither.
func verifyUnplacedRows(section string) []string {
	var out []string
	cmdIdx := -1
	first := ""
	flush := func() {
		if cmdIdx < 0 && first != "" && verifyShapedHeader(first) {
			out = append(out, "a table that looks like Verify rows but whose header names no `Command` column ("+truncateRunes(first, 60)+")")
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

// verifyRowIDCellRe is a Verify row id as authors write it: `1`, `#1`, `R1`,
// `1a`.
var verifyRowIDCellRe = regexp.MustCompile(`^(?i)#?r?[0-9]+[a-z]?$`)

// verifyShapedHeader reports whether the first line of a Command-less table
// looks like a Verify table's header (an Expect/Expected column) or like a
// Verify data row that lost its header (its first cell is a row id).
func verifyShapedHeader(line string) bool {
	cells := splitRowEscaped(line)
	for j, c := range cells {
		c = strings.ToLower(strings.TrimSpace(c))
		if c == "expect" || c == "expected" {
			return true
		}
		if j == 0 && verifyRowIDCellRe.MatchString(c) {
			return true
		}
	}
	return false
}

// truncateRunes shortens s to at most n runes, marking the cut.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// failFirstBaseRev resolves the base the fail-first run checks out, and the
// full delivering commit the witness records ("" when none).
//   - --delivering-commit <rev> (after the change has merged, when the
//     merge-base already holds it): the commit must be HEAD or an ancestor of
//     HEAD and the base is its first parent; a --base beside it must name that
//     same parent (medici-finance/assay#2721, option 2).
//   - --base <rev> alone: it must be the merge-base of HEAD and origin/main
//     itself.
//   - neither: the exact merge-base of HEAD and origin/main.
//
// The runner refuses here what the closure gate would refuse later
// (failFirstBaseProblem). With the merge-base unresolvable the gate runs
// degraded and says so, so only the delivering commit's own binding is checked.
func failFirstBaseRev(root, rev, delivering, briefPath string) (base, deliveringFull string, err error) {
	for _, op := range []struct{ flag, v string }{{"--base", rev}, {"--delivering-commit", delivering}} {
		if strings.HasPrefix(op.v, "-") {
			return "", "", fmt.Errorf("%s %q is not a revision", op.flag, op.v)
		}
	}
	full := ""
	if rev != "" {
		full, err = resolveCommit(root, rev)
		if err != nil || !isObjectID(full) {
			return "", "", fmt.Errorf("--base %q names no commit in this clone", rev)
		}
	}
	if delivering != "" {
		d, err := resolveCommit(root, delivering)
		if err != nil || !isObjectID(d) {
			return "", "", fmt.Errorf("--delivering-commit %q names no commit in this clone", delivering)
		}
		d, parent, why := deliveringParent(root, d)
		if why != "" {
			return "", "", fmt.Errorf("--delivering-commit %q is refused: %s", delivering, why)
		}
		if full != "" && full != parent {
			return "", "", fmt.Errorf("--base %q is refused: it is not the parent of the delivering commit %s (its first parent is %s); omit --base, which then defaults to that parent", rev, ffShort(d), ffShort(parent))
		}
		full, deliveringFull = parent, d
	}
	if full == "" {
		if b := mergeBaseExact(root, remoteMainRef); b != "" {
			return b, "", nil
		}
		return "", "", errors.New("the merge-base of HEAD and origin/main cannot be resolved (fetch origin/main, or pass --delivering-commit <the commit that delivered the change>)")
	}
	if mb := failFirstMergeBase(root); mb != "" {
		ancestry, held := failFirstBaseProblem(root, briefPath, full, deliveringFull, mb)
		flag, v := "--base", rev
		if deliveringFull != "" {
			flag, v = "--delivering-commit", delivering
		}
		if why := ancestry + held; why != "" {
			return "", "", fmt.Errorf("%s %q is refused: %s", flag, v, why)
		}
	}
	return full, deliveringFull, nil
}

// ffShort is the witness-length prefix of a commit id.
func ffShort(s string) string {
	if len(s) > treeSHALen {
		return s[:treeSHALen]
	}
	return s
}

// failFirstBaseProblem is the one base binding the runner and the closure gate
// share (medici-finance/assay#2721, option 2). It returns the ancestry problem
// and the brief-held problem, each "" when that half holds:
//   - with no delivering commit, the base must be the merge-base itself
//     (commitIsMergeBase); the brief-held rule then holds trivially, and is
//     still asked so a stand-in merge-base cannot skip it;
//   - with one, the delivering commit must be HEAD or an ancestor of HEAD, the
//     base must be exactly its first parent, and the base must be the
//     merge-base or an ancestor of it; the brief must be held by the base or by
//     the delivering commit (a brief its own delivering change added stays
//     closable after merge).
func failFirstBaseProblem(root, briefPath, base, delivering, mergeBase string) (ancestry, held string) {
	if delivering == "" {
		if ok, why := commitIsMergeBase(root, base, mergeBase); !ok {
			return why, ""
		}
		return "", briefHeldAtBase(root, briefPath, base, mergeBase)
	}
	d, parent, why := deliveringParent(root, delivering)
	if why != "" {
		return "the delivering commit " + delivering + " is refused: " + why, ""
	}
	if b, why := witnessCommit(root, base); why != "" || b != parent {
		if why == "" {
			why = "its first parent is " + ffShort(parent)
		}
		return "it is not the parent of the delivering commit " + ffShort(d) + " (" + why + ")", ""
	}
	if ok, why := commitIsAncestor(root, base, mergeBase); !ok {
		return why, ""
	}
	return "", briefHeldForDelivery(root, briefPath, parent, d)
}

// briefHeldForDelivery is the brief-held rule for a witness that records its
// delivering commit: the base (its parent) or the delivering commit itself must
// hold the brief file at its current path. A brief the delivering change added
// is held by the delivering commit; a commit from before the brief existed
// holds it at neither, so the root commit still cannot stand in. It fails
// closed when the brief path does not resolve.
func briefHeldForDelivery(root, briefPath, base, delivering string) string {
	rel, resolved := briefRelPath(root, briefPath)
	if !resolved {
		return "the brief path does not resolve under this clone, so neither the base nor the delivering commit can be shown to hold it"
	}
	if pathHeldAt(root, base, rel) || pathHeldAt(root, delivering, rel) {
		return ""
	}
	return "neither the base nor the delivering commit " + ffShort(delivering) + " holds " + rel + ", so the Verify rows did not exist on either tree and a red there is not about this change (name the commit that delivered the change, after the brief existed)"
}

// pathHeldAt reports whether rev's tree holds the file rel (a slash path from
// the repository root). A package var so tests can stand in for git.
var pathHeldAt = func(root, rev, rel string) bool {
	_, err := gitOut(root, "cat-file", "-e", rev+":"+rel)
	return err == nil
}

// canonicalPath makes p absolute and resolves symlinks when it can (a macOS
// /tmp is /private/tmp), so two spellings of one file compare equal.
func canonicalPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		p = r
	}
	return p
}

// briefHeldAtBase is the in-code half of the fail-first base binding: a base
// older than the brief is refused, so the repository's root commit (or any
// commit from before the brief was authored) cannot stand in for "the tree
// without this change". It returns "" when the base is acceptable and the
// reason otherwise.
//
// The rule has two arms and no exemption:
//   - the merge-base holds the brief file at its current path: the chosen base
//     must hold it too (any ancestor from the brief's creation up to the
//     merge-base passes);
//   - the merge-base does NOT hold it there (the brief was created, renamed or
//     moved on this branch, its path does not resolve under the root, or the
//     probe failed): no ancestor can be shown to hold the brief, so the only
//     base accepted is the merge-base itself. The check fails closed.
//
// With no delivering commit recorded the base must also be the merge-base
// itself (failFirstBaseProblem), so this rule is defence in depth there; a
// witness that records one is held by briefHeldForDelivery instead.
func briefHeldAtBase(root, briefPath, base, mergeBase string) string {
	if mergeBase == "" {
		return ""
	}
	rel, resolved := briefRelPath(root, briefPath)
	if resolved && pathHeldAt(root, mergeBase, rel) {
		if pathHeldAt(root, base, rel) {
			return ""
		}
		return "the base predates the brief: it does not hold " + rel + ", which the merge-base does, so the Verify rows did not exist on that tree and a red there is not about this change (use the default base, or --delivering-commit <the commit that delivered the change>)"
	}
	if sameCommit(root, base, mergeBase) {
		return ""
	}
	what := "the brief path does not resolve under this clone"
	if resolved {
		what = "the merge-base does not hold " + rel + " (the brief was created, renamed or moved on this branch, or the probe failed)"
	}
	return "the base cannot be shown to hold the brief: " + what + ", so no ancestor of the merge-base can be shown to hold it and only the merge-base itself is accepted as the base (use the default base)"
}

// briefRelPath returns the brief's slash path relative to the repository root,
// and false when it does not resolve to a file inside the root.
func briefRelPath(root, briefPath string) (string, bool) {
	if briefPath == "" {
		return "", false
	}
	rel, err := filepath.Rel(canonicalPath(root), canonicalPath(briefPath))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// sameCommit reports whether rev names the commit mergeBase names. An
// unresolvable rev is not the same commit (fail closed).
func sameCommit(root, rev, mergeBase string) bool {
	if rev == "" || mergeBase == "" {
		return false
	}
	if rev == mergeBase {
		return true
	}
	full, err := resolveCommit(root, rev)
	if err != nil || !isObjectID(full) {
		return false
	}
	return full == mergeBase
}

// runFailFirstBase checks base out into a temporary detached worktree, runs
// the risky rows there under the same sandbox mode and the same CI flag as the
// head run (a row --ci skips at head is skipped at base too, and recorded
// not-selected), and removes the worktree. Rows not in risky are recorded not-selected and never run.
func runFailFirstBase(plan shellPlan, sandbox, root, base string, rows []verifyRow, risky map[string]bool, timeout time.Duration, ci bool, logs ...*baseOutputLog) (map[string]failFirst, error) {
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
		_ = os.RemoveAll(tmp)
	}()
	if _, err := gitOut(root, "worktree", "add", "--detach", "--quiet", wt, base); err != nil {
		return nil, fmt.Errorf("checking out the base %s: %w", short, err)
	}
	var retained *baseOutputLog
	var captureErr error
	if len(logs) > 0 && logs[0] != nil {
		retained = logs[0]
		plan.captureOutput = func(id string, output []byte) error {
			captureErr = retained.record(id, output)
			return captureErr
		}
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
		if retained != nil && (f.State == baseRed || f.State == baseGreen) {
			if record, ok := retained.rows[w.ID]; ok {
				f.OutputDigest, f.LogName = record.digest, record.name
			}
		}
		out[w.ID] = f
	}
	if captureErr != nil {
		return nil, fmt.Errorf("retaining base primary output: %w", captureErr)
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
			fmt.Fprintf(stdout, "row %s: non-discriminating — green at base %s and pass at head; it passes with or without the change (if the change is already merged, pass --delivering-commit <the commit that delivered it>)\n", w.ID, f.Base)
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
