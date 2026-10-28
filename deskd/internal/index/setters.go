package index

import (
	"fmt"
	"github.com/medici-finance/assay/deskd/internal/humanid"
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
)

// This file is the read/ack setters (decision-model spec §1.1.2, ruling D7):
// the FIRST deskd state not rebuildable from GitHub. They write the
// local_attention table the read surface created and NOTHING ELSE — no GitHub write of
// any kind lands this brief. The write-through to a `operator` 👀 reaction is
// the answer flow's, where the authenticated write path actually exists.
//
// The setters live on internal/index, which holds no GitHub client and imports
// no transport, so "no platform write" is a property of what this code can
// reach, not a promise in a comment. The one table they touch is separate from
// the derived cache, so a `rebuild-derived` drops the cache and leaves these
// rows in place — the read/ack a human left survives an index rebuild.

// attentionColumn is the fixed allowlist of columns a setter may write. It is
// a typed constant, never caller input, so the column name can never be an
// injection vector even though it is interpolated into the statement.
type attentionColumn string

const (
	colRead attentionColumn = "read_at"
	colAck  attentionColumn = "acked_at"
)

// DefaultActor() is the human identity a setter attributes to when a caller
// names none. Read/Acked record HUMAN attention on the human-authed surface
// (spec §1.1.2); the Console answers and acks as `operator` (§6.4).
func DefaultActor() string { return humanid.Login() }

// SetRead records that actor first rendered the escalation (spec §1.1.2).
// It is per-audience: the row is keyed on (escalation, actor), so a passer-by
// from another team does not spend the owning team's one moment of noticing.
// Setting read again is idempotent — the FIRST read is what TTFL measures, so
// an existing read_at is left untouched.
func (d *DB) SetRead(id domain.EscalationID, actor string, at time.Time) error {
	return d.setAttentionFirst(id, actor, colRead, at)
}

// SetAck records an explicit human "seen, parked" (spec §1.1.2), distinct from
// read and from answered. Like read, the first ack is the one that counts, so
// it does not overwrite an existing acked_at.
func (d *DB) SetAck(id domain.EscalationID, actor string, at time.Time) error {
	return d.setAttentionFirst(id, actor, colAck, at)
}

// setAttentionFirst upserts the (escalation, actor) row and sets the column to
// `at` only if it is not already set — first-write-wins, because both TTFL and
// the ack timestamp measure the FIRST time, never the latest.
func (d *DB) setAttentionFirst(id domain.EscalationID, actor string, col attentionColumn, at time.Time) error {
	if !humanid.Enabled() {
		return humanid.ErrDisabled
	}
	if id == "" {
		return fmt.Errorf("setter refused: empty escalation id")
	}
	if actor == "" {
		actor = DefaultActor()
	}
	ts := at.UTC().Format(time.RFC3339Nano)
	// One statement, one table (local_attention), no derived table reachable.
	// COALESCE keeps the first write: a re-ack does not reset the clock.
	stmt := fmt.Sprintf(`INSERT INTO local_attention (escalation_id, actor, %[1]s)
		VALUES (?,?,?)
		ON CONFLICT(escalation_id, actor) DO UPDATE SET %[1]s = COALESCE(local_attention.%[1]s, excluded.%[1]s)`, string(col))
	_, err := d.sql.Exec(stmt, string(id), actor, ts)
	return err
}

// Attention returns the (read, acked) timestamps for one escalation, folded
// across actors the same way [DB.Inbox] serves them (earliest of each). It is
// the read side the CLI verbs print after a set, so a human sees the write
// took without a full inbox round-trip.
func (d *DB) Attention(id domain.EscalationID) (read, acked *time.Time, err error) {
	var r, a string
	qerr := d.sql.QueryRow(`SELECT COALESCE(MIN(read_at), ''), COALESCE(MIN(acked_at), '')
		FROM local_attention WHERE escalation_id = ?`, string(id)).Scan(&r, &a)
	if qerr != nil {
		return nil, nil, qerr
	}
	return parseOptTime(r), parseOptTime(a), nil
}

func parseOptTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return nil
	}
	return &t
}
