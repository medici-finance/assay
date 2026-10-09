package main

// reviewroundread.go — the forge reads behind a review round (#2444), and the one call
// dispatch() makes for them.
//
// WHERE IT RUNS. prepareReviewRound runs for a review-class dispatch of an open change, after
// every caller precondition and BEFORE the claim: a held dispatch (reviewgate.go) has claimed
// nothing, cut nothing and written nothing. It does not run on --dry-run, which touches
// nothing and reads nothing.
//
// IT READS ONLY. reviewRoundForge is the whole forge surface this file and reviewgate.go can
// reach, and it holds no method that writes. Nothing here can post a verdict, apply a label,
// flip a change ready or record that a head was reviewed.
//
// A READ THAT FAILS NEVER HOLDS. prepareReviewRound returns an error for a gate hold and for
// nothing else, and a gate condition holds only on a complete read. A failed read of the
// change, the forge, the reviews or a gate condition is a note on stderr; a failed read of a
// scope fact (a file list, a comparison, a commit) is the full-pass reason the assignment
// states. Either way the reviewer is dispatched.

import (
	"fmt"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// reviewRoundForge is the read-only forge surface the round and the gate use. Every method
// is one deskkit.Forge already has.
type reviewRoundForge interface {
	GetPullRequest(repo deskkit.ForgeRepo, number int) (*deskkit.PullRequest, error)
	GetIssue(repo deskkit.ForgeRepo, number int) (*deskkit.Issue, error)
	ReviewsAtHead(repo deskkit.ForgeRepo, number int) ([]deskkit.Review, error)
	ListChangedFiles(repo deskkit.ForgeRepo, number int) ([]deskkit.ChangedFile, error)
	ChecksAtHead(repo deskkit.ForgeRepo, sha string) (*deskkit.ChecksAtHead, error)
	RequiredStatusChecks(repo deskkit.ForgeRepo, branch string) ([]string, error)
	ListLabelEvents(repo deskkit.ForgeRepo, number int) ([]deskkit.LabelEvent, error)
	ListIssueLabelEvents(repo deskkit.ForgeRepo, number int) ([]deskkit.LabelEvent, error)
	PRTrustEvents(repo deskkit.ForgeRepo, number int) (*deskkit.TrustPayload, error)
	IssueTrustEvents(repo deskkit.ForgeRepo, number int) (*deskkit.TrustPayload, error)
	CompareRefs(repo deskkit.ForgeRepo, base, head string) (*deskkit.RefComparison, error)
	GetCommit(repo deskkit.ForgeRepo, sha string) (*deskkit.RepoCommit, error)
}

// reviewRoundForgeFn resolves the forge the round and the gate read from. It is nil unless
// main() wires liveReviewRoundForge, like the represented-change read: a run that did not
// wire it reads nothing, holds nothing and states a full pass.
var reviewRoundForgeFn func(repo string) (reviewRoundForge, deskkit.ForgeRepo, error)

// liveReviewRoundForge is the production resolution: the same forge, under the same role,
// as the review packet and the model-stamp step.
func liveReviewRoundForge(repo string) (reviewRoundForge, deskkit.ForgeRepo, error) {
	fr, err := forgeRepoOf(repo)
	if err != nil {
		return nil, fr, err
	}
	fg, _, err := deskkit.ResolveForge(fr, deskkit.ReviewDispatcherRole)
	if err != nil {
		return nil, fr, err
	}
	return fg, fr, nil
}

// forgeComparePageFiles is the most files one unpaginated comparison serves. A comparison
// that returns this many is treated as possibly cut short.
const forgeComparePageFiles = 300

// roundNote says, on stderr and regardless of --quiet, that a read behind the round or the
// gate failed and what the dispatch does instead.
func roundNote(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "deskdispatch: review round: "+format+"\n", args...)
}

// prepareReviewRound is the ONE call dispatch() makes. It returns the round statement for
// the assignment, or a refusal when the pre-dispatch gate holds the head. It returns an error
// for a hold and for nothing else.
func prepareReviewRound(o dispatchOpts, plan dispatchPlan, repo string) (reviewRound, error) {
	if reviewRoundForgeFn == nil {
		return reviewRound{}, nil
	}
	rr := reviewRound{read: true, lane: reviewLaneForClaim(plan.claimKey, repo, o.pr)}
	undetermined := func(why string) (reviewRound, error) {
		roundNote("%s — dispatching; the scope is a full pass and the round is not determined.", why)
		rr.scope = decideReviewScope(scopeFacts{unknown: why})
		rr.round = laneRound{why: why}
		return rr, nil
	}
	fg, fr, err := reviewRoundForgeFn(repo)
	if err != nil || fg == nil {
		return undetermined("could not reach the forge")
	}
	pr, err := fg.GetPullRequest(fr, o.pr)
	if err != nil || pr == nil {
		return undetermined("could not read the change")
	}
	rr.head = strings.TrimSpace(pr.HeadSHA)

	// The lane's own verdicts. Not known when the reviews, the reviewer identity or the lane
	// cannot be read; the gate's lane condition is then not evaluated either.
	var verdicts []laneVerdict
	unknown := ""
	reviewer, bound := reviewPacketReviewerFn()
	reviews, rerr := fg.ReviewsAtHead(fr, o.pr)
	switch {
	case rr.lane == "":
		unknown = "the claim key names no single lane"
	case !bound || reviewer == "":
		unknown = "the reviewer identity is not bound here"
	case rerr != nil:
		unknown = "could not read the change's reviews"
	default:
		verdicts = laneVerdictsOf(reviews, reviewer, rr.lane)
		// A verdict whose own record names another head than the forge records for it: none
		// of the lane's heads is relied on, here or in the gate's lane condition below.
		if why := disputedVerdictHead(verdicts); why != "" {
			verdicts, unknown = nil, why
		}
	}

	holds, notes := reviewGate(fg, fr, o.pr, pr, verdicts)
	for _, n := range notes {
		roundNote("%s — that condition does not hold the dispatch.", n)
	}
	if len(holds) > 0 {
		return rr, holdRefusal(repo, o.pr, rr.head, holds)
	}

	if unknown != "" {
		return undetermined(unknown)
	}
	heads := make([]string, 0, len(verdicts))
	for _, v := range verdicts {
		heads = append(heads, v.head)
	}
	rr.round = decideLaneRound(heads, rr.head)
	if len(verdicts) > 0 {
		prev := verdicts[len(verdicts)-1]
		rr.prev = &prev
	}
	rr.scope = decideReviewScope(readScopeFacts(fg, fr, o.pr, pr, rr.prev))
	return rr, nil
}

// readScopeFacts gathers what decideReviewScope reads. It stops at the first fact that
// already decides a full pass, so a first round costs no further read.
func readScopeFacts(fg reviewRoundForge, fr deskkit.ForgeRepo, number int, pr *deskkit.PullRequest, prev *laneVerdict) scopeFacts {
	if prev == nil {
		return scopeFacts{}
	}
	f := scopeFacts{verdict: true, prevHead: prev.head, head: strings.TrimSpace(pr.HeadSHA)}
	if !isCommitID(f.prevHead) || !isCommitID(f.head) || strings.EqualFold(f.prevHead, f.head) {
		return f
	}

	// The change's own file list at the dispatched head, reconciled against the forge's count.
	files, err := fg.ListChangedFiles(fr, number)
	if err != nil || len(files) != pr.ChangedFiles {
		f.unknown = "the change's file list at the dispatched head is not provably complete"
		return f
	}
	f.headFiles = fileNames(files)

	// The change's own file list at the previous head: what the previous round had in front
	// of it. Read as the comparison of the base branch with that head.
	if strings.TrimSpace(pr.BaseRef) == "" {
		f.unknown = "the change's base branch is not known"
		return f
	}
	was, err := fg.CompareRefs(fr, pr.BaseRef, f.prevHead)
	if err != nil || was == nil || len(was.Files) >= forgeComparePageFiles {
		f.unknown = "the change's file list at the previous head is not provably complete"
		return f
	}
	f.prevFiles = fileNames(was.Files)

	// The interval between the two heads, along the branch's first-parent chain.
	cmp, err := fg.CompareRefs(fr, f.prevHead, f.head)
	if err != nil || cmp == nil {
		f.diffErr = "the forge could not compare the two heads"
		return f
	}
	if !cmp.CommitsComplete {
		f.diffErr = "the interval's commit list is not complete"
		return f
	}
	chain, why := firstParentInterval(cmp.Commits, f.prevHead, f.head)
	if why != "" {
		f.diffErr = why
		return f
	}
	f.commits = len(chain)
	if f.commits > reviewDeltaMaxCommits {
		return f // already large; the per-commit reads would change nothing
	}
	for _, c := range chain {
		detail, err := fg.GetCommit(fr, c.SHA)
		if err != nil || detail == nil || !detail.FilesComplete {
			f.diffErr = "a commit in the interval has no complete file list"
			return f
		}
		names := fileNames(detail.Files)
		f.delta = append(f.delta, names...)
		if len(c.Parents) < 2 {
			continue
		}
		// A merge. Its own file list is what it changed against its FIRST parent, so a
		// conflict resolved to the branch's side is not in it. What each merged-in side
		// changed is read as the comparison of the first parent with that parent: a conflict
		// needs both sides to have changed the file, so this names it however it was resolved.
		f.merged = append(f.merged, names...)
		for _, side := range c.Parents[1:] {
			in, err := fg.CompareRefs(fr, c.Parents[0], side)
			if err != nil || in == nil || len(in.Files) >= forgeComparePageFiles {
				f.unknown = "what a merge in the interval brought in is not provably complete"
				return f
			}
			f.merged = append(f.merged, fileNames(in.Files)...)
		}
	}
	return f
}

// fileNames is every path a file list names: each file's path and, for a rename, the path
// it had before.
func fileNames(files []deskkit.ChangedFile) []string {
	var out []string
	for _, cf := range files {
		if cf.Filename != "" {
			out = append(out, cf.Filename)
		}
		if cf.PreviousFilename != "" {
			out = append(out, cf.PreviousFilename)
		}
	}
	return out
}

// firstParentInterval walks head's first parents back to base using only the parent lists
// the comparison returned, and returns the commits walked (head first). why is not empty
// when the walk cannot be completed: a commit without parent evidence, or a chain that leaves
// the interval before reaching base — a rewind, a force-push, or a merge that took the other
// branch as its first parent.
func firstParentInterval(commits []deskkit.RepoCommit, base, head string) (chain []deskkit.RepoCommit, why string) {
	byID := make(map[string]deskkit.RepoCommit, len(commits))
	for _, c := range commits {
		if c.Parents == nil || c.SHA == "" {
			return nil, "a commit in the interval carries no parent evidence"
		}
		byID[strings.ToLower(c.SHA)] = c
	}
	for cur := strings.ToLower(head); cur != strings.ToLower(base); {
		c, ok := byID[cur]
		if !ok || len(chain) >= len(byID) || len(c.Parents) == 0 {
			return nil, "the dispatched head's first-parent chain does not reach the previously reviewed head"
		}
		chain = append(chain, c)
		cur = strings.ToLower(c.Parents[0])
	}
	return chain, ""
}
