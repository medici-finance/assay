package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// triage.go — the intake front door's "close, no fix PR" exit for an ISSUE (issue #1207).
//
// The gap it fills: `deskclose`'s other lanes retire a PR (superseded / review-request),
// fold a duplicate, or run a human-ruled batch — none closes an idea-issue whose triage
// disposition is the plain "research says skip, no brief" (not-planned), nor one whose
// human decision has been recorded and whose remaining work is tracked elsewhere
// (human-decided). Before this lane the only sanctioned exit was a full fix-PR cycle, and
// a raw `gh issue close --reason "not planned"` is denied to an auto-mode session as an
// external write — so a triaged idea-issue kept its disposition comment but could not be
// closed by any sanctioned path.
//
// WHERE THIS SITS IN THE TWO-GATE MODEL (main.go). triage has TWO dispositions with two
// different authorities, deliberately:
//
//	not-planned    authorizes on a RECORDED DISPOSITION, not a fetched ruling: either the
//	               triage-disposition marker comment intake already posted on the issue, or
//	               a --tracker naming where any residual work lives. This is the desk's
//	               standing "drain the front door" job — a low-stakes skip whose control is
//	               that the disposition is on the record before the close, never a bare flag
//	               with nothing behind it. It does NOT call the R-1 ruling gate — the same
//	               structural reason self-withdraw / verify-gate-refire do not (lanes.go):
//	               a plain triage skip is not the retirement of another party's decided item.
//
//	human-decided  authorizes on the SAME fetched-and-verified R-1 sign-off (gateFor /
//	               authorize, authority.go) every ruled lane uses — a sign-off URL fetched
//	               and its author checked against the roster-pinned blessing authority, never
//	               a flag that manufactures authority. It ALSO requires --tracker: the
//	               recorded-decision close must NAME the brief / PR / issue carrying the
//	               remaining work (the intake close-authority ruling), so "the work is
//	               tracked" is falsifiable rather than asserted.
//
// THE CONTROL THIS LANE KEEPS (worker §2, issue #1207). A close verb is an authority
// surface. triage must never close a `needs-decision` issue (one still on the human's
// decision queue), and must never close an item lacking its required disposition/ruling.
// So:
//   - not-planned refuses BOTH decision labels (refuseDecisionItem, shared with the ruled
//     lanes) — a decision item is not a plain skip;
//   - human-decided refuses `needs-decision` (still undecided) but PERMITS `human-decided`
//     — closing a human-decided item, gated by the verified ruling and the named tracker,
//     is exactly the sanctioned new lane this verb adds. This is the "lane A" close the
//     decision-label gate's doc named as not-yet-in-the-mode-set; #1207 puts it in the set,
//     bounded by the ruling gate, not by the label.
//
// A non-issue target (a pull request) is refused; an already-closed issue is an idempotent
// no-op success; an unreadable item is could-not-check (exit 6), never a guessed close.

const modeTriage = "triage"

// The two dispositions. The set is CLOSED: an unknown value is refused, never coerced.
const (
	dispositionNotPlanned   = "not-planned"
	dispositionHumanDecided = "human-decided"
)

// triageDispositionMarker is the machine-readable envelope intake posts on an issue when it
// records a rejected/watching disposition — the issue-side analog of the PR disposition
// record's `<!-- desk-disposition v1 -->` marker (deskkit/disposition.go). not-planned reads
// it the way deskclose reads every authority: from the FETCHED comment thread, never from a
// flag. Until intake stamps it, --tracker is the alternative recorded signal.
const triageDispositionMarker = "<!-- desk-triage v1 -->"

func dispositions() []string { return []string{dispositionNotPlanned, dispositionHumanDecided} }

// triageReq is one triage close, fully specified before any forge write.
type triageReq struct {
	repo        string
	number      int
	disposition string
	// tracker is the resolved/rendered ref naming where residual or decided work lives
	// (`owner/repo#N` / `owner/repo!N`), or "" when the caller gave none. Recorded in the
	// close comment; for human-decided it is required, for not-planned it is one of two
	// accepted signals.
	tracker string
	rulings string
	kind    deskkit.TargetKind
	dryRun  bool
}

func cmdTriage(args []string, out io.Writer) error {
	fs := flag.NewFlagSet(modeTriage, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var c common
	c.bind(fs)
	disposition := fs.String("disposition", "",
		"the triage disposition: "+dispositionNotPlanned+" (a plain research-says-skip close) | "+
			dispositionHumanDecided+" (a recorded human decision; requires the verified R-1 sign-off and --tracker)")
	tracker := fs.String("tracker", "",
		"the item recording where residual or decided work lives (N | #N | !N | owner/repo#N | owner/repo!N | web URL)")
	n, err := parseItemVerb(fs, args, &c)
	if err != nil {
		return err
	}
	// A triage close acts on an ISSUE. A stated change kind names an object this lane never
	// touches; refuse it pre-flight, before any read, rather than resolve it to the change
	// sequence. (A bare number that RESOLVES to a PR is caught after the read, below.)
	if c.kind == deskkit.TargetChange {
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s closes an ISSUE, and the item was stated as a change (`!N` / --kind mr). "+
				"A pull request is not a triage target; state the issue (--kind issue) or omit the kind.",
			modeTriage))
	}
	disp := strings.TrimSpace(*disposition)
	switch disp {
	case dispositionNotPlanned, dispositionHumanDecided:
	case "":
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s requires --disposition {%s}", modeTriage, strings.Join(dispositions(), "|")))
	default:
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s --disposition %q is not a disposition this lane records; the set is CLOSED: %s",
			modeTriage, deskkit.StripControl(disp), strings.Join(dispositions(), " | ")))
	}
	trackerRef := ""
	if strings.TrimSpace(*tracker) != "" {
		r, err := resolveRef(c.repo, strings.TrimSpace(*tracker), "")
		if err != nil {
			return err
		}
		trackerRef = renderRef(r)
	}
	return applyTriage(triageReq{
		repo: c.repo, number: n, disposition: disp, tracker: trackerRef,
		rulings: c.rulings, kind: c.kind, dryRun: c.dryRun,
	}, out)
}

// applyTriage is the lane's own precondition chain. Fixed order, matching the ruled lanes:
//
//  1. read the item                 — a read failure is exit 6, never an assumption
//  2. idempotent no-op              — already closed is success, so a resumed run is safe
//  3. non-issue target → refused    — a pull request is not a triage target
//  4. authorization by disposition  — decision-label gate, then the disposition's own authority
//  5. comment (charged write)       — the trail lands BEFORE the close
//  6. close   (charged write)       — as not_planned, the intake exit's state reason
func applyTriage(r triageReq, out io.Writer) error {
	a := &auditCtx{verb: modeTriage, repo: r.repo, number: r.number}

	it, err := fetchItem(r.repo, r.number, r.kind)
	if err != nil {
		a.log(deskkit.ResultUnwritten, err.Error())
		return err
	}
	a.title = it.Title

	// A non-issue target is refused BEFORE the idempotency check, so a pull request is
	// rejected as the wrong kind of object whatever its state — never silently absorbed as
	// an "already closed" no-op.
	if it.isPR() {
		err := deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d is a pull request, not an issue. %s is the intake front door's ISSUE exit; "+
				"a PR is retired through the superseded / review-request / self-withdraw lanes, on a recorded "+
				"finding, never as a plain triage skip.", r.repo, r.number, modeTriage))
		a.log(deskkit.ResultRefused, err.Error())
		return err
	}
	if it.closed() {
		a.log(deskkit.ResultNoop, "already closed — idempotent no-op")
		fmt.Fprintf(out, "noop\t%s#%d\talready closed\n", r.repo, r.number)
		return nil
	}

	source, err := authorizeTriage(r, it)
	if err != nil {
		a.log(resultFor(err), err.Error())
		return err
	}

	body := triageComment(r, source)
	if r.dryRun {
		a.log(deskkit.ResultDryRun, "dry-run: verified, wrote nothing")
		fmt.Fprintf(out, "dry-run\t%s#%d\t%s\tdisposition=%s\ttracker=%s\n",
			r.repo, r.number, modeTriage, r.disposition, r.tracker)
		fmt.Fprintln(out, body)
		return nil
	}

	if err := allowWrite(r.repo, r.number); err != nil {
		a.log(deskkit.ResultRateLimited, err.Error())
		return err
	}
	if err := postComment(r.repo, r.number, deskkit.TargetIssue, body); err != nil {
		a.log(deskkit.ResultUnverifiable, err.Error())
		return err
	}
	a.log(deskkit.ResultOK, "posted the pre-close comment")

	if err := allowWrite(r.repo, r.number); err != nil {
		a.log(deskkit.ResultRateLimited, "comment posted, close deferred: "+err.Error())
		return err
	}
	if err := closeItem(r.repo, r.number, deskkit.TargetIssue, reasonNotPlanned); err != nil {
		a.log(deskkit.ResultUnverifiable, err.Error())
		return err
	}
	a.log(deskkit.ResultOK, fmt.Sprintf("closed as %s via lane %s (disposition %s, %s)",
		reasonNotPlanned, modeTriage, r.disposition, deskkit.StripControl(source)))
	fmt.Fprintf(out, "closed\t%s#%d\t%s\tdisposition=%s\treason=%s\n",
		r.repo, r.number, modeTriage, r.disposition, reasonNotPlanned)
	return nil
}

// authorizeTriage applies the decision-label gate and the disposition's own authority,
// returning a human-legible SOURCE of the authorization that the close comment cites.
func authorizeTriage(r triageReq, it item) (string, error) {
	switch r.disposition {
	case dispositionNotPlanned:
		// A decision item — needs-decision OR human-decided — is never a plain skip.
		if err := refuseDecisionItem(r.repo, it); err != nil {
			return "", err
		}
		return notPlannedAuthority(r)
	case dispositionHumanDecided:
		// Still on the human's decision queue → refused; a human-decided label is PERMITTED
		// (that is what this disposition closes), so only needs-decision is refused here.
		if err := refuseNeedsDecision(r.repo, it); err != nil {
			return "", err
		}
		return humanDecidedAuthority(r)
	default:
		// Unreachable: cmdTriage validated the closed set. Fail closed regardless.
		return "", deskkit.Refused(fmt.Sprintf("refused: unknown disposition %q", deskkit.StripControl(r.disposition)))
	}
}

// notPlannedAuthority accepts either a --tracker ref or a triage-disposition marker comment
// ALREADY on the issue, and refuses when neither is present. The marker is read from the
// FETCHED comment thread; an unreadable thread is could-not-check (exit 6), never a guess.
func notPlannedAuthority(r triageReq) (string, error) {
	if r.tracker != "" {
		return "tracker " + r.tracker, nil
	}
	fg, fr, ferr := forgeForFn(r.repo)
	if ferr != nil {
		return "", ferr
	}
	comments, err := fg.ListCommentsTyped(fr, r.number, deskkit.TargetIssue)
	if err != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: cannot read the comments on %s#%d to look for a triage disposition record "+
				"(%s) — an unreadable thread is not an absent record, so the close is refused",
			r.repo, r.number, triageDispositionMarker), err)
	}
	for _, c := range comments {
		if strings.Contains(c.Body, triageDispositionMarker) {
			return "recorded triage disposition (" + triageDispositionMarker + ")", nil
		}
	}
	return "", deskkit.Refused(fmt.Sprintf(
		"refused: %s#%d carries no triage disposition record (%s marker comment) and no --tracker was "+
			"given. %s --disposition %s closes only an issue whose skip is already on the record, or one "+
			"whose residual work is named by --tracker <ref>; a bare close with neither is exactly the "+
			"unfalsifiable skip the disposition record exists to replace.",
		r.repo, r.number, triageDispositionMarker, modeTriage, dispositionNotPlanned))
}

// humanDecidedAuthority reuses the R-1 ruling gate (gateFor / authorize) — the sign-off URL
// fetched and its author verified against the roster-pinned blessing authority — and requires
// a --tracker naming the work that continues. Neither is a flag the caller can satisfy by
// asserting it: the ruling is fetched, and the tracker is named on the record.
func humanDecidedAuthority(r triageReq) (string, error) {
	if r.tracker == "" {
		return "", deskkit.Refused(fmt.Sprintf(
			"refused: %s --disposition %s requires --tracker <ref> — the recorded-decision close must NAME "+
				"the brief, PR or issue carrying the remaining work, never assert that it is tracked. Create "+
				"the tracker first, then close.", modeTriage, dispositionHumanDecided))
	}
	g, err := gateFor(&common{repo: r.repo, rulings: r.rulings})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("ruling %s — %s; tracker %s", rulingID, g.SignOffURL, r.tracker), nil
}

// refuseNeedsDecision refuses ONLY the needs-decision label — an item still on the human's
// decision queue. Unlike refuseDecisionItem (which the ruled lanes and not-planned share),
// it PERMITS a human-decided item: closing one, gated by the verified ruling and the named
// tracker, is the human-decided disposition's entire purpose.
func refuseNeedsDecision(repo string, it item) error {
	for _, have := range it.labelNames() {
		if strings.EqualFold(strings.TrimSpace(have), labelNeedsDecision) {
			return deskkit.Refused(fmt.Sprintf(
				"refused: %s#%d carries the %q label — it is still on the human's decision queue, undecided. "+
					"%s --disposition %s closes an issue whose decision is ALREADY recorded (relabelled "+
					"%s); an undecided item leaves the queue through the decision digest, not a triage close.",
				repo, it.Number, labelNeedsDecision, modeTriage, dispositionHumanDecided, "human-decided"))
		}
	}
	return nil
}

// triageComment renders the trail: the lane, the disposition, the authorization source, and
// the tracker where one is given, so a reader of the closed issue can check the authority
// rather than take the tool's word for it.
func triageComment(r triageReq, source string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Closing via the **%s** lane (disposition: **%s**).\n\n", modeTriage, r.disposition)
	fmt.Fprintf(&b, "- State reason: %s\n", reasonNotPlanned)
	fmt.Fprintf(&b, "- Authorization: %s\n", deskkit.StripControl(source))
	if r.tracker != "" {
		fmt.Fprintf(&b, "- Tracker: %s\n", deskkit.StripControl(r.tracker))
	}
	if r.disposition == dispositionNotPlanned {
		b.WriteString("\nThis is a triage skip — research concluded no brief follows. Nothing here is " +
			"irreversible: reopen it if the disposition is wrong, and the record and any tracker are named " +
			"above so the decision is auditable.\n")
	} else {
		b.WriteString("\nThis close propagates a recorded human decision, not a judgement by the tool that " +
			"performed it — the authorizing ruling and the tracker carrying the remaining work are named " +
			"above. If it is wrong, reopen it.\n")
	}
	return b.String()
}
