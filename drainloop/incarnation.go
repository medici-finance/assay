package drainloop

import (
	"context"
	"errors"
)

// IncarnationSchema identifies the offline operator-incarnation record contract.
const IncarnationSchema = "operator-incarnation/v1"

// OperatorIncarnation contains references only, never signing-key material.
// The current value must come from protected operator state outside the runtime
// database. These records do not mint keys, authenticate a caller or grant effects.
type OperatorIncarnation struct {
	Schema string `json:"schema"`
	CellID string `json:"cell_id"`
	ID     string `json:"id"`
	KeyRef string `json:"key_ref"`
}

// IncarnationSource reads current protected operator state. A database snapshot,
// model response or echoed capability is not an implementation of this trust port.
// Consumers must hold on errors; this preparation installs no source.
type IncarnationSource interface {
	CurrentIncarnation(context.Context, string) (OperatorIncarnation, error)
}

// CellActivation is the runtime store's binding to independently selected operator
// state. StopReceipt refers to operator-confirmed execution-group teardown;
// recovered attempt rows are not confirmation. Paused is retained across restore.
type CellActivation struct {
	IncarnationID string `json:"incarnation_id"`
	KeyRef        string `json:"key_ref"`
	StopReceipt   string `json:"stop_receipt"`
	Paused        bool   `json:"paused"`
}

// MatchCellFence checks record consistency only. Even nil does not authenticate
// a capability, establish durable/current ownership or authorize an effect.
// Callers still owe those checks at the separately enforcing effect boundary.
func MatchCellFence(current OperatorIncarnation, active CellActivation, owned, presented CellOwnership) error {
	if current.Schema != IncarnationSchema || current.CellID == "" ||
		current.ID == "" || current.KeyRef == "" {
		return errors.New("operator incarnation unavailable")
	}
	if active.Paused || active.StopReceipt == "" ||
		active.IncarnationID != current.ID || active.KeyRef != current.KeyRef {
		return errors.New("activation paused or mismatched")
	}
	if owned.CellID != current.CellID || owned.IncarnationID != current.ID ||
		owned.OwnerID == "" || owned.Generation == "" || owned.Backend == "" {
		return errors.New("ownership fence mismatched")
	}
	if presented != owned {
		return errors.New("capability owner or generation mismatched")
	}
	return nil
}
