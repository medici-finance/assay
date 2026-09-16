package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestPublicRepoGateFetcherRoutesThroughResolvedForge — assay#1066 (the class assay#1060
// closed for deskpr's create/update/edit; the review that landed that fix found this defect
// still standing here).
//
// deskreply must ask the public-repo gate's visibility read through the SAME forge backend
// already resolved for every OTHER operation on this repo (the fg forgeFor(repo) returned,
// reused below for the PR-state read), never a second, independently-constructed,
// GitHub-only client. Before the fix, this call site built
// `&deskkit.HTTPRepoInfoFetcher{Token: ghToken}` regardless of which forge the resolver had
// actually picked, so a GitLab-resolved repo's visibility read went out over GitHub's REST
// API — which cannot answer for a project that lives on GitLab — and the gate failed closed
// (could-not-check) naming a foreign host error for a repo it never heard of.
//
// FAIL-FIRST: restore `fetcher := &deskkit.HTTPRepoInfoFetcher{Token: ghToken}` at deskreply.go
// and this test goes red at the type assertion below — a raw *HTTPRepoInfoFetcher is not a
// deskkit.ForgeRepoInfoFetcher, so the mismatch is caught BEFORE this test ever touches
// RepoVisibility. That ordering is deliberate: calling RepoVisibility on the pre-fix fetcher
// would either dial the real api.github.com (an empty BaseURL defaults there) or panic on a nil
// Client in this offline harness — this test must never depend on live network.
func TestPublicRepoGateFetcherRoutesThroughResolvedForge(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	body := bodyFileWith(t, "routing check")

	rec := forgeRec(t)
	rec.visibility = "private"

	var captured deskkit.RepoInfoFetcher
	oldGate := publicRepoGateFn
	publicRepoGateFn = func(fetcher deskkit.RepoInfoFetcher, owner, repo string) error {
		captured = fetcher
		return nil
	}
	t.Cleanup(func() { publicRepoGateFn = oldGate })

	rc := run([]string{"example-org/tracker", "7", "--body-file", body})
	if rc != deskkit.ExitOK {
		t.Fatalf("reply rc = %d, want 0", rc)
	}

	if captured == nil {
		t.Fatal("the gate seam was never reached")
	}
	routed, ok := captured.(deskkit.ForgeRepoInfoFetcher)
	if !ok {
		t.Fatalf("fetcher handed to the public-repo gate is %T, want deskkit.ForgeRepoInfoFetcher "+
			"wrapping the forge already resolved for this repo — a hardcoded GitHub-only client "+
			"cannot answer for a GitLab-resolved repo (assay#1066)", captured)
	}

	// Safe to exercise now: routed.Forge is the resolved backend pointed at THIS test's
	// httptest recorder (forgeAPIBase), so this makes no real network call. The pre-fix path
	// never reaches here — its type assertion above already failed.
	vis, verr := routed.RepoVisibility("example-org", "tracker")
	if verr != nil {
		t.Fatalf("RepoVisibility via the resolved forge: %v", verr)
	}
	if vis != "private" {
		t.Fatalf("RepoVisibility = %q, want %q (the recorder's configured value)", vis, "private")
	}

	var sawVisibilityGET bool
	for _, r := range rec.requests {
		if r.Method == http.MethodGet && strings.Count(strings.Trim(r.Path, "/"), "/") == 2 &&
			strings.HasPrefix(r.Path, "/repos/") {
			sawVisibilityGET = true
		}
	}
	if !sawVisibilityGET {
		t.Fatalf("no GET /repos/{owner}/{repo} request reached the resolved forge's recorder; requests: %v", rec.requests)
	}
}
