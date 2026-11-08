package store

import (
	"context"
	"database/sql"
	"errors"
	"slices"

	"github.com/pressly/goose/v3/database"
)

// ledger is a narrow Goose Store adapter for one PostgreSQL table. It adds the
// R2 checksum and forward-only policy; it does not schedule or run migrations.
type ledger struct {
	manifest  map[int64]string
	bootstrap string
}

var _ database.StoreExtender = (*ledger)(nil)

func (*ledger) TableExists(ctx context.Context, db database.DBTxConn) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, "SELECT to_regclass('public.coordination_schema') IS NOT NULL").Scan(&exists)
	if err == nil && exists {
		err = validateLedgerShape(ctx, db)
	}
	return exists, err
}

func (*ledger) Tablename() string { return "public.coordination_schema" }
func (l *ledger) CreateVersionTable(ctx context.Context, db database.DBTxConn) error {
	if l.bootstrap == "" {
		return errors.New("missing ledger schema")
	}
	_, err := db.ExecContext(ctx, l.bootstrap)
	return err
}
func (l *ledger) Insert(ctx context.Context, db database.DBTxConn, req database.InsertRequest) error {
	sum, ok := l.manifest[req.Version]
	if !ok {
		return errors.New("unknown migration version")
	}
	// Goose's SQL DDL and its version acknowledgment must share one transaction.
	if _, ok := db.(*sql.Tx); !ok {
		return errors.New("migration acknowledgment requires transaction")
	}
	_, err := db.ExecContext(ctx, "INSERT INTO public.coordination_schema(version_id,checksum) VALUES($1,$2)", req.Version, sum)
	return err
}
func (*ledger) Delete(context.Context, database.DBTxConn, int64) error {
	return errors.New("forward-only migration ledger")
}
func (l *ledger) GetMigration(ctx context.Context, db database.DBTxConn, version int64) (*database.GetMigrationResult, error) {
	all, err := l.ListMigrations(ctx, db)
	if err != nil {
		return nil, err
	}
	for _, row := range all {
		if row.Version == version {
			result := &database.GetMigrationResult{IsApplied: true}
			err = db.QueryRowContext(ctx, "SELECT applied_at FROM public.coordination_schema WHERE version_id=$1", version).Scan(&result.Timestamp)
			return result, err
		}
	}
	return nil, database.ErrVersionNotFound
}
func (l *ledger) GetLatestVersion(ctx context.Context, db database.DBTxConn) (int64, error) {
	rows, err := l.ListMigrations(ctx, db)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, database.ErrVersionNotFound
	}
	return rows[0].Version, nil
}
func (l *ledger) ListMigrations(ctx context.Context, db database.DBTxConn) ([]*database.ListMigrationsResult, error) {
	rows, err := db.QueryContext(ctx, "SELECT version_id,checksum,is_applied FROM public.coordination_schema ORDER BY version_id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*database.ListMigrationsResult
	for rows.Next() {
		var v int64
		var checksum string
		var applied bool
		if err = rows.Scan(&v, &checksum, &applied); err != nil {
			return nil, err
		}
		expected, ok := l.manifest[v]
		if !ok || !applied || checksum != expected {
			return nil, errors.New("migration compatibility mismatch")
		}
		result = append(result, &database.ListMigrationsResult{Version: v, IsApplied: applied})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, errors.New("empty ledger")
	}
	for i, r := range result {
		if r.Version != int64(len(result)-1-i) {
			return nil, errors.New("migration gap")
		}
	}
	return result, nil
}

func validateLedgerShape(ctx context.Context, db database.DBTxConn) error {
	rows, err := db.QueryContext(ctx, "SELECT column_name||':'||data_type||':'||is_nullable FROM information_schema.columns WHERE table_schema='public' AND table_name='coordination_schema' ORDER BY column_name")
	if err != nil {
		return err
	}
	defer rows.Close()
	var actual []string
	for rows.Next() {
		var column string
		if err = rows.Scan(&column); err != nil {
			return err
		}
		actual = append(actual, column)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	expected := []string{"applied_at:timestamp with time zone:NO", "checksum:text:NO", "is_applied:boolean:NO", "version_id:bigint:NO"}
	if !slices.Equal(actual, expected) {
		return errors.New("incompatible ledger columns")
	}
	return nil
}
