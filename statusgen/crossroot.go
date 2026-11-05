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
//   - PUBLISHED. A delivery names the foreign root's <owner>/<repo> in the
//     written row, the STATUS.md hold label, reconcile's witness and every
//     notice, so a root is read only when THIS tree's own
//     docs/streams/graph-repos.yaml publishes that repo (an entry whose repo is
//     named and not `unpublished`). That registry is where a tree already says
//     which sibling repos it is willing to name; a root the tree withholds (or
//     any root when the registry is absent or unreadable) is never read for a
//     delivery and is never named or located (no slug, no path) in any output:
//     one aggregate could-not-check line counts them.
//
// THREE-STATE. A configured root that cannot be read (absent, not a git
// checkout, shallow, git failing) yields one could-not-check line naming it,
// and no row is held or written on its account. It never fails the board's own
// read: rows the readable roots witness are still held. An UNREAD root is not
// an empty one: when the stream's carriers are being counted, a root whose
// docs/streams cannot be checked counts as a possible carrier, so the trailer
// is credited to none rather than to a board an unread root might share.

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
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
	published, regWhy := publishedRepos(root)
	var others []string // configured roots other than the board's own, in repo order
	var named []string  // the subset this tree publishes: the only ones read for a delivery
	for _, repo := range repos {
		if !validSiblingRepo(repo) || (ownRepo != "" && strings.EqualFold(repo, ownRepo)) || pathIsRoot(root, roots[repo]) {
			continue
		}
		others = append(others, repo)
		if published[strings.ToLower(repo)] {
			named = append(named, repo)
		}
	}
	// carriers lists the other configured roots that carry docs/streams/<stream>,
	// or that cannot be ruled out as carrying it: an unreadable root is not an
	// empty one. A root this tree does not publish counts toward the carrier set
	// but is never named — only counted.
	type carrierSet struct {
		names []string // published carriers, as "<owner>/<repo>" (+ " (unreadable)")
		anon  int      // unpublished carriers, never named
	}
	carried := map[string]carrierSet{}
	carriers := func(stream string) carrierSet {
		if c, ok := carried[stream]; ok {
			return c
		}
		var c carrierSet
		for _, repo := range others {
			state := streamPresence(roots[repo], stream)
			if state == streamAbsent {
				continue
			}
			switch {
			case !published[strings.ToLower(repo)]:
				c.anon++
			case state == streamUnknown:
				c.names = append(c.names, repo+" (unreadable)")
			default:
				c.names = append(c.names, repo)
			}
		}
		carried[stream] = c
		return c
	}
	carrierText := func(c carrierSet) string {
		parts := append([]string(nil), c.names...)
		if c.anon > 0 {
			parts = append(parts, fmt.Sprintf("%d other configured root(s)", c.anon))
		}
		return strings.Join(parts, ", ")
	}
	var out []mergedPR
	var notices []string
	if n := len(others) - len(named); n > 0 {
		why := "are not published in this tree's docs/streams/graph-repos.yaml"
		if regWhy != "" {
			why = "cannot be matched against docs/streams/graph-repos.yaml (" + regWhy + ")"
		}
		notices = append(notices, fmt.Sprintf("could-not-check: cross-root delivery witness: %d configured root(s) %s, so none is "+
			"read, named or located here — a `Brief:` trailer merged there is not seen this run, so no row is held or "+
			"written on its account.", n, why))
	}
	for _, repo := range named {
		path := roots[repo]
		cannot := func(why string) {
			notices = append(notices, fmt.Sprintf("could-not-check: cross-root delivery witness: %s: %s — a "+
				"`Brief:` trailer merged there is not seen this run, so no row is held or written on its account.",
				repo, noticeSafe(strings.ReplaceAll(why, path, "<root>"))))
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
			if c := carriers(stream); len(c.names) > 0 || c.anon > 0 {
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
				"`Brief:` trailer for stream %s, which %s also carries or could not be read to rule out — a trailer cannot say "+
				"which board it means, so none is held or written on its account.", repo, ambiguous[stream], noticeSafe(stream),
				carrierText(carriers(stream))))
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
	out, err, supported := commitReadOutput(ordinaryCommitRead(path, ref), ref)
	if !supported {
		out, err = exec.Command("git", "-C", path, "rev-parse", "--verify", "--quiet", ref+"^{commit}").Output()
	}
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

// streamPresence is whether a checkout carries docs/streams/<stream>, with the
// third state kept distinct: streamUnknown is a root that cannot be checked (the
// path is absent or any Stat error other than "not there"), which a caller
// counting carriers must treat as a possible carrier, never as an empty one.
type streamPresenceState int

const (
	streamAbsent streamPresenceState = iota
	streamPresent
	streamUnknown
)

func streamPresence(path, stream string) streamPresenceState {
	if stream == "" || strings.ContainsAny(stream, `/\`) || stream == "." || stream == ".." {
		return streamAbsent
	}
	fi, err := os.Stat(filepath.Join(path, "docs", "streams", stream))
	switch {
	case err == nil && fi.IsDir():
		return streamPresent
	case err == nil:
		return streamAbsent
	case errors.Is(err, fs.ErrNotExist):
		if rootInfo, rerr := os.Stat(path); rerr != nil || !rootInfo.IsDir() {
			return streamUnknown // the checkout itself is missing: nothing was checked
		}
		return streamAbsent
	}
	return streamUnknown
}

// publishedRepos is the set of lower-cased <owner>/<repo> names this tree's own
// docs/streams/graph-repos.yaml publishes (an entry with a repo and not
// `unpublished`). why is "" on success, else why the set is empty for want of a
// registry (absent, or unreadable) — never the file's path.
func publishedRepos(root string) (published map[string]bool, why string) {
	published = map[string]bool{}
	reg, found, err := loadGraphRepos(root)
	switch {
	case err != nil:
		return published, "the registry could not be read"
	case !found:
		return published, "this tree has no registry"
	}
	for _, e := range reg.Aliases {
		if !e.Unpublished && e.Repo != "" {
			published[strings.ToLower(e.Repo)] = true
		}
	}
	return published, ""
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
