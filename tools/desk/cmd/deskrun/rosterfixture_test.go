package main

// Test-fixture roster installer — deskrun's copy of the documented-duplicate fixture every
// sibling command package carries (see cmd/desklabel/rosterfixture_test.go), plus the one key
// this verb reads: ASSAY_RUN_CREDENTIALS. It binds three repos to the three outcomes the
// identity rule distinguishes, and binds the release-runner role to its OWN App slug (never a
// desk role's):
//
//	example-org/tracker    release-runner                 → proceeds
//	example-org/platform   release-runner+manual-job      → proceeds, GitLab gate shape declared
//	example-org/console    human:ada                      → refused (exit 5)
//	example-org/agents     (no entry)                     → could-not-check (exit 6)

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const fixtureRoster = `# Test-fixture roster.
ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001,shared-agent:2002
ASSAY_TRUSTED_BOT_SLUGS=desk=assay-desk-app:300000001,reviewer=assay-reviewer-app:300000004,verifier=assay-verifier-app:300000005,worker=assay-worker-app:300000006,release-runner=example-release-runner-app:300000009
ASSAY_ALLOWED_REPOS=example-org/tracker:ci:private,example-org/agents:ci:private,example-org/console:ci:private,example-org/platform:ci:private
ASSAY_HUMAN_LOGIN_MAP=alex:ada
ASSAY_REPO_FORGES=example-org/tracker=github,example-org/agents=github,example-org/console=github,example-org/platform=gitlab
ASSAY_RUN_CREDENTIALS=example-org/tracker=release-runner,example-org/platform=release-runner+manual-job,example-org/console=human:ada
`

// plantFixtureRoster writes the fixture roster under home.
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
