package loopengine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// recover_test.go — the crash-recovery replay scenarios (sample/11).
//
// Named TestReplay* so the brief's Verify row 2 (`go test -run TestReplay`) selects exactly
// these four scenarios plus the schema well-formedness check. Each drives Recover against a
// hand-planted journal + claim-dir state and asserts the classification the brief specifies.

// recoverFixture wires a fresh $HOME (so the audit stream + claims dir are isolated) and
// returns a Config whose lineage is (RunnerID=owner, Branch=branch) and whose claims dir is
// under that HOME. The audit stream deskkit.Log writes lands under the same HOME.
func recoverFixture(t *testing.T, owner, branch, runID string) (Config, *fakeLoop, string) {
	t.Helper()
	deskHome := setupDeskHome(t, testLoopName)
	cfg := Config{
		PoolSize:   1,
		ClaimsDir:  filepath.Join(deskHome, "claims"),
		StaleClaim: 120 * time.Minute, // production value — a fresh claim is NOT stale on age
		RunnerID:   owner,
		Branch:     branch,
		runID:      runID,
	}
	// deskHome is returned so a test that needs the audit-stream path builds it from the
	// home directly (deskHome/audit.jsonl) rather than via a parent-relative segment — the
	// CI cross-module-reader scanner flags any test file that pairs a filesystem read with a
	// parent-directory (dot-dot) path segment, and this test reads nothing outside its own
	// temp home.
	return cfg, &fakeLoop{name: "recover-fixture"}, deskHome
}

// plantClaim writes a claim file directly through the deskkit primitive, as a prior run
// would have. Owner+Branch set the lineage the claim belongs to.
func plantClaim(t *testing.T, cfg Config, item, owner, branch string) {
	t.Helper()
	ok, err := deskkit.Acquire(deskkit.ClaimConfig{ClaimsDir: cfg.ClaimsDir, StaleClaim: cfg.StaleClaim},
		deskkit.Claim{Kind: deskkit.KindDispatch, Item: item, Owner: owner, Branch: branch})
	if err != nil || !ok {
		t.Fatalf("plantClaim(%s): ok=%v err=%v", item, ok, err)
	}
}

// journalPrior writes one journal line as a PRIOR run would have (a runID distinct from the
// restart's). It goes through the same structured write path Recover reads.
func journalPrior(t *testing.T, priorRunID, kind, item, outcome string) {
	t.Helper()
	cfg := Config{runID: priorRunID}
	cfg.journalEvent(testLoopName, kind, Item{ID: item}, "", outcome, "")
}

func claimExists(t *testing.T, cfg Config, item string) bool {
	t.Helper()
	_, err := os.Stat(claimPath(cfg, Item{ID: item}))
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	t.Fatalf("stat claim %s: %v", item, err)
	return false
}

func journalHas(t *testing.T, kind, item string) bool {
	t.Helper()
	entries, err := deskkit.LoadEntries()
	if err != nil {
		t.Fatalf("LoadEntries: %v", err)
	}
	for i := range entries {
		rec, ok := parseJournalRecord(entries[i])
		if ok && rec.kind == kind && rec.Item == item && rec.Loop == testLoopName {
			return true
		}
	}
	return false
}

// TestReplayRecoversInFlightWithoutStaleWait is the headline scenario: a prior run claimed
// and dispatched an item, then the process died before it landed. On restart, Recover must
// free that FRESH claim for immediate re-dispatch — proving the 120m staleness wait is NOT
// paid — because the journal RECORDS it was in flight, not because time passed.
func TestReplayRecoversInFlightWithoutStaleWait(t *testing.T) {
	cfg, loop, _ := recoverFixture(t, "worker-app", "brief/le-11", "run-restart")
	// Auto-reclaim requires a WorkEvidence probe: recovery hands the item straight back to the
	// dispatch path, and with no external-evidence probe that path cannot tell whether the
	// prior worker already acted before the crash (see recover.go). A probe that reports the
	// item NOT taken is the configuration under which reclaim is permitted.
	cfg.WorkEvidence = func(Item) (bool, string, error) { return false, "", nil }

	// A prior run of THIS conductor (same owner+branch, a different runID) claimed and
	// dispatched item X, then crashed with no land.
	plantClaim(t, cfg, "sample/03", "worker-app", "brief/le-11")
	journalPrior(t, "run-prior", journalClaim, "sample/03", "")
	journalPrior(t, "run-prior", journalDispatch, "sample/03", "")

	// Sanity: the claim is FRESH — under the ordinary path it would be un-reclaimable for
	// the full 120m StaleClaim window.
	stale, err := deskkit.IsStale(deskkit.ClaimConfig{ClaimsDir: cfg.ClaimsDir, StaleClaim: cfg.StaleClaim}, "sample/03")
	if err != nil {
		t.Fatalf("IsStale: %v", err)
	}
	if stale {
		t.Fatal("precondition failed: a fresh claim must not be stale on age — the test would prove nothing")
	}

	Recover(cfg, loop)

	if claimExists(t, cfg, "sample/03") {
		t.Fatal("in-flight claim was NOT freed — recovery must make it reclaim-eligible immediately (no 120m wait)")
	}
	if !journalHas(t, journalReclaim, "sample/03") {
		t.Error("expected a `reclaim` journal event recording the recovery decision")
	}
}

// TestReplayLeavesAnotherConsumersClaimsUntouched proves the lineage guard: a claim owned by
// a DIFFERENT branch under the same shared app identity (a live sibling worker) is never
// touched, even though its journal shows it in flight. Misreading a live sibling as orphaned
// is the double-dispatch the runID/lineage guard exists to prevent.
func TestReplayLeavesAnotherConsumersClaimsUntouched(t *testing.T) {
	cfg, loop, _ := recoverFixture(t, "worker-app", "brief/le-11", "run-restart")

	// Same owner, DIFFERENT branch => a live sibling's lineage, not mine.
	plantClaim(t, cfg, "sample/07", "worker-app", "brief/other-worker")
	journalPrior(t, "run-sibling", journalClaim, "sample/07", "")
	journalPrior(t, "run-sibling", journalDispatch, "sample/07", "")

	// And a truly foreign consumer (different owner entirely).
	plantClaim(t, cfg, "sample/08", "other-app", "brief/foreign")

	Recover(cfg, loop)

	if !claimExists(t, cfg, "sample/07") {
		t.Error("a live sibling's claim (same owner, different branch) was reclaimed — double-dispatch risk")
	}
	if !claimExists(t, cfg, "sample/08") {
		t.Error("a foreign consumer's claim (different owner) was touched")
	}
	if journalHas(t, journalReclaim, "sample/07") || journalHas(t, journalReclaim, "sample/08") {
		t.Error("recovery emitted a reclaim for a claim that is not this conductor's lineage")
	}
}

// TestReplayCorruptJournalFallsBackConservative proves the fail-closed contract: an
// unreadable journal means NO classification. A mine-and-in-flight claim that recovery WOULD
// have freed on a clean journal is left strictly untouched when the journal is corrupt —
// recovery never guesses from a replay it cannot trust.
func TestReplayCorruptJournalFallsBackConservative(t *testing.T) {
	cfg, loop, deskHome := recoverFixture(t, "worker-app", "brief/le-11", "run-restart")

	// A mine-and-in-flight claim: on a CLEAN journal this would be freed for reclaim.
	plantClaim(t, cfg, "sample/05", "worker-app", "brief/le-11")
	journalPrior(t, "run-prior", journalClaim, "sample/05", "")

	// Corrupt the audit stream: a line that is not valid JSON. LoadEntries fails closed.
	// The audit stream lives at deskHome/audit.jsonl (deskDir resolves to $HOME/.config/assay,
	// which is deskHome); build the path from the home so this file carries no parent-directory
	// (dot-dot) segment.
	auditFile := filepath.Join(deskHome, "audit.jsonl")
	if err := os.WriteFile(auditFile, []byte("{not valid json\n"), 0o600); err != nil {
		t.Fatalf("corrupt audit: %v", err)
	}
	if _, err := deskkit.LoadEntries(); err == nil {
		t.Fatal("precondition failed: the corrupt journal must make LoadEntries fail")
	}

	Recover(cfg, loop)

	if !claimExists(t, cfg, "sample/05") {
		t.Error("recovery reclassified a claim on a CORRUPT journal — must fail closed and change nothing")
	}
}

// TestReplayReleasesCompletedLeftoverClaim proves the completed disposition: a claim whose
// newest journalled event is a TERMINAL land is a leftover the crash left behind after the
// work already landed — recovery releases it (it is done), it does not reclaim it.
func TestReplayReleasesCompletedLeftoverClaim(t *testing.T) {
	cfg, loop, _ := recoverFixture(t, "worker-app", "brief/le-11", "run-restart")

	plantClaim(t, cfg, "sample/02", "worker-app", "brief/le-11")
	journalPrior(t, "run-prior", journalClaim, "sample/02", "")
	journalPrior(t, "run-prior", journalDispatch, "sample/02", "")
	journalPrior(t, "run-prior", journalLand, "sample/02", VerdictPass) // terminal

	Recover(cfg, loop)

	if claimExists(t, cfg, "sample/02") {
		t.Error("a completed leftover claim was not released")
	}
	if !journalHas(t, journalRelease, "sample/02") {
		t.Error("expected a `release` journal event for the completed leftover claim")
	}
	// A completed claim is released, never reclaimed for re-dispatch.
	if journalHas(t, journalReclaim, "sample/02") {
		t.Error("a completed claim was reclaimed for re-dispatch instead of released")
	}
}

// TestReplayJournalEventsWellFormed proves the event schema: every emitted kind round-trips
// through the audit stream carrying a non-empty runID, item, and loop — the run identity the
// brief requires is present and journalled, and parseJournalRecord reads it back.
func TestReplayJournalEventsWellFormed(t *testing.T) {
	setupDeskHome(t, testLoopName)
	cfg := Config{runID: newRunID()}
	if cfg.runID == "" || cfg.runID == "run-unknown" {
		t.Fatalf("newRunID produced no usable token: %q", cfg.runID)
	}

	kinds := []string{journalClaim, journalDispatch, journalRetry, journalLand, journalReclaim, journalRelease, journalRefuse}
	for _, k := range kinds {
		cfg.journalEvent(testLoopName, k, Item{ID: "sample/09"}, "session", "PASS", "note")
	}

	entries, err := deskkit.LoadEntries()
	if err != nil {
		t.Fatalf("LoadEntries: %v", err)
	}
	seen := map[string]bool{}
	for i := range entries {
		rec, ok := parseJournalRecord(entries[i])
		if !ok {
			continue
		}
		if rec.RunID != cfg.runID {
			t.Errorf("event %q carried runID %q, want %q", rec.kind, rec.RunID, cfg.runID)
		}
		if rec.Item == "" || rec.Loop != testLoopName {
			t.Errorf("event %q malformed: item=%q loop=%q", rec.kind, rec.Item, rec.Loop)
		}
		seen[rec.kind] = true
	}
	for _, k := range kinds {
		if !seen[k] {
			t.Errorf("kind %q did not round-trip through the audit stream", k)
		}
	}
}

// TestReplayRefusesReclaimWithoutWorkEvidence proves the finding-1 guard: with NO WorkEvidence
// probe configured, an in-flight claim of a prior run is NOT auto-reclaimed. Recovery would
// otherwise hand the item to a dispatch path that consults only the claims dir — re-executing
// an outward intent without checking whether the prior worker already posted/flipped/filed. The
// claim is left for the age-only stale-claim heuristic instead.
func TestReplayRefusesReclaimWithoutWorkEvidence(t *testing.T) {
	cfg, loop, _ := recoverFixture(t, "worker-app", "brief/le-11", "run-restart")
	// cfg.WorkEvidence is nil (the fixture default) — the configuration under test.

	plantClaim(t, cfg, "sample/03", "worker-app", "brief/le-11")
	journalPrior(t, "run-prior", journalClaim, "sample/03", "")
	journalPrior(t, "run-prior", journalDispatch, "sample/03", "")

	Recover(cfg, loop)

	if !claimExists(t, cfg, "sample/03") {
		t.Error("in-flight claim was freed with NO WorkEvidence probe — recovery must leave it to the age-only heuristic, not remove the only remaining barrier")
	}
	if journalHas(t, journalReclaim, "sample/03") {
		t.Error("recovery emitted a reclaim with no WorkEvidence probe configured")
	}
}

// TestReplayCapsRepeatedReclaims proves the cross-boot crash-loop bound: an item already
// recovery-reclaimed MaxRecoveryReclaims times is not auto-reclaimed again, even with a
// WorkEvidence probe. Without this an item that itself crashes the conductor is re-dispatched
// once per boot forever (the retry bounds reset every boot).
func TestReplayCapsRepeatedReclaims(t *testing.T) {
	cfg, loop, _ := recoverFixture(t, "worker-app", "brief/le-11", "run-restart")
	cfg.WorkEvidence = func(Item) (bool, string, error) { return false, "", nil }

	plantClaim(t, cfg, "sample/03", "worker-app", "brief/le-11")
	journalPrior(t, "run-prior", journalClaim, "sample/03", "")
	journalPrior(t, "run-prior", journalDispatch, "sample/03", "")
	// MaxRecoveryReclaims prior reclaim events for this item, from earlier boots.
	for i := 0; i < MaxRecoveryReclaims; i++ {
		journalPrior(t, "run-boot", journalReclaim, "sample/03", "")
	}

	Recover(cfg, loop)

	if !claimExists(t, cfg, "sample/03") {
		t.Errorf("in-flight claim was reclaimed a %d+1th time — the crash-loop cap (%d) must stop auto-reclaiming it", MaxRecoveryReclaims, MaxRecoveryReclaims)
	}
}

// TestReplayLineageLockExcludesLiveSibling proves the boot-flock (finding 2): a SECOND live
// conductor of the same (RunnerID, Branch) lineage cannot take the lock the first one holds, so
// it must not recover (its "prior-run" claims may be the live first conductor's). A DIFFERENT
// lineage is unaffected, and the lock is reusable once released (a crashed conductor's flock is
// dropped by the OS, which the release() call stands in for here).
func TestReplayLineageLockExcludesLiveSibling(t *testing.T) {
	cfg, _, _ := recoverFixture(t, "worker-app", "brief/le-11", "run-a")

	first, sole := acquireLineageLock(cfg)
	if !sole || first == nil {
		t.Fatal("first conductor of a lineage must be the sole live holder")
	}

	// A second live conductor of the SAME lineage (a fresh runID, same owner+branch) must be
	// refused the lock — it is not the sole live conductor, so it must not recover.
	if _, sole2 := acquireLineageLock(cfg); sole2 {
		t.Error("a second live conductor of the same lineage acquired the lineage lock — recovery would delete the first conductor's LIVE claims")
	}

	// A different lineage (different branch) is independent.
	other := cfg
	other.Branch = "brief/other-worker"
	otherLock, soleOther := acquireLineageLock(other)
	if !soleOther {
		t.Error("a DIFFERENT lineage was blocked by an unrelated conductor's lock")
	}
	otherLock.release()

	// Once the first conductor exits (flock dropped), the lineage is re-acquirable.
	first.release()
	reacquired, soleAgain := acquireLineageLock(cfg)
	if !soleAgain {
		t.Error("lineage lock was not re-acquirable after the holder released it")
	}
	reacquired.release()
}

// TestReplayLineageLockUnanchored proves the fail-closed edge: a config that cannot anchor a
// lineage (no RunnerID or no Branch) never acquires the lock, so Run never recovers on it —
// consistent with classify, which also classifies nothing without a lineage.
func TestReplayLineageLockUnanchored(t *testing.T) {
	cfg, _, _ := recoverFixture(t, "", "brief/le-11", "run-a")
	if _, sole := acquireLineageLock(cfg); sole {
		t.Error("a config with no RunnerID acquired a lineage lock")
	}
	cfg2, _, _ := recoverFixture(t, "worker-app", "", "run-a")
	if _, sole := acquireLineageLock(cfg2); sole {
		t.Error("a config with no Branch acquired a lineage lock")
	}
}
