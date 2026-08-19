package loopengine

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Item is a typed unit of drain work. No prose fields except Payload (the template
// variables the adapter interpolates into the dispatch prompt). Everything the engine
// itself reasons about — risk, gate, effort, tier, implementer — is typed.
type Item struct {
	ID          string            // e.g. "team/01" or "issue-841" — stable, claim key
	BriefPath   string            // repo-relative brief file, when brief-shaped
	TargetSHA   string            // merged-main SHA the dispatched worker runs against
	Risk        RiskFlags         // parsed frontmatter
	Gate        string            // "model" | "human"
	Effort      string            // S | M | L
	ExecTier    string            // "any" | "strong"
	Implementer string            // for the author != runner typed guard (CheckAuthorRunner)
	Payload     map[string]string // template variables for the dispatch prompt
	// Retry optionally overrides Config.Retry for this item alone. nil
	// means the engine default. It is Item DATA, not a Loop hook.
	Retry *RetryPolicy
}

// RiskFlags is the parsed brief-v1 risk frontmatter (the four canonical keys).
type RiskFlags struct {
	Regulatory, Customer, Irreversible, SensitiveData bool
}

// Flagged reports whether the item is risk-flagged: gate is human OR any risk answer is
// yes. This is the exact predicate statusgen's brieffile.go uses for its verifier floor.
func (r RiskFlags) Flagged(gate string) bool {
	return gate == "human" || r.Regulatory || r.Customer || r.Irreversible || r.SensitiveData
}

// Tier is a dispatch DESTINATION, not a model name — the adapter maps a Tier onto a
// concrete runner. TierHuman is not dispatchable at all: it routes to the checkpoint-PR
// / labeled-issue path via Land, and the drain continues.
type Tier int

const (
	TierLocal   Tier = iota // local session model (verify-desk's ONLY model tier)
	TierCheap               // economy tier (batch M/L default)
	TierSession             // session tier (exec-tier: strong)
	TierHuman               // NOT dispatchable: route to checkpoint-PR / labeled issue
)

func (t Tier) String() string {
	switch t {
	case TierLocal:
		return "local"
	case TierCheap:
		return "cheap"
	case TierSession:
		return "session"
	case TierHuman:
		return "human"
	default:
		return fmt.Sprintf("tier(%d)", int(t))
	}
}

// Verdict values a Result can carry.
const (
	VerdictPass         = "PASS"
	VerdictFail         = "FAIL"
	VerdictBlocked      = "BLOCKED"
	VerdictNeedsContext = "NEEDS_CONTEXT"
	// VerdictRouteHuman is the engine-synthesized verdict for a TierHuman item that was
	// never dispatched: Land renders it as a checkpoint-PR / labeled-issue routing, never
	// a status flip.
	VerdictRouteHuman = "ROUTE_HUMAN"
)

// Result is the structured return Land consumes. Free text never reaches the board: the
// Evidence rows are rendered from Rows, and RunnerID is the identity the ENGINE
// dispatched (not a self-report the operator transcribes).
type Result struct {
	Item     Item
	Verdict  string        // PASS | FAIL | BLOCKED | NEEDS_CONTEXT | ROUTE_HUMAN
	Rows     []EvidenceRow // one per Verify-table row
	RunnerID string        // the identity the engine dispatched — attribution, not un-forgeability
	Artifact string        // PR URL / issue URL / commit SHA, when the loop produces one
}

// EvidenceRow is one observed Verify-table row. Output is the KEY observed line, never a
// claim ("PASS" transcribed by hand).
type EvidenceRow struct {
	Command string
	Exit    int
	Output  string
}

// Loop is the per-loop adapter — everything an archetype-A drain loop is allowed to
// customize. This interface is FROZEN (arch doc §8 "Contract erosion" risk): heterogeneity
// lives in Land and TierPolicy. Anything that does not fit these hooks does NOT go in the
// engine — a proposed new hook is a design review (file a contract-erosion issue), not a
// patch.
type Loop interface {
	Name() string                        // canonical DESK_LOOP name (stop-flag scoping)
	SelectQueue() ([]Item, error)        // deterministic board read (statusgen / issueboard)
	TierPolicy(Item) (Tier, error)       // per-loop tier + risk routing; TierHuman = don't dispatch
	Dispatch(Item, Tier) (Handle, error) // rendered prompt + isolated worktree; INTERIM: emit + await feed
	Land(Result) error                   // make durable (Evidence+flip / placeholder retire / no-op)
	OnIdle() error                       // refresh board, rescan orphans+claims
}

// Handle tracks one in-flight dispatched agent. In interim mode Done() fires when the
// operator feeds the structured Result back; under the native-primitive upgrade it fires
// when the child worker process returns. The engine treats both identically.
type Handle interface {
	Done() <-chan Result
	Item() Item
}

// Config is engine state, NOT a per-loop hook (the frozen contract is the Loop interface).
// PoolSize is a CONSTANT the engine holds — it is never a variable the model manages under
// load, which is the whole point.
type Config struct {
	PoolSize   int           // constant in-flight cap. verify: sized per queue; batch: 8
	IdlePoll   time.Duration // idle-poll cadence (~60-90s in prod; small in tests)
	ClaimsDir  string        // shared noclobber claims dir (~/.config/assay/claims)
	StaleClaim time.Duration // age past which a claim with no live branch may be reclaimed (120m)
	// RunnerID is the identity this engine instance dispatches AS (the session/operator
	// identity). It is the left-hand side of the author != runner structural guard: the
	// engine refuses to dispatch an item whose Implementer == RunnerID. Empty disables the
	// guard (the adapter is then responsible, e.g. worker-desk where authorship differs
	// per worker). This is engine config, not a Loop hook — it does not erode the contract.
	RunnerID string
	// Branch is the git branch this engine instance runs on. It is recorded in every
	// claim file so claimIsStale's branch-liveness guard has something to check: before
	// this field existed the guard read claimRecord.Branch, but Claim never WROTE it, so
	// the branch half of the stale-claim policy was dead code and every claim was
	// reclaimable on age alone.
	//
	// Set Branch together with BranchActive. Branch alone makes claims UN-reclaimable:
	// claimIsStale sees a recorded branch it cannot prove inactive (BranchActive == nil)
	// and conservatively declines to steal, so a crashed session's claim never frees.
	// Empty (the default) preserves the age-only behaviour.
	Branch string
	// Progress receives one line per engine decision ("landed: X", "filed-and-continued: X",
	// "refused: author==runner X", "routed-human: X", "idle", "stop: <flag>"). nil = stderr.
	// It is the debug/observability surface, never load-bearing state.
	Progress io.Writer
	// BranchActive optionally reports whether a stale claim's recorded branch still has
	// live work; nil means a recorded branch cannot be proven live, so a claim is reclaimed
	// only on age (StaleClaim) with no recorded branch. See Claim.
	BranchActive func(branch string) bool
	// WorkEvidence is the external-evidence probe Claim consults BEFORE it will return
	// "acquired" — the answer to "is this item already taken by something outside the
	// claims dir?" (an open PR naming it, a merged PR naming it, the board row). A claim
	// file records only that a dispatcher is working on the item; acquiring one is not
	// evidence the work is unclaimed, which is the gap the WorkEvidence probe names.
	//
	// nil means the check is NOT performed — permitted, and then Claim ANNOUNCES the
	// narrowed bound on Progress for every item (no silent caps). A probe that errors is
	// could-not-check and makes Claim fail closed (exit 6), never a soft acquire. This is
	// engine config, not a Loop hook — it does not erode the frozen contract.
	WorkEvidence WorkEvidence
	// Retry is the DECLARED retry policy applied to the dispatch path.
	// nil means DefaultRetryPolicy(); an Item may override it per item (Item.Retry). Retry
	// is engine-internal data wrapped around Loop.Dispatch — it adds no Loop hook and does
	// not erode the frozen contract. See retry.go for the taxonomy and every bound.
	Retry *RetryPolicy
	// Sleep is the back-off sleep seam. nil means time.Sleep; tests substitute a recorder
	// so a backoff schedule is asserted rather than waited on.
	Sleep func(time.Duration)
	// Liveness is the decomposed timeout taxonomy that replaces the single
	// coarse StaleClaim knob with schedule-to-start / per-tier start-to-close / heartbeat-gap
	// timers. nil DISABLES liveness entirely — the engine then relies only on StaleClaim, exactly
	// as before this field existed — so it is strictly additive engine data, not a Loop hook. See
	// liveness.go for the taxonomy and every conservative-reclaim guard.
	Liveness *LivenessPolicy
	// Observe is the consumer-supplied set of READ-ONLY observable-event probes (audit scan,
	// branch sha movement, PR activity) the engine polls each cycle to refresh a worker's
	// heartbeat. The liveness taxonomy's fast reclaim REQUIRES observability: nil Observe (or a
	// probe that could-not-check) means the engine cannot prove a worker absent, so NO reclaim
	// fires — schedule-to-start and heartbeat are both gated on a clean "we looked and saw
	// nothing" reading — and with no observation a worker never becomes "started", so
	// start-to-close cannot fire either. With Observe nil, liveness is therefore inert and the
	// StaleClaim backstop is the only clock, exactly as before this feature. The engine
	// implements no probe itself — the sources are per-consumer, like WorkEvidence. See
	// liveness.go.
	Observe *ObservableProbes
	// runID is the per-Run() lineage token for the scheduling journal (journal.go). It is
	// ENGINE-SET, not caller-set — it is unexported precisely so a caller cannot supply one:
	// Run() mints a fresh token per invocation and threads it here (Config passes by value),
	// so fillPool / landOne / Recover all read the same run's identity. Its only job is to
	// differ between two run instances, which is what lets crash recovery tell a prior run of
	// this conductor from a live sibling. See journal.go's runid discussion.
	runID string
}

// retryPolicy resolves the policy for one item: the item's override, else the engine
// default, else DefaultRetryPolicy.
func (c Config) retryPolicy(it Item) RetryPolicy {
	if it.Retry != nil {
		return *it.Retry
	}
	if c.Retry != nil {
		return *c.Retry
	}
	return DefaultRetryPolicy()
}

// sleep waits d through the configured seam.
func (c Config) sleep(d time.Duration) {
	if d <= 0 {
		return
	}
	if c.Sleep != nil {
		c.Sleep(d)
		return
	}
	time.Sleep(d)
}

func (c Config) progress() io.Writer {
	if c.Progress != nil {
		return c.Progress
	}
	return os.Stderr
}

func (c Config) logf(format string, args ...any) {
	fmt.Fprintf(c.progress(), format+"\n", args...)
}

// pending pairs a completed Handle's Result with the engine's tracking. Completions from
// all in-flight handles are merged onto one channel so the orchestrator lands them ONE AT
// A TIME, in arrival order — land-as-returned, never batched to wave-end.
type completion struct{ result Result }

// Run drives the loop until a stop flag. It NEVER exits on an empty queue — is_done()
// triggers OnIdle() and an idle-poll sleep; stop-flags (DISABLED > STOP > STOP.<name>) are
// the ONLY exit. Landing failures retry inside the adapter's Land; a Land that still cannot
// succeed returns an error and the engine files-and-continues — the drain never wedges on
// one item (FILE don't ask, DRAIN don't wait).
//
// This is the interim-mode driver (arch doc §9.1): the engine owns every scheduling fact;
// the Loop.Dispatch hook is where a real Agent call is emitted and its Result fed back. The
// native-primitive upgrade swaps only Dispatch (see package doc).
func Run(cfg Config, loop Loop) error {
	if cfg.PoolSize < 1 {
		return fmt.Errorf("loopengine: PoolSize must be >= 1, got %d", cfg.PoolSize)
	}
	// The retry policy is configuration a human wrote. An incoherent one fails closed here
	// rather than silently behaving like some other policy.
	pol := cfg.retryPolicy(Item{})
	if err := pol.Validate(); err != nil {
		return err
	}
	// A retrying dispatcher HOLDS its claim while it waits, and a claim ages. If all the
	// waiting one item may do could reach Config.StaleClaim, a second dispatcher could
	// correctly judge the live claim abandoned and reclaim it out from under an attempt
	// that is still coming — a double dispatch produced by the retry policy itself. Refuse
	// the config instead of clamping it silently.
	if cfg.StaleClaim > 0 && pol.MaxElapsed >= cfg.StaleClaim {
		return fmt.Errorf("loopengine: RetryPolicy.MaxElapsed (%s) must stay under Config.StaleClaim (%s) — a retrying dispatcher holds its claim while it waits, and a claim that ages past StaleClaim can be reclaimed by another dispatcher mid-retry",
			pol.MaxElapsed, cfg.StaleClaim)
	}
	// Honor per-loop stop flags (STOP.<name>) by scoping DESK_LOOP to this loop when the
	// operator has not already set it (the skill boot does; be robust if it did not).
	if os.Getenv("DESK_LOOP") == "" {
		_ = os.Setenv("DESK_LOOP", loop.Name())
	}

	// Mint this run's lineage token before anything is journalled, so every scheduling event
	// this run emits — and every classification recovery makes — carries one identity.
	cfg.runID = newRunID()

	inflight := map[string]Handle{} // item ID -> in-flight handle
	parked := map[string]bool{}     // item IDs routed-to-human / refused this session (do not re-select)
	tracker := newLivenessTracker() // per-handle liveness clocks; sweep is a no-op when cfg.Liveness == nil
	completions := make(chan completion)
	stopping := false

	// Consult the kill switch BEFORE recovery touches any claim file. Recover releases and
	// reclaims claims (mutation on disk + audit lines); an operator who arms DISABLED/STOP and
	// restarts the conductor to HALT it must not get claim mutation as a side effect of the
	// boot. Guard here, before Recover, not only at the loop head. An unverifiable guard state
	// fails closed (exit 6), exactly as inside the loop.
	if err := deskkit.Guard(); err != nil {
		if !deskkit.IsDisabled(err) {
			return err
		}
		cfg.logf("stop: %s — recovery skipped (kill switch armed before boot)", err.Error())
		stopping = true
	}

	// Crash recovery: replay this loop's journal and reclassify the claim files a prior,
	// crashed run left on disk (release completed leftovers; free in-flight claims for
	// immediate reclaim). Fail-closed on a corrupt journal — see recover.go. This runs
	// BEFORE the first pool fill so a recovered in-flight item is re-selectable at once.
	//
	// Recover only when this process is the SOLE live conductor of its (RunnerID, Branch)
	// lineage: the per-lineage flock (lineagelock.go) makes the "a prior-run claim under my
	// lineage is a CRASHED run of me" premise TRUE rather than assumed. A live sibling holding
	// the lock means that premise is false — its in-flight claims are live — so recovery is
	// skipped and the drain proceeds without touching them. The lock is held for the whole
	// Run() lifetime so a later boot still sees this conductor as live.
	if !stopping {
		if lock, sole := acquireLineageLock(cfg); sole {
			defer lock.release()
			Recover(cfg, loop)
		} else {
			cfg.logf("recover: skipped — not the sole live conductor of lineage %q@%q (a live sibling holds it, or the lineage cannot be locked); a prior-run claim may be a LIVE sibling's, not a crashed run's",
				cfg.RunnerID, cfg.Branch)
		}
	}

	for {
		// Guard at EVERY iteration boundary (never mid-action). Precedence
		// DISABLED > STOP > STOP.<DESK_LOOP> is enforced inside deskkit.Guard.
		if err := deskkit.Guard(); err != nil {
			if deskkit.IsDisabled(err) {
				// Stop flag / kill switch armed: dispatch NO new work; drain what is
				// already in flight (a started land completes), then exit cleanly.
				if !stopping {
					cfg.logf("stop: %s", err.Error())
					stopping = true
				}
			} else {
				// Unverifiable guard state fails closed (exit 6) — propagate, do not guess.
				return err
			}
		}

		// Fill the pool to N (skip entirely while stopping — no new dispatch).
		if !stopping {
			if err := fillPool(cfg, loop, inflight, parked, tracker, completions); err != nil {
				// SelectQueue / TierPolicy read error: cannot fill this pass. It is not a
				// per-item land failure, so do not wedge — surface it and fall through to
				// idle so the next pass retries against a refreshed board.
				cfg.logf("select-error: %v", err)
			}
		}

		// is_done: no in-flight work.
		if len(inflight) == 0 {
			if stopping {
				return nil // stop-flag exit: nothing owed
			}
			// The engine NEVER exits on an empty queue — idle-poll and re-poll.
			cfg.logf("idle")
			if err := loop.OnIdle(); err != nil {
				cfg.logf("onidle-error: %v", err)
			}
			time.Sleep(cfg.IdlePoll)
			continue
		}

		// Wait for exactly ONE completion, land it immediately (land-as-returned), then
		// loop back to refill the freed slot. While stopping we still land in-flight work.
		if stopping {
			c := <-completions
			if _, live := inflight[c.result.Item.ID]; !live {
				// Already disposed by a liveness sweep (reclaimed / blocked-timeout) — a late
				// result from a worker we stopped tracking. Dropping it prevents a double-land.
				cfg.logf("liveness: dropping late completion for %s (already disposed by liveness sweep)", c.result.Item.ID)
				continue
			}
			delete(inflight, c.result.Item.ID)
			tracker.unregister(c.result.Item.ID)
			if failed := landOne(cfg, loop, c.result); failed {
				parked[c.result.Item.ID] = true
			}
			continue
		}
		select {
		case c := <-completions:
			if _, live := inflight[c.result.Item.ID]; !live {
				// Disposed by a liveness sweep already; drop the late result (no double-land).
				cfg.logf("liveness: dropping late completion for %s (already disposed by liveness sweep)", c.result.Item.ID)
				continue
			}
			delete(inflight, c.result.Item.ID)
			tracker.unregister(c.result.Item.ID)
			if failed := landOne(cfg, loop, c.result); failed {
				// Bug filed (FILE don't ask); park it so the drain does not respin on a
				// known-unlandable item and can proceed to empty.
				parked[c.result.Item.ID] = true
			}
			// refill happens on the next iteration's fillPool — immediate, not wave-batched.
		case <-time.After(cfg.IdlePoll):
			// Periodic wake so a stop flag set while the pool is busy is noticed at the next
			// boundary even if no completion arrives. This IS the liveness poll cycle: sweep the
			// tracked workers, refresh each heartbeat from its read-only probes, and act on any
			// whose timer expired (a no-op when cfg.Liveness == nil).
			applyLivenessSweep(cfg, loop, tracker, inflight, parked)
		}
	}
}

// fillPool claims + dispatches eligible items until the pool is at cfg.PoolSize. Items
// already in flight or parked (routed-human / author-refused this session) are skipped;
// a claim collision is skipped (owned elsewhere — structural no-double-dispatch).
func fillPool(cfg Config, loop Loop, inflight map[string]Handle, parked map[string]bool, tracker *livenessTracker, completions chan<- completion) error {
	queue, err := loop.SelectQueue()
	if err != nil {
		return err
	}
	for _, it := range queue {
		if len(inflight) >= cfg.PoolSize {
			return nil
		}
		if _, busy := inflight[it.ID]; busy {
			continue
		}
		if parked[it.ID] {
			continue
		}

		// author != runner: a typed STRUCTURAL guard, not a prompt rule. Refuse to
		// dispatch an item to its own implementer; park it for a different runner.
		if err := CheckAuthorRunner(it, cfg.RunnerID); err != nil {
			cfg.logf("refused: author==runner %s (%s)", it.ID, cfg.RunnerID)
			cfg.journalEvent(loop.Name(), journalRefuse, it, "", "", err.Error())
			parked[it.ID] = true
			continue
		}

		tier, err := loop.TierPolicy(it)
		if err != nil {
			cfg.logf("tier-error: %s: %v", it.ID, err)
			parked[it.ID] = true
			continue
		}

		// TierHuman is not dispatchable: route via Land (checkpoint-PR / labeled issue),
		// keep the drain moving, and park it so it is not re-selected this session.
		if tier == TierHuman {
			cfg.logf("routed-human: %s", it.ID)
			landOne(cfg, loop, Result{Item: it, Verdict: VerdictRouteHuman, RunnerID: "human"})
			parked[it.ID] = true
			continue
		}

		// Claim (atomic noclobber). Collision => owned elsewhere, skip (do not park —
		// the owner may release it).
		claimed, err := Claim(cfg, it)
		if err != nil {
			cfg.logf("claim-error: %s: %v", it.ID, err)
			parked[it.ID] = true
			continue
		}
		if !claimed {
			continue
		}
		// Journal the claim decision (recovery groundwork): an item claimed but not yet
		// followed by a terminal event is what boot replay reads as "in flight at crash".
		cfg.journalEvent(loop.Name(), journalClaim, it, tier.String(), "", "")

		// Dispatch under the DECLARED retry policy. Every attempt runs
		// under the claim acquired just above: dispatchWithRetry never releases and never
		// re-acquires, so a retry can neither double-acquire nor orphan the claim. On a
		// terminal outcome the item is LANDED first (blocked-with-reason, or NEEDS_CONTEXT
		// when the failure was could-not-check) and the claim released after, so no other
		// dispatcher can pick the item up mid-landing.
		h, out := dispatchWithRetry(cfg, loop, it, tier)
		if out != nil {
			cfg.logf("dispatch-error: %s: %s", it.ID, out.Summary(it.ID))
			landOne(cfg, loop, out.Result(it, cfg.RunnerID))
			_ = ReleaseClaim(cfg, it) // hand the slot back; a failed dispatch is not a claim
			cfg.journalEvent(loop.Name(), journalRelease, it, tier.String(), "", "dispatch failed — claim released")
			parked[it.ID] = true
			continue
		}
		// Journal the dispatch: the item is now genuinely in flight under a live handle.
		cfg.journalEvent(loop.Name(), journalDispatch, it, tier.String(), "", "")
		inflight[it.ID] = h
		// Start this handle's liveness clock at dispatch: schedule-to-start runs
		// from here to the worker's first observable event.
		tracker.register(it, tier, time.Now())
		// Merge this handle's completion onto the shared channel so the orchestrator lands
		// results one at a time in arrival order.
		go func(h Handle) { completions <- completion{result: <-h.Done()} }(h)
	}
	return nil
}

// landOne makes one result durable and reports whether the land FAILED. Land does its own
// push-race retry and files-and-continues internally when it can; if it still returns an
// error the engine files-and-continues (audit line, progress line) so the drain never wedges
// on one item, and returns failed=true so the caller parks it.
func landOne(cfg Config, loop Loop, r Result) (failed bool) {
	if err := loop.Land(r); err != nil {
		cfg.logf("filed-and-continued: %s (%v)", r.Item.ID, err)
		// A land that failed is still a TERMINAL land for recovery: the dispatch completed,
		// so the leftover claim is a leftover to release, not an in-flight item to reclaim.
		// It rides the SINGLE journal write path (result=refused) as a well-formed record so
		// replay parses it, rather than a raw prose Detail replay would skip. The board row is
		// not flipped (land failed), so ordinary SelectQueue re-picks the item on a later drain.
		cfg.journalEventResult(loop.Name(), journalLand, r.Item, "", r.Verdict, fmt.Sprintf("land failed: %v — filed and continued", err), deskkit.ResultRefused)
		return true
	}
	cfg.logf("landed: %s (%s)", r.Item.ID, r.Verdict)
	// Journal the land as a TERMINAL event carrying the outcome class. This is the marker
	// boot replay reads as "completed" — a claim whose newest event is a land is a leftover
	// to release, not an item to reclaim.
	cfg.journalEvent(loop.Name(), journalLand, r.Item, "", r.Verdict, "")
	return false
}

// applyLivenessSweep runs one liveness poll cycle and acts on every worker
// whose timer expired. It is a no-op when cfg.Liveness == nil. Each disposition maps to a
// distinct action, and all three drop the worker from inflight + the tracker:
//
//   - never-started / heartbeat expiry → RECLAIM: free the claim through the lock-guarded
//     compare-and-delete so the item is re-dispatchable through the ordinary single-winner
//     Acquire; journal a `reclaim`. NOT parked — the whole point is minutes-scale re-dispatch.
//   - start-to-close expiry → BLOCKED-TIMEOUT: land the item blocked (file-and-continue) and
//     journal the decision, then park it so the drain does not immediately re-dispatch a worker
//     that would blow the same wall cap again.
//
// The completion goroutine for a reclaimed/blocked worker is left pending; if that worker was
// merely slow and later feeds a result, Run's completion handler drops it (the item is no longer
// in inflight), so a liveness disposition never races a late land into a double-land.
func applyLivenessSweep(cfg Config, loop Loop, tracker *livenessTracker, inflight map[string]Handle, parked map[string]bool) {
	for _, o := range cfg.sweepLiveness(tracker, time.Now()) {
		switch o.disp {
		case livenessReclaimNeverStarted, livenessReclaimHeartbeat:
			reason := "heartbeat gap exceeded — no observable event; claim freed for re-dispatch"
			if o.disp == livenessReclaimNeverStarted {
				reason = "schedule-to-start exceeded — worker never emitted a sign of life; claim freed for re-dispatch"
			}
			if ok, err := reclaimForLiveness(cfg, o.item.ID); err != nil {
				cfg.logf("liveness: could not free claim for %s (%s): %v", o.item.ID, o.disp, err)
			} else if ok {
				cfg.logf("liveness: reclaim-eligible %s — %s", o.item.ID, reason)
			} else {
				cfg.logf("liveness: %s claim changed/absent under sweep — left untouched", o.item.ID)
			}
			cfg.journalEvent(loop.Name(), journalReclaim, o.item, o.tier.String(), "", "liveness: "+reason)
			delete(inflight, o.item.ID)
			tracker.unregister(o.item.ID)
		case livenessBlockedStartToClose:
			cfg.logf("liveness: blocked-timeout %s — start-to-close wall cap for tier %s exceeded", o.item.ID, o.tier)
			landOne(cfg, loop, Result{Item: o.item, Verdict: VerdictBlocked, RunnerID: cfg.RunnerID})
			cfg.journalEvent(loop.Name(), journalLand, o.item, o.tier.String(), VerdictBlocked, "liveness: start-to-close wall cap exceeded — blocked-timeout, filed and continued")
			delete(inflight, o.item.ID)
			tracker.unregister(o.item.ID)
			parked[o.item.ID] = true
		}
	}
}
