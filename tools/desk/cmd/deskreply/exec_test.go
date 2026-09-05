package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestForgeRefusesWithoutMintedToken is the successor to TestGhRefusesWithoutMintedToken, and
// it hardens the same #562 finding one layer lower.
//
// THE PROPERTY, unchanged: deskreply must never fall back to an ambient forge identity — a
// stale keyring account, an operator's own login — instead of the freshly minted worker token.
// Tracing every forge call site shows they all run strictly after mintWorkerToken succeeds, so
// ghToken is never actually empty at a call in production; the guard is tested directly here
// rather than left implicit.
//
// WHAT CHANGED, and why it is stricter. The old test called this package's `gh()` helper and
// checked it refused. There is no CLI helper any more, so the refusal is asserted where it now
// lives: the custody hook this package installs on the resolver refuses to hand a backend an
// empty token, AND the backend itself refuses to construct a client without one. Two
// independent layers, and the second is checked against a recording server that must see zero
// requests — an assertion the argv-era test could not make at all.
func TestForgeRefusesWithoutMintedToken(t *testing.T) {
	t.Run("refuses_to_hand_over_an_empty_token", func(t *testing.T) {
		old := ghToken
		ghToken = ""
		t.Cleanup(func() { ghToken = old })

		_, _, err := forgeFor("example-org/tracker")
		if err == nil {
			t.Fatal("the resolver produced a backend with no minted worker token; it must refuse rather " +
				"than fall through to an ambient forge identity/keyring")
		}
		if !strings.Contains(err.Error(), "minted worker token") {
			t.Fatalf("refusal = %q, want it to name the missing minted token", err.Error())
		}
	})

	t.Run("and_the_backend_refuses_before_the_wire", func(t *testing.T) {
		var served int
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			served++
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(srv.Close)

		fg := &deskkit.GitHubForge{Token: "", BaseURL: srv.URL, Client: srv.Client()}
		if _, err := fg.GetPullRequest(deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}, 7); err == nil {
			t.Fatal("the backend reached the forge with no minted token")
		}
		if served != 0 {
			t.Fatalf("the refusal still issued %d request(s) — it must refuse BEFORE the wire", served)
		}
	})
}
