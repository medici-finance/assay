package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// run.go — `desksupervise run --interval DUR`: tick forever, honouring the kill switch
// between ticks and exiting cleanly on SIGTERM/SIGINT. Mirrors cmd/deskwt/prune.go's
// `prune --interval` loop shape (runPruneLoop) byte-for-byte in structure, so the two
// desk-tools loop verbs read the same way to an operator.

// cmdRun implements `desksupervise run --interval DUR [--root DIR] [--repo OWNER/NAME]
// [--dry-run]`. It has no fixture flags: a long-running loop is, by construction, a live
// consumer — the Verify table's offline path is `tick`, one sweep at a time.
func cmdRun(args []string) (err error) {
	ac := &auditCtx{verb: "run"}
	defer func() { ac.finalize(err) }()

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	intervalStr := fs.String("interval", "", "sweep every interval (e.g. 5m) — required")
	root := fs.String("root", "", "repo root to read the live claim tool against (default: cwd)")
	repo := fs.String("repo", "", "owner/name of the repo whose dispatch claims to sweep — required")
	limit := fs.Int("limit", 50, "maximum claim payloads per interval (1..500)")
	dryRun := fs.Bool("dry-run", false, "classify and print only on every tick — never release a claim, file an issue, or journal")
	if perr := fs.Parse(args); perr != nil {
		return deskkit.Refused("refused: bad flags: " + perr.Error())
	}
	if fs.NArg() != 0 {
		return deskkit.Refused("refused: run takes no positional arguments")
	}
	if *intervalStr == "" {
		return deskkit.Refused("refused: run needs --interval (e.g. 5m)")
	}
	interval, derr := time.ParseDuration(*intervalStr)
	if derr != nil {
		return deskkit.Refused("refused: --interval " + *intervalStr + " is not a valid duration (e.g. 5m)")
	}
	if interval <= 0 {
		return deskkit.Refused("refused: --interval must be positive: " + *intervalStr)
	}
	if strings.TrimSpace(*repo) == "" {
		return deskkit.Refused("refused: run needs --repo OWNER/NAME")
	}
	if *limit < 1 || *limit > 500 {
		return deskkit.Refused("refused: --limit must be between 1 and 500")
	}
	ac.repo = *repo

	dir := *root
	if dir == "" {
		wd, gerr := os.Getwd()
		if gerr != nil {
			return deskkit.Unverifiable("cannot resolve working directory", gerr)
		}
		dir = wd
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	stop := make(chan struct{})
	go func() { <-sigCh; close(stop) }()

	after := ""
	ticks, alive, reclaimed, blocked, blind, stopped := 0, 0, 0, 0, 0, 0
	for {
		if gErr := deskkit.Guard(); gErr != nil {
			fmt.Fprintf(os.Stdout, "%s desksupervise run: halting loop — %s\n", nowStamp(), gErr.Error())
			ac.detail = runDetail(interval, ticks, alive, reclaimed, blocked, blind, stopped, "halted ("+gErr.Error()+")")
			return gErr
		}

		pol, perr := loadPolicy()
		if perr != nil {
			ac.detail = runDetail(interval, ticks, alive, reclaimed, blocked, blind, stopped, "aborted on policy load error")
			return perr
		}
		now := time.Now().UTC()
		claims, next, cerr := readLiveClaimsPage(*repo, now, *limit, after)
		if cerr != nil {
			ac.detail = runDetail(interval, ticks, alive, reclaimed, blocked, blind, stopped, "aborted on claim read error")
			return cerr
		}
		eligible, recResults, recBlind, rerr := reconcile(claims, liveEligibilitySource(dir), now, *dryRun, doReclaim, doArmRunStop, os.Stdout)
		if rerr != nil {
			return rerr
		}
		results, tickBlind, serr := sweep(eligible, liveObservationSource(houseProbesOnce()), pol, now, *dryRun, doReclaim, doFileBlockedTimeout, doArmRunStop, os.Stdout)
		if serr != nil {
			ac.detail = runDetail(interval, ticks, alive, reclaimed, blocked, blind, stopped, "aborted on sweep error")
			return serr
		}
		pageOnly := after != "" || next != ""
		tickBlind = tickBlind || recBlind || pageOnly
		after = next
		if pageOnly {
			fmt.Fprintf(os.Stderr, "claims-page partial: next_after=%q limit=%d; continuing next interval\n", next, *limit)
		}
		ticks++
		// Write the runtime snapshot for a local reader, atomically, from THIS tick's own
		// results (no re-probe — see snapshotFromSweep). A write failure is logged and the loop
		// continues: status.json is a convenience for a local reader, never a precondition for
		// the observer's own reclaim work.
		if statusPath, spErr := statusJSONPath(); spErr == nil {
			if werr := writeStatusJSON(statusPath, pageSnapshot(results, recResults, pol, now, pageOnly)); werr != nil {
				fmt.Fprintf(os.Stderr, "%s desksupervise run: WARNING: could not write %s: %v\n", nowStamp(), statusPath, werr)
			}
		} else {
			fmt.Fprintf(os.Stderr, "%s desksupervise run: WARNING: could not resolve the status.json path: %v\n", nowStamp(), spErr)
		}
		for _, r := range recResults {
			if r.Blind {
				blind++
			} else {
				stopped++
			}
		}
		for _, r := range results {
			switch {
			case r.Blind:
				blind++
			case r.Action == "RECLAIM-ELIGIBLE":
				reclaimed++
			case r.Action == "BLOCKED-TIMEOUT":
				blocked++
			default:
				alive++
			}
		}
		fmt.Fprintf(os.Stdout, "%s desksupervise run tick %d: %d claim(s), %s\n",
			nowStamp(), ticks, len(results)+len(recResults), tickSummary(tickBlind, len(results)+len(recResults)))

		select {
		case <-stop:
			fmt.Fprintf(os.Stdout, "%s desksupervise run: shutdown signal — exiting after %d tick(s)\n", nowStamp(), ticks)
			ac.detail = runDetail(interval, ticks, alive, reclaimed, blocked, blind, stopped, "stopped by signal")
			return nil
		case <-time.After(interval):
			continue
		}
	}
}

func tickSummary(blind bool, n int) string {
	if blind {
		return fmt.Sprintf("some of %d unresolved (BLIND)", n)
	}
	return fmt.Sprintf("all %d resolved", n)
}

func runDetail(interval time.Duration, ticks, alive, reclaimed, blocked, blind, stopped int, tail string) string {
	return fmt.Sprintf("interval=%s: %d tick(s), alive=%d reclaimed=%d blocked-timeout=%d blind=%d reconciled-stopped=%d, %s",
		interval, ticks, alive, reclaimed, blocked, blind, stopped, tail)
}

func nowStamp() string { return time.Now().UTC().Format(time.RFC3339) }

// houseProbesOnce memoises ONE process-lifetime *loopengine.ObservableProbes across every
// tick of a `run --interval` loop — load-bearing for BranchProbe's in-process SHA memory
// (see probes.go's file doc): a fresh HouseProbes() every tick would hand BranchProbe a
// fresh, empty BranchSHAStore each time and defeat its own "SHA changed since last tick"
// comparison. `tick` (one sweep, one process) has no such loop to memoise across and calls
// loopengine.HouseProbes() directly instead (see tick.go).
var sharedHouseProbes = loopengine.HouseProbes()

func houseProbesOnce() *loopengine.ObservableProbes { return sharedHouseProbes }

func pageSnapshot(results []sweepResult, reconciled []reconcileResult, pol loopengine.LivenessPolicy, now time.Time, partial bool) StatusSnapshot {
	// Preserve reconciliation uncertainty in the snapshot even though those
	// claims were excluded from the liveness sweep.
	all := append([]sweepResult(nil), results...)
	for _, r := range reconciled {
		if !r.Blind {
			continue
		}
		all = append(all, sweepResult{Claim: r.Claim, Blind: true, Via: r.Source, Action: r.Action})
	}
	snapshot := snapshotFromSweep(all, pol, now)
	if partial {
		snapshot.Aggregates.BlindSources = append(snapshot.Aggregates.BlindSources, "partial claim page; other claims not measured this interval")
	}
	return snapshot
}
