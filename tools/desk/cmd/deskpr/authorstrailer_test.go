package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// authorstrailer_test.go — #1339: a briefs-AUTHORING PR carries `Authors:`, never `Brief:`.
//
// `Brief:` is read as DELIVERY by every reader of the link edge (the dispatcher's phantom check,
// the planner's reconciliation, the derived board). An authoring PR that carried it made the brief
// it wrote read as delivered once it merged, so the brief could never be dispatched. create now
// accepts `Authors: <stream>/<NN>[, …]` and refuses `Brief:` on a branch whose diff only authors
// that brief.

// newAuthoringFixture is newBaseFixture moved onto a branch that only AUTHORS fixture/02 and
// fixture/03: it adds both brief files, edits the stream board README and adds a changelog
// fragment, and touches nothing else. extra, when non-empty, is one more repo-relative path the
// branch adds, so a caller can turn the branch into a delivery.
func newAuthoringFixture(t *testing.T, extra string) string {
	t.Helper()
	work := newBaseFixture(t)
	mustGit(t, work, "checkout", "-b", "feature/author-briefs", "refs/remotes/origin/main")
	for rel, content := range map[string]string{
		"docs/streams/fixture/brief-02-second.md": "---\nschema: brief-v1\nbrief: fixture/02\ntitle: second\n---\n",
		"docs/streams/fixture/brief-03-third.md":  "---\nschema: brief-v1\nbrief: fixture/03\ntitle: third\n---\n",
		"docs/streams/fixture/README.md":          "| # | Status |\n| 02 | todo |\n| 03 | todo |\n",
		"changelog/fixture-02-03-briefs.md":       "- Authored fixture briefs 02 and 03.\n",
	} {
		p := filepath.Join(work, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		writeFile(t, p, content)
	}
	if extra != "" {
		p := filepath.Join(work, filepath.FromSlash(extra))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		writeFile(t, p, "package x\n")
	}
	mustGit(t, work, "add", "-A")
	mustGit(t, work, "commit", "-m", "author fixture briefs 02-03")
	return work
}

// TestCreateRefusesBriefOnAuthoringBranch is the fail-first proof of the deskpr half of #1339:
// before the fix, `deskpr create` accepted `Brief: fixture/02` on a branch that only wrote that
// brief, and the merged PR then read as its delivery. Now create refuses (exit 5) before any push,
// names the `Authors:` line to use, and does so under --check too (the gate is local).
func TestCreateRefusesBriefOnAuthoringBranch(t *testing.T) {
	for _, check := range []bool{false, true} {
		work := newAuthoringFixture(t, "")
		calls := withEnv(t, work)
		args := []string{"--title", "author briefs", "--body-min", "Authors two briefs.\nBrief: fixture/02"}
		if check {
			args = append(args, "--check")
		}
		err := cmdCreate(args)
		if !deskkit.IsRefused(err) {
			t.Fatalf("check=%v: create with `Brief:` on an authoring-only branch err = %v, want exit-5 refusal", check, err)
		}
		if !strings.Contains(err.Error(), "Authors: fixture/02") {
			t.Fatalf("check=%v: refusal must name the `Authors:` line to use; got: %v", check, err)
		}
		if anyCall(gitCalls(*calls), "push") || curForge.createCalls > 0 {
			t.Fatalf("check=%v: the refusal must precede any push or create; git calls: %v", check, gitCalls(*calls))
		}
	}
}

// TestCreateAcceptsAuthorsOnAuthoringBranch: the same branch with the right trailer opens its PR.
func TestCreateAcceptsAuthorsOnAuthoringBranch(t *testing.T) {
	work := newAuthoringFixture(t, "")
	calls := withEnv(t, work)
	rc := run([]string{"create", "--title", "author briefs", "--body-min", "Authors two briefs.\nAuthors: fixture/02, fixture/03"})
	if rc != deskkit.ExitOK {
		t.Fatalf("create with `Authors:` rc = %d, want 0", rc)
	}
	if !anyCall(gitCalls(*calls), "push", "-u", "origin", "feature/author-briefs") {
		t.Fatalf("expected the branch push; git calls: %v", gitCalls(*calls))
	}
}

// TestCreateBriefGateStaysNarrow: the gate refuses ONLY the provable authoring shape. A `Brief:`
// on a branch that also touches a code path (author-and-deliver), or that names a brief the branch
// did not add, is left to the existing trailer grammar and opens normally.
func TestCreateBriefGateStaysNarrow(t *testing.T) {
	for _, tc := range []struct {
		name, extra, trailer string
	}{
		{"authoring plus code", "tools/x/x.go", "Brief: fixture/02"},
		{"brief not added by the branch", "", "Brief: fixture/01"},
		{"docs deliverable under docs/streams", "docs/streams/fixture/audit.md", "Brief: fixture/02"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			work := newAuthoringFixture(t, tc.extra)
			withEnv(t, work)
			if rc := run([]string{"create", "--title", "t", "--body-min", "body\n" + tc.trailer}); rc != deskkit.ExitOK {
				t.Fatalf("%s: create rc = %d, want 0 — the authoring gate must not fire", tc.name, rc)
			}
		})
	}
}

// TestRequireTrailerAuthorsGrammar pins the `Authors:` grammar at the one parse deskpr owns: every
// entry must resolve to a brief file under --root, the list may not be empty or repeat a brief, and
// `Authors:` never shares a body with `Brief:` or `Issue:`.
func TestRequireTrailerAuthorsGrammar(t *testing.T) {
	work := newAuthoringFixture(t, "")
	for _, tc := range []struct {
		body string
		ok   bool
		want string
	}{
		{"Authors: fixture/02", true, ""},
		{"Authors: fixture/02, fixture/03", true, ""},
		{"Authors: fixture/02 fixture:03", true, ""},
		{"Authors: fixture/02, fixture/09", false, "`Authors: fixture/09` does not resolve"},
		{"Authors: not-a-brief", false, "does not name a brief"},
		{"Authors: fixture/02, fixture/02", false, "twice"},
		{"Authors: fixture/02\nBrief: fixture/02", false, "exactly one link"},
		{"Authors: fixture/02\nIssue: #7", false, "exactly one link"},
		{"Authors: fixture/02\nAuthors: fixture/03", false, "duplicate"},
	} {
		n, err := requireTrailer([]byte("body\n"+tc.body+"\n"), ".", work)
		if tc.ok {
			if err != nil || n != 0 {
				t.Errorf("%q: got (%d, %v), want (0, nil)", tc.body, n, err)
			}
			continue
		}
		if !deskkit.IsRefused(err) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q: got %v, want an exit-5 refusal containing %q", tc.body, err, tc.want)
		}
	}
}

// TestTrailerLinkAuthors: edit's trailer-immutability compare renders `Authors:` as its own link,
// so an Authors: body can neither be re-pointed at a Brief: nor silently compared equal to one.
func TestTrailerLinkAuthors(t *testing.T) {
	got, ok := trailerLink([]byte("x\nAuthors: fixture/02, fixture/03\n"))
	if !ok || got != "Authors: fixture/02, fixture/03" {
		t.Fatalf("trailerLink(Authors) = (%q, %v)", got, ok)
	}
	if b, _ := trailerLink([]byte("Brief: fixture/02\n")); b == got {
		t.Fatalf("an Authors: link must not compare equal to a Brief: link")
	}
}
