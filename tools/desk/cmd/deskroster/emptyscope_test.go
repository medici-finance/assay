package main

// #494 — deskroster's two AllowedRepos() loops (shortToFull, and
// the unclaimed-PR sweep) were not covered by #489's fix, which
// closed the same gap in deskboard. `deskroster list` is a board-wide READ: it takes no
// repo argument and sweeps deskkit.AllowedRepos() to resolve beacon repo names AND to
// scan for unclaimed open PRs. An empty repo set makes both loops iterate zero times,
// and before this fix that read nothing and reported it as a clean, silent result —
// "(no registered work)" with no unclaimed section at all — at exit 0.
//
// This is lower severity than deskboard's `actions` (no positive false statement like
// "MERGED — drop from your list", and it gates nothing), which is why #489 left it out
// and #494 was filed to not lose it. The fix shape is identical: gate on
// len(allowedRepos()) == 0 via deskkit.BoardScopeError(), NOT on Configured() or
// Problems — the reproducing state is configured=true, problems=0, repos=0, and keying
// on either of the first two misses it entirely (the load-bearing finding from #489,
// carried over unchanged).
//
// Every test here was run against the UNFIXED tree and observed to fail with the
// production symptom before the guard was added; the symptom each one reproduces is
// named in its doc comment so a future reader can re-run that experiment.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// partialRosterNoRepos is the state that actually reproduces #494 (and reproduced
// #489 before it): not an ABSENT roster, but a USABLE one with the scope missing.
// Config.Configured() answers TRUE here — it consults Problems, Bless and Logins and
// never consults Repos — so every "is the roster configured?" check in the tree is
// satisfied while there is nothing to read.
const partialRosterNoRepos = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001,shared-agent:2002
ASSAY_TRUSTED_BOT_SLUGS=reviewer=assay-reviewer-app:300000004
ASSAY_HUMAN_LOGIN_MAP=alex:ada
`

// installRosterEnv writes body as the roster.env under a fresh HOME and reloads config,
// mirroring deskboard's installBoardRoster (unboundrole_test.go) for the sibling fix.
func installRosterEnv(t *testing.T, body string) string {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("CLAUDE_SESSION_ID", "test-session")
	t.Setenv("PATH", fakeRosterGHDir+string(os.PathListSeparator)+origRosterPATH)
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
	return home
}

// emptyScopeRoster installs the partial roster and asserts the reproducing
// precondition, exactly as deskboard's emptyScopeBoard does for #489.
func emptyScopeRoster(t *testing.T) string {
	t.Helper()
	home := installRosterEnv(t, partialRosterNoRepos)

	if !deskkit.EffectiveConfig().Configured() {
		t.Fatal("precondition: the partial roster must load as CONFIGURED — that is the whole " +
			"point of #494 (inherited from #489); if it now reports unconfigured, " +
			"Configured() gained a Repos check and this test is asserting against a contract " +
			"that changed")
	}
	if n := len(deskkit.AllowedRepos()); n != 0 {
		t.Fatalf("precondition: the allowed-repo set must be EMPTY, got %d", n)
	}
	return home
}

// TestList_EmptyScopeReportsCouldNotCheck is the #494 headline.
//
// UNFIXED SYMPTOM (observed): exit 0, "(no registered work)", and no "unclaimed"
// section at all — the sweep that would have populated it iterated zero repos and
// nothing distinguished that from there being nothing to report.
func TestList_EmptyScopeReportsCouldNotCheck(t *testing.T) {
	emptyScopeRoster(t)
	t.Setenv("FAKEGH_LIST_PRS", "99:true:unclaimed feature")

	rc := run([]string{"list"})
	if rc != deskkit.ExitUnverifiable {
		t.Errorf("list rc = %d, want %d (unverifiable): a sweep of zero repos knows nothing, "+
			"and exit 0 tells every caller it does", rc, deskkit.ExitUnverifiable)
	}
}

// TestList_ConfiguredScopeStillWorks is the POSITIVE CONTROL for the guard above: with
// the ordinary fixture roster (a non-empty repo set) `list` must still succeed at exit
// 0. Without this, the guard could "fix" #494 by refusing every run.
func TestList_ConfiguredScopeStillWorks(t *testing.T) {
	rosterSetup(t)
	if len(deskkit.AllowedRepos()) == 0 {
		t.Fatal("precondition: the fixture roster must configure a non-empty repo set")
	}
	t.Setenv("FAKEGH_LIST_PRS", "")

	rc := run([]string{"list"})
	if rc != deskkit.ExitOK {
		t.Fatalf("list rc = %d, want %d (ok) with a CONFIGURED scope — the guard is refusing "+
			"work it should permit", rc, deskkit.ExitOK)
	}
}

// TestSetDropMine_NotScopeGuarded records a deliberate exclusion, mirroring
// TestNextupIsNotScopeGuarded in deskboard. set/drop act on a single named repo+PR the
// caller already supplied, and mine reads a single session's own beacon — neither
// sweeps deskkit.AllowedRepos(), so neither needs (or gets) the scope guard. This
// pins that the guard was added to list, and only list.
func TestSetDropMine_NotScopeGuarded(t *testing.T) {
	emptyScopeRoster(t)
	t.Setenv("DESK_SESSION", "alice")

	// set/drop degrade to their own existing behaviour (they act on a caller-named
	// repo+PR regardless of the configured scope) and must not be swallowed by the
	// list guard, which they never call.
	if rc := run([]string{"set", "--repo", "r", "--pr", "1", "--what", "x"}); rc == deskkit.ExitUnverifiable {
		t.Errorf("set rc = %d — the list scope guard leaked into set", rc)
	}
	if rc := run([]string{"drop", "--repo", "r", "--pr", "1"}); rc == deskkit.ExitUnverifiable {
		t.Errorf("drop rc = %d — the list scope guard leaked into drop", rc)
	}
	if rc := run([]string{"mine"}); rc == deskkit.ExitUnverifiable {
		t.Errorf("mine rc = %d — the list scope guard leaked into mine", rc)
	}
}

// TestBoardScopeErrorMessageNamesTheState pins the message content, so the fix is not
// just an exit code with no explanation.
func TestBoardScopeErrorMessageNamesTheState(t *testing.T) {
	emptyScopeRoster(t)
	err := deskkit.BoardScopeError()
	if err == nil {
		t.Fatal("expected a non-nil error from an empty repo set")
	}
	if !strings.Contains(err.Error(), "COULD-NOT-CHECK") {
		t.Errorf("error does not name the state it is in: %q", err.Error())
	}
}
