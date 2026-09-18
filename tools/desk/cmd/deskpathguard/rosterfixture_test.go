package main

// Test-fixture roster installer — the documented-duplicate pattern (see
// cmd/desklabel/rosterfixture_test.go's header): every sibling command package carries its
// own copy so a test asserts against the SAME roster the tool reads in production.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const fixtureRoster = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001,shared-agent:2002
ASSAY_TRUSTED_BOT_SLUGS=desk=assay-desk-app:300000001,intake-loop=assay-intake-loop-app:300000002,issue-loop=assay-issue-loop-app:300000003,reviewer=assay-reviewer-app:300000004,verifier=assay-verifier-app:300000005,worker=assay-worker-app:300000006
ASSAY_ALLOWED_REPOS=medici-finance/assay:ci:public
ASSAY_HUMAN_LOGIN_MAP=alex:ada
`

func plantFixtureRoster(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("planting the fixture roster: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(fixtureRoster), 0o600); err != nil {
		t.Fatalf("planting the fixture roster: %v", err)
	}
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
}
