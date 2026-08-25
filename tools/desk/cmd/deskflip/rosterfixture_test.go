package main

// Test-fixture roster installer.
//
// The trust roster and the allowed-repo set are no longer compiled in — they are
// adopter configuration read from a file under the config home (see
// deskkit/rosterconfig.go). A test binary therefore has to INSTALL a roster before
// any trust or write-authorisation decision, or every one of them correctly answers
// "unconfigured, refuse".
//
// Every sibling command package (deskpr, deskboard, deskpost, ...) carries its own copy
// of this fixture; this is deskflip's, per the documented-duplicate pattern.
//
// This installs, into a private HOME, THE VALUES THAT WILL ACTUALLY BE SET — so every
// behavioural assertion in this package asserts against the same roster the tool
// reads in production, not a hand-picked stand-in.
//
// The allowed-repo set is the pre-conversion compiled set of NINE, plus the FOUR that
// #456 added (platform and one slides repo per product) — THIRTEEN entries,
// set-equal to the consumer's documented ASSAY_ALLOWED_REPOS.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const fixtureRoster = `# Test-fixture roster. It reproduces the values this tree used to compile in, so
# every pre-existing behavioural test asserts the SAME verdicts it always did —
# that equivalence is the point.
# Test files may carry these literals; non-test source may not (Verify row 2).
ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001,shared-agent:2002
ASSAY_TRUSTED_BOT_SLUGS=desk=assay-desk-app:300000001,intake-loop=assay-intake-loop-app:300000002,issue-loop=assay-issue-loop-app:300000003,reviewer=assay-reviewer-app:300000004,verifier=assay-verifier-app:300000005,worker=assay-worker-app:300000006
ASSAY_ALLOWED_REPOS=example-org/tracker:ci:private,example-org/agents:ci:private,example-org/examples:no-ci:private,example-org/console:ci:private,medici-finance/assay:ci:private,example-org/example-k8s:ci:public,example-org/example-reconciler:ci:private,example-org/org-slides:no-ci:private,example-org/proposals:no-ci:public,example-org/platform:ci:private,example-org/demo-slides:no-ci:private,example-org/assay-slides:no-ci:private,example-org/example-reconciler-slides:no-ci:private
ASSAY_HUMAN_LOGIN_MAP=alex:ada
`

// plantFixtureRoster writes the fixture roster under home. A test that relocates
// HOME for its own reasons relocates the CONFIG HOME with it, so it must call this
// or every trust decision in that test correctly answers "unconfigured".
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

func installFixtureRoster() (cleanup func(), err error) {
	home, err := os.MkdirTemp("", "assay-roster-home-")
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(fixtureRoster), 0o600); err != nil {
		return nil, err
	}
	prev, had := os.LookupEnv("HOME")
	if err := os.Setenv("HOME", home); err != nil {
		return nil, err
	}
	deskkit.ReloadConfig()
	return func() {
		if had {
			_ = os.Setenv("HOME", prev)
		} else {
			_ = os.Unsetenv("HOME")
		}
		os.RemoveAll(home)
		deskkit.ReloadConfig()
	}, nil
}
