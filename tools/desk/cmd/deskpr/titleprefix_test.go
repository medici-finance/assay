package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// titleprefix_test.go — #4795: `deskpr create` leads the PR title with the brief id from a
// delivery trailer (`Brief:` / `Cross-repo-brief:`) unless the title already names it as a
// whole token. `Authors:` and `Issue:` PRs keep their title.

// regression: #4795
// TestPrefixTitleWithBrief is the table over the pure rule.
func TestPrefixTitleWithBrief(t *testing.T) {
	for _, tc := range []struct {
		name, title, body, want string
	}{
		{"prefix added", "add feature", "Brief: fixture/04", "fixture/04: add feature"},
		{"colon form reduces", "add feature", "Brief: repo:fixture:04", "fixture/04: add feature"},
		{"id already leading", "fixture/04: add feature", "Brief: fixture/04", "fixture/04: add feature"},
		{"id mid-title", "verify(fixture/04): add feature", "Brief: fixture/04", "verify(fixture/04): add feature"},
		{"id at end", "finish fixture/04", "Brief: fixture/04", "finish fixture/04"},
		{"case-insensitive", "Fixture/04: add feature", "Brief: fixture/04", "Fixture/04: add feature"},
		{"near-miss longer number", "fixture/046: add feature", "Brief: fixture/04", "fixture/04: fixture/046: add feature"},
		{"near-miss longer stream", "my-fixture/04: add feature", "Brief: fixture/04", "fixture/04: my-fixture/04: add feature"},
		{"near-miss path prefix", "a/fixture/04 x", "Brief: fixture/04", "fixture/04: a/fixture/04 x"},
		{"near-miss other alias", "other:fixture/04 x", "Brief: fixture/04", "fixture/04: other:fixture/04 x"},
		{"near-miss then real match", "fixture/046 and fixture/04", "Brief: fixture/04", "fixture/046 and fixture/04"},
		{"cross-repo prefix added", "add feature", "Cross-repo-brief: example-trk:fixture/05", "example-trk:fixture/05: add feature"},
		{"cross-repo already named", "example-trk:fixture/05: add feature", "Cross-repo-brief: example-trk:fixture/05", "example-trk:fixture/05: add feature"},
		{"cross-repo bare id is not it", "fixture/05: add feature", "Cross-repo-brief: example-trk:fixture/05", "example-trk:fixture/05: fixture/05: add feature"},
		{"authors unchanged", "author briefs", "Authors: fixture/02, fixture/03", "author briefs"},
		{"issue unchanged", "fix thing", "Issue: #12", "fix thing"},
		{"fixes unchanged", "fix thing", "Fixes: #12", "fix thing"},
		{"trailer in fence ignored", "add feature", "```\nBrief: fixture/04\n```\nIssue: #12", "add feature"},
		{"no trailer unchanged", "add feature", "just prose", "add feature"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := prefixTitleWithBrief(tc.title, []byte("body\n"+tc.body+"\n")); got != tc.want {
				t.Errorf("prefixTitleWithBrief(%q, %q) = %q, want %q", tc.title, tc.body, got, tc.want)
			}
		})
	}
}

// regression: #4795
// TestCreateUsesPrefixedTitle drives create end to end: the title handed to the forge is the
// prefixed one for Brief:/Cross-repo-brief:, and the original for Issue:.
func TestCreateUsesPrefixedTitle(t *testing.T) {
	root := xrRoot(t, xrRegistry)
	for _, tc := range []struct {
		name, title, body, root, want string
	}{
		{"brief", "add feature", "x\nBrief: fixture/01", ".", "fixture/01: add feature"},
		{"brief already named", "fixture/01: add feature", "x\nBrief: fixture/01", ".", "fixture/01: add feature"},
		{"cross-repo", "add feature", "x\nCross-repo-brief: example-trk:example-stream/05", root, "example-trk:example-stream/05: add feature"},
		{"issue", "add feature", "x\nIssue: #12", ".", "add feature"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			work := newBaseFixture(t)
			withEnv(t, work)
			if rc := run([]string{"create", "--title", tc.title, "--body-min", tc.body, "--root", tc.root}); rc != deskkit.ExitOK {
				t.Fatalf("create rc = %d, want 0", rc)
			}
			if curForge.created == nil || curForge.created.Title != tc.want {
				t.Fatalf("forge create title = %+v, want %q", curForge.created, tc.want)
			}
		})
	}
}

// regression: #4795
// TestCheckPrintsTitleUsed: --check prints the title create would use, with nothing created.
func TestCheckPrintsTitleUsed(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	rc := run([]string{"create", "--title", "add feature", "--body-min", "x\nBrief: fixture/01", "--check"})
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if rc != deskkit.ExitOK {
		t.Fatalf("--check rc = %d, want 0", rc)
	}
	if !strings.Contains(string(out), "check: title: fixture/01: add feature\n") {
		t.Fatalf("--check output lacks the prefixed title:\n%s", out)
	}
	if curForge.createCalls > 0 {
		t.Fatalf("--check created a PR")
	}
}
