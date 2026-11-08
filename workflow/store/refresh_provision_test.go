package store

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/refresh"
)

// regression: F-provision-schema-role-contract
func TestPGRefreshProvisionSchema(t *testing.T) {
	r := sqlMirror(t)
	ctx := context.Background()
	raw, e := os.ReadFile("../../spec/coordination-store-v1.md")
	if e != nil {
		t.Fatal(e)
	}
	want := contractColumns(string(raw))
	got, e := schemaColumns(ctx, r.raw)
	if e != nil || !reflect.DeepEqual(want, got) {
		t.Fatal("provisioning schema/catalog mismatch", e, want, got)
	}
	for _, table := range refreshRoleTables {
		var columns string
		if e = r.raw.QueryRow(`SELECT string_agg(attname::text,' ' ORDER BY attnum) FROM pg_attribute WHERE attrelid=to_regclass($1) AND attnum>0 AND NOT attisdropped`, "public."+table.name).Scan(&columns); e != nil || columns != table.columns {
			t.Fatal("declared role column profile differs from its actual table", table.name, e)
		}
	}
	// Immutable initialization provenance has an independent SQL boundary.
	p := provisionInput(t, r)
	if _, e = r.db.ProvisionRefresh(ctx, p); e != nil {
		t.Fatal(e)
	}
	for _, statement := range []string{`UPDATE public.refresh_initializations SET baseline='changed'`, `DELETE FROM public.refresh_initializations`} {
		tx, e := r.raw.Begin()
		if e != nil {
			t.Fatal(e)
		}
		_, e = tx.Exec(statement)
		tx.Rollback()
		if e == nil {
			t.Fatal("raw SQL rewrote immutable initialization provenance")
		}
	}
	if _, e = r.raw.Exec(`ALTER TABLE public.refresh_initializations DROP COLUMN baseline`); e != nil {
		t.Fatal(e)
	}
	changed, e := schemaColumns(ctx, r.raw)
	if e != nil || reflect.DeepEqual(want, changed) {
		t.Fatal("missing initialization provenance was invisible", e)
	}
	if _, e = r.db.ProvisionRefresh(ctx, p); e == nil {
		t.Fatal("missing initialization provenance became runtime bootstrap")
	}
	for _, role := range []RefreshRoleProfile{
		{}, {Database: "db", Operator: "same", Runtime: "same"}, {Database: "db", Operator: strings.Repeat("x", 64), Runtime: "runtime"}, {Database: "db", Operator: "operator", Runtime: "nul\x00role"},
	} {
		if _, e := role.SQL(); !errors.Is(e, refresh.ErrPolicy) {
			t.Fatal("unqualified SQL role identity accepted", e)
		}
	}
}

func provisionInput(t *testing.T, r mirrorRig) RefreshProvision {
	t.Helper()
	var now time.Time
	if e := r.raw.QueryRow(`SELECT clock_timestamp()`).Scan(&now); e != nil {
		t.Fatal(e)
	}
	u := RefreshInitialUsage{Known: true, WindowAt: now.UTC().Truncate(time.Microsecond), Provenance: "synthetic-initial-v1"}
	return RefreshProvision{Config: testRefreshConfig(), Source: RefreshInitialSource{Usage: u, RetryKnown: true}, Admission: u, Callers: []RefreshInitialCaller{{ID: "caller-a", Usage: u}, {ID: "caller-b", Usage: u}}}
}

func provisionCounts(t *testing.T, r mirrorRig) [4]int64 {
	t.Helper()
	var counts [4]int64
	for i, table := range []string{"refresh_initializations", "refresh_budgets", "refresh_admissions", "refresh_callers"} {
		if e := r.raw.QueryRow(`SELECT count(*) FROM public.` + table).Scan(&counts[i]); e != nil {
			t.Fatal(e)
		}
	}
	return counts
}

// regression: F-provision-known-baseline
func TestPGRefreshProvision(t *testing.T) {
	ctx := context.Background()
	t.Run("eviction", checkProvisionEviction)
	t.Run("atomic", func(t *testing.T) {
		r := sqlMirror(t)
		p := provisionInput(t, r)
		if _, e := r.raw.Exec(`CREATE FUNCTION public.fixture_initialization_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic initialization failure'; END $$; CREATE TRIGGER fixture_initialization_failure BEFORE INSERT ON public.refresh_callers FOR EACH ROW EXECUTE FUNCTION public.fixture_initialization_failure()`); e != nil {
			t.Fatal(e)
		}
		if _, e := r.db.ProvisionRefresh(ctx, p); !errors.Is(e, refresh.ErrHeld) {
			t.Fatal("injected initialization failure was not held", e)
		}
		if got := provisionCounts(t, r); got != [4]int64{} {
			t.Fatal("failed initialization retained a staged prefix", got)
		}
	})
	t.Run("live-fill-delete", func(t *testing.T) {
		r := sqlMirror(t)
		p := provisionInput(t, r)
		if _, e := r.db.ProvisionRefresh(ctx, p); e != nil {
			t.Fatal(e)
		}
		g, e := r.db.Refresh(r.s, p.Config, refreshTestAuthority{}, refreshTestClock{})
		if e != nil {
			t.Fatal(e)
		}
		rig := refreshRig{r, g, p.Config}
		_, u := mirrorInput()
		ticket := acquireRefresh(t, rig, rig.demand(u.Scope, "caller-a"))
		key, _, _ := fillIndex(rig.demand(u.Scope, "caller-a").Key)
		tx, e := r.raw.Begin()
		if e != nil {
			t.Fatal(e)
		}
		_, e = tx.Exec(`DELETE FROM public.refresh_fills WHERE fill_key=$1`, key)
		tx.Rollback()
		if e == nil {
			t.Fatal("direct SQL deleted a live accounted fill")
		}
		reservation, e := g.BeforeAttempt(ctx, ticket.AttemptGrant)
		if e != nil {
			t.Fatal("refused raw deletion damaged the healthy fill", e)
		}
		if e = g.ObserveAttempt(ctx, reservation, mirror.AttemptObservation{HTTP: mirror.WorkCount{Known: true}, DecodedBytes: mirror.WorkCount{Known: true}, Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNoConstraint}, ObservedAt: time.Now()}); e != nil {
			t.Fatal(e)
		}
	})
	t.Run("canceled", func(t *testing.T) {
		r := sqlMirror(t)
		p := provisionInput(t, r)
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		if _, e := r.db.ProvisionRefresh(canceled, p); !errors.Is(e, context.Canceled) || !errors.Is(e, refresh.ErrHeld) {
			t.Fatal("cancellation lost its bounded typed cause", e)
		}
		if got := provisionCounts(t, r); got != [4]int64{} {
			t.Fatal("canceled provisioning wrote state", got)
		}
	})
	for _, name := range []string{"unknown-source", "unknown-admission", "unknown-caller", "unknown-retry", "future", "expired", "duplicate", "child-accounting"} {
		t.Run(name, func(t *testing.T) {
			r := sqlMirror(t)
			p := provisionInput(t, r)
			switch name {
			case "unknown-source":
				p.Source.Usage.Known = false
			case "unknown-admission":
				p.Admission.Known = false
			case "unknown-caller":
				p.Callers[0].Usage.Known = false
			case "unknown-retry":
				p.Source.RetryKnown = false
			case "future":
				p.Source.Usage.WindowAt = p.Source.Usage.WindowAt.Add(time.Hour)
			case "expired":
				p.Admission.WindowAt = p.Admission.WindowAt.Add(-2 * time.Hour)
			case "duplicate":
				p.Callers[1].ID = p.Callers[0].ID
			case "child-accounting":
				p.Callers[0].Usage.ChargedAttempts = 1
			}
			if _, e := r.db.ProvisionRefresh(ctx, p); e == nil {
				t.Fatal("unqualified initial accounting accepted")
			}
			if got := provisionCounts(t, r); got != [4]int64{} {
				t.Fatal("refused provisioning left a partial initialization", got)
			}
		})
	}
	r := sqlMirror(t)
	p := provisionInput(t, r)
	g, e := r.db.Refresh(r.s, p.Config, refreshTestAuthority{}, refreshTestClock{})
	if e != nil {
		t.Fatal(e)
	}
	rig := refreshRig{r, g, p.Config}
	_, u := mirrorInput()
	if _, e = g.Acquire(ctx, rig.demand(u.Scope, "caller-a")); e == nil {
		t.Fatal("missing profiles became allowance")
	}
	first, e := r.db.ProvisionRefresh(ctx, p)
	if e != nil || first.Records != 4 || first.Digest == [32]byte{} {
		t.Fatal("known operator baseline refused", e)
	}
	if got := provisionCounts(t, r); got != [4]int64{4, 1, 1, 2} {
		t.Fatal("initial record set differs", got)
	}
	if ticket, e := g.Acquire(ctx, rig.demand(u.Scope, "caller-a")); e != nil || !ticket.Owner {
		t.Fatal("initialized profiles did not reach the real gate", e)
	}
	// Explicitly known positive charges are supported independently of measured
	// HTTP, with each child's charges covered by the shared parent baseline.
	r = sqlMirror(t)
	p = provisionInput(t, r)
	p.Source.Usage.ChargedAttempts = 4
	p.Admission.ChargedAttempts, p.Admission.Fills = 3, 3
	p.Callers[0].Usage.ChargedAttempts, p.Callers[0].Usage.Fills = 2, 1
	p.Callers[1].Usage.ChargedAttempts, p.Callers[1].Usage.Fills = 1, 2
	if _, e := r.db.ProvisionRefresh(ctx, p); e != nil {
		t.Fatal("known charged baseline confused with measured work", e)
	}
	var used int64
	if e := r.raw.QueryRow(`SELECT http_used FROM public.refresh_budgets WHERE source_id=$1`, []byte(p.Config.SourceID)).Scan(&used); e != nil || used != 4 {
		t.Fatal("positive source charges changed", e, used)
	}
}

func checkProvisionEviction(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	for _, name := range []string{"held", "unobserved", "unknown", "waiter", "retention", "retry", "base-retry", "review-retry", "current-generation"} {
		t.Run(name, func(t *testing.T) {
			r := sqlMirror(t)
			p := provisionInput(t, r)
			if _, e := r.db.ProvisionRefresh(ctx, p); e != nil {
				t.Fatal(e)
			}
			g, e := r.db.Refresh(r.s, p.Config, refreshTestAuthority{}, refreshTestClock{})
			if e != nil {
				t.Fatal(e)
			}
			rig := refreshRig{r, g, p.Config}
			a, u := mirrorInput()
			demand := rig.demand(u.Scope, "caller-a")
			owner := acquireRefresh(t, rig, demand)
			if name == "waiter" {
				acquireRefresh(t, rig, rig.demand(u.Scope, "caller-b"))
			}
			completion := completionRefresh(rig, owner, nil)
			if name == "unobserved" || name == "unknown" {
				reservation, e := g.BeforeAttempt(ctx, owner.AttemptGrant)
				if e != nil {
					t.Fatal(e)
				}
				if name == "unknown" {
					if e = g.ObserveAttempt(ctx, reservation, mirror.AttemptObservation{ObservedAt: time.Now()}); !errors.Is(e, mirror.ErrAttemptHeld) {
						t.Fatal("unknown individual observation did not retain its hold", e)
					}
				}
				// The canonical per-caller denial path can finish a generation
				// without reconciling individual reservations. It cannot refund them.
				completion.AdmissionDenied = true
			} else if name == "held" {
				completion.Attempts.Held = mirror.AttemptSourceHeld
			}
			if e = g.Complete(ctx, owner, r.s, a, completion); e != nil {
				t.Fatal(e)
			}
			if name != "waiter" {
				if _, e = g.Leave(ctx, owner, refresh.Outcome{}); e != nil {
					t.Fatal(e)
				}
			}
			key, _, _ := fillIndex(demand.Key)
			if name != "retention" {
				if _, e = r.raw.ExecContext(ctx, `UPDATE public.refresh_fills SET retire_at=clock_timestamp()-interval '1 second' WHERE fill_key=$1`, key); e != nil {
					t.Fatal(e)
				}
			}
			column := map[string]string{"retry": "retry_at", "base-retry": "base_retry_at", "review-retry": "review_retry_at"}[name]
			if column != "" {
				if _, e = r.raw.ExecContext(ctx, `UPDATE public.refresh_fills SET `+column+`=clock_timestamp()+interval '1 hour' WHERE fill_key=$1`, key); e != nil {
					t.Fatal(e)
				}
			}
			if name == "current-generation" {
				start := make(chan struct{})
				deleted := make(chan error, 1)
				go func() {
					<-start
					_, e := r.raw.ExecContext(ctx, `DELETE FROM public.refresh_fills WHERE fill_key=$1`, key)
					deleted <- e
				}()
				close(start)
				next, e := g.Acquire(ctx, demand)
				<-deleted // Either the old terminal row is deleted, or the new pending row refuses it.
				if e != nil || !next.Owner || next.Generation == owner.Generation {
					t.Fatal("eligible eviction raced away the current generation", e)
				}
				if _, e = g.BeforeAttempt(ctx, owner.AttemptGrant); !errors.Is(e, mirror.ErrAttemptHeld) {
					t.Fatal("obsolete generation retained attempt authority", e)
				}
				if _, e = g.BeforeAttempt(ctx, next.AttemptGrant); e != nil {
					t.Fatal("current generation lost its accounted authority", e)
				}
				var charged int64
				if e = r.raw.QueryRowContext(ctx, `SELECT http_used FROM public.refresh_budgets`).Scan(&charged); e != nil || charged != 1 {
					t.Fatal("eviction race reset the source allowance", e, charged)
				}
				return
			}
			tx, e := r.raw.BeginTx(ctx, nil)
			if e != nil {
				t.Fatal(e)
			}
			_, e = tx.ExecContext(ctx, `DELETE FROM public.refresh_fills WHERE fill_key=$1`, key)
			tx.Rollback()
			if e == nil {
				t.Fatal("direct SQL evicted protected accounting", name)
			}
		})
	}
}

// regression: F-provision-immutable-replay
func TestPGRefreshProvisionReplay(t *testing.T) {
	t.Run("runtime-window-roll", func(t *testing.T) {
		r := sqlMirror(t)
		p := provisionInput(t, r)
		p.Config.Limits.FillWindow = 250 * time.Millisecond
		p.Config.Native.Window = p.Config.Limits.FillWindow
		ctx := context.Background()
		first, e := r.db.ProvisionRefresh(ctx, p)
		if e != nil {
			t.Fatal(e)
		}
		g, e := r.db.Refresh(r.s, p.Config, refreshTestAuthority{}, refreshTestClock{})
		if e != nil {
			t.Fatal(e)
		}
		rig := refreshRig{r, g, p.Config}
		_, u := mirrorInput()
		ticket := acquireRefresh(t, rig, rig.demand(u.Scope, "caller-a"))
		rsv, e := g.BeforeAttempt(ctx, ticket.AttemptGrant)
		if e != nil {
			t.Fatal(e)
		}
		if e = g.ObserveAttempt(ctx, rsv, mirror.AttemptObservation{HTTP: mirror.WorkCount{Known: true}, DecodedBytes: mirror.WorkCount{Known: true}, Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNoConstraint}, ObservedAt: time.Now()}); e != nil {
			t.Fatal(e)
		}
		time.Sleep(300 * time.Millisecond)
		acquireRefresh(t, rig, rig.demand(u.Scope, "caller-b"))
		var before, after time.Time
		var charged int64
		if e = r.raw.QueryRow(`SELECT window_at,http_used FROM public.refresh_budgets`).Scan(&before, &charged); e != nil || charged != 0 || !before.After(p.Source.Usage.WindowAt) {
			t.Fatal("actual runtime window did not advance independently", e)
		}
		if got, e := r.db.ProvisionRefresh(ctx, p); e != nil || got != first {
			t.Fatal("original immutable replay refused after legitimate window rollover", e)
		}
		if e = r.raw.QueryRow(`SELECT window_at,http_used FROM public.refresh_budgets`).Scan(&after, &charged); e != nil || charged != 0 || !after.Equal(before) {
			t.Fatal("initialization replay rewound the current runtime window", e)
		}
	})
	r := sqlMirror(t)
	p := provisionInput(t, r)
	ctx := context.Background()
	first, e := r.db.ProvisionRefresh(ctx, p)
	if e != nil {
		t.Fatal(e)
	}
	g, e := r.db.Refresh(r.s, p.Config, refreshTestAuthority{}, refreshTestClock{})
	if e != nil {
		t.Fatal(e)
	}
	rig := refreshRig{r, g, p.Config}
	_, u := mirrorInput()
	ticket := acquireRefresh(t, rig, rig.demand(u.Scope, "caller-a"))
	reservation, e := g.BeforeAttempt(ctx, ticket.AttemptGrant)
	if e != nil {
		t.Fatal(e)
	}
	if e = g.ObserveAttempt(ctx, reservation, mirror.AttemptObservation{HTTP: mirror.WorkCount{Known: true}, DecodedBytes: mirror.WorkCount{Known: true}, Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNoConstraint}, ObservedAt: time.Now()}); e != nil {
		t.Fatal(e)
	}
	second, e := r.db.ProvisionRefresh(ctx, p)
	if e != nil || second != first {
		t.Fatal("exact replay failed after known zero-HTTP charged work", e)
	}
	var sourceUsed, admissionUsed, callerUsed, measured int64
	if e = r.raw.QueryRow(`SELECT b.http_used,a.http_used,c.http_used,x.http_count FROM public.refresh_budgets b JOIN public.refresh_admissions a ON a.source_id=b.source_id JOIN public.refresh_callers c ON c.admission=a.admission JOIN public.refresh_attempts x ON x.caller_id=c.caller_id AND x.admission=c.admission WHERE c.caller_id=$1`, []byte("caller-a")).Scan(&sourceUsed, &admissionUsed, &callerUsed, &measured); e != nil || [4]int64{sourceUsed, admissionUsed, callerUsed, measured} != [4]int64{1, 1, 1, 0} {
		t.Fatal("replay refunded a charged zero-entry reservation", e)
	}
	bad := p
	bad.Admission.Provenance = "synthetic-conflicting-baseline"
	if _, e = r.db.ProvisionRefresh(ctx, bad); !errors.Is(e, refresh.ErrHeld) {
		t.Fatal("conflicting immutable baseline accepted", e)
	}
	reservation, e = g.BeforeAttempt(ctx, ticket.AttemptGrant)
	if e != nil {
		t.Fatal(e)
	}
	floor := time.Now().UTC().Add(3 * time.Minute)
	if e = g.ObserveAttempt(ctx, reservation, mirror.AttemptObservation{HTTP: mirror.WorkCount{Known: true, Value: 1}, DecodedBytes: mirror.WorkCount{Known: true}, Status: 429, Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNotBefore, NotBefore: floor}, ObservedAt: time.Now()}); e != nil {
		t.Fatal(e)
	}
	var before, after time.Time
	if e = r.raw.QueryRow(`SELECT retry_at FROM public.refresh_budgets`).Scan(&before); e != nil {
		t.Fatal(e)
	}
	if got, e := r.db.ProvisionRefresh(ctx, p); e != nil || got != first {
		t.Fatal("known retry floor prevented exact non-mutating replay", e)
	}
	if e = r.raw.QueryRow(`SELECT retry_at,http_used FROM public.refresh_budgets`).Scan(&after, &sourceUsed); e != nil || !after.Equal(before) || sourceUsed != 2 {
		t.Fatal("replay reset known retry/accounting", e)
	}
	shared := p
	shared.Config.AdmissionID = "synthetic-shared-admission"
	shared.Admission.Provenance = "synthetic-shared-baseline"
	shared.Callers = []RefreshInitialCaller{{ID: "caller-c", Usage: p.Admission}}
	if _, e = r.db.ProvisionRefresh(ctx, shared); e != nil {
		t.Fatal("new admission could not reuse its known shared source", e)
	}
	if got := provisionCounts(t, r); got != [4]int64{6, 1, 2, 3} {
		t.Fatal("shared admission created another source allowance", got)
	}
	if e = r.raw.QueryRow(`SELECT retry_at,http_used FROM public.refresh_budgets`).Scan(&after, &sourceUsed); e != nil || !after.Equal(before) || sourceUsed != 2 {
		t.Fatal("new admission reset shared source", e)
	}
	countsBefore := provisionCounts(t, r)
	if _, e = r.raw.Exec(`CREATE FUNCTION public.fixture_provision_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.caller_id=convert_to('caller-d','UTF8') THEN RAISE EXCEPTION 'synthetic staged initialization failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER fixture_provision_failure BEFORE INSERT ON public.refresh_callers FOR EACH ROW EXECUTE FUNCTION public.fixture_provision_failure()`); e != nil {
		t.Fatal(e)
	}
	enroll := p
	enroll.Callers = []RefreshInitialCaller{{ID: "caller-c", Usage: p.Admission}, {ID: "caller-d", Usage: p.Admission}}
	if _, e = r.db.ProvisionRefresh(ctx, enroll); !errors.Is(e, refresh.ErrHeld) {
		t.Fatal("synthetic staged enrollment failure did not refuse", e)
	}
	if got := provisionCounts(t, r); got != countsBefore {
		t.Fatal("failed enrollment left partially initialized rows", got, countsBefore)
	}
	if _, e = r.raw.Exec(`DROP TRIGGER fixture_provision_failure ON public.refresh_callers; DROP FUNCTION public.fixture_provision_failure()`); e != nil {
		t.Fatal(e)
	}
	if got, e := r.db.ProvisionRefresh(ctx, p); e != nil || got != first {
		t.Fatal("failed enrollment damaged prior complete initialization", e)
	}
	if e = r.raw.QueryRow(`SELECT retry_at,http_used FROM public.refresh_budgets`).Scan(&after, &sourceUsed); e != nil || !after.Equal(before) || sourceUsed != 2 {
		t.Fatal("rollback reset previously charged work or retry floor", e)
	}
	if _, e = r.raw.Exec(`UPDATE public.refresh_budgets SET unknown_hold=true`); e != nil {
		t.Fatal(e)
	}
	if _, e = r.db.ProvisionRefresh(ctx, p); !errors.Is(e, refresh.ErrHeld) {
		t.Fatal("existing unknown retry state was reconciled by initialization", e)
	}
	if _, e = r.raw.Exec(`UPDATE public.refresh_budgets SET known=false`); e != nil {
		t.Fatal(e)
	}
	if _, e = r.db.ProvisionRefresh(ctx, p); !errors.Is(e, refresh.ErrHeld) {
		t.Fatal("existing unknown accounting became known through replay", e)
	}
}
