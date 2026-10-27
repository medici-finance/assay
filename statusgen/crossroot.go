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
// Three further conditions, each a way a trailer would otherwise be credited to
// the wrong brief or to no real merge:
//
//   - MERGED. The other root is read at its integration branch
//     (refs/remotes/origin/HEAD, else refs/remotes/origin/main), never at the
//     checkout's HEAD: HEAD can sit on a closed PR's head, an unpushed commit or
//     a contributor's branch, none of which is a merge. A root whose integration
//     branch cannot be resolved is could-not-check.
//   - ONE BRIEF. A merge carrying more than one distinct `Brief:` trailer is not
//     a derivable PR->brief edge (the own-repo rule, singleBriefTrailer), so it
//     witnesses nothing and is reported.
//   - UNAMBIGUOUS. A stream name is a flat per-repo namespace. A trailer naming a
//     stream that ANY other configured root (the merging root included) also
//     carries under docs/streams/ cannot say which board it means, so it is
//     credited to none and reported when this board carries the stream.
//
// THREE-STATE. A configured root that cannot be read (absent, not a git
// checkout, shallow, git failing) yields one could-not-check line naming it,
// and no row is held or written on its account. It never fails the board's own
// read: rows the readable roots witness are still held.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	merged, err = mergedPRsFromGit(root, "HEAD")
	if err != nil {
		return nil, nil, err
	}
	cross, notices := crossRootDeliveries(root, ownRepo)
	return append(merged, cross...), notices, nil
}

// crossRootDeliveries reads every OTHER configured root's first-parent merges at
// its integration branch and keeps each merged PR that carries exactly ONE
// distinct `Brief:` trailer id for a stream no other configured root carries.
// Each kept PR carries Repo (the configured <owner>/<repo>) and only its trailer
// id in Briefs, so no subject mention elsewhere becomes a finding. Roots are read
// in sorted repo order, so the result and the notices are deterministic.
func crossRootDeliveries(root, ownRepo string) ([]mergedPR, []string) {
	roots := effectiveSiblingRootOverrides(siblingRootFlagValues)
	repos := make([]string, 0, len(roots))
	for repo := range roots {
		repos = append(repos, repo)
	}
	sort.Strings(repos)
	var others []string // configured roots other than the board's own, in repo order
	for _, repo := range repos {
		if !validSiblingRepo(repo) || (ownRepo != "" && strings.EqualFold(repo, ownRepo)) || pathIsRoot(root, roots[repo]) {
			continue
		}
		others = append(others, repo)
	}
	// carriers lists the other configured roots carrying docs/streams/<stream>.
	carried := map[string][]string{}
	carriers := func(stream string) []string {
		if c, ok := carried[stream]; ok {
			return c
		}
		var c []string
		for _, repo := range others {
			if ownsStream(roots[repo], stream) {
				c = append(c, repo)
			}
		}
		carried[stream] = c
		return c
	}
	var out []mergedPR
	var notices []string
	for _, repo := range others {
		path := roots[repo]
		cannot := func(why string) {
			notices = append(notices, fmt.Sprintf("could-not-check: cross-root delivery witness: %s (%s): %s — a "+
				"`Brief:` trailer merged there is not seen this run, so no row is held or written on its account.",
				repo, noticeSafe(path), noticeSafe(why)))
		}
		if ok, why := checkSiblingCheckout(path); !ok {
			cannot(why)
			continue
		}
		tip, err := integrationTip(path)
		if err != nil {
			cannot(fmt.Sprintf("integration branch not resolvable: %v", err))
			continue
		}
		merged, err := mergedPRsFromGit(path, tip)
		if err != nil {
			cannot(fmt.Sprintf("merge-history read failed: %v", err))
			continue
		}
		ambiguous := map[string]int{} // stream -> merged PRs dropped as ambiguous in this root
		for _, pr := range merged {
			ids := distinctSorted(pr.Delivers)
			if len(ids) == 0 {
				continue
			}
			if len(ids) > 1 {
				if boardCarriesAny(root, ids) {
					notices = append(notices, fmt.Sprintf("could-not-check: cross-root delivery witness: %s %s carries %d "+
						"`Brief:` trailers (%s); a multi-linked merge is not a derivable delivery edge, so no row is held or "+
						"written on its account.", repo, mergedPR{Number: pr.Number}.prRef(), len(ids), noticeSafe(strings.Join(ids, ", "))))
				}
				continue
			}
			stream, _, ok := strings.Cut(ids[0], "/")
			if !ok {
				continue
			}
			if len(carriers(stream)) > 0 {
				if ownsStream(root, stream) {
					ambiguous[stream]++
				}
				continue
			}
			out = append(out, mergedPR{Number: pr.Number, Subject: pr.Subject, Briefs: ids, Delivers: ids, SHA: pr.SHA, Repo: repo})
		}
		streams := make([]string, 0, len(ambiguous))
		for stream := range ambiguous {
			streams = append(streams, stream)
		}
		sort.Strings(streams)
		for _, stream := range streams {
			notices = append(notices, fmt.Sprintf("could-not-check: cross-root delivery witness: %s: %d merged PR(s) carry a "+
				"`Brief:` trailer for stream %s, which %s also carries — a trailer cannot say which board it means, so none "+
				"is held or written on its account.", repo, ambiguous[stream], noticeSafe(stream), strings.Join(carriers(stream), ", ")))
		}
	}
	return out, notices
}

// integrationTip resolves the full object name of the checkout's integration
// branch tip: the remote-tracking ref origin/HEAD points at, else
// refs/remotes/origin/main. A remote-tracking ref is what the forge last
// confirmed; the checkout's own HEAD is whatever the operator left it on.
func integrationTip(path string) (string, error) {
	ref := "refs/remotes/origin/main"
	if out, err := exec.Command("git", "-C", path, "symbolic-ref", "-q", "refs/remotes/origin/HEAD").Output(); err == nil {
		if t := strings.TrimSpace(string(out)); integrationRefRe.MatchString(t) && !strings.Contains(t, "..") {
			ref = t
		}
	}
	out, err := exec.Command("git", "-C", path, "rev-parse", "--verify", "--quiet", ref+"^{commit}").Output()
	sha := strings.TrimSpace(string(out))
	if err != nil || !mergeSHARe.MatchString(sha) {
		return "", fmt.Errorf("%s does not resolve (fetch origin, or `git remote set-head origin -a`)", ref)
	}
	return sha, nil
}

// integrationRefRe is the shape of a remote-tracking ref integrationTip accepts
// from origin/HEAD's symbolic target.
var integrationRefRe = regexp.MustCompile(`^refs/remotes/origin/[A-Za-z0-9._/-]+$`)

// distinctSorted returns ids de-duplicated and sorted.
func distinctSorted(ids []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// boardCarriesAny reports whether the board at root carries the stream of any id.
func boardCarriesAny(root string, ids []string) bool {
	for _, id := range ids {
		if stream, _, ok := strings.Cut(id, "/"); ok && ownsStream(root, stream) {
			return true
		}
	}
	return false
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
