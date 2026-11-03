package store

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"strconv"
	"strings"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed migrations/*.sql migrations/ledger/create.sql
var embeddedMigrations embed.FS
var migrationFS, _ = fs.Sub(embeddedMigrations, "migrations")

// Manifest policy adapts Goose; Goose alone parses and executes migration SQL.
// Version zero checksums the bootstrap DDL. Later versions checksum their exact files.
func loadManifest(source fs.FS) (map[int64]string, error) {
	bootstrap, err := fs.ReadFile(source, "ledger/create.sql")
	if err != nil {
		return nil, err
	}
	sums := map[int64]string{0: digest(bootstrap)}
	names, err := fs.Glob(source, "*.sql")
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, errors.New("empty migration manifest")
	}
	for i, name := range names {
		prefix, _, ok := strings.Cut(name, "_")
		n, e := strconv.ParseInt(prefix, 10, 64)
		if !ok || e != nil || n != int64(i+1) || len(prefix) != 5 {
			return nil, errors.New("unordered migration manifest")
		}
		body, e := fs.ReadFile(source, name)
		if e != nil {
			return nil, e
		}
		up := false
		for _, line := range strings.Split(string(body), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "--" && fields[1] == "+goose" {
				directive := strings.Join(fields[2:], " ")
				switch directive {
				case "Up":
					if up {
						return nil, errors.New("duplicate Up")
					}
					up = true
				case "StatementBegin", "StatementEnd":
				default:
					return nil, errors.New("non-forward or ambient migration directive")
				}
			}
		}
		if !up {
			return nil, errors.New("missing Up")
		}
		sums[n] = digest(body)
	}
	return sums, nil
}
func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// Migrate is an explicit schema operation, independent of Open. It never discovers
// a database or grants admission/execution authority. Only embedded SQL may run.
func Migrate(ctx context.Context, c Config) error { return migrate(ctx, c, migrationFS) }
func migrate(ctx context.Context, c Config, source fs.FS) error {
	manifest, err := loadManifest(source)
	if err != nil {
		return failure(ctx, "manifest", InvalidManifest)
	}
	db, err := connect(ctx, c)
	if err != nil {
		return err
	}
	defer db.Close()
	locker, err := lock.NewPostgresSessionLocker(lock.WithLockTimeout(1, 10), lock.WithUnlockTimeout(1, 2))
	if err != nil {
		return failure(ctx, "lock", MigrationFailed)
	}
	bootstrap, _ := fs.ReadFile(source, "ledger/create.sql")
	provider, err := goose.NewProvider(goose.DialectCustom, db, source,
		goose.WithStore(&ledger{manifest: manifest, bootstrap: string(bootstrap)}),
		goose.WithSessionLocker(locker),
		goose.WithDisableGlobalRegistry(true),
		goose.WithSlog(slog.New(slog.NewTextHandler(io.Discard, nil))))
	if err != nil {
		return failure(ctx, "prepare", InvalidManifest)
	}
	if _, err = provider.Up(ctx); err != nil {
		return failure(ctx, "migrate", MigrationFailed)
	}
	// An independent read checks the final prefix, even if a runner ever reports a
	// successful but incomplete run. Later adapters add their own schema floor.
	records, err := (&ledger{manifest: manifest}).ListMigrations(ctx, db)
	if err != nil || len(records) != len(manifest) {
		return failure(ctx, "complete", Incompatible)
	}
	return nil
}
