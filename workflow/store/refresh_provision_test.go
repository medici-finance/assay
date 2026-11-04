package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/refresh"
)

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
	t.Run("live-fill-delete", func(t *testing.T) {
		r := sqlMirror(t)
		p := provisionInput(t, r)
		if _, e := r.db.ProvisionRefresh(ctx, p); e != nil {
			t.Fatal(e)
		}
		g, e := r.db.Refresh(r.s, p.Config, refreshTestAuthority{})
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
	g, e := r.db.Refresh(r.s, p.Config, refreshTestAuthority{})
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

// regression: F-provision-immutable-replay
func TestPGRefreshProvisionReplay(t *testing.T) {
	r := sqlMirror(t)
	p := provisionInput(t, r)
	ctx := context.Background()
	first, e := r.db.ProvisionRefresh(ctx, p)
	if e != nil {
		t.Fatal(e)
	}
	g, e := r.db.Refresh(r.s, p.Config, refreshTestAuthority{})
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
	if _, e = r.raw.Exec(`UPDATE public.refresh_budgets SET known=false`); e != nil {
		t.Fatal(e)
	}
	if _, e = r.db.ProvisionRefresh(ctx, p); !errors.Is(e, refresh.ErrHeld) {
		t.Fatal("existing unknown accounting became known through replay", e)
	}
}
