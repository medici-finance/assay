package main

// roletransport_test.go — `deskwt add --role` gives the new worktree the role App's OWN
// transport (#861), proven against REAL git in temp repos.
//
// THE FIELD SHAPE these fixtures reproduce: a shared checkout whose .git/config carries an SSH
// `remote.origin.url` and a deliberate operator sentinel as `remote.origin.pushurl`, on a
// machine whose global config rewrites `https://github.com/` to SSH. A linked worktree cut from
// it inherited all three — its push went nowhere and its fetch authenticated with the
// operator's SSH key — and a worktree-scoped `remote.origin.url` alone did not help: the key is
// multi-valued, so git kept fetching from the FIRST (shared, SSH) value.
//
// Every assertion reads what git ITSELF resolves. The fetch/push transport is proven OFFLINE
// with GIT_ALLOW_PROTOCOL=file: git picks the transport for the URL it resolved, then refuses
// it by name ("transport 'https' not allowed") before opening any connection.
//
// FAIL-FIRST: the committed mutation map cmd/deskwt/transport-mutations.json breaks each
// guarded behaviour (no url reset, no empty-entry reset, no read-back, no credential helper,
// no alias resolution, no :443) and names the test that catches it; `muhar -spec` re-runs it.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	// operatorSentinel is the deliberate "push nowhere" value an operator leaves in a shared
	// checkout's remote.origin.pushurl.
	operatorSentinel = "DISABLED-use-app-token-or-override"
	// sharedSSHOrigin is the shared checkout's SSH fetch url. It never has to resolve: nothing
	// here opens a connection.
	sharedSSHOrigin = "ssh://git@github.com/example-org/tracker.git"
	// wantAppURL is the role App's transport for the fixture repo.
	wantAppURL = "https://github.com:443/example-org/tracker.git"
)

// sshOperatorCheckout turns newRepo's checkout into the field shape: SSH origin url, sentinel
// pushurl, and (in the fixture HOME's global config) the https→SSH insteadOf rewrite.
func sshOperatorCheckout(t *testing.T, work, originURL string) {
	t.Helper()
	mustGit(t, work, "remote", "set-url", "origin", originURL)
	mustGit(t, work, "config", "remote.origin.pushurl", operatorSentinel)
	writeFile(t, filepath.Join(os.Getenv("HOME"), ".gitconfig"),
		"[url \"ssh://git@github.com/\"]\n\tinsteadOf = https://github.com/\n")
}

// gitLines runs git in dir and returns its non-empty stdout lines.
func gitLines(t *testing.T, dir string, args ...string) []string {
	t.Helper()
	var out []string
	for _, l := range strings.Split(mustGit(t, dir, args...), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// offlineTransport runs a real fetch or push in dir with every transport but file disallowed,
// and returns git's refusal — which names the transport git chose for the URL it resolved.
func offlineTransport(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_ALLOW_PROTOCOL=file", "GIT_TERMINAL_PROMPT=0")
	out, _ := cmd.CombinedOutput()
	return string(out)
}

// appTokenHelperShape is the credential helper role-init writes — byte-identical to
// deskkit.AppTokenHelper (#1614), the one shape the preflight's credential-chain check accepts.
func appTokenHelperShape(user, path string) string {
	return "!f(){ echo username=" + user + "; echo \"password=$(cat '" + path + "')\"; }; f"
}

func TestAddRoleWiresAppTransportOverSSHOriginAndSentinel(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	sshOperatorCheckout(t, work, sharedSSHOrigin)
	t.Setenv("DESK_LOOP", "worker-desk") // a bot session: the no-role add would refuse this checkout

	// The field symptom, reproduced on a plain linked worktree first: a worktree-scoped url
	// appended WITHOUT the empty-entry reset does not move fetch off the shared SSH value.
	plain := filepath.Join(t.TempDir(), "plain")
	mustGit(t, work, "worktree", "add", "--detach", plain, "HEAD")
	mustGit(t, plain, "config", "extensions.worktreeConfig", "true")
	mustGit(t, plain, "config", "--worktree", "--add", "remote.origin.url", wantAppURL)
	if got := gitLines(t, plain, "remote", "get-url", "origin"); len(got) != 1 || got[0] != sharedSSHOrigin {
		t.Fatalf("fixture drift: a worktree-scoped url without a reset should still fetch from %s, got %v", sharedSSHOrigin, got)
	}

	rc, stderr := runCapErr(t, []string{"add", "apptransport", "--role", "worker"})
	if rc != deskkit.ExitOK {
		t.Fatalf("add --role worker rc = %d, want 0; stderr:\n%s", rc, stderr)
	}
	target := filepath.Join(tmpBaseDir, "tracker-apptransport")

	// Exactly ONE push url and ONE fetch url, both the App's https transport.
	if got := gitLines(t, target, "remote", "get-url", "--push", "--all", "origin"); len(got) != 1 || got[0] != wantAppURL {
		t.Fatalf("push urls = %v, want exactly [%s] (the sentinel must be reset away, not pushed to alongside)", got, wantAppURL)
	}
	if got := gitLines(t, target, "remote", "get-url", "--all", "origin"); len(got) != 1 || got[0] != wantAppURL {
		t.Fatalf("fetch urls = %v, want exactly [%s]", got, wantAppURL)
	}
	// And git's transport choice agrees: https for fetch and push in the worktree ...
	if out := offlineTransport(t, target, "fetch", "origin"); !strings.Contains(out, "transport 'https' not allowed") {
		t.Fatalf("fetch in the role worktree did not select https:\n%s", out)
	}
	if out := offlineTransport(t, target, "push", "--dry-run", "origin", "HEAD:refs/heads/probe"); !strings.Contains(out, "transport 'https' not allowed") {
		t.Fatalf("push in the role worktree did not select https:\n%s", out)
	}
	// ... while the SHARED checkout is untouched: still SSH, still the sentinel.
	if out := offlineTransport(t, work, "fetch", "origin"); !strings.Contains(out, "transport 'ssh' not allowed") {
		t.Fatalf("the shared checkout's fetch transport changed (it must never be touched):\n%s", out)
	}
	if got := gitLines(t, work, "remote", "get-url", "--push", "--all", "origin"); len(got) != 1 || got[0] != operatorSentinel {
		t.Fatalf("the shared checkout's push url = %v, want the operator's sentinel untouched", got)
	}

	// The credential helper: the AppTokenHelper shape, host-scoped, and git's own chain answers
	// the App URL with the role's token.
	helpers := gitLines(t, target, "config", "--worktree", "--get-all", "credential.https://github.com.helper")
	tokenPath := filepath.Join(os.Getenv("HOME"), "worker-token-fixture")
	if len(helpers) != 1 || helpers[0] != appTokenHelperShape("x-access-token", tokenPath) {
		t.Fatalf("worktree credential helper = %q, want exactly the App token helper over %s", helpers, tokenPath)
	}
	cmd := exec.Command("git", "-C", target, "credential", "fill")
	cmd.Stdin = strings.NewReader("url=" + wantAppURL + "\n\n")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/usr/bin/false", "SSH_ASKPASS=")
	fill, _ := cmd.CombinedOutput()
	if !strings.Contains(string(fill), "username=x-access-token") || !strings.Contains(string(fill), "password="+fixtureTokenValue) {
		t.Fatalf("git credential fill for %s did not answer with the worker App token:\n%s", wantAppURL, fill)
	}
}

// An SSH host ALIAS (a ~/.ssh/config Host block) is resolved the way ssh resolves it, so the
// https URL names the real host rather than an alias no https client can reach.
func TestAddRoleResolvesSSHHostAlias(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	sshOperatorCheckout(t, work, "git@corp-alias:example-org/tracker.git")
	var asked []string
	old := sshConfigHostname
	sshConfigHostname = func(alias string) (string, error) { asked = append(asked, alias); return "github.com", nil }
	t.Cleanup(func() { sshConfigHostname = old })

	if rc, stderr := runCapErr(t, []string{"add", "aliased", "--role", "verifier", "--detach"}); rc != deskkit.ExitOK {
		t.Fatalf("add --role verifier over an SSH alias rc = %d, want 0; stderr:\n%s", rc, stderr)
	}
	if len(asked) != 1 || asked[0] != "corp-alias" {
		t.Fatalf("ssh config lookups = %v, want exactly [corp-alias]", asked)
	}
	target := filepath.Join(tmpBaseDir, "tracker-aliased")
	if got := gitLines(t, target, "remote", "get-url", "--all", "origin"); len(got) != 1 || got[0] != wantAppURL {
		t.Fatalf("fetch urls = %v, want exactly [%s]", got, wantAppURL)
	}
}

// An alias that resolves to no qualified host cannot be given an https transport: REFUSED,
// named, and the worktree rolled back — never handed out on the inherited SSH transport.
func TestAddRoleRefusesUnresolvableAlias(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	sshOperatorCheckout(t, work, "git@corp-alias:example-org/tracker.git")
	old := sshConfigHostname
	sshConfigHostname = func(alias string) (string, error) { return alias, nil } // no HostName in the Host block
	t.Cleanup(func() { sshConfigHostname = old })

	rc, stderr := runCapErr(t, []string{"add", "noalias", "--role", "worker"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want 5 (refused); stderr:\n%s", rc, stderr)
	}
	for _, frag := range []string{"could not give", "App-only transport", "ROLLED BACK", "corp-alias"} {
		if !strings.Contains(stderr, frag) {
			t.Errorf("refusal does not carry %q:\n%s", frag, stderr)
		}
	}
	if _, serr := os.Lstat(filepath.Join(tmpBaseDir, "tracker-noalias")); !os.IsNotExist(serr) {
		t.Fatalf("the worktree survived the refusal (stat err = %v)", serr)
	}
}

// The read-back is the fail-closed layer: when git does NOT resolve exactly the written URL —
// here a global insteadOf that rewrites the :443 URL itself back to SSH — the add is refused and
// rolled back rather than trusting that the config write took.
func TestAddRoleRefusesWhenGitResolvesAnotherTransport(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	sshOperatorCheckout(t, work, sharedSSHOrigin)
	writeFile(t, filepath.Join(os.Getenv("HOME"), ".gitconfig"),
		"[url \"ssh://git@github.com/\"]\n\tinsteadOf = https://github.com:443/\n")

	rc, stderr := runCapErr(t, []string{"add", "rewritten", "--role", "worker"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want 5 (refused); stderr:\n%s", rc, stderr)
	}
	if !strings.Contains(stderr, "git still resolves") || !strings.Contains(stderr, "ssh://git@github.com/") {
		t.Fatalf("refusal does not name what git resolved:\n%s", stderr)
	}
	if _, serr := os.Lstat(filepath.Join(tmpBaseDir, "tracker-rewritten")); !os.IsNotExist(serr) {
		t.Fatalf("the worktree survived the refusal (stat err = %v)", serr)
	}
}

// A local-path fetch url carries no key, but it does not make an SSH pushurl safe.
func TestAddRoleRefusesSSHPushBehindLocalFetch(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	mustGit(t, work, "config", "remote.origin.pushurl", sharedSSHOrigin)

	rc, stderr := runCapErr(t, []string{"add", "localfetch", "--role", "worker"})
	if rc != deskkit.ExitRefused || !strings.Contains(stderr, "over SSH") {
		t.Fatalf("rc = %d (want 5 naming the SSH push); stderr:\n%s", rc, stderr)
	}
	if _, serr := os.Lstat(filepath.Join(tmpBaseDir, "tracker-localfetch")); !os.IsNotExist(serr) {
		t.Fatalf("the worktree survived the refusal (stat err = %v)", serr)
	}
}

// The verifier case from the issue thread: a bot session (verify-desk) cutting a detached
// verifier worktree off a checkout whose ONLY url is SSH. The no-role add refuses that
// checkout outright; with --role the worktree gets the verifier App's https transport instead
// and the add succeeds.
func TestAddRoleReplacesInheritedSSHPushUnderBotLoop(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	mustGit(t, work, "remote", "set-url", "origin", sharedSSHOrigin)
	t.Setenv("DESK_LOOP", "verify-desk")

	if rc, stderr := runCapErr(t, []string{"add", "verifyssh"}); rc != deskkit.ExitRefused {
		t.Fatalf("fixture drift: the no-role add over an SSH origin under a bot loop rc = %d, want 5; stderr:\n%s", rc, stderr)
	}
	rc, stderr := runCapErr(t, []string{"add", "verifyssh", "--detach", "--role", "verifier"})
	if rc != deskkit.ExitOK {
		t.Fatalf("add --detach --role verifier rc = %d, want 0; stderr:\n%s", rc, stderr)
	}
	target := filepath.Join(tmpBaseDir, "tracker-verifyssh")
	if got := gitLines(t, target, "remote", "get-url", "--push", "--all", "origin"); len(got) != 1 || got[0] != wantAppURL {
		t.Fatalf("push urls = %v, want exactly [%s]", got, wantAppURL)
	}
	if out := offlineTransport(t, target, "fetch", "origin"); !strings.Contains(out, "transport 'https' not allowed") {
		t.Fatalf("fetch in the verifier worktree did not select https:\n%s", out)
	}
}
