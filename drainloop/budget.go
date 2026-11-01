package drainloop

// CellBudget retains separate account/provider/unit/window balances. No conversion
// or cross-provider fallback is implicit. This schema supplies no reservation
// algorithm or durability claim: its future store must atomically reserve against
// consumed usage, outstanding/unknown reservations and operator headroom.
type CellBudget struct {
	Balances     []BudgetBalance     `json:"balances"`
	Reservations []BudgetReservation `json:"reservations"`
	Usage        []AttemptUsage      `json:"usage"`
}

// BudgetBalance represents a hard brake. Nil Consumed means unknown, not zero.
// Window timestamps are RFC3339; rolling and longer periods remain separate rows.
// Ref names the approved policy; no commercial prices or defaults are embedded.
type BudgetBalance struct {
	Ref         string  `json:"ref"`
	Account     string  `json:"account"`
	Provider    string  `json:"provider"`
	Unit        string  `json:"unit"`
	WindowStart string  `json:"window_start"`
	WindowEnd   string  `json:"window_end"`
	Limit       uint64  `json:"limit"`
	Headroom    uint64  `json:"headroom"`
	Consumed    *uint64 `json:"consumed"`
}

// BudgetReservation remains outstanding until authoritative reconciliation.
// Unknown usage does not release Amount. WorkID is the original work charged.
type BudgetReservation struct {
	ID         string `json:"id"`
	WorkID     string `json:"work_id"`
	AttemptID  string `json:"attempt_id"`
	RequestID  string `json:"request_id"`
	BalanceRef string `json:"balance_ref"`
	Amount     uint64 `json:"amount"`
	Unknown    bool   `json:"unknown"`
}

// AttemptUsage accounts for preparation, summary, failed attempts, fallback and
// verification against original work. Nil telemetry is unknown. SourceRef names
// an authorized, trust-gated artifact, never credentials or an arbitrary payload.
// Pricing identity describes a measurement, not permission or a fixed rate.
type AttemptUsage struct {
	WorkID          string  `json:"work_id"`
	AttemptID       string  `json:"attempt_id"`
	ParentAttemptID string  `json:"parent_attempt_id"`
	RequestID       string  `json:"request_id"`
	Phase           string  `json:"phase"`
	Outcome         string  `json:"outcome"`
	Account         string  `json:"account"`
	Provider        string  `json:"provider"`
	Model           string  `json:"model"`
	PricingRef      string  `json:"pricing_ref"`
	SourceRef       string  `json:"source_ref"`
	StartedAt       string  `json:"started_at"`
	FinishedAt      string  `json:"finished_at"`
	ControlBoundary string  `json:"control_boundary"`
	Input           *uint64 `json:"input"`
	CacheWrite      *uint64 `json:"cache_write"`
	CacheRead       *uint64 `json:"cache_read"`
	Output          *uint64 `json:"output"`
	Uncertainty     string  `json:"uncertainty"`
}
