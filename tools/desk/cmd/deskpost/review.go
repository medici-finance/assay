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
		return reviewShape{flag: "approve", event: "APPROVE", state: "APPROVED"}, true
	case "request-changes":
		return reviewShape{flag: "request-changes", event: "REQUEST_CHANGES", state: "CHANGES_REQUESTED"}, true
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
		// Body validation (verdict schema + secret scan) BEFORE any network — a bad
		// body must refuse with zero side effects.
		if err := bodycheck.Review(body); err != nil {
			return withDigest(fromReadErr(preVerb, repo, pr, "", err), dig)
		}
		// #203: the PUBLIC-REPO SELF-CONTAINMENT scan. A review body is the densest
		// evidence surface the desk writes — it quotes paths, cites issues across repos and
		// names streams — which is precisely why it is also the likeliest to carry a span
		// that resolves only inside the house. No-op on a known-private repo.
		if err := deskkit.SelfContainCheck("review body", body,
			deskkit.SelfContainOpts{Repo: repo, NumberHint: pr}); err != nil {
			return withDigest(fromReadErr(preVerb, repo, pr, "", err), dig)
		}
		// The verdict KIND (correctness vs security) comes from the BODY,
		// not the flag — both kinds post as the same --verdict. It is part of the
		// idempotency key below; VerdictKind fails closed rather than yielding a key that
		// merges the two (#220).
		kind, kerr := bodycheck.VerdictKind(body)
		if kerr != nil {
			return withDigest(fromReadErr(preVerb, repo, pr, "", kerr), dig)
		}
		// The security verb declares BOTH the kind and the marker its body must carry, and
		// refuses a body that disagrees — before any network call. `review` declares
		// neither, so a correctness body and a security body both still go out through it
		// exactly as before.
		//
		// This is not decoration. `security-review --verdict pass` submits the COMMENT
		// event; handed a `Security-Review: fail` body it would post a RETRACTION as a
		// review that blocks nothing on GitHub's side. Gate (e0) would still read the fail
		// and block the flip, so nothing fails open — but the artifact would misrepresent
		// itself to every human reading the thread, and a refusal costs one exit 5 while a
		// submitted review cannot be retracted.
		if shape.wantKind != "" {
			if kind != shape.wantKind {
				return withDigest(fromReadErr(preVerb, repo, pr, "", deskkit.Refused(fmt.Sprintf(
					"refused: `security-review` posts the SECURITY verdict, but this body carries a "+
						"%s verdict line — post a correctness verdict with `deskpost review --verdict approve|request-changes`",
					kind))), dig)
			}
			if got := classifySecurityBody(string(body)); got != shape.wantSec {
				return withDigest(fromReadErr(preVerb, repo, pr, "", deskkit.Refused(fmt.Sprintf(
					"refused: --verdict %s does not match the body, which reads as %s — the flag and the "+
						"'Security-Review:' line must agree, or the posted artifact misstates its own verdict",
					verdictFlag, secVerdictName(got)))), dig)
			}
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

		client, err := newGHClient(owner, name)
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
		events, ferr := client.listLabelEvents(pr)
		if ferr != nil {
			return withDigest(fromReadErr(verb, repo, pr, curHead, ferr), dig)
		}
		fd := deskkit.ModelCapabilityFloor(events, deskkit.IsDispatcherLogin, deskkit.ModelFloorOverrideEngaged())
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
		if dup {
			// #238(2): the audit line records WHAT was suppressed and WHY, so a
			// duplicate-suppression is distinguishable in the ledger from every other
			// reason a write did not happen. The why names the suppressing review's id
			// and author (#518 direction 2), so a caller can tell "my retry" from
			// "someone else's verdict" without another API call.
			return withDigest(noop(verb, repo, pr, curHead,
				"equivalent "+verb+" by "+reviewerBotDisplay()+" already present at "+short(curHead)+
					" (idempotent no-op; "+why+")"), dig)
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
		// Trust gate: no verdict on unvetted third-party work (exit 5, audited).
		if terr := prTrustGate(client, pr, info.User.Login, info.User.ID); terr != nil {
			return withDigest(fromReadErr(verb, repo, pr, head, terr), dig)
		}
		// Public-repo gate: refuse to write to a public repo
		// without a qualifying +1 from an authorized human.
		if gerr := deskkit.PublicRepoGate(client, owner, name, pr); gerr != nil {
			return withDigest(fromErr(verb, repo, pr, head, gerr), dig)
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
		if err := client.postReview(pr, head, shape.event, string(body)); err != nil {
			return withDigest(fromErr(verb, repo, pr, head, err), dig)
		}
		// Mechanical, ADVISORY verdict-time labels (diff size class + surface tier) for
		// merge-queue triage. This runs AFTER the verdict has landed and NEVER changes its
		// outcome: a labeling failure is logged as a WARNING and swallowed, so the verdict
		// still reports success. Labels gate nothing (they are a `wc -l` + glob triage aid),
		// and a could-not-classify family is skipped, never guessed.
		if lo, lerr := applyVerdictLabels(client, pr, info.ChangedFiles); lerr != nil {
			fmt.Fprintln(stderr, "deskpost: WARNING: verdict-time labeling (advisory): "+lerr.Error())
		} else if s := lo.String(); s != "no label change" {
			fmt.Fprintln(stderr, "deskpost: verdict-time labels: "+s)
		}
		return done(verb, repo, pr, head, dig, "posted "+verdictFlag+" review as "+reviewerBotDisplay()+" at "+short(head))
	})
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
// sha256 of the body with CRLF normalized to LF and trailing newlines dropped — the only
// transforms a body plausibly picks up on the GitHub round trip. A true retry therefore
// still matches the review it posted, while any substantive difference does not. The
// normalization is deliberately minimal and biased toward POSTING: a miss here costs a
// visible duplicate, a false match costs an invisible drop.
func reviewBodyDigest(body []byte) string {
	s := strings.ReplaceAll(string(body), "\r\n", "\n")
	s = strings.TrimRight(s, "\n")
	return deskkit.Sha256Hex([]byte(s))
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
