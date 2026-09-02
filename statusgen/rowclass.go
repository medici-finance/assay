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

// ---------------------------------------------------------------------------
// Obligation classes (mistake-proofing/03, spec §4 B2 + §3 D7)
// ---------------------------------------------------------------------------
//
// AXIS DECISION — recorded here, in source, before the code, because it is the
// central design call this brief makes.
//
// The five execution classes above answer ONE question: WHO EXECUTES a row (a
// hermetic re-run, a runner, the pod runner, a model, a human). The four classes
// below answer a DIFFERENT, orthogonal question: WHAT OBLIGATION the row
// discharges — a mutation proves a guard reddens, a flow crosses a component
// boundary, a dereference resolves a claim, a neighbour exercises an untouched
// sibling. The two axes are independent: a mutation row is still executed by
// someone. Collapsing them into ONE closed set — adding `mutation` to
// knownRowClasses — would force every author to choose between naming WHO runs a
// row and naming WHAT it discharges, and would silently exempt every
// obligation-classed row from execution routing (its class() would resolve to an
// unknown execution value and route nowhere). So the obligation is a SECOND,
// independent closed set, carried on the SAME cell by a compound encoding.
//
// ENCODING — a COMPOUND CELL, chosen over a second table column. The `Class`
// cell carries an execution value, then zero or more `+`-prefixed obligation
// tokens, whitespace-separated. A brief-shaped worked example:
//
//	| # | Class                   | Command                          | Expect |
//	|---|-------------------------|----------------------------------|--------|
//	| 1 | check:ci +mutation      | `go test ./x -run TestGuard`     | exit 0; reddens on the mutation entry |
//	| 2 | check +flow             | docs/streams/x/verify.d/b-1/f.sh | exit 0 |
//	| 3 | gate:model +dereference | prose — the named symbol resolves | the claim resolves against the tree |
//
// Why a compound cell and NOT a second column: the execution column is OPTIONAL
// and its ABSENCE is the legacy-default hinge (a table with no `Class` column is
// legacy `check`). A second obligation column would add a THIRD table shape
// (no-class / class-only / class+obligation) and force verifyRowTable to grow a
// second optional-column search — multiplying the ways the legacy hinge could
// shift. The compound cell keeps the table's column set byte-identical to
// today's: the whole obligation axis lives INSIDE the one optional `Class` cell,
// so a column-less legacy table is untouched, an execution-only cell parses
// exactly as before, and only a cell that opts in by writing a `+token` gains an
// obligation. splitRowClassCell is the single seam that separates the two axes.
//
// PRESENCE, NOT ADEQUACY (spec §3 D7). This encoding, and the derivation that
// reads it (obligationderivation.go), carry only whether an obligation ROW IS
// PRESENT. Whether a mutation row actually reddens a guard, whether a flow row
// actually crosses a boundary, whether a dereference row actually dereferences —
// that ADEQUACY is not decidable from row text and stays the reviewer's call.
// Presence is the control; adequacy is review. Every message this surface emits
// says which half it covers.
//
// NEIGHBOUR IS VALIDATED BUT NOT DERIVED. The `+neighbour` token is a defined,
// closed-set obligation — an author may write it and it validates — but no
// honest path-only signal distinguishes "a sibling the change did not touch"
// from "any of the many files this change happens not to touch", so its
// derivation TRIGGER is deferred to a follow-up (task 3 explicitly sanctions
// this). mutation, flow and dereference are derived; neighbour is carried.
const (
	classMutation    = "mutation"    // breaks the guarded thing and proves the guard reddens (spec D1)
	classFlow        = "flow"        // exercises the cross-component path end to end, not just the changed site
	classDereference = "dereference" // resolves a claim rather than counting its presence
	classNeighbour   = "neighbour"   // exercises a sibling site the change did not touch (validated, not yet derived)
)

// knownObligations is the closed obligation set — the SECOND axis, deliberately
// separate from knownRowClasses. An obligation token outside it is a hard
// PROBLEM, exactly as an unknown execution class is: an unrecognised token
// routes nowhere. Kept unknown-fatal so a typo'd `+mutaton` cannot pass as a
// silently-absent obligation.
var knownObligations = map[string]bool{
	classMutation:    true,
	classFlow:        true,
	classDereference: true,
	classNeighbour:   true,
}

// obligationSeparator prefixes each obligation token in a compound Class cell
// (`check:ci +mutation`), distinguishing it from the single execution value.
const obligationSeparator = "+"

// splitRowClassCell separates a compound `Class` cell into its execution token
// and its obligation tokens. The cell is `<execution> +<obligation>...`: one
// execution value (optional — an empty or `+`-leading cell is the legacy
// default), then zero or more obligation tokens, each optionally `+`-prefixed.
// A cell with no obligation token returns nil obligations and parses exactly as
// the pre-obligation code did — which is why every inherited table and every
// existing execution-class test is untouched by this change.
func splitRowClassCell(raw string) (execCell string, obligations []string) {
	s := strings.ToLower(strings.TrimSpace(normalizeRowID(raw)))
	if s == "" {
		return "", nil
	}
	fields := strings.Fields(s)
	i := 0
	if !strings.HasPrefix(fields[0], obligationSeparator) {
		execCell = fields[0]
		i = 1
	}
	for _, f := range fields[i:] {
		obligations = append(obligations, strings.TrimPrefix(f, obligationSeparator))
	}
	return execCell, obligations
}

// obligations returns the KNOWN obligation tokens a row's Class cell carries.
// nil for a legacy (unclassed) table, an execution-only cell, or a cell whose
// only obligation tokens are unrecognised — those are caught as PROBLEMs by
// verifyRowClassProblems; here we report only what a consumer can route on.
func (r verifyRowCells) obligations() []string {
	if !r.Classed {
		return nil
	}
	_, obs := splitRowClassCell(r.Class)
	var known []string
	for _, o := range obs {
		if knownObligations[o] {
			known = append(known, o)
		}
	}
	return known
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
	// A compound cell (`check +mutation`) routes on its EXECUTION token only; the
	// obligation tokens are a second axis every execution consumer ignores.
	execCell, _ := splitRowClassCell(r.Class)
	if execCell == "" {
		return legacyRowClass
	}
	c, _ := resolveRowClass(execCell)
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
				// (a) unknown class OR unknown obligation. A compound cell
				// (`check +mutation`, mistake-proofing/03) is split into its
				// execution token and its obligation tokens; each axis is
				// validated against its own closed set, and an unrecognised value
				// on EITHER axis is fatal — a token the tool cannot resolve routes
				// nowhere and is silently exempt from its gate.
				if r.Classed {
					execCell, obs := splitRowClassCell(r.Class)
					if execCell != "" {
						if cls, known := resolveRowClass(execCell); !known {
							add("%s: %s declares class %q, which is not a defined Verify row class — the recognised classes are `check:ci` (hermetic, CI re-executes network-off), `check` (env-bound, runner-executed), `gate:model` and `gate:human`. A row whose class the tool cannot resolve routes nowhere and is silently exempt from every gate; fix the class or drop the column (an unclassed table is legacy `check`) — verdict-lane/02", path, where, cls)
							return // an unknown class makes the script check meaningless
						}
					}
					for _, ob := range obs {
						if !knownObligations[ob] {
							add("%s: %s declares obligation %q, which is not a defined Verify-row obligation — the recognised obligations are `+mutation`, `+flow`, `+dereference` and `+neighbour` (mistake-proofing/03). An obligation token the tool cannot resolve routes nowhere and is silently exempt from the obligation derivation; fix the token or drop it. This lint checks the PRESENCE of a typed obligation, not its adequacy, which stays the reviewer's call (spec §3 D7) — mistake-proofing/03", path, where, ob)
							return // an unknown obligation makes the script check meaningless
						}
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
