package journal

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// testDSN names the disposable PostgreSQL server the PostgreSQL-backed tests
// run against. An unset value fails the test; nothing self-skips.
const testDSN = "ASSAY_TEST_POSTGRES_DSN"

// pgFixture owns databases it created on the disposable server, each named
// with a fixed prefix and a random suffix, and drops them at cleanup.
type pgFixture struct {
	t     testing.TB
	admin *sql.DB
	base  Config
	mu    sync.Mutex
	owned map[string]bool
}

func clearPGEnv(t testing.TB) {
	for _, entry := range os.Environ() {
		if key, _, ok := strings.Cut(entry, "="); ok && strings.HasPrefix(key, "PG") {
			t.Setenv(key, "")
		}
	}
}

func parseDSN(raw string) (Config, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.User == nil {
		return Config{}, errors.New("unusable test DSN")
	}
	port, err := strconv.ParseUint(u.Port(), 10, 16)
	if err != nil {
		return Config{}, errors.New("unusable test DSN port")
	}
	password, _ := u.User.Password()
	return Config{
		Host: u.Hostname(), Port: uint16(port), User: u.User.Username(), Password: password,
		Database: strings.TrimPrefix(u.Path, "/"), Timeout: 10 * time.Second,
	}, nil
}

func newPG(t testing.TB) *pgFixture {
	t.Helper()
	raw := os.Getenv(testDSN)
	if raw == "" {
		t.Fatal(testDSN + " is unset: this test needs a disposable PostgreSQL server and never self-skips")
	}
	clearPGEnv(t)
	base, err := parseDSN(raw)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	admin, err := connect(ctx, base)
	if err != nil {
		t.Fatalf("disposable PostgreSQL unreachable: %v", err)
	}
	f := &pgFixture{t: t, admin: admin, base: base, owned: map[string]bool{}}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		f.mu.Lock()
		names := make([]string, 0, len(f.owned))
		for name := range f.owned {
			names = append(names, name)
		}
		f.mu.Unlock()
		for _, name := range names {
			f.drop(ctx, name)
		}
		admin.Close()
	})
	return f
}

func (f *pgFixture) name() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		f.t.Fatal(err)
	}
	return "loopadmin_journal_" + hex.EncodeToString(b)
}

// create makes a fresh database (optionally a copy of template) and returns
// its configuration.
func (f *pgFixture) create(ctx context.Context, template string) Config {
	f.t.Helper()
	name := f.name()
	stmt := "CREATE DATABASE " + pgx.Identifier{name}.Sanitize()
	if template != "" {
		stmt += " TEMPLATE " + pgx.Identifier{template}.Sanitize()
	}
	if _, err := f.admin.ExecContext(ctx, stmt); err != nil {
		f.t.Fatalf("create fixture database: %v", err)
	}
	f.mu.Lock()
	f.owned[name] = true
	f.mu.Unlock()
	c := f.base
	c.Database = name
	return c
}

// drop removes a database this fixture created, closing other sessions.
func (f *pgFixture) drop(ctx context.Context, name string) {
	f.t.Helper()
	f.mu.Lock()
	owned := f.owned[name]
	f.mu.Unlock()
	if !owned || !strings.HasPrefix(name, "loopadmin_journal_") {
		f.t.Errorf("refusing to drop a database the fixture does not own")
		return
	}
	if _, err := f.admin.ExecContext(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
		f.t.Errorf("drop fixture database: %v", err)
		return
	}
	f.mu.Lock()
	delete(f.owned, name)
	f.mu.Unlock()
}

// operator is a fake protected operator-state source: the test plays the
// trusted operator and rotates its current record.
type operator struct {
	mu  sync.Mutex
	cur map[string]Incarnation
	err error
}

func (o *operator) CurrentIncarnation(_ context.Context, cell string) (Incarnation, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.err != nil {
		return Incarnation{}, o.err
	}
	return o.cur[cell], nil
}

func (o *operator) rotate(cell, id, key string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.cur == nil {
		o.cur = map[string]Incarnation{}
	}
	o.cur[cell] = Incarnation{Schema: IncarnationSchema, CellID: cell, ID: id, KeyRef: key}
}
