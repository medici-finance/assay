package main

// multiroot_test.go — FAIL-FIRST coverage for the multi-root `verifyloop plan` (#1309 item 1).
//
// THE DEFECT. `plan` read only `<root>/docs/streams/*/README.md` and knew nothing of DESK_ROOTS,
// so a desk booted on one root saw that root's handful of briefs and called it "the queue" while
// the other configured roots carried the bulk of the dispatchable work. And the envelope
// preflight ran ONCE for the whole pass, so one red sibling would have hidden every root.
//
// WHAT THESE TESTS PIN. With DESK_ROOTS set and no --root, the plan reads every configured root
// (the same deskkit.ConfiguredRoots map deskboard reads), names the root on every item, runs the
// preflight PER ROOT (a red root is reported and skipped, the others still plan), and an explicit
// --root narrows to one. With DESK_ROOTS unset the single-root read is byte-identical to before.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// fixtureRoster is a private-HOME roster naming two allowed repos, so DESK_ROOTS can bind them.
// Neutral fixture values only — never a real bot id or a real repo set.
const fixtureRoster = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_TRUSTED_BOT_SLUGS=verifier=assay-verifier-app:300000005
ASSAY_ALLOWED_REPOS=example-org/tracker:ci:private,example-org/agents:ci:private
`

func plantFixtureRoster(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(fixtureRoster), 0o600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
}

// twoRoots plants one implemented brief in each of two roots and returns their paths.
func twoRoots(t *testing.T) (a, b string) {
	t.Helper()
	a, b = t.TempDir(), t.TempDir()
	table := "| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | x | 0 | S | implemented | — | — |\n"
	writeFixtureStream(t, a, "stream-a", table, map[string]string{"01": briefBody("no", "model", "")})
	writeFixtureStream(t, b, "stream-b", table, map[string]string{"01": briefBody("no", "model", "")})
	return a, b
}

func stubPreflight(t *testing.T, fn func(root string) error) {
	t.Helper()
	old := preflightRoot
	preflightRoot = fn
	t.Cleanup(func() { preflightRoot = old })
}

// TestPlan_MultiRootReadsEveryConfiguredRoot: DESK_ROOTS names two roots, no --root is given —
// the plan must dispatch BOTH briefs, each tagged with its repo and root. Before the fix the
// plan read only `.` (the process cwd, no streams) and dispatched nothing.
func TestPlan_MultiRootReadsEveryConfiguredRoot(t *testing.T) {
	plantFixtureRoster(t)
	a, b := twoRoots(t)
	t.Setenv(deskkit.RootsEnv, "example-org/tracker="+a+",example-org/agents="+b)
	stubPreflight(t, func(string) error { return nil })

	var err error
	out := captureStdout(t, func() { err = cmdPlan(nil) })
	if err != nil {
		t.Fatalf("cmdPlan: %v\n%s", err, out)
	}
	for _, must := range []string{
		"=== DISPATCH example-org/tracker:stream-a/01 (tier=local) root=" + a + " ===",
		"=== DISPATCH example-org/agents:stream-b/01 (tier=local) root=" + b + " ===",
		"2 brief(s) awaiting across 2 root(s)",
		"2 dispatchable",
	} {
		if !strings.Contains(out, must) {
			t.Fatalf("multi-root plan missing %q:\n%s", must, out)
		}
	}
	if !strings.Contains(out, "Repo: example-org/agents — checkout root") {
		t.Fatalf("dispatch prompt does not name the item's root:\n%s", out)
	}
}

// TestPlan_RedSiblingIsSkippedNotFatal: the preflight is PER ROOT. A red sibling is reported and
// skipped; the green root still plans; the exit is could-not-check (6) because the plan is not
// the whole queue. Before the fix one preflight gated the whole pass.
func TestPlan_RedSiblingIsSkippedNotFatal(t *testing.T) {
	plantFixtureRoster(t)
	a, b := twoRoots(t)
	t.Setenv(deskkit.RootsEnv, "example-org/tracker="+a+",example-org/agents="+b)
	stubPreflight(t, func(root string) error {
		if root == b {
			return deskkit.Unverifiable("preflight: cold-mint could-not-check", nil)
		}
		return nil
	})

	var err error
	out := captureStdout(t, func() { err = cmdPlan(nil) })
	if !strings.Contains(out, "=== DISPATCH example-org/tracker:stream-a/01") {
		t.Fatalf("green root was not planned:\n%s", out)
	}
	if strings.Contains(out, "stream-b/01") {
		t.Fatalf("red root's brief leaked into the plan:\n%s", out)
	}
	if !strings.Contains(out, "-- root example-org/agents ("+b+"): PREFLIGHT RED — skipped") {
		t.Fatalf("red root not reported as skipped:\n%s", out)
	}
	if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("a skipped root must make the plan could-not-check (exit 6), got err=%v", err)
	}
	// The preflight ran once for the WHOLE plan before; with a red root that aborted everything.
	if preflightBoot(nil) != nil {
		t.Fatalf("preflightBoot must defer the envelope check to cmdPlan on a multi-root plan")
	}
}

// TestPlan_ExplicitRootNarrowsToOne: --root given with DESK_ROOTS set is the single-root read.
func TestPlan_ExplicitRootNarrowsToOne(t *testing.T) {
	plantFixtureRoster(t)
	a, b := twoRoots(t)
	t.Setenv(deskkit.RootsEnv, "example-org/tracker="+a+",example-org/agents="+b)
	stubPreflight(t, func(string) error { return nil })

	out := captureStdout(t, func() { _ = cmdPlan([]string{"--root", b}) })
	if !strings.Contains(out, "=== DISPATCH stream-b/01 (tier=local) ===") {
		t.Fatalf("explicit --root should plan that root alone with bare IDs:\n%s", out)
	}
	if strings.Contains(out, "stream-a/01") || strings.Contains(out, "root=") {
		t.Fatalf("explicit --root leaked the other root or a provenance tag:\n%s", out)
	}
	if multiRootPlan([]string{"--root=" + b}) || !multiRootPlan(nil) {
		t.Fatalf("multiRootPlan must key on --root presence")
	}
}

// TestPlan_UnsetRootsIsSingleRootAsBefore: with DESK_ROOTS unset the plan is the --root read.
func TestPlan_UnsetRootsIsSingleRootAsBefore(t *testing.T) {
	plantFixtureRoster(t)
	a, _ := twoRoots(t)
	t.Setenv(deskkit.RootsEnv, "")
	if multiRootPlan(nil) {
		t.Fatalf("DESK_ROOTS unset must not be a multi-root plan")
	}
	out := captureStdout(t, func() { _ = cmdPlan([]string{"--root", a}) })
	if !strings.Contains(out, "=== DISPATCH stream-a/01 (tier=local) ===") || strings.Contains(out, "root=") {
		t.Fatalf("single-root output changed:\n%s", out)
	}
}
