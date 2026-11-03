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

// regression: F-refresh-independent-sql-schema
func TestPGRefreshSchemaBoundary(t *testing.T) {
	t.Run("shared-retry-policy", checkRefreshSharedRetryPolicy)
	t.Run("raw-expired-completion", checkRawExpiredRefresh)
	t.Run("identity-codec", checkRefreshIdentityCodec)
	TestPGFoundationBoundary(t)
	r := sqlRefresh(t)
	ctx := context.Background()
	raw, e := os.ReadFile("../../spec/coordination-store-v1.md")
	if e != nil {
		t.Fatal(e)
	}
	want := contractColumns(string(raw))
	got, e := schemaColumns(ctx, r.raw)
	if e != nil || !reflect.DeepEqual(want, got) {
		t.Fatalf("contract/catalog mismatch: %v / %v", want, got)
	}
	_, u := mirrorInput()
	ticket := acquireRefresh(t, r, r.demand(u.Scope, "caller-a"))
	key, _, _ := fillIndex(r.demand(u.Scope, "caller-a").Key)
	// Raw SQL completion cannot fabricate a new observation version outside Apply.
	rawRefused(t, r.raw, key, `UPDATE public.refresh_fills SET state='done',snapshot_version=1,completion_xid=pg_current_xact_id() WHERE fill_key=$1`)
	// An isolated increment is conservative consumption, not a permission to send:
	// inserting an unreserved individual slot below the Go API still refuses.
	tx, e := r.raw.Begin()
	if e != nil {
		t.Fatal(e)
	}
	_, e = tx.Exec(`INSERT INTO public.refresh_attempts(reservation_id,admission,source_id,fill_key,generation,caller_id,grant_id,expires_at) VALUES('raw',$1,$2,$3,$4,'caller-a',$4,$5)`, []byte(r.cfg.AdmissionID), []byte(r.cfg.SourceID), key, []byte(ticket.Generation), ticket.AttemptGrant.ExpiresAt)
	if e == nil {
		t.Fatal("unreserved raw attempt accepted")
	}
	tx.Rollback()
	res, e := r.g.BeforeAttempt(ctx, ticket.AttemptGrant)
	if e != nil {
		t.Fatal(e)
	}
	o := mirror.AttemptObservation{HTTP: mirror.WorkCount{Known: true, Value: 1}, Status: 429, ObservedAt: ticket.AttemptGrant.ExpiresAt, Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNotBefore, NotBefore: ticket.AttemptGrant.ExpiresAt}}
	if e = r.g.ObserveAttempt(ctx, res, o); e != nil {
		t.Fatal(e)
	}
	rawRefused(t, r.raw, []byte(r.cfg.SourceID), `UPDATE public.refresh_budgets SET retry_at=NULL WHERE source_id=$1`)
	rawRefused(t, r.raw, []byte(r.cfg.SourceID), `UPDATE public.refresh_budgets SET http_used=0,window_at=clock_timestamp() WHERE source_id=$1`)
	workflow, e := os.ReadFile("../../.github/workflows/assay-workflow.yml")
	if e != nil {
		t.Fatal(e)
	}
	for _, input := range []string{"workflow/**", "forgeread/mirror/**", "forgeread/refresh/**", "spec/coordination-store-v1.md"} {
		if strings.Count(string(workflow), input) != 2 {
			t.Fatal("missing live CI input", input)
		}
	}
	if _, e = r.raw.Exec(`ALTER TABLE public.refresh_attempts DROP COLUMN response_complete`); e != nil {
		t.Fatal(e)
	}
	changed, _ := schemaColumns(ctx, r.raw)
	if reflect.DeepEqual(want, changed) {
		t.Fatal("missing refresh column accepted")
	}
	if _, e = r.raw.Exec(`CREATE TABLE public.unexpected_refresh(id integer)`); e != nil {
		t.Fatal(e)
	}
	changed, _ = schemaColumns(ctx, r.raw)
	if reflect.DeepEqual(want, changed) {
		t.Fatal("extra refresh table accepted")
	}
}

func checkRawExpiredRefresh(t *testing.T) {
	cfg := testRefreshConfig()
	cfg.Limits.FillTimeout = 300 * time.Millisecond
	r := sqlRefreshConfig(t, cfg)
	_, u := mirrorInput()
	ticket := acquireRefresh(t, r, r.demand(u.Scope, "caller-a"))
	time.Sleep(350 * time.Millisecond)
	key, _, _ := fillIndex(r.demand(u.Scope, "caller-a").Key)
	rawRefused(t, r.raw, key, `UPDATE public.refresh_fills SET state='done',completion_xid=pg_current_xact_id() WHERE fill_key=$1 AND generation=$2`, []byte(ticket.Generation))
}

// regression: F-refresh-one-shared-retry-authority
func checkRefreshSharedRetryPolicy(t *testing.T) {
	r := sqlRefresh(t)
	for _, backoff := range []time.Duration{-time.Nanosecond, 10 * time.Second, 3 * time.Minute} {
		config := r.cfg
		config.Limits.Refresh.FailureBackoff = backoff
		gate, e := r.db.Refresh(r.s, config, refreshTestAuthority{})
		if !errors.Is(e, refresh.ErrPolicy) || gate != nil {
			t.Fatalf("nonzero independent failure policy accepted: %s", backoff)
		}
	}
	// Rejection neither normalizes the profile nor replaces the existing known one.
	gate, e := r.db.Refresh(r.s, r.cfg, refreshTestAuthority{})
	if e != nil {
		t.Fatal(e)
	}
	_, u := mirrorInput()
	if ticket, e := gate.Acquire(context.Background(), r.demand(u.Scope, "caller-b")); e != nil || !ticket.Owner {
		t.Fatal("explicit zero profile failed after invalid profiles", e)
	}
}
