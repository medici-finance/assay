package store_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/mirror/mirrortest"
	"github.com/medici-finance/assay/forgeread/refresh"
	"github.com/medici-finance/assay/workflow/pgtestfixture"
	"github.com/medici-finance/assay/workflow/store"
)

type provisionProcessPacket struct {
	Config               store.Config
	Request              store.RefreshProvision
	Ready, Start, Result string
}

type provisionProcessResult struct {
	PID     int
	Receipt store.RefreshProvisionReceipt
}

func waitProvisionPath(ctx context.Context, path string) error {
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, e := os.Stat(path); e == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// regression: F-provision-two-process-initialization
func TestPGRefreshProvisionChild(t *testing.T) {
	path := os.Getenv("ASSAY_PROVISION_PROCESS_PACKET")
	if path == "" {
		return
	}
	raw, e := os.ReadFile(path)
	var p provisionProcessPacket
	if e != nil || len(raw) > 64<<10 || json.Unmarshal(raw, &p) != nil || !strings.HasPrefix(p.Config.Database, "assay_pg_") || !strings.HasPrefix(p.Config.User, "assay_role_") {
		t.Fatal("invalid owned provisioning process packet")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db, e := store.Open(ctx, p.Config)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = os.WriteFile(p.Ready, []byte("ready"), 0600); e != nil {
		t.Fatal("process ready receipt unavailable")
	}
	if e = waitProvisionPath(ctx, p.Start); e != nil {
		t.Fatal("process start barrier unavailable")
	}
	receipt, e := db.ProvisionRefresh(ctx, p.Request)
	if e != nil {
		t.Fatal("process initialization refused", e)
	}
	result, e := json.Marshal(provisionProcessResult{PID: os.Getpid(), Receipt: receipt})
	if e != nil || os.WriteFile(p.Result, result, 0600) != nil {
		t.Fatal("process result receipt unavailable")
	}
}

// regression: F-provision-two-process-initialization
func TestPGRefreshProvisionProcesses(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	f := pgtestfixture.New(t)
	if e := store.Migrate(ctx, ownedRoleConfig(f.Config)); e != nil {
		t.Fatal(e)
	}
	roles := pgtestfixture.NewRefreshRoles(t, f)
	admin, e := f.ConnectOwned(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	// Hold the first actual source INSERT while both independent processes have
	// entered initialization. With the production source fence intact, one waits
	// here and one waits at that fence; without it both have read source absence.
	barrier, e := admin.Conn(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer barrier.Close()
	if _, e = barrier.ExecContext(ctx, `SELECT pg_advisory_lock(hashtext(current_database()),55)`); e != nil {
		t.Fatal("owned SQL initialization barrier unavailable")
	}
	defer func() {
		cleanup, finish := context.WithTimeout(context.Background(), 5*time.Second)
		defer finish()
		barrier.ExecContext(cleanup, `SELECT pg_advisory_unlock(hashtext(current_database()),55)`)
	}()
	if _, e = admin.ExecContext(ctx, `CREATE FUNCTION public.fixture_process_barrier() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.kind='source' THEN PERFORM pg_advisory_xact_lock(hashtext(current_database()),55); END IF; RETURN NEW; END $$; CREATE TRIGGER fixture_process_barrier BEFORE INSERT ON public.refresh_initializations FOR EACH ROW EXECUTE FUNCTION public.fixture_process_barrier()`); e != nil {
		t.Fatal("owned SQL process barrier unavailable")
	}
	var now time.Time
	if e = admin.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&now); e != nil {
		t.Fatal(e)
	}
	p := roleProvisionInput(now)
	dir := t.TempDir()
	start := filepath.Join(dir, "start")
	var commands [2]*exec.Cmd
	var outputs [2]strings.Builder
	var results [2]provisionProcessResult
	defer func() {
		for _, command := range commands {
			if command != nil && command.Process != nil && command.ProcessState == nil {
				command.Process.Kill()
				command.Wait()
			}
		}
	}()
	for i := range commands {
		packet := provisionProcessPacket{Config: ownedRoleConfig(roles.Operator), Request: p, Ready: filepath.Join(dir, fmt.Sprint("ready-", i)), Start: start, Result: filepath.Join(dir, fmt.Sprint("result-", i))}
		raw, e := json.Marshal(packet)
		if e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(dir, fmt.Sprint("packet-", i))
		if e = os.WriteFile(path, raw, 0600); e != nil {
			t.Fatal("owned process packet unavailable")
		}
		command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestPGRefreshProvisionChild$", "-test.v")
		// The child receives only its private operator packet and local scratch,
		// never the parent's fixture-admin DSN, PG discovery or forge credentials.
		command.Env = []string{"ASSAY_PROVISION_PROCESS_PACKET=" + path, "KUBECONFIG=/dev/null", "TMPDIR=" + dir, "TEMP=" + dir, "TMP=" + dir}
		if root := os.Getenv("SystemRoot"); root != "" {
			command.Env = append(command.Env, "SystemRoot="+root)
		}
		command.Stdout, command.Stderr = &outputs[i], &outputs[i]
		commands[i] = command
		if e = command.Start(); e != nil {
			t.Fatal("owned child process unavailable")
		}
	}
	for i := range commands {
		if e = waitProvisionPath(ctx, filepath.Join(dir, fmt.Sprint("ready-", i))); e != nil {
			t.Fatal("independent child did not reach its start barrier")
		}
	}
	if e = os.WriteFile(start, nil, 0600); e != nil {
		t.Fatal("shared process release unavailable")
	}
	poll := time.NewTicker(5 * time.Millisecond)
	defer poll.Stop()
	for {
		var waiting int
		if e = admin.QueryRowContext(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND usename=$1 AND wait_event_type='Lock' AND wait_event='advisory'`, roles.Operator.User).Scan(&waiting); e != nil {
			t.Fatal("independent operator SQL waiters unavailable")
		}
		if waiting == 2 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("two independent operators did not reach the SQL barrier")
		case <-poll.C:
		}
	}
	if _, e = barrier.ExecContext(ctx, `SELECT pg_advisory_unlock(hashtext(current_database()),55)`); e != nil {
		t.Fatal("owned SQL process barrier release unavailable")
	}
	for i, command := range commands {
		if e = command.Wait(); e != nil {
			t.Fatalf("operator child initialization failed: %s", outputs[i].String())
		}
		raw, e := os.ReadFile(filepath.Join(dir, fmt.Sprint("result-", i)))
		if e != nil || json.Unmarshal(raw, &results[i]) != nil {
			t.Fatal("owned process result unavailable")
		}
	}
	if results[0].PID == results[1].PID || results[0].PID == os.Getpid() || results[0].Receipt != results[1].Receipt || results[0].Receipt.Records != 3 {
		t.Fatal("independent processes did not reconcile one exact initialization")
	}
	var receipts, budgets, admissions, callers int
	if e = admin.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM public.refresh_initializations),(SELECT count(*) FROM public.refresh_budgets),(SELECT count(*) FROM public.refresh_admissions),(SELECT count(*) FROM public.refresh_callers)`).Scan(&receipts, &budgets, &admissions, &callers); e != nil || [4]int{receipts, budgets, admissions, callers} != [4]int{3, 1, 1, 1} {
		t.Fatal("process race duplicated or partially initialized accounting", e)
	}
	// Reopen through the actual restricted operator identity, not a seed helper.
	reopened, e := store.Open(ctx, ownedRoleConfig(roles.Operator))
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	if got, e := reopened.ProvisionRefresh(ctx, p); e != nil || got != results[0].Receipt {
		t.Fatal("reopened exact initialization lost immutable provenance", e)
	}
}

func ownedRoleConfig(c pgtestfixture.Connection) store.Config {
	return store.Config{Host: c.Host, Port: c.Port, User: c.User, Password: c.Password, Database: c.Database, Timeout: 5 * time.Second}
}

func roleProvisionInput(now time.Time) store.RefreshProvision {
	u := store.RefreshInitialUsage{Known: true, WindowAt: now.UTC().Truncate(time.Microsecond), Provenance: "synthetic-role-baseline"}
	c := store.RefreshConfig{SourceID: "synthetic-role-source", AdmissionID: "synthetic-role-admission", AccessEpoch: "access-1", ConfigEpoch: "config-1", PolicyClass: "synthetic", PolicyEpoch: "policy-1", Limits: refresh.Limits{CallerFillsPerWindow: 10, AdmissionFillsPerWindow: 100, FillWindow: time.Hour, MinFreshness: time.Millisecond, MaxFreshness: time.Hour, ActiveFills: 2, QueuedCallers: 4, AdmissionFills: 4, AdmissionWaiters: 8, FailureEntries: 8, WaitTimeout: 10 * time.Second, FillTimeout: 30 * time.Second, Cooldown: time.Millisecond, FailureTTL: 2 * time.Millisecond, Refresh: mirror.RefreshPolicy{FullInventoryInterval: time.Hour}}, Native: store.NativeLimits{CallerPerWindow: 10, AdmissionPerWindow: 100, SourcePerWindow: 100, PerFill: 8, Concurrent: 2, Reservations: 100, Window: time.Hour}, PollInterval: time.Millisecond}
	return store.RefreshProvision{Config: c, Source: store.RefreshInitialSource{Usage: u, RetryKnown: true}, Admission: u, Callers: []store.RefreshInitialCaller{{ID: "caller-a", Usage: u}}}
}

type roleAuthority struct{}

type roleClock func() time.Time

func (c roleClock) Now() time.Time { return c() }

func (roleAuthority) Authorize(ctx context.Context, c refresh.Caller, s mirror.Scope) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if c.ID == "" || c.AdmissionID != s.AdmissionID {
		return mirror.ErrPermission
	}
	return nil
}

// regression: F-provision-runtime-sql-role
func TestPGRefreshRuntimePrivileges(t *testing.T) {
	for _, escape := range []string{"membership", "ownership", "security-definer"} {
		t.Run("effective-"+escape, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			f := pgtestfixture.New(t)
			if e := store.Migrate(ctx, ownedRoleConfig(f.Config)); e != nil {
				t.Fatal(e)
			}
			roles := pgtestfixture.NewRefreshRoles(t, f)
			admin, e := f.ConnectOwned(ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer admin.Close()
			raw, e := f.ConnectRuntime(ctx, roles)
			if e != nil {
				t.Fatal(e)
			}
			defer raw.Close()
			var setup, restore, proof string
			switch escape {
			case "membership":
				setup = "GRANT " + pgx.Identifier{roles.Operator.User}.Sanitize() + " TO " + pgx.Identifier{roles.Runtime.User}.Sanitize()
				restore = "REVOKE " + pgx.Identifier{roles.Operator.User}.Sanitize() + " FROM " + pgx.Identifier{roles.Runtime.User}.Sanitize()
				proof = "SET LOCAL ROLE " + pgx.Identifier{roles.Operator.User}.Sanitize() + `; INSERT INTO public.refresh_budgets(source_id,profile,known,window_at,window_ns,http_used,http_limit,inflight_limit,reservation_limit) VALUES('synthetic-escaped-budget','synthetic',true,clock_timestamp(),1,0,1,1,1)`
			case "ownership":
				setup = "ALTER TABLE public.refresh_initializations OWNER TO " + pgx.Identifier{roles.Runtime.User}.Sanitize()
				restore = "ALTER TABLE public.refresh_initializations OWNER TO " + pgx.Identifier{f.Config.User}.Sanitize()
				proof = `ALTER TABLE public.refresh_initializations ADD COLUMN escaped integer`
			case "security-definer":
				setup = `CREATE FUNCTION public.fixture_role_escape() RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$ BEGIN TRUNCATE public.refresh_callers; END $$`
				restore = `DROP FUNCTION public.fixture_role_escape()`
				proof = `SELECT public.fixture_role_escape()`
			}
			if _, e = admin.ExecContext(ctx, setup); e != nil {
				t.Fatal("owned role escape setup failed", e)
			}
			restored := false
			defer func() {
				if !restored {
					cleanup, finish := context.WithTimeout(context.Background(), 5*time.Second)
					defer finish()
					if _, e := admin.ExecContext(cleanup, restore); e != nil {
						t.Error("owned role escape cleanup failed")
					}
				}
			}()
			// This is a real reachable capability, not merely a role-name test.
			tx, e := raw.BeginTx(ctx, nil)
			if e != nil {
				t.Fatal(e)
			}
			_, e = tx.ExecContext(ctx, proof)
			tx.Rollback()
			if e != nil {
				t.Fatal("effective escape control did not reach its protected operation", e)
			}
			if e = pgtestfixture.ApplyRefreshRoles(t, f, roles); e == nil {
				t.Fatal("unsafe effective role was accepted", escape)
			}
			if _, e = admin.ExecContext(ctx, restore); e != nil {
				t.Fatal("owned role restore failed", e)
			}
			restored = true
			if e = pgtestfixture.ApplyRefreshRoles(t, f, roles); e != nil {
				t.Fatal("healthy declared role refused after escape removal", e)
			}
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	f := pgtestfixture.New(t)
	if e := store.Migrate(ctx, ownedRoleConfig(f.Config)); e != nil {
		t.Fatal(e)
	}
	roles := pgtestfixture.NewRefreshRoles(t, f)
	operator, e := store.Open(ctx, ownedRoleConfig(roles.Operator))
	if e != nil {
		t.Fatal("restricted operator could not open canonical schema", e)
	}
	defer operator.Close()
	runtime, e := store.Open(ctx, ownedRoleConfig(roles.Runtime))
	if e != nil {
		t.Fatal("restricted runtime could not open canonical schema", e)
	}
	defer runtime.Close()
	admin, e := f.ConnectOwned(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	var now time.Time
	if e = admin.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&now); e != nil {
		t.Fatal(e)
	}
	p := roleProvisionInput(now)
	if _, e = operator.ProvisionRefresh(ctx, p); e != nil {
		t.Fatal("declared operator could not initialize known profiles", e)
	}
	if _, e = runtime.ProvisionRefresh(ctx, p); e == nil {
		t.Fatal("restricted runtime invoked operator initialization")
	}
	raw, e := f.ConnectRuntime(ctx, roles)
	if e != nil {
		t.Fatal(e)
	}
	defer raw.Close()
	var sessionUser, currentUser string
	if e = raw.QueryRowContext(ctx, `SELECT session_user,current_user`).Scan(&sessionUser, &currentUser); e != nil || sessionUser != roles.Runtime.User || currentUser != roles.Runtime.User || sessionUser == f.Config.User || currentUser == f.Config.User {
		t.Fatal("role proof did not use its genuine restricted login", e)
	}
	t.Log("RUNTIME_LOGIN: session_user=current_user=exact_owned_runtime; fixture_admin=false")
	for name, statement := range map[string]string{
		"insert-budget": `INSERT INTO public.refresh_budgets(source_id,profile,known,window_at,window_ns,http_used,http_limit,inflight_limit,reservation_limit) VALUES('foreign','foreign',true,clock_timestamp(),1,0,1,1,1)`,
		"known-upgrade": `UPDATE public.refresh_budgets SET known=true`,
		"profile":       `UPDATE public.refresh_admissions SET profile='changed'`,
		"provenance":    `UPDATE public.refresh_initializations SET baseline='changed'`,
		"delete":        `DELETE FROM public.refresh_callers`,
		"truncate":      `TRUNCATE public.refresh_callers`,
		"reset":         `UPDATE public.refresh_budgets SET http_used=0,window_at=clock_timestamp()`,
		"ddl":           `ALTER TABLE public.refresh_callers ADD COLUMN escaped integer`,
		"schema":        `CREATE SCHEMA escaped`,
		"temporary":     `CREATE TEMP TABLE escaped(id integer)`,
	} {
		t.Run(name, func(t *testing.T) {
			tx, e := raw.BeginTx(ctx, nil)
			if e != nil {
				t.Fatal(e)
			}
			_, e = tx.ExecContext(ctx, statement)
			tx.Rollback()
			if e == nil {
				t.Fatal("runtime escaped declared role boundary", name)
			}
		})
	}
	s, e := runtime.Mirror(store.MirrorLimits{MaxRecords: 128, MaxBytes: 1 << 20})
	if e != nil {
		t.Fatal(e)
	}
	g, e := runtime.Refresh(s, p.Config, roleAuthority{}, roleClock(func() time.Time { return time.Now().UTC() }))
	if e != nil {
		t.Fatal(e)
	}
	scope := mirror.Scope{Repository: mirror.RepositoryID{Provider: mirror.GitHub, Host: "forge.example", Path: "synthetic/project"}, Kind: mirror.Issues, AdmissionID: p.Config.AdmissionID}
	demand := refresh.Demand{Key: refresh.FillKey{Scope: scope, SourceID: p.Config.SourceID, AccessEpoch: p.Config.AccessEpoch, ConfigEpoch: p.Config.ConfigEpoch, PolicyClass: p.Config.PolicyClass, PolicyEpoch: p.Config.PolicyEpoch}, CallerID: "caller-a", Limits: p.Config.Limits, Now: time.Now()}
	ticket, e := g.Acquire(ctx, demand)
	if e != nil || !ticket.Owner {
		t.Fatal("declared runtime could not acquire the canonical gate", e)
	}
	reservation, e := g.BeforeAttempt(ctx, ticket.AttemptGrant)
	if e != nil {
		t.Fatal("declared runtime could not reserve native work", e)
	}
	// This tests the SQL role at the typed observation port. The separate joined
	// fixture supplies actual native HTTP evidence, never this synthetic value.
	dataAt := time.Now().UTC()
	if e = g.ObserveAttempt(ctx, reservation, mirror.AttemptObservation{HTTP: mirror.WorkCount{Known: true, Value: 1}, DecodedBytes: mirror.WorkCount{Known: true, Value: 2}, Status: 200, ResponseComplete: true, Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNoConstraint}, ObservedAt: dataAt}); e != nil {
		t.Fatal("declared runtime could not report its synthetic observation", e)
	}
	a := &mirrortest.TestAdmission{Name: p.Config.AdmissionID, Allowed: true}
	update := mirror.ScanUpdate{Scope: scope, CompleteScan: true, FullInventory: true, Coverage: mirror.Coverage{State: mirror.Complete, LastAttempted: dataAt, LastComplete: dataAt, LastInventory: dataAt, ObservedUpTo: dataAt}, Upsert: []mirror.Observation{{ID: mirror.RecordID{Repository: scope.Repository, Kind: "issue", Number: 1}, Title: "synthetic-role-canary", UpdatedAt: dataAt, ObservedAt: dataAt, InInventory: true}}}
	cfg := p.Config
	caller := refresh.Caller{ID: "caller-a", AdmissionID: cfg.AdmissionID, AccessEpoch: cfg.AccessEpoch, ConfigEpoch: cfg.ConfigEpoch, PolicyClass: cfg.PolicyClass, PolicyEpoch: cfg.PolicyEpoch, Limits: cfg.Limits}
	completion := refresh.Completion{Caller: caller, Authority: roleAuthority{}, Query: refresh.Query{Query: mirror.Query{Scope: scope, MaxAge: time.Hour, Now: time.Now()}}, Update: &update, Now: time.Now(), Attempts: mirror.AttemptReceipt{Grant: ticket.AttemptGrant, HTTP: mirror.WorkCount{Known: true, Value: 1}, DecodedBytes: mirror.WorkCount{Known: true, Value: 2}, ObservedAt: dataAt, Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNoConstraint}}}
	if e = g.Complete(ctx, ticket, s, a, completion); e != nil {
		t.Fatal("declared runtime could not commit through the canonical gate/store", e)
	}
	snapshot, e := s.Read(ctx, a, scope)
	if e != nil || len(snapshot.Records) != 1 || snapshot.Records[0].Title != "synthetic-role-canary" {
		t.Fatal("declared runtime could not query committed observations", e)
	}
	if _, e = g.Leave(ctx, ticket, refresh.Outcome{Checked: true, Fulfilled: true, SnapshotVersion: snapshot.Version}); e != nil {
		t.Fatal(e)
	}
	time.Sleep(5 * time.Millisecond)
	demand.Key.Scope.Repository.Path = "synthetic/other"
	if next, e := g.Acquire(ctx, demand); e != nil || !next.Owner {
		t.Fatal("healthy canonical eligible-terminal eviction failed", e)
	}
	var fills, charged int64
	if e = admin.QueryRowContext(ctx, `SELECT count(*) FROM public.refresh_fills`).Scan(&fills); e != nil || fills != 1 {
		t.Fatal("eligible terminal fill was not evicted", e, fills)
	}
	if e = admin.QueryRowContext(ctx, `SELECT http_used FROM public.refresh_budgets`).Scan(&charged); e != nil || charged != 1 {
		t.Fatal("eligible cache eviction reset source accounting", e, charged)
	}
}

type provisionedCompletionRig struct {
	db    *store.DB
	store *store.MirrorStore
	gate  *store.RefreshGate
	raw   *sql.DB
	cfg   store.RefreshConfig
	scope mirror.Scope
	a     *mirrortest.TestAdmission
}

func newProvisionedCompletionRig(t *testing.T, clock refresh.Clock) provisionedCompletionRig {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	f := pgtestfixture.New(t)
	if err := store.Migrate(ctx, ownedRoleConfig(f.Config)); err != nil {
		t.Fatal(err)
	}
	roles := pgtestfixture.NewRefreshRoles(t, f)
	operator, err := store.Open(ctx, ownedRoleConfig(roles.Operator))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { operator.Close() })
	admin, err := f.ConnectOwned(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	var databaseNow time.Time
	if err = admin.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&databaseNow); err != nil {
		t.Fatal(err)
	}
	p := roleProvisionInput(databaseNow)
	p.Callers = append(p.Callers, store.RefreshInitialCaller{ID: "caller-b", Usage: p.Admission})
	if _, err = operator.ProvisionRefresh(ctx, p); err != nil {
		t.Fatal("restricted operator could not initialize completion fixture", err)
	}
	// Each connection really logs in as its declared identity. The runtime does
	// not inherit an administrator session capable of RESET ROLE escalation.
	raw, err := f.ConnectRuntime(ctx, roles)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { raw.Close() })
	var session, current string
	if err = raw.QueryRowContext(ctx, `SELECT session_user,current_user`).Scan(&session, &current); err != nil || session != roles.Runtime.User || current != roles.Runtime.User || session == f.Config.User {
		t.Fatal("completion proof did not use genuine restricted runtime LOGIN", err)
	}
	t.Log("RUNTIME_LOGIN: session_user=current_user=exact_owned_runtime; fixture_admin=false")
	runtime, err := store.Open(ctx, ownedRoleConfig(roles.Runtime))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { runtime.Close() })
	s, err := runtime.Mirror(store.MirrorLimits{MaxRecords: 128, MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	g, err := runtime.Refresh(s, p.Config, roleAuthority{}, clock)
	if err != nil {
		t.Fatal(err)
	}
	scope := mirror.Scope{Repository: mirror.RepositoryID{Provider: mirror.GitHub, Host: "forge.example", Path: "synthetic/project"}, Kind: mirror.Issues, AdmissionID: p.Config.AdmissionID}
	return provisionedCompletionRig{runtime, s, g, raw, p.Config, scope, &mirrortest.TestAdmission{Name: p.Config.AdmissionID, Allowed: true}}
}

func (r provisionedCompletionRig) demand(caller string, now time.Time) refresh.Demand {
	c := r.cfg
	return refresh.Demand{Key: refresh.FillKey{Scope: r.scope, SourceID: c.SourceID, AccessEpoch: c.AccessEpoch, ConfigEpoch: c.ConfigEpoch, PolicyClass: c.PolicyClass, PolicyEpoch: c.PolicyEpoch}, CallerID: caller, Limits: c.Limits, Now: now}
}

func (r provisionedCompletionRig) completion(now time.Time) refresh.Completion {
	c := r.cfg
	return refresh.Completion{Caller: refresh.Caller{ID: "caller-a", AdmissionID: c.AdmissionID, AccessEpoch: c.AccessEpoch, ConfigEpoch: c.ConfigEpoch, PolicyClass: c.PolicyClass, PolicyEpoch: c.PolicyEpoch, Limits: c.Limits}, Authority: roleAuthority{}, Query: refresh.Query{Query: mirror.Query{Scope: r.scope, MaxAge: time.Hour, Now: now}}, Now: now}
}

func (r provisionedCompletionRig) update(at time.Time) mirror.ScanUpdate {
	return mirror.ScanUpdate{Scope: r.scope, CompleteScan: true, FullInventory: true, Coverage: mirror.Coverage{State: mirror.Complete, LastAttempted: at, LastComplete: at, LastInventory: at, ObservedUpTo: at}, Upsert: []mirror.Observation{{ID: mirror.RecordID{Repository: r.scope.Repository, Kind: "issue", Number: 1}, Title: "synthetic-populated-runtime", UpdatedAt: at, ObservedAt: at, InInventory: true}}}
}

// regression: F-provision-held-populated-runtime
func TestPGRefreshRuntimeHeldCompletion(t *testing.T) {
	for _, previous := range []bool{false, true} {
		for _, hold := range []mirror.AttemptHold{"", mirror.AttemptCallerHeld, mirror.AttemptAdmissionHeld} {
			name := string(hold)
			if name == "" {
				name = "admission-denied"
			}
			t.Run(fmt.Sprintf("%s/previous=%t", name, previous), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				clock := roleClock(func() time.Time { return time.Now().UTC() })
				r := newProvisionedCompletionRig(t, clock)
				update := r.update(clock.Now().Truncate(time.Microsecond))
				before, err := r.store.Apply(ctx, r.a, update)
				if err != nil {
					t.Fatal(err)
				}
				owner, err := r.gate.Acquire(ctx, r.demand("caller-a", clock.Now()))
				if err != nil || !owner.Owner {
					t.Fatal("restricted runtime could not acquire populated scope", err)
				}
				if previous {
					if err = r.gate.Complete(ctx, owner, r.store, r.a, r.completion(clock.Now())); err != nil {
						t.Fatal("previous runtime completion refused", err)
					}
					if _, err = r.gate.Leave(ctx, owner, refresh.Outcome{}); err != nil {
						t.Fatal(err)
					}
					owner, err = r.gate.Acquire(ctx, r.demand("caller-a", clock.Now()))
					if err != nil || !owner.Owner {
						t.Fatal("next runtime generation refused", err)
					}
					update.ExpectedVersion = before.Version
					update.Upsert[0].Title = "synthetic-independently-committed-runtime"
					before, err = r.store.Apply(ctx, r.a, update)
					if err != nil {
						t.Fatal(err)
					}
				}
				// This store cannot decode even the existing observation. Closing a
				// denied owner must require only the canonical identity/version fence.
				bounded, err := r.db.Mirror(store.MirrorLimits{MaxRecords: 1, MaxBytes: 1})
				if err != nil {
					t.Fatal(err)
				}
				if _, err = bounded.Read(ctx, r.a, r.scope); err == nil {
					t.Fatal("observation-decode negative control was not exercised")
				}
				gate, err := r.db.Refresh(bounded, r.cfg, roleAuthority{}, clock)
				if err != nil {
					t.Fatal(err)
				}
				completion := r.completion(clock.Now())
				completion.AdmissionDenied, completion.Attempts.Held = hold == "", hold
				if err = gate.Complete(ctx, owner, bounded, r.a, completion); err != nil {
					t.Fatal("restricted held completion could not fence populated scope", err)
				}
				state, err := gate.Wait(ctx, owner)
				if err != nil || state.Pending || !state.AttemptHeld || state.Reason != "" || !state.RetryAt.IsZero() {
					t.Fatal("held owner remained pending or poisoned shared failure", state, err)
				}
				after, err := r.store.Read(ctx, r.a, r.scope)
				if err != nil || !reflect.DeepEqual(before, after) {
					t.Fatal("held owner read/restamped or modified observation", err)
				}
				if _, err = gate.Leave(ctx, owner, refresh.Outcome{}); err != nil {
					t.Fatal(err)
				}
				next, err := r.gate.Acquire(ctx, r.demand("caller-b", clock.Now()))
				if err != nil || !next.Owner {
					t.Fatal("held owner denied another eligible runtime caller", err)
				}
				var charged, attempts int64
				if err = r.raw.QueryRowContext(ctx, `SELECT http_used,(SELECT count(*) FROM public.refresh_attempts) FROM public.refresh_budgets`).Scan(&charged, &attempts); err != nil || charged != 0 || attempts != 0 {
					t.Fatal("non-attempt completion changed native accounting", err)
				}
			})
		}
	}
}

// regression: F-provision-runtime-data-clock
func TestPGRefreshRuntimeDataClock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	// A controlled application offset is data evidence, never a lease tolerance.
	clock := roleClock(func() time.Time { return time.Now().UTC().Add(500 * time.Millisecond) })
	r := newProvisionedCompletionRig(t, clock)
	owner, err := r.gate.Acquire(ctx, r.demand("caller-a", clock.Now()))
	if err != nil || !owner.Owner {
		t.Fatal(err)
	}
	reservation, err := r.gate.BeforeAttempt(ctx, owner.AttemptGrant)
	if err != nil {
		t.Fatal(err)
	}
	dataAt := clock.Now().Truncate(time.Microsecond)
	update := r.update(dataAt)
	if err = r.gate.ObserveAttempt(ctx, reservation, mirror.AttemptObservation{HTTP: mirror.WorkCount{Value: 1, Known: true}, DecodedBytes: mirror.WorkCount{Known: true}, Status: 200, ResponseComplete: true, ObservedAt: dataAt, Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNoConstraint}}); err != nil {
		t.Fatal(err)
	}
	var databaseNow time.Time
	if err = r.raw.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&databaseNow); err != nil || !dataAt.After(databaseNow) {
		t.Fatal("runtime fixture did not establish application data-clock skew", err)
	}
	completion := r.completion(clock.Now())
	completion.Update = &update
	completion.Attempts = mirror.AttemptReceipt{Grant: owner.AttemptGrant, ObservedAt: dataAt, HTTP: mirror.WorkCount{Value: 1, Known: true}, Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNoConstraint}}
	if err = r.gate.Complete(ctx, owner, r.store, r.a, completion); err != nil {
		t.Fatal(err)
	}
	state, err := r.gate.Wait(ctx, owner)
	if err != nil || state.Pending || state.AttemptHeld {
		t.Fatal("restricted runtime data clock dropped qualified observation", state, err)
	}
	result, err := mirror.QueryScope(ctx, r.store, r.a, mirror.Query{Scope: r.scope, MaxAge: time.Hour, Now: clock.Now()})
	if err != nil || !result.Available || result.Stale || result.Snapshot.Coverage.State != mirror.Complete || result.Snapshot.Version != "1" || !result.Snapshot.Coverage.LastComplete.Equal(dataAt) {
		t.Fatal("qualified runtime observation was dropped or restamped", err)
	}
	var charged int64
	if err = r.raw.QueryRowContext(ctx, `SELECT http_used FROM public.refresh_budgets`).Scan(&charged); err != nil || charged != 1 {
		t.Fatal("runtime data clock changed canonical charged usage", err, charged)
	}
}
