package deskkit

// pushtransport_rewrite_test.go — the push-transport gate against git's url rewrites (#884).
//
// These tests drive REAL git, not a config fixture: the defect is that the gate read the
// configured URL string while git pushes to the URL it gets after applying
// url.<base>.insteadOf / url.<base>.pushInsteadOf. A fixture that encodes this code's own
// idea of the rewrite would test the reimplementation against itself; asking git is the
// only oracle that cannot share the bug. No remote is contacted — `remote get-url` and
// `config --list` read local config only.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// rewriteRepo is a throwaway repository whose git config is isolated from the host's
// (no system or global config), so a rewrite rule on the machine running the test cannot
// leak into — or rescue — the case under test.
func rewriteRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	tmp := t.TempDir()
	global := filepath.Join(tmp, "global.gitconfig")
	if err := os.WriteFile(global, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "xdg"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	for _, k := range []string{"GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT", "GIT_DIR", "GIT_WORK_TREE"} {
		unsetForTest(t, k)
	}
	dir := filepath.Join(tmp, "repo")
	rewriteGit(t, "init", "-q", dir)
	rewriteGit(t, "-C", dir, "config", "gc.auto", "0")
	return dir
}

func rewriteGit(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// realGitGateInput wires the gate the way deskpr and deskwt do: both readers are git
// itself, run in the worktree.
func realGitGateInput(dir string) PushTransportInput {
	run := func(args ...string) (string, error) {
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
		return string(out), err
	}
	return PushTransportInput{
		Tool: "deskpr", Verb: "create", Dir: dir, Remote: "origin",
		ConfigZ:  func() (string, error) { return run("config", "--list", "-z") },
		PushURLs: func() (string, error) { return run("remote", "get-url", "--push", "--all", "origin") },
	}
}

const rewriteHTTPS = "https://example.com/example-org/tracker.git"

// TestPushTransportRefusesInsteadOfRewrittenToSSH: the configured push URL is https, and
// url.<base>.insteadOf rewrites it to SSH before git connects. The gate must refuse — and
// name the rule — rather than pass the https string it was handed.
//
// FAIL-FIRST: on the unfixed gate (which decides from remote.origin.url as configured) this
// returns nil — the false pass #884 reports.
func TestPushTransportRefusesInsteadOfRewrittenToSSH(t *testing.T) {
	t.Setenv("DESK_LOOP", "worker-desk")
	dir := rewriteRepo(t)
	rewriteGit(t, "-C", dir, "config", "remote.origin.url", rewriteHTTPS)
	rewriteGit(t, "-C", dir, "config", "url.git@example.com:.insteadOf", "https://example.com/")

	// Oracle check: the fixture really does make git push over SSH.
	if got := strings.TrimSpace(rewriteGit(t, "-C", dir, "remote", "get-url", "--push", "origin")); got != "git@example.com:example-org/tracker.git" {
		t.Fatalf("fixture: git resolves the push url to %q, want the SSH rewrite", got)
	}

	err := CheckPushTransport(realGitGateInput(dir))
	if err == nil || ExitCodeOf(err) != ExitRefused {
		t.Fatalf("err = %v (exit %d), want a refusal (exit %d): git will push this over SSH",
			err, ExitCodeOf(err), ExitRefused)
	}
	for _, frag := range []string{
		"SSH transport",
		"worker App",
		"url.git@example.com:.insteadOf", // the rule, in git's own spelling
		rewriteHTTPS,                     // the configured URL it rewrote
		"git@example.com:example-org/tracker.git", // what git will actually push to
		"does NOT escape it",                      // the remedy is the rule, not a pushurl
	} {
		if !strings.Contains(err.Error(), frag) {
			t.Errorf("refusal is missing %q:\n%s", frag, err.Error())
		}
	}

	// insteadOf rewrites an explicit pushurl too, so an https pushurl override does not
	// clear it — the gate must not take the pushurl at face value either.
	t.Run("explicit https pushurl is still rewritten", func(t *testing.T) {
		rewriteGit(t, "-C", dir, "config", "remote.origin.pushurl", rewriteHTTPS)
		err := CheckPushTransport(realGitGateInput(dir))
		if err == nil || ExitCodeOf(err) != ExitRefused {
			t.Fatalf("err = %v (exit %d), want a refusal: insteadOf applies to pushurl values", err, ExitCodeOf(err))
		}
		if !strings.Contains(err.Error(), "remote.origin.pushurl") ||
			!strings.Contains(err.Error(), "url.git@example.com:.insteadOf") {
			t.Errorf("refusal should trace remote.origin.pushurl through the insteadOf rule:\n%s", err.Error())
		}
	})
}

// TestPushTransportRefusesPushInsteadOfRewrittenToSSH: url.<base>.pushInsteadOf rewrites
// only the PUSH url — fetch stays https, so `remote -v`'s first line and a fetch both look
// clean while every push leaves over SSH. The gate must refuse and name the rule.
//
// FAIL-FIRST: on the unfixed gate this returns nil.
func TestPushTransportRefusesPushInsteadOfRewrittenToSSH(t *testing.T) {
	t.Setenv("DESK_LOOP", "worker-desk")
	dir := rewriteRepo(t)
	rewriteGit(t, "-C", dir, "config", "remote.origin.url", rewriteHTTPS)
	rewriteGit(t, "-C", dir, "config", "url.ssh://git@example.com/.pushInsteadOf", "https://example.com/")

	if got := strings.TrimSpace(rewriteGit(t, "-C", dir, "remote", "get-url", "origin")); got != rewriteHTTPS {
		t.Fatalf("fixture: fetch url = %q, want it untouched by a push-only rule", got)
	}
	if got := strings.TrimSpace(rewriteGit(t, "-C", dir, "remote", "get-url", "--push", "origin")); got != "ssh://git@example.com/example-org/tracker.git" {
		t.Fatalf("fixture: git resolves the push url to %q, want the SSH rewrite", got)
	}

	err := CheckPushTransport(realGitGateInput(dir))
	if err == nil || ExitCodeOf(err) != ExitRefused {
		t.Fatalf("err = %v (exit %d), want a refusal (exit %d): git will push this over SSH",
			err, ExitCodeOf(err), ExitRefused)
	}
	for _, frag := range []string{
		"SSH transport",
		"url.ssh://git@example.com/.pushInsteadOf",
		"ssh://git@example.com/example-org/tracker.git",
		"remote set-url --push origin " + rewriteHTTPS, // an explicit pushurl clears a push-only rule
	} {
		if !strings.Contains(err.Error(), frag) {
			t.Errorf("refusal is missing %q:\n%s", frag, err.Error())
		}
	}

	// The remedy the refusal names really clears it: git never pushInsteadOf-rewrites an
	// explicit pushurl, and the gate must agree rather than over-refuse.
	t.Run("the named remedy clears it", func(t *testing.T) {
		rewriteGit(t, "-C", dir, "remote", "set-url", "--push", "origin", rewriteHTTPS)
		if err := CheckPushTransport(realGitGateInput(dir)); ExitCodeOf(err) == ExitRefused {
			t.Fatalf("an explicit https pushurl is not pushInsteadOf-rewritten, yet the gate refused:\n%v", err)
		}
	})
}
