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

// Behaviour change, pinned (the git binary is gone from fetch): the gate and the connection read
// ONE string — the repository's own configured remote.origin.url. The shared config says origin
// is the allowed upstream; the linked worktree resets the url list and rewrites an alias, via
// worktree-scoped config, to the DENIED upstream. The git binary used to follow that and fetch
// from the denied repo (so the old gate had to see it). The in-process transport reads no
// worktree-scope config, so the fetch goes to the allowed upstream the gate decided on and the
// denied repo is never contacted: the gate cannot be contradicted by a scope it does not read.
func TestFetch_WorktreeScopedURL_NotConsulted(t *testing.T) {
	work := newRepo(t, allowedSlug)
	shared := originURL(t, work)
	wt := newLinkedWorktree(t, work)
	denied := newDeniedUpstream(t)

	mustGit(t, wt, "config", "--worktree", "--add", "remote.origin.url", "")
	mustGit(t, wt, "config", "--worktree", "--add", "remote.origin.url", fixtureAlias)
	mustGit(t, wt, "config", "--worktree", "url."+denied+".insteadOf", fixtureAlias)

	withEnv(t, wt)
	if code := run([]string{"fetch"}); code != deskkit.ExitOK {
		t.Fatalf("fetch exit = %d, want ok (the worktree-scope rewrite is not part of this fetch)", code)
	}
	if got := onlyFetch(t); got.URL != shared {
		t.Fatalf("fetch connected to %q, want the gated shared URL %q (never the denied %q)", got.URL, shared, denied)
	}
}

// The gate still refuses when the repository's OWN url is bad, and a worktree-scoped alias
// cannot rescue it: the string that fails the gate is the string that would be connected to.
func TestFetch_BadSharedURL_NotRescuedByWorktreeScope(t *testing.T) {
	work := newRepo(t, allowedSlug)
	upstream := mustGit(t, work, "remote", "get-url", "origin")
	wt := newLinkedWorktree(t, work)

	mustGit(t, work, "config", "remote.origin.url", "not a url")
	mustGit(t, wt, "config", "--worktree", "--add", "remote.origin.url", "")
	mustGit(t, wt, "config", "--worktree", "--add", "remote.origin.url", fixtureAlias)
	mustGit(t, wt, "config", "--worktree", "url."+upstream+".insteadOf", fixtureAlias)

	calls := withEnv(t, wt)
	if code := run([]string{"fetch"}); code != deskkit.ExitRefused {
		t.Fatalf("fetch exit = %d, want %d (the repository's own url does not parse)", code, deskkit.ExitRefused)
	}
	if fetchArgv(*calls) != nil {
		t.Fatal("the transport must not be reached when the gated url is refused")
	}
}

// A repo-LOCAL insteadOf is part of the repository's own config, and go-git applies it to the
// url list it hands back — so the string the gate decides on is already the rewritten one.
// Recorded origin allowed, a local insteadOf rewrites it to a DENIED slug: refused.
func TestFetch_RepoLocalInsteadOf_GateSeesRewrittenURL(t *testing.T) {
	work := newRepo(t, allowedSlug)
	recorded := originURL(t, work)
	denied := newDeniedUpstream(t)
	mustGit(t, work, "config", "url."+denied+".insteadOf", recorded)

	calls := withEnv(t, work)
	if code := run([]string{"fetch"}); code != deskkit.ExitRefused {
		t.Fatalf("fetch exit = %d, want %d (the rewritten url is out of set)", code, deskkit.ExitRefused)
	}
	if fetchArgv(*calls) != nil {
		t.Fatal("the transport must not be reached when the rewritten url is out of set")
	}
	var saw bool
	for _, e := range readAudit(t) {
		if e.Verb == "fetch" && e.Result == deskkit.ResultRefused && strings.Contains(e.Detail, deniedSlug+".git") {
			saw = true
		}
	}
	if !saw {
		t.Fatalf("no refused fetch audit line names the rewritten url; got %+v", readAudit(t))
	}
}

// A multi-valued url list is REFUSED: fetch uses only the first value (and, with no pushurl set,
// push uses every one), so no single URL can stand for the list and the gate will not pick one.
// A second value is a positive smuggle shape the tool has determined, not a could-not-run, so it
// exits 5 (refused) — the same class as parseRepo's refusals — never 6 (unverifiable).
func TestFetch_MultiValuedOriginURL_FailsClosed(t *testing.T) {
	work := newRepo(t, allowedSlug)
	denied := newDeniedUpstream(t)
	mustGit(t, work, "config", "--add", "remote.origin.url", denied)

	calls := withEnv(t, work)
	code := run([]string{"fetch"})
	if code == deskkit.ExitOK {
		t.Fatal("fetch with a multi-valued origin url list must not succeed")
	}
	if code != deskkit.ExitRefused {
		t.Fatalf("fetch exit = %d, want %d (refused: a multi-valued url list is a smuggle shape)", code, deskkit.ExitRefused)
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

// The push twin of the multi-valued refusal: `push --as` decides its repo on the same read, so a
// second url value is refused (exit 5) there too — before any token is read and before git push
// runs.
func TestPush_MultiValuedOriginURL_Refused(t *testing.T) {
	work := newRepo(t, allowedSlug)
	onBranch(t, work, "feature-1")
	denied := newDeniedUpstream(t)
	mustGit(t, work, "config", "--add", "remote.origin.url", denied)

	calls := withEnv(t, work)
	tokenRead := asWorker(t)
	if code := run([]string{"push", "--as", "worker"}); code != deskkit.ExitRefused {
		t.Fatalf("push --as worker exit = %d, want %d (refused: a multi-valued url list is a smuggle shape)", code, deskkit.ExitRefused)
	}
	if gitCallWith(*calls, "push") != nil {
		t.Fatal("git push must NOT run when origin has more than one url")
	}
	if *tokenRead {
		t.Fatal("no token may be read when origin has more than one url")
	}
}
