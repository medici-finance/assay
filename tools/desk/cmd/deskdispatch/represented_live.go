package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// represented_live.go — the LIVE wiring of the phantom check's PR-list transport (phantom.go).
//
// phantom.go declares `listRepresentedPRs` as a nil-by-default package var: nil = the offline
// reference build performs no forge read. main() assigns liveRepresentedPRs to it so the shipped
// binary's fresh-worker dispatch is guarded, while tests — which call run()/cmdDispatch directly,
// never main() — keep the nil default (inert) unless they wire their own recorded transport.
//
// The read goes through the typed Forge seam (deskkit.RepresentedPRRefs → Forge.ListChanges),
// never a forge CLI, so the check going live does not re-open the closed forge surface
// (internal/forgeban; TestNoForgeCLIShellout). The role is the desk dispatcher role — the
// identity that dispatches a fresh worker — resolved through the same ResolveForge seam the
// stamp step uses.
func liveRepresentedPRs(repo string) ([]deskkit.PRRef, error) {
	fr, err := forgeRepoOf(repo)
	if err != nil {
		return nil, err
	}
	fg, _, rerr := deskkit.ResolveForge(fr, deskkit.DispatcherRole)
	if rerr != nil {
		return nil, rerr
	}
	refs, incomplete, rerr := deskkit.RepresentedPRRefs(fg, fr)
	if rerr != nil {
		return nil, rerr
	}
	if incomplete {
		// The seam read the most-recently-updated window up to its page ceiling and the repo
		// holds more open+merged changes than that. This is reported, not swallowed: a brief
		// whose representing PR fell outside the recent window is the one a phantom check has
		// least reason to find (a currently-queued brief's PR is recently active), so the
		// dispatch proceeds on the window rather than HOLDING every fresh dispatch on a repo
		// whose lifetime merged-PR count exceeds the ceiling — but the operator is told the
		// window was bounded, so a surprising non-refusal is diagnosable rather than silent.
		fmt.Fprintf(os.Stderr, "deskdispatch: NOTICE — %s has more open+merged changes than the "+
			"phantom check's recent-window ceiling; the check ran against the most-recently-updated "+
			"window and a representing PR older than that window would not be seen\n", repo)
	}
	return refs, nil
}
