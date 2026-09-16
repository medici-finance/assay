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
// The gap it fills: `deskclose`'s other lanes retire a PR (superseded / review-request /
// self-withdraw), fold a duplicate, or run a human-ruled batch — none closes an idea-issue
// whose triage disposition is the plain "research says skip, no brief" (not-planned) or one
// whose human decision has been recorded (human-decided). A raw `gh issue close --reason
// "not planned"` is denied to an auto-mode session as an external write, so a triaged
// idea-issue kept its disposition comment but could not be closed by any sanctioned path.
//
// AUTHORITY MODEL. A close verb is an authority surface — it closes another party's issue and
// writes WHY — so, exactly like every other deskclose lane, each disposition traces to an
// artifact deskclose FETCHED and VERIFIED, never to a flag the caller set or a login/id it
// claims. The two dispositions bind to two different artifacts (security review of PR #1211):
//
//	not-planned    the triage-disposition marker comment on issue N — read from the FETCHED
//	               comment thread, and authorized ONLY when it is (a) authored by a
//	               roster-trusted account (deskkit.TrustedAuthorID, login AND id) and (b) not
//	               minimized. A bare marker string any commenter could paste, a minimized
//	               marker, or a marker by an untrusted author does NOT authorize (S2). It does
//	               NOT call the R-1 ruling gate — the same structural reason self-withdraw /
//	               verify-gate-refire do not (lanes.go); the disposition record IS the per-item
//	               authority. `--tracker` is NEVER authority; when given it must name a FETCHED
//	               existing item (S3), recorded as where residual work lives.
//
//	human-decided  the human's OWN ruling comment ON issue N (`--decision <url>`) — fetched via
//	               fetchComment and its author verified against the roster-pinned blessing
//	               authority via verifyHumanAuthor, exactly as every ruled artifact is, and
//	               required to be ON the issue being closed (a ruling from elsewhere cannot be
//	               reused here). A blanket ruling grant and a caller-typed `--tracker` never
//	               stand in for it (S1). `--tracker` is REQUIRED and must name a FETCHED
//	               existing item: the recorded-decision close NAMES the work that continues,
//	               never asserts it. The cited authority in the close comment is THAT decision
//	               comment's URL, not a blanket ruling's.
//
// THE CONTROL THIS LANE KEEPS (worker §2, #1207). triage must never close a `needs-decision`
// issue (still on the human's decision queue), and never close an item lacking its required
// fetched artifact. not-planned refuses BOTH decision labels; human-decided refuses
// `needs-decision` but PERMITS a `human-decided` item — closing one, gated by the verified
// decision comment and the named tracker, is the sanctioned lane this verb adds.
//
// A non-issue target (a pull request) is refused; an already-closed issue is an idempotent
// no-op; an unreadable item is could-not-check (exit 6), never a guessed close.

const modeTriage = "triage"

// The two dispositions. The set is CLOSED: an unknown value is refused, never coerced.
const (
	dispositionNotPlanned   = "not-planned"
	dispositionHumanDecided = "human-decided"
)

// triageDispositionMarker is the machine-readable envelope a triage session posts on an issue
// when it records a rejected/watching disposition — the issue-side analog of the PR disposition
// record's `<!-- desk-disposition v1 -->` marker (deskkit/disposition.go). not-planned
// authorizes ONLY on this marker when carried by a trusted, non-minimized comment; the marker
// STRING alone is not authority. No writer ships in this PR, so not-planned fails CLOSED until
// intake stamps a trusted marker — by design, never a close on an unverified signal.
const triageDispositionMarker = "<!-- desk-triage v1 -->"

func dispositions() []string { return []string{dispositionNotPlanned, dispositionHumanDecided} }

// triageReq is one triage close, fully specified before any forge write.
type triageReq struct {
	repo        string
	number      int
	disposition string
	// tracker is the parsed/allowlisted ref naming where residual or decided work lives, or nil
	// when the caller gave none. Its EXISTENCE is verified (a fetch) before it is recorded; it
	// is never an authority.
	tracker *resolvedRef
	// decision is the human's ruling-comment permalink on issue N (human-decided only).
	decision string
	kind     deskkit.TargetKind
	dryRun   bool
}

func cmdTriage(args []string, out io.Writer) error {
	fs := flag.NewFlagSet(modeTriage, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var c common
	c.bind(fs)
	disposition := fs.String("disposition", "",
		"the triage disposition: "+dispositionNotPlanned+" (a research-says-skip close, authorized by a "+
			"trusted disposition marker on the issue) | "+dispositionHumanDecided+" (a recorded human "+
			"decision, authorized by the human's ruling comment on the issue via --decision)")
	tracker := fs.String("tracker", "",
		"the EXISTING item recording where residual or decided work lives "+
			"(N | #N | !N | owner/repo#N | owner/repo!N | web URL); verified present, never an authority")
	decision := fs.String("decision", "",
		dispositionHumanDecided+" only: permalink to the human's ruling comment ON this issue "+
			"(https://github.com/<owner>/<repo>/issues/<N>#issuecomment-<id>) — fetched and its author verified")
	n, err := parseItemVerb(fs, args, &c)
	if err != nil {
		return err
	}
	// A triage close acts on an ISSUE. A stated change kind names an object this lane never
	// touches; refuse it pre-flight. (A bare number that RESOLVES to a PR is caught after read.)
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

	dec := strings.TrimSpace(*decision)
	trk := strings.TrimSpace(*tracker)
	switch disp {
	case dispositionNotPlanned:
		if dec != "" {
			return deskkit.Refused(fmt.Sprintf(
				"refused: --decision is only for --disposition %s (the human's ruling comment). A not-planned "+
					"close is authorized by the trusted disposition marker on the issue, not a decision link.",
				dispositionHumanDecided))
		}
	case dispositionHumanDecided:
		if dec == "" {
			return deskkit.Refused(fmt.Sprintf(
				"refused: --disposition %s requires --decision <url> — the human's ruling comment ON this "+
					"issue, fetched and author-verified. A signed rulings register is a blanket grant, not a "+
					"decision about this issue.", dispositionHumanDecided))
		}
		if trk == "" {
			return deskkit.Refused(fmt.Sprintf(
				"refused: --disposition %s requires --tracker <ref> — the recorded-decision close must NAME "+
					"the existing brief, PR or issue carrying the remaining work.", dispositionHumanDecided))
		}
	}

	var trackerRef *resolvedRef
	if trk != "" {
		r, err := resolveRef(c.repo, trk, "")
		if err != nil {
			return err
		}
		trackerRef = &r
	}
	return applyTriage(triageReq{
		repo: c.repo, number: n, disposition: disp, tracker: trackerRef,
		decision: dec, kind: c.kind, dryRun: c.dryRun,
	}, out)
}

// applyTriage is the lane's own precondition chain. Fixed order, matching the ruled lanes:
//
//  1. read the item                 — a read failure is exit 6, never an assumption
//  2. non-issue target → refused    — a pull request is the wrong object, whatever its state
//  3. idempotent no-op              — an already-closed issue is success
//  4. authorization by disposition  — decision-label gate, then the disposition's fetched artifact,
//     then --tracker existence (never authority)
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
		fmt.Fprintf(out, "dry-run\t%s#%d\t%s\tdisposition=%s\n", r.repo, r.number, modeTriage, r.disposition)
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

// authorizeTriage applies the decision-label gate, the disposition's fetched-and-verified
// artifact, and then --tracker existence (never authority). It returns a human-legible SOURCE
// naming the fetched artifact for the close comment to cite.
func authorizeTriage(r triageReq, it item) (string, error) {
	var src string
	switch r.disposition {
	case dispositionNotPlanned:
		// A decision item — needs-decision OR human-decided — is never a plain skip.
		if err := refuseDecisionItem(r.repo, it); err != nil {
			return "", err
		}
		s, err := notPlannedAuthority(r)
		if err != nil {
			return "", err
		}
		src = s
	case dispositionHumanDecided:
		// Still on the human's decision queue → refused; a human-decided label is PERMITTED.
		if err := refuseNeedsDecision(r.repo, it); err != nil {
			return "", err
		}
		s, err := humanDecidedAuthority(r)
		if err != nil {
			return "", err
		}
		src = s
	default:
		return "", deskkit.Refused(fmt.Sprintf("refused: unknown disposition %q", deskkit.StripControl(r.disposition)))
	}

	if r.tracker != nil {
		if err := requireTrackerExists(*r.tracker); err != nil {
			return "", err
		}
		src += "; tracker " + renderRef(*r.tracker) + " (verified present)"
	}
	return src, nil
}

// notPlannedAuthority authorizes on a triage-disposition marker comment on issue N that is
// authored by a roster-trusted account (login AND id) and not minimized — read from the
// FETCHED comment thread. An unreadable thread is could-not-check (exit 6), never a guess; a
// marker by an untrusted author, or a minimized one, does not authorize.
func notPlannedAuthority(r triageReq) (string, error) {
	fg, fr, ferr := forgeForFn(r.repo)
	if ferr != nil {
		return "", ferr
	}
	comments, err := fg.ListCommentsTyped(fr, r.number, deskkit.TargetIssue)
	if err != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: cannot read the comments on %s#%d to find a triage disposition record "+
				"(%s) — an unreadable thread is not an absent record, so the close is refused",
			r.repo, r.number, triageDispositionMarker), err)
	}
	sawUntrusted := false
	for _, c := range comments {
		if !strings.Contains(c.Body, triageDispositionMarker) {
			continue
		}
		if c.Minimized {
			continue // a hidden/collapsed marker does not authorize
		}
		if !deskkit.TrustedAuthorID(c.Author.Login, c.Author.ID) {
			sawUntrusted = true
			continue // an untrusted author's marker is not authority
		}
		url := c.URL
		if url == "" {
			url = fmt.Sprintf("%s#%d", r.repo, r.number)
		}
		return fmt.Sprintf("recorded triage disposition %s (marker by %s, verified trusted)",
			deskkit.StripControl(url), deskkit.StripControl(c.Author.Login)), nil
	}
	detail := "no " + triageDispositionMarker + " marker comment is present"
	if sawUntrusted {
		detail = "the only " + triageDispositionMarker + " marker present is authored by an untrusted account " +
			"(or is minimized)"
	}
	return "", deskkit.Refused(fmt.Sprintf(
		"refused: %s#%d has no triage disposition record authorizing a not-planned close — %s. A not-planned "+
			"close is authorized ONLY by a %s marker comment from a roster-trusted account; --tracker names "+
			"residual work but never substitutes for the record. Record the disposition first.",
		r.repo, r.number, detail, triageDispositionMarker))
}

// humanDecidedAuthority authorizes on the human's OWN ruling comment ON issue N: fetched via
// fetchComment and its author verified as the roster-pinned blessing authority via
// verifyHumanAuthor, exactly as every ruled artifact is. The permalink must be ON the issue
// being closed — a ruling from elsewhere cannot be reused here.
func humanDecidedAuthority(r triageReq) (string, error) {
	m := commentURLRe.FindStringSubmatch(strings.TrimSpace(r.decision))
	if m == nil {
		return "", deskkit.Refused(
			"refused: --decision must be a GitHub comment permalink " +
				"(https://github.com/<owner>/<repo>/issues|pull/<N>#issuecomment-<id>) naming the human's " +
				"ruling comment on this issue — a link to a thread is not an authorization.")
	}
	owner, repo, itemStr := m[1], m[2], m[4]
	if !strings.EqualFold(owner+"/"+repo, r.repo) || itemStr != fmt.Sprint(r.number) {
		return "", deskkit.Refused(fmt.Sprintf(
			"refused: --decision names a comment on %s#%s, but the close targets %s#%d — the recorded human "+
				"decision must be ON the issue being closed, never a ruling from elsewhere reused here.",
			deskkit.StripControl(owner+"/"+repo), deskkit.StripControl(itemStr), r.repo, r.number))
	}
	// TargetIssue, never the untyped fetch: the decision comment lives on an ISSUE, whose
	// thread the untyped ListComments cannot read (it uses the change/pullRequest selection on
	// both backends), so an untyped fetch would return could-not-check at every real issue and
	// the lane could never execute.
	c, err := fetchCommentTyped(strings.TrimSpace(r.decision), deskkit.TargetIssue)
	if err != nil {
		return "", err
	}
	if err := verifyHumanAuthor(c, "the recorded human decision (--decision)"); err != nil {
		return "", err
	}
	return fmt.Sprintf("recorded human decision %s (by %s, verified blessing authority)",
		deskkit.StripControl(c.HTMLURL), deskkit.StripControl(c.User.Login)), nil
}

// requireTrackerExists confirms a --tracker names a REAL item (a fetch), so the close records
// an existing destination rather than an unverified caller string. A tracker that cannot be
// confirmed refuses the close — could-not-check is never a pass.
func requireTrackerExists(ref resolvedRef) error {
	if _, err := fetchItem(ref.repo, ref.number, ref.kind); err != nil {
		return deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: --tracker %s could not be confirmed to exist — a triage close names an EXISTING "+
				"item as where residual/decided work lives, never an unverified reference", renderRef(ref)), err)
	}
	return nil
}

// refuseNeedsDecision refuses ONLY the needs-decision label — an item still on the human's
// decision queue. Unlike refuseDecisionItem (which the ruled lanes and not-planned share), it
// PERMITS a human-decided item: closing one, gated by the verified decision comment and the
// named tracker, is the human-decided disposition's entire purpose.
func refuseNeedsDecision(repo string, it item) error {
	for _, have := range it.labelNames() {
		if strings.EqualFold(strings.TrimSpace(have), labelNeedsDecision) {
			return deskkit.Refused(fmt.Sprintf(
				"refused: %s#%d carries the %q label — it is still on the human's decision queue, undecided. "+
					"%s --disposition %s closes an issue whose decision is ALREADY recorded (relabelled "+
					"human-decided); an undecided item leaves the queue through the decision digest, not a "+
					"triage close.", repo, it.Number, labelNeedsDecision, modeTriage, dispositionHumanDecided))
		}
	}
	return nil
}

// triageComment renders the trail: the lane, the disposition, and the FETCHED authorizing
// artifact (a trusted marker, or the human's decision comment) that a reader can check.
func triageComment(r triageReq, source string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Closing via the **%s** lane (disposition: **%s**).\n\n", modeTriage, r.disposition)
	fmt.Fprintf(&b, "- State reason: %s\n", reasonNotPlanned)
	fmt.Fprintf(&b, "- Authorization: %s\n", deskkit.StripControl(source))
	if r.disposition == dispositionNotPlanned {
		b.WriteString("\nThis is a triage skip — research concluded no brief follows, and a roster-trusted " +
			"disposition record on this issue authorizes the close. Nothing here is irreversible: reopen it " +
			"if the disposition is wrong; the record and any tracker are named above so it is auditable.\n")
	} else {
		b.WriteString("\nThis close propagates the human's own recorded decision ON this issue (fetched and " +
			"author-verified, cited above), not a judgement by the tool that performed it — the tracker " +
			"carrying the remaining work is named above. If it is wrong, reopen it.\n")
	}
	return b.String()
}
