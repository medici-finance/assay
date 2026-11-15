package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// monitor_unarmed_test.go — issue #5890.
//
// On a GitLab-served owner the poller-identity probe went through the GitHub-only transport
// (deskkit.GitHubRoleToken), printed that transport's refusal as a "gh keyring fallback", and the
// pass then exited 0 with the monitor still NOT ARMED — an unwatched surface reported as a clean,
// empty drain. These tests pin both halves of the fix: the probe resolves through the forge-aware
// resolver (deskkit.ResolveRoleCredential), and `run` never exits 0 while nothing is watched or
// while the probe was refused.

// githubOnlyRefusal is the GitHub-only transport's refusal text the probe must never emit for a
// repo the roster maps to GitLab.
const githubOnlyRefusal = "speaks only GitHub"

// withRoleOnly stubs ONLY the session-role seam, so every credential read below runs the
// production resolver path.
func withRoleOnly(t *testing.T, role string) {
	t.Helper()
	old := sessionRoleFn
	sessionRoleFn = func(string) (string, string, error) { return role, "intake-desk", nil }
	t.Cleanup(func() { sessionRoleFn = old })
}

// gitlabRosterHome installs a private HOME whose roster maps example-gl/pilot (and, when asked,
// example-org/tracker) to GitLab, and returns the config dir.
func gitlabRosterHome(t *testing.T, forges string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(deskkit.EnvConfigHome, "")
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(fixtureRoster+"ASSAY_REPO_FORGES="+forges+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
	return dir
}

// githubAppMinter stubs the GitHub App minter so a GitHub owner resolves offline, and records
// every owner it was asked for.
func githubAppMinter(t *testing.T) *[]string {
	t.Helper()
	appToken := filepath.Join(t.TempDir(), "intake-loop-app-token")
	if err := os.WriteFile(appToken, []byte("x-example-installation-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var minted []string
	t.Cleanup(deskkit.SetRoleTokenMinter(func(_, owner string) (string, string, error) {
		minted = append(minted, owner)
		return appToken, "", nil
	}))
	return &minted
}

// regression: #5890
func TestProbeGitLabViaResolver(t *testing.T) {
	dir := gitlabRosterHome(t, "example-gl/pilot=gitlab")
	custody := filepath.Join(dir, "gitlab-intake-loop.token")
	if err := os.WriteFile(custody, []byte("x-example-gitlab-pat\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	minted := githubAppMinter(t)
	withRoleOnly(t, "intake-loop")

	id := ResolveMonitorIdentity([]string{"example-gh/tracker", "example-gl/pilot"})
	var out strings.Builder
	renderMonitorIdentity(&out, id)
	text := out.String()

	if strings.Contains(text, githubOnlyRefusal) {
		t.Fatalf("the probe emitted the GitHub-only transport refusal for a GitLab-served owner:\n%s", text)
	}
	if why, ok := id.Fallbacks["example-gl"]; ok {
		t.Fatalf("the GitLab-served owner is a fallback (%q) though its custody credential resolves", why)
	}
	if !strings.Contains(text, custody) || !strings.Contains(text, "gitlab") {
		t.Fatalf("the probe does not name the GitLab custody credential it resolved (%s):\n%s", custody, text)
	}
	if _, ok := id.TokenFiles["example-gl"]; ok {
		t.Fatalf("a GitLab custody file was handed to the poller as a GitHub installation token file: %v", id.TokenFiles)
	}
	for _, o := range *minted {
		if o == "example-gl" {
			t.Fatalf("the GitHub App minter ran for the GitLab-served owner (minted for %q)", *minted)
		}
	}
	if _, ok := id.TokenFiles["example-gh"]; !ok {
		t.Fatalf("the GitHub owner lost its token file: %v (fallbacks %v)", id.TokenFiles, id.Fallbacks)
	}
	for _, s := range append([]string{text}, id.Fallbacks["example-gh"]) {
		if strings.Contains(s, "x-example-gitlab-pat") || strings.Contains(s, "x-example-installation-token") {
			t.Fatalf("a token value reached the probe's output: %q", s)
		}
	}
}

// silentPoller writes a poller that prints nothing and writes no baseline: the shape of a pass in
// which the monitor never arms.
func silentPoller(t *testing.T, seed bool) string {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash on PATH — execMonitor runs the poller through it")
	}
	body := "#!/usr/bin/env bash\nexit 0\n"
	if seed {
		body = "#!/usr/bin/env bash\nset -eu\nd=\"${INBOUND_MONITOR_STATE_DIR:?}\"\nmkdir -p \"$d\"\n" +
			"for slug in \"$@\"; do case \"$slug\" in --*|*=*) continue;; esac\n" +
			"  printf '\\n' > \"$d/$(printf '%s' \"$slug\" | sed 's#/#__#g').state\"; done\n" +
			"echo \"MONITOR-ARMED: $#\"\n"
	}
	script := filepath.Join(t.TempDir(), "inbound-monitor.sh")
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return script
}

func runPass(t *testing.T, script, stateDir string, extra ...string) (string, error) {
	t.Helper()
	args := append([]string{
		"--root", t.TempDir(),
		"--scan-target", "medici-finance/assay",
		"--worktree-base", t.TempDir(),
		"--state-dir", stateDir,
		"--now", "2026-08-24T12:00:00Z",
	}, extra...)
	if script != "" {
		args = append(args, "--monitor", script)
	}
	var out strings.Builder
	err := cmdRun(args, &out)
	return out.String(), err
}

// regression: #5890
func TestRunUnarmedIsNotClean(t *testing.T) {
	isolatedDeskHome(t)
	old := sessionRoleFn
	sessionRoleFn = func(string) (string, string, error) { return "", "", errors.New("no role in this test") }
	t.Cleanup(func() { sessionRoleFn = old })

	t.Run("live pass that never arms", func(t *testing.T) {
		out, err := runPass(t, silentPoller(t, false), filepath.Join(t.TempDir(), "state"))
		if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
			t.Fatalf("an unarmed monitor exited %d (err %v), want could-not-check %d:\n%s",
				deskkit.ExitCodeOf(err), err, deskkit.ExitUnverifiable, out)
		}
		if !strings.Contains(err.Error(), "NOT ARMED") {
			t.Fatalf("the could-not-check does not say the monitor is unarmed: %v", err)
		}
	})
	t.Run("dry-run that never arms", func(t *testing.T) {
		withDryRunTempBase(t)
		_, err := runPass(t, silentPoller(t, false), filepath.Join(t.TempDir(), "state"), "--dry-run")
		if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
			t.Fatalf("an unarmed dry-run exited %d (err %v), want %d", deskkit.ExitCodeOf(err), err, deskkit.ExitUnverifiable)
		}
	})
	t.Run("offline pass over an unarmed state dir", func(t *testing.T) {
		_, err := runPass(t, "", filepath.Join(t.TempDir(), "state"),
			"--offline", "--inbound", writeTemp(t, "poll.txt", ""))
		if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
			t.Fatalf("an offline pass over an unarmed monitor exited %d (err %v), want %d",
				deskkit.ExitCodeOf(err), err, deskkit.ExitUnverifiable)
		}
	})
	t.Run("a pass that arms is still clean", func(t *testing.T) {
		out, err := runPass(t, silentPoller(t, true), filepath.Join(t.TempDir(), "state"))
		if err != nil {
			t.Fatalf("a pass that seeded every repo must stay exit 0: %v\n%s", err, out)
		}
	})
}

// regression: #5890
func TestRunProbeRefusedIsNotClean(t *testing.T) {
	isolatedDeskHome(t)
	// example-org is served by GitLab and this role has no GitLab custody file: the resolver
	// refuses that owner's credential. The poller still seeds every repo, so the ONLY signal the
	// pass has that it did not read as a resolved identity is the probe's refusal.
	gitlabRosterHome(t, "example-org/tracker=gitlab")
	githubAppMinter(t)
	withRoleOnly(t, "intake-loop")

	out, err := runPass(t, silentPoller(t, true), filepath.Join(t.TempDir(), "state"))
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("a refused poller-identity probe exited %d (err %v), want could-not-check %d:\n%s",
			deskkit.ExitCodeOf(err), err, deskkit.ExitUnverifiable, out)
	}
	if !strings.Contains(err.Error(), "example-org") {
		t.Fatalf("the could-not-check does not name the refused owner: %v", err)
	}
	if strings.Contains(out, githubOnlyRefusal) {
		t.Fatalf("the GitHub-only transport refusal reached the pass output:\n%s", out)
	}
}
