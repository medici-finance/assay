// Package fleet builds the spatial fleet map (the service contract): a
// read-only, glance-shaped view over dispatch claims (refs/dispatch/*) and
// open PRs, grouped by repo lane. It is a RENDERING of state that already
// exists elsewhere — refs/dispatch/*, GitHub's PR list — and derives
// nothing that outlives one request; it stores no second copy of truth
// (brief §Context: "(b) the map reads the same sources the sweep reports
// read (no second source of truth)").
//
// The single-point-of-failure named in the brief is a killed fetch quietly
// rendering as fresh, so staleness is the load-bearing property of this
// package: every [RepoLane] carries the age of the DATA it holds, and
// [Cache] never lets a failed fetch overwrite a good one with silence — it
// falls back to the last good snapshot and marks it Stale, or reports
// could-not-check when there has never been a good one.
package fleet

import "time"

// State is one of the four fleet-token states the brief's design record
// fixes (brief §"Design record for the sibling PR"), reusing the console's
// existing four-bucket render palette (web/panes.go paletteFor) rather than
// inventing a fifth:
//
//	working        → healthy
//	awaiting-review → neutral
//	conflicting    → fault
//	stale-claim    → amber   (amber-as-a-question, same convention as Silent)
type State string

const (
	StateWorking        State = "working"
	StateAwaitingReview State = "awaiting-review"
	StateConflicting    State = "conflicting"
	StaleClaimState     State = "stale-claim"
)

// Palette maps a fleet State to the console's render palette token
// (web/static/app.css `.pal-healthy`/`.pal-neutral`/`.pal-amber`/`.pal-fault`).
func (s State) Palette() string {
	switch s {
	case StateWorking:
		return "healthy"
	case StateAwaitingReview:
		return "neutral"
	case StateConflicting:
		return "fault"
	case StaleClaimState:
		return "amber"
	}
	return "amber" // unknown state renders as a question, never a false green
}

// Kind distinguishes the two token shapes a lane can hold.
type Kind string

const (
	KindClaim Kind = "claim"
	KindPR    Kind = "pr"
)

// Token is one located, state-colored fleet element (talk 41's presence
// shape: where is each agent/PR, what state is it in).
type Token struct {
	Kind  Kind
	Key   string // claim ref short name, or "#<number>"
	Title string // PR title; empty for a claim
	// Holder is who holds the claim, or who authored the PR.
	Holder string
	// Age is how long the underlying thing (the claim, or the PR since its
	// last update) has existed — this is DATA age, distinct from the
	// lane's FETCH age below. A long-held claim is a stale-claim on its
	// own terms even when the fetch that reported it was one second ago.
	Age time.Duration
	// State is the four-bucket classification (see [State]).
	State State
}

// Palette is the render palette token for this token's state.
func (t Token) Palette() string { return t.State.Palette() }

// RepoLane is one configured repo's column on the map.
type RepoLane struct {
	Repo string // "owner/name"

	// OK is the three-state read verdict for THIS lane (C4: could-not-check
	// is never rounded to a pass or a fail). OK=false always carries
	// Reason; Tokens is nil in that case unless Stale is also true (a
	// failed live read served from the last-good cache).
	OK     bool
	Reason string

	Tokens []Token

	// FetchedAt is when the DATA this lane holds was last successfully
	// fetched — never "now", even when this call's own live fetch just
	// failed and the lane is serving a cached snapshot. This is the field
	// the single-point-of-failure design leans on: a page that always
	// stamped "now" here would render a killed fetch as fresh, which the
	// brief names as worse than no map at all.
	FetchedAt time.Time

	// Stale is set when FetchAge exceeds the lane's threshold (see
	// [StaleThreshold]) OR when this snapshot was served from the
	// last-good cache after a live-fetch failure — a token can be
	// individually state-stale (stale-claim) while the LANE itself is
	// fetch-stale; the two are orthogonal dimensions by design (brief
	// §"Design record": "Staleness is a SEPARATE overlay dimension...
	// never folded into the 4 state colors").
	Stale bool
}

// FetchAge reports how long ago this lane's data was fetched, as of now.
func (l RepoLane) FetchAge(now time.Time) time.Duration {
	if l.FetchedAt.IsZero() {
		return 0
	}
	return now.Sub(l.FetchedAt)
}

// StaleThresholdMultiplier is the named constant the brief's design record
// calls for: "fetch age > 2× the source's own poll cadence... declared as
// a named constant next to the cadence it reads, never a bare wall-clock
// number." The cadence itself is [Cache.pollInterval], passed in by the
// caller (deskd's own --interval, the same cadence the escalation poller
// uses) — this file names only the MULTIPLIER, never a duration.
const StaleThresholdMultiplier = 2

// StaleThreshold is the fetch-age threshold past which a lane's DATA
// (distinct from any individual token's stale-claim state) renders stale,
// for a given poll cadence.
func StaleThreshold(pollInterval time.Duration) time.Duration {
	return StaleThresholdMultiplier * pollInterval
}

// ClaimStaleThresholdMultiplier is the analogous named constant for an
// individual CLAIM's own age (not the fetch age): a claim held longer than
// this many lease renewal windows is classified stale-claim rather than
// working, independent of whether the fetch that reported it was fresh.
// dispatch-claim leases in this house renew on a short cadence, so a claim
// this old was either abandoned by a dead holder or never renewed —
// either way it is the "stuck claim" the map exists to surface (brief
// §why).
const ClaimStaleThresholdMultiplier = 2

// ClaimStaleThreshold applies the multiplier to a lease TTL.
func ClaimStaleThreshold(leaseTTL time.Duration) time.Duration {
	return ClaimStaleThresholdMultiplier * leaseTTL
}
