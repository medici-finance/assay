package main

// Test-fixture roster installer.
//
// The trust roster and the allowed-repo set are no longer compiled in — they are
// adopter configuration read from a file under the config home (see
// deskkit/rosterconfig.go). A test binary therefore has to INSTALL a roster before
// any trust or write-authorisation decision, or every one of them correctly answers
// "unconfigured, refuse".
//
// deskfile was missing this file entirely (found while fixing PR #484's delta
// re-review: TestEveryRosterReadingMainDeclaresClassAndEchoes, added to
// internal/deskkit after this PR branched, flagged that main() never declared a tool
// class either — see main.go's SetToolClass call). Every sibling command package
// (deskpr, deskboard, deskpost, ...) carries its own copy of this fixture; this is
// deskfile's, verbatim, per the documented-duplicate pattern.
//
// This installs, into a private HOME, THE VALUES THAT WILL ACTUALLY BE SET — so every
// behavioural assertion in this package (starting with `allowedRepo =
// "medici-finance/assay"`) asserts against the same roster the tool reads in
// production, not a hand-picked stand-in.
//
// The allowed-repo set is the pre-conversion compiled set of NINE, plus the FOUR that
// #456 added (platform and one slides repo per product) — THIRTEEN entries,
// set-equal to the consumer's documented ASSAY_ALLOWED_REPOS.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// installFixtureRoster plants the fixture roster in a fresh private HOME under $TMPDIR and
// points HOME at it. The cleanup it returns restores HOME and removes that directory — and
// it MUST be called explicitly between m.Run() and os.Exit (finishFixtureRoster does that):
// registered with `defer`, it never fires past os.Exit, and one fixture HOME survived per
// run until #1195 found thousands of them.
//
// Before HOME moves, the host's Go caches are pinned into the process environment
// (pinHostGoEnv): with HOME relocated and nothing pinned, every `go build` of a fake
// binary and every `go list` in the suite resolved GOPATH/GOMODCACHE/GOCACHE under the
// fixture and re-downloaded the module graph and rebuilt the standard library into it on
// every run — a quarter of a gigabyte of read-only files per package per run.
func installFixtureRoster() (cleanup func(), err error) {
	if err := pinHostGoEnv(); err != nil {
		return nil, err
	}
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
		removeFixtureTree(home)
		deskkit.ReloadConfig()
	}, nil
}

// hostGoEnvKeys are the Go cache locations that default to somewhere under HOME. They are
// read from `go env` — the effective value, whether set or defaulted — BEFORE HOME is
// overridden, and set in the process environment so every child `go` (the fake-binary
// builds in TestMain, any `go list` a test runs) keeps using the host's caches.
var hostGoEnvKeys = []string{"GOMODCACHE", "GOPATH", "GOCACHE"}

func pinHostGoEnv() error {
	out, err := exec.Command("go", "env", hostGoEnvKeys[0], hostGoEnvKeys[1], hostGoEnvKeys[2]).Output()
	if err != nil {
		return fmt.Errorf("go env %s: %w", strings.Join(hostGoEnvKeys, " "), err)
	}
	vals := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(vals) != len(hostGoEnvKeys) {
		return fmt.Errorf("go env %s: want %d lines, got %q", strings.Join(hostGoEnvKeys, " "), len(hostGoEnvKeys), out)
	}
	for i, k := range hostGoEnvKeys {
		if vals[i] == "" {
			return fmt.Errorf("go env %s is empty — cannot pin it away from the fixture HOME", k)
		}
		if err := os.Setenv(k, vals[i]); err != nil {
			return err
		}
	}
	return nil
}

// fakeBuildEnv is the environment for a `go build` of a fake binary inside this package's
// TestMain: the process environment (carrying the pinned host caches) with the workspace
// and any inherited GOFLAGS switched off, so the throwaway module builds on its own.
func fakeBuildEnv() []string {
	return append(os.Environ(), "GOWORK=off", "GOFLAGS=")
}

// fixtureLeaked records that a fixture HOME survived its cleanup; finishFixtureRoster turns
// it into a failed run.
var fixtureLeaked bool

// removeFixtureTree deletes the fixture HOME. Everything under it is first made
// user-writable: a Go cache that landed there is 0444 files in 0555 directories, on which a
// bare os.RemoveAll fails with EACCES on macOS — and the closure used to drop that error.
// A removal that still fails is reported on stderr and flagged, never swallowed.
func removeFixtureTree(home string) {
	_ = filepath.WalkDir(home, func(p string, d os.DirEntry, werr error) error {
		if werr != nil {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return nil
		}
		mode := info.Mode().Perm() | 0o200
		if d.IsDir() {
			mode |= 0o300
		}
		_ = os.Chmod(p, mode)
		return nil
	})
	if err := os.RemoveAll(home); err != nil {
		fmt.Fprintf(os.Stderr, "roster fixture: removing %s: %v\n", home, err)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "roster fixture: %s still exists after cleanup — leaked fixture HOME\n", home)
		fixtureLeaked = true
	}
}

// finishFixtureRoster is the last thing TestMain does before os.Exit: run the cleanup
// installFixtureRoster returned, then assert the fixture HOME is actually gone. A cleanup
// that stopped running or stopped working turns the package red instead of leaving one
// directory per run in $TMPDIR (#1195).
func finishFixtureRoster(cleanup func(), code int) int {
	cleanup()
	if fixtureLeaked && code == 0 {
		code = 1
	}
	return code
}
