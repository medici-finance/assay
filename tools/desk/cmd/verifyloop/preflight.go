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
// work (#794, #571, #823, #638, #679).
//
// There is no --skip-preflight and no env silencer. A desk that may switch its
// own envelope check off has an envelope check only on the passes that would
// have been green anyway.

import (
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// verifyloopRole is the desk role this loop operates as. It is fixed, not a
// flag: the whole point of the identity checks is that the role is a property of
// the tool's App credentials, not something a caller asserts about itself.
const verifyloopRole = "verifier"

// preflightBoot runs the envelope check for the verify desk. A nil return means
// every check is checked-clean; anything else is the ONE-LINE could-not-run
// refusal the caller prints before exiting.
func preflightBoot(args []string) error {
	root := rootFlagValue(args)
	rep := deskkit.PreflightRequest{
		Role:    verifyloopRole,
		Root:    root,
		Landing: deskkit.Landing{Dir: root, Remote: "origin"},
	}.Run()
	return rep.Err()
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
