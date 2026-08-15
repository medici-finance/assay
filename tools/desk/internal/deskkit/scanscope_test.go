package deskkit

// #777 — ScanScopeError, the scan-scope twin of BoardScopeError.
//
// BoardScopeError guards the WRITE set (ASSAY_ALLOWED_REPOS); ScanScopeError guards
// the intake SCAN set (ASSAY_SCAN_REPOS) that issueboard's issue lane reads. They are
// DISTINCT keys, and #777 is the gap that distinctness opened: the write set was
// populated (so BoardScopeError stayed silent) while ASSAY_SCAN_REPOS was unset, so
// issueboard swept zero repos and reported a clean, empty board at exit 0. These tests
// pin both axes ScanScopeError checks, and — the load-bearing part — pin that it stays
// quiet once a scan repo is configured, so it cannot be satisfied by refusing all.
//
// installRoster / rosterTrustButNoScope live in boardscope_test.go (same package).

import (
	"strings"
	"testing"
)

const (
	// rosterTrustAndAllowedNoScope is the #777 reproduction: trust configured,
	// ASSAY_ALLOWED_REPOS configured (so BoardScopeError is silent), but the SCAN
	// scope is not — exactly the deploy-layer state that reddened nothing while the
	// board went blind.
	rosterTrustAndAllowedNoScope = rosterTrustButNoScope +
		"ASSAY_ALLOWED_REPOS=medici-finance/assay:ci:private\n"
	// rosterTrustAndScopeSet adds the one thing #777's state is missing.
	rosterTrustAndScopeSet = rosterTrustAndAllowedNoScope +
		"ASSAY_SCAN_REPOS=medici-finance/assay,example-org/agents\n"
)

// TestScanScopeError_AllowedSetButNoScanScope is the axis BoardScopeError does not
// cover and the one that actually reproduced #777: the WRITE boundary is populated,
// so BoardScopeError is SILENT, yet the SCAN scope is empty.
func TestScanScopeError_AllowedSetButNoScanScope(t *testing.T) {
	installRoster(t, rosterTrustAndAllowedNoScope)

	// The two preconditions that make this state interesting — asserted, because if
	// either changes this test stops testing what it claims to.
	if !EffectiveConfig().Configured() {
		t.Fatal("precondition: this roster must report CONFIGURED")
	}
	if err := BoardScopeError(); err != nil {
		t.Fatalf("precondition: BoardScopeError must be SILENT here (the WRITE set is populated) — "+
			"that silence is exactly why the scan scope needed its own guard: %v", err)
	}
	if len(ScanRepos()) != 0 {
		t.Fatalf("precondition: the SCAN scope must be empty in this roster; got %v", ScanRepos())
	}

	err := ScanScopeError()
	if err == nil {
		t.Fatal("ScanScopeError() = nil with an EMPTY scan scope. The issue lane then sweeps zero " +
			"repos and reports the empty sweep as a clean board at exit 0 (#777).")
	}
	if got := ExitCodeOf(err); got != ExitUnverifiable {
		t.Errorf("exit code = %d, want %d: the caller asked a question this process COULD NOT ANSWER (6)",
			got, ExitUnverifiable)
	}
	// The message must be actionable: name the empty state and the variable to set.
	for _, want := range []string{"COULD-NOT-CHECK", EnvScanRepos, ConfigHomePath()} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q:\n%s", want, err.Error())
		}
	}
}

// TestScanScopeError_UnconfiguredRosterDefersToTheLoudRefusal pins axis 1: an absent
// roster gets the roster-unconfigured message (which names the trust variables), not
// the scope message.
func TestScanScopeError_UnconfiguredRosterDefersToTheLoudRefusal(t *testing.T) {
	installRoster(t, "") // an empty roster file — nothing configured

	if EffectiveConfig().Configured() {
		t.Fatal("precondition: an empty roster must be unconfigured")
	}
	err := ScanScopeError()
	if err == nil {
		t.Fatal("ScanScopeError() = nil with NO roster at all")
	}
	if !strings.Contains(err.Error(), "NOT CONFIGURED") {
		t.Errorf("an unconfigured roster got the SCOPE message rather than the roster-unconfigured "+
			"one:\n%s", err)
	}
	if got := ExitCodeOf(err); got != ExitUnverifiable {
		t.Errorf("exit code = %d, want %d", got, ExitUnverifiable)
	}
}

// TestScanScopeError_ConfiguredScopeIsSilent is the POSITIVE CONTROL. The two tests
// above are satisfied by a function that returns an error unconditionally; this is
// what makes them mean something.
func TestScanScopeError_ConfiguredScopeIsSilent(t *testing.T) {
	installRoster(t, rosterTrustAndScopeSet)

	if n := len(ScanRepos()); n != 2 {
		t.Fatalf("precondition: want exactly 2 scan repos, got %d (%v)", n, ScanRepos())
	}
	if err := ScanScopeError(); err != nil {
		t.Fatalf("ScanScopeError() = %v with a configured scan scope — the guard is refusing reads "+
			"it must permit", err)
	}
}
