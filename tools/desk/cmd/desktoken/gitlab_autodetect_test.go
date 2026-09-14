package main

// gitlab_autodetect_test.go — reviewer-role (and every role) auth on a GitLab-resolved repo.
//
// The gap #798 §2 closes: `desktoken <role> --repo <gitlab-slug>` with NO explicit --forge used
// to fall through to the GitHub App mint and die with `no App ID for App "<role>-app"` (exit 6)
// — a GitHub App credential a PAT-backed GitLab bot never provisions. It now RESOLVES the forge
// from --repo and, on a definite GitLab resolution, takes the PAT custody path (rotate-on-mint),
// never reaching REVIEWER_APP_ID and never falling back to an ambient identity.

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Low-entropy, obvious-placeholder token-shaped fixtures (the `example` marker + a repeated
// digit) — the same convention forge_gitlab_test.go's glOld/glNewWorker use so the pattern
// leaksweep's placeholder filter classifies them as non-secret without an allowlist entry.
const (
	glOldReviewer = "glpat-example22222222222222222222-old"
	glNewReviewer = "glpat-example33333333333333333333-new"
)

// glExampleRepo is a sanctioned placeholder GitLab-resolved coordinate (never a real slug).
const glExampleRepo = "example-group/example-repo"

// rosterWithGitLabForge is the base single-owner roster PLUS an ASSAY_REPO_FORGES binding that
// resolves glExampleRepo to the GitLab forge, so ForgeKindFor answers GitLab without touching a
// remote. desktoken reads the roster from the config-home file (never the environment).
func rosterWithGitLabForge() string {
	return rosterForOwners("example-org") + "ASSAY_REPO_FORGES=" + glExampleRepo + "=gitlab\n"
}

// TestReviewerAuthGitlabPAT backs Verify item 5. `desktoken reviewer --repo <gitlab-repo>` (no
// --forge) auto-detects GitLab, resolves the custody-file PAT, and:
//   - takes the PAT rotate path (the rotate endpoint is hit) rather than the GitHub App mint;
//   - never emits the `no App ID for App "reviewer-app"` refusal (REVIEWER_APP_ID is unset and
//     is not read on the GitLab path);
//   - refuses (exit 6, "not found"), and does NOT fall back to an ambient identity, when the
//     custody PAT is absent.
func TestReviewerAuthGitlabPAT(t *testing.T) {
	t.Run("resolves_the_gitlab_pat_without_reaching_the_app_mint", func(t *testing.T) {
		homeDir := setupTest(t)
		plantRoster(t, homeDir, rosterWithGitLabForge())
		// REVIEWER_APP_ID deliberately unset — the GitHub App mint would exit 6 on its absence;
		// the GitLab path must never read it.
		t.Setenv("REVIEWER_APP_ID", "")

		tokPath := gitlabTokenPath(homeDir, "reviewer")
		writeTokenCache(t, tokPath, glOldReviewer)

		valid := glOldReviewer
		srv, calls := makeRotateServer(t, &valid, glNewReviewer, "2124-01-08T00:00:00Z")
		defer srv.Close()
		pointHTTPClientAt(t, srv) // sets GITLAB_API_BASE + routes httpClient at srv

		rc, stdout, stderr := runCap(t, []string{"reviewer", "--repo", glExampleRepo})
		if rc != deskkit.ExitOK {
			t.Fatalf("reviewer auth on a GitLab repo rc = %d, want 0 (exit 5 dedupe is also acceptable per the "+
				"brief, but NEVER the App-ID exit 6); stderr: %s", rc, stderr)
		}
		// The rotate endpoint was hit exactly once — proof the GitLab PAT custody path was
		// taken. The GitHub App mint never contacts the rotate endpoint.
		if *calls != 1 {
			t.Fatalf("GitLab rotate endpoint hit %d times, want 1 — the auto-detect did not route to the PAT path", *calls)
		}
		// The App-ID refusal (#772 symptom) must NEVER appear — that is the whole defect #798 §2 closes.
		if strings.Contains(stderr, "no App ID") || strings.Contains(stderr, "REVIEWER_APP_ID") {
			t.Fatalf("the GitLab path reached the GitHub App-ID mint (#772): %s", stderr)
		}
		if !strings.Contains(stdout, tokPath) {
			t.Fatalf("stdout must print the resolved PAT custody PATH %q; got: %s", tokPath, stdout)
		}
		assertNoTokenLeak(t, stdout+stderr)
	})

	t.Run("refuses_when_the_pat_is_absent_never_falls_back", func(t *testing.T) {
		homeDir := setupTest(t)
		plantRoster(t, homeDir, rosterWithGitLabForge())
		t.Setenv("REVIEWER_APP_ID", "")
		_ = homeDir
		// No gitlab-reviewer.token provisioned: the custody read refuses BEFORE any network.

		rc, stdout, stderr := runCap(t, []string{"reviewer", "--repo", glExampleRepo})
		if rc != deskkit.ExitUnverifiable {
			t.Fatalf("absent PAT rc = %d, want 6 (could-not-check); stderr: %s", rc, stderr)
		}
		if !strings.Contains(stderr, "not found") {
			t.Fatalf("absent-PAT refusal must name the missing custody file (not found), got: %s", stderr)
		}
		// It refused via the GitLab custody path (missing token file), NOT the GitHub App mint,
		// and NOT an ambient identity.
		if strings.Contains(stderr, "no App ID") || strings.Contains(stderr, "REVIEWER_APP_ID") {
			t.Fatalf("absent PAT must refuse via GitLab custody, never the GitHub App mint: %s", stderr)
		}
		assertNoTokenLeak(t, stdout+stderr)
	})
}

// TestGitLabAutoDetectDoesNotDivertGitHubOrUnresolved guards the three-state fall-through: a
// GitHub-resolved repo and an unresolved repo must NOT be diverted to the GitLab PAT path — only
// an affirmative GitLab resolution diverts. gitlabRepoResolved is the predicate the dispatch
// keys on, so asserting it directly pins the fall-through without needing a GitHub App credential.
func TestGitLabAutoDetectDoesNotDivertGitHubOrUnresolved(t *testing.T) {
	homeDir := setupTest(t)
	plantRoster(t, homeDir, rosterWithGitLabForge()+"") // example-org repos are GitHub via origin/roster default

	if gitlabRepoResolved(glExampleRepo) != true {
		t.Fatal("a repo bound to gitlab in ASSAY_REPO_FORGES must resolve as GitLab")
	}
	// A bare owner is not a resolvable coordinate — must not divert.
	if gitlabRepoResolved("example-org") {
		t.Fatal("a bare owner must not resolve as GitLab (no repo coordinate)")
	}
	// Empty --repo must not divert (falls through to the GitHub mint's own owner resolution).
	if gitlabRepoResolved("") {
		t.Fatal("an empty --repo must not resolve as GitLab")
	}
	// A repo the roster binds to github must not divert.
	if gitlabRepoResolved("example-org/example-repo") {
		// example-org/example-repo is not in the gitlab roster binding; a could-not-check or a
		// github resolution both mean "do not divert".
		t.Fatal("a non-gitlab-bound repo must not resolve as GitLab (fall through to the App mint)")
	}
}
