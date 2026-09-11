package main

// checkonlycr_test.go — the ONE exemption to rule 2 (the standing-CHANGES_REQUESTED block).
//
// Rule 2 refuses an APPROVE posted at an unchanged head over a standing CR: nothing new
// exists to verify. The exemption covers the one case that default had no path for — a CR
// whose SOLE finding was a required CHECK being red, where the check then turned green at
// the SAME head with no code push (a human applying `changelog:skip`, a flaked job re-run).
//
// Five conditions must ALL hold. There is one accepting test and one refusing test per
// condition, because the value of this exemption is entirely in what it does NOT accept: a
// grant that fires on four of five conditions is the laundering hole rule 2 exists to close.
// Every refusing case below starts from the ACCEPTING fixture and drops exactly one
// condition, so a refusal can only be attributed to the condition that case dropped.

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	// clearedRunID is the check-run id the reviewer cites. It is a RUN id, not a check
	// name: what the reviewer is asserting is that one specific EXECUTION went green.
	clearedRunID = "41234567890"
	// checkOnlyCRAt / reApproveAt bracket the run's completion — the CR first, the run's
	// green next, the re-approve last. That ordering IS the exemption's claim.
	checkOnlyCRAt  = "2026-01-01T00:00:00Z"
	runCompletedAt = "2026-01-01T00:10:00Z"
	reApproveAt    = "2026-01-01T00:20:00Z"
)

// checkOnlyStub builds the fully-accepting fixture: a check-only CR at head, a later
// re-approve at the SAME head citing the run, and the named run green in the rollup at that
// head, completed between the two.
//
// The `changelog` run is ADDED to the stub's green rollup rather than replacing it, so the
// checks-green condition is satisfied by the ordinary fixture and a refusal below can never
// be the CI gate wearing the exemption's clothes.
func checkOnlyStub(t *testing.T) *stub {
	t.Helper()
	s := newStub()
	s.rollup = append(s.rollup, rollupEntry{
		ID: clearedRunID, Name: "changelog", Status: "COMPLETED", Conclusion: "SUCCESS",
		StartedAt: checkOnlyCRAt, CompletedAt: runCompletedAt,
	})
	s.reviews = []reviewInfo{
		checkOnlyCR(t, "Blocked-On-Check: changelog", checkOnlyCRAt),
		citingApprove(t, "Cleared-Check-Run: "+clearedRunID, reApproveAt),
	}
	return s
}

func checkOnlyCR(t *testing.T, body, at string) reviewInfo {
	t.Helper()
	r := reviewInfo{State: "CHANGES_REQUESTED", CommitID: headSHA, Body: body, SubmittedAt: at}
	r.User.Login = reviewerBot(t)
	return r
}

func citingApprove(t *testing.T, body, at string) reviewInfo {
	t.Helper()
	r := reviewInfo{State: "APPROVED", CommitID: headSHA, Body: body, SubmittedAt: at}
	r.User.Login = reviewerBot(t)
	return r
}

// THE ACCEPTING CASE. All five conditions hold, so the standing CR is cleared and the flip
// runs. This is the only shape the exemption grants, and it is the test that was RED before
// the exemption existed.
func TestCheckOnlyCRClearedByACitedLaterGreenRun(t *testing.T) {
	s := checkOnlyStub(t)
	s.install(t)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
		t.Fatalf("check-only CR cleared by a cited later-green run: rc = %d, want %d (flip)",
			rc, deskkit.ExitOK)
	}
	if !s.flipped() {
		t.Errorf("the exemption held but the ready mutation never ran: %v", s.requests)
	}
}

// (a) DROPPED — the CR states findings in prose instead of declaring `Blocked-On-Check:`.
// Detection is by the body's explicit shape and nothing else: a CR that merely MENTIONS the
// check by name, in a sentence, has declared nothing. This is the case that keeps the
// exemption from being reachable by an ordinary CR that happens to discuss a check.
func TestProseAboutTheCheckIsNotACheckOnlyDeclaration(t *testing.T) {
	s := checkOnlyStub(t)
	s.reviews[0] = checkOnlyCR(t,
		"The only thing blocking this is the changelog check being red. Nothing else.", checkOnlyCRAt)
	s.install(t)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("prose-only CR: rc = %d, want %d — the exemption reads a declaration, never prose",
			rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("a prose-only CR produced mutations: %v", m)
	}
}

// (a) DROPPED, the fenced form. A `Blocked-On-Check:` line quoted inside a fenced code block
// is documentation explaining the format — a review that shows a colleague how to write the
// declaration must not thereby make one. Both exemption reads are grant-direction, so both
// skip fences.
func TestFencedDeclarationIsNotADeclaration(t *testing.T) {
	s := checkOnlyStub(t)
	s.reviews[0] = checkOnlyCR(t,
		"Write it like this next time:\n\n```\nBlocked-On-Check: changelog\n```\n\nAlso: the retry loop is wrong.",
		checkOnlyCRAt)
	s.install(t)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("fenced declaration: rc = %d, want %d — a quoted marker is not a claim",
			rc, deskkit.ExitRefused)
	}
}

// (a) DROPPED, the ambiguous form. Two declarations naming different checks leave the claim
// unestablished, and an unestablished claim withholds the grant rather than having one of
// its two values picked for it.
func TestTwoDisagreeingDeclarationsEstablishNothing(t *testing.T) {
	s := checkOnlyStub(t)
	s.reviews[0] = checkOnlyCR(t,
		"Blocked-On-Check: changelog\nBlocked-On-Check: go-test", checkOnlyCRAt)
	s.install(t)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("two disagreeing declarations: rc = %d, want %d", rc, deskkit.ExitRefused)
	}
}

// (b) DROPPED — the re-approve cites NO run. This is the bare at-head re-approval rule 2 was
// written for, and it must keep refusing exactly as it always did: the declaration on the CR
// does not, on its own, make the next APPROVE a re-verification of anything.
func TestReApproveCitingNoRunIsStillRefused(t *testing.T) {
	s := checkOnlyStub(t)
	s.reviews[1] = citingApprove(t, "green now, flipping", reApproveAt)
	s.install(t)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("re-approve citing no run: rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("an uncited re-approve produced mutations: %v", m)
	}
}

// (b) DROPPED, the OLDER-run form. The cited run completed BEFORE the CR was submitted, so
// the reviewer already had that result in front of them when they blocked. Citing it
// re-verifies nothing — this is the laundering shape the exemption would otherwise open, and
// the timestamp comparison is the only thing standing between the two.
func TestCitedRunThatPredatesTheCRIsRefused(t *testing.T) {
	s := checkOnlyStub(t)
	s.rollup[len(s.rollup)-1].CompletedAt = "2025-12-31T23:00:00Z" // before checkOnlyCRAt
	s.install(t)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("cited run older than the CR: rc = %d, want %d — a result the reviewer already had "+
			"when they blocked re-verifies nothing", rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("a stale cited run produced mutations: %v", m)
	}
}

// (b) DROPPED, the NOT-GREEN form. The reviewer cites a run of the right check, at the right
// head, later than the CR — but it is the FAILED run, superseded by a green re-run of the same
// name. Citing it is citing a red result.
//
// THE SUPERSEDED SHAPE IS WHAT MAKES THIS TESTABLE AT ALL. A cited run that is simply red
// also reddens the rollup, so the flip would refuse on checks-green whatever this condition
// did, and the case could not tell the two apart. Here checks-green reduces to the LATEST run
// per name — the green re-run — and passes, so the ONLY thing that can refuse is the
// exemption's own test of the run that was actually cited.
func TestCitedRunThatIsNotGreenIsRefused(t *testing.T) {
	s := checkOnlyStub(t)
	s.rollup[len(s.rollup)-1].Conclusion = "FAILURE" // the cited run failed...
	s.rollup = append(s.rollup, rollupEntry{         // ...and a later re-run went green
		ID: "41234567891", Name: "changelog", Status: "COMPLETED", Conclusion: "SUCCESS",
		StartedAt: runCompletedAt, CompletedAt: reApproveAt,
	})
	s.install(t)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("cited run is the superseded FAILED run: rc = %d, want %d — a citation is not a verdict",
			rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("citing a failed run produced mutations: %v", m)
	}
}

// THE ISSUE'S OWN SCENARIO, end to end. A required `changelog` check was red; a human applied
// `changelog:skip`; the check re-reported at the SAME head as SKIPPED. GitHub reports that
// conclusion for a check that deliberately did no work, and checks-green has always counted
// it green — so the exemption must too, or it would refuse the exact case it was authorized
// for while the gate two conditions below called the same run green.
func TestSkippedIsGreenForTheExemptionJustAsItIsForChecksGreen(t *testing.T) {
	s := checkOnlyStub(t)
	s.rollup[len(s.rollup)-1].Conclusion = "SKIPPED"
	s.install(t)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
		t.Fatalf("cited run concluded SKIPPED: rc = %d, want %d — a skip label is how the real case goes "+
			"green, and the exemption shares checks-green's accepted set", rc, deskkit.ExitOK)
	}
	if !s.flipped() {
		t.Errorf("a SKIPPED cited run did not flip: %v", s.requests)
	}
}

// (b) DROPPED, the NOT-YET-COMPLETE form. The cited run is the named check at the right head,
// but it is still RUNNING. "Still going" is could-not-check, and could-not-check never clears
// a standing rejection — the reviewer cited a result that does not exist yet.
func TestCitedRunStillRunningIsRefused(t *testing.T) {
	s := checkOnlyStub(t)
	s.rollup[len(s.rollup)-1].Status = "IN_PROGRESS"
	s.rollup[len(s.rollup)-1].Conclusion = ""
	s.install(t)

	// Refused, NOT unverifiable: the exemption decides before checks-green ever runs, so a
	// pending rollup is not what produced this. That distinction is the assertion — with the
	// completed-test dropped the exemption grants and the flip falls through to checks-green,
	// which returns could-not-verify instead.
	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("cited run still running: rc = %d, want %d (refused by the exemption, not deferred to CI)",
			rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("citing an unfinished run produced mutations: %v", m)
	}
}

// (c) DROPPED — the cited run is not in the rollup AT THIS HEAD. The exemption's head
// condition is structural rather than compared: the rollup is read at the head both reviews
// are pinned to, so a run belonging to another head is simply absent, and an absent run is
// no evidence about this head.
func TestCitedRunAbsentFromTheHeadRollupIsRefused(t *testing.T) {
	s := checkOnlyStub(t)
	s.rollup = s.rollup[:len(s.rollup)-1] // the run lives on some other head, not this one
	s.install(t)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("cited run absent at head: rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("a run absent from the head rollup produced mutations: %v", m)
	}
	// The read that decided it must have been addressed to the reviewed head. If it were
	// addressed anywhere else, "absent at head" would be an accident of the stub rather than
	// the property this case asserts.
	if !s.saw("GET", "/commits/"+headSHA+"/check-runs") {
		t.Errorf("the exemption's rollup read was not addressed to the reviewed head: %v", s.requests)
	}
}

// (d) DROPPED — the cited run is green, later, and at head, but it is a DIFFERENT check from
// the one the CR named. Without this the exemption would let any green run on the head clear
// a CR blocked on some other check.
func TestCitedRunForADifferentCheckIsRefused(t *testing.T) {
	s := checkOnlyStub(t)
	s.rollup[len(s.rollup)-1].Name = "go-test" // the CR named `changelog`
	s.install(t)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("cited run for another check: rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("a mismatched cited run produced mutations: %v", m)
	}
}

// The exemption clears a BLOCK; it does not manufacture a VERDICT. A check-only CR, cleared
// by a citing re-approve, followed by a SECOND ordinary CR at the same head still refuses —
// the reduction below rule 2 has to find an APPROVED governing at head, and here it does not.
func TestClearingTheCheckOnlyCRDoesNotSurviveALaterCR(t *testing.T) {
	s := checkOnlyStub(t)
	later := checkOnlyCR(t, "the retry loop drops the last error", "2026-01-01T00:30:00Z")
	s.reviews = append(s.reviews, later)
	s.install(t)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("a later ordinary CR: rc = %d, want %d — the exemption removes a block, it does not "+
			"supply an approval", rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("a later ordinary CR produced mutations: %v", m)
	}
}

// A SECURITY-marked APPROVE must not clear a CORRECTNESS block, for the same reason it
// cannot satisfy the correctness gate: the two verdicts are separate artifacts, and clearing
// a correctness block is acting in the correctness lane.
func TestSecurityMarkedApproveDoesNotClearTheCheckOnlyCR(t *testing.T) {
	s := checkOnlyStub(t)
	s.reviews[1] = citingApprove(t,
		"Security-Review: pass\nCleared-Check-Run: "+clearedRunID, reApproveAt)
	s.install(t)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("security-marked citing approve: rc = %d, want %d", rc, deskkit.ExitRefused)
	}
}

// A run the forge served NO id for maps to "" and is unmatchable. A body citing `0` — the
// value an id-less run would carry if the mapping rendered zero as a number — must not match
// it. This pins the fail-closed half of the id mapping at the gate that consumes it.
func TestCitationOfZeroDoesNotMatchAnIdlessRun(t *testing.T) {
	s := checkOnlyStub(t)
	s.rollup[len(s.rollup)-1].ID = "" // the forge reported no id for this run
	s.reviews[1] = citingApprove(t, "Cleared-Check-Run: 0", reApproveAt)
	s.install(t)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("citation of 0 against an id-less run: rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("a citation of 0 produced mutations: %v", m)
	}
}

// A SHORT rollup read is could-not-check, and could-not-check never clears a standing
// rejection: the cited run may simply be in the part the forge did not serve.
//
// THE EXIT CODE ALONE CANNOT PIN THIS. checks-green has its own short-read guard over the
// same endpoint and returns the same could-not-verify, so a run that reached it would look
// identical from the outside. What distinguishes them is WHICH CONDITION reported, and the
// answer has to be reviewer-approved: the exemption decides first, and an operator sent to
// the CI gate for a rejection that was never about CI goes looking in the wrong place.
func TestShortRollupReadCannotClearTheCheckOnlyCR(t *testing.T) {
	s := checkOnlyStub(t)
	s.checkTotalOverride = 99 // the forge asserts far more runs than it served
	s.install(t)

	var rc int
	out := captureStderr(t, func() { rc = run([]string{"7", "--repo", privateCIRepo}) })

	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("short rollup read: rc = %d, want %d (could-not-check)", rc, deskkit.ExitUnverifiable)
	}
	if !strings.Contains(out, "condition "+condReviewerApproved) {
		t.Errorf("the short read was reported by the wrong condition — want %s, got:\n%s",
			condReviewerApproved, out)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("a short rollup read produced mutations: %v", m)
	}
}
