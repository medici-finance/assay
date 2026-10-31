package drainloop

// CellOwnership is a generation inside one operator incarnation. Generation is
// opaque, not an increasing integer whose value can survive rollback as authority.
type CellOwnership struct {
	CellID        string `json:"cell_id"`
	IncarnationID string `json:"incarnation_id"`
	OwnerID       string `json:"owner_id"`
	Generation    string `json:"generation"`
	Backend       string `json:"backend"`
}

// CellCheckpoint is a passive persistence contract, not a second lease store.
// Outstanding effects, cancellation, stop and overrides must survive ordinary
// restart and restore. Missing history is retained as unknown, never new allowance.
// A consumer must not use Activation to select the current operator incarnation.
type CellCheckpoint struct {
	Schema          string         `json:"schema"`
	Ownership       CellOwnership  `json:"ownership"`
	Activation      CellActivation `json:"activation"`
	Budget          CellBudget     `json:"budget"`
	CancelRequested bool           `json:"cancel_requested"`
	Stopped         bool           `json:"stopped"`
	OverrideRefs    []string       `json:"override_refs"`
	PendingEffects  []string       `json:"pending_effects"`
}
