package main

// crossroot.go — the cross-root delivery witness.
//
// A brief's file lives in one root, but the PR that delivers it can merge in a
// different configured root (the deliverable is code in another repo). That
// PR's `Brief: <stream>/<NN>` trailer is the same proof of delivery it would be
// in the brief's own repo. Reading only the board's own merge history missed
// it: the Next-up planner went on offering the delivered row, and reconcile
// found no witness to write it with.
//
// DEFECT CLASS: a consumer of merged-delivery evidence that reads its own
// root's history only. There are two consumer shapes and each now has ONE
// entry point that also reads every other configured root:
//
//   - the planner hold (STATUS.md, --next-up, the roadmap deck) reads merged
//     PRs through deliveryHistory, never mergedPRsFromGit directly —
//     TestDeliveryReadChokePoint pins that only this file calls it;
//   - reconcile and the drift comparator admit PR witnesses through
//     admitPRWitnesses (briefauthoring.go), which appends crossRootPRRecords.
//
// WHICH ROOTS. The configured roots are the operator's map — DESK_ROOTS and
// --sibling-root, the same map the sibling-merge detector reads — never a value
// the scanned tree supplies. The board's own root (same path, or the same
// <owner>/<repo>) is skipped: its merges are already read as its own.
//
// WHAT COUNTS. Only a `Brief:` trailer on a merged PR. A subject, branch or body
// mention in another repo is not delivery (it is a lead, which is what the opt-in
// sibling-merge detector reports), and an `Authors:` trailer is never delivery.
// A trailer naming a stream the other root itself carries under docs/streams/
// is that root's own business and is ignored here.
//
// THREE-STATE. A configured root that cannot be read (absent, not a git
// checkout, shallow, git failing) yields one could-not-check line naming it,
// and no row is held or written on its account. It never fails the board's own
// read: rows the readable roots witness are still held.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// deliveryHistory is the ONE reader of merged-delivery evidence for the planner:
// the board's own first-parent merges, followed by every other configured
// root's merged `Brief:`-trailer PRs. err is the own-root read's verdict
// (errNoGitForMerge for a tree with no .git); when it is non-nil no cross-root
// read is made either, so class 1 is could-not-check as a whole. notices are
// the cross-root could-not-check lines. ownRepo is the board's own
// <owner>/<repo> when known, "" otherwise.
func deliveryHistory(root, ownRepo string) (merged []mergedPR, notices []string, err error) {
	if hasNoGitDir(root) {
		return nil, nil, errNoGitForMerge
	}
	merged, err = mergedPRsFromGit(root)
	if err != nil {
		return nil, nil, err
	}
	cross, notices := crossRootDeliveries(root, ownRepo)
	return append(merged, cross...), notices, nil
}

// crossRootDeliveries reads every OTHER configured root's first-parent merges
// and keeps each merged PR's `Brief:` trailer ids for streams that root does
// not own. Each kept PR carries Repo (the configured <owner>/<repo>) and only
// its trailer ids in Briefs, so no subject mention elsewhere becomes a finding.
// Roots are read in sorted repo order, so the result and the notices are
// deterministic.
func crossRootDeliveries(root, ownRepo string) ([]mergedPR, []string) {
	roots := effectiveSiblingRootOverrides(siblingRootFlagValues)
	repos := make([]string, 0, len(roots))
	for repo := range roots {
		repos = append(repos, repo)
	}
	sort.Strings(repos)
	var out []mergedPR
	var notices []string
	for _, repo := range repos {
		path := roots[repo]
		if !validSiblingRepo(repo) || (ownRepo != "" && strings.EqualFold(repo, ownRepo)) || pathIsRoot(root, path) {
			continue
		}
		cannot := func(why string) {
			notices = append(notices, fmt.Sprintf("could-not-check: cross-root delivery witness: %s (%s): %s — a "+
				"`Brief:` trailer merged there is not seen this run, so no row is held or written on its account.",
				repo, noticeSafe(path), noticeSafe(why)))
		}
		if ok, why := checkSiblingCheckout(path); !ok {
			cannot(why)
			continue
		}
		merged, err := mergedPRsFromGit(path)
		if err != nil {
			cannot(fmt.Sprintf("merge-history read failed: %v", err))
			continue
		}
		for _, pr := range merged {
			var ids []string
			for _, id := range pr.Delivers {
				stream, _, ok := strings.Cut(id, "/")
				if !ok || ownsStream(path, stream) {
					continue
				}
				ids = append(ids, id)
			}
			if len(ids) == 0 {
				continue
			}
			sort.Strings(ids)
			out = append(out, mergedPR{Number: pr.Number, Subject: pr.Subject, Briefs: ids, Delivers: ids, SHA: pr.SHA, Repo: repo})
		}
	}
	return out, notices
}

// ownsStream reports whether the checkout at path carries docs/streams/<stream>.
func ownsStream(path, stream string) bool {
	if stream == "" || strings.ContainsAny(stream, `/\`) || stream == "." || stream == ".." {
		return false
	}
	fi, err := os.Stat(filepath.Join(path, "docs", "streams", stream))
	return err == nil && fi.IsDir()
}

// crossRootPRRecords is reconcile's half: the cross-root deliveries as merged
// PRRecords for the briefs inScope, each carrying its repo, with every merge the
// brief's own `delivery:` list declares partial (resolved against THAT repo)
// marked Partial. boardRoot is "" when no board was found (no partial read).
// The could-not-check lines go to stderr only: an unread root leaves its
// briefs unwritten, which is not a wrong write, so it does not refuse the run.
func crossRootPRRecords(root, ownRepo, boardRoot string, inScope func(key string) bool, stderr io.Writer) []PRRecord {
	merged, notices := crossRootDeliveries(root, ownRepo)
	for _, n := range notices {
		fmt.Fprintf(stderr, "reconcile: %s\n", n)
	}
	byRepo := map[string][]PRRecord{}
	var order []string
	for _, pr := range merged {
		for _, id := range pr.Delivers {
			if !inScope(canonicalBriefKey(id)) {
				continue
			}
			if _, seen := byRepo[pr.Repo]; !seen {
				order = append(order, pr.Repo)
			}
			byRepo[pr.Repo] = append(byRepo[pr.Repo], PRRecord{
				BriefRef: id, Number: pr.Number, State: prMerged, MergeSHA: pr.SHA, Repo: pr.Repo,
			})
		}
	}
	var out []PRRecord
	for _, repo := range order {
		recs := byRepo[repo]
		if boardRoot != "" {
			markDeclaredPartials(recs, readDeclaredPartials(boardRoot, repo), stderr)
		}
		out = append(out, recs...)
	}
	return out
}
