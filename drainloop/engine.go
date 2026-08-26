package drainloop

import (
	"fmt"
	"time"
)

// Loop is the frozen six-method contract every drain role implements. Heterogeneity lives in
// TierPolicy and Land; everything else is the engine's, written once. Adding a seventh method
// is a design decision, not a convenience: a new hook propagates to every consumer, so the
// contract is kept small on purpose.
type Loop interface {
	// Name identifies this loop, for logging and for the claim namespace.
	Name() string
	// SelectQueue returns the items to consider this pass, most-eligible first. The engine
	// does not know or care where they come from.
	SelectQueue() ([]Item, error)
	// TierPolicy maps an item to a dispatch destination. Return TierHuman to route the item
	// to Land without dispatching it. The error return is the difference between "no tier"
	// and "could not tell": a could-not-check tier read is skipped this pass, never guessed.
	TierPolicy(Item) (Tier, error)
	// Dispatch carries out one dispatch and returns a Handle to await. The resolved Tier is
	// passed in so the dispatcher routes to the right runner class instead of re-deriving it.
	// This is the only method that differs between "emit an instruction" and "spawn a
	// process".
	Dispatch(Item, Tier) (Handle, error)
	// Land records one finished (or held, or failed) result and releases anything the item
	// held. The item is folded into Result.Item, so Land receives one structured value. All
	// per-loop behaviour that is not tiering lives here.
	Land(Result) error
	// OnIdle is called when a pass dispatched nothing. What "idle" means is a policy the
	// adapter owns: a long-running desk refreshes its board and keeps polling (Config default);
	// a batch adapter stops when its queue empties (Config.StopWhenIdle). OnIdle only reports
	// an error; the stop decision is the engine's, driven by Config.
	OnIdle() error
}

// Claimer is the dedupe chokepoint. An item is claimed before it is dispatched, and released
// after it is landed. The guarantee — an ID is in flight at most once — holds ONLY for
// dispatchers that route through Claim; skipping it is outside the guarantee by construction.
type Claimer interface {
	// Claim takes the item's ID. It returns (true, nil) if this caller now holds it,
	// (false, nil) if someone else already does (do not dispatch), or (false, err) if it
	// could not tell — which is could-not-check, never "assume free".
	Claim(id string) (bool, error)
	// Release drops a claim previously acquired by this caller. It is idempotent.
	Release(id string) error
}

// Config wires a Loop and a Claimer together with the scheduling knobs, plus the optional,
// deskkit-free shared-core layers. Every optional layer is off at its zero value, so a
// Config that sets only Loop/Claimer/PoolSize runs the plain six-method core unchanged.
type Config struct {
	// Loop is the role adapter. Required.
	Loop Loop
	// Claimer is the dedupe store. Required.
	Claimer Claimer
	// PoolSize is the in-flight ceiling the pool guard is configured with; it must be >= 1. In
	// the current emit-one-instruction form the engine dispatches an item and awaits its result
	// before considering the next, so exactly one item is in flight at a time regardless of
	// PoolSize — the ceiling becomes load-bearing only once Dispatch returns before the work it
	// dispatched has completed.
	PoolSize int
	// Log, if set, receives one line per scheduling decision. Attribution is a property of
	// the engine, not of the model: every claim, dispatch, land, and release is recorded.
	Log func(string)

	// --- optional shared-core layers; zero value = off ---

	// RunnerID, when non-empty, enables the author-not-runner structural guard: an item whose
	// Implementer equals RunnerID is routed to Land as held rather than dispatched (the author
	// of a change must not be the runner that verifies it). Empty ⇒ the guard is off.
	RunnerID string
	// Evidence, when set, is consulted before an item is claimed: if the work is already done
	// elsewhere, the item is landed without dispatch. nil ⇒ not performed.
	Evidence WorkEvidence
	// Journal, when set, receives a structured Event per scheduling decision alongside Log.
	// nil ⇒ not performed. The public interface is deskkit-free; a house sink maps it onto an
	// audit schema.
	Journal Journal
	// StopWhenIdle selects batch mode: when a pass dispatches nothing and StopWhenIdle is
	// true, the drain calls OnIdle once and returns. Default false = poll forever (a
	// long-running desk), sleeping IdlePoll between passes.
	StopWhenIdle bool
	// IdlePoll is the sleep between idle passes in poll-forever mode. Zero = no sleep (a busy
	// poll — set a real interval, or StopWhenIdle, in production).
	IdlePoll time.Duration
	// Sleep is a test seam for IdlePoll. nil ⇒ time.Sleep.
	Sleep func(time.Duration)
}

func (c Config) log(format string, a ...any) {
	if c.Log != nil {
		c.Log(fmt.Sprintf(format, a...))
	}
}

// record emits a structured Event to the Journal sink, if one is configured. A sink failure
// is logged but does not abort the drain: attribution is best-effort, and a lost journal
// entry is not a scheduling failure.
func (c Config) record(kind, id, detail string) {
	if c.Journal == nil {
		return
	}
	if err := c.Journal.Record(Event{Kind: kind, ItemID: id, Detail: detail, Time: time.Now()}); err != nil {
		c.log("JOURNAL %s %s — sink error (non-fatal): %v", kind, id, err)
	}
}

func (c Config) sleepIdle() {
	if c.Sleep != nil {
		c.Sleep(c.IdlePoll)
		return
	}
	if c.IdlePoll > 0 {
		time.Sleep(c.IdlePoll)
	}
}

// Run drives the drain: for each claimable item in turn, dispatch it, await its result, land it,
// and release the claim, then poll (or stop) on idle. One item is in flight at a time in the
// current form. It returns when a batch drain goes idle (Config.StopWhenIdle), or on the first
// hard error the loop cannot land around.
//
// The disciplines the contract enforces rather than requests:
//   - a dispatch that FAILS does not abort the drain — it is landed as VerdictError and its
//     claim released, so one bad item cannot freeze the pool;
//   - an item that CANNOT be claimed is skipped this pass, never dispatched twice;
//   - a HELD item (TierHuman) is landed without dispatch and the drain continues;
//   - a could-not-check tier read is skipped, never guessed.
func Run(c Config) error {
	if c.PoolSize < 1 {
		return fmt.Errorf("drainloop: PoolSize must be >= 1, got %d", c.PoolSize)
	}
	if c.Loop == nil || c.Claimer == nil {
		return fmt.Errorf("drainloop: Loop and Claimer are required")
	}
	name := c.Loop.Name()

	for {
		items, err := c.Loop.SelectQueue()
		if err != nil {
			return fmt.Errorf("drainloop[%s]: SelectQueue: %w", name, err)
		}

		dispatched := 0
		inFlight := 0

		for _, it := range items {
			if inFlight >= c.PoolSize {
				break // pool is full; the rest wait for the next pass
			}

			// Author-not-runner structural guard (opt-in). An item must never be dispatched
			// to a runner that authored it. Route it to Land as held instead.
			if err := CheckAuthorRunner(it, c.RunnerID); err != nil {
				res := Result{Item: it, Verdict: VerdictHold, Rows: []EvidenceRow{{Name: "author-runner-guard", Output: err.Error()}}}
				if lerr := c.Loop.Land(res); lerr != nil {
					return fmt.Errorf("drainloop[%s]: Land guarded %s: %w", name, it.ID, lerr)
				}
				c.record("GUARD", it.ID, err.Error())
				c.log("GUARD %s — author==runner, routed to Land not dispatched", it.ID)
				continue
			}

			tier, err := c.Loop.TierPolicy(it)
			if err != nil {
				// could-not-check: do not dispatch, do not guess.
				c.record("SKIP", it.ID, "TierPolicy could-not-check: "+err.Error())
				c.log("SKIP %s — TierPolicy could-not-check: %v", it.ID, err)
				continue
			}

			// A human-tiered item never touches the pool: land it as held and move on.
			if tier == TierHuman {
				res := Result{Item: it, Verdict: VerdictHold, Rows: []EvidenceRow{{Name: "held", Output: "routed to a human, not dispatched"}}}
				if lerr := c.Loop.Land(res); lerr != nil {
					return fmt.Errorf("drainloop[%s]: Land held %s: %w", name, it.ID, lerr)
				}
				c.record("HOLD", it.ID, "TierHuman")
				c.log("HOLD %s — TierHuman, routed to Land not dispatched", it.ID)
				continue
			}

			// External-evidence probe (opt-in), BEFORE the claim: if the work is already done
			// elsewhere, land it without dispatching.
			if c.Evidence != nil {
				taken, why, err := c.Evidence(it)
				if err != nil {
					c.record("SKIP", it.ID, "evidence probe could-not-check: "+err.Error())
					c.log("SKIP %s — evidence probe could-not-check: %v", it.ID, err)
					continue
				}
				if taken {
					res := Result{Item: it, Verdict: VerdictPass, Rows: []EvidenceRow{{Name: "work-evidence", Output: why}}}
					if lerr := c.Loop.Land(res); lerr != nil {
						return fmt.Errorf("drainloop[%s]: Land evidenced %s: %w", name, it.ID, lerr)
					}
					c.record("EVIDENCE", it.ID, why)
					c.log("EVIDENCE %s — already done elsewhere (%s), landed without dispatch", it.ID, why)
					continue
				}
			}

			// Claim before dispatch. This is the single dedupe chokepoint.
			ok, err := c.Claimer.Claim(it.ID)
			if err != nil {
				c.record("SKIP", it.ID, "claim could-not-check: "+err.Error())
				c.log("SKIP %s — claim could-not-check: %v", it.ID, err)
				continue
			}
			if !ok {
				c.record("SKIP", it.ID, "already claimed elsewhere")
				c.log("SKIP %s — already claimed elsewhere", it.ID)
				continue
			}

			inFlight++
			c.record("CLAIM", it.ID, "")
			c.log("CLAIM %s", it.ID)

			res := c.dispatchAndAwait(it, tier)

			// Land every outcome — success, clean failure, or dispatch error — then release
			// the claim exactly once. The drain continues regardless of the verdict.
			if err := c.Loop.Land(res); err != nil {
				_ = c.Claimer.Release(it.ID)
				return fmt.Errorf("drainloop[%s]: Land %s: %w", name, it.ID, err)
			}
			if err := c.Claimer.Release(it.ID); err != nil {
				return fmt.Errorf("drainloop[%s]: Release %s: %w", name, it.ID, err)
			}
			c.record("LAND", it.ID, res.Verdict.String())
			c.log("LAND %s verdict=%s; RELEASE", it.ID, res.Verdict)

			inFlight--
			dispatched++
		}

		if dispatched == 0 {
			if err := c.Loop.OnIdle(); err != nil {
				return fmt.Errorf("drainloop[%s]: OnIdle: %w", name, err)
			}
			if c.StopWhenIdle {
				c.record("IDLE", "", "stop")
				c.log("IDLE — stop")
				return nil
			}
			c.record("IDLE", "", "poll again")
			c.log("IDLE — poll again")
			c.sleepIdle()
		}
	}
}

// dispatchAndAwait runs one dispatch and folds a dispatch-level error into a landable
// VerdictError Result, so a failed dispatch is landed rather than aborting the drain.
func (c Config) dispatchAndAwait(it Item, tier Tier) Result {
	c.record("DISPATCH", it.ID, tier.String())
	c.log("DISPATCH %s tier=%s", it.ID, tier)
	h, err := c.Loop.Dispatch(it, tier)
	if err != nil {
		return Result{
			Item:    it,
			Verdict: VerdictError,
			Rows:    []EvidenceRow{{Name: "dispatch", Output: "dispatch failed: " + err.Error()}},
		}
	}
	r := <-h.Done()
	if r.Item.ID == "" {
		r.Item = it // adapters that build a Result directly need not repeat the item
	}
	return r
}
