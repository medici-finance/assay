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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/gitquiet"
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

// pinFixtureGoCaches preserves the caller's resolved Go paths before the roster
// fixture replaces HOME. Every Go child compiler, including the cross-module
// heads bridge, must use the dependencies prepared by the caller (#3327).
func pinFixtureGoCaches() error {
	out, err := exec.Command("go", "env", "-json", "GOMODCACHE", "GOCACHE", "GOPATH").Output()
	if err != nil {
		return fmt.Errorf("resolve Go fixture paths: %w", err)
	}
	var paths map[string]string
	if err := json.Unmarshal(out, &paths); err != nil {
		return fmt.Errorf("decode Go fixture paths: %w", err)
	}
	for _, key := range []string{"GOMODCACHE", "GOCACHE", "GOPATH"} {
		if paths[key] == "" {
			return fmt.Errorf("empty Go fixture path %s", key)
		}
		if err := os.Setenv(key, paths[key]); err != nil {
			return err
		}
	}
	return nil
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
	// The test binary stands in for statusgen as the check:ci sandbox helper
	// (netns.go) and as the network probe the netns tests run inside it.
	maybeRunNetnsHelper()
	maybeRunNetProbe()
	// No test inherits the caller's loop identity. verifyrun's pre-work
	// verifier admission turns on when DESK_LOOP names a verify loop and then
	// execs the real deskdispatch on PATH, so a test run from a verify-desk
	// session would reach live desk tooling instead of its fixture (#2297).
	// A test that needs a loop sets it itself with t.Setenv;
	// TestDeskLoopCleared pins this line.
	if err := os.Unsetenv("DESK_LOOP"); err != nil {
		panic("cannot clear DESK_LOOP for the test binary: " + err.Error())
	}
	// No test inherits the runner's GITHUB_ACTIONS either. reconcile picks its
	// roster class from it, so on an Actions runner every reconcile test would
	// read the empty env roster instead of the fixture file below, and the
	// process-global class would leak into later roster tests (#3082). A test
	// that needs Actions sets it itself with t.Setenv;
	// TestGitHubActionsCleared pins this line.
	if err := os.Unsetenv("GITHUB_ACTIONS"); err != nil {
		panic("cannot clear GITHUB_ACTIONS for the test binary: " + err.Error())
	}
	if err := pinFixtureGoCaches(); err != nil {
		panic(err)
	}
	cleanup, err := installFixtureRoster()
	if err != nil {
		panic("cannot install the test-fixture roster: " + err.Error())
	}
	// The decision-gate hold on --close-verify (lifecycle-v1 §4.5) reads the
	// forge. The close-verify fixtures predate it and test the flip itself, so
	// the hold is off for them; decisiongatehold_test.go restores the real check
	// (closeVerifyDecisionGate) for every test that exercises it.
	closeVerifyDecisionGateFn = func(string, string, []byte) error { return nil }
	code := gitquiet.Run(m)
	cleanup()
	os.Exit(code)
}
