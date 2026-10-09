package main

// postcondition.go — the write-boundary postcondition on every verified close.
//
// A close call that returns without error is not proof the item closed: a forge can accept
// the request and leave the state unchanged (the reported field case was a state_reason PATCH
// on a pull request, swallowed after the comment posted). So after the close, deskclose RE-READS
// the item and asserts the effect. Three outcomes, and only one of them is success:
//
//   - the re-read shows the item closed: the effect is present — success;
//   - the re-read fails: could-not-check, exit 6. The close is unconfirmed, not known to have
//     failed, so nothing is filed — the next run's read settles it;
//   - the re-read shows the item still OPEN after a claimed-successful close: the no-op shape.
//     The run fails (exit 6, never 0) AND files a repair issue on the item's repo, so the
//     silent-nothing is visible to someone who did not watch this run's output.
//
// The BENIGN replay is a different thing and is handled before any write (applyClose step 2):
// an item that already reads closed on the pre-write fetch is an idempotent no-op whose effect
// is proven present by that read. Only a close this run CLAIMED and the re-read contradicts is
// the failure this file reports.
//
// The repair issue is best-effort and never changes the verdict: it is deduped against an open
// issue with the same title, charged to the same outward-write meter as every other write, and
// when it cannot be filed the failure says so (could-not-check) — still exit 6.

import (
	"fmt"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// assertClosedPostcondition re-reads repo#n as kind after a close call and returns nil only
// when the item reads closed.
func assertClosedPostcondition(repo string, n int, kind deskkit.TargetKind) error {
	after, rerr := fetchItem(repo, n, kind)
	if rerr != nil {
		return deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s#%d was asked to close but its state could not be read back — the close is unconfirmed",
			repo, n), rerr)
	}
	if after.closed() {
		return nil
	}
	state := deskkit.StripControl(after.State)
	note := fileRepairIssue(repo, n, kind, state)
	return deskkit.Unverifiable(fmt.Sprintf(
		"%s#%d still reads state %q after the close call — the state change did not take (postcondition failed; %s)",
		repo, n, state, note), nil)
}

// repairTitle is the stable title of the repair issue for one item: stable so a re-run that
// hits the same no-op finds the open issue instead of filing a second one.
func repairTitle(n int, kind deskkit.TargetKind) string {
	return fmt.Sprintf("deskclose postcondition failed: %s %d still open after close", kind, n)
}

// fileRepairIssue files (or finds) the repair issue for a close that did not take, and returns
// a short note for the failure message. It never returns an error: the run is already failing,
// and a repair issue that could not be filed is reported in the note, not by changing the exit.
func fileRepairIssue(repo string, n int, kind deskkit.TargetKind, state string) string {
	fg, fr, ferr := forgeForFn(repo)
	if ferr != nil {
		return "repair issue NOT filed — could-not-check: " + ferr.Error()
	}
	title := repairTitle(n, kind)
	hits, serr := fg.SearchIssues(fr, deskkit.SearchIssuesInput{Query: fmt.Sprintf("%q in:title", title)})
	if serr != nil {
		return "repair issue NOT filed — could-not-check: the dedupe search failed: " + serr.Error()
	}
	for _, h := range hits {
		if strings.EqualFold(h.State, "open") && strings.TrimSpace(h.Title) == title {
			return fmt.Sprintf("repair issue already open #%d", h.Number)
		}
	}
	if err := allowWrite(repo, n); err != nil {
		return "repair issue NOT filed — " + err.Error()
	}
	ref, err := fg.FileIssue(fr, deskkit.IssueInput{Title: title, Body: repairBody(n, kind, state)})
	if err != nil {
		return "repair issue NOT filed — could-not-check: " + err.Error()
	}
	return fmt.Sprintf("filed repair issue #%d", ref.Number)
}

func repairBody(n int, kind deskkit.TargetKind, state string) string {
	return fmt.Sprintf(`deskclose asked the forge to close %s %d. The close call returned without error, but
the post-close re-read still shows the item in state %q: the close did not take.

deskclose reported this as a failure (exit 6), not success. Any comment the run posted
before the close is still on the item; the item itself was NOT closed.

What is needed: someone with the right access confirms why the forge accepted the close
without applying it (a permission or kind mismatch, a protection rule, a workflow that
re-opens bot closes), fixes that cause, and re-runs the close. Close this issue once the
item reads closed.
`, kind, n, state)
}
