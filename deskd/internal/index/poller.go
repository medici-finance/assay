package index

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/domain"
)

// Publisher is the narrow `/v1/events` producer interface [Poller.PollOnce]
// publishes through (the service contract increment 3, task 2). [serve.EventBus]
// satisfies it structurally; this package does not import internal/serve, so
// the dependency keeps pointing the one way it already does (serve depends on
// index, never the reverse). A nil Events (the Poller zero value) makes every
// publish call a no-op — a build with no SSE producer wired polls exactly as
// it always has.
type Publisher interface {
	Publish(typ contract.EventType, data any)
}

// DefaultInterval is the poll interval (design §5).
//
// The rate budget behind it, stated so nobody has to re-derive it: ~5 calls
// per repo per cycle × 10 repos × 60 cycles/hr ≈ 3,000 req/hr against a
// GitHub App installation floor of 5,000/hr — and conditional requests
// answered 304 cost nothing, so steady state is near zero. Headroom to ~15
// repos before webhooks become worth it.
const DefaultInterval = 60 * time.Second

// Fetcher is the GitHub read surface the poller needs. It is an interface so
// a test can supply a source that FAILS — which is the only way to prove the
// index renders could-not-check rather than an empty board.
type Fetcher interface {
	FetchEscalations(ctx context.Context, repo adapter.Repo) adapter.FetchResult
}

// CycleRefresher is implemented by a [Fetcher] whose read tokens must be
// re-resolved at the top of EVERY poll cycle (the service contract) rather than
// once at process start — [adapter.ReloadingRouter], for a hosted deskd whose
// per-org tokens rotate in-process via a sidecar-shared file. [PollOnce] calls
// RefreshCycle, when the configured Fetcher implements this, before touching a
// single repo, so a token that rotated between cycles is visible on the very
// next cycle with NO deskd restart.
//
// A Fetcher built once at start (the plain [adapter.RoutingFetcher] /
// [adapter.NewGitHubRouter]) does not implement this interface, so PollOnce
// calls nothing extra for it and it behaves exactly as it always has (Verify
// row 6, TestEnvTokenSourceUnchanged).
type CycleRefresher interface {
	RefreshCycle(ctx context.Context)
}

// Directory implements adapter.AutomationDirectory over the cell config.
type Directory struct {
	byCell map[domain.CellName]map[string]domain.Role
}

// NewDirectory builds the login → loop join table from cells.yaml.
//
// This is the ONLY source for the automation join. A filing identity that is
// not in here does not get attributed by some other route — it becomes an
// UNRESOLVED row. One declared source per fact; a second one would only make
// disagreement possible without saying who wins.
func NewDirectory(cfg *schema.File) *Directory {
	d := &Directory{byCell: map[domain.CellName]map[string]domain.Role{}}
	if cfg == nil {
		return d
	}
	for _, c := range cfg.Cells {
		m := map[string]domain.Role{}
		for _, l := range c.Loops {
			if l.App != "" {
				m[strings.TrimSuffix(l.App, "[bot]")] = domain.Role(l.Role)
			}
		}
		d.byCell[domain.CellName(c.Name)] = m
	}
	return d
}

// Lookup resolves a login within a cell.
func (d *Directory) Lookup(cell domain.CellName, login string) (domain.AutomationRef, bool) {
	m, ok := d.byCell[cell]
	if !ok {
		return domain.AutomationRef{}, false
	}
	role, ok := m[login]
	if !ok {
		return domain.AutomationRef{}, false
	}
	return domain.AutomationRef{Cell: cell, Role: role}, true
}

// Known lists the logins a cell can resolve.
func (d *Directory) Known(cell domain.CellName) []string {
	var out []string
	for login := range d.byCell[cell] {
		out = append(out, login)
	}
	sort.Strings(out)
	return out
}

// Poller drives one cycle per interval over every repo of every cell.
type Poller struct {
	DB       *DB
	Fetcher  Fetcher
	Config   *schema.File
	Dir      *Directory
	Interval time.Duration
	// Events is the service contract increment 3 producer. Nil (the default)
	// wires no event stream — see [Publisher].
	Events Publisher
	// Answers returns a cell's poll-side answer-record verifier
	// (the authorization contract gap 3, item (5)): the enrolled keys and the cell's
	// current answer_as. Nil — or a verifier with no keys — trusts no
	// bot-carried answer record: each derives answered-unverified.
	Answers func(cell domain.CellName) adapter.AnswerVerifier
}

// answersFor is the cell's verifier, or the zero (trust-nothing) one.
func (p *Poller) answersFor(cell domain.CellName) adapter.AnswerVerifier {
	if p.Answers == nil {
		return adapter.AnswerVerifier{}
	}
	return p.Answers(cell)
}

// CycleResult is what one poll cycle did, in three-state terms.
type CycleResult struct {
	Started time.Time `json:"started"`
	Ended   time.Time `json:"ended"`
	// Reached and Unreachable count repos, not issues. Both are
	// domain.Count so neither can be published without its probe.
	Reached     domain.Count         `json:"repos_reached"`
	Unreachable domain.Count         `json:"repos_could_not_check"`
	Health      []adapter.RepoHealth `json:"repo_health"`
	// Outcome is "ok" only when EVERY repo came back clean. A cycle in
	// which one repo was unreachable is not an ok cycle with fewer rows —
	// it is a degraded cycle, and the index age reflects that.
	Outcome string `json:"outcome"`
}

// PollOnce runs one cycle.
//
// The invariant this function exists to hold: a repo that could not be
// reached leaves its PREVIOUS rows in place and records a could-not-check
// health verdict. It never clears the repo's rows, because an empty result
// from an unreachable source is BLIND, not idle — and a board that turns
// blindness into an empty list has told its operator that there is nothing
// to do.
func (p *Poller) PollOnce(ctx context.Context) CycleResult {
	// the service contract: re-resolve this cycle's read tokens FIRST, before a
	// single repo is fetched, when the configured Fetcher is reloadable. A
	// Fetcher built once at start (the ordinary case) does not implement
	// CycleRefresher and this is a no-op — see [CycleRefresher].
	if rf, ok := p.Fetcher.(CycleRefresher); ok {
		rf.RefreshCycle(ctx)
	}

	started := time.Now().UTC()
	res := CycleResult{Started: started}

	// Increment 3's producer snapshot: taken BEFORE this cycle touches
	// anything, so it is exactly the state the PREVIOUS cycle (or cold start)
	// left behind. could-not-check on the read is treated as "no prior
	// snapshot" (nil maps), which publishes every row as a fresh appearance
	// rather than fabricating a diff against data this call never actually
	// read.
	var beforeStates map[domain.EscalationID]string
	var beforeBadge contract.BadgeData
	var beforeHealth []string
	if p.Events != nil {
		if rows, err := p.DB.Inbox("", true); err == nil {
			beforeStates = escalationStates(rows)
			beforeBadge = badgeCounts(rows)
		}
		if h, err := p.DB.Health(); err == nil {
			beforeHealth = couldNotCheckRepos(h)
		}
	}

	cycleProbe := domain.Probe{
		Source: "cells.yaml → repos[] × internal/adapter.FetchEscalations",
		Method: domain.MethodLocalTable,
		Window: "every repo of every configured cell, one cycle",
		Limit:  0,
	}

	reached, unreachable := 0, 0
	for _, cell := range p.Config.Cells {
		for _, slug := range cell.Repos {
			repo, err := adapter.ParseRepo(slug)
			if err != nil {
				unreachable++
				h := adapter.RepoHealth{Repo: slug, CheckedAt: time.Now().UTC(),
					Assessment: domain.Unchecked(cycleProbe, "config names a repo that is not owner/name: "+err.Error())}
				res.Health = append(res.Health, h)
				_ = p.DB.RecordHealth(h)
				continue
			}

			fr := p.Fetcher.FetchEscalations(ctx, repo)
			res.Health = append(res.Health, fr.Health)
			_ = p.DB.RecordHealth(fr.Health)

			if fr.Health.Assessment.Confidence != domain.CheckedClean {
				unreachable++
				// Deliberately NOT clearing this repo's rows. See the
				// function comment.
				continue
			}
			reached++

			if fr.Health.NotModified {
				// 304: the set is unchanged, the stored rows are current.
				continue
			}

			escs := make([]domain.Escalation, 0, len(fr.Issues))
			raw := map[domain.EscalationID]adapter.RawIssue{}
			for _, iss := range fr.Issues {
				e := adapter.MapEscalation(domain.CellName(cell.Name), repo, iss, p.Dir, p.DB.MintID, p.answersFor(domain.CellName(cell.Name)))
				escs = append(escs, e)
				raw[e.ID] = iss
			}
			if err := p.DB.UpsertEscalations(repo, escs, raw); err != nil {
				unreachable++
				reached--
				h := adapter.RepoHealth{Repo: repo.String(), CheckedAt: time.Now().UTC(),
					Assessment: domain.Unchecked(cycleProbe, "rows were derived but not stored: "+err.Error())}
				res.Health = append(res.Health, h)
				_ = p.DB.RecordHealth(h)
			}
		}
	}

	res.Reached = domain.NewCount(reached, cycleProbe)
	res.Unreachable = domain.NewCount(unreachable, cycleProbe)
	res.Ended = time.Now().UTC()
	res.Outcome = "ok"
	if unreachable > 0 {
		res.Outcome = fmt.Sprintf("%d of %d repo slice(s) could-not-check", unreachable, reached+unreachable)
	}
	_ = p.DB.MarkPoll(res.Started, res.Ended, res.Outcome)

	if p.Events != nil {
		p.publishTransitions(beforeStates, beforeBadge, beforeHealth, res.Ended)
	}
	return res
}

// publishTransitions is increment 3's producer half: one event per STATE
// TRANSITION, never per poll (facts: "the service contract dedup rule moves to
// the producer") — every comparison below is a before/after diff, and NOTHING
// publishes when nothing changed. A could-not-check "before" (nil map, this
// cycle's own read failed) is read as "no prior snapshot": every row reads as
// a fresh appearance rather than a fabricated diff, the badge publishes
// against the zero-value baseline, and the health set against the empty one.
// No nil check is needed for that: reading and ranging a nil map behave
// exactly like an empty map, so a nil "before" already yields existed ==
// false for every "after" row. Gating on before != nil is the ONE thing that
// would break the promise — it would publish nothing for the cycle, and a
// transition that opened and closed inside it would be dropped for good, not
// delayed, because the next cycle's before-read sees the same new state.
func (p *Poller) publishTransitions(before map[domain.EscalationID]string, beforeBadge contract.BadgeData, beforeHealth []string, at time.Time) {
	rows, err := p.DB.Inbox("", true)
	if err != nil {
		// The AFTER read failed: publishing a diff against data this call
		// never actually observed would be a fabricated transition, so this
		// cycle publishes nothing rather than guess. The next successful
		// cycle's "before" is this same (unpublished) state, so no transition
		// is lost — only delayed to the read that can actually see it.
		return
	}

	after := escalationStates(rows)
	for id, to := range after {
		from, existed := before[id]
		if existed && from == to {
			continue
		}
		p.Events.Publish(contract.EventTransition, contract.TransitionData{ID: string(id), From: from, To: to, At: at})
	}

	afterBadge := badgeCounts(rows)
	if afterBadge != beforeBadge {
		p.Events.Publish(contract.EventBadge, afterBadge)
	}

	if h, err := p.DB.Health(); err == nil {
		afterHealth := couldNotCheckRepos(h)
		if !sameStringSet(beforeHealth, afterHealth) {
			p.Events.Publish(contract.EventIndexHealth, contract.IndexHealthData{CouldNotCheck: afterHealth})
		}
	}
}

// escalationStates is the per-escalation signal [Poller.publishTransitions]
// diffs across cycles. "closed" and "open" are always derivable in this
// build; the richer working/waiting/idle attention states are NOT reachable
// on the live inbox surface until the artifact/pod substrate lands
// ([attentionFor]'s own doc comment states the same limitation) — so an open,
// attributable row's state is its rendered [domain.AttentionState] where one
// derives, and the bare "open" fallback otherwise (an unattributable row, or
// one this build's attention derivation cannot yet reach).
func escalationStates(rows []Row) map[domain.EscalationID]string {
	out := make(map[domain.EscalationID]string, len(rows))
	for _, r := range rows {
		switch {
		case r.Closed:
			out[r.ID] = "closed"
		case r.Attention != nil:
			out[r.ID] = string(r.Attention.State)
		default:
			out[r.ID] = "open"
		}
	}
	return out
}

// badgeCounts is the `badge` event payload: open/urgent/gold counts over the
// OPEN rows only — a closed escalation is decided and no longer badges.
// BadgeCounts is the badge payload over a row set. The poller publishes it
// over every row; the event stream recomputes it over the rows one viewer may
// read, so both use this one count.
func BadgeCounts(rows []Row) contract.BadgeData { return badgeCounts(rows) }

func badgeCounts(rows []Row) contract.BadgeData {
	var b contract.BadgeData
	for _, r := range rows {
		if r.Closed {
			continue
		}
		b.Open++
		switch r.Urgency {
		case domain.UrgencyUrgent:
			b.Urgent++
		case domain.UrgencyGold:
			b.Gold++
		}
	}
	return b
}

// couldNotCheckRepos is the `index.health` event payload: the sorted set of
// repos NOT at checked-clean confidence.
func couldNotCheckRepos(health []adapter.RepoHealth) []string {
	var out []string
	for _, h := range health {
		if h.Assessment.Confidence != domain.CheckedClean {
			out = append(out, h.Repo)
		}
	}
	sort.Strings(out)
	return out
}

// sameStringSet compares two already-sorted string slices for equality. nil
// and an empty slice compare equal (both mean "nothing could-not-check").
func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Run polls until ctx is cancelled. The first cycle runs immediately so a
// cold start does not serve an empty board for a whole interval.
func (p *Poller) Run(ctx context.Context) {
	interval := p.Interval
	if interval <= 0 {
		interval = DefaultInterval
	}
	p.PollOnce(ctx)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p.PollOnce(ctx)
		}
	}
}
