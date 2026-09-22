package main

import (
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// --- fixtures & builders -----------------------------------------------------

const (
	epqPrereqObject = "example-org/tracker#42"
	epqMergeSHA     = "7071466f1dd952b9580edc5e98dbf0251fbba9b3"
	epqCRAt         = "2026-09-21T10:00:00Z"
	epqApproveAt    = "2026-09-21T11:00:00Z"
	epqMergeAt      = "2026-09-21T10:30:00Z" // strictly AFTER the CR at epqCRAt
)

func epqExternalFinding(id, object string) deskkit.Finding {
	return deskkit.Finding{
		ID: id, Class: "ext:" + id, Severity: deskkit.SeverityBlocking,
		Blocker: deskkit.BlockerExternalPrereq, State: deskkit.StateOpen,
		SharedRepair: object, Resolution: object + " lands", Failure: "blocked on " + object,
	}
}

func epqContentFinding(id string) deskkit.Finding {
	return deskkit.Finding{
		ID: id, Class: "code:" + id, Severity: deskkit.SeverityBlocking,
		Blocker: deskkit.BlockerCodeContent, State: deskkit.StateOpen,
		Failure: "nil deref at handler.go:12",
	}
}

func epqCRBody(summary string, findings ...deskkit.Finding) string {
	block := deskkit.RenderFindingBlock(deskkit.FindingBlockV1{Findings: findings})
	return "Verdict: request-changes\n\nExternal-Prereq-Only: " + summary + "\n\n" + block + "\n"
}

func epqApproveBody(clearances ...string) string {
	var b strings.Builder
	b.WriteString("Verdict: approve\n\n")
	for _, c := range clearances {
		b.WriteString("Cleared-Prereq: " + c + "\n")
	}
	return b.String()
}

func epqReview(state, commit, submittedAt, body string) reviewInfo {
	r := reviewInfo{State: state, CommitID: commit, SubmittedAt: submittedAt, Body: body}
	r.User.Login = reviewerBotDisplay()
	return r
}

// epqSatisfied is the fresh observation an honest reviewer's citation should match.
func epqSatisfied() deskkit.PrereqObservation {
	at, _ := time.Parse(time.RFC3339, epqMergeAt)
	return deskkit.PrereqObservation{
		Object: epqPrereqObject, SatisfyingRef: epqMergeSHA, ObservedAt: at, Readable: true,
	}
}

// withObserver installs a fresh-observation seam for one test and restores it after.
func withObserver(t *testing.T, obs func(deskkit.PrereqCondition) deskkit.PrereqObservation) {
	t.Helper()
	old := observePrereq
	observePrereq = obs
	t.Cleanup(func() { observePrereq = old })
}

// --- Verify row 1 ------------------------------------------------------------

// TestExternalPrerequisiteSameHead — a sole typed prerequisite becoming satisfied allows
// independent re-review at an UNCHANGED head, and the board classifier and the ready gate
// agree about it. This is the POSITIVE path, driven end-to-end through the ready-flip verb.
func TestExternalPrerequisiteSameHead(t *testing.T) {
	f, _ := setupFake(t)
	withObserver(t, func(deskkit.PrereqCondition) deskkit.PrereqObservation { return epqSatisfied() })

	cr := epqCRBody("waiting on "+epqPrereqObject+" to merge", epqExternalFinding("epq-1", epqPrereqObject))
	ap := epqApproveBody("epq-1 " + epqPrereqObject + " " + epqMergeSHA)
	f.reviews = []reviewInfo{
		epqReview("CHANGES_REQUESTED", testHead, epqCRAt, cr),
		epqReview("APPROVED", testHead, epqApproveAt, ap),
	}
	f.status = greenStatus()

	// The board's shared classifier must agree the CR is a DECLARED external-prereq
	// re-review — not a suspected forgery — so the two surfaces do not diverge.
	if !deskkit.ExternalPrereqOnlyDeclared(cr) {
		t.Fatal("board classifier: ExternalPrereqOnlyDeclared(cr) = false, want true (board would mislabel as SUSPECT-APPROVAL)")
	}

	if code := run(readyArgs(exampleRepo)); code != 0 {
		t.Fatalf("ready exit = %d, want 0 (the verified external prerequisite should clear the same-head block)", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1", f.flips)
	}

	// Same inputs, same direction, from the decision itself: cleared.
	out := clearedByExternalPrereq(f.reviews, testHead, false)
	if !out.declared || !out.decision.Cleared {
		t.Fatalf("decision = %+v, want declared+cleared (ready and board must agree)", out)
	}
}

// --- Verify row 2 ------------------------------------------------------------

// TestExternalPrerequisiteMixedAndForged — mixed content findings, worker-authored
// clearance, quoted markers, unrelated objects and unreadable source data cannot clear the
// rejection.
func TestExternalPrerequisiteMixedAndForged(t *testing.T) {
	extFinding := epqExternalFinding("epq-1", epqPrereqObject)
	goodCite := "epq-1 " + epqPrereqObject + " " + epqMergeSHA

	t.Run("mixed content finding blocks", func(t *testing.T) {
		withObserver(t, func(deskkit.PrereqCondition) deskkit.PrereqObservation { return epqSatisfied() })
		cr := epqCRBody("waiting on upstream", extFinding, epqContentFinding("code-1"))
		reviews := []reviewInfo{
			epqReview("CHANGES_REQUESTED", testHead, epqCRAt, cr),
			epqReview("APPROVED", testHead, epqApproveAt, epqApproveBody(goodCite)),
		}
		out := clearedByExternalPrereq(reviews, testHead, false)
		if !out.declared || out.decision.Cleared {
			t.Fatalf("decision = %+v, want declared but NOT cleared (mixed content finding)", out)
		}
		if !strings.Contains(out.decision.Reason, "code/content") {
			t.Fatalf("reason = %q, want it to name the code/content blocker", out.decision.Reason)
		}
	})

	t.Run("worker-authored clearance is ignored", func(t *testing.T) {
		withObserver(t, func(deskkit.PrereqCondition) deskkit.PrereqObservation { return epqSatisfied() })
		cr := epqReview("CHANGES_REQUESTED", testHead, epqCRAt, epqCRBody("waiting", extFinding))
		// A worker login posts the APPROVE + citation — not the reviewer bot.
		worker := reviewInfo{State: "APPROVED", CommitID: testHead, SubmittedAt: epqApproveAt, Body: epqApproveBody(goodCite)}
		worker.User.Login = "assay-worker-app[bot]"
		out := clearedByExternalPrereq([]reviewInfo{cr, worker}, testHead, false)
		if out.decision.Cleared {
			t.Fatalf("decision cleared on a worker-authored citation: %+v", out)
		}
	})

	t.Run("quoted declaration is not a declaration", func(t *testing.T) {
		fenced := "Verdict: request-changes\n\n```\nExternal-Prereq-Only: quoted, not real\n```\n"
		reviews := []reviewInfo{epqReview("CHANGES_REQUESTED", testHead, epqCRAt, fenced)}
		out := clearedByExternalPrereq(reviews, testHead, false)
		if out.declared {
			t.Fatalf("a fenced marker counted as a declaration: %+v", out)
		}
		if deskkit.ExternalPrereqOnlyDeclared(fenced) {
			t.Fatal("ExternalPrereqOnlyDeclared read a fenced marker as declared")
		}
	})

	t.Run("unrelated object cannot clear", func(t *testing.T) {
		withObserver(t, func(deskkit.PrereqCondition) deskkit.PrereqObservation {
			o := epqSatisfied()
			o.Object = "example-org/tracker#999" // a different object than declared
			return o
		})
		cr := epqReview("CHANGES_REQUESTED", testHead, epqCRAt, epqCRBody("waiting", extFinding))
		ap := epqReview("APPROVED", testHead, epqApproveAt, epqApproveBody(goodCite))
		out := clearedByExternalPrereq([]reviewInfo{cr, ap}, testHead, false)
		if out.decision.Cleared {
			t.Fatalf("cleared on an unrelated object: %+v", out)
		}
	})

	t.Run("unreadable source data blocks", func(t *testing.T) {
		withObserver(t, func(deskkit.PrereqCondition) deskkit.PrereqObservation {
			return deskkit.PrereqObservation{Object: epqPrereqObject, Readable: false} // could-not-check
		})
		cr := epqReview("CHANGES_REQUESTED", testHead, epqCRAt, epqCRBody("waiting", extFinding))
		ap := epqReview("APPROVED", testHead, epqApproveAt, epqApproveBody(goodCite))
		out := clearedByExternalPrereq([]reviewInfo{cr, ap}, testHead, false)
		if out.decision.Cleared {
			t.Fatalf("cleared on unreadable evidence: %+v", out)
		}
		if !strings.Contains(out.decision.Reason, "could-not-check") {
			t.Fatalf("reason = %q, want a could-not-check", out.decision.Reason)
		}
	})
}

// --- Verify row 3 ------------------------------------------------------------

// TestExternalPrerequisiteFreshnessAndLanes — wrong revision, a prerequisite predating the
// rejection, a later revocation and a standing security failure continue to block.
func TestExternalPrerequisiteFreshnessAndLanes(t *testing.T) {
	extFinding := epqExternalFinding("epq-1", epqPrereqObject)
	goodCite := "epq-1 " + epqPrereqObject + " " + epqMergeSHA
	crRev := func(sec bool, obs func(deskkit.PrereqCondition) deskkit.PrereqObservation) externalPrereqOutcome {
		reviews := []reviewInfo{
			epqReview("CHANGES_REQUESTED", testHead, epqCRAt, epqCRBody("waiting", extFinding)),
			epqReview("APPROVED", testHead, epqApproveAt, epqApproveBody(goodCite)),
		}
		return clearedByExternalPrereq(reviews, testHead, sec)
	}

	t.Run("wrong revision blocks", func(t *testing.T) {
		withObserver(t, func(deskkit.PrereqCondition) deskkit.PrereqObservation {
			o := epqSatisfied()
			o.SatisfyingRef = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef" // not the cited merge sha
			return o
		})
		if out := crRev(false, nil); out.decision.Cleared {
			t.Fatalf("cleared on a wrong revision: %+v", out)
		}
	})

	t.Run("prerequisite predating the rejection blocks", func(t *testing.T) {
		withObserver(t, func(deskkit.PrereqCondition) deskkit.PrereqObservation {
			o := epqSatisfied()
			o.ObservedAt, _ = time.Parse(time.RFC3339, "2026-09-20T09:00:00Z") // BEFORE the CR
			return o
		})
		if out := crRev(false, nil); out.decision.Cleared {
			t.Fatalf("cleared on a prerequisite that predates the rejection: %+v", out)
		}
		if !strings.Contains(crRev(false, nil).decision.Reason, "AFTER the rejection") {
			t.Fatalf("reason did not name the freshness clause")
		}
	})

	t.Run("later revocation blocks", func(t *testing.T) {
		withObserver(t, func(deskkit.PrereqCondition) deskkit.PrereqObservation {
			o := epqSatisfied()
			o.Revoked = true
			return o
		})
		if out := crRev(false, nil); out.decision.Cleared {
			t.Fatalf("cleared despite a later revocation: %+v", out)
		}
	})

	t.Run("standing security failure blocks", func(t *testing.T) {
		withObserver(t, func(deskkit.PrereqCondition) deskkit.PrereqObservation { return epqSatisfied() })
		if out := crRev(true, nil); out.decision.Cleared { // securityFail = true
			t.Fatalf("cleared over a standing Security-Review: fail: %+v", out)
		}
		if !strings.Contains(crRev(true, nil).decision.Reason, "Security-Review") {
			t.Fatalf("reason did not name the security retraction")
		}
	})

	// The honest, fully-satisfied case still clears — so the negatives above are proving a
	// discriminating gate, not one that refuses everything.
	t.Run("fully satisfied clears (control)", func(t *testing.T) {
		withObserver(t, func(deskkit.PrereqCondition) deskkit.PrereqObservation { return epqSatisfied() })
		if out := crRev(false, nil); !out.decision.Cleared {
			t.Fatalf("the fully-satisfied control did NOT clear: %+v", out)
		}
	})
}

// --- Verify row 4 ------------------------------------------------------------

// TestExternalPrerequisiteRestartAndCheckOnly — restart preserves the exact condition and
// observation (the decision is deterministic over the same durable records), and the
// existing check-only behaviour plus the rejection of unchanged content findings remain
// intact.
func TestExternalPrerequisiteRestartAndCheckOnly(t *testing.T) {
	extFinding := epqExternalFinding("epq-1", epqPrereqObject)
	goodCite := "epq-1 " + epqPrereqObject + " " + epqMergeSHA
	reviews := []reviewInfo{
		epqReview("CHANGES_REQUESTED", testHead, epqCRAt, epqCRBody("waiting", extFinding)),
		epqReview("APPROVED", testHead, epqApproveAt, epqApproveBody(goodCite)),
	}

	t.Run("deterministic across restart", func(t *testing.T) {
		withObserver(t, func(deskkit.PrereqCondition) deskkit.PrereqObservation { return epqSatisfied() })
		a := clearedByExternalPrereq(reviews, testHead, false)
		b := clearedByExternalPrereq(reviews, testHead, false) // a "restart": re-derive from the same records
		if a.decision.Cleared != b.decision.Cleared || a.decision.Reason != b.decision.Reason {
			t.Fatalf("non-deterministic decision across restart:\n a=%+v\n b=%+v", a, b)
		}
		if !a.decision.Cleared {
			t.Fatalf("the deterministic case did not clear: %+v", a)
		}
	})

	t.Run("check-only exemption is untouched", func(t *testing.T) {
		// A check-only CR is a DIFFERENT exemption. Its marker still parses, and it is NOT
		// read as an external-prerequisite declaration — the two exemptions never cross-fire.
		checkOnly := "Verdict: request-changes\n\nBlocked-On-Check: changelog\n"
		if deskkit.BlockedOnCheckName(checkOnly) != "changelog" {
			t.Fatal("check-only marker no longer parses — the sibling exemption regressed")
		}
		if deskkit.ExternalPrereqOnlyDeclared(checkOnly) {
			t.Fatal("a check-only CR read as an external-prerequisite declaration")
		}
		out := clearedByExternalPrereq([]reviewInfo{
			epqReview("CHANGES_REQUESTED", testHead, epqCRAt, checkOnly),
		}, testHead, false)
		if out.declared {
			t.Fatalf("check-only CR classified as an external-prereq declaration: %+v", out)
		}
	})

	t.Run("unchanged content findings still block", func(t *testing.T) {
		// A CR with a content blocker and NO external declaration is an ordinary rejection:
		// declared=false, so the unchanged-head rule stands unchanged.
		withObserver(t, func(deskkit.PrereqCondition) deskkit.PrereqObservation { return epqSatisfied() })
		contentCR := "Verdict: request-changes\n\n" +
			deskkit.RenderFindingBlock(deskkit.FindingBlockV1{Findings: []deskkit.Finding{epqContentFinding("code-1")}}) + "\n"
		out := clearedByExternalPrereq([]reviewInfo{
			epqReview("CHANGES_REQUESTED", testHead, epqCRAt, contentCR),
			epqReview("APPROVED", testHead, epqApproveAt, epqApproveBody(goodCite)),
		}, testHead, false)
		if out.declared || out.decision.Cleared {
			t.Fatalf("an unchanged content finding cleared or declared: %+v", out)
		}
	})
}
