package main

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ---------------------------------------------------------------------------
// #216 — the security verdict is ORDER-SENSITIVE.
//
// The gate used to look only for a `Security-Review: pass` line and never parsed
// `fail` at all, so a retraction at the same head was invisible and the PR stayed
// flippable. These are the tests whose absence let that survive.
// ---------------------------------------------------------------------------

// TestReadySecurityPassThenFailRefuses — pass, then fail, then a plain re-approval, all
// at the SAME head. The last security verdict is `fail`, so the gate must REFUSE.
func TestReadySecurityPassThenFailRefuses(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"secrets/prod/token.yaml"} // risk-classed
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, "## Security review\n\nLooks clean.\n\nSecurity-Review: pass\n"),
		appReview("CHANGES_REQUESTED", testHead, "## Security review\n\nRetracting: leaks via the shared reader.\n\nSecurity-Review: fail\n"),
		appReview("APPROVED", testHead, okReviewBody), // correctness re-approval, no security verdict
	}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("pass→fail→approve exit = %d, want %d (ExitRefused)", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0 — a retracted security pass must not flip", f.flips)
	}
}

// TestReadySecurityFailThenPassFlips — the mirror image: the last verdict at head is
// `pass`, so an earlier `fail` must not block forever.
func TestReadySecurityFailThenPassFlips(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"secrets/prod/token.yaml"}
	f.reviews = []reviewInfo{
		// The correctness verdict is now REQUIRED in its own lane (#238 direction 3).
		// Before that split this fixture had none, and the security APPROVED doubled as
		// the correctness verdict — which is precisely the fail-open being closed.
		appReview("APPROVED", testHead, okReviewBody),
		appReview("CHANGES_REQUESTED", testHead, "## Security review\n\nBlocker.\n\nSecurity-Review: fail\n"),
		appReview("APPROVED", testHead, "## Security review\n\nFixed.\n\nSecurity-Review: pass\n"),
	}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != 0 {
		t.Fatalf("fail→pass exit = %d, want 0", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1 — the last verdict at head governs", f.flips)
	}
}

// TestReadySecurityAmbiguousBodyRefuses — one body carrying BOTH markers is ambiguous.
// The gate fails closed: a `pass` line smuggled next to a `fail` is not a pass.
func TestReadySecurityAmbiguousBodyRefuses(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"secrets/prod/token.yaml"}
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead,
			"## Security review\n\nSecurity-Review: fail\n\nbut on reflection\n\nSecurity-Review: pass\n"),
	}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("ambiguous body exit = %d, want %d (ExitRefused)", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0 — an ambiguous security body must fail closed", f.flips)
	}
}

// TestReadySecurityFailAtOldHeadIgnored — a `fail` at a SUPERSEDED head does not veto a
// `pass` at the current head; only verdicts at head are reduced.
func TestReadySecurityFailAtOldHeadIgnored(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"secrets/prod/token.yaml"}
	f.reviews = []reviewInfo{
		appReview("CHANGES_REQUESTED", testOldHead, "## Security review\n\nSecurity-Review: fail\n"),
		appReview("APPROVED", testHead, okReviewBody), // correctness lane (#238 direction 3)
		appReview("APPROVED", testHead, "## Security review\n\nSecurity-Review: pass\n"),
	}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != 0 {
		t.Fatalf("old-head fail exit = %d, want 0", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1", f.flips)
	}
}

// TestSecurityPassAtHeadReduction pins the reducer itself, independent of the gate.
func TestSecurityPassAtHeadReduction(t *testing.T) {
	pass := "Security-Review: pass"
	fail := "Security-Review: fail"
	cases := []struct {
		name    string
		bodies  []string
		heads   []string
		want    bool
		wantWhy string
	}{
		{"no verdict at all", []string{"## Review\n\nVerdict: approve"}, []string{"H"}, false, "absence is not a pass"},
		{"single pass", []string{pass}, []string{"H"}, true, ""},
		{"single fail", []string{fail}, []string{"H"}, false, ""},
		{"pass then fail", []string{pass, fail}, []string{"H", "H"}, false, "retraction governs"},
		{"fail then pass", []string{fail, pass}, []string{"H", "H"}, true, "last verdict governs"},
		{"pass, fail, silent", []string{pass, fail, "## Review\n\nVerdict: approve"}, []string{"H", "H", "H"}, false, "a silent review neither grants nor retracts"},
		{"both markers in one body", []string{fail + "\n" + pass}, []string{"H"}, false, "ambiguity fails closed"},
		{"fail at old head only", []string{fail, pass}, []string{"OLD", "H"}, true, ""},
		{"pass at old head only", []string{pass}, []string{"OLD"}, false, "a pass at a superseded head is not at head"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var rv []reviewInfo
			for i, b := range c.bodies {
				rv = append(rv, appReview("APPROVED", c.heads[i], b))
			}
			if got := securityPassAtHead(rv, "H"); got != c.want {
				t.Fatalf("securityPassAtHead = %v, want %v (%s)", got, c.want, c.wantWhy)
			}
		})
	}
}

// TestSecurityVerdictIgnoresNonAppAuthor — only the reviewer App's verdicts count; a
// forged `Security-Review: pass` from any other login is not a verdict.
func TestSecurityVerdictIgnoresNonAppAuthor(t *testing.T) {
	r := reviewInfo{State: "APPROVED", CommitID: "H", Body: "Security-Review: pass"}
	r.User.Login = "some-random-user"
	if securityPassAtHead([]reviewInfo{r}, "H") {
		t.Fatal("a non-App review must never supply the security verdict")
	}
}

// TestReadySecurityOrderPassFailApproveStaysRefused is the literal pass→fail→approve
// ordering proof: a `pass`, then a `fail`
// retraction, then a plain re-approval carrying no security verdict, all at the SAME
// head, must never read as green — the LAST security verdict at head (or any fail)
// governs, not the last review's approval state. Named so `-run Order` selects it
// directly; TestReadySecurityPassThenFailRefuses above proves the identical scenario
// under the name the ready-gate fix (PR #219, #216) was authored with.
func TestReadySecurityOrderPassFailApproveStaysRefused(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"secrets/prod/token.yaml"} // risk-classed
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, "## Security review\n\nLooks clean.\n\nSecurity-Review: pass\n"),
		appReview("CHANGES_REQUESTED", testHead, "## Security review\n\nRetracting: leaks via the shared reader.\n\nSecurity-Review: fail\n"),
		appReview("APPROVED", testHead, okReviewBody), // correctness re-approval, no security verdict
	}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("pass→fail→approve exit = %d, want %d (ExitRefused) — order-sensitivity must govern, not the last review's plain APPROVED state", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0 — a retracted security pass must not flip", f.flips)
	}
}

// TestSecurityVerdictEmptyHeadNeverGrants pins head-binding at its degenerate input.
//
// securityVerdictStanding head-pins a PASS with `r.CommitID != head`. That is a plain string
// compare, so if `head` is ever the empty string it stops rejecting anything carrying an
// empty commit_id, and every head comparison in the gate goes vacuous simultaneously — a
// verdict would bind to no commit at all and still grant. gate (a) refuses a malformed PR
// payload (state != "open") before runReady reaches this, so it is depth rather than a
// live bypass; it is pinned because head-binding is the property gate (e) rests on, and
// "holds for every input we happened to test" is not that property.
//
// The FAIL half is the asymmetry securityGrantOf is built on, and is asserted here too: an
// unbound retraction must still BLOCK. Doubt resolves toward blocking, never toward a grant.
func TestSecurityVerdictEmptyHeadNeverGrants(t *testing.T) {
	t.Run("unbound pass grants nothing", func(t *testing.T) {
		rv := []reviewInfo{appReview("COMMENTED", "", "Security-Review: pass")}
		if securityPassAtHead(rv, "") {
			t.Fatal("an empty head matched an empty commit_id and GRANTED — head-binding is vacuous")
		}
	})
	t.Run("unbound pass grants nothing on APPROVED either", func(t *testing.T) {
		rv := []reviewInfo{appReview("APPROVED", "", "Security-Review: pass")}
		if securityPassAtHead(rv, "") {
			t.Fatal("an empty head granted via the legacy APPROVED shape")
		}
	})
	t.Run("unbound fail still blocks", func(t *testing.T) {
		rv := []reviewInfo{appReview("CHANGES_REQUESTED", "", "Security-Review: fail")}
		if got := securityVerdictStanding(rv, ""); got != secFail {
			t.Fatalf("securityVerdictStanding = %v, want secFail — an unbound retraction must still block", got)
		}
	})
	t.Run("a bound pass at a real head is unaffected", func(t *testing.T) {
		rv := []reviewInfo{appReview("COMMENTED", testHead, "Security-Review: pass")}
		if !securityPassAtHead(rv, testHead) {
			t.Fatal("the empty-head guard must not withhold a genuine bound pass")
		}
	})
}

// ---------------------------------------------------------------------------
// #361 companion (deskflip twin: PR #529) — a `Security-Review: fail` STANDS across a
// content-preserving head move on the deskpost ready path.
//
// The reduction used to skip EVERY review whose commit_id was not the current head
// (`r.CommitID != head`), so a fail posted at head A read as not-current after a resync to
// head B and gate (e0) no longer refused the flip. A fail is a retraction of the reviewed
// CODE, not of a commit sha; a resync / merge-from-main that leaves the flagged code
// byte-identical must not launder it. Only a later `Security-Review: pass` AT THE CURRENT
// head — or a genuine content change a reviewer re-reviews and passes — clears it. A pass
// keeps its at-head binding, so the two verdict kinds stay asymmetric, both fail safe.
//
// These run on exampleRepo with the harness's default NON-risk changed file, so gate (e)
// (the risk-classed pass requirement) never fires: the ONLY thing that can block the flip
// is the standing fail itself (gate (e0)) — a clean isolation of the laundering bypass.
// ---------------------------------------------------------------------------

// TestReadySecurityFailStandsAcrossAContentPreservingHeadMove — THE BUG THIS BRANCH EXISTS
// FOR. A `Security-Review: fail` posted at the superseded head (head A) must STILL BLOCK the
// ready-flip after a content-preserving resync moved the head to testHead (head B). The
// correctness approval is at the CURRENT head (a resync re-ran the reviewer), so nothing but
// the standing fail is left to refuse.
func TestReadySecurityFailStandsAcrossAContentPreservingHeadMove(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, okReviewBody), // correctness current at head B
		appReview("CHANGES_REQUESTED", testOldHead, "## Security review\n\nSecurity-Review: fail\n"),
	}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("standing fail after a content-preserving resync exit = %d, want %d (ExitRefused) — "+
			"a head move that did not touch the flagged code must not launder the fail", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0 — a resync flipped a PR over a standing Security-Review: fail", f.flips)
	}
}

// TestReadyStalePassDoesNotClearAStandingFailAfterAResync — a `pass` at the OLD head is not a
// re-review of the current code, so it neither stands as a pass nor clears a standing fail.
// fail@A then pass@A, head now B: at B the pass is stale and the fail still stands.
func TestReadyStalePassDoesNotClearAStandingFailAfterAResync(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, okReviewBody), // correctness current at head B
		appReview("CHANGES_REQUESTED", testOldHead, "## Security review\n\nSecurity-Review: fail\n"),
		appReview("APPROVED", testOldHead, "## Security review\n\nSecurity-Review: pass\n"), // stale pass at head A
	}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("stale pass across a resync exit = %d, want %d (ExitRefused) — a pass at the old head "+
			"must not clear a standing fail", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0 — the PR flipped over a fail only a STALE pass tried to clear", f.flips)
	}
}

// TestReadyGenuineNewPassAtHeadClearsAStandingFail — the clearing side that keeps the
// standing-fail rule from being a one-way trap: a GENUINE `Security-Review: pass` AT THE
// CURRENT head clears a fail posted at the old head, so the flip proceeds. fail@A + pass@B
// (== head) reduces to a pass.
func TestReadyGenuineNewPassAtHeadClearsAStandingFail(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, okReviewBody), // correctness current at head B
		appReview("CHANGES_REQUESTED", testOldHead, "## Security review\n\nSecurity-Review: fail\n"),
		appReview("APPROVED", testHead, "## Security review\n\nRe-reviewed the current code.\n\nSecurity-Review: pass\n"),
	}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != 0 {
		t.Fatalf("a fresh pass at head exit = %d, want 0 — it must clear the standing fail", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1 — a genuine new pass at head did not clear the fail", f.flips)
	}
}

// TestSecurityFailStandsRegardlessOfCommitReduction pins the property at the reducer
// boundary, independent of the gate: a fail at a superseded head governs the reduction with
// no pass to clear it, and a fresh pass at head reduces the same inputs back to a pass.
func TestSecurityFailStandsRegardlessOfCommitReduction(t *testing.T) {
	fail := appReview("CHANGES_REQUESTED", testOldHead, "Security-Review: fail")
	if got := securityVerdictStanding([]reviewInfo{fail}, testHead); got != secFail {
		t.Fatalf("fail at old head, head moved: got %v, want secFail — a content-preserving move "+
			"must not launder a standing fail", got)
	}
	stalePass := appReview("APPROVED", testOldHead, "Security-Review: pass")
	if got := securityVerdictStanding([]reviewInfo{fail, stalePass}, testHead); got != secFail {
		t.Fatalf("fail@old + stale pass@old: got %v, want secFail — a stale pass clears nothing", got)
	}
	freshPass := appReview("APPROVED", testHead, "Security-Review: pass")
	if got := securityVerdictStanding([]reviewInfo{fail, freshPass}, testHead); got != secPass {
		t.Fatalf("fail@old + fresh pass@head: got %v, want secPass — a pass at head clears the fail", got)
	}
}
