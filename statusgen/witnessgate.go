package main

// witnessgate — the stream-README Status cell is DERIVED from the witness, not
// asserted by whoever edited the table (ground-truth/02, #284).
//
// THE CLAIM A CELL MAKES. `verified` on a stream README says: this brief's
// Verify rows were run by someone who did not do the work, and they passed.
// Since ground-truth/01 that claim has a machine-readable form —
// `statusgen verifyrun --check <brief>` exits 0 — and once a claim has a
// machine-readable form, leaving the cell hand-asserted means the board can
// disagree with its own evidence and nothing notices. This check is the
// disagreement detector: when a brief's own Evidence records a witness that
// FAILED, the cell cannot go on reading `verified`/`done`.
//
// WHY ONLY THE CONTRADICTION, AND NOT THE ABSENCE. Three states, three
// treatments, and they are deliberately not the same severity:
//
//	pass           the cell is corroborated. Nothing to report.
//	fail           the cell is CONTRADICTED by the brief's own record. THIS
//	               check — a PROBLEM when the branch made the closure.
//	could-not-run  the cell is UNCORROBORATED. witnessNotices' rolled-up NOTICE
//	               (see verifyrun.go) — not this check.
//
// The split is measured, not stylistic. On 2026-08-13, 319 of the 320 brief
// files in this repo had no witness for any row and exactly ZERO rows anywhere
// in the corpus recorded `fail`. So absence is the entire inherited corpus —
// a hard error there would red main on merge and the only way to green it would
// be to hand-write witnesses into already-closed briefs, manufacturing the very
// evidence the witness replaces. Contradiction, by contrast, cannot be
// inherited by accident: a `fail` witness only exists because some run wrote
// one, so a PROBLEM there costs nothing today and gates every future case.
//
// SCOPED TO THE TRANSITION, exactly as unrunGateChecks is. A brief already
// `verified`/`done` at the merge-base was closed by an earlier branch; making
// it a hard error would mean an unrelated PR that merely touches a stream
// inherits somebody else's red. Only a closure THIS branch made is a PROBLEM;
// the inherited case stays a NOTICE, visible and never silently green.
//
// THE OVERRIDE IS THE DEMOTION, and it is not a bypass. A brief whose witness
// is red is not stranded: edit the Status cell back to `implemented` and the
// check releases. That is the whole rule — `verified` reverts to `implemented`
// when its witness goes red — and it leaves an audit row in the diff, because
// what changed is that the repo stopped making a claim it could not support.
// There is deliberately no flag, label, or env var that suppresses this: a
// suppression a worker can apply to their own branch would make the cell
// asserted again, one level up.

import (
	"github.com/medici-finance/assay/statusgen/evaluation"

	"fmt"
	"sort"
	"strings"
)

// witnessAbsenceGateChecks reports briefs whose `verified`/`done` cell is a
// closure THIS BRANCH made with NO execution witness at all for one or more
// Verify rows.
//
// A THIRD CASE, distinct from the two above. witnessNotices (verifyrun.go) is
// the roll-up for the INHERITED corpus — every brief already closed, on main,
// with no witness — and stays a per-stream NOTICE forever; a hard error there
// would red main on a PR that never touched the offending brief. witnessGateChecks
// above is the CONTRADICTION case: a witness ran and recorded `fail`. This
// function is neither: nothing here contradicts the cell, and nothing was
// inherited either — verifyrun already existed when this branch opened, so a
// NEW closure with no witness behind it chose not to run it. That is exactly
// the self-report the witness was built to replace, and unlike the inherited
// backlog there is no excuse for it: PROBLEM, not NOTICE.
//
// SCOPED TO THE TRANSITION, and reusing the SAME closedAtBase predicate
// witnessGateChecks and unrunGateChecks already resolve — a brief already
// verified/done at the merge-base is the inherited backlog and stays covered
// by witnessNotices' roll-up; only a closure this branch newly makes is a
// PROBLEM here. An unresolvable base grandfathers everything, exactly as the
// two checks above do: with no observable transition there is nothing to gate.
func witnessAbsenceGateChecks(root string, streams []*Stream) (problems, notices []string) {
	grandfathered, baseOK := closedAtBase(root, streams)
	degraded := false
	for _, s := range streams {
		for i := range s.Briefs {
			br := &s.Briefs[i]
			if br.Status != "done" && br.Status != "verified" {
				continue
			}
			art, ok := loadBriefArtifacts(s, br.Num)
			if !ok {
				continue
			}
			rows := briefVerifyRows(art.Verify)
			if len(rows) == 0 {
				continue // no Verify table: verifySectionProblems' business
			}
			evidence := parseEvidenceRows(art.Evidence)
			var missing []string
			for _, r := range rows {
				witnessed := false
				for _, er := range evidence[r.ID] {
					if isWitnessRow(er.Text) {
						witnessed = true
						break
					}
				}
				if !witnessed {
					missing = append(missing, "#"+r.ID)
				}
			}
			if len(missing) == 0 {
				continue
			}
			id := s.Name + "/brief-" + br.Num
			if !baseOK || grandfathered[s.Name+"/"+br.Num] {
				if !baseOK {
					degraded = true
				}
				// witnessNotices already rolls this brief into its per-stream
				// NOTICE (the inherited-corpus case) — never double-report the
				// same absence as a second finding here.
				continue
			}
			problems = append(problems, fmt.Sprintf(
				"%s: cannot close as %s — this branch's own closure carries no EXECUTION WITNESS in Evidence for Verify row(s) %s. Run `statusgen verifyrun --brief %s` and commit the resulting witness table before closing, or set the Status cell back to `implemented` — a NEW closure must carry the witness it asserts",
				id, br.Status, strings.Join(missing, ", "), relDisplayPath(s.Root, art.Path)))
		}
	}
	if degraded {
		notices = append(notices, "witness-absence `done`-gate is running degraded: origin/main could not be resolved, so no brief can be shown to have been closed on THIS branch and every unwitnessed closure is grandfathered to the per-stream NOTICE (witnessNotices). If this is CI, fetch origin/main before the lint step")
	}
	sort.Strings(problems)
	sort.Strings(notices)
	return problems, notices
}

// witnessGateChecks reports briefs whose `verified`/`done` cell is contradicted
// by a failing execution witness in their own Evidence.
func witnessGateChecks(root string, streams []*Stream) (problems, notices []string) {
	grandfathered, baseOK := closedAtBase(root, streams)
	degraded := false
	for _, s := range streams {
		for i := range s.Briefs {
			br := &s.Briefs[i]
			if br.Status != "done" && br.Status != "verified" {
				continue
			}
			art, ok := loadBriefArtifacts(s, br.Num)
			if !ok {
				continue
			}
			if len(briefVerifyRows(art.Verify)) == 0 {
				continue // no Verify table: verifySectionProblems' business
			}
			failed := failedWitnessRows(checkWitnesses(art.Verify, art.Evidence))
			if len(failed) == 0 {
				continue
			}
			id := s.Name + "/brief-" + br.Num
			if !baseOK || grandfathered[s.Name+"/"+br.Num] {
				if !baseOK {
					degraded = true
				}
				notices = append(notices, fmt.Sprintf(
					"%s: %s over Verify row(s) %s whose EXECUTION WITNESS records a failure — pre-existing closure, grandfathered to a NOTICE. The cell is derived from the witness: it reads `implemented` until the row passes again, and the re-baseline belongs in the PR that turned it red (brief-rule 31)",
					id, br.Status, strings.Join(failed, ", ")))
				continue
			}
			problems = append(problems, fmt.Sprintf(
				"%s: cannot close as %s — the brief's own Evidence carries an EXECUTION WITNESS recording a FAILURE for Verify row(s) %s, so the cell contradicts the record it rests on. Either fix the work and re-run `statusgen verifyrun --brief %s` (runs APPEND; the red run stays), or set the Status cell to `implemented` — `verified` is derived from the witness, not asserted (brief-rule 30)",
				id, br.Status, strings.Join(failed, ", "), relDisplayPath(s.Root, art.Path)))
		}
	}
	if degraded {
		notices = append(notices, "witness `done`-gate is running degraded: origin/main could not be resolved, so no brief can be shown to have been closed on THIS branch and every contradicted cell is grandfathered to a NOTICE. If this is CI, fetch origin/main before the lint step")
	}
	sort.Strings(problems)
	sort.Strings(notices)
	return problems, notices
}

// failedWitnessRows lists the row IDs whose audit verdict is `fail`, rendered
// "#1, #4".
//
// `fail` ONLY. checkWitnesses returns could-not-run for a row with no witness,
// a stale witness, and a witness that recorded could-not-run — three different
// kinds of "nothing is established here", none of which is a contradiction of
// the cell. Folding them in would turn this check into a second copy of
// witnessNotices with a harder severity, which is the alarm-duplication shape
// this codebase already rejects elsewhere.
func failedWitnessRows(findings []checkFinding) []string {
	var ids []string
	for _, f := range findings {
		if f.State == stateFail {
			ids = append(ids, "#"+f.ID)
		}
	}
	return ids
}

// failFirstGateChecks is the fail-first closure gate (verify-integrity/03).
//
// PROBLEM, on a closure THIS branch makes (verified/done, not so at the
// merge-base), for each RISK-BEARING Verify row (brief-wide risk `yes`, or a
// risk-bearing|live|mutating|end-to-end row tag) whose LATEST witness for the
// row (`verifyrun --fail-first`, the Base cell):
//   - is not a fail-first record — the row was never shown to fail;
//   - was run for a different command than the row now carries — an edit after
//     the run must not keep a fail-first record (the rule checkWitnesses applies
//     to ordinary witnesses);
//   - is `unproven` — it could not run at base, so it was never shown to fail;
//   - names a base that is neither the merge-base of HEAD and the default
//     branch nor an ancestor of it — the red came from some other tree, HEAD's
//     own commit, or a commit of this branch (the SPOF layer: see failfirst.go);
//   - names a base that predates the brief (the merge-base holds the brief file
//     at its current path and the base does not), so the Verify rows did not
//     exist on that tree — a red there is not about this change; or, when the
//     merge-base does not hold the brief at its current path (created or
//     renamed on the branch), names anything but the merge-base itself;
//   - is green at base — `non-discriminating`: it passes with or without the
//     change.
//
// Rows are enumerated by briefVerifyRows, the SAME parser the runner uses, so no
// row the runner executes and witnesses escapes the gate; text in the Verify
// section that parser cannot place (verifyUnplacedRows) is itself a PROBLEM on
// such a closure.
//
// A NON-risk-bearing row is audited too, but non-discrimination there is a
// NOTICE, never a PROBLEM. Closures already at the merge-base are
// grandfathered silently (they predate the gate). With no resolvable base the
// gate cannot tell a new closure from an old one, so it holds nothing and says
// it is degraded.
func failFirstGateChecks(root string, streams []*Stream) (problems, notices []string) {
	grandfathered, baseOK := closedAtBase(root, streams)
	degraded := false
	for _, s := range streams {
		for i := range s.Briefs {
			br := &s.Briefs[i]
			if br.Status != "done" && br.Status != "verified" {
				continue
			}
			art, ok := loadBriefArtifacts(s, br.Num)
			if !ok {
				continue
			}
			plan, refusal := protectedAcceptance(art.Verify, art.Evidence, art.Path)
			if refusal != "" {
				continue // protectedClosureProblems reports the same refusal.
			}
			rows := briefVerifyRows(plan.Verify)
			unplaced := verifyUnplacedRows(plan.Verify)
			if len(rows) == 0 && len(unplaced) == 0 {
				continue // no Verify table: verifySectionProblems' business
			}
			risky := failFirstRiskyIDs(art.Risk, rows)
			if !baseOK {
				if len(risky) > 0 {
					degraded = true
				}
				continue
			}
			if grandfathered[s.Name+"/"+br.Num] {
				continue
			}
			p, n := failFirstClosureChecks(root, s.Name+"/brief-"+br.Num, br.Status, art, plan)
			problems = append(problems, p...)
			notices = append(notices, n...)
		}
	}
	if degraded {
		notices = append(notices, "fail-first `done`-gate is running degraded: origin/main could not be resolved, so no brief can be shown to have been closed on THIS branch and no risk-bearing row is held to a fail-first witness. If this is CI, fetch origin/main before the lint step")
	}
	sort.Strings(problems)
	sort.Strings(notices)
	return problems, notices
}

// failFirstClosureChecks is the shared per-brief fail-first validator used by
// lint (and therefore deskevidence flip) and brief --check-verified. Callers
// decide whether a closure is inherited; this function never grandfathers it.
// plan must come from protectedAcceptance so supplemental rows get no credit.
func failFirstClosureChecks(root, id, status string, art briefArtifacts, plan protectedPlan) (problems, notices []string) {
	rows := briefVerifyRows(plan.Verify)
	unplaced := verifyUnplacedRows(plan.Verify)
	risky := failFirstRiskyIDs(art.Risk, rows)
	mergeBase, mergeBaseDone := "", false
	for _, u := range unplaced {
		problems = append(problems, fmt.Sprintf("%s: cannot close as %s — the Verify section holds %s that `statusgen verifyrun` cannot place, so it is never run, witnessed or held to a fail-first record. Give the table a `Command` column (and a Command cell on every row)", id, status, u))
	}
	evidence := parseEvidenceRows(plan.Evidence)
	for _, r := range rows {
		ff, found, stale := latestFailFirst(evidence[r.ID], r.Command)
		if !risky[r.ID] {
			if found && ff.State == baseGreen {
				notices = append(notices, fmt.Sprintf("%s: Verify row #%s is non-discriminating — green at base %s and at head; it is not risk-bearing, so this is an audit NOTICE, not a closure block", id, r.ID, ff.Base))
			}
			continue
		}
		fact := evaluation.FailFirst{Required: true, Found: found, Stale: stale, State: ff.State}
		if found && !stale && ff.State != baseUnproven {
			if !mergeBaseDone {
				mergeBase, mergeBaseDone = failFirstMergeBase(root), true
			}
			if ok, why := commitIsAncestor(root, ff.Base, mergeBase); !ok {
				fact.AncestryProblem = why
			} else {
				fact.HeldProblem = briefHeldAtBase(root, art.Path, ff.Base, mergeBase)
			}
		}
		problem := evaluation.FailFirstProblem(fact)
		switch {
		case problem == "stale":
			problems = append(problems, fmt.Sprintf("%s: cannot close as %s — risk-bearing Verify row #%s: its latest witness records a different command than the row now carries, so any fail-first record before it is about an earlier command. Re-run `statusgen verifyrun --fail-first --brief %s` and commit the witness table (its Base cell) before closing", id, status, r.ID, relDisplayPath(root, art.Path)))
		case problem == "missing":
			problems = append(problems, fmt.Sprintf("%s: cannot close as %s — risk-bearing Verify row #%s has no fail-first witness: its latest witness carries no Base cell, so nothing shows it reds without the change. Run `statusgen verifyrun --fail-first --brief %s` and commit the witness table (its Base cell) before closing", id, status, r.ID, relDisplayPath(root, art.Path)))
		case problem == "unproven":
			problems = append(problems, fmt.Sprintf("%s: cannot close as %s — risk-bearing Verify row #%s could not run at base %s, so it was never shown to fail. Make the row runnable on the base tree, or re-author it, and re-run `statusgen verifyrun --fail-first`", id, status, r.ID, ff.Base))
		default:
			if why := fact.AncestryProblem; problem == "ancestry" {
				problems = append(problems, fmt.Sprintf("%s: cannot close as %s — risk-bearing Verify row #%s: its fail-first base is not the merge-base or an ancestor of it (base %s: %s), so its %s is not about the tree without this change. Re-run `statusgen verifyrun --fail-first` (default base, or --base <an ancestor of the merge-base>)", id, status, r.ID, ff.Base, why, ff.State))
				continue
			}
			if why := fact.HeldProblem; problem == "held" {
				problems = append(problems, fmt.Sprintf("%s: cannot close as %s — risk-bearing Verify row #%s: its fail-first base %s is not about this change (%s). Re-run `statusgen verifyrun --fail-first --base <a later ancestor of the merge-base>`", id, status, r.ID, ff.Base, why))
				continue
			}
			if problem == "green" {
				problems = append(problems, fmt.Sprintf("%s: cannot close as %s — risk-bearing Verify row #%s is non-discriminating: green at base %s and at head, so it passes with or without the change. Strengthen the row until it reds on the base (docs/verify-row-strength.md), then re-run `statusgen verifyrun --fail-first`", id, status, r.ID, ff.Base))
			}
		}
	}
	return problems, notices
}

// latestFailFirst reads a row's LATEST witness and answers whether it is a
// usable fail-first record for command (the row's current Verify command):
//   - stale is true when that witness was run for a different command;
//   - found is true when it is for this command and carries a Base cell that
//     actually ran at base (a not-selected cell says nothing about it).
//
// An older record is never consulted once a newer witness exists: an edit after
// the run, or a plain re-run after a fail-first run, must not keep the earlier
// record alive.
func latestFailFirst(rows []evidenceRow, command string) (ff failFirst, found, stale bool) {
	latest := ""
	for _, r := range rows {
		if isWitnessRow(r.Text) {
			latest = r.Text
		}
	}
	if latest == "" {
		return failFirst{}, false, false
	}
	if normalizeCommandText(witnessCommandOf(latest)) != normalizeCommandText(command) {
		return failFirst{}, false, true
	}
	f, ok := failFirstOf(latest)
	if !ok || f.State == baseNotSelected {
		return failFirst{}, false, false
	}
	return f, true, false
}
