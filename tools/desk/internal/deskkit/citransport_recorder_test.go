package deskkit

// citransport_recorder_test.go — a recording Forge that embeds nothing: one method per Forge method,
// each recording its own NAME and returning zero values. A decorator test uses it to say which
// methods reached the inner forge. The compile-time assertion below fails when Forge gains a method,
// until the method is written out here too.

import (
	"encoding/json"
	"sync"
)

var _ Forge = (*ciRecordingForge)(nil)
var _ = json.RawMessage(nil)

type ciRecordingForge struct {
	mu    sync.Mutex
	calls []string
}

func (f *ciRecordingForge) rec(name string) {
	f.mu.Lock()
	f.calls = append(f.calls, name)
	f.mu.Unlock()
}

func (f *ciRecordingForge) called() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func (f *ciRecordingForge) GetPullRequest(repo ForgeRepo, number int) (*PullRequest, error) {
	f.rec("GetPullRequest")
	return nil, nil
}

func (f *ciRecordingForge) GetIssue(repo ForgeRepo, number int) (*Issue, error) {
	f.rec("GetIssue")
	return nil, nil
}

func (f *ciRecordingForge) GetIssueTyped(repo ForgeRepo, number int, kind TargetKind) (*Issue, error) {
	f.rec("GetIssueTyped")
	return nil, nil
}

func (f *ciRecordingForge) OpenChangeForBranch(repo ForgeRepo, branch string) (*PullRequest, error) {
	f.rec("OpenChangeForBranch")
	return nil, nil
}

func (f *ciRecordingForge) SearchIssues(repo ForgeRepo, in SearchIssuesInput) ([]IssueSearchResult, error) {
	f.rec("SearchIssues")
	return nil, nil
}

func (f *ciRecordingForge) ListLabels(repo ForgeRepo) ([]string, error) {
	f.rec("ListLabels")
	return nil, nil
}

func (f *ciRecordingForge) ListOpenChanges(repo ForgeRepo) (*OpenChanges, error) {
	f.rec("ListOpenChanges")
	return nil, nil
}

func (f *ciRecordingForge) ListChanges(repo ForgeRepo, states ChangeStates) (*ChangeList, error) {
	f.rec("ListChanges")
	return nil, nil
}

func (f *ciRecordingForge) ListOpenIssues(repo ForgeRepo) ([]IssueSummary, error) {
	f.rec("ListOpenIssues")
	return nil, nil
}

func (f *ciRecordingForge) ListIssues(repo ForgeRepo, in IssueListQuery) (*IssueList, error) {
	f.rec("ListIssues")
	return nil, nil
}

func (f *ciRecordingForge) IssueStateEvents(repo ForgeRepo, number int) (*IssueStateHistory, error) {
	f.rec("IssueStateEvents")
	return nil, nil
}

func (f *ciRecordingForge) ListChangeCommits(repo ForgeRepo, number int) (*ChangeCommits, error) {
	f.rec("ListChangeCommits")
	return nil, nil
}

func (f *ciRecordingForge) RepoDefaultBranch(repo ForgeRepo) (string, error) {
	f.rec("RepoDefaultBranch")
	return "", nil
}

func (f *ciRecordingForge) PRTrustEvents(repo ForgeRepo, number int) (*TrustPayload, error) {
	f.rec("PRTrustEvents")
	return nil, nil
}

func (f *ciRecordingForge) IssueTrustEvents(repo ForgeRepo, number int) (*TrustPayload, error) {
	f.rec("IssueTrustEvents")
	return nil, nil
}

func (f *ciRecordingForge) IssueContentEvents(repo ForgeRepo, number int) (*TrustPayload, error) {
	f.rec("IssueContentEvents")
	return nil, nil
}

func (f *ciRecordingForge) ReviewsAtHead(repo ForgeRepo, number int) ([]Review, error) {
	f.rec("ReviewsAtHead")
	return nil, nil
}

func (f *ciRecordingForge) ReviewQueueSnapshot(repo ForgeRepo) (*ReviewQueue, error) {
	f.rec("ReviewQueueSnapshot")
	return nil, nil
}

func (f *ciRecordingForge) ListChangedFiles(repo ForgeRepo, number int) ([]ChangedFile, error) {
	f.rec("ListChangedFiles")
	return nil, nil
}

func (f *ciRecordingForge) ChecksAtHead(repo ForgeRepo, sha string) (*ChecksAtHead, error) {
	f.rec("ChecksAtHead")
	return nil, nil
}

func (f *ciRecordingForge) RequiredStatusChecks(repo ForgeRepo, branch string) ([]string, error) {
	f.rec("RequiredStatusChecks")
	return nil, nil
}

func (f *ciRecordingForge) IssueReactions(repo ForgeRepo, number int) ([]Reaction, error) {
	f.rec("IssueReactions")
	return nil, nil
}

func (f *ciRecordingForge) ListLabelEvents(repo ForgeRepo, number int) ([]LabelEvent, error) {
	f.rec("ListLabelEvents")
	return nil, nil
}

func (f *ciRecordingForge) ListIssueLabelEvents(repo ForgeRepo, number int) ([]LabelEvent, error) {
	f.rec("ListIssueLabelEvents")
	return nil, nil
}

func (f *ciRecordingForge) ListComments(repo ForgeRepo, number int) ([]Comment, error) {
	f.rec("ListComments")
	return nil, nil
}

func (f *ciRecordingForge) ListCommentsTyped(repo ForgeRepo, number int, kind TargetKind) ([]Comment, error) {
	f.rec("ListCommentsTyped")
	return nil, nil
}

func (f *ciRecordingForge) RepoVisibility(repo ForgeRepo) (string, error) {
	f.rec("RepoVisibility")
	return "", nil
}

func (f *ciRecordingForge) ReadFile(repo ForgeRepo, in ReadFileInput) (*FileContent, error) {
	f.rec("ReadFile")
	return nil, nil
}

func (f *ciRecordingForge) ListRecentCommits(repo ForgeRepo, limit int) ([]RepoCommit, error) {
	f.rec("ListRecentCommits")
	return nil, nil
}

func (f *ciRecordingForge) GetCommit(repo ForgeRepo, sha string) (*RepoCommit, error) {
	f.rec("GetCommit")
	return nil, nil
}

func (f *ciRecordingForge) ListFileCommits(repo ForgeRepo, ref string, file string, limit int) ([]RepoCommit, error) {
	f.rec("ListFileCommits")
	return nil, nil
}

func (f *ciRecordingForge) ListCommitChanges(repo ForgeRepo, sha string) ([]int, error) {
	f.rec("ListCommitChanges")
	return nil, nil
}

func (f *ciRecordingForge) CompareRefs(repo ForgeRepo, base string, head string) (*RefComparison, error) {
	f.rec("CompareRefs")
	return nil, nil
}

func (f *ciRecordingForge) SearchOpenChanges(owner string) (*ChangeSearchResults, error) {
	f.rec("SearchOpenChanges")
	return nil, nil
}

func (f *ciRecordingForge) ListWorkflowFiles(repo ForgeRepo, ref string) ([]string, error) {
	f.rec("ListWorkflowFiles")
	return nil, nil
}

func (f *ciRecordingForge) ChangeDiff(repo ForgeRepo, number int) (string, error) {
	f.rec("ChangeDiff")
	return "", nil
}

func (f *ciRecordingForge) RefExists(repo ForgeRepo, ref string) (bool, error) {
	f.rec("RefExists")
	return false, nil
}

func (f *ciRecordingForge) MatchingRefs(repo ForgeRepo, refPrefix string) ([]string, error) {
	f.rec("MatchingRefs")
	return nil, nil
}

func (f *ciRecordingForge) RepoHardeningRead(repo ForgeRepo, kind HardeningReadKind) (json.RawMessage, error) {
	f.rec("RepoHardeningRead")
	return nil, nil
}

func (f *ciRecordingForge) ReadMergeHold(repo ForgeRepo, number int) (*MergeHold, error) {
	f.rec("ReadMergeHold")
	return nil, nil
}

func (f *ciRecordingForge) CreateDraftChange(repo ForgeRepo, in DraftChangeInput) (*PullRef, error) {
	f.rec("CreateDraftChange")
	return nil, nil
}

func (f *ciRecordingForge) OpenMergeHold(repo ForgeRepo, number int) (string, error) {
	f.rec("OpenMergeHold")
	return "", nil
}

func (f *ciRecordingForge) SetMergeHold(repo ForgeRepo, number int, in MergeHoldUpdate) error {
	f.rec("SetMergeHold")
	return nil
}

func (f *ciRecordingForge) EditChange(repo ForgeRepo, number int, in EditChangeInput) error {
	f.rec("EditChange")
	return nil
}

func (f *ciRecordingForge) PostComment(repo ForgeRepo, number int, body string) (*CommentRef, error) {
	f.rec("PostComment")
	return nil, nil
}

func (f *ciRecordingForge) PostCommentTyped(repo ForgeRepo, number int, kind TargetKind, body string) (*CommentRef, error) {
	f.rec("PostCommentTyped")
	return nil, nil
}

func (f *ciRecordingForge) PostReview(repo ForgeRepo, number int, in ReviewInput) error {
	f.rec("PostReview")
	return nil
}

func (f *ciRecordingForge) MarkReadyForReview(nodeID string) error {
	f.rec("MarkReadyForReview")
	return nil
}

func (f *ciRecordingForge) ApplyLabels(repo ForgeRepo, number int, change LabelChange) (*LabelOutcome, error) {
	f.rec("ApplyLabels")
	return nil, nil
}

func (f *ciRecordingForge) EditComment(repo ForgeRepo, commentID string, body string) error {
	f.rec("EditComment")
	return nil
}

func (f *ciRecordingForge) FileIssue(repo ForgeRepo, in IssueInput) (*IssueRef, error) {
	f.rec("FileIssue")
	return nil, nil
}

func (f *ciRecordingForge) CloseIssue(repo ForgeRepo, number int, stateReason string) error {
	f.rec("CloseIssue")
	return nil
}

func (f *ciRecordingForge) CloseIssueTyped(repo ForgeRepo, number int, kind TargetKind, stateReason string) error {
	f.rec("CloseIssueTyped")
	return nil
}

func (f *ciRecordingForge) ReopenIssue(repo ForgeRepo, number int) error {
	f.rec("ReopenIssue")
	return nil
}

func (f *ciRecordingForge) WriteFile(repo ForgeRepo, in WriteFileInput) (*WriteFileResult, error) {
	f.rec("WriteFile")
	return nil, nil
}

func (f *ciRecordingForge) DeleteRef(repo ForgeRepo, ref string) error {
	f.rec("DeleteRef")
	return nil
}

func (f *ciRecordingForge) RunWorkflow(repo ForgeRepo, in RunWorkflowInput) (RunRef, error) {
	f.rec("RunWorkflow")
	return RunRef{}, nil
}

func (f *ciRecordingForge) ApproveGate(repo ForgeRepo, run RunRef, in ApproveGateInput) error {
	f.rec("ApproveGate")
	return nil
}

func (f *ciRecordingForge) RunStatus(repo ForgeRepo, run RunRef) (*RunState, error) {
	f.rec("RunStatus")
	return nil, nil
}

func (f *ciRecordingForge) RunLog(repo ForgeRepo, run RunRef) ([]RunLogPart, error) {
	f.rec("RunLog")
	return nil, nil
}

func (f *ciRecordingForge) RetryRun(repo ForgeRepo, run RunRef) error {
	f.rec("RetryRun")
	return nil
}

func (f *ciRecordingForge) PushTransportHint(repo ForgeRepo) PushTransport {
	f.rec("PushTransportHint")
	return PushTransport{}
}
