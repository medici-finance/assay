package main

// statusgen reconcile on a GitLab-hosted repository (#7252): the merge requests,
// their files and the open issues' labels come through the stub deskread, and
// the GitHub REST client — which answers 404 for a GitLab project, the
// reported symptom — is never asked.

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// glrcWorld is the stub forge for reconcile: changes is the `changes --state
// all` list (raw JSON), and every listed MR's record and delivery files.
func glrcWorld(changes string, incomplete bool, mrs ...int) *glWorld {
	w := &glWorld{answers: map[string]string{}, codes: map[string]int{}}
	w.set(fmt.Sprintf(`{"schema":%d,"kind":"changes","repos":[{"repo":%q,"changes":%s,"incomplete":%t,"pageCap":10}],"partial":[]}`,
		deskreadSchema, glrProject, changes, incomplete), "changes", "--repo", glrProject, "--state", "all")
	w.set(fmt.Sprintf(`{"schema":%d,"kind":"issue-list","repos":[{"repo":%q,"issues":[],"incomplete":false,"pageCap":10}],"partial":[]}`,
		deskreadSchema, glrProject), "issue-list", "--repo", glrProject, "--state", "open")
	for _, n := range mrs {
		target := fmt.Sprintf("%s#%d", glrProject, n)
		w.set(glrItem("change", n, fmt.Sprintf(`"change":{"state":"merged","draft":false,"merged":true,"mergedAt":%q,"mergeCommitSha":%q,"headSha":%q,"baseRef":"main","changedFiles":1,"body":"Brief: dg/03","author":{"login":"example-author"}}`,
			glMergedAt, glMergeSHA, afHeadSHA)), "change", "--change", target)
		w.set(glrItem("change-files", n, `"changeFiles":{"complete":true,"changedFiles":1,"files":[{"filename":"internal/dg/dg.go","status":"modified","patchAbsent":true}]}`),
			"change-files", "--change", target)
	}
	return w
}

func glrcMR(n int, state, body string) string {
	mergedAt := ""
	if state == "MERGED" {
		mergedAt = glMergedAt
	}
	return fmt.Sprintf(`{"number":%d,"state":%q,"headSha":%q,"headRef":"feat/dg","title":"t","body":%s,"mergedAt":%q,"updatedAt":%q,"baseRef":"main"}`,
		n, state, afHeadSHA, glJSON(body), mergedAt, glMergedAt)
}

// glrcGitHub is the GitHub REST API as it answers for a GitLab project: 404 on
// every path. A GitLab reconcile that still read it fails here.
func glrcGitHub(t *testing.T) *int {
	t.Helper()
	hits := new(int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*hits++
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"Not Found"}`)
	}))
	t.Cleanup(srv.Close)
	prev := reconcileGHClient
	reconcileGHClient = func(tok string) *ghClient { return &ghClient{doer: srv.Client(), base: srv.URL, token: tok} }
	t.Cleanup(func() { reconcileGHClient = prev })
	t.Setenv("GITHUB_TOKEN", "test-github")
	foldFlipSeam(t, &fakeFlipSource{})
	return hits
}

// regression: #7252
func TestGLReconcileMergedMR(t *testing.T) {
	glrRoster(t, "gitlab")
	hits := glrcGitHub(t)
	glrcWorld("["+glrcMR(30, "MERGED", "Delivers the gate.\n\nBrief: dg/03\n")+"]", false, 30).install(t)
	root := copyDesignGateFixture(t)

	code, stdout := runReconcileCaptured(t, "--apply", "--root", root, "--repo", glrProject, "--json")
	if code != reconcileOK {
		t.Fatalf("exit %d, want 0:\n%s", code, stdout)
	}
	if got := appliedIDs(t, stdout); len(got) != 1 || !strings.HasPrefix(got[0], "dg/03<PR #30") {
		t.Fatalf("want dg/03 witnessed by merged MR !30, got %v", got)
	}
	if !strings.Contains(readFixtureReadme(t, root), "(./brief-03-model-scoped.md) | 0 | S | implemented |") {
		t.Fatalf("dg/03 was not written implemented:\n%s", readFixtureReadme(t, root))
	}
	if *hits != 0 {
		t.Fatalf("a GitLab reconcile made %d GitHub API read(s)", *hits)
	}
}

// regression: #7252
func TestGLReconcileUnmergedMR(t *testing.T) {
	for name, changes := range map[string]string{
		"open":   "[" + glrcMR(30, "OPEN", "Brief: dg/03\n") + "]",
		"closed": "[" + glrcMR(30, "CLOSED", "Brief: dg/03\n") + "]",
		"absent": "[]",
	} {
		t.Run(name, func(t *testing.T) {
			glrRoster(t, "gitlab")
			hits := glrcGitHub(t)
			w := glrcWorld(changes, false)
			if name == "open" {
				w = glrcWorld(changes, false, 30)
			}
			w.install(t)
			root := copyDesignGateFixture(t)
			before := readFixtureReadme(t, root)
			code, stdout := runReconcileCaptured(t, "--apply", "--root", root, "--repo", glrProject, "--json")
			if code != reconcileOK {
				t.Fatalf("exit %d, want 0:\n%s", code, stdout)
			}
			if got := appliedIDs(t, stdout); len(got) != 0 {
				t.Fatalf("an unmerged or absent MR flipped a row: %v", got)
			}
			if readFixtureReadme(t, root) != before || *hits != 0 {
				t.Fatalf("README changed or GitHub was read (%d hits)", *hits)
			}
		})
	}
}

// An incomplete or unreadable MR list is could-not-check, never a short list.
// regression: #7252
func TestGLReconcileUnreadList(t *testing.T) {
	for name, w := range map[string]*glWorld{
		"incomplete": glrcWorld("["+glrcMR(30, "MERGED", "Brief: dg/03\n")+"]", true, 30),
		"unreadable": glrcWorld("null", false),
	} {
		t.Run(name, func(t *testing.T) {
			glrRoster(t, "gitlab")
			glrcGitHub(t)
			w.install(t)
			root := copyDesignGateFixture(t)
			before := readFixtureReadme(t, root)
			code, stdout := runReconcileCaptured(t, "--apply", "--root", root, "--repo", glrProject)
			if code != reconcileCouldNotCheck {
				t.Fatalf("exit %d, want %d (could-not-check):\n%s", code, reconcileCouldNotCheck, stdout)
			}
			if readFixtureReadme(t, root) != before {
				t.Fatal("an unread MR list must write nothing")
			}
		})
	}
}
