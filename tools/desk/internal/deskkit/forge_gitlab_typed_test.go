package deskkit

// forge_gitlab_typed_test.go — the TYPED read/write pair (GetIssueTyped / PostCommentTyped) on
// both backends, driven by the same recorded servers the golden corpora use. The property under
// test is the routing, not the payload: on GitLab a number that carries BOTH an issue and a
// merge request is the ordinary case, and the typed operations must touch exactly the one
// endpoint the stated kind names — the bare GetIssue keeps refusing that same number. On GitHub
// (one number sequence) the typed read validates the kind and the typed write posts to the one
// shared comments endpoint. Named TestForgeGitlab* so the GitLab mutation spec's -run filter
// covers the GitLab half.

import (
	"strings"
	"testing"
)

// glBothKinds seeds a project in which #7 is an open issue AND !7 is an open merge request.
func glBothKinds(t *testing.T) *glServer {
	t.Helper()
	s := newGLServer(t)
	s.issue = glIssue(map[string]any{"iid": 7, "title": "the issue", "state": "opened",
		"web_url": "https://gitlab.example/medici-finance/assay/-/issues/7"})
	s.mr = glMR(map[string]any{"iid": 7, "title": "the change", "state": "opened",
		"web_url": "https://gitlab.example/medici-finance/assay/-/merge_requests/7"})
	return s
}

// paths renders the recorded requests as "METHOD path" lines for assertions.
func glPaths(s *glServer) []string {
	out := make([]string, 0, len(s.requests))
	for _, r := range s.requests {
		out = append(out, r.Method+" "+r.Path)
	}
	return out
}

func TestForgeGitlabTypedReadRoutesOnKind(t *testing.T) {
	cases := []struct {
		kind     TargetKind
		wantPath string
		wantPR   bool
		wantURL  string
		notPath  string
	}{
		{TargetIssue, "GET /api/v4/projects/medici-finance%2Fassay/issues/7", false,
			"https://gitlab.example/medici-finance/assay/-/issues/7", "/merge_requests/"},
		{TargetChange, "GET /api/v4/projects/medici-finance%2Fassay/merge_requests/7", true,
			"https://gitlab.example/medici-finance/assay/-/merge_requests/7", "/issues/"},
	}
	for _, c := range cases {
		t.Run(string(c.kind), func(t *testing.T) {
			s := glBothKinds(t)
			iss, err := s.forge().GetIssueTyped(glRepo, 7, c.kind)
			if err != nil {
				t.Fatalf("GetIssueTyped(%s): %v", c.kind, err)
			}
			if iss.IsPullRequest != c.wantPR || iss.URL != c.wantURL || iss.State != "open" || iss.Number != 7 {
				t.Fatalf("GetIssueTyped(%s) = %+v, want pr=%v url=%s state=open number=7", c.kind, iss, c.wantPR, c.wantURL)
			}
			got := glPaths(s)
			if len(got) != 1 || got[0] != c.wantPath {
				t.Fatalf("GetIssueTyped(%s) must probe exactly the %s endpoint; requests=%v", c.kind, c.kind, got)
			}
			if strings.Contains(got[0], c.notPath) {
				t.Fatalf("GetIssueTyped(%s) touched the other kind: %v", c.kind, got)
			}
		})
	}
}

func TestForgeGitlabTypedReadMissingKindIs404(t *testing.T) {
	s := glBothKinds(t)
	s.issueMissing = true
	if _, err := s.forge().GetIssueTyped(glRepo, 7, TargetIssue); !IsForgeNotFound(err) {
		t.Fatalf("issue absent (merge request present): want a 404 for the ISSUE kind, got %v", err)
	}
	s2 := glBothKinds(t)
	s2.mrMissing = true
	if _, err := s2.forge().GetIssueTyped(glRepo, 7, TargetChange); !IsForgeNotFound(err) {
		t.Fatalf("merge request absent (issue present): want a 404 for the CHANGE kind, got %v", err)
	}
}

func TestForgeGitlabBareReadStillRefusesBothKinds(t *testing.T) {
	s := glBothKinds(t)
	_, err := s.forge().GetIssue(glRepo, 7)
	if err == nil || !strings.Contains(err.Error(), "BOTH issue #7 and merge request !7") {
		t.Fatalf("bare GetIssue on a both-kinds number must still refuse naming both; got %v", err)
	}
}

func TestForgeGitlabTypedWriteRoutesOnKind(t *testing.T) {
	cases := []struct {
		kind     TargetKind
		wantPath string
		wantID   string // opaque editable id: set for a merge-request note, empty for an issue note
	}{
		{TargetIssue, "POST /api/v4/projects/medici-finance%2Fassay/issues/7/notes", ""},
		{TargetChange, "POST /api/v4/projects/medici-finance%2Fassay/merge_requests/7/notes", gitlabNoteID(glRepo, 7, 900)},
	}
	for _, c := range cases {
		t.Run(string(c.kind), func(t *testing.T) {
			s := glBothKinds(t)
			ref, err := s.forge().PostCommentTyped(glRepo, 7, c.kind, "an observation")
			if err != nil {
				t.Fatalf("PostCommentTyped(%s): %v", c.kind, err)
			}
			got := glPaths(s)
			if len(got) != 1 || got[0] != c.wantPath {
				t.Fatalf("PostCommentTyped(%s) must POST exactly to the %s notes endpoint with NO resolving read; requests=%v", c.kind, c.kind, got)
			}
			if string(s.requests[0].Body) != `{"body":"an observation"}` {
				t.Fatalf("note body = %s", s.requests[0].Body)
			}
			if ref.ID != c.wantID || ref.DatabaseID == 0 {
				t.Fatalf("PostCommentTyped(%s) ref = %+v, want ID=%q and a numeric id", c.kind, ref, c.wantID)
			}
		})
	}
}

func TestForgeGitlabTypedUnknownKindRefused(t *testing.T) {
	s := glBothKinds(t)
	if _, err := s.forge().GetIssueTyped(glRepo, 7, TargetKind("")); err == nil || len(s.requests) != 0 {
		t.Fatalf("empty kind must be refused before any request; err=%v requests=%v", err, glPaths(s))
	}
	if _, err := s.forge().PostCommentTyped(glRepo, 7, TargetKind("bogus"), "x"); err == nil || len(s.requests) != 0 {
		t.Fatalf("unknown kind must be refused before any request; err=%v requests=%v", err, glPaths(s))
	}
}

func TestParseTargetKind(t *testing.T) {
	for in, want := range map[string]TargetKind{"issue": TargetIssue, "ISSUE": TargetIssue,
		"mr": TargetChange, "pr": TargetChange, "PR": TargetChange, "change": TargetChange, " mr ": TargetChange} {
		got, err := ParseTargetKind(in)
		if err != nil || got != want {
			t.Fatalf("ParseTargetKind(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "issues", "merge", "x"} {
		if _, err := ParseTargetKind(in); err == nil {
			t.Fatalf("ParseTargetKind(%q) must refuse", in)
		} else if de, ok := err.(*DeskError); !ok || de.Code != ExitRefused {
			t.Fatalf("ParseTargetKind(%q) must refuse with ExitRefused, got %v", in, err)
		}
	}
}

// --- GitHub: one number sequence, so the kind is validated, never routed on ---

func ghIssueWireFor(number int, pr bool) map[string]any {
	m := map[string]any{"number": number, "title": "t", "state": "open",
		"user":     map[string]any{"login": "someone", "id": 5},
		"html_url": "https://github.com/medici-finance/assay/issues/7"}
	if pr {
		m["pull_request"] = map[string]any{"url": "https://api.github.com/repos/medici-finance/assay/pulls/7"}
	}
	return m
}

func TestForgeGithubTypedReadValidatesKind(t *testing.T) {
	s := newGoldenServer(t)
	s.issue = ghIssueWireFor(7, false)
	iss, err := s.forge().GetIssueTyped(forgeTestRepo, 7, TargetIssue)
	if err != nil || iss.IsPullRequest {
		t.Fatalf("issue read as issue: %+v, %v", iss, err)
	}
	if len(s.requests) != 1 || s.requests[0].Path != "/repos/medici-finance/assay/issues/7" {
		t.Fatalf("GitHub typed read is the ONE issues read, unchanged: %+v", s.requests)
	}
	if _, err := s.forge().GetIssueTyped(forgeTestRepo, 7, TargetChange); err == nil || !strings.Contains(err.Error(), "is an issue, not a pull request") {
		t.Fatalf("issue read as change must be could-not-check naming the mismatch; got %v", err)
	}

	s2 := newGoldenServer(t)
	s2.issue = ghIssueWireFor(7, true)
	if _, err := s2.forge().GetIssueTyped(forgeTestRepo, 7, TargetIssue); err == nil || !strings.Contains(err.Error(), "is a pull request, not an issue") {
		t.Fatalf("pull request read as issue must be could-not-check naming the mismatch; got %v", err)
	}
	iss, err = s2.forge().GetIssueTyped(forgeTestRepo, 7, TargetChange)
	if err != nil || !iss.IsPullRequest {
		t.Fatalf("pull request read as change: %+v, %v", iss, err)
	}
}

func TestForgeGithubTypedWriteSharesCommentsEndpoint(t *testing.T) {
	for _, kind := range []TargetKind{TargetIssue, TargetChange} {
		s := newGoldenServer(t)
		ref, err := s.forge().PostCommentTyped(forgeTestRepo, 7, kind, "an observation")
		if err != nil || ref.URL == "" {
			t.Fatalf("PostCommentTyped(%s): %+v, %v", kind, ref, err)
		}
		if len(s.requests) != 1 || s.requests[0].Method != "POST" || s.requests[0].Path != "/repos/medici-finance/assay/issues/7/comments" {
			t.Fatalf("PostCommentTyped(%s) must be the one comments POST, no resolving read: %+v", kind, s.requests)
		}
	}
}
