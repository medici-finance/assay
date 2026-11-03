package deskkit

import (
	"context"
	"errors"
	"fmt"
	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/mirror/mirrortest"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

type ingestTransport struct {
	payload string
	status  int
	more    bool
	hits    int
	failure error
}

func (r *ingestTransport) RoundTrip(q *http.Request) (*http.Response, error) {
	r.hits++
	if r.failure != nil {
		return nil, r.failure
	}
	h := http.Header{}
	if r.more {
		h.Set("Link", `<https://fixture.example.invalid/next>; rel="next"`)
		h.Set("X-Next-Page", "2")
	}
	status := r.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{StatusCode: status, Status: http.StatusText(status), Header: h, Body: io.NopCloser(strings.NewReader(r.payload)), ContentLength: int64(len(r.payload)), Request: q}, nil
}
func ingestRow(provider ForgeKind, kind mirror.RecordKind, n int, stamp time.Time, known bool) string {
	common := fmt.Sprintf(`"title":"private-canary","updated_at":%q`, stamp.Format(time.RFC3339Nano))
	body := ""
	if known {
		body = `,"body":"","labels":[],"requested_reviewers":[]`
	}
	if provider == ForgeGitHub {
		extra := ""
		if kind == mirror.Changes {
			extra = `,"head":{"sha":"head"},"base":{"sha":"base"}`
		}
		return fmt.Sprintf(`{"number":%d,"state":"open","user":{"login":"author","id":7,"type":"Bot"},%s%s%s}`, n, common, body, extra)
	}
	body = ""
	if known {
		body = `,"description":"","labels":[],"reviewers":[]`
	}
	extra := ""
	if kind == mirror.Changes {
		extra = `,"sha":"head","source_branch":"topic","target_branch":"main"`
	}
	return fmt.Sprintf(`{"iid":%d,"state":"opened","author":{"username":"author","id":7,"bot":true},%s%s%s}`, n, common, body, extra)
}
func ingestFixture(t *testing.T, provider ForgeKind, kind mirror.RecordKind, rt http.RoundTripper, now *time.Time) (*mirrorIngestSource, mirror.Scope, *mirrortest.TestAdmission) {
	t.Helper()
	c := &http.Client{Transport: rt}
	var f Forge
	if provider == ForgeGitHub {
		f = &GitHubForge{Token: ciFixtureToken, BaseURL: "https://api.github.com", Client: c, onlyRepo: "o/r"}
	} else {
		f = &GitLabForge{Token: glTestToken, BaseURL: "https://gitlab.example.invalid", Client: c, onlyRepo: "o/r"}
	}
	read, e := NewMirrorReadSource(ReadOnly(f), ForgeResolution{Kind: provider, Repo: ForgeRepo{Owner: "o", Name: "r"}})
	if e != nil {
		t.Fatal(e)
	}
	source, e := NewMirrorIngestSource(read, func() time.Time { return *now })
	if e != nil {
		t.Fatal(e)
	}
	s := source.(*mirrorIngestSource)
	a := &mirrortest.TestAdmission{Name: "admission-a", Allowed: true}
	return s, mirror.Scope{Repository: s.repository, Kind: kind, AdmissionID: a.ID()}, a
}
func ingestApply(t *testing.T, store mirror.Store, s mirror.Source, a mirror.Admission, scope mirror.Scope, now time.Time, policy mirror.RefreshPolicy) mirror.Snapshot {
	t.Helper()
	old, e := store.Read(context.Background(), a, scope)
	if e != nil {
		t.Fatal(e)
	}
	p, e := mirror.PlanRefresh(old, now, policy)
	if e != nil {
		t.Fatal(e)
	}
	scan, e := s.Scan(context.Background(), a, scope, p)
	if e != nil {
		t.Fatal(e)
	}
	u, e := mirror.PlanScan(old, p, scan, now)
	if e != nil {
		t.Fatal(e)
	}
	next, e := store.Apply(context.Background(), a, u)
	if e != nil {
		t.Fatal(e)
	}
	answer, e := mirror.QueryScope(context.Background(), store, a, mirror.Query{Scope: scope, Now: now, MaxAge: time.Minute})
	if e != nil || !answer.Available {
		t.Fatalf("joined answer unavailable: %+v %v", answer, e)
	}
	return next
}

// regression: forge-neutral/50
func TestMirrorSourceFlow(t *testing.T) {
	for _, provider := range []ForgeKind{ForgeGitHub, ForgeGitLab} {
		for _, kind := range []mirror.RecordKind{mirror.Issues, mirror.Changes} {
			t.Run(string(provider)+"/"+string(kind), func(t *testing.T) {
				now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
				newest := now.Add(-time.Minute)
				older := now.Add(-2 * time.Minute)
				rt := &ingestTransport{payload: "[" + ingestRow(provider, kind, 1, newest, true) + "," + ingestRow(provider, kind, 2, older, false) + "]"}
				source, scope, a := ingestFixture(t, provider, kind, rt, &now)
				store := mirrortest.NewStore()
				policy := mirror.RefreshPolicy{Overlap: time.Minute, FullInventoryInterval: time.Hour, FailureBackoff: time.Minute}
				first := ingestApply(t, store, source, a, scope, now, policy)
				if rt.hits != 1 || len(first.Records) != 2 || !first.Records[0].BodyKnown || first.Records[0].Body != "" || first.Records[0].Labels == nil || first.Records[1].BodyKnown || first.Records[1].Labels != nil || !first.Coverage.ObservedUpTo.Equal(newest) {
					t.Fatalf("native mapping/presence: %+v requests=%d", first, rt.hits)
				}
				if first.Records[0].Author.ID != "7" || first.Records[0].Reviews.State != mirror.NeverRead || first.Records[0].Reviews.GitHub != nil || first.Records[0].Reviews.GitLab != nil {
					t.Fatalf("actor or unfetched reviews changed: %+v", first.Records[0])
				}
				if kind == mirror.Changes && (first.Records[0].HeadRevision != "head" || first.Records[0].RequestedReviewers == nil || first.Records[1].RequestedReviewers != nil) {
					t.Fatal("change or reviewer presence lost", first.Records)
				}
				now = now.Add(2 * time.Hour)
				rt.payload = "[" + ingestRow(provider, kind, 2, older, false) + "]"
				second := ingestApply(t, store, source, a, scope, now, policy)
				if !second.Coverage.ObservedUpTo.Equal(newest) || !second.Coverage.LastComplete.Equal(now) || second.Records[0].InInventory || second.Records[0].State != "open" || !second.Records[1].InInventory {
					t.Fatalf("older inventory regressed/closed: %+v", second)
				}
				now = now.Add(time.Minute)
				stamp := now.Add(-time.Second)
				p, _ := mirror.PlanRefresh(second, now, policy)
				if p.FullInventory || !p.Since.Equal(newest.Add(-time.Minute)) || !p.PriorObservedUpTo.Equal(newest) {
					t.Fatalf("incremental plan: %+v", p)
				}
				rt.payload = "[" + ingestRow(provider, kind, 2, stamp, true) + "]"
				third := ingestApply(t, store, source, a, scope, now, policy)
				if !third.Coverage.ObservedUpTo.Equal(stamp) || !third.Records[1].BodyKnown {
					t.Fatalf("incremental mapping: %+v", third)
				}
				rt.payload = "[" + ingestRow(provider, kind, 2, older, false) + "]"
				now = now.Add(time.Minute)
				fourth := ingestApply(t, store, source, a, scope, now, policy)
				if !fourth.Coverage.ObservedUpTo.Equal(stamp) || !fourth.Records[1].InInventory {
					t.Fatalf("filtered-empty regressed/retired: %+v", fourth)
				}
				now = now.Add(2 * time.Hour)
				rt.payload = "[]"
				empty := ingestApply(t, store, source, a, scope, now, policy)
				for _, row := range empty.Records {
					if row.InInventory || row.State != "open" {
						t.Fatalf("empty inferred closure: %+v", row)
					}
				}
				if !empty.Coverage.ObservedUpTo.Equal(stamp) || rt.hits != 5 {
					t.Fatalf("empty/delegation count: %+v hits=%d", empty, rt.hits)
				}
			})
		}
	}
}

// regression: forge-neutral/50
func TestMirrorSourceScope(t *testing.T) {
	for _, provider := range []ForgeKind{ForgeGitHub, ForgeGitLab} {
		now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
		rt := &ingestTransport{payload: "[]"}
		source, scope, a := ingestFixture(t, provider, mirror.Issues, rt, &now)
		store := mirrortest.NewStore()
		blank, _ := store.Read(context.Background(), a, scope)
		plan, _ := mirror.PlanRefresh(blank, now, mirror.RefreshPolicy{FullInventoryInterval: time.Hour})
		for _, change := range []func(*mirror.Scope, *mirror.ScanPlan){func(s *mirror.Scope, p *mirror.ScanPlan) { s.AdmissionID = "other" }, func(s *mirror.Scope, p *mirror.ScanPlan) { s.Repository.Host = "other.example.invalid" }, func(s *mirror.Scope, p *mirror.ScanPlan) { s.Repository.Path = "b/r" },
			func(s *mirror.Scope, p *mirror.ScanPlan) { s.Repository.Path = "O/r"; p.Scope = *s }, func(s *mirror.Scope, p *mirror.ScanPlan) { s.Kind = "unsupported" }, func(s *mirror.Scope, p *mirror.ScanPlan) {
				if s.Repository.Provider == mirror.GitHub {
					s.Repository.Provider = mirror.GitLab
				} else {
					s.Repository.Provider = mirror.GitHub
				}
			}, func(s *mirror.Scope, p *mirror.ScanPlan) { p.Scope.AdmissionID = "other" }, func(s *mirror.Scope, p *mirror.ScanPlan) { p.PriorObservedUpTo = now.Add(time.Hour) }, func(s *mirror.Scope, p *mirror.ScanPlan) { p.PriorObservedUpTo = now.Add(-time.Minute) }, func(s *mirror.Scope, p *mirror.ScanPlan) { p.Since = now; p.FullInventory = true }} {
			bad, bp := scope, plan
			change(&bad, &bp)
			if _, e := source.Scan(context.Background(), a, bad, bp); e == nil || rt.hits != 0 {
				t.Fatalf("invalid scope/plan sent: %+v %+v %v hits=%d", bad, bp, e, rt.hits)
			}
		}
		denied := &mirrortest.TestAdmission{Name: a.ID(), Allowed: false}
		var typedNil *mirrortest.TestAdmission
		for _, admission := range []mirror.Admission{nil, typedNil, denied} {
			if _, e := source.Scan(context.Background(), admission, scope, plan); !errors.Is(e, mirror.ErrPermission) || rt.hits != 0 {
				t.Fatal("admission not independently refused", e)
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, e := source.Scan(ctx, a, scope, plan); !errors.Is(e, context.Canceled) || rt.hits != 0 {
			t.Fatal("canceled source sent", e)
		}
		if _, e := source.Scan(context.Background(), a, scope, plan); e != nil || rt.hits != 1 {
			t.Fatal("positive scoped read failed", e)
		}
		rt.hits = 0
		source.source.repo = ForgeRepo{Owner: "b", Name: "r"}
		source.repository.Path = "b/r"
		foreign := scope
		foreign.Repository = source.repository
		bp := plan
		bp.Scope = foreign
		if _, e := source.Scan(context.Background(), a, foreign, bp); e == nil || rt.hits != 0 {
			t.Fatalf("native lower scope bypass: %v hits=%d", e, rt.hits)
		}
	}
}

// regression: forge-neutral/50
func TestMirrorSourceUncertainty(t *testing.T) {
	for _, provider := range []ForgeKind{ForgeGitHub, ForgeGitLab} {
		for _, scenario := range []string{"page-limit", "aggregate", "native", "malformed", "duplicate", "403", "404", "429", "oversized403", "oversized404", "transport"} {
			t.Run(string(provider)+"/"+scenario, func(t *testing.T) {
				now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
				rt := &ingestTransport{payload: "[" + ingestRow(provider, mirror.Issues, 1, now.Add(-time.Minute), true) + "]"}
				source, scope, a := ingestFixture(t, provider, mirror.Issues, rt, &now)
				store := mirrortest.NewStore()
				policy := mirror.RefreshPolicy{Overlap: time.Minute, FullInventoryInterval: time.Hour, FailureBackoff: time.Minute}
				initial := ingestApply(t, store, source, a, scope, now, policy)
				original := errors.New("fixture transport cause")
				switch scenario {
				case "page-limit":
					rt.payload = "[]"
					rt.more = true
				case "aggregate":
					rt.payload = strings.Replace("["+ingestRow(provider, mirror.Issues, 2, now.Add(-time.Minute), true)+"]", `"title":"private-canary"`, `"title":"`+strings.Repeat("x", (4<<20)+10)+`"`, 1)
					rt.more = true
				case "native":
					rt.payload = "[]" + strings.Repeat(" ", 8<<20)
				case "malformed":
					rt.payload = "[{"
				case "duplicate":
					row := ingestRow(provider, mirror.Issues, 1, now.Add(-time.Minute), true)
					rt.payload = "[" + row + "," + row + "]"
				case "403", "404", "429":
					rt.status, _ = strconv.Atoi(scenario)
					rt.payload = `{"message":"unqualified status"}`
				case "oversized403", "oversized404":
					rt.status, _ = strconv.Atoi(strings.TrimPrefix(scenario, "oversized"))
					rt.payload = strings.Repeat("x", (8<<20)+1)
				case "transport":
					rt.failure = original
				}
				now = now.Add(time.Minute)
				plan, e := mirror.PlanRefresh(initial, now, policy)
				if e != nil {
					t.Fatal(e)
				}
				scan, readErr := source.Scan(context.Background(), a, scope, plan)
				if scan.Complete || !scan.ObservedUpTo.IsZero() || scan.Failure == mirror.Permission || scan.ConditionalSupported || scan.NotModified {
					t.Fatalf("failure became complete/permission: complete=%v watermark=%v reason=%s error=%v", scan.Complete, scan.ObservedUpTo, scan.Failure, readErr)
				}
				if scenario == "page-limit" || scenario == "aggregate" {
					if readErr != nil || scan.Failure != mirror.Truncated {
						t.Fatalf("partial walk mapping: %+v %v", scan, readErr)
					}
				} else if readErr == nil || len(scan.Records) != 0 {
					t.Fatalf("failed source population: %+v %v", scan, readErr)
				}
				if strings.Contains(scenario, "403") || strings.Contains(scenario, "404") || scenario == "429" {
					var status *ForgeAPIError
					if !errors.As(readErr, &status) || status.Status != rt.status {
						t.Fatalf("status cause lost: %v", readErr)
					}
				}
				if scenario == "429" && scan.Failure != mirror.RateLimited {
					t.Fatal("qualified rate limit lost", scan.Failure, readErr)
				}
				if scenario == "transport" && !errors.Is(readErr, original) {
					t.Fatal("original transport cause lost", readErr)
				}
				update, e := mirror.PlanScan(initial, plan, scan, now)
				if e != nil || update.RetentionDecisionRequired || !update.Coverage.LastComplete.Equal(initial.Coverage.LastComplete) || len(update.NotInInventory) != 0 {
					t.Fatalf("failure erased coverage/selected retention: coverage=%+v retention=%v error=%v", update.Coverage, update.RetentionDecisionRequired, e)
				}
				next, e := store.Apply(context.Background(), a, update)
				canaryRetained := false
				for _, row := range next.Records {
					if row.ID.Number == 1 && row.Title == "private-canary" && row.InInventory {
						canaryRetained = true
					}
				}
				if e != nil || !canaryRetained || !next.Coverage.LastComplete.Equal(initial.Coverage.LastComplete) {
					t.Fatalf("private canary lost: rows=%d coverage=%+v error=%v", len(next.Records), next.Coverage, e)
				}
				now = now.Add(2 * time.Minute)
				rt.payload = "[]"
				rt.more = false
				rt.status = 0
				rt.failure = nil
				recovered := ingestApply(t, store, source, a, scope, now, policy)
				if !recovered.Coverage.LastComplete.Equal(now) {
					t.Fatalf("later success did not recover: %+v", recovered)
				}
			})
		}
	}
}
