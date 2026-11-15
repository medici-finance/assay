// Package store owns the canonical PostgreSQL connection and migration substrate.
// It grants no workflow admission, claims, effect or activation authority.
package store

import (
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

type Code string

const (
	InvalidConfig   Code = "invalid-config"
	Unavailable     Code = "unavailable"
	Incompatible    Code = "incompatible-schema"
	InvalidManifest Code = "invalid-manifest"
	MigrationFailed Code = "migration-failed"
)

// Error deliberately omits driver errors, DSNs and PostgreSQL message/detail text.
// Cancellation retains errors.Is compatibility without exposing a driver wrapper.
type Error struct {
	Code      Code
	Operation string
	cause     error
}

func (e *Error) Error() string { return "workflow/store: " + e.Operation + ": " + string(e.Code) }
func (e *Error) Unwrap() error { return e.cause }
func failure(ctx context.Context, op string, code Code) error {
	var cause error
	if ctx != nil {
		cause = ctx.Err()
	}
	return &Error{Code: code, Operation: op, cause: cause}
}

// Config is supplied by the connection owner, never populated from process state.
// TLS is mandatory for TCP except a literal loopback address. Unix sockets may omit it.
type Config struct {
	Host     string
	Port     uint16
	User     string
	Database string
	Password string
	TLS      *tls.Config
	Timeout  time.Duration
}

func (Config) String() string { return "explicit PostgreSQL configuration (redacted)" }

func driverConfig(c Config) (*pgx.ConnConfig, error) {
	if c.Host == "" || c.Port == 0 || c.User == "" || c.Database == "" || c.Timeout <= 0 || c.Timeout > time.Minute {
		return nil, failure(nil, "configure", InvalidConfig)
	}
	for _, v := range []string{c.Host, c.User, c.Database, c.Password} {
		if strings.ContainsRune(v, 0) {
			return nil, failure(nil, "configure", InvalidConfig)
		}
	}
	if strings.Contains(c.Host, ",") {
		return nil, failure(nil, "configure", InvalidConfig)
	}
	socket := filepath.IsAbs(c.Host)
	ip := net.ParseIP(c.Host)
	if !socket && c.TLS == nil && (ip == nil || !ip.IsLoopback()) {
		return nil, failure(nil, "configure", InvalidConfig)
	}
	if c.TLS != nil && (c.TLS.InsecureSkipVerify || c.TLS.MinVersion < tls.VersionTLS12) {
		return nil, failure(nil, "configure", InvalidConfig)
	}
	// pgx deliberately supports libpq environment and service/password files. The
	// production API refuses that ambient state before invoking its parser; it never
	// chooses a credential, endpoint, TLS policy or fallback from those values.
	for _, entry := range os.Environ() {
		if key, value, ok := strings.Cut(entry, "="); ok && strings.HasPrefix(key, "PG") && value != "" {
			return nil, failure(nil, "configure", InvalidConfig)
		}
	}
	// A nonempty seed password disables .pgpass discovery. No supplied values are
	// interpolated into this constant. All connection and protocol state is assigned
	// below, including empty password, TLS and the absence of fallback endpoints.
	cfg, err := pgx.ParseConfig("host=127.0.0.1 port=1 user=unused dbname=unused password=unused sslmode=disable connect_timeout=5")
	if err != nil {
		return nil, failure(nil, "configure", InvalidConfig)
	}
	cfg.Host = c.Host
	cfg.Port = c.Port
	cfg.User = c.User
	cfg.Database = c.Database
	cfg.Password = c.Password
	cfg.TLSConfig = nil
	if c.TLS != nil {
		cfg.TLSConfig = c.TLS.Clone()
		if cfg.TLSConfig.ServerName == "" {
			cfg.TLSConfig.ServerName = c.Host
		}
	}
	cfg.ConnectTimeout = c.Timeout
	cfg.DialFunc = (&net.Dialer{Timeout: c.Timeout}).DialContext
	cfg.Fallbacks = nil
	cfg.ValidateConnect = nil
	cfg.AfterConnect = nil
	cfg.AfterNetConnect = nil
	cfg.OAuthTokenProvider = nil
	cfg.RuntimeParams = map[string]string{"application_name": "assay_workflow", "search_path": "pg_catalog,public"}
	cfg.KerberosSrvName = ""
	cfg.KerberosSpn = ""
	cfg.SSLNegotiation = "postgres"
	cfg.SSLCertMode = "disable"
	cfg.RequireAuth = ""
	cfg.ChannelBinding = "prefer"
	cfg.MinProtocolVersion = "3.0"
	cfg.MaxProtocolVersion = "3.0"
	cfg.DefaultQueryExecMode = pgx.QueryExecModeExec
	cfg.MaxProtocolMessageBodyLen = 16 << 20
	return cfg, nil
}

func connect(ctx context.Context, c Config) (*sql.DB, error) {
	cfg, err := driverConfig(c)
	if err != nil {
		return nil, err
	}
	db := stdlib.OpenDB(*cfg)
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	ping, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	if err = db.PingContext(ping); err != nil {
		db.Close()
		return nil, failure(ping, "connect", Unavailable)
	}
	return db, nil
}

// DB is a compatible foundation handle. Opening it never performs migration DDL.
// Later domain adapters live under this same owner and consume its private pool.
type DB struct{ db *sql.DB }

func (d *DB) Close() error { return d.db.Close() }
func Open(ctx context.Context, c Config) (*DB, error) {
	db, err := connect(ctx, c)
	if err != nil {
		return nil, err
	}
	manifest, err := loadManifest(migrationFS)
	if err == nil {
		err = validateLedgerShape(ctx, db)
	}
	if err == nil {
		_, err = (&ledger{manifest: manifest}).ListMigrations(ctx, db)
	}
	if err == nil {
		var version int64
		err = db.QueryRowContext(ctx, "SELECT max(version_id) FROM public.coordination_schema").Scan(&version)
		if version != int64(len(manifest)-1) {
			err = errors.New("incomplete schema")
		}
	}
	if err != nil {
		db.Close()
		return nil, failure(ctx, "open", Incompatible)
	}
	return &DB{db: db}, nil
}
