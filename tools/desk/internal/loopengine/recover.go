package loopengine

import (
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// recover.go — boot-time crash recovery by replaying the scheduling journal.
//
// A restarted conductor has no in-memory pool: the process died, and with it the map of
// which items were in flight. What SURVIVES is (a) the claim files on disk and (b) the
// append-only journal (journal.go). Recover replays the journal — scoped to this consumer's
// loop name — to classify every claim file present on disk, replacing the old boot heuristic
// ("is this claim mine? is it live?") with a RECORD:
//
//	mine-and-completed      → release        (a leftover claim whose work already landed)
//	mine-and-in-flight       → reclaim-eligible immediately, NO 120m wait
//	another consumer's        → untouched
//
// # Fail-closed, like idempotent.go (the load-bearing contract)
//
// A corrupt or unreadable journal means NO classification at all. Recover does not guess
// from a partial replay: it logs a conservative fallback and returns having touched nothing,
// leaving every claim to the existing conservative heuristics (the age-only stale-claim
// path in claim.go). Misclassifying a live claim as orphaned double-dispatches precisely
// when trust is lowest — recovery code runs exactly when state is already abnormal — so the
// only safe answer to "I cannot read my own history" is to change nothing.
//
// # Recovery changes claim DISPOSITION TIMING only
//
// Recover never takes ownership of another run's work by rewriting a claim to itself. For an
// in-flight claim of a prior run of ME it RELEASES the claim file, so the item becomes
// immediately re-claimable through the ordinary lock-guarded single-winner path in
// claim.go (the next fillPool's Claim → deskkit.Acquire O_EXCL). That is what "reclaim
// eligible immediately (no 120m wait)" means: recovery clears the staleness barrier; the
// ACTUAL reclaim still routes through Acquire, where the flock makes exactly one winner.
// This is recovery bookkeeping, not durable execution — the engine recovers the SCHEDULER's
// view, it does not resume a dispatched worker mid-flight (workers are artifact-durable).

// MaxRecoveryReclaims caps how many times crash recovery will auto-reclaim the SAME item
// across boots before it stops and leaves the item to the age-only heuristic / a human. It
// bounds the crash-loop where an item that itself crashes the conductor is re-dispatched once
// per boot indefinitely — the retry bounds are per-Run() and reset at every boot,
// so this is the only cross-boot bound on re-dispatch.
const MaxRecoveryReclaims = 3

// recoverClass is the classification recover.go assigns a claim file.
type recoverClass int

const (
	recoverUntouched recoverClass = iota // another consumer's, or not classifiable — leave it
	recoverRelease                       // mine and completed — release the leftover claim
	recoverReclaim                       // mine and in flight at crash — make reclaim-eligible now
)

// Recover replays this loop's journal at boot and applies the classification above to every
// claim file on disk. It is called once by Run() before the first pool fill. It never
// returns an error that aborts the drain: a journal it cannot read is a conservative
// fallback (change nothing), announced on Progress, not a crash.
func Recover(cfg Config, loop Loop) {
	// No claims directory => nothing on disk to classify.
	if cfg.ClaimsDir == "" {
		return
	}
	// Cannot anchor lineage (no RunnerID or no Branch) => classify can never classify a
	// single claim, so parsing the whole shared audit stream would be pure waste. Return
	// before the read — this is the cheap early exit, not a policy change (classify makes the
	// same decision item-by-item).
	if cfg.RunnerID == "" || cfg.Branch == "" {
		return
	}
	loopName := loop.Name()

	// Read the whole journal. LoadEntries fails closed (exit 6) on a corrupt/unreadable
	// audit file; here that error is our signal to make NO classification and fall back to
	// the existing conservative heuristics — never a partial replay. NOTE: LoadEntries refuses
	// the WHOLE file on any malformed line, so a torn final line (a crash mid-append — the very
	// event this feature recovers from) makes recovery UNAVAILABLE and falls back to the
	// age-only heuristic. That is the fail-closed direction; it is not the same as "nothing to
	// recover".
	entries, err := deskkit.LoadEntries()
	if err != nil {
		cfg.logf("recover: journal unreadable — fail-closed, conservative fallback (NO reclassification): %v", err)
		return
	}

	// Latest journalled event per item, scoped to THIS loop's history, plus how many times
	// each item has ALREADY been recovery-reclaimed (the cross-boot reclaim counter).
	latest := latestByItem(entries, loopName)
	reclaims := reclaimCounts(entries, loopName)

	claims, cerr := deskkit.List(toClaimConfig(cfg))
	if cerr != nil {
		cfg.logf("recover: claims dir unreadable — conservative fallback (NO reclassification): %v", cerr)
		return
	}

	var released, reclaimed int
	for _, c := range claims {
		class, rec := cfg.classify(loopName, c, latest)
		switch class {
		case recoverRelease:
			// Compare-and-delete under the claims-dir flock: remove the leftover ONLY if the
			// on-disk claim still matches the one we classified. If another consumer reclaimed
			// it in place between List and now, its ts differs and we leave it alone.
			ok, err := deskkit.ReleaseMatching(toClaimConfig(cfg), c)
			if err != nil {
				cfg.logf("recover: could not release completed leftover claim %s: %v", c.Item, err)
				continue
			}
			if !ok {
				cfg.logf("recover: completed leftover claim %s changed under recovery (reclaimed/rewritten elsewhere) — left untouched", c.Item)
				continue
			}
			cfg.journalEvent(loopName, journalRelease, Item{ID: c.Item}, rec.Tier, rec.Outcome, "recovery: completed leftover claim released at boot")
			cfg.logf("recover: released completed leftover claim %s (last event %q)", c.Item, rec.kind)
			released++
		case recoverReclaim:
			// A reclaim FREES the barrier and hands the item straight back to the ordinary
			// dispatch path, which re-EXECUTES an outward intent (post/flip/file). With no
			// WorkEvidence probe configured, that dispatch path consults only the claims dir —
			// it cannot see whether the prior worker already posted/flipped/filed before the
			// crash. Removing the staleness barrier in that configuration is removing the ONLY
			// remaining check, so refuse the auto-reclaim and leave the claim to the age-only
			// stale-claim heuristic (which still frees it after StaleClaim).
			if cfg.WorkEvidence == nil {
				cfg.logf("recover: in-flight claim %s NOT auto-reclaimed — no WorkEvidence probe configured, so re-dispatch would re-execute an outward intent without checking whether the prior worker already acted; left to the age-only stale-claim heuristic (last event %q)", c.Item, rec.kind)
				continue
			}
			// A claim that has already been recovery-reclaimed MaxRecoveryReclaims times is an
			// item that keeps crashing the conductor. The retry bounds reset every
			// boot, so without this a crash-loop item is re-dispatched once per boot forever.
			// Stop auto-reclaiming it and leave it to the age heuristic / a human.
			if reclaims[c.Item] >= MaxRecoveryReclaims {
				cfg.logf("recover: in-flight claim %s already recovery-reclaimed %d time(s) (cap %d) — NOT auto-reclaimed again; a repeatedly-crashing item is left to the age-only heuristic / a human", c.Item, reclaims[c.Item], MaxRecoveryReclaims)
				continue
			}
			// Compare-and-delete under the flock (see recoverRelease). Release the in-flight
			// claim so the ordinary lock-guarded Acquire can re-claim it fresh on the next
			// fillPool — immediately, with no 120m staleness wait. The reclaim itself is
			// Acquire's single-winner; recovery only clears the barrier.
			ok, err := deskkit.ReleaseMatching(toClaimConfig(cfg), c)
			if err != nil {
				cfg.logf("recover: could not free in-flight claim %s for reclaim: %v", c.Item, err)
				continue
			}
			if !ok {
				cfg.logf("recover: in-flight claim %s changed under recovery (reclaimed/rewritten elsewhere) — left untouched", c.Item)
				continue
			}
			cfg.journalEvent(loopName, journalReclaim, Item{ID: c.Item}, rec.Tier, "", "recovery: prior run's in-flight claim made reclaim-eligible at boot (no 120m wait)")
			cfg.logf("recover: reclaim-eligible %s — prior run in flight at crash (last event %q), freed for immediate re-dispatch", c.Item, rec.kind)
			reclaimed++
		default:
			// Untouched: another consumer's claim, or one this replay cannot positively
			// classify. The existing conservative heuristics own it.
		}
	}
	if released > 0 || reclaimed > 0 {
		cfg.logf("recover: reclassified %d completed + %d in-flight claim(s) for loop %q", released, reclaimed, loopName)
	}
}

// latestByItem returns, per item, the LAST journalled record for the given loop — the
// record that decides the item's disposition (terminal vs in-flight). Entries are in
// append order, so the last matching record for an item is its most recent decision.
func latestByItem(entries []deskkit.Entry, loopName string) map[string]journalRecord {
	latest := map[string]journalRecord{}
	for i := range entries {
		rec, ok := parseJournalRecord(entries[i])
		if !ok || rec.Loop != loopName {
			continue
		}
		latest[rec.Item] = rec
	}
	return latest
}

// reclaimCounts returns, per item, how many `reclaim` events this loop's history already
// carries — the cross-boot count of times recovery has freed that item. It bounds the
// crash-loop (MaxRecoveryReclaims): the shared audit stream is append-only and survives boots,
// so a reclaim journalled by a prior boot is still counted today.
func reclaimCounts(entries []deskkit.Entry, loopName string) map[string]int {
	counts := map[string]int{}
	for i := range entries {
		rec, ok := parseJournalRecord(entries[i])
		if !ok || rec.Loop != loopName || rec.kind != journalReclaim {
			continue
		}
		counts[rec.Item]++
	}
	return counts
}

// classify decides one claim file's disposition. It is a PURE function of the claim record,
// this run's identity, and the replayed journal — no IO — so it is directly unit-testable.
//
// Lineage is the safety hinge. A claim is "mine" only when its Owner matches this run's
// RunnerID AND its Branch matches this run's Branch: Owner alone is ambiguous because
// batch-fanout shares one app identity across concurrently-live siblings, and the Branch is
// the per-conductor discriminator a restart preserves. With no RunnerID or no Branch to
// anchor lineage, recovery cannot safely tell itself from a sibling, so it classifies
// nothing (conservative). Once lineage is established, the journal's runID must belong to a
// DIFFERENT run than this one (a prior run) — never the current run — before an in-flight
// claim is touched. That a "prior run" is a CRASHED run (not a still-live sibling) is not
// decided here: Run() only calls classify when it holds the per-lineage boot flock
// (lineagelock.go), which a live sibling would still be holding. classify itself assumes that
// exclusion has already been established.
func (cfg Config) classify(loopName string, c deskkit.Claim, latest map[string]journalRecord) (recoverClass, journalRecord) {
	// Cannot anchor lineage => cannot distinguish self from a live sibling => touch nothing.
	if cfg.RunnerID == "" || cfg.Branch == "" {
		return recoverUntouched, journalRecord{}
	}
	// Another consumer's claim (different app identity, or a different branch under the same
	// shared identity) is left strictly untouched.
	if c.Owner != cfg.RunnerID || c.Branch != cfg.Branch {
		return recoverUntouched, journalRecord{}
	}
	// Recovery is a DISPATCH-claim concern only. Kinds share one on-disk file per item, so a
	// route/file/verify/close claim under my lineage is not a dispatch I made — leave it to its
	// own adopter. Empty Kind is accepted as a legacy dispatch record (pre-Kind claims).
	if c.Kind != "" && c.Kind != deskkit.KindDispatch {
		return recoverUntouched, journalRecord{}
	}
	// Mine by lineage. Now the journal decides completed vs in-flight.
	rec, ok := latest[c.Item]
	if !ok {
		// No journalled decision for a claim under my own lineage: cannot positively
		// classify it, so fall back to the conservative heuristic rather than guess.
		return recoverUntouched, journalRecord{}
	}
	// The latest event must come from a DIFFERENT run than this one. My current run has
	// journalled nothing at boot, so this only guards against the pathological case of a
	// claim whose newest event is somehow already this run's — never a reason to reclaim.
	if rec.RunID == cfg.runID {
		return recoverUntouched, rec
	}
	if terminalKind(rec.kind) {
		// Completed: work already landed/released, but a leftover claim survived the crash.
		return recoverRelease, rec
	}
	// In flight at crash: a claim/dispatch/retry with no terminal event after it.
	return recoverReclaim, rec
}
