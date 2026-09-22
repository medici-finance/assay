package deskkit

// repairobligation.go — the durable REPAIR OBLIGATION (example-stream/17).
//
// example-stream/16 records WHY a failed verification should not simply re-run (a WAKE receipt).
// This is its sibling for the OTHER half: an ACTIONABLE failed verification is durable worker work
// that must survive the reporting agent, be worked exactly once, and return to INDEPENDENT
// reverification after its repair merges. That durable unit is a REPAIR OBLIGATION.
//
// It is a versioned, structured MARKER — the same shape whether an agent-driven landing or the
// deterministic runner produced it — keyed by (repo, brief, source receipt, failing rows). The key
// is IMMUTABLE: it is derived from those inputs, so a restart, a duplicate delivery, or a lost
// acknowledgement all reconcile to the SAME obligation rather than manufacturing a second. The
// durable authority remains the target-repository issue/PR records plus the dispatch claim; this
// marker is the machine-readable projection over them (mirroring verify-outcomes.jsonl), never a
// second lifecycle database and never an authority grant.
//
// THE LOAD-BEARING INVARIANT (common-clause C4 applied to completion): a repair obligation is
// SCHEDULING state, not acceptance. Worker completion, issue closure and a merge ALONE can never
// resolve an implementation obligation — only a VALID INDEPENDENT verification result at the
// repaired revision does. A merge WAKES reverification; it does not close the obligation. The
// existing per-item claim and the reviewer/verifier identity gates remain the barriers; this marker
// sits beside them. See docs/streams/example-stream/repair-obligation-v1.md.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SchemaRepairV1 is the schema tag a complete repair obligation carries. A row without it is a
// legacy/incomplete marker — visibly unclassified, never a fabricated obligation.
const SchemaRepairV1 = "repair-obligation-v1"

// Obligation states. These are OBLIGATION states, not brief lifecycle cells: they describe where a
// repair stands, never what a board Status reads. The set is CLOSED.
const (
	// RepairNeedsAssignment: an actionable failed verification with no live worker — assignable now.
	RepairNeedsAssignment = "needs-assignment"
	// RepairRepairing: a worker holds the obligation's lease and is producing the fix.
	RepairRepairing = "repairing"
	// RepairAwaitingReview: a repair PR is open; the reviewer decides correctness.
	RepairAwaitingReview = "awaiting-review"
	// RepairAwaitingMerge: the repair PR is approved and awaits a human merge.
	RepairAwaitingMerge = "awaiting-merge"
	// RepairAwaitingReverification: the repair MERGED — a legitimate wake event — so independent
	// reverification is now owed. A merge lands HERE, never on resolved.
	RepairAwaitingReverification = "awaiting-reverification"
	// RepairWaitingExternal: a human/environment blocker holds it; it carries the exact required
	// action and is NOT dispatched to a worker.
	RepairWaitingExternal = "waiting-external"
	// RepairResolved: a valid INDEPENDENT verification at the repaired revision cleared it. Terminal.
	RepairResolved = "resolved"
)

var repairStates = map[string]bool{
	RepairNeedsAssignment: true, RepairRepairing: true, RepairAwaitingReview: true,
	RepairAwaitingMerge: true, RepairAwaitingReverification: true,
	RepairWaitingExternal: true, RepairResolved: true,
}

// repairStateRank orders states from least to most progressed, so reconciliation of a duplicated
// or partially-written obligation folds to the furthest-advanced record rather than regressing it.
var repairStateRank = map[string]int{
	RepairNeedsAssignment:        0,
	RepairWaitingExternal:        1,
	RepairRepairing:              2,
	RepairAwaitingReview:         3,
	RepairAwaitingMerge:          4,
	RepairAwaitingReverification: 5,
	RepairResolved:               6,
}

// RepairObligation is one versioned repair marker. Its first two fields (TS/Brief) match the
// append-only sidecar row shape so it round-trips through the same JSONL the projection stores and
// an older reader that only knows those keeps working.
type RepairObligation struct {
	// --- sidecar row header (stable shape) ---
	TS    string `json:"ts"`
	Brief string `json:"brief"` // <stream>/<NN>

	// --- repair-obligation-v1 additive fields ---
	Schema string `json:"repair_schema,omitempty"` // SchemaRepairV1 on a complete obligation
	ID     string `json:"obligation_id,omitempty"` // stable, IMMUTABLE dedupe key

	// The failed verification this obligation repairs.
	Repo      string `json:"repo,omitempty"`       // DELIVERABLE repo (owner/repo) — where the fix lands
	ReceiptID string `json:"receipt_id,omitempty"` // source example-stream/16 wake receipt ID
	Rows      []int  `json:"rows,omitempty"`       // the failing Verify-row numbers

	BlockerKind     string `json:"blocker_kind,omitempty"`     // one of the deskkit Blocker* constants
	ResponsibleRole string `json:"responsible_role,omitempty"` // next actor (worker / brief-author / human / operator / verifier)
	State           string `json:"state,omitempty"`

	// Human/worker task payload — the reproduction and the expected behaviour, so a replacement
	// worker starts from the failed verification, not from a cold read of the brief.
	Reproduction string `json:"reproduction,omitempty"`
	Expected     string `json:"expected,omitempty"`
	// RequiredAction is the exact next act for a waiting-external obligation (never a worker task).
	RequiredAction string `json:"required_action,omitempty"`

	// Current attempt / lease. The lease makes a dead claim assignable again WITHOUT a new
	// obligation: an expired lease returns the SAME ID to needs-assignment.
	Attempt int    `json:"attempt,omitempty"`
	ClaimID string `json:"claim_id,omitempty"` // the current worker's claim/session id
	LeaseTS string `json:"lease_ts,omitempty"` // RFC3339 — when the current lease was taken

	// The repair itself.
	RepairPR  string `json:"repair_pr,omitempty"`  // linked repair PR (owner/repo#N)
	RepairSHA string `json:"repair_sha,omitempty"` // the revision the repair produced/merged at
	RepairedBy string `json:"repaired_by,omitempty"` // the worker identity that produced the repair

	// The original (failed) deliverable. When the original PR is MERGED, repair needs a NEW
	// branch: a merged PR is immutable history, so the worker must not resume it.
	OriginalPR     string `json:"original_pr,omitempty"`
	OriginalMerged bool   `json:"original_merged,omitempty"`

	// Resolution provenance — set ONLY by a valid independent reverification.
	ResolvedByVerifier string `json:"resolved_by_verifier,omitempty"`
	ResolvedAtSHA      string `json:"resolved_at_sha,omitempty"`
}

// RepairObligationID is the stable, IMMUTABLE key for the failed verification (repo, brief, source
// receipt, failing rows). Identical inputs yield an identical key, so a duplicate delivery, a lost
// acknowledgement, or a process restart all address the SAME obligation. Rows are sorted first so
// row ORDER never changes the key.
func RepairObligationID(repo, brief, receiptID string, rows []int) string {
	sorted := append([]int(nil), rows...)
	sort.Ints(sorted)
	h := sha256.Sum256([]byte(strings.Join([]string{
		strings.TrimSpace(repo), strings.TrimSpace(brief), strings.TrimSpace(receiptID), joinRepairInts(sorted),
	}, "\x00")))
	return "repair/" + hex.EncodeToString(h[:])[:16]
}

// NewRepairObligation builds a complete obligation from a failed verification. The blocker kind
// decides routing: implementation and check-definition are ACTIONABLE (a worker repairs the code
// or amends the check under the existing review policy); human-action and environment stay
// waiting-external with their exact required action; unknown is explicit triage (responsible:
// verifier), NEVER manufactured as an implementation bug. The obligation is deterministic — the
// same failed verification always yields the same ID and starting state.
func NewRepairObligation(repo, brief, receiptID string, rows []int, blockerKind, reproduction, expected, requiredAction, ts string) RepairObligation {
	o := RepairObligation{
		TS:           ts,
		Brief:        strings.TrimSpace(brief),
		Schema:       SchemaRepairV1,
		ID:           RepairObligationID(repo, brief, receiptID, rows),
		Repo:         strings.TrimSpace(repo),
		ReceiptID:    strings.TrimSpace(receiptID),
		Rows:         rows,
		BlockerKind:  blockerKind,
		Reproduction: reproduction,
		Expected:     expected,
	}
	switch blockerKind {
	case BlockerImplementation, BlockerCheckDef:
		o.State = RepairNeedsAssignment
		o.ResponsibleRole = repairActorFor(blockerKind)
	case BlockerHumanAction, BlockerEnvironment:
		o.State = RepairWaitingExternal
		o.ResponsibleRole = repairActorFor(blockerKind)
		o.RequiredAction = strings.TrimSpace(requiredAction)
	default: // unknown / unrecognised — explicit triage, never an invented implementation bug
		o.BlockerKind = BlockerUnknown
		o.State = RepairNeedsAssignment
		o.ResponsibleRole = "verifier"
	}
	return o
}

// repairActorFor names who owns the next action for a blocker kind. It mirrors WakeReceipt.NextActor
// but is scoped to repair routing (check-definition routes to a worker to amend the check).
func repairActorFor(blockerKind string) string {
	switch blockerKind {
	case BlockerImplementation:
		return "worker"
	case BlockerCheckDef:
		return "worker" // amend the check under the existing review policy
	case BlockerHumanAction:
		return "human"
	case BlockerEnvironment:
		return "operator"
	default:
		return "verifier"
	}
}

// Complete reports whether the marker carries a well-formed repair-obligation-v1 payload: the
// schema tag, a stable ID, a known state and a known blocker kind. A legacy/incomplete marker is
// deliberately NOT complete — it stays visibly unclassified, never a fabricated obligation.
func (o RepairObligation) Complete() bool {
	return o.Schema == SchemaRepairV1 &&
		strings.TrimSpace(o.ID) != "" &&
		repairStates[o.State] &&
		(blockerKinds[o.BlockerKind])
}

// IsActionable reports whether this obligation is worker-repairable (implementation or
// check-definition). Human/environment/unknown obligations are visible but NOT worker work.
func (o RepairObligation) IsActionable() bool {
	return o.BlockerKind == BlockerImplementation || o.BlockerKind == BlockerCheckDef
}

// IsResolved reports whether the obligation has been cleared by a valid independent reverification.
func (o RepairObligation) IsResolved() bool { return o.State == RepairResolved }

// LeaseExpired reports whether the current worker lease is older than ttl as of now. A malformed or
// absent lease timestamp is treated as EXPIRED (could-not-read a live lease → assignable again,
// never a claim held forever by an unparseable stamp). ttl <= 0 disables expiry.
func (o RepairObligation) LeaseExpired(now time.Time, ttl time.Duration) bool {
	if ttl <= 0 {
		return false
	}
	if strings.TrimSpace(o.LeaseTS) == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(o.LeaseTS))
	if err != nil {
		return true
	}
	return now.Sub(t) >= ttl
}

// Assignable reports whether a worker may pick this obligation up THIS pass: it must be actionable,
// unresolved, and either awaiting assignment or held under a DEAD (expired) lease. A live
// repairing claim is NOT assignable — that is the mutual-exclusion the immutable key preserves; an
// expired lease returns the SAME obligation to the queue without duplicating it.
func (o RepairObligation) Assignable(now time.Time, ttl time.Duration) bool {
	if !o.IsActionable() || o.IsResolved() {
		return false
	}
	switch o.State {
	case RepairNeedsAssignment:
		return true
	case RepairRepairing:
		return o.LeaseExpired(now, ttl)
	default:
		return false
	}
}

// Claim moves the obligation to repairing under claimID, stamps a fresh lease, and increments the
// attempt counter — value semantics, so the caller persists the returned obligation. A replacement
// worker (after a dead lease) claims the SAME ID; only Attempt and the lease change.
func (o RepairObligation) Claim(claimID string, now time.Time) RepairObligation {
	o.State = RepairRepairing
	o.ClaimID = strings.TrimSpace(claimID)
	o.LeaseTS = now.UTC().Format(time.RFC3339)
	o.Attempt++
	return o
}

// WithRepairPR records the linked repair PR and its head revision, produced by repairedBy, and moves
// the obligation to awaiting-review. The reviewer decides correctness; this only records the link.
func (o RepairObligation) WithRepairPR(repairPR, repairSHA, repairedBy string) RepairObligation {
	o.RepairPR = strings.TrimSpace(repairPR)
	o.RepairSHA = strings.TrimSpace(repairSHA)
	o.RepairedBy = strings.TrimSpace(repairedBy)
	o.State = RepairAwaitingReview
	return o
}

// MarkAwaitingMerge records that the repair PR is approved and awaits a human merge.
func (o RepairObligation) MarkAwaitingMerge() RepairObligation {
	o.State = RepairAwaitingMerge
	return o
}

// MarkRepairMerged records the repair MERGE at mergedSHA — a legitimate wake event — and moves the
// obligation to awaiting-reverification. It deliberately does NOT resolve: a merge alone never
// closes an implementation obligation. It only makes an INDEPENDENT reverification owed.
func (o RepairObligation) MarkRepairMerged(mergedSHA string) RepairObligation {
	if strings.TrimSpace(mergedSHA) != "" {
		o.RepairSHA = strings.TrimSpace(mergedSHA)
	}
	o.State = RepairAwaitingReverification
	return o
}

// Resolve is the RESOLVE GUARD — the one door to RepairResolved for an implementation/check
// obligation. It resolves ONLY when all three hold, and reports which one failed otherwise:
//
//   - a legitimate wake event exists: the state is awaiting-reverification (the repair merged);
//   - the verification is INDEPENDENT: verifier is non-empty and is NOT the worker that produced
//     the repair (a same-actor pass cannot resolve — that is self-certification);
//   - it is at the REPAIRED revision: atSHA equals the merged repair revision (a stale/wrong-
//     revision pass cannot resolve — it did not observe the fix).
//
// It returns the (possibly updated) obligation, whether it resolved, and a human-readable reason.
func (o RepairObligation) Resolve(verifier, atSHA string) (RepairObligation, bool, string) {
	verifier = strings.TrimSpace(verifier)
	atSHA = strings.TrimSpace(atSHA)
	if o.State != RepairAwaitingReverification {
		return o, false, "not awaiting reverification (state " + o.State + "): a merge or a worker assertion is not a legitimate wake event — the blocked obligation is preserved"
	}
	if verifier == "" {
		return o, false, "no verifier identity: an anonymous pass cannot resolve an obligation"
	}
	if o.RepairedBy != "" && strings.EqualFold(verifier, o.RepairedBy) {
		return o, false, "same-actor pass: the verifier " + verifier + " produced the repair — independent reverification is required, worker self-certification cannot resolve"
	}
	if strings.TrimSpace(o.RepairSHA) == "" || atSHA != strings.TrimSpace(o.RepairSHA) {
		return o, false, "wrong-revision pass: verified at " + atSHA + " but the repair is at " + o.RepairSHA + " — a pass that did not observe the fix cannot resolve"
	}
	o.State = RepairResolved
	o.ResolvedByVerifier = verifier
	o.ResolvedAtSHA = atSHA
	return o, true, "resolved by independent verification " + verifier + " at repaired revision " + atSHA
}

// RequiresFollowUpBranch reports whether repair must open a NEW branch/PR rather than resume the
// original: true exactly when the original deliverable PR has MERGED. A merged PR is immutable
// history; resuming it is impossible, so the worker branches fresh.
func (o RepairObligation) RequiresFollowUpBranch() bool { return o.OriginalMerged }

// FollowUpBranch is the NEW branch name a merged-original repair takes. It embeds the obligation's
// brief and attempt so a second dead-lease reassignment does not collide with the first, and it
// never names the merged original branch. Callers use it only when RequiresFollowUpBranch is true.
func (o RepairObligation) FollowUpBranch() string {
	slug := strings.NewReplacer("/", "-", ":", "-", " ", "-").Replace(strings.TrimSpace(o.Brief))
	attempt := o.Attempt
	if attempt < 1 {
		attempt = 1
	}
	return "repair/" + slug + "-followup-" + strconv.Itoa(attempt)
}

// TargetRepo names the DELIVERABLE repository the repair lands in — the repo of the failed
// verification, resolved through the existing dispatcher resolver, never wherever the tracking
// board lives. Empty when the obligation carried no repo.
func (o RepairObligation) TargetRepo() string { return strings.TrimSpace(o.Repo) }

// ParseRepairObligation unmarshals one JSONL sidecar line into an obligation. A malformed line, or
// one with no brief key, is not an obligation (ok=false) — the caller skips it exactly as the
// legacy reader skips a bad line.
func ParseRepairObligation(line []byte) (RepairObligation, bool) {
	var o RepairObligation
	if json.Unmarshal(line, &o) != nil || strings.TrimSpace(o.Brief) == "" {
		return RepairObligation{}, false
	}
	return o, true
}

// MarshalLine renders the obligation as ONE JSONL line (no trailing newline) for the append-only
// projection. The header keys (ts/brief) stay first via struct field order, so an older reader
// keeps working.
func (o RepairObligation) MarshalLine() ([]byte, error) { return json.Marshal(o) }

// ReconcileObligations folds an append-only obligation log into the CURRENT obligation per key.
// The log is append-only, so one obligation ID may appear many times: a duplicate delivery, a lost
// acknowledgement re-posted, a claim then a repair then a merge. Reconciliation collapses each ID
// to ONE record — the FURTHEST-progressed state, with the latest lease/attempt and any non-empty
// field filled in — so an idempotent re-post never creates a second obligation and a partial write
// is repaired by reading, never by blindly appending again. The result is sorted by ID for
// determinism.
func ReconcileObligations(log []RepairObligation) []RepairObligation {
	byID := map[string]RepairObligation{}
	for _, o := range log {
		key := strings.TrimSpace(o.ID)
		if key == "" {
			continue
		}
		cur, seen := byID[key]
		if !seen {
			byID[key] = o
			continue
		}
		byID[key] = mergeObligation(cur, o)
	}
	out := make([]RepairObligation, 0, len(byID))
	for _, o := range byID {
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// mergeObligation folds b into a, keeping the furthest-progressed state and the most-current
// scheduling fields. It is the per-ID reducer ReconcileObligations applies: state advances (never
// regresses), the higher attempt and its lease win, and empty fields are filled from either side.
func mergeObligation(a, b RepairObligation) RepairObligation {
	out := a
	// State never regresses — take the higher-ranked of the two.
	if repairStateRank[b.State] > repairStateRank[out.State] {
		out.State = b.State
		out.ResponsibleRole = firstNonEmpty(b.ResponsibleRole, out.ResponsibleRole)
	}
	// The higher attempt (and its lease/claim) is the live one — a replacement worker's later
	// claim supersedes the dead one.
	if b.Attempt > out.Attempt {
		out.Attempt = b.Attempt
		out.ClaimID = b.ClaimID
		out.LeaseTS = b.LeaseTS
	} else if b.Attempt == out.Attempt {
		out.LeaseTS = laterStamp(out.LeaseTS, b.LeaseTS)
		out.ClaimID = firstNonEmpty(out.ClaimID, b.ClaimID)
	}
	out.TS = laterStamp(out.TS, b.TS)
	out.RepairPR = firstNonEmpty(out.RepairPR, b.RepairPR)
	out.RepairSHA = firstNonEmpty(out.RepairSHA, b.RepairSHA)
	out.RepairedBy = firstNonEmpty(out.RepairedBy, b.RepairedBy)
	out.OriginalPR = firstNonEmpty(out.OriginalPR, b.OriginalPR)
	out.OriginalMerged = out.OriginalMerged || b.OriginalMerged
	out.Reproduction = firstNonEmpty(out.Reproduction, b.Reproduction)
	out.Expected = firstNonEmpty(out.Expected, b.Expected)
	out.RequiredAction = firstNonEmpty(out.RequiredAction, b.RequiredAction)
	out.ReceiptID = firstNonEmpty(out.ReceiptID, b.ReceiptID)
	out.Repo = firstNonEmpty(out.Repo, b.Repo)
	if len(out.Rows) == 0 {
		out.Rows = b.Rows
	}
	// A resolution provenance, once present, is kept.
	out.ResolvedByVerifier = firstNonEmpty(out.ResolvedByVerifier, b.ResolvedByVerifier)
	out.ResolvedAtSHA = firstNonEmpty(out.ResolvedAtSHA, b.ResolvedAtSHA)
	return out
}

// laterStamp returns the later of two RFC3339 stamps; an unparseable stamp loses to a parseable
// one, and two unparseable stamps keep the first non-empty.
func laterStamp(a, b string) string {
	ta, ea := time.Parse(time.RFC3339, strings.TrimSpace(a))
	tb, eb := time.Parse(time.RFC3339, strings.TrimSpace(b))
	switch {
	case ea != nil && eb != nil:
		return firstNonEmpty(a, b)
	case ea != nil:
		return b
	case eb != nil:
		return a
	case tb.After(ta):
		return b
	default:
		return a
	}
}

func joinRepairInts(xs []int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = strconv.Itoa(x)
	}
	return strings.Join(parts, ",")
}
