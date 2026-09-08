package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Test-fixture roster for deskack. The only roster surface deskack reads is the
// DISPLAY-ONLY repo alias (RepoShortLabel), so the fixture carries a minimal trust block
// plus an ASSAY_REPO_ALIASES map with a short name to prove <repo-short> resolves through
// the alias rather than always falling back to the basename.
const fixtureRoster = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_TRUSTED_BOT_SLUGS=worker=assay-worker-app:300000006
ASSAY_ALLOWED_REPOS=medici-finance/assay:ci:private,example-org/example-reconciler:ci:private
ASSAY_REPO_ALIASES=example-reconciler=recon:example-reconciler
`

// plantFixtureRoster writes the fixture roster under home and reloads the config cache.
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
