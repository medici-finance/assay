package deskkit

// bodyeditcr_test.go — the documented body-edit re-verification class, decided in isolation.
//
// ONE accepting case, then one refusing case per clause. Every refusing case starts from the
// accepting fixture and changes exactly one thing, so a refusal can only be attributed to
// the clause that case changed — the value of this exemption is entirely in what it does
// NOT accept.

import (
	"strings"
	"testing"
)

const (
	beHead       = "aaaaaaaabbbbbbbbccccccccdddddddd11111111"
	beOtherHead  = "1111111122222222333333334444444455555555"
	beFinding    = "f-body-1"
	beBodyBefore = "Summary: the example check FALSE-PASSES.\r\n\r\nIssue: #1\r\n"
	beBodyAfter  = "Summary: the example check passes on the corrected fixture.\n\nIssue: #1\n"
	beCRAt       = "2026-01-01T00:00:00Z"
	beApproveAt  = "2026-01-01T00:20:00Z"
)

func beCR() string {
	return "The PR body still asserts the retracted claim.\n\nBlocked-On-Body: " + beFinding + " " +
		PRBodyDigest(beBodyBefore)
}

func beApproveBody() string {
	return "Re-read the live PR body via the API; the retracted claim is gone.\n\n" +
		"Resolved-Body-Finding: " + beFinding + "\n" +
		"Body-Reread-Digest: " + PRBodyDigest(beBodyAfter) + "\n" +
		"CI-Green-At: " + beHead
}

func beInput() BodyEditInput {
	return BodyEditInput{
		CRBody:        beCR(),
		CRSubmittedAt: beCRAt,
		Head:          beHead,
		Approves:      []BodyEditApprove{{Body: beApproveBody(), SubmittedAt: beApproveAt}},
		LiveBody:      beBodyAfter,
	}
}

func TestBodyEdit_DocumentedClassIsAdmitted(t *testing.T) {
	dec := EvaluateBodyEditReverification(beInput())
	if !dec.Declared || !dec.Cleared {
		t.Fatalf("documented body-edit re-verification: declared=%v cleared=%v (%s), want both true",
			dec.Declared, dec.Cleared, dec.Reason)
	}
}

// The digest normalisation is the shell recipe's: CRs removed, trailing newlines trimmed.
func TestPRBodyDigest_MatchesRecipeNormalisation(t *testing.T) {
	if PRBodyDigest("a\r\nb\r\n\n\n") != PRBodyDigest("a\nb") {
		t.Fatal("CRLF and trailing newlines must not change the digest")
	}
	if PRBodyDigest("a\nb ") == PRBodyDigest("a\nb") {
		t.Fatal("trailing spaces are content, not normalised away")
	}
}

// refused asserts a near-miss does NOT clear, and that the reason names the expected clause.
func refused(t *testing.T, in BodyEditInput, wantReason string) {
	t.Helper()
	dec := EvaluateBodyEditReverification(in)
	if dec.Cleared {
		t.Fatalf("near-miss cleared the standing CR (%s) — the class must admit nothing wider than the ruling", dec.Reason)
	}
	if !strings.Contains(dec.Reason, wantReason) {
		t.Fatalf("reason = %q, want it to name %q", dec.Reason, wantReason)
	}
}

// DIFFERENT CLASS — an ordinary CR (no declaration) answered by a fully documented approve.
func TestBodyEdit_NearMiss_UndeclaredCR(t *testing.T) {
	in := beInput()
	in.CRBody = "The PR body still asserts the retracted claim, and the retry loop is wrong."
	refused(t, in, "no `Blocked-On-Body:` declaration")
}

// DIFFERENT CLASS — the CR declares the CHECK-ONLY class; body documentation clears nothing.
func TestBodyEdit_NearMiss_CheckOnlyCRIsNotThisClass(t *testing.T) {
	in := beInput()
	in.CRBody = "Blocked-On-Check: changelog"
	refused(t, in, "no `Blocked-On-Body:` declaration")
}

// DIFFERENT CLASS — two contradictory sole-blocker claims on one CR.
func TestBodyEdit_NearMiss_CoDeclaredWithAnotherClass(t *testing.T) {
	for _, extra := range []string{"Blocked-On-Check: changelog", "External-Prereq-Only: waiting on example-org/example#1"} {
		in := beInput()
		in.CRBody = beCR() + "\n" + extra
		refused(t, in, "two contradictory sole-blocker claims")
	}
}

// CODE CHANGE NEEDED — a mixed CR whose typed block carries a code finding beside the body one.
func TestBodyEdit_NearMiss_MixedCRWithCodeFinding(t *testing.T) {
	in := beInput()
	in.CRBody = beCR() + "\n\n" + RenderFindingBlock(FindingBlockV1{Findings: []Finding{
		{ID: beFinding, Class: "pr-body", Severity: SeverityBlocking, Blocker: BlockerCodeContent, State: StateOpen, Failure: "body claim"},
		{ID: "f-code-2", Class: "retry-loop", Severity: SeverityBlocking, Blocker: BlockerCodeContent, State: StateOpen, Failure: "loop never exits"},
	}})
	refused(t, in, "a mixed rejection is never body-edit-only")
}

// The typed block naming ONLY the declared body finding is not mixed.
func TestBodyEdit_TypedBlockWithOnlyTheBodyFindingIsAdmitted(t *testing.T) {
	in := beInput()
	in.CRBody = beCR() + "\n\n" + RenderFindingBlock(FindingBlockV1{Findings: []Finding{
		{ID: beFinding, Class: "pr-body", Severity: SeverityBlocking, Blocker: BlockerCodeContent, State: StateOpen, Failure: "body claim"},
	}})
	if dec := EvaluateBodyEditReverification(in); !dec.Cleared {
		t.Fatalf("typed block naming only the body finding: %s", dec.Reason)
	}
}

// UNDOCUMENTED EDIT — each of the three documented elements dropped in turn.
func TestBodyEdit_NearMiss_UndocumentedApprove(t *testing.T) {
	cases := map[string]string{
		"Resolved-Body-Finding": "does not document which finding",
		"Body-Reread-Digest":    "does not document a live-API re-read",
		"CI-Green-At":           "does not document CI green",
	}
	for marker, want := range cases {
		t.Run(marker, func(t *testing.T) {
			in := beInput()
			var kept []string
			for _, ln := range strings.Split(beApproveBody(), "\n") {
				if !strings.HasPrefix(ln, marker+":") {
					kept = append(kept, ln)
				}
			}
			in.Approves[0].Body = strings.Join(kept, "\n")
			refused(t, in, want)
		})
	}
	t.Run("prose only", func(t *testing.T) {
		in := beInput()
		in.Approves[0].Body = "I re-read the live body via the API, the finding is resolved, CI is green at head."
		refused(t, in, "does not document which finding")
	})
	t.Run("fenced markers are documentation, not a claim", func(t *testing.T) {
		in := beInput()
		in.Approves[0].Body = "```\n" + beApproveBody() + "\n```"
		refused(t, in, "does not document which finding")
	})
}

// NO EDIT — the body the approve re-read is the body the CR blocked on.
func TestBodyEdit_NearMiss_BodyNeverEdited(t *testing.T) {
	in := beInput()
	in.LiveBody = beBodyBefore
	in.Approves[0].Body = strings.Replace(beApproveBody(), PRBodyDigest(beBodyAfter), PRBodyDigest(beBodyBefore), 1)
	refused(t, in, "the body was not edited after the block")
}

// STALE RE-READ — the body changed again after the approve re-read it.
func TestBodyEdit_NearMiss_LiveBodyChangedSinceReRead(t *testing.T) {
	in := beInput()
	in.LiveBody = beBodyAfter + "\nAlso: a new unreviewed claim."
	refused(t, in, "is not the digest of the live PR body")
}

func TestBodyEdit_NearMiss_WrongFindingCited(t *testing.T) {
	in := beInput()
	in.Approves[0].Body = strings.Replace(beApproveBody(), "Resolved-Body-Finding: "+beFinding, "Resolved-Body-Finding: f-other", 1)
	refused(t, in, "not the finding the CR declared")
}

func TestBodyEdit_NearMiss_CIGreenAtAnotherHead(t *testing.T) {
	in := beInput()
	in.Approves[0].Body = strings.Replace(beApproveBody(), "CI-Green-At: "+beHead, "CI-Green-At: "+beOtherHead, 1)
	refused(t, in, "not the current head")

	in = beInput()
	in.Approves[0].Body = strings.Replace(beApproveBody(), "CI-Green-At: "+beHead, "CI-Green-At: "+beHead[:12], 1)
	refused(t, in, "does not document CI green")
}

func TestBodyEdit_NearMiss_ApproveNotAfterCR(t *testing.T) {
	in := beInput()
	in.Approves[0].SubmittedAt = beCRAt
	refused(t, in, "no correctness APPROVE from the reviewer at this head was submitted after it")
}

func TestBodyEdit_NearMiss_MalformedDeclaration(t *testing.T) {
	for _, crb := range []string{
		"Blocked-On-Body: " + beFinding,
		"Blocked-On-Body: " + beFinding + " not-a-digest",
		"Blocked-On-Body: " + beFinding + " " + PRBodyDigest(beBodyBefore) + " extra",
	} {
		in := beInput()
		in.CRBody = crb
		refused(t, in, "is malformed")
	}
	in := beInput()
	in.CRBody = beCR() + "\nBlocked-On-Body: f-other " + PRBodyDigest(beBodyBefore)
	refused(t, in, "disagree")
}

func TestBodyEdit_NearMiss_UnknownHead(t *testing.T) {
	in := beInput()
	in.Head = ""
	refused(t, in, "the head is unknown")
}
