package main

// kindcollision_test.go — #6264: scanloop's inbound items are ISSUES (deskmonitor inbound
// reads the open-issue listing), so the trust gate's author read asks for an issue. On GitLab
// issue #N and merge request !N are separate objects in separate sequences and routinely
// coexist. The bare-number resolver refuses such a number, which made every colliding inbound
// item COULD-NOT-CHECK and the whole drain pass blind. The backend here is the real GitLab
// forge against a recorded server, so the refusal is the resolver's own.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #6264
// Issue #9 is authored by one trusted login, merge request !9 by another, so reading the
// wrong object is visible in the author the probe returns.
func TestTrustProbeKindCollision(t *testing.T) {
	const repo = "example-org/tracker"
	project := "/api/v4/projects/" + strings.ReplaceAll(repo, "/", "%2F")
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		path := r.URL.EscapedPath()
		mu.Lock()
		seen = append(seen, r.Method+" "+path)
		mu.Unlock()
		rw.Header().Set("Content-Type", "application/json")
		switch path {
		case project + "/issues/9":
			_ = json.NewEncoder(rw).Encode(map[string]any{"id": 1009, "iid": 9, "title": "inbound issue",
				"state": "opened", "author": map[string]any{"id": 2001, "username": "ada"}})
		case project + "/merge_requests/9":
			_ = json.NewEncoder(rw).Encode(map[string]any{"id": 2009, "iid": 9, "title": "a change",
				"state": "opened", "author": map[string]any{"id": 2002, "username": "shared-agent"}})
		default:
			rw.WriteHeader(http.StatusNotFound) // the users API: account type stays unresolved
		}
	}))
	t.Cleanup(srv.Close)
	gl := deskkit.OutboundChecked(&deskkit.GitLabForge{Token: "fixture-token", BaseURL: srv.URL}, "worker")
	probe := forgeTrustProbe(func(string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		return gl, deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}, nil
	})

	adm := ApplyTrustGate([]Inbound{inbound(repo, 9)}, probe)
	if len(adm) != 1 || adm[0].State != AdmissionAdmitted || adm[0].Author != "ada" {
		t.Fatalf("#6264: issue #9 and merge request !9 both exist; the gate must read the ISSUE's author "+
			"(ada) and admit it, not refuse the number or read the change. got %+v", adm)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, req := range seen {
		if strings.Contains(req, "/merge_requests/") {
			t.Errorf("the trust read touched the merge request at the same number: %s", req)
		}
	}
}
