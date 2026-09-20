package main

// roleinitcred_test.go — FAIL-FIRST coverage for role-init's credential wiring + preflight
// (#1309 item 7), including the CREDENTIAL-CUSTODY fix from the #1374 security review.
//
// THE ORIGINAL DEFECT (item 7). `deskwt role-init --repo-root <sibling>` set the worktree-scoped
// commit IDENTITY but not the worktree-scoped CREDENTIAL HELPER it knows how to mint, so a
// sibling root's first https fetch fell through to whatever the shared config carried and the
// preflight went RED ("could not read Username for 'https://github.com'") — and that root's
// whole queue was invisible.
//
// THE SECURITY DEFECT (found on #1374, fixed here). The first cut of that fix installed the
// inline token helper under the UNSCOPED `credential.helper` key. That key answers for EVERY
// host over EVERY protocol, and the inline helper ignores git's stdin request (it emits a fixed
// username/password), so `git credential fill` returned the role's App token for ANY host — a
// foreign https host AND plaintext http on the real host all received it. The remedy: install
// the helper under `credential.https://<origin-host>.helper`, host RESOLVED FROM THE ORIGIN, so
// git offers the token ONLY on https to that exact host and nothing else matches.
//
// These pin, after the fix: the helper lives under the HOST-SCOPED key (never the unscoped one);
// the origin host over https answers with the role token; a FOREIGN host and PLAINTEXT HTTP on
// the real host get NOTHING ("could not read Username"); the unscoped chain is still reset so a
// stale sibling helper in shared config is shadowed in the worktree and left untouched in the
// primary; the token value never reaches argv or config; the reuse path re-wires; and the
// preflight runs LAST, against the worktree, red = exit 6.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// fixtureOriginHost is the https host the credential tests give the fixture origin, so role-init
// has a real host to SCOPE its helper to. newRepo's default origin is a hostless local path
// (git's local transport, no credential helper), which is a legitimate skip case — but to prove
// the scoping we need a host-bearing origin.
const fixtureOriginHost = "github.com"

// giveOriginHost repoints work's origin at an https URL bearing fixtureOriginHost while keeping
// the same owner/name slug the fixture roster allows (example-org/tracker). The URL is
// unreachable, so a role-init that uses it must pass --no-fetch: the worktree is cut from the
// local refs/remotes/origin/main newRepo already planted, and the origin URL is read only to
// resolve the host the credential helper is scoped to.
func giveOriginHost(t *testing.T, work string) {
	t.Helper()
	mustGit(t, work, "remote", "set-url", "origin", "https://"+fixtureOriginHost+"/example-org/tracker.git")
}

// credentialFill asks git's OWN credential chain, in dir, what it would send for a given
// protocol+host — the exact question a fetch/push to that URL asks. It returns the combined
// output (stdout carries a filled `password=` line; stderr carries "could not read Username"
// when no helper answers and prompts are disabled).
func credentialFill(t *testing.T, dir, protocol, host string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "credential", "fill")
	cmd.Stdin = strings.NewReader("protocol=" + protocol + "\nhost=" + host + "\n\n")
	// GIT_TERMINAL_PROMPT=0 / GIT_ASKPASS=false: never fall through to a tty or askpass prompt
	// when no helper answers — an unanswered request must FAIL loudly, not block.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/usr/bin/false", "SSH_ASKPASS=")
	out, _ := cmd.CombinedOutput()
	return string(out)
}

func TestRoleInitWiresHostScopedCredentialHelper(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	giveOriginHost(t, work)
	// The shared config carries a STALE helper from a sibling role — the pollution shape.
	mustGit(t, work, "config", "credential.helper", "!f(){ echo username=stale; echo password=STALE-SIBLING; }; f")

	rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "cred1", "--no-fetch"})
	if rc != deskkit.ExitOK {
		t.Fatalf("role-init rc = %d, want 0; stderr: %s", rc, stderr)
	}
	target := filepath.Join(tmpBaseDir, "tracker-verify-desk-cred1")
	scopedKey := "credential.https://" + fixtureOriginHost + ".helper"

	// The token helper lives under the HOST-SCOPED key, reading the token FILE.
	scoped, err := exec.Command("git", "-C", target, "config", "--worktree", "--get-all", "-z", scopedKey).Output()
	if err != nil {
		t.Fatalf("get-all %s: %v", scopedKey, err)
	}
	scopedHelpers := strings.Split(strings.TrimSuffix(string(scoped), "\x00"), "\x00")
	if len(scopedHelpers) != 1 || !strings.HasPrefix(scopedHelpers[0], "!f(){ echo username=x-access-token; echo \"password=$(cat '") {
		t.Fatalf("%s = %q; want exactly one inline token-file helper", scopedKey, string(scoped))
	}

	// The UNSCOPED key carries ONLY the reset (a single empty value) — never the token helper.
	// This is the crux of the fix: the leaky code put the helper HERE.
	unscoped, uerr := exec.Command("git", "-C", target, "config", "--worktree", "--get-all", "-z", "credential.helper").Output()
	if uerr != nil {
		t.Fatalf("get-all credential.helper: %v", uerr)
	}
	unscopedHelpers := strings.Split(strings.TrimSuffix(string(unscoped), "\x00"), "\x00")
	if len(unscopedHelpers) != 1 || unscopedHelpers[0] != "" {
		t.Fatalf("unscoped credential.helper = %q; want a single empty reset value and NOTHING else", string(unscoped))
	}
	if strings.Contains(string(unscoped), "x-access-token") || strings.Contains(string(unscoped), fixtureTokenValue) {
		t.Fatalf("the token helper is installed under the UNSCOPED key — the leak this fix removes: %q", string(unscoped))
	}

	// The token VALUE is nowhere in config, and never reached a git argv (only its PATH does).
	if strings.Contains(string(scoped), fixtureTokenValue) {
		t.Fatalf("the token VALUE was written into config: %q", string(scoped))
	}
	for _, c := range gitCalls(*calls) {
		if strings.Contains(strings.Join(c, " "), fixtureTokenValue) {
			t.Fatalf("the token VALUE reached a git argv: %v", c)
		}
	}

	// The origin host over HTTPS answers with the ROLE's token — not the stale one.
	if fill := credentialFill(t, target, "https", fixtureOriginHost); !strings.Contains(fill, "username=x-access-token") ||
		!strings.Contains(fill, "password="+fixtureTokenValue) {
		t.Fatalf("git credential fill for https://%s did not answer with the role token:\n%s", fixtureOriginHost, fill)
	} else if strings.Contains(fill, "STALE-SIBLING") {
		t.Fatalf("the shared config's stale helper still answers in the worktree:\n%s", fill)
	}

	// A FOREIGN https host gets NOTHING — not the role token, not the stale sibling helper.
	if fill := credentialFill(t, target, "https", "example.invalid"); strings.Contains(fill, fixtureTokenValue) ||
		strings.Contains(fill, "STALE-SIBLING") {
		t.Fatalf("a foreign host received a credential — the token leaks to any host:\n%s", fill)
	} else if !strings.Contains(fill, "could not read Username") {
		t.Fatalf("a foreign host was not denied a credential (want \"could not read Username\"):\n%s", fill)
	}

	// PLAINTEXT HTTP on the REAL host gets NOTHING — the scope pins scheme=https.
	if fill := credentialFill(t, target, "http", fixtureOriginHost); strings.Contains(fill, fixtureTokenValue) {
		t.Fatalf("plaintext http on the real host received the token — the scope does not pin https:\n%s", fill)
	} else if !strings.Contains(fill, "could not read Username") {
		t.Fatalf("plaintext http on the real host was not denied a credential:\n%s", fill)
	}

	// The primary checkout is untouched: its stale helper still answers THERE.
	if fill := credentialFill(t, work, "https", fixtureOriginHost); !strings.Contains(fill, "STALE-SIBLING") {
		t.Fatalf("role-init mutated the shared checkout's credential chain:\n%s", fill)
	}
	if got := mustGit(t, "", "config", "--file", filepath.Join(work, ".git", "config"), "--get-all", "credential.helper"); strings.Contains(got, "x-access-token") {
		t.Fatalf("the role helper leaked into the SHARED .git/config: %q", got)
	}
	if got := mustGit(t, "", "config", "--file", filepath.Join(work, ".git", "config"), "--list"); strings.Contains(got, scopedKey) {
		t.Fatalf("the host-scoped role helper leaked into the SHARED .git/config: %q", got)
	}
}

// The reuse (idempotent) path re-wires the helper, so a worktree polluted since its creation is
// scrubbed on the next role-init rather than left to answer with the wrong credential.
func TestRoleInitReuseRewiresCredentialHelper(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	giveOriginHost(t, work)
	scopedKey := "credential.https://" + fixtureOriginHost + ".helper"
	if rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "cred2", "--no-fetch"}); rc != deskkit.ExitOK {
		t.Fatalf("first role-init rc = %d; stderr: %s", rc, stderr)
	}
	target := filepath.Join(tmpBaseDir, "tracker-verify-desk-cred2")
	// Pollute the host-scoped key itself (a hand-rolled fix gone wrong).
	mustGit(t, target, "config", "--worktree", "--replace-all", scopedKey, "!f(){ echo password=POLLUTED; }; f")
	if rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "cred2", "--no-fetch"}); rc != deskkit.ExitOK {
		t.Fatalf("reuse role-init rc = %d; stderr: %s", rc, stderr)
	}
	if fill := credentialFill(t, target, "https", fixtureOriginHost); strings.Contains(fill, "POLLUTED") ||
		!strings.Contains(fill, "password="+fixtureTokenValue) {
		t.Fatalf("reuse did not re-wire the helper:\n%s", fill)
	}
}

// A hostless origin (local filesystem path — a fixture or a directory clone) is served by git's
// local transport, which consults no credential helper. role-init wires NOTHING (never falls
// back to the leaky unscoped key) and still succeeds — the fail-safe branch of the fix.
func TestRoleInitHostlessOriginWiresNoCredentialHelper(t *testing.T) {
	work := newRepo(t) // origin is a local bare PATH, no host
	withEnv(t, work)
	rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "local"})
	if rc != deskkit.ExitOK {
		t.Fatalf("role-init on a hostless origin rc = %d, want 0; stderr: %s", rc, stderr)
	}
	target := filepath.Join(tmpBaseDir, "tracker-verify-desk-local")
	// No token helper anywhere in the worktree config — not under any key.
	if got := mustGit(t, target, "config", "--worktree", "--list"); strings.Contains(got, "x-access-token") || strings.Contains(got, fixtureTokenValue) {
		t.Fatalf("a credential helper was wired for a hostless (local) origin:\n%s", got)
	}
	if !strings.Contains(stderr, "no https host") {
		t.Fatalf("the skip was not announced on stderr:\n%s", stderr)
	}
}

// The preflight runs LAST, against the provisioned worktree, as the role, on the repo — and a
// red preflight is exit 6 with the path still printed (the worktree IS provisioned).
func TestRoleInitRunsPreflightAgainstTheWorktree(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	giveOriginHost(t, work)
	var got deskkit.PreflightRequest
	ran := 0
	roleInitPreflight = func(req deskkit.PreflightRequest) error {
		got = req
		ran++
		return deskkit.Unverifiable("preflight: 1/5 red — write-transport could-not-check", nil)
	}
	target := filepath.Join(tmpBaseDir, "tracker-verify-desk-pf")

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "pf", "--no-fetch"})
	_ = w.Close()
	os.Stdout = oldStdout
	outb := make([]byte, 4096)
	n, _ := r.Read(outb)
	stdout := string(outb[:n])

	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("role-init with a red preflight rc = %d, want 6; stderr: %s", rc, stderr)
	}
	if ran != 1 {
		t.Fatalf("preflight ran %d times, want exactly once", ran)
	}
	if got.Role != "verifier" || got.Root != target || got.Landing.Dir != target || got.Landing.Remote != "origin" || got.Repo == "" {
		t.Fatalf("preflight request = %+v; want role verifier, root/landing = the worktree, repo set", got)
	}
	if !strings.Contains(stderr, "write-transport could-not-check") {
		t.Fatalf("the red preflight report did not reach stderr: %s", stderr)
	}
	if strings.TrimSpace(stdout) != target {
		t.Fatalf("stdout = %q; the provisioned path must still be printed", stdout)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("worktree must remain provisioned after a red preflight: %v", err)
	}
	// The helper was wired BEFORE the preflight (the preflight proves the wiring), scoped to the
	// origin host.
	if fill := credentialFill(t, target, "https", fixtureOriginHost); !strings.Contains(fill, "password="+fixtureTokenValue) {
		t.Fatalf("helper not wired before the preflight:\n%s", fill)
	}
}

// A mint failure is could-not-check (exit 6) naming the role, never a token, and never a
// worktree that reads as ready.
func TestRoleInitMintFailureIsUnverifiable(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	pfRan := false
	roleInitPreflight = func(deskkit.PreflightRequest) error { pfRan = true; return nil }
	roleTokenPath = func(role, owner string) (string, error) {
		return "", deskkit.Unverifiable("cannot mint the "+role+" App installation token for "+owner+" (no App ID)", nil)
	}
	rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "mint"})
	if rc != deskkit.ExitUnverifiable || !strings.Contains(stderr, "cannot mint the verifier App installation token") {
		t.Fatalf("rc = %d (want 6), stderr: %s", rc, stderr)
	}
	if pfRan {
		t.Fatalf("the preflight must not run when the credential could not be wired")
	}
}
