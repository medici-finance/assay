package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestPublicRepoGateWired — deskpr refuses (and makes ZERO write calls) when the gate
// refuses, and asks the gate about the REAL target repository. Authorization is
// repository-scoped now (a listed :public allowed-repos entry), so there is no issue/PR
// number in the gate signature to pin.
func TestPublicRepoGateWired(t *testing.T) {
	t.Run("create_refused", func(t *testing.T) {
		work := newBaseFixture(t)
		calls := withEnv(t, work)

		publicRepoGateFn = func(_ deskkit.RepoInfoFetcher, owner, repo string) error {
			return deskkit.Refused("public-repo gate: " + owner + "/" + repo + " is not authorized")
		}

		rc := run([]string{"create", "--title", "Test PR", "--body-min", "Brief: fixture/01\nbody"})
		if rc != deskkit.ExitRefused {
			t.Fatalf("create on a refused repo rc = %d, want 5 (refused)", rc)
		}
		for _, c := range *calls {
			if len(c) >= 2 && c[0] == "git" && c[1] == "push" {
				t.Fatalf("gate refused -- must NOT make git push call; calls: %v", *calls)
			}
		}
	})

	t.Run("update_refused", func(t *testing.T) {
		work := newBaseFixture(t)
		calls := withEnv(t, work)
		t.Setenv("FAKEGH_LIST_HAS_PR", "1") // open draft PR on the branch

		publicRepoGateFn = func(_ deskkit.RepoInfoFetcher, owner, repo string) error {
			return deskkit.Refused("public-repo gate: " + owner + "/" + repo + " is not authorized")
		}

		rc := run([]string{"update"})
		if rc != deskkit.ExitRefused {
			t.Fatalf("update on a refused repo rc = %d, want 5 (refused)", rc)
		}
		for _, c := range *calls {
			if len(c) >= 2 && c[0] == "git" && c[1] == "push" {
				t.Fatalf("gate refused -- must NOT make git push call; calls: %v", *calls)
			}
		}
	})

	// The two subtests above prove the caller PROPAGATES a refusal, but their stubs ignore
	// every argument — so they would pass identically if `update` asked the gate about the
	// wrong repo. What the gate is ASKED is the whole authorization question, so pin it.
	t.Run("update_asks_the_gate_about_the_real_target", func(t *testing.T) {
		work := newBaseFixture(t)
		withEnv(t, work)
		t.Setenv("FAKEGH_LIST_HAS_PR", "1")

		var gotOwner, gotRepo string
		var calledTimes int
		publicRepoGateFn = func(_ deskkit.RepoInfoFetcher, owner, repo string) error {
			calledTimes++
			gotOwner, gotRepo = owner, repo
			return deskkit.Refused("public-repo gate: stub refusal")
		}

		if rc := run([]string{"update"}); rc != deskkit.ExitRefused {
			t.Fatalf("update rc = %d, want 5 (refused)", rc)
		}
		if calledTimes != 1 {
			t.Fatalf("gate called %d times, want exactly 1", calledTimes)
		}
		if gotOwner != "example-org" || gotRepo != "tracker" {
			t.Fatalf("gate asked about %s/%s, want example-org/tracker — a gate "+
				"asked about the wrong repo reads the wrong repo's visibility", gotOwner, gotRepo)
		}
	})
}

// TestPublicRepoGateFetcherRoutesThroughResolvedForge — assay#1054.
//
// create/update/edit must ask the public-repo gate's visibility read through the SAME forge
// backend already resolved for every OTHER operation on this repo (forgeForFn's fg), never a
// second, independently-constructed, GitHub-only client. Before the fix, all three call sites
// built `&deskkit.HTTPRepoInfoFetcher{Token: ghToken}` for the gate regardless of which forge
// the resolver had actually picked, so a GitLab-resolved repo's visibility read went out over
// GitHub's REST API — which cannot answer for a project that lives on GitLab, and the gate
// failed closed (could-not-check, exit 6) naming a GitHub HTTP error for a repo GitHub never
// heard of, rather than reading GitLab's own visibility. GitLabForge.RepoVisibility
// (forge_gitlab.go) was already correct; nothing downstream of the resolver was broken — the
// gate simply never asked it.
//
// FAIL-FIRST: restore `fetcher := &deskkit.HTTPRepoInfoFetcher{Token: ghToken}` at any of the
// three deskpr call sites (deskpr.go's create/update, edit.go's edit) and the matching subtest
// below goes red at the type assertion — a raw *HTTPRepoInfoFetcher is not a
// deskkit.ForgeRepoInfoFetcher, so the mismatch is caught BEFORE this test ever touches
// RepoVisibility. That ordering is deliberate: calling RepoVisibility on the pre-fix fetcher
// would issue a real HTTP GET to api.github.com with the fixture's fake token, which this test
// must never do (offline, deterministic, no flaky network dependency).
func TestPublicRepoGateFetcherRoutesThroughResolvedForge(t *testing.T) {
	assertRoutedThroughFake := func(t *testing.T, fake *envForge, captured deskkit.RepoInfoFetcher, wantOwner, wantRepo string) {
		t.Helper()
		if captured == nil {
			t.Fatal("the gate seam was never reached")
		}
		routed, ok := captured.(deskkit.ForgeRepoInfoFetcher)
		if !ok {
			t.Fatalf("fetcher handed to the public-repo gate is %T, want deskkit.ForgeRepoInfoFetcher "+
				"wrapping the forge already resolved for this repo — a hardcoded GitHub-only client "+
				"cannot answer for a GitLab-resolved repo (assay#1054)", captured)
		}
		if routed.Forge != fake {
			t.Fatal("the fetcher wraps a different forge than the one forgeForFn resolved for this repo")
		}
		// Safe to exercise now: routed.Forge is the in-memory fake, so this makes no network
		// call. The pre-fix path never reaches here — its type assertion above already failed.
		vis, err := routed.RepoVisibility(wantOwner, wantRepo)
		if err != nil {
			t.Fatalf("RepoVisibility via the resolved forge: %v", err)
		}
		if vis != "private" {
			t.Fatalf("RepoVisibility = %q, want %q (the fake's default)", vis, "private")
		}
		if fake.visibilityCalls != 1 {
			t.Fatalf("fake.visibilityCalls = %d, want 1 — the gate's read must reach the resolved "+
				"forge exactly once", fake.visibilityCalls)
		}
		if fake.visibilityRepo != (deskkit.ForgeRepo{Owner: wantOwner, Name: wantRepo}) {
			t.Fatalf("fake asked about %+v, want %s/%s", fake.visibilityRepo, wantOwner, wantRepo)
		}
	}

	t.Run("create", func(t *testing.T) {
		work := newBaseFixture(t)
		withEnv(t, work)
		fake := curForge

		var captured deskkit.RepoInfoFetcher
		publicRepoGateFn = func(fetcher deskkit.RepoInfoFetcher, owner, repo string) error {
			captured = fetcher
			return nil
		}

		run([]string{"create", "--title", "Test PR", "--body-min", "Brief: fixture/01\nbody"})
		assertRoutedThroughFake(t, fake, captured, "example-org", "tracker")
	})

	t.Run("update", func(t *testing.T) {
		work := newBaseFixture(t)
		withEnv(t, work)
		t.Setenv("FAKEGH_LIST_HAS_PR", "1")
		fake := curForge

		var captured deskkit.RepoInfoFetcher
		publicRepoGateFn = func(fetcher deskkit.RepoInfoFetcher, owner, repo string) error {
			captured = fetcher
			return nil
		}

		run([]string{"update"})
		assertRoutedThroughFake(t, fake, captured, "example-org", "tracker")
	})

	t.Run("edit", func(t *testing.T) {
		work := newBaseFixture(t)
		withEnv(t, work)
		t.Setenv("FAKEGH_LIST_HAS_PR", "1")
		fake := curForge

		var captured deskkit.RepoInfoFetcher
		publicRepoGateFn = func(fetcher deskkit.RepoInfoFetcher, owner, repo string) error {
			captured = fetcher
			return nil
		}

		bodyPath := writeTempFile(t, "the corrected body\nBrief: fixture/01\n")
		run([]string{"edit", "--body-file", bodyPath})
		assertRoutedThroughFake(t, fake, captured, "example-org", "tracker")
	})
}

// TestBriefCarryingCreateOnListedPublicRepoPassesGate — the defect this brief closes.
//
// A brief-carrying create has NO issue/PR number (the trailer resolves to a file, not an
// issue). Under the former gate that meant `issueNumber == 0`, which was a hard exit-6
// refusal on any public repo — the first pull request on a public repo was unopenable by
// the tool. With the per-item `+1` replaced by the repository-scoped `:public`
// authorization, a brief-carrying create on a LISTED PUBLIC repo passes the gate seam.
//
// The seam runs the REAL deskkit.PublicRepoGate (proving the number is truly gone from the
// decision) against a fake fetcher reporting live "public"; the fixture roster lists
// example-org/tracker — normally :private, so this test installs a roster that lists it
// :public — and drives the whole create through the fake forge.
//
// FAIL-FIRST: re-add an `issueNumber int` parameter to deskkit.PublicRepoGate and restore
// its `issueNumber <= 0 → Unverifiable` arm, and this test goes red — a brief-carrying
// create (number 0) would refuse at exit 6 again, which is exactly the state this closes.
func TestBriefCarryingCreateOnListedPublicRepoPassesGate(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)

	// Install a roster in which the fixture repo (example-org/tracker) is listed :public,
	// so the configured-visibility read the real gate performs authorizes it.
	installGateRoster(t, "example-org/tracker:ci:public")

	// Drive the REAL gate with a fetcher that reports the repo live-public. The seam
	// substitutes the fake fetcher for the production HTTPRepoInfoFetcher; the gate itself
	// is genuine, so a reintroduced issue-number arm would refuse a brief-carrying create.
	// We assert on the gate's OWN verdict (not the create's overall exit code), so the test
	// pins the gate-seam claim without depending on the rest of the create happy path.
	var sawGate bool
	var gateErr error
	publicRepoGateFn = func(_ deskkit.RepoInfoFetcher, owner, repo string) error {
		sawGate = true
		gateErr = deskkit.PublicRepoGate(livePublicFetcher{}, owner, repo)
		return gateErr
	}

	run([]string{"create", "--title", "Test PR", "--body-min", "Brief: fixture/01\nbody"})
	if !sawGate {
		t.Fatal("the gate seam was never reached on create — a brief-carrying create must still consult it")
	}
	if gateErr != nil {
		t.Fatalf("brief-carrying create on a listed :public repo: the gate REFUSED (%v) — the create "+
			"path must no longer refuse for want of an issue number", gateErr)
	}
}

// installGateRoster rewrites the roster under the current HOME with a valid identity set
// and the given ASSAY_ALLOWED_REPOS value, then reloads the config cache. Call it AFTER
// withEnv so it overrides the fixture roster for this test.
func installGateRoster(t *testing.T, allowedRepos string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	roster := "ASSAY_BLESS_LOGIN=ada:2001\n" +
		"ASSAY_TRUSTED_LOGINS=ada:2001,shared-agent:2002\n" +
		"ASSAY_TRUSTED_BOT_SLUGS=desk=assay-desk-app:300000001,worker=assay-worker-app:300000006,reviewer=assay-reviewer-app:300000004,verifier=assay-verifier-app:300000005\n" +
		"ASSAY_ALLOWED_REPOS=" + allowedRepos + "\n"
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
}

// livePublicFetcher is a deskkit.RepoInfoFetcher that reports every repo live-public, so a
// test can exercise the real PublicRepoGate without a network call.
type livePublicFetcher struct{}

func (livePublicFetcher) RepoVisibility(owner, repo string) (string, error) { return "public", nil }

// TestGateSeamIsRealInProduction — the seam these tests replace must, in a fresh binary, be
// the real deskkit.PublicRepoGate. publicRepoGateFn is stubbed by every other test here, so
// they prove only that the caller PROPAGATES the gate's error; asserting the production
// binding is what makes those assertions mean something.
func TestGateSeamIsRealInProduction(t *testing.T) {
	if productionGateFn == nil {
		t.Fatal("productionGateFn is nil — the seam has no recorded production binding")
	}
	if fmt.Sprintf("%p", productionGateFn) != fmt.Sprintf("%p", deskkit.PublicRepoGate) {
		t.Fatal("publicRepoGateFn is not bound to deskkit.PublicRepoGate at init — the gate " +
			"is stubbed out in the shipped binary, and every other gate test here is vacuous")
	}
}
