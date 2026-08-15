package main

// #777 regression — an EMPTY intake scan scope (ASSAY_SCAN_REPOS unset) must FAIL
// CLOSED, never report a clean, empty board at exit 0.
//
// The pre-fix bug: ownedRepos() is deskkit.ScanRepos(); with ASSAY_SCAN_REPOS unset
// it is empty, so computeIssueBoard's per-repo loop ran zero iterations, made no gh
// call, and the empty result rendered as "(no open issues across owned repos)" at
// exit 0 — a clean front door over a board owning 200+ open issues. The #489
// BoardScope guard only ever covered the WRITE set (ASSAY_ALLOWED_REPOS, which was
// populated), so the scan scope had no loud-refusal twin; deskkit.ScanScopeError()
// is that twin, wired into computeIssueBoard.
//
// The package's TestMain plants a fixture roster that SETS ASSAY_SCAN_REPOS, so no
// pre-existing test ever exercised the empty-key state. These tests install a roster
// WITHOUT the scan key to reproduce it directly.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// rosterMissingScan is the fixture roster with the ASSAY_SCAN_REPOS line removed — the
// #777 deploy-layer state: trust and ASSAY_ALLOWED_REPOS set (so the #489 BoardScope
// guard stays silent), but the scan scope unset.
func rosterMissingScan() string {
	var b strings.Builder
	for _, ln := range strings.Split(fixtureRoster, "\n") {
		if strings.HasPrefix(ln, deskkit.EnvScanRepos+"=") {
			continue
		}
		b.WriteString(ln)
		b.WriteString("\n")
	}
	return b.String()
}

// installScanlessRoster points HOME at a fresh home carrying a roster with no
// ASSAY_SCAN_REPOS and reloads the config cache. It registers a ReloadConfig cleanup
// FIRST (so it runs LAST, after t.Setenv restores HOME) so the package's TestMain
// fixture is back in force for whatever test runs next — no stale scan-less cache
// leaks across tests.
func installScanlessRoster(t *testing.T) {
	t.Helper()
	t.Cleanup(deskkit.ReloadConfig)
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("installing the scan-less roster: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(rosterMissingScan()), 0o600); err != nil {
		t.Fatalf("installing the scan-less roster: %v", err)
	}
	deskkit.ReloadConfig()
}

// TestEmptyScanScope_FailsClosed_NotEmptyBoard is the primary #777 regression: with
// ASSAY_SCAN_REPOS unset, both scan-dependent verbs (board, issues) must refuse LOUDLY
// at exit 6 and NEVER print the clean-and-empty board.
func TestEmptyScanScope_FailsClosed_NotEmptyBoard(t *testing.T) {
	installScanlessRoster(t)
	t.Setenv("DESK_TOOLS_DISABLED", "") // kill switch disarmed

	// The precondition that reproduces #777: the write boundary is populated (so
	// BoardScopeError is silent) while ownedRepos()/ScanRepos() is empty.
	if got := ownedRepos(); len(got) != 0 {
		t.Fatalf("precondition: ownedRepos() must be empty with %s unset; got %v", deskkit.EnvScanRepos, got)
	}

	for _, sub := range []string{"board", "issues"} {
		t.Run(sub, func(t *testing.T) {
			root := t.TempDir()
			var out, errb bytes.Buffer
			code := run([]string{"--root", root, sub}, &out, &errb)
			if code != 6 {
				t.Fatalf("run(%s) with empty scan scope = exit %d, want 6 (COULD-NOT-CHECK, never exit-0-empty)",
					sub, code)
			}
			if strings.Contains(out.String(), "no open issues across owned repos") {
				t.Errorf("run(%s) emitted the clean-and-empty board that #777 is about:\n%s", sub, out.String())
			}
			if out.Len() != 0 {
				t.Errorf("a refused run must not emit a (partial) board on stdout; got:\n%s", out.String())
			}
			if !strings.Contains(errb.String(), deskkit.EnvScanRepos) ||
				!strings.Contains(errb.String(), "COULD-NOT-CHECK") {
				t.Errorf("run(%s) refusal must name %s and COULD-NOT-CHECK; got:\n%s",
					sub, deskkit.EnvScanRepos, errb.String())
			}
		})
	}
}

// TestEmptyScanScope_IntakeLaneUnaffected proves the guard is scoped to the ISSUE
// lane: the intake lane reads local files only and must still work with an empty scan
// scope. A guard that reddened intake too would be over-broad.
func TestEmptyScanScope_IntakeLaneUnaffected(t *testing.T) {
	installScanlessRoster(t)
	t.Setenv("DESK_TOOLS_DISABLED", "")

	root := t.TempDir()
	var out, errb bytes.Buffer
	code := run([]string{"--root", root, "intake"}, &out, &errb)
	if code != 0 {
		t.Fatalf("run(intake) with empty scan scope = exit %d, want 0 — the intake lane does not sweep "+
			"the scan scope; stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "no untriaged intake entries") {
		t.Errorf("run(intake) should render the (empty) intake lane; got:\n%s", out.String())
	}
}
