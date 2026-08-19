package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// VerifyLoop is the verify-desk reference consumer of the drain engine. It
// implements loopengine.Loop; every scheduling fact (pool, claims, refill, land-as-returned,
// author!=runner, guard) lives in the engine — this adapter contributes only the four
// heterogeneous hooks (SelectQueue / TierPolicy / Dispatch / Land) plus OnIdle.
//
// There is NO inline-verify path: the only way a brief leaves the Awaiting queue is through
// the engine's Dispatch. The inline-verify failure mode (inline verification, zero durable artifacts)
// is unrepresentable here, not merely discouraged.
//
// Honest bound (arch doc §1.1): this buys ATTRIBUTION, AUDIT, and STRUCTURAL SEPARATION — the
// Evidence rows are rendered from the dispatched runner's structured Result, not free text —
// NOT cryptographic un-forgeability. A holder of signing material can still forge; the close
// is author!=approver between distinct Apps + branch protection, which is out of scope here.
type VerifyLoop struct {
	Root      string // repo root the Awaiting scan runs against
	TargetSHA string // merged-main SHA verifiers run against (stamped onto every Item)
	RunnerID  string // this session's identity (engine's author!=runner left-hand side)

	// F16ReversibleRiskToSession is the arch-doc §9.2 middle rung — the owner's OPEN decision, left
	// OFF. Flipping it to true is the ENTIRE change to restore the session-tier
	// verification for risk-flagged-but-reversible briefs (see tier.go).
	F16ReversibleRiskToSession bool

	// Emit is where interim-mode dispatch instructions are printed (the operator executes them
	// verbatim as Agent calls). nil = stdout.
	Emit io.Writer
	// Feeder obtains the structured Result for a dispatched item (interim mode: the operator
	// feeds it back). nil means no result path is wired — the autonomous cutover is
	// BLOCKED-ON-HUMAN, so Dispatch refuses rather than pretend to run.
	Feeder func(loopengine.Item, loopengine.Tier, string) (loopengine.Result, error)
	// DurableSink makes results durable. nil defaults to the SAFE dry-run sink (must
	// not push to main). The real git-pushing sink is wired only at cutover.
	DurableSink Durable
	// Now is injectable for deterministic Evidence dates in tests.
	Now func() time.Time

	// --- Native ACP dispatch --------------------------------
	// Native selects the dispatch MODE. false (the zero value, the default) keeps
	// today's interim emit-and-await behaviour BYTE-FOR-BYTE. true drives the
	// dispatched verifier as a real ACP child process over internal/acp. Native
	// dispatch is opt-in ONLY — nothing flips it on implicitly.
	Native bool
	// RunnerTable is the tier→runner map: each dispatchable Tier
	// resolves to a pinned `{cmd, model, pin}` runner. When set it is THE runner
	// surface — native dispatch resolves the runner for the item's tier from it, and
	// Result.RunnerID is derived from the resolved entry (runnertable.go). nil keeps
	// the legacy single-value path below (additive-and-inert-by-default).
	RunnerTable *RunnerTable
	// RunnerCmd is the legacy single runner value, kept as the
	// fallback / test-injection path: a runner that silently reaches the network
	// needs an explicit config value, so native mode with BOTH RunnerTable nil AND
	// RunnerCmd empty refuses to dispatch. When RunnerTable is set it supersedes this
	// (the table replaced the single value as the real runner selection).
	RunnerCmd []string
	// NativeEnv is appended to the spawned runner's environment (e.g. an
	// ANTHROPIC_API_KEY for per-worker billing per the acp README note).
	// Empty inherits the parent environment unchanged.
	NativeEnv []string
	// NativeTimeout caps one native dispatch (spawn→initialize→session→prompt→
	// parse). Zero uses defaultNativeTimeout.
	NativeTimeout time.Duration
	// MakeWorktree creates the isolated worktree a native dispatch runs in, at
	// Item.TargetSHA, returning its dir and a cleanup. nil uses the real
	// `git worktree add --detach` implementation (gitDetachedWorktree). Injected
	// in tests so the native path is exercised without a real clone.
	MakeWorktree func(loopengine.Item) (dir string, cleanup func(), err error)
}

func (v *VerifyLoop) Name() string { return "verify-desk" }

// SelectQueue is the deterministic board read: the statusgen Awaiting filter, tier-1
// (implemented + empty Evidence) before tier-2 (free-closes / Evidence-present), oldest-first
// within class. See briefscan.go.
func (v *VerifyLoop) SelectQueue() ([]loopengine.Item, error) {
	return scanAwaiting(v.Root, v.TargetSHA)
}

// Dispatch is the ONE method the native-primitive upgrade swaps (arch doc §9.1; loopengine
// doc.go: "that swap touches only Dispatch"). Mode is config, not a code path anyone reaches
// by accident: v.Native == false (the default) keeps interim emit-and-await; v.Native == true
// drives a real ACP child process (dispatch_native.go). The rendered prompt is identical
// either way — renderDispatchPrompt is the single source, unchanged.
func (v *VerifyLoop) Dispatch(it loopengine.Item, tier loopengine.Tier) (loopengine.Handle, error) {
	if v.Native {
		return v.dispatchNative(it, tier)
	}
	return v.dispatchInterim(it, tier)
}

// dispatchInterim renders the verifier prompt (Verify table + target SHA + isolation; no
// shared-checkout path), emits the exact instruction, and returns a Handle fed by Feeder.
// This is interim mode (arch doc §9.1): a Go conductor cannot call the Agent tool, so the
// engine EMITS the dispatch and the model executes it verbatim, then feeds the structured
// Result back.
func (v *VerifyLoop) dispatchInterim(it loopengine.Item, tier loopengine.Tier) (loopengine.Handle, error) {
	prompt := renderDispatchPrompt(it, tier)
	if err := assertNoSharedCheckout(prompt); err != nil {
		return nil, err
	}
	fmt.Fprintf(v.emit(), "\n=== DISPATCH %s (tier=%s) ===\n%s\n=== END DISPATCH ===\n", it.ID, tier, prompt)

	if v.Feeder == nil {
		return nil, fmt.Errorf(
			"interim dispatch for %s: no Agent binding and no result feeder wired — the autonomous cutover is BLOCKED-ON-HUMAN (arch doc §9.1)", it.ID)
	}
	done := make(chan loopengine.Result, 1)
	go func() {
		r, err := v.Feeder(it, tier, prompt)
		if err != nil {
			r = loopengine.Result{Item: it, Verdict: loopengine.VerdictBlocked, RunnerID: v.RunnerID}
		}
		if r.Item.ID == "" {
			r.Item = it
		}
		done <- r
	}()
	return &handle{item: it, done: done}, nil
}

// OnIdle refreshes for the next poll. In the reference build it re-scans nothing beyond the
// next SelectQueue; the live wiring re-fetches origin/main here (kept minimal — the cutover
// owns the live refresh).
func (v *VerifyLoop) OnIdle() error { return nil }

func (v *VerifyLoop) emit() io.Writer {
	if v.Emit != nil {
		return v.Emit
	}
	return os.Stdout
}

func (v *VerifyLoop) durable() Durable {
	if v.DurableSink != nil {
		return v.DurableSink
	}
	return dryRunDurable{out: v.emit()}
}

// handle is the interim in-flight tracker: Done() fires when Feeder returns the structured
// Result. Under the native-primitive upgrade it would wrap a real child-worker completion.
type handle struct {
	item loopengine.Item
	done chan loopengine.Result
}

func (h *handle) Done() <-chan loopengine.Result { return h.done }
func (h *handle) Item() loopengine.Item          { return h.item }

// compile-time assertion: VerifyLoop is a valid drain-engine consumer.
var _ loopengine.Loop = (*VerifyLoop)(nil)
