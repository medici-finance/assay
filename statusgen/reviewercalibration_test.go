package main

import "testing"

// TestReversalRateDemotes pins the demotion rule (brief Verify row 3): a per-class
// reversal rate above 50% for two consecutive months marks the class advisory; a
// later month under 50% restores it.
func TestReversalRateDemotes(t *testing.T) {
	pct := func(num, den int) Fraction { return Fraction{Num: num, Den: den} }

	cases := []struct {
		name    string
		months  []Fraction
		wantAdv bool
	}{
		{
			// 60% then 55% over two consecutive months → advisory.
			name:    "60 then 55 -> advisory",
			months:  []Fraction{pct(6, 10), pct(11, 20)},
			wantAdv: true,
		},
		{
			// 60% then 40% → not advisory (the second month is under 50%).
			name:    "60 then 40 -> not advisory",
			months:  []Fraction{pct(6, 10), pct(4, 10)},
			wantAdv: false,
		},
		{
			// A single high month is not two consecutive months.
			name:    "single high month -> not advisory",
			months:  []Fraction{pct(9, 10)},
			wantAdv: false,
		},
		{
			// Demoted, then a later month under 50% restores it.
			name:    "demoted then restored",
			months:  []Fraction{pct(6, 10), pct(6, 10), pct(3, 10)},
			wantAdv: false,
		},
		{
			// Restored, then two more high months re-demote.
			name:    "restored then re-demoted",
			months:  []Fraction{pct(6, 10), pct(6, 10), pct(3, 10), pct(6, 10), pct(7, 10)},
			wantAdv: true,
		},
		{
			// Exactly 50% is neither a trigger nor a restore.
			name:    "exactly half is neutral",
			months:  []Fraction{pct(5, 10), pct(5, 10)},
			wantAdv: false,
		},
		{
			// A no-findings month does not restore an advisory class, but it does
			// break the consecutive-above-50% streak.
			name:    "gap month breaks the streak",
			months:  []Fraction{pct(6, 10), pct(0, 0), pct(6, 10)},
			wantAdv: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := demoteToAdvisory(c.months); got != c.wantAdv {
				t.Errorf("demoteToAdvisory(%v) = %v, want %v", c.months, got, c.wantAdv)
			}
		})
	}
}

// TestClassReversalRates checks the mining: findings tally per class into a
// reversed/total fraction, and a class with no findings is absent (not 0/0).
func TestClassReversalRates(t *testing.T) {
	findings := []FindingOutcome{
		{Class: "style", Reversed: true},
		{Class: "style", Reversed: true},
		{Class: "style", Reversed: false},
		{Class: "correctness", Reversed: false},
		{Class: "correctness", Reversed: false},
		{Class: "", Reversed: true}, // unclassed → ignored
	}
	rates := ClassReversalRates(findings)

	if got := rates["style"]; got.Num != 2 || got.Den != 3 {
		t.Errorf("style reversal rate = %d/%d, want 2/3", got.Num, got.Den)
	}
	if !rates["style"].exceedsHalf() {
		t.Errorf("style (2/3) should exceed 50%%")
	}
	if got := rates["correctness"]; got.Num != 0 || got.Den != 2 {
		t.Errorf("correctness reversal rate = %d/%d, want 0/2", got.Num, got.Den)
	}
	if rates["correctness"].exceedsHalf() {
		t.Errorf("correctness (0/2) should not exceed 50%%")
	}
	if _, ok := rates[""]; ok {
		t.Errorf("unclassed findings must not produce a class")
	}
	if _, ok := rates["nonexistent"]; ok {
		t.Errorf("a class with no findings must be absent, not 0/0")
	}
}
