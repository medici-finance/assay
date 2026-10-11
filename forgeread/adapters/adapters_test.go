package adapters_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/forgeread"
	"github.com/medici-finance/assay/forgeread/adapters"
	ghadapter "github.com/medici-finance/assay/forgeread/adapters/github"
	gladapter "github.com/medici-finance/assay/forgeread/adapters/gitlab"
)

type observed struct {
	Pages   []json.RawMessage `json:"pages"`
	Status  int               `json:"status"`
	Endless bool              `json:"endless"`
}

// fixtureAccess serves the shared fake-provider observations (testdata/parity) one page at a time
// and records every request.
type fixtureAccess struct {
	repos    map[string]observed
	requests []string
}

func (f *fixtureAccess) GetPage(_ context.Context, repo, rel string, q url.Values) (adapters.Page, error) {
	f.requests = append(f.requests, repo+"/"+rel+"?"+q.Encode())
	o, ok := f.repos[repo]
	if !ok {
		return adapters.Page{}, fmt.Errorf("404 %s", repo)
	}
	if o.Status != 0 {
		return adapters.Page{}, fmt.Errorf("provider answered HTTP %d", o.Status)
	}
	page, _ := strconv.Atoi(q.Get("page"))
	if o.Endless {
		return adapters.Page{Body: []byte("[]"), More: true}, nil
	}
	if page < 1 || page > len(o.Pages) {
		return adapters.Page{}, fmt.Errorf("unexpected page %d", page)
	}
	return adapters.Page{Body: o.Pages[page-1], More: page < len(o.Pages)}, nil
}

func loadFixtures(t *testing.T) (map[string]map[string]observed, map[string]struct {
	Issues      map[string][]forgeread.Issue `json:"issues"`
	Unavailable map[string]string            `json:"unavailable"`
}, []string) {
	t.Helper()
	raw, err := os.ReadFile("../testdata/parity/observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var obs map[string]map[string]observed
	if err := json.Unmarshal(raw, &obs); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile("../testdata/parity/expected.json")
	if err != nil {
		t.Fatal(err)
	}
	var exp struct {
		Repos []string `json:"repos"`
		GH    struct {
			Issues      map[string][]forgeread.Issue `json:"issues"`
			Unavailable map[string]string            `json:"unavailable"`
		} `json:"github"`
		GL struct {
			Issues      map[string][]forgeread.Issue `json:"issues"`
			Unavailable map[string]string            `json:"unavailable"`
		} `json:"gitlab"`
	}
	if err := json.Unmarshal(raw, &exp); err != nil {
		t.Fatal(err)
	}
	type E = struct {
		Issues      map[string][]forgeread.Issue `json:"issues"`
		Unavailable map[string]string            `json:"unavailable"`
	}
	return obs, map[string]E{"github": exp.GH, "gitlab": exp.GL}, exp.Repos
}

// TestAdaptersMatchSharedObservations runs each adapter over the shared fake-provider observations
// and compares with the recorded expectation the CLI and statusgen paths are held to as well.
// regression: #2395
func TestAdaptersMatchSharedObservations(t *testing.T) {
	obs, exp, repos := loadFixtures(t)
	for provider, read := range map[string]func(adapters.Access) func(context.Context, string) ([]forgeread.Issue, error){
		"github": func(a adapters.Access) func(context.Context, string) ([]forgeread.Issue, error) {
			return ghadapter.New(a, nil).OpenIssues
		},
		"gitlab": func(a adapters.Access) func(context.Context, string) ([]forgeread.Issue, error) {
			return gladapter.New(a, nil).OpenIssues
		},
	} {
		t.Run(provider, func(t *testing.T) {
			acc := &fixtureAccess{repos: obs[provider]}
			fetch := read(acc)
			for _, repo := range repos {
				got, err := fetch(context.Background(), repo)
				if want, ok := exp[provider].Issues[repo]; ok {
					if err != nil {
						t.Fatalf("%s: %v", repo, err)
					}
					if !reflect.DeepEqual(strip(got), strip(want)) {
						t.Errorf("%s:\n got %+v\nwant %+v", repo, got, want)
					}
					continue
				}
				class := exp[provider].Unavailable[repo]
				if err == nil {
					t.Fatalf("%s: read %d issues, want unavailable (%s)", repo, len(got), class)
				}
				if got != nil {
					t.Errorf("%s: a failed read must return NO list, got %+v", repo, got)
				}
				if (class == "truncated") != adapters.IsTruncated(err) {
					t.Errorf("%s: err = %v, truncated=%v want class %s", repo, err, adapters.IsTruncated(err), class)
				}
			}
		})
	}
}

// strip drops the Go-API-only fields so wire expectations compare.
func strip(in []forgeread.Issue) []forgeread.Issue {
	out := make([]forgeread.Issue, len(in))
	for i, is := range in {
		is.UpdatedAt = ""
		if len(is.Labels) == 0 {
			is.Labels = nil
		}
		out[i] = is
	}
	return out
}

// regression: #2395
func TestGitHubWalkContinuesOnEitherSignal(t *testing.T) {
	full := make([]string, 100)
	for i := range full {
		full[i] = fmt.Sprintf(`{"number":%d,"title":"t","user":{"login":"u","id":1}}`, i+1)
	}
	// Page 1 is FULL with no more-pages signal; page 2 is SHORT but carries the signal; page 3 ends.
	acc := &sequenceAccess{pages: []adapters.Page{
		{Body: []byte("[" + strings.Join(full, ",") + "]")},
		{Body: []byte(`[{"number":200,"title":"t","user":{"login":"u","id":1}}]`), More: true},
		{Body: []byte(`[{"number":201,"title":"t","user":{"login":"u","id":1}}]`)},
	}}
	// A full page with no signal must not END the walk (a page shortened under load looks the same
	// from the other side); the third request proves both signals are honoured.
	got, err := ghadapter.New(acc, nil).OpenIssues(context.Background(), "o/r")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 102 || acc.calls != 3 {
		t.Fatalf("got %d issues in %d requests, want 102 in 3", len(got), acc.calls)
	}
}

type sequenceAccess struct {
	pages []adapters.Page
	calls int
}

func (s *sequenceAccess) GetPage(context.Context, string, string, url.Values) (adapters.Page, error) {
	if s.calls >= len(s.pages) {
		return adapters.Page{}, errors.New("walked past the end")
	}
	p := s.pages[s.calls]
	s.calls++
	return p, nil
}

func TestSkipPolicyIsInjectedNotBuiltIn(t *testing.T) {
	body := []byte(`[{"number":1,"title":"keep","user":{"login":"u","id":1}},{"number":2,"title":"drop me","user":{"login":"bot","id":2}}]`)
	skip := func(title, login string) bool { return login == "bot" }
	got, err := ghadapter.New(&sequenceAccess{pages: []adapters.Page{{Body: body}}}, skip).OpenIssues(context.Background(), "o/r")
	if err != nil || len(got) != 1 || got[0].Number != 1 {
		t.Fatalf("skip hook: %+v %v", got, err)
	}
	got, err = ghadapter.New(&sequenceAccess{pages: []adapters.Page{{Body: body}}}, nil).OpenIssues(context.Background(), "o/r")
	if err != nil || len(got) != 2 {
		t.Fatalf("no skip hook must list everything: %+v %v", got, err)
	}
}

func TestMalformedPageIsAnErrorNotAnEmptyList(t *testing.T) {
	for name, fn := range map[string]func(adapters.Access) error{
		"github": func(a adapters.Access) error {
			_, err := ghadapter.New(a, nil).OpenIssues(context.Background(), "o/r")
			return err
		},
		"gitlab": func(a adapters.Access) error {
			_, err := gladapter.New(a, nil).OpenIssues(context.Background(), "o/r")
			return err
		},
	} {
		if err := fn(&sequenceAccess{pages: []adapters.Page{{Body: []byte(`{"message":"oops"}`)}}}); err == nil {
			t.Errorf("%s: a page that is not a list came back as an empty listing", name)
		}
	}
}
