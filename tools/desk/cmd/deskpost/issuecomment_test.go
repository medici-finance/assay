package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// `deskpost comment` on an ISSUE (#296). Before this, the number was read
// through GET /pulls/{n} alone: an issue number 404'd and the tool exited 6, so every
// register update on an issue went out through a raw `gh issue comment` — outside the
// write budget, the audit line, the kill switch and the body checks.
//
// These tests pin BOTH halves: the issue path now works end to end, and it works under
// the SAME controls the PR path has always had (repo gate, body checks, trust gate,
// rate limit, idempotency, one audit line).

const issueNum = "21" // a number the fake serves as an ISSUE, not a PR

// issueOnly marks the fixture number as an issue and returns the `comment` argv for it.
func issueOnly(t *testing.T, f *fakeGH, repo, bodyFile string) []string {
	t.Helper()
	f.issueNums[21] = true
	return commentArgs(repo, issueNum, bodyFile)
}

// TestCommentOnIssuePosts is the headline fix: an issue number resolves to an issue and
// the comment lands, exit 0, audited ok.
func TestCommentOnIssuePosts(t *testing.T) {
	f, _ := setupFake(t)
	const issueBody = "Still open: the embargoed fix is unreleased as of today."
	bf := writeBody(t, "i.md", issueBody)
	code := run(issueOnly(t, f, "example-org/org-slides", bf))
	if code != 0 {
		t.Fatalf("comment on issue exit = %d, want 0", code)
	}
	if f.postedCmt != 1 {
		t.Fatalf("postedCmt = %d, want 1", f.postedCmt)
	}
	e := lastAudit(t)
	if e.Result != deskkit.ResultOK {
		t.Fatalf("audit result = %q, want ok", e.Result)
	}
	if e.BodyDigest == "" {
		t.Fatal("audit line carries no body digest")
	}
	// The EXACT issue verb, recomputed — not a prefix. From the moment this lands the desk
	// starts writing `comment:issue:<digest>` entries, so the stake is identical to the PR
	// key's: change these bytes later and suppression stops matching the audit history on
	// disk, and the desk double-posts. A prefix assertion passes for `comment:issue:v2:…`.
	wantVerb := "comment:issue:" + deskkit.Sha256Hex([]byte(issueBody))
	if e.Verb != wantVerb {
		t.Fatalf("audit verb = %q, want %q", e.Verb, wantVerb)
	}
	if e.PR == nil || *e.PR != 21 {
		t.Fatalf("audit number = %v, want 21", e.PR)
	}
}

// TestCommentOnIssueRepeatNoNewPost — idempotency holds WITHOUT a head SHA to key on.
// The issue key is (repo, number, verb) over head-less entries; a second identical post
// must be a no-op, not a duplicate comment.
func TestCommentOnIssueRepeatNoNewPost(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "i.md", "same issue comment body")
	args := issueOnly(t, f, "example-org/org-slides", bf)

	if code := run(args); code != 0 {
		t.Fatalf("first issue comment exit = %d", code)
	}
	if code := run(args); code != 0 {
		t.Fatalf("repeat issue comment exit = %d, want 0 (noop)", code)
	}
	if f.postedCmt != 1 {
		t.Fatalf("postedCmt = %d, want 1 (repeat must not POST again)", f.postedCmt)
	}
	if lastAudit(t).Result != deskkit.ResultNoop {
		t.Fatal("repeat should audit result=noop")
	}
}

// TestCommentOnIssueDifferentBodyPosts — the flip side: idempotency must not swallow a
// DIFFERENT comment on the same issue (the digest is in the verb).
func TestCommentOnIssueDifferentBodyPosts(t *testing.T) {
	f, _ := setupFake(t)
	first := writeBody(t, "i1.md", "status: still reproducing")
	second := writeBody(t, "i2.md", "status: fixed on main, closing when released")

	if code := run(issueOnly(t, f, "example-org/org-slides", first)); code != 0 {
		t.Fatalf("first exit = %d", code)
	}
	if code := run(issueOnly(t, f, "example-org/org-slides", second)); code != 0 {
		t.Fatalf("second exit = %d", code)
	}
	if f.postedCmt != 2 {
		t.Fatalf("postedCmt = %d, want 2 (a different body is a different write)", f.postedCmt)
	}
}

// TestCommentOnIssueBadBodyExit5NoNetwork — the body checks are NOT relaxed for issues:
// a secret-carrying body refuses with zero network calls, exactly as on a PR.
func TestCommentOnIssueBadBodyExit5NoNetwork(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "bad.md", "ghp_"+repeat("A", 40))
	code := run(issueOnly(t, f, "example-org/org-slides", bf))
	if code != deskkit.ExitRefused {
		t.Fatalf("bad-body exit = %d, want 5", code)
	}
	if len(f.hits) != 0 {
		t.Fatalf("expected ZERO network hits, got %v", f.hits)
	}
	if f.postedCmt != 0 {
		t.Fatal("no comment on a refusal")
	}
}

// TestCommentOnIssueRepoGateExit5 — the repo gate still fires first, before any
// resolution or network call.
func TestCommentOnIssueRepoGateExit5(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "i.md", "a comment")
	code := run(issueOnly(t, f, "some-org/not-in-the-set", bf))
	if code != deskkit.ExitRefused {
		t.Fatalf("out-of-set repo exit = %d, want 5", code)
	}
	if len(f.hits) != 0 {
		t.Fatalf("expected ZERO network hits, got %v", f.hits)
	}
}

// TestCommentOnIssueUntrustedAuthorExit5 — the trust gate covers issues too. A public
// repo's issues are exactly the surface it exists for: anyone can open one.
func TestCommentOnIssueUntrustedAuthorExit5(t *testing.T) {
	f, _ := setupFake(t)
	f.issueAuthor = "external-user"
	f.issueAuthorID = 424242
	bf := writeBody(t, "i.md", "a comment")

	code := run(issueOnly(t, f, "example-org/org-slides", bf))
	if code != deskkit.ExitRefused {
		t.Fatalf("comment on untrusted-author issue = exit %d, want 5", code)
	}
	if f.postedCmt != 0 {
		t.Fatal("no comment may be posted on an untrusted, unblessed issue")
	}
	if e := lastAudit(t); e.Result != deskkit.ResultRefused {
		t.Fatalf("audit result = %q, want refused", e.Result)
	}
}

// adaBlessedIssuePayload is an IssueTrustQuery response with one ada comment
// (correct numeric databaseId) and no later untrusted content.
const adaBlessedIssuePayload = `{"data":{"repository":{"issue":{"lastEditedAt":null,` +
	`"comments":{"pageInfo":{"hasNextPage":false},"nodes":[` +
	`{"createdAt":"2026-07-21T10:00:00Z","lastEditedAt":null,"author":{"login":"ada","__typename":"User","databaseId":2001}}]}}}}}`

// blessThenEditIssuePayload: ada blessed at 07-21, the issue BODY was edited at 07-22
// — the blessing is void, on an issue exactly as on a PR.
const blessThenEditIssuePayload = `{"data":{"repository":{"issue":{"lastEditedAt":"2026-07-22T10:00:00Z",` +
	`"comments":{"pageInfo":{"hasNextPage":false},"nodes":[` +
	`{"createdAt":"2026-07-21T10:00:00Z","lastEditedAt":null,"author":{"login":"ada","__typename":"User","databaseId":2001}}]}}}}}`

func TestCommentOnIssueAdaBlessedPosts(t *testing.T) {
	f, _ := setupFake(t)
	f.issueAuthor = "external-user"
	f.issueAuthorID = 424242
	f.issueTrustJSON = adaBlessedIssuePayload
	bf := writeBody(t, "i.md", "a comment")

	code := run(issueOnly(t, f, "example-org/org-slides", bf))
	if code != 0 {
		t.Fatalf("comment on ada-blessed issue = exit %d, want 0", code)
	}
	if f.postedCmt != 1 {
		t.Fatalf("postedCmt = %d, want 1", f.postedCmt)
	}
}

func TestCommentOnIssueBlessThenEditExit5(t *testing.T) {
	f, _ := setupFake(t)
	f.issueAuthor = "external-user"
	f.issueAuthorID = 424242
	f.issueTrustJSON = blessThenEditIssuePayload
	bf := writeBody(t, "i.md", "a comment")

	code := run(issueOnly(t, f, "example-org/org-slides", bf))
	if code != deskkit.ExitRefused {
		t.Fatalf("comment on bless-then-edited issue = exit %d, want 5", code)
	}
	if f.postedCmt != 0 {
		t.Fatal("no comment after the blessed issue body was edited")
	}
}

// TestCommentOnIssueRateLimitExit4 — the budget covers the issue path (the whole point:
// register updates used to bypass it).
func TestCommentOnIssueRateLimitExit4(t *testing.T) {
	f, _ := setupFake(t)
	n := 21
	for i := 0; i < deskkit.RateLimitPerPRPerHour; i++ {
		if err := deskkit.Log(deskkit.Entry{
			Tool: toolName, Verb: "comment:seed", Repo: "example-org/org-slides",
			PR: &n, Result: deskkit.ResultOK,
		}); err != nil {
			t.Fatalf("seed audit: %v", err)
		}
	}
	bf := writeBody(t, "i.md", "should be rate limited")
	code := run(issueOnly(t, f, "example-org/org-slides", bf))
	if code != deskkit.ExitRateLimited {
		t.Fatalf("rate-limit exit = %d, want 4", code)
	}
	if f.postedCmt != 0 {
		t.Fatal("no comment posted when rate limited")
	}
}

// TestCommentOnAbsentNumberUnverifiable — a number that is NEITHER a PR nor an issue is
// still exit 6, and the message says so: the fix must not turn "no such object" into a
// silent success or a misleading refusal.
func TestCommentOnAbsentNumberUnverifiable(t *testing.T) {
	f, _ := setupFake(t)
	f.missingNums[404] = true
	bf := writeBody(t, "i.md", "a comment")

	code := run(commentArgs("example-org/org-slides", "404", bf))
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("absent number exit = %d, want 6", code)
	}
	if f.postedCmt != 0 {
		t.Fatal("nothing may be posted for a number that resolves to no object")
	}
	// The message lands in the audit line's detail (finishAudit writes the human line to
	// the process stderr, which the harness does not capture).
	if d := lastAudit(t).Detail; !strings.Contains(d, "neither a pull request nor an issue") {
		t.Fatalf("detail does not name the real problem: %q", d)
	}
}

// TestReviewOnIssueRefusesNamingComment / TestReadyOnIssueRefusesNamingComment — the
// PR-only verbs no longer report a wrong-kind number as unverifiable (exit 6, "the write
// may or may not have landed"). They refuse (exit 5) and name the verb that works. The
// distinction is load-bearing for a caller following the exit-code contract, and a
// refusal does not charge the outward-write budget where an unverifiable does.
func TestReviewOnIssueRefusesNamingComment(t *testing.T) {
	f, _ := setupFake(t)
	f.issueNums[21] = true
	bf := writeBody(t, "rev.md", okReviewBody)

	code := run(reviewArgs("example-org/org-slides", issueNum, "approve", testHead, bf))
	if code != deskkit.ExitRefused {
		t.Fatalf("review on an issue number = exit %d, want 5", code)
	}
	if f.postedReview != 0 {
		t.Fatal("no review may be posted against an issue")
	}
	d := lastAudit(t).Detail
	if !strings.Contains(d, "is an ISSUE") || !strings.Contains(d, "deskpost comment") {
		t.Fatalf("refusal does not name the kind and the right verb: %q", d)
	}
}

func TestReadyOnIssueRefusesNamingComment(t *testing.T) {
	f, _ := setupFake(t)
	f.issueNums[21] = true

	code := run([]string{"ready", "example-org/org-slides", issueNum})
	if code != deskkit.ExitRefused {
		t.Fatalf("ready on an issue number = exit %d, want 5", code)
	}
	if f.flips != 0 {
		t.Fatal("no flip may happen against an issue")
	}
	if d := lastAudit(t).Detail; !strings.Contains(d, "is an ISSUE") {
		t.Fatalf("refusal does not name the kind: %q", d)
	}
}

// TestCommentOnPRUnchangedByIssueSupport — regression guard for the PR path: it still
// resolves in ONE REST read (no extra issues lookup) and still keys idempotency on the
// pre-#296 verb `comment:<digest>` with the head recorded, so existing audit history
// keeps suppressing repeats.
func TestCommentOnPRUnchangedByIssueSupport(t *testing.T) {
	f, _ := setupFake(t)
	const prBody = "a PR comment"
	bf := writeBody(t, "c.md", prBody)
	if code := run(commentArgs(exampleRepo, "1", bf)); code != 0 {
		t.Fatalf("comment on PR exit = %d, want 0", code)
	}
	if n := f.hitCount("GET", "/issues/1"); n != 0 {
		t.Fatalf("PR path made %d issues-endpoint reads, want 0", n)
	}
	e := lastAudit(t)
	// The EXACT pre-#296 verb, recomputed here rather than prefix-matched: a prefix test
	// passes for `comment:pr:<digest>` too, and any change to this string silently breaks
	// suppression against the audit history already on disk — the desk would double-post.
	wantVerb := "comment:" + deskkit.Sha256Hex([]byte(prBody))
	if e.Verb != wantVerb {
		t.Fatalf("PR audit verb = %q, want the unchanged %q", e.Verb, wantVerb)
	}
	if e.HeadSHA == nil || *e.HeadSHA != testHead {
		t.Fatalf("PR audit head = %v, want %s", e.HeadSHA, testHead)
	}
}

// --- Only a 404 licenses the second lookup ----------------------------------------------
//
// The whole safety property of the #296 resolution is that ONLY a 404 from GET /pulls/{n}
// re-resolves through the issues endpoint. A 404 says "this number is not a PR"; a 403 or a
// 5xx says nothing about the kind at all. Without the guard a permission failure on the PR
// endpoint would fall through and be ANSWERED as "it's an issue" — the tool would comment on
// an object it was never allowed to read as a PR, and record it under the issue verb.
//
// These tests drive that with `pullStatus`, and every one of them uses a number the fake
// ALSO serves as a real issue: the issues endpoint would happily answer, so the assertion
// `hitCount("GET", "/issues/21") == 0` is the guard and nothing else. Delete the
// `if !isNotFound(perr)` check in resolveTarget (or the matching one in requirePRErr) and
// these fail.

// pullFails marks 21 as an issue AND forces a non-404 on its /pulls read, so a fallthrough
// would resolve successfully — the failure mode the guard exists to prevent.
func pullFails(t *testing.T, f *fakeGH, status int) {
	t.Helper()
	f.issueNums[21] = true
	f.pullStatus[21] = status
}

func TestCommentPull403NotReResolvedAsIssue(t *testing.T) {
	f, _ := setupFake(t)
	pullFails(t, f, 403)
	bf := writeBody(t, "i.md", "would have been an issue comment")

	code := run(commentArgs("example-org/org-slides", issueNum, bf))
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("403 on /pulls exit = %d, want 6 (the original error, unchanged)", code)
	}
	if n := f.hitCount("GET", "/issues/21"); n != 0 {
		t.Fatalf("a 403 licensed %d issues-endpoint reads, want 0 — only a 404 may", n)
	}
	if f.postedCmt != 0 {
		t.Fatal("no comment may be posted when the PR read failed for a non-404 reason")
	}
	if d := lastAudit(t).Detail; !strings.Contains(d, "403") {
		t.Fatalf("audit detail should carry the original 403: %q", d)
	}
}

func TestCommentPull500NotReResolvedAsIssue(t *testing.T) {
	f, _ := setupFake(t)
	pullFails(t, f, 500)
	bf := writeBody(t, "i.md", "would have been an issue comment")

	code := run(commentArgs("example-org/org-slides", issueNum, bf))
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("500 on /pulls exit = %d, want 6", code)
	}
	if n := f.hitCount("GET", "/issues/21"); n != 0 {
		t.Fatalf("a 500 licensed %d issues-endpoint reads, want 0 — only a 404 may", n)
	}
	if f.postedCmt != 0 {
		t.Fatal("no comment may be posted when the PR read failed for a non-404 reason")
	}
}

// The same guard on the PR-ONLY verbs: requirePRErr must not upgrade a non-404 to the
// wrong-kind refusal. A 403 stays exit 6 — "we could not read it" — never exit 5, which
// would assert the number is an issue on evidence that says no such thing.
func TestReviewPull403NotReResolvedAsIssue(t *testing.T) {
	f, _ := setupFake(t)
	pullFails(t, f, 403)
	bf := writeBody(t, "rev.md", okReviewBody)

	code := run(reviewArgs("example-org/org-slides", issueNum, "approve", testHead, bf))
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("review with a 403 on /pulls = exit %d, want 6 not 5", code)
	}
	if n := f.hitCount("GET", "/issues/21"); n != 0 {
		t.Fatalf("a 403 licensed %d issues-endpoint reads, want 0", n)
	}
	if f.postedReview != 0 {
		t.Fatal("no review may be posted")
	}
	if d := lastAudit(t).Detail; strings.Contains(d, "is an ISSUE") {
		t.Fatalf("a 403 must NOT be reported as a wrong-kind refusal: %q", d)
	}
}

func TestReadyPull403NotReResolvedAsIssue(t *testing.T) {
	f, _ := setupFake(t)
	pullFails(t, f, 403)

	code := run([]string{"ready", "example-org/org-slides", issueNum})
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("ready with a 403 on /pulls = exit %d, want 6 not 5", code)
	}
	if n := f.hitCount("GET", "/issues/21"); n != 0 {
		t.Fatalf("a 403 licensed %d issues-endpoint reads, want 0", n)
	}
	if f.flips != 0 {
		t.Fatal("no flip may happen")
	}
	if d := lastAudit(t).Detail; strings.Contains(d, "is an ISSUE") {
		t.Fatalf("a 403 must NOT be reported as a wrong-kind refusal: %q", d)
	}
}

// --- The other two answers resolveTarget can give ----------------------------------------
//
// The 404-only guard above is one of three checks in resolveTarget that decide what a
// number IS. The other two were unpinned: the `pull_request` discriminator (which stops a
// PR being commented on AS an issue) and the 404-only reading of the SECOND lookup (which
// stops a 403 there being reported as "this number does not exist"). Both are pinned here.

// TestCommentPhantomPRNotCommentedOnAsIssue is the write-consequential one. A number that
// IS a pull request whose /pulls read 404'd — a race with a delete, a permissions edge — is
// a state we cannot explain. deskpost must report the ORIGINAL failure, not fall through to
// the issue path: doing so would post on the object anyway and record the write under the
// issue verb, with no head, against an object that has one.
//
// The fixture is the point: `phantomPRNums` makes /pulls/{n} 404 while the issues endpoint
// serves the number WITH `pull_request`, so the ONLY thing standing between this test and a
// successful post is the discriminator check. Delete `if iss.PullRequest != nil` and this
// goes red.
func TestCommentPhantomPRNotCommentedOnAsIssue(t *testing.T) {
	f, _ := setupFake(t)
	f.phantomPRNums[21] = true
	bf := writeBody(t, "i.md", "must not land as an issue comment")

	code := run(commentArgs("example-org/org-slides", issueNum, bf))
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("phantom-PR exit = %d, want 6 (the original /pulls failure, unchanged)", code)
	}
	if f.postedCmt != 0 {
		t.Fatal("a number that IS a pull request must not be commented on as an issue")
	}
	e := lastAudit(t)
	if strings.HasPrefix(e.Verb, "comment:issue:") {
		t.Fatalf("audit verb = %q — a phantom PR must never be recorded under the issue verb", e.Verb)
	}
	// The issues endpoint WAS consulted (that is how the discriminator is read) — what must
	// not happen is resolving on it.
	if n := f.hitCount("GET", "/issues/21"); n != 1 {
		t.Fatalf("issues-endpoint reads = %d, want exactly 1 (the discriminator read)", n)
	}
	if d := e.Detail; strings.Contains(d, "neither a pull request nor an issue") {
		t.Fatalf("the number DOES exist as a PR; detail must not claim otherwise: %q", d)
	}
}

// TestCommentSecondLookupNon404NotReportedAsAbsent — the 404-only rule applies to the
// SECOND lookup too. The "neither a pull request nor an issue … check the number and the
// repo" message positively asserts the object does not exist; a 403 or a 5xx from the issues
// endpoint is not evidence of that, and telling a caller to check a number that is fine
// sends them looking in the wrong place. Both statuses stay exit 6 either way — what is
// under test is the CLAIM the message makes.
func TestCommentSecondLookupNon404NotReportedAsAbsent(t *testing.T) {
	for _, status := range []int{403, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			f, _ := setupFake(t)
			f.issueNums[21] = true // /pulls 404s, so the second lookup IS reached
			f.issueStatus[21] = status
			bf := writeBody(t, "i.md", "body")

			code := run(commentArgs("example-org/org-slides", issueNum, bf))
			if code != deskkit.ExitUnverifiable {
				t.Fatalf("issues-endpoint %d exit = %d, want 6", status, code)
			}
			if f.postedCmt != 0 {
				t.Fatal("nothing may be posted when the kind could not be resolved")
			}
			if d := lastAudit(t).Detail; strings.Contains(d, "neither a pull request nor an issue") {
				t.Fatalf("a %d must be reported as itself, not as absence: %q", status, d)
			}
		})
	}
}

// --- #296 × #396: the modifiers on an ISSUE target ---------------------------------------
//
// --dry-run and --wait landed while this branch was open (#355, #197). Both are kind-blind
// by construction — the dry-run terminus sits after the trust gate and reports describe(tgt),
// and --wait retries plan(), which re-resolves the target each attempt — but "by
// construction" is the claim these tests exist to stop being the only evidence for.

// TestDryRunOnIssue — a rehearsal on an issue runs every check, posts nothing, exits 0, and
// audits `dryrun` under the ISSUE verb with no head. It must also not suppress the later
// real post: a rehearsal is not a write, and AlreadyDoneIn/alreadyCommented match ok/noop
// only, never dryrun.
func TestDryRunOnIssue(t *testing.T) {
	f, _ := setupFake(t)
	f.issueNums[21] = true
	const body = "a rehearsed issue comment"
	bf := writeBody(t, "i.md", body)
	args := append(commentArgs("example-org/org-slides", issueNum, bf), "--dry-run")

	if code := run(args); code != 0 {
		t.Fatalf("dry-run on issue exit = %d, want 0", code)
	}
	if f.postedCmt != 0 {
		t.Fatal("a dry run must post nothing")
	}
	e := lastAudit(t)
	if e.Result != deskkit.ResultDryRun {
		t.Fatalf("audit result = %q, want dryrun", e.Result)
	}
	if want := "comment:issue:" + deskkit.Sha256Hex([]byte(body)); e.Verb != want {
		t.Fatalf("dry-run verb = %q, want %q", e.Verb, want)
	}
	if e.HeadSHA != nil && *e.HeadSHA != "" {
		t.Fatalf("dry-run on an issue recorded head %v, want none", e.HeadSHA)
	}
	if !strings.Contains(e.Detail, "issue #21") {
		t.Fatalf("dry-run detail must name the resolved kind, got %q", e.Detail)
	}
	// The real post still has to go through.
	if code := run(commentArgs("example-org/org-slides", issueNum, bf)); code != 0 {
		t.Fatalf("real post after a dry run exit = %d, want 0", code)
	}
	if f.postedCmt != 1 {
		t.Fatalf("postedCmt = %d, want 1 — the dry run must not have suppressed it", f.postedCmt)
	}
}

// TestCommentOnIssueRefusalDoesNotSuppressRetry — the fail-closed direction of the issue
// idempotency filter. A REFUSED first attempt (untrusted author) must not look like a
// completed write: after the author is blessed, the identical body has to post. If the
// filter ever counted refusals, a blessing would be unable to unblock the comment it was
// written to admit.
func TestCommentOnIssueRefusalDoesNotSuppressRetry(t *testing.T) {
	f, _ := setupFake(t)
	f.issueNums[21] = true
	f.issueAuthor, f.issueAuthorID = "some-drive-by", 424242
	bf := writeBody(t, "i.md", "the same body both times")

	if code := run(commentArgs("example-org/org-slides", issueNum, bf)); code != deskkit.ExitRefused {
		t.Fatalf("untrusted-author exit = %d, want 5", code)
	}
	if f.postedCmt != 0 {
		t.Fatal("nothing may be posted for an unblessed author")
	}
	// ada blesses the issue; the identical body must now post exactly once.
	f.issueTrustJSON = adaBlessedIssuePayload
	if code := run(commentArgs("example-org/org-slides", issueNum, bf)); code != 0 {
		t.Fatalf("post after blessing exit = %d, want 0", code)
	}
	if f.postedCmt != 1 {
		t.Fatalf("postedCmt = %d, want 1 — the refusal must not have suppressed the retry", f.postedCmt)
	}
}
