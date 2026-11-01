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

// Control-volume roles. Only the operator holds signing custody.
const (
	ControlOperator = "operator"
	ControlExecutor = "executor"
)

// ControlGrant is a passive record of one identity's access to the protected
// operator-control volume. It names references only and creates no resource.
// Ruled restriction on custody 1A: a role executor that mounts the volume never
// reads the signing material on it, and only the operator writes control state.
type ControlGrant struct {
	Identity    string `json:"identity"`
	Role        string `json:"role"`
	VolumeRef   string `json:"volume_ref"`
	ReadSigning bool   `json:"read_signing"`
	Write       bool   `json:"write"`
}

// CheckControlGrant refuses a grant that breaks the custody restriction. Nil is
// record consistency only; the qualified profile's mount and file-access layer
// still owes enforcement, and this check installs none.
func CheckControlGrant(g ControlGrant) error {
	if g.Identity == "" || g.VolumeRef == "" {
		return errors.New("control grant incomplete")
	}
	switch g.Role {
	case ControlOperator:
		return nil
	case ControlExecutor:
		if g.ReadSigning {
			return errors.New("executor may not read signing material")
		}
		if g.Write {
			return errors.New("executor may not write control state")
		}
		return nil
	}
	return errors.New("control grant role unknown")
}

// RestoredControl is provenance for operator-control state recovered from a
// snapshot or backup of the protected volume. Ruled restriction on custody 1A:
// such a restore never presents operator-control state as current.
type RestoredControl struct {
	SnapshotRef   string `json:"snapshot_ref"`
	IncarnationID string `json:"incarnation_id"`
	KeyRef        string `json:"key_ref"`
}

// CheckNotRestored refuses a current incarnation that reuses the identity or the
// key reference of restored control state: the operator must rotate both before
// a match can count. Incomplete provenance cannot be compared and is refused.
// It complements MatchCellFence; neither alone authorizes an effect.
func CheckNotRestored(now OperatorIncarnation, restored RestoredControl) error {
	if restored.SnapshotRef == "" || restored.IncarnationID == "" || restored.KeyRef == "" {
		return errors.New("restore provenance incomplete")
	}
	if now.ID == restored.IncarnationID || now.KeyRef == restored.KeyRef {
		return errors.New("restored control state presented as current")
	}
	return nil
}
