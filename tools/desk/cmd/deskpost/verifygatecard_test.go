package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The verify-gate card carve-out, end to end through deskpost (#868).
//
// Every verify-gate sign-off card is filed by the repo's own verify-gate-open
// workflow under GITHUB_TOKEN, so its author is `github-actions[bot]` — an
// identity the trust gate correctly refuses (an Actions workflow is not a desk
// identity). The consequence was that a desk could not annotate a card at all:
// not to mark one an inert duplicate, not to say it cannot flip on close. The
// human closing the card saw no warning, because nothing could write one.
//
// The carve-out is deliberately the narrowest read that fixes that: the `comment`
// verb, on an ISSUE, authored by the Actions identity, carrying the `verify-gate`
// label. The card's body is generated from the TREE by statusgen — it is not
// inbound third-party text — which is the property that makes it safe to annotate
// and the reason the label, not the author, is what scopes it.
//
// These tests pin the one admission and the refusals around it. The refusals are
// the load-bearing half: a carve-out that widened to another verb or another label
// would put a desk verdict or a ready-flip behind an identity any workflow file in
// the repo can assume.

const actionsBot = "github-actions[bot]"

// actionsBotCard marks 21 as an ISSUE authored by the Actions identity and labelled
// `verify-gate` — the sign-off card shape — and returns the `comment` argv for it.
func actionsBotCard(t *testing.T, f *fakeGH, repo, bodyFile string) []string {
	t.Helper()
	f.issueNums[21] = true
	f.issueAuthor, f.issueAuthorID = actionsBot, 41898282
	f.issueLabels[21] = []string{"verify-gate", "gate:human"}
	return commentArgs(repo, issueNum, bodyFile)
}

// TestCommentOnVerifyGateCardPosts is the admission: the desk can annotate a card.
func TestCommentOnVerifyGateCardPosts(t *testing.T) {
	f, _ := setupFake(t)
	const body = "This card is an inert duplicate of the earlier one; closing it will not flip the row."
	bf := writeBody(t, "card.md", body)

	code := run(actionsBotCard(t, f, "example-org/org-slides", bf))
	if code != deskkit.ExitOK {
		t.Fatalf("comment on a verify-gate card = exit %d, want 0", code)
	}
	if f.postedCmt != 1 {
		t.Fatalf("postedCmt = %d, want 1 — the annotation must land on the card", f.postedCmt)
	}
	if e := lastAudit(t); e.Result != deskkit.ResultOK {
		t.Fatalf("audit result = %q, want ok", e.Result)
	}
	// The carve-out changes WHO may be commented on, nothing else: the write is still
	// recorded under the issue verb, so idempotency and the budget keep working.
	if want := "comment:issue:" + deskkit.Sha256Hex([]byte(body)); lastAudit(t).Verb != want {
		t.Fatalf("audit verb = %q, want %q", lastAudit(t).Verb, want)
	}
	// It reads the trust events for NOTHING: a carve-out that still paid for the
	// GraphQL blessing read would be a carve-out that had not actually decided.
	if n := f.hitCount("POST", "/graphql"); n != 0 {
		t.Fatalf("GraphQL trust reads = %d, want 0 on an admitted card", n)
	}
}

// TestCommentOnActionsBotIssueWithoutLabelRefuses — refusal 1, the label scope. The
// SAME author, the same verb, no `verify-gate` label: still exit 5, nothing posted.
// Any workflow in the repo can file an issue as this identity; only the sign-off
// card's body is generated from the tree, and the label is how the card says so.
func TestCommentOnActionsBotIssueWithoutLabelRefuses(t *testing.T) {
	f, _ := setupFake(t)
	f.issueNums[21] = true
	f.issueAuthor, f.issueAuthorID = actionsBot, 41898282
	f.issueLabels[21] = []string{"bug", "verify-gate-close"} // NOT the label
	bf := writeBody(t, "i.md", "an annotation that must not land")

	code := run(commentArgs("example-org/org-slides", issueNum, bf))
	if code != deskkit.ExitRefused {
		t.Fatalf("comment on an unlabelled Actions-bot issue = exit %d, want 5", code)
	}
	if f.postedCmt != 0 {
		t.Fatal("nothing may be posted on an Actions-bot issue without the verify-gate label")
	}
	if e := lastAudit(t); e.Result != deskkit.ResultRefused {
		t.Fatalf("audit result = %q, want refused", e.Result)
	}
}

// TestReviewOnVerifyGateCardRefuses — refusal 2, the verb scope. `review` posts a
// VERDICT; the carve-out admits an annotation and nothing else, so a card stays
// refused for it.
func TestReviewOnVerifyGateCardRefuses(t *testing.T) {
	f, _ := setupFake(t)
	f.issueNums[21] = true
	f.issueAuthor, f.issueAuthorID = actionsBot, 41898282
	f.issueLabels[21] = []string{"verify-gate"}
	bf := writeBody(t, "rev.md", okReviewBody)

	code := run(reviewArgs("example-org/org-slides", issueNum, "approve", testHead, bf))
	if code != deskkit.ExitRefused {
		t.Fatalf("review on a verify-gate card = exit %d, want 5", code)
	}
	if f.postedReview != 0 {
		t.Fatal("no verdict may be posted on a verify-gate card")
	}
}

// TestSecurityReviewOnVerifyGateCardRefuses — refusal 3, the same verb scope on the
// security verdict, which is its own verb and its own review object.
func TestSecurityReviewOnVerifyGateCardRefuses(t *testing.T) {
	f, _ := setupFake(t)
	f.issueNums[21] = true
	f.issueAuthor, f.issueAuthorID = actionsBot, 41898282
	f.issueLabels[21] = []string{"verify-gate"}
	bf := writeBody(t, "sec.md", okSecurityBody)

	code := run([]string{"security-review", "example-org/org-slides", issueNum,
		"--verdict", "pass", "--head", testHead, "--body-file", bf})
	if code != deskkit.ExitRefused {
		t.Fatalf("security-review on a verify-gate card = exit %d, want 5", code)
	}
	if f.postedReview != 0 {
		t.Fatal("no security verdict may be posted on a verify-gate card")
	}
}

// TestReadyOnVerifyGateCardRefuses — refusal 4, the flip. A card is an issue and
// has nothing to flip; what matters is that the carve-out did not make it look
// flippable.
func TestReadyOnVerifyGateCardRefuses(t *testing.T) {
	f, _ := setupFake(t)
	f.issueNums[21] = true
	f.issueAuthor, f.issueAuthorID = actionsBot, 41898282
	f.issueLabels[21] = []string{"verify-gate"}

	code := run([]string{"ready", "example-org/org-slides", issueNum})
	if code != deskkit.ExitRefused {
		t.Fatalf("ready on a verify-gate card = exit %d, want 5", code)
	}
	if f.flips != 0 {
		t.Fatal("no flip may happen on a verify-gate card")
	}
	if d := lastAudit(t).Detail; !strings.Contains(d, "is an ISSUE") {
		t.Fatalf("refusal does not name the kind: %q", d)
	}
}

// TestVerifyGateLabelOnAnUntrustedAuthorsIssueStillRefuses — the author scope. The
// label is not a bless: an external user who labels their own issue `verify-gate`
// gains nothing. Without this, the carve-out would be a self-service admission on
// any repo where a drive-by can add labels.
func TestVerifyGateLabelOnAnUntrustedAuthorsIssueStillRefuses(t *testing.T) {
	f, _ := setupFake(t)
	f.issueNums[21] = true
	f.issueAuthor, f.issueAuthorID = "external-user", 424242
	f.issueLabels[21] = []string{"verify-gate"}
	bf := writeBody(t, "i.md", "must not land")

	code := run(commentArgs("example-org/org-slides", issueNum, bf))
	if code != deskkit.ExitRefused {
		t.Fatalf("comment on a self-labelled external issue = exit %d, want 5", code)
	}
	if f.postedCmt != 0 {
		t.Fatal("the verify-gate label must admit nothing on its own")
	}
}

// TestVerifyGateCardStillGetsTheOtherCommentProtections — the carve-out is scoped to
// the author-trust dimension ONLY. The body checks still run on a card comment, with
// zero network calls: a secret-carrying annotation refuses exactly as anywhere else.
func TestVerifyGateCardStillGetsTheBodyChecks(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "bad.md", "ghp_"+repeat("A", 40))

	code := run(actionsBotCard(t, f, "example-org/org-slides", bf))
	if code != deskkit.ExitRefused {
		t.Fatalf("secret-carrying card comment = exit %d, want 5", code)
	}
	if len(f.hits) != 0 {
		t.Fatalf("expected ZERO network hits, got %v", f.hits)
	}
	if f.postedCmt != 0 {
		t.Fatal("no comment on a refusal")
	}
}

// TestVerifyGateCardStillGetsTheRepoGate — and the repo gate: a card in a repo
// outside the desk set is still out of reach, before any network call.
func TestVerifyGateCardStillGetsTheRepoGate(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "i.md", "an annotation")

	code := run(actionsBotCard(t, f, "some-org/not-in-the-set", bf))
	if code != deskkit.ExitRefused {
		t.Fatalf("card in an out-of-set repo = exit %d, want 5", code)
	}
	if len(f.hits) != 0 {
		t.Fatalf("expected ZERO network hits, got %v", f.hits)
	}
}
