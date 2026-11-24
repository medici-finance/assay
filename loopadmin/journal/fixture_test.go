package journal

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"net"
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

// fixturePrefix names every database and role the fixture creates;
// markerPrefix starts the comment that marks them as this fixture's own.
const (
	fixturePrefix = "loopadmin_journal_"
	markerPrefix  = "loopadmin-journal-fixture/"
)

// pgFixture owns the databases and roles it created on the disposable server.
// Each carries a fixed name prefix, a random suffix and this fixture's own
// marker comment, and only such an object is dropped at cleanup (Task 6 rules
// a to d, re-implemented from workflow/internal/pgtest, which loopadmin cannot
// import).
type pgFixture struct {
	t      testing.TB
	admin  *sql.DB
	base   Config
	marker string
	mu     sync.Mutex
	owned  map[string]bool
	roles  map[string]bool
}

func closeTestHandle(t testing.TB, handle io.Closer) {
	t.Helper()
	if err := handle.Close(); err != nil {
		t.Error("failed to close PostgreSQL test handle")
	}
}

// clearPGEnv scrubs every PG* variable from the test process, and so from
// every child it starts, for the rest of the test (rule d).
func clearPGEnv(t testing.TB) {
	for _, entry := range os.Environ() {
		if key, _, ok := strings.Cut(entry, "="); ok && strings.HasPrefix(key, "PG") {
			t.Setenv(key, "") // Restores the original value at cleanup.
			if err := os.Unsetenv(key); err != nil {
				t.Fatal("cannot scrub a PG* variable")
			}
		}
	}
}

// errDSN is the only refusal parseDSN returns: it never carries the DSN or
// any part of it (rule c).
var errDSN = errors.New("unusable test DSN: need postgres[ql]://user:password@<loopback IP>:<port>/postgres with at most sslmode=disable and connect_timeout=1..30")

// parseDSN accepts only a literal-loopback server's admin database, with a
// user, a password and a port, and no settings beyond sslmode=disable and a
// bounded connect_timeout (rule a).
func parseDSN(raw string) (Config, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.User == nil || u.Opaque != "" || u.Fragment != "" {
		return Config{}, errDSN
	}
	password, set := u.User.Password()
	if u.User.Username() == "" || !set || password == "" {
		return Config{}, errDSN
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return Config{}, errDSN
	}
	port, err := strconv.ParseUint(u.Port(), 10, 16)
	if err != nil || port == 0 {
		return Config{}, errDSN
	}
	if u.Path != "/postgres" {
		return Config{}, errDSN
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return Config{}, errDSN
	}
	timeout := 10 * time.Second
	for key, values := range query {
		if len(values) != 1 {
			return Config{}, errDSN
		}
		switch key {
		case "sslmode":
			if values[0] != "disable" {
				return Config{}, errDSN
			}
		case "connect_timeout":
			n, err := strconv.Atoi(values[0])
			if err != nil || n < 1 || n > 30 {
				return Config{}, errDSN
			}
			timeout = time.Duration(n) * time.Second
		default:
			return Config{}, errDSN
		}
	}
	return Config{
		Host: ip.String(), Port: uint16(port), User: u.User.Username(), Password: password,
		Database: "postgres", Timeout: timeout,
	}, nil
}

// mayDrop is rule b: the fixture drops a database only when it created it in
// this run, the name carries the fixture prefix, the database carries this
// fixture's own marker and the connected admin owns it. Anything else is
// retained.
func mayDrop(name string, created bool, marker, want, owner, self string) bool {
	return created && strings.HasPrefix(name, fixturePrefix) && want != "" && marker == want && owner != "" && owner == self
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
	f := &pgFixture{t: t, admin: admin, base: base, owned: map[string]bool{}, roles: map[string]bool{}}
	f.marker = markerPrefix + f.suffix()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		f.mu.Lock()
		names := make([]string, 0, len(f.owned))
		for name := range f.owned {
			names = append(names, name)
		}
		roles := make([]string, 0, len(f.roles))
		for name := range f.roles {
			roles = append(roles, name)
		}
		f.mu.Unlock()
		for _, name := range names {
			f.drop(ctx, name)
		}
		for _, name := range roles {
			f.dropRole(ctx, name)
		}
		closeTestHandle(t, admin)
	})
	return f
}

func (f *pgFixture) suffix() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		f.t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

// create makes a fresh database (optionally a copy of template), marks it as
// this fixture's own and returns its configuration.
func (f *pgFixture) create(ctx context.Context, template string) Config {
	f.t.Helper()
	name := fixturePrefix + f.suffix()
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
	// The marker is hex behind a constant prefix, so it is safe as a literal.
	if _, err := f.admin.ExecContext(ctx, "COMMENT ON DATABASE "+pgx.Identifier{name}.Sanitize()+" IS '"+f.marker+"'"); err != nil {
		f.t.Fatalf("mark fixture database: %v", err)
	}
	c := f.base
	c.Database = name
	return c
}

// drop removes a database this fixture created and marked, closing other
// sessions. A database whose marker or owner does not match is retained.
func (f *pgFixture) drop(ctx context.Context, name string) {
	f.t.Helper()
	f.mu.Lock()
	created := f.owned[name]
	f.mu.Unlock()
	var marker, owner, self sql.NullString
	err := f.admin.QueryRowContext(ctx, `SELECT shobj_description(d.oid, 'pg_database'), pg_get_userbyid(d.datdba), current_user
		FROM pg_database d WHERE d.datname = $1`, name).Scan(&marker, &owner, &self)
	if errors.Is(err, sql.ErrNoRows) {
		f.forget(name)
		return
	}
	if err != nil {
		f.t.Errorf("read fixture database marker: %v", err)
		return
	}
	if !mayDrop(name, created, marker.String, f.marker, owner.String, self.String) {
		f.t.Errorf("retaining a database whose fixture marker or owner does not match")
		return
	}
	if _, err := f.admin.ExecContext(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
		f.t.Errorf("drop fixture database: %v", err)
		return
	}
	f.forget(name)
}

func (f *pgFixture) forget(name string) {
	f.mu.Lock()
	delete(f.owned, name)
	f.mu.Unlock()
}

// runtimeLogin creates a marked login role that is a member of
// loopadmin_runtime only, and returns c connecting as it. The migration has
// already created the group role.
func (f *pgFixture) runtimeLogin(ctx context.Context, c Config) Config {
	f.t.Helper()
	name := fixturePrefix + "rt_" + f.suffix()
	password := f.suffix() + f.suffix()
	ident := pgx.Identifier{name}.Sanitize()
	// The password is hex, so it is safe as a literal; it never reaches an error.
	if _, err := f.admin.ExecContext(ctx, "CREATE ROLE "+ident+" LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD '"+password+"'"); err != nil {
		f.t.Fatal("create fixture runtime role failed")
	}
	f.mu.Lock()
	f.roles[name] = true
	f.mu.Unlock()
	for _, stmt := range []string{
		"COMMENT ON ROLE " + ident + " IS '" + f.marker + "'",
		"GRANT loopadmin_runtime TO " + ident,
	} {
		if _, err := f.admin.ExecContext(ctx, stmt); err != nil {
			f.t.Fatalf("prepare fixture runtime role: %v", err)
		}
	}
	c.User, c.Password = name, password
	return c
}

// dropRole removes a login role this fixture created and marked.
func (f *pgFixture) dropRole(ctx context.Context, name string) {
	f.t.Helper()
	var marker, self sql.NullString
	err := f.admin.QueryRowContext(ctx, "SELECT shobj_description(r.oid, 'pg_authid'), current_user FROM pg_roles r WHERE r.rolname = $1", name).Scan(&marker, &self)
	if errors.Is(err, sql.ErrNoRows) {
		return
	}
	if err != nil {
		f.t.Errorf("read fixture role marker: %v", err)
		return
	}
	f.mu.Lock()
	created := f.roles[name]
	f.mu.Unlock()
	if !mayDrop(name, created, marker.String, f.marker, self.String, self.String) {
		f.t.Errorf("retaining a role whose fixture marker does not match")
		return
	}
	if _, err := f.admin.ExecContext(ctx, "DROP ROLE IF EXISTS "+pgx.Identifier{name}.Sanitize()); err != nil {
		f.t.Errorf("drop fixture role: %v", err)
		return
	}
	f.mu.Lock()
	delete(f.roles, name)
	f.mu.Unlock()
}

// operator is a fake protected operator-state source: the test plays the
// trusted operator, rotates its current record and reports restores.
type operator struct {
	mu       sync.Mutex
	cur      map[string]Incarnation
	restored map[string][]RestoredControl
	err      error
}

func (o *operator) CurrentIncarnation(_ context.Context, cell string) (Incarnation, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.err != nil {
		return Incarnation{}, o.err
	}
	return o.cur[cell], nil
}

func (o *operator) RestoredControls(_ context.Context, cell string) ([]RestoredControl, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.err != nil {
		return nil, o.err
	}
	return append([]RestoredControl(nil), o.restored[cell]...), nil
}

func (o *operator) rotate(cell, id, key string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.cur == nil {
		o.cur = map[string]Incarnation{}
	}
	o.cur[cell] = Incarnation{Schema: IncarnationSchema, CellID: cell, ID: id, KeyRef: key}
}

// restore records, as the operator's protected state outside the database,
// that the cell's journal was restored from snapshot carrying the cell's
// current incarnation.
func (o *operator) restore(cell, snapshot string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.restored == nil {
		o.restored = map[string][]RestoredControl{}
	}
	cur := o.cur[cell]
	o.restored[cell] = append(o.restored[cell], RestoredControl{SnapshotRef: snapshot, IncarnationID: cur.ID, KeyRef: cur.KeyRef})
}

// TestJournalFixtureDisposability covers Verify row 7 (Task 6) with no DSN
// and no server: the fixture's DSN, drop, redaction and environment rules.
// regression: #5492
func TestJournalFixtureDisposability(t *testing.T) {
	const secret = "fixture-secret-pw"
	ok := "postgresql://admin:" + secret + "@127.0.0.1:5432/postgres?sslmode=disable&connect_timeout=5"
	c, err := parseDSN(ok)
	if err != nil || c.Host != "127.0.0.1" || c.Port != 5432 || c.Database != "postgres" || c.Timeout != 5*time.Second {
		t.Fatalf("positive control: a loopback admin DSN must parse, got %v", err)
	}
	if _, err = parseDSN("postgres://admin:" + secret + "@[::1]:5432/postgres"); err != nil {
		t.Fatalf("positive control: an IPv6 loopback DSN must parse, got %v", err)
	}
	for name, raw := range map[string]string{
		"non-loopback host":  "postgresql://admin:" + secret + "@10.0.0.5:5432/postgres",
		"host name":          "postgresql://admin:" + secret + "@localhost:5432/postgres",
		"other database":     "postgresql://admin:" + secret + "@127.0.0.1:5432/celldb",
		"no password":        "postgresql://admin@127.0.0.1:5432/postgres",
		"empty password":     "postgresql://admin:@127.0.0.1:5432/postgres",
		"no user":            "postgresql://:" + secret + "@127.0.0.1:5432/postgres",
		"no port":            "postgresql://admin:" + secret + "@127.0.0.1/postgres",
		"extra setting":      "postgresql://admin:" + secret + "@127.0.0.1:5432/postgres?target_session_attrs=any",
		"sslmode other":      "postgresql://admin:" + secret + "@127.0.0.1:5432/postgres?sslmode=prefer",
		"unbounded timeout":  "postgresql://admin:" + secret + "@127.0.0.1:5432/postgres?connect_timeout=0",
		"repeated setting":   "postgresql://admin:" + secret + "@127.0.0.1:5432/postgres?sslmode=disable&sslmode=disable",
		"other scheme":       "mysql://admin:" + secret + "@127.0.0.1:5432/postgres",
		"unix socket by key": "postgresql://admin:" + secret + "@127.0.0.1:5432/postgres?host=/tmp",
	} {
		_, err := parseDSN(raw)
		if err == nil {
			t.Errorf("%s: must be refused", name)
			continue
		}
		if strings.Contains(err.Error(), secret) {
			t.Errorf("%s: the refusal must omit the password", name)
		}
	}
	if s := c.String(); strings.Contains(s, secret) {
		t.Error("the parsed configuration must not print its password")
	}

	// Rule b: only a database this run created, prefixed, carrying this
	// fixture's own marker and owned by the connected admin is dropped.
	name, mark := fixturePrefix+"0123456789abcdef", markerPrefix+"0123456789abcdef"
	if !mayDrop(name, true, mark, mark, "admin", "admin") {
		t.Fatal("positive control: an owned, marked fixture database must be dropped")
	}
	for why, keep := range map[string]bool{
		"not created here": mayDrop(name, false, mark, mark, "admin", "admin"),
		"foreign prefix":   mayDrop("celldb", true, mark, mark, "admin", "admin"),
		"missing marker":   mayDrop(name, true, "", mark, "admin", "admin"),
		"foreign marker":   mayDrop(name, true, markerPrefix+"ffff", mark, "admin", "admin"),
		"no own marker":    mayDrop(name, true, "", "", "admin", "admin"),
		"foreign owner":    mayDrop(name, true, mark, mark, "someone", "admin"),
	} {
		if keep {
			t.Errorf("%s: the database must be retained", why)
		}
	}

	// Rule d: PG* variables are scrubbed from the test and its children.
	t.Setenv("PGPASSWORD", "ambient")
	t.Setenv("PGHOST", "elsewhere")
	clearPGEnv(t)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "PG") {
			t.Errorf("a PG* variable survived the scrub: %s", strings.SplitN(entry, "=", 2)[0])
		}
	}
}
