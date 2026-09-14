package deskkit

import "testing"

// TestGitLabRepoInfoFetcherDrivesPublicRepoGate proves the public-repo WRITE gate
// (repovis.go's PublicRepoGate) runs on a GitLab-resolved repo through the adapter — the whole
// point of the shim. The gate itself is exercised unchanged; the only new surface is the
// adapter routing its one read (RepoVisibility) onto the GitLab backend. The gate now
// authorizes by REPOSITORY (a configured allowed-repos entry tagged `:public`), not by a
// per-item reaction — so these rows drive (live visibility × configured entry) exactly as
// repovis_test.go's generic-fetcher rows do, just through the GitLab adapter instead of the
// package stub.
func TestGitLabRepoInfoFetcherDrivesPublicRepoGate(t *testing.T) {
	t.Run("private repo skips the gate regardless of the roster", func(t *testing.T) {
		s := newGLServer(t)
		s.project = map[string]any{"visibility": "private"}
		fetcher := GitLabRepoInfoFetcher{Forge: s.forge()}
		if err := PublicRepoGate(fetcher, glRepo.Owner, glRepo.Name); err != nil {
			t.Fatalf("a private repo must pass the gate, got %v", err)
		}
	})

	t.Run("public repo listed :public in the roster passes", func(t *testing.T) {
		installRoster(t, "ASSAY_ALLOWED_REPOS="+glRepo.Owner+"/"+glRepo.Name+":ci:public\n")
		s := newGLServer(t)
		s.project = map[string]any{"visibility": "public"}
		fetcher := GitLabRepoInfoFetcher{Forge: s.forge()}
		if err := PublicRepoGate(fetcher, glRepo.Owner, glRepo.Name); err != nil {
			t.Fatalf("a public repo listed :public must pass, got %v", err)
		}
	})

	t.Run("public repo NOT listed :public is refused", func(t *testing.T) {
		installRoster(t, "ASSAY_ALLOWED_REPOS="+glRepo.Owner+"/"+glRepo.Name+":ci:private\n")
		s := newGLServer(t)
		s.project = map[string]any{"visibility": "public"}
		fetcher := GitLabRepoInfoFetcher{Forge: s.forge()}
		err := PublicRepoGate(fetcher, glRepo.Owner, glRepo.Name)
		if err == nil || !IsRefused(err) {
			t.Fatalf("a public repo whose roster entry is :private must be refused (exit 5), got %v", err)
		}
	})

	t.Run("public repo absent from the roster is refused", func(t *testing.T) {
		installRoster(t, "ASSAY_ALLOWED_REPOS=example-org/unrelated:ci:public\n")
		s := newGLServer(t)
		s.project = map[string]any{"visibility": "public"}
		fetcher := GitLabRepoInfoFetcher{Forge: s.forge()}
		err := PublicRepoGate(fetcher, glRepo.Owner, glRepo.Name)
		if err == nil || !IsRefused(err) {
			t.Fatalf("a public repo absent from the roster must be refused (exit 5), got %v", err)
		}
	})

	t.Run("an unreadable visibility fails the gate closed", func(t *testing.T) {
		installRoster(t, "ASSAY_ALLOWED_REPOS="+glRepo.Owner+"/"+glRepo.Name+":ci:public\n")
		s := newGLServer(t)
		// A project payload with no .visibility field: the backend surfaces could-not-check, and
		// the gate must refuse rather than treat the repo as private (fail closed, not open).
		s.project = map[string]any{}
		fetcher := GitLabRepoInfoFetcher{Forge: s.forge()}
		if err := PublicRepoGate(fetcher, glRepo.Owner, glRepo.Name); err == nil {
			t.Fatal("an unreadable visibility must fail the gate closed, got nil (a fall-open hole)")
		}
	})
}
