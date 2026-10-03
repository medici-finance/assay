package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestRoleInitRefreshesCredentialBeforeItsFirstFetch(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	giveOriginHost(t, work)
	refreshed, fetched := false, false
	oldCred := roleCredential
	roleCredential = func(role string, repo deskkit.ForgeRepo, origin string) (deskkit.RoleCredential, error) {
		refreshed = true
		return oldCred(role, repo, origin)
	}
	oldExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		for _, arg := range args {
			if name == "git" && arg == "fetch" {
				if !refreshed {
					t.Fatal("role-init fetched before resolving its role credential")
				}
				fetched = true
				// Keep the integration offline; the fixture already has origin/main.
				return exec.Command("git", "rev-parse", "--verify", "origin/main")
			}
		}
		return oldExec(name, args...)
	}
	if rc, stderr := runCapErr(t, []string{"role-init", "verifier", "--session", "fresh-auth"}); rc != 0 {
		t.Fatalf("role-init failed: %d %s", rc, stderr)
	}
	if !fetched {
		t.Fatal("role-init skipped its fresh fetch")
	}
}

func TestRoleFetchRefreshesBeforeFetchAndOverridesSourceCredentials(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	giveOriginHost(t, work)
	mustGit(t, work, "config", "credential.helper", "!f(){ echo username=stale; echo password=STALE-SIBLING; }; f")
	mustGit(t, work, "config", "credential.https://github.com.helper", "!f(){ echo username=stale; echo password=STALE-SCOPED; }; f")
	configPath := filepath.Join(work, ".git", "config")
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	minted := false
	oldCred := roleCredential
	roleCredential = func(role string, repo deskkit.ForgeRepo, origin string) (deskkit.RoleCredential, error) {
		if role != "verifier" || repo.Slug() != "example-org/tracker" || origin != "https://github.com/example-org/tracker.git" {
			t.Fatalf("wrong credential requested: role=%s repo=%s origin=%s", role, repo.Slug(), origin)
		}
		minted = true
		return oldCred(role, repo, origin)
	}
	var prefix []string
	oldExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		for i, arg := range args {
			if name == "git" && arg == "fetch" {
				if !minted {
					t.Fatal("fetch ran before refreshing the role credential")
				}
				prefix = append([]string{}, args[:i]...)
				if got := strings.Join(args[i:], " "); got != "fetch --no-tags --no-recurse-submodules https://github.com/example-org/tracker.git +refs/heads/main:refs/remotes/origin/main" {
					t.Fatalf("unexpected fetch: %s", got)
				}
				// Exercise git's credential resolution offline instead of contacting a forge.
				cmd := exec.Command("git", append(prefix, "credential", "fill")...)
				cmd.Stdin = strings.NewReader("protocol=https\nhost=github.com\n\n")
				return cmd
			}
		}
		return oldExec(name, args...)
	}
	if err := fetchRoleBase(work, "verifier", "example-org/tracker", "x-access-token"); err != nil {
		t.Fatal(err)
	}
	if len(prefix) == 0 {
		t.Fatal("no authenticated fetch ran")
	}
	if strings.Contains(strings.Join(prefix, " "), fixtureTokenValue) {
		t.Fatal("token leaked to argv")
	}
	for _, host := range []string{"github.com", "foreign.invalid"} {
		cmd := exec.Command("git", append(prefix, "credential", "fill")...)
		cmd.Dir = work
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=", "SSH_ASKPASS=")
		cmd.Stdin = strings.NewReader("protocol=https\nhost=" + host + "\n\n")
		out, err := cmd.CombinedOutput()
		if host == "github.com" {
			if err != nil || !strings.Contains(string(out), "password="+fixtureTokenValue) || strings.Contains(string(out), "STALE-") {
				t.Fatal("origin did not receive the requested role credential")
			}
		} else if err == nil || strings.Contains(string(out), fixtureTokenValue) || strings.Contains(string(out), "STALE-") {
			t.Fatal("foreign host received credentials")
		}
	}
	after, err := os.ReadFile(configPath)
	if err != nil || string(before) != string(after) {
		t.Fatal("source config changed")
	}
}

func TestRoleFetchMintRefusalStopsBeforeNetwork(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	giveOriginHost(t, work)
	roleCredential = func(string, deskkit.ForgeRepo, string) (deskkit.RoleCredential, error) {
		return deskkit.RoleCredential{}, deskkit.Refused("refused: custody unavailable")
	}
	err := fetchRoleBase(work, "verifier", "example-org/tracker", "x-access-token")
	if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("want custody refusal, got %v", err)
	}
	for _, call := range *calls {
		for _, arg := range call {
			if arg == "fetch" {
				t.Fatal("fetch ran after credential refusal")
			}
		}
	}
}

func TestRoleFetchRejectsUnsafeTransportBeforeCredentialResolution(t *testing.T) {
	for _, origin := range []string{
		"http://github.com/example-org/tracker.git",
		"https://embedded@example.invalid/example-org/tracker.git",
	} {
		t.Run(origin, func(t *testing.T) {
			work := newRepo(t)
			calls := withEnv(t, work)
			mustGit(t, work, "remote", "set-url", "origin", origin)
			roleCredential = func(string, deskkit.ForgeRepo, string) (deskkit.RoleCredential, error) {
				t.Fatal("unsafe transport reached credential resolution")
				return deskkit.RoleCredential{}, nil
			}
			if err := fetchRoleBase(work, "verifier", "example-org/tracker", "x-access-token"); err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
				t.Fatalf("unsafe transport did not refuse: %v", err)
			}
			for _, call := range *calls {
				for _, arg := range call {
					if arg == "fetch" {
						t.Fatal("unsafe transport reached fetch")
					}
				}
			}
		})
	}
}

func TestRoleFetchSupportedOrigins(t *testing.T) {
	for _, origin := range []string{"git@github.com:example-org/tracker.git", "ssh://git@github.com/example-org/tracker.git", "https://github.com/example-org/tracker.git"} {
		t.Run(origin, func(t *testing.T) {
			work := newRepo(t)
			withEnv(t, work)
			mustGit(t, work, "remote", "set-url", "origin", origin)
			mustGit(t, work, "config", "--add", "remote.origin.url", "https://unused.invalid/example-org/tracker.git")
			old := execCommand
			fetched := false
			execCommand = func(name string, args ...string) *exec.Cmd {
				for i, arg := range args {
					if name == "git" && arg == "fetch" {
						fetched = true
						if !strings.HasPrefix(args[i+3], "https://github.com") {
							t.Fatalf("non-HTTPS fetch: %v", args)
						}
						return exec.Command("git", "rev-parse", "origin/main")
					}
				}
				return old(name, args...)
			}
			if err := fetchRoleBase(work, "verifier", "example-org/tracker", "x-access-token"); err != nil {
				t.Fatal(err)
			}
			if !fetched {
				t.Fatal("no fetch")
			}
		})
	}
	work := newRepo(t)
	withEnv(t, work)
	mustGit(t, work, "config", "--add", "remote.origin.url", "/unused/local")
	if err := fetchRoleBase(work, "verifier", "example-org/tracker", "x-access-token"); err != nil {
		t.Fatal(err)
	}
}

// Every Git path-valued TLS option keeps its original meaning, including scoped
// keys and repeated values. Non-path TLS strings must not be path-expanded.
func TestRoleFetchTLSPathContext(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	giveOriginHost(t, work)
	want := map[string]string{}
	for _, prefix := range []string{"http.", "http.https://github.com/example-org/."} {
		for _, option := range []string{"sslCert", "sslKey", "sslCAPath", "sslCAInfo", "pinnedPubkey"} {
			key := prefix + option
			mustGit(t, work, "config", "--global", "--add", key, "~/earlier.pem")
			mustGit(t, work, "config", "--global", "--add", key, "~/TLS material.pem")
			want[key] = mustGit(t, work, "config", "--path", "--get", key)
		}
	}
	// Git treats these as strings, including PKCS#11 URI and pin hash forms.
	for key, value := range map[string]string{"http.proxySSLCert": "~/proxy-cert", "http.sslCert": "pkcs11:token=fixture", "http.pinnedPubkey": "sha256//fixture"} {
		mustGit(t, work, "config", "--replace-all", key, value)
		want[key] = mustGit(t, work, "config", "--get", key)
	}
	var fetch *exec.Cmd
	old := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		for _, arg := range args {
			if name == "git" && arg == "fetch" {
				fetch = exec.Command("git", "rev-parse", "origin/main")
				return fetch
			}
		}
		return old(name, args...)
	}
	if err := fetchRoleBase(work, "verifier", "example-org/tracker", "x-access-token"); err != nil {
		t.Fatal(err)
	}
	if fetch == nil {
		t.Fatal("no fetch")
	}
	for key, value := range want {
		cmd := exec.Command("git", "config", "--path", "--get", key)
		if key == "http.proxySSLCert" {
			cmd = exec.Command("git", "config", "--get", key)
		}
		cmd.Dir = work
		cmd.Env = fetch.Env
		out, err := cmd.CombinedOutput()
		if err != nil || strings.TrimSpace(string(out)) != value {
			t.Errorf("%s changed meaning: got %q, want %q, error %v", key, out, value, err)
		}
	}
}
