package consumers

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/qualgen/attribution"
	"github.com/medici-finance/assay/qualgen/reflex"
)

// TestRetro_EmitsFourInputSet is Verify #5: the retro output carries all four
// inputs — churn trend, gate yield, per-stage ledger, and budget status — as
// generated/logged output only.
func TestRetro_EmitsFourInputSet(t *testing.T) {
	src := RetroSources{
		ChurnTrend: []ChurnPoint{
			{Window: "w1", ChurnLines: 120},
			{Window: "w2", ChurnLines: 90},
		},
		GateYield: []reflex.LaneYield{
			{
				Lane: "security-review", Catches: 3, Escapes: 1,
				CatchRate:   reflex.Measured(0.75),
				EscapeRate:  reflex.Measured(0.25),
				LatencyCost: reflex.Measured(4.0),
			},
		},
		StageLedger: []attribution.LedgerEntry{
			{DefectID: "d1", Stream: "quality", Stage: attribution.Stage("author")},
		},
		BudgetStatus: []BudgetStatus{
			EvaluateBudget(
				Budget{Stream: "quality", MaxChurn: 100, MaxDefectInducing: 2},
				StreamMeasurement{Stream: "quality", Windows: []WindowSample{
					{Window: "w1", ChurnLines: 10, DefectInducing: 0},
					{Window: "w2", ChurnLines: 20, DefectInducing: 1},
				}},
			),
		},
		Now: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
	}

	out := EmitRetro(src)

	if len(out.ChurnTrend) != 2 {
		t.Fatalf("churn trend not carried: %d", len(out.ChurnTrend))
	}
	if len(out.GateYield) != 1 {
		t.Fatalf("gate yield not carried: %d", len(out.GateYield))
	}
	if len(out.StageLedger) != 1 {
		t.Fatalf("per-stage ledger not carried: %d", len(out.StageLedger))
	}
	if len(out.BudgetStatus) != 1 {
		t.Fatalf("budget status not carried: %d", len(out.BudgetStatus))
	}
	if out.GeneratedAt.IsZero() {
		t.Fatalf("generated_at must be stamped")
	}

	// The emitted set must round-trip to JSON carrying all four input keys — it
	// is generated/logged output a retrospective consumes.
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{"churn_trend", "gate_yield", "per_stage_ledger", "budget_status"} {
		if !strings.Contains(string(b), key) {
			t.Fatalf("emitted retro input set missing %q: %s", key, b)
		}
	}
}

// TestRetro_DefaultsGeneratedAt: a zero Now defaults to a non-zero timestamp,
// so the log line is always stamped.
func TestRetro_DefaultsGeneratedAt(t *testing.T) {
	out := EmitRetro(RetroSources{})
	if out.GeneratedAt.IsZero() {
		t.Fatalf("EmitRetro must stamp GeneratedAt when Now is zero")
	}
}

// TestRetro_PureFunctionNoMutation: EmitRetro passes its inputs through
// unchanged — it is a generator, not a mutator (spec §9.7).
func TestRetro_PureFunctionNoMutation(t *testing.T) {
	trend := []ChurnPoint{{Window: "w1", ChurnLines: 5}}
	out := EmitRetro(RetroSources{ChurnTrend: trend})
	if len(out.ChurnTrend) != 1 || out.ChurnTrend[0].ChurnLines != 5 {
		t.Fatalf("input not passed through unchanged: %+v", out.ChurnTrend)
	}
}
