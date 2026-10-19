package deskkit

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPerfAccountTransport(t *testing.T) {
	perfFixture(t)
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		switch {
		case strings.HasPrefix(r.URL.Path, "/users/"):
			fmt.Fprint(w, `{"id":2001,"login":"example-driver"}`)
		case r.URL.Path == "/api/v4/users":
			fmt.Fprint(w, `[{"id":2001,"username":"example-driver"}]`)
		case r.URL.Path == "/repos/example-org/tracker":
			fmt.Fprint(w, `{"visibility":"public"}`)
		default:
			t.Errorf("unexpected request %s", r.URL.String())
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client := server.Client()
	before := client.Transport
	PerfRun("deskroster", []string{"check"}, []string{"check"}, func([]string) int {
		gh := HTTPAccountFetcher{Token: "fixture-token", BaseURL: server.URL, Client: client}
		if a, err := gh.GetAccount("example-driver"); err != nil || a.ID != 2001 {
			t.Fatal(a, err)
		}
		gl := HTTPGitLabAccountFetcher{Token: "fixture-token", BaseURL: server.URL, Client: client}
		if a, err := gl.GetAccount("example-driver"); err != nil || a.ID != 2001 {
			t.Fatal(a, err)
		}
		vis := HTTPRepoInfoFetcher{Token: "fixture-token", BaseURL: server.URL, Client: client}
		if v, err := vis.RepoVisibility("example-org", "tracker"); err != nil || v != "public" {
			t.Fatal(v, err)
		}
		_, _, calls := PerfCounters()
		if calls != 3 || attempts != 3 {
			t.Fatalf("account/visibility attempts=%d wire=%d; want three", calls, attempts)
		}
		if client.Transport != before {
			t.Fatal("caller-owned client mutated")
		}
		return 0
	})
}
