package main

// repair.go — the REPAIR-OBLIGATION rework source for worker-desk (example-stream/17).
//
// example-stream/16 recorded WHY a failed verification should not simply re-run (a wake receipt).
// This is its sibling for the OTHER half: an ACTIONABLE failed verification is durable worker work
// that must survive the reporting agent and return to INDEPENDENT reverification after its repair
// merges. deskkit.RepairObligation is that durable unit; this file makes it a live worker-desk
// source of work rather than a record an operator reads by hand.
//
// The obligation projection is an append-only JSONL sidecar — docs/streams/repair-obligations.jsonl
// — a sibling of verify-outcomes.jsonl (example-stream/16's wake sidecar), kept in a SEPARATE file
// so a repair line never parses as an incomplete wake receipt and vice versa. The log is
// append-only, so one immutable obligation ID may appear many times (a duplicate delivery, a lost
// acknowledgement re-posted, a claim then a repair then a merge); deskkit.ReconcileObligations
// folds each ID to the furthest-progressed record BEFORE anything acts on it — the read side is the
// partial-write recovery the interface contract requires.
//
// This is the SAME reference/interim-build posture as the rest of fanoutloop: the source is
// injectable (RepairObligations), the default reads the local trees across configured roots, and
// nothing here contacts a network. It EXTENDS the existing rework lane (kind=kindRework) rather than
// standing up a competing board, so a repair obligation inherits the rework class's reservation
// floor and orphan-behind priority already wired in SelectQueue.

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// repairSidecarName is the append-only repair-obligation projection under docs/streams/ — a sibling
// of example-stream/16's verify-outcomes.jsonl, kept separate so the two projections never collide
// on a shared parser.
const repairSidecarName = "repair-obligations.jsonl"

// repairLeaseTTL is how long a worker's obligation lease is honoured before a dead claim makes the
// SAME obligation assignable again (task 2: lease expiry re-queues without duplicating). It mirrors
// the dispatch-claim lease horizon: long enough that a live worker is never stolen from, short
// enough that a silently-dead session does not park the obligation forever.
const repairLeaseTTL = 45 * time.Minute

// repairPayloadKeys — the typed payload a repair rework item carries, so a dispatched worker starts
// from the failed verification (reproduction + expected) and can never resume a merged branch.
const (
	payloadObligationID   = "obligation_id"
	payloadReceiptID      = "receipt_id"
	payloadReproduction   = "reproduction"
	payloadExpected       = "expected"
	payloadRows           = "rows"
	payloadOriginalPR     = "original_pr"
	payloadOriginalMerged = "original_merged"
	payloadRepairPR       = "repair_pr"
	payloadFollowUpBranch = "follow_up_branch"
	payloadAttempt        = "attempt"
	payloadBrief          = "brief"
	payloadBlockerKind    = "blocker_kind"
)

// readRepairLog reads ONE append-only repair-obligation sidecar into its raw (un-reconciled) rows.
// A malformed or non-obligation line is skipped exactly as the wake reader skips a bad line
// (deskkit.ParseRepairObligation reports ok=false). A missing sidecar is not an error — a root that
// has never landed a failed verification simply has no obligations — so os.IsNotExist yields an
// empty slice, never a failure that would wedge the whole cross-root read.
func readRepairLog(path string) ([]deskkit.RepairObligation, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []deskkit.RepairObligation
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if o, ok := deskkit.ParseRepairObligation([]byte(line)); ok {
			out = append(out, o)
		}
	}
	return out, nil
}

// collectRepairObligations reads every root's repair sidecar, folds the whole append-only log across
// ALL roots through deskkit.ReconcileObligations, and returns the CURRENT obligation per immutable
// key, sorted by ID for determinism. Reconciling ACROSS roots (not per-root) is what makes a
// cross-repo obligation — one whose deliverable is a sibling repo but whose projection may have been
// re-posted from more than one tracking tree — fold to a single record rather than one per tree.
//
// A root whose sidecar cannot be read (a genuine I/O error, not a missing file) is returned as the
// error: a could-not-read root is could-not-check, never silently dropped, exactly as the
// multi-repo board refuses to skip a configured root.
func collectRepairObligations(roots []deskkit.RootConfig) ([]deskkit.RepairObligation, error) {
	var log []deskkit.RepairObligation
	for _, r := range roots {
		rows, err := readRepairLog(filepath.Join(r.Path, "docs", "streams", repairSidecarName))
		if err != nil {
			return nil, err
		}
		log = append(log, rows...)
	}
	return deskkit.ReconcileObligations(log), nil
}

// assignableRepairItems is the rework-lane projection of the reconciled obligations: it keeps only
// the obligations a worker may pick up THIS pass — actionable (implementation / check-definition),
// unresolved, and either awaiting assignment or held under a DEAD lease (deskkit's Assignable rule)
// — and maps each to a rework loopengine.Item. A live repairing claim, a resolved obligation, and a
// waiting-external (human/environment) obligation are all correctly withheld from dispatch: they are
// visible in the projection but are not worker work. The order follows the reconciled ID order.
func assignableRepairItems(obligs []deskkit.RepairObligation, roots []deskkit.RootConfig, targetSHA string, now time.Time, ttl time.Duration) []loopengine.Item {
	var items []loopengine.Item
	for _, o := range obligs {
		if !o.Assignable(now, ttl) {
			continue
		}
		items = append(items, obligationToReworkItem(o, roots, targetSHA))
	}
	return items
}

// obligationToReworkItem maps one assignable obligation to a rework loopengine.Item. The item's ID
// is the IMMUTABLE obligation key (not a stream/num board id): that is what lets a replacement
// worker, after a dead lease, claim the SAME obligation instead of manufacturing a second (task 2).
// The payload carries the reproduction + expected behaviour so the worker starts from the failed
// verification, the deliverable repo resolved from the obligation (task 3), and — when the original
// deliverable PR has MERGED — the follow-up branch it must open instead of resuming immutable
// history (task 3 / Verify row 4).
func obligationToReworkItem(o deskkit.RepairObligation, roots []deskkit.RootConfig, targetSHA string) loopengine.Item {
	it := loopengine.Item{
		ID:        o.ID,
		TargetSHA: targetSHA,
		Payload: map[string]string{
			"kind":                kindRework,
			"repo":                o.TargetRepo(),
			payloadObligationID:   o.ID,
			payloadReceiptID:      o.ReceiptID,
			payloadReproduction:   o.Reproduction,
			payloadExpected:       o.Expected,
			payloadRows:           joinInts(o.Rows),
			payloadOriginalPR:     o.OriginalPR,
			payloadOriginalMerged: boolStr(o.OriginalMerged),
			payloadRepairPR:       o.RepairPR,
			payloadAttempt:        strconv.Itoa(o.Attempt),
			payloadBrief:          o.Brief,
			payloadBlockerKind:    o.BlockerKind,
		},
	}
	// A merged original is immutable history: the worker must branch fresh, never resume it. The
	// follow-up branch name embeds the brief + attempt so a second dead-lease reassignment does not
	// collide with the first. It is emitted ONLY when the original merged, so an ordinary in-flight
	// repair carries no follow-up-branch instruction that would confuse it.
	if o.RequiresFollowUpBranch() {
		it.Payload[payloadFollowUpBranch] = o.FollowUpBranch()
	}
	// Resolve the brief's own frontmatter (tier / gate / risk / write-scopes) from whichever
	// configured root actually carries the brief file, so a repair dispatch tiers exactly as the
	// original brief did. A brief that cannot be resolved degrades to the economy default rather
	// than dropping the obligation — the obligation, not the board row, is the authority here.
	if stream, num, ok := splitBriefID(o.Brief); ok {
		for _, r := range roots {
			path, effort, execTier, gate, risk, implementer, _, scopes := resolveBrief(r.Path, stream, num)
			if path != "" {
				it.BriefPath = path
				it.Effort, it.ExecTier, it.Gate, it.Risk, it.Implementer, it.WriteScopes =
					effort, execTier, gate, risk, implementer, scopes
				break
			}
		}
	}
	return it
}

// ---- forge-transition reconciliation (task 4) ---------------------------------------------------

// RepairForgeObservation is what an ALREADY-AUTHORIZED forge read reports about an obligation's
// repair PR. It is the ONLY input applyForgeTransition consumes: the interim/offline build makes no
// probe of its own (exactly like the orphan and represented sources), so tests inject it and the
// live cutover supplies it from the forge resolver.
type RepairForgeObservation struct {
	ReviewOpened bool   // a repair PR is open and under review
	Approved     bool   // the repair PR is approved and awaits a human merge
	Merged       bool   // the repair PR MERGED — a legitimate wake event
	MergedSHA    string // the revision the merge landed at (the repaired revision)
	RepairPR     string // owner/repo#N of the repair PR, recorded on first observation
	RepairedBy   string // the worker identity that produced the repair
}

// applyForgeTransition reconciles ONE obligation against a forge observation and returns the
// (possibly advanced) obligation plus whether it changed. It NEVER resolves an obligation: a merge
// WAKES independent reverification (awaiting-reverification), it does not close the obligation —
// only deskkit.RepairObligation.Resolve, driven by an independent verifier at the repaired revision,
// can do that. State only ADVANCES here (merge outranks approve outranks review-open), so a stale or
// out-of-order observation can never regress a further-progressed obligation. This is the whole
// load-bearing invariant restated at the transition boundary: a worker assertion or a merge is a
// scheduling event, never acceptance.
func applyForgeTransition(o deskkit.RepairObligation, obs RepairForgeObservation) (deskkit.RepairObligation, bool) {
	before := o.State
	switch {
	case obs.Merged:
		if o.RepairPR == "" && strings.TrimSpace(obs.RepairPR) != "" {
			o = o.WithRepairPR(obs.RepairPR, obs.MergedSHA, obs.RepairedBy)
		}
		if o.RepairedBy == "" && strings.TrimSpace(obs.RepairedBy) != "" {
			o.RepairedBy = strings.TrimSpace(obs.RepairedBy)
		}
		o = o.MarkRepairMerged(obs.MergedSHA)
	case obs.Approved:
		if o.State == deskkit.RepairAwaitingReview || o.State == deskkit.RepairRepairing {
			o = o.MarkAwaitingMerge()
		}
	case obs.ReviewOpened:
		if o.State == deskkit.RepairRepairing {
			o = o.WithRepairPR(obs.RepairPR, obs.MergedSHA, obs.RepairedBy)
		}
	}
	return o, o.State != before
}

// ---- small shared helpers -----------------------------------------------------------------------

// splitBriefID splits a "<stream>/<NN>" brief id into its parts. ok=false for a malformed id, so a
// caller degrades to the economy default rather than resolving a nonsense path.
func splitBriefID(brief string) (stream, num string, ok bool) {
	i := strings.LastIndex(strings.TrimSpace(brief), "/")
	if i <= 0 || i == len(brief)-1 {
		return "", "", false
	}
	return brief[:i], brief[i+1:], true
}

// joinInts renders row numbers as a comma-joined string for the item payload (sorted, so the
// payload is stable regardless of the order the rows were recorded in).
func joinInts(xs []int) string {
	sorted := append([]int(nil), xs...)
	sort.Ints(sorted)
	parts := make([]string, len(sorted))
	for i, x := range sorted {
		parts[i] = strconv.Itoa(x)
	}
	return strings.Join(parts, ",")
}
