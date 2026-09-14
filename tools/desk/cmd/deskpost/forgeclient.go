package main

import (
	"fmt"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// forgeclient.go — the forge-agnostic read+write seam for deskpost's verdict/comment/flip
// verbs (forge-gitlab brief 09 §1). Before this, `review`/`security-review`/`comment`/`ready`
// read every precondition through the GitHub-App-authenticated ghClient and failed CLOSED on
// a GitLab-resolved repo (`deskpost has no gitlab write backend … exit 6`, the #772 follow-up).
// The typed Forge surface (brief 02) already answers every one of those preconditions and lands
// every write on GitLab; this file is the adapter that lets the ONE verb body drive either
// backend, selected by the repo's resolved forge.
//
// WHY AN INTERFACE RATHER THAN A SECOND VERB BODY. The verdict path is a security control (the
// reviewer-approval gate). A GitLab fork of postVerdictReview would be a second place the head
// pin, the trust gate, the public-repo +1, the model-capability floor and the non-author
// assertion could drift out of step — exactly the drift `security-review` was extracted to
// prevent. One body over an interface keeps every gate byte-identical across forges; only the
// transport (a GitHub REST client vs. a typed Forge) differs.
//
// WHAT STAYS ON ghClient. The GitHub path is unchanged: newPostBackend returns the same
// *ghClient for a GitHub-resolved (or not-positively-resolved, pre-772) repo, and requireGitHubForge
// still guards a DIRECT newGHClient call so a GitLab repo can never reach the GitHub App mint.
type postBackend interface {
	deskkit.RepoInfoFetcher // RepoVisibility / IssueReactions — the public-repo +1 gate's surface

	slug() (owner, name string)
	getPR(pr int) (*prInfo, error)
	getPRHead(pr int) (string, error)
	getIssue(n int) (*issueInfo, error)
	listReviews(pr int) ([]reviewInfo, error)
	listFiles(pr int) ([]prFile, error)
	stampTimeline(pr int) (deskkit.StampTimeline, error)
	claimLiveness(repo, body string) deskkit.ClaimLiveness
	headCommitAuthor(sha string) (string, error)
	prTrustPayload(n int) (*deskkit.TrustPayload, error)
	issueTrustPayload(n int) (*deskkit.TrustPayload, error)
	combinedStatusAt(sha string) (*combinedStatus, error)
	checkRunsAt(sha string) (*checkRunsResp, error)
	postReview(pr int, head, event, body string) error
	markReadyForReview(nodeID string) error
	// verdictLabels applies the mechanical, ADVISORY verdict-time labels (size + surface). It
	// is post-write and gates nothing; a backend that cannot compute them returns a note, not
	// an error (see forgeBackend.verdictLabels).
	verdictLabels(pr, reportedFiles int) (verdictLabelOutcome, error)
}

var (
	_ postBackend = (*ghClient)(nil)
	_ postBackend = (*forgeBackend)(nil)
)

// forgeForReviewer resolves the typed Forge backend the reviewer App acts under for repo. It is
// a package var ONLY so in-package tests can inject a fake Forge without a live GitLab instance;
// production always resolves through deskkit.ForgeFor (the one sanctioned construction site),
// which reads the reviewer PAT from the custody file and NEVER falls back to an ambient identity
// (brief 03 / brief 07 auth parity).
var forgeForReviewer = func(repo deskkit.ForgeRepo) (deskkit.Forge, error) {
	return deskkit.ForgeFor(repo, "reviewer")
}

// newPostBackend selects the read+write backend for repo by its RESOLVED forge:
//
//   - a POSITIVELY non-GitHub resolution (a roster `…=gitlab` binding, or a gitlab.com origin)
//     routes through the typed Forge surface (forgeBackend), so the verb reads its preconditions
//     and lands its verdict/comment/flip through the resolved backend — the #772 fail-closed is
//     never reached for a GitLab repo;
//   - GitHub, or a forge this build cannot POSITIVELY resolve, keeps the exact pre-772 path: the
//     App-authenticated ghClient (newGHClient). A could-not-check resolution is never read as
//     "it is GitLab" — same fail-safe direction requireGitHubForge already took.
func newPostBackend(owner, name string) (postBackend, error) {
	res, err := deskkit.ForgeKindFor(deskkit.ForgeRepo{Owner: owner, Name: name})
	if err == nil && res.Kind != deskkit.ForgeGitHub {
		return newForgeBackend(owner, name, string(res.Kind))
	}
	return newGHClient(owner, name)
}

// forgeBackend adapts the typed deskkit.Forge to the postBackend surface the verb bodies drive.
type forgeBackend struct {
	fg    deskkit.Forge
	owner string
	name  string
	repo  deskkit.ForgeRepo
	kind  string // the resolved forge kind, for the advisory verdictLabels note
}

func newForgeBackend(owner, name, kind string) (*forgeBackend, error) {
	fr := deskkit.ForgeRepo{Owner: owner, Name: name}
	fg, err := forgeForReviewer(fr)
	if err != nil {
		return nil, err
	}
	return &forgeBackend{fg: fg, owner: owner, name: name, repo: fr, kind: kind}, nil
}

func (b *forgeBackend) slug() (string, string) { return b.owner, b.name }

func (b *forgeBackend) getPR(pr int) (*prInfo, error) {
	p, err := b.fg.GetPullRequest(b.repo, pr)
	if err != nil {
		return nil, err
	}
	out := &prInfo{
		Number:       p.Number,
		State:        p.State,
		Draft:        p.Draft,
		NodeID:       p.NodeID,
		Body:         p.Body,
		ChangedFiles: p.ChangedFiles,
	}
	out.User.Login = p.Author.Login
	out.User.ID = p.Author.ID
	out.Head.SHA = p.HeadSHA
	return out, nil
}

func (b *forgeBackend) getPRHead(pr int) (string, error) {
	p, err := b.fg.GetPullRequest(b.repo, pr)
	if err != nil {
		return "", err
	}
	return p.HeadSHA, nil
}

func (b *forgeBackend) listFiles(pr int) ([]prFile, error) {
	// Risk-path detection reads only the PATHS (prFilePaths → RiskPathTriggered); the
	// Additions/Deletions the size label uses are advisory and not carried on this read, which
	// is fine because verdictLabels (the only size-label consumer) is a no-op on this backend.
	cf, err := b.fg.ListChangedFiles(b.repo, pr)
	if err != nil {
		return nil, err
	}
	out := make([]prFile, 0, len(cf))
	for _, f := range cf {
		out = append(out, prFile{Filename: f.Filename, PreviousFilename: f.PreviousFilename, Status: f.Status})
	}
	return out, nil
}

func (b *forgeBackend) getIssue(n int) (*issueInfo, error) {
	iss, err := b.fg.GetIssue(b.repo, n)
	if err != nil {
		return nil, err
	}
	out := &issueInfo{Number: iss.Number, State: iss.State}
	out.User.Login = iss.Author.Login
	out.User.ID = iss.Author.ID
	if iss.IsPullRequest {
		// The PullRequest sub-object is the object-kind discriminator resolveTarget /
		// requirePRErr read; only its presence matters, its URL is carried for parity.
		out.PullRequest = &struct {
			URL string `json:"url"`
		}{URL: iss.URL}
	}
	return out, nil
}

func (b *forgeBackend) listReviews(pr int) ([]reviewInfo, error) {
	rs, err := b.fg.ReviewsAtHead(b.repo, pr)
	if err != nil {
		return nil, err
	}
	out := make([]reviewInfo, 0, len(rs))
	for _, r := range rs {
		ri := reviewInfo{ID: r.ID, State: r.State, CommitID: r.CommitID, Body: r.Body, SubmittedAt: r.SubmittedAt}
		ri.User.Login = r.Author.Login
		out = append(out, ri)
	}
	return out, nil
}

func (b *forgeBackend) stampTimeline(pr int) (deskkit.StampTimeline, error) {
	// Present labels come from the change read; the applier-aware event stream from the
	// label-events read — the same two halves ghClient.stampTimeline pairs.
	p, err := b.fg.GetPullRequest(b.repo, pr)
	if err != nil {
		return deskkit.StampTimeline{}, err
	}
	events, err := b.fg.ListLabelEvents(b.repo, pr)
	if err != nil {
		return deskkit.StampTimeline{}, err
	}
	return deskkit.StampTimeline{Present: p.Labels, Events: events}, nil
}

func (b *forgeBackend) claimLiveness(repo, body string) deskkit.ClaimLiveness {
	// Mirrors ghClient.claimLiveness exactly, reading the claim ref through this backend's OWN
	// resolved Forge rather than a second ForgeFor construction. Every uncertain path is
	// Unknown, which changes nothing; only a positive ABSENT ages a stamp out.
	key, ok := deskkit.ClaimKeyForPR(repo, body)
	if !ok {
		return deskkit.ClaimLivenessUnknown
	}
	refPath, err := deskkit.ClaimRefPath(key)
	if err != nil {
		return deskkit.ClaimLivenessUnknown
	}
	present, rerr := b.fg.RefExists(b.repo, refPath)
	return deskkit.ClaimLivenessFromRefPresence(present, rerr)
}

func (b *forgeBackend) headCommitAuthor(sha string) (string, error) {
	// On GitLab the commit payload carries the raw git author but resolves no instance account,
	// so RepoCommit.AuthorLogin is EMPTY — a per-field could-not-check the non-author verdict
	// check reads as "unattributed" and falls back to the PR author (review.go), never a silent
	// pass. On GitHub it is the resolved login.
	c, err := b.fg.GetCommit(b.repo, sha)
	if err != nil {
		return "", err
	}
	return c.AuthorLogin, nil
}

func (b *forgeBackend) prTrustPayload(n int) (*deskkit.TrustPayload, error) {
	return b.fg.PRTrustEvents(b.repo, n)
}

func (b *forgeBackend) issueTrustPayload(n int) (*deskkit.TrustPayload, error) {
	return b.fg.IssueTrustEvents(b.repo, n)
}

func (b *forgeBackend) combinedStatusAt(sha string) (*combinedStatus, error) {
	ch, err := b.fg.ChecksAtHead(b.repo, sha)
	if err != nil {
		return nil, err
	}
	cs := &combinedStatus{State: ch.CombinedState, TotalCount: ch.StatusTotalCount}
	for _, s := range ch.Statuses {
		cs.Statuses = append(cs.Statuses, struct {
			State   string `json:"state"`
			Context string `json:"context"`
		}{State: s.State, Context: s.Context})
	}
	return cs, nil
}

func (b *forgeBackend) checkRunsAt(sha string) (*checkRunsResp, error) {
	ch, err := b.fg.ChecksAtHead(b.repo, sha)
	if err != nil {
		return nil, err
	}
	cr := &checkRunsResp{TotalCount: ch.CheckRunsTotalCount}
	for _, r := range ch.CheckRuns {
		cr.CheckRuns = append(cr.CheckRuns, struct {
			Name       string `json:"name"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
		}{Name: r.Name, Status: r.Status, Conclusion: r.Conclusion})
	}
	return cr, nil
}

func (b *forgeBackend) postReview(pr int, head, event, body string) error {
	// The shipping consumer of the typed reviewer verdict-write op on the GitLab backend: the
	// forge-neutral event (APPROVE / REQUEST_CHANGES / COMMENT) maps to a head-pinned approval +
	// verdict note that ReviewsAtHead reads at head (brief 02). No `glab` shell, no
	// arbitrary-endpoint method — the write goes through the enumerated Forge op or it does not
	// ship (brief 08's ban / no-passthrough test).
	return b.fg.PostReview(b.repo, pr, deskkit.ReviewInput{HeadSHA: head, Event: event, Body: body})
}

func (b *forgeBackend) markReadyForReview(nodeID string) error {
	return b.fg.MarkReadyForReview(nodeID)
}

func (b *forgeBackend) RepoVisibility(owner, repo string) (string, error) {
	return b.fg.RepoVisibility(deskkit.ForgeRepo{Owner: owner, Name: repo})
}

func (b *forgeBackend) IssueReactions(owner, repo string, issueNumber int) ([]deskkit.Reaction, error) {
	return b.fg.IssueReactions(deskkit.ForgeRepo{Owner: owner, Name: repo}, issueNumber)
}

func (b *forgeBackend) verdictLabels(pr, reportedFiles int) (verdictLabelOutcome, error) {
	// The size/surface labels are a mechanical, advisory merge-queue triage aid that GATES
	// NOTHING (label.go), and their GitHub form leans on the go-gh files/contents reads and the
	// timeline. Rather than fork that ghClient-shaped machinery onto every forge, the non-GitHub
	// path records a could-not-check note (reported as itself, C4) and applies no label: an
	// absent advisory label changes no gate, and the verdict — the actual control — has already
	// landed through the typed Forge op above.
	return verdictLabelOutcome{notes: []string{fmt.Sprintf(
		"verdict-time size/surface labels not applied on the %s backend (advisory; gates nothing)", b.kind)}}, nil
}
