package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// authoring.go — keeping a briefs-AUTHORING PR from reading as the brief's DELIVERY.
//
// The phantom check (phantom.go) refuses a fresh worker dispatch whose brief already has an OPEN or
// MERGED PR, matched on the PR body's `Brief:` trailer. A docs-only PR that WROTE the brief carries
// that trailer too, so after it merged the brief could never be dispatched: every attempt was refused
// as "already represented" by the PR that authored it. dropBriefAuthoringPRs runs between the PR-list
// read and the phantom match, and removes each PR whose changed files show it only authored the brief
// (deskkit.BriefAuthoringOnly). Everything else stays in the list and is matched as before.
//
// Only PRs whose trailer names THIS brief are inspected, so the file reads stay bounded by the number
// of PRs that name one brief (usually zero or one). The classification can only ever REMOVE a PR from
// the match. Any doubt keeps the PR in, which leaves the dispatch refused as it was before this file:
// a file list with any path that is not a stream board README, a brief file or a changelog fragment
// (so a document delivered under `docs/streams/` keeps the PR a delivery), a list that does not add
// the brief's own file, or no file transport wired at all.

// listPRFiles reads one PR's COMPLETE changed-file list. Like listRepresentedPRs it is nil by default:
// nil means the offline reference build performs no forge read, and every representing PR is then
// counted as a delivery (the behaviour before the authoring exemption existed). main() wires
// livePRFiles. A list the transport cannot read, or cannot prove complete, is an error, and the
// dispatch is HELD as could-not-check. It is never guessed in either direction.
var listPRFiles func(repo string, number int) ([]deskkit.ChangedFile, error)

// dropBriefAuthoringPRs returns prs without the PRs that represent briefID only by having authored
// it. A PR whose files cannot be read makes the whole answer could-not-check (exit 6).
func dropBriefAuthoringPRs(repo, briefID string, prs []deskkit.PRRef) ([]deskkit.PRRef, error) {
	if listPRFiles == nil {
		return prs, nil
	}
	id := strings.ToLower(strings.TrimSpace(briefID))
	kept := make([]deskkit.PRRef, 0, len(prs))
	for _, pr := range prs {
		rp, names := deskkit.RepresentedBriefPRs([]deskkit.PRRef{pr})[id]
		if !names {
			kept = append(kept, pr)
			continue
		}
		files, err := listPRFiles(repo, rp.Number)
		if err != nil {
			return nil, deskkit.Unverifiable(fmt.Sprintf(
				"step %s: %s#%d carries a `Brief: %s` trailer, but its changed files could not be read to tell "+
					"whether it DELIVERED %s or only AUTHORED it (%v). Could-not-check is neither answer, so the "+
					"dispatch is HELD — nothing was claimed; retry.",
				stepClaimAcquire, repo, rp.Number, briefID, briefID, err), err)
		}
		if deskkit.BriefAuthoringOnly(id, files) {
			fmt.Fprintf(os.Stderr, "deskdispatch: NOTICE — %s#%d names %s in its `Brief:` trailer but only "+
				"AUTHORED it (it adds the brief's file and touches only stream board READMEs, brief files and "+
				"changelog fragments), so it is not counted as the brief's delivery\n", repo, rp.Number, briefID)
			continue
		}
		kept = append(kept, pr)
	}
	return kept, nil
}

// livePRFiles is listPRFiles' shipped transport. It resolves the forge under the dispatcher role, the
// same way liveRepresentedPRs does, and reads through the typed Forge seam, never a forge CLI.
func livePRFiles(repo string, number int) ([]deskkit.ChangedFile, error) {
	fr, err := forgeRepoOf(repo)
	if err != nil {
		return nil, err
	}
	fg, _, rerr := deskkit.ResolveForge(fr, deskkit.DispatcherRole)
	if rerr != nil {
		return nil, rerr
	}
	return completePRFiles(fg, fr, number)
}

// completePRFiles reads a change's file list and reconciles its length against the forge's own
// changed-file count. ListChangedFiles is bounded (GitHub stops listing at 3000 files), and a short
// walk that happened to show only docs could hide the code that makes the PR a delivery, so a list
// that does not match the count is an error, never a partial answer.
func completePRFiles(fg deskkit.Forge, fr deskkit.ForgeRepo, number int) ([]deskkit.ChangedFile, error) {
	pr, err := fg.GetPullRequest(fr, number)
	if err != nil {
		return nil, err
	}
	files, err := fg.ListChangedFiles(fr, number)
	if err != nil {
		return nil, err
	}
	if len(files) != pr.ChangedFiles {
		return nil, fmt.Errorf("%s#%d lists %d changed files but the forge counts %d; the list is not "+
			"provably complete", fr.Slug(), number, len(files), pr.ChangedFiles)
	}
	return files, nil
}
