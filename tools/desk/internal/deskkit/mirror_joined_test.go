package deskkit

import (
	"context"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/refresh"
	"github.com/medici-finance/assay/workflow/pgtestfixture"
	"github.com/medici-finance/assay/workflow/store"
)

// The real native joined tests below use one canonical exported test harness;
// neither a second database fixture nor a local production gate is supplied.
type persistedMirrorFixture struct {
	db           *store.DB
	observations *store.MirrorStore
	gate         *store.RefreshGate
	config       store.RefreshConfig
	fixture      *pgtestfixture.Fixture
}

func persistedMirrorConfig(admission, source string) store.RefreshConfig {
	return store.RefreshConfig{AdmissionID: admission, SourceID: source, AccessEpoch: "access-1", ConfigEpoch: "config-1", PolicyClass: "synthetic", PolicyEpoch: "policy-1", Limits: refresh.Limits{CallerFillsPerWindow: 100, AdmissionFillsPerWindow: 1000, FillWindow: time.Hour, MinFreshness: time.Millisecond, MaxFreshness: time.Hour, ActiveFills: 5, QueuedCallers: 100, AdmissionFills: 50, AdmissionWaiters: 1000, FailureEntries: 100, WaitTimeout: 20 * time.Second, FillTimeout: 15 * time.Second, Cooldown: time.Second, FailureTTL: time.Minute, Refresh: mirror.RefreshPolicy{Overlap: time.Minute, FullInventoryInterval: time.Hour}}, Native: store.NativeLimits{CallerPerWindow: 100, AdmissionPerWindow: 1000, SourcePerWindow: 1000, PerFill: 20, Concurrent: 5, Reservations: 1000, Window: time.Hour}, PollInterval: time.Millisecond}
}
func newPersistedMirrorFixture(t *testing.T, c store.RefreshConfig, authority refresh.CallerAuthority, callers []string) persistedMirrorFixture {
	t.Helper()
	f := pgtestfixture.New(t)
	cfg := f.Config
	connection := store.Config{Host: cfg.Host, Port: cfg.Port, User: cfg.User, Password: cfg.Password, Database: cfg.Database, Timeout: 5 * time.Second}
	if e := store.Migrate(context.Background(), connection); e != nil {
		t.Fatal(e)
	}
	db, e := store.Open(context.Background(), connection)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	observations, e := db.Mirror(store.MirrorLimits{MaxRecords: 128, MaxBytes: 1 << 20})
	if e != nil {
		t.Fatal(e)
	}
	gate, e := db.Refresh(observations, c, authority)
	if e != nil {
		t.Fatal(e)
	}
	pgtestfixture.SeedRefresh(t, f, c, callers...)
	return persistedMirrorFixture{db, observations, gate, c, f}
}
