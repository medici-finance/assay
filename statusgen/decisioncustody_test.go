package main

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

// regression: #4823, review F1-decision-gate-ruling-read
// Exercise the production credential reader and both decision-gate layers, not
// a substituted client that resolves a ruling regardless of authentication.
func TestDecisionRulingCustody(t *testing.T) {
	for _, tc := range []struct {
		name, token                  string
		foreign, unavailable, edited bool
		wantRuled                    bool
	}{
		{name: "minted", token: "fixture-minted", wantRuled: true},
		{name: "no-permission-marker", token: storedLoginPlaceholder},
		{name: "missing"},
		{name: "wrong-credential", token: "fixture-wrong"},
		{name: "foreign-ruling", token: "fixture-minted", foreign: true},
		{name: "unavailable", token: "fixture-minted", unavailable: true},
		{name: "edited-ruling", token: "fixture-minted", edited: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			forge := dhForge("Option 1 for " + dhBoardID + ", as written.")
			if tc.edited {
				forge.commentBody = rlCommentJSONWith(rlHuman, "User", rlRepo, rlIssue, "Option 1 for "+dhBoardID+", as written.", rlPosted, "2026-09-02T10:00:00Z")
			}
			dhSeams(t, forge)
			root := dhFixture(t, unruledBase())
			ruling := dhRuledLines()
			if tc.foreign {
				ruling = strings.ReplaceAll(ruling, rlRepo, "example-org/foreign-repo")
			}
			dhWrite(t, root, map[string]string{
				dhReadmePath: dhReadme(dhRow("18", "implemented")),
				dhBriefPath:  dhFM("gate: human", ruling),
			})
			storedLog := slStubGH(t)
			slUnset(t, append(forgeTokenVars, "CELLCTL_GH_AMBIENT")...)
			t.Setenv("DESK_LOOP", "verify-desk")
			t.Setenv("GH_TOKEN", tc.token)
			// Route only the fixed production endpoint to the local fake;
			// reject everything except the invocation's marker credential.
			rt := slForge(t, forge)
			requests := 0
			http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				requests++
				if r.Method != http.MethodGet || r.URL.Host != "api.github.com" ||
					(r.URL.Path != "/repos/"+rlRepo+"/issues/comments/9001" && r.URL.Path != "/repos/"+rlRepo+"/issues/41") {
					t.Errorf("ruling credential reached an unexpected sink: %s %s", r.Method, r.URL)
				}
				if r.Header.Get("Authorization") != "Bearer fixture-minted" || tc.unavailable {
					status := http.StatusUnauthorized
					if tc.unavailable {
						status = http.StatusServiceUnavailable
					}
					return &http.Response{StatusCode: status, Header: make(http.Header),
						Body: io.NopCloser(strings.NewReader(`{"message":"unauthorized"}`)), Request: r}, nil
				}
				return rt.RoundTrip(r)
			})
			decisionGateClientFn = rulingForgeClient
			code, out := dhGate(t, root)
			if tc.wantRuled {
				if code != 0 || !strings.Contains(out, dhBoardID+" RULED") || requests != 2 {
					t.Fatalf("recorded ruling did not resolve: code=%d requests=%d report=%s", code, requests, out)
				}
			} else if code == 0 || !strings.Contains(out, dhBoardID+" REFUSED") {
				t.Fatalf("unproven ruling passed: code=%d report=%s", code, out)
			}
			if tc.token != "" && !tc.foreign && requests == 0 {
				t.Error("authenticated refusal did not exercise the production reader")
			}
			if tc.edited && !strings.Contains(out, "edited-comment") {
				t.Error("edited ruling refusal did not name its reason")
			}
			if (tc.token == "" || tc.foreign) && requests != 0 {
				t.Errorf("unavailable or foreign custody contacted the forge: %d requests", requests)
			}
			if strings.Contains(out, "fixture-minted") || strings.Contains(out, "fixture-wrong") {
				t.Error("credential leaked into ruling report")
			}
			if _, err := os.Stat(storedLog); !os.IsNotExist(err) {
				t.Error("consulted a stored human login")
			}
			if os.Getenv("DESK_LOOP") != "verify-desk" {
				t.Error("changed the caller role")
			}
		})
	}
}
