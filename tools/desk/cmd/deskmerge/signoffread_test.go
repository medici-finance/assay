package main

// signoffread_test.go — the R-5 sign-off read, served through the PRODUCTION read path
// (forgeFor -> deskkit.ForgeFor -> the custody minter -> the GitHub backend) against an
// httptest GraphQL endpoint that answers the comments query the backend actually sends.
//
// The stub forge every other test in this package installs hands back the one correct
// comment whatever item or kind it is asked for, so it cannot tell a lookup that matches the
// permalink's comment id from one that takes the first comment it sees, and it never has more
// than one comment to page through. These tests serve real threads:
//
//   - the sign-off among other comments (including another comment by the same roster human,
//     placed FIRST, so a lookup that ignores the id passes the author check and only the id
//     assertion catches it);
//   - the sign-off past the first 100 comments of a pull-request thread (the read must walk
//     the thread, never conclude absence from its first page);
//   - an id that is on no comment of the thread (refused);
//   - a permalink naming an item that does not hold the comment (refused, and only the named
//     item is ever read).

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const signOffCID int64 = 5206838120

// gqlComment is one comment node as the GitHub GraphQL API renders it.
type gqlComment struct {
	ID         string `json:"id"`
	DatabaseID int64  `json:"databaseId"`
	Body       string `json:"body"`
	CreatedAt  string `json:"createdAt"`
	URL        string `json:"url"`
	Author     struct {
		Login      string `json:"login"`
		Typename   string `json:"__typename"`
		DatabaseID int64  `json:"databaseId"`
	} `json:"author"`
}

func mkComment(id int64, login string, uid int64, typ, body string) gqlComment {
	c := gqlComment{ID: fmt.Sprintf("IC_%d", id), DatabaseID: id, Body: body,
		CreatedAt: "2026-08-13T00:00:00Z",
		URL:       fmt.Sprintf("https://github.com/medici-finance/assay/pull/444#issuecomment-%d", id)}
	c.Author.Login, c.Author.Typename, c.Author.DatabaseID = login, typ, uid
	return c
}

// threadServer answers the GitHub comments GraphQL query for the pull requests in prs. A query
// that selects no pageInfo gets the first 100 nodes and nothing else — exactly what GitHub does
// for `comments(first: 100)` — so a read that does not walk sees only the head of the thread.
type threadServer struct {
	prs map[int][]gqlComment
	// issues holds the threads of real issues (not pull requests), served for an issue query.
	issues  map[int][]gqlComment
	mu      sync.Mutex
	numbers []int
	// queries counts the requests by the noun the query read: "issue" or "pullRequest".
	queries map[string]int
}

func (s *threadServer) handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost || r.URL.Path != "/graphql" {
		http.Error(w, `{"message":"not served by this fixture"}`, http.StatusNotFound)
		return
	}
	raw, _ := io.ReadAll(r.Body)
	var req struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	n := 0
	if f, ok := req.Variables["number"].(float64); ok {
		n = int(f)
	}
	s.mu.Lock()
	s.numbers = append(s.numbers, n)
	s.mu.Unlock()
	noun := "pullRequest"
	if !strings.Contains(req.Query, "pullRequest(number:") {
		noun = "issue"
	}
	s.mu.Lock()
	if s.queries == nil {
		s.queries = map[string]int{}
	}
	s.queries[noun]++
	s.mu.Unlock()
	threads := s.prs
	if noun == "issue" {
		threads = s.issues
	}
	thread, ok := threads[n]
	if !ok {
		// No such item of this kind: GitHub resolves null, and the backend reports it.
		_, _ = fmt.Fprintf(w, `{"data":{"repository":{%q:null}}}`, noun)
		return
	}
	start := 0
	if a, ok := req.Variables["after"].(string); ok && a != "" {
		start, _ = strconv.Atoi(strings.TrimPrefix(a, "cur-"))
	}
	end := start + 100
	if end > len(thread) {
		end = len(thread)
	}
	conn := map[string]any{"nodes": thread[start:end]}
	if strings.Contains(req.Query, "pageInfo") {
		conn["pageInfo"] = map[string]any{"hasNextPage": end < len(thread), "endCursor": fmt.Sprintf("cur-%d", end)}
	}
	out := map[string]any{"data": map[string]any{"repository": map[string]any{
		noun: map[string]any{"comments": conn}}}}
	_ = json.NewEncoder(w).Encode(out)
}

func (s *threadServer) asked(noun string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.queries[noun]
}

func (s *threadServer) read() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]int(nil), s.numbers...)
}

// signOffEnv installs the production read path against a thread server holding prs.
func signOffEnv(t *testing.T, prs map[int][]gqlComment) *threadServer {
	t.Helper()
	return threadsEnv(t, &threadServer{prs: prs})
}

// threadsEnv installs the production read path against ts.
func threadsEnv(t *testing.T, ts *threadServer) *threadServer {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(ts.handler))
	t.Cleanup(srv.Close)
	readCustodyEnv(t, srv)
	mintTokenFn = func(string, string) (string, string, error) { return mintedReadToken, "", nil }
	return ts
}

func others(n int, from int64) []gqlComment {
	out := make([]gqlComment, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, mkComment(from+int64(i), "someone", 42, "User", "noise"))
	}
	return out
}

func signOffNode() gqlComment {
	return mkComment(signOffCID, blessLogin, blessID, "User", "accepted")
}

// TestSignOffReadAmongOthers — the comment returned is the one the permalink's id names, not
// the first comment on the thread nor the roster human's other comment.
func TestSignOffReadAmongOthers(t *testing.T) {
	thread := []gqlComment{
		mkComment(111, blessLogin, blessID, "User", "an earlier remark, not the sign-off"),
		mkComment(112, "someone", 42, "User", "noise"),
		signOffNode(),
		mkComment(113, blessLogin, blessID, "User", "a later remark, not the sign-off"),
	}
	signOffEnv(t, map[int][]gqlComment{444: thread})
	c, err := fetchComment(signOffURL)
	if err != nil {
		t.Fatalf("sign-off read: %v", err)
	}
	if c.ID != signOffCID || c.Body != "accepted" {
		t.Fatalf("read returned comment %d (%q), want %d — the lookup must match the permalink's id", c.ID, c.Body, signOffCID)
	}
	if err := verifyHumanAuthor(c, "sign-off"); err != nil {
		t.Fatalf("the roster human's sign-off was refused: %v", err)
	}
}

// TestSignOffReadPastFirst100 — F1: a sign-off that is comment 151 on a pull-request thread is
// found. A read that stops at the first page reports it as absent ("deleted"), which is a
// truncated read presented as a definitive one.
func TestSignOffReadPastFirst100(t *testing.T) {
	// An /issues/N permalink naming a pull request resolves no issue and falls back to the
	// change read, so it must walk the same way.
	issuesURL := strings.Replace(signOffURL, "/pull/", "/issues/", 1)
	for _, link := range []string{signOffURL, issuesURL} {
		for _, pos := range []int{100, 150} {
			kind := "pull"
			if link == issuesURL {
				kind = "issues"
			}
			t.Run(fmt.Sprintf("%s position %d", kind, pos+1), func(t *testing.T) {
				thread := append(others(pos, 1000), signOffNode())
				signOffEnv(t, map[int][]gqlComment{444: thread})
				c, err := fetchComment(link)
				if err != nil {
					t.Fatalf("sign-off at comment %d: exit %d: %v", pos+1, deskkit.ExitCodeOf(err), err)
				}
				if c.ID != signOffCID {
					t.Fatalf("read returned comment %d, want %d", c.ID, signOffCID)
				}
			})
		}
	}
}

// TestSignOffReadAbsentID — an id on no comment of a fully read thread is refused (exit 5).
func TestSignOffReadAbsentID(t *testing.T) {
	for _, n := range []int{3, 150} {
		t.Run(fmt.Sprintf("%d comments", n), func(t *testing.T) {
			signOffEnv(t, map[int][]gqlComment{444: others(n, 1000)})
			c, err := fetchComment(signOffURL)
			if err == nil {
				t.Fatalf("an absent id authorized comment %d (%q)", c.ID, c.Body)
			}
			if code := deskkit.ExitCodeOf(err); code != deskkit.ExitRefused {
				t.Fatalf("absent id: exit %d, want refused (%d): %v", code, deskkit.ExitRefused, err)
			}
		})
	}
}

// TestSignOffReadWrongItem — the sign-off exists, but on #445; the permalink names #444. Only
// the named item is read, and the comment is refused as not on it.
func TestSignOffReadWrongItem(t *testing.T) {
	ts := signOffEnv(t, map[int][]gqlComment{
		444: others(3, 1000),
		445: {signOffNode()},
	})
	c, err := fetchComment(signOffURL)
	if err == nil {
		t.Fatalf("a comment on #445 authorized through a permalink naming #444: %d", c.ID)
	}
	if code := deskkit.ExitCodeOf(err); code != deskkit.ExitRefused {
		t.Fatalf("wrong item: exit %d, want refused (%d): %v", code, deskkit.ExitRefused, err)
	}
	for _, n := range ts.read() {
		if n != 444 {
			t.Errorf("the read asked for #%d — only the permalink's item (#444) may be read", n)
		}
	}
}

// TestSignOffReadFromIssueThread — A3: a sign-off on a REAL issue thread (an /issues/N link whose
// number is an issue, not a pull request) is read through the issue query and found, at the head
// of the thread and past the first 100 comments. No pull-request query is sent for it, so a
// kind selection that read every permalink as a pull request would get "no such pull request"
// here and fail closed on a sign-off that is genuinely there.
func TestSignOffReadFromIssueThread(t *testing.T) {
	issueURL := strings.Replace(signOffURL, "/pull/444", "/issues/450", 1)
	for _, pos := range []int{0, 150} {
		t.Run(fmt.Sprintf("position %d", pos+1), func(t *testing.T) {
			thread := append(others(pos, 1000), signOffNode())
			ts := threadsEnv(t, &threadServer{issues: map[int][]gqlComment{450: thread}})
			c, err := fetchComment(issueURL)
			if err != nil {
				t.Fatalf("issue-thread sign-off at comment %d: exit %d: %v", pos+1, deskkit.ExitCodeOf(err), err)
			}
			if c.ID != signOffCID {
				t.Fatalf("read returned comment %d, want %d", c.ID, signOffCID)
			}
			if n := ts.asked("pullRequest"); n != 0 {
				t.Errorf("%d pull-request quer(ies) sent for an /issues/ link naming a real issue — the issue read must come first", n)
			}
		})
	}
}

// TestSignOffAbsentOnIssueThread — SEC-A2: an /issues/N permalink whose number IS a real issue,
// read in full, with no comment carrying the permalink's id, is REFUSED (exit 5) on the issue
// read alone. The fall-back to a change read is reserved for an issue read that could not check
// (the number names a pull request); a refusal from the issue read is the answer and is never
// retried as a change, which would turn "this sign-off is not on its item" into a could-not-check.
//
// FAIL-FIRST: with fetchComment's retry condition widened from deskkit.IsUnverifiable(err) to
// err != nil, the read retries as a change, sends a pull-request query, and exits 6.
func TestSignOffAbsentOnIssueThread(t *testing.T) {
	issueURL := strings.Replace(signOffURL, "/pull/444", "/issues/450", 1)
	ts := threadsEnv(t, &threadServer{issues: map[int][]gqlComment{450: others(3, 1000)}})
	c, err := fetchComment(issueURL)
	if err == nil {
		t.Fatalf("an id absent from the issue thread authorized comment %d (%q)", c.ID, c.Body)
	}
	if code := deskkit.ExitCodeOf(err); code != deskkit.ExitRefused {
		t.Fatalf("absent id on a real issue: exit %d, want refused (%d): %v", code, deskkit.ExitRefused, err)
	}
	if n := ts.asked("pullRequest"); n != 0 {
		t.Errorf("%d pull-request quer(ies) sent after the issue read refused — a refusal is never retried as a change", n)
	}
}

// TestSignOffAuthorIDAbsent — SEC-A2: the sign-off read returns the roster human's login and
// type User but NO numeric author id. The author check is the strict one (login AND id, both
// present), so a read that dropped the id is refused rather than admitted on the login alone.
//
// FAIL-FIRST: with verifyHumanAuthor calling deskkit.IsBlessAuthorityID (the id==0 compatibility
// form) instead of IsBlessAuthorityIDStrict, the comment is accepted.
func TestSignOffAuthorIDAbsent(t *testing.T) {
	signOffEnv(t, map[int][]gqlComment{444: {mkComment(signOffCID, blessLogin, 0, "User", "accepted")}})
	c, err := fetchComment(signOffURL)
	if err != nil {
		t.Fatalf("sign-off read: %v", err)
	}
	if c.ID != signOffCID {
		t.Fatalf("read returned comment %d, want %d", c.ID, signOffCID)
	}
	err = verifyHumanAuthor(c, "sign-off")
	if err == nil {
		t.Fatalf("a sign-off with the roster login and no author id was accepted on the login alone")
	}
	if code := deskkit.ExitCodeOf(err); code != deskkit.ExitRefused {
		t.Fatalf("absent author id: exit %d, want refused (%d): %v", code, deskkit.ExitRefused, err)
	}
}
