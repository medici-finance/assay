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
func liveRepresentedPRs(repo string) ([]deskkit.PRRef, bool, error) {
	fr, err := forgeRepoOf(repo)
	if err != nil {
		return nil, false, err
	}
	fg, _, rerr := deskkit.ResolveForge(fr, deskkit.DispatcherRole)
	if rerr != nil {
		return nil, false, rerr
	}
	return representedWindow(fg, repo, fr)
}

// representedWindow is the transport's body past forge resolution, split out so tests drive the
// exact reduction the shipped binary runs: the window and its completeness, never one without the
// other.
func representedWindow(fg deskkit.Forge, repo string, fr deskkit.ForgeRepo) ([]deskkit.PRRef, bool, error) {
	refs, incomplete, rerr := deskkit.RepresentedPRRefs(fg, fr)
	if rerr != nil {
		return nil, false, rerr
	}
	if incomplete {
		// The seam read the most-recently-updated window up to its page ceiling and the repo
		// holds more open+merged changes than that. The flag is RETURNED, not just reported: a
		// brief whose delivering PR is older than the window is absent from it, and absence from
		// an incomplete list is could-not-check (#5853). phantomCheckFor looks a brief the window
		// missed up by its trailer (liveBriefChangeLookup) rather than dispatching it as new work.
		fmt.Fprintf(os.Stderr, "deskdispatch: NOTICE — %s has more open+merged changes than the "+
			"phantom check's recent-window ceiling; a brief the window does not represent is looked "+
			"up by its `Brief:` trailer\n", repo)
	}
	return refs, incomplete, nil
}

// liveBriefChangeLookup is lookupBriefChanges' shipped transport: deskkit.ChangesByBriefReader over
// the dispatcher role's forge, the complete body-marker search the decision-gate discovery uses. A
// backend without that search is an error, which the phantom check turns into could-not-check.
func liveBriefChangeLookup(repo, id string) ([]deskkit.PRRef, error) {
	fr, err := forgeRepoOf(repo)
	if err != nil {
		return nil, err
	}
	fg, _, rerr := deskkit.ResolveForge(fr, deskkit.DispatcherRole)
	if rerr != nil {
		return nil, rerr
	}
	lookup := deskkit.ChangesByBriefReader(fg)
	if lookup == nil {
		return nil, fmt.Errorf("this forge backend has no `Brief:` trailer search")
	}
	return lookup(fr, id)
}
