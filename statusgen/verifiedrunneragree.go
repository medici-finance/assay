package main

// verifiedrunneragree.go — the VERIFIED-CELL / EVIDENCE-RUNNER agreement notice.
//
// THE HOLE IT CLOSES. A stream board carries ONE `Verified` cell per brief —
// "YYYY-MM-DD <runner>", naming who ran the Verify table — and the brief's
// `## Evidence` section records, per row, who actually ran each check. The two
// are meant to name the SAME actor. They drift when a Verify table is legitimately
// RE-RUN: a checkpoint PR gets CHANGES_REQUESTED, a shepherd fixes the findings,
// and because its own fix changed the artifact a Verify row measures, the shepherd
// re-runs the rows and stamps the Evidence table with ITS OWN identity and date.
// But nobody rewrites the register's `Verified` cell, so the cell keeps naming the
// ORIGINAL non-implementer verifier at the original date.
//
// The result is a register cell and an Evidence table that disagree about who
// verified — and the disagreement points the UNSAFE way: the cell claims a
// non-implementer ran the table, while the table records that the implementing
// shepherd re-ran the rows now standing as that verification. A reader asking "was
// this verified by someone who did not build it?" reads the cell, sees a verifier
// name, and stops; the rows underneath say otherwise. The re-run itself is
// honest — the old Evidence was stale and re-running is the right thing — the
// defect is that the attribution did not travel with it.
//
// THE MECHANISM. For every `verified`/`done` brief-v1 brief this reads the actor
// the register CREDITS (the Verified cell's runner token) and the actor the
// Evidence table RECORDS (the runner who ran a strict MAJORITY of the brief's own
// rows), and NOTICEs when they name different people. It is the disposition the
// live instance's issue calls "mechanical, catches it at the source, no judgement
// call": it never guesses the right runner, only reports that the cell's runner is
// not the one who ran most of the rows it is credited for.
//
// WHICH EVIDENCE ROWS COUNT, AND WHY. Only rows that are COMPLETE
// (evidenceRowComplete: a date and a runner, not marked UNRUN) and whose runner was
// read from a DECLARED `Runner` column (RunnerFromColumn) participate — the same
// two guards evidenceFloorFailure applies, and for the same reason: a free-text
// output cell misread as a runner would both manufacture false disagreements and
// launder real ones. A row that appears in more than one table (an implementer run,
// then an independent re-run) is credited to its LATEST-dated completed entry — the
// actor who most recently ran it — because that is exactly the runner the cell is
// supposed to have tracked.
//
// SEVERITY: NOTICE. This is the same F-verify-self-attest family as
// evidenceActorNotices, and ships at the same severity for the same reason: the
// register schema historically had no place to record a re-run's actor, so a
// backlog of drifted cells predates the check, and arming a hard PROBLEM against it
// would red unrelated PRs. It also cannot be a PROBLEM safely because the two
// columns use different vocabularies (the cell a model-tier slug like
// `opus-verifier`, the Evidence a bot login like `assay-worker-app[bot]` or a human
// stamp), so a residual false-positive is possible; a NOTICE surfaces the record
// disagreement without gating on a name comparison across two namespaces. It
// changes no exit code and weakens no existing verification-integrity assertion —
// the verifier floor, the Evidence-independence gate, and the Evidence-actor blame
// check are all untouched; this only names a drift none of them was looking for.
//
// NOT IN SCOPE. Rewriting the cell is not this check's job: the lifecycle columns
// are PRESERVED, not derived, on the offline render path (readmetable.go's interim
// ruling), and deriving the runner attribution would need the same online reconcile
// the Status cell's DeriveLifecycle does. This check makes the drift VISIBLE so the
// re-runner updates the cell to match; the update itself stays a human/tool edit.

import (
	"fmt"
	"sort"
	"strings"
)

// runnerKey normalizes a runner label to a comparison key: lowercased, with a
// trailing `[bot]` suffix and any parenthetical qualifier dropped, so
// "Opus-Verifier", "opus-verifier (non-implementer)" and "opus-verifier[bot]" all
// key to the same "opus-verifier". The parenthetical is dropped because the
// Evidence convention decorates a runner with a role note
// ("sonnet-verifier (non-implementer)") that the bare Verified-cell token does not
// carry.
func runnerKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if i := strings.IndexByte(s, '('); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	s = strings.TrimSpace(strings.TrimSuffix(s, "[bot]"))
	return s
}

// runnersAgree reports whether a Verified-cell runner and an Evidence runner name
// the same actor, tolerant of the two columns' different vocabularies. It matches
// on normalized equality OR containment either way — so a cell `sonnet-verifier`
// agrees with an Evidence `sonnet (non-implementer)` (one key contains the other),
// while `k3-verifier` and `assay-worker-app` (neither contains the other) do not.
// An empty key on either side yields AGREE: with nothing to compare, this notice
// must not manufacture a disagreement.
func runnersAgree(cell, evidence string) bool {
	a, b := runnerKey(cell), runnerKey(evidence)
	if a == "" || b == "" {
		return true
	}
	return a == b || strings.Contains(a, b) || strings.Contains(b, a)
}

// evidenceRunnerTally counts, over the COMPLETED Evidence rows that name a DECLARED
// `Runner` column, how many rows each distinct runner ran, and returns the DOMINANT
// runner (the most rows), a representative raw label for it, its count, and the
// total number of such rows. ok is false when there is no completed, declared-runner
// Evidence to compare against — the could-not-compare state, kept out of any
// verdict rather than rounded to a false agreement.
//
// A row credited across several tables is counted for its LATEST-dated completed
// declared-runner entry only, so a re-run's runner — not the original — is the one
// tallied. That is the whole point: the tally names who MOST RECENTLY ran each row,
// which is the actor the Verified cell is supposed to track.
func evidenceRunnerTally(evidence string) (dominantRaw string, dominantCount, total int, ok bool) {
	rows := parseEvidenceRows(evidence)
	counts := map[string]int{}
	raw := map[string]string{}
	for _, ers := range rows {
		var best evidenceRow
		have := false
		for _, er := range ers {
			if !evidenceRowComplete(er) || !er.RunnerFromColumn {
				continue
			}
			if !have || er.Date > best.Date {
				best, have = er, true
			}
		}
		if !have {
			continue
		}
		k := runnerKey(best.Runner)
		if k == "" {
			continue
		}
		counts[k]++
		if raw[k] == "" {
			raw[k] = strings.TrimSpace(best.Runner)
		}
		total++
	}
	if total == 0 {
		return "", 0, 0, false
	}
	bestKey := ""
	for k, c := range counts {
		if bestKey == "" || c > counts[bestKey] || (c == counts[bestKey] && k < bestKey) {
			bestKey = k
		}
	}
	return raw[bestKey], counts[bestKey], total, true
}

// verifiedRunnerDisagreementNotices is the `--lint` arm: one NOTICE per
// `verified`/`done` brief-v1 brief whose Verified cell credits a runner other than
// the actor who ran a strict MAJORITY of its Evidence rows. Offline, tree-only,
// deterministic — the same iteration and schema opt-in as attributionProblems.
func verifiedRunnerDisagreementNotices(streams []*Stream) []string {
	var notices []string
	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				continue
			}
			_, num, okName := expectedBriefID(path)
			if !okName {
				continue
			}
			var row *Brief
			for i := range s.Briefs {
				if s.Briefs[i].Num == num {
					row = &s.Briefs[i]
					break
				}
			}
			if row == nil || (row.Status != "verified" && row.Status != "done") {
				continue
			}
			m := verifiedTokenRe.FindStringSubmatch(row.Verified)
			if m == nil {
				continue // no dated runner — attributionProblems already PROBLEMs this
			}
			cellRunner := m[1]

			dominantRaw, dcount, total, ok := evidenceRunnerTally(bf.Evidence)
			if !ok {
				continue // no declared-runner Evidence rows to compare against
			}
			// Speak only when ONE runner ran a strict majority: a mixed table with
			// no majority runner has no single actor for the cell to disagree with,
			// and reporting it would be a judgement call this check refuses to make.
			if dcount*2 <= total {
				continue
			}
			if runnersAgree(cellRunner, dominantRaw) {
				continue // the cell names the actor who ran the rows — the record agrees
			}
			notices = append(notices, fmt.Sprintf(
				"verified-runner-attribution: %s/brief-%s Verified cell credits %q, but its ## Evidence "+
					"records %q as the runner on %d of %d rows — the register cell and the Evidence table "+
					"disagree about who (re-)ran the Verify rows, and the disagreement points the unsafe way "+
					"(the cell names one actor; the rows record another, e.g. after a shepherd re-ran the "+
					"table to fix review findings). A re-run is legitimate, but its attribution must travel "+
					"with it: update the Verified cell to name the actor who actually (re-)ran the rows so the "+
					"cell matches the Evidence (F-verify-self-attest family).",
				s.Name, num, cellRunner, dominantRaw, dcount, total))
		}
	}
	sort.Strings(notices)
	return notices
}
