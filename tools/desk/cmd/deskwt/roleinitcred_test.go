package main

// roleinitcred_test.go — FAIL-FIRST coverage for role-init's credential wiring + preflight
// (#1309 item 7).
//
// THE DEFECT. `deskwt role-init --repo-root <sibling>` set the worktree-scoped commit IDENTITY
// but not the worktree-scoped CREDENTIAL HELPER it knows how to mint, so a sibling root's first
// https fetch fell through to whatever the shared config carried and the preflight went RED
// ("could not read Username for 'https://github.com'") — and that root's whole queue was
// invisible. These pin: the helper chain is reset at WORKTREE scope and one inline helper
// reading the role's token file is added (proven by `git credential fill` answering with the
// role's token through git's own chain); a stale helper in the shared config is shadowed in the
// worktree and left untouched in the primary; the token value never reaches argv; the reuse
// path re-wires; and the preflight runs LAST, against the worktree, red = exit 6.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// credentialFill asks git's OWN credential chain, in dir, what it would send for an https
// host — the exact question a fetch asks. It returns the filled username/password lines.
func credentialFill(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "credential", "fill")
	cmd.Stdin = strings.NewReader("protocol=https\nhost=example.invalid\n\n")
	// GIT_TERMINAL_PROMPT=0: never fall through to a tty prompt when no helper answers.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/usr/bin/false", "SSH_ASKPASS=")
	out, _ := cmd.CombinedOutput()
	return string(out)
}

func TestRoleInitWiresWorktreeScopedCredentialHelper(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	// The shared config carries a STALE helper from a sibling role — the pollution shape.
	mustGit(t, work, "config", "credential.helper", "!f(){ echo username=stale; echo password=STALE-SIBLING; }; f")

	rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "cred1"})
	if rc != deskkit.ExitOK {
		t.Fatalf("role-init rc = %d, want 0; stderr: %s", rc, stderr)
	}
	target := filepath.Join(tmpBaseDir, "tracker-verify-desk-cred1")

	// Worktree-scoped: the chain is reset then ONE inline helper reading the token FILE.
	raw, err := exec.Command("git", "-C", target, "config", "--worktree", "--get-all", "-z", "credential.helper").Output()
	if err != nil {
		t.Fatalf("get-all credential.helper: %v", err)
	}
	helpers := string(raw)
	lines := strings.Split(strings.TrimSuffix(helpers, "\x00"), "\x00")
	if len(lines) != 2 || lines[0] != "" || !strings.HasPrefix(lines[1], "!f(){ echo username=x-access-token; echo \"password=$(cat '") {
		t.Fatalf("worktree-scoped credential.helper = %q; want a reset (empty) then one inline token-file helper", helpers)
	}
	if strings.Contains(helpers, fixtureTokenValue) {
		t.Fatalf("the token VALUE was written into config: %q", helpers)
	}
	for _, c := range gitCalls(*calls) {
		if strings.Contains(strings.Join(c, " "), fixtureTokenValue) {
			t.Fatalf("the token VALUE reached a git argv: %v", c)
		}
	}
	// git's own chain, asked in the worktree, answers with the ROLE's token — not the stale one.
	fill := credentialFill(t, target)
	if !strings.Contains(fill, "username=x-access-token") || !strings.Contains(fill, "password="+fixtureTokenValue) {
		t.Fatalf("git credential fill in the worktree did not answer with the role token:\n%s", fill)
	}
	if strings.Contains(fill, "STALE-SIBLING") {
		t.Fatalf("the shared config's stale helper still answers in the worktree:\n%s", fill)
	}
	// The primary checkout is untouched: its stale helper still answers THERE.
	if fill := credentialFill(t, work); !strings.Contains(fill, "STALE-SIBLING") {
		t.Fatalf("role-init mutated the shared checkout's credential chain:\n%s", fill)
	}
	if got := mustGit(t, "", "config", "--file", filepath.Join(work, ".git", "config"), "--get-all", "credential.helper"); strings.Contains(got, "x-access-token") {
		t.Fatalf("the role helper leaked into the SHARED .git/config: %q", got)
	}
}

// The reuse (idempotent) path re-wires the helper, so a worktree polluted since its creation
// is scrubbed on the next role-init rather than left to 401.
func TestRoleInitReuseRewiresCredentialHelper(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	if rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "cred2"}); rc != deskkit.ExitOK {
		t.Fatalf("first role-init rc = %d; stderr: %s", rc, stderr)
	}
	target := filepath.Join(tmpBaseDir, "tracker-verify-desk-cred2")
	// Pollute the worktree scope itself (a hand-rolled fix gone wrong).
	mustGit(t, target, "config", "--worktree", "--replace-all", "credential.helper", "!f(){ echo password=POLLUTED; }; f")
	if rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "cred2"}); rc != deskkit.ExitOK {
		t.Fatalf("reuse role-init rc = %d; stderr: %s", rc, stderr)
	}
	if fill := credentialFill(t, target); strings.Contains(fill, "POLLUTED") || !strings.Contains(fill, "password="+fixtureTokenValue) {
		t.Fatalf("reuse did not re-wire the helper:\n%s", fill)
	}
}

// The preflight runs LAST, against the provisioned worktree, as the role, on the repo — and a
// red preflight is exit 6 with the path still printed (the worktree IS provisioned).
func TestRoleInitRunsPreflightAgainstTheWorktree(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
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
	rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "pf"})
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
	// The helper was wired BEFORE the preflight (the preflight proves the wiring).
	if fill := credentialFill(t, target); !strings.Contains(fill, "password="+fixtureTokenValue) {
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
