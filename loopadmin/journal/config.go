// Package journal is the loop-admin execution journal: process facts
// (invocations, receipts, lifecycle, limits, usage, checkpoints) kept in the
// cell's PostgreSQL coordination database, in loopadmin's own schema with its
// own forward-only migrations and version table.
//
// The journal owns process facts only. Work, decisions and acceptance stay
// with the desk queues and the instance store; caller intent there reconciles
// with a journal receipt by stable request ID. There is no embedded engine and
// no local fallback: an unreachable database refuses the launch.
//
// Before every launch and every receipt delivery the journal compares the
// current operator incarnation (spec/operator-incarnation-v1.md) with its
// passive activation record. A missing record, or one naming another
// incarnation, holds the journal paused until the operator records the current
// incarnation together with a confirmed stop of the execution group.
package journal

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

// Code classifies a journal refusal. Errors carry a code and an operation,
// never driver text, so a refusal cannot echo a credential or a query.
type Code string

const (
	InvalidConfig   Code = "invalid-config"
	Unavailable     Code = "unavailable"
	Incompatible    Code = "incompatible-schema"
	InvalidManifest Code = "invalid-manifest"
	MigrationFailed Code = "migration-failed"
	StoreFailed     Code = "store-failed"
)

// Error is a classified journal failure.
type Error struct {
	Code      Code
	Operation string
	cause     error
}

func (e *Error) Error() string { return "loopadmin/journal: " + e.Operation + ": " + string(e.Code) }

// Unwrap exposes only a context error, never the driver's.
func (e *Error) Unwrap() error { return e.cause }

func failure(ctx context.Context, op string, code Code) error {
	var cause error
	if ctx != nil {
		cause = ctx.Err()
	}
	return &Error{Code: code, Operation: op, cause: cause}
}

// Config is an explicit connection to the cell's coordination database. No
// field is read from the ambient PG* environment, a password file or a
// service file.
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

// driverConfig builds the driver configuration from a constant seed and the
// explicit fields. It refuses a non-loopback TCP host without TLS, a weakened
// TLS configuration, and any ambient PG* variable that could redirect it.
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
	ip := net.ParseIP(c.Host)
	if !filepath.IsAbs(c.Host) && c.TLS == nil && (ip == nil || !ip.IsLoopback()) {
		return nil, failure(nil, "configure", InvalidConfig)
	}
	if c.TLS != nil && (c.TLS.InsecureSkipVerify || c.TLS.MinVersion < tls.VersionTLS12) {
		return nil, failure(nil, "configure", InvalidConfig)
	}
	for _, entry := range os.Environ() {
		if key, value, ok := strings.Cut(entry, "="); ok && strings.HasPrefix(key, "PG") && value != "" {
			return nil, failure(nil, "configure", InvalidConfig)
		}
	}
	cfg, err := pgx.ParseConfig("host=127.0.0.1 port=1 user=unused dbname=unused password=unused sslmode=disable connect_timeout=5")
	if err != nil {
		return nil, failure(nil, "configure", InvalidConfig)
	}
	cfg.Host, cfg.Port, cfg.User, cfg.Database, cfg.Password = c.Host, c.Port, c.User, c.Database, c.Password
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
	cfg.RuntimeParams = map[string]string{"application_name": "assay_loopadmin", "search_path": "pg_catalog"}
	cfg.KerberosSrvName = ""
	cfg.KerberosSpn = ""
	cfg.DefaultQueryExecMode = pgx.QueryExecModeExec
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

// classify maps a driver error to a code without keeping its text.
func classify(ctx context.Context, op string, err error) error {
	if err == nil {
		return nil
	}
	var known *Error
	if errors.As(err, &known) {
		return err
	}
	if errors.Is(err, sql.ErrConnDone) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return failure(ctx, op, Unavailable)
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return failure(ctx, op, Unavailable)
	}
	return failure(ctx, op, StoreFailed)
}
