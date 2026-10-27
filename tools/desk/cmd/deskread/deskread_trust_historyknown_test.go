package main

// deskread_trust_historyknown_test.go — the trust kind's additive bodyHistoryKnown field
// (desktools-v2/71 Task 8). An absent bodyEditedAt cannot be told apart from an unreported
// one, so the field carries TrustPayload.BodyHistoryKnown as each forge backend gave it, and
// is always rendered: a reader that needs "never edited" must see an explicit false.

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// gitlabGraphQLServer answers every POST /api/graphql with body.
func gitlabGraphQLServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/graphql" {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestDeskreadTrustBodyHistoryKnown(t *testing.T) {
	cases := []struct {
		name, provider, body string
		want                 bool
	}{
		{
			name: "github_reported_never_edited", provider: "github", want: true,
			body: `{"data":{"repository":{"issue":{"lastEditedAt":null,
			  "comments":{"pageInfo":{"hasNextPage":false},"nodes":[]}}}}}`,
		},
		{
			name: "github_unreported", provider: "github", want: false,
			body: `{"data":{"repository":{"issue":{
			  "comments":{"pageInfo":{"hasNextPage":false},"nodes":[]}}}}}`,
		},
		{
			name: "gitlab_reported", provider: "gitlab", want: true,
			body: `{"data":{"project":{"issue":{"updatedAt":"2026-09-09T09:00:00Z",
			  "comments":{"pageInfo":{"hasNextPage":false},"nodes":[]},
			  "activity":{"pageInfo":{"hasPreviousPage":false},"nodes":[
			    {"body":"added ~bug label","createdAt":"2026-09-01T10:00:00Z"}]}}}}}`,
		},
		{
			name: "gitlab_unreported", provider: "gitlab", want: false,
			body: `{"data":{"project":{"issue":{"updatedAt":"2026-09-09T09:00:00Z",
			  "comments":{"pageInfo":{"hasNextPage":false},"nodes":[]},
			  "activity":{"pageInfo":{},"nodes":[]}}}}}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var forge deskkit.Forge
			if tc.provider == "github" {
				srv := graphqlServer(t, tc.body, nil)
				forge = &deskkit.GitHubForge{Token: testToken, BaseURL: srv.URL, Client: srv.Client()}
			} else {
				srv := gitlabGraphQLServer(t, tc.body)
				forge = &deskkit.GitLabForge{Token: testToken, BaseURL: srv.URL, Client: srv.Client()}
			}
			defer stubForges(t, map[string]deskkit.Forge{"example-org/alpha": forge}, nil)()

			var out, errb strings.Builder
			code := run([]string{"trust", "--issue", "example-org/alpha#7"}, &out, &errb)
			if code != deskkit.ExitOK {
				t.Fatalf("exit=%d, want %d; stderr=%s", code, deskkit.ExitOK, errb.String())
			}
			env, _, _ := runItemEnvelope(t, "trust", "--issue", "example-org/alpha#7")
			if len(env.Items) != 1 || env.Items[0].Trust == nil {
				t.Fatalf("items=%+v partial=%+v, want one trust item", env.Items, env.Partial)
			}
			if got := env.Items[0].Trust.BodyHistoryKnown; got != tc.want {
				t.Errorf("bodyHistoryKnown = %v, want %v", got, tc.want)
			}
			// Always rendered: an explicit false, never an omitted key a reader could take
			// for "not reported by this build".
			wantKey := `"bodyHistoryKnown":` + map[bool]string{true: "true", false: "false"}[tc.want]
			if compact := strings.Join(strings.Fields(out.String()), ""); !strings.Contains(compact, wantKey) {
				t.Errorf("envelope does not carry %s:\n%s", wantKey, out.String())
			}
		})
	}
}
