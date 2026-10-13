package main

// shared_parity_test.go — forge-neutral brief 36: the CLI half of the SDK/CLI parity row.
//
// The SAME fake-provider observations (forgeread/testdata/parity) that the adapters' own tests and
// statusgen's parity test read are served here over HTTP to the REAL GitHubForge / GitLabForge,
// run through the REAL deskread, and the stdout envelope is parsed with the SDK's own parser. What
// comes out must equal the recorded expectation, which is also what the SDK path produces from the
// same observations.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/forgeread"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

type parityObserved struct {
	Pages   []json.RawMessage `json:"pages"`
	Status  int               `json:"status"`
	Endless bool              `json:"endless"`
}

type parityExpect struct {
	Issues      map[string][]forgeread.Issue `json:"issues"`
	Unavailable map[string]string            `json:"unavailable"`
}

func loadParity(t *testing.T) (map[string]map[string]parityObserved, map[string]parityExpect, []string) {
	t.Helper()
	raw, err := os.ReadFile("../../../../forgeread/testdata/parity/observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var obs map[string]map[string]parityObserved
	if err := json.Unmarshal(raw, &obs); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile("../../../../forgeread/testdata/parity/expected.json")
	if err != nil {
		t.Fatal(err)
	}
	var exp struct {
		Repos  []string     `json:"repos"`
		GitHub parityExpect `json:"github"`
		GitLab parityExpect `json:"gitlab"`
	}
	if err := json.Unmarshal(raw, &exp); err != nil {
		t.Fatal(err)
	}
	return obs, map[string]parityExpect{"github": exp.GitHub, "gitlab": exp.GitLab}, exp.Repos
}

func parityServer(t *testing.T, provider string, repos map[string]parityObserved) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var slug string
		if provider == "github" {
			rest := strings.TrimPrefix(r.URL.Path, "/repos/")
			slug = strings.TrimSuffix(rest, "/issues")
		} else {
			rest := strings.TrimPrefix(r.URL.EscapedPath(), "/api/v4/projects/")
			slug, _ = url.PathUnescape(strings.TrimSuffix(rest, "/issues"))
		}
		o, ok := repos[slug]
		if !ok {
			http.Error(w, "unknown repo "+slug, http.StatusNotFound)
			return
		}
		if o.Status != 0 {
			http.Error(w, "fixture failure", o.Status)
			return
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		body, more := json.RawMessage("[]"), o.Endless
		if !o.Endless {
			if page < 1 || page > len(o.Pages) {
				http.Error(w, "unexpected page", http.StatusBadRequest)
				return
			}
			body, more = o.Pages[page-1], page < len(o.Pages)
		}
		w.Header().Set("Content-Type", "application/json")
		if more {
			if provider == "github" {
				w.Header().Set("Link", fmt.Sprintf(`<http://%s%s?page=%d>; rel="next"`, r.Host, r.URL.Path, page+1))
			} else {
				w.Header().Set("X-Next-Page", strconv.Itoa(page+1))
			}
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestDeskreadMatchesSharedObservations is the CLI arm of TestReaderSDKCLIParity.
// regression: #2395
func TestDeskreadMatchesSharedObservations(t *testing.T) {
	obs, exp, repos := loadParity(t)
	for _, provider := range []string{"github", "gitlab"} {
		t.Run(provider, func(t *testing.T) {
			srv := parityServer(t, provider, obs[provider])
			forges := map[string]deskkit.Forge{}
			for _, r := range repos {
				if provider == "github" {
					forges[r] = &deskkit.GitHubForge{Token: testToken, BaseURL: srv.URL, Client: srv.Client()}
				} else {
					forges[r] = &deskkit.GitLabForge{Token: testToken, BaseURL: srv.URL, Client: srv.Client()}
				}
			}
			defer stubForges(t, forges, nil)()
			args := []string{"issues"}
			for _, r := range repos {
				args = append(args, "--repo", r)
			}
			var out, errb strings.Builder
			code := run(args, &out, &errb)
			if code != deskkit.ExitOK {
				t.Fatalf("exit=%d stderr=%s", code, errb.String())
			}
			checkParityGolden(t, provider, out.String())
			res, err := forgeread.ParseIssuesEnvelope([]byte(out.String()), repos)
			if err != nil {
				t.Fatalf("the SDK cannot read the verb's own envelope: %v", err)
			}
			if len(res.Issues)+len(res.Unavailable) != len(repos) {
				t.Fatalf("not every repo exactly once: %+v", res)
			}
			for repo, want := range exp[provider].Issues {
				got, ok := res.Issues[repo]
				if !ok {
					t.Errorf("%s missing from the read set", repo)
					continue
				}
				if len(got) == 0 && len(want) == 0 {
					continue
				}
				for i := range got {
					got[i].UpdatedAt = ""
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("%s:\n got %+v\nwant %+v", repo, got, want)
				}
			}
			for repo, class := range exp[provider].Unavailable {
				var found *forgeread.Unavailable
				for i := range res.Unavailable {
					if res.Unavailable[i].Repo == repo {
						found = &res.Unavailable[i]
					}
				}
				if found == nil {
					t.Errorf("%s should be unavailable (%s) but was read", repo, class)
					continue
				}
				if string(found.Class) != class {
					t.Errorf("%s class = %q, want %q (reason %q)", repo, found.Class, class, found.Reason)
				}
			}
		})
	}
}

// checkParityGolden holds the verb's stdout (identity record removed) to the golden envelope in
// the shared fixtures — the very bytes statusgen's parity test feeds its deskread stub. Set
// UPDATE_PARITY_GOLDEN=1 to rewrite it after a deliberate contract change.
func checkParityGolden(t *testing.T, provider, stdout string) {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("stdout is not JSON: %v", err)
	}
	delete(doc, "identity")
	got, _ := json.MarshalIndent(doc, "", "  ")
	got = append(got, '\n')
	path := "../../../../forgeread/testdata/parity/deskread-" + provider + ".json"
	if os.Getenv("UPDATE_PARITY_GOLDEN") == "1" {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no golden envelope: %v", err)
	}
	if string(want) != string(got) {
		t.Errorf("the verb's %s envelope drifted from the golden shared with the SDK and statusgen:\n got %s\nwant %s", provider, got, want)
	}
}
