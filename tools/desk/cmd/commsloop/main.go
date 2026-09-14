// Command commsloop is the DRAIN half of the cell gateway: the fifth consumer
// of the frozen loopengine.Loop contract
// (Name/SelectQueue/TierPolicy/Dispatch/Land/OnIdle — see loop.go), reading
// the SAME accepted-queue ../commsgw writes (a separate process, agreeing on
// disk via internal/commsqueue) and landing every accepted message exactly
// once: EVERY accepted message is routed by the contained prose consult
// (decide.go) — there is no deterministic routing table and no fast path
// (#1767 ruling 3) — and lands done+journaled or quarantined per the
// consult's action and assign.go's compiled (action, class, risk) -> Tier
// table.
//
// This file also carries the (action, class, risk) -> Tier assign table
// (assign.go) the prose router (decide.go) consults through — a SEPARATE
// concern from the Loop wiring here, kept in this package because both are
// this comms system's "action-routing layer" (see assign.go's doc).
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
	"github.com/medici-finance/assay/tools/desk/internal/commsqueue"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// EnvQueueDir names the SAME accepted-queue root commsgw writes
// (ASSAY_COMMS_QUEUE_DIR) — the two binaries must be pointed at one directory.
const EnvQueueDir = "ASSAY_COMMS_QUEUE_DIR"

// EnvRepo names the owner/repo commsloop's own quarantine issues are filed
// against.
const EnvRepo = "ASSAY_COMMS_REPO"

// EnvCell names this cell (ASSAY_COMMS_CELL — the same key
// ../commsgw/config.go's EnvCell reads), used only to attribute a router
// containment-anomaly filing to a cell; absent leaves that attribution blank,
// never a boot refusal (the router's fail-closed posture does not depend on
// knowing the cell name).
const EnvCell = "ASSAY_COMMS_CELL"

// idlePollCadence is the steady-state idle-poll cadence on the empty
// accepted-queue (loopengine.Config.IdlePoll). A zero value here makes
// loopengine.Run's idle branch call time.Sleep(0) / time.After(0) in a tight
// loop — a CPU spin plus a stderr "idle" flood — every cycle the
// accepted-queue is empty, which is the steady state for this drain
// consumer. Pinned as a named constant (rather than inlined) so
// main_test.go can assert on the exact value the production wiring uses;
// see cmd/scanloop/run.go's IdlePoll: time.Minute for the sibling pattern.
const idlePollCadence = time.Minute

func main() {
	os.Exit(dispatch(os.Args[1:], os.Getenv, os.Stdout))
}

// dispatch is main's testable body ahead of the drain-loop/sweep split: with
// no args (production wiring) it is byte-identical to calling run(getenv)
// directly — the standing drain loop. "sweep" routes to the daily
// lane-violation sweep (sweep.go) instead; every other
// first argument is refused rather than silently falling back to the drain
// loop, so a typo'd subcommand cannot be mistaken for "start the loop".
func dispatch(args []string, getenv func(string) string, stdout io.Writer) int {
	if len(args) == 0 {
		return run(getenv)
	}
	switch args[0] {
	case "sweep":
		err := cmdSweep(args[1:], getenv, stdout)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		return exitCodeOf(err)
	default:
		fmt.Fprintf(os.Stderr, "commsloop: unknown subcommand %q (want: sweep, or no args for the drain loop)\n", args[0])
		return deskkit.ExitRefused
	}
}

// run is main's testable body: it never calls os.Exit itself.
func run(getenv func(string) string) int {
	root := strings.TrimSpace(getenv(EnvQueueDir))
	if root == "" {
		fmt.Fprintf(os.Stderr, "commsloop: inert — %s is not set (config-off default: no queue root, no drain)\n", EnvQueueDir)
		return deskkit.ExitRefused
	}
	repo := strings.TrimSpace(getenv(EnvRepo))
	if repo == "" {
		repo = "medici-finance/assay"
	}

	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitCodeOf(err)
	}

	claimsDir, err := deskkit.StateDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, deskkit.Unverifiable("commsloop: cannot resolve the desk state dir (HOME missing?)", err))
		return deskkit.ExitUnverifiable
	}

	acl := comms.Compiled()
	filer := commsqueue.DeskfileIssueFiler{Repo: repo}
	cell := strings.TrimSpace(getenv(EnvCell))

	// The inbound prose router is consulted for every accepted message
	// (decide.go). Its contained advisor is wired from the pinned decider
	// runner entry (brief 06) when one is configured; a configured-but-unsafe
	// entry refuses to boot here (containment never silently degrades), and
	// an unconfigured one leaves the valve off so every message quarantines
	// (fail closed) — mirrors ../commsgw's outbound NewGate wiring exactly.
	router, err := NewRouter(getenv, cell, filer)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitCodeOf(err)
	}

	loop := &Loop{
		Root:   root,
		Mon:    DirMonitor{Root: root},
		ACL:    &acl,
		Filer:  filer,
		Router: router,
	}

	cfg := loopengine.Config{
		PoolSize:   1,
		IdlePoll:   idlePollCadence,
		ClaimsDir:  filepath.Join(claimsDir, "claims"),
		StaleClaim: deskkit.DefaultStaleClaim,
		Progress:   os.Stderr,
	}

	if err := loopengine.Run(cfg, loop); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitCodeOf(err)
	}
	return 0
}

func exitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	if de, ok := deskErrorOf(err); ok {
		return de
	}
	return 1
}

func deskErrorOf(err error) (int, bool) {
	type exitCoder interface{ ExitCode() int }
	if ec, ok := err.(exitCoder); ok {
		return ec.ExitCode(), true
	}
	return 0, false
}
