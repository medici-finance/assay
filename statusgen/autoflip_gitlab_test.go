package main

// autoflip_gitlab_test.go — the GitLab auto-flip's live read through `deskread` (#7202). Each
// case runs the PRODUCTION source (liveModelFlipSource(forgeGitLab, "")) end to end through
// autoFlipModel in dry-run, against a stub `deskread` on PATH that answers each argv from a
// fixture file. Only the brief file's git history is substituted.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	glRepo     = "example-org/tracker"
	glMergeSHA = "bbbb000000000000000000000000000000000001"
	glMidSHA   = "cccc000000000000000000000000000000000001"
	glMergedAt = "2026-09-10T00:00:00Z"
)

// glLiveSource is the production GitLab source with only CommitsTouching replaced: the fixture
// tree's git history does not carry the commit SHAs the stub forge answers for.
type glLiveSource struct {
	modelFlipSource
	commits []string
}

func (g glLiveSource) CommitsTouching(root, relPath string, limit int) ([]string, error) {
	if filepath.Base(relPath) == "brief-01-model-approved-at-head.md" {
		return g.commits, nil
	}
	return nil, nil
}

// glWorld is the stub forge: argv key -> envelope (and optional exit code).
type glWorld struct {
	answers map[string]string
	codes   map[string]int
}

func glArgKey(args ...string) string {
	var b strings.Builder
	for _, a := range args {
		b.WriteString(a)
		b.WriteByte('_')
	}
	return strings.Map(func(r rune) rune {
		if r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, b.String())
}

func glItem(kind string, n int, payload string) string {
	return fmt.Sprintf(`{"schema":%d,"kind":%q,"items":[{"repo":%q,"number":%d,%s}],"partial":[]}`, deskreadSchema, kind, glRepo, n, payload)
}

func glJSON(v string) string { return fmt.Sprintf("%q", v) }

// newGLWorld is MR !101 merged at afHeadSHA, delivering af/01, approved by the reviewer, with
// the reviewer's note pinning the head: the one shape that flips. Cases mutate it.
func newGLWorld() *glWorld {
	w := &glWorld{answers: map[string]string{}, codes: map[string]int{}}
	w.set(fmt.Sprintf(`{"schema":%d,"kind":"commit-changes","repos":[{"repo":%q,"commitChanges":[101]}],"partial":[]}`, deskreadSchema, glRepo),
		"commit-changes", "--repo", glRepo, "--sha", afHeadSHA)
	w.change(101, true, afHeadSHA, "Brief: af/01")
	w.set(glItem("change-files", 101, `"changeFiles":{"complete":true,"changedFiles":1,"files":[{"filename":"internal/example/example.go","patch":"@@ -1 +1 @@","patchAbsent":false}]}`),
		"change-files", "--change", glRepo+"#101")
	w.set(fmt.Sprintf(`{"schema":%d,"kind":"default-branch","repos":[{"repo":%q,"defaultBranch":"main"}],"partial":[]}`, deskreadSchema, glRepo),
		"default-branch", "--repo", glRepo)
	w.reviews(`{"authorLogin":` + glJSON(glReviewer) + `,"state":"APPROVED"}`)
	w.notes(glNote{Author: glReviewer, Body: "Verdict: approve at " + afHeadSHA})
	w.merged(`[]`, false)
	return w
}

func (w *glWorld) set(answer string, args ...string) { w.answers[glArgKey(args...)] = answer }

func (w *glWorld) change(n int, merged bool, head, body string) {
	mergedAt := ""
	if merged {
		mergedAt = glMergedAt
	}
	w.set(glItem("change", n, fmt.Sprintf(`"change":{"state":%q,"merged":%t,"mergedAt":%q,"mergeCommitSha":%q,"headSha":%q,"baseRef":"main","changedFiles":1,"body":%s,"author":{"login":"example-author"}}`,
		map[bool]string{true: "merged", false: "opened"}[merged], merged, mergedAt, glMergeSHA, head, glJSON(body))),
		"change", "--change", fmt.Sprintf("%s#%d", glRepo, n))
}

func (w *glWorld) reviews(entries ...string) {
	w.set(glItem("reviews", 101, `"reviews":[`+strings.Join(entries, ",")+`]`), "reviews", "--change", glRepo+"#101")
}

func (w *glWorld) notes(ns ...glNote) {
	var parts []string
	for _, n := range ns {
		parts = append(parts, fmt.Sprintf(`{"authorLogin":%q,"authorId":7,"createdAt":"2026-09-09T00:00:00Z","body":%s}`, n.Author, glJSON(n.Body)))
	}
	w.set(glItem("comments", 101, `"comments":[`+strings.Join(parts, ",")+`]`), "comments", "--change", glRepo+"#101")
}

func (w *glWorld) merged(changes string, incomplete bool) {
	w.set(fmt.Sprintf(`{"schema":%d,"kind":"changes","repos":[{"repo":%q,"changes":%s,"incomplete":%t,"pageCap":10}],"partial":[]}`, deskreadSchema, glRepo, changes, incomplete),
		"changes", "--repo", glRepo, "--state", "merged")
}

// install writes the stub `deskread` and its answers, and puts it first on PATH.
func (w *glWorld) install(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	for k, v := range w.answers {
		if err := os.WriteFile(filepath.Join(dir, k+".json"), []byte(v), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for k, c := range w.codes {
		if err := os.WriteFile(filepath.Join(dir, k+".code"), []byte(fmt.Sprint(c)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeStubBin(t, dir, "deskread", "#!/bin/sh\n"+
		"k=$(printf '%s_' \"$@\" | tr -c 'A-Za-z0-9_' '_')\n"+
		"f=\""+dir+"/$k.json\"\n"+
		"[ -f \"$f\" ] || { echo \"stub deskread: no answer for $*\" >&2; exit 7; }\n"+
		"cat \"$f\"\n"+
		"[ -f \""+dir+"/$k.code\" ] && exit $(cat \""+dir+"/$k.code\")\n"+
		"exit 0\n")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// runGL runs the dry-run auto-flip on the af fixture with the live GitLab source and returns
// af/01's result; the README must be untouched (dry run).
func runGL(t *testing.T, w *glWorld, commits ...string) modelFlipResult {
	t.Helper()
	w.install(t)
	switch {
	case commits == nil:
		commits = []string{afHeadSHA}
	case len(commits) == 1 && commits[0] == "": // no history: only the trailer resolver can find the MR
		commits = nil
	}
	root, streams := loadAFStreams(t)
	before := afReadme(t, root)
	src := glLiveSource{modelFlipSource: liveModelFlipSource(forgeGitLab, ""), commits: commits}
	results, err := autoFlipModel(root, streams, src, glReviewerIdent(), afNow, true)
	if err != nil {
		t.Fatalf("autoFlipModel: %v", err)
	}
	if afReadme(t, root) != before {
		t.Fatal("a dry run rewrote the README")
	}
	got := afResult(t, results, "af/01")
	t.Logf("af/01: %v PR=%d (%s)", got.Outcome, got.PR, got.Reason)
	return got
}

func wantGL(t *testing.T, got modelFlipResult, want flipOutcome, reason string) {
	t.Helper()
	if got.Outcome != want || !strings.Contains(got.Reason, reason) {
		t.Fatalf("af/01 = %v (%s), want %v with a reason naming %q", got.Outcome, got.Reason, want, reason)
	}
}

// regression: #7202
func TestGitLabLiveFlipAtHead(t *testing.T) {
	got := runGL(t, newGLWorld())
	if got.Outcome != flipDone || got.PR != 101 || got.SHA != afHeadSHA {
		t.Fatalf("af/01 = %v PR=%d SHA=%s (%s), want FLIPPED on MR !101 at the merged head", got.Outcome, got.PR, got.SHA, got.Reason)
	}
}

// regression: #7202
func TestGitLabLiveRefusals(t *testing.T) {
	approval := func(login string) string { return `{"authorLogin":` + glJSON(login) + `,"state":"APPROVED"}` }
	for _, c := range []struct {
		name   string
		mutate func(w *glWorld)
		reason string
	}{
		{"no approval", func(w *glWorld) { w.reviews() }, "no approval"},
		{"approval but no note", func(w *glWorld) { w.notes() }, "no note"},
		{"note pins a stale sha", func(w *glWorld) {
			w.notes(glNote{Author: glReviewer, Body: "Verdict: approve at " + afStaleSHA})
		}, "no note"},
		{"pinning note by another login", func(w *glWorld) {
			w.notes(glNote{Author: "someone-else", Body: "Verdict: approve at " + afHeadSHA})
		}, "no note"},
		{"approval by another login", func(w *glWorld) { w.reviews(approval("someone-else")) }, "no approval"},
		// GitLab's reviews read renders a "Verdict: approve" note APPROVED too; with a body it
		// is a note, never the approval.
		{"verdict note is not an approval", func(w *glWorld) {
			w.reviews(`{"authorLogin":` + glJSON(glReviewer) + `,"state":"APPROVED","body":"Verdict: approve at ` + afHeadSHA + `"}`)
		}, "no approval"},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := newGLWorld()
			c.mutate(w)
			wantGL(t, runGL(t, w), flipRefused, c.reason)
		})
	}
}

// regression: #7202
func TestGitLabLiveCouldNotCheck(t *testing.T) {
	partial := func(kind string) string {
		return fmt.Sprintf(`{"schema":%d,"kind":%q,"items":[],"partial":[{"repo":%q,"number":101,"reason":"forge 503"}]}`, deskreadSchema, kind, glRepo)
	}
	for _, c := range []struct {
		name    string
		mutate  func(w *glWorld)
		commits []string
		reason  string
	}{
		{"unmerged MR", func(w *glWorld) { w.change(101, false, afHeadSHA, "Brief: af/01") }, nil, "merged"},
		{"no MR for the commit", func(w *glWorld) {
			w.set(fmt.Sprintf(`{"schema":%d,"kind":"commit-changes","repos":[{"repo":%q,"commitChanges":[]}],"partial":[]}`, deskreadSchema, glRepo),
				"commit-changes", "--repo", glRepo, "--sha", afHeadSHA)
		}, nil, "merged"},
		{"commit read unrunnable", func(w *glWorld) {
			delete(w.answers, glArgKey("commit-changes", "--repo", glRepo, "--sha", afHeadSHA))
		}, nil, "could-not-check"},
		{"reviews partial", func(w *glWorld) {
			w.set(partial("reviews"), "reviews", "--change", glRepo+"#101")
			w.codes[glArgKey("reviews", "--change", glRepo+"#101")] = 6
		}, nil, "forge 503"},
		{"notes partial", func(w *glWorld) {
			w.set(partial("comments"), "comments", "--change", glRepo+"#101")
			w.codes[glArgKey("comments", "--change", glRepo+"#101")] = 6
		}, nil, "forge 503"},
		{"truncated file list", func(w *glWorld) {
			w.set(glItem("change-files", 101, `"changeFiles":{"complete":false,"changedFiles":3,"files":[{"filename":"internal/example/example.go","patchAbsent":true}]}`),
				"change-files", "--change", glRepo+"#101")
		}, nil, "truncated"},
		{"intermediate commit list incomplete", func(w *glWorld) {
			w.set(fmt.Sprintf(`{"schema":%d,"kind":"commit-changes","repos":[{"repo":%q,"commitChanges":[101]}],"partial":[]}`, deskreadSchema, glRepo),
				"commit-changes", "--repo", glRepo, "--sha", glMidSHA)
			w.set(glItem("change-commits", 101, `"changeCommits":{"complete":false,"shas":["`+glMidSHA+`"]}`),
				"change-commits", "--change", glRepo+"#101")
		}, []string{glMidSHA}, "page bound"},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := newGLWorld()
			c.mutate(w)
			wantGL(t, runGL(t, w, c.commits...), flipUnchecked, c.reason)
		})
	}

	// The intermediate-commit path credits a commit the MR introduced (complete list).
	t.Run("intermediate commit introduced", func(t *testing.T) {
		w := newGLWorld()
		w.set(fmt.Sprintf(`{"schema":%d,"kind":"commit-changes","repos":[{"repo":%q,"commitChanges":[101]}],"partial":[]}`, deskreadSchema, glRepo),
			"commit-changes", "--repo", glRepo, "--sha", glMidSHA)
		w.set(glItem("change-commits", 101, `"changeCommits":{"complete":true,"shas":["`+glMidSHA+`","`+afHeadSHA+`"]}`),
			"change-commits", "--change", glRepo+"#101")
		if got := runGL(t, w, glMidSHA); got.Outcome != flipDone {
			t.Fatalf("af/01 = %v (%s), want FLIPPED from an introduced intermediate commit", got.Outcome, got.Reason)
		}
	})
}

// regression: #7202
func TestGitLabLiveTrailerBody(t *testing.T) {
	trailerWorld := func(trust string) *glWorld {
		w := newGLWorld()
		w.merged(`[{"number":101,"state":"merged","body":"Brief: af/01","baseRef":"main","mergedAt":"`+glMergedAt+`"}]`, false)
		w.set(glItem("change-trust", 101, `"trust":`+trust), "change-trust", "--change", glRepo+"#101")
		return w
	}
	t.Run("never edited flips", func(t *testing.T) {
		got := runGL(t, trailerWorld(`{"complete":true,"bodyHistoryKnown":true,"events":[]}`), "")
		if got.Outcome != flipDone {
			t.Fatalf("af/01 = %v (%s), want FLIPPED via the Brief: trailer", got.Outcome, got.Reason)
		}
	})
	for _, c := range []struct{ name, trust, reason string }{
		{"edited after merge", `{"bodyEditedAt":"2026-09-11T00:00:00Z","complete":true,"bodyHistoryKnown":true,"events":[]}`, "edited"},
		{"edit history unknown", `{"complete":true,"bodyHistoryKnown":false,"events":[]}`, "edit"},
		{"activity truncated", `{"complete":false,"bodyHistoryKnown":true,"events":[]}`, "edit"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := runGL(t, trailerWorld(c.trust), "")
			if got.Outcome == flipDone || !strings.Contains(got.Reason, c.reason) {
				t.Fatalf("af/01 = %v (%s), want no flip with a reason naming %q", got.Outcome, got.Reason, c.reason)
			}
		})
	}
	t.Run("truncated merged list is only a note", func(t *testing.T) {
		w := newGLWorld()
		w.merged(`[]`, true)
		if got := runGL(t, w); got.Outcome != flipDone {
			t.Fatalf("af/01 = %v (%s), want the history walk to decide when the trailer search is unread", got.Outcome, got.Reason)
		}
	})
}
