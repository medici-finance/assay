package deskkit

// readonlyforge.go — the read-only Forge decorator behind the CI workflow-token transport
// (forge-neutral brief 34).
//
// ReadOnly wraps a Forge so that only the methods classified as reads reach it. It is
// DEFAULT-DENY BY CONSTRUCTION, and that is the whole point of how it is written:
//
//   - The decorator holds the inner forge in a NAMED field. It does not embed Forge, so it
//     delegates nothing implicitly. outboundForge embeds Forge and overrides only the
//     text-carrying writes, which is right for a checker (a method it forgets is still
//     delegated) and exactly wrong for a fence (a method it forgot would be a write that
//     goes through).
//   - It implements EVERY Forge method itself. The assertion below turns a method added to
//     Forge later into a compile error in this file until someone writes the method out and
//     decides, here, whether it reads or writes.
//   - A method not listed as a read returns a Refused error naming the method and never
//     touches the inner forge, with whatever arguments it was called with.
//
// The classification is not derived at runtime. Completeness comes from the compiler plus
// TestReadOnlyForgeRefusesEveryWriteMethod, which runs the decorator over a recording inner
// forge and checks every method against the test-side classification table (obClass).

import (
	"encoding/json"
	"fmt"
)

// readOnlyRefusalText is the phrase every refusal of this decorator carries. Tests and the
// deskread transport match on it.
const readOnlyRefusalText = "the CI workflow-token transport is read-only"

// readOnlyForge is the decorator. The inner forge is a NAMED field on purpose: an embedded
// Forge would delegate every method this file does not write out.
type readOnlyForge struct {
	inner Forge
}

var _ Forge = (*readOnlyForge)(nil)

// ReadOnly returns f wrapped so that every method not classified as a read is refused
// without reaching f. A nil f stays nil; an already read-only forge is returned unchanged.
func ReadOnly(f Forge) Forge {
	if f == nil {
		return nil
	}
	if _, ok := f.(*readOnlyForge); ok {
		return f
	}
	return &readOnlyForge{inner: f}
}

// readOnlyRefusal is the one refusal every non-read method returns.
func readOnlyRefusal(method string) error {
	return Refused(fmt.Sprintf("Forge.%s is refused: %s", method, readOnlyRefusalText))
}

// ---- reads: delegated ----

func (r *readOnlyForge) GetPullRequest(repo ForgeRepo, number int) (*PullRequest, error) {
	return r.inner.GetPullRequest(repo, number)
}

func (r *readOnlyForge) GetIssue(repo ForgeRepo, number int) (*Issue, error) {
	return r.inner.GetIssue(repo, number)
}

func (r *readOnlyForge) GetIssueTyped(repo ForgeRepo, number int, kind TargetKind) (*Issue, error) {
	return r.inner.GetIssueTyped(repo, number, kind)
}

func (r *readOnlyForge) OpenChangeForBranch(repo ForgeRepo, branch string) (*PullRequest, error) {
	return r.inner.OpenChangeForBranch(repo, branch)
}

func (r *readOnlyForge) SearchIssues(repo ForgeRepo, in SearchIssuesInput) ([]IssueSearchResult, error) {
	return r.inner.SearchIssues(repo, in)
}

func (r *readOnlyForge) ListLabels(repo ForgeRepo) ([]string, error) {
	return r.inner.ListLabels(repo)
}

func (r *readOnlyForge) ListOpenChanges(repo ForgeRepo) (*OpenChanges, error) {
	return r.inner.ListOpenChanges(repo)
}

func (r *readOnlyForge) ListChanges(repo ForgeRepo, states ChangeStates) (*ChangeList, error) {
	return r.inner.ListChanges(repo, states)
}

func (r *readOnlyForge) ListOpenIssues(repo ForgeRepo) ([]IssueSummary, error) {
	return r.inner.ListOpenIssues(repo)
}

func (r *readOnlyForge) ListIssues(repo ForgeRepo, in IssueListQuery) (*IssueList, error) {
	return r.inner.ListIssues(repo, in)
}

func (r *readOnlyForge) IssueStateEvents(repo ForgeRepo, number int) (*IssueStateHistory, error) {
	return r.inner.IssueStateEvents(repo, number)
}

func (r *readOnlyForge) ListChangeCommits(repo ForgeRepo, number int) (*ChangeCommits, error) {
	return r.inner.ListChangeCommits(repo, number)
}

func (r *readOnlyForge) RepoDefaultBranch(repo ForgeRepo) (string, error) {
	return r.inner.RepoDefaultBranch(repo)
}

func (r *readOnlyForge) PRTrustEvents(repo ForgeRepo, number int) (*TrustPayload, error) {
	return r.inner.PRTrustEvents(repo, number)
}

func (r *readOnlyForge) IssueTrustEvents(repo ForgeRepo, number int) (*TrustPayload, error) {
	return r.inner.IssueTrustEvents(repo, number)
}

func (r *readOnlyForge) IssueContentEvents(repo ForgeRepo, number int) (*TrustPayload, error) {
	return r.inner.IssueContentEvents(repo, number)
}

func (r *readOnlyForge) ReviewsAtHead(repo ForgeRepo, number int) ([]Review, error) {
	return r.inner.ReviewsAtHead(repo, number)
}

func (r *readOnlyForge) ReviewQueueSnapshot(repo ForgeRepo) (*ReviewQueue, error) {
	return r.inner.ReviewQueueSnapshot(repo)
}

func (r *readOnlyForge) ListChangedFiles(repo ForgeRepo, number int) ([]ChangedFile, error) {
	return r.inner.ListChangedFiles(repo, number)
}

func (r *readOnlyForge) ChecksAtHead(repo ForgeRepo, sha string) (*ChecksAtHead, error) {
	return r.inner.ChecksAtHead(repo, sha)
}

func (r *readOnlyForge) RequiredStatusChecks(repo ForgeRepo, branch string) ([]string, error) {
	return r.inner.RequiredStatusChecks(repo, branch)
}

func (r *readOnlyForge) IssueReactions(repo ForgeRepo, number int) ([]Reaction, error) {
	return r.inner.IssueReactions(repo, number)
}

func (r *readOnlyForge) ListLabelEvents(repo ForgeRepo, number int) ([]LabelEvent, error) {
	return r.inner.ListLabelEvents(repo, number)
}

func (r *readOnlyForge) ListIssueLabelEvents(repo ForgeRepo, number int) ([]LabelEvent, error) {
	return r.inner.ListIssueLabelEvents(repo, number)
}

func (r *readOnlyForge) ListComments(repo ForgeRepo, number int) ([]Comment, error) {
	return r.inner.ListComments(repo, number)
}

func (r *readOnlyForge) ListCommentsTyped(repo ForgeRepo, number int, kind TargetKind) ([]Comment, error) {
	return r.inner.ListCommentsTyped(repo, number, kind)
}

func (r *readOnlyForge) RepoVisibility(repo ForgeRepo) (string, error) {
	return r.inner.RepoVisibility(repo)
}

func (r *readOnlyForge) ReadFile(repo ForgeRepo, in ReadFileInput) (*FileContent, error) {
	return r.inner.ReadFile(repo, in)
}

func (r *readOnlyForge) ListRecentCommits(repo ForgeRepo, limit int) ([]RepoCommit, error) {
	return r.inner.ListRecentCommits(repo, limit)
}

func (r *readOnlyForge) GetCommit(repo ForgeRepo, sha string) (*RepoCommit, error) {
	return r.inner.GetCommit(repo, sha)
}

func (r *readOnlyForge) ListFileCommits(repo ForgeRepo, ref string, file string, limit int) ([]RepoCommit, error) {
	return r.inner.ListFileCommits(repo, ref, file, limit)
}

func (r *readOnlyForge) ListCommitChanges(repo ForgeRepo, sha string) ([]int, error) {
	return r.inner.ListCommitChanges(repo, sha)
}

func (r *readOnlyForge) CompareRefs(repo ForgeRepo, base string, head string) (*RefComparison, error) {
	return r.inner.CompareRefs(repo, base, head)
}

func (r *readOnlyForge) SearchOpenChanges(owner string) (*ChangeSearchResults, error) {
	return r.inner.SearchOpenChanges(owner)
}

func (r *readOnlyForge) ListWorkflowFiles(repo ForgeRepo, ref string) ([]string, error) {
	return r.inner.ListWorkflowFiles(repo, ref)
}

func (r *readOnlyForge) ChangeDiff(repo ForgeRepo, number int) (string, error) {
	return r.inner.ChangeDiff(repo, number)
}

func (r *readOnlyForge) RefExists(repo ForgeRepo, ref string) (bool, error) {
	return r.inner.RefExists(repo, ref)
}

func (r *readOnlyForge) MatchingRefs(repo ForgeRepo, refPrefix string) ([]string, error) {
	return r.inner.MatchingRefs(repo, refPrefix)
}

func (r *readOnlyForge) RepoHardeningRead(repo ForgeRepo, kind HardeningReadKind) (json.RawMessage, error) {
	return r.inner.RepoHardeningRead(repo, kind)
}

func (r *readOnlyForge) ReadMergeHold(repo ForgeRepo, number int) (*MergeHold, error) {
	return r.inner.ReadMergeHold(repo, number)
}

func (r *readOnlyForge) RunStatus(repo ForgeRepo, run RunRef) (*RunState, error) {
	return r.inner.RunStatus(repo, run)
}

func (r *readOnlyForge) PushTransportHint(repo ForgeRepo) PushTransport {
	return r.inner.PushTransportHint(repo)
}

// ---- everything else: refused, the inner forge never touched ----

func (r *readOnlyForge) CreateDraftChange(repo ForgeRepo, in DraftChangeInput) (*PullRef, error) {
	return nil, readOnlyRefusal("CreateDraftChange")
}

func (r *readOnlyForge) OpenMergeHold(repo ForgeRepo, number int) (string, error) {
	return "", readOnlyRefusal("OpenMergeHold")
}

func (r *readOnlyForge) SetMergeHold(repo ForgeRepo, number int, in MergeHoldUpdate) error {
	return readOnlyRefusal("SetMergeHold")
}

func (r *readOnlyForge) EditChange(repo ForgeRepo, number int, in EditChangeInput) error {
	return readOnlyRefusal("EditChange")
}

func (r *readOnlyForge) PostComment(repo ForgeRepo, number int, body string) (*CommentRef, error) {
	return nil, readOnlyRefusal("PostComment")
}

func (r *readOnlyForge) PostCommentTyped(repo ForgeRepo, number int, kind TargetKind, body string) (*CommentRef, error) {
	return nil, readOnlyRefusal("PostCommentTyped")
}

func (r *readOnlyForge) PostReview(repo ForgeRepo, number int, in ReviewInput) error {
	return readOnlyRefusal("PostReview")
}

func (r *readOnlyForge) MarkReadyForReview(nodeID string) error {
	return readOnlyRefusal("MarkReadyForReview")
}

func (r *readOnlyForge) ApplyLabels(repo ForgeRepo, number int, change LabelChange) (*LabelOutcome, error) {
	return nil, readOnlyRefusal("ApplyLabels")
}

func (r *readOnlyForge) EditComment(repo ForgeRepo, commentID string, body string) error {
	return readOnlyRefusal("EditComment")
}

func (r *readOnlyForge) FileIssue(repo ForgeRepo, in IssueInput) (*IssueRef, error) {
	return nil, readOnlyRefusal("FileIssue")
}

func (r *readOnlyForge) CloseIssue(repo ForgeRepo, number int, stateReason string) error {
	return readOnlyRefusal("CloseIssue")
}

func (r *readOnlyForge) CloseIssueTyped(repo ForgeRepo, number int, kind TargetKind, stateReason string) error {
	return readOnlyRefusal("CloseIssueTyped")
}

func (r *readOnlyForge) ReopenIssue(repo ForgeRepo, number int) error {
	return readOnlyRefusal("ReopenIssue")
}

func (r *readOnlyForge) WriteFile(repo ForgeRepo, in WriteFileInput) (*WriteFileResult, error) {
	return nil, readOnlyRefusal("WriteFile")
}

func (r *readOnlyForge) DeleteRef(repo ForgeRepo, ref string) error {
	return readOnlyRefusal("DeleteRef")
}

func (r *readOnlyForge) RunWorkflow(repo ForgeRepo, in RunWorkflowInput) (RunRef, error) {
	return RunRef{}, readOnlyRefusal("RunWorkflow")
}

func (r *readOnlyForge) ApproveGate(repo ForgeRepo, run RunRef, in ApproveGateInput) error {
	return readOnlyRefusal("ApproveGate")
}
