package main

import (
	"regexp"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestRefusesSameVendor pins the calibration SPOF: a re-review whose model vendor
// equals the reviewer role's vendor measures two instances of one model, not an
// independent judge, so it is refused; perturbing the vendor to a different one
// accepts. A human re-reviewer is independent of every model vendor and accepts.
func TestRefusesSameVendor(t *testing.T) {
	const reviewer = "anthropic"

	// Same vendor → refused, with the deskkit refusal exit code.
	if err := CheckIndependence(reviewer, ReReviewer{Vendor: "anthropic"}); err == nil {
		t.Fatalf("same-vendor re-review (%q vs %q) was accepted; the calibration SPOF must refuse it", reviewer, "anthropic")
	} else if got := deskkit.ExitCodeOf(err); got != deskkit.ExitRefused {
		t.Fatalf("same-vendor refusal has exit code %d, want ExitRefused (%d)", got, deskkit.ExitRefused)
	}

	// Case/whitespace variants of the SAME vendor are still the same vendor.
	for _, same := range []string{"Anthropic", "  anthropic ", "ANTHROPIC"} {
		if err := CheckIndependence(reviewer, ReReviewer{Vendor: same}); err == nil {
			t.Errorf("re-reviewer vendor %q is the same vendor as %q but was accepted", same, reviewer)
		}
	}

	// Perturb the vendor → accepted.
	if err := CheckIndependence(reviewer, ReReviewer{Vendor: "openai"}); err != nil {
		t.Errorf("different-vendor re-review (%q vs %q) was refused: %v", reviewer, "openai", err)
	}

	// A human checklist is independent by construction.
	if err := CheckIndependence(reviewer, ReReviewer{Human: "ada"}); err != nil {
		t.Errorf("human re-reviewer was refused: %v", err)
	}

	// Fail-closed: an unknown reviewer vendor cannot prove independence.
	if err := CheckIndependence("", ReReviewer{Vendor: "openai"}); err == nil {
		t.Errorf("unknown reviewer vendor was accepted; independence cannot be proven against an unrecorded vendor")
	}
	// ...and an unset re-reviewer is a refusal, not a pass.
	if err := CheckIndependence(reviewer, ReReviewer{}); err == nil {
		t.Errorf("empty re-reviewer was accepted; must refuse")
	}
}

// TestSampleIsSeeded pins reproducibility: the same seed over the same candidate
// set yields the same ten PRs, and a different seed yields a different set.
func TestSampleIsSeeded(t *testing.T) {
	// A candidate pool larger than the sample so a different seed CAN pick a
	// different subset.
	candidates := make([]int, 0, 40)
	for i := 100; i < 140; i++ {
		candidates = append(candidates, i)
	}

	a1, err := SeededSample(candidates, 42, 10)
	if err != nil {
		t.Fatalf("seed 42: %v", err)
	}
	a2, err := SeededSample(candidates, 42, 10)
	if err != nil {
		t.Fatalf("seed 42 (repeat): %v", err)
	}
	if !equalInts(a1, a2) {
		t.Fatalf("same seed produced different samples:\n  %v\n  %v", a1, a2)
	}
	if len(a1) != 10 {
		t.Fatalf("sample size = %d, want 10", len(a1))
	}

	// Same seed is stable even when the candidates arrive in a different order:
	// the sample is a function of the SET, not the input order.
	shuffled := append([]int(nil), candidates...)
	for i := range shuffled {
		j := (i * 7) % len(shuffled)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}
	a3, err := SeededSample(shuffled, 42, 10)
	if err != nil {
		t.Fatalf("seed 42 (reordered candidates): %v", err)
	}
	if !equalInts(a1, a3) {
		t.Fatalf("same seed over reordered candidates changed the sample:\n  %v\n  %v", a1, a3)
	}

	// A different seed generally yields a different set.
	b, err := SeededSample(candidates, 43, 10)
	if err != nil {
		t.Fatalf("seed 43: %v", err)
	}
	if equalInts(a1, b) {
		t.Fatalf("different seeds produced the same sample: %v", a1)
	}

	// Every sampled PR is a real candidate, and there are no duplicates.
	set := map[int]struct{}{}
	for _, c := range candidates {
		set[c] = struct{}{}
	}
	seen := map[int]struct{}{}
	for _, pr := range a1 {
		if _, ok := set[pr]; !ok {
			t.Errorf("sampled PR %d is not a candidate", pr)
		}
		if _, dup := seen[pr]; dup {
			t.Errorf("sampled PR %d appears twice", pr)
		}
		seen[pr] = struct{}{}
	}

	// Too few candidates for the sample size is a refusal, not a short sample.
	if _, err := SeededSample([]int{1, 2, 3}, 1, 10); err == nil {
		t.Errorf("sampling 10 from 3 candidates was accepted; must refuse")
	}
}

// TestReportRendersFraction pins Verify row 4's contract: the agreement metric is
// an explicit numerator/denominator fraction, never a bare percentage.
func TestReportRendersFraction(t *testing.T) {
	rep := Report{
		Month:          "2026-08",
		Seed:           42,
		ReviewerVendor: "anthropic",
		ReReviewer:     ReReviewer{Vendor: "deepseek"},
		Sample:         []int{1, 2, 3},
		Verdicts: []PRVerdict{
			{PR: 1, ReReviewerApproves: true},
			{PR: 2, ReReviewerApproves: false, MissedFindings: []string{"unchecked nil deref at foo.go:12"}},
			{PR: 3, ReReviewerApproves: true},
		},
	}
	md, err := rep.Render()
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !regexp.MustCompile(`(?m)^agreement: 2/3$`).MatchString(md) {
		t.Errorf("report does not carry `agreement: 2/3` as a fraction:\n%s", md)
	}
	// No bare percentage anywhere.
	if regexp.MustCompile(`\d+(\.\d+)?%`).MatchString(md) {
		t.Errorf("report contains a bare percentage; the metric must be fraction-only:\n%s", md)
	}
	if !strings.Contains(md, "unchecked nil deref at foo.go:12") {
		t.Errorf("disagreement did not list the missed finding:\n%s", md)
	}

	// A same-vendor report refuses to render.
	bad := rep
	bad.ReReviewer = ReReviewer{Vendor: "anthropic"}
	if _, err := bad.Render(); err == nil {
		t.Errorf("same-vendor report rendered; must refuse (SPOF enforced at render time)")
	}

	// An empty result is a could-not-check, never 0/0.
	empty := rep
	empty.Verdicts = nil
	if _, err := empty.Render(); err == nil {
		t.Errorf("empty result rendered; agreement of 0/0 must refuse")
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
