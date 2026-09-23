package main

// Reviewer-calibration reversal mining (verify-integrity/10).
//
// The join point is the gate-yield accounting that already reads PR review
// threads (--gate-telemetry's pr-verdicts.json / gatetelemetry.go). That
// instrument mines override-rate at the PR level — a human outcome overturning
// an App APPROVED verdict. THIS file mines one level finer: the per-finding-CLASS
// reversal rate, the fraction of findings of a class the worker disputed and the
// reviewer conceded (or the desk overruled), over a month. A class whose findings
// are mostly reversed is measuring noise, and the demotion rule below is how a
// noisy class is moved to advisory so it stops blocking merges.
//
// Everything here is a pure function of its inputs so it is unit-testable without
// a forge, and it uses INTEGER fraction arithmetic throughout: a reversal rate is
// a numerator/denominator (findings-reversed / findings-of-class), and the >50%
// threshold is `2*num > den`, never a float rounded to a percentage. Same reason
// the report renders a fraction: a bare percentage hides its denominator, and a
// 3/5 month and a 30/50 month are not equally strong evidence.

// Fraction is a reversal rate as an explicit numerator over denominator. Den == 0
// means "no findings of this class this month" — a could-not-check for the
// demotion rule, never a rate of 0.
type Fraction struct {
	Num int // findings reversed (disputed→conceded, or desk-overruled)
	Den int // findings of the class
}

// exceedsHalf reports whether the rate is strictly greater than 50% (the demotion
// trigger). Integer arithmetic: 2*Num > Den. A zero-denominator month never
// exceeds the threshold — it is an absence of evidence, not a low rate.
func (f Fraction) exceedsHalf() bool { return f.Den > 0 && 2*f.Num > f.Den }

// belowHalf reports whether the rate is strictly below 50% (the restore trigger).
// A zero-denominator month is NOT "below half" either — it neither demotes nor
// restores; a class with no findings this month keeps whatever state it had.
func (f Fraction) belowHalf() bool { return f.Den > 0 && 2*f.Num < f.Den }

// FindingOutcome is one reviewer finding's terminal state on a PR thread, mined
// from the same review-thread source the gate-yield accounting reads. Only the
// two fields the reversal rate needs are modelled here.
type FindingOutcome struct {
	Class    string `json:"class"`
	Reversed bool   `json:"reversed"` // worker disputed AND reviewer conceded, or the desk overruled
}

// ClassReversalRates mines the per-finding-class reversal rate for one month from
// a flat list of finding outcomes. The result maps each class to its fraction
// (reversed / total-of-class). A class absent from the input is absent from the
// result — no findings is not a rate of 0 (see Fraction).
func ClassReversalRates(findings []FindingOutcome) map[string]Fraction {
	out := map[string]Fraction{}
	for _, f := range findings {
		if f.Class == "" {
			continue
		}
		fr := out[f.Class]
		fr.Den++
		if f.Reversed {
			fr.Num++
		}
		out[f.Class] = fr
	}
	return out
}

// demoteToAdvisory applies the two-month demotion rule to one class's monthly
// reversal rates, oldest first:
//
//   - a reversal rate > 50% for two CONSECUTIVE months marks the class advisory;
//   - a later month < 50% restores it (active).
//
// A month with no findings (Den == 0) is a could-not-check: it neither advances
// the demotion streak nor restores the class — the state carries across it
// unchanged. Exactly 50% (2*Num == Den) is likewise neutral: the brief triggers
// on ">50%" and restores on "under 50%", so the boundary does neither.
//
// It returns true when the class is currently advisory after walking the whole
// series.
func demoteToAdvisory(monthlyOldestFirst []Fraction) bool {
	advisory := false
	streak := 0 // consecutive months strictly above 50%, most recent run
	for _, m := range monthlyOldestFirst {
		switch {
		case m.exceedsHalf():
			streak++
			if streak >= 2 {
				advisory = true
			}
		case m.belowHalf():
			streak = 0
			advisory = false // a later month under 50% restores the class
		default:
			// Den == 0 (no findings) or exactly 50%: neutral. Break the
			// consecutive-months streak — the two months must be consecutive
			// ABOVE-50% months — but leave the advisory state as it stands.
			streak = 0
		}
	}
	return advisory
}
