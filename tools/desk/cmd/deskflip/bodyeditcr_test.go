package main

// bodyeditcr_test.go — rule 2's SECOND exemption, the documented body-edit re-verification
// class, driven end to end through the verb against the fake forge.
//
// The accepting case flips. Every near-miss starts from the accepting fixture and changes one
// thing, and must refuse with NO mutation: a different exemption class, an undocumented
// re-approve, a code finding still standing at head, a body that moved after the re-read, a
// body the forge records as not edited after the CR, an edit time that cannot be read, a
// security-lane approve (alone, and beside a plain correctness approve), and CI red at head
// (the flip gate's own mechanical check, which the class never replaces).

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	bodyBeforeEdit = "Summary: the example check FALSE-PASSES.\n\nNo trailer in this fixture."
	bodyAfterEdit  = "Summary: the example check passes on the corrected fixture.\n\nNo trailer in this fixture."
	bodyFindingID  = "f-body-1"
	bodyCRAt       = "2026-01-01T00:00:00Z"
	bodyEditedAt   = "2026-01-01T00:10:00Z" // the forge's own record of the body edit: after the CR
	bodyApproveAt  = "2026-01-01T00:20:00Z"
)

func bodyEditCRBody() string {
	return "The PR body still asserts the retracted claim.\n\nBlocked-On-Body: " + bodyFindingID + " " +
		deskkit.PRBodyDigest(bodyBeforeEdit)
}

func bodyEditApproveBody() string {
	return "Re-read the live PR body via the API; the retracted claim is gone.\n\n" +
		"Resolved-Body-Finding: " + bodyFindingID + "\n" +
		"Body-Reread-Digest: " + deskkit.PRBodyDigest(bodyAfterEdit) + "\n" +
		"CI-Green-At: " + headSHA
}

// bodyEditStub is the fully-accepting fixture: a body-only CR at head, the body since edited,
// and a later same-head APPROVE documenting all three elements against the live body.
func bodyEditStub(t *testing.T) *stub {
	t.Helper()
	s := newStub()
	s.pr.Body = bodyAfterEdit
	s.bodyEditedAt = bodyEditedAt
	s.reviews = []reviewInfo{
		checkOnlyCR(t, bodyEditCRBody(), bodyCRAt),
		citingApprove(t, bodyEditApproveBody(), bodyApproveAt),
	}
	return s
}

func wantBodyEditRefused(t *testing.T, s *stub, why string) {
	t.Helper()
	s.install(t)
	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("%s: rc = %d, want %d (refused)", why, rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("%s: produced mutations %v", why, m)
	}
}

// THE ACCEPTING CASE — the ruled class flips. RED before the exemption existed: rule 2
// refused every same-head APPROVE over a standing CR that did not declare Blocked-On-Check.
func TestBodyEditCRCleared(t *testing.T) {
	s := bodyEditStub(t)
	s.install(t)
	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
		t.Fatalf("documented body-edit re-verification: rc = %d, want %d (flip)", rc, deskkit.ExitOK)
	}
	if !s.flipped() {
		t.Errorf("the exemption held but the ready mutation never ran: %v", s.requests)
	}
}

// DIFFERENT CLASS — an ordinary CR that declared nothing; a fully documented approve still
// re-verifies nothing the CR claimed, so rule 2's refusal stands.
func TestBodyEdit_OrdinaryCRNotExempt(t *testing.T) {
	s := bodyEditStub(t)
	s.reviews[0] = checkOnlyCR(t, "The PR body still asserts the retracted claim.", bodyCRAt)
	wantBodyEditRefused(t, s, "undeclared CR + documented approve")
}

// DIFFERENT CLASS — a CHECK-ONLY CR answered with body-edit documentation.
func TestBodyEdit_CheckOnlyCRNotClearedByBodyDocs(t *testing.T) {
	s := bodyEditStub(t)
	s.reviews[0] = checkOnlyCR(t, "Blocked-On-Check: changelog", bodyCRAt)
	wantBodyEditRefused(t, s, "check-only CR + body-edit documentation")
}

// UNDOCUMENTED EDIT — the body was edited, but the re-approve documents nothing.
func TestBodyEdit_UndocumentedReApprove(t *testing.T) {
	s := bodyEditStub(t)
	s.reviews[1] = citingApprove(t, "body fixed, looks good now", bodyApproveAt)
	wantBodyEditRefused(t, s, "undocumented same-head re-approve")
}

// CODE CHANGE STILL NEEDED — a second CR at the same head carries a code finding. Every
// standing CR at head must clear on its own; the body-edit class clears only the body one.
func TestBodyEdit_CodeFindingCRStillStands(t *testing.T) {
	s := bodyEditStub(t)
	s.reviews = []reviewInfo{
		checkOnlyCR(t, "The retry loop never exits — needs a code change.", "2025-12-31T23:00:00Z"),
		checkOnlyCR(t, bodyEditCRBody(), bodyCRAt),
		citingApprove(t, bodyEditApproveBody(), bodyApproveAt),
	}
	wantBodyEditRefused(t, s, "code-finding CR at head beside the body CR")
}

// THE BODY MOVED between the gate's first read and the pre-mutation re-read: the documented
// re-read is no longer of the live body, so the post-TOCTOU re-check refuses.
func TestBodyEdit_BodyEditedAgainBeforeMutation(t *testing.T) {
	s := bodyEditStub(t)
	s.body2 = bodyAfterEdit + "\n\nA new, unreviewed claim."
	wantBodyEditRefused(t, s, "body edited between reads")
}

// SECURITY LANE — a security-marked approve never acts in the correctness lane.
func TestBodyEdit_SecurityLaneApproveDoesNotClear(t *testing.T) {
	s := bodyEditStub(t)
	s.reviews[1] = citingApprove(t, bodyEditApproveBody()+"\nSecurity-Review: pass", bodyApproveAt)
	wantBodyEditRefused(t, s, "security-lane approve")
}

// CI RED AT HEAD — the approve CITES green, but the flip gate's own checks-green condition
// reads the rollup and refuses. The citation never substitutes for the mechanical check.
func TestBodyEdit_CIRedStillRefuses(t *testing.T) {
	s := bodyEditStub(t)
	s.rollup = []rollupEntry{{Name: "test", Status: "COMPLETED", Conclusion: "FAILURE"}}
	s.install(t)
	rc := run([]string{"7", "--repo", privateCIRepo})
	if rc == deskkit.ExitOK || s.flipped() {
		t.Fatalf("CI red at head with a documented approve: rc = %d flipped=%v, want a refusal", rc, s.flipped())
	}
}

// The refusal for a claimed-but-failed class names the clause, so the reviewer can fix it.
func TestBodyEditRefusalNamesTheClause(t *testing.T) {
	err := bodyEditCRCleared("reviewer[bot]",
		reviewInfo{State: "CHANGES_REQUESTED", CommitID: headSHA, Body: bodyEditCRBody(), SubmittedAt: bodyCRAt},
		nil, headSHA, liveBodyRead{Body: bodyAfterEdit})
	if err == nil || !strings.Contains(err.Error(), "body-edit re-verification class") {
		t.Fatalf("err = %v, want a refusal naming the body-edit class", err)
	}
}

// NOT EDITED, BY THE FORGE'S RECORD — the body is exactly what the CR blocked on, but the CR
// recorded a digest from a slipped recipe (here over the body plus one stray byte, as hashing
// jq's trailing newline produces). The approve's re-read digest honestly matches the live
// body and so DIFFERS from the slipped CR digest; the digest inequality alone would read that
// as "edited". The forge's own edit record is what refuses it: never edited, or last edited
// before the CR.
func TestBodyEdit_UneditedBodyWithMismatchedCRDigest(t *testing.T) {
	unedited := func(t *testing.T, editedAt string) *stub {
		s := bodyEditStub(t)
		s.pr.Body = bodyBeforeEdit
		s.bodyEditedAt = editedAt
		s.reviews = []reviewInfo{
			checkOnlyCR(t, "The PR body still asserts the retracted claim.\n\nBlocked-On-Body: "+bodyFindingID+" "+
				deskkit.PRBodyDigest(bodyBeforeEdit+"x"), bodyCRAt),
			citingApprove(t, strings.Replace(bodyEditApproveBody(), deskkit.PRBodyDigest(bodyAfterEdit),
				deskkit.PRBodyDigest(bodyBeforeEdit), 1), bodyApproveAt),
		}
		return s
	}
	t.Run("forge reports no edit", func(t *testing.T) {
		wantBodyEditRefused(t, unedited(t, ""), "unedited body, never edited per the forge")
	})
	t.Run("forge's last edit is before the CR", func(t *testing.T) {
		wantBodyEditRefused(t, unedited(t, "2025-12-31T23:00:00Z"), "unedited body, last edited before the CR")
	})
}

// COULD-NOT-CHECK — the forge's edit record cannot be read. That is never a clearance.
func TestBodyEdit_EditTimeUnreadableRefuses(t *testing.T) {
	s := bodyEditStub(t)
	s.trustErr = true
	s.install(t)
	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("unreadable body edit time: rc = %d, want %d (could-not-check)", rc, deskkit.ExitUnverifiable)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("unreadable body edit time: produced mutations %v", m)
	}
	if s.trustReads == 0 {
		t.Fatal("the edit-time read never happened — the refusal came from somewhere else")
	}
}

// SECURITY LANE, WITH A PLAIN CORRECTNESS APPROVE BESIDE IT — the shape where the body-edit
// gate's own security-marker filter is the ONLY control. A body-edit CR, then a plain,
// undocumented correctness APPROVE, then a documented APPROVE that also carries a
// security-lane line. Rule 1's lane reduction still holds the plain APPROVE as the governing
// correctness verdict, so if the body-edit gate let the security-marked APPROVE clear the CR,
// the PR would flip. It must refuse: a security-marked body never acts in the correctness lane.
func TestBodyEdit_SecurityMarkedDocumentedApproveBesidePlainApprove(t *testing.T) {
	s := bodyEditStub(t)
	s.reviews = []reviewInfo{
		checkOnlyCR(t, bodyEditCRBody(), bodyCRAt),
		citingApprove(t, "looks fine now", "2026-01-01T00:15:00Z"),
		citingApprove(t, bodyEditApproveBody()+"\nSecurity-Review: pass", bodyApproveAt),
	}
	wantBodyEditRefused(t, s, "security-marked documented approve beside a plain approve")
}
