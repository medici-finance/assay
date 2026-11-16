package deskkit

import (
	"strconv"
	"strings"
	"testing"
)

func blockingFinding(id, class, state string) Finding {
	return Finding{ID: id, Class: class, Severity: SeverityBlocking, Blocker: BlockerCodeContent,
		State: FindingState(state), OriginHead: "head", EvidenceHead: "head",
		Failure: "concrete reproduction for " + id, Evidence: []string{"ev-" + id}}
}

// TestReviewFindingBlockRoundTrips proves a rendered block parses back to an equal block, and
// that a body with no block is the legacy case (present=false, err=nil), never an error.
func TestReviewFindingBlockRoundTrips(t *testing.T) {
	in := FindingBlockV1{Schema: FindingBlockSchema, Findings: []Finding{blockingFinding("A", "c", "open")}}
	body := "some prose\n\n" + RenderFindingBlock(in) + "\n\nmore prose"
	got, present, err := ParseFindingBlock(body)
	if err != nil || !present {
		t.Fatalf("round-trip parse failed: present=%v err=%v", present, err)
	}
	if len(got.Findings) != 1 || got.Findings[0].ID != "A" {
		t.Fatalf("round-trip lost the finding: %+v", got)
	}

	// A plain body with no marker is the legacy case: not present, not an error.
	if _, present, err := ParseFindingBlock("just a normal review, no block here"); present || err != nil {
		t.Fatalf("legacy prose was not treated as the no-block case: present=%v err=%v", present, err)
	}
}

// TestReviewFindingBlockMalformed proves a body that CLAIMS the block but is broken refuses
// rather than reading as legacy prose.
func TestReviewFindingBlockMalformed(t *testing.T) {
	cases := map[string]string{
		"unterminated":  "x <!-- assay:review-finding:v1 {\"schema\":\"review-finding/v1\"}",
		"bad json":      "x <!-- assay:review-finding:v1 {nope} -->",
		"unknown field": `x <!-- assay:review-finding:v1 {"schema":"review-finding/v1","bogus":1} -->`,
		"wrong schema":  `x <!-- assay:review-finding:v1 {"schema":"review-finding/v2"} -->`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			_, present, err := ParseFindingBlock(body)
			if !present {
				t.Fatal("a present-but-broken block was read as absent")
			}
			if err == nil {
				t.Fatal("a malformed block parsed clean")
			}
			if ExitCodeOf(err) != ExitRefused {
				t.Fatalf("malformed block should refuse (exit %d), got %d", ExitRefused, ExitCodeOf(err))
			}
		})
	}
}

// regression: resolvedHead refused with no route to evidenceHead
// TestFindingUnknownHeadField proves a block carrying a head field v1 does not define still
// refuses — v1 readers are strict, so the field is never silently dropped — and that the
// refusal names the field and the v1 field that carries the fact: evidenceHead, the head a
// resolution was checked at. resolvedHead is the field a reviewer reached for; fixedHead and
// resolved_head are planted siblings the fix does not name.
func TestFindingUnknownHeadField(t *testing.T) {
	const sha = "9aa837adaf9b6074ca703755216ad448e185ae4a"
	for _, field := range []string{"resolvedHead", "fixedHead", "resolved_head"} {
		t.Run(field, func(t *testing.T) {
			body := `x <!-- assay:review-finding:v1 {"schema":"review-finding/v1","findings":[{"id":"F1","class":"c",` +
				`"severity":"blocking","blocker":"code-content","state":"resolved","failure":"f","` + field + `":"` + sha + `"}]} -->`
			_, present, err := ParseFindingBlock(body)
			if !present || err == nil || ExitCodeOf(err) != ExitRefused {
				t.Fatalf("an undefined head field must refuse the block: present=%v err=%v", present, err)
			}
			for _, want := range []string{strconv.Quote(field), "For a head, use evidenceHead", "review-finding/v2"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("refusal does not name %s: %v", want, err)
				}
			}
		})
	}
	// A field that is not a head names the field and the v1 field list, but no head route.
	body := `x <!-- assay:review-finding:v1 {"schema":"review-finding/v1","findings":[{"id":"F1","class":"c","severity":"advisory","blocker":"code-content","state":"open","note":"n"}]} -->`
	_, _, err := ParseFindingBlock(body)
	if err == nil || !strings.Contains(err.Error(), `"note"`) || !strings.Contains(err.Error(), "sharedRepair") {
		t.Fatalf("an undefined field must be named with the v1 field list: %v", err)
	}
	if strings.Contains(err.Error(), "For a head") {
		t.Fatalf("a non-head field was given the head route: %v", err)
	}
}

// TestFindingResolvedAtEvidenceHead proves the route the refusal names works: a reviewer
// resolution whose evidenceHead is the head it was taken at passes the write gate and stops
// blocking at that head, and keeps blocking at any later head.
func TestFindingResolvedAtEvidenceHead(t *testing.T) {
	const sha, later = "9aa837adaf9b6074ca703755216ad448e185ae4a", "b3ef023c5b3ef023c5b3ef023c5b3ef023c5b3ef"
	f := blockingFinding("F1", "c", "resolved")
	f.EvidenceHead = sha
	b := &FindingBlockV1{Schema: FindingBlockSchema, Findings: []Finding{f}}
	if err := b.Validate(RoleReviewer); err != nil {
		t.Fatalf("a reviewer resolution with an evidenceHead was refused: %v", err)
	}
	if f.StandingBlockerAt(sha) || !f.StandingBlockerAt(later) {
		t.Fatalf("evidenceHead does not pin the resolution: at=%v later=%v", f.StandingBlockerAt(sha), f.StandingBlockerAt(later))
	}
}

// TestReviewFindingRoleGate proves the write-time role gate: a worker cannot author a
// reviewer resolution of a blocking finding nor hand-assert the cap; a reviewer can; and a
// blocking finding with no concrete basis is refused from either role.
func TestReviewFindingRoleGate(t *testing.T) {
	workerResolves := &FindingBlockV1{Schema: FindingBlockSchema, Findings: []Finding{blockingFinding("A", "c", "resolved")}}
	if err := workerResolves.Validate(RoleWorker); err == nil {
		t.Fatal("a worker was allowed to resolve a blocking finding")
	}
	if err := workerResolves.Validate(RoleReviewer); err != nil {
		t.Fatalf("a reviewer was refused a resolution: %v", err)
	}

	workerArbitrates := &FindingBlockV1{Schema: FindingBlockSchema, Findings: []Finding{blockingFinding("A", "c", "awaiting-arbitration")}}
	if err := workerArbitrates.Validate(RoleWorker); err == nil {
		t.Fatal("a worker was allowed to hand-assert the arbitration cap")
	}

	// A blocking finding with no concrete reproduction, explanation or evidence cannot block.
	bare := &FindingBlockV1{Schema: FindingBlockSchema, Findings: []Finding{{
		ID: "B", Class: "c", Severity: SeverityBlocking, Blocker: BlockerCodeContent, State: StateOpen}}}
	if err := bare.Validate(RoleReviewer); err == nil {
		t.Fatal("a blocking finding with no concrete basis was accepted")
	}

	// A worker CAN move a finding to fixed-awaiting-review or disputed.
	for _, st := range []string{"fixed-awaiting-review", "disputed"} {
		b := &FindingBlockV1{Schema: FindingBlockSchema, Findings: []Finding{blockingFinding("A", "c", st)}}
		if err := b.Validate(RoleWorker); err != nil {
			t.Fatalf("a worker was refused a legitimate state %q: %v", st, err)
		}
	}

	// Unknown enum values refuse.
	for _, f := range []Finding{
		{ID: "x", Class: "c", Severity: "huge", Blocker: BlockerCodeContent, State: StateOpen},
		{ID: "x", Class: "c", Severity: SeverityAdvisory, Blocker: "elsewhere", State: StateOpen},
		{ID: "x", Class: "c", Severity: SeverityAdvisory, Blocker: BlockerCodeContent, State: "vibes"},
		{ID: "", Class: "c", Severity: SeverityAdvisory, Blocker: BlockerCodeContent, State: StateOpen},
		{ID: "x", Class: "", Severity: SeverityAdvisory, Blocker: BlockerCodeContent, State: StateOpen},
	} {
		b := &FindingBlockV1{Schema: FindingBlockSchema, Findings: []Finding{f}}
		if err := b.Validate(RoleReviewer); err == nil {
			t.Fatalf("an invalid finding was accepted: %+v", f)
		}
	}
}

// TestValidateReviewFindingBlockNoBlock proves the write-verb convenience is a no-op on a
// body with no block — the additive-record contract the write path relies on.
func TestValidateReviewFindingBlockNoBlock(t *testing.T) {
	if err := ValidateReviewFindingBlock([]byte("an ordinary review body"), RoleReviewer); err != nil {
		t.Fatalf("a body with no finding block was refused: %v", err)
	}
	if err := ValidateReviewFindingBlock([]byte("an ordinary worker reply"), RoleWorker); err != nil {
		t.Fatalf("a body with no finding block was refused for a worker: %v", err)
	}
}

// TestDeriveLedgerBlindOnMissingIdentity proves a record whose authenticated role or head is
// absent is could-not-check — it never clears a finding and never counts a round.
func TestDeriveLedgerBlindOnMissingIdentity(t *testing.T) {
	records := []ForgeRecord{
		{Seq: 1, Kind: RecordReview, Role: RoleReviewer, Head: "h", Block: &FindingBlockV1{
			Schema: FindingBlockSchema, Findings: []Finding{blockingFinding("A", "c", "open")}}},
		// No role: could-not-check.
		{Seq: 2, Kind: RecordReview, Head: "h", Block: &FindingBlockV1{
			Schema: FindingBlockSchema, Findings: []Finding{blockingFinding("A", "c", "resolved")}}},
		// No head: could-not-check.
		{Seq: 3, Kind: RecordReview, Role: RoleReviewer, Block: &FindingBlockV1{
			Schema: FindingBlockSchema, Findings: []Finding{blockingFinding("A", "c", "resolved")}}},
	}
	l := DeriveLedger(records)
	if l.Findings["A"].State == StateResolved {
		t.Fatal("an unattributed record cleared a finding")
	}
	if len(l.Blind) < 2 {
		t.Fatalf("expected two could-not-check records, got %d: %v", len(l.Blind), l.Blind)
	}
	if !strings.Contains(strings.Join(l.Blind, "\n"), "no authenticated role") {
		t.Fatalf("the missing-role record was not reported as itself: %v", l.Blind)
	}
}
