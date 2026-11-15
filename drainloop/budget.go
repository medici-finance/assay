package drainloop

import (
	"errors"
	"math"
	"time"
)

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

func reserveBudget(b *CellBudget, r CellAdmission, now time.Time) error {
	if len(r.Reservations) == 0 || now.IsZero() {
		return errors.New("budget reservation or time unavailable")
	}
	seen := map[string]bool{}
	for _, q := range r.Reservations {
		if q.ID == "" || seen[q.ID] || q.Amount == 0 || q.WorkID != r.Permit.WorkID || q.AttemptID != r.Permit.AttemptID || q.RequestID != r.Permit.RequestID {
			return errors.New("reservation identity invalid")
		}
		seen[q.ID] = true
		for _, old := range b.Reservations {
			if old.ID == q.ID {
				return errors.New("reservation recorded")
			}
		}
		var found *BudgetBalance
		for i := range b.Balances {
			if b.Balances[i].Ref == q.BalanceRef {
				if found != nil {
					return errors.New("ambiguous balance")
				}
				found = &b.Balances[i]
			}
		}
		if found == nil || found.Consumed == nil {
			return errors.New("brake usage unavailable")
		}
		if err := balanceWindow(*found, now); err != nil {
			return err
		}
		used := *found.Consumed
		add := func(v uint64) bool {
			if v > math.MaxUint64-used {
				return false
			}
			used += v
			return true
		}
		if !add(found.Headroom) {
			return errors.New("budget overflow")
		}
		for _, old := range b.Reservations {
			if old.BalanceRef == q.BalanceRef {
				if !add(old.Amount) {
					return errors.New("reservation overflow")
				}
			}
		}
		if !add(q.Amount) || used > found.Limit {
			return errors.New("budget exhausted")
		}
		b.Reservations = append(b.Reservations, q)
	}
	return nil
}

// CellPacing is a shared window observation, not a hard budget or per-process
// counter. Effect counts re-derive from ALL retained intents/receipts every time;
// Cached is optional and never a source. Seed is the retained initial estimate.
// Epoch 1 denotes the ONE independently recorded reset after loss. ResetKnown
// must come from a qualified anchor outside the lost store, not restored SQL.
// Configuration owns Cap/C, including qualification of bounded concurrency C.
// C is qualification/reporting metadata. This conservative implementation admits
// below Cap itself, so it is tighter than the allowed Cap+C bound; C grants no slot.
type CellPacing struct {
	Scope, Window, Incarnation, Class string
	Cap, C                            uint64
	HistoryKnown                      bool
	Seed                              *uint64
	Cached                            uint64
	Epoch                             uint64
	ResetKnown                        bool
}

func cellPacing(s CellState, scope, window, incarnation string, epoch uint64) error {
	var p *CellPacing
	for i := range s.Pacing {
		q := &s.Pacing[i]
		if q.Scope == scope && q.Window == window && q.Incarnation == incarnation {
			if p != nil {
				return errors.New("ambiguous pacing")
			}
			p = q
		}
	}
	if p == nil || !p.HistoryKnown || p.Seed == nil || !p.ResetKnown || p.Epoch > 1 || p.Epoch != epoch {
		return errors.New("pacing unavailable/exhausted")
	}
	n := *p.Seed
	switch p.Class {
	case "effect":
		keys := map[string]bool{}
		for _, r := range append(append([]CellAdmission{}, s.Pending...), s.Completed...) {
			if r.PacingScope == scope && r.PacingWindow == window && r.Permit.Ownership.IncarnationID == incarnation && !keys[r.Permit.Key] {
				if n == math.MaxUint64 {
					return errors.New("pacing overflow")
				}
				n++
				keys[r.Permit.Key] = true
			}
		}
	case "read": // its adapter updates the retained shared seed after each read.
	default:
		return errors.New("unknown pacing class")
	}
	if n >= p.Cap {
		return errors.New("pacing exhausted")
	}
	return nil
}

// balanceWindow uses composition's authority clock, not the intent provenance.
func balanceWindow(b BudgetBalance, now time.Time) error {
	start, e1 := time.Parse(time.RFC3339, b.WindowStart)
	end, e2 := time.Parse(time.RFC3339, b.WindowEnd)
	if now.IsZero() || e1 != nil || e2 != nil || !end.After(start) || now.Before(start) || !now.Before(end) || b.Account == "" || b.Provider == "" || b.Unit == "" {
		return errors.New("balance scope/window unavailable")
	}
	return nil
}
