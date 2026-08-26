package deskkit

import (
	"bytes"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// forge_github_golden_test.go — TestForgeGithubGolden pins the GitHub Forge implementation's
// observable behavior per operation: the request(s) it emits (method, path, query, and body
// for writes), the mapped result, and the error classification. This IS the single-point-of-
// failure control the brief names — the corpus proving the extraction changed nothing at the
// wire. Regenerate with `go test ./tools/desk/internal/deskkit/ -run TestForgeGithubGolden
// -update` after an INTENTIONAL behavior change; an unintentional one shows as a diff.

var updateGolden = flag.Bool("update", false, "regenerate the forge golden corpus")

// reqCapture is one HTTP request the implementation emitted, in a wire-stable form.
type reqCapture struct {
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Query  string          `json:"query,omitempty"`
	Body   json.RawMessage `json:"body,omitempty"`
}

// capture is the full observable footprint of one operation.
type capture struct {
	Requests []reqCapture    `json:"requests"`
	Result   json.RawMessage `json:"result,omitempty"`
	Err      string          `json:"err,omitempty"`
	NotFound *bool           `json:"not_found,omitempty"`
}

// goldenServer records every request and serves canned JSON keyed by a small router. The
// per-page datasets are deliberately under 100 entries so a single page is fetched — which
// pins that the client sends per_page=100 (not GitHub's truncating default of 30) and stops
// on the short page. The dedicated "walk_two_pages" case proves the multi-page continuation.
type goldenServer struct {
	srv      *httptest.Server
	requests []reqCapture

	reviews    []map[string]any
	files      []map[string]any
	status     map[string]any
	checks     map[string]any
	reactions  []map[string]any
	pull       map[string]any
	issue      map[string]any
	repo       map[string]any
	createResp map[string]any
	graphql    map[string]any
	// forceStatus, when set for a path suffix, returns that HTTP status (error-mapping cases).
	forceStatus map[string]int
	// bigReviewPages: when true, /reviews returns 100 entries on page 1, 1 on page 2.
	bigReviewPages bool
}

var (
	gPull      = regexp.MustCompile(`^/repos/[^/]+/[^/]+/pulls/[0-9]+$`)
	gPullsRoot = regexp.MustCompile(`^/repos/[^/]+/[^/]+/pulls$`)
	gReviews   = regexp.MustCompile(`/pulls/[0-9]+/reviews$`)
	gFiles     = regexp.MustCompile(`/pulls/[0-9]+/files$`)
	gComments  = regexp.MustCompile(`/issues/[0-9]+/comments$`)
	gIssueNum  = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues/[0-9]+$`)
	gIssueRoot = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues$`)
	gStatus    = regexp.MustCompile(`/commits/[^/]+/status$`)
	gChecks    = regexp.MustCompile(`/commits/[^/]+/check-runs$`)
	gRepo      = regexp.MustCompile(`^/repos/[^/]+/[^/]+$`)
	gReactions = regexp.MustCompile(`/issues/[0-9]+/reactions$`)
)

func (s *goldenServer) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := readAllCompact(r)
	s.requests = append(s.requests, reqCapture{
		Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: body,
	})
	path := r.URL.Path

	for suffix, code := range s.forceStatus {
		if strings.HasSuffix(path, suffix) {
			w.WriteHeader(code)
			return
		}
	}

	enc := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	page := r.URL.Query().Get("page")

	switch {
	case r.Method == http.MethodPost && path == "/graphql":
		enc(s.graphql)
	case r.Method == http.MethodGet && gReactions.MatchString(path):
		enc(s.reactions)
	case r.Method == http.MethodGet && gReviews.MatchString(path):
		if s.bigReviewPages {
			if page == "" || page == "1" {
				enc(makeReviews(100))
			} else if page == "2" {
				enc(makeReviews(1))
			} else {
				enc([]map[string]any{})
			}
			return
		}
		if page != "" && page != "1" {
			enc([]map[string]any{})
			return
		}
		enc(s.reviews)
	case r.Method == http.MethodPost && gReviews.MatchString(path):
		w.WriteHeader(http.StatusOK)
		enc(map[string]any{"id": 1})
	case r.Method == http.MethodGet && gFiles.MatchString(path):
		if page != "" && page != "1" {
			enc([]map[string]any{})
			return
		}
		enc(s.files)
	case r.Method == http.MethodGet && gStatus.MatchString(path):
		enc(s.status)
	case r.Method == http.MethodGet && gChecks.MatchString(path):
		enc(s.checks)
	case r.Method == http.MethodGet && gPull.MatchString(path):
		enc(s.pull)
	case r.Method == http.MethodPost && gPullsRoot.MatchString(path):
		w.WriteHeader(http.StatusCreated)
		enc(s.createResp)
	case r.Method == http.MethodPost && gComments.MatchString(path):
		w.WriteHeader(http.StatusCreated)
		enc(map[string]any{"id": 1})
	case r.Method == http.MethodPost && gIssueRoot.MatchString(path):
		w.WriteHeader(http.StatusCreated)
		enc(s.createResp)
	case (r.Method == http.MethodGet || r.Method == http.MethodPatch) && gIssueNum.MatchString(path):
		enc(s.issue)
	case r.Method == http.MethodGet && gRepo.MatchString(path):
		enc(s.repo)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func readAllCompact(r *http.Request) (json.RawMessage, error) {
	if r.Body == nil {
		return nil, nil
	}
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r.Body)
	if buf.Len() == 0 {
		return nil, nil
	}
	var out bytes.Buffer
	if err := json.Compact(&out, buf.Bytes()); err != nil {
		return json.RawMessage(buf.Bytes()), nil
	}
	return json.RawMessage(out.Bytes()), nil
}

func makeReviews(n int) []map[string]any {
	out := make([]map[string]any, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, map[string]any{
			"id":           i + 1,
			"state":        "APPROVED",
			"commit_id":    "deadbeef",
			"user":         map[string]any{"login": "reviewer[bot]", "id": 42},
			"submitted_at": "2026-08-24T00:00:00Z",
		})
	}
	return out
}

func newGoldenServer(t *testing.T) *goldenServer {
	t.Helper()
	s := &goldenServer{forceStatus: map[string]int{}}
	s.srv = httptest.NewServer(http.HandlerFunc(s.handler))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *goldenServer) forge() *GitHubForge {
	return &GitHubForge{Token: "test-token", BaseURL: s.srv.URL, Client: s.srv.Client()}
}

var forgeTestRepo = ForgeRepo{Owner: "medici-finance", Name: "assay"}

func TestForgeGithubGolden(t *testing.T) {
	cases := []struct {
		name  string
		setup func(s *goldenServer)
		run   func(f *GitHubForge) (any, error)
	}{
		{
			name: "get_pull_request",
			setup: func(s *goldenServer) {
				s.pull = map[string]any{
					"number": 7, "state": "open", "draft": true, "node_id": "PR_node",
					"changed_files": 3,
					"user":          map[string]any{"login": "worker[bot]", "id": 99},
					"head":          map[string]any{"sha": "abc123"},
				}
			},
			run: func(f *GitHubForge) (any, error) { return f.GetPullRequest(forgeTestRepo, 7) },
		},
		{
			name: "get_issue_plain",
			setup: func(s *goldenServer) {
				s.issue = map[string]any{"number": 12, "state": "open",
					"user": map[string]any{"login": "someone", "id": 5}}
			},
			run: func(f *GitHubForge) (any, error) { return f.GetIssue(forgeTestRepo, 12) },
		},
		{
			name: "get_issue_is_pull_request",
			setup: func(s *goldenServer) {
				s.issue = map[string]any{"number": 8, "state": "open",
					"user":         map[string]any{"login": "worker[bot]", "id": 99},
					"pull_request": map[string]any{"url": "https://example/pulls/8"}}
			},
			run: func(f *GitHubForge) (any, error) { return f.GetIssue(forgeTestRepo, 8) },
		},
		{
			name: "reviews_at_head",
			setup: func(s *goldenServer) {
				s.reviews = []map[string]any{
					{"id": 1, "state": "APPROVED", "commit_id": "abc123",
						"user": map[string]any{"login": "reviewer[bot]", "id": 42}, "body": "lgtm", "submitted_at": "2026-08-24T00:00:00Z"},
				}
			},
			run: func(f *GitHubForge) (any, error) { return f.ReviewsAtHead(forgeTestRepo, 7) },
		},
		{
			name:  "reviews_walk_two_pages",
			setup: func(s *goldenServer) { s.bigReviewPages = true },
			run: func(f *GitHubForge) (any, error) {
				rs, err := f.ReviewsAtHead(forgeTestRepo, 7)
				// Result omitted from the golden (101 synthetic entries); the point of this
				// case is the request SEQUENCE — page=1 (full) then page=2 (short, stop).
				return len(rs), err
			},
		},
		{
			name: "list_changed_files",
			setup: func(s *goldenServer) {
				s.files = []map[string]any{
					{"filename": "config/authz/token.go", "previous_filename": "secrets/auth/token.go", "status": "renamed"},
					{"filename": "docs/desk-tools.md", "status": "modified"},
				}
			},
			run: func(f *GitHubForge) (any, error) { return f.ListChangedFiles(forgeTestRepo, 7) },
		},
		{
			name: "checks_at_head",
			setup: func(s *goldenServer) {
				s.status = map[string]any{"state": "success", "total_count": 1,
					"statuses": []map[string]any{{"state": "success", "context": "ci/legacy"}}}
				s.checks = map[string]any{"total_count": 1,
					"check_runs": []map[string]any{{"name": "go-test", "status": "completed", "conclusion": "success"}}}
			},
			run: func(f *GitHubForge) (any, error) { return f.ChecksAtHead(forgeTestRepo, "abc123") },
		},
		{
			name: "issue_reactions",
			setup: func(s *goldenServer) {
				s.reactions = []map[string]any{{"content": "+1", "user": map[string]any{"login": "ian", "type": "User", "id": 1}}}
			},
			run: func(f *GitHubForge) (any, error) { return f.IssueReactions(forgeTestRepo, 7) },
		},
		{
			name:  "repo_visibility",
			setup: func(s *goldenServer) { s.repo = map[string]any{"visibility": "public"} },
			run:   func(f *GitHubForge) (any, error) { return f.RepoVisibility(forgeTestRepo) },
		},
		{
			name: "create_draft_change",
			setup: func(s *goldenServer) {
				s.createResp = map[string]any{"number": 21, "node_id": "PR_new", "html_url": "https://example/pull/21"}
			},
			run: func(f *GitHubForge) (any, error) {
				return f.CreateDraftChange(forgeTestRepo, DraftChangeInput{Title: "t", Body: "b", Head: "feat/x", Base: "main"})
			},
		},
		{
			name:  "post_comment",
			setup: func(s *goldenServer) {},
			run:   func(f *GitHubForge) (any, error) { return nil, f.PostComment(forgeTestRepo, 7, "hello") },
		},
		{
			name:  "post_review",
			setup: func(s *goldenServer) {},
			run: func(f *GitHubForge) (any, error) {
				return nil, f.PostReview(forgeTestRepo, 7, ReviewInput{HeadSHA: "abc123", Event: "APPROVE", Body: "ok"})
			},
		},
		{
			name: "mark_ready_for_review",
			setup: func(s *goldenServer) {
				s.graphql = map[string]any{"data": map[string]any{"markPullRequestReadyForReview": map[string]any{"pullRequest": map[string]any{"isDraft": false}}}}
			},
			run: func(f *GitHubForge) (any, error) { return nil, f.MarkReadyForReview("PR_node") },
		},
		{
			name: "file_issue",
			setup: func(s *goldenServer) {
				s.createResp = map[string]any{"number": 33, "html_url": "https://example/issues/33"}
			},
			run: func(f *GitHubForge) (any, error) {
				return f.FileIssue(forgeTestRepo, IssueInput{Title: "bug", Body: "detail"})
			},
		},
		{
			name:  "close_issue",
			setup: func(s *goldenServer) { s.issue = map[string]any{"number": 33, "state": "closed"} },
			run:   func(f *GitHubForge) (any, error) { return nil, f.CloseIssue(forgeTestRepo, 33, "completed") },
		},
		{
			name:  "error_not_found",
			setup: func(s *goldenServer) { s.forceStatus["/issues/404"] = http.StatusNotFound },
			run:   func(f *GitHubForge) (any, error) { return f.GetIssue(forgeTestRepo, 404) },
		},
		{
			name:  "error_forbidden",
			setup: func(s *goldenServer) { s.forceStatus["/pulls/7"] = http.StatusForbidden },
			run:   func(f *GitHubForge) (any, error) { return f.GetPullRequest(forgeTestRepo, 7) },
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := newGoldenServer(t)
			tc.setup(s)
			res, err := tc.run(s.forge())

			cap := capture{Requests: normalizeRequests(s.requests)}
			if err != nil {
				cap.Err = err.Error()
				nf := IsForgeNotFound(err)
				cap.NotFound = &nf
			}
			if res != nil {
				b, mErr := json.MarshalIndent(res, "", "  ")
				if mErr != nil {
					t.Fatalf("marshal result: %v", mErr)
				}
				cap.Result = json.RawMessage(b)
			}

			got, mErr := json.MarshalIndent(cap, "", "  ")
			if mErr != nil {
				t.Fatalf("marshal capture: %v", mErr)
			}
			got = append(got, '\n')

			goldenPath := filepath.Join("testdata", "forge_golden", tc.name+".golden.json")
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

// normalizeRequests sorts nothing (order is meaningful) but strips volatile fields — there
// are none here, so it returns the sequence as recorded. Kept as a seam so a future volatile
// header/field has one place to be scrubbed.
func normalizeRequests(in []reqCapture) []reqCapture {
	if in == nil {
		return []reqCapture{}
	}
	return in
}

// TestForgeGithubGoldenCount is a guard: the golden-pinned operation count must stay at or
// above the brief's floor (Verify item 3: "lists ≥ 10 golden-pinned operations"). A future
// edit that trims the corpus below the floor fails here with a clear message rather than
// silently weakening the single-point-of-failure control.
func TestForgeGithubGoldenCount(t *testing.T) {
	dir := filepath.Join("testdata", "forge_golden")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read golden dir %s: %v (run TestForgeGithubGolden -update first)", dir, err)
	}
	var ops []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".golden.json") {
			ops = append(ops, strings.TrimSuffix(e.Name(), ".golden.json"))
		}
	}
	sort.Strings(ops)
	if len(ops) < 10 {
		t.Fatalf("golden corpus pins %d operations, below the floor of 10: %v", len(ops), ops)
	}
	t.Logf("golden corpus pins %d operations: %s", len(ops), strings.Join(ops, ", "))
}
