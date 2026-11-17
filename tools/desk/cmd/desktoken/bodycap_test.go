package main

// bodycap_test.go — the coverage reads go through the shared forge byte cap: an over-cap page
// is an error, never a truncated list read as complete.

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// bigBodyTransport answers every request with HTTP 200 and a JSON body padded with spaces.
type bigBodyTransport struct{ body string }

func (b bigBodyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{}, Request: r,
		Body: io.NopCloser(strings.NewReader(b.body + strings.Repeat(" ", 200))),
	}, nil
}

// regression: #6730
func TestCoverageOverCapBody(t *testing.T) {
	t.Cleanup(deskkit.SetForgeBodyCapForTest(64))
	installs, err := listInstallations("example-jwt", &http.Client{Transport: bigBodyTransport{`[]`}})
	if err == nil || !deskkit.IsForgeBodyOverCap(err) {
		t.Errorf("installations: an over-cap answer: %v, %v; want the over-cap refusal", installs, err)
	}
	repos, err := listInstallationRepos("example-credential", &http.Client{Transport: bigBodyTransport{`{"repositories":[]}`}})
	if err == nil || !deskkit.IsForgeBodyOverCap(err) {
		t.Errorf("repositories: an over-cap page: %v, %v; want the over-cap refusal", repos, err)
	}
}

// fullPageTransport answers every repositories page with a full page.
type fullPageTransport struct{ n *int }

func (f fullPageTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	*f.n++
	items := make([]string, coveragePerPage)
	for i := range items {
		items[i] = `{"full_name":"example-org/r"}`
	}
	return &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{}, Request: r,
		Body: io.NopCloser(strings.NewReader(`{"repositories":[` + strings.Join(items, ",") + `]}`)),
	}, nil
}

// A forge that always answers a full page cannot hold the walk open.
//
// regression: #6730
func TestReposPageCeiling(t *testing.T) {
	prevMax := maxRepoPages
	maxRepoPages = 3
	t.Cleanup(func() { maxRepoPages = prevMax })
	n := 0
	repos, err := listInstallationRepos("example-credential", &http.Client{Transport: fullPageTransport{&n}})
	if err == nil || repos != nil {
		t.Fatalf("a walk past the page ceiling: %d repos, %v; want an error, no list", len(repos), err)
	}
	if n > 3 {
		t.Errorf("the walk read %d pages past a ceiling of 3", n)
	}
}
