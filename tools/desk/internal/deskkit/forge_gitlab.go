package deskkit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// forge_gitlab.go — the GitLab implementation of Forge, against GitLab REST v4. It is the
// second forge the seam was extracted for (brief 02): every fleet operation the
// `github` implementation pins by goldens, mapped to GitLab's concept vocabulary (spec §6)
// with the SAME observable semantics, so the difference between the forges lives here and
// not in a fork of every desk tool.
//
// It is handed an ALREADY-MINTED token (a rotate-on-mint PAT; custody is brief 03's,
// this file consumes the token VALUE only) and authenticates with the `PRIVATE-TOKEN`
// header. It NEVER constructs a URL that embeds the token — the push-transport hint carries
// the transport SHAPE only (see PushTransportHint).
//
// Concept mapping (spec §6, inventory.md GitLab column), where GitLab differs from GitHub:
//
//   - draft PR ↔ MR with a `Draft:` title prefix (GitLab has no draft boolean in REST v4;
//     the flag is DERIVED from the title, so MarkReadyForReview edits the title).
//   - review approval ↔ the MR approve endpoint, head-pinned by its `sha` parameter (GitLab
//     REJECTS the approval when sha != source HEAD — a STRONGER pin than GitHub's, which
//     merely records the commit the review was against).
//   - required check ↔ pipeline status at the head SHA, with the per-job rollup and the
//     external/commit-status rollup as the two ChecksAtHead lists.
//   - reaction admission gate ↔ award emoji; the GitLab award name is NORMALIZED to the
//     GitHub reaction vocabulary (thumbsup→+1, thumbsdown→-1) so the forge-agnostic gate
//     sees one vocabulary across forges.
//   - `Fixes #N` ↔ `Closes #N` (this is a body-text convention the tools own, not an API op).
//
// Divergences that are STRUCTURAL, not stylistic — GitLab keeps issues and merge requests in
// SEPARATE iid sequences (an iid is never both), unlike GitHub's single number space:
//
//   - GetIssue reports IsPullRequest=false unconditionally: the issues endpoint serves only
//     issues, so the GitHub discriminator (an issue number that is really a PR) has no GitLab
//     analog. A caller that needs to know a number is an MR calls GetPullRequest.
//   - PostComment cannot know from the number alone whether it addresses an issue or an MR,
//     and iids COLLIDE across the two sequences. It tries the MR note endpoint first and
//     falls back to the issue note endpoint on 404. See PostComment for the caveat.

// GitLabAPIBase is the single home of the GitLab REST v4 host literal — the "host literal in
// the forge module" contract the seam is built on (see forge.go's GitHubAPIBase). The
// default targets gitlab.com; a self-managed EE instance points BaseURL at its own
// `https://<host>/api/v4`.
const GitLabAPIBase = "https://gitlab.com/api/v4"

// GitLabForge implements Forge against the GitLab REST v4 API with a PAT. Same shape as
// GitHubForge: BaseURL defaults to GitLabAPIBase, Client to http.DefaultClient, so a test
// points it at an httptest server.
type GitLabForge struct {
	Token   string
	BaseURL string
	Client  *http.Client
}

var _ Forge = (*GitLabForge)(nil)

func (g *GitLabForge) baseURL() string {
	if g.BaseURL != "" {
		return g.BaseURL
	}
	return GitLabAPIBase
}

func (g *GitLabForge) client() *http.Client {
	if g.Client != nil {
		return g.Client
	}
	return http.DefaultClient
}

// projID renders a repo coordinate as the URL-encoded project id GitLab addresses a project
// by: "owner/name" → "owner%2Fname". GitLab accepts either the numeric id or the encoded
// full path; the fleet knows the path, not the id.
func projID(repo ForgeRepo) string {
	return url.PathEscape(repo.Slug())
}

// gitlabNodeID synthesizes the opaque node id this implementation puts in PullRequest.NodeID
// and PullRef.NodeID and consumes in MarkReadyForReview. GitLab REST has no GraphQL node id,
// so the seam's opaque id encodes the two coordinates the flip needs — the project path and
// the MR iid — in GitLab's own `path!iid` MR-reference shape. It is opaque to callers: only
// this implementation, which produced it, interprets it.
func gitlabNodeID(repo ForgeRepo, iid int) string {
	return repo.Slug() + "!" + strconv.Itoa(iid)
}

// parseGitlabNodeID reverses gitlabNodeID.
func parseGitlabNodeID(nodeID string) (ForgeRepo, int, error) {
	bang := strings.LastIndex(nodeID, "!")
	if bang < 0 {
		return ForgeRepo{}, 0, Unverifiable("gitlab node id has no '!' separator: "+nodeID, nil)
	}
	slug, iidStr := nodeID[:bang], nodeID[bang+1:]
	slash := strings.LastIndex(slug, "/")
	if slash < 0 {
		return ForgeRepo{}, 0, Unverifiable("gitlab node id has no owner/name in "+nodeID, nil)
	}
	iid, err := strconv.Atoi(iidStr)
	if err != nil {
		return ForgeRepo{}, 0, Unverifiable("gitlab node id has non-numeric iid in "+nodeID, err)
	}
	return ForgeRepo{Owner: slug[:slash], Name: slug[slash+1:]}, iid, nil
}

// do performs one REST call and returns the raw 2xx body plus the response headers (GitLab's
// pagination lives in X-Next-Page / X-Total headers, not link relations — brief fact). A
// non-2xx is a *ForgeAPIError carrying the STATUS, so a caller distinguishes 401 (credential),
// 403 (permission/tier), and 404 (visibility) — a tier-gated feature's error surfaces as
// could-not-check via ForgeCheckState, never as clean. A transport/marshal failure is
// Unverifiable.
func (g *GitLabForge) do(method, path string, in any) ([]byte, http.Header, error) {
	var bodyReader io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return nil, nil, Unverifiable("cannot marshal request body", err)
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, g.baseURL()+path, bodyReader)
	if err != nil {
		return nil, nil, Unverifiable("cannot build request", err)
	}
	req.Header.Set("PRIVATE-TOKEN", g.Token)
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := g.client().Do(req)
	if err != nil {
		return nil, nil, Unverifiable(fmt.Sprintf("%s %s failed", method, path), err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.Header, &ForgeAPIError{Status: resp.StatusCode, Method: method, Path: path}
	}
	return raw, resp.Header, nil
}

// doJSON is do plus a JSON decode of a 2xx body into out (if non-nil). Returns the response
// headers so a paginating caller can read X-Next-Page / X-Total.
func (g *GitLabForge) doJSON(method, path string, in, out any) (http.Header, error) {
	raw, hdr, err := g.do(method, path, in)
	if err != nil {
		return hdr, err
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return hdr, Unverifiable(fmt.Sprintf("cannot parse %s %s response", method, path), err)
		}
	}
	return hdr, nil
}

// ForgeCheckState classifies a forge operation's error into the three-state instrument
// vocabulary: "checked-clean" (nil error — the operation positively completed) or
// "could-not-check" (ANY API/transport error — a precondition that could not be POSITIVELY
// verified). It deliberately never returns "checked-failed": a failed CHECK (CI red, an
// unapproved MR) is read from a SUCCESSFUL response body (e.g. ChecksAtHead.CombinedState ==
// "failure"), not from an error. A GitLab tier-gated feature (approval rules, external status
// checks) that 401/403/404s therefore surfaces as could-not-check — never rounded up to
// clean. It is forge-neutral (both GitHubForge and GitLabForge return *ForgeAPIError).
func ForgeCheckState(err error) string {
	if err == nil {
		return "checked-clean"
	}
	return "could-not-check"
}

// --- REST wire shapes (only the fields consumed) ---

type glUser struct {
	Username string `json:"username"`
	ID       int64  `json:"id"`
}

type glMRWire struct {
	IID          int    `json:"iid"`
	State        string `json:"state"` // opened | closed | merged | locked
	Draft        bool   `json:"draft"`
	Title        string `json:"title"`
	SHA          string `json:"sha"`           // head of the source branch
	ChangesCount string `json:"changes_count"` // GitLab returns this as a STRING (e.g. "3", or "3+")
	Author       glUser `json:"author"`
	WebURL       string `json:"web_url"`
}

type glIssueWire struct {
	IID    int    `json:"iid"`
	State  string `json:"state"` // opened | closed
	Author glUser `json:"author"`
	WebURL string `json:"web_url"`
}

type glApprovalsWire struct {
	ApprovedBy []struct {
		User glUser `json:"user"`
	} `json:"approved_by"`
}

type glNoteWire struct {
	ID        int64  `json:"id"`
	Body      string `json:"body"`
	Author    glUser `json:"author"`
	System    bool   `json:"system"`
	CreatedAt string `json:"created_at"`
}

type glDiffWire struct {
	OldPath     string `json:"old_path"`
	NewPath     string `json:"new_path"`
	NewFile     bool   `json:"new_file"`
	RenamedFile bool   `json:"renamed_file"`
	DeletedFile bool   `json:"deleted_file"`
}

type glPipelineWire struct {
	ID     int64  `json:"id"`
	SHA    string `json:"sha"`
	Status string `json:"status"`
}

type glJobWire struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type glCommitStatusWire struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type glAwardWire struct {
	Name string `json:"name"`
	User glUser `json:"user"`
}

// Pagination bounds — parallel to the GitHub implementation's, and for the same
// fail-closed reason: a walk that stops early leaves len < the asserted total. GitLab's
// default page size is 20, so an omitted per_page silently truncates; every walk sends
// per_page=100 and follows X-Next-Page to exhaustion (bounded).
const (
	glPerPage      = 100
	glMaxFilePages = 40 // 4000 diff entries
	glMaxNotePages = 40
	glMaxCIPages   = 25 // 2500 items
)

// --- state / vocabulary mapping (spec §6) ---

// mapMRState maps a GitLab MR state to the seam's open|closed. opened|locked → open (a locked
// MR is still open); closed|merged → closed.
func mapMRState(s string) string {
	switch s {
	case "opened", "locked":
		return "open"
	default: // closed, merged
		return "closed"
	}
}

// mapIssueState maps a GitLab issue state to open|closed.
func mapIssueState(s string) string {
	if s == "opened" {
		return "open"
	}
	return "closed"
}

// mapPipelineState maps a GitLab pipeline/commit-status state to the GitHub combined-status
// vocabulary success|pending|failure|error. The mapping fails toward NOT-green: anything not
// positively successful and not obviously terminal-failed is pending, so a caller that gates
// on "== success" never merges on an in-flight or cancelled pipeline.
func mapPipelineState(s string) string {
	switch s {
	case "success":
		return "success"
	case "skipped":
		return "success" // a skipped required pipeline is not a block
	case "failed":
		return "failure"
	case "canceled", "cancelled":
		return "error"
	default: // created, waiting_for_resource, preparing, pending, running, manual, scheduled
		return "pending"
	}
}

// mapJobStatus maps a GitLab job status to the check-run status vocabulary
// queued|in_progress|completed.
func mapJobStatus(s string) string {
	switch s {
	case "running":
		return "in_progress"
	case "success", "failed", "canceled", "cancelled", "skipped":
		return "completed"
	default: // created, pending, waiting_for_resource, preparing, manual, scheduled
		return "queued"
	}
}

// mapJobConclusion maps a GitLab job status to the check-run conclusion vocabulary. Empty for
// a job that has not concluded.
func mapJobConclusion(s string) string {
	switch s {
	case "success":
		return "success"
	case "failed":
		return "failure"
	case "canceled", "cancelled":
		return "cancelled"
	case "skipped":
		return "skipped"
	case "manual":
		return "neutral"
	default:
		return ""
	}
}

// mapAwardName normalizes a GitLab award-emoji name to the GitHub reaction vocabulary so the
// forge-agnostic admission gate (which looks for "+1") sees one vocabulary across forges. An
// unrecognised award passes through unchanged.
func mapAwardName(name string) string {
	switch name {
	case "thumbsup":
		return "+1"
	case "thumbsdown":
		return "-1"
	default:
		return name
	}
}

// --- Reads ---

func (g *GitLabForge) GetPullRequest(repo ForgeRepo, number int) (*PullRequest, error) {
	var w glMRWire
	path := fmt.Sprintf("/projects/%s/merge_requests/%d", projID(repo), number)
	if _, err := g.doJSON(http.MethodGet, path, nil, &w); err != nil {
		return nil, err
	}
	// changes_count is a string ("3", or "3+" when truncated); parse the leading integer,
	// defaulting to 0 when absent or non-numeric (the caller reconciles against
	// ListChangedFiles anyway).
	changed := 0
	if n := strings.TrimRight(w.ChangesCount, "+"); n != "" {
		if v, err := strconv.Atoi(n); err == nil {
			changed = v
		}
	}
	return &PullRequest{
		Number:       w.IID,
		State:        mapMRState(w.State),
		Draft:        w.Draft,
		NodeID:       gitlabNodeID(repo, w.IID),
		ChangedFiles: changed,
		Author:       Account{Login: w.Author.Username, ID: w.Author.ID},
		HeadSHA:      w.SHA,
	}, nil
}

// GetIssue reads an issue. IsPullRequest is ALWAYS false on GitLab: issues and merge requests
// occupy separate iid sequences, so an issue iid is never a merge request — the GitHub
// discriminator has no GitLab analog. A caller that must know a number is an MR calls
// GetPullRequest.
func (g *GitLabForge) GetIssue(repo ForgeRepo, number int) (*Issue, error) {
	var w glIssueWire
	path := fmt.Sprintf("/projects/%s/issues/%d", projID(repo), number)
	if _, err := g.doJSON(http.MethodGet, path, nil, &w); err != nil {
		return nil, err
	}
	return &Issue{
		Number:        w.IID,
		State:         mapIssueState(w.State),
		Author:        Account{Login: w.Author.Username, ID: w.Author.ID},
		IsPullRequest: false,
	}, nil
}

// ReviewsAtHead reconstructs the GitHub review set from GitLab's TWO surfaces — MR approvals
// (the verdict STATE) and MR notes (the verdict BODY) — the "approvals + notes" mapping the
// inventory names. Each approval becomes an APPROVED review stamped with the MR's current head
// SHA (GitLab's approvals endpoint carries no per-approval sha; the pin is deployment-provided
// by reset-approvals-on-push, spec §3 — an honest could-not-check nuance, documented). Each
// non-system note becomes a COMMENTED review. GitLab has no CHANGES_REQUESTED approval state
// (it is expressed as an unresolved blocking discussion, out of this read's scope).
func (g *GitLabForge) ReviewsAtHead(repo ForgeRepo, number int) ([]Review, error) {
	pid := projID(repo)
	// Head SHA to pin the approvals against.
	var mr glMRWire
	if _, err := g.doJSON(http.MethodGet, fmt.Sprintf("/projects/%s/merge_requests/%d", pid, number), nil, &mr); err != nil {
		return nil, err
	}
	var out []Review
	// Approvals → APPROVED reviews.
	var appr glApprovalsWire
	if _, err := g.doJSON(http.MethodGet, fmt.Sprintf("/projects/%s/merge_requests/%d/approvals", pid, number), nil, &appr); err != nil {
		return nil, err
	}
	for _, a := range appr.ApprovedBy {
		out = append(out, Review{
			ID:       a.User.ID, // approvals carry no review id; the approver id is the stable handle
			Author:   Account{Login: a.User.Username, ID: a.User.ID},
			State:    "APPROVED",
			CommitID: mr.SHA,
		})
	}
	// Notes → COMMENTED reviews (paginated; skip system notes).
	for page := 1; page <= glMaxNotePages; page++ {
		var chunk []glNoteWire
		path := fmt.Sprintf("/projects/%s/merge_requests/%d/notes?per_page=%d&page=%d", pid, number, glPerPage, page)
		hdr, err := g.doJSON(http.MethodGet, path, nil, &chunk)
		if err != nil {
			return nil, err
		}
		for _, n := range chunk {
			if n.System {
				continue
			}
			out = append(out, Review{
				ID:          n.ID,
				Author:      Account{Login: n.Author.Username, ID: n.Author.ID},
				State:       "COMMENTED",
				CommitID:    mr.SHA,
				Body:        n.Body,
				SubmittedAt: n.CreatedAt,
			})
		}
		if hdr.Get("X-Next-Page") == "" {
			break
		}
	}
	return out, nil
}

// ListChangedFiles walks the MR diffs endpoint (the paginated successor to /changes),
// following X-Next-Page to exhaustion. Rename-aware: a renamed_file carries both new_path
// and old_path so the risk-path gate still sees the pre-rename path.
func (g *GitLabForge) ListChangedFiles(repo ForgeRepo, number int) ([]ChangedFile, error) {
	pid := projID(repo)
	var all []ChangedFile
	for page := 1; page <= glMaxFilePages; page++ {
		var chunk []glDiffWire
		path := fmt.Sprintf("/projects/%s/merge_requests/%d/diffs?per_page=%d&page=%d", pid, number, glPerPage, page)
		hdr, err := g.doJSON(http.MethodGet, path, nil, &chunk)
		if err != nil {
			return nil, err
		}
		for _, d := range chunk {
			all = append(all, ChangedFile{
				Filename:         d.NewPath,
				PreviousFilename: gitlabPrevFilename(d),
				Status:           gitlabFileStatus(d),
			})
		}
		if hdr.Get("X-Next-Page") == "" {
			break
		}
	}
	return all, nil
}

// gitlabPrevFilename mirrors GitHub's previous_filename: set only on a rename.
func gitlabPrevFilename(d glDiffWire) string {
	if d.RenamedFile {
		return d.OldPath
	}
	return ""
}

// gitlabFileStatus maps a GitLab diff entry's boolean flags to the GitHub status vocabulary.
// Order matters: a rename can also carry edits, and rename is the load-bearing status for the
// risk-path gate.
func gitlabFileStatus(d glDiffWire) string {
	switch {
	case d.RenamedFile:
		return "renamed"
	case d.NewFile:
		return "added"
	case d.DeletedFile:
		return "removed"
	default:
		return "modified"
	}
}

// ChecksAtHead maps GitLab's checks at a commit to the two-rollup shape. Statuses[] ← the
// commit-statuses endpoint (the external/commit-status analog of GitHub's combined status);
// CheckRuns[] ← the jobs of the latest pipeline at that SHA. CombinedState ← the latest
// pipeline's status (the authoritative "pipeline must succeed" signal). Each rollup's asserted
// total comes from GitLab's X-Total header so a caller can fail CLOSED when the walk read fewer
// entries than the head claims.
func (g *GitLabForge) ChecksAtHead(repo ForgeRepo, sha string) (*ChecksAtHead, error) {
	pid := projID(repo)
	out := &ChecksAtHead{}

	// Latest pipeline at the SHA → CombinedState.
	var pipes []glPipelineWire
	pipePath := fmt.Sprintf("/projects/%s/pipelines?sha=%s&order_by=id&sort=desc&per_page=1", pid, url.QueryEscape(sha))
	if _, err := g.doJSON(http.MethodGet, pipePath, nil, &pipes); err != nil {
		return nil, err
	}
	var pipelineID int64
	if len(pipes) > 0 {
		out.CombinedState = mapPipelineState(pipes[0].Status)
		pipelineID = pipes[0].ID
	}

	// Commit statuses → Statuses[] (paginated, X-Next-Page). X-Total asserts the count.
	for page := 1; page <= glMaxCIPages; page++ {
		var chunk []glCommitStatusWire
		path := fmt.Sprintf("/projects/%s/repository/commits/%s/statuses?per_page=%d&page=%d",
			pid, url.PathEscape(sha), glPerPage, page)
		hdr, err := g.doJSON(http.MethodGet, path, nil, &chunk)
		if err != nil {
			return nil, err
		}
		if page == 1 {
			out.StatusTotalCount = headerInt(hdr, "X-Total", -1)
		}
		for _, s := range chunk {
			out.Statuses = append(out.Statuses, StatusContext{State: mapPipelineState(s.Status), Context: s.Name})
		}
		if hdr.Get("X-Next-Page") == "" {
			break
		}
	}
	if out.StatusTotalCount < 0 {
		out.StatusTotalCount = len(out.Statuses)
	}

	// Pipeline jobs → CheckRuns[] (paginated). Only when a pipeline exists.
	if pipelineID != 0 {
		for page := 1; page <= glMaxCIPages; page++ {
			var chunk []glJobWire
			path := fmt.Sprintf("/projects/%s/pipelines/%d/jobs?per_page=%d&page=%d", pid, pipelineID, glPerPage, page)
			hdr, err := g.doJSON(http.MethodGet, path, nil, &chunk)
			if err != nil {
				return nil, err
			}
			if page == 1 {
				out.CheckRunsTotalCount = headerInt(hdr, "X-Total", -1)
			}
			for _, j := range chunk {
				out.CheckRuns = append(out.CheckRuns, CheckRun{
					Name:       j.Name,
					Status:     mapJobStatus(j.Status),
					Conclusion: mapJobConclusion(j.Status),
				})
			}
			if hdr.Get("X-Next-Page") == "" {
				break
			}
		}
	}
	if out.CheckRunsTotalCount < 0 {
		out.CheckRunsTotalCount = len(out.CheckRuns)
	}
	return out, nil
}

// headerInt reads an integer response header, returning def when absent or unparseable.
func headerInt(h http.Header, key string, def int) int {
	v := h.Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// IssueReactions reads the award emoji on an issue as the reaction admission surface. SINGLE
// PAGE by decision — parity with the GitHub implementation, which fails CLOSED past 100
// reactions rather than risk a false pass. Award names are normalized to the GitHub reaction
// vocabulary (thumbsup→+1) so the admission gate sees one vocabulary across forges.
func (g *GitLabForge) IssueReactions(repo ForgeRepo, number int) ([]Reaction, error) {
	path := fmt.Sprintf("/projects/%s/issues/%d/award_emoji?per_page=100", projID(repo), number)
	var awards []glAwardWire
	if _, err := g.doJSON(http.MethodGet, path, nil, &awards); err != nil {
		return nil, err
	}
	out := make([]Reaction, 0, len(awards))
	for _, a := range awards {
		out = append(out, Reaction{
			User:    ReactionUser{Login: a.User.Username, Type: "User"},
			Content: mapAwardName(a.Name),
		})
	}
	return out, nil
}

func (g *GitLabForge) RepoVisibility(repo ForgeRepo) (string, error) {
	var info struct {
		Visibility string `json:"visibility"`
	}
	path := fmt.Sprintf("/projects/%s", projID(repo))
	if _, err := g.doJSON(http.MethodGet, path, nil, &info); err != nil {
		return "", err
	}
	if info.Visibility == "" {
		return "", Unverifiable(fmt.Sprintf("project %s has no .visibility field in API response", repo.Slug()), nil)
	}
	return info.Visibility, nil
}

// --- Writes ---

// CreateDraftChange opens a merge request as a DRAFT — the frozen property. GitLab has no
// draft boolean in REST v4; the flag is derived from a `Draft:` title prefix, so the title is
// prefixed when it is not already.
func (g *GitLabForge) CreateDraftChange(repo ForgeRepo, in DraftChangeInput) (*PullRef, error) {
	path := fmt.Sprintf("/projects/%s/merge_requests", projID(repo))
	body := map[string]any{
		"title":         gitlabDraftTitle(in.Title),
		"description":   in.Body,
		"source_branch": in.Head,
		"target_branch": in.Base,
	}
	var w glMRWire
	if _, err := g.doJSON(http.MethodPost, path, body, &w); err != nil {
		return nil, err
	}
	return &PullRef{Number: w.IID, NodeID: gitlabNodeID(repo, w.IID), URL: w.WebURL}, nil
}

// gitlabDraftTitle ensures a `Draft:` prefix without doubling one that is already present
// (in either the modern `Draft:` or legacy `WIP:` spelling).
func gitlabDraftTitle(title string) string {
	if isGitlabDraftTitle(title) {
		return title
	}
	return "Draft: " + title
}

func isGitlabDraftTitle(title string) bool {
	t := strings.TrimSpace(title)
	lower := strings.ToLower(t)
	return strings.HasPrefix(lower, "draft:") || strings.HasPrefix(lower, "wip:")
}

// PostComment posts a note. The number's kind (issue vs MR) is NOT carried by the frozen
// interface, and on GitLab issue and MR iids COLLIDE (separate sequences). It tries the MR
// note endpoint first and falls back to the issue note endpoint only on a 404 — so a note
// meant for issue #N lands on MR #N when both exist. Callers whose context is issue-only
// should be aware of this; resolving it cleanly needs a kind hint on the interface, which the
// freeze rule defers to a change that has a consuming tool.
func (g *GitLabForge) PostComment(repo ForgeRepo, number int, body string) error {
	pid := projID(repo)
	mrPath := fmt.Sprintf("/projects/%s/merge_requests/%d/notes", pid, number)
	_, err := g.doJSON(http.MethodPost, mrPath, map[string]any{"body": body}, nil)
	if IsForgeNotFound(err) {
		issuePath := fmt.Sprintf("/projects/%s/issues/%d/notes", pid, number)
		_, err = g.doJSON(http.MethodPost, issuePath, map[string]any{"body": body}, nil)
	}
	return err
}

// PostReview submits a head-pinned verdict on an MR. The verdict BODY is posted as a note
// (GitLab's approve endpoint takes no body), then the EVENT is mapped:
//
//   - APPROVE → POST .../approve with `sha` = HeadSHA. GitLab REJECTS the approval when sha
//     != the source HEAD, so the head-pin is enforced by the FORGE, not merely recorded.
//   - REQUEST_CHANGES / COMMENT → the note alone. GitLab has no request-changes approval
//     state; changes-requested is an unresolved blocking discussion, which this write does not
//     fabricate (it would need a discussion, not a note).
func (g *GitLabForge) PostReview(repo ForgeRepo, number int, in ReviewInput) error {
	pid := projID(repo)
	if in.Body != "" {
		notePath := fmt.Sprintf("/projects/%s/merge_requests/%d/notes", pid, number)
		if _, err := g.doJSON(http.MethodPost, notePath, map[string]any{"body": in.Body}, nil); err != nil {
			return err
		}
	}
	if in.Event == "APPROVE" {
		approvePath := fmt.Sprintf("/projects/%s/merge_requests/%d/approve", pid, number)
		body := map[string]any{}
		if in.HeadSHA != "" {
			body["sha"] = in.HeadSHA // GitLab fails the approval if this != source HEAD
		}
		if _, err := g.doJSON(http.MethodPost, approvePath, body, nil); err != nil {
			return err
		}
	}
	return nil
}

// MarkReadyForReview clears an MR's draft status by removing the `Draft:` title prefix — the
// only representation of draft in REST v4. The opaque nodeID encodes the project + iid; the
// method reads the current title, strips the prefix, and PUTs the de-drafted title. A no-op
// when the MR is already not a draft.
func (g *GitLabForge) MarkReadyForReview(nodeID string) error {
	repo, iid, err := parseGitlabNodeID(nodeID)
	if err != nil {
		return err
	}
	pid := projID(repo)
	var mr glMRWire
	if _, err := g.doJSON(http.MethodGet, fmt.Sprintf("/projects/%s/merge_requests/%d", pid, iid), nil, &mr); err != nil {
		return err
	}
	if !isGitlabDraftTitle(mr.Title) {
		return nil // already ready
	}
	putPath := fmt.Sprintf("/projects/%s/merge_requests/%d", pid, iid)
	_, err = g.doJSON(http.MethodPut, putPath, map[string]any{"title": stripGitlabDraftPrefix(mr.Title)}, nil)
	return err
}

// stripGitlabDraftPrefix removes a leading Draft:/WIP: prefix (case-insensitive) and the
// whitespace after it, preserving the rest of the title verbatim.
func stripGitlabDraftPrefix(title string) string {
	t := strings.TrimSpace(title)
	for _, p := range []string{"draft:", "wip:"} {
		if strings.HasPrefix(strings.ToLower(t), p) {
			return strings.TrimSpace(t[len(p):])
		}
	}
	return t
}

func (g *GitLabForge) FileIssue(repo ForgeRepo, in IssueInput) (*IssueRef, error) {
	path := fmt.Sprintf("/projects/%s/issues", projID(repo))
	var w glIssueWire
	if _, err := g.doJSON(http.MethodPost, path, map[string]any{"title": in.Title, "description": in.Body}, &w); err != nil {
		return nil, err
	}
	return &IssueRef{Number: w.IID, URL: w.WebURL}, nil
}

// CloseIssue closes an issue via `state_event: "close"` (GitLab's mutation verb, not GitHub's
// `state: "closed"`). GitLab REST v4 has no close-REASON param on the issue update, so the
// GitHub stateReason (completed | not_planned) is accepted for signature parity but not sent;
// its absence is a documented divergence, not a dropped requirement.
func (g *GitLabForge) CloseIssue(repo ForgeRepo, number int, stateReason string) error {
	path := fmt.Sprintf("/projects/%s/issues/%d", projID(repo), number)
	_, err := g.doJSON(http.MethodPut, path, map[string]any{"state_event": "close"}, nil)
	return err
}

// --- Identity / transport ---

// PushTransportHint returns GitLab's push-transport shape: a PAT authenticates an https push
// as the username "oauth2", supplied via an inline credential.helper reading the token file —
// never a token-in-URL (classifier-blocked, and it leaks the secret into argv/reflog). The
// remote host is DERIVED from BaseURL (gitlab.com by default; a self-managed instance's own
// host otherwise), so a single-host hardcode does not break EE deployments. Pure function, no
// network call.
func (g *GitLabForge) PushTransportHint(repo ForgeRepo) PushTransport {
	return PushTransport{
		RemoteHost:    gitlabRemoteHost(g.baseURL()),
		TokenUsername: "oauth2",
		CredentialHelperHint: "supply the token via an inline credential.helper that reads the 0600 token " +
			"file; never embed it in the remote URL",
	}
}

// gitlabRemoteHost extracts the git host from an API base URL (https://gitlab.com/api/v4 →
// gitlab.com), falling back to gitlab.com when the base cannot be parsed.
func gitlabRemoteHost(base string) string {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return "gitlab.com"
	}
	return u.Host
}
