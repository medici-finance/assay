package main

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// assess.go — the one determination both verbs run. `check` reads its verdict and
// throws the worktree away; `merge` reads the SAME verdict and, if a human granted it,
// commits and pushes from the worktree it left standing. There is deliberately not a
// second implementation of "would this merge cleanly?" for the write path: two copies of
// that question is the derive-or-diff failure, and the copy that drifts would be the one
// authorizing a push.

// trial is the outcome of a merge attempt left in the worktree, unconmitted.
type trial struct {
	wt  *worktree
	rep *report
	// Listed/Unlisted are the conflicted paths, split by the compiled-in regenerable
	// list. Both empty means a clean merge.
	Listed   []string
	Unlisted []string
	// UpToDate means the base is already an ancestor of the head: there is nothing to
	// merge and nothing to push.
	UpToDate bool
}

// close tears down the worktree. Safe to call twice and on every exit path.
func (t *trial) close() {
	if t != nil {
		t.wt.remove()
	}
}

// assess fetches, measures and trial-merges. On success the returned trial owns a
// worktree the CALLER must close — including on its own error paths.
//
// The order matters: everything that can be established WITHOUT creating a worktree is
// established first, so a run that is going to report could-not-check does so before it
// touches the filesystem.
func assess(root, repo string, p prInfo, doProbe bool) (*trial, error) {
	rep := &report{
		Repo: repo, PR: p.Number,
		Base: p.BaseRefName, Head: p.HeadRefName,
		Mergeability:         mergeUnknown,
		BehindState:          stateUnknown,
		CIContractDriftState: stateUnknown,
		SemanticValidity:     stateNotAsked,
	}

	baseSHA, headSHA, err := fetchState(root, repo, p)
	if err != nil {
		return &trial{rep: rep}, err
	}
	rep.BaseSHA, rep.HeadSHA = baseSHA, headSHA

	mergeBase, err := runGit(root, "merge-base", headSHA, baseSHA)
	if err != nil {
		return &trial{rep: rep}, deskkit.Unverifiable(
			"could-not-check: the head and the base have no common ancestor git can find — "+
				"deskmerge cannot compute currency between two unrelated histories", err)
	}
	rep.MergeBase = mergeBase

	// `--left-right --count A...B` prints "<commits only in A>\t<commits only in B>":
	// ahead of base, then behind base.
	counts, err := runGit(root, "rev-list", "--left-right", "--count", headSHA+"..."+baseSHA)
	if err != nil {
		return &trial{rep: rep}, deskkit.Unverifiable(
			"could-not-check: cannot count the distance between the head and the base", err)
	}
	f := strings.Fields(counts)
	if len(f) != 2 {
		return &trial{rep: rep}, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: `git rev-list --left-right --count` returned %q, which is not two counts",
			deskkit.StripControl(counts)), nil)
	}
	rep.Ahead, _ = strconv.Atoi(f[0])
	rep.Behind, err = strconv.Atoi(f[1])
	if err != nil {
		return &trial{rep: rep}, deskkit.Unverifiable(
			"could-not-check: the behind-count did not parse as an integer", err)
	}
	rep.BehindState = stateClean

	// CI-contract drift. Measured between the MERGE BASE and the base head, not
	// between the two heads: the question is what CI machinery main gained that this
	// branch has never had, which is exactly the merge-base..base range.
	drift, err := runGit(root, "diff", "--name-only", mergeBase, baseSHA,
		"--", ".github/workflows", ".github/scripts")
	if err != nil {
		rep.CIContractDriftState = stateUnknown
		rep.note("could-not-check: the CI-contract diff failed, so a check failing on this branch " +
			"cannot be told apart from a defect in it")
	} else {
		for _, ln := range strings.Split(drift, "\n") {
			if ln = strings.TrimSpace(ln); ln != "" {
				rep.CIContractDrift = append(rep.CIContractDrift, ln)
			}
		}
		rep.CIContractDriftState = stateClean
	}

	// Nothing to merge. Report it and skip the worktree entirely — creating one to
	// discover there is no work is the kind of cost that gets a probe switched off.
	if rep.Behind == 0 {
		rep.Mergeability = mergeClean
		if doProbe {
			rep.note("--probe not run: the branch is already current, so there is no merged tree " +
				"distinct from the head that CI already builds")
		}
		return &trial{rep: rep, UpToDate: true}, nil
	}

	wt, err := newWorktree(root, headSHA)
	if err != nil {
		return &trial{rep: rep}, err
	}
	t := &trial{wt: wt, rep: rep}

	// --no-ff: never fast-forward. A fast-forward produces a SINGLE-parent head that
	// looks current and is not a merge — the #72 shape, reached by
	// accident rather than by intent. --no-commit: stop with the result in the index so
	// the caller decides whether it ever becomes a commit.
	if _, merr := runGit(wt.dir, "merge", "--no-ff", "--no-commit", baseSHA); merr != nil {
		paths, cerr := conflictedPaths(wt.dir)
		if cerr != nil {
			rep.Mergeability = mergeUnknown
			return t, cerr
		}
		t.Listed, t.Unlisted = classifyConflicts(paths)
		rep.ConflictedListed, rep.ConflictedOther = t.Listed, t.Unlisted
		if len(t.Unlisted) > 0 {
			rep.Mergeability = mergeConflicted
			return t, nil
		}
		rep.Mergeability = mergeRegenerable
		return t, nil
	}
	rep.Mergeability = mergeClean

	if doProbe {
		runProbe(root, wt.dir, mergeBase, baseSHA, headSHA, rep)
	}
	return t, nil
}

// changedBothSides is the union of the paths each side changed since the merge base —
// the region where a semantic collision can live at all.
func changedBothSides(root, mergeBase, baseSHA, headSHA string) ([]string, error) {
	var all []string
	for _, tip := range []string{baseSHA, headSHA} {
		out, err := runGit(root, "diff", "--name-only", mergeBase, tip)
		if err != nil {
			return nil, err
		}
		for _, ln := range strings.Split(out, "\n") {
			if ln = strings.TrimSpace(ln); ln != "" {
				all = append(all, ln)
			}
		}
	}
	return all, nil
}

// runProbe builds the merged tree and records the compiler's verdict.
//
// This is the ONLY thing in deskmerge that can see a semantic collision, and its reach
// is exactly the reach of the compiled-in commands — no further. On a repo with no
// configured probe the answer is could-not-check, not clean: "I have no probe for this
// repo" and "this repo's merged tree builds" are different statements and only one of
// them is true.
func runProbe(root, dir, mergeBase, baseSHA, headSHA string, rep *report) {
	changed, err := changedBothSides(root, mergeBase, baseSHA, headSHA)
	if err != nil {
		rep.SemanticValidity = stateUnknown
		rep.ProbeDetail = "the changed-path sets could not be read, so there is nothing to scope a probe to"
		return
	}
	steps := probeTargets(dir, changed)
	if len(steps) == 0 {
		rep.SemanticValidity = stateUnknown
		rep.ProbeDetail = "no buildable module in the merged tree was touched by either side — " +
			"deskmerge has no probe that applies here, which is not the same as a tree that builds"
		return
	}
	for _, s := range steps {
		out, err := runCmdIn(filepath.Join(dir, s.Dir), s.Argv[0], s.Argv[1:]...)
		if err == nil {
			continue
		}
		rep.SemanticValidity = stateFailed
		rep.ProbeDetail = fmt.Sprintf(
			"the MERGED tree does not build: `%s` in %s failed. This is the class no textual "+
				"conflict check can see — two individually-green branches, invalid against each "+
				"other. It is worker work, not merge-currency work. %s",
			strings.Join(s.Argv, " "), s.Dir,
			deskkit.StripControl(firstLines(out+" "+err.Error(), 3)))
		return
	}
	rep.SemanticValidity = stateClean
	rep.ProbeDetail = "the merged tree builds under the compiled-in probe — which is a BUILD, " +
		"not a test run, and is blind to any collision the compiler cannot see"
}

func firstLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, " / ")
}
