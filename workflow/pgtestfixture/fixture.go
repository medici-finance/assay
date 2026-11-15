// Package pgtestfixture exposes only the canonical disposable-database test
// harness. It starts/discovers no server and is not a production initialization API.
package pgtestfixture

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/medici-finance/assay/workflow/internal/pgtest"
	"github.com/medici-finance/assay/workflow/internal/refreshcodec"
	"github.com/medici-finance/assay/workflow/store"
)

type Fixture = pgtest.Fixture
type Connection = pgtest.Connection
type RoleConnections = pgtest.RoleConnections

func New(t testing.TB) *Fixture { t.Helper(); return pgtest.New(t) }

// NewRefreshRoles applies only the production-declared role manifest, only to
// identities this exact disposable fixture created and marked. It is test
// support, not an operational grant/role installer or profile initializer.
func NewRefreshRoles(t testing.TB, f *Fixture) RoleConnections {
	t.Helper()
	roles := f.NewRoles(t)
	if e := ApplyRefreshRoles(t, f, roles); e != nil {
		t.Fatal(e)
	}
	return roles
}

// ApplyRefreshRoles rechecks and reapplies only that same bounded declaration to
// this fixture's marked roles. It exposes no arbitrary grant text or SQL callback.
func ApplyRefreshRoles(t testing.TB, f *Fixture, roles RoleConnections) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if e := f.CheckRoles(ctx, roles); e != nil {
		return e
	}
	manifest, e := (store.RefreshRoleProfile{Database: f.Config.Database, Operator: roles.Operator.User, Runtime: roles.Runtime.User}).SQL()
	if e != nil {
		return e
	}
	db, e := f.ConnectOwned(ctx)
	if e != nil {
		return e
	}
	defer db.Close()
	connection, e := db.Conn(ctx)
	if e != nil {
		return errors.New("owned role manifest connection unavailable")
	}
	defer connection.Close()
	if _, e = connection.ExecContext(ctx, manifest); e != nil {
		connection.ExecContext(ctx, "ROLLBACK")
		return errors.New("declared fixture role manifest refused")
	}
	return nil
}

// SeedRefresh is explicitly synthetic test support. It verifies the existing
// fixture's exact owned database and never replaces an existing usage profile.
func SeedRefresh(t testing.TB, f *Fixture, c store.RefreshConfig, callers ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, e := f.ConnectOwned(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	profile, e := refreshcodec.Profile([6]string{c.SourceID, c.AdmissionID, c.AccessEpoch, c.ConfigEpoch, c.PolicyClass, c.PolicyEpoch}, c.Limits, refreshcodec.Native(c.Native), c.PollInterval)
	if e != nil {
		t.Fatal(e)
	}
	budget, e := refreshcodec.Budget(refreshcodec.Native(c.Native))
	if e != nil {
		t.Fatal(e)
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `INSERT INTO public.refresh_budgets(source_id,profile,known,window_at,window_ns,http_used,http_limit,inflight_limit,reservation_limit) VALUES($1,$2,true,clock_timestamp(),$3,0,$4,$5,$6) ON CONFLICT(source_id) DO NOTHING`, []byte(c.SourceID), budget, int64(c.Native.Window), c.Native.SourcePerWindow, c.Native.Concurrent, c.Native.Reservations); e != nil {
		t.Fatal("synthetic budget seed refused")
	}
	var existing []byte
	var known bool
	if tx.QueryRowContext(ctx, `SELECT profile,known FROM public.refresh_budgets WHERE source_id=$1 FOR UPDATE`, []byte(c.SourceID)).Scan(&existing, &known) != nil || !known || !bytes.Equal(existing, budget) {
		t.Fatal("synthetic shared budget profile mismatch")
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO public.refresh_admissions(admission,source_id,profile,known,window_at,window_ns,fills_used,fill_limit,http_used,http_limit,active_limit,waiter_limit,failure_limit) VALUES($1,$2,$3,true,clock_timestamp(),$4,0,$5,0,$6,$7,$8,$9)`, []byte(c.AdmissionID), []byte(c.SourceID), profile, int64(c.Limits.FillWindow), c.Limits.AdmissionFillsPerWindow, c.Native.AdmissionPerWindow, c.Limits.AdmissionFills, c.Limits.AdmissionWaiters, c.Limits.FailureEntries); e != nil {
		t.Fatal("synthetic admission seed refused")
	}
	for _, id := range callers {
		if _, e = tx.ExecContext(ctx, `INSERT INTO public.refresh_callers(admission,caller_id,known,window_at,fills_used,fill_limit,http_used,http_limit,active_limit,waiter_limit) VALUES($1,$2,true,clock_timestamp(),0,$3,0,$4,$5,$6)`, []byte(c.AdmissionID), []byte(id), c.Limits.CallerFillsPerWindow, c.Native.CallerPerWindow, c.Limits.ActiveFills, c.Limits.QueuedCallers); e != nil {
			t.Fatal("synthetic caller seed refused")
		}
	}
	if e = tx.Commit(); e != nil {
		t.Fatal("synthetic seed transaction failed")
	}
}
