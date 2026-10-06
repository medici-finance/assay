package deskkit

// reviewfinding.go — persistent review findings and the existing round cap, derived
// from durable forge records rather than from an agent's memory.
//
// THE PROBLEM. A review round loses continuity the moment the reviewing (or working)
// agent is replaced: the successor rereads the whole pull request from prose and
// restates old objections under fresh IDs, the round counter resets, and a class that
// was one re-review away from the human arbitration lane starts again from zero. The
// same rereading lets a worker's own reply read as if it cleared a blocking finding, and
// lets a poll tick look like a review round. None of that state was anywhere durable: it
// lived in the transcript of whichever agent happened to be running.
//
// THE MODEL. A finding is a typed record embedded in the forge review/reply body it was
// raised on — additively, inside an HTML comment a legacy reader ignores — so the durable
// forge history IS the ledger. Every scheduling fact this file answers (what is still
// open, what a worker disputed, how many rounds a class has run, whether the cap is hit)
// is DERIVED by folding those records in their durable order, so it is identical after an
// agent is replaced or a process restarts: re-deriving from the same records yields the
// same IDs, the same counts and the same single arbiter packet. Statelessness is the
// restart-survival guarantee — there is no in-memory ledger to lose.
//
// THREE INDEPENDENT BARRIERS, unchanged by this file. The derivation classifies; it never
// authorizes. Actor identity and head come from the AUTHENTICATED forge event, so a
// worker's prose cannot author a reviewer's resolution; the existing per-item claim and
// the reviewer/verifier identity gates still stand; and the human decision authority at
// the cap is untouched — the cap produces ONE packet for a human, it never overrules a
// reviewer or manufactures an approval. A scheduling receipt can never authorize a write.

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// FindingBlockSchema is the versioned schema tag every block carries. The version is in
// the tag, not only in a field, so a future reader can route a v2 block without parsing a
// v1 body it does not understand.
const FindingBlockSchema = "review-finding/v1"

// The block is delimited by an HTML comment so a legacy Markdown reader — and every human
// reading the thread — sees nothing, while a parser can find it unambiguously. This is the
// additive-record compatibility contract: an older tool that never learned the block skips
// it as a comment and reads the surrounding prose exactly as before.
const (
	findingBlockOpen  = "<!-- assay:review-finding:v1"
	findingBlockClose = "-->"
)

// FindingState is the lifecycle state of one finding. The five states are the interface
// contract's states, and the zero value is deliberately NOT one of them: an unset state is
// a parse/ederivation defect, never a silent "open" or "resolved".
type FindingState string

const (
	// StateOpen — raised by a reviewer, not yet responded to.
	StateOpen FindingState = "open"
	// StateFixedAwaitingReview — a worker asserts a fix; only a reviewer re-review at the
	// current head can move it to resolved. A head change lands a finding here, never at
	// resolved.
	StateFixedAwaitingReview FindingState = "fixed-awaiting-review"
	// StateDisputed — a worker contests the finding with counter-evidence; still blocking
	// until a reviewer resolves or maintains it.
	StateDisputed FindingState = "disputed"
	// StateResolved — a reviewer cleared it with current-head evidence. A worker can never
	// author this state for a blocking finding.
	StateResolved FindingState = "resolved"
	// StateAwaitingArbitration — the class hit the existing round cap; one packet is filed
	// for the human decision lane and the class is held.
	StateAwaitingArbitration FindingState = "awaiting-arbitration"
)

func (s FindingState) known() bool {
	switch s {
	case StateOpen, StateFixedAwaitingReview, StateDisputed, StateResolved, StateAwaitingArbitration:
		return true
	}
	return false
}

// BlockerKind separates a defect in THIS branch's code/content from a shared external
// prerequisite (a red shared-CI leg, an upstream fact). A shared CI fault is an integration
// blocker with one shared repair reference; it is NOT automatically a correctness defect in
// every dependent PR, and this distinction is what stops a red shared leg from being
// re-invented as N independent content findings.
type BlockerKind string

const (
	BlockerCodeContent    BlockerKind = "code-content"
	BlockerExternalPrereq BlockerKind = "external-prerequisite"
)

func (k BlockerKind) known() bool {
	return k == BlockerCodeContent || k == BlockerExternalPrereq
}

// Severity is whether a finding blocks. Only a reviewer may promote advisory->blocking, and
// only with changed impact or new evidence (see the role gate in Validate).
type Severity string

const (
	SeverityBlocking Severity = "blocking"
	SeverityAdvisory Severity = "advisory"
)

func (s Severity) known() bool { return s == SeverityBlocking || s == SeverityAdvisory }

// ActorRole is who authored a record, DERIVED from the authenticated forge event — the
// reviewer App on a review, the worker App on a reply. It is never read from prose: that is
// the whole point of persisting it, because prose is exactly what a replacement agent
// rewrites.
type ActorRole string

const (
	RoleReviewer ActorRole = "reviewer"
	RoleWorker   ActorRole = "worker"
)

// Finding is one typed finding as it appears in a block.
type Finding struct {
	// ID is the stable identity of the finding across heads and agents. A newly discovered
	// OCCURRENCE of the same proposition reuses this ID; only a genuinely new proposition
	// gets a new one. An ID is unique only WITHIN a lane: the correctness and security
	// reviewers each number their own findings, so the ledger identity is (Lane, ID).
	ID string `json:"id"`
	// Lane optionally names the review lane the finding belongs to (LaneCorrectness,
	// LaneSecurity, ...). It is normally taken from the review record the finding was read
	// off (ForgeRecord.Lane). A worker has no lane of its own: a worker reply names the lane
	// here whenever the ID is held by more than one lane, and a block lane wins over a worker
	// record's. A reviewer record with an established lane — including LaneAmbiguous — speaks
	// only for that lane: a block lane naming another is reported and keyed under the record's
	// lane. Validate refuses any word that is not a published lane name (reviewlanes.go), and
	// the fold ignores one on read.
	Lane string `json:"lane,omitempty"`
	// Class is the claim class the finding belongs to. The round cap is per class: fixing
	// one sentence of a class never resets the class, and a sibling occurrence retains it.
	Class string `json:"class"`
	// Severity — blocking or advisory.
	Severity Severity `json:"severity"`
	// Blocker — a code/content defect or a shared external prerequisite.
	Blocker BlockerKind `json:"blocker"`
	// State — the lifecycle state this record asserts for the finding.
	State FindingState `json:"state"`
	// OriginHead is the head SHA the finding was FIRST raised against (a reviewer record).
	// EvidenceHead below is the head the ASSERTING record was authored against.
	OriginHead string `json:"originHead,omitempty"`
	// EvidenceHead is the head the evidence in THIS record was gathered at. A reviewer
	// resolution whose EvidenceHead is not the current head cannot clear the finding — a
	// verdict on stale code is never carried blindly across a change.
	EvidenceHead string `json:"evidenceHead,omitempty"`
	// Failure is the concrete failure or reproduction. A blocking finding MUST carry this
	// (or an explicit evidence-based Explanation) — a bare assertion cannot block.
	Failure string `json:"failure,omitempty"`
	// Explanation is the evidence-based explanation a blocking finding may carry in place of
	// a mechanical reproduction (e.g. an omitted acceptance deliverable).
	Explanation string `json:"explanation,omitempty"`
	// Resolution is the condition that would resolve the finding.
	Resolution string `json:"resolution,omitempty"`
	// Evidence is the list of evidence links/refs backing the record's assertion.
	Evidence []string `json:"evidence,omitempty"`
	// SharedRepair is the one shared repair reference an external-prerequisite (shared-CI)
	// blocker points at, so multiple PRs cite ONE repair rather than inventing one defect
	// each.
	SharedRepair string `json:"sharedRepair,omitempty"`
}

func (f Finding) blocking() bool { return f.Severity == SeverityBlocking }

// StandingBlocker reports whether the finding still blocks: blocking severity AND not
// resolved. It is the ONE predicate every reader outside this file uses to classify a
// block's findings — a re-review CR lists the findings it now records as resolved beside the
// ones still open, and a classifier that reads severity alone counts those resolved entries
// as live blockers (#1985). The severity constant and blocking() are referenced only in this
// file; a guard test (TestStandingBlockerIsTheOnlyReader) pins that.
func (f Finding) StandingBlocker() bool { return f.blocking() && f.State != StateResolved }

// StatedLane is the lane the finding's block states, case-folded and trimmed the way the
// ledger compares lanes; empty when the block states none.
func (f Finding) StatedLane() string { return normLane(f.Lane) }

// hasConcreteBasis reports whether a blocking finding carries a concrete reproduction or an
// explicit evidence-based explanation, as the interface contract requires.
func (f Finding) hasConcreteBasis() bool {
	return strings.TrimSpace(f.Failure) != "" ||
		strings.TrimSpace(f.Explanation) != "" ||
		len(f.Evidence) > 0
}

// FindingBlockV1 is the versioned, additive block embedded in a forge review/reply body.
type FindingBlockV1 struct {
	Schema   string    `json:"schema"`
	Findings []Finding `json:"findings"`
}

// ParseFindingBlock extracts the finding block from a forge body. It returns
// (block, present, error):
//
//   - present=false, err=nil — the body carries no block. This is the LEGACY-PROSE case and
//     is not an error: an older review/reply, or a plain comment, simply asserts nothing
//     about the typed ledger. A clean finding set is NEVER inferred from unparseable prose.
//   - present=true, err!=nil — a block is present but malformed or wrong-schema. This is a
//     refusal: a body that claims to carry the typed record but does not is a defect, not a
//     silent legacy body.
//   - present=true, err=nil — a well-formed block.
func ParseFindingBlock(body string) (*FindingBlockV1, bool, error) {
	i := strings.Index(body, findingBlockOpen)
	if i < 0 {
		return nil, false, nil
	}
	rest := body[i+len(findingBlockOpen):]
	j := strings.Index(rest, findingBlockClose)
	if j < 0 {
		return nil, true, Refused("review-finding: the '" + findingBlockOpen +
			"' marker is not closed by '" + findingBlockClose + "' — the block is truncated")
	}
	raw := strings.TrimSpace(rest[:j])
	var b FindingBlockV1
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return nil, true, Refused("review-finding: the finding block is not valid JSON: " + err.Error())
	}
	if b.Schema != FindingBlockSchema {
		return nil, true, Refused(fmt.Sprintf("review-finding: block schema %q is not %q — refusing to read a block whose schema this reader does not know",
			b.Schema, FindingBlockSchema))
	}
	return &b, true, nil
}

// Validate checks a parsed block against the interface contract for the authoring role. It
// is the write-time gate deskpost (reviewer) and deskreply (worker) run BEFORE any network:
// a body that carries a malformed or role-violating finding block refuses with zero side
// effects, exactly like every other pre-network body check.
//
// The role gate is the enforcement of "worker prose cannot author a reviewer resolution":
//
//   - a WORKER record may set open, fixed-awaiting-review or disputed. It may NOT set
//     resolved on a blocking finding (only a reviewer clears a blocker), and it may NOT set
//     awaiting-arbitration (the cap is derived, never hand-asserted).
//   - a WORKER record may not PROMOTE a finding to blocking severity — a promotion from
//     advisory to blocking is a reviewer act, recorded with changed impact or new evidence.
//   - any blocking finding, from either role, MUST carry a concrete reproduction or an
//     explicit evidence-based explanation.
func (b *FindingBlockV1) Validate(role ActorRole) error {
	if b.Schema != FindingBlockSchema {
		return Refused("review-finding: block schema is not " + FindingBlockSchema)
	}
	seen := map[string]bool{}
	for i := range b.Findings {
		f := b.Findings[i]
		id := strings.TrimSpace(f.ID)
		if id == "" {
			return Refused("review-finding: a finding has no id — every finding needs a stable id")
		}
		if seen[id] {
			return Refused("review-finding: finding id " + id + " appears twice in one block")
		}
		seen[id] = true
		if strings.TrimSpace(f.Class) == "" {
			return Refused("review-finding: finding " + id + " has no class — the round cap is per class")
		}
		if strings.Contains(f.ID, laneSep) || strings.Contains(f.Class, laneSep) {
			return Refused("review-finding: finding " + id + " has an id or class containing '" + laneSep +
				"' — the ledger joins a lane to an id or class with it, so an id or class carrying one would collide with another lane's finding")
		}
		if l := normLane(f.Lane); l != "" && !knownFindingLane(l) {
			return Refused("review-finding: finding " + id + " names an unknown lane " + strconv.Quote(f.Lane) +
				" — a lane is one of " + strings.Join(FindingLaneNames(), ", ") + " (an unknown word would fork the finding into an identity space of its own)")
		}
		if !f.Severity.known() {
			return Refused("review-finding: finding " + id + " has an unknown severity " + string(f.Severity))
		}
		if !f.Blocker.known() {
			return Refused("review-finding: finding " + id + " has an unknown blocker kind " + string(f.Blocker))
		}
		if !f.State.known() {
			return Refused("review-finding: finding " + id + " has an unknown state " + string(f.State))
		}
		if f.blocking() && !f.hasConcreteBasis() {
			return Refused("review-finding: blocking finding " + id +
				" carries no concrete reproduction, evidence-based explanation, or evidence link — a bare assertion cannot block")
		}
		if role == RoleWorker {
			if f.State == StateResolved && f.blocking() {
				return Refused("review-finding: a worker reply cannot resolve blocking finding " + id +
					" — only a reviewer resolves a blocker at the current head")
			}
			if f.State == StateAwaitingArbitration {
				return Refused("review-finding: a worker reply cannot set finding " + id +
					" to awaiting-arbitration — the cap is derived from the durable round history, never hand-asserted")
			}
		}
	}
	return nil
}

// ValidateReviewFindingBlock is the one call a write verb makes: parse the body's finding
// block (if any) and validate it for the authoring role. A body with NO block is fine
// (nil) — the block is additive, and the overwhelming majority of reviews and replies do
// not carry one. A malformed or role-violating block refuses.
func ValidateReviewFindingBlock(body []byte, role ActorRole) error {
	b, present, err := ParseFindingBlock(string(body))
	if err != nil {
		return err
	}
	if !present {
		return nil
	}
	return b.Validate(role)
}

// RenderFindingBlock renders a block as the HTML-comment-delimited body fragment a review or
// reply appends. It is the inverse of ParseFindingBlock and the helper a reviewer/worker
// prompt names when it tells an agent to emit the typed record.
func RenderFindingBlock(b FindingBlockV1) string {
	b.Schema = FindingBlockSchema
	raw, _ := json.MarshalIndent(b, "", "  ")
	return findingBlockOpen + "\n" + string(raw) + "\n" + findingBlockClose
}

// ---------------------------------------------------------------------------
// Records and derivation
// ---------------------------------------------------------------------------

// RecordKind is whether a forge record is a reviewer verdict or a worker reply.
type RecordKind string

const (
	RecordReview RecordKind = "review"
	RecordReply  RecordKind = "reply"
)

// Verdict is a reviewer record's verdict shape (for review records only).
type Verdict string

const (
	VerdictApprove        Verdict = "approve"
	VerdictRequestChanges Verdict = "request-changes"
	VerdictComment        Verdict = "comment"
	VerdictNone           Verdict = ""
)

// ForgeRecord is one durable forge event, with its actor identity and head taken from the
// authenticated event rather than from its prose. A record MAY carry a typed finding block
// (Block != nil); a legacy record does not, and asserts nothing about the ledger.
type ForgeRecord struct {
	// Seq is the durable, monotonic order the forge assigns (a review id, a comment id). The
	// ledger is folded in Seq order, which is what makes the derivation identical across a
	// restart: it depends only on the durable records, never on wall-clock arrival.
	Seq int
	// Kind — review or reply.
	Kind RecordKind
	// Role — reviewer or worker, DERIVED from the authenticated actor.
	Role ActorRole
	// Actor is the authenticated login (for could-not-check reporting).
	Actor string
	// Head is the head SHA the record was authored against.
	Head string
	// Verdict — for review records.
	Verdict Verdict
	// Lane is the review lane the record belongs to (a Lane.Name such as "correctness" or "security"),
	// established from the record itself (the review's stamp / verdict block), never from a
	// finding's prose. Empty means the lane could not be established: the record's findings
	// then key by their bare ID, which is the pre-lane behaviour and is unambiguous only on a
	// single-lane thread. A worker reply may leave it empty; its findings are then matched to
	// the one lane that holds the ID, or reported Blind when more than one does. A producer
	// sets it on every record of a thread or on none: a payload that mixes lane-less and
	// laned reviewer records for one ID splits that finding in two, so DeriveLedger reports
	// such a payload Blind. A reviewer record that cannot be attributed to one published
	// lane carries LaneAmbiguous, never an empty lane, so that it never takes its block's lane.
	Lane string
	// Block — the typed finding block, or nil for a legacy record.
	Block *FindingBlockV1
}

// findingLanes is the closed lane vocabulary a finding block may name: the published review
// lanes (reviewlanes.go). Every lane keys its own identity space, so an open vocabulary
// would let a typo fork a finding; Validate refuses any other word.
var findingLanes = []Lane{LaneCorrectness, LaneSecurity, LaneFactCheck, LaneFailFirst}

// LaneAmbiguous is the reserved RECORD lane for a reviewer record whose lane cannot be
// attributed to exactly one published lane — a single review body claiming both the
// correctness and the security verdict. It is deliberately outside findingLanes: Validate
// refuses it in a block and the fold never honours it as a block lane, so no block can name
// it. A record carrying it takes the reviewer-record branch of the fold, which ignores the
// block's own lane, so its findings key into an identity space of their own: such a record
// can still raise an open finding, but it can never resolve a finding of any published lane.
// The parentheses keep it from ever colliding with a published lane name.
const LaneAmbiguous = "(ambiguous)"

func knownFindingLane(lane string) bool {
	for _, l := range findingLanes {
		if l.Name == lane {
			return true
		}
	}
	return false
}

// FindingLaneNames is the closed lane vocabulary a finding block may name, in order. A
// consumer that checks a write gate against the read side enumerates it, so a lane added
// here is covered by that check without the check being edited.
func FindingLaneNames() []string {
	out := make([]string, 0, len(findingLanes))
	for _, l := range findingLanes {
		out = append(out, l.Name)
	}
	return out
}

// laneSep joins a lane to an ID or class in a ledger key. Validate refuses it in a finding's
// id and class (and a lane is one of the published lane names, none of which carries it), so
// every key written through the write gate is unambiguous. The fold re-applies the same
// rules on READ for a record that reached the forge without the gate: a finding whose id or
// class carries the separator, or whose block names a word outside the vocabulary, is
// reported Blind rather than keyed (see applyFinding), and a thread that mixes lane-less and
// laned reviewer records is reported Blind too (see DeriveLedger). The deskpost
// content-defect gate goes further and fails closed on any of these.
const laneSep = "/"

// ledgerKey is a finding's identity in the ledger: (lane, id). An empty lane keys by the
// bare id, so a single-lane thread reads exactly as it did before lanes existed.
func ledgerKey(lane, id string) string {
	lane = normLane(lane)
	if lane == "" {
		return id
	}
	return lane + laneSep + id
}

func normLane(lane string) string { return strings.ToLower(strings.TrimSpace(lane)) }

// RoundCap is the existing per-class round cap. It is NOT introduced or changed by this
// brief: the canonical pr-review-desk skill already caps full verdict/fix/re-review rounds
// at three per finding class, then files an arbiter packet to the human decision lane. This
// constant is that same threshold, made derivable so it survives agent replacement.
const RoundCap = 3

// LedgerFinding is a finding's derived, current state in the ledger.
type LedgerFinding struct {
	Finding
	// Key is the finding's ledger identity — (lane, id) rendered as "lane/id", or the bare ID
	// when no lane is established — and the key it holds in FindingLedger.Findings.
	Key string
	// ClassKey is the finding's class scoped to its lane — the key it holds in
	// FindingLedger.Rounds and FindingLedger.Held. Two lanes that happen to name a class the
	// same way run independent round counters.
	ClassKey string
	// FirstHead is the head the finding was first raised against.
	FirstHead string
	// LastReviewerHead is the head of the most recent REVIEWER record touching the finding —
	// the head any resolution is pinned to.
	LastReviewerHead string
	// Rounds mirrors the class's derived round count, for readers that hold a finding.
	Rounds int
}

// Label is the finding as a reader should name it in prose: its bare ID, with its lane in
// parentheses when one is established ("A3 (lane security)"). The ledger key is an internal
// identity, never something a successor should copy into the id field of a new record.
func (f *LedgerFinding) Label() string { return f.ID + laneNote(f.Lane) }

// classState accumulates the per-class round machine during a fold.
type classState struct {
	class    string
	ref      ClassRef // the lane and bare class the lane-scoped key stands for
	rounds   int
	severity Severity
	blocker  BlockerKind
	state    FindingState
	held     bool
	// pendingResponse is set when a worker has responded since the last reviewer verdict —
	// the middle leg of a review -> response -> re-review transition. A reviewer re-review
	// counts a round ONLY when this is set, which is exactly why a poll tick or a second
	// reviewer verdict with no intervening worker response does not inflate the count.
	pendingResponse bool
	// raised is true once the class has had its opening reviewer verdict.
	raised    bool
	findingID string
}

// ArbiterPacket is the single deduplicated packet a class produces when it reaches the cap.
// It is DATA for the sanctioned filing path (deskfile to the human decision lane); this
// package never files it and never overrules a reviewer. Exactly one is produced per class
// that hits the cap, no matter how many times the records are re-derived.
type ArbiterPacket struct {
	// Class is the lane-scoped class key (see LedgerFinding.ClassKey); FindingIDs are ledger
	// keys. FindingLedger.Classes and LedgerFinding.Label give the bare forms to print.
	Class      string
	Rounds     int
	FindingIDs []string
	Summary    string
}

// ClassRef is the (lane, class) pair a lane-scoped class key stands for.
type ClassRef struct {
	Lane  string
	Class string
}

// Label is the class as a reader should name it in prose: the bare class, with its lane in
// parentheses when one is established.
func (c ClassRef) Label() string { return c.Class + laneNote(c.Lane) }

// laneNote is the " (lane <name>)" suffix a label carries when a lane is established.
func laneNote(lane string) string {
	if lane == "" {
		return ""
	}
	return " (lane " + lane + ")"
}

// ClassOf returns the lane and bare class a lane-scoped class key stands for. A key the fold
// never produced reads as a lane-less class of that name.
func (l *FindingLedger) ClassOf(classKey string) ClassRef {
	if ref, ok := l.Classes[classKey]; ok {
		return ref
	}
	return ClassRef{Class: classKey}
}

// FindingLedger is the whole derived state of a fold: the current findings, the per-class
// round counts, the arbiter packets, and the could-not-check reasons.
type FindingLedger struct {
	// Findings is the current state of each finding, keyed by (lane, ID) — see ledgerKey.
	Findings map[string]*LedgerFinding
	// Rounds is the per-class round count, keyed by the lane-scoped class (see
	// LedgerFinding.ClassKey).
	Rounds map[string]int
	// Held is the set of lane-scoped classes held at the cap (awaiting arbitration).
	Held map[string]bool
	// Classes maps each lane-scoped class key (a key of Rounds and Held, an ArbiterPacket.Class)
	// to the lane and bare class it stands for — what a reader prints and a successor reuses.
	Classes map[string]ClassRef
	// Arbiter is one packet per class that reached the cap.
	Arbiter []ArbiterPacket
	// Blind names every record the fold could not positively interpret — a record missing
	// its actor role or head, a worker record asserting a reviewer resolution. A blind
	// record is reported AS ITSELF: it never clears a finding and never counts a round.
	Blind []string
}

// FindingIDs returns the ledger's finding keys ((lane, ID), see ledgerKey), sorted, for deterministic output.
func (l *FindingLedger) FindingIDs() []string {
	out := make([]string, 0, len(l.Findings))
	for id := range l.Findings {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// OpenBlocking returns the IDs of findings that are still blocking and not resolved — the
// outstanding work a follow-up review must carry. External-prerequisite (shared-CI)
// blockers are INCLUDED: they still block a ready-flip, but as a shared repair, not a
// per-PR content defect (see ContentDefects / SharedRepairs).
func (l *FindingLedger) OpenBlocking() []string {
	var out []string
	for _, id := range l.FindingIDs() {
		f := l.Findings[id]
		if f.blocking() && f.State != StateResolved {
			out = append(out, id)
		}
	}
	return out
}

// ContentDefects returns the IDs of blocking code/content findings that are still open — the
// defects a reviewer must see resolved in THIS branch. A shared external prerequisite is not
// one of them.
func (l *FindingLedger) ContentDefects() []string {
	var out []string
	for _, id := range l.FindingIDs() {
		f := l.Findings[id]
		if f.blocking() && f.Blocker == BlockerCodeContent && f.State != StateResolved {
			out = append(out, id)
		}
	}
	return out
}

// SharedRepairs returns the distinct shared-repair references cited by external-prerequisite
// blockers, so a reader can see that N PRs cite ONE repair rather than N invented defects.
func (l *FindingLedger) SharedRepairs() []string {
	set := map[string]bool{}
	for _, f := range l.Findings {
		if f.Blocker == BlockerExternalPrereq && strings.TrimSpace(f.SharedRepair) != "" {
			set[f.SharedRepair] = true
		}
	}
	out := make([]string, 0, len(set))
	for r := range set {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// ResolvedAtHead reports whether the finding with ledger key id is resolved AND its resolution was pinned to
// currentHead. A resolution recorded at a stale head is not carried across a change: the
// finding reads unresolved at the new head, which is what "no stale approval" means.
func (l *FindingLedger) ResolvedAtHead(id, currentHead string) bool {
	f, ok := l.Findings[id]
	if !ok {
		return false
	}
	return f.State == StateResolved && f.LastReviewerHead == currentHead
}

// DeriveLedger folds the records in Seq order into the current ledger. It is a PURE function
// of the durable records: the same records always fold to the same ledger, which is the
// restart- and replacement-survival guarantee. Nothing here authorizes a write, overrules a
// reviewer, or files the arbiter packet — it classifies, and reports could-not-check as
// itself.
func DeriveLedger(records []ForgeRecord) *FindingLedger {
	l := &FindingLedger{
		Findings: map[string]*LedgerFinding{},
		Rounds:   map[string]int{},
		Held:     map[string]bool{},
		Classes:  map[string]ClassRef{},
	}
	classes := map[string]*classState{}
	arbFiled := map[string]bool{}

	// Fold in durable order. A stable sort by Seq makes the derivation independent of the
	// order the caller happened to assemble the slice in.
	ordered := make([]ForgeRecord, len(records))
	copy(ordered, records)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Seq < ordered[j].Seq })

	// A thread whose reviewer records are partly laned and partly lane-less splits a finding
	// two lanes' ids could name, and a lane-less reviewer record takes its block's lane — so
	// the mix is reported as itself rather than silently keyed both ways.
	var laned, laneless []string
	for _, r := range ordered {
		if r.Role != RoleReviewer || r.Block == nil || strings.TrimSpace(r.Head) == "" {
			continue
		}
		if normLane(r.Lane) == "" {
			laneless = append(laneless, strconv.Itoa(r.Seq))
		} else {
			laned = append(laned, strconv.Itoa(r.Seq))
		}
	}
	if len(laned) > 0 && len(laneless) > 0 {
		l.Blind = append(l.Blind, fmt.Sprintf("reviewer records mix laned (seq %s) and lane-less (seq %s) — a lane-less reviewer record takes its block's lane, so its findings cannot be attributed safely; set the lane on every reviewer record of a thread or on none",
			strings.Join(laned, ","), strings.Join(laneless, ",")))
	}

	for _, r := range ordered {
		// A record whose authenticated role or head could not be established is
		// could-not-check: it never clears a finding and never counts a round.
		if r.Role != RoleReviewer && r.Role != RoleWorker {
			l.Blind = append(l.Blind, fmt.Sprintf("record seq %d has no authenticated role — cannot attribute it", r.Seq))
			continue
		}
		if strings.TrimSpace(r.Head) == "" {
			l.Blind = append(l.Blind, fmt.Sprintf("record seq %d by %s has no head SHA — cannot pin its assertion", r.Seq, r.Role))
			continue
		}
		if rl := normLane(r.Lane); rl != "" && rl != LaneAmbiguous && !knownFindingLane(rl) {
			// A record lane outside the published vocabulary (a producer's typo, or a payload
			// that bypassed the readers) would fork its findings into an identity space of
			// their own, so the record is not attributed at all.
			l.Blind = append(l.Blind, fmt.Sprintf("record seq %d carries lane %q, which is not a published lane — cannot attribute its findings; ignored", r.Seq, r.Lane))
			continue
		}
		if r.Block == nil {
			// A legacy record with no typed block asserts nothing about the ledger. It is
			// NOT evidence of a clean finding set — an unparseable prose reply leaves every
			// standing finding exactly where it was.
			continue
		}
		for _, f := range r.Block.Findings {
			l.applyFinding(classes, arbFiled, r, f)
		}
	}

	for c, cs := range classes {
		l.Classes[c] = cs.ref
		l.Rounds[c] = cs.rounds
		if cs.held {
			l.Held[c] = true
		}
	}
	// Mirror each class's round count onto its findings for holders of a single finding.
	for _, lf := range l.Findings {
		lf.Rounds = l.Rounds[lf.ClassKey]
	}
	return l
}

// applyFinding folds one finding-assertion from one record into the ledger.
func (l *FindingLedger) applyFinding(classes map[string]*classState, arbFiled map[string]bool, r ForgeRecord, f Finding) {
	if strings.TrimSpace(f.ID) == "" || strings.TrimSpace(f.Class) == "" {
		l.Blind = append(l.Blind, fmt.Sprintf("record seq %d carries a finding with no id/class — cannot track it", r.Seq))
		return
	}
	// The write gate's key rules, re-applied on read for a record that reached the forge
	// without the gate: an id or class carrying the key separator would collide with another
	// lane's key, so it is reported and not keyed at all.
	if strings.Contains(f.ID, laneSep) || strings.Contains(f.Class, laneSep) {
		l.Blind = append(l.Blind, fmt.Sprintf("record seq %d: finding %q has an id or class containing %q — it would collide with another lane's ledger key; ignored",
			r.Seq, f.ID, laneSep))
		return
	}
	lane, recLane, blockLane := "", normLane(r.Lane), normLane(f.Lane)
	if blockLane != "" && !knownFindingLane(blockLane) {
		// A block lane outside the closed vocabulary — a typo, or the reserved LaneAmbiguous —
		// is never honoured: it would fork the finding into an identity space of its own, or
		// write into the space reserved for unattributable reviewer records.
		l.Blind = append(l.Blind, fmt.Sprintf("record seq %d: finding %s states lane %q, which is not a published lane — the block lane is ignored",
			r.Seq, f.ID, f.Lane))
		blockLane = ""
	}
	switch {
	case r.Role == RoleReviewer && recLane != "":
		// A reviewer record speaks only for its own lane. A block lane that names a
		// different one is NOT honoured: honouring it would let one lane's verdict resolve
		// another lane's finding. Keying the assertion under the record's own lane keeps it
		// (an open finding still counts, a resolution clears only that lane's finding) and
		// the mismatch is reported as itself.
		lane = recLane
		if blockLane != "" && blockLane != recLane {
			l.Blind = append(l.Blind, fmt.Sprintf("record seq %d: reviewer finding %s states lane %q but the record is the %q lane — a reviewer speaks only for its own lane; keyed under %q",
				r.Seq, f.ID, blockLane, recLane, recLane))
		}
	default:
		// A worker has no lane of its own, so the lane its block states wins over whatever
		// the record carries. A reviewer record with no established lane at all takes the
		// block's — the single-lane, pre-lane shape; a reviewer whose lane is ambiguous
		// carries LaneAmbiguous and so never reaches this branch.
		lane = firstNonEmpty(blockLane, recLane)
	}
	if lane == "" && r.Role == RoleWorker {
		// A worker reply need not name a lane: attach it to the one lane that holds the ID.
		// Two lanes holding it is exactly the ambiguity this ledger exists to refuse to
		// guess at, so it is reported as itself and asserts nothing.
		var holders, holderLanes []string
		for k, lf := range l.Findings {
			if lf.ID == f.ID {
				holders = append(holders, k)
			}
		}
		sort.Strings(holders)
		for _, k := range holders {
			holderLanes = append(holderLanes, firstNonEmpty(l.Findings[k].Lane, "(none)"))
		}
		switch {
		case len(holders) > 1:
			l.Blind = append(l.Blind, fmt.Sprintf("record seq %d: worker finding %s names no lane and the id is held by lanes %s — cannot tell which lane it answers; ignored (state the lane in the block)",
				r.Seq, f.ID, strings.Join(holderLanes, ", ")))
			return
		case len(holders) == 1:
			lane = l.Findings[holders[0]].Lane
		}
	}
	f.Lane = lane
	key := ledgerKey(lane, f.ID)
	classKey := ledgerKey(lane, f.Class)

	cs := classes[classKey]
	if cs == nil {
		cs = &classState{class: classKey, ref: ClassRef{Lane: lane, Class: f.Class}, state: StateOpen}
		classes[classKey] = cs
	}

	lf := l.Findings[key]
	if lf == nil {
		lf = &LedgerFinding{Finding: f, Key: key, ClassKey: classKey, FirstHead: firstNonEmpty(f.OriginHead, r.Head)}
		l.Findings[key] = lf
	} else {
		// A newly discovered OCCURRENCE reuses the ID and keeps the class and round history.
		// Preserve the accumulated evidence across records (a fix that changes one head must
		// not drop the evidence the finding was raised with) and keep the earliest head.
		merged := f
		merged.Evidence = mergeEvidence(lf.Evidence, f.Evidence)
		if strings.TrimSpace(merged.OriginHead) == "" {
			merged.OriginHead = lf.OriginHead
		}
		first := lf.FirstHead
		lf.Finding = merged
		lf.FirstHead = first
		// The finding's class is the one its latest record names, so its class key follows.
		lf.ClassKey = classKey
	}
	if cs.blocker == "" {
		cs.blocker = f.Blocker
	}
	if f.Severity == SeverityBlocking {
		cs.severity = SeverityBlocking
	} else if cs.severity == "" {
		cs.severity = f.Severity
	}

	switch r.Role {
	case RoleReviewer:
		l.applyReviewer(cs, arbFiled, r, lf)
	case RoleWorker:
		l.applyWorker(cs, r, lf)
	}
}

// applyReviewer folds a reviewer record: it raises, maintains, resolves or (at the cap)
// arbitrates a class.
func (l *FindingLedger) applyReviewer(cs *classState, arbFiled map[string]bool, r ForgeRecord, lf *LedgerFinding) {
	lf.LastReviewerHead = r.Head
	cs.findingID = lf.Key

	// A resolution only lands from a reviewer, and only with current-head evidence: the
	// EvidenceHead of the asserting record must be the head it was authored against. A
	// reviewer record that claims resolved on stale evidence does not clear the finding.
	if lf.State == StateResolved && lf.blocking() {
		evHead := firstNonEmpty(lf.EvidenceHead, r.Head)
		if evHead != r.Head {
			// Stale-head resolution: refuse to carry it. Leave the finding open.
			lf.State = StateFixedAwaitingReview
			l.Blind = append(l.Blind, fmt.Sprintf("finding %s: reviewer resolution at seq %d carries evidence head %s != record head %s — not cleared",
				lf.Label(), r.Seq, short12(evHead), short12(r.Head)))
			cs.state = lf.State
			return
		}
		cs.state = StateResolved
		return
	}

	if !cs.raised {
		// The opening verdict: round 1's clock starts, but this is not yet a completed
		// round (a round completes on the re-review after a worker response).
		cs.raised = true
		cs.rounds = 0
		cs.pendingResponse = false
		if cs.state != StateResolved {
			cs.state = lf.State
		}
		return
	}
	if cs.held {
		// The class is already at the cap: a further reviewer verdict (including one that
		// merely notices a sibling sentence of the same class) does NOT count a round, does
		// NOT reset the class, and does NOT refile the packet.
		lf.State = StateAwaitingArbitration
		return
	}
	if !cs.pendingResponse {
		// A re-review with no intervening worker response is a poll, not a round.
		return
	}
	// A genuine review -> response -> re-review transition completes a round.
	cs.pendingResponse = false
	if cs.rounds >= RoundCap {
		// The cap is reached: produce ONE arbiter packet for the human decision lane and
		// hold the class. Re-deriving the same records refiles nothing (arbFiled dedup), and
		// a duplicate reviewer sweep at the cap is caught by cs.held above.
		if !arbFiled[cs.class] {
			arbFiled[cs.class] = true
			l.Arbiter = append(l.Arbiter, ArbiterPacket{
				Class:      cs.class,
				Rounds:     cs.rounds,
				FindingIDs: l.findingsInClass(cs.class),
				Summary: fmt.Sprintf("class %q%s reached the %d-round cap; %d finding(s) held for the human decision lane",
					cs.ref.Class, laneNote(cs.ref.Lane), RoundCap, len(l.findingsInClass(cs.class))),
			})
		}
		cs.held = true
		cs.state = StateAwaitingArbitration
		lf.State = StateAwaitingArbitration
		return
	}
	cs.rounds++
	if cs.state != StateResolved {
		cs.state = lf.State
	}
}

// applyWorker folds a worker record. A worker can move a finding to fixed-awaiting-review or
// disputed and set the middle leg of a round, but it can never resolve a blocking finding or
// hand-assert the cap — those are enforced here as a second layer behind Validate's write
// gate, because a derivation must be correct even on records that reached the forge by a
// path that skipped the tool (a raw comment, a legacy body).
func (l *FindingLedger) applyWorker(cs *classState, r ForgeRecord, lf *LedgerFinding) {
	if cs.held {
		// Nothing a worker posts moves a class already at the cap.
		lf.State = StateAwaitingArbitration
		return
	}
	if lf.blocking() && lf.State == StateResolved {
		// A worker record cannot author a reviewer resolution. Demote its claim to
		// fixed-awaiting-review and report the attempt.
		lf.State = StateFixedAwaitingReview
		l.Blind = append(l.Blind, fmt.Sprintf("finding %s: worker record seq %d asserted 'resolved' on a blocking finding — a worker cannot clear a blocker; held as fixed-awaiting-review",
			lf.Label(), r.Seq))
	}
	if lf.State == StateAwaitingArbitration {
		// A worker cannot hand-assert the cap either.
		lf.State = StateFixedAwaitingReview
		l.Blind = append(l.Blind, fmt.Sprintf("finding %s: worker record seq %d asserted 'awaiting-arbitration' — the cap is derived, not asserted; held as fixed-awaiting-review", lf.Label(), r.Seq))
	}
	// A worker response to a still-open class arms the middle leg of a round.
	if cs.raised && cs.state != StateResolved {
		cs.pendingResponse = true
	}
	if lf.State != StateResolved && lf.State != StateAwaitingArbitration {
		cs.state = lf.State
	}
}

func (l *FindingLedger) findingsInClass(class string) []string {
	var out []string
	for _, id := range l.FindingIDs() {
		if l.Findings[id].ClassKey == class {
			out = append(out, id)
		}
	}
	return out
}

func mergeEvidence(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range append(append([]string{}, a...), b...) {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func short12(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
