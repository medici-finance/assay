// Package pgtest manages only uniquely owned databases inside an explicitly
// supplied disposable PostgreSQL container. It never starts or discovers a server.
package pgtest

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

type Connection struct {
	Host                     string
	Port                     uint16
	User, Password, Database string
}

func (Connection) String() string { return "disposable PostgreSQL configuration (redacted)" }
func Parse(raw string) (Connection, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return Connection{}, errors.New("explicit container DSN is required")
	}
	if u.Scheme != "postgresql" && u.Scheme != "postgres" || u.User == nil || u.Fragment != "" {
		return Connection{}, errors.New("explicit container DSN is required")
	}
	ip := net.ParseIP(u.Hostname())
	port, err := strconv.ParseUint(u.Port(), 10, 16)
	pw, hasPW := u.User.Password()
	db := strings.TrimPrefix(u.Path, "/")
	q := u.Query()
	if err != nil || port == 0 || ip == nil || !ip.IsLoopback() || u.User.Username() == "" || !hasPW || pw == "" || db != "postgres" || len(q) < 1 || len(q) > 2 || q.Get("sslmode") != "disable" {
		return Connection{}, errors.New("fixture requires an explicit literal-loopback container DSN with user, password, port and postgres database")
	}
	for key, values := range q {
		if len(values) != 1 || (key != "sslmode" && key != "connect_timeout") {
			return Connection{}, errors.New("fixture DSN contains unsupported settings")
		}
	}
	if value := q.Get("connect_timeout"); value != "" {
		seconds, e := strconv.Atoi(value)
		if e != nil || seconds < 1 || seconds > 10 {
			return Connection{}, errors.New("fixture timeout is invalid")
		}
	}
	return Connection{Host: u.Hostname(), Port: uint16(port), User: u.User.Username(), Password: pw, Database: db}, nil
}
func (c Connection) URI() string {
	u := url.URL{Scheme: "postgresql", Host: net.JoinHostPort(c.Host, strconv.Itoa(int(c.Port))), User: url.UserPassword(c.User, c.Password), Path: "/" + c.Database}
	u.RawQuery = "sslmode=disable"
	return u.String()
}
func ScrubEnvironment(env []string) []string {
	result := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if !strings.HasPrefix(key, "PG") {
			result = append(result, entry)
		}
	}
	return result
}
func CleanEnvironment(t testing.TB) {
	t.Helper()
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "PG") {
			t.Setenv(key, "")
		}
	}
}
func dial(ctx context.Context, c Connection) (*sql.DB, error) {
	cfg, err := pgx.ParseConfig(c.URI())
	if err != nil {
		return nil, errors.New("fixture configuration failed")
	}
	cfg.Fallbacks = nil
	cfg.ConnectTimeout = 5 * time.Second
	db := stdlib.OpenDB(*cfg)
	db.SetMaxOpenConns(4)
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, errors.New("explicit container fixture is unavailable")
	}
	return db, nil
}

type Fixture struct {
	Config Connection
	Admin  *sql.DB
	marker string
	owned  bool
	ready  bool
}

func New(t testing.TB) *Fixture {
	t.Helper()
	CleanEnvironment(t)
	config, err := Parse(os.Getenv("ASSAY_TEST_POSTGRES_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	admin, err := dial(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, 16)
	if _, err = rand.Read(nonce); err != nil {
		admin.Close()
		t.Fatal("fixture identity unavailable")
	}
	id := hex.EncodeToString(nonce)
	f := &Fixture{Config: config, Admin: admin, marker: "assay-test-owned:" + id}
	f.Config.Database = "assay_pg_" + id
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})
	if err = f.create(ctx); err != nil {
		t.Fatal("fixture create failed")
	}
	return f
}
func (f *Fixture) create(ctx context.Context) error {
	if !strings.HasPrefix(f.Config.Database, "assay_pg_") {
		return errors.New("unowned fixture database")
	}
	name := pgx.Identifier{f.Config.Database}.Sanitize()
	if _, err := f.Admin.ExecContext(ctx, "CREATE DATABASE "+name); err != nil {
		return err
	}
	f.owned = true
	// Both name and marker are generated hex, never caller-controlled SQL input.
	if _, err := f.Admin.ExecContext(ctx, "COMMENT ON DATABASE "+name+" IS '"+f.marker+"'"); err != nil {
		return err
	}
	f.ready = true
	return nil
}
func (f *Fixture) drop(ctx context.Context) error {
	if !f.owned || !strings.HasPrefix(f.Config.Database, "assay_pg_") {
		return errors.New("unowned fixture database")
	}
	var marker string
	err := f.Admin.QueryRowContext(ctx, "SELECT coalesce(shobj_description(oid,'pg_database'),'') FROM pg_database WHERE datname=$1 AND datdba=(SELECT oid FROM pg_roles WHERE rolname=current_user)", f.Config.Database).Scan(&marker)
	if err != nil || (marker != f.marker && (f.ready || marker != "")) {
		return errors.New("fixture ownership mismatch; database retained")
	}
	if _, err = f.Admin.ExecContext(ctx, "DROP DATABASE "+pgx.Identifier{f.Config.Database}.Sanitize()+" WITH (FORCE)"); err != nil {
		return errors.New("owned fixture database cleanup failed")
	}
	f.owned = false
	f.ready = false
	return nil
}
func (f *Fixture) Reset(ctx context.Context) error {
	if err := f.drop(ctx); err != nil {
		return err
	}
	return f.create(ctx)
}
func (f *Fixture) Close() error {
	if !f.owned {
		return f.Admin.Close()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := f.drop(ctx)
	closeErr := f.Admin.Close()
	if err != nil {
		return err
	}
	return closeErr
}
