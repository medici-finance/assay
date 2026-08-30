package deskkit

import (
	"encoding/json"
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

// forge_gitlab_test.go — the GitLab contract suite. It replays brief-01's golden SCENARIOS
// (same names) against recorded GitLab REST v4 fixtures on an httptest server, asserting the
// GitLab implementation maps each operation to the SAME observable semantics the github
// goldens pin. This is the single-point-of-failure control the brief names: the concept-
// mapping table backed by contract tests, so a wrong mapping (approval vs note, award vs
// reaction, MR vs issue) fails a fixture here, not a live pilot.
//
// It does not use golden JSON files (the github corpus does): the GitLab wire differs per
// operation and the point is the MAPPED result, so results are asserted inline against
// expected structs, and request SEQUENCES (pagination, the approve sha-pin) against the
// recorded requests. reqCapture / readAllCompact are shared with forge_github_golden_test.go.

// gitlabServer is a canned GitLab REST v4 endpoint that records every request and serves
// per-scenario fixtures keyed by a small router. Pagination is driven by the `page` query and
// the X-Next-Page header (GitLab's mechanism — not link relations).
type gitlabServer struct {
	srv      *httptest.Server
	requests []reqCapture

	mr          map[string]any
	issue       map[string]any
	approvals   map[string]any
	notes       []map[string]any
	notesPage2  []map[string]any
	diffs       []map[string]any
	diffsPage2  []map[string]any
	pipelines   []map[string]any
	jobs        []map[string]any
	statuses    []map[string]any
	statusTotal string
	jobsTotal   string
	awards      []map[string]any
	project     map[string]any
	createMR    map[string]any
	createIssue map[string]any

	// forceStatus: when a path SUFFIX matches, return that HTTP status (error-mapping cases).
	forceStatus map[string]int
}

var (
	xMRNum       = regexp.MustCompile(`^/api/v4/projects/[^/]+/[^/]+/merge_requests/[0-9]+$`)
	xMRRoot      = regexp.MustCompile(`^/api/v4/projects/[^/]+/[^/]+/merge_requests$`)
	xMRApprovals = regexp.MustCompile(`/merge_requests/[0-9]+/approvals$`)
	xMRApprove   = regexp.MustCompile(`/merge_requests/[0-9]+/approve$`)
	xMRNotes     = regexp.MustCompile(`/merge_requests/[0-9]+/notes$`)
	xMRDiffs     = regexp.MustCompile(`/merge_requests/[0-9]+/diffs$`)
	xIssueNum    = regexp.MustCompile(`^/api/v4/projects/[^/]+/[^/]+/issues/[0-9]+$`)
	xIssueRoot   = regexp.MustCompile(`^/api/v4/projects/[^/]+/[^/]+/issues$`)
	xIssueNotes  = regexp.MustCompile(`/issues/[0-9]+/notes$`)
	xAward       = regexp.MustCompile(`/issues/[0-9]+/award_emoji$`)
	xPipelines   = regexp.MustCompile(`^/api/v4/projects/[^/]+/[^/]+/pipelines$`)
	xJobs        = regexp.MustCompile(`/pipelines/[0-9]+/jobs$`)
	xStatuses    = regexp.MustCompile(`/repository/commits/[^/]+/statuses$`)
	xProject     = regexp.MustCompile(`^/api/v4/projects/[^/]+/[^/]+$`)
)

func (s *gitlabServer) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := readAllCompact(r)
	s.requests = append(s.requests, reqCapture{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: body})
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
	// --- writes (most specific first) ---
	case r.Method == http.MethodPost && xMRApprove.MatchString(path):
		w.WriteHeader(http.StatusCreated)
		enc(map[string]any{"id": 1})
	case r.Method == http.MethodPost && xMRNotes.MatchString(path):
		w.WriteHeader(http.StatusCreated)
		enc(map[string]any{"id": 1})
	case r.Method == http.MethodPost && xIssueNotes.MatchString(path):
		w.WriteHeader(http.StatusCreated)
		enc(map[string]any{"id": 1})
	case r.Method == http.MethodPost && xMRRoot.MatchString(path):
		w.WriteHeader(http.StatusCreated)
		enc(s.createMR)
	case r.Method == http.MethodPost && xIssueRoot.MatchString(path):
		w.WriteHeader(http.StatusCreated)
		enc(s.createIssue)
	case r.Method == http.MethodPut && xMRNum.MatchString(path):
		enc(map[string]any{"iid": 1})
	case r.Method == http.MethodPut && xIssueNum.MatchString(path):
		enc(map[string]any{"iid": 1, "state": "closed"})

	// --- reads ---
	case r.Method == http.MethodGet && xMRApprovals.MatchString(path):
		enc(s.approvals)
	case r.Method == http.MethodGet && xMRNotes.MatchString(path):
		if page == "2" {
			enc(s.notesPage2)
			return
		}
		if len(s.notesPage2) > 0 {
			w.Header().Set("X-Next-Page", "2")
		}
		enc(s.notes)
	case r.Method == http.MethodGet && xMRDiffs.MatchString(path):
		if page == "2" {
			enc(s.diffsPage2)
			return
		}
		if len(s.diffsPage2) > 0 {
			w.Header().Set("X-Next-Page", "2")
		}
		enc(s.diffs)
	case r.Method == http.MethodGet && xMRNum.MatchString(path):
		enc(s.mr)
	case r.Method == http.MethodGet && xStatuses.MatchString(path):
		if s.statusTotal != "" {
			w.Header().Set("X-Total", s.statusTotal)
		}
		enc(s.statuses)
	case r.Method == http.MethodGet && xJobs.MatchString(path):
		if s.jobsTotal != "" {
			w.Header().Set("X-Total", s.jobsTotal)
		}
		enc(s.jobs)
	case r.Method == http.MethodGet && xPipelines.MatchString(path):
		enc(s.pipelines)
	case r.Method == http.MethodGet && xAward.MatchString(path):
		enc(s.awards)
	case r.Method == http.MethodGet && xIssueNum.MatchString(path):
		enc(s.issue)
	case r.Method == http.MethodGet && xProject.MatchString(path):
		enc(s.project)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func newGitlabServer(t *testing.T) *gitlabServer {
	t.Helper()
	s := &gitlabServer{forceStatus: map[string]int{}}
	s.srv = httptest.NewServer(http.HandlerFunc(s.handler))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *gitlabServer) forge() *GitLabForge {
	return &GitLabForge{Token: "test-token", BaseURL: s.srv.URL + "/api/v4", Client: s.srv.Client()}
}

// countRequests counts recorded requests whose decoded path matches re.
func (s *gitlabServer) countRequests(re *regexp.Regexp, method string) int {
	n := 0
	for _, rq := range s.requests {
		if rq.Method == method && re.MatchString(rq.Path) {
			n++
		}
	}
	return n
}

// lastBody returns the compacted JSON body of the last request matching re+method.
func (s *gitlabServer) lastBody(re *regexp.Regexp, method string) string {
	last := ""
	for _, rq := range s.requests {
		if rq.Method == method && re.MatchString(rq.Path) {
			last = string(rq.Body)
		}
	}
	return last
}

// glScenario is one contract scenario: fixtures, the operation, the expected mapped result
// (nil for a write with no return), and optional request-sequence assertions. methods names
// the interface method(s) it exercises — the coverage test reads this to prove every
// inventoried op has a contract test.
type glScenario struct {
	name    string
	methods []string
	setup   func(s *gitlabServer)
	run     func(f *GitLabForge) (any, error)
	want    any
	check   func(t *testing.T, s *gitlabServer)
}

var glTestRepo = ForgeRepo{Owner: "medici-finance", Name: "assay"}

// gitlabScenarios is the shared scenario set (used by both the contract test and the
// coverage test).
func gitlabScenarios() []glScenario {
	return []glScenario{
		{
			name:    "get_pull_request",
			methods: []string{"GetPullRequest"},
			setup: func(s *gitlabServer) {
				s.mr = map[string]any{
					"iid": 7, "state": "opened", "draft": true, "sha": "abc123",
					"changes_count": "3",
					"author":        map[string]any{"username": "worker[bot]", "id": 99},
					"web_url":       "https://example/mr/7",
				}
			},
			run:  func(f *GitLabForge) (any, error) { return f.GetPullRequest(glTestRepo, 7) },
			want: &PullRequest{Number: 7, State: "open", Draft: true, NodeID: "medici-finance/assay!7", ChangedFiles: 3, Author: Account{Login: "worker[bot]", ID: 99}, HeadSHA: "abc123"},
		},
		{
			name:    "get_issue_plain",
			methods: []string{"GetIssue"},
			setup: func(s *gitlabServer) {
				s.issue = map[string]any{"iid": 12, "state": "opened", "author": map[string]any{"username": "someone", "id": 5}}
			},
			run:  func(f *GitLabForge) (any, error) { return f.GetIssue(glTestRepo, 12) },
			want: &Issue{Number: 12, State: "open", Author: Account{Login: "someone", ID: 5}, IsPullRequest: false},
		},
		{
			name:    "reviews_at_head",
			methods: []string{"ReviewsAtHead"},
			setup: func(s *gitlabServer) {
				s.mr = map[string]any{"iid": 7, "sha": "abc123", "state": "opened"}
				s.approvals = map[string]any{"approved_by": []map[string]any{
					{"user": map[string]any{"username": "reviewer[bot]", "id": 42}},
				}}
				s.notes = []map[string]any{
					{"id": 7, "body": "lgtm", "system": false, "author": map[string]any{"username": "reviewer[bot]", "id": 42}, "created_at": "2026-08-24T00:00:00Z"},
					{"id": 8, "body": "changed the milestone", "system": true, "author": map[string]any{"username": "reviewer[bot]", "id": 42}, "created_at": "2026-08-24T00:00:01Z"},
				}
			},
			run: func(f *GitLabForge) (any, error) { return f.ReviewsAtHead(glTestRepo, 7) },
			want: []Review{
				{ID: 42, Author: Account{Login: "reviewer[bot]", ID: 42}, State: "APPROVED", CommitID: "abc123"},
				{ID: 7, Author: Account{Login: "reviewer[bot]", ID: 42}, State: "COMMENTED", CommitID: "abc123", Body: "lgtm", SubmittedAt: "2026-08-24T00:00:00Z"},
			},
		},
		{
			name:    "list_changed_files",
			methods: []string{"ListChangedFiles"},
			setup: func(s *gitlabServer) {
				s.diffs = []map[string]any{
					{"old_path": "secrets/auth/token.go", "new_path": "config/authz/token.go", "renamed_file": true},
					{"old_path": "docs/desk-tools.md", "new_path": "docs/desk-tools.md"},
				}
			},
			run: func(f *GitLabForge) (any, error) { return f.ListChangedFiles(glTestRepo, 7) },
			want: []ChangedFile{
				{Filename: "config/authz/token.go", PreviousFilename: "secrets/auth/token.go", Status: "renamed"},
				{Filename: "docs/desk-tools.md", Status: "modified"},
			},
		},
		{
			name:    "list_changed_files_two_pages",
			methods: []string{"ListChangedFiles"},
			setup: func(s *gitlabServer) {
				s.diffs = []map[string]any{{"new_path": "a.go", "new_file": true}}
				s.diffsPage2 = []map[string]any{{"old_path": "b.go", "new_path": "b.go", "deleted_file": true}}
			},
			run: func(f *GitLabForge) (any, error) { return f.ListChangedFiles(glTestRepo, 7) },
			want: []ChangedFile{
				{Filename: "a.go", Status: "added"},
				{Filename: "b.go", Status: "removed"},
			},
			check: func(t *testing.T, s *gitlabServer) {
				if n := s.countRequests(xMRDiffs, http.MethodGet); n != 2 {
					t.Errorf("X-Next-Page walk: want 2 diffs requests, got %d", n)
				}
			},
		},
		{
			name:    "checks_at_head",
			methods: []string{"ChecksAtHead"},
			setup: func(s *gitlabServer) {
				s.pipelines = []map[string]any{{"id": 555, "sha": "abc123", "status": "success"}}
				s.statuses = []map[string]any{{"name": "ci/external", "status": "success"}}
				s.statusTotal = "1"
				s.jobs = []map[string]any{{"name": "go-test", "status": "success"}}
				s.jobsTotal = "1"
			},
			run: func(f *GitLabForge) (any, error) { return f.ChecksAtHead(glTestRepo, "abc123") },
			want: &ChecksAtHead{
				CombinedState:       "success",
				StatusTotalCount:    1,
				Statuses:            []StatusContext{{State: "success", Context: "ci/external"}},
				CheckRunsTotalCount: 1,
				CheckRuns:           []CheckRun{{Name: "go-test", Status: "completed", Conclusion: "success"}},
			},
		},
		{
			name:    "issue_reactions",
			methods: []string{"IssueReactions"},
			setup: func(s *gitlabServer) {
				s.awards = []map[string]any{{"name": "thumbsup", "user": map[string]any{"username": "ian", "id": 1}}}
			},
			run:  func(f *GitLabForge) (any, error) { return f.IssueReactions(glTestRepo, 7) },
			want: []Reaction{{User: ReactionUser{Login: "ian", Type: "User"}, Content: "+1"}},
		},
		{
			name:    "repo_visibility",
			methods: []string{"RepoVisibility"},
			setup:   func(s *gitlabServer) { s.project = map[string]any{"visibility": "public"} },
			run:     func(f *GitLabForge) (any, error) { return f.RepoVisibility(glTestRepo) },
			want:    "public",
		},
		{
			name:    "create_draft_change",
			methods: []string{"CreateDraftChange"},
			setup: func(s *gitlabServer) {
				s.createMR = map[string]any{"iid": 21, "web_url": "https://example/mr/21"}
			},
			run: func(f *GitLabForge) (any, error) {
				return f.CreateDraftChange(glTestRepo, DraftChangeInput{Title: "t", Body: "b", Head: "feat/x", Base: "main"})
			},
			want: &PullRef{Number: 21, NodeID: "medici-finance/assay!21", URL: "https://example/mr/21"},
			check: func(t *testing.T, s *gitlabServer) {
				b := s.lastBody(xMRRoot, http.MethodPost)
				if !strings.Contains(b, `"title":"Draft: t"`) {
					t.Errorf("create MR body must carry the Draft: title prefix, got %s", b)
				}
			},
		},
		{
			name:    "post_comment",
			methods: []string{"PostComment"},
			setup:   func(s *gitlabServer) {},
			run:     func(f *GitLabForge) (any, error) { return nil, f.PostComment(glTestRepo, 7, "hello") },
			check: func(t *testing.T, s *gitlabServer) {
				if s.countRequests(xMRNotes, http.MethodPost) != 1 {
					t.Errorf("post_comment: want 1 MR-notes POST, got %d", s.countRequests(xMRNotes, http.MethodPost))
				}
				if s.countRequests(xIssueNotes, http.MethodPost) != 0 {
					t.Errorf("post_comment: want no issue-notes POST when MR notes succeeds")
				}
			},
		},
		{
			name:    "post_comment_issue_fallback",
			methods: []string{"PostComment"},
			setup:   func(s *gitlabServer) { s.forceStatus["/merge_requests/7/notes"] = http.StatusNotFound },
			run:     func(f *GitLabForge) (any, error) { return nil, f.PostComment(glTestRepo, 7, "hello") },
			check: func(t *testing.T, s *gitlabServer) {
				if s.countRequests(xIssueNotes, http.MethodPost) != 1 {
					t.Errorf("post_comment_issue_fallback: want 1 issue-notes POST after MR 404, got %d", s.countRequests(xIssueNotes, http.MethodPost))
				}
			},
		},
		{
			name:    "post_review",
			methods: []string{"PostReview"},
			setup:   func(s *gitlabServer) {},
			run: func(f *GitLabForge) (any, error) {
				return nil, f.PostReview(glTestRepo, 7, ReviewInput{HeadSHA: "abc123", Event: "APPROVE", Body: "ok"})
			},
			check: func(t *testing.T, s *gitlabServer) {
				if s.countRequests(xMRNotes, http.MethodPost) != 1 {
					t.Errorf("post_review: verdict body must be posted as a note")
				}
				if s.countRequests(xMRApprove, http.MethodPost) != 1 {
					t.Errorf("post_review APPROVE: want 1 approve POST, got %d", s.countRequests(xMRApprove, http.MethodPost))
				}
				if b := s.lastBody(xMRApprove, http.MethodPost); !strings.Contains(b, `"sha":"abc123"`) {
					t.Errorf("post_review APPROVE must head-pin with sha, got %s", b)
				}
			},
		},
		{
			name:    "mark_ready_for_review",
			methods: []string{"MarkReadyForReview"},
			setup: func(s *gitlabServer) {
				s.mr = map[string]any{"iid": 7, "title": "Draft: my change", "state": "opened"}
			},
			run: func(f *GitLabForge) (any, error) { return nil, f.MarkReadyForReview("medici-finance/assay!7") },
			check: func(t *testing.T, s *gitlabServer) {
				if s.countRequests(xMRNum, http.MethodPut) != 1 {
					t.Errorf("mark_ready: want 1 MR PUT, got %d", s.countRequests(xMRNum, http.MethodPut))
				}
				if b := s.lastBody(xMRNum, http.MethodPut); !strings.Contains(b, `"title":"my change"`) {
					t.Errorf("mark_ready must PUT the de-drafted title, got %s", b)
				}
			},
		},
		{
			name:    "file_issue",
			methods: []string{"FileIssue"},
			setup: func(s *gitlabServer) {
				s.createIssue = map[string]any{"iid": 33, "web_url": "https://example/issues/33"}
			},
			run: func(f *GitLabForge) (any, error) {
				return f.FileIssue(glTestRepo, IssueInput{Title: "bug", Body: "detail"})
			},
			want: &IssueRef{Number: 33, URL: "https://example/issues/33"},
		},
		{
			name:    "close_issue",
			methods: []string{"CloseIssue"},
			setup:   func(s *gitlabServer) { s.issue = map[string]any{"iid": 33, "state": "closed"} },
			run:     func(f *GitLabForge) (any, error) { return nil, f.CloseIssue(glTestRepo, 33, "completed") },
			check: func(t *testing.T, s *gitlabServer) {
				if b := s.lastBody(xIssueNum, http.MethodPut); !strings.Contains(b, `"state_event":"close"`) {
					t.Errorf("close_issue must PUT state_event=close (GitLab verb), got %s", b)
				}
			},
		},
		{
			name:    "push_transport_hint",
			methods: []string{"PushTransportHint"},
			setup:   func(s *gitlabServer) {},
			run:     func(f *GitLabForge) (any, error) { return f.PushTransportHint(glTestRepo), nil },
			check: func(t *testing.T, s *gitlabServer) {
				h := s.forge().PushTransportHint(glTestRepo)
				if h.TokenUsername != "oauth2" {
					t.Errorf("gitlab push username must be oauth2, got %q", h.TokenUsername)
				}
				if strings.Contains(h.CredentialHelperHint, "URL") == false || !strings.Contains(h.CredentialHelperHint, "credential.helper") {
					t.Errorf("push hint must forbid token-in-URL via credential.helper, got %q", h.CredentialHelperHint)
				}
				if h.RemoteHost == "" {
					t.Errorf("push hint must derive a remote host")
				}
			},
		},
	}
}

func TestForgeGitlab(t *testing.T) {
	for _, tc := range gitlabScenarios() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := newGitlabServer(t)
			tc.setup(s)
			got, err := tc.run(s.forge())
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", tc.name, err)
			}
			if tc.want != nil && !reflect.DeepEqual(got, tc.want) {
				gj, _ := json.MarshalIndent(got, "", "  ")
				wj, _ := json.MarshalIndent(tc.want, "", "  ")
				t.Errorf("%s mapping mismatch\n--- got ---\n%s\n--- want ---\n%s", tc.name, gj, wj)
			}
			if tc.check != nil {
				tc.check(t, s)
			}
		})
	}
}

// TestForgeGitlabCoverage reads the committed stream inventory (the one carrying the frozen
// `Forge` interface table), parses the method set from that table, and FAILS naming any
// inventoried operation with no gitlab contract test. Coverage is measured against the committed
// inventory, not asserted against a hand-kept count — so a method added to the interface (and the
// inventory) with no GitLab test reddens here.
//
// The inventory is located by CONTENT (the stream whose inventory carries the `Forge` interface
// table), not by a hard-coded `docs/streams/<slug>` path — a shipping file that named that path
// would publish a map to a do-not-copy stream tree (the corpusleak_test.go #1316 gate). Discovery
// stays generic over docs/streams and never embeds a withheld-stream slug.
func TestForgeGitlabCoverage(t *testing.T) {
	invPath := findForgeInventory(t)
	invMethods := parseInventoryMethods(t, invPath)
	if len(invMethods) < 10 {
		t.Fatalf("parsed only %d methods from %s — parser or inventory drifted: %v", len(invMethods), invPath, invMethods)
	}

	covered := map[string]bool{}
	for _, sc := range gitlabScenarios() {
		for _, m := range sc.methods {
			covered[m] = true
		}
	}

	var missing []string
	for _, m := range invMethods {
		if !covered[m] {
			missing = append(missing, m)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("inventoried forge ops with NO gitlab contract test: %v (inventory lists %d: %v)", missing, len(invMethods), invMethods)
	}
	t.Logf("gitlab contract tests cover all %d inventoried forge ops: %s", len(invMethods), strings.Join(invMethods, ", "))
}

// findForgeInventory locates the committed stream inventory that carries the `Forge` interface
// table, by globbing docs/streams/*/inventory.md and selecting by CONTENT. It embeds no
// withheld-stream slug (see the corpusleak_test.go gate) and asserts EXACTLY ONE match, so a
// second stream growing a `Forge` interface table — or the inventory going missing — reddens
// here rather than silently reading the wrong file.
func findForgeInventory(t *testing.T) string {
	t.Helper()
	root := filepath.Join("..", "..", "..", "..")
	matches, err := filepath.Glob(filepath.Join(root, "docs", "streams", "*", "inventory.md"))
	if err != nil {
		t.Fatalf("glob stream inventories: %v", err)
	}
	var found []string
	for _, m := range matches {
		b, rerr := os.ReadFile(m)
		if rerr != nil {
			continue
		}
		if strings.Contains(string(b), "`Forge` interface") {
			found = append(found, m)
		}
	}
	if len(found) != 1 {
		t.Fatalf("expected exactly one stream inventory carrying a `Forge` interface table, found %d: %v", len(found), found)
	}
	return found[0]
}

// parseInventoryMethods extracts the interface method NAMES from the inventory's frozen
// method-set table (the section headed "The `Forge` interface"). It reads the Method column
// (backticked `Name(...)`) and returns the identifiers, deduplicated in table order.
func parseInventoryMethods(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read inventory %s: %v", path, err)
	}
	lines := strings.Split(string(raw), "\n")
	methodCell := regexp.MustCompile("`([A-Z][A-Za-z0-9]*)\\(")

	inTable := false
	seen := map[string]bool{}
	var out []string
	for _, ln := range lines {
		if strings.HasPrefix(ln, "## ") {
			inTable = strings.Contains(ln, "`Forge` interface")
			continue
		}
		if !inTable || !strings.HasPrefix(strings.TrimSpace(ln), "|") {
			continue
		}
		cols := strings.Split(ln, "|")
		if len(cols) < 3 {
			continue
		}
		// Column index 2 is the Method column (index 0 is the empty pre-| field, 1 is #).
		m := methodCell.FindStringSubmatch(cols[2])
		if m == nil {
			continue
		}
		name := m[1]
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// TestForgeGitlabTierErrors pins the three-state error surface: a tier- or permission-gated
// GitLab feature that 401/403/404s returns a could-not-check error DISTINCT from an empty
// result — never rounded up to clean. It exercises a 403 (permission/tier), a 404 (visibility),
// and a 401 (credential), asserting each classifies as could-not-check via ForgeCheckState and
// that a 403 read returns an error rather than an empty-and-nil slice.
func TestForgeGitlabTierErrors(t *testing.T) {
	cases := []struct {
		name         string
		status       int
		forcePath    string
		run          func(f *GitLabForge) error
		wantNotFound bool
	}{
		{
			name: "forbidden_tier_gate", status: http.StatusForbidden, forcePath: "/merge_requests/7",
			run: func(f *GitLabForge) error { _, err := f.GetPullRequest(glTestRepo, 7); return err },
		},
		{
			name: "forbidden_approval_rules", status: http.StatusForbidden, forcePath: "/approvals",
			run: func(f *GitLabForge) error { _, err := f.ReviewsAtHead(glTestRepo, 7); return err },
		},
		{
			name: "not_found_visibility", status: http.StatusNotFound, forcePath: "/projects/medici-finance/assay", wantNotFound: true,
			run: func(f *GitLabForge) error { _, err := f.RepoVisibility(glTestRepo); return err },
		},
		{
			name: "unauthorized_credential", status: http.StatusUnauthorized, forcePath: "/award_emoji",
			run: func(f *GitLabForge) error { _, err := f.IssueReactions(glTestRepo, 7); return err },
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := newGitlabServer(t)
			// A valid MR head fixture so ReviewsAtHead reaches the approvals call before it 403s.
			s.mr = map[string]any{"iid": 7, "sha": "abc123", "state": "opened"}
			s.forceStatus[tc.forcePath] = tc.status

			err := tc.run(s.forge())
			if err == nil {
				t.Fatalf("%s: a %d must NOT surface as a clean nil error", tc.name, tc.status)
			}
			state := ForgeCheckState(err)
			if state != "could-not-check" {
				t.Fatalf("%s: %d must classify as could-not-check, got %q (err=%v)", tc.name, tc.status, state, err)
			}
			if IsForgeNotFound(err) != tc.wantNotFound {
				t.Errorf("%s: IsForgeNotFound=%v want %v", tc.name, IsForgeNotFound(err), tc.wantNotFound)
			}
			// Log so a human/CI sees the three-state verdict explicitly (Verify item 3).
			t.Logf("%s: HTTP %d → %s (distinct from an empty result)", tc.name, tc.status, state)
		})
	}

	// Distinctness: a 403 read returns an ERROR, not an empty-and-nil slice that a caller
	// could mistake for "checked, nothing there".
	t.Run("could_not_check_distinct_from_empty", func(t *testing.T) {
		s := newGitlabServer(t)
		s.forceStatus["/award_emoji"] = http.StatusForbidden
		got, err := s.forge().IssueReactions(glTestRepo, 7)
		if err == nil {
			t.Fatalf("403 IssueReactions returned nil error with %d results — could-not-check collapsed to clean-empty", len(got))
		}
		if ForgeCheckState(err) != "could-not-check" {
			t.Fatalf("403 IssueReactions must be could-not-check, got %q", ForgeCheckState(err))
		}
		t.Logf("403 IssueReactions → could-not-check error (results=%d), not a clean empty slice", len(got))
	})
}
