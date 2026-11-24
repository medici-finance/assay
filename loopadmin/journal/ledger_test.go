package journal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// artifactStore is a fake operator artifact store: reference to content.
type artifactStore map[string][]byte

func (s artifactStore) Open(_ context.Context, ref string) (io.ReadCloser, error) {
	b, ok := s[ref]
	if !ok {
		return nil, errors.New("no such artifact")
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func digestOf(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// activated returns a migrated, activated fixture database for testCell.
func activated(t *testing.T, ctx context.Context, pg *pgFixture) (Config, *operator) {
	t.Helper()
	cfg := pg.create(ctx, "")
	must(t, Migrate(ctx, cfg))
	op := &operator{}
	op.rotate(testCell, "inc-a", "key-a")
	must(t, Activate(ctx, cfg, testCell, op, StopConfirmation{Receipt: "stop-a"}))
	return cfg, op
}

// TestLoopAdminLaunchBudgetBrake covers Verify row 4 (parent row 9): admission
// reads the profile's window budget and the launches recorded in the ledger
// under one lock, so unreadable or unverifiable use refuses without a child,
// restarts and concurrent callers cannot exceed the budget, and a missing
// pacing estimate alone never refuses.
// regression: #5492
func TestLoopAdminLaunchBudgetBrake(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pg := newPG(t)
	cfg, op := activated(t, ctx, pg)

	launches := 0
	try := func(j *Journal, in Intent) (Decision, error) {
		d, err := j.Intend(ctx, in, nil)
		if d.Launch {
			launches++
		}
		return d, err
	}
	j, err := Open(ctx, cfg, testCell, op)
	must(t, err)

	// A profile without a budget refuses and records nothing.
	if _, err = try(j, intent("inv-0", "req-0")); !errors.Is(err, ErrNoBudget) {
		t.Fatalf("a missing budget must refuse, got %v", err)
	}
	if n := rows(t, ctx, cfg, "SELECT count(*) FROM loopadmin.invocation"); n != 0 {
		t.Fatalf("a refused launch must record no intent, found %d", n)
	}

	// The first launch has no recorded launch to pace from: the estimate is
	// unavailable and the launch is still admitted.
	must(t, SetBudget(ctx, cfg, Budget{Profile: "worker", Window: time.Hour, Max: 2}))
	d, err := try(j, intent("inv-1", "req-1"))
	if err != nil || !d.Launch || !d.NotBefore.IsZero() {
		t.Fatalf("an unavailable pacing estimate alone must admit the launch, got %+v, %v", d, err)
	}
	d, err = try(j, intent("inv-2", "req-2"))
	if err != nil || !d.Launch || d.NotBefore.IsZero() {
		t.Fatalf("a second launch must be admitted with a pacing estimate, got %+v, %v", d, err)
	}
	if _, err = try(j, intent("inv-3", "req-3")); !errors.Is(err, ErrBudgetSpent) {
		t.Fatalf("a spent window budget must refuse, got %v", err)
	}

	// Restarts read the ledger, never a cached count.
	for i := 0; i < 3; i++ {
		must(t, j.Close())
		j, err = Open(ctx, cfg, testCell, op)
		must(t, err)
		if _, err = try(j, intent("inv-3", "req-3")); !errors.Is(err, ErrBudgetSpent) {
			t.Fatalf("restart %d must not regain allowance, got %v", i, err)
		}
	}
	must(t, j.Close())

	// Concurrent callers in separate handles share one budget.
	must(t, SetBudget(ctx, cfg, Budget{Profile: "burst", Window: time.Hour, Max: 3}))
	var wg sync.WaitGroup
	var mu sync.Mutex
	admitted, spent := 0, 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			jj, err := Open(ctx, cfg, testCell, op)
			if err != nil {
				t.Error(err)
				return
			}
			defer closeTestHandle(t, jj)
			in := intent("burst-"+string(rune('a'+i)), "req-burst-"+string(rune('a'+i)))
			in.Profile = "burst"
			d, err := jj.Intend(ctx, in, nil)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil && d.Launch:
				admitted++
			case errors.Is(err, ErrBudgetSpent):
				spent++
			default:
				t.Errorf("a concurrent admission must launch or refuse on budget, got %+v, %v", d, err)
			}
		}(i)
	}
	wg.Wait()
	if admitted != 3 || spent != 5 {
		t.Fatalf("concurrent callers must share the budget exactly, got %d admitted, %d refused", admitted, spent)
	}

	// Unreadable launch use refuses without a child: the runtime role loses
	// read access to the ledger.
	must(t, SetBudget(ctx, cfg, Budget{Profile: "fresh", Window: time.Hour, Max: 5}))
	fresh := func(inv string) Intent {
		in := intent(inv, "req-"+inv)
		in.Profile = "fresh"
		return in
	}
	rt := pg.runtimeLogin(ctx, cfg)
	run(t, ctx, cfg, "REVOKE SELECT ON loopadmin.launch FROM loopadmin_runtime")
	jr, err := Open(ctx, rt, testCell, op)
	must(t, err)
	defer closeTestHandle(t, jr)
	before := launches
	if _, err = try(jr, fresh("inv-4")); !errors.Is(err, ErrUsageUnknown) {
		t.Fatalf("unreadable launch use must refuse, got %v", err)
	}
	if launches != before {
		t.Fatal("unreadable launch use must start no child")
	}
	run(t, ctx, cfg, "GRANT SELECT ON loopadmin.launch TO loopadmin_runtime")
	if d, err = try(jr, fresh("inv-4")); err != nil || !d.Launch {
		t.Fatalf("positive control: readable use under budget must launch, got %+v, %v", d, err)
	}

	// An unknown invocation of the profile makes its use unverifiable.
	lost := fresh("inv-lost")
	if held, err := jr.Intend(ctx, lost, &Receipt{ID: "lost-receipt", Key: lost.Key}); err != nil || held.State != Unknown {
		t.Fatalf("a lost receipt must hold as unknown, got %+v, %v", held, err)
	}
	if _, err = try(jr, fresh("inv-5")); !errors.Is(err, ErrUsageUnknown) {
		t.Fatalf("an unknown launch must refuse further launches of its profile, got %v", err)
	}
	if launches != 3 {
		t.Fatalf("exactly three launches may be admitted outside the concurrent burst, got %d", launches)
	}
}

// TestJournalPassiveRestore covers Verify row 5: a fixture backup restored
// under external restore provenance retains unknowns, pauses, spent
// allowance, limits, usage completeness, checkpoints and artifact identity;
// the restore check pauses and never launches; a missing or corrupt
// referenced artifact holds.
// regression: #5492
func TestJournalPassiveRestore(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pg := newPG(t)
	cfg, op := activated(t, ctx, pg)
	must(t, SetBudget(ctx, cfg, Budget{Profile: "worker", Window: time.Hour, Max: 2}))
	must(t, AdmitLimit(ctx, cfg, ProfileLimit{Profile: "worker", Name: "max_tokens", Value: 1000}))
	if err := AdmitLimit(ctx, cfg, ProfileLimit{Profile: "worker", Name: "max_tokens", Value: 9000}); !errors.Is(err, ErrConflict) {
		t.Fatalf("an admitted limit must be immutable, got %v", err)
	}
	content := []byte("artifact one")
	store := artifactStore{"art/1": content}
	art := Ref{Ref: "art/1", Digest: digestOf(content)}

	j, err := Open(ctx, cfg, testCell, op)
	must(t, err)
	in := intent("inv-1", "req-1")
	d, err := j.Intend(ctx, in, nil)
	if err != nil || !d.Launch {
		t.Fatalf("positive control: the first launch must be admitted, got %+v, %v", d, err)
	}
	must(t, j.MarkLaunched(ctx, in.Key))
	must(t, j.RecordUsage(ctx, in.Key, UsageReport{Complete: false, Launches: 1}))
	must(t, j.RecordCheckpoint(ctx, in.Key, Ref{Ref: "ckpt/1", Digest: digestOf([]byte("checkpoint"))}))
	must(t, j.RecordArtifact(ctx, in.Key, art))
	lost := intent("inv-lost", "req-lost")
	lost.Profile = "other"
	if held, err := j.Intend(ctx, lost, &Receipt{ID: "lost-receipt", Key: lost.Key}); err != nil || held.State != Unknown {
		t.Fatalf("a lost receipt must hold as unknown, got %+v, %v", held, err)
	}
	must(t, j.Close())

	// Backup: the inventory verifies before it is recorded.
	if _, err = Inventory(ctx, cfg, artifactStore{"art/1": []byte("corrupt")}); !errors.Is(err, ErrRestoreHeld) {
		t.Fatalf("a backup over a corrupt artifact must hold, got %v", err)
	}
	m, err := Inventory(ctx, cfg, store)
	must(t, err)
	if m.Version <= 0 || len(m.Artifacts) != 1 || m.Artifacts[0] != art {
		t.Fatalf("the inventory must carry the schema version and the artifact, got %+v", m)
	}
	restored := pg.create(ctx, cfg.Database)
	op.restore(testCell, "snapshot-1")

	launches := 0
	try := func(j *Journal, inv string) error {
		d, err := j.Intend(ctx, intent(inv, "req-"+inv), nil)
		if d.Launch {
			launches++
		}
		return err
	}

	// A missing or corrupt artifact, or an inventory that differs, holds.
	for name, s := range map[string]artifactStore{
		"missing": {},
		"corrupt": {"art/1": []byte("tampered")},
	} {
		if err = CheckRestore(ctx, restored, testCell, m, s); !errors.Is(err, ErrRestoreHeld) {
			t.Fatalf("%s artifact: the restore must hold, got %v", name, err)
		}
	}
	extra := m
	extra.Artifacts = append([]Ref{{Ref: "art/2", Digest: art.Digest}}, m.Artifacts...)
	if err = CheckRestore(ctx, restored, testCell, extra, store); !errors.Is(err, ErrRestoreHeld) {
		t.Fatalf("an inventory that differs from the backup must hold, got %v", err)
	}
	must(t, CheckRestore(ctx, restored, testCell, m, store))
	if n := rows(t, ctx, restored, "SELECT count(*) FROM loopadmin.activation WHERE paused"); n != 1 {
		t.Fatal("the restore check must pause the restored activation")
	}

	// Everything the backup carried is retained.
	for query, want := range map[string]int{
		"SELECT count(*) FROM loopadmin.invocation WHERE state = 'unknown'":                       1,
		"SELECT count(*) FROM loopadmin.launch WHERE profile = 'worker'":                          1,
		"SELECT count(*) FROM loopadmin.profile_limit WHERE name = 'max_tokens' AND value = 1000": 1,
		"SELECT count(*) FROM loopadmin.usage_report WHERE NOT complete":                          1,
		"SELECT count(*) FROM loopadmin.checkpoint":                                               1,
		"SELECT count(*) FROM loopadmin.artifact":                                                 1,
	} {
		if n := rows(t, ctx, restored, query); n != want {
			t.Fatalf("restore must retain the backup's records: %s = %d, want %d", query, n, want)
		}
	}

	// The restored journal launches nothing, even if its activation record is
	// reopened in the database: restore provenance outside the database still
	// names the incarnation it carried.
	jr, err := Open(ctx, restored, testCell, op)
	must(t, err)
	defer closeTestHandle(t, jr)
	if err = try(jr, "inv-2"); !errors.Is(err, ErrPaused) {
		t.Fatalf("a restored journal must start paused, got %v", err)
	}
	run(t, ctx, restored, "UPDATE loopadmin.activation SET paused = false")
	if err = try(jr, "inv-2"); !errors.Is(err, ErrPaused) {
		t.Fatalf("a restored incarnation must hold even with its record reopened, got %v", err)
	}
	if launches != 0 {
		t.Fatalf("restore must launch nothing, started %d", launches)
	}

	// Rotation and a confirmed stop reopen it, with the spent allowance kept.
	op.rotate(testCell, "inc-b", "key-b")
	must(t, Activate(ctx, restored, testCell, op, StopConfirmation{Receipt: "stop-b"}))
	if err = try(jr, "inv-2"); err != nil || launches != 1 {
		t.Fatalf("rotation plus a confirmed stop must reopen the journal, got %v, %d launches", err, launches)
	}
	if err = try(jr, "inv-3"); !errors.Is(err, ErrBudgetSpent) {
		t.Fatalf("a restore must not regain spent allowance, got %v", err)
	}
}

// TestJournalRuntimeGrants covers Verify row 8 (DR-graph-exec-41, option 1):
// the migration owner owns every journal object, and the runtime role cannot
// change the schema, rewrite or remove consumed-launch rows, or write a
// budget or a limit.
// regression: #5492
func TestJournalRuntimeGrants(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pg := newPG(t)
	cfg, op := activated(t, ctx, pg)
	budget(t, ctx, cfg)
	must(t, AdmitLimit(ctx, cfg, ProfileLimit{Profile: "worker", Name: "max_tokens", Value: 1000}))
	if n := rows(t, ctx, cfg, "SELECT count(*) FROM pg_catalog.pg_tables WHERE schemaname = 'loopadmin' AND tableowner <> 'loopadmin_owner'"); n != 0 {
		t.Fatalf("the migration owner must own every journal table, %d do not", n)
	}
	rt := pg.runtimeLogin(ctx, cfg)

	// Positive controls: the runtime path works under the runtime role.
	j, err := Open(ctx, rt, testCell, op)
	must(t, err)
	defer closeTestHandle(t, j)
	in := intent("inv-1", "req-1")
	d, err := j.Intend(ctx, in, nil)
	if err != nil || !d.Launch {
		t.Fatalf("positive control: the runtime role must admit a launch, got %+v, %v", d, err)
	}
	must(t, j.MarkLaunched(ctx, in.Key))
	must(t, j.RecordUsage(ctx, in.Key, UsageReport{Complete: true, Launches: 1}))
	must(t, j.RecordArtifact(ctx, in.Key, Ref{Ref: "art/1", Digest: digestOf([]byte("a"))}))
	if _, err = j.Limits(ctx, "worker"); err != nil {
		t.Fatalf("positive control: the runtime role must read limits, got %v", err)
	}

	db, err := connect(ctx, rt)
	must(t, err)
	defer closeTestHandle(t, db)
	for _, stmt := range []string{
		"UPDATE loopadmin.launch SET launched_at = launched_at - interval '1 day'",
		"DELETE FROM loopadmin.launch",
		"TRUNCATE loopadmin.launch",
		"INSERT INTO loopadmin.profile_budget (profile, window_seconds, max_launches) VALUES ('rogue', 1, 1000)",
		"UPDATE loopadmin.profile_budget SET max_launches = 1000",
		"DELETE FROM loopadmin.profile_budget",
		"INSERT INTO loopadmin.profile_limit (profile, name, value) VALUES ('worker', 'rogue', 1)",
		"UPDATE loopadmin.profile_limit SET value = 9000",
		"DELETE FROM loopadmin.profile_limit",
		"UPDATE loopadmin.invocation SET profile = 'rogue'",
		"DELETE FROM loopadmin.usage_report",
		"UPDATE loopadmin.artifact SET digest = digest",
		"UPDATE loopadmin.activation SET incarnation_id = 'rogue'",
		"CREATE TABLE loopadmin.rogue (x int)",
		"ALTER TABLE loopadmin.launch ADD COLUMN rogue int",
		"DROP TABLE loopadmin.launch",
	} {
		_, err := db.ExecContext(ctx, stmt)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Errorf("the runtime role must be refused %q, got %v", stmt, err)
		}
	}
	if err = SetBudget(ctx, rt, Budget{Profile: "worker", Window: time.Hour, Max: 1000}); err == nil {
		t.Error("the runtime role must not set a budget")
	}
	if n := rows(t, ctx, cfg, "SELECT count(*) FROM loopadmin.launch"); n != 1 {
		t.Fatalf("the consumed-launch ledger must be unchanged, found %d rows", n)
	}
	if n := rows(t, ctx, cfg, "SELECT count(*) FROM loopadmin.profile_budget WHERE max_launches = 100"); n != 1 {
		t.Fatal("the budget must be unchanged")
	}
}
