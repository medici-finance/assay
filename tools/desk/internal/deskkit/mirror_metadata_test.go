package deskkit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/adapters"
	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/refresh"
	"github.com/medici-finance/assay/workflow/store"
)

func TestProjectedMirrorAccountsForCompleteInventory(t *testing.T) {
	for _, test := range []struct {
		name     string
		overflow bool
	}{
		{name: "complete"},
		{name: "aggregate-cap", overflow: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newNativeFixture(t, ForgeGitHub, mirror.Changes, func(w http.ResponseWriter, r *http.Request) {
				var request mirrorMetadataRequest
				if r.Method != http.MethodPost || r.URL.Path != "/graphql" || r.URL.RawQuery != "" ||
					json.NewDecoder(r.Body).Decode(&request) != nil || request.Query != mirrorMetadataQuery ||
					request.Variables.Owner != "o" || request.Variables.Name != "r" {
					t.Error("unexpected source request")
					http.Error(w, "invalid test request", http.StatusBadRequest)
					return
				}
				number, more := 1, true
				if request.Variables.After != nil {
					if *request.Variables.After != "page-2" {
						t.Error("unexpected continuation")
						http.Error(w, "invalid test continuation", http.StatusBadRequest)
						return
					}
					number, more = 2, false
				}
				padding := ""
				if test.overflow {
					padding = strings.Repeat("x", int(adapters.MaxUpdateBytes)/2)
				}
				writeMetadataFixture(t, w, more, number, padding)
			})
			source, err := NewGitHubMetadataMirrorIngestSource(fixture.source.source, fixture.admission.ID(),
				fixture.plan.AttemptGrant.SourceID, fixture.clock.Now, fixture.authority)
			if err != nil {
				t.Fatal(err)
			}
			got, err := source.Scan(context.Background(), fixture.admission, fixture.scope, fixture.plan)
			if fixture.requests.Load() != 2 || fixture.authority.observed != 2 || got.Attempts.HTTP.Value != 2 {
				t.Fatalf("unexpected native accounting: requests=%d observed=%d err=%v", fixture.requests.Load(), fixture.authority.observed, err)
			}
			if test.overflow {
				if err == nil || got.Complete || len(got.Records) != 0 || got.Attempts.DecodedBytes.Value != adapters.MaxUpdateBytes {
					t.Fatalf("overflow escaped the aggregate bound: complete=%t records=%d decoded=%d err=%v", got.Complete, len(got.Records), got.Attempts.DecodedBytes.Value, err)
				}
				return
			}
			if err != nil || !got.Complete || len(got.Records) != 2 || got.Attempts.DecodedBytes.Value <= 0 ||
				got.Records[0].BodyKnown || got.Records[1].BodyKnown ||
				got.Records[1].Labels == nil || got.Records[1].RequestedReviewers == nil {
				t.Fatalf("could not establish complete metadata without bodies: complete=%t records=%d err=%v", got.Complete, len(got.Records), err)
			}
		})
	}
}

func writeMetadataFixture(t *testing.T, w http.ResponseWriter, more bool, number int, padding string) {
	t.Helper()
	_, err := fmt.Fprintf(w, `{"data":{"repository":{"nameWithOwner":"o/r","pullRequests":{"totalCount":2,"pageInfo":{"hasNextPage":%t,"endCursor":"page-2"},"nodes":[{"number":%d,"title":"title","state":"OPEN","createdAt":"2026-11-02T12:00:00Z","updatedAt":"2026-11-02T12:00:00Z","headRefOid":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","baseRefOid":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","labels":{"totalCount":0,"nodes":[],"pageInfo":{"hasNextPage":false}},"reviewRequests":{"totalCount":0,"nodes":[],"pageInfo":{"hasNextPage":false}}}]}}},"padding":"%s"}`, more, number, padding)
	if err != nil {
		t.Errorf("write metadata response: %v", err)
	}
}

func checkProjectedMirrorWarmUp(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var requests atomic.Int64
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(requests.Add(1))
		if n == 1 {
			// Force the second reservation after the interactive waiter would expire.
			timer := time.NewTimer(1200 * time.Millisecond)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-r.Context().Done():
				return
			}
		}
		writeMetadataFixture(t, w, n == 1, n, "")
	}))
	defer upstream.Close()
	repo := ForgeRepo{Owner: "o", Name: "r"}
	backend := ReadOnly(&GitHubForge{Token: ciFixtureToken, BaseURL: upstream.URL, Client: upstream.Client(), onlyRepo: repo.Slug()})
	source, err := NewMirrorReadSource(backend, ForgeResolution{Kind: ForgeGitHub, Repo: repo})
	if err != nil {
		t.Fatal(err)
	}
	admission := &nativeAdmission{}
	admission.allowed.Store(true)
	scope := mirror.Scope{Repository: source.RepositoryID(), Kind: mirror.Changes, AdmissionID: admission.ID()}
	cfg := persistedMirrorConfig(admission.ID(), "native-source")
	cfg.Limits.WaitTimeout = time.Second
	cfg.Limits.FillTimeout = 8 * time.Second
	authority := &persistedCallerAuthority{config: cfg, repository: scope.Repository}
	limits := store.MirrorLimits{MaxRecords: 128, MaxBytes: 1 << 20}
	db := newPersistedMirrorSQL(t, cfg, authority, []string{"caller-a"}, true, limits)
	if _, err := db.operator.ProvisionRefresh(ctx, *db.initial); err != nil {
		t.Fatal(err)
	}
	ingest, err := NewGitHubMetadataMirrorIngestSource(source, admission.ID(), cfg.SourceID, time.Now, db.gate)
	if err != nil {
		t.Fatal(err)
	}
	coordinator, err := refresh.New(ctx, db.observations, ingest, db.gate, persistedClock{}, cfg.SourceID, authority)
	if err != nil {
		t.Fatal(err)
	}
	query := mirror.Query{Scope: scope, MaxAge: cfg.Limits.MaxFreshness}
	caller := persistedCaller(cfg, "caller-a")
	authority.denied.Store(true)
	if ready, err := coordinator.Warm(ctx, admission, query, caller); ready || err == nil || requests.Load() != 0 {
		t.Fatal("background warm-up bypassed current caller authority")
	}
	authority.denied.Store(false)
	result, err := coordinator.Warm(ctx, admission, query, caller)
	if err != nil || !result || requests.Load() != 2 {
		t.Fatalf("background fill did not finish: confirmed=%t requests=%d err=%v", result, requests.Load(), err)
	}
	snapshot, err := db.observations.Read(ctx, admission, scope)
	if err != nil || snapshot.Version == "" || snapshot.Coverage.State != mirror.Complete || len(snapshot.Records) != 2 {
		t.Fatalf("background fill did not persist: records=%d err=%v", len(snapshot.Records), err)
	}
	reused, err := coordinator.Read(ctx, admission, refresh.Query{Query: query}, caller)
	if err != nil || !reused.Fulfilled || reused.Snapshot.Version != snapshot.Version || requests.Load() != 2 {
		t.Fatalf("interactive read did not reuse the committed snapshot: requests=%d err=%v", requests.Load(), err)
	}
	observedAt := snapshot.Coverage.LastComplete
	first, err := db.observations.ReadPage(ctx, admission, scope, snapshot.Version, observedAt, 0, 1, 1<<20)
	if err != nil || !first.More || len(first.Snapshot.Records) != 1 ||
		first.Snapshot.Records[0].ID.Number != 1 || first.Snapshot.Records[0].BodyKnown ||
		first.Snapshot.Version != snapshot.Version || !first.Snapshot.Coverage.LastComplete.Equal(observedAt) {
		t.Fatalf("first page did not preserve the warmed snapshot: more=%t records=%d err=%v", first.More, len(first.Snapshot.Records), err)
	}
	next, err := db.observations.ReadPage(ctx, admission, scope, snapshot.Version, observedAt, first.Snapshot.Records[0].ID.Number, 1, 1<<20)
	if err != nil || next.More || len(next.Snapshot.Records) != 1 ||
		next.Snapshot.Records[0].ID.Number != 2 || next.Snapshot.Records[0].BodyKnown ||
		next.Snapshot.Version != snapshot.Version || !next.Snapshot.Coverage.LastComplete.Equal(observedAt) || requests.Load() != 2 {
		t.Fatalf("continuation did not reuse the warmed snapshot: more=%t records=%d requests=%d err=%v",
			next.More, len(next.Snapshot.Records), requests.Load(), err)
	}
	raw, err := db.fixture.ConnectOwned(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = raw.Close() }()
	var used, observed int
	err = raw.QueryRowContext(ctx, `SELECT (SELECT http_used FROM public.refresh_budgets),
		(SELECT count(*) FROM public.refresh_attempts WHERE observed AND response_complete)`).Scan(&used, &observed)
	if err != nil || used != 2 || observed != used {
		t.Fatalf("unexpected persisted attempt accounting: charged=%d observed=%d err=%v", used, observed, err)
	}
}
