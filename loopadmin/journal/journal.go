package journal

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
)

// State is an invocation's journalled lifecycle state.
type State string

const (
	Intended  State = "intended"
	Launched  State = "launched"
	Unknown   State = "unknown"
	Completed State = "completed"
	Failed    State = "failed"
	Cancelled State = "cancelled"
)

func (s State) terminal() bool { return s == Completed || s == Failed || s == Cancelled }

// Key identifies one process record: caller namespace, invocation ID and
// owner generation.
type Key struct {
	Namespace  string
	Invocation string
	Generation int64
}

func (k Key) valid() bool { return k.Namespace != "" && k.Invocation != "" && k.Generation >= 0 }

// Intent is a launch intent. RequestID is the caller's stable request ID, the
// reconciliation key with the caller's own queue; Digest identifies the
// request content so that a reused request ID with other content is refused.
type Intent struct {
	Key       Key
	RequestID string
	Profile   string
	Digest    string
}

// Receipt is the journal receipt for a launch intent. The caller keeps it with
// its own record of the request.
type Receipt struct {
	ID  string
	Key Key
}

// Decision is the journal's answer to Intend. Launch is true only when this
// call committed a new intent; the caller starts the child only then.
type Decision struct {
	Receipt Receipt
	State   State
	Launch  bool
}

// Kind is an append-only process fact kind.
type Kind string

const (
	Lifecycle    Kind = "lifecycle"
	Cancellation Kind = "cancellation"
	Limit        Kind = "limit"
	Usage        Kind = "usage"
	Checkpoint   Kind = "checkpoint"
)

var (
	// ErrInvalid refuses a malformed intent, key or fact.
	ErrInvalid = errors.New("loopadmin/journal: invalid record")
	// ErrConflict refuses a request ID or key reused with other content.
	ErrConflict = errors.New("loopadmin/journal: conflicting record")
	// ErrNotFound reports that no record matches.
	ErrNotFound = errors.New("loopadmin/journal: no such record")
)

// Journal is a runtime handle on one cell's journal. It cannot activate or
// alter the activation record except to pause it.
type Journal struct {
	db     *sql.DB
	cell   string
	source IncarnationSource
}

// Open connects to the cell database and checks that loopadmin's schema is
// at this build's version. It never migrates and never falls back to another
// store: an unreachable database or a missing schema refuses.
func Open(ctx context.Context, c Config, cell string, src IncarnationSource) (*Journal, error) {
	if cell == "" || src == nil {
		return nil, failure(nil, "open", InvalidConfig)
	}
	db, err := connect(ctx, c)
	if err != nil {
		return nil, err
	}
	if err = checkSchema(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return &Journal{db: db, cell: cell, source: src}, nil
}

// Close releases the handle.
func (j *Journal) Close() error { return j.db.Close() }

// Gate reports whether the journal admits a launch or a receipt delivery now.
func (j *Journal) Gate(ctx context.Context) error {
	cur, err := current(ctx, j.source, j.cell)
	if err != nil {
		return err
	}
	tx, err := j.db.BeginTx(ctx, nil)
	if err != nil {
		return classify(ctx, "gate", err)
	}
	seen, err := gate(ctx, tx, j.cell, cur)
	tx.Rollback()
	return refuse(ctx, j.db, j.source, j.cell, seen, err)
}

func newReceiptID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "rcpt-" + hex.EncodeToString(b), nil
}

// Intend records a launch intent and its receipt in one transaction, after
// checking the operator incarnation against the activation record.
//
// prior is the receipt the caller already holds for this request, if any. A
// presented receipt that the journal does not know means the journal lost the
// record (a dropped or rebuilt database): the launch may have happened, so the
// invocation is held as unknown and nothing launches until it is reconciled.
// A request ID the journal already holds returns the existing record without
// launching. Only a new intent returns Launch true.
func (j *Journal) Intend(ctx context.Context, in Intent, prior *Receipt) (Decision, error) {
	if !in.Key.valid() || in.RequestID == "" || in.Digest == "" {
		return Decision{}, ErrInvalid
	}
	if prior != nil && (prior.ID == "" || prior.Key != in.Key) {
		return Decision{}, ErrConflict
	}
	cur, err := current(ctx, j.source, j.cell)
	if err != nil {
		return Decision{}, err
	}
	tx, err := j.db.BeginTx(ctx, nil)
	if err != nil {
		return Decision{}, classify(ctx, "intend", err)
	}
	defer tx.Rollback()
	if seen, err := gate(ctx, tx, j.cell, cur); err != nil {
		tx.Rollback()
		return Decision{}, refuse(ctx, j.db, j.source, j.cell, seen, err)
	}
	// Serialize intents for one request so that a concurrent duplicate waits
	// for this transaction instead of racing it.
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1 || '/' || $2, 0))", in.Key.Namespace, in.RequestID); err != nil {
		return Decision{}, classify(ctx, "intend", err)
	}
	existing, found, err := lookup(ctx, tx, in)
	if err != nil {
		return Decision{}, err
	}
	if found {
		if prior != nil && existing.Receipt.ID != "" && existing.Receipt.ID != prior.ID {
			return Decision{}, ErrConflict
		}
		if existing.Receipt.ID == "" && prior != nil {
			existing.Receipt = *prior
		}
		return existing, classify(ctx, "intend", tx.Commit())
	}
	if prior != nil {
		// The caller holds a receipt the journal has no record of.
		if _, err = tx.ExecContext(ctx, `INSERT INTO loopadmin.invocation
			(caller_namespace, invocation_id, owner_generation, request_id, cell_id, profile, intent_digest, incarnation_id, state)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'unknown')`,
			in.Key.Namespace, in.Key.Invocation, in.Key.Generation, in.RequestID, j.cell, in.Profile, in.Digest, cur.ID); err != nil {
			return Decision{}, classify(ctx, "intend", err)
		}
		if err = tx.Commit(); err != nil {
			return Decision{}, classify(ctx, "intend", err)
		}
		return Decision{Receipt: *prior, State: Unknown}, nil
	}
	id, err := newReceiptID()
	if err != nil {
		return Decision{}, failure(ctx, "intend", StoreFailed)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO loopadmin.invocation
		(caller_namespace, invocation_id, owner_generation, request_id, cell_id, profile, intent_digest, incarnation_id, state)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'intended')`,
		in.Key.Namespace, in.Key.Invocation, in.Key.Generation, in.RequestID, j.cell, in.Profile, in.Digest, cur.ID); err != nil {
		return Decision{}, classify(ctx, "intend", err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO loopadmin.receipt (receipt_id, caller_namespace, invocation_id, owner_generation)
		VALUES ($1, $2, $3, $4)`, id, in.Key.Namespace, in.Key.Invocation, in.Key.Generation); err != nil {
		return Decision{}, classify(ctx, "intend", err)
	}
	if err = tx.Commit(); err != nil {
		return Decision{}, classify(ctx, "intend", err)
	}
	return Decision{Receipt: Receipt{ID: id, Key: in.Key}, State: Intended, Launch: true}, nil
}

// lookup finds an existing record by request ID or key. A record matching one
// but not the other, or with other content, is a conflict.
func lookup(ctx context.Context, tx *sql.Tx, in Intent) (Decision, bool, error) {
	var k Key
	var request, digest, state string
	var receipt sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT i.caller_namespace, i.invocation_id, i.owner_generation, i.request_id, i.intent_digest, i.state, r.receipt_id
		FROM loopadmin.invocation i
		LEFT JOIN loopadmin.receipt r USING (caller_namespace, invocation_id, owner_generation)
		WHERE i.caller_namespace = $1 AND (i.request_id = $2 OR (i.invocation_id = $3 AND i.owner_generation = $4))
		ORDER BY i.created_at LIMIT 1`,
		in.Key.Namespace, in.RequestID, in.Key.Invocation, in.Key.Generation).
		Scan(&k.Namespace, &k.Invocation, &k.Generation, &request, &digest, &state, &receipt)
	if errors.Is(err, sql.ErrNoRows) {
		return Decision{}, false, nil
	}
	if err != nil {
		return Decision{}, false, classify(ctx, "intend", err)
	}
	if k != in.Key || request != in.RequestID || digest != in.Digest {
		return Decision{}, false, ErrConflict
	}
	return Decision{Receipt: Receipt{ID: receipt.String, Key: k}, State: State(state)}, true, nil
}

// MarkLaunched records that the child for an intended invocation started.
func (j *Journal) MarkLaunched(ctx context.Context, k Key) error {
	return j.transition(ctx, k, Launched, Intended)
}

// Reconcile settles an open invocation (intended, launched or held unknown)
// with an observed terminal outcome. It reports whether this call settled it;
// a second call for the same invocation settles nothing.
func (j *Journal) Reconcile(ctx context.Context, k Key, outcome State) (bool, error) {
	if !outcome.terminal() {
		return false, ErrInvalid
	}
	err := j.transition(ctx, k, outcome, Intended, Launched, Unknown)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (j *Journal) transition(ctx context.Context, k Key, to State, from ...State) error {
	if !k.valid() {
		return ErrInvalid
	}
	states := make([]string, len(from))
	for i, s := range from {
		states[i] = string(s)
	}
	res, err := j.db.ExecContext(ctx, `UPDATE loopadmin.invocation SET state = $1, updated_at = now()
		WHERE caller_namespace = $2 AND invocation_id = $3 AND owner_generation = $4 AND cell_id = $5 AND state = ANY($6)`,
		string(to), k.Namespace, k.Invocation, k.Generation, j.cell, states)
	if err != nil {
		return classify(ctx, "transition", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return classify(ctx, "transition", err)
	}
	if n != 1 {
		return ErrNotFound
	}
	return nil
}

// Record is one open invocation as Pending returns it.
type Record struct {
	Key       Key
	RequestID string
	State     State
}

// Pending lists the cell's open invocations: intended, launched and held
// unknown. A restarted supervisor reconciles each of them before launching.
func (j *Journal) Pending(ctx context.Context) ([]Record, error) {
	rows, err := j.db.QueryContext(ctx, `SELECT caller_namespace, invocation_id, owner_generation, request_id, state
		FROM loopadmin.invocation WHERE cell_id = $1 AND state IN ('intended', 'launched', 'unknown')
		ORDER BY created_at, caller_namespace, invocation_id, owner_generation`, j.cell)
	if err != nil {
		return nil, classify(ctx, "pending", err)
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		var r Record
		var state string
		if err = rows.Scan(&r.Key.Namespace, &r.Key.Invocation, &r.Key.Generation, &r.RequestID, &state); err != nil {
			return nil, classify(ctx, "pending", err)
		}
		r.State = State(state)
		out = append(out, r)
	}
	return out, classify(ctx, "pending", rows.Err())
}

// Append records one process fact for an invocation.
func (j *Journal) Append(ctx context.Context, k Key, kind Kind, detail json.RawMessage) error {
	switch kind {
	case Lifecycle, Cancellation, Limit, Usage, Checkpoint:
	default:
		return ErrInvalid
	}
	if !k.valid() || !json.Valid(detail) {
		return ErrInvalid
	}
	res, err := j.db.ExecContext(ctx, `INSERT INTO loopadmin.event (caller_namespace, invocation_id, owner_generation, kind, detail)
		SELECT caller_namespace, invocation_id, owner_generation, $4, $5::jsonb FROM loopadmin.invocation
		WHERE caller_namespace = $1 AND invocation_id = $2 AND owner_generation = $3 AND cell_id = $6`,
		k.Namespace, k.Invocation, k.Generation, string(kind), string(detail), j.cell)
	if err != nil {
		return classify(ctx, "append", err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		return ErrNotFound
	}
	return nil
}
