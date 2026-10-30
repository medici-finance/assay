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

// liveRepresentedPRFiles is representedPRFiles' shipped transport: one PR's changed-file list, read
// through the typed Forge seam under the session's own role and reconciled against the forge's own
// count (deskkit.CompleteChangedFiles) — a list that cannot be proven complete is an error, never a
// partial answer that could hide the code making the PR a delivery.
func liveRepresentedPRFiles(repo string, number int) ([]deskkit.ChangedFile, error) {
	owner, name, ok := strings.Cut(strings.TrimSpace(repo), "/")
	if !ok || owner == "" || name == "" {
		return nil, fmt.Errorf("changed-file read: target repo %q is not owner/name", repo)
	}
	fr := deskkit.ForgeRepo{Owner: owner, Name: name}
	fg, ferr := productionForgeResolver(fr)
	if ferr != nil {
		return nil, ferr
	}
	return deskkit.CompleteChangedFiles(fg, fr, number)
}

// liveRepresentedPRBatchFiles reuses the admitted shared planning reader. Unsupported
// backends retain native complete reads; no new credential or query owner is introduced.
func liveRepresentedPRBatchFiles(repo string, numbers []int) map[int]deskkit.PendingChangeObservation {
	owner, name, ok := strings.Cut(strings.TrimSpace(repo), "/")
	var fg deskkit.Forge
	var err error
	fr := deskkit.ForgeRepo{Owner: owner, Name: name}
	if !ok || owner == "" || name == "" {
		err = fmt.Errorf("changed-file read: target repo %q is not owner/name", repo)
	} else {
		fg, err = productionForgeResolver(fr)
	}
	if err != nil {
		out := map[int]deskkit.PendingChangeObservation{}
		for _, n := range numbers {
			out[n] = deskkit.PendingChangeObservation{Err: err}
		}
		return out
	}
	return representedFilesBatch(fg, fr, numbers, deskkit.PendingChangeBatchReader(fg))
}

// The compact projection intentionally omits previous paths. A rename/copy cannot
// reach BriefAuthoringOnly until the complete native reader supplies its ancestry,
// bound to the same metadata observation. Errors remain errors, never a second vote.
func representedFilesBatch(fg deskkit.Forge, fr deskkit.ForgeRepo, numbers []int, batch func(deskkit.ForgeRepo, []int) map[int]deskkit.PendingChangeObservation) map[int]deskkit.PendingChangeObservation {
	out := map[int]deskkit.PendingChangeObservation{}
	if batch == nil {
		for _, n := range numbers {
			pr, files, err := deskkit.CompleteChangedFilesOf(fg, fr, n)
			out[n] = deskkit.PendingChangeObservation{PR: pr, Files: files, Err: err}
		}
		return out
	}
	observed := batch(fr, numbers)
	for _, n := range numbers {
		ob, ok := observed[n]
		if !ok {
			ob.Err = fmt.Errorf("complete changed-file observation absent")
		}
		if ob.Err == nil && (ob.PR == nil || ob.PR.Number != n || len(ob.Files) != ob.PR.ChangedFiles) {
			ob.Err = fmt.Errorf("incomplete changed-file observation")
		}
		if ob.Err == nil {
			for _, f := range ob.Files {
				if f.Status != "renamed" && f.Status != "copied" {
					continue
				}
				pr, files, err := deskkit.CompleteChangedFilesOf(fg, fr, n)
				if err == nil && (pr.HeadSHA != ob.PR.HeadSHA || pr.ChangedFiles != ob.PR.ChangedFiles) {
					err = fmt.Errorf("pending change moved during ancestry fallback")
				}
				ob = deskkit.PendingChangeObservation{PR: pr, Files: files, Err: err}
				break
			}
		}
		out[n] = ob
	}
	return out
}
