package main

// reviewscope_test.go — the acceptance suite for the review scope contract
// (example-stream/20).
//
// The contract itself is the prompt kit (review-prompt clause 12) and the review-desk
// reader; deskkit.reviewscope is its executable specification. These tests drive that
// specification over SYNTHETIC multi-file examples so the three scope outcomes are proved as
// BEHAVIOUR, not asserted as prose — a literal-text test on the kit alone cannot satisfy the
// Verify rows, which is exactly what the brief says. TestReviewScopeKitMatchesModel then
// pins the kit's machine-checkable basis block to the model, so the document a reviewer
// reads and the specification these tests prove cannot drift apart.

import (
	"regexp"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestReviewScopeFirstPassInventory — Verify row 1. A first-pass packet over a synthetic
// multi-file example names its search, its exclusions and ALL known in-scope occurrences,
// and an incomplete search never asserts clean.
func TestReviewScopeFirstPassInventory(t *testing.T) {
	// A synthetic false-claim class with occurrences spread across three files: two in-scope
	// occurrences of the same proposition and one incidental substring match the reviewer read
	// and excluded.
	inv := deskkit.FirstPassInventory{
		Search:     "grep -rn 'settles in one block' docs/ src/",
		Scope:      "changed surface (src/pay.go), the item's required deliverables, and references to the settlement claim across the repository",
		Exclusions: "vendored third-party trees (read, not this item's deliverable)",
		Revision:   "synthetic@feedface",
		Complete:   true,
		Occurrences: []deskkit.Occurrence{
			{ClassID: "settles-in-one-block", Location: "src/pay.go:42", InScope: true},
			{ClassID: "settles-in-one-block", Location: "docs/overview.md:9", InScope: true},
			{ClassID: "settles-in-one-block", Location: "vendor/x/readme.md:3", InScope: false},
		},
	}

	// The packet records the four required provenance fields.
	if ok, why := inv.Recorded(); !ok {
		t.Fatalf("a complete first-pass packet must be Recorded; got not-recorded: %s", why)
	}

	// It names ALL known in-scope occurrences together — not one, leaving the next round to
	// find the other.
	got := inv.InScopeOccurrences()
	wantLocs := map[string]bool{"src/pay.go:42": true, "docs/overview.md:9": true}
	if len(got) != len(wantLocs) {
		t.Fatalf("first-pass packet must name all %d in-scope occurrences together; named %d", len(wantLocs), len(got))
	}
	for _, o := range got {
		if !wantLocs[o.Location] {
			t.Errorf("unexpected in-scope occurrence %q — an excluded substring match must not be counted in scope", o.Location)
		}
		delete(wantLocs, o.Location)
	}
	for loc := range wantLocs {
		t.Errorf("first-pass packet omitted a known in-scope occurrence %q — naming a subset is the failure this rule closes", loc)
	}

	// A COMPLETE search with in-scope occurrences is not clean, and it does not need to be — the
	// point here is the third state: an INCOMPLETE search never certifies clean, whatever it
	// found. Take the same packet, mark the search incomplete, drop the occurrences, and confirm
	// it still refuses to certify clean.
	incomplete := inv
	incomplete.Complete = false
	incomplete.Occurrences = nil
	if clean, why := incomplete.CertifyClean(); clean {
		t.Errorf("an incomplete search certified the class clean (%q) — a search that did not look has cleared nothing", why)
	}

	// And a complete, recorded search with no in-scope occurrence IS clean, so the guard is not
	// simply always-false.
	empty := inv
	empty.Complete = true
	empty.Occurrences = []deskkit.Occurrence{{ClassID: "x", Location: "vendor/x/readme.md:3", InScope: false}}
	if clean, why := empty.CertifyClean(); !clean {
		t.Errorf("a complete recorded search with no in-scope occurrence must certify clean; refused: %s", why)
	}

	// An unrecorded packet (missing exclusions) never certifies clean even when complete.
	unrecorded := inv
	unrecorded.Exclusions = ""
	unrecorded.Occurrences = nil
	if clean, _ := unrecorded.CertifyClean(); clean {
		t.Errorf("a packet missing its exclusions must not certify clean — an unstated exclusion set is not no exclusions")
	}
}

// TestReviewScopeRequiredAndUnrelated — Verify row 2. An omitted required operator table
// remains blocking; unrelated stale prose is a follow-up; a concrete safety consequence
// remains blocking even outside the edited lines.
func TestReviewScopeRequiredAndUnrelated(t *testing.T) {
	cases := []struct {
		name    string
		finding deskkit.ReviewFinding
		want    deskkit.ScopeDisposition
	}{
		{
			// A required operator-state table the worker omitted from the diff. Untouched, but a
			// deliverable — blocking.
			name: "omitted required operator table",
			finding: deskkit.ReviewFinding{
				ClassID:            "operator-state-table",
				Location:           "docs/ops/runbook.md",
				Basis:              deskkit.BasisAcceptanceObligation,
				OutsideEditedLines: true,
			},
			want: deskkit.DispBlocker,
		},
		{
			// Pre-existing prose unrelated to this change, no scope basis — a follow-up, not a hold.
			name: "unrelated pre-existing prose",
			finding: deskkit.ReviewFinding{
				ClassID:              "stale-copyright-year",
				Location:             "NOTICE:1",
				UnrelatedPreexisting: true,
			},
			want: deskkit.DispFollowUp,
		},
		{
			// A demonstrated safety consequence of this change, sitting outside the edited lines —
			// still blocking. "Untouched" is not "irrelevant".
			name: "safety consequence outside edited lines",
			finding: deskkit.ReviewFinding{
				ClassID:            "delete-guard-disarmed",
				Location:           "src/gc.go:88",
				Basis:              deskkit.BasisSafetyConsequence,
				OutsideEditedLines: true,
			},
			want: deskkit.DispBlocker,
		},
		{
			// Co-location only: shares a directory/substring with the change but names no concrete
			// failure and no basis. Insufficient scope — a follow-up, never a blocker.
			name: "co-location without a basis",
			finding: deskkit.ReviewFinding{
				ClassID:  "same-dir-typo",
				Location: "src/pay_notes.md:2",
			},
			want: deskkit.DispFollowUp,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, reason := deskkit.ClassifyFinding(c.finding)
			if got != c.want {
				t.Fatalf("ClassifyFinding = %q, want %q (reason: %s)", got, c.want, reason)
			}
		})
	}
}

// TestReviewScopeNoSilentPromotion — Verify row 3. A previously non-blocking occurrence
// requires changed impact or new evidence before promotion; a late sibling instance retains
// its original claim class and round count.
func TestReviewScopeNoSilentPromotion(t *testing.T) {
	// A previously non-blocking occurrence. Another file was edited, but nothing about THIS
	// occurrence changed — no changed impact, no new evidence. It must NOT be promoted, even
	// though it now carries a basis someone attached to it.
	stale := deskkit.ReviewFinding{
		ClassID:               "advisory-note",
		Location:              "docs/notes.md:12",
		Basis:                 deskkit.BasisChangedBehaviour,
		PreviouslyNonBlocking: true,
	}
	if got, reason := deskkit.ClassifyFinding(stale); got != deskkit.DispFollowUp {
		t.Fatalf("a previously non-blocking occurrence with no changed impact/new evidence was promoted to %q (reason: %s) — "+
			"another file being edited is not grounds to promote", got, reason)
	}

	// The SAME occurrence, now with changed impact, may be promoted: a genuine change is the
	// thing that distinguishes a real new blocker from a bypass of the standing rejection.
	promoted := stale
	promoted.ChangedImpact = true
	if got, reason := deskkit.ClassifyFinding(promoted); got != deskkit.DispBlocker {
		t.Fatalf("a previously non-blocking occurrence WITH changed impact must be promotable to a blocker; got %q (reason: %s)", got, reason)
	}
	// New evidence is the other admissible ground.
	viaEvidence := stale
	viaEvidence.ChangedImpact = false
	viaEvidence.NewEvidence = true
	if got, _ := deskkit.ClassifyFinding(viaEvidence); got != deskkit.DispBlocker {
		t.Fatalf("new evidence must also permit promotion; got %q", got)
	}

	// Late sibling retains its original claim class and round count. Start a class already two
	// rounds in, then absorb a newly-discovered sibling occurrence.
	class := deskkit.ClaimClass{
		ID:         "settles-in-one-block",
		RoundCount: 2,
		Occurrences: []deskkit.Occurrence{
			{ClassID: "settles-in-one-block", Location: "src/pay.go:42", InScope: true},
		},
	}
	// The sibling arrives labelled with a DIFFERENT class id, as a naive re-discovery would.
	sibling := deskkit.Occurrence{ClassID: "settles-instantly", Location: "docs/overview.md:9", InScope: true}
	after := class.AbsorbSibling(sibling)

	if after.ID != class.ID {
		t.Errorf("absorbing a late sibling changed the class id from %q to %q — a missed occurrence is not a fresh class", class.ID, after.ID)
	}
	if after.RoundCount != class.RoundCount {
		t.Errorf("absorbing a late sibling reset the round count %d -> %d — coverage failure does not re-open the counter", class.RoundCount, after.RoundCount)
	}
	if n := len(after.Occurrences); n != 2 {
		t.Fatalf("absorbed class must carry both occurrences; got %d", n)
	}
	// The sibling is re-stamped into the existing class, not left under its re-discovered id.
	found := false
	for _, o := range after.Occurrences {
		if o.Location == "docs/overview.md:9" {
			found = true
			if o.ClassID != class.ID {
				t.Errorf("late sibling kept its re-discovered class id %q instead of the original %q", o.ClassID, class.ID)
			}
		}
	}
	if !found {
		t.Fatalf("absorbed sibling occurrence is missing from the class")
	}
	// The original class value is not mutated (AbsorbSibling returns by value).
	if len(class.Occurrences) != 1 {
		t.Errorf("AbsorbSibling mutated the receiver's occurrences (len now %d) — it must return a new value", len(class.Occurrences))
	}
}

// TestReviewScopeKitMatchesModel pins the review kit's machine-checkable basis block to
// deskkit.ScopeBases, so the document a reviewer reads and the specification the rows above
// prove cannot drift. A kit that names a basis the model does not carry — or omits one it
// does — fails here.
func TestReviewScopeKitMatchesModel(t *testing.T) {
	text, err := kitText("review")
	if err != nil {
		t.Fatalf("cannot read the review kit: %v", err)
	}
	const begin = "<!-- reviewscope:begin -->"
	const end = "<!-- reviewscope:end -->"
	bi := strings.Index(text, begin)
	ei := strings.Index(text, end)
	if bi < 0 || ei < 0 || ei < bi {
		t.Fatalf("the review kit is missing the reviewscope machine-checkable block (begin=%d end=%d)", bi, ei)
	}
	block := text[bi+len(begin) : ei]

	sep := regexp.MustCompile(`^-+$`)
	kitBases := map[string]bool{}
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) != 2 {
			continue
		}
		first := strings.TrimSpace(cells[0])
		if first == "" || first == "basis" || sep.MatchString(first) {
			continue // header row or separator
		}
		kitBases[first] = true
	}

	modelBases := map[string]bool{}
	for _, d := range deskkit.ScopeBases() {
		modelBases[string(d.Basis)] = true
	}

	if len(kitBases) == 0 {
		t.Fatalf("parsed no bases out of the kit block — the parser or the block shape is wrong")
	}
	for b := range modelBases {
		if !kitBases[b] {
			t.Errorf("the model names basis %q but the review kit's block does not — a reviewer is told a different set than the model proves", b)
		}
	}
	for b := range kitBases {
		if !modelBases[b] {
			t.Errorf("the review kit names basis %q but the model does not carry it — the kit describes a boundary the specification does not", b)
		}
	}
}
