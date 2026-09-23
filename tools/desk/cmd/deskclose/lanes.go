package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// lanes.go — the two IDENTITY+STRUCTURE lanes (forge-neutral brief 16).
//
// Every mode in verbs.go authorizes on a FETCHED HUMAN ARTIFACT: the R-1 ruling gate
// (gateFor/authorize) and, for a batch, the digest-bound manifest comment. Those gates exist
// because those lanes close OTHER people's items. The two lanes here close nothing that
// belongs to another party, so neither calls gateFor — structurally, not by omission:
//
//	self-withdraw        the authoring App closes its OWN unreviewed draft. The authority is
//	                     the one a human already has over their own pull request; the control
//	                     is an AUTHORSHIP PIN — login AND roster-pinned numeric bot id, the
//	                     same two-field shape IsBlessAuthorityIDStrict uses for the human
//	                     blessing authority, applied here to the App's own identity.
//	verify-gate-refire   the verifier session reopens and re-closes a CLOSED issue carrying
//	                     the verify-gate label, so the card's close event fires again. The
//	                     control is a ROLE+LABEL PIN, and the independent second layer is not
//	                     in this package at all: the repository's verify-gate close workflow
//	                     unconditionally reopens any close whose sender is not an allowlisted
//	                     human, so a bot's close of a verify-gate issue can never complete the
//	                     human sign-off, whatever this lane decides.
//
// Both lanes share postComment / closeItem / refuseDecisionItem / allowWrite with the ruled
// lanes and run their OWN precondition chains. Neither is a manifest row mode (rowModes):
// neither is a human-ruled batch primitive. Neither adds a flag that bypasses a refusal —
// TestNoForceEscapeHatch scans this file with the rest of the package.

// The two modes, beside the four in verbs.go. modes() lists all six; rowModes() lists
// neither of these.
const (
	modeSelfWithdraw     = "self-withdraw"
	modeVerifyGateRefire = "verify-gate-refire"
)

// roleVerifier is the roster role a verify-desk session mints under
// (deskkit.SessionTokenRole binds the verify-desk loop to it). It is the ONE role
// verify-gate-refire accepts; the two-role superseded lane never matches it.
const roleVerifier = "verifier"

// The --because vocabulary of self-withdraw. Closed: an unknown reason is a refusal.
const (
	becauseSuperseded = "superseded"
	becauseAbandoned  = "abandoned"
)

// ---------------------------------------------------------------- self-withdraw

// selfWithdrawReq is one self-withdraw, fully specified before any forge read.
type selfWithdrawReq struct {
	repo    string
	number  int
	because string
	// by is the item the author says replaces this draft, in its resolved spelling
	// (`owner/repo#N` or `owner/repo!N`). Recorded in the close comment, NOT verified merged:
	// this is the author's own claim about its own item, the same as a human closing their
	// own PR needs no second party to confirm the reason.
	by     string
	dryRun bool
}

func cmdSelfWithdraw(args []string, out io.Writer) error {
	fs := flag.NewFlagSet(modeSelfWithdraw, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var c common
	c.bind(fs)
	because := fs.String("because", "", "why the author withdraws its own draft: "+
		becauseSuperseded+" (requires --by) | "+becauseAbandoned+" (refuses --by)")
	by := fs.String("by", "", "with --because "+becauseSuperseded+": the item that replaces this draft "+
		"(N | #N | !N | owner/repo#N | owner/repo!N | web URL) — recorded in the close comment, not verified")
	n, err := parseItemVerb(fs, args, &c)
	if err != nil {
		return err
	}
	// The item is a CHANGE by construction — this lane reads it through GetPullRequest and
	// never through the issue sequence — so a stated issue kind names an object the lane
	// does not act on. Refused pre-flight, before any read.
	if c.kind == deskkit.TargetIssue {
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s acts on a pull request / merge request of your own authorship, and the item was "+
				"stated as an issue (--kind issue). An issue is not this lane's business; state the change "+
				"(`!N` or --kind mr) or omit the kind.", modeSelfWithdraw))
	}
	why := strings.TrimSpace(*because)
	target := strings.TrimSpace(*by)
	switch why {
	case becauseSuperseded:
		if target == "" {
			return deskkit.Refused(fmt.Sprintf(
				"refused: %s --because %s requires --by <ref>, the item that replaces this draft. It is "+
					"recorded, not verified: the author's own statement about its own proposal.",
				modeSelfWithdraw, becauseSuperseded))
		}
	case becauseAbandoned:
		if target != "" {
			return deskkit.Refused(fmt.Sprintf(
				"refused: %s --because %s takes no --by — an abandoned draft is replaced by nothing; if "+
					"something replaces it, say --because %s.", modeSelfWithdraw, becauseAbandoned, becauseSuperseded))
		}
	case "":
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s requires --because {%s|%s}", modeSelfWithdraw, becauseSuperseded, becauseAbandoned))
	default:
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s --because %q is not a withdrawal reason this lane records; the set is CLOSED: %s | %s",
			modeSelfWithdraw, deskkit.StripControl(why), becauseSuperseded, becauseAbandoned))
	}
	byRef := ""
	if target != "" {
		// Parsed and allowlisted (a ref in a repo outside the desk's set is refused), never
		// READ: nothing here verifies the target merged, by design.
		r, err := resolveRef(c.repo, target, "")
		if err != nil {
			return err
		}
		byRef = renderRef(r)
	}
	return applySelfWithdraw(selfWithdrawReq{
		repo: c.repo, number: n, because: why, by: byRef, dryRun: c.dryRun,
	}, out)
}

// renderRef spells a resolved reference with the kind the caller stated, so the record
// carries the typed form where the caller used one and the neutral form otherwise.
func renderRef(r resolvedRef) string {
	if r.kind == deskkit.TargetChange {
		return fmt.Sprintf("%s!%d", r.repo, r.number)
	}
	return fmt.Sprintf("%s#%d", r.repo, r.number)
}

// actingIdentity resolves the session-role App's OWN identity: the rendered login the
// roster binds the minted role to, and the roster-pinned numeric bot id. Both halves are
// required, and an unpinned id (0) is a could-not-check REFUSAL — never a login-only pass.
// This is the two-field read RoleBotCommitIdentity already makes to construct a commit
// address, used here for a comparison instead.
type actingID struct {
	login string
	id    int64
}

func actingIdentity() (actingID, error) {
	login, ok := deskkit.RoleAppLogin(mintedRole)
	if !ok {
		return actingID{}, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: this session's App role %q is not bound to an App in the roster, so the "+
				"acting identity is unknown and no authorship can be established",
			deskkit.StripControl(mintedRole)), nil)
	}
	ident, ok := deskkit.EffectiveConfig().RoleBotIdentity(mintedRole)
	if !ok || ident.ID == 0 {
		return actingID{}, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: the roster binds role %q to %s but pins no numeric bot id for it — the "+
				"authorship pin is login AND id, and a login-only match is exactly the same-named-account "+
				"spoof the id exists to refuse; pin the id (`role=<slug>:<id>`) before this lane can run",
			deskkit.StripControl(mintedRole), deskkit.StripControl(login)), nil)
	}
	return actingID{login: login, id: ident.ID}, nil
}

// applySelfWithdraw is the lane's own precondition chain. Fixed order:
//
//  1. read the change (GetPullRequest)   — an issue number or an unreadable change is exit 6
//  2. idempotent no-op                   — already closed is success
//  3. decision-label refusal             — UNCHANGED, absolute, before the lane's own checks
//  4. draft gate                         — a PR out for review is not this lane's business
//  5. authorship pin, login AND id       — the single control; each half refuses by name
//  6. comment, then close                — the trail lands before the close
//
// gateFor/authorize is NEVER called here. R-1 grants the desk authority over OTHER people's
// items; an author withdrawing its own unreviewed draft exercises the authority a human
// already has over their own pull request, no ruling and no disposition record required.
// TestSelfWithdrawIgnoresUnsignedRuling pins that with R-1 unsigned.
func applySelfWithdraw(r selfWithdrawReq, out io.Writer) error {
	a := &auditCtx{verb: modeSelfWithdraw, repo: r.repo, number: r.number}

	fg, fr, ferr := forgeForFn(r.repo)
	if ferr != nil {
		a.log(deskkit.ResultUnwritten, ferr.Error())
		return ferr
	}
	pr, err := fg.GetPullRequest(fr, r.number)
	if err != nil {
		err = deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: cannot read %s#%d as a pull request — %s acts only on a change of the "+
				"caller's own authorship, and an issue number, or a change whose draft flag, author and "+
				"labels could not be read, is not one", r.repo, r.number, modeSelfWithdraw), err)
		a.log(deskkit.ResultUnwritten, err.Error())
		return err
	}
	a.title = pr.Title

	if strings.EqualFold(pr.State, "closed") || pr.Merged {
		a.log(deskkit.ResultNoop, "already closed — idempotent no-op")
		fmt.Fprintf(out, "noop\t%s#%d\talready closed\n", r.repo, r.number)
		return nil
	}
	it := item{Number: pr.Number, Title: pr.Title, State: pr.State, Labels: append([]string(nil), pr.Labels...), IsPR: true}
	if err := refuseDecisionItem(r.repo, it); err != nil {
		a.log(deskkit.ResultRefused, err.Error())
		return err
	}
	if !pr.Draft {
		err := deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d is not a draft. %s closes a DRAFT of your own authorship; a PR out for review "+
				"is not this lane's business — it has reviewers whose findings decide its fate, and the "+
				"lanes that retire it (superseded, review-request) run on a recorded finding under R-1.",
			r.repo, r.number, modeSelfWithdraw))
		a.log(deskkit.ResultRefused, err.Error())
		return err
	}
	self, err := actingIdentity()
	if err != nil {
		a.log(deskkit.ResultUnwritten, err.Error())
		return err
	}
	// THE AUTHORSHIP PIN. Login first (cheap; the rendering-normalised compare the two-role
	// lane already uses), then the numeric id (the half a same-named account cannot spoof).
	// Each half refuses by name; neither silently falls through to the other.
	if !deskkit.SameActor(pr.Author.Login, self.login) {
		err := deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d was authored by %s, and this session acts as the %s role (%s). %s withdraws "+
				"the acting App's OWN draft and nobody else's — a different author's item is not this "+
				"lane's business, whatever its state.",
			r.repo, r.number, deskkit.StripControl(pr.Author.Login), deskkit.StripControl(mintedRole),
			deskkit.StripControl(self.login), modeSelfWithdraw))
		a.log(deskkit.ResultRefused, err.Error())
		return err
	}
	if pr.Author.ID != self.id {
		err := deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d's author login %s matches the %s role's rendered login, but its numeric id %d is "+
				"not the roster-pinned bot id %d. The authorship pin is login AND id: a login match alone is "+
				"the same-named-account shape the id exists to refuse, so this is not the acting App's own draft.",
			r.repo, r.number, deskkit.StripControl(pr.Author.Login), deskkit.StripControl(mintedRole),
			pr.Author.ID, self.id))
		a.log(deskkit.ResultRefused, err.Error())
		return err
	}

	body := selfWithdrawComment(r, self)
	if r.dryRun {
		a.log(deskkit.ResultDryRun, "dry-run: verified, wrote nothing")
		fmt.Fprintf(out, "dry-run\t%s#%d\t%s\tbecause=%s\tby=%s\n", r.repo, r.number, modeSelfWithdraw, r.because, r.by)
		fmt.Fprintln(out, body)
		return nil
	}
	if err := allowWrite(r.repo, r.number); err != nil {
		a.log(deskkit.ResultRateLimited, err.Error())
		return err
	}
	if err := postComment(r.repo, r.number, deskkit.TargetChange, body); err != nil {
		a.log(deskkit.ResultUnverifiable, err.Error())
		return err
	}
	a.log(deskkit.ResultOK, "posted the withdrawal comment")
	if err := allowWrite(r.repo, r.number); err != nil {
		a.log(deskkit.ResultRateLimited, "comment posted, close deferred: "+err.Error())
		return err
	}
	// reasonNotPlanned is the lane's disposition; closeItem drops it for a change (no forge
	// records a state reason there), and the comment above carries it instead.
	if err := closeItem(r.repo, r.number, deskkit.TargetChange, reasonNotPlanned, true); err != nil {
		a.log(deskkit.ResultUnverifiable, "partial: comment posted, close refused: "+err.Error())
		return deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s#%d — partial: comment posted, close refused", r.repo, r.number), err)
	}
	a.log(deskkit.ResultOK, fmt.Sprintf("closed via lane %s (because %s, by %s) as %s",
		modeSelfWithdraw, r.because, r.by, deskkit.StripControl(self.login)))
	fmt.Fprintf(out, "closed\t%s#%d\t%s\tbecause=%s\tby=%s\n", r.repo, r.number, modeSelfWithdraw, r.because, r.by)
	return nil
}

// selfWithdrawComment renders the trail. It states the reason, the replacing item where
// there is one, the acting identity, and — explicitly — that no ruling artifact is cited.
func selfWithdrawComment(r selfWithdrawReq, self actingID) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Closing via the **%s** lane.\n\n", modeSelfWithdraw)
	fmt.Fprintf(&b, "- Because: %s\n", r.because)
	if r.by != "" {
		fmt.Fprintf(&b, "- Replaced by: %s (recorded as the author's own statement; not verified merged)\n",
			deskkit.StripControl(r.by))
	}
	fmt.Fprintf(&b, "- Withdrawn by: %s (the draft's own author, pinned by login and numeric id)\n",
		deskkit.StripControl(self.login))
	fmt.Fprintf(&b, "- State reason: %s (carried here; a change records no state reason on either forge)\n", reasonNotPlanned)
	b.WriteString("\nNo ruling artifact is cited — this is the item's own author withdrawing its own proposal, " +
		"the same authority a human already has over their own pull request. Nothing here is " +
		"irreversible: reopen it if the withdrawal is wrong.\n")
	return b.String()
}

// ---------------------------------------------------------------- verify-gate-refire

func cmdVerifyGateRefire(args []string, out io.Writer) error {
	fs := flag.NewFlagSet(modeVerifyGateRefire, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var c common
	c.bind(fs)
	reason := fs.String("reason", "", "why THIS re-fire cycle runs — mandatory free text, echoed into the "+
		"comment so the trail states it without the reader inferring it")
	n, err := parseItemVerb(fs, args, &c)
	if err != nil {
		return err
	}
	// A verify-gate card is an ISSUE. A stated change kind names an object this lane never
	// touches, and is refused before any read rather than resolved to the other sequence.
	if c.kind == deskkit.TargetChange {
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s acts on an ISSUE carrying the %q label, and the item was stated as a change "+
				"(`!N` / --kind mr). A change is not a verify-gate card; state the issue (--kind issue) or "+
				"omit the kind.", modeVerifyGateRefire, deskkit.VerifyGateLabel))
	}
	why := strings.TrimSpace(*reason)
	if why == "" {
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s requires --reason <text> — the audit trail states why this cycle ran; it is not "+
				"inferred from context", modeVerifyGateRefire))
	}
	return applyVerifyGateRefire(c, n, why, out)
}

// resolveVerifier is the ROLE gate: the session's minted App role must be the verifier
// role, and nothing else. Same three-way shape as resolveCaller — could-not-check when the
// role or its roster binding cannot be resolved, refused for any other resolved role
// (worker and reviewer included, by name), accepted only for the verifier — but keyed on
// ONE role, not two. There is no default-accept arm.
func resolveVerifier(repo string) (caller, error) {
	if _, _, ferr := forgeForFn(repo); ferr != nil {
		return caller{}, ferr
	}
	login, ok := deskkit.RoleAppLogin(roleVerifier)
	if !ok {
		return caller{}, deskkit.Unverifiable("could-not-check: "+deskkit.RequireRole(roleVerifier).Error(), nil)
	}
	if mintedRole != roleVerifier {
		return caller{}, deskkit.Refused(fmt.Sprintf(
			"refused: this session's App role is %q, not the %s role (%s). %s is keyed on the verifier "+
				"session identity alone — a worker or reviewer session is refused by name, and no flag "+
				"claims the role. Run it from the verify-desk window.",
			deskkit.StripControl(mintedRole), roleVerifier, deskkit.StripControl(login), modeVerifyGateRefire))
	}
	return caller{login: login, role: roleVerifier}, nil
}

// hasLabel reports whether the item carries the label, case-insensitively — a forge that
// lowercases labels must not turn a present label into an absent one.
func hasLabel(it item, want string) bool {
	for _, have := range it.labelNames() {
		if strings.EqualFold(strings.TrimSpace(have), want) {
			return true
		}
	}
	return false
}

// applyVerifyGateRefire is the lane's own precondition chain. Fixed order:
//
//  1. role gate                 — verifier session, or nothing runs
//  2. read the item             — a read failure is exit 6
//  3. not an issue → refused    — a verify-gate card is an issue
//  4. already OPEN → no-op      — nothing to re-fire; a retried cycle must not fail twice
//  5. decision-label refusal    — UNCHANGED, absolute
//  6. label gate                — verify-gate present, or this is the general reopen tool
//     github.go's package doc says does not exist
//  7. reopen → comment → close  — three charged writes, in that order
//
// gateFor/authorize is NEVER called here either: a verify-gate card is a machine-filed
// process artifact, not another party's item whose retirement needs a human ruling — and
// the close leg cannot complete the human sign-off regardless of what this lane decides,
// because the repository's verify-gate close workflow decides that independently, on the
// sender type GitHub itself reports for the token that made the call.
func applyVerifyGateRefire(c common, n int, reason string, out io.Writer) error {
	a := &auditCtx{verb: modeVerifyGateRefire, repo: c.repo, number: n}

	who, err := resolveVerifier(c.repo)
	if err != nil {
		a.log(resultFor(err), err.Error())
		return err
	}
	it, err := fetchItem(c.repo, n, c.kind)
	if err != nil {
		a.log(deskkit.ResultUnwritten, err.Error())
		return err
	}
	a.title = it.Title
	if it.isPR() {
		err := deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d is a pull request, not an issue — a verify-gate card is an issue, and %s "+
				"reopens nothing else", c.repo, n, modeVerifyGateRefire))
		a.log(deskkit.ResultRefused, err.Error())
		return err
	}
	if !it.closed() {
		a.log(deskkit.ResultNoop, "already open — nothing to re-fire; idempotent no-op")
		fmt.Fprintf(out, "noop\t%s#%d\talready open\n", c.repo, n)
		return nil
	}
	if err := refuseDecisionItem(c.repo, it); err != nil {
		a.log(deskkit.ResultRefused, err.Error())
		return err
	}
	if !hasLabel(it, deskkit.VerifyGateLabel) {
		err := deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d does not carry the %q label. %s is not a general reopen tool; it acts only on "+
				"issues already carrying %s.", c.repo, n, deskkit.VerifyGateLabel, modeVerifyGateRefire,
			deskkit.VerifyGateLabel))
		a.log(deskkit.ResultRefused, err.Error())
		return err
	}

	body := refireComment(c.repo, n, reason, who)
	if c.dryRun {
		a.log(deskkit.ResultDryRun, "dry-run: verified, wrote nothing")
		fmt.Fprintf(out, "dry-run\t%s#%d\t%s\treason=%s\n", c.repo, n, modeVerifyGateRefire, deskkit.StripControl(reason))
		fmt.Fprintln(out, body)
		return nil
	}
	if err := allowWrite(c.repo, n); err != nil {
		a.log(deskkit.ResultRateLimited, err.Error())
		return err
	}
	if err := reopenItem(c.repo, n); err != nil {
		a.log(deskkit.ResultUnverifiable, err.Error())
		return err
	}
	a.log(deskkit.ResultOK, "reopened for the re-fire cycle")
	if err := allowWrite(c.repo, n); err != nil {
		// Reopened, not yet commented or re-closed. Re-running is safe: the item now reads
		// as open, which is the idempotent no-op — the operator re-closes by hand or waits.
		a.log(deskkit.ResultRateLimited, "reopened, comment and re-close deferred: "+err.Error())
		return err
	}
	if err := postComment(c.repo, n, deskkit.TargetIssue, body); err != nil {
		a.log(deskkit.ResultUnverifiable, err.Error())
		return err
	}
	a.log(deskkit.ResultOK, "posted the re-fire comment")
	if err := allowWrite(c.repo, n); err != nil {
		a.log(deskkit.ResultRateLimited, "reopened and commented, re-close deferred: "+err.Error())
		return err
	}
	// No state reason: this cycle states no disposition of its own, only that the marker
	// needed to re-fire (the same carve-out closeItem applies to a change). verifyClosed is
	// FALSE here alone: this close is meant to be undone — verify-gate-close.yml reopens any
	// bot close of a verify-gate card — so requiring the item to READ closed afterwards would
	// fail the lane on the very behaviour it exists to trigger.
	if err := closeItem(c.repo, n, deskkit.TargetIssue, "", false); err != nil {
		a.log(deskkit.ResultUnverifiable, err.Error())
		return err
	}
	a.log(deskkit.ResultOK, fmt.Sprintf("re-closed via lane %s as %s (reason: %s)",
		modeVerifyGateRefire, deskkit.StripControl(who.login), deskkit.StripControl(reason)))
	fmt.Fprintf(out, "refired\t%s#%d\t%s\treason=%s\n", c.repo, n, modeVerifyGateRefire, deskkit.StripControl(reason))
	return nil
}

// refireComment renders the ONE comment the cycle posts, between the reopen and the
// re-close. It is written as both the reopen note and the re-close note — the reason is
// stated under each — so a reader of the thread sees why the card toggled without inferring
// it, and sees explicitly what this cycle is NOT.
func refireComment(repo string, n int, reason string, who caller) string {
	var b strings.Builder
	fmt.Fprintf(&b, "**%s** cycle on %s#%d, by %s.\n\n", modeVerifyGateRefire, repo, n, deskkit.StripControl(who.login))
	fmt.Fprintf(&b, "- Reopened — reason: %s\n", deskkit.StripControl(reason))
	fmt.Fprintf(&b, "- Re-closed — reason: %s\n", deskkit.StripControl(reason))
	b.WriteString("\nThis reopen+close is a verify-gate re-fire cycle, not the human sign-off — only an " +
		"allowlisted human's own close advances this brief to done; a bot's close of a verify-gate issue " +
		"is unconditionally reopened by verify-gate-close.yml.\n")
	return b.String()
}
