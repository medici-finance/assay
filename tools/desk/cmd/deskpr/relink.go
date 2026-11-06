package main

// relink.go — `deskpr edit --relink-from N --relink-to M --relink-reason R`, the ONE way an
// edit may change a PR's Issue-kind link trailer (#5020).
//
// The default stays as edit.go states it: a replacement body whose link differs from the PR's
// current one is refused. The relink flags are the explicit, audited exception the ruling on
// #5020 (option A) asked for, and they are deliberately narrow:
//
//   - all three flags are given together or not at all; a partial set (a blank reason given
//     alone included), a non-positive number, or an empty, multi-line, over-long or
//     render-garbling reason refuses (exit 5) before any forge call;
//   - the PR must be this checkout's own (on its head branch, or HEAD at its head commit): the
//     other-checkout admission for `--pr N` rests on the link being fixed, so it never relinks;
//   - only an Issue-kind link in this repository moves (`Issue: #N` / `Fixes: #N`), never a
//     Brief:, Authors:, Cross-repo-brief: or owner/repo#N link;
//   - --relink-from must be the number the PR's CURRENT trailer carries, and the replacement
//     body's trailer must carry --relink-to, so the flag names exactly the edge that moves;
//   - `Issue: #N` -> `Fixes: #N` (or back) on the same number goes through the same flag;
//   - the NEW issue must exist, be an open issue (a pull request number refuses, exit 5), pass
//     the trust gate (a trusted
//     author or a current blessing; an unreadable trust record is exit 6), and carry no human
//     gate or decision gate — so the flag cannot re-point a PR at work the desk may not act on;
//   - the change and its reason are recorded in an audit comment on the PR, posted BEFORE the
//     body edit, so no landed edit lacks its record.

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/reviewflow"
)

// relinkMaxReason bounds the reason text: it is a one-line justification, not a second body.
const relinkMaxReason = 500

// relinkReq is a validated --relink-* request.
type relinkReq struct {
	from, to int
	reason   string
}

// parseRelink validates the three --relink-* flags offline. given reports whether any of them
// appeared on the command line; when none did it returns (nil, nil), so an edit without the
// flag runs exactly as before. A flag given with a blank value is a partial set, not an
// absent one.
func parseRelink(given bool, from, to int, reason string) (*relinkReq, error) {
	if !given {
		return nil, nil
	}
	reason = strings.TrimSpace(reason)
	var missing []string
	if from <= 0 {
		missing = append(missing, "--relink-from N (the issue number the PR's trailer names now)")
	}
	if to <= 0 {
		missing = append(missing, "--relink-to N (the issue number it should name)")
	}
	if reason == "" {
		missing = append(missing, "--relink-reason TEXT (why the link is wrong)")
	}
	if len(missing) > 0 {
		return nil, deskkit.Refused("refused: a link correction names the old number, the new number and a reason, " +
			"all three — missing or invalid: " + strings.Join(missing, "; "))
	}
	if strings.ContainsAny(reason, "\r\n") {
		return nil, deskkit.Refused("refused: --relink-reason must be one line — it is recorded verbatim in the audit comment")
	}
	if bad := relinkReasonGarble(reason); bad != "" {
		return nil, deskkit.Refused("refused: --relink-reason carries " + bad + ", which would garble the audit comment " +
			"it is recorded in — write it as plain text")
	}
	if len(reason) > relinkMaxReason {
		return nil, deskkit.Refused(fmt.Sprintf("refused: --relink-reason is %d bytes; keep it to %d", len(reason), relinkMaxReason))
	}
	return &relinkReq{from: from, to: to, reason: reason}, nil
}

// relinkReasonGarble names the first thing in a reason that could hide or break the rest of
// the audit comment when rendered: an HTML comment opener, a backtick (the reason renders in a
// code span), or a control or Unicode line/paragraph separator. "" when the reason is plain.
func relinkReasonGarble(reason string) string {
	if strings.Contains(reason, "<!--") {
		return "an HTML comment opener (<!--)"
	}
	if strings.Contains(reason, "`") {
		return "a backtick"
	}
	for _, r := range reason {
		if r == '\u2028' || r == '\u2029' || unicode.IsControl(r) {
			return fmt.Sprintf("the control or separator character U+%04X", r)
		}
	}
	return ""
}

// sameRepoIssue returns the body's trailer when it is an Issue-kind link to an issue in repo.
func sameRepoIssue(body []byte, repo string) (deskkit.Trailer, bool) {
	if isScanCarrierBody(body) {
		return deskkit.Trailer{}, false
	}
	trs, err := deskkit.ParseTrailers(body)
	if err != nil || len(trs) != 1 || trs[0].Kind != deskkit.TrailerIssue {
		return deskkit.Trailer{}, false
	}
	if trs[0].Repo != "" && !strings.EqualFold(trs[0].Repo, repo) {
		return deskkit.Trailer{}, false
	}
	return trs[0], true
}

// checkBody is the offline half: the replacement body's trailer must be a same-repository
// Issue-kind link carrying --relink-to. requireTrailer has already run the full grammar.
func (r *relinkReq) checkBody(body []byte, repo string) error {
	t, ok := sameRepoIssue(body, repo)
	if !ok {
		return deskkit.Refused("refused: --relink-* corrects an Issue: or Fixes: link to an issue in this repository; " +
			"the replacement body's trailer is not one")
	}
	if t.Value != strconv.Itoa(r.to) {
		return deskkit.Refused(fmt.Sprintf("refused: --relink-to is #%d but the replacement body's trailer is `%s` — "+
			"the flag must name the link the body carries", r.to, t.IssueLink()))
	}
	return nil
}

// admit is the PR-side half: the PR's CURRENT trailer must be a same-repository Issue-kind link
// numbered --relink-from, and the replacement link must actually differ from it.
func (r *relinkReq) admit(curBody, body []byte, repo string) (oldLink, newLink string, err error) {
	cur, ok := sameRepoIssue(curBody, repo)
	if !ok {
		l, had := trailerLink(curBody)
		if !had {
			l = "no link trailer"
		}
		return "", "", deskkit.Refused(fmt.Sprintf("refused: --relink-from #%d does not match the PR's current trailer (%s) — "+
			"only an Issue: or Fixes: link to an issue in this repository can be corrected", r.from, l))
	}
	if cur.Value != strconv.Itoa(r.from) {
		hint := "name the number the PR links now"
		if nt, ok := sameRepoIssue(body, repo); ok && cur.IssueLink() == nt.IssueLink() {
			// A re-run after an edit that landed (say its re-review notice failed): the PR
			// already carries the target link, so the correction is done.
			hint = "the PR already carries the replacement body's link, so the correction has landed; " +
				"re-run without --relink-* if the rest of the body still needs editing"
		}
		return "", "", deskkit.Refused(fmt.Sprintf("refused: --relink-from #%d does not match the PR's current trailer `%s` — %s",
			r.from, cur.IssueLink(), hint))
	}
	nt, _ := sameRepoIssue(body, repo)
	oldLink, newLink = cur.IssueLink(), nt.IssueLink()
	if oldLink == newLink {
		return "", "", deskkit.Refused(fmt.Sprintf("refused: the replacement body keeps the link `%s` — there is nothing "+
			"for --relink-* to correct; drop the flags", oldLink))
	}
	return oldLink, newLink, nil
}

// relinkIssueGate runs the checks the NEW issue must pass before a PR may link it: it exists,
// it is an open issue (not a PR), its author is trusted or it carries a current blessing (the
// same trust gate deskpost applies, built from the same deskkit primitives), and it declares no
// human gate or decision gate. Unreadable is exit 6, never a pass.
//
// GitHub numbers issues and pull requests in one sequence, and its typed issue read answers a
// pull-request number with a could-not-check error rather than a record. So a failed issue
// read is followed by ONE change-typed read of the same number: only a record that read
// positively reports as a pull request turns the failure into the exit-5 "is a pull request"
// refusal. Every other outcome keeps the original read's exit 6.
func relinkIssueGate(fg deskkit.Forge, fr deskkit.ForgeRepo, n int) error {
	is, err := fg.GetIssueTyped(fr, n, deskkit.TargetIssue)
	if err != nil {
		if deskkit.IsForgeNotFound(err) {
			return deskkit.Refused(fmt.Sprintf("refused: --relink-to #%d does not exist in %s", n, fr.Slug()))
		}
		if ch, cerr := fg.GetIssueTyped(fr, n, deskkit.TargetChange); cerr == nil && ch != nil && ch.IsPullRequest {
			return deskkit.Refused(fmt.Sprintf("refused: --relink-to #%d is a pull request, not an issue", n))
		}
		return deskkit.Unverifiable(fmt.Sprintf("cannot read issue #%d (--relink-to)", n), err)
	}
	if is == nil {
		return deskkit.Unverifiable(fmt.Sprintf("the forge returned no record for issue #%d (--relink-to)", n), nil)
	}
	if is.IsPullRequest {
		return deskkit.Refused(fmt.Sprintf("refused: --relink-to #%d is a pull request, not an issue", n))
	}
	if !strings.EqualFold(strings.TrimSpace(is.State), "open") {
		return deskkit.Refused(fmt.Sprintf("refused: --relink-to #%d is %s, not open — a PR links open work", n, is.State))
	}
	if !deskkit.TrustedAuthorID(is.Author.Login, is.Author.ID) {
		tp, terr := fg.IssueTrustEvents(fr, n)
		if terr != nil {
			return deskkit.Unverifiable(fmt.Sprintf("cannot read trust events for issue #%d (trust gate)", n), terr)
		}
		if tp == nil || !tp.Complete || !deskkit.Blessed(tp.BodyEdited, tp.Events) {
			return deskkit.Refused(fmt.Sprintf("refused: --relink-to #%d author %q is not a trusted desk identity and the "+
				"issue carries no current blessing (trust gate) — a PR may not be re-pointed at unvetted work", n, is.Author.Login))
		}
	}
	if gated, why := relinkGated(is); gated {
		return deskkit.Refused(fmt.Sprintf("refused: --relink-to #%d is gated (%s) — a desk verb may not re-point a PR at "+
			"work waiting on a human", n, why))
	}
	return nil
}

// relinkGated reports whether an issue declares a human gate or a decision gate.
func relinkGated(is *deskkit.Issue) (bool, string) {
	if ok, why := reviewflow.HumanGateDeclared(is.Title, is.Body, is.Labels); ok {
		return true, why
	}
	for _, l := range is.Labels {
		name := strings.ToLower(strings.TrimSpace(l))
		if name == deskkit.DecisionLabel {
			return true, "label " + deskkit.DecisionLabel
		}
		for _, ow := range deskkit.OneWayLabels {
			if name == ow {
				return true, "label " + ow
			}
		}
	}
	if strings.Contains(is.Body, deskkit.DecisionGateMarkerPrefix+":") {
		return true, "a " + deskkit.DecisionGateMarkerPrefix + " marker"
	}
	return false, ""
}

// relinkAudit is the audit comment. Its only caller-supplied text is the reason, which has
// already been through the secret scan and the self-containment scan, carries no backtick,
// HTML comment opener or control character, and renders inside a code span.
func relinkAudit(oldLink, newLink, reason string) string {
	return fmt.Sprintf(
		"Link trailer correction by %s: `%s` → `%s`.\n\n"+
			"Reason: `%s`\n\n"+
			"Posted by `deskpr edit --relink-from/--relink-to/--relink-reason` BEFORE the body edit, so the record "+
			"exists for every edit that lands. If the PR body still carries `%s`, the edit did not land. The new "+
			"issue passed the existence, open-state, trust and gate checks.\n",
		editActor(), oldLink, newLink, reason, oldLink)
}
