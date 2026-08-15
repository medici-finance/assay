package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestMintTokenSuccess proves the JWT→installation-token exchange works against a fake
// access_tokens endpoint (endpoints/env identical to mint-reviewer-token.go).
func TestMintTokenSuccess(t *testing.T) {
	setupFake(t)
	tok, err := mintInstallationToken("example-org")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if tok != "fake-installation-token" {
		t.Fatalf("token = %q, want fake-installation-token", tok)
	}
}

// TestMintTokenMissingPEM — a missing/unreadable App key is Unverifiable (exit 6) and the
// message names the manual restore path. The token is NEVER a silent empty string.
func TestMintTokenMissingPEM(t *testing.T) {
	setupFake(t)
	h, _ := os.UserHomeDir()
	t.Setenv("REVIEWER_PEM", filepath.Join(h, "does-not-exist.pem"))

	_, err := mintInstallationToken("example-org")
	if err == nil {
		t.Fatal("expected an error for a missing PEM")
	}
	if !deskkit.IsUnverifiable(err) {
		t.Fatalf("missing PEM err code = %d, want 6 (unverifiable)", deskkit.ExitCodeOf(err))
	}
}

// TestReadyMissingPEMExit6 — the missing PEM surfaces through a real verb as exit 6 with
// no flip.
func TestReadyMissingPEMExit6(t *testing.T) {
	f, _ := setupFake(t)
	h, _ := os.UserHomeDir()
	t.Setenv("REVIEWER_PEM", filepath.Join(h, "nope.pem"))

	code := run([]string{"ready", "example-org/tracker", "1"})
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("exit = %d, want 6", code)
	}
	if f.flips != 0 {
		t.Fatal("no flip on a token-mint failure")
	}
}

// TestInstallForOwner keeps the per-owner installation mapping resolvable — but now from
// per-deployment config, not a source-baked map (the ids are machine identities that must
// not ship in source, a public-repo security review). The single REVIEWER_INSTALL_ID override is cleared
// so the per-owner <ROLE>_INSTALL_ID_<OWNER> keys are what answer.
func TestInstallForOwner(t *testing.T) {
	t.Setenv("REVIEWER_INSTALL_ID", "")
	t.Setenv("REVIEWER_INSTALL_ID_EXAMPLE_ORG", "100000002")
	t.Setenv("REVIEWER_INSTALL_ID_MEDICI_FINANCE", "100000001")
	if got, err := installForOwner("example-org"); err != nil || got != "100000002" {
		t.Fatalf("example-org install = %q err=%v", got, err)
	}
	if got, err := installForOwner("medici-finance"); err != nil || got != "100000001" {
		t.Fatalf("medici-finance install = %q err=%v", got, err)
	}
}

// TestInstallForOwnerFailsClosed — with no override and no per-owner key, mint has no
// installation to target and must refuse rather than guess (fail closed, a public-repo security review).
func TestInstallForOwnerFailsClosed(t *testing.T) {
	t.Setenv("REVIEWER_INSTALL_ID", "")
	t.Setenv("REVIEWER_INSTALL_ID_EXAMPLE_ORG", "")
	if _, err := installForOwner("example-org"); err == nil {
		t.Fatal("expected an error when no install id is configured for the owner")
	}
}

// TestMintTokenMissingAppID — with REVIEWER_APP_ID unset the mint fails loud (Unverifiable,
// exit 6) rather than falling back to a source-baked App ID. Deployability guard: the App ID
// must come from per-deployment config, never a hardcoded default.
func TestMintTokenMissingAppID(t *testing.T) {
	setupFake(t)
	t.Setenv("REVIEWER_APP_ID", "")

	_, err := mintInstallationToken("example-org")
	if err == nil {
		t.Fatal("expected an error when REVIEWER_APP_ID is unset")
	}
	if !deskkit.IsUnverifiable(err) {
		t.Fatalf("missing app-id err code = %d, want 6 (unverifiable)", deskkit.ExitCodeOf(err))
	}
}

// --- App-credential search path (#794) -------------------------------------------

// TestMintTokenUsesConfigHomeSearchPath is the #794 regression for deskpost specifically.
// desktoken honouring ASSAY_CONFIG_HOME is not enough: deskpost mints its OWN reviewer
// token, and while it resolved the key at a hardcoded ~/.config/assay it kept failing in
// a fresh shell on a deployment that provisions elsewhere — with the App ID resolving
// fine off the search path, so the failure read as a broken key rather than a wrong
// directory. No REVIEWER_PEM here on purpose: the point is the DEFAULT resolution.
func TestMintTokenUsesConfigHomeSearchPath(t *testing.T) {
	setupFake(t)
	home, _ := os.UserHomeDir()
	t.Setenv("REVIEWER_PEM", "")

	provisioned := filepath.Join(home, "vault", "provisioned")
	if err := os.MkdirAll(provisioned, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(provisioned, "reviewer-app.pem"), reviewerPEM(t), 0o600); err != nil {
		t.Fatalf("write pem: %v", err)
	}
	t.Setenv(deskkit.EnvConfigHome, provisioned)

	if got, _ := resolveReviewerPEM(); got != filepath.Join(provisioned, "reviewer-app.pem") {
		t.Fatalf("resolveReviewerPEM = %q, want the key on the search path", got)
	}
	tok, err := mintInstallationToken("example-org")
	if err != nil {
		t.Fatalf("mint from the search path: %v", err)
	}
	if tok != "fake-installation-token" {
		t.Fatalf("token = %q, want fake-installation-token", tok)
	}
}

// TestMintTokenWrongPlacedKeyRefused is the POSITIVE CONTROL for the resolver: a key that
// exists but sits in a directory NOT on the search path must be refused (exit 6), and the
// refusal must name every directory searched plus both knobs. A resolver that cannot find
// its source has to fail closed and say where it looked — "found nothing, carried on" is
// the shape that made #794 surface an hour later as an authentication failure.
func TestMintTokenWrongPlacedKeyRefused(t *testing.T) {
	setupFake(t)
	home, _ := os.UserHomeDir()
	t.Setenv("REVIEWER_PEM", "")
	t.Setenv(deskkit.EnvConfigHome, "")

	// The key is real and readable — just in a directory nothing searches.
	stray := filepath.Join(home, "elsewhere")
	if err := os.MkdirAll(stray, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stray, "reviewer-app.pem"), reviewerPEM(t), 0o600); err != nil {
		t.Fatalf("write pem: %v", err)
	}

	_, err := mintInstallationToken("example-org")
	if err == nil {
		t.Fatal("a key off the search path must NOT mint — the resolver has to fail closed")
	}
	if !deskkit.IsUnverifiable(err) {
		t.Fatalf("off-path key err code = %d, want 6 (unverifiable)", deskkit.ExitCodeOf(err))
	}
	for _, want := range []string{
		"cannot read reviewer App key",
		filepath.Join(home, ".config", "assay"),
		deskkit.EnvConfigHome,
		"REVIEWER_PEM",
		"#794",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal should name %q; got: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), stray) {
		t.Errorf("refusal names a directory it never searched (%s): %v", stray, err)
	}
}
