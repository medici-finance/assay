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

	ticks, alive, reclaimed, blocked, blind := 0, 0, 0, 0, 0
	for {
		if gErr := deskkit.Guard(); gErr != nil {
			fmt.Fprintf(os.Stdout, "%s desksupervise run: halting loop — %s\n", nowStamp(), gErr.Error())
			ac.detail = runDetail(interval, ticks, alive, reclaimed, blocked, blind, "halted ("+gErr.Error()+")")
			return gErr
		}

		pol, perr := loadPolicy()
		if perr != nil {
			ac.detail = runDetail(interval, ticks, alive, reclaimed, blocked, blind, "aborted on policy load error")
			return perr
		}
		now := time.Now().UTC()
		claims, cerr := readLiveClaims(dir, *repo, now)
		if cerr != nil {
			ac.detail = runDetail(interval, ticks, alive, reclaimed, blocked, blind, "aborted on claim read error")
			return cerr
		}
		results, tickBlind, serr := sweep(claims, liveObservationSource(houseProbesOnce()), pol, now, *dryRun, doReclaim, doFileBlockedTimeout, os.Stdout)
		if serr != nil {
			ac.detail = runDetail(interval, ticks, alive, reclaimed, blocked, blind, "aborted on sweep error")
			return serr
		}
		ticks++
		// Write the runtime snapshot for a local reader, atomically, from THIS tick's own
		// results (no re-probe — see snapshotFromSweep). A write failure is logged and the loop
		// continues: status.json is a convenience for a local reader, never a precondition for
		// the observer's own reclaim work.
		if statusPath, spErr := statusJSONPath(); spErr == nil {
			if werr := writeStatusJSON(statusPath, snapshotFromSweep(results, pol, now)); werr != nil {
				fmt.Fprintf(os.Stderr, "%s desksupervise run: WARNING: could not write %s: %v\n", nowStamp(), statusPath, werr)
			}
		} else {
			fmt.Fprintf(os.Stderr, "%s desksupervise run: WARNING: could not resolve the status.json path: %v\n", nowStamp(), spErr)
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
			nowStamp(), ticks, len(results), tickSummary(tickBlind, len(results)))

		select {
		case <-stop:
			fmt.Fprintf(os.Stdout, "%s desksupervise run: shutdown signal — exiting after %d tick(s)\n", nowStamp(), ticks)
			ac.detail = runDetail(interval, ticks, alive, reclaimed, blocked, blind, "stopped by signal")
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

func runDetail(interval time.Duration, ticks, alive, reclaimed, blocked, blind int, tail string) string {
	return fmt.Sprintf("interval=%s: %d tick(s), alive=%d reclaimed=%d blocked-timeout=%d blind=%d, %s",
		interval, ticks, alive, reclaimed, blocked, blind, tail)
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
