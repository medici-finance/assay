package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/domain"
)

// This file is desk__escalate: the tool that flips the incentive (design §7.1).
// Filing a well-formed escalation is one call instead of work, so a blocked
// loop files and stays alive rather than blocking in-terminal. It is a
// FORMATTER + FILER — GitHub stays the record (spec §1.4); the tool writes the
// same issue the escalation wire contract defines.

// Filer is the App-identity write path desk__escalate needs, plus the
// open-escalation read the filing-time collapse consults.
//
// It is an interface so deskd can back OpenEscalations by the derived index
// (the console's own view of what is open) while the writes go through the App,
// and so a test can supply a Filer with no network at all. A nil Filer on the
// Server means the mutating tools return an honest error rather than a silent
// no-op.
type Filer interface {
	// Identity is the login every write here is attributed to. It MUST be the
	// service App (AppIdentity); the authority model asserts no tool authors as
	// a human, and this is the value that makes the write side match the claim.
	Identity() string
	// OpenEscalations returns the currently-open escalations for a cell that a
	// filing could collapse into, with the collapse key and merge guard the
	// index derived for each. It is the read half of filing-time collapse.
	OpenEscalations(ctx context.Context, cell string) ([]OpenEscalation, error)
	// FileEscalation opens a new issue and returns its platform ref.
	FileEscalation(ctx context.Context, e NewEscalation) (FiledRef, error)
	// CommentReRaise records a repeat on an existing open escalation without
	// opening a new issue ("re-raised ×k"), so GitHub stays the record and the
	// count is re-derivable from cold (spec §1.1.3).
	CommentReRaise(ctx context.Context, ref FiledRef, note string) error
}

// FiledRef is a filed escalation's platform ref, as returned by the write path.
// It lives at the adapter edge, never in the domain (spec §1.5).
type FiledRef struct {
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	URL    string `json:"url"`
}

// OpenEscalation is one open escalation as a collapse candidate.
type OpenEscalation struct {
	Ref         FiledRef `json:"ref"`
	CollapseKey string   `json:"collapse_key"`
	MergeGuard  string   `json:"merge_guard"`
	Repeat      int      `json:"repeat"`
	FloodItem   bool     `json:"flood_item"`
}

// NewEscalation is the request the write path files.
type NewEscalation struct {
	Repo   string
	Cell   string
	Title  string
	Body   string
	Labels []string
}

// OptionInput is one option as an escalate argument. The id is optional: an
// agent that supplies none gets option-1..n; a supplied id is validated at
// filing (spec §1.1.1).
type OptionInput struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Consequence string `json:"consequence"`
}

// EscalateArgs is the desk__escalate argument shape.
//
// There is deliberately NO provenance field: provenance is enforced by caller
// identity, not parameter trust (spec §1.4), so any provenance a loop supplies
// is ignored — a loop cannot dress its question as a tripped wire. Every v1
// filing derives agent-raised.
type EscalateArgs struct {
	Cell      string        `json:"cell"`
	Repo      string        `json:"repo"`
	Kind      string        `json:"kind"`
	Title     string        `json:"title"`
	Situation string        `json:"situation"`
	Options   []OptionInput `json:"options"`
	Urgency   string        `json:"urgency"`
	Blocks    []string      `json:"blocks"`
}

// EscalateResult is what desk__escalate returns. It returns immediately; the
// loop continues (design §7.1).
type EscalateResult struct {
	// Outcome is one of: "filed" (a new issue), "merged" (filing-time collapse
	// onto an open escalation), "flood-collapsed" (this loop is escalating
	// abnormally and its filings folded into one signal).
	Outcome     string    `json:"outcome"`
	Ref         *FiledRef `json:"ref,omitempty"`
	Repeat      int       `json:"repeat,omitempty"`
	Provenance  string    `json:"provenance"`
	CollapseKey string    `json:"collapse_key,omitempty"`
	OptionsRev  string    `json:"options_rev,omitempty"`
	Attributed  bool      `json:"attributed"`
	Note        string    `json:"note,omitempty"`
}

// The escalation-label vocabulary has ONE owner in this module: the adapter's
// label table ([adapter.KindForLabel] / [adapter.WireLabel]). The filer reads
// it rather than restating it, so the wire spelling a filing carries and the
// spelling the poller parses back cannot drift apart. `help-wanted` carries a
// space on the wire; that drift is handled once, in the adapter.

var urgencyFromArg = map[string]domain.Urgency{
	"":         domain.UrgencyRoutine,
	"routine":  domain.UrgencyRoutine,
	"elevated": domain.UrgencyElevated,
	"urgent":   domain.UrgencyUrgent,
	"gold":     domain.UrgencyGold,
}

// handleEscalate is the desk__escalate handler.
func handleEscalate(ctx context.Context, s *Server, call Call) (any, error) {
	if s.Filer == nil {
		return nil, fmt.Errorf("desk__escalate has no write path configured: deskd was started without an %s write token, "+
			"so it can file nothing — could-not-check, never a silent no-op", AppIdentity)
	}
	var a EscalateArgs
	if err := json.Unmarshal(nonNull(call.Args), &a); err != nil {
		return nil, fmt.Errorf("desk__escalate arguments did not decode: %w", err)
	}
	a.Cell = strings.TrimSpace(a.Cell)
	a.Title = strings.TrimSpace(a.Title)
	a.Situation = strings.TrimSpace(a.Situation)
	if a.Cell == "" {
		return nil, fmt.Errorf("desk__escalate: cell is required")
	}
	if a.Title == "" {
		return nil, fmt.Errorf("desk__escalate: title is required")
	}
	if a.Situation == "" {
		return nil, fmt.Errorf("desk__escalate: situation is required — an escalation with no situation is not actionable")
	}
	kind, ok := adapter.KindForLabel(a.Kind)
	if !ok {
		return nil, fmt.Errorf("desk__escalate: kind %q is not one of needs-decision|question|help-wanted|ask", a.Kind)
	}
	urg, ok := urgencyFromArg[strings.ToLower(strings.TrimSpace(a.Urgency))]
	if !ok {
		return nil, fmt.Errorf("desk__escalate: urgency %q is not one of routine|elevated|urgent|gold", a.Urgency)
	}

	// Options are validated AT FILING (spec §1.1.1): malformed and duplicate
	// ids are rejected here, missing ids fall back to option-1..n.
	opts, err := normalizeOptions(a.Options)
	if err != nil {
		return nil, fmt.Errorf("desk__escalate: %w", err)
	}

	repo := strings.TrimSpace(a.Repo)
	if repo == "" {
		repo = s.defaultRepo(a.Cell)
	}
	if repo == "" {
		return nil, fmt.Errorf("desk__escalate: no repo given and cell %q has no configured repo to file against", a.Cell)
	}

	// Attribution is by CALLER IDENTITY. A caller in no configured loop files
	// an UNATTRIBUTABLE escalation: still filed, but with no collapse key, so
	// it never merges (the adapter cannot prove same-origin).
	automation, attributed := domain.AutomationRef{}, false
	if s.Dir != nil && call.CallerLogin != "" {
		automation, attributed = s.Dir.Lookup(domain.CellName(a.Cell), normaliseCaller(call.CallerLogin))
	}
	rateKey := call.CallerLogin
	if attributed {
		rateKey = automation.String()
	}
	if rateKey == "" {
		rateKey = "anonymous"
	}

	now := s.Now()

	// Flood collapse (§6.6): past a threshold, a loop's escalations collapse
	// into ONE "loop X is escalating abnormally" item. A flood is itself a
	// signal and must not be able to bury the queue it targets.
	if s.rl.observe(rateKey, now) > s.floodCfg.FloodThreshold {
		return s.floodCollapse(ctx, repo, a.Cell, rateKey)
	}

	// Build the escalation shape so the collapse key and merge guard are
	// derived by the SAME code both sides use.
	esc := domain.Escalation{
		Cell:       domain.CellName(a.Cell),
		Automation: automation,
		Kind:       kind,
		Provenance: domain.AgentRaised, // always — never from the caller's arguments
		Urgency:    urg,
		Title:      a.Title,
		Situation:  a.Situation,
		Options:    opts,
		Rev:        domain.OptionsRev(opts),
	}
	collapseKey := esc.Key().String()
	guard := esc.MergeGuard()

	res := EscalateResult{
		Provenance:  string(domain.AgentRaised),
		CollapseKey: collapseKey,
		OptionsRev:  domain.ShortRev(esc.Rev),
		Attributed:  attributed,
	}

	// Filing-time collapse (spec §1.1.3): a filing whose key + guard match an
	// OPEN escalation comments "re-raised ×k" instead of opening a new issue.
	// An unattributable filing has an empty collapse key and never matches.
	if collapseKey != "" {
		open, err := s.Filer.OpenEscalations(ctx, a.Cell)
		if err != nil {
			return nil, fmt.Errorf("desk__escalate: could not read open escalations to collapse against (BLIND, not empty): %w", err)
		}
		if match, found := findCollapseTarget(open, collapseKey, guard); found {
			k := match.Repeat + 1
			if k < 2 {
				k = 2
			}
			if err := s.Filer.CommentReRaise(ctx, match.Ref, fmt.Sprintf("re-raised ×%d", k)); err != nil {
				return nil, fmt.Errorf("desk__escalate: re-raise comment failed: %w", err)
			}
			ref := match.Ref
			res.Outcome, res.Ref, res.Repeat = "merged", &ref, k
			res.Note = "filing-time collapse: matched an open escalation's collapse key and merge guard; no new issue opened (spec §1.1.3)"
			return res, nil
		}
	}

	body := FormatBody(a.Situation, opts)
	labels := escalationLabels(kind, urg)
	ref, err := s.Filer.FileEscalation(ctx, NewEscalation{
		Repo: repo, Cell: a.Cell, Title: a.Title, Body: body, Labels: labels,
	})
	if err != nil {
		return nil, fmt.Errorf("desk__escalate: filing failed: %w", err)
	}
	res.Outcome, res.Ref, res.Repeat = "filed", &ref, 1
	return res, nil
}

// floodCollapse folds a flooding loop's filings into one abnormal-escalation
// item per identity per window.
func (s *Server) floodCollapse(ctx context.Context, repo, cell, identity string) (any, error) {
	title := fmt.Sprintf("loop %s is escalating abnormally", identity)
	if ref, ok := s.rl.flood(identity); ok {
		if err := s.Filer.CommentReRaise(ctx, ref, "re-raised (flood) — this loop is still escalating abnormally"); err != nil {
			return nil, fmt.Errorf("desk__escalate: flood re-raise failed: %w", err)
		}
		return EscalateResult{Outcome: "flood-collapsed", Ref: &ref, Provenance: string(domain.AgentRaised),
			Note: "per-identity flood threshold exceeded — folded onto the standing abnormal-escalation item (§6.6)"}, nil
	}
	body := fmt.Sprintf("`%s` has filed more than %d escalations within %s.\n\n"+
		"A flood is itself a signal (§6.6). Its individual filings are collapsed here so they cannot bury the queue they target. "+
		"Inspect the loop rather than the individual items.", identity, s.floodCfg.FloodThreshold, s.floodCfg.Window)
	ref, err := s.Filer.FileEscalation(ctx, NewEscalation{
		Repo: repo, Cell: cell, Title: title, Body: body, Labels: []string{"needs-decision", "urgent"},
	})
	if err != nil {
		return nil, fmt.Errorf("desk__escalate: could not open the flood-collapse item: %w", err)
	}
	s.rl.recordFlood(identity, ref)
	return EscalateResult{Outcome: "flood-collapsed", Ref: &ref, Provenance: string(domain.AgentRaised),
		Note: "per-identity flood threshold exceeded — opened one abnormal-escalation item; further filings fold onto it (§6.6)"}, nil
}

// findCollapseTarget returns the open escalation a filing folds into: same
// collapse key AND same merge guard, and not itself a flood item.
func findCollapseTarget(open []OpenEscalation, key, guard string) (OpenEscalation, bool) {
	for _, o := range open {
		if o.FloodItem {
			continue
		}
		if o.CollapseKey == key && o.MergeGuard == guard {
			return o, true
		}
	}
	return OpenEscalation{}, false
}

// normalizeOptions validates option ids at filing (spec §1.1.1).
//
//   - No options at all ⇒ nil (an option-less kind; the merge guard becomes the
//     situation content hash, not an OptionsRev).
//   - A supplied id must match the slug grammar [a-z0-9][a-z0-9-]{1,31}.
//   - A missing id falls back to option-<position>.
//   - Duplicate ids (after fallback) are rejected — the recorded answer binds
//     by id, and two identical ids make an answer ambiguous.
//   - Label and consequence are required: the v2 grammar needs one Consequence
//     line per option, and a labelless option is not a choice.
func normalizeOptions(in []OptionInput) ([]domain.Option, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]domain.Option, 0, len(in))
	seen := map[string]bool{}
	for i, o := range in {
		label := strings.TrimSpace(o.Label)
		cons := strings.TrimSpace(o.Consequence)
		if label == "" {
			return nil, fmt.Errorf("option %d has no label — a labelless option is not a choice", i+1)
		}
		if cons == "" {
			return nil, fmt.Errorf("option %d (%q) has no consequence — the v2 grammar requires one Consequence line per option (spec §1.4)", i+1, label)
		}
		id := strings.TrimSpace(o.ID)
		if id == "" {
			id = fmt.Sprintf("option-%d", i+1)
		} else if !domain.OptionID(id).Valid() {
			return nil, fmt.Errorf("option %d id %q is malformed — ids must match the slug grammar [a-z0-9][a-z0-9-]{1,31} (spec §1.1)", i+1, id)
		}
		if seen[id] {
			return nil, fmt.Errorf("duplicate option id %q — ids must be unique within one escalation (spec §1.1.1)", id)
		}
		seen[id] = true
		out = append(out, domain.Option{ID: domain.OptionID(id), Label: label, Consequence: cons})
	}
	return out, nil
}

// FormatBody emits the spec §1.4 wire grammar: the situation prose, then an
// `### Options` section carrying the v2 marker and one heading per option. It
// ROUND-TRIPS — adapter.ParseOptions parses exactly this back — which is what
// makes "no side channel" (spec §1.4) checkable rather than asserted.
//
// An option-less escalation is prose-v1 by construction: the situation with no
// marker, which the adapter reads as Options=nil, exactly today's semantics.
func FormatBody(situation string, opts []domain.Option) string {
	situation = strings.TrimRight(situation, "\n")
	if len(opts) == 0 {
		return situation + "\n"
	}
	var b strings.Builder
	b.WriteString(situation)
	b.WriteString("\n\n### Options\n")
	b.WriteString(optionsMarker())
	b.WriteString("\n")
	for _, o := range opts {
		fmt.Fprintf(&b, "#### `%s` — %s\n", o.ID, o.Label)
		fmt.Fprintf(&b, "Consequence: %s\n\n", o.Consequence)
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// optionsMarker is the v2 grammar marker. It is spelled here to match the
// adapter's OptionsMarker byte-for-byte (the two must agree, spec §1.4); a
// round-trip test asserts the adapter parses what this emits.
func optionsMarker() string { return "<!-- assay:options v2 -->" }

// escalationLabels are the wire labels a filed escalation carries: the kind
// label always, plus an urgency label where the derivation would read one, so
// the index re-derives the urgency this tool set.
func escalationLabels(kind domain.EscalationKind, urg domain.Urgency) []string {
	wire, _ := adapter.WireLabel(kind)
	labels := []string{wire}
	switch urg {
	case domain.UrgencyGold:
		labels = append(labels, "gold")
	case domain.UrgencyUrgent:
		labels = append(labels, "urgent")
	case domain.UrgencyElevated:
		labels = append(labels, "priority:high")
	}
	return labels
}

// normaliseCaller strips the [bot] suffix so a caller login joins against the
// cells.yaml directory, which names Apps without it.
func normaliseCaller(login string) string { return strings.TrimSuffix(login, "[bot]") }

func nonNull(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("{}")
	}
	return raw
}

// rateLimiter tracks per-identity escalation timestamps for flood collapse.
type rateLimiter struct {
	mu        sync.Mutex
	cfg       FloodConfig
	events    map[string][]time.Time
	floodOpen map[string]FiledRef
}

// FloodConfig configures the per-identity flood-collapse control (§6.6).
type FloodConfig struct {
	// Window is the sliding window escalations are counted over.
	Window time.Duration
	// FloodThreshold is the count WITHIN the window past which a loop's
	// filings collapse into one abnormal-escalation item. Strictly-greater:
	// the (threshold+1)-th filing in a window is the first to collapse.
	FloodThreshold int
}

// DefaultFloodConfig is the shipped control: more than 10 escalations from one
// identity in 10 minutes is abnormal.
var DefaultFloodConfig = FloodConfig{Window: 10 * time.Minute, FloodThreshold: 10}

func newRateLimiter(cfg FloodConfig) *rateLimiter {
	return &rateLimiter{cfg: cfg, events: map[string][]time.Time{}, floodOpen: map[string]FiledRef{}}
}

// observe records an escalation and returns the count in the window including
// this one.
func (r *rateLimiter) observe(identity string, now time.Time) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := now.Add(-r.cfg.Window)
	kept := r.events[identity][:0:0]
	for _, t := range r.events[identity] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	kept = append(kept, now)
	r.events[identity] = kept
	return len(kept)
}

func (r *rateLimiter) recordFlood(identity string, ref FiledRef) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.floodOpen[identity] = ref
}

func (r *rateLimiter) flood(identity string) (FiledRef, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ref, ok := r.floodOpen[identity]
	return ref, ok
}
