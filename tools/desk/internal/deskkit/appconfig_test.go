package deskkit

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAppIDEnvHit — an exported <ROLE>_APP_ID wins, no file needed.
func TestAppIDEnvHit(t *testing.T) {
	t.Setenv("REVIEWER_APP_ID", "111111")
	got, err := AppID("reviewer")
	if err != nil {
		t.Fatalf("AppID: %v", err)
	}
	if got != "111111" {
		t.Fatalf("AppID = %q, want 111111", got)
	}
}

// TestAppIDRoleEnvName — dashed roles map to underscored env names.
func TestAppIDRoleEnvName(t *testing.T) {
	if got := roleEnvName("issue-loop"); got != "ISSUE_LOOP_APP_ID" {
		t.Fatalf("roleEnvName = %q, want ISSUE_LOOP_APP_ID", got)
	}
	t.Setenv("ISSUE_LOOP_APP_ID", "222222")
	got, err := AppID("issue-loop")
	if err != nil {
		t.Fatalf("AppID: %v", err)
	}
	if got != "222222" {
		t.Fatalf("AppID = %q, want 222222", got)
	}
}

// TestAppIDFileHit — with the env var unset, the value is read from apps.env
// (leading `export `, quotes, and comment lines all handled).
func TestAppIDFileHit(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("REVIEWER_APP_ID", "")

	cfgDir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := "# desk App IDs\nexport WORKER_APP_ID=999\nexport REVIEWER_APP_ID=\"333333\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "apps.env"), []byte(body), 0o600); err != nil {
		t.Fatalf("write apps.env: %v", err)
	}

	got, err := AppID("reviewer")
	if err != nil {
		t.Fatalf("AppID: %v", err)
	}
	if got != "333333" {
		t.Fatalf("AppID = %q, want 333333", got)
	}
}

// TestAppIDNeither — env unset and no file line for the role → a clear error naming both fixes.
func TestAppIDNeither(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("REVIEWER_APP_ID", "")

	_, err := AppID("reviewer")
	if err == nil {
		t.Fatal("expected an error when neither env nor file provides the App ID")
	}
}

// TestInstallIDSingleOverride — a single <ROLE>_INSTALL_ID wins for every owner.
func TestInstallIDSingleOverride(t *testing.T) {
	t.Setenv("REVIEWER_INSTALL_ID", "100000009")
	for _, owner := range []string{"example-org", "medici-finance", "anything"} {
		got, err := InstallID("reviewer", owner)
		if err != nil || got != "100000009" {
			t.Fatalf("InstallID(reviewer,%s) = %q err=%v, want 100000009", owner, got, err)
		}
	}
}

// TestInstallIDPerOwner — with the single override unset, the per-owner key answers, and
// the env-var name derives from role+owner (dashes → underscores, upper-cased).
func TestInstallIDPerOwner(t *testing.T) {
	if got := ownerInstallEnvName("reviewer", "medici-finance"); got != "REVIEWER_INSTALL_ID_MEDICI_FINANCE" {
		t.Fatalf("ownerInstallEnvName = %q", got)
	}
	t.Setenv("REVIEWER_INSTALL_ID", "")
	t.Setenv("REVIEWER_INSTALL_ID_EXAMPLE_ORG", "100000002")
	t.Setenv("REVIEWER_INSTALL_ID_MEDICI_FINANCE", "100000001")
	if got, err := InstallID("reviewer", "example-org"); err != nil || got != "100000002" {
		t.Fatalf("InstallID example-org = %q err=%v", got, err)
	}
	if got, err := InstallID("reviewer", "medici-finance"); err != nil || got != "100000001" {
		t.Fatalf("InstallID medici-finance = %q err=%v", got, err)
	}
}

// TestInstallIDFromFile — with all env unset the id is read from apps.env, per-owner key.
func TestInstallIDFromFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("REVIEWER_INSTALL_ID", "")
	t.Setenv("REVIEWER_INSTALL_ID_EXAMPLE_ORG", "")
	cfgDir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := "export REVIEWER_INSTALL_ID_EXAMPLE_ORG=100000007\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "apps.env"), []byte(body), 0o600); err != nil {
		t.Fatalf("write apps.env: %v", err)
	}
	got, err := InstallID("reviewer", "example-org")
	if err != nil || got != "100000007" {
		t.Fatalf("InstallID from file = %q err=%v, want 100000007", got, err)
	}
}

// TestInstallIDFailsClosed — no override, no per-owner key → an error naming both fixes.
func TestInstallIDFailsClosed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("REVIEWER_INSTALL_ID", "")
	t.Setenv("REVIEWER_INSTALL_ID_EXAMPLE_ORG", "")
	if _, err := InstallID("reviewer", "example-org"); err == nil {
		t.Fatal("expected an error when no install id is configured (fail closed)")
	}
}
