package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// EscalationID is a ULID minted by the adapter on first ingest (spec §1.5).
// The domain sees only the opaque id; the platform ref ↔ domain id mapping
// lives in an adapter-owned table. Nothing durable dangles on it, because no
// domain id appears on the wire.
type EscalationID string

// CellName names a configured cell (config/schema/cells.go).
type CellName string

// Role is the loop role an escalation is attributed to. It mirrors the cell
// config's loop roles; kept as a domain string so the domain does not import
// the config package.
type Role string

// AutomationRef identifies the automation that filed an escalation.
// Its ZERO VALUE means unattributable — the adapter could not join the filing
// identity to a configured loop. That is a could-not-check, never a default,
// and [Escalation.Fields] carries the reason.
type AutomationRef struct {
	Cell CellName `json:"cell"`
	Role Role     `json:"role"`
}

// Zero reports whether the ref is the unattributable zero value.
func (a AutomationRef) Zero() bool { return a.Cell == "" && a.Role == "" }

func (a AutomationRef) String() string {
	if a.Zero() {
		return ""
	}
	return string(a.Cell) + "/" + string(a.Role)
}

// Actor is a named identity — a human or a bot login, platform-corroborated.
type Actor struct {
	Name  string `json:"name"`
	IsBot bool   `json:"is_bot"`
}

// Provenance records who raised the escalation (spec §1.1, §1.4).
//
// Ruling D4 puts the policy engine in service contract, so this build lands the
// field and NOTHING that produces PolicyRaised. Every escalation this adapter
// derives is AgentRaised, from the absence of the wire label.
type Provenance string

const (
	AgentRaised  Provenance = "agent-raised"
	PolicyRaised Provenance = "policy-raised"
)

// EscalationKind is the escalation vocabulary (spec §1.1).
//
// The wire carries `help wanted` with a space (the GitHub label); the domain
// spells it with a hyphen. The adapter normalises; the drift is real and is
// handled at exactly one place.
type EscalationKind string

const (
	KindNeedsDecision EscalationKind = "needs-decision"
	KindQuestion      EscalationKind = "question"
	KindHelpWanted    EscalationKind = "help-wanted"
	KindAsk           EscalationKind = "ask"
)

// Urgency orders the inbox. Sort is urgency-then-age (design §5.3).
type Urgency int

const (
	UrgencyRoutine Urgency = iota
	UrgencyElevated
	UrgencyUrgent
	UrgencyGold // an irreversible / human-gate sign-off: always the full item view
)

var urgencyNames = map[Urgency]string{
	UrgencyRoutine:  "routine",
	UrgencyElevated: "elevated",
	UrgencyUrgent:   "urgent",
	UrgencyGold:     "gold",
}

func (u Urgency) String() string {
	if s, ok := urgencyNames[u]; ok {
		return s
	}
	return "routine"
}

// MarshalJSON renders urgency as its word. Never as colour alone, and never
// as a bare integer a consumer has to know the scale of.
func (u Urgency) MarshalJSON() ([]byte, error) {
	return []byte(`"` + u.String() + `"`), nil
}

// urgencyByName is the reverse of urgencyNames, built once, so the word form
// [MarshalJSON] emits decodes back to the same value.
var urgencyByName = func() map[string]Urgency {
	m := make(map[string]Urgency, len(urgencyNames))
	for u, name := range urgencyNames {
		m[name] = u
	}
	return m
}()

// UnmarshalJSON restores an urgency from the word form MarshalJSON emits.
//
// A custom marshaller without its matching unmarshaller is a one-way lie
// (render.go says exactly this of the Escalation join): without this method a
// decoded Escalation would fail on the "routine"/"gold" word — which is what
// serving domain.Escalation over the v1 contract seam (the service contract) needs to
// round-trip. An unknown word is an error, never a silent fallback to routine,
// so a garbled urgency is visible rather than quietly downgraded.
func (u *Urgency) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("urgency must be one of routine|elevated|urgent|gold as a JSON string: %w", err)
	}
	v, ok := urgencyByName[s]
	if !ok {
		return fmt.Errorf("unknown urgency %q (want one of routine|elevated|urgent|gold)", s)
	}
	*u = v
	return nil
}

// OptionID is an agent-chosen slug (spec §1.1): [a-z0-9][a-z0-9-]{1,31},
// unique within one escalation. Platform-clean by construction.
type OptionID string

var optionIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,31}$`)

// Valid reports whether the id matches the spec's slug grammar.
func (o OptionID) Valid() bool { return optionIDRe.MatchString(string(o)) }

// Option is one typed choice (spec §1.1.1). The label and consequence are
// one line each; everything else is rationale prose, which explains options
// and is no longer how a choice is expressed.
type Option struct {
	ID          OptionID `json:"id"`
	Label       string   `json:"label"`
	Consequence string   `json:"consequence"`
}

// Answer is the recorded human decision (spec §1.1).
//
// Unverified marks `answered-unverified` (the authorization contract gap 3, item (5)):
// the issue closed with an answer marker that could NOT be verified as the
// named human's decision — a bot-authored record whose signature, key,
// answer_as, issue or time binding failed, or a human-authored record naming
// someone other than its author. It is an ANOMALY, never a human decision: By
// is the comment's real author, and Option/OptionsRev are left empty so no
// consumer can read the record's claimed choice as decided.
type Answer struct {
	Option     *OptionID `json:"option"`
	OptionsRev string    `json:"options_rev"`
	Text       string    `json:"text,omitempty"`
	By         Actor     `json:"by"`
	At         time.Time `json:"at"`
	Unverified bool      `json:"unverified,omitempty"`
}

// WorkItemRef is what an escalation blocks, in domain terms.
type WorkItemRef struct {
	Kind  string `json:"kind"`  // "brief" | "stream" | "work-item"
	Ident string `json:"ident"` // e.g. "the service contract"
}

// Escalation is the v2 domain type (spec §1.1).
//
// Fields whose derivation can fail carry their verdict in Fields, and
// serialise as JSON null when they are at could-not-check. Repeat, Updated,
// Read and Acked are SCHEMA ONLY in this build: the folding behaviour and the
// setters are the service contract. They are here because a field is near-free at
// greenfield and costs a migration later.
type Escalation struct {
	ID         EscalationID   `json:"id"`
	Cell       CellName       `json:"cell"`
	Automation AutomationRef  `json:"-"` // rendered via automationJSON, null when unattributable
	Kind       EscalationKind `json:"kind"`
	Provenance Provenance     `json:"provenance"`
	Urgency    Urgency        `json:"urgency"`

	Title     string        `json:"title"`
	Situation string        `json:"situation"`
	Options   []Option      `json:"options"`
	Rev       string        `json:"options_rev"`
	Rationale string        `json:"rationale,omitempty"`
	Blocks    []WorkItemRef `json:"blocks,omitempty"`

	// RawBody is retained when the v2 marker is present but the block does
	// not parse. Design §10's malformed-escalation rule: render raw, never
	// drop — an unparseable question is still a stalled loop.
	RawBody string `json:"raw_body,omitempty"`

	FiledAt time.Time  `json:"filed_at"`
	Repeat  int        `json:"repeat"`
	Updated *time.Time `json:"updated"`
	Read    *time.Time `json:"read"`
	Acked   *time.Time `json:"acked"`

	Supersedes *EscalationID `json:"supersedes"`
	Answer     *Answer       `json:"answer"`

	// Fields is the per-field three-state map. Every field the adapter had
	// to DERIVE (as opposed to copy) appears here.
	Fields FieldConfidence `json:"field_confidence"`
}

// Age derives from FiledAt — it is not stored (spec §1.1).
func (e Escalation) Age(now time.Time) time.Duration { return now.Sub(e.FiledAt) }

// UnreadAgain is computed, never toggled (spec §1.1.2).
func (e Escalation) UnreadAgain() bool {
	return e.Read != nil && e.Updated != nil && e.Updated.After(*e.Read)
}

// Attributable reports whether the automation join succeeded.
func (e Escalation) Attributable() bool { return !e.Automation.Zero() }

// CollapseKey is (Cell, Automation, Kind) — spec §1.1.3.
//
// This build stores the key and the repeat count as COLUMNS. It performs no
// folding: the merge rule, the OptionsRev guard and the Updated bump are
// the service contract. Key derivation lives here so both briefs compute the same
// key from the same code.
type CollapseKey struct {
	Cell       CellName
	Automation AutomationRef
	Kind       EscalationKind
}

// String renders the key for storage in the collapse_key column.
//
// An UNATTRIBUTABLE escalation has no collapse key at all — it returns "".
// Spec §1.1.3: unattributable automations never merge, because the adapter
// cannot prove same-origin. Storing a shared placeholder would let two
// unrelated unattributable filings collide the moment 14 turns folding on.
func (k CollapseKey) String() string {
	if k.Automation.Zero() {
		return ""
	}
	return fmt.Sprintf("%s|%s|%s", k.Cell, k.Automation, k.Kind)
}

// Key returns the escalation's collapse key.
func (e Escalation) Key() CollapseKey {
	return CollapseKey{Cell: e.Cell, Automation: e.Automation, Kind: e.Kind}
}

// OptionsRev fingerprints the ORDERED (ID, Label, Consequence) set
// (spec §1.1, §1.1.1). It returns "" when opts is nil — the prose-v1 case.
//
// The spec pins neither algorithm nor encoding, and the rev crosses the wire
// (the options grammar emits `options-rev <shorthash>`; this adapter re-derives it from
// a re-polled body), so the encoding is FIXED HERE and both sides must use
// this function:
//
//	sha256 over, for each option in order: len(ID) "\x1f" ID "\x1f"
//	len(Label) "\x1f" Label "\x1f" len(Consequence) "\x1f" Consequence "\x1e"
//
// Length prefixes and non-printable separators mean no field content can
// forge a boundary — `a|b` and `a` + `|b` fingerprint differently, which
// matters because these strings are agent-authored (§6.6: untrusted text).
// Rendered as full lowercase hex; ShortRev takes the wire form.
func OptionsRev(opts []Option) string {
	if opts == nil {
		return ""
	}
	h := sha256.New()
	for _, o := range opts {
		for _, part := range []string{string(o.ID), o.Label, o.Consequence} {
			fmt.Fprintf(h, "%d\x1f%s\x1f", len(part), part)
		}
		h.Write([]byte{0x1e})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ShortRev is the wire form of a rev — the first 12 hex chars, as written
// into `Decision: \`id\` (options-rev <shorthash>)`.
func ShortRev(rev string) string {
	if len(rev) <= 12 {
		return rev
	}
	return rev[:12]
}

// SituationRev is the merge guard for OPTION-LESS kinds (spec §1.1.3): a
// content hash of the normalised situation. Normalisation collapses runs of
// whitespace and lowercases, so a reflowed body is the same question.
func SituationRev(situation string) string {
	norm := strings.ToLower(strings.Join(strings.Fields(situation), " "))
	sum := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(sum[:])
}

// MergeGuard returns the guard for this escalation: OptionsRev where options
// exist, the situation content hash otherwise.
func (e Escalation) MergeGuard() string {
	if e.Options != nil {
		return e.Rev
	}
	return SituationRev(e.Situation)
}

// SortInbox orders escalations urgency-then-age, newest-urgent first
// (design §5.3). Ties break on ID so the order is total and a re-poll does
// not reshuffle a stable queue.
func SortInbox(items []Escalation) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.Urgency != b.Urgency {
			return a.Urgency > b.Urgency
		}
		if !a.FiledAt.Equal(b.FiledAt) {
			return a.FiledAt.Before(b.FiledAt) // oldest first within an urgency band
		}
		return a.ID < b.ID
	})
}
