package pgtestfixture

import (
	"context"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/refresh"
	"github.com/medici-finance/assay/workflow/store"
)

type seedAuthority struct{}

func (seedAuthority) Authorize(ctx context.Context, c refresh.Caller, s mirror.Scope) error {
	return ctx.Err()
}

// regression: F-owned-refresh-seed-capability
func TestOwnedRefreshSeed(t *testing.T) {
	f := New(t)
	ctx := context.Background()
	fc := f.Config
	c := store.Config{Host: fc.Host, Port: fc.Port, User: fc.User, Password: fc.Password, Database: fc.Database, Timeout: 5 * time.Second}
	if e := store.Migrate(ctx, c); e != nil {
		t.Fatal(e)
	}
	db, e := store.Open(ctx, c)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	s, e := db.Mirror(store.MirrorLimits{MaxRecords: 10, MaxBytes: 1 << 20})
	if e != nil {
		t.Fatal(e)
	}
	profile := store.RefreshConfig{SourceID: "source", AdmissionID: "admission-a", AccessEpoch: "access", ConfigEpoch: "config", PolicyClass: "synthetic", PolicyEpoch: "policy", Limits: refresh.Limits{CallerFillsPerWindow: 10, AdmissionFillsPerWindow: 100, FillWindow: time.Hour, MinFreshness: time.Millisecond, MaxFreshness: time.Hour, ActiveFills: 2, QueuedCallers: 10, AdmissionFills: 10, AdmissionWaiters: 100, FailureEntries: 100, WaitTimeout: time.Second, FillTimeout: time.Second, Cooldown: time.Second, FailureTTL: time.Minute, Refresh: mirror.RefreshPolicy{FullInventoryInterval: time.Hour}}, Native: store.NativeLimits{CallerPerWindow: 10, AdmissionPerWindow: 100, SourcePerWindow: 100, PerFill: 10, Concurrent: 5, Reservations: 100, Window: time.Hour}, PollInterval: time.Millisecond}
	gate, e := db.Refresh(s, profile, seedAuthority{})
	if e != nil {
		t.Fatal(e)
	}
	scope := mirror.Scope{Repository: mirror.RepositoryID{Provider: mirror.GitHub, Host: "forge.example", Path: "o/r"}, Kind: mirror.Issues, AdmissionID: profile.AdmissionID}
	d := refresh.Demand{Key: refresh.FillKey{Scope: scope, SourceID: profile.SourceID, AccessEpoch: profile.AccessEpoch, ConfigEpoch: profile.ConfigEpoch, PolicyClass: profile.PolicyClass, PolicyEpoch: profile.PolicyEpoch}, CallerID: "caller", Limits: profile.Limits, Now: time.Now()}
	if _, e = gate.Acquire(ctx, d); e == nil {
		t.Fatal("unseeded profile allowed")
	}
	SeedRefresh(t, f, profile, "caller")
	ticket, e := gate.Acquire(ctx, d)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = gate.BeforeAttempt(ctx, ticket.AttemptGrant); e != nil {
		t.Fatal(e)
	}
	profile.AdmissionID = "admission-b"
	SeedRefresh(t, f, profile, "caller-b")
	raw, e := f.ConnectOwned(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer raw.Close()
	var used int
	if e = raw.QueryRow(`SELECT http_used FROM public.refresh_budgets WHERE source_id=$1`, []byte(profile.SourceID)).Scan(&used); e != nil || used != 1 {
		t.Fatal("second synthetic admission reset shared source usage")
	}
	unowned := &Fixture{Config: f.Config}
	if conn, e := unowned.ConnectOwned(ctx); e == nil {
		conn.Close()
		t.Fatal("forged fixture capability connected")
	}
	second := New(t)
	saved := f.Config
	f.Config = second.Config
	if conn, e := f.ConnectOwned(ctx); e == nil {
		conn.Close()
		t.Fatal("foreign owned marker accepted")
	}
	f.Config = saved
}
