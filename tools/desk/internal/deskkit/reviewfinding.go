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
	// gets a new one.
	ID string `json:"id"`
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
	// Block — the typed finding block, or nil for a legacy record.
	Block *FindingBlockV1
}

// RoundCap is the existing per-class round cap. It is NOT introduced or changed by this
// brief: the canonical pr-review-desk skill already caps full verdict/fix/re-review rounds
// at three per finding class, then files an arbiter packet to the human decision lane. This
// constant is that same threshold, made derivable so it survives agent replacement.
const RoundCap = 3

// LedgerFinding is a finding's derived, current state in the ledger.
type LedgerFinding struct {
	Finding
	// FirstHead is the head the finding was first raised against.
	FirstHead string
	// LastReviewerHead is the head of the most recent REVIEWER record touching the finding —
	// the head any resolution is pinned to.
	LastReviewerHead string
	// Rounds mirrors the class's derived round count, for readers that hold a finding.
	Rounds int
}

// classState accumulates the per-class round machine during a fold.
type classState struct {
	class    string
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
	Class      string
	Rounds     int
	FindingIDs []string
	Summary    string
}

// FindingLedger is the whole derived state of a fold: the current findings, the per-class
// round counts, the arbiter packets, and the could-not-check reasons.
type FindingLedger struct {
	// Findings is the current state of each finding, keyed by ID.
	Findings map[string]*LedgerFinding
	// Rounds is the per-class round count.
	Rounds map[string]int
	// Held is the set of classes held at the cap (awaiting arbitration).
	Held map[string]bool
	// Arbiter is one packet per class that reached the cap.
	Arbiter []ArbiterPacket
	// Blind names every record the fold could not positively interpret — a record missing
	// its actor role or head, a worker record asserting a reviewer resolution. A blind
	// record is reported AS ITSELF: it never clears a finding and never counts a round.
	Blind []string
}

// FindingIDs returns the ledger's finding IDs, sorted, for deterministic output.
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

// ResolvedAtHead reports whether finding id is resolved AND its resolution was pinned to
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
	}
	classes := map[string]*classState{}
	arbFiled := map[string]bool{}

	// Fold in durable order. A stable sort by Seq makes the derivation independent of the
	// order the caller happened to assemble the slice in.
	ordered := make([]ForgeRecord, len(records))
	copy(ordered, records)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Seq < ordered[j].Seq })

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
		l.Rounds[c] = cs.rounds
		if cs.held {
			l.Held[c] = true
		}
	}
	// Mirror each class's round count onto its findings for holders of a single finding.
	for _, lf := range l.Findings {
		lf.Rounds = l.Rounds[lf.Class]
	}
	return l
}

// applyFinding folds one finding-assertion from one record into the ledger.
func (l *FindingLedger) applyFinding(classes map[string]*classState, arbFiled map[string]bool, r ForgeRecord, f Finding) {
	if strings.TrimSpace(f.ID) == "" || strings.TrimSpace(f.Class) == "" {
		l.Blind = append(l.Blind, fmt.Sprintf("record seq %d carries a finding with no id/class — cannot track it", r.Seq))
		return
	}
	cs := classes[f.Class]
	if cs == nil {
		cs = &classState{class: f.Class, state: StateOpen}
		classes[f.Class] = cs
	}

	lf := l.Findings[f.ID]
	if lf == nil {
		lf = &LedgerFinding{Finding: f, FirstHead: firstNonEmpty(f.OriginHead, r.Head)}
		l.Findings[f.ID] = lf
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
	cs.findingID = lf.ID

	// A resolution only lands from a reviewer, and only with current-head evidence: the
	// EvidenceHead of the asserting record must be the head it was authored against. A
	// reviewer record that claims resolved on stale evidence does not clear the finding.
	if lf.State == StateResolved && lf.blocking() {
		evHead := firstNonEmpty(lf.EvidenceHead, r.Head)
		if evHead != r.Head {
			// Stale-head resolution: refuse to carry it. Leave the finding open.
			lf.State = StateFixedAwaitingReview
			l.Blind = append(l.Blind, fmt.Sprintf("finding %s: reviewer resolution at seq %d carries evidence head %s != record head %s — not cleared",
				lf.ID, r.Seq, short12(evHead), short12(r.Head)))
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
				Summary: fmt.Sprintf("class %q reached the %d-round cap; %d finding(s) held for the human decision lane",
					cs.class, RoundCap, len(l.findingsInClass(cs.class))),
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
			lf.ID, r.Seq))
	}
	if lf.State == StateAwaitingArbitration {
		// A worker cannot hand-assert the cap either.
		lf.State = StateFixedAwaitingReview
		l.Blind = append(l.Blind, fmt.Sprintf("finding %s: worker record seq %d asserted 'awaiting-arbitration' — the cap is derived, not asserted; held as fixed-awaiting-review", lf.ID, r.Seq))
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
		if l.Findings[id].Class == class {
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
