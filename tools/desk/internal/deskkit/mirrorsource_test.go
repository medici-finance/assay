package deskkit

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/adapters"
)

// incrementalFixture drives the actual native backend and its admitted Access.
// No production endpoint or credential is involved.
func incrementalFixture(t *testing.T, provider ForgeKind, kind MirrorRecordKind, failPage int, endless bool) (Forge, *atomic.Int64) {
	t.Helper()
	hits := &atomic.Int64{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method %s", r.Method)
			w.WriteHeader(405)
			return
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page == failPage {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		rel := "issues"
		if kind == MirrorChanges {
			rel = "pulls"
			if provider == ForgeGitLab {
				rel = "merge_requests"
			}
		}
		if !strings.HasSuffix(r.URL.Path, "/"+rel) {
			t.Errorf("unexpected route %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		if r.URL.Query().Get("state") != "all" {
			t.Errorf("state filter: %s", r.URL.RawQuery)
		}
		if page < 3 || endless {
			if provider == ForgeGitHub {
				w.Header().Set("Link", `<https://example.invalid/next>; rel="next"`)
			} else {
				w.Header().Set("X-Next-Page", strconv.Itoa(page+1))
			}
		}
		if endless {
			_, _ = w.Write([]byte("[]"))
			return
		}
		stamp := time.Date(2026, 10, 1, 12, 0, 4-page, 0, time.UTC).Format(time.RFC3339)
		if provider == ForgeGitHub {
			extra := ""
			if kind == MirrorChanges {
				extra = `,"head":{"sha":"head","ref":"topic"},"base":{"sha":"base","ref":"main","repo":{"full_name":"o/r"}}`
			}
			_, _ = fmt.Fprintf(w, `[{"number":%d,"state":"open","title":"record","body":"body","updated_at":%q,"user":{"login":"author","id":7,"type":"Bot"}%s}]`, page, stamp, extra)
		} else {
			extra := ""
			if kind == MirrorChanges {
				extra = `,"sha":"head","source_branch":"topic","target_branch":"main"`
			}
			_, _ = fmt.Fprintf(w, `[{"iid":%d,"state":"opened","title":"record","description":"body","updated_at":%q,"author":{"username":"author","id":7,"bot":true}%s}]`, page, stamp, extra)
		}
	}))
	t.Cleanup(srv.Close)
	if provider == ForgeGitHub {
		return &GitHubForge{Token: ciFixtureToken, BaseURL: srv.URL, Client: srv.Client(), onlyRepo: "o/r"}, hits
	}
	return &GitLabForge{Token: glTestToken, BaseURL: srv.URL, Client: srv.Client(), onlyRepo: "o/r"}, hits
}

// regression: F-incremental-source
func TestIncrementalReadComposition(t *testing.T) {
	for _, provider := range []ForgeKind{ForgeGitHub, ForgeGitLab} {
		for _, kind := range []MirrorRecordKind{MirrorIssues, MirrorChanges} {
			t.Run(string(provider)+"/"+string(kind), func(t *testing.T) {
				f, hits := incrementalFixture(t, provider, kind, 0, false)
				source, err := NewMirrorReadSource(ReadOnly(f), ForgeResolution{Kind: provider, Repo: ForgeRepo{Owner: "o", Name: "r"}})
				if err != nil {
					t.Fatal(err)
				}
				q := adapters.UpdateQuery{Since: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)}
				got, err := source.Read(context.Background(), kind, q)
				if err != nil {
					t.Fatal(err)
				}
				if hits.Load() != 3 || got.PageCount != 3 || got.Incomplete || got.ConditionalSupported || got.Provider != string(provider) || got.Repository != "o/r" || got.Kind != kind {
					t.Fatalf("source/requests: %+v %d", got, hits.Load())
				}
				if kind == MirrorIssues {
					if len(got.Issues) != 3 || len(got.Changes) != 0 || got.Issues[0].Author.ID != 7 {
						t.Fatalf("issue mapping: %+v", got)
					}
				} else {
					if len(got.Changes) != 3 || len(got.Issues) != 0 || got.Changes[0].HeadSHA != "head" {
						t.Fatalf("change mapping: %+v", got)
					}
				}
				before := hits.Load()
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				if _, err := source.Read(ctx, kind, q); err == nil || hits.Load() != before {
					t.Fatalf("canceled request: %v hits=%d", err, hits.Load())
				}
			})
		}
	}
	for _, provider := range []ForgeKind{ForgeGitHub, ForgeGitLab} {
		for _, kind := range []MirrorRecordKind{MirrorIssues, MirrorChanges} {
			f, hits := incrementalFixture(t, provider, kind, 2, false)
			source, _ := NewMirrorReadSource(f, ForgeResolution{Kind: provider, Repo: ForgeRepo{Owner: "o", Name: "r"}})
			if got, err := source.Read(context.Background(), kind, adapters.UpdateQuery{}); err == nil || got.Issues != nil || got.Changes != nil || !IsForgeForbidden(err) || hits.Load() != 2 {
				t.Fatalf("permission page failure: %+v %v %d", got, err, hits.Load())
			}
			f, _ = incrementalFixture(t, provider, kind, 0, true)
			source, _ = NewMirrorReadSource(f, ForgeResolution{Kind: provider, Repo: ForgeRepo{Owner: "o", Name: "r"}})
			if got, err := source.Read(context.Background(), kind, adapters.UpdateQuery{}); err != nil || !got.Incomplete {
				t.Fatalf("ceiling became complete: %+v %v", got, err)
			}
		}
	}
}

// regression: F-incremental-source
func TestIncrementalReadLowerScope(t *testing.T) {
	for _, provider := range []ForgeKind{ForgeGitHub, ForgeGitLab} {
		for _, kind := range []MirrorRecordKind{MirrorIssues, MirrorChanges} {
			t.Run(string(provider)+"/"+string(kind), func(t *testing.T) {
				f, hits := incrementalFixture(t, provider, kind, 0, false)
				// A valid request must reach the fake before zero-request refusals count.
				source, _ := NewMirrorReadSource(f, ForgeResolution{Kind: provider, Repo: ForgeRepo{Owner: "o", Name: "r"}})
				if _, err := source.Read(context.Background(), kind, adapters.UpdateQuery{}); err != nil || hits.Load() == 0 {
					t.Fatalf("positive control: %v %d", err, hits.Load())
				}
				before := hits.Load()
				for _, repo := range []ForgeRepo{{Owner: "o", Name: "other"}, {Owner: "o", Name: "r-lookalike"}, {Owner: "o", Name: "r/extra"}} {
					var err error
					// Bypass MirrorReadSource and reach the actual backend's adapter path.
					if kind == MirrorIssues {
						_, err = f.IssuesUpdatedSince(context.Background(), repo, adapters.UpdateQuery{})
					} else {
						_, err = f.ChangesUpdatedSince(context.Background(), repo, adapters.UpdateQuery{})
					}
					if err == nil || !IsRefused(err) || hits.Load() != before {
						t.Fatalf("lower scope bypass: %+v %v requests=%d", repo, err, hits.Load())
					}
				}
			})
		}
	}
}

type wrongIncrementalForge struct {
	Forge
	result *adapters.IssueUpdates
}

func (f wrongIncrementalForge) IssuesUpdatedSince(context.Context, ForgeRepo, adapters.UpdateQuery) (adapters.IssueUpdates, error) {
	if f.result != nil {
		return *f.result, nil
	}
	return adapters.IssueUpdates{UpdateCoverage: adapters.UpdateCoverage{Provider: "gitlab", Repository: "other/repo", PageCount: 1}}, nil
}

// regression: F-incremental-source
func TestIncrementalSourceProvenance(t *testing.T) {
	source := &MirrorReadSource{forge: wrongIncrementalForge{}, provider: ForgeGitHub, repo: ForgeRepo{Owner: "o", Name: "r"}}
	if _, err := source.Read(context.Background(), MirrorIssues, adapters.UpdateQuery{}); err == nil {
		t.Fatal("foreign source passed through")
	}
	unsafe := adapters.IssueUpdates{UpdateCoverage: adapters.UpdateCoverage{Provider: "github", Repository: "o/r", PageCount: 1, Incomplete: true, ObservedUpTo: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)}}
	source.forge = wrongIncrementalForge{result: &unsafe}
	if _, err := source.Read(context.Background(), MirrorIssues, adapters.UpdateQuery{}); err == nil {
		t.Fatal("incomplete source exposed an advanceable bound")
	}

	if _, err := source.Read(context.Background(), "route/from/request", adapters.UpdateQuery{}); err == nil {
		t.Fatal("arbitrary source kind accepted")
	}
}

// regression: F-incremental-source
func TestIncrementalReadCancellation(t *testing.T) {
	for _, provider := range []ForgeKind{ForgeGitHub, ForgeGitLab} {
		for _, kind := range []MirrorRecordKind{MirrorIssues, MirrorChanges} {
			t.Run(string(provider)+"/"+string(kind), func(t *testing.T) {
				started := make(chan struct{})
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					close(started)
					select {
					case <-r.Context().Done():
					case <-time.After(3 * time.Second):
					}
				}))
				t.Cleanup(srv.Close)
				var f Forge = &GitHubForge{Token: ciFixtureToken, BaseURL: srv.URL, Client: srv.Client(), onlyRepo: "o/r"}
				if provider == ForgeGitLab {
					f = &GitLabForge{Token: glTestToken, BaseURL: srv.URL, Client: srv.Client(), onlyRepo: "o/r"}
				}
				source, err := NewMirrorReadSource(f, ForgeResolution{Kind: provider, Repo: ForgeRepo{Owner: "o", Name: "r"}})
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				done := make(chan error, 1)
				go func() { _, err := source.Read(ctx, kind, adapters.UpdateQuery{}); done <- err }()
				select {
				case <-started:
				case <-time.After(2 * time.Second):
					t.Fatal("read did not reach the fake")
				}
				cancel()
				select {
				case err := <-done:
					if !errors.Is(err, context.Canceled) {
						t.Fatalf("cancellation cause lost: %v", err)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("in-flight read ignored cancellation")
				}
			})
		}
	}
}
