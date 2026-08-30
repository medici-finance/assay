package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/medici-finance/assay/qualgen/adapters"
)

// --- a minimal stub LinkageAdapter, for exercising ClassifyFix's precedence
// logic directly (independent of the reference adapter under test elsewhere). ---

type stubLinkageAdapter struct {
	closedIssue    IssueRef
	hasClosedIssue bool
	closedIssueErr error
	defectClassed  bool
	defectClassErr error
	isFixTax       bool
	isFixTaxErr    error
	hasKeyword     bool
}

func (s stubLinkageAdapter) ClosedIssue(FixCandidate) (IssueRef, bool, error) {
	return s.closedIssue, s.hasClosedIssue, s.closedIssueErr
}
func (s stubLinkageAdapter) IssueDefectClassed(IssueRef) (bool, error) {
	return s.defectClassed, s.defectClassErr
}
func (s stubLinkageAdapter) PRIsFixTaxonomy(FixCandidate) (bool, error) {
	return s.isFixTax, s.isFixTaxErr
}
func (s stubLinkageAdapter) MessageHasFixKeyword(FixCandidate) bool {
	return s.hasKeyword
}

// TestFixLinkage_ClassifierPrecedence pins the tier-precedence order directly
// against a stub adapter: whichever tier could match, ClassifyFix stops at the
// STRONGEST one, never falls through past it.
func TestFixLinkage_ClassifierPrecedence(t *testing.T) {
	c := FixCandidate{CommitSHA: "deadbeef"}

	t.Run("tier 1 wins even when tier 2 and 3 would also match", func(t *testing.T) {
		a := stubLinkageAdapter{
			closedIssue: IssueRef{Number: 7}, hasClosedIssue: true,
			defectClassed: true,
			isFixTax:      true,
			hasKeyword:    true,
		}
		fix, ok := ClassifyFix(a, c)
		if !ok || fix.Tier != Tier1 {
			t.Fatalf("expected tier 1, got fix=%+v ok=%v", fix, ok)
		}
		if fix.ClosedIssue == nil || fix.ClosedIssue.Number != 7 {
			t.Fatalf("expected closed issue #7 recorded, got %+v", fix.ClosedIssue)
		}
	})

	t.Run("tier 2 wins over tier 3 when tier 1 does not match", func(t *testing.T) {
		a := stubLinkageAdapter{isFixTax: true, hasKeyword: true}
		fix, ok := ClassifyFix(a, c)
		if !ok || fix.Tier != Tier2 {
			t.Fatalf("expected tier 2, got fix=%+v ok=%v", fix, ok)
		}
	})

	t.Run("tier 3 is the last resort", func(t *testing.T) {
		a := stubLinkageAdapter{hasKeyword: true}
		fix, ok := ClassifyFix(a, c)
		if !ok || fix.Tier != Tier3 {
			t.Fatalf("expected tier 3, got fix=%+v ok=%v", fix, ok)
		}
	})

	t.Run("no tier matches: confirmed non-fix, not recorded", func(t *testing.T) {
		a := stubLinkageAdapter{}
		fix, ok := ClassifyFix(a, c)
		if ok {
			t.Fatalf("expected a confirmed non-fix to not be recorded, got fix=%+v ok=%v", fix, ok)
		}
	})

	t.Run("an adapter error is could-not-identify, never a silent non-fix", func(t *testing.T) {
		a := stubLinkageAdapter{closedIssueErr: fmt.Errorf("boom")}
		fix, ok := ClassifyFix(a, c)
		if !ok {
			t.Fatal("an unresolvable candidate must still be recorded (as could-not-identify)")
		}
		if fix.Identified.State != StateCouldNotMeasure {
			t.Fatalf("expected could-not-measure, got %+v", fix.Identified)
		}
		if fix.Tier != "" {
			t.Fatalf("a could-not-identify record must carry no tier, got %q", fix.Tier)
		}
	})
}

// --- the planted-fixture, known-answer tests (Verify rows 3-6). These exercise
// the REAL reference adapter (adapters.GithubLabels via GithubLabelsLinkage),
// not the stub above, so the classifier and the reference adapter are proven
// to work together end to end. ---

// plantedFixture mirrors testdata/fixid/planted.json.
type plantedFixture struct {
	Candidates []struct {
		ID        string `json:"id"`
		CommitSHA string `json:"commit_sha"`
		PRNumber  int    `json:"pr_number"`
		PRBranch  string `json:"pr_branch"`
		PRTitle   string `json:"pr_title"`
		Message   string `json:"message"`
	} `json:"candidates"`
	IssueLabels        map[string][]string `json:"issue_labels"`
	UnresolvableIssues []int               `json:"unresolvable_issues"`
}

// stubIssueLabelSource backs adapters.GithubLabels in these tests: it answers
// from the planted fixture's issue_labels map, and returns an error for any
// issue listed under unresolvable_issues — simulating a linkage lookup that
// genuinely cannot be resolved (not found, rate-limited, ...).
type stubIssueLabelSource struct {
	labels       map[string][]string
	unresolvable map[int]bool
}

func (s stubIssueLabelSource) IssueLabels(repo string, number int) ([]string, error) {
	if s.unresolvable[number] {
		return nil, fmt.Errorf("issue #%d: not found (stub)", number)
	}
	return s.labels[fmt.Sprintf("%d", number)], nil
}

func loadPlantedFixture(t *testing.T) (plantedFixture, map[string]FixCandidate, LinkageAdapter) {
	t.Helper()
	raw, err := os.ReadFile("testdata/fixid/planted.json")
	if err != nil {
		t.Fatalf("reading planted fixture: %v", err)
	}
	var fx plantedFixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("parsing planted fixture: %v", err)
	}

	candidates := make(map[string]FixCandidate, len(fx.Candidates))
	for _, c := range fx.Candidates {
		candidates[c.ID] = FixCandidate{
			CommitSHA: c.CommitSHA,
			PRNumber:  c.PRNumber,
			PRBranch:  c.PRBranch,
			PRTitle:   c.PRTitle,
			Message:   c.Message,
		}
	}

	unresolvable := make(map[int]bool, len(fx.UnresolvableIssues))
	for _, n := range fx.UnresolvableIssues {
		unresolvable[n] = true
	}
	source := stubIssueLabelSource{labels: fx.IssueLabels, unresolvable: unresolvable}
	adapter := GithubLabelsLinkage{Impl: adapters.NewGithubLabels(source)}
	return fx, candidates, adapter
}

// TestFixID_TierPrecedence_PlantedFixtures dereferences the known-answer
// planted fixture: the defect-labeled-issue closer lands at EXACTLY tier 1,
// the fix/-branch + fix:-title PR at EXACTLY tier 2, and the keyword-only
// commit at EXACTLY tier 3 (Verify row 3).
func TestFixID_TierPrecedence_PlantedFixtures(t *testing.T) {
	_, candidates, adapter := loadPlantedFixture(t)

	cases := []struct {
		id   string
		tier EvidenceTier
	}{
		{"tier1-defect-labeled-issue-closer", Tier1},
		{"tier2-fix-branch-and-title", Tier2},
		{"tier3-keyword-only", Tier3},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			c, found := candidates[tc.id]
			if !found {
				t.Fatalf("fixture missing candidate %q", tc.id)
			}
			fix, ok := ClassifyFix(adapter, c)
			if !ok {
				t.Fatalf("expected %q to be identified as a fix, got non-fix", tc.id)
			}
			if fix.Identified.State != StateMeasured {
				t.Fatalf("expected %q identified=measured, got %+v", tc.id, fix.Identified)
			}
			if fix.Tier != tc.tier {
				t.Fatalf("%q: expected %s, got %s", tc.id, tc.tier, fix.Tier)
			}
		})
	}
}

// TestFixID_TierComposition_Reported dereferences the known tier split over
// the whole planted set: 1 tier-1 / 1 tier-2 / 1 tier-3, with tier 3 reported
// SEPARATELY — never folded into the tier-1/2 sum (Verify row 4).
func TestFixID_TierComposition_Reported(t *testing.T) {
	fx, candidates, adapter := loadPlantedFixture(t)

	var fixes []DefectFix
	for _, c := range fx.Candidates {
		fix, ok := ClassifyFix(adapter, candidates[c.ID])
		if ok {
			fixes = append(fixes, fix)
		}
	}

	tc := ComputeTierComposition(fixes)
	if tc.Tier1Count != 1 || tc.Tier2Count != 1 || tc.Tier3Count != 1 {
		t.Fatalf("expected 1/1/1 tier split, got %+v", tc)
	}
	if got := tc.Tier1And2Count(); got != 2 {
		t.Fatalf("Tier1And2Count must exclude tier 3 (expected 2, got %d) — tier 3 must never be folded in", got)
	}
}

// TestFixID_NonFixNotClassified dereferences the two negative cases: the
// planted non-fix is NOT recorded as a DefectFix at all, and the unresolvable
// candidate is could-not-identify — never a silent non-fix (Verify row 5).
func TestFixID_NonFixNotClassified(t *testing.T) {
	_, candidates, adapter := loadPlantedFixture(t)

	t.Run("non-fix is not recorded", func(t *testing.T) {
		fix, ok := ClassifyFix(adapter, candidates["non-fix"])
		if ok {
			t.Fatalf("expected the planted non-fix to not be recorded as a DefectFix, got %+v", fix)
		}
	})

	t.Run("unresolvable candidate is could-not-identify", func(t *testing.T) {
		fix, ok := ClassifyFix(adapter, candidates["unresolvable-issue-lookup"])
		if !ok {
			t.Fatal("an unresolvable candidate must still be recorded, as could-not-identify")
		}
		if fix.Identified.State != StateCouldNotMeasure {
			t.Fatalf("expected could-not-measure, got %+v", fix.Identified)
		}
		if fix.Identified.Reason == "" {
			t.Fatal("could-not-measure requires a non-empty reason")
		}
		if fix.Tier != "" {
			t.Fatalf("a could-not-identify record must carry no tier, got %q", fix.Tier)
		}
	})
}

// TestDefectFix_RecordContract_ForSZZ proves the DefectFix record round-trips
// through JSON carrying exactly the field set quality/07's B-SZZ trace
// consumes: fix identity, closed-issue ref (or none), evidence tier, and the
// three-state identified flag (Verify row 6).
func TestDefectFix_RecordContract_ForSZZ(t *testing.T) {
	t.Run("a tier-1 fix with a closed issue", func(t *testing.T) {
		in := DefectFix{
			FixCommitSHA: "c111111111111111111111111111111111111111",
			ClosedIssue:  &IssueRef{Number: 101},
			Tier:         Tier1,
			Identified:   Measured(true),
		}
		raw, err := json.Marshal(in)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var out DefectFix
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if out.FixCommitSHA != in.FixCommitSHA {
			t.Fatalf("fix identity did not round-trip: %+v", out)
		}
		if out.ClosedIssue == nil || out.ClosedIssue.Number != 101 {
			t.Fatalf("closed-issue ref did not round-trip: %+v", out.ClosedIssue)
		}
		if out.Tier != Tier1 {
			t.Fatalf("tier did not round-trip: %q", out.Tier)
		}
		if out.Identified.State != StateMeasured || out.Identified.Value != true {
			t.Fatalf("identified flag did not round-trip: %+v", out.Identified)
		}
	})

	t.Run("a could-not-identify fix carries no closed issue and no tier", func(t *testing.T) {
		in := DefectFix{
			FixCommitSHA: "c555555555555555555555555555555555555555",
			Identified:   CouldNotMeasure[bool]("issue lookup failed"),
		}
		raw, err := json.Marshal(in)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var out DefectFix
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if out.ClosedIssue != nil {
			t.Fatalf("expected no closed issue, got %+v", out.ClosedIssue)
		}
		if out.Tier != "" {
			t.Fatalf("expected no tier, got %q", out.Tier)
		}
		if out.Identified.State != StateCouldNotMeasure || out.Identified.Reason == "" {
			t.Fatalf("could-not-measure flag did not round-trip: %+v", out.Identified)
		}
	})
}

// TestDefectFix_StoreRoundTrip proves DefectFix records append into the
// defects table under the tracking root through the same Store the commit and
// diff tables use — the seam quality/07 reads from (task item 4).
func TestDefectFix_StoreRoundTrip(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	in := DefectFix{
		FixCommitSHA: "c111111111111111111111111111111111111111",
		ClosedIssue:  &IssueRef{Number: 101},
		Tier:         Tier1,
		Identified:   Measured(true),
	}
	if err := s.Append(KindDefect, in); err != nil {
		t.Fatalf("append: %v", err)
	}

	out, err := s.ReadDefects()
	if err != nil {
		t.Fatalf("read defects: %v", err)
	}
	if len(out) != 1 || out[0].FixCommitSHA != in.FixCommitSHA || out[0].Tier != Tier1 {
		t.Fatalf("defects table did not round-trip: %+v", out)
	}
}
