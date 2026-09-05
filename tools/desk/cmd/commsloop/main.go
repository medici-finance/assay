// Command commsloop is the DRAIN half of the cell gateway: the fifth consumer
// of the frozen loopengine.Loop contract
// (Name/SelectQueue/TierPolicy/Dispatch/Land/OnIdle — see loop.go), reading
// the SAME accepted-queue ../commsgw writes (a separate process, agreeing on
// disk via internal/commsqueue) and landing every accepted message exactly
// once: report-class messages land done+journaled with no session ever
// fired; everything else quarantines until the (not-yet-landed) prose router
// lands.
//
// This file also carries the (action, class, risk) -> Tier assign table
// (assign.go) that the prose router will consult once it exists — a SEPARATE
// concern from the Loop wiring here, kept in this package because both are
// this comms system's "action-routing layer" (see assign.go's doc).
package main

import (
	"fmt"
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
	os.Exit(run(os.Getenv))
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
	loop := &Loop{
		Root:  root,
		Mon:   DirMonitor{Root: root},
		ACL:   &acl,
		Filer: commsqueue.DeskfileIssueFiler{Repo: repo},
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
