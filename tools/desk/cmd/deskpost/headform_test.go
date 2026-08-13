package main

import (
	"strings"
	"testing"
)

// --- #214 item 3: `deskpost review --head` form validation ---
//
// The head pin is the thing that stops a verdict landing on code nobody reviewed, and
// before this gate its ARGUMENT had no form check at all. An abbreviated SHA was
// accepted, carried all the way to the equality test against the resolved head, failed
// it, and was then rendered by short() as `5d529c27e3b1` vs `5d529c27e3b1a04f9c2d…` —
// a "head mismatch" whose two SHAs differ only in length.
//
// That report names the wrong problem. A head mismatch means "the branch moved, re-review
// it"; a short SHA means "pass the full one". A reviewer told the first does the second's
// work for nothing, and — because a re-review ends in another `deskpost review` — does it
// in a loop that the non-progress breaker will eventually stop for reasons unrelated to
// the actual mistake (#1255's third item).

// TestAbbreviatedHeadIsAFormErrorNotAHeadMismatch is the core of the item: the SHORT
// form of the CORRECT head must be rejected as a form error, before any network call,
// and the message must say "abbreviated" rather than reporting a mismatch.
func TestAbbreviatedHeadIsAFormErrorNotAHeadMismatch(t *testing.T) {
	for _, short := range []string{testHead[:7], testHead[:8], testHead[:12], testHead[:39]} {
		t.Run(short, func(t *testing.T) {
			f, errb := setupFake(t)
			f.pullHeads = []string{testHead} // the SHORT sha is a prefix of the REAL head
			bf := writeBody(t, "rev.md", okReviewBody)

			code := run(reviewArgs("example-org/tracker", "1", "approve", short, bf))
			if code != 2 {
				t.Fatalf("exit = %d, want 2 (usage) — an abbreviated --head is a malformed "+
					"argument, in the same class as a malformed repo or PR number", code)
			}
			msg := errb.String()
			if !strings.Contains(msg, "ABBREVIATED") {
				t.Fatalf("stderr %q does not name the abbreviation — the whole point of the "+
					"gate is that the caller learns the REAL problem on the first try", msg)
			}
			if !strings.Contains(msg, "40-character") {
				t.Fatalf("stderr %q does not state the required form; #1255's third item "+
					"asks specifically for the form to be surfaced in the refusal", msg)
			}
			if f.postedReview != 0 || len(f.hits) != 0 {
				t.Fatalf("a malformed --head reached the network: postedReview=%d hits=%v",
					f.postedReview, f.hits)
			}
			// It must NOT have taken the head-mismatch path. That path audits a
			// `refused` line carrying the two-SHA message; a form error is caught in
			// argv parsing and audits nothing, exactly as a malformed repo or PR number
			// does. The audit log is the durable discriminator between the two — and
			// the one that matters, since it is also what the non-progress breaker
			// reads, so a form typo cannot spend the caller's way toward a breaker trip.
			if got := auditEntries(t); len(got) != 0 {
				t.Fatalf("a form error wrote %d audit lines, want 0 (it never reached the "+
					"outward-write flow): %+v", len(got), got)
			}
		})
	}
}

// TestMalformedHeadFormsAreRefused walks the rest of the shape space. Each row is
// rejected for a reason the message states, and none reaches the network.
//
// Uppercase hex is in the REFUSE set deliberately. It is not normalised: git and the
// GitHub API emit lowercase, so an uppercased SHA is byte-unequal to the resolved head
// and would land back in the misleading mismatch report this gate removes. Case-folding
// it would silently repair an argument nothing legitimate produces.
func TestMalformedHeadFormsAreRefused(t *testing.T) {
	cases := []struct {
		name string
		head string
	}{
		{"a placeholder word", "headSHA"},
		{"a branch name", "origin/main"},
		{"HEAD", "HEAD"},
		{"a revision expression", testHead + "^{}"},
		{"a ref with a peel", "refs/heads/main"},
		{"39 hex chars — one short", testHead[:39]},
		{"41 hex chars — one long", testHead + "0"},
		{"right length, non-hex character", strings.Repeat("g", 40)},
		{"right length, uppercase hex", strings.ToUpper(testHead)},
		{"right length, mixed case", "5D529C27E3B1A04F9C2D8E7B6A1F0C3D4E5F6A7b"},
		{"a SHA with surrounding whitespace", " " + testHead + " "},
		{"a SHA with a trailing newline", testHead + "\n"},
		{"a URL containing the SHA", "https://github.com/o/r/commit/" + testHead},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, errb := setupFake(t)
			bf := writeBody(t, "rev.md", okReviewBody)

			code := run(reviewArgs("example-org/tracker", "1", "approve", c.head, bf))
			if code != 2 {
				t.Fatalf("--head %q exited %d, want 2", c.head, code)
			}
			if !strings.Contains(errb.String(), "--head") {
				t.Fatalf("stderr %q does not name --head as the offending argument", errb.String())
			}
			if f.postedReview != 0 || len(f.hits) != 0 {
				t.Fatalf("--head %q reached the network: postedReview=%d hits=%v",
					c.head, f.postedReview, f.hits)
			}
		})
	}
}

// TestFullSHAFormsAreAccepted is the other direction, and it is what keeps the gate from
// being tightened into a false refusal. A form error must never be raised for a SHA a
// remote could actually report — the failure mode would be a completed verdict that
// cannot be posted at all, which is strictly worse than a confusing message.
func TestFullSHAFormsAreAccepted(t *testing.T) {
	cases := []struct {
		name string
		head string
	}{
		{"a 40-char SHA-1", testHead},
		{"all digits", strings.Repeat("0", 40)},
		{"all letters", strings.Repeat("abcdef", 6) + "abcd"},
		// deskkit's secret scanner defines a git SHA as 40 OR 64 lowercase hex; the two
		// definitions are kept aligned so the tool has one notion of "a SHA".
		{"a 64-char SHA-256 object id", strings.Repeat("0123456789abcdef", 4)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !isFullSHA(c.head) {
				t.Fatalf("isFullSHA(%q) = false — a legitimate head form would be refused, "+
					"stranding a completed verdict", c.head)
			}
			f, _ := setupFake(t)
			f.pullHeads = []string{c.head}
			bf := writeBody(t, "rev.md", okReviewBody)

			if code := run(reviewArgs("example-org/tracker", "1", "approve", c.head, bf)); code != 0 {
				t.Fatalf("--head %q exited %d, want 0 — the form gate must not refuse a real SHA", c.head, code)
			}
			if f.postedReview != 1 {
				t.Fatalf("postedReview = %d, want 1", f.postedReview)
			}
		})
	}
}

// TestGenuineHeadMismatchStillReportsAMismatch — the form gate must not swallow the
// case it was carved out of. Two DIFFERENT full SHAs is a real head race, and it must
// still refuse with exit 5 and the mismatch wording, not exit 2.
// The mismatch wording is asserted against the AUDIT DETAIL rather than stderr: this
// binary's finishAudit prints refusal detail to os.Stderr directly (writeflow.go), which
// the harness does not capture. The audit line carries the same text and is the durable
// record, so asserting there is both possible and stronger.
func TestGenuineHeadMismatchStillReportsAMismatch(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testNewHead}
	bf := writeBody(t, "rev.md", okReviewBody)

	code := run(reviewArgs("example-org/tracker", "1", "approve", testOldHead, bf))
	if code != 5 {
		t.Fatalf("exit = %d, want 5 (refused) — a real head race is not a form error", code)
	}
	e := lastAudit(t)
	if !strings.Contains(e.Detail, "current head") {
		t.Fatalf("audit detail %q does not report the head mismatch — the form gate must "+
			"not have swallowed the case it was carved out of", e.Detail)
	}
	if f.postedReview != 0 {
		t.Fatal("posted a review at a stale head")
	}
}

// TestHeadFormErrorWordingDiscriminates pins the two messages apart at the unit level,
// so a future edit cannot collapse them into one generic string. The abbreviated case is
// the one worth its own wording: every other malformed value is obviously not a SHA,
// while an abbreviated one looks correct and is what people actually type.
func TestHeadFormErrorWordingDiscriminates(t *testing.T) {
	abbrev := headFormError(testHead[:8])
	if !strings.Contains(abbrev, "ABBREVIATED") || !strings.Contains(abbrev, "8 chars") {
		t.Fatalf("abbreviated-head message %q does not name the abbreviation and its length", abbrev)
	}
	other := headFormError("origin/main")
	if strings.Contains(other, "ABBREVIATED") {
		t.Fatalf("non-SHA message %q claims abbreviation", other)
	}
	for _, m := range []string{abbrev, other} {
		if !strings.Contains(m, "headRefOid") {
			t.Fatalf("message %q does not tell the caller how to GET the full SHA", m)
		}
	}
}
