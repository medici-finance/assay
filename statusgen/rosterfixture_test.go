package main

// Test-fixture roster installer.
//
// The trust roster, the allowed-repo set and the human-login map are no longer
// compiled in — they are adopter configuration read from a file under the config
// home (see statusgen/rosterconfig.go). A test binary therefore has to
// INSTALL a roster before any trust decision, or every one of them correctly
// answers "unconfigured, refuse".
//
// This installs THE VALUES THIS TREE USED TO COMPILE IN, into a private HOME, so
// every pre-existing behavioural assertion in this package keeps asserting the
// same verdict it always did. That equivalence is deliberate evidence, not
// convenience: if the conversion changed a verdict, these suites go red.
//
// It is a TestMain rather than a per-test helper because several suites here run
// the built binary as a subprocess, which inherits the process environment — a
// t.Setenv-scoped HOME would not reach the child.

import (
	"os"
	"path/filepath"
	"testing"
)

// fixtureRoster is the config-home file contents. 0600, in a directory this
// process owns — the loader enforces the sshd rule and refuses anything looser.
const fixtureRoster = `# Test-fixture roster. It reproduces the values this tree used to compile in, so
# every pre-existing behavioural test asserts the SAME verdicts it always did —
# that equivalence is the point.
# Test files may carry these literals; non-test source may not (Verify row 2).
ASSAY_BLESS_LOGIN=ada:100001
ASSAY_TRUSTED_LOGINS=ada:100001,shared-agent:100002
ASSAY_TRUSTED_BOT_SLUGS=desk=assay-desk-app:300000001,intake-loop=assay-intake-loop-app:300000002,issue-loop=assay-issue-loop-app:300000003,reviewer=assay-reviewer-app:300000004,verifier=assay-verifier-app:300000005,worker=assay-worker-app:300000006
ASSAY_ALLOWED_REPOS=example-org/tracker:ci:private,example-org/agents:ci:private,example-org/examples:no-ci:private,example-org/console:ci:private,example-org/toolkit:ci:private,example-org/ledger-k8s:ci:public,example-org/example-service:ci:private,example-org/org-slides:no-ci:private,example-org/proposals:no-ci:public,example-org/platform:ci:private,example-org/docs-slides:no-ci:private,example-org/team-slides:no-ci:private,example-org/example-service-slides:no-ci:private
ASSAY_HUMAN_LOGIN_MAP=alex:ada
ASSAY_FORMER_HUMAN_LOGIN_MAP=bob:bob-gh,former_lead:former-lead-gh
ASSAY_HOME_REPO=example-org/tracker
ASSAY_SCAN_REPOS=example-org/tracker,example-org/agents,example-org/examples,example-org/toolkit,example-org/example-service,example-org/platform,example-org/console,example-org/website,example-org/site,example-org/proposals,example-org/example-service-slides,example-org/team-slides,example-org/docs-slides,example-org/org-slides
`

// darFixtureExtra returns the product-config lines appended to the fixture roster
// in TestMain. It is a no-op hook (rosterfixture_openstub_test.go) so this fixture
// file carries no product-namespaced (non-ASSAY_) configuration.

func installFixtureRoster() (cleanup func(), err error) {
	home, err := os.MkdirTemp("", "assay-roster-home-")
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(fixtureRoster+darFixtureExtra()), 0o600); err != nil {
		return nil, err
	}
	prev, had := os.LookupEnv("HOME")
	if err := os.Setenv("HOME", home); err != nil {
		return nil, err
	}
	scanReloadConfig()
	return func() {
		if had {
			_ = os.Setenv("HOME", prev)
		} else {
			_ = os.Unsetenv("HOME")
		}
		os.RemoveAll(home)
		scanReloadConfig()
	}, nil
}

func TestMain(m *testing.M) {
	cleanup, err := installFixtureRoster()
	if err != nil {
		panic("cannot install the test-fixture roster: " + err.Error())
	}
	code := m.Run()
	cleanup()
	os.Exit(code)
}
