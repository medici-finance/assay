package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// helpMainEnv makes TestMain run the real main() in a re-exec of this test binary.
const helpMainEnv = "DESKPOST_TEST_RUN_MAIN"

// helpLeakRoster is synthetic; every value is distinctive, so one substring hit is a leak.
const helpLeakRoster = `ASSAY_BLESS_LOGIN=qzpostbless:9201
ASSAY_TRUSTED_LOGINS=qzpostbless:9201,qzposttrust:9202
ASSAY_TRUSTED_BOT_SLUGS=reviewer=qz-post-bot:9203
ASSAY_ALLOWED_REPOS=qzorg/qzpostrepo:ci:private
`

var helpLeakValues = []string{"qzpostbless", "9201", "qzposttrust", "9202", "qz-post-bot", "9203", "qzpostrepo"}

// runDeskpostMain runs the real main() as a child process with a private HOME holding
// helpLeakRoster, and returns its stdout and stderr separately.
func runDeskpostMain(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(helpLeakRoster), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), helpMainEnv+"=1", "HOME="+home, "XDG_CONFIG_HOME=",
		"ASSAY_CONFIG_ECHO=full", "DESK_LOOP=pr-review-desk")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	code := 0
	if err := cmd.Run(); err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running main: %v", err)
		}
		code = ee.ExitCode()
	}
	return out.String(), errb.String(), code
}

// regression: #3754
func TestHelpPrintsNoRosterValues(t *testing.T) {
	for _, args := range [][]string{
		{"finish", "--help"}, {"--help"}, {"-h"}, {"help"},
		{"review", "--help"}, {"finish", "review", "-h"}, {"finish", "security-review", "--help"},
	} {
		stdout, stderr, code := runDeskpostMain(t, args...)
		if code != 0 {
			t.Errorf("%q: help must exit 0, got %d\nstderr:\n%s", args, code, stderr)
		}
		if !strings.Contains(stderr, "usage:") {
			t.Errorf("%q: the usage screen is missing from stderr:\n%s", args, stderr)
		}
		for _, v := range helpLeakValues {
			if strings.Contains(stdout, v) {
				t.Errorf("%q: stdout carries the roster value %q:\n%s", args, v, stdout)
			}
			if strings.Contains(stderr, v) {
				t.Errorf("%q: stderr carries the roster value %q:\n%s", args, v, stderr)
			}
		}
	}
}

// regression: #3754
func TestRealRunStillEchoes(t *testing.T) {
	// The negative control: an invocation that is not a help request keeps the full echo.
	// It stops at its own usage error (no PR), before any forge call.
	_, stderr, _ := runDeskpostMain(t, "finish", "review")
	if !strings.Contains(stderr, "qzposttrust:9202") {
		t.Errorf("a non-help invocation must still echo the effective roster:\n%s", stderr)
	}
}
