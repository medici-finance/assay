package main

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// superseded.go — the TWO-ROLE superseded lane.
//
// THE DEFECT. `deskdisposition set --verdict SUPERSEDED` records a worker's finding that
// a PR's scope landed through another PR, and `deskclose superseded` executed it. Both
// halves ran under whatever token the caller held, so ONE identity — the worker that
// authored the PR — could record the finding and close its own PR on it in one sitting.
// The one time it happened it was the right call (a 3-file PR closed in favour of a
// 41-file PR for the same brief), and it was unreviewed BY DESIGN: nothing in the tool
// forced a second pair of eyes onto "the other PR really does carry my scope".
//
// THE SHAPE OF THE FIX is the one the duplicate lane already has (see cmdDuplicate's
// refusal): two roles, and neither may do the other's half.
//
//	worker    PROPOSES  — label `superseded?` + a marker comment naming the target.
//	                      Closes nothing. Cannot confirm, cannot dispute.
//	reviewer  CONFIRMS  — reads the standing proposal, requires it was made by a
//	                      DIFFERENT actor, requires the target MERGED, posts the verdict
//	                      on the PR, a back-reference on the target, and closes.
//	reviewer  DISPUTES  — posts the verdict with the reason and applies `needs-decision`,
//	                      which the decision-label gate (refuseDecisionItem) then refuses
//	                      to close in every mode. From there the close is a human's.
//
// THE ROLE COMES FROM THE TOKEN, NEVER FROM A FLAG. The caller's identity is read from
// the forge (`viewer { login }` — the one identity read an App installation token can
// make about itself; the REST `/user` endpoint refuses integrations) and mapped through
// the roster binding (ASSAY_TRUSTED_BOT_SLUGS `worker=` / `reviewer=`). A flag saying
// "--as reviewer" would be a claim; the forge's answer about the token is a fact. A
// login bound to neither role is refused, an unreadable login is could-not-check, and
// a roster that binds both roles to one App is refused outright — with one App the
// two-role property is vacuous and the lane must say so rather than enforce nothing.
//
// The manifest lane is deliberately NOT role-keyed: a manifest row is authorized by a
// human whose comment carries the row set's digest (authorizeManifest), and that
// authorization is stronger than a reviewer's confirmation, not weaker.

// The two roster roles this lane keys on. The logins behind them are read from the
// roster at run time; neither is a literal anywhere in this package.
const (
	roleWorker   = "worker"
	roleReviewer = "reviewer"
)

// labelProposed is the index a queue reader and the orphan sweep filter on: "a worker
// has proposed this item is superseded; the review desk owes it a verdict".
const labelProposed = "superseded?"

// labelNeedsDecision is the label a dispute applies. It is the FIRST decision label the
// close gate refuses (decisionLabels), and TestDisputeLabelIsCloseRefused pins that the
// two agree — the dispute's whole effect is that every later close is refused.
const labelNeedsDecision = "needs-decision"

// The proposal and verdict envelopes. HTML comments so they render invisibly; versioned
// so a schema change is detectable rather than silently mis-parsed. Field lines are
// plain `Key: value` inside the envelope, tolerant of prose underneath (the same
// discipline as the deskdisposition marker, and for the same reasons).
const (
	proposalMarker   = "<!-- desk-superseded-proposal v1 -->"
	verdictMarker    = "<!-- desk-superseded-verdict v1 -->"
	verdictConfirmed = "SUPERSEDED-CONFIRMED"
	verdictDisputed  = "SUPERSEDED-DISPUTED"
)

// nowFunc is the clock seam; tests pin it so the recorded date is deterministic.
var nowFunc = time.Now

// ---------------------------------------------------------------- who is calling

// caller is the resolved identity of the token in use.
type caller struct {
	login string // as the forge reports it, e.g. "<slug>[bot]"
	role  string // roleWorker or roleReviewer
}

// resolveCaller reads the token's own identity from the forge and maps it to a role.
//
// Three answers, and they are different:
//
//	the read failed / returned no login  → Unverifiable (6). A role that could not be
//	                                       read is not a role; the lane does nothing.
//	the login is bound to neither role   → Refused (5). Not this lane's caller.
//	the login is the worker / reviewer   → that role.
//
// The roster must bind BOTH roles, to DIFFERENT Apps, before any answer is given: an
// unbound role turns the comparison into `login == ""` (the shape RoleAppLogin's ok
// return exists to forbid), and a roster that binds both roles to one App cannot host a
// two-role lane at all.
func resolveCaller() (caller, error) {
	raw, err := runGH("api", "graphql", "-f", "query={ viewer { login } }")
	if err != nil {
		return caller{}, deskkit.Unverifiable(
			"could-not-check: the identity of the token in use could not be read from the forge "+
				"(GraphQL viewer) — a role that could not be read is not a role, so the superseded lane "+
				"does nothing. This is not 'the caller is a worker' and not 'the caller is a reviewer'.", err)
	}
	var resp struct {
		Data struct {
			Viewer struct {
				Login string `json:"login"`
			} `json:"viewer"`
		} `json:"data"`
	}
	if jerr := json.Unmarshal([]byte(raw), &resp); jerr != nil {
		return caller{}, deskkit.Unverifiable("could-not-check: the forge's viewer answer did not parse", jerr)
	}
	login := strings.TrimSpace(resp.Data.Viewer.Login)
	if login == "" {
		return caller{}, deskkit.Unverifiable(
			"could-not-check: the forge reported no login for the token in use — an empty identity is "+
				"missing data, never a role", nil)
	}

	worker, wok := deskkit.RoleAppLogin(roleWorker)
	reviewer, rok := deskkit.RoleAppLogin(roleReviewer)
	if !wok {
		return caller{}, deskkit.Unverifiable("could-not-check: "+deskkit.RequireRole(roleWorker).Error(), nil)
	}
	if !rok {
		return caller{}, deskkit.Unverifiable("could-not-check: "+deskkit.RequireRole(roleReviewer).Error(), nil)
	}
	if deskkit.SameActor(worker, reviewer) {
		return caller{}, deskkit.Refused(fmt.Sprintf(
			"refused: the roster binds the %s and %s roles to the same App (%s). The superseded lane is "+
				"TWO-role — a proposal and its confirmation must come from different identities — and with "+
				"one App behind both roles that property cannot be enforced, so the lane refuses rather "+
				"than pretending to.", roleWorker, roleReviewer, deskkit.StripControl(reviewer)))
	}
	switch {
	case deskkit.SameActor(login, reviewer):
		return caller{login: login, role: roleReviewer}, nil
	case deskkit.SameActor(login, worker):
		return caller{login: login, role: roleWorker}, nil
	default:
		return caller{}, deskkit.Refused(fmt.Sprintf(
			"refused: the token in use is %s, which the roster binds to neither the %s role (%s) nor the "+
				"%s role (%s). The superseded lane is role-keyed on the TOKEN: a worker proposes, a reviewer "+
				"confirms or disputes, and nothing else runs it. A human closes on the forge directly.",
			deskkit.StripControl(login), roleWorker, deskkit.StripControl(worker),
			roleReviewer, deskkit.StripControl(reviewer)))
	}
}

// ---------------------------------------------------------------- the proposal record

// proposal is the standing record a worker leaves on the item.
//
// Author is the FORGE-ATTESTED comment author, read from the thread — never the
// self-reported Proposed-By line. The same-actor check that keeps one identity from
// proposing and confirming has to rest on something the proposer cannot write.
type proposal struct {
	Target string
	At     string
	Author string
}

// threadComment is the subset of an issue/PR comment the proposal reader consumes.
type threadComment struct {
	Body string `json:"body"`
	User struct {
		Login string `json:"login"`
	} `json:"user"`
}

// readThread fetches every comment on the item. `gh api --paginate` concatenates the
// pages as successive top-level JSON arrays; the decoder loop joins them. A read failure
// is could-not-check — a proposal that could not be read is not an absent one.
func readThread(repo string, n int) ([]threadComment, error) {
	raw, err := runGH("api", "--paginate", fmt.Sprintf("repos/%s/issues/%d/comments?per_page=100", repo, n))
	if err != nil {
		return nil, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: cannot read the comment thread on %s#%d — whether a proposal stands is "+
				"unknown, so nothing is confirmed, disputed or re-proposed", repo, n), err)
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	var out []threadComment
	for dec.More() {
		var page []threadComment
		if derr := dec.Decode(&page); derr != nil {
			return nil, deskkit.Unverifiable(fmt.Sprintf(
				"could-not-check: the comment thread on %s#%d did not parse", repo, n), derr)
		}
		out = append(out, page...)
	}
	return out, nil
}

var proposalFieldRe = regexp.MustCompile(`(?i)^\s*(Superseded-By|Proposed-By|Proposed-At)\s*:\s*(.*)$`)

// parseProposalMarker extracts a proposal from one comment body. ok is false when the
// body carries no envelope or names no target. Lines inside a fenced code block are
// skipped, so a documentation example quoting the schema never reads as a live proposal
// on the PR that adds it.
func parseProposalMarker(body string) (proposal, bool) {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	fenced, inRecord := false, false
	var p proposal
	for _, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		if strings.Contains(ln, proposalMarker) {
			inRecord = true
			continue
		}
		if !inRecord {
			continue
		}
		m := proposalFieldRe.FindStringSubmatch(ln)
		if m == nil {
			if strings.TrimSpace(ln) == "" {
				continue
			}
			break
		}
		switch strings.ToLower(m[1]) {
		case "superseded-by":
			p.Target = strings.TrimSpace(m[2])
		case "proposed-at":
			p.At = strings.TrimSpace(m[2])
		}
	}
	if strings.TrimSpace(p.Target) == "" {
		return proposal{}, false
	}
	return p, true
}

// findProposal returns the standing proposal on the item: the LAST proposal marker in
// the thread, with its forge-attested author. A worker may re-propose against a
// different target, and comments arrive in submission order, so last wins.
func findProposal(repo string, n int) (proposal, bool, error) {
	thread, err := readThread(repo, n)
	if err != nil {
		return proposal{}, false, err
	}
	var found proposal
	ok := false
	for _, c := range thread {
		if p, isP := parseProposalMarker(c.Body); isP {
			p.Author = strings.TrimSpace(c.User.Login)
			found, ok = p, true
		}
	}
	return found, ok, nil
}

func itemHasLabel(it item, want string) bool {
	for _, have := range it.labelNames() {
		if strings.EqualFold(strings.TrimSpace(have), want) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- the lane

// runSupersededLane is the item verb's entry: resolve the caller, then run that role's
// half. There is no path from a worker token to a close and no path from a reviewer
// token to a proposal; the role selects the half and the flags cannot override it.
func runSupersededLane(c common, n int, target, dispute string, out io.Writer) error {
	who, err := resolveCaller()
	if err != nil {
		return err
	}
	if who.role == roleWorker {
		if strings.TrimSpace(dispute) != "" {
			return deskkit.Refused(fmt.Sprintf(
				"refused: the token in use is the %s role, and a worker only PROPOSES. Confirming and "+
					"disputing a supersession belong to the %s role — drop --dispute; the review desk answers "+
					"the proposal.", roleWorker, roleReviewer))
		}
		return propose(c, n, target, who, out)
	}
	if strings.TrimSpace(dispute) != "" {
		return disputeProposal(c, n, target, dispute, who, out)
	}
	return confirm(c, n, target, who, out)
}

// propose is the worker's half. It writes the label and the marker comment and STOPS.
//
// Preconditions, in order: the item is readable and open; it is not on the decision
// queue; a PR target already carries its disposition record naming this target (the
// finding comes first — this tool executes findings, it does not make them); the
// superseding target exists, is in an allowed repo, and is not dead (a closed-unmerged
// PR or a closed issue). A standing proposal for the same target is an idempotent
// no-op, so a worker pass may re-run it freely.
func propose(c common, n int, target string, who caller, out io.Writer) error {
	a := &auditCtx{verb: modeSuperseded, repo: c.repo, number: n}

	it, err := fetchItem(c.repo, n)
	if err != nil {
		a.log(deskkit.ResultUnwritten, err.Error())
		return err
	}
	a.title = it.Title
	if it.closed() {
		a.log(deskkit.ResultNoop, "already closed — nothing to propose")
		fmt.Fprintf(out, "noop\t%s#%d\talready closed\n", c.repo, n)
		return nil
	}
	if err := refuseDecisionItem(c.repo, it); err != nil {
		a.log(deskkit.ResultRefused, err.Error())
		return err
	}

	var disp dispositionRead
	if it.isPR() {
		d, derr := requireTerminalDisposition(c.repo, n, target)
		if derr != nil {
			a.log(resultFor(derr), derr.Error())
			return derr
		}
		disp = d
	}

	tRepo, tN, err := resolveRef(c.repo, target)
	if err != nil {
		a.log(deskkit.ResultRefused, err.Error())
		return err
	}
	tItem, err := fetchItem(tRepo, tN)
	if err != nil {
		a.log(deskkit.ResultUnwritten, err.Error())
		return err
	}
	if tItem.isPR() {
		if tItem.closed() {
			// Closed AND merged is the strongest evidence there is; closed-unmerged is
			// work that never landed and can supersede nothing.
			if merr := requireMergedPR(tRepo, tN); merr != nil {
				a.log(resultFor(merr), merr.Error())
				return merr
			}
		}
	} else if tItem.closed() {
		err := deskkit.Refused(fmt.Sprintf(
			"refused: the superseding target %s#%d is a closed issue — proposing that a live item is "+
				"superseded by a retired one retires both", tRepo, tN))
		a.log(deskkit.ResultRefused, err.Error())
		return err
	}

	existing, found, err := findProposal(c.repo, n)
	if err != nil {
		a.log(deskkit.ResultUnwritten, err.Error())
		return err
	}
	if found && itemHasLabel(it, labelProposed) && refsAgree(existing.Target, target) {
		a.log(deskkit.ResultNoop, "a proposal for the same target already stands")
		fmt.Fprintf(out, "noop\t%s#%d\tproposal for %s already stands — awaiting the %s role\n",
			c.repo, n, deskkit.StripControl(target), roleReviewer)
		return nil
	}

	body := proposalBody(target, who, disp)
	if c.dryRun {
		a.log(deskkit.ResultDryRun, "dry-run: verified, wrote nothing")
		fmt.Fprintf(out, "dry-run\t%s#%d\tpropose\ttarget=%s\n", c.repo, n, target)
		fmt.Fprintln(out, body)
		return nil
	}

	// LABEL FIRST, comment second — the label is the index a sweep filters on; a
	// partial failure must leave the item indexed-but-unevidenced (visible, and a
	// re-run repairs it) rather than evidenced-but-invisible.
	if err := allowWrite(c.repo, n); err != nil {
		a.log(deskkit.ResultRateLimited, err.Error())
		return err
	}
	ensureProposalLabel(c.repo)
	if err := addLabel(c.repo, n, it.isPR(), labelProposed); err != nil {
		a.log(deskkit.ResultUnverifiable, err.Error())
		return err
	}
	if err := allowWrite(c.repo, n); err != nil {
		a.log(deskkit.ResultRateLimited, "label applied, proposal comment deferred: "+err.Error())
		return err
	}
	if err := postComment(c.repo, n, body); err != nil {
		a.log(deskkit.ResultUnverifiable, err.Error())
		return err
	}
	a.log(deskkit.ResultOK, fmt.Sprintf("proposed superseded by %s as %s (no close)", target, who.role))
	fmt.Fprintf(out, "proposed\t%s#%d\tsuperseded-by=%s\tawaiting the %s role's confirm or dispute — this tool did not close it\n",
		c.repo, n, deskkit.StripControl(target), roleReviewer)
	return nil
}

// confirm is the reviewer's half. It requires a standing proposal by a DIFFERENT actor
// that names the SAME target the caller declares, then runs the ordinary close — ruling
// gate, disposition record, merged target, comment, close — with two additions: the
// verdict block in the close comment, and a back-reference posted on the target BEFORE
// the close so the record is bidirectional even if the close itself fails.
func confirm(c common, n int, target string, who caller, out io.Writer) error {
	a := &auditCtx{verb: modeSuperseded, repo: c.repo, number: n}

	it, err := fetchItem(c.repo, n)
	if err != nil {
		a.log(deskkit.ResultUnwritten, err.Error())
		return err
	}
	a.title = it.Title
	if it.closed() {
		a.log(deskkit.ResultNoop, "already closed — idempotent no-op")
		fmt.Fprintf(out, "noop\t%s#%d\talready closed\n", c.repo, n)
		return nil
	}
	if err := refuseDecisionItem(c.repo, it); err != nil {
		a.log(deskkit.ResultRefused, err.Error())
		return err
	}
	p, err := standingProposal(c.repo, n, target, who, "confirm")
	if err != nil {
		a.log(resultFor(err), err.Error())
		return err
	}
	g, err := gateFor(&c)
	if err != nil {
		return err
	}
	tRepo, tN, err := resolveRef(c.repo, target)
	if err != nil {
		a.log(deskkit.ResultRefused, err.Error())
		return err
	}
	return applyClose(closeReq{
		repo: c.repo, number: n, mode: modeSuperseded,
		target: strings.TrimSpace(target), dryRun: c.dryRun, g: g,
		verdict:  &supersedeVerdict{kind: verdictConfirmed, proposal: p, by: who.login},
		crossRef: &crossRef{repo: tRepo, number: tN, body: crossRefBody(c.repo, n, p, who)},
	}, out)
}

// disputeProposal is the reviewer's other half. It posts the verdict with the reason,
// then applies `needs-decision` — after which refuseDecisionItem refuses every close in
// every mode, including a manifest row. The item is the human's from here; this tool
// has no verb that takes it back.
func disputeProposal(c common, n int, target, reason string, who caller, out io.Writer) error {
	a := &auditCtx{verb: modeSuperseded, repo: c.repo, number: n}

	it, err := fetchItem(c.repo, n)
	if err != nil {
		a.log(deskkit.ResultUnwritten, err.Error())
		return err
	}
	a.title = it.Title
	if it.closed() {
		err := deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d is already closed — a dispute parks an OPEN proposal on the decision queue; "+
				"reopening a closed item is a human act on the forge, and this tool has no reopen verb",
			c.repo, n))
		a.log(deskkit.ResultRefused, err.Error())
		return err
	}
	if itemHasLabel(it, labelNeedsDecision) {
		a.log(deskkit.ResultNoop, "already on the decision queue")
		fmt.Fprintf(out, "noop\t%s#%d\talready carries %s — it is the human's\n", c.repo, n, labelNeedsDecision)
		return nil
	}
	if err := refuseDecisionItem(c.repo, it); err != nil {
		a.log(deskkit.ResultRefused, err.Error())
		return err
	}
	p, err := standingProposal(c.repo, n, target, who, "dispute")
	if err != nil {
		a.log(resultFor(err), err.Error())
		return err
	}

	body := disputeBody(p, who, reason)
	if c.dryRun {
		a.log(deskkit.ResultDryRun, "dry-run: verified, wrote nothing")
		fmt.Fprintf(out, "dry-run\t%s#%d\tdispute\ttarget=%s\n", c.repo, n, p.Target)
		fmt.Fprintln(out, body)
		return nil
	}

	// COMMENT FIRST, label second. A bare `needs-decision` is unanswerable — the
	// escalation vocabulary requires the reason on the record — so the reason lands
	// before the label that puts the item in front of a human.
	if err := allowWrite(c.repo, n); err != nil {
		a.log(deskkit.ResultRateLimited, err.Error())
		return err
	}
	if err := postComment(c.repo, n, body); err != nil {
		a.log(deskkit.ResultUnverifiable, err.Error())
		return err
	}
	if err := allowWrite(c.repo, n); err != nil {
		a.log(deskkit.ResultRateLimited, "dispute posted, needs-decision label deferred: "+err.Error())
		return err
	}
	if err := addLabel(c.repo, n, it.isPR(), labelNeedsDecision); err != nil {
		// The label is not this tool's to provision: it is the human decision queue's,
		// shipped by the adoption guide. Say exactly what is missing.
		a.log(deskkit.ResultUnverifiable, err.Error())
		return deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: the dispute was posted on %s#%d but the %q label did not apply — the item "+
				"is NOT yet human-only-close. Provision the label in the repo and re-run; the re-run is a "+
				"no-op on the comment.", c.repo, n, labelNeedsDecision), err)
	}
	a.log(deskkit.ResultOK, fmt.Sprintf("disputed the supersession by %s; %s applied (no close)", p.Target, labelNeedsDecision))
	fmt.Fprintf(out, "disputed\t%s#%d\tsuperseded-by=%s\t%s applied — human-only close from here\n",
		c.repo, n, deskkit.StripControl(p.Target), labelNeedsDecision)
	return nil
}

// standingProposal is the reviewer-side precondition shared by confirm and dispute: a
// proposal exists, it was made by someone other than the caller, and it names the
// target the caller declares.
func standingProposal(repo string, n int, target string, who caller, half string) (proposal, error) {
	p, found, err := findProposal(repo, n)
	if err != nil {
		return proposal{}, err
	}
	if !found {
		return proposal{}, deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d carries no standing proposal (no %s comment), so there is nothing to %s. "+
				"A %s answers a %s's proposal; it does not originate the supersession — the two-role "+
				"property is the point of the lane. Have the worker run `%s %s -R %s %d --by <ref>` first.",
			repo, n, proposalMarker, half, roleReviewer, roleWorker, toolName, modeSuperseded, repo, n))
	}
	if deskkit.SameActor(p.Author, who.login) {
		return proposal{}, deskkit.Refused(fmt.Sprintf(
			"refused: the standing proposal on %s#%d was posted by %s — the same identity now asking to %s "+
				"it. A proposal and its verdict must come from different actors; one identity doing both "+
				"is the single-actor close this lane exists to prevent.",
			repo, n, deskkit.StripControl(p.Author), half))
	}
	if !refsAgree(p.Target, target) {
		return proposal{}, deskkit.Refused(fmt.Sprintf(
			"refused: the standing proposal on %s#%d names %s as the superseding item, but this %s declares "+
				"%s. The record and the caller disagree about what settled it; deskclose does not pick a "+
				"winner — re-propose against the right target, or %s the proposal as it stands.",
			repo, n, deskkit.StripControl(p.Target), half, deskkit.StripControl(target), "dispute"))
	}
	return p, nil
}

// ---------------------------------------------------------------- the writes

// supersedeVerdict is the reviewer's verdict, rendered into the close comment.
type supersedeVerdict struct {
	kind     string
	proposal proposal
	by       string
}

// crossRef is the back-reference posted on the superseding target before the close, so
// a reader of the SURVIVING item sees what it retired.
type crossRef struct {
	repo   string
	number int
	body   string
}

func (v *supersedeVerdict) block() string {
	var b strings.Builder
	b.WriteString(verdictMarker + "\n")
	fmt.Fprintf(&b, "Superseded-Verdict: %s\n", v.kind)
	fmt.Fprintf(&b, "Superseded-By: %s\n", deskkit.StripControl(v.proposal.Target))
	fmt.Fprintf(&b, "Proposed-By: %s\n", deskkit.StripControl(v.proposal.Author))
	fmt.Fprintf(&b, "Verdict-By: %s\n", deskkit.StripControl(v.by))
	return b.String()
}

func proposalBody(target string, who caller, disp dispositionRead) string {
	var b strings.Builder
	b.WriteString(proposalMarker + "\n")
	fmt.Fprintf(&b, "Superseded-By: %s\n", deskkit.StripControl(target))
	fmt.Fprintf(&b, "Proposed-By: %s\n", deskkit.StripControl(who.login))
	fmt.Fprintf(&b, "Proposed-At: %s\n", nowFunc().UTC().Format("2006-01-02"))
	b.WriteString("\n")
	fmt.Fprintf(&b, "**Proposal — superseded by %s.** The %s role records that this item's scope landed "+
		"through %s. This is a PROPOSAL, not a close: the item stays open, labelled `%s`, until the %s "+
		"role confirms it (`%s` — the reviewer closes) or disputes it (`%s` — `%s` is applied and the "+
		"close is a human's).\n",
		deskkit.StripControl(target), roleWorker, deskkit.StripControl(target), labelProposed, roleReviewer,
		verdictConfirmed, verdictDisputed, labelNeedsDecision)
	if disp.Record.Verdict != "" {
		fmt.Fprintf(&b, "\n- Disposition record: %s — evidence %s (recorded %s by %s)\n",
			deskkit.StripControl(disp.Record.Verdict), deskkit.StripControl(disp.Record.Evidence),
			deskkit.StripControl(disp.Record.RecordedAt), deskkit.StripControl(disp.Record.RecordedBy))
	}
	return b.String()
}

func disputeBody(p proposal, who caller, reason string) string {
	v := &supersedeVerdict{kind: verdictDisputed, proposal: p, by: who.login}
	var b strings.Builder
	b.WriteString(v.block())
	fmt.Fprintf(&b, "Reason: %s\n\n", deskkit.StripControl(strings.TrimSpace(reason)))
	fmt.Fprintf(&b, "**%s: %s**\n\n", verdictDisputed, deskkit.StripControl(strings.TrimSpace(reason)))
	fmt.Fprintf(&b, "The %s role does not agree that %s carries this item's scope. `%s` is applied: from "+
		"here the close is a human's — deskclose refuses every close on a decision-labelled item, in "+
		"every mode. The proposal by %s stands on the record above it; nothing here is irreversible.\n",
		roleReviewer, deskkit.StripControl(p.Target), labelNeedsDecision, deskkit.StripControl(p.Author))
	return b.String()
}

func crossRefBody(repo string, n int, p proposal, who caller) string {
	return fmt.Sprintf("Supersedes %s#%d — %s by %s (proposed by %s). The superseded item's close comment "+
		"carries the verdict record; this note is the back-reference so the record reads in both directions.\n",
		repo, n, verdictConfirmed, deskkit.StripControl(who.login), deskkit.StripControl(p.Author))
}

// addLabel applies one label. Issues and PRs take different gh verbs; the label name is
// drawn from this package's constants, never from the caller, so it cannot be an option.
func addLabel(repo string, n int, isPR bool, label string) error {
	kind := "issue"
	if isPR {
		kind = "pr"
	}
	if _, err := runGH(kind, "edit", fmt.Sprintf("%d", n), "-R", repo, "--add-label", label); err != nil {
		return deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: applying %q to %s#%d did not confirm", label, repo, n), err)
	}
	return nil
}

// ensureProposalLabel creates this tool's OWN label when the repo lacks it. A repo that
// has never carried a proposal would otherwise fail every first propose. Failure here is
// not fatal on its own: the add-label call that follows is the real gate.
func ensureProposalLabel(repo string) {
	have, err := runGH("label", "list", "-R", repo, "--search", labelProposed, "--json", "name")
	if err == nil && strings.Contains(have, fmt.Sprintf("%q", labelProposed)) {
		return
	}
	_, _ = runGH("label", "create", labelProposed, "-R", repo,
		"--description", "worker proposes this item is superseded; the review desk owes it a confirm or dispute",
		"--color", "FBCA04")
}
