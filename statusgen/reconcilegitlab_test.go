package main

// statusgen reconcile on a GitLab-hosted repository (#7252): the merge requests,
// their files and the open issues' labels come through the stub deskread, and
// the GitHub REST client — which answers 404 for a GitLab project, the
// reported symptom — is never asked.

import (
	"encoding/json"
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
		w.set(glrcChange(n, false), "change", "--change", target)
		w.set(glrcFiles(n, true), "change-files", "--change", target)
	}
	return w
}

// glrcChange is MR n's `deskread change` record: its draft flag and merge commit.
func glrcChange(n int, draft bool) string {
	return glrItem("change", n, fmt.Sprintf(`"change":{"state":"merged","draft":%t,"merged":true,"mergedAt":%q,"mergeCommitSha":%q,"headSha":%q,"baseRef":"main","changedFiles":1,"body":"Brief: dg/03","author":{"login":"example-author"}}`,
		draft, glMergedAt, glMergeSHA, afHeadSHA))
}

// glrcFiles is MR n's `deskread change-files` list, marked complete or not.
func glrcFiles(n int, complete bool) string {
	return glrItem("change-files", n, fmt.Sprintf(`"changeFiles":{"complete":%t,"changedFiles":1,"files":[{"filename":"internal/dg/dg.go","status":"modified","patchAbsent":true}]}`, complete))
}

// glrcChangesKey is the stub key of the `changes --state all` read.
func glrcChangesKey() []string { return []string{"changes", "--repo", glrProject, "--state", "all"} }

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
	// The witness names the merge commit the `change` read reports.
	want := "dg/03<PR #30 (merged " + shortSHA(glMergeSHA) + ")"
	if got := appliedIDs(t, stdout); len(got) != 1 || !strings.HasPrefix(got[0], want) {
		t.Fatalf("want dg/03 witnessed by merged MR !30 as %q, got %v", want, got)
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

// An open draft MR's in-progress witness says draft: the flag comes from the
// `change` read, since the changes list carries none. regression: #7252 (review A1)
func TestGLReconcileDraftMR(t *testing.T) {
	glrRoster(t, "gitlab")
	glrcGitHub(t)
	w := glrcWorld("["+glrcMR(30, "OPEN", "Brief: dg/03\n")+"]", false, 30)
	w.set(glrcChange(30, true), "change", "--change", fmt.Sprintf("%s#%d", glrProject, 30))
	w.install(t)
	code, stdout := runReconcileCaptured(t, "--root", copyDesignGateFixture(t), "--repo", glrProject, "--json")
	if code != reconcileOK {
		t.Fatalf("exit %d, want 0:\n%s", code, stdout)
	}
	var res reconcileResult
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("decoding --json: %v\n%s", err, stdout)
	}
	want := "PR #30 (draft " + shortSHA(afHeadSHA) + ")"
	for _, b := range res.Briefs {
		if b.ID == "dg/03" {
			if b.Cell != "in-progress" || b.Witness != want {
				t.Fatalf("dg/03 = %s %q, want in-progress %q", b.Cell, b.Witness, want)
			}
			return
		}
	}
	t.Fatalf("dg/03 is not in the result:\n%s", stdout)
}

// Every envelope guard on the MR reads is could-not-check, never a short or
// guessed list: a partial for the repo (even beside a full entry), no
// completeness field, a merged MR with no merge time, a state this build does
// not know, a `change` read that fails, a file list not marked complete.
// Each world would flip dg/03 without its guard. regression: #7252 (review A1)
func TestGLReconcileEnvelopeGuards(t *testing.T) {
	merged := "[" + glrcMR(30, "MERGED", "Brief: dg/03\n") + "]"
	entry := func(changes, rest string) string {
		return fmt.Sprintf(`{"schema":%d,"kind":"changes","repos":[{"repo":%q,"changes":%s%s}],"partial":[]}`, deskreadSchema, glrProject, changes, rest)
	}
	cases := map[string]func(w *glWorld){
		"partial": func(w *glWorld) {
			w.set(strings.Replace(entry(merged, `,"incomplete":false,"pageCap":10`), `"partial":[]`,
				fmt.Sprintf(`"partial":[{"repo":%q,"reason":"rate limited"}]`, glrProject), 1), glrcChangesKey()...)
		},
		"no completeness": func(w *glWorld) { w.set(entry(merged, `,"pageCap":10`), glrcChangesKey()...) },
		"merged no time": func(w *glWorld) {
			mr := strings.Replace(glrcMR(30, "MERGED", "Brief: dg/03\n"), `"mergedAt":"`+glMergedAt+`"`, `"mergedAt":""`, 1)
			w.set(entry("["+mr+"]", `,"incomplete":false,"pageCap":10`), glrcChangesKey()...)
		},
		"unknown state": func(w *glWorld) {
			w.set(entry("["+glrcMR(30, "LOCKED", "Brief: dg/03\n")+"]", `,"incomplete":false,"pageCap":10`), glrcChangesKey()...)
		},
		"change unread": func(w *glWorld) {
			delete(w.answers, glArgKey("change", "--change", fmt.Sprintf("%s#%d", glrProject, 30)))
		},
		"files incomplete": func(w *glWorld) {
			w.set(glrcFiles(30, false), "change-files", "--change", fmt.Sprintf("%s#%d", glrProject, 30))
		},
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			glrRoster(t, "gitlab")
			hits := glrcGitHub(t)
			w := glrcWorld(merged, false, 30)
			edit(w)
			w.install(t)
			root := copyDesignGateFixture(t)
			before := readFixtureReadme(t, root)
			code, stdout := runReconcileCaptured(t, "--apply", "--root", root, "--repo", glrProject)
			if code != reconcileCouldNotCheck {
				t.Fatalf("exit %d, want %d (could-not-check):\n%s", code, reconcileCouldNotCheck, stdout)
			}
			if readFixtureReadme(t, root) != before || *hits != 0 {
				t.Fatalf("README changed or GitHub was read (%d hits)", *hits)
			}
		})
	}
}
