package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// #1573 follow-up: the origin gate must decide on the URL git itself will use, not on a
// parallel read of the repository config file. A reader that only sees `.git/config` misses
// worktree-scoped (and global) values, the empty-value list reset, and insteadOf rules held in
// those scopes — so it can pass an allowed slug while `git fetch origin` connects elsewhere.

// fixtureAlias is a url.<base>.insteadOf key used only by these fixtures. It is not a
// transport git knows, so it resolves to nothing unless the rewrite applies.
const fixtureAlias = "deskgit-fixture-alias:tracker"

// newLinkedWorktree adds a linked worktree of work and turns on per-worktree config, which is
// how every desk worktree is isolated. Returns the linked worktree's path.
func newLinkedWorktree(t *testing.T, work string) string {
	t.Helper()
	wt := filepath.Join(t.TempDir(), "linked")
	mustGit(t, work, "worktree", "add", "-b", "linked-wt", wt)
	mustGit(t, work, "config", "extensions.worktreeConfig", "true")
	return wt
}

// newDeniedUpstream creates a real bare repo whose path ends in the out-of-set slug, so a
// fetch that reaches it SUCCEEDS — the smuggle is observable as exit 0, not as a network fault.
func newDeniedUpstream(t *testing.T) string {
	t.Helper()
	denied := filepath.Join(t.TempDir(), filepath.FromSlash(deniedSlug)+".git")
	if err := os.MkdirAll(filepath.Dir(denied), 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "", "init", "--bare", "-b", "main", denied)
	return denied
}

// The fail-first case. The shared config says origin is the allowed upstream; the linked
// worktree resets the url list (an empty value) and sets an alias, which a worktree-scoped
// insteadOf rewrites to the DENIED upstream. `git remote get-url --all origin` — what the
// fetch uses — names the denied repo. A read of `.git/config` alone sees the allowed one and
// lets `git fetch` run against the denied repo.
func TestFetch_WorktreeScopedURLAndInsteadOf_GateSeesGitsURL(t *testing.T) {
	work := newRepo(t, allowedSlug)
	wt := newLinkedWorktree(t, work)
	denied := newDeniedUpstream(t)

	mustGit(t, wt, "config", "--worktree", "--add", "remote.origin.url", "")
	mustGit(t, wt, "config", "--worktree", "--add", "remote.origin.url", fixtureAlias)
	mustGit(t, wt, "config", "--worktree", "url."+denied+".insteadOf", fixtureAlias)

	// Precondition: git itself resolves origin to the denied upstream in this worktree.
	if got := mustGit(t, wt, "remote", "get-url", "--all", "origin"); got != denied {
		t.Fatalf("fixture: git resolves origin to %q, want the denied upstream %q", got, denied)
	}

	calls := withEnv(t, wt)
	if code := run([]string{"fetch"}); code != deskkit.ExitRefused {
		t.Fatalf("fetch exit = %d, want %d (git's effective origin is out of set)", code, deskkit.ExitRefused)
	}
	if fetchArgv(*calls) != nil {
		t.Fatal("git fetch must NOT run when the URL git will use is not allowed")
	}
	var saw bool
	for _, e := range readAudit(t) {
		if e.Verb == "fetch" && e.Result == deskkit.ResultRefused {
			saw = true
			// The refusal must be about the URL git would fetch from, not the shared one.
			if !strings.Contains(e.Detail, deniedSlug+".git") {
				t.Fatalf("audit detail = %q, want it to name the URL git would fetch from (…%s.git)", e.Detail, deniedSlug)
			}
		}
	}
	if !saw {
		t.Fatal("no refused fetch audit line was written")
	}
}

// The same resolution on the allowed side: a worktree whose OWN url (reset + alias rewritten
// by a worktree-scoped insteadOf) lands on the allowed upstream is admitted, even though the
// shared config names a repo that does not parse. The gate follows git, in both directions.
func TestFetch_WorktreeScopedURL_AllowedByGitsResolution(t *testing.T) {
	work := newRepo(t, allowedSlug)
	upstream := mustGit(t, work, "remote", "get-url", "origin")
	wt := newLinkedWorktree(t, work)

	mustGit(t, work, "config", "remote.origin.url", "not a url")
	mustGit(t, wt, "config", "--worktree", "--add", "remote.origin.url", "")
	mustGit(t, wt, "config", "--worktree", "--add", "remote.origin.url", fixtureAlias)
	mustGit(t, wt, "config", "--worktree", "url."+upstream+".insteadOf", fixtureAlias)

	calls := withEnv(t, wt)
	if code := run([]string{"fetch"}); code != deskkit.ExitOK {
		t.Fatalf("fetch exit = %d, want ok (git resolves origin to the allowed upstream)", code)
	}
	if fetchArgv(*calls) == nil {
		t.Fatal("git fetch should have run")
	}
}

// A multi-valued url list is refused fail-closed: fetch uses only the first value, push uses
// every one, so no single URL can stand for the list and the gate will not pick one.
func TestFetch_MultiValuedOriginURL_FailsClosed(t *testing.T) {
	work := newRepo(t, allowedSlug)
	denied := newDeniedUpstream(t)
	mustGit(t, work, "config", "--add", "remote.origin.url", denied)

	calls := withEnv(t, work)
	code := run([]string{"fetch"})
	if code == deskkit.ExitOK {
		t.Fatal("fetch with a multi-valued origin url list must not succeed")
	}
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("fetch exit = %d, want %d (fail-closed)", code, deskkit.ExitUnverifiable)
	}
	if fetchArgv(*calls) != nil {
		t.Fatal("git fetch must NOT run when origin has more than one url")
	}
	var saw bool
	for _, e := range readAudit(t) {
		if e.Verb == "fetch" && strings.Contains(e.Detail, "2 URLs") {
			saw = true
		}
	}
	if !saw {
		t.Fatalf("audit detail should name the multi-valued list; got %+v", readAudit(t))
	}
}

// A global-scope insteadOf (the operator's ~/.gitconfig) also rewrites what git fetches from,
// so the gate must see it too.
func TestFetch_GlobalInsteadOf_GateSeesGitsURL(t *testing.T) {
	work := newRepo(t, allowedSlug)
	recorded := mustGit(t, work, "config", "--get", "remote.origin.url")
	denied := newDeniedUpstream(t)

	calls := withEnv(t, work) // sets a private HOME
	gc := "[url \"" + denied + "\"]\n\tinsteadOf = " + recorded + "\n"
	if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".gitconfig"), []byte(gc), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"fetch"}); code != deskkit.ExitRefused {
		t.Fatalf("fetch exit = %d, want %d (a global insteadOf points origin out of set)", code, deskkit.ExitRefused)
	}
	if fetchArgv(*calls) != nil {
		t.Fatal("git fetch must NOT run when a global insteadOf rewrites origin out of set")
	}
}
