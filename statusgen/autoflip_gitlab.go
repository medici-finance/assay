package main

// autoflip_gitlab.go — the GitLab auto-flip's live read (forge-neutral/18, #7202): every
// merge-request fact decideModelFlip needs, read through `deskread`, the seam's read verb, so a
// verified gate:model brief on a GitLab repo can flip on the reviewer's approval at the merged
// head instead of reporting a standing could-not-check. The DECISION is unchanged
// (decideGitLabFlip): this file only feeds it real merge-request state.
//
// The reads, one deskread kind each, all through deskreadReader.readVerb (the one launch site):
//
//	MergedPRForCommit  commit-changes --repo R --sha S, then change --change R#N per number,
//	                   and change-commits --change R#N for the intermediate-commit path
//	PRShape            change, change-files, default-branch
//	ReviewState        change, reviews (approvals: APPROVED with no body), comments --change (notes)
//	PRsNamingBrief     changes --repo R --state merged, filtered on the `Brief:` trailer text
//	PRBodyEditedAt     change-trust --change R#N
//
// Every way a read fails, comes back truncated, or omits a field the decision rests on is an
// ERROR, which the callers turn into a could-not-check; none is ever read as an empty answer.

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// deskreadModelFlipSource is the live modelFlipSource for a GitLab reviewer.
type deskreadModelFlipSource struct {
	r deskreadReader
}

// CommitsTouching is plain git and works on any forge.
func (deskreadModelFlipSource) CommitsTouching(root, relPath string, limit int) ([]string, error) {
	return ghModelFlipSource{}.CommitsTouching(root, relPath, limit)
}

// change reads one change's record (`deskread change`).
func (s deskreadModelFlipSource) change(repo string, n int) (*deskreadChange, error) {
	env, i, err := s.r.readItemAt("--change", "change", repo, n)
	if err != nil {
		return nil, err
	}
	c := env.Items[i].Change
	if c == nil {
		return nil, fmt.Errorf("%s change --change %s#%d: could-not-check: the item carries no change record", s.r.bin, repo, n)
	}
	return c, nil
}

// MergedPRForCommit resolves sha to the merge request that owns it: the forge's
// commit-to-change association (any state), each candidate's merge record, and — only for the
// intermediate-commit path — the commits a candidate introduced. The decision is the shared
// mergedPRForCommit, the same one the GitHub transport feeds.
func (s deskreadModelFlipSource) MergedPRForCommit(repo, sha string) (int, bool, error) {
	e, err := s.r.readRepoKind("commit-changes", repo, "--sha", sha)
	if err != nil {
		return 0, false, err
	}
	if e.CommitChanges == nil {
		return 0, false, fmt.Errorf("%s commit-changes --repo %s --sha %s: could-not-check: the entry carries no change list", s.r.bin, repo, sha)
	}
	prs := make([]ghCommitPR, 0, len(*e.CommitChanges))
	for _, n := range *e.CommitChanges {
		c, cerr := s.change(repo, n)
		if cerr != nil {
			return 0, false, cerr
		}
		p := ghCommitPR{Number: n, MergeCommitSHA: c.MergeCommitSHA}
		p.Head.SHA = c.HeadSHA
		if c.Merged {
			if c.MergedAt == "" {
				return 0, false, fmt.Errorf("MR !%d is reported merged with no merge time — could-not-check, never an unmerged candidate", n)
			}
			p.MergedAt = c.MergedAt
		}
		prs = append(prs, p)
	}
	return mergedPRForCommit(sha, prs, func(n int) ([]string, error) {
		return s.changeCommits(repo, n)
	})
}

// changeCommits lists the commits change n introduced. An incomplete list is an error: a SHA
// missing from it is unknown, not "not introduced".
func (s deskreadModelFlipSource) changeCommits(repo string, n int) ([]string, error) {
	env, i, err := s.r.readItemAt("--change", "change-commits", repo, n)
	if err != nil {
		return nil, err
	}
	cc := env.Items[i].ChangeCommits
	if cc == nil || cc.Complete == nil {
		return nil, fmt.Errorf("%s change-commits --change %s#%d: could-not-check: the item carries no commit list or no completeness field", s.r.bin, repo, n)
	}
	if !*cc.Complete {
		return nil, fmt.Errorf("MR !%d's commit list stopped at its page bound — could-not-check, never \"the commit was not introduced\"", n)
	}
	return cc.SHAs, nil
}

// PRShape reads a candidate merge request's body, author, base branch and COMPLETE file list.
// Heads is left nil: a missing head is never Evidence-only, so such a change stays a candidate
// and is judged on its approval like any other (evidenceOnlyReason).
func (s deskreadModelFlipSource) PRShape(repo string, pr int) (prShape, error) {
	c, err := s.change(repo, pr)
	if err != nil {
		return prShape{}, err
	}
	env, i, err := s.r.readItemAt("--change", "change-files", repo, pr)
	if err != nil {
		return prShape{}, err
	}
	cf := env.Items[i].ChangeFiles
	if cf == nil || cf.Complete == nil {
		return prShape{}, fmt.Errorf("%s change-files --change %s#%d: could-not-check: the item carries no file list or no completeness field", s.r.bin, repo, pr)
	}
	if !*cf.Complete {
		return prShape{}, fmt.Errorf("MR !%d reports %d changed files but the list is truncated — refusing to judge a truncated file list", pr, cf.ChangedFiles)
	}
	var files, renamedFrom []string
	patches := map[string]string{}
	for _, f := range cf.Files {
		if f.Filename == "" {
			return prShape{}, fmt.Errorf("MR !%d lists a changed file with no path", pr)
		}
		files = append(files, f.Filename)
		if f.PreviousFilename != "" {
			renamedFrom = append(renamedFrom, f.PreviousFilename)
		}
		if !f.PatchAbsent {
			patches[f.Filename] = f.Patch
		}
	}
	shape, err := shapeFromListing(pr, c.Body, cf.ChangedFiles, files)
	if err != nil {
		return prShape{}, err
	}
	if c.Author != nil {
		shape.Author = c.Author.Login
	}
	shape.RenamedFrom = renamedFrom
	shape.Patches = patches
	db, err := s.r.readRepoKind("default-branch", repo)
	if err != nil {
		return prShape{}, err
	}
	if c.BaseRef == "" || db.DefaultBranch == "" {
		return prShape{}, fmt.Errorf("MR !%d reports no target branch (%q) or default branch (%q)", pr, c.BaseRef, db.DefaultBranch)
	}
	shape.BaseRef, shape.DefaultBranch = c.BaseRef, db.DefaultBranch
	return shape, nil
}

// ReviewState reads the merge request's merge state, merged head, approvals and notes — the
// facts decideGitLabFlip judges. The `reviews` kind serves GitLab's approvals AND its non-system
// notes in one list, and renders a "Verdict: approve" note APPROVED too; only an APPROVED entry
// with NO body is an approval (the approvals endpoint carries no body, a note always has one),
// so a note can never stand in for the approval. The notes themselves are read from
// `comments --change`, which carries every note's author and body.
func (s deskreadModelFlipSource) ReviewState(repo string, pr int) (prReviewState, error) {
	c, err := s.change(repo, pr)
	if err != nil {
		return prReviewState{}, err
	}
	st := prReviewState{Merged: c.Merged && c.MergedAt != "", HeadSHA: c.HeadSHA}
	if t, perr := time.Parse(time.RFC3339, c.MergedAt); perr == nil {
		st.MergedAt = t // an unparseable time stays zero: it never ends the guard walk
	}
	env, i, err := s.r.readItemAt("--change", "reviews", repo, pr)
	if err != nil {
		return prReviewState{}, err
	}
	rs := env.Items[i].Reviews
	if rs == nil {
		return prReviewState{}, fmt.Errorf("%s reviews --change %s#%d: could-not-check: the item carries no review list", s.r.bin, repo, pr)
	}
	for _, r := range *rs {
		if r.State == "APPROVED" && r.Body == "" && r.AuthorLogin != "" {
			st.Approvals = append(st.Approvals, r.AuthorLogin)
		}
	}
	env, i, err = s.r.readItemAt("--change", "comments", repo, pr)
	if err != nil {
		return prReviewState{}, err
	}
	cs := env.Items[i].Comments
	if cs == nil {
		return prReviewState{}, fmt.Errorf("%s comments --change %s#%d: could-not-check: the item carries no note list", s.r.bin, repo, pr)
	}
	for _, n := range *cs {
		st.Notes = append(st.Notes, glNote{Author: n.AuthorLogin, Body: n.Body})
	}
	return st, nil
}

// PRsNamingBrief lists the merged merge requests whose description carries the literal
// `Brief: <key>`. Like the GitHub search it is a PREFILTER: every hit is re-read (PRShape) and
// its trailers parsed before it can be credited. A list the verb stopped at its page bound is an
// error — resolveByTrailer then notes the search as unread and the file-history walk decides.
func (s deskreadModelFlipSource) PRsNamingBrief(repo, briefKey string) ([]int, error) {
	what := fmt.Sprintf("%s changes --repo %s --state merged", s.r.bin, repo)
	out, err := s.r.readVerb([]string{"changes", "--repo", repo, "--state", "merged"})
	var env deskreadChangesEnvelope
	if jerr := json.Unmarshal(out, &env); jerr != nil {
		if err != nil {
			return nil, fmt.Errorf("%s: could-not-check: %v", what, err)
		}
		return nil, fmt.Errorf("parsing the %s envelope: %w", what, jerr)
	}
	if env.Schema != deskreadSchema || env.Kind != "changes" {
		return nil, fmt.Errorf("%s: could-not-check: envelope schema %d kind %q, this build understands schema %d kind \"changes\"", what, env.Schema, env.Kind, deskreadSchema)
	}
	for _, p := range env.Partial {
		if p.Repo == repo {
			return nil, fmt.Errorf("%s: could-not-check: %s", what, p.Reason)
		}
	}
	needle := "Brief: " + briefKey
	for _, r := range env.Repos {
		if r.Repo != repo {
			continue
		}
		if r.Changes == nil || r.Incomplete == nil {
			return nil, fmt.Errorf("%s: could-not-check: the entry carries no change list or no completeness field", what)
		}
		if *r.Incomplete {
			return nil, fmt.Errorf("%s: could-not-check: the list stopped at its %d-page bound, so a merge request naming %s may be unlisted", what, r.PageCap, briefKey)
		}
		var hits []int
		for _, c := range *r.Changes {
			if strings.Contains(c.Body, needle) {
				hits = append(hits, c.Number)
			}
		}
		return hits, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: could-not-check: %v", what, err)
	}
	return nil, fmt.Errorf("%s: could-not-check: the envelope returned no result and no reason for this repo", what)
}

// PRBodyEditedAt reads when the merge request's description was last edited. An empty edit time
// is "never edited" ONLY when the forge reported the body's edit history and the read was
// complete; otherwise it is an error, never a zero time (trailerBodyProblem would read zero as a
// body nobody changed after review).
func (s deskreadModelFlipSource) PRBodyEditedAt(repo string, pr int) (time.Time, error) {
	env, i, err := s.r.readItemAt("--change", "change-trust", repo, pr)
	if err != nil {
		return time.Time{}, err
	}
	tr := env.Items[i].Trust
	if tr == nil {
		return time.Time{}, fmt.Errorf("%s change-trust --change %s#%d: could-not-check: the item carries no trust payload", s.r.bin, repo, pr)
	}
	if !tr.Complete {
		return time.Time{}, fmt.Errorf("MR !%d's activity read stopped at its page bound — its last description edit is unknown", pr)
	}
	if tr.BodyEditedAt == "" {
		if !tr.BodyHistoryKnown {
			return time.Time{}, fmt.Errorf("MR !%d: the forge reported no description-edit history — unknown, never \"never edited\"", pr)
		}
		return time.Time{}, nil
	}
	t, perr := time.Parse(time.RFC3339Nano, tr.BodyEditedAt)
	if perr != nil {
		return time.Time{}, fmt.Errorf("MR !%d: bad bodyEditedAt %q: %w", pr, tr.BodyEditedAt, perr)
	}
	return t, nil
}
