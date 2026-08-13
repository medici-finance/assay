package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// #355: the costly property of body validation was never strictness — it was
// that the ONLY way to discover a violation was to spend a charged attempt, one offender
// at a time (the reported ratchet: 49 → 37 → 33 chars over three charged attempts on one
// body). --dry-run makes discovery free. These tests pin the three properties that make it
// worth having: it makes no write, it costs no charged attempt, and it waives no check.

func TestDryRunReviewMakesNoWrite(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "rev.md", okReviewBody)

	args := append(reviewArgs(exampleRepo, "1", "approve", testHead, bf), "--dry-run")
	if code := run(args); code != 0 {
		t.Fatalf("dry-run review exit = %d, want 0", code)
	}
	if f.postedReview != 0 {
		t.Fatalf("postedReview = %d, want 0 — a dry run must not POST", f.postedReview)
	}
	e := lastAudit(t)
	if e.Result != deskkit.ResultDryRun {
		t.Fatalf("audit result = %q, want %q", e.Result, deskkit.ResultDryRun)
	}
	if e.Verb != "review:correctness:approve" {
		t.Fatalf("audit verb = %q — a dry run records the verb it rehearsed", e.Verb)
	}
	if e.HeadSHA == nil || *e.HeadSHA != testHead {
		t.Fatalf("audit head = %v, want the rehearsed head", e.HeadSHA)
	}
}

func TestDryRunCommentMakesNoWrite(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "c.md", "A plain status comment.\n")

	if code := run([]string{"comment", exampleRepo, "1", "--body-file", bf, "--dry-run"}); code != 0 {
		t.Fatalf("dry-run comment exit = %d, want 0", code)
	}
	if f.postedCmt != 0 {
		t.Fatalf("postedCmt = %d, want 0", f.postedCmt)
	}
	if e := lastAudit(t); e.Result != deskkit.ResultDryRun {
		t.Fatalf("audit result = %q, want dryrun", e.Result)
	}
}

func TestDryRunReadyMakesNoFlip(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()

	if code := run(append(readyArgs(exampleRepo), "--dry-run")); code != 0 {
		t.Fatalf("dry-run ready exit = %d, want 0", code)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0 — a dry run must not flip", f.flips)
	}
	if e := lastAudit(t); e.Result != deskkit.ResultDryRun || e.Verb != "ready" {
		t.Fatalf("audit = %+v, want verb=ready result=dryrun", e)
	}
}

// The load-bearing property: a rehearsal must not consume the budget it exists to protect.
// deskkit's chargesBudget excludes ResultDryRun, so N rehearsals leave the real post with
// the full budget — which is the whole difference between "validate freely" and "guess
// three times and lose the hour".
func TestDryRunDoesNotChargeTheWriteBudget(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "c.md", "A plain status comment.\n")

	for i := 0; i < 12; i++ { // > the 10/hour budget
		if code := run([]string{"comment", exampleRepo, "1", "--body-file", bf, "--dry-run"}); code != 0 {
			t.Fatalf("rehearsal %d exit = %d, want 0 — dry runs must not exhaust the budget", i, code)
		}
	}
	if f.postedCmt != 0 {
		t.Fatalf("postedCmt = %d after 12 rehearsals, want 0", f.postedCmt)
	}
	// The real post still has budget.
	if code := run([]string{"comment", exampleRepo, "1", "--body-file", bf}); code != 0 {
		t.Fatalf("real post after 12 rehearsals exit = %d, want 0", code)
	}
	if f.postedCmt != 1 {
		t.Fatalf("postedCmt = %d, want 1", f.postedCmt)
	}
}

// Nor may a rehearsal feed the non-progress breaker: deskkit's breakerIgnores excludes
// ResultDryRun. Without this, rehearsing a body five times would open a breaker against
// the real post — checking whether a write is safe would lock the write out, which is
// exactly the failure #214 fixed for deskrelease.
func TestDryRunDoesNotTripTheBreaker(t *testing.T) {
	_, _ = setupFake(t)
	bf := writeBody(t, "c.md", "A plain status comment.\n")

	for i := 0; i < 8; i++ {
		if code := run([]string{"comment", exampleRepo, "1", "--body-file", bf, "--dry-run"}); code != 0 {
			t.Fatalf("rehearsal %d exit = %d — the breaker must ignore dry runs", i, code)
		}
	}
}

// --dry-run is a REHEARSAL, not a waiver. Every refusal a real post would raise, a dry run
// raises identically — otherwise it would be a flag that validates a body different from
// the one that gets posted, which is worse than no rehearsal at all.
func TestDryRunStillRefusesABadBody(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "rev.md", "no heading and no verdict line\n")

	code := run(append(reviewArgs(exampleRepo, "1", "approve", testHead, bf), "--dry-run"))
	if code != deskkit.ExitRefused {
		t.Fatalf("dry-run on a schema-invalid body exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if f.postedReview != 0 {
		t.Fatal("no POST on a refused dry run")
	}
	if e := lastAudit(t); e.Result != deskkit.ResultRefused {
		t.Fatalf("audit result = %q, want refused — a refused rehearsal is a refusal", e.Result)
	}
}

// ...including the gates that are not about the body at all.
func TestDryRunStillRefusesAWrongHead(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "rev.md", okReviewBody)

	code := run(append(reviewArgs(exampleRepo, "1", "approve", testOldHead, bf), "--dry-run"))
	if code != deskkit.ExitRefused {
		t.Fatalf("dry-run with a stale --head exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if f.postedReview != 0 {
		t.Fatal("no POST on a refused dry run")
	}
}

func TestDryRunReadyStillRefusesRedCI(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.checks = checksWith("completed", "failure")

	code := run(append(readyArgs(exampleRepo), "--dry-run"))
	if code != deskkit.ExitRefused {
		t.Fatalf("dry-run ready on red CI exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatal("no flip")
	}
}

// A rehearsal that reports "would post" and then a real run that no-ops would be a lie of
// omission. ResultDryRun is deliberately NOT an idempotency key (AlreadyDoneIn matches
// ok/noop only), so rehearsing does not suppress the post it rehearsed.
func TestDryRunThenRealPostStillPosts(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "rev.md", okReviewBody)

	if code := run(append(reviewArgs(exampleRepo, "1", "approve", testHead, bf), "--dry-run")); code != 0 {
		t.Fatalf("rehearsal exit = %d", code)
	}
	if code := run(reviewArgs(exampleRepo, "1", "approve", testHead, bf)); code != 0 {
		t.Fatalf("real post after rehearsal exit = %d, want 0", code)
	}
	if f.postedReview != 1 {
		t.Fatalf("postedReview = %d, want 1 — the rehearsal must not suppress the post", f.postedReview)
	}
}

func TestDryRunAndWaitAreMutuallyExclusive(t *testing.T) {
	_, errBuf := setupFake(t)
	bf := writeBody(t, "c.md", "x\n")

	if code := run([]string{"comment", exampleRepo, "1", "--body-file", bf, "--dry-run", "--wait", "5m"}); code != 2 {
		t.Fatalf("--dry-run --wait exit = %d, want 2 (usage)", code)
	}
	if !strings.Contains(errBuf.String(), "mutually exclusive") {
		t.Fatalf("stderr does not explain the conflict:\n%s", errBuf.String())
	}
}

// An unknown flag is a usage error on every verb, not a silent ignore — `ready` gained a
// FlagSet for --dry-run/--wait and must not become the verb that swallows typos.
func TestReadyRejectsUnknownFlag(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()

	if code := run(append(readyArgs(exampleRepo), "--force")); code != 2 {
		t.Fatalf("unknown flag on ready exit = %d, want 2", code)
	}
	if f.flips != 0 {
		t.Fatal("no flip on a usage error")
	}
}
