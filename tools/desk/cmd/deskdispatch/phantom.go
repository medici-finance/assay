package main

import (
	"fmt"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// phantom.go — the pre-claim PHANTOM CHECK.
//
// THE DEFECT IT CLOSES. A fresh worker dispatch derives its branch as `feat/<stream>-<NN>`
// (sanitizeSegment of the item key), but a live PR for the same item is branched under the
// claim-key form, `feat/<repo>--<stream>--<NN>`. A branch-name existence check keys on the derived
// spelling, finds nothing, and lets the dispatch proceed — so a worker is spawned for an item that
// already has an OPEN or MERGED PR, re-derives that the PR exists at a large token cost, and returns
// no new work. This check reconciles the candidate against the repo's open+merged PRs by the item's
// IDENTITY — the PR body's `Brief:` link trailer — BEFORE the durable claim is taken, so a branch
// spelled differently from the derived pattern can no longer hide a phantom.
//
// SCOPE. Only a FRESH implementation dispatch is a phantom candidate: `--kit worker` with no `--pr`.
// A review or verifier dispatch, and a `--pr` resume, act ON an existing PR by design — a PR
// existing for them is not a phantom, so the check does not apply.
//
// TRANSPORT. listRepresentedPRs is a package var, nil by default. Nil = the check is NOT wired: the
// OFFLINE reference build performs no forge read, exactly as fanoutloop's orphan sweep is wired only
// at the live cutover. This keeps the CLOSED forge surface closed — deskdispatch ships no forge-CLI
// call — while the live wiring supplies a transport built on a typed forge op (never a raw CLI), and
// tests drive the check against a recorded PR list. A refusal here wedges nothing (no claim yet), and
// a PR list the transport could not read is UNVERIFIABLE, never rounded to "no PR exists":
// could-not-check is not the same answer as clear, and treating it as clear is how a phantom slips
// through.
var listRepresentedPRs func(repo string) ([]deskkit.PRRef, error)

// briefIDFromItem reduces a dispatch item key to the brief id `<stream>/<NN>` a PR's `Brief:`
// trailer carries, so the two can be compared. It accepts every item-key form deskdispatch is
// handed:
//
//   - a repo-qualified plan key `<owner>/<name>:<stream>/<NN>` — the `<repo>:` prefix is dropped;
//   - a slash plan key `<stream>/<NN>` — returned as-is;
//   - a claim key `<short>--<stream>--<NN>` (or `<repo>--<stream>--<NN>`) — the `--` segments are
//     split and the LAST TWO (stream, NN) are joined by `/`, so a stream carrying single dashes
//     survives, matching claimKeyFor's own `/`→`--` mapping in reverse.
//
// It returns "" when the key does not reduce to a `<stream>/<NN>` (e.g. an `issue-<NN>` placeholder
// carries an `Issue:` link, not a `Brief:` one, and a bare `item-1` names no brief) — the caller
// treats an empty id as "nothing to reconcile" and does not apply the check.
func briefIDFromItem(item string) string {
	s := strings.TrimSpace(item)
	if i := strings.LastIndex(s, ":"); i >= 0 {
		s = s[i+1:]
	}
	s = strings.Trim(s, "/")
	if s == "" {
		return ""
	}
	if strings.Contains(s, "/") {
		return s
	}
	if strings.Contains(s, "--") {
		parts := strings.Split(s, "--")
		if len(parts) >= 2 {
			stream := parts[len(parts)-2]
			num := parts[len(parts)-1]
			if stream != "" && num != "" {
				return stream + "/" + num
			}
		}
	}
	return ""
}

// phantomCheck refuses a fresh worker dispatch whose brief already has an OPEN or MERGED PR. It is a
// no-op for anything that is not a fresh implementation dispatch, for an item that does not reduce to
// a brief id, and when no PR-list transport is wired (the offline reference build).
func phantomCheck(o dispatchOpts, repo string) error {
	if o.kit != "worker" || o.pr != 0 {
		return nil
	}
	if listRepresentedPRs == nil {
		return nil
	}
	briefID := briefIDFromItem(o.item)
	if briefID == "" {
		return nil
	}
	prs, err := listRepresentedPRs(repo)
	if err != nil {
		return deskkit.Unverifiable(fmt.Sprintf(
			"step %s: could not read %s's open+merged PRs to check whether %s already has one (%v). "+
				"Could-not-check is not no-PR-exists, so the dispatch is HELD rather than risk spending a "+
				"worker on a phantom row — nothing was claimed; retry.",
			stepClaimAcquire, repo, briefID, err), err)
	}
	if n, ok := deskkit.BriefRepresentedPR(briefID, prs); ok {
		return deskkit.Refused(fmt.Sprintf(
			"step %s: %s is already represented by %s#%d — matched on that PR's `Brief: %s` trailer, not on "+
				"a branch name, so a branch spelled differently from this verb's derived `feat/%s` cannot hide "+
				"it. Dispatching would spawn a worker that re-derives the PR already exists and returns no new "+
				"work. Resume the PR (--pr %d) or close it; do not fresh-dispatch over it.",
			stepClaimAcquire, briefID, repo, n, briefID, sanitizeSegment(o.item), n))
	}
	return nil
}
