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
	"fmt"
	"sort"
	"strings"
)

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
