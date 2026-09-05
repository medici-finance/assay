package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// push_test.go — the authenticated-transport verbs (`deskgit push --as`, `deskgit fetch
// --as`), desk-tools/08. Every test here is fixture-only: a bare LOCAL repo is the "remote",
// so no test ever contacts a real host, and the token is a fixture supplied through the
// roleTokenForRepo seam so no App credential is minted.

// fixtureToken is the sentinel the token seam hands back. Its VALUE is what row 6 greps every
// output surface for — it must never appear in argv, stdout, stderr or the audit line. It is
// deliberately unlike anything deskgit emits so a match is unambiguous — and deliberately NOT
// shaped like a real GitHub App/personal token (it carries none of their prefixes), so this
// test fixture never trips a secret-scan or leak-sweep in a public tree.
const fixtureToken = "FIXTURE-deadbeefcafef00d-DESKGIT08-role-token"

// asWorker binds this session to the worker App role (loop worker-desk → role "worker") and
// replaces the token resolver with a fixture, so the authenticated path runs without minting
// a real credential. The returned pointer records whether roleTokenForRepo was EVER called —
// the "before any token is read" assertions read it. Call AFTER withEnv/withEnvCmds so the
// DESK_LOOP set here is not clobbered.
func asWorker(t *testing.T) *bool {
	t.Helper()
	t.Setenv("DESK_LOOP", "worker-desk")
	called := new(bool)
	prev := roleTokenForRepo
	roleTokenForRepo = func(role, repo string) (string, string, error) {
		*called = true
		return fixtureToken, "/fixture/config/worker.token", nil
	}
	t.Cleanup(func() { roleTokenForRepo = prev })
	return called
}

// onBranch checks out a fresh non-protected branch in work (push refuses main/master), so a
// push has a legitimate current branch to move. It carries main's commit, which is all the
// remote ref needs to advance to.
func onBranch(t *testing.T, work, branch string) {
	t.Helper()
	mustGit(t, work, "checkout", "-b", branch)
}

// gitCallWith returns the recorded argv of the git invocation whose token slice contains
// verb (e.g. "push"/"fetch"), or nil. Unlike fetchArgv it does not assume a fixed position,
// because the authenticated forms prepend `-c credential.helper=` before the verb.
func gitCallWith(calls [][]string, verb string) []string {
	for _, c := range calls {
		if len(c) == 0 || c[0] != "git" {
			continue
		}
		for _, a := range c[1:] {
			if a == verb {
				return c
			}
		}
	}
	return nil
}

// assertCredentialHelperCleared asserts the argv silences the ambient credential helper on
// the command line, BEFORE the verb — the placement the pre-mortem calls out (row 5). A
// local-path transport does not consult a credential helper at all, so this argv-position
// assertion, not a canary side effect, is what actually catches the reset being misplaced.
func assertCredentialHelperCleared(t *testing.T, argv, verb string) {
	t.Helper()
	prefix := "-c credential.helper="
	vi := strings.Index(argv, " "+verb)
	ci := strings.Index(argv, prefix)
	if ci < 0 {
		t.Fatalf("argv %q does not clear the ambient credential helper (%q missing)", argv, prefix)
	}
	if vi < 0 || ci > vi {
		t.Fatalf("argv %q places %q AFTER the verb %q — git would not honour it as a global option",
			argv, prefix, verb)
	}
}

// Row 2: a `push --as` advances the bare fixture remote's ref to the worktree HEAD, and the
// argv the runner constructed is EXACTLY the fixed authenticated form.
func TestPushAdvancesFixtureRemote(t *testing.T) {
	work := newRepo(t, allowedSlug)
	onBranch(t, work, "feature-1")
	calls := withEnv(t, work)
	asWorker(t)

	upstream := mustGit(t, work, "remote", "get-url", "origin")
	wantHEAD := mustGit(t, work, "rev-parse", "HEAD")

	if code := run([]string{"push", "--as", "worker"}); code != deskkit.ExitOK {
		t.Fatalf("push --as worker exit = %d, want %d (ok)", code, deskkit.ExitOK)
	}
	// The remote ref now equals the worktree HEAD — the push actually landed.
	got := mustGit(t, upstream, "rev-parse", "refs/heads/feature-1")
	if got != wantHEAD {
		t.Fatalf("remote refs/heads/feature-1 = %s, want worktree HEAD %s", got, wantHEAD)
	}
	// The argv is exactly the fixed form: helper cleared before the verb, receive-pack pinned,
	// origin + the current-branch refspec, and NOTHING else.
	argv := strings.Join(gitCallWith(*calls, "push"), " ")
	want := "git -c credential.helper= push --receive-pack=git-receive-pack origin refs/heads/feature-1:refs/heads/feature-1"
	if argv != want {
		t.Fatalf("push argv = %q\n want %q", argv, want)
	}
}

// Row 3: push refuses main/master (in any case) and a detached HEAD; both exit 5 and the
// remote ref is untouched, and NO token is read on either refusal.
func TestPushRefusesMainAndDetachedHead(t *testing.T) {
	t.Run("current branch main is refused", func(t *testing.T) {
		work := newRepo(t, allowedSlug) // newRepo leaves work on main
		calls := withEnv(t, work)
		called := asWorker(t)

		upstream := mustGit(t, work, "remote", "get-url", "origin")
		before := mustGit(t, upstream, "rev-parse", "refs/heads/main")

		if code := run([]string{"push", "--as", "worker"}); code != deskkit.ExitRefused {
			t.Fatalf("push on main exit = %d, want %d (refused)", code, deskkit.ExitRefused)
		}
		if gitCallWith(*calls, "push") != nil {
			t.Fatal("git push must not run when the current branch is main")
		}
		if *called {
			t.Fatal("a token was read on a branch refusal — the refusal must precede the token read")
		}
		if after := mustGit(t, upstream, "rev-parse", "refs/heads/main"); after != before {
			t.Fatalf("remote main moved on a refused push: %s -> %s", before, after)
		}
	})

	// Mixed-case master must be refused too — isProtectedBranch compares with EqualFold, so a
	// current branch spelled Master/MASTER/mASTEr is caught. (A mixed-case spelling of *main*
	// is not reachable by checkout here: the fixture already carries `main`, and the desk's
	// filesystem is case-INSENSITIVE, so `git checkout -b Main` resolves to the SAME ref and
	// git refuses to create it — which is itself why the guard must be case-insensitive. The
	// master variants exercise the same EqualFold path with no such collision.)
	t.Run("mixed-case master is refused", func(t *testing.T) {
		for _, b := range []string{"Master", "MASTER", "mASTEr"} {
			work := newRepo(t, allowedSlug)
			// Create the branch with its literal spelling and check it out.
			mustGit(t, work, "checkout", "-b", b)
			calls := withEnv(t, work)
			asWorker(t)
			if code := run([]string{"push", "--as", "worker"}); code != deskkit.ExitRefused {
				t.Fatalf("push on %s exit = %d, want refused", b, code)
			}
			if gitCallWith(*calls, "push") != nil {
				t.Fatalf("git push must not run for current branch %s", b)
			}
		}
	})

	t.Run("detached HEAD is refused", func(t *testing.T) {
		work := newRepo(t, allowedSlug)
		onBranch(t, work, "feature-2")
		mustGit(t, work, "checkout", "--detach")
		calls := withEnv(t, work)
		called := asWorker(t)
		if code := run([]string{"push", "--as", "worker"}); code != deskkit.ExitRefused {
			t.Fatalf("push on detached HEAD exit = %d, want %d (refused)", code, deskkit.ExitRefused)
		}
		if gitCallWith(*calls, "push") != nil {
			t.Fatal("git push must not run on a detached HEAD")
		}
		if *called {
			t.Fatal("a token was read on a detached-HEAD refusal")
		}
	})
}

// Row 4: --as for a role the session's loop identity does not bind exits 5, BEFORE any token
// is read. worker-desk binds "worker"; naming "reviewer" must be refused with no token read.
func TestAsRoleMustMatchSessionIdentity(t *testing.T) {
	work := newRepo(t, allowedSlug)
	onBranch(t, work, "feature-3")
	calls := withEnv(t, work)
	called := asWorker(t) // binds worker

	if code := run([]string{"push", "--as", "reviewer"}); code != deskkit.ExitRefused {
		t.Fatalf("push --as reviewer (session is worker) exit = %d, want %d (refused)", code, deskkit.ExitRefused)
	}
	if *called {
		t.Fatal("roleTokenForRepo was called for a role the session is not bound to — the identity check must precede any token read")
	}
	if gitCallWith(*calls, "push") != nil {
		t.Fatal("git push must not run when --as names an unbound role")
	}

	// The same rule binds fetch --as.
	if code := run([]string{"fetch", "--as", "reviewer"}); code != deskkit.ExitRefused {
		t.Fatalf("fetch --as reviewer exit = %d, want %d (refused)", code, deskkit.ExitRefused)
	}
	if *called {
		t.Fatal("fetch --as read a token for an unbound role")
	}
}

// Row 5: the NEGATIVE control. A repo-config credential.helper is ARMED with a canary script;
// after a `--as` push the canary must not exist, AND — the assertion that actually catches a
// misplaced reset, since a local-path push never consults a helper — the argv must clear the
// helper list BEFORE the verb.
func TestAmbientCredentialHelperNeverConsulted(t *testing.T) {
	work := newRepo(t, allowedSlug)
	onBranch(t, work, "feature-4")

	canary := filepath.Join(t.TempDir(), "CANARY_FIRED")
	helper := filepath.Join(t.TempDir(), "canary-helper.sh")
	script := "#!/bin/sh\ntouch " + canary + "\necho username=x\necho password=y\n"
	if err := os.WriteFile(helper, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	// Arm the ambient helper in the repo config — the shadowing the transcript sweep observed.
	mustGit(t, work, "config", "credential.helper", helper)

	calls := withEnv(t, work)
	asWorker(t)
	if code := run([]string{"push", "--as", "worker"}); code != deskkit.ExitOK {
		t.Fatalf("push --as worker exit = %d, want ok", code)
	}
	if _, err := os.Stat(canary); err == nil {
		t.Fatal("the armed credential.helper canary FIRED — the ambient helper was consulted")
	}
	argv := strings.Join(gitCallWith(*calls, "push"), " ")
	assertCredentialHelperCleared(t, argv, "push")
}

// Row 6: the token never leaves the child. Its fixture VALUE must be absent from argv,
// stdout, stderr and the audit line; and the ephemeral askpass dir must be gone after both a
// successful push and an injected failure.
func TestTokenNeverLeavesTheChild(t *testing.T) {
	// Confine the ephemeral askpass dir to a scratch parent so its removal can be asserted
	// without racing other /tmp entries.
	askParent := t.TempDir()
	prevParent := askpassTempParent
	askpassTempParent = askParent
	t.Cleanup(func() { askpassTempParent = prevParent })

	noLeak := func(t *testing.T) {
		t.Helper()
		ents, err := os.ReadDir(askParent)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range ents {
			if strings.HasPrefix(e.Name(), "deskgit-askpass-") {
				t.Fatalf("askpass temp dir leaked: %s", e.Name())
			}
		}
	}

	t.Run("success path", func(t *testing.T) {
		work := newRepo(t, allowedSlug)
		onBranch(t, work, "feature-5")
		calls := withEnv(t, work)
		asWorker(t)

		out := captureStdoutStderr(t, func() {
			if code := run([]string{"push", "--as", "worker"}); code != deskkit.ExitOK {
				t.Fatalf("push exit = %d, want ok", code)
			}
		})
		if strings.Contains(out, fixtureToken) {
			t.Fatal("the token value appeared in stdout/stderr")
		}
		for _, c := range *calls {
			if strings.Contains(strings.Join(c, " "), fixtureToken) {
				t.Fatalf("the token value appeared in a git argv: %v", c)
			}
		}
		for _, e := range readAudit(t) {
			if strings.Contains(e.Detail, fixtureToken) || strings.Contains(e.Repo, fixtureToken) {
				t.Fatalf("the token value appeared in the audit line: %+v", e)
			}
		}
		noLeak(t)
	})

	t.Run("injected failure path still cleans up and leaks no token", func(t *testing.T) {
		work := newRepo(t, allowedSlug)
		onBranch(t, work, "feature-6")
		// Inject a failure the credential path cannot recover from: remove the bare remote so
		// the push itself fails. The effective-URL gate still passes (RepoForLocalPath falls
		// back to the cleaned absolute path, which is still the configured root), so the flow
		// reaches askpassSupply + the push, then fails — exercising the cleanup on the error path.
		upstream := mustGit(t, work, "remote", "get-url", "origin")
		if err := os.RemoveAll(upstream); err != nil {
			t.Fatal(err)
		}
		calls := withEnv(t, work)
		asWorker(t)
		out := captureStdoutStderr(t, func() {
			if code := run([]string{"push", "--as", "worker"}); code != deskkit.ExitUnverifiable {
				t.Fatalf("push to a removed remote exit = %d, want %d (unverifiable)", code, deskkit.ExitUnverifiable)
			}
		})
		if strings.Contains(out, fixtureToken) {
			t.Fatal("the token value appeared in stdout/stderr on the failure path")
		}
		for _, c := range *calls {
			if strings.Contains(strings.Join(c, " "), fixtureToken) {
				t.Fatalf("the token value appeared in a git argv on the failure path: %v", c)
			}
		}
		noLeak(t)
	})
}

// captureStdoutStderr runs fn with BOTH os.Stdout and os.Stderr redirected to one temp file
// and returns everything written to either. No test in this package runs in parallel, so
// swapping the globals is safe.
func captureStdoutStderr(t *testing.T, fn func()) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "outerr-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = f, f
	fn()
	os.Stdout, os.Stderr = oldOut, oldErr
	if cerr := f.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	b, rerr := os.ReadFile(f.Name())
	if rerr != nil {
		t.Fatal(rerr)
	}
	return string(b)
}

// Row 7: the push-widening options are each refused BY NAME with their OWN reason, before
// the FlagSet, and no git push is constructed. --receive-pack is refused by the transport-exec
// guard (it names a program); the rest by checkPushSafety.
func TestPushOptionsRefusedByName(t *testing.T) {
	cases := []struct {
		name   string
		argv   []string
		reason string // a phrase only that option's own refusal carries
	}{
		{"force", []string{"push", "--as", "worker", "--force"}, "non-fast-forward"},
		{"delete", []string{"push", "--as", "worker", "--delete"}, "delete the remote ref"},
		{"no-verify", []string{"push", "--as", "worker", "--no-verify"}, "pre-push hook"},
		{"receive-pack", []string{"push", "--as", "worker", "--receive-pack=git-receive-pack.evil"}, namedGuardMarker},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			work := newRepo(t, allowedSlug)
			onBranch(t, work, "feature-7")
			calls := withEnv(t, work)
			asWorker(t)
			var code int
			msg := captureStderr(t, func() { code = run(tc.argv) })
			if code != deskkit.ExitRefused {
				t.Fatalf("%s exit = %d, want %d (refused)", tc.name, code, deskkit.ExitRefused)
			}
			if !strings.Contains(msg, tc.reason) {
				t.Errorf("%s refusal %q does not carry its own reason %q", tc.name, msg, tc.reason)
			}
			if gitCallWith(*calls, "push") != nil {
				t.Fatalf("git push must not run for a refused option (%s)", tc.name)
			}
		})
	}
}

// Row 8: fetch --as keeps every fetch guard — the hardening pins and the effective-URL gate —
// and only adds the credential channel.
func TestFetchAsRoleKeepsEveryFetchGuard(t *testing.T) {
	t.Run("hardening pins still present, helper cleared before the verb", func(t *testing.T) {
		work := newRepo(t, allowedSlug)
		calls := withEnv(t, work)
		asWorker(t)
		if code := run([]string{"fetch", "--as", "worker"}); code != deskkit.ExitOK {
			t.Fatalf("fetch --as worker exit = %d, want ok", code)
		}
		argv := gitCallWith(*calls, "fetch")
		if argv == nil {
			t.Fatal("no git fetch was constructed for fetch --as")
		}
		assertHardened(t, argv)
		assertCredentialHelperCleared(t, strings.Join(argv, " "), "fetch")
	})

	t.Run("effective-URL gate still refuses a rewritten origin", func(t *testing.T) {
		work := newRepo(t, allowedSlug)
		recorded := mustGit(t, work, "config", "--get", "remote.origin.url")
		denied := filepath.Join(t.TempDir(), filepath.FromSlash(deniedSlug)+".git")
		mustGit(t, work, "config", "url."+denied+".insteadOf", recorded)

		calls := withEnv(t, work)
		called := asWorker(t)
		if code := run([]string{"fetch", "--as", "worker"}); code != deskkit.ExitRefused {
			t.Fatalf("fetch --as worker on a rewritten origin exit = %d, want %d (refused)", code, deskkit.ExitRefused)
		}
		if gitCallWith(*calls, "fetch") != nil {
			t.Fatal("git fetch must not run when the effective URL is denied, even under --as")
		}
		if *called {
			t.Fatal("a token was read though the origin gate refused — the gate must precede the token read")
		}
	})
}
