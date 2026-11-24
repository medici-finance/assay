package journal

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"time"
)

// admissionLockID is the one advisory lock every admission takes, so that
// reading a budget, counting recorded launches and recording the new launch
// happen as one step across every caller and process.
const admissionLockID = 0x6c6f6f7061646d69

var (
	// ErrNoBudget refuses a launch for a profile without a recorded budget.
	ErrNoBudget = errors.New("loopadmin/journal: launch refused: the profile has no budget")
	// ErrBudgetSpent refuses a launch that would exceed the profile's window
	// budget.
	ErrBudgetSpent = errors.New("loopadmin/journal: launch refused: the profile's window budget is spent")
	// ErrUsageUnknown refuses a launch while the profile's consumption cannot
	// be read or verified: the ledger is unreadable, or an invocation of the
	// profile is held unknown.
	ErrUsageUnknown = errors.New("loopadmin/journal: launch refused: launch usage is unknown or unreadable")
)

// Budget is a profile's launch budget: at most Max recorded launches in any
// window of length Window.
type Budget struct {
	Profile string
	Window  time.Duration
	Max     int64
}

// ProfileLimit is one admitted, immutable limit of a profile.
type ProfileLimit struct {
	Profile string
	Name    string
	Value   int64
}

// UsageReport is one usage report for an invocation. Complete is false while the
// report may still change.
type UsageReport struct {
	Complete bool
	Launches int64
}

// Ref is a checkpoint or referenced artifact: an opaque reference and its
// sha256 digest, written "sha256:<64 hex>".
type Ref struct {
	Ref    string
	Digest string
}

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func (r Ref) valid() bool { return r.Ref != "" && digestPattern.MatchString(r.Digest) }

// admit is the single locked admission. Inside tx, under the admission lock,
// it reads the profile's budget and the launches recorded in its window,
// refuses on a missing budget, an unreadable count or an invocation of the
// profile held unknown, and records the new launch in the ledger. The caller
// commits tx; nothing is admitted unless it does. The returned time is a
// pacing estimate derived from the recorded launches: the caller may delay
// the launch until then, and a zero time means no estimate is available,
// which never refuses on its own.
func admit(ctx context.Context, tx *sql.Tx, cell, incarnation string, in Intent) (time.Time, error) {
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", int64(admissionLockID)); err != nil {
		return time.Time{}, classify(ctx, "admit", err)
	}
	var window, max int64
	err := tx.QueryRowContext(ctx, "SELECT window_seconds, max_launches FROM loopadmin.profile_budget WHERE profile = $1", in.Profile).Scan(&window, &max)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, ErrNoBudget
	}
	if err != nil {
		return time.Time{}, errors.Join(ErrUsageUnknown, classify(ctx, "admit", err))
	}
	var used, unknown int64
	var last sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT
			(SELECT count(*) FROM loopadmin.launch WHERE profile = $1 AND launched_at > now() - ($2::bigint * interval '1 second')),
			(SELECT max(launched_at) FROM loopadmin.launch WHERE profile = $1),
			(SELECT count(*) FROM loopadmin.invocation WHERE profile = $1 AND state = 'unknown')`,
		in.Profile, window).Scan(&used, &last, &unknown)
	if err != nil {
		return time.Time{}, errors.Join(ErrUsageUnknown, classify(ctx, "admit", err))
	}
	if unknown > 0 {
		return time.Time{}, ErrUsageUnknown
	}
	if used >= max {
		return time.Time{}, ErrBudgetSpent
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO loopadmin.launch
		(profile, caller_namespace, invocation_id, owner_generation, cell_id, incarnation_id)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		in.Profile, in.Key.Namespace, in.Key.Invocation, in.Key.Generation, cell, incarnation); err != nil {
		return time.Time{}, classify(ctx, "admit", err)
	}
	var pace time.Time
	if last.Valid && max > 0 {
		pace = last.Time.Add(time.Duration(window) * time.Second / time.Duration(max))
	}
	return pace, nil
}

// Limits reads the profile's admitted limits.
func (j *Journal) Limits(ctx context.Context, profile string) ([]ProfileLimit, error) {
	rows, err := j.db.QueryContext(ctx, "SELECT name, value FROM loopadmin.profile_limit WHERE profile = $1 ORDER BY name", profile)
	if err != nil {
		return nil, classify(ctx, "limits", err)
	}
	defer func() { _ = rows.Close() }() // Rows.Err reports iteration and close errors.
	var out []ProfileLimit
	for rows.Next() {
		l := ProfileLimit{Profile: profile}
		if err = rows.Scan(&l.Name, &l.Value); err != nil {
			return nil, classify(ctx, "limits", err)
		}
		out = append(out, l)
	}
	return out, classify(ctx, "limits", rows.Err())
}

// RecordUsage records a usage report for an invocation of this cell.
func (j *Journal) RecordUsage(ctx context.Context, k Key, u UsageReport) error {
	if !k.valid() || u.Launches < 0 {
		return ErrInvalid
	}
	return j.insertFor(ctx, "usage", k, `INSERT INTO loopadmin.usage_report (caller_namespace, invocation_id, owner_generation, complete, launches)
		SELECT caller_namespace, invocation_id, owner_generation, $5, $6 FROM loopadmin.invocation
		WHERE caller_namespace = $1 AND invocation_id = $2 AND owner_generation = $3 AND cell_id = $4`, u.Complete, u.Launches)
}

// RecordCheckpoint records a checkpoint of an invocation of this cell.
func (j *Journal) RecordCheckpoint(ctx context.Context, k Key, r Ref) error {
	return j.recordRef(ctx, "checkpoint", k, r)
}

// RecordArtifact records an artifact an invocation of this cell references.
// A backup's inventory lists these, and a restore verifies each one.
func (j *Journal) RecordArtifact(ctx context.Context, k Key, r Ref) error {
	return j.recordRef(ctx, "artifact", k, r)
}

func (j *Journal) recordRef(ctx context.Context, table string, k Key, r Ref) error {
	if !k.valid() || !r.valid() {
		return ErrInvalid
	}
	return j.insertFor(ctx, table, k, `INSERT INTO loopadmin.`+table+` (caller_namespace, invocation_id, owner_generation, ref, digest)
		SELECT caller_namespace, invocation_id, owner_generation, $5, $6 FROM loopadmin.invocation
		WHERE caller_namespace = $1 AND invocation_id = $2 AND owner_generation = $3 AND cell_id = $4`, r.Ref, r.Digest)
}

func (j *Journal) insertFor(ctx context.Context, op string, k Key, stmt string, args ...any) error {
	res, err := j.db.ExecContext(ctx, stmt, append([]any{k.Namespace, k.Invocation, k.Generation, j.cell}, args...)...)
	if err != nil {
		return classify(ctx, op, err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		return ErrNotFound
	}
	return nil
}

// SetBudget is an operator act under the migration owner or another operator
// role, never reachable from a runtime Journal handle: it records a profile's
// launch budget. The runtime role may only read it.
func SetBudget(ctx context.Context, c Config, b Budget) error {
	if b.Profile == "" || b.Window < time.Second || b.Window%time.Second != 0 || b.Max < 0 {
		return ErrInvalid
	}
	db, err := connect(ctx, c)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }() // Preserve the statement result.
	if err = checkSchema(ctx, db); err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `INSERT INTO loopadmin.profile_budget (profile, window_seconds, max_launches)
		VALUES ($1, $2, $3)
		ON CONFLICT (profile) DO UPDATE SET window_seconds = EXCLUDED.window_seconds,
			max_launches = EXCLUDED.max_launches, recorded_at = now()`,
		b.Profile, int64(b.Window/time.Second), b.Max)
	return classify(ctx, "budget", err)
}

// AdmitLimit is an operator act: it records one immutable limit of a profile.
// An admitted limit is never changed; admitting another value for the same
// name is a conflict.
func AdmitLimit(ctx context.Context, c Config, l ProfileLimit) error {
	if l.Profile == "" || l.Name == "" || l.Value < 0 {
		return ErrInvalid
	}
	db, err := connect(ctx, c)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }() // Preserve the statement result.
	if err = checkSchema(ctx, db); err != nil {
		return err
	}
	var value int64
	err = db.QueryRowContext(ctx, `INSERT INTO loopadmin.profile_limit (profile, name, value) VALUES ($1, $2, $3)
		ON CONFLICT (profile, name) DO UPDATE SET profile = EXCLUDED.profile
		RETURNING value`, l.Profile, l.Name, l.Value).Scan(&value)
	if err != nil {
		return classify(ctx, "limit", err)
	}
	if value != l.Value {
		return ErrConflict
	}
	return nil
}
