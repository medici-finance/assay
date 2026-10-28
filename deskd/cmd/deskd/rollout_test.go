package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/identity"
)

// ── the service contract Task 9 — deskd's wiring of the ROLLOUT GATE ───────────
//
// the authorization contract's headline defect: "the per-user path is enabled globally
// by env-var presence — `if oauthCfg.Configured()` — uniform across every
// cell". These tests are the fixture-config half of the fix: the ENABLE is a
// per-cell config opt-in, and the env vars are only a precondition for
// serving what the config already permitted.

// writeCells materialises a cells.yaml fixture. Tests use fixtures, never the
// committed config — the committed config carries no opt-in and must not.
func writeCells(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "cells.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// cellDoc renders one cell with the given name, extra lines and (optionally) a
// dev environments block, in a document that may hold several.
func cellDoc(name, repo, extra string, withDevEnv bool) string {
	doc := `  - name: ` + name + `
    repos: [` + repo + `]
    streams_root: docs/streams
    loops:
      - role: review
        schedule: "*/10 * * * *"
        models: [deepseek-chat]
        app: assay-reviewer-app
    silence_threshold: 48h
    notification_tiers: {review: push, question: badge, notify: none}
` + extra
	if withDevEnv {
		doc += `    environments:
      - name: ` + name + `-dev
        kind: k8s
        tier: dev
        cluster: fixture-cluster
        namespaces: [example-dev-app]
        access: [read]
        runner_pool: pool-` + name + `
`
	}
	return doc
}

// (a) THE ENV VARS ALONE DO NOT ENABLE THE PATH. This is the regression test
// for the defect issue #55 reports: a fully configured OAuth client on a
// config with no opt-in serves no sign-in and enables no cell.
func TestEnvVarsAloneDoNotEnableThePerUserPath(t *testing.T) {
	t.Setenv(oauthClientEnv, "client-id-from-the-environment")
	t.Setenv(oauthSecretEnv, "client-secret-from-the-environment")
	t.Setenv(oauthRedirectEnv, "http://127.0.0.1:8787/auth/callback")

	path := writeCells(t, "cells:\n"+
		cellDoc("assay", "example-org/toolkit", "", true)+
		cellDoc("tracker", "example-other/tracker", "", false))
	cfg, err := schema.Load(path)
	if err != nil {
		t.Fatalf("fixture must validate: %v", err)
	}

	oauth := oauthConfigFromEnv("127.0.0.1:8787", "https://api.github.com")
	if !oauth.Configured() {
		t.Fatal("precondition: the fixture sets all three env vars, so the OAuth client IS configured")
	}

	rollout := buildCellRollout(cfg)
	if rollout.SignInServed() {
		t.Fatal("sign-in is served with no cell opted in — env-var presence is enabling the path (the authorization contract)")
	}
	if perUserSignInEnabled(oauth, rollout) {
		t.Fatal("the per-user path is ENABLED by env vars alone — the enable must be the per-cell config opt-in")
	}
	if rollout.EnabledForRepo("example-org", "toolkit") {
		t.Fatal("a cell with a dev environment but NO opt-in must stay OFF — environments: is a precondition, never the enable")
	}
}

// Both preconditions must hold for the path to actually serve: the config
// opt-in AND a usable OAuth client. An opted-in cell on a deskd with no OAuth
// client serves no sign-in — it cannot, and it must say so rather than
// half-enable.
func TestOptInWithoutOAuthClientStillServesNoSignIn(t *testing.T) {
	path := writeCells(t, "cells:\n"+
		cellDoc("assay", "example-org/toolkit", "    per_user_identity: enabled\n", true))
	cfg, err := schema.Load(path)
	if err != nil {
		t.Fatalf("fixture must validate: %v", err)
	}
	rollout := buildCellRollout(cfg)
	if !rollout.SignInServed() {
		t.Fatal("the opted-in cell must make sign-in servable in principle")
	}
	if perUserSignInEnabled(identity.Config{}, rollout) {
		t.Fatal("an unconfigured OAuth client must not serve sign-in even on an opted-in cell")
	}
}

// (f) TWO CELLS IN ONE PROCESS DIFFER, resolved from the ITEM's repo — Verify
// row 12. The proving cell is opted in; the second cell is private with no
// opt-in and stays OFF in the same process.
func TestRolloutResolvesPerCellFromTheItemsRepo(t *testing.T) {
	path := writeCells(t, "cells:\n"+
		cellDoc("assay", "example-org/toolkit", "    per_user_identity: enabled\n", true)+
		cellDoc("tracker", "example-other/tracker", "", true))
	cfg, err := schema.Load(path)
	if err != nil {
		t.Fatalf("fixture must validate: %v", err)
	}
	r := buildCellRollout(cfg)

	if !r.EnabledForRepo("example-org", "toolkit") {
		t.Error("the opted-in proving cell's repo must be ON")
	}
	if r.EnabledForRepo("example-other", "tracker") {
		t.Error("the second cell has no opt-in and must be OFF in the same process")
	}
	if !r.SignInServed() {
		t.Error("sign-in is servable once ANY cell is opted in")
	}
}

// Fail-closed on everything unresolvable: no config, an empty config, and a
// repo no cell owns. Verify 12a's positive control — the default is off, not
// merely absent.
func TestRolloutFailsClosedWhenUnresolvable(t *testing.T) {
	if buildCellRollout(nil).SignInServed() {
		t.Error("a nil config must fail closed to OFF")
	}
	if buildCellRollout(nil).EnabledForRepo("example-org", "toolkit") {
		t.Error("a nil config must enable no repo")
	}
	if buildCellRollout(&schema.File{}).SignInServed() {
		t.Error("a config with no cells must fail closed to OFF")
	}

	path := writeCells(t, "cells:\n"+
		cellDoc("assay", "example-org/toolkit", "    per_user_identity: enabled\n", true))
	cfg, err := schema.Load(path)
	if err != nil {
		t.Fatalf("fixture must validate: %v", err)
	}
	r := buildCellRollout(cfg)
	if r.EnabledForRepo("someone-else", "unowned") {
		t.Error("a repo no cell owns must fail closed to OFF, never inherit another cell's opt-in")
	}
	if r.EnabledForRepo("", "") {
		t.Error("an unresolvable repo must fail closed to OFF")
	}
}
