package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// run.go — the drain.
//
// One pass: arm the monitor if it is not armed (arming and draining are the same act — the first
// cycle seeds silently and every later one reports the delta), apply the trust gate at the queueing
// boundary, claim, execute the lanes through the seam, and record one tracked exit per item. Stop
// flags are honoured at the pass boundary AND at every item boundary, with the precedence
// DISABLED > STOP > STOP.<loop>, so a human halting this drain never waits for a pass to finish.
//
// WHY A PASS AND NOT A STANDING CONDUCTOR. The engine's own Run() is a standing loop: it never
// exits on an empty queue, it idle-polls, and a stop flag is its only way out. That is right for a
// drain whose cadence it owns — and wrong here, because this drain's cadence is the inbound
// poller's. A Go idle-poll beside it would be a second clock for one surface, and two clocks
// disagree. So the pass drives the SAME six hooks in the engine's documented order and through the
// engine's own claim gate: dedupe-at-start still holds, because it is Claim() that extends the
// guarantee, not the shape of the loop around it.
//
// The in-flight cap is ONE, and it is a constant rather than a knob: the scan-carrier lane writes to
// a single scan branch and a single PR, so two concurrent lanes would race on the same head. The
// judgment lane inherits the cap because interleaving an emitted routing question with a carrier
// push would move the head under whoever is answering it.

func cmdRun(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var o planOptions
	o.bind(fs, true)
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused("scanloop run: bad flags: " + err.Error())
	}
	now, err := o.now()
	if err != nil {
		return err
	}
	if err := deskkit.ScanScopeError(); err != nil {
		return err
	}
	scope := deskkit.ScanRepos()

	fmt.Fprintf(stdout, "scanloop run — intake-desk drain\n")
	fmt.Fprintf(stdout, "scan scope: %d repo(s) — %s\n", len(scope), strings.Join(scope, ", "))

	stateDir := o.resolvedStateDir()
	state, serr := ReadMonitorState(stateDir, scope)
	if serr != nil {
		return serr
	}
	fmt.Fprintf(stdout, "monitor: %s (%s)\n", armingLine(state), state.Dir)

	// The queue source. Offline mode takes the pass's events from a captured poll and does not
	// touch the poller at all — that is the CI-testable surface, and it is also what a session
	// already holding the harness monitor's output wants.
	var source func() (*MonitorReport, error)
	if o.offline {
		source = func() (*MonitorReport, error) {
			if strings.TrimSpace(o.inbound) == "" {
				return nil, deskkit.Refused("scanloop run --offline: --inbound is required. " +
					"An offline run with no captured poll is BLIND, and blind is not idle.")
			}
			raw, rerr := readInput(o.inbound)
			if rerr != nil {
				return nil, deskkit.Unverifiable("scanloop run: cannot read the captured monitor output", rerr)
			}
			return ParseMonitorOutput(string(raw)), nil
		}
	} else {
		script, ferr := FindMonitorScript(o.root, o.monitor)
		if ferr != nil {
			return ferr
		}
		fmt.Fprintf(stdout, "poller: %s (wrapped, never copied)\n", script)
		source = func() (*MonitorReport, error) {
			// Running the poller IS the arming step: with no baseline it seeds silently, and the
			// pass that seeds deliberately reports no inbound rather than replaying the whole
			// backlog as new work.
			return RunMonitor(script, stateDir, scope, nil)
		}
	}

	probe := TrustProbe(nil)
	if !o.offline {
		probe = ghTrustProbe(nil)
	}

	worktreeBase := o.worktrees
	if worktreeBase == "" {
		abs, aerr := filepath.Abs(o.root)
		if aerr != nil {
			return deskkit.Unverifiable("scanloop run: cannot resolve --root", aerr)
		}
		worktreeBase = filepath.Dir(abs)
	}
	if !filepath.IsAbs(worktreeBase) {
		return deskkit.Refused("scanloop run: --worktree-base must be an ABSOLUTE path (" + worktreeBase + ")")
	}

	openPR := o.openScanPR()
	loop := &ScanLoop{
		Root:         o.root,
		WorktreeBase: worktreeBase,
		Policy:       CoalescePolicy{Window: o.window},
		Scope:        scope,
		Monitor:      source,
		Probe:        probe,
		OpenPR:       func() (*OpenScanPR, error) { return openPR, nil },
		Emit:         stdout,
		DryRun:       o.dryRun,
		Now:          func() time.Time { return now },
	}

	claimsDir, cerr := deskkit.StateDir()
	if cerr != nil {
		return deskkit.Unverifiable("scanloop run: cannot resolve the desk state dir (HOME missing?)", cerr)
	}

	cfg := loopengine.Config{
		PoolSize:   1,
		IdlePoll:   time.Minute, // unused by the pass; a sane value keeps the config coherent
		ClaimsDir:  filepath.Join(claimsDir, "claims"),
		StaleClaim: deskkit.DefaultStaleClaim,
		Progress:   stdout,
	}

	rerr := drainPass(cfg, loop, stdout)

	// The pass report is printed whether or not the drain errored: a drain that failed halfway
	// still has to say what left and what did not.
	renderPassReport(stdout, loop)

	if rerr != nil {
		return rerr
	}
	if leaked := loop.Ledger().Unexited(loop.exited()); len(leaked) > 0 {
		return deskkit.Unverifiable("scanloop run: "+fmt.Sprint(len(leaked))+
			" dispatched item(s) landed with no tracked exit — the front door leaked: "+strings.Join(leaked, ", "), nil)
	}
	if rep := loop.Report(); rep != nil && rep.Blind() {
		return deskkit.Unverifiable("scanloop run: the inbound surface was not fully readable this pass "+
			"(degraded repos or a suppressed burst) — blind is not idle", nil)
	}
	return nil
}

// drainPass is ONE pass of the drain, driving the six hooks in the engine's documented order and
// through the engine's own claim gate.
//
// Failure handling is per-item, not per-pass: one item's lane failing must not strand the rest of
// the queue behind it. The first error is remembered and returned after the report, so the pass's
// exit code still carries the failure while the operator still gets the ledger.
func drainPass(cfg loopengine.Config, loop *ScanLoop, w io.Writer) error {
	// Guard is the FIRST action, before any surface is read.
	if err := deskkit.Guard(); err != nil {
		return err
	}
	items, err := loop.SelectQueue()
	if err != nil {
		return err
	}

	var firstErr error
	keep := func(e error) {
		if firstErr == nil {
			firstErr = e
		}
	}

	for _, it := range items {
		// Every item is an iteration boundary: an operator arming a stop flag mid-pass is obeyed
		// at the next item, never mid-action.
		if gerr := deskkit.Guard(); gerr != nil {
			fmt.Fprintf(w, "stop: %s — %d item(s) not dispatched this pass\n", gerr.Error(), len(items))
			if deskkit.IsDisabled(gerr) {
				break
			}
			return gerr // an unverifiable guard state fails closed
		}

		tier, terr := loop.TierPolicy(it)
		if terr != nil {
			fmt.Fprintf(w, "tier-error: %s: %v\n", it.ID, terr)
			keep(terr)
			continue
		}

		// Claim is the SINGLE entry point for dispatch: it is what extends the
		// dispatched-at-most-once guarantee across every consumer sharing the claims dir. A
		// could-not-check claim fails closed and is never read as "assume free".
		acquired, cerr := loopengine.Claim(cfg, it)
		if cerr != nil {
			keep(cerr)
			continue
		}
		if !acquired {
			fmt.Fprintf(w, "dedup: %s — a live claim is held elsewhere; not dispatched\n", it.ID)
			continue
		}

		h, derr := loop.Dispatch(it, tier)
		if derr != nil {
			_ = loopengine.ReleaseClaim(cfg, it)
			var awaiting *awaitingRoutingError
			if errors.As(derr, &awaiting) {
				// EMITTED for a model tier and parked. Not a failure and not an exit: the item is
				// named in the report so it cannot be mistaken for one that drained.
				loop.park(it.ID)
				continue
			}
			fmt.Fprintf(w, "dispatch-error: %s: %v\n", it.ID, derr)
			keep(derr)
			continue
		}

		result := <-h.Done()
		if lerr := loop.Land(result); lerr != nil {
			fmt.Fprintf(w, "land-error: %s: %v\n", it.ID, lerr)
			keep(lerr)
		}
		_ = loopengine.ReleaseClaim(cfg, it)
	}

	if oerr := loop.OnIdle(); oerr != nil {
		keep(oerr)
	}
	return firstErr
}

func renderPassReport(w io.Writer, loop *ScanLoop) {
	adm := loop.Admissions()
	admitted, quarantined, unknown := AdmissionCounts(adm)
	fmt.Fprintf(w, "\nTRUST GATE: %d admitted · %d quarantined · %d could-not-check\n", admitted, quarantined, unknown)
	for _, a := range adm {
		if a.Admitted() {
			continue
		}
		fmt.Fprintf(w, "  %-28s %-16s %s\n", a.Item.ID(), a.State, a.Why)
	}

	fmt.Fprintf(w, "\nEXIT LEDGER (one tracked exit per item):\n")
	records := loop.Ledger().Records()
	if len(records) == 0 {
		fmt.Fprintf(w, "  (nothing landed this pass)\n")
	}
	for _, r := range records {
		fmt.Fprintf(w, "  %-28s %-18s %-16s %s\n", r.ItemID, r.Exit, r.Lane, r.Artifact)
	}
	counts := loop.Ledger().CountByExit()
	var parts []string
	for _, e := range trackedExits {
		parts = append(parts, fmt.Sprintf("%d %s", counts[e], e))
	}
	fmt.Fprintf(w, "  totals: %s\n", strings.Join(parts, " · "))

	// Items EMITTED for a model tier are named separately. They are neither landed nor lost, and
	// folding them into either count would hide a decision that has not been made yet.
	if parked := loop.Parked(); len(parked) > 0 {
		fmt.Fprintf(w, "\nEMITTED — awaiting a routing decision from the model tier (%d):\n", len(parked))
		for _, id := range parked {
			fmt.Fprintf(w, "  %-28s route to exactly one of: %s\n", id, strings.Join(exitNames(), " · "))
		}
		fmt.Fprintf(w, "  these are NOT recorded as exited — the item leaves the front door when its exit is recorded, not when it is emitted.\n")
	}

	// The leak check prints on EVERY pass, clean or not: a check that is silent when it passes is
	// indistinguishable from a check that did not run.
	leaked := loop.Ledger().Unexited(loop.exited())
	if len(leaked) == 0 {
		fmt.Fprintf(w, "  leak check: CLEAN — every dispatched item left by a tracked exit\n")
	} else {
		fmt.Fprintf(w, "  leak check: %d item(s) with NO exit — %s\n", len(leaked), strings.Join(leaked, ", "))
	}

	if rep := loop.Report(); rep != nil {
		for _, d := range rep.Degraded {
			fmt.Fprintf(w, "  DEGRADED: %s\n", d)
		}
		for _, b := range rep.Bursts {
			fmt.Fprintf(w, "  BURST (items suppressed, not enumerable this pass): %s\n", b)
		}
	}
}
