package reflex

import (
	"testing"

	"github.com/medici-finance/assay/qualgen/attribution"
)

func TestResolveEscaped(t *testing.T) {
	index := BuildLedgerIndex([]attribution.LedgerEntry{
		{DefectID: "pr-1", ReviewEscape: attribution.ReviewEscape{Lanes: []string{"security"}}},
	})

	if got := ResolveEscaped("", index); got.State != StateMeasured || got.Value != false {
		t.Fatalf("no defect id: want measured(false), got %+v", got)
	}
	if got := ResolveEscaped("pr-1", index); got.State != StateMeasured || got.Value != true {
		t.Fatalf("defect on ledger: want measured(true), got %+v", got)
	}
	if got := ResolveEscaped("pr-not-yet-attributed", index); got.State != StateCouldNotMeasure {
		t.Fatalf("defect absent from ledger: want could-not-measure, got %+v", got)
	}
}

func TestComputeCostPerDurableKLOC(t *testing.T) {
	changes := []AuthoredChange{
		{ModelTier: "strong", BrittlenessBand: BandHigh, CostUSD: Measured(100.0), DurableKLOC: Measured(2.0)},
		{ModelTier: "strong", BrittlenessBand: BandHigh, CostUSD: Measured(50.0), DurableKLOC: Measured(1.0)},
		{ModelTier: "strong", BrittlenessBand: BandLow, CostUSD: Measured(30.0), DurableKLOC: Measured(3.0)},
		{ModelTier: "default", BrittlenessBand: BandHigh, CostUSD: Measured(200.0), DurableKLOC: Measured(1.0)},
		// Missing cost: excluded from the sum but still counted in SampleSize.
		{ModelTier: "default", BrittlenessBand: BandHigh, CostUSD: CouldNotMeasure[float64]("no spend record"), DurableKLOC: Measured(5.0)},
	}

	got := ComputeCostPerDurableKLOC(changes)

	byKey := map[string]CostPerKLOCReadout{}
	for _, r := range got {
		byKey[string(r.ModelTier)+"/"+string(r.Band)] = r
	}

	strongHigh := byKey["strong/high"]
	if strongHigh.CostPerKLOC.State != StateMeasured || strongHigh.CostPerKLOC.Value != 50.0 {
		// (100+50) / (2+1) = 50
		t.Fatalf("strong/high cost-per-KLOC: want measured 50, got %+v", strongHigh.CostPerKLOC)
	}
	if strongHigh.SampleSize != 2 {
		t.Fatalf("strong/high sample size: want 2, got %d", strongHigh.SampleSize)
	}

	strongLow := byKey["strong/low"]
	if strongLow.CostPerKLOC.State != StateMeasured || strongLow.CostPerKLOC.Value != 10.0 {
		t.Fatalf("strong/low cost-per-KLOC: want measured 10, got %+v", strongLow.CostPerKLOC)
	}

	defaultHigh := byKey["default/high"]
	if defaultHigh.SampleSize != 2 {
		t.Fatalf("default/high sample size: want 2 (one measured, one not), got %d", defaultHigh.SampleSize)
	}
	if defaultHigh.CostPerKLOC.State != StateMeasured || defaultHigh.CostPerKLOC.Value != 200.0 {
		t.Fatalf("default/high cost-per-KLOC (only the measured change contributes): want measured 200, got %+v", defaultHigh.CostPerKLOC)
	}
}

func TestComputeCostPerDurableKLOC_AllUnmeasuredIsCouldNotMeasure(t *testing.T) {
	changes := []AuthoredChange{
		{ModelTier: "strong", BrittlenessBand: BandHigh, CostUSD: CouldNotMeasure[float64]("x"), DurableKLOC: Measured(1.0)},
	}
	got := ComputeCostPerDurableKLOC(changes)
	if len(got) != 1 {
		t.Fatalf("want 1 stratum, got %d", len(got))
	}
	if got[0].CostPerKLOC.State != StateCouldNotMeasure {
		t.Fatalf("want could-not-measure, got %+v", got[0].CostPerKLOC)
	}
	if got[0].SampleSize != 1 {
		t.Fatalf("sample size should still count the unmeasured change: want 1, got %d", got[0].SampleSize)
	}
}

func TestComputeCostPerDurableKLOC_ZeroKLOCSumIsCouldNotMeasure(t *testing.T) {
	changes := []AuthoredChange{
		{ModelTier: "strong", BrittlenessBand: BandLow, CostUSD: Measured(10.0), DurableKLOC: MeasuredZero[float64]()},
	}
	got := ComputeCostPerDurableKLOC(changes)
	if got[0].CostPerKLOC.State != StateCouldNotMeasure {
		t.Fatalf("a zero durable-KLOC denominator must be could-not-measure, not a division by zero: got %+v", got[0].CostPerKLOC)
	}
}

func TestComputeVerifyDepthVsEscapeRate(t *testing.T) {
	changes := []AuthoredChange{
		{VerifyDepth: 2, BrittlenessBand: BandHigh, Escaped: Measured(true)},
		{VerifyDepth: 2, BrittlenessBand: BandHigh, Escaped: Measured(false)},
		{VerifyDepth: 2, BrittlenessBand: BandHigh, Escaped: Measured(false)},
		{VerifyDepth: 2, BrittlenessBand: BandHigh, Escaped: Measured(false)},
		{VerifyDepth: 8, BrittlenessBand: BandHigh, Escaped: Measured(false)},
		{VerifyDepth: 8, BrittlenessBand: BandHigh, Escaped: Measured(false)},
		{VerifyDepth: 8, BrittlenessBand: BandHigh, Escaped: CouldNotMeasure[bool]("not yet attributed")},
	}
	got := ComputeVerifyDepthVsEscapeRate(changes, nil)

	byKey := map[string]VerifyDepthEscapeReadout{}
	for _, r := range got {
		byKey[r.DepthBucket+"/"+string(r.Band)] = r
	}

	shallow := byKey["shallow/high"]
	if shallow.EscapeRate.State != StateMeasured || shallow.EscapeRate.Value != 0.25 {
		t.Fatalf("shallow/high escape-rate: want measured 0.25, got %+v", shallow.EscapeRate)
	}
	if shallow.Measured != 4 || shallow.Unmeasured != 0 {
		t.Fatalf("shallow/high coverage: want measured=4 unmeasured=0, got measured=%d unmeasured=%d", shallow.Measured, shallow.Unmeasured)
	}

	deep := byKey["deep/high"]
	if deep.EscapeRate.State != StateMeasured || deep.EscapeRate.Value != 0.0 {
		t.Fatalf("deep/high escape-rate: want measured 0.0, got %+v", deep.EscapeRate)
	}
	if deep.Measured != 2 || deep.Unmeasured != 1 {
		t.Fatalf("deep/high coverage: want measured=2 unmeasured=1, got measured=%d unmeasured=%d", deep.Measured, deep.Unmeasured)
	}
}

func TestComputeVerifyDepthVsEscapeRate_NoMeasuredOutcomeIsCouldNotMeasure(t *testing.T) {
	changes := []AuthoredChange{
		{VerifyDepth: 1, BrittlenessBand: BandLow, Escaped: CouldNotMeasure[bool]("x")},
	}
	got := ComputeVerifyDepthVsEscapeRate(changes, nil)
	if got[0].EscapeRate.State != StateCouldNotMeasure {
		t.Fatalf("want could-not-measure, got %+v", got[0].EscapeRate)
	}
	if got[0].Unmeasured != 1 {
		t.Fatalf("want unmeasured=1, got %d", got[0].Unmeasured)
	}
}

func TestBudgets(t *testing.T) {
	changes := []AuthoredChange{
		{SurvivedNDays: Measured(true), FirstPassApproved: Measured(true), MergedWithoutReview: Measured(false), TimeInReviewHours: Measured(2.0)},
		{SurvivedNDays: Measured(true), FirstPassApproved: Measured(false), MergedWithoutReview: Measured(false), TimeInReviewHours: Measured(4.0)},
		{SurvivedNDays: Measured(false), FirstPassApproved: Measured(false), MergedWithoutReview: Measured(true), TimeInReviewHours: Measured(6.0)},
	}

	survival := ComputeAgentPRSurvivalRate(changes, 0.7)
	if survival.Value.State != StateMeasured || survival.Value.Value < 0.666 || survival.Value.Value > 0.667 {
		t.Fatalf("survival rate: want ~0.667, got %+v", survival.Value)
	}
	if !survival.Breached {
		t.Fatalf("survival rate 0.667 < threshold 0.7 must breach")
	}

	firstPass := ComputeFirstPassApprovalRate(changes, 0.2)
	if firstPass.Breached {
		t.Fatalf("first-pass rate ~0.333 >= threshold 0.2 must NOT breach")
	}

	guardrails := ComputeReviewDisciplineGuardrails(changes, 0.5, 3.0)
	if guardrails.MergedWithoutReview.Value.State != StateMeasured {
		t.Fatalf("merged-without-review: want measured, got %+v", guardrails.MergedWithoutReview.Value)
	}
	if guardrails.MergedWithoutReview.Breached {
		t.Fatalf("merged-without-review ~0.333 <= threshold 0.5 must NOT breach")
	}
	if guardrails.TimeInReview.Value.State != StateMeasured || guardrails.TimeInReview.Value.Value != 4.0 {
		t.Fatalf("time-in-review mean: want measured 4.0, got %+v", guardrails.TimeInReview.Value)
	}
	if !guardrails.TimeInReview.Breached {
		t.Fatalf("time-in-review 4.0 > threshold 3.0 must breach")
	}
}

func TestBudgetUnmeasuredNeverBreaches(t *testing.T) {
	var changes []AuthoredChange // empty corpus: every signal could-not-measure
	survival := ComputeAgentPRSurvivalRate(changes, 0.9)
	if survival.Value.State != StateCouldNotMeasure {
		t.Fatalf("empty corpus: want could-not-measure, got %+v", survival.Value)
	}
	if survival.Breached {
		t.Fatalf("an unmeasured budget must never be reported as breached")
	}
}
