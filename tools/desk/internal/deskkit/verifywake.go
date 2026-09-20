package deskkit

// verifywake.go — the verification WAKE receipt (desk-supervision/16).
//
// A verifier run that ends in a failed or blocked outcome records WHY the next expensive run
// should not simply repeat: what it observed, what inputs it observed it against, what class of
// blocker it hit, and — the load-bearing part — the CHECKABLE condition that must change before
// re-running is worth a slot. That record is a WAKE RECEIPT. It is scheduling evidence ONLY: it
// can never grant verified/done, satisfy acceptance, or authorize a write. The existing per-item
// claim and the reviewer/verifier identity gates remain the barriers; a receipt sits beside them.
//
// The receipt is an OPTIONAL, VERSIONED extension of the existing append-only verify-outcomes
// sidecar row — not a second lifecycle database. A legacy row (no schema, no wake fields) parses
// as an INCOMPLETE receipt and stays visibly unclassified; the additive fields never invalidate
// an older reader, which simply ignores them. See docs/streams/desk-supervision/verify-wake-v1.md.
//
// The three-state instrument rule (common-clause C4) is the spine of the evaluator: an input the
// reader could not read is reported AS could-not-check, never rounded up to "unchanged" and never
// down to a failure it did not observe. An incomplete input scope cannot establish unchangedness,
// so it too refuses to claim a hold.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SchemaWakeV1 is the schema tag a complete wake receipt carries. A row without it is legacy.
const SchemaWakeV1 = "verify-wake-v1"

// Blocker kinds — the class of thing standing between the failed verification and completion.
// The set is CLOSED: a value outside it makes the receipt incomplete (unclassified), never a
// silent hold. Each maps to a next actor (WakeReceipt.NextActor).
const (
	BlockerImplementation = "implementation"   // a code/deliverable defect a worker must repair
	BlockerCheckDef       = "check-definition" // the Verify definition itself is wrong/missing
	BlockerHumanAction    = "human-action"     // a human decision / merge / grant is required
	BlockerEnvironment    = "environment"      // an environment / external prerequisite is absent
	BlockerUnknown        = "unknown"          // not yet classified — eligible for one classification pass
)

// Wake predicates — the CHECKABLE condition that must hold before a re-run is worth a slot.
// The set is CLOSED for the same reason.
const (
	WakeRelevantInputChanged = "relevant-input-changed"       // a declared input's revision differs
	WakeReferencedActionDone = "referenced-action-completed"  // the referenced external action completed
	WakeDeadlineReached      = "declared-deadline-reached"    // a declared calendar deadline is reached
	WakeExplicitRecheck      = "explicit-recheck-with-reason" // an operator asked for a recheck, with a stated reason
)

var blockerKinds = map[string]bool{
	BlockerImplementation: true, BlockerCheckDef: true, BlockerHumanAction: true,
	BlockerEnvironment: true, BlockerUnknown: true,
}

var wakePredicates = map[string]bool{
	WakeRelevantInputChanged: true, WakeReferencedActionDone: true,
	WakeDeadlineReached: true, WakeExplicitRecheck: true,
}

// WakeReceipt is one versioned scheduling receipt. Its first four fields are exactly the legacy
// verify-outcomes row (TS/Brief/Outcome/SHA), so a receipt round-trips through the same JSONL
// line the sidecar already stores and an older reader that only knows those four keeps working.
type WakeReceipt struct {
	// --- legacy verify-outcomes row (unchanged shape) ---
	TS      string `json:"ts"`
	Brief   string `json:"brief"`
	Outcome string `json:"outcome"` // observed outcome: verify-fail | blocked | verified | …
	SHA     string `json:"sha"`

	// --- verify-wake-v1 additive fields ---
	Schema   string `json:"wake_schema,omitempty"` // SchemaWakeV1 on a complete receipt
	ID       string `json:"receipt_id,omitempty"`  // stable receipt ID (dedupe key)
	Repo     string `json:"repo,omitempty"`        // owner/repo the brief lives in
	Verifier string `json:"verifier,omitempty"`    // the TRUSTED WRITER identity, never free text
	Rows     []int  `json:"rows,omitempty"`        // the Verify-row numbers this receipt holds

	// Inputs is the declared input scope: input key -> the revision observed at receipt time.
	// Keys are opaque to the evaluator; the reader interprets them. An empty scope for a
	// relevant-input-changed predicate cannot establish unchangedness (incomplete).
	Inputs      map[string]string `json:"inputs,omitempty"`
	ToolVersion string            `json:"tool_version,omitempty"` // applicable tool version at receipt time

	BlockerKind   string `json:"blocker_kind,omitempty"`
	BlockerRef    string `json:"blocker_ref,omitempty"`    // issue/PR/action ref the blocker points at
	WakePredicate string `json:"wake_predicate,omitempty"` // one of the WakeXxx constants
	Deadline      string `json:"deadline,omitempty"`       // RFC3339 / date, for declared-deadline-reached
	RecheckReason string `json:"recheck_reason,omitempty"` // required for explicit-recheck-with-reason
}

// ParseWakeReceipt unmarshals one JSONL sidecar line into a receipt. A malformed line, or one
// with no brief key, is not a receipt (ok=false) — the caller skips it exactly as the legacy
// reader skips a bad line.
func ParseWakeReceipt(line []byte) (WakeReceipt, bool) {
	var r WakeReceipt
	if json.Unmarshal(line, &r) != nil || strings.TrimSpace(r.Brief) == "" {
		return WakeReceipt{}, false
	}
	return r, true
}

// IsFailedOrBlocked reports whether the receipt's observed outcome is one this brief's wake
// scheduling governs. A verified outcome is the stuck-flip lane's concern, not this one.
func (r WakeReceipt) IsFailedOrBlocked() bool {
	switch strings.ToLower(strings.TrimSpace(r.Outcome)) {
	case "verify-fail", "fail", "blocked", "needs_context", "needs-context":
		return true
	default:
		return false
	}
}

// Complete reports whether the receipt carries a full, well-formed verify-wake-v1 payload: the
// schema tag, a stable ID, a KNOWN blocker kind and a KNOWN wake predicate, plus the predicate's
// own required field. An incomplete or legacy receipt is deliberately NOT complete — it stays
// visibly unclassified and eligible for one classification pass, never a fabricated hold.
func (r WakeReceipt) Complete() bool {
	if r.Schema != SchemaWakeV1 || strings.TrimSpace(r.ID) == "" {
		return false
	}
	if !blockerKinds[r.BlockerKind] || !wakePredicates[r.WakePredicate] {
		return false
	}
	switch r.WakePredicate {
	case WakeRelevantInputChanged:
		return len(r.Inputs) > 0 // an empty scope cannot establish unchangedness
	case WakeReferencedActionDone:
		return strings.TrimSpace(r.BlockerRef) != ""
	case WakeDeadlineReached:
		return strings.TrimSpace(r.Deadline) != ""
	case WakeExplicitRecheck:
		return strings.TrimSpace(r.RecheckReason) != ""
	}
	return false
}

// NextActor names who owns the next action for this receipt's blocker kind. It is a scheduling
// hint surfaced in the WAIT line, never an authority grant.
func (r WakeReceipt) NextActor() string {
	switch r.BlockerKind {
	case BlockerImplementation:
		return "worker"
	case BlockerCheckDef:
		return "brief-author"
	case BlockerHumanAction:
		return "human"
	case BlockerEnvironment:
		return "operator"
	default:
		return "verifier"
	}
}

// WakeState is the evaluator's verdict for one receipt against current inputs.
type WakeState int

const (
	// WakeHold: a valid, complete receipt whose wake condition has NOT been met. The failure
	// stays visible as a WAIT row; re-verifying now only reproduces the same non-verdict, so it
	// is excluded from costly dispatch.
	WakeHold WakeState = iota
	// WakeFire: the wake condition HAS been met (an input changed, the action completed, the
	// deadline passed, or an explicit recheck was requested) — the affected work is dispatchable.
	WakeFire
	// WakeCouldNotCheck: a declared input could not be read, so unchangedness cannot be
	// established. Surfaced as could-not-check — never an empty queue, never a pass.
	WakeCouldNotCheck
	// WakeUnclassified: a legacy or incomplete receipt. It is not a hold and not a pass; it is
	// eligible for ONE classification pass (a normal dispatch) that produces a complete receipt.
	WakeUnclassified
)

func (s WakeState) String() string {
	switch s {
	case WakeHold:
		return "hold"
	case WakeFire:
		return "fire"
	case WakeCouldNotCheck:
		return "could-not-check"
	default:
		return "unclassified"
	}
}

// WakeDecision is the evaluated result plus the human-facing detail the WAIT line prints.
type WakeDecision struct {
	State      WakeState
	Reason     string   // one-line detail: the blocker, the predicate, what changed / what is unreadable
	Changed    []string // declared inputs whose revision differs (WakeFire on relevant-input-changed)
	Unreadable []string // declared inputs that could not be read (WakeCouldNotCheck)
}

// WakeInputs is the ALREADY-AUTHORIZED reader of external observation. This code adds no
// production probes: the evaluator only asks the reader, and the offline reader answers from the
// local tree (revisions) or refuses to observe (actions). ok=false is could-not-check, never
// rounded to a value.
type WakeInputs interface {
	// Revision returns the CURRENT revision of a declared input key, or ok=false if it cannot
	// be read. The reader defines what a key means (a file path, "tool", "verify-def:<brief>").
	Revision(input string) (rev string, ok bool)
	// ActionCompleted reports whether a referenced external action has completed. ok=false when
	// the reader cannot observe it (the offline reader never can — that is could-not-check).
	ActionCompleted(ref string) (done bool, ok bool)
}

// EvaluateWake decides whether the receipt's held work should stay held, wake, be reported as
// could-not-check, or be reclassified. It is pure: all external observation comes through reader
// and the clock through now, so a fake reader and a fake clock exercise every path.
func (r WakeReceipt) EvaluateWake(reader WakeInputs, now time.Time) WakeDecision {
	if !r.Complete() {
		return WakeDecision{State: WakeUnclassified,
			Reason: "legacy/incomplete receipt — no complete verify-wake-v1 wake condition; eligible for one classification pass"}
	}
	base := "blocker: " + r.BlockerKind + refSuffix(r.BlockerRef) + "; next: " + r.NextActor() + "; wakes on " + r.WakePredicate

	switch r.WakePredicate {
	case WakeRelevantInputChanged:
		var changed, unreadable []string
		for _, in := range wakeSortedKeys(r.Inputs) {
			cur, ok := reader.Revision(in)
			if !ok {
				unreadable = append(unreadable, in)
				continue
			}
			if cur != r.Inputs[in] {
				changed = append(changed, in)
			}
		}
		// Could-not-check is decided FIRST: an unreadable input means unchangedness is not
		// establishable, so we never claim a hold on a partially-read scope.
		if len(unreadable) > 0 {
			return WakeDecision{State: WakeCouldNotCheck, Unreadable: unreadable,
				Reason: base + " — could-not-check: unreadable declared input(s): " + strings.Join(unreadable, ", ")}
		}
		if len(changed) > 0 {
			return WakeDecision{State: WakeFire, Changed: changed,
				Reason: base + " — woke: changed input(s): " + strings.Join(changed, ", ")}
		}
		return WakeDecision{State: WakeHold, Reason: base + " — all " + strconv.Itoa(len(r.Inputs)) + " declared input(s) unchanged"}

	case WakeReferencedActionDone:
		done, ok := reader.ActionCompleted(r.BlockerRef)
		if !ok {
			return WakeDecision{State: WakeCouldNotCheck, Unreadable: []string{r.BlockerRef},
				Reason: base + " — could-not-check: cannot observe action " + r.BlockerRef}
		}
		if done {
			return WakeDecision{State: WakeFire, Reason: base + " — woke: referenced action completed"}
		}
		return WakeDecision{State: WakeHold, Reason: base + " — referenced action not yet completed"}

	case WakeDeadlineReached:
		dl, err := parseWakeTime(r.Deadline)
		if err != nil {
			// A complete receipt is supposed to carry a parseable deadline; a malformed one is
			// could-not-check, never a silent hold past a date nobody can read.
			return WakeDecision{State: WakeCouldNotCheck,
				Reason: base + " — could-not-check: unparseable deadline " + r.Deadline}
		}
		if !now.Before(dl) {
			return WakeDecision{State: WakeFire, Reason: base + " — woke: deadline " + r.Deadline + " reached"}
		}
		return WakeDecision{State: WakeHold, Reason: base + " — deadline " + r.Deadline + " not yet reached"}

	case WakeExplicitRecheck:
		return WakeDecision{State: WakeFire, Reason: base + " — woke: explicit recheck requested (" + r.RecheckReason + ")"}
	}
	// Unreachable: Complete() already rejected an unknown predicate.
	return WakeDecision{State: WakeUnclassified, Reason: base + " — unknown predicate"}
}

func refSuffix(ref string) string {
	if strings.TrimSpace(ref) == "" {
		return ""
	}
	return " (ref " + ref + ")"
}

func wakeSortedKeys(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func parseWakeTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, os.ErrInvalid
}

// --- generation ----------------------------------------------------------------------------

// NewWakeReceipt builds a complete receipt from an ALREADY-VERIFIED result. The verifier
// identity is passed by the trusted-writer caller (the engine's RunnerID / the signed verdict
// principal), NEVER read from a free-text assertion — the same shape whether an agent-driven
// landing or the deterministic runner produced it. It returns the receipt and whether the inputs
// make it a complete receipt (an incomplete one is still returned, so the caller can persist a
// legacy-shaped row rather than nothing).
func NewWakeReceipt(id, repo, brief, verifier, outcome, sha string, rows []int, inputs map[string]string,
	toolVersion, blockerKind, blockerRef, predicate, deadline, recheckReason, ts string) WakeReceipt {
	return WakeReceipt{
		TS: ts, Brief: brief, Outcome: outcome, SHA: sha,
		Schema: SchemaWakeV1, ID: id, Repo: repo, Verifier: verifier, Rows: rows,
		Inputs: inputs, ToolVersion: toolVersion,
		BlockerKind: blockerKind, BlockerRef: blockerRef, WakePredicate: predicate,
		Deadline: deadline, RecheckReason: recheckReason,
	}
}

// MarshalLine renders the receipt as ONE JSONL line (no trailing newline) for the append-only
// sidecar. It is stable: the legacy keys stay first via the struct field order.
func (r WakeReceipt) MarshalLine() ([]byte, error) {
	return json.Marshal(r)
}

// --- offline production reader -------------------------------------------------------------

// RootRevisionReader is the OFFLINE, probe-free reader used by the live plan. It answers a
// revision query by content-hashing the named file under the repo root (a "file:<relpath>" key),
// the applicable tool version (the "tool" key), or refuses (ok=false) for a key it does not know.
// It NEVER observes an external action — ActionCompleted always returns ok=false, which is
// exactly could-not-check, so an offline plan cannot fabricate an action-completed wake.
type RootRevisionReader struct {
	Root        string
	ToolVersion string
}

// NewRootRevisionReader builds the default offline reader for a scanned root.
func NewRootRevisionReader(root, toolVersion string) *RootRevisionReader {
	return &RootRevisionReader{Root: root, ToolVersion: toolVersion}
}

const (
	inputKeyTool       = "tool"
	inputKeyFilePrefix = "file:"
)

// Revision content-hashes a declared input against the local tree. A "tool" key returns the
// configured tool version; a "file:<relpath>" key returns the sha256 of that file's bytes; any
// unreadable file or unknown key is ok=false (could-not-check).
func (rr *RootRevisionReader) Revision(input string) (string, bool) {
	switch {
	case input == inputKeyTool:
		if strings.TrimSpace(rr.ToolVersion) == "" {
			return "", false
		}
		return rr.ToolVersion, true
	case strings.HasPrefix(input, inputKeyFilePrefix):
		rel := strings.TrimPrefix(input, inputKeyFilePrefix)
		// Contain the read within the root: a key that climbs out is could-not-check, never a
		// read of an arbitrary path.
		clean := filepath.Clean(rel)
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.IsAbs(clean) {
			return "", false
		}
		b, err := os.ReadFile(filepath.Join(rr.Root, clean))
		if err != nil {
			return "", false
		}
		sum := sha256.Sum256(b)
		return hex.EncodeToString(sum[:]), true
	default:
		return "", false
	}
}

// ActionCompleted never observes an action offline — this is could-not-check by construction, so
// a referenced-action-completed receipt stays could-not-check on an offline plan until an
// already-authorized online reader supplies the observation.
func (rr *RootRevisionReader) ActionCompleted(string) (bool, bool) { return false, false }

// compile-time assertion: the offline reader satisfies the interface.
var _ WakeInputs = (*RootRevisionReader)(nil)
