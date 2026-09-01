package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Head-SHA fixtures. These are FULL 40-char lowercase-hex SHAs — the only form
// `deskpost review --head` accepts (#214, main.go's isFullSHA gate) and the only form
// GitHub ever reports. They were placeholder words ("headSHA", "OLDHEAD") until that
// gate landed, which meant the whole review suite exercised a --head shape production
// now refuses; a test suite that cannot represent the real argument cannot catch a
// regression in how it is handled.
const (
	testHead    = "5d529c27e3b1a04f9c2d8e7b6a1f0c3d4e5f6a7b"
	testOldHead = "0f1e2d3c4b5a69788796a5b4c3d2e1f009182736"
	testNewHead = "9a8b7c6d5e4f30211203344556677889aabbccdd"
)

// --- one RSA key for the whole test binary (keygen is slow) ---

var (
	testKeyOnce sync.Once
	testKeyPEM  []byte
)

func reviewerPEM(t *testing.T) []byte {
	t.Helper()
	testKeyOnce.Do(func() {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			panic(err)
		}
		der := x509.MarshalPKCS1PrivateKey(key)
		testKeyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})
	})
	return testKeyPEM
}

// fakeGH is a scriptable stand-in for the GitHub REST + GraphQL API. Every request is
// recorded so tests can assert that a refusal made NO mutating call.
type fakeGH struct {
	srv *httptest.Server

	mu   sync.Mutex
	hits []string // "METHOD path"

	// GET /pulls/{n} returns pullHeads[i] on the i-th call (clamped to last), so a test
	// can move the head between the precondition checks and the TOCTOU re-read.
	pullHeads []string
	prState   string
	prDraft   bool
	prNodeID  string

	// Trust-gate fixtures: the PR author identity (defaults to the trusted shared-agent
	// so pre-gate tests run unchanged) and the GraphQL trust-query response for
	// untrusted-author flows (defaults to an empty, unblessed payload).
	prAuthor   string
	prAuthorID int64
	trustJSON  string

	// Issue fixtures (#296). GitHub numbers issues and PRs from ONE
	// sequence, so the fake models that: a number in issueNums is an ISSUE — GET
	// /pulls/{n} 404s for it and GET /issues/{n} serves it with NO `pull_request` key —
	// while any other number is a PR, which the issues endpoint also serves, WITH that
	// key. A number in missingNums is neither, and both endpoints 404.
	issueNums   map[int]bool
	missingNums map[int]bool
	// pullStatus forces a NON-404 status on GET /pulls/{n} (403, 500, …). It is the knob
	// that makes the "only a 404 licenses the issues lookup" guard testable at all: without
	// it the fake can only ever fail that endpoint with the one status that DOES license
	// re-resolution, so deleting the guard would be invisible to the suite.
	pullStatus map[int]int
	// issueStatus is the same knob for the SECOND lookup: a NON-404 status on GET
	// /issues/{n}. Without it the fake can only 404 that endpoint, so "a 403 here is
	// reported as itself, not as 'the number does not exist'" cannot be pinned.
	issueStatus map[int]int
	// phantomPRNums is the state neither map above can express: a number that 404s on GET
	// /pulls/{n} but that the issues endpoint serves WITH a `pull_request` sub-object —
	// i.e. it IS a pull request whose PR read failed (a race with a delete, a permissions
	// edge). It is the only fixture that exercises the `pull_request` discriminator as a
	// GUARD rather than as a happy path: without it, deleting that check would let a PR be
	// commented on AS an issue, unnoticed.
	phantomPRNums  map[int]bool
	issueAuthor    string
	issueAuthorID  int64
	issueTrustJSON string

	reviews []reviewInfo
	// issueComments is what GET /issues/{n}/comments serves — the shape the
	// pr-review-desk skill used to prescribe for a clean security pass. The ready gate
	// must never read it (#513).
	issueComments []map[string]any
	files         []string
	// fileEntries, when non-nil, REPLACES files — for fixtures that need the full
	// /pulls/{n}/files entry shape (renames carry previous_filename).
	fileEntries []prFile
	// changedFilesCount, when > 0, is what GET /pulls/{n} reports as changed_files.
	// Zero means "tell the truth" (the number of entries served), so the ready gate's
	// reconciliation passes by default and a SHORT read has to be asked for explicitly.
	changedFilesCount int
	status            combinedStatus
	checks            checkRunsResp

	// intercept forces a status for a matching request (nil = normal handling).
	intercept func(method, path string) (int, bool)

	// repoVisibility is the .visibility returned for GET /repos/{owner}/{repo}.
	// Defaults to "private" so the public-repo gate does not block existing tests.
	repoVisibility string

	// repoReactions is the reaction list returned for GET .../reactions.
	repoReactions []deskkit.Reaction

	pullCalls    int
	postedReview int
	postedCmt    int
	flips        int

	// Label + surface-config fixtures for the mechanical verdict-time labeler.
	// prLabels is the current label set on the PR (mutated by add/remove routes, as GitHub
	// does — labels are a set). surfaceConfig, when non-nil, is the raw bytes GET
	// /contents/.assay-surfaces serves (base64-wrapped by the route); nil ⇒ 404 (absent
	// config). createdLabels records POST /labels calls (ensure).
	prLabels      []string
	surfaceConfig *string
	createdLabels []string
}

var (
	reTokens    = regexp.MustCompile(`/access_tokens$`)
	rePull      = regexp.MustCompile(`/pulls/[0-9]+$`)
	reReviews   = regexp.MustCompile(`/pulls/[0-9]+/reviews$`)
	reFiles     = regexp.MustCompile(`/pulls/[0-9]+/files$`)
	reComments  = regexp.MustCompile(`/issues/[0-9]+/comments$`)
	reIssue     = regexp.MustCompile(`/issues/([0-9]+)$`)
	reStatus    = regexp.MustCompile(`/commits/[^/]+/status$`)
	reChecks    = regexp.MustCompile(`/commits/[^/]+/check-runs$`)
	reRepo      = regexp.MustCompile(`^/repos/[^/]+/[^/]+$`)
	reReactions = regexp.MustCompile(`/issues/[0-9]+/reactions$`)

	reRepoLabels   = regexp.MustCompile(`^/repos/[^/]+/[^/]+/labels$`)
	reIssueLabels  = regexp.MustCompile(`/issues/[0-9]+/labels$`)
	reIssueLabelOf = regexp.MustCompile(`/issues/[0-9]+/labels/(.+)$`)
	reContents     = regexp.MustCompile(`^/repos/[^/]+/[^/]+/contents/(.+)$`)
)

// ghPaging mimics GitHub's documented paging contract: `per_page` defaults to **30** and
// caps at 100; `page` defaults to 1. This default is the whole point — a client that
// sends NO per_page silently gets only the first 30 items of a longer rollup.
func ghPaging(q url.Values) (per, page int) {
	per, page = 30, 1
	if v, err := strconv.Atoi(q.Get("per_page")); err == nil && v > 0 {
		per = v
		if per > 100 {
			per = 100
		}
	}
	if v, err := strconv.Atoi(q.Get("page")); err == nil && v > 0 {
		page = v
	}
	return per, page
}

// pageBounds returns the [lo,hi) slice bounds of page `page` (1-based, `per` per page)
// over n items, clamped to n.
func pageBounds(n, per, page int) (lo, hi int) {
	lo = (page - 1) * per
	if lo > n {
		lo = n
	}
	hi = lo + per
	if hi > n {
		hi = n
	}
	return lo, hi
}

// manyChecks builds n completed check runs; the run at index failAt (0-based, <0 for
// none) is a `failure`. TotalCount is set to n — what GitHub reports at the head.
func manyChecks(n, failAt int) checkRunsResp {
	cr := checkRunsResp{TotalCount: n}
	for i := 0; i < n; i++ {
		concl := "success"
		if i == failAt {
			concl = "failure"
		}
		cr.CheckRuns = append(cr.CheckRuns, struct {
			Name       string `json:"name"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
		}{Name: fmt.Sprintf("check-%02d", i+1), Status: "completed", Conclusion: concl})
	}
	return cr
}

// numFromPath extracts the trailing object number from "/repos/o/r/{pulls,issues}/{n}".
// Returns 0 when there is none (which matches no fixture map, so the caller falls through
// to the normal PR handling).
func numFromPath(path string) int {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return 0
	}
	n, err := strconv.Atoi(path[i+1:])
	if err != nil {
		return 0
	}
	return n
}

func (f *fakeGH) headFor(call int) string {
	if len(f.pullHeads) == 0 {
		return "head0"
	}
	if call >= len(f.pullHeads) {
		return f.pullHeads[len(f.pullHeads)-1]
	}
	return f.pullHeads[call]
}

func (f *fakeGH) handler(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.hits = append(f.hits, r.Method+" "+r.URL.Path)
	f.mu.Unlock()

	if f.intercept != nil {
		if code, ok := f.intercept(r.Method, r.URL.Path); ok {
			w.WriteHeader(code)
			return
		}
	}

	path := r.URL.Path
	page := r.URL.Query().Get("page")
	writeJSON := func(v any) { _ = json.NewEncoder(w).Encode(v) }

	switch {
	case r.Method == http.MethodPost && reTokens.MatchString(path):
		w.WriteHeader(http.StatusCreated)
		writeJSON(map[string]any{"token": "fake-installation-token", "expires_at": time.Now().Add(time.Hour).Format(time.RFC3339)})

	case r.Method == http.MethodGet && rePull.MatchString(path):
		// A number that is an ISSUE (or absent entirely) 404s here — exactly what GitHub
		// does, and the 404 deskpost re-resolves against the issues endpoint (#296).
		n := numFromPath(path)
		// A forced non-404 (403/5xx) says nothing about the KIND — deskpost must surface it
		// unchanged and never reach the issues endpoint. Checked BEFORE the issue fixtures
		// so a test can drive "this number really is an issue, but /pulls 403s".
		if code := f.pullStatus[n]; code != 0 {
			w.WriteHeader(code)
			return
		}
		if f.issueNums[n] || f.missingNums[n] || f.phantomPRNums[n] {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		f.mu.Lock()
		call := f.pullCalls
		f.pullCalls++
		f.mu.Unlock()
		p := prInfo{
			Number: 1, State: f.prState, Draft: f.prDraft, NodeID: f.prNodeID,
			ChangedFiles: f.reportedChangedFiles(),
			Head: struct {
				SHA string `json:"sha"`
			}{SHA: f.headFor(call)},
		}
		p.User.Login = f.prAuthor
		p.User.ID = f.prAuthorID
		if p.User.Login == "" {
			p.User.Login = "shared-agent"
			p.User.ID = 2002
		}
		writeJSON(p)

	case r.Method == http.MethodPost && reReviews.MatchString(path):
		// Record the submitted review into f.reviews, as GitHub does: a subsequent
		// listReviews on the same PR must SEE it. Without this the fake's GET is frozen at
		// its fixture and the cross-session (GitHub-state) idempotency guard can never be
		// exercised by a two-post sequence (#220).
		var in struct {
			CommitID string `json:"commit_id"`
			Event    string `json:"event"`
			Body     string `json:"body"`
		}
		reqBody, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(reqBody, &in)
		// GitHub's event→state mapping, all three events. COMMENT→COMMENTED is what
		// `security-review --verdict pass` submits (#513); a fake that
		// collapsed it to APPROVED would make the whole point of that shape — a review
		// gate (e) can see WITHOUT flipping the board green — untestable.
		state := map[string]string{
			"APPROVE":         "APPROVED",
			"REQUEST_CHANGES": "CHANGES_REQUESTED",
			"COMMENT":         "COMMENTED",
		}[in.Event]
		if state == "" {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		f.mu.Lock()
		f.postedReview++
		// Assign a review id as GitHub does; the dedup guard's messages name it (#518).
		submitted := reviewInfo{ID: int64(1000 + f.postedReview), State: state, CommitID: in.CommitID, Body: in.Body}
		submitted.User.Login = reviewerBotDisplay()
		f.reviews = append(f.reviews, submitted)
		id := submitted.ID
		f.mu.Unlock()
		w.WriteHeader(http.StatusOK)
		writeJSON(map[string]any{"id": id, "state": state})

	case r.Method == http.MethodGet && reReviews.MatchString(path):
		if page != "" && page != "1" {
			writeJSON([]reviewInfo{})
			return
		}
		writeJSON(f.reviews)

	case r.Method == http.MethodGet && reFiles.MatchString(path):
		if page != "" && page != "1" {
			writeJSON([]prFile{})
			return
		}
		writeJSON(f.servedFiles())

	case r.Method == http.MethodGet && reStatus.MatchString(path):
		per, pg := ghPaging(r.URL.Query())
		lo, hi := pageBounds(len(f.status.Statuses), per, pg)
		out := f.status                         // copy (TotalCount echoed verbatim — the fake never recomputes it)
		out.Statuses = f.status.Statuses[lo:hi] // the page GitHub would return
		writeJSON(out)

	case r.Method == http.MethodGet && reChecks.MatchString(path):
		per, pg := ghPaging(r.URL.Query())
		lo, hi := pageBounds(len(f.checks.CheckRuns), per, pg)
		out := f.checks
		out.CheckRuns = f.checks.CheckRuns[lo:hi]
		writeJSON(out)

	case r.Method == http.MethodGet && reIssue.MatchString(path):
		// The issues endpoint serves BOTH kinds; `pull_request` is present iff the number
		// is a PR. That sub-object is the whole discriminator deskpost resolves on.
		n := numFromPath(path)
		// A forced non-404 here says nothing about existence — deskpost must surface it as
		// itself, never as "the number is neither a PR nor an issue".
		if code := f.issueStatus[n]; code != 0 {
			w.WriteHeader(code)
			return
		}
		if f.missingNums[n] {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		out := map[string]any{"number": n, "state": "open"}
		if f.issueNums[n] {
			login, id := f.issueAuthor, f.issueAuthorID
			if login == "" {
				login, id = "shared-agent", int64(2002)
			}
			out["user"] = map[string]any{"login": login, "id": id}
		} else {
			out["user"] = map[string]any{"login": "shared-agent", "id": 2002}
			out["pull_request"] = map[string]any{"url": "https://api.github.com/pulls/" + strconv.Itoa(n)}
		}
		writeJSON(out)

	case r.Method == http.MethodPost && reComments.MatchString(path):
		var in struct {
			Body string `json:"body"`
		}
		reqBody, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(reqBody, &in)
		f.mu.Lock()
		f.postedCmt++
		// Record the posted body so a subsequent GET sees it — the surface-tier comment's
		// marker dedup (a re-run must not post a duplicate) is only testable if the fake
		// remembers what was posted.
		f.issueComments = append(f.issueComments, map[string]any{"body": in.Body})
		f.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		writeJSON(map[string]any{"id": 1})

	case r.Method == http.MethodGet && reRepo.MatchString(path):
		vis := f.repoVisibility
		if vis == "" {
			vis = "private"
		}
		writeJSON(map[string]any{"visibility": vis})

	case r.Method == http.MethodGet && reReactions.MatchString(path):
		if f.repoReactions != nil {
			writeJSON(f.repoReactions)
		} else {
			writeJSON([]deskkit.Reaction{})
		}

	case r.Method == http.MethodPost && reRepoLabels.MatchString(path):
		// Ensure-label. Record the create; return 201. (A real repo 422s on an existing
		// label; the client swallows that, so returning 201 unconditionally is a superset.)
		var in struct {
			Name string `json:"name"`
		}
		reqBody, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(reqBody, &in)
		f.mu.Lock()
		f.createdLabels = append(f.createdLabels, in.Name)
		f.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		writeJSON(map[string]any{"name": in.Name})

	case r.Method == http.MethodGet && reIssueLabels.MatchString(path):
		f.mu.Lock()
		out := make([]map[string]any, 0, len(f.prLabels))
		for _, l := range f.prLabels {
			out = append(out, map[string]any{"name": l})
		}
		f.mu.Unlock()
		writeJSON(out)

	case r.Method == http.MethodPost && reIssueLabels.MatchString(path):
		var in struct {
			Labels []string `json:"labels"`
		}
		reqBody, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(reqBody, &in)
		f.mu.Lock()
		for _, add := range in.Labels {
			present := false
			for _, cur := range f.prLabels {
				if cur == add {
					present = true
					break
				}
			}
			if !present {
				f.prLabels = append(f.prLabels, add) // labels are a SET — no duplicate
			}
		}
		f.mu.Unlock()
		w.WriteHeader(http.StatusOK)
		writeJSON([]map[string]any{})

	case r.Method == http.MethodDelete && reIssueLabelOf.MatchString(path):
		name := reIssueLabelOf.FindStringSubmatch(path)[1]
		if dec, err := url.PathUnescape(name); err == nil {
			name = dec
		}
		f.mu.Lock()
		kept := f.prLabels[:0:0]
		found := false
		for _, cur := range f.prLabels {
			if cur == name {
				found = true
				continue
			}
			kept = append(kept, cur)
		}
		f.prLabels = kept
		f.mu.Unlock()
		if !found {
			w.WriteHeader(http.StatusNotFound) // idempotent-remove path the client swallows
			return
		}
		w.WriteHeader(http.StatusOK)
		writeJSON([]map[string]any{})

	case r.Method == http.MethodGet && reContents.MatchString(path):
		if f.surfaceConfig == nil {
			w.WriteHeader(http.StatusNotFound) // absent-config signal
			return
		}
		enc := base64.StdEncoding.EncodeToString([]byte(*f.surfaceConfig))
		writeJSON(map[string]any{"content": enc, "encoding": "base64"})

	case r.Method == http.MethodGet && reComments.MatchString(path):
		// Serving this endpoint at all is the point (#513): the ready gate
		// must be shown to make NO read here, and a fixture that 404s could not tell
		// "deskpost did not ask" from "deskpost asked and got nothing".
		if page != "" && page != "1" {
			writeJSON([]map[string]any{})
			return
		}
		writeJSON(f.issueComments)

	case r.Method == http.MethodPost && path == "/graphql":
		// Three GraphQL calls exist: the ready-flip MUTATION and the trust-gate QUERY in
		// its PR and ISSUE forms. Branch on the request body — only the mutation counts as
		// a flip, and the two trust queries return different response shapes.
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "markPullRequestReadyForReview") {
			f.mu.Lock()
			f.flips++
			f.mu.Unlock()
			writeJSON(map[string]any{"data": map[string]any{"markPullRequestReadyForReview": map[string]any{"pullRequest": map[string]any{"isDraft": false}}}})
			return
		}
		if strings.Contains(string(body), "issue(number:") {
			if f.issueTrustJSON != "" {
				_, _ = w.Write([]byte(f.issueTrustJSON))
				return
			}
			_, _ = w.Write([]byte(`{"data":{"repository":{"issue":{"lastEditedAt":null,"comments":{"pageInfo":{"hasNextPage":false},"nodes":[]}}}}}`))
			return
		}
		if f.trustJSON != "" {
			_, _ = w.Write([]byte(f.trustJSON))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"repository":{"pullRequest":{"lastEditedAt":null,"comments":{"pageInfo":{"hasNextPage":false},"nodes":[]},"reviews":{"pageInfo":{"hasNextPage":false},"nodes":[]},"reviewThreads":{"pageInfo":{"hasNextPage":false},"nodes":[]}}}}}`))

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (f *fakeGH) hitCount(method, suffix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	re := regexp.MustCompile(regexp.QuoteMeta(suffix) + "$")
	for _, h := range f.hits {
		if len(h) >= len(method) && h[:len(method)] == method && re.MatchString(h) {
			n++
		}
	}
	return n
}

// setupFake wires isolation (a temp HOME so the audit log + kill switch + lock live in a
// throwaway dir), the reviewer PEM, the fake API base URL, and captured stdout/stderr.
// It returns the fake and a buffer capturing stderr.
func setupFake(t *testing.T) (*fakeGH, *bytes.Buffer) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("CLAUDE_SESSION_ID", "deskpost-test")

	pemPath := filepath.Join(home, "reviewer-app.pem")
	if err := os.WriteFile(pemPath, reviewerPEM(t), 0o600); err != nil {
		t.Fatalf("write pem: %v", err)
	}
	t.Setenv("REVIEWER_PEM", pemPath)
	t.Setenv("REVIEWER_APP_ID", "999999")
	t.Setenv("REVIEWER_INSTALL_ID", "100000002")

	f := &fakeGH{
		pullHeads:     []string{testHead},
		prState:       "open",
		prDraft:       true,
		prNodeID:      "PR_node_1",
		issueNums:     map[int]bool{},
		missingNums:   map[int]bool{},
		pullStatus:    map[int]int{},
		issueStatus:   map[int]int{},
		phantomPRNums: map[int]bool{},
		// A default NON-risk changed file. It has to be stated: under the public-repo risk rule, an
		// EMPTY changed-file list is itself risk-classed (fail closed — "we could not
		// see the diff" is not "the diff is clean"), so a fixture that says nothing
		// about files would silently exercise the security gate in every test.
		files: []string{"docs/desk-tools.md"},
	}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handler))
	t.Cleanup(f.srv.Close)

	oldBase := apiBaseURL
	apiBaseURL = f.srv.URL
	t.Cleanup(func() { apiBaseURL = oldBase })

	var errBuf bytes.Buffer
	oldOut, oldErr := stdout, stderr
	stdout = &bytes.Buffer{}
	stderr = &errBuf
	t.Cleanup(func() { stdout, stderr = oldOut, oldErr })

	return f, &errBuf
}

// servedFiles is the /pulls/{n}/files payload: fileEntries when set, else one plain
// entry per f.files path.
func (f *fakeGH) servedFiles() []prFile {
	if f.fileEntries != nil {
		return f.fileEntries
	}
	out := make([]prFile, 0, len(f.files))
	for _, n := range f.files {
		out = append(out, prFile{Filename: n})
	}
	return out
}

// reportedChangedFiles is what GET /pulls/{n} claims the diff size is. It tells the
// truth unless a test deliberately inflates it to simulate a short read.
func (f *fakeGH) reportedChangedFiles() int {
	if f.changedFilesCount > 0 {
		return f.changedFilesCount
	}
	return len(f.servedFiles())
}

// auditEntries reads the isolated audit log for assertions.
func auditEntries(t *testing.T) []deskkit.Entry {
	t.Helper()
	e, err := deskkit.LoadEntries()
	if err != nil {
		t.Fatalf("load audit: %v", err)
	}
	return e
}

func lastAudit(t *testing.T) deskkit.Entry {
	t.Helper()
	e := auditEntries(t)
	if len(e) == 0 {
		t.Fatal("no audit entries")
	}
	return e[len(e)-1]
}

// writeBody writes a body file under the temp HOME and returns its path.
func writeBody(t *testing.T, name, content string) string {
	t.Helper()
	h, _ := os.UserHomeDir()
	p := filepath.Join(h, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write body: %v", err)
	}
	return p
}
