package main

import (
	"fmt"

	"github.com/medici-finance/assay/tools/desk/cmd/deskpost/internal/bodycheck"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// runComment posts a plain comment AS THE REVIEWER APP on EITHER a pull request or an
// issue (#296) — the number is resolved to its object kind by resolveTarget,
// never assumed. Comments get the size cap and the shared secret scan only, on both kinds
// — no verdict schema.
//
// Idempotency keys on (repo, number, head, verb). A PR keeps the pre-#296 key exactly —
// verb "comment:<digest>", head = the PR head — so existing audit history still suppresses
// a repeat. An issue has no head, so its key is (repo, number, verb) with the kind carried
// in the verb ("comment:issue:<digest>"); the two verbs never have to be told apart by the
// head field alone. Two DIFFERENT comments on one object are both legitimate; an identical
// re-post is a no-op.
//
// wantHead, when non-empty, is the caller's assertion of the PR head the comment was
// written against; it is verified against the live head and refuses on a mismatch
// (#513). Empty means "no assertion" — the pre-existing behaviour, which
// stamps whatever head is live at post time.
func runComment(owner, name string, num int, wantHead string, body []byte, args []string, opts postOpts) int {
	repo := owner + "/" + name
	dig := deskkit.Sha256Hex(body)
	// preVerb labels audit lines for refusals raised BEFORE the object kind is known
	// (bad repo, bad body, unresolvable number). It is byte-identical to the PR verb, so
	// the PR idempotency key is unchanged from before #296; it is never an idempotency
	// key in its own right (AlreadyDoneIn matches ok/noop entries only, and every ok/noop
	// path below is reached after the kind is resolved).
	preVerb := "comment:" + dig

	// repo + num are the budget scope, known from argv with no network call — so the gate
	// runs upstream of the POST below rather than after it (writeflow.go, runOutward).
	return runOutward(args, opts, repo, num, func(entries []deskkit.Entry, opts postOpts) writeResult {
		if !deskkit.IsAllowedRepo(repo) {
			return refused(preVerb, repo, num, "", "repo "+repo+" is not in the fixed desk repo set")
		}
		// Size + secret scan BEFORE any network — a bad body must refuse with zero side
		// effects. This runs identically for both kinds: the checks are a
		// property of the BODY, and nothing about targeting an issue relaxes them.
		if err := bodycheck.Comment(body); err != nil {
			return withDigest(fromReadErr(preVerb, repo, num, "", err), dig)
		}

		client, err := newGHClient(owner, name)
		if err != nil {
			return withDigest(fromReadErr(preVerb, repo, num, "", err), dig)
		}
		tgt, err := resolveTarget(client, num)
		if err != nil {
			return withDigest(fromReadErr(preVerb, repo, num, "", err), dig)
		}

		verb := preVerb
		if tgt.kind == kindIssue {
			verb = "comment:issue:" + dig
		}
		// The head assertion runs BEFORE the idempotency read, so a stale --head can never
		// be answered with a success-shaped no-op: "an identical comment is already there"
		// is not an answer to "the commit you reviewed is no longer the head".
		if wantHead != "" {
			if tgt.kind == kindIssue {
				return withDigest(refused(verb, repo, num, "",
					fmt.Sprintf("--head was given but #%d is an ISSUE, which has no head — drop the flag", num)), dig)
			}
			if tgt.head != wantHead {
				return withDigest(refused(verb, repo, num, tgt.head, fmt.Sprintf(
					"--head %s != current head %s — the comment was written against a commit that is no longer "+
						"the head; re-read the PR at %s before posting", short(wantHead), short(tgt.head), short(tgt.head))), dig)
			}
		}
		if alreadyCommented(entries, repo, tgt, verb) {
			return withDigest(noop(verb, repo, num, tgt.head,
				fmt.Sprintf("identical comment already posted on %s (idempotent no-op)", describe(tgt))), dig)
		}
		// Trust gate — ISSUE comments only (exit 5, audited). The gate quarantines desk
		// writes on unvetted third-party TEXT until the blessing authority admits it, and an
		// issue in a PUBLIC repo is exactly that surface: any external user can open one and
		// fill its body with arbitrary prose the desk must not engage until it is blessed.
		//
		// A PR comment is deliberately EXEMPT from the author-trust/bless gate. A plain
		// comment carries no verdict and flips no state — unlike `deskpost review`
		// (approve/request-changes) and `deskpost ready`, which STAY gated on the very same
		// predicate via prTrustGate (review.go, ready.go) — so refusing informational review
		// feedback on an untrusted-author PR (a Dependabot bump, say) until a human blesses it
		// is friction with no security value (#943 protects the verdict/flip, not a comment).
		//
		// The loosening is scoped to the author-trust/bless dimension ONLY. Every OTHER
		// comment-path protection still runs on a PR comment: the size cap + body
		// secret/impersonation scan (bodycheck.Comment, above) and the public-repo +1 gate
		// (PublicRepoGate, below). A PR comment therefore still cannot carry a
		// Verdict:/Security-Review: line — bodycheck keeps those verdict-class.
		if tgt.kind == kindIssue {
			if terr := trustGate(client, tgt.kind, num, tgt.authorLogin, tgt.authorID); terr != nil {
				return withDigest(fromReadErr(verb, repo, num, tgt.head, terr), dig)
			}
		}
		// Public-repo gate: refuse to write to a public repo
		// without a qualifying +1 from an authorized human.
		if gerr := deskkit.PublicRepoGate(client, owner, name, num); gerr != nil {
			return withDigest(fromErr(verb, repo, num, tgt.head, gerr), dig)
		}
		if opts.dryRun {
			return withDigest(dryRun(verb, repo, num, tgt.head,
				"DRY RUN: comment body passed the size cap and secret scan, repo is in the desk set, "+
					"author-trust gate cleared (enforced on issue comments only), public-repo gate passed, "+
					"no identical comment on "+describe(tgt)+" — stopped before POST"), dig)
		}
		if err := client.postComment(num, string(body)); err != nil {
			return withDigest(fromErr(verb, repo, num, tgt.head, err), dig)
		}
		return done(verb, repo, num, tgt.head, dig,
			"posted comment as "+reviewerBotDisplay()+" on "+describe(tgt))
	})
}

// describe renders the target for the human-readable detail line (which also lands in the
// audit record), so a reader can tell at a glance WHICH object kind a write went to.
func describe(t *target) string {
	if t.kind == kindIssue {
		return fmt.Sprintf("issue #%d", t.number)
	}
	return fmt.Sprintf("PR #%d at %s", t.number, short(t.head))
}

// alreadyCommented is the idempotency read for both kinds. A PR delegates to deskkit's
// AlreadyDoneIn unchanged. An ISSUE cannot: AlreadyDoneIn requires a non-nil head_sha on
// the stored entry, and an issue write has no head to record — so the match here is on
// (repo, number, verb) plus "the entry recorded no head", which is exactly the shape an
// issue comment writes. It stays fail-closed in the same direction as AlreadyDoneIn: only
// ok/noop entries count, so a prior REFUSAL never suppresses a legitimate retry.
func alreadyCommented(entries []deskkit.Entry, repo string, t *target, verb string) bool {
	if t.kind == kindPR {
		return deskkit.AlreadyDoneIn(entries, repo, t.number, t.head, verb)
	}
	for i := range entries {
		e := &entries[i]
		if e.Repo == repo && e.Verb == verb &&
			e.PR != nil && *e.PR == t.number &&
			(e.HeadSHA == nil || *e.HeadSHA == "") &&
			(e.Result == deskkit.ResultOK || e.Result == deskkit.ResultNoop) {
			return true
		}
	}
	return false
}
