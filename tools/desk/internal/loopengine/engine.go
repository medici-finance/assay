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
	ID          string            // e.g. "loop-engine/01" or "issue-841" — stable, claim key
	BriefPath   string            // repo-relative brief file, when brief-shaped
	TargetSHA   string            // merged-main SHA the dispatched worker runs against
	Risk        RiskFlags         // parsed frontmatter
	Gate        string            // "model" | "human"
	Effort      string            // S | M | L
	ExecTier    string            // "any" | "strong"
	Implementer string            // for the author != runner typed guard (CheckAuthorRunner)
	Payload     map[string]string // template variables for the dispatch prompt
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
	// guard (the adapter is then responsible, e.g. batch-fanout where authorship differs
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
	// Honor per-loop stop flags (STOP.<name>) by scoping DESK_LOOP to this loop when the
	// operator has not already set it (the skill boot does; be robust if it did not).
	if os.Getenv("DESK_LOOP") == "" {
		_ = os.Setenv("DESK_LOOP", loop.Name())
	}

	inflight := map[string]Handle{} // item ID -> in-flight handle
	parked := map[string]bool{}     // item IDs routed-to-human / refused this session (do not re-select)
	completions := make(chan completion)
	stopping := false

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
			if err := fillPool(cfg, loop, inflight, parked, completions); err != nil {
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
			delete(inflight, c.result.Item.ID)
			if failed := landOne(cfg, loop, c.result); failed {
				parked[c.result.Item.ID] = true
			}
			continue
		}
		select {
		case c := <-completions:
			delete(inflight, c.result.Item.ID)
			if failed := landOne(cfg, loop, c.result); failed {
				// Bug filed (FILE don't ask); park it so the drain does not respin on a
				// known-unlandable item and can proceed to empty.
				parked[c.result.Item.ID] = true
			}
			// refill happens on the next iteration's fillPool — immediate, not wave-batched.
		case <-time.After(cfg.IdlePoll):
			// Periodic wake so a stop flag set while the pool is busy is noticed at the
			// next boundary even if no completion arrives.
		}
	}
}

// fillPool claims + dispatches eligible items until the pool is at cfg.PoolSize. Items
// already in flight or parked (routed-human / author-refused this session) are skipped;
// a claim collision is skipped (owned elsewhere — structural no-double-dispatch).
func fillPool(cfg Config, loop Loop, inflight map[string]Handle, parked map[string]bool, completions chan<- completion) error {
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
			_ = deskkit.Log(deskkit.Entry{Tool: "loopengine", Verb: "dispatch", Result: deskkit.ResultRefused, Detail: err.Error()})
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

		h, err := loop.Dispatch(it, tier)
		if err != nil {
			cfg.logf("dispatch-error: %s: %v", it.ID, err)
			_ = ReleaseClaim(cfg, it) // hand the slot back; a failed dispatch is not a claim
			parked[it.ID] = true
			continue
		}
		inflight[it.ID] = h
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
		_ = deskkit.Log(deskkit.Entry{Tool: "loopengine", Verb: "land", Result: deskkit.ResultRefused, Detail: fmt.Sprintf("land failed for %s: %v — filed and continued", r.Item.ID, err)})
		return true
	}
	cfg.logf("landed: %s (%s)", r.Item.ID, r.Verdict)
	return false
}
