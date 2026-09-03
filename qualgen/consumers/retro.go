package consumers

import (
	"time"

	"github.com/medici-finance/assay/qualgen/attribution"
	"github.com/medici-finance/assay/qualgen/reflex"
)

// retro.go emits the retrospective INPUT set a cadence retrospective consumes
// (spec §9.7). It is generated/logged output ONLY: this file assembles the four
// inputs into one record and hands it back. It does not schedule, run, or
// narrate a retrospective — there is no cadence, no timer, and no side effect
// here beyond composing the struct.
//
// The four inputs (spec §9.7) are consumed from already-mined/joined artifacts,
// never re-derived: the churn trend (a per-window churn series), the gate yield
// (quality/12 — reflex.LaneYield, read through its real type), the per-stage
// ledger (M3 attribution — attribution.LedgerEntry, read through its real
// type), and the budget status (this brief's own budgets.go output).

// ChurnPoint is one window's churn reading in the retro's churn-trend series.
// Churn has no exported cross-package type to reuse (churn.go lives in package
// main, which cannot be imported), so the trend is carried as this small local
// point — the same value budgets.go's WindowSample records, here as a series.
type ChurnPoint struct {
	Window     string `json:"window"`
	ChurnLines int    `json:"churn_lines"`
}

// RetroInput is the four-part retrospective input set (spec §9.7), generated for
// a cadence retrospective to consume. Every field is an INPUT the retro reads;
// nothing here is a conclusion this tool draws.
type RetroInput struct {
	// ChurnTrend is the per-window churn series.
	ChurnTrend []ChurnPoint `json:"churn_trend"`
	// GateYield is quality/12's per-lane gate-yield accounting (reflex.LaneYield).
	GateYield []reflex.LaneYield `json:"gate_yield"`
	// StageLedger is the M3 per-stage attribution ledger (attribution.LedgerEntry).
	StageLedger []attribution.LedgerEntry `json:"per_stage_ledger"`
	// BudgetStatus is the per-stream error-budget status (budgets.go output).
	BudgetStatus []BudgetStatus `json:"budget_status"`
	// GeneratedAt stamps when the input set was emitted (the only non-input
	// field — a generation timestamp, not a retrospective outcome).
	GeneratedAt time.Time `json:"generated_at"`
}

// RetroSources bundles the four already-produced input families EmitRetro
// assembles. Passing them in — rather than mining them here — keeps this a pure
// generator with no git, filesystem, or network access, the same "join input,
// not a shared mining type" discipline the rest of qualgen's consumers use.
type RetroSources struct {
	ChurnTrend   []ChurnPoint
	GateYield    []reflex.LaneYield
	StageLedger  []attribution.LedgerEntry
	BudgetStatus []BudgetStatus
	// Now is the generation timestamp; zero defaults to time.Now().UTC().
	Now time.Time
}

// EmitRetro assembles the four-part retrospective input set. It is a pure
// function of its inputs (plus the generation timestamp) — generated/logged
// output only, per spec §9.7. It runs no retrospective.
func EmitRetro(src RetroSources) RetroInput {
	now := src.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return RetroInput{
		ChurnTrend:   src.ChurnTrend,
		GateYield:    src.GateYield,
		StageLedger:  src.StageLedger,
		BudgetStatus: src.BudgetStatus,
		GeneratedAt:  now,
	}
}
