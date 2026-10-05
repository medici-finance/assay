package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestStaleFailVsMissingCard(t *testing.T) {
	for _, state := range []string{"stale-fail", "missing-card", "current-fail"} {
		t.Run(state, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS("testdata/"+state)); err != nil {
				t.Fatal(err)
			}
			attrGitInit(t, root)
			runGit(t, root, "checkout", "-b", "main")
			if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "src/check.go"), []byte("fixed"), 0644); err != nil {
				t.Fatal(err)
			}
			runGit(t, root, "add", "docs", "src")
			runGitEnv(t, root, []string{"GIT_AUTHOR_DATE=2026-08-03T12:00:00Z", "GIT_COMMITTER_DATE=2026-08-03T12:00:00Z"}, "commit", "-m", "Repair check")
			sha, err := exec.Command("git", "-C", root, "rev-parse", "--short", "HEAD").Output()
			if err != nil {
				t.Fatal(err)
			}
			streams, _, err := loadStreams(root)
			if err != nil {
				t.Fatal(err)
			}
			_, notices := checkBriefFiles(streams, streams)
			got := strings.Join(notices, "\n")
			want := "no decision-issue — file one via --decision-issues"
			switch state {
			case "stale-fail":
				want = "stale FAIL"
			case "missing-card":
				want = "sign-off card missing"
			}
			if !strings.Contains(got, want) {
				t.Errorf("want %q; got %s", want, got)
			}
			if state != "current-fail" && strings.Contains(got, "no decision-issue") {
				t.Errorf("wrong decision-issue route: %s", got)
			}
			if state == "stale-fail" && !strings.Contains(got, strings.TrimSpace(string(sha))) {
				t.Errorf("newest fix commit missing: %s", got)
			}
			t.Log(got)
		})
	}
}

// TestFailNoticeClass inventories every production Go source: new copies of the
// old routing message must go through the verdict/history-aware choke point.
func TestFailNoticeClass(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]int{}
	// The todo-only Next-up nudge is outside the waiting-brief class.
	allowed := map[string]bool{"stalefail.go:waitingBriefNotice": true, "decisionissues.go:nextUpDecisionNotices": true}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		tree, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range tree.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			ast.Inspect(fn, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				value, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(value, "no decision-issue — file one"+" via --decision-issues") {
					site := file + ":" + fn.Name.Name
					found[site]++
					if !allowed[site] {
						t.Errorf("unclassified waiting-brief route in %s", site)
					}
				}
				return true
			})
		}
	}
	for site := range allowed {
		if found[site] != 1 {
			t.Errorf("want one route at %s, found %d", site, found[site])
		}
	}
}

func TestFailHistoryRoutes(t *testing.T) {
	for _, name := range []string{"model", "linked-card", "fix-issue", "unrelated", "same-day", "head-only", "unavailable", "undated", "newest", "superseded", "quoted-pass"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS("testdata/stale-fail")); err != nil {
				t.Fatal(err)
			}
			brief := filepath.Join(root, "docs/streams/check/brief-01-check.md")
			data, err := os.ReadFile(brief)
			if err != nil {
				t.Fatal(err)
			}
			body := string(data)
			switch name {
			case "model":
				body = strings.Replace(body, "gate: human", "gate: model", 1)
			case "linked-card":
				body = strings.Replace(body, "gate: human", "gate: human\ndecision-issue: 12", 1)
			case "fix-issue":
				body = strings.Replace(body, "files: src/check.go", "files: missing.go\nfacts: fix issue #72", 1)
			case "unrelated":
				body = strings.Replace(body, "files: src/check.go", "files: missing.go", 1)
			case "undated":
				body = strings.ReplaceAll(body, "2026-08-01", "unknown")
			case "superseded":
				body += "\n### Run 2026-08-05\n**VERIFY: PASS**\n"
			case "quoted-pass":
				body += "\n> **VERIFY: PASS** 2026-08-05\n"
			}
			if err := os.WriteFile(brief, []byte(body), 0644); err != nil {
				t.Fatal(err)
			}
			attrGitInit(t, root)
			runGit(t, root, "checkout", "-b", "main")
			if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "src/check.go"), []byte("base"), 0644); err != nil {
				t.Fatal(err)
			}
			runGit(t, root, "add", "docs", "src")
			runGitEnv(t, root, []string{"GIT_AUTHOR_DATE=2026-07-30T12:00:00Z", "GIT_COMMITTER_DATE=2026-07-30T12:00:00Z"}, "commit", "-m", "Base")
			if name == "head-only" {
				runGit(t, root, "checkout", "-b", "feature")
			}
			date := "2026-08-03T12:00:00Z"
			if name == "same-day" {
				date = "2026-08-01T23:59:59Z"
			}
			if err := os.WriteFile(filepath.Join(root, "src/check.go"), []byte("fixed"), 0644); err != nil {
				t.Fatal(err)
			}
			runGit(t, root, "add", "src/check.go")
			runGitEnv(t, root, []string{"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date}, "commit", "-m", "Fix #72")
			if name == "newest" {
				if err := os.WriteFile(filepath.Join(root, "src/check.go"), []byte("newest"), 0644); err != nil {
					t.Fatal(err)
				}
				runGit(t, root, "add", "src/check.go")
				runGitEnv(t, root, []string{"GIT_AUTHOR_DATE=2026-08-04T12:00:00Z", "GIT_COMMITTER_DATE=2026-08-04T12:00:00Z"}, "commit", "-m", "Further repair")
			}
			sha, err := exec.Command("git", "-C", root, "rev-parse", "--short", "HEAD").Output()
			if err != nil {
				t.Fatal(err)
			}
			if name == "unavailable" {
				runGit(t, root, "branch", "-m", "unpublished")
			}
			streams, _, err := loadStreams(root)
			if err != nil {
				t.Fatal(err)
			}
			_, notices := checkBriefFiles(streams, streams)
			got := strings.Join(notices, "\n")
			want := "stale FAIL"
			switch name {
			case "unavailable", "undated":
				want = "could-not-check stale FAIL"
			case "unrelated", "same-day", "head-only":
				want = "no decision-issue — file one via --decision-issues"
			case "superseded":
				want = "sign-off card missing"
			}
			if !strings.Contains(got, want) {
				t.Errorf("want %q; got %s", want, got)
			}
			if want == "stale FAIL" && (!strings.Contains(got, strings.TrimSpace(string(sha))) || strings.Contains(got, "no decision-issue")) {
				t.Errorf("wrong stale route: %s", got)
			}
			if want != "stale FAIL" && want != "could-not-check stale FAIL" && strings.Contains(got, "stale FAIL") {
				t.Errorf("false stale route: %s", got)
			}
		})
	}
}
