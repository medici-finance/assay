package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// pushdest_test.go — #1623: deskpr's origin gates read what GIT resolves, not go-git's read of
// the repository config file. Every case drives REAL git config (url/pushurl values in local and
// worktree scope, insteadOf rewrites, multi-valued lists) through the real command path.
//
// No case reaches a network. An SSH destination is made to fail instantly by
// GIT_SSH_COMMAND=false, so on the UNFIXED code — where the gate passes and git attempts the
// push — the attempt is observable as rc 6 (or a partial push) without a connection being made.

// sentinelPushURL is the shape a checkout carries when its push has been disabled on purpose.
const sentinelPushURL = "DISABLED-use-app-token-or-override"

// bareHead returns the SHA a bare repository's branch points at, or "" when it has none.
func bareHead(t *testing.T, bare, branch string) string {
	t.Helper()
	out, err := exec.Command("git", "--git-dir", bare, "rev-parse", "--verify", "--quiet",
		"refs/heads/"+branch).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// hermeticGitConfig isolates a direct gate call from the machine's global and system git config
// (a machine-wide insteadOf would otherwise rewrite the fixture URLs these cases classify).
func hermeticGitConfig(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func createErr(t *testing.T) error {
	t.Helper()
	return cmdCreate([]string{"--title", "x", "--body-min", "y\nBrief: fixture/01"})
}

func wantRefusal(t *testing.T, err error, substrs ...string) string {
	t.Helper()
	if err == nil {
		t.Fatal("want a refusal (exit 5), got success")
	}
	if code := deskkit.ExitCodeOf(err); code != deskkit.ExitRefused {
		t.Fatalf("want exit %d (refused), got %d: %v", deskkit.ExitRefused, code, err)
	}
	msg := err.Error()
	for _, s := range substrs {
		if !strings.Contains(msg, s) {
			t.Fatalf("refusal does not name %q:\n%s", s, msg)
		}
	}
	return msg
}

// (a) A non-URL sentinel push URL. Unfixed: nothing classified it, so the push ran and failed
// late with "does not appear to be a git repository" (exit 6). Fixed: a named refusal before any
// push, naming the value, the key and its scope, and the worktree-scoped remedy.
func TestPushDestSentinelIsRefusedByName(t *testing.T) {
	work := newBaseFixture(t)
	mustGit(t, work, "remote", "set-url", "--push", "origin", sentinelPushURL)
	calls := withEnv(t, work)

	msg := wantRefusal(t, createErr(t),
		sentinelPushURL, "not an absolute URL or path", "remote.origin.pushurl", "local",
		"config --worktree --replace-all remote.origin.pushurl ''",
		"--add remote.origin.pushurl https://github.com/example-org/tracker.git")
	if !strings.Contains(msg, "config extensions.worktreeConfig true") {
		t.Fatalf("worktreeConfig is off in this fixture, so the remedy must enable it first:\n%s", msg)
	}
	assertNoPushNoCreate(t, *calls)
}

// (b) A multi-valued pushurl: an INHERITED value (the shared checkout's config) plus a
// WORKTREE-scoped one. git pushes to every value. The inherited one is an https URL that a
// url.<base>.insteadOf rule turns into SSH — the shape a machine-wide rewrite produces — so the
// raw-config transport gate sees only https and a file path and passes it.
//
// Unfixed: the push fans out; the worktree's own destination receives the branch while the
// other half goes out over SSH (rc 6 here only because GIT_SSH_COMMAND=false stops it). Fixed:
// refused before any push, both values and both scopes named, the local bare untouched.
func TestPushDestMultiValuedRefuses(t *testing.T) {
	shared := newBaseFixture(t)
	mustGit(t, shared, "remote", "set-url", "--push", "origin", "https://example.com/example-org/tracker.git")
	mustGit(t, shared, "config", "url.ssh://git@127.0.0.1:1/.insteadOf", "https://example.com/")
	mustGit(t, shared, "config", "extensions.worktreeConfig", "true")

	bare := filepath.Join(t.TempDir(), "wt-origin.git")
	mustGit(t, "", "init", "--bare", "-b", "main", bare)
	wt := filepath.Join(t.TempDir(), "wt")
	mustGit(t, shared, "worktree", "add", "-b", "feature/wt-branch", wt)
	mustGit(t, wt, "config", "--worktree", "--add", "remote.origin.pushurl", "file://"+bare)
	if got := mustGit(t, wt, "remote", "get-url", "--push", "--all", "origin"); strings.Count(got, "\n") != 1 {
		t.Fatalf("fixture: want exactly two resolved push URLs, got:\n%s", got)
	}

	calls := withEnv(t, wt)
	t.Setenv("GIT_SSH_COMMAND", "false")

	wantRefusal(t, createErr(t),
		"2 values", "pushes to EVERY", "ssh://", "file://",
		"local    remote.origin.pushurl", "worktree remote.origin.pushurl",
		"url.ssh://git@127.0.0.1:1/.insteadof",
		"config --worktree --replace-all remote.origin.pushurl ''")
	assertNoPushNoCreate(t, *calls)
	if h := bareHead(t, bare, "feature/wt-branch"); h != "" {
		t.Fatalf("the worktree-scoped destination received the branch (%s) — a partial push happened", h)
	}
}

// An https push URL that a url.<base>.insteadOf rule rewrites to SSH. The raw config says https,
// git pushes over SSH.
func TestPushDestInsteadOfToSSHRefuses(t *testing.T) {
	work := newBaseFixture(t)
	mustGit(t, work, "remote", "set-url", "--push", "origin", "https://example.com/example-org/tracker.git")
	mustGit(t, work, "config", "url.ssh://git@127.0.0.1:1/.insteadOf", "https://example.com/")
	calls := withEnv(t, work)
	t.Setenv("GIT_SSH_COMMAND", "false")

	wantRefusal(t, createErr(t), "not an https URL", "ssh://", "an SSH transport",
		"url.ssh://git@127.0.0.1:1/.insteadof")
	assertNoPushNoCreate(t, *calls)
}

// (#1638 F-scp-single-letter) A single-letter scp-like host — `g:example-org/tracker.git`,
// exactly the shape a `~/.ssh/config` `Host g` alias resolves to a real forge on macOS/Linux.
// Pre-fix, classifyPushDest's unconditional `colon == 1` exception (a comment claiming it was
// "a Windows drive path, which git also treats as local" on every OS) classified this as
// pushLocal, which skips pushDestinationGate's transport check outright — so `deskpr create`
// reached `git push` over SSH to host "g" with no forge contacted, no credential presented,
// and no refusal, contradicting the PR's own claim that the only other admitted shape is a
// LOCAL destination. On a non-Windows runner (this repo's CI), the fix must refuse before any
// push is attempted.
func TestPushDestSingleLetterSSHHostRefuses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a single-letter host IS a drive path on Windows — that shape stays admitted there")
	}
	work := newBaseFixture(t)
	mustGit(t, work, "remote", "set-url", "--push", "origin", "g:example-org/tracker.git")
	calls := withEnv(t, work)
	t.Setenv("GIT_SSH_COMMAND", "false")

	wantRefusal(t, createErr(t), "g:example-org/tracker.git", "an SSH transport")
	assertNoPushNoCreate(t, *calls)
}

// update pushes too, so it is gated on the same terms and refuses BEFORE the token mint.
func TestPushDestUpdateRefusesBeforeMint(t *testing.T) {
	work := newBaseFixture(t)
	mustGit(t, work, "remote", "set-url", "--push", "origin", sentinelPushURL)
	calls := withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")

	rc := run([]string{"update"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("update over a sentinel push url rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	assertNoPushNoCreate(t, *calls)
	for _, c := range *calls {
		if len(c) > 0 && filepath.Base(c[0]) == "desktoken" {
			t.Fatalf("the push-destination refusal ran AFTER a token mint: %v", *calls)
		}
	}
}

// The FETCH side of preflight: origin's repository config names the allowed repo, but a
// GLOBAL-scope insteadOf rule makes git resolve it to another one. go-git's RemoteURL reads only
// the repository's own config file (it applies that file's insteadOf rules, not the global
// ones), so unfixed, the raw read passed the allow-list and the create went through. Fixed: the
// repo gate decides on git's resolution and refuses.
func TestPreflightGatesOnGitsOriginURL(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	global := filepath.Join(t.TempDir(), "gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	mustGit(t, "", "config", "--file", global,
		"url.https://github.com/someone-else/.insteadOf", "https://github.com/example-org/")

	wantRefusal(t, createErr(t), "someone-else/tracker", "not in the desk-tools repo set")
	assertNoPushNoCreate(t, *calls)
}

// A multi-valued remote.origin.url: no single URL names the repo. Unfixed: go-git's first value
// was gated and the create went through. Fixed: refused, both values named.
func TestPreflightMultiValuedOriginURLRefuses(t *testing.T) {
	work := newBaseFixture(t)
	mustGit(t, work, "config", "--add", "remote.origin.url", "https://github.com/someone-else/tracker.git")
	calls := withEnv(t, work)

	wantRefusal(t, createErr(t), "2 fetch URLs", "someone-else/tracker", "set exactly one")
	assertNoPushNoCreate(t, *calls)
}

// The positive controls: the gate is not a blanket refusal. A single LOCAL destination (every
// offline fixture) creates, and a single https destination naming the origin repo passes the
// gate itself (driven directly — pushing it would need the network).
func TestPushDestLocalAndHttpsAdmitted(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	if err := createErr(t); err != nil {
		t.Fatalf("a single local push destination must create, got: %v", err)
	}
	if !anyCall(gitCalls(*calls), "push", "-u", "origin", "feature/test-branch") {
		t.Fatalf("expected the push to proceed; git calls: %v", gitCalls(*calls))
	}

	hermeticGitConfig(t)
	gated := newBaseFixture(t)
	mustGit(t, gated, "remote", "set-url", "--push", "origin", "https://github.com:443/example-org/tracker.git")
	if err := pushDestinationGate(gated, "create", "example-org/tracker", ghURL); err != nil {
		t.Fatalf("one https push URL naming the origin repo must pass, got: %v", err)
	}
}

// An https destination that names a different repo than the fetch origin is refused.
func TestPushDestOtherRepoRefuses(t *testing.T) {
	hermeticGitConfig(t)
	work := newBaseFixture(t)
	mustGit(t, work, "remote", "set-url", "--push", "origin", "https://github.com/someone-else/tracker.git")
	err := pushDestinationGate(work, "create", "example-org/tracker", ghURL)
	wantRefusal(t, err, "names someone-else/tracker", "origin's fetch URL names example-org/tracker")
}

// An https destination carrying userinfo never prints the credential.
func TestPushDestRefusalRedactsUserinfo(t *testing.T) {
	hermeticGitConfig(t)
	work := newBaseFixture(t)
	mustGit(t, work, "remote", "set-url", "--push", "origin", "https://x-access-token:s3cr3t@github.com/someone-else/tracker.git")
	msg := wantRefusal(t, pushDestinationGate(work, "create", "example-org/tracker", ghURL), "<redacted>@github.com")
	if strings.Contains(msg, "s3cr3t") {
		t.Fatalf("the refusal leaked the URL's credential:\n%s", msg)
	}
}

// With extensions.worktreeConfig already on, the remedy does not re-enable it; an insteadOf rule
// that would rewrite the suggested URL is called out.
func TestPushDestRemedyShape(t *testing.T) {
	hermeticGitConfig(t)
	work := newBaseFixture(t)
	mustGit(t, work, "config", "extensions.worktreeConfig", "true")
	mustGit(t, work, "remote", "set-url", "--push", "origin", sentinelPushURL)
	mustGit(t, work, "config", "url.ssh://git@127.0.0.1:1/.insteadOf", "https://github.com/")
	msg := wantRefusal(t, pushDestinationGate(work, "create", "example-org/tracker", ghURL),
		"NOTE: a url.<base>.insteadOf rule rewrites URLs starting \"https://github.com/\"")
	if strings.Contains(msg, "extensions.worktreeConfig true") {
		t.Fatalf("worktreeConfig is already on; the remedy must not re-enable it:\n%s", msg)
	}
}

func TestClassifyPushDest(t *testing.T) {
	cases := map[string]pushDestKind{
		"https://github.com/o/r.git":        pushHTTPS,
		"HTTPS://github.com:443/o/r.git":    pushHTTPS,
		"https:///o/r.git":                  pushNotURL,
		"http://github.com/o/r.git":         pushCleartext,
		"file:///tmp/r.git":                 pushLocal,
		"/tmp/r.git":                        pushLocal,
		"ssh://git@github.com/o/r.git":      pushSSH,
		"git+ssh://git@github.com/o/r.git":  pushSSH,
		"git@github.com:o/r.git":            pushSSH,
		"github-alias:o/r.git":              pushSSH,
		"ext::sh -c touch% /tmp/pwned":      pushHelper,
		"git://github.com/o/r.git":          pushOtherTransport,
		sentinelPushURL:                     pushNotURL,
		"../relative/r.git":                 pushNotURL,
		"https://github.com/o/r::weird.git": pushHTTPS,
	}
	for in, want := range cases {
		if got := classifyPushDest(in); got != want {
			t.Errorf("classifyPushDest(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestClassifyPushDestSingleLetterHostGOOSGated pins #1638 F-scp-single-letter: a single letter
// before the colon (`g:owner/repo.git`) is a Windows drive path ONLY when this process actually
// runs on Windows. On macOS and Linux it is an scp-like SSH destination to host "g" — exactly
// what a `~/.ssh/config` `Host g` alias resolves — so it must classify as pushSSH there, never
// unconditionally as pushLocal. The pre-fix code returned pushLocal on every OS, which let
// deskpr's push-destination gate admit it (pushLocal skips the transport check entirely) and
// git push it over SSH.
func TestClassifyPushDestSingleLetterHostGOOSGated(t *testing.T) {
	want := pushSSH
	if runtime.GOOS == "windows" {
		want = pushLocal
	}
	for _, in := range []string{"g:example-org/tracker.git", "g:o/r.git", "C:o/r.git"} {
		if got := classifyPushDest(in); got != want {
			t.Errorf("classifyPushDest(%q) on GOOS=%s = %v, want %v", in, runtime.GOOS, got, want)
		}
	}
}
