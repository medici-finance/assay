package main

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Trust-gate tests (deskkit/trust.go): deskpost must REFUSE (exit 5, audited, zero
// mutating calls) every verb on a PR authored outside the trusted desk identities,
// unless the PR carries a CURRENT ada blessing.

// adaBlessedPayload is a PRTrustQuery response with one ada conversation
// comment (correct numeric databaseId) and no later untrusted content.
const adaBlessedPayload = `{"data":{"repository":{"pullRequest":{"lastEditedAt":null,` +
	`"comments":{"pageInfo":{"hasNextPage":false},"nodes":[` +
	`{"createdAt":"2026-07-21T10:00:00Z","lastEditedAt":null,"author":{"login":"ada","__typename":"User","databaseId":2001}}]},` +
	`"reviews":{"pageInfo":{"hasNextPage":false},"nodes":[]},` +
	`"reviewThreads":{"pageInfo":{"hasNextPage":false},"nodes":[]}}}}}`

// blessThenEditPayload: ada blessed at 07-21 but the PR BODY was edited at 07-22 —
// the blessing is void.
const blessThenEditPayload = `{"data":{"repository":{"pullRequest":{"lastEditedAt":"2026-07-22T10:00:00Z",` +
	`"comments":{"pageInfo":{"hasNextPage":false},"nodes":[` +
	`{"createdAt":"2026-07-21T10:00:00Z","lastEditedAt":null,"author":{"login":"ada","__typename":"User","databaseId":2001}}]},` +
	`"reviews":{"pageInfo":{"hasNextPage":false},"nodes":[]},` +
	`"reviewThreads":{"pageInfo":{"hasNextPage":false},"nodes":[]}}}}}`

func TestReadyUntrustedAuthorExit5(t *testing.T) {
	f, _ := setupFake(t)
	f.prAuthor = "external-user"
	f.prAuthorID = 424242
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	// default trustJSON: empty payload, no ada — unblessed.

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("ready on untrusted-author PR = exit %d, want 5", code)
	}
	if f.flips != 0 {
		t.Fatal("no flip may happen on an untrusted, unblessed PR")
	}
	if e := lastAudit(t); e.Result != deskkit.ResultRefused {
		t.Fatalf("audit result = %q, want refused", e.Result)
	}
}

func TestReadyAdaBlessedFlips(t *testing.T) {
	f, _ := setupFake(t)
	f.prAuthor = "external-user"
	f.prAuthorID = 424242
	f.trustJSON = adaBlessedPayload
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != 0 {
		t.Fatalf("ready on ada-blessed PR = exit %d, want 0", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1", f.flips)
	}
}

func TestReadyBlessThenEditExit5(t *testing.T) {
	f, _ := setupFake(t)
	f.prAuthor = "external-user"
	f.prAuthorID = 424242
	f.trustJSON = blessThenEditPayload
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("ready on bless-then-edited PR = exit %d, want 5", code)
	}
	if f.flips != 0 {
		t.Fatal("no flip after the blessed body was edited")
	}
}

// TestReadyTrustedLoginWrongIDExit5 — recycled-login defense: the REST author login
// reads as a trusted identity but the numeric id does not match; the gate treats the
// author as untrusted and (unblessed) refuses.
func TestReadyTrustedLoginWrongIDExit5(t *testing.T) {
	f, _ := setupFake(t)
	f.prAuthor = "shared-agent"
	f.prAuthorID = 31337 // NOT shared-agent (2002) — wrong id
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("ready with recycled trusted login = exit %d, want 5", code)
	}
	if f.flips != 0 {
		t.Fatal("no flip for a trusted login carrying the wrong numeric id")
	}
}

func TestReviewUntrustedAuthorExit5(t *testing.T) {
	f, _ := setupFake(t)
	f.prAuthor = "external-user"
	f.prAuthorID = 424242
	bf := writeBody(t, "rev.md", okReviewBody)

	code := run(reviewArgs(exampleRepo, "1", "approve", testHead, bf))
	if code != deskkit.ExitRefused {
		t.Fatalf("review on untrusted-author PR = exit %d, want 5", code)
	}
	if f.postedReview != 0 {
		t.Fatal("no review may be posted on an untrusted, unblessed PR")
	}
}

// TestUntrustedAuthorPRGateSplitByVerb pins the by-verb split of the trust gate on a
// PR authored OUTSIDE the trusted desk identities and carrying NO blessing (empty
// trustJSON): a plain `comment` POSTS, while `review` (a verdict) and `ready` (a flip)
// STILL REFUSE with exit 5 and make zero mutating calls. A comment carries no approval
// authority and changes no state; the verdict and the flip are what the quarantine (#943)
// exists to protect, so they stay gated on the unchanged prTrustGate predicate.
func TestUntrustedAuthorPRGateSplitByVerb(t *testing.T) {
	t.Run("comment_posts", func(t *testing.T) {
		f, _ := setupFake(t)
		f.prAuthor = "external-user"
		f.prAuthorID = 424242
		// default trustJSON empty (unblessed); default repoVisibility private (public-repo
		// +1 gate not in play) — so the ONLY gate that could refuse is the author-trust gate.
		bf := writeBody(t, "c.md", "reviewed: clean single-dep patch bump.")

		code := run(commentArgs(exampleRepo, "1", bf))
		if code != deskkit.ExitOK {
			t.Fatalf("comment on untrusted-author PR = exit %d, want 0 (posts)", code)
		}
		if f.postedCmt != 1 {
			t.Fatalf("postedCmt = %d, want 1 — the comment must post on an unblessed-author PR", f.postedCmt)
		}
		if f.postedReview != 0 || f.flips != 0 {
			t.Fatal("a comment must post no review and flip nothing")
		}
		if e := lastAudit(t); e.Result != deskkit.ResultOK {
			t.Fatalf("audit result = %q, want ok", e.Result)
		}
	})

	t.Run("review_refuses", func(t *testing.T) {
		f, _ := setupFake(t)
		f.prAuthor = "external-user"
		f.prAuthorID = 424242
		bf := writeBody(t, "rev.md", okReviewBody)

		code := run(reviewArgs(exampleRepo, "1", "approve", testHead, bf))
		if code != deskkit.ExitRefused {
			t.Fatalf("review on untrusted-author PR = exit %d, want 5 (refused)", code)
		}
		if f.postedReview != 0 {
			t.Fatal("no verdict may be posted on an untrusted, unblessed PR")
		}
	})

	t.Run("ready_refuses", func(t *testing.T) {
		f, _ := setupFake(t)
		f.prAuthor = "external-user"
		f.prAuthorID = 424242
		// Give it an APPROVED review at head + green status, so the ONLY thing left to refuse
		// the flip is the trust gate — proving the gate, not a missing approval, holds.
		f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
		f.status = greenStatus()

		code := run(readyArgs(exampleRepo))
		if code != deskkit.ExitRefused {
			t.Fatalf("ready on untrusted-author PR = exit %d, want 5 (refused)", code)
		}
		if f.flips != 0 {
			t.Fatal("no flip may happen on an untrusted, unblessed PR")
		}
	})
}

func TestCommentAdaBlessedPosts(t *testing.T) {
	f, _ := setupFake(t)
	f.prAuthor = "external-user"
	f.prAuthorID = 424242
	f.trustJSON = adaBlessedPayload
	bf := writeBody(t, "c.md", "a comment")

	code := run(commentArgs(exampleRepo, "1", bf))
	if code != 0 {
		t.Fatalf("comment on ada-blessed PR = exit %d, want 0", code)
	}
	if f.postedCmt != 1 {
		t.Fatalf("postedCmt = %d, want 1", f.postedCmt)
	}
}
