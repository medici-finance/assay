package deskkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// forge_gitlab_test.go — the contract corpus for the GitLab Forge implementation.
//
// It is the SAME instrument brief-01 built for the GitHub backend
// (forge_github_golden_test.go): each case drives one operation against a recorded REST
// fixture set and pins the full observable footprint — every request the implementation
// emitted (method, ESCAPED path, query, and body for writes), the mapped result, and the
// error classification. Scenario names mirror the GitHub corpus's wherever the operation is
// the same, so a gap between the backends reads as a named missing row rather than as
// silence.
//
// It is the single-point-of-failure control the brief names, and it earns that title only
// because the mapping is where GitLab support can go wrong: the transport is a maintained
// library and will not silently mis-send a request, but a wrong CONCEPT MAPPING — an
// approval stamped with a head it was not given at, a `thumbsup` that never becomes a `+1`,
// a bot's award typed as a human's — compiles, passes a happy-path test, and fails on a live
// group weeks later. Each such mapping has a case here whose fixture disagrees with the
// naive implementation.
//
// Regenerate on an INTENTIONAL change with
// `go test ./tools/desk/internal/deskkit/ -run TestForgeGitlabGolden -update`; an
// unintentional change shows as a golden diff.

// glServer is the recorded GitLab instance. Every field is a canned payload for one route;
// a case sets only what it needs.
type glServer struct {
	srv      *httptest.Server
	requests []reqCapture

	project      map[string]any
	projApproval map[string]any
	mr           map[string]any
	mrMissing    bool
	issue        map[string]any
	issueMissing bool
	versions     []map[string]any
	approvals    map[string]any
	notes        []map[string]any
	diffs        []map[string]any
	commit       map[string]any
	statuses     []map[string]any
	jobs         []map[string]any
	awards       []map[string]any
	users        map[string]map[string]any
	createMR     map[string]any
	createIssue  map[string]any
	updateMR     map[string]any

	// notePages, when true, serves 2 pages of notes and sets X-Next-Page on the first —
	// GitLab's continuation signal (it uses headers, not Link relations).
	notePages bool
	// forceStatus maps an escaped-path suffix to the HTTP status to return instead.
	forceStatus map[string]int
}

// Routes are matched against the ESCAPED path, because the project coordinate travels as a
// single URL-encoded segment ("owner%2Fname"). Matching the decoded path would hide a
// backend that failed to escape it — which on GitLab is not cosmetic: an unescaped slash
// addresses a different endpoint entirely.
var (
	lProject      = regexp.MustCompile(`^/api/v4/projects/[^/]+$`)
	lProjApproval = regexp.MustCompile(`^/api/v4/projects/[^/]+/approvals$`)
	lMR           = regexp.MustCompile(`^/api/v4/projects/[^/]+/merge_requests/[0-9]+$`)
	lMRRoot       = regexp.MustCompile(`^/api/v4/projects/[^/]+/merge_requests$`)
	lMRVersions   = regexp.MustCompile(`/merge_requests/[0-9]+/versions$`)
	lMRApprovals  = regexp.MustCompile(`/merge_requests/[0-9]+/approvals$`)
	lMRApprove    = regexp.MustCompile(`/merge_requests/[0-9]+/approve$`)
	lMRUnapprove  = regexp.MustCompile(`/merge_requests/[0-9]+/unapprove$`)
	lMRNotes      = regexp.MustCompile(`/merge_requests/[0-9]+/notes$`)
	lMRDiffs      = regexp.MustCompile(`/merge_requests/[0-9]+/diffs$`)
	lIssue        = regexp.MustCompile(`^/api/v4/projects/[^/]+/issues/[0-9]+$`)
	lIssueRoot    = regexp.MustCompile(`^/api/v4/projects/[^/]+/issues$`)
	lIssueNotes   = regexp.MustCompile(`/issues/[0-9]+/notes$`)
	lAwards       = regexp.MustCompile(`/issues/[0-9]+/award_emoji$`)
	lUser         = regexp.MustCompile(`^/api/v4/users/[0-9]+$`)
	lCommit       = regexp.MustCompile(`/repository/commits/[^/]+$`)
	lCommitStatus = regexp.MustCompile(`/repository/commits/[^/]+/statuses$`)
	lPipelineJobs = regexp.MustCompile(`/pipelines/[0-9]+/jobs$`)
	lBranch       = regexp.MustCompile(`^/api/v4/projects/[^/]+/repository/branches/[^/]+$`)
)

func (s *glServer) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := readAllCompact(r)
	path := r.URL.EscapedPath()
	s.requests = append(s.requests, reqCapture{
		Method: r.Method, Path: path, Query: r.URL.RawQuery, Body: body,
	})

	for suffix, code := range s.forceStatus {
		if strings.HasSuffix(path, suffix) {
			w.WriteHeader(code)
			return
		}
	}

	enc := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	page := r.URL.Query().Get("page")

	switch {
	case r.Method == http.MethodGet && lMRVersions.MatchString(path):
		enc(s.versions)
	case r.Method == http.MethodGet && lMRApprovals.MatchString(path):
		enc(s.approvals)
	case r.Method == http.MethodPost && lMRApprove.MatchString(path):
		w.WriteHeader(http.StatusCreated)
		enc(s.approvals)
	case r.Method == http.MethodPost && lMRUnapprove.MatchString(path):
		w.WriteHeader(http.StatusCreated)
	case r.Method == http.MethodGet && lMRNotes.MatchString(path):
		if s.notePages {
			if page == "" || page == "1" {
				w.Header().Set("X-Next-Page", "2")
				enc(glNotes(1, 2))
				return
			}
			enc(glNotes(3, 1))
			return
		}
		enc(s.notes)
	case r.Method == http.MethodPost && lMRNotes.MatchString(path):
		w.WriteHeader(http.StatusCreated)
		enc(map[string]any{"id": 900})
	case r.Method == http.MethodGet && lMRDiffs.MatchString(path):
		enc(s.diffs)
	case r.Method == http.MethodGet && lMR.MatchString(path):
		if s.mrMissing {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		enc(s.mr)
	case r.Method == http.MethodPut && lMR.MatchString(path):
		enc(s.updateMR)
	case r.Method == http.MethodPost && lMRRoot.MatchString(path):
		w.WriteHeader(http.StatusCreated)
		enc(s.createMR)
	case r.Method == http.MethodGet && lAwards.MatchString(path):
		enc(s.awards)
	case r.Method == http.MethodGet && lUser.MatchString(path):
		id := path[strings.LastIndex(path, "/")+1:]
		u, ok := s.users[id]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		enc(u)
	case r.Method == http.MethodPost && lIssueNotes.MatchString(path):
		w.WriteHeader(http.StatusCreated)
		enc(map[string]any{"id": 901})
	case (r.Method == http.MethodGet || r.Method == http.MethodPut) && lIssue.MatchString(path):
		if s.issueMissing {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		enc(s.issue)
	case r.Method == http.MethodPost && lIssueRoot.MatchString(path):
		w.WriteHeader(http.StatusCreated)
		enc(s.createIssue)
	case r.Method == http.MethodGet && lCommitStatus.MatchString(path):
		enc(s.statuses)
	case r.Method == http.MethodGet && lCommit.MatchString(path):
		enc(s.commit)
	case r.Method == http.MethodGet && lPipelineJobs.MatchString(path):
		enc(s.jobs)
	case r.Method == http.MethodGet && lProjApproval.MatchString(path):
		enc(s.projApproval)
	case r.Method == http.MethodDelete && lBranch.MatchString(path):
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodGet && lProject.MatchString(path):
		enc(s.project)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// glNotes builds n synthetic non-system notes starting at the given id.
func glNotes(startID, n int) []map[string]any {
	out := make([]map[string]any, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, map[string]any{
			"id":         startID + i,
			"body":       fmt.Sprintf("note %d", startID+i),
			"system":     false,
			"created_at": "2026-08-30T12:00:00Z",
			"author":     map[string]any{"id": 42, "username": "reviewer-bot"},
		})
	}
	return out
}

func newGLServer(t *testing.T) *glServer {
	t.Helper()
	s := &glServer{forceStatus: map[string]int{}, users: map[string]map[string]any{}}
	s.srv = httptest.NewServer(http.HandlerFunc(s.handler))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *glServer) forge() *GitLabForge {
	return &GitLabForge{Token: glTestToken, BaseURL: s.srv.URL, Client: s.srv.Client()}
}

// glTestToken is a low-entropy placeholder — NOT a real credential.
const glTestToken = "test-injected-token-0000"

// glRepo is the project coordinate every case addresses. It contains a slash once rendered,
// which is the whole point: it must reach the wire URL-encoded.
var glRepo = ForgeRepo{Owner: "medici-finance", Name: "assay"}

// glMR is a stock open draft merge request payload.
func glMR(overrides map[string]any) map[string]any {
	base := map[string]any{
		"iid": 7, "state": "opened", "draft": true, "title": "Draft: add the thing",
		"sha": "abc123", "changes_count": "3",
		"author":     map[string]any{"id": 99, "username": "worker-bot"},
		"web_url":    "https://gitlab.example/medici-finance/assay/-/merge_requests/7",
		"updated_at": "2026-09-01T12:00:00Z",
	}
	for k, v := range overrides {
		base[k] = v
	}
	return base
}

// glIssue is a stock issue payload.
//
// The instance-global `id` is present on every fixture on purpose. GitLab always sends it,
// and the client library's Issue.UnmarshalJSON reflects on `raw["id"]` unconditionally — it
// PANICS on a payload without one. A fixture that omitted it would be testing a response
// shape the real API never produces, and would crash rather than fail.
func glIssue(overrides map[string]any) map[string]any {
	base := map[string]any{
		"id": 1000, "iid": 12, "state": "opened",
		"author":  map[string]any{"id": 5, "username": "someone"},
		"web_url": "https://gitlab.example/medici-finance/assay/-/issues/12",
	}
	for k, v := range overrides {
		base[k] = v
	}
	return base
}

// glCase is one golden-pinned operation. `method` names the Forge method it exercises and is
// what TestForgeGitlabCoverage reconciles against the committed inventory.
type glCase struct {
	name   string
	method string
	setup  func(s *glServer)
	run    func(f *GitLabForge) (any, error)
}

func glCases() []glCase {
	return []glCase{
		{
			name: "get_pull_request", method: "GetPullRequest",
			setup: func(s *glServer) { s.mr = glMR(nil) },
			run:   func(f *GitLabForge) (any, error) { return f.GetPullRequest(glRepo, 7) },
		},
		{
			// GitLab reports a TRUNCATABLE changed-file count. "1000+" must not come back as
			// 1000: a walk that also truncated at 1000 would then reconcile clean against a
			// change set that is larger than either number.
			name: "get_pull_request_truncated_changes_count", method: "GetPullRequest",
			setup: func(s *glServer) { s.mr = glMR(map[string]any{"changes_count": "1000+"}) },
			run:   func(f *GitLabForge) (any, error) { return f.GetPullRequest(glRepo, 7) },
		},
		{
			name: "get_issue_plain", method: "GetIssue",
			setup: func(s *glServer) {
				s.issue = glIssue(nil)
				s.mrMissing = true
			},
			run: func(f *GitLabForge) (any, error) { return f.GetIssue(glRepo, 12) },
		},
		{
			name: "get_issue_is_merge_request", method: "GetIssue",
			setup: func(s *glServer) {
				s.issueMissing = true
				s.mr = glMR(map[string]any{"iid": 8})
			},
			run: func(f *GitLabForge) (any, error) { return f.GetIssue(glRepo, 8) },
		},
		{
			// The mapping with no GitHub analog: GitLab numbers issues and MRs separately, so
			// #9 and !9 can both exist. There is no correct single answer, so the backend
			// refuses instead of picking.
			name: "get_issue_ambiguous_both_kinds", method: "GetIssue",
			setup: func(s *glServer) {
				s.issue = glIssue(map[string]any{"iid": 9})
				s.mr = glMR(map[string]any{"iid": 9})
			},
			run: func(f *GitLabForge) (any, error) { return f.GetIssue(glRepo, 9) },
		},
		{
			name: "get_issue_neither_kind", method: "GetIssue",
			setup: func(s *glServer) {
				s.issueMissing = true
				s.mrMissing = true
			},
			run: func(f *GitLabForge) (any, error) { return f.GetIssue(glRepo, 404) },
		},
		{
			// reset_approvals_on_push ON: "an approval exists" and "it was given at this
			// head" are the same statement, so the approval carries the head sha.
			name: "reviews_at_head_approvals_pinned", method: "ReviewsAtHead",
			setup: func(s *glServer) {
				s.mr = glMR(nil)
				s.versions = []map[string]any{
					{"id": 3, "head_commit_sha": "abc123", "created_at": "2026-08-30T10:00:00Z"},
				}
				s.projApproval = map[string]any{"reset_approvals_on_push": true}
				s.approvals = map[string]any{"approved_by": []map[string]any{
					{"user": map[string]any{"id": 42, "username": "reviewer-bot"}},
				}}
				s.notes = []map[string]any{
					{"id": 1, "body": "approved this merge request", "system": true,
						"created_at": "2026-08-30T11:00:00Z",
						"author":     map[string]any{"id": 42, "username": "reviewer-bot"}},
					{"id": 2, "body": "Verdict: APPROVE", "system": false,
						"created_at": "2026-08-30T11:00:01Z",
						"author":     map[string]any{"id": 42, "username": "reviewer-bot"}},
					{"id": 3, "body": "stale comment from before this head", "system": false,
						"created_at": "2026-08-30T09:00:00Z",
						"author":     map[string]any{"id": 7, "username": "human"}},
				}
			},
			run: func(f *GitLabForge) (any, error) { return f.ReviewsAtHead(glRepo, 7) },
		},
		{
			// reset_approvals_on_push OFF: the approval may predate the current head, so it
			// is reported WITHOUT a CommitID rather than stamped with one it cannot claim.
			name: "reviews_at_head_approvals_not_pinned", method: "ReviewsAtHead",
			setup: func(s *glServer) {
				s.mr = glMR(nil)
				s.versions = []map[string]any{
					{"id": 3, "head_commit_sha": "abc123", "created_at": "2026-08-30T10:00:00Z"},
				}
				s.projApproval = map[string]any{"reset_approvals_on_push": false}
				s.approvals = map[string]any{"approved_by": []map[string]any{
					{"user": map[string]any{"id": 42, "username": "reviewer-bot"}},
				}}
				s.notes = []map[string]any{}
			},
			run: func(f *GitLabForge) (any, error) { return f.ReviewsAtHead(glRepo, 7) },
		},
		{
			// Pagination is signalled by the X-Next-Page HEADER on GitLab, not by Link
			// relations and not by a short page. This case serves a FULL-looking first page
			// with the header set and a second page without it: a walk that stopped on a
			// short page would read only two of the three notes.
			name: "reviews_walk_two_note_pages", method: "ReviewsAtHead",
			setup: func(s *glServer) {
				s.mr = glMR(nil)
				s.versions = []map[string]any{
					{"id": 3, "head_commit_sha": "abc123", "created_at": "2026-08-30T10:00:00Z"},
				}
				s.projApproval = map[string]any{"reset_approvals_on_push": true}
				s.approvals = map[string]any{"approved_by": []map[string]any{}}
				s.notePages = true
			},
			run: func(f *GitLabForge) (any, error) {
				rs, err := f.ReviewsAtHead(glRepo, 7)
				return len(rs), err
			},
		},
		{
			name: "list_changed_files", method: "ListChangedFiles",
			setup: func(s *glServer) {
				s.diffs = []map[string]any{
					{"old_path": "secrets/auth/token.go", "new_path": "config/authz/token.go", "renamed_file": true},
					{"old_path": "docs/desk-tools.md", "new_path": "docs/desk-tools.md"},
					{"old_path": "cmd/new.go", "new_path": "cmd/new.go", "new_file": true},
					{"old_path": "cmd/gone.go", "new_path": "cmd/gone.go", "deleted_file": true},
				}
			},
			run: func(f *GitLabForge) (any, error) { return f.ListChangedFiles(glRepo, 7) },
		},
		{
			name: "checks_at_head", method: "ChecksAtHead",
			setup: func(s *glServer) {
				s.commit = map[string]any{"id": "abc123", "status": "failed",
					"last_pipeline": map[string]any{"id": 55, "status": "failed"}}
				s.statuses = []map[string]any{
					{"name": "leak-sweep", "status": "success"},
					{"name": "external/policy", "status": "canceled"},
				}
				s.jobs = []map[string]any{
					{"name": "go-test", "status": "success"},
					{"name": "lint", "status": "failed"},
					{"name": "deploy", "status": "manual"},
				}
			},
			run: func(f *GitLabForge) (any, error) { return f.ChecksAtHead(glRepo, "abc123") },
		},
		{
			// No pipeline at the head: empty check-runs and a zero count is a truthful "no
			// jobs", which the caller must be able to tell apart from a could-not-check.
			name: "checks_at_head_no_pipeline", method: "ChecksAtHead",
			setup: func(s *glServer) {
				s.commit = map[string]any{"id": "abc123", "status": "success"}
				s.statuses = []map[string]any{{"name": "leak-sweep", "status": "success"}}
			},
			run: func(f *GitLabForge) (any, error) { return f.ChecksAtHead(glRepo, "abc123") },
		},
		{
			// Both halves of the reaction mapping at once: GitLab's `thumbsup` must arrive as
			// the `+1` the admission gate matches on, and the BOT's award must NOT be typed
			// "User" — the gate admits only a human, and GitLab's award payload carries no
			// bot flag of its own.
			name: "issue_reactions_human_and_bot", method: "IssueReactions",
			setup: func(s *glServer) {
				s.awards = []map[string]any{
					{"name": "thumbsup", "user": map[string]any{"id": 1, "username": "ian"}},
					{"name": "thumbsup", "user": map[string]any{"id": 2, "username": "desk-bot"}},
					{"name": "tada", "user": map[string]any{"id": 1, "username": "ian"}},
				}
				s.users["1"] = map[string]any{"id": 1, "username": "ian", "bot": false}
				s.users["2"] = map[string]any{"id": 2, "username": "desk-bot", "bot": true}
			},
			run: func(f *GitLabForge) (any, error) { return f.IssueReactions(glRepo, 7) },
		},
		{
			// An unresolvable actor is typed EMPTY, never "User": a gate that requires a
			// human must refuse rather than admit an actor whose kind is unknown.
			name: "issue_reactions_unresolvable_actor", method: "IssueReactions",
			setup: func(s *glServer) {
				s.awards = []map[string]any{
					{"name": "thumbsup", "user": map[string]any{"id": 3, "username": "ghost"}},
				}
			},
			run: func(f *GitLabForge) (any, error) { return f.IssueReactions(glRepo, 7) },
		},
		{
			name: "repo_visibility", method: "RepoVisibility",
			setup: func(s *glServer) { s.project = map[string]any{"visibility": "public"} },
			run:   func(f *GitLabForge) (any, error) { return f.RepoVisibility(glRepo) },
		},
		{
			name: "create_draft_change", method: "CreateDraftChange",
			setup: func(s *glServer) {
				s.createMR = glMR(map[string]any{"iid": 21, "title": "Draft: t"})
			},
			run: func(f *GitLabForge) (any, error) {
				return f.CreateDraftChange(glRepo, DraftChangeInput{Title: "t", Body: "b", Head: "feat/x", Base: "main"})
			},
		},
		{
			// A caller that spelled the prefix itself must not end up with it twice.
			name: "create_draft_change_title_already_prefixed", method: "CreateDraftChange",
			setup: func(s *glServer) {
				s.createMR = glMR(map[string]any{"iid": 22, "title": "Draft: t"})
			},
			run: func(f *GitLabForge) (any, error) {
				return f.CreateDraftChange(glRepo, DraftChangeInput{Title: "Draft: t", Body: "b", Head: "feat/x", Base: "main"})
			},
		},
		{
			// The prefix is the ONLY draft mechanism GitLab has, so if the created MR does
			// not come back marked draft the change is open for review when it was meant to
			// be a draft. Handing back a PullRef would misrepresent it.
			name: "create_draft_change_not_marked_draft", method: "CreateDraftChange",
			setup: func(s *glServer) {
				s.createMR = glMR(map[string]any{"iid": 23, "title": "t", "draft": false})
			},
			run: func(f *GitLabForge) (any, error) {
				return f.CreateDraftChange(glRepo, DraftChangeInput{Title: "t", Body: "b", Head: "feat/x", Base: "main"})
			},
		},
		{
			name: "post_comment_on_issue", method: "PostComment",
			setup: func(s *glServer) {
				s.issue = glIssue(nil)
				s.mrMissing = true
			},
			run: func(f *GitLabForge) (any, error) { return nil, f.PostComment(glRepo, 12, "hello") },
		},
		{
			name: "post_comment_on_merge_request", method: "PostComment",
			setup: func(s *glServer) {
				s.issueMissing = true
				s.mr = glMR(nil)
			},
			run: func(f *GitLabForge) (any, error) { return nil, f.PostComment(glRepo, 7, "hello") },
		},
		{
			// The note is posted BEFORE the approval. The request sequence in the golden is
			// the assertion: a failure between the two must leave reasoning-without-a-grant,
			// never a grant-without-reasoning.
			name: "post_review_approve", method: "PostReview",
			setup: func(s *glServer) { s.approvals = map[string]any{"approved": true} },
			run: func(f *GitLabForge) (any, error) {
				return nil, f.PostReview(glRepo, 7, ReviewInput{HeadSHA: "abc123", Event: "APPROVE", Body: "ok"})
			},
		},
		{
			name: "post_review_request_changes", method: "PostReview",
			setup: func(s *glServer) {},
			run: func(f *GitLabForge) (any, error) {
				return nil, f.PostReview(glRepo, 7, ReviewInput{HeadSHA: "abc123", Event: "REQUEST_CHANGES", Body: "no"})
			},
		},
		{
			name: "post_review_comment", method: "PostReview",
			setup: func(s *glServer) {},
			run: func(f *GitLabForge) (any, error) {
				return nil, f.PostReview(glRepo, 7, ReviewInput{HeadSHA: "abc123", Event: "COMMENT", Body: "fyi"})
			},
		},
		{
			// An unknown verb must be refused, not degraded to a comment: a failed approval
			// that returns nil is indistinguishable from a granted one.
			name: "post_review_unknown_event", method: "PostReview",
			setup: func(s *glServer) {},
			run: func(f *GitLabForge) (any, error) {
				return nil, f.PostReview(glRepo, 7, ReviewInput{HeadSHA: "abc123", Event: "DISMISS", Body: "x"})
			},
		},
		{
			name: "mark_ready_for_review", method: "MarkReadyForReview",
			setup: func(s *glServer) {
				s.mr = glMR(nil)
				s.updateMR = glMR(map[string]any{"draft": false, "title": "add the thing"})
			},
			run: func(f *GitLabForge) (any, error) {
				return nil, f.MarkReadyForReview(gitlabNodeID(glRepo, 7))
			},
		},
		{
			// Already ready: no write is emitted at all. The empty request tail after the
			// read is the assertion.
			name: "mark_ready_for_review_already_ready", method: "MarkReadyForReview",
			setup: func(s *glServer) {
				s.mr = glMR(map[string]any{"draft": false, "title": "add the thing"})
			},
			run: func(f *GitLabForge) (any, error) {
				return nil, f.MarkReadyForReview(gitlabNodeID(glRepo, 7))
			},
		},
		{
			// A GitHub node id handed to the GitLab backend is a wiring bug; it must not be
			// half-parsed into a request at some other project.
			name: "mark_ready_for_review_foreign_id", method: "MarkReadyForReview",
			setup: func(s *glServer) {},
			run:   func(f *GitLabForge) (any, error) { return nil, f.MarkReadyForReview("PR_kwDOabc123") },
		},
		{
			name: "file_issue", method: "FileIssue",
			setup: func(s *glServer) {
				s.createIssue = glIssue(map[string]any{"iid": 33,
					"web_url": "https://gitlab.example/medici-finance/assay/-/issues/33"})
			},
			run: func(f *GitLabForge) (any, error) {
				return f.FileIssue(glRepo, IssueInput{Title: "bug", Body: "detail"})
			},
		},
		{
			// GitLab has no state_reason. The reason is recorded as a note before the close
			// so it survives, rather than being dropped where nobody can see it went missing.
			name: "close_issue", method: "CloseIssue",
			setup: func(s *glServer) { s.issue = glIssue(map[string]any{"iid": 33, "state": "closed"}) },
			run:   func(f *GitLabForge) (any, error) { return nil, f.CloseIssue(glRepo, 33, "not_planned") },
		},
		{
			name: "close_issue_no_reason", method: "CloseIssue",
			setup: func(s *glServer) { s.issue = glIssue(map[string]any{"iid": 33, "state": "closed"}) },
			run:   func(f *GitLabForge) (any, error) { return nil, f.CloseIssue(glRepo, 33, "") },
		},
		{
			// The half GitLab CE CAN serve: a ref in refs/heads is a branch, and the Branches
			// API deletes branches at Free tier. The golden pins that the project coordinate
			// travels URL-encoded and the branch name occupies its own segment.
			name: "delete_ref_branch", method: "DeleteRef",
			setup: func(s *glServer) {},
			run:   func(f *GitLabForge) (any, error) { return nil, f.DeleteRef(glRepo, "heads/feat/x") },
		},
		{
			// The half it cannot: GitLab CE exposes no general ref-delete endpoint, so a claim
			// ref outside refs/heads is a could-not-check REFUSAL with zero requests emitted —
			// not a silent success, and not a guessed endpoint. A backend that reported this as
			// released would tell the dispatcher a held claim is free.
			name: "delete_ref_non_branch_namespace_refused", method: "DeleteRef",
			setup: func(s *glServer) {},
			run:   func(f *GitLabForge) (any, error) { return nil, f.DeleteRef(glRepo, "dispatch/item--01") },
		},
		{
			name: "push_transport_hint", method: "PushTransportHint",
			setup: func(s *glServer) {},
			run:   func(f *GitLabForge) (any, error) { return f.PushTransportHint(glRepo), nil },
		},
		{
			name: "error_not_found", method: "GetPullRequest",
			setup: func(s *glServer) { s.forceStatus["/merge_requests/7"] = http.StatusNotFound },
			run:   func(f *GitLabForge) (any, error) { return f.GetPullRequest(glRepo, 7) },
		},
		{
			name: "error_forbidden_tier", method: "GetPullRequest",
			setup: func(s *glServer) { s.forceStatus["/merge_requests/7"] = http.StatusForbidden },
			run:   func(f *GitLabForge) (any, error) { return f.GetPullRequest(glRepo, 7) },
		},
		{
			// The approval-configuration read is a Premium+ surface. A 403 there is a
			// could-not-check for the WHOLE review read — never a licence to fall back to
			// "assume approvals are head-pinned" or to report no reviews.
			name: "error_forbidden_approval_config_tier", method: "ReviewsAtHead",
			setup: func(s *glServer) {
				s.mr = glMR(nil)
				s.versions = []map[string]any{
					{"id": 3, "head_commit_sha": "abc123", "created_at": "2026-08-30T10:00:00Z"},
				}
				s.forceStatus["/approvals"] = http.StatusForbidden
			},
			run: func(f *GitLabForge) (any, error) { return f.ReviewsAtHead(glRepo, 7) },
		},
	}
}

func TestForgeGitlabGolden(t *testing.T) {
	for _, tc := range glCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := newGLServer(t)
			tc.setup(s)
			res, err := tc.run(s.forge())

			c := capture{Requests: normalizeRequests(s.requests)}
			if err != nil {
				c.Err = err.Error()
				nf := IsForgeNotFound(err)
				c.NotFound = &nf
			}
			if res != nil {
				b, mErr := json.MarshalIndent(res, "", "  ")
				if mErr != nil {
					t.Fatalf("marshal result: %v", mErr)
				}
				c.Result = json.RawMessage(b)
			}

			got, mErr := json.MarshalIndent(c, "", "  ")
			if mErr != nil {
				t.Fatalf("marshal capture: %v", mErr)
			}
			got = append(got, '\n')

			goldenPath := filepath.Join("testdata", "forge_gitlab_golden", tc.name+".golden.json")
			if *updateGolden {
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				t.Logf("golden-pinned operation %q updated", tc.name)
				return
			}
			want, rErr := os.ReadFile(goldenPath)
			if rErr != nil {
				t.Fatalf("read golden %s: %v (run with -update to create)", goldenPath, rErr)
			}
			if !bytes.Equal(bytes.TrimRight(got, "\n"), bytes.TrimRight(want, "\n")) {
				t.Errorf("golden mismatch for %q\n--- got ---\n%s\n--- want ---\n%s", tc.name, got, want)
			} else {
				t.Logf("golden-pinned operation %q OK", tc.name)
			}
		})
	}
}

// inventoryMethodRow matches a method row of a stream inventory's frozen method-set table:
// "| 7 | `RepoVisibility(repo)` | … |". The method NAME is the identifier before the paren.
var inventoryMethodRow = regexp.MustCompile("^\\|\\s*[0-9]+\\s*\\|\\s*`([A-Za-z]+)\\(")

// forgeInterfaceMethods returns the method set of the frozen Forge interface, sorted. This
// is the PRIMARY coverage oracle: it is derived from the seam itself, so it cannot drift
// from what a backend must implement, and it is available in every checkout.
func forgeInterfaceMethods() []string {
	rt := reflect.TypeOf((*Forge)(nil)).Elem()
	out := make([]string, 0, rt.NumMethod())
	for i := 0; i < rt.NumMethod(); i++ {
		out = append(out, rt.Method(i).Name)
	}
	sort.Strings(out)
	return out
}

// committedInventoryMethods reads the stream inventories COMMITTED to this repository and
// returns the union of the forge operations their method-set tables name, plus the files
// they were read from.
//
// It DISCOVERS the inventory by walking the stream registers for `inventory.md` files whose
// table names real Forge methods, rather than by carrying a stream path of its own. That is
// not indirection for its own sake: a shipping file under tools/desk may not name a
// `docs/streams/<stream>` path (TestCorpusHasNoWithheldStreamPaths — the whole register tree
// is do-not-copy, so a baked-in path publishes a map to withheld material), and the register
// tree is absent entirely from a published adopter checkout. Discovery satisfies both
// without exempting this file from that guard or widening its allowlist.
//
// Rows that do not name a real Forge method are ignored, so an unrelated stream that happens
// to keep a similarly-shaped table cannot inject a phantom operation.
func committedInventoryMethods(root string) (methods map[string]bool, sources []string, err error) {
	real := map[string]bool{}
	for _, m := range forgeInterfaceMethods() {
		real[m] = true
	}
	methods = map[string]bool{}
	base := filepath.Join(root, "docs", "streams")
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(base, e.Name(), "inventory.md")
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			continue
		}
		found := false
		for _, line := range strings.Split(string(raw), "\n") {
			m := inventoryMethodRow.FindStringSubmatch(line)
			if m == nil || !real[m[1]] {
				continue
			}
			methods[m[1]] = true
			found = true
		}
		if found {
			sources = append(sources, p)
		}
	}
	sort.Strings(sources)
	return methods, sources, nil
}

// TestForgeGitlabCoverage fails naming any inventoried forge operation that has no gitlab
// contract test.
//
// Coverage is DEREFERENCED, never asserted. A list of operations restated inside this test
// would report full coverage of whatever it happened to list — and would keep reporting it
// after the seam grew a method, which is a check that answers the wrong question in green.
// So the set comes from two independent places, and both must be satisfied:
//
//  1. the frozen Forge interface itself, read by reflection — always present, and the
//     definition of what a backend must implement; and
//  2. the COMMITTED stream inventory, when this checkout carries the stream registers —
//     the table the stream publishes, which is what the brief's Verify row dereferences.
//
// The second is skipped, loudly, in a checkout that does not carry the registers (a
// published adopter subset). It is a supplement, not the floor: dropping it cannot make the
// check pass on an uncovered operation, because the interface oracle still runs.
func TestForgeGitlabCoverage(t *testing.T) {
	const repoRoot = "../../../.."

	covered := map[string][]string{}
	for _, c := range glCases() {
		if c.method == "" {
			t.Fatalf("golden case %q declares no Forge method, so it cannot count towards coverage", c.name)
		}
		covered[c.method] = append(covered[c.method], c.name)
	}

	iface := forgeInterfaceMethods()
	if len(iface) == 0 {
		t.Fatal("reflected 0 methods off the Forge interface — the oracle is broken, which is " +
			"could-not-check, not full coverage")
	}
	real := map[string]bool{}
	for _, m := range iface {
		real[m] = true
	}

	var missing []string
	for _, m := range iface {
		if len(covered[m]) == 0 {
			missing = append(missing, m)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("%d forge operation(s) have no gitlab contract test: %s\n"+
			"(every method of the frozen Forge interface needs at least one case in glCases())",
			len(missing), strings.Join(missing, ", "))
	}

	// A case naming a method the interface does not carry is a typo — which means the method
	// it MEANT to cover is silently uncovered.
	var stray []string
	for m := range covered {
		if !real[m] {
			stray = append(stray, m)
		}
	}
	if len(stray) > 0 {
		sort.Strings(stray)
		t.Fatalf("gitlab contract case(s) name method(s) that are not on the Forge interface: %s",
			strings.Join(stray, ", "))
	}
	t.Logf("gitlab contract corpus covers all %d operations of the frozen Forge interface: %s",
		len(iface), strings.Join(iface, ", "))

	// Layer 2 — the committed inventory, when this checkout has the stream registers.
	inv, sources, err := committedInventoryMethods(repoRoot)
	if err != nil {
		if os.IsNotExist(err) {
			t.Logf("could-not-check (supplementary layer only): this checkout carries no stream " +
				"registers, so the committed inventory could not be reconciled; the interface oracle above " +
				"still measured full coverage")
			return
		}
		t.Fatalf("the stream registers are present but unreadable: %v — that is could-not-check, not a pass", err)
	}
	if len(inv) == 0 {
		t.Logf("could-not-check (supplementary layer only): no committed inventory table naming Forge " +
			"operations was found in the stream registers")
		return
	}
	var invMissing []string
	for m := range inv {
		if len(covered[m]) == 0 {
			invMissing = append(invMissing, m)
		}
	}
	if len(invMissing) > 0 {
		sort.Strings(invMissing)
		t.Fatalf("%d operation(s) named by the committed inventory (%s) have no gitlab contract test: %s",
			len(invMissing), strings.Join(sources, ", "), strings.Join(invMissing, ", "))
	}
	// The inventory must also be COMPLETE: an interface method it never tabulates is a stale
	// inventory, and a stale inventory is exactly what makes a dereference meaningless.
	var untabulated []string
	for _, m := range iface {
		if !inv[m] {
			untabulated = append(untabulated, m)
		}
	}
	if len(untabulated) > 0 {
		sort.Strings(untabulated)
		t.Fatalf("the committed inventory (%s) tabulates no row for %d Forge operation(s): %s — "+
			"a coverage dereference against an inventory that is missing rows measures nothing",
			strings.Join(sources, ", "), len(untabulated), strings.Join(untabulated, ", "))
	}
	t.Logf("committed inventory (%s) reconciles: %d operations, all covered",
		strings.Join(sources, ", "), len(inv))
}

// TestForgeGitlabTierErrors proves the three error tiers the brief names come back as
// could-not-check — a non-nil error carrying the literal token, a nil result, and exit code
// 6 — never as a clean empty result.
//
// The tier that motivates the test is 403: approval rules and external status checks are
// Premium/Ultimate features (spec §3), so a Free/Premium instance answers 403 for surfaces a
// desk tool expects to exist. A backend that mapped that to "no approvals" would report an
// unapproved MR as read-and-clean, and a flip gate reading it would refuse for the wrong
// reason today and pass for the wrong reason after any change to how it counts approvals.
func TestForgeGitlabTierErrors(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		wantNotFnd bool
	}{
		{"forbidden_403_permission_or_tier", http.StatusForbidden, false},
		{"unauthorized_401_credential", http.StatusUnauthorized, false},
		{"not_found_404_visibility", http.StatusNotFound, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := newGLServer(t)
			s.forceStatus["/merge_requests/7"] = tc.status
			f := s.forge()

			pr, err := f.GetPullRequest(glRepo, 7)
			if err == nil {
				t.Fatalf("HTTP %d must surface an error (could-not-check), not a clean result", tc.status)
			}
			if pr != nil {
				t.Fatalf("HTTP %d must yield a nil result distinct from an empty PullRequest, got %+v", tc.status, pr)
			}
			if !strings.Contains(err.Error(), "could-not-check") {
				t.Fatalf("HTTP %d refusal must say could-not-check, got %q", tc.status, err.Error())
			}
			var fae *ForgeAPIError
			if !errors.As(err, &fae) {
				t.Fatalf("HTTP %d should carry a *ForgeAPIError, got %T (%v)", tc.status, err, err)
			}
			if fae.Status != tc.status {
				t.Fatalf("ForgeAPIError.Status = %d, want %d", fae.Status, tc.status)
			}
			if code := ExitCodeOf(err); code != ExitUnverifiable {
				t.Fatalf("HTTP %d should map to ExitUnverifiable (%d), got %d", tc.status, ExitUnverifiable, code)
			}
			if got := IsForgeNotFound(err); got != tc.wantNotFnd {
				t.Fatalf("IsForgeNotFound = %v, want %v (only 404 licenses a not-found re-resolution)", got, tc.wantNotFnd)
			}
			t.Logf("HTTP %d → could-not-check: %v", tc.status, err)
		})
	}

	// A tier failure on a LIST operation is the one most easily mistaken for an empty
	// result, because the happy-path shape of "no approvals yet" is also empty.
	t.Run("tier_failure_is_not_an_empty_list", func(t *testing.T) {
		s := newGLServer(t)
		s.mr = glMR(nil)
		s.versions = []map[string]any{{"id": 3, "head_commit_sha": "abc123", "created_at": "2026-08-30T10:00:00Z"}}
		s.forceStatus["/approvals"] = http.StatusForbidden

		rs, err := s.forge().ReviewsAtHead(glRepo, 7)
		if err == nil {
			t.Fatalf("a 403 on the Premium-gated approval surface must be could-not-check, not %d reviews", len(rs))
		}
		if rs != nil {
			t.Fatalf("a could-not-check must yield a nil list, got %d entries", len(rs))
		}
		if !strings.Contains(err.Error(), "could-not-check") {
			t.Fatalf("tier refusal must say could-not-check, got %q", err.Error())
		}
		t.Logf("Premium-gated approval read 403 → could-not-check: %v", err)
	})
}

// TestForgeGitlabAuth proves the injected token is the authenticating identity — over
// GitLab's PRIVATE-TOKEN header, not a bearer — and that an unset token is REFUSED rather
// than sent as an empty header, which would silently downgrade the caller to whatever the
// instance exposes anonymously.
func TestForgeGitlabAuth(t *testing.T) {
	t.Run("authenticates_from_injected_token", func(t *testing.T) {
		var gotPrivate, gotAuthz string
		hits := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits++
			gotPrivate = r.Header.Get("PRIVATE-TOKEN")
			gotAuthz = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"visibility":"public"}`))
		}))
		t.Cleanup(srv.Close)

		f := &GitLabForge{Token: glTestToken, BaseURL: srv.URL, Client: srv.Client()}
		if _, err := f.RepoVisibility(glRepo); err != nil {
			t.Fatalf("RepoVisibility with a valid token: unexpected error %v", err)
		}
		if hits != 1 {
			t.Fatalf("expected exactly one request, got %d", hits)
		}
		if gotPrivate != glTestToken {
			t.Fatalf("backend did not authenticate from the injected token\n got PRIVATE-TOKEN: %q\nwant: %q",
				gotPrivate, glTestToken)
		}
		if gotAuthz != "" {
			t.Fatalf("a PAT must travel in PRIVATE-TOKEN, not Authorization; got Authorization: %q", gotAuthz)
		}
	})

	t.Run("refuses_unset_token_no_anonymous_fallback", func(t *testing.T) {
		hits := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			hits++
			_, _ = w.Write([]byte(`{"visibility":"public"}`))
		}))
		t.Cleanup(srv.Close)

		f := &GitLabForge{Token: "", BaseURL: srv.URL, Client: srv.Client()}
		_, err := f.RepoVisibility(glRepo)
		if err == nil {
			t.Fatalf("an unset token must refuse, not fall back to an anonymous identity")
		}
		if code := ExitCodeOf(err); code != ExitUnverifiable {
			t.Fatalf("unset-token refusal should be ExitUnverifiable (%d), got %d (%v)", ExitUnverifiable, code, err)
		}
		if hits != 0 {
			t.Fatalf("an unset token must never reach the network, but the server saw %d request(s)", hits)
		}
	})
}

// TestForgeGitlabPushTransportHint pins the two properties of the transport hint that are
// security-relevant rather than cosmetic: it names the instance's OWN host (a self-managed EE
// deployment does not push to gitlab.com), and it never suggests a token-in-URL.
func TestForgeGitlabPushTransportHint(t *testing.T) {
	f := &GitLabForge{Token: glTestToken, BaseURL: "https://gitlab.internal.example"}
	h := f.PushTransportHint(glRepo)
	if h.RemoteHost != "gitlab.internal.example" {
		t.Fatalf("RemoteHost = %q, want the configured instance host", h.RemoteHost)
	}
	if h.TokenUsername != "oauth2" {
		t.Fatalf("TokenUsername = %q, want oauth2 (GitLab's token-over-https username)", h.TokenUsername)
	}
	if !strings.Contains(h.CredentialHelperHint, "credential.helper") ||
		!strings.Contains(h.CredentialHelperHint, "never embed it in the remote URL") {
		t.Fatalf("the hint must direct callers to a credential.helper and away from a token-in-URL, got %q",
			h.CredentialHelperHint)
	}

	def := (&GitLabForge{Token: glTestToken}).PushTransportHint(glRepo)
	if def.RemoteHost != "gitlab.com" {
		t.Fatalf("default RemoteHost = %q, want gitlab.com", def.RemoteHost)
	}
}

// TestForgeGitlabChangedFileCount pins the truncation rule directly, because the golden that
// exercises it can only show the mapped OUTPUT: this states the property.
//
// GitLab's changes_count is a string that may be a truncated lower bound. The count exists so
// a caller can reconcile it against the file walk and refuse a short read; a truncated count
// reported as-is would let a walk that truncated at the same number reconcile clean.
func TestForgeGitlabChangedFileCount(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"3", 3, true},
		{"0", 0, true},
		{"1000+", 1001, true},
		{" 42 ", 42, true},
		{"", 0, false},
		{"many", 0, false},
		{"+", 0, false},
	}
	for _, tc := range cases {
		got, ok := gitlabChangedFileCount(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("gitlabChangedFileCount(%q) = (%d, %v), want (%d, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
	// The property, stated: a truncated count can never be satisfied by a walk that stopped
	// at the same truncation point.
	n, _ := gitlabChangedFileCount("1000+")
	if n <= 1000 {
		t.Fatalf("a truncated changes_count of %q reported as %d would let a 1000-entry walk reconcile clean",
			"1000+", n)
	}
}

// TestForgeGitlabNodeID pins the opaque-id round trip and, more importantly, the refusal:
// MarkReadyForReview receives only this id, so an id from another forge must stop the
// operation rather than be half-parsed into a request at some other project.
func TestForgeGitlabNodeID(t *testing.T) {
	id := gitlabNodeID(glRepo, 7)
	repo, iid, err := parseGitLabNodeID(id)
	if err != nil {
		t.Fatalf("round trip of %q failed: %v", id, err)
	}
	if repo != glRepo || iid != 7 {
		t.Fatalf("round trip gave %+v !%d, want %+v !7", repo, iid, glRepo)
	}
	// The unprefixed form is the one that matters most: it is otherwise well-shaped, so only
	// the scheme check stands between it and a request issued at a project this backend was
	// never told to act on.
	for _, bad := range []string{"", "PR_kwDOabc", "medici-finance/assay!7",
		"gitlab:medici-finance/assay", "gitlab:assay!7",
		"gitlab:medici-finance/assay!0", "gitlab:medici-finance/assay!x", "gitlab:/assay!7"} {
		if _, _, err := parseGitLabNodeID(bad); err == nil {
			t.Errorf("parseGitLabNodeID(%q) accepted an id it did not mint", bad)
		}
	}
}
