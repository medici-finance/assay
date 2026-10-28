package deskkit

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// regression: medici-finance/assay#3373
func TestDecisionMarkerSearchGitHubPages(t *testing.T) {
	gh := newB33GitHub(t, func(w http.ResponseWriter, r *http.Request, _ string) {
		q := r.URL.Query()
		if r.URL.Path != "/search/issues" || q.Get("q") != `repo:example-org/tracker is:issue "decision-gate" in:body` || q.Get("per_page") != "100" {
			t.Errorf("unexpected scope/query %s", r.URL)
		}
		if q.Get("sort") != "created" || q.Get("order") != "asc" {
			t.Errorf("unstable marker paging: %s", r.URL)
		}
		page := q.Get("page")
		if page == "1" {
			w.Header().Set("Link", `<https://example.invalid/?page=2>; rel="next"`)
		} else if page != "2" {
			t.Errorf("unexpected page %s", page)
		}
		fmt.Fprintf(w, `{"total_count":2,"incomplete_results":false,"items":[{"number":%s,"body":"<!-- decision-gate: s/01 -->"}]}`, page)
	})
	// Use the real wrapper: discovery remains a read, never a new credential path.
	got, err := ReadOnly(gh.forge()).SearchIssues(ForgeRepo{Owner: "example-org", Name: "tracker"}, SearchIssuesInput{DecisionGateMarkers: true})
	if err != nil || len(got) != 2 || got[1].Number != 2 || len(gh.seen) != 2 {
		t.Fatalf("walk: %+v %v requests=%v", got, err, gh.seen)
	}
}

// regression: medici-finance/assay#3373
func TestDecisionMarkerSearchGitHubIncomplete(t *testing.T) {
	for name, body := range map[string]string{
		"partial":          `{"total_count":1,"incomplete_results":true,"items":[{"number":1,"body":"decision-gate: s/01"}]}`,
		"missing_total":    `{"incomplete_results":false,"items":[]}`,
		"missing_coverage": `{"total_count":0,"items":[]}`,
		"null_items":       `{"total_count":0,"incomplete_results":false,"items":null}`,
		"missing_body":     `{"total_count":1,"incomplete_results":false,"items":[{"number":1}]}`,
		"search_ceiling":   `{"total_count":1001,"incomplete_results":false,"items":[]}`,
		"early_empty":      `{"total_count":2,"incomplete_results":false,"items":[]}`,
		"duplicate":        `{"total_count":2,"incomplete_results":false,"items":[{"number":1,"body":"x"},{"number":1,"body":"x"}]}`,
		"pull_request":     `{"total_count":1,"incomplete_results":false,"items":[{"number":1,"body":"x","pull_request":{}}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			gh := newB33GitHub(t, func(w http.ResponseWriter, _ *http.Request, _ string) { fmt.Fprint(w, body) })
			if got, err := gh.forge().SearchIssues(forgeTestRepo, SearchIssuesInput{DecisionGateMarkers: true}); err == nil || got != nil {
				t.Fatalf("partial became absence: %+v %v", got, err)
			}
		})
	}
	// More pages despite a matching total must not be silently dropped.
	gh := newB33GitHub(t, func(w http.ResponseWriter, _ *http.Request, _ string) {
		w.Header().Set("Link", `<https://example.invalid/>; rel="next"`)
		fmt.Fprint(w, `{"total_count":1,"incomplete_results":false,"items":[{"number":1,"body":"x"}]}`)
	})
	if got, err := gh.forge().SearchIssues(forgeTestRepo, SearchIssuesInput{DecisionGateMarkers: true}); err == nil || got != nil {
		t.Fatal("next page ignored", got, err)
	}
}

// regression: medici-finance/assay#3373
func TestDecisionMarkerSearchGitLabPages(t *testing.T) {
	pages := 0
	s := newB33GitHub(t, func(w http.ResponseWriter, r *http.Request, _ string) {
		pages++
		q := r.URL.Query()
		if q.Get("search") != "decision-gate" || q.Get("in") != "description" || q.Get("state") != "all" || q.Get("scope") != "all" || !strings.HasSuffix(r.URL.Path, "/issues") {
			t.Errorf("wrong candidate population: %s", r.URL)
		}
		if pages == 1 {
			w.Header().Set("X-Next-Page", "2")
		}
		fmt.Fprintf(w, `[{"id":100,"iid":%d,"description":"<!-- decision-gate: s/01 -->"}]`, pages)
	})
	f := &GitLabForge{Token: "fixture-token", BaseURL: s.srv.URL}
	got, err := ReadOnly(f).SearchIssues(forgeTestRepo, SearchIssuesInput{DecisionGateMarkers: true})
	if err != nil || len(got) != 2 || pages != 2 || got[1].Number != 2 {
		t.Fatalf("walk: %+v %v pages=%d", got, err, pages)
	}
}

// regression: medici-finance/assay#3373
func TestDecisionMarkerSearchGitLabIncomplete(t *testing.T) {
	for _, mode := range []string{"ceiling", "skip", "duplicate", "null"} {
		t.Run(mode, func(t *testing.T) {
			pages := 0
			s := newB33GitHub(t, func(w http.ResponseWriter, _ *http.Request, _ string) {
				pages++
				if mode == "null" {
					fmt.Fprint(w, `null`)
					return
				}
				n := pages
				switch mode {
				case "ceiling":
					w.Header().Set("X-Next-Page", fmt.Sprint(pages+1))
				case "skip":
					w.Header().Set("X-Next-Page", "3")
				case "duplicate":
					n = 1
					if pages == 1 {
						w.Header().Set("X-Next-Page", "2")
					}
				}
				fmt.Fprintf(w, `[{"id":100,"iid":%d,"description":"x"}]`, n)
			})
			got, err := (&GitLabForge{Token: "fixture-token", BaseURL: s.srv.URL}).SearchIssues(forgeTestRepo, SearchIssuesInput{DecisionGateMarkers: true})
			if err == nil || got != nil {
				t.Fatal("partial became absence", got, err)
			}
			if mode == "ceiling" && pages != gitlabMaxIssuePage {
				t.Fatal("page bound changed", pages)
			}
		})
	}
}

// regression: medici-finance/assay#3373
func TestDecisionMarkerSearchRejectsQualifiers(t *testing.T) {
	gh := newB33GitHub(t, func(http.ResponseWriter, *http.Request, string) { t.Error("invalid mode reached transport") })
	for _, f := range []DecisionForge{gh.forge(), &GitLabForge{Token: "fixture-token", BaseURL: gh.srv.URL}} {
		if _, err := f.SearchIssues(forgeTestRepo, SearchIssuesInput{Query: "repo:other/tracker", DecisionGateMarkers: true}); err == nil {
			t.Fatal("qualifier accepted")
		}
		for _, in := range []SearchIssuesInput{
			{DecisionGateMarkers: true, DecisionGateBrief: "s/01 repo:other/tracker"},
			{DecisionGateMarkers: true, DecisionGateBrief: "s/01\x00"},
			{DecisionGateBrief: "s/01"},
			{DecisionGateMarkers: true, DecisionGateBrief: `s/0"1`},
			{DecisionGateMarkers: true, DecisionGateBrief: `s/0\1`},
		} {
			if _, err := f.SearchIssues(forgeTestRepo, in); err == nil {
				t.Fatal("untyped marker syntax reached search", in)
			}
		}
	}
}

// regression: medici-finance/assay#3373
func TestDecisionMarkerGitHubPageBoundCause(t *testing.T) {
	pages := 0
	gh := newB33GitHub(t, func(w http.ResponseWriter, _ *http.Request, _ string) {
		pages++
		w.Header().Set("Link", `<https://example.invalid/>; rel="next"`)
		fmt.Fprintf(w, `{"total_count":100,"incomplete_results":false,"items":[{"number":%d,"body":"x"}]}`, pages)
	})
	_, err := gh.forge().SearchIssues(forgeTestRepo, SearchIssuesInput{DecisionGateMarkers: true})
	if pages != decisionSearchPages || !errors.Is(err, errDecisionSearchIncomplete) || errors.Is(err, errDecisionSearchCap) {
		t.Fatalf("short continuing pages mislabelled: pages=%d err=%v", pages, err)
	}
}

// Invalid tokens must have the same local refusal on the shared and selective
// paths, before any provider request; explicit issue refs remain independent.
// regression: medici-finance/assay#3373
func TestDecisionSweepInvalidSpellingParity(t *testing.T) {
	gh := newB33GitHub(t, func(http.ResponseWriter, *http.Request, string) { t.Error("invalid spelling reached provider") })
	for _, id := range []string{`s/0"1`, `s/0\1`, "s/01\x00", "s/0 1", "s/0\u20281"} {
		b := BriefAdmission{ID: id}
		f := gh.forge()
		want := BriefDecisionWait(f, forgeTestRepo, b, sweepRatifier)
		got := NewDecisionSweep(f, forgeTestRepo, sweepRatifier).Wait(b)
		if got != want || got.Waiting || !strings.Contains(got.Unknown, "refused locally") {
			t.Fatalf("invalid token parity: shared=%+v single=%+v", got, want)
		}
	}
}

// Exercise both canonical single-brief lookup and shared discovery through each
// native adapter. Only the remote HTTP endpoint is replaced; no live probe.
// regression: medici-finance/assay#3373
func TestDecisionMarkerSearchSingleAndSweepParity(t *testing.T) {
	for _, backend := range []string{"github", "gitlab", "gitlab-collision"} {
		t.Run(backend, func(t *testing.T) {
			id := "assay:tracker:s:01"
			bodies := map[int]string{1: "<!-- decision-gate: assay:otherproj:s:01 -->", 2: "<!-- decision-gate: " + id + " -->"}
			selective := 0
			s := newB33GitHub(t, func(w http.ResponseWriter, r *http.Request, _ string) {
				q := r.URL.Query()
				if strings.HasSuffix(r.URL.Path, "/merge_requests/2") {
					if backend == "gitlab-collision" {
						fmt.Fprint(w, `{"id":5,"iid":2,"state":"closed"}`)
					} else {
						w.WriteHeader(404)
						fmt.Fprint(w, `{"message":"fixture absent"}`)
					}
					return
				}
				if strings.HasSuffix(r.URL.Path, "/issues/2") {
					fmt.Fprintf(w, `{"id":2,"iid":2,"number":2,"state":"closed","labels":[],"body":%q,"description":%q}`, bodies[2], bodies[2])
					return
				}
				if backend == "github" {
					if strings.Contains(q.Get("q"), id) || strings.Contains(q.Get("q"), "s/01") {
						selective++
					}
					if q.Get("sort") != "created" || q.Get("order") != "asc" {
						t.Error("unstable GitHub paging")
					}
					fmt.Fprintf(w, `{"total_count":2,"incomplete_results":false,"items":[{"number":1,"body":%q},{"number":2,"body":%q}]}`, bodies[1], bodies[2])
					return
				}
				if q.Get("in") != "description" || q.Get("scope") != "all" || q.Get("state") != "all" || strings.Contains(q.Get("search"), "in:body") {
					t.Errorf("GitHub query syntax leaked to GitLab: %s", r.URL)
				}
				if q.Get("search") != "decision-gate" {
					selective++
					if q.Get("search") != "decision-gate: "+id && q.Get("search") != "decision-gate: s/01" {
						t.Errorf("wrong selective query %s", r.URL)
					}
				}
				fmt.Fprintf(w, `[{"id":1,"iid":1,"description":%q},{"id":2,"iid":2,"description":%q}]`, bodies[1], bodies[2])
			})
			var f DecisionForge = s.forge()
			if strings.HasPrefix(backend, "gitlab") {
				f = &GitLabForge{Token: "fixture-token", BaseURL: s.srv.URL, Client: s.srv.Client()}
			}
			b := BriefAdmission{ID: id}
			want := BriefDecisionWait(f, forgeTestRepo, b, sweepRatifier)
			got := NewDecisionSweep(f, forgeTestRepo, sweepRatifier).Wait(b)
			if backend == "gitlab-collision" {
				if got != want || got.Unknown == "" || got.Waiting {
					t.Fatalf("GitLab kind ambiguity was waived: shared=%+v single=%+v", got, want)
				}
				return
			}
			if got != want || got.Issue != 2 || got.Unknown != "" || got.Waiting || selective != 2 {
				t.Fatalf("typed lookup differs: shared=%+v single=%+v selective=%d", got, want, selective)
			}
		})
	}
}

// Missing minted credentials remain unverifiable and never reach a provider or
// become a clean absence; the shared and selective paths retain a useful cause.
// regression: medici-finance/assay#3373
func TestDecisionMissingMintedTokenCause(t *testing.T) {
	for _, backend := range []string{"github", "gitlab"} {
		t.Run(backend, func(t *testing.T) {
			gh := newB33GitHub(t, func(http.ResponseWriter, *http.Request, string) { t.Error("missing token reached provider") })
			var f DecisionForge
			if backend == "github" {
				f = &GitHubForge{BaseURL: gh.srv.URL}
			} else {
				f = &GitLabForge{BaseURL: gh.srv.URL}
			}
			hits, err := f.SearchIssues(forgeTestRepo, SearchIssuesInput{DecisionGateMarkers: true})
			if hits != nil || ExitCodeOf(err) != ExitUnverifiable {
				t.Fatalf("missing token changed error contract: hits=%+v err=%v", hits, err)
			}
			b := BriefAdmission{ID: "s/01"}
			single := BriefDecisionWait(f, forgeTestRepo, b, sweepRatifier)
			shared := NewDecisionSweep(f, forgeTestRepo, sweepRatifier).Wait(b)
			if single != shared || shared.Waiting || !strings.Contains(shared.Unknown, "minted provider token unavailable") {
				t.Fatalf("missing token lost cause or became absence: single=%+v shared=%+v", single, shared)
			}
			if len(gh.seen) != 0 {
				t.Fatalf("missing token reached provider: %v", gh.seen)
			}
		})
	}
}

// The four native missing-token origins retain exit 6 and a typed cause. This
// calls only guarded client constructors and an offline diff read, never a trigger.
// regression: medici-finance/assay#3373
func TestDecisionForgeTokenMissingOrigins(t *testing.T) {
	gh := newB33GitHub(t, func(http.ResponseWriter, *http.Request, string) { t.Error("missing token reached provider") })
	g := &GitHubForge{BaseURL: gh.srv.URL}
	l := &GitLabForge{BaseURL: gh.srv.URL}
	for _, tc := range []struct {
		name string
		read func() error
	}{
		{"github-client", func() error { _, err := g.restClient(); return err }},
		{"github-diff", func() error { _, err := g.ChangeDiff(forgeTestRepo, 1); return err }},
		{"gitlab-client", func() error { _, err := l.client(); return err }},
		{"gitlab-trigger-constructor", func() error { _, err := l.triggerClient(); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.read()
			if ExitCodeOf(err) != ExitUnverifiable || !IsUnverifiable(err) || !errors.Is(err, errForgeTokenMissing) {
				t.Fatalf("missing token lost exit/cause: %v", err)
			}
		})
	}
	if len(gh.seen) != 0 {
		t.Fatalf("missing token reached provider: %v", gh.seen)
	}
}
