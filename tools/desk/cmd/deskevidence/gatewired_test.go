package main

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestPublicRepoGateFetcherRoutesThroughResolvedForge — assay#1066 (the class assay#1060
// closed for deskpr's create/update/edit; the review that landed that fix found this defect
// still standing here).
//
// deskevidence must ask the public-repo gate's visibility read through the SAME forge backend
// already resolved for every OTHER operation on this repo (the fg forgeForFn returned, reused
// below for ReadFile/WriteFile), never a second, independently-constructed, GitHub-only
// client. Before the fix, this call site built
// `&deskkit.HTTPRepoInfoFetcher{Token: ghToken}` regardless of which forge the resolver had
// actually picked, so a GitLab-resolved repo's visibility read went out over GitHub's REST
// API — which cannot answer for a project that lives on GitLab — and the gate failed closed
// (could-not-check) naming a foreign host error for a repo it never heard of.
//
// FAIL-FIRST: restore `fetcher := &deskkit.HTTPRepoInfoFetcher{Token: ghToken}` at
// deskevidence.go and this test goes red at the type assertion below — a raw
// *HTTPRepoInfoFetcher is not a deskkit.ForgeRepoInfoFetcher, so the mismatch is caught BEFORE
// this test ever touches RepoVisibility. That ordering is deliberate: calling RepoVisibility on
// the pre-fix fetcher would dial the real api.github.com (an empty BaseURL defaults there) with
// a fake token, which this offline test must never do.
func TestPublicRepoGateFetcherRoutesThroughResolvedForge(t *testing.T) {
	f, _ := setupFake(t)
	f.visibility = "private"
	evidencePath := writeRepoFile(t, "docs/brief.md", "# Brief\n\n## Evidence\n| 1 | ... | evidence row |\n")
	f.setFile(evidencePath, "# Brief\n\n## Evidence\n")

	var captured deskkit.RepoInfoFetcher
	oldGate := publicRepoGateFn
	publicRepoGateFn = func(fetcher deskkit.RepoInfoFetcher, owner, repo string) error {
		captured = fetcher
		return nil
	}
	t.Cleanup(func() { publicRepoGateFn = oldGate })

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})
	if code != deskkit.ExitOK {
		t.Fatalf("run exit = %d, want 0", code)
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
	if routed.Forge != deskkit.Forge(f) {
		t.Fatal("the fetcher wraps a different forge than the one forgeForFn resolved for this repo")
	}

	// Safe to exercise now: routed.Forge is the in-memory fake, so this makes no network call.
	// The pre-fix path never reaches here — its type assertion above already failed.
	vis, verr := routed.RepoVisibility("example-org", "tracker")
	if verr != nil {
		t.Fatalf("RepoVisibility via the resolved forge: %v", verr)
	}
	if vis != "private" {
		t.Fatalf("RepoVisibility = %q, want %q (the fake's configured value)", vis, "private")
	}
	if f.visibilityCalls != 1 {
		t.Fatalf("fake.visibilityCalls = %d, want 1 — the gate's read must reach the resolved "+
			"forge exactly once", f.visibilityCalls)
	}
	if f.visibilityRepo != (deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}) {
		t.Fatalf("fake asked about %+v, want example-org/tracker", f.visibilityRepo)
	}
}
