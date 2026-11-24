package store

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/medici-finance/assay/workflow/internal/pgtest"
)

func TestOpenRefreshRuntimeRejectsMismatchedConfig(t *testing.T) {
	profile := RefreshRoleProfile{Database: "fixture", Operator: "operator", Runtime: "runtime"}
	for _, tt := range []struct {
		name string
		user string
		tls  *tls.Config
	}{
		{"no TLS", profile.Runtime, nil},
		{"wrong user", profile.Operator, &tls.Config{MinVersion: tls.VersionTLS12}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := Config{Database: profile.Database, User: tt.user, Password: "fixture-only", TLS: tt.tls}
			db, err := OpenRefreshRuntime(context.Background(), c, profile)
			if db != nil {
				_ = db.Close()
			}
			var refusal *Error
			if db != nil || !errors.As(err, &refusal) || refusal.Code != InvalidConfig || refusal.Operation != "open refresh runtime" {
				t.Fatalf("expected a runtime configuration refusal before connecting, got %v", err)
			}
		})
	}
}

// regression: F-runtime-opener-privilege-envelope
func TestPGRefreshRuntimeOpenPrivileges(t *testing.T) {
	if os.Getenv("ASSAY_TEST_POSTGRES_DSN") == "" {
		t.Skip("explicit disposable PostgreSQL fixture required")
	}
	f := pgtest.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := Config{Host: f.Config.Host, Port: f.Config.Port, User: f.Config.User,
		Database: f.Config.Database, Password: f.Config.Password, Timeout: 5 * time.Second}
	if err := Migrate(ctx, c); err != nil {
		t.Fatal(err)
	}
	roles := f.NewRoles(t)
	profile := RefreshRoleProfile{Database: c.Database, Operator: roles.Operator.User, Runtime: roles.Runtime.User}
	manifest, err := profile.SQL()
	if err != nil {
		t.Fatal(err)
	}
	admin, err := f.ConnectOwned(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = admin.Close() }()
	apply := func(t *testing.T) {
		t.Helper()
		if _, err := admin.ExecContext(ctx, manifest); err != nil {
			_, _ = admin.ExecContext(ctx, "ROLLBACK")
			t.Fatal("canonical role manifest refused", err)
		}
	}
	apply(t)

	c.User, c.Password = roles.Runtime.User, roles.Runtime.Password
	if file := os.Getenv("ASSAY_TEST_POSTGRES_TLS_CA"); file != "" {
		pem, err := os.ReadFile(file)
		roots := x509.NewCertPool()
		if err != nil || !roots.AppendCertsFromPEM(pem) {
			t.Fatal("disposable PostgreSQL TLS CA is unavailable")
		}
		c.TLS = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	}
	check := func() error {
		if c.TLS != nil {
			db, err := OpenRefreshRuntime(ctx, c, profile)
			if db != nil {
				_ = db.Close()
			}
			return err
		}
		// CI's PostgreSQL container uses cleartext on loopback. Exercise the
		// opener's same catalog assertion on the fixture's restricted login.
		db, err := Open(ctx, c)
		if err != nil {
			return err
		}
		defer func() { _ = db.Close() }()
		conn, err := db.db.Conn(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()
		return checkRefreshRuntime(ctx, conn, profile)
	}
	if err := check(); err != nil {
		t.Fatal("declared runtime refused", err)
	}

	runtime := pgx.Identifier{profile.Runtime}.Sanitize()
	operator := pgx.Identifier{profile.Operator}.Sanitize()
	owner := pgx.Identifier{f.Config.User}.Sanitize()
	for _, tt := range []struct{ name, widen, restore string }{
		{"createrole", "ALTER ROLE " + runtime + " CREATEROLE", "ALTER ROLE " + runtime + " NOCREATEROLE"},
		{"membership", "GRANT " + operator + " TO " + runtime, "REVOKE " + operator + " FROM " + runtime},
		{"inherit", "ALTER ROLE " + runtime + " INHERIT", "ALTER ROLE " + runtime + " NOINHERIT"},
		{"initialization insert", "GRANT INSERT (kind) ON public.refresh_initializations TO " + runtime, "REVOKE INSERT (kind) ON public.refresh_initializations FROM " + runtime},
		{"ownership", "ALTER TABLE public.refresh_fills OWNER TO " + runtime, "ALTER TABLE public.refresh_fills OWNER TO " + owner},
		{"profile update", "GRANT UPDATE (profile) ON public.refresh_admissions TO " + runtime, "REVOKE UPDATE (profile) ON public.refresh_admissions FROM " + runtime},
		{"security definer", "CREATE FUNCTION public.fixture_definer() RETURNS integer LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog AS 'SELECT 1'", "DROP FUNCTION public.fixture_definer()"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := admin.ExecContext(ctx, tt.widen); err != nil {
				t.Fatal("owned privilege control could not be applied", err)
			}
			err := check()
			if _, err := admin.ExecContext(ctx, tt.restore); err != nil {
				t.Fatal("owned privilege control could not be restored", err)
			}
			var refusal *Error
			if err == nil || c.TLS != nil && (!errors.As(err, &refusal) || refusal.Code != InvalidConfig) {
				t.Fatal("expected the runtime privilege envelope to refuse widened access", err)
			}
			if tt.name == "ownership" {
				// PostgreSQL rewrites ACLs on owner changes. Restore the
				// column and table grants that the original owner issued.
				apply(t)
			}
			if err := check(); err != nil {
				t.Fatal("declared runtime refused after restoring the privilege envelope", err)
			}
		})
	}
}
