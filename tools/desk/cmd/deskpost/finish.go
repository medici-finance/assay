package main

// finish.go — `deskpost finish`: the ONE command a reviewer runs when its verdict is written.
//
//	deskpost finish review          <owner/repo> <pr> --verdict approve|request-changes --head <full sha> --body-file F --claim <claim key>
//	deskpost finish security-review <owner/repo> <pr> --verdict pass|fail              --head <full sha> --body-file F --claim <claim key>
//
// A reviewer's last three acts were three commands — post the verdict, look that it landed,
// release the dispatch claim — and a reviewer that stopped after the first left a claim held
// on a change nobody was reading any more. `finish` does the three in order and stops at the
// first one that does not succeed:
//
//  1. POST. It calls runReview / runSecurityReview — the functions `deskpost review` and
//     `deskpost security-review` call — with the arguments those verbs parse. There is no
//     second post path here: every gate, the identity, the budget, the audit row and every
//     refusal are theirs, in their order, with their text and their exit code. `finish`
//     refuses exactly where they refuse because it IS them for this step.
//  2. CONFIRM. It reads the change's reviews back and requires one by the reviewer identity,
//     at the reviewed head, in the state this verdict produces, of this verdict kind, with
//     this body. The match is appReviewExistsAt — the predicate the post path's own duplicate
//     guard uses — applied to one review at a time.
//  3. RELEASE. It releases the dispatch claim named by --claim.
//  4. One result line on stdout.
//
// THE ORDER IS LOAD-BEARING. The post path validates the reviewer's dispatch stamp, and that
// stamp stops counting once the review claim is released (claimliveness.go). Releasing first
// would make the tool refuse its own verdict; so the claim is released LAST, and only after
// the verdict has been read back.
//
// WHAT A STOP MEANS. Each stop names the step, says what did and did not happen, and leaves
// everything after it untouched: a refused post confirms and releases nothing; an unconfirmed
// post releases nothing; a failed release leaves a posted, confirmed verdict and a held claim.
// Re-running the same command is safe at every stop — a verdict this session already posted
// is not posted twice (the post path's own duplicate guard), and releasing a claim that is
// already gone is reported as such, not as a failure.
//
// THE CLAIM KEY IS CHECKED BEFORE ANYTHING IS WRITTEN. --claim must be a review-dispatch claim
// key of THIS change (deskkit.ValidateReviewClaimKey, the check review dispatch itself
// applies), so this verb cannot be pointed at another change's claim or at any other ref. A
// key that fails it is an argument error (exit 2), like an abbreviated --head.
//
// ONE AUDIT ROW. The row is the post step's, as for the plain verdict verbs; its argument
// digest is this invocation's. Confirm is a read. The release writes no row — the claim tool,
// which it stands in for, writes none either — and it spends nothing from the write budget.

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	finishClaimReleased        = "released"
	finishClaimAlreadyReleased = "already-released"
)

// finishLane is one of the two verdict verbs `finish` can end with.
type finishLane struct {
	verb          string // "review" | "security-review" — the verb whose post path runs
	verdictValues string
	shapeFor      func(verdictFlag string) (reviewShape, bool)
	post          func(owner, name string, pr int, verdictFlag, head string, body []byte, args []string, opts postOpts) int
	// recipeOnRateLimit mirrors cmdReview / cmdSecurityReview: only the correctness verb
	// prints the raw-command fallback on exit 4 (see cmdSecurityReview for why not both).
	recipeOnRateLimit bool
}

func finishLaneFor(verb string) (finishLane, bool) {
	switch verb {
	case "review":
		return finishLane{verb: verb, verdictValues: "approve|request-changes",
			shapeFor: correctnessShapeFor, post: runReview, recipeOnRateLimit: true}, true
	case "security-review":
		return finishLane{verb: verb, verdictValues: "pass|fail",
			shapeFor: securityShapeFor, post: runSecurityReview}, true
	}
	return finishLane{}, false
}

// releaseReviewClaimFn is a seam so a test can fail the release without a forge.
var releaseReviewClaimFn = releaseReviewClaim

func cmdFinish(argv []string) int {
	if len(argv) < 2 {
		fmt.Fprintln(stderr, "usage: deskpost finish review|security-review <owner/repo> <pr> --verdict <…> --head <full-40-or-64-char-sha> --body-file F --claim <claim key>")
		return 2
	}
	lane, ok := finishLaneFor(argv[1])
	if !ok {
		fmt.Fprintf(stderr, "deskpost finish: the first argument names the verdict to post — `review` or `security-review` — got %q\n", argv[1])
		return 2
	}

	// The SAME parse the verdict verb runs, plus --claim. argv[1:] is exactly what that verb
	// is handed, so the positionals, the three required flags, the full-sha rule and the
	// shared modifiers are parsed once, in one place.
	var claim *string
	a, code, ok := parseVerdictArgs(lane.verb, lane.verdictValues, argv[1:], func(fs *flag.FlagSet) {
		claim = fs.String("claim", "", "the dispatch claim key to release once the verdict is confirmed (required)")
	})
	if !ok {
		if code == 2 {
			fmt.Fprintf(stderr, "deskpost finish: `finish %s` takes `deskpost %s`'s arguments plus --claim <claim key>\n", lane.verb, lane.verb)
		}
		return code
	}
	repo := a.owner + "/" + a.name
	key := strings.TrimSpace(*claim)
	if key == "" {
		fmt.Fprintln(stderr, "deskpost finish: --claim <claim key> is required — it is the dispatch claim this command releases once the verdict is confirmed (the key your assignment names). Nothing was posted")
		return 2
	}
	if err := deskkit.ValidateReviewClaimKey(key, repo, a.pr); err != nil {
		// An argument that does not name this change's claim is a FORM error, like an
		// abbreviated --head: exit 2, before any check that writes an audit row.
		fmt.Fprintf(stderr, "deskpost finish: --claim: %s. Nothing was posted\n", err.Error())
		return 2
	}
	shape, ok := lane.shapeFor(a.verdict)
	if !ok {
		// Hand the bad value to the verb's own writer so the message and exit code are its.
		return lane.post(a.owner, a.name, a.pr, a.verdict, a.head, a.body, argv, a.opts)
	}

	// Step 1 — post, through the verdict verb's own function. Its success line is sent to
	// stderr for this call so that stdout carries exactly ONE line: the result line below.
	code = finishPost(lane, a, argv)
	if code == deskkit.ExitRateLimited && lane.recipeOnRateLimit {
		fmt.Fprint(stderr, fallbackRecipe(a.owner, a.name, a.pr, a.verdict, a.head, a.bodyFile))
	}
	if code != deskkit.ExitOK {
		fmt.Fprintf(stderr, "deskpost finish: STOPPED at step 1 of 3 (post) — `deskpost %s` exited %d; its message is above. "+
			"Nothing was confirmed and the claim %s is still held. Fix what the message names and run this command again\n",
			lane.verb, code, key)
		return code
	}
	if a.opts.dryRun {
		fmt.Fprintf(stderr, "deskpost finish: dry run — the post step was rehearsed and nothing was posted, so nothing was confirmed and the claim %s is still held\n", key)
		return deskkit.ExitOK
	}

	// Step 2 — confirm the verdict is recorded at the reviewed head.
	posted, err := finishConfirm(a, repo, shape)
	if err != nil {
		fmt.Fprintf(stderr, "deskpost finish: STOPPED at step 2 of 3 (confirm) — %s. The claim %s was NOT released. "+
			"Look at the change: if the verdict is there, release the claim with the release command in your assignment; "+
			"if it is not, run this command again\n", err.Error(), key)
		return deskkit.ExitUnverifiable
	}

	// Step 3 — release the dispatch claim.
	state, err := releaseReviewClaimFn(a.owner, a.name, key)
	if err != nil {
		fmt.Fprintf(stderr, "deskpost finish: STOPPED at step 3 of 3 (release) — the verdict IS posted and confirmed (%s, %s at %s), "+
			"but the claim %s is STILL HELD: %s. Run this command again, or release the claim with the release command in your assignment\n",
			finishReviewRef(repo, a.pr, posted), posted.State, a.head, key, firstLineOf(err.Error()))
		if c := deskkit.ExitCodeOf(err); c != deskkit.ExitOK {
			return c
		}
		return deskkit.ExitUnverifiable
	}

	fmt.Fprintf(stdout, "deskpost finish: review=%s state=%s head=%s claim=%s\n",
		finishReviewRef(repo, a.pr, posted), posted.State, a.head, state)
	return deskkit.ExitOK
}

// finishPost runs the verdict verb's own post function with stdout pointed at stderr, so the
// post path's own "posted …" line is still shown but does not share stdout with the result
// line. args is the WHOLE finish invocation: it is what the audit row's digest records.
func finishPost(lane finishLane, a verdictArgs, args []string) int {
	saved := stdout
	stdout = stderr
	defer func() { stdout = saved }()
	return lane.post(a.owner, a.name, a.pr, a.verdict, a.head, a.body, args, a.opts)
}

// finishConfirm reads the change's reviews and returns the one that IS this verdict: by the
// reviewer identity, at head, in shape's state, of shape's kind, with this body.
//
// The predicate is appReviewExistsAt, asked about one review at a time so the caller learns
// WHICH review matched. The one thing added is that on-behalf-of lines are taken off both
// sides before the comparison: the writer appends one to every body it posts and removes any
// the caller's body held (deskkit.AppendOnBehalfOf), so no posted review carries the caller's
// bytes verbatim. deskkit.WithoutOnBehalfOf is that same removal, so what is compared is what
// the caller wrote. Nothing is rendered here and no principal is resolved.
func finishConfirm(a verdictArgs, repo string, shape reviewShape) (reviewInfo, error) {
	want := reviewBodyDigest([]byte(finishComparableBody(string(a.body))))

	client, err := newPostBackend(a.owner, a.name)
	if err != nil {
		return reviewInfo{}, fmt.Errorf("the reviews of %s#%d could not be read back: %s", repo, a.pr, firstLineOf(err.Error()))
	}
	reviews, err := client.listReviews(a.pr)
	if err != nil {
		return reviewInfo{}, fmt.Errorf("the reviews of %s#%d could not be read back: %s", repo, a.pr, firstLineOf(err.Error()))
	}
	// Newest first: a forge lists reviews oldest first, and the one just posted is the last.
	for i := len(reviews) - 1; i >= 0; i-- {
		probe := reviews[i]
		probe.Body = finishComparableBody(probe.Body)
		if dup, _ := appReviewExistsAt([]reviewInfo{probe}, a.head, shape.state, shape.wantKind, want); dup {
			return reviews[i], nil
		}
	}
	return reviewInfo{}, fmt.Errorf("no %s review by %s with this body is recorded on %s#%d at head %s (%d review(s) read)",
		shape.state, reviewerBotDisplay(), repo, a.pr, a.head, len(reviews))
}

// finishComparableBody is a review body with its on-behalf-of lines and the surrounding
// whitespace removed — the part of a posted body the caller wrote.
func finishComparableBody(body string) string {
	s := strings.ReplaceAll(body, "\r\n", "\n")
	return strings.TrimSpace(deskkit.WithoutOnBehalfOf(s))
}

// finishReviewRef names the confirmed review for a human: the forge's own link when the
// listing carries one, otherwise the change and the review id. It never builds a link.
func finishReviewRef(repo string, pr int, r reviewInfo) string {
	if u := strings.TrimSpace(r.HTMLURL); u != "" && !strings.ContainsAny(u, " \t\r\n") {
		return u
	}
	return fmt.Sprintf("%s#%d/review-%d", repo, pr, r.ID)
}

// releaseReviewClaim releases the review-dispatch claim key on owner/name and reports
// "released" or "already-released". key has already passed ValidateReviewClaimKey.
//
// WHERE THE CLAIM IS. The store is resolved the way every claim writer resolves it
// (deskkit.ResolveClaimStore) and a configured store is released through its own Remove — the
// configured store is never swapped for another one. With no store configured the claim is a
// ref on the forge under deskkit.DispatchClaimActiveRefsPrefix — the namespace the claim tool
// writes and the one this tool's own liveness read looks in (claimliveness.go) — and it is
// deleted through the reviewer identity's typed forge operation, which validates the ref path
// before any request exists.
//
// "ALREADY GONE" IS VERIFIED, NEVER ASSUMED. A delete the forge answers with not-found (or
// with unprocessable, which is how some forges answer a delete of a missing ref) is followed
// by a read of that one ref: only a positive "absent" is reported as already-released. A
// forbidden, a server error or an unreadable ref is a failure — the claim may still be held.
func releaseReviewClaim(owner, name, key string) (string, error) {
	repo := owner + "/" + name
	res, err := deskkit.ResolveClaimStore(repo)
	if err != nil {
		return "", err
	}
	if res.Store != nil {
		outcome, existed := res.Store.Remove(key)
		if outcome != deskkit.ClaimWriteApplied {
			return "", deskkit.Unverifiable(fmt.Sprintf("the %s claim store did not release %s (%s)",
				res.Label(), key, res.Store.TransportCause()), nil)
		}
		if !existed {
			return finishClaimAlreadyReleased, nil
		}
		return finishClaimReleased, nil
	}
	if !res.Legacy {
		return "", deskkit.Unverifiable(fmt.Sprintf("the %s claim store is configured for %s but this tool was given no handle to it",
			res.Label(), repo), nil)
	}

	fr := deskkit.ForgeRepo{Owner: owner, Name: name}
	fg, err := forgeForReviewer(fr)
	if err != nil {
		return "", err
	}
	ref := strings.TrimPrefix(deskkit.DispatchClaimActiveRefsPrefix, "refs/") + key
	derr := fg.DeleteRef(fr, ref)
	if derr == nil {
		return finishClaimReleased, nil
	}
	var ae *deskkit.ForgeAPIError
	if errors.As(derr, &ae) && (ae.Status == http.StatusNotFound || ae.Status == http.StatusUnprocessableEntity) {
		exists, rerr := fg.RefExists(fr, ref)
		if rerr == nil && !exists {
			return finishClaimAlreadyReleased, nil
		}
		if rerr != nil {
			return "", deskkit.Unverifiable(fmt.Sprintf("the forge would not delete the claim (HTTP %d) and whether it still exists could not be read", ae.Status), rerr)
		}
		return "", deskkit.Unverifiable(fmt.Sprintf("the forge would not delete the claim (HTTP %d) and it still exists", ae.Status), derr)
	}
	return "", derr
}

// firstLineOf is the first line of a possibly multi-line message, for a one-line stop.
func firstLineOf(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
