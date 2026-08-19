package loopengine

import (
	"sync"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// liveness.go — the timeout TAXONOMY and heartbeat liveness that decomposes the single
// coarse StaleClaim knob (arch doc §4: ">120 min, no branch") into the three distinct
// timers Temporal names.
//
// One knob answers two unrelated questions with the same number: a worker that DIES AT
// DISPATCH holds its item for 120 minutes before the stale-claim policy frees it, and a
// worker that dies AFTER PUSHING A BRANCH may hold it forever (branch-liveness reads as
// "live work"). The taxonomy separates those failure modes so each gets its own clock, all
// engine-internal Config DATA — no new Loop hook, so the frozen contract (doc.go) is intact.
//
// # The three timers (Temporal vocabulary → AT meaning)
//
//   - ScheduleToStart — dispatch → first observable worker event. A worker that never boots
//     is reclaimable in MINUTES (this timer), not hours (StaleClaim). Answers "did it ever
//     start?".
//   - StartToClose — per-tier wall cap on a RUNNING worker (start → close). A TierLocal verify
//     run and a TierCheap L-effort implementation deserve different clocks, so the cap is keyed
//     off the dispatch Tier. Answers "is it running too long for its class?".
//   - HeartbeatGap — max gap between observable events before a started worker is presumed
//     dead. Answers "has it stopped emitting signs of life?".
//
// # Observable events — derive liveness from artifacts, add NO worker-side protocol
//
// An "observable event" is evidence the worker exists that the engine can poll READ-ONLY:
// audit.jsonl lines attributable to the dispatch, the claim's recorded branch gaining commits
// (git ls-remote sha movement), PR creation/updates. This is the same derive-from-artifacts
// principle the boards use — the worker already produces these; the engine only reads them.
// Like Config.WorkEvidence, the concrete sources are supplied by the consumer (ObservableProbes),
// never hard-coded into the engine, so no network dependency enters the frozen contract.
//
// # Reclaim stays CONSERVATIVE (the double-dispatch guard)
//
// A too-eager reclaim double-dispatches (the exact double-dispatch failure); a too-lazy one recreates the
// two-hour wedge. Two rules keep the eager direction closed:
//
//   - COULD-NOT-CHECK IS NEVER "NO LIFE". A probe that cannot reach its source returns an
//     error, exactly like WorkEvidence's three-state contract. A worker is presumed dead only
//     on a CLEAN "no observable event" reading, never on an unreachable probe — "cannot prove
//     inactive ⇒ do not steal" is preserved.
//   - RECLAIM ROUTES THROUGH THE LOCK-GUARDED SINGLE WINNER. ScheduleToStart / HeartbeatGap
//     expiry only marks the claim reclaim-ELIGIBLE by freeing it through deskkit.ReleaseMatching
//     (compare-and-delete under the claims-dir flock — the same primitive recover.go uses); the
//     actual re-dispatch still races through Claim → deskkit.Acquire, where the flock makes
//     exactly one winner. StaleClaim (120m) remains the backstop of last resort.
//
// StartToClose expiry is different: the worker is (probably) alive but over its class budget,
// so the item is LANDED as blocked-timeout (file-and-continue) and the decision journalled,
// rather than freed for an identical re-dispatch that would blow the same budget again.

// LivenessPolicy is the decomposed timeout taxonomy — engine-internal Config data, not a Loop
// hook. A nil *LivenessPolicy on Config disables liveness entirely and the engine falls back to
// the single StaleClaim knob (zero behaviour change), so this is strictly additive.
type LivenessPolicy struct {
	// ScheduleToStart bounds dispatch → first observable worker event. Past it, a worker that
	// emitted NO sign of life is presumed never-booted and its claim is reclaim-eligible. Zero
	// disables this timer (StaleClaim then owns the never-booted case). Keep it under StaleClaim
	// — its whole point is minutes-scale orphan detection instead of hours-scale.
	ScheduleToStart time.Duration
	// StartToClose is the per-tier wall cap (start → close) on a RUNNING worker, keyed off the
	// dispatch Tier so a local verify run and a cheap-tier L-effort implementation get different
	// clocks. A Tier absent from the map (or a zero value) has NO wall cap and falls back to the
	// StaleClaim backstop. TierHuman is never dispatched, so it needs no entry.
	StartToClose map[Tier]time.Duration
	// HeartbeatGap is the maximum gap between observable events for a STARTED worker before it is
	// presumed dead and its claim is reclaim-eligible. Zero disables the heartbeat timer. A
	// worker whose branch keeps gaining commits keeps refreshing its heartbeat and is never
	// reclaimed, however quiet its audit stream.
	HeartbeatGap time.Duration
}

// DefaultLivenessPolicy returns conservative defaults: minutes-scale never-booted detection,
// per-tier wall caps, and a heartbeat gap — all comfortably under the 120-minute StaleClaim
// backstop so StaleClaim remains the last resort, never the first responder.
func DefaultLivenessPolicy() LivenessPolicy {
	return LivenessPolicy{
		ScheduleToStart: 10 * time.Minute,
		StartToClose: map[Tier]time.Duration{
			TierLocal:   20 * time.Minute, // a local verify run is short-lived
			TierCheap:   90 * time.Minute, // batch M/L implementation, the longest class
			TierSession: 90 * time.Minute, // exec-tier: strong implementation
		},
		HeartbeatGap: 20 * time.Minute,
	}
}

// startToClose returns the wall cap for a tier, or 0 when the tier has no configured cap
// (fall back to the StaleClaim backstop). A negative configured value is treated as 0.
func (p LivenessPolicy) startToClose(t Tier) time.Duration {
	if p.StartToClose == nil {
		return 0
	}
	d := p.StartToClose[t]
	if d < 0 {
		return 0
	}
	return d
}

// Observation is one read-only sign of life. At is when the worker most recently proved it
// exists; What names the source ("branch sha moved", "audit line", "PR updated") for the log.
// A zero At means "no sign of life seen" — distinct from could-not-check, which is an error.
type Observation struct {
	At   time.Time
	What string
}

// ObservableProbe polls ONE read-only source for signs the dispatched worker for `it` exists,
// looking no earlier than `since`. It is three-state, exactly like Config.WorkEvidence:
//
//	(obs, nil)  — checked; obs.At is the most recent sign of life, or zero if none was seen
//	(_,   err)  — COULD-NOT-CHECK; the source was unreachable/unreadable. This is NEVER read as
//	              "no life" — presuming death from an unreachable probe is the eager reclaim
//	              that double-dispatches. The engine keeps the worker alive on an errored probe.
//
// The engine deliberately implements no probe: what counts as an observable event is
// per-consumer, and hard-coding git/GitHub into the engine would put a network dependency
// inside the frozen contract. The consumer supplies the sources via ObservableProbes.
type ObservableProbe func(it Item, since time.Time) (Observation, error)

// ObservableProbes is the consumer-supplied set of read-only liveness sources named in the
// brief. Any field may be nil (that source is simply not consulted). They are polled together
// and the MOST RECENT observation across them is the worker's heartbeat.
type ObservableProbes struct {
	// AuditScan looks for audit.jsonl lines attributable to the dispatch.
	AuditScan ObservableProbe
	// BranchMoved reports the claim's recorded branch gaining commits (git ls-remote sha
	// movement) — the signal that catches the "silent but still pushing" worker.
	BranchMoved ObservableProbe
	// PRActivity reports PR creation/updates naming the item.
	PRActivity ObservableProbe
}

// Latest polls every configured probe and reduces them to a single heartbeat reading. The
// reduction is the conservative one: a POSITIVE observation from ANY source proves life and
// wins outright (errors from the other sources are then irrelevant). Only when NO source saw
// life does an error matter — and then Latest reports could-not-check so the caller keeps the
// worker alive rather than presuming death from an unreachable source.
//
//	(obs, nil)   — obs.At is the newest sign of life (zero At = cleanly silent, all probes agreed)
//	(_,   err)   — no source saw life AND at least one source could-not-check ⇒ do NOT reclaim
func (p *ObservableProbes) Latest(it Item, since time.Time) (Observation, error) {
	if p == nil {
		return Observation{}, nil
	}
	var best Observation
	var firstErr error
	for _, probe := range []ObservableProbe{p.AuditScan, p.BranchMoved, p.PRActivity} {
		if probe == nil {
			continue
		}
		obs, err := probe(it, since)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if obs.At.After(best.At) {
			best = obs
		}
	}
	if !best.At.IsZero() {
		return best, nil // a positive sign of life outranks any could-not-check
	}
	if firstErr != nil {
		return Observation{}, deskkit.Unverifiable("liveness: no observable event and a probe could not check "+it.ID, firstErr)
	}
	return Observation{}, nil // cleanly silent — every probe checked and saw nothing
}

// livenessState is the per-handle liveness clock the engine refreshes during its poll cycle.
type livenessState struct {
	item         Item
	tier         Tier
	dispatchedAt time.Time // when Dispatch returned a live handle (schedule-to-start origin)
	startedAt    time.Time // first observable event; zero until the worker proves it started
	lastObserved time.Time // most recent observable event; zero until started (heartbeat clock)
}

func (s *livenessState) started() bool { return !s.startedAt.IsZero() }

// since is the lower bound handed to the probes: look no earlier than the last known sign of
// life, or the dispatch time before the first one.
func (s *livenessState) since() time.Time {
	if s.lastObserved.IsZero() {
		return s.dispatchedAt
	}
	return s.lastObserved
}

// observe folds a fresh observation into the clock: the first observation satisfies
// schedule-to-start (sets startedAt), and any later observation refreshes the heartbeat.
func (s *livenessState) observe(at time.Time) {
	if at.IsZero() {
		return
	}
	if s.startedAt.IsZero() {
		s.startedAt = at
	}
	if at.After(s.lastObserved) {
		s.lastObserved = at
	}
}

// livenessDisposition is the taxonomy verdict for one worker at one poll tick.
type livenessDisposition int

const (
	livenessAlive               livenessDisposition = iota // within every timer — leave it running
	livenessReclaimNeverStarted                            // schedule-to-start expiry (never booted)
	livenessReclaimHeartbeat                               // heartbeat-gap expiry (stopped emitting)
	livenessBlockedStartToClose                            // start-to-close wall cap (running too long)
)

// evaluate applies the taxonomy to one worker's clock. It is a PURE function of the state, the
// policy, now, and whether this cycle got a CLEAN observability reading (observed) — no IO — so
// every timing decision is directly unit-testable (TestLiveness*).
//
// `observed` is the conservative-reclaim hinge. The two RECLAIM verdicts (schedule-to-start,
// heartbeat) presume a worker ABSENT, and a presumption of absence is only safe when the engine
// actually LOOKED and saw nothing this cycle. So both fire only when observed is true: an
// errored probe (could-not-check) or no probe configured at all leaves observed false, and the
// worker is kept alive — falling back to the StaleClaim backstop rather than reclaiming blind
// (the double-dispatch guard). Start-to-close is different: it is a pure WALL CLOCK
// ("running too long for its class"), independent of observability, and its action is a
// blocked-timeout LAND (a decision, not a blind re-dispatch), so it fires regardless of observed.
//
// Order matters: start-to-close is checked before heartbeat so a worker both over its wall cap
// AND silent lands as blocked-timeout rather than being freed for an identical re-dispatch.
func (p LivenessPolicy) evaluate(s *livenessState, now time.Time, observed bool) livenessDisposition {
	if s.started() {
		if cap := p.startToClose(s.tier); cap > 0 && now.Sub(s.startedAt) > cap {
			return livenessBlockedStartToClose
		}
		if observed && p.HeartbeatGap > 0 && now.Sub(s.lastObserved) > p.HeartbeatGap {
			return livenessReclaimHeartbeat
		}
		return livenessAlive
	}
	if observed && p.ScheduleToStart > 0 && now.Sub(s.dispatchedAt) > p.ScheduleToStart {
		return livenessReclaimNeverStarted
	}
	return livenessAlive
}

// livenessTracker holds one clock per in-flight dispatched worker. Every method runs in Run()'s
// single goroutine (register from fillPool, sweep from the poll wake, unregister on completion),
// but the mutex is kept so the invariant survives a future concurrent caller.
type livenessTracker struct {
	mu     sync.Mutex
	states map[string]*livenessState
}

func newLivenessTracker() *livenessTracker {
	return &livenessTracker{states: map[string]*livenessState{}}
}

// register begins tracking a worker's liveness from its dispatch time.
func (t *livenessTracker) register(it Item, tier Tier, at time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.states[it.ID] = &livenessState{item: it, tier: tier, dispatchedAt: at}
}

// unregister stops tracking a worker (landed, reclaimed, or blocked-timeout).
func (t *livenessTracker) unregister(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.states, id)
}

// snapshot returns the tracked states as a slice of live pointers (safe to iterate while the
// caller mutates individual states, since it never deletes during the range).
func (t *livenessTracker) snapshot() []*livenessState {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]*livenessState, 0, len(t.states))
	for _, s := range t.states {
		out = append(out, s)
	}
	return out
}

// livenessOutcome pairs a worker with the non-alive disposition the sweep assigned it.
type livenessOutcome struct {
	item Item
	tier Tier
	disp livenessDisposition
}

// sweepLiveness is the poll-cycle refresh-and-evaluate over every tracked worker. It probes
// each worker's observable sources READ-ONLY, refreshes its lastObserved, then evaluates the
// taxonomy — returning the workers whose clocks expired for the caller (Run) to act on. A nil
// LivenessPolicy makes this a no-op, so an engine without liveness configured is unaffected.
//
// A could-not-check probe is logged and does NOT advance the clock and does NOT reclaim: the
// worker is judged on the last reading it COULD get, never presumed dead from an unreachable
// source (the conservative-reclaim rule).
func (cfg Config) sweepLiveness(t *livenessTracker, now time.Time) []livenessOutcome {
	if cfg.Liveness == nil {
		return nil
	}
	pol := *cfg.Liveness
	var out []livenessOutcome
	for _, s := range t.snapshot() {
		// observed is true only when a probe RAN and returned without error this cycle — the
		// clean "we looked" signal the two reclaim verdicts are gated on. No probe configured, or
		// an errored (could-not-check) probe, leaves it false, and evaluate keeps the worker alive.
		observed := false
		if cfg.Observe != nil {
			obs, err := cfg.Observe.Latest(s.item, s.since())
			if err != nil {
				cfg.logf("liveness: COULD-NOT-CHECK %s — observable probe unreachable; NOT reclaiming (conservative, cannot prove inactive): %v", s.item.ID, err)
			} else {
				observed = true
				s.observe(obs.At)
			}
		}
		if disp := pol.evaluate(s, now, observed); disp != livenessAlive {
			out = append(out, livenessOutcome{item: s.item, tier: s.tier, disp: disp})
		}
	}
	return out
}

// reclaimForLiveness frees an item's claim through the SAME lock-guarded compare-and-delete
// recover.go uses (deskkit.ReleaseMatching): it deletes the on-disk claim only if it still
// matches what List read, so a claim reclaimed in place by another conductor between the read
// and the delete is left alone rather than stolen. Freeing the claim makes the item
// reclaim-ELIGIBLE; the actual re-dispatch still races through Claim → Acquire's single winner.
//
//	(true, nil)  — the claim matched and was freed (item now re-dispatchable)
//	(false, nil) — no claim on disk, or it changed underneath us — nothing freed, not an error
//	(false, err) — the claims dir / lock could not be reached — fail closed, never a silent steal
func reclaimForLiveness(cfg Config, item string) (bool, error) {
	if cfg.ClaimsDir == "" {
		return false, nil // no claims dir to reclaim against
	}
	claims, err := deskkit.List(toClaimConfig(cfg))
	if err != nil {
		return false, err
	}
	for _, c := range claims {
		if c.Item == item {
			return deskkit.ReleaseMatching(toClaimConfig(cfg), c)
		}
	}
	return false, nil
}
