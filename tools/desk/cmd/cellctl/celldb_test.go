package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestCellDB covers the cell database pointer (docs/postgres.md): the two cell.env levers, their
// composition into every launch a host cell makes, and the `check` rows. Nothing here starts a
// database server or contacts one; the values are example strings and the credential file is an
// empty temporary file (no password, real or example, is written).
func TestCellDB(t *testing.T) {
	const dsn = "host=127.0.0.1 port=5432 dbname=cell_example_cell user=cell_example_cell sslmode=disable"

	// regression: both keys are cell.env levers, so `cellctl set` takes them without --force.
	t.Run("keys are levers", func(t *testing.T) {
		for _, k := range []string{"CELL_DB_DSN", "CELL_DB_PASSFILE"} {
			if !knownCellEnvKey(k) {
				t.Errorf("%s is not a cell.env lever", k)
			}
		}
	})

	// regression: with CELL_DB_DSN set, the host launch, the scrubbed launch and the codex arm
	// all carry ASSAY_CELL_DB_DSN and PGPASSFILE.
	t.Run("set keys compose into host scrubbed and codex launches", func(t *testing.T) {
		passfile := filepath.Join(t.TempDir(), "example-cell.pgpass")

		host := &Cell{Env: envWith(map[string]string{"CELL_DB_DSN": dsn, "CELL_DB_PASSFILE": passfile})}
		env := host.cellDBEnv([]string{"PATH=/usr/bin"})
		if got := envValue(env, envCellDBDSN); got != dsn {
			t.Errorf("host: %s = %q, want %q", envCellDBDSN, got, dsn)
		}
		if got := envValue(env, envCellDBPassfile); got != passfile {
			t.Errorf("host: %s = %q, want %q", envCellDBPassfile, got, passfile)
		}
		requireLaunchComposesCellDB(t)

		sc := scrubbedFixture(t)
		sc.Env.Put("CELL_DB_DSN", dsn)
		sc.Env.Put("CELL_DB_PASSFILE", passfile)
		composed := sc.scrubbedComposeEnv("worker-desk", "claude", "s")
		if got := envValue(composed.Pairs, envCellDBDSN); got != dsn {
			t.Errorf("scrubbed: %s = %q, want %q", envCellDBDSN, got, dsn)
		}
		if got := envValue(composed.Pairs, envCellDBPassfile); got != passfile {
			t.Errorf("scrubbed: %s = %q, want %q", envCellDBPassfile, got, passfile)
		}

		cx := codexEnvironmentCell(t)
		cx.Env.Put("CELL_DB_DSN", dsn)
		cx.Env.Put("CELL_DB_PASSFILE", passfile)
		values, err := cx.codexCommandEnvironment(cx.cellDBEnv(envSet(nil, "HOME", cx.Env.Get("HOME"))))
		if err != nil {
			t.Fatal(err)
		}
		if values[envCellDBDSN] != dsn || values[envCellDBPassfile] != passfile {
			t.Errorf("codex: forwarded %s=%q %s=%q, want the DSN and the passfile path",
				envCellDBDSN, values[envCellDBDSN], envCellDBPassfile, values[envCellDBPassfile])
		}

		// The dry-run plan of a real cell prints one line each.
		f := newRepairFixture(t)
		f.writeCellEnv(t, "CELL_DB_DSN="+dsn+"\nCELL_DB_PASSFILE="+passfile+"\n")
		r := f.run(t, []string{"DRY_RUN=1"}, "desk", "example", "worker-desk")
		if r.code != 0 {
			t.Fatalf("dry-run: exit %d: %s%s", r.code, r.stdout, r.stderr)
		}
		for _, want := range []string{
			"[dry-run] env " + envCellDBDSN + "=" + dsn + " ",
			"[dry-run] env " + envCellDBPassfile + "=" + passfile + " ",
		} {
			if !strings.Contains(r.stdout, want) {
				t.Errorf("dry-run lacks %q:\n%s", want, r.stdout)
			}
		}
	})

	// regression: a cell with no database composes neither variable anywhere.
	t.Run("unset keys compose neither", func(t *testing.T) {
		host := &Cell{Env: envWith(map[string]string{"CELL_DB_PASSFILE": "/example/ignored.pgpass"})}
		env := host.cellDBEnv([]string{"PATH=/usr/bin"})
		for _, k := range []string{envCellDBDSN, envCellDBPassfile} {
			if _, ok := envLookup(env, k); ok {
				t.Errorf("host: %s composed with CELL_DB_DSN unset", k)
			}
		}

		sc := scrubbedFixture(t)
		composed := sc.scrubbedComposeEnv("worker-desk", "claude", "s")
		for _, k := range []string{envCellDBDSN, envCellDBPassfile} {
			if _, ok := envLookup(composed.Pairs, k); ok {
				t.Errorf("scrubbed: %s composed with CELL_DB_DSN unset", k)
			}
		}

		cx := codexEnvironmentCell(t)
		values, err := cx.codexCommandEnvironment(cx.cellDBEnv(envSet(nil, "HOME", cx.Env.Get("HOME"))))
		if err != nil {
			t.Fatal(err)
		}
		for _, k := range []string{envCellDBDSN, envCellDBPassfile} {
			if _, ok := values[k]; ok {
				t.Errorf("codex: %s forwarded with CELL_DB_DSN unset", k)
			}
		}

		f := newRepairFixture(t)
		r := f.run(t, []string{"DRY_RUN=1"}, "desk", "example", "worker-desk")
		if r.code != 0 {
			t.Fatalf("dry-run: exit %d: %s%s", r.code, r.stdout, r.stderr)
		}
		if strings.Contains(r.stdout, envCellDBDSN) || strings.Contains(r.stdout, envCellDBPassfile) {
			t.Errorf("dry-run names a database variable with CELL_DB_DSN unset:\n%s", r.stdout)
		}
	})

	// regression: F3 (review) — a launch from inside a database cell's window carries that cell's
	// ASSAY_CELL_DB_DSN and PGPASSFILE; a cell that does not compose a key must not inherit it.
	t.Run("inherited keys are removed not passed through", func(t *testing.T) {
		inherited := []string{"PATH=/usr/bin", envCellDBDSN + "=host=127.0.0.1 dbname=cell_other", envCellDBPassfile + "=/example/other.pgpass"}

		none := &Cell{Env: envWith(map[string]string{})}
		env := none.cellDBEnv(inherited)
		for _, k := range []string{envCellDBDSN, envCellDBPassfile} {
			if v, ok := envLookup(env, k); ok {
				t.Errorf("unset cell inherits %s=%q", k, v)
			}
		}
		if envValue(env, "PATH") != "/usr/bin" {
			t.Errorf("unrelated keys must survive, PATH = %q", envValue(env, "PATH"))
		}

		noPassfile := &Cell{Env: envWith(map[string]string{"CELL_DB_DSN": dsn})}
		env = noPassfile.cellDBEnv(inherited)
		if got := envValue(env, envCellDBDSN); got != dsn {
			t.Errorf("%s = %q, want the cell's own DSN", envCellDBDSN, got)
		}
		if v, ok := envLookup(env, envCellDBPassfile); ok {
			t.Errorf("cell with no CELL_DB_PASSFILE inherits %s=%q", envCellDBPassfile, v)
		}
	})

	// regression: A1 (review) — a password-bearing DSN is withheld at every compose site and the
	// launch is refused, and the value is never printed.
	t.Run("password DSN is withheld and refused", func(t *testing.T) {
		const bad = "host=127.0.0.1 dbname='password=example-not-a-secret'"
		pf := writePassfile(t, 0o600)
		c := &Cell{Env: envWith(map[string]string{"CELL_DB_DSN": bad, "CELL_DB_PASSFILE": pf})}
		env := c.cellDBEnv([]string{"PATH=/usr/bin"})
		for _, k := range []string{envCellDBDSN, envCellDBPassfile} {
			if _, ok := envLookup(env, k); ok {
				t.Errorf("host: %s composed from a password-bearing DSN", k)
			}
		}
		sc := scrubbedFixture(t)
		sc.Env.Put("CELL_DB_DSN", bad)
		sc.Env.Put("CELL_DB_PASSFILE", pf)
		composed := sc.scrubbedComposeEnv("worker-desk", "claude", "s")
		for _, k := range []string{envCellDBDSN, envCellDBPassfile} {
			if _, ok := envLookup(composed.Pairs, k); ok {
				t.Errorf("scrubbed: %s composed from a password-bearing DSN", k)
			}
		}
		if c.cellDBRefusal() == "" {
			t.Error("cellDBRefusal is empty for a password-bearing DSN")
		}
		if strings.Contains(c.cellDBRefusal(), "example-not-a-secret") {
			t.Error("the refusal printed the password")
		}
		ok := &Cell{Env: envWith(map[string]string{"CELL_DB_DSN": dsn})}
		if msg := ok.cellDBRefusal(); msg != "" {
			t.Errorf("a clean DSN is refused: %s", msg)
		}

		f := newRepairFixture(t)
		f.writeCellEnv(t, "CELL_DB_DSN="+bad+"\nCELL_DB_PASSFILE="+pf+"\n")
		r := f.run(t, []string{"DRY_RUN=1"}, "desk", "example", "worker-desk")
		if r.code == 0 {
			t.Fatalf("dry-run with a password-bearing DSN exited 0:\n%s%s", r.stdout, r.stderr)
		}
		if strings.Contains(r.stdout+r.stderr, "example-not-a-secret") {
			t.Errorf("desk printed the password:\n%s%s", r.stdout, r.stderr)
		}
	})

	// regression: a DSN carrying a password fails `check`, in either libpq form, and the row
	// never prints the value.
	t.Run("DSN carrying a password fails check", func(t *testing.T) {
		passfile := writePassfile(t, 0o600)
		for _, bad := range []string{
			dsn + " password=example-not-a-secret",
			"host=127.0.0.1 PASSWORD = example-not-a-secret dbname=cell_example_cell",
			"postgresql://cell_example_cell:example-not-a-secret@127.0.0.1:5432/cell_example_cell",
			"postgres://cell_example_cell:example-not-a-secret@127.0.0.1:5432,127.0.0.2:5432/cell_example_cell",
			"postgresql://cell_example_cell@127.0.0.1/cell_example_cell?sslmode=disable&password=example-not-a-secret",
			// libpq expands a quoted dbname value, or a URI given as one, as a nested conninfo.
			"host=127.0.0.1 dbname='password=example-not-a-secret'",
			`host=127.0.0.1 dbname="password = example-not-a-secret"`,
			"host=127.0.0.1 dbname=password=example-not-a-secret",
			"host=127.0.0.1 dbname=postgresql://cell_example_cell:example-not-a-secret@127.0.0.1/cell_example_cell",
		} {
			ok, out := runCellDBCheck(t, "house", bad, passfile)
			if ok || !strings.Contains(out, "MISS  cell database: CELL_DB_DSN carries a password") {
				t.Errorf("DSN %q: want a MISS row, got ok=%v:\n%s", bad, ok, out)
			}
			if strings.Contains(out, "example-not-a-secret") {
				t.Errorf("the check row printed the password:\n%s", out)
			}
		}
		for _, good := range []string{
			dsn,
			"postgresql://cell_example_cell@127.0.0.1:5432/cell_example_cell?sslmode=disable",
			"postgresql://127.0.0.1/cell_example_cell?passfile=/example/x.pgpass",
		} {
			if ok, out := runCellDBCheck(t, "house", good, passfile); !ok {
				t.Errorf("DSN %q carries no password and should pass:\n%s", good, out)
			}
		}
	})

	// regression: the credential file must be set, absolute, readable, and 0600 or 0400.
	t.Run("passfile at mode 0644 fails check", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("POSIX mode bits; the mode rows behave as the PEM mode rows do on Windows")
		}
		if ok, out := runCellDBCheck(t, "house", dsn, writePassfile(t, 0o644)); ok || !strings.Contains(out, "MISS  cell database: CELL_DB_PASSFILE mode 0600 or 0400") {
			t.Errorf("a 0644 passfile should MISS, got ok=%v:\n%s", ok, out)
		}
		for _, mode := range []os.FileMode{0o600, 0o400} {
			if ok, out := runCellDBCheck(t, "scrubbed", dsn, writePassfile(t, mode)); !ok {
				t.Errorf("a %#o passfile should pass:\n%s", mode, out)
			}
		}
		for name, p := range map[string]string{"unset": "", "relative": "example.pgpass", "missing": filepath.Join(t.TempDir(), "absent.pgpass")} {
			if ok, out := runCellDBCheck(t, "house", dsn, p); ok {
				t.Errorf("an %s passfile should MISS:\n%s", name, out)
			}
		}
	})

	// regression: a container cell cannot reach the database, and says so rather than leaving
	// the key silently unused.
	t.Run("container cell fails check naming the gap", func(t *testing.T) {
		ok, out := runCellDBCheck(t, "container", dsn, writePassfile(t, 0o600))
		if ok || !strings.Contains(out, "MISS  cell database: container cells cannot reach the cell database yet") {
			t.Errorf("a container cell with CELL_DB_DSN should MISS naming the gap, got ok=%v:\n%s", ok, out)
		}
	})

	t.Run("unset prints no check row", func(t *testing.T) {
		if ok, out := runCellDBCheck(t, "house", "", ""); !ok || out != "" {
			t.Errorf("CELL_DB_DSN unset: want no row and no failure, got ok=%v %q", ok, out)
		}
	})
}

// runCellDBCheck runs the database check rows for one cell of the given kind and returns
// whether they passed and what they printed.
func runCellDBCheck(t *testing.T, kind, dsn, passfile string) (bool, string) {
	t.Helper()
	kv := map[string]string{}
	if dsn != "" {
		kv["CELL_DB_DSN"] = dsn
	}
	if passfile != "" {
		kv["CELL_DB_PASSFILE"] = passfile
	}
	c := &Cell{Kind: kind, Env: envWith(kv)}
	k := &checker{ok: true}
	out := captureStdout(t, func() { c.checkCellDB(k) })
	return k.ok, out
}

// writePassfile creates an EMPTY pgpass file at the given mode: the rows read its mode and
// readability, never its content.
func writePassfile(t *testing.T, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "example-cell.pgpass")
	if err := os.WriteFile(p, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, mode); err != nil {
		t.Fatal(err)
	}
	return p
}

// requireLaunchComposesCellDB: the live host launch cannot run in a test (it fetches and execs a
// harness), so its source is read: deskLaunch must call cellDBEnv on the environment it hands
// the harness.
func requireLaunchComposesCellDB(t *testing.T) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "launch.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range file.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "deskLaunch" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "cellDBEnv" {
					found = true
				}
			}
			return true
		})
	}
	if !found {
		t.Error("host: deskLaunch in launch.go does not compose the cell database variables (no cellDBEnv call)")
	}
}
