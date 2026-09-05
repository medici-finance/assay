package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const toolName = "deskflip"

// Condition names — the verb's contract. A refusal names exactly one of these, and a
// caller (a loop, a runbook, a human reading the line) keys on it. A name that drifts
// breaks every consumer, so these are constants.
const (
	condCallerRole       = "caller-role"
	condAppToken         = "app-token"
	condPROpenDraft      = "pr-open-draft"
	condModelFloor       = "model-floor"
	condReviewerApproved = "reviewer-approved"
	condChecksGreen      = "checks-green"
	condMergeable        = "mergeable"
	condSecurityVerdict  = "security-verdict"
	condHeadStable       = "head-stable"
)

// flipConditions is the ordered condition list, pinned by a test so a condition cannot be
// silently dropped or reordered. The order is deliberate: the cheap, no-network refusals
// come first, and the head re-read comes LAST because its whole purpose is to be the final
// thing checked before the mutation.
var flipConditions = []string{
	condCallerRole,
	condAppToken,
	condPROpenDraft,
	condModelFloor,
	condReviewerApproved,
	condChecksGreen,
	condMergeable,
	condSecurityVerdict,
	condHeadStable,
}

// flipRole is the loop identity that OWNS the ready-flip: the role that watched the
// review. The flip stays with the desk that watched the review — a flip issued by a
// session that did not watch it is a flip whose preconditions nobody was tracking as they
// changed.
const flipRole = "pr-review-desk"

// reviewerRole is the roster role whose App verdict governs the correctness gate. The
// login itself is NEVER a literal here: it is read from the roster binding, so a
// deployment's own App identity stays in configuration and out of published source.
const reviewerRole = "reviewer"

// The two queue-legibility labels. A PR carries exactly one of them, and the flip SWAPS
// them: before the flip the PR is waiting on the review lane, after it the PR is waiting
// on the human's merge. Keeping them mutually exclusive is what makes the queue readable
// at a glance — two labels, or none, and a human has to open the PR to find out who it is
// waiting on.
const (
	labelBeforeFlip = "authorization-needed"
	labelAfterFlip  = "approval-needed"
)

type flipOpts struct {
	pr     int
	repo   string
	root   string
	quiet  bool
	dryRun bool
}

func cmdFlip(args []string) error {
	fs := flag.NewFlagSet("deskflip", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	repo := fs.String("repo", "", "owner/name the PR belongs to (default: derived from --root's origin)")
	root := fs.String("root", ".", "checkout whose origin names the repo, when --repo is omitted")
	quiet := fs.Bool("quiet", false, "suppress the per-condition OK lines")
	dryRun := fs.Bool("dry-run", false, "check every condition and stop before the mutation")

	if len(args) == 0 {
		return deskkit.Refused("deskflip requires a PR number")
	}
	n, err := strconv.Atoi(args[0])
	if err != nil || n <= 0 {
		return deskkit.Refused("deskflip: first argument must be a positive PR number, got " + args[0])
	}
	if perr := fs.Parse(args[1:]); perr != nil {
		return deskkit.Refused("deskflip: bad flags: " + perr.Error())
	}
	if fs.NArg() != 0 {
		return deskkit.Refused("deskflip: unexpected extra arguments after <N>: " + strings.Join(fs.Args(), " "))
	}

	o := flipOpts{pr: n, repo: *repo, root: *root, quiet: *quiet, dryRun: *dryRun}
	ferr := flip(o)
	audit(o, ferr)
	return ferr
}

func flip(o flipOpts) error {
	// --- caller-role -------------------------------------------------------------
	if err := checkCallerRole(); err != nil {
		return err
	}
	o.say("%s OK: caller presents %s", condCallerRole, flipRole)

	repo, err := o.resolveRepo()
	if err != nil {
		return err
	}
	if !deskkit.IsAllowedRepo(repo) {
		return deskkit.Refused(fmt.Sprintf(
			"condition %s: %s is not in the desk repo set — this verb flips only PRs in repos the desk is "+
				"rostered to act on.", condPROpenDraft, repo))
	}
	// The reviewer role must be BOUND before any verdict is read. Comparing an author
	// against an empty identity is how a review nobody wrote matches the reviewer App.
	if rerr := deskkit.RequireRole(reviewerRole); rerr != nil {
		return deskkit.Unverifiable(fmt.Sprintf(
			"condition %s: %v", condReviewerApproved, rerr), nil)
	}
	reviewerLogin, _ := deskkit.RoleAppLogin(reviewerRole)
	fr, ferr := forgeRepoOf(repo)
	if ferr != nil {
		return ferr
	}

	// --- app-token ---------------------------------------------------------------
	// Before the FIRST forge call, so no read and no write can happen on the ambient
	// credential even if a later condition refuses. This step is also where the FORGE is
	// resolved: which forge serves this repo is the resolver's answer (roster binding, else
	// an unambiguous origin host, else a refusal), never this verb's assumption.
	fg, res, err := checkAppToken(o, fr)
	if err != nil {
		return err
	}

	// --- pr-open-draft -----------------------------------------------------------
	pr, err := readPR(o, fg, fr)
	if err != nil {
		return err
	}
	if !strings.EqualFold(pr.State, "OPEN") {
		return deskkit.Refused(fmt.Sprintf(
			"condition %s: PR #%d in %s is %s, not open — there is nothing to flip.",
			condPROpenDraft, o.pr, repo, strings.ToLower(pr.State)))
	}
	// relabelOnly is the already-ready path: the flip itself is done, but the queue labels
	// may still need reconciling.
	//
	// WHY THE LABEL WRITE IS GATED TOO (the fix, and the reasoning behind choosing it).
	// Writing `approval-needed` is not bookkeeping — it is an ASSERTION to every human
	// reading the queue that the review lane is finished with this PR and only a merge is
	// outstanding. On a PR that is no longer a draft that assertion can be false: a human
	// may have flipped it by hand, or it may have been pushed to since the flip and now
	// carry a standing CHANGES_REQUESTED at its new head. Relabelling without re-gating
	// would then tell the queue the PR is waiting on the human when it is really waiting on
	// the reviewer — and a queue that misreports who is blocked is worse than one that says
	// nothing, because nobody re-checks a PR that claims to be done.
	//
	// So the two cases are separated by whether a WRITE is actually required:
	//
	//   labels already correct  → pure no-op, exit 0, nothing read further, nothing written.
	//                             This is the common re-run case, and it must stay cheap and
	//                             non-failing: a loop re-running its Land step over a landed
	//                             item must not report a failure.
	//   labels need changing    → a write, so it runs the SAME gate as the flip path and
	//                             skips only the ready mutation. On a failed condition it
	//                             refuses by name and leaves the labels untouched.
	relabelOnly := false
	if !pr.IsDraft {
		if hasLabel(pr.Labels, labelAfterFlip) && !hasLabel(pr.Labels, labelBeforeFlip) {
			o.say("%s: PR #%d is already ready-for-human and its queue label is correct — nothing to do",
				condPROpenDraft, o.pr)
			return nil
		}
		relabelOnly = true
		o.say("%s: PR #%d is already ready-for-human but its queue label is stale; re-gating before the "+
			"label write", condPROpenDraft, o.pr)
	}
	head := strings.TrimSpace(pr.HeadRefOid)
	if head == "" {
		return deskkit.Unverifiable(fmt.Sprintf(
			"condition %s: PR #%d reports no head commit, so every at-head comparison below would go "+
				"vacuous at once — exactly when there is least reason to believe any of them.",
			condPROpenDraft, o.pr), nil)
	}
	if relabelOnly {
		o.say("%s OK: open, already flipped, at %s", condPROpenDraft, short(head))
	} else {
		o.say("%s OK: open + draft at %s", condPROpenDraft, short(head))
	}

	// --- model-floor -------------------------------------------------------------
	// The authority-bearing-write floor: a ready-flip requires a strong-tier dispatch. It
	// is read from the target PR's dispatcher-attested tier stamp (the applier-aware reader,
	// so a self-applied stamp is worthless), and it fails CLOSED — an attested below-tier
	// dispatch, or a stamp present-but-unreadable, refuses. An UNATTESTED PR (human-driven
	// or pre-attestation) is not bricked: it proceeds with a NOTICE. The override is loud.
	if err := checkModelFloor(o, fg, fr); err != nil {
		return err
	}

	// --- reviewer-approved -------------------------------------------------------
	reviews, err := readReviews(o, fg, fr)
	if err != nil {
		return err
	}
	if err := checkReviewerApproved(reviewerLogin, reviews, head, o.pr); err != nil {
		return err
	}
	o.say("%s OK: %s APPROVED at %s", condReviewerApproved, reviewerLogin, short(head))

	// --- checks-green ------------------------------------------------------------
	// Reduce to the LATEST run per check NAME first — branch protection's own rule — so a
	// superseded run (an older CANCELLED, a stale QUEUED left by a push+pull_request
	// double-trigger) does not count against a PR whose current run for that name is green.
	// The reduction changes only WHICH run is judged, never HOW: the reduced set flows
	// through the same evaluation, so a name whose LATEST run is red/pending still blocks.
	rollup, err := readChecks(o, fg, fr, head)
	if err != nil {
		return err
	}
	checks := latestPerRollupName(rollup)
	switch evalRollup(checks) {
	case ciFail:
		return deskkit.Refused(fmt.Sprintf(
			"condition %s: a check at %s failed — %s. A red rollup outranks any local verification: CI runs "+
				"the real toolchain and a local trace does not.",
			condChecksGreen, short(head), strings.Join(failedChecks(checks), ", ")))
	case ciPending:
		return deskkit.Unverifiable(fmt.Sprintf(
			"condition %s: checks at %s are not positively green (a check has not completed). Not-yet-green "+
				"is could-not-verify, never green.", condChecksGreen, short(head)), nil)
	case ciEmpty:
		if deskkit.CIRequired(repo) {
			return deskkit.Unverifiable(fmt.Sprintf(
				"condition %s: no check rollup at %s on CI-required repo %s — an absent rollup on a repo that "+
					"runs CI means the checks have not reported yet, which cannot be read as green.",
				condChecksGreen, short(head), repo), nil)
		}
		// A repo with no PR CI at all: an empty rollup is everything there will ever be.
	case ciGreen:
	}
	o.say("%s OK: %d check(s) green at %s", condChecksGreen, len(checks), short(head))

	// --- mergeable ---------------------------------------------------------------
	switch strings.ToUpper(strings.TrimSpace(pr.Mergeable)) {
	case "MERGEABLE":
	case "CONFLICTING":
		return deskkit.Refused(fmt.Sprintf(
			"condition %s: PR #%d is CONFLICTING — a conflicting PR is not flippable, and its resolution "+
				"touches the PR's own files, which is authored work that invalidates the approval and "+
				"requires a re-review.", condMergeable, o.pr))
	default:
		return deskkit.Unverifiable(fmt.Sprintf(
			"condition %s: PR #%d reports mergeable=%q — the forge has not computed it yet. Unknown is not "+
				"mergeable; re-run once it settles.", condMergeable, o.pr, pr.Mergeable), nil)
	}
	o.say("%s OK", condMergeable)

	// --- security-verdict --------------------------------------------------------
	// The changed-file list is read HERE, in full, and only here: it is the security
	// lane's input and nothing above needs it. Both calls to checkSecurityVerdict — this
	// one and the post-TOCTOU re-check below — are handed the SAME walked list, matching
	// how they share `pr`. The head is re-read between them, and a moved head refuses
	// before the second call, so a stale list can never be what a flip is decided on.
	files, err := readChangedFiles(o, fg, fr)
	if err != nil {
		return err
	}
	if err := checkSecurityVerdict(o, repo, pr, files, reviews, reviewerLogin, head); err != nil {
		return err
	}
	o.say("%s OK: %d changed file(s) read", condSecurityVerdict, len(files))

	// --- head-stable (TOCTOU) ----------------------------------------------------
	// The ready mutation has no compare-and-swap, so the head is re-read HERE, after every
	// condition above and immediately before the mutation. A head that moved means each
	// verdict above was read against code that is no longer what would flip.
	head2, err := readHead(o, fg, fr)
	if err != nil {
		return err
	}
	if head2 != head {
		return deskkit.Refused(fmt.Sprintf(
			"condition %s: the head moved during the checks (%s -> %s) — every condition above was verified "+
				"against code that is no longer current. No flip; re-run against the new head.",
			condHeadStable, short(head), short(head2)))
	}
	// A STABLE HEAD IS NOT A STABLE VERDICT. Re-reading only the head catches a push, and a
	// push is not the only thing that can invalidate the decision: a reviewer can post a
	// `Security-Review: fail` — a deliberate retraction — at the SAME head, between the
	// reviews read above and the mutation below. The head does not move for that, so a
	// head-only re-read reports "still current" and the flip proceeds over a live
	// retraction. The verdicts are therefore re-read too, and both gates re-run against the
	// fresh list.
	//
	// The window is small, but it is exactly the window a reviewer uses: they are looking at
	// the PR at the moment the desk is deciding about it. Cost is one extra read; the thing
	// it prevents is flipping a PR whose security verdict was withdrawn seconds earlier.
	reviews2, err := readReviews(o, fg, fr)
	if err != nil {
		return err
	}
	if err := checkReviewerApproved(reviewerLogin, reviews2, head, o.pr); err != nil {
		return err
	}
	if err := checkSecurityVerdict(o, repo, pr, files, reviews2, reviewerLogin, head); err != nil {
		return err
	}
	o.say("%s OK: still %s, and the verdicts at that head are unchanged", condHeadStable, short(head))

	if o.dryRun {
		fmt.Printf("deskflip: DRY RUN — every condition holds for %s#%d at %s (%d/%d); stopped before the "+
			"mutation.\n", repo, o.pr, short(head), len(flipConditions), len(flipConditions))
		return nil
	}

	// relabelOnly: the PR is already out of draft, so there is no ready mutation to make —
	// only the queue label to reconcile, and the gate above is what earns the right to
	// write it.
	if relabelOnly {
		if err := ensureLabelSwap(o, fg, fr, pr); err != nil {
			return err
		}
		fmt.Printf("deskflip: RELABELLED %s#%d — already ready-for-human at %s, queue label reconciled "+
			"after a full re-gate.\n", repo, o.pr, short(head))
		return nil
	}

	// The mutation. The opaque change id is the one the PR READ returned — deskkit.ReadyFlip
	// takes the change rather than an id string precisely so this call site cannot compose
	// one, and refuses could-not-check (naming forge and operation, writing nothing) when the
	// resolved backend serves none.
	if err := deskkit.ReadyFlip(res, fg, pr.change); err != nil {
		msg := fmt.Sprintf("the ready mutation on %s#%d failed (%s) — the PR is still a draft.",
			repo, o.pr, firstLine(err.Error()))
		// The backend's own CLASS is preserved rather than flattened: a refusal (a change id
		// the backend will not act on) and a could-not-check (a transport or permission
		// failure) call for different operator responses, and collapsing both to one exit
		// code is how a settled state gets retried forever.
		if deskkit.ExitCodeOf(err) == deskkit.ExitRefused {
			return deskkit.Refused(msg)
		}
		return deskkit.Unverifiable(msg, err)
	}
	if err := ensureLabelSwap(o, fg, fr, pr); err != nil {
		return err
	}
	fmt.Printf("deskflip: FLIPPED %s#%d ready-for-human at %s — the merge is the human's.\n",
		repo, o.pr, short(head))
	return nil
}

// checkCallerRole enforces the ownership ruling: the ready-flip belongs to the role that
// watched the review.
//
// An UNRECOGNISED loop name is could-not-check, not "some other role": the same
// distinction the kill switch makes, for the same reason — a name nothing recognises tells
// you nothing, and reading it as an answer is how a silent failure gets reported as clean.
func checkCallerRole() error {
	raw := strings.TrimSpace(os.Getenv("DESK_LOOP"))
	if raw == "" {
		return deskkit.Refused(fmt.Sprintf(
			"condition %s: $DESK_LOOP is unset. The ready-flip belongs to the %s role — the desk that "+
				"WATCHED the review — so a session that does not present that identity does not flip. "+
				"Run `export DESK_LOOP=%s` in the review window, or let that window make the flip.",
			condCallerRole, flipRole, flipRole))
	}
	names, known := deskkit.LoopFlagNames(raw)
	if !known {
		return deskkit.Unverifiable(fmt.Sprintf(
			"condition %s: $DESK_LOOP=%q is not a loop name the kill switch recognises, so which role is "+
				"calling CANNOT be established — that is could-not-check, never 'the right one'.",
			condCallerRole, raw), nil)
	}
	if names[0] != flipRole {
		return deskkit.Refused(fmt.Sprintf(
			"condition %s: this session presents loop %q, but the ready-flip belongs to %s — the desk that "+
				"watched the review. Hand the flip to that window rather than issuing it from here.",
			condCallerRole, names[0], flipRole))
	}
	return nil
}

// checkAppToken resolves the App installation token this verb's writes are made under, and
// REFUSES rather than proceeding on the ambient credential when it cannot.
//
// WHICH IDENTITY, AND WHY IT IS DERIVED. The flip belongs to the loop named by flipRole, so
// the App it acts as is that loop's App role — read from the one shared loop-to-role table
// rather than spelled again here, so the window a session presents and the identity its
// writes carry cannot drift apart.
//
// EXIT 5, NOT 6. A missing App credential is not "the tool could not tell": it is a
// completely determined state — the credential is not there — and the correct response is
// to stop, not to write under someone else's name. The refusal names the role and the token
// path so the operator can fix the credential rather than guess at it. The token VALUE is
// never printed.
func checkAppToken(o flipOpts, fr deskkit.ForgeRepo) (deskkit.Forge, deskkit.ForgeResolution, error) {
	role, ok := deskkit.TokenRoleForLoop(flipRole)
	if !ok {
		return nil, deskkit.ForgeResolution{}, deskkit.Unverifiable(fmt.Sprintf(
			"condition %s: loop %s has no App role, so which identity this flip would be written under "+
				"cannot be established.", condAppToken, flipRole), nil)
	}
	// The lookup runs HERE, before the resolver, so the refusal can name the role and the
	// token PATH — which is what an operator needs and what a generic custody failure from
	// inside the resolver would not carry.
	_, path, err := mintTokenFn(role, fr.Slug())
	if err != nil {
		return nil, deskkit.ForgeResolution{}, deskkit.Refused(fmt.Sprintf(
			"condition %s: the %s App installation token for %s could not be minted or read (%s): %v. "+
				"deskflip does NOT fall back to an ambient forge-CLI credential — a ready-flip and its queue "+
				"labels written under an operator's own login read as a human decision and cannot be "+
				"taken back. Restore the credential and re-run.",
			condAppToken, role, fr.Owner, tokenPathForMessage(path), err))
	}
	// WHICH FORGE, and under WHOSE custody, is one question answered in one place. The
	// resolver reads the repo's configured forge (else an unambiguous origin host) and hands
	// the backend the role's already-minted token; there is no forge flag, no host default,
	// and no ambient-credential fallback anywhere below this line.
	fg, res, rerr := deskkit.ResolveForge(fr, role)
	if rerr != nil {
		return nil, deskkit.ForgeResolution{}, rerr
	}
	o.say("%s OK: writes authenticate as the %s App on %s (%s; token file %s)",
		condAppToken, role, res.Kind, res.Source, path)
	return fg, res, nil
}

// forgeRepoOf splits an owner/name slug into the coordinate the forge seam addresses repos
// by. A slug that does not split is could-not-check, never half-parsed.
func forgeRepoOf(repo string) (deskkit.ForgeRepo, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" {
		return deskkit.ForgeRepo{}, deskkit.Unverifiable(fmt.Sprintf(
			"condition %s: %q does not parse to owner/name", condPROpenDraft, repo), nil)
	}
	return deskkit.ForgeRepo{Owner: owner, Name: name}, nil
}

// tokenPathForMessage renders the token file path for a refusal. The PATH is what an
// operator needs and is safe to print; the token VALUE never is, and never reaches a
// message from anywhere in this verb.
func tokenPathForMessage(path string) string {
	if strings.TrimSpace(path) == "" {
		return "the minter named no token path"
	}
	return "token path " + path
}

// checkModelFloor enforces the model-capability floor on the ready-flip: an authority-
// bearing write requires a strong-tier dispatch. It reads the target PR's dispatcher-
// attested tier stamp from the label timeline and delegates the DECISION to the shared
// deskkit floor, so deskflip and the verdict verb apply one wording of the rule and one
// remediation.
//
// FAIL-CLOSED. A below-tier or present-but-unreadable attestation REFUSES (exit 5). A
// timeline that cannot be READ is could-not-check, never a pass — it refuses UNVERIFIABLE
// (exit 6). Only a genuinely UNATTESTED PR proceeds, and it says so (NOTICE), so a
// human-driven or pre-attestation lane is not bricked. The override line is always printed,
// regardless of --quiet, because a silent bypass would nullify the layer.
func checkModelFloor(o flipOpts, fg deskkit.Forge, fr deskkit.ForgeRepo) error {
	events, err := readLabelEvents(o, fg, fr)
	if err != nil {
		return err
	}
	d := deskkit.ModelCapabilityFloor(events, deskkit.IsDispatcherLogin, deskkit.ModelFloorOverrideEngaged())
	switch d.Outcome {
	case deskkit.FloorOverrideAllow:
		fmt.Fprintf(os.Stderr, "deskflip: %s\n", d.Message)
		return nil
	case deskkit.FloorNoticeAllow:
		o.say("%s: %s", condModelFloor, d.Message)
		return nil
	case deskkit.FloorAllow:
		o.say("%s OK: %s", condModelFloor, d.Message)
		return nil
	default: // FloorRefuse
		return deskkit.Refused(fmt.Sprintf("condition %s: %s", condModelFloor, d.Message))
	}
}

// readLabelEvents reads the target PR's `labeled` timeline events — the label name AND the
// login that applied it — which is what the applier-aware stamp reader needs to tell a
// dispatcher attestation from a self-applied one. It walks every page.
//
// An EMPTY timeline is not an error: it is a PR with no labels, which the floor reads as
// UNATTESTED (a NOTICE, not a refusal). A failed READ is could-not-check and refuses
// unverifiable at the caller.
func readLabelEvents(o flipOpts, fg deskkit.Forge, fr deskkit.ForgeRepo) ([]deskkit.LabelEvent, error) {
	events, err := fg.ListLabelEvents(fr, o.pr)
	if err != nil {
		return nil, deskkit.Unverifiable(fmt.Sprintf(
			"condition %s: cannot read PR #%d's label timeline (%s) — the dispatch tier could not be "+
				"established, and could-not-check is never a cleared floor.",
			condModelFloor, o.pr, firstLine(err.Error())), err)
	}
	return events, nil
}

// checkReviewerApproved is the correctness gate: the reviewer App's latest correctness
// verdict must be APPROVED, AT THE CURRENT HEAD.
//
// THE REDUCTION IS DELIBERATELY NARROWER-GRANTING THAN THE WRITE PATH'S, and never wider.
// Three rules, each in the fail-closed direction:
//
//  1. A body carrying a security marker is EXCLUDED from the correctness lane. The two
//     verdicts are separate artifacts; a security pass must never satisfy the correctness
//     gate, which is the one direction of that confusion that fails OPEN.
//  2. ANY App CHANGES_REQUESTED at the current head blocks, whatever came after it. An
//     APPROVED at an unchanged head cannot be a re-verification — there is nothing new to
//     verify — and the forge's self-approval block only keys on the PR AUTHOR account, so
//     it has nothing to say about a third-party App re-posting a verdict at an unchanged
//     head. Only a NEW commit can clear a standing block.
//  3. STALE and NONE are reported as DIFFERENT refusals. "There is a verdict, just not at
//     this code" and "nobody has reviewed this" call for different actions, and collapsing
//     them sends the operator after the wrong one.
func checkReviewerApproved(reviewerLogin string, reviews []reviewInfo, head string, pr int) error {
	// Rule 2 first: a standing block at head is decisive whatever else is present.
	for _, r := range reviews {
		if !deskkit.SameActor(r.User.Login, reviewerLogin) || r.CommitID != head {
			continue
		}
		if r.State == "CHANGES_REQUESTED" {
			return deskkit.Refused(fmt.Sprintf(
				"condition %s: a CHANGES_REQUESTED from %s stands at the current head %s. An APPROVED at an "+
					"unchanged head cannot be a re-verification — there is nothing new to verify. Only a new "+
					"commit clears this, or a human clearing the standing rejection directly on the PR.",
				condReviewerApproved, reviewerLogin, short(head)))
		}
	}

	// Rule 1: build the CORRECTNESS lane only.
	lane := make([]deskkit.AppReview, 0, len(reviews))
	for _, r := range reviews {
		if hasSecurityMarker(r.Body) {
			continue
		}
		lane = append(lane, deskkit.AppReview{
			AuthorLogin: r.User.Login,
			State:       r.State,
			CommitID:    r.CommitID,
			SubmittedAt: r.SubmittedAt,
		})
	}
	switch deskkit.ReduceAppVerdict(reviewerLogin, lane, head) {
	case deskkit.AppVerdictApproved:
		return nil
	case deskkit.AppVerdictChangesRequested:
		return deskkit.Refused(fmt.Sprintf(
			"condition %s: the latest correctness verdict from %s at %s is CHANGES_REQUESTED — blocked.",
			condReviewerApproved, reviewerLogin, short(head)))
	case deskkit.AppVerdictStale:
		return deskkit.Refused(fmt.Sprintf(
			"condition %s: %s has a correctness verdict on PR #%d, but it was submitted at a DIFFERENT head — "+
				"the PR advanced past it. A verdict that is not at this code is stale; re-review at %s.",
			condReviewerApproved, reviewerLogin, pr, short(head)))
	default:
		return deskkit.Refused(fmt.Sprintf(
			"condition %s: %s has posted no APPROVED/CHANGES_REQUESTED correctness verdict on PR #%d. A "+
				"security verdict alone does not satisfy the correctness gate.",
			condReviewerApproved, reviewerLogin, pr))
	}
}

// checkSecurityVerdict runs the security lane.
//
// TWO RULES, and the first is unconditional. An explicit `Security-Review: fail` at head
// is a reviewer's deliberate RETRACTION, and whether the diff happens to trigger automatic
// risk-classification has nothing to do with whether that retraction was made — so it
// blocks every flip, risk-classed or not. The second rule is the risk-classed requirement:
// a PR that IS risk-classed needs an affirmative pass at the same head, and absence is
// never a pass.
//
// A repo whose visibility risk-classes it (a public repo always does) is risk-classed
// unconditionally — no diff reading required, and no way for a quiet path to opt out.
//
// The changed-file list arrives as an EXPLICIT PARAMETER rather than as a field of
// prInfo, and that is the shape of the fix for the truncating read. `gh pr view --json
// files` answers from a single unpaginated page, so on a PR with more changed files than
// that page holds it returns a prefix of the diff while reporting the true `changedFiles`
// total alongside it. Whichever way that prefix was then used, the outcome was wrong: the
// reconcile below refused every larger PR as unverifiable, and had the reconcile not been
// there, a risky path sitting past the page boundary would have been invisible. Carrying
// the list on prInfo is what made the truncated read reachable at all, so the type no
// longer has the field: the only value that can reach this gate is the one the caller
// walked to completion (readChangedFiles), and the compiler is what enforces it.
func checkSecurityVerdict(o flipOpts, repo string, pr prInfo, files []fileInfo, reviews []reviewInfo, reviewerLogin, head string) error {
	verdict := securityVerdictAtHead(reviews, reviewerLogin, head)
	if verdict == secFail {
		return deskkit.Refused(fmt.Sprintf(
			"condition %s: an App review at head %s carries `Security-Review: fail` — the security verdict is "+
				"RETRACTED. This blocks the flip whether or not the PR is risk-classed; clear it with a later "+
				"`Security-Review: pass` at this head.", condSecurityVerdict, short(head)))
	}

	riskClassed := deskkit.VisibilityRiskClassed(repo)
	reason := "repo visibility"
	if !riskClassed {
		// Reconcile the files walk against the forge's OWN count. A walk that stopped
		// early otherwise believes it saw the whole diff — pad the PR with enough files
		// ahead of the risky one and the gate waives itself. A short read is UNVERIFIABLE,
		// not clean. This branch STAYS after the paginated read lands: a forge that
		// asserts more files than it will serve is still a diff nobody read in full.
		if pr.ChangedFiles > 0 && len(files) < pr.ChangedFiles {
			return deskkit.Unverifiable(fmt.Sprintf(
				"condition %s: read %d changed files but the forge reports %d for PR #%d — the diff could not "+
					"be read in full, so the risk-class determination is unverifiable.",
				condSecurityVerdict, len(files), pr.ChangedFiles, o.pr), nil)
		}
		paths := make([]string, 0, len(files))
		for _, f := range files {
			paths = append(paths, f.Path)
		}
		if deskkit.RiskPathTriggered(repo, paths) {
			riskClassed = true
			reason = deskkit.RiskClassReason(repo, paths)
		}
	}
	if !riskClassed {
		return nil
	}
	if verdict != secPass {
		return deskkit.Refused(fmt.Sprintf(
			"condition %s: this is a RISK-CLASSED PR (%s) and no App review at head %s carries a "+
				"`Security-Review: pass` line. Absence is never a pass — run the security review at this head "+
				"before the flip.", condSecurityVerdict, reason, short(head)))
	}
	return nil
}

// secVerdict is the security lane's reduction.
type secVerdict int

const (
	secNone secVerdict = iota
	secPass
	secFail
)

// hasSecurityMarker reports whether a body carries EITHER security marker, using the
// canonical readers. It is deliberately not a third parser: a re-implemented marker read
// is how two tools came to disagree about whether a retraction existed.
func hasSecurityMarker(body string) bool {
	return deskkit.HasSecurityReviewPass(body) || deskkit.HasSecurityReviewFail(body)
}

// securityVerdictAtHead reduces every reviewer-App security verdict AT HEAD to one
// governing verdict. Reviews arrive in ascending submitted order, so the reduction is
// ORDER-SENSITIVE: the last verdict at head governs, and a `pass` later retracted by a
// `fail` at the same head is NOT green. Returning the verdict rather than a bool is what
// keeps "nobody spoke" and "the verdict is fail" distinguishable — collapsing both to
// false is what made an explicit retraction indistinguishable from silence.
func securityVerdictAtHead(reviews []reviewInfo, reviewerLogin, head string) secVerdict {
	out := secNone
	for _, r := range reviews {
		if !deskkit.SameActor(r.User.Login, reviewerLogin) || r.CommitID != head {
			continue
		}
		switch {
		case deskkit.HasSecurityReviewFail(r.Body):
			out = secFail
		case deskkit.HasSecurityReviewPass(r.Body):
			// A verdict GRANTS only by binding to the commit it was issued against, and
			// the empty string is not a commit. A FAIL is not subject to the same test:
			// every reason to doubt a retraction is a reason to keep blocking.
			if head != "" {
				out = secPass
			}
		}
	}
	return out
}

// ensureLabelSwap makes the queue-legibility labels say who the PR is waiting on now.
//
// It is IDEMPOTENT and never fails the flip. The flip itself is the state change that
// matters; a label the forge would not accept (most often because the repo has never been
// provisioned with it) is a provisioning gap to report, not a reason to leave a converged
// PR sitting as a draft. So a label failure is a loud warning and exit 0, and the
// mutation's own failure — above — is what fails.
func ensureLabelSwap(o flipOpts, fg deskkit.Forge, fr deskkit.ForgeRepo, pr prInfo) error {
	if o.dryRun {
		return nil
	}
	if hasLabel(pr.Labels, labelAfterFlip) && !hasLabel(pr.Labels, labelBeforeFlip) {
		return nil // already swapped
	}
	change := deskkit.LabelChange{
		Add: []deskkit.LabelSpec{{
			Name:        labelAfterFlip,
			Color:       queueLabelColor,
			Description: "Queue: the review lane is finished; this change is waiting on a human merge",
		}},
	}
	if hasLabel(pr.Labels, labelBeforeFlip) {
		change.Remove = []string{labelBeforeFlip}
	}
	if _, err := fg.ApplyLabels(fr, o.pr, change); err != nil {
		fmt.Fprintf(os.Stderr, "deskflip: WARNING: could not swap the queue labels on %s#%d (%s) — the flip "+
			"stands; the queue label is a provisioning gap to file.\n", fr.Slug(), o.pr, firstLine(err.Error()))
	}
	return nil
}

// queueLabelColor is the cosmetic colour the queue label is CREATED with if the repo does
// not already carry it. Bare hex digits: each backend renders its own form (GitHub forbids a
// leading `#`, GitLab requires one), so this value stays forge-agnostic.
const queueLabelColor = "0e8a16"

func hasLabel(labels []labelInfo, want string) bool {
	for _, l := range labels {
		if strings.EqualFold(l.Name, want) {
			return true
		}
	}
	return false
}

func (o flipOpts) say(format string, args ...any) {
	if o.quiet {
		return
	}
	fmt.Fprintf(os.Stderr, "deskflip: "+format+"\n", args...)
}

func short(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

// resolveRepo returns the owner/name: --repo, else --root's origin.
func (o flipOpts) resolveRepo() (string, error) {
	if strings.TrimSpace(o.repo) != "" {
		return strings.TrimSpace(o.repo), nil
	}
	r := runCmd(o.root, "git", "remote", "get-url", "origin")
	if r.err != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"cannot read origin's URL in %s (%s) — pass --repo <owner/name>.", o.root, firstLine(r.stderr)), r.err)
	}
	slug := repoSlugFromURL(r.stdout)
	if slug == "" {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"origin %q does not parse to an owner/name — pass --repo <owner/name>.", r.stdout), nil)
	}
	return slug, nil
}

// --- the gate's view of what the forge returned ----------------------------------

// prInfo is the gate's view of the change: the fields every condition below reads, plus the
// change the forge actually returned.
//
// It carries ChangedFiles — the forge's own total — but deliberately NOT the file list. A
// single unpaginated read returns a PREFIX on any PR larger than one page while ChangedFiles
// beside it stays true. A field holding that prefix is a loaded gun pointed at the
// risk-class determination, and having no such field is what makes a truncated value
// unreachable rather than merely unused. The complete list comes from readChangedFiles and is
// passed explicitly.
//
// `change` is the deskkit.PullRequest the forge returned, kept whole so the ready mutation
// can be handed the change rather than an id: the opaque id is the BACKEND's encoding, and a
// call site that held only a string could compose one. Nothing else reads it.
type prInfo struct {
	Number       int
	State        string
	IsDraft      bool
	Mergeable    string
	HeadRefOid   string
	ChangedFiles int
	Labels       []labelInfo

	change *deskkit.PullRequest
}

// fileInfo is one entry of the reconciled changed-file list.
type fileInfo struct {
	Path string
}

type labelInfo struct {
	Name string
}

// rollupEntry is one status-check rollup element. The forge renders two different shapes
// here — a check run (status + conclusion) and a status context (state) — and a reader
// that understands only one silently sees an empty rollup for the other, which on a
// CI-less policy reads as green. Both shapes are carried.
//
// The three timestamp fields are the recency signal the latest-run-per-name reduction
// needs. A CheckRun carries StartedAt/CompletedAt; a StatusContext carries only CreatedAt.
// The forge serves all of them in `statusCheckRollup` already — the fields were simply not
// decoded before — so reducing by them costs no extra read.
type rollupEntry struct {
	Name        string
	Context     string
	Status      string
	Conclusion  string
	State       string
	StartedAt   string
	CompletedAt string
	CreatedAt   string
}

func (e rollupEntry) label() string {
	if e.Name != "" {
		return e.Name
	}
	if e.Context != "" {
		return e.Context
	}
	return "(unnamed check)"
}

// groupKey is the identity a check is reduced BY: its NAME for a check run, its CONTEXT for
// a status context. Branch protection keys "latest run per context" on exactly this name,
// which is why grouping by it reproduces the platform's own rule. An entry with NEITHER a
// name nor a context has no identity to group on, so it returns "" and the reducer keeps it
// standalone — collapsing every nameless entry into one bucket would be a reduction the
// forge never made.
func (e rollupEntry) groupKey() string {
	if e.Name != "" {
		return e.Name
	}
	return e.Context
}

// recencyKey returns the entry's best available recency stamp for the latest-run-per-name
// reduction. The forge renders RFC3339 timestamps, which sort lexicographically in
// chronological order, so a plain string compare orders two runs of one name by age.
//
// CompletedAt is preferred — a finished run's true end — then StartedAt (a run that began
// but has not finished: an in-progress run, or the moment a queued run was picked up), then
// CreatedAt (a status context's only stamp). An entry that carries none — a freshly QUEUED
// check run the forge has not stamped yet, the #282-class orphan left by a double-trigger —
// returns "" and sorts OLDEST, so it loses to any run that actually ran. That is the
// fail-SAFE direction: a stampless queued run does not get to supersede a completed one, and
// a run whose LATEST is genuinely pending or failing is never reduced away by this rule.
func (e rollupEntry) recencyKey() string {
	switch {
	case e.CompletedAt != "":
		return e.CompletedAt
	case e.StartedAt != "":
		return e.StartedAt
	default:
		return e.CreatedAt
	}
}

// latestPerRollupName reduces the rollup to one entry per check NAME — the LATEST run for
// each — before the green/pending/fail evaluation runs over it. It delegates to the SHARED
// deskkit.LatestRunPerName so this ready-flip gate and deskboard's CI render reduce by
// identical semantics and cannot drift: a flip that fires on a push+pull_request
// double-triggered PR while the board still renders it CI-fail is exactly the divergence a
// second, local copy of this rule would reintroduce. The rollupEntry methods above
// (groupKey, recencyKey) are this type's adapters into the generic reducer.
//
// The reduction NEVER relaxes the gate: it changes only WHICH run of a name is evaluated,
// never how a run is judged — the reduced set still flows through evalRollup/failedChecks
// unchanged, so a name whose latest run is red, cancelled, or pending still reddens or
// blocks the flip exactly as before.
func latestPerRollupName(entries []rollupEntry) []rollupEntry {
	return deskkit.LatestRunPerName(entries, rollupEntry.groupKey, rollupEntry.recencyKey)
}

type reviewInfo struct {
	User        struct{ Login string }
	State       string
	CommitID    string
	Body        string
	SubmittedAt string
}

// --- forge reads: every one served by the resolved Forge backend ------------------
//
// Each read below states the CONDITION it belongs to in its refusal, because a read that
// fails is not a neutral fact: it is that condition returning could-not-check, and a
// could-not-check is never the cleared answer.

// readPR reads the change's state, draft flag, mergeability, head, changed-file count and
// current labels, and keeps the change itself for the ready mutation.
//
// It NO LONGER requests a status-check rollup alongside the PR. The rollup is a separate
// operation (readChecks) for two reasons: the already-flipped-and-correctly-labelled no-op
// path returns before the checks matter, so the common re-run makes one fewer read; and the
// rollup this gate judges is now the one at the head it verified, addressed by SHA, rather
// than whatever the PR document happened to embed.
func readPR(o flipOpts, fg deskkit.Forge, fr deskkit.ForgeRepo) (prInfo, error) {
	ch, err := fg.GetPullRequest(fr, o.pr)
	if err != nil {
		return prInfo{}, deskkit.Unverifiable(fmt.Sprintf(
			"condition %s: cannot read PR #%d in %s (%s) — a PR whose state could not be read is not a PR "+
				"that may be flipped.", condPROpenDraft, o.pr, fr.Slug(), firstLine(err.Error())), err)
	}
	labels := make([]labelInfo, 0, len(ch.Labels))
	for _, l := range ch.Labels {
		labels = append(labels, labelInfo{Name: l})
	}
	return prInfo{
		Number:       ch.Number,
		State:        ch.State,
		IsDraft:      ch.Draft,
		Mergeable:    ch.Mergeable,
		HeadRefOid:   ch.HeadSHA,
		ChangedFiles: ch.ChangedFiles,
		Labels:       labels,
		change:       ch,
	}, nil
}

// readChecks reads the two CI rollups AT THE HEAD the gate verified and flattens them into
// the single entry list the reduction and the green/pending/fail evaluation run over.
//
// It FAILS CLOSED on a short read. Each rollup carries the forge's OWN asserted total, and a
// walk that returned fewer entries than the head claims is a rollup nobody read in full —
// which on this gate would mean judging a head green on a partial view, the exact fail-open
// the paginated reads exist to prevent. That is could-not-check, never green.
func readChecks(o flipOpts, fg deskkit.Forge, fr deskkit.ForgeRepo, head string) ([]rollupEntry, error) {
	checks, err := fg.ChecksAtHead(fr, head)
	if err != nil {
		return nil, deskkit.Unverifiable(fmt.Sprintf(
			"condition %s: cannot read the check rollups at %s (%s) — a rollup that could not be read is "+
				"could-not-check, and could-not-check is never green.",
			condChecksGreen, short(head), firstLine(err.Error())), err)
	}
	if checks.StatusTotalCount > len(checks.Statuses) || checks.CheckRunsTotalCount > len(checks.CheckRuns) {
		return nil, deskkit.Unverifiable(fmt.Sprintf(
			"condition %s: read %d/%d status contexts and %d/%d check runs at %s — the forge asserts more "+
				"than it served, so the rollup was not read in full and cannot be judged green.",
			condChecksGreen, len(checks.Statuses), checks.StatusTotalCount,
			len(checks.CheckRuns), checks.CheckRunsTotalCount, short(head)), nil)
	}
	out := make([]rollupEntry, 0, len(checks.Statuses)+len(checks.CheckRuns))
	for _, s := range checks.Statuses {
		out = append(out, rollupEntry{Context: s.Context, State: s.State, CreatedAt: s.CreatedAt})
	}
	for _, c := range checks.CheckRuns {
		out = append(out, rollupEntry{
			Name: c.Name, Status: c.Status, Conclusion: c.Conclusion,
			StartedAt: c.StartedAt, CompletedAt: c.CompletedAt,
		})
	}
	return out, nil
}

func readReviews(o flipOpts, fg deskkit.Forge, fr deskkit.ForgeRepo) ([]reviewInfo, error) {
	reviews, err := fg.ReviewsAtHead(fr, o.pr)
	if err != nil {
		return nil, deskkit.Unverifiable(fmt.Sprintf(
			"condition %s: cannot read PR #%d's reviews (%s) — an unreadable review list is could-not-check, "+
				"and could-not-check is never an approval.", condReviewerApproved, o.pr, firstLine(err.Error())), err)
	}
	out := make([]reviewInfo, 0, len(reviews))
	for _, r := range reviews {
		var ri reviewInfo
		ri.User.Login = r.Author.Login
		ri.State = r.State
		ri.CommitID = r.CommitID
		ri.Body = r.Body
		ri.SubmittedAt = r.SubmittedAt
		out = append(out, ri)
	}
	return out, nil
}

// readChangedFiles reads the COMPLETE changed-file list for the PR — the backend walks the
// listing to exhaustion.
//
// It intentionally does NOT decide anything about the length it got. Reconciling the count
// against the forge's asserted `changedFiles` belongs to checkSecurityVerdict, where the
// fail-closed refusal already lives; a reader that also judged would give the gate two
// places to be lenient in.
//
// The pre-rename path is not read. On a rename the forge reports the DESTINATION path, and
// that is the path the risk-class determination has always been given — widening it to the
// source path too would change what counts as risk-classed, which is policy and not this
// read's business.
func readChangedFiles(o flipOpts, fg deskkit.Forge, fr deskkit.ForgeRepo) ([]fileInfo, error) {
	files, err := fg.ListChangedFiles(fr, o.pr)
	if err != nil {
		return nil, deskkit.Unverifiable(fmt.Sprintf(
			"condition %s: cannot read PR #%d's changed files (%s) — a diff that could not be read is "+
				"could-not-check, and could-not-check is never a clean risk classification.",
			condSecurityVerdict, o.pr, firstLine(err.Error())), err)
	}
	out := make([]fileInfo, 0, len(files))
	for _, f := range files {
		out = append(out, fileInfo{Path: f.Filename})
	}
	return out, nil
}

func readHead(o flipOpts, fg deskkit.Forge, fr deskkit.ForgeRepo) (string, error) {
	ch, err := fg.GetPullRequest(fr, o.pr)
	if err != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"condition %s: cannot re-read PR #%d's head immediately before the flip (%s) — without the "+
				"re-read there is no way to know the verified state is still current, so the flip does not "+
				"happen.", condHeadStable, o.pr, firstLine(err.Error())), err)
	}
	return strings.TrimSpace(ch.HeadSHA), nil
}

// --- CI reduction ----------------------------------------------------------------

type ciState int

const (
	ciEmpty ciState = iota
	ciGreen
	ciPending
	ciFail
)

// evalRollup reduces the rollup to one state. NOT-COMPLETED and UNRECOGNISED both fall to
// pending rather than to green: a conclusion this reader does not know is a conclusion it
// has not verified, and the only safe reading of an unverified check is "not yet green".
func evalRollup(entries []rollupEntry) ciState {
	if len(entries) == 0 {
		return ciEmpty
	}
	pending := false
	for _, e := range entries {
		switch {
		case e.Conclusion != "":
			switch strings.ToUpper(e.Conclusion) {
			case "SUCCESS", "NEUTRAL", "SKIPPED":
			default:
				return ciFail
			}
		case e.State != "":
			switch strings.ToUpper(e.State) {
			case "SUCCESS":
			case "PENDING", "EXPECTED":
				pending = true
			default:
				return ciFail
			}
		default:
			// A check run that has not completed carries a status and no conclusion.
			pending = true
		}
		if e.Conclusion == "" && e.State == "" && !strings.EqualFold(e.Status, "COMPLETED") {
			pending = true
		}
	}
	if pending {
		return ciPending
	}
	return ciGreen
}

func failedChecks(entries []rollupEntry) []string {
	var out []string
	for _, e := range entries {
		bad := false
		if e.Conclusion != "" {
			switch strings.ToUpper(e.Conclusion) {
			case "SUCCESS", "NEUTRAL", "SKIPPED":
			default:
				bad = true
			}
		} else if e.State != "" {
			switch strings.ToUpper(e.State) {
			case "SUCCESS", "PENDING", "EXPECTED":
			default:
				bad = true
			}
		}
		if bad {
			out = append(out, e.label())
		}
	}
	if len(out) == 0 {
		return []string{"(the rollup reports a failure but names no check)"}
	}
	return out
}

func audit(o flipOpts, err error) {
	result := deskkit.ResultOK
	detail := fmt.Sprintf("flipped PR #%d", o.pr)
	if o.dryRun {
		detail = fmt.Sprintf("dry-run PR #%d", o.pr)
	}
	if err != nil {
		switch deskkit.ExitCodeOf(err) {
		case deskkit.ExitRefused:
			result = deskkit.ResultRefused
		default:
			result = deskkit.ResultUnverifiable
		}
		detail = firstLine(err.Error())
	}
	if lerr := deskkit.Log(deskkit.Entry{
		Tool:   toolName,
		Verb:   "flip",
		Result: result,
		Detail: detail,
		Title:  fmt.Sprintf("%s#%d", o.repo, o.pr),
	}); lerr != nil {
		fmt.Fprintf(os.Stderr, "deskflip: WARNING: could not write audit line: %v\n", lerr)
	}
}
