package store

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/medici-finance/assay/workflow/internal/pgtest"
	"github.com/pressly/goose/v3/lock"
)

func testConfig(c pgtest.Connection) Config {
	return Config{Host: c.Host, Port: c.Port, User: c.User, Password: c.Password, Database: c.Database, Timeout: 5 * time.Second}
}
func fixture(t *testing.T) (*pgtest.Fixture, Config, *sql.DB) {
	t.Helper()
	f := pgtest.New(t)
	c := testConfig(f.Config)
	db, err := connect(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return f, c, db
}
func extraSource(t *testing.T, sql string) fstest.MapFS {
	t.Helper()
	source := fstest.MapFS{}
	err := fs.WalkDir(migrationFS, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		body, err := fs.ReadFile(migrationFS, name)
		if err != nil {
			return err
		}
		source[name] = &fstest.MapFile{Data: body}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := loadManifest(migrationFS)
	if err != nil {
		t.Fatal(err)
	}
	if sql != "" {
		source[fmt.Sprintf("%05d_control.sql", len(manifest))] = &fstest.MapFile{Data: []byte("-- +goose Up\n" + sql)}
	}
	return source
}
func nextVersion(t *testing.T) int64 {
	t.Helper()
	m, err := loadManifest(migrationFS)
	if err != nil {
		t.Fatal(err)
	}
	return int64(len(m))
}

func requireError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("refusal missing")
	}
}
func TestPGFoundationMigrations(t *testing.T) {
	_, c, db := fixture(t)
	ctx := context.Background()
	requireError(t, func() error {
		d, e := Open(ctx, c)
		if d != nil {
			d.Close()
		}
		return e
	}())
	if err := Migrate(ctx, c); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		d, err := Open(ctx, c)
		if err != nil {
			t.Fatal(err)
		}
		d.Close()
		if err = Migrate(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	broken := extraSource(t, "CREATE TABLE public.rollback_canary(id integer);\nSELECT 1/0;")
	requireError(t, migrate(ctx, c, broken))
	var present bool
	if err := db.QueryRow("SELECT to_regclass('public.rollback_canary') IS NOT NULL").Scan(&present); err != nil {
		t.Fatal(err)
	}
	if present {
		t.Fatal("failed DDL persisted")
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM public.coordination_schema WHERE version_id=$1", nextVersion(t)).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("failed migration acknowledged")
	}
	if err := Migrate(ctx, c); err != nil {
		t.Fatal("failed migration leaked lock:", err)
	}
	healthy := extraSource(t, "CREATE TABLE public.healthy_canary(id integer);")
	if err := migrate(ctx, c, healthy); err != nil {
		t.Fatal(err)
	}
	requireError(t, func() error {
		d, e := Open(ctx, c)
		if d != nil {
			d.Close()
		}
		return e
	}()) // old binaries cannot use a future mandatory schema
}

const raceBarrier int64 = 820513

func TestPGFoundationConcurrent(t *testing.T) {
	if raw := os.Getenv("ASSAY_PG_CHILD_CONFIG"); raw != "" {
		var c Config
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			t.Fatal("child config")
		}
		fmt.Println("ready")
		if err := migrate(context.Background(), c, extraSource(t, fmt.Sprintf("SELECT pg_advisory_xact_lock(%d);\nCREATE TABLE public.race_canary(id integer);", raceBarrier))); err != nil {
			t.Fatal(err)
		}
		return
	}
	_, c, db := fixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	gate, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer gate.Close()
	if _, err = gate.ExecContext(ctx, "SELECT pg_advisory_lock($1)", raceBarrier); err != nil {
		t.Fatal(err)
	}
	defer gate.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", raceBarrier)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(c)
	launch := func() (*exec.Cmd, <-chan error) {
		cmd := exec.CommandContext(ctx, exe, "-test.run=^TestPGFoundationConcurrent$", "-test.timeout=20s")
		cmd.Env = append(pgtest.ScrubEnvironment(os.Environ()), "ASSAY_PG_CHILD_CONFIG="+string(raw))
		out, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		cmd.Stderr = os.Stderr
		if err = cmd.Start(); err != nil {
			t.Fatal(err)
		}
		reader := bufio.NewReader(out)
		line, err := reader.ReadString('\n')
		if err != nil || strings.TrimSpace(line) != "ready" {
			t.Fatal("child did not reach barrier")
		}
		done := make(chan error, 1)
		go func() {
			for {
				if _, err := reader.ReadString('\n'); err != nil {
					break
				}
			}
			done <- cmd.Wait()
		}()
		return cmd, done
	}
	first, a := launch()
	defer first.Process.Kill()
	// Wait for the actual upstream PostgreSQL migration lock, not a timing guess.
	for {
		var held bool
		if err = db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND objid::bigint=$1 AND granted AND database=(SELECT oid FROM pg_database WHERE datname=current_database()))", int64(lock.DefaultLockID)).Scan(&held); err != nil {
			t.Fatal(err)
		}
		if held {
			break
		}
		select {
		case err := <-a:
			t.Fatalf("first exited before lock: %v", err)
		case <-ctx.Done():
			t.Fatal("migration lock not observed")
		case <-time.After(10 * time.Millisecond):
		}
	}
	second, b := launch()
	defer second.Process.Kill()
	select {
	case err := <-b:
		t.Fatalf("second completed while first migration blocked: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	if _, err = gate.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", raceBarrier); err != nil {
		t.Fatal(err)
	}
	for _, done := range []<-chan error{a, b} {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal("independent migrator failed:", err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	var count int
	if err = db.QueryRow("SELECT count(*) FROM public.coordination_schema").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != int(nextVersion(t))+1 {
		t.Fatalf("migration recorded %d rows", count)
	}
}

func TestPGFoundationCompatibility(t *testing.T) {
	cases := map[string]string{
		"checksum": "UPDATE public.coordination_schema SET checksum=repeat('0',64) WHERE version_id=1",
		"gap":      "DELETE FROM public.coordination_schema WHERE version_id=0",
		"future":   "INSERT INTO public.coordination_schema(version_id,checksum) VALUES(99,repeat('0',64))",
		"empty":    "DELETE FROM public.coordination_schema",
	}
	for name, sql := range cases {
		t.Run(name, func(t *testing.T) {
			_, c, db := fixture(t)
			ctx := context.Background()
			if err := Migrate(ctx, c); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(sql); err != nil {
				t.Fatal(err)
			}
			manifest, _ := loadManifest(migrationFS)
			_, err := (&ledger{manifest: manifest}).ListMigrations(ctx, db)
			requireError(t, err)
			requireError(t, Migrate(ctx, c))
			d, err := Open(ctx, c)
			if d != nil {
				d.Close()
			}
			requireError(t, err)
		})
	}
	for name, source := range map[string]fstest.MapFS{
		"unordered": extraSource(t, "SELECT 1;"), "duplicate": extraSource(t, "SELECT 1;"), "down": extraSource(t, "-- +goose Down\nSELECT 1;"), "non-transaction": extraSource(t, "-- +goose NO TRANSACTION\nSELECT 1;"),
	} {
		t.Run(name, func(t *testing.T) {
			switch name {
			case "unordered":
				control := fmt.Sprintf("%05d_control.sql", nextVersion(t))
				source[fmt.Sprintf("%05d_gap.sql", nextVersion(t)+1)] = source[control]
				delete(source, control)
			case "duplicate":
				source["00001_duplicate.sql"] = source["00001_foundation.sql"]
			}
			_, err := loadManifest(source)
			requireError(t, err)
		})
	}
}

func schemaColumns(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT table_name||'.'||column_name||':'||data_type FROM information_schema.columns WHERE table_schema='public' ORDER BY table_name,column_name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var s string
		if err = rows.Scan(&s); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}
func contractColumns(text string) []string {
	section := strings.Split(text, "## Planned domain schema")[0]
	var result []string
	for _, line := range strings.Split(section, "\n") {
		cells := strings.Split(line, "|")
		if len(cells) == 5 && strings.TrimSpace(cells[1]) != "" && strings.TrimSpace(cells[1]) != "Table" && !strings.HasPrefix(strings.TrimSpace(cells[1]), "---") {
			result = append(result, strings.TrimSpace(cells[1])+"."+strings.TrimSpace(cells[2])+":"+strings.TrimSpace(cells[3]))
		}
	}
	slices.Sort(result)
	return result
}
func TestPGFoundationSchema(t *testing.T) {
	_, c, db := fixture(t)
	if err := Migrate(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../spec/coordination-store-v1.md")
	if err != nil {
		t.Fatal(err)
	}
	expected := contractColumns(string(raw))
	actual, err := schemaColumns(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if len(expected) < 4 || !reflect.DeepEqual(expected, actual) {
		t.Fatalf("contract/catalog mismatch: %v versus %v", expected, actual)
	}
	// Negative controls prove both directions, beyond file presence.
	if reflect.DeepEqual(expected[:len(expected)-1], actual) {
		t.Fatal("missing contract column accepted")
	}
	if _, err = db.Exec("ALTER TABLE public.coordination_schema DROP COLUMN applied_at"); err != nil {
		t.Fatal(err)
	}
	d, openErr := Open(context.Background(), c)
	if d != nil {
		d.Close()
	}
	requireError(t, openErr)
	changed, _ := schemaColumns(context.Background(), db)
	if reflect.DeepEqual(expected, changed) {
		t.Fatal("missing SQL column accepted")
	}
	if _, err = db.Exec("CREATE TABLE public.unexpected_table(id integer)"); err != nil {
		t.Fatal(err)
	}
	changed, _ = schemaColumns(context.Background(), db)
	if reflect.DeepEqual(expected, changed) {
		t.Fatal("unexpected table accepted")
	}
}

func TestPGFoundationNoFallback(t *testing.T) {
	_, c, _ := fixture(t)
	ctx := context.Background()
	if err := Migrate(ctx, c); err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string]Config{"empty": {}, "missing-host": {User: "x", Database: "x", Port: 1, Timeout: time.Second}, "unreachable": {Host: "127.0.0.1", Port: 1, User: "x", Database: "x", Password: "synthetic-secret", Timeout: time.Second}} {
		t.Run(name, func(t *testing.T) {
			d, err := Open(ctx, bad)
			if d != nil {
				d.Close()
			}
			requireError(t, err)
			if strings.Contains(err.Error(), "synthetic-secret") || strings.Contains(err.Error(), "127.0.0.1") {
				t.Fatal("connection detail leaked")
			}
		})
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err := Open(canceled, c)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	for _, key := range []string{"PGHOST", "PGPORT", "PGUSER", "PGPASSWORD", "PGDATABASE", "PGSERVICE", "PGSERVICEFILE", "PGPASSFILE", "PGSSLMODE", "PGOPTIONS"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, "synthetic-canary")
			_, err := driverConfig(c)
			var typed *Error
			if !errors.As(err, &typed) || typed.Code != InvalidConfig {
				t.Fatal("ambient config not refused")
			}
			if strings.Contains(err.Error(), "synthetic-canary") {
				t.Fatal("ambient value leaked")
			}
		})
	}
	good, err := Open(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	good.Close()
}
func TestPGFoundationFixture(t *testing.T) {
	f, c, db := fixture(t)
	ctx := context.Background()
	if err := Migrate(ctx, c); err != nil {
		t.Fatal(err)
	}
	original := f.Config.Database
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	requireError(t, f.Reset(canceled))
	// An owned name without its exact marker may not be dropped/reset.
	var marker string
	if err := f.Admin.QueryRow("SELECT shobj_description(oid,'pg_database') FROM pg_database WHERE datname=$1", original).Scan(&marker); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Admin.Exec("COMMENT ON DATABASE \"" + original + "\" IS 'foreign-marker'"); err != nil {
		t.Fatal(err)
	}
	requireError(t, f.Reset(ctx))
	if _, err := f.Admin.Exec("COMMENT ON DATABASE \"" + original + "\" IS '" + marker + "'"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if err := f.Reset(ctx); err != nil {
		t.Fatal(err)
	}
	fresh, err := connect(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	var exists bool
	if err = fresh.QueryRow("SELECT to_regclass('public.coordination_schema') IS NOT NULL").Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("fixture reset retained schema")
	}
	for _, raw := range []string{"", "postgresql://user@127.0.0.1/postgres", "postgresql://user:pw@192.0.2.1:5432/postgres?sslmode=disable", "postgresql://user:pw@127.0.0.1:5432/postgres?sslmode=disable&servicefile=canary"} {
		_, err := pgtest.Parse(raw)
		requireError(t, err)
	}
	if got := pgtest.ScrubEnvironment([]string{"PGHOST=canary", "PGSERVICE=canary", "SAFE=keep"}); !reflect.DeepEqual(got, []string{"SAFE=keep"}) {
		t.Fatal("fixture environment retained PostgreSQL discovery")
	}
}

func TestPGFoundationBoundary(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "./...")
	cmd.Dir = ".."
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOTOOLCHAIN=local")
	raw, err := cmd.Output()
	if err != nil {
		t.Fatal("runtime dependency inventory failed")
	}
	for _, pkg := range strings.Fields(string(raw)) {
		if strings.Contains(pkg, "sqlite") || strings.HasPrefix(pkg, "modernc.org/") || strings.Contains(pkg, "mysql") || strings.HasPrefix(pkg, "net/http") {
			t.Fatalf("unexpected runtime package %s", pkg)
		}
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range file.Imports {
			v, _ := strconv.Unquote(imp.Path.Value)
			if v == "os/exec" || v == "net/http" {
				t.Fatalf("forbidden capability in %s", name)
			}
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && ast.IsExported(fn.Name.Name) {
				if fn.Recv != nil {
					recv := fn.Recv.List[0].Type
					if ptr, ok := recv.(*ast.StarExpr); ok {
						recv = ptr.X
					}
					if id, ok := recv.(*ast.Ident); ok && !ast.IsExported(id.Name) {
						continue
					}
				}

				for _, forbidden := range []string{"Execute", "Dispatch", "Admit", "Claim", "Outbox", "Activate"} {
					if strings.HasPrefix(fn.Name.Name, forbidden) {
						t.Fatalf("forbidden authority API %s", fn.Name.Name)
					}
				}
			}
		}
	}
	raw, err = os.ReadFile("../../.github/workflows/assay-workflow.yml")
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"workflow/**", "spec/coordination-store-v1.md", ".github/workflows/assay-workflow.yml"} {
		if strings.Count(string(raw), input) < 2 {
			t.Fatalf("CI input missing on an event: %s", input)
		}
	}
}
