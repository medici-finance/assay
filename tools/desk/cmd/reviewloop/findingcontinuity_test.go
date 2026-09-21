package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// findingcontinuity_test.go — the executable Verify rows of the persistent review-finding brief.
//
// Each test injects a review thread as durable forge records and derives the finding
// ledger, exactly as the reactor does in production (the body->block->ledger path). The
// derivation is a PURE function of the records, so "survives a restart / an agent
// replacement" is proved by deriving the SAME records twice and getting the SAME answer.

// blk builds a one-finding block for a record.
func blk(f deskkit.Finding) *deskkit.FindingBlockV1 {
	return &deskkit.FindingBlockV1{Schema: deskkit.FindingBlockSchema, Findings: []deskkit.Finding{f}}
}

// blkN builds a multi-finding block.
func blkN(fs ...deskkit.Finding) *deskkit.FindingBlockV1 {
	return &deskkit.FindingBlockV1{Schema: deskkit.FindingBlockSchema, Findings: fs}
}

func reviewerRec(seq int, head, verdict string, block *deskkit.FindingBlockV1) deskkit.ForgeRecord {
	return deskkit.ForgeRecord{Seq: seq, Kind: deskkit.RecordReview, Role: deskkit.RoleReviewer,
		Actor: "assay-reviewer-app[bot]", Head: head, Verdict: deskkit.Verdict(verdict), Block: block}
}

func workerRec(seq int, head string, block *deskkit.FindingBlockV1) deskkit.ForgeRecord {
	return deskkit.ForgeRecord{Seq: seq, Kind: deskkit.RecordReply, Role: deskkit.RoleWorker,
		Actor: "assay-worker-app[bot]", Head: head, Block: block}
}

func blockingCode(id, class, head, state string, evidence ...string) deskkit.Finding {
	return deskkit.Finding{ID: id, Class: class, Severity: deskkit.SeverityBlocking,
		Blocker: deskkit.BlockerCodeContent, State: deskkit.FindingState(state),
		OriginHead: head, EvidenceHead: head, Failure: "concrete reproduction for " + id, Evidence: evidence}
}

// TestReviewFindingContinuityAcrossHeads — Verify row 1.
//
// Fix A and change B: A retains its ID and evidence; the follow-up review targets both the
// fix and the changed surface; there is no stale approval carried across the head change.
func TestReviewFindingContinuityAcrossHeads(t *testing.T) {
	const h1, h2 = "1a2b3c4d", "5e6f7a8b"

	records := []deskkit.ForgeRecord{
		// h1: reviewer raises finding A (class claimA), with evidence ev-A1.
		reviewerRec(1, h1, "request-changes", blk(blockingCode("A", "claimA", h1, "open", "ev-A1"))),
		// h2: worker fixes A and pushes a head that also changed surface B. A is now
		// fixed-awaiting-review, with the worker's own fix evidence.
		workerRec(2, h2, blk(blockingCode("A", "claimA", h1, "fixed-awaiting-review", "ev-fix-A"))),
		// h2: follow-up reviewer targets BOTH the changed surface B (a new finding) — A is
		// left as it was, deliberately NOT approved on the old head's basis.
		reviewerRec(3, h2, "request-changes", blk(blockingCode("B", "claimB", h2, "open", "ev-B1"))),
	}

	l := DeriveContinuity(records)

	// A survives the head change with its ID, class and ORIGINAL evidence.
	a, ok := l.Findings["A"]
	if !ok {
		t.Fatal("finding A was lost across the head change — continuity failed")
	}
	if a.Class != "claimA" {
		t.Fatalf("finding A lost its class: got %q", a.Class)
	}
	if !contains(a.Evidence, "ev-A1") {
		t.Fatalf("finding A lost its original evidence ev-A1: %v", a.Evidence)
	}
	// No stale approval: A was never resolved by a reviewer at the current head.
	if a.State == deskkit.StateResolved {
		t.Fatalf("finding A is resolved, but no reviewer resolved it at the current head — a stale approval was carried across the change")
	}
	if l.ResolvedAtHead("A", h2) {
		t.Fatal("finding A reads resolved at the current head with no current-head reviewer resolution")
	}
	// The follow-up targets the changed surface B.
	if _, ok := l.Findings["B"]; !ok {
		t.Fatal("the follow-up review did not target the changed surface B")
	}
	// Both are outstanding.
	open := l.OpenBlocking()
	if !contains(open, "A") || !contains(open, "B") {
		t.Fatalf("both A and B must be outstanding blocking findings; got %v", open)
	}
}

// TestReviewFindingCannotSelfResolve — Verify row 2.
//
// A worker self-resolution, a resolution on wrong-head evidence, and a malformed / legacy
// prose reply all fail to clear a blocking finding.
func TestReviewFindingCannotSelfResolve(t *testing.T) {
	const h1, h2 = "11aa22bb", "33cc44dd"

	t.Run("worker cannot self-resolve", func(t *testing.T) {
		records := []deskkit.ForgeRecord{
			reviewerRec(1, h1, "request-changes", blk(blockingCode("W", "classW", h1, "open", "ev"))),
			// Worker asserts RESOLVED on its own blocking finding.
			workerRec(2, h1, blk(blockingCode("W", "classW", h1, "resolved", "ev"))),
		}
		l := DeriveContinuity(records)
		if l.Findings["W"].State == deskkit.StateResolved {
			t.Fatal("a worker cleared its own blocking finding — the ledger accepted a self-resolution")
		}
		if !contains(l.OpenBlocking(), "W") {
			t.Fatal("finding W is no longer outstanding after a worker self-resolution attempt")
		}
		if len(l.Blind) == 0 {
			t.Fatal("the self-resolution attempt was not reported as could-not-check")
		}
		// The write-time gate refuses the same block as a worker.
		body := deskkit.RenderFindingBlock(*blk(blockingCode("W", "classW", h1, "resolved", "ev")))
		if err := deskkit.ValidateReviewFindingBlock([]byte(body), deskkit.RoleWorker); err == nil {
			t.Fatal("deskreply's write gate accepted a worker block that resolves a blocking finding")
		}
		// The same block IS valid from a reviewer.
		if err := deskkit.ValidateReviewFindingBlock([]byte(body), deskkit.RoleReviewer); err != nil {
			t.Fatalf("a reviewer resolution block was refused: %v", err)
		}
	})

	t.Run("wrong-head evidence cannot resolve", func(t *testing.T) {
		// Reviewer at h2 asserts resolved but with evidence gathered at the STALE head h1.
		stale := blockingCode("S", "classS", h1, "resolved", "ev")
		stale.EvidenceHead = h1 // stale
		records := []deskkit.ForgeRecord{
			reviewerRec(1, h1, "request-changes", blk(blockingCode("S", "classS", h1, "open", "ev"))),
			workerRec(2, h2, blk(blockingCode("S", "classS", h1, "fixed-awaiting-review", "ev"))),
			reviewerRec(3, h2, "approve", blk(stale)), // record head h2, evidence head h1
		}
		l := DeriveContinuity(records)
		if l.ResolvedAtHead("S", h2) {
			t.Fatal("a resolution on stale-head evidence cleared the finding at the current head")
		}
		if !contains(l.OpenBlocking(), "S") {
			t.Fatal("finding S is no longer outstanding after a stale-evidence resolution")
		}
	})

	t.Run("legacy prose cannot resolve", func(t *testing.T) {
		records := []deskkit.ForgeRecord{
			reviewerRec(1, h1, "request-changes", blk(blockingCode("L", "classL", h1, "open", "ev"))),
			// A legacy worker reply: no typed block at all (Block == nil).
			workerRec(2, h1, nil),
		}
		l := DeriveContinuity(records)
		if !contains(l.OpenBlocking(), "L") {
			t.Fatal("a legacy prose reply cleared a blocking finding — a clean set was inferred from unparseable prose")
		}
		// A body that CLAIMS the block but is malformed poisons the read (fail closed).
		bad := `text <!-- assay:review-finding:v1 {not json} --> more`
		if _, _, err := deskkit.ParseFindingBlock(bad); err == nil {
			t.Fatal("a malformed finding block parsed clean")
		}
	})
}

// TestReviewFindingCapSurvivesRestart — Verify row 3.
//
// Three full rounds of one class survive a restart; the next round emits ONE arbiter packet;
// duplicate sweeps do not refile; an unrelated class starts separately; and a newly noticed
// sibling sentence retains the old class and cannot evade the cap.
func TestReviewFindingCapSurvivesRestart(t *testing.T) {
	const h = "aabbccdd"

	// Raise C, then three review->response->re-review transitions, then a fourth re-review.
	records := []deskkit.ForgeRecord{
		reviewerRec(1, h, "request-changes", blk(blockingCode("C", "classC", h, "open", "ev"))), // raise
		workerRec(2, h, blk(blockingCode("C", "classC", h, "fixed-awaiting-review", "ev"))),
		reviewerRec(3, h, "request-changes", blk(blockingCode("C", "classC", h, "open", "ev"))), // round 1
		workerRec(4, h, blk(blockingCode("C", "classC", h, "fixed-awaiting-review", "ev"))),
		reviewerRec(5, h, "request-changes", blk(blockingCode("C", "classC", h, "open", "ev"))), // round 2
		workerRec(6, h, blk(blockingCode("C", "classC", h, "fixed-awaiting-review", "ev"))),
		reviewerRec(7, h, "request-changes", blk(blockingCode("C", "classC", h, "open", "ev"))), // round 3
		workerRec(8, h, blk(blockingCode("C", "classC", h, "fixed-awaiting-review", "ev"))),
		reviewerRec(9, h, "request-changes", blk(blockingCode("C", "classC", h, "open", "ev"))), // hits the cap
	}

	l := DeriveContinuity(records)
	if l.Rounds["classC"] != deskkit.RoundCap {
		t.Fatalf("classC round count = %d, want the cap %d", l.Rounds["classC"], deskkit.RoundCap)
	}
	if !l.Held["classC"] {
		t.Fatal("classC did not hold at the cap")
	}
	if len(l.Arbiter) != 1 {
		t.Fatalf("expected exactly ONE arbiter packet at the cap, got %d", len(l.Arbiter))
	}
	if l.Arbiter[0].Class != "classC" {
		t.Fatalf("arbiter packet is for the wrong class: %q", l.Arbiter[0].Class)
	}

	// RESTART: re-derive the SAME durable records. The counts, the hold and the single packet
	// must be identical — the derivation carried no in-memory state across the "restart".
	l2 := DeriveContinuity(records)
	if l2.Rounds["classC"] != deskkit.RoundCap || len(l2.Arbiter) != 1 {
		t.Fatalf("restart changed the derived state: rounds=%d arbiter=%d", l2.Rounds["classC"], len(l2.Arbiter))
	}

	// A DUPLICATE SWEEP after the cap does not refile, does not add a round, and does not
	// reset the class. A newly noticed SIBLING sentence of the same class keeps the class ID
	// and is held, not started fresh.
	more := append(append([]deskkit.ForgeRecord{}, records...),
		reviewerRec(10, h, "request-changes", blk(blockingCode("C", "classC", h, "open", "ev"))),         // duplicate sweep
		reviewerRec(11, h, "request-changes", blk(blockingCode("C-sibling", "classC", h, "open", "ev"))), // sibling sentence, same class
		// An unrelated class D starts on its own count.
		reviewerRec(12, h, "request-changes", blk(blockingCode("D", "classD", h, "open", "ev"))),
	)
	l3 := DeriveContinuity(more)
	if l3.Rounds["classC"] != deskkit.RoundCap {
		t.Fatalf("a duplicate sweep / sibling changed classC's round count: %d", l3.Rounds["classC"])
	}
	if len(l3.Arbiter) != 1 {
		t.Fatalf("a duplicate sweep refiled the arbiter packet: got %d", len(l3.Arbiter))
	}
	if sib := l3.Findings["C-sibling"]; sib == nil || sib.State != deskkit.StateAwaitingArbitration {
		t.Fatalf("a sibling sentence evaded the cap: %+v", sib)
	}
	if l3.Held["classD"] {
		t.Fatal("unrelated class D was swept up in classC's hold")
	}
	if l3.Rounds["classD"] != 0 {
		t.Fatalf("unrelated class D started with classC's round count: %d", l3.Rounds["classD"])
	}

	// Polls are not rounds: consecutive reviewer verdicts with NO intervening worker response
	// do not count. A round is a review -> response -> re-review transition, never a poll tick.
	polls := DeriveContinuity([]deskkit.ForgeRecord{
		reviewerRec(1, h, "request-changes", blk(blockingCode("P", "classP", h, "open", "ev"))),
		reviewerRec(2, h, "request-changes", blk(blockingCode("P", "classP", h, "open", "ev"))),
		reviewerRec(3, h, "request-changes", blk(blockingCode("P", "classP", h, "open", "ev"))),
	})
	if polls.Rounds["classP"] != 0 {
		t.Fatalf("reviewer polls with no intervening worker response counted as %d round(s)", polls.Rounds["classP"])
	}
}

// TestReviewFindingSharedCIBlocker — Verify row 4.
//
// Multiple PRs cite ONE shared repair without inventing multiple content defects; a
// ready-flip still requires the applicable checks (the shared blocker does not auto-clear).
func TestReviewFindingSharedCIBlocker(t *testing.T) {
	const h1, h2 = "0011eecc", "2233ffdd"

	shared := func(id, head string) deskkit.Finding {
		return deskkit.Finding{ID: id, Class: "shared-ci", Severity: deskkit.SeverityBlocking,
			Blocker: deskkit.BlockerExternalPrereq, State: deskkit.StateOpen, OriginHead: head,
			EvidenceHead: head, Failure: "shared CI leg red", Evidence: []string{"run-123"},
			SharedRepair: "assay-toolkit#999"}
	}

	// PR one: a shared-CI blocker plus a genuine code-content defect of its own.
	pr1 := DeriveContinuity([]deskkit.ForgeRecord{
		reviewerRec(1, h1, "request-changes", blkN(
			shared("ci-1", h1),
			blockingCode("own-1", "own-defect", h1, "open", "ev"),
		)),
	})
	// PR two: the SAME shared-CI blocker, no content defect.
	pr2 := DeriveContinuity([]deskkit.ForgeRecord{
		reviewerRec(1, h2, "request-changes", blk(shared("ci-2", h2))),
	})

	// The shared-CI finding is NOT a per-PR content defect on either PR.
	if contains(pr1.ContentDefects(), "ci-1") {
		t.Fatal("the shared-CI blocker was invented as a content defect on PR one")
	}
	if contains(pr2.ContentDefects(), "ci-2") {
		t.Fatal("the shared-CI blocker was invented as a content defect on PR two")
	}
	// PR one's OWN defect is still a content defect.
	if !contains(pr1.ContentDefects(), "own-1") {
		t.Fatal("PR one's own content defect was dropped")
	}

	// Both PRs cite ONE shared repair, not two invented ones.
	repairs := map[string]bool{}
	for _, r := range append(pr1.SharedRepairs(), pr2.SharedRepairs()...) {
		repairs[r] = true
	}
	if len(repairs) != 1 {
		t.Fatalf("the two PRs cite %d distinct shared repairs, want 1: %v", len(repairs), repairs)
	}

	// The shared blocker still BLOCKS — a ready-flip cannot treat it as cleared, so the
	// applicable checks are still required. It stays an outstanding blocking finding.
	if !contains(pr1.OpenBlocking(), "ci-1") || !contains(pr2.OpenBlocking(), "ci-2") {
		t.Fatal("the shared-CI blocker stopped blocking — a ready-flip could proceed without the applicable checks")
	}
}

// TestReadRecordsParsesForgeBodies proves the production path: a records payload whose
// records carry the finding block INSIDE their raw forge body parses to the same ledger, and
// a malformed block fails closed.
func TestReadRecordsParsesForgeBodies(t *testing.T) {
	body := "Changes requested.\n\n" +
		deskkit.RenderFindingBlock(*blk(blockingCode("F", "classF", "abcdef01", "open", "ev"))) +
		"\n\nthanks"
	payload := `{"repo":"medici-finance/assay","pr":7,"currentHead":"abcdef01","records":[` +
		`{"seq":1,"kind":"review","role":"reviewer","actor":"assay-reviewer-app[bot]","head":"abcdef01","verdict":"request-changes","body":` + jsonString(body) + `}` +
		`]}`
	rep, records, err := ReadRecords([]byte(payload))
	if err != nil {
		t.Fatalf("ReadRecords refused a valid payload: %v", err)
	}
	if rep.PR != 7 {
		t.Fatalf("PR not read: %d", rep.PR)
	}
	l := DeriveContinuity(records)
	if !contains(l.OpenBlocking(), "F") {
		t.Fatal("the block embedded in the forge body was not derived")
	}

	// A malformed block in a record body poisons the read.
	badBody := "text <!-- assay:review-finding:v1 {broken --> tail"
	badPayload := `{"repo":"medici-finance/assay","pr":7,"records":[` +
		`{"seq":1,"kind":"review","role":"reviewer","head":"h","body":` + jsonString(badBody) + `}]}`
	if _, _, err := ReadRecords([]byte(badPayload)); err == nil {
		t.Fatal("ReadRecords accepted a malformed finding block instead of failing closed")
	} else if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("a malformed block should be could-not-check (exit %d), got exit %d", deskkit.ExitUnverifiable, deskkit.ExitCodeOf(err))
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// jsonString quotes s as a JSON string literal for the inline payloads above.
func jsonString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
