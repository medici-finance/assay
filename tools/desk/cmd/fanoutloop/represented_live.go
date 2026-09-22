package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// represented_live.go — the LIVE wiring of `plan`'s already-represented PR-list transport.
//
// represented.go declares `representedPRs` as a nil-by-default package var (the shipped offline
// build performs no forge read and `plan` offers rows as before). #1458 left it deferred behind
// "the typed open+merged-changes op is the cutover work"; that op now exists
// (deskkit.ListChanges / deskkit.RepresentedPRRefs), so main() assigns liveRepresentedPRs to it —
// the same shape deskdispatch's phantom check is wired with. Tests call cmdPlan/run directly (not
// main()) and keep the nil default, so the offline reference build and every test stay inert
// unless they wire their own recorded transport.
//
// The read goes through the typed Forge seam (never a forge CLI), so activating it does not
// re-open the closed forge surface (internal/forgeban; TestNoForgeCLIShellout). The forge is
// resolved through productionForgeResolver — the session's own role, no ambient-credential
// fallback — exactly as the claim-release sink resolves its forge.
func liveRepresentedPRs(repo string) ([]deskkit.PRRef, error) {
	owner, name, ok := strings.Cut(strings.TrimSpace(repo), "/")
	if !ok || owner == "" || name == "" {
		return nil, fmt.Errorf("represented-PR read: target repo %q is not owner/name", repo)
	}
	fg, ferr := productionForgeResolver(deskkit.ForgeRepo{Owner: owner, Name: name})
	if ferr != nil {
		return nil, ferr
	}
	refs, incomplete, rerr := deskkit.RepresentedPRRefs(fg, deskkit.ForgeRepo{Owner: owner, Name: name})
	if rerr != nil {
		return nil, rerr
	}
	if incomplete {
		// The seam read the most-recently-updated window up to its page ceiling and the repo holds
		// more open+merged changes than that. Reported, not swallowed: the window is ordered so a
		// currently-queued brief's representing PR (open, or merged-but-row-not-reconciled) is
		// within it, so `plan` reconciles against the recent window rather than HOLDING the whole
		// fresh lane on a repo whose lifetime PR count exceeds the ceiling. A transport ERROR
		// (above) still holds the fresh lane — could-not-check is not no-PR-exists.
		fmt.Fprintf(os.Stderr, "fanoutloop: NOTICE — %s has more open+merged changes than the "+
			"represented-PR read's recent-window ceiling; plan reconciled against the most-recently-"+
			"updated window and a representing PR older than that window would not be seen\n", repo)
	}
	return refs, nil
}
