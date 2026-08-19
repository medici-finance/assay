package deskkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// statusgenSmokeBinEnv names the env var that points at the PINNED,
// checksum-verified released statusgen binary (statusgen is canonical in this
// repository — see the root statusgen/ tree). It is a DEDICATED opt-in for this
// security smoke, deliberately separate from deskboard's STATUSGEN_BIN exec
// override (tools/desk/cmd/deskboard): a developer who points STATUSGEN_BIN at a
// local build to preview a board should not be silently enrolled into a
// fail-first security assertion. CI sets this variable to the pinned released
// binary; a developer can set it to the same pinned binary to run the smoke
// locally.
const statusgenSmokeBinEnv = "ASSAY_STATUSGEN_BIN"

// TestStatusgenTrustGateEnforcedInReleasedBinary is the BEHAVIOURAL companion to
// the source-read coupling in scancoupling_test.go
// (TestScanIssuesTrustGateEnforced). The two are complementary, not a
// replacement:
//
//   - The source-read coupling reads the in-tree statusgen/ source and proves the
//     --scan-issues author trust gate EXISTS and stays wired to the config reader.
//   - This smoke proves the SHIPPED ARTIFACT actually ENFORCES it: it runs the
//     pinned released statusgen binary and asserts its runtime behaviour. Source
//     that says the gate is present and a binary that honours it at run time are
//     different guarantees; defense-in-depth wants both.
//
// WHAT IT ASSERTS. --scan-issues is the surface that creates durable desk work
// items (placeholder files) from GitHub issues on repos arbitrary external users
// can author, so the author trust gate is a security control that must hold in
// the binary we run:
//
//	(1) FAIL-CLOSED ON UNSET ROSTER. With no roster configured the binary REFUSES
//	    to scan (exit 2) rather than admitting every issue ungated.
//	(2) UNTRUSTED AUTHOR REJECTED, TRUSTED AUTHOR ADMITTED. With a roster whose
//	    trusted set EXCLUDES the issue author, that issue is QUARANTINED (no
//	    placeholder); an issue by a trusted author in the SAME batch IS planned.
//	    The two together are the fail-first control: a binary that dropped the
//	    gate would plan a placeholder for the untrusted author, failing (2); a
//	    binary that scanned nothing at all would fail the trusted-author half.
//
// If this test fires, do NOT relax it to green: a released binary that admits an
// untrusted external author's issue as a desk work item is the exact hole the
// gate closes. Re-point the binary env var if the artifact moved; never skip or
// weaken the assertion.
func TestStatusgenTrustGateEnforcedInReleasedBinary(t *testing.T) {
	bin := strings.TrimSpace(os.Getenv(statusgenSmokeBinEnv))
	if bin == "" {
		t.Skipf("%s is unset — this smoke verifies the SHIPPED statusgen artifact enforces the "+
			"--scan-issues author trust gate, so it needs the pinned released binary. Set %s to the "+
			"pinned released statusgen (the same one CI installs from the .assay-versions pin) to run "+
			"this locally; wiring CI to set it is a follow-on.", statusgenSmokeBinEnv, statusgenSmokeBinEnv)
	}
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("%s=%q is not a usable binary: %v", statusgenSmokeBinEnv, bin, err)
	}

	t.Run("untrusted_author_rejected", func(t *testing.T) {
		root := smokeScanRoot(t)
		home := smokeConfigHome(t, smokeRosterExcludingOutsider)
		out, err := runSmokeScan(t, bin, root, home)
		if err != nil {
			t.Fatalf("--scan-issues (dry-run) with a configured roster exited non-zero: %v\n%s", err, out)
		}
		// Positive control: the scan is LIVE — a trusted author's issue is planned.
		if !strings.Contains(out, "testrepo-issue-1.md") {
			t.Fatalf("the trusted-author issue (#1) was NOT planned as a placeholder — the scan is not "+
				"actually running, so the negative assertion below would prove nothing.\n%s", out)
		}
		// The gate: the untrusted author's issue is quarantined, never planned.
		if strings.Contains(out, "testrepo-issue-2.md") {
			t.Fatalf("the UNTRUSTED author (\"outsider\") issue (#2) was planned as a desk work item — "+
				"the released binary's --scan-issues author trust gate is MISSING or bypassed.\n%s", out)
		}
		if !strings.Contains(out, "quarantined") || !strings.Contains(strings.ToLower(out), "outsider") {
			t.Fatalf("the untrusted author (#2) was not surfaced as a quarantine NOTICE — the gate must "+
				"reject it VISIBLY, never silently drop it.\n%s", out)
		}
	})

	t.Run("unconfigured_roster_refuses", func(t *testing.T) {
		root := smokeScanRoot(t)
		home := t.TempDir() // no ~/.config/assay/roster.env under here
		out, err := runSmokeScan(t, bin, root, home)
		if err == nil {
			t.Fatalf("--scan-issues ran to success with NO roster configured — it must fail closed "+
				"(refuse) rather than scan ungated.\n%s", out)
		}
		if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 2 {
			t.Fatalf("expected a REFUSED exit (code 2) on an unconfigured roster, got %v\n%s", err, out)
		}
		if !strings.Contains(out, "REFUSED") || !strings.Contains(out, "NOT CONFIGURED") {
			t.Fatalf("the refusal did not name the unconfigured roster as the reason.\n%s", out)
		}
		if strings.Contains(out, "would create") {
			t.Fatalf("the unconfigured scan planned a placeholder — it must create nothing.\n%s", out)
		}
	})
}

// runSmokeScan invokes `<bin> --scan-issues --root <root> --dry-run` with a
// hermetic environment: HOME points at the fixture config home (so the
// write-class roster resolves to the fixture's ~/.config/assay/roster.env, never
// the developer's), PATH is prefixed with a stub `gh`, and
// STATUSGEN_SCAN_PRIMARY_OK short-circuits the primary-checkout isolation guard
// (the root is a throwaway tempdir).
func runSmokeScan(t *testing.T, bin, root, home string) (string, error) {
	t.Helper()
	cmd := exec.Command(bin, "--scan-issues", "--root", root, "--dry-run")
	cmd.Env = []string{
		"HOME=" + home,
		"PATH=" + smokeStubBin(t) + string(os.PathListSeparator) + os.Getenv("PATH"),
		"STATUSGEN_SCAN_PRIMARY_OK=1",
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// smokeScanRoot builds a minimal scan root: a valid issue-loop stream so
// loadStreams succeeds and placeholders have a home.
func smokeScanRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "issue-loop")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(smokeStreamREADME), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// smokeConfigHome builds a fixture HOME carrying ~/.config/assay/roster.env with
// the given contents, at the owner-only permissions the write-class reader
// enforces (0700 dir, 0600 file).
func smokeConfigHome(t *testing.T, roster string) string {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatal(err)
	}
	return home
}

// smokeStubBin writes a stub `gh` into a fresh dir and returns the dir. The stub
// returns two OPEN issues on the scan repo — #1 by a TRUSTED author, #2 by an
// UNTRUSTED one ("outsider") — and an empty blessing (no authority comment), so
// the untrusted issue has no path to admission except the gate failing.
func smokeStubBin(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gh := filepath.Join(dir, "gh")
	if err := os.WriteFile(gh, []byte(smokeStubGH), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

const smokeStreamREADME = `---
stream: issue-loop
status: active
priority: P1
track: platform
issues: []
---

# Issue-Loop Stream

Smoke fixture stream.

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | Schema | 0 | M | todo | — | — |
`

// smokeRosterExcludingOutsider is a valid write-class roster whose trusted set
// does NOT include "outsider" and whose blessing authority is a different login.
const smokeRosterExcludingOutsider = `ASSAY_BLESS_LOGIN=blessboss:111
ASSAY_TRUSTED_LOGINS=blessboss:111,trusteduser:222
ASSAY_TRUSTED_BOT_SLUGS=worker=some-app:333
ASSAY_ALLOWED_REPOS=testorg/testrepo:ci:public
ASSAY_SCAN_REPOS=testorg/testrepo
`

const smokeStubGH = `#!/usr/bin/env bash
if [ "$1" = "issue" ] && [ "$2" = "list" ]; then
  cat <<'JSON'
[
  {"number":1,"title":"trusted issue","author":{"login":"trusteduser","is_bot":false},"labels":[],"url":"https://github.com/testorg/testrepo/issues/1"},
  {"number":2,"title":"untrusted issue","author":{"login":"outsider","is_bot":false},"labels":[],"url":"https://github.com/testorg/testrepo/issues/2"}
]
JSON
  exit 0
fi
if [ "$1" = "api" ] && [ "$2" = "graphql" ]; then
  echo '{"data":{"repository":{"issue":{"comments":{"nodes":[]}}}}}'
  exit 0
fi
echo "[]"
exit 0
`
