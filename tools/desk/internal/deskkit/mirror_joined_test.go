package deskkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/adapters"
	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/mirror/mirrortest"
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
	return store.RefreshConfig{AdmissionID: admission, SourceID: source, AccessEpoch: "access-1", ConfigEpoch: "config-1", PolicyClass: "synthetic", PolicyEpoch: "policy-1", Limits: refresh.Limits{CallerFillsPerWindow: 100, AdmissionFillsPerWindow: 1000, FillWindow: time.Hour, MinFreshness: time.Nanosecond, MaxFreshness: time.Hour, ActiveFills: 5, QueuedCallers: 100, AdmissionFills: 50, AdmissionWaiters: 1000, FailureEntries: 100, WaitTimeout: 45 * time.Second, FillTimeout: 30 * time.Second, Cooldown: time.Second, FailureTTL: time.Minute, Refresh: mirror.RefreshPolicy{Overlap: time.Minute, FullInventoryInterval: time.Nanosecond}}, Native: store.NativeLimits{CallerPerWindow: 100, AdmissionPerWindow: 1000, SourcePerWindow: 1000, PerFill: 20, Concurrent: 5, Reservations: 1000, Window: time.Hour}, PollInterval: 25 * time.Millisecond}
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

type persistedClock struct{}

func (persistedClock) Now() time.Time { return time.Now().UTC() }

type persistedCallerAuthority struct {
	config     store.RefreshConfig
	repository mirror.RepositoryID
	denied     atomic.Bool
}

func (a *persistedCallerAuthority) Authorize(ctx context.Context, c refresh.Caller, s mirror.Scope) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	p := a.config
	if a.denied.Load() || c.ID == "" || c.AdmissionID != p.AdmissionID || s.AdmissionID != p.AdmissionID || s.Repository != a.repository || c.AccessEpoch != p.AccessEpoch || c.ConfigEpoch != p.ConfigEpoch || c.PolicyClass != p.PolicyClass || c.PolicyEpoch != p.PolicyEpoch || c.Limits != p.Limits {
		return mirror.ErrPermission
	}
	return nil
}

type persistedObserver struct{ scans atomic.Int64 }

func (o *persistedObserver) Observe(ctx context.Context, v refresh.Observation) error {
	if v.Kind == "source-attempt" && v.Unit == "scan-call" && v.Known {
		o.scans.Add(v.Count)
	}
	return ctx.Err()
}

type persistedNative struct {
	persistedMirrorFixture
	source      mirror.Source
	reader      *MirrorReadSource
	scope       mirror.Scope
	admission   *nativeAdmission
	authority   *persistedCallerAuthority
	coordinator *refresh.Coordinator
	observer    *persistedObserver
	client      *http.Client
	requests    *atomic.Int64
}

func persistedCaller(c store.RefreshConfig, id string) refresh.Caller {
	return refresh.Caller{ID: id, AdmissionID: c.AdmissionID, AccessEpoch: c.AccessEpoch, ConfigEpoch: c.ConfigEpoch, PolicyClass: c.PolicyClass, PolicyEpoch: c.PolicyEpoch, Limits: c.Limits}
}
func newPersistedNative(t *testing.T, provider ForgeKind, kind mirror.RecordKind, handler http.HandlerFunc, callers []string, options ...func(*store.RefreshConfig)) persistedNative {
	t.Helper()
	requests := &atomic.Int64{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	client := server.Client()
	transport := client.Transport.(*http.Transport)
	t.Cleanup(transport.CloseIdleConnections)
	admission := &nativeAdmission{}
	admission.allowed.Store(true)
	profile := persistedMirrorConfig(admission.ID(), "persisted-native-source")
	for _, option := range options {
		option(&profile)
	}
	authority := &persistedCallerAuthority{config: profile}
	sql := newPersistedMirrorFixture(t, profile, authority, callers)
	var forge Forge
	if provider == ForgeGitHub {
		forge = &GitHubForge{Token: ciFixtureToken, BaseURL: server.URL, Client: client, onlyRepo: "o/r"}
	} else {
		forge = &GitLabForge{Token: glTestToken, BaseURL: server.URL, Client: client, onlyRepo: "o/r"}
	}
	reader, e := NewMirrorReadSource(ReadOnly(forge), ForgeResolution{Kind: provider, Repo: ForgeRepo{Owner: "o", Name: "r"}})
	if e != nil {
		t.Fatal(e)
	}
	source, e := NewMirrorIngestSource(reader, admission.ID(), profile.SourceID, persistedClock{}.Now, sql.gate)
	if e != nil {
		t.Fatal(e)
	}
	scope := mirror.Scope{Repository: source.(*mirrorIngestSource).repository, Kind: kind, AdmissionID: admission.ID()}
	authority.repository = scope.Repository
	lifetime, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	observer := &persistedObserver{}
	coordinator, e := refresh.New(lifetime, sql.observations, source, sql.gate, persistedClock{}, profile.SourceID, authority, observer)
	if e != nil {
		t.Fatal(e)
	}
	return persistedNative{sql, source, reader, scope, admission, authority, coordinator, observer, client, requests}
}
func joinedRead(t *testing.T, f persistedNative, id string, maxAge time.Duration) (refresh.Result, error) {
	t.Helper()
	return f.coordinator.Read(context.Background(), f.admission, refresh.Query{Query: mirror.Query{Scope: f.scope, MaxAge: maxAge}}, persistedCaller(f.config, id))
}

// regression: F-persisted-native-read-through
func TestMirrorPersistedReadThroughJoin(t *testing.T) {
	if os.Getenv("ASSAY_TEST_POSTGRES_DSN") == "" {
		t.Skip("explicit disposable PostgreSQL container required; this skip is not integration proof")
	}
	t.Run("shared-failure-cache", checkPersistedFailures)
	t.Run("changed-empty", checkPersistedChangedEmpty)
	t.Run("cache-admission", checkPersistedCacheAdmission)
	t.Run("data-age", checkPersistedDataAge)
	t.Run("admission-isolation", checkPersistedAdmissionIsolation)
	t.Run("requirement-isolation", checkPersistedRequirementIsolation)
	t.Run("lower-native-budget", checkPersistedLowerBudget)
	for _, provider := range []ForgeKind{ForgeGitHub, ForgeGitLab} {
		t.Run("cohort-"+string(provider), func(t *testing.T) {
			stamp := time.Now().UTC().Add(-time.Minute)
			callers := make([]string, 50)
			for i := range callers {
				callers[i] = fmt.Sprintf("caller-%02d", i)
			}
			f := newPersistedNative(t, provider, mirror.Issues, func(w http.ResponseWriter, r *http.Request) {
				page := r.URL.Query().Get("page")
				number := 1
				if page == "" || page == "1" {
					if provider == ForgeGitHub {
						w.Header().Set("Link", fmt.Sprintf(`<https://%s/repos/o/r/issues?page=2>; rel="next"`, r.Host))
					} else {
						w.Header().Set("X-Next-Page", "2")
					}
				} else {
					number = 2
				}
				_, _ = io.WriteString(w, "["+ingestRow(provider, mirror.Issues, number, stamp, number == 1)+"]")
			}, callers)
			start := make(chan struct{})
			type answer struct {
				result refresh.Result
				err    error
			}
			answers := make(chan answer, len(callers))
			var group sync.WaitGroup
			for _, id := range callers {
				group.Add(1)
				go func(id string) {
					defer group.Done()
					<-start
					r, e := joinedRead(t, f, id, time.Hour)
					answers <- answer{r, e}
				}(id)
			}
			close(start)
			group.Wait()
			close(answers)
			for answer := range answers {
				r := answer.result
				if answer.err != nil || !r.Fulfilled || !r.Available || r.Stale || len(r.Snapshot.Records) != 2 {
					t.Fatalf("real persisted cohort unavailable: fulfilled=%v available=%v records=%d error=%v", r.Fulfilled, r.Available, len(r.Snapshot.Records), answer.err)
				}
				if !r.Snapshot.Records[0].BodyKnown || r.Snapshot.Records[0].Labels == nil || r.Snapshot.Records[1].BodyKnown || r.Snapshot.Records[1].Labels != nil {
					t.Fatal("native mapping/SQL field presence flattened")
				}
			}
			if f.requests.Load() != 2 || f.observer.scans.Load() != 1 {
				t.Fatalf("cohort repeated native work: base entries=%d Source scans=%d", f.requests.Load(), f.observer.scans.Load())
			}
			warm, e := joinedRead(t, f, callers[0], time.Hour)
			if e != nil || !warm.Fulfilled || f.requests.Load() != 2 {
				t.Fatal("warm query repeated provider work", e)
			}
			sql, e := f.fixture.ConnectOwned(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			defer sql.Close()
			var measured, charged int64
			if e = sql.QueryRow(`SELECT coalesce(sum(http_count),0) FROM public.refresh_attempts WHERE observed AND http_known`).Scan(&measured); e != nil {
				t.Fatal(e)
			}
			if e = sql.QueryRow(`SELECT http_used FROM public.refresh_budgets WHERE source_id=$1`, []byte(f.config.SourceID)).Scan(&charged); e != nil || measured != 2 || charged != 2 {
				t.Fatal("SQL accounting confused one Scan with its native HTTP entries")
			}
			// A fresh coordinator has no process cache to carry the warm result.
			restarted, e := refresh.New(context.Background(), f.observations, f.source, f.gate, persistedClock{}, f.config.SourceID, f.authority)
			if e != nil {
				t.Fatal(e)
			}
			again, e := restarted.Read(context.Background(), f.admission, refresh.Query{Query: mirror.Query{Scope: f.scope, MaxAge: time.Hour}}, persistedCaller(f.config, callers[1]))
			if e != nil || !again.Fulfilled || f.requests.Load() != 2 {
				t.Fatal("new coordinator lost persistent warm observation", e)
			}
		})
	}
}

func independentPersistedCoordinator(t *testing.T, f persistedNative) (*refresh.Coordinator, *persistedObserver) {
	t.Helper()
	c := f.fixture.Config
	db, e := store.Open(context.Background(), store.Config{Host: c.Host, Port: c.Port, User: c.User, Password: c.Password, Database: c.Database, Timeout: 5 * time.Second})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	observations, e := db.Mirror(store.MirrorLimits{MaxRecords: 128, MaxBytes: 1 << 20})
	if e != nil {
		t.Fatal(e)
	}
	authority := &persistedCallerAuthority{config: f.config, repository: f.scope.Repository}
	gate, e := db.Refresh(observations, f.config, authority)
	if e != nil {
		t.Fatal(e)
	}
	source, e := NewMirrorIngestSource(f.reader, f.admission.ID(), f.config.SourceID, persistedClock{}.Now, gate)
	if e != nil {
		t.Fatal(e)
	}
	lifetime, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	observer := &persistedObserver{}
	coordinator, e := refresh.New(lifetime, observations, source, gate, persistedClock{}, f.config.SourceID, authority, observer)
	if e != nil {
		t.Fatal(e)
	}
	return coordinator, observer
}
func persistedRetry(t *testing.T, f persistedNative) *time.Time {
	t.Helper()
	db, e := f.fixture.ConnectOwned(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var retry *time.Time
	if e = db.QueryRow(`SELECT retry_at FROM public.refresh_fills`).Scan(&retry); e != nil {
		t.Fatal(e)
	}
	return retry
}
func checkPersistedFailures(t *testing.T) {
	for _, mode := range []string{"partial", "oversize", "transport", "rate-floor", "unknown-retry"} {
		t.Run(mode, func(t *testing.T) {
			var failed atomic.Bool
			var dials atomic.Int64
			stamp := time.Now().UTC().Add(-time.Minute)
			f := newPersistedNative(t, ForgeGitHub, mirror.Issues, func(w http.ResponseWriter, r *http.Request) {
				if !failed.Load() {
					_, _ = io.WriteString(w, "["+ingestRow(ForgeGitHub, mirror.Issues, 1, stamp, true)+"]")
					return
				}
				switch mode {
				case "partial":
					if r.URL.Query().Get("page") == "2" {
						w.WriteHeader(503)
						_, _ = io.WriteString(w, `{"message":"synthetic unavailable"}`)
					} else {
						w.Header().Set("Link", fmt.Sprintf(`<https://%s/repos/o/r/issues?page=2>; rel="next"`, r.Host))
						_, _ = io.WriteString(w, "["+ingestRow(ForgeGitHub, mirror.Issues, 2, stamp, true)+"]")
					}
				case "oversize":
					_, _ = io.WriteString(w, strings.Repeat(" ", int(adapters.MaxUpdateBytes)+2))
				case "rate-floor":
					w.Header().Set("Retry-After", "20")
					w.WriteHeader(429)
					_, _ = io.WriteString(w, `{"message":"synthetic rate limit"}`)
				case "unknown-retry":
					w.WriteHeader(429)
					_, _ = io.WriteString(w, `{"message":"synthetic rate limit"}`)
				default:
					t.Error("transport failure unexpectedly reached server")
				}
			}, []string{"caller-a", "caller-b"}, func(c *store.RefreshConfig) { c.Limits.Cooldown = 10 * time.Second })
			first, e := joinedRead(t, f, "caller-a", time.Hour)
			if e != nil || !first.Fulfilled {
				t.Fatal("initial native population unavailable", e)
			}
			before := first.Snapshot
			failed.Store(true)
			if mode == "transport" {
				transport := f.client.Transport.(*http.Transport).Clone()
				transport.DisableKeepAlives = true
				transport.DialContext = func(context.Context, string, string) (net.Conn, error) {
					dials.Add(1)
					return nil, errors.New("synthetic transport unavailable")
				}
				f.client.Transport = transport
				t.Cleanup(transport.CloseIdleConnections)
			}
			result, e := joinedRead(t, f, "caller-a", time.Nanosecond)
			if e != nil || result.Fulfilled || result.Available {
				t.Fatalf("failed refresh claimed fulfilment: %v %+v", e, result.Result.Snapshot.Coverage)
			}
			after, e := f.observations.Read(context.Background(), f.admission, f.scope)
			if e != nil {
				t.Fatal(e)
			}
			if !after.Coverage.LastComplete.Equal(before.Coverage.LastComplete) || !after.Coverage.LastInventory.Equal(before.Coverage.LastInventory) || !after.Coverage.ObservedUpTo.Equal(before.Coverage.ObservedUpTo) || len(after.Records) != len(before.Records) || after.Records[0].Title != before.Records[0].Title {
				t.Fatal("failed source promoted watermarks or erased prior facts")
			}
			requests := f.requests.Load()
			beforeRetry := persistedRetry(t, f)
			if mode == "rate-floor" && (beforeRetry == nil || beforeRetry.Before(time.Now().Add(15*time.Second))) {
				t.Fatal("provider floor lost behind fixed cooldown")
			}
			if mode == "unknown-retry" && !result.RetryAt.IsZero() {
				t.Fatal("unknown retry timing became fixed eligibility")
			}
			other, observer := independentPersistedCoordinator(t, f)
			for range 2 {
				again, _ := other.Read(context.Background(), f.admission, refresh.Query{Query: mirror.Query{Scope: f.scope, MaxAge: time.Nanosecond}}, persistedCaller(f.config, "caller-b"))
				if again.Fulfilled || again.Available {
					t.Fatal("independent coordinator ignored shared failure")
				}
			}
			afterRetry := persistedRetry(t, f)
			if (beforeRetry == nil) != (afterRetry == nil) || beforeRetry != nil && !beforeRetry.Equal(*afterRetry) {
				t.Fatal("shared failure retry slid on another caller")
			}
			if f.requests.Load() != requests || observer.scans.Load() != 0 || mode == "transport" && dials.Load() != 1 {
				t.Fatal("independent coordinator repeated cached failure work")
			}
		})
	}
}

func titledJoinedRow(title string, stamp time.Time) string {
	var row map[string]json.RawMessage
	if json.Unmarshal([]byte(ingestRow(ForgeGitHub, mirror.Issues, 1, stamp, true)), &row) != nil {
		panic("invalid synthetic row")
	}
	row["title"], _ = json.Marshal(title)
	body, e := json.Marshal(row)
	if e != nil {
		panic("invalid synthetic row")
	}
	return string(body)
}
func checkPersistedChangedEmpty(t *testing.T) {
	t.Run("complete-empty", func(t *testing.T) {
		f := newPersistedNative(t, ForgeGitHub, mirror.Issues, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "[]") }, []string{"caller-a"})
		r, e := joinedRead(t, f, "caller-a", time.Hour)
		if e != nil || !r.Fulfilled || !r.Available || len(r.Snapshot.Records) != 0 || r.Snapshot.Coverage.LastComplete.IsZero() || f.requests.Load() != 1 {
			t.Fatal("real complete-empty native inventory unavailable", e)
		}
	})
	t.Run("changed-and-empty-inventory", func(t *testing.T) {
		var mode atomic.Int64
		stamp := time.Now().UTC().Add(-time.Minute)
		f := newPersistedNative(t, ForgeGitHub, mirror.Issues, func(w http.ResponseWriter, _ *http.Request) {
			if mode.Load() == 2 {
				_, _ = io.WriteString(w, "[]")
				return
			}
			title := "synthetic-original"
			if mode.Load() == 1 {
				title = "synthetic-changed"
			}
			_, _ = io.WriteString(w, "["+titledJoinedRow(title, stamp)+"]")
		}, []string{"caller-a"}, func(c *store.RefreshConfig) { c.Limits.Cooldown = time.Millisecond })
		first, e := joinedRead(t, f, "caller-a", time.Hour)
		if e != nil || !first.Fulfilled {
			t.Fatal(e)
		}
		mode.Store(1)
		_, _ = joinedRead(t, f, "caller-a", time.Nanosecond)
		changed, e := joinedRead(t, f, "caller-a", time.Hour)
		if e != nil || !changed.Fulfilled || changed.Snapshot.Records[0].Title != "synthetic-changed" || !changed.Snapshot.Coverage.LastComplete.After(first.Snapshot.Coverage.LastComplete) || f.requests.Load() != 2 {
			t.Fatal("stale data failed to trigger real source owner", e)
		}
		mode.Store(2)
		time.Sleep(5 * time.Millisecond)
		_, _ = joinedRead(t, f, "caller-a", time.Nanosecond)
		empty, e := joinedRead(t, f, "caller-a", time.Hour)
		if e != nil || !empty.Fulfilled || len(empty.Snapshot.Records) != 1 || empty.Snapshot.Records[0].InInventory || empty.Snapshot.Records[0].State != "open" || !empty.Snapshot.Coverage.LastInventory.After(changed.Snapshot.Coverage.LastInventory) || f.requests.Load() != 3 {
			t.Fatal("qualified empty inventory fabricated closure/deletion or lost coverage", e)
		}
	})
}
func checkPersistedCacheAdmission(t *testing.T) {
	t.Run("opaque-cache-replay", func(t *testing.T) {
		stamp := time.Now().UTC().Add(-time.Minute)
		f := newPersistedNative(t, ForgeGitHub, mirror.Issues, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "["+titledJoinedRow("synthetic-observed", stamp)+"]")
		}, []string{"caller-a"})
		first, e := joinedRead(t, f, "caller-a", time.Hour)
		if e != nil || !first.Fulfilled {
			t.Fatal(e)
		}
		original := f.client.Transport
		cache := &nativeCache{base: original, body: "[" + titledJoinedRow("synthetic-replayed", stamp) + "]"}
		f.client.Transport = countedTransport{next: cache}
		again, _ := joinedRead(t, f, "caller-a", time.Nanosecond)
		if again.Fulfilled || again.Available || f.requests.Load() != 1 || cache.calls.Load() != 0 || !again.Snapshot.Coverage.LastComplete.Equal(first.Snapshot.Coverage.LastComplete) || again.Snapshot.Records[0].Title != "synthetic-observed" {
			t.Fatal("opaque cached replay fabricated fresh SQL observations")
		}
	})
	t.Run("current-caller-and-admission", func(t *testing.T) {
		stamp := time.Now().UTC().Add(-time.Minute)
		f := newPersistedNative(t, ForgeGitHub, mirror.Issues, func(w http.ResponseWriter, _ *http.Request) {
			row := titledJoinedRow("synthetic-observed", stamp)
			var body map[string]json.RawMessage
			_ = json.Unmarshal([]byte(row), &body)
			body["body"] = json.RawMessage(`"synthetic-observation-body"`)
			raw, _ := json.Marshal(body)
			_, _ = w.Write(append(append([]byte{'['}, raw...), ']'))
		}, []string{"caller-a"})
		first, e := joinedRead(t, f, "caller-a", time.Hour)
		if e != nil || !first.Fulfilled || first.Snapshot.Records[0].Body != "synthetic-observation-body" {
			t.Fatal("body fixture not populated", e)
		}
		stale := persistedCaller(f.config, "caller-a")
		stale.AccessEpoch = "stale-epoch"
		denied, e := f.coordinator.Read(context.Background(), f.admission, refresh.Query{Query: mirror.Query{Scope: f.scope, MaxAge: time.Hour}}, stale)
		if e == nil || len(denied.Snapshot.Records) != 0 || f.requests.Load() != 1 {
			t.Fatal("stale caller epoch exposed copied body")
		}
		late, e := refresh.New(context.Background(), f.observations, f.source, f.gate, persistedClock{}, f.config.SourceID, f.authority, denyPersistedCacheRelease{f.authority})
		if e != nil {
			t.Fatal(e)
		}
		denied, e = late.Read(context.Background(), f.admission, refresh.Query{Query: mirror.Query{Scope: f.scope, MaxAge: time.Hour}}, persistedCaller(f.config, "caller-a"))
		if e == nil || len(denied.Snapshot.Records) != 0 {
			t.Fatal("post-store observer revocation exposed copied body")
		}
		f.authority.denied.Store(false)
		f.authority.denied.Store(true)
		denied, e = joinedRead(t, f, "caller-a", time.Hour)
		if e == nil || len(denied.Snapshot.Records) != 0 {
			t.Fatal("current caller refusal exposed copied body")
		}
		f.authority.denied.Store(false)
		f.admission.allowed.Store(false)
		denied, e = joinedRead(t, f, "caller-a", time.Hour)
		if e == nil || len(denied.Snapshot.Records) != 0 || f.requests.Load() != 1 {
			t.Fatal("lost current admission exposed copied body or contacted source")
		}
		direct, e := f.observations.Read(context.Background(), f.admission, f.scope)
		if e == nil || len(direct.Records) != 0 {
			t.Fatal("direct Store bypass exposed copied body after admission loss")
		}
	})
}

type delayedPersistedObservation struct {
	mirror.AttemptAuthority
	entered atomic.Int64
	delay   time.Duration
}

func (a *delayedPersistedObservation) ObserveAttempt(ctx context.Context, r mirror.AttemptReservation, o mirror.AttemptObservation) error {
	a.entered.CompareAndSwap(0, time.Now().UTC().UnixNano())
	timer := time.NewTimer(a.delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	return a.AttemptAuthority.ObserveAttempt(ctx, r, o)
}
func checkPersistedDataAge(t *testing.T) {
	f := newPersistedNative(t, ForgeGitHub, mirror.Issues, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "[]") }, []string{"caller-a"})
	authority := &delayedPersistedObservation{AttemptAuthority: f.gate, delay: 150 * time.Millisecond}
	source, e := NewMirrorIngestSource(f.reader, f.admission.ID(), f.config.SourceID, persistedClock{}.Now, authority)
	if e != nil {
		t.Fatal(e)
	}
	f.coordinator, e = refresh.New(context.Background(), f.observations, source, f.gate, persistedClock{}, f.config.SourceID, f.authority)
	if e != nil {
		t.Fatal(e)
	}
	r, e := joinedRead(t, f, "caller-a", 50*time.Millisecond)
	entered := time.Unix(0, authority.entered.Load()).UTC()
	if e != nil || r.Fulfilled || r.Available || r.Snapshot.Coverage.State != mirror.Complete || r.Snapshot.Coverage.LastComplete.IsZero() || r.Snapshot.Coverage.LastComplete.After(entered) || f.requests.Load() != 1 {
		t.Fatal("actual observation I/O inflated already-read empty data freshness", e)
	}
}

func checkPersistedAdmissionIsolation(t *testing.T) {
	var second atomic.Bool
	stamp := time.Now().UTC().Add(-time.Minute)
	f := newPersistedNative(t, ForgeGitHub, mirror.Issues, func(w http.ResponseWriter, _ *http.Request) {
		title := "synthetic-admission-A"
		if second.Load() {
			title = "synthetic-admission-B"
		}
		_, _ = io.WriteString(w, "["+titledJoinedRow(title, stamp)+"]")
	}, []string{"caller-a"})
	first, e := joinedRead(t, f, "caller-a", time.Hour)
	if e != nil || !first.Fulfilled {
		t.Fatal(e)
	}
	second.Store(true)
	admission := &mirrortest.TestAdmission{Name: "other-synthetic-admission", Allowed: true}
	profile := f.config
	profile.AdmissionID = admission.ID()
	pgtestfixture.SeedRefresh(t, f.fixture, profile, "caller-b")
	authority := &persistedCallerAuthority{config: profile, repository: f.scope.Repository}
	gate, e := f.db.Refresh(f.observations, profile, authority)
	if e != nil {
		t.Fatal(e)
	}
	source, e := NewMirrorIngestSource(f.reader, admission.ID(), profile.SourceID, persistedClock{}.Now, gate)
	if e != nil {
		t.Fatal(e)
	}
	coordinator, e := refresh.New(context.Background(), f.observations, source, gate, persistedClock{}, profile.SourceID, authority)
	if e != nil {
		t.Fatal(e)
	}
	scope := f.scope
	scope.AdmissionID = admission.ID()
	other, e := coordinator.Read(context.Background(), admission, refresh.Query{Query: mirror.Query{Scope: scope, MaxAge: time.Hour}}, persistedCaller(profile, "caller-b"))
	if e != nil || !other.Fulfilled || other.Snapshot.Records[0].Title != "synthetic-admission-B" || f.requests.Load() != 2 {
		t.Fatal("independent admission reused another admission's copied observations", e)
	}
	original, e := joinedRead(t, f, "caller-a", time.Hour)
	if e != nil || !original.Fulfilled || original.Snapshot.Records[0].Title != "synthetic-admission-A" || f.requests.Load() != 2 {
		t.Fatal("other admission overwrote original scope", e)
	}
	denied, e := f.observations.Read(context.Background(), admission, f.scope)
	if e == nil || len(denied.Records) != 0 {
		t.Fatal("direct Store crossed admission boundary")
	}
}
func checkPersistedRequirementIsolation(t *testing.T) {
	for _, strongOwner := range []bool{false, true} {
		t.Run(fmt.Sprint("strong-owner=", strongOwner), func(t *testing.T) {
			started := make(chan struct{})
			release := make(chan struct{})
			var once, startOnce sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			stamp := time.Now().UTC().Add(-time.Minute)
			f := newPersistedNative(t, ForgeGitHub, mirror.Changes, func(w http.ResponseWriter, r *http.Request) {
				startOnce.Do(func() { close(started) })
				select {
				case <-release:
				case <-r.Context().Done():
					return
				}
				_, _ = io.WriteString(w, "["+ingestRow(ForgeGitHub, mirror.Changes, 1, stamp, true)+"]")
			}, []string{"owner", "joined"}, func(c *store.RefreshConfig) { c.Limits.Cooldown = 10 * time.Second })
			t.Cleanup(unblock)
			read := func(id string, reviews bool) (refresh.Result, error) {
				return f.coordinator.Read(context.Background(), f.admission, refresh.Query{Query: mirror.Query{Scope: f.scope, MaxAge: time.Hour}, RequireReviews: reviews}, persistedCaller(f.config, id))
			}
			type answer struct {
				r refresh.Result
				e error
			}
			ownerResult := make(chan answer, 1)
			joinedResult := make(chan answer, 1)
			go func() { r, e := read("owner", strongOwner); ownerResult <- answer{r, e} }()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("native owner did not start")
			}
			go func() { r, e := read("joined", !strongOwner); joinedResult <- answer{r, e} }()
			raw, e := f.fixture.ConnectOwned(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			defer raw.Close()
			deadline := time.Now().Add(5 * time.Second)
			for {
				var waiters int
				if e = raw.QueryRow(`SELECT count(*) FROM public.refresh_waiters WHERE NOT released`).Scan(&waiters); e != nil {
					t.Fatal(e)
				}
				if waiters == 2 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("strong/base query did not durably join")
				}
				time.Sleep(time.Millisecond)
			}
			unblock()
			owner, joined := <-ownerResult, <-joinedResult
			if owner.e != nil || joined.e != nil || f.requests.Load() != 1 {
				t.Fatal("native requirement cohort failed or repeated source", owner.e, joined.e)
			}
			strong, base := joined.r, owner.r
			strongID, baseID := "joined", "owner"
			if strongOwner {
				strong, base = owner.r, joined.r
				strongID, baseID = "owner", "joined"
			}
			if strong.Fulfilled || strong.RetryAt.IsZero() {
				t.Fatal("unfetched Reviews claimed fulfilment or lost backoff")
			}
			if strongOwner {
				if base.Fulfilled || !base.Snapshot.Coverage.LastComplete.IsZero() {
					t.Fatal("review-only partial upgraded LastComplete")
				}
			} else if !base.Fulfilled {
				t.Fatal("strong joined requirement poisoned complete base owner")
			}
			repeat, _ := read(strongID, true)
			if repeat.Fulfilled || f.requests.Load() != 1 {
				t.Fatal("strong immediate retry ignored shared requirement failure")
			}
			refreshed, e := read(baseID, false)
			want := int64(1)
			if strongOwner {
				want = 2
			}
			if e != nil || !refreshed.Fulfilled || f.requests.Load() != want {
				t.Fatal("strong-owner gap blocked the one legitimate base refetch", e)
			}
			_, _ = read(strongID, true)
			if f.requests.Load() != want {
				t.Fatal("base recovery erased stronger backoff")
			}
		})
	}
}

type denyPersistedCacheRelease struct{ authority *persistedCallerAuthority }

func (o denyPersistedCacheRelease) Observe(ctx context.Context, v refresh.Observation) error {
	if v.Kind == "cache-hit" {
		o.authority.denied.Store(true)
	}
	return ctx.Err()
}
func checkPersistedLowerBudget(t *testing.T) {
	stamp := time.Now().UTC().Add(-time.Minute)
	f := newPersistedNative(t, ForgeGitHub, mirror.Issues, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "2" {
			w.Header().Set("Link", fmt.Sprintf(`<https://%s/repos/o/r/issues?page=2>; rel="next"`, r.Host))
		}
		_, _ = io.WriteString(w, "["+titledJoinedRow("synthetic-budgeted", stamp)+"]")
	}, []string{"caller-a"}, func(c *store.RefreshConfig) { c.Native.CallerPerWindow = 1 })
	r, e := joinedRead(t, f, "caller-a", time.Hour)
	if e != nil || r.Fulfilled || r.Available || f.requests.Load() != 1 || !r.Snapshot.Coverage.LastComplete.IsZero() {
		t.Fatal("actual native pagination bypassed lower SQL caller HTTP ceiling", e)
	}
	raw, e := f.fixture.ConnectOwned(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	defer raw.Close()
	var charged int
	if e = raw.QueryRow(`SELECT http_used FROM public.refresh_budgets WHERE source_id=$1`, []byte(f.config.SourceID)).Scan(&charged); e != nil || charged != 1 {
		t.Fatal("refused second native entry consumed or invented allowance")
	}
}
