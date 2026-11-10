package journal

import (
	"context"
	"database/sql"
	"errors"
)

// IncarnationSchema is the only operator-incarnation schema this journal
// accepts (spec/operator-incarnation-v1.md).
const IncarnationSchema = "operator-incarnation/v1"

// Incarnation is the protected current operator incarnation for a cell. It
// conforms to spec/operator-incarnation-v1.md's record fields; loopadmin does
// not import the drainloop module that carries the reference type.
type Incarnation struct {
	Schema string `json:"schema"`
	CellID string `json:"cell_id"`
	ID     string `json:"id"`
	KeyRef string `json:"key_ref"`
}

// IncarnationSource reads the current incarnation from trusted operator state
// outside PostgreSQL. An error or an empty record holds admission.
type IncarnationSource interface {
	CurrentIncarnation(ctx context.Context, cellID string) (Incarnation, error)
}

// StopConfirmation is the operator's confirmation that the cell's execution
// group stopped under the previous incarnation. Receipt is an authorized,
// opaque receipt reference; it is never a secret.
type StopConfirmation struct {
	Receipt string
}

var (
	// ErrPaused refuses a launch or a receipt delivery: the activation record
	// is missing, paused, or names another incarnation than the current one.
	ErrPaused = errors.New("loopadmin/journal: paused: operator incarnation does not match the activation record")
	// ErrRotationRequired refuses an activation that reuses the identity or
	// key reference of the record it replaces.
	ErrRotationRequired = errors.New("loopadmin/journal: activation refused: rotate the incarnation identity and key reference")
	// ErrStopUnconfirmed refuses an activation without a stop confirmation.
	ErrStopUnconfirmed = errors.New("loopadmin/journal: activation refused: execution group stop is not confirmed")
)

func usable(cell string, cur Incarnation) bool {
	return cur.Schema == IncarnationSchema && cur.CellID == cell && cur.ID != "" && cur.KeyRef != ""
}

func current(ctx context.Context, src IncarnationSource, cell string) (Incarnation, error) {
	if src == nil || cell == "" {
		return Incarnation{}, ErrPaused
	}
	cur, err := src.CurrentIncarnation(ctx, cell)
	if err != nil || !usable(cell, cur) {
		return Incarnation{}, ErrPaused
	}
	return cur, nil
}

// gate checks, inside tx, that the activation record names the current
// incarnation, is not paused and carries a stop confirmation. A present
// record naming another incarnation is marked paused so that the journal stays
// paused until the operator activates again.
func gate(ctx context.Context, tx *sql.Tx, cell string, cur Incarnation) (mismatch bool, err error) {
	var id, key, receipt string
	var paused bool
	err = tx.QueryRowContext(ctx, "SELECT incarnation_id, key_ref, paused, stop_receipt FROM loopadmin.activation WHERE cell_id = $1 FOR SHARE", cell).Scan(&id, &key, &paused, &receipt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrPaused
	}
	if err != nil {
		return false, classify(ctx, "gate", err)
	}
	if id != cur.ID || key != cur.KeyRef {
		return true, ErrPaused
	}
	if paused || receipt == "" {
		return false, ErrPaused
	}
	return false, nil
}

// markPaused durably pauses a mismatched activation record.
func markPaused(ctx context.Context, db *sql.DB, cell string) error {
	_, err := db.ExecContext(ctx, "UPDATE loopadmin.activation SET paused = true, recorded_at = now() WHERE cell_id = $1", cell)
	return classify(ctx, "gate", err)
}

// Activate is the operator path, never reachable from a runtime Journal
// handle: it records the current incarnation, read from src, as the cell's
// activation together with a confirmed stop of the execution group. A record
// it replaces must have been rotated away from in both identity and key
// reference; a restored record is never re-activated as it stands.
func Activate(ctx context.Context, c Config, cell string, src IncarnationSource, stop StopConfirmation) error {
	if stop.Receipt == "" {
		return ErrStopUnconfirmed
	}
	cur, err := current(ctx, src, cell)
	if err != nil {
		return err
	}
	db, err := connect(ctx, c)
	if err != nil {
		return err
	}
	defer db.Close()
	if err = checkSchema(ctx, db); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return classify(ctx, "activate", err)
	}
	defer tx.Rollback()
	var id, key string
	err = tx.QueryRowContext(ctx, "SELECT incarnation_id, key_ref FROM loopadmin.activation WHERE cell_id = $1 FOR UPDATE", cell).Scan(&id, &key)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return classify(ctx, "activate", err)
	case id == cur.ID || key == cur.KeyRef:
		return ErrRotationRequired
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO loopadmin.activation (cell_id, incarnation_id, key_ref, paused, stop_receipt)
		VALUES ($1, $2, $3, false, $4)
		ON CONFLICT (cell_id) DO UPDATE SET incarnation_id = EXCLUDED.incarnation_id, key_ref = EXCLUDED.key_ref,
			paused = false, stop_receipt = EXCLUDED.stop_receipt, recorded_at = now()`,
		cell, cur.ID, cur.KeyRef, stop.Receipt); err != nil {
		return classify(ctx, "activate", err)
	}
	return classify(ctx, "activate", tx.Commit())
}
