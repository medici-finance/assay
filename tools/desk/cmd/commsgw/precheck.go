package main

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// precheck.go — the deterministic inbound pre-check pipeline. Every accepted
// message crosses ALL of these, in this order, whether it arrived cross-cell
// (a2a.go) or within-cell (socket.go) — ONE enforcement point. Each stage
// returns a DISTINCT typed refusal so the refusal battery, the daily sweep and
// an incident review can tell one failure mode from another; none of them is
// ever collapsed into a bare "rejected".
//
// NO DETERMINISTIC ROUTING (#1767 ruling 3): PreCheck answers accept-or-refuse
// only. What an accepted message DOES next is commsloop's job (loop.go), via
// the prose router once it lands — until then, every accepted message
// quarantines, which is fail-closed by construction: PreCheck never decides
// delivery, so there is no routing decision here to get wrong.
var (
	// ErrPeerUnauthenticated — the connection presented no verified peer identity
	// (no client certificate under mTLS, or the presented certificate did not
	// chain to the house trust store). Checked FIRST: a message from an
	// unauthenticated peer is refused before a single byte of it is parsed.
	ErrPeerUnauthenticated = errors.New("commsgw: peer connection is not authenticated (mTLS peer auth failed)")
	// ErrEnvelopeParse — the raw bytes do not parse as a well-formed cellmsg-v1
	// envelope. Wraps the specific comms.Err* (oversize / malformed / bad schema
	// / absent triple / unknown verb or class) — see comms.ParseEnvelope.
	ErrEnvelopeParse = errors.New("commsgw: envelope does not parse")
	// ErrAssertionInvalid — the envelope's signed assertion failed
	// verify-or-refuse. Wraps the specific comms identity error (unknown cell /
	// bad signature / expired / not-yet-valid / replayed / identity mismatch) —
	// see comms.VerifyEnvelope.
	ErrAssertionInvalid = errors.New("commsgw: signed assertion failed verification")
	// ErrLaneDenied — a within-cell (fromCell == toCell) message whose
	// (fromRole, verb, toRole) is not in the compiled lane ACL.
	ErrLaneDenied = errors.New("commsgw: lane ACL denies this within-cell message")
	// ErrCrossCellPair — a cross-cell message whose (fromRole, toRole) pair is
	// not the-desk<->the-desk. DISTINCT from ErrCrossCellVerb (Verify row 12):
	// an out-of-pair message and an out-of-verb message are different incidents.
	ErrCrossCellPair = errors.New("commsgw: cross-cell pair is not the-desk<->the-desk")
	// ErrCrossCellVerb — a cross-cell message whose pair is legal but whose verb
	// is outside the ruled four-verb allow-set (#1896: status, metrics,
	// help-offered, focus-on). DISTINCT from ErrCrossCellPair.
	ErrCrossCellVerb = errors.New("commsgw: verb is not in the cross-cell allow-set")
	// ErrDuplicateMessage — this message id is already claimed: either a live
	// in-flight duplicate (concurrent submit) or a replay of an id already
	// landed. The lineage lock (claimDedupe) is deskkit.Acquire, the same
	// double-dispatch-proof primitive loopengine uses — never a soft
	// "assume free".
	ErrDuplicateMessage = errors.New("commsgw: message id already claimed (duplicate)")
	// ErrBudgetExhausted — this sender (cell/role) is over its rate/budget.
	ErrBudgetExhausted = errors.New("commsgw: sender rate/budget exhausted")
	// ErrKillSwitch — the desk kill switch (deskkit.Guard) is armed. Checked
	// LAST, deliberately: every prior stage is a property of THIS message, so a
	// malformed/forged/out-of-lane message is refused on ITS OWN merits even
	// while the kill switch is armed, rather than the kill switch masking what
	// specifically was wrong with it.
	ErrKillSwitch = errors.New("commsgw: kill switch armed")
)

// PreCheckInput is the per-message input PreCheck evaluates. PeerAuthenticated
// is supplied by the transport (a2a.go's mTLS listener, socket.go's loopback
// listener) — this package never re-derives peer identity itself, so the
// pipeline is testable independent of any real TLS handshake or Unix socket.
type PreCheckInput struct {
	PeerAuthenticated bool
	Raw               []byte
	Now               time.Time
}

// PreCheckDeps is everything PreCheck consults besides the message itself.
// Every field is REQUIRED for a real gateway (see NewPreCheckDeps, which is the
// gateway's one construction site and the site Verify row 8/8a pin); a nil/zero
// field here fails a specific stage closed rather than skipping it silently —
// see the nil guards in PreCheck.
type PreCheckDeps struct {
	Trust       comms.TrustStore
	Skew        time.Duration
	ReplayGuard comms.ReplayGuard
	ACL         *comms.ACL

	ClaimConfig deskkit.ClaimConfig
	Owner       string
	// Acquire is the claim primitive PreCheck dedupes through. nil defaults to
	// deskkit.Acquire; a test substitutes a fake to drive the collision branch
	// without a real claims directory race.
	Acquire func(deskkit.ClaimConfig, deskkit.Claim) (bool, error)

	RateLimiter *RateLimiter

	// GuardFn is the kill-switch check. nil defaults to deskkit.Guard; a test
	// substitutes a fake so "STOP flag halts mid-drain" is provable without
	// touching the real ~/.config/assay state directory.
	GuardFn func() error
}

// PreCheck runs the deterministic pipeline in the frozen order:
// mTLS peer accept -> envelope parse-or-refuse -> signed-assertion verify ->
// lane ACL (incl. cross-cell pair + verb allow-set) -> Claim() dedupe -> per-
// sender rate/budget -> kill switch. It returns the parsed, authenticated
// Envelope on ACCEPT, or the first stage's distinct typed refusal — PreCheck
// never runs a later stage once an earlier one has refused, so a caller reading
// the returned error sees exactly one cause.
func PreCheck(in PreCheckInput, deps PreCheckDeps) (*comms.Envelope, error) {
	if !in.PeerAuthenticated {
		return nil, ErrPeerUnauthenticated
	}

	env, err := comms.ParseEnvelope(in.Raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEnvelopeParse, err)
	}

	if err := comms.VerifyEnvelope(env, in.Now, deps.Trust, deps.Skew, deps.ReplayGuard); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrAssertionInvalid, err)
	}

	if err := checkLane(env, deps.ACL); err != nil {
		return nil, err
	}

	if err := claimDedupe(env, deps); err != nil {
		return nil, err
	}

	if deps.RateLimiter != nil {
		if !deps.RateLimiter.Allow(env.From.Cell+"/"+env.From.Role, in.Now) {
			return nil, fmt.Errorf("%w: %s/%s", ErrBudgetExhausted, env.From.Cell, env.From.Role)
		}
	}

	guard := deps.GuardFn
	if guard == nil {
		guard = deskkit.Guard
	}
	if err := guard(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrKillSwitch, err)
	}

	return env, nil
}

// checkLane is the lane-ACL stage. Cross-cell carries TWO distinct refusals
// (ErrCrossCellPair, ErrCrossCellVerb — Verify rows 11/12) so an out-of-pair
// message is never confused with an in-pair, out-of-verb one in an audit trail.
func checkLane(env *comms.Envelope, acl *comms.ACL) error {
	if acl == nil {
		return fmt.Errorf("%w: nil lane ACL (fail closed, never default-allow)", ErrLaneDenied)
	}
	if env.From.Cell == env.To.Cell {
		if !acl.Allow(env.From.Cell, env.From.Role, env.Verb, env.To.Cell, env.To.Role) {
			return fmt.Errorf("%w: (%s -> %s) verb %q", ErrLaneDenied, env.From.Role, env.To.Role, env.Verb)
		}
		return nil
	}

	pairOK := false
	for _, p := range acl.CrossPairs {
		if p.From == env.From.Role && p.To == env.To.Role {
			pairOK = true
			break
		}
	}
	if !pairOK {
		return fmt.Errorf("%w: (%s -> %s)", ErrCrossCellPair, env.From.Role, env.To.Role)
	}

	verbOK := false
	for _, v := range acl.CrossVerbs {
		if v == env.Verb {
			verbOK = true
			break
		}
	}
	if !verbOK {
		return fmt.Errorf("%w: %q (allow-set: %v)", ErrCrossCellVerb, env.Verb, acl.CrossVerbs)
	}
	return nil
}

// claimDedupe is the lineage-lock stage: a message id is claimed exactly once,
// through deskkit's double-dispatch-proof Acquire (never a soft "assume free").
// A LIVE collision (already claimed, not stale) is ErrDuplicateMessage — the
// caller's own message replayed, or a concurrent duplicate submit. An
// unreadable/unlockable claims directory is NOT treated as "unclaimed": it is
// propagated as-is (deskkit.Unverifiable, fail closed).
func claimDedupe(env *comms.Envelope, deps PreCheckDeps) error {
	acquire := deps.Acquire
	if acquire == nil {
		acquire = deskkit.Acquire
	}
	owner := deps.Owner
	if owner == "" {
		owner = "commsgw"
	}
	acquired, err := acquire(deps.ClaimConfig, deskkit.Claim{
		Kind:  deskkit.KindRoute,
		Item:  "commsmsg/" + env.ID,
		Owner: owner,
	})
	if err != nil {
		return fmt.Errorf("commsgw: message claim unverifiable: %w", err)
	}
	if !acquired {
		return fmt.Errorf("%w: %s", ErrDuplicateMessage, env.ID)
	}
	return nil
}

// RateLimiter is a simple per-sender fixed-window counter: at most Limit
// messages per Window per key. It is deliberately small — the pre-check
// pipeline's job is to bound abuse, not to implement a general limiter — and
// concurrency-safe, since a gateway serves both the loopback socket and the
// network A2A listener from the same process.
type RateLimiter struct {
	mu     sync.Mutex
	Limit  int
	Window time.Duration
	counts map[string][]time.Time
}

// NewRateLimiter returns a RateLimiter admitting at most limit messages per
// window, per sender key.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{Limit: limit, Window: window, counts: make(map[string][]time.Time)}
}

// Allow records one attempt for key at now and reports whether it is admitted.
// Entries older than Window are pruned first, so the window slides rather than
// resetting on a boundary.
func (r *RateLimiter) Allow(key string, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.counts == nil {
		r.counts = make(map[string][]time.Time)
	}
	cutoff := now.Add(-r.Window)
	kept := r.counts[key][:0]
	for _, t := range r.counts[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= r.Limit {
		r.counts[key] = kept
		return false
	}
	r.counts[key] = append(kept, now)
	return true
}
