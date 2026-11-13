package journal

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"io/fs"
	"log/slog"
	"path"
	"regexp"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// Schema is loopadmin's own schema in the cell database; VersionTable is the
// goose version table inside it. Neither is shared with the instance store.
const (
	Schema       = "loopadmin"
	VersionTable = Schema + ".journal_schema"
)

// migrationLockID keeps loopadmin's migration lock apart from goose's default,
// which other schemas in the same cell database may use.
const migrationLockID int64 = 0x6c6f6f7061646d6e // "loopadmn"

//go:embed migrations/*.sql
var migrationFS embed.FS

var migrationName = regexp.MustCompile(`^[0-9]{5}_[a-z0-9_]+\.sql$`)

// checkManifest refuses a migration set the journal cannot run forward-only:
// an unexpected file name, or any goose directive other than Up and the
// statement markers. A Down section is refused, not ignored.
func checkManifest(fsys fs.FS) (fs.FS, error) {
	sub, err := fs.Sub(fsys, "migrations")
	if err != nil {
		return nil, failure(nil, "manifest", InvalidManifest)
	}
	entries, err := fs.ReadDir(sub, ".")
	if err != nil || len(entries) == 0 {
		return nil, failure(nil, "manifest", InvalidManifest)
	}
	for _, e := range entries {
		if e.IsDir() || !migrationName.MatchString(e.Name()) {
			return nil, failure(nil, "manifest", InvalidManifest)
		}
		body, err := fs.ReadFile(sub, path.Clean(e.Name()))
		if err != nil {
			return nil, failure(nil, "manifest", InvalidManifest)
		}
		ups := 0
		for _, line := range bytes.Split(body, []byte("\n")) {
			line = bytes.TrimSpace(line)
			if !bytes.HasPrefix(line, []byte("-- +goose")) {
				continue
			}
			switch string(bytes.TrimSpace(bytes.TrimPrefix(line, []byte("-- +goose")))) {
			case "Up":
				ups++
			case "StatementBegin", "StatementEnd":
			default:
				return nil, failure(nil, "manifest", InvalidManifest)
			}
		}
		if ups != 1 {
			return nil, failure(nil, "manifest", InvalidManifest)
		}
	}
	return sub, nil
}

func provider(db *sql.DB) (*goose.Provider, error) {
	sub, err := checkManifest(migrationFS)
	if err != nil {
		return nil, err
	}
	locker, err := lock.NewPostgresSessionLocker(lock.WithLockID(migrationLockID), lock.WithLockTimeout(1, 10), lock.WithUnlockTimeout(1, 2))
	if err != nil {
		return nil, failure(nil, "migrate", MigrationFailed)
	}
	p, err := goose.NewProvider(goose.DialectPostgres, db, sub,
		goose.WithTableName(VersionTable),
		goose.WithSessionLocker(locker),
		goose.WithDisableGlobalRegistry(true),
		goose.WithSlog(slog.New(slog.DiscardHandler)),
	)
	if err != nil {
		return nil, failure(nil, "migrate", InvalidManifest)
	}
	return p, nil
}

// Migrate creates loopadmin's schema if absent and applies every pending
// forward migration. It is the operator's install step, never run implicitly
// by Open.
func Migrate(ctx context.Context, c Config) error {
	db, err := connect(ctx, c)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err = db.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS "+Schema); err != nil {
		return failure(ctx, "migrate", MigrationFailed)
	}
	p, err := provider(db)
	if err != nil {
		return err
	}
	if _, err = p.Up(ctx); err != nil {
		return failure(ctx, "migrate", MigrationFailed)
	}
	return nil
}

// checkSchema refuses a database whose journal schema is missing, behind or
// ahead of this build's migrations.
func checkSchema(ctx context.Context, db *sql.DB) error {
	var present bool
	if err := db.QueryRowContext(ctx, "SELECT to_regclass($1) IS NOT NULL", VersionTable).Scan(&present); err != nil {
		return classify(ctx, "open", err)
	}
	if !present {
		return failure(ctx, "open", Incompatible)
	}
	p, err := provider(db)
	if err != nil {
		return err
	}
	current, target, err := p.GetVersions(ctx)
	if err != nil || current != target {
		return failure(ctx, "open", Incompatible)
	}
	return nil
}
