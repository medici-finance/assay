package main

// Verify row CLASSES (verdict-lane/02).
//
// Every Verify row is authored as prose+shell and re-interpreted at run time.
// The verdict lane (R-6) needs the common case to be DETERMINISTIC — a machine
// re-executes it, a human trusts the run — so each row now declares its class at
// brief-author time, in an optional `Class` column:
//
//	| # | Class | Command | Expect |
//
// Four classes, two kinds:
//
//	check:ci     HERMETIC. Tree-only, no environment beyond the checkout. The
//	             lane RE-EXECUTES the row network-off against the candidate tree
//	             and refuses the whole verdict on mismatch (rulings.md R-6 c.6).
//	             Hermeticity is ENFORCED at execution (verifyrun disables the
//	             network), never merely declared.
//	check        DETERMINISTIC but ENV-BOUND — needs a live PEM, a real queue, a
//	             tool on PATH. A runner executes it; CI does not. It rests on the
//	             verdict's authorship+signature (R-6 c.1–3), not on re-execution.
//	check:cluster A SUBCLASS of `check` (verdict-lane/07): env-bound to a LIVE
//	             cluster, whose runner must be the privileged pod runner. The
//	             OFFLINE lane cannot execute it — it records could-not-check with a
//	             stable, greppable marker naming the probe, and the brief is
//	             code-verified/cluster-pending until the pod lane completes it.
//	gate:model   JUDGMENT — a model reads the row and decides.
//	gate:human   JUDGMENT — a human reads the row and decides; stays on the
//	             verify-gate issue pair, outside the transcription lane.
//
// THE LEGACY DEFAULT is the compatibility hinge: a Verify table with NO `Class`
// column is legacy, and every one of its rows is treated as `check`
// (runner-executed). Nothing in the inherited corpus breaks on day one — the
// 300-plus existing tables keep the exact behaviour they had before the column
// existed, and only a table that OPTS IN by adding the column gains the new
// routing.
//
// SCRIPT CONVENTION. A hermetic (`check:ci`) or env-bound (`check`) row may be a
// reviewed SCRIPT rather than an inline command:
//
//	docs/streams/<stream>/verify.d/brief-NN/row-K.sh   (executable, exit 0 = PASS)
//
// and then the row's Command cell IS that script path. The reviewer who approves
// the brief approves the script — the trust decision lives where it already is.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The row classes. Stable identifiers — the lint, verifyrun and the
// verdict lane all select on these exact strings.
const (
	classCheckCI      = "check:ci"      // hermetic, tree-only, CI re-executes network-off
	classCheck        = "check"         // deterministic but env-bound — runner-executed
	classCheckCluster = "check:cluster" // env-bound to a live cluster — the pod runner only (verdict-lane/07)
	classGateModel    = "gate:model"    // judgment: a model decides
	classGateHuman    = "gate:human"    // judgment: a human decides (verify-gate)
)

// classCheckCluster is a fifth class, a strict SUBCLASS of `check` (verdict-lane/07,
// the pod/online verify lane). A cluster row is deterministic but env-bound to a
// LIVE cluster — a probe run against a real participant/validator — whose runner
// MUST be the privileged pod runner; the OFFLINE verify lane holds no cluster
// access and can never execute it. Two facts flow from that:
//
//   - the offline lane records a cluster row could-not-check with a stable,
//     greppable marker (clusterPendingMarker) naming the probe, distinct from an
//     ordinary env-bound `check` skip — so "parked awaiting the pod" is
//     distinguishable from "skipped, will run later in CI"; and
//   - a brief whose only unrun Verify rows are cluster rows is code-verified but
//     cluster-pending — the pod runner's worklist, derived by --cluster-pending-queue.
//
// The probe a cluster row names is validated against the documented-probe
// registry (docs/streams/.pod-probes): an unknown probe routes the row at a
// script no pod runner provides, so it is a lint PROBLEM, never a silent pass.

// legacyRowClass is what a row resolves to when the table declares no `Class`
// column (and what an explicit-but-empty cell falls back to): `check`,
// runner-executed. This is the single fact that makes the whole column additive
// — see the file header.
const legacyRowClass = classCheck

// knownRowClasses is the closed set the lint validates against. A class outside
// it is a PROBLEM (a typo, or a class nobody has defined), never a silent pass.
var knownRowClasses = map[string]bool{
	classCheckCI:      true,
	classCheck:        true,
	classCheckCluster: true,
	classGateModel:    true,
	classGateHuman:    true,
}

// verifyRowCells is one Verify-table data row, as located by verifyRowTable.
//
// Class is the RAW cell text (markdown intact); resolveRowClass normalises it.
// Classed records whether the TABLE carried a `Class` column at all — the one
// bit that separates an explicitly-declared `check` (env-bound, skipped in CI)
// from a legacy-default `check` (the inherited corpus, which CI still
// re-executes). Collapsing the two would silently exempt every legacy row from
// the scheduled main-rerun, which is the opposite of what the column is for.
type verifyRowCells struct {
	Num     string
	Class   string
	Command string
	Expect  string
	Classed bool
}

// resolveRowClass normalises a Class cell to one of the class constants and
// reports whether it names a KNOWN class.
//
// An empty cell — or a row from a table with no Class column — resolves to the
// legacy default and is known. A non-empty cell that matches no class is
// returned verbatim (lowercased) with known=false, so the lint can name exactly
// what the author wrote.
func resolveRowClass(raw string) (class string, known bool) {
	s := strings.ToLower(strings.TrimSpace(normalizeRowID(raw))) // strip backticks/emphasis, trim
	if s == "" {
		return legacyRowClass, true
	}
	return s, knownRowClasses[s]
}

// class returns the resolved class for a parsed row: the declared class when the
// table opted into the column, else the legacy default. It does NOT report
// known-ness — the lint (verifyRowClassProblems) is the sole place an unknown
// class is judged; every other consumer wants a class to route on.
func (r verifyRowCells) class() string {
	if !r.Classed {
		return legacyRowClass
	}
	c, _ := resolveRowClass(r.Class)
	return c
}

// ---------------------------------------------------------------------------
// verify.d scripted rows
// ---------------------------------------------------------------------------

// verifyScriptRe matches a Command cell whose command IS a verify.d script path
// (the script convention above). Anchored to the whole trimmed command so an
// inline command that merely mentions such a path (`cat …/row-1.sh`) is not
// mistaken for a scripted row — a scripted row's Command is the bare path and
// nothing else.
var verifyScriptRe = regexp.MustCompile(`^docs/streams/[^/\s]+/verify\.d/[^\s]+\.sh$`)

// scriptPath returns the verify.d script a row points at, or "" when the row is
// an inline command rather than a scripted one.
func scriptPath(command string) string {
	s := strings.TrimSpace(command)
	if verifyScriptRe.MatchString(s) {
		return s
	}
	return ""
}

// ---------------------------------------------------------------------------
// Lint (a) unknown class · (b) missing / non-executable script
// ---------------------------------------------------------------------------

// verifyRowClassProblems is the class lint. Two PROBLEM shapes, both hard:
//
//	(a) an unknown class — a Class cell that names no defined class. Always a
//	    PROBLEM: a class the tool does not recognise routes the row NOWHERE, and
//	    a row that routes nowhere is silently exempt from every gate. This fires
//	    on every brief-v1 file regardless of its README-row status, because a
//	    typo'd class is wrong the moment it is written.
//
//	(b) a `check:ci`/`check` SCRIPTED row whose script is missing or not
//	    executable. A scripted row's whole contract is "a runner re-executes this
//	    file"; a file that is absent, or present but not `chmod +x`, cannot be
//	    re-executed, so the row proves nothing. SCOPED to briefs the board has
//	    moved past `todo`: a `todo` brief listing its planned verify.d scripts is
//	    a PLAN, not a claim, and reddening the tree over scripts a future brief
//	    has not written yet would punish authoring the plan. Once a brief is
//	    in-progress/implemented/verified/done, its scripted rows must resolve to
//	    real, executable files.
//
// gate:model / gate:human rows are never scripted and are not checked here.
func verifyRowClassProblems(streams []*Stream) []string {
	var problems []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }

	for _, s := range streams {
		// Map brief number → README status, so (b) can exempt todo briefs. A
		// brief file with no README row (status "") is treated as active for (b):
		// a scripted row in a file nobody lists is still a claim in the tree.
		status := map[string]string{}
		for _, br := range s.Briefs {
			status[br.Num] = br.Status
		}
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				continue // malformed reported by checkBriefFiles; legacy exempt
			}
			_, num, _ := expectedBriefID(path)
			briefStatus := status[num]
			enforceScripts := briefStatus != "" && briefStatus != "todo"
			verifyRowTable(bf.Verify, func(r verifyRowCells) {
				where := "a Verify row"
				if r.Num != "" {
					where = "Verify row " + r.Num
				}
				// (a) unknown class.
				if r.Classed {
					if cls, known := resolveRowClass(r.Class); !known {
						add("%s: %s declares class %q, which is not a defined Verify row class — the recognised classes are `check:ci` (hermetic, CI re-executes network-off), `check` (env-bound, runner-executed), `gate:model` and `gate:human`. A row whose class the tool cannot resolve routes nowhere and is silently exempt from every gate; fix the class or drop the column (an unclassed table is legacy `check`) — verdict-lane/02", path, where, cls)
						return // an unknown class makes the script check meaningless
					}
				}
				// (b) missing / non-executable script.
				if !enforceScripts {
					return
				}
				cls := r.class()
				if cls != classCheckCI && cls != classCheck {
					return
				}
				sp := scriptPath(r.Command)
				if sp == "" {
					return // inline command, not a scripted row
				}
				abs := filepath.Join(s.Root, sp)
				info, statErr := os.Stat(abs)
				switch {
				case statErr != nil:
					add("%s: %s is a `%s` scripted row whose script %s does not exist — a scripted row's Command IS the script a runner re-executes, and a row pointing at a missing file proves nothing. Create the script (executable, exit 0 = PASS) or make the row an inline command — verdict-lane/02", path, where, cls, sp)
				case info.IsDir():
					add("%s: %s points its `%s` script cell at %s, which is a directory, not an executable file — verdict-lane/02", path, where, cls, sp)
				case info.Mode()&0o111 == 0:
					add("%s: %s is a `%s` scripted row whose script %s is not executable (no +x bit) — a runner cannot re-execute it. `chmod +x` the script and commit the mode — verdict-lane/02", path, where, cls, sp)
				}
			})
		}
	}
	sort.Strings(problems)
	return problems
}

// ---------------------------------------------------------------------------
// Lint (c) reviewer-conspicuity NOTICE on a verify.d diff
// ---------------------------------------------------------------------------

// changedVerifyScriptRe matches a changed repo-relative path that lives under a
// stream's verify.d/ tree — the scripts a runner will re-execute as the machine
// half of a verdict.
var changedVerifyScriptRe = regexp.MustCompile(`^docs/streams/[^/]+/verify\.d/`)

// verifyScriptDiffNotices raises a CONSPICUOUS NOTICE, naming each script, when a
// PR's diff touches any `docs/streams/*/verify.d/**` file.
//
// The reviewer is the trust anchor for verify scripts (R-6): there is no freeze
// rule and no automated gate that says a script edit is safe — the point is the
// opposite, that a human explicitly assesses every change to code a runner will
// later execute as the deterministic half of a verdict. So this fires whenever
// the diff touches the tree, listing the exact files, so the change cannot slip
// past review as an ordinary docs edit. It is advisory (never changes the exit
// code); its whole job is to be SEEN.
//
// It reads the `changed` set (the `--changed` file CI passes); with no changed
// set it is inert, exactly like the DAR check.
func verifyScriptDiffNotices(changed []string) []string {
	var scripts []string
	for _, p := range changed {
		p = strings.TrimSpace(p)
		if changedVerifyScriptRe.MatchString(p) {
			scripts = append(scripts, p)
		}
	}
	if len(scripts) == 0 {
		return nil
	}
	sort.Strings(scripts)
	return []string{fmt.Sprintf(
		"VERIFY-SCRIPT REVIEW: this PR changes %d verify.d script(s) — %s. These are executed VERBATIM by the verdict runner as the deterministic half of a verify verdict (verdict-lane), so a change here is a change to what the machine will trust. There is no freeze and no automated safety gate on them by design: the reviewer is the trust anchor. Read each diff as executable code, not as a docs edit, before approving — verdict-lane/02",
		len(scripts), strings.Join(scripts, ", "))}
}
