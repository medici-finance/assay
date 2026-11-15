package deskkit

import (
	"strings"
	"testing"
)

// TestForgeRepoInfoFetcherDrivesPublicRepoGateOnGitLab proves the public-repo WRITE gate
// (repovis.go's PublicRepoGate) runs correctly on a GitLab-resolved repo through the generic
// ForgeRepoInfoFetcher (repovis.go) wrapping a real *GitLabForge — the same adapter every
// command site now uses, rather than the retired single-forge GitLabRepoInfoFetcher shim
// (assay#1066). The gate itself is exercised unchanged; the only surface under test is that
// the adapter's one read (RepoVisibility) resolves correctly against the GitLab backend. The
// gate authorizes by REPOSITORY (a configured allowed-repos entry tagged `:public`), not by a
// per-item reaction — so these rows drive (live visibility × configured entry) exactly as
// repovis_test.go's generic-fetcher rows do, just through a live-shaped GitLab server instead
// of the package fake.
func TestForgeRepoInfoFetcherDrivesPublicRepoGateOnGitLab(t *testing.T) {
	t.Run("private repo skips the gate regardless of the roster", func(t *testing.T) {
		s := newGLServer(t)
		s.project = map[string]any{"visibility": "private"}
		fetcher := ForgeRepoInfoFetcher{Forge: s.forge()}
		if err := PublicRepoGate(fetcher, glRepo.Owner, glRepo.Name); err != nil {
			t.Fatalf("a private repo must pass the gate, got %v", err)
		}
	})

	t.Run("public repo listed :public in the roster passes", func(t *testing.T) {
		installRoster(t, "ASSAY_ALLOWED_REPOS="+glRepo.Owner+"/"+glRepo.Name+":ci:public\n")
		s := newGLServer(t)
		s.project = map[string]any{"visibility": "public"}
		fetcher := ForgeRepoInfoFetcher{Forge: s.forge()}
		if err := PublicRepoGate(fetcher, glRepo.Owner, glRepo.Name); err != nil {
			t.Fatalf("a public repo listed :public must pass, got %v", err)
		}
	})

	t.Run("public repo NOT listed :public is refused", func(t *testing.T) {
		installRoster(t, "ASSAY_ALLOWED_REPOS="+glRepo.Owner+"/"+glRepo.Name+":ci:private\n")
		s := newGLServer(t)
		s.project = map[string]any{"visibility": "public"}
		fetcher := ForgeRepoInfoFetcher{Forge: s.forge()}
		err := PublicRepoGate(fetcher, glRepo.Owner, glRepo.Name)
		if err == nil || !IsRefused(err) {
			t.Fatalf("a public repo whose roster entry is :private must be refused (exit 5), got %v", err)
		}
	})

	t.Run("public repo absent from the roster is refused", func(t *testing.T) {
		installRoster(t, "ASSAY_ALLOWED_REPOS=example-org/unrelated:ci:public\n")
		s := newGLServer(t)
		s.project = map[string]any{"visibility": "public"}
		fetcher := ForgeRepoInfoFetcher{Forge: s.forge()}
		err := PublicRepoGate(fetcher, glRepo.Owner, glRepo.Name)
		if err == nil || !IsRefused(err) {
			t.Fatalf("a public repo absent from the roster must be refused (exit 5), got %v", err)
		}
	})

	// #5514: a GitLab `internal` project listed `:public` passes — the gate tolerates the
	// mislabel, since the entry states a wider audience than the project has. `:internal` is
	// the accurate token and passes too; every entry that does not rank at least as exposed as
	// live-internal still refuses, naming live-internal and its cause.
	t.Run("internal project listed :public passes", func(t *testing.T) {
		installRoster(t, "ASSAY_ALLOWED_REPOS="+glRepo.Owner+"/"+glRepo.Name+":ci:public\n")
		s := newGLServer(t)
		s.project = map[string]any{"visibility": "internal"}
		if err := PublicRepoGate(ForgeRepoInfoFetcher{Forge: s.forge()}, glRepo.Owner, glRepo.Name); err != nil {
			t.Fatalf("an internal project listed :public must pass, got %v", err)
		}
	})

	t.Run("public project listed :internal is refused", func(t *testing.T) {
		installRoster(t, "ASSAY_ALLOWED_REPOS="+glRepo.Owner+"/"+glRepo.Name+":ci:internal\n")
		s := newGLServer(t)
		s.project = map[string]any{"visibility": "public"}
		err := PublicRepoGate(ForgeRepoInfoFetcher{Forge: s.forge()}, glRepo.Owner, glRepo.Name)
		if !IsRefused(err) {
			t.Fatalf("a public project listed :internal must be refused (exit 5), got %v", err)
		}
		if msg := err.Error(); !strings.Contains(msg, "reads live-public") || !strings.Contains(msg, glRepo.Owner+"/"+glRepo.Name+":public") {
			t.Fatalf("the refusal must read the project as public and name the :public entry: %v", err)
		}
	})

	for _, tc := range []struct{ name, roster, cause string }{
		{"internal project listed :private is refused", glRepo.Owner + "/" + glRepo.Name + ":ci:private", "a :private entry authorizes no outward write"},
		{"internal project matched only by a pattern is refused", glRepo.Owner + "/*", "matched only by an owner/* pattern"},
		{"internal project listed with no token is refused", glRepo.Owner + "/" + glRepo.Name + ":ci", "the entry carries no visibility token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			installRoster(t, "ASSAY_ALLOWED_REPOS="+tc.roster+"\n")
			s := newGLServer(t)
			s.project = map[string]any{"visibility": "internal"}
			err := PublicRepoGate(ForgeRepoInfoFetcher{Forge: s.forge()}, glRepo.Owner, glRepo.Name)
			if !IsRefused(err) {
				t.Fatalf("%s: got %v, want Refused/exit 5", tc.name, err)
			}
			msg := err.Error()
			if !strings.Contains(msg, "reads live-internal") || strings.Contains(msg, "live-public") || !strings.Contains(msg, tc.cause) {
				t.Fatalf("%s: the refusal must read the project as internal and name its cause %q: %v", tc.name, tc.cause, err)
			}
		})
	}

	t.Run("internal project listed :internal passes", func(t *testing.T) {
		installRoster(t, "ASSAY_ALLOWED_REPOS="+glRepo.Owner+"/"+glRepo.Name+":ci:internal\n")
		s := newGLServer(t)
		s.project = map[string]any{"visibility": "internal"}
		if err := PublicRepoGate(ForgeRepoInfoFetcher{Forge: s.forge()}, glRepo.Owner, glRepo.Name); err != nil {
			t.Fatalf("an internal project listed :internal must pass, got %v", err)
		}
		if RepoVisibility(glRepo.Owner+"/"+glRepo.Name) != VisibilityInternal {
			t.Fatal("a :internal entry must configure VisibilityInternal, never public")
		}
	})

	t.Run("an unreadable visibility fails the gate closed", func(t *testing.T) {
		installRoster(t, "ASSAY_ALLOWED_REPOS="+glRepo.Owner+"/"+glRepo.Name+":ci:public\n")
		s := newGLServer(t)
		// A project payload with no .visibility field: the backend surfaces could-not-check, and
		// the gate must refuse rather than treat the repo as private (fail closed, not open).
		s.project = map[string]any{}
		fetcher := ForgeRepoInfoFetcher{Forge: s.forge()}
		if err := PublicRepoGate(fetcher, glRepo.Owner, glRepo.Name); err == nil {
			t.Fatal("an unreadable visibility must fail the gate closed, got nil (a fall-open hole)")
		}
	})
}
