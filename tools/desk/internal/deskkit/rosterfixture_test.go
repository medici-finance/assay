package deskkit

// Test-fixture roster installer.
//
// The trust roster, the allowed-repo set and the human-login map are no longer
// compiled in — they are adopter configuration read from a file under the config
// home (see rosterconfig.go). A test binary therefore has to
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
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The fixture roster's identities, named so tests read against the fixture rather
// than against a value that used to be compiled in.
const (
	fixtureBlessLogin       = "ada"
	fixtureBlessID    int64 = 2001
	fixtureHumanLogin       = "example-org"
	fixtureHumanID    int64 = 2002
	fixtureBotSlug          = "assay-reviewer-app"
	fixtureBotID      int64 = 300000004
)

// fixtureRoster is the config-home file contents. 0600, in a directory this
// process owns — the loader enforces the sshd rule and refuses anything looser.
const fixtureRoster = `# Test-fixture roster. It reproduces the values this tree used to compile in, so
# every pre-existing behavioural test asserts the SAME verdicts it always did —
# that equivalence is the point (the golden-roster property).
# Test files may carry these literals; non-test source may not.
ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001,shared-agent:2002
ASSAY_TRUSTED_BOT_SLUGS=desk=assay-desk-app:300000001,intake-loop=assay-intake-loop-app:300000002,issue-loop=assay-issue-loop-app:300000003,reviewer=assay-reviewer-app:300000004,verifier=assay-verifier-app:300000005,worker=assay-worker-app:300000006
ASSAY_ALLOWED_REPOS=example-org/tracker:ci:private,example-org/agents:ci:private,example-org/examples:no-ci:private,example-org/console:ci:private,medici-finance/assay:ci:private,example-org/example-k8s:ci:public,example-org/example-reconciler:ci:private,example-org/org-slides:no-ci:private,example-org/proposals:no-ci:public,example-org/platform:ci:private,example-org/demo-slides:no-ci:private,example-org/assay-slides:no-ci:private,example-org/example-reconciler-slides:no-ci:private
ASSAY_HUMAN_LOGIN_MAP=alex:ada
`

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
	ReloadConfig()
	return func() {
		if had {
			_ = os.Setenv("HOME", prev)
		} else {
			_ = os.Unsetenv("HOME")
		}
		os.RemoveAll(home)
		ReloadConfig()
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

// withRoster installs an explicit roster into a private config home for one test
// and reloads. It goes through the REAL loader (file, permissions, parsing), so a
// test that passes here proves the shipped path, not a shortcut around it.
func withRoster(t *testing.T, vals map[string]string) string {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	keys := make([]string, 0, len(vals))
	for k := range vals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%s\n", k, vals[k])
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	ReloadConfig()
	t.Cleanup(ReloadConfig)
	return home
}

// withNoRoster points the config home at an EMPTY directory: the P1 unset case.
func withNoRoster(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	ReloadConfig()
	t.Cleanup(ReloadConfig)
	return home
}
