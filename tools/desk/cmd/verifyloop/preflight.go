package main

// preflight.go — verifyloop's BOOT gate.
//
// verifyloop is the reference consumer of the drain engine, so it is also the
// reference consumer of the operating-envelope preflight: this file is what an
// adopting desk copies.
//
// The property being demonstrated is narrow and total. The preflight runs BEFORE
// the Awaiting queue is read. If it is not green the loop prints ONE line and
// exits 6 — it does not read the queue, does not claim an item, does not dispatch,
// and does not file an issue about its own envelope. Four of the five checks
// exist because a live desk discovered that failure three quarters of the way
// through a pass and spent the rest of the pass writing the issue instead of the
// work.
//
// There is no --skip-preflight and no env silencer. A desk that may switch its
// own envelope check off has an envelope check only on the passes that would
// have been green anyway.

import (
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/runnertable"
)

// verifyloopRole is the desk role this loop operates as. It is fixed, not a
// flag: the whole point of the identity checks is that the role is a property of
// the tool's App credentials, not something a caller asserts about itself.
const verifyloopRole = "verifier"

// preflightBoot runs the envelope check for the verify desk. A nil return means
// every check is checked-clean; anything else is the ONE-LINE could-not-run
// refusal the caller prints before exiting.
func preflightBoot(args []string) error {
	// MULTI-ROOT plan: the envelope check is PER ROOT and runs inside cmdPlan, where a RED
	// sibling is reported and skipped rather than aborting the whole pass (a desk booted on
	// one root must still see the other roots' queues). Only the root-independent runner
	// table is validated here.
	if !multiRootPlan(args) {
		if err := preflightRoot(rootFlagValue(args)); err != nil {
			return err
		}
	}
	// When a tier→runner table is configured, validate it at BOOT — an
	// invalid table, or a reachable tier with no runner, is a could-not-run refusal for the
	// whole pass, never a dispatch-time surprise. No table configured => legacy path,
	// this is a no-op.
	return preflightRunnerTable(os.Getenv)
}

// preflightRunnerTable loads and validates the tier→runner table against the
// verify loop's reachable tiers at boot, when one is configured. It is inert (returns nil) when
// no ASSAY_RUNNER_* key is present — additive-and-inert-by-default, matching the legacy gating.
func preflightRunnerTable(getenv func(string) string) error {
	if !runnertable.RunnerTableConfigured(getenv) {
		return nil
	}
	table, err := runnertable.LoadRunnerTable(getenv, nil)
	if err != nil {
		return err
	}
	// Reachable set is the default loop configuration's (TierLocal; TierSession only if the
	// middle-rung flag is on — off by default). The live-window cutover that flips that flag
	// owns re-validating against its own reachable set.
	return table.ValidateReachable((&VerifyLoop{}).reachableTiers())
}

// preflightRoot runs the five envelope checks against ONE root and returns its one-line
// could-not-run refusal, or nil when every check is passing. It is a package var ONLY as a
// test seam (a fixture root has no App credential to mint); production is the real
// deskkit preflight, unchanged from what the single-root boot has always run.
var preflightRoot = func(root string) error {
	rep := deskkit.PreflightRequest{
		Role:    verifyloopRole,
		Root:    root,
		Landing: deskkit.Landing{Dir: root, Remote: "origin"},
	}.Run()
	return rep.Err()
}

// multiRootPlan reports whether this plan spans EVERY configured stream root: DESK_ROOTS
// is set AND no explicit --root narrows the pass to one. With DESK_ROOTS unset the plan is
// the single-root read it has always been (the default `.`); an explicit --root always wins,
// so a desk that wants one root asks for it by name.
func multiRootPlan(args []string) bool {
	return !rootFlagGiven(args) && strings.TrimSpace(os.Getenv(deskkit.RootsEnv)) != ""
}

// rootFlagGiven reports whether --root was passed at all (in any spelling rootFlagValue
// accepts). It PEEKS for the same reason rootFlagValue does: the preflight and the plan must
// agree on whether the pass is single- or multi-root.
func rootFlagGiven(args []string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, "--root=") || strings.HasPrefix(a, "-root=") || a == "--root" || a == "-root" {
			return true
		}
	}
	return false
}

// rootFlagValue peeks --root out of the subcommand args so the preflight scans
// the same tree the plan will. It PEEKS rather than parses because the real flag
// parse belongs to cmdPlan; a second FlagSet that disagreed with the first about
// the root is how a preflight ends up checking a different repo than the pass runs in.
func rootFlagValue(args []string) string {
	for i, a := range args {
		if v, ok := strings.CutPrefix(a, "--root="); ok {
			return v
		}
		if v, ok := strings.CutPrefix(a, "-root="); ok {
			return v
		}
		if (a == "--root" || a == "-root") && i+1 < len(args) {
			return args[i+1]
		}
	}
	return "."
}
