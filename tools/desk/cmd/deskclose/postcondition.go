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
// The repair issue is best-effort and never changes the verdict. It is a charged outward write
// like every other write in this package: the meter gate runs before it, its outcome is
// recorded as its own audit line keyed to the item (the bucket that gate reads), and that line
// is written AFTER the failed close's own line (closeRefused), so the gate counts the close.
// It is deduped against an open issue with exactly the same title, found through a search
// whose query is plain [a-z0-9] tokens — the SearchIssues contract forbids caller-supplied
// search syntax, and a backend that forwards the query verbatim as search text (GitLab) would
// read a qualifier as literal words no issue contains. When the issue cannot be filed the
// failure says so (could-not-check) — still exit 6.

import (
	"errors"
	"fmt"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// closeNotTaken is the no-op shape: the close call returned without error and the re-read
// still shows the item open. closeRefused keys the repair-issue filing on it.
type closeNotTaken struct {
	repo  string
	n     int
	kind  deskkit.TargetKind
	state string
}

func (e *closeNotTaken) Error() string {
	return fmt.Sprintf("%s#%d still reads state %q after the close call — the state change did not take (postcondition failed)",
		e.repo, e.n, e.state)
}

// assertClosedPostcondition re-reads repo#n as kind after a close call and returns nil only
// when the item reads closed. It writes nothing: the repair issue is the caller's, filed by
// closeRefused once the close's own audit line is recorded.
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
	return &closeNotTaken{repo: repo, n: n, kind: kind, state: deskkit.StripControl(after.State)}
}

// closeRefused is the one failure path of every verified-close lane, after the pre-close
// comment landed: it records the close's audit line FIRST, then — only for the no-op shape —
// files the repair issue, and returns the exit-6 partial.
func closeRefused(a *auditCtx, repo string, n int, cerr error) error {
	a.log(deskkit.ResultUnverifiable, "partial: comment posted, close refused: "+cerr.Error())
	msg := fmt.Sprintf("could-not-check: %s#%d — partial: comment posted, close refused", repo, n)
	var nt *closeNotTaken
	if errors.As(cerr, &nt) {
		msg += " (" + fileRepairIssue(a, nt) + ")"
	}
	return deskkit.Unverifiable(msg, cerr)
}

// repairTitle is the stable title of the repair issue for one item: stable so a re-run that
// hits the same no-op finds the open issue instead of filing a second one.
func repairTitle(n int, kind deskkit.TargetKind) string {
	return fmt.Sprintf("deskclose postcondition failed: %s %d still open after close", kind, n)
}

// searchTokens reduces text to lower-case [a-z0-9] runs joined by single spaces — the
// already-tokenised free text the SearchIssues contract requires, so no token can be read as
// a forge search qualifier.
func searchTokens(text string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	}), " ")
}

// fileRepairIssue files (or finds) the repair issue for a close that did not take, records
// the outcome as its own audit line, and returns a short note for the failure message. It
// never returns an error: the run is already failing, and a repair issue that could not be
// filed is reported in the note, not by changing the exit.
func fileRepairIssue(a *auditCtx, nt *closeNotTaken) string {
	note := func(result, s string) string {
		a.log(result, s)
		return s
	}
	fg, fr, ferr := forgeForFn(nt.repo)
	if ferr != nil {
		return note(deskkit.ResultUnwritten, "repair issue NOT filed — could-not-check: "+ferr.Error())
	}
	title := repairTitle(nt.n, nt.kind)
	hits, serr := fg.SearchIssues(fr, deskkit.SearchIssuesInput{Query: searchTokens(title)})
	if serr != nil {
		return note(deskkit.ResultUnwritten, "repair issue NOT filed — could-not-check: the dedupe search failed: "+serr.Error())
	}
	for _, h := range hits {
		if strings.EqualFold(h.State, "open") && strings.TrimSpace(h.Title) == title {
			return note(deskkit.ResultNoop, fmt.Sprintf("repair issue already open #%d", h.Number))
		}
	}
	if err := allowWrite(nt.repo, nt.n); err != nil {
		return note(deskkit.ResultRateLimited, "repair issue NOT filed — "+err.Error())
	}
	ref, err := fg.FileIssue(fr, deskkit.IssueInput{Title: title, Body: repairBody(nt.n, nt.kind, nt.state)})
	if err != nil {
		return note(deskkit.ResultUnverifiable, "repair issue NOT filed — could-not-check: "+err.Error())
	}
	return note(deskkit.ResultOK, fmt.Sprintf("filed repair issue #%d", ref.Number))
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
