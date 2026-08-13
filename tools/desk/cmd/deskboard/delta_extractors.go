package main

// delta_extractors.go — the per-subcommand normalization of a report into a
// deltaSet. Each extractor runs on the ALREADY-BUILT report value
// (no network), picking the identity fields, the tracked fields whose change matters,
// and a one-line Display for the delta rendering.
//
// Identity is stable across runs: repo+number for PRs/issues, repo+stream+brief for
// nextup. The signature captures every field whose change a desk sweep would want
// flagged; a field that should NOT re-surface (e.g. a transient ordering) is omitted.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// prsDeltaSet normalizes a prsReport. ID = repo#number; signature captures title,
// draft, head, merge-state, and CI counts — the fields a sweep quotes.
func prsDeltaSet(v any) (deltaSet, bool) {
	rep, ok := v.(prsReport)
	if !ok {
		return deltaSet{}, false
	}
	items := make([]deltaItem, 0, len(rep.PRs)+len(rep.External))
	for _, r := range rep.PRs {
		items = append(items, deltaItem{
			ID:        fmt.Sprintf("%s#%d", r.Repo, r.Number),
			Signature: fmt.Sprintf("%s|%t|%s|%s|%d/%d/%d", r.Title, r.Draft, r.HeadSHA, r.MergeState, r.CIPass, r.CIPending, r.CIFail),
			Display:   fmt.Sprintf("%s #%-4d %-6t %s  ci %d✓ %dpend %dfail  %s", shortRepo(r.Repo), r.Number, r.Draft, short(r.HeadSHA), r.CIPass, r.CIPending, r.CIFail, title(r.Title, 46)),
		})
	}
	// Quarantined (external) items ride the delta too — a NEWLY quarantined PR is a
	// change a desk wants to see — with a distinct marker so they read as advisory.
	for _, e := range rep.External {
		items = append(items, deltaItem{
			ID:        fmt.Sprintf("%s#%d", e.Repo, e.Number),
			Signature: "quarantined|" + e.Author + "|" + e.Title,
			Display:   fmt.Sprintf("%s #%-4d (quarantined) %s", shortRepo(e.Repo), e.Number, title(e.Title, 46)),
		})
	}
	draft, attention := 0, 0
	for _, r := range rep.PRs {
		if r.Draft {
			draft++
		}
		if prNeedsAttention(r) {
			attention++
		}
	}
	summary := fmt.Sprintf("%d open (%d draft)", len(rep.PRs), draft)
	return deltaSet{
		Items:   items,
		Summary: summary,
		// NOT len(rep.PRs): that restated the summary's own first number (review
		// Minor 1). The `prs` payload carries no review state, so it cannot compute
		// the ACTION class — that is `deskboard actions`. It CAN prove CI-red and
		// un-mergeable, so the segment counts exactly that and SAYS so, rather than
		// printing a bare "N actionable" that a desk could read as "and the other
		// rows are fine" (under-claiming here would be quiet-by-accident).
		Actionable:      attention,
		ActionableLabel: "ci-red/conflicting (see `actions` for the ACTION class)",
		RepoSet:         deskkit.AllowedRepos(),
	}, true
}

// prNeedsAttention is the subset of "needs the desk now" that the `prs` payload
// alone can PROVE: a failed check, or a merge state that makes the PR un-mergeable
// (DIRTY/BLOCKED — the same pair board.go's classifier treats as mergeConflict).
// Deliberately narrow and deliberately labelled: the full classification needs the
// review + security-verdict reads that only `deskboard actions` performs.
func prNeedsAttention(r prRow) bool {
	return r.CIFail > 0 || r.MergeState == "DIRTY" || r.MergeState == "BLOCKED"
}

// actionsDeltaSet normalizes an actionsReport — the CLASSIFIED board. This is the
// extractor that makes the desk's quiet cadence loop review-RESPONSIVE: before it,
// `--delta`/`--quiet` existed only on verbs that carry no review state (`prs`
// deliberately labels its count "ci-red/conflicting … see `actions`"), and `actions`
// itself refused both flags (exit 5) — so a desk observing the console noise floor had
// NO quiet sweep that could say "a PR needs a reviewer". The first loud signal it got
// was the UNREVIEWED neglect banner at the threshold age, and a 2h neglect alarm had
// become the de-facto review trigger. With this extractor, one
// `deskboard actions --delta --quiet` per cadence tick surfaces a fresh NEEDS-REVIEW
// row within one tick, at any age.
//
// Actionable = NEEDS-REVIEW + RE-REVIEW — the pr-review-desk dispatch gate (its idle
// gate reads exactly these two buckets), labelled so the count can never degenerate
// into a bare len(rows).
//
// The SUMMARY always restates the standing counts of the decision-critical buckets
// (NEEDS-REVIEW / RE-REVIEW / MERGE-NOW / FLIP) plus the UNREVIEWED and DECAY alarms —
// every sweep, changed or not. That is deliberate: --delta silences an UNCHANGED row
// after its first sighting, and MERGE-NOW/alarm visibility is a standing duty (mm/20),
// so the counts ride the quiet line itself rather than depending on a row re-surfacing.
func actionsDeltaSet(v any) (deltaSet, bool) {
	rep, ok := v.(actionsReport)
	if !ok {
		return deltaSet{}, false
	}
	items := make([]deltaItem, 0, len(rep.Rows)+len(rep.External))
	byAction := map[string]int{}
	for _, r := range rep.Rows {
		byAction[r.Action]++
		items = append(items, deltaItem{
			ID: fmt.Sprintf("%s#%d", r.Repo, r.Number),
			// Age fields (ApprovedAge/OpenAge), Score and Note are EXCLUDED on
			// purpose: ages tick on every sweep and score/note re-derive from state
			// already captured here, so including any of them would mark every row
			// "changed" each run — the delta channel would become the full board
			// again and the noise discipline would be lost by accident.
			Signature: fmt.Sprintf("%s|%t|%d/%d/%d/%d|%s|risk=%t|sec=%t|hg=%t",
				r.Action, r.Draft, r.CIPass, r.CIPending, r.CIFail, r.CIUnknown,
				r.CIZero, r.RiskClassed, r.SecurityPass, r.HumanGate),
			Display: fmt.Sprintf("%s #%-4d %-24s ci %d✓ %dpend %dfail  %s",
				shortRepo(r.Repo), r.Number, r.Action, r.CIPass, r.CIPending, r.CIFail, title(r.Title, 46)),
		})
	}
	for _, e := range rep.External {
		items = append(items, deltaItem{
			ID:        fmt.Sprintf("%s#%d", e.Repo, e.Number),
			Signature: "quarantined|" + e.Author + "|" + e.Title,
			Display:   fmt.Sprintf("%s #%-4d (quarantined) %s", shortRepo(e.Repo), e.Number, title(e.Title, 46)),
		})
	}
	dispatch := byAction[actNeedsReview] + byAction[actReReview]
	// Fixed bucket order for a deterministic line; zero buckets are omitted, the
	// remainder is aggregated so the line stays one line on a wide board.
	named := 0
	var parts []string
	for _, a := range []string{actNeedsReview, actReReview, actMergeNow, actFlip} {
		if n := byAction[a]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, a))
			named += n
		}
	}
	if rest := len(rep.Rows) - named; rest > 0 {
		parts = append(parts, fmt.Sprintf("%d other", rest))
	}
	summary := fmt.Sprintf("%d classified", len(rep.Rows))
	if len(parts) > 0 {
		summary += " (" + strings.Join(parts, ", ") + ")"
	}
	// Standing alarms ride the summary EVERY sweep (never delta-gated): the neglect
	// alarm (#359) and its unmeasurable lane, and the MERGE-NOW decay alarm. Their
	// absence when the counts are zero is the quiet board paying nothing.
	if n := rep.Header.UnreviewedCount; n > 0 {
		summary += fmt.Sprintf(" · UNREVIEWED %d (neglect alarm >%s — the trigger path missed these)", n, rep.Header.UnreviewedThreshold)
	}
	if n := rep.Header.UnreviewedAgeUnknownCount; n > 0 {
		summary += fmt.Sprintf(" · UNREVIEWED-AGE-UNKNOWN %d", n)
	}
	if rep.Header.MergeNowDecay {
		summary += fmt.Sprintf(" · DECAY %d (approved >%s, unmerged)", len(rep.Header.MergeNowDecayPRs), rep.Header.MergeNowThreshold)
	}
	return deltaSet{
		Items:           items,
		Summary:         summary,
		Actionable:      dispatch,
		ActionableLabel: "NEEDS-REVIEW/RE-REVIEW (dispatch a reviewer NOW — a fresh PR is actionable at any age)",
		RepoSet:         deskkit.AllowedRepos(),
	}, true
}

// queueDeltaSet normalizes a queueReport. ID = repo#number; signature captures
// title + sorted labels (a label change — e.g. verify-gate added/removed — is a
// field change worth surfacing).
func queueDeltaSet(v any) (deltaSet, bool) {
	rep, ok := v.(queueReport)
	if !ok {
		return deltaSet{}, false
	}
	items := make([]deltaItem, 0, len(rep.Issues)+len(rep.External))
	for _, is := range rep.Issues {
		labels := append([]string(nil), is.Labels...)
		sort.Strings(labels)
		items = append(items, deltaItem{
			ID:        fmt.Sprintf("%s#%d", is.Repo, is.Number),
			Signature: fmt.Sprintf("%s|labels=%s", is.Title, strings.Join(labels, ",")),
			Display:   fmt.Sprintf("%s #%-5d %s", shortRepo(is.Repo), is.Number, title(is.Title, 60)),
		})
	}
	for _, e := range rep.External {
		items = append(items, deltaItem{
			ID:        fmt.Sprintf("%s#%d", e.Repo, e.Number),
			Signature: "quarantined|" + e.Author + "|" + e.Title,
			Display:   fmt.Sprintf("%s #%-5d (quarantined) %s", shortRepo(e.Repo), e.Number, title(e.Title, 60)),
		})
	}
	summary := fmt.Sprintf("%d verify-gate", len(rep.Issues))
	return deltaSet{
		Items:      items,
		Summary:    summary,
		Actionable: len(rep.Issues),
		RepoSet:    deskkit.AllowedRepos(),
	}, true
}

// nextupDeltaSet normalizes a nextupReport. ID = repo|stream|brief; signature
// captures status, score, and blocked-count. Summary breaks out the actionable
// statuses (todo + in-progress) the desk picks from.
func nextupDeltaSet(v any) (deltaSet, bool) {
	rep, ok := v.(nextupReport)
	if !ok {
		return deltaSet{}, false
	}
	items := make([]deltaItem, 0, len(rep.Rows))
	todo, inprog := 0, 0
	for _, r := range rep.Rows {
		items = append(items, deltaItem{
			ID:        fmt.Sprintf("%s|%s|%s", r.Repo, r.Stream, r.Brief),
			Signature: fmt.Sprintf("%s|%d|%d", r.Status, r.Score, r.BlockedCount),
			Display:   fmt.Sprintf("%s %-22s %-6d %-14s %s", shortRepo(r.Repo), trunc(r.Stream, 22), r.Score, r.Status, r.Brief),
		})
		switch r.Status {
		case "todo":
			todo++
		case "in-progress":
			inprog++
		}
	}
	// Repo set = the configured roots this board merged from (stable per run config).
	repoSet := make([]string, 0, len(rep.Roots))
	for _, r := range rep.Roots {
		repoSet = append(repoSet, r.Repo)
	}
	summary := fmt.Sprintf("%d awaiting (%d todo, %d in-progress)", len(rep.Rows), todo, inprog)
	return deltaSet{
		Items:      items,
		Summary:    summary,
		Actionable: todo + inprog,
		RepoSet:    repoSet,
	}, true
}
