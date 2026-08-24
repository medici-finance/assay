package deskkit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Decide is the escape-valve primitive: a bounded ask-an-agent consult for the
// deterministic desk loops. A loop reaches a point its code cannot classify — is
// this verify FAIL rot or a real regression? is this refusal retryable? — and
// instead of guessing badly (the reviewer-retry incident tripped a 15-minute breaker
// by retrying a refusal five times) or stopping, it asks a read-only agent to pick
// ONE member of a fixed vocabulary the loop declared up front.
//
// The contract is deliberately narrow, and every narrowing is a safety property:
//
//   - The agent ADVISES, the loop ACTS. The consult returns a member of the
//     vocabulary; acting on it is the loop's own code. The valve never grows the
//     loop's action space — it only chooses among moves the loop could already make.
//   - The consulted agent is READ-ONLY: structured context in, one enum member plus a
//     one-line justification out. It is never handed a shell and never handed tools.
//     That is the Advisor contract (a pure function of the Consult), not an honour
//     system the primitive can enforce, so the primitive bounds the BLAST RADIUS
//     instead: whatever the agent returns is validated against the vocabulary.
//   - RESERVED VERBS: no vocabulary may contain a member equivalent to a human-gate
//     action (approve / flip / merge / ready / sign / close-gate). The deny-list is
//     enforced at CONSTRUCTION (NewQuestion), not at answer time — a vocabulary that
//     could name a human-gate move is rejected before it can ever be consulted.
//   - Fail-closed default: an invalid answer, a timeout, an advisor error, a spent
//     budget, or the kill switch all resolve to the pre-declared conservative
//     DEFAULT. A confused or captured valve degrades to the loop's no-valve
//     behaviour; it never spins and never picks an unbounded move.
//   - Every consult JOURNALS: the question, a digest of the context (never the raw,
//     possibly-untrusted context), the answer, its justification, the elapsed time,
//     and which of the above outcomes produced the answer. An advised (non-default)
//     answer is returned ONLY if its journal line was written — un-journalled advice
//     is discarded and the default is used instead.
//   - The valve is an OPTIMIZATION. Every consumer must run correctly with it
//     disabled: setting DESK_DECIDE_DISABLED=1 makes every Decide return its default
//     without consulting anyone.
//
// Injection posture: the Context field carries untrusted repo text (PR bodies, CI
// logs). Enum validation bounds a prompt injection to picking a wrong-but-safe branch
// of the vocabulary; anything the agent returns that is not a vocabulary member is
// malformed and lands on the default. Because reserved verbs are refused at
// construction, no branch the injection could steer toward is a human-gate move.

// decideDisabledEnv is the valve's own kill switch. It is intentionally SEPARATE from
// the desk-tools DISABLED switch: the valve is an optimization layered on top of loops
// that already run correctly without it, so an operator can turn the valve off (forcing
// every consult to its conservative default) without halting the loops themselves.
const decideDisabledEnv = "DESK_DECIDE_DISABLED"

// ValveDisabled reports whether the escape valve is turned off by its env kill switch.
// When true, Decide consults no one and returns the declared default. Exported so a
// consumer can log "valve off" once at startup rather than infer it from behaviour.
func ValveDisabled() bool { return os.Getenv(decideDisabledEnv) == "1" }

// reservedVerbs are the human-gate action roots no vocabulary may name. A member is
// rejected at construction if any of its word-tokens equals one of these, or if its
// separators-removed form contains one of the multi-word phrases below. The list is
// the compiled-in embodiment of "a strong model does not satisfy a human gate": the
// valve must never be able to advise a move only a human may make.
var reservedVerbs = map[string]bool{
	"approve":  true,
	"approved": true,
	"approval": true,
	"flip":     true,
	"flipped":  true,
	"merge":    true,
	"merged":   true,
	"ready":    true,
	"sign":     true,
	"signed":   true,
	"signoff":  true,
}

// reservedPhrases catch hyphen/space-joined human-gate actions whose individual
// tokens ("close", "gate") are innocuous but whose joined form is not.
var reservedPhrases = []string{"closegate", "signoff", "readyflip"}

var tokenSplit = regexp.MustCompile(`[^a-z0-9]+`)

// reservedMember reports whether a vocabulary member names a human-gate action, and if
// so which reserved root/phrase it matched (for the refusal message).
func reservedMember(member string) (string, bool) {
	low := strings.ToLower(strings.TrimSpace(member))
	for _, tok := range tokenSplit.Split(low, -1) {
		if tok == "" {
			continue
		}
		if reservedVerbs[tok] {
			return tok, true
		}
	}
	joined := tokenSplit.ReplaceAllString(low, "")
	for _, phrase := range reservedPhrases {
		if strings.Contains(joined, phrase) {
			return phrase, true
		}
	}
	return "", false
}

// Question is a validated, reusable consult template: a fixed vocabulary paired with a
// pre-declared conservative default. It is constructed once and consulted many times.
// The reserved-verb deny-list, the non-empty check, the no-duplicates check, and the
// default-membership invariant are all enforced HERE, at NewQuestion — never at answer
// time — so a vocabulary that violates the contract can never reach a consult.
type Question struct {
	prompt     string
	vocabulary []string
	def        string
	member     map[string]bool
}

// NewQuestion builds a Question, enforcing every vocabulary invariant. It refuses
// (returns a nil Question and an error) when:
//   - vocabulary is empty;
//   - any member is blank or duplicated;
//   - def is not a member of vocabulary;
//   - any member names a human-gate action (the reserved-verb deny-list).
//
// prompt is the fixed framing of the question (e.g. "Classify this verify FAIL"); the
// per-situation detail is supplied at Decide time. A copy of vocabulary is retained, so
// a later mutation of the caller's slice cannot change what was validated.
func NewQuestion(prompt string, vocabulary []string, def string) (*Question, error) {
	if len(vocabulary) == 0 {
		return nil, Refused("decide: vocabulary is empty (a consult needs at least one option)")
	}
	member := make(map[string]bool, len(vocabulary))
	vocab := make([]string, 0, len(vocabulary))
	for _, m := range vocabulary {
		if strings.TrimSpace(m) == "" {
			return nil, Refused("decide: vocabulary contains a blank member")
		}
		if member[m] {
			return nil, Refused(fmt.Sprintf("decide: vocabulary contains a duplicate member %q", m))
		}
		if root, bad := reservedMember(m); bad {
			return nil, Refused(fmt.Sprintf(
				"decide: vocabulary member %q names a human-gate action (%q) — the escape valve may never advise a move only a human may make", m, root))
		}
		member[m] = true
		vocab = append(vocab, m)
	}
	if !member[def] {
		return nil, Refused(fmt.Sprintf("decide: default %q is not a member of the vocabulary", def))
	}
	return &Question{prompt: prompt, vocabulary: vocab, def: def, member: member}, nil
}

// Vocabulary returns a copy of the validated vocabulary.
func (q *Question) Vocabulary() []string {
	out := make([]string, len(q.vocabulary))
	copy(out, q.vocabulary)
	return out
}

// Default returns the pre-declared conservative default.
func (q *Question) Default() string { return q.def }

// Advice is what a consulted agent returns: exactly one candidate answer and a
// one-line justification. Answer is validated against the vocabulary by Decide; an
// answer outside the vocabulary is malformed and resolves to the default.
type Advice struct {
	Answer        string
	Justification string
}

// Advisor is the read-only agent the valve consults. The contract is that Advise is a
// PURE FUNCTION of the Consultation: structured context in, one Advice out. An Advisor
// is never given a shell or tools — it reads and answers. The primitive cannot enforce
// that property inside a third-party Advisor, so it enforces the blast radius instead
// (vocabulary validation + reserved-verb deny-list + fail-closed default).
type Advisor interface {
	Advise(ctx context.Context, c Consultation) (Advice, error)
}

// AdvisorFunc adapts a plain function to the Advisor interface.
type AdvisorFunc func(ctx context.Context, c Consultation) (Advice, error)

// Advise implements Advisor.
func (f AdvisorFunc) Advise(ctx context.Context, c Consultation) (Advice, error) { return f(ctx, c) }

// Consultation is the read-only view handed to an Advisor. It restates the fixed
// prompt, the per-situation detail, the untrusted context, and the vocabulary/default
// the agent must choose within. It deliberately carries no callbacks, no tools, and no
// mutable state.
type Consultation struct {
	Prompt     string
	Detail     string
	Context    string
	Vocabulary []string
	Default    string
}

// DecisionRecord is the journal line for one consult. The raw Context is never stored;
// only its digest is, so an untrusted PR body cannot be laundered into the journal.
type DecisionRecord struct {
	TS            string        `json:"ts"`
	Loop          string        `json:"loop"`
	Item          string        `json:"item"`
	Prompt        string        `json:"prompt"`
	Detail        string        `json:"detail"`
	ContextDigest string        `json:"contextDigest"`
	Vocabulary    []string      `json:"vocabulary"`
	Default       string        `json:"default"`
	Answer        string        `json:"answer"`
	Justification string        `json:"justification"`
	Elapsed       time.Duration `json:"elapsedNs"`
	Outcome       string        `json:"outcome"`
}

// Decide outcomes (the Outcome field of a DecisionRecord).
const (
	// OutcomeAdvised — the agent returned a valid vocabulary member and it was used.
	OutcomeAdvised = "advised"
	// OutcomeDisabled — the valve's kill switch was armed; the default was used.
	OutcomeDisabled = "default-disabled"
	// OutcomeNoAdvisor — no advisor was wired; the default was used.
	OutcomeNoAdvisor = "default-no-advisor"
	// OutcomeBudget — the per-item or per-hour budget was spent; the default was used.
	OutcomeBudget = "default-budget"
	// OutcomeTimeout — the consult exceeded its timeout; the default was used.
	OutcomeTimeout = "default-timeout"
	// OutcomeError — the advisor returned an error; the default was used.
	OutcomeError = "default-error"
	// OutcomeInvalid — the advisor's answer was not a vocabulary member; the default
	// was used.
	OutcomeInvalid = "default-invalid"
	// OutcomeUnjournalled — an advised answer could not be journalled, so it was
	// discarded and the default was used (an advised move is never taken un-audited).
	OutcomeUnjournalled = "default-unjournalled"
)

// Journal is where a consult records its DecisionRecord. A consumer supplies its
// loop's journal; a nil Journal falls back to the shared desk-tools audit log.
type Journal interface {
	Record(DecisionRecord) error
}

// JournalFunc adapts a plain function to the Journal interface.
type JournalFunc func(DecisionRecord) error

// Record implements Journal.
func (f JournalFunc) Record(r DecisionRecord) error { return f(r) }

// Budget bounds how often the valve may be consulted, on two independent axes:
//
//   - PerItem: consults for a single item key (e.g. "owner/repo#123") within the
//     rolling hour — a loop stuck on one item cannot consult it forever;
//   - PerHour: consults across ALL items within the rolling hour — a loop confused
//     across many items degrades to its no-valve behaviour instead of amplifying.
//
// A zero limit on an axis means "unbounded on that axis". A Budget is safe for
// concurrent use and is meant to be shared across the loop's items (that is what makes
// the per-hour axis a fleet-wide cap rather than a per-item one). The zero Budget is
// usable (both axes unbounded); prefer NewBudget for a configured one.
type Budget struct {
	PerItem int
	PerHour int

	mu    sync.Mutex
	calls []budgetCall
	now   func() time.Time // test hook; nil → time.Now
}

type budgetCall struct {
	item string
	at   time.Time
}

// NewBudget returns a Budget with the given per-item and per-hour caps (0 = unbounded).
func NewBudget(perItem, perHour int) *Budget {
	return &Budget{PerItem: perItem, PerHour: perHour}
}

func (b *Budget) clock() time.Time {
	if b.now != nil {
		return b.now()
	}
	return time.Now()
}

// spend records a consult against item if both budget axes permit it, and reports
// whether it was permitted. Calls older than one hour are pruned first, so both caps
// are rolling-window caps. A permitted call is recorded; a denied call is not (a denial
// must not itself consume budget).
func (b *Budget) spend(item string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.clock()
	cutoff := now.Add(-time.Hour)
	kept := b.calls[:0]
	for _, c := range b.calls {
		if c.at.After(cutoff) {
			kept = append(kept, c)
		}
	}
	b.calls = kept

	if b.PerHour > 0 && len(b.calls) >= b.PerHour {
		return false
	}
	if b.PerItem > 0 {
		n := 0
		for _, c := range b.calls {
			if c.item == item {
				n++
			}
		}
		if n >= b.PerItem {
			return false
		}
	}
	b.calls = append(b.calls, budgetCall{item: item, at: now})
	return true
}

// Consult is one invocation of a Question against a concrete situation.
type Consult struct {
	// Item is the stable key for the per-item budget axis (e.g. "owner/repo#123").
	Item string
	// Detail is the per-situation framing appended to the Question's fixed prompt.
	Detail string
	// Context is structured, possibly-UNTRUSTED text handed to the advisor. Its digest
	// is journalled; the raw text is not.
	Context string
	// Advisor is the read-only agent to consult. A nil Advisor is treated as "valve not
	// wired": Decide returns the default (OutcomeNoAdvisor).
	Advisor Advisor
	// Journal receives the DecisionRecord. A nil Journal falls back to the shared
	// desk-tools audit log (AuditJournal).
	Journal Journal
	// Budget bounds consult frequency. A nil Budget is unbounded (still kill-switch
	// gated). Share ONE Budget across a loop's items so the per-hour axis is fleet-wide.
	Budget *Budget
	// Timeout bounds one advisor call. Zero means defaultDecideTimeout.
	Timeout time.Duration
}

// defaultDecideTimeout bounds an advisor call when Consult.Timeout is zero. A consult
// that cannot answer promptly must fall to the default rather than stall the loop.
const defaultDecideTimeout = 30 * time.Second

// Decide consults the valve and returns a member of the Question's vocabulary — the
// advised answer when the agent returned a valid one and it was journalled, otherwise
// the pre-declared default. It NEVER returns a value outside the vocabulary, and on the
// operational paths it NEVER returns an error: every failure mode (kill switch, no
// advisor, spent budget, timeout, advisor error, invalid answer, un-journallable
// advice) resolves to the default so the loop proceeds with its conservative choice.
//
// The only non-nil error is a nil receiver (a programming error) — a real Question is
// always non-nil once NewQuestion has returned it.
func (q *Question) Decide(ctx context.Context, c Consult) (string, error) {
	if q == nil {
		return "", Unverifiable("decide: nil Question (construct one with NewQuestion)", nil)
	}

	journal := c.Journal
	if journal == nil {
		journal = AuditJournal{}
	}

	rec := DecisionRecord{
		Loop:          currentLoop(),
		Item:          c.Item,
		Prompt:        StripControl(q.prompt),
		Detail:        StripControl(c.Detail),
		ContextDigest: Sha256Hex([]byte(c.Context)),
		Vocabulary:    q.Vocabulary(),
		Default:       q.def,
		Answer:        q.def,
	}

	// Kill switch: the valve is an optimization; when off, consult no one.
	if ValveDisabled() {
		rec.Outcome = OutcomeDisabled
		return q.finalize(journal, rec)
	}
	// No advisor wired is functionally the same as the valve being off.
	if c.Advisor == nil {
		rec.Outcome = OutcomeNoAdvisor
		return q.finalize(journal, rec)
	}
	// Budget: a spent budget degrades to the no-valve default.
	if c.Budget != nil && !c.Budget.spend(c.Item) {
		rec.Outcome = OutcomeBudget
		return q.finalize(journal, rec)
	}

	timeout := c.Timeout
	if timeout <= 0 {
		timeout = defaultDecideTimeout
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	consultation := Consultation{
		Prompt:     q.prompt,
		Detail:     c.Detail,
		Context:    c.Context,
		Vocabulary: q.Vocabulary(),
		Default:    q.def,
	}

	start := time.Now()
	advice, err := c.Advisor.Advise(cctx, consultation)
	rec.Elapsed = time.Since(start)

	switch {
	case err != nil && errors.Is(err, context.DeadlineExceeded):
		rec.Outcome = OutcomeTimeout
	case cctx.Err() != nil && errors.Is(cctx.Err(), context.DeadlineExceeded):
		// The advisor returned without flagging the deadline, but the context is
		// past it: treat as a timeout, not as usable advice.
		rec.Outcome = OutcomeTimeout
	case err != nil:
		rec.Outcome = OutcomeError
	case !q.member[advice.Answer]:
		// Malformed or injected answer: bounded to the default.
		rec.Outcome = OutcomeInvalid
		rec.Justification = StripControl(truncate(advice.Justification, 280))
	default:
		rec.Outcome = OutcomeAdvised
		rec.Answer = advice.Answer
		rec.Justification = StripControl(truncate(advice.Justification, 280))
	}

	return q.finalize(journal, rec)
}

// finalize journals the record and returns the answer. For an advised (non-default)
// answer, a journal failure is fatal to the ADVICE: the answer is downgraded to the
// default, re-journalled as OutcomeUnjournalled, and the default is returned — an
// advised move is never taken un-audited. For a default outcome, a journal failure is
// logged to stderr but does not change the (already conservative) result.
func (q *Question) finalize(journal Journal, rec DecisionRecord) (string, error) {
	if rec.TS == "" {
		rec.TS = time.Now().UTC().Format(time.RFC3339)
	}
	err := journal.Record(rec)
	if err == nil {
		return rec.Answer, nil
	}
	if rec.Outcome == OutcomeAdvised {
		// Discard un-journallable advice and fall to the default. Record the
		// downgrade best-effort; if THAT also fails, the default is still safe.
		down := rec
		down.Outcome = OutcomeUnjournalled
		down.Answer = q.def
		down.Justification = ""
		if derr := journal.Record(down); derr != nil {
			fmt.Fprintf(os.Stderr, "deskkit.Decide: WARNING: journal write failed twice; using default %q: %v\n", q.def, derr)
		}
		return q.def, nil
	}
	fmt.Fprintf(os.Stderr, "deskkit.Decide: WARNING: journal write failed for %s outcome; using default %q: %v\n", rec.Outcome, q.def, err)
	return rec.Answer, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// AuditJournal is the default Journal: it writes each DecisionRecord to the shared
// desk-tools audit log (~/.config/assay/audit.jsonl) as a "decide" verb entry, reusing
// the one append-only, control-sanitized sink the other desk tools already share. The
// context digest is the entry's ArgsDigest; the answer, outcome, elapsed, and
// justification are packed into Detail. The raw context is never written.
type AuditJournal struct{}

// Record implements Journal against the desk-tools audit log.
func (AuditJournal) Record(r DecisionRecord) error {
	tool := r.Loop
	if tool == "" {
		tool = "decide"
	}
	detail := fmt.Sprintf("item=%q outcome=%s answer=%q elapsed=%s just=%q",
		r.Item, r.Outcome, r.Answer, r.Elapsed, r.Justification)
	return Log(Entry{
		TS:         r.TS,
		Tool:       tool,
		Verb:       "decide",
		ArgsDigest: r.ContextDigest,
		Result:     ResultOK,
		Detail:     StripControl(detail),
	})
}

// currentLoop reports the loop name for journalling, from $DESK_LOOP, else "decide".
func currentLoop() string {
	if s := strings.TrimSpace(os.Getenv(loopEnv)); s != "" {
		return s
	}
	return "decide"
}
