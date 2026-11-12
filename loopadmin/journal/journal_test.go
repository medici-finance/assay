package journal

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

const testCell = "cell-a"

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func rows(t *testing.T, ctx context.Context, c Config, query string, args ...any) int {
	t.Helper()
	db, err := connect(ctx, c)
	must(t, err)
	defer db.Close()
	var n int
	must(t, db.QueryRowContext(ctx, query, args...).Scan(&n))
	return n
}

func run(t *testing.T, ctx context.Context, c Config, stmt string, args ...any) {
	t.Helper()
	db, err := connect(ctx, c)
	must(t, err)
	defer db.Close()
	if _, err = db.ExecContext(ctx, stmt, args...); err != nil {
		t.Fatal(err)
	}
}

func intent(inv, req string) Intent {
	return Intent{Key: Key{Namespace: "desk/worker", Invocation: inv, Generation: 1}, RequestID: req, Profile: "worker", Digest: "sha256:" + req}
}

// TestJournalSharedStoreRestart covers Verify row 6: launch intent and receipt
// commit in one transaction before the child starts; a restart with the
// database intact reconciles once; after the journal database is dropped an
// unknown launch holds instead of relaunching.
// regression: #5492
func TestJournalSharedStoreRestart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pg := newPG(t)
	cfg := pg.create(ctx, "")
	must(t, Migrate(ctx, cfg))
	must(t, Migrate(ctx, cfg)) // idempotent: nothing pending
	op := &operator{}
	op.rotate(testCell, "inc-1", "key-1")
	must(t, Activate(ctx, cfg, testCell, op, StopConfirmation{Receipt: "stop-1"}))

	launches := 0
	start := func(d Decision) {
		if d.Launch {
			launches++
		}
	}
	j, err := Open(ctx, cfg, testCell, op)
	must(t, err)
	in := intent("inv-1", "req-1")
	d, err := j.Intend(ctx, in, nil)
	must(t, err)
	if !d.Launch || d.State != Intended || d.Receipt.ID == "" {
		t.Fatalf("a new intent must commit and admit one launch, got %+v", d)
	}
	// Both rows are committed, visible to another session, before the child starts.
	if n := rows(t, ctx, cfg, "SELECT count(*) FROM loopadmin.invocation i JOIN loopadmin.receipt r USING (caller_namespace, invocation_id, owner_generation) WHERE r.receipt_id = $1 AND i.state = 'intended'", d.Receipt.ID); n != 1 {
		t.Fatalf("intent and receipt must both be committed before launch, found %d", n)
	}
	start(d)
	must(t, j.MarkLaunched(ctx, in.Key))

	// Duplicate input cannot launch twice, with or without the held receipt.
	for _, prior := range []*Receipt{nil, &d.Receipt} {
		again, err := j.Intend(ctx, in, prior)
		must(t, err)
		if again.Launch || again.Receipt.ID != d.Receipt.ID || again.State != Launched {
			t.Fatalf("a duplicate intent must return the existing record, got %+v", again)
		}
		start(again)
	}
	other := in
	other.Digest = "sha256:other"
	if _, err = j.Intend(ctx, other, nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("a reused request ID with other content must conflict, got %v", err)
	}

	// One transaction: a receipt that cannot be written leaves no intent.
	run(t, ctx, cfg, `CREATE FUNCTION loopadmin.refuse_receipt() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN IF NEW.invocation_id = 'inv-torn' THEN RAISE EXCEPTION 'planted receipt failure'; END IF; RETURN NEW; END $$`)
	run(t, ctx, cfg, "CREATE TRIGGER refuse_receipt BEFORE INSERT ON loopadmin.receipt FOR EACH ROW EXECUTE FUNCTION loopadmin.refuse_receipt()")
	torn, err := j.Intend(ctx, intent("inv-torn", "req-torn"), nil)
	if err == nil || torn.Launch {
		t.Fatalf("a failed receipt write must refuse the launch, got %+v, %v", torn, err)
	}
	if n := rows(t, ctx, cfg, "SELECT count(*) FROM loopadmin.invocation WHERE invocation_id = 'inv-torn'"); n != 0 {
		t.Fatalf("a failed receipt write must roll the intent back, found %d rows", n)
	}

	// Restart with the database intact: the open invocation reconciles once.
	must(t, j.Close())
	j, err = Open(ctx, cfg, testCell, op)
	must(t, err)
	pending, err := j.Pending(ctx)
	must(t, err)
	if len(pending) != 1 || pending[0].Key != in.Key || pending[0].State != Launched {
		t.Fatalf("restart must find the one launched invocation, got %+v", pending)
	}
	settled, err := j.Reconcile(ctx, in.Key, Completed)
	must(t, err)
	if !settled {
		t.Fatal("the first reconciliation must settle the invocation")
	}
	must(t, j.Close())
	j, err = Open(ctx, cfg, testCell, op)
	must(t, err)
	pending, err = j.Pending(ctx)
	must(t, err)
	if len(pending) != 0 {
		t.Fatalf("a second restart must find nothing open, got %+v", pending)
	}
	if settled, err = j.Reconcile(ctx, in.Key, Completed); err != nil || settled {
		t.Fatalf("a second reconciliation must settle nothing, got %v, %v", settled, err)
	}
	if again, err := j.Intend(ctx, in, &d.Receipt); err != nil || again.Launch || again.State != Completed {
		t.Fatalf("a settled invocation must not relaunch, got %+v, %v", again, err)
	}
	must(t, j.Close())

	// The journal database is lost and rebuilt empty.
	pg.drop(ctx, cfg.Database)
	if _, err = Open(ctx, cfg, testCell, op); err == nil {
		t.Fatal("an unreachable journal database must refuse, never fall back")
	}
	cfg = pg.create(ctx, "")
	if _, err = Open(ctx, cfg, testCell, op); err == nil {
		t.Fatal("a database without the journal schema must refuse")
	}
	must(t, Migrate(ctx, cfg))
	j, err = Open(ctx, cfg, testCell, op)
	must(t, err)
	defer j.Close()
	if _, err = j.Intend(ctx, in, &d.Receipt); !errors.Is(err, ErrPaused) {
		t.Fatalf("a rebuilt journal without an activation record must start paused, got %v", err)
	}
	op.rotate(testCell, "inc-2", "key-2")
	must(t, Activate(ctx, cfg, testCell, op, StopConfirmation{Receipt: "stop-2"}))
	held, err := j.Intend(ctx, in, &d.Receipt)
	must(t, err)
	start(held)
	if held.Launch || held.State != Unknown || held.Receipt != d.Receipt {
		t.Fatalf("a receipt the journal lost must hold as unknown, not relaunch, got %+v", held)
	}
	for _, prior := range []*Receipt{nil, &d.Receipt} {
		again, err := j.Intend(ctx, in, prior)
		must(t, err)
		start(again)
		if again.Launch || again.State != Unknown {
			t.Fatalf("a held unknown launch must keep holding, got %+v", again)
		}
	}
	pending, err = j.Pending(ctx)
	must(t, err)
	if len(pending) != 1 || pending[0].State != Unknown {
		t.Fatalf("the held launch must stay open for reconciliation, got %+v", pending)
	}
	if launches != 1 {
		t.Fatalf("exactly one child may start across restart and database loss, started %d", launches)
	}
}

// TestJournalIncarnationPaused covers Verify row 7: a journal restored from
// backup under a mismatched or missing operator incarnation starts paused and
// launches no child until rotation and a confirmed group stop.
// regression: #5492
func TestJournalIncarnationPaused(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pg := newPG(t)
	cfg := pg.create(ctx, "")
	must(t, Migrate(ctx, cfg))
	op := &operator{}
	op.rotate(testCell, "inc-a", "key-a")
	if err := Activate(ctx, cfg, testCell, op, StopConfirmation{}); !errors.Is(err, ErrStopUnconfirmed) {
		t.Fatalf("activation without a stop confirmation must be refused, got %v", err)
	}
	must(t, Activate(ctx, cfg, testCell, op, StopConfirmation{Receipt: "stop-a"}))
	j, err := Open(ctx, cfg, testCell, op)
	must(t, err)
	if d, err := j.Intend(ctx, intent("inv-1", "req-1"), nil); err != nil || !d.Launch {
		t.Fatalf("positive control: the activated incarnation must launch, got %+v, %v", d, err)
	}
	must(t, j.Close())

	// Backups: database copies taken now, then restored under new operator state.
	restored := pg.create(ctx, cfg.Database)
	missing := pg.create(ctx, cfg.Database)
	run(t, ctx, missing, "DELETE FROM loopadmin.activation")

	launches := 0
	try := func(j *Journal, inv string) error {
		d, err := j.Intend(ctx, intent(inv, "req-"+inv), nil)
		if d.Launch {
			launches++
		}
		return err
	}

	// Mismatch: the operator rotated after the backup.
	op.rotate(testCell, "inc-b", "key-b")
	jr, err := Open(ctx, restored, testCell, op)
	must(t, err)
	defer jr.Close()
	if err = jr.Gate(ctx); !errors.Is(err, ErrPaused) {
		t.Fatalf("a restored activation naming another incarnation must start paused, got %v", err)
	}
	if err = try(jr, "inv-2"); !errors.Is(err, ErrPaused) {
		t.Fatalf("a paused journal must refuse the launch, got %v", err)
	}
	if n := rows(t, ctx, restored, "SELECT count(*) FROM loopadmin.invocation WHERE invocation_id = 'inv-2'"); n != 0 {
		t.Fatalf("a refused launch must record no intent, found %d", n)
	}
	if n := rows(t, ctx, restored, "SELECT count(*) FROM loopadmin.activation WHERE paused"); n != 1 {
		t.Fatal("a mismatch must pause the activation record durably")
	}
	// The pause outlives the mismatch: presenting the restored identity again
	// does not reopen the journal.
	op.rotate(testCell, "inc-a", "key-a")
	if err = try(jr, "inv-2"); !errors.Is(err, ErrPaused) {
		t.Fatalf("a paused record must stay paused until the operator activates, got %v", err)
	}
	// Activation needs rotation away from the restored identity and key.
	if err = Activate(ctx, restored, testCell, op, StopConfirmation{Receipt: "stop-x"}); !errors.Is(err, ErrRotationRequired) {
		t.Fatalf("re-activating the restored incarnation must be refused, got %v", err)
	}
	op.rotate(testCell, "inc-c", "key-a")
	if err = Activate(ctx, restored, testCell, op, StopConfirmation{Receipt: "stop-x"}); !errors.Is(err, ErrRotationRequired) {
		t.Fatalf("a kept key reference must be refused, got %v", err)
	}
	op.rotate(testCell, "inc-b", "key-b")
	if err = Activate(ctx, restored, testCell, op, StopConfirmation{}); !errors.Is(err, ErrStopUnconfirmed) {
		t.Fatalf("rotation without a confirmed stop must not reopen, got %v", err)
	}
	if err = try(jr, "inv-2"); !errors.Is(err, ErrPaused) {
		t.Fatalf("still paused before the stop is confirmed, got %v", err)
	}
	must(t, Activate(ctx, restored, testCell, op, StopConfirmation{Receipt: "stop-b"}))
	if err = try(jr, "inv-2"); err != nil || launches != 1 {
		t.Fatalf("rotation plus a confirmed stop must reopen the journal, got %v, %d launches", err, launches)
	}

	// Missing activation record.
	jm, err := Open(ctx, missing, testCell, op)
	must(t, err)
	defer jm.Close()
	if err = try(jm, "inv-3"); !errors.Is(err, ErrPaused) {
		t.Fatalf("a restored journal without an activation record must start paused, got %v", err)
	}
	must(t, Activate(ctx, missing, testCell, op, StopConfirmation{Receipt: "stop-b"}))
	if err = try(jm, "inv-3"); err != nil || launches != 2 {
		t.Fatalf("activation must reopen the journal, got %v, %d launches", err, launches)
	}

	// An unreadable or unusable current incarnation holds admission.
	op.err = errors.New("operator state unavailable")
	if err = try(jm, "inv-4"); !errors.Is(err, ErrPaused) {
		t.Fatalf("an unreadable incarnation must hold, got %v", err)
	}
	op.err = nil
	op.mu.Lock()
	cur := op.cur[testCell]
	cur.Schema = "operator-incarnation/v0"
	op.cur[testCell] = cur
	op.mu.Unlock()
	if err = try(jm, "inv-4"); !errors.Is(err, ErrPaused) {
		t.Fatalf("an unsupported incarnation schema must hold, got %v", err)
	}
	if launches != 2 {
		t.Fatalf("paused journals must start no child, started %d", launches)
	}

	// A pause never lands on a newer activation (finding F-pause-race, both
	// orders). Each case runs the production steps of a runtime gate by hand:
	// the operator snapshot, the gate transaction, then the settling of its
	// refusal, with the operator's Activate committing at a chosen point.
	setup := func() (*Journal, Config, *operator) {
		c := pg.create(ctx, "")
		must(t, Migrate(ctx, c))
		o := &operator{}
		o.rotate(testCell, "inc-a", "key-a")
		must(t, Activate(ctx, c, testCell, o, StopConfirmation{Receipt: "stop-a"}))
		jj, err := Open(ctx, c, testCell, o)
		must(t, err)
		t.Cleanup(func() { jj.Close() })
		return jj, c, o
	}
	gateSeen := func(jj *Journal, snap Incarnation) *seenRecord {
		t.Helper()
		tx, err := jj.db.BeginTx(ctx, nil)
		must(t, err)
		seen, gerr := gate(ctx, tx, testCell, snap)
		tx.Rollback()
		if seen == nil || !errors.Is(gerr, ErrPaused) {
			t.Fatalf("the gate must report the record it saw, got %v, %v", seen, gerr)
		}
		return seen
	}
	settle := func(jj *Journal, o *operator, seen *seenRecord) {
		t.Helper()
		if err := refuse(ctx, jj.db, o, testCell, seen, ErrPaused); !errors.Is(err, ErrPaused) {
			t.Fatalf("the refused attempt itself stays refused, got %v", err)
		}
	}
	launchOK := func(jj *Journal, inv string) {
		t.Helper()
		if d, err := jj.Intend(ctx, intent(inv, "req-"+inv), nil); err != nil || !d.Launch {
			t.Fatalf("a pause from a stale gate must not wedge the new activation, got %+v, %v", d, err)
		}
	}
	snapshot := func(o *operator) Incarnation {
		t.Helper()
		cur, err := current(ctx, o, testCell)
		must(t, err)
		return cur
	}

	// Positive control: a record the source has moved past is paused.
	j0, c0, o0 := setup()
	o0.rotate(testCell, "inc-b", "key-b")
	settle(j0, o0, gateSeen(j0, snapshot(o0)))
	if n := rows(t, ctx, c0, "SELECT count(*) FROM loopadmin.activation WHERE paused"); n != 1 {
		t.Fatal("a record the operator source moved past must be paused durably")
	}

	// Order 1: the gate sees the old record, Activate commits, then the pause
	// would run.
	j1, c1, o1 := setup()
	o1.rotate(testCell, "inc-b", "key-b")
	seen := gateSeen(j1, snapshot(o1))
	must(t, Activate(ctx, c1, testCell, o1, StopConfirmation{Receipt: "stop-b"}))
	settle(j1, o1, seen)
	launchOK(j1, "inv-5")

	// Order 2, the reverse: the snapshot is read first, Activate commits, and
	// the gate then reads the NEW record, which differs from the older
	// snapshot. The record the gate saw is the newer one.
	j2, c2, o2 := setup()
	snap := snapshot(o2)
	o2.rotate(testCell, "inc-b", "key-b")
	must(t, Activate(ctx, c2, testCell, o2, StopConfirmation{Receipt: "stop-b"}))
	seen = gateSeen(j2, snap)
	if seen.id != "inc-b" {
		t.Fatalf("the gate must have seen the new record, got %+v", seen)
	}
	settle(j2, o2, seen)
	if n := rows(t, ctx, c2, "SELECT count(*) FROM loopadmin.activation WHERE paused"); n != 0 {
		t.Fatal("a record equal to the operator source must never be paused")
	}
	launchOK(j2, "inv-6")
}

// TestJournalConfigRefuses pins the explicit-connection rules offline.
// regression: #5492
func TestJournalConfigRefuses(t *testing.T) {
	clearPGEnv(t)
	ok := Config{Host: "127.0.0.1", Port: 5432, User: "u", Database: "d", Timeout: time.Second}
	if _, err := driverConfig(ok); err != nil {
		t.Fatalf("positive control: %v", err)
	}
	for name, c := range map[string]Config{
		"remote without TLS": {Host: "db.example.test", Port: 5432, User: "u", Database: "d", Timeout: time.Second},
		"no timeout":         {Host: "127.0.0.1", Port: 5432, User: "u", Database: "d"},
		"multi-host":         {Host: "127.0.0.1,127.0.0.2", Port: 5432, User: "u", Database: "d", Timeout: time.Second},
		"no database":        {Host: "127.0.0.1", Port: 5432, User: "u", Timeout: time.Second},
	} {
		if _, err := driverConfig(c); err == nil {
			t.Errorf("%s: must be refused", name)
		}
	}
	t.Setenv("PGHOST", "elsewhere")
	if _, err := driverConfig(ok); err == nil {
		t.Error("an ambient PG* variable must be refused")
	}
	if s := (Config{Password: "hunter2"}).String(); s != "explicit PostgreSQL configuration (redacted)" {
		t.Errorf("config must not print its fields, got %q", s)
	}
	for _, verb := range []string{"%v", "%+v", "%#v", "%s"} {
		if s := fmt.Sprintf(verb, Config{Password: "hunter2"}); strings.Contains(s, "hunter2") {
			t.Errorf("config must not print its password under %s, got %q", verb, s)
		}
	}
}

// TestJournalManifestForwardOnly pins the forward-only migration manifest.
// regression: #5492
func TestJournalManifestForwardOnly(t *testing.T) {
	if _, err := checkManifest(migrationFS); err != nil {
		t.Fatalf("positive control: the shipped migrations must pass: %v", err)
	}
	for name, body := range map[string]string{
		"down section": "-- +goose Up\nSELECT 1;\n-- +goose Down\nSELECT 1;\n",
		"no up":        "SELECT 1;\n",
		"no tx":        "-- +goose NO TRANSACTION\n-- +goose Up\nSELECT 1;\n",
	} {
		fsys := fstest.MapFS{"migrations/00001_x.sql": {Data: []byte(body)}}
		if _, err := checkManifest(fsys); err == nil {
			t.Errorf("%s: must be refused", name)
		}
	}
	fsys := fstest.MapFS{"migrations/1_x.sql": {Data: []byte("-- +goose Up\nSELECT 1;\n")}}
	if _, err := checkManifest(fsys); err == nil {
		t.Error("an unexpected migration file name must be refused")
	}
}
