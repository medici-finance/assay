package deskkit

// custodylink_test.go — a token-custody path must not be FOLLOWED blindly when it is a
// symlink (follow-up to #1573). The custody readers used os.Stat, so the mode and
// regular-file checks ran against whatever a link at the custody path pointed at, and the
// read that followed took that file's bytes as the credential.
//
// Two layouts, two rules:
//   - GitHub App token custody (the cache desktoken writes) has no symlink layout, so ANY
//     link at the custody path is refused.
//   - GitLab PAT custody documents one link: gitlab-<role>.token -> <prefix>-<role>-bot.token
//     in the SAME directory (docs/adopting-assay-gitlab.md §2). That link stays accepted; a
//     link that resolves out of the custody directory is refused.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// plantLink creates link -> target, skipping where the platform cannot make symlinks.
func plantLink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}
}

func writeCustody(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestVerifyCustodyFileModeRefusesSymlink — the GitHub custody check refuses a link planted
// at the token path, even one whose target is a perfectly good 0600 regular file.
func TestVerifyCustodyFileModeRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "elsewhere-0600")
	writeCustody(t, real, "ghs-stub-value")
	link := filepath.Join(dir, "worker-token-1")
	plantLink(t, real, link)

	err := verifyCustodyFileMode(link)
	if err == nil {
		t.Fatal("verifyCustodyFileMode accepted a SYMLINK at the custody path — it must refuse, " +
			"not check and read whatever the link points at")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("refusal must name the symlink; got: %v", err)
	}
	if ExitCodeOf(err) != ExitRefused {
		t.Errorf("a symlinked custody path must be Refused (exit %d); got %v", ExitRefused, err)
	}

	// The regular file itself still passes: the owner-only mode check is unchanged.
	if err := verifyCustodyFileMode(real); err != nil {
		t.Fatalf("a regular 0600 custody file must still pass: %v", err)
	}
	if err := os.Chmod(real, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyCustodyFileMode(real); err == nil || !strings.Contains(err.Error(), "0600") {
		t.Fatalf("a 0644 custody file must still be refused naming 0600; got %v", err)
	}
}

// TestGitLabRoleTokenRefusesOutOfDirLink — a gitlab-<role>.token link that resolves OUT of
// the custody directory is refused; the documented same-directory link still reads.
func TestGitLabRoleTokenRefusesOutOfDirLink(t *testing.T) {
	credDir := t.TempDir()
	t.Setenv(EnvConfigHome, credDir)
	custody := filepath.Join(credDir, gitlabTokenFileName("worker"))

	outside := filepath.Join(t.TempDir(), "planted-0600")
	writeCustody(t, outside, "glpat-planted-value")
	plantLink(t, outside, custody)

	if tok, _, err := GitLabRoleToken("worker"); err == nil {
		t.Fatalf("GitLabRoleToken followed a custody link out of its directory and read a "+
			"credential from it (len %d) — it must refuse", len(tok))
	} else if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("refusal must name the symlink; got: %v", err)
	}

	// The documented layout: a relative link to <prefix>-<role>-bot.token in the SAME dir.
	if err := os.Remove(custody); err != nil {
		t.Fatal(err)
	}
	writeCustody(t, filepath.Join(credDir, "example-worker-bot.token"), "glpat-provisioned")
	plantLink(t, "example-worker-bot.token", custody)
	tok, path, err := GitLabRoleToken("worker")
	if err != nil {
		t.Fatalf("the documented same-directory custody link must still read: %v", err)
	}
	if tok != "glpat-provisioned" || path != custody {
		t.Fatalf("documented link read = (len %d, %q), want the provisioned value via %q", len(tok), path, custody)
	}

	// A loose target behind the documented link is still refused on its mode.
	if err := os.Chmod(filepath.Join(credDir, "example-worker-bot.token"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := GitLabRoleToken("worker"); err == nil || !strings.Contains(err.Error(), "0600") {
		t.Fatalf("a 0644 target behind the custody link must be refused naming 0600; got %v", err)
	}
}

// TestGitLabColdCustodyProbeRefusesOutOfDirLink — the boot preflight's read-only probe makes
// the same custody decision as the read path, so boot never passes custody a verb refuses.
func TestGitLabColdCustodyProbeRefusesOutOfDirLink(t *testing.T) {
	home := withRoster(t, goldenRoster())
	credDir := filepath.Join(home, ".config", "assay")
	custody := filepath.Join(credDir, gitlabTokenFileName(pfRole))
	t.Setenv("GITLAB_API_BASE", "https://gitlab.example.com/api/v4")

	outside := filepath.Join(t.TempDir(), "planted-0600")
	writeCustody(t, outside, "glpat-planted-value")
	if err := os.MkdirAll(credDir, 0o700); err != nil {
		t.Fatal(err)
	}
	plantLink(t, outside, custody)

	if _, err := gitlabColdCustodyProbe(pfRole); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("the cold custody probe must refuse a custody link out of its directory, naming "+
			"the symlink; got %v", err)
	}

	if err := os.Remove(custody); err != nil {
		t.Fatal(err)
	}
	writeCustody(t, filepath.Join(credDir, "example-"+pfRole+"-bot.token"), "glpat-provisioned")
	plantLink(t, "example-"+pfRole+"-bot.token", custody)
	if _, err := gitlabColdCustodyProbe(pfRole); err != nil {
		t.Fatalf("the documented same-directory custody link must pass the cold probe: %v", err)
	}
}
