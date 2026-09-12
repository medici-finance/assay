package deskkit

import (
	"strconv"
	"testing"
)

// TestGitLabRepoInfoFetcherDrivesPublicRepoGate proves the public-repo SECURITY gate runs on a
// GitLab-resolved repo through the adapter — the whole point of the shim. The gate is exercised
// unchanged (it is repovis.go's PublicRepoGate); the only new surface is the adapter routing its
// two reads onto the GitLab backend. The bless authority is the fixture roster's "ada":2001
// (rosterfixture_test.go), so a +1 from that identity is the one that clears the gate.
//
// The rows walk the gate's decision table on GitLab: private skips it; public needs a human +1;
// a bot award and an absent award are both refused; and an unreadable visibility fails CLOSED —
// the adapter adds no fall-open path.
func TestGitLabRepoInfoFetcherDrivesPublicRepoGate(t *testing.T) {
	t.Run("private repo skips the gate without a +1", func(t *testing.T) {
		s := newGLServer(t)
		s.project = map[string]any{"visibility": "private"}
		fetcher := GitLabRepoInfoFetcher{Forge: s.forge()}
		if err := PublicRepoGate(fetcher, glRepo.Owner, glRepo.Name, 7); err != nil {
			t.Fatalf("a private repo must pass the gate, got %v", err)
		}
	})

	t.Run("public repo with a bless-authority +1 passes", func(t *testing.T) {
		s := newGLServer(t)
		s.project = map[string]any{"visibility": "public"}
		s.awards = []map[string]any{
			{"name": "thumbsup", "user": map[string]any{"id": fixtureBlessID, "username": fixtureBlessLogin}},
		}
		s.users[strconv.FormatInt(fixtureBlessID, 10)] = map[string]any{
			"id": fixtureBlessID, "username": fixtureBlessLogin, "bot": false,
		}
		fetcher := GitLabRepoInfoFetcher{Forge: s.forge()}
		if err := PublicRepoGate(fetcher, glRepo.Owner, glRepo.Name, 7); err != nil {
			t.Fatalf("a public repo with the bless authority's +1 must pass, got %v", err)
		}
	})

	t.Run("public repo with only a bot +1 is refused", func(t *testing.T) {
		s := newGLServer(t)
		s.project = map[string]any{"visibility": "public"}
		s.awards = []map[string]any{
			{"name": "thumbsup", "user": map[string]any{"id": int64(2), "username": "desk-bot"}},
		}
		s.users["2"] = map[string]any{"id": int64(2), "username": "desk-bot", "bot": true}
		fetcher := GitLabRepoInfoFetcher{Forge: s.forge()}
		err := PublicRepoGate(fetcher, glRepo.Owner, glRepo.Name, 7)
		if err == nil || !IsRefused(err) {
			t.Fatalf("a bot +1 must be refused (exit 5), got %v", err)
		}
	})

	t.Run("public repo with no qualifying +1 is refused", func(t *testing.T) {
		s := newGLServer(t)
		s.project = map[string]any{"visibility": "public"}
		s.awards = nil
		fetcher := GitLabRepoInfoFetcher{Forge: s.forge()}
		err := PublicRepoGate(fetcher, glRepo.Owner, glRepo.Name, 7)
		if err == nil || !IsRefused(err) {
			t.Fatalf("no qualifying +1 must be refused (exit 5), got %v", err)
		}
	})

	t.Run("an unreadable visibility fails the gate closed", func(t *testing.T) {
		s := newGLServer(t)
		// A project payload with no .visibility field: the backend surfaces could-not-check, and
		// the gate must refuse rather than treat the repo as private (fail closed, not open).
		s.project = map[string]any{}
		fetcher := GitLabRepoInfoFetcher{Forge: s.forge()}
		if err := PublicRepoGate(fetcher, glRepo.Owner, glRepo.Name, 7); err == nil {
			t.Fatal("an unreadable visibility must fail the gate closed, got nil (a fall-open hole)")
		}
	})
}
