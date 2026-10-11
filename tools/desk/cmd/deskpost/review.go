package main

import (
	"fmt"
	"strings"

	"github.com/medici-finance/assay/tools/desk/cmd/deskpost/internal/bodycheck"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// runReview posts a head-pinned review AS THE REVIEWER APP. --head is
// REQUIRED and must equal the PR's CURRENT head: a verdict must never land on code the
// reviewer did not form its verdict against. The App identity is the whole
// point — deskpost mints the App token and posts as the reviewer App; it cannot
// post as, and does not fall back to, the caller's account, so a PR author still cannot
// forge an approval.
func runReview(owner, name string, pr int, verdictFlag, head string, body []byte, args []string, opts postOpts) int {
	shape, ok := correctnessShapeFor(verdictFlag)
	if !ok {
		fmt.Fprintln(stderr, "deskpost: review --verdict must be 'approve' or 'request-changes'")
		return 2
	}
	return postVerdictReview(owner, name, pr, shape, head, body, args, opts)
}

// runSecurityReview posts the SECURITY verdict as its own head-pinned
// review (#513 / #438). It is `review`'s sibling, not a second implementation:
// same body checks, same head assertion, same trust gate, same budget, same audit line,
// same two idempotency guards — only the shape differs.
//
// WHY IT IS A SEPARATE VERB RATHER THAN A `--verdict` VALUE ON `review`. The two artifacts
// a risk-classed PR requires at one head are submitted with DIFFERENT GitHub events, and
// the event is the whole point of the fix:
//
//   - a clean security pass submits the COMMENT event → state COMMENTED. GitHub records it
//     in GET /pulls/{n}/reviews with a commit_id, so gate (e) can SEE it, while
//     COMMENTED never enters GitHub's approval reduction — the board does not go green and
//     a live correctness CHANGES_REQUESTED is not dismissed. That pair of properties is
//     exactly what the pr-review-desk skill was reaching for when it prescribed an issue
//     comment, which no review reader can see (#513: five live artifacts, gate (e) blind to
//     all of them, #455 one correctness verdict away from being unflippable).
//   - a security blocker submits REQUEST_CHANGES, unchanged: a retraction should be as loud
//     as GitHub can make it.
//
// WHERE THE HEAD-BINDING ACTUALLY COMES FROM, because an earlier draft of this comment said
// "server-side commit_id" and that is wrong in a direction that invites a real weakening.
// On POST /pulls/{n}/reviews the commit_id is CALLER-SUPPLIED — github.go sends it in the
// request body, and GitHub defaults it to the latest commit only when it is omitted, which
// deskpost never does. What makes the stamped SHA truthful is postVerdictReview's OWN check
// a few lines below: it re-reads the PR's live head and refuses when curHead != head, before
// the POST. So the pin is as strong as the comment claimed, but it is this tool's guard and
// not GitHub's. Anyone reading it as a server guarantee could delete that check as redundant
// — and then verdicts would land on whatever SHA the caller typed. It is not redundant.
//
// The correctness lane cannot be satisfied by either, and now for a STRUCTURAL reason on
// top of the parse-based one: latestAppVerdict skips every state that is not APPROVED /
// CHANGES_REQUESTED before it reads a body at all, so a COMMENTED pass cannot reach the
// correctness reduction even if classifyLane misread it. The `--approve` shape (#513
// direction 3) has no such backstop — it re-couples the lanes at the GitHub level and rests
// entirely on a body parse this package measures as failing on most live bodies.
func runSecurityReview(owner, name string, pr int, verdictFlag, head string, body []byte, args []string, opts postOpts) int {
	shape, ok := securityShapeFor(verdictFlag)
	if !ok {
		fmt.Fprintln(stderr, "deskpost: security-review --verdict must be 'pass' or 'fail'")
		return 2
	}
	return postVerdictReview(owner, name, pr, shape, head, body, args, opts)
}

// reviewShape is everything that differs between the two verdict verbs: the audit/
// idempotency flag component, the GitHub submit EVENT, the review STATE that event
// produces (which the cross-session dup guard matches on), and — for the security verb —
// the verdict kind and marker the body is REQUIRED to carry.
//
// Deriving the state from the event here rather than at the call site is deliberate: the
// dup guard compares against a state GitHub will report, and a hand-maintained second
// mapping is how "COMMENT produces COMMENTED" becomes wrong later.
type reviewShape struct {
	flag     string     // "approve" | "request-changes" | "pass" | "fail"
	event    string     // APPROVE | REQUEST_CHANGES | COMMENT
	state    string     // APPROVED | CHANGES_REQUESTED | COMMENTED
	wantKind string     // required bodycheck kind; "" = whatever the body carries
	wantSec  secVerdict // required security marker when wantKind is KindSecurity
}

func correctnessShapeFor(verdictFlag string) (reviewShape, bool) {
	switch verdictFlag {
	case "approve":
		return reviewShape{flag: "approve", event: "APPROVE", state: "APPROVED", wantKind: bodycheck.KindCorrectness}, true
	case "request-changes":
		return reviewShape{flag: "request-changes", event: "REQUEST_CHANGES", state: "CHANGES_REQUESTED", wantKind: bodycheck.KindCorrectness}, true
	}
	return reviewShape{}, false
}

func securityShapeFor(verdictFlag string) (reviewShape, bool) {
	switch verdictFlag {
	case "pass":
		return reviewShape{flag: "pass", event: "COMMENT", state: "COMMENTED",
			wantKind: bodycheck.KindSecurity, wantSec: secPass}, true
	case "fail":
		return reviewShape{flag: "fail", event: "REQUEST_CHANGES", state: "CHANGES_REQUESTED",
			wantKind: bodycheck.KindSecurity, wantSec: secFail}, true
	}
	return reviewShape{}, false
}

// findingLaneRefusal refuses a review body whose finding block names a lane the posting
// verb does not speak for: `security-review` posts the security lane and `review` posts the
// correctness lane, so every lane a block states must be that verb's own. The deep-set lanes
// (`fact-check`, `fail-first`) are refused on both verbs: the record `review` writes carries
// the correctness lane, and the ledger keys a reviewer finding under its record's lane, so a
// block stating a deep-set lane would be keyed under correctness anyway and reported
// could-not-check in every later fold — which the content-defect check reads as blocking. A
// block stating no lane, or a body with no block, passes — the record's lane is established
// from the body.
func findingLaneRefusal(body []byte, wantKind string) error {
	var verb, own string
	switch wantKind {
	case bodycheck.KindSecurity:
		verb, own = "security-review", deskkit.LaneSecurity.Name
	case bodycheck.KindCorrectness:
		verb, own = "review", deskkit.LaneCorrectness.Name
	default:
		return nil
	}
	b, present, err := deskkit.ParseFindingBlock(string(body))
	if err != nil || !present {
		return err
	}
	for _, f := range b.Findings {
		got := f.StatedLane()
		if got == "" || got == own {
			continue
		}
		hint := fmt.Sprintf("drop the lane or state %q", own)
		if got == deskkit.LaneSecurity.Name {
			hint = "post security findings with `deskpost security-review`"
		}
		return deskkit.Refused(fmt.Sprintf("refused: `%s` posts the %s lane, but finding %s states lane %q — a reviewer speaks only for its own lane, and the record this verb writes carries only the %s lane; %s",
			verb, own, f.ID, got, own, hint))
	}
	return nil
}

// postVerdictReview is the one write path both verdict verbs run. Extracting it is what
// keeps `security-review` from becoming a second, drifting copy of the hardening in
// `review` (#197 head pinning, #73 cross-session dedup, #220 kinded keys, #238/#239
// unreadable-body handling).
func postVerdictReview(owner, name string, pr int, shape reviewShape, head string, body []byte, args []string, opts postOpts) int {
	repo := owner + "/" + name
	verdictFlag := shape.flag
	dig := deskkit.Sha256Hex(body)
	// preVerb labels audit lines for refusals raised BEFORE the body's verdict kind is
	// known (bad repo, bad body, unparseable kind). It is deliberately NOT an idempotency
	// key: AlreadyDoneIn matches ok/noop entries only, and every ok/noop path below is
	// reached after the kind is parsed and so carries the full kinded verb.
	preVerb := "review:" + verdictFlag

	return runOutward(args, opts, repo, pr, func(entries []deskkit.Entry, opts postOpts) writeResult {
		if !deskkit.IsAllowedRepo(repo) {
			return refused(preVerb, repo, pr, "", "repo "+repo+" is not in the fixed desk repo set")
		}
		// Body validation BEFORE any network — a bad body must refuse with zero side
		// effects: the size cap and verdict schema, then the ONE outbound-write check
		// (desktools-v2/10) on this target. A review body is the densest evidence surface
		// the desk writes — it quotes paths, cites issues across repos and names streams —
		// which is precisely why it is also the likeliest to carry a span that resolves only
		// inside the house; the check's public layers (self-containment, withheld register)
		// run on any target not stated private, and its credential arms, impersonation guard
		// and personal-data pass run everywhere. The checking Forge and the raw client re-run
		// it at the write itself.
		if err := bodycheck.Review(body); err != nil {
			deskkit.MaybeExplain(stderr, opts.explain, err)
			return withDigest(fromReadErr(preVerb, repo, pr, "", err), dig)
		}
		if err := deskkit.OutboundCheck(deskkit.OutboundWrite{Repo: repo, Kind: deskkit.OutboundKindReview, NumberHint: pr,
			Fields: []deskkit.OutboundField{{Name: "body", Text: string(body)}}}); err != nil {
			deskkit.MaybeExplain(stderr, opts.explain, err)
			return withDigest(fromReadErr(preVerb, repo, pr, "", err), dig)
		}
		// A review body MAY carry a typed persistent finding block (additive — a legacy body
		// carries none and this is a no-op). Validate it for the REVIEWER role before any
		// network: a reviewer may raise, maintain and resolve findings, but a malformed block
		// — a blocking finding with no concrete reproduction/evidence, an unknown state — is a
		// refusal with zero side effects, the same as every other pre-network body check.
		if err := deskkit.ValidateReviewFindingBlock(body, deskkit.RoleReviewer); err != nil {
			return withDigest(fromReadErr(preVerb, repo, pr, "", err), dig)
		}
		// On-behalf-of trailer (multi-principal/01): resolved before any network call,
		// refuses (exit 5) rather than post without one. Appended to the POSTED body only
		// — `dig` (and the local-log idempotency key derived from it) stays keyed on the
		// CALLER-supplied body. The forge-state check below compares against POSTED bodies,
		// which carry the trailer, so it reduces both sides with reviewComparableBody; that
		// is what lets the same verdict retried from a different session still dedupe.
		postBody, oerr := deskkit.AppendOnBehalfOf(body, "", repo)
		if oerr != nil {
			return withDigest(fromReadErr(preVerb, repo, pr, "", oerr), dig)
		}
		// The verdict KIND (correctness vs security) comes from the BODY,
		// not the flag — both kinds post as the same --verdict. It is part of the
		// idempotency key below; VerdictKind fails closed rather than yielding a key that
		// merges the two (#220).
		kind, kerr := bodycheck.VerdictKind(body)
		if kerr != nil {
			return withDigest(fromReadErr(preVerb, repo, pr, "", kerr), dig)
		}
		// EACH verdict verb declares the kind its body must carry, and refuses a body
		// from the other lane — before any network call. The two refusals are symmetric
		// on purpose, and neither is decoration:
		//
		//   - `security-review --verdict pass` submits the COMMENT event; handed a
		//     `Security-Review: fail` body it would post a RETRACTION as a review that
		//     blocks nothing on GitHub's side. Gate (e0) would still read the fail and
		//     block the flip, so nothing fails open — but the artifact would misrepresent
		//     itself to every human reading the thread.
		//   - `review --verdict approve` submits the APPROVE event; handed a
		//     `Security-Review: pass` body it posts the security lane's all-clear as an
		//     APPROVED review — the exact same-head APPROVE shape the verb split exists to
		//     keep a security pass OUT of (a COMMENTED pass is readable by gate (e) while
		//     leaving GitHub's review roll-up alone; an APPROVED one erases a standing
		//     CHANGES_REQUESTED from the shared App). This happened for real when two
		//     lanes dispatched to one PR shared a scratchpad and one lane's default body
		//     filename was read by the other's `review --verdict approve`; before this
		//     guard the verb let it through and the stray APPROVE had to be dismissed by
		//     hand.
		//
		// A refusal costs one exit 5; a submitted review cannot be retracted.
		if shape.wantKind != "" && kind != shape.wantKind {
			var msg string
			if shape.wantKind == bodycheck.KindSecurity {
				msg = fmt.Sprintf("refused: `security-review` posts the SECURITY verdict, but this body carries a "+
					"%s verdict line — post a correctness verdict with `deskpost review --verdict approve|request-changes`",
					kind)
			} else {
				msg = fmt.Sprintf("refused: `review` posts the CORRECTNESS verdict, but this body carries a "+
					"%s verdict line — post a security verdict with `deskpost security-review --verdict pass|fail` "+
					"(a security PASS must land as a COMMENTED review, never APPROVED)", kind)
			}
			return withDigest(fromReadErr(preVerb, repo, pr, "", deskkit.Refused(msg)), dig)
		}
		// The kind check above parses STRICTLY (VerdictKind: whole-line anchored, no
		// Markdown-emphasis unwrapping — the write gate's rule). The flip gate and the
		// board read with the TOLERANT reader (#232/#238: `**Security-Review: pass**`
		// counts, because live artifacts wrap markers in emphasis). So a body carrying a
		// bare `Verdict: approve` PLUS an emphasised security marker parses as pure
		// correctness here, would post as APPROVED, and would then be READ as a security
		// pass at that head — the strict/tolerant split reopening the exact shape the
		// kind check closes. `review` therefore also refuses whatever the tolerant reader
		// would call a security verdict. A line quoted with a leading `> ` is a citation
		// to that reader too, so citing the other lane stays possible.
		if shape.wantKind == bodycheck.KindCorrectness {
			if got := classifySecurityBody(string(body)); got != secNone {
				return withDigest(fromReadErr(preVerb, repo, pr, "", deskkit.Refused(fmt.Sprintf(
					"refused: `review` posts the CORRECTNESS verdict, but this body also carries a "+
						"'Security-Review: %s' marker that the flip gate reads as a security verdict "+
						"(emphasis such as `**Security-Review: pass**` counts) — a review posts exactly ONE "+
						"verdict kind: post the security verdict with `deskpost security-review --verdict pass|fail`, "+
						"or quote the other lane's line (prefix '> ') when citing it",
					secVerdictName(got)))), dig)
			}
		}
		if shape.wantKind == bodycheck.KindSecurity {
			if got := classifySecurityBody(string(body)); got != shape.wantSec {
				return withDigest(fromReadErr(preVerb, repo, pr, "", deskkit.Refused(fmt.Sprintf(
					"refused: --verdict %s does not match the body, which reads as %s — the flag and the "+
						"'Security-Review:' line must agree, or the posted artifact misstates its own verdict",
					verdictFlag, secVerdictName(got)))), dig)
			}
		}
		// A finding block speaks for the lane of the verb that posts it. The ledger already
		// keys a reviewer record's findings under the record's own lane whatever the block
		// says; refusing a block that names another lane here catches the confusion at the
		// write, a second and independent point, before it becomes a could-not-check entry
		// in every later fold.
		if err := findingLaneRefusal(body, kind); err != nil {
			return withDigest(fromReadErr(preVerb, repo, pr, "", err), dig)
		}
		verb := reviewVerbFor(kind, verdictFlag)
		// Idempotency BEFORE any network: --head is the caller-provided reviewed SHA, so
		// a repeat of the same verdict at the same head is a no-op with ZERO HTTP calls
		// (a prior ok entry at that head was only recorded after curHead==head was
		// verified, so re-noop'ing is safe). The key carries the KIND, so the OTHER
		// required verdict at that same head is not mistaken for this one's repeat — and
		// the match additionally requires the recorded BODY DIGEST to equal this body's
		// (#518): two desk lanes sharing one HOME can legitimately post the
		// same kind+flag at the same head with DIFFERENT findings, and only the body
		// distinguishes that from a true retry. See reviewAlreadyPostedIn.
		if reviewAlreadyPostedIn(entries, repo, pr, head, verb, dig) {
			return withDigest(noop(verb, repo, pr, head, "already posted "+verb+" with this exact body at "+short(head)+" (idempotent no-op)"), dig)
		}

		client, err := newPostBackend(owner, name)
		if err != nil {
			return withDigest(fromReadErr(verb, repo, pr, "", err), dig)
		}
		info, err := client.getPR(pr)
		if err != nil {
			// A number that names an ISSUE is a refusal naming `comment`, not the generic
			// unverifiable (#296) — see requirePRErr.
			return withDigest(fromReadErr(verb, repo, pr, "", requirePRErr(client, repo, pr, err)), dig)
		}
		curHead := info.Head.SHA
		if curHead != head {
			return withDigest(refused(verb, repo, pr, curHead,
				fmt.Sprintf("--head %s != current head %s — a verdict must not land on unreviewed code", short(head), short(curHead))), dig)
		}
		// Model-capability floor. A review verdict is an authority-bearing write, so it
		// requires a strong-tier dispatch. The tier is read from the target PR's
		// DISPATCHER-attested stamp (applier-aware, so a self-applied stamp is worthless),
		// and it FAILS CLOSED: an attested below-tier dispatch, or a stamp present-but-
		// unreadable, refuses with remediation. An UNATTESTED PR (a human-driven session or
		// a pre-attestation dispatch) is not bricked — it proceeds with a NOTICE. The
		// override is an env toggle, and every bypass is logged loudly. A timeline that
		// cannot be READ is could-not-check, never a cleared floor.
		tl, ferr := client.stampTimeline(pr)
		if ferr != nil {
			return withDigest(fromReadErr(verb, repo, pr, curHead, ferr), dig)
		}
		// The reviewer stamp this verdict validates ages out when the REVIEW-dispatch claim
		// behind it (the "<short>--pr-<N>" family, not this PR's worker Brief: claim) is no longer
		// held — the review cycle is over, so the stamp attests nothing about this verdict and the
		// PR reads unstamped (claimLiveness → deskkit review-claim family). Every uncertain path is
		// Unknown and changes nothing.
		claim := client.claimLiveness(repo, pr)
		fd := deskkit.ModelCapabilityFloor(tl, deskkit.IsStampAuthorityLogin, deskkit.ModelFloorOverrideEngaged(), claim)
		// Ruling 3: the floor is RISK-CONDITIONAL on an UNSTAMPED PR. A review verdict is a
		// security-review-bearing write, so on a risk-classed PR it must carry a trustable
		// strong-tier attestation; an unstamped NON-risk PR still proceeds with a NOTICE. Only
		// the proceed-with-NOTICE outcome is subject to the overlay, and ONLY THEN is the
		// changed-file list needed — a stamped verdict clears the floor without reading the
		// diff. Risk reuses the SAME RiskPathTriggered signal the ready-flip's security gate
		// reads; diff readability is resolved here with its own could-not-check exit, so the
		// floor is handed a definite risk and never mislabels an unread diff as a plain refusal.
		if fd.Outcome == deskkit.FloorNoticeAllow {
			prFiles, filesErr := client.listFiles(pr)
			if filesErr != nil {
				return withDigest(fromReadErr(verb, repo, pr, curHead, filesErr), dig) // exit 6
			}
			if info.ChangedFiles > 0 && len(prFiles) < info.ChangedFiles {
				return withDigest(unverifiableNoWrite(verb, repo, pr, curHead,
					fmt.Sprintf("read %d changed files but GitHub reports %d for PR #%d — the diff could not be "+
						"read in full, so the risk-class determination behind the model floor is unverifiable",
						len(prFiles), info.ChangedFiles, pr), nil), dig)
			}
			fd = deskkit.ModelCapabilityFloorRiskAware(tl, deskkit.IsStampAuthorityLogin, deskkit.ModelFloorOverrideEngaged(),
				claim, deskkit.FloorRiskOf(repo, prFilePaths(prFiles)))
		}
		fd.Message += claimReleaseNote(repo, pr, claim)
		switch fd.Outcome {
		case deskkit.FloorRefuse:
			return withDigest(refused(verb, repo, pr, curHead, fd.Message), dig)
		case deskkit.FloorOverrideAllow, deskkit.FloorNoticeAllow:
			// Loud (override) or a NOTICE (absent): both proceed, but neither silently.
			fmt.Fprintln(stderr, "deskpost: "+fd.Message)
		case deskkit.FloorAllow:
			// Attested at/above the floor: proceed silently.
		}
		// GitHub-STATE idempotency (#73). The local guard above reads only
		// this HOME's audit log; a retry from a FRESH reviewer subagent (empty log —
		// e.g. re-dispatched after a masked exit code) would sail past it and POST A SECOND
		// identical review. A submitted review cannot be retracted, so that duplicate is
		// permanent thread noise. Before posting, read the PR's ACTUAL reviews and no-op if
		// this App already carries THIS verdict — same resulting state, same kind, and
		// (#518) the same body content — pinned to the current head. Matched
		// on login AND commit_id AND state AND kind AND body digest, so neither a genuine
		// verdict CHANGE at the same head (request-changes → approve) nor a DISTINCT
		// same-shaped review from another lane is suppressed.
		existing, err := client.listReviews(pr)
		if err != nil {
			return withDigest(fromReadErr(verb, repo, pr, curHead, err), dig)
		}
		dup, why := appReviewExistsAt(existing, curHead, shape.state, kind, reviewBodyDigest(body))
		// writeGates are the two gates every outward write of this verb passes: the trust
		// gate (no write on unvetted third-party work; exit 5, audited) and the public-repo
		// gate (private, or a listed :public allowed-repos entry — see
		// deskkit.PublicRepoGate). The post below passes them; so does a merge-hold write on
		// the already-recorded path, which is an outward write with no post before it.
		writeGates := func() (writeResult, bool) {
			if terr := prTrustGate(client, pr, info.User.Login, info.User.ID); terr != nil {
				return fromReadErr(verb, repo, pr, head, terr), true
			}
			if gerr := deskkit.PublicRepoGate(client, owner, name); gerr != nil {
				return fromErr(verb, repo, pr, head, gerr), true
			}
			return writeResult{}, false
		}
		if dup {
			// #238(2): the audit line records WHAT was suppressed and WHY, so a
			// duplicate-suppression is distinguishable in the ledger from every other
			// reason a write did not happen. The why names the suppressing review's id
			// and author (#518 direction 2), so a caller can tell "my retry" from
			// "someone else's verdict" without another API call.
			found := "equivalent " + verb + " by " + reviewerBotDisplay() + " already present at " + short(curHead) +
				" (idempotent no-op; " + why + ")"
			if shape.wantKind != bodycheck.KindCorrectness {
				// The security lane's verdict has no merge-hold step (see the post path).
				return withDigest(noop(verb, repo, pr, curHead, found), dig)
			}
			// Nothing is posted — but the post is only the first half of what this verb
			// does for a correctness verdict. A run that posted and then failed its
			// merge-hold write exits non-zero, and the retry it asks for arrives HERE; the
			// hold is checked against the recorded verdict before this run may exit 0.
			return withDigest(holdForRecordedVerdict(recordedVerdict{
				client: client, verb: verb, repo: repo, pr: pr, head: curHead, event: shape.event,
				reviews: existing, at: lastRecordedVerdict(existing, curHead, shape.state, kind, reviewBodyDigest(body)),
				bodyDig: dig, found: found, dryRun: opts.dryRun, gates: writeGates,
			}), dig)
		}
		if why != "" {
			// NOT a duplicate, but the App carries same-shaped material at this head:
			// either a review whose kind could not be read (#238/#239) or a same-kind
			// verdict with a DIFFERENT body (#518 — another lane's findings, or a retry
			// whose body changed between attempts). Posting is the safe answer in both —
			// a visible duplicate beats an invisible drop — but it must never be SILENT:
			// the operator has to know what this post landed next to. Fail loud, then
			// proceed.
			fmt.Fprintln(stderr, "deskpost: WARNING: "+why)
		}
		// Trust gate, then public-repo gate — see writeGates above.
		if wr, stop := writeGates(); stop {
			return withDigest(wr, dig)
		}
		// Non-author verdict assertion (sdlc/10) — the SECOND layer behind the forge's own
		// "an author cannot approve their own PR" refusal. The forge's refusal is keyed on
		// PR authorship and may NOT fire on a collapsed identity path (the supported minimal
		// set sdlc/10's human gate is deciding); this one fires at verdict time, in the desk
		// tool, on identity equality against the CERTIFIED HEAD. The posting identity is the
		// reviewer App; the certified identity is who authored the head commit. An unreadable
		// head-commit author is a could-not-check: it FALLS BACK to the PR author (always
		// present) rather than vanishing, and warns — never a silent pass.
		posting := reviewerBotDisplay()
		headAuthor, haErr := client.headCommitAuthor(curHead)
		if haErr != nil || strings.TrimSpace(headAuthor) == "" {
			if haErr != nil {
				fmt.Fprintln(stderr, "deskpost: WARNING: could not read head-commit author for the non-author "+
					"verdict check ("+haErr.Error()+") — falling back to the PR author")
			} else {
				fmt.Fprintln(stderr, "deskpost: WARNING: GitHub attributes the head commit to no account — "+
					"falling back to the PR author for the non-author verdict check")
			}
			headAuthor = info.User.Login
		}
		switch deskkit.NonAuthorVerdict(posting, headAuthor) {
		case deskkit.NonAuthorRefused:
			return withDigest(fromErr(verb, repo, pr, curHead,
				deskkit.AssertNonAuthorVerdict(posting, headAuthor)), dig)
		case deskkit.NonAuthorUnknown:
			// Both the head-commit author AND the PR author were unreadable — could-not-check.
			// Proceed (a transient read gap must not brick the reviewer loop) but say so.
			fmt.Fprintln(stderr, "deskpost: WARNING: could not determine the head author for the non-author "+
				"verdict check — proceeding, but the poster-vs-author separation could NOT be verified")
		case deskkit.NonAuthorOK:
			// Poster and head author are distinct actors — the separation holds.
		}
		if opts.dryRun {
			return withDigest(dryRun(verb, repo, pr, head,
				"DRY RUN: review body passed the verdict schema, size cap and secret scan (kind="+kind+
					"), --head matches the current head "+short(head)+", trust gate passed, public-repo "+
					"gate passed, no equivalent verdict already at head — stopped before POST"), dig)
		}
		postNote, err := client.postReview(pr, head, shape.event, string(postBody))
		if err != nil {
			return withDigest(fromErr(verb, repo, pr, head, err), dig)
		}
		if postNote != "" {
			// The verdict is in force by a route other than the plain POST (GitLab's
			// already-approved 401, #1106): success, but say which route it was.
			fmt.Fprintln(stderr, "deskpost: NOTE: "+postNote)
		}
		// Merge-hold release/re-arm (the forge-gitlab merge-hold brief, task 3): the CORRECTNESS
		// verdict's own gate — the security lane (wantKind == KindSecurity) touches it not at
		// all, and keeps its own gate (deskflip's security-verdict). A GitHub-resolved repo's
		// readMergeHold answers the typed not-applicable and this is a no-op. A failure here
		// exits non-zero with the verdict already posted; the run that retries it finds the
		// verdict recorded and goes through holdForRecordedVerdict above.
		if shape.wantKind == bodycheck.KindCorrectness {
			if hErr := applyMergeHoldForVerdict(client, pr, shape.event, head); hErr != nil {
				return withDigest(fromErr(verb, repo, pr, head, hErr), dig)
			}
		}
		// Mechanical, ADVISORY verdict-time labels (diff size class + surface tier) for
		// merge-queue triage. This runs AFTER the verdict has landed and NEVER changes its
		// outcome: a labeling failure is logged as a WARNING and swallowed, so the verdict
		// still reports success. Labels gate nothing (they are a `wc -l` + glob triage aid),
		// and a could-not-classify family is skipped, never guessed.
		if lo, lerr := client.verdictLabels(pr, info.ChangedFiles); lerr != nil {
			fmt.Fprintln(stderr, "deskpost: WARNING: verdict-time labeling (advisory): "+lerr.Error())
		} else if s := lo.String(); s != "no label change" {
			fmt.Fprintln(stderr, "deskpost: verdict-time labels: "+s)
		}
		detail := "posted " + verdictFlag + " review as " + reviewerBotDisplay() + " at " + short(head)
		if postNote != "" {
			detail += " (" + postNote + ")"
		}
		return done(verb, repo, pr, head, dig, detail)
	})
}

// recordedVerdict is what holdForRecordedVerdict works from: the correctness verdict this
// run was asked to post, found already on the change.
type recordedVerdict struct {
	client  postBackend
	verb    string
	repo    string
	pr      int
	head    string // the change's current head, which the recorded verdict is pinned to
	event   string // APPROVE or REQUEST_CHANGES
	reviews []reviewInfo
	at      int    // index in reviews of the recorded verdict
	bodyDig string // digest of the body this run was given
	found   string // the duplicate guard's account of the match — the plain no-op's detail
	dryRun  bool
	// gates are the checks an outward write of this verb passes first. They are run only
	// when a hold write is about to be sent.
	gates func() (writeResult, bool)
}

// lastRecordedVerdict returns the index of the LAST review appReviewExistsAt matches — the
// same test, applied one review at a time from the newest end.
func lastRecordedVerdict(reviews []reviewInfo, head, wantState, wantKind, wantBodyDigest string) int {
	for i := len(reviews) - 1; i >= 0; i-- {
		if ok, _ := appReviewExistsAt(reviews[i:i+1], head, wantState, wantKind, wantBodyDigest); ok {
			return i
		}
	}
	return -1
}

// laterVerdictByReviewer reports the first review AFTER reviews[at] that the reviewer
// identity submitted as an approve or a request-changes and that is not readably the
// security lane's. Reviews arrive oldest first (Forge.ReviewsAtHead), so such a review may
// be a newer correctness verdict than the one at reviews[at]. A body whose kind cannot be
// read counts: "could not tell" is never "nothing followed".
func laterVerdictByReviewer(reviews []reviewInfo, at int) (id int64, found bool) {
	if at < 0 {
		return 0, true
	}
	for _, r := range reviews[at+1:] {
		if !isReviewerBot(r.User.Login) || (r.State != "APPROVED" && r.State != "CHANGES_REQUESTED") {
			continue
		}
		if k, err := bodycheck.VerdictKind([]byte(r.Body)); err == nil && k == bodycheck.KindSecurity {
			continue
		}
		return r.ID, true
	}
	return 0, false
}

// holdForRecordedVerdict is the merge-hold step of a run that posts NOTHING because its
// correctness verdict is already on the change at this head. It exists because the hold step
// otherwise runs only after a post: a run that posted a request-changes and then failed to
// re-arm the hold exits non-zero, and without this the retry would find the verdict, post
// nothing, skip the hold and exit 0 with the server-side gate still down.
//
// It only ever ARMS the hold or CONFIRMS it. It never releases one:
//
//   - a forge with no merge-hold (the typed not-applicable): the plain no-op, unchanged;
//   - the hold stands resolved at a head other than this one: re-armed, whatever the verdict
//     — no verdict at this head can have released it there;
//   - a recorded request-changes that no later verdict by the reviewer identity follows: the
//     hold step runs again exactly as it does after a post (a re-arm), and the run exits 0
//     with an `ok` row saying the verdict was not posted again;
//   - a recorded request-changes that a later verdict follows: no write is sent on its
//     account. An armed hold is confirmed (exit 0); a released one is a mismatch this run
//     cannot settle, so it exits non-zero naming the later review;
//   - a recorded approve: exit 0 only when the hold reads released at this head by the
//     reviewer identity. Anything else exits non-zero. A release is sent only by a run that
//     posts an approve, where the verdict and the release are one act of one run.
//
// A hold that cannot be read, or a change with no hold thread, exits non-zero: "could not
// check" is never reported as agreement. A hold write here is an outward write with no post
// before it, so it first passes the gates the post passes, and a dry run sends none.
func holdForRecordedVerdict(v recordedVerdict) writeResult {
	hold, err := v.client.readMergeHold(v.pr)
	if err != nil && deskkit.IsMergeHoldNotApplicable(err) {
		return noop(v.verb, v.repo, v.pr, v.head, v.found)
	}
	word := "approve"
	rc := strings.EqualFold(v.event, "REQUEST_CHANGES")
	if rc {
		word = "request-changes"
	}
	rec := fmt.Sprintf("the %s verdict is already recorded on PR #%d at %s and was not posted again", word, v.pr, short(v.head))
	if err != nil {
		return unverifiableNoWrite(v.verb, v.repo, v.pr, v.head, fmt.Sprintf(
			"%s, but the PR's merge-hold could not be read: %v — whether the server-side gate agrees with "+
				"the recorded verdict is unknown. Run the command again.", rec, err), err)
	}
	switch hold.State {
	case deskkit.MergeHoldNotApplicable:
		return noop(v.verb, v.repo, v.pr, v.head, v.found)
	case deskkit.MergeHoldUnresolved, deskkit.MergeHoldResolved:
	case deskkit.MergeHoldAbsent:
		return unverifiableNoWrite(v.verb, v.repo, v.pr, v.head, rec+", but the PR has no merge-hold thread, so "+
			"there is no server-side gate to bring into step with it. This command does not open one; the "+
			"command that opens a change does.", nil)
	default:
		return unverifiableNoWrite(v.verb, v.repo, v.pr, v.head, fmt.Sprintf(
			"%s, but the PR's merge-hold reports an unrecognised state %q.", rec, hold.State), nil)
	}

	var laterID int64
	superseded := false
	if rc {
		laterID, superseded = laterVerdictByReviewer(v.reviews, v.at)
	}
	stale := hold.State == deskkit.MergeHoldResolved && hold.Head != "" && hold.Head != v.head
	var writes []deskkit.MergeHoldUpdate
	if stale {
		writes = append(writes, deskkit.MergeHoldUpdate{Reason: fmt.Sprintf("new head %s", v.head)})
	}
	if rc && !superseded {
		writes = append(writes, deskkit.MergeHoldUpdate{Reason: "request-changes"})
	}
	if len(writes) > 0 {
		if wr, stop := v.gates(); stop {
			return wr
		}
		if v.dryRun {
			return dryRun(v.verb, v.repo, v.pr, v.head, "DRY RUN: "+rec+"; a real run would re-arm the PR's "+
				"merge-hold — stopped before any write")
		}
		for _, w := range writes {
			if serr := v.client.setMergeHold(v.pr, w); serr != nil {
				return unverifiable(v.verb, v.repo, v.pr, v.head, fmt.Sprintf(
					"%s, but re-arming the PR's merge-hold (%s) failed: %v — the server-side gate may still "+
						"read released. Run the command again.", rec, w.Reason, serr), serr)
			}
		}
	}
	wrote := len(writes) > 0
	rearmed := ""
	if wrote {
		reasons := make([]string, 0, len(writes))
		for _, w := range writes {
			reasons = append(reasons, w.Reason)
		}
		rearmed = "merge-hold re-armed (" + strings.Join(reasons, "; ") + ")"
	}

	if rc {
		switch {
		case wrote:
			return done(v.verb, v.repo, v.pr, v.head, v.bodyDig, v.found+"; the verdict was not posted again; "+rearmed)
		case hold.State == deskkit.MergeHoldUnresolved:
			return noop(v.verb, v.repo, v.pr, v.head, fmt.Sprintf(
				"%s; merge-hold is armed, as a request-changes requires (no hold write was sent: review id %d "+
					"by %s follows this verdict)", v.found, laterID, reviewerBotDisplay()))
		default:
			return unverifiableNoWrite(v.verb, v.repo, v.pr, v.head, fmt.Sprintf(
				"%s, but the PR's merge-hold reads released and a later verdict by %s (review id %d) follows "+
					"this one, so this run cannot tell which of the two the hold should follow and did not "+
					"change it. If the request-changes is the verdict that stands, post it again with a body "+
					"that says so: a body that differs is posted as a new review, and its hold step re-arms.",
				rec, reviewerBotDisplay(), laterID), nil)
		}
	}

	// A recorded approve. Confirm a release; never send one.
	reviewer, bound := deskkit.RoleAppLogin("reviewer")
	var state string
	switch {
	case stale:
		state = fmt.Sprintf("its merge-hold stood resolved at %s, not at this head — %s", short(hold.Head), rearmed)
	case hold.State == deskkit.MergeHoldUnresolved:
		state = "its merge-hold is still up"
	case hold.Head == "":
		state = "its merge-hold is resolved but names no head, so it was not released by an approve at this head"
	case !bound || !deskkit.SameActor(hold.ResolvedBy, reviewer):
		state = fmt.Sprintf("its merge-hold was resolved by %q, not by the reviewer identity", hold.ResolvedBy)
	default:
		return noop(v.verb, v.repo, v.pr, v.head, v.found+"; merge-hold reads released at this head by "+reviewer)
	}
	detail := rec + ", but " + state + ". A run that finds its verdict already recorded does not release a " +
		"hold; only a run that posts an approve does. To release it, post the approve again with a body " +
		"that says the release is being retried: a body that differs is posted as a new review, and its " +
		"hold step releases."
	if wrote {
		return unverifiable(v.verb, v.repo, v.pr, v.head, detail, nil)
	}
	return unverifiableNoWrite(v.verb, v.repo, v.pr, v.head, detail, nil)
}

// secVerdictName renders a secVerdict for a refusal message.
func secVerdictName(v secVerdict) string {
	switch v {
	case secPass:
		return "'Security-Review: pass'"
	case secFail:
		return "'Security-Review: fail'"
	default:
		return "neither pass nor fail (no readable 'Security-Review:' line)"
	}
}

// reviewVerbFor builds the discriminated idempotency/audit verb — `review:<kind>:<flag>`,
// e.g. `review:correctness:approve` / `review:security:pass`.
//
// The KIND is in the key because the FLAG alone does not identify the write
// (#220): a correctness approve and a `Security-Review: pass` are both
// `--verdict approve`, so a flag-only verb made the two required verdicts on a
// risk-classed PR collide, and whichever arrived second was dropped as an "idempotent
// no-op" — success-shaped output, no artifact, no signal. Both components are needed:
// dropping the flag would merge a `request-changes` into an `approve` of the same kind.
//
// The kind is never empty here — callers get it from bodycheck.VerdictKind, which refuses
// rather than returning a blank that would collapse this back to the flag-only key.
//
// Since #518 the full local idempotency identity is this verb PLUS the
// entry's recorded BodyDigest (reviewAlreadyPostedIn) — the digest is NOT encoded into
// the verb (unlike comment's "comment:<digest>") because every review entry already
// records it in the schema's own bodyDigest field, and keeping the verb stable keeps
// ledger history readable under one name per lane.
func reviewVerbFor(kind, verdictFlag string) string {
	return "review:" + kind + ":" + verdictFlag
}

// appReviewExistsAt reports whether the reviewer App has ALREADY submitted THIS verdict
// — same KIND, same resulting state, same body content, pinned to head — on this PR. The
// cross-session idempotency guard (#73): the local audit log is per-HOME,
// so a retry from a fresh reviewer subagent would otherwise post a duplicate that cannot
// be retracted. Matched on the App login AND commit_id AND state AND verdict kind AND
// the normalized body digest, so none of a real verdict CHANGE at the same head, the
// OTHER required verdict of a risk-classed PR (#220), or a DISTINCT
// same-shaped review from a parallel lane (#518) is mistaken for a
// duplicate.
//
// THE BODY-DIGEST ARM (#518). kind+state+head alone is not the identity of a review: the
// desk's own multi-lane dispatch sends SEVERAL reviewers against ONE head, each covering
// a different lane, and two of them can legitimately conclude the same verdict shape
// (both correctness CHANGES_REQUESTED) with entirely different findings. Under the
// digest-less tuple the second lane's body was swallowed as an "idempotent no-op" —
// success-shaped output, no artifact, the exact dropped-artifact failure the #238/#239
// paragraphs below argue against, and one that degrades quietly as lane fan-out grows.
// What distinguishes the two cases is the CONTENT: a true #73 retry re-submits the same
// body-file, so its normalized digest equals the review it already posted; a distinct
// lane's findings do not. The residual trade is deliberate: a re-dispatched reviewer
// that REGENERATES its body text between attempts now posts a second same-kind review —
// a visible, but unretractable, duplicate — because from this side of the API it is
// indistinguishable from a second lane. #518 chooses the visible duplicate over the
// invisible drop, in both guards consistently.
//
// WHERE THE EXISTING BODY'S KIND CANNOT BE READ (#238, #239), this guard
// DOES NOT SUPPRESS — it posts, and says so. #233 made an unreadable body match
// unconditionally (`err != nil → return true`); that is #231's explicitly-rejected
// direction 1 and it fails UNSAFE. Probed against merged main: a `Security-Review: fail`
// arriving behind an unparseable App review at the same head was swallowed —
// postedReview=0, audit noop, exit 0, success-shaped. A dropped `pass` blocks a flip and
// someone notices; a dropped `fail` is a RETRACTION THAT NEVER LANDS, the earlier `pass`
// stays the visible state, and the PR can flip on a verdict its reviewer tried to
// withdraw. That is the exact hole #219 closed at the read path, reopened at the write
// path. #233's stated escape ("recoverable by re-posting with an explicit `Verdict:`
// line") does not exist: this guard reads the EXISTING review's body and never consults
// the re-poster's, so no canonical re-post can clear the suppression.
//
// THE #73 PROTECTION IS NOT WEAKENED BY THIS, and the reason is a property of the write
// path rather than a hope. runReview runs bodycheck.Review AND bodycheck.VerdictKind over
// the incoming body before it ever reaches this guard, and both refuse: a body with no
// verdict line cannot be posted, and a body with both kinds cannot be posted. So every
// review deskpost has EVER posted carries exactly one determinable kind — and therefore
// a #73 fresh-session replay is always caught by the kind+digest arm, never by this one.
// An unreadable App review at head was necessarily written OUT-OF-BAND (the raw
// `gh pr review` fallback of #197), which is a body deskpost could not have produced and
// cannot be replaying.
//
// This is also why the unreadable arm carries no "byte-identical bodies suppress" escape
// hatch (the digest comparison runs only on bodies whose kind PARSES): for the incoming
// body to equal an unreadable one it would have to be unreadable itself, which the write
// path has already refused. Such a branch could never execute, and a guard that can
// never fire is not a protection — it is an unfalsifiable claim.
//
// Not suppressing is not the same as staying quiet: the caller returns a REASON string on
// this path and runReview prints it as a WARNING, so "posted over an unreadable verdict at
// head" is visible rather than inferred.
//
// The historical note that made #233 look necessary still stands and is why the check is
// not simply "parse or refuse". #224 asserted an unparseable body could not exist, on the
// premise that "every review this App posts carries a verdict line, so the App's own
// history parses". That premise is false, and so is the "a human review" case it also
// named:
//
//   - The human case is UNREACHABLE. The loop filters on r.User.Login !=
//     reviewerBotDisplay() BEFORE it ever parses a body, so a human review is skipped and
//     never reaches the kind check.
//   - The App's own history does NOT parse in general. Many of the App's review bodies do
//     NOT yield a kind, and many would be refused by the stricter bodycheck.Review schema
//     gate — i.e. they could not be posted through today's gate at all. Unparseable bodies
//     are a live population, not a frozen tail. Typically the verdict is written as prose
//     rather than on its own line
//     (`**Verdict: CHANGES REQUESTED — solely because CI is red.**`, #1480), which the
//     anchored whole-line verdictLine regexp deliberately does not match.
//
// The #220 fix is preserved exactly where the kind is KNOWABLE: a body that DOES parse to
// a different kind is not a match, so the two required verdicts of a risk-classed PR still
// both land on the fresh-session path.
//
// What does NOT fix the unreadable-body population, measured rather than guessed: relaxing
// verdictLine to match prose recovers only a handful of them, and makes the kind partly
// caller-controllable — a correctness body that merely MENTIONS or blockquotes
// `Security-Review: pass` then classifies as BOTH kinds and is refused outright, breaking
// the documented `> ` quoting escape hatch. Anyone changing this must keep three
// properties: the kind stays non-caller-controllable, a determinable DIFFERENT kind never
// suppresses, and an UNREADABLE body never suppresses a different write.
//
// why is always non-empty when it matters: it is the audit detail on a suppression
// (#238 item 2 — the ledger must distinguish "duplicate" from every other silence, and
// since #518 it names the suppressing review's id and author, so a second reviewer can
// tell "my retry" from "someone else's verdict" without an API call), and the caller's
// WARNING when an unreadable or distinct-bodied review at head was posted over.
//
// wantBodyDigest is the reviewBodyDigest of the INCOMING body — the one deliberate
// departure from "the write path does not consult the incoming body" that #518 requires,
// and it is consulted only to COMPARE, never to classify: the kind still comes from the
// flag/body checks upstream and stays non-caller-controllable.
func appReviewExistsAt(reviews []reviewInfo, head, wantState, wantKind, wantBodyDigest string) (dup bool, why string) {
	unreadable := 0
	distinct := 0
	var distinctID int64
	for _, r := range reviews {
		if !isReviewerBot(r.User.Login) || r.CommitID != head || r.State != wantState {
			continue
		}
		k, err := bodycheck.VerdictKind([]byte(r.Body))
		if err != nil {
			unreadable++ // never a match — see the #238/#239 paragraph above
			continue
		}
		if k != wantKind {
			continue
		}
		if reviewBodyDigest([]byte(r.Body)) == wantBodyDigest {
			return true, fmt.Sprintf(
				"same verdict kind (%s) with an IDENTICAL body already recorded at this head (review id %d by %s)",
				k, r.ID, r.User.Login)
		}
		distinct++ // same shape, different findings — a parallel lane, not a retry (#518)
		if distinctID == 0 {
			distinctID = r.ID
		}
	}
	var parts []string
	if distinct > 0 {
		parts = append(parts, fmt.Sprintf(
			"%d %s %s review(s) by %s at head %s carry a DIFFERENT body (e.g. review id %d). This body "+
				"is being POSTED as a distinct review — a second lane's findings must not be swallowed by "+
				"an earlier lane's same-shaped verdict (#518). If this was meant as a retry, "+
				"the body changed between attempts; check the thread for a duplicate.",
			distinct, wantKind, wantState, reviewerBotDisplay(), short(head), distinctID))
	}
	if unreadable > 0 {
		parts = append(parts, fmt.Sprintf(
			"%d review(s) by %s at head %s carry NO READABLE verdict kind. This %s verdict is being "+
				"POSTED anyway — an unreadable body must not suppress a different verdict, and a "+
				"dropped retraction is worse than a duplicate (#238, #239). If it turns "+
				"out to be a duplicate it cannot be retracted; check the thread.",
			unreadable, reviewerBotDisplay(), short(head), wantKind))
	}
	return false, strings.Join(parts, " ALSO: ")
}

// reviewBodyDigest is the content identity the cross-session guard compares (#518): the
// sha256 of reviewComparableBody(body). It is applied to BOTH sides — the incoming body and
// each recorded one — so a true retry matches the review it posted, while any difference in
// what the caller wrote does not. The reduction is deliberately small and biased toward
// POSTING: a miss here costs a visible duplicate, a false match costs an invisible drop.
func reviewBodyDigest(body []byte) string {
	return deskkit.Sha256Hex([]byte(reviewComparableBody(string(body))))
}

// reviewComparableBody is the part of a review body the caller wrote, in the form both sides
// of the duplicate check are reduced to:
//
//   - CRLF becomes LF;
//   - on-behalf-of lines are removed (deskkit.WithoutOnBehalfOf). The writer appends one to
//     every body it posts and first removes any the caller's body held
//     (deskkit.AppendOnBehalfOf), so NO posted review carries the caller's bytes verbatim.
//     Comparing the caller's bytes with a posted body therefore never matched a review this
//     tool posted, and a retry whose audit row was not in this HOME posted a second one;
//   - surrounding whitespace is trimmed.
//
// Two bodies that differ only in those three ways are the same verdict to this check. It
// compares text as the forge returns it: a forge that rewrote review text in some other way
// would make a retry look distinct, and the retry would be posted with the warning
// appReviewExistsAt writes for that case.
func reviewComparableBody(body string) string {
	s := strings.ReplaceAll(body, "\r\n", "\n")
	return strings.TrimSpace(deskkit.WithoutOnBehalfOf(s))
}

// reviewAlreadyPostedIn is the LOCAL-audit idempotency guard for verdict reviews. It is
// deskkit.AlreadyDoneIn narrowed twice (#518):
//
//   - it additionally matches the entry's recorded BodyDigest, so "same kind + same flag
//     at this head" is no longer treated as "same review". The desk's reviewers share one
//     HOME and therefore ONE audit ledger; under the digest-less key a second lane's
//     DIFFERENT findings at the same head matched the first lane's entry and were
//     swallowed with success-shaped output. The digest here is the RAW body hash — the
//     exact value every review entry has always recorded — because a local retry re-reads
//     the same --body-file, so byte equality is the right identity on this side (the
//     GitHub-state guard, which compares against a body that made a round trip, uses the
//     normalized reviewBodyDigest instead).
//
//   - it counts ResultOK entries ONLY, not noop (a narrowing of the store's usual
//     "ok/noop = done"). An ok entry proves THIS body reached GitHub, so re-noop'ing with
//     zero HTTP is sound. A noop entry proves only that a write was once SUPPRESSED —
//     under the pre-#518 key possibly wrongly — so a body whose only local history is a
//     noop goes forward to the GitHub-state guard and is decided against what is actually
//     on the PR. That re-check costs two reads on a rare path, keeps the true-duplicate
//     answer identical (the digest arm there re-suppresses it), and is what lets a body
//     wrongly swallowed BEFORE this fix land on re-run instead of being re-suppressed by
//     its own noop line.
func reviewAlreadyPostedIn(entries []deskkit.Entry, repo string, pr int, head, verb, bodyDigest string) bool {
	for i := range entries {
		e := &entries[i]
		if e.Repo == repo && e.Verb == verb &&
			e.PR != nil && *e.PR == pr &&
			e.HeadSHA != nil && *e.HeadSHA == head &&
			e.Result == deskkit.ResultOK &&
			e.BodyDigest == bodyDigest {
			return true
		}
	}
	return false
}

// applyMergeHoldForVerdict is `review`'s task-3 half of the forge-gitlab merge-hold brief,
// run AFTER postReview has already landed the verdict itself: an approve releases the
// change's merge-hold at head; a request-changes re-arms it; and a verdict that lands
// against a hold RESOLVED AT A DIFFERENT HEAD re-arms it FIRST (the stale resolution is never
// left standing), before applying its own release or re-arm. GitHub's readMergeHold answers
// the typed not-applicable and this is a no-op there.
//
// A verdict that posted but whose hold write fails returns a non-zero error naming which half
// landed — never a silent success that leaves the server-side gate out of step with the
// verdict that was just recorded.
func applyMergeHoldForVerdict(client postBackend, pr int, event, head string) error {
	hold, err := client.readMergeHold(pr)
	if err != nil {
		if deskkit.IsMergeHoldNotApplicable(err) {
			return nil
		}
		return deskkit.Unverifiable(fmt.Sprintf(
			"the %s verdict posted, but reading PR #%d's merge-hold to release/re-arm it failed: %v — "+
				"the verdict landed; the server-side gate may be out of step with it", event, pr, err), err)
	}
	if hold.State == deskkit.MergeHoldNotApplicable {
		return nil
	}
	if hold.State == deskkit.MergeHoldResolved && hold.Head != "" && hold.Head != head {
		if rerr := client.setMergeHold(pr, deskkit.MergeHoldUpdate{
			Reason: fmt.Sprintf("new head %s", head),
		}); rerr != nil {
			return deskkit.Unverifiable(fmt.Sprintf(
				"the %s verdict posted, but re-arming PR #%d's merge-hold — resolved at %s, now at %s — "+
					"failed: %v — the verdict landed; the server-side gate is still resolved at the STALE head",
				event, pr, short(hold.Head), short(head), rerr), rerr)
		}
	}
	switch strings.ToUpper(event) {
	case "APPROVE":
		if serr := client.setMergeHold(pr, deskkit.MergeHoldUpdate{Resolved: true, Head: head}); serr != nil {
			return deskkit.Unverifiable(fmt.Sprintf(
				"the approve verdict posted on PR #%d, but releasing its merge-hold at %s failed: %v — the "+
					"verdict landed; the server-side gate is still UP", pr, short(head), serr), serr)
		}
	case "REQUEST_CHANGES":
		if serr := client.setMergeHold(pr, deskkit.MergeHoldUpdate{Reason: "request-changes"}); serr != nil {
			return deskkit.Unverifiable(fmt.Sprintf(
				"the request-changes verdict posted on PR #%d, but re-arming its merge-hold failed: %v — the "+
					"verdict landed; the server-side gate may still read released", pr, serr), serr)
		}
	}
	return nil
}
