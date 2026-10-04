package runner

// Usage is token and cost accounting for one invocation. A nil field is
// UNKNOWN, not zero: an adapter without telemetry, or a provider that did not
// report a figure, leaves it nil and it stays nil through every sum.
type Usage struct {
	InputTokens  *int64 `json:"input_tokens,omitempty"`
	OutputTokens *int64 `json:"output_tokens,omitempty"`
	CostMicros   *int64 `json:"cost_micros,omitempty"`
}

// Int64 returns a pointer to v, for building a measured figure (0 is a real
// reading).
func Int64(v int64) *int64 { return &v }

// Complete reports whether every figure is known.
func (u Usage) Complete() bool {
	return u.InputTokens != nil && u.OutputTokens != nil && u.CostMicros != nil
}

// Add sums two usages. A figure unknown on either side is unknown in the sum.
func (u Usage) Add(o Usage) Usage {
	return Usage{
		InputTokens:  addKnown(u.InputTokens, o.InputTokens),
		OutputTokens: addKnown(u.OutputTokens, o.OutputTokens),
		CostMicros:   addKnown(u.CostMicros, o.CostMicros),
	}
}

// readings drops a negative figure: it is not a measurement, so it is unknown.
func (u Usage) readings() Usage {
	keep := func(p *int64) *int64 {
		if p == nil || *p < 0 {
			return nil
		}
		return p
	}
	return Usage{InputTokens: keep(u.InputTokens), OutputTokens: keep(u.OutputTokens), CostMicros: keep(u.CostMicros)}
}

func addKnown(a, b *int64) *int64 {
	if a == nil || b == nil {
		return nil
	}
	return Int64(*a + *b)
}

// Capabilities is what an adapter declares it can do. The zero value declares
// nothing, so every optional facility is opt-in.
type Capabilities struct {
	// Resume: can continue a pinned session.
	Resume bool `json:"resume"`
	// CancelAck: Cancel's acknowledgment is meaningful. Even so, only an
	// observation of a stopped invocation confirms the stop.
	CancelAck bool `json:"cancel_ack"`
	// BudgetScopes: the budget scopes the adapter can enforce.
	BudgetScopes []BudgetScope `json:"budget_scopes"`
	// Telemetry: how much of Usage the adapter reports.
	Telemetry Telemetry `json:"telemetry"`
	// Snapshot: can expose a read-only snapshot of a session.
	Snapshot bool `json:"snapshot"`
	// LaunchDedupe: a Start repeated with the same identity is deduplicated by
	// the adapter, so an absent record after reconcile proves no launch.
	LaunchDedupe bool `json:"launch_dedupe"`
	// Extensions: optional request extensions the adapter understands.
	Extensions []string `json:"extensions,omitempty"`
}

// Telemetry says how much usage an adapter reports.
type Telemetry string

const (
	TelemetryNone     Telemetry = "none"
	TelemetryPartial  Telemetry = "partial"
	TelemetryComplete Telemetry = "complete"
)

// Has reports whether the capability is declared.
func (c Capabilities) Has(cap Capability) bool {
	switch cap {
	case CapResume:
		return c.Resume
	case CapCancelAck:
		return c.CancelAck
	case CapBudgetLaunch:
		return c.supportsBudget(BudgetLaunch)
	case CapBudgetRequest:
		return c.supportsBudget(BudgetRequest)
	case CapTelemetry:
		return c.Telemetry == TelemetryPartial || c.Telemetry == TelemetryComplete
	case CapSnapshot:
		return c.Snapshot
	case CapLaunchDedupe:
		return c.LaunchDedupe
	}
	return false
}

func (c Capabilities) supportsBudget(s BudgetScope) bool {
	for _, have := range c.BudgetScopes {
		if have == s {
			return true
		}
	}
	return false
}

func (c Capabilities) hasExtension(name string) bool {
	for _, have := range c.Extensions {
		if have == name {
			return true
		}
	}
	return false
}
