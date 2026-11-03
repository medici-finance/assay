package deskkit

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/adapters"
	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/mirror/mirrortest"
	"github.com/medici-finance/assay/forgeread/refresh"
)

type nativeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *nativeClock) Now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *nativeClock) add(d time.Duration) { c.mu.Lock(); c.now = c.now.Add(d); c.mu.Unlock() }

// A stateful authority fixture, never a production quota implementation.
type nativeAuthority struct {
	mu                       sync.Mutex
	clock                    *nativeClock
	grant                    mirror.AttemptGrant
	grants                   map[string]mirror.AttemptGrant
	serial, before, observed int64
	pending                  map[string]mirror.AttemptReservation
	reports                  map[string]mirror.AttemptObservation
	floor                    time.Time
	unknown                  bool
	capacity                 int
	beforeErr, observeErr    bool
	deny                     mirror.AttemptHold
	normalizeReservation     bool
	currentGeneration        string
	reportDeadline           time.Time
	afterObserve             func()
	afterBefore              func()
}

func (a *nativeAuthority) BeforeAttempt(ctx context.Context, g mirror.AttemptGrant) (mirror.AttemptReservation, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.before++
	if err := ctx.Err(); err != nil {
		return mirror.AttemptReservation{}, err
	}
	if a.beforeErr {
		a.serial++
		r := mirror.AttemptReservation{ID: fmt.Sprintf("uncertain-%d", a.serial), Grant: g}
		a.pending[r.ID] = r
		a.unknown = true
		return mirror.AttemptReservation{}, errors.New("synthetic-authority-detail")
	}
	if a.deny != "" {
		return mirror.AttemptReservation{}, &mirror.AttemptError{Hold: a.deny}
	}
	expected, currentGeneration := a.grant, a.currentGeneration
	if a.grants != nil {
		expected = a.grants[g.ID]
		currentGeneration = expected.Generation
	}
	if !g.Equal(expected) || g.Validate(a.clock.Now()) != nil || g.Generation != currentGeneration || a.unknown || a.clock.Now().Before(a.floor) || a.serial >= g.MaxHTTPAttempts || len(a.pending) >= a.capacity {
		return mirror.AttemptReservation{}, &mirror.AttemptError{Hold: mirror.AttemptSourceHeld}
	}
	a.serial++
	r := mirror.AttemptReservation{ID: fmt.Sprintf("attempt-%d", a.serial), Grant: g}
	if a.normalizeReservation {
		r.Grant.ExpiresAt = r.Grant.ExpiresAt.In(time.FixedZone("alternate-utc", 0))
	}
	a.pending[r.ID] = r
	if a.afterBefore != nil {
		a.afterBefore()
	}
	return r, nil
}
func (a *nativeAuthority) ObserveAttempt(ctx context.Context, r mirror.AttemptReservation, o mirror.AttemptObservation) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.reportDeadline, _ = ctx.Deadline()
	if err := ctx.Err(); err != nil {
		return err
	}
	if a.observeErr {
		a.unknown = true
		return errors.New("synthetic-observation-detail")
	}
	if p, ok := a.pending[r.ID]; !ok || p.ID != r.ID || !p.Grant.Equal(r.Grant) || o.HTTP.Value < 0 || o.HTTP.Value > 1 || o.DecodedBytes.Value < 0 || o.ObservedAt.IsZero() {
		return &mirror.AttemptError{Hold: mirror.AttemptUnknownHeld}
	}
	if o.Retry.NotBefore.After(a.floor) {
		a.floor = o.Retry.NotBefore
	}
	if o.Retry.DelayKnown && o.Retry.Delay > 0 {
		if floor := a.clock.Now().Add(o.Retry.Delay); floor.After(a.floor) {
			a.floor = floor
		}
	}
	if o.Retry.Knowledge == mirror.RetryUnknown || !o.HTTP.Known {
		a.unknown = true
	}
	a.observed++
	a.reports[r.ID] = o
	delete(a.pending, r.ID)
	if a.afterObserve != nil {
		a.afterObserve()
	}
	return nil
}

type nativeFixture struct {
	source    *mirrorIngestSource
	scope     mirror.Scope
	admission *nativeAdmission
	clock     *nativeClock
	authority *nativeAuthority
	plan      mirror.ScanPlan
	requests  atomic.Int64
	client    *http.Client
}
type nativeAdmission struct{ allowed atomic.Bool }

func (*nativeAdmission) ID() string { return "native-admission" }
func (a *nativeAdmission) Authorize(context.Context, mirror.RepositoryID) error {
	if !a.allowed.Load() {
		return mirror.ErrPermission
	}
	return nil
}

func newNativeFixture(t *testing.T, provider ForgeKind, kind mirror.RecordKind, handler http.HandlerFunc, repositories ...ForgeRepo) *nativeFixture {
	t.Helper()
	f := &nativeFixture{clock: &nativeClock{now: time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)}, admission: &nativeAdmission{}}
	f.admission.allowed.Store(true)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { f.requests.Add(1); handler(w, r) }))
	t.Cleanup(server.Close)
	f.client = server.Client()
	repo := ForgeRepo{Owner: "o", Name: "r"}
	if len(repositories) > 0 {
		repo = repositories[0]
	}
	var forge Forge
	if provider == ForgeGitHub {
		forge = &GitHubForge{Token: ciFixtureToken, BaseURL: server.URL, Client: f.client, onlyRepo: repo.Slug()}
	} else {
		forge = &GitLabForge{Token: glTestToken, BaseURL: server.URL, Client: f.client, onlyRepo: repo.Slug()}
	}
	reader, err := NewMirrorReadSource(ReadOnly(forge), ForgeResolution{Kind: provider, Repo: repo})
	if err != nil {
		t.Fatal(err)
	}
	f.authority = &nativeAuthority{clock: f.clock, pending: map[string]mirror.AttemptReservation{}, reports: map[string]mirror.AttemptObservation{}, capacity: 4, currentGeneration: "generation-1"}
	source, err := NewMirrorIngestSource(reader, f.admission.ID(), "native-source", f.clock.Now, f.authority)
	if err != nil {
		t.Fatal(err)
	}
	f.source = source.(*mirrorIngestSource)
	f.scope = mirror.Scope{Repository: f.source.repository, Kind: kind, AdmissionID: f.admission.ID()}
	f.plan, err = mirror.PlanRefresh(mirror.Snapshot{Scope: f.scope, Coverage: mirror.Coverage{State: mirror.NeverRead}}, f.clock.Now(), mirror.RefreshPolicy{FullInventoryInterval: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	f.plan.AttemptGrant = mirror.AttemptGrant{ID: "native-grant", Scope: f.scope, SourceID: "native-source", Generation: "generation-1", CallerID: "caller-1", AccessEpoch: "access-1", ConfigEpoch: "config-1", PolicyClass: "read", PolicyEpoch: "policy-1", MaxHTTPAttempts: 32, ExpiresAt: f.clock.Now().Add(time.Hour)}
	f.authority.grant = f.plan.AttemptGrant
	return f
}

type nativeRebindingAdmission struct {
	mirror.Admission
	id        string
	calls, at int
}

func (a *nativeRebindingAdmission) ID() string { return a.id }
func (a *nativeRebindingAdmission) Authorize(ctx context.Context, r mirror.RepositoryID) error {
	a.calls++
	err := a.Admission.Authorize(ctx, r)
	if a.calls == a.at {
		a.id = "other-admission"
	}
	return err
}

type nativeReplayedReservation struct{ mirror.AttemptAuthority }

func (a nativeReplayedReservation) BeforeAttempt(ctx context.Context, g mirror.AttemptGrant) (mirror.AttemptReservation, error) {
	r, e := a.AttemptAuthority.BeforeAttempt(ctx, g)
	if e == nil {
		r.ID = "attempt-1"
	}
	return r, e
}

type nativeErrorReader struct{ err error }

func (r nativeErrorReader) Read([]byte) (int, error) { return 0, r.err }

type nativeRT func(*http.Request) (*http.Response, error)

func (f nativeRT) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func nativeErrorDetail(err error) bool {
	pending := []error{err}
	for n := 0; len(pending) > 0 && n < 64; n++ {
		current := pending[0]
		pending = pending[1:]
		if current == nil {
			continue
		}
		if strings.Contains(current.Error(), "synthetic-") {
			return true
		}
		if many, ok := current.(interface{ Unwrap() []error }); ok {
			pending = append(pending, many.Unwrap()...)
		} else if one, ok := current.(interface{ Unwrap() error }); ok {
			pending = append(pending, one.Unwrap())
		}
	}
	return len(pending) > 0
}

func nativeResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
}
func nativeComposed(t *testing.T, f *nativeFixture, base http.RoundTripper) (*mirrorAttemptState, http.RoundTripper, *http.Request) {
	t.Helper()
	s, err := newMirrorAttempt(context.Background(), f.source, f.admission, f.scope, f.plan)
	if err != nil {
		t.Fatal(err)
	}
	u := s.origin
	u.RawQuery = s.query()
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return s, mirrorResponseTransport{next: countingTransport{next: base, mirror: s}, attempt: s}, req
}

// regression: F-mirror-native-attempts
func TestMirrorNativeAttemptReceipt(t *testing.T) {
	for _, provider := range []ForgeKind{ForgeGitHub, ForgeGitLab} {
		t.Run("actual-no-hidden-retry-"+string(provider), func(t *testing.T) {
			f := newNativeFixture(t, provider, mirror.Issues, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(503)
				_, _ = io.WriteString(w, `{"message":"unavailable"}`)
			})
			scan, err := f.source.Scan(context.Background(), f.admission, f.scope, f.plan)
			if err == nil || scan.Complete || f.requests.Load() != 1 || scan.Attempts.HTTP != (mirror.WorkCount{Value: 1, Known: true}) || f.authority.observed != 1 {
				t.Fatalf("pinned SDK retried or lost failed work: %+v err=%v HTTP=%d", scan, err, f.requests.Load())
			}
		})
	}
	for _, mode := range []string{"read-error", "cancelled-read", "overflow", "gzip-overflow", "gzip-complete"} {
		t.Run("decoded-"+mode, func(t *testing.T) {
			f := newNativeFixture(t, ForgeGitHub, mirror.Issues, func(http.ResponseWriter, *http.Request) { t.Error("unexpected server call") })
			cause := errors.New("synthetic decoded failure")
			if mode == "cancelled-read" {
				cause = context.Canceled
			}
			body := strings.Repeat(" ", int(adapters.MaxUpdateBytes)+2)
			want := int64(adapters.MaxUpdateBytes + 1)
			complete := mode == "gzip-complete"
			if complete {
				body = "[]"
				want = 2
			}
			state, transport, req := nativeComposed(t, f, nativeRT(func(*http.Request) (*http.Response, error) {
				r := nativeResponse(200, body)
				switch mode {
				case "read-error", "cancelled-read":
					r.Body = io.NopCloser(io.MultiReader(strings.NewReader("prefix"), nativeErrorReader{cause}))
					want = 6
				case "gzip-overflow", "gzip-complete":
					var compressed bytes.Buffer
					z := gzip.NewWriter(&compressed)
					_, _ = io.WriteString(z, body)
					_ = z.Close()
					r.Body = io.NopCloser(bytes.NewReader(compressed.Bytes()))
					r.Header.Set("Content-Encoding", "gzip")
				}
				return r, nil
			}))
			resp, err := transport.RoundTrip(req)
			if resp != nil {
				resp.Body.Close()
			}
			if (err == nil) != complete {
				t.Fatalf("decoded completeness changed: %v", err)
			}
			if (mode == "read-error" || mode == "cancelled-read") && !errors.Is(err, cause) {
				t.Fatal("decoded cause lost", err)
			}
			got := state.snapshot()
			report := f.authority.reports["attempt-1"]
			if got.DecodedBytes != (mirror.WorkCount{Value: want, Known: true}) || report.DecodedBytes != got.DecodedBytes || report.ResponseComplete != complete || got.HTTP != (mirror.WorkCount{Value: 1, Known: true}) || report.HTTP != got.HTTP {
				t.Fatalf("own consumed prefix was dropped or confused with full coverage: %+v report=%+v", got, report)
			}
		})
	}

	for _, provider := range []ForgeKind{ForgeGitHub, ForgeGitLab} {
		t.Run(string(provider), func(t *testing.T) {
			f := newNativeFixture(t, provider, mirror.Issues, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("page") == "1" {
					w.Header().Set("Link", `<https://foreign.example.invalid/wrong>; rel="next"`)
					w.Header().Set("X-Next-Page", "2")
				}
				_, _ = io.WriteString(w, "[]")
			})
			scan, err := f.source.Scan(context.Background(), f.admission, f.scope, f.plan)
			if err != nil || !scan.Complete || scan.Attempts.HTTP != (mirror.WorkCount{Value: 2, Known: true}) || scan.Attempts.DecodedBytes != (mirror.WorkCount{Value: 4, Known: true}) || f.requests.Load() != 2 || f.authority.observed != 2 || f.authority.serial != 2 {
				t.Fatalf("actual SDK pages/attempts/bytes changed: %+v err=%v requests=%d reports=%d", scan.Attempts, err, f.requests.Load(), f.authority.observed)
			}
			for _, o := range f.authority.reports {
				if o.HTTP != (mirror.WorkCount{Value: 1, Known: true}) || o.DecodedBytes.Value != 2 || !o.ResponseComplete || o.Status != 200 {
					t.Fatalf("individual report not own complete attempt: %+v", o)
				}
			}
		})
	}
	for _, mode := range []string{"repeated-leaf", "response-error", "no-response"} {
		t.Run(mode, func(t *testing.T) {
			f := newNativeFixture(t, ForgeGitHub, mirror.Issues, func(http.ResponseWriter, *http.Request) { t.Error("unexpected server call") })
			cause := errors.New("synthetic native cause")
			calls := 0
			state, transport, req := nativeComposed(t, f, nativeRT(func(*http.Request) (*http.Response, error) {
				calls++
				if mode == "no-response" {
					return nil, cause
				}
				if mode == "response-error" {
					return nativeResponse(403, "[]"), cause
				}
				if calls == 1 {
					return nativeResponse(503, "[]"), nil
				}
				return nativeResponse(200, "[]"), nil
			}))
			resp, err := transport.RoundTrip(req)
			if resp != nil {
				resp.Body.Close()
			}
			if mode == "repeated-leaf" {
				resp, err = transport.RoundTrip(req)
				if resp != nil {
					resp.Body.Close()
				}
				if err != nil || calls != 2 || state.snapshot().HTTP.Value != 2 || f.authority.observed != 2 {
					t.Fatalf("repeated composed leaf boundary changed: calls=%d err=%v", calls, err)
				}
			} else {
				if !errors.Is(err, cause) || calls != 1 || state.snapshot().HTTP != (mirror.WorkCount{Value: 1, Known: true}) || f.authority.observed != 1 {
					t.Fatalf("native failure lost actual work/cause: %+v %v", state.snapshot(), err)
				}
				want := mirror.RetryUnknown
				if mode == "no-response" {
					want = mirror.RetryNoResponse
				}
				if state.snapshot().Retry.Knowledge != want {
					t.Fatalf("response/error and no-response conflated: %+v", state.snapshot())
				}
			}
		})
	}
}

// Compatibility support for the inherited mapper cases; native authority/budget
// qualification uses the independently configured stateful fixture below.
type compatMirrorAuthority struct{ serial atomic.Int64 }

func (a *compatMirrorAuthority) BeforeAttempt(ctx context.Context, g mirror.AttemptGrant) (mirror.AttemptReservation, error) {
	if err := ctx.Err(); err != nil {
		return mirror.AttemptReservation{}, err
	}
	return mirror.AttemptReservation{ID: fmt.Sprint(a.serial.Add(1)), Grant: g}, nil
}
func (*compatMirrorAuthority) ObserveAttempt(ctx context.Context, _ mirror.AttemptReservation, _ mirror.AttemptObservation) error {
	return ctx.Err()
}

// regression: F-mirror-native-grant
func TestMirrorNativeAttemptGrant(t *testing.T) {
	for _, at := range []int{1, 2} {
		t.Run(fmt.Sprint("admission-rebind-during-check-", at), func(t *testing.T) {
			f := newNativeFixture(t, ForgeGitHub, mirror.Issues, func(http.ResponseWriter, *http.Request) { t.Error("unexpected server call") })
			calls := 0
			state, tr, req := nativeComposed(t, f, nativeRT(func(*http.Request) (*http.Response, error) { calls++; return nativeResponse(200, "[]"), nil }))
			state.admission = &nativeRebindingAdmission{Admission: f.admission, id: f.admission.ID(), at: at}
			_, err := tr.RoundTrip(req)
			if !errors.Is(err, mirror.ErrPermission) || calls != 0 || f.authority.before != int64(at-1) || state.snapshot().HTTP.Value != 0 {
				t.Fatalf("changed admission escaped current check: calls=%d before=%d err=%v", calls, f.authority.before, err)
			}
		})
	}

	for _, mode := range []string{"revoked", "cancelled", "expired"} {
		t.Run("post-debit-"+mode, func(t *testing.T) {
			f := newNativeFixture(t, ForgeGitHub, mirror.Issues, func(http.ResponseWriter, *http.Request) { t.Error("unexpected server call") })
			calls := 0
			state, tr, req := nativeComposed(t, f, nativeRT(func(*http.Request) (*http.Response, error) { calls++; return nativeResponse(200, "[]"), nil }))
			ctx, cancel := context.WithCancel(req.Context())
			defer cancel()
			req = req.WithContext(ctx)
			f.authority.afterBefore = func() {
				switch mode {
				case "revoked":
					f.admission.allowed.Store(false)
				case "cancelled":
					cancel()
				case "expired":
					f.clock.add(2 * time.Hour)
				}
			}
			_, err := tr.RoundTrip(req)
			if err == nil || calls != 0 || state.snapshot().HTTP != (mirror.WorkCount{Known: true}) || f.authority.serial != 1 {
				t.Fatalf("reserved work skipped current boundary: %+v err=%v calls=%d", state.snapshot(), err, calls)
			}
			if mode == "revoked" && !errors.Is(err, mirror.ErrPermission) {
				t.Fatal("revocation classified as source failure", err)
			}
			if mode == "cancelled" && !errors.Is(err, context.Canceled) {
				t.Fatal("cancellation cause lost", err)
			}
		})
	}

	for _, provider := range []ForgeKind{ForgeGitHub, ForgeGitLab} {
		t.Run("actual-redirect-"+string(provider), func(t *testing.T) {
			var foreign atomic.Int64
			target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { foreign.Add(1); _, _ = io.WriteString(w, "[]") }))
			defer target.Close()
			f := newNativeFixture(t, provider, mirror.Issues, func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, target.URL+"/foreign", http.StatusFound)
			})
			scan, err := f.source.Scan(context.Background(), f.admission, f.scope, f.plan)
			if !errors.Is(err, mirror.ErrAttemptHeld) || scan.Complete || f.requests.Load() != 1 || foreign.Load() != 0 || f.authority.before != 1 || scan.Attempts.HTTP.Value != 1 {
				t.Fatalf("metered redirect escaped: %+v err=%v foreign=%d", scan, err, foreign.Load())
			}
		})
	}

	t.Run("reservation-time-identity", func(t *testing.T) {
		f := newNativeFixture(t, ForgeGitHub, mirror.Issues, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "[]") })
		f.authority.normalizeReservation = true
		scan, err := f.source.Scan(context.Background(), f.admission, f.scope, f.plan)
		if err != nil || !scan.Complete || f.requests.Load() != 1 {
			t.Fatalf("same-instant reservation rejected: HTTP=%d err=%v", f.requests.Load(), err)
		}
	})
	for _, provider := range []ForgeKind{ForgeGitHub, ForgeGitLab} {
		for _, mode := range []string{"host", "host-header", "scheme", "userinfo", "method", "path", "escaped-dot", "double-escape", "query", "duplicate-query", "page", "missing", "scope", "kind", "relabel-repository", "relabel-host", "source", "expired", "taken-over", "foreign-caller", "caller-quota", "uncertain", "ceiling"} {
			t.Run(string(provider)+"/"+mode, func(t *testing.T) {
				f := newNativeFixture(t, provider, mirror.Issues, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "[]") })
				calls := 0
				state, transport, req := nativeComposed(t, f, nativeRT(func(*http.Request) (*http.Response, error) { calls++; return nativeResponse(200, "[]"), nil }))
				wantBefore, wantHTTP := int64(0), int64(0)
				switch mode {
				case "host":
					req.URL.Host = "foreign.example.invalid"
				case "host-header":
					req.Host = "foreign.example.invalid"
				case "scheme":
					req.URL.Scheme = "http"
				case "userinfo":
					req.URL.User = url.User("other")
				case "method":
					req.Method = http.MethodPost
				case "path":
					req.URL.Path += "/other"
					req.URL.RawPath = ""
				case "escaped-dot":
					req.URL.Path += "/../other"
					req.URL.RawPath = state.path + "/%2E%2E/other"
				case "double-escape":
					req.URL.Path = strings.Replace(req.URL.Path, "o/r", "o%2Fr", 1)
					req.URL.RawPath = strings.Replace(state.path, "o%2Fr", "o%252Fr", 1)
				case "query":
					q := req.URL.Query()
					q.Set("state", "open")
					req.URL.RawQuery = q.Encode()
				case "duplicate-query":
					req.URL.RawQuery = "page=1&" + req.URL.RawQuery
				case "page":
					q := req.URL.Query()
					q.Set("page", "2")
					req.URL.RawQuery = q.Encode()
				case "missing":
					state.grant = mirror.AttemptGrant{}
				case "scope":
					state.grant.Scope.AdmissionID = "other"
				case "relabel-repository", "relabel-host":
					if mode == "relabel-repository" {
						state.scope.Repository.Path = "other/repo"
					} else {
						state.scope.Repository.Host = "foreign.example.invalid"
					}
					state.grant.Scope = state.scope
					f.authority.grant = state.grant
				case "kind":
					state.grant.Scope.Kind = mirror.Changes
				case "source":
					state.grant.SourceID = "other"
				case "expired":
					state.grant.ExpiresAt = f.clock.Now()
				case "taken-over":
					f.authority.currentGeneration = "generation-2"
					wantBefore = 1
				case "foreign-caller":
					state.grant.CallerID = "caller-2"
					wantBefore = 1
				case "caller-quota":
					f.authority.deny = mirror.AttemptCallerHeld
					wantBefore = 1
				case "uncertain":
					f.authority.beforeErr = true
					wantBefore = 1
				case "ceiling":
					state.receipt.HTTP.Value = state.grant.MaxHTTPAttempts
					wantHTTP = state.grant.MaxHTTPAttempts
				}
				_, err := transport.RoundTrip(req)
				if !errors.Is(err, mirror.ErrAttemptHeld) || calls != 0 || f.authority.before != wantBefore || state.snapshot().HTTP.Value != wantHTTP {
					t.Fatalf("lower native guard failed: err=%v calls=%d before=%d receipt=%+v", err, calls, f.authority.before, state.snapshot())
				}
				if nativeErrorDetail(err) {
					t.Fatal("authority implementation detail escaped")
				}
				if mode == "uncertain" && (f.authority.serial != 1 || len(f.authority.pending) != 1 || !f.authority.unknown) {
					t.Fatal("uncertain debit became measured zero/refundable")
				}
			})
		}
	}
	for _, kind := range []mirror.RecordKind{mirror.Issues, mirror.Changes} {
		t.Run("gitlab-case-"+string(kind), func(t *testing.T) {
			f := newNativeFixture(t, ForgeGitLab, kind, func(w http.ResponseWriter, r *http.Request) {
				if !strings.Contains(r.URL.EscapedPath(), "Group%2FProject") {
					t.Error("encoded project identity changed")
				}
				_, _ = io.WriteString(w, "[]")
			}, ForgeRepo{Owner: "Group", Name: "Project"})
			scan, err := f.source.Scan(context.Background(), f.admission, f.scope, f.plan)
			if err != nil || !scan.Complete || f.requests.Load() != 1 || f.scope.Repository.Path != "Group/Project" {
				t.Fatalf("matching encoded/case-bound request refused: %+v %v", scan.Attempts, err)
			}
			state, transport, req := nativeComposed(t, f, nativeRT(func(*http.Request) (*http.Response, error) {
				t.Error("foreign case reached base")
				return nativeResponse(200, "[]"), nil
			}))
			req.URL.Path = strings.Replace(req.URL.Path, "Group", "group", 1)
			req.URL.RawPath = strings.Replace(state.path, "Group", "group", 1)
			before := f.authority.before
			if _, err := transport.RoundTrip(req); !errors.Is(err, mirror.ErrAttemptHeld) || f.authority.before != before {
				t.Fatal("GitLab case relabel reached authority", err)
			}
		})
	}
	for _, mode := range []string{"ceiling", "caller-quota", "uncertain-before", "uncertain-report", "missing-grant"} {
		t.Run("actual-source-"+mode, func(t *testing.T) {
			f := newNativeFixture(t, ForgeGitHub, mirror.Issues, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Link", `<https://foreign.example.invalid/>; rel="next"`)
				_, _ = io.WriteString(w, "[]")
			})
			wantHTTP := int64(0)
			switch mode {
			case "ceiling":
				f.plan.AttemptGrant.MaxHTTPAttempts = 1
				f.authority.grant = f.plan.AttemptGrant
				wantHTTP = 1
			case "caller-quota":
				f.authority.deny = mirror.AttemptCallerHeld
			case "uncertain-before":
				f.authority.beforeErr = true
			case "uncertain-report":
				f.authority.observeErr = true
				wantHTTP = 1
			case "missing-grant":
				f.plan.AttemptGrant = mirror.AttemptGrant{}
			}
			scan, err := f.source.Scan(context.Background(), f.admission, f.scope, f.plan)
			if !errors.Is(err, mirror.ErrAttemptHeld) || scan.Complete || !scan.ObservedUpTo.IsZero() || scan.Failure != "" || scan.Attempts.HTTP != (mirror.WorkCount{Value: wantHTTP, Known: true}) || f.requests.Load() != wantHTTP {
				t.Fatalf("budget hold became source error/complete/false work: %+v err=%v HTTP=%d", scan, err, f.requests.Load())
			}
			if nativeErrorDetail(err) {
				t.Fatal("authority implementation detail escaped source error chain")
			}
			if mode == "ceiling" && f.authority.before != 1 {
				t.Fatal("local ceiling did not refuse before another debit")
			}
			if mode == "caller-quota" && scan.Attempts.Held != mirror.AttemptCallerHeld {
				t.Fatal("caller quota classification lost")
			}
			if mode == "uncertain-report" {
				if f.authority.serial != 1 || len(f.authority.pending) != 1 {
					t.Fatal("uncertain observation released its slot")
				}
				_, _ = f.source.Scan(context.Background(), f.admission, f.scope, f.plan)
				if f.requests.Load() != 1 {
					t.Fatal("unreported reservation became another allowance")
				}
			}
		})
	}
}

// regression: F-mirror-native-retry
func TestMirrorNativeRetryReceipt(t *testing.T) {
	t.Run("report-error-status", func(t *testing.T) {
		f := newNativeFixture(t, ForgeGitHub, mirror.Issues, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(403)
			_, _ = io.WriteString(w, `{"message":"denied"}`)
		})
		f.authority.observeErr = true
		_, err := f.source.Scan(context.Background(), f.admission, f.scope, f.plan)
		var status *ForgeAPIError
		if !errors.Is(err, mirror.ErrAttemptHeld) || !errors.As(err, &status) || status.Status != 403 {
			t.Fatalf("native observation failure lost known status: %v", err)
		}
	})
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name       string
		provider   mirror.Provider
		status     int
		header     http.Header
		knowledge  mirror.RetryKnowledge
		floor      time.Time
		delay      time.Duration
		delayKnown bool
	}{
		{"observed-empty", mirror.GitHub, 200, http.Header{}, mirror.RetryNoConstraint, time.Time{}, 0, false},
		{"lost-metadata", mirror.GitHub, 200, nil, mirror.RetryUnknown, time.Time{}, 0, false},
		{"delta", mirror.GitHub, 429, http.Header{"Retry-After": {"60"}}, mirror.RetryNotBefore, now.Add(time.Minute), time.Minute, true},
		{"zero", mirror.GitLab, 429, http.Header{"Retry-After": {"0"}}, mirror.RetryNotBefore, now, 0, true},
		{"past-date", mirror.GitLab, 429, http.Header{"Retry-After": {now.Add(-time.Minute).Format(http.TimeFormat)}}, mirror.RetryUnknown, now.Add(-time.Minute), 0, false},
		{"signed-reset", mirror.GitHub, 429, http.Header{"X-Ratelimit-Remaining": {"0"}, "X-Ratelimit-Reset": {"+1793707380"}}, mirror.RetryUnknown, time.Time{}, 0, false},
		{"date", mirror.GitLab, 429, http.Header{"Retry-After": {now.Add(2 * time.Minute).Format(http.TimeFormat)}}, mirror.RetryNotBefore, now.Add(2 * time.Minute), 0, false},
		{"primary", mirror.GitHub, 403, http.Header{"X-Ratelimit-Remaining": {"0"}, "X-Ratelimit-Reset": {fmt.Sprint(now.Add(3 * time.Minute).Unix())}}, mirror.RetryNotBefore, now.Add(3 * time.Minute), 0, false},
		{"maximum", mirror.GitHub, 429, http.Header{"Retry-After": {"60"}, "X-Ratelimit-Remaining": {"0"}, "X-Ratelimit-Reset": {fmt.Sprint(now.Add(3 * time.Minute).Unix())}}, mirror.RetryNotBefore, now.Add(3 * time.Minute), time.Minute, true},
		{"absent-required", mirror.GitLab, 429, http.Header{}, mirror.RetryUnknown, time.Time{}, 0, false},
		{"secondary-without-time", mirror.GitHub, 403, http.Header{"X-Ratelimit-Remaining": {"10"}}, mirror.RetryUnknown, time.Time{}, 0, false},
		{"missing-reset", mirror.GitHub, 429, http.Header{"X-Ratelimit-Remaining": {"0"}}, mirror.RetryUnknown, time.Time{}, 0, false},
		{"malformed", mirror.GitHub, 429, http.Header{"Retry-After": {"secret-raw-header"}}, mirror.RetryUnknown, time.Time{}, 0, false},
		{"overflow", mirror.GitLab, 429, http.Header{"Retry-After": {"18446744073709551615"}}, mirror.RetryUnknown, time.Time{}, 0, false},
		{"duplicate", mirror.GitHub, 429, http.Header{"Retry-After": {"10", "20"}}, mirror.RetryUnknown, time.Time{}, 0, false},
		{"oversized", mirror.GitLab, 429, http.Header{"Retry-After": {strings.Repeat("1", 129)}}, mirror.RetryUnknown, time.Time{}, 0, false},
		{"partial-good-floor", mirror.GitHub, 429, http.Header{"Retry-After": {"60"}, "X-Ratelimit-Remaining": {"0"}, "X-Ratelimit-Reset": {"invalid"}}, mirror.RetryUnknown, now.Add(time.Minute), time.Minute, true},
		{"other-provider-header", mirror.GitLab, 200, http.Header{"X-Ratelimit-Remaining": {"bad", "bad"}}, mirror.RetryNoConstraint, time.Time{}, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := mirrorRetryEvidence(tc.provider, tc.status, tc.header, now)
			if got.Knowledge != tc.knowledge || !got.NotBefore.Equal(tc.floor) || got.Delay != tc.delay || got.DelayKnown != tc.delayKnown {
				t.Fatalf("typed retry knowledge/floor changed: %+v", got)
			}
			if strings.Contains(fmt.Sprint(got), "secret-raw-header") {
				t.Fatal("raw metadata retained")
			}
		})
	}
	prior := mirror.RetryEvidence{Knowledge: mirror.RetryNotBefore, NotBefore: now.Add(3 * time.Minute), Delay: time.Minute, DelayKnown: true}
	got := mergeMirrorRetry(prior, mirror.RetryEvidence{Knowledge: mirror.RetryUnknown, NotBefore: now.Add(time.Minute)})
	if got.Knowledge != mirror.RetryUnknown || !got.NotBefore.Equal(prior.NotBefore) || got.Delay != prior.Delay || !got.DelayKnown {
		t.Fatal("unknown evidence erased prior positive bounds", got)
	}
	if got := mergeMirrorRetry(mirror.RetryEvidence{Knowledge: mirror.RetryNoResponse}, mirror.RetryEvidence{Knowledge: mirror.RetryNoConstraint}); got.Knowledge != mirror.RetryNoConstraint {
		t.Fatal("a known response became qualified no-response", got)
	}
	for _, mode := range []string{"floor", "unknown", "lost-metadata"} {
		t.Run("repeated-"+mode, func(t *testing.T) {
			f := newNativeFixture(t, ForgeGitHub, mirror.Issues, func(http.ResponseWriter, *http.Request) { t.Error("unexpected server call") })
			calls := 0
			state, transport, req := nativeComposed(t, f, nativeRT(func(*http.Request) (*http.Response, error) {
				calls++
				r := nativeResponse(503, "[]")
				switch mode {
				case "floor":
					r.Header.Set("Retry-After", "60")
				case "unknown":
					r.StatusCode = 429
				case "lost-metadata":
					r.Header = nil
				}
				return r, nil
			}))
			resp, err := transport.RoundTrip(req)
			if resp != nil {
				resp.Body.Close()
			}
			if mode == "lost-metadata" && !errors.Is(err, mirror.ErrAttemptHeld) || mode != "lost-metadata" && err != nil {
				t.Fatal(err)
			}
			before := f.authority.before
			if _, err := transport.RoundTrip(req); !errors.Is(err, mirror.ErrAttemptHeld) || calls != 1 || f.authority.before != before || state.snapshot().HTTP.Value != 1 {
				t.Fatalf("new retry evidence did not hold before debit/base: calls=%d before=%d err=%v", calls, f.authority.before, err)
			}
		})
	}
	for _, skew := range []time.Duration{-10 * time.Minute, 10 * time.Minute} {
		t.Run("clock-skew-"+skew.String(), func(t *testing.T) {
			f := newNativeFixture(t, ForgeGitHub, mirror.Issues, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(429)
				_, _ = io.WriteString(w, `{"message":"rate limited"}`)
			})
			db := &nativeClock{now: f.clock.Now()}
			f.authority.clock = db
			f.clock.add(skew)
			scan, err := f.source.Scan(context.Background(), f.admission, f.scope, f.plan)
			if err == nil || scan.Failure != mirror.RateLimited || scan.Attempts.Retry.Delay != time.Minute || !scan.Attempts.Retry.DelayKnown || f.authority.observed != 1 {
				t.Fatalf("qualified native delay lost: %+v %v", scan, err)
			}
			want := db.Now().Add(time.Minute)
			if source := f.clock.Now().Add(time.Minute); source.After(want) {
				want = source
			}
			if !f.authority.floor.Equal(want) {
				t.Fatalf("source skew shortened canonical delay: floor=%v want=%v", f.authority.floor, want)
			}
			_, _ = f.source.Scan(context.Background(), f.admission, f.scope, f.plan)
			if f.requests.Load() != 1 {
				t.Fatal("source skew reset a canonical allowance")
			}
		})
	}
}

type nativeCache struct {
	base  http.RoundTripper
	body  string
	calls atomic.Int64
}

// One invocation's assembly stub, with no coalescing, quota or failure backend.
// SQL generation/transaction/fairness qualification remains the canonical owner.
type nativeJoinGate struct {
	mu     sync.Mutex
	ticket refresh.Ticket
	ready  chan struct{}
	state  refresh.State
}

func (g *nativeJoinGate) Acquire(context.Context, refresh.Demand) (refresh.Ticket, error) {
	return g.ticket, nil
}
func (g *nativeJoinGate) Wait(ctx context.Context, _ refresh.Ticket) (refresh.State, error) {
	select {
	case <-ctx.Done():
		return refresh.State{}, ctx.Err()
	case <-g.ready:
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.state, nil
}
func (g *nativeJoinGate) Complete(ctx context.Context, _ refresh.Ticket, store mirror.Store, a mirror.Admission, c refresh.Completion) error {
	r, e := refresh.ApplyCompletion(ctx, store, a, c)
	g.mu.Lock()
	g.state = refresh.State{Reason: r.Reason, AttemptHeld: c.Attempts.Held != ""}
	close(g.ready)
	g.mu.Unlock()
	return e
}
func (g *nativeJoinGate) Leave(context.Context, refresh.Ticket, refresh.Outcome) (refresh.State, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.state, nil
}

type nativeCallerAuthority struct {
	caller refresh.Caller
	scope  mirror.Scope
}

func (a nativeCallerAuthority) Authorize(ctx context.Context, c refresh.Caller, s mirror.Scope) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	if c != a.caller || s != a.scope {
		return mirror.ErrPermission
	}
	return nil
}

func (c *nativeCache) RoundTrip(*http.Request) (*http.Response, error) {
	c.calls.Add(1)
	return nativeResponse(200, c.body), nil
}

// regression: F-mirror-native-freshness
func TestMirrorNativeFreshnessBoundary(t *testing.T) {
	t.Run("empty-report-age", func(t *testing.T) {
		f := newNativeFixture(t, ForgeGitHub, mirror.Issues, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "[]") })
		bodyAt := f.clock.Now()
		f.authority.afterObserve = func() { f.clock.add(2 * time.Minute) }
		caller := refresh.Caller{ID: "caller-1", AdmissionID: f.admission.ID(), AccessEpoch: "access-1", ConfigEpoch: "config-1", PolicyClass: "read", PolicyEpoch: "policy-1", Limits: refresh.Limits{CallerFillsPerWindow: 4, AdmissionFillsPerWindow: 4, FillWindow: time.Minute, MinFreshness: time.Second, MaxFreshness: time.Hour, ActiveFills: 1, QueuedCallers: 1, AdmissionFills: 1, AdmissionWaiters: 1, FailureEntries: 1, WaitTimeout: 3 * time.Second, FillTimeout: 3 * time.Second, Cooldown: time.Minute, FailureTTL: time.Minute, Refresh: mirror.RefreshPolicy{FullInventoryInterval: time.Hour}}}
		gate := &nativeJoinGate{ticket: refresh.Ticket{ID: "single-owner", Generation: f.plan.AttemptGrant.Generation, Owner: true, AttemptGrant: f.plan.AttemptGrant}, ready: make(chan struct{})}
		coordinator, e := refresh.New(context.Background(), mirrortest.NewStore(), f.source, gate, f.clock, "native-source", nativeCallerAuthority{caller, f.scope})
		if e != nil {
			t.Fatal(e)
		}
		r, e := coordinator.Read(context.Background(), f.admission, refresh.Query{Query: mirror.Query{Scope: f.scope, MaxAge: time.Minute}}, caller)
		if e != nil || r.Fulfilled || r.Available || !r.Snapshot.Coverage.LastComplete.Equal(bodyAt) || f.requests.Load() != 1 {
			t.Fatalf("slow report refreshed already-read empty inventory: fulfilled=%v available=%v complete=%v body=%v HTTP=%d err=%v", r.Fulfilled, r.Available, r.Snapshot.Coverage.LastComplete, bodyAt, f.requests.Load(), e)
		}
	})
	for _, provider := range []ForgeKind{ForgeGitHub, ForgeGitLab} {
		t.Run(string(provider), func(t *testing.T) {
			f := newNativeFixture(t, provider, mirror.Issues, func(w http.ResponseWriter, r *http.Request) {
				if provider == ForgeGitHub && r.Header.Get("Authorization") != "token "+ciFixtureToken {
					t.Error("admitted GitHub header custody lost")
				}
				if provider == ForgeGitLab && r.Header.Get("Private-Token") != glTestToken {
					t.Error("admitted GitLab header custody lost")
				}
				_, _ = io.WriteString(w, "[]")
			})
			original := f.client.Transport.(*http.Transport)
			tls := original.TLSClientConfig
			scan, err := f.source.Scan(context.Background(), f.admission, f.scope, f.plan)
			if err != nil || !scan.Complete || scan.Attempts.HTTP.Value != 1 || f.client.Transport != original || original.TLSClientConfig != tls {
				t.Fatalf("fresh admitted transport/custody changed: %+v %v", scan, err)
			}
			old := f.clock.Now().Add(-2 * time.Hour)
			cache := &nativeCache{base: original, body: "[" + ingestRow(provider, mirror.Issues, 1, old, true) + "]"}
			// This is an existing marker with an opaque cache above its real base.
			// The new metered composition cannot establish fresh native coverage.
			f.client.Transport = countedTransport{next: cache}
			configured := f.client.Transport
			ordinary, ordinaryErr := f.source.source.Read(context.Background(), MirrorIssues, adapters.UpdateQuery{})
			if ordinaryErr != nil || len(ordinary.Issues) != 1 || cache.calls.Load() != 1 || f.authority.before != 1 {
				t.Fatalf("ordinary reader behavior changed: %v", ordinaryErr)
			}
			cache.calls.Store(0)
			store := mirrortest.NewStore()
			prior, err := store.Apply(context.Background(), f.admission, mirror.ScanUpdate{Scope: f.scope, CompleteScan: true, FullInventory: true, Coverage: mirror.Coverage{State: mirror.Complete, LastAttempted: old, LastComplete: old, LastInventory: old, ObservedUpTo: old}, Upsert: []mirror.Observation{{ID: mirror.RecordID{Repository: f.scope.Repository, Kind: "issue", Number: 1}, Title: "private-canary", UpdatedAt: old, ObservedAt: old}}})
			if err != nil {
				t.Fatal(err)
			}
			plan, err := mirror.PlanRefresh(prior, f.clock.Now(), mirror.RefreshPolicy{FullInventoryInterval: time.Hour})
			if err != nil {
				t.Fatal(err)
			}
			plan.AttemptGrant = f.plan.AttemptGrant
			scan, err = f.source.Scan(context.Background(), f.admission, f.scope, plan)
			if !errors.Is(err, mirror.ErrAttemptHeld) || scan.Complete || !scan.ObservedUpTo.IsZero() || scan.Attempts.HTTP != (mirror.WorkCount{Known: true}) || f.requests.Load() != 1 || cache.calls.Load() != 0 || f.client.Transport != configured || cache.base != original {
				t.Fatalf("opaque cache replay became fresh provider evidence or changed custody: %+v %v provider=%d cache=%d", scan, err, f.requests.Load(), cache.calls.Load())
			}
			update, err := mirror.PlanScan(prior, plan, scan, f.clock.Now())
			if err != nil {
				t.Fatal(err)
			}
			after, err := store.Apply(context.Background(), f.admission, update)
			if err != nil || !after.Coverage.LastComplete.Equal(prior.Coverage.LastComplete) || !after.Coverage.ObservedUpTo.Equal(prior.Coverage.ObservedUpTo) || len(after.Records) != 1 || after.Records[0].Title != "private-canary" {
				t.Fatal("held transport advanced or erased stored coverage", err)
			}
		})
	}
}

// regression: F-mirror-native-observation
func TestMirrorNativeObservationBoundary(t *testing.T) {
	t.Run("lower-replayed-reservation", func(t *testing.T) {
		f := newNativeFixture(t, ForgeGitHub, mirror.Issues, func(http.ResponseWriter, *http.Request) { t.Error("unexpected server call") })
		f.source.authority = nativeReplayedReservation{f.authority}
		calls := 0
		state, tr, req := nativeComposed(t, f, nativeRT(func(*http.Request) (*http.Response, error) { calls++; return nativeResponse(503, "[]"), nil }))
		r, e := tr.RoundTrip(req)
		if r != nil {
			r.Body.Close()
		}
		if e != nil {
			t.Fatal(e)
		}
		_, e = tr.RoundTrip(req)
		if !errors.Is(e, mirror.ErrAttemptHeld) || calls != 1 || state.snapshot().HTTP.Value != 1 || f.authority.serial != 2 || f.authority.observed != 1 || len(f.authority.pending) != 1 {
			t.Fatalf("replayed reservation spent twice: %+v %v", state.snapshot(), e)
		}
	})

	for _, unknown := range []bool{false, true} {
		t.Run(fmt.Sprint("concurrent-unknown-", unknown), func(t *testing.T) {
			body1, body2 := make(chan struct{}), make(chan struct{})
			headers := make(chan int, 4)
			var one, two sync.Once
			defer one.Do(func() { close(body1) })
			defer two.Do(func() { close(body2) })
			var sequence atomic.Int64
			f := newNativeFixture(t, ForgeGitHub, mirror.Issues, func(w http.ResponseWriter, _ *http.Request) {
				n := int(sequence.Add(1))
				if n == 1 {
					if !unknown {
						w.Header().Set("Retry-After", "60")
					}
					w.WriteHeader(429)
				}
				w.(http.Flusher).Flush()
				headers <- n
				if n == 1 {
					<-body1
				} else if n == 2 {
					<-body2
				}
				_, _ = io.WriteString(w, "[]")
			})
			second := f.plan
			second.AttemptGrant.ID = "second-grant"
			second.AttemptGrant.Generation = "generation-2"
			second.AttemptGrant.ConfigEpoch = "config-2"
			second.AttemptGrant.CallerID = "caller-2"
			f.authority.grants = map[string]mirror.AttemptGrant{f.plan.AttemptGrant.ID: f.plan.AttemptGrant, second.AttemptGrant.ID: second.AttemptGrant}
			type answer struct {
				scan mirror.ScanResult
				err  error
			}
			firstDone, secondDone := make(chan answer, 1), make(chan answer, 1)
			go func() {
				s, e := f.source.Scan(context.Background(), f.admission, f.scope, f.plan)
				firstDone <- answer{s, e}
			}()
			select {
			case <-headers:
			case <-time.After(3 * time.Second):
				t.Fatal("first native headers absent")
			}
			go func() {
				s, e := f.source.Scan(context.Background(), f.admission, f.scope, second)
				secondDone <- answer{s, e}
			}()
			select {
			case <-headers:
			case <-time.After(3 * time.Second):
				t.Fatal("second finite in-flight reservation absent")
			}
			f.authority.mu.Lock()
			pending, observed := len(f.authority.pending), f.authority.observed
			f.authority.mu.Unlock()
			if pending != 2 || observed != 0 || f.requests.Load() != 2 {
				t.Fatal("header observation fabricated instantaneous shared publication", pending, observed, f.requests.Load())
			}
			one.Do(func() { close(body1) })
			var first answer
			select {
			case first = <-firstDone:
			case <-time.After(3 * time.Second):
				t.Fatal("first attempt did not report")
			}
			if first.err == nil || first.scan.Attempts.HTTP.Value != 1 {
				t.Fatal("first native response lost own work", first.err)
			}
			third, err := f.source.Scan(context.Background(), f.admission, f.scope, second)
			if !errors.Is(err, mirror.ErrAttemptHeld) || third.Attempts.HTTP != (mirror.WorkCount{Known: true}) || f.requests.Load() != 2 {
				t.Fatal("authorizations after publication ignored floor/unknown", err, f.requests.Load())
			}
			two.Do(func() { close(body2) })
			var later answer
			select {
			case later = <-secondDone:
			case <-time.After(3 * time.Second):
				t.Fatal("earlier reserved attempt did not complete")
			}
			if later.err != nil || !later.scan.Complete || later.scan.Attempts.HTTP.Value != 1 {
				t.Fatal("already-authorized in-flight request was invented or lost", later.err)
			}
			f.authority.mu.Lock()
			pending = len(f.authority.pending)
			observed = f.authority.observed
			held := f.authority.unknown
			floor := f.authority.floor
			f.authority.mu.Unlock()
			if pending != 0 || observed != 2 || unknown && !held || !unknown && !floor.Equal(f.clock.Now().Add(time.Minute)) {
				t.Fatal("individual reports lost durable monotone state", pending, observed, held, floor)
			}
		})
	}
	t.Run("reservation-identity", func(t *testing.T) {
		f := newNativeFixture(t, ForgeGitHub, mirror.Issues, func(http.ResponseWriter, *http.Request) { t.Error("unexpected request") })
		a := f.authority
		r1, e := a.BeforeAttempt(context.Background(), f.plan.AttemptGrant)
		if e != nil {
			t.Fatal(e)
		}
		r2, e := a.BeforeAttempt(context.Background(), f.plan.AttemptGrant)
		if e != nil {
			t.Fatal(e)
		}
		o := mirror.AttemptObservation{HTTP: mirror.WorkCount{Value: 1, Known: true}, DecodedBytes: mirror.WorkCount{Known: true}, ObservedAt: f.clock.Now(), Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNotBefore, NotBefore: f.clock.Now().Add(3 * time.Minute)}}
		if e := a.ObserveAttempt(context.Background(), r2, o); e != nil {
			t.Fatal(e)
		}
		if e := a.ObserveAttempt(context.Background(), r2, o); !errors.Is(e, mirror.ErrAttemptHeld) {
			t.Fatal("duplicate reservation accepted", e)
		}
		foreign := r1
		foreign.Grant.Scope.Repository.Path = "other/repo"
		if e := a.ObserveAttempt(context.Background(), foreign, o); !errors.Is(e, mirror.ErrAttemptHeld) || len(a.pending) != 1 {
			t.Fatal("foreign report closed another slot", e)
		}
		o.Retry.NotBefore = f.clock.Now().Add(time.Minute)
		if e := a.ObserveAttempt(context.Background(), r1, o); e != nil || len(a.pending) != 0 || !a.floor.Equal(f.clock.Now().Add(3*time.Minute)) {
			t.Fatal("out-of-order own report weakened later floor", e)
		}
	})
	t.Run("owner-report-deadline", func(t *testing.T) {
		f := newNativeFixture(t, ForgeGitHub, mirror.Issues, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "[]") })
		f.authority.clock = &nativeClock{now: f.clock.Now()}
		f.clock.add(-10 * time.Minute)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		deadline, _ := ctx.Deadline()
		if _, e := f.source.Scan(ctx, f.admission, f.scope, f.plan); e != nil {
			t.Fatal(e)
		}
		if !f.authority.reportDeadline.Equal(deadline) {
			t.Fatal("source skew extended observation beyond owner deadline", f.authority.reportDeadline, deadline)
		}
	})
}

func nativeFieldShape(typ reflect.Type, want map[string]string) error {
	if typ.NumField() != len(want) {
		return fmt.Errorf("unexpected typed field count")
	}
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if want[f.Name] != f.Type.String() {
			return fmt.Errorf("unqualified field %s", f.Name)
		}
	}
	return nil
}

// regression: F-mirror-native-schema
func TestMirrorNativeReceiptBoundary(t *testing.T) {
	rosters := map[reflect.Type]map[string]string{
		reflect.TypeOf(mirror.WorkCount{}):          {"Value": "int64", "Known": "bool"},
		reflect.TypeOf(mirror.RetryEvidence{}):      {"Knowledge": "mirror.RetryKnowledge", "NotBefore": "time.Time", "Delay": "time.Duration", "DelayKnown": "bool"},
		reflect.TypeOf(mirror.AttemptGrant{}):       {"ID": "string", "SourceID": "string", "Generation": "string", "CallerID": "string", "AccessEpoch": "string", "ConfigEpoch": "string", "PolicyClass": "string", "PolicyEpoch": "string", "Scope": "mirror.Scope", "MaxHTTPAttempts": "int64", "ExpiresAt": "time.Time"},
		reflect.TypeOf(mirror.AttemptReservation{}): {"ID": "string", "Grant": "mirror.AttemptGrant"},
		reflect.TypeOf(mirror.AttemptObservation{}): {"HTTP": "mirror.WorkCount", "DecodedBytes": "mirror.WorkCount", "Status": "int", "ResponseComplete": "bool", "Retry": "mirror.RetryEvidence", "ObservedAt": "time.Time"},
		reflect.TypeOf(mirror.AttemptReceipt{}):     {"Grant": "mirror.AttemptGrant", "ObservedAt": "time.Time", "HTTP": "mirror.WorkCount", "DecodedBytes": "mirror.WorkCount", "Retry": "mirror.RetryEvidence", "Held": "mirror.AttemptHold"},
	}
	for typ, want := range rosters {
		if e := nativeFieldShape(typ, want); e != nil {
			t.Fatal(typ, e)
		}
	}
	typ := reflect.TypeOf(mirror.AttemptReceipt{})
	fields := make([]reflect.StructField, 0, typ.NumField()+1)
	for i := 0; i < typ.NumField(); i++ {
		fields = append(fields, typ.Field(i))
	}
	fields = append(fields, reflect.StructField{Name: "RawHeaders", Type: reflect.TypeOf(map[string]string{})})
	if nativeFieldShape(reflect.StructOf(fields), rosters[typ]) == nil {
		t.Fatal("typed boundary accepted planted raw metadata")
	}
	authority := reflect.TypeOf((*mirror.AttemptAuthority)(nil)).Elem()
	before, _ := authority.MethodByName("BeforeAttempt")
	observe, _ := authority.MethodByName("ObserveAttempt")
	if authority.NumMethod() != 2 || before.Type.NumIn() != 2 || before.Type.In(1) != reflect.TypeOf(mirror.AttemptGrant{}) || before.Type.NumOut() != 2 || before.Type.Out(0) != reflect.TypeOf(mirror.AttemptReservation{}) || observe.Type.NumIn() != 3 || observe.Type.In(1) != reflect.TypeOf(mirror.AttemptReservation{}) || observe.Type.In(2) != reflect.TypeOf(mirror.AttemptObservation{}) || observe.Type.NumOut() != 1 {
		t.Fatal("attempt authority became a generic effect/transport boundary")
	}
	root := "../../../.."
	for _, dir := range []string{"forgeread", "forgeread/mirror", "forgeread/refresh"} {
		files, e := filepath.Glob(filepath.Join(root, dir, "*.go"))
		if e != nil || len(files) == 0 {
			t.Fatal("missing actual pure owner", e)
		}
		for _, name := range files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			f, e := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
			if e != nil {
				t.Fatal(e)
			}
			for _, i := range f.Imports {
				p, e := strconv.Unquote(i.Path.Value)
				if e != nil {
					t.Fatal(e)
				}
				if p == "net" || p == "net/http" || p == "database/sql" || p == "os/exec" || strings.Contains(p, "/workflow") || strings.Contains(p, "credential") || strings.Contains(p, "/tools/desk") {
					t.Fatal("pure receipt owner acquired effect/custody dependency", p)
				}
			}
		}
	}
	source, e := parser.ParseFile(token.NewFileSet(), "mirrorsource.go", nil, 0)
	if e != nil {
		t.Fatal(e)
	}
	kinds := 0
	for _, decl := range source.Decls {
		g, ok := decl.(*ast.GenDecl)
		if !ok || g.Tok != token.CONST {
			continue
		}
		for _, spec := range g.Specs {
			v := spec.(*ast.ValueSpec)
			id, ok := v.Type.(*ast.Ident)
			if ok && id.Name == "MirrorRecordKind" {
				for _, n := range v.Names {
					if n.Name != "MirrorIssues" && n.Name != "MirrorChanges" {
						t.Fatal("new unqualified native mirror operation", n.Name)
					}
					kinds++
				}
			}
		}
	}
	if kinds != 2 {
		t.Fatal("closed native operation inventory changed", kinds)
	}
	github, e := os.ReadFile("forge_github.go")
	if e != nil {
		t.Fatal(e)
	}
	gitlab, e := os.ReadFile("forge_gitlab.go")
	if e != nil {
		t.Fatal(e)
	}
	mod, e := os.ReadFile("../../go.mod")
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(github), "ghapi.NewRESTClient(") || !strings.Contains(string(gitlab), "gitlab.WithoutRetries()") || !strings.Contains(string(gitlab), "gitlab.WithCustomLimiter(gitlabNoLimiter{})") || !strings.Contains(string(mod), "github.com/cli/go-gh/v2 v2.13.0") || !strings.Contains(string(mod), "gitlab.com/gitlab-org/api/client-go v1.46.0") {
		t.Fatal("pinned current SDK retry configuration changed; actual retry-path qualification required")
	}
	registered := false
	for _, entry := range ciCrossModuleRegistry() {
		if entry.test != "tools/desk/internal/deskkit/mirror_attempts_test.go" {
			continue
		}
		registered = true
		raw, e := os.ReadFile(filepath.Join(root, entry.workflow))
		if e != nil {
			t.Fatal(e)
		}
		for _, event := range []string{"pull_request", "push"} {
			job := entry.prJob
			if event == "push" {
				job = entry.pushJob
			}
			ciAssertEventReachesJob(t, string(raw), entry.workflow, entry.module, event, job, entry.runInvokes, entry.test, entry.why)
			globs, filtered := ciOnEventKey(t, string(raw), entry.workflow, event, "paths")
			if !filtered {
				globs = []string{"**"}
			}
			if faults := ciReadCoverage(t, root, globs, entry.reads); len(faults) != 0 {
				t.Fatal("native receipt reader lacks actual CI coverage", faults)
			}
		}
	}
	if !registered {
		t.Fatal("native receipt cross-module reader is not registered")
	}
}

var compatGrantSerial atomic.Int64

func ingestPlan(s mirror.Snapshot, now time.Time, policy mirror.RefreshPolicy) (mirror.ScanPlan, error) {
	p, err := mirror.PlanRefresh(s, now, policy)
	if err == nil {
		p.AttemptGrant = mirror.AttemptGrant{ID: fmt.Sprint(compatGrantSerial.Add(1)), SourceID: "fixture-source", Generation: "fixture-generation", CallerID: "fixture-caller", AccessEpoch: "access-1", ConfigEpoch: "config-1", PolicyClass: "read", PolicyEpoch: "policy-1", Scope: s.Scope, MaxHTTPAttempts: 1000, ExpiresAt: now.Add(24 * time.Hour)}
	}
	return p, err
}
