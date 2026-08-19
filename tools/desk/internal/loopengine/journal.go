package loopengine

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// journal.go — the scheduling-decision JOURNAL, and the runID that gives it lineage.
//
// The engine's pool and in-flight bookkeeping die with the process. This file makes the
// scheduler's view RECOVERABLE the way Temporal recovers a workflow: every scheduling
// decision is journalled to the append-only audit stream, and recover.go replays it at
// boot to classify the claim files left on disk (see recover.go). No new file, no new
// location — journal lines are a new event FAMILY (Tool "loopengine") in the existing
// deskkit audit schema, written through deskkit.Log, never a raw file write.
//
// # runID: the lineage token (the under-specification lesson, applied)
//
// Every event carries a runID — a per-Run() unique token minted in Run(). It is the field
// that lets a RESTARTED conductor tell "claimed by a prior run of ME" from "claimed by
// another live consumer": a claim file records only a stable Owner (the app identity, which
// batch-fanout SHARES across concurrently-live workers) and the Branch. Owner alone is
// therefore ambiguous — a sibling worker on a different branch wears the same Owner. The
// lineage that a restart preserves is (Owner, Branch); the runID proves the journalled
// event came from a DIFFERENT run instance of that lineage than the one now booting. A
// different-run event under my own lineage is then a PRIOR run of me — but "prior" only means
// "crashed, safe to reclaim" if no OTHER live conductor of this lineage exists right now: a
// live sibling's in-flight claim is byte-identical to a crashed run's. That premise is NOT
// assumed — it is ENFORCED by the per-lineage boot flock in lineagelock.go, which recover.go
// takes before it will classify anything (a live sibling holds the lock, so recovery is
// skipped). The runID is still the discriminator between run instances; the flock is what makes
// "a prior run of me" imply "a crashed run of me". An event that omitted the runid would be, in
// the idempotent.go sense, indistinguishable from a live sibling's even with the flock.
//
// This groundwork was seeded earlier (retry.go still journals `Verb: "retry"`
// lines directly, part of the same family); the seven-verb vocabulary below is the full set.

// Journal event kinds — the vocabulary emitted at every scheduling decision point. They are
// the `verb` of the audit line (Tool is always "loopengine").
const (
	journalClaim    = "claim"    // a claim file was acquired for an item
	journalDispatch = "dispatch" // a worker was dispatched under a live handle
	journalRetry    = "retry"    // a dispatch attempt was retried (emitted by retry.go)
	journalLand     = "land"     // a result was made durable — carries the outcome class
	journalReclaim  = "reclaim"  // recovery reclaimed a prior run's in-flight claim
	journalRelease  = "release"  // a claim was released (completed leftover, or failed dispatch)
	journalRefuse   = "refuse"   // dispatch refused (author == runner)
)

// terminalKind reports whether a kind is a TERMINAL disposition for an item — the marker
// recover.go keys "completed" on. A claim/dispatch/retry/reclaim with no subsequent
// terminal event is an item that was in flight when the process died.
func terminalKind(kind string) bool {
	return kind == journalLand || kind == journalRelease
}

// journalRecord is the structured payload every loopengine journal line carries, encoded as
// JSON in the audit entry's Detail field (the only free field in the frozen Entry schema —
// this adds no new schema field and no new file). Replay parses it back; a loopengine audit
// line whose Detail does not parse as this shape (e.g. an older prose retry line) is simply
// skipped, never guessed at.
type journalRecord struct {
	// Loop scopes the event to a consumer's loop name so replay reads only its OWN history
	// out of the shared audit stream ("scoped to this consumer's loop name").
	Loop string `json:"loop"`
	// Item is the claim key the event is about.
	Item string `json:"item"`
	// RunID is the per-Run() lineage token (see the runid discussion above). It is what a
	// restart uses to tell a prior run of itself from a live sibling.
	RunID string `json:"runID"`
	// Tier is the dispatch destination, when known ("" for events raised before tiering,
	// e.g. a refuse).
	Tier string `json:"tier,omitempty"`
	// Outcome is the verdict class, set on a `land` event only.
	Outcome string `json:"outcome,omitempty"`
	// Note is optional human-readable context; it never carries load-bearing state.
	Note string `json:"note,omitempty"`
	// kind is the event verb, carried onto the record by parseJournalRecord from the audit
	// entry's Verb. It is not serialised (the verb lives in the entry, not the payload).
	kind string `json:"-"`
}

// newRunID mints a fresh per-Run() lineage token. It is random (not derived from any stable
// identity) on purpose: its ONLY job is to differ between two run instances, so that a
// restart's current runID never collides with the crashed run's journalled runID. crypto/rand
// so two near-simultaneous boots cannot mint the same token.
func newRunID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		// rand should not fail; a time-free fallback would defeat the differ-between-runs
		// property, so degrade to a fixed marker that recover.go treats conservatively
		// (a runID it cannot distinguish is never grounds to reclaim).
		return "run-unknown"
	}
	return "run-" + hex.EncodeToString(b[:])
}

// journalResultFor maps an event kind onto the audit `result` field. Every line must
// classify its outcome (deskkit.Log refuses an empty result). Scheduling acts are ok;
// a refusal is refused.
func journalResultFor(kind string) string {
	if kind == journalRefuse {
		return deskkit.ResultRefused
	}
	return deskkit.ResultOK
}

// journalEvent writes one scheduling-decision line to the shared audit stream through
// deskkit.Log. It is the SINGLE write path for the new event family — no scheduling code
// touches the audit file directly. A write failure is swallowed (best-effort, like retry.go's
// journal): the journal is recovery groundwork, never load-bearing for the drain in progress,
// and a drain must not wedge because the audit disk hiccuped. What IS load-bearing —
// recover.go's read — fails closed on its own side.
func (c Config) journalEvent(loopName, kind string, it Item, tier, outcome, note string) {
	c.journalEventResult(loopName, kind, it, tier, outcome, note, journalResultFor(kind))
}

// journalEventResult is journalEvent with an EXPLICIT audit result, for the one caller whose
// result is not derivable from the kind: a `land` that FAILED (landOne's file-and-continue
// path) is a land event with result=refused. Routing it here keeps journalEvent the SINGLE
// write path for the family — the land-failed line is a well-formed journalRecord that replay
// parses, not a raw prose Detail that parseJournalRecord silently skips (which would leave the
// item's newest parseable event a `dispatch` and misclassify a landed-but-unflipped item as
// in-flight).
func (c Config) journalEventResult(loopName, kind string, it Item, tier, outcome, note, result string) {
	rec := journalRecord{
		Loop:    loopName,
		Item:    it.ID,
		RunID:   c.runID,
		Tier:    tier,
		Outcome: outcome,
		Note:    note,
	}
	detail, err := json.Marshal(rec)
	if err != nil {
		return
	}
	_ = deskkit.Log(deskkit.Entry{
		Tool:   "loopengine",
		Verb:   kind,
		Result: result,
		Detail: string(detail),
	})
}

// parseJournalRecord decodes a loopengine journal line back into its structured record. It
// returns ok=false for any entry that is not a well-formed journal event — a non-loopengine
// line, or a loopengine line (e.g. an older prose retry entry) whose Detail is not this JSON
// shape or carries no item. Replay skips those rather than guessing.
func parseJournalRecord(e deskkit.Entry) (journalRecord, bool) {
	if e.Tool != "loopengine" || e.Detail == "" {
		return journalRecord{}, false
	}
	var rec journalRecord
	if err := json.Unmarshal([]byte(e.Detail), &rec); err != nil {
		return journalRecord{}, false
	}
	if rec.Item == "" || rec.Loop == "" {
		return journalRecord{}, false
	}
	// The verb IS the kind; carry it onto the record so callers read one value.
	rec.kind = e.Verb
	return rec, true
}
