package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func commentArgs(repo, pr, bodyFile string) []string {
	return []string{"comment", repo, pr, "--body-file", bodyFile}
}

func TestCommentSuccess(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "c.md", "Re-reviewed the delta; the cursor off-by-one is fixed.")
	code := run(commentArgs("example-org/tracker", "1", bf))
	if code != 0 {
		t.Fatalf("comment exit = %d, want 0", code)
	}
	if f.postedCmt != 1 {
		t.Fatalf("postedCmt = %d, want 1", f.postedCmt)
	}
	if e := lastAudit(t); e.Result != deskkit.ResultOK || e.BodyDigest == "" {
		t.Fatalf("audit = %+v", e)
	}
}

// TestCommentBadBodyExit5NoNetwork checks that a secret-carrying body refuses (5)
// with no comment posted and no network call.
func TestCommentBadBodyExit5NoNetwork(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "bad.md", "ghp_"+repeat("A", 40))
	code := run(commentArgs("example-org/tracker", "1", bf))
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

func TestCommentOversizedExit5(t *testing.T) {
	f, _ := setupFake(t)
	big := make([]byte, 16*1024+1)
	for i := range big {
		big[i] = 'x'
	}
	bf := writeBody(t, "big.md", string(big))
	code := run(commentArgs("example-org/tracker", "1", bf))
	if code != deskkit.ExitRefused {
		t.Fatalf("oversized exit = %d, want 5", code)
	}
	if f.postedCmt != 0 {
		t.Fatal("no comment on an oversized body")
	}
}

// TestCommentRepeatNoNewPost — an identical re-post is a no-op that makes no MUTATING
// call (a GET to resolve the head still happens; the POST does not).
func TestCommentRepeatNoNewPost(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "c.md", "same comment body")
	args := commentArgs("example-org/tracker", "1", bf)

	if code := run(args); code != 0 {
		t.Fatalf("first comment exit = %d", code)
	}
	if code := run(args); code != 0 {
		t.Fatalf("repeat comment exit = %d, want 0 (noop)", code)
	}
	if f.postedCmt != 1 {
		t.Fatalf("postedCmt = %d, want 1 (repeat must not POST again)", f.postedCmt)
	}
	if lastAudit(t).Result != deskkit.ResultNoop {
		t.Fatal("repeat should audit result=noop")
	}
}

// TestCommentRateLimitExit4 — 10 deskpost writes already this hour → the next is refused
// with exit 4 and no comment posted.
func TestCommentRateLimitExit4(t *testing.T) {
	f, _ := setupFake(t)
	pr := 1
	for i := 0; i < deskkit.RateLimitPerPRPerHour; i++ {
		if err := deskkit.Log(deskkit.Entry{
			Tool: toolName, Verb: "comment:seed", Repo: "example-org/tracker",
			PR: &pr, Result: deskkit.ResultOK,
		}); err != nil {
			t.Fatalf("seed audit: %v", err)
		}
	}
	bf := writeBody(t, "c.md", "should be rate limited")
	code := run(commentArgs("example-org/tracker", "1", bf))
	if code != deskkit.ExitRateLimited {
		t.Fatalf("rate-limit exit = %d, want 4", code)
	}
	if f.postedCmt != 0 {
		t.Fatal("no comment posted when rate limited")
	}
}

// ---------------------------------------------------------------------------
// #513, second defect — `comment` had no --head, so it stamped
// whichever head was live at POST time into the audit ledger and the idempotency
// key. On #505 the reviewer examined f9c9ca94 and d7b2d1d3 was recorded, because a
// merge landed mid-review. That one was a clean keep-current merge so the body's
// conclusions still held — but the tool guaranteed nothing and would have stamped
// the new head identically had the push been real work.
//
// --head is OPTIONAL (an ISSUE has no head at all, #296) and ENFORCED when given.
// ---------------------------------------------------------------------------

func commentArgsAtHead(repo, pr, bodyFile, head string) []string {
	return append(commentArgs(repo, pr, bodyFile), "--head", head)
}

// TestCommentHeadMovedRefusesWithoutPosting is the #505 shape: the head moved between
// forming the comment and posting it. The comment must NOT go out silently stamped with
// the new head.
func TestCommentHeadMovedRefusesWithoutPosting(t *testing.T) {
	f, errBuf := setupFake(t)
	bf := writeBody(t, "c.md", "Reviewed the delta; no findings.")

	code := run(commentArgsAtHead(exampleRepo, "1", bf, testOldHead))
	if code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d — a stale --head must refuse", code, deskkit.ExitRefused)
	}
	if f.postedCmt != 0 {
		t.Fatalf("postedCmt = %d, want 0 — nothing may be posted against a head that moved", f.postedCmt)
	}
	if !strings.Contains(errBuf.String(), short(testHead)) {
		t.Fatalf("refusal must name the CURRENT head so the caller can re-read it, got: %s", errBuf.String())
	}
}

// TestCommentHeadMatchesPosts — the assertion holds, so the write proceeds unchanged.
func TestCommentHeadMatchesPosts(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "c.md", "Reviewed the delta; no findings.")

	if code := run(commentArgsAtHead(exampleRepo, "1", bf, testHead)); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if f.postedCmt != 1 {
		t.Fatalf("postedCmt = %d, want 1", f.postedCmt)
	}
	if e := lastAudit(t); e.HeadSHA == nil || *e.HeadSHA != testHead {
		t.Fatalf("audit head = %v, want %s", e.HeadSHA, testHead)
	}
}

// TestCommentWithoutHeadStillPosts — the flag is optional, and its absence must not change
// anything. Most desk comments are not written against a specific commit, and every existing
// caller omits it.
func TestCommentWithoutHeadStillPosts(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "c.md", "Filed the follow-up as #513.")

	if code := run(commentArgs(exampleRepo, "1", bf)); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if f.postedCmt != 1 {
		t.Fatalf("postedCmt = %d, want 1", f.postedCmt)
	}
}

// TestCommentHeadOnAnIssueRefuses — an issue has no head, so asserting one is a caller
// error. Accepting it silently would let `--head` read as "verified" where nothing was.
//
// The stderr assertion is the whole test. An issue's tgt.head is "", so the generic
// mismatch branch below refuses it anyway — delete the ISSUE branch and the exit code and
// postedCmt are both unchanged. The branch's ONLY contribution is the message, so the
// message is the only thing that can pin it. What the caller gets without it is actively
// misleading: "--head <sha> != current head " (empty), telling them to re-read a PR at ""
// when the number does not name a PR at all.
func TestCommentHeadOnAnIssueRefuses(t *testing.T) {
	f, errBuf := setupFake(t)
	f.issueNums[21] = true
	bf := writeBody(t, "c.md", "Triaged.")

	if code := run(commentArgsAtHead("medici-finance/assay", "21", bf, testHead)); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if f.postedCmt != 0 {
		t.Fatalf("postedCmt = %d, want 0", f.postedCmt)
	}
	if !strings.Contains(errBuf.String(), "is an ISSUE, which has no head") {
		t.Fatalf("the refusal must say the number names an ISSUE — the generic head-mismatch "+
			"branch refuses too, but tells the caller to re-read a PR at an EMPTY head. "+
			"stderr: %s", errBuf.String())
	}
}

// TestCommentStaleHeadRefusesEvenWhenTheBodyIsAlreadyPosted pins the ORDERING that
// comment.go states as a property: the --head assertion runs BEFORE the idempotency read.
//
// Nothing else observes it. Every other --head test uses a body that was never posted, so
// the two blocks can be swapped with the whole suite green — and swapped, this exact
// sequence returns exit 0. That is the #505 shape one step further along: the body was
// already posted at the NEW head, and the reviewer is now asserting the OLD one. The
// caller's question is "is the commit I reviewed still the head?" and the answer would be
// "an identical comment is already there" — success-shaped, and an answer to a different
// question. A stale --head must REFUSE; it must never be absorbed as a no-op.
func TestCommentStaleHeadRefusesEvenWhenTheBodyIsAlreadyPosted(t *testing.T) {
	f, errBuf := setupFake(t)
	bf := writeBody(t, "c.md", "Reviewed the delta; no findings.")

	// Seed: the identical body is already on the PR at the CURRENT head.
	if code := run(commentArgs(exampleRepo, "1", bf)); code != 0 {
		t.Fatalf("seed comment exit = %d, want 0", code)
	}
	if f.postedCmt != 1 {
		t.Fatalf("seed postedCmt = %d, want 1", f.postedCmt)
	}

	// Now the same body, asserted against a head that has since moved on.
	code := run(commentArgsAtHead(exampleRepo, "1", bf, testOldHead))
	if code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d — a stale --head must REFUSE. Exit 0 here means the "+
			"idempotency read answered first and the caller was told 'already posted' when "+
			"the real answer is 'the commit you reviewed is no longer the head'",
			code, deskkit.ExitRefused)
	}
	if f.postedCmt != 1 {
		t.Fatalf("postedCmt = %d, want 1 — the refusal must not post again", f.postedCmt)
	}
	if !strings.Contains(errBuf.String(), "no longer") {
		t.Fatalf("the refusal must name the head having moved, not idempotency: %s", errBuf.String())
	}
}

// TestCommentAbbreviatedHeadIsUsageError — same rule as `review --head` (#214): an
// abbreviated SHA is a FORM error (exit 2), not a head mismatch (exit 5). The two call for
// opposite responses from the caller.
func TestCommentAbbreviatedHeadIsUsageError(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "c.md", "Reviewed.")

	if code := run(commentArgsAtHead(exampleRepo, "1", bf, testHead[:12])); code != 2 {
		t.Fatalf("exit = %d, want 2 (usage)", code)
	}
	if f.postedCmt != 0 {
		t.Fatalf("postedCmt = %d, want 0", f.postedCmt)
	}
}
