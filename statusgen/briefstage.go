package main

// briefstage — the deterministic brief-flow stage reducer
// (spec/brief-flow-event-v1.md §"Stage reducer").
//
// Two things are kept apart on purpose:
//
//   - FIRST milestones (the user definitions: written, coded, reviewed, merged,
//     verified, done). Each is set once, by the earliest qualifying fact, and
//     is never moved or cleared by a later episode.
//   - The CURRENT whole-brief stage, recomputed from the live contribution set
//     and the acceptance owner's scope. Completion (verified/done) comes only
//     from a fact whose acceptance coverage is complete — never from the
//     furthest-ahead pull request, and never from a count of pull requests.
//
// Repeats are EPISODES (a ready→draft regression, a pull-request reopen, a
// brief reopen), recorded alongside the firsts rather than overwriting them.

import (
	"sort"
	"time"
)

// Milestone is one first-milestone fact, carrying what was true when it
// occurred (owner and revision at occurrence, not current values).
type Milestone struct {
	FactID          string    `json:"fact_id"`
	OccurredAt      time.Time `json:"occurred_at"`
	Precision       string    `json:"precision,omitempty"`
	SourceAuthority string    `json:"source_authority,omitempty"`
	InitiatorRole   string    `json:"initiator_role"`
	ExecutorRole    string    `json:"executor_role"`
	OwnerCell       string    `json:"owner_cell,omitempty"`
	OwnerStream     string    `json:"owner_stream,omitempty"`
	Contribution    string    `json:"contribution,omitempty"`
	BriefRevision   int       `json:"brief_revision,omitempty"`
	ReadyAtCreation bool      `json:"ready_at_creation,omitempty"`
}

// BriefFirsts holds the first-milestone facts; nil = not (yet) reached.
type BriefFirsts struct {
	Written  *Milestone `json:"written"`
	Coded    *Milestone `json:"coded"`
	Reviewed *Milestone `json:"reviewed"`
	Merged   *Milestone `json:"merged"`
	Verified *Milestone `json:"verified"`
	Done     *Milestone `json:"done"`
}

// Episode is a repeated transition kept separate from the firsts.
type Episode struct {
	Kind         string    `json:"kind"` // ready_to_draft | pr_reopen | brief_reopen
	FactID       string    `json:"fact_id"`
	OccurredAt   time.Time `json:"occurred_at"`
	Contribution string    `json:"contribution,omitempty"`
}

// BriefProjection is one brief's projected flow state.
type BriefProjection struct {
	BriefUUID     string      `json:"brief_uuid"`
	Stage         string      `json:"stage"`
	Births        int         `json:"births"`
	Aliases       []string    `json:"aliases"`
	Firsts        BriefFirsts `json:"firsts"`
	Episodes      []Episode   `json:"episodes"`
	Seeded        bool        `json:"seeded"`
	Observations  int         `json:"observations"`
	LastObserved  string      `json:"last_observed,omitempty"`
	AuthoringPRs  []string    `json:"authoring_prs"`
	Abandoned     []string    `json:"abandoned"`
	OtherMerges   int         `json:"other_merges"`
	UnknownMerges int         `json:"unknown_merges"`
	Unqualified   int         `json:"unqualified"`
	Coverage      string      `json:"coverage"` // worst event coverage seen
}

type stageContrib struct {
	kind     string // implementation | authoring | "" (never seen opened)
	state    string // draft | ready | merged | closed
	prev     string // state before a close, restored on reopen
	replaced bool
	prior    bool // belongs to a delivery round before a brief reopen
}

type stageState struct {
	contribs   map[string]*stageContrib
	order      []string
	scope      map[string]bool
	scopeCov   string
	verified   bool
	done       bool
	reopened   bool
	hasWritten bool
}

var coverageRank = map[string]int{"observed": 0, "partial": 1, "unknown": 2}

func asMilestone(ev *BriefEvent) *Milestone {
	return &Milestone{
		FactID:          ev.FactID,
		OccurredAt:      ev.OccurredAt,
		Precision:       ev.Precision,
		SourceAuthority: ev.SourceAuthority,
		InitiatorRole:   ev.InitiatorRole,
		ExecutorRole:    ev.ExecutorRole,
		OwnerCell:       ev.OwnerCell,
		OwnerStream:     ev.OwnerStream,
		Contribution:    ev.ContribRef,
		BriefRevision:   ev.BriefRevision,
	}
}

// ProjectBriefs reduces effective facts (EventStore.Effective) to one
// projection per brief uuid. Input order does not matter: facts are replayed
// by occurrence, then fact id.
func ProjectBriefs(evs []*BriefEvent) map[string]*BriefProjection {
	sorted := make([]*BriefEvent, len(evs))
	copy(sorted, evs)
	sortEvents(sorted)

	out := map[string]*BriefProjection{}
	states := map[string]*stageState{}
	for _, ev := range sorted {
		key := ev.BriefUUID // identity-key
		p, ok := out[key]
		if !ok {
			p = &BriefProjection{BriefUUID: ev.BriefUUID, Coverage: "observed",
				Aliases: []string{}, Episodes: []Episode{}, AuthoringPRs: []string{}, Abandoned: []string{}}
			out[key] = p
			states[key] = &stageState{contribs: map[string]*stageContrib{}}
		}
		st := states[key]
		if coverageRank[ev.Coverage] > coverageRank[p.Coverage] {
			p.Coverage = ev.Coverage
		}
		// Alias history: the first alias seen, then each alias_assigned
		// change. An alias is recorded, never used as the key.
		switch n := len(p.Aliases); {
		case n == 0:
			p.Aliases = append(p.Aliases, ev.Alias)
		case ev.Milestone == "alias_assigned" && p.Aliases[n-1] != ev.Alias:
			p.Aliases = append(p.Aliases, ev.Alias)
		}
		applyFact(p, st, ev)
	}
	for key, p := range out {
		p.Stage = stageOf(states[key], p)
	}
	return out
}

func contribOf(st *stageState, ref string) *stageContrib {
	c, ok := st.contribs[ref]
	if !ok {
		c = &stageContrib{}
		st.contribs[ref] = c
		st.order = append(st.order, ref)
	}
	return c
}

func applyFact(p *BriefProjection, st *stageState, ev *BriefEvent) {
	switch ev.Milestone {
	case "written":
		p.Births++
		st.hasWritten = true
		if p.Firsts.Written == nil {
			p.Firsts.Written = asMilestone(ev)
		}
	case "status_observed":
		p.Observations++
		p.LastObserved = ev.StatusTo
		if ev.StatusFrom == "" {
			p.Seeded = true // seed is not a birth
		}
	case "pr_opened":
		c := contribOf(st, ev.ContribRef)
		c.kind = ev.ContribKind
		if c.kind == "authoring" {
			p.AuthoringPRs = append(p.AuthoringPRs, ev.ContribRef)
			return
		}
		c.state = "ready"
		if ev.ContribDraft {
			c.state = "draft"
		}
		if ev.ContribReplaces != "" {
			contribOf(st, ev.ContribReplaces).replaced = true
		}
		if p.Firsts.Coded == nil {
			p.Firsts.Coded = asMilestone(ev)
		}
		if !ev.ContribDraft && p.Firsts.Reviewed == nil {
			m := asMilestone(ev)
			m.ReadyAtCreation = true
			p.Firsts.Reviewed = m
		}
	case "pr_ready", "pr_draft", "pr_merged", "pr_closed", "pr_reopened":
		c := contribOf(st, ev.ContribRef)
		if c.kind == "authoring" {
			return
		}
		applyContrib(p, c, ev)
	case "acceptance_scope":
		st.scope = map[string]bool{}
		for _, ref := range ev.AcceptContribs {
			st.scope[ref] = true
		}
		st.scopeCov = ev.AcceptCoverage
	case "verified", "done":
		if ev.AcceptCoverage != "complete" {
			p.Unqualified++
			return
		}
		if ev.Milestone == "verified" {
			st.verified = true
			if p.Firsts.Verified == nil {
				p.Firsts.Verified = asMilestone(ev)
			}
		} else {
			st.done = true
			if p.Firsts.Done == nil {
				p.Firsts.Done = asMilestone(ev)
			}
		}
	case "reopened":
		p.Episodes = append(p.Episodes, Episode{Kind: "brief_reopen", FactID: ev.FactID, OccurredAt: ev.OccurredAt})
		st.verified, st.done, st.reopened = false, false, true
		st.scope, st.scopeCov = nil, ""
		for _, c := range st.contribs {
			c.prior = true
		}
	}
}

func applyContrib(p *BriefProjection, c *stageContrib, ev *BriefEvent) {
	switch ev.Milestone {
	case "pr_ready":
		c.state = "ready"
		if c.kind == "implementation" && p.Firsts.Reviewed == nil {
			p.Firsts.Reviewed = asMilestone(ev)
		}
	case "pr_draft":
		if c.state == "ready" {
			p.Episodes = append(p.Episodes, Episode{Kind: "ready_to_draft", FactID: ev.FactID, OccurredAt: ev.OccurredAt, Contribution: ev.ContribRef})
		}
		c.state = "draft"
	case "pr_merged":
		c.state = "merged"
		if c.kind != "implementation" {
			return
		}
		switch ev.ExecutorRole {
		case "human":
			if p.Firsts.Merged == nil {
				p.Firsts.Merged = asMilestone(ev)
			}
		case "unknown", "": // merge-executor-unknown: not an app merge either
			p.UnknownMerges++
		default:
			p.OtherMerges++
		}
	case "pr_closed":
		if c.state != "merged" {
			c.prev, c.state = c.state, "closed"
		}
	case "pr_reopened":
		if c.state == "closed" {
			c.state = c.prev
			if c.state == "" {
				c.state = "draft"
			}
			p.Episodes = append(p.Episodes, Episode{Kind: "pr_reopen", FactID: ev.FactID, OccurredAt: ev.OccurredAt, Contribution: ev.ContribRef})
		}
	}
}

// stageOf computes the current whole-brief stage. See the spec for the table;
// every branch that cannot be decided from the facts is explicit mixed or
// unknown, never a guess toward completion.
func stageOf(st *stageState, p *BriefProjection) string {
	if st.done {
		return "done"
	}
	if st.verified {
		return "verified"
	}
	var live []string
	var abandoned []string
	unknownKind := false
	for _, ref := range st.order {
		c := st.contribs[ref]
		if c.prior || c.kind == "authoring" {
			continue
		}
		if c.kind == "" {
			unknownKind = true // a PR fact without its opening: coverage gap
			continue
		}
		if c.replaced {
			continue
		}
		if c.state == "closed" {
			abandoned = append(abandoned, ref)
			continue
		}
		live = append(live, ref)
	}
	sort.Strings(abandoned)
	p.Abandoned = append([]string{}, abandoned...)
	if unknownKind {
		return "unknown"
	}
	if len(live) == 0 {
		switch {
		case len(abandoned) > 0:
			return "unknown"
		case st.reopened:
			return "reopened"
		case st.hasWritten:
			return "written"
		}
		return "unknown"
	}
	if len(abandoned) > 0 {
		return "mixed"
	}
	counts := map[string]int{}
	for _, ref := range live {
		counts[st.contribs[ref].state]++
	}
	merged := counts["merged"]
	if st.scope != nil {
		if st.scopeCov != "complete" {
			return "unknown"
		}
		if len(st.scope) != len(live) {
			return "mixed"
		}
		for _, ref := range live {
			if !st.scope[ref] {
				return "mixed"
			}
		}
		switch {
		case merged == len(live): // partial-merge guard
			return "merged"
		case counts["draft"] == len(live):
			return "coded"
		case counts["ready"] == len(live):
			return "reviewed"
		}
		return "mixed"
	}
	if len(live) > 1 {
		return "mixed"
	}
	switch st.contribs[live[0]].state {
	case "draft":
		return "coded"
	case "ready":
		return "reviewed"
	case "merged":
		return "merged"
	}
	return "unknown"
}
