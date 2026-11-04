package pgtest

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// regression: F-provision-owned-role-capability
func TestOwnedRefreshRoles(t *testing.T) {
	if os.Getenv("ASSAY_TEST_POSTGRES_DSN") == "" {
		t.Skip("explicit disposable PostgreSQL fixture not supplied; no role proof")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	t.Run("binding", func(t *testing.T) {
		f := New(t)
		roles := f.NewRoles(t)
		if e := f.CheckRoles(ctx, roles); e != nil {
			t.Fatal(e)
		}
		for _, name := range []string{"database", "host", "port", "role", "swap"} {
			foreign := roles
			switch name {
			case "database":
				foreign.Runtime.Database = "postgres"
			case "host":
				foreign.Runtime.Host = "192.0.2.1"
			case "port":
				foreign.Runtime.Port = 1
			case "role":
				foreign.Runtime.User = f.Config.User
			case "swap":
				foreign.Operator, foreign.Runtime = foreign.Runtime, foreign.Operator
			}
			if e := f.CheckRoles(ctx, foreign); e == nil {
				t.Fatal("foreign fixture role binding accepted", name)
			}
			if connection, e := f.ConnectRuntime(ctx, foreign); e == nil {
				connection.Close()
				t.Fatal("foreign fixture role reached a dial", name)
			}
		}
	})
	t.Run("cleanup-marker", func(t *testing.T) {
		f := New(t)
		roles := f.NewRoles(t)
		role := &f.roles[1]
		quoted := pgx.Identifier{role.name}.Sanitize()
		if _, e := f.Admin.ExecContext(ctx, "COMMENT ON ROLE "+quoted+" IS 'synthetic-foreign-marker'"); e != nil {
			t.Fatal("owned marker control unavailable")
		}
		// Restore the exact marker before the normal cleanup even if a control
		// fails. No unrelated role or database is selected by this test.
		defer func() {
			if _, e := f.Admin.ExecContext(ctx, "COMMENT ON ROLE "+quoted+" IS '"+role.marker+"'"); e != nil {
				t.Error("owned marker restoration unavailable")
			}
		}()
		if e := f.CheckRoles(ctx, roles); e == nil {
			t.Fatal("altered ownership marker accepted")
		}
		if e := f.dropRoles(ctx); e == nil {
			t.Fatal("uncertain role ownership was silently pruned")
		}
		var count int
		if e := f.Admin.QueryRowContext(ctx, `SELECT count(*) FROM pg_roles WHERE rolname=$1`, role.name).Scan(&count); e != nil || count != 1 || role.removed {
			t.Fatal("cleanup removed an ownership-uncertain role")
		}
	})
	t.Run("cleanup-ack", func(t *testing.T) {
		f := New(t)
		f.NewRoles(t)
		role := &f.roles[1]
		role.ack = false
		defer func() { role.ack = true }()
		if e := f.dropRoles(ctx); e == nil {
			t.Fatal("unacknowledged creation was silently pruned")
		}
		var count int
		if e := f.Admin.QueryRowContext(ctx, `SELECT count(*) FROM pg_roles WHERE rolname=$1`, role.name).Scan(&count); e != nil || count != 1 || role.removed {
			t.Fatal("cleanup removed an unacknowledged role")
		}
	})
}
